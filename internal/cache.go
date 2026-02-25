package internal

import (
	"container/list"
	"crypto/sha256"
	"encoding/hex"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

// maxCacheKeyLength limits the maximum length of cache keys to prevent memory issues
const maxCacheKeyLength = 1024

// CacheConfig provides the configuration needed by CacheManager
// This minimal interface avoids circular dependencies with the main json package
type CacheConfig interface {
	IsCacheEnabled() bool
	GetMaxCacheSize() int
	GetCacheTTL() time.Duration
}

// Global cleanup semaphore to limit concurrent cleanup goroutines
var (
	cleanupSem     chan struct{}
	cleanupSemOnce sync.Once
)

// getCleanupSem returns the cleanup semaphore (max 4 concurrent cleanups)
func getCleanupSem() chan struct{} {
	cleanupSemOnce.Do(func() {
		cleanupSem = make(chan struct{}, 4) // Limit to 4 concurrent cleanups
	})
	return cleanupSem
}

// CacheManager handles all caching operations with performance and memory management
type CacheManager struct {
	shards      []*cacheShard
	config      CacheConfig
	hitCount    int64
	missCount   int64
	memoryUsage int64
	evictions   int64
	shardCount  int
	shardMask   uint64
	entryPool   *sync.Pool // Pool for lruEntry structs
}

// cacheShard represents a single cache shard with LRU eviction
type cacheShard struct {
	items       map[string]*list.Element
	evictList   *list.List
	mu          sync.RWMutex
	size        int64
	maxSize     int
	lastCleanup int64
}

// lruEntry represents an entry in the LRU cache
type lruEntry struct {
	key        string
	value      any
	timestamp  int64
	accessTime int64
	size       int32
	hits       int64
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
}

// NewCacheManager creates a new cache manager with sharding
func NewCacheManager(config CacheConfig) *CacheManager {
	// Create entry pool for reuse
	entryPool := &sync.Pool{
		New: func() any {
			return &lruEntry{}
		},
	}

	if config == nil || !config.IsCacheEnabled() {
		// Return disabled cache manager
		return &CacheManager{
			shards:     []*cacheShard{newCacheShard(1)},
			config:     nil,
			shardCount: 1,
			shardMask:  0,
			entryPool:  entryPool,
		}
	}

	shardCount := calculateOptimalShardCount(config.GetMaxCacheSize())
	// Ensure shard count is power of 2 for efficient masking
	shardCount = nextPowerOf2(shardCount)
	shards := make([]*cacheShard, shardCount)
	shardSize := config.GetMaxCacheSize() / shardCount
	if shardSize < 1 {
		shardSize = 1
	}

	for i := range shards {
		shards[i] = newCacheShard(shardSize)
	}

	return &CacheManager{
		shards:     shards,
		config:     config,
		shardCount: shardCount,
		shardMask:  uint64(shardCount - 1),
		entryPool:  entryPool,
	}
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
func (cm *CacheManager) Get(key string) (any, bool) {
	if cm.config == nil || !cm.config.IsCacheEnabled() {
		atomic.AddInt64(&cm.missCount, 1)
		return nil, false
	}

	shard := cm.getShard(key)
	now := time.Now().UnixNano()
	ttlNanos := int64(0)
	if cm.config.GetCacheTTL() > 0 {
		ttlNanos = int64(cm.config.GetCacheTTL().Nanoseconds())
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
		// Double-check after acquiring write lock (entry might have been updated)
		element, exists = shard.items[key]
		if exists {
			entry = element.Value.(*lruEntry)
			if now-entry.timestamp > ttlNanos {
				delete(shard.items, entry.key)
				shard.evictList.Remove(element)
				shard.size--
				atomic.AddInt64(&cm.memoryUsage, -int64(entry.size))
				atomic.AddInt64(&cm.missCount, 1)

				// Return entry to pool if available
				if cm.entryPool != nil {
					entry.reset()
					cm.entryPool.Put(entry)
				}
				shard.mu.Unlock()
				return nil, false
			}
		}
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
			shard.evictList.MoveToFront(element)
		}
		shard.mu.Unlock()
	}

	atomic.AddInt64(&cm.hitCount, 1)
	return value, true
}

// Set stores a value in the cache
func (cm *CacheManager) Set(key string, value any) {
	if cm.config == nil || !cm.config.IsCacheEnabled() {
		return
	}

	// SECURITY: Handle long cache keys safely to prevent collisions
	// Instead of simple truncation, use hash-based truncation to avoid collisions
	if len(key) > maxCacheKeyLength {
		key = truncateCacheKey(key)
	}

	shard := cm.getShard(key)
	now := time.Now().UnixNano()
	entrySize := cm.estimateSize(value)

	// Get entry from pool
	entry := cm.entryPool.Get().(*lruEntry)
	entry.key = key
	entry.value = value
	entry.timestamp = now
	entry.accessTime = now
	entry.size = int32(entrySize)
	entry.hits = 1

	shard.mu.Lock()
	defer shard.mu.Unlock()

	// Evict if needed
	if int(shard.size) >= shard.maxSize {
		cm.evictLRU(shard)
	}

	// Store entry
	if oldElement, exists := shard.items[key]; exists {
		oldEntry := oldElement.Value.(*lruEntry)
		atomic.AddInt64(&cm.memoryUsage, int64(entry.size-oldEntry.size))
		// SECURITY: Return old entry to pool to prevent memory leak
		oldEntry.reset()
		cm.entryPool.Put(oldEntry)
		oldElement.Value = entry
		shard.evictList.MoveToFront(oldElement)
	} else {
		element := shard.evictList.PushFront(entry)
		shard.items[key] = element
		shard.size++
		atomic.AddInt64(&cm.memoryUsage, int64(entry.size))
	}

	// Periodic cleanup - trigger if enough time has passed
	// Only spawn cleanup goroutine if TTL is enabled and cleanup interval has passed
	if cm.config != nil && cm.config.GetCacheTTL() > 0 {
		lastCleanup := atomic.LoadInt64(&shard.lastCleanup)
		cleanupInterval := 30 * time.Second.Nanoseconds()
		if now-lastCleanup > cleanupInterval {
			if atomic.CompareAndSwapInt64(&shard.lastCleanup, lastCleanup, now) {
				// Use goroutine pool or limit concurrent cleanups to avoid goroutine explosion
				sem := getCleanupSem()
				select {
				case sem <- struct{}{}:
					go func(s *cacheShard) {
						defer func() { <-sem }()
						cm.cleanupShard(s)
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
	shard := cm.getShard(key)
	shard.mu.Lock()
	defer shard.mu.Unlock()

	if element, exists := shard.items[key]; exists {
		entry := element.Value.(*lruEntry)
		atomic.AddInt64(&cm.memoryUsage, -int64(entry.size))
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

// Clear removes all entries from the cache
func (cm *CacheManager) Clear() {
	for _, shard := range cm.shards {
		shard.mu.Lock()
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
	if cm.config == nil || cm.config.GetCacheTTL() <= 0 {
		return
	}

	sem := getCleanupSem()
	for _, shard := range cm.shards {
		s := shard
		select {
		case sem <- struct{}{}:
			go func() {
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

// hashKey generates a hash for the key using inline FNV-1a (no allocations)
func (cm *CacheManager) hashKey(key string) uint64 {
	// Inline FNV-1a hash - no heap allocations
	const (
		offsetBasis = 14695981039346656037
		prime       = 1099511628211
	)
	h := uint64(offsetBasis)
	for i := 0; i < len(key); i++ {
		h ^= uint64(key[i])
		h *= prime
	}
	return h
}

// evictLRU evicts the least recently used entry from a shard
func (cm *CacheManager) evictLRU(shard *cacheShard) {
	element := shard.evictList.Back()
	if element == nil {
		return
	}

	entry := element.Value.(*lruEntry)
	delete(shard.items, entry.key)
	shard.evictList.Remove(element)
	shard.size--
	atomic.AddInt64(&cm.memoryUsage, -int64(entry.size))
	atomic.AddInt64(&cm.evictions, 1)

	// Reset and return entry to pool
	entry.reset()
	cm.entryPool.Put(entry)
}

// cleanupShard removes expired entries from a shard
func (cm *CacheManager) cleanupShard(shard *cacheShard) {
	if cm.config == nil || cm.config.GetCacheTTL() <= 0 {
		return
	}

	now := time.Now().UnixNano()
	ttlNanos := int64(cm.config.GetCacheTTL().Nanoseconds())

	shard.mu.Lock()
	defer shard.mu.Unlock()

	// Iterate from back (oldest) and remove expired entries
	for element := shard.evictList.Back(); element != nil; {
		entry := element.Value.(*lruEntry)
		if now-entry.timestamp > ttlNanos {
			prev := element.Prev()
			delete(shard.items, entry.key)
			shard.evictList.Remove(element)
			shard.size--
			atomic.AddInt64(&cm.memoryUsage, -int64(entry.size))

			// Reset and return entry to pool
			entry.reset()
			cm.entryPool.Put(entry)

			element = prev
		} else {
			break
		}
	}
}

// estimateSize estimates the memory size of a value more accurately
// Uses int64 for intermediate calculations to prevent overflow
func (cm *CacheManager) estimateSize(value any) int {
	const maxEstimate = 1 << 30 // 1GB max estimate to prevent overflow

	switch v := value.(type) {
	case string:
		// String header (16 bytes) + data
		result := int64(16) + int64(len(v))
		if result > maxEstimate {
			return maxEstimate
		}
		return int(result)
	case []byte:
		// Slice header (24 bytes) + data
		result := int64(24) + int64(len(v))
		if result > maxEstimate {
			return maxEstimate
		}
		return int(result)
	case map[string]any:
		// Map overhead + estimated per-entry cost
		// Each entry: key (string) + value (interface) + hash table overhead
		result := int64(48) + int64(len(v))*64
		if result > maxEstimate {
			return maxEstimate
		}
		return int(result)
	case []any:
		// Slice header + per-element interface overhead
		result := int64(24) + int64(len(v))*16
		if result > maxEstimate {
			return maxEstimate
		}
		return int(result)
	case []PathSegment:
		// Slice header + per-element struct size
		result := int64(24) + int64(len(v))*128
		if result > maxEstimate {
			return maxEstimate
		}
		return int(result)
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

// truncateCacheKey safely truncates a long cache key using hash-based truncation
// to prevent key collisions that could occur with simple truncation
func truncateCacheKey(key string) string {
	if len(key) <= maxCacheKeyLength {
		return key
	}

	// Use SHA-256 hash of the excess portion to create a unique suffix
	// This ensures different long keys produce different truncated keys
	hashSuffixLen := 16                                // Length of hash suffix
	prefixLen := maxCacheKeyLength - hashSuffixLen - 3 // 3 for "..." separator

	if prefixLen < 0 {
		prefixLen = 0
	}

	// Calculate hash of the full key for uniqueness
	hash := sha256.Sum256([]byte(key))
	hashStr := hex.EncodeToString(hash[:])[:hashSuffixLen]

	// Return: prefix + "..." + hash suffix
	return key[:prefixLen] + "..." + hashStr
}
