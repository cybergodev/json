package json

import (
	"fmt"
	"strings"
	"sync"

	"github.com/cybergodev/json/internal"
)

// iterableValuePool pools IterableValue objects to reduce allocations
// PERFORMANCE: Significant reduction in allocations during nested iteration
var iterableValuePool = sync.Pool{
	New: func() any {
		return &IterableValue{}
	},
}

// releaseIterableValues returns a slice of IterableValue objects to the pool.
func releaseIterableValues(items []*IterableValue) {
	for _, iv := range items {
		iv.data = nil
		iterableValuePool.Put(iv)
	}
}

// IterableValue wraps a value to provide convenient access methods during iteration.
// Used by Foreach and ForeachKey callback functions to provide structured access.
// Note: Simplified to avoid resource leaks from holding processor/iterator references.
//
// Example:
//
//	err := processor.Foreach(data, "items", func(item json.IterableValue) error {
//	    name, _ := item.GetString("name")
//	    age, _ := item.GetInt("age")
//	    fmt.Printf("Name: %s, Age: %d\n", name, age)
//	    return nil
//	})
type IterableValue struct {
	data any
}

// newIterableValue creates an IterableValue from data.
// This is primarily used internally by iteration functions.
func newIterableValue(data any) *IterableValue {
	return &IterableValue{data: data}
}

// GetData returns the underlying data
func (iv *IterableValue) GetData() any {
	return iv.data
}

// Break returns a signal to stop iteration without error.
// Use it in ForeachFile/ForeachFileChunked callback to exit early.
//
// Example:
//
//	processor.ForeachFile("data.json", func(key any, item *json.IterableValue) error {
//	    if item.GetInt("id") == targetId {
//	        return item.Break() // stop iteration
//	    }
//	    return nil // continue
//	})
func (iv *IterableValue) Break() error {
	return errBreak
}

// Get returns a value by path (supports dot notation and array indices)
func (iv *IterableValue) Get(path string) any {
	if path == "" || path == "." {
		return iv.data
	}

	// Use enhanced path navigation for complex paths
	if isComplexPathIterator(path) {
		// Use compiled path cache for complex paths
		// NOTE: Do NOT call Release() on cached paths - they are shared!
		cp, err := internal.GetGlobalCompiledPathCache().Get(path)
		if err != nil {
			return nil
		}
		result, err := cp.Get(iv.data)
		if err != nil {
			return nil
		}
		return result
	}

	// Fast path for simple paths - avoid strings.Split allocation
	current := iv.data
	start := 0
	for i := 0; i <= len(path); i++ {
		if i == len(path) || path[i] == '.' {
			if i > start {
				part := path[start:i]
				obj, ok := current.(map[string]any)
				if !ok {
					return nil
				}
				current, ok = obj[part]
				if !ok {
					return nil
				}
			}
			start = i + 1
		}
	}
	return current
}

// getSimpleValue retrieves a raw value from the underlying data by simple key lookup.
// Returns (value, true) if found, (nil, false) otherwise.
func (iv *IterableValue) getSimpleValue(key string) (any, bool) {
	obj, ok := iv.data.(map[string]any)
	if !ok {
		return nil, false
	}
	val, exists := obj[key]
	return val, exists
}

// resolveValue retrieves a value by key or path, handling both simple and complex paths.
// For complex paths, delegates to iv.Get(). For simple paths, does a direct map lookup.
func (iv *IterableValue) resolveValue(key string) (any, bool) {
	switch getPathType(key) {
	case pathTypeComplex:
		val := iv.Get(key)
		return val, val != nil
	case pathTypeSimple:
		return iv.getSimpleValue(key)
	}
	return nil, false
}

// getTypedValue resolves a value and applies a type conversion function.
// Returns the converted value on success, or the provided zero value on failure.
func getTypedValue[T any](iv *IterableValue, key string, convert func(any) (T, bool), zero T) T {
	val, found := iv.resolveValue(key)
	if !found || val == nil {
		return zero
	}
	if result, ok := convert(val); ok {
		return result
	}
	return zero
}

// getTypedValueWithDefault resolves a value and applies a type conversion function.
// Returns the converted value on success, or the provided default on failure.
func getTypedValueWithDefault[T any](iv *IterableValue, key string, convert func(any) (T, bool), defaultVal T) T {
	val, found := iv.resolveValue(key)
	if !found || val == nil {
		return defaultVal
	}
	if result, ok := convert(val); ok {
		return result
	}
	return defaultVal
}

// GetString returns a string value by key or path.
func (iv *IterableValue) GetString(key string) string {
	val, found := iv.resolveValue(key)
	if !found || val == nil {
		return ""
	}
	if str, ok := val.(string); ok {
		return str
	}
	return convertToString(val)
}

// GetInt returns an int value by key or path.
func (iv *IterableValue) GetInt(key string) int {
	return getTypedValue(iv, key, convertToInt, 0)
}

// GetFloat64 returns a float64 value by key or path.
func (iv *IterableValue) GetFloat64(key string) float64 {
	return getTypedValue(iv, key, convertToFloat64, 0.0)
}

// GetBool returns a bool value by key or path.
func (iv *IterableValue) GetBool(key string) bool {
	return getTypedValue(iv, key, convertToBool, false)
}

// GetArray returns an array value by key or path.
func (iv *IterableValue) GetArray(key string) []any {
	return getTypedValue(iv, key, func(v any) ([]any, bool) {
		arr, ok := v.([]any)
		return arr, ok
	}, nil)
}

// GetObject returns an object value by key or path.
func (iv *IterableValue) GetObject(key string) map[string]any {
	return getTypedValue(iv, key, func(v any) (map[string]any, bool) {
		m, ok := v.(map[string]any)
		return m, ok
	}, nil)
}

// GetWithDefault returns a value by key or path with a default fallback.
func (iv *IterableValue) GetWithDefault(key string, defaultValue any) any {
	val, found := iv.resolveValue(key)
	if !found || val == nil {
		return defaultValue
	}
	return val
}

// GetStringWithDefault returns a string value by key or path with a default fallback.
func (iv *IterableValue) GetStringWithDefault(key string, defaultValue string) string {
	return getTypedValueWithDefault(iv, key, func(v any) (string, bool) {
		str, ok := v.(string)
		return str, ok
	}, defaultValue)
}

// GetIntWithDefault returns an int value by key or path with a default fallback.
func (iv *IterableValue) GetIntWithDefault(key string, defaultValue int) int {
	return getTypedValueWithDefault(iv, key, convertToInt, defaultValue)
}

// GetFloat64WithDefault returns a float64 value by key or path with a default fallback.
func (iv *IterableValue) GetFloat64WithDefault(key string, defaultValue float64) float64 {
	return getTypedValueWithDefault(iv, key, convertToFloat64, defaultValue)
}

// GetBoolWithDefault returns a bool value by key or path with a default fallback.
func (iv *IterableValue) GetBoolWithDefault(key string, defaultValue bool) bool {
	return getTypedValueWithDefault(iv, key, convertToBool, defaultValue)
}

// Exists checks if a key or path exists in the object.
func (iv *IterableValue) Exists(key string) bool {
	_, found := iv.resolveValue(key)
	return found
}

// IsNullData checks if the whole value is null (for backward compatibility).
func (iv *IterableValue) IsNullData() bool {
	return iv.data == nil
}

// IsNull checks if a specific key's or path's value is null.
func (iv *IterableValue) IsNull(key string) bool {
	val, found := iv.resolveValue(key)
	if !found {
		return true
	}
	return val == nil
}

// IsEmptyData checks if the whole value is empty (for backward compatibility).
func (iv *IterableValue) IsEmptyData() bool {
	if iv.data == nil {
		return true
	}
	switch v := iv.data.(type) {
	case []any:
		return len(v) == 0
	case map[string]any:
		return len(v) == 0
	case string:
		return v == ""
	default:
		return false
	}
}

// IsEmpty checks if a specific key's or path's value is empty.
func (iv *IterableValue) IsEmpty(key string) bool {
	val, found := iv.resolveValue(key)
	if !found || val == nil {
		return true
	}
	return isEmptyOrZero(val)
}

// ForeachNested iterates over nested JSON structures with a path
func (iv *IterableValue) ForeachNested(path string, fn func(key any, item *IterableValue)) {
	var data any = iv.data

	if path != "" && path != "." {
		var err error
		data, err = navigateToPathSimple(iv.data, path)
		if err != nil {
			return
		}
	}

	foreachNestedOnValue(data, fn)
}

// Release returns the IterableValue to the pool
func (iv *IterableValue) Release() {
	iv.data = nil
	iterableValuePool.Put(iv)
}

// navigateToPathSimple provides simple path navigation for IterableValue
func navigateToPathSimple(data any, path string) (any, error) {
	current := data
	parts := strings.Split(path, ".")

	for _, part := range parts {
		if part == "" {
			continue
		}

		switch v := current.(type) {
		case map[string]any:
			var ok bool
			current, ok = v[part]
			if !ok {
				return nil, newPathError(part, fmt.Sprintf("key not found: %s", part), ErrPathNotFound)
			}
		default:
			return nil, newPathError(part, fmt.Sprintf("cannot access property '%s' on type %T", part, current), ErrTypeMismatch)
		}
	}

	return current, nil
}

// isComplexPathIterator checks if the path contains array indices or other complex syntax
func isComplexPathIterator(path string) bool {
	return strings.ContainsAny(path, "[]")
}
