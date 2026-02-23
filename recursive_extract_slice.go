package json

import (
	"fmt"

	"github.com/cybergodev/json/internal"
)

// handleExtractThenSlice handles the special case of {extract}[slice] pattern
func (urp *RecursiveProcessor) handleExtractThenSlice(data any, extractSegment, sliceSegment internal.PathSegment, segments []internal.PathSegment, segmentIndex int, operation Operation, value any) (any, error) {
	// For Delete operations on {extract}[slice] patterns, we need to apply the slice operation
	// to each extracted array individually, not to the collection of extracted results
	if operation == OpDelete {
		return urp.handleExtractThenSliceDelete(data, extractSegment, sliceSegment, segments, segmentIndex, value)
	}

	// For Get operations, use the original logic
	var extractedResults []any

	switch container := data.(type) {
	case []any:
		// Extract from each array element
		for _, item := range container {
			if itemMap, ok := item.(map[string]any); ok {
				if val, exists := itemMap[extractSegment.Key]; exists {
					extractedResults = append(extractedResults, val)
				}
			}
		}
	case map[string]any:
		// Extract from single object
		if val, exists := container[extractSegment.Key]; exists {
			extractedResults = append(extractedResults, val)
		}
	default:
		return nil, fmt.Errorf("cannot extract from type %T", data)
	}

	// Now apply the slice to the extracted results
	if len(extractedResults) > 0 {
		var startVal, endVal int
		if sliceSegment.Start != nil {
			startVal = *sliceSegment.Start
		} else {
			startVal = 0
		}
		if sliceSegment.End != nil {
			endVal = *sliceSegment.End
		} else {
			endVal = len(extractedResults)
		}

		start, end := internal.NormalizeSlice(startVal, endVal, len(extractedResults))

		// Check if this is the last operation (extract + slice)
		isLastOperation := segmentIndex+2 >= len(segments)

		if isLastOperation {
			// Final result: slice the extracted data
			if start >= len(extractedResults) || end <= 0 || start >= end {
				return []any{}, nil
			}
			return extractedResults[start:end], nil
		} else {
			// More segments to process: slice first, then continue processing
			if start >= len(extractedResults) || end <= 0 || start >= end {
				return []any{}, nil
			}

			slicedData := extractedResults[start:end]

			// Process remaining segments on each sliced element
			var results []any
			var errs []error

			for _, item := range slicedData {
				result, err := urp.processRecursivelyAtSegmentsWithOptions(item, segments, segmentIndex+2, operation, value, false)
				if err != nil {
					errs = append(errs, err)
					continue
				}

				if operation == OpGet && result != nil {
					results = append(results, result)
				}
			}

			if operation == OpGet {
				return results, nil
			}

			return nil, urp.combineErrors(errs)
		}
	}

	// No extraction results
	return []any{}, nil
}

// handleExtractThenSliceDelete handles Delete operations for {extract}[slice] patterns
func (urp *RecursiveProcessor) handleExtractThenSliceDelete(data any, extractSegment, sliceSegment internal.PathSegment, segments []internal.PathSegment, segmentIndex int, value any) (any, error) {
	switch container := data.(type) {
	case []any:
		// Apply slice deletion to each extracted array
		var errs []error
		for _, item := range container {
			if itemMap, ok := item.(map[string]any); ok {
				if extractedValue, exists := itemMap[extractSegment.Key]; exists {
					if extractedArray, isArray := extractedValue.([]any); isArray {
						// Apply slice deletion to this array
						err := urp.applySliceDeletion(extractedArray, sliceSegment)
						if err != nil {
							errs = append(errs, err)
							continue
						}
						// Update the array in the map
						itemMap[extractSegment.Key] = extractedArray
					}
				}
			}
		}
		return nil, urp.combineErrors(errs)
	case map[string]any:
		// Apply slice deletion to single extracted array
		if extractedValue, exists := container[extractSegment.Key]; exists {
			if extractedArray, isArray := extractedValue.([]any); isArray {
				err := urp.applySliceDeletion(extractedArray, sliceSegment)
				if err != nil {
					return nil, err
				}
				container[extractSegment.Key] = extractedArray
			}
		}
		return nil, nil
	default:
		return nil, fmt.Errorf("cannot extract from type %T", data)
	}
}

// applySliceDeletion applies slice deletion to an array
func (urp *RecursiveProcessor) applySliceDeletion(arr []any, sliceSegment internal.PathSegment) error {
	var startVal, endVal int
	if sliceSegment.Start != nil {
		startVal = *sliceSegment.Start
	} else {
		startVal = 0
	}
	if sliceSegment.End != nil {
		endVal = *sliceSegment.End
	} else {
		endVal = len(arr)
	}

	start, end := internal.NormalizeSlice(startVal, endVal, len(arr))

	// Mark elements in slice for deletion
	for i := start; i < end && i < len(arr); i++ {
		arr[i] = DeletedMarker
	}

	return nil
}
