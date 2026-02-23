package json

import (
	"fmt"
	"strings"
)

// Extraction-related set operations

func (p *Processor) setValueForExtract(current any, segment PathSegment, value any, createPaths bool) error {
	field := segment.Key
	if field == "" {
		return fmt.Errorf("invalid extraction syntax: %s", segment.String())
	}

	// Handle array extraction
	if arr, ok := current.([]any); ok {
		if segment.IsFlat {
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

// Helper function to check if string contains substring (used in this file)
var stringsContains = strings.Contains
