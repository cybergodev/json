package json

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"sync"

	"github.com/cybergodev/json/internal"
)

// ============================================================================
// INTERNAL OPERATION TYPES
// These types are used internally for tracking operation types during processing.
// ============================================================================

// operation represents the type of operation being performed
type operation int

const (
	opGet operation = iota
	opSet
	opDelete
	opValidate
)

// String returns the string representation of the operation
func (op operation) String() string {
	switch op {
	case opGet:
		return "get"
	case opSet:
		return "set"
	case opDelete:
		return "delete"
	case opValidate:
		return "validate"
	default:
		return "unknown"
	}
}

// ============================================================================
// INTERNAL ERROR TYPES
// ============================================================================

// arrayExtensionSignal is an internal control-flow value (implements error for
// upward propagation through call stack). It is NOT a real error — it signals
// that an array needs to be extended before the value can be set.
// Used exclusively within the set-operation path when createPaths is true.
type arrayExtensionSignal struct {
	requiredLength int
	currentLength  int
	start          int
	end            int
	step           int
	value          any
}

func (e *arrayExtensionSignal) Error() string {
	return "array extension needed: current length " + strconv.Itoa(e.currentLength) +
		", required length " + strconv.Itoa(e.requiredLength) + " for slice [" +
		strconv.Itoa(e.start) + ":" + strconv.Itoa(e.end) + "]"
}

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

func (p *Processor) deleteValueAtPath(data any, path string) error {
	// Handle JSON Pointer format
	if strings.HasPrefix(path, "/") {
		return p.deleteValueJSONPointer(data, path)
	}

	// Check for complex paths
	if p.isComplexPath(path) {
		return p.deleteValueComplexPath(data, path)
	}

	// Use dot notation for simple paths
	return p.deleteValueDotNotation(data, path)
}

// navigateToParent navigates through data to the parent container of the final segment.
// Returns the parent container and the final segment key/index.
// Shared by deleteValueDotNotation and deleteValueJSONPointer to avoid duplicating
// the map[string]any/map[any]any/[]any switch logic.
func (p *Processor) navigateToParent(data any, segments []string) (any, string, error) {
	if len(segments) == 0 {
		return nil, "", fmt.Errorf("empty path")
	}

	current := data
	for i := 0; i < len(segments)-1; i++ {
		segment := segments[i]

		switch v := current.(type) {
		case map[string]any:
			if next, exists := v[segment]; exists {
				current = next
			} else {
				return nil, "", fmt.Errorf("path not found: %s", segment)
			}
		case map[any]any:
			if next, exists := v[segment]; exists {
				current = next
			} else {
				return nil, "", fmt.Errorf("path not found: %s", segment)
			}
		case []any:
			if index, ok := internal.ParseAndValidateArrayIndex(segment, len(v)); ok {
				current = v[index]
			} else {
				return nil, "", fmt.Errorf("invalid array index: %s", segment)
			}
		default:
			return nil, "", fmt.Errorf("cannot navigate through %T at segment %s", current, segment)
		}
	}

	return current, segments[len(segments)-1], nil
}

func (p *Processor) deleteValueDotNotation(data any, path string) error {
	segments, err := p.parsePath(path)
	if err != nil {
		return err
	}

	parent, finalSegment, err := p.navigateToParent(data, segments)
	if err != nil {
		return err
	}
	return p.deletePropertyValue(parent, finalSegment)
}

func (p *Processor) deleteValueJSONPointer(data any, path string) error {
	if path == "/" {
		return fmt.Errorf("cannot delete root")
	}

	// Split and unescape JSON Pointer segments upfront
	segments := strings.Split(path[1:], "/")
	for i := range segments {
		if strings.Contains(segments[i], "~") {
			segments[i] = internal.UnescapeJSONPointer(segments[i])
		}
	}

	parent, finalSegment, err := p.navigateToParent(data, segments)
	if err != nil {
		return err
	}
	return p.deletePropertyValue(parent, finalSegment)
}

func (p *Processor) deletePropertyValue(current any, property string) error {
	switch current.(type) {
	case []any:
		if _, err := strconv.Atoi(property); err == nil {
			return p.deleteArrayElement(current, property)
		}
		return fmt.Errorf("invalid array index: %s", property)
	default:
		return p.deletePropertyFromContainer(current, property)
	}
}

func (p *Processor) deleteValueComplexPath(data any, path string) error {
	// Parse path into segments
	segments := p.getPathSegments()
	defer p.putPathSegments(segments)

	segments = p.splitPath(path, segments)

	// Check if this requires complex deletion
	if p.hasComplexSegments(segments) {
		return p.deleteValueComplexSegments(data, segments, 0)
	}

	// Convert to internal segments for complex processing
	// Since internal.PathSegment is now an alias for internal.PathSegment, we can copy directly
	internalSegments := make([]internal.PathSegment, len(segments))
	copy(internalSegments, segments)

	return p.deleteValueWithInternalSegments(data, internalSegments)
}

func (p *Processor) deleteValueWithInternalSegments(data any, segments []internal.PathSegment) error {
	if len(segments) == 0 {
		return fmt.Errorf("no segments provided")
	}

	// Navigate to parent
	current := data
	for i := 0; i < len(segments)-1; i++ {
		next, err := p.navigateSegmentForDeletion(current, segments[i])
		if err != nil {
			return err
		}
		current = next
	}

	// Delete final segment
	finalSegment := segments[len(segments)-1]
	return p.deleteValueForSegment(current, finalSegment)
}

func (p *Processor) navigateSegmentForDeletion(current any, segment internal.PathSegment) (any, error) {
	switch segment.Type {
	case internal.PropertySegment:
		return p.navigatePropertyForDeletion(current, segment.Key)
	case internal.ArrayIndexSegment:
		return p.navigateArrayIndexForDeletion(current, segment.String())
	case internal.ArraySliceSegment:
		// For slices, return the current container
		return current, nil
	case internal.ExtractSegment:
		// For extractions, return the current container
		return current, nil
	default:
		return nil, fmt.Errorf("unsupported segment type for deletion: %v", segment.Type.String())
	}
}

func (p *Processor) navigatePropertyForDeletion(current any, property string) (any, error) {
	switch v := current.(type) {
	case map[string]any:
		if val, exists := v[property]; exists {
			return val, nil
		}
		return nil, fmt.Errorf("property not found: %s", property)
	case map[any]any:
		if val, exists := v[property]; exists {
			return val, nil
		}
		return nil, fmt.Errorf("property not found: %s", property)
	default:
		return nil, fmt.Errorf("cannot access property '%s' on type %T", property, current)
	}
}

func (p *Processor) navigateArrayIndexForDeletion(current any, indexStr string) (any, error) {
	arr, ok := current.([]any)
	if !ok {
		return nil, fmt.Errorf("cannot access array index on type %T", current)
	}

	// PERFORMANCE: Use fastParseInt instead of strconv.Atoi to avoid error allocation overhead
	index, ok := internal.ParseArrayIndex(indexStr)
	if !ok {
		return nil, fmt.Errorf("invalid array index: %s", indexStr)
	}

	index, err := normalizeNegativeIndex(index, len(arr))
	if err != nil {
		return nil, err
	}

	return arr[index], nil
}

func (p *Processor) deleteValueForSegment(current any, segment internal.PathSegment) error {
	switch segment.Type {
	case internal.PropertySegment:
		return p.deletePropertyFromContainer(current, segment.Key)
	case internal.ArrayIndexSegment:
		return p.deleteArrayElementByIndex(current, segment.Index)
	case internal.ArraySliceSegment:
		return p.deleteArraySlice(current, segment)
	case internal.ExtractSegment:
		return p.deleteExtractedValues(current, segment)
	default:
		return fmt.Errorf("unsupported segment type for deletion: %v", segment.Type.String())
	}
}

func (p *Processor) deletePropertyFromContainer(current any, property string) error {
	switch v := current.(type) {
	case map[string]any:
		if _, exists := v[property]; exists {
			delete(v, property)
			return nil
		}
		return fmt.Errorf("property not found: %s", property)
	case map[any]any:
		if _, exists := v[property]; exists {
			delete(v, property)
			return nil
		}
		return fmt.Errorf("property not found: %s", property)
	default:
		return fmt.Errorf("cannot delete property '%s' from type %T", property, current)
	}
}

func (p *Processor) deleteArrayElement(current any, indexStr string) error {
	// PERFORMANCE: Use fastParseInt instead of strconv.Atoi
	index, ok := internal.ParseArrayIndex(indexStr)
	if !ok {
		return fmt.Errorf("invalid array index: %s", indexStr)
	}
	return p.deleteArrayElementByIndex(current, index)
}

func (p *Processor) deleteArrayElementByIndex(current any, index int) error {
	arr, ok := current.([]any)
	if !ok {
		return fmt.Errorf("cannot delete array element from type %T", current)
	}

	index, err := normalizeNegativeIndex(index, len(arr))
	if err != nil {
		return err
	}

	// Mark element for deletion (set to special marker)
	arr[index] = deletedMarker
	return nil
}

func (p *Processor) deleteArraySlice(current any, segment internal.PathSegment) error {
	arr, ok := current.([]any)
	if !ok {
		return fmt.Errorf("cannot delete slice from type %T", current)
	}

	// Parse slice parameters
	start, end, step, err := p.parseSliceParameters(segment.String(), len(arr))
	if err != nil {
		return err
	}

	// Normalize negative indices and validate bounds
	start, end, err = normalizeNegativeSliceBounds(start, end, len(arr))
	if err != nil {
		return err
	}

	// Mark elements for deletion
	for i := start; i < end; i += step {
		arr[i] = deletedMarker
	}

	return nil
}

func (p *Processor) deleteExtractedValues(current any, segment internal.PathSegment) error {
	field := segment.Key
	if field == "" {
		return fmt.Errorf("invalid extraction syntax: %s", segment.String())
	}

	// Handle array extraction
	if arr, ok := current.([]any); ok {
		for _, item := range arr {
			if obj, ok := item.(map[string]any); ok {
				delete(obj, field)
			}
		}
		return nil
	}

	// Handle single object
	if obj, ok := current.(map[string]any); ok {
		delete(obj, field)
		return nil
	}

	return fmt.Errorf("cannot perform extraction deletion on type %T", current)
}

func (p *Processor) deleteValueComplexSegments(data any, segments []internal.PathSegment, segmentIndex int) error {
	if segmentIndex >= len(segments) {
		return nil
	}

	segment := segments[segmentIndex]

	switch segment.Type {
	case internal.PropertySegment:
		return p.deleteComplexProperty(data, segment, segments, segmentIndex)
	case internal.ArrayIndexSegment:
		return p.deleteComplexArray(data, segment, segments, segmentIndex)
	case internal.ArraySliceSegment:
		return p.deleteComplexSlice(data, segment, segments, segmentIndex)
	case internal.ExtractSegment:
		return p.deleteComplexExtract(data, segment, segments, segmentIndex)
	default:
		return fmt.Errorf("unsupported complex segment type: %v", segment.Type.String())
	}
}

func (p *Processor) deleteComplexProperty(data any, segment internal.PathSegment, segments []internal.PathSegment, segmentIndex int) error {
	if segmentIndex == len(segments)-1 {
		// Last segment, delete the property
		return p.deletePropertyFromContainer(data, segment.Key)
	}

	// Navigate to next level
	switch v := data.(type) {
	case map[string]any:
		if next, exists := v[segment.Key]; exists {
			return p.deleteValueComplexSegments(next, segments, segmentIndex+1)
		}
		return fmt.Errorf("property not found: %s", segment.Key)
	case map[any]any:
		if next, exists := v[segment.Key]; exists {
			return p.deleteValueComplexSegments(next, segments, segmentIndex+1)
		}
		return fmt.Errorf("property not found: %s", segment.Key)
	default:
		return fmt.Errorf("cannot access property '%s' on type %T", segment.Key, data)
	}
}

func (p *Processor) deleteComplexArray(data any, segment internal.PathSegment, segments []internal.PathSegment, segmentIndex int) error {
	arr, ok := data.([]any)
	if !ok {
		return fmt.Errorf("cannot access array on type %T", data)
	}

	index, err := normalizeNegativeIndex(segment.Index, len(arr))
	if err != nil {
		return err
	}

	if segmentIndex == len(segments)-1 {
		// Last segment, delete the array element
		arr[index] = deletedMarker
		return nil
	}

	// Navigate to next level
	return p.deleteValueComplexSegments(arr[index], segments, segmentIndex+1)
}

func (p *Processor) deleteComplexSlice(data any, segment internal.PathSegment, segments []internal.PathSegment, segmentIndex int) error {
	arr, ok := data.([]any)
	if !ok {
		return fmt.Errorf("cannot perform slice operation on type %T", data)
	}

	if segmentIndex == len(segments)-1 {
		// Last segment, delete the slice
		return p.deleteArraySlice(data, segment)
	}

	// For intermediate slices, we need to apply the operation to each element in the slice
	start, end, step, err := p.parseSliceParameters(segment.String(), len(arr))
	if err != nil {
		return err
	}

	// Normalize negative indices and validate bounds
	start, end, err = normalizeNegativeSliceBounds(start, end, len(arr))
	if err != nil {
		return err
	}

	// Apply deletion to each element in the slice
	var firstErr error
	for i := start; i < end; i += step {
		if err := p.deleteValueComplexSegments(arr[i], segments, segmentIndex+1); err != nil {
			if firstErr == nil {
				firstErr = err
			}
			if !p.config.ContinueOnError {
				return firstErr
			}
		}
	}

	return firstErr
}

func (p *Processor) deleteComplexExtract(data any, segment internal.PathSegment, segments []internal.PathSegment, segmentIndex int) error {
	field := segment.Key
	if field == "" {
		return fmt.Errorf("invalid extraction syntax: %s", segment.String())
	}

	if segmentIndex == len(segments)-1 {
		// Last segment, delete extracted values
		return p.deleteExtractedValues(data, segment)
	}

	// Check for consecutive extractions
	if p.hasConsecutiveExtractions(segments, segmentIndex) {
		return p.deleteConsecutiveExtractions(data, segments, segmentIndex)
	}

	// Handle array extraction with further navigation
	if arr, ok := data.([]any); ok {
		for _, item := range arr {
			if obj, ok := item.(map[string]any); ok {
				if extractedValue, exists := obj[field]; exists {
					if err := p.deleteValueComplexSegments(extractedValue, segments, segmentIndex+1); err != nil {
						// Continue with other items even if one fails
						continue
					}
				}
			}
		}
		return nil
	}

	// Handle single object extraction
	if obj, ok := data.(map[string]any); ok {
		if extractedValue, exists := obj[field]; exists {
			return p.deleteValueComplexSegments(extractedValue, segments, segmentIndex+1)
		}
		return fmt.Errorf("extraction field '%s' not found", field)
	}

	return fmt.Errorf("cannot perform extraction on type %T", data)
}

func (p *Processor) hasConsecutiveExtractions(segments []internal.PathSegment, startIndex int) bool {
	if startIndex+1 >= len(segments) {
		return false
	}

	return segments[startIndex].Type == internal.ExtractSegment &&
		segments[startIndex+1].Type == internal.ExtractSegment
}

func (p *Processor) deleteConsecutiveExtractions(data any, segments []internal.PathSegment, segmentIndex int) error {
	// Find all consecutive extraction segments
	var extractionSegments []internal.PathSegment
	i := segmentIndex
	for i < len(segments) && segments[i].Type == internal.ExtractSegment {
		extractionSegments = append(extractionSegments, segments[i])
		i++
	}

	remainingSegments := segments[i:]

	return p.processConsecutiveExtractionsForDeletion(data, extractionSegments, remainingSegments)
}

func (p *Processor) processConsecutiveExtractionsForDeletion(data any, extractionSegments []internal.PathSegment, remainingSegments []internal.PathSegment) error {
	if len(extractionSegments) == 0 {
		return nil
	}

	// Apply first extraction
	firstExtraction := extractionSegments[0]
	field := firstExtraction.Key

	if arr, ok := data.([]any); ok {
		for _, item := range arr {
			if obj, ok := item.(map[string]any); ok {
				if extractedValue, exists := obj[field]; exists {
					if len(extractionSegments) == 1 {
						// Last extraction, apply remaining segments or delete
						if len(remainingSegments) == 0 {
							delete(obj, field)
						} else {
							if err := p.deleteValueComplexSegments(extractedValue, remainingSegments, 0); err != nil {
							return err
						}
						}
					} else {
						// More extractions to process
						if err := p.processConsecutiveExtractionsForDeletion(extractedValue, extractionSegments[1:], remainingSegments); err != nil {
						return err
					}
					}
				}
			}
		}
	}

	return nil
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

func (p *Processor) getValueWithDistributedOperation(data any, path string) (any, error) {
	// Parse the path to identify distributed operation patterns
	segments := p.getPathSegments()
	defer p.putPathSegments(segments)

	segments = p.splitPath(path, segments)

	// Find the extraction segment that triggers distributed operation
	extractionIndex := -1
	for i, segment := range segments {
		if segment.Type == internal.ExtractSegment {
			// Check if this is followed by array operations
			if i+1 < len(segments) {
				nextSegment := segments[i+1]
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
	preSegments := segments[:extractionIndex]
	extractionSegment := segments[extractionIndex]
	postSegments := segments[extractionIndex+1:]

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

// setValueAtPath sets a value at the specified path
func (p *Processor) setValueAtPath(data any, path string, value any) error {
	return p.setValueAtPathWithOptions(data, path, value, false)
}

func (p *Processor) setValueAtPathWithOptions(data any, path string, value any, createPaths bool) error {
	if path == "" || path == "." {
		return fmt.Errorf("cannot set root value")
	}

	// Use advanced path parsing for full feature support
	return p.setValueAdvancedPath(data, path, value, createPaths)
}

func (p *Processor) setValueAdvancedPath(data any, path string, value any, createPaths bool) error {
	// Handle JSON Pointer format first
	if strings.HasPrefix(path, "/") {
		if createPaths {
			return p.setValueJSONPointerWithCreation(data, path, value)
		}
		return p.setValueJSONPointer(data, path, value)
	}

	// Check for append syntax [+] - this takes priority
	if strings.Contains(path, "[+]") {
		return p.setValueDotNotationWithCreation(data, path, value, createPaths)
	}

	// Check if this is a simple array index access that might need extension
	if createPaths && p.isSimpleArrayIndexPath(path) {
		// Use dot notation handler for simple array index access with extension support
		return p.setValueDotNotationWithCreation(data, path, value, createPaths)
	}

	// Check if this is a complex path that should use RecursiveProcessor
	// But exclude simple array slice paths that need array extension support
	if p.isComplexPath(path) && !p.isSimpleArraySlicePath(path) {
		// Use cached RecursiveProcessor for complex paths like flat extraction
		_, err := p.recursiveProcessor.ProcessRecursivelyWithOptions(data, path, opSet, value, createPaths)
		return err
	}

	// Use dot notation with segments for simple paths
	return p.setValueDotNotationWithCreation(data, path, value, createPaths)
}

func (p *Processor) isSimpleArraySlicePath(path string) bool {
	// Check for simple patterns like "property[start:end]" or "property.subprop[start:end]"
	// These should use legacy handling for array extension support

	// Must contain slice syntax
	if !strings.Contains(path, ":") {
		return false
	}

	// Must not contain extraction syntax (which needs RecursiveProcessor)
	if strings.Contains(path, "{") || strings.Contains(path, "}") {
		return false
	}

	// Check if it's a simple property.array[slice] pattern
	// Count the number of bracket pairs
	openBrackets := strings.Count(path, "[")
	closeBrackets := strings.Count(path, "]")

	// Should have exactly one bracket pair for simple slice
	if openBrackets != 1 || closeBrackets != 1 {
		return false
	}

	// Find the bracket positions
	bracketStart := strings.Index(path, "[")
	bracketEnd := strings.Index(path, "]")

	if bracketStart == -1 || bracketEnd == -1 || bracketEnd <= bracketStart {
		return false
	}

	// Extract the slice part
	slicePart := path[bracketStart+1 : bracketEnd]

	// Check if it's a valid slice syntax (contains colon)
	if !strings.Contains(slicePart, ":") {
		return false
	}

	// Check if the part before brackets is a simple property path (no complex operations)
	beforeBrackets := path[:bracketStart]
	if strings.Contains(beforeBrackets, "{") || strings.Contains(beforeBrackets, "}") {
		return false
	}

	return true
}

func (p *Processor) isSimpleArrayIndexPath(path string) bool {
	// Must contain array index syntax
	if !strings.Contains(path, "[") || !strings.Contains(path, "]") {
		return false
	}

	// Must not contain slice syntax (colons)
	if strings.Contains(path, ":") {
		return false
	}

	// Must not contain extraction syntax
	if strings.Contains(path, "{") || strings.Contains(path, "}") {
		return false
	}

	// Check if it's a simple pattern like "property[index]" or "property.subprop[index]"
	// Count the number of bracket pairs
	openBrackets := strings.Count(path, "[")
	closeBrackets := strings.Count(path, "]")

	// Should have exactly one bracket pair for simple index access
	if openBrackets != 1 || closeBrackets != 1 {
		return false
	}

	// Find the bracket positions
	bracketStart := strings.Index(path, "[")
	bracketEnd := strings.Index(path, "]")

	if bracketStart == -1 || bracketEnd == -1 || bracketEnd <= bracketStart {
		return false
	}

	return true
}

func (p *Processor) setValueWithSegments(data any, segments []internal.PathSegment, value any, createPaths bool) error {
	if len(segments) == 0 {
		return fmt.Errorf("no segments provided")
	}

	// Check if the last segment is an append operation
	finalSegment := segments[len(segments)-1]
	if finalSegment.Type == internal.AppendSegment {
		return p.handleAppendOperation(data, segments, value, createPaths)
	}

	// Navigate to the parent of the target
	current := data
	for i := 0; i < len(segments)-1; i++ {
		next, err := p.navigateToSegment(current, segments[i], createPaths, segments, i)
		if err != nil {
			return err
		}
		current = next
	}

	// Special handling for array index or slice access that might need extension
	if createPaths && (finalSegment.Type == internal.ArrayIndexSegment || finalSegment.Type == internal.ArraySliceSegment) {
		return p.setValueForArrayIndexWithExtension(current, finalSegment, value, data, segments)
	}

	err := p.setValueForSegment(current, finalSegment, value, createPaths)

	// Handle array extension error
	if arrayExtErr, ok := err.(*arrayExtensionSignal); ok && createPaths {
		// We need to extend the array and then set the values
		return p.handleArrayExtensionAndSet(data, segments, arrayExtErr)
	}

	return err
}

// handleAppendOperation handles the [+] append syntax
// It navigates to the parent container and appends the value to the array
func (p *Processor) handleAppendOperation(data any, segments []internal.PathSegment, value any, createPaths bool) error {
	if len(segments) < 2 {
		return fmt.Errorf("append operation requires a parent path before [+]")
	}

	// Navigate to the parent container (everything before [+])
	current := data
	for i := 0; i < len(segments)-1; i++ {
		next, err := p.navigateToSegment(current, segments[i], createPaths, segments, i)
		if err != nil {
			return err
		}
		current = next
	}

	// Now current should be the array we want to append to
	arr, ok := current.([]any)
	if !ok {
		return fmt.Errorf("cannot append to non-array type %T", current)
	}

	// Get the parent container to update the array reference
	// Navigate to parent of the array
	parent := data
	parentSegments := segments[:len(segments)-1]
	if len(parentSegments) == 0 {
		return fmt.Errorf("cannot determine parent container for append operation")
	}

	// Navigate to the parent of the array
	for i := 0; i < len(parentSegments)-1; i++ {
		next, err := p.navigateToSegment(parent, parentSegments[i], createPaths, parentSegments, i)
		if err != nil {
			return err
		}
		parent = next
	}

	// Get the last segment that identifies the array in its parent
	arraySegment := parentSegments[len(parentSegments)-1]

	// Perform the append and update the parent container
	return p.appendAndSetParent(parent, arraySegment, arr, value)
}

// appendAndSetParent appends value to arr and updates the parent container
func (p *Processor) appendAndSetParent(parent any, arraySegment internal.PathSegment, arr []any, value any) error {
	// Append the value(s) to the array
	switch v := value.(type) {
	case []any:
		// Expand slice and append all elements
		arr = append(arr, v...)
	default:
		// Append single value
		arr = append(arr, v)
	}

	// Now set the updated array back to the parent
	switch p := parent.(type) {
	case map[string]any:
		p[arraySegment.Key] = arr
		return nil
	case map[any]any:
		p[arraySegment.Key] = arr
		return nil
	case []any:
		// Parent is an array, so arraySegment should be an index
		if arraySegment.Type == internal.ArrayIndexSegment {
			idx := arraySegment.Index
			if idx >= 0 && idx < len(p) {
				p[idx] = arr
				return nil
			}
			return fmt.Errorf("array index %d out of bounds for append operation", idx)
		}
		return fmt.Errorf("invalid segment type for array parent in append operation")
	default:
		return fmt.Errorf("cannot update array in parent type %T", parent)
	}
}

func (p *Processor) setValueDotNotationWithCreation(data any, path string, value any, createPaths bool) error {
	// Parse path into segments
	segments := p.getPathSegments()
	defer p.putPathSegments(segments)

	segments = p.splitPath(path, segments)

	return p.setValueWithSegments(data, segments, value, createPaths)
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

func (p *Processor) setValueForSegment(current any, segment internal.PathSegment, value any, createPaths bool) error {
	switch segment.Type {
	case internal.PropertySegment:
		return p.setValueForProperty(current, segment.Key, value, createPaths)
	case internal.ArrayIndexSegment:
		index := segment.Index
		return p.setValueForArrayIndex(current, index, value, createPaths)
	case internal.ArraySliceSegment:
		return p.setValueForArraySlice(current, segment, value, createPaths)
	case internal.ExtractSegment:
		return p.setValueForExtract(current, segment, value, createPaths)
	default:
		return fmt.Errorf("unsupported segment type for set: %v", segment.Type.String())
	}
}

func (p *Processor) setValueForProperty(current any, property string, value any, createPaths bool) error {
	switch v := current.(type) {
	case map[string]any:
		v[property] = value
		return nil
	case map[any]any:
		v[property] = value
		return nil
	default:
		if createPaths {
			// Cannot convert non-map types to map for property setting
			// This is a fundamental limitation
			return fmt.Errorf("cannot convert %T to map for property setting", current)
		}
		return fmt.Errorf("cannot set property '%s' on type %T", property, current)
	}
}

// Array extension and index/slice operations

func (p *Processor) handleArrayExtensionAndSet(data any, segments []internal.PathSegment, arrayExtErr *arrayExtensionSignal) error {
	if len(segments) == 0 {
		return fmt.Errorf("no segments provided for array extension")
	}

	// Navigate to the parent of the array that needs extension
	current := data
	for i := 0; i < len(segments)-1; i++ {
		next, err := p.navigateToSegment(current, segments[i], true, segments, i)
		if err != nil {
			return fmt.Errorf("failed to navigate to segment %d during array extension: %w", i, err)
		}
		current = next
	}

	// Get the final segment (can be array or slice)
	finalSegment := segments[len(segments)-1]

	switch finalSegment.Type {
	case internal.ArrayIndexSegment:
		// Handle simple array index extension
		return p.handleArrayIndexExtension(current, finalSegment, arrayExtErr)
	case internal.ArraySliceSegment:
		// Handle array slice extension
		return p.handleArraySliceExtension(current, finalSegment, arrayExtErr)
	default:
		return fmt.Errorf("expected array or slice segment for array extension, got %s", finalSegment.Type.String())
	}
}

func (p *Processor) handleArrayIndexExtension(current any, _ internal.PathSegment, arrayExtErr *arrayExtensionSignal) error {
	// For array index access, current should be the array that needs extension
	arr, ok := current.([]any)
	if !ok {
		return fmt.Errorf("expected array for index extension, got %T", current)
	}

	// Try to extend the slice in place if there is enough capacity
	if cap(arr) >= arrayExtErr.requiredLength {
		for len(arr) < arrayExtErr.requiredLength {
			arr = append(arr, nil)
		}
		arr[arrayExtErr.start] = arrayExtErr.value
		return nil
	}

	// Cannot extend in place - this is a fundamental limitation
	// We need to signal that the parent should handle this
	return fmt.Errorf("cannot extend array in place for index %d", arrayExtErr.start)
}

func (p *Processor) handleArraySliceExtension(parent any, _ internal.PathSegment, arrayExtErr *arrayExtensionSignal) error {
	// Get the array that needs extension
	arr, ok := parent.([]any)
	if !ok {
		return fmt.Errorf("expected array for slice extension, got %T", parent)
	}

	// Create extended array
	extendedArr := make([]any, arrayExtErr.requiredLength)
	copy(extendedArr, arr)

	// Set values in the extended array
	for i := arrayExtErr.start; i < arrayExtErr.end; i += arrayExtErr.step {
		if i >= 0 && i < len(extendedArr) {
			extendedArr[i] = arrayExtErr.value
		}
	}

	// For slice operations, we can't easily replace the parent array
	// This is a limitation of the current approach
	return fmt.Errorf("slice array extension not fully supported yet")
}

func (p *Processor) setValueForArrayIndexWithExtension(current any, segment internal.PathSegment, value any, rootData any, segments []internal.PathSegment) error {
	switch segment.Type {
	case internal.ArrayIndexSegment:
		return p.setValueForArrayIndexWithAutoExtension(current, segment, value, rootData, segments)
	case internal.ArraySliceSegment:
		return p.setValueForArraySliceWithAutoExtension(current, segment, value, rootData, segments)
	default:
		return fmt.Errorf("unsupported segment type for array extension: %s", segment.Type.String())
	}
}

func (p *Processor) setValueForArrayIndexWithAutoExtension(current any, segment internal.PathSegment, value any, rootData any, segments []internal.PathSegment) error {
	// Get the array index from the segment
	index := segment.Index

	switch v := current.(type) {
	case []any:
		idx, err := normalizeNegativeIndexAllowExtend(index, len(v))
		if err != nil {
			return err
		}

		if idx >= len(v) {
			// Need to extend the array - find the parent and replace the array
			return p.extendArrayAndSetValue(rootData, segments, idx, value)
		}

		// Set value within bounds
		v[idx] = value
		return nil

	default:
		return fmt.Errorf("cannot set array index %d on type %T", index, current)
	}
}

func (p *Processor) setValueForArraySliceWithAutoExtension(current any, segment internal.PathSegment, value any, rootData any, segments []internal.PathSegment) error {
	arr, ok := current.([]any)
	if !ok {
		return fmt.Errorf("cannot set slice on type %T", current)
	}

	// Get slice parameters
	start, end, step := p.getSliceParameters(segment, len(arr))

	// Check if we need to extend the array
	maxIndex := end - 1
	if maxIndex >= len(arr) {
		// Need to extend the array
		return p.extendArrayAndSetSliceValue(rootData, segments, start, end, step, value)
	}

	// Set values within bounds
	for i := start; i < end; i += step {
		if i >= 0 && i < len(arr) {
			arr[i] = value
		}
	}

	return nil
}

func (p *Processor) getSliceParameters(segment internal.PathSegment, arrayLength int) (start, end, step int) {
	// Default values
	start = 0
	end = arrayLength
	step = 1

	// Get start
	if segment.HasStart() {
		start = segment.Index // Index stores start for slices
		if start < 0 {
			start = arrayLength + start
		}
	}

	// Get end
	if segment.HasEnd() {
		end = segment.End
		if end < 0 {
			end = arrayLength + end
		}
	}

	// Get step
	if segment.HasStep() {
		step = segment.Step
	}

	// Ensure step is positive for extension purposes
	if step <= 0 {
		step = 1
	}

	// Ensure start is non-negative
	if start < 0 {
		start = 0
	}

	return start, end, step
}

func (p *Processor) extendArrayAndSetSliceValue(rootData any, segments []internal.PathSegment, start, end, step int, value any) error {
	if len(segments) == 0 {
		return fmt.Errorf("no segments provided")
	}

	// For array extension, we need to navigate to the parent of the array container
	current := rootData
	for i := 0; i < len(segments)-2; i++ {
		next, err := p.navigateToSegment(current, segments[i], true, segments, i)
		if err != nil {
			return fmt.Errorf("failed to navigate to segment %d: %w", i, err)
		}
		current = next
	}

	// Get the array container segment and the slice access segment
	var arrayContainerSegment, sliceAccessSegment internal.PathSegment
	if len(segments) >= 2 {
		arrayContainerSegment = segments[len(segments)-2]
		sliceAccessSegment = segments[len(segments)-1]
	} else {
		// Single segment case - the array is at root level
		sliceAccessSegment = segments[0]
	}

	// Handle different parent types
	switch v := current.(type) {
	case map[string]any:
		// Get the property name from the array container segment
		propertyName := arrayContainerSegment.Key
		if propertyName == "" && len(segments) == 1 {
			// Single segment case - extract property name from slice access segment
			propertyName = sliceAccessSegment.Key
		}

		// Get or create the array
		var currentArr []any
		if existingArr, ok := v[propertyName].([]any); ok {
			currentArr = existingArr
		} else {
			currentArr = []any{}
		}

		// Create extended array
		extendedArr := make([]any, end)
		copy(extendedArr, currentArr)

		// Set values in the slice range
		for i := start; i < end; i += step {
			if i >= 0 && i < len(extendedArr) {
				extendedArr[i] = value
			}
		}

		// Replace the array in parent
		v[propertyName] = extendedArr
		return nil

	case []any:
		// Parent is array - this would be for nested array access
		parentIndex := arrayContainerSegment.Index
		if parentIndex >= 0 && parentIndex < len(v) {
			if nestedArr, ok := v[parentIndex].([]any); ok {
				// Create extended nested array
				extendedArr := make([]any, end)
				copy(extendedArr, nestedArr)

				// Set values in the slice range
				for i := start; i < end; i += step {
					if i >= 0 && i < len(extendedArr) {
						extendedArr[i] = value
					}
				}

				// Replace the nested array
				v[parentIndex] = extendedArr
				return nil
			}
		}
		return fmt.Errorf("cannot extend nested array at index %d", parentIndex)

	default:
		return fmt.Errorf("cannot extend array in parent of type %T", current)
	}
}

func (p *Processor) extendArrayAndSetValue(rootData any, segments []internal.PathSegment, targetIndex int, value any) error {
	if len(segments) == 0 {
		return fmt.Errorf("no segments provided")
	}

	// For array extension, we need to navigate to the parent of the array container
	// not the array itself. So we navigate to len(segments)-2 instead of len(segments)-1
	current := rootData
	for i := 0; i < len(segments)-2; i++ {
		next, err := p.navigateToSegment(current, segments[i], true, segments, i)
		if err != nil {
			return fmt.Errorf("failed to navigate to segment %d: %w", i, err)
		}
		current = next
	}

	// Get the array container segment and the array access segment
	var arrayContainerSegment, arrayAccessSegment internal.PathSegment
	if len(segments) >= 2 {
		arrayContainerSegment = segments[len(segments)-2]
		arrayAccessSegment = segments[len(segments)-1]
	} else {
		// Single segment case - the array is at root level
		arrayAccessSegment = segments[0]
	}

	// Handle different parent types
	switch v := current.(type) {
	case map[string]any:
		// Get the property name from the array container segment
		propertyName := arrayContainerSegment.Key
		if propertyName == "" && len(segments) == 1 {
			// Single segment case - extract property name from array access segment
			propertyName = arrayAccessSegment.Key
			if propertyName == "" {
				propertyName = arrayAccessSegment.String()
				if strings.Contains(propertyName, "[") {
					bracketIndex := strings.Index(propertyName, "[")
					propertyName = propertyName[:bracketIndex]
				}
			}
		}

		// Get or create the array
		var currentArr []any
		if existingArr, ok := v[propertyName].([]any); ok {
			currentArr = existingArr
		} else {
			currentArr = []any{}
		}

		// Create extended array
		extendedArr := make([]any, targetIndex+1)
		copy(extendedArr, currentArr)
		extendedArr[targetIndex] = value

		// Replace the array in parent
		v[propertyName] = extendedArr
		return nil

	case []any:
		// Parent is array - this would be for nested array access
		// The arrayContainerSegment.Index should give us the parent array index
		parentIndex := arrayContainerSegment.Index
		if parentIndex >= 0 && parentIndex < len(v) {
			if nestedArr, ok := v[parentIndex].([]any); ok {
				// Create extended nested array
				extendedArr := make([]any, targetIndex+1)
				copy(extendedArr, nestedArr)
				extendedArr[targetIndex] = value

				// Replace the nested array
				v[parentIndex] = extendedArr
				return nil
			}
		}
		return fmt.Errorf("cannot extend nested array at index %d", parentIndex)

	default:
		return fmt.Errorf("cannot extend array in parent of type %T", current)
	}
}

func (p *Processor) setValueForArrayIndex(current any, index int, value any, createPaths bool) error {
	switch v := current.(type) {
	case []any:
		idx, err := normalizeNegativeIndexAllowExtend(index, len(v))
		if err != nil {
			return err
		}

		if idx >= len(v) {
			if createPaths {
				// Return arrayExtensionSignal to signal parent needs to handle extension
				return &arrayExtensionSignal{
					requiredLength: idx + 1,
					currentLength:  len(v),
					start:          idx,
					end:            idx + 1,
					step:           1,
					value:          value,
				}
			}
			return fmt.Errorf("array index %d out of bounds (length %d)", idx, len(v))
		}

		v[idx] = value
		return nil
	default:
		return fmt.Errorf("cannot set array index %d on type %T", index, current)
	}
}

func (p *Processor) setValueForArraySlice(current any, segment internal.PathSegment, value any, createPaths bool) error {
	// This method is called on the array itself, so we need to handle array extension differently
	// The problem is that we can't modify the parent reference from here
	// We need to return an error that indicates array extension is needed

	arr, ok := current.([]any)
	if !ok {
		return fmt.Errorf("cannot perform slice operation on type %T", current)
	}

	// Use slice parameters from segment
	start := 0
	end := len(arr)
	step := 1

	if segment.HasStart() {
		start = segment.Index // Index stores start for slices
	}
	if segment.HasEnd() {
		end = segment.End
	}
	if segment.HasStep() {
		step = segment.Step
	}

	// Handle negative indices
	if start < 0 {
		start = len(arr) + start
	}
	if end < 0 {
		end = len(arr) + end
	}

	// Bounds checking
	if start < 0 {
		start = 0
	}

	// Check if we need to extend the array
	if end > len(arr) {
		if !createPaths {
			return fmt.Errorf("slice end %d out of bounds for array length %d", end, len(arr))
		}
		// For array extension, we need to signal that the parent needs to handle this
		return &arrayExtensionSignal{
			requiredLength: end,
			currentLength:  len(arr),
			start:          start,
			end:            end,
			step:           step,
			value:          value,
		}
	}

	if start >= end {
		return fmt.Errorf("invalid slice range [%d:%d]", start, end)
	}

	// Assign value to slice (within current bounds)
	return p.assignValueToSlice(arr, start, end, step, value)
}

// Navigation methods for path traversal

func (p *Processor) navigateToSegment(current any, segment internal.PathSegment, createPaths bool, allSegments []internal.PathSegment, currentIndex int) (any, error) {
	switch segment.Type {
	case internal.PropertySegment:
		return p.navigateToProperty(current, segment.Key, createPaths, allSegments, currentIndex)
	case internal.ArrayIndexSegment:
		// Get array index from segment
		index := segment.Index
		return p.navigateToArrayIndexWithNegative(current, index, createPaths)
	case internal.ArraySliceSegment:
		// Check if this is the last segment before an extract operation
		if currentIndex+1 < len(allSegments) && allSegments[currentIndex+1].Type == internal.ExtractSegment {
			// This is a slice followed by extract - return the current array for slice processing
			return current, nil
		}
		// For other cases, array slices are not supported as intermediate paths
		return nil, fmt.Errorf("array slice not supported as intermediate path segment")
	case internal.ExtractSegment:
		// Handle extract operations as intermediate path segments
		return p.navigateToExtraction(current, segment, createPaths, allSegments, currentIndex)
	default:
		return nil, fmt.Errorf("unsupported segment type: %v", segment.Type.String())
	}
}

func (p *Processor) navigateToExtraction(current any, segment internal.PathSegment, createPaths bool, allSegments []internal.PathSegment, currentIndex int) (any, error) {
	field := segment.Key
	if field == "" {
		return nil, fmt.Errorf("invalid extraction syntax: %s", segment.String())
	}

	// For set operations on extractions, we need to handle this differently
	// This is a complex case that might require distributed operations
	if _, ok := current.([]any); ok {
		// For arrays, we need to set values in each extracted field
		// This is handled by distributed operations
		return current, nil
	}

	// For single objects, extract the field
	if obj, ok := current.(map[string]any); ok {
		if value := p.handlePropertyAccessValue(obj, field); value != nil {
			return value, nil
		}
		if createPaths {
			// Create the field if it doesn't exist
			newContainer, err := p.createContainerForNextSegment(allSegments, currentIndex)
			if err != nil {
				return nil, err
			}
			obj[field] = newContainer
			return newContainer, nil
		}
	}

	return nil, fmt.Errorf("extraction field '%s' not found", field)
}

func (p *Processor) navigateToProperty(current any, property string, createPaths bool, allSegments []internal.PathSegment, currentIndex int) (any, error) {
	switch v := current.(type) {
	case map[string]any:
		if val, exists := v[property]; exists {
			return val, nil
		}
		if createPaths {
			// Create missing property
			newContainer, err := p.createContainerForNextSegment(allSegments, currentIndex)
			if err != nil {
				return nil, err
			}
			v[property] = newContainer
			return newContainer, nil
		}
		return nil, fmt.Errorf("property '%s' not found", property)
	case map[any]any:
		if val, exists := v[property]; exists {
			return val, nil
		}
		if createPaths {
			newContainer, err := p.createContainerForNextSegment(allSegments, currentIndex)
			if err != nil {
				return nil, err
			}
			v[property] = newContainer
			return newContainer, nil
		}
		return nil, fmt.Errorf("property '%s' not found", property)
	default:
		return nil, fmt.Errorf("cannot access property '%s' on type %T", property, current)
	}
}

func (p *Processor) createContainerForNextSegment(allSegments []internal.PathSegment, currentIndex int) (any, error) {
	if currentIndex+1 >= len(allSegments) {
		// This is the last segment, return nil (will be replaced by the actual value)
		return nil, nil
	}

	nextSegment := allSegments[currentIndex+1]
	switch nextSegment.Type {
	case internal.PropertySegment, internal.ExtractSegment:
		return make(map[string]any), nil
	case internal.ArrayIndexSegment:
		// For array access, create an empty array that can be extended
		return make([]any, 0), nil
	case internal.ArraySliceSegment:
		// For slice access, we need to create an array large enough for the slice
		end := 0
		if nextSegment.HasEnd() {
			end = nextSegment.End
		}
		if end > 0 {
			return make([]any, end), nil
		}
		return make([]any, 0), nil
	default:
		return make(map[string]any), nil // Default to object
	}
}

// Extraction-related set operations

func (p *Processor) setValueForExtract(current any, segment internal.PathSegment, value any, _ bool) error {
	field := segment.Key
	if field == "" {
		return fmt.Errorf("invalid extraction syntax: %s", segment.String())
	}

	// Handle array extraction
	if arr, ok := current.([]any); ok {
		if segment.IsFlatExtract() {
			return p.setValueForArrayExtractFlat(arr, field, value)
		} else {
			return p.setValueForArrayExtract(arr, field, value)
		}
	}

	// Handle single object
	if obj, ok := current.(map[string]any); ok {
		obj[field] = value
		return nil
	}

	return fmt.Errorf("cannot perform extraction set on type %T", current)
}

func (p *Processor) setValueForArrayExtract(arr []any, extractKey string, value any) error {
	for i, item := range arr {
		if obj, ok := item.(map[string]any); ok {
			obj[extractKey] = value
		} else {
			// Create new object if item is not a map
			newObj := map[string]any{extractKey: value}
			arr[i] = newObj
		}
	}
	return nil
}

func (p *Processor) setValueForArrayExtractFlat(arr []any, extractKey string, value any) error {
	// For flat extraction, we need to handle nested arrays
	for i, item := range arr {
		if obj, ok := item.(map[string]any); ok {
			// Check if the field contains an array that should be flattened
			if existingValue, exists := obj[extractKey]; exists {
				if existingArr, ok := existingValue.([]any); ok {
					// Flatten the value into the existing array
					if valueArr, ok := value.([]any); ok {
						// Merge arrays
						existingArr = append(existingArr, valueArr...)
						obj[extractKey] = existingArr
					} else {
						// Add single value to array
						existingArr = append(existingArr, value)
						obj[extractKey] = existingArr
					}
				} else {
					// Convert existing value to array and add new value
					newArr := []any{existingValue}
					if valueArr, ok := value.([]any); ok {
						newArr = append(newArr, valueArr...)
					} else {
						newArr = append(newArr, value)
					}
					obj[extractKey] = newArr
				}
			} else {
				// Create new field
				if valueArr, ok := value.([]any); ok {
					obj[extractKey] = valueArr
				} else {
					obj[extractKey] = []any{value}
				}
			}
		} else {
			// Create new object with array field
			var newValue any
			if valueArr, ok := value.([]any); ok {
				newValue = valueArr
			} else {
				newValue = []any{value}
			}
			newObj := map[string]any{extractKey: newValue}
			arr[i] = newObj
		}
	}
	return nil
}

func (p *Processor) setValueJSONPointer(data any, path string, value any) error {
	return p.setValueJSONPointerWithCreation(data, path, value)
}

func (p *Processor) setValueJSONPointerWithCreation(data any, path string, value any) error {
	if path == "/" {
		return fmt.Errorf("cannot set root value")
	}

	// Remove leading slash and split
	pathWithoutSlash := path[1:]
	segments := strings.Split(pathWithoutSlash, "/")

	// Handle array extension for JSON Pointer
	return p.setValueJSONPointerWithArrayExtension(data, segments, value)
}

func (p *Processor) setValueJSONPointerWithArrayExtension(data any, segments []string, value any) error {
	if len(segments) == 0 {
		return fmt.Errorf("no segments provided")
	}

	// Navigate to parent segments
	current := data
	for i := 0; i < len(segments)-1; i++ {
		segment := segments[i]

		// Unescape JSON Pointer characters
		if strings.Contains(segment, "~") {
			segment = p.unescapeJSONPointer(segment)
		}

		next, err := p.createPathSegmentForJSONPointerWithExtension(current, segment, segments, i)
		if err != nil {
			return err
		}
		current = next
	}

	// Set final value
	finalSegment := segments[len(segments)-1]
	if strings.Contains(finalSegment, "~") {
		finalSegment = internal.UnescapeJSONPointer(finalSegment)
	}

	return p.setJSONPointerFinalValue(current, finalSegment, value)
}

func (p *Processor) createPathSegmentForJSONPointerWithExtension(current any, segment string, allSegments []string, currentIndex int) (any, error) {
	switch v := current.(type) {
	case map[string]any:
		if val, exists := v[segment]; exists {
			return val, nil
		}
		// Create missing property
		var newContainer any
		if currentIndex+1 < len(allSegments) {
			nextSegment := allSegments[currentIndex+1]
			if p.isArrayIndex(nextSegment) {
				newContainer = make([]any, 0)
			} else {
				newContainer = make(map[string]any)
			}
		} else {
			newContainer = make(map[string]any)
		}
		v[segment] = newContainer
		return newContainer, nil

	case []any:
		if index, err := strconv.Atoi(segment); err == nil {
			if index >= 0 && index < len(v) {
				return v[index], nil
			}
			if index >= len(v) {
				// Extend array to accommodate the index
				extendedArr := make([]any, index+1)
				copy(extendedArr, v)

				// Determine what to put at the target index
				var newContainer any
				if currentIndex+1 < len(allSegments) {
					nextSegment := allSegments[currentIndex+1]
					if p.isArrayIndex(nextSegment) {
						newContainer = make([]any, 0)
					} else {
						newContainer = make(map[string]any)
					}
				} else {
					newContainer = nil
				}

				extendedArr[index] = newContainer

				// Replace the array in the parent - we need to find the parent
				// This is a complex operation that requires tracking the parent
				return p.replaceArrayInJSONPointerParent(current, v, extendedArr, index, newContainer)
			}
		}
		return nil, fmt.Errorf("invalid array index for JSON Pointer: %s", segment)

	default:
		return nil, fmt.Errorf("cannot navigate through %T with segment %s", current, segment)
	}
}

func (p *Processor) setJSONPointerFinalValue(current any, segment string, value any) error {
	switch v := current.(type) {
	case map[string]any:
		v[segment] = value
		return nil
	case []any:
		if index, err := strconv.Atoi(segment); err == nil {
			if index >= 0 && index < len(v) {
				v[index] = value
				return nil
			}
			if index >= len(v) {
				// Extend array to accommodate the index
				extendedArr := make([]any, index+1)
				copy(extendedArr, v)
				extendedArr[index] = value

				// Try to replace in place if possible
				if cap(v) >= len(extendedArr) {
					for i := len(v); i < len(extendedArr); i++ {
						v = append(v, nil)
					}
					v[index] = value
					return nil
				}

				// Cannot extend in place — return error to prevent silent data loss
				return &JsonsError{
					Op:      "array_extension",
					Path:    segment,
					Message: fmt.Sprintf("cannot extend array in place for index %d", index),
					Err:     errOperationFailed,
				}
			}
		}
		return fmt.Errorf("invalid array index: %s", segment)
	default:
		return fmt.Errorf("cannot set value on type %T", current)
	}
}

func (p *Processor) replaceArrayInJSONPointerParent(_ any, oldArray, newArray []any, index int, newContainer any) (any, error) {
	// This is a complex operation that would require tracking parent references
	// For now, we'll try to extend in place if possible
	if cap(oldArray) >= len(newArray) {
		for i := len(oldArray); i < len(newArray); i++ {
			oldArray = append(oldArray, nil)
		}
		oldArray[index] = newContainer
		return newContainer, nil
	}

	// Cannot extend in place — return error to prevent silent data loss
	return nil, &JsonsError{
		Op:      "array_extension",
		Message: fmt.Sprintf("cannot extend array in place: need cap %d, have %d", len(newArray), cap(oldArray)),
		Err:     errOperationFailed,
	}
}

// ============================================================================
// OPTIMIZED SET/DELETE OPERATIONS
// Reduces allocations for common JSON modification operations
// ============================================================================

// resultBufferPool pools byte slices for marshaling results
var resultBufferPool = sync.Pool{
	New: func() any {
		buf := make([]byte, 0, 1024)
		return &buf
	},
}

// getResultBuffer gets a buffer for result marshaling
func getResultBuffer() *[]byte {
	buf := resultBufferPool.Get().(*[]byte)
	*buf = (*buf)[:0]
	return buf
}

// putResultBuffer returns a buffer to the pool
func putResultBuffer(buf *[]byte) {
	if cap(*buf) <= 16*1024 {
		resultBufferPool.Put(buf)
	}
}

// fastSet is an optimized Set operation for simple paths.
// EXPERIMENTAL: Not yet integrated into production Set path; used by benchmarks.
// Uses pooled resources and optimized marshaling.
func (p *Processor) fastSet(jsonStr, path string, value any) (string, error) {
	if err := p.checkClosed(); err != nil {
		return jsonStr, err
	}

	// Fast path for simple property access
	if isSimplePropertyAccess(path) {
		return p.fastSetSimple(jsonStr, path, value)
	}

	// Fall back to standard Set for complex paths
	return p.Set(jsonStr, path, value)
}

// fastSetSimple handles simple single-level property access
// PERFORMANCE: Uses FastMarshalToString to reduce allocations
func (p *Processor) fastSetSimple(jsonStr, key string, value any) (string, error) {
	// Quick validation
	if err := p.validateInput(jsonStr); err != nil {
		return jsonStr, err
	}

	// Parse JSON
	var data any
	if err := p.Parse(jsonStr, &data); err != nil {
		return jsonStr, err
	}

	// Fast path for map[string]any
	obj, ok := data.(map[string]any)
	if !ok {
		return p.Set(jsonStr, key, value) // Fall back to standard
	}

	// Direct set
	obj[key] = value

	// PERFORMANCE: Use FastMarshalToString to avoid double allocation
	result, err := internal.FastMarshalToString(obj)
	if err != nil {
		return jsonStr, err
	}

	return result, nil
}

// fastDelete is an optimized Delete operation for simple paths.
// EXPERIMENTAL: Not yet integrated into production Delete path; used by benchmarks.
func (p *Processor) fastDelete(jsonStr, path string) (string, error) {
	if err := p.checkClosed(); err != nil {
		return jsonStr, err
	}

	// Fast path for simple property access
	if isSimplePropertyAccess(path) {
		return p.fastDeleteSimple(jsonStr, path)
	}

	// Fall back to standard Delete for complex paths
	return p.Delete(jsonStr, path)
}

// fastDeleteSimple handles simple single-level property deletion
// PERFORMANCE: Uses FastMarshalToString to reduce allocations
func (p *Processor) fastDeleteSimple(jsonStr, key string) (string, error) {
	// Quick validation
	if err := p.validateInput(jsonStr); err != nil {
		return jsonStr, err
	}

	// Parse JSON
	var data any
	if err := p.Parse(jsonStr, &data); err != nil {
		return jsonStr, err
	}

	// Fast path for map[string]any
	obj, ok := data.(map[string]any)
	if !ok {
		return p.Delete(jsonStr, key) // Fall back to standard
	}

	// Direct delete
	delete(obj, key)

	// PERFORMANCE: Use FastMarshalToString to avoid double allocation
	result, err := internal.FastMarshalToString(obj)
	if err != nil {
		return jsonStr, err
	}

	return result, nil
}

// batchSetOptimized performs multiple Set operations efficiently.
// EXPERIMENTAL: Not yet integrated into production SetMultiple path; used by benchmarks.
// PERFORMANCE: Uses pooled encoder and single-parse optimization
func (p *Processor) batchSetOptimized(jsonStr string, updates map[string]any) (string, error) {
	if err := p.checkClosed(); err != nil {
		return jsonStr, err
	}

	if len(updates) == 0 {
		return jsonStr, nil
	}

	// Parse JSON once
	var data any
	if err := p.Parse(jsonStr, &data); err != nil {
		return jsonStr, err
	}

	// Apply all updates
	for path, value := range updates {
		if isSimplePropertyAccess(path) {
			if obj, ok := data.(map[string]any); ok {
				obj[path] = value
				continue
			}
		}

		// Use standard set for complex paths
		if err := p.setValueAtPath(data, path, value); err != nil {
			return jsonStr, err
		}
	}

	// PERFORMANCE: Use FastMarshalToString to avoid double allocation (bytes -> string)
	result, err := internal.FastMarshalToString(data)
	if err != nil {
		return jsonStr, err
	}

	return result, nil
}

// batchDeleteOptimized performs multiple Delete operations efficiently.
// EXPERIMENTAL: Not yet integrated into production path; used by benchmarks.
// PERFORMANCE: Uses pooled encoder and single-parse optimization
func (p *Processor) batchDeleteOptimized(jsonStr string, paths []string) (string, error) {
	if err := p.checkClosed(); err != nil {
		return jsonStr, err
	}

	if len(paths) == 0 {
		return jsonStr, nil
	}

	// Parse JSON once
	var data any
	if err := p.Parse(jsonStr, &data); err != nil {
		return jsonStr, err
	}

	// Apply all deletions
	for _, path := range paths {
		if isSimplePropertyAccess(path) {
			if obj, ok := data.(map[string]any); ok {
				delete(obj, path)
				continue
			}
		}

		// Use standard delete for complex paths
		if err := p.deleteValueAtPath(data, path); err != nil {
			if !p.config.ContinueOnError {
				return jsonStr, err
			}
			// ContinueOnError: record error and proceed
			continue
		}
	}

	// PERFORMANCE: Use FastMarshalToString to avoid double allocation (bytes -> string)
	result, err := internal.FastMarshalToString(data)
	if err != nil {
		return jsonStr, err
	}

	return result, nil
}

// ============================================================================
// BULK GET OPERATIONS
// ============================================================================

// fastGetMultiple performs multiple Get operations with single parse.
// EXPERIMENTAL: Not yet integrated into production GetMultiple path; used by benchmarks.
// PERFORMANCE v3: Direct allocation with pre-sized map — avoids pool overhead and double-copy
func (p *Processor) fastGetMultiple(jsonStr string, paths []string) (map[string]any, error) {
	if err := p.checkClosed(); err != nil {
		return nil, err
	}

	if len(paths) == 0 {
		return map[string]any{}, nil
	}

	// Parse JSON once
	var data any
	if err := p.Parse(jsonStr, &data); err != nil {
		return nil, err
	}

	// Pre-allocate result map to exact size — single allocation, no pool overhead
	results := make(map[string]any, len(paths))

	for _, path := range paths {
		// Fast path for simple access
		if isSimplePropertyAccess(path) {
			if obj, ok := data.(map[string]any); ok {
				if val, exists := obj[path]; exists {
					results[path] = val
				}
				continue
			}
		}

		// Use navigation for complex paths
		val, err := p.navigateToPath(data, path)
		if err == nil {
			results[path] = val
		}
	}

	return results, nil
}
