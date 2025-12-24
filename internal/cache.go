package internal

import (
	"container/list"
	"crypto/sha256"
	"fmt"
	"hash/fnv"
	"sync"
	"sync/atomic"
	"time"
)

// CacheManager handles all caching operations with performance and memory management
type CacheManager struct {
	shards []*cacheShard
	config ConfigInterface

	// Global statistics
	hitCount    int64
	missCount   int64
	memoryUsage int64
	evictions   int64

	shardCount int
	shardMask  uint64
}

// cacheShard represents a single cache shard with LRU eviction
type cacheShard struct {
	items       map[string]*list.Element
	evictList   *list.List
	mu          sync.RWMutex
	size        int64
	memory      int64
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
	if config == nil {
		return &CacheManager{
			shards:     []*cacheShard{newCacheShard(100)},
			shardCount: 1,
			shardMask:  0,
		}
	}

	shardCount := calculateOptimalShardCount(config.GetMaxCacheSize())
	shards := make([]*cacheShard, shardCount)
	shardSize := config.GetMaxCacheSize() / shardCount

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

// Get retrieves a value from cache with O(1) complexity
func (cm *CacheManager) Get(key string) (any, bool) {
	if cm.config == nil || !cm.config.IsCacheEnabled() {
		atomic.AddInt64(&cm.missCount, 1)
		return nil, false
	}

	shard := cm.getShard(key)
	shard.mu.RLock()

	element, exists := shard.items[key]
	if !exists {
		shard.mu.RUnlock()
		atomic.AddInt64(&cm.missCount, 1)
		return nil, false
	}

	// Check if element.Value is nil to prevent panic
	if element.Value == nil {
		shard.mu.RUnlock()
		// Remove invalid entry
		cm.Delete(key)
		atomic.AddInt64(&cm.missCount, 1)
		return nil, false
	}

	entry, ok := element.Value.(*lruEntry)
	if !ok || entry == nil {
		shard.mu.RUnlock()
		// Remove invalid entry
		cm.Delete(key)
		atomic.AddInt64(&cm.missCount, 1)
		return nil, false
	}

	// Check TTL
	now := time.Now().UnixNano()
	if cm.config.GetCacheTTL() > 0 && now-entry.timestamp > int64(cm.config.GetCacheTTL().Nanoseconds()) {
		shard.mu.RUnlock()
		// Remove expired entry
		cm.Delete(key)
		atomic.AddInt64(&cm.missCount, 1)
		return nil, false
	}

	// Update access time and move to front (LRU)
	entry.accessTime = now
	atomic.AddInt64(&entry.hits, 1)
	shard.evictList.MoveToFront(element)
	value := entry.value

	shard.mu.RUnlock()
	atomic.AddInt64(&cm.hitCount, 1)
	return value, true
}

// Set stores a value in the cache
func (cm *CacheManager) Set(key string, value any) {
	if cm.config == nil || !cm.config.IsCacheEnabled() {
		return
	}

	shard := cm.getShard(key)
	now := time.Now().UnixNano()

	// Estimate entry size
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

	// Check if we need to evict entries
	if int(shard.size) >= shard.maxSize {
		cm.evictLRU(shard)
	}

	// Store the entry
	if oldElement, exists := shard.items[key]; exists {
		// Update existing entry
		oldEntry := oldElement.Value.(*lruEntry)
		atomic.AddInt64(&cm.memoryUsage, int64(entry.size-oldEntry.size))
		oldElement.Value = entry
		shard.evictList.MoveToFront(oldElement)
	} else {
		// New entry
		element := shard.evictList.PushFront(entry)
		shard.items[key] = element
		shard.size++
		atomic.AddInt64(&cm.memoryUsage, int64(entry.size))
	}

	// Periodic cleanup
	if now-shard.lastCleanup > 30 { // Every 30 seconds
		go cm.cleanupShard(shard)
		shard.lastCleanup = now
	}
}

// Delete removes a value from the cache
func (cm *CacheManager) Delete(key string) {
	shard := cm.getShard(key)
	shard.mu.Lock()
	defer shard.mu.Unlock()

	if element, exists := shard.items[key]; exists {
		if element.Value != nil {
			if entry, ok := element.Value.(*lruEntry); ok && entry != nil {
				atomic.AddInt64(&cm.memoryUsage, -int64(entry.size))
			}
		}
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
		shard.memory = 0
		shard.mu.Unlock()
	}
	atomic.StoreInt64(&cm.memoryUsage, 0)
	atomic.StoreInt64(&cm.hitCount, 0)
	atomic.StoreInt64(&cm.missCount, 0)
	atomic.StoreInt64(&cm.evictions, 0)
}

// ClearCache is an alias for Clear for backward compatibility
func (cm *CacheManager) ClearCache() {
	cm.Clear()
}

// CleanExpiredCache removes expired entries from all shards
func (cm *CacheManager) CleanExpiredCache() {
	if cm.config.GetCacheTTL() <= 0 {
		return
	}

	for _, shard := range cm.shards {
		go cm.cleanupShard(shard)
	}
}

// SecureHash generates a secure hash for cache keys
func (cm *CacheManager) SecureHash(data string) string {
	return fmt.Sprintf("%x", cm.hashKey(data))
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

// hashKey generates a hash for the key
func (cm *CacheManager) hashKey(key string) uint64 {
	if len(key) > 256 {
		// Use SHA256 for large keys
		h := sha256.Sum256([]byte(key))
		return uint64(h[0])<<56 | uint64(h[1])<<48 | uint64(h[2])<<40 | uint64(h[3])<<32 |
			uint64(h[4])<<24 | uint64(h[5])<<16 | uint64(h[6])<<8 | uint64(h[7])
	}

	// Use FNV-1a for small keys (faster)
	h := fnv.New64a()
	h.Write([]byte(key))
	return h.Sum64()
}

// evictLRU evicts the least recently used entry from a shard
func (cm *CacheManager) evictLRU(shard *cacheShard) {
	if shard.evictList.Len() == 0 {
		return
	}

	// Remove the least recently used entry (back of the list)
	element := shard.evictList.Back()
	if element != nil && element.Value != nil {
		if entry, ok := element.Value.(*lruEntry); ok && entry != nil {
			delete(shard.items, entry.key)
			shard.evictList.Remove(element)
			shard.size--
			atomic.AddInt64(&cm.memoryUsage, -int64(entry.size))
			atomic.AddInt64(&cm.evictions, 1)
		} else {
			// Remove invalid entry
			shard.evictList.Remove(element)
		}
	}
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

	// Iterate through the list and remove expired entries
	for element := shard.evictList.Back(); element != nil; {
		// Check if element.Value is nil to prevent panic
		if element.Value == nil {
			prev := element.Prev()
			shard.evictList.Remove(element)
			element = prev
			continue
		}

		entry, ok := element.Value.(*lruEntry)
		if !ok || entry == nil {
			// Skip invalid entries
			prev := element.Prev()
			shard.evictList.Remove(element)
			element = prev
			continue
		}

		if now-entry.timestamp > ttlNanos {
			prev := element.Prev()
			delete(shard.items, entry.key)
			shard.evictList.Remove(element)
			shard.size--
			atomic.AddInt64(&cm.memoryUsage, -int64(entry.size))
			element = prev
		} else {
			break // Since entries are ordered by access time, we can stop here
		}
	}
}

// estimateSize estimates the memory size of a value
func (cm *CacheManager) estimateSize(value any) int {
	switch v := value.(type) {
	case string:
		return len(v) + 16 // String overhead
	case []byte:
		return len(v) + 24 // Slice overhead
	case map[string]any:
		size := 48 // Map overhead
		for key, val := range v {
			size += len(key) + 16 + cm.estimateSize(val)
		}
		return size
	case []any:
		size := 24 // Slice overhead
		for _, val := range v {
			size += cm.estimateSize(val)
		}
		return size
	case int, int32, int64, uint, uint32, uint64:
		return 8
	case float32, float64:
		return 8
	case bool:
		return 1
	default:
		return 64 // Default estimate for unknown types
	}
}
