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
	if end < 0 {
		// An end that wrapped past the array start (e.g. [0:-100] on length
		// 5) denotes "nothing selected" — clamp so callers never observe a
		// negative bound from a function documented to clamp out-of-range
		// values.
		end = 0
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

	// Fast path: unit step (step == nil or *step == 1). This is the common case
	// (e.g. [0:50], [:10], [5:]) and avoids allocating a temporary []int index
	// slice — a contiguous subslice is copied directly via copy (memmove).
	// Behavior is identical to the general path for unit step; verified against
	// PerformArraySliceIndices (same normalization, same non-nil empty result).
	if step == nil || *step == 1 {
		startIdx, endIdx := 0, length
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
		if startIdx < 0 {
			startIdx = 0
		}
		if endIdx > length {
			endIdx = length
		}
		// Non-nil empty slice (not nil) for empty results so that a zero-element
		// slice serializes to JSON `[]` rather than `null`. Distinguishes "slice
		// matched nothing" from "path not found" — mirrors the general path.
		if startIdx >= endIdx {
			return make([]any, 0)
		}
		result := make([]any, endIdx-startIdx)
		copy(result, arr[startIdx:endIdx])
		return result
	}

	// General path: non-unit step (e.g. [0:10:2], [::-1]) must walk indices to
	// honor stride and direction.
	indices := PerformArraySliceIndices(length, start, end, step)
	// Return a non-nil empty slice (not nil) for empty results so that a
	// zero-element slice serializes to JSON `[]` (matching Python and the
	// forward dot-notation path) rather than `null`. Distinguishes "slice
	// matched nothing" from "path not found".
	result := make([]any, 0, len(indices))
	for _, i := range indices {
		result = append(result, arr[i])
	}
	return result
}

// PerformArraySliceIndices returns the ordered list of indices that
// PerformArraySlice would visit, honoring both positive and negative step
// (Python-style reverse slicing, e.g. [::-1]).
//
// It exists so that opSet/opDelete can apply a value to — or mark for
// deletion — every element a slice touches without hand-rolling a
// step-direction-dependent loop. A naive `for i := start; i < end; i += step`
// panics on negative step (i decrements below 0 and indexes container[-1]);
// routing through this helper keeps reverse slices safe.
//
// All returned indices are guaranteed to be within [0, length).
func PerformArraySliceIndices(length int, start, end, step *int) []int {
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

		result := make([]int, 0, capacity)
		for i := startIdx; i < endIdx; i += stepVal {
			result = append(result, i)
		}
		return result
	}

	// Negative step
	if startIdx >= length {
		startIdx = length - 1
	}
	if startIdx < 0 {
		// A start that wraps to before the array (e.g. [-10::-1] on length 5)
		// yields nothing under Python semantics — not index 0. Clamping to 0
		// would instead return [arr[0]], diverging from Python.
		return nil
	}
	if endIdx < -1 {
		// An end that wraps past the array start (e.g. [4:-10:-1] on length 5
		// resolves to -5) means "stop at the beginning" — the same clamp
		// applySlice (compiled_path.go) applies. Without it the loop below
		// runs past 0 and callers index with negative values (panic).
		endIdx = -1
	}

	rangeSize := startIdx - endIdx
	capacity := calculateSliceCapacity(rangeSize, -stepVal)

	result := make([]int, 0, capacity)
	for i := startIdx; i > endIdx; i += stepVal {
		result = append(result, i)
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
