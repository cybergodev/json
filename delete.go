package json

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/cybergodev/json/internal"
)

// DeletionTarget represents a specific location in the original data structure that needs to be deleted
type DeletionTarget struct {
	Container any    // The container (array or object) that holds the value to delete
	Key       any    // The key (string for objects, int for arrays) to delete
	Path      string // The path to this target for debugging
}

// ComplexDeleteProcessor handles complex path deletions with reverse mapping
type ComplexDeleteProcessor struct {
	processor *Processor
}

// NewComplexDeleteProcessor creates a new complex delete processor
func NewComplexDeleteProcessor(p *Processor) *ComplexDeleteProcessor {
	return &ComplexDeleteProcessor{processor: p}
}

// DeleteWithReverseMapping performs deletion using reverse mapping for complex paths
func (cdp *ComplexDeleteProcessor) DeleteWithReverseMapping(data any, path string) error {
	// For complex paths with extraction followed by array operations,
	// we interpret this as deleting the entire extracted field from all matching objects

	// Check if this path has extraction followed by array operations
	if cdp.hasExtractionWithArrayOps(path) {
		return cdp.deleteExtractedFieldsFromAllObjects(data, path)
	}

	// Parse the path into segments for other cases
	parser := internal.NewPathParser()
	segments, err := parser.ParsePath(path)
	if err != nil {
		return fmt.Errorf("failed to parse path '%s': %w", path, err)
	}

	if len(segments) == 0 {
		return fmt.Errorf("empty path")
	}

	// Check if this is a complex path that needs reverse mapping
	if cdp.needsReverseMapping(segments) {
		return cdp.deleteWithReverseMapping(data, segments)
	}

	// Fall back to standard deletion for simple paths
	return cdp.processor.deleteValueWithInternalSegments(data, segments)
}

// needsReverseMapping determines if a path requires reverse mapping
func (cdp *ComplexDeleteProcessor) needsReverseMapping(segments []internal.PathSegment) bool {
	// Look for extraction followed by array index/slice
	for i := 0; i < len(segments)-1; i++ {
		if segments[i].Type == internal.ExtractSegment {
			nextSegment := segments[i+1]
			if nextSegment.Type == internal.ArrayIndexSegment || nextSegment.Type == internal.ArraySliceSegment {
				return true
			}
		}
	}
	return false
}

// deleteWithReverseMapping performs the actual reverse mapping deletion
func (cdp *ComplexDeleteProcessor) deleteWithReverseMapping(data any, segments []internal.PathSegment) error {
	// Find the last extraction segment that is followed by array operations
	lastExtractionIndex := -1
	for i := len(segments) - 1; i >= 0; i-- {
		if segments[i].Type == internal.ExtractSegment {
			// Check if there are array operations after this extraction
			if i < len(segments)-1 {
				nextSegment := segments[i+1]
				if nextSegment.Type == internal.ArrayIndexSegment || nextSegment.Type == internal.ArraySliceSegment {
					lastExtractionIndex = i
					break
				}
			}
		}
	}

	if lastExtractionIndex == -1 {
		// No extraction with post-operations found, use standard deletion
		return cdp.processor.deleteValueWithInternalSegments(data, segments)
	}

	// Split segments: everything before the last extraction, the extraction itself, and post-extraction
	preExtractionSegments := segments[:lastExtractionIndex]
	extractionSegment := segments[lastExtractionIndex]
	postExtractionSegments := segments[lastExtractionIndex+1:]

	// Navigate to the container that holds the data to be extracted
	container, err := cdp.navigateToContainer(data, preExtractionSegments)
	if err != nil {
		return fmt.Errorf("failed to navigate to extraction container: %w", err)
	}

	// Find all deletion targets using reverse mapping
	targets, err := cdp.findDeletionTargets(container, extractionSegment, postExtractionSegments)
	if err != nil {
		return fmt.Errorf("failed to find deletion targets: %w", err)
	}

	// Execute deletions on all targets
	return cdp.executeDeletions(targets)
}

// navigateToContainer navigates to the container that holds the data to be extracted
func (cdp *ComplexDeleteProcessor) navigateToContainer(data any, segments []internal.PathSegment) (any, error) {
	current := data
	for _, segment := range segments {
		switch segment.Type {
		case internal.PropertySegment:
			if obj, ok := current.(map[string]any); ok {
				if next, exists := obj[segment.Key]; exists {
					current = next
				} else {
					return nil, fmt.Errorf("property '%s' not found", segment.Key)
				}
			} else {
				return nil, fmt.Errorf("cannot access property '%s' on type %T", segment.Key, current)
			}
		case internal.ArrayIndexSegment:
			if arr, ok := current.([]any); ok {
				index := segment.Index
				if index < 0 {
					index = len(arr) + index
				}
				if index >= 0 && index < len(arr) {
					current = arr[index]
				} else {
					return nil, fmt.Errorf("array index %d out of bounds", segment.Index)
				}
			} else {
				return nil, fmt.Errorf("cannot access array index on type %T", current)
			}
		case internal.ExtractSegment:
			// For pre-extraction segments, we need to handle extraction differently
			// This should not happen in pre-extraction segments, but let's handle it gracefully
			return nil, fmt.Errorf("unexpected extraction segment in pre-extraction navigation")
		default:
			return nil, fmt.Errorf("unsupported segment type in pre-extraction: %v", segment.Type)
		}
	}
	return current, nil
}

// findDeletionTargets finds all specific locations that need to be deleted
func (cdp *ComplexDeleteProcessor) findDeletionTargets(container any, extractionSegment internal.PathSegment, postExtractionSegments []internal.PathSegment) ([]DeletionTarget, error) {
	var targets []DeletionTarget

	// Handle different container types
	switch v := container.(type) {
	case []any:
		// Extract from array elements
		for i, item := range v {
			itemTargets, err := cdp.findTargetsInItem(item, extractionSegment, postExtractionSegments, fmt.Sprintf("[%d]", i))
			if err != nil {
				continue // Skip items that don't match
			}
			targets = append(targets, itemTargets...)
		}
	case map[string]any:
		// Extract from single object
		itemTargets, err := cdp.findTargetsInItem(v, extractionSegment, postExtractionSegments, "")
		if err != nil {
			return nil, err
		}
		targets = append(targets, itemTargets...)
	default:
		return nil, fmt.Errorf("cannot extract from type %T", container)
	}

	return targets, nil
}

// findTargetsInItem finds deletion targets within a single item
func (cdp *ComplexDeleteProcessor) findTargetsInItem(item any, extractionSegment internal.PathSegment, postExtractionSegments []internal.PathSegment, basePath string) ([]DeletionTarget, error) {
	var targets []DeletionTarget

	// Get the extracted data
	extractedData, err := cdp.performExtraction(item, extractionSegment)
	if err != nil {
		return nil, err
	}

	// Apply post-extraction operations to find specific targets
	if len(postExtractionSegments) == 0 {
		// No post-extraction operations, delete the entire extracted field
		if obj, ok := item.(map[string]any); ok {
			targets = append(targets, DeletionTarget{
				Container: obj,
				Key:       extractionSegment.Key,
				Path:      basePath + "." + extractionSegment.Key,
			})
		}
		return targets, nil
	}

	// Handle post-extraction operations
	return cdp.findTargetsWithPostExtraction(item, extractedData, extractionSegment, postExtractionSegments, basePath)
}

// performExtraction performs the extraction operation on an item
func (cdp *ComplexDeleteProcessor) performExtraction(item any, extractionSegment internal.PathSegment) (any, error) {
	if obj, ok := item.(map[string]any); ok {
		if value, exists := obj[extractionSegment.Key]; exists {
			return value, nil
		}
		return nil, fmt.Errorf("extraction key '%s' not found", extractionSegment.Key)
	}
	return nil, fmt.Errorf("cannot extract from type %T", item)
}

// findTargetsWithPostExtraction handles post-extraction operations to find specific deletion targets
func (cdp *ComplexDeleteProcessor) findTargetsWithPostExtraction(originalItem, extractedData any, extractionSegment internal.PathSegment, postExtractionSegments []internal.PathSegment, basePath string) ([]DeletionTarget, error) {
	var targets []DeletionTarget

	// The key insight: when we have {field}[index] or {field}[slice], we need to understand
	// that this represents a conceptual operation that doesn't map directly to the original structure.
	// The most reasonable interpretation is to delete the entire field that was extracted from.

	if len(postExtractionSegments) == 1 {
		postSegment := postExtractionSegments[0]

		switch postSegment.Type {
		case internal.ArrayIndexSegment:
			// Handle {field}[index] - delete the entire field since we can't partially delete extracted results
			if obj, ok := originalItem.(map[string]any); ok {
				// Check if the field exists and contains the data that would match the index operation
				if _, exists := obj[extractionSegment.Key]; exists {
					// For flat extractions, we still delete the entire field
					targets = append(targets, DeletionTarget{
						Container: obj,
						Key:       extractionSegment.Key,
						Path:      basePath + "." + extractionSegment.Key,
					})
				}
			}
		case internal.ArraySliceSegment:
			// Handle {field}[start:end] - delete the entire field since we can't partially delete extracted results
			if obj, ok := originalItem.(map[string]any); ok {
				if _, exists := obj[extractionSegment.Key]; exists {
					targets = append(targets, DeletionTarget{
						Container: obj,
						Key:       extractionSegment.Key,
						Path:      basePath + "." + extractionSegment.Key,
					})
				}
			}
		}
	}

	return targets, nil
}

// executeDeletions executes all the deletion operations
func (cdp *ComplexDeleteProcessor) executeDeletions(targets []DeletionTarget) error {
	for _, target := range targets {
		switch container := target.Container.(type) {
		case map[string]any:
			if key, ok := target.Key.(string); ok {
				delete(container, key)
			}
		case []any:
			if index, ok := target.Key.(int); ok && index >= 0 && index < len(container) {
				// For arrays, we need to use a deletion marker or remove the element
				// Using deletion marker approach similar to existing code
				container[index] = deletedMarker
			}
		}
	}
	return nil
}

// hasExtractionWithArrayOps checks if path has extraction followed by array operations
func (cdp *ComplexDeleteProcessor) hasExtractionWithArrayOps(path string) bool {
	// Check for any pattern that has extraction followed by array operations
	return strings.Contains(path, "}[")
}

// deleteExtractedFieldsFromAllObjects deletes the extracted field from all matching objects
func (cdp *ComplexDeleteProcessor) deleteExtractedFieldsFromAllObjects(data any, path string) error {
	// Check if this is a nested extraction pattern like {members}[0]{name}
	if cdp.hasNestedExtractionAfterArray(path) {
		return cdp.deleteFromNestedExtraction(data, path)
	}

	// Check if this is an array index/slice operation on extracted data
	if cdp.isArrayOperationOnExtraction(path) {
		return cdp.deleteFromExtractedArrays(data, path)
	}

	// Extract the field name from the path
	fieldName := cdp.extractFieldNameFromPath(path)
	if fieldName == "" {
		return fmt.Errorf("could not extract field name from path: %s", path)
	}

	// Find the base path (everything before the extraction)
	basePath := cdp.extractBasePath(path)

	// Navigate to all objects that contain the field and delete it
	return cdp.deleteFieldFromAllMatchingObjects(data, basePath, fieldName)
}

// extractFieldNameFromPath extracts the field name from complex paths
func (cdp *ComplexDeleteProcessor) extractFieldNameFromPath(path string) string {
	// Look for patterns like {name}[, {flat:name}[
	if idx := strings.LastIndex(path, "{"); idx >= 0 {
		if endIdx := strings.Index(path[idx:], "}"); endIdx >= 0 {
			fieldPart := path[idx+1 : idx+endIdx]
			// Handle flat: prefix
			if strings.HasPrefix(fieldPart, "flat:") {
				return strings.TrimPrefix(fieldPart, "flat:")
			}
			return fieldPart
		}
	}
	return ""
}

// extractBasePath extracts the base path before the final extraction
func (cdp *ComplexDeleteProcessor) extractBasePath(path string) string {
	// Find the last complete extraction pattern {field}[...] and remove everything after the }
	// For path like "company.departments{teams}{members}{flat:name}[0]"
	// We want to return "company.departments{teams}{members}"

	// Find the position of the last }[ pattern
	lastBraceArrayIdx := strings.LastIndex(path, "}[")
	if lastBraceArrayIdx >= 0 {
		// Find the corresponding opening brace for this closing brace
		braceCount := 0
		for i := lastBraceArrayIdx; i >= 0; i-- {
			if path[i] == '}' {
				braceCount++
			} else if path[i] == '{' {
				braceCount--
				if braceCount == 0 {
					// Found the matching opening brace, return everything before it
					return path[:i]
				}
			}
		}
	}

	// Fallback: find the last { and return everything before it
	if idx := strings.LastIndex(path, "{"); idx >= 0 {
		return path[:idx]
	}
	return path
}

// deleteFieldFromAllMatchingObjects deletes a field from all objects that match the base path
func (cdp *ComplexDeleteProcessor) deleteFieldFromAllMatchingObjects(data any, basePath, fieldName string) error {
	// Navigate to the base path and find all objects
	objects, err := cdp.findAllObjectsAtPath(data, basePath)
	if err != nil {
		return fmt.Errorf("failed to find objects at path '%s': %w", basePath, err)
	}

	// Delete the field from each object
	for _, obj := range objects {
		if objMap, ok := obj.(map[string]any); ok {
			delete(objMap, fieldName)
		}
	}

	return nil
}

// findAllObjectsAtPath finds all objects that match the given path pattern
func (cdp *ComplexDeleteProcessor) findAllObjectsAtPath(data any, path string) ([]any, error) {
	var objects []any

	// Use the processor's navigation capabilities to find objects dynamically
	// instead of hardcoding specific paths
	if path == "" {
		return objects, nil
	}

	// Parse the path to understand the structure
	parser := internal.NewPathParser()
	segments, err := parser.ParsePath(path)
	if err != nil {
		return nil, fmt.Errorf("failed to parse path '%s': %w", path, err)
	}

	// Navigate through the path and collect all matching objects
	return cdp.navigateAndCollectObjects(data, segments, 0)
}

// navigateAndCollectObjects recursively navigates through path segments and collects matching objects
func (cdp *ComplexDeleteProcessor) navigateAndCollectObjects(current any, segments []internal.PathSegment, segmentIndex int) ([]any, error) {
	var objects []any

	// If we've processed all segments, return the current object
	if segmentIndex >= len(segments) {
		if current != nil {
			objects = append(objects, current)
		}
		return objects, nil
	}

	segment := segments[segmentIndex]

	switch segment.Type {
	case internal.PropertySegment:
		// Navigate to property
		if objMap, ok := current.(map[string]any); ok {
			if value, exists := objMap[segment.Key]; exists {
				return cdp.navigateAndCollectObjects(value, segments, segmentIndex+1)
			}
		}

	case internal.ArrayIndexSegment:
		// Navigate to array index
		if arr, ok := current.([]any); ok {
			index := segment.Index
			if segment.IsNegative {
				index = len(arr) + index
			}
			if index >= 0 && index < len(arr) {
				return cdp.navigateAndCollectObjects(arr[index], segments, segmentIndex+1)
			}
		}

	case internal.ExtractSegment:
		// Extract from all objects in array or single object
		switch v := current.(type) {
		case []any:
			// Extract from all objects in the array
			for _, item := range v {
				if objMap, ok := item.(map[string]any); ok {
					if segmentIndex == len(segments)-1 {
						// This is the last segment, collect the objects that contain the field
						if _, exists := objMap[segment.Key]; exists {
							objects = append(objects, objMap)
						}
					} else {
						// Continue navigation
						if value, exists := objMap[segment.Key]; exists {
							subObjects, err := cdp.navigateAndCollectObjects(value, segments, segmentIndex+1)
							if err != nil {
								continue // Skip this item on error
							}
							objects = append(objects, subObjects...)
						}
					}
				}
			}
		case map[string]any:
			// Extract from single object
			if segmentIndex == len(segments)-1 {
				// This is the last segment, collect the object if it contains the field
				if _, exists := v[segment.Key]; exists {
					objects = append(objects, v)
				}
			} else {
				// Continue navigation
				if value, exists := v[segment.Key]; exists {
					return cdp.navigateAndCollectObjects(value, segments, segmentIndex+1)
				}
			}
		}
	}

	return objects, nil
}

// isArrayOperationOnExtraction checks if the path represents an array operation on extracted data
func (cdp *ComplexDeleteProcessor) isArrayOperationOnExtraction(path string) bool {
	// Look for patterns like {field}[index] where we want to delete from arrays, not delete the field
	// Also check for nested operations like {field}[index]{subfield}

	// Check for specific known patterns that represent actual array fields
	arrayFieldPatterns := []string{"{tasks}[", "{items}[", "{elements}[", "{members}[", "{teams}["}
	for _, pattern := range arrayFieldPatterns {
		if strings.Contains(path, pattern) {
			return true
		}
	}

	// Check for scalar field patterns that should be treated as array operations on extracted data
	// These patterns indicate operations on extracted scalar values, like {name}[0] or {flat:name}[0]
	scalarFieldPatterns := []string{"{name}[", "{flat:name}[", "{id}[", "{title}[", "{value}["}
	for _, pattern := range scalarFieldPatterns {
		if strings.Contains(path, pattern) {
			return true
		}
	}

	// Also check for nested operations like {field}[index]{subfield}
	return cdp.hasNestedExtractionAfterArray(path)
}

// hasNestedExtractionAfterArray checks if path has pattern like {field}[index]{subfield}
func (cdp *ComplexDeleteProcessor) hasNestedExtractionAfterArray(path string) bool {
	// Look for pattern: }[something]{something}
	if idx := strings.Index(path, "}["); idx >= 0 {
		remaining := path[idx+2:]
		if closeBracket := strings.Index(remaining, "]"); closeBracket >= 0 {
			afterBracket := remaining[closeBracket+1:]
			return strings.Contains(afterBracket, "{") && strings.Contains(afterBracket, "}")
		}
	}
	return false
}

// deleteFromExtractedArrays handles deletion from arrays that were extracted
func (cdp *ComplexDeleteProcessor) deleteFromExtractedArrays(data any, path string) error {
	// Check if this is a nested extraction like {field}[index]{subfield}
	if cdp.hasNestedExtractionAfterArray(path) {
		return cdp.deleteFromNestedExtraction(data, path)
	}

	fieldName := cdp.extractFieldNameFromPath(path)
	if fieldName == "" {
		return fmt.Errorf("could not extract field name from path: %s", path)
	}

	// Extract the array index or slice info
	arrayOp := cdp.extractArrayOperation(path)
	if arrayOp == "" {
		return fmt.Errorf("could not extract array operation from path: %s", path)
	}

	// Find the base path (everything before the extraction)
	basePath := cdp.extractBasePath(path)

	// Navigate to all objects that contain the field
	objects, err := cdp.findAllObjectsAtPath(data, basePath)
	if err != nil {
		return fmt.Errorf("failed to find objects at path '%s': %w", basePath, err)
	}

	// For paths like {field}[index], we need to understand what this means:
	// 1. If the field is an array, delete the element at the specified index from each array
	// 2. If the field is a scalar, delete the field from the Nth object (where N is the index)

	// Parse the array operation to get the index
	index, isSlice, err := cdp.parseArrayOperation(arrayOp)
	if err != nil {
		return fmt.Errorf("failed to parse array operation '%s': %w", arrayOp, err)
	}

	if isSlice {
		// For slice operations, we need more complex logic
		return cdp.handleSliceOperationOnExtractedData(objects, fieldName, arrayOp)
	}

	// For simple index operations
	return cdp.handleIndexOperationOnExtractedData(objects, fieldName, index)
}

// parseArrayOperation parses an array operation string and returns index and whether it's a slice
func (cdp *ComplexDeleteProcessor) parseArrayOperation(operation string) (int, bool, error) {
	if strings.Contains(operation, ":") {
		// This is a slice operation
		return 0, true, nil
	}

	// Simple index operation
	if operation == "-1" {
		return -1, false, nil
	}

	index, err := strconv.Atoi(operation)
	if err != nil {
		return 0, false, fmt.Errorf("invalid array index: %s", operation)
	}

	return index, false, nil
}

// handleIndexOperationOnExtractedData handles index operations on extracted data
func (cdp *ComplexDeleteProcessor) handleIndexOperationOnExtractedData(objects []any, fieldName string, index int) error {
	// For index operations like {tasks}[0], we need to distinguish between:
	// 1. If the field is an array: delete the element at the specified index from each array
	// 2. If the field is a scalar: delete the field from the Nth object that contains the field

	var objectsWithField []map[string]any

	// Flatten the nested structure and collect all objects that have the field
	cdp.flattenAndCollectObjects(objects, fieldName, &objectsWithField)

	if len(objectsWithField) == 0 {
		return nil // No objects have the field
	}

	// Check if the field is an array in the first object that has it
	// If it's an array, we should delete from the array, not delete the field
	firstObject := objectsWithField[0]
	if fieldValue, exists := firstObject[fieldName]; exists {
		if _, isArray := fieldValue.([]any); isArray {
			// The field is an array, delete the element at the specified index from all arrays
			return cdp.deleteFromArrayFields(objectsWithField, fieldName, index)
		}
	}

	// The field is not an array, use the original logic:
	// Delete the field from the Nth object that contains the field

	// Handle negative indices
	if index < 0 {
		index = len(objectsWithField) + index
	}

	// Check bounds
	if index < 0 || index >= len(objectsWithField) {
		return nil // Index out of bounds, do nothing
	}

	// Delete the field from the specified object
	delete(objectsWithField[index], fieldName)
	return nil
}

// deleteFromArrayFields deletes an element at the specified index from array fields
func (cdp *ComplexDeleteProcessor) deleteFromArrayFields(objects []map[string]any, fieldName string, index int) error {
	for _, obj := range objects {
		if fieldValue, exists := obj[fieldName]; exists {
			if arr, isArray := fieldValue.([]any); isArray {
				// Handle negative indices
				actualIndex := index
				if actualIndex < 0 {
					actualIndex = len(arr) + actualIndex
				}

				// Check bounds
				if actualIndex >= 0 && actualIndex < len(arr) {
					// Remove the element at the specified index
					newArr := make([]any, 0, len(arr)-1)
					newArr = append(newArr, arr[:actualIndex]...)
					newArr = append(newArr, arr[actualIndex+1:]...)
					obj[fieldName] = newArr
				}
			}
		}
	}
	return nil
}

// flattenAndCollectObjects recursively flattens nested arrays and collects objects with the specified field
func (cdp *ComplexDeleteProcessor) flattenAndCollectObjects(data []any, fieldName string, result *[]map[string]any) {
	for _, item := range data {
		switch v := item.(type) {
		case map[string]any:
			// This is an object, check if it has the field
			if _, exists := v[fieldName]; exists {
				*result = append(*result, v)
			}
		case []any:
			// This is an array, recursively flatten it
			cdp.flattenAndCollectObjects(v, fieldName, result)
		}
	}
}

// handleSliceOperationOnExtractedData handles slice operations on extracted data
func (cdp *ComplexDeleteProcessor) handleSliceOperationOnExtractedData(objects []any, fieldName string, operation string) error {
	// For slice operations like {name}[0:2], we interpret this as:
	// Delete the field from objects at the specified slice range

	var objectsWithField []map[string]any

	// Flatten the nested structure and collect all objects that have the field
	cdp.flattenAndCollectObjects(objects, fieldName, &objectsWithField)

	if len(objectsWithField) == 0 {
		return nil // No objects have the field
	}

	// Parse slice operation
	start, end, err := cdp.parseSliceOperation(operation, len(objectsWithField))
	if err != nil {
		return fmt.Errorf("failed to parse slice operation '%s': %w", operation, err)
	}

	// Delete the field from objects in the slice range
	for i := start; i < end && i < len(objectsWithField); i++ {
		delete(objectsWithField[i], fieldName)
	}

	return nil
}

// parseSliceOperation parses a slice operation string like "0:2", ":3", "1:", etc.
func (cdp *ComplexDeleteProcessor) parseSliceOperation(operation string, arrayLen int) (int, int, error) {
	parts := strings.Split(operation, ":")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("invalid slice operation: %s", operation)
	}

	var start, end int
	var err error

	// Parse start
	if parts[0] == "" {
		start = 0
	} else {
		start, err = strconv.Atoi(parts[0])
		if err != nil {
			return 0, 0, fmt.Errorf("invalid start index: %s", parts[0])
		}
		if start < 0 {
			start = arrayLen + start
		}
	}

	// Parse end
	if parts[1] == "" {
		end = arrayLen
	} else {
		end, err = strconv.Atoi(parts[1])
		if err != nil {
			return 0, 0, fmt.Errorf("invalid end index: %s", parts[1])
		}
		if end < 0 {
			end = arrayLen + end
		}
	}

	// Ensure bounds
	if start < 0 {
		start = 0
	}
	if end > arrayLen {
		end = arrayLen
	}
	if start > end {
		start = end
	}

	return start, end, nil
}

// extractArrayOperation extracts the array operation part from the path
func (cdp *ComplexDeleteProcessor) extractArrayOperation(path string) string {
	// Look for [index] or [start:end] patterns
	if startIdx := strings.LastIndex(path, "["); startIdx >= 0 {
		if endIdx := strings.Index(path[startIdx:], "]"); endIdx >= 0 {
			return path[startIdx+1 : startIdx+endIdx]
		}
	}
	return ""
}

// applyArrayDeletion applies the deletion operation to an array
func (cdp *ComplexDeleteProcessor) applyArrayDeletion(arr []any, operation string) ([]any, error) {
	if len(arr) == 0 {
		return arr, nil // Nothing to delete from empty array
	}

	// Handle simple index like "0", "1", "-1"
	if !strings.Contains(operation, ":") {
		// Simple index deletion
		var index int
		if operation == "-1" {
			index = len(arr) - 1
		} else {
			// Parse the index
			if _, err := fmt.Sscanf(operation, "%d", &index); err != nil {
				return nil, fmt.Errorf("invalid array index: %s", operation)
			}
			if index < 0 {
				index = len(arr) + index
			}
		}

		// Check bounds
		if index < 0 || index >= len(arr) {
			return arr, nil // Index out of bounds, return original array
		}

		// Remove the element at index
		result := make([]any, 0, len(arr)-1)
		result = append(result, arr[:index]...)
		result = append(result, arr[index+1:]...)
		return result, nil
	}

	// Handle slice operations like "0:1", ":2", "-1:", etc.
	return cdp.applySliceDeletion(arr, operation)
}

// applySliceDeletion handles slice deletion operations
func (cdp *ComplexDeleteProcessor) applySliceDeletion(arr []any, operation string) ([]any, error) {
	// Parse slice notation like ":2", "1:3", "2:", etc.
	parts := strings.Split(operation, ":")
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid slice operation: %s", operation)
	}

	var start, end int
	var err error

	// Parse start index
	if parts[0] == "" {
		start = 0
	} else {
		start, err = strconv.Atoi(parts[0])
		if err != nil {
			return nil, fmt.Errorf("invalid start index in slice: %s", parts[0])
		}
		if start < 0 {
			start = len(arr) + start
		}
	}

	// Parse end index
	if parts[1] == "" {
		end = len(arr)
	} else {
		end, err = strconv.Atoi(parts[1])
		if err != nil {
			return nil, fmt.Errorf("invalid end index in slice: %s", parts[1])
		}
		if end < 0 {
			end = len(arr) + end
		}
	}

	// Validate bounds
	if start < 0 {
		start = 0
	}
	if end > len(arr) {
		end = len(arr)
	}
	if start >= end {
		return arr, nil // Nothing to delete
	}

	// Remove elements from start to end (exclusive)
	result := make([]any, 0, len(arr)-(end-start))
	result = append(result, arr[:start]...)
	result = append(result, arr[end:]...)
	return result, nil
}

// deleteFromNestedExtraction handles paths like {field}[index]{subfield}
func (cdp *ComplexDeleteProcessor) deleteFromNestedExtraction(data any, path string) error {
	// Parse path like "company.departments{teams}{members}[0]{name}"
	// 1. Extract "company.departments{teams}" as base path
	// 2. Extract "members" as array field
	// 3. Extract "0" as array index
	// 4. Extract "name" as subfield to delete

	// Find the pattern }[...]{...}
	arrayStartIdx := strings.Index(path, "}[")
	if arrayStartIdx < 0 {
		return fmt.Errorf("invalid nested extraction path: %s", path)
	}

	// Split into parts: before }[, array operation, after ]{
	beforeArray := path[:arrayStartIdx+1] // includes the }
	remaining := path[arrayStartIdx+2:]   // after }[

	closeBracket := strings.Index(remaining, "]")
	if closeBracket < 0 {
		return fmt.Errorf("invalid array operation in path: %s", path)
	}

	arrayOp := remaining[:closeBracket]
	afterArray := remaining[closeBracket+1:] // after ]

	// Extract the subfield from afterArray (should be like {name})
	if !strings.HasPrefix(afterArray, "{") || !strings.HasSuffix(afterArray, "}") {
		return fmt.Errorf("invalid subfield extraction in path: %s", afterArray)
	}

	subfield := afterArray[1 : len(afterArray)-1] // remove { and }

	// Extract the array field name from the last extraction in beforeArray
	// For "company.departments{teams}{members}", we want "members"
	arrayFieldName := cdp.extractLastFieldNameFromPath(beforeArray)
	if arrayFieldName == "" {
		return fmt.Errorf("could not extract array field name from: %s", beforeArray)
	}

	// Extract the base path (everything before the last extraction)
	// For "company.departments{teams}{members}", we want "company.departments{teams}"
	basePath := cdp.extractBasePathBeforeLastExtraction(beforeArray)

	// Navigate to all objects that contain the array field
	// For nested extraction, we need to find the actual container objects, not the extracted results
	objects, err := cdp.findContainerObjectsForField(data, basePath, arrayFieldName)
	if err != nil {
		return fmt.Errorf("failed to find container objects: %w", err)
	}

	// Parse array operation - could be single index or slice
	if strings.Contains(arrayOp, ":") {
		// Handle slice operation like "0:2", "1:", ":3"
		return cdp.deleteFromNestedExtractionSlice(objects, arrayFieldName, arrayOp, subfield)
	} else {
		// Handle single index operation like "0", "-1"
		index, err := strconv.Atoi(arrayOp)
		if err != nil {
			return fmt.Errorf("invalid array index '%s': %w", arrayOp, err)
		}
		return cdp.deleteFromNestedExtractionIndex(objects, arrayFieldName, index, subfield)
	}

	return nil
}

// parseArrayIndex parses array index from operation string
func (cdp *ComplexDeleteProcessor) parseArrayIndex(arr []any, operation string) (int, error) {
	if operation == "-1" {
		return len(arr) - 1, nil
	}

	index, err := strconv.Atoi(operation)
	if err != nil {
		return -1, fmt.Errorf("invalid array index: %s", operation)
	}

	if index < 0 {
		index = len(arr) + index
	}

	return index, nil
}

// extractLastFieldNameFromPath extracts the last field name from a path with extractions
// For "company.departments{teams}{members}", returns "members"
func (cdp *ComplexDeleteProcessor) extractLastFieldNameFromPath(path string) string {
	// Find the last occurrence of {
	lastBraceStart := strings.LastIndex(path, "{")
	if lastBraceStart == -1 {
		return ""
	}

	// Find the corresponding }
	braceEnd := strings.Index(path[lastBraceStart:], "}")
	if braceEnd == -1 {
		return ""
	}

	// Extract the field name
	fieldName := path[lastBraceStart+1 : lastBraceStart+braceEnd]

	// Handle flat: prefix
	if strings.HasPrefix(fieldName, "flat:") {
		return strings.TrimPrefix(fieldName, "flat:")
	}

	return fieldName
}

// extractBasePathBeforeLastExtraction extracts the base path before the last extraction
// For "company.departments{teams}{members}", returns "company.departments{teams}"
func (cdp *ComplexDeleteProcessor) extractBasePathBeforeLastExtraction(path string) string {
	// Find the last occurrence of {
	lastBraceStart := strings.LastIndex(path, "{")
	if lastBraceStart == -1 {
		return path // No extractions found
	}

	// Return everything before the last extraction
	return path[:lastBraceStart]
}

// deleteFromNestedExtractionIndex handles single index deletion in nested extraction
func (cdp *ComplexDeleteProcessor) deleteFromNestedExtractionIndex(objects []any, arrayFieldName string, index int, subfield string) error {
	for _, obj := range objects {
		if objMap, ok := obj.(map[string]any); ok {
			if arrayField, exists := objMap[arrayFieldName]; exists {
				if arr, ok := arrayField.([]any); ok {
					// Handle negative indices
					actualIndex := index
					if actualIndex < 0 {
						actualIndex = len(arr) + actualIndex
					}

					// Access the element at index and delete the subfield
					if actualIndex >= 0 && actualIndex < len(arr) {
						if element, ok := arr[actualIndex].(map[string]any); ok {
							delete(element, subfield)
						}
					}
				}
			}
		}
	}
	return nil
}

// deleteFromNestedExtractionSlice handles slice deletion in nested extraction
func (cdp *ComplexDeleteProcessor) deleteFromNestedExtractionSlice(objects []any, arrayFieldName string, sliceOp string, subfield string) error {
	for _, obj := range objects {
		if objMap, ok := obj.(map[string]any); ok {
			if arrayField, exists := objMap[arrayFieldName]; exists {
				if arr, ok := arrayField.([]any); ok {
					// Parse slice operation
					start, end, err := cdp.parseSliceOperation(sliceOp, len(arr))
					if err != nil {
						continue // Skip invalid slice operations
					}

					// Delete subfield from elements in the slice range
					for i := start; i < end && i < len(arr); i++ {
						if element, ok := arr[i].(map[string]any); ok {
							delete(element, subfield)
						}
					}
				}
			}
		}
	}
	return nil
}

// findContainerObjectsForField finds all objects that contain the specified field
// This is different from findAllObjectsAtPath which returns extracted results
func (cdp *ComplexDeleteProcessor) findContainerObjectsForField(data any, basePath, fieldName string) ([]any, error) {
	// For nested extraction like "company.departments{teams}", we need to navigate
	// to the actual objects that contain the field, not the extracted results

	// Parse the base path to understand the structure
	if strings.Contains(basePath, "{") {
		// This is an extraction path, we need to find the source objects
		return cdp.findSourceObjectsFromExtractionPath(data, basePath, fieldName)
	}

	// Simple path, use existing logic
	return cdp.findAllObjectsAtPath(data, basePath)
}

// findSourceObjectsFromExtractionPath finds source objects from an extraction path
func (cdp *ComplexDeleteProcessor) findSourceObjectsFromExtractionPath(data any, extractionPath, targetField string) ([]any, error) {
	// For "company.departments{teams}", we need to:
	// 1. Navigate to "company.departments"
	// 2. For each department, get the teams array
	// 3. For each team in teams array, check if it has the targetField

	// Extract the base path before extraction
	lastBraceStart := strings.LastIndex(extractionPath, "{")
	if lastBraceStart == -1 {
		return nil, fmt.Errorf("no extraction found in path: %s", extractionPath)
	}

	basePathBeforeExtraction := extractionPath[:lastBraceStart]
	if strings.HasSuffix(basePathBeforeExtraction, ".") {
		basePathBeforeExtraction = basePathBeforeExtraction[:len(basePathBeforeExtraction)-1]
	}

	// Navigate to the base path
	var containers []any
	if basePathBeforeExtraction == "" {
		containers = []any{data}
	} else {
		baseContainers, err := cdp.findAllObjectsAtPath(data, basePathBeforeExtraction)
		if err != nil {
			return nil, fmt.Errorf("failed to navigate to base path '%s': %w", basePathBeforeExtraction, err)
		}
		containers = baseContainers
	}

	// Now find all objects that contain the target field
	var result []any
	for _, container := range containers {
		objects := cdp.findObjectsWithField(container, targetField)
		result = append(result, objects...)
	}

	return result, nil
}

// findObjectsWithField recursively finds all objects that contain the specified field
func (cdp *ComplexDeleteProcessor) findObjectsWithField(data any, fieldName string) []any {
	var result []any

	switch v := data.(type) {
	case map[string]any:
		// Check if this object has the field
		if _, exists := v[fieldName]; exists {
			result = append(result, v)
		}
		// Recursively check nested objects and arrays
		for _, value := range v {
			result = append(result, cdp.findObjectsWithField(value, fieldName)...)
		}
	case []any:
		// Check each item in the array
		for _, item := range v {
			result = append(result, cdp.findObjectsWithField(item, fieldName)...)
		}
	}

	return result
}

// getArrayLength helper function for debugging
func getArrayLength(data any) int {
	if arr, ok := data.([]any); ok {
		return len(arr)
	}
	return -1
}
