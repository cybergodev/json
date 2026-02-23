package json

import (
	"fmt"
	"reflect"
	"strings"
)

func (p *Processor) handleExtraction(data any, segment PathSegment) (any, error) {
	field := segment.Key
	if field == "" {
		return nil, fmt.Errorf("invalid extraction syntax: %s", segment.String())
	}

	// Handle array extraction with pre-allocated results slice and flattening
	if arr, ok := data.([]any); ok {
		results := make([]any, 0, len(arr)) // Pre-allocate with array length

		for _, item := range arr {
			// Use the existing handlePropertyAccessValue function for consistent field extraction
			if value := p.handlePropertyAccessValue(item, field); value != nil {
				if segment.IsFlat {
					// For flat extraction, always flatten arrays recursively
					p.flattenValue(value, &results)
				} else {
					// For regular extraction, add the field value directly
					results = append(results, value)
				}
			}
		}
		return results, nil
	}

	// Handle single object extraction
	if obj, ok := data.(map[string]any); ok {
		if value := p.handlePropertyAccessValue(obj, field); value != nil {
			return value, nil
		}
	}

	// For non-extractable types (strings, numbers, etc.), return nil without error
	// This matches the expected behavior in tests
	return nil, nil
}

func (p *Processor) flattenValue(value any, results *[]any) {
	if arr, ok := value.([]any); ok {
		// If it's an array, recursively flatten each element
		for _, item := range arr {
			p.flattenValue(item, results)
		}
	} else {
		// If it's not an array, add it directly to results
		*results = append(*results, value)
	}
}

func (p *Processor) handleStructAccess(data any, fieldName string) any {
	if data == nil {
		return nil
	}

	v := reflect.ValueOf(data)
	if v.Kind() == reflect.Ptr {
		if v.IsNil() {
			return nil
		}
		v = v.Elem()
	}

	if v.Kind() != reflect.Struct {
		return nil
	}

	// Try direct field access first
	field := v.FieldByName(fieldName)
	if field.IsValid() && field.CanInterface() {
		return field.Interface()
	}

	// Try case-insensitive field access
	t := v.Type()
	for i := 0; i < v.NumField(); i++ {
		structField := t.Field(i)
		if strings.EqualFold(structField.Name, fieldName) {
			field := v.Field(i)
			if field.CanInterface() {
				return field.Interface()
			}
		}
	}

	return nil
}

func (p *Processor) handleDeepExtractionInNavigation(data any, segments []PathSegment) (any, error) {
	// Group consecutive extraction segments
	extractionGroups := p.detectConsecutiveExtractions(segments)

	current := data
	segmentIndex := 0

	for _, group := range extractionGroups {
		// Process consecutive extractions
		result, err := p.processConsecutiveExtractions(current, group.Segments, segments[segmentIndex+len(group.Segments):])
		if err != nil {
			return nil, err
		}
		current = result
		segmentIndex += len(group.Segments)
	}

	return current, nil
}

func (p *Processor) hasDeepExtractionPattern(segments []PathSegment) bool {
	extractionCount := 0
	for _, segment := range segments {
		if segment.TypeString() == "extract" {
			extractionCount++
			if extractionCount > 1 {
				return true
			}
		}
	}
	return false
}

func (p *Processor) hasMixedExtractionOperations(segments []PathSegment) bool {
	hasFlat := false
	hasRegular := false

	for _, segment := range segments {
		if segment.TypeString() == "extract" {
			if segment.IsFlat {
				hasFlat = true
			} else {
				hasRegular = true
			}

			if hasFlat && hasRegular {
				return true
			}
		}
	}

	return false
}

func (p *Processor) getValueWithDistributedOperation(data any, path string) (any, error) {
	// Parse the path to identify distributed operation patterns
	segments := p.getPathSegments()
	defer p.putPathSegments(segments)

	segments = p.splitPath(path, segments)

	// Find the extraction segment that triggers distributed operation
	extractionIndex := -1
	for i, segment := range segments {
		if segment.TypeString() == "extract" {
			// Check if this is followed by array operations
			if i+1 < len(segments) {
				nextSegment := segments[i+1]
				if nextSegment.TypeString() == "array" || nextSegment.TypeString() == "slice" {
					extractionIndex = i
					break
				}
			}
		}
	}

	if extractionIndex == -1 {
		// No distributed operation pattern found, use regular navigation
		return p.navigateToPath(data, path)
	}

	// Split segments into pre-extraction, extraction, and post-extraction
	preSegments := segments[:extractionIndex]
	extractionSegment := segments[extractionIndex]
	postSegments := segments[extractionIndex+1:]

	// Navigate to the extraction point
	current := data
	for _, segment := range preSegments {
		switch segment.TypeString() {
		case "property":
			result := p.handlePropertyAccess(current, segment.Key)
			if !result.Exists {
				return nil, ErrPathNotFound
			}
			current = result.Value
		case "array":
			result := p.handleArrayAccess(current, segment)
			if !result.Exists {
				return nil, ErrPathNotFound
			}
			current = result.Value
		}
	}

	// Extract individual arrays
	extractedArrays, err := p.extractIndividualArrays(current, extractionSegment)
	if err != nil {
		return nil, err
	}

	// Apply post-extraction operations to each array
	results := make([]any, 0, len(extractedArrays))
	for _, arr := range extractedArrays {
		// Apply post-extraction segments
		result := arr
		for _, segment := range postSegments {
			switch segment.TypeString() {
			case "array":
				result = p.applySingleArrayOperation(result, segment)
			case "slice":
				result = p.applySingleArraySlice(result, segment)
			}
		}

		// Add result if it's not nil
		if result != nil {
			results = append(results, result)
		}
	}

	return results, nil
}

func (p *Processor) extractIndividualArrays(data any, extractionSegment PathSegment) ([]any, error) {
	field := extractionSegment.Key
	if field == "" {
		return nil, fmt.Errorf("invalid extraction syntax: %s", extractionSegment.String())
	}

	// Pre-allocate with estimated capacity
	var results []any
	if arr, ok := data.([]any); ok {
		results = make([]any, 0, len(arr))
		for _, item := range arr {
			if obj, ok := item.(map[string]any); ok {
				if value := p.handlePropertyAccessValue(obj, field); value != nil {
					// Check if the extracted value is an array
					if extractedArr, ok := value.([]any); ok {
						results = append(results, extractedArr)
					}
				}
			}
		}
	}

	return results, nil
}

func (p *Processor) applySingleArrayOperation(array any, segment PathSegment) any {
	if arr, ok := array.([]any); ok {
		result := p.handleArrayAccess(arr, segment)
		if result.Exists {
			return result.Value
		}
	}
	return nil
}

func (p *Processor) applySingleArraySlice(array any, segment PathSegment) any {
	if arr, ok := array.([]any); ok {
		result := p.handleArraySlice(arr, segment)
		if result.Exists {
			return result.Value
		}
	}
	return nil
}

func (p *Processor) findTargetArrayForDistributedOperation(item any) []any {
	// This method would contain logic to find the target array within an item
	// For now, return the item if it's an array
	if arr, ok := item.([]any); ok {
		return arr
	}
	return nil
}

func (p *Processor) handlePostExtractionArrayAccess(data any, segment PathSegment) any {
	// Check if data is an array of arrays (result of extraction)
	if arr, ok := data.([]any); ok {
		results := make([]any, 0, len(arr))

		for _, item := range arr {
			if itemArr, ok := item.([]any); ok {
				// Apply array operation to each sub-array
				result := p.applySingleArrayOperation(itemArr, segment)
				if result != nil {
					results = append(results, result)
				}
			}
		}

		return results
	}

	// For single array, apply operation directly
	return p.applySingleArrayOperation(data, segment)
}

func (p *Processor) handleDistributedArrayAccess(data any, segment PathSegment) any {
	return p.handlePostExtractionArrayAccess(data, segment)
}

func (p *Processor) handlePostExtractionArraySlice(data any, segment PathSegment) any {
	// Check if data is an array of arrays (result of extraction)
	if arr, ok := data.([]any); ok {
		results := make([]any, 0, len(arr))

		for _, item := range arr {
			if itemArr, ok := item.([]any); ok {
				// Apply slice operation to each sub-array
				result := p.applySingleArraySlice(itemArr, segment)
				if result != nil {
					results = append(results, result)
				}
			}
		}

		return results
	}

	// For single array, apply operation directly
	return p.applySingleArraySlice(data, segment)
}

func (p *Processor) processConsecutiveExtractions(data any, extractionSegments []PathSegment, remainingSegments []PathSegment) (any, error) {
	current := data

	// Apply each extraction in sequence
	for _, segment := range extractionSegments {
		result, err := p.handleExtraction(current, segment)
		if err != nil {
			return nil, err
		}
		current = result
	}

	return current, nil
}

func (p *Processor) buildExtractionContext(data any, segments []PathSegment) (*ExtractionContext, error) {
	// Find extraction segments and build context
	var extractionSegments []PathSegment
	var arrayFieldName string

	for _, segment := range segments {
		if segment.TypeString() == "extract" {
			extractionSegments = append(extractionSegments, segment)
			if arrayFieldName == "" {
				arrayFieldName = segment.Key
			}
		}
	}

	if len(extractionSegments) == 0 {
		return nil, fmt.Errorf("no extraction segments found")
	}

	// Collect source containers
	containers, err := p.collectSourceContainers(data, extractionSegments)
	if err != nil {
		return nil, err
	}

	return &ExtractionContext{
		OriginalContainers: containers,
		ArrayFieldName:     arrayFieldName,
		TargetIndices:      make([]int, len(containers)),
		OperationType:      "get",
	}, nil
}

func (p *Processor) collectSourceContainers(data any, extractionSegments []PathSegment) ([]any, error) {
	var containers []any

	// This is a simplified implementation
	// The full implementation would recursively traverse the data structure
	if arr, ok := data.([]any); ok {
		for _, item := range arr {
			containers = append(containers, item)
		}
	} else {
		containers = append(containers, data)
	}

	return containers, nil
}

func (p *Processor) collectContainersForExtraction(data any, extractionSegments []PathSegment, containers *[]any) error {
	return p.collectContainersRecursive(data, extractionSegments, 0, containers)
}

func (p *Processor) collectContainersRecursive(current any, segments []PathSegment, segmentIndex int, containers *[]any) error {
	if segmentIndex >= len(segments) {
		*containers = append(*containers, current)
		return nil
	}

	segment := segments[segmentIndex]

	switch segment.TypeString() {
	case "property":
		if obj, ok := current.(map[string]any); ok {
			if value, exists := obj[segment.Key]; exists {
				return p.collectContainersRecursive(value, segments, segmentIndex+1, containers)
			}
		}
	case "array":
		if arr, ok := current.([]any); ok {
			for _, item := range arr {
				if err := p.collectContainersRecursive(item, segments, segmentIndex+1, containers); err != nil {
					return err
				}
			}
		}
	case "extract":
		// For extraction, collect all items that would be extracted
		if arr, ok := current.([]any); ok {
			for _, item := range arr {
				*containers = append(*containers, item)
			}
		}
	}

	return nil
}

func (p *Processor) flattenContainersRecursive(arr []any, containers *[]any) {
	for _, item := range arr {
		if nestedArr, ok := item.([]any); ok {
			p.flattenContainersRecursive(nestedArr, containers)
		} else {
			*containers = append(*containers, item)
		}
	}
}
