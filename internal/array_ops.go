package internal

import (
	"sync"
)

// ============================================================================
// OPTIMIZED ARRAY OPERATIONS
// This file contains performance-optimized array manipulation functions
// ============================================================================

// arraySlicePool pools slices for array operations to reduce allocations
var arraySlicePool = sync.Pool{
	New: func() any {
		s := make([]any, 0, 32)
		return &s
	},
}

// GetPooledSlice gets a pooled slice for array operations
func GetPooledSlice() *[]any {
	s := arraySlicePool.Get().(*[]any)
	*s = (*s)[:0]
	return s
}

// PutPooledSlice returns a slice to the pool
func PutPooledSlice(s *[]any) {
	if s == nil {
		return
	}
	if cap(*s) <= 1024 { // Don't pool very large slices
		*s = (*s)[:0]
		arraySlicePool.Put(s)
	}
}

// CompactArrayOptimized removes null and empty values from array with pooling
func CompactArrayOptimized(arr []any) []any {
	if len(arr) == 0 {
		return []any{}
	}

	// Use pooled slice for result
	resultPtr := GetPooledSlice()
	result := *resultPtr

	for _, item := range arr {
		if !isNilOrEmptyFast(item) {
			result = append(result, item)
		}
	}

	// Copy to new slice if we have results
	if len(result) == 0 {
		PutPooledSlice(resultPtr)
		return []any{}
	}

	final := make([]any, len(result))
	copy(final, result)
	PutPooledSlice(resultPtr)

	return final
}

// isNilOrEmptyFast is a fast check for nil or empty values
func isNilOrEmptyFast(item any) bool {
	if item == nil {
		return true
	}

	switch v := item.(type) {
	case string:
		return v == ""
	case []any:
		return len(v) == 0
	case map[string]any:
		return len(v) == 0
	default:
		return false
	}
}

// FilterArrayOptimized filters array with a predicate function using pooling
func FilterArrayOptimized(arr []any, predicate func(any) bool) []any {
	if len(arr) == 0 {
		return []any{}
	}

	resultPtr := GetPooledSlice()
	result := *resultPtr

	for _, item := range arr {
		if predicate(item) {
			result = append(result, item)
		}
	}

	if len(result) == 0 {
		PutPooledSlice(resultPtr)
		return []any{}
	}

	final := make([]any, len(result))
	copy(final, result)
	PutPooledSlice(resultPtr)

	return final
}

// MapArrayOptimized transforms array elements using pooling
func MapArrayOptimized(arr []any, transform func(any) any) []any {
	if len(arr) == 0 {
		return []any{}
	}

	result := make([]any, len(arr))
	for i, item := range arr {
		result[i] = transform(item)
	}

	return result
}

// UniqueArrayOptimized removes duplicates from array using map for O(n) lookup
func UniqueArrayOptimized(arr []any) []any {
	if len(arr) <= 1 {
		return arr
	}

	seen := make(map[any]struct{}, len(arr))
	resultPtr := GetPooledSlice()
	result := *resultPtr

	for _, item := range arr {
		if _, exists := seen[item]; !exists {
			seen[item] = struct{}{}
			result = append(result, item)
		}
	}

	if len(result) == 0 {
		PutPooledSlice(resultPtr)
		return []any{}
	}

	final := make([]any, len(result))
	copy(final, result)
	PutPooledSlice(resultPtr)

	return final
}

// FlattenArrayOptimized flattens nested arrays with pooling
func FlattenArrayOptimized(arr []any) []any {
	if len(arr) == 0 {
		return []any{}
	}

	// Estimate final size
	estimatedSize := estimateFlatSize(arr)
	result := make([]any, 0, estimatedSize)

	flattenInto(arr, &result)
	return result
}

func estimateFlatSize(arr []any) int {
	size := 0
	for _, item := range arr {
		if nested, ok := item.([]any); ok {
			size += estimateFlatSize(nested)
		} else {
			size++
		}
	}
	return size
}

func flattenInto(arr []any, result *[]any) {
	for _, item := range arr {
		if nested, ok := item.([]any); ok {
			flattenInto(nested, result)
		} else {
			*result = append(*result, item)
		}
	}
}

// ChunkArrayOptimized splits array into chunks
func ChunkArrayOptimized(arr []any, chunkSize int) [][]any {
	if len(arr) == 0 || chunkSize <= 0 {
		return nil
	}

	numChunks := (len(arr) + chunkSize - 1) / chunkSize
	result := make([][]any, numChunks)

	for i := 0; i < numChunks; i++ {
		start := i * chunkSize
		end := start + chunkSize
		if end > len(arr) {
			end = len(arr)
		}
		result[i] = arr[start:end]
	}

	return result
}

// ReverseArrayOptimized reverses array in place
func ReverseArrayOptimized(arr []any) {
	if len(arr) <= 1 {
		return
	}

	for i, j := 0, len(arr)-1; i < j; i, j = i+1, j-1 {
		arr[i], arr[j] = arr[j], arr[i]
	}
}

// TakeFirst returns first n elements
func TakeFirst(arr []any, n int) []any {
	if n <= 0 || len(arr) == 0 {
		return nil
	}
	if n >= len(arr) {
		return arr
	}
	return arr[:n]
}

// TakeLast returns last n elements
func TakeLast(arr []any, n int) []any {
	if n <= 0 || len(arr) == 0 {
		return nil
	}
	if n >= len(arr) {
		return arr
	}
	return arr[len(arr)-n:]
}
