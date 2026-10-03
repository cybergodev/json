package json

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync/atomic"

	"github.com/cybergodev/json/internal"
)

// Parse parses a JSON string into the provided target with improved error handling.
// This is the core parsing method that supports both standard and number-preserving modes.
//
// Parameters:
//   - jsonStr: the JSON string to parse
//   - target: pointer to the target variable where parsed data will be stored
//   - opts: optional Config for parsing options (e.g., PreserveNumbers)
//
// Errors:
//   - ErrProcessorClosed: processor has been closed
//   - ErrInvalidJSON: jsonStr is not valid JSON
//   - ErrSizeLimit: JSON exceeds MaxJSONSize
//   - ErrTypeMismatch: JSON structure doesn't match target type
//
// Example:
//
//	// Parse into map
//	var obj map[string]any
//	err := processor.Parse(`{"name":"Alice"}`, &obj)
//
//	// Parse into struct
//	type User struct { Name string }
//	var user User
//	err := processor.Parse(`{"name":"Alice"}`, &user)
//
//	// Parse with number preservation
//	cfg := json.DefaultConfig()
//	cfg.PreserveNumbers = true
//	var data any
//	err := processor.Parse(`{"price":19.99}`, &data, cfg)
func (p *Processor) Parse(jsonStr string, target any, cfg ...Config) error {
	// PERFORMANCE v2: Fast path for the most common case — no config,
	// target is *any, not preserving numbers. Avoids config allocation
	// and uses streamlined error wrapping.
	if len(cfg) == 0 {
		if p == nil || atomic.LoadInt32(&p.state) != processorStateActive {
			return &JsonsError{Op: "parse", Message: "processor is closed", Err: ErrProcessorClosed}
		}
		if _, ok := target.(*any); ok && !p.config.PreserveNumbers {
			// SECURITY: Full input validation is required (size, depth, security patterns)
			if err := p.validateInput(jsonStr); err != nil {
				return err
			}
			if err := json.Unmarshal(stringToBytes(jsonStr), target); err != nil {
				return &JsonsError{
					Op:      "parse",
					Message: err.Error(),
					Err:     ErrInvalidJSON,
				}
			}
			return nil
		}
	}

	if err := p.checkClosed(); err != nil {
		return err
	}

	options, err := p.prepareOptions(cfg...)
	if err != nil {
		return err
	}
	defer releaseConfig(options)

	if err := p.validateInputForOptions(jsonStr, options); err != nil {
		return err
	}

	if target == nil {
		return &JsonsError{
			Op:      "parse",
			Message: "target cannot be nil, use Parse for any type result",
			Err:     errOperationFailed,
		}
	}

	// PERFORMANCE: Fast path for the most common case — parsing into *any
	// without number preservation. Avoids the fmt.Sprintf allocation for error wrapping
	// and skips the preservingUnmarshal indirection.
	if _, ok := target.(*any); ok && !options.PreserveNumbers {
		if err := json.Unmarshal(stringToBytes(jsonStr), target); err != nil {
			return &JsonsError{
				Op:      "parse",
				Message: fmt.Sprintf("invalid JSON for target type %T: %v", target, err),
				Err:     ErrInvalidJSON,
			}
		}
		return nil
	}

	// Parse with number preservation to maintain original format
	if options.PreserveNumbers {
		// Use numberPreservingDecoder to keep json.Number as-is
		decoder := newNumberPreservingDecoder(true)
		data, err := decoder.DecodeToAny(jsonStr)
		if err != nil {
			return &JsonsError{
				Op:      "parse",
				Message: fmt.Sprintf("invalid JSON for target type %T: %v", target, err),
				Err:     ErrInvalidJSON,
			}
		}

		// For *any type, directly assign the result
		if anyPtr, ok := target.(*any); ok {
			*anyPtr = data
			return nil
		}

		// For other types, use custom encoder/decoder to preserve numbers
		config := PrettyConfig()
		config.PreserveNumbers = true

		encoder := newCustomEncoder(config)
		defer encoder.Close()

		encodedJSON, err := encoder.Encode(data)
		if err != nil {
			return &JsonsError{
				Op:      "parse",
				Message: fmt.Sprintf("failed to encode data for target type %T", target),
				Err:     err,
			}
		}

		// Use number-preserving unmarshal for final conversion
		if err := preservingUnmarshal(stringToBytes(encodedJSON), target, true); err != nil {
			return &JsonsError{
				Op:      "parse",
				Message: fmt.Sprintf("invalid JSON for target type %T: %v", target, err),
				Err:     ErrInvalidJSON,
			}
		}
	} else {
		// Standard parsing without number preservation
		if err := preservingUnmarshal(stringToBytes(jsonStr), target, false); err != nil {
			return &JsonsError{
				Op:      "parse",
				Message: fmt.Sprintf("invalid JSON for target type %T: %v", target, err),
				Err:     ErrInvalidJSON,
			}
		}
	}

	return nil
}

// ParseAny parses a JSON string and returns the result as any.
// This method provides the same behavior as the package-level Parse function.
// Use Parse when you need to unmarshal into a specific target type.
//
// Errors:
//   - ErrProcessorClosed: processor has been closed
//   - ErrInvalidJSON: jsonStr is not valid JSON
//   - ErrSizeLimit: JSON exceeds MaxJSONSize
//
// Example:
//
//	data, err := processor.ParseAny(`{"name": "Alice"}`)
//	if err != nil {
//	    // Handle error
//	}
//	obj := data.(map[string]any)
func (p *Processor) ParseAny(jsonStr string, cfg ...Config) (any, error) {
	if err := p.checkClosed(); err != nil {
		return nil, err
	}

	var data any
	if err := p.Parse(jsonStr, &data, cfg...); err != nil {
		return nil, err
	}
	return data, nil
}

// Valid validates JSON format without parsing the entire structure.
// Returns (true, nil) if valid, (false, error) if invalid.
// The error contains details about what makes the JSON invalid.
//
// Errors:
//   - ErrProcessorClosed: processor has been closed
//   - ErrInvalidJSON: jsonStr is not valid JSON (returned with false)
//   - ErrSizeLimit: JSON exceeds MaxJSONSize
//
// Example:
//
//	valid, err := processor.Valid(`{"name":"Alice"}`)
//	if err != nil {
//	    // Handle validation error
//	}
//	if valid {
//	    // JSON is valid
//	}
func (p *Processor) Valid(jsonStr string, cfg ...Config) (bool, error) {
	if err := p.checkClosed(); err != nil {
		return false, err
	}

	// Prepare options, then validate against them so a caller-supplied Config
	// (MaxJSONSize / FullSecurityScan / etc.) is actually enforced. When no
	// Config is supplied, validateInputForOptions falls back to the processor's
	// own baked-in limits.
	options, err := p.prepareOptions(cfg...)
	if err != nil {
		return false, err
	}
	defer releaseConfig(options)

	if err := p.validateInputForOptions(jsonStr, options); err != nil {
		return false, err
	}

	// Check cache first
	cacheKey := p.createCacheKey("validate", jsonStr, "", options)
	if cached, ok := p.getCachedResult(cacheKey); ok {
		if val, typeOk := cached.(bool); typeOk {
			return val, nil
		}
		// Cache type mismatch — evict corrupted entry
		p.invalidateCachedResult(cacheKey)
	}

	// Valid JSON by attempting to parse
	decoder := newNumberPreservingDecoder(options.PreserveNumbers)
	_, err = decoder.DecodeToAny(jsonStr)

	if err != nil {
		// Return error for invalid JSON
		return false, &JsonsError{
			Op:      "validate",
			Message: fmt.Sprintf("invalid JSON: %v", err),
			Err:     ErrInvalidJSON,
		}
	}

	// Cache result if enabled
	p.setCachedResult(cacheKey, true, options)

	return true, nil
}

// ValidBytes validates JSON format from byte slice (matches encoding/json.Valid signature)
// This method provides compatibility with the standard library's json.Valid function
func (p *Processor) ValidBytes(data []byte) bool {
	jsonStr := string(data)
	valid, err := p.Valid(jsonStr)
	return err == nil && valid
}

// stringToBytes converts string to []byte efficiently
// Using standard conversion for safety and compatibility
// While unsafe.StringData could provide zero-copy conversion,
// we prioritize safety over marginal performance gains
func stringToBytes(s string) []byte {
	return internal.StringToBytes(s)
}

// splitPath splits a path into segments for the segment-based walkers
// (Set's operation path, Delete's dot notation). D-002 (M33): when
// Config.CustomPathParser is installed it replaces the standard splitter
// entirely — its dot/bracket assumptions do not hold for custom syntaxes,
// and the caller-provided scratch slice is not reused (custom parsers
// return their own storage).
func (p *Processor) splitPath(path string, segments []internal.PathSegment) ([]internal.PathSegment, error) {
	if p.config.CustomPathParser != nil {
		return p.config.CustomPathParser.ParsePath(path)
	}

	segments = segments[:0]

	// Direct call to internal package - reduces method call overhead
	if !internal.NeedsPathPreprocessing(path) {
		return internal.SplitPathIntoSegments(path, segments), nil
	}

	sb := p.getStringBuilder()
	defer p.putStringBuilder(sb)

	processedPath := internal.PreprocessPath(path, sb)
	return internal.SplitPathIntoSegments(processedPath, segments), nil
}

// parsePath splits a path into string segments. Non-complex (pure dot
// notation) paths — the only kind Delete's dot-notation walker routes here —
// are split escape-aware with empty parts dropped, so "a\.b" resolves to the
// literal key "a.b" like Get and Set do (D-002: the previous raw
// strings.Split produced ["a\", "b"] and Delete failed with "path not found:
// a\" while Get succeeded). Complex paths keep the historical
// segment.String() rendering for direct callers.
func (p *Processor) parsePath(path string) ([]string, error) {
	if path == "" {
		return []string{}, nil
	}

	segments := p.getPathSegments()
	defer p.putPathSegments(segments)

	var splitErr error
	*segments, splitErr = p.splitPath(path, *segments)
	if splitErr != nil {
		return nil, splitErr
	}

	if p.isComplexPath(path) {
		result := make([]string, len(*segments))
		for i, segment := range *segments {
			result[i] = segment.String()
		}
		return result, nil
	}

	result := make([]string, 0, len(*segments))
	for _, segment := range *segments {
		switch segment.Type {
		case internal.PropertySegment:
			result = append(result, segment.Key)
		case internal.ArrayIndexSegment:
			// Dot-separated numerics ("a.0") parse as index segments but must
			// stay numeric strings here: the dot-notation walker resolves
			// them against []any parents (previously strings.Split produced
			// "0" as a plain part).
			result = append(result, strconv.Itoa(segment.Index))
		default:
			return nil, fmt.Errorf("unexpected segment type %v in dot-notation path %q", segment.Type, path)
		}
	}

	return result, nil
}

func (p *Processor) handlePropertyAccess(data any, property string) propertyAccessResult {
	switch v := data.(type) {
	case map[string]any:
		if val, exists := v[property]; exists {
			return propertyAccessResult{value: val, exists: true}
		}
		return propertyAccessResult{exists: false}

	case map[any]any:
		if val, exists := v[property]; exists {
			return propertyAccessResult{value: val, exists: true}
		}
		return propertyAccessResult{exists: false}

	case []any:
		if index := p.parseArrayIndex(property); index >= 0 && index < len(v) {
			return propertyAccessResult{value: v[index], exists: true}
		}
		return propertyAccessResult{exists: false}

	default:
		if structValue := p.handleStructAccess(data, property); structValue != nil {
			return propertyAccessResult{value: structValue, exists: true}
		}
		return propertyAccessResult{exists: false}
	}
}

// handlePropertyAccessValue returns the value of a property access, or nil if not found.
// Convenience wrapper around handlePropertyAccess for callers that don't need the exists flag.
func (p *Processor) handlePropertyAccessValue(data any, property string) any {
	result := p.handlePropertyAccess(data, property)
	if result.exists {
		return result.value
	}
	return nil
}

// numberPreservingDecoder provides JSON decoding with optimized number format preservation.
// OPTIMIZATION: Two pre-allocated instances avoid heap allocation for the common cases.
// Use decoderNoPreserve (preserveNumbers=false) for standard decoding and
// decoderPreserve (preserveNumbers=true) for number-preserving decoding.
type numberPreservingDecoder struct {
	preserveNumbers bool
}

var (
	// Pre-allocated decoders for zero-allocation hot paths
	decoderNoPreserve = &numberPreservingDecoder{preserveNumbers: false}
	decoderPreserve   = &numberPreservingDecoder{preserveNumbers: true}
)

// newNumberPreservingDecoder returns a decoder for the given number preservation setting.
// Returns pre-allocated instances for the two common cases to avoid heap allocation.
func newNumberPreservingDecoder(preserveNumbers bool) *numberPreservingDecoder {
	if preserveNumbers {
		return decoderPreserve
	}
	return decoderNoPreserve
}

// DecodeToAny decodes JSON string to any type with performance and number preservation
func (d *numberPreservingDecoder) DecodeToAny(jsonStr string) (any, error) {
	if !d.preserveNumbers {
		// Fast path: use standard JSON decoding without number preservation
		var result any
		if err := json.Unmarshal(stringToBytes(jsonStr), &result); err != nil {
			return nil, err
		}
		return result, nil
	}

	// Create a new decoder for each call (json.Decoder cannot be reused with different inputs)
	decoder := json.NewDecoder(strings.NewReader(jsonStr))
	decoder.UseNumber()

	var result any
	if err := decoder.Decode(&result); err != nil {
		return nil, err
	}

	// Convert json.Number to our Number type for encoding/json.UseNumber compatibility
	result = d.convertStdJSONNumbers(result)
	return result, nil
}

// convertStdJSONNumbers converts standard library json.Number to our Number type.
// This preserves the original number representation for UseNumber() compatibility.
func (d *numberPreservingDecoder) convertStdJSONNumbers(value any) any {
	switch v := value.(type) {
	case json.Number:
		return Number(string(v))
	case map[string]any:
		result := make(map[string]any, len(v))
		for key, val := range v {
			result[key] = d.convertStdJSONNumbers(val)
		}
		return result
	case []any:
		result := make([]any, len(v))
		for i, val := range v {
			result[i] = d.convertStdJSONNumbers(val)
		}
		return result
	default:
		return v
	}
}

// convertNumbers recursively converts json.Number to native types (int, float64) when possible,
// falling back to Number type for very large numbers. Used by preservingUnmarshal.
func (d *numberPreservingDecoder) convertNumbers(value any) any {
	switch v := value.(type) {
	case json.Number:
		return d.convertJSONNumber(v)
	case map[string]any:
		// Pre-allocate map with known size for better performance
		result := make(map[string]any, len(v))
		for key, val := range v {
			result[key] = d.convertNumbers(val)
		}
		return result
	case []any:
		// Pre-allocate slice with known size
		result := make([]any, len(v))
		for i, val := range v {
			result[i] = d.convertNumbers(val)
		}
		return result
	default:
		return v
	}
}

// convertJSONNumber converts json.Number with precision handling
// PERFORMANCE: Optimized to minimize allocations and use manual parsing where possible
func (d *numberPreservingDecoder) convertJSONNumber(num json.Number) any {
	numStr := string(num)
	numLen := len(numStr)

	// Ultra-fast path for single digits
	if numLen == 1 {
		c := numStr[0]
		if c >= '0' && c <= '9' {
			return int(c - '0')
		}
	}

	// PERFORMANCE: Single scan to detect number format
	hasDecimal := false
	hasScientific := false
	for i := range numLen {
		c := numStr[i]
		switch c {
		case '.':
			hasDecimal = true
		case 'e', 'E':
			hasScientific = true
		}
	}

	// Fast path for small integers without decimal or scientific notation
	if !hasDecimal && !hasScientific && numLen <= 10 {
		// Try manual parsing for small integers
		negative := false
		start := 0
		if numStr[0] == '-' {
			negative = true
			start = 1
		}

		if numLen-start > 0 && numLen-start <= 10 {
			var result int64
			valid := true
			for i := start; i < numLen; i++ {
				c := numStr[i]
				if c < '0' || c > '9' {
					valid = false
					break
				}
				result = result*10 + int64(c-'0')
			}
			if valid {
				if negative {
					result = -result
				}
				// Check if it fits in int32
				if result >= -2147483648 && result <= 2147483647 {
					return int(result)
				}
				return result
			}
		}
	}

	// Integer parsing with optimized range checking
	if !hasDecimal && !hasScientific {
		if i, err := strconv.ParseInt(numStr, 10, 64); err == nil {
			// Use bit operations for faster range checking
			if i >= -2147483648 && i <= 2147483647 { // int32 range
				return int(i)
			}
			return i
		}

		// Try uint64 for large positive numbers
		if u, err := strconv.ParseUint(numStr, 10, 64); err == nil {
			return u
		}

		// Number too large for standard types, preserve as Number for type safety
		return Number(numStr)
	}

	// Handle "clean" floats (ending with .0)
	if hasDecimal && numLen > 2 && numStr[numLen-2] == '.' && numStr[numLen-1] == '0' {
		intStr := numStr[:numLen-2]
		if i, err := strconv.ParseInt(intStr, 10, 64); err == nil {
			if i >= -2147483648 && i <= 2147483647 {
				return int(i)
			}
			return i
		}
		// If integer conversion fails, try to parse as float
		if f, err := strconv.ParseFloat(numStr, 64); err == nil {
			return f
		}
		// Last resort: return as Number to maintain numeric type identity
		return Number(numStr)
	}

	// Handle decimal numbers with precision checking
	if hasDecimal && !hasScientific {
		if f, err := strconv.ParseFloat(numStr, 64); err == nil {
			// Always return the float64 value to maintain numeric type consistency
			// Precision checking is less important than type consistency
			return f
		}
		// If parsing fails, return as Number to maintain numeric type identity
		return Number(numStr)
	}

	// Handle scientific notation
	if hasScientific {
		if f, err := strconv.ParseFloat(numStr, 64); err == nil {
			return f
		}
	}

	// Fallback: return as Number to maintain numeric type identity
	return Number(numStr)
}

// preservingUnmarshal unmarshals JSON with number preservation
// OPTIMIZED: Uses single-pass decoding with json.Number, then direct type conversion
// to avoid the overhead of marshal/unmarshal cycle for target types that support it.
func preservingUnmarshal(data []byte, v any, preserveNumbers bool) error {
	if !preserveNumbers {
		return json.Unmarshal(data, v)
	}

	// Use json.Number for preservation
	// PERFORMANCE: Use bytes.NewReader to avoid string(data) allocation
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()

	// OPTIMIZED: Try direct decoding for *any type to avoid double conversion
	if anyPtr, ok := v.(*any); ok {
		var temp any
		if err := decoder.Decode(&temp); err != nil {
			return err
		}
		// Convert json.Number to our Number type for consistency
		*anyPtr = newNumberPreservingDecoder(true).convertNumbers(temp)
		return nil
	}

	// For other target types, we still need the conversion step
	// but we optimize by reusing the decoder's buffer
	var temp any
	if err := decoder.Decode(&temp); err != nil {
		return err
	}

	// Convert numbers and then marshal/unmarshal to target type
	converted := newNumberPreservingDecoder(true).convertNumbers(temp)

	// OPTIMIZED: For map[string]any and []any targets, use direct type assertion
	// to avoid the marshal/unmarshal overhead
	switch target := v.(type) {
	case *map[string]any:
		if m, ok := converted.(map[string]any); ok {
			*target = m
			return nil
		}
	case *[]any:
		if s, ok := converted.([]any); ok {
			*target = s
			return nil
		}
	}

	// Fallback: marshal and unmarshal for complex types (structs, custom types)
	convertedBytes, err := json.Marshal(converted)
	if err != nil {
		return err
	}

	return json.Unmarshal(convertedBytes, v)
}
