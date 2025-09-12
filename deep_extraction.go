package json

import (
	"fmt"
	"strings"
)

// DeepExtractionResult represents the result of a deep extraction operation
type DeepExtractionResult struct {
	Values []any
	Paths  []string // Optional: track the paths where values were found
}

// handleDeepExtraction handles multiple consecutive extraction operations and mixed operations
// This solves the problem where a{g}{name} doesn't work as expected
func (p *Processor) handleDeepExtraction(data any, segments []PathSegment, startIndex int) (any, error) {
	if startIndex >= len(segments) {
		return data, nil
	}

	// Check if this is a single extraction followed by other operations
	if startIndex < len(segments) && segments[startIndex].TypeString() == "extract" {
		// Check if the next segment is not an extraction (mixed operation case)
		if startIndex+1 < len(segments) && segments[startIndex+1].TypeString() != "extract" {
			return p.handleMixedExtractionOperations(data, segments, startIndex)
		}
	}

	// Find all consecutive extract operations
	var extractSegments []PathSegment
	currentIndex := startIndex

	for currentIndex < len(segments) && segments[currentIndex].TypeString() == "extract" {
		extractSegments = append(extractSegments, segments[currentIndex])
		currentIndex++
	}

	if len(extractSegments) == 0 {
		return data, fmt.Errorf("no extract segments found")
	}

	// Apply consecutive extractions with proper flattening
	result, err := p.performConsecutiveExtractions(data, extractSegments)
	if err != nil {
		return nil, err
	}

	// If there are remaining segments after extractions, handle them properly
	if currentIndex < len(segments) {
		remainingSegments := segments[currentIndex:]

		// Check if the remaining segments are array operations (slice/index)
		// These need special handling for post-extraction operations
		if len(remainingSegments) > 0 {
			firstRemaining := remainingSegments[0]
			switch firstRemaining.TypeString() {
			case "slice":
				// For post-extraction slicing, we need to handle the structure correctly
				// If the extraction result is a flattened array, apply slice directly
				// If it's a structured array (array of arrays), we need different handling
				slicedResult := p.handlePostExtractionSliceWithStructureAwareness(result, firstRemaining, extractSegments)
				if slicedResult == nil {
					return nil, fmt.Errorf("failed to apply slice after extraction")
				}

				// If there are more segments after the slice, continue navigation
				if len(remainingSegments) > 1 {
					return p.navigateToPathWithSegments(slicedResult, remainingSegments[1:])
				}
				return slicedResult, nil

			case "array":
				// Apply array index operation to the extraction result
				indexedResult := p.handlePostExtractionArrayAccess(result, firstRemaining)
				if indexedResult == nil {
					return nil, fmt.Errorf("failed to apply array index after extraction")
				}

				// If there are more segments after the index, continue navigation
				if len(remainingSegments) > 1 {
					return p.navigateToPathWithSegments(indexedResult, remainingSegments[1:])
				}
				return indexedResult, nil

			default:
				// For other segment types, use regular navigation
				return p.navigateToPathWithSegments(result, remainingSegments)
			}
		}
	}

	return result, nil
}

// handlePostExtractionSliceWithStructureAwareness handles slice operations after extraction with structure awareness
func (p *Processor) handlePostExtractionSliceWithStructureAwareness(data any, sliceSegment PathSegment, extractSegments []PathSegment) any {
	arr, ok := data.([]any)
	if !ok {
		return nil
	}

	// Parse slice parameters
	start, end, step := p.parseSliceFromSegment(sliceSegment.Value)
	if start == -999999 { // Invalid slice
		return nil
	}

	// Check if any extraction segment is flat
	hasFlat := false
	for _, seg := range extractSegments {
		if seg.IsFlat {
			hasFlat = true
			break
		}
	}

	if hasFlat {
		// For flat extraction, apply slice to the flattened array with context
		return p.applySliceToArrayWithContext(arr, start, end, step, sliceSegment.Value)
	} else {
		// For non-flat extraction, the array contains structured results
		// Apply slice to the outer array (slice the collection of extracted results)
		slicedResults := p.applySliceToArrayWithContext(arr, start, end, step, sliceSegment.Value)
		return slicedResults
	}
}

// applySliceToArray applies slice operation to an array with proper bounds checking
func (p *Processor) applySliceToArray(arr []any, start, end, step int) []any {
	// Handle negative indices
	if start < 0 {
		start = len(arr) + start
	}
	// Handle negative end index
	if end < 0 {
		end = len(arr) + end
	}

	// Normalize bounds
	if start < 0 {
		start = 0
	}
	if end > len(arr) {
		end = len(arr)
	}
	if start > end {
		return []any{}
	}

	// Apply step if specified
	if step <= 1 {
		return arr[start:end]
	}

	// Handle step > 1
	var result []any
	for i := start; i < end; i += step {
		if i < len(arr) {
			result = append(result, arr[i])
		}
	}
	return result
}

// applySliceToArrayWithContext applies slice operation with segment context to distinguish open vs explicit negative slices
func (p *Processor) applySliceToArrayWithContext(arr []any, start, end, step int, segmentValue string) []any {
	// Handle negative indices
	if start < 0 {
		start = len(arr) + start
	}

	// Handle negative end index - distinguish between open slice and explicit negative
	if end < 0 {
		// Check if this is an open slice by examining the segment value
		isOpenSlice := strings.HasSuffix(segmentValue, ":") || strings.HasSuffix(segmentValue, ":]")

		if isOpenSlice {
			// Open slice like "[-1:]" - end should be array length
			end = len(arr)
		} else {
			// Explicit negative end index like "[-3:-1]" - convert to positive
			end = len(arr) + end
		}
	}

	// Normalize bounds
	if start < 0 {
		start = 0
	}
	if end > len(arr) {
		end = len(arr)
	}
	if start > end {
		return []any{}
	}

	// Apply step if specified
	if step <= 1 {
		return arr[start:end]
	}

	// Handle step > 1
	var result []any
	for i := start; i < end; i += step {
		if i < len(arr) {
			result = append(result, arr[i])
		}
	}
	return result
}

// handleMixedExtractionOperations handles extraction operations mixed with other operations
// This handles cases like {field}[0:2]{another} where extraction is followed by slice then extraction
func (p *Processor) handleMixedExtractionOperations(data any, segments []PathSegment, startIndex int) (any, error) {
	current := data

	for i := startIndex; i < len(segments); i++ {
		segment := segments[i]

		switch segment.TypeString() {
		case "extract":
			// Apply extraction - for mixed operations, we never preserve structure
			// to avoid extra wrapping that causes issues with subsequent slice operations
			result, err := p.performSingleExtraction(current, segment)
			if err != nil {
				return nil, fmt.Errorf("extraction failed at segment %d: %w", i, err)
			}
			current = result

		case "slice":
			// Apply slice operation - this needs special handling for extracted arrays
			if arr, ok := current.([]any); ok {
				// Parse slice parameters directly and apply slice
				start, end, step := p.parseSliceFromSegment(segment.Value)
				if start != -999999 { // Valid slice
					// Apply slice with segment context to handle open vs explicit negative slices
					slicedResult := p.applySliceToArrayWithContext(arr, start, end, step, segment.Value)
					current = slicedResult
				} else {
					current = []any{}
				}
			} else {
				return nil, fmt.Errorf("cannot apply slice to non-array type %T", current)
			}

		case "array":
			// Apply array index operation
			current = p.handlePostExtractionArrayAccess(current, segment)
			if current == nil {
				return nil, fmt.Errorf("array index operation failed")
			}

		default:
			// For other operations, use regular navigation
			result := p.handlePropertyAccess(current, segment.Value)
			if !result.Exists {
				return nil, ErrPathNotFound
			}
			current = result.Value
		}
	}

	return current, nil
}

// handleMidChainSliceAfterExtraction handles slice operations in the middle of extraction chains
// This is needed for paths like {field}[0:2]{another} where we need to slice each extracted array
func (p *Processor) handleMidChainSliceAfterExtraction(data any, sliceSegment PathSegment, segmentIndex int, allSegments []PathSegment) any {
	if _, ok := data.([]any); !ok {
		return nil
	}

	// We need to reconstruct the original array structure to apply the slice correctly
	// The problem is that extraction flattens arrays, but for mid-chain slicing,
	// we need to know which items came from which original array

	// For now, let's implement a simpler approach: apply the slice to the flattened array
	// and then continue with the remaining operations
	sliced := p.handlePostExtractionArraySlice(data, sliceSegment)
	return sliced
}

// performConsecutiveExtractions performs multiple consecutive extractions with proper flattening
func (p *Processor) performConsecutiveExtractions(data any, extractSegments []PathSegment) (any, error) {
	current := data

	for i, segment := range extractSegments {
		// Determine structure preservation based on context
		// - If this is the last extraction and no flat: prefix, preserve structure for potential post-operations
		// - If flat: prefix is used, always flatten
		// - For intermediate extractions, preserve structure to maintain nesting
		isLastExtraction := i == len(extractSegments)-1
		preserveStructure := !isLastExtraction && !segment.IsFlat

		// Special handling for flat extractions
		if segment.IsFlat {
			result, err := p.performFlatExtraction(current, segment)
			if err != nil {
				return nil, fmt.Errorf("flat extraction failed at segment %d (%s): %w", i, segment.Extract, err)
			}
			current = result
		} else {
			result, err := p.performSingleExtractionWithStructure(current, segment, preserveStructure)
			if err != nil {
				return nil, fmt.Errorf("extraction failed at segment %d (%s): %w", i, segment.Extract, err)
			}
			current = result
		}
	}

	return current, nil
}

// performFlatExtraction performs flat extraction with proper flattening
func (p *Processor) performFlatExtraction(data any, segment PathSegment) (any, error) {
	switch data := data.(type) {
	case []any:
		return p.extractFromArrayWithFlat(data, segment.Extract, true)
	case map[string]any:
		// For objects, flat extraction is the same as regular extraction
		return p.extractFromObject(data, segment.Extract)
	default:
		return nil, nil
	}
}

// flattenExtractionResult flattens nested arrays from extraction results
func (p *Processor) flattenExtractionResult(data any) any {
	arr, ok := data.([]any)
	if !ok {
		return data
	}

	var flattened []any
	for _, item := range arr {
		if subArr, ok := item.([]any); ok {
			// If item is an array, flatten it
			flattened = append(flattened, subArr...)
		} else {
			// If item is not an array, add it directly
			flattened = append(flattened, item)
		}
	}

	return flattened
}

// performSingleExtraction performs a single extraction operation
func (p *Processor) performSingleExtraction(data any, segment PathSegment) (any, error) {
	switch data := data.(type) {
	case []any:
		return p.extractFromArrayWithFlat(data, segment.Extract, segment.IsFlat)
	case map[string]any:
		return p.extractFromObject(data, segment.Extract)
	default:
		// Return nil for boundary case - trying to extract from non-extractable type
		return nil, nil
	}
}

// performSingleExtractionWithStructure performs extraction with optional structure preservation
func (p *Processor) performSingleExtractionWithStructure(data any, segment PathSegment, preserveStructure bool) (any, error) {
	switch data := data.(type) {
	case []any:
		if preserveStructure {
			return p.extractFromArrayPreservingStructureWithFlat(data, segment.Extract, segment.IsFlat)
		}
		return p.extractFromArrayWithFlat(data, segment.Extract, segment.IsFlat)
	case map[string]any:
		return p.extractFromObject(data, segment.Extract)
	default:
		// Return nil for boundary case - trying to extract from non-extractable type
		return nil, nil
	}
}

// extractFromArray extracts values from an array with nested support
func (p *Processor) extractFromArray(data any, fieldName string) (any, error) {
	return p.extractFromArrayWithFlat(data, fieldName, false)
}

// extractFromArrayWithFlat extracts values from an array with optional flattening
func (p *Processor) extractFromArrayWithFlat(data any, fieldName string, isFlat bool) (any, error) {
	arr, ok := data.([]any)
	if !ok {
		return nil, fmt.Errorf("expected array, got %T", data)
	}

	var results []any
	for _, item := range arr {
		switch v := item.(type) {
		case map[string]any:
			// Extract from object
			if value, exists := v[fieldName]; exists {
				if isFlat {
					// For flat extraction, recursively flatten arrays
					p.flattenValue(value, &results)
				} else {
					// For regular extraction, preserve array structure (don't flatten)
					results = append(results, value)
				}
			} else if isFlat {
				// For flat extraction, when field doesn't exist, add empty array to preserve structure
				results = append(results, []any{})
			}
		case []any:
			// If item is an array, recursively extract from it
			subResult, err := p.extractFromArrayWithFlat(item, fieldName, isFlat)
			if err == nil {
				if subArr, ok := subResult.([]any); ok {
					if isFlat {
						// For flat extraction, flatten recursive results
						// But preserve empty arrays to maintain structure
						if len(subArr) == 0 {
							results = append(results, []any{})
						} else {
							results = append(results, subArr...)
						}
					} else {
						// For non-flat extraction, preserve structure - don't flatten
						results = append(results, subArr)
					}
				} else {
					results = append(results, subResult)
				}
			} else if isFlat {
				// For flat extraction on empty arrays, add empty array to preserve structure
				results = append(results, []any{})
			}
		}
	}

	return results, nil
}

// extractFromArrayPreservingStructure extracts values from an array while preserving array structure
// This is used when the extraction is followed by slice operations that need to operate on individual arrays
func (p *Processor) extractFromArrayPreservingStructure(data any, fieldName string) (any, error) {
	arr, ok := data.([]any)
	if !ok {
		return nil, fmt.Errorf("expected array, got %T", data)
	}

	var results []any
	for _, item := range arr {
		switch v := item.(type) {
		case map[string]any:
			// Extract from object - preserve as individual array if the value is an array
			if value, exists := v[fieldName]; exists {
				if valueArr, ok := value.([]any); ok {
					// Keep as separate array instead of flattening
					results = append(results, valueArr)
				} else {
					// Single value - wrap in array to maintain structure
					results = append(results, []any{value})
				}
			}
		case []any:
			// If item is an array, recursively extract from it
			subResult, err := p.extractFromArrayPreservingStructure(item, fieldName)
			if err == nil {
				if subArr, ok := subResult.([]any); ok {
					results = append(results, subArr...)
				} else {
					results = append(results, subResult)
				}
			}
		}
	}

	return results, nil
}

// extractFromArrayPreservingStructureWithFlat extracts values from an array while preserving array structure with flat support
func (p *Processor) extractFromArrayPreservingStructureWithFlat(data any, fieldName string, isFlat bool) (any, error) {
	arr, ok := data.([]any)
	if !ok {
		return nil, fmt.Errorf("expected array, got %T", data)
	}

	var results []any
	for _, item := range arr {
		switch v := item.(type) {
		case map[string]any:
			// Extract from object - preserve as individual array if the value is an array
			if value, exists := v[fieldName]; exists {
				if isFlat {
					// For flat extraction with structure preservation, still flatten but wrap in array
					var flatResults []any
					p.flattenValue(value, &flatResults)
					results = append(results, flatResults)
				} else {
					// Original behavior
					if valueArr, ok := value.([]any); ok {
						// Keep as separate array instead of flattening
						results = append(results, valueArr)
					} else {
						// Single value - wrap in array to maintain structure
						results = append(results, []any{value})
					}
				}
			}
		case []any:
			// If item is an array, recursively extract from it with improved depth handling
			subResult, err := p.extractFromArrayPreservingStructureWithFlat(item, fieldName, isFlat)
			if err == nil {
				if subArr, ok := subResult.([]any); ok {
					// For deep nesting (3+ levels), maintain proper structure
					if isFlat {
						// Flatten all levels for flat extraction
						for _, subItem := range subArr {
							p.flattenValue(subItem, &results)
						}
					} else {
						// Preserve structure for non-flat extraction
						results = append(results, subArr...)
					}
				} else {
					results = append(results, subResult)
				}
			}
		}
	}

	return results, nil
}

// extractFromObject extracts values from an object
func (p *Processor) extractFromObject(data any, fieldName string) (any, error) {
	obj, ok := data.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("expected object, got %T", data)
	}

	if value, exists := obj[fieldName]; exists {
		return value, nil
	}

	return nil, fmt.Errorf("field '%s' not found", fieldName)
}

// navigateToPathWithSegments navigates through remaining path segments
func (p *Processor) navigateToPathWithSegments(data any, segments []PathSegment) (any, error) {
	current := data

	for i, segment := range segments {
		switch segment.TypeString() {
		case "property":
			result := p.handlePropertyAccess(current, segment.Value)
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

		case "slice":
			result := p.handleArraySlice(current, segment)
			if !result.Exists {
				return nil, ErrPathNotFound
			}
			current = result.Value

		case "extract":
			// If we encounter another extraction, handle it with deep extraction
			return p.handleDeepExtraction(current, segments, i)

		default:
			return nil, fmt.Errorf("unsupported segment type: %s", segment.TypeString())
		}
	}

	return current, nil
}

// extractFieldName extracts field name from extraction syntax
func (p *Processor) extractFieldName(segment string) string {
	// Remove braces from {fieldName}
	if strings.HasPrefix(segment, "{") && strings.HasSuffix(segment, "}") {
		return segment[1 : len(segment)-1]
	}
	return segment
}

// ConsecutiveExtractionGroup represents a group of consecutive extraction operations
type ConsecutiveExtractionGroup struct {
	StartIndex int
	EndIndex   int
	Segments   []PathSegment
}

// isDeepExtractionPath checks if a path contains consecutive extraction operations
func (p *Processor) isDeepExtractionPath(segments []PathSegment) bool {
	consecutiveCount := 0
	for _, segment := range segments {
		if segment.TypeString() == "extract" {
			consecutiveCount++
			if consecutiveCount >= 2 {
				return true
			}
		} else {
			consecutiveCount = 0
		}
	}
	return false
}

// preprocessPathForDeepExtraction preprocesses path segments for deep extraction
func (p *Processor) preprocessPathForDeepExtraction(segments []PathSegment) []PathSegment {
	// For now, just return the segments as-is
	// This can be extended for more complex preprocessing
	return segments
}

// detectConsecutiveExtractions detects groups of consecutive extraction operations
func (p *Processor) detectConsecutiveExtractions(segments []PathSegment) []ConsecutiveExtractionGroup {
	var groups []ConsecutiveExtractionGroup
	var currentGroup *ConsecutiveExtractionGroup

	for i, segment := range segments {
		if segment.TypeString() == "extract" {
			if currentGroup == nil {
				currentGroup = &ConsecutiveExtractionGroup{
					StartIndex: i,
					Segments:   []PathSegment{segment},
				}
			} else {
				currentGroup.Segments = append(currentGroup.Segments, segment)
			}
		} else {
			if currentGroup != nil && len(currentGroup.Segments) >= 1 { // Changed from >= 2 to >= 1
				currentGroup.EndIndex = i - 1
				groups = append(groups, *currentGroup)
			}
			currentGroup = nil
		}
	}

	// Handle case where consecutive extractions are at the end
	if currentGroup != nil && len(currentGroup.Segments) >= 1 { // Changed from >= 2 to >= 1
		currentGroup.EndIndex = len(segments) - 1
		groups = append(groups, *currentGroup)
	}

	return groups
}

// navigateWithDeepExtraction handles navigation with deep extraction groups
func (p *Processor) navigateWithDeepExtraction(data any, segments []PathSegment, groups []ConsecutiveExtractionGroup) (any, error) {
	if len(groups) == 0 {
		return p.navigateToPathWithSegments(data, segments)
	}

	// Handle the first group and any segments after it
	firstGroup := groups[0]

	// Navigate to the start of the first extraction group
	current := data
	for i := 0; i < firstGroup.StartIndex; i++ {
		result, err := p.navigateToPathWithSegments(current, segments[i:i+1])
		if err != nil {
			return nil, err
		}
		current = result
	}

	// Handle the deep extraction with all remaining segments
	// This includes both the extraction segments and any post-extraction segments
	remainingSegments := segments[firstGroup.StartIndex:]
	return p.handleDeepExtraction(current, remainingSegments, 0)
}
