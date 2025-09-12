package json

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
)

// Type checking utilities

// isArrayType checks if a value is an array type
func (p *Processor) isArrayType(data any) bool {
	switch data.(type) {
	case []any:
		return true
	default:
		return false
	}
}

// isObjectType checks if a value is an object type
func (p *Processor) isObjectType(data any) bool {
	switch data.(type) {
	case map[string]any, map[any]any:
		return true
	default:
		return false
	}
}

// isMapType checks if a value is a map type
func (p *Processor) isMapType(data any) bool {
	switch data.(type) {
	case map[string]any, map[any]any:
		return true
	default:
		return false
	}
}

// isSliceType checks if a value is a slice type
func (p *Processor) isSliceType(data any) bool {
	if data == nil {
		return false
	}

	v := reflect.ValueOf(data)
	return v.Kind() == reflect.Slice
}

// isPrimitiveType checks if a value is a primitive type
func (p *Processor) isPrimitiveType(data any) bool {
	switch data.(type) {
	case string, int, int8, int16, int32, int64,
		uint, uint8, uint16, uint32, uint64,
		float32, float64, bool:
		return true
	default:
		return false
	}
}

// isNilOrEmpty checks if a value is nil or empty
func (p *Processor) isNilOrEmpty(data any) bool {
	if data == nil {
		return true
	}

	switch v := data.(type) {
	case string:
		return v == ""
	case []any:
		return len(v) == 0
	case map[string]any:
		return len(v) == 0
	case map[any]any:
		return len(v) == 0
	default:
		return false
	}
}

// Data conversion utilities

// convertToStringMap converts a map[any]any to map[string]any
func (p *Processor) convertToStringMap(data map[any]any) map[string]any {
	result := make(map[string]any)
	for key, value := range data {
		if strKey, ok := key.(string); ok {
			result[strKey] = value
		} else {
			result[fmt.Sprintf("%v", key)] = value
		}
	}
	return result
}

// convertToAnyMap converts a map[string]any to map[any]any
func (p *Processor) convertToAnyMap(data map[string]any) map[any]any {
	result := make(map[any]any)
	for key, value := range data {
		result[key] = value
	}
	return result
}

// ensureStringMap ensures data is a map[string]any
func (p *Processor) ensureStringMap(data any) (map[string]any, bool) {
	switch v := data.(type) {
	case map[string]any:
		return v, true
	case map[any]any:
		return p.convertToStringMap(v), true
	default:
		return nil, false
	}
}

// ensureArray ensures data is a []any
func (p *Processor) ensureArray(data any) ([]any, bool) {
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

// Deep copy utilities

// deepCopyData creates a deep copy of data
func (p *Processor) deepCopyData(data any) any {
	switch v := data.(type) {
	case map[string]any:
		return p.deepCopyStringMap(v)
	case map[any]any:
		return p.deepCopyAnyMap(v)
	case []any:
		return p.deepCopyArray(v)
	case string, int, int8, int16, int32, int64,
		uint, uint8, uint16, uint32, uint64,
		float32, float64, bool:
		return v // Primitives are copied by value
	default:
		// For other types, try to use reflection
		return p.deepCopyReflection(data)
	}
}

// deepCopyStringMap creates a deep copy of a string map
func (p *Processor) deepCopyStringMap(data map[string]any) map[string]any {
	result := make(map[string]any)
	for key, value := range data {
		result[key] = p.deepCopyData(value)
	}
	return result
}

// deepCopyAnyMap creates a deep copy of an any map
func (p *Processor) deepCopyAnyMap(data map[any]any) map[any]any {
	result := make(map[any]any)
	for key, value := range data {
		result[key] = p.deepCopyData(value)
	}
	return result
}

// deepCopyArray creates a deep copy of an array
func (p *Processor) deepCopyArray(data []any) []any {
	result := make([]any, len(data))
	for i, value := range data {
		result[i] = p.deepCopyData(value)
	}
	return result
}

// deepCopyReflection creates a deep copy using reflection
func (p *Processor) deepCopyReflection(data any) any {
	if data == nil {
		return nil
	}

	v := reflect.ValueOf(data)
	switch v.Kind() {
	case reflect.Ptr:
		if v.IsNil() {
			return nil
		}
		// Create new pointer and copy the value
		newPtr := reflect.New(v.Elem().Type())
		newPtr.Elem().Set(reflect.ValueOf(p.deepCopyReflection(v.Elem().Interface())))
		return newPtr.Interface()
	case reflect.Struct:
		// Create new struct and copy fields
		newStruct := reflect.New(v.Type()).Elem()
		for i := 0; i < v.NumField(); i++ {
			if v.Field(i).CanInterface() {
				newStruct.Field(i).Set(reflect.ValueOf(p.deepCopyReflection(v.Field(i).Interface())))
			}
		}
		return newStruct.Interface()
	default:
		// For other types, return as-is
		return data
	}
}

// Path utilities

// unescapeJSONPointer unescapes JSON Pointer characters

// escapeJSONPointer escapes JSON Pointer characters
func (p *Processor) escapeJSONPointer(segment string) string {
	// JSON Pointer escaping: ~ becomes ~0, / becomes ~1
	segment = strings.ReplaceAll(segment, "~", "~0")
	segment = strings.ReplaceAll(segment, "/", "~1")
	return segment
}

// normalizePathSeparators normalizes path separators
func (p *Processor) normalizePathSeparators(path string) string {
	// Replace multiple dots with single dots
	for strings.Contains(path, "..") {
		path = strings.ReplaceAll(path, "..", ".")
	}

	// Remove leading and trailing dots
	path = strings.Trim(path, ".")

	return path
}

// splitPathSegments splits a path into segments
func (p *Processor) splitPathSegments(path string) []string {
	if path == "" {
		return []string{}
	}

	// Handle JSON Pointer format
	if strings.HasPrefix(path, "/") {
		pathWithoutSlash := path[1:]
		if pathWithoutSlash == "" {
			return []string{}
		}
		return strings.Split(pathWithoutSlash, "/")
	}

	// Handle dot notation
	return strings.Split(path, ".")
}

// joinPathSegments joins segments into a path
func (p *Processor) joinPathSegments(segments []string, useJSONPointer bool) string {
	if len(segments) == 0 {
		return ""
	}

	if useJSONPointer {
		return "/" + strings.Join(segments, "/")
	}

	return strings.Join(segments, ".")
}

// Validation utilities

// isValidPropertyName checks if a property name is valid
func (p *Processor) isValidPropertyName(name string) bool {
	return name != "" && !strings.ContainsAny(name, ".[]{}()")
}

// isValidArrayIndex checks if an array index is valid
func (p *Processor) isValidArrayIndex(index string) bool {
	if index == "" {
		return false
	}

	// Check for negative indices
	if strings.HasPrefix(index, "-") {
		index = index[1:]
	}

	// Check if it's a valid number
	_, err := strconv.Atoi(index)
	return err == nil
}

// isValidSliceRange checks if a slice range is valid
func (p *Processor) isValidSliceRange(rangeStr string) bool {
	parts := strings.Split(rangeStr, ":")
	if len(parts) < 2 || len(parts) > 3 {
		return false
	}

	// Check each part
	for _, part := range parts {
		if part != "" {
			if _, err := strconv.Atoi(part); err != nil {
				return false
			}
		}
	}

	return true
}

// needsLegacyComplexHandling determines if a path needs legacy complex handling
func (p *Processor) needsLegacyComplexHandling(path string) bool {
	// Check if this is a distributed operation - if so, don't use legacy handling
	if p.isDistributedOperationPath(path) {
		return false
	}

	// Only use legacy handling for very specific complex patterns that the unified processor can't handle yet
	var legacyPatterns []string

	for _, pattern := range legacyPatterns {
		if strings.Contains(path, pattern) {
			return true
		}
	}

	// Check for multiple consecutive extractions (very complex patterns)
	// Note: Modern distributed operations can handle multiple extractions, so we're more lenient
	extractCount := strings.Count(path, "{")
	if extractCount > 4 { // Increased threshold since distributed operations can handle more complexity
		return true
	}

	// Check for step-based slicing (e.g., [::2], [1:5:2])
	if strings.Contains(path, "::") || (strings.Count(path, ":") > 2) {
		return true
	}

	return false
}

// Error utilities

// wrapError wraps an error with additional context
func (p *Processor) wrapError(err error, context string) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", context, err)
}

// createPathError creates a path-specific error
func (p *Processor) createPathError(path string, operation string, err error) error {
	return fmt.Errorf("failed to %s at path '%s': %w", operation, path, err)
}

// Memory management utilities

// getStringBuilder gets a string builder from the pool with enhanced safety and monitoring
func (p *Processor) getStringBuilder() *strings.Builder {
	if p.resources.stringBuilderPool != nil && !p.isClosing() {
		if builder := p.resources.stringBuilderPool.Get(); builder != nil {
			if sb, ok := builder.(*strings.Builder); ok {
				sb.Reset()
				if p.resourceMonitor != nil {
					p.resourceMonitor.RecordPoolHit()
				}
				return sb
			}
		}
	}

	// Record pool miss
	if p.resourceMonitor != nil {
		p.resourceMonitor.RecordPoolMiss()
		p.resourceMonitor.RecordAllocation(512) // Estimated initial capacity
	}

	// Fallback: create new builder with optimized capacity
	sb := &strings.Builder{}
	sb.Grow(512)
	return sb
}

// putStringBuilder returns a string builder to the pool with optimized size limits and monitoring
func (p *Processor) putStringBuilder(builder *strings.Builder) {
	if p.resources.stringBuilderPool != nil && builder != nil && !p.isClosing() {
		// Optimized size limits for better memory efficiency
		capacity := builder.Cap()
		if capacity <= 4096 && capacity >= 128 { // Reduced upper limit for better memory usage
			builder.Reset() // Clear content but keep capacity
			p.resources.stringBuilderPool.Put(builder)
		} else {
			// Record eviction for oversized or undersized builders
			if p.resourceMonitor != nil {
				p.resourceMonitor.RecordPoolEviction()
				p.resourceMonitor.RecordDeallocation(int64(capacity))
			}
		}
	}
}
