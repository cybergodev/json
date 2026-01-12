package json

import (
	"fmt"
	"reflect"
	"strings"
)

// SafeTypeAssert performs a safe type assertion with generics
func SafeTypeAssert[T any](value any) (T, bool) {
	var zero T

	if value == nil {
		return zero, false
	}

	// Direct type assertion
	if result, ok := value.(T); ok {
		return result, true
	}

	// Try conversion via reflection
	val := reflect.ValueOf(value)
	targetType := reflect.TypeOf(zero)

	if targetType != nil && val.Type().ConvertibleTo(targetType) {
		converted := val.Convert(targetType)
		return converted.Interface().(T), true
	}

	return zero, false
}

// Iterator represents an iterator over JSON data
type Iterator struct {
	processor *Processor
	data      any
	options   *ProcessorOptions
	position  int
}

// NewIterator creates a new Iterator
func NewIterator(processor *Processor, data any, opts *ProcessorOptions) *Iterator {
	return &Iterator{
		processor: processor,
		data:      data,
		options:   opts,
		position:  0,
	}
}

// HasNext checks if there are more elements
func (it *Iterator) HasNext() bool {
	if arr, ok := it.data.([]any); ok {
		return it.position < len(arr)
	}
	if obj, ok := it.data.(map[string]any); ok {
		return it.position < len(obj)
	}
	return false
}

// Next returns the next element
func (it *Iterator) Next() (any, bool) {
	if !it.HasNext() {
		return nil, false
	}

	if arr, ok := it.data.([]any); ok {
		result := arr[it.position]
		it.position++
		return result, true
	}

	if obj, ok := it.data.(map[string]any); ok {
		keys := reflect.ValueOf(obj).MapKeys()
		if it.position < len(keys) {
			key := keys[it.position].String()
			it.position++
			return obj[key], true
		}
	}

	return nil, false
}

// IterableValue wraps a value to provide convenient access methods
type IterableValue struct {
	data      any
	processor *Processor
	iterator  *Iterator
}

// NewIterableValueWithIterator creates an IterableValue with an iterator
func NewIterableValueWithIterator(data any, processor *Processor, iterator *Iterator) *IterableValue {
	return &IterableValue{
		data:      data,
		processor: processor,
		iterator:  iterator,
	}
}

// GetString returns a string value by key
func (iv *IterableValue) GetString(key string) string {
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

// GetInt returns an int value by key
func (iv *IterableValue) GetInt(key string) int {
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

// GetFloat64 returns a float64 value by key
func (iv *IterableValue) GetFloat64(key string) float64 {
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

// GetBool returns a bool value by key
func (iv *IterableValue) GetBool(key string) bool {
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

// GetArray returns an array value by key
func (iv *IterableValue) GetArray(key string) []any {
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

// GetObject returns an object value by key
func (iv *IterableValue) GetObject(key string) map[string]any {
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

// GetWithDefault returns a value by key with a default fallback
func (iv *IterableValue) GetWithDefault(key string, defaultValue any) any {
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

// GetStringWithDefault returns a string value by key with a default fallback
func (iv *IterableValue) GetStringWithDefault(key string, defaultValue string) string {
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

// GetIntWithDefault returns an int value by key with a default fallback
func (iv *IterableValue) GetIntWithDefault(key string, defaultValue int) int {
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

// GetFloat64WithDefault returns a float64 value by key with a default fallback
func (iv *IterableValue) GetFloat64WithDefault(key string, defaultValue float64) float64 {
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

// GetBoolWithDefault returns a bool value by key with a default fallback
func (iv *IterableValue) GetBoolWithDefault(key string, defaultValue bool) bool {
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

// Exists checks if a key exists in the object
func (iv *IterableValue) Exists(key string) bool {
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

// IsNull checks if a specific key's value is null
func (iv *IterableValue) IsNull(key string) bool {
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

// IsEmpty checks if a specific key's value is empty
func (iv *IterableValue) IsEmpty(key string) bool {
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

// Get returns a value by path (supports dot notation)
func (iv *IterableValue) Get(path string) any {
	if path == "" || path == "." {
		return iv.data
	}

	result, err := navigateToPathSimple(iv.data, path)
	if err != nil {
		return nil
	}
	return result
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
				return nil, fmt.Errorf("key not found: %s", part)
			}
		default:
			return nil, fmt.Errorf("cannot access property '%s' on type %T", part, current)
		}
	}

	return current, nil
}

// ForeachWithPathAndControl Foreach iterates over JSON arrays or objects and applies a function
// This is the 3-parameter version used by most code
func ForeachWithPathAndControl(jsonStr, path string, fn func(key any, value any) IteratorControl) error {
	processor := getDefaultProcessor()

	data, err := processor.Get(jsonStr, path)
	if err != nil {
		return err
	}

	return foreachOnValue(data, fn)
}

// Foreach iterates over JSON arrays or objects with simplified signature (for test compatibility)
func Foreach(jsonStr string, fn func(key any, item *IterableValue)) {
	processor := getDefaultProcessor()

	data, err := processor.Get(jsonStr, ".")
	if err != nil {
		return
	}

	foreachWithIterableValue(data, fn)
}

// foreachWithIterableValue iterates over a value and applies a function with IterableValue
func foreachWithIterableValue(data any, fn func(key any, item *IterableValue)) {
	switch v := data.(type) {
	case []any:
		for i, item := range v {
			iv := &IterableValue{data: item}
			fn(i, iv)
		}
	case map[string]any:
		for key, val := range v {
			iv := &IterableValue{data: val}
			fn(key, iv)
		}
	}
}

// ForechWithPath iterates over JSON arrays or objects with simplified signature (for test compatibility)
func ForeachWithPath(jsonStr, path string, fn func(key any, item *IterableValue)) error {
	processor := getDefaultProcessor()

	data, err := processor.Get(jsonStr, path)
	if err != nil {
		return err
	}

	foreachWithIterableValue(data, fn)
	return nil
}

// foreachWithPathAndIterator iterates with IterableValue and path information (full version)
func foreachWithPathAndIterator(jsonStr, path string, fn func(key any, item *IterableValue, currentPath string) IteratorControl) error {
	processor := getDefaultProcessor()

	data, err := processor.Get(jsonStr, path)
	if err != nil {
		return err
	}

	return foreachWithPathIterableValue(data, "", fn)
}

// foreachWithPathIterableValue iterates with IterableValue and path information
func foreachWithPathIterableValue(data any, currentPath string, fn func(key any, item *IterableValue, currentPath string) IteratorControl) error {
	switch v := data.(type) {
	case []any:
		for i, item := range v {
			path := fmt.Sprintf("%s[%d]", currentPath, i)
			iv := &IterableValue{data: item}
			if ctrl := fn(i, iv, path); ctrl == IteratorBreak {
				return nil
			}
		}
	case map[string]any:
		for key, val := range v {
			path := currentPath + "." + key
			iv := &IterableValue{data: val}
			if ctrl := fn(key, iv, path); ctrl == IteratorBreak {
				return nil
			}
		}
	default:
		return fmt.Errorf("value is not iterable: %T", data)
	}

	return nil
}

// ForeachReturn is a variant that returns error (for compatibility with test expectations)
func ForeachReturn(jsonStr string, fn func(key any, item *IterableValue)) (string, error) {
	processor := getDefaultProcessor()

	data, err := processor.Get(jsonStr, ".")
	if err != nil {
		return "", err
	}

	// Execute the iteration
	foreachWithIterableValue(data, fn)

	// Return the original JSON string
	return jsonStr, nil
}

// foreachOnValue iterates over a value and applies a function
func foreachOnValue(data any, fn func(key any, value any) IteratorControl) error {
	switch v := data.(type) {
	case []any:
		for i, item := range v {
			if ctrl := fn(i, item); ctrl == IteratorBreak {
				return nil
			}
		}
	case map[string]any:
		for key, val := range v {
			if ctrl := fn(key, val); ctrl == IteratorBreak {
				return nil
			}
		}
	default:
		return fmt.Errorf("value is not iterable: %T", data)
	}

	return nil
}

// foreachWithPathOnValue iterates over a value and applies a function with path information
func foreachWithPathOnValue(data any, currentPath string, fn func(key any, value any, currentPath string) IteratorControl) error {
	switch v := data.(type) {
	case []any:
		for i, item := range v {
			path := fmt.Sprintf("%s[%d]", currentPath, i)
			if ctrl := fn(i, item, path); ctrl == IteratorBreak {
				return nil
			}
		}
	case map[string]any:
		for key, val := range v {
			path := currentPath + "." + key
			if ctrl := fn(key, val, path); ctrl == IteratorBreak {
				return nil
			}
		}
	default:
		return fmt.Errorf("value is not iterable: %T", data)
	}

	return nil
}

// ForeachNested iterates over nested JSON structures
func ForeachNested(jsonStr string, fn func(key any, item *IterableValue)) {
	processor := getDefaultProcessor()

	data, err := processor.Get(jsonStr, ".")
	if err != nil {
		return
	}

	foreachNestedOnValue(data, fn)
}

// foreachNestedOnValue recursively iterates over nested values
func foreachNestedOnValue(data any, fn func(key any, item *IterableValue)) {
	switch v := data.(type) {
	case []any:
		for i, item := range v {
			iv := &IterableValue{data: item}
			fn(i, iv)
			foreachNestedOnValue(item, fn)
		}
	case map[string]any:
		for key, val := range v {
			iv := &IterableValue{data: val}
			fn(key, iv)
			foreachNestedOnValue(val, fn)
		}
	}
}
