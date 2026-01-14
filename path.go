package json

import (
	"encoding/json"
	"fmt"
	"reflect"
	"regexp"
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
	config := NewCompactConfig()
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

// Package-level compiled regex patterns (shared across all pathParser instances for performance)
var (
	simplePropertyPattern = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)
	arrayIndexPattern     = regexp.MustCompile(`^\[(-?\d+)]$`)
	slicePattern          = regexp.MustCompile(`^\[(-?\d*):(-?\d*)(?::(-?\d+))?]$`)
	extractPattern        = regexp.MustCompile(`^\{([^}]+)}$`)
)

// PathSegmentInfo contains detailed information about a parsed path segment
// This is an internal type used by pathParser for navigation operations
type PathSegmentInfo struct {
	Type    string
	Value   string
	Key     string
	Index   int
	Start   *int
	End     *int
	Step    *int
	Extract string
	IsFlat  bool
}

// pathParser provides path parsing functionality
type pathParser struct {
	// String builder pool for efficient string operations
	stringBuilderPool *stringBuilderPool
}

// NewPathParser creates a new path parser instance
func NewPathParser() *pathParser {
	return &pathParser{
		stringBuilderPool: newStringBuilderPool(),
	}
}

// ParsePath parses a path string into segments
func (pp *pathParser) ParsePath(path string) ([]PathSegmentInfo, error) {
	if path == "" {
		return []PathSegmentInfo{}, nil
	}

	// Handle different path formats
	if strings.HasPrefix(path, "/") {
		return pp.parseJSONPointer(path)
	}

	return pp.parseDotNotation(path)
}

// ValidatePath validates a path string for correctness
func (pp *pathParser) ValidatePath(path string) error {
	if path == "" {
		return nil // Empty path is valid (root access)
	}

	// Check for unmatched brackets and braces
	if err := pp.validateBrackets(path); err != nil {
		return err
	}

	// Check for invalid characters or patterns
	if err := pp.validatePathSyntax(path); err != nil {
		return err
	}

	// Try to parse the path to ensure it's valid
	_, err := pp.ParsePath(path)
	return err
}

// SplitPathIntoSegments splits a path into segments
func (pp *pathParser) SplitPathIntoSegments(path string) []PathSegmentInfo {
	segments, err := pp.ParsePath(path)
	if err != nil {
		// Return empty slice on error
		return []PathSegmentInfo{}
	}
	return segments
}

// PreprocessPath preprocesses a path string for parsing
func (pp *pathParser) PreprocessPath(path string) string {
	if path == "" || strings.HasPrefix(path, "/") {
		return path
	}

	sb := pp.stringBuilderPool.Get()
	defer pp.stringBuilderPool.Put(sb)

	sb.Reset()
	sb.Grow(len(path) + 10) // Pre-allocate with some extra space

	for i, char := range path {
		// Check if we need to add a dot before [ or {
		if (char == '[' || char == '{') && i > 0 {
			prevChar := rune(path[i-1])
			if prevChar != '.' && pp.needsDotBefore(prevChar) {
				// Don't add dot for simple property+array access like "hobbies[0]"
				// Only add dot for complex cases like "a.b[0]" -> "a.b.[0]"
				if char == '[' && pp.isSimplePropertyArrayAccess(path, i) {
					// Skip adding dot for simple cases
				} else {
					sb.WriteRune('.')
				}
			}
		}
		sb.WriteRune(char)
	}

	return sb.String()
}

// parseDotNotation parses dot notation paths like "user.name" or "users[0].name"
func (pp *pathParser) parseDotNotation(path string) ([]PathSegmentInfo, error) {
	var segments []PathSegmentInfo

	// Preprocess the path
	preprocessedPath := pp.PreprocessPath(path)

	// Smart split that respects extraction and array operation boundaries
	parts := pp.smartSplitPath(preprocessedPath)

	for _, part := range parts {
		if part == "" {
			continue
		}

		segmentInfo, err := pp.parsePathSegment(part)
		if err != nil {
			return nil, fmt.Errorf("failed to parse segment '%s': %w", part, err)
		}

		segments = append(segments, segmentInfo...)
	}

	return segments, nil
}

// parseJSONPointer parses JSON Pointer format paths
func (pp *pathParser) parseJSONPointer(path string) ([]PathSegmentInfo, error) {
	if path == "/" {
		return []PathSegmentInfo{}, nil
	}

	// Remove leading slash and split
	path = strings.TrimPrefix(path, "/")
	parts := strings.Split(path, "/")

	var segments []PathSegmentInfo
	for _, part := range parts {
		// Unescape JSON Pointer special characters
		part = strings.ReplaceAll(part, "~1", "/")
		part = strings.ReplaceAll(part, "~0", "~")

		// Check if it's a numeric index
		if pp.isNumericIndex(part) {
			index, err := strconv.Atoi(part)
			if err != nil {
				return nil, fmt.Errorf("invalid numeric index '%s': %w", part, err)
			}
			segments = append(segments, PathSegmentInfo{
				Type:  "array",
				Value: fmt.Sprintf("[%d]", index),
				Index: index,
			})
		} else {
			// Property access
			segments = append(segments, PathSegmentInfo{
				Type:  "property",
				Value: part,
				Key:   part,
			})
		}
	}

	return segments, nil
}

// smartSplitPath splits path by dots while respecting extraction and array operation boundaries
func (pp *pathParser) smartSplitPath(path string) []string {
	// Pre-allocate with estimated capacity to reduce allocations
	estimatedParts := strings.Count(path, ".") + 1
	parts := make([]string, 0, estimatedParts)

	var current strings.Builder
	current.Grow(len(path) / estimatedParts) // Pre-allocate based on average part size

	var braceDepth int
	var bracketDepth int

	for _, char := range path {
		switch char {
		case '{':
			braceDepth++
			current.WriteRune(char)
		case '}':
			braceDepth--
			current.WriteRune(char)
		case '[':
			bracketDepth++
			current.WriteRune(char)
		case ']':
			bracketDepth--
			current.WriteRune(char)
		case '.':
			// Only split on dots when we're not inside braces or brackets
			if braceDepth == 0 && bracketDepth == 0 {
				if current.Len() > 0 {
					parts = append(parts, current.String())
					current.Reset()
				}
			} else {
				current.WriteRune(char)
			}
		default:
			current.WriteRune(char)
		}
	}

	// Add the last part
	if current.Len() > 0 {
		parts = append(parts, current.String())
	}

	return parts
}

// parsePathSegment parses a single path segment
func (pp *pathParser) parsePathSegment(part string) ([]PathSegmentInfo, error) {
	if part == "" {
		return []PathSegmentInfo{}, nil
	}

	// Check for complex segments that may contain mixed syntax
	if pp.isComplexSegment(part) {
		return pp.parseComplexSegment(part)
	}

	// Simple segment parsing
	segment, err := pp.parseSimpleSegment(part)
	if err != nil {
		return nil, err
	}

	return []PathSegmentInfo{segment}, nil
}

// parseSimpleSegment parses a simple path segment
func (pp *pathParser) parseSimpleSegment(part string) (PathSegmentInfo, error) {
	// Check for extraction syntax first
	if matches := extractPattern.FindStringSubmatch(part); matches != nil {
		return PathSegmentInfo{
			Type:    "extract",
			Value:   part,
			Extract: matches[1],
		}, nil
	}

	// Check for array slice syntax
	if matches := slicePattern.FindStringSubmatch(part); matches != nil {
		segment := PathSegmentInfo{
			Type:  "slice",
			Value: part,
		}

		// Parse start
		if matches[1] != "" {
			start, err := strconv.Atoi(matches[1])
			if err != nil {
				return PathSegmentInfo{}, fmt.Errorf("invalid slice start: %s", matches[1])
			}
			segment.Start = &start
		}

		// Parse end
		if matches[2] != "" {
			end, err := strconv.Atoi(matches[2])
			if err != nil {
				return PathSegmentInfo{}, fmt.Errorf("invalid slice end: %s", matches[2])
			}
			segment.End = &end
		}

		// Parse step
		if matches[3] != "" {
			step, err := strconv.Atoi(matches[3])
			if err != nil {
				return PathSegmentInfo{}, fmt.Errorf("invalid slice step: %s", matches[3])
			}
			// Step cannot be zero
			if step == 0 {
				return PathSegmentInfo{}, fmt.Errorf("slice step cannot be zero")
			}
			segment.Step = &step
		}

		return segment, nil
	}

	// Check for array index syntax
	if matches := arrayIndexPattern.FindStringSubmatch(part); matches != nil {
		index, err := strconv.Atoi(matches[1])
		if err != nil {
			return PathSegmentInfo{}, fmt.Errorf("invalid array index: %s", matches[1])
		}

		return PathSegmentInfo{
			Type:  "array",
			Value: part,
			Index: index,
		}, nil
	}

	// Check for property with array/slice access (e.g., "users[0]" or "items[1:3]")
	if bracketStart := strings.IndexByte(part, '['); bracketStart >= 0 {
		if bracketEnd := strings.IndexByte(part[bracketStart:], ']'); bracketEnd >= 0 {
			property := part[:bracketStart]
			bracketContent := part[bracketStart+1 : bracketStart+bracketEnd]

			// Check if it's slice syntax (contains colon)
			if strings.Contains(bracketContent, ":") {
				// Parse as slice
				segment := PathSegmentInfo{
					Type:  "slice",
					Value: part,
					Key:   property,
				}

				// Use regex to parse slice parameters
				if matches := slicePattern.FindStringSubmatch(part[bracketStart:]); matches != nil {
					// Parse start
					if matches[1] != "" {
						start, err := strconv.Atoi(matches[1])
						if err != nil {
							return PathSegmentInfo{}, fmt.Errorf("invalid slice start: %s", matches[1])
						}
						segment.Start = &start
					}

					// Parse end
					if matches[2] != "" {
						end, err := strconv.Atoi(matches[2])
						if err != nil {
							return PathSegmentInfo{}, fmt.Errorf("invalid slice end: %s", matches[2])
						}
						segment.End = &end
					}

					// Parse step
					if matches[3] != "" {
						step, err := strconv.Atoi(matches[3])
						if err != nil {
							return PathSegmentInfo{}, fmt.Errorf("invalid slice step: %s", matches[3])
						}
						// Step cannot be zero
						if step == 0 {
							return PathSegmentInfo{}, fmt.Errorf("slice step cannot be zero")
						}
						segment.Step = &step
					}

					return segment, nil
				} else {
					return PathSegmentInfo{}, fmt.Errorf("invalid slice syntax in '%s'", part)
				}
			} else {
				// Parse as simple array index
				index, err := strconv.Atoi(bracketContent)
				if err != nil {
					return PathSegmentInfo{}, fmt.Errorf("invalid array index in '%s': %s", part, bracketContent)
				}

				return PathSegmentInfo{
					Type:  "array",
					Value: part,
					Key:   property,
					Index: index,
				}, nil
			}
		}
	}

	// Simple property access
	return PathSegmentInfo{
		Type:  "property",
		Value: part,
		Key:   part,
	}, nil
}

// parseComplexSegment parses complex segments that may contain mixed syntax
func (pp *pathParser) parseComplexSegment(part string) ([]PathSegmentInfo, error) {
	// For now, delegate to the internal parser for complex segments
	internalSegments, err := internal.ParseComplexSegment(part)
	if err != nil {
		return nil, err
	}

	// Convert internal segments to PathSegmentInfo
	var segments []PathSegmentInfo
	for _, seg := range internalSegments {
		segmentInfo := PathSegmentInfo{
			Type:    seg.TypeString(),
			Value:   seg.String(),
			Key:     seg.Key,
			Index:   seg.Index,
			Extract: seg.Key,
			IsFlat:  seg.IsFlat,
		}

		// Handle slice parameters
		if seg.Start != nil {
			segmentInfo.Start = seg.Start
		}
		if seg.End != nil {
			segmentInfo.End = seg.End
		}
		if seg.Step != nil {
			segmentInfo.Step = seg.Step
		}

		segments = append(segments, segmentInfo)
	}

	return segments, nil
}

// isComplexSegment checks if a segment contains complex syntax
func (pp *pathParser) isComplexSegment(part string) bool {
	// Check for extraction followed by array operations
	if strings.Contains(part, "{") && (strings.Contains(part, "[") || strings.Contains(part, ":")) {
		return true
	}

	// Check for multiple operations in one segment
	braceCount := strings.Count(part, "{")
	bracketCount := strings.Count(part, "[")

	return braceCount > 1 || bracketCount > 1 || (braceCount > 0 && bracketCount > 0)
}

// validateBrackets validates that brackets and braces are properly matched
func (pp *pathParser) validateBrackets(path string) error {
	var braceDepth, bracketDepth int

	for i, char := range path {
		switch char {
		case '{':
			braceDepth++
		case '}':
			braceDepth--
			if braceDepth < 0 {
				return fmt.Errorf("unmatched closing brace at position %d", i)
			}
		case '[':
			bracketDepth++
		case ']':
			bracketDepth--
			if bracketDepth < 0 {
				return fmt.Errorf("unmatched closing bracket at position %d", i)
			}
		}
	}

	if braceDepth > 0 {
		return fmt.Errorf("unmatched opening brace")
	}
	if bracketDepth > 0 {
		return fmt.Errorf("unmatched opening bracket")
	}

	return nil
}

// validatePathSyntax validates path syntax for common errors
func (pp *pathParser) validatePathSyntax(path string) error {
	// Check for empty extraction
	if strings.Contains(path, "{}") {
		return fmt.Errorf("empty extraction syntax not allowed")
	}

	// Check for empty array access
	if strings.Contains(path, "[]") {
		return fmt.Errorf("empty array access not allowed")
	}

	// Check for invalid characters in property names
	// This is a basic check - more sophisticated validation could be added
	if strings.Contains(path, "..") {
		return fmt.Errorf("consecutive dots not allowed")
	}

	return nil
}

// needsDotBefore checks if a dot should be added before a character
func (pp *pathParser) needsDotBefore(prevChar rune) bool {
	// Add dot before [ or { if the previous character is alphanumeric or underscore
	return (prevChar >= 'a' && prevChar <= 'z') ||
		(prevChar >= 'A' && prevChar <= 'Z') ||
		(prevChar >= '0' && prevChar <= '9') ||
		prevChar == '_' || prevChar == ']' || prevChar == '}'
}

// isSimplePropertyArrayAccess checks if this is a simple property+array access pattern
func (pp *pathParser) isSimplePropertyArrayAccess(path string, bracketIndex int) bool {
	// Check if this looks like "property[index]" without any dots before
	if bracketIndex == 0 {
		return false
	}

	// Find the start of the current property name
	start := 0
	for i := bracketIndex - 1; i >= 0; i-- {
		if path[i] == '.' {
			start = i + 1
			break
		}
	}

	// Extract the property name
	propertyName := path[start:bracketIndex]

	// Check if it's a simple property name (no dots, no complex syntax)
	if strings.Contains(propertyName, ".") || strings.Contains(propertyName, "{") || strings.Contains(propertyName, "[") {
		return false
	}

	// Check if the property name is a valid identifier
	if len(propertyName) == 0 {
		return false
	}

	// Simple heuristic: if there's no dot immediately before the property name,
	// and the property name is a simple identifier, treat it as simple access
	return start == 0 || (start > 0 && path[start-1] == '.')
}

// isNumericIndex checks if a string represents a numeric index
func (pp *pathParser) isNumericIndex(s string) bool {
	if s == "" {
		return false
	}

	// Handle negative indices
	start := 0
	if s[0] == '-' {
		if len(s) == 1 {
			return false
		}
		start = 1
	}

	for i := start; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}

	return true
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
// In modern Go versions, the compiler optimizes this conversion
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

// NumberPreservingValue wraps a value with its original string representation
type NumberPreservingValue struct {
	Value    any    `json:"value"`
	Original string `json:"original,omitempty"`
}

// String returns the string representation, preferring original if available
func (npv NumberPreservingValue) String() string {
	if npv.Original != "" {
		return npv.Original
	}
	return fmt.Sprintf("%v", npv.Value)
}

// IsNumber checks if the value represents a number
func (npv NumberPreservingValue) IsNumber() bool {
	switch npv.Value.(type) {
	case int, int8, int16, int32, int64:
		return true
	case uint, uint8, uint16, uint32, uint64:
		return true
	case float32, float64:
		return true
	case json.Number:
		return true
	default:
		return false
	}
}

// ToInt converts the value to int if possible
func (npv NumberPreservingValue) ToInt() (int, error) {
	switch v := npv.Value.(type) {
	case int:
		return v, nil
	case int64:
		return int(v), nil
	case float64:
		return int(v), nil
	case json.Number:
		if i, err := v.Int64(); err == nil {
			return int(i), nil
		}
		if f, err := v.Float64(); err == nil {
			return int(f), nil
		}
		return 0, fmt.Errorf("cannot convert %s to int", string(v))
	default:
		return 0, fmt.Errorf("value is not a number: %T", v)
	}
}

// ToFloat64 converts the value to float64 if possible
func (npv NumberPreservingValue) ToFloat64() (float64, error) {
	switch v := npv.Value.(type) {
	case int:
		return float64(v), nil
	case int64:
		return float64(v), nil
	case float64:
		return v, nil
	case json.Number:
		return v.Float64()
	default:
		return 0, fmt.Errorf("value is not a number: %T", v)
	}
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

// Note: FormatNumber, SafeConvertToInt64, and SafeConvertToUint64 are now in type_conversion.go

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

// navigator implements the Navigator interface
type navigator struct {
	pathParser *pathParser
	utils      *processorUtils
}

// NewNavigator creates a new navigator instance
func NewNavigator(pathParser *pathParser, utils *processorUtils) *navigator {
	return &navigator{
		pathParser: pathParser,
		utils:      utils,
	}
}

// NavigateToPath navigates to a specific path in the data
func (n *navigator) NavigateToPath(data any, segments []PathSegmentInfo) (any, error) {
	if len(segments) == 0 {
		return data, nil
	}

	current := data
	for i, segment := range segments {
		result, err := n.NavigateToSegment(current, segment)
		if err != nil {
			return nil, fmt.Errorf("failed to navigate segment %d (%s): %w", i, segment.Value, err)
		}

		if !result.Exists {
			// Path not found - return error as documented
			return nil, ErrPathNotFound
		}

		current = result.Value
	}

	return current, nil
}

// NavigateToSegment navigates to a single segment
func (n *navigator) NavigateToSegment(data any, segment PathSegmentInfo) (PropertyAccessResult, error) {
	switch segment.Type {
	case "property":
		result := n.HandlePropertyAccess(data, segment.Key)
		return PropertyAccessResult{Value: result.Value, Exists: result.Exists, Path: segment.Key}, nil
	case "array":
		return n.handleArrayAccess(data, segment)
	case "slice":
		return n.handleArraySlice(data, segment)
	case "extract":
		return n.handleExtraction(data, segment)
	default:
		return PropertyAccessResult{}, fmt.Errorf("unsupported segment type: %s", segment.Type)
	}
}

// HandlePropertyAccess handles property access on objects
func (n *navigator) HandlePropertyAccess(data any, property string) PropertyAccessResult {
	switch v := data.(type) {
	case map[string]any:
		value, exists := v[property]
		if !exists {
			if arr, ok := tryConvertToArray(v); ok {
				if index := n.parseArrayIndex(property); index != -2 {
					// Handle negative indices
					if index < 0 {
						index = len(arr) + index
					}
					// Check bounds after negative index conversion
					if index >= 0 && index < len(arr) {
						return PropertyAccessResult{Value: arr[index], Exists: true, Path: property}
					}
				}
			}
		}
		return PropertyAccessResult{Value: value, Exists: exists, Path: property}

	case map[any]any:
		value, exists := v[property]
		return PropertyAccessResult{Value: value, Exists: exists, Path: property}

	case []any:
		// Handle numeric property access on arrays (like array.5)
		if index := n.parseArrayIndex(property); index != -2 {
			// Handle negative indices
			if index < 0 {
				index = len(v) + index
			}
			// Check bounds after negative index conversion
			if index >= 0 && index < len(v) {
				return PropertyAccessResult{Value: v[index], Exists: true, Path: property}
			}
		}
		return PropertyAccessResult{Value: nil, Exists: false, Path: property}

	case nil:
		// Accessing property on nil - boundary case
		return PropertyAccessResult{Value: nil, Exists: false, Path: property}

	default:
		// Cannot access property on this type
		return PropertyAccessResult{Value: nil, Exists: false, Path: property}
	}
}

// handleArrayAccess handles array index access
func (n *navigator) handleArrayAccess(data any, segment PathSegmentInfo) (PropertyAccessResult, error) {
	// Handle property.array[index] syntax
	if segment.Key != "" {
		// First access the property
		propResult := n.HandlePropertyAccess(data, segment.Key)
		if !propResult.Exists {
			return PropertyAccessResult{Value: nil, Exists: false}, nil
		}
		data = propResult.Value
	}

	// Now handle array access
	arr, ok := data.([]any)
	if !ok {
		return PropertyAccessResult{Value: nil, Exists: false}, nil
	}

	index := segment.Index

	// Handle negative indices
	if index < 0 {
		index = len(arr) + index
	}

	// Check bounds
	if index < 0 || index >= len(arr) {
		return PropertyAccessResult{Value: nil, Exists: false}, nil
	}

	return PropertyAccessResult{Value: arr[index], Exists: true}, nil
}

// handleArraySlice handles array slice operations
func (n *navigator) handleArraySlice(data any, segment PathSegmentInfo) (PropertyAccessResult, error) {
	// Handle property.array[start:end:step] syntax
	if segment.Key != "" {
		// First access the property
		propResult := n.HandlePropertyAccess(data, segment.Key)
		if !propResult.Exists {
			return PropertyAccessResult{Value: nil, Exists: false}, nil
		}
		data = propResult.Value
	}

	// Now handle array slice
	arr, ok := data.([]any)
	if !ok {
		return PropertyAccessResult{Value: nil, Exists: false}, nil
	}

	// Parse slice parameters
	start, end, step := n.parseSliceParameters(segment, len(arr))

	// Perform the slice operation
	slicedArray := n.performArraySlice(arr, start, end, step)
	return PropertyAccessResult{Value: slicedArray, Exists: true}, nil
}

// handleExtraction handles extraction operations ({key} syntax)
func (n *navigator) handleExtraction(data any, segment PathSegmentInfo) (PropertyAccessResult, error) {
	extractKey := segment.Key
	if extractKey == "" {
		return PropertyAccessResult{Value: nil, Exists: false}, nil
	}

	switch v := data.(type) {
	case []any:
		// Extract the specified key from all objects in the array
		var results []any
		for _, item := range v {
			if obj, ok := item.(map[string]any); ok {
				if value, exists := obj[extractKey]; exists {
					results = append(results, value)
				} else {
					results = append(results, nil)
				}
			} else {
				results = append(results, nil)
			}
		}
		return PropertyAccessResult{Value: results, Exists: true}, nil

	case map[string]any:
		// Extract the key from the object
		value, exists := v[extractKey]
		return PropertyAccessResult{Value: value, Exists: exists}, nil

	default:
		return PropertyAccessResult{Value: nil, Exists: false}, nil
	}
}

// parseSliceParameters parses slice parameters from a segment
func (n *navigator) parseSliceParameters(segment PathSegmentInfo, arrayLength int) (start, end, step int) {
	// Default values
	start = 0
	end = arrayLength
	step = 1

	// Parse start
	if segment.Start != nil {
		start = *segment.Start
		if start < 0 {
			start = arrayLength + start
		}
	}

	// Parse end
	if segment.End != nil {
		end = *segment.End
		if end < 0 {
			end = arrayLength + end
		}
	}

	// Parse step
	if segment.Step != nil {
		step = *segment.Step
		if step == 0 {
			step = 1 // Avoid division by zero
		}
	}

	// Normalize bounds
	if start < 0 {
		start = 0
	}
	if end > arrayLength {
		end = arrayLength
	}
	if start > end {
		start = end
	}

	return start, end, step
}

// performArraySlice performs the actual array slicing
func (n *navigator) performArraySlice(arr []any, start, end, step int) []any {
	if step == 1 {
		// Simple slice without step
		return arr[start:end]
	}

	// Slice with step
	var result []any
	for i := start; i < end; i += step {
		if i < len(arr) {
			result = append(result, arr[i])
		}
	}
	return result
}

// parseArrayIndex parses an array index from a string
func (n *navigator) parseArrayIndex(indexStr string) int {
	// Remove brackets if present
	if strings.HasPrefix(indexStr, "[") && strings.HasSuffix(indexStr, "]") {
		indexStr = indexStr[1 : len(indexStr)-1]
	}

	// Try to parse as integer
	if index, err := strconv.Atoi(indexStr); err == nil {
		return index
	}

	return -2 // Invalid index marker
}

// isBoundaryCase determines if a navigation failure is a boundary case
func (n *navigator) isBoundaryCase(data any, segment PathSegmentInfo) bool {
	switch segment.Type {
	case "property":
		return n.isPropertyBoundaryCase(data, segment.Key)
	case "array":
		return n.isArrayBoundaryCase(data, segment)
	default:
		return false
	}
}

// isPropertyBoundaryCase checks if property access failure is a boundary case
func (n *navigator) isPropertyBoundaryCase(data any, property string) bool {
	switch data.(type) {
	case map[string]any, map[any]any, nil, []any, string, float64, int, bool:
		return true
	default:
		return false
	}
}

// isArrayBoundaryCase checks if array access failure is a boundary case
func (n *navigator) isArrayBoundaryCase(data any, segment PathSegmentInfo) bool {
	// Get the target array
	var arrayData any = data
	if segment.Key != "" {
		result := n.HandlePropertyAccess(data, segment.Key)
		if !result.Exists {
			return true // Property doesn't exist - boundary case
		}
		arrayData = result.Value
	}

	// Check if target is actually an array
	arr, ok := arrayData.([]any)
	if !ok {
		return false // Not an array - structural error
	}

	// Check if index is out of bounds
	index := segment.Index
	if index < 0 {
		index = len(arr) + index
	}

	return index < 0 || index >= len(arr) // Out of bounds is boundary case
}

// isNumericProperty checks if a property name is numeric
func (n *navigator) isNumericProperty(property string) bool {
	for _, char := range property {
		if char < '0' || char > '9' {
			return false
		}
	}
	return len(property) > 0
}

// navigateToPath navigates to a specific path
func (p *Processor) navigateToPath(data any, path string) (any, error) {
	// Fast path for root access
	if path == "" || path == "." || path == "/" {
		return data, nil
	}

	// Determine path format and use optimized navigation
	if strings.HasPrefix(path, "/") {
		return p.navigateJSONPointer(data, path)
	}

	// Use dot notation navigation
	return p.navigateDotNotation(data, path)
}

// navigateDotNotation handles dot notation paths
func (p *Processor) navigateDotNotation(data any, path string) (any, error) {
	current := data

	// Use path segment pool for memory efficiency
	segments := p.getPathSegments()
	defer p.putPathSegments(segments)

	// Parse path into segments
	segments = p.splitPath(path, segments)

	// Navigate through each segment
	for i, segment := range segments {
		// Check for distributed operations
		if p.isDistributedOperationSegment(segment) {
			// Handle distributed operations
			return p.handleDistributedOperation(current, segments[i:])
		}

		// Handle different segment types
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
			// Handle extraction operations
			extractResult, err := p.handleExtraction(current, segment)
			if err != nil {
				return nil, err
			}
			current = extractResult

			// Check if this is followed by array operations
			if i+1 < len(segments) {
				nextSegment := segments[i+1]
				if nextSegment.TypeString() == "array" || nextSegment.TypeString() == "slice" {
					// Check if the current extraction was flat
					// If it was flat, the result is a flattened array, not an array of arrays
					if segment.IsFlat {
						// For flat extraction results, apply array operations directly
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
						// For non-flat extraction, use post-extraction handling
						current = p.handlePostExtractionArrayAccess(current, nextSegment)
					}
					i++ // Skip the next segment as we've processed it
				}
			}

		default:
			return nil, fmt.Errorf("unsupported segment type: %v", segment.TypeString())
		}
	}

	return current, nil
}

// navigateJSONPointer handles JSON Pointer format paths
func (p *Processor) navigateJSONPointer(data any, path string) (any, error) {
	if path == "/" {
		return data, nil
	}

	// Remove leading slash and split with pre-allocated capacity
	pathWithoutSlash := path[1:]
	segmentCount := strings.Count(pathWithoutSlash, "/") + 1
	segments := make([]string, 0, segmentCount)
	segments = strings.Split(pathWithoutSlash, "/")

	current := data

	for _, segment := range segments {
		if segment == "" {
			continue
		}

		// Optimized unescape for JSON Pointer special characters
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
	// JSON Pointer escaping: ~1 -> /, ~0 -> ~
	segment = strings.ReplaceAll(segment, "~1", "/")
	segment = strings.ReplaceAll(segment, "~0", "~")
	return segment
}

// handlePropertyAccess handles property access with optimized type checking
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
		// Try to parse property as array index
		if index := p.parseArrayIndex(property); index >= 0 && index < len(v) {
			return PropertyAccessResult{Value: v[index], Exists: true}
		}
		return PropertyAccessResult{Exists: false}

	default:
		// Try struct field access
		if structValue := p.handleStructAccess(data, property); structValue != nil {
			return PropertyAccessResult{Value: structValue, Exists: true}
		}
		return PropertyAccessResult{Exists: false}
	}
}

// handlePropertyAccessValue returns the value directly (for backward compatibility)
func (p *Processor) handlePropertyAccessValue(data any, property string) any {
	result := p.handlePropertyAccess(data, property)
	if result.Exists {
		return result.Value
	}
	return nil
}

// parseArrayIndexFromPath parses a string as an array index, returns -1 if invalid
func (p *Processor) parseArrayIndexFromPath(property string) int {
	if index, err := strconv.Atoi(property); err == nil && index >= 0 {
		return index
	}
	return -1
}

// splitPath splits a path into segments with memory pooling
func (p *Processor) splitPath(path string, segments []PathSegment) []PathSegment {
	// Clear existing segments
	segments = segments[:0]

	// Preprocess path to handle special cases
	sb := p.getStringBuilder()
	defer p.putStringBuilder(sb)

	processedPath := p.preprocessPath(path, sb)

	// Split into segments
	return p.splitPathIntoSegments(processedPath, segments)
}

// preprocessPath preprocesses the path to handle special syntax
func (p *Processor) preprocessPath(path string, sb *strings.Builder) string {
	sb.Reset()

	runes := []rune(path)
	for i, r := range runes {
		switch r {
		case '[':
			// Add dot before [ if needed
			if i > 0 && p.needsDotBefore(runes[i-1]) {
				sb.WriteRune('.')
			}
			sb.WriteRune(r)
		case '{':
			// Add dot before { if needed
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

// needsDotBefore checks if a dot is needed before the current character
func (p *Processor) needsDotBefore(prevChar rune) bool {
	// Use same logic as pathParser for consistency
	return (prevChar >= 'a' && prevChar <= 'z') ||
		(prevChar >= 'A' && prevChar <= 'Z') ||
		(prevChar >= '0' && prevChar <= '9') ||
		prevChar == '_' || prevChar == ']' || prevChar == '}'
}

// splitPathIntoSegments splits a preprocessed path into segments
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

// parsePathSegment parses a single path segment
func (p *Processor) parsePathSegment(part string, segments []PathSegment) []PathSegment {
	// Handle different segment types
	if strings.Contains(part, "[") {
		// Array or slice segment
		return p.parseArraySegment(part, segments)
	} else if strings.Contains(part, "{") {
		// Extraction segment
		return p.parseExtractionSegment(part, segments)
	} else {
		// Check if this is a numeric index (dot notation like "0", "1", etc.)
		if index, err := strconv.Atoi(part); err == nil {
			// This is a numeric index, treat as array access
			segments = append(segments, PathSegment{
				Type:  internal.ArrayIndexSegment,
				Index: index,
			})
			return segments
		}

		// Simple property segment
		segments = append(segments, PathSegment{
			Key:  part,
			Type: internal.PropertySegment,
		})
		return segments
	}
}

// isComplexPath determines if a path requires complex processing
func (p *Processor) isComplexPath(path string) bool {
	// Check for complex patterns
	complexPatterns := []string{
		"{", "}", // Extraction syntax
		"[", "]", // Array access
		":", // Slice syntax
	}

	for _, pattern := range complexPatterns {
		if strings.Contains(path, pattern) {
			return true
		}
	}

	return false
}

// hasComplexSegments checks if segments contain complex operations
func (p *Processor) hasComplexSegments(segments []PathSegment) bool {
	for _, segment := range segments {
		switch segment.TypeString() {
		case "slice", "extract":
			return true
		}
	}
	return false
}

// parsePath parses a path string into segments (legacy method)
func (p *Processor) parsePath(path string) ([]string, error) {
	if path == "" {
		return []string{}, nil
	}

	// Simple split for basic paths
	if !p.isComplexPath(path) {
		return strings.Split(path, "."), nil
	}

	// For complex paths, use the full parser
	segments := p.getPathSegments()
	defer p.putPathSegments(segments)

	segments = p.splitPath(path, segments)

	// Convert to string array
	result := make([]string, len(segments))
	for i, segment := range segments {
		result[i] = segment.String()
	}

	return result, nil
}

// isDistributedOperationPath checks if a path requires distributed operation handling
func (p *Processor) isDistributedOperationPath(path string) bool {
	// Check for patterns that indicate distributed operations
	distributedPatterns := []string{
		"}[", // Extraction followed by array access
		"}:", // Extraction followed by slice
		"}{", // Consecutive extractions
	}

	for _, pattern := range distributedPatterns {
		if strings.Contains(path, pattern) {
			return true
		}
	}

	// Check for flat extraction patterns
	if strings.Contains(path, "{flat:") {
		return true
	}

	return false
}

// isDistributedOperationSegment checks if a segment requires distributed handling
func (p *Processor) isDistributedOperationSegment(segment PathSegment) bool {
	// Check segment properties for distributed operation indicators
	// Note: IsDistributed field was removed as it was never used
	return segment.Key != ""
}

// handleDistributedOperation handles distributed operations across extracted arrays
func (p *Processor) handleDistributedOperation(data any, segments []PathSegment) (any, error) {
	return p.getValueWithDistributedOperation(data, p.reconstructPath(segments))
}

// reconstructPath reconstructs a path string from segments
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

// isDeepExtractionPath checks if segments contain deep extraction patterns

// parseArraySegment parses array access segments like [0], [1:3], etc.
func (p *Processor) parseArraySegment(part string, segments []PathSegment) []PathSegment {
	// Find the bracket positions
	openBracket := strings.Index(part, "[")
	closeBracket := strings.LastIndex(part, "]")

	if openBracket == -1 || closeBracket == -1 || closeBracket <= openBracket {
		// Invalid bracket syntax, treat as property
		segments = append(segments, PathSegment{
			Key:  part,
			Type: internal.PropertySegment,
		})
		return segments
	}

	// Extract property name before bracket (if any)
	if openBracket > 0 {
		propertyName := part[:openBracket]
		segments = append(segments, PathSegment{
			Key:  propertyName,
			Type: internal.PropertySegment,
		})
	}

	// Extract bracket content
	bracketContent := part[openBracket+1 : closeBracket]

	// Determine if this is a slice or array index
	if strings.Contains(bracketContent, ":") {
		// Slice syntax - parse slice parameters
		segment := PathSegment{
			Type: internal.ArraySliceSegment,
		}

		// Parse slice parameters
		parts := strings.Split(bracketContent, ":")
		if len(parts) >= 2 {
			// Parse start
			if parts[0] != "" {
				if start, err := strconv.Atoi(parts[0]); err == nil {
					segment.Start = &start
				}
			}

			// Parse end
			if parts[1] != "" {
				if end, err := strconv.Atoi(parts[1]); err == nil {
					segment.End = &end
				}
			}

			// Parse step (if provided)
			if len(parts) == 3 && parts[2] != "" {
				if step, err := strconv.Atoi(parts[2]); err == nil {
					segment.Step = &step
				}
			}
		}

		segments = append(segments, segment)
	} else {
		// Array index
		segment := PathSegment{
			Type: internal.ArrayIndexSegment,
		}

		// Parse index
		if index, err := strconv.Atoi(bracketContent); err == nil {
			segment.Index = index
		}

		segments = append(segments, segment)
	}

	// Handle any remaining part after the bracket
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
	// Find the brace positions
	openBrace := strings.Index(part, "{")
	closeBrace := strings.LastIndex(part, "}")

	if openBrace == -1 || closeBrace == -1 || closeBrace <= openBrace {
		// Invalid brace syntax, treat as property
		segments = append(segments, PathSegment{
			Key:  part,
			Type: internal.PropertySegment,
		})
		return segments
	}

	// Extract property name before brace (if any)
	if openBrace > 0 {
		propertyName := part[:openBrace]
		segments = append(segments, PathSegment{
			Key:  propertyName,
			Type: internal.PropertySegment,
		})
	}

	// Extract brace content
	braceContent := part[openBrace+1 : closeBrace]

	// Create extraction segment
	extractSegment := PathSegment{
		Type: internal.ExtractSegment,
	}

	// Check for flat extraction
	if strings.HasPrefix(braceContent, "flat:") {
		extractSegment.Key = braceContent[5:] // Remove "flat:" prefix
		extractSegment.IsFlat = true
	} else {
		extractSegment.Key = braceContent
	}

	segments = append(segments, extractSegment)

	// Handle any remaining part after the brace
	if closeBrace+1 < len(part) {
		remaining := part[closeBrace+1:]
		if remaining != "" {
			segments = p.parsePathSegment(remaining, segments)
		}
	}

	return segments
}

// Type checking utilities

// isArrayType checks if a value is an array type
func (p *Processor) isArrayType(data any) bool {
	switch data.(type) {
	case []any:
		return true
	default:
		return false
	}
}

// isObjectType checks if a value is an object type
func (p *Processor) isObjectType(data any) bool {
	switch data.(type) {
	case map[string]any, map[any]any:
		return true
	default:
		return false
	}
}

// isMapType checks if a value is a map type
func (p *Processor) isMapType(data any) bool {
	switch data.(type) {
	case map[string]any, map[any]any:
		return true
	default:
		return false
	}
}

// isSliceType checks if a value is a slice type
func (p *Processor) isSliceType(data any) bool {
	if data == nil {
		return false
	}

	v := reflect.ValueOf(data)
	return v.Kind() == reflect.Slice
}

// isPrimitiveType checks if a value is a primitive type
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

// Deep copy utilities

// deepCopyData creates a deep copy of data
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
		return v // Primitives are copied by value
	default:
		// For other types, try to use reflection
		return p.deepCopyReflection(data)
	}
}

// deepCopyStringMap creates a deep copy of a string map
func (p *Processor) deepCopyStringMap(data map[string]any) map[string]any {
	result := make(map[string]any)
	for key, value := range data {
		result[key] = p.deepCopyData(value)
	}
	return result
}

// deepCopyAnyMap creates a deep copy of an any map
func (p *Processor) deepCopyAnyMap(data map[any]any) map[any]any {
	result := make(map[any]any)
	for key, value := range data {
		result[key] = p.deepCopyData(value)
	}
	return result
}

// deepCopyArray creates a deep copy of an array
func (p *Processor) deepCopyArray(data []any) []any {
	result := make([]any, len(data))
	for i, value := range data {
		result[i] = p.deepCopyData(value)
	}
	return result
}

// deepCopyReflection creates a deep copy using reflection
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
		// Create new pointer and copy the value
		newPtr := reflect.New(v.Elem().Type())
		newPtr.Elem().Set(reflect.ValueOf(p.deepCopyReflection(v.Elem().Interface())))
		return newPtr.Interface()
	case reflect.Struct:
		// Create new struct and copy fields
		newStruct := reflect.New(v.Type()).Elem()
		for i := 0; i < v.NumField(); i++ {
			if v.Field(i).CanInterface() {
				newStruct.Field(i).Set(reflect.ValueOf(p.deepCopyReflection(v.Field(i).Interface())))
			}
		}
		return newStruct.Interface()
	default:
		// For other types, return as-is
		return data
	}
}

// Path utilities

// unescapeJSONPointer unescapes JSON Pointer characters

// escapeJSONPointer escapes JSON Pointer characters
func (p *Processor) escapeJSONPointer(segment string) string {
	// JSON Pointer escaping: ~ becomes ~0, / becomes ~1
	segment = strings.ReplaceAll(segment, "~", "~0")
	segment = strings.ReplaceAll(segment, "/", "~1")
	return segment
}

// normalizePathSeparators normalizes path separators
func (p *Processor) normalizePathSeparators(path string) string {
	// Replace multiple dots with single dots
	for strings.Contains(path, "..") {
		path = strings.ReplaceAll(path, "..", ".")
	}

	// Remove leading and trailing dots
	path = strings.Trim(path, ".")

	return path
}

// splitPathSegments splits a path into segments
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

// joinPathSegments joins segments into a path
func (p *Processor) joinPathSegments(segments []string, useJSONPointer bool) string {
	if len(segments) == 0 {
		return ""
	}

	if useJSONPointer {
		return "/" + strings.Join(segments, "/")
	}

	return strings.Join(segments, ".")
}

// Validation utilities

// isValidPropertyName checks if a property name is valid
func (p *Processor) isValidPropertyName(name string) bool {
	return name != "" && !strings.ContainsAny(name, ".[]{}()")
}

// isValidArrayIndex checks if an array index is valid
func (p *Processor) isValidArrayIndex(index string) bool {
	if index == "" {
		return false
	}

	// Check for negative indices
	index = strings.TrimPrefix(index, "-")

	// Check if it's a valid number
	_, err := strconv.Atoi(index)
	return err == nil
}

// isValidSliceRange checks if a slice range is valid
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

// Error utilities

// wrapError wraps an error with additional context
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
