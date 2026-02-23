package json

import (
	"fmt"
	"strings"

	"github.com/cybergodev/json/internal"
)

// isComplexPathIterator checks if the path contains array indices or other complex syntax
func isComplexPathIterator(path string) bool {
	return strings.ContainsAny(path, "[]")
}

// navigateToPathWithArraySupport provides path navigation with array index support
func navigateToPathWithArraySupport(data any, path string) (any, error) {
	current := data

	// Parse path using internal parser
	segments, err := internal.ParsePath(path)
	if err != nil {
		return nil, err
	}

	for _, segment := range segments {
		switch segment.Type {
		case internal.PropertySegment:
			// Property access
			obj, ok := current.(map[string]any)
			if !ok {
				return nil, newPathError(segment.Key, fmt.Sprintf("cannot access property '%s' on type %T", segment.Key, current), ErrTypeMismatch)
			}
			var exists bool
			current, exists = obj[segment.Key]
			if !exists {
				return nil, newPathError(segment.Key, fmt.Sprintf("key not found: %s", segment.Key), ErrPathNotFound)
			}

		case internal.ArrayIndexSegment:
			// Array index access
			arr, ok := current.([]any)
			if !ok {
				return nil, newPathError(path, fmt.Sprintf("cannot access index on type %T", current), ErrTypeMismatch)
			}

			// Handle negative index
			index := segment.Index
			if index < 0 {
				index = len(arr) + index
			}

			if index < 0 || index >= len(arr) {
				return nil, newPathError(path, fmt.Sprintf("array index out of bounds: %d", segment.Index), ErrPathNotFound)
			}
			current = arr[index]

		case internal.ArraySliceSegment:
			// Array slice access - build slice part string
			arr, ok := current.([]any)
			if !ok {
				return nil, newPathError(path, fmt.Sprintf("cannot slice type %T", current), ErrTypeMismatch)
			}

			// Build slice string from Start, End, Step
			var sliceStr string
			if segment.Start != nil {
				sliceStr += fmt.Sprintf("%d", *segment.Start)
			}
			sliceStr += ":"
			if segment.End != nil {
				sliceStr += fmt.Sprintf("%d", *segment.End)
			}
			if segment.Step != nil {
				sliceStr += fmt.Sprintf(":%d", *segment.Step)
			}

			start, end, step, err := internal.ParseSliceComponents(sliceStr)
			if err != nil {
				return nil, err
			}

			// Normalize indices
			if start == nil {
				startVal := 0
				start = &startVal
			}
			if end == nil {
				endVal := len(arr)
				end = &endVal
			}

			// Handle negative indices
			if *start < 0 {
				*start = len(arr) + *start
			}
			if *end < 0 {
				*end = len(arr) + *end
			}

			// Apply slice
			result := make([]any, 0)
			stepVal := 1
			if step != nil {
				stepVal = *step
			}
			for i := *start; i < *end; i += stepVal {
				if i >= 0 && i < len(arr) {
					result = append(result, arr[i])
				}
			}
			current = result
		}
	}

	return current, nil
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
