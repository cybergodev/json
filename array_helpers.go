package json

import (
	"strconv"
	"strings"
)

// ArrayHelper provides centralized array operation utilities
type ArrayHelper struct{}

// ParseArrayIndex parses an array index from a string (consolidated implementation)
func (ah *ArrayHelper) ParseArrayIndex(indexStr string) int {
	indexStr = strings.Trim(indexStr, "[] \t")
	if indexStr == "" {
		return InvalidArrayIndex
	}
	
	if index, err := strconv.Atoi(indexStr); err == nil {
		return index
	}
	return InvalidArrayIndex
}

// NormalizeIndex converts negative indices to positive indices
func (ah *ArrayHelper) NormalizeIndex(index, length int) int {
	if index < 0 {
		return length + index
	}
	return index
}

// ValidateBounds checks if an index is within valid bounds
func (ah *ArrayHelper) ValidateBounds(index, length int) bool {
	return index >= 0 && index < length
}

// ClampIndex clamps an index to valid bounds [0, length]
func (ah *ArrayHelper) ClampIndex(index, length int) int {
	if index < 0 {
		return 0
	}
	if index > length {
		return length
	}
	return index
}

// CompactArray removes nil values and deletion markers from an array
func (ah *ArrayHelper) CompactArray(arr []any) []any {
	if len(arr) == 0 {
		return arr
	}
	
	result := make([]any, 0, len(arr))
	for _, item := range arr {
		if item != nil && item != DeletedMarker {
			result = append(result, item)
		}
	}
	return result
}

// ExtendArray extends an array to the specified length, filling with nil values
func (ah *ArrayHelper) ExtendArray(arr []any, targetLength int) []any {
	if len(arr) >= targetLength {
		return arr
	}
	
	extended := make([]any, targetLength)
	copy(extended, arr)
	return extended
}

// GetElement safely gets an element from an array with bounds checking
func (ah *ArrayHelper) GetElement(arr []any, index int) (any, bool) {
	normalizedIndex := ah.NormalizeIndex(index, len(arr))
	if !ah.ValidateBounds(normalizedIndex, len(arr)) {
		return nil, false
	}
	return arr[normalizedIndex], true
}

// SetElement safely sets an element in an array with bounds checking
func (ah *ArrayHelper) SetElement(arr []any, index int, value any) bool {
	normalizedIndex := ah.NormalizeIndex(index, len(arr))
	if !ah.ValidateBounds(normalizedIndex, len(arr)) {
		return false
	}
	arr[normalizedIndex] = value
	return true
}

// PerformSlice performs array slicing with step support (optimized)
func (ah *ArrayHelper) PerformSlice(arr []any, start, end, step int) []any {
	if len(arr) == 0 || step == 0 {
		return []any{}
	}
	
	// Normalize indices
	start = ah.NormalizeIndex(start, len(arr))
	end = ah.NormalizeIndex(end, len(arr))
	
	// Clamp to valid bounds
	start = ah.ClampIndex(start, len(arr))
	end = ah.ClampIndex(end, len(arr))
	
	// Simple slice without step
	if step == 1 && start <= end {
		return arr[start:end]
	}
	
	// Slice with step
	var result []any
	if step > 0 {
		for i := start; i < end && i < len(arr); i += step {
			result = append(result, arr[i])
		}
	} else {
		for i := start; i > end && i >= 0; i += step {
			if i < len(arr) {
				result = append(result, arr[i])
			}
		}
	}
	
	return result
}

// Global array helper instance
var globalArrayHelper = &ArrayHelper{}

// ParseArrayIndexGlobal is a package-level function for backward compatibility
func ParseArrayIndexGlobal(indexStr string) int {
	return globalArrayHelper.ParseArrayIndex(indexStr)
}

