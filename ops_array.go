package json

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/cybergodev/json/internal"
)

func (p *Processor) handleArrayAccess(data any, segment PathSegment) PropertyAccessResult {
	var arrayData any = data
	if segment.Key != "" {
		propResult := p.handlePropertyAccess(data, segment.Key)
		if !propResult.Exists {
			return PropertyAccessResult{Value: nil, Exists: false}
		}
		arrayData = propResult.Value
	}

	if arr, ok := arrayData.([]any); ok {
		index := segment.Index
		if index < 0 {
			index = len(arr) + index
		}
		if index >= 0 && index < len(arr) {
			return PropertyAccessResult{Value: arr[index], Exists: true}
		}
		return PropertyAccessResult{Value: nil, Exists: false}
	}

	return PropertyAccessResult{Value: nil, Exists: false}
}

func (p *Processor) parseArrayIndex(indexStr string) int {
	return globalArrayHelper.ParseArrayIndex(indexStr)
}

func (p *Processor) handleArraySlice(data any, segment PathSegment) PropertyAccessResult {
	arr, ok := data.([]any)
	if !ok {
		return PropertyAccessResult{Value: nil, Exists: false}
	}

	result := p.performArraySlice(arr, segment.Start, segment.End, segment.Step)
	return PropertyAccessResult{Value: result, Exists: true}
}

func (p *Processor) performArraySlice(arr []any, start, end, step *int) []any {
	if len(arr) == 0 {
		return []any{}
	}

	actualStart, actualEnd, actualStep := 0, len(arr), 1

	if step != nil {
		actualStep = *step
		if actualStep == 0 {
			return []any{}
		}
	}

	if actualStep < 0 {
		actualStart, actualEnd = len(arr)-1, -1
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

	if actualStep > 0 {
		return p.forwardSlice(arr, actualStart, actualEnd, actualStep)
	}
	return p.reverseSlice(arr, actualStart, actualEnd, actualStep)
}

func (p *Processor) forwardSlice(arr []any, start, end, step int) []any {
	if start < 0 {
		start = 0
	}
	if start >= len(arr) || end <= start {
		return []any{}
	}
	if end > len(arr) {
		end = len(arr)
	}

	result := make([]any, 0, (end-start+step-1)/step)
	for i := start; i < end; i += step {
		result = append(result, arr[i])
	}
	return result
}

func (p *Processor) reverseSlice(arr []any, start, end, step int) []any {
	if start >= len(arr) {
		start = len(arr) - 1
	}
	if start < 0 || end >= start {
		return []any{}
	}

	// Pre-allocate capacity to avoid repeated reallocations
	estimatedSize := (start - end - step - 1) / (-step)
	result := make([]any, 0, estimatedSize)
	for i := start; i > end; i += step { // step is negative
		result = append(result, arr[i])
	}
	return result
}

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

func (p *Processor) isSliceSyntax(segment string) bool {
	return strings.Contains(segment, ":")
}

func (p *Processor) parseSliceSegment(segment *PathSegment) error {
	if segment == nil {
		return fmt.Errorf("segment cannot be nil")
	}

	// Parse slice parameters
	start, end, step, err := p.parseSliceParameters(segment.String(), 0)
	if err != nil {
		return err
	}

	// Set slice properties
	segment.Start = &start
	segment.End = &end
	segment.Step = &step

	return nil
}

func (p *Processor) parseSliceFromSegment(segmentValue string) (start, end, step int) {
	start, end, step, _ = p.parseSliceParameters(segmentValue, 0)
	return start, end, step
}

func (p *Processor) isArrayIndex(segment string) bool {
	// Remove brackets if present
	if strings.HasPrefix(segment, "[") && strings.HasSuffix(segment, "]") {
		segment = segment[1 : len(segment)-1]
	}

	// Check if it's a valid integer
	_, err := strconv.Atoi(segment)
	return err == nil
}

func (p *Processor) isNumericIndex(segment string) bool {
	_, err := strconv.Atoi(segment)
	return err == nil
}

func (p *Processor) isNumericProperty(property string) bool {
	_, err := strconv.Atoi(property)
	return err == nil
}

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
				if next, exists := obj[segment.Key]; exists {
					current = next
				} else {
					return fmt.Errorf("property %s not found", segment.Key)
				}
			} else {
				return fmt.Errorf("cannot access property %s on type %T", segment.Key, current)
			}
		case "array":
			if arr, ok := current.([]any); ok {
				index := segment.Index
				if index < 0 {
					index = len(arr) + index
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
			if arr, ok := obj[lastSegment.Key].([]any); ok {
				// Extend the array
				for len(arr) < requiredLen {
					arr = append(arr, nil)
				}
				obj[lastSegment.Key] = arr
			}
		}
	}

	return nil
}

func (p *Processor) extendArrayAtPath(data any, segments []PathSegment, currentLen, requiredLen int) error {
	return p.extendArrayInPath(data, segments, currentLen, requiredLen)
}

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
				if next, exists := obj[segment.Key]; exists {
					current = next
				} else {
					return fmt.Errorf("property %s not found", segment.Key)
				}
			} else {
				return fmt.Errorf("cannot access property %s on type %T", segment.Key, current)
			}
		case "array":
			if arr, ok := current.([]any); ok {
				index := segment.Index
				if index < 0 {
					index = len(arr) + index
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
			obj[lastSegment.Key] = newArray
		}
	} else if lastSegment.TypeString() == "array" {
		if arr, ok := current.([]any); ok {
			index := lastSegment.Index
			if index < 0 {
				index = len(arr) + index
			}
			if index >= 0 && index < len(arr) {
				arr[index] = newArray
			}
		}
	}

	return nil
}

// detectConsecutiveExtractions identifies groups of consecutive extraction segments
func (p *Processor) detectConsecutiveExtractions(segments []PathSegment) []ExtractionGroup {
	return internal.DetectConsecutiveExtractions(segments)
}
