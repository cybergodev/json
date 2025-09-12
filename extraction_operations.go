package json

import (
	"fmt"
	"strings"
)

// extractionOperations implements the ExtractionOperations interface
type extractionOperations struct {
	utils ProcessorUtils
}

// NewExtractionOperations creates a new extraction operations instance
func NewExtractionOperations(utils ProcessorUtils) ExtractionOperations {
	return &extractionOperations{
		utils: utils,
	}
}

// HandleExtraction handles basic extraction operations ({key} syntax)
func (eo *extractionOperations) HandleExtraction(data any, key string) (any, error) {
	if key == "" {
		return nil, fmt.Errorf("extraction key cannot be empty")
	}

	switch v := data.(type) {
	case []any:
		// Extract the specified key from all objects in the array
		var results []any
		for _, item := range v {
			if obj, ok := item.(map[string]any); ok {
				if value, exists := obj[key]; exists {
					results = append(results, value)
				} else {
					results = append(results, nil)
				}
			} else {
				results = append(results, nil)
			}
		}
		return results, nil

	case map[string]any:
		// Extract the key from the object
		if value, exists := v[key]; exists {
			return value, nil
		}
		return nil, nil

	case map[any]any:
		// Handle generic map
		if value, exists := v[key]; exists {
			return value, nil
		}
		return nil, nil

	default:
		return nil, fmt.Errorf("cannot extract from type %T", data)
	}
}

// HandleDeepExtraction handles deep extraction operations with multiple levels
func (eo *extractionOperations) HandleDeepExtraction(data any, keys []string) (any, error) {
	if len(keys) == 0 {
		return data, nil
	}

	current := data
	for i, key := range keys {
		result, err := eo.HandleExtraction(current, key)
		if err != nil {
			return nil, fmt.Errorf("deep extraction failed at level %d (key: %s): %w", i, key, err)
		}
		current = result
	}

	return current, nil
}

// HandleConsecutiveExtractions handles consecutive extraction operations like {field1}{field2}
func (eo *extractionOperations) HandleConsecutiveExtractions(data any, segments []PathSegmentInfo) (any, error) {
	// Extract only the extraction segments
	var extractionKeys []string
	for _, segment := range segments {
		if segment.Type == "extract" && segment.Extract != "" {
			extractionKeys = append(extractionKeys, segment.Extract)
		}
	}

	if len(extractionKeys) == 0 {
		return data, nil
	}

	return eo.processConsecutiveExtractions(data, extractionKeys, 0)
}

// DetectConsecutiveExtractions detects groups of consecutive extraction operations
func (eo *extractionOperations) DetectConsecutiveExtractions(segments []PathSegmentInfo) [][]PathSegmentInfo {
	var groups [][]PathSegmentInfo
	var currentGroup []PathSegmentInfo

	for _, segment := range segments {
		if segment.Type == "extract" {
			currentGroup = append(currentGroup, segment)
		} else {
			if len(currentGroup) > 1 {
				// Only consider groups with multiple consecutive extractions
				groups = append(groups, currentGroup)
			}
			currentGroup = nil
		}
	}

	// Don't forget the last group
	if len(currentGroup) > 1 {
		groups = append(groups, currentGroup)
	}

	return groups
}

// processConsecutiveExtractions recursively processes consecutive extractions
func (eo *extractionOperations) processConsecutiveExtractions(data any, keys []string, depth int) (any, error) {
	if depth >= len(keys) {
		return data, nil
	}

	currentKey := keys[depth]

	// Extract current level
	extracted, err := eo.HandleExtraction(data, currentKey)
	if err != nil {
		return nil, fmt.Errorf("consecutive extraction failed at depth %d (key: %s): %w", depth, currentKey, err)
	}

	// If this is the last key, return the result
	if depth == len(keys)-1 {
		return extracted, nil
	}

	// Continue with next level
	return eo.processConsecutiveExtractions(extracted, keys, depth+1)
}

// HandleMixedExtractionOperations handles mixed operations like {field}[0:2]{another}
func (eo *extractionOperations) HandleMixedExtractionOperations(data any, segments []PathSegmentInfo) (any, error) {
	current := data

	for i, segment := range segments {
		switch segment.Type {
		case "extract":
			result, err := eo.HandleExtraction(current, segment.Extract)
			if err != nil {
				return nil, fmt.Errorf("mixed extraction failed at segment %d: %w", i, err)
			}
			current = result

		case "array":
			// Handle array access after extraction
			arr, ok := current.([]any)
			if !ok {
				return nil, fmt.Errorf("expected array for index access at segment %d, got %T", i, current)
			}

			index := segment.Index
			if index < 0 {
				index = len(arr) + index
			}

			if index < 0 || index >= len(arr) {
				return nil, fmt.Errorf("array index %d out of bounds at segment %d", segment.Index, i)
			}

			current = arr[index]

		case "slice":
			// Handle array slice after extraction
			arr, ok := current.([]any)
			if !ok {
				return nil, fmt.Errorf("expected array for slice access at segment %d, got %T", i, current)
			}

			start, end, step := eo.parseSliceParameters(segment)
			current = eo.performSlice(arr, start, end, step)

		case "property":
			// Handle property access
			result, err := eo.handlePropertyAccess(current, segment.Key)
			if err != nil {
				return nil, fmt.Errorf("property access failed at segment %d: %w", i, err)
			}
			current = result

		default:
			return nil, fmt.Errorf("unsupported segment type in mixed extraction: %s", segment.Type)
		}
	}

	return current, nil
}

// parseSliceParameters parses slice parameters from a segment
func (eo *extractionOperations) parseSliceParameters(segment PathSegmentInfo) (start, end, step int) {
	start = 0
	end = -1 // Will be set to array length
	step = 1

	if segment.Start != nil {
		start = *segment.Start
	}
	if segment.End != nil {
		end = *segment.End
	}
	if segment.Step != nil {
		step = *segment.Step
		if step == 0 {
			step = 1
		}
	}

	return start, end, step
}

// performSlice performs array slicing
func (eo *extractionOperations) performSlice(arr []any, start, end, step int) []any {
	length := len(arr)

	// Handle negative indices
	if start < 0 {
		start = length + start
	}
	if end < 0 {
		end = length + end
	}
	if end == -1 {
		end = length
	}

	// Clamp to bounds
	if start < 0 {
		start = 0
	}
	if end > length {
		end = length
	}
	if start > end {
		start = end
	}

	// Perform slice
	if step == 1 {
		return arr[start:end]
	}

	var result []any
	for i := start; i < end; i += step {
		if i < length {
			result = append(result, arr[i])
		}
	}
	return result
}

// handlePropertyAccess handles property access for mixed operations
func (eo *extractionOperations) handlePropertyAccess(data any, property string) (any, error) {
	switch v := data.(type) {
	case map[string]any:
		if value, exists := v[property]; exists {
			return value, nil
		}
		return nil, fmt.Errorf("property '%s' not found", property)

	case map[any]any:
		if value, exists := v[property]; exists {
			return value, nil
		}
		return nil, fmt.Errorf("property '%s' not found", property)

	default:
		return nil, fmt.Errorf("cannot access property '%s' on type %T", property, data)
	}
}

// ExtractFromArray extracts a specific key from all objects in an array
func (eo *extractionOperations) ExtractFromArray(arr []any, key string) []any {
	var results []any
	for _, item := range arr {
		if obj, ok := item.(map[string]any); ok {
			if value, exists := obj[key]; exists {
				results = append(results, value)
			} else {
				results = append(results, nil)
			}
		} else {
			results = append(results, nil)
		}
	}
	return results
}

// ExtractFromObject extracts a specific key from an object
func (eo *extractionOperations) ExtractFromObject(obj map[string]any, key string) (any, bool) {
	value, exists := obj[key]
	return value, exists
}

// FlattenExtractionResults flattens nested extraction results
func (eo *extractionOperations) FlattenExtractionResults(data any) []any {
	var results []any
	eo.flattenRecursive(data, &results)
	return results
}

// flattenRecursive recursively flattens extraction results
func (eo *extractionOperations) flattenRecursive(data any, results *[]any) {
	switch v := data.(type) {
	case []any:
		for _, item := range v {
			if subArr, ok := item.([]any); ok {
				eo.flattenRecursive(subArr, results)
			} else {
				*results = append(*results, item)
			}
		}
	default:
		*results = append(*results, data)
	}
}

// IsExtractionPath checks if a path contains extraction operations
func (eo *extractionOperations) IsExtractionPath(path string) bool {
	return strings.Contains(path, "{") && strings.Contains(path, "}")
}

// CountExtractions counts the number of extraction operations in a path
func (eo *extractionOperations) CountExtractions(path string) int {
	return strings.Count(path, "{")
}

// ValidateExtractionSyntax validates extraction syntax in a path
func (eo *extractionOperations) ValidateExtractionSyntax(path string) error {
	openCount := strings.Count(path, "{")
	closeCount := strings.Count(path, "}")

	if openCount != closeCount {
		return fmt.Errorf("unmatched extraction braces in path: %s", path)
	}

	// Check for empty extractions
	if strings.Contains(path, "{}") {
		return fmt.Errorf("empty extraction syntax not allowed: %s", path)
	}

	return nil
}

// ExtractMultipleKeys extracts multiple keys from data in a single operation
func (eo *extractionOperations) ExtractMultipleKeys(data any, keys []string) (map[string]any, error) {
	results := make(map[string]any)

	for _, key := range keys {
		result, err := eo.HandleExtraction(data, key)
		if err != nil {
			results[key] = nil
		} else {
			results[key] = result
		}
	}

	return results, nil
}

// FilterExtractionResults filters extraction results based on a predicate
func (eo *extractionOperations) FilterExtractionResults(data any, predicate func(any) bool) []any {
	var results []any

	switch v := data.(type) {
	case []any:
		for _, item := range v {
			if predicate(item) {
				results = append(results, item)
			}
		}
	default:
		if predicate(data) {
			results = append(results, data)
		}
	}

	return results
}
