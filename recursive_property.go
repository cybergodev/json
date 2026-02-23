package json

import (
	"fmt"

	"github.com/cybergodev/json/internal"
)

// handlePropertySegmentUnified handles property access segments for all operations
func (urp *RecursiveProcessor) handlePropertySegmentUnified(data any, segment internal.PathSegment, segments []internal.PathSegment, segmentIndex int, isLastSegment bool, operation Operation, value any, createPaths bool) (any, error) {
	switch container := data.(type) {
	case map[string]any:
		if isLastSegment {
			switch operation {
			case OpGet:
				if val, exists := container[segment.Key]; exists {
					return val, nil
				}
				// Property doesn't exist - return ErrPathNotFound as documented
				return nil, ErrPathNotFound
			case OpSet:
				container[segment.Key] = value
				return value, nil
			case OpDelete:
				delete(container, segment.Key)
				return nil, nil
			}
		}

		// Recursively process next segment
		if nextValue, exists := container[segment.Key]; exists {
			return urp.processRecursivelyAtSegmentsWithOptions(nextValue, segments, segmentIndex+1, operation, value, createPaths)
		}

		// Handle path creation for Set operations
		if operation == OpSet && createPaths {
			// Create missing path segment
			nextSegment := segments[segmentIndex+1]
			var newContainer any

			switch nextSegment.Type {
			case internal.ArrayIndexSegment:
				// For array index, create array with sufficient size
				requiredSize := nextSegment.Index + 1
				if requiredSize < 0 {
					requiredSize = 1
				}
				newContainer = make([]any, requiredSize)
			case internal.ArraySliceSegment:
				// For array slice, create array with sufficient size based on slice end
				requiredSize := 0
				if nextSegment.End != nil {
					requiredSize = *nextSegment.End
				}
				if requiredSize <= 0 {
					requiredSize = 1
				}
				newContainer = make([]any, requiredSize)
			default:
				newContainer = make(map[string]any)
			}

			container[segment.Key] = newContainer
			return urp.processRecursivelyAtSegmentsWithOptions(newContainer, segments, segmentIndex+1, operation, value, createPaths)
		}

		// Path doesn't exist and we're not creating paths
		if operation == OpSet {
			return nil, fmt.Errorf("path not found: %s", segment.Key)
		}

		// For Get operation, return ErrPathNotFound as documented
		if operation == OpGet {
			return nil, ErrPathNotFound
		}

		return nil, nil // Property doesn't exist for Delete

	case []any:
		// Apply property access to each array element recursively
		var results []any
		var errs []error

		for i, item := range container {
			result, err := urp.handlePropertySegmentUnified(item, segment, segments, segmentIndex, isLastSegment, operation, value, createPaths)
			if err != nil {
				errs = append(errs, err)
				continue
			}

			if operation == OpGet && result != nil {
				results = append(results, result)
			} else if operation == OpSet {
				container[i] = item // Item was modified in place
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
		if operation == OpGet {
			return nil, nil // Cannot access property on non-object/array
		}
		return nil, fmt.Errorf("cannot access property '%s' on type %T", segment.Key, data)
	}
}
