package json

import (
	"reflect"
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

// Iterator-related types and functions have been refactored into separate files:
//
//   - iterable_value.go     : IterableValue struct and all Get methods
//   - iterator_foreach.go   : Foreach* functions and IteratorControl
//   - iterator_navigation.go: Path navigation helpers
//
// This file is kept for the core Iterator struct and SafeTypeAssert function.
// All types remain in package json and maintain 100% API compatibility.
