package json

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"github.com/cybergodev/json/internal"
)
type ExtractionGroup struct {
	Segments []PathSegment
}
func (p *Processor) detectConsecutiveExtractions(segments []PathSegment) []ExtractionGroup {
	var groups []ExtractionGroup
	var currentGroup []PathSegment

	for _, seg := range segments {
		if seg.Type == internal.ExtractSegment {
			currentGroup = append(currentGroup, seg)
		} else {
			if len(currentGroup) > 0 {
				groups = append(groups, ExtractionGroup{Segments: currentGroup})
				currentGroup = nil
			}
		}
	}

	if len(currentGroup) > 0 {
		groups = append(groups, ExtractionGroup{Segments: currentGroup})
	}

	return groups
}
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
func (p *Processor) deleteValueAtPath(data any, path string) error {
	// Handle JSON Pointer format
	if strings.HasPrefix(path, "/") {
		return p.deleteValueJSONPointer(data, path)
	}

	// Check for complex paths
	if p.isComplexPath(path) {
		return p.deleteValueComplexPath(data, path)
	}

	// Use dot notation for simple paths
	return p.deleteValueDotNotation(data, path)
}
func (p *Processor) deleteValueDotNotation(data any, path string) error {
	// Parse path
	segments, err := p.parsePath(path)
	if err != nil {
		return err
	}

	if len(segments) == 0 {
		return fmt.Errorf("empty path")
	}

	// Navigate to parent
	current := data
	for i := 0; i < len(segments)-1; i++ {
		segment := segments[i]

		switch v := current.(type) {
		case map[string]any:
			if next, exists := v[segment]; exists {
				current = next
			} else {
				return fmt.Errorf("path not found: %s", segment)
			}
		case []any:
			if index, ok := internal.ParseAndValidateArrayIndex(segment, len(v)); ok {
				current = v[index]
			} else {
				return fmt.Errorf("invalid array index: %s", segment)
			}
		default:
			return fmt.Errorf("cannot navigate through %T at segment %s", current, segment)
		}
	}

	// Delete final property
	finalSegment := segments[len(segments)-1]
	return p.deletePropertyValue(current, finalSegment)
}
func (p *Processor) deleteValueJSONPointer(data any, path string) error {
	if path == "/" {
		return fmt.Errorf("cannot delete root")
	}

	// Remove leading slash and split
	pathWithoutSlash := path[1:]
	segments := strings.Split(pathWithoutSlash, "/")

	// Navigate to parent
	current := data
	for i := 0; i < len(segments)-1; i++ {
		segment := segments[i]

		// Unescape JSON Pointer characters
		if strings.Contains(segment, "~") {
			segment = p.unescapeJSONPointer(segment)
		}

		switch v := current.(type) {
		case map[string]any:
			if next, exists := v[segment]; exists {
				current = next
			} else {
				return fmt.Errorf("path not found: %s", segment)
			}
		case []any:
			if index, ok := internal.ParseAndValidateArrayIndex(segment, len(v)); ok {
				current = v[index]
			} else {
				return fmt.Errorf("invalid array index: %s", segment)
			}
		default:
			return fmt.Errorf("cannot navigate through %T at segment %s", current, segment)
		}
	}

	// Delete final property
	finalSegment := segments[len(segments)-1]
	if strings.Contains(finalSegment, "~") {
		finalSegment = p.unescapeJSONPointer(finalSegment)
	}

	return p.deletePropertyValue(current, finalSegment)
}
func (p *Processor) deletePropertyValue(current any, property string) error {
	switch v := current.(type) {
	case map[string]any:
		if _, exists := v[property]; exists {
			delete(v, property)
			return nil
		}
		return fmt.Errorf("property not found: %s", property)

	case map[any]any:
		if _, exists := v[property]; exists {
			delete(v, property)
			return nil
		}
		return fmt.Errorf("property not found: %s", property)

	case []any:
		if _, err := strconv.Atoi(property); err == nil {
			return p.deleteArrayElement(current, property)
		}
		return fmt.Errorf("invalid array index: %s", property)

	default:
		return fmt.Errorf("cannot delete property '%s' from type %T", property, current)
	}
}
func (p *Processor) deleteValueComplexPath(data any, path string) error {
	// Parse path into segments
	segments := p.getPathSegments()
	defer p.putPathSegments(segments)

	segments = p.splitPath(path, segments)

	// Check if this requires complex deletion
	if p.hasComplexSegments(segments) {
		return p.deleteValueComplexSegments(data, segments, 0)
	}

	// Convert to internal segments for complex processing
	internalSegments := make([]internal.PathSegment, len(segments))
	for i, seg := range segments {
		internalSegments[i] = internal.PathSegment{
			Type:       seg.Type,
			Key:        seg.Key,
			Index:      seg.Index,
			Start:      seg.Start,
			End:        seg.End,
			Step:       seg.Step,
			IsNegative: seg.Index < 0,
			IsWildcard: seg.Type == internal.WildcardSegment,
			IsFlat:     seg.IsFlat,
		}
	}

	return p.deleteValueWithInternalSegments(data, internalSegments)
}
func (p *Processor) requiresComplexDeletion(segments []internal.PathSegment) bool {
	for _, segment := range segments {
		switch segment.Type {
		case internal.ArraySliceSegment, internal.ExtractSegment:
			return true
		}
	}
	return false
}
func (p *Processor) deleteValueWithInternalSegments(data any, segments []internal.PathSegment) error {
	if len(segments) == 0 {
		return fmt.Errorf("no segments provided")
	}

	// Navigate to parent
	current := data
	for i := 0; i < len(segments)-1; i++ {
		next, err := p.navigateSegmentForDeletion(current, segments[i])
		if err != nil {
			return err
		}
		current = next
	}

	// Delete final segment
	finalSegment := segments[len(segments)-1]
	return p.deleteValueForSegment(current, finalSegment)
}
func (p *Processor) navigateSegmentForDeletion(current any, segment PathSegment) (any, error) {
	switch segment.TypeString() {
	case "property":
		return p.navigatePropertyForDeletion(current, segment.Key)
	case "array":
		return p.navigateArrayIndexForDeletion(current, segment.String())
	case "slice":
		// For slices, return the current container
		return current, nil
	case "extract":
		// For extractions, return the current container
		return current, nil
	default:
		return nil, fmt.Errorf("unsupported segment type for deletion: %v", segment.TypeString())
	}
}
func (p *Processor) navigatePropertyForDeletion(current any, property string) (any, error) {
	switch v := current.(type) {
	case map[string]any:
		if val, exists := v[property]; exists {
			return val, nil
		}
		return nil, fmt.Errorf("property not found: %s", property)
	case map[any]any:
		if val, exists := v[property]; exists {
			return val, nil
		}
		return nil, fmt.Errorf("property not found: %s", property)
	default:
		return nil, fmt.Errorf("cannot access property '%s' on type %T", property, current)
	}
}
func (p *Processor) navigateArrayIndexForDeletion(current any, indexStr string) (any, error) {
	arr, ok := current.([]any)
	if !ok {
		return nil, fmt.Errorf("cannot access array index on type %T", current)
	}

	index, err := strconv.Atoi(indexStr)
	if err != nil {
		return nil, fmt.Errorf("invalid array index: %s", indexStr)
	}

	// Handle negative indices
	if index < 0 {
		index = len(arr) + index
	}

	if index < 0 || index >= len(arr) {
		return nil, fmt.Errorf("array index %d out of bounds", index)
	}

	return arr[index], nil
}
func (p *Processor) deleteValueForSegment(current any, segment PathSegment) error {
	switch segment.TypeString() {
	case "property":
		return p.deletePropertyFromContainer(current, segment.Key)
	case "array":
		return p.deleteArrayElementByIndex(current, segment.Index)
	case "slice":
		return p.deleteArraySlice(current, segment)
	case "extract":
		return p.deleteExtractedValues(current, segment)
	default:
		return fmt.Errorf("unsupported segment type for deletion: %v", segment.TypeString())
	}
}
func (p *Processor) deletePropertyFromContainer(current any, property string) error {
	switch v := current.(type) {
	case map[string]any:
		if _, exists := v[property]; exists {
			delete(v, property)
			return nil
		}
		return fmt.Errorf("property not found: %s", property)
	case map[any]any:
		if _, exists := v[property]; exists {
			delete(v, property)
			return nil
		}
		return fmt.Errorf("property not found: %s", property)
	default:
		return fmt.Errorf("cannot delete property '%s' from type %T", property, current)
	}
}
func (p *Processor) deleteArrayElement(current any, indexStr string) error {
	arr, ok := current.([]any)
	if !ok {
		return fmt.Errorf("cannot delete array element from type %T", current)
	}

	index, err := strconv.Atoi(indexStr)
	if err != nil {
		return fmt.Errorf("invalid array index: %s", indexStr)
	}

	// Handle negative indices
	if index < 0 {
		index = len(arr) + index
	}

	if index < 0 || index >= len(arr) {
		return fmt.Errorf("array index %d out of bounds", index)
	}

	// Mark element for deletion (set to special marker)
	arr[index] = DeletedMarker
	return nil
}
func (p *Processor) deleteArrayElementByIndex(current any, index int) error {
	arr, ok := current.([]any)
	if !ok {
		return fmt.Errorf("cannot delete array element from type %T", current)
	}

	// Handle negative indices
	if index < 0 {
		index = len(arr) + index
	}

	if index < 0 || index >= len(arr) {
		return fmt.Errorf("array index %d out of bounds", index)
	}

	// Mark element for deletion (set to special marker)
	arr[index] = DeletedMarker
	return nil
}
func (p *Processor) deleteArraySlice(current any, segment PathSegment) error {
	arr, ok := current.([]any)
	if !ok {
		return fmt.Errorf("cannot delete slice from type %T", current)
	}

	// Parse slice parameters
	start, end, step, err := p.parseSliceParameters(segment.String(), len(arr))
	if err != nil {
		return err
	}

	// Handle negative indices
	if start < 0 {
		start = len(arr) + start
	}
	if end < 0 {
		end = len(arr) + end
	}

	// Bounds checking
	if start < 0 || start >= len(arr) || end < 0 || end > len(arr) || start >= end {
		return fmt.Errorf("slice range [%d:%d] out of bounds for array length %d", start, end, len(arr))
	}

	// Mark elements for deletion
	for i := start; i < end; i += step {
		arr[i] = DeletedMarker
	}

	return nil
}
func (p *Processor) deleteExtractedValues(current any, segment PathSegment) error {
	field := segment.Key
	if field == "" {
		return fmt.Errorf("invalid extraction syntax: %s", segment.String())
	}

	// Handle array extraction
	if arr, ok := current.([]any); ok {
		for _, item := range arr {
			if obj, ok := item.(map[string]any); ok {
				delete(obj, field)
			}
		}
		return nil
	}

	// Handle single object
	if obj, ok := current.(map[string]any); ok {
		delete(obj, field)
		return nil
	}

	return fmt.Errorf("cannot perform extraction deletion on type %T", current)
}
func (p *Processor) deleteValueComplexSegments(data any, segments []PathSegment, segmentIndex int) error {
	if segmentIndex >= len(segments) {
		return nil
	}

	segment := segments[segmentIndex]

	switch segment.TypeString() {
	case "property":
		return p.deleteComplexProperty(data, segment, segments, segmentIndex)
	case "array":
		return p.deleteComplexArray(data, segment, segments, segmentIndex)
	case "slice":
		return p.deleteComplexSlice(data, segment, segments, segmentIndex)
	case "extract":
		return p.deleteComplexExtract(data, segment, segments, segmentIndex)
	default:
		return fmt.Errorf("unsupported complex segment type: %v", segment.TypeString())
	}
}
func (p *Processor) deleteComplexProperty(data any, segment PathSegment, segments []PathSegment, segmentIndex int) error {
	if segmentIndex == len(segments)-1 {
		// Last segment, delete the property
		return p.deletePropertyFromContainer(data, segment.Key)
	}

	// Navigate to next level
	switch v := data.(type) {
	case map[string]any:
		if next, exists := v[segment.Key]; exists {
			return p.deleteValueComplexSegments(next, segments, segmentIndex+1)
		}
		return fmt.Errorf("property not found: %s", segment.Key)
	case map[any]any:
		if next, exists := v[segment.Key]; exists {
			return p.deleteValueComplexSegments(next, segments, segmentIndex+1)
		}
		return fmt.Errorf("property not found: %s", segment.Key)
	default:
		return fmt.Errorf("cannot access property '%s' on type %T", segment.Key, data)
	}
}
func (p *Processor) deleteComplexArray(data any, segment PathSegment, segments []PathSegment, segmentIndex int) error {
	arr, ok := data.([]any)
	if !ok {
		return fmt.Errorf("cannot access array on type %T", data)
	}

	index := segment.Index

	// Handle negative indices
	if index < 0 {
		index = len(arr) + index
	}

	if index < 0 || index >= len(arr) {
		return fmt.Errorf("array index %d out of bounds", index)
	}

	if segmentIndex == len(segments)-1 {
		// Last segment, delete the array element
		arr[index] = DeletedMarker
		return nil
	}

	// Navigate to next level
	return p.deleteValueComplexSegments(arr[index], segments, segmentIndex+1)
}
func (p *Processor) deleteComplexSlice(data any, segment PathSegment, segments []PathSegment, segmentIndex int) error {
	arr, ok := data.([]any)
	if !ok {
		return fmt.Errorf("cannot perform slice operation on type %T", data)
	}

	if segmentIndex == len(segments)-1 {
		// Last segment, delete the slice
		return p.deleteArraySlice(data, segment)
	}

	// For intermediate slices, we need to apply the operation to each element in the slice
	start, end, step, err := p.parseSliceParameters(segment.String(), len(arr))
	if err != nil {
		return err
	}

	// Handle negative indices
	if start < 0 {
		start = len(arr) + start
	}
	if end < 0 {
		end = len(arr) + end
	}

	// Apply deletion to each element in the slice
	for i := start; i < end && i < len(arr); i += step {
		if err := p.deleteValueComplexSegments(arr[i], segments, segmentIndex+1); err != nil {
			// Continue with other elements even if one fails
			continue
		}
	}

	return nil
}
func (p *Processor) deleteComplexExtract(data any, segment PathSegment, segments []PathSegment, segmentIndex int) error {
	field := segment.Key
	if field == "" {
		return fmt.Errorf("invalid extraction syntax: %s", segment.String())
	}

	if segmentIndex == len(segments)-1 {
		// Last segment, delete extracted values
		return p.deleteExtractedValues(data, segment)
	}

	// Check for consecutive extractions
	if p.hasConsecutiveExtractions(segments, segmentIndex) {
		return p.deleteConsecutiveExtractions(data, segments, segmentIndex)
	}

	// Handle array extraction with further navigation
	if arr, ok := data.([]any); ok {
		for _, item := range arr {
			if obj, ok := item.(map[string]any); ok {
				if extractedValue, exists := obj[field]; exists {
					if err := p.deleteValueComplexSegments(extractedValue, segments, segmentIndex+1); err != nil {
						// Continue with other items even if one fails
						continue
					}
				}
			}
		}
		return nil
	}

	// Handle single object extraction
	if obj, ok := data.(map[string]any); ok {
		if extractedValue, exists := obj[field]; exists {
			return p.deleteValueComplexSegments(extractedValue, segments, segmentIndex+1)
		}
		return fmt.Errorf("extraction field '%s' not found", field)
	}

	return fmt.Errorf("cannot perform extraction on type %T", data)
}
func (p *Processor) hasConsecutiveExtractions(segments []PathSegment, startIndex int) bool {
	if startIndex+1 >= len(segments) {
		return false
	}

	return segments[startIndex].TypeString() == "extract" &&
		segments[startIndex+1].TypeString() == "extract"
}
func (p *Processor) deleteConsecutiveExtractions(data any, segments []PathSegment, segmentIndex int) error {
	// Find all consecutive extraction segments
	var extractionSegments []PathSegment
	i := segmentIndex
	for i < len(segments) && segments[i].TypeString() == "extract" {
		extractionSegments = append(extractionSegments, segments[i])
		i++
	}

	remainingSegments := segments[i:]

	return p.processConsecutiveExtractionsForDeletion(data, extractionSegments, remainingSegments)
}
func (p *Processor) processConsecutiveExtractionsForDeletion(data any, extractionSegments []PathSegment, remainingSegments []PathSegment) error {
	if len(extractionSegments) == 0 {
		return nil
	}

	// Apply first extraction
	firstExtraction := extractionSegments[0]
	field := firstExtraction.Key

	if arr, ok := data.([]any); ok {
		for _, item := range arr {
			if obj, ok := item.(map[string]any); ok {
				if extractedValue, exists := obj[field]; exists {
					if len(extractionSegments) == 1 {
						// Last extraction, apply remaining segments or delete
						if len(remainingSegments) == 0 {
							delete(obj, field)
						} else {
							p.deleteValueComplexSegments(extractedValue, remainingSegments, 0)
						}
					} else {
						// More extractions to process
						p.processConsecutiveExtractionsForDeletion(extractedValue, extractionSegments[1:], remainingSegments)
					}
				}
			}
		}
	}

	return nil
}
func (p *Processor) deleteDeepExtractedValues(data any, extractKey string, remainingSegments []PathSegment) error {
	if arr, ok := data.([]any); ok {
		for _, item := range arr {
			if obj, ok := item.(map[string]any); ok {
				if extractedValue, exists := obj[extractKey]; exists {
					if len(remainingSegments) == 0 {
						delete(obj, extractKey)
					} else {
						p.deleteValueComplexSegments(extractedValue, remainingSegments, 0)
					}
				}
			}
		}
	}

	return nil
}
func (p *Processor) cleanupNullValues(data any) {
	p.cleanupNullValuesRecursive(data)
}
func (p *Processor) cleanupNullValuesRecursive(data any) {
	switch v := data.(type) {
	case map[string]any:
		for key, value := range v {
			if value == nil {
				delete(v, key)
			} else {
				p.cleanupNullValuesRecursive(value)
			}
		}
	case []any:
		p.cleanupArrayNulls(v)
		for _, item := range v {
			if item != nil {
				p.cleanupNullValuesRecursive(item)
			}
		}
	}
}
func (p *Processor) isEmptyContainer(data any) bool {
	switch v := data.(type) {
	case map[string]any:
		return len(v) == 0
	case map[any]any:
		return len(v) == 0
	case []any:
		return len(v) == 0
	case string:
		return v == ""
	default:
		return false
	}
}
func (p *Processor) isContainer(data any) bool {
	switch data.(type) {
	case map[string]any, map[any]any, []any:
		return true
	default:
		return false
	}
}
func (p *Processor) cleanupNullValuesWithReconstruction(data any, compactArrays bool) any {
	return p.cleanupNullValuesRecursiveWithReconstruction(data, compactArrays)
}
func (p *Processor) cleanupNullValuesRecursiveWithReconstruction(data any, compactArrays bool) any {
	switch v := data.(type) {
	case map[string]any:
		result := make(map[string]any)
		for key, value := range v {
			if value != nil {
				cleanedValue := p.cleanupNullValuesRecursiveWithReconstruction(value, compactArrays)
				if cleanedValue != nil && !p.isEmptyContainer(cleanedValue) {
					result[key] = cleanedValue
				}
			}
		}
		return result

	case []any:
		if compactArrays {
			return p.cleanupArrayWithReconstruction(v, compactArrays)
		}
		result := make([]any, len(v))
		for i, item := range v {
			if item != nil {
				result[i] = p.cleanupNullValuesRecursiveWithReconstruction(item, compactArrays)
			}
		}
		return result

	default:
		return data
	}
}
func (p *Processor) cleanupDeletedMarkers(data any) any {
	switch v := data.(type) {
	case []any:
		result := make([]any, 0, len(v))
		for _, item := range v {
			if item != DeletedMarker {
				result = append(result, p.cleanupDeletedMarkers(item))
			}
		}
		return result

	case map[string]any:
		result := make(map[string]any)
		for key, value := range v {
			if value != DeletedMarker {
				result[key] = p.cleanupDeletedMarkers(value)
			}
		}
		return result

	default:
		return data
	}
}
func (p *Processor) handleExtraction(data any, segment PathSegment) (any, error) {
	field := segment.Key
	if field == "" {
		return nil, fmt.Errorf("invalid extraction syntax: %s", segment.String())
	}

	// Handle array extraction with pre-allocated results slice and flattening
	if arr, ok := data.([]any); ok {
		results := make([]any, 0, len(arr)) // Pre-allocate with array length

		for _, item := range arr {
			// Use the existing handlePropertyAccessValue function for consistent field extraction
			if value := p.handlePropertyAccessValue(item, field); value != nil {
				if segment.IsFlat {
					// For flat extraction, always flatten arrays recursively
					p.flattenValue(value, &results)
				} else {
					// For regular extraction, add the field value directly
					results = append(results, value)
				}
			}
		}
		return results, nil
	}

	// Handle single object extraction
	if obj, ok := data.(map[string]any); ok {
		if value := p.handlePropertyAccessValue(obj, field); value != nil {
			return value, nil
		}
	}

	// For non-extractable types (strings, numbers, etc.), return nil without error
	// This matches the expected behavior in tests
	return nil, nil
}
func (p *Processor) flattenValue(value any, results *[]any) {
	if arr, ok := value.([]any); ok {
		// If it's an array, recursively flatten each element
		for _, item := range arr {
			p.flattenValue(item, results)
		}
	} else {
		// If it's not an array, add it directly to results
		*results = append(*results, value)
	}
}
func (p *Processor) handleStructAccess(data any, fieldName string) any {
	if data == nil {
		return nil
	}

	v := reflect.ValueOf(data)
	if v.Kind() == reflect.Ptr {
		if v.IsNil() {
			return nil
		}
		v = v.Elem()
	}

	if v.Kind() != reflect.Struct {
		return nil
	}

	// Try direct field access first
	field := v.FieldByName(fieldName)
	if field.IsValid() && field.CanInterface() {
		return field.Interface()
	}

	// Try case-insensitive field access
	t := v.Type()
	for i := 0; i < v.NumField(); i++ {
		structField := t.Field(i)
		if strings.EqualFold(structField.Name, fieldName) {
			field := v.Field(i)
			if field.CanInterface() {
				return field.Interface()
			}
		}
	}

	return nil
}
func (p *Processor) handleDeepExtractionInNavigation(data any, segments []PathSegment) (any, error) {
	// Group consecutive extraction segments
	extractionGroups := p.detectConsecutiveExtractions(segments)

	current := data
	segmentIndex := 0

	for _, group := range extractionGroups {
		// Process consecutive extractions
		result, err := p.processConsecutiveExtractions(current, group.Segments, segments[segmentIndex+len(group.Segments):])
		if err != nil {
			return nil, err
		}
		current = result
		segmentIndex += len(group.Segments)
	}

	return current, nil
}
func (p *Processor) hasDeepExtractionPattern(segments []PathSegment) bool {
	extractionCount := 0
	for _, segment := range segments {
		if segment.TypeString() == "extract" {
			extractionCount++
			if extractionCount > 1 {
				return true
			}
		}
	}
	return false
}
func (p *Processor) hasMixedExtractionOperations(segments []PathSegment) bool {
	hasFlat := false
	hasRegular := false

	for _, segment := range segments {
		if segment.TypeString() == "extract" {
			if segment.IsFlat {
				hasFlat = true
			} else {
				hasRegular = true
			}

			if hasFlat && hasRegular {
				return true
			}
		}
	}

	return false
}
func (p *Processor) getValueWithDistributedOperation(data any, path string) (any, error) {
	// Parse the path to identify distributed operation patterns
	segments := p.getPathSegments()
	defer p.putPathSegments(segments)

	segments = p.splitPath(path, segments)

	// Find the extraction segment that triggers distributed operation
	extractionIndex := -1
	for i, segment := range segments {
		if segment.TypeString() == "extract" {
			// Check if this is followed by array operations
			if i+1 < len(segments) {
				nextSegment := segments[i+1]
				if nextSegment.TypeString() == "array" || nextSegment.TypeString() == "slice" {
					extractionIndex = i
					break
				}
			}
		}
	}

	if extractionIndex == -1 {
		// No distributed operation pattern found, use regular navigation
		return p.navigateToPath(data, path)
	}

	// Split segments into pre-extraction, extraction, and post-extraction
	preSegments := segments[:extractionIndex]
	extractionSegment := segments[extractionIndex]
	postSegments := segments[extractionIndex+1:]

	// Navigate to the extraction point
	current := data
	for _, segment := range preSegments {
		switch segment.TypeString() {
		case "property":
			result := p.handlePropertyAccess(current, segment.Key)
			if !result.Exists {
				return nil, ErrPathNotFound
			}
			current = result.Value
		case "array":
			result := p.handleArrayAccess(current, segment)
			if !result.Exists {
				return nil, ErrPathNotFound
			}
			current = result.Value
		}
	}

	// Extract individual arrays
	extractedArrays, err := p.extractIndividualArrays(current, extractionSegment)
	if err != nil {
		return nil, err
	}

	// Apply post-extraction operations to each array
	results := make([]any, 0, len(extractedArrays))
	for _, arr := range extractedArrays {
		// Apply post-extraction segments
		result := arr
		for _, segment := range postSegments {
			switch segment.TypeString() {
			case "array":
				result = p.applySingleArrayOperation(result, segment)
			case "slice":
				result = p.applySingleArraySlice(result, segment)
			}
		}

		// Add result if it's not nil
		if result != nil {
			results = append(results, result)
		}
	}

	return results, nil
}
func (p *Processor) extractIndividualArrays(data any, extractionSegment PathSegment) ([]any, error) {
	field := extractionSegment.Key
	if field == "" {
		return nil, fmt.Errorf("invalid extraction syntax: %s", extractionSegment.String())
	}

	// Pre-allocate with estimated capacity
	var results []any
	if arr, ok := data.([]any); ok {
		results = make([]any, 0, len(arr))
		for _, item := range arr {
			if obj, ok := item.(map[string]any); ok {
				if value := p.handlePropertyAccessValue(obj, field); value != nil {
					// Check if the extracted value is an array
					if extractedArr, ok := value.([]any); ok {
						results = append(results, extractedArr)
					}
				}
			}
		}
	}

	return results, nil
}
func (p *Processor) applySingleArrayOperation(array any, segment PathSegment) any {
	if arr, ok := array.([]any); ok {
		result := p.handleArrayAccess(arr, segment)
		if result.Exists {
			return result.Value
		}
	}
	return nil
}
func (p *Processor) applySingleArraySlice(array any, segment PathSegment) any {
	if arr, ok := array.([]any); ok {
		result := p.handleArraySlice(arr, segment)
		if result.Exists {
			return result.Value
		}
	}
	return nil
}
func (p *Processor) findTargetArrayForDistributedOperation(item any) []any {
	// This method would contain logic to find the target array within an item
	// For now, return the item if it's an array
	if arr, ok := item.([]any); ok {
		return arr
	}
	return nil
}
func (p *Processor) handlePostExtractionArrayAccess(data any, segment PathSegment) any {
	// Check if data is an array of arrays (result of extraction)
	if arr, ok := data.([]any); ok {
		results := make([]any, 0, len(arr))

		for _, item := range arr {
			if itemArr, ok := item.([]any); ok {
				// Apply array operation to each sub-array
				result := p.applySingleArrayOperation(itemArr, segment)
				if result != nil {
					results = append(results, result)
				}
			}
		}

		return results
	}

	// For single array, apply operation directly
	return p.applySingleArrayOperation(data, segment)
}
func (p *Processor) handleDistributedArrayAccess(data any, segment PathSegment) any {
	return p.handlePostExtractionArrayAccess(data, segment)
}
func (p *Processor) handlePostExtractionArraySlice(data any, segment PathSegment) any {
	// Check if data is an array of arrays (result of extraction)
	if arr, ok := data.([]any); ok {
		results := make([]any, 0, len(arr))

		for _, item := range arr {
			if itemArr, ok := item.([]any); ok {
				// Apply slice operation to each sub-array
				result := p.applySingleArraySlice(itemArr, segment)
				if result != nil {
					results = append(results, result)
				}
			}
		}

		return results
	}

	// For single array, apply operation directly
	return p.applySingleArraySlice(data, segment)
}
func (p *Processor) processConsecutiveExtractions(data any, extractionSegments []PathSegment, remainingSegments []PathSegment) (any, error) {
	current := data

	// Apply each extraction in sequence
	for _, segment := range extractionSegments {
		result, err := p.handleExtraction(current, segment)
		if err != nil {
			return nil, err
		}
		current = result
	}

	return current, nil
}
func (p *Processor) buildExtractionContext(data any, segments []PathSegment) (*ExtractionContext, error) {
	// Find extraction segments and build context
	var extractionSegments []PathSegment
	var arrayFieldName string

	for _, segment := range segments {
		if segment.TypeString() == "extract" {
			extractionSegments = append(extractionSegments, segment)
			if arrayFieldName == "" {
				arrayFieldName = segment.Key
			}
		}
	}

	if len(extractionSegments) == 0 {
		return nil, fmt.Errorf("no extraction segments found")
	}

	// Collect source containers
	containers, err := p.collectSourceContainers(data, extractionSegments)
	if err != nil {
		return nil, err
	}

	return &ExtractionContext{
		OriginalContainers: containers,
		ArrayFieldName:     arrayFieldName,
		TargetIndices:      make([]int, len(containers)),
		OperationType:      "get",
	}, nil
}
func (p *Processor) collectSourceContainers(data any, extractionSegments []PathSegment) ([]any, error) {
	var containers []any

	// This is a simplified implementation
	// The full implementation would recursively traverse the data structure
	if arr, ok := data.([]any); ok {
		for _, item := range arr {
			containers = append(containers, item)
		}
	} else {
		containers = append(containers, data)
	}

	return containers, nil
}
func (p *Processor) collectContainersForExtraction(data any, extractionSegments []PathSegment, containers *[]any) error {
	return p.collectContainersRecursive(data, extractionSegments, 0, containers)
}
func (p *Processor) collectContainersRecursive(current any, segments []PathSegment, segmentIndex int, containers *[]any) error {
	if segmentIndex >= len(segments) {
		*containers = append(*containers, current)
		return nil
	}

	segment := segments[segmentIndex]

	switch segment.TypeString() {
	case "property":
		if obj, ok := current.(map[string]any); ok {
			if value, exists := obj[segment.Key]; exists {
				return p.collectContainersRecursive(value, segments, segmentIndex+1, containers)
			}
		}
	case "array":
		if arr, ok := current.([]any); ok {
			for _, item := range arr {
				if err := p.collectContainersRecursive(item, segments, segmentIndex+1, containers); err != nil {
					return err
				}
			}
		}
	case "extract":
		// For extraction, collect all items that would be extracted
		if arr, ok := current.([]any); ok {
			for _, item := range arr {
				*containers = append(*containers, item)
			}
		}
	}

	return nil
}
func (p *Processor) flattenContainersRecursive(arr []any, containers *[]any) {
	for _, item := range arr {
		if nestedArr, ok := item.([]any); ok {
			p.flattenContainersRecursive(nestedArr, containers)
		} else {
			*containers = append(*containers, item)
		}
	}
}
type ExtractionContext struct {
	OriginalContainers []any  // Original containers that hold the target arrays
	ArrayFieldName     string // Name of the array field being operated on
	TargetIndices      []int  // Target indices for each container
	OperationType      string // Type of operation: "get", "set", "delete"
}

// extractionOperations provides extraction operations
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

func (p *Processor) setValueAtPathWithOptions(data any, path string, value any, createPaths bool) error {
	if path == "" || path == "." {
		return fmt.Errorf("cannot set root value")
	}

	// Use advanced path parsing for full feature support
	return p.setValueAdvancedPath(data, path, value, createPaths)
}
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
	if p.isComplexPath(path) && !p.isSimpleArraySlicePath(path) {
		// Use RecursiveProcessor for complex paths like flat extraction
		unifiedProcessor := NewRecursiveProcessor(p)
		_, err := unifiedProcessor.ProcessRecursivelyWithOptions(data, path, OpSet, value, createPaths)
		return err
	}

	// Use dot notation with segments for simple paths
	return p.setValueDotNotationWithCreation(data, path, value, createPaths)
}
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
func (p *Processor) setValueWithSegments(data any, segments []PathSegment, value any, createPaths bool) error {
	if len(segments) == 0 {
		return fmt.Errorf("no segments provided")
	}

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
func (p *Processor) navigateToSegment(current any, segment PathSegment, createPaths bool, allSegments []PathSegment, currentIndex int) (any, error) {
	switch segment.TypeString() {
	case "property":
		return p.navigateToProperty(current, segment.Key, createPaths, allSegments, currentIndex)
	case "array":
		// Get array index from segment
		index := segment.Index
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
func (p *Processor) navigateToExtraction(current any, segment PathSegment, createPaths bool, allSegments []PathSegment, currentIndex int) (any, error) {
	field := segment.Key
	if field == "" {
		return nil, fmt.Errorf("invalid extraction syntax: %s", segment.String())
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
func (p *Processor) setValueForSegment(current any, segment PathSegment, value any, createPaths bool) error {
	switch segment.TypeString() {
	case "property":
		return p.setValueForProperty(current, segment.Key, value, createPaths)
	case "array":
		index := segment.Index
		return p.setValueForArrayIndex(current, index, value, createPaths)
	case "slice":
		return p.setValueForArraySlice(current, segment, value, createPaths)
	case "extract":
		return p.setValueForExtract(current, segment, value, createPaths)
	default:
		return fmt.Errorf("unsupported segment type for set: %v", segment.TypeString())
	}
}
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
			// Cannot convert non-map types to map for property setting
			// This is a fundamental limitation
			return fmt.Errorf("cannot convert %T to map for property setting", current)
		}
		return fmt.Errorf("cannot set property '%s' on type %T", property, current)
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
func (p *Processor) setValueForExtract(current any, segment PathSegment, value any, createPaths bool) error {
	field := segment.Key
	if field == "" {
		return fmt.Errorf("invalid extraction syntax: %s", segment.String())
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
func (p *Processor) setValueDotNotation(data any, path string, value any) error {
	return p.setValueDotNotationWithCreation(data, path, value, false)
}
func (p *Processor) setValueDotNotationWithCreation(data any, path string, value any, createPaths bool) error {
	// Parse path into segments
	segments := p.getPathSegments()
	defer p.putPathSegments(segments)

	segments = p.splitPath(path, segments)

	return p.setValueWithSegments(data, segments, value, createPaths)
}
func (p *Processor) setValueJSONPointer(data any, path string, value any) error {
	return p.setValueJSONPointerWithCreation(data, path, value)
}
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
