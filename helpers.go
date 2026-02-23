package json

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/cybergodev/json/internal"
)

// IsValidJSON quickly checks if a string is valid JSON
func IsValidJSON(jsonStr string) bool {
	decoder := NewNumberPreservingDecoder(false)
	_, err := decoder.DecodeToAny(jsonStr)
	return err == nil
}

// IsValidPath checks if a path expression is valid
func IsValidPath(path string) bool {
	if path == "" {
		return false
	}
	if path == "." {
		return true
	}
	processor := getDefaultProcessor()
	err := processor.validatePath(path)
	return err == nil
}

// ValidatePath validates a path expression and returns detailed error information
func ValidatePath(path string) error {
	if path == "" {
		return &JsonsError{
			Op:      "validate_path",
			Path:    path,
			Message: "path cannot be empty",
			Err:     ErrInvalidPath,
		}
	}
	if path == "." {
		return nil
	}
	processor := getDefaultProcessor()
	return processor.validatePath(path)
}

// DeepCopy creates a deep copy of JSON-compatible data
func DeepCopy(data any) (any, error) {
	switch v := data.(type) {
	case nil, bool, int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64, string:
		return v, nil
	}

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

// MergeJson merges two JSON objects using deep merge strategy
// For nested objects, it recursively merges keys (union merge)
// For primitive values and arrays, the value from json2 takes precedence
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

	obj1, ok1 := data1.(map[string]any)
	obj2, ok2 := data2.(map[string]any)

	if !ok1 {
		return "", fmt.Errorf("first JSON is not an object")
	}
	if !ok2 {
		return "", fmt.Errorf("second JSON is not an object")
	}

	merged := internal.DeepMerge(obj1, obj2)

	result, err := json.Marshal(merged)
	if err != nil {
		return "", fmt.Errorf("failed to marshal merged result: %v", err)
	}

	return string(result), nil
}

// deepMerge recursively merges two JSON values using union merge strategy
// - If both values are objects, recursively merge their keys
// - If both values are arrays, merge with deduplication (union)
// - For all other cases (primitives), value2 takes precedence
func deepMerge(base, override any) any {
	return internal.DeepMerge(base, override)
}

// arrayItemKey generates a unique key for array item deduplication
func arrayItemKey(item any) string {
	return internal.ArrayItemKey(item)
}

// formatNumberForDedup formats a number for deduplication key generation
func formatNumberForDedup(f float64) string {
	return internal.FormatNumberForDedup(f)
}

// GetTypedWithProcessor retrieves a typed value from JSON using a specific processor
func GetTypedWithProcessor[T any](processor *Processor, jsonStr, path string, opts ...*ProcessorOptions) (T, error) {
	var zero T

	value, err := processor.Get(jsonStr, path, opts...)
	if err != nil {
		return zero, err
	}

	if value == nil {
		return handleNullValue[T](path)
	}

	if converted, ok := UnifiedTypeConversion[T](value); ok {
		return converted, nil
	}

	jsonBytes, err := json.Marshal(value)
	if err != nil {
		return zero, &JsonsError{
			Op:      "get_typed",
			Path:    path,
			Message: fmt.Sprintf("failed to marshal value for type conversion: %v", err),
			Err:     ErrTypeMismatch,
		}
	}

	var finalResult T
	if err := json.Unmarshal(jsonBytes, &finalResult); err != nil {
		return zero, &JsonsError{
			Op:      "get_typed",
			Path:    path,
			Message: fmt.Sprintf("failed to convert value to type %T: %v", finalResult, err),
			Err:     ErrTypeMismatch,
		}
	}

	return finalResult, nil
}

// handleNullValue handles null values for different target types using direct type checking
func handleNullValue[T any](path string) (T, error) {
	var zero T

	// Use direct type checking instead of string reflection for better performance
	switch any(zero).(type) {
	case string:
		// Return empty string for null values
		if result, ok := any("").(T); ok {
			return result, nil
		}
	case *string:
		if result, ok := any((*string)(nil)).(T); ok {
			return result, nil
		}
	case int, int8, int16, int32, int64,
		uint, uint8, uint16, uint32, uint64,
		float32, float64, bool:
		return zero, nil
	default:
		return zero, nil
	}

	return zero, &JsonsError{
		Op:      "get_typed",
		Path:    path,
		Message: fmt.Sprintf("cannot convert null to type %T", zero),
		Err:     ErrTypeMismatch,
	}
}

// TypeSafeConvert attempts to convert a value to the target type safely
func TypeSafeConvert[T any](value any) (T, error) {
	var zero T

	if result, ok := value.(T); ok {
		return result, nil
	}

	targetType := fmt.Sprintf("%T", zero)
	return convertWithTypeInfo[T](value, targetType)
}

// convertWithTypeInfo handles type conversion with type information
func convertWithTypeInfo[T any](value any, targetType string) (T, error) {
	var zero T

	convResult, handled := handleLargeNumberConversion[T](value, "type_conversion")
	if handled {
		return convResult.value, convResult.err
	}

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

// conversionResult holds the result of a type conversion attempt
type conversionResult[T any] struct {
	value T
	err   error
}

// handleLargeNumberConversion handles conversion of large numbers to specific types
func handleLargeNumberConversion[T any](value any, path string) (conversionResult[T], bool) {
	var zero T
	targetType := fmt.Sprintf("%T", zero)

	switch targetType {
	case "int64":
		if converted, err := SafeConvertToInt64(value); err == nil {
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
		if strResult, ok := any(FormatNumber(value)).(T); ok {
			return conversionResult[T]{value: strResult, err: nil}, true
		}
	}

	return conversionResult[T]{value: zero, err: nil}, false
}

// IteratorControl represents control flags for iteration
type IteratorControl int

const (
	IteratorNormal IteratorControl = iota
	IteratorContinue
	IteratorBreak
)

// Internal path type checking functions - delegate to internal package
func isJSONPointerPath(path string) bool {
	return internal.IsJSONPointerPath(path)
}

func isDotNotationPath(path string) bool {
	return internal.IsDotNotationPath(path)
}

func isArrayPath(path string) bool {
	return internal.IsArrayPath(path)
}

func isSlicePath(path string) bool {
	return internal.IsSlicePath(path)
}

func isExtractionPath(path string) bool {
	return internal.IsExtractionPath(path)
}

func isJsonObject(data any) bool {
	return internal.IsJSONObject(data)
}

func isJsonArray(data any) bool {
	return internal.IsJSONArray(data)
}

func isJsonPrimitive(data any) bool {
	return internal.IsJSONPrimitive(data)
}

// tryConvertToArray attempts to convert a map to an array if it has numeric keys
func tryConvertToArray(m map[string]any) ([]any, bool) {
	return internal.TryConvertToArray(m)
}
