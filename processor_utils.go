package json

import (
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/cybergodev/json/internal"
)

// processorUtils provides utility functions for JSON processing
type processorUtils struct {
	// String builder pool for efficient string operations
	stringBuilderPool *stringBuilderPool
}

// NewProcessorUtils creates a new processor utils instance
func NewProcessorUtils() *processorUtils {
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

// stringBuilderPool provides a pool of string builders for efficient string operations
type stringBuilderPool struct {
	pool sync.Pool
}

// newStringBuilderPool creates a new string builder pool
func newStringBuilderPool() *stringBuilderPool {
	return &stringBuilderPool{
		pool: sync.Pool{
			New: func() any {
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
	return internal.EscapeJSONPointer(s)
}

// UnescapeJSONPointer unescapes JSON Pointer special characters
func UnescapeJSONPointer(s string) string {
	return internal.UnescapeJSONPointer(s)
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
	return internal.MergeObjects(obj1, obj2)
}

// flattenArray flattens a nested array structure (internal use)
func flattenArray(arr []any) []any {
	return internal.FlattenArray(arr)
}

// uniqueArray removes duplicate values from an array (internal use)
func uniqueArray(arr []any) []any {
	return internal.UniqueArray(arr)
}

// reverseArray reverses an array in place (internal use)
func reverseArray(arr []any) {
	internal.ReverseArray(arr)
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
