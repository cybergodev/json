package internal

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// NormalizeIndex converts a negative index to a positive one using Python-style wrapping.
// Returns the normalized index, which may be negative if the negative index exceeds the length.
// Callers MUST validate the result is within [0, length) before using it as an array index.
func NormalizeIndex(index, length int) int {
	if index < 0 {
		return length + index
	}
	return index
}

// ParseArrayIndex parses a string as an array index integer.
// Returns the index and true if valid, or 0 and false otherwise.
func ParseArrayIndex(property string) (int, bool) {
	return ParseIntFast(property)
}

// ParseSliceComponents parses a slice expression like "1:5:2" into start, end, step pointers.
// Missing components are returned as nil.
func ParseSliceComponents(slicePart string) (start, end, step *int, err error) {
	if slicePart == ":" {
		return nil, nil, nil, nil
	}

	parts := strings.Split(slicePart, ":")
	if len(parts) < 2 || len(parts) > 3 {
		return nil, nil, nil, fmt.Errorf("invalid slice format, expected [start:end] or [start:end:step]")
	}

	if parts[0] != "" {
		startVal, parseErr := strconv.Atoi(parts[0])
		if parseErr != nil {
			return nil, nil, nil, fmt.Errorf("invalid start index: %s", parts[0])
		}
		start = &startVal
	}

	if parts[1] != "" {
		endVal, parseErr := strconv.Atoi(parts[1])
		if parseErr != nil {
			return nil, nil, nil, fmt.Errorf("invalid end index: %s", parts[1])
		}
		end = &endVal
	}

	if len(parts) == 3 && parts[2] != "" {
		stepVal, parseErr := strconv.Atoi(parts[2])
		if parseErr != nil {
			return nil, nil, nil, fmt.Errorf("invalid step value: %s", parts[2])
		}
		if stepVal == 0 {
			return nil, nil, nil, fmt.Errorf("step cannot be zero")
		}
		step = &stepVal
	}

	return start, end, step, nil
}

// NormalizeSlice normalizes slice bounds using Python-style semantics.
// Negative indices wrap from the end. Out-of-bounds values are clamped.
// If start > end after normalization, start is set to end (produces empty range).
// This matches Python slice behavior where [5:3] returns an empty slice.
func NormalizeSlice(start, end, length int) (int, int) {
	if start < 0 {
		start = length + start
	}
	if end < 0 {
		end = length + end
	}

	if start < 0 {
		start = 0
	}
	if end > length {
		end = length
	}
	if start > end {
		start = end
	}

	return start, end
}

// PerformArraySlice performs Python-style array slicing with optimized capacity calculation
func PerformArraySlice(arr []any, start, end, step *int) []any {
	length := len(arr)
	if length == 0 {
		return nil
	}

	startIdx, endIdx, stepVal := 0, length, 1

	if step != nil {
		stepVal = *step
		if stepVal == 0 {
			return nil
		}
	}

	if stepVal < 0 {
		if start == nil {
			startIdx = length - 1
		}
		if end == nil {
			endIdx = -1
		}
	}

	if start != nil {
		startIdx = *start
		if startIdx < 0 {
			startIdx += length
		}
	}

	if end != nil {
		endIdx = *end
		if endIdx < 0 {
			endIdx += length
		}
	}

	if stepVal > 0 {
		if startIdx < 0 {
			startIdx = 0
		}
		if endIdx > length {
			endIdx = length
		}
		if startIdx >= endIdx {
			return nil
		}

		rangeSize := endIdx - startIdx
		capacity := calculateSliceCapacity(rangeSize, stepVal)

		result := make([]any, 0, capacity)
		for i := startIdx; i < endIdx; i += stepVal {
			result = append(result, arr[i])
		}
		return result
	}

	// Negative step
	if startIdx >= length {
		startIdx = length - 1
	}
	if startIdx < 0 {
		startIdx = 0
	}

	rangeSize := startIdx - endIdx
	capacity := calculateSliceCapacity(rangeSize, -stepVal)

	result := make([]any, 0, capacity)
	for i := startIdx; i > endIdx; i += stepVal {
		result = append(result, arr[i])
	}
	return result
}

func calculateSliceCapacity(rangeSize, step int) int {
	if rangeSize <= 0 || step <= 0 {
		return 0
	}
	if rangeSize > math.MaxInt32 {
		return 0
	}
	return (rangeSize-1)/step + 1
}

// IsValidIndex checks whether the given index is within bounds [0, length)
// after normalizing negative indices.
func IsValidIndex(index, length int) bool {
	normalizedIndex := NormalizeIndex(index, length)
	return normalizedIndex >= 0 && normalizedIndex < length
}

// GetSafeArrayElement retrieves an element by index with bounds checking.
// Supports negative indices. Returns the element and true, or nil and false.
func GetSafeArrayElement(arr []any, index int) (any, bool) {
	normalizedIndex := NormalizeIndex(index, len(arr))
	if normalizedIndex < 0 || normalizedIndex >= len(arr) {
		return nil, false
	}
	return arr[normalizedIndex], true
}
