package json

import (
	"fmt"
)

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
