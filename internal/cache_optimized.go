package internal

import (
	"container/list"
	"crypto/sha256"
	"fmt"
	"hash/fnv"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

// OptimizedCacheManager is an improved cache manager with O(1) LRU eviction
type OptimizedCacheManager struct {
	shards []*optimizedCacheShard
	config ConfigInterface

	// Global statistics
	hitCount    int64
	missCount   int64
	memoryUsage int64
	evictions   int64

	shardCount int
	shardMask  uint64
}

// optimizedCacheShard uses LRU with O(1) operations
type optimizedCacheShard struct {
	items      map[string]*list.Element
	evictList  *list.List
	mu         sync.RWMutex
	size       int64
	memory     int64
	maxSize    int
	lastAccess int64
}

// lruEntry represents an entry in the LRU cache
type lruEntry struct {
	key        string
	value      any
	timestamp  int64
	size       int32
	hits       int64
	lastAccess int64
}

// NewOptimizedCacheManager creates an optimized cache manager
func NewOptimizedCacheManager(config ConfigInterface) *OptimizedCacheManager {
	if config == nil {
		return &OptimizedCacheManager{
			shards:     []*optimizedCacheShard{newOptimizedShard(100)},
			shardCount: 1,
			shardMask:  0,
		}
	}

	shardCount := calculateOptimalShardCount(config.GetMaxCacheSize())
	shards := make([]*optimizedCacheShard, shardCount)
	shardSize := config.GetMaxCacheSize() / shardCount
	
	for i := range shards {
		shards[i] = newOptimizedShard(shardSize)
	}

	return &OptimizedCacheManager{
		shards:     shards,
		config:     config,
		shardCount: shardCount,
		shardMask:  uint64(shardCount - 1),
	}
}

// newOptimizedShard creates a new optimized cache shard
func newOptimizedShard(maxSize int) *optimizedCacheShard {
	return &optimizedCacheShard{
		items:     make(map[string]*list.Element, maxSize),
		evictList: list.New(),
		maxSize:   maxSize,
	}
}

// calculateOptimalShardCount determines optimal shard count based on cache size
func calculateOptimalShardCount(maxSize int) int {
	if maxSize > 10000 {
		return 32
	} else if maxSize > 1000 {
		return 16
	}
	return 8
}

// Get retrieves a value from cache with O(1) complexity
func (ocm *OptimizedCacheManager) Get(key string) (any, bool) {
	if ocm.config == nil || !ocm.config.IsCacheEnabled() {
		return nil, false
	}

	shard := ocm.getShard(key)
	shard.mu.RLock()
	
	elem, exists := shard.items[key]
	if !exists {
		shard.mu.RUnlock()
		atomic.AddInt64(&ocm.missCount, 1)
		return nil, false
	}

	entry := elem.Value.(*lruEntry)
	
	// Check expiration
	if ocm.config.GetCacheTTL() > 0 {
		now := time.Now().UnixNano()
		if now-entry.timestamp > int64(ocm.config.GetCacheTTL()) {
			shard.mu.RUnlock()
			// Remove expired entry
			shard.mu.Lock()
			ocm.removeElement(shard, elem)
			shard.mu.Unlock()
			atomic.AddInt64(&ocm.missCount, 1)
			return nil, false
		}
	}

	// Update access statistics
	now := time.Now().UnixNano()
	atomic.StoreInt64(&entry.lastAccess, now)
	atomic.AddInt64(&entry.hits, 1)
	
	// Move to front (most recently used)
	shard.evictList.MoveToFront(elem)
	
	value := entry.value
	shard.mu.RUnlock()
	
	atomic.AddInt64(&ocm.hitCount, 1)
	return value, true
}

// Set stores a value in cache with O(1) complexity
func (ocm *OptimizedCacheManager) Set(key string, value any) {
	if ocm.config == nil || !ocm.config.IsCacheEnabled() {
		return
	}

	shard := ocm.getShard(key)
	now := time.Now().UnixNano()
	size := estimateSize(value)

	entry := &lruEntry{
		key:        key,
		value:      value,
		timestamp:  now,
		lastAccess: now,
		size:       int32(size),
		hits:       0,
	}

	shard.mu.Lock()
	defer shard.mu.Unlock()

	// Check if key already exists
	if elem, exists := shard.items[key]; exists {
		// Update existing entry
		oldEntry := elem.Value.(*lruEntry)
		atomic.AddInt64(&shard.memory, int64(size)-int64(oldEntry.size))
		elem.Value = entry
		shard.evictList.MoveToFront(elem)
		return
	}

	// Evict if necessary
	if shard.evictList.Len() >= shard.maxSize {
		ocm.evictOldest(shard)
	}

	// Add new entry
	elem := shard.evictList.PushFront(entry)
	shard.items[key] = elem
	atomic.AddInt64(&shard.size, 1)
	atomic.AddInt64(&shard.memory, int64(size))
	atomic.AddInt64(&ocm.memoryUsage, int64(size))
}

// evictOldest removes the least recently used entry (O(1))
func (ocm *OptimizedCacheManager) evictOldest(shard *optimizedCacheShard) {
	elem := shard.evictList.Back()
	if elem != nil {
		ocm.removeElement(shard, elem)
		atomic.AddInt64(&ocm.evictions, 1)
	}
}

// removeElement removes an element from the cache (O(1))
func (ocm *OptimizedCacheManager) removeElement(shard *optimizedCacheShard, elem *list.Element) {
	shard.evictList.Remove(elem)
	entry := elem.Value.(*lruEntry)
	delete(shard.items, entry.key)
	atomic.AddInt64(&shard.size, -1)
	atomic.AddInt64(&shard.memory, -int64(entry.size))
}

// getShard returns the appropriate shard for a key
func (ocm *OptimizedCacheManager) getShard(key string) *optimizedCacheShard {
	hash := fastHash(key)
	return ocm.shards[hash&ocm.shardMask]
}

// fastHash implements a fast hash function for cache keys
func fastHash(key string) uint64 {
	h := fnv.New64a()
	h.Write([]byte(key))
	return h.Sum64()
}

// ClearCache clears all cached data
func (ocm *OptimizedCacheManager) ClearCache() {
	for _, shard := range ocm.shards {
		shard.mu.Lock()
		shard.items = make(map[string]*list.Element, shard.maxSize)
		shard.evictList.Init()
		atomic.StoreInt64(&shard.size, 0)
		atomic.StoreInt64(&shard.memory, 0)
		shard.mu.Unlock()
	}
	atomic.StoreInt64(&ocm.memoryUsage, 0)
}

// GetStats returns cache statistics
func (ocm *OptimizedCacheManager) GetStats() CacheStats {
	totalSize := int64(0)
	totalMemory := int64(0)

	for _, shard := range ocm.shards {
		totalSize += atomic.LoadInt64(&shard.size)
		totalMemory += atomic.LoadInt64(&shard.memory)
	}

	hits := atomic.LoadInt64(&ocm.hitCount)
	misses := atomic.LoadInt64(&ocm.missCount)
	total := hits + misses

	var hitRatio float64
	if total > 0 {
		hitRatio = float64(hits) / float64(total) * 100.0
	}

	return CacheStats{
		Size:      totalSize,
		Memory:    totalMemory,
		HitCount:  hits,
		MissCount: misses,
		HitRatio:  hitRatio,
		Evictions: atomic.LoadInt64(&ocm.evictions),
	}
}

// CleanExpiredCache removes expired entries
func (ocm *OptimizedCacheManager) CleanExpiredCache() {
	if ocm.config == nil || ocm.config.GetCacheTTL() <= 0 {
		return
	}

	now := time.Now().UnixNano()
	ttl := int64(ocm.config.GetCacheTTL())

	for _, shard := range ocm.shards {
		shard.mu.Lock()
		
		// Iterate from back (oldest) to front
		for elem := shard.evictList.Back(); elem != nil; {
			entry := elem.Value.(*lruEntry)
			if now-entry.timestamp > ttl {
				next := elem.Prev()
				ocm.removeElement(shard, elem)
				elem = next
			} else {
				// Since list is ordered by access time, we can stop here
				break
			}
		}
		
		shard.mu.Unlock()
	}
}

// GetCacheSize returns total number of cached entries
func (ocm *OptimizedCacheManager) GetCacheSize() int64 {
	totalSize := int64(0)
	for _, shard := range ocm.shards {
		totalSize += atomic.LoadInt64(&shard.size)
	}
	return totalSize
}

// SecureHash creates a secure hash for cache keys
func (ocm *OptimizedCacheManager) SecureHash(input string) string {
	hash := sha256.Sum256([]byte(input))
	return fmt.Sprintf("%x", hash[:16])
}

// OptimizedCacheKey generates an optimized cache key
func OptimizedCacheKey(op, jsonStr, path string) string {
	// For small JSON, use direct concatenation
	if len(jsonStr) <= 256 {
		return op + ":" + path + ":" + jsonStr
	}

	// For large JSON, use hash-based key
	h := fnv.New64a()
	h.Write([]byte(op))
	h.Write([]byte(path))
	
	// Sample large JSON for better performance
	if len(jsonStr) > 2048 {
		h.Write([]byte(jsonStr[:1024]))
		h.Write([]byte(jsonStr[len(jsonStr)-1024:]))
	} else {
		h.Write([]byte(jsonStr))
	}
	
	return op + ":" + path + ":" + strconv.FormatUint(h.Sum64(), 36)
}
