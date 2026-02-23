package json

import (
	"fmt"

	"github.com/cybergodev/json/internal"
)

// handleArrayIndexSegmentUnified handles array index access segments for all operations
func (urp *RecursiveProcessor) handleArrayIndexSegmentUnified(data any, segment internal.PathSegment, segments []internal.PathSegment, segmentIndex int, isLastSegment bool, operation Operation, value any, createPaths bool) (any, error) {
	switch container := data.(type) {
	case []any:
		// Determine if this should be a distributed operation based on actual data structure
		// A distributed operation is needed when we have nested arrays that need individual processing
		shouldUseDistributed := urp.shouldUseDistributedArrayOperation(container)

		if shouldUseDistributed {
			// For distributed operations, apply the index to each element in the container
			var results []any
			var errs []error

			for _, item := range container {
				// Find the actual target array for distributed operation
				targetArray := urp.findTargetArrayForDistributedOperation(item)
				if targetArray != nil {
					// Apply index operation to this array
					index := internal.NormalizeIndex(segment.Index, len(targetArray))
					if index < 0 || index >= len(targetArray) {
						if operation == OpGet {
							continue // Skip out of bounds items
						}
						errs = append(errs, fmt.Errorf("array index %d out of bounds (length %d)", segment.Index, len(targetArray)))
						continue
					}

					if isLastSegment {
						switch operation {
						case OpGet:
							// Get the result from the target array
							result := targetArray[index]

							// For distributed array operations, unwrap single element results for flattening
							// This mimics the behavior of the original getValueWithDistributedOperation
							if segment.Type != internal.ArraySliceSegment {
								// For index operations (not slice), add the result directly
								// This will be a single value like "Alice", not an array
								results = append(results, result)
							} else {
								// For slice operations, add the result as-is (could be an array)
								results = append(results, result)
							}
						case OpSet:
							targetArray[index] = value
						case OpDelete:
							targetArray[index] = DeletedMarker
						}
					} else {
						// Recursively process next segment
						result, err := urp.processRecursivelyAtSegmentsWithOptions(targetArray[index], segments, segmentIndex+1, operation, value, createPaths)
						if err != nil {
							errs = append(errs, err)
							continue
						}
						if operation == OpGet && result != nil {
							results = append(results, result)
						}
					}
				}
			}

			if operation == OpGet {
				// For distributed array operations, flatten the results to match expected behavior
				// This mimics the behavior of the original getValueWithDistributedOperation
				if isLastSegment && segment.Type != internal.ArraySliceSegment {
					// Return flattened results for distributed array index operations
					return results, nil
				}
				return results, nil
			}
			return nil, urp.combineErrors(errs)
		}

		// Non-distributed operation - standard array index access
		index := internal.NormalizeIndex(segment.Index, len(container))
		if index < 0 || index >= len(container) {
			if operation == OpGet {
				return nil, nil // Index out of bounds
			}
			if operation == OpSet && createPaths && index >= 0 {
				// Array extension required
				return nil, fmt.Errorf("array extension required for index %d on array length %d", index, len(container))
			}
			return nil, fmt.Errorf("array index %d out of bounds (length %d)", segment.Index, len(container))
		}

		if isLastSegment {
			switch operation {
			case OpGet:
				return container[index], nil
			case OpSet:
				container[index] = value
				return value, nil
			case OpDelete:
				// Mark for deletion (will be cleaned up later)
				container[index] = DeletedMarker
				return nil, nil
			}
		}

		// Recursively process next segment
		return urp.processRecursivelyAtSegmentsWithOptions(container[index], segments, segmentIndex+1, operation, value, createPaths)

	case map[string]any:
		// Apply array index to each map value recursively
		var results []any
		var errs []error

		for key, mapValue := range container {
			result, err := urp.handleArrayIndexSegmentUnified(mapValue, segment, segments, segmentIndex, isLastSegment, operation, value, createPaths)
			if err != nil {
				errs = append(errs, err)
				continue
			}

			if operation == OpGet && result != nil {
				results = append(results, result)
			} else if operation == OpSet {
				container[key] = mapValue // Value was modified in place
			}
		}

		if operation == OpGet {
			if len(results) == 0 {
				return nil, nil
			}
			return results, nil
		}

		return nil, urp.combineErrors(errs)

	default:
		// Cannot perform array index access on non-array types
		return nil, fmt.Errorf("cannot access array index [%d] on type %T", segment.Index, data)
	}
}
