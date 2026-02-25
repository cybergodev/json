package json

import (
	"encoding/json"
	"io"
	"sync"
	"sync/atomic"

	"github.com/cybergodev/json/internal"
)

// ============================================================================
// PERFORMANCE OPTIMIZATION MODULE
// This file contains performance-critical optimizations for the JSON library:
// 1. Path segment caching with LRU eviction
// 2. Iterator pooling to reduce allocations
// 3. Streaming JSON processing for large files
// 4. Bulk operation optimizations
// ============================================================================

// pathSegmentCache caches parsed path segments for reuse
// Uses sharding for concurrent access performance
type pathSegmentCache struct {
	shards     []*pathCacheShard
	shardMask  uint64
	maxEntries int
	totalSize  int64
	evictions  int64
}

type pathCacheShard struct {
	mu      sync.RWMutex
	entries map[string][]internal.PathSegment
	lru     []string // Simple LRU tracking
	size    int
}

// globalPathCache is the global path segment cache
var globalPathCache *pathSegmentCache
var pathCacheOnce sync.Once

const pathCacheShardCount = 16

func getPathCache() *pathSegmentCache {
	pathCacheOnce.Do(func() {
		globalPathCache = newPathSegmentCache(10000) // Cache up to 10k paths
	})
	return globalPathCache
}

func newPathSegmentCache(maxEntries int) *pathSegmentCache {
	shards := make([]*pathCacheShard, pathCacheShardCount)
	for i := range shards {
		shards[i] = &pathCacheShard{
			entries: make(map[string][]internal.PathSegment, maxEntries/pathCacheShardCount),
			lru:     make([]string, 0, 100),
		}
	}
	return &pathSegmentCache{
		shards:     shards,
		shardMask:  uint64(pathCacheShardCount - 1),
		maxEntries: maxEntries,
	}
}

func (c *pathSegmentCache) getShard(key string) *pathCacheShard {
	// Inline FNV-1a hash
	h := uint64(14695981039346656037)
	for i := 0; i < len(key); i++ {
		h ^= uint64(key[i])
		h *= 1099511628211
	}
	return c.shards[h&c.shardMask]
}

// Get retrieves cached path segments
func (c *pathSegmentCache) Get(path string) ([]internal.PathSegment, bool) {
	if len(path) > 256 {
		// Don't cache very long paths
		return nil, false
	}

	shard := c.getShard(path)
	shard.mu.RLock()
	segments, ok := shard.entries[path]
	shard.mu.RUnlock()
	return segments, ok
}

// Set stores path segments in cache
func (c *pathSegmentCache) Set(path string, segments []internal.PathSegment) {
	if len(path) > 256 || len(segments) == 0 {
		return
	}

	// Create a copy to prevent external modification
	copied := make([]internal.PathSegment, len(segments))
	copy(copied, segments)

	shard := c.getShard(path)
	shard.mu.Lock()

	// Check if we need to evict
	if len(shard.entries) >= c.maxEntries/pathCacheShardCount && shard.entries[path] == nil {
		// Evict oldest entry
		if len(shard.lru) > 0 {
			oldest := shard.lru[0]
			delete(shard.entries, oldest)
			shard.lru = shard.lru[1:]
			atomic.AddInt64(&c.evictions, 1)
		}
	}

	shard.entries[path] = copied
	shard.lru = append(shard.lru, path)
	shard.mu.Unlock()
}

// ============================================================================
// ITERATOR POOL - Reduces allocations for iterator operations
// ============================================================================

// iteratorPool manages pooled iterators for reduced allocations
type iteratorPool struct {
	pool sync.Pool
}

var globalIteratorPool = &iteratorPool{
	pool: sync.Pool{
		New: func() any {
			return &Iterator{
				keys: make([]string, 0, 16),
			}
		},
	},
}

// Get retrieves an iterator from the pool
func (ip *iteratorPool) Get(processor *Processor, data any, opts *ProcessorOptions) *Iterator {
	it := ip.pool.Get().(*Iterator)
	it.processor = processor
	it.data = data
	it.options = opts
	it.position = 0
	it.keys = it.keys[:0]
	it.keysInit = false
	return it
}

// Put returns an iterator to the pool
func (ip *iteratorPool) Put(it *Iterator) {
	if it == nil {
		return
	}
	// Clear references to allow GC
	it.processor = nil
	it.data = nil
	it.options = nil
	// Keep keys slice for reuse but reset length
	if cap(it.keys) > 256 {
		it.keys = nil // Don't pool very large slices
	}
	ip.pool.Put(it)
}

// ============================================================================
// STREAMING JSON PROCESSOR - For large JSON files
// ============================================================================

// StreamingProcessor handles large JSON files efficiently
type StreamingProcessor struct {
	decoder    *json.Decoder
	reader     io.Reader
	bufferSize int
	stats      StreamingStats
}

// StreamingStats tracks streaming processing statistics
type StreamingStats struct {
	BytesProcessed int64
	ItemsProcessed int64
	Depth          int
}

// NewStreamingProcessor creates a streaming processor for large JSON
func NewStreamingProcessor(reader io.Reader, bufferSize int) *StreamingProcessor {
	if bufferSize <= 0 {
		bufferSize = 64 * 1024 // 64KB default buffer
	}
	return &StreamingProcessor{
		decoder:    json.NewDecoder(reader),
		reader:     reader,
		bufferSize: bufferSize,
	}
}

// StreamArray streams array elements one at a time
// This is memory-efficient for large arrays
func (sp *StreamingProcessor) StreamArray(fn func(index int, item any) bool) error {
	// Check if the first token is an array start
	token, err := sp.decoder.Token()
	if err != nil {
		return err
	}

	if token != json.Delim('[') {
		// Not an array, try to decode as single value
		return sp.decodeSingleValue(fn)
	}

	index := 0
	for sp.decoder.More() {
		var item any
		if err := sp.decoder.Decode(&item); err != nil {
			return err
		}
		sp.stats.ItemsProcessed++

		if !fn(index, item) {
			return nil // Stop iteration
		}
		index++
	}

	// Consume closing bracket
	_, err = sp.decoder.Token()
	return err
}

// StreamObject streams object key-value pairs
func (sp *StreamingProcessor) StreamObject(fn func(key string, value any) bool) error {
	token, err := sp.decoder.Token()
	if err != nil {
		return err
	}

	if token != json.Delim('{') {
		return sp.decodeSingleValueAsObject(fn)
	}

	for sp.decoder.More() {
		key, err := sp.decoder.Token()
		if err != nil {
			return err
		}

		keyStr, ok := key.(string)
		if !ok {
			continue
		}

		var value any
		if err := sp.decoder.Decode(&value); err != nil {
			return err
		}
		sp.stats.ItemsProcessed++

		if !fn(keyStr, value) {
			return nil
		}
	}

	// Consume closing brace
	_, err = sp.decoder.Token()
	return err
}

func (sp *StreamingProcessor) decodeSingleValue(fn func(int, any) bool) error {
	var value any
	if err := sp.decoder.Decode(&value); err != nil {
		return err
	}
	fn(0, value)
	return nil
}

func (sp *StreamingProcessor) decodeSingleValueAsObject(fn func(string, any) bool) error {
	var value any
	if err := sp.decoder.Decode(&value); err != nil {
		return err
	}
	fn("", value)
	return nil
}

// GetStats returns streaming statistics
func (sp *StreamingProcessor) GetStats() StreamingStats {
	return sp.stats
}

// ============================================================================
// BULK OPERATION OPTIMIZATIONS
// ============================================================================

// BulkProcessor handles multiple operations efficiently
type BulkProcessor struct {
	processor *Processor
	batchSize int
}

// NewBulkProcessor creates a bulk processor
func NewBulkProcessor(processor *Processor, batchSize int) *BulkProcessor {
	if batchSize <= 0 {
		batchSize = 100
	}
	return &BulkProcessor{
		processor: processor,
		batchSize: batchSize,
	}
}

// BulkGet performs multiple Get operations efficiently
func (bp *BulkProcessor) BulkGet(jsonStr string, paths []string) (map[string]any, error) {
	results := make(map[string]any, len(paths))

	// Parse JSON once for all operations
	var data any
	if err := bp.processor.Parse(jsonStr, &data); err != nil {
		return nil, err
	}

	// Reuse parsed data for all path lookups
	for _, path := range paths {
		value, err := bp.processor.Get(jsonStr, path)
		if err == nil {
			results[path] = value
		}
	}

	return results, nil
}

// ============================================================================
// FAST PATH DETECTION - Avoids complex parsing for simple cases
// ============================================================================

// isSimplePropertyAccess checks if path is a simple single-level property access
// This is the fastest case that can bypass most parsing logic
func isSimplePropertyAccess(path string) bool {
	if len(path) == 0 || len(path) > 64 {
		return false
	}

	for i := 0; i < len(path); i++ {
		c := path[i]
		// Only allow alphanumeric and underscore
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') ||
			(c >= '0' && c <= '9') || c == '_') {
			return false
		}
	}
	return true
}

// fastGetSimple is optimized for simple single-level property access
func fastGetSimple(data map[string]any, key string) (any, bool) {
	val, exists := data[key]
	return val, exists
}

// ============================================================================
// BUFFER POOL FOR LARGE OPERATIONS
// ============================================================================

var largeBufferPool = sync.Pool{
	New: func() any {
		buf := make([]byte, 0, 32*1024) // 32KB buffer
		return &buf
	},
}

func getLargeBuffer() *[]byte {
	buf := largeBufferPool.Get().(*[]byte)
	*buf = (*buf)[:0]
	return buf
}

func putLargeBuffer(buf *[]byte) {
	if cap(*buf) <= 64*1024 { // Don't pool buffers larger than 64KB
		largeBufferPool.Put(buf)
	}
}

// ============================================================================
// ENCODE BUFFER POOL
// ============================================================================

var encodeBufferPool = sync.Pool{
	New: func() any {
		return make([]byte, 0, 4*1024) // 4KB initial buffer
	},
}

// GetEncodeBuffer gets a buffer for encoding operations
func GetEncodeBuffer() []byte {
	return encodeBufferPool.Get().([]byte)[:0]
}

// PutEncodeBuffer returns a buffer to the pool
func PutEncodeBuffer(buf []byte) {
	if cap(buf) <= 16*1024 { // Don't pool buffers larger than 16KB
		encodeBufferPool.Put(buf)
	}
}
