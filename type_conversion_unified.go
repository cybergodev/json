package json

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
)

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
