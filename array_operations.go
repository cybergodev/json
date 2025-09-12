package json

import (
	"fmt"
	"strconv"
	"strings"
)

// arrayOperations implements the ArrayOperations interface
type arrayOperations struct {
	utils ProcessorUtils
}

// NewArrayOperations creates a new array operations instance
func NewArrayOperations(utils ProcessorUtils) ArrayOperations {
	return &arrayOperations{
		utils: utils,
	}
}

// HandleArrayAccess handles array index access
func (ao *arrayOperations) HandleArrayAccess(data any, index int) NavigationResult {
	arr, ok := data.([]any)
	if !ok {
		return NavigationResult{Value: nil, Exists: false}
	}

	// Handle negative indices
	normalizedIndex := ao.HandleNegativeIndex(index, len(arr))

	// Check bounds
	if !ao.ValidateArrayBounds(normalizedIndex, len(arr)) {
		return NavigationResult{Value: nil, Exists: false}
	}

	return NavigationResult{Value: arr[normalizedIndex], Exists: true}
}

// HandleArraySlice handles array slice operations
func (ao *arrayOperations) HandleArraySlice(data any, start, end, step int) NavigationResult {
	arr, ok := data.([]any)
	if !ok {
		return NavigationResult{Value: nil, Exists: false}
	}

	// Normalize parameters
	normalizedStart := ao.HandleNegativeIndex(start, len(arr))
	normalizedEnd := ao.HandleNegativeIndex(end, len(arr))

	// Clamp to valid bounds
	normalizedStart = ao.clampIndex(normalizedStart, len(arr))
	normalizedEnd = ao.clampIndex(normalizedEnd, len(arr))

	// Ensure start <= end for positive step
	if step > 0 && normalizedStart > normalizedEnd {
		normalizedStart = normalizedEnd
	}

	// Perform the slice operation
	slicedArray := ao.performArraySlice(arr, normalizedStart, normalizedEnd, step)
	return NavigationResult{Value: slicedArray, Exists: true}
}

// ParseArrayIndex parses an array index from a string
func (ao *arrayOperations) ParseArrayIndex(indexStr string) int {
	// Remove brackets if present
	if strings.HasPrefix(indexStr, "[") && strings.HasSuffix(indexStr, "]") {
		indexStr = indexStr[1 : len(indexStr)-1]
	}

	// Try to parse as integer
	if index, err := strconv.Atoi(indexStr); err == nil {
		return index
	}

	return -999999 // Invalid index marker (using same convention as original code)
}

// HandleNegativeIndex converts negative indices to positive indices
func (ao *arrayOperations) HandleNegativeIndex(index, length int) int {
	if index < 0 {
		return length + index
	}
	return index
}

// ValidateArrayBounds checks if an index is within valid bounds
func (ao *arrayOperations) ValidateArrayBounds(index, length int) bool {
	return index >= 0 && index < length
}

// performArraySlice performs the actual array slicing with step support
func (ao *arrayOperations) performArraySlice(arr []any, start, end, step int) []any {
	if step == 0 {
		step = 1 // Avoid division by zero
	}

	if step == 1 {
		// Simple slice without step
		if start >= 0 && end <= len(arr) && start <= end {
			return arr[start:end]
		}
		return []any{}
	}

	// Slice with step
	var result []any
	if step > 0 {
		// Forward iteration
		for i := start; i < end && i < len(arr); i += step {
			if i >= 0 {
				result = append(result, arr[i])
			}
		}
	} else {
		// Backward iteration
		for i := start; i > end && i >= 0; i += step {
			if i < len(arr) {
				result = append(result, arr[i])
			}
		}
	}

	return result
}

// clampIndex clamps an index to valid bounds for an array
func (ao *arrayOperations) clampIndex(index, length int) int {
	if index < 0 {
		return 0
	}
	if index > length {
		return length
	}
	return index
}

// ParseSliceParameters parses slice parameters from a string like "[start:end:step]"
func (ao *arrayOperations) ParseSliceParameters(sliceStr string, arrayLength int) (start, end, step int, err error) {
	// Default values
	start = 0
	end = arrayLength
	step = 1

	// Remove brackets
	if strings.HasPrefix(sliceStr, "[") && strings.HasSuffix(sliceStr, "]") {
		sliceStr = sliceStr[1 : len(sliceStr)-1]
	}

	// Split by colons
	parts := strings.Split(sliceStr, ":")
	if len(parts) < 2 {
		return 0, 0, 0, fmt.Errorf("slice must have at least start:end format")
	}

	// Parse start
	if parts[0] != "" {
		if start, err = strconv.Atoi(parts[0]); err != nil {
			return 0, 0, 0, fmt.Errorf("invalid start index: %s", parts[0])
		}
	}

	// Parse end
	if parts[1] != "" {
		if end, err = strconv.Atoi(parts[1]); err != nil {
			return 0, 0, 0, fmt.Errorf("invalid end index: %s", parts[1])
		}
	}

	// Parse step (optional)
	if len(parts) > 2 && parts[2] != "" {
		if step, err = strconv.Atoi(parts[2]); err != nil {
			return 0, 0, 0, fmt.Errorf("invalid step: %s", parts[2])
		}
		if step == 0 {
			return 0, 0, 0, fmt.Errorf("step cannot be zero")
		}
	}

	return start, end, step, nil
}

// HandleComplexArrayAccess handles complex array access patterns like "property[index]"
func (ao *arrayOperations) HandleComplexArrayAccess(data any, segmentValue string) NavigationResult {
	// Find bracket positions
	bracketStart := strings.IndexByte(segmentValue, '[')
	bracketEnd := strings.IndexByte(segmentValue, ']')

	if bracketStart == -1 || bracketEnd == -1 || bracketEnd <= bracketStart {
		return NavigationResult{Value: nil, Exists: false}
	}

	property := segmentValue[:bracketStart]
	indexStr := segmentValue[bracketStart+1 : bracketEnd]

	// First access the property if specified
	var arrayData any = data
	if property != "" {
		// This would need to be implemented with property access logic
		// For now, assume data is the target array
		if obj, ok := data.(map[string]any); ok {
			if val, exists := obj[property]; exists {
				arrayData = val
			} else {
				return NavigationResult{Value: nil, Exists: false}
			}
		}
	}

	// Parse and handle array access
	index := ao.ParseArrayIndex(indexStr)
	if index == -999999 {
		return NavigationResult{Value: nil, Exists: false}
	}

	return ao.HandleArrayAccess(arrayData, index)
}

// HandleComplexArraySlice handles complex array slice patterns like "property[start:end:step]"
func (ao *arrayOperations) HandleComplexArraySlice(data any, segmentValue string) NavigationResult {
	// Find bracket positions
	bracketStart := strings.IndexByte(segmentValue, '[')
	bracketEnd := strings.IndexByte(segmentValue, ']')

	if bracketStart == -1 || bracketEnd == -1 || bracketEnd <= bracketStart {
		return NavigationResult{Value: nil, Exists: false}
	}

	property := segmentValue[:bracketStart]
	sliceStr := segmentValue[bracketStart : bracketEnd+1]

	// First access the property if specified
	var arrayData any = data
	if property != "" {
		if obj, ok := data.(map[string]any); ok {
			if val, exists := obj[property]; exists {
				arrayData = val
			} else {
				return NavigationResult{Value: nil, Exists: false}
			}
		}
	}

	// Get array length for slice parsing
	arr, ok := arrayData.([]any)
	if !ok {
		return NavigationResult{Value: nil, Exists: false}
	}

	// Parse slice parameters
	start, end, step, err := ao.ParseSliceParameters(sliceStr, len(arr))
	if err != nil {
		return NavigationResult{Value: nil, Exists: false}
	}

	return ao.HandleArraySlice(arrayData, start, end, step)
}

// ExtendArray extends an array to the specified length, filling with nil values
func (ao *arrayOperations) ExtendArray(arr []any, targetLength int) []any {
	if len(arr) >= targetLength {
		return arr
	}

	// Create new array with target length
	extended := make([]any, targetLength)
	copy(extended, arr)

	// Fill remaining positions with nil
	for i := len(arr); i < targetLength; i++ {
		extended[i] = nil
	}

	return extended
}

// CompactArray removes nil values from an array
func (ao *arrayOperations) CompactArray(arr []any) []any {
	var result []any
	for _, item := range arr {
		if item != nil && item != DeletedMarker {
			result = append(result, item)
		}
	}
	return result
}

// ReverseArray reverses an array in place
func (ao *arrayOperations) ReverseArray(arr []any) {
	for i, j := 0, len(arr)-1; i < j; i, j = i+1, j-1 {
		arr[i], arr[j] = arr[j], arr[i]
	}
}

// IsValidSliceRange checks if a slice range is valid
func (ao *arrayOperations) IsValidSliceRange(start, end, step, arrayLength int) bool {
	// Normalize negative indices
	normalizedStart := ao.HandleNegativeIndex(start, arrayLength)
	normalizedEnd := ao.HandleNegativeIndex(end, arrayLength)

	// Check bounds
	if normalizedStart < 0 || normalizedStart > arrayLength {
		return false
	}
	if normalizedEnd < 0 || normalizedEnd > arrayLength {
		return false
	}

	// Check step
	if step == 0 {
		return false
	}

	// For positive step, start should be <= end
	if step > 0 && normalizedStart > normalizedEnd {
		return false
	}

	// For negative step, start should be >= end
	if step < 0 && normalizedStart < normalizedEnd {
		return false
	}

	return true
}

// GetArrayElement safely gets an element from an array with bounds checking
func (ao *arrayOperations) GetArrayElement(arr []any, index int) (any, bool) {
	normalizedIndex := ao.HandleNegativeIndex(index, len(arr))
	if !ao.ValidateArrayBounds(normalizedIndex, len(arr)) {
		return nil, false
	}
	return arr[normalizedIndex], true
}

// SetArrayElement safely sets an element in an array with bounds checking
func (ao *arrayOperations) SetArrayElement(arr []any, index int, value any) error {
	normalizedIndex := ao.HandleNegativeIndex(index, len(arr))
	if !ao.ValidateArrayBounds(normalizedIndex, len(arr)) {
		return fmt.Errorf("array index %d out of bounds for array of length %d", index, len(arr))
	}
	arr[normalizedIndex] = value
	return nil
}
