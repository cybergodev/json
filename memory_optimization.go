package json

import (
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

// MemoryOptimizer provides memory optimization strategies
type MemoryOptimizer struct {
	// Memory pressure tracking
	lastGC           int64
	gcInterval       time.Duration
	memoryThreshold  int64
	forceGCThreshold int64

	// Pool management
	pools   []PoolCleaner
	poolsMu sync.RWMutex

	// Statistics
	gcCount         int64
	lastMemoryCheck int64
}

// PoolCleaner interface for pools that can be cleaned
type PoolCleaner interface {
	Clean()
}

// NewMemoryOptimizer creates a new memory optimizer
func NewMemoryOptimizer() *MemoryOptimizer {
	return &MemoryOptimizer{
		gcInterval:       30 * time.Second,
		memoryThreshold:  100 * 1024 * 1024, // 100MB
		forceGCThreshold: 500 * 1024 * 1024, // 500MB
		pools:            make([]PoolCleaner, 0),
	}
}

// RegisterPool registers a pool for cleanup
func (mo *MemoryOptimizer) RegisterPool(pool PoolCleaner) {
	mo.poolsMu.Lock()
	defer mo.poolsMu.Unlock()
	mo.pools = append(mo.pools, pool)
}

// CheckMemoryPressure checks if system is under memory pressure
func (mo *MemoryOptimizer) CheckMemoryPressure() bool {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	currentMemory := int64(m.Alloc)
	atomic.StoreInt64(&mo.lastMemoryCheck, time.Now().Unix())

	return currentMemory > mo.memoryThreshold
}

// OptimizeMemory performs memory optimization
func (mo *MemoryOptimizer) OptimizeMemory(force bool) {
	now := time.Now().Unix()
	lastGC := atomic.LoadInt64(&mo.lastGC)

	// Check if we should run GC
	shouldGC := force || (now-lastGC > int64(mo.gcInterval.Seconds()))

	if shouldGC {
		// Clean pools first
		mo.cleanPools()

		// Check memory pressure
		var m runtime.MemStats
		runtime.ReadMemStats(&m)

		// Force GC if memory is high
		if force || int64(m.Alloc) > mo.forceGCThreshold {
			runtime.GC()
			atomic.StoreInt64(&mo.lastGC, now)
			atomic.AddInt64(&mo.gcCount, 1)
		}
	}
}

// cleanPools cleans all registered pools
func (mo *MemoryOptimizer) cleanPools() {
	mo.poolsMu.RLock()
	defer mo.poolsMu.RUnlock()

	for _, pool := range mo.pools {
		if pool != nil {
			pool.Clean()
		}
	}
}

// GetStats returns memory optimization statistics
func (mo *MemoryOptimizer) GetStats() MemoryOptimizerStats {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	return MemoryOptimizerStats{
		CurrentMemory:   int64(m.Alloc),
		TotalAllocated:  int64(m.TotalAlloc),
		SystemMemory:    int64(m.Sys),
		GCCount:         atomic.LoadInt64(&mo.gcCount),
		LastGC:          atomic.LoadInt64(&mo.lastGC),
		NumGoroutines:   int64(runtime.NumGoroutine()),
		MemoryThreshold: mo.memoryThreshold,
	}
}

// MemoryOptimizerStats represents memory optimizer statistics
type MemoryOptimizerStats struct {
	CurrentMemory   int64
	TotalAllocated  int64
	SystemMemory    int64
	GCCount         int64
	LastGC          int64
	NumGoroutines   int64
	MemoryThreshold int64
}

// SmartCache implements a memory-aware cache
type SmartCache struct {
	cache           map[string]any
	mu              sync.RWMutex
	maxSize         int
	currentSize     int
	memoryOptimizer *MemoryOptimizer
}

// NewSmartCache creates a new smart cache
func NewSmartCache(maxSize int, optimizer *MemoryOptimizer) *SmartCache {
	sc := &SmartCache{
		cache:           make(map[string]any, maxSize),
		maxSize:         maxSize,
		memoryOptimizer: optimizer,
	}

	if optimizer != nil {
		optimizer.RegisterPool(sc)
	}

	return sc
}

// Get retrieves a value from cache
func (sc *SmartCache) Get(key string) (any, bool) {
	sc.mu.RLock()
	defer sc.mu.RUnlock()

	value, ok := sc.cache[key]
	return value, ok
}

// Set stores a value in cache
func (sc *SmartCache) Set(key string, value any) {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	// Check if we need to evict
	if sc.currentSize >= sc.maxSize {
		// Check memory pressure
		if sc.memoryOptimizer != nil && sc.memoryOptimizer.CheckMemoryPressure() {
			// Under memory pressure, clear more aggressively
			sc.clearHalf()
		} else {
			// Normal eviction
			sc.evictOne()
		}
	}

	sc.cache[key] = value
	sc.currentSize++
}

// Clean implements PoolCleaner interface
func (sc *SmartCache) Clean() {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	// Clear half of the cache
	sc.clearHalf()
}

// clearHalf clears half of the cache entries
func (sc *SmartCache) clearHalf() {
	targetSize := sc.maxSize / 2
	count := 0

	for key := range sc.cache {
		if count >= targetSize {
			break
		}
		delete(sc.cache, key)
		count++
	}

	sc.currentSize = len(sc.cache)
}

// evictOne evicts one entry from cache
func (sc *SmartCache) evictOne() {
	// Simple eviction: remove first entry
	for key := range sc.cache {
		delete(sc.cache, key)
		sc.currentSize--
		break
	}
}

// Clear clears all cache entries
func (sc *SmartCache) Clear() {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	sc.cache = make(map[string]any, sc.maxSize)
	sc.currentSize = 0
}
