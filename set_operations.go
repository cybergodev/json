package json

import (
	"fmt"
	"strconv"
	"strings"
)

// setOperations implements the SetOperations interface
type setOperations struct {
	utils      ProcessorUtils
	pathParser PathParser
	navigator  Navigator
	arrayOps   ArrayOperations
}

// NewSetOperations creates a new set operations instance
func NewSetOperations(utils ProcessorUtils, pathParser PathParser, navigator Navigator, arrayOps ArrayOperations) SetOperations {
	return &setOperations{
		utils:      utils,
		pathParser: pathParser,
		navigator:  navigator,
		arrayOps:   arrayOps,
	}
}

// SetValue sets a value at the specified path
func (so *setOperations) SetValue(data any, path string, value any, createPaths bool) error {
	if path == "" || path == "." {
		return fmt.Errorf("cannot set root value")
	}

	segments, err := so.pathParser.ParsePath(path)
	if err != nil {
		return fmt.Errorf("failed to parse path '%s': %w", path, err)
	}

	return so.SetValueWithSegments(data, segments, value, createPaths)
}

// SetValueWithSegments sets a value using parsed path segments
func (so *setOperations) SetValueWithSegments(data any, segments []PathSegmentInfo, value any, createPaths bool) error {
	if len(segments) == 0 {
		return fmt.Errorf("no segments to process")
	}

	// Handle single segment (direct property/array access)
	if len(segments) == 1 {
		return so.setValueForSegment(data, segments[0], value, createPaths)
	}

	// Navigate to parent segments
	current := data
	for i, segment := range segments[:len(segments)-1] {
		next, err := so.navigateToSegment(current, segment, createPaths, segments, i)
		if err != nil {
			return fmt.Errorf("failed to navigate to segment '%s': %w", segment.Key, err)
		}
		current = next
	}

	// Set the final value
	finalSegment := segments[len(segments)-1]
	err := so.setValueForSegment(current, finalSegment, value, createPaths)

	// Handle array extension error
	if arrayExtErr, ok := err.(*ArrayExtensionError); ok && createPaths {
		return so.handleArrayExtension(data, segments, arrayExtErr)
	}

	return err
}

// CreatePath creates missing path segments
func (so *setOperations) CreatePath(data any, segments []PathSegmentInfo) error {
	current := data
	for i, segment := range segments {
		next, err := so.navigateToSegment(current, segment, true, segments, i)
		if err != nil {
			return fmt.Errorf("failed to create path segment '%s': %w", segment.Key, err)
		}
		current = next
	}
	return nil
}

// HandleTypeConversion handles type conversion for setting operations
func (so *setOperations) HandleTypeConversion(data any, requiredType string) (any, error) {
	switch requiredType {
	case "array":
		return so.convertToArray(data)
	case "object":
		return so.convertToObject(data)
	case "string":
		return so.utils.ConvertToString(data), nil
	case "number":
		return so.utils.ConvertToNumber(data)
	default:
		return data, nil
	}
}

// setValueForSegment sets a value for a specific segment type
func (so *setOperations) setValueForSegment(current any, segment PathSegmentInfo, value any, createPaths bool) error {
	switch segment.Type {
	case "property":
		return so.setValueForProperty(current, segment.Key, value, createPaths)
	case "array":
		return so.setValueForArrayIndex(current, segment.Index, value, createPaths)
	case "slice":
		return so.setValueForArraySlice(current, segment, value, createPaths)
	case "extract":
		return so.setValueForExtract(current, segment, value, createPaths)
	default:
		return fmt.Errorf("unsupported segment type for set operation: %s", segment.Type)
	}
}

// setValueForProperty sets a value for a property
func (so *setOperations) setValueForProperty(current any, property string, value any, createPaths bool) error {
	switch v := current.(type) {
	case map[string]any:
		v[property] = value
		return nil
	case map[any]any:
		v[property] = value
		return nil
	case []any:
		// Check if property is a numeric index for dot notation array access
		if index := so.parseArrayIndex(property); index != -999999 {
			// Handle negative indices
			if index < 0 {
				index = len(v) + index
			}
			// Check bounds after negative index conversion
			if index < 0 {
				return fmt.Errorf("negative array index %d out of bounds (length %d)", index, len(v))
			}
			if index >= len(v) {
				if createPaths {
					// Extend array to accommodate the index
					return so.extendArrayForIndex(v, index, value)
				}
				return fmt.Errorf("array index %d out of bounds (length %d)", index, len(v))
			}
			v[index] = value
			return nil
		}
		// Not a numeric index, cannot set property on array
		if createPaths {
			return fmt.Errorf("cannot set non-numeric property '%s' on array", property)
		}
		return fmt.Errorf("cannot set property '%s' on array", property)
	default:
		if createPaths {
			// Convert to object and set property
			newObj := make(map[string]any)
			newObj[property] = value
			return so.replaceCurrentData(current, newObj)
		}
		return fmt.Errorf("cannot set property '%s' on type %T", property, current)
	}
}

// setValueForArrayIndex sets a value at a specific array index
func (so *setOperations) setValueForArrayIndex(current any, index int, value any, createPaths bool) error {
	arr, ok := current.([]any)
	if !ok {
		if createPaths {
			// Convert to array
			newArr := make([]any, index+1)
			newArr[index] = value
			return so.replaceCurrentData(current, newArr)
		}
		return fmt.Errorf("cannot set array index on type %T", current)
	}

	// Handle negative indices
	if index < 0 {
		index = len(arr) + index
	}

	// Check bounds
	if index < 0 {
		return fmt.Errorf("negative array index %d out of bounds (length %d)", index, len(arr))
	}

	if index >= len(arr) {
		if createPaths {
			// Return ArrayExtensionError to signal parent needs to handle extension
			return &ArrayExtensionError{
				CurrentLength:  len(arr),
				RequiredLength: index + 1,
				TargetIndex:    index,
				Value:          value,
			}
		}
		return fmt.Errorf("array index %d out of bounds (length %d)", index, len(arr))
	}

	arr[index] = value
	return nil
}

// setValueForArraySlice sets values for an array slice
func (so *setOperations) setValueForArraySlice(current any, segment PathSegmentInfo, value any, createPaths bool) error {
	arr, ok := current.([]any)
	if !ok {
		if createPaths {
			// Convert to array
			newArr := []any{value}
			return so.replaceCurrentData(current, newArr)
		}
		return fmt.Errorf("cannot set array slice on type %T", current)
	}

	start, end, step := so.parseSliceParameters(segment)
	length := len(arr)

	// Handle negative indices
	if start < 0 {
		start = length + start
	}
	if end < 0 {
		end = length + end
	}

	// Clamp to bounds
	if start < 0 {
		start = 0
	}
	if end > length {
		if createPaths {
			// Extend array to accommodate the slice
			newLength := end
			newArr := make([]any, newLength)
			copy(newArr, arr)
			arr = newArr
		} else {
			end = length
		}
	}

	// Set values in the slice
	valueArr, ok := value.([]any)
	if !ok {
		// Single value, set to all positions in slice
		for i := start; i < end; i += step {
			if i < len(arr) {
				arr[i] = value
			}
		}
	} else {
		// Array of values, set each position
		valueIndex := 0
		for i := start; i < end && valueIndex < len(valueArr); i += step {
			if i < len(arr) {
				arr[i] = valueArr[valueIndex]
				valueIndex++
			}
		}
	}

	return nil
}

// setValueForExtract sets values for extract operations
func (so *setOperations) setValueForExtract(current any, segment PathSegmentInfo, value any, createPaths bool) error {
	switch v := current.(type) {
	case []any:
		// Extract operations on arrays - set value to all matching elements
		return so.setValueForArrayExtract(v, segment.Extract, value)
	case map[string]any:
		// Extract operations on objects - set value to specific key
		if segment.Extract != "" {
			v[segment.Extract] = value
			return nil
		}
		return fmt.Errorf("extract key is required for object extract operations")
	case map[any]any:
		if segment.Extract != "" {
			v[segment.Extract] = value
			return nil
		}
		return fmt.Errorf("extract key is required for generic map extract operations")
	default:
		if createPaths {
			// Convert to object and set extract key
			newObj := make(map[string]any)
			newObj[segment.Extract] = value
			return so.replaceCurrentData(current, newObj)
		}
		return fmt.Errorf("cannot perform extract operation on type %T", current)
	}
}

// setValueForArrayExtract sets values for array extraction
func (so *setOperations) setValueForArrayExtract(arr []any, extractKey string, value any) error {
	for _, item := range arr {
		if obj, ok := item.(map[string]any); ok {
			obj[extractKey] = value
		} else if genericObj, ok := item.(map[any]any); ok {
			genericObj[extractKey] = value
		}
		// Skip non-object items
	}
	return nil
}

// navigateToSegment navigates to a specific segment, creating path if needed
func (so *setOperations) navigateToSegment(current any, segment PathSegmentInfo, createPaths bool, segments []PathSegmentInfo, segmentIndex int) (any, error) {
	switch segment.Type {
	case "property":
		return so.navigateToProperty(current, segment.Key, createPaths, segments, segmentIndex)
	case "array":
		return so.navigateToArrayIndex(current, segment.Index, createPaths)
	case "slice":
		return so.navigateToArraySlice(current, segment, createPaths)
	case "extract":
		return so.navigateToExtract(current, segment.Extract, createPaths)
	default:
		return nil, fmt.Errorf("unsupported segment type for navigation: %s", segment.Type)
	}
}

// navigateToProperty navigates to a property, creating if needed
func (so *setOperations) navigateToProperty(current any, property string, createPaths bool, segments []PathSegmentInfo, segmentIndex int) (any, error) {
	switch v := current.(type) {
	case map[string]any:
		if value, exists := v[property]; exists {
			return value, nil
		}
		if createPaths {
			// Determine what type to create based on next segment
			nextContainer := so.determineContainerType(segments, segmentIndex+1)
			v[property] = nextContainer
			return nextContainer, nil
		}
		return nil, fmt.Errorf("property '%s' not found", property)
	case map[any]any:
		if value, exists := v[property]; exists {
			return value, nil
		}
		if createPaths {
			nextContainer := so.determineContainerType(segments, segmentIndex+1)
			v[property] = nextContainer
			return nextContainer, nil
		}
		return nil, fmt.Errorf("property '%s' not found", property)
	case []any:
		// Check if property is a numeric index
		if index := so.parseArrayIndex(property); index != -999999 {
			if index < 0 {
				index = len(v) + index
			}
			if index >= 0 && index < len(v) {
				return v[index], nil
			}
		}
		return nil, fmt.Errorf("cannot access property '%s' on array", property)
	default:
		if createPaths {
			// Convert to object
			newObj := make(map[string]any)
			nextContainer := so.determineContainerType(segments, segmentIndex+1)
			newObj[property] = nextContainer
			return nextContainer, nil
		}
		return nil, fmt.Errorf("cannot access property '%s' on type %T", property, current)
	}
}

// navigateToArrayIndex navigates to an array index, creating if needed
func (so *setOperations) navigateToArrayIndex(current any, index int, createPaths bool) (any, error) {
	arr, ok := current.([]any)
	if !ok {
		if createPaths {
			// Convert to array
			newArr := make([]any, index+1)
			return newArr[index], nil
		}
		return nil, fmt.Errorf("cannot access array index on type %T", current)
	}

	// Handle negative indices
	if index < 0 {
		index = len(arr) + index
	}

	if index < 0 || index >= len(arr) {
		if createPaths {
			// Extend array
			if index >= len(arr) {
				newArr := make([]any, index+1)
				copy(newArr, arr)
				arr = newArr
			}
			return arr[index], nil
		}
		return nil, fmt.Errorf("array index %d out of bounds", index)
	}

	return arr[index], nil
}

// navigateToArraySlice navigates to an array slice
func (so *setOperations) navigateToArraySlice(current any, segment PathSegmentInfo, createPaths bool) (any, error) {
	arr, ok := current.([]any)
	if !ok {
		if createPaths {
			return []any{}, nil
		}
		return nil, fmt.Errorf("cannot access array slice on type %T", current)
	}

	start, end, step := so.parseSliceParameters(segment)
	result := so.arrayOps.HandleArraySlice(arr, start, end, step)
	if !result.Exists {
		return nil, fmt.Errorf("array slice failed")
	}

	return result.Value, nil
}

// navigateToExtract navigates to an extract operation
func (so *setOperations) navigateToExtract(current any, extractKey string, createPaths bool) (any, error) {
	switch v := current.(type) {
	case []any:
		// Extract from array - return array of extracted values
		var results []any
		for _, item := range v {
			if obj, ok := item.(map[string]any); ok {
				if value, exists := obj[extractKey]; exists {
					results = append(results, value)
				} else if createPaths {
					newValue := make(map[string]any)
					obj[extractKey] = newValue
					results = append(results, newValue)
				} else {
					results = append(results, nil)
				}
			} else if createPaths {
				// Convert item to object
				newObj := make(map[string]any)
				newValue := make(map[string]any)
				newObj[extractKey] = newValue
				results = append(results, newValue)
			} else {
				results = append(results, nil)
			}
		}
		return results, nil
	case map[string]any:
		if value, exists := v[extractKey]; exists {
			return value, nil
		}
		if createPaths {
			newValue := make(map[string]any)
			v[extractKey] = newValue
			return newValue, nil
		}
		return nil, fmt.Errorf("extract key '%s' not found", extractKey)
	default:
		if createPaths {
			// Convert to object
			newObj := make(map[string]any)
			newValue := make(map[string]any)
			newObj[extractKey] = newValue
			return newValue, nil
		}
		return nil, fmt.Errorf("cannot perform extract operation on type %T", current)
	}
}

// Helper functions

// parseArrayIndex parses an array index from a string
func (so *setOperations) parseArrayIndex(indexStr string) int {
	// Remove brackets if present
	indexStr = strings.Trim(indexStr, "[]")

	if index, err := strconv.Atoi(indexStr); err == nil {
		return index
	}
	return -999999 // Invalid index marker
}

// parseSliceParameters parses slice parameters from a segment
func (so *setOperations) parseSliceParameters(segment PathSegmentInfo) (start, end, step int) {
	start = 0
	end = -1 // Will be set to array length
	step = 1

	if segment.Start != nil {
		start = *segment.Start
	}
	if segment.End != nil {
		end = *segment.End
	}
	if segment.Step != nil {
		step = *segment.Step
		if step == 0 {
			step = 1
		}
	}

	return start, end, step
}

// determineContainerType determines what type of container to create based on next segment
func (so *setOperations) determineContainerType(segments []PathSegmentInfo, nextIndex int) any {
	if nextIndex >= len(segments) {
		return make(map[string]any) // Default to object
	}

	nextSegment := segments[nextIndex]
	switch nextSegment.Type {
	case "array", "slice":
		return make([]any, 0)
	case "extract":
		return make([]any, 0) // Extract operations typically work on arrays
	default:
		return make(map[string]any)
	}
}

// convertToArray converts data to array type
func (so *setOperations) convertToArray(data any) ([]any, error) {
	if arr, ok := data.([]any); ok {
		return arr, nil
	}

	// Try to convert from other types
	switch v := data.(type) {
	case map[string]any:
		// Try to convert map with numeric keys to array
		return so.tryConvertMapToArray(v)
	case nil:
		return make([]any, 0), nil
	default:
		// Wrap single value in array
		return []any{data}, nil
	}
}

// convertToObject converts data to object type
func (so *setOperations) convertToObject(data any) (map[string]any, error) {
	if obj, ok := data.(map[string]any); ok {
		return obj, nil
	}

	if genericObj, ok := data.(map[any]any); ok {
		// Convert generic map to string map
		result := make(map[string]any)
		for k, v := range genericObj {
			if strKey, ok := k.(string); ok {
				result[strKey] = v
			}
		}
		return result, nil
	}

	// For other types, create empty object
	return make(map[string]any), nil
}

// tryConvertMapToArray tries to convert a map with numeric keys to an array
func (so *setOperations) tryConvertMapToArray(m map[string]any) ([]any, error) {
	// Check if all keys are numeric and consecutive starting from 0
	maxIndex := -1
	for key := range m {
		if index, err := strconv.Atoi(key); err == nil && index >= 0 {
			if index > maxIndex {
				maxIndex = index
			}
		} else {
			// Non-numeric key found, cannot convert to array
			return []any{m}, nil
		}
	}

	// If no numeric keys found or gaps in sequence, wrap in array
	if maxIndex == -1 || len(m) != maxIndex+1 {
		return []any{m}, nil
	}

	// Convert to array
	arr := make([]any, maxIndex+1)
	for i := 0; i <= maxIndex; i++ {
		arr[i] = m[strconv.Itoa(i)]
	}
	return arr, nil
}

// handleArrayExtension handles array extension by navigating to parent and replacing array
func (so *setOperations) handleArrayExtension(data any, segments []PathSegmentInfo, arrayExtErr *ArrayExtensionError) error {
	if len(segments) == 0 {
		return fmt.Errorf("no segments provided for array extension")
	}

	// Navigate to the parent of the array that needs extension
	current := data
	for i := 0; i < len(segments)-1; i++ {
		next, err := so.navigateToSegment(current, segments[i], true, segments, i)
		if err != nil {
			return fmt.Errorf("failed to navigate to segment %d during array extension: %w", i, err)
		}
		current = next
	}

	// Get the final segment and the array that needs extension
	finalSegment := segments[len(segments)-1]

	// Handle different segment types
	switch finalSegment.Type {
	case "array":
		// Direct array access - extend the array in parent container
		return so.extendArrayInParent(current, finalSegment, arrayExtErr)
	case "property":
		// Property access that leads to array - extend the array property
		return so.extendArrayProperty(current, finalSegment, arrayExtErr)
	default:
		return fmt.Errorf("unsupported segment type for array extension: %s", finalSegment.Type)
	}
}

// extendArrayInParent extends an array within its parent container
func (so *setOperations) extendArrayInParent(parent any, segment PathSegmentInfo, arrayExtErr *ArrayExtensionError) error {
	switch v := parent.(type) {
	case map[string]any:
		// Get the current array
		if currentArr, ok := v[segment.Key].([]any); ok {
			// Create extended array
			extendedArr := make([]any, arrayExtErr.RequiredLength)
			copy(extendedArr, currentArr)
			// Set the value at target index
			extendedArr[arrayExtErr.TargetIndex] = arrayExtErr.Value
			// Replace the array in parent
			v[segment.Key] = extendedArr
			return nil
		}
		return fmt.Errorf("property %s is not an array", segment.Key)
	case []any:
		// Parent is array, extend it directly
		if segment.Index >= 0 && segment.Index < len(v) {
			if currentArr, ok := v[segment.Index].([]any); ok {
				// Create extended array
				extendedArr := make([]any, arrayExtErr.RequiredLength)
				copy(extendedArr, currentArr)
				// Set the value at target index
				extendedArr[arrayExtErr.TargetIndex] = arrayExtErr.Value
				// Replace the array in parent
				v[segment.Index] = extendedArr
				return nil
			}
		}
		return fmt.Errorf("array index %d is not an array", segment.Index)
	default:
		return fmt.Errorf("cannot extend array in parent of type %T", parent)
	}
}

// extendArrayProperty extends an array property
func (so *setOperations) extendArrayProperty(parent any, segment PathSegmentInfo, arrayExtErr *ArrayExtensionError) error {
	switch v := parent.(type) {
	case map[string]any:
		// Get the current array
		if currentArr, ok := v[segment.Key].([]any); ok {
			// Create extended array
			extendedArr := make([]any, arrayExtErr.RequiredLength)
			copy(extendedArr, currentArr)
			// Set the value at target index
			extendedArr[arrayExtErr.TargetIndex] = arrayExtErr.Value
			// Replace the array in parent
			v[segment.Key] = extendedArr
			return nil
		}
		// If property doesn't exist or is not an array, create new array
		newArr := make([]any, arrayExtErr.RequiredLength)
		newArr[arrayExtErr.TargetIndex] = arrayExtErr.Value
		v[segment.Key] = newArr
		return nil
	default:
		return fmt.Errorf("cannot set array property on type %T", parent)
	}
}

// extendArrayForIndex extends an array to accommodate a specific index
func (so *setOperations) extendArrayForIndex(arr []any, index int, value any) error {
	// This is a placeholder - in practice, we'd need to modify the parent container
	// For now, return an error indicating array extension is needed
	return fmt.Errorf("array extension required for index %d (current length %d)", index, len(arr))
}

// replaceCurrentData replaces current data with new data
func (so *setOperations) replaceCurrentData(current, newData any) error {
	// This is a placeholder - in practice, we'd need to modify the parent container
	// For now, return an error indicating data replacement is needed
	return fmt.Errorf("data replacement required: %T -> %T", current, newData)
}
