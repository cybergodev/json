package internal

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// NormalizeIndex normalizes array index, handling negative indices
func NormalizeIndex(index, length int) int {
	if index < 0 {
		return length + index
	}
	return index
}

// ParseArrayIndex parses array index, supporting negative indices
// Returns the parsed index and a boolean indicating success
func ParseArrayIndex(property string) (int, bool) {
	// Fast path for single digit
	if len(property) == 1 && property[0] >= '0' && property[0] <= '9' {
		return int(property[0] - '0'), true
	}

	// Parse as integer
	if index, err := strconv.Atoi(property); err == nil {
		return index, true
	}

	return 0, false
}

// ParseSliceComponents parses slice syntax into components
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

// NormalizeSlice normalizes slice bounds
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
		return []any{}
	}

	startIdx, endIdx, stepVal := 0, length, 1

	if step != nil {
		stepVal = *step
		if stepVal == 0 {
			return []any{}
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

	var result []any

	if stepVal > 0 {
		// Clamp indices to valid range
		if startIdx < 0 {
			startIdx = 0
		}
		if endIdx > length {
			endIdx = length
		}
		if startIdx >= endIdx {
			return []any{}
		}

		// Calculate capacity safely to avoid overflow
		rangeSize := endIdx - startIdx
		capacity := calculateSliceCapacity(rangeSize, stepVal)
		if capacity > 0 && capacity <= length {
			result = make([]any, 0, capacity)
		} else {
			result = []any{}
		}

		for i := startIdx; i < endIdx; i += stepVal {
			result = append(result, arr[i])
		}
	} else {
		// Negative step
		if startIdx >= length {
			startIdx = length - 1
		}
		if startIdx < 0 {
			startIdx = 0
		}

		// Calculate capacity for negative step
		rangeSize := startIdx - endIdx
		capacity := calculateSliceCapacity(rangeSize, -stepVal)
		if capacity > 0 && capacity <= length {
			result = make([]any, 0, capacity)
		} else {
			result = []any{}
		}

		for i := startIdx; i > endIdx; i += stepVal {
			result = append(result, arr[i])
		}
	}

	return result
}

// calculateSliceCapacity safely calculates slice capacity to avoid overflow
func calculateSliceCapacity(rangeSize, step int) int {
	if rangeSize <= 0 || step <= 0 {
		return 0
	}
	// Avoid overflow by checking if rangeSize is too large
	if rangeSize > math.MaxInt32 {
		return 0
	}
	return (rangeSize + step - 1) / step
}

// IsValidIndex checks if an index is valid
func IsValidIndex(index, length int) bool {
	normalizedIndex := NormalizeIndex(index, length)
	return normalizedIndex >= 0 && normalizedIndex < length
}

// GetSafeArrayElement safely gets an array element with bounds checking
func GetSafeArrayElement(arr []any, index int) (any, bool) {
	normalizedIndex := NormalizeIndex(index, len(arr))
	if normalizedIndex < 0 || normalizedIndex >= len(arr) {
		return nil, false
	}
	return arr[normalizedIndex], true
}
