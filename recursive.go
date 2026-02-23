package json

import (
	"errors"
	"fmt"
	"strings"

	"github.com/cybergodev/json/internal"
)

// RecursiveProcessor implements true recursive processing for all operations
type RecursiveProcessor struct {
	processor *Processor
}

// NewRecursiveProcessor creates a new unified recursive processor
func NewRecursiveProcessor(p *Processor) *RecursiveProcessor {
	return &RecursiveProcessor{
		processor: p,
	}
}

// ProcessRecursively performs recursive processing for any operation
func (urp *RecursiveProcessor) ProcessRecursively(data any, path string, operation Operation, value any) (any, error) {
	return urp.ProcessRecursivelyWithOptions(data, path, operation, value, false)
}

// ProcessRecursivelyWithOptions performs recursive processing with path creation options
func (urp *RecursiveProcessor) ProcessRecursivelyWithOptions(data any, path string, operation Operation, value any, createPaths bool) (any, error) {
	// Parse path into segments using cached parsing
	segments, err := urp.processor.getCachedPathSegments(path)
	if err != nil {
		return nil, fmt.Errorf("failed to parse path '%s': %w", path, err)
	}

	if len(segments) == 0 {
		switch operation {
		case OpGet:
			return data, nil
		case OpSet:
			return nil, fmt.Errorf("cannot set root value")
		case OpDelete:
			return nil, fmt.Errorf("cannot delete root value")
		}
	}

	// Start recursive processing from root
	result, err := urp.processRecursivelyAtSegmentsWithOptions(data, segments, 0, operation, value, createPaths)
	if err != nil {
		return nil, err
	}

	// Check if any segment in the path was a flat extraction
	// If so, we need special handling to apply flattening and subsequent operations correctly
	if operation == OpGet {
		// Find the LAST flat segment, not the first one
		// This is important for paths like orders{flat:items}{flat:tags}[0:3]
		flatSegmentIndex := -1
		for i, segment := range segments {
			if segment.Type == internal.ExtractSegment && segment.IsFlat {
				flatSegmentIndex = i // Keep updating to find the last one
			}
		}

		if flatSegmentIndex >= 0 {
			// Check if there are any operations after the flat extraction
			hasPostFlatOps := flatSegmentIndex+1 < len(segments)

			if hasPostFlatOps {
				// There are operations after flat extraction - need special handling
				// Process the path in two phases:
				// Phase 1: Process up to and including the flat segment
				// Phase 2: Apply flattening and then process remaining segments

				// Step 1: Process up to and including the flat segment
				preFlatSegments := segments[:flatSegmentIndex+1]
				preFlatResult, err := urp.processRecursivelyAtSegmentsWithOptions(data, preFlatSegments, 0, operation, value, createPaths)
				if err != nil {
					return nil, err
				}

				// Step 2: Apply flattening to the pre-flat result
				var flattened []any
				if resultArray, ok := preFlatResult.([]any); ok {
					urp.deepFlattenResults(resultArray, &flattened)
				} else {
					flattened = []any{preFlatResult}
				}

				// Step 3: Process remaining segments on the flattened result
				postFlatSegments := segments[flatSegmentIndex+1:]
				if len(postFlatSegments) > 0 {
					finalResult, err := urp.processRecursivelyAtSegmentsWithOptions(flattened, postFlatSegments, 0, operation, value, createPaths)
					if err != nil {
						return nil, err
					}
					return finalResult, nil
				}

				return flattened, nil
			}

			// No operations after flat extraction - the flat extraction should have been handled
			// during normal processing, so just return the result as-is
			return result, nil
		}
	}

	return result, nil
}

// processRecursivelyAtSegments recursively processes path segments for any operation
func (urp *RecursiveProcessor) processRecursivelyAtSegments(data any, segments []internal.PathSegment, segmentIndex int, operation Operation, value any) (any, error) {
	return urp.processRecursivelyAtSegmentsWithOptions(data, segments, segmentIndex, operation, value, false)
}

// processRecursivelyAtSegmentsWithOptions recursively processes path segments with path creation options
func (urp *RecursiveProcessor) processRecursivelyAtSegmentsWithOptions(data any, segments []internal.PathSegment, segmentIndex int, operation Operation, value any, createPaths bool) (any, error) {
	// Base case: no more segments to process
	if segmentIndex >= len(segments) {
		switch operation {
		case OpGet:
			return data, nil
		case OpSet:
			return nil, fmt.Errorf("cannot set value: no target segment")
		case OpDelete:
			return nil, fmt.Errorf("cannot delete value: no target segment")
		}
	}

	// Check for extract-then-slice pattern
	if segmentIndex < len(segments)-1 {
		currentSegment := segments[segmentIndex]
		nextSegment := segments[segmentIndex+1]

		// Special handling for {extract}[slice] pattern
		if currentSegment.Type == internal.ExtractSegment && nextSegment.Type == internal.ArraySliceSegment {
			return urp.handleExtractThenSlice(data, currentSegment, nextSegment, segments, segmentIndex, operation, value)
		}
	}

	currentSegment := segments[segmentIndex]
	isLastSegment := segmentIndex == len(segments)-1

	switch currentSegment.Type {
	case internal.PropertySegment:
		return urp.handlePropertySegmentUnified(data, currentSegment, segments, segmentIndex, isLastSegment, operation, value, createPaths)

	case internal.ArrayIndexSegment:
		return urp.handleArrayIndexSegmentUnified(data, currentSegment, segments, segmentIndex, isLastSegment, operation, value, createPaths)

	case internal.ArraySliceSegment:
		return urp.handleArraySliceSegmentUnified(data, currentSegment, segments, segmentIndex, isLastSegment, operation, value, createPaths)

	case internal.ExtractSegment:
		return urp.handleExtractSegmentUnified(data, currentSegment, segments, segmentIndex, isLastSegment, operation, value, createPaths)

	case internal.WildcardSegment:
		return urp.handleWildcardSegmentUnified(data, currentSegment, segments, segmentIndex, isLastSegment, operation, value, createPaths)

	default:
		return nil, fmt.Errorf("unsupported segment type: %v", currentSegment.Type)
	}
}

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

// handleExtractSegmentUnified handles extraction segments for all operations
func (urp *RecursiveProcessor) handleExtractSegmentUnified(data any, segment internal.PathSegment, segments []internal.PathSegment, segmentIndex int, isLastSegment bool, operation Operation, value any, createPaths bool) (any, error) {
	// Check for special flat extraction syntax - use the IsFlat flag from parsing
	isFlat := segment.IsFlat
	actualKey := segment.Key
	if isFlat {
		// The key should already be cleaned by the parser, but double-check
		actualKey = strings.TrimPrefix(actualKey, "flat:")
	}

	switch container := data.(type) {
	case []any:
		// Extract from each array element
		var results []any
		var errs []error

		for i, item := range container {
			if itemMap, ok := item.(map[string]any); ok {
				if isLastSegment {
					switch operation {
					case OpGet:
						if val, exists := itemMap[actualKey]; exists {
							if isFlat {
								// Flatten the result if it's an array
								if valArray, ok := val.([]any); ok {
									results = append(results, valArray...)
								} else {
									results = append(results, val)
								}
							} else {
								results = append(results, val)
							}
						}
					case OpSet:
						itemMap[actualKey] = value
					case OpDelete:
						delete(itemMap, actualKey)
					}
				} else {
					// For non-last segments, we need to handle array operations specially
					if extractedValue, exists := itemMap[actualKey]; exists {
						if operation == OpGet {
							// Check if the next segment is an array operation
							nextSegmentIndex := segmentIndex + 1
							if nextSegmentIndex < len(segments) && segments[nextSegmentIndex].Type == internal.ArrayIndexSegment {
								// For array operations following extraction, collect values first
								results = append(results, extractedValue)
							} else {
								// For non-array operations, process recursively
								result, err := urp.processRecursivelyAtSegmentsWithOptions(extractedValue, segments, segmentIndex+1, operation, value, createPaths)
								if err != nil {
									errs = append(errs, err)
									continue
								}
								if result != nil {
									results = append(results, result)
								}
							}
						} else if operation == OpDelete {
							// For Delete operations on extraction paths, check if this is the last extraction
							// followed by array/slice operation
							nextSegmentIndex := segmentIndex + 1
							isLastExtraction := true

							// Check if there are more extraction segments after this one
							for i := nextSegmentIndex; i < len(segments); i++ {
								if segments[i].Type == internal.ExtractSegment {
									isLastExtraction = false
									break
								}
							}

							if isLastExtraction && nextSegmentIndex < len(segments) {
								nextSegment := segments[nextSegmentIndex]
								if nextSegment.Type == internal.ArrayIndexSegment || nextSegment.Type == internal.ArraySliceSegment {
									// For delete operations like {tasks}[0], we need to check if the extracted value is an array
									// If it's an array, delete from the array; if it's a scalar, delete the field
									if _, isArray := extractedValue.([]any); isArray {
										// The extracted value is an array, apply the array operation to it
										_, err := urp.processRecursivelyAtSegmentsWithOptions(extractedValue, segments, segmentIndex+1, operation, value, createPaths)
										if err != nil {
											errs = append(errs, err)
											continue
										}
									} else {
										// The extracted value is a scalar, delete the field itself
										// This matches the expected behavior for scalar fields like {name}[0]
										delete(itemMap, actualKey)
									}
								} else {
									// For other delete operations, process recursively
									_, err := urp.processRecursivelyAtSegmentsWithOptions(extractedValue, segments, segmentIndex+1, operation, value, createPaths)
									if err != nil {
										errs = append(errs, err)
										continue
									}
								}
							} else {
								// For other delete operations, process recursively
								_, err := urp.processRecursivelyAtSegmentsWithOptions(extractedValue, segments, segmentIndex+1, operation, value, createPaths)
								if err != nil {
									errs = append(errs, err)
									continue
								}
							}
						} else {
							// For Set operations, always process recursively
							_, err := urp.processRecursivelyAtSegmentsWithOptions(extractedValue, segments, segmentIndex+1, operation, value, createPaths)
							if err != nil {
								errs = append(errs, err)
								continue
							}
							if operation == OpSet {
								container[i] = item // Item was modified in place
							}
						}
					}
				}
			}
		}

		if operation == OpGet {
			// If this is not the last segment and we have collected results for array operations
			if !isLastSegment && len(results) > 0 {
				nextSegmentIndex := segmentIndex + 1
				if nextSegmentIndex < len(segments) && segments[nextSegmentIndex].Type == internal.ArrayIndexSegment {
					// Process the collected results with the remaining segments
					result, err := urp.processRecursivelyAtSegmentsWithOptions(results, segments, nextSegmentIndex, operation, value, createPaths)
					if err != nil {
						return nil, err
					}

					// For distributed array operations, apply deep flattening to match expected behavior
					// This flattens nested arrays from distributed operations like {name}[0]
					if resultArray, ok := result.([]any); ok {
						// Check if the next segment is an array index operation (not slice)
						nextSegment := segments[nextSegmentIndex]
						if nextSegment.Type == internal.ArrayIndexSegment {
							// For array index operations, apply deep flattening
							flattened := urp.deepFlattenDistributedResults(resultArray)
							return flattened, nil
						}
					}
					return result, nil
				}
			}

			// Apply flattening if this was a flat extraction
			if isFlat && len(results) > 0 {
				var flattened []any
				urp.deepFlattenResults(results, &flattened)
				return flattened, nil
			}

			// For distributed operations that end with array index operations, apply deep flattening
			// This handles cases like {name}[0] where we want ["Alice", "David", "Frank"] not [["Alice", "David"], ["Frank"]]
			// Only apply this for paths that have multiple extraction segments followed by array operations
			if len(results) > 0 && len(segments) > 0 {
				lastSegment := segments[len(segments)-1]
				if lastSegment.Type == internal.ArrayIndexSegment {
					// Count extraction segments to determine if deep flattening is needed
					extractionCount := 0
					for _, seg := range segments {
						if seg.Type == internal.ExtractSegment {
							extractionCount++
						}
					}

					// Only apply deep flattening for multi-level extractions like {teams}{members}{name}[0]
					// Don't apply it for simple extractions like {name} which should preserve structure
					if extractionCount >= 3 {
						flattened := urp.deepFlattenDistributedResults(results)
						return flattened, nil
					}
				}
			}

			return results, nil
		}

		return nil, urp.combineErrors(errs)

	case map[string]any:
		if isLastSegment {
			switch operation {
			case OpGet:
				if val, exists := container[actualKey]; exists {
					if isFlat {
						// Flatten the result if it's an array
						if valArray, ok := val.([]any); ok {
							return valArray, nil // Return flattened array
						}
					}
					return val, nil
				}
				return nil, nil
			case OpSet:
				container[actualKey] = value
				return value, nil
			case OpDelete:
				delete(container, actualKey)
				return nil, nil
			}
		}

		// Recursively process extracted value
		if extractedValue, exists := container[actualKey]; exists {
			return urp.processRecursivelyAtSegmentsWithOptions(extractedValue, segments, segmentIndex+1, operation, value, createPaths)
		}

		return nil, nil

	default:
		if operation == OpGet {
			return nil, nil // Cannot extract from non-object/array
		}
		return nil, fmt.Errorf("cannot extract from type %T", data)
	}
}

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

// combineErrors combines multiple errors into a single error using modern Go 1.24+ patterns
func (urp *RecursiveProcessor) combineErrors(errs []error) error {
	if len(errs) == 0 {
		return nil
	}

	// Filter out nil errors
	var validErrors []error
	for _, err := range errs {
		if err != nil {
			validErrors = append(validErrors, err)
		}
	}

	if len(validErrors) == 0 {
		return nil
	}

	// Use errors.Join() for modern error composition (Go 1.20+)
	return errors.Join(validErrors...)
}

// findTargetArrayForDistributedOperation finds the actual target array for distributed operations
// This handles nested array structures that may result from extraction operations
func (urp *RecursiveProcessor) findTargetArrayForDistributedOperation(item any) []any {
	// If item is directly an array, return it
	if arr, ok := item.([]any); ok {
		// Check if this array contains only one element that is also an array
		// This handles the case where extraction creates nested structures like [[[members]]]
		if len(arr) == 1 {
			if nestedArr, ok := arr[0].([]any); ok {
				// Check if the nested array contains objects (actual data)
				// vs another level of nesting
				if len(nestedArr) > 0 {
					if _, ok := nestedArr[0].(map[string]any); ok {
						// This is the target array containing objects
						return nestedArr
					} else if _, ok := nestedArr[0].([]any); ok {
						// Another level of nesting, recurse
						return urp.findTargetArrayForDistributedOperation(nestedArr)
					} else {
						// This is the target array containing primitive values (like strings)
						return nestedArr
					}
				}
				// Return the nested array even if empty
				return nestedArr
			}
		}
		// Return the array as-is if it doesn't match the nested pattern
		return arr
	}

	// If item is not an array, return nil
	return nil
}

// deepFlattenDistributedResults performs deep flattening of distributed operation results
// This handles nested array structures like [["Alice", "David"], ["Frank"]] -> ["Alice", "David", "Frank"]
func (urp *RecursiveProcessor) deepFlattenDistributedResults(results []any) []any {
	var flattened []any

	for _, item := range results {
		if itemArray, ok := item.([]any); ok {
			// Recursively flatten nested arrays
			for _, nestedItem := range itemArray {
				if nestedArray, ok := nestedItem.([]any); ok {
					// Another level of nesting, flatten it
					flattened = append(flattened, nestedArray...)
				} else {
					// This is a leaf value, add it directly
					flattened = append(flattened, nestedItem)
				}
			}
		} else {
			// This is a leaf value, add it directly
			flattened = append(flattened, item)
		}
	}

	return flattened
}

// deepFlattenResults recursively flattens nested arrays into a single flat array
// This is used for flat: extraction syntax to completely flatten all nested structures
func (urp *RecursiveProcessor) deepFlattenResults(results []any, flattened *[]any) {
	for _, result := range results {
		if resultArray, ok := result.([]any); ok {
			// Recursively flatten nested arrays
			urp.deepFlattenResults(resultArray, flattened)
		} else {
			// Add non-array items directly
			*flattened = append(*flattened, result)
		}
	}
}

// shouldUseDistributedArrayOperation determines if an array operation should be distributed
// based on the actual data structure
func (urp *RecursiveProcessor) shouldUseDistributedArrayOperation(container []any) bool {
	// Distributed operations should ONLY be used for extraction results, not regular nested arrays
	// Regular nested arrays like [[1,2,3], [4,5,6]] should use normal array indexing
	// Extraction results have specific patterns that distinguish them from regular nested arrays

	// If the container is empty, no distributed operation needed
	if len(container) == 0 {
		return false
	}

	// CRITICAL: Do NOT use distributed operations for regular nested arrays
	// Only use distributed operations when we have clear extraction result patterns:
	// 1. Triple-nested arrays like [[[item1, item2]]] (extraction wrapping)
	// 2. Arrays where ALL elements are arrays AND they contain objects (extraction from array of objects)

	// Check for triple-nested pattern (extraction result wrapper)
	if len(container) == 1 {
		if arr, ok := container[0].([]any); ok {
			if len(arr) == 1 {
				if _, ok := arr[0].([]any); ok {
					// This is [[[...]]] pattern - extraction result
					return true
				}
			}
		}
	}

	// Check if ALL elements are arrays containing objects (extraction from array of objects)
	// This distinguishes extraction results like [{name}] from regular nested arrays like [[1,2,3]]
	allArrays := true
	hasObjects := false
	for _, item := range container {
		if arr, ok := item.([]any); ok {
			// Check if this array contains objects
			for _, elem := range arr {
				if _, isObj := elem.(map[string]any); isObj {
					hasObjects = true
					break
				}
			}
		} else {
			allArrays = false
			break
		}
	}

	// Only use distributed operation if ALL elements are arrays AND at least one contains objects
	// This prevents treating [[1,2,3], [4,5,6]] as an extraction result
	if allArrays && hasObjects {
		return true
	}

	// Default to normal indexing for regular nested arrays
	return false
}
