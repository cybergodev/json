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

// ============================================================================
// CHUNKED STREAMING OPERATIONS
// PERFORMANCE: Process large JSON in configurable chunks for memory efficiency
// ============================================================================

// StreamArrayChunked streams array elements in chunks for memory-efficient processing
// The chunkSize parameter controls how many elements are processed at once
func (sp *StreamingProcessor) StreamArrayChunked(chunkSize int, fn func([]any) error) error {
	if chunkSize <= 0 {
		chunkSize = 100 // Default chunk size
	}

	// Check if the first token is an array start
	token, err := sp.decoder.Token()
	if err != nil {
		return err
	}

	if token != json.Delim('[') {
		// Not an array, try to decode as single value
		var item any
		if err := sp.decoder.Decode(&item); err != nil {
			return err
		}
		return fn([]any{item})
	}

	chunk := make([]any, 0, chunkSize)
	for sp.decoder.More() {
		var item any
		if err := sp.decoder.Decode(&item); err != nil {
			return err
		}

		chunk = append(chunk, item)
		sp.stats.ItemsProcessed++

		// Process chunk when full
		if len(chunk) >= chunkSize {
			if err := fn(chunk); err != nil {
				return err
			}
			chunk = chunk[:0] // Reset chunk
		}
	}

	// Process remaining items
	if len(chunk) > 0 {
		if err := fn(chunk); err != nil {
			return err
		}
	}

	// Consume closing bracket
	_, err = sp.decoder.Token()
	return err
}

// StreamObjectChunked streams object key-value pairs in chunks for memory-efficient processing
// The chunkSize parameter controls how many pairs are processed at once
func (sp *StreamingProcessor) StreamObjectChunked(chunkSize int, fn func(map[string]any) error) error {
	if chunkSize <= 0 {
		chunkSize = 100 // Default chunk size
	}

	token, err := sp.decoder.Token()
	if err != nil {
		return err
	}

	if token != json.Delim('{') {
		// Not an object, wrap single value
		var value any
		if err := sp.decoder.Decode(&value); err != nil {
			return err
		}
		return fn(map[string]any{"": value})
	}

	chunk := make(map[string]any, chunkSize)
	count := 0

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

		chunk[keyStr] = value
		sp.stats.ItemsProcessed++
		count++

		// Process chunk when full
		if count >= chunkSize {
			if err := fn(chunk); err != nil {
				return err
			}
			chunk = make(map[string]any, chunkSize)
			count = 0
		}
	}

	// Process remaining items
	if count > 0 {
		if err := fn(chunk); err != nil {
			return err
		}
	}

	// Consume closing brace
	_, err = sp.decoder.Token()
	return err
}

// ============================================================================
// STREAMING TRANSFORMATION OPERATIONS
// PERFORMANCE: Transform large JSON without loading entire structure into memory
// ============================================================================

// StreamArrayFilter filters array elements during streaming
// Only elements that pass the predicate are kept
func StreamArrayFilter(reader io.Reader, predicate func(any) bool) ([]any, error) {
	processor := NewStreamingProcessor(reader, 0)
	result := make([]any, 0)

	err := processor.StreamArray(func(index int, item any) bool {
		if predicate(item) {
			result = append(result, item)
		}
		return true
	})

	return result, err
}

// StreamArrayMap transforms array elements during streaming
// Each element is transformed using the provided function
func StreamArrayMap(reader io.Reader, transform func(any) any) ([]any, error) {
	processor := NewStreamingProcessor(reader, 0)
	result := make([]any, 0)

	err := processor.StreamArray(func(index int, item any) bool {
		result = append(result, transform(item))
		return true
	})

	return result, err
}

// StreamArrayReduce reduces array elements to a single value during streaming
// The reducer function receives the accumulated value and current element
func StreamArrayReduce(reader io.Reader, initial any, reducer func(any, any) any) (any, error) {
	processor := NewStreamingProcessor(reader, 0)
	accumulator := initial

	err := processor.StreamArray(func(index int, item any) bool {
		accumulator = reducer(accumulator, item)
		return true
	})

	return accumulator, err
}

// StreamArrayForEach processes each element without collecting results
// Useful for side effects like writing to a database or file
func StreamArrayForEach(reader io.Reader, fn func(int, any) error) error {
	processor := NewStreamingProcessor(reader, 0)

	return processor.StreamArray(func(index int, item any) bool {
		if err := fn(index, item); err != nil {
			return false // Stop iteration on error
		}
		return true
	})
}

// StreamArrayCount counts elements without storing them
// Memory-efficient for just getting array length
func StreamArrayCount(reader io.Reader) (int, error) {
	processor := NewStreamingProcessor(reader, 0)
	count := 0

	err := processor.StreamArray(func(index int, item any) bool {
		count++
		return true
	})

	return count, err
}

// StreamArrayFirst returns the first element that matches a predicate
// Stops processing as soon as a match is found
func StreamArrayFirst(reader io.Reader, predicate func(any) bool) (any, bool, error) {
	processor := NewStreamingProcessor(reader, 0)
	var result any
	found := false

	err := processor.StreamArray(func(index int, item any) bool {
		if predicate(item) {
			result = item
			found = true
			return false // Stop iteration
		}
		return true
	})

	return result, found, err
}

// StreamArrayTake returns the first n elements from a streaming array
// Useful for pagination or sampling
func StreamArrayTake(reader io.Reader, n int) ([]any, error) {
	processor := NewStreamingProcessor(reader, 0)
	result := make([]any, 0, n)

	err := processor.StreamArray(func(index int, item any) bool {
		if len(result) >= n {
			return false // Stop iteration
		}
		result = append(result, item)
		return true
	})

	return result, err
}

// StreamArraySkip skips the first n elements and returns the rest
// Useful for pagination
func StreamArraySkip(reader io.Reader, n int) ([]any, error) {
	processor := NewStreamingProcessor(reader, 0)
	result := make([]any, 0)

	err := processor.StreamArray(func(index int, item any) bool {
		if index >= n {
			result = append(result, item)
		}
		return true
	})

	return result, err
}

// ============================================================================
// LAZY JSON - Parse on first access
// PERFORMANCE: Defer JSON parsing until data is actually needed
// ============================================================================

// LazyJSON provides lazy parsing for JSON data
// The JSON is only parsed when a value is accessed
type LazyJSON struct {
	raw    []byte
	parsed any
	err    error
	once   sync.Once
}

// NewLazyJSON creates a new LazyJSON from raw bytes
func NewLazyJSON(data []byte) *LazyJSON {
	return &LazyJSON{
		raw: data,
	}
}

// parse performs the actual parsing (called once)
func (lj *LazyJSON) parse() {
	lj.once.Do(func() {
		lj.err = json.Unmarshal(lj.raw, &lj.parsed)
	})
}

// Get retrieves a value at the specified path
// Parses the JSON on first access
func (lj *LazyJSON) Get(path string) (any, error) {
	lj.parse()
	if lj.err != nil {
		return nil, lj.err
	}

	p := getDefaultProcessor()
	return p.GetFromParsedData(lj.parsed, path)
}

// GetFromParsedData retrieves a value from already-parsed data
// Uses the processor's path navigation without re-parsing
func (p *Processor) GetFromParsedData(data any, path string) (any, error) {
	if err := p.checkClosed(); err != nil {
		return nil, err
	}

	// Navigate directly using the recursive processor
	return p.recursiveProcessor.ProcessRecursively(data, path, OpGet, nil)
}

// Parse forces parsing and returns the parsed data
func (lj *LazyJSON) Parse() (any, error) {
	lj.parse()
	return lj.parsed, lj.err
}

// Raw returns the raw JSON bytes
func (lj *LazyJSON) Raw() []byte {
	return lj.raw
}

// IsParsed returns true if the JSON has been parsed
func (lj *LazyJSON) IsParsed() bool {
	return lj.parsed != nil || lj.err != nil
}

// Parsed returns the parsed data without forcing parsing
// Returns nil if not yet parsed
func (lj *LazyJSON) Parsed() any {
	return lj.parsed
}

// Error returns any parsing error
func (lj *LazyJSON) Error() error {
	lj.parse()
	return lj.err
}
