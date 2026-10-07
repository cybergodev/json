package json

import (
	"errors"
	"fmt"
	"log/slog"
	"reflect"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"

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
	// IteratorNormal continues iteration normally (the default control value).
	IteratorNormal IteratorControl = iota
	// IteratorContinue is a no-op alias of IteratorNormal, kept for API
	// symmetry. Skipping an item is implicit: iteration continues regardless.
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
//
// NOTE (D-002/R8 M5): no production caller — unifiedTypeConversion
// (helpers.go) is the live conversion path; retained for tests/future use.
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
func NewIterator(data any, _ ...Config) *Iterator {
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
// Keys are sorted so iteration order is deterministic: Go randomizes map
// iteration order per iteration, so an unsorted copy made Next() yield
// key/value pairs in a different order on every Iterator construction.
func (it *Iterator) initKeysOnce() {
	it.keysOnce.Do(func() {
		if obj, ok := it.data.(map[string]any); ok {
			it.keys = make([]string, 0, len(obj))
			// Do not intern: these keys are transient (one-shot iteration), and
			// interning would retain them in a process-global cache and take its
			// write lock per key for keys that will never repeat.
			for k := range obj {
				it.keys = append(it.keys, k)
			}
			slices.Sort(it.keys)
		}
	})
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
// NOTE: Not concurrency-safe. Do not call while another goroutine is iterating
// via HasNext()/Next(), or while ResetWith is called concurrently.
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
// NOTE: Not concurrency-safe. Do not call while another goroutine is iterating
// via HasNext()/Next(), or while Reset is called concurrently.
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

// ForeachWithPathAndControl iterates over JSON arrays or objects and applies a function.
// This is the 3-parameter version used by most code.
// Accepts optional Config for consistency with Processor.ForeachWithPathAndControl.
//
// Errors:
//   - ErrProcessorClosed: the default processor has been closed
//   - errors from resolving path (ErrInvalidJSON, ErrPathNotFound, ErrSizeLimit)
func ForeachWithPathAndControl(jsonStr, path string, fn func(key any, value any) IteratorControl, cfg ...Config) error {
	// Delegate to the Processor method so the callback runs on a deep copy of
	// the resolved value, preventing mutation callbacks from corrupting cached
	// parse data (mirror principle: json.ForeachWithPathAndControl ↔ p.ForeachWithPathAndControl).
	return withProcessorError(func(p *Processor) error {
		return p.ForeachWithPathAndControl(jsonStr, path, fn, cfg...)
	})
}

// Foreach iterates over JSON arrays or objects with simplified signature (for test compatibility).
// Accepts optional Config for consistency with Processor.Foreach.
//
// Deprecated: Foreach drops errors — an invalid document or a closed processor
// silently skips the callback entirely. Use ForeachWithError, which returns the
// error (its callback must return nil to continue):
//
//	err := json.ForeachWithError(jsonStr, ".", func(key any, item *json.IterableValue) error {
//	    // ...
//	    return nil
//	})
//
// Foreach will not be removed within v1 (per D-005 the module stays on v1.x).
func Foreach(jsonStr string, fn func(key any, item *IterableValue), cfg ...Config) {
	// Delegate to the Processor method so the callback runs on a deep copy of
	// the resolved value, preventing mutation callbacks from corrupting cached
	// parse data. Void wrapper: processor setup errors are intentionally
	// ignored to match the void signature; the closed-processor check happens
	// inside p.Foreach.
	p, err := getProcessorOrFail()
	if err != nil {
		return
	}
	p.Foreach(jsonStr, fn, cfg...)
}

// foreachWithIterableValue iterates over a value and applies a function with IterableValue
// PERFORMANCE: Uses pooled IterableValue to reduce allocations
func foreachWithIterableValue(data any, fn func(key any, item *IterableValue)) {
	// SAFETY (SEC-003): a panicking user callback must not crash the program. The
	// void signature has no error return, so recover, log, and stop iteration.
	// A pooled IterableValue held during the panic is simply not returned — sync.Pool
	// tolerates that (it is best-effort by design).
	defer func() {
		if r := recover(); r != nil {
			slog.Error("foreach callback panicked", slog.Any("panic", r))
		}
	}()
	switch v := data.(type) {
	case []any:
		for i, item := range v {
			iv := getIterableValue(item)
			fn(i, iv)
			iv.data = nil
			putIterableValue(iv)
		}
	case map[string]any:
		// Deterministic callback order: iterate keys in sorted order rather
		// than randomized map order (see sortedMapKeys).
		for _, key := range sortedMapKeys(v) {
			iv := getIterableValue(v[key])
			fn(key, iv)
			iv.data = nil
			putIterableValue(iv)
		}
	}
}

// ForeachWithPath iterates over JSON arrays or objects with simplified signature (for test compatibility).
// Accepts optional Config for consistency with Processor.ForeachWithPath.
//
// Errors:
//   - ErrProcessorClosed: the default processor has been closed
//   - errors from resolving path (ErrInvalidJSON, ErrPathNotFound, ErrSizeLimit)
func ForeachWithPath(jsonStr, path string, fn func(key any, item *IterableValue), cfg ...Config) error {
	// Delegate to the Processor method so the callback runs on a deep copy of
	// the resolved value, preventing mutation callbacks from corrupting cached
	// parse data (mirror principle: json.ForeachWithPath ↔ p.ForeachWithPath).
	return withProcessorError(func(p *Processor) error {
		return p.ForeachWithPath(jsonStr, path, fn, cfg...)
	})
}

// foreachWithPathIterableValue iterates with IterableValue and path information
// PERFORMANCE: Uses pooled IterableValue to reduce allocations
func foreachWithPathIterableValue(data any, currentPath string, fn func(key any, item *IterableValue, currentPath string) IteratorControl) (err error) {
	// SAFETY (SEC-003): recover a panicking user callback and surface it as an error.
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("foreach callback panicked: %v", r)
		}
	}()
	// PERFORMANCE: reuse one path buffer across all elements instead of
	// allocating a fresh []byte per element. Only the string(buf) conversion
	// (the path handed to the callback) allocates per element now.
	buf := make([]byte, 0, 64)
	switch v := data.(type) {
	case []any:
		for i, item := range v {
			buf = buf[:0]
			buf = append(buf, currentPath...)
			buf = append(buf, '[')
			buf = strconv.AppendInt(buf, int64(i), 10)
			buf = append(buf, ']')
			path := string(buf)
			iv := getIterableValue(item)
			ctrl := fn(i, iv, path)
			iv.data = nil
			putIterableValue(iv)
			if ctrl == IteratorBreak {
				return nil
			}
		}
	case map[string]any:
		// Deterministic callback order: iterate keys in sorted order rather
		// than randomized map order (see sortedMapKeys).
		for _, key := range sortedMapKeys(v) {
			val := v[key]
			buf = buf[:0]
			buf = append(buf, currentPath...)
			buf = append(buf, '.')
			buf = append(buf, key...)
			path := string(buf)
			iv := getIterableValue(val)
			ctrl := fn(key, iv, path)
			iv.data = nil
			putIterableValue(iv)
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
//
// Errors:
//   - ErrProcessorClosed: the default processor has been closed
//   - errors from parsing the root (ErrInvalidJSON, ErrSizeLimit)
func ForeachReturn(jsonStr string, fn func(key any, item *IterableValue), cfg ...Config) (string, error) {
	// Delegate to the Processor method, which deep-copies the resolved value
	// before iteration and re-marshals it afterwards, so callback mutations via
	// IterableValue are reflected in the returned string and cached parse data
	// is not corrupted. Previously this returned the original jsonStr unchanged
	// while iterating on (mutable) cached data — a divergence from
	// Processor.ForeachReturn.
	p, err := getProcessorOrFail()
	if err != nil {
		return "", err
	}
	return p.ForeachReturn(jsonStr, fn, cfg...)
}

// foreachOnValue iterates over a value and applies a function
func foreachOnValue(data any, fn func(key any, value any) IteratorControl) (err error) {
	// SAFETY (SEC-003): recover a panicking user callback and surface it as an error.
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("foreach callback panicked: %v", r)
		}
	}()
	switch v := data.(type) {
	case []any:
		for i, item := range v {
			if ctrl := fn(i, item); ctrl == IteratorBreak {
				return nil
			}
		}
	case map[string]any:
		// Deterministic callback order: sorted keys, not randomized map order.
		for _, key := range sortedMapKeys(v) {
			if ctrl := fn(key, v[key]); ctrl == IteratorBreak {
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
//
// Deprecated: ForeachNested drops errors the same way Foreach does — a failed
// parse or closed processor silently skips the callback. Use
// ForeachNestedWithError, which returns the error (its callback must return
// nil to continue). ForeachNested will not be removed within v1 (per D-005
// the module stays on v1.x).
func ForeachNested(jsonStr string, fn func(key any, item *IterableValue), cfg ...Config) {
	// Delegate to the Processor method so the callback runs on a deep copy of
	// the resolved value, preventing mutation callbacks from corrupting cached
	// parse data (mirror principle: json.ForeachNested ↔ p.ForeachNested).
	p, err := getProcessorOrFail()
	if err != nil {
		return
	}
	p.ForeachNested(jsonStr, fn, cfg...)
}

// foreachNestedMaxDepth limits recursion depth for nested iteration to prevent stack overflow.
const foreachNestedMaxDepth = 200

// foreachNestedOnValue recursively iterates over nested values.
// PERFORMANCE: Uses pooled IterableValue to reduce allocations.
// SECURITY: Depth-limited to prevent stack overflow from deeply nested structures.
func foreachNestedOnValue(data any, fn func(key any, item *IterableValue)) {
	// SAFETY (SEC-003): recover is placed here (the entry), NOT in the recursive
	// *Depth variant, so a panic at any depth unwinds fully out of nested iteration
	// (full stop) rather than resuming sibling subtrees. Void signature → log + stop.
	defer func() {
		if r := recover(); r != nil {
			slog.Error("foreach nested callback panicked", slog.Any("panic", r))
		}
	}()
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
			iv := getIterableValue(item)
			fn(i, iv)
			foreachNestedOnValueDepth(item, fn, depth+1)
			iv.data = nil
			putIterableValue(iv)
		}
	case map[string]any:
		// Deterministic callback order: sorted keys, not randomized map order.
		for _, key := range sortedMapKeys(v) {
			val := v[key]
			iv := getIterableValue(val)
			fn(key, iv)
			foreachNestedOnValueDepth(val, fn, depth+1)
			iv.data = nil
			putIterableValue(iv)
		}
	}
}

// foreachWithIterableValueError iterates with error-returning callback
// PERFORMANCE: Uses pooled IterableValue to reduce allocations
func foreachWithIterableValueError(data any, fn func(key any, item *IterableValue) error) (err error) {
	// SAFETY (SEC-003): recover a panicking user callback and surface it as an error.
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("foreach callback panicked: %v", r)
		}
	}()
	switch v := data.(type) {
	case []any:
		for i, item := range v {
			iv := getIterableValue(item)
			err := fn(i, iv)
			iv.data = nil
			putIterableValue(iv)
			if err != nil {
				if errors.Is(err, errBreak) {
					return nil // Break is not an error
				}
				return err
			}
		}
	case map[string]any:
		// Deterministic callback order: sorted keys, not randomized map order.
		for _, key := range sortedMapKeys(v) {
			iv := getIterableValue(v[key])
			err := fn(key, iv)
			iv.data = nil
			putIterableValue(iv)
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
func foreachNestedOnValueError(data any, fn func(key any, item *IterableValue) error) (err error) {
	// SAFETY (SEC-003): recover is placed here (the entry), NOT in the recursive
	// *Depth variant, so a panic at any depth unwinds fully out of nested iteration.
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("foreach nested callback panicked: %v", r)
		}
	}()
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
			iv := getIterableValue(item)
			err := fn(i, iv)
			if err != nil {
				iv.data = nil
				putIterableValue(iv)
				if errors.Is(err, errBreak) {
					return nil
				}
				return err
			}
			err = foreachNestedOnValueErrorDepth(item, fn, depth+1)
			iv.data = nil
			putIterableValue(iv)
			if err != nil {
				return err
			}
		}
	case map[string]any:
		// Deterministic callback order: sorted keys, not randomized map order.
		for _, key := range sortedMapKeys(v) {
			val := v[key]
			iv := getIterableValue(val)
			err := fn(key, iv)
			if err != nil {
				iv.data = nil
				putIterableValue(iv)
				if errors.Is(err, errBreak) {
					return nil
				}
				return err
			}
			err = foreachNestedOnValueErrorDepth(val, fn, depth+1)
			iv.data = nil
			putIterableValue(iv)
			if err != nil {
				return err
			}
		}
	}
	return nil
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
// Package-level Foreach* wrappers for Processor methods (dual-layer design)
// ============================================================================

// ForeachWithError iterates over JSON arrays or objects with error-returning callback.
// The callback returns an error to signal iteration control:
//   - nil: continue iteration
//   - item.Break(): stop iteration without error
//   - other error: stop iteration and return the error
//
// Accepts an optional trailing Config (cfg) which is forwarded to the underlying
// Get for per-call validation/security limits, consistent with the rest of the
// iterate family and the mirror principle (json.ForeachWithError(..., cfg) ↔
// p.ForeachWithError(..., cfg)).
//
// Example:
//
//	err := json.ForeachWithError(jsonStr, ".", func(key any, item *json.IterableValue) error {
//	    if item.GetInt("id") == targetId {
//	        return item.Break() // stop iteration
//	    }
//	    return nil // continue
//	})
func ForeachWithError(jsonStr, path string, fn func(key any, item *IterableValue) error, cfg ...Config) error {
	return withProcessorError(func(p *Processor) error {
		return p.ForeachWithError(jsonStr, path, fn, cfg...)
	})
}

// ForeachNestedWithError recursively iterates over all nested JSON structures with error-returning callback.
//
// Accepts an optional trailing Config (cfg) forwarded to the underlying Get for
// per-call validation/security limits (json.ForeachNestedWithError(..., cfg) ↔
// p.ForeachNestedWithError(..., cfg)).
//
// Example:
//
//	err := json.ForeachNestedWithError(jsonStr, func(key any, item *json.IterableValue) error {
//	    fmt.Printf("Key: %v\n", key)
//	    return nil
//	})
//
// Errors:
//   - ErrProcessorClosed: the default processor has been closed
//   - any error returned by fn, or ErrInvalidJSON if jsonStr is not valid JSON
func ForeachNestedWithError(jsonStr string, fn func(key any, item *IterableValue) error, cfg ...Config) error {
	return withProcessorError(func(p *Processor) error {
		return p.ForeachNestedWithError(jsonStr, fn, cfg...)
	})
}

// ForeachWithPathAndIterator iterates over JSON at a path with path information in the callback.
// The callback receives the current path and returns IteratorControl to control iteration flow.
//
// Accepts an optional trailing Config (cfg) forwarded to the underlying Get for
// per-call validation/security limits (json.ForeachWithPathAndIterator(..., cfg) ↔
// p.ForeachWithPathAndIterator(..., cfg)).
//
// Example:
//
//	err := json.ForeachWithPathAndIterator(jsonStr, ".users", func(key any, item *json.IterableValue, currentPath string) json.IteratorControl {
//	    fmt.Printf("Path: %s, Key: %v\n", currentPath, key)
//	    return json.IteratorNormal // continue
//	})
//
// Errors:
//   - ErrProcessorClosed: the default processor has been closed
//   - errors from resolving path (ErrInvalidJSON, ErrPathNotFound, ErrSizeLimit)
func ForeachWithPathAndIterator(jsonStr, path string, fn func(key any, item *IterableValue, currentPath string) IteratorControl, cfg ...Config) error {
	return withProcessorError(func(p *Processor) error {
		return p.ForeachWithPathAndIterator(jsonStr, path, fn, cfg...)
	})
}
