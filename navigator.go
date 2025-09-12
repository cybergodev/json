package json

import (
	"fmt"
	"strconv"
	"strings"
)

// navigator implements the Navigator interface
type navigator struct {
	pathParser PathParser
	utils      ProcessorUtils
}

// NewNavigator creates a new navigator instance
func NewNavigator(pathParser PathParser, utils ProcessorUtils) Navigator {
	return &navigator{
		pathParser: pathParser,
		utils:      utils,
	}
}

// NavigateToPath navigates to a specific path in the data
func (n *navigator) NavigateToPath(data any, segments []PathSegmentInfo) (any, error) {
	if len(segments) == 0 {
		return data, nil
	}

	current := data
	for i, segment := range segments {
		result, err := n.NavigateToSegment(current, segment)
		if err != nil {
			return nil, fmt.Errorf("failed to navigate segment %d (%s): %w", i, segment.Value, err)
		}

		if !result.Exists {
			// Check if this is a boundary case (property doesn't exist on valid object)
			if n.isBoundaryCase(current, segment) {
				return nil, nil // Return nil for boundary cases
			}
			return nil, ErrPathNotFoundNew
		}

		current = result.Value
	}

	return current, nil
}

// NavigateToSegment navigates to a single segment
func (n *navigator) NavigateToSegment(data any, segment PathSegmentInfo) (NavigationResult, error) {
	switch segment.Type {
	case "property":
		return n.HandlePropertyAccess(data, segment.Key), nil
	case "array":
		return n.handleArrayAccess(data, segment)
	case "slice":
		return n.handleArraySlice(data, segment)
	case "extract":
		return n.handleExtraction(data, segment)
	default:
		return NavigationResult{}, fmt.Errorf("unsupported segment type: %s", segment.Type)
	}
}

// HandlePropertyAccess handles property access on objects
func (n *navigator) HandlePropertyAccess(data any, property string) NavigationResult {
	switch v := data.(type) {
	case map[string]any:
		value, exists := v[property]
		if !exists {
			// Check if it's a numeric property for array-like access
			if arr, ok := n.tryConvertToArray(v); ok {
				if index := n.parseArrayIndex(property); index != -2 {
					// Handle negative indices
					if index < 0 {
						index = len(arr) + index
					}
					// Check bounds after negative index conversion
					if index >= 0 && index < len(arr) {
						return NavigationResult{Value: arr[index], Exists: true}
					}
				}
			}
		}
		return NavigationResult{Value: value, Exists: exists}

	case map[any]any:
		value, exists := v[property]
		return NavigationResult{Value: value, Exists: exists}

	case []any:
		// Handle numeric property access on arrays (like array.5)
		if index := n.parseArrayIndex(property); index != -2 {
			// Handle negative indices
			if index < 0 {
				index = len(v) + index
			}
			// Check bounds after negative index conversion
			if index >= 0 && index < len(v) {
				return NavigationResult{Value: v[index], Exists: true}
			}
		}
		return NavigationResult{Value: nil, Exists: false}

	case nil:
		// Accessing property on nil - boundary case
		return NavigationResult{Value: nil, Exists: false}

	default:
		// Cannot access property on this type
		return NavigationResult{Value: nil, Exists: false}
	}
}

// handleArrayAccess handles array index access
func (n *navigator) handleArrayAccess(data any, segment PathSegmentInfo) (NavigationResult, error) {
	// Handle property.array[index] syntax
	if segment.Key != "" {
		// First access the property
		propResult := n.HandlePropertyAccess(data, segment.Key)
		if !propResult.Exists {
			return NavigationResult{Value: nil, Exists: false}, nil
		}
		data = propResult.Value
	}

	// Now handle array access
	arr, ok := data.([]any)
	if !ok {
		return NavigationResult{Value: nil, Exists: false}, nil
	}

	index := segment.Index

	// Handle negative indices
	if index < 0 {
		index = len(arr) + index
	}

	// Check bounds
	if index < 0 || index >= len(arr) {
		return NavigationResult{Value: nil, Exists: false}, nil
	}

	return NavigationResult{Value: arr[index], Exists: true}, nil
}

// handleArraySlice handles array slice operations
func (n *navigator) handleArraySlice(data any, segment PathSegmentInfo) (NavigationResult, error) {
	// Handle property.array[start:end:step] syntax
	if segment.Key != "" {
		// First access the property
		propResult := n.HandlePropertyAccess(data, segment.Key)
		if !propResult.Exists {
			return NavigationResult{Value: nil, Exists: false}, nil
		}
		data = propResult.Value
	}

	// Now handle array slice
	arr, ok := data.([]any)
	if !ok {
		return NavigationResult{Value: nil, Exists: false}, nil
	}

	// Parse slice parameters
	start, end, step := n.parseSliceParameters(segment, len(arr))

	// Perform the slice operation
	slicedArray := n.performArraySlice(arr, start, end, step)
	return NavigationResult{Value: slicedArray, Exists: true}, nil
}

// handleExtraction handles extraction operations ({key} syntax)
func (n *navigator) handleExtraction(data any, segment PathSegmentInfo) (NavigationResult, error) {
	extractKey := segment.Extract
	if extractKey == "" {
		return NavigationResult{Value: nil, Exists: false}, nil
	}

	switch v := data.(type) {
	case []any:
		// Extract the specified key from all objects in the array
		var results []any
		for _, item := range v {
			if obj, ok := item.(map[string]any); ok {
				if value, exists := obj[extractKey]; exists {
					results = append(results, value)
				} else {
					results = append(results, nil)
				}
			} else {
				results = append(results, nil)
			}
		}
		return NavigationResult{Value: results, Exists: true}, nil

	case map[string]any:
		// Extract the key from the object
		value, exists := v[extractKey]
		return NavigationResult{Value: value, Exists: exists}, nil

	default:
		return NavigationResult{Value: nil, Exists: false}, nil
	}
}

// parseSliceParameters parses slice parameters from a segment
func (n *navigator) parseSliceParameters(segment PathSegmentInfo, arrayLength int) (start, end, step int) {
	// Default values
	start = 0
	end = arrayLength
	step = 1

	// Parse start
	if segment.Start != nil {
		start = *segment.Start
		if start < 0 {
			start = arrayLength + start
		}
	}

	// Parse end
	if segment.End != nil {
		end = *segment.End
		if end < 0 {
			end = arrayLength + end
		}
	}

	// Parse step
	if segment.Step != nil {
		step = *segment.Step
		if step == 0 {
			step = 1 // Avoid division by zero
		}
	}

	// Normalize bounds
	if start < 0 {
		start = 0
	}
	if end > arrayLength {
		end = arrayLength
	}
	if start > end {
		start = end
	}

	return start, end, step
}

// performArraySlice performs the actual array slicing
func (n *navigator) performArraySlice(arr []any, start, end, step int) []any {
	if step == 1 {
		// Simple slice without step
		return arr[start:end]
	}

	// Slice with step
	var result []any
	for i := start; i < end; i += step {
		if i < len(arr) {
			result = append(result, arr[i])
		}
	}
	return result
}

// parseArrayIndex parses an array index from a string
func (n *navigator) parseArrayIndex(indexStr string) int {
	// Remove brackets if present
	if strings.HasPrefix(indexStr, "[") && strings.HasSuffix(indexStr, "]") {
		indexStr = indexStr[1 : len(indexStr)-1]
	}

	// Try to parse as integer
	if index, err := strconv.Atoi(indexStr); err == nil {
		return index
	}

	return -2 // Invalid index marker
}

// tryConvertToArray tries to convert a map to an array if it has numeric keys
func (n *navigator) tryConvertToArray(m map[string]any) ([]any, bool) {
	// This is a simplified implementation
	// In a real scenario, you might want more sophisticated logic
	return nil, false
}

// isBoundaryCase determines if a navigation failure is a boundary case
func (n *navigator) isBoundaryCase(data any, segment PathSegmentInfo) bool {
	switch segment.Type {
	case "property":
		return n.isPropertyBoundaryCase(data, segment.Key)
	case "array":
		return n.isArrayBoundaryCase(data, segment)
	default:
		return false
	}
}

// isPropertyBoundaryCase checks if property access failure is a boundary case
func (n *navigator) isPropertyBoundaryCase(data any, property string) bool {
	// Check if data is a valid object type that could have properties
	switch data.(type) {
	case map[string]any, map[any]any:
		return true // Property doesn't exist on valid object - boundary case
	case nil:
		return true // Accessing property on nil - boundary case
	case []any:
		// Check if this is a numeric property access on array
		if n.isNumericProperty(property) {
			return true // Numeric property on array - boundary case
		}
		return true // Non-numeric property on array - boundary case
	case string, float64, int, bool:
		return true // Accessing property on primitive types - boundary case
	default:
		return false // Unknown type - structural error
	}
}

// isArrayBoundaryCase checks if array access failure is a boundary case
func (n *navigator) isArrayBoundaryCase(data any, segment PathSegmentInfo) bool {
	// Get the target array
	var arrayData any = data
	if segment.Key != "" {
		result := n.HandlePropertyAccess(data, segment.Key)
		if !result.Exists {
			return true // Property doesn't exist - boundary case
		}
		arrayData = result.Value
	}

	// Check if target is actually an array
	arr, ok := arrayData.([]any)
	if !ok {
		return false // Not an array - structural error
	}

	// Check if index is out of bounds
	index := segment.Index
	if index < 0 {
		index = len(arr) + index
	}

	return index < 0 || index >= len(arr) // Out of bounds is boundary case
}

// isNumericProperty checks if a property name is numeric
func (n *navigator) isNumericProperty(property string) bool {
	for _, char := range property {
		if char < '0' || char > '9' {
			return false
		}
	}
	return len(property) > 0
}
