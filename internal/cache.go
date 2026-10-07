package internal

import (
	"context"
	"log/slog"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

// cacheConfigValues holds the cache configuration values extracted from Config.
// This replaces the previous CacheConfig interface to avoid forcing
// exported accessor methods on the public Config type.
type cacheConfigValues struct {
	enableCache bool
	cacheTTL    time.Duration
}

// Cache memory limits - configurable based on system resources
const (
	// DefaultMaxCacheMemory is the default maximum memory for cache (256MB)
	// This is more conservative than 2GB to work well on all systems including containers
	DefaultMaxCacheMemory = 256 * 1024 * 1024
	// CacheHighWatermarkPercent is the percentage of max memory at which proactive eviction begins
	CacheHighWatermarkPercent = 80

	// Memory bounds for cache size estimation
	minCacheMemoryBytes   = 64 * 1024 * 1024   // 64MB minimum
	maxCacheMemoryBytes   = 1024 * 1024 * 1024 // 1GB maximum
	cacheSizeMultiplier   = 4                  // Multiplier for overhead estimation
	maxConcurrentCleanups = 4                  // Maximum concurrent cleanup goroutines
)

// Global cleanup semaphore to limit concurrent cleanup goroutines
var (
	cleanupSem     chan struct{}
	cleanupSemOnce sync.Once
)

// getCleanupSem returns the cleanup semaphore (limited concurrent cleanups)
func getCleanupSem() chan struct{} {
	cleanupSemOnce.Do(func() {
		cleanupSem = make(chan struct{}, maxConcurrentCleanups)
	})
	return cleanupSem
}

// calculateMaxCacheMemory returns the maximum cache memory based on configuration
// If config provides MaxCacheSize, we estimate a reasonable memory limit
func calculateMaxCacheMemory(maxCacheSize int) int64 {
	// Base the memory limit on the cache size configuration
	// Assume each cache entry averages 1KB, so max entries * 1KB * multiplier for overhead
	estimated := int64(maxCacheSize) * 1024 * cacheSizeMultiplier
	// Clamp to reasonable bounds
	estimated = max(estimated, minCacheMemoryBytes)
	estimated = min(estimated, maxCacheMemoryBytes)
	return estimated
}

// CacheManager handles all caching operations with performance and memory management
type CacheManager struct {
	shards      []*cacheShard
	cacheConfig cacheConfigValues
	hitCount    int64
	missCount   int64
	memoryUsage int64
	evictions   int64
	// entryCount is the total number of live entries across all shards, maintained
	// atomically alongside shard.size. It lets mutation-path invalidation
	// (DeleteByPrefix, called by Set/Delete) short-circuit when the cache is empty
	// — avoiding a per-shard write-lock scan on every mutation. See EntryCount().
	entryCount int64
	shardCount int
	shardMask  uint64
	entryPool  *sync.Pool // Pool for lruEntry structs
	// Memory management
	maxMemory     int64 // Maximum memory for cache
	highWatermark int64 // Memory threshold for proactive eviction (80% of max)
	// Lifecycle management for cleanup goroutines.
	// ctx is an OWNED lifecycle context (created in NewCacheManager, cancelled
	// in Close) used solely to stop this cache's background cleanup workers —
	// not a request context stored in a struct (which would be a leak vector).
	ctx        context.Context
	cancelFunc context.CancelFunc
	wg         sync.WaitGroup
	closed     atomic.Bool // prevents wg.Add after Close
}

// CacheKey is the composite key for result-cache entries: which operation
// produced the entry, on which source document (FNV-1a of the raw JSON), at
// which path, under which configuration. OptHash 0 denotes the default
// configuration.
//
// PERFORMANCE (P-001): a comparable struct key replaces the previous
// "op:hash16:path:opts" string. Constructing that string allocated on every
// cache-enabled operation (profiling showed ~12% of all allocated objects on
// the Get path), and the cache then re-hashed the full string just to pick a
// shard. The struct is built without allocation and shards off JSONHash.
type CacheKey struct {
	Op       string
	JSONHash uint64
	Path     string
	OptHash  uint64
}

// cacheShard represents a single cache shard with LRU eviction
type cacheShard struct {
	items map[CacheKey]*lruEntry
	// lruHead/lruTail are the ends of the shard's intrusive LRU list
	// (most-recently-used at head, eviction candidates at tail). They replace
	// container/list, whose per-insert *list.Element allocation was ~12% of
	// allocated objects on the Get path (P-001 profile).
	lruHead, lruTail *lruEntry
	mu               sync.RWMutex
	size             int64
	maxSize          int
	lastCleanup      int64
	// PERFORMANCE: Counter for probabilistic frequency decay
	// Instead of decaying on every eviction, we decay every N evictions
	evictionsSinceDecay int64
}

// lruEntry represents an entry in the LRU cache
type lruEntry struct {
	key   CacheKey
	value any
	// prev/next are the intrusive LRU links; both must be nil whenever the
	// entry is not linked into a shard list (in particular when pooled).
	prev, next *lruEntry
	timestamp  int64
	accessTime int64
	size       int32
	hits       int64
	freq       uint8 // Access frequency counter (0-255) for LFU-style eviction
}

// linkFront inserts e at the head of the shard's LRU list.
func (s *cacheShard) linkFront(e *lruEntry) {
	e.prev = nil
	e.next = s.lruHead
	if s.lruHead != nil {
		s.lruHead.prev = e
	}
	s.lruHead = e
	if s.lruTail == nil {
		s.lruTail = e
	}
}

// unlink removes e from the shard's LRU list.
func (s *cacheShard) unlink(e *lruEntry) {
	if e.prev != nil {
		e.prev.next = e.next
	} else {
		s.lruHead = e.next
	}
	if e.next != nil {
		e.next.prev = e.prev
	} else {
		s.lruTail = e.prev
	}
	e.prev, e.next = nil, nil
}

// moveToFront relinks an already-linked entry at the head of the LRU list.
func (s *cacheShard) moveToFront(e *lruEntry) {
	if s.lruHead == e {
		return
	}
	s.unlink(e)
	s.linkFront(e)
}

// resetEntry resets all fields of an lruEntry for pool reuse
// This centralizes the reset logic to avoid missing fields
func (e *lruEntry) reset() {
	e.key = CacheKey{}
	e.value = nil
	e.prev, e.next = nil, nil
	e.timestamp = 0
	e.accessTime = 0
	e.size = 0
	e.hits = 0
	e.freq = 0
}

// NewCacheManager creates a new cache manager with sharding.
// enableCache controls whether caching is active.
// maxCacheSize sets the maximum number of cache entries.
// cacheTTL sets the time-to-live for cache entries.
func NewCacheManager(enableCache bool, maxCacheSize int, cacheTTL time.Duration) *CacheManager {
	// Create entry pool for reuse
	entryPool := &sync.Pool{
		New: func() any {
			return &lruEntry{}
		},
	}

	// Create context for lifecycle management
	ctx, cancel := context.WithCancel(context.Background())

	if !enableCache {
		// Return disabled cache manager
		return &CacheManager{
			shards:        []*cacheShard{newCacheShard(1)},
			cacheConfig:   cacheConfigValues{enableCache: false, cacheTTL: cacheTTL},
			shardCount:    1,
			shardMask:     0,
			entryPool:     entryPool,
			ctx:           ctx,
			cancelFunc:    cancel,
			maxMemory:     DefaultMaxCacheMemory,
			highWatermark: int64(DefaultMaxCacheMemory * CacheHighWatermarkPercent / 100),
		}
	}

	shardCount := calculateOptimalShardCount(maxCacheSize)
	// Ensure shard count is power of 2 for efficient masking
	shardCount = nextPowerOf2(shardCount)
	shards := make([]*cacheShard, shardCount)
	shardSize := maxCacheSize / shardCount
	shardSize = max(shardSize, 1)

	for i := range shards {
		shards[i] = newCacheShard(shardSize)
	}

	// Calculate memory limits based on configuration
	maxMem := calculateMaxCacheMemory(maxCacheSize)
	highWater := int64(maxMem * int64(CacheHighWatermarkPercent) / 100)

	return &CacheManager{
		shards:        shards,
		cacheConfig:   cacheConfigValues{enableCache: true, cacheTTL: cacheTTL},
		shardCount:    shardCount,
		shardMask:     uint64(shardCount - 1),
		entryPool:     entryPool,
		ctx:           ctx,
		cancelFunc:    cancel,
		maxMemory:     maxMem,
		highWatermark: highWater,
	}
}

// Close gracefully shuts down the cache manager, waiting for cleanup goroutines to complete
func (cm *CacheManager) Close() {
	// Set closed flag first to prevent new wg.Add(1) calls
	cm.closed.Store(true)
	if cm.cancelFunc != nil {
		cm.cancelFunc()
	}
	// Wait for all cleanup goroutines to finish
	cm.wg.Wait()
}

// newCacheShard creates a new cache shard
func newCacheShard(maxSize int) *cacheShard {
	return &cacheShard{
		items:   make(map[CacheKey]*lruEntry, maxSize),
		maxSize: maxSize,
	}
}

// calculateOptimalShardCount determines optimal shard count based on cache size and CPU count
// PERFORMANCE: CPU-aware sharding reduces lock contention on multi-core systems
func calculateOptimalShardCount(maxSize int) int {
	cpuCount := runtime.GOMAXPROCS(0)
	if maxSize > 10000 {
		// Large cache: scale with CPU count, minimum 32 shards
		return max(cpuCount*4, 32)
	} else if maxSize > 1000 {
		// Medium cache: scale with CPU count, minimum 16 shards
		return max(cpuCount*2, 16)
	} else if maxSize > 100 {
		// Small cache: scale with CPU count, minimum 8 shards
		return max(cpuCount, 8)
	}
	// Very small cache: minimum 4 shards
	return 4
}

// nextPowerOf2 returns the next power of 2 greater than or equal to n
func nextPowerOf2(n int) int {
	if n <= 1 {
		return 1
	}
	// Check if already power of 2
	if n&(n-1) == 0 {
		return n
	}
	// Find next power of 2
	power := 1
	for power < n {
		power <<= 1
	}
	return power
}

// Get retrieves a value from cache with O(1) complexity
// PERFORMANCE: Optimized to minimize lock contention
// - Uses RLock for the common fast path
// - Only upgrades to Lock when TTL expiration needs cleanup
// - LRU position update is deferred to reduce write lock frequency
// FIX: Properly handles TOCTOU race condition by re-validating entry after lock upgrade
func (cm *CacheManager) Get(key CacheKey) (any, bool) {
	if !cm.cacheConfig.enableCache {
		atomic.AddInt64(&cm.missCount, 1)
		return nil, false
	}

	shard := cm.getShard(key)

	// Fast path: read lock only
	shard.mu.RLock()
	entry, exists := shard.items[key]
	if !exists {
		shard.mu.RUnlock()
		atomic.AddInt64(&cm.missCount, 1)
		return nil, false
	}

	// PERFORMANCE: defer time.Now() and the TTL computation until we know the
	// entry exists. On a cache miss (above) the timestamp is never used, so this
	// avoids a time.Now() call (~50-100ns on Windows) per miss. The check already
	// holds the shared RLock, and readers do not block one another, so computing
	// the timestamp here does not reduce read concurrency.
	now := time.Now().UnixNano()
	ttlNanos := int64(0)
	if cm.cacheConfig.cacheTTL > 0 {
		ttlNanos = int64(cm.cacheConfig.cacheTTL.Nanoseconds())
	}

	// Check TTL while holding read lock
	if ttlNanos > 0 && now-entry.timestamp > ttlNanos {
		shard.mu.RUnlock()
		// Entry is expired, need write lock to delete
		shard.mu.Lock()
		// FIX: Double-check after acquiring write lock (entry might have been updated)
		// This handles the TOCTOU race condition properly
		entry, exists = shard.items[key]
		if exists {
			// FIX: Re-check TTL with fresh timestamp after acquiring write lock
			// Another goroutine might have updated this entry
			if now-entry.timestamp > ttlNanos {
				// Still expired - delete it
				delete(shard.items, entry.key)
				shard.unlink(entry)
				shard.size--
				atomic.AddInt64(&cm.entryCount, -1)
				cm.decMemoryUsage(int64(entry.size))
				atomic.AddInt64(&cm.missCount, 1)

				// Return entry to pool if available
				if cm.entryPool != nil {
					entry.reset()
					cm.entryPool.Put(entry)
				}
				shard.mu.Unlock()
				return nil, false
			}
			// FIX: Entry was updated by another goroutine and is now valid
			// Return the updated value instead of a miss
			value := entry.value
			atomic.AddInt64(&entry.hits, 1) // Update hit count, return value not needed here
			entry.accessTime = now
			if entry.freq < 255 {
				entry.freq++
			}
			shard.moveToFront(entry)
			shard.mu.Unlock()

			atomic.AddInt64(&cm.hitCount, 1)
			return value, true
		}
		// Entry was deleted by another goroutine
		shard.mu.Unlock()
		atomic.AddInt64(&cm.missCount, 1)
		return nil, false
	}

	// Entry exists and is valid, copy value before releasing lock
	value := entry.value
	// PERFORMANCE: Update hit count atomically without lock
	hits := atomic.AddInt64(&entry.hits, 1)
	shard.mu.RUnlock()

	// PERFORMANCE: Adaptive LRU update intervals based on hit count.
	//
	// Profiling (P-001) showed the periodic moveToFront write lock was the
	// dominant source of reader stalls under concurrent hot-key access: every
	// RWMutex writer arrival forces in-flight/blocked readers through the
	// kernel semaphore (runtime.lock2/stdlib2 on Windows), which accounted for
	// ~37% of CPU on BenchmarkConcurrent_Get. Raising the interval means a
	// writer arrives far less often, so readers stay on the cheap atomic
	// RLock fast path.
	//
	// This is safe for eviction quality: entry.hits advances atomically
	// (lock-free) on every access, so evictLRU still sees an accurate
	// per-entry frequency even when the list position lags behind. The list
	// position only refines the LFU tie-break among entries with similar hit
	// counts, so a stale position never evicts a genuinely hot entry.
	updateInterval := int64(64)
	switch {
	case hits > 4096:
		updateInterval = 2048
	case hits > 1024:
		updateInterval = 512
	case hits > 256:
		updateInterval = 128
	}

	// Only move to front periodically to reduce write lock frequency
	if hits%updateInterval == 1 {
		shard.mu.Lock()
		// Verify entry still exists (could have been deleted between unlock and lock)
		if entry, exists := shard.items[key]; exists {
			entry.accessTime = now
			// Increment frequency for LFU-style eviction (cap at 255)
			if entry.freq < 255 {
				entry.freq++
			}
			shard.moveToFront(entry)
		}
		shard.mu.Unlock()
	}

	atomic.AddInt64(&cm.hitCount, 1)
	return value, true
}

// Set stores a value in the cache
func (cm *CacheManager) Set(key CacheKey, value any) {
	if !cm.cacheConfig.enableCache {
		return
	}

	shard := cm.getShard(key)
	now := time.Now().UnixNano()
	entrySize := cm.estimateSize(value)

	shard.mu.Lock()
	defer shard.mu.Unlock()

	// Evict if needed
	if int(shard.size) >= shard.maxSize {
		cm.evictLRU(shard)
	}

	// SECURITY: Check total memory usage to prevent unbounded growth
	// Proactively evict if approaching memory limit (highWatermark is 80% of max)
	currentMemory := atomic.LoadInt64(&cm.memoryUsage)
	if currentMemory+int64(entrySize) > cm.highWatermark {
		// Proactively evict to prevent memory exhaustion
		for i := 0; i < 3 && atomic.LoadInt64(&cm.memoryUsage)+int64(entrySize) > cm.highWatermark; i++ {
			cm.evictLRU(shard)
		}
	}

	// Store entry - OPTIMIZED: Update existing entry in-place to avoid pool churn
	if oldEntry, exists := shard.items[key]; exists {
		atomic.AddInt64(&cm.memoryUsage, int64(entrySize)-int64(oldEntry.size))
		// Clamp to prevent negative values from estimation inaccuracies
		if atomic.LoadInt64(&cm.memoryUsage) < 0 {
			atomic.StoreInt64(&cm.memoryUsage, 0)
		}
		// Update existing entry in-place to avoid pool churn and race conditions
		oldEntry.value = value
		oldEntry.timestamp = now
		oldEntry.accessTime = now
		oldEntry.size = int32(entrySize)
		oldEntry.hits = 1
		oldEntry.freq = 0 // Reset frequency for new entry
		shard.moveToFront(oldEntry)
	} else {
		// Only allocate new entry for new keys
		entry := cm.entryPool.Get().(*lruEntry)
		entry.key = key
		entry.value = value
		entry.timestamp = now
		entry.accessTime = now
		entry.size = int32(entrySize)
		entry.hits = 1
		entry.freq = 0

		shard.linkFront(entry)
		shard.items[key] = entry
		shard.size++
		atomic.AddInt64(&cm.entryCount, 1)
		atomic.AddInt64(&cm.memoryUsage, int64(entrySize))
	}

	// Periodic cleanup - trigger if enough time has passed
	// Only spawn cleanup goroutine if TTL is enabled and cleanup interval has passed
	if cm.cacheConfig.cacheTTL > 0 {
		lastCleanup := atomic.LoadInt64(&shard.lastCleanup)
		cleanupInterval := 30 * time.Second.Nanoseconds()
		if now-lastCleanup > cleanupInterval {
			if atomic.CompareAndSwapInt64(&shard.lastCleanup, lastCleanup, now) {
				// Use goroutine pool or limit concurrent cleanups to avoid goroutine explosion
				sem := getCleanupSem()
				select {
				case sem <- struct{}{}:
					// Check closed flag before wg.Add to prevent WaitGroup reuse panic.
					// Close() sets closed=true before wg.Wait(), so if we see
					// closed=true here, Close() will not wait for us.
					if cm.closed.Load() {
						<-sem
						return
					}
					cm.wg.Add(1)
					go func(s *cacheShard) {
						// SAFETY (SEC-003): cleanup runs as a side effect of cache writes;
						// a panic must not crash the caller. Registered first so wg.Done()
						// and the semaphore release still run on panic (defers are LIFO).
						defer func() {
							if r := recover(); r != nil {
								slog.Error("cache cleanup panicked", slog.Any("panic", r))
							}
						}()
						defer cm.wg.Done()
						defer func() { <-sem }()
						// Check context before running cleanup
						select {
						case <-cm.ctx.Done():
							return
						default:
							cm.cleanupShard(s)
						}
					}(shard)
				default:
					// Skip cleanup if semaphore is full (too many concurrent cleanups)
					// Next Set operation will try again
				}
			}
		}
	}
}

// Delete removes a value from the cache
func (cm *CacheManager) Delete(key CacheKey) {
	if !cm.cacheConfig.enableCache {
		return
	}

	shard := cm.getShard(key)
	shard.mu.Lock()
	defer shard.mu.Unlock()

	if entry, exists := shard.items[key]; exists {
		cm.decMemoryUsage(int64(entry.size))
		delete(shard.items, key)
		shard.unlink(entry)
		shard.size--
		atomic.AddInt64(&cm.entryCount, -1)

		// Return entry to pool if available
		if cm.entryPool != nil {
			entry.reset()
			cm.entryPool.Put(entry)
		}
	}
}

// EntryCount returns the total number of live entries across all shards.
// It is the fast, lock-free probe used by mutation-path cache invalidation to
// detect the empty case and skip a full per-shard scan.
func (cm *CacheManager) EntryCount() int64 {
	if !cm.cacheConfig.enableCache {
		return 0
	}
	return atomic.LoadInt64(&cm.entryCount)
}

// DeleteByJSONHash removes every cache entry produced from the document with
// the given FNV-1a hash (all ops, paths, and configs). Used by mutation
// operations to invalidate everything derived from a JSON input.
//
// P-001: replaces DeleteByPrefix(hexString), which formatted the hash to hex
// and substring-matched it against every key — besides the formatting cost,
// a key whose PATH portion happened to contain the 16 hex digits of another
// document's hash was falsely invalidated. Exact field equality has neither
// cost nor false positive.
func (cm *CacheManager) DeleteByJSONHash(jsonHash uint64) {
	if !cm.cacheConfig.enableCache {
		return
	}

	// PERFORMANCE: skip the per-shard write-lock scan entirely when the cache
	// holds no entries. Set/Delete call this on EVERY mutation; with an empty
	// (or just-Clear'd) cache there is nothing to invalidate, and the scan would
	// still acquire and release a write lock on every shard for no work.
	// Profiling (P-001) showed this scan dominated Set/Delete CPU when the cache
	// was unpopulated. entryCount is maintained atomically alongside shard.size.
	if atomic.LoadInt64(&cm.entryCount) == 0 {
		return
	}

	for _, shard := range cm.shards {
		shard.mu.Lock()
		for key, entry := range shard.items {
			if key.JSONHash != jsonHash {
				continue
			}
			cm.decMemoryUsage(int64(entry.size))
			delete(shard.items, key)
			shard.unlink(entry)
			shard.size--
			atomic.AddInt64(&cm.entryCount, -1)
			if cm.entryPool != nil {
				entry.reset()
				cm.entryPool.Put(entry)
			}
		}
		shard.mu.Unlock()
	}
}

// Clear removes all entries from the cache.
// Returns all lruEntry objects to the pool to prevent memory leaks.
func (cm *CacheManager) Clear() {
	for _, shard := range cm.shards {
		shard.mu.Lock()
		// Return all entries to pool before discarding maps
		if cm.entryPool != nil {
			for _, entry := range shard.items {
				entry.reset()
				cm.entryPool.Put(entry)
			}
		}
		shard.items = make(map[CacheKey]*lruEntry, shard.maxSize)
		shard.lruHead, shard.lruTail = nil, nil
		shard.size = 0
		shard.mu.Unlock()
	}
	atomic.StoreInt64(&cm.memoryUsage, 0)
	atomic.StoreInt64(&cm.entryCount, 0)
	atomic.StoreInt64(&cm.hitCount, 0)
	atomic.StoreInt64(&cm.missCount, 0)
	atomic.StoreInt64(&cm.evictions, 0)
}

// CleanExpiredCache removes expired entries from all shards (with goroutine limit)
func (cm *CacheManager) CleanExpiredCache() {
	if cm.cacheConfig.cacheTTL <= 0 {
		return
	}

	// Check if context is already cancelled
	select {
	case <-cm.ctx.Done():
		return
	default:
	}

	sem := getCleanupSem()
	for _, shard := range cm.shards {
		s := shard
		select {
		case <-cm.ctx.Done():
			return // Stop spawning new goroutines if context is cancelled
		case sem <- struct{}{}:
			// Check closed flag before wg.Add to prevent WaitGroup reuse panic
			if cm.closed.Load() {
				<-sem
				return
			}
			cm.wg.Add(1)
			go func() {
				// SAFETY (SEC-003): see the periodic-cleanup goroutine above.
				defer func() {
					if r := recover(); r != nil {
						slog.Error("cache cleanup panicked", slog.Any("panic", r))
					}
				}()
				defer cm.wg.Done()
				defer func() { <-sem }()
				cm.cleanupShard(s)
			}()
		default:
			// Skip this shard if semaphore is full
			// This prevents unbounded goroutine growth
		}
	}
}

// CacheStats represents cache statistics
type CacheStats struct {
	Entries          int64
	TotalMemory      int64
	HitCount         int64
	MissCount        int64
	HitRatio         float64
	MemoryEfficiency float64
	Evictions        int64
	ShardCount       int
}

// GetStats returns cache statistics
func (cm *CacheManager) GetStats() CacheStats {
	totalEntries := int64(0)
	for _, shard := range cm.shards {
		shard.mu.RLock()
		totalEntries += shard.size
		shard.mu.RUnlock()
	}

	hits := atomic.LoadInt64(&cm.hitCount)
	misses := atomic.LoadInt64(&cm.missCount)
	total := hits + misses

	var hitRatio float64
	if total > 0 {
		hitRatio = float64(hits) / float64(total)
	}

	var memoryEfficiency float64
	memory := atomic.LoadInt64(&cm.memoryUsage)
	if memory > 0 {
		memoryMB := float64(memory) / (1024 * 1024)
		memoryEfficiency = float64(hits) / memoryMB
	}

	return CacheStats{
		Entries:          totalEntries,
		TotalMemory:      memory,
		HitCount:         hits,
		MissCount:        misses,
		HitRatio:         hitRatio,
		MemoryEfficiency: memoryEfficiency,
		Evictions:        atomic.LoadInt64(&cm.evictions),
		ShardCount:       len(cm.shards),
	}
}

// getShard returns the appropriate shard for a key.
//
// PERFORMANCE (P-001): shards off the key's well-distributed JSONHash mixed
// with the op tag and config hash. The previous string-key design re-hashed
// the entire composed key with FNV-1a on every Get/Set/Delete just to pick a
// shard; JSONHash is already uniform, so only the short op string is hashed.
func (cm *CacheManager) getShard(key CacheKey) *cacheShard {
	h := key.JSONHash
	h ^= key.OptHash * 0x9E3779B97F4A7C15
	h ^= HashStringFNV1a(key.Op)
	return cm.shards[h&cm.shardMask]
}

// evictLRU evicts entries using frequency-aware LRU strategy
// PERFORMANCE: Considers access frequency to keep hot entries in cache
// OPTIMIZED: Uses probabilistic frequency decay instead of full traversal on every eviction
func (cm *CacheManager) evictLRU(shard *cacheShard) {
	entry := shard.lruTail
	if entry == nil {
		return
	}

	// Find the best candidate for eviction among the last 5 entries
	// This provides LFU-style behavior while keeping overhead low
	candidates := 0
	bestCandidate := entry

	for e := entry; e != nil && candidates < 5; e = e.prev {
		// Prefer evicting entries with lower frequency, or lower hits if frequency is equal
		if e.freq < bestCandidate.freq || (e.freq == bestCandidate.freq && e.hits < bestCandidate.hits) {
			bestCandidate = e
		}
		candidates++
	}

	entry = bestCandidate
	delete(shard.items, entry.key)
	shard.unlink(entry)
	shard.size--
	atomic.AddInt64(&cm.entryCount, -1)
	cm.decMemoryUsage(int64(entry.size))
	atomic.AddInt64(&cm.evictions, 1)

	// PERFORMANCE: Probabilistic frequency decay
	// Only decay frequencies every 10 evictions to reduce CPU overhead
	// This still prevents old hot entries from dominating while being much faster
	shard.evictionsSinceDecay++
	if shard.evictionsSinceDecay >= 10 {
		shard.evictionsSinceDecay = 0
		for e := shard.lruHead; e != nil; e = e.next {
			if e.freq > 0 {
				e.freq = e.freq - 1
			}
		}
	}

	// Reset and return entry to pool
	entry.reset()
	cm.entryPool.Put(entry)
}

// cleanupBatchSize is the number of entries to process before yielding the lock
const cleanupBatchSize = 50

// cleanupShard removes expired entries from a shard
// OPTIMIZED: Uses batched cleanup with lock release intervals to allow concurrent reads
// FIX: Previously held write lock during entire traversal, blocking concurrent reads
func (cm *CacheManager) cleanupShard(shard *cacheShard) {
	if cm.cacheConfig.cacheTTL <= 0 {
		return
	}

	now := time.Now().UnixNano()
	ttlNanos := int64(cm.cacheConfig.cacheTTL.Nanoseconds())

	processed := 0

	for {
		shard.mu.Lock()
		entry := shard.lruTail
		if entry == nil {
			shard.mu.Unlock()
			return // All remaining entries are valid
		}

		if now-entry.timestamp > ttlNanos {
			// Remove expired entry
			delete(shard.items, entry.key)
			shard.unlink(entry)
			shard.size--
			atomic.AddInt64(&cm.entryCount, -1)
			cm.decMemoryUsage(int64(entry.size))

			// Reset and return entry to pool
			entry.reset()
			cm.entryPool.Put(entry)

			processed++
			shard.mu.Unlock()

			// Yield lock every batchSize to allow concurrent reads
			if processed%cleanupBatchSize == 0 {
				runtime.Gosched()
			}
		} else {
			shard.mu.Unlock()
			return // All remaining entries are valid (oldest is not expired)
		}
	}
}

// safeMultiply performs overflow-safe multiplication for size estimation.
// Returns (maxVal, false) if overflow would occur, otherwise (a*b, true).
func safeMultiply(a, b, maxVal int64) (int64, bool) {
	if a == 0 || b == 0 {
		return 0, true
	}
	if a > maxVal/b {
		return maxVal, false
	}
	return a * b, true
}

// safeAdd performs overflow-safe addition for size estimation.
// Returns (maxVal, false) if result would exceed maxVal, otherwise (a+b, true).
func safeAdd(a, b, maxVal int64) (int64, bool) {
	result := a + b
	if result > maxVal || result < a { // second check handles actual overflow
		return maxVal, false
	}
	return result, true
}

// estimateSize estimates the memory size of a value more accurately
// Uses int64 for intermediate calculations to prevent overflow
//
// GEN-001: containers are now descended with a BOUNDED recursive walk — up to
// estimateMaxDepth levels and estimateNodeBudget nodes — so key strings and
// nested contents are accounted, fixing the D-002 limitation where a parsed
// 1MB document with 50 keys was estimated at ~3KB (2-3 orders of magnitude
// under its real footprint, leaving the memory high-watermark unable to bind).
// Portions beyond the depth/budget bounds still fall back to the flat
// per-entry charge, so worst-case cost is O(budget) per estimate while
// typical documents are accounted to within a small factor. The ENTRY-COUNT
// LRU bound (MaxCacheSize) remains the hard memory limit; the watermark is a
// soft secondary bound.
func (cm *CacheManager) estimateSize(value any) int {
	budget := estimateNodeBudget
	return int(estimateValueSize(value, 0, &budget))
}

// Bounds for the recursive size estimate (GEN-001). maxDepth counts
// container levels below the root value; nodeBudget caps total visited nodes
// so a hostile deep/wide tree cannot turn estimation into a full traversal.
const (
	estimateMaxDepth   = 4
	estimateNodeBudget = 4096
)

// estimateValueSize is the recursive core of estimateSize. budget is
// decremented per visited node; when it reaches zero, remaining siblings are
// charged only the flat per-entry overhead (the pre-GEN-001 behavior).
func estimateValueSize(value any, depth int, budget *int) int64 {
	const maxEstimate int64 = 1 << 30 // 1GB max estimate to prevent overflow

	switch v := value.(type) {
	case string:
		// String header (16 bytes) + data
		if result, ok := safeAdd(16, int64(len(v)), maxEstimate); ok {
			return result
		}
		return maxEstimate
	case []byte:
		// Slice header (24 bytes) + data
		if result, ok := safeAdd(24, int64(len(v)), maxEstimate); ok {
			return result
		}
		return maxEstimate
	case map[string]any:
		// Map header (48 bytes) + per entry: bucket overhead (64 B), key
		// string (16 B header + data), and the value's recursive estimate.
		total := int64(48)
		visited := 0
		for k, val := range v {
			if *budget == 0 {
				break
			}
			*budget--
			visited++
			entry := int64(64 + 16 + len(k))
			if depth+1 < estimateMaxDepth {
				entry += estimateValueSize(val, depth+1, budget)
				if entry > maxEstimate {
					entry = maxEstimate
				}
			}
			if result, ok := safeAdd(total, entry, maxEstimate); ok {
				total = result
			} else {
				return maxEstimate
			}
		}
		if remaining := int64(len(v) - visited); remaining > 0 {
			// Budget exhausted: flat per-entry charge for the unvisited rest
			if cost, ok := safeMultiply(remaining, 64, maxEstimate); ok {
				if result, ok := safeAdd(total, cost, maxEstimate); ok {
					return result
				}
			}
			return maxEstimate
		}
		return total
	case []any:
		// Slice header (24 bytes) + per element: interface overhead (16 B)
		// and the element's recursive estimate.
		total := int64(24)
		visited := 0
		for _, elem := range v {
			if *budget == 0 {
				break
			}
			*budget--
			visited++
			entry := int64(16)
			if depth+1 < estimateMaxDepth {
				entry += estimateValueSize(elem, depth+1, budget)
				if entry > maxEstimate {
					entry = maxEstimate
				}
			}
			if result, ok := safeAdd(total, entry, maxEstimate); ok {
				total = result
			} else {
				return maxEstimate
			}
		}
		if remaining := int64(len(v) - visited); remaining > 0 {
			if cost, ok := safeMultiply(remaining, 16, maxEstimate); ok {
				if result, ok := safeAdd(total, cost, maxEstimate); ok {
					return result
				}
			}
			return maxEstimate
		}
		return total
	case []PathSegment:
		// Slice header (24 bytes) + per-element struct size (128 bytes each)
		pathLen := int64(len(v))
		if elemCost, ok := safeMultiply(pathLen, 128, maxEstimate); ok {
			if result, ok := safeAdd(24, elemCost, maxEstimate); ok {
				return result
			}
		}
		return maxEstimate
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return 8
	case float32, float64:
		return 8
	case bool:
		return 1
	case nil:
		return 0
	default:
		// Conservative estimate for unknown types
		return 128
	}
}

// decMemoryUsage decrements memoryUsage by delta and clamps to zero
// to prevent negative values from estimation inaccuracies.
func (cm *CacheManager) decMemoryUsage(delta int64) {
	for {
		old := atomic.LoadInt64(&cm.memoryUsage)
		newVal := old - delta
		if newVal < 0 {
			newVal = 0
		}
		if atomic.CompareAndSwapInt64(&cm.memoryUsage, old, newVal) {
			return
		}
	}
}
