package json

import (
	"errors"
)

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
