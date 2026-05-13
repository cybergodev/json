package internal

import (
	"sync"
	"sync/atomic"
)

// ============================================================================
// BYTE TO STRING CONVERSION OPTIMIZATION
// PERFORMANCE: Reduces allocations when converting []byte to string for interning
// SECURITY: Uses safe conversion for untrusted input
// ============================================================================

// safeStringFromBytes converts []byte to string with allocation
// PERFORMANCE: Allocates a new string, but safer for untrusted input
func safeStringFromBytes(b []byte) string {
	if len(b) == 0 {
		return ""
	}
	return string(b)
}

// ============================================================================
// STRING INTERNING
// Reduces memory allocations for frequently used strings (JSON keys, paths)
// PERFORMANCE: Significant memory reduction for JSON with repeated keys
// SECURITY: Fixed memory exhaustion issues with proactive eviction
// ============================================================================

// maxStringCopyThreshold is the threshold below which string copies are avoided.
// Strings shorter than this threshold are returned directly (Go strings are immutable).
// Longer strings are force-copied via []byte to guarantee independence from any
// underlying buffer the original string may reference (e.g., from pooled buffers).
const maxStringCopyThreshold = 8192

// copyString creates an independent copy of a string.
// Short strings are returned directly (Go strings are immutable).
// Large strings are force-copied via []byte to guarantee independence
// from any underlying buffer the original string may reference.
func copyString(s string) string {
	if len(s) == 0 {
		return ""
	}
	if len(s) <= maxStringCopyThreshold {
		return s // Go strings are immutable; assignment is safe
	}
	return string([]byte(s)) // force copy for very large strings
}

// StringIntern stores interned strings for reuse
type StringIntern struct {
	mu        sync.RWMutex
	strings   map[string]string
	size      int64
	maxSize   int64
	hits      int64
	misses    int64
	evictions int64 // SECURITY: Track eviction count for monitoring
}

// GlobalStringIntern is the default string interner
var GlobalStringIntern = NewStringIntern(10 * 1024 * 1024) // 10MB max

// NewStringIntern creates a new string interner with a maximum size
func NewStringIntern(maxSize int64) *StringIntern {
	return &StringIntern{
		strings: make(map[string]string),
		maxSize: maxSize,
	}
}

// Intern returns an interned version of the string
// If the string is already interned, returns the existing copy
// Otherwise, stores and returns a copy of the string
// SECURITY: Fixed race condition and memory exhaustion with proactive eviction at 80%
// PERFORMANCE: Uses pooled buffers for string copying
func (si *StringIntern) Intern(s string) string {
	if len(s) == 0 {
		return ""
	}

	// Don't intern very long strings
	if len(s) > 256 {
		return s
	}

	// Check if already interned (read lock)
	si.mu.RLock()
	if interned, ok := si.strings[s]; ok {
		si.mu.RUnlock()
		atomic.AddInt64(&si.hits, 1)
		return interned
	}
	si.mu.RUnlock()

	// SECURITY: Hold write lock for entire check-evict-store sequence to prevent race
	si.mu.Lock()
	defer si.mu.Unlock()

	// Double-check after acquiring write lock
	if interned, ok := si.strings[s]; ok {
		atomic.AddInt64(&si.hits, 1)
		return interned
	}

	// SECURITY FIX: Proactive eviction at 80% to prevent sudden memory spikes
	// This gives more headroom before hitting the hard limit
	highWatermark := int64(float64(si.maxSize) * 0.8)
	for si.size+int64(len(s)) > highWatermark {
		if !si.evictRandomLocked() {
			// Can't evict any more, skip interning to prevent memory exhaustion
			atomic.AddInt64(&si.misses, 1)
			return s
		}
		atomic.AddInt64(&si.evictions, 1)
	}

	// PERFORMANCE: Use pooled buffer for string copying
	copied := copyString(s)
	si.strings[copied] = copied
	si.size += int64(len(s))
	atomic.AddInt64(&si.misses, 1)

	return copied
}

// InternBytes returns an interned string from a byte slice
// SECURITY: Uses safe conversion to avoid potential race conditions with pooled buffers
func (si *StringIntern) InternBytes(b []byte) string {
	if len(b) == 0 {
		return ""
	}
	// SECURITY: Use safe conversion for both lookup and storage
	// This avoids potential race conditions when byte slices are reused from pools
	s := safeStringFromBytes(b)
	return si.Intern(s)
}

// evictRandomLocked removes entries when size limit is reached
// Returns true if entries were evicted, false if no entries to evict
// SECURITY: Must be called with lock already held
// NOTE: Go map iteration order is random, so eviction is effectively random
// PERFORMANCE: Evicts 1/4 of entries (not 1/2) to avoid sudden performance
// degradation after eviction. Gradual eviction allows the intern to rebuild
// its working set more smoothly.
func (si *StringIntern) evictRandomLocked() bool {
	if len(si.strings) == 0 {
		return false
	}
	// Remove a quarter of entries for smoother eviction
	count := 0
	target := len(si.strings) / 4
	target = max(target, 1)
	for k := range si.strings {
		if count >= target {
			break
		}
		si.size -= int64(len(k))
		delete(si.strings, k)
		count++
	}
	return count > 0
}

// Stats returns statistics about the string intern
type InternStats struct {
	Entries   int
	Size      int64
	Hits      int64
	Misses    int64
	Evictions int64
}

// GetStats returns current statistics including eviction count
func (si *StringIntern) GetStats() InternStats {
	si.mu.RLock()
	defer si.mu.RUnlock()

	return InternStats{
		Entries:   len(si.strings),
		Size:      si.size,
		Hits:      atomic.LoadInt64(&si.hits),
		Misses:    atomic.LoadInt64(&si.misses),
		Evictions: atomic.LoadInt64(&si.evictions),
	}
}

// Clear removes all interned strings
func (si *StringIntern) Clear() {
	si.mu.Lock()
	defer si.mu.Unlock()

	si.strings = make(map[string]string)
	si.size = 0
}

// ============================================================================
// KEY INTERN - Specialized for JSON keys
// PERFORMANCE: Increased shards to 64, added hot key cache with sync.Map
// SECURITY FIX: Added memory-based eviction and size limit for hot key cache
// ============================================================================

// maxShardSize is the maximum size per shard before eviction (256KB)
const maxShardSize = 256 * 1024

// maxHotKeys is the maximum number of hot keys to cache
// SECURITY: Prevents unbounded memory growth from hot key cache
const maxHotKeys = 10000

// KeyIntern is a specialized interner for JSON keys
// Uses sharding for better concurrent performance with hot key cache
type KeyIntern struct {
	shards      []*keyInternShard
	shardMask   uint64
	hotKeys     sync.Map // Lock-free cache for frequently accessed keys
	hotKeyCount int64    // Atomic counter for hot key cache size
	trimming    int32    // Atomic flag: 1 = trim in progress, prevents concurrent trimHotCache
}

type keyInternShard struct {
	mu        sync.RWMutex
	strings   map[string]string
	size      int64 // Track memory usage for eviction
	evictions int64 // SECURITY: Track eviction count for monitoring
}

// GlobalKeyIntern is the global key interner
var GlobalKeyIntern = NewKeyIntern()

// NewKeyIntern creates a new sharded key interner with 64 shards
func NewKeyIntern() *KeyIntern {
	const shardCount = 64 // Increased from 16 for better concurrency
	shards := make([]*keyInternShard, shardCount)
	for i := range shards {
		shards[i] = &keyInternShard{
			strings: make(map[string]string, 256),
		}
	}
	return &KeyIntern{
		shards:    shards,
		shardMask: uint64(shardCount - 1),
	}
}

// Intern returns an interned version of the key
// PERFORMANCE: First checks hot key cache (lock-free), then falls back to sharded lookup
// SECURITY FIX: Added memory-based eviction and hot key cache size limit
func (ki *KeyIntern) Intern(key string) string {
	if len(key) == 0 {
		return ""
	}

	// Don't intern very long keys
	if len(key) > 128 {
		return key
	}

	// PERFORMANCE: Check hot key cache first (lock-free read)
	if interned, ok := ki.hotKeys.Load(key); ok {
		return interned.(string)
	}

	shard := ki.getShard(key)

	// Check read lock first
	shard.mu.RLock()
	if interned, ok := shard.strings[key]; ok {
		shard.mu.RUnlock()
		// Promote to hot key cache with size limit
		ki.promoteToHotCache(key, interned)
		return interned
	}
	shard.mu.RUnlock()

	// Write lock
	shard.mu.Lock()
	defer shard.mu.Unlock()

	// Double-check
	if interned, ok := shard.strings[key]; ok {
		// Promote to hot key cache with size limit
		ki.promoteToHotCache(key, interned)
		return interned
	}

	// SECURITY FIX: Check if we need to evict before adding
	// Use 80% watermark for proactive eviction (256KB * 80% = 204.8KB)
	const highWatermark = (maxShardSize * 80) / 100
	if shard.size+int64(len(key)) > highWatermark {
		ki.evictShardLocked(shard)
	}

	// SECURITY FIX: Use safe string copy instead of unsafe
	copied := string([]byte(key))
	shard.strings[copied] = copied
	shard.size += int64(len(copied))

	// Promote to hot key cache with size limit
	ki.promoteToHotCache(key, copied)
	return copied
}

// promoteToHotCache adds a key to the hot cache with size limiting
// SECURITY: Prevents unbounded growth of the hot key cache
func (ki *KeyIntern) promoteToHotCache(key, interned string) {
	// Use LoadOrStore for atomic check-and-set to prevent counter overcounting
	if _, loaded := ki.hotKeys.LoadOrStore(key, interned); loaded {
		return
	}

	count := atomic.AddInt64(&ki.hotKeyCount, 1)
	if count > maxHotKeys {
		// Trim cache: delete ~25% of entries
		ki.trimHotCache()
	}
}

// trimHotCache removes approximately 25% of hot cache entries
// SECURITY: Prevents memory exhaustion from unbounded hot key cache
// Uses CAS flag to prevent multiple goroutines from trimming simultaneously,
// which could cause hotKeyCount to go negative.
func (ki *KeyIntern) trimHotCache() {
	// CAS guard: only one goroutine trims at a time
	if !atomic.CompareAndSwapInt32(&ki.trimming, 0, 1) {
		return
	}
	defer atomic.StoreInt32(&ki.trimming, 0)

	toDelete := maxHotKeys / 4
	deleted := int64(0)

	ki.hotKeys.Range(func(key, value any) bool {
		if deleted >= int64(toDelete) {
			return false
		}
		ki.hotKeys.Delete(key)
		deleted++
		return true
	})

	// Update counter, clamped to prevent negative values
	newCount := atomic.AddInt64(&ki.hotKeyCount, -deleted)
	if newCount < 0 {
		atomic.StoreInt64(&ki.hotKeyCount, 0)
	}
}

// evictShardLocked removes entries when shard size limit is reached
// SECURITY: Must be called with shard lock already held
// PERFORMANCE: Collects keys first to avoid multiple sync.Map operations during iteration
func (ki *KeyIntern) evictShardLocked(shard *keyInternShard) bool {
	if len(shard.strings) == 0 {
		return false
	}

	// Remove half the entries (aggressive eviction to prevent memory bloat)
	target := len(shard.strings) / 2
	target = max(target, 1)

	// SECURITY FIX: Collect keys to delete first
	keysToDelete := make([]string, 0, target)
	count := 0
	for k := range shard.strings {
		if count >= target {
			break
		}
		keysToDelete = append(keysToDelete, k)
		count++
	}

	// SECURITY FIX: Delete from hot key cache FIRST, then from shard
	// This prevents a race where a lookup finds the key in hot cache
	// but the shard no longer has it, causing inconsistent behavior.
	// The correct order is:
	// 1. Remove from hot cache (fast path lookup will fail)
	// 2. Remove from shard (slow path lookup will also fail)
	// sync.Map.Delete is safe for concurrent use
	for _, k := range keysToDelete {
		ki.hotKeys.Delete(k)
	}

	// Now delete from shard
	for _, k := range keysToDelete {
		shard.size -= int64(len(k))
		delete(shard.strings, k)
	}

	shard.evictions += int64(count)
	return count > 0
}

// InternBytes returns an interned string from a byte slice
// SECURITY: Uses safe conversion to avoid potential race conditions with pooled buffers
func (ki *KeyIntern) InternBytes(b []byte) string {
	if len(b) == 0 {
		return ""
	}
	// SECURITY: Use safe conversion for both lookup and storage
	// This avoids potential race conditions when byte slices are reused from pools
	s := safeStringFromBytes(b)
	return ki.Intern(s)
}

// getShard returns the shard for a key using FNV-1a hash
func (ki *KeyIntern) getShard(key string) *keyInternShard {
	h := HashStringFNV1a(key)
	return ki.shards[h&ki.shardMask]
}

// Clear removes all interned keys
func (ki *KeyIntern) Clear() {
	for _, shard := range ki.shards {
		shard.mu.Lock()
		shard.strings = make(map[string]string, 256)
		shard.size = 0
		shard.mu.Unlock()
	}
	// Clear hot key cache
	ki.hotKeys = sync.Map{}
	atomic.StoreInt64(&ki.hotKeyCount, 0)
}

// Size returns the total number of interned keys across all shards
func (ki *KeyIntern) Size() int {
	var total int
	for _, shard := range ki.shards {
		shard.mu.RLock()
		total += len(shard.strings)
		shard.mu.RUnlock()
	}
	return total
}

// Stats returns statistics about the key interner
type KeyInternStats struct {
	ShardCount  int
	HotKeyCount int64
}

// GetStats returns current statistics
func (ki *KeyIntern) GetStats() KeyInternStats {
	// Count hot keys (approximate)
	var hotKeyCount int64
	ki.hotKeys.Range(func(_, _ any) bool {
		hotKeyCount++
		return true
	})

	return KeyInternStats{
		ShardCount:  len(ki.shards),
		HotKeyCount: hotKeyCount,
	}
}

// ============================================================================
// PATH INTERN - Specialized for JSON paths
// ============================================================================

// PathIntern caches parsed path segments with their string representations
// SECURITY FIX: Added memory-based eviction to prevent unbounded growth
type PathIntern struct {
	mu        sync.RWMutex
	paths     map[string][]PathSegment
	size      int64 // Track memory usage for eviction
	maxSize   int   // Max number of entries
	maxMemory int64 // Max memory in bytes (10MB default)
}

// GlobalPathIntern is the global path interner.
// Deprecated: use internal.ParsePath caching instead. Kept for existing test usage.
// Lazy-initialized with zero initial capacity to avoid wasting memory when unused.
var GlobalPathIntern = NewPathIntern(0)

// NewPathIntern creates a new path interner
func NewPathIntern(maxSize int) *PathIntern {
	initialCap := maxSize / 2
	if initialCap < 0 {
		initialCap = 0
	}
	return &PathIntern{
		paths:     make(map[string][]PathSegment, initialCap),
		maxSize:   maxSize,
		maxMemory: 10 * 1024 * 1024, // 10MB default memory limit
	}
}

// Get retrieves cached path segments
func (pi *PathIntern) Get(path string) ([]PathSegment, bool) {
	pi.mu.RLock()
	segments, ok := pi.paths[path]
	pi.mu.RUnlock()
	return segments, ok
}

// Set stores path segments in cache
// SECURITY FIX: Added memory-based eviction at 80% watermark
func (pi *PathIntern) Set(path string, segments []PathSegment) {
	if len(path) > 256 {
		return // Don't cache very long paths
	}

	// Estimate entry size: path string + segments
	entrySize := int64(len(path)) + int64(len(segments))*int64(pathSegmentSize)

	pi.mu.Lock()
	defer pi.mu.Unlock()

	// SECURITY: Check memory-based eviction at 80% watermark
	highWatermark := int64(float64(pi.maxMemory) * 0.8)
	for pi.size+entrySize > highWatermark && len(pi.paths) > 0 {
		if !pi.evictOneLocked() {
			break // Can't evict anymore
		}
	}

	// Also check entry count limit
	if len(pi.paths) >= pi.maxSize {
		pi.evictOneLocked()
	}

	// Check if we have room
	if pi.size+entrySize > pi.maxMemory {
		return // Skip caching to prevent memory exhaustion
	}

	// Make a copy of segments
	copied := make([]PathSegment, len(segments))
	copy(copied, segments)

	// Update size tracking (remove old entry if exists)
	if old, exists := pi.paths[path]; exists {
		pi.size -= int64(len(path)) + int64(len(old))*int64(pathSegmentSize)
	}

	pi.paths[path] = copied
	pi.size += entrySize
}

// pathSegmentSize is the estimated size of a PathSegment struct in bytes
const pathSegmentSize = 128

// evictOneLocked removes one entry from the cache (must be called with lock held)
func (pi *PathIntern) evictOneLocked() bool {
	if len(pi.paths) == 0 {
		return false
	}
	// Remove a random entry
	for k, v := range pi.paths {
		pi.size -= int64(len(k)) + int64(len(v))*int64(pathSegmentSize)
		delete(pi.paths, k)
		return true
	}
	return false
}

// Clear removes all cached paths
func (pi *PathIntern) Clear() {
	pi.mu.Lock()
	defer pi.mu.Unlock()
	pi.paths = make(map[string][]PathSegment, pi.maxSize/2)
	pi.size = 0
}

// ============================================================================
// HELPER FUNCTIONS
// ============================================================================

// InternKey interns a JSON key using the global key interner
func InternKey(key string) string {
	return GlobalKeyIntern.Intern(key)
}

// InternKeyBytes interns a JSON key from bytes
func InternKeyBytes(b []byte) string {
	return GlobalKeyIntern.InternBytes(b)
}

// InternString interns a string using the global string interner
func InternString(s string) string {
	return GlobalStringIntern.Intern(s)
}

// InternStringBytes interns a string from bytes
func InternStringBytes(b []byte) string {
	return GlobalStringIntern.InternBytes(b)
}

// ============================================================================
// BATCH INTERN - For processing multiple keys efficiently
// ============================================================================

// BatchIntern interns multiple strings at once
// More efficient than calling Intern multiple times due to reduced lock overhead
// SECURITY FIX: Added memory-based eviction to prevent unbounded growth
func BatchIntern(strings []string) []string {
	if len(strings) == 0 {
		return strings
	}

	result := make([]string, len(strings))
	intern := GlobalStringIntern
	intern.mu.Lock()
	defer intern.mu.Unlock()

	// SECURITY: Pre-check memory watermark (80%)
	highWatermark := int64(float64(intern.maxSize) * 0.8)

	for i, s := range strings {
		if len(s) == 0 || len(s) > 256 {
			result[i] = s
			continue
		}

		if interned, ok := intern.strings[s]; ok {
			result[i] = interned
			continue
		}

		// SECURITY: Check memory before adding
		for intern.size+int64(len(s)) > highWatermark {
			if !intern.evictRandomLocked() {
				// Can't evict anymore, skip interning this string
				result[i] = s
				continue
			}
		}

		// SECURITY FIX: Use safe string copy instead of unsafe
		copied := string([]byte(s))
		intern.strings[copied] = copied
		intern.size += int64(len(s))
		result[i] = copied
	}

	return result
}

// BatchInternKeys interns multiple keys at once using the key interner
func BatchInternKeys(keys []string) []string {
	if len(keys) == 0 {
		return keys
	}

	result := make([]string, len(keys))
	for i, key := range keys {
		result[i] = GlobalKeyIntern.Intern(key)
	}
	return result
}
