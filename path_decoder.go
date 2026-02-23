package json

import (
	"encoding/json"
	"strconv"
	"strings"
	"sync"

	"github.com/cybergodev/json/internal"
)

// NumberPreservingDecoder provides JSON decoding with optimized number format preservation
type NumberPreservingDecoder struct {
	preserveNumbers bool

	// bufferPool is used for efficient string formatting operations
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

	// Create a new decoder for each call (json.Decoder cannot be reused with different inputs)
	decoder := json.NewDecoder(strings.NewReader(jsonStr))
	decoder.UseNumber()

	var result any
	if err := decoder.Decode(&result); err != nil {
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
	return internal.ContainsAnyByte(s, chars)
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
	return internal.IsValidNumberString(s)
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
