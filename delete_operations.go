package json

import (
	"fmt"
	"strconv"
	"strings"
)

// deleteOperations implements the DeleteOperations interface
type deleteOperations struct {
	utils      ProcessorUtils
	pathParser PathParser
	navigator  Navigator
	arrayOps   ArrayOperations
}

// NewDeleteOperations creates a new delete operations instance
func NewDeleteOperations(utils ProcessorUtils, pathParser PathParser, navigator Navigator, arrayOps ArrayOperations) DeleteOperations {
	return &deleteOperations{
		utils:      utils,
		pathParser: pathParser,
		navigator:  navigator,
		arrayOps:   arrayOps,
	}
}

// DeleteValue deletes a value at the specified path
func (do *deleteOperations) DeleteValue(data any, path string) error {
	if path == "" || path == "." {
		return fmt.Errorf("cannot delete root value")
	}

	segments, err := do.pathParser.ParsePath(path)
	if err != nil {
		return fmt.Errorf("failed to parse path '%s': %w", path, err)
	}

	return do.DeleteValueWithSegments(data, segments)
}

// DeleteValueWithSegments deletes a value using parsed path segments
func (do *deleteOperations) DeleteValueWithSegments(data any, segments []PathSegmentInfo) error {
	if len(segments) == 0 {
		return fmt.Errorf("no segments to process")
	}

	return do.deleteValueComplexSegments(data, segments, 0)
}

// MarkForDeletion marks a value for deletion (using deletion marker)
func (do *deleteOperations) MarkForDeletion(data any, path string) error {
	segments, err := do.pathParser.ParsePath(path)
	if err != nil {
		return fmt.Errorf("failed to parse path '%s': %w", path, err)
	}

	return do.markSegmentsForDeletion(data, segments, 0)
}

// CleanupDeletedValues removes all deletion markers from the data structure
func (do *deleteOperations) CleanupDeletedValues(data any) any {
	return do.cleanupDeletedMarkersRecursive(data)
}

// CompactArray removes null values and compacts an array
func (do *deleteOperations) CompactArray(arr []any) []any {
	if len(arr) == 0 {
		return arr
	}

	var result []any
	for _, item := range arr {
		if item != nil && item != DeletedMarker {
			result = append(result, item)
		}
	}

	return result
}

// deleteValueComplexSegments handles deletion for complex path segments
func (do *deleteOperations) deleteValueComplexSegments(data any, segments []PathSegmentInfo, segmentIndex int) error {
	if segmentIndex >= len(segments) {
		return nil
	}

	segment := segments[segmentIndex]

	// Special handling for consecutive extraction segments
	if segment.Type == "extract" && do.hasConsecutiveExtractions(segments, segmentIndex) {
		return do.deleteConsecutiveExtractions(data, segments, segmentIndex)
	}

	// Handle different segment types
	switch segment.Type {
	case "property":
		return do.deleteComplexProperty(data, segment, segments, segmentIndex)
	case "array":
		return do.deleteComplexArray(data, segment, segments, segmentIndex)
	case "slice":
		return do.deleteComplexSlice(data, segment, segments, segmentIndex)
	case "extract":
		return do.deleteComplexExtract(data, segment, segments, segmentIndex)
	default:
		return fmt.Errorf("unsupported segment type for deletion: %s", segment.Type)
	}
}

// deleteComplexProperty handles property deletion in complex paths
func (do *deleteOperations) deleteComplexProperty(data any, segment PathSegmentInfo, segments []PathSegmentInfo, segmentIndex int) error {
	switch v := data.(type) {
	case map[string]any:
		if next, exists := v[segment.Key]; exists {
			if segmentIndex+1 >= len(segments) {
				// This is the final segment, delete the property
				delete(v, segment.Key)
				return nil
			}
			// Continue with the next segment
			return do.deleteValueComplexSegments(next, segments, segmentIndex+1)
		}
		// Property not found - ignore deletion
		return nil
	case map[any]any:
		if next, exists := v[segment.Key]; exists {
			if segmentIndex+1 >= len(segments) {
				delete(v, segment.Key)
				return nil
			}
			return do.deleteValueComplexSegments(next, segments, segmentIndex+1)
		}
		return nil
	case []any:
		// Handle numeric property on array (dot notation array access)
		if index := do.parseArrayIndex(segment.Key); index != -999999 {
			if index < 0 {
				index = len(v) + index
			}
			if index >= 0 && index < len(v) {
				if segmentIndex+1 >= len(segments) {
					v[index] = DeletedMarker
					return nil
				}
				if v[index] != nil {
					return do.deleteValueComplexSegments(v[index], segments, segmentIndex+1)
				}
			}
		}
		return nil
	default:
		// Cannot access property on this type - ignore deletion
		return nil
	}
}

// deleteComplexArray handles array deletion in complex paths
func (do *deleteOperations) deleteComplexArray(data any, segment PathSegmentInfo, segments []PathSegmentInfo, segmentIndex int) error {
	arr, ok := data.([]any)
	if !ok {
		// Not an array - ignore deletion
		return nil
	}

	index := segment.Index
	// Handle negative indices
	if index < 0 {
		index = len(arr) + index
	}

	// Check bounds after negative index conversion
	if index >= 0 && index < len(arr) {
		if segmentIndex+1 >= len(segments) {
			// This is the final segment, delete the array element
			arr[index] = DeletedMarker
			return nil
		}
		// Continue with the next segment
		if arr[index] != nil {
			return do.deleteValueComplexSegments(arr[index], segments, segmentIndex+1)
		}
	}
	// Index out of bounds - ignore deletion
	return nil
}

// deleteComplexSlice handles slice deletion in complex paths
func (do *deleteOperations) deleteComplexSlice(data any, segment PathSegmentInfo, segments []PathSegmentInfo, segmentIndex int) error {
	arr, ok := data.([]any)
	if !ok {
		// Not an array - ignore deletion
		return nil
	}

	start, end, step := do.parseSliceParameters(segment)
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
		end = length
	}

	// Process each element in the slice range
	for i := start; i < end; i += step {
		if i < len(arr) {
			if segmentIndex+1 >= len(segments) {
				// This is the final segment, delete the array element
				arr[i] = DeletedMarker
			} else {
				// Continue with the next segment
				if arr[i] != nil {
					err := do.deleteValueComplexSegments(arr[i], segments, segmentIndex+1)
					if err != nil {
						// Continue processing other elements even if one fails
						continue
					}
				}
			}
		}
	}
	return nil
}

// deleteComplexExtract handles extraction deletion in complex paths
func (do *deleteOperations) deleteComplexExtract(data any, segment PathSegmentInfo, segments []PathSegmentInfo, segmentIndex int) error {
	extractKey := segment.Extract
	if extractKey == "" {
		return fmt.Errorf("extract key is required for deletion")
	}

	switch v := data.(type) {
	case []any:
		// Delete the extracted property from all objects in the array
		for _, item := range v {
			if obj, ok := item.(map[string]any); ok {
				if segmentIndex+1 >= len(segments) {
					// This is the final segment, delete the property
					delete(obj, extractKey)
				} else {
					// Continue with the next segment
					if next, exists := obj[extractKey]; exists && next != nil {
						err := do.deleteValueComplexSegments(next, segments, segmentIndex+1)
						if err != nil {
							// Continue processing other objects even if one fails
							continue
						}
					}
				}
			} else if itemArr, ok := item.([]any); ok {
				// Handle nested arrays
				for _, nestedItem := range itemArr {
					if nestedObj, ok := nestedItem.(map[string]any); ok {
						if segmentIndex+1 >= len(segments) {
							delete(nestedObj, extractKey)
						} else {
							if next, exists := nestedObj[extractKey]; exists && next != nil {
								err := do.deleteValueComplexSegments(next, segments, segmentIndex+1)
								if err != nil {
									continue
								}
							}
						}
					}
				}
			}
		}
		return nil
	case map[string]any:
		if segmentIndex+1 >= len(segments) {
			// This is the final segment, delete the property
			delete(v, extractKey)
			return nil
		}
		// Continue with the next segment
		if next, exists := v[extractKey]; exists && next != nil {
			return do.deleteValueComplexSegments(next, segments, segmentIndex+1)
		}
		return nil
	default:
		// Cannot extract from this type - ignore deletion
		return nil
	}
}

// hasConsecutiveExtractions checks if there are consecutive extraction segments
func (do *deleteOperations) hasConsecutiveExtractions(segments []PathSegmentInfo, startIndex int) bool {
	if startIndex+1 >= len(segments) {
		return false
	}
	return segments[startIndex+1].Type == "extract"
}

// deleteConsecutiveExtractions handles deletion for consecutive extraction operations
func (do *deleteOperations) deleteConsecutiveExtractions(data any, segments []PathSegmentInfo, segmentIndex int) error {
	// Find all consecutive extraction segments
	extractionSegments := []PathSegmentInfo{}
	currentIndex := segmentIndex

	for currentIndex < len(segments) && segments[currentIndex].Type == "extract" {
		extractionSegments = append(extractionSegments, segments[currentIndex])
		currentIndex++
	}

	// Get remaining segments after all extractions
	remainingSegments := segments[currentIndex:]

	// Process the consecutive extractions
	return do.processConsecutiveExtractions(data, extractionSegments, remainingSegments, 0)
}

// processConsecutiveExtractions recursively processes multiple extraction levels
func (do *deleteOperations) processConsecutiveExtractions(data any, extractionSegments []PathSegmentInfo, remainingSegments []PathSegmentInfo, extractionIndex int) error {
	if extractionIndex >= len(extractionSegments) {
		// All extractions processed, handle remaining segments if any
		if len(remainingSegments) > 0 {
			return do.deleteValueComplexSegments(data, remainingSegments, 0)
		}
		return nil
	}

	currentExtraction := extractionSegments[extractionIndex]
	extractKey := currentExtraction.Extract

	if extractKey == "" {
		return nil
	}

	switch v := data.(type) {
	case []any:
		// Process each item in the array
		for _, item := range v {
			if obj, ok := item.(map[string]any); ok {
				if extractionIndex+1 >= len(extractionSegments) && len(remainingSegments) == 0 {
					// This is the final extraction and no remaining segments, delete the property
					delete(obj, extractKey)
				} else {
					// Continue with next extraction or remaining segments
					if next, exists := obj[extractKey]; exists && next != nil {
						err := do.processConsecutiveExtractions(next, extractionSegments, remainingSegments, extractionIndex+1)
						if err != nil {
							// Continue processing other objects even if one fails
							continue
						}
					}
				}
			} else if itemArr, ok := item.([]any); ok {
				// Handle nested arrays
				err := do.processConsecutiveExtractions(itemArr, extractionSegments, remainingSegments, extractionIndex)
				if err != nil {
					continue
				}
			}
		}
		return nil
	case map[string]any:
		if extractionIndex+1 >= len(extractionSegments) && len(remainingSegments) == 0 {
			// This is the final extraction and no remaining segments, delete the property
			delete(v, extractKey)
			return nil
		}
		// Continue with next extraction or remaining segments
		if next, exists := v[extractKey]; exists && next != nil {
			return do.processConsecutiveExtractions(next, extractionSegments, remainingSegments, extractionIndex+1)
		}
		return nil
	default:
		// Cannot extract from this type
		return nil
	}
}

// markSegmentsForDeletion marks segments for deletion using deletion markers
func (do *deleteOperations) markSegmentsForDeletion(data any, segments []PathSegmentInfo, segmentIndex int) error {
	if segmentIndex >= len(segments) {
		return nil
	}

	segment := segments[segmentIndex]

	switch segment.Type {
	case "property":
		return do.markPropertyForDeletion(data, segment, segments, segmentIndex)
	case "array":
		return do.markArrayForDeletion(data, segment, segments, segmentIndex)
	case "slice":
		return do.markSliceForDeletion(data, segment, segments, segmentIndex)
	case "extract":
		return do.markExtractForDeletion(data, segment, segments, segmentIndex)
	default:
		return fmt.Errorf("unsupported segment type for marking deletion: %s", segment.Type)
	}
}

// markPropertyForDeletion marks a property for deletion
func (do *deleteOperations) markPropertyForDeletion(data any, segment PathSegmentInfo, segments []PathSegmentInfo, segmentIndex int) error {
	switch v := data.(type) {
	case map[string]any:
		if segmentIndex+1 >= len(segments) {
			// Final segment, mark for deletion
			v[segment.Key] = DeletedMarker
			return nil
		}
		// Continue to next segment
		if next, exists := v[segment.Key]; exists && next != nil {
			return do.markSegmentsForDeletion(next, segments, segmentIndex+1)
		}
		return nil
	case []any:
		if index := do.parseArrayIndex(segment.Key); index != -999999 {
			if index < 0 {
				index = len(v) + index
			}
			if index >= 0 && index < len(v) {
				if segmentIndex+1 >= len(segments) {
					v[index] = DeletedMarker
					return nil
				}
				if v[index] != nil {
					return do.markSegmentsForDeletion(v[index], segments, segmentIndex+1)
				}
			}
		}
		return nil
	default:
		return nil
	}
}

// markArrayForDeletion marks an array element for deletion
func (do *deleteOperations) markArrayForDeletion(data any, segment PathSegmentInfo, segments []PathSegmentInfo, segmentIndex int) error {
	arr, ok := data.([]any)
	if !ok {
		return nil
	}

	index := segment.Index
	if index < 0 {
		index = len(arr) + index
	}

	if index >= 0 && index < len(arr) {
		if segmentIndex+1 >= len(segments) {
			arr[index] = DeletedMarker
			return nil
		}
		if arr[index] != nil {
			return do.markSegmentsForDeletion(arr[index], segments, segmentIndex+1)
		}
	}
	return nil
}

// markSliceForDeletion marks array slice elements for deletion
func (do *deleteOperations) markSliceForDeletion(data any, segment PathSegmentInfo, segments []PathSegmentInfo, segmentIndex int) error {
	arr, ok := data.([]any)
	if !ok {
		return nil
	}

	start, end, step := do.parseSliceParameters(segment)
	length := len(arr)

	if start < 0 {
		start = length + start
	}
	if end < 0 {
		end = length + end
	}
	if start < 0 {
		start = 0
	}
	if end > length {
		end = length
	}

	for i := start; i < end; i += step {
		if i < len(arr) {
			if segmentIndex+1 >= len(segments) {
				arr[i] = DeletedMarker
			} else if arr[i] != nil {
				do.markSegmentsForDeletion(arr[i], segments, segmentIndex+1)
			}
		}
	}
	return nil
}

// markExtractForDeletion marks extracted values for deletion
func (do *deleteOperations) markExtractForDeletion(data any, segment PathSegmentInfo, segments []PathSegmentInfo, segmentIndex int) error {
	extractKey := segment.Extract
	if extractKey == "" {
		return nil
	}

	switch v := data.(type) {
	case []any:
		for _, item := range v {
			if obj, ok := item.(map[string]any); ok {
				if segmentIndex+1 >= len(segments) {
					obj[extractKey] = DeletedMarker
				} else if next, exists := obj[extractKey]; exists && next != nil {
					do.markSegmentsForDeletion(next, segments, segmentIndex+1)
				}
			}
		}
		return nil
	case map[string]any:
		if segmentIndex+1 >= len(segments) {
			v[extractKey] = DeletedMarker
			return nil
		}
		if next, exists := v[extractKey]; exists && next != nil {
			return do.markSegmentsForDeletion(next, segments, segmentIndex+1)
		}
		return nil
	default:
		return nil
	}
}

// cleanupDeletedMarkersRecursive recursively removes deletion markers
func (do *deleteOperations) cleanupDeletedMarkersRecursive(data any) any {
	switch v := data.(type) {
	case map[string]any:
		result := make(map[string]any)
		for key, value := range v {
			if value != DeletedMarker {
				cleanedValue := do.cleanupDeletedMarkersRecursive(value)
				result[key] = cleanedValue
			}
		}
		return result
	case []any:
		var result []any
		for _, item := range v {
			if item != DeletedMarker {
				cleanedItem := do.cleanupDeletedMarkersRecursive(item)
				result = append(result, cleanedItem)
			}
		}
		return result
	default:
		return data
	}
}

// Helper functions

// parseArrayIndex parses an array index from a string
func (do *deleteOperations) parseArrayIndex(indexStr string) int {
	indexStr = strings.Trim(indexStr, "[]")
	if index, err := strconv.Atoi(indexStr); err == nil {
		return index
	}
	return -999999 // Invalid index marker
}

// parseSliceParameters parses slice parameters from a segment
func (do *deleteOperations) parseSliceParameters(segment PathSegmentInfo) (start, end, step int) {
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
