package json

import (
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
