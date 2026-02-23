package json

import (
	"fmt"
	"strings"
)

// Array extension and index/slice operations

func (p *Processor) handleArrayExtensionAndSet(data any, segments []PathSegment, arrayExtErr *ArrayExtensionNeededError) error {
	if len(segments) == 0 {
		return fmt.Errorf("no segments provided for array extension")
	}

	// Navigate to the parent of the array that needs extension
	current := data
	for i := 0; i < len(segments)-1; i++ {
		next, err := p.navigateToSegment(current, segments[i], true, segments, i)
		if err != nil {
			return fmt.Errorf("failed to navigate to segment %d during array extension: %w", i, err)
		}
		current = next
	}

	// Get the final segment (can be array or slice)
	finalSegment := segments[len(segments)-1]

	switch finalSegment.TypeString() {
	case "array":
		// Handle simple array index extension
		return p.handleArrayIndexExtension(current, finalSegment, arrayExtErr)
	case "slice":
		// Handle array slice extension
		return p.handleArraySliceExtension(current, finalSegment, arrayExtErr)
	default:
		return fmt.Errorf("expected array or slice segment for array extension, got %s", finalSegment.TypeString())
	}
}

func (p *Processor) handleArrayIndexExtension(current any, segment PathSegment, arrayExtErr *ArrayExtensionNeededError) error {
	// For array index access, current should be the array that needs extension
	arr, ok := current.([]any)
	if !ok {
		return fmt.Errorf("expected array for index extension, got %T", current)
	}

	// Create extended array
	extendedArr := make([]any, arrayExtErr.RequiredLength)
	copy(extendedArr, arr)

	// Set the value at target index
	extendedArr[arrayExtErr.Start] = arrayExtErr.Value

	// The problem is we can't replace the array reference from here
	// We need to handle this at a higher level
	// For now, try to extend in place if possible
	if cap(arr) >= arrayExtErr.RequiredLength {
		// Extend the slice in place
		for len(arr) < arrayExtErr.RequiredLength {
			arr = append(arr, nil)
		}
		arr[arrayExtErr.Start] = arrayExtErr.Value
		return nil
	}

	// Cannot extend in place - this is a fundamental limitation
	// We need to signal that the parent should handle this
	return fmt.Errorf("cannot extend array in place for index %d", arrayExtErr.Start)
}

func (p *Processor) handleArraySliceExtension(parent any, segment PathSegment, arrayExtErr *ArrayExtensionNeededError) error {
	// Get the array that needs extension
	arr, ok := parent.([]any)
	if !ok {
		return fmt.Errorf("expected array for slice extension, got %T", parent)
	}

	// Create extended array
	extendedArr := make([]any, arrayExtErr.RequiredLength)
	copy(extendedArr, arr)

	// Set values in the extended array
	for i := arrayExtErr.Start; i < arrayExtErr.End; i += arrayExtErr.Step {
		if i >= 0 && i < len(extendedArr) {
			extendedArr[i] = arrayExtErr.Value
		}
	}

	// For slice operations, we can't easily replace the parent array
	// This is a limitation of the current approach
	return fmt.Errorf("slice array extension not fully supported yet")
}

func (p *Processor) replaceArrayInParent(data any, parentSegments []PathSegment, newArray []any) error {
	if len(parentSegments) == 0 {
		// The array is at the root level - we can't replace it
		return fmt.Errorf("cannot replace root array")
	}

	// Navigate to the parent of the parent (grandparent)
	current := data
	for i := 0; i < len(parentSegments)-1; i++ {
		next, err := p.navigateToSegment(current, parentSegments[i], true, parentSegments, i)
		if err != nil {
			return fmt.Errorf("failed to navigate to grandparent segment %d: %w", i, err)
		}
		current = next
	}

	// Get the parent segment (the one that contains the array)
	parentSegment := parentSegments[len(parentSegments)-1]

	// Replace the array in the parent
	switch parentSegment.TypeString() {
	case "property":
		if obj, ok := current.(map[string]any); ok {
			obj[parentSegment.Key] = newArray
			return nil
		}
		if obj, ok := current.(map[any]any); ok {
			obj[parentSegment.Key] = newArray
			return nil
		}
		return fmt.Errorf("cannot set property %s on type %T", parentSegment.Key, current)
	case "array":
		if arr, ok := current.([]any); ok {
			index := parentSegment.Index
			if index < 0 {
				index = len(arr) + index
			}
			if index >= 0 && index < len(arr) {
				arr[index] = newArray
				return nil
			}
			return fmt.Errorf("array index %d out of bounds for length %d", index, len(arr))
		}
		return fmt.Errorf("cannot set array index on type %T", current)
	default:
		return fmt.Errorf("unsupported parent segment type for array replacement: %s", parentSegment.TypeString())
	}
}

func (p *Processor) setValueForArrayIndexWithExtension(current any, segment PathSegment, value any, rootData any, segments []PathSegment) error {
	switch segment.TypeString() {
	case "array":
		return p.setValueForArrayIndexWithAutoExtension(current, segment, value, rootData, segments)
	case "slice":
		return p.setValueForArraySliceWithAutoExtension(current, segment, value, rootData, segments)
	default:
		return fmt.Errorf("unsupported segment type for array extension: %s", segment.TypeString())
	}
}

func (p *Processor) setValueForArrayIndexWithAutoExtension(current any, segment PathSegment, value any, rootData any, segments []PathSegment) error {
	// Get the array index from the segment
	index := segment.Index

	switch v := current.(type) {
	case []any:
		// Handle negative indices
		if index < 0 {
			index = len(v) + index
		}

		if index < 0 {
			return fmt.Errorf("array index %d out of bounds after negative conversion", index)
		}

		if index >= len(v) {
			// Need to extend the array - find the parent and replace the array
			return p.extendArrayAndSetValue(rootData, segments, index, value)
		}

		// Set value within bounds
		v[index] = value
		return nil

	default:
		return fmt.Errorf("cannot set array index %d on type %T", index, current)
	}
}

func (p *Processor) setValueForArraySliceWithAutoExtension(current any, segment PathSegment, value any, rootData any, segments []PathSegment) error {
	arr, ok := current.([]any)
	if !ok {
		return fmt.Errorf("cannot set slice on type %T", current)
	}

	// Get slice parameters
	start, end, step := p.getSliceParameters(segment, len(arr))

	// Check if we need to extend the array
	maxIndex := end - 1
	if maxIndex >= len(arr) {
		// Need to extend the array
		return p.extendArrayAndSetSliceValue(rootData, segments, start, end, step, value)
	}

	// Set values within bounds
	for i := start; i < end; i += step {
		if i >= 0 && i < len(arr) {
			arr[i] = value
		}
	}

	return nil
}

func (p *Processor) getSliceParameters(segment PathSegment, arrayLength int) (start, end, step int) {
	// Default values
	start = 0
	end = arrayLength
	step = 1

	// Get start
	if segment.Start != nil {
		start = *segment.Start
		if start < 0 {
			start = arrayLength + start
		}
	}

	// Get end
	if segment.End != nil {
		end = *segment.End
		if end < 0 {
			end = arrayLength + end
		}
	}

	// Get step
	if segment.Step != nil {
		step = *segment.Step
	}

	// Ensure step is positive for extension purposes
	if step <= 0 {
		step = 1
	}

	// Ensure start is non-negative
	if start < 0 {
		start = 0
	}

	return start, end, step
}

func (p *Processor) extendArrayAndSetSliceValue(rootData any, segments []PathSegment, start, end, step int, value any) error {
	if len(segments) == 0 {
		return fmt.Errorf("no segments provided")
	}

	// For array extension, we need to navigate to the parent of the array container
	current := rootData
	for i := 0; i < len(segments)-2; i++ {
		next, err := p.navigateToSegment(current, segments[i], true, segments, i)
		if err != nil {
			return fmt.Errorf("failed to navigate to segment %d: %w", i, err)
		}
		current = next
	}

	// Get the array container segment and the slice access segment
	var arrayContainerSegment, sliceAccessSegment PathSegment
	if len(segments) >= 2 {
		arrayContainerSegment = segments[len(segments)-2]
		sliceAccessSegment = segments[len(segments)-1]
	} else {
		// Single segment case - the array is at root level
		sliceAccessSegment = segments[0]
	}

	// Handle different parent types
	switch v := current.(type) {
	case map[string]any:
		// Get the property name from the array container segment
		propertyName := arrayContainerSegment.Key
		if propertyName == "" && len(segments) == 1 {
			// Single segment case - extract property name from slice access segment
			propertyName = sliceAccessSegment.Key
		}

		// Get or create the array
		var currentArr []any
		if existingArr, ok := v[propertyName].([]any); ok {
			currentArr = existingArr
		} else {
			currentArr = []any{}
		}

		// Create extended array
		extendedArr := make([]any, end)
		copy(extendedArr, currentArr)

		// Set values in the slice range
		for i := start; i < end; i += step {
			if i >= 0 && i < len(extendedArr) {
				extendedArr[i] = value
			}
		}

		// Replace the array in parent
		v[propertyName] = extendedArr
		return nil

	case []any:
		// Parent is array - this would be for nested array access
		parentIndex := arrayContainerSegment.Index
		if parentIndex >= 0 && parentIndex < len(v) {
			if nestedArr, ok := v[parentIndex].([]any); ok {
				// Create extended nested array
				extendedArr := make([]any, end)
				copy(extendedArr, nestedArr)

				// Set values in the slice range
				for i := start; i < end; i += step {
					if i >= 0 && i < len(extendedArr) {
						extendedArr[i] = value
					}
				}

				// Replace the nested array
				v[parentIndex] = extendedArr
				return nil
			}
		}
		return fmt.Errorf("cannot extend nested array at index %d", parentIndex)

	default:
		return fmt.Errorf("cannot extend array in parent of type %T", current)
	}
}

func (p *Processor) extendArrayAndSetValue(rootData any, segments []PathSegment, targetIndex int, value any) error {
	if len(segments) == 0 {
		return fmt.Errorf("no segments provided")
	}

	// For array extension, we need to navigate to the parent of the array container
	// not the array itself. So we navigate to len(segments)-2 instead of len(segments)-1
	current := rootData
	for i := 0; i < len(segments)-2; i++ {
		next, err := p.navigateToSegment(current, segments[i], true, segments, i)
		if err != nil {
			return fmt.Errorf("failed to navigate to segment %d: %w", i, err)
		}
		current = next
	}

	// Get the array container segment and the array access segment
	var arrayContainerSegment, arrayAccessSegment PathSegment
	if len(segments) >= 2 {
		arrayContainerSegment = segments[len(segments)-2]
		arrayAccessSegment = segments[len(segments)-1]
	} else {
		// Single segment case - the array is at root level
		arrayAccessSegment = segments[0]
	}

	// Handle different parent types
	switch v := current.(type) {
	case map[string]any:
		// Get the property name from the array container segment
		propertyName := arrayContainerSegment.Key
		if propertyName == "" && len(segments) == 1 {
			// Single segment case - extract property name from array access segment
			propertyName = arrayAccessSegment.Key
			if propertyName == "" {
				propertyName = arrayAccessSegment.String()
				if strings.Contains(propertyName, "[") {
					bracketIndex := strings.Index(propertyName, "[")
					propertyName = propertyName[:bracketIndex]
				}
			}
		}

		// Get or create the array
		var currentArr []any
		if existingArr, ok := v[propertyName].([]any); ok {
			currentArr = existingArr
		} else {
			currentArr = []any{}
		}

		// Create extended array
		extendedArr := make([]any, targetIndex+1)
		copy(extendedArr, currentArr)
		extendedArr[targetIndex] = value

		// Replace the array in parent
		v[propertyName] = extendedArr
		return nil

	case []any:
		// Parent is array - this would be for nested array access
		// The arrayContainerSegment.Index should give us the parent array index
		parentIndex := arrayContainerSegment.Index
		if parentIndex >= 0 && parentIndex < len(v) {
			if nestedArr, ok := v[parentIndex].([]any); ok {
				// Create extended nested array
				extendedArr := make([]any, targetIndex+1)
				copy(extendedArr, nestedArr)
				extendedArr[targetIndex] = value

				// Replace the nested array
				v[parentIndex] = extendedArr
				return nil
			}
		}
		return fmt.Errorf("cannot extend nested array at index %d", parentIndex)

	default:
		return fmt.Errorf("cannot extend array in parent of type %T", current)
	}
}

func (p *Processor) setValueForArrayIndex(current any, index int, value any, createPaths bool) error {
	switch v := current.(type) {
	case []any:
		// Handle negative indices
		if index < 0 {
			index = len(v) + index
		}

		if index < 0 {
			return fmt.Errorf("array index %d out of bounds after negative conversion", index)
		}

		if index >= len(v) {
			if createPaths {
				// Return ArrayExtensionNeededError to signal parent needs to handle extension
				return &ArrayExtensionNeededError{
					RequiredLength: index + 1,
					CurrentLength:  len(v),
					Start:          index,
					End:            index + 1,
					Step:           1,
					Value:          value,
				}
			}
			return fmt.Errorf("array index %d out of bounds (length %d)", index, len(v))
		}

		v[index] = value
		return nil
	default:
		return fmt.Errorf("cannot set array index %d on type %T", index, current)
	}
}

func (p *Processor) setValueForArraySlice(current any, segment PathSegment, value any, createPaths bool) error {
	// This method is called on the array itself, so we need to handle array extension differently
	// The problem is that we can't modify the parent reference from here
	// We need to return an error that indicates array extension is needed

	arr, ok := current.([]any)
	if !ok {
		return fmt.Errorf("cannot perform slice operation on type %T", current)
	}

	// Use slice parameters from segment
	start := 0
	end := len(arr)
	step := 1

	if segment.Start != nil {
		start = *segment.Start
	}
	if segment.End != nil {
		end = *segment.End
	}
	if segment.Step != nil {
		step = *segment.Step
	}

	// Handle negative indices
	if start < 0 {
		start = len(arr) + start
	}
	if end < 0 {
		end = len(arr) + end
	}

	// Bounds checking
	if start < 0 {
		start = 0
	}

	// Check if we need to extend the array
	if end > len(arr) {
		if !createPaths {
			return fmt.Errorf("slice end %d out of bounds for array length %d", end, len(arr))
		}
		// For array extension, we need to signal that the parent needs to handle this
		return &ArrayExtensionNeededError{
			RequiredLength: end,
			CurrentLength:  len(arr),
			Start:          start,
			End:            end,
			Step:           step,
			Value:          value,
		}
	}

	if start >= end {
		return fmt.Errorf("invalid slice range [%d:%d]", start, end)
	}

	// Assign value to slice (within current bounds)
	return p.assignValueToSlice(arr, start, end, step, value)
}
