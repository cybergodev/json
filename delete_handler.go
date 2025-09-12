package json

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/cybergodev/json/internal"
)

// deleteValueAtPath deletes a value at the specified path
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

// deleteValueDotNotation deletes a value using dot notation
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
			if index, err := strconv.Atoi(segment); err == nil && index >= 0 && index < len(v) {
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

// deleteValueJSONPointer deletes a value using JSON Pointer format
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
			if index, err := strconv.Atoi(segment); err == nil && index >= 0 && index < len(v) {
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

// deletePropertyValue deletes a property from a container
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

// deleteValueComplexPath handles complex path deletions
func (p *Processor) deleteValueComplexPath(data any, path string) error {
	// Parse path into segments
	segments := p.getPathSegments()
	defer p.putPathSegments(segments)

	segments = p.splitPath(path, segments)

	// Check if this requires complex deletion
	if p.hasComplexSegments(segments) {
		return p.deleteValueComplexSegments(data, segments, 0)
	}

	// Use internal segments for complex processing
	internalSegments := make([]internal.PathSegment, len(segments))
	for i, seg := range segments {
		internalSegments[i] = internal.NewLegacyPathSegment(seg.TypeString(), seg.Value)
	}

	return p.deleteValueWithInternalSegments(data, internalSegments)
}

// requiresComplexDeletion checks if segments require complex deletion
func (p *Processor) requiresComplexDeletion(segments []internal.PathSegment) bool {
	for _, segment := range segments {
		switch segment.Type {
		case internal.ArraySliceSegment, internal.ExtractSegment:
			return true
		}
	}
	return false
}

// deleteValueWithInternalSegments deletes using internal path segments
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

// navigateSegmentForDeletion navigates to a segment for deletion
func (p *Processor) navigateSegmentForDeletion(current any, segment PathSegment) (any, error) {
	switch segment.TypeString() {
	case "property":
		return p.navigatePropertyForDeletion(current, segment.Value)
	case "array":
		return p.navigateArrayIndexForDeletion(current, segment.Value)
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

// navigatePropertyForDeletion navigates to a property for deletion
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

// navigateArrayIndexForDeletion navigates to an array index for deletion
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

// deleteValueForSegment deletes a value for a specific segment
func (p *Processor) deleteValueForSegment(current any, segment PathSegment) error {
	switch segment.TypeString() {
	case "property":
		return p.deletePropertyFromContainer(current, segment.Value)
	case "array":
		return p.deleteArrayElement(current, segment.Value)
	case "slice":
		return p.deleteArraySlice(current, segment)
	case "extract":
		return p.deleteExtractedValues(current, segment)
	default:
		return fmt.Errorf("unsupported segment type for deletion: %v", segment.TypeString())
	}
}

// deletePropertyFromContainer deletes a property from a container
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

// deleteArrayElement deletes an element from an array
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

// DeletedMarker is a special type to mark deleted array elements

// deleteArraySlice deletes a slice from an array
func (p *Processor) deleteArraySlice(current any, segment PathSegment) error {
	arr, ok := current.([]any)
	if !ok {
		return fmt.Errorf("cannot delete slice from type %T", current)
	}

	// Parse slice parameters
	start, end, step, err := p.parseSliceParameters(segment.Value, len(arr))
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

// deleteExtractedValues deletes values based on extraction
func (p *Processor) deleteExtractedValues(current any, segment PathSegment) error {
	field := segment.Extract
	if field == "" {
		return fmt.Errorf("invalid extraction syntax: %s", segment.Value)
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

// deleteValueComplexSegments handles complex segment deletion
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

// deleteComplexProperty handles complex property deletion
func (p *Processor) deleteComplexProperty(data any, segment PathSegment, segments []PathSegment, segmentIndex int) error {
	if segmentIndex == len(segments)-1 {
		// Last segment, delete the property
		return p.deletePropertyFromContainer(data, segment.Value)
	}

	// Navigate to next level
	switch v := data.(type) {
	case map[string]any:
		if next, exists := v[segment.Value]; exists {
			return p.deleteValueComplexSegments(next, segments, segmentIndex+1)
		}
		return fmt.Errorf("property not found: %s", segment.Value)
	case map[any]any:
		if next, exists := v[segment.Value]; exists {
			return p.deleteValueComplexSegments(next, segments, segmentIndex+1)
		}
		return fmt.Errorf("property not found: %s", segment.Value)
	default:
		return fmt.Errorf("cannot access property '%s' on type %T", segment.Value, data)
	}
}

// deleteComplexArray handles complex array deletion
func (p *Processor) deleteComplexArray(data any, segment PathSegment, segments []PathSegment, segmentIndex int) error {
	arr, ok := data.([]any)
	if !ok {
		return fmt.Errorf("cannot access array on type %T", data)
	}

	index, err := strconv.Atoi(segment.Value)
	if err != nil {
		return fmt.Errorf("invalid array index: %s", segment.Value)
	}

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

// deleteComplexSlice handles complex slice deletion
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
	start, end, step, err := p.parseSliceParameters(segment.Value, len(arr))
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

// deleteComplexExtract handles complex extraction deletion
func (p *Processor) deleteComplexExtract(data any, segment PathSegment, segments []PathSegment, segmentIndex int) error {
	field := segment.Extract
	if field == "" {
		return fmt.Errorf("invalid extraction syntax: %s", segment.Value)
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

// hasConsecutiveExtractions checks for consecutive extractions starting at index
func (p *Processor) hasConsecutiveExtractions(segments []PathSegment, startIndex int) bool {
	if startIndex+1 >= len(segments) {
		return false
	}

	return segments[startIndex].TypeString() == "extract" &&
		segments[startIndex+1].TypeString() == "extract"
}

// deleteConsecutiveExtractions handles consecutive extraction deletions
func (p *Processor) deleteConsecutiveExtractions(data any, segments []PathSegment, segmentIndex int) error {
	// Find all consecutive extraction segments
	extractionSegments := []PathSegment{}
	i := segmentIndex
	for i < len(segments) && segments[i].TypeString() == "extract" {
		extractionSegments = append(extractionSegments, segments[i])
		i++
	}

	remainingSegments := segments[i:]

	return p.processConsecutiveExtractionsForDeletion(data, extractionSegments, remainingSegments)
}

// processConsecutiveExtractionsForDeletion processes consecutive extractions for deletion
func (p *Processor) processConsecutiveExtractionsForDeletion(data any, extractionSegments []PathSegment, remainingSegments []PathSegment) error {
	if len(extractionSegments) == 0 {
		return nil
	}

	// Apply first extraction
	firstExtraction := extractionSegments[0]
	field := firstExtraction.Extract

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

// deleteDeepExtractedValues handles deep extraction deletion
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

// cleanupNullValues removes null values from data structures
func (p *Processor) cleanupNullValues(data any) {
	p.cleanupNullValuesRecursive(data)
}

// cleanupNullValuesRecursive recursively cleans up null values
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

// isEmptyContainer checks if a container is empty
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

// isContainer checks if data is a container type
func (p *Processor) isContainer(data any) bool {
	switch data.(type) {
	case map[string]any, map[any]any, []any:
		return true
	default:
		return false
	}
}

// cleanupNullValuesWithReconstruction cleans up nulls and returns new data
func (p *Processor) cleanupNullValuesWithReconstruction(data any, compactArrays bool) any {
	return p.cleanupNullValuesRecursiveWithReconstruction(data, compactArrays)
}

// cleanupNullValuesRecursiveWithReconstruction recursively cleans with reconstruction
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

// cleanupDeletedMarkers removes deleted markers from data
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
