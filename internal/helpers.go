package internal

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// DeepMerge recursively merges two JSON values using union merge strategy
// - If both values are objects, recursively merge their keys
// - If both values are arrays, merge with deduplication (union)
// - For all other cases (primitives), value2 takes precedence
func DeepMerge(base, override any) any {
	baseMap, baseIsMap := base.(map[string]any)
	overrideMap, overrideIsMap := override.(map[string]any)

	if baseIsMap && overrideIsMap {
		result := make(map[string]any)

		// First, copy all keys from base
		for key, value := range baseMap {
			result[key] = value
		}

		// Then, merge override keys
		for key, overrideValue := range overrideMap {
			if baseValue, exists := baseMap[key]; exists {
				// Both exist - recursively merge
				result[key] = DeepMerge(baseValue, overrideValue)
			} else {
				// Only in override - add directly
				result[key] = overrideValue
			}
		}

		return result
	}

	baseArray, baseIsArray := base.([]any)
	overrideArray, overrideIsArray := override.([]any)

	if baseIsArray && overrideIsArray {
		// Merge arrays with deduplication
		result := make([]any, 0, len(baseArray)+len(overrideArray))
		seen := make(map[string]bool)

		// Add elements from base array
		for _, item := range baseArray {
			key := ArrayItemKey(item)
			if !seen[key] {
				seen[key] = true
				result = append(result, item)
			}
		}

		// Add elements from override array
		for _, item := range overrideArray {
			key := ArrayItemKey(item)
			if !seen[key] {
				seen[key] = true
				result = append(result, item)
			}
		}

		return result
	}

	// For non-map, non-array types, override takes precedence
	return override
}

// ArrayItemKey generates a unique key for array item deduplication
func ArrayItemKey(item any) string {
	switch v := item.(type) {
	case string:
		return "s:" + v
	case float64:
		// JSON numbers are parsed as float64
		return "n:" + FormatNumberForDedup(v)
	case bool:
		if v {
			return "b:true"
		}
		return "b:false"
	case nil:
		return "null"
	case map[string]any:
		// For objects, use JSON marshaling for comparison
		bytes, err := json.Marshal(v)
		if err != nil {
			return fmt.Sprintf("obj:%p", v)
		}
		return "o:" + string(bytes)
	case []any:
		// For arrays, use JSON marshaling for comparison
		bytes, err := json.Marshal(v)
		if err != nil {
			return fmt.Sprintf("arr:%p", v)
		}
		return "a:" + string(bytes)
	default:
		// Fallback for other types
		return fmt.Sprintf("other:%v", v)
	}
}

// FormatNumberForDedup formats a number for deduplication key generation
func FormatNumberForDedup(f float64) string {
	// Check if it's an integer
	if f == float64(int64(f)) {
		return fmt.Sprintf("%d", int64(f))
	}
	return fmt.Sprintf("%g", f)
}

// IsJSONPointerPath checks if a path uses JSON Pointer format
func IsJSONPointerPath(path string) bool {
	return path != "" && path[0] == '/'
}

// IsDotNotationPath checks if a path uses dot notation format
func IsDotNotationPath(path string) bool {
	return path != "" && path != "." && path[0] != '/'
}

// IsArrayPath checks if a path contains array access
func IsArrayPath(path string) bool {
	return strings.Contains(path, "[") && strings.Contains(path, "]")
}

// IsSlicePath checks if a path contains slice notation
func IsSlicePath(path string) bool {
	return strings.Contains(path, "[") && strings.Contains(path, ":") && strings.Contains(path, "]")
}

// IsExtractionPath checks if a path contains extraction syntax
func IsExtractionPath(path string) bool {
	return strings.Contains(path, "{") && strings.Contains(path, "}")
}

// IsJSONObject checks if data is a JSON object (map[string]any)
func IsJSONObject(data any) bool {
	_, ok := data.(map[string]any)
	return ok
}

// IsJSONArray checks if data is a JSON array ([]any)
func IsJSONArray(data any) bool {
	_, ok := data.([]any)
	return ok
}

// IsJSONPrimitive checks if data is a JSON primitive type
func IsJSONPrimitive(data any) bool {
	switch data.(type) {
	case string, int, int32, int64, float32, float64, bool, nil:
		return true
	default:
		return false
	}
}

// TryConvertToArray attempts to convert a map to an array if it has numeric keys
func TryConvertToArray(m map[string]any) ([]any, bool) {
	if len(m) == 0 {
		return []any{}, true
	}

	maxIndex := -1
	for key := range m {
		if index, err := strconv.Atoi(key); err == nil && index >= 0 {
			if index > maxIndex {
				maxIndex = index
			}
		} else {
			return nil, false
		}
	}

	arr := make([]any, maxIndex+1)
	for key, value := range m {
		if index, err := strconv.Atoi(key); err == nil {
			arr[index] = value
		}
	}

	return arr, true
}
