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
	enabled bool // Flag to enable/disable metrics collection

	// Core metrics (atomic for thread safety)
	totalOperations int64
	successfulOps   int64
	failedOps       int64
	cacheHits       int64
	cacheMisses     int64

	// Performance metrics (atomic)
	totalProcessingTime int64 // nanoseconds
	maxProcessingTime   int64 // nanoseconds
	minProcessingTime   int64 // nanoseconds

	// Memory metrics (atomic)
	totalMemoryAllocated int64
	peakMemoryUsage      int64
	currentMemoryUsage   int64

	// Concurrency metrics (atomic)
	activeConcurrentOps int64
	maxConcurrentOps    int64

	// Error tracking (thread-safe)
	errorsByType sync.Map // map[string]int64

	// Timing
	startTime time.Time
}

// NewMetricsCollector creates a new metrics collector with conditional collection
func NewMetricsCollector(enabled bool) *MetricsCollector {
	return &MetricsCollector{
		enabled:           enabled,
		errorsByType:      sync.Map{},
		startTime:         time.Now(),
		minProcessingTime: int64(^uint64(0) >> 1), // Max int64 value
	}
}

// RecordOperation records a completed operation only if metrics are enabled
func (mc *MetricsCollector) RecordOperation(duration time.Duration, success bool, memoryUsed int64) {
	if !mc.enabled {
		return
	}

	atomic.AddInt64(&mc.totalOperations, 1)

	if success {
		atomic.AddInt64(&mc.successfulOps, 1)
	} else {
		atomic.AddInt64(&mc.failedOps, 1)
	}

	// Update timing metrics
	durationNs := duration.Nanoseconds()
	atomic.AddInt64(&mc.totalProcessingTime, durationNs)

	// Update max processing time
	for {
		current := atomic.LoadInt64(&mc.maxProcessingTime)
		if durationNs <= current || atomic.CompareAndSwapInt64(&mc.maxProcessingTime, current, durationNs) {
			break
		}
	}

	// Update min processing time
	for {
		current := atomic.LoadInt64(&mc.minProcessingTime)
		if durationNs >= current || atomic.CompareAndSwapInt64(&mc.minProcessingTime, current, durationNs) {
			break
		}
	}

	// Update memory metrics
	if memoryUsed > 0 {
		atomic.AddInt64(&mc.totalMemoryAllocated, memoryUsed)
		atomic.AddInt64(&mc.currentMemoryUsage, memoryUsed)

		// Update peak memory usage
		for {
			current := atomic.LoadInt64(&mc.peakMemoryUsage)
			newUsage := atomic.LoadInt64(&mc.currentMemoryUsage)
			if newUsage <= current || atomic.CompareAndSwapInt64(&mc.peakMemoryUsage, current, newUsage) {
				break
			}
		}
	}
}

// RecordCacheHit records a cache hit
func (mc *MetricsCollector) RecordCacheHit() {
	if mc.enabled {
		atomic.AddInt64(&mc.cacheHits, 1)
	}
}

// RecordCacheMiss records a cache miss
func (mc *MetricsCollector) RecordCacheMiss() {
	if mc.enabled {
		atomic.AddInt64(&mc.cacheMisses, 1)
	}
}

// UpdateMemoryUsage updates memory usage metrics
func (mc *MetricsCollector) UpdateMemoryUsage(current int64) {
	if !mc.enabled {
		return
	}

	atomic.StoreInt64(&mc.currentMemoryUsage, current)

	// Update peak memory usage
	for {
		peak := atomic.LoadInt64(&mc.peakMemoryUsage)
		if current <= peak || atomic.CompareAndSwapInt64(&mc.peakMemoryUsage, peak, current) {
			break
		}
	}
}

// StartConcurrentOperation records the start of a concurrent operation
func (mc *MetricsCollector) StartConcurrentOperation() {
	if !mc.enabled {
		return
	}

	current := atomic.AddInt64(&mc.activeConcurrentOps, 1)

	// Update max concurrent operations
	for {
		maxInt := atomic.LoadInt64(&mc.maxConcurrentOps)
		if current <= maxInt || atomic.CompareAndSwapInt64(&mc.maxConcurrentOps, maxInt, current) {
			break
		}
	}
}

// EndConcurrentOperation records the end of a concurrent operation
func (mc *MetricsCollector) EndConcurrentOperation() {
	if mc.enabled {
		atomic.AddInt64(&mc.activeConcurrentOps, -1)
	}
}

// RecordError records an error by type (thread-safe)
func (mc *MetricsCollector) RecordError(errorType string) {
	if !mc.enabled {
		return
	}

	// Use sync.Map for thread-safe error tracking
	if value, ok := mc.errorsByType.Load(errorType); ok {
		if count, ok := value.(int64); ok {
			mc.errorsByType.Store(errorType, count+1)
		}
	} else {
		mc.errorsByType.Store(errorType, int64(1))
	}
}

// GetMetrics returns current metrics
func (mc *MetricsCollector) GetMetrics() Metrics {
	if !mc.enabled {
		return Metrics{
			Uptime: time.Since(mc.startTime),
		}
	}

	totalOps := atomic.LoadInt64(&mc.totalOperations)
	totalTime := atomic.LoadInt64(&mc.totalProcessingTime)

	var avgProcessingTime time.Duration
	if totalOps > 0 {
		avgProcessingTime = time.Duration(totalTime / totalOps)
	}

	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	return Metrics{
		// Operation metrics
		TotalOperations: totalOps,
		SuccessfulOps:   atomic.LoadInt64(&mc.successfulOps),
		FailedOps:       atomic.LoadInt64(&mc.failedOps),
		CacheHits:       atomic.LoadInt64(&mc.cacheHits),
		CacheMisses:     atomic.LoadInt64(&mc.cacheMisses),

		// Performance metrics
		TotalProcessingTime: time.Duration(totalTime),
		AvgProcessingTime:   avgProcessingTime,
		MaxProcessingTime:   time.Duration(atomic.LoadInt64(&mc.maxProcessingTime)),
		MinProcessingTime:   time.Duration(atomic.LoadInt64(&mc.minProcessingTime)),

		// Memory metrics
		TotalMemoryAllocated: atomic.LoadInt64(&mc.totalMemoryAllocated),
		PeakMemoryUsage:      atomic.LoadInt64(&mc.peakMemoryUsage),
		CurrentMemoryUsage:   atomic.LoadInt64(&mc.currentMemoryUsage),

		// Concurrency metrics
		ActiveConcurrentOps: atomic.LoadInt64(&mc.activeConcurrentOps),
		MaxConcurrentOps:    atomic.LoadInt64(&mc.maxConcurrentOps),

		// Runtime metrics
		RuntimeMemStats: memStats,
		Uptime:          time.Since(mc.startTime),
		ErrorsByType:    copyErrorMapFromSync(&mc.errorsByType),
	}
}

// Reset resets all metrics
func (mc *MetricsCollector) Reset() {
	if !mc.enabled {
		return
	}

	atomic.StoreInt64(&mc.totalOperations, 0)
	atomic.StoreInt64(&mc.successfulOps, 0)
	atomic.StoreInt64(&mc.failedOps, 0)
	atomic.StoreInt64(&mc.cacheHits, 0)
	atomic.StoreInt64(&mc.cacheMisses, 0)
	atomic.StoreInt64(&mc.totalProcessingTime, 0)
	atomic.StoreInt64(&mc.maxProcessingTime, 0)
	atomic.StoreInt64(&mc.minProcessingTime, int64(^uint64(0)>>1))
	atomic.StoreInt64(&mc.totalMemoryAllocated, 0)
	atomic.StoreInt64(&mc.peakMemoryUsage, 0)
	atomic.StoreInt64(&mc.currentMemoryUsage, 0)
	atomic.StoreInt64(&mc.activeConcurrentOps, 0)
	atomic.StoreInt64(&mc.maxConcurrentOps, 0)

	mc.errorsByType = sync.Map{}
	mc.startTime = time.Now()
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

// copyErrorMapFromSync creates a copy of the error map from sync.Map for thread safety
func copyErrorMapFromSync(original *sync.Map) map[string]int64 {
	cp := make(map[string]int64)
	original.Range(func(key, value interface{}) bool {
		if k, ok := key.(string); ok {
			if v, ok := value.(int64); ok {
				cp[k] = v
			}
		}
		return true
	})
	return cp
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

// GetBasicMetrics returns essential metrics for performance monitoring
func (mc *MetricsCollector) GetBasicMetrics() BasicMetrics {
	if !mc.enabled {
		return BasicMetrics{
			Uptime:  time.Since(mc.startTime),
			Enabled: false,
		}
	}

	totalOps := atomic.LoadInt64(&mc.totalOperations)
	totalTime := atomic.LoadInt64(&mc.totalProcessingTime)

	var avgProcessingTime time.Duration
	if totalOps > 0 {
		avgProcessingTime = time.Duration(totalTime / totalOps)
	}

	return BasicMetrics{
		TotalOperations:    totalOps,
		SuccessfulOps:      atomic.LoadInt64(&mc.successfulOps),
		FailedOps:          atomic.LoadInt64(&mc.failedOps),
		CacheHits:          atomic.LoadInt64(&mc.cacheHits),
		CacheMisses:        atomic.LoadInt64(&mc.cacheMisses),
		AvgProcessingTime:  avgProcessingTime,
		MaxProcessingTime:  time.Duration(atomic.LoadInt64(&mc.maxProcessingTime)),
		CurrentMemoryUsage: atomic.LoadInt64(&mc.currentMemoryUsage),
		PeakMemoryUsage:    atomic.LoadInt64(&mc.peakMemoryUsage),
		Uptime:             time.Since(mc.startTime),
		Enabled:            mc.enabled,
	}
}

// BasicMetrics represents essential performance metrics
type BasicMetrics struct {
	TotalOperations    int64         `json:"total_operations"`
	SuccessfulOps      int64         `json:"successful_ops"`
	FailedOps          int64         `json:"failed_ops"`
	CacheHits          int64         `json:"cache_hits"`
	CacheMisses        int64         `json:"cache_misses"`
	AvgProcessingTime  time.Duration `json:"avg_processing_time"`
	MaxProcessingTime  time.Duration `json:"max_processing_time"`
	CurrentMemoryUsage int64         `json:"current_memory_usage"`
	PeakMemoryUsage    int64         `json:"peak_memory_usage"`
	Uptime             time.Duration `json:"uptime"`
	Enabled            bool          `json:"enabled"`
}
