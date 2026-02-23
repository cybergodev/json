package json

import (
	"fmt"
	"strings"

	"github.com/cybergodev/json/internal"
)

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
