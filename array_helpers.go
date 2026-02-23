package json

import (
	"strings"

	"github.com/cybergodev/json/internal"
)

// ArrayHelper provides centralized array operation utilities
type ArrayHelper struct{}

// ParseArrayIndex parses an array index from a string
// Delegates to internal implementation for consistency
func (ah *ArrayHelper) ParseArrayIndex(indexStr string) int {
	indexStr = strings.Trim(indexStr, "[] \t")
	if indexStr == "" {
		return InvalidArrayIndex
	}

	if index, ok := internal.ParseArrayIndex(indexStr); ok {
		return index
	}
	return InvalidArrayIndex
}

// NormalizeIndex converts negative indices to positive indices
// Delegates to internal implementation for consistency
func (ah *ArrayHelper) NormalizeIndex(index, length int) int {
	return internal.NormalizeIndex(index, length)
}

// ValidateBounds checks if an index is within valid bounds [0, length)
// Note: This does NOT support negative indices (unlike IsValidIndex in internal)
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
// Delegates to internal implementation for consistency
func (ah *ArrayHelper) GetElement(arr []any, index int) (any, bool) {
	return internal.GetSafeArrayElement(arr, index)
}

// SetElement safely sets an element in an array with bounds checking
// Note: This does NOT support negative indices for bounds checking
func (ah *ArrayHelper) SetElement(arr []any, index int, value any) bool {
	normalizedIndex := ah.NormalizeIndex(index, len(arr))
	// Check bounds on normalized index
	if normalizedIndex < 0 || normalizedIndex >= len(arr) {
		return false
	}
	arr[normalizedIndex] = value
	return true
}

// PerformSlice performs array slicing with step support
// Delegates to internal implementation for consistency
func (ah *ArrayHelper) PerformSlice(arr []any, start, end, step int) []any {
	if len(arr) == 0 || step == 0 {
		return []any{}
	}

	// Convert to pointers for internal API
	startPtr := &start
	endPtr := &end
	stepPtr := &step

	return internal.PerformArraySlice(arr, startPtr, endPtr, stepPtr)
}

// Global array helper instance
var globalArrayHelper = &ArrayHelper{}

// ParseArrayIndexGlobal is a package-level function for backward compatibility
func ParseArrayIndexGlobal(indexStr string) int {
	return globalArrayHelper.ParseArrayIndex(indexStr)
}
