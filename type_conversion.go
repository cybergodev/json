package json

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
)

// Unified type conversion module
// Consolidates all type conversion logic into a single, maintainable location
// This module provides the core conversion functions used throughout the library

// ConvertToInt converts any value to int with comprehensive type support
func ConvertToInt(value any) (int, bool) {
	switch v := value.(type) {
	case int:
		return v, true
	case int8:
		return int(v), true
	case int16:
		return int(v), true
	case int32:
		return int(v), true
	case int64:
		if v >= -2147483648 && v <= 2147483647 {
			return int(v), true
		}
	case uint:
		if v <= 2147483647 {
			return int(v), true
		}
	case uint8:
		return int(v), true
	case uint16:
		return int(v), true
	case uint32:
		if v <= 2147483647 {
			return int(v), true
		}
	case uint64:
		if v <= 2147483647 {
			return int(v), true
		}
	case float32:
		if v == float32(int(v)) && v >= -2147483648 && v <= 2147483647 {
			return int(v), true
		}
	case float64:
		if v == float64(int(v)) && v >= -2147483648 && v <= 2147483647 {
			return int(v), true
		}
	case string:
		if i, err := strconv.Atoi(v); err == nil {
			return i, true
		}
	case bool:
		if v {
			return 1, true
		}
		return 0, true
	case json.Number:
		if i, err := v.Int64(); err == nil && i >= -2147483648 && i <= 2147483647 {
			return int(i), true
		}
	}
	return 0, false
}

// ConvertToInt64 converts any value to int64
func ConvertToInt64(value any) (int64, bool) {
	switch v := value.(type) {
	case int:
		return int64(v), true
	case int8:
		return int64(v), true
	case int16:
		return int64(v), true
	case int32:
		return int64(v), true
	case int64:
		return v, true
	case uint:
		return int64(v), true
	case uint8:
		return int64(v), true
	case uint16:
		return int64(v), true
	case uint32:
		return int64(v), true
	case uint64:
		if v <= 9223372036854775807 {
			return int64(v), true
		}
	case float32:
		if v == float32(int64(v)) {
			return int64(v), true
		}
	case float64:
		if v == float64(int64(v)) {
			return int64(v), true
		}
	case string:
		if i, err := strconv.ParseInt(v, 10, 64); err == nil {
			return i, true
		}
	case bool:
		if v {
			return 1, true
		}
		return 0, true
	case json.Number:
		if i, err := v.Int64(); err == nil {
			return i, true
		}
	}
	return 0, false
}

// ConvertToUint64 converts any value to uint64
func ConvertToUint64(value any) (uint64, bool) {
	switch v := value.(type) {
	case uint:
		return uint64(v), true
	case uint8:
		return uint64(v), true
	case uint16:
		return uint64(v), true
	case uint32:
		return uint64(v), true
	case uint64:
		return v, true
	case int:
		if v >= 0 {
			return uint64(v), true
		}
	case int8:
		if v >= 0 {
			return uint64(v), true
		}
	case int16:
		if v >= 0 {
			return uint64(v), true
		}
	case int32:
		if v >= 0 {
			return uint64(v), true
		}
	case int64:
		if v >= 0 {
			return uint64(v), true
		}
	case float32:
		if v >= 0 && v == float32(uint64(v)) {
			return uint64(v), true
		}
	case float64:
		if v >= 0 && v == float64(uint64(v)) {
			return uint64(v), true
		}
	case string:
		if u, err := strconv.ParseUint(v, 10, 64); err == nil {
			return u, true
		}
	case bool:
		if v {
			return 1, true
		}
		return 0, true
	}
	return 0, false
}

// ConvertToFloat64 converts any value to float64
func ConvertToFloat64(value any) (float64, bool) {
	switch v := value.(type) {
	case float64:
		return v, true
	case float32:
		return float64(v), true
	case int:
		return float64(v), true
	case int8:
		return float64(v), true
	case int16:
		return float64(v), true
	case int32:
		return float64(v), true
	case int64:
		return float64(v), true
	case uint:
		return float64(v), true
	case uint8:
		return float64(v), true
	case uint16:
		return float64(v), true
	case uint32:
		return float64(v), true
	case uint64:
		return float64(v), true
	case string:
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f, true
		}
	case bool:
		if v {
			return 1.0, true
		}
		return 0.0, true
	case json.Number:
		if f, err := v.Float64(); err == nil {
			return f, true
		}
	}
	return 0, false
}

// ConvertToString converts any value to string
func ConvertToString(value any) (string, bool) {
	switch v := value.(type) {
	case string:
		return v, true
	case int:
		return strconv.Itoa(v), true
	case int8:
		return strconv.FormatInt(int64(v), 10), true
	case int16:
		return strconv.FormatInt(int64(v), 10), true
	case int32:
		return strconv.FormatInt(int64(v), 10), true
	case int64:
		return strconv.FormatInt(v, 10), true
	case uint:
		return strconv.FormatUint(uint64(v), 10), true
	case uint8:
		return strconv.FormatUint(uint64(v), 10), true
	case uint16:
		return strconv.FormatUint(uint64(v), 10), true
	case uint32:
		return strconv.FormatUint(uint64(v), 10), true
	case uint64:
		return strconv.FormatUint(v, 10), true
	case float32:
		return strconv.FormatFloat(float64(v), 'g', -1, 32), true
	case float64:
		return strconv.FormatFloat(v, 'g', -1, 64), true
	case bool:
		return strconv.FormatBool(v), true
	case json.Number:
		return string(v), true
	case fmt.Stringer:
		return v.String(), true
	}
	return "", false
}

// ConvertToBool converts any value to bool
func ConvertToBool(value any) (bool, bool) {
	switch v := value.(type) {
	case bool:
		return v, true
	case int:
		return v != 0, true
	case int64:
		return v != 0, true
	case float64:
		return v != 0, true
	case string:
		if b, err := strconv.ParseBool(v); err == nil {
			return b, true
		}
		if v == "1" {
			return true, true
		}
		if v == "0" {
			return false, true
		}
	}
	return false, false
}

// FormatNumber formats a number value to string preserving its type
func FormatNumber(value any) string {
	switch v := value.(type) {
	case int, int8, int16, int32, int64:
		if s, ok := ConvertToString(v); ok {
			return s
		}
	case uint, uint8, uint16, uint32, uint64:
		if s, ok := ConvertToString(v); ok {
			return s
		}
	case float32, float64:
		if s, ok := ConvertToString(v); ok {
			return s
		}
	case json.Number:
		return string(v)
	case string:
		return v
	}
	return fmt.Sprintf("%v", value)
}

// SafeConvertToInt64 converts any value to int64 with error return
func SafeConvertToInt64(value any) (int64, error) {
	if result, ok := ConvertToInt64(value); ok {
		return result, nil
	}
	return 0, fmt.Errorf("cannot convert %T to int64", value)
}

// SafeConvertToUint64 converts any value to uint64 with error return
func SafeConvertToUint64(value any) (uint64, error) {
	if result, ok := ConvertToUint64(value); ok {
		return result, nil
	}
	return 0, fmt.Errorf("cannot convert %T to uint64", value)
}

// ============================================================================
// Generic Type Conversion (from type_conversion_unified.go)
// ============================================================================

// UnifiedTypeConversion provides a single entry point for all type conversions
// This replaces the scattered conversion logic in helpers.go and path.go
func UnifiedTypeConversion[T any](value any) (T, bool) {
	var zero T

	// Fast path: direct type match
	if typedValue, ok := value.(T); ok {
		return typedValue, true
	}

	// Handle array/slice conversions
	if converted, ok := convertArray[T](value); ok {
		return converted, true
	}

	// Use type switch for common conversions
	targetType := fmt.Sprintf("%T", zero)

	switch targetType {
	case "string":
		if str, ok := convertToStringUnified(value); ok {
			if result, ok := any(str).(T); ok {
				return result, true
			}
		}
	case "int":
		if intVal, ok := ConvertToInt(value); ok {
			if result, ok := any(intVal).(T); ok {
				return result, true
			}
		}
	case "int64":
		if int64Val, ok := ConvertToInt64(value); ok {
			if result, ok := any(int64Val).(T); ok {
				return result, true
			}
		}
	case "uint64":
		if uint64Val, ok := ConvertToUint64(value); ok {
			if result, ok := any(uint64Val).(T); ok {
				return result, true
			}
		}
	case "float64":
		if float64Val, ok := ConvertToFloat64(value); ok {
			if result, ok := any(float64Val).(T); ok {
				return result, true
			}
		}
	case "bool":
		if boolVal, ok := ConvertToBool(value); ok {
			if result, ok := any(boolVal).(T); ok {
				return result, true
			}
		}
	}

	return zero, false
}

// convertToStringUnified handles all string conversions including json.Number
func convertToStringUnified(value any) (string, bool) {
	switch v := value.(type) {
	case string:
		return v, true
	case json.Number:
		return string(v), true
	case int:
		return strconv.Itoa(v), true
	case int64:
		return strconv.FormatInt(v, 10), true
	case float64:
		return strconv.FormatFloat(v, 'g', -1, 64), true
	case bool:
		return strconv.FormatBool(v), true
	default:
		return ConvertToString(value)
	}
}

// convertArray handles array/slice type conversions
func convertArray[T any](value any) (T, bool) {
	var zero T

	targetType := reflect.TypeOf(zero)
	if targetType.Kind() != reflect.Slice {
		return zero, false
	}

	sourceSlice, ok := value.([]any)
	if !ok {
		return zero, false
	}

	elemType := targetType.Elem()
	resultSlice := reflect.MakeSlice(targetType, len(sourceSlice), len(sourceSlice))

	for i, elem := range sourceSlice {
		convertedElem, err := convertElement(elem, elemType)
		if err != nil {
			return zero, false
		}
		resultSlice.Index(i).Set(reflect.ValueOf(convertedElem))
	}

	if result, ok := resultSlice.Interface().(T); ok {
		return result, true
	}

	return zero, false
}

// convertElement converts a single element to the target type
func convertElement(elem any, targetType reflect.Type) (any, error) {
	if elem == nil {
		return reflect.Zero(targetType).Interface(), nil
	}

	if reflect.TypeOf(elem) == targetType {
		return elem, nil
	}

	switch targetType.Kind() {
	case reflect.String:
		if str, ok := convertToStringUnified(elem); ok {
			return str, nil
		}
	case reflect.Int:
		if i, ok := ConvertToInt(elem); ok {
			return i, nil
		}
	case reflect.Int64:
		if i, ok := ConvertToInt64(elem); ok {
			return i, nil
		}
	case reflect.Float64:
		if f, ok := ConvertToFloat64(elem); ok {
			return f, nil
		}
	case reflect.Bool:
		if b, ok := ConvertToBool(elem); ok {
			return b, nil
		}
	}

	return nil, fmt.Errorf("cannot convert %T to %v", elem, targetType)
}
