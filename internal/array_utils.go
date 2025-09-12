package internal

import (
	"fmt"
	"strconv"
	"strings"
)

// ArrayUtils provides unified array index and slice operations
type ArrayUtils struct{}

// NewArrayUtils creates a new array utilities instance
func NewArrayUtils() *ArrayUtils {
	return &ArrayUtils{}
}

// NormalizeIndex normalizes array index, handling negative indices
func (au *ArrayUtils) NormalizeIndex(index, length int) int {
	if index < 0 {
		return length + index
	}
	return index
}

// NormalizeSlice normalizes slice bounds, handling negative indices and bounds checking
func (au *ArrayUtils) NormalizeSlice(start, end, length int) (int, int) {
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

// ParseArrayIndex parses array index with performance, supporting negative indices
func (au *ArrayUtils) ParseArrayIndex(property string) int {
	// Fast path for single digit indices
	if len(property) == 1 && property[0] >= '0' && property[0] <= '9' {
		return int(property[0] - '0')
	}

	// Use strconv.Atoi for multi-digit indices (including negative)
	if index, err := strconv.Atoi(property); err == nil {
		return index
	}

	return -999999 // Invalid index (use a very negative number to distinguish from valid negative indices)
}

// ParseSliceFromSegment parses slice parameters from a segment string
func (au *ArrayUtils) ParseSliceFromSegment(segmentValue string) (start, end, step int) {
	// Remove brackets if present
	value := strings.Trim(segmentValue, "[]")

	// Handle empty slice (all elements)
	if value == "" || value == ":" {
		return 0, -1, 1
	}

	// Handle step notation (e.g., "1:5:2")
	parts := strings.Split(value, ":")

	switch len(parts) {
	case 1:
		// Single index, treat as start:start+1
		if idx, err := strconv.Atoi(parts[0]); err == nil {
			start, end = idx, idx+1
		} else {
			start = -999999 // Invalid
		}
	case 2:
		// start:end
		start = 0 // Default start
		if parts[0] != "" {
			if s, err := strconv.Atoi(parts[0]); err == nil {
				start = s
			}
		}
		end = -1 // Default to end of array
		if parts[1] != "" {
			if e, err := strconv.Atoi(parts[1]); err == nil {
				end = e
			}
		}
	case 3:
		// start:end:step
		start = 0
		if parts[0] != "" {
			if s, err := strconv.Atoi(parts[0]); err == nil {
				start = s
			}
		}
		end = -1
		if parts[1] != "" {
			if e, err := strconv.Atoi(parts[1]); err == nil {
				end = e
			}
		}
		step = 1
		if parts[2] != "" {
			if st, err := strconv.Atoi(parts[2]); err == nil && st > 0 {
				step = st
			}
		}
	default:
		start = -999999 // Invalid
	}

	return start, end, step
}

// ParseSliceComponents parses slice syntax like "1:3", "::2", "::-1" into components
func (au *ArrayUtils) ParseSliceComponents(slicePart string) (start, end, step *int, err error) {
	// Handle empty slice [:]
	if slicePart == ":" {
		return nil, nil, nil, nil
	}

	// Split by colons
	parts := strings.Split(slicePart, ":")

	// Valid number of parts
	if len(parts) < 2 || len(parts) > 3 {
		return nil, nil, nil, fmt.Errorf("invalid slice format, expected [start:end] or [start:end:step]")
	}

	// Parse start
	if parts[0] != "" {
		if startVal, parseErr := strconv.Atoi(parts[0]); parseErr != nil {
			return nil, nil, nil, fmt.Errorf("invalid start index: %s", parts[0])
		} else {
			start = &startVal
		}
	}

	// Parse end
	if parts[1] != "" {
		if endVal, parseErr := strconv.Atoi(parts[1]); parseErr != nil {
			return nil, nil, nil, fmt.Errorf("invalid end index: %s", parts[1])
		} else {
			end = &endVal
		}
	}

	// Parse step (if provided)
	if len(parts) == 3 {
		if parts[2] != "" {
			if stepVal, parseErr := strconv.Atoi(parts[2]); parseErr != nil {
				return nil, nil, nil, fmt.Errorf("invalid step value: %s", parts[2])
			} else if stepVal == 0 {
				return nil, nil, nil, fmt.Errorf("step cannot be zero")
			} else {
				step = &stepVal
			}
		}
	}

	return start, end, step, nil
}

// ParseSliceParameters parses slice parameters with bounds checking
func (au *ArrayUtils) ParseSliceParameters(segmentValue string, arrayLength int) (start, end, step int, err error) {
	start, end, step = au.ParseSliceFromSegment(segmentValue)
	if start == -999999 {
		return 0, 0, 1, fmt.Errorf("invalid slice syntax: %s", segmentValue)
	}

	// Handle default end value
	if end == -1 {
		end = arrayLength
	}

	// Normalize negative indices
	start, end = au.NormalizeSlice(start, end, arrayLength)

	return start, end, step, nil
}

// PerformArraySlice performs Python-style array slicing
func (au *ArrayUtils) PerformArraySlice(arr []any, start, end, step *int) []any {
	length := len(arr)
	if length == 0 {
		return []any{}
	}

	// Default values
	startIdx := 0
	endIdx := length
	stepVal := 1

	// Set step value
	if step != nil {
		stepVal = *step
		if stepVal == 0 {
			return []any{} // Invalid step
		}
	}

	// Handle negative step (reverse iteration) - set proper defaults
	if stepVal < 0 {
		// For negative step, default start is end-1, default end is before first element
		if start == nil {
			startIdx = length - 1
		}
		if end == nil {
			endIdx = -1 // Before first element
		}
	}

	// Set start index
	if start != nil {
		startIdx = *start
		if startIdx < 0 {
			startIdx += length // Convert negative index
		}
	}

	// Set end index
	if end != nil {
		endIdx = *end
		if endIdx < 0 {
			endIdx += length // Convert negative index
		}
	}

	// Perform slicing
	var result []any

	if stepVal > 0 {
		// Forward iteration
		if startIdx < 0 {
			startIdx = 0
		}
		if endIdx > length {
			endIdx = length
		}
		if startIdx >= endIdx {
			return []any{}
		}

		for i := startIdx; i < endIdx; i += stepVal {
			if i >= 0 && i < length {
				result = append(result, arr[i])
			}
		}
	} else {
		// Backward iteration
		if startIdx >= length {
			startIdx = length - 1
		}
		if startIdx < 0 {
			startIdx = 0
		}

		// For negative step, we iterate while i > endIdx
		for i := startIdx; i > endIdx; i += stepVal { // stepVal is negative
			if i >= 0 && i < length {
				result = append(result, arr[i])
			}
		}
	}

	return result
}

// IsValidIndex checks if an index is valid for the given array length
func (au *ArrayUtils) IsValidIndex(index, length int) bool {
	normalizedIndex := au.NormalizeIndex(index, length)
	return normalizedIndex >= 0 && normalizedIndex < length
}

// GetSafeArrayElement safely gets an array element with bounds checking
func (au *ArrayUtils) GetSafeArrayElement(arr []any, index int) (any, bool) {
	if !au.IsValidIndex(index, len(arr)) {
		return nil, false
	}
	normalizedIndex := au.NormalizeIndex(index, len(arr))
	return arr[normalizedIndex], true
}
