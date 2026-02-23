package json

import (
	"fmt"

	"github.com/cybergodev/json/internal"
)

// handleArraySliceSegmentUnified handles array slice segments for all operations
func (urp *RecursiveProcessor) handleArraySliceSegmentUnified(data any, segment internal.PathSegment, segments []internal.PathSegment, segmentIndex int, isLastSegment bool, operation Operation, value any, createPaths bool) (any, error) {
	switch container := data.(type) {
	case []any:
		// Check if this should be a distributed operation
		shouldUseDistributed := urp.shouldUseDistributedArrayOperation(container)

		if shouldUseDistributed {
			// Distributed slice operation - apply slice to each array element
			var results []any
			var errs []error

			for _, item := range container {
				targetArray := urp.findTargetArrayForDistributedOperation(item)
				if targetArray == nil {
					continue // Skip non-array items
				}

				var startVal, endVal int
				if segment.Start != nil {
					startVal = *segment.Start
				} else {
					startVal = 0
				}
				if segment.End != nil {
					endVal = *segment.End
				} else {
					endVal = len(targetArray)
				}

				if isLastSegment {
					switch operation {
					case OpGet:
						// Use the array utils for proper slicing with step support
						startPtr := &startVal
						endPtr := &endVal
						if segment.Start == nil {
							startPtr = nil
						}
						if segment.End == nil {
							endPtr = nil
						}
						sliceResult := internal.PerformArraySlice(targetArray, startPtr, endPtr, segment.Step)
						results = append(results, sliceResult)
					case OpSet:
						// For distributed set operations on slices, we need special handling
						return nil, fmt.Errorf("distributed set operations on slices not yet supported")
					case OpDelete:
						// For distributed delete operations on slices, we need special handling
						return nil, fmt.Errorf("distributed delete operations on slices not yet supported")
					}
				} else {
					// Recursively process next segment on sliced result
					startPtr := &startVal
					endPtr := &endVal
					if segment.Start == nil {
						startPtr = nil
					}
					if segment.End == nil {
						endPtr = nil
					}
					sliceResult := internal.PerformArraySlice(targetArray, startPtr, endPtr, segment.Step)

					result, err := urp.processRecursivelyAtSegmentsWithOptions(sliceResult, segments, segmentIndex+1, operation, value, createPaths)
					if err != nil {
						errs = append(errs, err)
						continue
					}
					if operation == OpGet && result != nil {
						results = append(results, result)
					}
				}
			}

			if len(errs) > 0 {
				return nil, urp.combineErrors(errs)
			}

			if operation == OpGet {
				return results, nil
			}
			return nil, nil
		}

		// Non-distributed slice operation
		var startVal, endVal int
		if segment.Start != nil {
			startVal = *segment.Start
		} else {
			startVal = 0
		}
		if segment.End != nil {
			endVal = *segment.End
		} else {
			endVal = len(container)
		}

		start, end := internal.NormalizeSlice(startVal, endVal, len(container))

		if isLastSegment {
			switch operation {
			case OpGet:
				// Use the array utils for proper slicing with step support
				startPtr := &startVal
				endPtr := &endVal
				if segment.Start == nil {
					startPtr = nil
				}
				if segment.End == nil {
					endPtr = nil
				}
				return internal.PerformArraySlice(container, startPtr, endPtr, segment.Step), nil
			case OpSet:
				// Check if we need to extend the array for slice assignment
				if end > len(container) && createPaths {
					// For array slice extension, we need to fall back to legacy handling
					// because the unified processor can't modify parent references directly
					return nil, fmt.Errorf("array slice extension required: use legacy handling for path with slice [%d:%d] on array length %d", start, end, len(container))
				}

				// Set value to all elements in slice
				for i := start; i < end && i < len(container); i++ {
					container[i] = value
				}
				return value, nil
			case OpDelete:
				// Mark elements in slice for deletion
				for i := start; i < end && i < len(container); i++ {
					container[i] = DeletedMarker
				}
				return nil, nil
			}
		}

		// For non-last segments, we need to decide whether to:
		// 1. Apply slice first, then process remaining segments on each sliced element
		// 2. Process remaining segments on each element, then apply slice to results

		// The correct behavior depends on the context:
		// If this slice comes after an extraction, we should slice the extracted results
		// If this slice comes before further processing, we should slice first then process

		// Apply slice first, then process remaining segments
		slicedContainer := container[start:end]

		if len(slicedContainer) == 0 {
			return []any{}, nil
		}

		// Process remaining segments on each sliced element
		var results []any
		var errs []error

		for i, item := range slicedContainer {
			result, err := urp.processRecursivelyAtSegmentsWithOptions(item, segments, segmentIndex+1, operation, value, createPaths)
			if err != nil {
				errs = append(errs, err)
				continue
			}

			if operation == OpGet && result != nil {
				results = append(results, result)
			} else if operation == OpSet {
				slicedContainer[i] = item // Item was modified in place
			}
		}

		if operation == OpGet {
			return results, nil
		}

		return nil, urp.combineErrors(errs)

	case map[string]any:
		// Apply array slice to each map value recursively
		var results []any
		var errs []error

		for key, mapValue := range container {
			result, err := urp.handleArraySliceSegmentUnified(mapValue, segment, segments, segmentIndex, isLastSegment, operation, value, createPaths)
			if err != nil {
				errs = append(errs, err)
				continue
			}

			if operation == OpGet && result != nil {
				// Preserve structure for map values - don't flatten
				results = append(results, result)
			} else if operation == OpSet {
				container[key] = mapValue // Value was modified in place
			}
		}

		if operation == OpGet {
			return results, nil
		}

		return nil, urp.combineErrors(errs)

	default:
		if operation == OpGet {
			return nil, nil // Cannot slice non-array
		}
		return nil, fmt.Errorf("cannot slice type %T", data)
	}
}
