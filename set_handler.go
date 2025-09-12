package json

import (
	"fmt"
	"strconv"
	"strings"
)

// ArrayExtensionNeededError indicates that an array needs to be extended
type ArrayExtensionNeededError struct {
	RequiredLength int
	CurrentLength  int
	Start          int
	End            int
	Step           int
	Value          any
}

func (e *ArrayExtensionNeededError) Error() string {
	return fmt.Sprintf("array extension needed: current length %d, required length %d for slice [%d:%d]",
		e.CurrentLength, e.RequiredLength, e.Start, e.End)
}

// setValueAtPath sets a value at the specified path
func (p *Processor) setValueAtPath(data any, path string, value any) error {
	return p.setValueAtPathWithOptions(data, path, value, false)
}

// setValueAtPathWithOptions sets a value at the specified path with creation options
func (p *Processor) setValueAtPathWithOptions(data any, path string, value any, createPaths bool) error {
	if path == "" || path == "." {
		return fmt.Errorf("cannot set root value")
	}

	// Use advanced path parsing for full feature support
	return p.setValueAdvancedPath(data, path, value, createPaths)
}

// setValueAdvancedPath sets a value using hybrid processing
func (p *Processor) setValueAdvancedPath(data any, path string, value any, createPaths bool) error {
	// Handle JSON Pointer format first
	if strings.HasPrefix(path, "/") {
		if createPaths {
			return p.setValueJSONPointerWithCreation(data, path, value)
		}
		return p.setValueJSONPointer(data, path, value)
	}

	// Check if this is a simple array index access that might need extension
	if createPaths && p.isSimpleArrayIndexPath(path) {
		// Use dot notation handler for simple array index access with extension support
		return p.setValueDotNotationWithCreation(data, path, value, createPaths)
	}

	// Check if this is a complex path that should use RecursiveProcessor
	// But exclude simple array slice paths that need array extension support
	if p.isComplexPath(path) && !p.needsLegacyComplexHandling(path) && !p.isSimpleArraySlicePath(path) {
		// Use RecursiveProcessor for complex paths like flat extraction
		unifiedProcessor := NewRecursiveProcessor(p)
		_, err := unifiedProcessor.ProcessRecursivelyWithOptions(data, path, OpSet, value, createPaths)
		return err
	}

	// Check for distributed operations
	// TODO: Implement distributed operation handling
	// if p.isDistributedOperationPath(path) {
	//     return p.setValueWithDistributedOperation(data, path, value, createPaths)
	// }

	// Use dot notation with segments for simple paths
	return p.setValueDotNotationWithCreation(data, path, value, createPaths)
}

// isSimpleArraySlicePath checks if a path is a simple array slice that needs array extension
func (p *Processor) isSimpleArraySlicePath(path string) bool {
	// Check for simple patterns like "property[start:end]" or "property.subprop[start:end]"
	// These should use legacy handling for array extension support

	// Must contain slice syntax
	if !strings.Contains(path, ":") {
		return false
	}

	// Must not contain extraction syntax (which needs RecursiveProcessor)
	if strings.Contains(path, "{") || strings.Contains(path, "}") {
		return false
	}

	// Check if it's a simple property.array[slice] pattern
	// Count the number of bracket pairs
	openBrackets := strings.Count(path, "[")
	closeBrackets := strings.Count(path, "]")

	// Should have exactly one bracket pair for simple slice
	if openBrackets != 1 || closeBrackets != 1 {
		return false
	}

	// Find the bracket positions
	bracketStart := strings.Index(path, "[")
	bracketEnd := strings.Index(path, "]")

	if bracketStart == -1 || bracketEnd == -1 || bracketEnd <= bracketStart {
		return false
	}

	// Extract the slice part
	slicePart := path[bracketStart+1 : bracketEnd]

	// Check if it's a valid slice syntax (contains colon)
	if !strings.Contains(slicePart, ":") {
		return false
	}

	// Check if the part before brackets is a simple property path (no complex operations)
	beforeBrackets := path[:bracketStart]
	if strings.Contains(beforeBrackets, "{") || strings.Contains(beforeBrackets, "}") {
		return false
	}

	return true
}

// isSimpleArrayIndexPath checks if a path is a simple array index access that might need extension
func (p *Processor) isSimpleArrayIndexPath(path string) bool {
	// Must contain array index syntax
	if !strings.Contains(path, "[") || !strings.Contains(path, "]") {
		return false
	}

	// Must not contain slice syntax (colons)
	if strings.Contains(path, ":") {
		return false
	}

	// Must not contain extraction syntax
	if strings.Contains(path, "{") || strings.Contains(path, "}") {
		return false
	}

	// Check if it's a simple pattern like "property[index]" or "property.subprop[index]"
	// Count the number of bracket pairs
	openBrackets := strings.Count(path, "[")
	closeBrackets := strings.Count(path, "]")

	// Should have exactly one bracket pair for simple index access
	if openBrackets != 1 || closeBrackets != 1 {
		return false
	}

	// Find the bracket positions
	bracketStart := strings.Index(path, "[")
	bracketEnd := strings.Index(path, "]")

	if bracketStart == -1 || bracketEnd == -1 || bracketEnd <= bracketStart {
		return false
	}

	// Extract the index part
	indexPart := path[bracketStart+1 : bracketEnd]

	// Check if it's a valid numeric index (including negative indices)
	if indexPart == "" {
		return false
	}

	// Try to parse as integer
	if _, err := strconv.Atoi(indexPart); err != nil {
		return false
	}

	return true
}

// handleArrayExtensionAndSet handles array extension and sets values
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

// handleArrayIndexExtension handles array extension for simple index access
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

// handleArraySliceExtension handles array extension for slice access
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

// replaceArrayInParent replaces an array in its parent container
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
			obj[parentSegment.Value] = newArray
			return nil
		}
		if obj, ok := current.(map[any]any); ok {
			obj[parentSegment.Value] = newArray
			return nil
		}
		return fmt.Errorf("cannot set property %s on type %T", parentSegment.Value, current)
	case "array":
		if arr, ok := current.([]any); ok {
			index := p.parseArrayIndexFromSegment(parentSegment.Value)
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

// setValueWithSegments sets a value using parsed segments
func (p *Processor) setValueWithSegments(data any, segments []PathSegment, value any, createPaths bool) error {
	if len(segments) == 0 {
		return fmt.Errorf("no segments provided")
	}

	// TODO: Implement deep extraction and consecutive extraction handling
	// if p.hasDeepExtractionPattern(segments) {
	//     extractionGroups := p.groupConsecutiveExtractions(segments)
	//     return p.setValueWithDeepExtraction(data, segments, extractionGroups, value)
	// }

	// if p.hasConsecutiveExtractions(segments, 0) {
	//     return p.setValueWithConsecutiveExtractions(data, segments, value)
	// }

	// Navigate to the parent of the target
	current := data
	for i := 0; i < len(segments)-1; i++ {
		next, err := p.navigateToSegment(current, segments[i], createPaths, segments, i)
		if err != nil {
			return err
		}
		current = next
	}

	// Set the value for the final segment
	finalSegment := segments[len(segments)-1]

	// Special handling for array index or slice access that might need extension
	if createPaths && (finalSegment.TypeString() == "array" || finalSegment.TypeString() == "slice") {
		return p.setValueForArrayIndexWithExtension(current, finalSegment, value, data, segments)
	}

	err := p.setValueForSegment(current, finalSegment, value, createPaths)

	// Handle array extension error
	if arrayExtErr, ok := err.(*ArrayExtensionNeededError); ok && createPaths {
		// We need to extend the array and then set the values
		return p.handleArrayExtensionAndSet(data, segments, arrayExtErr)
	}

	return err
}

// setValueForArrayIndexWithExtension sets a value for array index or slice with extension support
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

// setValueForArrayIndexWithAutoExtension handles array index access with extension
func (p *Processor) setValueForArrayIndexWithAutoExtension(current any, segment PathSegment, value any, rootData any, segments []PathSegment) error {
	// Parse the array index from the segment
	index := p.parseArrayIndexFromSegment(segment.Value)
	if index == -999999 {
		return fmt.Errorf("invalid array index: %s", segment.Value)
	}

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

// setValueForArraySliceWithAutoExtension handles array slice access with extension
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

// getSliceParameters extracts slice parameters from segment
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

// extendArrayAndSetSliceValue extends an array for slice operations
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
		propertyName := arrayContainerSegment.Value
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

// extendArrayAndSetValue extends an array at the specified path and sets the value
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
		propertyName := arrayContainerSegment.Value
		if propertyName == "" && len(segments) == 1 {
			// Single segment case - extract property name from array access segment
			propertyName = arrayAccessSegment.Value
			if strings.Contains(propertyName, "[") {
				bracketIndex := strings.Index(propertyName, "[")
				propertyName = propertyName[:bracketIndex]
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

// navigateToSegment navigates to a specific segment, creating if necessary
func (p *Processor) navigateToSegment(current any, segment PathSegment, createPaths bool, allSegments []PathSegment, currentIndex int) (any, error) {
	switch segment.TypeString() {
	case "property":
		return p.navigateToProperty(current, segment.Value, createPaths, allSegments, currentIndex)
	case "array":
		// Parse array index from segment value
		index := p.parseArrayIndexFromSegment(segment.Value)
		if index == -999999 { // -999999 means invalid index
			return nil, fmt.Errorf("invalid array index in segment '%s'", segment.Value)
		}
		return p.navigateToArrayIndexWithNegative(current, index, createPaths)
	case "slice":
		// Check if this is the last segment before an extract operation
		if currentIndex+1 < len(allSegments) && allSegments[currentIndex+1].TypeString() == "extract" {
			// This is a slice followed by extract - return the current array for slice processing
			return current, nil
		}
		// For other cases, array slices are not supported as intermediate paths
		return nil, fmt.Errorf("array slice not supported as intermediate path segment")
	case "extract":
		// Handle extract operations as intermediate path segments
		return p.navigateToExtraction(current, segment, createPaths, allSegments, currentIndex)
	default:
		return nil, fmt.Errorf("unsupported segment type: %v", segment.TypeString())
	}
}

// navigateToExtraction navigates to an extraction result for Set operations
func (p *Processor) navigateToExtraction(current any, segment PathSegment, createPaths bool, allSegments []PathSegment, currentIndex int) (any, error) {
	field := segment.Extract
	if field == "" {
		return nil, fmt.Errorf("invalid extraction syntax: %s", segment.Value)
	}

	// For set operations on extractions, we need to handle this differently
	// This is a complex case that might require distributed operations
	if _, ok := current.([]any); ok {
		// For arrays, we need to set values in each extracted field
		// This is handled by distributed operations
		return current, nil
	}

	// For single objects, extract the field
	if obj, ok := current.(map[string]any); ok {
		if value := p.handlePropertyAccessValue(obj, field); value != nil {
			return value, nil
		}
		if createPaths {
			// Create the field if it doesn't exist
			newContainer, err := p.createContainerForNextSegment(allSegments, currentIndex)
			if err != nil {
				return nil, err
			}
			obj[field] = newContainer
			return newContainer, nil
		}
	}

	return nil, fmt.Errorf("extraction field '%s' not found", field)
}

// navigateToProperty navigates to a property, creating if necessary
func (p *Processor) navigateToProperty(current any, property string, createPaths bool, allSegments []PathSegment, currentIndex int) (any, error) {
	switch v := current.(type) {
	case map[string]any:
		if val, exists := v[property]; exists {
			return val, nil
		}
		if createPaths {
			// Create missing property
			newContainer, err := p.createContainerForNextSegment(allSegments, currentIndex)
			if err != nil {
				return nil, err
			}
			v[property] = newContainer
			return newContainer, nil
		}
		return nil, fmt.Errorf("property '%s' not found", property)
	case map[any]any:
		if val, exists := v[property]; exists {
			return val, nil
		}
		if createPaths {
			newContainer, err := p.createContainerForNextSegment(allSegments, currentIndex)
			if err != nil {
				return nil, err
			}
			v[property] = newContainer
			return newContainer, nil
		}
		return nil, fmt.Errorf("property '%s' not found", property)
	default:
		return nil, fmt.Errorf("cannot access property '%s' on type %T", property, current)
	}
}

// createContainerForNextSegment creates appropriate container for the next segment
func (p *Processor) createContainerForNextSegment(allSegments []PathSegment, currentIndex int) (any, error) {
	if currentIndex+1 >= len(allSegments) {
		// This is the last segment, return nil (will be replaced by the actual value)
		return nil, nil
	}

	nextSegment := allSegments[currentIndex+1]
	switch nextSegment.TypeString() {
	case "property", "extract":
		return make(map[string]any), nil
	case "array":
		// For array access, create an empty array that can be extended
		return make([]any, 0), nil
	case "slice":
		// For slice access, we need to create an array large enough for the slice
		end := 0
		if nextSegment.End != nil {
			end = *nextSegment.End
		}
		if end > 0 {
			return make([]any, end), nil
		}
		return make([]any, 0), nil
	default:
		return make(map[string]any), nil // Default to object
	}
}

// setValueForSegment sets a value for a specific segment
func (p *Processor) setValueForSegment(current any, segment PathSegment, value any, createPaths bool) error {
	switch segment.TypeString() {
	case "property":
		return p.setValueForProperty(current, segment.Value, value, createPaths)
	case "array":
		index := p.parseArrayIndexFromSegment(segment.Value)
		if index == -999999 {
			return fmt.Errorf("invalid array index: %s", segment.Value)
		}
		return p.setValueForArrayIndex(current, index, value, createPaths)
	case "slice":
		return p.setValueForArraySlice(current, segment, value, createPaths)
	case "extract":
		return p.setValueForExtract(current, segment, value, createPaths)
	default:
		return fmt.Errorf("unsupported segment type for set: %v", segment.TypeString())
	}
}

// setValueForProperty sets a value for a property
func (p *Processor) setValueForProperty(current any, property string, value any, createPaths bool) error {
	switch v := current.(type) {
	case map[string]any:
		v[property] = value
		return nil
	case map[any]any:
		v[property] = value
		return nil
	default:
		if createPaths {
			// Try to convert to map
			if newMap := make(map[string]any); newMap != nil {
				newMap[property] = value
				// Note: This doesn't actually replace the original,
				// which is a limitation of this approach
				return fmt.Errorf("cannot convert %T to map for property setting", current)
			}
		}
		return fmt.Errorf("cannot set property '%s' on type %T", property, current)
	}
}

// setValueForArrayIndex sets a value at an array index
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

// setValueForArraySlice sets a value for an array slice
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

// setValueForExtract sets a value for extraction operations
func (p *Processor) setValueForExtract(current any, segment PathSegment, value any, createPaths bool) error {
	field := segment.Extract
	if field == "" {
		return fmt.Errorf("invalid extraction syntax: %s", segment.Value)
	}

	// Handle array extraction
	if arr, ok := current.([]any); ok {
		if segment.IsFlat {
			return p.setValueForArrayExtractFlat(arr, field, value)
		} else {
			return p.setValueForArrayExtract(arr, field, value)
		}
	}

	// Handle single object
	if obj, ok := current.(map[string]any); ok {
		obj[field] = value
		return nil
	}

	return fmt.Errorf("cannot perform extraction set on type %T", current)
}

// setValueForArrayExtract sets values for array extraction (regular)
func (p *Processor) setValueForArrayExtract(arr []any, extractKey string, value any) error {
	for i, item := range arr {
		if obj, ok := item.(map[string]any); ok {
			obj[extractKey] = value
		} else {
			// Create new object if item is not a map
			newObj := map[string]any{extractKey: value}
			arr[i] = newObj
		}
	}
	return nil
}

// setValueForArrayExtractFlat sets values for flat array extraction
func (p *Processor) setValueForArrayExtractFlat(arr []any, extractKey string, value any) error {
	// For flat extraction, we need to handle nested arrays
	for i, item := range arr {
		if obj, ok := item.(map[string]any); ok {
			// Check if the field contains an array that should be flattened
			if existingValue, exists := obj[extractKey]; exists {
				if existingArr, ok := existingValue.([]any); ok {
					// Flatten the value into the existing array
					if valueArr, ok := value.([]any); ok {
						// Merge arrays
						existingArr = append(existingArr, valueArr...)
						obj[extractKey] = existingArr
					} else {
						// Add single value to array
						existingArr = append(existingArr, value)
						obj[extractKey] = existingArr
					}
				} else {
					// Convert existing value to array and add new value
					newArr := []any{existingValue}
					if valueArr, ok := value.([]any); ok {
						newArr = append(newArr, valueArr...)
					} else {
						newArr = append(newArr, value)
					}
					obj[extractKey] = newArr
				}
			} else {
				// Create new field
				if valueArr, ok := value.([]any); ok {
					obj[extractKey] = valueArr
				} else {
					obj[extractKey] = []any{value}
				}
			}
		} else {
			// Create new object with array field
			var newValue any
			if valueArr, ok := value.([]any); ok {
				newValue = valueArr
			} else {
				newValue = []any{value}
			}
			newObj := map[string]any{extractKey: newValue}
			arr[i] = newObj
		}
	}
	return nil
}

// setValueDotNotation sets a value using dot notation
func (p *Processor) setValueDotNotation(data any, path string, value any) error {
	return p.setValueDotNotationWithCreation(data, path, value, false)
}

// setValueDotNotationWithCreation sets a value using dot notation with path creation
func (p *Processor) setValueDotNotationWithCreation(data any, path string, value any, createPaths bool) error {
	// Parse path into segments
	segments := p.getPathSegments()
	defer p.putPathSegments(segments)

	segments = p.splitPath(path, segments)

	return p.setValueWithSegments(data, segments, value, createPaths)
}

// setValueJSONPointer sets a value using JSON Pointer format
func (p *Processor) setValueJSONPointer(data any, path string, value any) error {
	return p.setValueJSONPointerWithCreation(data, path, value)
}

// setValueJSONPointerWithCreation sets a value using JSON Pointer with path creation
func (p *Processor) setValueJSONPointerWithCreation(data any, path string, value any) error {
	if path == "/" {
		return fmt.Errorf("cannot set root value")
	}

	// Remove leading slash and split
	pathWithoutSlash := path[1:]
	segments := strings.Split(pathWithoutSlash, "/")

	// Handle array extension for JSON Pointer
	return p.setValueJSONPointerWithArrayExtension(data, segments, value)
}

// setValueJSONPointerWithArrayExtension sets a value using JSON Pointer with array extension support
func (p *Processor) setValueJSONPointerWithArrayExtension(data any, segments []string, value any) error {
	if len(segments) == 0 {
		return fmt.Errorf("no segments provided")
	}

	// Navigate to parent segments
	current := data
	for i := 0; i < len(segments)-1; i++ {
		segment := segments[i]

		// Unescape JSON Pointer characters
		if strings.Contains(segment, "~") {
			segment = p.unescapeJSONPointer(segment)
		}

		next, err := p.createPathSegmentForJSONPointerWithExtension(current, segment, segments, i)
		if err != nil {
			return err
		}
		current = next
	}

	// Set final value
	finalSegment := segments[len(segments)-1]
	if strings.Contains(finalSegment, "~") {
		finalSegment = p.unescapeJSONPointer(finalSegment)
	}

	return p.setJSONPointerFinalValue(current, finalSegment, value)
}

// createPathSegmentForJSONPointer creates path segments for JSON Pointer format
func (p *Processor) createPathSegmentForJSONPointer(current any, segment string, allSegments []string, currentIndex int) (any, error) {
	switch v := current.(type) {
	case map[string]any:
		if val, exists := v[segment]; exists {
			return val, nil
		}
		// Create missing property
		var newContainer any
		if currentIndex+1 < len(allSegments) {
			nextSegment := allSegments[currentIndex+1]
			if p.isArrayIndex(nextSegment) {
				newContainer = make([]any, 0)
			} else {
				newContainer = make(map[string]any)
			}
		} else {
			newContainer = make(map[string]any)
		}
		v[segment] = newContainer
		return newContainer, nil

	case []any:
		if index, err := strconv.Atoi(segment); err == nil {
			if index >= 0 && index < len(v) {
				return v[index], nil
			}
			if index >= len(v) {
				// Need to extend array - create extended array and replace in parent
				return p.handleJSONPointerArrayExtension(current, v, index, allSegments, currentIndex)
			}
		}
		return nil, fmt.Errorf("invalid array index for JSON Pointer: %s", segment)

	default:
		return nil, fmt.Errorf("cannot navigate through %T with segment %s", current, segment)
	}
}

// setPropertyValue sets a property value without path creation
func (p *Processor) setPropertyValue(current any, property string, value any) error {
	switch v := current.(type) {
	case map[string]any:
		v[property] = value
		return nil
	case map[any]any:
		v[property] = value
		return nil
	default:
		return fmt.Errorf("cannot set property '%s' on type %T", property, current)
	}
}

// setPropertyValueWithCreation sets a property value with path creation
func (p *Processor) setPropertyValueWithCreation(current any, property string, value any) error {
	switch v := current.(type) {
	case map[string]any:
		v[property] = value
		return nil
	case map[any]any:
		v[property] = value
		return nil
	case []any:
		// Try to parse property as array index
		if index, err := strconv.Atoi(property); err == nil {
			if index >= 0 && index < len(v) {
				v[index] = value
				return nil
			}
			if index == len(v) {
				// Extend array
				v = append(v, value)
				return nil
			}
		}
		return fmt.Errorf("invalid array index: %s", property)
	default:
		return fmt.Errorf("cannot set property '%s' on type %T", property, current)
	}
}

// handleJSONPointerArrayExtension handles array extension for JSON Pointer format
func (p *Processor) handleJSONPointerArrayExtension(parent any, arr []any, targetIndex int, allSegments []string, currentIndex int) (any, error) {
	// Create extended array
	extendedArr := make([]any, targetIndex+1)
	copy(extendedArr, arr)

	// Determine what to put at the target index
	var newContainer any
	if currentIndex+1 < len(allSegments) {
		nextSegment := allSegments[currentIndex+1]
		if p.isArrayIndex(nextSegment) {
			newContainer = make([]any, 0)
		} else {
			newContainer = make(map[string]any)
		}
	} else {
		newContainer = nil // Will be set by the final value
	}

	extendedArr[targetIndex] = newContainer

	// Replace the array in parent - this is tricky for JSON Pointer
	// We need to find a way to update the parent reference
	// For now, we'll modify the original array in place if possible
	if cap(arr) >= len(extendedArr) {
		// Can extend in place
		for i := len(arr); i < len(extendedArr); i++ {
			arr = append(arr, nil)
		}
		arr[targetIndex] = newContainer
		return newContainer, nil
	}

	// Cannot extend in place - this is a limitation
	// Return the new container but note that parent won't be updated
	return newContainer, nil
}

// createPathSegmentForJSONPointerWithExtension creates path segments for JSON Pointer with array extension
func (p *Processor) createPathSegmentForJSONPointerWithExtension(current any, segment string, allSegments []string, currentIndex int) (any, error) {
	switch v := current.(type) {
	case map[string]any:
		if val, exists := v[segment]; exists {
			return val, nil
		}
		// Create missing property
		var newContainer any
		if currentIndex+1 < len(allSegments) {
			nextSegment := allSegments[currentIndex+1]
			if p.isArrayIndex(nextSegment) {
				newContainer = make([]any, 0)
			} else {
				newContainer = make(map[string]any)
			}
		} else {
			newContainer = make(map[string]any)
		}
		v[segment] = newContainer
		return newContainer, nil

	case []any:
		if index, err := strconv.Atoi(segment); err == nil {
			if index >= 0 && index < len(v) {
				return v[index], nil
			}
			if index >= len(v) {
				// Extend array to accommodate the index
				extendedArr := make([]any, index+1)
				copy(extendedArr, v)

				// Determine what to put at the target index
				var newContainer any
				if currentIndex+1 < len(allSegments) {
					nextSegment := allSegments[currentIndex+1]
					if p.isArrayIndex(nextSegment) {
						newContainer = make([]any, 0)
					} else {
						newContainer = make(map[string]any)
					}
				} else {
					newContainer = nil
				}

				extendedArr[index] = newContainer

				// Replace the array in the parent - we need to find the parent
				// This is a complex operation that requires tracking the parent
				return p.replaceArrayInJSONPointerParent(current, v, extendedArr, index, newContainer)
			}
		}
		return nil, fmt.Errorf("invalid array index for JSON Pointer: %s", segment)

	default:
		return nil, fmt.Errorf("cannot navigate through %T with segment %s", current, segment)
	}
}

// setJSONPointerFinalValue sets the final value for JSON Pointer with array extension support
func (p *Processor) setJSONPointerFinalValue(current any, segment string, value any) error {
	switch v := current.(type) {
	case map[string]any:
		v[segment] = value
		return nil
	case []any:
		if index, err := strconv.Atoi(segment); err == nil {
			if index >= 0 && index < len(v) {
				v[index] = value
				return nil
			}
			if index >= len(v) {
				// Extend array to accommodate the index
				extendedArr := make([]any, index+1)
				copy(extendedArr, v)
				extendedArr[index] = value

				// Try to replace in place if possible
				if cap(v) >= len(extendedArr) {
					for i := len(v); i < len(extendedArr); i++ {
						v = append(v, nil)
					}
					v[index] = value
					return nil
				}

				// Cannot extend in place - this is a limitation of the current approach
				// The parent reference won't be updated
				return fmt.Errorf("cannot extend array in place for index %d", index)
			}
		}
		return fmt.Errorf("invalid array index: %s", segment)
	default:
		return fmt.Errorf("cannot set value on type %T", current)
	}
}

// replaceArrayInJSONPointerParent replaces an array in its JSON Pointer parent context
func (p *Processor) replaceArrayInJSONPointerParent(parent any, oldArray, newArray []any, index int, newContainer any) (any, error) {
	// This is a complex operation that would require tracking parent references
	// For now, we'll try to extend in place if possible
	if cap(oldArray) >= len(newArray) {
		for i := len(oldArray); i < len(newArray); i++ {
			oldArray = append(oldArray, nil)
		}
		oldArray[index] = newContainer
		return newContainer, nil
	}

	// Cannot extend in place
	return newContainer, nil
}
