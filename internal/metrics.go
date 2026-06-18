package internal

import (
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

// MetricsCollector collects and provides performance metrics for the JSON processor
type MetricsCollector struct {
	totalOperations      int64
	successfulOps        int64
	failedOps            int64
	cacheHits            int64
	cacheMisses          int64
	totalProcessingTime  int64
	maxProcessingTime    int64
	minProcessingTime    int64
	totalMemoryAllocated int64
	peakMemoryUsage      int64
	// cumulativeMemoryUsage tracks total memory allocated (never decreases).
	// Exposed as CurrentMemoryUsage in the Metrics struct for API compatibility.
	cumulativeMemoryUsage int64
	activeConcurrentOps   int64
	maxConcurrentOps      int64
	errorsByType          sync.Map
	startTime             time.Time
	// errorsMu protects errorsByType during Reset to prevent data race
	// Uses RWMutex so RecordError (RLock) and Reset (Lock) are properly synchronized
	errorsMu sync.RWMutex
	// Cached runtime memory stats to avoid STW on every GetMetrics call.
	// Refreshed lazily in maybeRefreshMemStats with a minimum interval.
	lastMemStats   atomic.Int64 // unix nano of last refresh
	cachedMemStats atomic.Pointer[runtime.MemStats]
}

// NewMetricsCollector creates a new metrics collector
func NewMetricsCollector() *MetricsCollector {
	return &MetricsCollector{
		errorsByType:      sync.Map{},
		startTime:         time.Now(),
		minProcessingTime: -1, // -1 means not yet recorded
	}
}

// memStatsRefreshInterval is the minimum interval between runtime.ReadMemStats calls.
// This avoids frequent STW pauses while still providing reasonably fresh data.
const memStatsRefreshInterval = int64(5 * time.Second)

// RecordOperation records a completed operation
func (mc *MetricsCollector) RecordOperation(duration time.Duration, success bool, memoryUsed int64) {
	atomic.AddInt64(&mc.totalOperations, 1)

	if success {
		atomic.AddInt64(&mc.successfulOps, 1)
	} else {
		atomic.AddInt64(&mc.failedOps, 1)
	}

	durationNs := duration.Nanoseconds()
	if durationNs > 0 {
		atomic.AddInt64(&mc.totalProcessingTime, durationNs)
		updateMax(&mc.maxProcessingTime, durationNs)
	}
	// Always update min (including zero-duration ops) since initial value is 0
	updateMin(&mc.minProcessingTime, durationNs)

	if memoryUsed > 0 {
		atomic.AddInt64(&mc.totalMemoryAllocated, memoryUsed)
		newUsage := atomic.AddInt64(&mc.cumulativeMemoryUsage, memoryUsed)
		updateMax(&mc.peakMemoryUsage, newUsage)
	}

	// Lazily refresh cached memory stats to avoid STW in GetMetrics
	mc.maybeRefreshMemStats()
}

// maybeRefreshMemStats refreshes the cached runtime.MemStats if enough time has
// elapsed since the last refresh. Uses CAS to ensure only one goroutine performs
// the refresh, avoiding duplicate STW pauses.
func (mc *MetricsCollector) maybeRefreshMemStats() {
	now := time.Now().UnixNano()
	last := mc.lastMemStats.Load()
	if now-last < memStatsRefreshInterval {
		return
	}
	if !mc.lastMemStats.CompareAndSwap(last, now) {
		return // another goroutine is already refreshing
	}
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	mc.cachedMemStats.Store(&ms)
}

// RecordCacheHit records a cache hit
func (mc *MetricsCollector) RecordCacheHit() {
	atomic.AddInt64(&mc.cacheHits, 1)
}

// RecordCacheMiss records a cache miss
func (mc *MetricsCollector) RecordCacheMiss() {
	atomic.AddInt64(&mc.cacheMisses, 1)
}

// StartConcurrentOperation records the start of a concurrent operation
func (mc *MetricsCollector) StartConcurrentOperation() {
	current := atomic.AddInt64(&mc.activeConcurrentOps, 1)
	updateMax(&mc.maxConcurrentOps, current)
}

// EndConcurrentOperation records the end of a concurrent operation
func (mc *MetricsCollector) EndConcurrentOperation() {
	atomic.AddInt64(&mc.activeConcurrentOps, -1)
}

// RecordError records an error by type.
// Uses RLock to synchronize with Reset() which replaces the entire map.
func (mc *MetricsCollector) RecordError(errorType string) {
	mc.errorsMu.RLock()
	defer mc.errorsMu.RUnlock()
	actual, _ := mc.errorsByType.LoadOrStore(errorType, new(int64))
	counter := actual.(*int64)
	atomic.AddInt64(counter, 1)
}

// GetMetrics returns current metrics with runtime stats
func (mc *MetricsCollector) GetMetrics() Metrics {
	totalOps := atomic.LoadInt64(&mc.totalOperations)
	totalTime := atomic.LoadInt64(&mc.totalProcessingTime)

	var avgProcessingTime time.Duration
	if totalOps > 0 {
		avgProcessingTime = time.Duration(totalTime / totalOps)
	}

	// Acquire read lock for consistent snapshot of errorsByType.
	// Reset() replaces the entire sync.Map under write lock, so without
	// this we could iterate over a stale/partial map during a reset.
	mc.errorsMu.RLock()
	errorsByType := make(map[string]int64)
	mc.errorsByType.Range(func(key, value any) bool {
		if k, ok := key.(string); ok {
			if v, ok := value.(*int64); ok {
				errorsByType[k] = atomic.LoadInt64(v)
			}
		}
		return true
	})
	// Read startTime under the same RLock that guards Reset()'s write to it.
	// time.Time is a multi-word struct; reading it without synchronization races
	// with Reset()'s assignment and can produce a torn read (DATA RACE).
	uptime := time.Since(mc.startTime)
	mc.errorsMu.RUnlock()

	metrics := Metrics{
		TotalOperations:      totalOps,
		SuccessfulOps:        atomic.LoadInt64(&mc.successfulOps),
		FailedOps:            atomic.LoadInt64(&mc.failedOps),
		CacheHits:            atomic.LoadInt64(&mc.cacheHits),
		CacheMisses:          atomic.LoadInt64(&mc.cacheMisses),
		TotalProcessingTime:  time.Duration(totalTime),
		AvgProcessingTime:    avgProcessingTime,
		MaxProcessingTime:    time.Duration(atomic.LoadInt64(&mc.maxProcessingTime)),
		MinProcessingTime:    minProcessingTimeToDuration(atomic.LoadInt64(&mc.minProcessingTime)),
		TotalMemoryAllocated: atomic.LoadInt64(&mc.totalMemoryAllocated),
		PeakMemoryUsage:      atomic.LoadInt64(&mc.peakMemoryUsage),
		CurrentMemoryUsage:   atomic.LoadInt64(&mc.cumulativeMemoryUsage),
		ActiveConcurrentOps:  atomic.LoadInt64(&mc.activeConcurrentOps),
		MaxConcurrentOps:     atomic.LoadInt64(&mc.maxConcurrentOps),
		Uptime:               uptime,
		ErrorsByType:         errorsByType,
	}

	// Use cached memory stats instead of triggering STW with runtime.ReadMemStats.
	// Stats are refreshed lazily in RecordOperation at most once every 5 seconds.
	if p := mc.cachedMemStats.Load(); p != nil {
		metrics.RuntimeMemStats = *p
	}
	return metrics
}

// Reset resets all metrics
func (mc *MetricsCollector) Reset() {
	atomic.StoreInt64(&mc.totalOperations, 0)
	atomic.StoreInt64(&mc.successfulOps, 0)
	atomic.StoreInt64(&mc.failedOps, 0)
	atomic.StoreInt64(&mc.cacheHits, 0)
	atomic.StoreInt64(&mc.cacheMisses, 0)
	atomic.StoreInt64(&mc.totalProcessingTime, 0)
	atomic.StoreInt64(&mc.maxProcessingTime, 0)
	atomic.StoreInt64(&mc.minProcessingTime, -1) // Reset to not-yet-recorded
	atomic.StoreInt64(&mc.totalMemoryAllocated, 0)
	atomic.StoreInt64(&mc.peakMemoryUsage, 0)
	atomic.StoreInt64(&mc.cumulativeMemoryUsage, 0)
	atomic.StoreInt64(&mc.activeConcurrentOps, 0)
	atomic.StoreInt64(&mc.maxConcurrentOps, 0)
	// Replace errorsByType and startTime atomically. GetMetrics() reads both
	// under errorsMu.RLock(), so the writes must occur under the matching Lock
	// to avoid a torn read of the multi-word time.Time (DATA RACE).
	mc.errorsMu.Lock()
	mc.errorsByType = sync.Map{}
	mc.startTime = time.Now()
	mc.errorsMu.Unlock()
}

// GetSummary returns a formatted summary of metrics
func (mc *MetricsCollector) GetSummary() string {
	metrics := mc.GetMetrics()

	return fmt.Sprintf(`Metrics Summary:
  Operations: %d total (%d successful, %d failed)
  Cache: %d hits, %d misses (%.2f%% hit rate)
  Performance: avg %v, max %v, min %v
  Memory: %d bytes allocated, %d peak usage
  Concurrency: %d active, %d max concurrent
  Uptime: %v`,
		metrics.TotalOperations,
		metrics.SuccessfulOps,
		metrics.FailedOps,
		metrics.CacheHits,
		metrics.CacheMisses,
		getCacheHitRate(metrics.CacheHits, metrics.CacheMisses),
		metrics.AvgProcessingTime,
		metrics.MaxProcessingTime,
		metrics.MinProcessingTime,
		metrics.TotalMemoryAllocated,
		metrics.PeakMemoryUsage,
		metrics.ActiveConcurrentOps,
		metrics.MaxConcurrentOps,
		metrics.Uptime,
	)
}

// getCacheHitRate calculates cache hit rate
func getCacheHitRate(hits, misses int64) float64 {
	total := hits + misses
	if total == 0 {
		return 0.0
	}
	return float64(hits) / float64(total) * 100.0
}

// Metrics represents collected performance metrics
type Metrics struct {
	// Operation metrics
	TotalOperations int64 `json:"total_operations"`
	SuccessfulOps   int64 `json:"successful_ops"`
	FailedOps       int64 `json:"failed_ops"`
	CacheHits       int64 `json:"cache_hits"`
	CacheMisses     int64 `json:"cache_misses"`

	// Performance metrics
	TotalProcessingTime time.Duration `json:"total_processing_time"`
	AvgProcessingTime   time.Duration `json:"avg_processing_time"`
	MaxProcessingTime   time.Duration `json:"max_processing_time"`
	MinProcessingTime   time.Duration `json:"min_processing_time"`

	// Memory metrics
	TotalMemoryAllocated int64 `json:"total_memory_allocated"`
	PeakMemoryUsage      int64 `json:"peak_memory_usage"`
	CurrentMemoryUsage   int64 `json:"current_memory_usage"`

	// Concurrency metrics
	ActiveConcurrentOps int64 `json:"active_concurrent_ops"`
	MaxConcurrentOps    int64 `json:"max_concurrent_ops"`

	// Runtime metrics
	RuntimeMemStats runtime.MemStats `json:"runtime_mem_stats"`
	Uptime          time.Duration    `json:"uptime"`
	ErrorsByType    map[string]int64 `json:"errors_by_type"`
}

// minProcessingTimeToDuration converts the stored min value to time.Duration.
// Returns 0 if no operations have been recorded yet (sentinel: -1).
func minProcessingTimeToDuration(val int64) time.Duration {
	if val < 0 {
		return 0
	}
	return time.Duration(val)
}

// updateMax atomically updates target to value if value is greater
func updateMax(target *int64, value int64) {
	for {
		current := atomic.LoadInt64(target)
		if value <= current || atomic.CompareAndSwapInt64(target, current, value) {
			return
		}
	}
}

// updateMin atomically updates target to value if value is smaller.
// Handles sentinel value -1 (meaning "not yet recorded") by always updating.
func updateMin(target *int64, value int64) {
	for {
		current := atomic.LoadInt64(target)
		if current < 0 || value >= current || atomic.CompareAndSwapInt64(target, current, value) {
			return
		}
	}
}
