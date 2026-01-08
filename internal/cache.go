package internal

import (
	"container/list"
	"hash/fnv"
	"sync"
	"sync/atomic"
	"time"
)

// ConfigInterface is imported from root package to avoid circular dependency
type ConfigInterface interface {
	IsCacheEnabled() bool
	GetMaxCacheSize() int
	GetCacheTTL() time.Duration
}

// CacheManager handles all caching operations with performance and memory management
type CacheManager struct {
	shards      []*cacheShard
	config      ConfigInterface
	hitCount    int64
	missCount   int64
	memoryUsage int64
	evictions   int64
	shardCount  int
	shardMask   uint64
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

// NewCacheManager creates a new cache manager with sharding
func NewCacheManager(config ConfigInterface) *CacheManager {
	if config == nil || !config.IsCacheEnabled() {
		// Return disabled cache manager
		return &CacheManager{
			shards:     []*cacheShard{newCacheShard(1)},
			config:     nil,
			shardCount: 1,
			shardMask:  0,
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

// calculateOptimalShardCount determines optimal shard count based on cache size
func calculateOptimalShardCount(maxSize int) int {
	if maxSize > 1000 {
		return 16
	} else if maxSize > 100 {
		return 8
	}
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

	shard.mu.Lock()
	defer shard.mu.Unlock()

	element, exists := shard.items[key]
	if !exists {
		atomic.AddInt64(&cm.missCount, 1)
		return nil, false
	}

	entry := element.Value.(*lruEntry)

	// Check TTL
	if ttlNanos > 0 && now-entry.timestamp > ttlNanos {
		delete(shard.items, entry.key)
		shard.evictList.Remove(element)
		shard.size--
		atomic.AddInt64(&cm.memoryUsage, -int64(entry.size))
		atomic.AddInt64(&cm.missCount, 1)
		return nil, false
	}

	// Update LRU
	entry.accessTime = now
	atomic.AddInt64(&entry.hits, 1)
	shard.evictList.MoveToFront(element)

	atomic.AddInt64(&cm.hitCount, 1)
	return entry.value, true
}

// Set stores a value in the cache
func (cm *CacheManager) Set(key string, value any) {
	if cm.config == nil || !cm.config.IsCacheEnabled() {
		return
	}

	shard := cm.getShard(key)
	now := time.Now().UnixNano()
	entrySize := cm.estimateSize(value)

	entry := &lruEntry{
		key:        key,
		value:      value,
		timestamp:  now,
		accessTime: now,
		size:       int32(entrySize),
		hits:       1,
	}

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
				go cm.cleanupShard(shard)
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

// CleanExpiredCache removes expired entries from all shards
func (cm *CacheManager) CleanExpiredCache() {
	if cm.config == nil || cm.config.GetCacheTTL() <= 0 {
		return
	}

	for _, shard := range cm.shards {
		go cm.cleanupShard(shard)
	}
}

// GetStats returns cache statistics
func (cm *CacheManager) GetStats() map[string]any {
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

	return map[string]any{
		"hit_count":         hits,
		"miss_count":        misses,
		"total_memory":      memory,
		"hit_ratio":         hitRatio,
		"memory_efficiency": memoryEfficiency,
		"evictions":         atomic.LoadInt64(&cm.evictions),
		"shard_count":       len(cm.shards),
		"entries":           totalEntries,
	}
}

// getShard returns the appropriate shard for a key
func (cm *CacheManager) getShard(key string) *cacheShard {
	hash := cm.hashKey(key)
	return cm.shards[hash&cm.shardMask]
}

// hashKey generates a hash for the key using FNV-1a (fast and sufficient for cache keys)
func (cm *CacheManager) hashKey(key string) uint64 {
	h := fnv.New64a()
	h.Write([]byte(key))
	return h.Sum64()
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
			element = prev
		} else {
			break
		}
	}
}

// estimateSize estimates the memory size of a value more accurately
func (cm *CacheManager) estimateSize(value any) int {
	switch v := value.(type) {
	case string:
		// String header (16 bytes) + data
		return 16 + len(v)
	case []byte:
		// Slice header (24 bytes) + data
		return 24 + len(v)
	case map[string]any:
		// Map overhead + estimated per-entry cost
		// Each entry: key (string) + value (interface) + hash table overhead
		return 48 + len(v)*64
	case []any:
		// Slice header + per-element interface overhead
		return 24 + len(v)*16
	case []PathSegment:
		// Slice header + per-element struct size
		return 24 + len(v)*128
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
