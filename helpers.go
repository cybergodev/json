package json

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"github.com/cybergodev/json/internal"
)

// ArrayHelper provides centralized array operation utilities
type ArrayHelper struct{}

// ParseArrayIndex parses an array index from a string
// Delegates to internal implementation for consistency
func (ah *ArrayHelper) ParseArrayIndex(indexStr string) int {
	indexStr = strings.Trim(indexStr, "[] \t")
	if indexStr == "" {
		return InvalidArrayIndex
	}

	if index, ok := internal.ParseArrayIndex(indexStr); ok {
		return index
	}
	return InvalidArrayIndex
}

// NormalizeIndex converts negative indices to positive indices
// Delegates to internal implementation for consistency
func (ah *ArrayHelper) NormalizeIndex(index, length int) int {
	return internal.NormalizeIndex(index, length)
}

// ValidateBounds checks if an index is within valid bounds [0, length)
// Note: This does NOT support negative indices (unlike IsValidIndex in internal)
func (ah *ArrayHelper) ValidateBounds(index, length int) bool {
	return index >= 0 && index < length
}

// ClampIndex clamps an index to valid bounds [0, length]
func (ah *ArrayHelper) ClampIndex(index, length int) int {
	if index < 0 {
		return 0
	}
	if index > length {
		return length
	}
	return index
}

// CompactArray removes nil values and deletion markers from an array
func (ah *ArrayHelper) CompactArray(arr []any) []any {
	if len(arr) == 0 {
		return arr
	}

	result := make([]any, 0, len(arr))
	for _, item := range arr {
		if item != nil && item != DeletedMarker {
			result = append(result, item)
		}
	}
	return result
}

// ExtendArray extends an array to the specified length, filling with nil values
func (ah *ArrayHelper) ExtendArray(arr []any, targetLength int) []any {
	if len(arr) >= targetLength {
		return arr
	}

	extended := make([]any, targetLength)
	copy(extended, arr)
	return extended
}

// GetElement safely gets an element from an array with bounds checking
// Delegates to internal implementation for consistency
func (ah *ArrayHelper) GetElement(arr []any, index int) (any, bool) {
	return internal.GetSafeArrayElement(arr, index)
}

// SetElement safely sets an element in an array with bounds checking
// Note: This does NOT support negative indices for bounds checking
func (ah *ArrayHelper) SetElement(arr []any, index int, value any) bool {
	normalizedIndex := ah.NormalizeIndex(index, len(arr))
	// Check bounds on normalized index
	if normalizedIndex < 0 || normalizedIndex >= len(arr) {
		return false
	}
	arr[normalizedIndex] = value
	return true
}

// PerformSlice performs array slicing with step support
// Delegates to internal implementation for consistency
func (ah *ArrayHelper) PerformSlice(arr []any, start, end, step int) []any {
	if len(arr) == 0 || step == 0 {
		return []any{}
	}

	// Convert to pointers for internal API
	startPtr := &start
	endPtr := &end
	stepPtr := &step

	return internal.PerformArraySlice(arr, startPtr, endPtr, stepPtr)
}

// Global array helper instance
var globalArrayHelper = &ArrayHelper{}

// ParseArrayIndexGlobal is a package-level function for backward compatibility
func ParseArrayIndexGlobal(indexStr string) int {
	return globalArrayHelper.ParseArrayIndex(indexStr)
}

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
		if i, err := strconv.ParseUint(v, 10, 64); err == nil {
			return i, true
		}
	case bool:
		if v {
			return 1, true
		}
		return 0, true
	case json.Number:
		if i, err := v.Int64(); err == nil && i >= 0 {
			return uint64(i), true
		}
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
	return 0.0, false
}

// ConvertToBool converts any value to bool
func ConvertToBool(value any) (bool, bool) {
	switch v := value.(type) {
	case bool:
		return v, true
	case int, int8, int16, int32, int64:
		return reflect.ValueOf(v).Int() != 0, true
	case uint, uint8, uint16, uint32, uint64:
		return reflect.ValueOf(v).Uint() != 0, true
	case float32, float64:
		return reflect.ValueOf(v).Float() != 0.0, true
	case string:
		switch strings.ToLower(v) {
		case "true", "1", "yes", "on":
			return true, true
		case "false", "0", "no", "off", "":
			return false, true
		}
	case json.Number:
		if f, err := v.Float64(); err == nil {
			return f != 0.0, true
		}
	}
	return false, false
}

// UnifiedTypeConversion provides optimized type conversion with comprehensive support
func UnifiedTypeConversion[T any](value any) (T, bool) {
	var zero T

	// Handle nil values
	if value == nil {
		return zero, true
	}

	// Direct type assertion (fastest path)
	if typedValue, ok := value.(T); ok {
		return typedValue, true
	}

	// Get target type information
	targetType := reflect.TypeOf(zero)
	if targetType == nil {
		return zero, false
	}

	// Handle pointer types
	if targetType.Kind() == reflect.Ptr {
		elemType := targetType.Elem()
		elemValue := reflect.New(elemType).Interface()
		if converted, ok := convertValue(value, elemValue); ok {
			if result, ok := converted.(T); ok {
				return result, true
			}
		}
		return zero, false
	}

	// Convert to target type
	if converted, ok := convertValue(value, zero); ok {
		if result, ok := converted.(T); ok {
			return result, true
		}
	}

	return zero, false
}

// convertValue handles the actual conversion logic
func convertValue(value any, target any) (any, bool) {
	targetType := reflect.TypeOf(target)

	switch targetType.Kind() {
	case reflect.String:
		// Inline string conversion - fix order to handle json.Number before fmt.Stringer
		switch v := value.(type) {
		case string:
			return v, true
		case []byte:
			return string(v), true
		case json.Number:
			return string(v), true
		case fmt.Stringer:
			return v.String(), true
		default:
			return fmt.Sprintf("%v", v), true
		}
	case reflect.Int:
		if i, ok := ConvertToInt(value); ok {
			return i, true
		}
	case reflect.Int64:
		if i, ok := ConvertToInt64(value); ok {
			return i, true
		}
	case reflect.Uint64:
		if i, ok := ConvertToUint64(value); ok {
			return i, true
		}
	case reflect.Float64:
		if f, ok := ConvertToFloat64(value); ok {
			return f, true
		}
	case reflect.Bool:
		if b, ok := ConvertToBool(value); ok {
			return b, true
		}
	case reflect.Slice:
		if s, ok := convertToSlice(value, targetType); ok {
			return s, true
		}
	case reflect.Map:
		if m, ok := convertToMap(value, targetType); ok {
			return m, true
		}
	}

	return nil, false
}

// convertToSlice converts value to slice type
func convertToSlice(value any, targetType reflect.Type) (any, bool) {
	rv := reflect.ValueOf(value)
	if rv.Kind() != reflect.Slice && rv.Kind() != reflect.Array {
		return nil, false
	}

	elemType := targetType.Elem()
	result := reflect.MakeSlice(targetType, rv.Len(), rv.Len())

	for i := 0; i < rv.Len(); i++ {
		elem := rv.Index(i).Interface()
		if converted, ok := convertValue(elem, reflect.Zero(elemType).Interface()); ok {
			result.Index(i).Set(reflect.ValueOf(converted))
		} else {
			return nil, false
		}
	}

	return result.Interface(), true
}

// convertToMap converts value to map type
func convertToMap(value any, targetType reflect.Type) (any, bool) {
	rv := reflect.ValueOf(value)
	if rv.Kind() != reflect.Map {
		return nil, false
	}

	keyType := targetType.Key()
	elemType := targetType.Elem()
	result := reflect.MakeMap(targetType)

	for _, key := range rv.MapKeys() {
		keyInterface := key.Interface()
		valueInterface := rv.MapIndex(key).Interface()

		convertedKey, keyOk := convertValue(keyInterface, reflect.Zero(keyType).Interface())
		convertedValue, valueOk := convertValue(valueInterface, reflect.Zero(elemType).Interface())

		if keyOk && valueOk {
			result.SetMapIndex(reflect.ValueOf(convertedKey), reflect.ValueOf(convertedValue))
		} else {
			return nil, false
		}
	}

	return result.Interface(), true
}

// SafeConvertToInt64 safely converts any value to int64 with error handling
func SafeConvertToInt64(value any) (int64, error) {
	if result, ok := ConvertToInt64(value); ok {
		return result, nil
	}
	return 0, fmt.Errorf("cannot convert %T to int64", value)
}

// SafeConvertToUint64 safely converts any value to uint64 with error handling
func SafeConvertToUint64(value any) (uint64, error) {
	if result, ok := ConvertToUint64(value); ok {
		return result, nil
	}
	return 0, fmt.Errorf("cannot convert %T to uint64", value)
}

// FormatNumber formats a number value as a string
func FormatNumber(value any) string {
	switch v := value.(type) {
	case int:
		return strconv.Itoa(v)
	case int64:
		return strconv.FormatInt(v, 10)
	case uint64:
		return strconv.FormatUint(v, 10)
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	case json.Number:
		return string(v)
	default:
		return fmt.Sprintf("%v", v)
	}
}

// ConvertToString converts any value to string (for backward compatibility)
func ConvertToString(value any) string {
	switch v := value.(type) {
	case string:
		return v
	case []byte:
		return string(v)
	case json.Number:
		return string(v)
	case fmt.Stringer:
		return v.String()
	default:
		return fmt.Sprintf("%v", v)
	}
}

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
