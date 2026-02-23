package internal

import (
	"fmt"
)

// MergeObjects merges two objects, with the second object taking precedence
func MergeObjects(obj1, obj2 map[string]any) map[string]any {
	// Pre-allocate with combined size to avoid rehashing
	result := make(map[string]any, len(obj1)+len(obj2))

	// Copy from first object
	for k, v := range obj1 {
		result[k] = v
	}

	// Override with second object
	for k, v := range obj2 {
		result[k] = v
	}

	return result
}

// FlattenArray flattens a nested array structure
func FlattenArray(arr []any) []any {
	// Pre-allocate with at least the input size (might grow with nested arrays)
	result := make([]any, 0, len(arr))

	for _, item := range arr {
		if subArr, ok := item.([]any); ok {
			result = append(result, FlattenArray(subArr)...)
		} else {
			result = append(result, item)
		}
	}

	return result
}

// UniqueArray removes duplicate values from an array
func UniqueArray(arr []any) []any {
	// Pre-allocate map and result with estimated sizes
	seen := make(map[string]bool, len(arr))
	result := make([]any, 0, len(arr))

	for _, item := range arr {
		key := fmt.Sprintf("%v", item)
		if !seen[key] {
			seen[key] = true
			result = append(result, item)
		}
	}

	return result
}

// ReverseArray reverses an array in place
func ReverseArray(arr []any) {
	for i, j := 0, len(arr)-1; i < j; i, j = i+1, j-1 {
		arr[i], arr[j] = arr[j], arr[i]
	}
}
