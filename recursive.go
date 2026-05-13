package json

import (
	"errors"
	"fmt"
	"strings"

	"github.com/cybergodev/json/internal"
)

// pathSegmentProvider is the minimal interface RecursiveProcessor needs.
// This interface breaks the circular dependency between Processor and RecursiveProcessor.
type pathSegmentProvider interface {
	getCachedPathSegments(path string) ([]internal.PathSegment, error)
}

// recursiveProcessor implements true recursive processing for all ops
type recursiveProcessor struct {
	provider pathSegmentProvider
}

// newRecursiveProcessor creates a new unified recursive processor.
// The provider must implement pathSegmentProvider interface.
// Note: *Processor implicitly implements this interface.
func newRecursiveProcessor(provider pathSegmentProvider) *recursiveProcessor {
	return &recursiveProcessor{
		provider: provider,
	}
}

// ProcessRecursively performs recursive processing for any op
func (urp *recursiveProcessor) ProcessRecursively(data any, path string, op operation, value any) (any, error) {
	return urp.ProcessRecursivelyWithOptions(data, path, op, value, false)
}

// ProcessRecursivelyWithOptions performs recursive processing with path creation options
func (urp *recursiveProcessor) ProcessRecursivelyWithOptions(data any, path string, op operation, value any, createPaths bool) (any, error) {
	// Parse path into segments using cached parsing
	segments, err := urp.provider.getCachedPathSegments(path)
	if err != nil {
		return nil, fmt.Errorf("failed to parse path '%s': %w", path, err)
	}

	if len(segments) == 0 {
		switch op {
		case opGet:
			return data, nil
		case opSet:
			return nil, fmt.Errorf("cannot set root value")
		case opDelete:
			return nil, fmt.Errorf("cannot delete root value")
		}
	}

	// Start recursive processing from root
	result, err := urp.processRecursivelyAtSegmentsWithOptions(data, segments, 0, op, value, createPaths)
	if err != nil {
		return nil, err
	}

	// Check if any segment in the path was a flat extraction
	// PERFORMANCE: Quick check — if no segment is an ExtractSegment at all,
	// skip the entire flat-handling block. This avoids iterating all segments
	// for the common case of simple property/array paths.
	if op == opGet {
		hasExtract := false
		for _, segment := range segments {
			if segment.Type == internal.ExtractSegment {
				hasExtract = true
				break
			}
		}
		if !hasExtract {
			return result, nil
		}

		// Find the LAST flat segment, not the first one
		// This is important for paths like orders{flat:items}{flat:tags}[0:3]
		flatSegmentIndex := -1
		for i, segment := range segments {
			if segment.Type == internal.ExtractSegment && segment.IsFlatExtract() {
				flatSegmentIndex = i // Keep updating to find the last one
			}
		}

		if flatSegmentIndex >= 0 {
			// Check if there are any ops after the flat extraction
			hasPostFlatOps := flatSegmentIndex+1 < len(segments)

			if hasPostFlatOps {
				// There are ops after flat extraction - need special handling
				// Process the path in two phases:
				// Phase 1: Process up to and including the flat segment
				// Phase 2: Apply flattening and then process remaining segments

				// Step 1: Process up to and including the flat segment
				preFlatSegments := segments[:flatSegmentIndex+1]
				preFlatResult, err := urp.processRecursivelyAtSegmentsWithOptions(data, preFlatSegments, 0, op, value, createPaths)
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
					finalResult, err := urp.processRecursivelyAtSegmentsWithOptions(flattened, postFlatSegments, 0, op, value, createPaths)
					if err != nil {
						return nil, err
					}
					return finalResult, nil
				}

				return flattened, nil
			}

			// No ops after flat extraction - the flat extraction should have been handled
			// during normal processing, so just return the result as-is
			return result, nil
		}
	}

	return result, nil
}

// processRecursivelyAtSegmentsWithOptions recursively processes path segments with path creation options
func (urp *recursiveProcessor) processRecursivelyAtSegmentsWithOptions(data any, segments []internal.PathSegment, segmentIndex int, op operation, value any, createPaths bool) (any, error) {
	// Base case: no more segments to process
	if segmentIndex >= len(segments) {
		switch op {
		case opGet:
			return data, nil
		case opSet:
			return nil, fmt.Errorf("cannot set value: no target segment")
		case opDelete:
			return nil, fmt.Errorf("cannot delete value: no target segment")
		}
	}

	// Check for extract-then-slice pattern
	if segmentIndex < len(segments)-1 {
		currentSegment := segments[segmentIndex]
		nextSegment := segments[segmentIndex+1]

		// Special handling for {extract}[slice] pattern
		if currentSegment.Type == internal.ExtractSegment && nextSegment.Type == internal.ArraySliceSegment {
			return urp.handleExtractThenSlice(data, currentSegment, nextSegment, segments, segmentIndex, op, value)
		}
	}

	currentSegment := segments[segmentIndex]
	isLastSegment := segmentIndex == len(segments)-1

	switch currentSegment.Type {
	case internal.PropertySegment:
		return urp.handlePropertySegmentUnified(data, currentSegment, segments, segmentIndex, isLastSegment, op, value, createPaths)

	case internal.ArrayIndexSegment:
		return urp.handleArrayIndexSegmentUnified(data, currentSegment, segments, segmentIndex, isLastSegment, op, value, createPaths)

	case internal.ArraySliceSegment:
		return urp.handleArraySliceSegmentUnified(data, currentSegment, segments, segmentIndex, isLastSegment, op, value, createPaths)

	case internal.ExtractSegment:
		return urp.handleExtractSegmentUnified(data, currentSegment, segments, segmentIndex, isLastSegment, op, value, createPaths)

	case internal.WildcardSegment:
		return urp.handleWildcardSegmentUnified(data, currentSegment, segments, segmentIndex, isLastSegment, op, value, createPaths)

	default:
		return nil, fmt.Errorf("unsupported segment type: %v", currentSegment.Type)
	}
}

// handleArrayIndexSegmentUnified handles array index access segments for all ops
func (urp *recursiveProcessor) handleArrayIndexSegmentUnified(data any, segment internal.PathSegment, segments []internal.PathSegment, segmentIndex int, isLastSegment bool, op operation, value any, createPaths bool) (any, error) {
	switch container := data.(type) {
	case []any:
		// Determine if this should be a distributed op based on actual data structure
		// A distributed op is needed when we have nested arrays that need individual processing
		shouldUseDistributed := urp.shouldUseDistributedArrayOp(container)

		if shouldUseDistributed {
			// For distributed ops, apply the index to each element in the container
			// PERFORMANCE: Pre-allocate slices with capacity hints
			results := make([]any, 0, len(container))
			errs := make([]error, 0, 4)

			for _, item := range container {
				// Find the actual target array for distributed op
				targetArray := urp.findTargetArrayForDistributedOp(item)
				if targetArray != nil {
					// Apply index op to this array
					index := internal.NormalizeIndex(segment.Index, len(targetArray))
					if index < 0 || index >= len(targetArray) {
						if op == opGet {
							continue // Skip out of bounds items
						}
						errs = append(errs, fmt.Errorf("array index %d out of bounds (length %d)", segment.Index, len(targetArray)))
						continue
					}

					if isLastSegment {
						switch op {
						case opGet:
							result := targetArray[index]
							results = append(results, result)
						case opSet:
							targetArray[index] = value
						case opDelete:
							targetArray[index] = deletedMarker
						}
					} else {
						// Recursively process next segment
						result, err := urp.processRecursivelyAtSegmentsWithOptions(targetArray[index], segments, segmentIndex+1, op, value, createPaths)
						if err != nil {
							errs = append(errs, err)
							continue
						}
						if op == opGet && result != nil {
							results = append(results, result)
						}
					}
				}
			}

			if op == opGet {
				// For distributed array ops, flatten the results to match expected behavior
				// This mimics the behavior of the original getValueWithDistributedOp
				if isLastSegment && segment.Type != internal.ArraySliceSegment {
					// Return flattened results for distributed array index ops
					return results, nil
				}
				return results, nil
			}
			return nil, urp.combineErrors(errs)
		}

		// Non-distributed op - standard array index access
		index := internal.NormalizeIndex(segment.Index, len(container))
		if index < 0 || index >= len(container) {
			if op == opGet {
				return nil, nil // Index out of bounds
			}
			if op == opSet && createPaths && index >= 0 {
				// Array extension required
				return nil, fmt.Errorf("array extension required for index %d on array length %d", index, len(container))
			}
			return nil, fmt.Errorf("array index %d out of bounds (length %d)", segment.Index, len(container))
		}

		if isLastSegment {
			switch op {
			case opGet:
				return container[index], nil
			case opSet:
				container[index] = value
				return value, nil
			case opDelete:
				// Mark for deletion (will be cleaned up later)
				container[index] = deletedMarker
				return nil, nil
			}
		}

		// Recursively process next segment
		return urp.processRecursivelyAtSegmentsWithOptions(container[index], segments, segmentIndex+1, op, value, createPaths)

	case map[string]any:
		// Apply array index to each map value recursively
		// PERFORMANCE: Pre-allocate slices with capacity hints
		results := make([]any, 0, len(container))
		errs := make([]error, 0, 4)

		for key, mapValue := range container {
			result, err := urp.handleArrayIndexSegmentUnified(mapValue, segment, segments, segmentIndex, isLastSegment, op, value, createPaths)
			if err != nil {
				errs = append(errs, err)
				continue
			}

			if op == opGet && result != nil {
				results = append(results, result)
			} else if op == opSet {
				container[key] = mapValue // Value was modified in place
			}
		}

		if op == opGet {
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

// handlePropertySegmentUnified handles property access segments for all ops
func (urp *recursiveProcessor) handlePropertySegmentUnified(data any, segment internal.PathSegment, segments []internal.PathSegment, segmentIndex int, isLastSegment bool, op operation, value any, createPaths bool) (any, error) {
	switch container := data.(type) {
	case map[string]any:
		if isLastSegment {
			switch op {
			case opGet:
				if val, exists := container[segment.Key]; exists {
					return val, nil
				}
				// Property doesn't exist - return ErrPathNotFound as documented
				return nil, ErrPathNotFound
			case opSet:
				container[segment.Key] = value
				return value, nil
			case opDelete:
				delete(container, segment.Key)
				return nil, nil
			}
		}

		// Recursively process next segment
		if nextValue, exists := container[segment.Key]; exists {
			return urp.processRecursivelyAtSegmentsWithOptions(nextValue, segments, segmentIndex+1, op, value, createPaths)
		}

		// Handle path creation for Set ops
		if op == opSet && createPaths {
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
				if nextSegment.HasEnd() {
					requiredSize = nextSegment.End
				}
				if requiredSize <= 0 {
					requiredSize = 1
				}
				newContainer = make([]any, requiredSize)
			default:
				newContainer = make(map[string]any)
			}

			container[segment.Key] = newContainer
			return urp.processRecursivelyAtSegmentsWithOptions(newContainer, segments, segmentIndex+1, op, value, createPaths)
		}

		// Path doesn't exist and we're not creating paths
		if op == opSet {
			return nil, fmt.Errorf("path not found: %s", segment.Key)
		}

		// For Get op, return ErrPathNotFound as documented
		if op == opGet {
			return nil, ErrPathNotFound
		}

		return nil, nil // Property doesn't exist for Delete

	case []any:
		// Apply property access to each array element recursively
		// PERFORMANCE: Pre-allocate slices with capacity hints
		results := make([]any, 0, len(container))
		errs := make([]error, 0, 4)

		for i, item := range container {
			result, err := urp.handlePropertySegmentUnified(item, segment, segments, segmentIndex, isLastSegment, op, value, createPaths)
			if err != nil {
				errs = append(errs, err)
				continue
			}

			if op == opGet && result != nil {
				results = append(results, result)
			} else if op == opSet {
				container[i] = item // Item was modified in place
			}
		}

		if op == opGet {
			if len(results) == 0 {
				return nil, nil
			}
			if len(results) == 1 {
				return results[0], nil
			}
			return results, nil
		}

		return nil, urp.combineErrors(errs)

	default:
		if op == opGet {
			return nil, nil // Cannot access property on non-object/array
		}
		return nil, fmt.Errorf("cannot access property '%s' on type %T", segment.Key, data)
	}
}

// handleArraySliceSegmentUnified handles array slice segments for all ops
func (urp *recursiveProcessor) handleArraySliceSegmentUnified(data any, segment internal.PathSegment, segments []internal.PathSegment, segmentIndex int, isLastSegment bool, op operation, value any, createPaths bool) (any, error) {
	switch container := data.(type) {
	case []any:
		// Check if this should be a distributed op
		shouldUseDistributed := urp.shouldUseDistributedArrayOp(container)

		if shouldUseDistributed {
			// Distributed slice op - apply slice to each array element
			// PERFORMANCE: Pre-allocate slices with capacity hints
			results := make([]any, 0, len(container))
			errs := make([]error, 0, 4)

			for _, item := range container {
				targetArray := urp.findTargetArrayForDistributedOp(item)
				if targetArray == nil {
					continue // Skip non-array items
				}

				var startVal, endVal int
				if segment.HasStart() {
					startVal = segment.Index // Index stores start for slices
				} else {
					startVal = 0
				}
				if segment.HasEnd() {
					endVal = segment.End
				} else {
					endVal = len(targetArray)
				}

				if isLastSegment {
					switch op {
					case opGet:
						// Use the array utils for proper slicing with step support
						var startPtr, endPtr, stepPtr *int
						if segment.HasStart() {
							startPtr = &startVal
						}
						if segment.HasEnd() {
							endPtr = &endVal
						}
						if segment.HasStep() {
							stepVal := segment.Step
							stepPtr = &stepVal
						}
						sliceResult := internal.PerformArraySlice(targetArray, startPtr, endPtr, stepPtr)
						results = append(results, sliceResult)
					case opSet:
						// For distributed set ops on slices, we need special handling
						return nil, fmt.Errorf("distributed set ops on slices not yet supported")
					case opDelete:
						// For distributed delete ops on slices, we need special handling
						return nil, fmt.Errorf("distributed delete ops on slices not yet supported")
					}
				} else {
					// Recursively process next segment on sliced result
					var startPtr, endPtr, stepPtr *int
					if segment.HasStart() {
						startPtr = &startVal
					}
					if segment.HasEnd() {
						endPtr = &endVal
					}
					if segment.HasStep() {
						stepVal := segment.Step
						stepPtr = &stepVal
					}
					sliceResult := internal.PerformArraySlice(targetArray, startPtr, endPtr, stepPtr)

					result, err := urp.processRecursivelyAtSegmentsWithOptions(sliceResult, segments, segmentIndex+1, op, value, createPaths)
					if err != nil {
						errs = append(errs, err)
						continue
					}
					if op == opGet && result != nil {
						results = append(results, result)
					}
				}
			}

			if len(errs) > 0 {
				return nil, urp.combineErrors(errs)
			}

			if op == opGet {
				return results, nil
			}
			return nil, nil
		}

		// Non-distributed slice op
		var startVal, endVal int
		if segment.HasStart() {
			startVal = segment.Index // Index stores start for slices
		} else {
			startVal = 0
		}
		if segment.HasEnd() {
			endVal = segment.End
		} else {
			endVal = len(container)
		}

		start, end := internal.NormalizeSlice(startVal, endVal, len(container))

		if isLastSegment {
			switch op {
			case opGet:
				// Use the array utils for proper slicing with step support
				var startPtr, endPtr, stepPtr *int
				if segment.HasStart() {
					startPtr = &startVal
				}
				if segment.HasEnd() {
					endPtr = &endVal
				}
				if segment.HasStep() {
					stepVal := segment.Step
					stepPtr = &stepVal
				}
				return internal.PerformArraySlice(container, startPtr, endPtr, stepPtr), nil
			case opSet:
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
			case opDelete:
				// Mark elements in slice for deletion
				for i := start; i < end && i < len(container); i++ {
					container[i] = deletedMarker
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
		// PERFORMANCE: Pre-allocate slices with capacity hints
		results := make([]any, 0, len(slicedContainer))
		errs := make([]error, 0, 4)

		for i, item := range slicedContainer {
			result, err := urp.processRecursivelyAtSegmentsWithOptions(item, segments, segmentIndex+1, op, value, createPaths)
			if err != nil {
				errs = append(errs, err)
				continue
			}

			if op == opGet && result != nil {
				results = append(results, result)
			} else if op == opSet {
				slicedContainer[i] = item // Item was modified in place
			}
		}

		if op == opGet {
			return results, nil
		}

		return nil, urp.combineErrors(errs)

	case map[string]any:
		// Apply array slice to each map value recursively
		// PERFORMANCE: Pre-allocate slices with capacity hints
		results := make([]any, 0, len(container))
		errs := make([]error, 0, 4)

		for key, mapValue := range container {
			result, err := urp.handleArraySliceSegmentUnified(mapValue, segment, segments, segmentIndex, isLastSegment, op, value, createPaths)
			if err != nil {
				errs = append(errs, err)
				continue
			}

			if op == opGet && result != nil {
				// Preserve structure for map values - don't flatten
				results = append(results, result)
			} else if op == opSet {
				container[key] = mapValue // Value was modified in place
			}
		}

		if op == opGet {
			return results, nil
		}

		return nil, urp.combineErrors(errs)

	default:
		if op == opGet {
			return nil, nil // Cannot slice non-array
		}
		return nil, fmt.Errorf("cannot slice type %T", data)
	}
}

// handleExtractSegmentUnified handles extraction segments for all ops
func (urp *recursiveProcessor) handleExtractSegmentUnified(data any, segment internal.PathSegment, segments []internal.PathSegment, segmentIndex int, isLastSegment bool, op operation, value any, createPaths bool) (any, error) {
	// Check for special flat extraction syntax - use the IsFlat flag from parsing
	isFlat := segment.IsFlatExtract()
	actualKey := segment.Key
	if isFlat {
		// The key should already be cleaned by the parser, but double-check
		actualKey = strings.TrimPrefix(actualKey, "flat:")
	}

	// Check for multi-field extraction (comma-separated fields)
	if strings.Contains(actualKey, ",") {
		return urp.handleMultiFieldExtractSegment(data, actualKey, isFlat, segments, segmentIndex, isLastSegment, op, value, createPaths)
	}

	switch container := data.(type) {
	case []any:
		// Extract from each array element
		// PERFORMANCE: Pre-allocate slices with capacity hints
		results := make([]any, 0, len(container))
		errs := make([]error, 0, 4)

		for i, item := range container {
			if itemMap, ok := item.(map[string]any); ok {
				if isLastSegment {
					switch op {
					case opGet:
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
					case opSet:
						itemMap[actualKey] = value
					case opDelete:
						delete(itemMap, actualKey)
					}
				} else {
					// For non-last segments, we need to handle array ops specially
					if extractedValue, exists := itemMap[actualKey]; exists {
						if op == opGet {
							// Check if the next segment is an array op
							nextSegmentIndex := segmentIndex + 1
							if nextSegmentIndex < len(segments) && segments[nextSegmentIndex].Type == internal.ArrayIndexSegment {
								// For array ops following extraction, collect values first
								results = append(results, extractedValue)
							} else {
								// For non-array ops, process recursively
								result, err := urp.processRecursivelyAtSegmentsWithOptions(extractedValue, segments, segmentIndex+1, op, value, createPaths)
								if err != nil {
									errs = append(errs, err)
									continue
								}
								if result != nil {
									results = append(results, result)
								}
							}
						} else if op == opDelete {
							// For Delete ops on extraction paths, check if this is the last extraction
							// followed by array/slice op
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
									// For delete ops like {tasks}[0], we need to check if the extracted value is an array
									// If it's an array, delete from the array; if it's a scalar, delete the field
									if _, isArray := extractedValue.([]any); isArray {
										// The extracted value is an array, apply the array op to it
										_, err := urp.processRecursivelyAtSegmentsWithOptions(extractedValue, segments, segmentIndex+1, op, value, createPaths)
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
									// For other delete ops, process recursively
									_, err := urp.processRecursivelyAtSegmentsWithOptions(extractedValue, segments, segmentIndex+1, op, value, createPaths)
									if err != nil {
										errs = append(errs, err)
										continue
									}
								}
							} else {
								// For other delete ops, process recursively
								_, err := urp.processRecursivelyAtSegmentsWithOptions(extractedValue, segments, segmentIndex+1, op, value, createPaths)
								if err != nil {
									errs = append(errs, err)
									continue
								}
							}
						} else {
							// For Set ops, always process recursively
							_, err := urp.processRecursivelyAtSegmentsWithOptions(extractedValue, segments, segmentIndex+1, op, value, createPaths)
							if err != nil {
								errs = append(errs, err)
								continue
							}
							if op == opSet {
								container[i] = item // Item was modified in place
							}
						}
					}
				}
			}
		}

		if op == opGet {
			// If this is not the last segment and we have collected results for array ops
			if !isLastSegment && len(results) > 0 {
				nextSegmentIndex := segmentIndex + 1
				if nextSegmentIndex < len(segments) && segments[nextSegmentIndex].Type == internal.ArrayIndexSegment {
					// Process the collected results with the remaining segments
					result, err := urp.processRecursivelyAtSegmentsWithOptions(results, segments, nextSegmentIndex, op, value, createPaths)
					if err != nil {
						return nil, err
					}

					// For distributed array ops, apply deep flattening to match expected behavior
					// This flattens nested arrays from distributed ops like {name}[0]
					if resultArray, ok := result.([]any); ok {
						// Check if the next segment is an array index op (not slice)
						nextSegment := segments[nextSegmentIndex]
						if nextSegment.Type == internal.ArrayIndexSegment {
							// For array index ops, apply deep flattening
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

			// For distributed ops that end with array index ops, apply deep flattening
			// This handles cases like {name}[0] where we want ["Alice", "David", "Frank"] not [["Alice", "David"], ["Frank"]]
			// Only apply this for paths that have multiple extraction segments followed by array ops
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
			switch op {
			case opGet:
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
			case opSet:
				container[actualKey] = value
				return value, nil
			case opDelete:
				delete(container, actualKey)
				return nil, nil
			}
		}

		// Recursively process extracted value
		if extractedValue, exists := container[actualKey]; exists {
			return urp.processRecursivelyAtSegmentsWithOptions(extractedValue, segments, segmentIndex+1, op, value, createPaths)
		}

		return nil, nil

	default:
		if op == opGet {
			return nil, nil // Cannot extract from non-object/array
		}
		return nil, fmt.Errorf("cannot extract from type %T", data)
	}
}

// handleMultiFieldExtractSegment handles multi-field extraction (e.g., {id,name})
// Returns a new object (or array of objects) containing only the specified fields.
// Note: isFlat is unused for multi-field extraction as flattening doesn't apply when
// extracting multiple fields into an object (only applicable to single-field extraction).
func (urp *recursiveProcessor) handleMultiFieldExtractSegment(data any, fieldsStr string, _ bool, segments []internal.PathSegment, segmentIndex int, isLastSegment bool, op operation, value any, createPaths bool) (any, error) {
	fields := strings.Split(fieldsStr, ",")

	// Trim whitespace from field names
	for i, f := range fields {
		fields[i] = strings.TrimSpace(f)
	}

	switch container := data.(type) {
	case []any:
		// Extract from each array element
		results := make([]any, 0, len(container))

		for _, item := range container {
			if itemMap, ok := item.(map[string]any); ok {
				extracted := urp.extractMultipleFieldsFromMap(itemMap, fields)
				if extracted != nil {
					results = append(results, extracted)
				}
			}
		}

		// If not last segment, continue processing
		if !isLastSegment && len(results) > 0 {
			return urp.processRecursivelyAtSegmentsWithOptions(results, segments, segmentIndex+1, op, value, createPaths)
		}

		return results, nil

	case map[string]any:
		extracted := urp.extractMultipleFieldsFromMap(container, fields)
		if extracted == nil {
			return nil, nil
		}

		// If not last segment, continue processing
		if !isLastSegment {
			return urp.processRecursivelyAtSegmentsWithOptions(extracted, segments, segmentIndex+1, op, value, createPaths)
		}

		return extracted, nil

	default:
		if op == opGet {
			return nil, nil // Cannot extract from non-object/array
		}
		return nil, fmt.Errorf("cannot extract from type %T", data)
	}
}

// extractMultipleFieldsFromMap extracts specified fields from a map
// Returns a new map containing only the specified fields that exist in the source
func (urp *recursiveProcessor) extractMultipleFieldsFromMap(source map[string]any, fields []string) map[string]any {
	result := make(map[string]any, len(fields))

	for _, field := range fields {
		if field == "" {
			continue
		}
		if value, exists := source[field]; exists {
			result[field] = value
		}
	}

	if len(result) == 0 {
		return nil
	}
	return result
}

// handleExtractThenSlice handles the special case of {extract}[slice] pattern
func (urp *recursiveProcessor) handleExtractThenSlice(data any, extractSegment, sliceSegment internal.PathSegment, segments []internal.PathSegment, segmentIndex int, op operation, value any) (any, error) {
	// For Delete ops on {extract}[slice] patterns, we need to apply the slice op
	// to each extracted array individually, not to the collection of extracted results
	if op == opDelete {
		return urp.handleExtractThenSliceDelete(data, extractSegment, sliceSegment, segments, segmentIndex, value)
	}

	// For Get ops, use the original logic
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
		if sliceSegment.HasStart() {
			startVal = sliceSegment.Index // Index stores start for slices
		} else {
			startVal = 0
		}
		if sliceSegment.HasEnd() {
			endVal = sliceSegment.End
		} else {
			endVal = len(extractedResults)
		}

		start, end := internal.NormalizeSlice(startVal, endVal, len(extractedResults))

		// Check if this is the last op (extract + slice)
		isLastop := segmentIndex+2 >= len(segments)

		if isLastop {
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
				result, err := urp.processRecursivelyAtSegmentsWithOptions(item, segments, segmentIndex+2, op, value, false)
				if err != nil {
					errs = append(errs, err)
					continue
				}

				if op == opGet && result != nil {
					results = append(results, result)
				}
			}

			if op == opGet {
				return results, nil
			}

			return nil, urp.combineErrors(errs)
		}
	}

	// No extraction results
	return []any{}, nil
}

// handleExtractThenSliceDelete handles Delete ops for {extract}[slice] patterns
// Note: segments, segmentIndex, and value are unused but kept for API consistency with other handlers.
func (urp *recursiveProcessor) handleExtractThenSliceDelete(data any, extractSegment, sliceSegment internal.PathSegment, _ []internal.PathSegment, _ int, _ any) (any, error) {
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
func (urp *recursiveProcessor) applySliceDeletion(arr []any, sliceSegment internal.PathSegment) error {
	var startVal, endVal int
	if sliceSegment.HasStart() {
		startVal = sliceSegment.Index // Index stores start for slices
	} else {
		startVal = 0
	}
	if sliceSegment.HasEnd() {
		endVal = sliceSegment.End
	} else {
		endVal = len(arr)
	}

	start, end := internal.NormalizeSlice(startVal, endVal, len(arr))

	// Mark elements in slice for deletion
	for i := start; i < end && i < len(arr); i++ {
		arr[i] = deletedMarker
	}

	return nil
}

// handleWildcardSegmentUnified handles wildcard segments for all ops.
// Note: segment parameter is unused as wildcard operations don't need segment-specific data.
func (urp *recursiveProcessor) handleWildcardSegmentUnified(data any, _ internal.PathSegment, segments []internal.PathSegment, segmentIndex int, isLastSegment bool, op operation, value any, createPaths bool) (any, error) {
	switch container := data.(type) {
	case []any:
		if isLastSegment {
			switch op {
			case opGet:
				return container, nil
			case opSet:
				// Set value to all array elements
				for i := range container {
					container[i] = value
				}
				return value, nil
			case opDelete:
				// Mark all array elements for deletion
				for i := range container {
					container[i] = deletedMarker
				}
				return nil, nil
			}
		}

		// Recursively process all array elements
		// PERFORMANCE: Pre-allocate slices with capacity hints
		results := make([]any, 0, len(container))
		errs := make([]error, 0, 4)

		for i, item := range container {
			result, err := urp.processRecursivelyAtSegmentsWithOptions(item, segments, segmentIndex+1, op, value, createPaths)
			if err != nil {
				errs = append(errs, err)
				continue
			}

			if op == opGet && result != nil {
				// Preserve structure - don't flatten unless explicitly requested
				results = append(results, result)
			} else if op == opSet {
				container[i] = item // Item was modified in place
			}
		}

		if op == opGet {
			return results, nil
		}

		return nil, urp.combineErrors(errs)

	case map[string]any:
		if isLastSegment {
			switch op {
			case opGet:
				// PERFORMANCE: Pre-allocate slice with capacity hint
				results := make([]any, 0, len(container))
				for _, val := range container {
					results = append(results, val)
				}
				return results, nil
			case opSet:
				// Set value to all map entries
				for key := range container {
					container[key] = value
				}
				return value, nil
			case opDelete:
				// Delete all map entries
				for key := range container {
					delete(container, key)
				}
				return nil, nil
			}
		}

		// Recursively process all map values
		// PERFORMANCE: Pre-allocate slices with capacity hints
		results := make([]any, 0, len(container))
		errs := make([]error, 0, 4)

		for key, mapValue := range container {
			result, err := urp.processRecursivelyAtSegmentsWithOptions(mapValue, segments, segmentIndex+1, op, value, createPaths)
			if err != nil {
				errs = append(errs, err)
				continue
			}

			if op == opGet && result != nil {
				// Preserve structure - don't flatten unless explicitly requested
				results = append(results, result)
			} else if op == opSet {
				container[key] = mapValue // Value was modified in place
			}
		}

		if op == opGet {
			return results, nil
		}

		return nil, urp.combineErrors(errs)

	default:
		if op == opGet {
			return nil, nil // Cannot wildcard non-container
		}
		return nil, fmt.Errorf("cannot apply wildcard to type %T", data)
	}
}

// combineErrors combines multiple errors into a single error using modern Go 1.24+ patterns
func (urp *recursiveProcessor) combineErrors(errs []error) error {
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

// findTargetArrayForDistributedOp finds the actual target array for distributed ops
// This handles nested array structures that may result from extraction ops
func (urp *recursiveProcessor) findTargetArrayForDistributedOp(item any) []any {
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
						return urp.findTargetArrayForDistributedOp(nestedArr)
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

// deepFlattenDistributedResults performs deep flattening of distributed op results
// This handles nested array structures like [["Alice", "David"], ["Frank"]] -> ["Alice", "David", "Frank"]
func (urp *recursiveProcessor) deepFlattenDistributedResults(results []any) []any {
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
func (urp *recursiveProcessor) deepFlattenResults(results []any, flattened *[]any) {
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

// shouldUseDistributedArrayOp determines if an array op should be distributed
// based on the actual data structure. Optimized with early exit and sampling.
func (urp *recursiveProcessor) shouldUseDistributedArrayOp(container []any) bool {
	// Distributed ops should ONLY be used for extraction results, not regular nested arrays
	// Regular nested arrays like [[1,2,3], [4,5,6]] should use normal array indexing
	// Extraction results have specific patterns that distinguish them from regular nested arrays

	n := len(container)

	// If the container is empty, no distributed op needed
	if n == 0 {
		return false
	}

	// Fast path: Check for triple-nested pattern (extraction result wrapper)
	// This is the most common extraction result pattern
	if n == 1 {
		if arr, ok := container[0].([]any); ok && len(arr) == 1 {
			if _, ok := arr[0].([]any); ok {
				// This is [[[...]]] pattern - extraction result
				return true
			}
		}
	}

	// Optimization: Only check up to maxCheckElements to avoid O(n) traversal for large arrays
	// Statistical sampling is sufficient for pattern detection
	maxCheckElements := n
	if n > 20 {
		maxCheckElements = 20 // Check at most 20 elements
	}

	// Check if ALL sampled elements are arrays AND ALL contain objects.
	// Stricter than before: every sampled inner array must contain at least one object.
	// This prevents treating [[1,2,3], [4,5,6]] as an extraction result.
	allArrays := true
	allContainObjects := true

	for i := 0; i < maxCheckElements; i++ {
		item := container[i]
		arr, ok := item.([]any)
		if !ok {
			allArrays = false
			break
		}

		// Check if this inner array contains at least one object
		foundObject := false
		maxInnerCheck := len(arr)
		if maxInnerCheck > 10 {
			maxInnerCheck = 10
		}
		for j := 0; j < maxInnerCheck; j++ {
			if _, isObj := arr[j].(map[string]any); isObj {
				foundObject = true
				break
			}
		}
		if !foundObject {
			allContainObjects = false
		}
	}

	// Only use distributed op if ALL elements are arrays AND all contain objects
	return allArrays && allContainObjects
}
