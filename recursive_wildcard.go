package json

import (
	"fmt"

	"github.com/cybergodev/json/internal"
)

// handleWildcardSegmentUnified handles wildcard segments for all operations
func (urp *RecursiveProcessor) handleWildcardSegmentUnified(data any, segment internal.PathSegment, segments []internal.PathSegment, segmentIndex int, isLastSegment bool, operation Operation, value any, createPaths bool) (any, error) {
	switch container := data.(type) {
	case []any:
		if isLastSegment {
			switch operation {
			case OpGet:
				return container, nil
			case OpSet:
				// Set value to all array elements
				for i := range container {
					container[i] = value
				}
				return value, nil
			case OpDelete:
				// Mark all array elements for deletion
				for i := range container {
					container[i] = DeletedMarker
				}
				return nil, nil
			}
		}

		// Recursively process all array elements
		var results []any
		var errs []error

		for i, item := range container {
			result, err := urp.processRecursivelyAtSegmentsWithOptions(item, segments, segmentIndex+1, operation, value, createPaths)
			if err != nil {
				errs = append(errs, err)
				continue
			}

			if operation == OpGet && result != nil {
				// Preserve structure - don't flatten unless explicitly requested
				results = append(results, result)
			} else if operation == OpSet {
				container[i] = item // Item was modified in place
			}
		}

		if operation == OpGet {
			return results, nil
		}

		return nil, urp.combineErrors(errs)

	case map[string]any:
		if isLastSegment {
			switch operation {
			case OpGet:
				var results []any
				for _, val := range container {
					results = append(results, val)
				}
				return results, nil
			case OpSet:
				// Set value to all map entries
				for key := range container {
					container[key] = value
				}
				return value, nil
			case OpDelete:
				// Delete all map entries
				for key := range container {
					delete(container, key)
				}
				return nil, nil
			}
		}

		// Recursively process all map values
		var results []any
		var errs []error

		for key, mapValue := range container {
			result, err := urp.processRecursivelyAtSegmentsWithOptions(mapValue, segments, segmentIndex+1, operation, value, createPaths)
			if err != nil {
				errs = append(errs, err)
				continue
			}

			if operation == OpGet && result != nil {
				// Preserve structure - don't flatten unless explicitly requested
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
			return nil, nil // Cannot wildcard non-container
		}
		return nil, fmt.Errorf("cannot apply wildcard to type %T", data)
	}
}
