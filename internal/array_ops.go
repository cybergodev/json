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
		if !IsNilOrEmpty(item) {
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

// UniqueArrayOptimized removes duplicates from array using map for O(n) lookup.
// Always returns a new slice; the input is never modified or returned directly.
func UniqueArrayOptimized(arr []any) []any {
	if len(arr) == 0 {
		return []any{}
	}
	if len(arr) == 1 {
		cp := make([]any, 1)
		cp[0] = arr[0]
		return cp
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

// maxFlattenDepth limits recursion depth for flatten operations to prevent stack overflow
const maxFlattenDepth = 200

// FlattenArrayOptimized flattens nested arrays with pooling
func FlattenArrayOptimized(arr []any) []any {
	if len(arr) == 0 {
		return []any{}
	}

	// Estimate final size
	estimatedSize := estimateFlatSize(arr, 0)
	result := make([]any, 0, estimatedSize)

	flattenInto(arr, &result, 0)
	return result
}

func estimateFlatSize(arr []any, depth int) int {
	if depth >= maxFlattenDepth {
		return len(arr)
	}
	size := 0
	for _, item := range arr {
		if nested, ok := item.([]any); ok {
			size += estimateFlatSize(nested, depth+1)
		} else {
			size++
		}
	}
	return size
}

func flattenInto(arr []any, result *[]any, depth int) {
	if depth >= maxFlattenDepth {
		*result = append(*result, arr...)
		return
	}
	for _, item := range arr {
		if nested, ok := item.([]any); ok {
			flattenInto(nested, result, depth+1)
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

	for i := range numChunks {
		start := i * chunkSize
		end := min(start+chunkSize, len(arr))
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

// TakeFirst returns first n elements as a new slice.
// The returned slice is independent of the input; modifications do not affect the original.
func TakeFirst(arr []any, n int) []any {
	if n <= 0 || len(arr) == 0 {
		return nil
	}
	if n > len(arr) {
		n = len(arr)
	}
	result := make([]any, n)
	copy(result, arr[:n])
	return result
}

// TakeLast returns last n elements as a new slice.
// The returned slice is independent of the input; modifications do not affect the original.
func TakeLast(arr []any, n int) []any {
	if n <= 0 || len(arr) == 0 {
		return nil
	}
	if n > len(arr) {
		n = len(arr)
	}
	result := make([]any, n)
	copy(result, arr[len(arr)-n:])
	return result
}
