package internal

import (
	"sync"
	"sync/atomic"
)

// ============================================================================
// STRING INTERNING
// Reduces memory allocations for frequently used strings (JSON keys, paths)
// PERFORMANCE: Significant memory reduction for JSON with repeated keys
// SECURITY: Fixed memory exhaustion issues with proactive eviction
//
// STATUS (D-002/R8 M5): this module is currently NOT wired into any production
// path — no parser or encoder calls Intern/InternBytes (iterator.go
// deliberately skips interning for its transient keys, see initKeysOnce). It
// is retained, with its tests, for potential future use; treat it as
// test-covered but dormant until a caller is added.
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

// InternBytes returns an interned string from a byte slice.
//
// SECURITY: The returned string never aliases the caller's byte buffer — on a
// hit we return the canonical interned copy, on a miss we convert once to an
// owned string and intern that.
//
// PERFORMANCE: The cache-hit path is allocation-free: si.strings[string(b)] is
// a map index over a byte-slice-to-string conversion, which the Go compiler
// optimizes to not materialize the temporary string. Only a genuine miss pays
// the single string(b) conversion that Intern needs for storage.
func (si *StringIntern) InternBytes(b []byte) string {
	if len(b) == 0 {
		return ""
	}
	// Match Intern's length guard: long strings are returned as-is, never stored.
	if len(b) > 256 {
		return string(b)
	}

	si.mu.RLock()
	// Zero-allocation lookup: string(b) in a map index does not allocate.
	if interned, ok := si.strings[string(b)]; ok {
		si.mu.RUnlock()
		atomic.AddInt64(&si.hits, 1)
		return interned
	}
	si.mu.RUnlock()

	// Cache miss: convert to an owned string once and run the full Intern path.
	return si.Intern(string(b))
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

// InternStats holds statistics about the string interner.
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

	ki.hotKeys.Range(func(key, _ any) bool {
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

// InternBytes returns an interned string from a byte slice.
//
// SECURITY: The returned string NEVER references the caller's byte buffer.
// On a cache hit we return the canonical interned copy; on a miss we convert
// the bytes to an owned string once and intern that copy. This preserves the
// original safe-conversion guarantee (no aliasing of pooled buffers) while
// removing the per-call string(b) allocation on the hot cache-hit path.
//
// PERFORMANCE: The cache-hit path is allocation-free. Go's compiler treats a
// map index of the form m[string(byteSlice)] specially and avoids materializing
// the temporary string, so shard.strings[string(b)] costs zero allocations. The
// shard is selected by hashing the bytes directly (HashBytesFNV1a), again with
// no conversion. Only a genuine cache miss pays the single string(b) conversion
// that Intern requires for storage.
func (ki *KeyIntern) InternBytes(b []byte) string {
	if len(b) == 0 {
		return ""
	}
	// Match Intern's length guard: long keys are returned as-is, never stored.
	if len(b) > 128 {
		return string(b)
	}

	shard := ki.getShardFromBytes(b)
	shard.mu.RLock()
	// Zero-allocation lookup: the string(b) conversion in a map index expression
	// does not allocate (compiler optimization), so cache hits cost nothing here.
	if interned, ok := shard.strings[string(b)]; ok {
		shard.mu.RUnlock()
		return interned
	}
	shard.mu.RUnlock()

	// Cache miss: convert to an owned string once and run the full Intern path
	// (hot-cache + shard double-check, copy, store, promote). Promotion to the
	// hot cache happens here, not on hits — calling promoteToHotCache on every
	// byte-path hit would box two strings into sync.Map.LoadOrStore's interface
	// args and allocate, defeating the zero-alloc goal. The shard is the source
	// of truth, so returning without promotion is correct.
	return ki.Intern(string(b))
}

// getShard returns the shard for a key using FNV-1a hash
func (ki *KeyIntern) getShard(key string) *keyInternShard {
	h := HashStringFNV1a(key)
	return ki.shards[h&ki.shardMask]
}

// getShardFromBytes returns the shard for a byte slice using FNV-1a hash.
// PERFORMANCE: Hashes the bytes directly to avoid a string(b) allocation, so
// InternBytes can locate the shard on the cache-hit path without allocating.
// The result is identical to getShard(string(b)).
func (ki *KeyIntern) getShardFromBytes(b []byte) *keyInternShard {
	h := HashBytesFNV1a(b)
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
	// Clear hot key cache in place. Reassigning the sync.Map field
	// (ki.hotKeys = sync.Map{}) is a DATA RACE: Intern() reads ki.hotKeys
	// (Load/LoadOrStore/Delete/Range) with no field-level lock, and a struct
	// assignment is a non-atomic multi-word copy that a concurrent reader could
	// observe torn. sync.Map is safe for concurrent Range+Delete, so removing
	// entries in place keeps the field identity stable and is race-free. A key
	// added by a concurrent Intern() during the Range may survive; that is the
	// standard best-effort semantics of clearing a concurrent structure.
	ki.hotKeys.Range(func(k, _ any) bool {
		ki.hotKeys.Delete(k)
		return true
	})
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

// KeyInternStats holds statistics about the key interner.
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
