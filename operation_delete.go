package json

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/cybergodev/json/internal"
)

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

// navigateToParent navigates through data to the parent container of the final segment.
// Returns the parent container and the final segment key/index.
// Shared by deleteValueDotNotation and deleteValueJSONPointer to avoid duplicating
// the map[string]any/map[any]any/[]any switch logic.
func (p *Processor) navigateToParent(data any, segments []string) (any, string, error) {
	if len(segments) == 0 {
		return nil, "", fmt.Errorf("empty path")
	}

	current := data
	for i := 0; i < len(segments)-1; i++ {
		segment := segments[i]

		switch v := current.(type) {
		case map[string]any:
			if next, exists := v[segment]; exists {
				current = next
			} else {
				return nil, "", fmt.Errorf("path not found: %s", segment)
			}
		case map[any]any:
			if next, exists := v[segment]; exists {
				current = next
			} else {
				return nil, "", fmt.Errorf("path not found: %s", segment)
			}
		case []any:
			if index, ok := internal.ParseAndValidateArrayIndex(segment, len(v)); ok {
				current = v[index]
			} else {
				return nil, "", fmt.Errorf("invalid array index: %s", segment)
			}
		default:
			return nil, "", fmt.Errorf("cannot navigate through %T at segment %s", current, segment)
		}
	}

	return current, segments[len(segments)-1], nil
}

func (p *Processor) deleteValueDotNotation(data any, path string) error {
	segments, err := p.parsePath(path)
	if err != nil {
		return err
	}

	parent, finalSegment, err := p.navigateToParent(data, segments)
	if err != nil {
		return err
	}
	return p.deletePropertyValue(parent, finalSegment)
}

func (p *Processor) deleteValueJSONPointer(data any, path string) error {
	if path == "/" {
		return fmt.Errorf("cannot delete root")
	}

	// Split and unescape JSON Pointer segments upfront
	segments := strings.Split(path[1:], "/")
	for i := range segments {
		if strings.Contains(segments[i], "~") {
			segments[i] = internal.UnescapeJSONPointer(segments[i])
		}
	}

	parent, finalSegment, err := p.navigateToParent(data, segments)
	if err != nil {
		return err
	}
	return p.deletePropertyValue(parent, finalSegment)
}

func (p *Processor) deletePropertyValue(current any, property string) error {
	switch current.(type) {
	case []any:
		if _, err := strconv.Atoi(property); err == nil {
			return p.deleteArrayElement(current, property)
		}
		return fmt.Errorf("invalid array index: %s", property)
	default:
		return p.deletePropertyFromContainer(current, property)
	}
}

func (p *Processor) deleteValueComplexPath(data any, path string) error {
	// Parse path into segments
	segments := p.getPathSegments()
	defer p.putPathSegments(segments)

	*segments = p.splitPath(path, *segments)

	// Check if this requires complex deletion
	if p.hasComplexSegments(*segments) {
		return p.deleteValueComplexSegments(data, *segments, 0)
	}

	// Copy pooled segments to avoid holding backing array during recursion
	internalSegments := make([]internal.PathSegment, len(*segments))
	copy(internalSegments, *segments)

	return p.deleteValueWithInternalSegments(data, internalSegments)
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

func (p *Processor) navigateSegmentForDeletion(current any, segment internal.PathSegment) (any, error) {
	switch segment.Type {
	case internal.PropertySegment:
		return p.navigatePropertyForDeletion(current, segment.Key)
	case internal.ArrayIndexSegment:
		return p.navigateArrayIndexForDeletion(current, segment.String())
	case internal.ArraySliceSegment:
		// For slices, return the current container
		return current, nil
	case internal.ExtractSegment:
		// For extractions, return the current container
		return current, nil
	default:
		return nil, fmt.Errorf("unsupported segment type for deletion: %v", segment.Type.String())
	}
}

func (p *Processor) navigatePropertyForDeletion(current any, property string) (any, error) {
	if val, ok := containerGetProperty(current, property); ok {
		return val, nil
	}
	if containerIsMap(current) {
		return nil, fmt.Errorf("property not found: %s", property)
	}
	return nil, fmt.Errorf("cannot access property '%s' on type %T", property, current)
}

func (p *Processor) navigateArrayIndexForDeletion(current any, indexStr string) (any, error) {
	arr, ok := current.([]any)
	if !ok {
		return nil, fmt.Errorf("cannot access array index on type %T", current)
	}

	// PERFORMANCE: Use fastParseInt instead of strconv.Atoi to avoid error allocation overhead
	index, ok := internal.ParseArrayIndex(indexStr)
	if !ok {
		return nil, fmt.Errorf("invalid array index: %s", indexStr)
	}

	index, err := normalizeNegativeIndex(index, len(arr))
	if err != nil {
		return nil, err
	}

	return arr[index], nil
}

func (p *Processor) deleteValueForSegment(current any, segment internal.PathSegment) error {
	switch segment.Type {
	case internal.PropertySegment:
		return p.deletePropertyFromContainer(current, segment.Key)
	case internal.ArrayIndexSegment:
		return p.deleteArrayElementByIndex(current, segment.Index)
	case internal.ArraySliceSegment:
		return p.deleteArraySlice(current, segment)
	case internal.ExtractSegment:
		return p.deleteExtractedValues(current, segment)
	default:
		return fmt.Errorf("unsupported segment type for deletion: %v", segment.Type.String())
	}
}

func (p *Processor) deletePropertyFromContainer(current any, property string) error {
	if containerDeleteProperty(current, property) {
		return nil
	}
	if containerIsMap(current) {
		return fmt.Errorf("property not found: %s", property)
	}
	return fmt.Errorf("cannot delete property '%s' from type %T", property, current)
}

func (p *Processor) deleteArrayElement(current any, indexStr string) error {
	// PERFORMANCE: Use fastParseInt instead of strconv.Atoi
	index, ok := internal.ParseArrayIndex(indexStr)
	if !ok {
		return fmt.Errorf("invalid array index: %s", indexStr)
	}
	return p.deleteArrayElementByIndex(current, index)
}

func (p *Processor) deleteArrayElementByIndex(current any, index int) error {
	arr, ok := current.([]any)
	if !ok {
		return fmt.Errorf("cannot delete array element from type %T", current)
	}

	index, err := normalizeNegativeIndex(index, len(arr))
	if err != nil {
		return err
	}

	// Mark element for deletion (set to special marker)
	arr[index] = deletedMarker
	return nil
}

func (p *Processor) deleteArraySlice(current any, segment internal.PathSegment) error {
	arr, ok := current.([]any)
	if !ok {
		return fmt.Errorf("cannot delete slice from type %T", current)
	}

	// Parse slice parameters
	start, end, step, err := p.parseSliceParameters(segment.String(), len(arr))
	if err != nil {
		return err
	}

	// Normalize negative indices and validate bounds
	start, end, err = normalizeNegativeSliceBounds(start, end, len(arr))
	if err != nil {
		return err
	}

	// Mark elements for deletion
	for i := start; i < end; i += step {
		arr[i] = deletedMarker
	}

	return nil
}

func (p *Processor) deleteExtractedValues(current any, segment internal.PathSegment) error {
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

func (p *Processor) deleteValueComplexSegments(data any, segments []internal.PathSegment, segmentIndex int) error {
	if segmentIndex >= len(segments) {
		return nil
	}

	segment := segments[segmentIndex]

	switch segment.Type {
	case internal.PropertySegment:
		return p.deleteComplexProperty(data, segment, segments, segmentIndex)
	case internal.ArrayIndexSegment:
		return p.deleteComplexArray(data, segment, segments, segmentIndex)
	case internal.ArraySliceSegment:
		return p.deleteComplexSlice(data, segment, segments, segmentIndex)
	case internal.ExtractSegment:
		return p.deleteComplexExtract(data, segment, segments, segmentIndex)
	default:
		return fmt.Errorf("unsupported complex segment type: %v", segment.Type.String())
	}
}

func (p *Processor) deleteComplexProperty(data any, segment internal.PathSegment, segments []internal.PathSegment, segmentIndex int) error {
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

func (p *Processor) deleteComplexArray(data any, segment internal.PathSegment, segments []internal.PathSegment, segmentIndex int) error {
	arr, ok := data.([]any)
	if !ok {
		return fmt.Errorf("cannot access array on type %T", data)
	}

	index, err := normalizeNegativeIndex(segment.Index, len(arr))
	if err != nil {
		return err
	}

	if segmentIndex == len(segments)-1 {
		// Last segment, delete the array element
		arr[index] = deletedMarker
		return nil
	}

	// Navigate to next level
	return p.deleteValueComplexSegments(arr[index], segments, segmentIndex+1)
}

func (p *Processor) deleteComplexSlice(data any, segment internal.PathSegment, segments []internal.PathSegment, segmentIndex int) error {
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

	// Normalize negative indices and validate bounds
	start, end, err = normalizeNegativeSliceBounds(start, end, len(arr))
	if err != nil {
		return err
	}

	// Apply deletion to each element in the slice
	var firstErr error
	for i := start; i < end; i += step {
		if err := p.deleteValueComplexSegments(arr[i], segments, segmentIndex+1); err != nil {
			if firstErr == nil {
				firstErr = err
			}
			if !p.config.ContinueOnError {
				return firstErr
			}
		}
	}

	return firstErr
}

func (p *Processor) deleteComplexExtract(data any, segment internal.PathSegment, segments []internal.PathSegment, segmentIndex int) error {
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
		var errs []error
		for _, item := range arr {
			if obj, ok := item.(map[string]any); ok {
				if extractedValue, exists := obj[field]; exists {
					if err := p.deleteValueComplexSegments(extractedValue, segments, segmentIndex+1); err != nil {
						errs = append(errs, err)
					}
				}
			}
		}
		if len(errs) > 0 {
			return errors.Join(errs...)
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

func (p *Processor) hasConsecutiveExtractions(segments []internal.PathSegment, startIndex int) bool {
	if startIndex+1 >= len(segments) {
		return false
	}

	return segments[startIndex].Type == internal.ExtractSegment &&
		segments[startIndex+1].Type == internal.ExtractSegment
}

func (p *Processor) deleteConsecutiveExtractions(data any, segments []internal.PathSegment, segmentIndex int) error {
	// Find all consecutive extraction segments
	var extractionSegments []internal.PathSegment
	i := segmentIndex
	for i < len(segments) && segments[i].Type == internal.ExtractSegment {
		extractionSegments = append(extractionSegments, segments[i])
		i++
	}

	remainingSegments := segments[i:]

	return p.processConsecutiveExtractionsForDeletion(data, extractionSegments, remainingSegments)
}

func (p *Processor) processConsecutiveExtractionsForDeletion(data any, extractionSegments []internal.PathSegment, remainingSegments []internal.PathSegment) error {
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
							if err := p.deleteValueComplexSegments(extractedValue, remainingSegments, 0); err != nil {
								return err
							}
						}
					} else {
						// More extractions to process
						if err := p.processConsecutiveExtractionsForDeletion(extractedValue, extractionSegments[1:], remainingSegments); err != nil {
							return err
						}
					}
				}
			}
		}
	}

	return nil
}
