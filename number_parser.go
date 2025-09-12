package json

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"
)

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
			New: func() interface{} {
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

// FormatNumber formats a number without scientific notation when possible
func FormatNumber(value any) string {
	switch v := value.(type) {
	case int, int8, int16, int32, int64:
		return fmt.Sprintf("%d", v)
	case uint, uint8, uint16, uint32, uint64:
		return fmt.Sprintf("%d", v)
	case float32:
		// Use fixed-point notation for reasonable range
		if v >= -1e6 && v <= 1e6 {
			return strconv.FormatFloat(float64(v), 'f', -1, 32)
		}
		return strconv.FormatFloat(float64(v), 'g', -1, 32)
	case float64:
		// Use fixed-point notation for reasonable range
		if v >= -1e15 && v <= 1e15 {
			return strconv.FormatFloat(v, 'f', -1, 64)
		}
		return strconv.FormatFloat(v, 'g', -1, 64)
	case json.Number:
		return string(v)
	case string:
		// If it's a string that represents a number, return as-is
		return v
	default:
		return fmt.Sprintf("%v", v)
	}
}

// SafeConvertToInt64 safely converts a value to int64, handling large numbers
func SafeConvertToInt64(value any) (int64, error) {
	switch v := value.(type) {
	case int:
		return int64(v), nil
	case int32:
		return int64(v), nil
	case int64:
		return v, nil
	case uint:
		if v <= uint(^uint64(0)>>1) {
			return int64(v), nil
		}
		return 0, fmt.Errorf("value %d exceeds int64 range", v)
	case uint32:
		return int64(v), nil
	case uint64:
		if v <= uint64(^uint64(0)>>1) {
			return int64(v), nil
		}
		return 0, fmt.Errorf("value %d exceeds int64 range", v)
	case float64:
		if v >= float64(^uint64(0)>>1)*-1 && v <= float64(^uint64(0)>>1) && v == float64(int64(v)) {
			return int64(v), nil
		}
		return 0, fmt.Errorf("value %g cannot be safely converted to int64", v)
	case string:
		// Try to parse string as int64
		if i, err := strconv.ParseInt(v, 10, 64); err == nil {
			return i, nil
		}
		return 0, fmt.Errorf("string %q cannot be converted to int64", v)
	case json.Number:
		return SafeConvertToInt64(string(v))
	default:
		return 0, fmt.Errorf("cannot convert %T to int64", v)
	}
}

// SafeConvertToUint64 safely converts a value to uint64, handling large numbers
func SafeConvertToUint64(value any) (uint64, error) {
	switch v := value.(type) {
	case int:
		if v >= 0 {
			return uint64(v), nil
		}
		return 0, fmt.Errorf("negative value %d cannot be converted to uint64", v)
	case int32:
		if v >= 0 {
			return uint64(v), nil
		}
		return 0, fmt.Errorf("negative value %d cannot be converted to uint64", v)
	case int64:
		if v >= 0 {
			return uint64(v), nil
		}
		return 0, fmt.Errorf("negative value %d cannot be converted to uint64", v)
	case uint:
		return uint64(v), nil
	case uint32:
		return uint64(v), nil
	case uint64:
		return v, nil
	case float64:
		if v >= 0 && v <= float64(^uint64(0)) && v == float64(uint64(v)) {
			return uint64(v), nil
		}
		return 0, fmt.Errorf("value %g cannot be safely converted to uint64", v)
	case string:
		// Try to parse string as uint64
		if u, err := strconv.ParseUint(v, 10, 64); err == nil {
			return u, nil
		}
		return 0, fmt.Errorf("string %q cannot be converted to uint64", v)
	case json.Number:
		return SafeConvertToUint64(string(v))
	default:
		return 0, fmt.Errorf("cannot convert %T to uint64", v)
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
