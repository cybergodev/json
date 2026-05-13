package internal

import (
	"container/list"
	"context"
	"runtime"
		"strings"
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
	shardCount  int
	shardMask   uint64
	entryPool   *sync.Pool // Pool for lruEntry structs
	// Memory management
	maxMemory     int64 // Maximum memory for cache
	highWatermark int64 // Memory threshold for proactive eviction (80% of max)
	// Lifecycle management for cleanup goroutines
	ctx        context.Context
	cancelFunc context.CancelFunc
	wg         sync.WaitGroup
	closed     atomic.Bool // prevents wg.Add after Close
}

// cacheShard represents a single cache shard with LRU eviction
type cacheShard struct {
	items       map[string]*list.Element
	evictList   *list.List
	mu          sync.RWMutex
	size        int64
	maxSize     int
	lastCleanup int64
	// PERFORMANCE: Counter for probabilistic frequency decay
	// Instead of decaying on every eviction, we decay every N evictions
	evictionsSinceDecay int64
}

// lruEntry represents an entry in the LRU cache
type lruEntry struct {
	key        string
	value      any
	timestamp  int64
	accessTime int64
	size       int32
	hits       int64
	freq       uint8 // Access frequency counter (0-255) for LFU-style eviction
}

// resetEntry resets all fields of an lruEntry for pool reuse
// This centralizes the reset logic to avoid missing fields
func (e *lruEntry) reset() {
	e.key = ""
	e.value = nil
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
		items:     make(map[string]*list.Element, maxSize),
		evictList: list.New(),
		maxSize:   maxSize,
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
func (cm *CacheManager) Get(key string) (any, bool) {
	if !cm.cacheConfig.enableCache {
		atomic.AddInt64(&cm.missCount, 1)
		return nil, false
	}

	shard := cm.getShard(key)
	now := time.Now().UnixNano()
	ttlNanos := int64(0)
	if cm.cacheConfig.cacheTTL > 0 {
		ttlNanos = int64(cm.cacheConfig.cacheTTL.Nanoseconds())
	}

	// Fast path: read lock only
	shard.mu.RLock()
	element, exists := shard.items[key]
	if !exists {
		shard.mu.RUnlock()
		atomic.AddInt64(&cm.missCount, 1)
		return nil, false
	}

	entry := element.Value.(*lruEntry)

	// Check TTL while holding read lock
	if ttlNanos > 0 && now-entry.timestamp > ttlNanos {
		shard.mu.RUnlock()
		// Entry is expired, need write lock to delete
		shard.mu.Lock()
		// FIX: Double-check after acquiring write lock (entry might have been updated)
		// This handles the TOCTOU race condition properly
		element, exists = shard.items[key]
		if exists {
			entry = element.Value.(*lruEntry)
			// FIX: Re-check TTL with fresh timestamp after acquiring write lock
			// Another goroutine might have updated this entry
			if now-entry.timestamp > ttlNanos {
				// Still expired - delete it
				delete(shard.items, entry.key)
				shard.evictList.Remove(element)
				shard.size--
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
			shard.evictList.MoveToFront(element)
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

	// PERFORMANCE: Adaptive LRU update intervals based on hit count
	// Hot keys get less frequent position updates to reduce write lock contention
	updateInterval := int64(8)
	if hits > 100 {
		updateInterval = 32 // Less frequent updates for hot keys
	} else if hits > 50 {
		updateInterval = 16
	}

	// Only move to front periodically to reduce write lock frequency
	if hits%updateInterval == 1 {
		shard.mu.Lock()
		// Verify entry still exists (could have been deleted between unlock and lock)
		if element, exists := shard.items[key]; exists {
			entry := element.Value.(*lruEntry)
			entry.accessTime = now
			// Increment frequency for LFU-style eviction (cap at 255)
			if entry.freq < 255 {
				entry.freq++
			}
			shard.evictList.MoveToFront(element)
		}
		shard.mu.Unlock()
	}

	atomic.AddInt64(&cm.hitCount, 1)
	return value, true
}

// Set stores a value in the cache
func (cm *CacheManager) Set(key string, value any) {
	if !cm.cacheConfig.enableCache {
		return
	}

	// SECURITY: Handle long cache keys safely to prevent collisions
	// Instead of simple truncation, use hash-based truncation to avoid collisions
	if len(key) > MaxCacheKeyLength {
		key = truncateCacheKey(key)
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
	if oldElement, exists := shard.items[key]; exists {
		oldEntry := oldElement.Value.(*lruEntry)
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
		shard.evictList.MoveToFront(oldElement)
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

		element := shard.evictList.PushFront(entry)
		shard.items[key] = element
		shard.size++
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
func (cm *CacheManager) Delete(key string) {
	if !cm.cacheConfig.enableCache {
		return
	}

	shard := cm.getShard(key)
	shard.mu.Lock()
	defer shard.mu.Unlock()

	if element, exists := shard.items[key]; exists {
		entry := element.Value.(*lruEntry)
		cm.decMemoryUsage(int64(entry.size))
		delete(shard.items, key)
		shard.evictList.Remove(element)
		shard.size--

		// Return entry to pool if available
		if cm.entryPool != nil {
			entry.reset()
			cm.entryPool.Put(entry)
		}
	}
}


// DeleteByPrefix removes all cache entries whose keys contain the given prefix.
// Used for invalidating all entries related to a specific JSON input hash.
func (cm *CacheManager) DeleteByPrefix(prefix string) {
	if !cm.cacheConfig.enableCache || prefix == "" {
		return
	}

	for _, shard := range cm.shards {
		shard.mu.Lock()
		var toDelete []string
		for key := range shard.items {
			if strings.Contains(key, prefix) {
				toDelete = append(toDelete, key)
			}
		}
		for _, key := range toDelete {
			if element, exists := shard.items[key]; exists {
				entry := element.Value.(*lruEntry)
				cm.decMemoryUsage(int64(entry.size))
				delete(shard.items, key)
				shard.evictList.Remove(element)
				shard.size--
				if cm.entryPool != nil {
					entry.reset()
					cm.entryPool.Put(entry)
				}
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
			for _, element := range shard.items {
				if entry, ok := element.Value.(*lruEntry); ok {
					entry.reset()
					cm.entryPool.Put(entry)
				}
			}
		}
		shard.items = make(map[string]*list.Element, shard.maxSize)
		shard.evictList = list.New()
		shard.size = 0
		shard.mu.Unlock()
	}
	atomic.StoreInt64(&cm.memoryUsage, 0)
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

// getShard returns the appropriate shard for a key
func (cm *CacheManager) getShard(key string) *cacheShard {
	hash := cm.hashKey(key)
	return cm.shards[hash&cm.shardMask]
}

// hashKey generates a hash for the key using FNV-1a (no allocations)
func (cm *CacheManager) hashKey(key string) uint64 {
	return HashStringFNV1a(key)
}

// evictLRU evicts entries using frequency-aware LRU strategy
// PERFORMANCE: Considers access frequency to keep hot entries in cache
// OPTIMIZED: Uses probabilistic frequency decay instead of full traversal on every eviction
func (cm *CacheManager) evictLRU(shard *cacheShard) {
	element := shard.evictList.Back()
	if element == nil {
		return
	}

	// Find the best candidate for eviction among the last 5 entries
	// This provides LFU-style behavior while keeping overhead low
	candidates := 0
	bestCandidate := element
	bestEntry := element.Value.(*lruEntry)

	for e := element; e != nil && candidates < 5; e = e.Prev() {
		entry := e.Value.(*lruEntry)
		// Prefer evicting entries with lower frequency, or lower hits if frequency is equal
		if entry.freq < bestEntry.freq || (entry.freq == bestEntry.freq && entry.hits < bestEntry.hits) {
			bestCandidate = e
			bestEntry = entry
		}
		candidates++
	}

	entry := bestCandidate.Value.(*lruEntry)
	delete(shard.items, entry.key)
	shard.evictList.Remove(bestCandidate)
	shard.size--
	cm.decMemoryUsage(int64(entry.size))
	atomic.AddInt64(&cm.evictions, 1)

	// PERFORMANCE: Probabilistic frequency decay
	// Only decay frequencies every 10 evictions to reduce CPU overhead
	// This still prevents old hot entries from dominating while being much faster
	shard.evictionsSinceDecay++
	if shard.evictionsSinceDecay >= 10 {
		shard.evictionsSinceDecay = 0
		for e := shard.evictList.Front(); e != nil; e = e.Next() {
			if en := e.Value.(*lruEntry); en.freq > 0 {
				en.freq = en.freq - 1
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
		element := shard.evictList.Back()
		if element == nil {
			shard.mu.Unlock()
			return // All remaining entries are valid
		}

		entry := element.Value.(*lruEntry)
		if now-entry.timestamp > ttlNanos {
			// Remove expired entry
			delete(shard.items, entry.key)
			shard.evictList.Remove(element)
			shard.size--
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
func (cm *CacheManager) estimateSize(value any) int {
	const maxEstimate int64 = 1 << 30 // 1GB max estimate to prevent overflow

	switch v := value.(type) {
	case string:
		// String header (16 bytes) + data
		if result, ok := safeAdd(16, int64(len(v)), maxEstimate); ok {
			return int(result)
		}
		return int(maxEstimate)
	case []byte:
		// Slice header (24 bytes) + data
		if result, ok := safeAdd(24, int64(len(v)), maxEstimate); ok {
			return int(result)
		}
		return int(maxEstimate)
	case map[string]any:
		// Map overhead (48 bytes) + per-entry cost (64 bytes each)
		mapLen := int64(len(v))
		if entryCost, ok := safeMultiply(mapLen, 64, maxEstimate); ok {
			if result, ok := safeAdd(48, entryCost, maxEstimate); ok {
				return int(result)
			}
		}
		return int(maxEstimate)
	case []any:
		// Slice header (24 bytes) + per-element interface overhead (16 bytes each)
		sliceLen := int64(len(v))
		if elemCost, ok := safeMultiply(sliceLen, 16, maxEstimate); ok {
			if result, ok := safeAdd(24, elemCost, maxEstimate); ok {
				return int(result)
			}
		}
		return int(maxEstimate)
	case []PathSegment:
		// Slice header (24 bytes) + per-element struct size (128 bytes each)
		pathLen := int64(len(v))
		if elemCost, ok := safeMultiply(pathLen, 128, maxEstimate); ok {
			if result, ok := safeAdd(24, elemCost, maxEstimate); ok {
				return int(result)
			}
		}
		return int(maxEstimate)
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

// truncateCacheKey safely truncates a long cache key using FNV-1a hash.
// PERFORMANCE v2: Replaced SHA-256 with FNV-1a for ~50x faster truncation.
// Cache keys are not security-critical (they are internal), so FNV-1a's
// collision resistance is sufficient for cache key deduplication.
func truncateCacheKey(key string) string {
	if len(key) <= MaxCacheKeyLength {
		return key
	}

	// Use FNV-1a hash for fast truncation (~2ns vs ~100ns for SHA-256)
	prefixLen := MaxCacheKeyLength - 19 // "..." + 16 hex chars
	prefixLen = max(prefixLen, 0)

	// Fast FNV-1a hash of full key
	h := HashStringFNV1a(key)

	// Format hash as hex directly into result
	var hashBuf [16]byte
	const hexChars = "0123456789abcdef"
	for i := 15; i >= 0; i-- {
		hashBuf[i] = hexChars[h&0xF]
		h >>= 4
	}

	// Build result: prefix + "..." + hex hash
	result := make([]byte, 0, prefixLen+3+16)
	result = append(result, key[:prefixLen]...)
	result = append(result, '.', '.', '.')
	result = append(result, hashBuf[:]...)
	return string(result)
}
