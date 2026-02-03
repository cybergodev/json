package json

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"sync"

	"github.com/cybergodev/json/internal"
)

// Parse parses a JSON string into the provided target with improved error handling
func (p *Processor) Parse(jsonStr string, target any, opts ...*ProcessorOptions) error {
	if err := p.checkClosed(); err != nil {
		return err
	}

	options, err := p.prepareOptions(opts...)
	if err != nil {
		return err
	}

	if err := p.validateInput(jsonStr); err != nil {
		return err
	}

	if target == nil {
		return &JsonsError{
			Op:      "parse",
			Message: "target cannot be nil, use Parse for any type result",
			Err:     ErrOperationFailed,
		}
	}

	// Parse with number preservation to maintain original format
	if options.PreserveNumbers {
		// Use NumberPreservingDecoder to keep json.Number as-is
		decoder := NewNumberPreservingDecoder(true)
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
		config := NewPrettyConfig()
		config.PreserveNumbers = true

		encoder := NewCustomEncoder(config)
		defer encoder.Close()

		encodedJson, err := encoder.Encode(data)
		if err != nil {
			return &JsonsError{
				Op:      "parse",
				Message: fmt.Sprintf("failed to encode data for target type %T: %v", target, err),
				Err:     ErrOperationFailed,
			}
		}

		// Use number-preserving unmarshal for final conversion
		if err := PreservingUnmarshal([]byte(encodedJson), target, true); err != nil {
			return &JsonsError{
				Op:      "parse",
				Message: fmt.Sprintf("invalid JSON for target type %T: %v", target, err),
				Err:     ErrInvalidJSON,
			}
		}
	} else {
		// Standard parsing without number preservation
		if err := PreservingUnmarshal([]byte(jsonStr), target, false); err != nil {
			return &JsonsError{
				Op:      "parse",
				Message: fmt.Sprintf("invalid JSON for target type %T: %v", target, err),
				Err:     ErrInvalidJSON,
			}
		}
	}

	return nil
}

// Valid validates JSON format without parsing the entire structure
func (p *Processor) Valid(jsonStr string, opts ...*ProcessorOptions) (bool, error) {
	if err := p.checkClosed(); err != nil {
		return false, err
	}

	if err := p.validateInput(jsonStr); err != nil {
		return false, err
	}

	// Prepare options
	options, err := p.prepareOptions(opts...)
	if err != nil {
		return false, err
	}

	// Check cache first
	cacheKey := p.createCacheKey("validate", jsonStr, "", options)
	if cached, ok := p.getCachedResult(cacheKey); ok {
		return cached.(bool), nil
	}

	// Valid JSON by attempting to parse
	decoder := NewNumberPreservingDecoder(options.PreserveNumbers)
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

// FormatPretty formats JSON string with indentation
func (p *Processor) FormatPretty(jsonStr string, opts ...*ProcessorOptions) (string, error) {
	if err := p.checkClosed(); err != nil {
		return "", err
	}

	options, err := p.prepareOptions(opts...)
	if err != nil {
		return "", err
	}

	if err := p.validateInput(jsonStr); err != nil {
		return "", err
	}

	// Check cache first
	cacheKey := p.createCacheKey("pretty", jsonStr, "", options)
	if cached, ok := p.getCachedResult(cacheKey); ok {
		return cached.(string), nil
	}

	// Parse with number preservation to maintain original number types
	decoder := NewNumberPreservingDecoder(options.PreserveNumbers)
	data, err := decoder.DecodeToAny(jsonStr)
	if err != nil {
		return "", &JsonsError{
			Op:      "pretty",
			Message: fmt.Sprintf("failed to parse JSON: %v", err),
			Err:     ErrInvalidJSON,
		}
	}

	// Use custom encoder with pretty formatting to preserve number types
	config := NewPrettyConfig()
	config.PreserveNumbers = options.PreserveNumbers

	encoder := NewCustomEncoder(config)
	defer encoder.Close()

	result, err := encoder.Encode(data)
	if err != nil {
		return "", &JsonsError{
			Op:      "pretty",
			Message: fmt.Sprintf("failed to format JSON: %v", err),
			Err:     ErrOperationFailed,
		}
	}

	// Cache result if enabled
	p.setCachedResult(cacheKey, result, options)

	return result, nil
}

// Compact removes whitespace from JSON string
func (p *Processor) Compact(jsonStr string, opts ...*ProcessorOptions) (string, error) {
	if err := p.checkClosed(); err != nil {
		return "", err
	}

	options, err := p.prepareOptions(opts...)
	if err != nil {
		return "", err
	}

	if err := p.validateInput(jsonStr); err != nil {
		return "", err
	}

	// Check cache first
	cacheKey := p.createCacheKey("compact", jsonStr, "", options)
	if cached, ok := p.getCachedResult(cacheKey); ok {
		return cached.(string), nil
	}

	// Parse with number preservation to maintain original number types
	decoder := NewNumberPreservingDecoder(options.PreserveNumbers)
	data, err := decoder.DecodeToAny(jsonStr)
	if err != nil {
		return "", &JsonsError{
			Op:      "compact",
			Message: fmt.Sprintf("failed to parse JSON: %v", err),
			Err:     ErrInvalidJSON,
		}
	}

	// Use custom encoder with compact formatting to preserve number types
	config := DefaultEncodeConfig()
	config.PreserveNumbers = options.PreserveNumbers

	encoder := NewCustomEncoder(config)
	defer encoder.Close()

	result, err := encoder.Encode(data)
	if err != nil {
		return "", &JsonsError{
			Op:      "compact",
			Message: fmt.Sprintf("failed to compact JSON: %v", err),
			Err:     ErrOperationFailed,
		}
	}

	// Cache result if enabled
	p.setCachedResult(cacheKey, result, options)

	return result, nil
}

// FormatCompact removes whitespace from JSON string (alias for Compact)
func (p *Processor) FormatCompact(jsonStr string, opts ...*ProcessorOptions) (string, error) {
	return p.Compact(jsonStr, opts...)
}

// CompactBuffer appends to dst the JSON-encoded src with insignificant space characters elided.
//
// This method provides compatibility with the encoding/json.Compact function signature,
// with the addition of optional ProcessorOptions for advanced customization.
//
// API Design Note:
//   - Processor.Compact(jsonStr) operates on strings and returns formatted strings
//   - Processor.CompactBuffer(dst, src) operates on buffers for stream processing
//   - This naming convention distinguishes string operations from buffer operations
//   - Both methods support optional ProcessorOptions for consistency
//
// For package-level usage with standard library signature, see json.Compact(dst, src).
func (p *Processor) CompactBuffer(dst *bytes.Buffer, src []byte, opts ...*ProcessorOptions) error {
	compacted, err := p.Compact(string(src), opts...)
	if err != nil {
		return err
	}
	_, err = dst.WriteString(compacted)
	return err
}

// IndentBuffer appends to dst an indented form of the JSON-encoded src.
//
// This method provides compatibility with the encoding/json.Indent function signature,
// with the addition of optional ProcessorOptions for advanced customization.
//
// API Design Note:
//   - Processor.FormatPretty(jsonStr) operates on strings for pretty formatting
//   - Processor.IndentBuffer(dst, src, prefix, indent) operates on buffers for stream processing
//   - This naming convention distinguishes string operations from buffer operations
//   - Both approaches support optional ProcessorOptions for consistency
//
// For package-level usage with standard library signature, see json.Indent(dst, src, prefix, indent).
func (p *Processor) IndentBuffer(dst *bytes.Buffer, src []byte, prefix, indent string, opts ...*ProcessorOptions) error {
	var data any
	if err := p.Unmarshal(src, &data, opts...); err != nil {
		return err
	}
	indented, err := p.MarshalIndent(data, prefix, indent, opts...)
	if err != nil {
		return err
	}
	_, err = dst.Write(indented)
	return err
}

// HTMLEscapeBuffer appends to dst the JSON-encoded src with HTML-safe escaping.
//
// This method provides compatibility with the encoding/json.HTMLEscape function signature,
// with the addition of optional ProcessorOptions for advanced customization.
//
// The function replaces &, <, and > with \u0026, \u003c, and \u003e to avoid certain
// safety problems that can arise when embedding JSON in HTML.
//
// API Design Note:
//   - The "Buffer" suffix distinguishes this buffer operation from potential string operations
//   - This naming is consistent with CompactBuffer and IndentBuffer
//   - Processor methods use descriptive names to avoid ambiguity
//
// For package-level usage with standard library signature, see json.HTMLEscape(dst, src).
func (p *Processor) HTMLEscapeBuffer(dst *bytes.Buffer, src []byte, opts ...*ProcessorOptions) {
	var data any
	if err := p.Unmarshal(src, &data, opts...); err != nil {
		dst.Write(src)
		return
	}

	config := DefaultEncodeConfig()
	config.EscapeHTML = true
	escaped, err := p.EncodeWithConfig(data, config, opts...)
	if err != nil {
		dst.Write(src)
		return
	}

	dst.WriteString(escaped)
}

// NumberPreservingDecoder provides JSON decoding with optimized number format preservation
type NumberPreservingDecoder struct {
	preserveNumbers bool

	// Reusable decoder for performance
	decoder *json.Decoder

	// Buffer pool for string operations
	bufferPool *sync.Pool
}

// NewNumberPreservingDecoder creates a new decoder with performance and number preservation
func NewNumberPreservingDecoder(preserveNumbers bool) *NumberPreservingDecoder {
	return &NumberPreservingDecoder{
		preserveNumbers: preserveNumbers,
		bufferPool: &sync.Pool{
			New: func() any {
				return make([]byte, 0, 1024) // Pre-allocate 1KB buffer
			},
		},
	}
}

// DecodeToAny decodes JSON string to any type with performance and number preservation
func (d *NumberPreservingDecoder) DecodeToAny(jsonStr string) (any, error) {
	if !d.preserveNumbers {
		// Fast path: use standard JSON decoding without number preservation
		var result any
		if err := json.Unmarshal(stringToBytes(jsonStr), &result); err != nil {
			return nil, err
		}
		return result, nil
	}

	// Path: use reusable decoder with number preservation
	if d.decoder == nil {
		d.decoder = json.NewDecoder(strings.NewReader(jsonStr))
		d.decoder.UseNumber()
	} else {
		// Reset decoder with new input
		d.decoder = json.NewDecoder(strings.NewReader(jsonStr))
		d.decoder.UseNumber()
	}

	var result any
	if err := d.decoder.Decode(&result); err != nil {
		return nil, err
	}

	// Convert json.Number to our Number type for compatibility
	result = d.convertStdJSONNumbers(result)
	return result, nil
}

// convertStdJSONNumbers converts standard library json.Number to our Number type
func (d *NumberPreservingDecoder) convertStdJSONNumbers(value any) any {
	switch v := value.(type) {
	case json.Number:
		// Convert standard library json.Number to our Number type
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

// stringToBytes converts string to []byte efficiently
// Using standard conversion for safety and compatibility
// While unsafe.StringData could provide zero-copy conversion,
// we prioritize safety over marginal performance gains
func stringToBytes(s string) []byte {
	return []byte(s)
}

// convertNumbers recursively converts json.Number
func (d *NumberPreservingDecoder) convertNumbers(value any) any {
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
func (d *NumberPreservingDecoder) convertJSONNumber(num json.Number) any {
	numStr := string(num)
	numLen := len(numStr)

	// Ultra-fast path for single digits
	if numLen == 1 {
		if numStr[0] >= '0' && numStr[0] <= '9' {
			return int(numStr[0] - '0')
		}
	}

	// Fast path for small integers without decimal or scientific notation
	if numLen <= 10 && !containsAnyByte(numStr, ".eE") {
		if i, err := strconv.Atoi(numStr); err == nil {
			return i
		}
	}

	// Check for integer format (no decimal point and no scientific notation)
	hasDecimal := strings.Contains(numStr, ".")
	hasScientific := containsAnyByte(numStr, "eE")

	if !hasDecimal && !hasScientific {
		// Integer parsing with optimized range checking
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

		// Number too large for standard types, preserve as string
		return numStr
	}

	// Handle "clean" floats (ending with .0)
	if hasDecimal && strings.HasSuffix(numStr, ".0") {
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
		// Last resort: return as string
		return numStr
	}

	// Handle decimal numbers with precision checking
	if hasDecimal && !hasScientific {
		if f, err := strconv.ParseFloat(numStr, 64); err == nil {
			// Always return the float64 value to maintain numeric type consistency
			// Precision checking is less important than type consistency
			return f
		}
		// If parsing fails, return as string
		return numStr
	}

	// Handle scientific notation
	if hasScientific {
		if f, err := strconv.ParseFloat(numStr, 64); err == nil {
			return f
		}
	}

	// Fallback: return as string
	return numStr
}

// containsAnyByte checks if string contains any of the specified bytes (faster than strings.ContainsAny)
func containsAnyByte(s, chars string) bool {
	for i := 0; i < len(s); i++ {
		for j := 0; j < len(chars); j++ {
			if s[i] == chars[j] {
				return true
			}
		}
	}
	return false
}

// checkFloatPrecision quickly checks if float64 preserves the original string representation
func (d *NumberPreservingDecoder) checkFloatPrecision(f float64, original string) bool {
	// Use buffer from pool for efficient string formatting
	buf := d.bufferPool.Get().([]byte)
	defer d.bufferPool.Put(buf[:0])

	formatted := strconv.AppendFloat(buf, f, 'f', -1, 64)
	return string(formatted) == original
}

// PreservingUnmarshal unmarshals JSON with number preservation
func PreservingUnmarshal(data []byte, v any, preserveNumbers bool) error {
	if !preserveNumbers {
		return json.Unmarshal(data, v)
	}

	// Use json.Number for preservation
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.UseNumber()

	// First decode to any to handle json.Number conversion
	var temp any
	if err := decoder.Decode(&temp); err != nil {
		return err
	}

	// Convert numbers and then marshal/unmarshal to target type
	converted := NewNumberPreservingDecoder(true).convertNumbers(temp)

	// Marshal the converted data and unmarshal to target
	convertedBytes, err := json.Marshal(converted)
	if err != nil {
		return err
	}

	return json.Unmarshal(convertedBytes, v)
}

// SmartNumberConversion provides intelligent number type conversion
func SmartNumberConversion(value any) any {
	switch v := value.(type) {
	case json.Number:
		decoder := NewNumberPreservingDecoder(true)
		return decoder.convertJSONNumber(v)
	case string:
		// Try to parse string as number
		if num := json.Number(v); num.String() == v {
			decoder := NewNumberPreservingDecoder(true)
			return decoder.convertJSONNumber(num)
		}
		return v
	default:
		return v
	}
}

// IsLargeNumber checks if a string represents a number that's too large for standard numeric types
func IsLargeNumber(numStr string) bool {
	// Remove leading/trailing whitespace
	numStr = strings.TrimSpace(numStr)

	// Check if it's a valid number format
	if !isValidNumberString(numStr) {
		return false
	}

	// If it's an integer (no decimal point)
	if !strings.Contains(numStr, ".") && !strings.ContainsAny(numStr, "eE") {
		// Try parsing as int64 and uint64
		_, errInt := strconv.ParseInt(numStr, 10, 64)
		_, errUint := strconv.ParseUint(numStr, 10, 64)
		// If both fail, it's too large
		return errInt != nil && errUint != nil
	}

	return false
}

// isValidNumberString checks if a string represents a valid number
func isValidNumberString(s string) bool {
	if s == "" {
		return false
	}

	// Use json.Number to validate
	num := json.Number(s)
	_, err := num.Float64()
	return err == nil
}

// IsScientificNotation checks if a string represents a number in scientific notation
func IsScientificNotation(s string) bool {
	return strings.ContainsAny(s, "eE")
}

// ConvertFromScientific converts a scientific notation string to regular number format
func ConvertFromScientific(s string) (string, error) {
	if !IsScientificNotation(s) {
		return s, nil
	}

	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return s, err
	}

	// Format without scientific notation
	return FormatNumber(f), nil
}

func (p *Processor) navigateToPath(data any, path string) (any, error) {
	if path == "" || path == "." || path == "/" {
		return data, nil
	}

	if strings.HasPrefix(path, "/") {
		return p.navigateJSONPointer(data, path)
	}

	return p.navigateDotNotation(data, path)
}

func (p *Processor) navigateDotNotation(data any, path string) (any, error) {
	current := data

	segments := p.getPathSegments()
	defer p.putPathSegments(segments)

	segments = p.splitPath(path, segments)

	for i, segment := range segments {
		if p.isDistributedOperationSegment(segment) {
			return p.handleDistributedOperation(current, segments[i:])
		}

		switch segment.TypeString() {
		case "property":
			result := p.handlePropertyAccess(current, segment.Key)
			if !result.Exists {
				return nil, ErrPathNotFound
			}
			current = result.Value

		case "array":
			result := p.handleArrayAccess(current, segment)
			if !result.Exists {
				return nil, ErrPathNotFound
			}
			current = result.Value

		case "slice":
			result := p.handleArraySlice(current, segment)
			if !result.Exists {
				return nil, ErrPathNotFound
			}
			current = result.Value

		case "extract":
			extractResult, err := p.handleExtraction(current, segment)
			if err != nil {
				return nil, err
			}
			current = extractResult

			if i+1 < len(segments) {
				nextSegment := segments[i+1]
				if nextSegment.TypeString() == "array" || nextSegment.TypeString() == "slice" {
					if segment.IsFlat {
						if nextSegment.TypeString() == "slice" {
							result := p.handleArraySlice(current, nextSegment)
							if result.Exists {
								current = result.Value
							}
						} else {
							result := p.handleArrayAccess(current, nextSegment)
							if result.Exists {
								current = result.Value
							}
						}
					} else {
						current = p.handlePostExtractionArrayAccess(current, nextSegment)
					}
					i++
				}
			}

		default:
			return nil, fmt.Errorf("unsupported segment type: %v", segment.TypeString())
		}
	}

	return current, nil
}

func (p *Processor) navigateJSONPointer(data any, path string) (any, error) {
	if path == "/" {
		return data, nil
	}

	pathWithoutSlash := path[1:]
	segmentCount := strings.Count(pathWithoutSlash, "/") + 1
	segments := make([]string, 0, segmentCount)
	segments = strings.Split(pathWithoutSlash, "/")

	current := data

	for _, segment := range segments {
		if segment == "" {
			continue
		}

		if strings.Contains(segment, "~") {
			segment = p.unescapeJSONPointer(segment)
		}

		result := p.handlePropertyAccess(current, segment)
		if !result.Exists {
			return nil, ErrPathNotFound
		}
		current = result.Value
	}

	return current, nil
}

// unescapeJSONPointer unescapes JSON Pointer special characters
func (p *Processor) unescapeJSONPointer(segment string) string {
	var result strings.Builder
	result.Grow(len(segment))

	i := 0
	for i < len(segment) {
		if i+1 < len(segment) && segment[i] == '~' {
			switch segment[i+1] {
			case '1':
				result.WriteByte('/')
				i += 2
				continue
			case '0':
				result.WriteByte('~')
				i += 2
				continue
			}
		}
		// Copy normal characters
		result.WriteByte(segment[i])
		i++
	}

	return result.String()
}

func (p *Processor) handlePropertyAccess(data any, property string) PropertyAccessResult {
	switch v := data.(type) {
	case map[string]any:
		if val, exists := v[property]; exists {
			return PropertyAccessResult{Value: val, Exists: true}
		}
		return PropertyAccessResult{Exists: false}

	case map[any]any:
		if val, exists := v[property]; exists {
			return PropertyAccessResult{Value: val, Exists: true}
		}
		return PropertyAccessResult{Exists: false}

	case []any:
		if index := p.parseArrayIndex(property); index >= 0 && index < len(v) {
			return PropertyAccessResult{Value: v[index], Exists: true}
		}
		return PropertyAccessResult{Exists: false}

	default:
		if structValue := p.handleStructAccess(data, property); structValue != nil {
			return PropertyAccessResult{Value: structValue, Exists: true}
		}
		return PropertyAccessResult{Exists: false}
	}
}

func (p *Processor) handlePropertyAccessValue(data any, property string) any {
	result := p.handlePropertyAccess(data, property)
	if result.Exists {
		return result.Value
	}
	return nil
}

func (p *Processor) parseArrayIndexFromPath(property string) int {
	if index, ok := internal.ParseArrayIndex(property); ok {
		return index
	}
	return -1
}

func (p *Processor) splitPath(path string, segments []PathSegment) []PathSegment {
	segments = segments[:0]

	if !p.needsPathPreprocessing(path) {
		return p.splitPathIntoSegments(path, segments)
	}

	sb := p.getStringBuilder()
	defer p.putStringBuilder(sb)

	processedPath := p.preprocessPath(path, sb)

	return p.splitPathIntoSegments(processedPath, segments)
}

func (p *Processor) needsPathPreprocessing(path string) bool {
	for i := 0; i < len(path); i++ {
		c := path[i]
		if c == '[' || c == '{' {
			return true
		}
	}
	return false
}

func (p *Processor) preprocessPath(path string, sb *strings.Builder) string {
	sb.Reset()

	runes := []rune(path)
	for i, r := range runes {
		switch r {
		case '[':
			if i > 0 && p.needsDotBefore(runes[i-1]) {
				sb.WriteRune('.')
			}
			sb.WriteRune(r)
		case '{':
			if i > 0 && p.needsDotBefore(runes[i-1]) {
				sb.WriteRune('.')
			}
			sb.WriteRune(r)
		default:
			sb.WriteRune(r)
		}
	}

	return sb.String()
}

func (p *Processor) needsDotBefore(prevChar rune) bool {
	return (prevChar >= 'a' && prevChar <= 'z') ||
		(prevChar >= 'A' && prevChar <= 'Z') ||
		(prevChar >= '0' && prevChar <= '9') ||
		prevChar == '_' || prevChar == ']' || prevChar == '}'
}

func (p *Processor) splitPathIntoSegments(path string, segments []PathSegment) []PathSegment {
	parts := strings.Split(path, ".")

	for _, part := range parts {
		if part == "" {
			continue
		}
		segments = p.parsePathSegment(part, segments)
	}

	return segments
}

func (p *Processor) parsePathSegment(part string, segments []PathSegment) []PathSegment {
	if strings.Contains(part, "[") {
		return p.parseArraySegment(part, segments)
	} else if strings.Contains(part, "{") {
		return p.parseExtractionSegment(part, segments)
	} else {
		if index, err := strconv.Atoi(part); err == nil {
			segments = append(segments, PathSegment{
				Type:  internal.ArrayIndexSegment,
				Index: index,
			})
			return segments
		}

		segments = append(segments, PathSegment{
			Key:  part,
			Type: internal.PropertySegment,
		})
		return segments
	}
}

func (p *Processor) isComplexPath(path string) bool {
	complexPatterns := []string{
		"{", "}",
		"[", "]",
		":",
	}

	for _, pattern := range complexPatterns {
		if strings.Contains(path, pattern) {
			return true
		}
	}

	return false
}

func (p *Processor) hasComplexSegments(segments []PathSegment) bool {
	for _, segment := range segments {
		switch segment.TypeString() {
		case "slice", "extract":
			return true
		}
	}
	return false
}

func (p *Processor) parsePath(path string) ([]string, error) {
	if path == "" {
		return []string{}, nil
	}

	if !p.isComplexPath(path) {
		return strings.Split(path, "."), nil
	}

	segments := p.getPathSegments()
	defer p.putPathSegments(segments)

	segments = p.splitPath(path, segments)

	result := make([]string, len(segments))
	for i, segment := range segments {
		result[i] = segment.String()
	}

	return result, nil
}

func (p *Processor) isDistributedOperationPath(path string) bool {
	distributedPatterns := []string{
		"}[",
		"}:",
		"}{",
	}

	for _, pattern := range distributedPatterns {
		if strings.Contains(path, pattern) {
			return true
		}
	}

	if strings.Contains(path, "{flat:") {
		return true
	}

	return false
}

func (p *Processor) isDistributedOperationSegment(segment PathSegment) bool {
	return segment.Key != ""
}

func (p *Processor) handleDistributedOperation(data any, segments []PathSegment) (any, error) {
	return p.getValueWithDistributedOperation(data, p.reconstructPath(segments))
}

func (p *Processor) reconstructPath(segments []PathSegment) string {
	if len(segments) == 0 {
		return ""
	}

	sb := p.getStringBuilder()
	defer p.putStringBuilder(sb)

	for i, segment := range segments {
		if i > 0 {
			sb.WriteRune('.')
		}
		sb.WriteString(segment.String())
	}

	return sb.String()
}

// parseArraySegment parses array access segments like [0], [1:3], etc.
func (p *Processor) parseArraySegment(part string, segments []PathSegment) []PathSegment {
	openBracket := strings.Index(part, "[")
	closeBracket := strings.LastIndex(part, "]")

	if openBracket == -1 || closeBracket == -1 || closeBracket <= openBracket {
		segments = append(segments, PathSegment{
			Key:  part,
			Type: internal.PropertySegment,
		})
		return segments
	}

	if openBracket > 0 {
		propertyName := part[:openBracket]
		segments = append(segments, PathSegment{
			Key:  propertyName,
			Type: internal.PropertySegment,
		})
	}

	bracketContent := part[openBracket+1 : closeBracket]

	if strings.Contains(bracketContent, ":") {
		segment := PathSegment{
			Type: internal.ArraySliceSegment,
		}

		parts := strings.Split(bracketContent, ":")
		if len(parts) >= 2 {
			if parts[0] != "" {
				if start, err := strconv.Atoi(parts[0]); err == nil {
					segment.Start = &start
				}
			}

			if parts[1] != "" {
				if end, err := strconv.Atoi(parts[1]); err == nil {
					segment.End = &end
				}
			}

			if len(parts) == 3 && parts[2] != "" {
				if step, err := strconv.Atoi(parts[2]); err == nil {
					segment.Step = &step
				}
			}
		}

		segments = append(segments, segment)
	} else {
		segment := PathSegment{
			Type: internal.ArrayIndexSegment,
		}

		if index, err := strconv.Atoi(bracketContent); err == nil {
			segment.Index = index
		}

		segments = append(segments, segment)
	}

	if closeBracket+1 < len(part) {
		remaining := part[closeBracket+1:]
		if remaining != "" {
			segments = p.parsePathSegment(remaining, segments)
		}
	}

	return segments
}

// parseExtractionSegment parses extraction segments like {key}, {flat:key}, etc.
func (p *Processor) parseExtractionSegment(part string, segments []PathSegment) []PathSegment {
	openBrace := strings.Index(part, "{")
	closeBrace := strings.LastIndex(part, "}")

	if openBrace == -1 || closeBrace == -1 || closeBrace <= openBrace {
		segments = append(segments, PathSegment{
			Key:  part,
			Type: internal.PropertySegment,
		})
		return segments
	}

	if openBrace > 0 {
		propertyName := part[:openBrace]
		segments = append(segments, PathSegment{
			Key:  propertyName,
			Type: internal.PropertySegment,
		})
	}

	braceContent := part[openBrace+1 : closeBrace]

	extractSegment := PathSegment{
		Type: internal.ExtractSegment,
	}

	if strings.HasPrefix(braceContent, "flat:") {
		extractSegment.Key = braceContent[5:]
		extractSegment.IsFlat = true
	} else {
		extractSegment.Key = braceContent
	}

	segments = append(segments, extractSegment)

	if closeBrace+1 < len(part) {
		remaining := part[closeBrace+1:]
		if remaining != "" {
			segments = p.parsePathSegment(remaining, segments)
		}
	}

	return segments
}

func (p *Processor) isArrayType(data any) bool {
	switch data.(type) {
	case []any:
		return true
	default:
		return false
	}
}

func (p *Processor) isObjectType(data any) bool {
	switch data.(type) {
	case map[string]any, map[any]any:
		return true
	default:
		return false
	}
}

func (p *Processor) isMapType(data any) bool {
	switch data.(type) {
	case map[string]any, map[any]any:
		return true
	default:
		return false
	}
}

func (p *Processor) isSliceType(data any) bool {
	if data == nil {
		return false
	}

	v := reflect.ValueOf(data)
	return v.Kind() == reflect.Slice
}

func (p *Processor) isPrimitiveType(data any) bool {
	switch data.(type) {
	case string, int, int8, int16, int32, int64,
		uint, uint8, uint16, uint32, uint64,
		float32, float64, bool:
		return true
	default:
		return false
	}
}

// isNilOrEmpty checks if a value is nil or empty
func (p *Processor) isNilOrEmpty(data any) bool {
	if data == nil {
		return true
	}

	switch v := data.(type) {
	case string:
		return v == ""
	case []any:
		return len(v) == 0
	case map[string]any:
		return len(v) == 0
	case map[any]any:
		return len(v) == 0
	default:
		return false
	}
}

func (p *Processor) deepCopyData(data any) any {
	switch v := data.(type) {
	case map[string]any:
		return p.deepCopyStringMap(v)
	case map[any]any:
		return p.deepCopyAnyMap(v)
	case []any:
		return p.deepCopyArray(v)
	case string, int, int8, int16, int32, int64,
		uint, uint8, uint16, uint32, uint64,
		float32, float64, bool:
		return v
	default:
		return p.deepCopyReflection(data)
	}
}

func (p *Processor) deepCopyStringMap(data map[string]any) map[string]any {
	result := make(map[string]any)
	for key, value := range data {
		result[key] = p.deepCopyData(value)
	}
	return result
}

func (p *Processor) deepCopyAnyMap(data map[any]any) map[any]any {
	result := make(map[any]any)
	for key, value := range data {
		result[key] = p.deepCopyData(value)
	}
	return result
}

func (p *Processor) deepCopyArray(data []any) []any {
	result := make([]any, len(data))
	for i, value := range data {
		result[i] = p.deepCopyData(value)
	}
	return result
}

func (p *Processor) deepCopyReflection(data any) any {
	if data == nil {
		return nil
	}

	v := reflect.ValueOf(data)
	switch v.Kind() {
	case reflect.Ptr:
		if v.IsNil() {
			return nil
		}
		newPtr := reflect.New(v.Elem().Type())
		newPtr.Elem().Set(reflect.ValueOf(p.deepCopyReflection(v.Elem().Interface())))
		return newPtr.Interface()
	case reflect.Struct:
		newStruct := reflect.New(v.Type()).Elem()
		for i := 0; i < v.NumField(); i++ {
			if v.Field(i).CanInterface() {
				newStruct.Field(i).Set(reflect.ValueOf(p.deepCopyReflection(v.Field(i).Interface())))
			}
		}
		return newStruct.Interface()
	default:
		return data
	}
}

func (p *Processor) escapeJSONPointer(segment string) string {
	// JSON Pointer escaping: ~ becomes ~0, / becomes ~1
	segment = strings.ReplaceAll(segment, "~", "~0")
	segment = strings.ReplaceAll(segment, "/", "~1")
	return segment
}

func (p *Processor) normalizePathSeparators(path string) string {
	for strings.Contains(path, "..") {
		path = strings.ReplaceAll(path, "..", ".")
	}

	// Remove leading and trailing dots
	path = strings.Trim(path, ".")

	return path
}

func (p *Processor) splitPathSegments(path string) []string {
	if path == "" {
		return []string{}
	}

	// Handle JSON Pointer format
	if strings.HasPrefix(path, "/") {
		pathWithoutSlash := path[1:]
		if pathWithoutSlash == "" {
			return []string{}
		}
		return strings.Split(pathWithoutSlash, "/")
	}

	// Handle dot notation
	return strings.Split(path, ".")
}

func (p *Processor) joinPathSegments(segments []string, useJSONPointer bool) string {
	if len(segments) == 0 {
		return ""
	}

	if useJSONPointer {
		return "/" + strings.Join(segments, "/")
	}

	return strings.Join(segments, ".")
}

func (p *Processor) isValidPropertyName(name string) bool {
	return name != "" && !strings.ContainsAny(name, ".[]{}()")
}

func (p *Processor) isValidArrayIndex(index string) bool {
	if index == "" {
		return false
	}

	index = strings.TrimPrefix(index, "-")

	_, err := strconv.Atoi(index)
	return err == nil
}

func (p *Processor) isValidSliceRange(rangeStr string) bool {
	parts := strings.Split(rangeStr, ":")
	if len(parts) < 2 || len(parts) > 3 {
		return false
	}

	// Check each part
	for _, part := range parts {
		if part != "" {
			if _, err := strconv.Atoi(part); err != nil {
				return false
			}
		}
	}

	return true
}

func (p *Processor) wrapError(err error, context string) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", context, err)
}

// createPathError creates a path-specific error
func (p *Processor) createPathError(path string, operation string, err error) error {
	return fmt.Errorf("failed to %s at path '%s': %w", operation, path, err)
}
