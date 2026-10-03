package json

import (
	"encoding/json"
	"fmt"
	"maps"
	"math"
	"reflect"
	"slices"
	"strconv"
	"strings"

	"github.com/cybergodev/json/internal"
)

// ============================================================================
// TYPE CONVERSION HELPERS
// Internal helpers to reduce code duplication while maintaining
// zero-allocation type switches for performance-critical paths.
// ============================================================================

// Platform-adaptive int range constants for correct convertToInt behavior
const (
	minInt = -1 << (strconv.IntSize - 1)
	maxInt = 1<<(strconv.IntSize-1) - 1
)

// int64Result holds the result of integer conversion to avoid multiple returns
type int64Result struct {
	value int64
	ok    bool
}

// convertToInt64Core is the internal core function for integer conversion.
// Uses a single type switch with all integer types to minimize branching.
// MAINTENANCE: Keep type cases in sync with convertToFloatCore for consistency.
func convertToInt64Core(value any) int64Result {
	switch v := value.(type) {
	case int:
		return int64Result{int64(v), true}
	case int8:
		return int64Result{int64(v), true}
	case int16:
		return int64Result{int64(v), true}
	case int32:
		return int64Result{int64(v), true}
	case int64:
		return int64Result{v, true}
	case uint:
		u64 := uint64(v)
		if u64 > uint64(math.MaxInt64) {
			return int64Result{0, false}
		}
		return int64Result{int64(u64), true}
	case uint8:
		return int64Result{int64(v), true}
	case uint16:
		return int64Result{int64(v), true}
	case uint32:
		return int64Result{int64(v), true}
	case uint64:
		if v <= uint64(math.MaxInt64) {
			return int64Result{int64(v), true}
		}
		return int64Result{0, false}
	}
	return int64Result{0, false}
}

// convertToFloatCore handles float-specific type conversion.
// MAINTENANCE: Keep type cases in sync with convertToInt64Core for consistency.
func convertToFloatCore(value any) (float64, bool) {
	switch v := value.(type) {
	case float32:
		return float64(v), true
	case float64:
		return v, true
	}
	return 0, false
}

// convertToInt converts any value to int with comprehensive type support.
// Returns (value, true) on success, (0, false) on failure.
//
// Supported types: int*, uint*, float*, string, bool, json.Number
// Float values are truncated (not rounded) if they have no fractional part.
// Bool converts: true -> 1, false -> 0.
// String values are parsed as base-10 integers.
//
// Example:
//
//	i, ok := json.convertToInt("42")    // i=42, ok=true
//	i, ok := json.convertToInt(3.0)     // i=3, ok=true
//	i, ok := json.convertToInt(3.14)    // i=0, ok=false (has fractional part)
//	i, ok := json.convertToInt(true)    // i=1, ok=true
//	i, ok := json.convertToInt("abc")   // i=0, ok=false
func convertToInt(value any) (int, bool) {
	// Fast path: use core integer conversion
	if result := convertToInt64Core(value); result.ok {
		if result.value >= int64(minInt) && result.value <= int64(maxInt) {
			return int(result.value), true
		}
		return 0, false
	}

	// Handle non-integer types
	switch v := value.(type) {
	case float32:
		// Bounds BEFORE the conversion: int(v) is implementation-defined for
		// out-of-range/NaN/Inf inputs (see convertToInt64). float64(maxInt)+1
		// is exactly 2^63 (2^31 on 32-bit int), giving a strict upper bound.
		f := float64(v)
		if f >= float64(minInt) && f < float64(maxInt)+1 && v == float32(int(v)) {
			return int(v), true
		}
	case float64:
		if v >= float64(minInt) && v < float64(maxInt)+1 && v == float64(int(v)) {
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
		if i, err := v.Int64(); err == nil && i >= int64(minInt) && i <= int64(maxInt) {
			return int(i), true
		}
	}
	return 0, false
}

// convertToInt64 converts any value to int64 with comprehensive type support.
// Returns (value, true) on success, (0, false) on failure.
//
// Supported types: int*, uint*, float*, string, bool, json.Number
// Float values are truncated if they have no fractional part.
// Bool converts: true -> 1, false -> 0.
//
// Example:
//
//	i, ok := json.convertToInt64("9223372036854775807")  // i=9223372036854775807, ok=true
//	i, ok := json.convertToInt64(int32(100))             // i=100, ok=true
func convertToInt64(value any) (int64, bool) {
	// Fast path: use core integer conversion
	if result := convertToInt64Core(value); result.ok {
		return result.value, true
	}

	// Handle non-integer types
	switch v := value.(type) {
	case float32:
		// Explicit bounds first: int64(v) is implementation-defined for
		// out-of-range/NaN/Inf inputs, and on saturating platforms (arm64)
		// a round-trip check alone lets 2^63 pass as MaxInt64. NaN and ±Inf
		// fail the range comparison and are rejected.
		f := float64(v)
		if f >= -9223372036854775808.0 && f < 9223372036854775808.0 && v == float32(int64(v)) {
			return int64(v), true
		}
	case float64:
		if v >= -9223372036854775808.0 && v < 9223372036854775808.0 && v == float64(int64(v)) {
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

// convertToUint64 converts any value to uint64 with comprehensive type support.
// Returns (value, true) on success, (0, false) on failure.
// Negative values always fail conversion.
//
// Supported types: int*, uint*, float*, string, bool, json.Number
// Float values are converted if >= 0 and have no fractional part.
//
// Example:
//
//	u, ok := json.convertToUint64("18446744073709551615")  // max uint64, ok=true
//	u, ok := json.convertToUint64(-1)                      // u=0, ok=false (negative)
//	u, ok := json.convertToUint64(uint(42))                // u=42, ok=true
func convertToUint64(value any) (uint64, bool) {
	// Special case: uint64 needs direct handling for values > int64 max
	switch v := value.(type) {
	case uint64:
		return v, true
	case uint:
		return uint64(v), true
	}

	// Fast path: use core integer conversion for other signed types
	if result := convertToInt64Core(value); result.ok && result.value >= 0 {
		return uint64(result.value), true
	}

	// Handle non-integer types
	switch v := value.(type) {
	case float32:
		// Explicit upper bound: uint64(v) is implementation-defined for
		// out-of-range/NaN/Inf inputs; on saturating platforms the old
		// round-trip check let 2^64 pass as MaxUint64.
		f := float64(v)
		if f >= 0 && f < 18446744073709551616.0 && v == float32(uint64(v)) {
			return uint64(v), true
		}
	case float64:
		if v >= 0 && v < 18446744073709551616.0 && v == float64(uint64(v)) {
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
		// Values in (MaxInt64, MaxUint64] fit uint64 but overflow Int64; fall
		// back to ParseUint so this branch agrees with the string branch above.
		if u, err := strconv.ParseUint(string(v), 10, 64); err == nil {
			return u, true
		}
	}
	return 0, false
}

// convertToFloat64 converts any value to float64 with comprehensive type support.
// Returns (value, true) on success, (0.0, false) on failure.
//
// Supported types: int*, uint*, float*, string, bool, json.Number
// Bool converts: true -> 1.0, false -> 0.0.
// String values are parsed as floating-point numbers.
//
// Example:
//
//	f, ok := json.convertToFloat64("3.14")   // f=3.14, ok=true
//	f, ok := json.convertToFloat64(42)       // f=42.0, ok=true
//	f, ok := json.convertToFloat64(true)     // f=1.0, ok=true
//	f, ok := json.convertToFloat64("abc")    // f=0.0, ok=false
func convertToFloat64(value any) (float64, bool) {
	// uint64/uint values above MaxInt64 cannot route through the int64 core
	// but are trivially representable as float64.
	switch v := value.(type) {
	case uint64:
		return float64(v), true
	case uint:
		return float64(v), true
	}

	// Fast path 1: use core integer conversion
	if result := convertToInt64Core(value); result.ok {
		return float64(result.value), true
	}

	// Fast path 2: use core float conversion
	if f, ok := convertToFloatCore(value); ok {
		return f, true
	}

	// Handle non-numeric types
	switch v := value.(type) {
	case string:
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			// strconv accepts "NaN"/"Inf"/"Infinity" tokens; JSON cannot
			// represent them, so treat them as failed conversions and let
			// callers fall back to their documented default value.
			if math.IsNaN(f) || math.IsInf(f, 0) {
				return 0.0, false
			}
			return f, true
		}
	case bool:
		if v {
			return 1.0, true
		}
		return 0.0, true
	case json.Number:
		if f, err := v.Float64(); err == nil {
			if math.IsNaN(f) || math.IsInf(f, 0) {
				return 0.0, false
			}
			return f, true
		}
	}
	return 0.0, false
}

// convertToBool converts any value to bool with comprehensive type support.
// Returns (value, true) on success, (false, false) on failure.
//
// Supported types and conversions:
//   - bool: returned as-is
//   - int*, uint*, float*: 0 -> false, non-zero -> true
//   - string: parses using strconv.ParseBool plus extended formats
//   - json.Number: 0 -> false, non-zero -> true
//
// Extended string formats:
//   - "yes", "on" -> true
//   - "no", "off", "" -> false
//   - Standard: "1", "t", "T", "TRUE", "true", "True", "0", "f", "F", "FALSE", "false", "False"
//
// Example:
//
//	b, ok := json.convertToBool("true")   // b=true, ok=true
//	b, ok := json.convertToBool("yes")    // b=true, ok=true
//	b, ok := json.convertToBool(1)        // b=true, ok=true
//	b, ok := json.convertToBool(0)        // b=false, ok=true
//	b, ok := json.convertToBool("maybe")  // b=false, ok=false
func convertToBool(value any) (bool, bool) {
	// Fast path: use core integer conversion for numeric types
	if result := convertToInt64Core(value); result.ok {
		return result.value != 0, true
	}

	// Handle non-integer types
	switch v := value.(type) {
	case bool:
		return v, true
	case float32:
		return v != 0.0, true
	case float64:
		return v != 0.0, true
	case string:
		// First try standard library format
		if result, err := strconv.ParseBool(v); err == nil {
			return result, true
		}
		// Then try extended user-friendly formats
		switch strings.ToLower(v) {
		case "yes", "on":
			return true, true
		case "no", "off", "":
			return false, true
		}
	case json.Number:
		if f, err := v.Float64(); err == nil {
			return f != 0.0, true
		}
	}
	return false, false
}

// getTypedWithDefault retrieves a typed value from JSON using a specific processor.
// Returns defaultValue if: path not found, value is null, or type conversion fails.
func getTypedWithDefault[T any](processor *Processor, jsonStr, path string, defaultValue ...T) T {
	var def T
	if len(defaultValue) > 0 {
		def = defaultValue[0]
	}
	rawValue, err := processor.Get(jsonStr, path)
	if err != nil {
		return def
	}
	if rawValue == nil {
		return def
	}
	result, err := convertToTypedCore[T](rawValue, path)
	if err != nil {
		return def
	}
	return result
}

// unifiedTypeConversion provides optimized type conversion with comprehensive support
func unifiedTypeConversion[T any](value any) (T, bool) {
	var zero T

	// Handle nil values
	if value == nil {
		return zero, true
	}

	// Direct type assertion (fastest path)
	if typedValue, ok := value.(T); ok {
		return typedValue, true
	}

	// Unwrap single-element arrays when converting to non-slice target types.
	// This handles distributed property access (e.g., "choices.message.content")
	// where an array with one element produces a single-element []any result
	// that should be unwrapped for typed getters like GetString/GetInt/etc.
	if arr, ok := value.([]any); ok && len(arr) == 1 {
		targetType := reflect.TypeOf(zero)
		if targetType != nil && targetType.Kind() != reflect.Slice {
			return unifiedTypeConversion[T](arr[0])
		}
	}

	// Get target type information
	targetType := reflect.TypeOf(zero)
	if targetType == nil {
		return zero, false
	}

	// Handle pointer types
	if targetType.Kind() == reflect.Pointer {
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
	return convertValueWithDepth(value, target, 0)
}

// convertValueWithDepth handles conversion with depth tracking to prevent stack overflow.
func convertValueWithDepth(value any, target any, depth int) (any, bool) {
	if depth > deepCopyMaxDepth {
		return nil, false
	}

	targetType := reflect.TypeOf(target)

	switch targetType.Kind() {
	case reflect.String:
		// Single implementation shared with IterableValue (order matters:
		// json.Number before fmt.Stringer).
		return convertToString(value), true
	case reflect.Int:
		if i, ok := convertToInt(value); ok {
			return i, true
		}
	case reflect.Int64:
		if i, ok := convertToInt64(value); ok {
			return i, true
		}
	case reflect.Uint64:
		if i, ok := convertToUint64(value); ok {
			return i, true
		}
	case reflect.Float64:
		if f, ok := convertToFloat64(value); ok {
			return f, true
		}
	case reflect.Bool:
		if b, ok := convertToBool(value); ok {
			return b, true
		}
	case reflect.Slice:
		if s, ok := convertToSliceWithDepth(value, targetType, depth); ok {
			return s, true
		}
	case reflect.Map:
		if m, ok := convertToMapWithDepth(value, targetType, depth); ok {
			return m, true
		}
	}

	return nil, false
}

func convertToSliceWithDepth(value any, targetType reflect.Type, depth int) (any, bool) {
	rv := reflect.ValueOf(value)
	if rv.Kind() != reflect.Slice && rv.Kind() != reflect.Array {
		return nil, false
	}

	elemType := targetType.Elem()
	result := reflect.MakeSlice(targetType, rv.Len(), rv.Len())

	for i := 0; i < rv.Len(); i++ {
		elem := rv.Index(i).Interface()
		if converted, ok := convertValueWithDepth(elem, reflect.Zero(elemType).Interface(), depth+1); ok {
			result.Index(i).Set(reflect.ValueOf(converted))
		} else {
			return nil, false
		}
	}

	return result.Interface(), true
}

func convertToMapWithDepth(value any, targetType reflect.Type, depth int) (any, bool) {
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

		convertedKey, keyOk := convertValueWithDepth(keyInterface, reflect.Zero(keyType).Interface(), depth+1)
		convertedValue, valueOk := convertValueWithDepth(valueInterface, reflect.Zero(elemType).Interface(), depth+1)

		if keyOk && valueOk {
			result.SetMapIndex(reflect.ValueOf(convertedKey), reflect.ValueOf(convertedValue))
		} else {
			return nil, false
		}
	}

	return result.Interface(), true
}

// convertToString converts any value to its string representation.
// Handles string, []byte, json.Number, fmt.Stringer, and falls back to fmt.Sprintf.
func convertToString(value any) string {
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

// isValidJSON quickly checks if a string is valid JSON without a Processor.
// For detailed error information, use Processor.Valid() instead.
func isValidJSON(jsonStr string) bool {
	decoder := newNumberPreservingDecoder(false)
	_, err := decoder.DecodeToAny(jsonStr)
	return err == nil
}

// containerGetProperty retrieves a property from a map container (map[string]any or map[any]any).
// Returns the value and true if found, nil and false otherwise.
func containerGetProperty(container any, key string) (any, bool) {
	switch m := container.(type) {
	case map[string]any:
		v, ok := m[key]
		return v, ok
	case map[any]any:
		v, ok := m[key]
		return v, ok
	}
	return nil, false
}

// containerDeleteProperty deletes a property from a map container (map[string]any or map[any]any).
// Returns true if the property was found and deleted.
func containerDeleteProperty(container any, key string) bool {
	switch m := container.(type) {
	case map[string]any:
		if _, ok := m[key]; ok {
			delete(m, key)
			return true
		}
	case map[any]any:
		if _, ok := m[key]; ok {
			delete(m, key)
			return true
		}
	}
	return false
}

// containerSetProperty sets a property on a map container (map[string]any or map[any]any).
// Returns true if the container type was recognized.
func containerSetProperty(container any, key string, value any) bool {
	switch m := container.(type) {
	case map[string]any:
		m[key] = value
		return true
	case map[any]any:
		m[key] = value
		return true
	}
	return false
}

// containerIsMap checks if a value is a map[string]any or map[any]any.
func containerIsMap(v any) bool {
	switch v.(type) {
	case map[string]any, map[any]any:
		return true
	}
	return false
}

// deepCopyMaxDepth is the maximum recursion depth for DeepCopy operations.
// SECURITY: Prevents stack overflow from deeply nested structures.
const deepCopyMaxDepth = 200

// DeepCopy creates a deep copy of JSON-compatible data
// Uses direct recursive copying for better performance (avoids marshal/unmarshal overhead)
// SECURITY: Added depth limit to prevent stack overflow
func deepCopy(data any) (any, error) {
	// deepCopySubtree handles JSON trees (Tiers 0-2) and falls back to
	// deepCopyValueWithDepth for non-JSON types (Tier 3) itself; both legs
	// only fail past the depth limit, where the fallback below would fail
	// identically. Kept as a guard so a future Tier-3 regression still
	// produces a copy attempt rather than propagating the depth error.
	if result, err := deepCopySubtree(data); err == nil {
		return result, nil
	}
	return deepCopyValueWithDepth(data, 0)
}

// deepCopyValueWithDepth performs recursive deep copy with depth tracking
// SECURITY: Depth parameter prevents stack overflow from deeply nested structures
func deepCopyValueWithDepth(data any, depth int) (any, error) {
	// SECURITY: Check depth limit to prevent stack overflow
	if depth > deepCopyMaxDepth {
		return nil, fmt.Errorf("deep copy depth limit exceeded: maximum depth is %d", deepCopyMaxDepth)
	}

	if data == nil {
		return nil, nil
	}

	// Fast path for primitive types (no allocation needed)
	switch v := data.(type) {
	case bool:
		return v, nil
	case int:
		return v, nil
	case int8:
		return v, nil
	case int16:
		return v, nil
	case int32:
		return v, nil
	case int64:
		return v, nil
	case uint:
		return v, nil
	case uint8:
		return v, nil
	case uint16:
		return v, nil
	case uint32:
		return v, nil
	case uint64:
		return v, nil
	case float32:
		return v, nil
	case float64:
		return v, nil
	case string:
		return v, nil
	case json.Number:
		// json.Number is immutable, return as-is
		return v, nil
	case Number:
		// Number (type Number string) is immutable, return as-is
		return v, nil
	}

	// Handle complex types with type-specific optimizations
	switch v := data.(type) {
	case map[string]any:
		return deepCopyMapWithDepth(v, depth)
	case []any:
		return deepCopySliceWithDepth(v, depth)
	case map[string]string:
		// Fast path for map[string]string - no recursion needed
		result := make(map[string]string, len(v))
		maps.Copy(result, v)
		return result, nil
	case []string:
		// Fast path for []string - no recursion needed
		result := make([]string, len(v))
		copy(result, v)
		return result, nil
	case []int:
		// Fast path for []int - no recursion needed
		result := make([]int, len(v))
		copy(result, v)
		return result, nil
	case []float64:
		// Fast path for []float64 - no recursion needed
		result := make([]float64, len(v))
		copy(result, v)
		return result, nil
	case []bool:
		// Fast path for []bool - no recursion needed
		result := make([]bool, len(v))
		copy(result, v)
		return result, nil
	default:
		// Fallback to marshal/unmarshal for unknown types (structs, custom types, etc.)
		jsonBytes, err := json.Marshal(data)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal data for deep copy: %w", err)
		}
		var result any
		if err := json.Unmarshal(jsonBytes, &result); err != nil {
			return nil, fmt.Errorf("failed to unmarshal data for deep copy: %w", err)
		}
		return result, nil
	}
}

// deepCopyMapWithDepth creates a deep copy of a map with depth tracking
// SECURITY: Pass depth to recursive calls for stack overflow protection
func deepCopyMapWithDepth(m map[string]any, depth int) (map[string]any, error) {
	result := make(map[string]any, len(m))
	for key, val := range m {
		copied, err := deepCopyValueWithDepth(val, depth+1)
		if err != nil {
			return nil, fmt.Errorf("error copying key '%s': %w", key, err)
		}
		result[key] = copied
	}
	return result, nil
}

// sortedMapKeys returns the keys of m in ascending order. Whenever results or
// callback order are collected from a map[string]any (Get wildcard/distributed
// results, ForEach/Iterate over objects), iteration must be deterministic:
// Go randomizes map iteration order per iteration, so the same operation on the
// same input previously produced a different element order on every call
// (empirically 293 distinct orderings in 300 runs on a 12-key object).
// Copy-building loops (deep copy, merge) don't need this — their output is a
// map, whose order is immaterial.
func sortedMapKeys(m map[string]any) []string {
	return slices.Sorted(maps.Keys(m))
}

// deepCopySliceWithDepth creates a deep copy of a slice with depth tracking
// SECURITY: Pass depth to recursive calls for stack overflow protection
func deepCopySliceWithDepth(s []any, depth int) ([]any, error) {
	result := make([]any, len(s))
	for i, val := range s {
		copied, err := deepCopyValueWithDepth(val, depth+1)
		if err != nil {
			return nil, fmt.Errorf("error copying index %d: %w", i, err)
		}
		result[i] = copied
	}
	return result, nil
}

// deepCopySubtree performs an optimized deep copy of only the returned subtree.
// This is significantly faster than deepCopy for large documents where Get
// returns a small portion, because it only copies the actual result value
// instead of the entire cached document.
// PERFORMANCE v3: JSON-specialized fast path avoids broad type switches.
//   - Tier 0: nil → immediate return
//   - Tier 1: JSON primitives (bool, float64, string, json.Number) → zero allocation
//   - Tier 2: map[string]any / []any → specialized inline copy without error wrapping
//   - Tier 3: Fallback for non-JSON types
//
// safeCopyResult returns a safe copy of result for JSON primitives or a deep copy for containers.
// Prevents callers from corrupting cached data.
func safeCopyResult(result any) any {
	switch result.(type) {
	case nil, bool, float64, string, int, int64, uint, uint64, json.Number, Number:
		return result
	}
	copied, err := deepCopySubtree(result)
	if err != nil {
		// deepCopySubtree only fails past the depth limit (>200), i.e. on
		// pathological input. Returning the original would hand the caller a
		// mutable reference into cached parse data and corrupt the cache for
		// subsequent readers; return nil instead.
		return nil
	}
	return copied
}

func deepCopySubtree(data any) (any, error) {
	return deepCopySubtreeWithDepth(data, 0)
}

func deepCopySubtreeWithDepth(data any, depth int) (any, error) {
	if depth > deepCopyMaxDepth {
		return nil, fmt.Errorf("deep copy depth limit exceeded: maximum depth is %d", deepCopyMaxDepth)
	}

	// Tier 1: JSON primitives — immutable, return as-is.
	switch v := data.(type) {
	case nil:
		return nil, nil
	case bool:
		return v, nil
	case float64:
		return v, nil
	case string:
		return v, nil
	case json.Number:
		return v, nil
	case Number:
		// Number (type Number string) is a distinct named type not matched by
		// `case string`; without this it falls through to the marshal fallback
		// and is corrupted to a quoted JSON string. Immutable — return as-is.
		return v, nil
	}

	// Tier 2: JSON containers — specialized inline copy
	switch v := data.(type) {
	case map[string]any:
		return deepCopyJSONMapWithDepth(v, depth+1)
	case []any:
		return deepCopyJSONSliceWithDepth(v, depth+1)
	}

	// Tier 3: Fallback for non-JSON types (int slices, custom types, etc.)
	// NOTE: Calls deepCopyValueWithDepth directly (not deepCopy) to break the
	// circular call chain: deepCopy → deepCopySubtree → deepCopy.
	return deepCopyValueWithDepth(data, depth)
}

// deepCopyJSONMap copies a map[string]any that contains only JSON-compatible values.
// PERFORMANCE v2: Inlined leaf-value copy avoids function call overhead for the
// common case where most values are JSON primitives (bool, float64, string).
// No fmt.Errorf wrapping — errors propagate directly.
func deepCopyJSONMapWithDepth(m map[string]any, depth int) (map[string]any, error) {
	if depth > deepCopyMaxDepth {
		return nil, fmt.Errorf("deep copy depth limit exceeded: maximum depth is %d", deepCopyMaxDepth)
	}
	result := make(map[string]any, len(m))
	for k, v := range m {
		// Inline leaf-value handling to avoid deepCopyJSONValueWithDepth call overhead
		// for the common case (~80% of values are primitives in typical JSON).
		switch val := v.(type) {
		case nil, bool, float64, string, json.Number, Number:
			// Immutable leaf: copy the interface word-pair directly. Routing these
			// through a local `any` (`copied = val`) would unbox and then re-box the
			// value on the heap once per element; copying the existing interface value
			// avoids that allocation and remains correct because immutable data is safe
			// to share between the cached original and the copy.
			result[k] = v
		case map[string]any:
			c, err := deepCopyJSONMapWithDepth(val, depth+1)
			if err != nil {
				return nil, err
			}
			result[k] = c
		case []any:
			c, err := deepCopyJSONSliceWithDepth(val, depth+1)
			if err != nil {
				return nil, err
			}
			result[k] = c
		default:
			c, err := deepCopyValueWithDepth(v, depth+1)
			if err != nil {
				return nil, err
			}
			result[k] = c
		}
	}
	return result, nil
}

// deepCopyJSONSlice copies a []any that contains only JSON-compatible values.
// PERFORMANCE v2: Same inline leaf-value optimization as deepCopyJSONMapWithDepth.
func deepCopyJSONSliceWithDepth(s []any, depth int) ([]any, error) {
	if depth > deepCopyMaxDepth {
		return nil, fmt.Errorf("deep copy depth limit exceeded: maximum depth is %d", deepCopyMaxDepth)
	}
	result := make([]any, len(s))
	for i, v := range s {
		switch val := v.(type) {
		case nil, bool, float64, string, json.Number, Number:
			// Immutable leaf: copy the interface word-pair directly to avoid the
			// unbox→rebox heap allocation (see deepCopyJSONMapWithDepth for rationale).
			result[i] = v
		case map[string]any:
			c, err := deepCopyJSONMapWithDepth(val, depth+1)
			if err != nil {
				return nil, err
			}
			result[i] = c
		case []any:
			c, err := deepCopyJSONSliceWithDepth(val, depth+1)
			if err != nil {
				return nil, err
			}
			result[i] = c
		default:
			c, err := deepCopyValueWithDepth(v, depth+1)
			if err != nil {
				return nil, err
			}
			result[i] = c
		}
	}
	return result, nil
}

// CompareJSON compares two JSON strings for equality by parsing and normalizing them.
// This function handles numeric precision differences and key ordering.
//
// Without config, calling json.CompareJSON(a, b) is unchanged. Pass an optional
// Config to apply security validation (size/depth/dangerous-pattern limits) to
// both inputs and to control the encoding used for the symmetric comparison
// (e.g. EscapeHTML).
//
// Example:
//
//	// Default behavior (no config)
//	equal, err := json.CompareJSON(`{"a":1}`, `{"a":1.0}`)
//	// equal == true
//
//	// With configuration (non-breaking, optional trailing Config)
//	equal, err = json.CompareJSON(a, b, json.SecurityConfig())
//
// Errors:
//   - an error wrapping the parse failure if either argument is not valid JSON
//   - an error if number normalization or marshaling fails
//   - when config is supplied, an error if either argument fails security validation
func CompareJSON(json1, json2 string, cfg ...Config) (bool, error) {
	// Without config, behavior is unchanged: no security validation and encoding/json
	// marshaling on both sides. With config, delegate to Processor.CompareJSON (built
	// from cfg) so size/depth/dangerous-pattern limits and the configured encoding
	// apply symmetrically.
	if len(cfg) == 0 {
		return compareJSONCore(json1, json2, func(v any) ([]byte, error) {
			return json.Marshal(v)
		})
	}
	p, err := getProcessorWithConfig(cfg[0])
	if err != nil {
		return false, err
	}
	return p.CompareJSON(json1, json2, cfg...)
}

// CompareJSON is the Processor method equivalent of the package-level CompareJSON.
// Unlike the package function's no-config path, the method always runs security
// validation — against the supplied cfg when given, otherwise against the processor's
// own configuration — and marshals both sides with the library encoder so that the
// configured encoding (e.g. EscapeHTML) applies symmetrically.
//
// Example:
//
//	equal, err := processor.CompareJSON(a, b)
//	equal, err = processor.CompareJSON(a, b, json.SecurityConfig())
//
// Errors:
//   - an error wrapping the parse failure if either argument is not valid JSON
//   - an error if number normalization or marshaling fails
//   - an error if either argument fails security validation
func (p *Processor) CompareJSON(json1, json2 string, cfg ...Config) (bool, error) {
	if err := p.checkClosed(); err != nil {
		return false, err
	}

	options, err := p.prepareOptions(cfg...)
	if err != nil {
		return false, err
	}
	defer releaseConfig(options)

	if err := p.validateInputForOptions(json1, options); err != nil {
		return false, fmt.Errorf("invalid JSON in first argument: %w", err)
	}
	if err := p.validateInputForOptions(json2, options); err != nil {
		return false, fmt.Errorf("invalid JSON in second argument: %w", err)
	}

	return compareJSONCore(json1, json2, func(v any) ([]byte, error) {
		return Marshal(v, *options)
	})
}

// compareJSONCore decodes both inputs with number preservation, normalizes the
// library's Number type to float64 (so 1 and 1.0 compare equal), marshals both sides
// with the same marshal function, and reports byte-identity. It performs no security
// validation; callers validate inputs as needed. Using one marshal function for both
// sides keeps the comparison symmetric regardless of which encoder is chosen.
func compareJSONCore(json1, json2 string, marshal func(any) ([]byte, error)) (bool, error) {
	decoder := newNumberPreservingDecoder(true)

	data1, err := decoder.DecodeToAny(json1)
	if err != nil {
		return false, fmt.Errorf("invalid JSON in first argument: %w", err)
	}

	data2, err := decoder.DecodeToAny(json2)
	if err != nil {
		return false, fmt.Errorf("invalid JSON in second argument: %w", err)
	}

	// Normalize Number to float64 so that 1 and 1.0 compare equal
	data1, err = convertLibraryNumbers(data1)
	if err != nil {
		return false, fmt.Errorf("number conversion failed in first argument: %w", err)
	}
	data2, err = convertLibraryNumbers(data2)
	if err != nil {
		return false, fmt.Errorf("number conversion failed in second argument: %w", err)
	}

	bytes1, err := marshal(data1)
	if err != nil {
		return false, err
	}

	bytes2, err := marshal(data2)
	if err != nil {
		return false, err
	}

	return string(bytes1) == string(bytes2), nil
}

// MergeJSON merges two JSON objects using deep merge strategy.
// For nested objects, it recursively merges keys according to Config.MergeMode.
// For primitive values and arrays, the value from json2 takes precedence.
//
// Merge modes (Config.MergeMode, defaults to MergeUnion):
//   - MergeUnion: combines all keys from both objects (default)
//   - MergeIntersection: only keys present in both objects
//   - MergeDifference: keys in json1 but not in json2
//
// Example:
//
//	// Union merge (default)
//	result, err := json.MergeJSON(a, b)
//
//	// Intersection merge
//	cfg := json.DefaultConfig()
//	cfg.MergeMode = json.MergeIntersection
//	result, err := json.MergeJSON(a, b, cfg)
//
//	// Difference merge
//	cfg.MergeMode = json.MergeDifference
//	result, err := json.MergeJSON(a, b, cfg)
//
// Errors:
//   - an error if either argument is not valid JSON
//   - an error if either argument is not a JSON object
//   - an error if number conversion or final encoding fails
func MergeJSON(json1, json2 string, cfg ...Config) (string, error) {
	return withProcessor(func(p *Processor) (string, error) {
		return p.MergeJSON(json1, json2, cfg...)
	})
}

// MergeJSON is the Processor method equivalent of the package-level MergeJSON.
// It resolves options from cfg (or the processor's own configuration when cfg is
// omitted), deep-merges the two objects under Config.MergeMode, and re-encodes the
// result with this processor.
//
// Like the package-level function, MergeJSON performs no security validation — it is
// a structural utility that only decodes, deep-merges, and re-encodes. CompareJSON, by
// contrast, validates when configuration is supplied.
//
// Example:
//
//	// Union merge (default)
//	result, err := processor.MergeJSON(a, b)
//
//	// Intersection merge
//	cfg := json.DefaultConfig()
//	cfg.MergeMode = json.MergeIntersection
//	result, err = processor.MergeJSON(a, b, cfg)
//
// Errors:
//   - an error if either argument is not valid JSON
//   - an error if either argument is not a JSON object
//   - an error if number conversion or final encoding fails
func (p *Processor) MergeJSON(json1, json2 string, cfg ...Config) (string, error) {
	if err := p.checkClosed(); err != nil {
		return "", err
	}

	options, err := p.prepareOptions(cfg...)
	if err != nil {
		return "", err
	}
	defer releaseConfig(options)

	return mergeJSONWithMode(json1, json2, options.MergeMode, func(v any) (string, error) {
		return p.EncodeWithConfig(v, *options)
	})
}

// mergeJSONWithMode decodes both inputs, requires both to be JSON objects, deep-merges
// them under the given mode, normalizes the library's Number type to float64, and
// encodes the result with the provided marshal function. It performs no security
// validation.
func mergeJSONWithMode(json1, json2 string, mode MergeMode, marshal func(any) (string, error)) (string, error) {
	decoder := newNumberPreservingDecoder(true)

	data1, err := decoder.DecodeToAny(json1)
	if err != nil {
		return "", fmt.Errorf("invalid JSON in first argument: %w", err)
	}

	data2, err := decoder.DecodeToAny(json2)
	if err != nil {
		return "", fmt.Errorf("invalid JSON in second argument: %w", err)
	}

	obj1, ok1 := data1.(map[string]any)
	obj2, ok2 := data2.(map[string]any)

	if !ok1 {
		return "", fmt.Errorf("first JSON is not an object")
	}
	if !ok2 {
		return "", fmt.Errorf("second JSON is not an object")
	}

	merged := internal.DeepMergeWithMode(obj1, obj2, internal.MergeMode(mode))

	// Convert library Number types to float64 for proper encoding
	converted, convErr := convertLibraryNumbers(merged)
	if convErr != nil {
		return "", fmt.Errorf("number conversion failed: %w", convErr)
	}

	return marshal(converted)
}

// convertLibraryNumbers recursively converts the library's Number type to float64.
// This is needed because the library's NumberPreservingDecoder returns Number (not json.Number).
func convertLibraryNumbers(data any) (any, error) {
	return convertLibraryNumbersWithDepth(data, 0)
}

// convertLibraryNumbersWithDepth performs recursive Number conversion with depth tracking.
// SECURITY: Depth limit prevents stack overflow from deeply nested structures.
func convertLibraryNumbersWithDepth(data any, depth int) (any, error) {
	if depth > deepCopyMaxDepth {
		return nil, fmt.Errorf("number conversion depth limit exceeded: maximum depth is %d", deepCopyMaxDepth)
	}

	switch v := data.(type) {
	case Number:
		f, err := v.Float64()
		if err != nil {
			return v, nil // Keep original if conversion fails
		}
		return f, nil
	case map[string]any:
		result := make(map[string]any, len(v))
		for key, value := range v {
			converted, err := convertLibraryNumbersWithDepth(value, depth+1)
			if err != nil {
				return nil, err
			}
			result[key] = converted
		}
		return result, nil
	case []any:
		result := make([]any, len(v))
		for i, item := range v {
			converted, err := convertLibraryNumbersWithDepth(item, depth+1)
			if err != nil {
				return nil, err
			}
			result[i] = converted
		}
		return result, nil
	default:
		return data, nil
	}
}

// MergeMany merges multiple JSON objects using the unified Config pattern.
// Uses Config.MergeMode to determine the merge strategy (default: MergeUnion).
// Returns error if less than 2 JSON strings are provided.
//
// Example:
//
//	// Union merge (default)
//	result, err := json.MergeMany([]string{config1, config2, config3})
//
//	// Intersection merge
//	cfg := json.DefaultConfig()
//	cfg.MergeMode = json.MergeIntersection
//	result, err := json.MergeMany([]string{config1, config2, config3}, cfg)
func MergeMany(jsons []string, cfg ...Config) (string, error) {
	return withProcessor(func(p *Processor) (string, error) {
		return p.MergeMany(jsons, cfg...)
	})
}

// MergeMany is the Processor method equivalent of the package-level MergeMany.
// It folds the slice left-to-right via MergeJSON, using Config.MergeMode to determine
// the merge strategy (default: MergeUnion).
//
// Example:
//
//	// Union merge (default)
//	result, err := processor.MergeMany([]string{config1, config2, config3})
//
//	// Intersection merge
//	cfg := json.DefaultConfig()
//	cfg.MergeMode = json.MergeIntersection
//	result, err = processor.MergeMany([]string{config1, config2, config3}, cfg)
//
// Errors:
//   - an error if fewer than 2 JSON strings are provided
//   - an error if any merge step fails (wrapped with the failing index)
func (p *Processor) MergeMany(jsons []string, cfg ...Config) (string, error) {
	if err := p.checkClosed(); err != nil {
		return "", err
	}

	if len(jsons) < 2 {
		return "", fmt.Errorf("MergeMany requires at least 2 JSON strings, got %d", len(jsons))
	}

	result := jsons[0]
	for i := 1; i < len(jsons); i++ {
		var err error
		result, err = p.MergeJSON(result, jsons[i], cfg...)
		if err != nil {
			return "", fmt.Errorf("merge failed at index %d: %w", i, err)
		}
	}

	return result, nil
}

// convertToTypedCore is the shared core logic for converting a value to type T.
func convertToTypedCore[T any](value any, path string) (T, error) {
	var zero T

	if converted, ok := unifiedTypeConversion[T](value); ok {
		return converted, nil
	}

	// Fallback: re-marshal and unmarshal for complex types
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

// ============================================================================
// VALUE UTILITIES
// ============================================================================

// isEmptyOrZero checks if a value is empty or its zero value.
// Supports all standard numeric types, bool, string, slices, maps, Number, and json.Number.
// For slices and maps, returns true if nil or empty (len == 0).
func isEmptyOrZero(v any) bool {
	if v == nil {
		return true
	}
	switch val := v.(type) {
	case string:
		return val == ""
	case int:
		return val == 0
	case int8:
		return val == 0
	case int16:
		return val == 0
	case int32:
		return val == 0
	case int64:
		return val == 0
	case uint:
		return val == 0
	case uint8:
		return val == 0
	case uint16:
		return val == 0
	case uint32:
		return val == 0
	case uint64:
		return val == 0
	case float32:
		return val == 0
	case float64:
		return val == 0
	case bool:
		return !val
	case Number:
		// "0.0" and "0e0" are numeric zeros but not integer literals, so
		// fall back to Float64 when Int64 refuses the format.
		if n, err := val.Int64(); err == nil {
			return n == 0
		}
		f, err := val.Float64()
		return err == nil && f == 0
	case json.Number:
		if n, err := val.Int64(); err == nil {
			return n == 0
		}
		f, err := val.Float64()
		return err == nil && f == 0
	case []any:
		return len(val) == 0
	case map[string]any:
		return len(val) == 0
	case map[any]any:
		return len(val) == 0
	default:
		return false
	}
}
