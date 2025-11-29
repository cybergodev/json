package json

import (
	"hash/fnv"
	"strconv"
	"sync"
	"time"
)

// OptimizedCache provides a simplified, high-performance cache implementation
type OptimizedCache struct {
	mu        sync.RWMutex
	items     map[uint64]*optimizedCacheItem
	maxSize   int
	ttl       time.Duration
	hits      int64
	misses    int64
	evictions int64
}

type optimizedCacheItem struct {
	value      any
	expireAt   int64 // Unix nano for faster comparison
	accessTime int64 // Unix nano
}

// NewOptimizedCache creates a new optimized cache
func NewOptimizedCache(maxSize int, ttl time.Duration) *OptimizedCache {
	return &OptimizedCache{
		items:   make(map[uint64]*optimizedCacheItem, maxSize),
		maxSize: maxSize,
		ttl:     ttl,
	}
}

// Get retrieves a value from cache
func (c *OptimizedCache) Get(key uint64) (any, bool) {
	c.mu.RLock()
	item, ok := c.items[key]
	c.mu.RUnlock()

	if !ok {
		c.misses++
		return nil, false
	}

	now := time.Now().UnixNano()
	if now > item.expireAt {
		c.misses++
		// Lazy deletion
		c.mu.Lock()
		delete(c.items, key)
		c.mu.Unlock()
		return nil, false
	}

	// Update access time without lock for better performance
	item.accessTime = now
	c.hits++
	return item.value, true
}

// Set stores a value in cache
func (c *OptimizedCache) Set(key uint64, value any) {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now().UnixNano()

	// Check if we need to evict
	if len(c.items) >= c.maxSize {
		c.evictLocked(now)
	}

	c.items[key] = &optimizedCacheItem{
		value:      value,
		expireAt:   now + c.ttl.Nanoseconds(),
		accessTime: now,
	}
}

// evictLocked evicts items (must be called with lock held)
func (c *OptimizedCache) evictLocked(now int64) {
	// First, remove expired items
	for key, item := range c.items {
		if now > item.expireAt {
			delete(c.items, key)
			c.evictions++
		}
	}

	// If still too full, use LRU
	if len(c.items) >= c.maxSize {
		targetSize := c.maxSize * 3 / 4 // Keep 75%
		toEvict := len(c.items) - targetSize

		// Find oldest items
		type keyTime struct {
			key  uint64
			time int64
		}
		oldest := make([]keyTime, 0, toEvict)

		for key, item := range c.items {
			if len(oldest) < toEvict {
				oldest = append(oldest, keyTime{key, item.accessTime})
			} else {
				// Find the newest in oldest list
				maxIdx := 0
				for i := 1; i < len(oldest); i++ {
					if oldest[i].time > oldest[maxIdx].time {
						maxIdx = i
					}
				}
				// Replace if current is older
				if item.accessTime < oldest[maxIdx].time {
					oldest[maxIdx] = keyTime{key, item.accessTime}
				}
			}
		}

		// Evict oldest items
		for _, kt := range oldest {
			delete(c.items, kt.key)
			c.evictions++
		}
	}
}

// Clear removes all items from cache
func (c *OptimizedCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items = make(map[uint64]*optimizedCacheItem, c.maxSize)
}

// Stats returns cache statistics
func (c *OptimizedCache) Stats() CacheStats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	total := c.hits + c.misses
	hitRatio := 0.0
	if total > 0 {
		hitRatio = float64(c.hits) / float64(total)
	}

	return CacheStats{
		HitCount:    c.hits,
		MissCount:   c.misses,
		HitRatio:    hitRatio,
		Evictions:   c.evictions,
		TotalMemory: int64(len(c.items) * 64), // Rough estimate
	}
}

// createOptimizedCacheKey creates a cache key using FNV hash
func createOptimizedCacheKey(operation, jsonStr, path string) uint64 {
	h := fnv.New64a()
	h.Write([]byte(operation))
	h.Write([]byte(path))

	// For small JSON, hash the entire string
	if len(jsonStr) < 1024 {
		h.Write([]byte(jsonStr))
	} else {
		// For large JSON, hash first and last 512 bytes
		h.Write([]byte(jsonStr[:512]))
		h.Write([]byte(jsonStr[len(jsonStr)-512:]))
		// Also include length
		h.Write([]byte(strconv.Itoa(len(jsonStr))))
	}

	return h.Sum64()
}
