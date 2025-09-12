package json

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/cybergodev/json/internal"
)

// handleArrayAccess handles array access with support for negative indices
func (p *Processor) handleArrayAccess(data any, segment PathSegment) PropertyAccessResult {
	segmentValue := segment.Value

	// Fast bracket position finding
	bracketStart := strings.IndexByte(segmentValue, '[')
	bracketEnd := strings.IndexByte(segmentValue, ']')

	if bracketStart == -1 || bracketEnd == -1 || bracketEnd <= bracketStart {
		return PropertyAccessResult{Value: nil, Exists: false}
	}

	property := segmentValue[:bracketStart]
	indexStr := segmentValue[bracketStart+1 : bracketEnd]

	// Get the array with property access
	var arrayResult PropertyAccessResult
	if property != "" {
		arrayResult = p.handlePropertyAccess(data, property)
		if !arrayResult.Exists {
			return PropertyAccessResult{Value: nil, Exists: false}
		}
	} else {
		arrayResult = PropertyAccessResult{Value: data, Exists: true}
	}

	// Handle array indexing with optimized parsing (supporting negative indices)
	if arr, ok := arrayResult.Value.([]any); ok {
		if index := p.parseArrayIndexFromSegment(indexStr); index != -999999 { // -999999 means invalid index
			// Handle negative indices
			if index < 0 {
				index = len(arr) + index
			}
			// Check bounds after negative index conversion
			if index >= 0 && index < len(arr) {
				return PropertyAccessResult{Value: arr[index], Exists: true}
			}
		}
		// Index out of bounds on valid array - boundary case
		return PropertyAccessResult{Value: nil, Exists: false}
	}

	// Attempting array access on non-array type - this should be treated as a structural error
	// Return false to indicate this is not a valid array access
	return PropertyAccessResult{Value: nil, Exists: false}
}

// handleArrayAccessValue returns the value directly (for backward compatibility)
func (p *Processor) handleArrayAccessValue(data any, segment string) any {
	// Convert string segment to PathSegment for processing
	pathSegment := internal.NewLegacyPathSegment("array", segment)
	result := p.handleArrayAccess(data, pathSegment)
	if !result.Exists {
		return nil
	}
	return result.Value
}

// parseArrayIndexFromSegment parses array index from segment value
func (p *Processor) parseArrayIndexFromSegment(segmentValue string) int {
	// Handle bracket notation
	if strings.HasPrefix(segmentValue, "[") && strings.HasSuffix(segmentValue, "]") {
		segmentValue = segmentValue[1 : len(segmentValue)-1]
	}

	// Try to parse as integer
	if index, err := strconv.Atoi(segmentValue); err == nil {
		return index
	}

	return -999999 // Invalid index marker
}

// handleArraySlice handles array slicing operations
func (p *Processor) handleArraySlice(data any, segment PathSegment) PropertyAccessResult {
	// Get the array
	var arr []any
	var ok bool
	if arr, ok = data.([]any); !ok {
		return PropertyAccessResult{Value: nil, Exists: false}
	}

	// Use slice parameters from the segment
	result := p.performArraySlice(arr, segment.Start, segment.End, segment.Step)
	return PropertyAccessResult{Value: result, Exists: true}
}

// performArraySlice performs the actual array slicing
func (p *Processor) performArraySlice(arr []any, start, end, step *int) []any {
	if len(arr) == 0 {
		return []any{}
	}

	// Set defaults
	actualStart := 0
	actualEnd := len(arr)
	actualStep := 1

	if step != nil {
		actualStep = *step
	}

	// Handle negative step (reverse slicing)
	if actualStep < 0 {
		// For negative step, default start is end-1, default end is -1
		actualStart = len(arr) - 1
		actualEnd = -1
	}

	if start != nil {
		actualStart = *start
		if actualStart < 0 {
			actualStart = len(arr) + actualStart
		}
	}

	if end != nil {
		actualEnd = *end
		if actualEnd < 0 {
			actualEnd = len(arr) + actualEnd
		}
	}

	// Bounds checking for positive step
	if actualStep > 0 {
		if actualStart < 0 {
			actualStart = 0
		}
		if actualStart >= len(arr) {
			return []any{}
		}
		if actualEnd > len(arr) {
			actualEnd = len(arr)
		}
		if actualEnd <= actualStart {
			return []any{}
		}

		// Perform forward slicing
		result := make([]any, 0)
		for i := actualStart; i < actualEnd; i += actualStep {
			result = append(result, arr[i])
		}
		return result
	} else {
		// Bounds checking for negative step
		if actualStart >= len(arr) {
			actualStart = len(arr) - 1
		}
		if actualStart < 0 {
			return []any{}
		}
		if actualEnd < -1 {
			actualEnd = -1
		}
		if actualEnd >= actualStart {
			return []any{}
		}

		// Perform reverse slicing
		result := make([]any, 0)
		for i := actualStart; i > actualEnd; i += actualStep {
			result = append(result, arr[i])
		}
		return result
	}
}

// parseSliceParameters parses slice parameters from a string like "1:5:2"
func (p *Processor) parseSliceParameters(segmentValue string, arrayLength int) (start, end, step int, err error) {
	// Remove brackets if present
	if strings.HasPrefix(segmentValue, "[") && strings.HasSuffix(segmentValue, "]") {
		segmentValue = segmentValue[1 : len(segmentValue)-1]
	}

	// Split by colons
	parts := strings.Split(segmentValue, ":")
	if len(parts) < 2 || len(parts) > 3 {
		return 0, 0, 0, fmt.Errorf("invalid slice syntax: %s", segmentValue)
	}

	// Parse start
	if parts[0] == "" {
		start = 0
	} else {
		if start, err = strconv.Atoi(parts[0]); err != nil {
			return 0, 0, 0, fmt.Errorf("invalid start index: %s", parts[0])
		}
	}

	// Parse end
	if parts[1] == "" {
		end = arrayLength
	} else {
		if end, err = strconv.Atoi(parts[1]); err != nil {
			return 0, 0, 0, fmt.Errorf("invalid end index: %s", parts[1])
		}
	}

	// Parse step (optional)
	step = 1
	if len(parts) == 3 {
		if parts[2] != "" {
			if step, err = strconv.Atoi(parts[2]); err != nil {
				return 0, 0, 0, fmt.Errorf("invalid step: %s", parts[2])
			}
			if step <= 0 {
				return 0, 0, 0, fmt.Errorf("step must be positive: %d", step)
			}
		}
	}

	return start, end, step, nil
}

// parseSliceParametersWithExtension parses slice parameters with array extension support
func (p *Processor) parseSliceParametersWithExtension(segmentValue string, arrayLength int) (start, end, step int, err error) {
	start, end, step, err = p.parseSliceParameters(segmentValue, arrayLength)
	if err != nil {
		return 0, 0, 0, err
	}

	// Handle negative indices
	if start < 0 {
		start = arrayLength + start
	}
	if end < 0 {
		end = arrayLength + end
	}

	// Bounds checking with extension support
	if start < 0 {
		start = 0
	}
	if end < 0 {
		end = 0
	}

	return start, end, step, nil
}

// isSliceSyntax checks if a segment contains slice syntax (contains colons)
func (p *Processor) isSliceSyntax(segment string) bool {
	return strings.Contains(segment, ":")
}

// parseSliceSegment parses a slice segment and populates its properties
func (p *Processor) parseSliceSegment(segment *PathSegment) error {
	if segment == nil {
		return fmt.Errorf("segment cannot be nil")
	}

	// Parse slice parameters
	start, end, step, err := p.parseSliceParameters(segment.Value, 0)
	if err != nil {
		return err
	}

	// Set slice properties
	segment.Start = &start
	segment.End = &end
	segment.Step = &step

	return nil
}

// parseSliceFromSegment extracts slice parameters from segment value
func (p *Processor) parseSliceFromSegment(segmentValue string) (start, end, step int) {
	start, end, step, _ = p.parseSliceParameters(segmentValue, 0)
	return start, end, step
}

// isArrayIndex checks if a segment represents an array index
func (p *Processor) isArrayIndex(segment string) bool {
	// Remove brackets if present
	if strings.HasPrefix(segment, "[") && strings.HasSuffix(segment, "]") {
		segment = segment[1 : len(segment)-1]
	}

	// Check if it's a valid integer
	_, err := strconv.Atoi(segment)
	return err == nil
}

// isNumericIndex checks if a string represents a numeric index
func (p *Processor) isNumericIndex(segment string) bool {
	_, err := strconv.Atoi(segment)
	return err == nil
}

// isNumericProperty checks if a property name is numeric
func (p *Processor) isNumericProperty(property string) bool {
	_, err := strconv.Atoi(property)
	return err == nil
}

// navigateToArrayIndex navigates to a specific array index
func (p *Processor) navigateToArrayIndex(current any, index int, createPaths bool) (any, error) {
	switch v := current.(type) {
	case []any:
		if index < 0 || index >= len(v) {
			return nil, fmt.Errorf("array index %d out of bounds (length %d)", index, len(v))
		}
		return v[index], nil
	default:
		return nil, fmt.Errorf("cannot access array index %d on type %T", index, current)
	}
}

// navigateToArrayIndexWithNegative navigates to array index with negative index support
func (p *Processor) navigateToArrayIndexWithNegative(current any, index int, createPaths bool) (any, error) {
	switch v := current.(type) {
	case []any:
		// Handle negative indices
		if index < 0 {
			index = len(v) + index
		}

		if index < 0 || index >= len(v) {
			if createPaths && index == len(v) {
				// Extend array by one element
				return nil, nil // Placeholder for new element
			}
			return nil, fmt.Errorf("array index %d out of bounds (length %d)", index, len(v))
		}
		return v[index], nil
	default:
		return nil, fmt.Errorf("cannot access array index %d on type %T", index, current)
	}
}

// tryConvertToArray attempts to convert a map to an array if it has numeric keys
func (p *Processor) tryConvertToArray(m map[string]any) ([]any, bool) {
	if len(m) == 0 {
		return []any{}, true
	}

	// Check if all keys are numeric and sequential
	maxIndex := -1
	for key := range m {
		if index, err := strconv.Atoi(key); err == nil && index >= 0 {
			if index > maxIndex {
				maxIndex = index
			}
		} else {
			return nil, false // Non-numeric key found
		}
	}

	// Create array with proper size
	arr := make([]any, maxIndex+1)
	for key, value := range m {
		if index, err := strconv.Atoi(key); err == nil {
			arr[index] = value
		}
	}

	return arr, true
}

// extendAndSetArray extends an array and sets a value at the specified index
func (p *Processor) extendAndSetArray(arr []any, index int, value any) error {
	if index < 0 {
		return fmt.Errorf("cannot extend array with negative index: %d", index)
	}

	// Check if we need to extend the array
	if index >= len(arr) {
		// Extend array with nil values
		for len(arr) <= index {
			arr = append(arr, nil)
		}
	}

	// Set the value
	arr[index] = value
	return nil
}

// assignValueToSlice assigns a value to a slice range
func (p *Processor) assignValueToSlice(arr []any, start, end, step int, value any) error {
	if start < 0 || end > len(arr) || start >= end {
		return fmt.Errorf("invalid slice range: [%d:%d] for array length %d", start, end, len(arr))
	}

	if step <= 0 {
		return fmt.Errorf("step must be positive: %d", step)
	}

	// Assign value to each position in the slice
	for i := start; i < end; i += step {
		arr[i] = value
	}

	return nil
}

// cleanupArrayNulls removes null values from an array
func (p *Processor) cleanupArrayNulls(arr []any) {
	writeIndex := 0
	for readIndex := 0; readIndex < len(arr); readIndex++ {
		if arr[readIndex] != nil {
			if writeIndex != readIndex {
				arr[writeIndex] = arr[readIndex]
			}
			writeIndex++
		}
	}

	// Clear the remaining elements
	for i := writeIndex; i < len(arr); i++ {
		arr[i] = nil
	}

	// Truncate the slice
	arr = arr[:writeIndex]
}

// cleanupArrayWithReconstruction cleans up an array and returns a new one
func (p *Processor) cleanupArrayWithReconstruction(arr []any, compactArrays bool) []any {
	if !compactArrays {
		return arr
	}

	result := make([]any, 0, len(arr))
	for _, item := range arr {
		if item != nil {
			// Recursively clean nested structures
			if nestedArr, ok := item.([]any); ok {
				item = p.cleanupArrayWithReconstruction(nestedArr, compactArrays)
			} else if nestedMap, ok := item.(map[string]any); ok {
				item = p.cleanupNullValuesRecursiveWithReconstruction(nestedMap, compactArrays)
			}
			result = append(result, item)
		}
	}

	return result
}

// extendArrayInPath extends an array at a specific path
func (p *Processor) extendArrayInPath(data any, segments []PathSegment, currentLen, requiredLen int) error {
	if requiredLen <= currentLen {
		return nil // No extension needed
	}

	// Navigate to the parent of the array
	current := data
	for i := 0; i < len(segments)-1; i++ {
		segment := segments[i]
		switch segment.TypeString() {
		case "property":
			if obj, ok := current.(map[string]any); ok {
				if next, exists := obj[segment.Value]; exists {
					current = next
				} else {
					return fmt.Errorf("property %s not found", segment.Value)
				}
			} else {
				return fmt.Errorf("cannot access property %s on type %T", segment.Value, current)
			}
		case "array":
			if arr, ok := current.([]any); ok {
				index, err := strconv.Atoi(segment.Value)
				if err != nil {
					return fmt.Errorf("invalid array index: %s", segment.Value)
				}
				if index >= 0 && index < len(arr) {
					current = arr[index]
				} else {
					return fmt.Errorf("array index %d out of bounds", index)
				}
			} else {
				return fmt.Errorf("cannot access array index on type %T", current)
			}
		}
	}

	// The last segment should point to the array to extend
	lastSegment := segments[len(segments)-1]
	if lastSegment.TypeString() == "property" {
		if obj, ok := current.(map[string]any); ok {
			if arr, ok := obj[lastSegment.Value].([]any); ok {
				// Extend the array
				for len(arr) < requiredLen {
					arr = append(arr, nil)
				}
				obj[lastSegment.Value] = arr
			}
		}
	}

	return nil
}

// extendArrayAtPath extends an array at the specified path
func (p *Processor) extendArrayAtPath(data any, segments []PathSegment, currentLen, requiredLen int) error {
	return p.extendArrayInPath(data, segments, currentLen, requiredLen)
}

// replaceArrayInParentContext replaces an array in its parent context
func (p *Processor) replaceArrayInParentContext(data any, segments []PathSegment, arrayIndex int, newArray []any) error {
	if len(segments) == 0 {
		return fmt.Errorf("no segments provided")
	}

	// Navigate to the parent
	current := data
	for i := 0; i < len(segments)-1; i++ {
		segment := segments[i]
		switch segment.TypeString() {
		case "property":
			if obj, ok := current.(map[string]any); ok {
				if next, exists := obj[segment.Value]; exists {
					current = next
				} else {
					return fmt.Errorf("property %s not found", segment.Value)
				}
			} else {
				return fmt.Errorf("cannot access property %s on type %T", segment.Value, current)
			}
		case "array":
			if arr, ok := current.([]any); ok {
				index, err := strconv.Atoi(segment.Value)
				if err != nil {
					return fmt.Errorf("invalid array index: %s", segment.Value)
				}
				if index >= 0 && index < len(arr) {
					current = arr[index]
				} else {
					return fmt.Errorf("array index %d out of bounds", index)
				}
			} else {
				return fmt.Errorf("cannot access array index on type %T", current)
			}
		}
	}

	// Replace the array in the parent
	lastSegment := segments[len(segments)-1]
	if lastSegment.TypeString() == "property" {
		if obj, ok := current.(map[string]any); ok {
			obj[lastSegment.Value] = newArray
		}
	} else if lastSegment.TypeString() == "array" {
		if arr, ok := current.([]any); ok {
			index, err := strconv.Atoi(lastSegment.Value)
			if err != nil {
				return fmt.Errorf("invalid array index: %s", lastSegment.Value)
			}
			if index >= 0 && index < len(arr) {
				arr[index] = newArray
			}
		}
	}

	return nil
}
