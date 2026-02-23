package json

import (
	"strings"
)

// IterableValue wraps a value to provide convenient access methods
// Note: Simplified to avoid resource leaks from holding processor/iterator references
type IterableValue struct {
	data any
}

// NewIterableValue creates an IterableValue from data
func NewIterableValue(data any) *IterableValue {
	return &IterableValue{data: data}
}

// GetData returns the underlying data
func (iv *IterableValue) GetData() any {
	return iv.data
}

// Get returns a value by path (supports dot notation and array indices)
func (iv *IterableValue) Get(path string) any {
	if path == "" || path == "." {
		return iv.data
	}

	// Use enhanced path navigation for complex paths
	if isComplexPathIterator(path) {
		result, err := navigateToPathWithArraySupport(iv.data, path)
		if err != nil {
			return nil
		}
		return result
	}

	// Fall back to simple path navigation for non-complex paths
	result, err := navigateToPathSimple(iv.data, path)
	if err != nil {
		return nil
	}
	return result
}

// GetString returns a string value by key or path
// Supports path navigation with dot notation and array indices (e.g., "user.address.city" or "users[0].name")
func (iv *IterableValue) GetString(key string) string {
	// Check if key is a path (contains dots or brackets)
	if strings.ContainsAny(key, ".[]") {
		val := iv.Get(key)
		if val == nil {
			return ""
		}
		if str, ok := val.(string); ok {
			return str
		}
		return ConvertToString(val)
	}

	// Original single-key lookup logic
	obj, ok := iv.data.(map[string]any)
	if !ok {
		return ""
	}

	val, exists := obj[key]
	if !exists {
		return ""
	}

	if str, ok := val.(string); ok {
		return str
	}

	return ConvertToString(val)
}

// GetInt returns an int value by key or path
// Supports path navigation with dot notation and array indices (e.g., "user.age" or "users[0].id")
func (iv *IterableValue) GetInt(key string) int {
	// Check if key is a path (contains dots or brackets)
	if strings.ContainsAny(key, ".[]") {
		val := iv.Get(key)
		if val == nil {
			return 0
		}
		if result, ok := ConvertToInt(val); ok {
			return result
		}
		return 0
	}

	// Original single-key lookup logic
	obj, ok := iv.data.(map[string]any)
	if !ok {
		return 0
	}

	val, exists := obj[key]
	if !exists {
		return 0
	}

	if result, ok := ConvertToInt(val); ok {
		return result
	}

	return 0
}

// GetFloat64 returns a float64 value by key or path
// Supports path navigation with dot notation and array indices
func (iv *IterableValue) GetFloat64(key string) float64 {
	// Check if key is a path (contains dots or brackets)
	if strings.ContainsAny(key, ".[]") {
		val := iv.Get(key)
		if val == nil {
			return 0
		}
		if result, ok := ConvertToFloat64(val); ok {
			return result
		}
		return 0
	}

	// Original single-key lookup logic
	obj, ok := iv.data.(map[string]any)
	if !ok {
		return 0
	}

	val, exists := obj[key]
	if !exists {
		return 0
	}

	if result, ok := ConvertToFloat64(val); ok {
		return result
	}

	return 0
}

// GetBool returns a bool value by key or path
// Supports path navigation with dot notation and array indices
func (iv *IterableValue) GetBool(key string) bool {
	// Check if key is a path (contains dots or brackets)
	if strings.ContainsAny(key, ".[]") {
		val := iv.Get(key)
		if val == nil {
			return false
		}
		if result, ok := ConvertToBool(val); ok {
			return result
		}
		return false
	}

	// Original single-key lookup logic
	obj, ok := iv.data.(map[string]any)
	if !ok {
		return false
	}

	val, exists := obj[key]
	if !exists {
		return false
	}

	if result, ok := ConvertToBool(val); ok {
		return result
	}

	return false
}

// GetArray returns an array value by key or path
// Supports path navigation with dot notation and array indices
func (iv *IterableValue) GetArray(key string) []any {
	// Check if key is a path (contains dots or brackets)
	if strings.ContainsAny(key, ".[]") {
		val := iv.Get(key)
		if val == nil {
			return nil
		}
		if arr, ok := val.([]any); ok {
			return arr
		}
		return nil
	}

	// Original single-key lookup logic
	obj, ok := iv.data.(map[string]any)
	if !ok {
		return nil
	}

	val, exists := obj[key]
	if !exists {
		return nil
	}

	if arr, ok := val.([]any); ok {
		return arr
	}

	return nil
}

// GetObject returns an object value by key or path
// Supports path navigation with dot notation and array indices
func (iv *IterableValue) GetObject(key string) map[string]any {
	// Check if key is a path (contains dots or brackets)
	if strings.ContainsAny(key, ".[]") {
		val := iv.Get(key)
		if val == nil {
			return nil
		}
		if result, ok := val.(map[string]any); ok {
			return result
		}
		return nil
	}

	// Original single-key lookup logic
	obj, ok := iv.data.(map[string]any)
	if !ok {
		return nil
	}

	val, exists := obj[key]
	if !exists {
		return nil
	}

	if result, ok := val.(map[string]any); ok {
		return result
	}

	return nil
}

// GetWithDefault returns a value by key or path with a default fallback
// Supports path navigation with dot notation and array indices
func (iv *IterableValue) GetWithDefault(key string, defaultValue any) any {
	// Check if key is a path (contains dots or brackets)
	if strings.ContainsAny(key, ".[]") {
		val := iv.Get(key)
		if val == nil {
			return defaultValue
		}
		return val
	}

	// Original single-key lookup logic
	obj, ok := iv.data.(map[string]any)
	if !ok {
		return defaultValue
	}

	val, exists := obj[key]
	if !exists {
		return defaultValue
	}

	return val
}

// GetStringWithDefault returns a string value by key or path with a default fallback
// Supports path navigation with dot notation and array indices
func (iv *IterableValue) GetStringWithDefault(key string, defaultValue string) string {
	// Check if key is a path (contains dots or brackets)
	if strings.ContainsAny(key, ".[]") {
		val := iv.Get(key)
		if val == nil {
			return defaultValue
		}
		if str, ok := val.(string); ok {
			return str
		}
		return defaultValue
	}

	// Original single-key lookup logic
	obj, ok := iv.data.(map[string]any)
	if !ok {
		return defaultValue
	}

	val, exists := obj[key]
	if !exists {
		return defaultValue
	}

	if str, ok := val.(string); ok {
		return str
	}

	return defaultValue
}

// GetIntWithDefault returns an int value by key or path with a default fallback
// Supports path navigation with dot notation and array indices
func (iv *IterableValue) GetIntWithDefault(key string, defaultValue int) int {
	// Check if key is a path (contains dots or brackets)
	if strings.ContainsAny(key, ".[]") {
		val := iv.Get(key)
		if val == nil {
			return defaultValue
		}
		if result, ok := ConvertToInt(val); ok {
			return result
		}
		return defaultValue
	}

	// Original single-key lookup logic
	obj, ok := iv.data.(map[string]any)
	if !ok {
		return defaultValue
	}

	val, exists := obj[key]
	if !exists {
		return defaultValue
	}

	if result, ok := ConvertToInt(val); ok {
		return result
	}

	return defaultValue
}

// GetFloat64WithDefault returns a float64 value by key or path with a default fallback
// Supports path navigation with dot notation and array indices
func (iv *IterableValue) GetFloat64WithDefault(key string, defaultValue float64) float64 {
	// Check if key is a path (contains dots or brackets)
	if strings.ContainsAny(key, ".[]") {
		val := iv.Get(key)
		if val == nil {
			return defaultValue
		}
		if result, ok := ConvertToFloat64(val); ok {
			return result
		}
		return defaultValue
	}

	// Original single-key lookup logic
	obj, ok := iv.data.(map[string]any)
	if !ok {
		return defaultValue
	}

	val, exists := obj[key]
	if !exists {
		return defaultValue
	}

	if result, ok := ConvertToFloat64(val); ok {
		return result
	}

	return defaultValue
}

// GetBoolWithDefault returns a bool value by key or path with a default fallback
// Supports path navigation with dot notation and array indices
func (iv *IterableValue) GetBoolWithDefault(key string, defaultValue bool) bool {
	// Check if key is a path (contains dots or brackets)
	if strings.ContainsAny(key, ".[]") {
		val := iv.Get(key)
		if val == nil {
			return defaultValue
		}
		if result, ok := ConvertToBool(val); ok {
			return result
		}
		return defaultValue
	}

	// Original single-key lookup logic
	obj, ok := iv.data.(map[string]any)
	if !ok {
		return defaultValue
	}

	val, exists := obj[key]
	if !exists {
		return defaultValue
	}

	if result, ok := ConvertToBool(val); ok {
		return result
	}

	return defaultValue
}

// Exists checks if a key or path exists in the object
// Supports path navigation with dot notation and array indices
func (iv *IterableValue) Exists(key string) bool {
	// Check if key is a path (contains dots or brackets)
	if strings.ContainsAny(key, ".[]") {
		return iv.Get(key) != nil
	}

	// Original single-key lookup logic
	obj, ok := iv.data.(map[string]any)
	if !ok {
		return false
	}

	_, exists := obj[key]
	return exists
}

// IsNullData checks if the whole value is null (for backward compatibility)
func (iv *IterableValue) IsNullData() bool {
	return iv.data == nil
}

// IsNull checks if a specific key's or path's value is null
// Supports path navigation with dot notation and array indices
func (iv *IterableValue) IsNull(key string) bool {
	// Check if key is a path (contains dots or brackets)
	if strings.ContainsAny(key, ".[]") {
		val := iv.Get(key)
		return val == nil
	}

	// Original single-key lookup logic
	obj, ok := iv.data.(map[string]any)
	if !ok {
		return true
	}

	val, exists := obj[key]
	if !exists {
		return true
	}

	return val == nil
}

// IsEmptyData checks if the whole value is empty (for backward compatibility)
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

// IsEmpty checks if a specific key's or path's value is empty
// Supports path navigation with dot notation and array indices
func (iv *IterableValue) IsEmpty(key string) bool {
	// Check if key is a path (contains dots or brackets)
	if strings.ContainsAny(key, ".[]") {
		val := iv.Get(key)
		if val == nil {
			return true
		}
		switch v := val.(type) {
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

	// Original single-key lookup logic
	obj, ok := iv.data.(map[string]any)
	if !ok {
		return true
	}

	val, exists := obj[key]
	if !exists {
		return true
	}

	switch v := val.(type) {
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
