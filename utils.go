package json

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"strings"
)

// Modern Go 1.24+ type constraints with enhanced generic support
type (
	// Numeric represents all numeric types with improved constraint definition
	Numeric interface {
		~int | ~int8 | ~int16 | ~int32 | ~int64 |
			~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64 |
			~float32 | ~float64
	}

	// Ordered represents types that can be ordered (supports comparison operators)
	Ordered interface {
		Numeric | ~string
	}

	// JSONValue represents valid JSON value types with type safety
	JSONValue interface {
		~bool | ~string | Numeric | ~[]any | ~map[string]any | any
	}

	// Signed represents signed integer types for better type safety
	Signed interface {
		~int | ~int8 | ~int16 | ~int32 | ~int64
	}

	// Unsigned represents unsigned integer types for better type safety
	Unsigned interface {
		~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64
	}

	// Float represents floating-point types for better type safety
	Float interface {
		~float32 | ~float64
	}
)

// GetTypedWithProcessor retrieves a typed value from JSON using a specific processor
func GetTypedWithProcessor[T any](processor *Processor, jsonStr, path string, opts ...*ProcessorOptions) (T, error) {
	var zero T

	// Get the raw value
	value, err := processor.Get(jsonStr, path, opts...)
	if err != nil {
		return zero, err
	}

	// Handle null values specially
	if value == nil {
		return handleNullValue[T](path)
	}

	// Try direct type assertion first
	if typedValue, ok := value.(T); ok {
		return typedValue, nil
	}

	// Special handling for numeric types with large numbers
	convResult, handled := handleLargeNumberConversion[T](value, path)
	if handled {
		return convResult.value, convResult.err
	}

	// Try more efficient type conversions before falling back to JSON marshaling
	if converted, ok := tryDirectConversion[T](value); ok {
		return converted, nil
	}

	// Last resort: JSON marshaling/unmarshaling for type conversion with number preservation
	// Use custom encoder to preserve number formats
	config := NewPrettyConfig()
	config.PreserveNumbers = true

	encoder := NewCustomEncoder(config)
	defer encoder.Close()

	encodedJson, err := encoder.Encode(value)
	if err != nil {
		return zero, &JsonsError{
			Op:      "get_typed",
			Path:    path,
			Message: fmt.Sprintf("failed to marshal value for type conversion: %v", err),
			Err:     ErrTypeMismatch,
		}
	}

	var finalResult T
	// Use number-preserving unmarshal for better type conversion
	if err := PreservingUnmarshal([]byte(encodedJson), &finalResult, true); err != nil {
		return zero, &JsonsError{
			Op:      "get_typed",
			Path:    path,
			Message: fmt.Sprintf("failed to convert value to type %T: %v", finalResult, err),
			Err:     ErrTypeMismatch,
		}
	}

	return finalResult, nil
}

// tryDirectConversion attempts direct type conversion without JSON marshaling
func tryDirectConversion[T any](value any) (T, bool) {
	var zero T

	// Handle array/slice conversions first
	if converted, ok := tryArrayConversion[T](value); ok {
		return converted, true
	}

	// Try direct conversion for common types
	switch any(&zero).(type) {
	case *string:
		if str, ok := value.(string); ok {
			return any(str).(T), true
		}
		// Convert numbers to string
		switch v := value.(type) {
		case json.Number:
			// Handle json.Number to preserve original format
			return any(string(v)).(T), true
		case int:
			return any(fmt.Sprintf("%d", v)).(T), true
		case int64:
			return any(fmt.Sprintf("%d", v)).(T), true
		case float64:
			return any(fmt.Sprintf("%g", v)).(T), true
		case bool:
			return any(fmt.Sprintf("%t", v)).(T), true
		}
	case *int:
		switch v := value.(type) {
		case int:
			return any(v).(T), true
		case int64:
			if v >= int64(^uint(0)>>1) && v <= int64(^uint(0)>>1) {
				return any(int(v)).(T), true
			}
		case float64:
			if v == float64(int(v)) {
				return any(int(v)).(T), true
			}
		case json.Number:
			// Handle json.Number to preserve original format
			if i, err := v.Int64(); err == nil {
				return any(int(i)).(T), true
			}
			// Try as float if integer conversion fails
			if f, err := v.Float64(); err == nil && f == float64(int(f)) {
				return any(int(f)).(T), true
			}
		case string:
			if i, err := strconv.Atoi(v); err == nil {
				return any(i).(T), true
			}
		}
	case *int64:
		switch v := value.(type) {
		case int:
			return any(int64(v)).(T), true
		case int64:
			return any(v).(T), true
		case float64:
			if v == float64(int64(v)) {
				return any(int64(v)).(T), true
			}
		case json.Number:
			// Handle json.Number to preserve original format
			if i, err := v.Int64(); err == nil {
				return any(i).(T), true
			}
			// Try as float if integer conversion fails
			if f, err := v.Float64(); err == nil && f == float64(int64(f)) {
				return any(int64(f)).(T), true
			}
		case string:
			if i, err := strconv.ParseInt(v, 10, 64); err == nil {
				return any(i).(T), true
			}
		}
	case *float64:
		switch v := value.(type) {
		case float64:
			return any(v).(T), true
		case int:
			return any(float64(v)).(T), true
		case int64:
			return any(float64(v)).(T), true
		case json.Number:
			// Handle json.Number to preserve original format
			if f, err := v.Float64(); err == nil {
				return any(f).(T), true
			}
		case string:
			if f, err := strconv.ParseFloat(v, 64); err == nil {
				return any(f).(T), true
			}
		}
	case *bool:
		switch v := value.(type) {
		case bool:
			return any(v).(T), true
		case string:
			if b, err := strconv.ParseBool(v); err == nil {
				return any(b).(T), true
			}
		}
	}

	return zero, false
}

// handleNullValue handles null values for different target types
func handleNullValue[T any](path string) (T, error) {
	var zero T
	targetType := fmt.Sprintf("%T", zero)

	switch targetType {
	case "string":
		// For string type, return "null" as string representation
		if result, ok := any("null").(T); ok {
			return result, nil
		}
	case "*string":
		// For pointer to string, return nil pointer
		if result, ok := any((*string)(nil)).(T); ok {
			return result, nil
		}
	case "int", "int8", "int16", "int32", "int64":
		// For integer types, return zero value
		return zero, nil
	case "uint", "uint8", "uint16", "uint32", "uint64":
		// For unsigned integer types, return zero value
		return zero, nil
	case "float32", "float64":
		// For float types, return zero value
		return zero, nil
	case "bool":
		// For bool type, return false
		return zero, nil
	default:
		// For other types (slices, maps, structs, pointers), return zero value
		return zero, nil
	}

	return zero, &JsonsError{
		Op:      "get_typed",
		Path:    path,
		Message: fmt.Sprintf("cannot convert null to type %T", zero),
		Err:     ErrTypeMismatch,
	}
}

// conversionResult holds the result of a type conversion attempt
type conversionResult[T any] struct {
	value T
	err   error
}

// handleLargeNumberConversion handles conversion of large numbers to specific types
func handleLargeNumberConversion[T any](value any, path string) (conversionResult[T], bool) {
	var zero T

	// Get the target type information
	targetType := fmt.Sprintf("%T", zero)

	switch targetType {
	case "int64":
		if converted, err := SafeConvertToInt64(value); err == nil {
			// Use type assertion to convert to T (which we know is int64)
			if typedResult, ok := any(converted).(T); ok {
				return conversionResult[T]{value: typedResult, err: nil}, true
			}
		} else {
			// Return error with helpful message
			return conversionResult[T]{
				value: zero,
				err: &JsonsError{
					Op:      "get_typed",
					Path:    path,
					Message: fmt.Sprintf("large number conversion failed: %v", err),
					Err:     ErrTypeMismatch,
				},
			}, true
		}

	case "uint64":
		if converted, err := SafeConvertToUint64(value); err == nil {
			if typedResult, ok := any(converted).(T); ok {
				return conversionResult[T]{value: typedResult, err: nil}, true
			}
		} else {
			return conversionResult[T]{
				value: zero,
				err: &JsonsError{
					Op:      "get_typed",
					Path:    path,
					Message: fmt.Sprintf("large number conversion failed: %v", err),
					Err:     ErrTypeMismatch,
				},
			}, true
		}

	case "string":
		// For string type, convert any numeric value to string representation
		if strResult, ok := any(FormatNumber(value)).(T); ok {
			return conversionResult[T]{value: strResult, err: nil}, true
		}
	}

	// Not handled by this function
	return conversionResult[T]{value: zero, err: nil}, false
}

// IsValidJson quickly checks if a string is valid JSON
func IsValidJson(jsonStr string) bool {
	decoder := NewNumberPreservingDecoder(false) // Use fast validation
	_, err := decoder.DecodeToAny(jsonStr)
	return err == nil
}

// IsValidPath checks if a path expression is valid with comprehensive validation
func IsValidPath(path string) bool {
	// Empty path is invalid
	if path == "" {
		return false
	}

	// Root path "." is valid
	if path == "." {
		return true
	}

	// Use default processor for validation
	processor := getDefaultProcessor()
	err := processor.validatePath(path)
	return err == nil
}

// ValidatePath validates a path expression and returns detailed error information
func ValidatePath(path string) error {
	// Empty path is invalid
	if path == "" {
		return &JsonsError{
			Op:      "validate_path",
			Path:    path,
			Message: "path cannot be empty",
			Err:     ErrInvalidPath,
		}
	}

	// Root path "." is valid
	if path == "." {
		return nil
	}

	// Use default processor for detailed validation
	processor := getDefaultProcessor()
	return processor.validatePath(path)
}

// isJSONPointerPath checks if a path is in JSON Pointer format (internal use)
func isJSONPointerPath(path string) bool {
	return path != "" && path[0] == '/'
}

// isDotNotationPath checks if a path is in dot notation format (internal use)
func isDotNotationPath(path string) bool {
	return path != "" && path != "." && path[0] != '/' && !isJSONPointerPath(path)
}

// isArrayPath checks if a path contains array access syntax (internal use)
func isArrayPath(path string) bool {
	return strings.Contains(path, "[") && strings.Contains(path, "]")
}

// isSlicePath checks if a path contains slice syntax (internal use)
func isSlicePath(path string) bool {
	return strings.Contains(path, "[") && strings.Contains(path, ":") && strings.Contains(path, "]")
}

// isExtractionPath checks if a path contains extraction syntax (internal use)
func isExtractionPath(path string) bool {
	return strings.Contains(path, "{") && strings.Contains(path, "}")
}

// getPathType returns the type of path (dot_notation, json_pointer, or mixed) (internal use)
func getPathType(path string) string {
	if path == "" {
		return "invalid"
	}

	if path == "." {
		return "root"
	}

	if isJSONPointerPath(path) {
		return "json_pointer"
	}

	if isDotNotationPath(path) {
		if isArrayPath(path) || isSlicePath(path) || isExtractionPath(path) {
			return "dot_notation_complex"
		}
		return "dot_notation_simple"
	}

	return "unknown"
}

// isJsonObject checks if data is a JSON object type (internal use)
func isJsonObject(data any) bool {
	_, ok := data.(map[string]any)
	return ok
}

// isJsonArray checks if data is a JSON array type (internal use)
func isJsonArray(data any) bool {
	_, ok := data.([]any)
	return ok
}

// isJsonPrimitive checks if data is a JSON primitive type (internal use)
func isJsonPrimitive(data any) bool {
	switch data.(type) {
	case string, int, int32, int64, float32, float64, bool, nil:
		return true
	default:
		return false
	}
}

// DeepCopy creates a deep copy of JSON-compatible data with improved efficiency
func DeepCopy(data any) (any, error) {
	// Fast path for simple types that don't need deep copying
	switch v := data.(type) {
	case nil, bool, int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64, string:
		return v, nil
	}

	// Use JSON marshaling/unmarshaling for deep copy with number preservation
	jsonBytes, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal data for deep copy: %v", err)
	}

	decoder := NewNumberPreservingDecoder(true)
	result, err := decoder.DecodeToAny(string(jsonBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal data for deep copy: %v", err)
	}

	return result, nil
}

// CompareJson compares two JSON strings for equality
func CompareJson(json1, json2 string) (bool, error) {
	decoder := NewNumberPreservingDecoder(true)

	data1, err := decoder.DecodeToAny(json1)
	if err != nil {
		return false, fmt.Errorf("invalid JSON in first argument: %v", err)
	}

	data2, err := decoder.DecodeToAny(json2)
	if err != nil {
		return false, fmt.Errorf("invalid JSON in second argument: %v", err)
	}

	// Convert both to JSON strings for comparison
	bytes1, err := json.Marshal(data1)
	if err != nil {
		return false, err
	}

	bytes2, err := json.Marshal(data2)
	if err != nil {
		return false, err
	}

	return string(bytes1) == string(bytes2), nil
}

// MergeJson merges two JSON objects
func MergeJson(json1, json2 string) (string, error) {
	decoder := NewNumberPreservingDecoder(true)

	data1, err := decoder.DecodeToAny(json1)
	if err != nil {
		return "", fmt.Errorf("invalid JSON in first argument: %v", err)
	}

	data2, err := decoder.DecodeToAny(json2)
	if err != nil {
		return "", fmt.Errorf("invalid JSON in second argument: %v", err)
	}

	// Ensure both are objects
	obj1, ok1 := data1.(map[string]any)
	obj2, ok2 := data2.(map[string]any)

	if !ok1 {
		return "", fmt.Errorf("first JSON is not an object")
	}
	if !ok2 {
		return "", fmt.Errorf("second JSON is not an object")
	}

	// Merge obj2 into obj1
	for key, value := range obj2 {
		obj1[key] = value
	}

	result, err := json.Marshal(obj1)
	if err != nil {
		return "", fmt.Errorf("failed to marshal merged result: %v", err)
	}

	return string(result), nil
}

// GetNumeric retrieves a numeric value with modern generic constraints (alias for GetTyped)
func GetNumeric[T Numeric](jsonStr, path string, opts ...*ProcessorOptions) (T, error) {
	return GetTyped[T](jsonStr, path, opts...)
}

// GetOrdered retrieves an ordered value with default fallback (alias for GetTypedWithDefault)
func GetOrdered[T Ordered](jsonStr, path string, defaultValue T, opts ...*ProcessorOptions) T {
	return GetTypedWithDefault(jsonStr, path, defaultValue, opts...)
}

// GetJSONValue retrieves any valid JSON value with type constraints
func GetJSONValue[T JSONValue](jsonStr, path string, opts ...*ProcessorOptions) (T, error) {
	return GetTyped[T](jsonStr, path, opts...)
}

// SafeTypeAssert performs a safe type assertion with error handling
func SafeTypeAssert[T any](value any) (T, bool) {
	if result, ok := value.(T); ok {
		return result, true
	}
	var zero T
	return zero, false
}

// MustTypeAssert performs a type assertion that panics on failure (for internal use only)
func MustTypeAssert[T any](value any, context string) T {
	if result, ok := value.(T); ok {
		return result
	}
	var zero T
	panic(fmt.Sprintf("type assertion failed in %s: expected %T, got %T", context, zero, value))
}

// TypeSafeConvert attempts to convert a value to the target type safely
func TypeSafeConvert[T any](value any) (T, error) {
	var zero T

	// Direct type assertion first
	if result, ok := value.(T); ok {
		return result, nil
	}

	// Handle common conversions
	targetType := fmt.Sprintf("%T", zero)
	return convertWithTypeInfo[T](value, targetType)
}

// convertWithTypeInfo handles type conversion with type information
func convertWithTypeInfo[T any](value any, targetType string) (T, error) {
	var zero T

	// Use the existing conversion logic but with better type safety
	convResult, handled := handleLargeNumberConversion[T](value, "type_conversion")
	if handled {
		return convResult.value, convResult.err
	}

	// Handle string conversions
	if str, ok := value.(string); ok {
		return convertStringToType[T](str, targetType)
	}

	return zero, fmt.Errorf("cannot convert %T to %s", value, targetType)
}

// convertStringToType converts string values to target types safely
func convertStringToType[T any](str, targetType string) (T, error) {
	var zero T

	switch targetType {
	case "int", "int64":
		if val, err := strconv.ParseInt(str, 10, 64); err == nil {
			if result, ok := any(val).(T); ok {
				return result, nil
			}
		}
	case "float64":
		if val, err := strconv.ParseFloat(str, 64); err == nil {
			if result, ok := any(val).(T); ok {
				return result, nil
			}
		}
	case "bool":
		if val, err := strconv.ParseBool(str); err == nil {
			if result, ok := any(val).(T); ok {
				return result, nil
			}
		}
	case "string":
		if result, ok := any(str).(T); ok {
			return result, nil
		}
	}

	return zero, fmt.Errorf("cannot convert string %q to %s", str, targetType)
}

// getJsonSize returns the size of JSON string in bytes (internal use)
func getJsonSize(jsonStr string) int64 {
	return int64(len(jsonStr))
}

// estimateMemoryUsage estimates the memory usage of parsed JSON data (internal use)
func estimateMemoryUsage(data any) int64 {
	return estimateSize(data)
}

// estimateSize estimates the memory size of data with improved accuracy
func estimateSize(data any) int64 {
	if data == nil {
		return 8 // Size of a pointer
	}

	switch v := data.(type) {
	case string:
		return int64(len(v) + 16) // String header overhead
	case []byte:
		return int64(len(v) + 24) // Slice header overhead
	case bool:
		return 1
	case int8, uint8:
		return 1
	case int16, uint16:
		return 2
	case int32, uint32, float32:
		return 4
	case int, int64, uint64, float64:
		return 8
	case map[string]any:
		size := int64(48) // Map header overhead
		for key, value := range v {
			size += int64(len(key)+16) + estimateSize(value)
		}
		return size
	case []any:
		size := int64(24) // Slice header overhead
		for _, item := range v {
			size += estimateSize(item)
		}
		return size
	default:
		return 64 // Default size for unknown types
	}
}

// tryArrayConversion attempts to convert []interface{} to typed arrays/slices
func tryArrayConversion[T any](value any) (T, bool) {
	var zero T

	// Check if the target type is a slice
	targetType := reflect.TypeOf(zero)
	if targetType.Kind() != reflect.Slice {
		return zero, false
	}

	// Check if the source is []interface{}
	sourceSlice, ok := value.([]interface{})
	if !ok {
		return zero, false
	}

	// Get the element type of the target slice
	elemType := targetType.Elem()

	// Create a new slice of the target type
	resultSlice := reflect.MakeSlice(targetType, len(sourceSlice), len(sourceSlice))

	// Convert each element
	for i, elem := range sourceSlice {
		// Convert the element to the target element type
		convertedElem, err := convertElementToType(elem, elemType)
		if err != nil {
			return zero, false
		}
		resultSlice.Index(i).Set(reflect.ValueOf(convertedElem))
	}

	// Convert the result back to T
	if result, ok := resultSlice.Interface().(T); ok {
		return result, true
	}

	return zero, false
}

// convertElementToType converts a single element to the target type
func convertElementToType(elem interface{}, targetType reflect.Type) (interface{}, error) {
	if elem == nil {
		return reflect.Zero(targetType).Interface(), nil
	}

	// If the element is already the correct type, return it
	if reflect.TypeOf(elem) == targetType {
		return elem, nil
	}

	// Handle common type conversions
	switch targetType.Kind() {
	case reflect.String:
		return fmt.Sprintf("%v", elem), nil
	case reflect.Int:
		if f, ok := elem.(float64); ok {
			return int(f), nil
		}
		if i, ok := elem.(int); ok {
			return i, nil
		}
		if s, ok := elem.(string); ok {
			if i, err := strconv.Atoi(s); err == nil {
				return i, nil
			}
		}
	case reflect.Int64:
		if f, ok := elem.(float64); ok {
			return int64(f), nil
		}
		if i, ok := elem.(int64); ok {
			return i, nil
		}
		if i, ok := elem.(int); ok {
			return int64(i), nil
		}
	case reflect.Float64:
		if f, ok := elem.(float64); ok {
			return f, nil
		}
		if i, ok := elem.(int); ok {
			return float64(i), nil
		}
		if s, ok := elem.(string); ok {
			if f, err := strconv.ParseFloat(s, 64); err == nil {
				return f, nil
			}
		}
	case reflect.Bool:
		if b, ok := elem.(bool); ok {
			return b, nil
		}
		if s, ok := elem.(string); ok {
			if b, err := strconv.ParseBool(s); err == nil {
				return b, nil
			}
		}
	}

	return nil, fmt.Errorf("cannot convert %T to %v", elem, targetType)
}
