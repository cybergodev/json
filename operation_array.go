package json

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"github.com/cybergodev/json/internal"
)

// normalizeNegativeIndex converts a negative array index to a positive one.
// Returns an error if the resulting index is out of bounds.
func normalizeNegativeIndex(index, length int) (int, error) {
	if index < 0 {
		index = length + index
	}
	if index < 0 || index >= length {
		return 0, fmt.Errorf("index %d out of range [0, %d)", index, length)
	}
	return index, nil
}

// normalizeNegativeIndexAllowExtend converts a negative array index to a positive one.
// Unlike normalizeNegativeIndex, it allows index >= length (for array extension scenarios).
func normalizeNegativeIndexAllowExtend(index, length int) (int, error) {
	if index < 0 {
		index = length + index
	}
	if index < 0 {
		return 0, fmt.Errorf("array index %d out of bounds after negative conversion", index)
	}
	return index, nil
}

// normalizeNegativeSliceBounds converts negative start/end indices for slice operations.
// end may equal length (exclusive upper bound). Returns normalized start, end, or an error.
func normalizeNegativeSliceBounds(start, end, length int) (int, int, error) {
	if start < 0 {
		start = length + start
	}
	if end < 0 {
		end = length + end
	}
	if start < 0 || start >= length || end < 0 || end > length || start >= end {
		return 0, 0, fmt.Errorf("slice range [%d:%d] out of bounds for array length %d", start, end, length)
	}
	return start, end, nil
}

func (p *Processor) handleArrayAccess(data any, segment internal.PathSegment) propertyAccessResult {
	var arrayData any = data
	if segment.Key != "" {
		propResult := p.handlePropertyAccess(data, segment.Key)
		if !propResult.exists {
			return propertyAccessResult{value: nil, exists: false}
		}
		arrayData = propResult.value
	}

	if arr, ok := arrayData.([]any); ok {
		index, err := normalizeNegativeIndex(segment.Index, len(arr))
		if err != nil {
			return propertyAccessResult{value: nil, exists: false}
		}
		return propertyAccessResult{value: arr[index], exists: true}
	}

	return propertyAccessResult{value: nil, exists: false}
}

func (p *Processor) parseArrayIndex(indexStr string) int {
	if idx, ok := internal.ParseArrayIndex(indexStr); ok {
		return idx
	}
	return -1
}

func (p *Processor) handleArraySlice(data any, segment internal.PathSegment) propertyAccessResult {
	arr, ok := data.([]any)
	if !ok {
		return propertyAccessResult{value: nil, exists: false}
	}

	// Extract slice parameters from segment
	var start, end, step *int
	var startVal, endVal, stepVal int

	if segment.HasStart() {
		startVal = segment.Index // Index stores start for slices
		start = &startVal
	}
	if segment.HasEnd() {
		endVal = segment.End
		end = &endVal
	}
	if segment.HasStep() {
		stepVal = segment.Step
		step = &stepVal
	}

	// Use unified implementation from internal package
	result := internal.PerformArraySlice(arr, start, end, step)
	return propertyAccessResult{value: result, exists: true}
}

func (p *Processor) parseSliceParameters(segmentValue string, arrayLength int) (start, end, step int, err error) {
	// Remove brackets if present
	if strings.HasPrefix(segmentValue, "[") && strings.HasSuffix(segmentValue, "]") {
		segmentValue = segmentValue[1 : len(segmentValue)-1]
	}

	// Delegate to internal package for parsing
	s, e, st, parseErr := internal.ParseSliceComponents(segmentValue)
	if parseErr != nil {
		return 0, 0, 0, parseErr
	}

	// Apply defaults for nil pointers
	start = 0
	if s != nil {
		start = *s
	}

	end = arrayLength
	if e != nil {
		end = *e
	}

	step = 1
	if st != nil {
		step = *st
		if step <= 0 {
			return 0, 0, 0, fmt.Errorf("step must be positive: %d", step)
		}
	}

	return start, end, step, nil
}

func (p *Processor) isArrayIndex(segment string) bool {
	// Remove brackets if present
	if strings.HasPrefix(segment, "[") && strings.HasSuffix(segment, "]") {
		segment = segment[1 : len(segment)-1]
	}

	// Check if it's a valid integer
	_, err := strconv.Atoi(segment)
	return err == nil
}

func (p *Processor) navigateToArrayIndexWithNegative(current any, index int, createPaths bool) (any, error) {
	switch v := current.(type) {
	case []any:
		idx, err := normalizeNegativeIndexAllowExtend(index, len(v))
		if err != nil {
			return nil, err
		}

		if idx >= len(v) {
			if createPaths && idx == len(v) {
				// Extend array by one element
				return nil, nil // Placeholder for new element
			}
			return nil, fmt.Errorf("array index %d out of bounds (length %d)", idx, len(v))
		}
		return v[idx], nil
	default:
		return nil, fmt.Errorf("cannot access array index %d on type %T", index, current)
	}
}

func (p *Processor) assignValueToSlice(arr []any, start, end, step int, value any) error {
	if start < 0 || end > len(arr) || start >= end {
		return fmt.Errorf("invalid slice range: [%d:%d] for array length %d", start, end, len(arr))
	}

	if step <= 0 {
		return fmt.Errorf("step must be positive: %d", step)
	}

	// Assign value to each position in the slice
	for i := start; i < end; i += step {
		arr[i] = value
	}

	return nil
}

func (p *Processor) cleanupNullValuesWithReconstruction(data any, compactArrays bool) any {
	return internal.CleanupNullValues(data, compactArrays)
}

func (p *Processor) cleanupDeletedMarkers(data any) any {
	switch v := data.(type) {
	case []any:
		// PERFORMANCE: Count non-deleted elements for precise pre-allocation
		n := 0
		for _, item := range v {
			if item != deletedMarker {
				n++
			}
		}
		result := make([]any, 0, n)
		for _, item := range v {
			if item != deletedMarker {
				result = append(result, p.cleanupDeletedMarkers(item))
			}
		}
		return result

	case map[string]any:
		result := make(map[string]any, len(v))
		for key, value := range v {
			if value != deletedMarker {
				result[key] = p.cleanupDeletedMarkers(value)
			}
		}
		return result

	default:
		return data
	}
}

func (p *Processor) handleExtraction(data any, segment internal.PathSegment) (any, error) {
	field := segment.Key
	if field == "" {
		return nil, fmt.Errorf("invalid extraction syntax: %s", segment.String())
	}

	// Check for multi-field extraction (comma-separated fields)
	if strings.Contains(field, ",") {
		return p.handleMultiFieldExtraction(data, field, segment.IsFlatExtract())
	}

	// Handle array extraction with pre-allocated results slice and flattening
	if arr, ok := data.([]any); ok {
		results := make([]any, 0, len(arr)) // Pre-allocate with array length

		for _, item := range arr {
			// Use the existing handlePropertyAccessValue function for consistent field extraction
			if value := p.handlePropertyAccessValue(item, field); value != nil {
				if segment.IsFlatExtract() {
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

// handleMultiFieldExtraction handles extraction of multiple fields from an object or array
// Returns a new object (or array of objects) containing only the specified fields
func (p *Processor) handleMultiFieldExtraction(data any, fieldsStr string, isFlat bool) (any, error) {
	fields := strings.Split(fieldsStr, ",")

	// Handle array extraction
	if arr, ok := data.([]any); ok {
		results := make([]any, 0, len(arr))
		for _, item := range arr {
			extracted := p.extractFieldsFromObject(item, fields)
			if extracted != nil {
				if isFlat {
					// For flat extraction, flatten nested arrays
					p.flattenValue(extracted, &results)
				} else {
					results = append(results, extracted)
				}
			}
		}
		return results, nil
	}

	// Handle single object extraction
	return p.extractFieldsFromObject(data, fields), nil
}

// extractFieldsFromObject extracts specified fields from a single object
// Returns a new map containing only the specified fields that exist in the source
func (p *Processor) extractFieldsFromObject(data any, fields []string) map[string]any {
	obj, ok := data.(map[string]any)
	if !ok {
		return nil
	}

	result := make(map[string]any, len(fields))
	for _, field := range fields {
		field = strings.TrimSpace(field)
		if field == "" {
			continue
		}
		if value, exists := obj[field]; exists {
			result[field] = value
		}
	}

	if len(result) == 0 {
		return nil
	}
	return result
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
	if v.Kind() == reflect.Pointer {
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

func (p *Processor) getValueWithDistributedOperation(data any, path string) (any, error) {
	// Parse the path to identify distributed operation patterns
	segments := p.getPathSegments()
	defer p.putPathSegments(segments)

	*segments = p.splitPath(path, *segments)

	// Find the extraction segment that triggers distributed operation
	extractionIndex := -1
	for i, segment := range *segments {
		if segment.Type == internal.ExtractSegment {
			// Check if this is followed by array operations
			if i+1 < len(*segments) {
				nextSegment := (*segments)[i+1]
				if nextSegment.Type == internal.ArrayIndexSegment || nextSegment.Type == internal.ArraySliceSegment {
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
	preSegments := (*segments)[:extractionIndex]
	extractionSegment := (*segments)[extractionIndex]
	postSegments := (*segments)[extractionIndex+1:]

	// Navigate to the extraction point
	current := data
	for _, segment := range preSegments {
		switch segment.Type {
		case internal.PropertySegment:
			result := p.handlePropertyAccess(current, segment.Key)
			if !result.exists {
				return nil, ErrPathNotFound
			}
			current = result.value
		case internal.ArrayIndexSegment:
			result := p.handleArrayAccess(current, segment)
			if !result.exists {
				return nil, ErrPathNotFound
			}
			current = result.value
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
			switch segment.Type {
			case internal.ArrayIndexSegment:
				result = p.applySingleArrayOperation(result, segment)
			case internal.ArraySliceSegment:
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

func (p *Processor) extractIndividualArrays(data any, extractionSegment internal.PathSegment) ([]any, error) {
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

func (p *Processor) applySingleArrayOperation(array any, segment internal.PathSegment) any {
	if arr, ok := array.([]any); ok {
		result := p.handleArrayAccess(arr, segment)
		if result.exists {
			return result.value
		}
	}
	return nil
}

func (p *Processor) applySingleArraySlice(array any, segment internal.PathSegment) any {
	if arr, ok := array.([]any); ok {
		result := p.handleArraySlice(arr, segment)
		if result.exists {
			return result.value
		}
	}
	return nil
}

func (p *Processor) handlePostExtractionArrayAccess(data any, segment internal.PathSegment) any {
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

func (p *Processor) isComplexPath(path string) bool {
	return internal.IsComplexPath(path)
}

// PERFORMANCE: Compares segment.Type directly instead of TypeString() to avoid
// string allocation on every comparison in hot loops.
func (p *Processor) hasComplexSegments(segments []internal.PathSegment) bool {
	for _, seg := range segments {
		if seg.Type == internal.ExtractSegment || seg.Type == internal.ArraySliceSegment {
			return true
		}
	}
	return false
}
