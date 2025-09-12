package json

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"sync"
)

// processorUtils implements the ProcessorUtils interface
type processorUtils struct {
	// String builder pool for efficient string operations
	stringBuilderPool *stringBuilderPool
}

// NewProcessorUtils creates a new processor utils instance
func NewProcessorUtils() ProcessorUtils {
	return &processorUtils{
		stringBuilderPool: newStringBuilderPool(),
	}
}

// IsArrayType checks if the data is an array type
func (u *processorUtils) IsArrayType(data any) bool {
	switch data.(type) {
	case []any:
		return true
	default:
		return false
	}
}

// IsObjectType checks if the data is an object type
func (u *processorUtils) IsObjectType(data any) bool {
	switch data.(type) {
	case map[string]any, map[any]any:
		return true
	default:
		return false
	}
}

// IsEmptyContainer checks if a container (object or array) is empty
func (u *processorUtils) IsEmptyContainer(data any) bool {
	switch v := data.(type) {
	case map[string]any:
		return len(v) == 0
	case map[any]any:
		return len(v) == 0
	case []any:
		// Check if all elements are nil
		for _, item := range v {
			if item != nil {
				return false
			}
		}
		return true
	default:
		return false
	}
}

// DeepCopy creates a deep copy of the data structure
func (u *processorUtils) DeepCopy(data any) (any, error) {
	// Use JSON marshal/unmarshal for deep copy
	jsonBytes, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal data for deep copy: %w", err)
	}

	var result any
	err = json.Unmarshal(jsonBytes, &result)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal data for deep copy: %w", err)
	}

	return result, nil
}

// GetDataType returns a string representation of the data type
func (u *processorUtils) GetDataType(data any) string {
	if data == nil {
		return "null"
	}

	switch data.(type) {
	case map[string]any:
		return "object"
	case map[any]any:
		return "object"
	case []any:
		return "array"
	case string:
		return "string"
	case float64:
		return "number"
	case int:
		return "number"
	case bool:
		return "boolean"
	default:
		return reflect.TypeOf(data).String()
	}
}

// ConvertToMap converts data to map[string]any if possible
func (u *processorUtils) ConvertToMap(data any) (map[string]any, bool) {
	switch v := data.(type) {
	case map[string]any:
		return v, true
	case map[any]any:
		// Convert map[any]any to map[string]any
		result := make(map[string]any)
		for key, value := range v {
			if strKey, ok := key.(string); ok {
				result[strKey] = value
			} else {
				result[fmt.Sprintf("%v", key)] = value
			}
		}
		return result, true
	default:
		return nil, false
	}
}

// ConvertToArray converts data to []any if possible
func (u *processorUtils) ConvertToArray(data any) ([]any, bool) {
	switch v := data.(type) {
	case []any:
		return v, true
	default:
		// Try to convert using reflection
		rv := reflect.ValueOf(data)
		if rv.Kind() == reflect.Slice {
			result := make([]any, rv.Len())
			for i := 0; i < rv.Len(); i++ {
				result[i] = rv.Index(i).Interface()
			}
			return result, true
		}
		return nil, false
	}
}

// stringBuilderPool provides a pool of string builders for efficient string operations
type stringBuilderPool struct {
	pool sync.Pool
}

// newStringBuilderPool creates a new string builder pool
func newStringBuilderPool() *stringBuilderPool {
	return &stringBuilderPool{
		pool: sync.Pool{
			New: func() interface{} {
				return &strings.Builder{}
			},
		},
	}
}

// Get gets a string builder from the pool
func (p *stringBuilderPool) Get() *strings.Builder {
	return p.pool.Get().(*strings.Builder)
}

// Put returns a string builder to the pool
func (p *stringBuilderPool) Put(sb *strings.Builder) {
	sb.Reset()
	p.pool.Put(sb)
}

// Helper functions for common operations

// ParseInt parses a string to integer with error handling
func ParseInt(s string) (int, error) {
	return strconv.Atoi(s)
}

// ParseFloat parses a string to float64 with error handling
func ParseFloat(s string) (float64, error) {
	return strconv.ParseFloat(s, 64)
}

// ParseBool parses a string to boolean with error handling
func ParseBool(s string) (bool, error) {
	return strconv.ParseBool(s)
}

// IsNumeric checks if a string represents a numeric value
func IsNumeric(s string) bool {
	_, err := strconv.ParseFloat(s, 64)
	return err == nil
}

// IsInteger checks if a string represents an integer value
func IsInteger(s string) bool {
	_, err := strconv.Atoi(s)
	return err == nil
}

// NormalizeIndex normalizes an array index (handles negative indices)
func NormalizeIndex(index, length int) int {
	if index < 0 {
		return length + index
	}
	return index
}

// IsValidIndex checks if an index is valid for an array of given length
func IsValidIndex(index, length int) bool {
	normalizedIndex := NormalizeIndex(index, length)
	return normalizedIndex >= 0 && normalizedIndex < length
}

// ClampIndex clamps an index to valid bounds for an array
func ClampIndex(index, length int) int {
	if index < 0 {
		return 0
	}
	if index >= length {
		return length - 1
	}
	return index
}

// SanitizeKey sanitizes a key for safe use in maps
func SanitizeKey(key string) string {
	// Remove any null bytes or other problematic characters
	return strings.ReplaceAll(key, "\x00", "")
}

// EscapeJSONPointer escapes special characters for JSON Pointer
func EscapeJSONPointer(s string) string {
	s = strings.ReplaceAll(s, "~", "~0")
	s = strings.ReplaceAll(s, "/", "~1")
	return s
}

// UnescapeJSONPointer unescapes JSON Pointer special characters
func UnescapeJSONPointer(s string) string {
	s = strings.ReplaceAll(s, "~1", "/")
	s = strings.ReplaceAll(s, "~0", "~")
	return s
}

// IsContainer checks if the data is a container type (map or slice)
func IsContainer(data any) bool {
	switch data.(type) {
	case map[string]any, map[any]any, []any:
		return true
	default:
		return false
	}
}

// GetContainerSize returns the size of a container
func GetContainerSize(data any) int {
	switch v := data.(type) {
	case map[string]any:
		return len(v)
	case map[any]any:
		return len(v)
	case []any:
		return len(v)
	default:
		return 0
	}
}

// CreateEmptyContainer creates an empty container of the specified type
func CreateEmptyContainer(containerType string) any {
	switch containerType {
	case "object":
		return make(map[string]any)
	case "array":
		return make([]any, 0)
	default:
		return make(map[string]any) // Default to object
	}
}

// mergeObjects merges two objects, with the second object taking precedence (internal use)
func mergeObjects(obj1, obj2 map[string]any) map[string]any {
	result := make(map[string]any)

	// Copy from first object
	for k, v := range obj1 {
		result[k] = v
	}

	// Override with second object
	for k, v := range obj2 {
		result[k] = v
	}

	return result
}

// flattenArray flattens a nested array structure (internal use)
func flattenArray(arr []any) []any {
	var result []any

	for _, item := range arr {
		if subArr, ok := item.([]any); ok {
			result = append(result, flattenArray(subArr)...)
		} else {
			result = append(result, item)
		}
	}

	return result
}

// uniqueArray removes duplicate values from an array (internal use)
func uniqueArray(arr []any) []any {
	seen := make(map[string]bool)
	var result []any

	for _, item := range arr {
		key := fmt.Sprintf("%v", item)
		if !seen[key] {
			seen[key] = true
			result = append(result, item)
		}
	}

	return result
}

// reverseArray reverses an array in place (internal use)
func reverseArray(arr []any) {
	for i, j := 0, len(arr)-1; i < j; i, j = i+1, j-1 {
		arr[i], arr[j] = arr[j], arr[i]
	}
}

// ConvertToString converts a value to string
func (u *processorUtils) ConvertToString(value any) string {
	if value == nil {
		return ""
	}

	switch v := value.(type) {
	case string:
		return v
	case int:
		return strconv.Itoa(v)
	case int64:
		return strconv.FormatInt(v, 10)
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(v)
	default:
		return fmt.Sprintf("%v", v)
	}
}

// ConvertToNumber converts a value to a number (float64)
func (u *processorUtils) ConvertToNumber(value any) (float64, error) {
	switch v := value.(type) {
	case float64:
		return v, nil
	case int:
		return float64(v), nil
	case int64:
		return float64(v), nil
	case string:
		return strconv.ParseFloat(v, 64)
	default:
		return 0, fmt.Errorf("cannot convert %T to number", value)
	}
}
