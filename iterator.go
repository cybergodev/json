package json

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/cybergodev/json/internal"
)

// pathType represents the complexity level of a path
type pathType int

const (
	// pathTypeSimple indicates a single key with no dots or brackets
	pathTypeSimple pathType = iota
	// pathTypeComplex indicates a path containing dots or brackets
	pathTypeComplex
)

// IteratorControl represents control flags for iteration operations.
// Used by Foreach* functions to control iteration flow.
type IteratorControl int

const (
	IteratorNormal IteratorControl = iota // continue normally
	// IteratorContinue skips the current item and continues iteration
	IteratorContinue
	// IteratorBreak stops iteration entirely
	IteratorBreak
)

// pathTypeCacheShard represents a single shard of the path type cache
// PERFORMANCE: Sharding reduces lock contention for concurrent access
// SECURITY: Added size limit with LRU-style eviction to prevent unbounded growth
type pathTypeCacheShard struct {
	mu      sync.RWMutex
	entries map[string]pathType
	size    int
}

// pathTypeCacheShards is a sharded cache for path type results
// Using 16 shards for good distribution with minimal overhead
var pathTypeCacheShards [16]pathTypeCacheShard

// maxEntriesPerShard limits the number of entries per shard to prevent memory exhaustion
const maxEntriesPerShard = 256

// init initializes the path type cache shards
func init() {
	for i := range pathTypeCacheShards {
		pathTypeCacheShards[i].entries = make(map[string]pathType, 64)
		pathTypeCacheShards[i].size = 0
	}
}

// clearPathTypeCache clears the global path type cache.
// Called during processor shutdown to prevent memory accumulation from cached path types.
func clearPathTypeCache() {
	for i := range pathTypeCacheShards {
		shard := &pathTypeCacheShards[i]
		shard.mu.Lock()
		shard.entries = make(map[string]pathType, 64)
		shard.size = 0
		shard.mu.Unlock()
	}
}

// getPathTypeShard returns the shard for a path using FNV-1a hash
func getPathTypeShard(path string) *pathTypeCacheShard {
	h := internal.HashStringFNV1a(path)
	return &pathTypeCacheShards[h&15]
}

// getPathType determines if a path is simple or complex
// Simple paths are single keys with no dots or brackets
// SECURITY: Added size limit with eviction to prevent unbounded memory growth
// FIX: Added double-check pattern to prevent race condition between RLock and Lock
// FIX: Use deterministic eviction based on hash to avoid unpredictable map iteration
func getPathType(path string) pathType {
	// Check cache first (only for short paths to avoid memory bloat)
	if len(path) <= 64 {
		shard := getPathTypeShard(path)

		// Fast path: read lock only
		shard.mu.RLock()
		if pt, ok := shard.entries[path]; ok {
			shard.mu.RUnlock()
			return pt
		}
		shard.mu.RUnlock()

		// Calculate path type (no lock needed)
		var pt pathType
		if strings.ContainsAny(path, ".[]") {
			pt = pathTypeComplex
		} else {
			pt = pathTypeSimple
		}

		// Slow path: write lock with double-check to prevent duplicate entries
		shard.mu.Lock()

		// Double-check: another goroutine may have filled the entry
		if existing, ok := shard.entries[path]; ok {
			shard.mu.Unlock()
			return existing
		}

		// Evict entries if shard is full using deterministic hash-based eviction
		if shard.size >= maxEntriesPerShard {
			evictCount := maxEntriesPerShard / 2

			// Collect keys for deterministic eviction
			keys := make([]string, 0, len(shard.entries))
			for k := range shard.entries {
				keys = append(keys, k)
			}

			// Sort by hash value for deterministic eviction order
			sort.Slice(keys, func(i, j int) bool {
				return internal.HashStringFNV1a(keys[i]) < internal.HashStringFNV1a(keys[j])
			})

			// Evict entries with lowest hash values
			for i := 0; i < evictCount && i < len(keys); i++ {
				delete(shard.entries, keys[i])
			}
			shard.size -= min(evictCount, len(keys))
		}

		shard.entries[path] = pt
		shard.size++
		shard.mu.Unlock()

		return pt
	}

	var pt pathType
	if strings.ContainsAny(path, ".[]") {
		pt = pathTypeComplex
	} else {
		pt = pathTypeSimple
	}

	return pt
}

// safeTypeAssert performs a safe type assertion with generics
func safeTypeAssert[T any](value any) (T, bool) {
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
		// Use comma-ok to avoid panic if converted value doesn't satisfy T
		if result, ok := converted.Interface().(T); ok {
			return result, true
		}
	}

	return zero, false
}

// Iterator represents an iterator over JSON data for sequential access.
// Supports iteration over both arrays and objects.
// Thread-safe for single goroutine use; for concurrent access, create separate iterators.
//
// Example:
//
//	// Iterate over an array
//	data, _ := json.ParseAny(`[1, 2, 3]`)
//	iter := json.NewIterator(data)
//	for iter.HasNext() {
//	    value, _ := iter.Next()
//	    fmt.Println(value)
//	}
//
//	// Iterate over an object
//	data, _ := json.ParseAny(`{"a": 1, "b": 2}`)
//	iter := json.NewIterator(data)
//	for iter.HasNext() {
//	    value, _ := iter.Next()
//	    fmt.Println(value)
//	}
type Iterator struct {
	data     any
	position int
	keys     []string // Cached keys for map iteration
	keysOnce sync.Once
}

// NewIterator creates a new Iterator over the provided data.
// Creates an iterator for traversing arrays and objects.
//
// The optional cfg parameter is reserved for future configuration options
// and maintains API consistency with other constructors. Currently no
// configuration options affect Iterator behavior.
//
// Example:
//
//	data, _ := json.ParseAny(`{"name": "Alice", "age": 30}`)
//	iter := json.NewIterator(data)
//	for iter.HasNext() {
//	    value, ok := iter.Next()
//	    if !ok {
//	        break
//	    }
//	    fmt.Println(value)
//	}
func NewIterator(data any, cfg ...Config) *Iterator {
	// Note: cfg parameter is reserved for future use.
	// Currently Iterator does not use any configuration options.
	// The parameter is kept for API consistency.
	return &Iterator{
		data:     data,
		position: 0,
	}
}

// initKeysOnce lazily initializes cached keys for map iteration.
// Thread-safe via sync.Once; avoids allocating a new slice on every Next() call.
func (it *Iterator) initKeysOnce() {
	it.keysOnce.Do(func() {
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
	})
}

// iterableValuePool pools IterableValue objects to reduce allocations
// PERFORMANCE: Significant reduction in allocations during nested iteration
var iterableValuePool = sync.Pool{
	New: func() any {
		return &IterableValue{}
	},
}

// releaseIterableValues returns a slice of IterableValue objects to the pool.
func releaseIterableValues(items []*IterableValue) {
	for _, iv := range items {
		iv.data = nil
		iterableValuePool.Put(iv)
	}
}

// HasNext checks if there are more elements to iterate.
// Returns true if the iterator has not reached the end of the data.
// For arrays, checks if position < array length.
// For objects, checks if position < number of keys.
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

// Next returns the next element and advances the iterator.
// Returns (value, true) if an element is available, or (nil, false) at the end.
// For arrays, returns the array element at the current position.
// For objects, returns the value at the current key position.
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

// Reset clears the iterator state and releases cached resources.
// After calling Reset, the iterator can be reused with new data via ResetWith.
// This is useful for reducing allocations when iterating over multiple JSON structures.
//
// Example:
//
//	iter := json.NewIterator(data1)
//	for iter.HasNext() {
//	    iter.Next()
//	}
//	iter.Reset() // Clear cached keys
//	iter.ResetWith(data2) // Reuse iterator with new data
func (it *Iterator) Reset() {
	it.data = nil
	it.position = 0
	// Clear cached keys to release memory
	it.keys = nil
	// Reset sync.Once to allow re-initialization with new data
	it.keysOnce = sync.Once{}
}

// ResetWith clears the iterator state and initializes it with new data.
// This allows reusing the iterator to avoid allocations.
//
// Example:
//
//	iter := json.NewIterator(data1)
//	// ... iterate over data1 ...
//	iter.ResetWith(data2) // Reuse iterator with new data
//	// ... iterate over data2 ...
func (it *Iterator) ResetWith(data any) {
	it.data = data
	it.position = 0
	// Clear cached keys to release memory
	it.keys = nil
	// Reset sync.Once to allow re-initialization with new data
	it.keysOnce = sync.Once{}
}

// IterableValue wraps a value to provide convenient access methods during iteration.
// Used by Foreach and ForeachKey callback functions to provide structured access.
// Note: Simplified to avoid resource leaks from holding processor/iterator references.
//
// Example:
//
//	err := processor.Foreach(data, "items", func(item json.IterableValue) error {
//	    name, _ := item.GetString("name")
//	    age, _ := item.GetInt("age")
//	    fmt.Printf("Name: %s, Age: %d\n", name, age)
//	    return nil
//	})
type IterableValue struct {
	data any
}

// newIterableValue creates an IterableValue from data.
// This is primarily used internally by iteration functions.
func newIterableValue(data any) *IterableValue {
	return &IterableValue{data: data}
}

// GetData returns the underlying data
func (iv *IterableValue) GetData() any {
	return iv.data
}

// Break returns a signal to stop iteration without error.
// Use it in ForeachFile/ForeachFileChunked callback to exit early.
//
// Example:
//
//	processor.ForeachFile("data.json", func(key any, item *json.IterableValue) error {
//	    if item.GetInt("id") == targetId {
//	        return item.Break() // stop iteration
//	    }
//	    return nil // continue
//	})
func (iv *IterableValue) Break() error {
	return errBreak
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
	switch getPathType(key) {
	case pathTypeComplex:
		val := iv.Get(key)
		if val == nil {
			return ""
		}
		if str, ok := val.(string); ok {
			return str
		}
		return convertToString(val)
	case pathTypeSimple:
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

		return convertToString(val)
	}
	return ""
}

// GetInt returns an int value by key or path
// Supports path navigation with dot notation and array indices (e.g., "user.age" or "users[0].id")
func (iv *IterableValue) GetInt(key string) int {
	// PERFORMANCE: Use cached path type check instead of strings.ContainsAny
	switch getPathType(key) {
	case pathTypeComplex:
		val := iv.Get(key)
		if val == nil {
			return 0
		}
		if result, ok := convertToInt(val); ok {
			return result
		}
		return 0
	case pathTypeSimple:
		obj, ok := iv.data.(map[string]any)
		if !ok {
			return 0
		}

		val, exists := obj[key]
		if !exists {
			return 0
		}

		if result, ok := convertToInt(val); ok {
			return result
		}
	}
	return 0
}

// GetFloat64 returns a float64 value by key or path
// Supports path navigation with dot notation and array indices
func (iv *IterableValue) GetFloat64(key string) float64 {
	// PERFORMANCE: Use cached path type check instead of strings.ContainsAny
	switch getPathType(key) {
	case pathTypeComplex:
		val := iv.Get(key)
		if val == nil {
			return 0
		}
		if result, ok := convertToFloat64(val); ok {
			return result
		}
		return 0
	case pathTypeSimple:
		obj, ok := iv.data.(map[string]any)
		if !ok {
			return 0
		}

		val, exists := obj[key]
		if !exists {
			return 0
		}

		if result, ok := convertToFloat64(val); ok {
			return result
		}
	}
	return 0
}

// GetBool returns a bool value by key or path
// Supports path navigation with dot notation and array indices
func (iv *IterableValue) GetBool(key string) bool {
	// PERFORMANCE: Use cached path type check instead of strings.ContainsAny
	switch getPathType(key) {
	case pathTypeComplex:
		val := iv.Get(key)
		if val == nil {
			return false
		}
		if result, ok := convertToBool(val); ok {
			return result
		}
		return false
	case pathTypeSimple:
		obj, ok := iv.data.(map[string]any)
		if !ok {
			return false
		}

		val, exists := obj[key]
		if !exists {
			return false
		}

		if result, ok := convertToBool(val); ok {
			return result
		}
	}
	return false
}

// GetArray returns an array value by key or path
// Supports path navigation with dot notation and array indices
func (iv *IterableValue) GetArray(key string) []any {
	// PERFORMANCE: Use cached path type check instead of strings.ContainsAny
	switch getPathType(key) {
	case pathTypeComplex:
		val := iv.Get(key)
		if val == nil {
			return nil
		}
		if arr, ok := val.([]any); ok {
			return arr
		}
		return nil
	case pathTypeSimple:
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
	switch getPathType(key) {
	case pathTypeComplex:
		val := iv.Get(key)
		if val == nil {
			return nil
		}
		if result, ok := val.(map[string]any); ok {
			return result
		}
		return nil
	case pathTypeSimple:
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
	switch getPathType(key) {
	case pathTypeComplex:
		val := iv.Get(key)
		if val == nil {
			return defaultValue
		}
		return val
	case pathTypeSimple:
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
	switch getPathType(key) {
	case pathTypeComplex:
		val := iv.Get(key)
		if val == nil {
			return defaultValue
		}
		if str, ok := val.(string); ok {
			return str
		}
		return defaultValue
	case pathTypeSimple:
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
	switch getPathType(key) {
	case pathTypeComplex:
		val := iv.Get(key)
		if val == nil {
			return defaultValue
		}
		if result, ok := convertToInt(val); ok {
			return result
		}
		return defaultValue
	case pathTypeSimple:
		obj, ok := iv.data.(map[string]any)
		if !ok {
			return defaultValue
		}

		val, exists := obj[key]
		if !exists {
			return defaultValue
		}

		if result, ok := convertToInt(val); ok {
			return result
		}
	}
	return defaultValue
}

// GetFloat64WithDefault returns a float64 value by key or path with a default fallback
// Supports path navigation with dot notation and array indices
func (iv *IterableValue) GetFloat64WithDefault(key string, defaultValue float64) float64 {
	// PERFORMANCE: Use cached path type check instead of strings.ContainsAny
	switch getPathType(key) {
	case pathTypeComplex:
		val := iv.Get(key)
		if val == nil {
			return defaultValue
		}
		if result, ok := convertToFloat64(val); ok {
			return result
		}
		return defaultValue
	case pathTypeSimple:
		obj, ok := iv.data.(map[string]any)
		if !ok {
			return defaultValue
		}

		val, exists := obj[key]
		if !exists {
			return defaultValue
		}

		if result, ok := convertToFloat64(val); ok {
			return result
		}
	}
	return defaultValue
}

// GetBoolWithDefault returns a bool value by key or path with a default fallback
// Supports path navigation with dot notation and array indices
func (iv *IterableValue) GetBoolWithDefault(key string, defaultValue bool) bool {
	// PERFORMANCE: Use cached path type check instead of strings.ContainsAny
	switch getPathType(key) {
	case pathTypeComplex:
		val := iv.Get(key)
		if val == nil {
			return defaultValue
		}
		if result, ok := convertToBool(val); ok {
			return result
		}
		return defaultValue
	case pathTypeSimple:
		obj, ok := iv.data.(map[string]any)
		if !ok {
			return defaultValue
		}

		val, exists := obj[key]
		if !exists {
			return defaultValue
		}

		if result, ok := convertToBool(val); ok {
			return result
		}
	}
	return defaultValue
}

// Exists checks if a key or path exists in the object
// Supports path navigation with dot notation and array indices
func (iv *IterableValue) Exists(key string) bool {
	// PERFORMANCE: Use cached path type check instead of strings.ContainsAny
	switch getPathType(key) {
	case pathTypeComplex:
		return iv.Get(key) != nil
	case pathTypeSimple:
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
	switch getPathType(key) {
	case pathTypeComplex:
		val := iv.Get(key)
		return val == nil
	case pathTypeSimple:
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
	switch getPathType(key) {
	case pathTypeComplex:
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
	case pathTypeSimple:
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

// ForeachWithPathAndControl iterates over JSON arrays or objects and applies a function.
// This is the 3-parameter version used by most code.
// Accepts optional Config for consistency with Processor.ForeachWithPathAndControl.
func ForeachWithPathAndControl(jsonStr, path string, fn func(key any, value any) IteratorControl, cfg ...Config) error {
	processor := getDefaultProcessor()
	if processor == nil {
		return errInternalError
	}
	if processor.IsClosed() {
		return ErrProcessorClosed
	}

	data, err := processor.Get(jsonStr, path, cfg...)
	if err != nil {
		return err
	}

	return foreachOnValue(data, fn)
}

// Foreach iterates over JSON arrays or objects with simplified signature (for test compatibility).
// Accepts optional Config for consistency with Processor.Foreach.
func Foreach(jsonStr string, fn func(key any, item *IterableValue), cfg ...Config) {
	processor := getDefaultProcessor()
	if processor == nil || processor.IsClosed() {
		return
	}

	data, err := processor.Get(jsonStr, ".", cfg...)
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

// ForeachWithPath iterates over JSON arrays or objects with simplified signature (for test compatibility).
// Accepts optional Config for consistency with Processor.ForeachWithPath.
func ForeachWithPath(jsonStr, path string, fn func(key any, item *IterableValue), cfg ...Config) error {
	processor := getDefaultProcessor()
	if processor == nil {
		return errInternalError
	}
	if processor.IsClosed() {
		return ErrProcessorClosed
	}

	data, err := processor.Get(jsonStr, path, cfg...)
	if err != nil {
		return err
	}

	foreachWithIterableValue(data, fn)
	return nil
}

// foreachWithPathIterableValue iterates with IterableValue and path information
// PERFORMANCE: Uses pooled IterableValue to reduce allocations
func foreachWithPathIterableValue(data any, currentPath string, fn func(key any, item *IterableValue, currentPath string) IteratorControl) error {
	switch v := data.(type) {
	case []any:
		for i, item := range v {
			// PERFORMANCE: Build path using strconv.AppendInt instead of fmt.Sprintf
			var buf []byte
			buf = append(buf, currentPath...)
			buf = append(buf, '[')
			buf = strconv.AppendInt(buf, int64(i), 10)
			buf = append(buf, ']')
			path := string(buf)
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
			// PERFORMANCE: Build path using append instead of string concatenation
			var buf []byte
			buf = append(buf, currentPath...)
			buf = append(buf, '.')
			buf = append(buf, key...)
			path := string(buf)
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

// ForeachReturn is a variant that returns error (for compatibility with test expectations).
// Accepts optional Config for consistency with Processor.ForeachReturn.
func ForeachReturn(jsonStr string, fn func(key any, item *IterableValue), cfg ...Config) (string, error) {
	processor := getDefaultProcessor()
	if processor == nil {
		return "", errInternalError
	}
	if processor.IsClosed() {
		return "", ErrProcessorClosed
	}

	data, err := processor.Get(jsonStr, ".", cfg...)
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

// ForeachNested iterates over nested JSON structures.
// Accepts optional Config for consistency with Processor.ForeachNested.
func ForeachNested(jsonStr string, fn func(key any, item *IterableValue), cfg ...Config) {
	processor := getDefaultProcessor()
	if processor == nil || processor.IsClosed() {
		return
	}

	data, err := processor.Get(jsonStr, ".", cfg...)
	if err != nil {
		return
	}

	foreachNestedOnValue(data, fn)
}

// foreachNestedMaxDepth limits recursion depth for nested iteration to prevent stack overflow.
const foreachNestedMaxDepth = 200

// foreachNestedOnValue recursively iterates over nested values.
// PERFORMANCE: Uses pooled IterableValue to reduce allocations.
// SECURITY: Depth-limited to prevent stack overflow from deeply nested structures.
func foreachNestedOnValue(data any, fn func(key any, item *IterableValue)) {
	foreachNestedOnValueDepth(data, fn, 0)
}

// foreachNestedOnValueDepth is the depth-tracked implementation of foreachNestedOnValue.
func foreachNestedOnValueDepth(data any, fn func(key any, item *IterableValue), depth int) {
	if depth > foreachNestedMaxDepth {
		return
	}
	switch v := data.(type) {
	case []any:
		for i, item := range v {
			iv := iterableValuePool.Get().(*IterableValue)
			iv.data = item
			fn(i, iv)
			foreachNestedOnValueDepth(item, fn, depth+1)
			iv.data = nil
			iterableValuePool.Put(iv)
		}
	case map[string]any:
		for key, val := range v {
			iv := iterableValuePool.Get().(*IterableValue)
			iv.data = val
			fn(key, iv)
			foreachNestedOnValueDepth(val, fn, depth+1)
			iv.data = nil
			iterableValuePool.Put(iv)
		}
	}
}

// foreachWithIterableValueError iterates with error-returning callback
// PERFORMANCE: Uses pooled IterableValue to reduce allocations
func foreachWithIterableValueError(data any, fn func(key any, item *IterableValue) error) error {
	switch v := data.(type) {
	case []any:
		for i, item := range v {
			iv := iterableValuePool.Get().(*IterableValue)
			iv.data = item
			err := fn(i, iv)
			iv.data = nil
			iterableValuePool.Put(iv)
			if err != nil {
				if errors.Is(err, errBreak) {
					return nil // Break is not an error
				}
				return err
			}
		}
	case map[string]any:
		for key, val := range v {
			iv := iterableValuePool.Get().(*IterableValue)
			iv.data = val
			err := fn(key, iv)
			iv.data = nil
			iterableValuePool.Put(iv)
			if err != nil {
				if errors.Is(err, errBreak) {
					return nil // Break is not an error
				}
				return err
			}
		}
	}
	return nil
}

// foreachNestedOnValueError recursively iterates with error-returning callback.
// PERFORMANCE: Uses pooled IterableValue to reduce allocations.
// SECURITY: Depth-limited to prevent stack overflow from deeply nested structures.
func foreachNestedOnValueError(data any, fn func(key any, item *IterableValue) error) error {
	return foreachNestedOnValueErrorDepth(data, fn, 0)
}

// foreachNestedOnValueErrorDepth is the depth-tracked implementation of foreachNestedOnValueError.
func foreachNestedOnValueErrorDepth(data any, fn func(key any, item *IterableValue) error, depth int) error {
	if depth > foreachNestedMaxDepth {
		return fmt.Errorf("foreach nested depth limit exceeded: maximum depth is %d", foreachNestedMaxDepth)
	}
	switch v := data.(type) {
	case []any:
		for i, item := range v {
			iv := iterableValuePool.Get().(*IterableValue)
			iv.data = item
			err := fn(i, iv)
			if err != nil {
				iv.data = nil
				iterableValuePool.Put(iv)
				if errors.Is(err, errBreak) {
					return nil
				}
				return err
			}
			err = foreachNestedOnValueErrorDepth(item, fn, depth+1)
			iv.data = nil
			iterableValuePool.Put(iv)
			if err != nil {
				return err
			}
		}
	case map[string]any:
		for key, val := range v {
			iv := iterableValuePool.Get().(*IterableValue)
			iv.data = val
			err := fn(key, iv)
			if err != nil {
				iv.data = nil
				iterableValuePool.Put(iv)
				if errors.Is(err, errBreak) {
					return nil
				}
				return err
			}
			err = foreachNestedOnValueErrorDepth(val, fn, depth+1)
			iv.data = nil
			iterableValuePool.Put(iv)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// isComplexPathIterator checks if the path contains array indices or other complex syntax
func isComplexPathIterator(path string) bool {
	return strings.ContainsAny(path, "[]")
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

// StreamIterator provides memory-efficient iteration over large JSON arrays.
// It processes elements one at a time without loading the entire array into memory.
type StreamIterator struct {
	decoder    *json.Decoder
	index      int
	err        error
	done       bool
	current    any
	buffer     *bufio.Reader // Buffered reader for improved I/O performance
	bufferSize int           // Configured buffer size
}

// NewStreamIterator creates a stream iterator from a reader with default settings.
// The optional cfg parameter allows customization using the unified Config pattern.
//
// Example:
//
//	// Default settings
//	iter := json.NewStreamIterator(reader)
//
//	// With custom configuration
//	cfg := json.DefaultConfig()
//	cfg.BufferSize = 64 * 1024
//	iter := json.NewStreamIterator(reader, cfg)
func NewStreamIterator(reader io.Reader, cfg ...Config) *StreamIterator {
	var config Config
	if len(cfg) > 0 {
		config = cfg[0]
	} else {
		config = DefaultConfig()
	}

	// Get buffer size from config, default to 32KB
	bufSize := config.BufferSize
	if bufSize <= 0 {
		bufSize = 32 * 1024
	}

	// Create buffered reader for improved I/O performance
	buffered := bufio.NewReaderSize(reader, bufSize)
	decoder := json.NewDecoder(buffered)

	return &StreamIterator{
		decoder:    decoder,
		index:      -1,
		buffer:     buffered,
		bufferSize: bufSize,
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

// NewStreamObjectIterator creates a stream object iterator from a reader.
// The optional cfg parameter allows customization using the unified Config pattern.
// When config is provided, cfg.BufferSize is used for buffered reading.
//
// Example:
//
//	// Default settings
//	iter := json.NewStreamObjectIterator(reader)
//
//	// With custom buffer size
//	cfg := json.DefaultConfig()
//	cfg.BufferSize = 128 * 1024
//	iter := json.NewStreamObjectIterator(reader, cfg)
func NewStreamObjectIterator(reader io.Reader, cfg ...Config) *StreamObjectIterator {
	var config Config
	if len(cfg) > 0 {
		config = cfg[0]
	} else {
		config = DefaultConfig()
	}

	bufSize := config.BufferSize
	if bufSize <= 0 {
		bufSize = 32 * 1024 // Default buffer size
	}

	// Create buffered reader for improved I/O performance
	buffered := bufio.NewReaderSize(reader, bufSize)
	decoder := json.NewDecoder(buffered)

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

// pooledSliceIterator uses pooled slices for efficient array iteration
type pooledSliceIterator struct {
	data    []any
	index   int
	current any
}

var sliceIteratorPool = sync.Pool{
	New: func() any {
		return &pooledSliceIterator{
			index: -1,
		}
	},
}

// newPooledSliceIterator creates a pooled slice iterator
func newPooledSliceIterator(data []any) *pooledSliceIterator {
	it := sliceIteratorPool.Get().(*pooledSliceIterator)
	it.data = data
	it.index = -1
	it.current = nil
	return it
}

// Next advances to the next element
func (it *pooledSliceIterator) Next() bool {
	it.index++
	if it.index >= len(it.data) {
		return false
	}
	it.current = it.data[it.index]
	return true
}

// Value returns the current element
func (it *pooledSliceIterator) Value() any {
	return it.current
}

// Index returns the current index
func (it *pooledSliceIterator) Index() int {
	return it.index
}

// Release returns the iterator to the pool
func (it *pooledSliceIterator) Release() {
	it.data = nil
	it.current = nil
	it.index = -1
	sliceIteratorPool.Put(it)
}

// ============================================================================
// POOLED MAP ITERATOR - For efficient object iteration
// ============================================================================

// pooledMapIterator uses pooled slices for efficient map iteration
type pooledMapIterator struct {
	data    map[string]any
	keys    []string
	index   int
	key     string
	current any
}

var mapIteratorPool = sync.Pool{
	New: func() any {
		return &pooledMapIterator{
			keys:  make([]string, 0, 16),
			index: -1,
		}
	},
}

// newPooledMapIterator creates a pooled map iterator
func newPooledMapIterator(m map[string]any) *pooledMapIterator {
	it := mapIteratorPool.Get().(*pooledMapIterator)
	it.data = m
	it.index = -1
	it.key = ""
	it.current = nil

	// PERFORMANCE: Ensure keys slice has sufficient capacity
	// This avoids repeated slice growth during append
	mapLen := len(m)
	if cap(it.keys) < mapLen {
		it.keys = make([]string, 0, mapLen)
	} else {
		it.keys = it.keys[:0]
	}

	// Pre-populate keys without interning (faster for one-time iteration)
	for k := range m {
		it.keys = append(it.keys, k)
	}

	return it
}

// Next advances to the next key-value pair
func (it *pooledMapIterator) Next() bool {
	it.index++
	if it.index >= len(it.keys) {
		return false
	}
	it.key = it.keys[it.index]
	it.current = it.data[it.key]
	return true
}

// Key returns the current key
func (it *pooledMapIterator) Key() string {
	return it.key
}

// Value returns the current value
func (it *pooledMapIterator) Value() any {
	return it.current
}

// Release returns the iterator to the pool
func (it *pooledMapIterator) Release() {
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
// BATCH ITERATOR - Efficient batch processing for large arrays
// PERFORMANCE: Processes arrays in batches to reduce per-element overhead
// ============================================================================

// BatchIterator processes arrays in batches for efficient bulk operations
type BatchIterator struct {
	data      []any
	batchSize int
	current   int
}

// NewBatchIterator creates a new batch iterator.
// The optional cfg parameter allows customization using the unified Config pattern.
// When config is provided, cfg.MaxBatchSize is used as the batch size.
//
// Example:
//
//	// Default settings (batch size = 100)
//	iter := json.NewBatchIterator(data)
//
//	// With custom batch size
//	cfg := json.DefaultConfig()
//	cfg.MaxBatchSize = 50
//	iter := json.NewBatchIterator(data, cfg)
//
//	// Legacy pattern (backward compatible)
//	iter := json.NewBatchIteratorWithSize(data, 50)
func NewBatchIterator(data []any, cfg ...Config) *BatchIterator {
	var config Config
	if len(cfg) > 0 {
		config = cfg[0]
	} else {
		config = DefaultConfig()
	}

	batchSize := config.MaxBatchSize
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

// TotalBatches returns the total number of batches.
// Returns 0 if batch size is not positive.
func (it *BatchIterator) TotalBatches() int {
	if it.batchSize <= 0 {
		return 0
	}
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
	done    chan struct{}
}

// NewParallelIterator creates a new parallel iterator.
// The optional cfg parameter allows customization using the unified Config pattern.
// When config is provided, cfg.MaxConcurrency is used as the worker count.
//
// Example:
//
//	// Default settings (workers = 4)
//	iter := json.NewParallelIterator(data)
//
//	// With custom worker count
//	cfg := json.DefaultConfig()
//	cfg.MaxConcurrency = 8
//	iter := json.NewParallelIterator(data, cfg)
//
//	// Legacy pattern (backward compatible)
//	iter := json.NewParallelIteratorWithWorkers(data, 8)
func NewParallelIterator(data []any, cfg ...Config) *ParallelIterator {
	var config Config
	if len(cfg) > 0 {
		config = cfg[0]
	} else {
		config = DefaultConfig()
	}

	workers := config.MaxConcurrency
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
		done:    make(chan struct{}),
	}
}

// ForEach processes each element in parallel using the provided function
// The function receives the index and value of each element
// Returns the first error encountered, or nil if all operations succeed
func (it *ParallelIterator) ForEach(fn func(int, any) error) error {
	return it.ForEachWithContext(context.Background(), fn)
}

// ForEachWithContext processes each element in parallel with context support for cancellation
// The function receives the index and value of each element
// Returns the first error encountered, or ctx.Err() if context is cancelled
// RESOURCE FIX: Added context support for graceful goroutine termination
func (it *ParallelIterator) ForEachWithContext(ctx context.Context, fn func(int, any) error) error {
	errCh := make(chan error, 1)
	var wg sync.WaitGroup
	var hasError int32

	for i, item := range it.data {
		// Check context cancellation and close signal
		select {
		case <-ctx.Done():
			wg.Wait()
			return ctx.Err()
		case <-it.done:
			wg.Wait()
			return nil
		default:
		}

		// Check if we already have an error
		if atomic.LoadInt32(&hasError) == 1 {
			break
		}

		select {
		case it.sem <- struct{}{}: // Acquire semaphore
		case <-ctx.Done():
			wg.Wait()
			return ctx.Err()
		case <-it.done:
			wg.Wait()
			return nil
		}
		wg.Add(1)

		go func(idx int, val any) {
			defer wg.Done()
			defer func() { <-it.sem }()

			select {
			case <-ctx.Done():
				return
			case <-it.done:
				return
			default:
			}

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
	return it.ForEachBatchWithContext(context.Background(), batchSize, fn)
}

// ForEachBatchWithContext processes elements in batches with context support for cancellation
// Each batch is processed by a single goroutine
// RESOURCE FIX: Added context support for graceful goroutine termination
func (it *ParallelIterator) ForEachBatchWithContext(ctx context.Context, batchSize int, fn func(int, []any) error) error {
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
		// Check context cancellation and close signal
		select {
		case <-ctx.Done():
			wg.Wait()
			return ctx.Err()
		case <-it.done:
			wg.Wait()
			return nil
		default:
		}

		if atomic.LoadInt32(&hasError) == 1 {
			break
		}

		select {
		case it.sem <- struct{}{}: // Acquire semaphore
		case <-ctx.Done():
			wg.Wait()
			return ctx.Err()
		case <-it.done:
			wg.Wait()
			return nil
		}
		wg.Add(1)

		go func(idx int, b []any) {
			defer wg.Done()
			defer func() { <-it.sem }()

			select {
			case <-ctx.Done():
				return
			case <-it.done:
				return
			default:
			}

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

// Close releases resources associated with the ParallelIterator.
// Signals any running goroutines to stop and waits for them to finish.
func (it *ParallelIterator) Close() {
	select {
	case <-it.done:
		return // Already closed
	default:
		close(it.done)
	}
}

// Release returns the IterableValue to the pool
func (iv *IterableValue) Release() {
	iv.data = nil
	iterableValuePool.Put(iv)
}

// ============================================================================
// Package-level Foreach* wrappers for Processor methods (dual-layer design)
// ============================================================================

// ForeachWithError iterates over JSON arrays or objects with error-returning callback.
// The callback returns an error to signal iteration control:
//   - nil: continue iteration
//   - item.Break(): stop iteration without error
//   - other error: stop iteration and return the error
//
// Example:
//
//	err := json.ForeachWithError(jsonStr, ".", func(key any, item *json.IterableValue) error {
//	    if item.GetInt("id") == targetId {
//	        return item.Break() // stop iteration
//	    }
//	    return nil // continue
//	})
func ForeachWithError(jsonStr, path string, fn func(key any, item *IterableValue) error) error {
	return withProcessorError(func(p *Processor) error {
		return p.ForeachWithError(jsonStr, path, fn)
	})
}

// ForeachNestedWithError recursively iterates over all nested JSON structures with error-returning callback.
//
// Example:
//
//	err := json.ForeachNestedWithError(jsonStr, func(key any, item *json.IterableValue) error {
//	    fmt.Printf("Key: %v\n", key)
//	    return nil
//	})
func ForeachNestedWithError(jsonStr string, fn func(key any, item *IterableValue) error) error {
	return withProcessorError(func(p *Processor) error {
		return p.ForeachNestedWithError(jsonStr, fn)
	})
}

// ForeachWithPathAndIterator iterates over JSON at a path with path information in the callback.
// The callback receives the current path and returns IteratorControl to control iteration flow.
//
// Example:
//
//	err := json.ForeachWithPathAndIterator(jsonStr, ".users", func(key any, item *json.IterableValue, currentPath string) json.IteratorControl {
//	    fmt.Printf("Path: %s, Key: %v\n", currentPath, key)
//	    return json.IteratorNormal // continue
//	})
func ForeachWithPathAndIterator(jsonStr, path string, fn func(key any, item *IterableValue, currentPath string) IteratorControl) error {
	return withProcessorError(func(p *Processor) error {
		return p.ForeachWithPathAndIterator(jsonStr, path, fn)
	})
}
