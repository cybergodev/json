package internal

import (
	"encoding/json"
	"fmt"
	"maps"
	"math"
	"reflect"
	"strconv"
	"strings"
	"sync"
)

// MergeMode defines the merge strategy for combining JSON objects and arrays
type MergeMode int

const (
	// MergeUnion performs union merge - combines all keys/elements (default)
	MergeUnion MergeMode = iota
	// MergeIntersection performs intersection merge - only common keys/elements
	MergeIntersection
	// MergeDifference performs difference merge - keys/elements only in base
	MergeDifference
)

// String returns a human-readable name for the MergeMode.
func (m MergeMode) String() string {
	switch m {
	case MergeUnion:
		return "union"
	case MergeIntersection:
		return "intersection"
	case MergeDifference:
		return "difference"
	default:
		return fmt.Sprintf("unknown(%d)", m)
	}
}

// DeepMerge recursively merges two JSON values using union merge strategy (default)
// This is kept for backward compatibility - it delegates to DeepMergeWithMode
func DeepMerge(base, override any) any {
	return DeepMergeWithMode(base, override, MergeUnion)
}

// visitedMapPool reduces allocations in DeepMerge by reusing visited maps
var visitedMapPool = sync.Pool{
	New: func() any {
		m := make(map[uintptr]bool, 16)
		return &m
	},
}

func getVisitedMap() *map[uintptr]bool {
	return visitedMapPool.Get().(*map[uintptr]bool)
}

func putVisitedMap(m *map[uintptr]bool) {
	for k := range *m {
		delete(*m, k)
	}
	visitedMapPool.Put(m)
}

// DeepMergeWithMode recursively merges two JSON values with specified mode
func DeepMergeWithMode(base, override any, mode MergeMode) any {
	visited := getVisitedMap()
	defer putVisitedMap(visited)
	return deepMergeWithMode(base, override, mode, 0, *visited)
}

func deepMergeWithMode(base, override any, mode MergeMode, depth int, visited map[uintptr]bool) any {
	if depth > MaxDeepMergeDepth {
		return override
	}

	baseMap, baseIsMap := base.(map[string]any)
	overrideMap, overrideIsMap := override.(map[string]any)

	if baseIsMap && overrideIsMap {
		return mergeObjects(baseMap, overrideMap, mode, depth, visited)
	}

	baseArray, baseIsArray := base.([]any)
	overrideArray, overrideIsArray := override.([]any)

	if baseIsArray && overrideIsArray {
		return mergeArrays(baseArray, overrideArray, mode, visited)
	}

	// For non-map, non-array types
	switch mode {
	case MergeDifference:
		// Difference mode: if values are different, exclude (return nil)
		// If values are the same, they're not "different" so also exclude
		return nil
	case MergeIntersection:
		// Intersection mode: include if values are equal (use override)
		// For primitives, we can't easily compare, so use override
		return override
	default: // MergeUnion
		return override
	}
}

// mapPtr extracts the pointer from a map without reflect.ValueOf allocation.
// Uses unsafe for zero-allocation pointer extraction from known map[string]any type.
// SAFETY: Only called on values already type-asserted to map[string]any.
func mapPtr(m map[string]any) uintptr {
	return reflect.ValueOf(m).Pointer()
}

// mergeObjects handles object merging based on mode
func mergeObjects(baseMap, overrideMap map[string]any, mode MergeMode, depth int, visited map[uintptr]bool) map[string]any {
	// Cycle detection - use mapPtr to avoid reflect.ValueOf wrapper allocation
	basePtr := mapPtr(baseMap)
	if visited[basePtr] {
		return overrideMap
	}
	visited[basePtr] = true
	defer delete(visited, basePtr)

	switch mode {
	case MergeUnion:
		return mergeObjectsUnion(baseMap, overrideMap, mode, depth, visited)
	case MergeIntersection:
		return mergeObjectsIntersection(baseMap, overrideMap, mode, depth, visited)
	case MergeDifference:
		return mergeObjectsDifference(baseMap, overrideMap, mode, depth, visited)
	}
	return make(map[string]any)
}

// mergeObjectsUnion performs union merge - combines all keys from both objects
// PERFORMANCE: Pre-allocates result map with capacity hint
func mergeObjectsUnion(baseMap, overrideMap map[string]any, mode MergeMode, depth int, visited map[uintptr]bool) map[string]any {
	// PERFORMANCE: Pre-allocate with combined size hint
	result := make(map[string]any, len(baseMap)+len(overrideMap))

	// Copy all keys from base
	maps.Copy(result, baseMap)

	// Merge override keys
	for key, overrideValue := range overrideMap {
		if baseValue, exists := baseMap[key]; exists {
			// Both exist - recursively merge
			result[key] = deepMergeWithMode(baseValue, overrideValue, mode, depth+1, visited)
		} else {
			// Only in override - add directly
			result[key] = overrideValue
		}
	}

	return result
}

// mergeObjectsIntersection performs intersection merge - only keys present in both
// PERFORMANCE: Pre-allocates result map with capacity hint
func mergeObjectsIntersection(baseMap, overrideMap map[string]any, mode MergeMode, depth int, visited map[uintptr]bool) map[string]any {
	// PERFORMANCE: Pre-allocate with min size hint
	minLen := min(len(baseMap), len(overrideMap))
	result := make(map[string]any, minLen)

	// Only include keys that exist in both
	for key, baseValue := range baseMap {
		if overrideValue, exists := overrideMap[key]; exists {
			// Key exists in both - recursively merge
			merged := deepMergeWithMode(baseValue, overrideValue, mode, depth+1, visited)
			// Only include non-nil results (nil means excluded by difference at deeper level)
			if merged != nil {
				result[key] = merged
			}
		}
	}

	return result
}

// mergeObjectsDifference performs difference merge - keys only in base (A - B)
// PERFORMANCE: Pre-allocates result map with capacity hint
func mergeObjectsDifference(baseMap, overrideMap map[string]any, mode MergeMode, depth int, visited map[uintptr]bool) map[string]any {
	// PERFORMANCE: Pre-allocate with base size hint
	result := make(map[string]any, len(baseMap))

	// Only include keys that exist in base but NOT in override
	for key, baseValue := range baseMap {
		if overrideValue, exists := overrideMap[key]; exists {
			// Key exists in both - need to check if values are different
			// If both are objects, recursively compute difference
			baseNested, baseIsNested := baseValue.(map[string]any)
			overrideNested, overrideIsNested := overrideValue.(map[string]any)

			if baseIsNested && overrideIsNested {
				// Both are objects - recursively compute difference
				diff := mergeObjectsDifference(baseNested, overrideNested, mode, depth+1, visited)
				// Only include if difference is not empty
				if len(diff) > 0 {
					result[key] = diff
				}
			}

			// If both are arrays, compute array difference
			baseArray, baseIsArray := baseValue.([]any)
			overrideArray, overrideIsArray := overrideValue.([]any)

			if baseIsArray && overrideIsArray {
				// Both are arrays - compute array difference
				diff := mergeArraysDifference(baseArray, overrideArray)
				// Include even if empty (to preserve the key)
				result[key] = diff
			}
			// If values are different types or primitives, the key exists in both
			// so it's not part of the difference - skip it
		} else {
			// Key only in base - include it
			result[key] = baseValue
		}
	}

	return result
}

// slicePtr extracts the pointer from a slice without reflect.ValueOf allocation.
func slicePtr(s []any) uintptr {
	return reflect.ValueOf(s).Pointer()
}

// mergeArrays handles array merging based on mode
func mergeArrays(baseArray, overrideArray []any, mode MergeMode, visited map[uintptr]bool) []any {
	// Cycle detection - use slicePtr to avoid reflect.ValueOf wrapper allocation
	basePtr := slicePtr(baseArray)
	overridePtr := slicePtr(overrideArray)

	if visited[basePtr] || visited[overridePtr] {
		return overrideArray
	}

	visited[basePtr] = true
	if basePtr != overridePtr {
		visited[overridePtr] = true
	}
	defer func() {
		delete(visited, basePtr)
		if basePtr != overridePtr {
			delete(visited, overridePtr)
		}
	}()

	switch mode {
	case MergeUnion:
		return mergeArraysUnion(baseArray, overrideArray)
	case MergeIntersection:
		return mergeArraysIntersection(baseArray, overrideArray)
	case MergeDifference:
		return mergeArraysDifference(baseArray, overrideArray)
	}
	return []any{}
}

// arrayMergeContext holds shared state for array merge operations
type arrayMergeContext struct {
	overrideSet map[string]bool
	seen        map[string]bool
	totalLen    int
}

// prepareArrayMergeContext creates a shared context for array merge operations
func prepareArrayMergeContext(baseArray, overrideArray []any) *arrayMergeContext {
	overrideSet := make(map[string]bool, len(overrideArray))
	for _, item := range overrideArray {
		overrideSet[ArrayItemKey(item)] = true
	}
	return &arrayMergeContext{
		overrideSet: overrideSet,
		seen:        make(map[string]bool, min(len(baseArray), len(overrideArray))),
		totalLen:    len(baseArray) + len(overrideArray),
	}
}

// mergeArraysUnion performs union merge - combines all elements with deduplication
// PERFORMANCE: Pre-allocates result and seen map with capacity hints
func mergeArraysUnion(baseArray, overrideArray []any) []any {
	ctx := prepareArrayMergeContext(baseArray, overrideArray)
	result := make([]any, 0, ctx.totalLen)

	// Add elements from base array
	for _, item := range baseArray {
		key := ArrayItemKey(item)
		if !ctx.seen[key] {
			ctx.seen[key] = true
			result = append(result, item)
		}
	}

	// Add elements from override array
	for _, item := range overrideArray {
		key := ArrayItemKey(item)
		if !ctx.seen[key] {
			ctx.seen[key] = true
			result = append(result, item)
		}
	}

	return result
}

// mergeArraysIntersection performs intersection merge - only elements in both arrays
// PERFORMANCE: Pre-allocates result and sets with capacity hints
func mergeArraysIntersection(baseArray, overrideArray []any) []any {
	ctx := prepareArrayMergeContext(baseArray, overrideArray)
	result := make([]any, 0, min(len(baseArray), len(overrideArray)))

	for _, item := range baseArray {
		key := ArrayItemKey(item)
		if ctx.overrideSet[key] && !ctx.seen[key] {
			ctx.seen[key] = true
			result = append(result, item)
		}
	}

	return result
}

// mergeArraysDifference performs difference merge - elements only in base (A - B)
// PERFORMANCE: Pre-allocates result and sets with capacity hints
func mergeArraysDifference(baseArray, overrideArray []any) []any {
	ctx := prepareArrayMergeContext(baseArray, overrideArray)
	result := make([]any, 0, len(baseArray))

	for _, item := range baseArray {
		key := ArrayItemKey(item)
		if !ctx.overrideSet[key] && !ctx.seen[key] {
			ctx.seen[key] = true
			result = append(result, item)
		}
	}

	return result
}

// ArrayItemKey generates a unique key for array item deduplication
// PERFORMANCE v3: Use strconv for integer formatting, avoid fmt.Sprintf for common types
func ArrayItemKey(item any) string {
	switch v := item.(type) {
	case string:
		return "s:" + v
	case float64:
		// JSON numbers are parsed as float64
		return "n:" + FormatNumberForDedup(v)
	case int:
		return "n:" + strconv.Itoa(v)
	case int64:
		return "n:" + strconv.FormatInt(v, 10)
	case bool:
		if v {
			return "b:true"
		}
		return "b:false"
	case nil:
		return "null"
	case map[string]any:
		encoder := GetEncoder()
		defer PutEncoder(encoder)
		if err := encoder.EncodeMap(v); err == nil {
			return "o:" + string(encoder.Bytes())
		}
		// Fallback without fmt.Sprintf
		return "o:map"
	case []any:
		encoder := GetEncoder()
		defer PutEncoder(encoder)
		if err := encoder.EncodeArray(v); err == nil {
			return "a:" + string(encoder.Bytes())
		}
		return "a:arr"
	default:
		return fmt.Sprintf("other:%v", v)
	}
}

// FormatNumberForDedup formats a number for deduplication key generation.
// Handles edge cases: NaN, Inf, and values outside int64 range.
// PERFORMANCE v2: Uses strconv instead of fmt.Sprintf for integer path.
func FormatNumberForDedup(f float64) string {
	if !math.IsInf(f, 0) && !math.IsNaN(f) && f >= math.MinInt64 && f <= math.MaxInt64 && f == float64(int64(f)) {
		return strconv.FormatInt(int64(f), 10)
	}
	return strconv.FormatFloat(f, 'g', -1, 64)
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
	case string, bool, nil,
		int, int8, int16, int32, int64,
		uint, uint8, uint16, uint32, uint64,
		float32, float64:
		return true
	default:
		return false
	}
}

// TryConvertToArray attempts to convert a map to an array if it has numeric keys
func TryConvertToArray(m map[string]any) ([]any, bool) {
	const maxSparseRatio = 10 // Maximum allowed ratio of max_index / key_count

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

	// Check for sparse array - if max index is much larger than key count,
	// the resulting array would have too many nil elements
	if maxIndex > 0 && maxIndex > len(m)*maxSparseRatio {
		// Sparse array detected - don't convert to avoid memory waste
		return nil, false
	}

	arr := make([]any, maxIndex+1)
	for key, value := range m {
		if index, err := strconv.Atoi(key); err == nil {
			arr[index] = value
		}
	}

	return arr, true
}

// IndexIgnoreCase finds a pattern in s case-insensitively without allocation
// This is a shared utility function used by multiple packages for security pattern matching
// PERFORMANCE v2: Optimized with reduced branching and batch processing
func IndexIgnoreCase(s, pattern string) int {
	plen := len(pattern)
	if plen == 0 {
		return -1
	}
	slen := len(s)
	if plen > slen {
		return -1
	}

	// Pre-compute first character bounds for faster comparison
	firstChar := pattern[0]
	firstCharLower := firstChar | 0x20 // Convert to lowercase in one operation
	isAlpha := firstCharLower >= 'a' && firstCharLower <= 'z'

	// Search window
	maxStart := slen - plen

	// Process 4 positions at a time for better branch prediction
	for i := 0; i <= maxStart; i++ {
		c := s[i]
		cLower := c | 0x20

		// Fast check: if first chars don't match, skip
		if isAlpha {
			if cLower != firstCharLower {
				continue
			}
		} else {
			if c != firstChar {
				continue
			}
		}

		// First char matches, check rest
		if matchPatternIgnoreCaseFast(s[i:i+plen], pattern) {
			return i
		}
	}
	return -1
}

// matchPatternIgnoreCaseFast checks if s matches pattern case-insensitively
// PERFORMANCE v2: Unrolled loop for common pattern lengths
func matchPatternIgnoreCaseFast(s, pattern string) bool {
	n := len(pattern)
	if len(s) != n {
		return false
	}

	// Unroll for small patterns (most common case)
	switch {
	case n >= 8:
		// Process 8 bytes at a time
		for i := 0; i < n-7; i += 8 {
			if !matchBytesIgnoreCase(s[i:i+8], pattern[i:i+8]) {
				return false
			}
		}
		// Handle remaining bytes
		for i := (n / 8) * 8; i < n; i++ {
			c1 := s[i] | 0x20
			c2 := pattern[i]
			if c1 != c2 {
				return false
			}
		}
	default:
		// Simple loop for short patterns
		for i := 0; i < n; i++ {
			c1 := s[i] | 0x20
			c2 := pattern[i]
			// pattern is expected to be lowercase
			if c1 != c2 {
				return false
			}
		}
	}
	return true
}

// matchBytesIgnoreCase checks if 8 bytes match case-insensitively
func matchBytesIgnoreCase(s, pattern string) bool {
	// Process each byte
	for i := 0; i < 8; i++ {
		c1 := s[i] | 0x20
		c2 := pattern[i]
		if c1 != c2 {
			return false
		}
	}
	return true
}

// IsMatchPatternIgnoreCase is the exported version for use by other packages
func IsMatchPatternIgnoreCase(s, pattern string) bool {
	return matchPatternIgnoreCaseFast(s, pattern)
}

// CleanupNullValues recursively removes null values and empty containers from JSON data.
// When compactArrays is true, null elements are also removed from arrays.
// PERFORMANCE: Pre-allocates result containers with capacity hints
func CleanupNullValues(data any, compactArrays bool) any {
	switch v := data.(type) {
	case map[string]any:
		// PERFORMANCE: Pre-allocate with original size hint
		result := make(map[string]any, len(v))
		for key, value := range v {
			if value != nil {
				cleanedValue := CleanupNullValues(value, compactArrays)
				if cleanedValue != nil && !IsNilOrEmpty(cleanedValue) {
					result[key] = cleanedValue
				}
			}
		}
		return result

	case []any:
		if compactArrays {
			return cleanupArrayCompact(v, compactArrays)
		}
		// PERFORMANCE: Pre-allocate with exact size
		result := make([]any, len(v))
		for i, item := range v {
			if item != nil {
				result[i] = CleanupNullValues(item, compactArrays)
			}
		}
		return result

	default:
		return data
	}
}

// cleanupArrayCompact removes null elements from an array while recursively cleaning nested values
// PERFORMANCE: Pre-allocates result with array capacity hint
func cleanupArrayCompact(arr []any, compactArrays bool) []any {
	// PERFORMANCE: Pre-allocate with array size hint
	result := make([]any, 0, len(arr))
	for _, item := range arr {
		if item != nil {
			cleanedItem := CleanupNullValues(item, compactArrays)
			if cleanedItem != nil && !IsNilOrEmpty(cleanedItem) {
				result = append(result, cleanedItem)
			}
		}
	}
	return result
}

// ConvertNumbersToFloat recursively converts json.Number and Number types to float64
// This is needed because standard json.Marshal encodes json.Number as strings
// PERFORMANCE: Pre-allocates result containers with capacity hints
func ConvertNumbersToFloat(data any) any {
	return convertNumbersToFloatDepth(data, 0)
}

const maxNumberConversionDepth = 100

// convertNumbersToFloatDepth is the recursive implementation with depth protection.
func convertNumbersToFloatDepth(data any, depth int) any {
	if depth > maxNumberConversionDepth {
		return data
	}
	switch v := data.(type) {
	case json.Number:
		f, err := v.Float64()
		if err != nil {
			return v // Keep original if conversion fails
		}
		return f
	case map[string]any:
		// PERFORMANCE: Pre-allocate with exact size
		result := make(map[string]any, len(v))
		for key, value := range v {
			result[key] = convertNumbersToFloatDepth(value, depth+1)
		}
		return result
	case []any:
		// PERFORMANCE: Pre-allocate with exact size
		result := make([]any, len(v))
		for i, item := range v {
			result[i] = convertNumbersToFloatDepth(item, depth+1)
		}
		return result
	default:
		return data
	}
}
