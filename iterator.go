package json

import (
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/cybergodev/json/internal"
)

// PathType represents the complexity level of a path
type PathType int

const (
	// PathTypeSimple indicates a single key with no dots or brackets
	PathTypeSimple PathType = iota
	// PathTypeComplex indicates a path containing dots or brackets
	PathTypeComplex
)

// pathTypeCacheShard represents a single shard of the path type cache
// PERFORMANCE: Sharding reduces lock contention for concurrent access
type pathTypeCacheShard struct {
	mu      sync.RWMutex
	entries map[string]PathType
}

// pathTypeCacheShards is a sharded cache for path type results
// Using 16 shards for good distribution with minimal overhead
var pathTypeCacheShards [16]pathTypeCacheShard

// init initializes the path type cache shards
func init() {
	for i := range pathTypeCacheShards {
		pathTypeCacheShards[i].entries = make(map[string]PathType, 64)
	}
}

// getPathTypeShard returns the shard for a path using FNV-1a hash
func getPathTypeShard(path string) *pathTypeCacheShard {
	h := uint64(14695981039346656037)
	for i := 0; i < len(path); i++ {
		h ^= uint64(path[i])
		h *= 1099511628211
	}
	return &pathTypeCacheShards[h&15]
}

// GetPathType determines if a path is simple or complex
// Simple paths are single keys with no dots or brackets
func GetPathType(path string) PathType {
	// Check cache first (only for short paths to avoid memory bloat)
	if len(path) <= 64 {
		shard := getPathTypeShard(path)
		shard.mu.RLock()
		if pt, ok := shard.entries[path]; ok {
			shard.mu.RUnlock()
			return pt
		}
		shard.mu.RUnlock()

		var pt PathType
		if strings.ContainsAny(path, ".[]") {
			pt = PathTypeComplex
		} else {
			pt = PathTypeSimple
		}

		// Cache short paths
		shard.mu.Lock()
		shard.entries[path] = pt
		shard.mu.Unlock()

		return pt
	}

	var pt PathType
	if strings.ContainsAny(path, ".[]") {
		pt = PathTypeComplex
	} else {
		pt = PathTypeSimple
	}

	return pt
}

// SafeTypeAssert performs a safe type assertion with generics
func SafeTypeAssert[T any](value any) (T, bool) {
	var zero T

	if value == nil {
		return zero, false
	}

	// Direct type assertion
	if result, ok := value.(T); ok {
		return result, true
	}

	// Try conversion via reflection
	val := reflect.ValueOf(value)
	targetType := reflect.TypeOf(zero)

	if targetType != nil && val.Type().ConvertibleTo(targetType) {
		converted := val.Convert(targetType)
		return converted.Interface().(T), true
	}

	return zero, false
}

// Iterator represents an iterator over JSON data
type Iterator struct {
	processor *Processor
	data      any
	options   *ProcessorOptions
	position  int
	keys      []string // Cached keys for map iteration
	keysInit  bool     // Flag for lazy initialization
}

// NewIterator creates a new Iterator
func NewIterator(processor *Processor, data any, opts *ProcessorOptions) *Iterator {
	return &Iterator{
		processor: processor,
		data:      data,
		options:   opts,
		position:  0,
	}
}

// initKeysOnce lazily initializes cached keys for map iteration
// PERFORMANCE: Avoids allocating a new slice on every Next() call
// Uses key interning to reduce memory for repeated keys
func (it *Iterator) initKeysOnce() {
	if it.keysInit {
		return
	}
	if obj, ok := it.data.(map[string]any); ok {
		// Reuse existing slice if capacity is sufficient
		if cap(it.keys) < len(obj) {
			it.keys = make([]string, 0, len(obj))
		} else {
			it.keys = it.keys[:0]
		}
		for k := range obj {
			it.keys = append(it.keys, internal.InternKey(k))
		}
	}
	it.keysInit = true
}

// iterableValuePool pools IterableValue objects to reduce allocations
// PERFORMANCE: Significant reduction in allocations during nested iteration
var iterableValuePool = sync.Pool{
	New: func() any {
		return &IterableValue{}
	},
}

// HasNext checks if there are more elements
func (it *Iterator) HasNext() bool {
	if arr, ok := it.data.([]any); ok {
		return it.position < len(arr)
	}
	if _, ok := it.data.(map[string]any); ok {
		// PERFORMANCE: Use cached keys instead of calling len() on map
		it.initKeysOnce()
		return it.position < len(it.keys)
	}
	return false
}

// Next returns the next element
func (it *Iterator) Next() (any, bool) {
	if !it.HasNext() {
		return nil, false
	}

	if arr, ok := it.data.([]any); ok {
		result := arr[it.position]
		it.position++
		return result, true
	}

	if obj, ok := it.data.(map[string]any); ok {
		// PERFORMANCE: Use cached keys instead of reflect.ValueOf(obj).MapKeys()
		// which allocates a new slice on every call
		it.initKeysOnce()
		if it.position < len(it.keys) {
			key := it.keys[it.position]
			it.position++
			return obj[key], true
		}
	}

	return nil, false
}

// IterableValue wraps a value to provide convenient access methods
// Note: Simplified to avoid resource leaks from holding processor/iterator references
type IterableValue struct {
	data any
}

// NewIterableValue creates an IterableValue from data
func NewIterableValue(data any) *IterableValue {
	return &IterableValue{data: data}
}

// GetData returns the underlying data
func (iv *IterableValue) GetData() any {
	return iv.data
}

// Get returns a value by path (supports dot notation and array indices)
func (iv *IterableValue) Get(path string) any {
	if path == "" || path == "." {
		return iv.data
	}

	// Use enhanced path navigation for complex paths
	if isComplexPathIterator(path) {
		// Use compiled path cache for complex paths
		// NOTE: Do NOT call Release() on cached paths - they are shared!
		cp, err := internal.GetGlobalCompiledPathCache().Get(path)
		if err != nil {
			return nil
		}
		result, err := cp.Get(iv.data)
		if err != nil {
			return nil
		}
		return result
	}

	// Fast path for simple paths - avoid strings.Split allocation
	current := iv.data
	start := 0
	for i := 0; i <= len(path); i++ {
		if i == len(path) || path[i] == '.' {
			if i > start {
				part := path[start:i]
				obj, ok := current.(map[string]any)
				if !ok {
					return nil
				}
				current, ok = obj[part]
				if !ok {
					return nil
				}
			}
			start = i + 1
		}
	}
	return current
}

// GetString returns a string value by key or path
// Supports path navigation with dot notation and array indices (e.g., "user.address.city" or "users[0].name")
func (iv *IterableValue) GetString(key string) string {
	// PERFORMANCE: Use cached path type check instead of strings.ContainsAny
	switch GetPathType(key) {
	case PathTypeComplex:
		val := iv.Get(key)
		if val == nil {
			return ""
		}
		if str, ok := val.(string); ok {
			return str
		}
		return ConvertToString(val)
	case PathTypeSimple:
		obj, ok := iv.data.(map[string]any)
		if !ok {
			return ""
		}

		val, exists := obj[key]
		if !exists {
			return ""
		}

		if str, ok := val.(string); ok {
			return str
		}

		return ConvertToString(val)
	}
	return ""
}

// GetInt returns an int value by key or path
// Supports path navigation with dot notation and array indices (e.g., "user.age" or "users[0].id")
func (iv *IterableValue) GetInt(key string) int {
	// PERFORMANCE: Use cached path type check instead of strings.ContainsAny
	switch GetPathType(key) {
	case PathTypeComplex:
		val := iv.Get(key)
		if val == nil {
			return 0
		}
		if result, ok := ConvertToInt(val); ok {
			return result
		}
		return 0
	case PathTypeSimple:
		obj, ok := iv.data.(map[string]any)
		if !ok {
			return 0
		}

		val, exists := obj[key]
		if !exists {
			return 0
		}

		if result, ok := ConvertToInt(val); ok {
			return result
		}
	}
	return 0
}

// GetFloat64 returns a float64 value by key or path
// Supports path navigation with dot notation and array indices
func (iv *IterableValue) GetFloat64(key string) float64 {
	// PERFORMANCE: Use cached path type check instead of strings.ContainsAny
	switch GetPathType(key) {
	case PathTypeComplex:
		val := iv.Get(key)
		if val == nil {
			return 0
		}
		if result, ok := ConvertToFloat64(val); ok {
			return result
		}
		return 0
	case PathTypeSimple:
		obj, ok := iv.data.(map[string]any)
		if !ok {
			return 0
		}

		val, exists := obj[key]
		if !exists {
			return 0
		}

		if result, ok := ConvertToFloat64(val); ok {
			return result
		}
	}
	return 0
}

// GetBool returns a bool value by key or path
// Supports path navigation with dot notation and array indices
func (iv *IterableValue) GetBool(key string) bool {
	// PERFORMANCE: Use cached path type check instead of strings.ContainsAny
	switch GetPathType(key) {
	case PathTypeComplex:
		val := iv.Get(key)
		if val == nil {
			return false
		}
		if result, ok := ConvertToBool(val); ok {
			return result
		}
		return false
	case PathTypeSimple:
		obj, ok := iv.data.(map[string]any)
		if !ok {
			return false
		}

		val, exists := obj[key]
		if !exists {
			return false
		}

		if result, ok := ConvertToBool(val); ok {
			return result
		}
	}
	return false
}

// GetArray returns an array value by key or path
// Supports path navigation with dot notation and array indices
func (iv *IterableValue) GetArray(key string) []any {
	// PERFORMANCE: Use cached path type check instead of strings.ContainsAny
	switch GetPathType(key) {
	case PathTypeComplex:
		val := iv.Get(key)
		if val == nil {
			return nil
		}
		if arr, ok := val.([]any); ok {
			return arr
		}
		return nil
	case PathTypeSimple:
		obj, ok := iv.data.(map[string]any)
		if !ok {
			return nil
		}

		val, exists := obj[key]
		if !exists {
			return nil
		}

		if arr, ok := val.([]any); ok {
			return arr
		}
	}
	return nil
}

// GetObject returns an object value by key or path
// Supports path navigation with dot notation and array indices
func (iv *IterableValue) GetObject(key string) map[string]any {
	// PERFORMANCE: Use cached path type check instead of strings.ContainsAny
	switch GetPathType(key) {
	case PathTypeComplex:
		val := iv.Get(key)
		if val == nil {
			return nil
		}
		if result, ok := val.(map[string]any); ok {
			return result
		}
		return nil
	case PathTypeSimple:
		obj, ok := iv.data.(map[string]any)
		if !ok {
			return nil
		}

		val, exists := obj[key]
		if !exists {
			return nil
		}

		if result, ok := val.(map[string]any); ok {
			return result
		}
	}
	return nil
}

// GetWithDefault returns a value by key or path with a default fallback
// Supports path navigation with dot notation and array indices
func (iv *IterableValue) GetWithDefault(key string, defaultValue any) any {
	// PERFORMANCE: Use cached path type check instead of strings.ContainsAny
	switch GetPathType(key) {
	case PathTypeComplex:
		val := iv.Get(key)
		if val == nil {
			return defaultValue
		}
		return val
	case PathTypeSimple:
		obj, ok := iv.data.(map[string]any)
		if !ok {
			return defaultValue
		}

		val, exists := obj[key]
		if !exists {
			return defaultValue
		}

		return val
	}
	return defaultValue
}

// GetStringWithDefault returns a string value by key or path with a default fallback
// Supports path navigation with dot notation and array indices
func (iv *IterableValue) GetStringWithDefault(key string, defaultValue string) string {
	// PERFORMANCE: Use cached path type check instead of strings.ContainsAny
	switch GetPathType(key) {
	case PathTypeComplex:
		val := iv.Get(key)
		if val == nil {
			return defaultValue
		}
		if str, ok := val.(string); ok {
			return str
		}
		return defaultValue
	case PathTypeSimple:
		obj, ok := iv.data.(map[string]any)
		if !ok {
			return defaultValue
		}

		val, exists := obj[key]
		if !exists {
			return defaultValue
		}

		if str, ok := val.(string); ok {
			return str
		}
	}
	return defaultValue
}

// GetIntWithDefault returns an int value by key or path with a default fallback
// Supports path navigation with dot notation and array indices
func (iv *IterableValue) GetIntWithDefault(key string, defaultValue int) int {
	// PERFORMANCE: Use cached path type check instead of strings.ContainsAny
	switch GetPathType(key) {
	case PathTypeComplex:
		val := iv.Get(key)
		if val == nil {
			return defaultValue
		}
		if result, ok := ConvertToInt(val); ok {
			return result
		}
		return defaultValue
	case PathTypeSimple:
		obj, ok := iv.data.(map[string]any)
		if !ok {
			return defaultValue
		}

		val, exists := obj[key]
		if !exists {
			return defaultValue
		}

		if result, ok := ConvertToInt(val); ok {
			return result
		}
	}
	return defaultValue
}

// GetFloat64WithDefault returns a float64 value by key or path with a default fallback
// Supports path navigation with dot notation and array indices
func (iv *IterableValue) GetFloat64WithDefault(key string, defaultValue float64) float64 {
	// PERFORMANCE: Use cached path type check instead of strings.ContainsAny
	switch GetPathType(key) {
	case PathTypeComplex:
		val := iv.Get(key)
		if val == nil {
			return defaultValue
		}
		if result, ok := ConvertToFloat64(val); ok {
			return result
		}
		return defaultValue
	case PathTypeSimple:
		obj, ok := iv.data.(map[string]any)
		if !ok {
			return defaultValue
		}

		val, exists := obj[key]
		if !exists {
			return defaultValue
		}

		if result, ok := ConvertToFloat64(val); ok {
			return result
		}
	}
	return defaultValue
}

// GetBoolWithDefault returns a bool value by key or path with a default fallback
// Supports path navigation with dot notation and array indices
func (iv *IterableValue) GetBoolWithDefault(key string, defaultValue bool) bool {
	// PERFORMANCE: Use cached path type check instead of strings.ContainsAny
	switch GetPathType(key) {
	case PathTypeComplex:
		val := iv.Get(key)
		if val == nil {
			return defaultValue
		}
		if result, ok := ConvertToBool(val); ok {
			return result
		}
		return defaultValue
	case PathTypeSimple:
		obj, ok := iv.data.(map[string]any)
		if !ok {
			return defaultValue
		}

		val, exists := obj[key]
		if !exists {
			return defaultValue
		}

		if result, ok := ConvertToBool(val); ok {
			return result
		}
	}
	return defaultValue
}

// Exists checks if a key or path exists in the object
// Supports path navigation with dot notation and array indices
func (iv *IterableValue) Exists(key string) bool {
	// PERFORMANCE: Use cached path type check instead of strings.ContainsAny
	switch GetPathType(key) {
	case PathTypeComplex:
		return iv.Get(key) != nil
	case PathTypeSimple:
		obj, ok := iv.data.(map[string]any)
		if !ok {
			return false
		}

		_, exists := obj[key]
		return exists
	}
	return false
}

// IsNullData checks if the whole value is null (for backward compatibility)
func (iv *IterableValue) IsNullData() bool {
	return iv.data == nil
}

// IsNull checks if a specific key's or path's value is null
// Supports path navigation with dot notation and array indices
func (iv *IterableValue) IsNull(key string) bool {
	// PERFORMANCE: Use cached path type check instead of strings.ContainsAny
	switch GetPathType(key) {
	case PathTypeComplex:
		val := iv.Get(key)
		return val == nil
	case PathTypeSimple:
		obj, ok := iv.data.(map[string]any)
		if !ok {
			return true
		}

		val, exists := obj[key]
		if !exists {
			return true
		}

		return val == nil
	}
	return true
}

// IsEmptyData checks if the whole value is empty (for backward compatibility)
func (iv *IterableValue) IsEmptyData() bool {
	if iv.data == nil {
		return true
	}

	switch v := iv.data.(type) {
	case []any:
		return len(v) == 0
	case map[string]any:
		return len(v) == 0
	case string:
		return v == ""
	default:
		return false
	}
}

// IsEmpty checks if a specific key's or path's value is empty
// Supports path navigation with dot notation and array indices
func (iv *IterableValue) IsEmpty(key string) bool {
	// PERFORMANCE: Use cached path type check instead of strings.ContainsAny
	switch GetPathType(key) {
	case PathTypeComplex:
		val := iv.Get(key)
		if val == nil {
			return true
		}
		switch v := val.(type) {
		case []any:
			return len(v) == 0
		case map[string]any:
			return len(v) == 0
		case string:
			return v == ""
		default:
			return false
		}
	case PathTypeSimple:
		obj, ok := iv.data.(map[string]any)
		if !ok {
			return true
		}

		val, exists := obj[key]
		if !exists {
			return true
		}

		switch v := val.(type) {
		case []any:
			return len(v) == 0
		case map[string]any:
			return len(v) == 0
		case string:
			return v == ""
		default:
			return false
		}
	}
	return true
}

// ForeachNested iterates over nested JSON structures with a path
func (iv *IterableValue) ForeachNested(path string, fn func(key any, item *IterableValue)) {
	var data any = iv.data

	if path != "" && path != "." {
		var err error
		data, err = navigateToPathSimple(iv.data, path)
		if err != nil {
			return
		}
	}

	foreachNestedOnValue(data, fn)
}

// ForeachWithPathAndControl iterates over JSON arrays or objects and applies a function
// This is the 3-parameter version used by most code
func ForeachWithPathAndControl(jsonStr, path string, fn func(key any, value any) IteratorControl) error {
	processor := getDefaultProcessor()

	data, err := processor.Get(jsonStr, path)
	if err != nil {
		return err
	}

	return foreachOnValue(data, fn)
}

// Foreach iterates over JSON arrays or objects with simplified signature (for test compatibility)
func Foreach(jsonStr string, fn func(key any, item *IterableValue)) {
	processor := getDefaultProcessor()

	data, err := processor.Get(jsonStr, ".")
	if err != nil {
		return
	}

	foreachWithIterableValue(data, fn)
}

// foreachWithIterableValue iterates over a value and applies a function with IterableValue
// PERFORMANCE: Uses pooled IterableValue to reduce allocations
func foreachWithIterableValue(data any, fn func(key any, item *IterableValue)) {
	switch v := data.(type) {
	case []any:
		for i, item := range v {
			iv := iterableValuePool.Get().(*IterableValue)
			iv.data = item
			fn(i, iv)
			iv.data = nil
			iterableValuePool.Put(iv)
		}
	case map[string]any:
		for key, val := range v {
			iv := iterableValuePool.Get().(*IterableValue)
			iv.data = val
			fn(key, iv)
			iv.data = nil
			iterableValuePool.Put(iv)
		}
	}
}

// ForeachWithPath iterates over JSON arrays or objects with simplified signature (for test compatibility)
func ForeachWithPath(jsonStr, path string, fn func(key any, item *IterableValue)) error {
	processor := getDefaultProcessor()

	data, err := processor.Get(jsonStr, path)
	if err != nil {
		return err
	}

	foreachWithIterableValue(data, fn)
	return nil
}

// foreachWithPathAndIterator iterates with IterableValue and path information (full version)
func foreachWithPathAndIterator(jsonStr, path string, fn func(key any, item *IterableValue, currentPath string) IteratorControl) error {
	processor := getDefaultProcessor()

	data, err := processor.Get(jsonStr, path)
	if err != nil {
		return err
	}

	return foreachWithPathIterableValue(data, "", fn)
}

// foreachWithPathIterableValue iterates with IterableValue and path information
// PERFORMANCE: Uses pooled IterableValue to reduce allocations
func foreachWithPathIterableValue(data any, currentPath string, fn func(key any, item *IterableValue, currentPath string) IteratorControl) error {
	switch v := data.(type) {
	case []any:
		for i, item := range v {
			path := fmt.Sprintf("%s[%d]", currentPath, i)
			iv := iterableValuePool.Get().(*IterableValue)
			iv.data = item
			ctrl := fn(i, iv, path)
			iv.data = nil
			iterableValuePool.Put(iv)
			if ctrl == IteratorBreak {
				return nil
			}
		}
	case map[string]any:
		for key, val := range v {
			path := currentPath + "." + key
			iv := iterableValuePool.Get().(*IterableValue)
			iv.data = val
			ctrl := fn(key, iv, path)
			iv.data = nil
			iterableValuePool.Put(iv)
			if ctrl == IteratorBreak {
				return nil
			}
		}
	default:
		return newOperationPathError("foreach", currentPath, fmt.Sprintf("value is not iterable: %T", data), ErrTypeMismatch)
	}

	return nil
}

// ForeachReturn is a variant that returns error (for compatibility with test expectations)
func ForeachReturn(jsonStr string, fn func(key any, item *IterableValue)) (string, error) {
	processor := getDefaultProcessor()

	data, err := processor.Get(jsonStr, ".")
	if err != nil {
		return "", err
	}

	// Execute the iteration
	foreachWithIterableValue(data, fn)

	// Return the original JSON string
	return jsonStr, nil
}

// foreachOnValue iterates over a value and applies a function
func foreachOnValue(data any, fn func(key any, value any) IteratorControl) error {
	switch v := data.(type) {
	case []any:
		for i, item := range v {
			if ctrl := fn(i, item); ctrl == IteratorBreak {
				return nil
			}
		}
	case map[string]any:
		for key, val := range v {
			if ctrl := fn(key, val); ctrl == IteratorBreak {
				return nil
			}
		}
	default:
		return newOperationError("foreach", fmt.Sprintf("value is not iterable: %T", data), ErrTypeMismatch)
	}

	return nil
}

// foreachWithPathOnValue iterates over a value and applies a function with path information
func foreachWithPathOnValue(data any, currentPath string, fn func(key any, value any, currentPath string) IteratorControl) error {
	switch v := data.(type) {
	case []any:
		for i, item := range v {
			path := fmt.Sprintf("%s[%d]", currentPath, i)
			if ctrl := fn(i, item, path); ctrl == IteratorBreak {
				return nil
			}
		}
	case map[string]any:
		for key, val := range v {
			path := currentPath + "." + key
			if ctrl := fn(key, val, path); ctrl == IteratorBreak {
				return nil
			}
		}
	default:
		return newOperationPathError("foreach", currentPath, fmt.Sprintf("value is not iterable: %T", data), ErrTypeMismatch)
	}

	return nil
}

// ForeachNested iterates over nested JSON structures
func ForeachNested(jsonStr string, fn func(key any, item *IterableValue)) {
	processor := getDefaultProcessor()

	data, err := processor.Get(jsonStr, ".")
	if err != nil {
		return
	}

	foreachNestedOnValue(data, fn)
}

// foreachNestedOnValue recursively iterates over nested values
// PERFORMANCE: Uses pooled IterableValue to reduce allocations
func foreachNestedOnValue(data any, fn func(key any, item *IterableValue)) {
	switch v := data.(type) {
	case []any:
		for i, item := range v {
			iv := iterableValuePool.Get().(*IterableValue)
			iv.data = item
			fn(i, iv)
			foreachNestedOnValue(item, fn)
			iv.data = nil
			iterableValuePool.Put(iv)
		}
	case map[string]any:
		for key, val := range v {
			iv := iterableValuePool.Get().(*IterableValue)
			iv.data = val
			fn(key, iv)
			foreachNestedOnValue(val, fn)
			iv.data = nil
			iterableValuePool.Put(iv)
		}
	}
}

// isComplexPathIterator checks if the path contains array indices or other complex syntax
func isComplexPathIterator(path string) bool {
	return strings.ContainsAny(path, "[]")
}

// navigateToPathWithArraySupport provides path navigation with array index support
func navigateToPathWithArraySupport(data any, path string) (any, error) {
	current := data

	// Parse path using internal parser
	segments, err := internal.ParsePath(path)
	if err != nil {
		return nil, err
	}

	for _, segment := range segments {
		switch segment.Type {
		case internal.PropertySegment:
			// Property access
			obj, ok := current.(map[string]any)
			if !ok {
				return nil, newPathError(segment.Key, fmt.Sprintf("cannot access property '%s' on type %T", segment.Key, current), ErrTypeMismatch)
			}
			var exists bool
			current, exists = obj[segment.Key]
			if !exists {
				return nil, newPathError(segment.Key, fmt.Sprintf("key not found: %s", segment.Key), ErrPathNotFound)
			}

		case internal.ArrayIndexSegment:
			// Array index access
			arr, ok := current.([]any)
			if !ok {
				return nil, newPathError(path, fmt.Sprintf("cannot access index on type %T", current), ErrTypeMismatch)
			}

			// Handle negative index
			index := segment.Index
			if index < 0 {
				index = len(arr) + index
			}

			if index < 0 || index >= len(arr) {
				return nil, newPathError(path, fmt.Sprintf("array index out of bounds: %d", segment.Index), ErrPathNotFound)
			}
			current = arr[index]

		case internal.ArraySliceSegment:
			// Array slice access - build slice part string
			arr, ok := current.([]any)
			if !ok {
				return nil, newPathError(path, fmt.Sprintf("cannot slice type %T", current), ErrTypeMismatch)
			}

			// Build slice string from segment flags and values
			var sliceStr string
			if segment.HasStart() {
				sliceStr += fmt.Sprintf("%d", segment.Index) // Index stores start for slices
			}
			sliceStr += ":"
			if segment.HasEnd() {
				sliceStr += fmt.Sprintf("%d", segment.End)
			}
			if segment.HasStep() {
				sliceStr += fmt.Sprintf(":%d", segment.Step)
			}

			start, end, step, err := internal.ParseSliceComponents(sliceStr)
			if err != nil {
				return nil, err
			}

			// Normalize indices
			startVal := 0
			if start != nil {
				startVal = *start
			}
			endVal := len(arr)
			if end != nil {
				endVal = *end
			}
			stepVal := 1
			if step != nil {
				stepVal = *step
			}

			// Handle negative indices
			if startVal < 0 {
				startVal = len(arr) + startVal
			}
			if endVal < 0 {
				endVal = len(arr) + endVal
			}

			// Calculate expected size before allocation
			expectedSize := 0
			if stepVal > 0 && startVal < endVal {
				for i := startVal; i < endVal && i < len(arr); i += stepVal {
					if i >= 0 {
						expectedSize++
					}
				}
			} else if stepVal < 0 && startVal > endVal {
				for i := startVal; i > endVal && i >= 0; i += stepVal {
					if i < len(arr) {
						expectedSize++
					}
				}
			}

			// Apply slice with pre-allocated result
			result := make([]any, 0, expectedSize)
			if stepVal > 0 {
				for i := startVal; i < endVal; i += stepVal {
					if i >= 0 && i < len(arr) {
						result = append(result, arr[i])
					}
				}
			} else if stepVal < 0 {
				for i := startVal; i > endVal; i += stepVal {
					if i >= 0 && i < len(arr) {
						result = append(result, arr[i])
					}
				}
			}
			current = result
		}
	}

	return current, nil
}

// navigateToPathSimple provides simple path navigation for IterableValue
func navigateToPathSimple(data any, path string) (any, error) {
	current := data
	parts := strings.Split(path, ".")

	for _, part := range parts {
		if part == "" {
			continue
		}

		switch v := current.(type) {
		case map[string]any:
			var ok bool
			current, ok = v[part]
			if !ok {
				return nil, newPathError(part, fmt.Sprintf("key not found: %s", part), ErrPathNotFound)
			}
		default:
			return nil, newPathError(part, fmt.Sprintf("cannot access property '%s' on type %T", part, current), ErrTypeMismatch)
		}
	}

	return current, nil
}

// ============================================================================
// STREAM ITERATOR - Memory-efficient iteration over large JSON data
// ============================================================================

// StreamIterator provides memory-efficient iteration over large JSON arrays
// It processes elements one at a time without loading the entire array into memory
type StreamIterator struct {
	decoder *json.Decoder
	index   int
	err     error
	done    bool
	current any
}

// NewStreamIterator creates a stream iterator from a reader
func NewStreamIterator(reader io.Reader) *StreamIterator {
	decoder := json.NewDecoder(reader)
	return &StreamIterator{
		decoder: decoder,
		index:   -1,
	}
}

// Next advances to the next element
// Returns true if there is a next element, false otherwise
func (si *StreamIterator) Next() bool {
	if si.done || si.err != nil {
		return false
	}

	// First call - check for array start
	if si.index < 0 {
		token, err := si.decoder.Token()
		if err != nil {
			si.err = err
			si.done = true
			return false
		}

		// Handle single value (not an array)
		if token != json.Delim('[') {
			si.current = token
			si.index = 0
			// Try to decode the rest if it's a complex value
			var rest any
			if err := si.decoder.Decode(&rest); err == nil {
				// It was a complex object/array
				si.current = rest
			}
			si.done = true
			return true
		}
	}

	// Check if there are more elements
	if !si.decoder.More() {
		// Consume closing bracket
		si.decoder.Token()
		si.done = true
		return false
	}

	// Decode next element
	var item any
	if err := si.decoder.Decode(&item); err != nil {
		si.err = err
		si.done = true
		return false
	}

	si.current = item
	si.index++
	return true
}

// Value returns the current element
func (si *StreamIterator) Value() any {
	return si.current
}

// Index returns the current index
func (si *StreamIterator) Index() int {
	return si.index
}

// Err returns any error encountered during iteration
func (si *StreamIterator) Err() error {
	return si.err
}

// ============================================================================
// STREAM OBJECT ITERATOR - For iterating over JSON objects
// ============================================================================

// StreamObjectIterator provides memory-efficient iteration over JSON objects
type StreamObjectIterator struct {
	decoder *json.Decoder
	key     string
	value   any
	err     error
	done    bool
	started bool
}

// NewStreamObjectIterator creates a stream object iterator from a reader
func NewStreamObjectIterator(reader io.Reader) *StreamObjectIterator {
	decoder := json.NewDecoder(reader)
	return &StreamObjectIterator{
		decoder: decoder,
	}
}

// Next advances to the next key-value pair
func (soi *StreamObjectIterator) Next() bool {
	if soi.done || soi.err != nil {
		return false
	}

	// First call - check for object start
	if !soi.started {
		token, err := soi.decoder.Token()
		if err != nil {
			soi.err = err
			soi.done = true
			return false
		}

		if token != json.Delim('{') {
			soi.done = true
			return false
		}
		soi.started = true
	}

	// Check if there are more elements
	if !soi.decoder.More() {
		// Consume closing brace
		soi.decoder.Token()
		soi.done = true
		return false
	}

	// Read key
	key, err := soi.decoder.Token()
	if err != nil {
		soi.err = err
		soi.done = true
		return false
	}

	keyStr, ok := key.(string)
	if !ok {
		soi.done = true
		return false
	}
	soi.key = keyStr

	// Read value
	var value any
	if err := soi.decoder.Decode(&value); err != nil {
		soi.err = err
		soi.done = true
		return false
	}
	soi.value = value

	return true
}

// Key returns the current key
func (soi *StreamObjectIterator) Key() string {
	return soi.key
}

// Value returns the current value
func (soi *StreamObjectIterator) Value() any {
	return soi.value
}

// Err returns any error encountered
func (soi *StreamObjectIterator) Err() error {
	return soi.err
}

// ============================================================================
// POOLED SLICE ITERATOR - For in-memory iteration with reduced allocations
// ============================================================================

// PooledSliceIterator uses pooled slices for efficient array iteration
type PooledSliceIterator struct {
	data    []any
	index   int
	current any
}

var sliceIteratorPool = sync.Pool{
	New: func() any {
		return &PooledSliceIterator{
			index: -1,
		}
	},
}

// NewPooledSliceIterator creates a pooled slice iterator
func NewPooledSliceIterator(data []any) *PooledSliceIterator {
	it := sliceIteratorPool.Get().(*PooledSliceIterator)
	it.data = data
	it.index = -1
	it.current = nil
	return it
}

// Next advances to the next element
func (it *PooledSliceIterator) Next() bool {
	it.index++
	if it.index >= len(it.data) {
		return false
	}
	it.current = it.data[it.index]
	return true
}

// Value returns the current element
func (it *PooledSliceIterator) Value() any {
	return it.current
}

// Index returns the current index
func (it *PooledSliceIterator) Index() int {
	return it.index
}

// Release returns the iterator to the pool
func (it *PooledSliceIterator) Release() {
	it.data = nil
	it.current = nil
	it.index = -1
	sliceIteratorPool.Put(it)
}

// ============================================================================
// POOLED MAP ITERATOR - For efficient object iteration
// ============================================================================

// PooledMapIterator uses pooled slices for efficient map iteration
type PooledMapIterator struct {
	data    map[string]any
	keys    []string
	index   int
	key     string
	current any
}

var mapIteratorPool = sync.Pool{
	New: func() any {
		return &PooledMapIterator{
			keys:  make([]string, 0, 16),
			index: -1,
		}
	},
}

// NewPooledMapIterator creates a pooled map iterator
func NewPooledMapIterator(m map[string]any) *PooledMapIterator {
	it := mapIteratorPool.Get().(*PooledMapIterator)
	it.data = m
	it.index = -1
	it.key = ""
	it.current = nil

	// Pre-populate keys
	it.keys = it.keys[:0]
	for k := range m {
		it.keys = append(it.keys, k)
	}

	return it
}

// Next advances to the next key-value pair
func (it *PooledMapIterator) Next() bool {
	it.index++
	if it.index >= len(it.keys) {
		return false
	}
	it.key = it.keys[it.index]
	it.current = it.data[it.key]
	return true
}

// Key returns the current key
func (it *PooledMapIterator) Key() string {
	return it.key
}

// Value returns the current value
func (it *PooledMapIterator) Value() any {
	return it.current
}

// Release returns the iterator to the pool
func (it *PooledMapIterator) Release() {
	it.data = nil
	it.key = ""
	it.current = nil
	it.index = -1
	// Keep keys slice for reuse but reset length
	if cap(it.keys) > 256 {
		it.keys = make([]string, 0, 16)
	} else {
		it.keys = it.keys[:0]
	}
	mapIteratorPool.Put(it)
}

// ============================================================================
// LAZY JSON DECODER - Parse JSON on-demand
// ============================================================================

// LazyJSONDecoder provides lazy parsing for nested structures
type LazyJSONDecoder struct {
	raw    []byte
	parsed any
	err    error
}

// NewLazyJSONDecoder creates a lazy JSON decoder
func NewLazyJSONDecoder(data []byte) *LazyJSONDecoder {
	return &LazyJSONDecoder{
		raw: data,
	}
}

// Parse parses the JSON data if not already parsed
func (l *LazyJSONDecoder) Parse() (any, error) {
	if l.parsed != nil || l.err != nil {
		return l.parsed, l.err
	}
	l.err = json.Unmarshal(l.raw, &l.parsed)
	return l.parsed, l.err
}

// GetPath gets a value at the specified path with lazy parsing
func (l *LazyJSONDecoder) GetPath(path string) (any, error) {
	_, err := l.Parse()
	if err != nil {
		return nil, err
	}

	// Use processor for path navigation
	p := getDefaultProcessor()
	return p.Get(string(l.raw), path)
}

// Raw returns the raw JSON bytes
func (l *LazyJSONDecoder) Raw() []byte {
	return l.raw
}

// ============================================================================
// BATCH ITERATOR - Efficient batch processing for large arrays
// PERFORMANCE: Processes arrays in batches to reduce per-element overhead
// ============================================================================

// BatchIterator processes arrays in batches for efficient bulk operations
type BatchIterator struct {
	data      []any
	batchSize int
	current   int
}

// NewBatchIterator creates a new batch iterator
func NewBatchIterator(data []any, batchSize int) *BatchIterator {
	if batchSize <= 0 {
		batchSize = 100 // Default batch size
	}
	return &BatchIterator{
		data:      data,
		batchSize: batchSize,
		current:   0,
	}
}

// NextBatch returns the next batch of elements
// Returns nil when no more batches are available
func (it *BatchIterator) NextBatch() []any {
	if it.current >= len(it.data) {
		return nil
	}

	end := it.current + it.batchSize
	if end > len(it.data) {
		end = len(it.data)
	}

	batch := it.data[it.current:end]
	it.current = end
	return batch
}

// HasNext returns true if there are more batches to process
func (it *BatchIterator) HasNext() bool {
	return it.current < len(it.data)
}

// Reset resets the iterator to the beginning
func (it *BatchIterator) Reset() {
	it.current = 0
}

// TotalBatches returns the total number of batches
func (it *BatchIterator) TotalBatches() int {
	return (len(it.data) + it.batchSize - 1) / it.batchSize
}

// CurrentIndex returns the current position in the array
func (it *BatchIterator) CurrentIndex() int {
	return it.current
}

// Remaining returns the number of remaining elements
func (it *BatchIterator) Remaining() int {
	if it.current >= len(it.data) {
		return 0
	}
	return len(it.data) - it.current
}

// ============================================================================
// PARALLEL ITERATOR - Concurrent processing for CPU-bound operations
// PERFORMANCE: Parallelizes processing across multiple goroutines
// ============================================================================

// ParallelIterator processes arrays in parallel using worker goroutines
type ParallelIterator struct {
	data    []any
	workers int
	sem     chan struct{}
}

// NewParallelIterator creates a new parallel iterator
func NewParallelIterator(data []any, workers int) *ParallelIterator {
	if workers <= 0 {
		workers = 4 // Default worker count
	}
	if workers > len(data) {
		workers = len(data)
		if workers == 0 {
			workers = 1
		}
	}
	return &ParallelIterator{
		data:    data,
		workers: workers,
		sem:     make(chan struct{}, workers),
	}
}

// ForEach processes each element in parallel using the provided function
// The function receives the index and value of each element
// Returns the first error encountered, or nil if all operations succeed
func (it *ParallelIterator) ForEach(fn func(int, any) error) error {
	errCh := make(chan error, 1)
	var wg sync.WaitGroup
	var hasError int32

	for i, item := range it.data {
		// Check if we already have an error
		if atomic.LoadInt32(&hasError) == 1 {
			break
		}

		it.sem <- struct{}{} // Acquire semaphore
		wg.Add(1)

		go func(idx int, val any) {
			defer wg.Done()
			defer func() { <-it.sem }() // Release semaphore

			if atomic.LoadInt32(&hasError) == 1 {
				return
			}

			if err := fn(idx, val); err != nil {
				if atomic.CompareAndSwapInt32(&hasError, 0, 1) {
					select {
					case errCh <- err:
					default:
					}
				}
			}
		}(i, item)
	}

	wg.Wait()

	select {
	case err := <-errCh:
		return err
	default:
		return nil
	}
}

// ForEachBatch processes elements in batches in parallel
// Each batch is processed by a single goroutine
func (it *ParallelIterator) ForEachBatch(batchSize int, fn func(int, []any) error) error {
	if batchSize <= 0 {
		batchSize = 100
	}

	// Create batches
	batches := make([][]any, 0)
	for i := 0; i < len(it.data); i += batchSize {
		end := i + batchSize
		if end > len(it.data) {
			end = len(it.data)
		}
		batches = append(batches, it.data[i:end])
	}

	errCh := make(chan error, 1)
	var wg sync.WaitGroup
	var hasError int32

	for batchIdx, batch := range batches {
		if atomic.LoadInt32(&hasError) == 1 {
			break
		}

		it.sem <- struct{}{}
		wg.Add(1)

		go func(idx int, b []any) {
			defer wg.Done()
			defer func() { <-it.sem }()

			if atomic.LoadInt32(&hasError) == 1 {
				return
			}

			if err := fn(idx, b); err != nil {
				if atomic.CompareAndSwapInt32(&hasError, 0, 1) {
					select {
					case errCh <- err:
					default:
					}
				}
			}
		}(batchIdx, batch)
	}

	wg.Wait()

	select {
	case err := <-errCh:
		return err
	default:
		return nil
	}
}

// Map applies a transformation function to each element in parallel
// Returns a new slice with the transformed values
func (it *ParallelIterator) Map(transform func(int, any) (any, error)) ([]any, error) {
	result := make([]any, len(it.data))
	var mu sync.Mutex
	var hasError int32
	var firstError error

	err := it.ForEach(func(idx int, val any) error {
		transformed, err := transform(idx, val)
		if err != nil {
			return err
		}

		mu.Lock()
		result[idx] = transformed
		mu.Unlock()

		return nil
	})

	if err != nil {
		return nil, err
	}

	if atomic.LoadInt32(&hasError) == 1 {
		return nil, firstError
	}

	return result, nil
}

// Filter filters elements in parallel using a predicate function
// Returns a new slice with elements that pass the predicate
func (it *ParallelIterator) Filter(predicate func(int, any) bool) []any {
	var mu sync.Mutex
	result := make([]any, 0)

	it.ForEach(func(idx int, val any) error {
		if predicate(idx, val) {
			mu.Lock()
			result = append(result, val)
			mu.Unlock()
		}
		return nil
	})

	return result
}

// ============================================================================
// ITERABLE VALUE POOL - Reduces allocations for IterableValue
// PERFORMANCE: Pooling reduces GC pressure for frequent iterations
// NOTE: iterableValuePool is declared earlier in the file (near line 159)
// ============================================================================

// NewPooledIterableValue creates an IterableValue from the pool
func NewPooledIterableValue(data any) *IterableValue {
	iv := iterableValuePool.Get().(*IterableValue)
	iv.data = data
	return iv
}

// Release returns the IterableValue to the pool
func (iv *IterableValue) Release() {
	iv.data = nil
	iterableValuePool.Put(iv)
}

// ============================================================================
// CONFIGURABLE STREAM ITERATOR - Enhanced StreamIterator with options
// PERFORMANCE: Configurable buffer sizes and prefetch for large data
// ============================================================================

// StreamIteratorConfig holds configuration for StreamIterator
type StreamIteratorConfig struct {
	BufferSize     int  // Buffer size for reading (default: 4096)
	EnablePrefetch bool // Enable prefetching next element
}

// DefaultStreamIteratorConfig returns the default configuration
func DefaultStreamIteratorConfig() StreamIteratorConfig {
	return StreamIteratorConfig{
		BufferSize:     4096,
		EnablePrefetch: false,
	}
}

// NewStreamIteratorWithConfig creates a stream iterator with custom configuration
func NewStreamIteratorWithConfig(reader io.Reader, config StreamIteratorConfig) *StreamIterator {
	// For now, this is the same as NewStreamIterator
	// Future enhancement: implement prefetching with config.EnablePrefetch
	return NewStreamIterator(reader)
}
