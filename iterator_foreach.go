package json

import (
	"fmt"
)

// ForeachWithPathAndControl iterates over JSON arrays or objects and applies a function
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

// ForeachWithPath iterates over JSON arrays or objects with simplified signature (for test compatibility)
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
		return newOperationPathError("foreach", currentPath, fmt.Sprintf("value is not iterable: %T", data), ErrTypeMismatch)
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
		return newOperationError("foreach", fmt.Sprintf("value is not iterable: %T", data), ErrTypeMismatch)
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
		return newOperationPathError("foreach", currentPath, fmt.Sprintf("value is not iterable: %T", data), ErrTypeMismatch)
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
