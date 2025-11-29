package internal

import (
	"crypto/sha256"
	"fmt"
	"reflect"
	"sync"
	"sync/atomic"
	"time"
)

// CacheManager handles all caching operations with performance and memory management
type CacheManager struct {
	// Core cache storage with sharding for better concurrency
	shards []*cacheShard

	// Global statistics (atomic counters)
	hitCount    int64 // Cache hits
	missCount   int64 // Cache misses
	memoryUsage int64 // Estimated memory usage in bytes
	evictions   int64 // Number of evictions performed

	// Configuration
	config ConfigInterface

	// Sharding configuration
	shardCount int
	shardMask  uint64
}

// cacheShard represents a single cache shard for improved concurrency
type cacheShard struct {
	data   sync.Map     // Thread-safe map for this shard
	mu     sync.RWMutex // Read-write mutex for shard-level operations
	size   int64        // Number of entries in this shard (atomic)
	memory int64        // Memory usage of this shard (atomic)

	// Enhanced concurrency control
	lastCleanup    int64 // Last cleanup timestamp (atomic)
	cleanupRunning int32 // Cleanup operation flag (atomic)
	accessCount    int64 // Access count for this shard (atomic)
}

// cacheEntry represents a cache entry with memory tracking and access patterns
type cacheEntry struct {
	data       any   // The cached data
	timestamp  int64 // Creation timestamp (Unix nanoseconds, atomic)
	lastAccess int64 // Last access timestamp (Unix nanoseconds, atomic)
	hits       int64 // Access count (atomic)
	size       int32 // Estimated size in bytes
	frequency  int32 // Access frequency for LFU (atomic)
}

// NewCacheManager creates a new cache manager with sharding
func NewCacheManager(config ConfigInterface) *CacheManager {
	if config == nil {
		return &CacheManager{
			shards:     []*cacheShard{{}},
			shardCount: 1,
			shardMask:  0,
		}
	}

	// Calculate optimal shard count (power of 2)
	shardCount := 16 // Default shard count
	maxCacheSize := config.GetMaxCacheSize()
	if maxCacheSize > 10000 {
		shardCount = 32
	} else if maxCacheSize > 1000 {
		shardCount = 16
	} else {
		shardCount = 8
	}

	shards := make([]*cacheShard, shardCount)
	for i := range shards {
		shards[i] = &cacheShard{}
	}

	return &CacheManager{
		shards:     shards,
		config:     config,
		shardCount: shardCount,
		shardMask:  uint64(shardCount - 1),
	}
}

// getShard returns the appropriate shard for a given key
func (cm *CacheManager) getShard(key string) *cacheShard {
	hash := fnv1aHash(key)
	return cm.shards[hash&cm.shardMask]
}

// Get retrieves a value from cache with enhanced concurrency
func (cm *CacheManager) Get(key string) (any, bool) {
	if cm.config == nil || !cm.config.IsCacheEnabled() {
		return nil, false
	}

	shard := cm.getShard(key)

	// Increment access count for this shard
	atomic.AddInt64(&shard.accessCount, 1)

	value, exists := shard.data.Load(key)
	if !exists {
		atomic.AddInt64(&cm.missCount, 1)
		return nil, false
	}

	entry := value.(*cacheEntry)

	// Fast path: check if expired without creating time objects
	if cm.config != nil && cm.config.GetCacheTTL() > 0 {
		timestamp := atomic.LoadInt64(&entry.timestamp)
		now := time.Now().UnixNano()
		if now-timestamp > int64(cm.config.GetCacheTTL()) {
			// Entry is expired, remove it
			if shard.data.CompareAndDelete(key, value) {
				atomic.AddInt64(&shard.size, -1)
				atomic.AddInt64(&shard.memory, -int64(entry.size))
			}
			atomic.AddInt64(&cm.missCount, 1)
			return nil, false
		}
	}

	// Update access statistics atomically for thread safety
	now := time.Now().UnixNano()
	atomic.StoreInt64(&entry.lastAccess, now)
	atomic.AddInt64(&entry.hits, 1)
	atomic.AddInt32(&entry.frequency, 1)
	atomic.AddInt64(&cm.hitCount, 1)

	return entry.data, true
}

// Set stores a value in cache
func (cm *CacheManager) Set(key string, value any) {
	if cm.config == nil || !cm.config.IsCacheEnabled() {
		return
	}

	shard := cm.getShard(key)
	now := time.Now().UnixNano()
	size := estimateSize(value)

	entry := &cacheEntry{
		data:       value,
		timestamp:  now,
		lastAccess: now,
		hits:       0,
		size:       int32(size),
		frequency:  1,
	}

	// Enhanced eviction logic with memory pressure consideration
	maxSize := int64(1000) // default
	if cm.config != nil {
		maxSize = int64(cm.config.GetMaxCacheSize())
	}

	shardMaxSize := maxSize / int64(cm.shardCount)
	currentSize := atomic.LoadInt64(&shard.size)

	// Trigger eviction earlier if memory usage is high
	memoryUsage := atomic.LoadInt64(&cm.memoryUsage)
	if currentSize >= shardMaxSize || (currentSize >= shardMaxSize*3/4 && memoryUsage > maxSize*1024) {
		cm.evictLRU(shard)
	}

	shard.data.Store(key, entry)
	atomic.AddInt64(&shard.size, 1)
	atomic.AddInt64(&shard.memory, int64(size))
	atomic.AddInt64(&cm.memoryUsage, int64(size))
}

// ClearCache clears all cached data
func (cm *CacheManager) ClearCache() {
	for _, shard := range cm.shards {
		shard.mu.Lock()
		shard.data.Range(func(key, value any) bool {
			shard.data.Delete(key)
			return true
		})
		atomic.StoreInt64(&shard.size, 0)
		atomic.StoreInt64(&shard.memory, 0)
		shard.mu.Unlock()
	}
	atomic.StoreInt64(&cm.memoryUsage, 0)
}

// GetStats returns cache statistics
func (cm *CacheManager) GetStats() CacheStats {
	totalSize := int64(0)
	totalMemory := int64(0)

	for _, shard := range cm.shards {
		totalSize += atomic.LoadInt64(&shard.size)
		totalMemory += atomic.LoadInt64(&shard.memory)
	}

	hits := atomic.LoadInt64(&cm.hitCount)
	misses := atomic.LoadInt64(&cm.missCount)
	total := hits + misses

	var hitRatio float64
	if total > 0 {
		hitRatio = float64(hits) / float64(total) * 100.0 // Convert to percentage
	}

	return CacheStats{
		Size:      totalSize,
		Memory:    totalMemory,
		HitCount:  hits,
		MissCount: misses,
		HitRatio:  hitRatio,
		Evictions: atomic.LoadInt64(&cm.evictions),
	}
}

// evictLRU evicts the least recently used entry from a shard
func (cm *CacheManager) evictLRU(shard *cacheShard) {
	shard.mu.Lock()
	defer shard.mu.Unlock()

	var oldestKey any
	var oldestTime int64
	var oldestEntry *cacheEntry

	shard.data.Range(func(key, value any) bool {
		entry := value.(*cacheEntry)
		lastAccess := atomic.LoadInt64(&entry.lastAccess)
		if oldestKey == nil || lastAccess < oldestTime {
			oldestKey = key
			oldestTime = lastAccess
			oldestEntry = entry
		}
		return true
	})

	if oldestKey != nil {
		shard.data.Delete(oldestKey)
		atomic.AddInt64(&shard.size, -1)
		atomic.AddInt64(&shard.memory, -int64(oldestEntry.size))
		atomic.AddInt64(&cm.evictions, 1)
	}
}

// fnv1aHash implements FNV-1a hash algorithm for string keys
func fnv1aHash(key string) uint64 {
	const (
		offset64 = 14695981039346656037
		prime64  = 1099511628211
	)

	hash := uint64(offset64)
	for i := 0; i < len(key); i++ {
		hash ^= uint64(key[i])
		hash *= prime64
	}
	return hash
}

// CleanExpiredCache removes expired entries from cache
func (cm *CacheManager) CleanExpiredCache() {
	if cm.config == nil || cm.config.GetCacheTTL() <= 0 {
		return
	}

	now := time.Now()
	ttl := cm.config.GetCacheTTL()
	for _, shard := range cm.shards {
		shard.data.Range(func(key, value any) bool {
			entry := value.(*cacheEntry)
			creationTime := time.Unix(0, atomic.LoadInt64(&entry.timestamp))
			if now.Sub(creationTime) > ttl {
				shard.data.Delete(key)
				atomic.AddInt64(&shard.size, -1)
				atomic.AddInt64(&shard.memory, -int64(entry.size))
			}
			return true
		})
	}
}

// GetCacheSize returns the total number of cached entries
func (cm *CacheManager) GetCacheSize() int64 {
	totalSize := int64(0)
	for _, shard := range cm.shards {
		totalSize += atomic.LoadInt64(&shard.size)
	}
	return totalSize
}

// SecureHash creates a secure hash for cache keys using SHA-256
func (cm *CacheManager) SecureHash(input string) string {
	// Use SHA-256 for cryptographically secure hashing
	hash := sha256.Sum256([]byte(input))
	return fmt.Sprintf("%x", hash[:16]) // Use first 16 bytes for performance
}

// estimateSize estimates the memory size of a value with optimized performance
func estimateSize(value any) int {
	if value == nil {
		return 8 // pointer size
	}

	switch v := value.(type) {
	case string:
		return len(v) + 16 // string header + data
	case []byte:
		return len(v) + 24 // slice header + data
	case int, int64, float64:
		return 8
	case int32, float32:
		return 4
	case bool:
		return 1
	case []any:
		// Optimized: use sampling for large slices to avoid performance issues
		size := 24 // slice header
		length := len(v)
		if length == 0 {
			return size
		}
		if length > 100 {
			// Sample-based estimation for large slices
			sampleSize := min(10, length)
			totalSample := 0
			step := length / sampleSize
			for i := 0; i < sampleSize; i++ {
				totalSample += estimateSize(v[i*step])
			}
			avgItemSize := totalSample / sampleSize
			return size + (length * avgItemSize)
		}
		// Full calculation for small slices
		for _, item := range v {
			size += estimateSize(item)
		}
		return size
	case map[string]any:
		// Optimized: use sampling for large maps
		size := 48 // map header estimate
		length := len(v)
		if length == 0 {
			return size
		}
		if length > 50 {
			// Sample-based estimation for large maps
			sampleSize := min(10, length)
			totalSample := 0
			count := 0
			for k, val := range v {
				if count >= sampleSize {
					break
				}
				totalSample += len(k) + 16 + estimateSize(val)
				count++
			}
			avgItemSize := totalSample / sampleSize
			return size + (length * avgItemSize)
		}
		// Full calculation for small maps
		for k, val := range v {
			size += len(k) + 16 + estimateSize(val)
		}
		return size
	default:
		// Use reflection for complex types
		return int(reflect.TypeOf(value).Size())
	}
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// CacheStats represents cache statistics
type CacheStats struct {
	Size      int64   `json:"size"`
	Memory    int64   `json:"memory"`
	HitCount  int64   `json:"hit_count"`
	MissCount int64   `json:"miss_count"`
	HitRatio  float64 `json:"hit_ratio"`
	Evictions int64   `json:"evictions"`
}
