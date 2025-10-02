package json

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode/utf8"

	"github.com/cybergodev/json/internal"
)

// Processor is the main JSON processing engine with thread safety and performance optimization
type Processor struct {
	config             *Config
	cache              *internal.CacheManager
	state              int32 // 0=active, 1=closing, 2=closed
	cleanupOnce        sync.Once
	pathPatterns       *internal.PathPatterns
	resources          *processorResources
	metrics            *processorMetrics
	resourceMonitor    *ResourceMonitor
	concurrencyManager *ConcurrencyManager
	logger             *slog.Logger
}

// processorResources consolidates all resource management
type processorResources struct {
	stringBuilderPool *sync.Pool
	pathSegmentPool   *sync.Pool
	pathParseCache    map[string]*cacheEntry
	pathCacheMutex    sync.RWMutex
	pathCacheSize     int64
	jsonParseCache    map[string]*cacheEntry
	jsonCacheMutex    sync.RWMutex
	jsonCacheSize     int64
	maxCacheEntries   int
	lastPoolReset     int64
	lastMemoryCheck   int64
	memoryPressure    int32
}

// cacheEntry represents a cache entry with access tracking for proper LRU
type cacheEntry struct {
	value      any
	lastAccess int64 // Unix nanoseconds
}

// processorMetrics consolidates all metrics and performance tracking
type processorMetrics struct {
	operationCount       int64
	errorCount           int64
	concurrentOps        int64
	maxConcurrentOps     int64
	totalWaitTime        int64
	lastOperationTime    int64
	operationWindow      int64
	concurrencySemaphore chan struct{}
	collector            *internal.MetricsCollector
}

// New creates a new JSON processor with the given configuration.
// If no configuration is provided, uses default configuration.
func New(config ...*Config) *Processor {
	var cfg *Config
	if len(config) > 0 && config[0] != nil {
		cfg = config[0]
	} else {
		cfg = DefaultConfig()
	}

	if err := ValidateConfig(cfg); err != nil {
		panic(fmt.Sprintf("invalid configuration: %v", err))
	}

	p := &Processor{
		config:             cfg,
		cache:              internal.NewCacheManager(cfg),
		pathPatterns:       internal.NewPathPatterns(),
		resourceMonitor:    NewResourceMonitor(),
		concurrencyManager: NewConcurrencyManager(cfg.MaxConcurrency, 0),
		logger:             slog.Default().With("component", "json-processor"),

		resources: &processorResources{
			stringBuilderPool: &sync.Pool{
				New: func() interface{} {
					sb := &strings.Builder{}
					sb.Grow(256)
					return sb
				},
			},
			pathSegmentPool: &sync.Pool{
				New: func() interface{} {
					return make([]PathSegment, 0, 8)
				},
			},
			pathParseCache:  make(map[string]*cacheEntry, 128),
			jsonParseCache:  make(map[string]*cacheEntry, 64),
			maxCacheEntries: 512,
		},

		metrics: &processorMetrics{
			operationWindow:      0,
			concurrencySemaphore: make(chan struct{}, cfg.MaxConcurrency),
			collector:            internal.NewMetricsCollector(),
		},
	}

	return p
}

// getPathSegments gets a path segments slice from the pool
func (p *Processor) getPathSegments() []PathSegment {
	if p.resources.pathSegmentPool == nil {
		return make([]PathSegment, 0, 8)
	}

	segments := p.resources.pathSegmentPool.Get().([]PathSegment)
	return segments[:0] // Reset length but keep capacity
}

// putPathSegments returns a path segments slice to the pool
func (p *Processor) putPathSegments(segments []PathSegment) {
	if p.resources.pathSegmentPool != nil && segments != nil && !p.isClosing() {
		if cap(segments) <= 128 && cap(segments) >= 8 {
			segments = segments[:0]
			p.resources.pathSegmentPool.Put(segments)
		}
	}
}

// performMaintenance performs periodic maintenance tasks
func (p *Processor) performMaintenance() {
	if p.isClosing() {
		return // Skip maintenance if closing
	}

	// Clean expired cache entries
	if p.cache != nil {
		p.cache.CleanExpiredCache()
	}

	// Clean parsing caches if they grow too large
	p.cleanParsingCaches()

	// Reset resource pools if they grow too large or under memory pressure
	p.resetResourcePools()

	// Check memory pressure
	p.checkMemoryPressure()

	// Perform leak detection
	if p.resourceMonitor != nil {
		if issues := p.resourceMonitor.CheckForLeaks(); len(issues) > 0 {
			for _, issue := range issues {
				p.logger.Warn("Resource issue detected", "issue", issue)
			}
		}
	}
}

// cleanParsingCaches cleans parsing caches when they grow too large using proper LRU
func (p *Processor) cleanParsingCaches() {
	// Clean path parse cache with proper LRU
	p.resources.pathCacheMutex.Lock()
	if len(p.resources.pathParseCache) > p.resources.maxCacheEntries {
		// Find and keep the most recently accessed entries
		type entryWithKey struct {
			key        string
			entry      *cacheEntry
			lastAccess int64
		}

		entries := make([]entryWithKey, 0, len(p.resources.pathParseCache))
		for k, v := range p.resources.pathParseCache {
			entries = append(entries, entryWithKey{
				key:        k,
				entry:      v,
				lastAccess: atomic.LoadInt64(&v.lastAccess),
			})
		}

		// Sort by last access time (descending)
		for i := 0; i < len(entries)-1; i++ {
			for j := i + 1; j < len(entries); j++ {
				if entries[j].lastAccess > entries[i].lastAccess {
					entries[i], entries[j] = entries[j], entries[i]
				}
			}
		}

		// Keep only the top half
		newCache := make(map[string]*cacheEntry, p.resources.maxCacheEntries/2)
		keepCount := p.resources.maxCacheEntries / 2
		for i := 0; i < keepCount && i < len(entries); i++ {
			newCache[entries[i].key] = entries[i].entry
		}
		p.resources.pathParseCache = newCache
		atomic.StoreInt64(&p.resources.pathCacheSize, int64(len(newCache)))
	}
	p.resources.pathCacheMutex.Unlock()

	// Clean JSON parse cache with proper LRU
	p.resources.jsonCacheMutex.Lock()
	if len(p.resources.jsonParseCache) > p.resources.maxCacheEntries {
		type entryWithKey struct {
			key        string
			entry      *cacheEntry
			lastAccess int64
		}

		entries := make([]entryWithKey, 0, len(p.resources.jsonParseCache))
		for k, v := range p.resources.jsonParseCache {
			entries = append(entries, entryWithKey{
				key:        k,
				entry:      v,
				lastAccess: atomic.LoadInt64(&v.lastAccess),
			})
		}

		// Sort by last access time (descending)
		for i := 0; i < len(entries)-1; i++ {
			for j := i + 1; j < len(entries); j++ {
				if entries[j].lastAccess > entries[i].lastAccess {
					entries[i], entries[j] = entries[j], entries[i]
				}
			}
		}

		// Keep only the top half
		newCache := make(map[string]*cacheEntry, p.resources.maxCacheEntries/2)
		keepCount := p.resources.maxCacheEntries / 2
		for i := 0; i < keepCount && i < len(entries); i++ {
			newCache[entries[i].key] = entries[i].entry
		}
		p.resources.jsonParseCache = newCache
		atomic.StoreInt64(&p.resources.jsonCacheSize, int64(len(newCache)))
	}
	p.resources.jsonCacheMutex.Unlock()
}

// checkMemoryPressure monitors memory usage and adjusts behavior accordingly
func (p *Processor) checkMemoryPressure() {
	opCount := atomic.LoadInt64(&p.metrics.operationCount)
	lastCheck := atomic.LoadInt64(&p.resources.lastMemoryCheck)

	// Check memory pressure every 50,000 operations
	if opCount-lastCheck > 50000 {
		if atomic.CompareAndSwapInt64(&p.resources.lastMemoryCheck, lastCheck, opCount) {
			// Simple memory pressure detection based on cache sizes
			pathCacheSize := atomic.LoadInt64(&p.resources.pathCacheSize)
			jsonCacheSize := atomic.LoadInt64(&p.resources.jsonCacheSize)

			if pathCacheSize+jsonCacheSize > int64(p.resources.maxCacheEntries) {
				atomic.StoreInt32(&p.resources.memoryPressure, 1)
				// Trigger more aggressive cleanup
				p.cleanParsingCaches()
			} else {
				atomic.StoreInt32(&p.resources.memoryPressure, 0)
			}
		}
	}
}

// resetResourcePools resets resource pools to prevent memory bloat (thread-safe)
func (p *Processor) resetResourcePools() {
	opCount := atomic.LoadInt64(&p.metrics.operationCount)
	memoryPressure := atomic.LoadInt32(&p.resources.memoryPressure)

	// Reset more frequently under memory pressure
	resetInterval := int64(100000)
	if memoryPressure > 0 {
		resetInterval = 50000
	}

	// Only reset if we've processed a significant number of operations
	if opCount > 0 && opCount%resetInterval == 0 {
		lastReset := atomic.LoadInt64(&p.resources.lastPoolReset)
		if atomic.CompareAndSwapInt64(&p.resources.lastPoolReset, lastReset, opCount) {
			if !p.isClosing() {
				p.recreateResourcePools()
			}
		}
	}
}

// recreateResourcePools recreates the resource pools with optimized settings
func (p *Processor) recreateResourcePools() {
	if p.resources.stringBuilderPool != nil {
		p.resources.stringBuilderPool = &sync.Pool{
			New: func() interface{} {
				sb := &strings.Builder{}
				sb.Grow(512) // Optimized initial capacity
				return sb
			},
		}
	}

	if p.resources.pathSegmentPool != nil {
		p.resources.pathSegmentPool = &sync.Pool{
			New: func() interface{} {
				return make([]PathSegment, 0, 16) // Optimized for common path depths
			},
		}
	}
}

// Close closes the processor and cleans up resources
func (p *Processor) Close() error {
	p.cleanupOnce.Do(func() {
		atomic.StoreInt32(&p.state, 1)

		if p.cache != nil {
			p.cache.ClearCache()
		}

		p.resources.pathCacheMutex.Lock()
		for k := range p.resources.pathParseCache {
			delete(p.resources.pathParseCache, k)
		}
		p.resources.pathParseCache = nil
		atomic.StoreInt64(&p.resources.pathCacheSize, 0)
		p.resources.pathCacheMutex.Unlock()

		p.resources.jsonCacheMutex.Lock()
		for k := range p.resources.jsonParseCache {
			delete(p.resources.jsonParseCache, k)
		}
		p.resources.jsonParseCache = nil
		atomic.StoreInt64(&p.resources.jsonCacheSize, 0)
		p.resources.jsonCacheMutex.Unlock()

		p.resources.stringBuilderPool = nil
		p.resources.pathSegmentPool = nil

		atomic.StoreInt32(&p.resources.memoryPressure, 0)
		atomic.StoreInt64(&p.resources.lastMemoryCheck, 0)
		atomic.StoreInt64(&p.resources.lastPoolReset, 0)

		atomic.StoreInt32(&p.state, 2)
	})
	return nil
}

// IsClosed returns true if the processor has been closed
func (p *Processor) IsClosed() bool {
	return atomic.LoadInt32(&p.state) == 2
}

// isClosing returns true if the processor is in the process of closing
func (p *Processor) isClosing() bool {
	state := atomic.LoadInt32(&p.state)
	return state == 1 || state == 2
}

// acquireConcurrencySlot acquires a concurrency slot with timeout
func (p *Processor) acquireConcurrencySlot(timeout time.Duration) error {
	if p.isClosing() {
		return &JsonsError{
			Op:      "acquire_concurrency_slot",
			Message: "processor is closing",
			Err:     ErrProcessorClosed,
		}
	}

	start := time.Now()

	select {
	case p.metrics.concurrencySemaphore <- struct{}{}:
		// Successfully acquired slot
		current := atomic.AddInt64(&p.metrics.concurrentOps, 1)

		// Update max concurrent operations
		for {
			maxInt := atomic.LoadInt64(&p.metrics.maxConcurrentOps)
			if current <= maxInt || atomic.CompareAndSwapInt64(&p.metrics.maxConcurrentOps, maxInt, current) {
				break
			}
		}

		// Record wait time
		waitTime := time.Since(start).Nanoseconds()
		atomic.AddInt64(&p.metrics.totalWaitTime, waitTime)

		return nil

	case <-time.After(timeout):
		return &JsonsError{
			Op:      "acquire_concurrency_slot",
			Message: "timeout waiting for concurrency slot",
			Err:     ErrOperationTimeout,
		}
	}
}

// releaseConcurrencySlot releases a concurrency slot
func (p *Processor) releaseConcurrencySlot() {
	select {
	case <-p.metrics.concurrencySemaphore:
		atomic.AddInt64(&p.metrics.concurrentOps, -1)
	default:
		// Should not happen, but handle gracefully
	}
}

// executeWithConcurrencyControl executes an operation with full concurrency control
func (p *Processor) executeWithConcurrencyControl(
	operation func() error,
	timeout time.Duration,
) error {
	if p.concurrencyManager == nil {
		// Fallback to direct execution if concurrency manager is not available
		return operation()
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	return p.concurrencyManager.ExecuteWithConcurrencyControl(ctx, operation, timeout)
}

// executeOperation executes an operation with metrics tracking and error handling
func (p *Processor) executeOperation(operationName string, operation func() error) error {
	if err := p.checkClosed(); err != nil {
		return err
	}

	// Record operation start
	atomic.AddInt64(&p.metrics.operationCount, 1)
	start := time.Now()

	// Execute with concurrency control
	err := p.executeWithConcurrencyControl(operation, 30*time.Second)

	// Record metrics
	if p.resourceMonitor != nil {
		p.resourceMonitor.RecordOperation(time.Since(start))
	}

	if err != nil {
		atomic.AddInt64(&p.metrics.errorCount, 1)
		if p.logger != nil {
			p.logger.Error("Operation failed",
				"operation", operationName,
				"error", err,
				"duration", time.Since(start),
			)
		}
	}

	return err
}

// GetStats returns processor performance statistics
func (p *Processor) GetStats() Stats {
	cacheStats := p.cache.GetStats()

	return Stats{
		CacheSize:        cacheStats.Size,
		CacheMemory:      cacheStats.Memory,
		MaxCacheSize:     p.config.MaxCacheSize,
		HitCount:         cacheStats.HitCount,
		MissCount:        cacheStats.MissCount,
		HitRatio:         cacheStats.HitRatio,
		CacheTTL:         p.config.CacheTTL,
		CacheEnabled:     p.config.EnableCache,
		IsClosed:         p.IsClosed(),
		MemoryEfficiency: 0.0,
		OperationCount:   atomic.LoadInt64(&p.metrics.operationCount),
		ErrorCount:       atomic.LoadInt64(&p.metrics.errorCount),
	}
}

// getDetailedStats returns detailed performance statistics for internal debugging
func (p *Processor) getDetailedStats() DetailedStats {
	stats := p.GetStats()

	return DetailedStats{
		Stats:          stats,
		state:          atomic.LoadInt32(&p.state),
		configSnapshot: *p.config, // Safe to copy as config is immutable
		resourcePoolStats: ResourcePoolStats{
			StringBuilderPoolActive: p.resources.stringBuilderPool != nil,
			PathSegmentPoolActive:   p.resources.pathSegmentPool != nil,
		},
	}
}

// getMetrics returns comprehensive processor metrics for internal use
func (p *Processor) getMetrics() ProcessorMetrics {
	if p.metrics == nil {
		// Return empty metrics if collector is not initialized
		return ProcessorMetrics{}
	}
	internalMetrics := p.metrics.collector.GetMetrics()
	// Convert internal.Metrics to ProcessorMetrics
	return ProcessorMetrics{
		TotalOperations:       internalMetrics.TotalOperations,
		SuccessfulOperations:  internalMetrics.SuccessfulOps,
		FailedOperations:      internalMetrics.FailedOps,
		SuccessRate:           calculateSuccessRate(internalMetrics.SuccessfulOps, internalMetrics.TotalOperations),
		CacheHits:             internalMetrics.CacheHits,
		CacheMisses:           internalMetrics.CacheMisses,
		CacheHitRate:          calculateHitRatio(internalMetrics.CacheHits, internalMetrics.CacheMisses),
		AverageProcessingTime: internalMetrics.AvgProcessingTime,
		MaxProcessingTime:     internalMetrics.MaxProcessingTime,
		MinProcessingTime:     internalMetrics.MinProcessingTime,
		TotalMemoryAllocated:  internalMetrics.TotalMemoryAllocated,
		PeakMemoryUsage:       internalMetrics.PeakMemoryUsage,
		CurrentMemoryUsage:    internalMetrics.CurrentMemoryUsage,
		ActiveConcurrentOps:   internalMetrics.ActiveConcurrentOps,
		MaxConcurrentOps:      internalMetrics.MaxConcurrentOps,
		runtimeMemStats:       internalMetrics.RuntimeMemStats,
		uptime:                internalMetrics.Uptime,
		errorsByType:          internalMetrics.ErrorsByType,
	}
}

// Helper functions for metric calculations
func calculateSuccessRate(successful, total int64) float64 {
	if total == 0 {
		return 0.0
	}
	return float64(successful) / float64(total) * 100.0
}

func calculateHitRatio(hits, misses int64) float64 {
	total := hits + misses
	if total == 0 {
		return 0.0
	}
	return float64(hits) / float64(total) * 100.0
}

// GetHealthStatus returns the current health status
func (p *Processor) GetHealthStatus() HealthStatus {
	if p.metrics == nil {
		return HealthStatus{
			Timestamp: time.Now(),
			Healthy:   false,
			Checks: map[string]CheckResult{
				"metrics": {
					Healthy: false,
					Message: "Metrics collector not initialized",
				},
			},
		}
	}

	healthChecker := internal.NewHealthChecker(p, p.metrics.collector)
	internalStatus := healthChecker.CheckHealth()

	// Convert internal.HealthStatus to HealthStatus
	checks := make(map[string]CheckResult)
	for name, result := range internalStatus.Checks {
		checks[name] = CheckResult{
			Healthy: result.Healthy,
			Message: result.Message,
		}
	}

	return HealthStatus{
		Timestamp: internalStatus.Timestamp,
		Healthy:   internalStatus.Healthy,
		Checks:    checks,
	}
}

// ClearCache clears all cached data
func (p *Processor) ClearCache() {
	if p.cache != nil {
		p.cache.ClearCache()
	}
}

// WarmupCache pre-loads commonly used paths into cache to improve first-access performance
func (p *Processor) WarmupCache(jsonStr string, paths []string, opts ...*ProcessorOptions) (*WarmupResult, error) {
	if err := p.checkClosed(); err != nil {
		return nil, err
	}

	if !p.config.EnableCache {
		return nil, &JsonsError{
			Op:      "warmup_cache",
			Message: "cache is disabled, cannot warmup cache",
			Err:     ErrCacheDisabled,
		}
	}

	if len(paths) == 0 {
		return &WarmupResult{
			TotalPaths:  0,
			Successful:  0,
			Failed:      0,
			SuccessRate: 100.0,
			FailedPaths: nil,
		}, nil // Nothing to warmup
	}

	// Validate JSON input
	if err := p.validateInput(jsonStr); err != nil {
		return nil, &JsonsError{
			Op:      "warmup_cache",
			Message: "invalid JSON input for cache warmup",
			Err:     err,
		}
	}

	// Prepare options
	options, err := p.prepareOptions(opts...)
	if err != nil {
		return nil, &JsonsError{
			Op:      "warmup_cache",
			Message: "invalid options for cache warmup",
			Err:     err,
		}
	}

	// Track warmup statistics
	successCount := 0
	errorCount := 0
	var lastError error
	var failedPaths []string

	// Preload each path into cache
	for _, path := range paths {
		// Validate path
		if err := p.validatePath(path); err != nil {
			errorCount++
			failedPaths = append(failedPaths, path)
			lastError = &JsonsError{
				Op:      "warmup_cache",
				Path:    path,
				Message: fmt.Sprintf("invalid path '%s' for cache warmup: %v", path, err),
				Err:     err,
			}
			continue
		}

		// Try to get the value (this will cache it if successful)
		_, err := p.Get(jsonStr, path, options)
		if err != nil {
			errorCount++
			failedPaths = append(failedPaths, path)
			lastError = &JsonsError{
				Op:      "warmup_cache",
				Path:    path,
				Message: fmt.Sprintf("failed to warmup path '%s': %v", path, err),
				Err:     err,
			}
		} else {
			successCount++
		}
	}

	// Create warmup result
	successRate := 100.0
	if len(paths) > 0 {
		successRate = float64(successCount) / float64(len(paths)) * 100
	}

	result := &WarmupResult{
		TotalPaths:  len(paths),
		Successful:  successCount,
		Failed:      errorCount,
		SuccessRate: successRate,
		FailedPaths: failedPaths,
	}

	// Return error if all paths failed
	if successCount == 0 && errorCount > 0 {
		return result, &JsonsError{
			Op:      "warmup_cache",
			Message: fmt.Sprintf("cache warmup failed for all %d paths, last error: %v", len(paths), lastError),
			Err:     lastError,
		}
	}

	return result, nil
}

// warmupCacheWithSampleData pre-loads paths using sample JSON data for better cache preparation
func (p *Processor) warmupCacheWithSampleData(sampleData map[string]string, opts ...*ProcessorOptions) (*WarmupResult, error) {
	if err := p.checkClosed(); err != nil {
		return nil, err
	}

	if !p.config.EnableCache {
		return nil, &JsonsError{
			Op:      "warmup_cache_sample",
			Message: "cache is disabled, cannot warmup cache",
			Err:     ErrCacheDisabled,
		}
	}

	if len(sampleData) == 0 {
		return &WarmupResult{
			TotalPaths:  0,
			Successful:  0,
			Failed:      0,
			SuccessRate: 100.0,
			FailedPaths: nil,
		}, nil // Nothing to warmup
	}

	totalPaths := 0
	successCount := 0
	errorCount := 0
	var lastError error
	var allFailedPaths []string

	// Process each JSON sample with its associated paths
	for jsonStr, pathsStr := range sampleData {
		// Parse paths (comma-separated)
		paths := strings.Split(pathsStr, ",")
		for i, path := range paths {
			paths[i] = strings.TrimSpace(path)
		}

		totalPaths += len(paths)

		// Warmup cache for this JSON with these paths
		result, err := p.WarmupCache(jsonStr, paths, opts...)
		if err != nil {
			errorCount += len(paths)
			lastError = err
			// Add all paths as failed if the entire warmup failed
			allFailedPaths = append(allFailedPaths, paths...)
		} else if result != nil {
			successCount += result.Successful
			errorCount += result.Failed
			allFailedPaths = append(allFailedPaths, result.FailedPaths...)
		}
	}

	// Create warmup result
	successRate := 100.0
	if totalPaths > 0 {
		successRate = float64(successCount) / float64(totalPaths) * 100
	}

	result := &WarmupResult{
		TotalPaths:  totalPaths,
		Successful:  successCount,
		Failed:      errorCount,
		SuccessRate: successRate,
		FailedPaths: allFailedPaths,
	}

	// Return error if all operations failed
	if successCount == 0 && errorCount > 0 {
		return result, &JsonsError{
			Op:      "warmup_cache_sample",
			Message: fmt.Sprintf("sample data cache warmup failed for all %d paths, last error: %v", totalPaths, lastError),
			Err:     lastError,
		}
	}

	return result, nil
}

// GetConfig returns a copy of the processor configuration
func (p *Processor) GetConfig() *Config {
	configCopy := *p.config
	return &configCopy
}

// SetLogger sets a custom structured logger for the processor
func (p *Processor) SetLogger(logger *slog.Logger) {
	if logger != nil {
		p.logger = logger.With("component", "json-processor")
	} else {
		p.logger = slog.Default().With("component", "json-processor")
	}
}

// checkClosed returns an error if the processor is closed or closing
func (p *Processor) checkClosed() error {
	state := atomic.LoadInt32(&p.state)
	if state != 0 {
		if state == 1 {
			return &JsonsError{
				Op:      "check_closed",
				Message: "processor is closing",
				Err:     ErrProcessorClosed,
			}
		}
		return &JsonsError{
			Op:      "check_closed",
			Message: "processor is closed",
			Err:     ErrProcessorClosed,
		}
	}
	return nil
}

// incrementOperationCount atomically increments the operation counter with rate limiting
func (p *Processor) incrementOperationCount() {
	atomic.AddInt64(&p.metrics.operationCount, 1)
}

// checkRateLimit checks if the operation rate is within acceptable limits
func (p *Processor) checkRateLimit() error {
	if p.metrics.operationWindow <= 0 {
		return nil // Rate limiting disabled
	}

	now := time.Now().UnixNano()
	lastOp := atomic.LoadInt64(&p.metrics.lastOperationTime)

	if lastOp > 0 {
		timeDiff := now - lastOp
		if timeDiff < int64(time.Second)/p.metrics.operationWindow {
			return &JsonsError{
				Op:      "rate_limit",
				Message: "operation rate limit exceeded",
				Err:     ErrOperationFailed,
			}
		}
	}

	atomic.StoreInt64(&p.metrics.lastOperationTime, now)
	return nil
}

// incrementErrorCount atomically increments the error counter with optional logging
func (p *Processor) incrementErrorCount() {
	atomic.AddInt64(&p.metrics.errorCount, 1)
}

// logError logs an error with structured logging and enhanced context
func (p *Processor) logError(ctx context.Context, operation, path string, err error) {
	if p.logger == nil {
		return
	}

	// Extract error type for metrics using modern error handling
	errorType := "unknown"
	var jsonErr *JsonsError
	if errors.As(err, &jsonErr) && jsonErr.Err != nil {
		errorType = jsonErr.Err.Error()
	}

	// Record error in metrics
	if p.metrics != nil {
		p.metrics.collector.RecordError(errorType)
	}

	// Sanitize sensitive information from path and error
	sanitizedPath := p.sanitizePath(path)
	sanitizedError := p.sanitizeError(err)

	// Use structured logging with modern Go 1.24+ patterns
	p.logger.ErrorContext(ctx, "JSON operation failed",
		slog.String("operation", operation),
		slog.String("path", sanitizedPath),
		slog.String("error", sanitizedError),
		slog.String("error_type", errorType),
		slog.Int64("error_count", atomic.LoadInt64(&p.metrics.errorCount)),
		slog.String("processor_id", p.getProcessorID()),
		slog.Bool("cache_enabled", p.config.EnableCache),
		slog.Int64("concurrent_ops", atomic.LoadInt64(&p.metrics.operationCount)),
	)
}

// sanitizePath removes potentially sensitive information from paths
func (p *Processor) sanitizePath(path string) string {
	if len(path) > 100 {
		return path[:50] + "...[truncated]..." + path[len(path)-20:]
	}
	// Remove potential sensitive patterns but keep structure
	sanitized := path
	sensitivePatterns := []string{"password", "token", "key", "secret", "auth"}
	for _, pattern := range sensitivePatterns {
		if strings.Contains(strings.ToLower(sanitized), pattern) {
			return "[REDACTED_PATH]"
		}
	}
	return sanitized
}

// sanitizeError removes potentially sensitive information from error messages
func (p *Processor) sanitizeError(err error) string {
	if err == nil {
		return ""
	}
	errMsg := err.Error()
	if len(errMsg) > 200 {
		return errMsg[:100] + "...[truncated]"
	}
	return errMsg
}

// logOperation logs a successful operation with structured logging and performance warnings
func (p *Processor) logOperation(ctx context.Context, operation, path string, duration time.Duration) {
	if p.logger == nil {
		return
	}

	// Check for performance issues
	const slowOperationThreshold = 100 * time.Millisecond

	// Use modern structured logging with typed attributes
	commonAttrs := []slog.Attr{
		slog.String("operation", operation),
		slog.String("path", path),
		slog.Int64("duration_ms", duration.Milliseconds()),
		slog.Int64("operation_count", atomic.LoadInt64(&p.metrics.operationCount)),
		slog.String("processor_id", p.getProcessorID()),
	}

	if duration > slowOperationThreshold {
		// Log as warning for slow operations
		attrs := append(commonAttrs, slog.Int64("threshold_ms", slowOperationThreshold.Milliseconds()))
		p.logger.LogAttrs(ctx, slog.LevelWarn, "Slow JSON operation detected", attrs...)
	} else {
		// Log as debug for normal operations
		p.logger.LogAttrs(ctx, slog.LevelDebug, "JSON operation completed", commonAttrs...)
	}
}

// getProcessorID returns a unique identifier for this processor instance
func (p *Processor) getProcessorID() string {
	// Use the processor's memory address as a unique identifier
	return fmt.Sprintf("proc_%p", p)
}

// isRetryableError checks if an error is retryable
func (p *Processor) isRetryableError(err error) bool {
	if err == nil {
		return false
	}

	// Check for specific retryable error types
	var jsonErr *JsonsError
	if errors.As(err, &jsonErr) {
		switch {
		case errors.Is(jsonErr.Err, ErrCacheFull), errors.Is(jsonErr.Err, ErrConcurrencyLimit), errors.Is(jsonErr.Err, ErrRateLimitExceeded):
			return true
		default:
			return false
		}
	}

	return false
}

// executeWithRetry executes an operation with automatic retry for retryable errors
func (p *Processor) executeWithRetry(ctx context.Context, operation string, fn func() error) error {
	const maxRetries = 3
	const baseDelay = 10 * time.Millisecond

	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		err := fn()
		if err == nil {
			return nil // Success
		}

		lastErr = err

		// Don't retry on the last attempt or if error is not retryable
		if attempt == maxRetries || !p.isRetryableError(err) {
			break
		}

		// Exponential backoff
		delay := baseDelay * time.Duration(1<<uint(attempt))

		if p.logger != nil {
			p.logger.WarnContext(ctx, "Retrying operation after error",
				"operation", operation,
				"attempt", attempt+1,
				"max_retries", maxRetries,
				"delay_ms", delay.Milliseconds(),
				"error", err,
			)
		}

		// Wait before retry
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
			// Continue to next attempt
		}
	}

	return lastErr
}

// validateInput validates JSON input string with enhanced security checks
func (p *Processor) validateInput(jsonString string) error {
	if jsonString == "" {
		return &JsonsError{
			Op:      "validate_input",
			Message: "JSON string cannot be empty",
			Err:     ErrInvalidJSON,
		}
	}

	// Enhanced size limits with stricter defaults
	jsonSize := int64(len(jsonString))
	maxSize := p.config.MaxJSONSize
	if maxSize <= 0 {
		maxSize = 10 * 1024 * 1024 // Default 10MB limit for security
	}
	if jsonSize > maxSize {
		return &JsonsError{
			Op:      "validate_input",
			Message: fmt.Sprintf("JSON size %d exceeds maximum %d", jsonSize, maxSize),
			Err:     ErrSizeLimit,
		}
	}

	// Check for minimum size to prevent empty attacks
	if jsonSize < 1 { // At least one character
		return &JsonsError{
			Op:      "validate_input",
			Message: "JSON string too short to be valid",
			Err:     ErrInvalidJSON,
		}
	}

	// Valid UTF-8 encoding with BOM detection
	if !utf8.ValidString(jsonString) {
		return &JsonsError{
			Op:      "validate_input",
			Message: "JSON string contains invalid UTF-8 sequences",
			Err:     ErrInvalidJSON,
		}
	}

	// Check for BOM and other problematic byte sequences
	if len(jsonString) >= 3 && jsonString[:3] == "\xEF\xBB\xBF" {
		return &JsonsError{
			Op:      "validate_input",
			Message: "JSON string contains UTF-8 BOM which is not allowed",
			Err:     ErrInvalidJSON,
		}
	}

	// Check for potential security threats in JSON content
	if err := p.validateJSONSecurity(jsonString); err != nil {
		return err
	}

	// Basic JSON structure validation (quick check)
	if err := p.validateJSONStructure(jsonString); err != nil {
		return &JsonsError{
			Op:      "validate_input",
			Message: fmt.Sprintf("invalid JSON structure: %v", err),
			Err:     ErrInvalidJSON,
		}
	}

	return nil
}

// validateJSONSecurity checks for potential security threats in JSON content
func (p *Processor) validateJSONSecurity(jsonString string) error {
	// Get security limits from configuration
	braceDepth := 0
	bracketDepth := 0
	maxDepth := p.config.MaxNestingDepthSecurity
	if maxDepth <= 0 {
		maxDepth = 50 // Fallback default
	}

	// Check size limits from configuration
	maxSize := p.config.MaxSecurityValidationSize
	if maxSize <= 0 {
		maxSize = 100 * 1024 * 1024 // Fallback to 100MB
	}
	if int64(len(jsonString)) > maxSize {
		return &JsonsError{
			Op:      "validate_input",
			Message: fmt.Sprintf("JSON too large for security validation (size: %d, max: %d)", len(jsonString), maxSize),
			Err:     ErrSizeLimit,
		}
	}

	// Get limits from configuration
	keyCount := 0
	arrayElementCount := 0
	maxKeys := p.config.MaxObjectKeys
	if maxKeys <= 0 {
		maxKeys = 10000 // Fallback default
	}
	maxArrayElements := p.config.MaxArrayElements
	if maxArrayElements <= 0 {
		maxArrayElements = 10000 // Fallback default
	}
	inString := false
	escaped := false

	for i, char := range jsonString {
		// Handle string parsing to avoid counting delimiters inside strings
		if !escaped && char == '"' {
			inString = !inString
		}
		if inString {
			escaped = !escaped && char == '\\'
			continue
		}
		escaped = false

		switch char {
		case '{':
			if !inString {
				braceDepth++
				keyCount++ // Approximate key counting
				if braceDepth > maxDepth {
					return &JsonsError{
						Op:      "validate_input",
						Message: fmt.Sprintf("JSON nesting too deep at position %d (max %d)", i, maxDepth),
						Err:     ErrInvalidJSON,
					}
				}
				if keyCount > maxKeys {
					return &JsonsError{
						Op:      "validate_input",
						Message: "Too many keys in JSON object",
						Err:     ErrSizeLimit,
					}
				}
			}
		case '}':
			if !inString {
				braceDepth--
				if braceDepth < 0 {
					return &JsonsError{
						Op:      "validate_input",
						Message: fmt.Sprintf("Unmatched closing brace at position %d", i),
						Err:     ErrInvalidJSON,
					}
				}
			}
		case '[':
			if !inString {
				bracketDepth++
				if bracketDepth > maxDepth {
					return &JsonsError{
						Op:      "validate_input",
						Message: fmt.Sprintf("Array nesting too deep at position %d (max %d)", i, maxDepth),
						Err:     ErrInvalidJSON,
					}
				}
			}
		case ']':
			if !inString {
				bracketDepth--
				if bracketDepth < 0 {
					return &JsonsError{
						Op:      "validate_input",
						Message: fmt.Sprintf("Unmatched closing bracket at position %d", i),
						Err:     ErrInvalidJSON,
					}
				}
			}
		case ',':
			if !inString && bracketDepth > 0 {
				arrayElementCount++
				if arrayElementCount > maxArrayElements {
					return &JsonsError{
						Op:      "validate_input",
						Message: "Too many array elements in JSON",
						Err:     ErrSizeLimit,
					}
				}
			}
		}
	}

	// Check for potential billion laughs attack patterns
	if strings.Count(jsonString, "\"") > 10000 {
		return &JsonsError{
			Op:      "validate_input",
			Message: "Excessive number of string delimiters detected",
			Err:     ErrInvalidJSON,
		}
	}

	// Check for suspicious control characters
	for i, char := range jsonString {
		if char < 32 && char != '\t' && char != '\n' && char != '\r' {
			return &JsonsError{
				Op:      "validate_input",
				Message: fmt.Sprintf("Suspicious control character at position %d", i),
				Err:     ErrInvalidJSON,
			}
		}
	}

	return nil
}

// validateJSONStructure performs basic JSON structure validation
func (p *Processor) validateJSONStructure(jsonString string) error {
	trimmed := strings.TrimSpace(jsonString)
	if len(trimmed) == 0 {
		return fmt.Errorf("empty JSON after trimming whitespace")
	}

	// Check for basic JSON structure
	firstChar := trimmed[0]
	lastChar := trimmed[len(trimmed)-1]

	switch firstChar {
	case '{':
		if lastChar != '}' {
			return fmt.Errorf("JSON object not properly closed")
		}
	case '[':
		if lastChar != ']' {
			return fmt.Errorf("JSON array not properly closed")
		}
	case '"':
		if lastChar != '"' || len(trimmed) < 2 {
			return fmt.Errorf("JSON string not properly closed")
		}
	case 't', 'f':
		// Boolean values
		if !strings.HasPrefix(trimmed, "true") && !strings.HasPrefix(trimmed, "false") {
			return fmt.Errorf("invalid boolean value")
		}
	case 'n':
		// Null value
		if !strings.HasPrefix(trimmed, "null") {
			return fmt.Errorf("invalid null value")
		}
	default:
		// Should be a number
		if !p.isValidNumberStart(firstChar) {
			return fmt.Errorf("invalid JSON start character: %c", firstChar)
		}
	}

	return nil
}

// isValidNumberStart checks if character can start a valid JSON number
func (p *Processor) isValidNumberStart(c byte) bool {
	return (c >= '0' && c <= '9') || c == '-'
}

// validatePath validates a JSON path string with enhanced security and efficiency
func (p *Processor) validatePath(path string) error {
	if path == "" || path == "." {
		// Empty path or "." means root, which is valid
		return nil
	}

	// Check for path injection attempts
	if err := p.validatePathSecurity(path); err != nil {
		return err
	}

	// Preprocess path for validation (same as navigation)
	sb := p.getStringBuilder()
	defer p.putStringBuilder(sb)
	preprocessedPath := p.preprocessPath(path, sb)

	// Fast path depth check without string splitting for simple paths
	if !strings.Contains(preprocessedPath, ".") && !strings.Contains(preprocessedPath, "/") {
		return p.validateSingleSegment(preprocessedPath)
	}

	// Check path depth by counting segments
	var segmentCount int
	if strings.HasPrefix(preprocessedPath, "/") {
		// JSON Pointer format validation (use original path for JSON Pointer)
		if err := p.validateJSONPointerPath(path); err != nil {
			return err
		}
		segmentCount = strings.Count(path[1:], "/") + 1
	} else {
		// Dot notation format validation (use preprocessed path)
		if err := p.validateDotNotationPath(preprocessedPath); err != nil {
			return err
		}
		segmentCount = strings.Count(preprocessedPath, ".") + 1
	}

	if segmentCount > p.config.MaxPathDepth {
		return &JsonsError{
			Op:      "validate_path",
			Path:    path,
			Message: fmt.Sprintf("path depth %d exceeds maximum %d", segmentCount, p.config.MaxPathDepth),
			Err:     ErrDepthLimit,
		}
	}

	return nil
}

// validatePathSecurity checks for potential security issues in paths
func (p *Processor) validatePathSecurity(path string) error {
	// Check for null bytes (potential injection)
	if strings.Contains(path, "\x00") {
		return &JsonsError{
			Op:      "validate_path",
			Path:    path,
			Message: "path contains null bytes",
			Err:     ErrInvalidPath,
		}
	}

	// Check for excessively long paths
	if len(path) > 10000 {
		return &JsonsError{
			Op:      "validate_path",
			Path:    path,
			Message: fmt.Sprintf("path too long: %d characters", len(path)),
			Err:     ErrInvalidPath,
		}
	}

	// Check for suspicious patterns that could indicate injection attempts
	// Enhanced pattern list to prevent bypass attempts
	suspiciousPatterns := []string{
		// Path traversal patterns
		"../", "./", "//", "\\", "\\\\", "..", "..\\", "..%2f", "..%5c",
		"%2e%2e%2f", "%2e%2e%5c", "%2e%2e/", "%2e%2e\\",
		"....//", "....\\\\", ".%2e/", ".%2e\\",

		// Script injection patterns
		"<script", "</script", "javascript:", "data:", "vbscript:",
		"onload=", "onerror=", "onclick=", "onmouseover=",

		// Protocol patterns
		"file://", "ftp://", "http://", "https://", "ldap://", "gopher://",

		// Command injection patterns
		"eval(", "exec(", "system(", "cmd(", "shell_exec(", "passthru(",
		"popen(", "proc_open(", "`", "$(", "${", "#{", "%{", "{{",

		// Encoding bypass attempts
		"%00", "%0a", "%0d", "%09", "%20", "%22", "%27", "%3c", "%3e",
		"\\x00", "\\n", "\\r", "\\t", "\x00", "\n", "\r", "\t",

		// SQL injection patterns (in case paths are used in queries)
		"'", "\"", ";", "--", "/*", "*/", "union", "select", "drop", "insert",

		// Template injection patterns
		"{{", "}}", "${", "#{", "%{", "<%", "%>", "<#", "#>",
	}

	lowerPath := strings.ToLower(path)
	for _, pattern := range suspiciousPatterns {
		if strings.Contains(lowerPath, pattern) {
			return &JsonsError{
				Op:      "validate_path",
				Path:    path,
				Message: fmt.Sprintf("path contains suspicious pattern: %s", pattern),
				Err:     ErrInvalidPath,
			}
		}
	}

	return nil
}

// validateSingleSegment validates a single path segment
func (p *Processor) validateSingleSegment(segment string) error {
	// Check for unmatched brackets or braces first
	if strings.Contains(segment, "[") && !strings.Contains(segment, "]") {
		return &JsonsError{
			Op:      "validate_path",
			Path:    segment,
			Message: "unclosed bracket '[' in path segment",
			Err:     ErrInvalidPath,
		}
	}

	if strings.Contains(segment, "]") && !strings.Contains(segment, "[") {
		return &JsonsError{
			Op:      "validate_path",
			Path:    segment,
			Message: "unmatched bracket ']' in path segment",
			Err:     ErrInvalidPath,
		}
	}

	if strings.Contains(segment, "{") && !strings.Contains(segment, "}") {
		return &JsonsError{
			Op:      "validate_path",
			Path:    segment,
			Message: "unclosed brace '{' in path segment",
			Err:     ErrInvalidPath,
		}
	}

	if strings.Contains(segment, "}") && !strings.Contains(segment, "{") {
		return &JsonsError{
			Op:      "validate_path",
			Path:    segment,
			Message: "unmatched brace '}' in path segment",
			Err:     ErrInvalidPath,
		}
	}

	// Check for valid identifier or array index (optimized without regex)
	if isSimpleProperty(segment) {
		return nil
	}

	if isNumericIndex(segment) {
		return nil
	}

	// Check for array access syntax
	if strings.Contains(segment, "[") && strings.Contains(segment, "]") {
		return p.validateArrayAccess(segment)
	}

	// Check for pure slice syntax (starts with [ and contains :)
	if strings.HasPrefix(segment, "[") && strings.Contains(segment, ":") && strings.HasSuffix(segment, "]") {
		return p.validateSliceSyntax(segment)
	}

	// Check for extraction syntax
	if strings.Contains(segment, "{") && strings.Contains(segment, "}") {
		return p.validateExtractionSyntax(segment)
	}

	return &JsonsError{
		Op:      "validate_path",
		Path:    segment,
		Message: "invalid path segment format",
		Err:     ErrInvalidPath,
	}
}

// validateArrayAccess validates array access syntax
func (p *Processor) validateArrayAccess(segment string) error {
	// Check for unmatched brackets
	openCount := strings.Count(segment, "[")
	closeCount := strings.Count(segment, "]")
	if openCount != closeCount {
		return &JsonsError{
			Op:      "validate_path",
			Path:    segment,
			Message: "unmatched brackets in array access",
			Err:     ErrInvalidPath,
		}
	}

	// Find bracket positions
	start := strings.Index(segment, "[")
	end := strings.LastIndex(segment, "]")
	if start == -1 || end == -1 || end <= start {
		return &JsonsError{
			Op:      "validate_path",
			Path:    segment,
			Message: "malformed bracket syntax",
			Err:     ErrInvalidPath,
		}
	}

	indexPart := segment[start+1 : end]

	// Check if it's a slice (contains colon)
	if strings.Contains(indexPart, ":") {
		return p.validateSliceSyntax(segment)
	}

	// Valid simple array index
	if indexPart == "" {
		return &JsonsError{
			Op:      "validate_path",
			Path:    segment,
			Message: "empty array index",
			Err:     ErrInvalidPath,
		}
	}

	// Check for wildcard
	if indexPart == "*" {
		return nil // Wildcard is valid
	}

	// Check if it's a valid number (including negative)
	if _, err := strconv.Atoi(indexPart); err != nil {
		return &JsonsError{
			Op:      "validate_path",
			Path:    segment,
			Message: fmt.Sprintf("invalid array index '%s': must be a number or '*'", indexPart),
			Err:     ErrInvalidPath,
		}
	}

	return nil
}

// validateSliceSyntax validates array slice syntax like [1:3], [::2], [::-1]
func (p *Processor) validateSliceSyntax(segment string) error {
	// Extract the slice part between brackets
	start := strings.Index(segment, "[")
	end := strings.LastIndex(segment, "]")
	if start == -1 || end == -1 || end <= start {
		return &JsonsError{
			Op:      "validate_path",
			Path:    segment,
			Message: "malformed slice syntax",
			Err:     ErrInvalidPath,
		}
	}

	slicePart := segment[start+1 : end]

	// Parse slice components
	_, _, _, err := p.parseSliceComponents(slicePart)
	if err != nil {
		return &JsonsError{
			Op:      "validate_path",
			Path:    segment,
			Message: fmt.Sprintf("invalid slice syntax: %v", err),
			Err:     ErrInvalidPath,
		}
	}

	return nil
}

// validateExtractionSyntax validates extraction syntax like {name}
func (p *Processor) validateExtractionSyntax(segment string) error {
	// Check for unmatched braces
	openCount := strings.Count(segment, "{")
	closeCount := strings.Count(segment, "}")
	if openCount != closeCount {
		return &JsonsError{
			Op:      "validate_path",
			Path:    segment,
			Message: "unmatched braces in extraction syntax",
			Err:     ErrInvalidPath,
		}
	}

	// Find brace positions
	start := strings.Index(segment, "{")
	end := strings.LastIndex(segment, "}")
	if start == -1 || end == -1 || end <= start {
		return &JsonsError{
			Op:      "validate_path",
			Path:    segment,
			Message: "malformed extraction syntax",
			Err:     ErrInvalidPath,
		}
	}

	fieldName := segment[start+1 : end]
	if fieldName == "" {
		return &JsonsError{
			Op:      "validate_path",
			Path:    segment,
			Message: "empty field name in extraction syntax",
			Err:     ErrInvalidPath,
		}
	}

	// Check for unsupported conditional filter syntax
	if strings.HasPrefix(fieldName, "?") {
		return &JsonsError{
			Op:      "validate_path",
			Path:    segment,
			Message: fmt.Sprintf("conditional filter syntax '{%s}' is not supported. Use standard extraction syntax like '{fieldName}' instead", fieldName),
			Err:     ErrInvalidPath,
		}
	}

	// Check for other unsupported query-like syntax patterns
	if strings.Contains(fieldName, "=") || strings.Contains(fieldName, ">") || strings.Contains(fieldName, "<") || strings.Contains(fieldName, "&") || strings.Contains(fieldName, "|") {
		return &JsonsError{
			Op:      "validate_path",
			Path:    segment,
			Message: fmt.Sprintf("query syntax '{%s}' is not supported. Use standard extraction syntax like '{fieldName}' instead", fieldName),
			Err:     ErrInvalidPath,
		}
	}

	return nil
}

// validateJSONPointerPath validates JSON Pointer format paths
func (p *Processor) validateJSONPointerPath(path string) error {
	// Basic JSON Pointer validation
	if !strings.HasPrefix(path, "/") {
		return &JsonsError{
			Op:      "validate_path",
			Path:    path,
			Message: "JSON Pointer must start with /",
			Err:     ErrInvalidPath,
		}
	}

	// Check for trailing slash (invalid except for root "/")
	if len(path) > 1 && strings.HasSuffix(path, "/") {
		return &JsonsError{
			Op:      "validate_path",
			Path:    path,
			Message: "JSON Pointer cannot end with trailing slash",
			Err:     ErrInvalidPath,
		}
	}

	// Check for proper escaping
	segments := strings.Split(path[1:], "/")
	for _, segment := range segments {
		if strings.Contains(segment, "~") {
			// Valid escape sequences
			if !p.isValidJSONPointerEscape(segment) {
				return &JsonsError{
					Op:      "validate_path",
					Path:    path,
					Message: "invalid JSON Pointer escape sequence",
					Err:     ErrInvalidPath,
				}
			}
		}
	}

	return nil
}

// validateDotNotationPath validates dot notation format paths
func (p *Processor) validateDotNotationPath(path string) error {
	// Check for consecutive dots
	if strings.Contains(path, "..") {
		return &JsonsError{
			Op:      "validate_path",
			Path:    path,
			Message: "path contains consecutive dots",
			Err:     ErrInvalidPath,
		}
	}

	// Check for leading/trailing dots
	if strings.HasPrefix(path, ".") || strings.HasSuffix(path, ".") {
		return &JsonsError{
			Op:      "validate_path",
			Path:    path,
			Message: "path has leading or trailing dots",
			Err:     ErrInvalidPath,
		}
	}

	// Split path into segments and validate each one
	segments := strings.Split(path, ".")
	for _, segment := range segments {
		if segment == "" {
			continue // Skip empty segments (shouldn't happen after above checks)
		}

		if err := p.validateSingleSegment(segment); err != nil {
			return err
		}
	}

	return nil
}

// isValidJSONPointerEscape validates JSON Pointer escape sequences
func (p *Processor) isValidJSONPointerEscape(segment string) bool {
	i := 0
	for i < len(segment) {
		if segment[i] == '~' {
			if i+1 >= len(segment) {
				return false // Incomplete escape
			}
			next := segment[i+1]
			if next != '0' && next != '1' {
				return false // Invalid escape
			}
			i += 2
		} else {
			i++
		}
	}
	return true
}

// prepareOptions prepares and validates processor options
func (p *Processor) prepareOptions(opts ...*ProcessorOptions) (*ProcessorOptions, error) {
	var options *ProcessorOptions
	if len(opts) > 0 && opts[0] != nil {
		options = opts[0]
	} else {
		options = DefaultOptions()
	}

	// Valid options
	if err := ValidateOptions(options); err != nil {
		return nil, err
	}

	return options, nil
}

// createCacheKey creates a cache key with optimized efficiency using resource pools
func (p *Processor) createCacheKey(operation, jsonStr, path string, options *ProcessorOptions) string {
	// Get string builder from pool for memory efficiency
	sb := p.getStringBuilder()
	defer p.putStringBuilder(sb)

	// Pre-allocate capacity more accurately to reduce reallocations
	estimatedSize := len(operation) + len(path) + 32 // Reduced overhead estimation
	if len(jsonStr) < 1024 {
		estimatedSize += 32 // Hash size for small JSON
	} else {
		estimatedSize += 64 // Hash size for larger JSON
	}
	sb.Grow(estimatedSize)

	sb.WriteString(operation)
	sb.WriteByte(':')

	// Optimize JSON normalization for better cache hit rates
	normalizedJSON := p.normalizeJSONForCache(jsonStr)
	sb.WriteString(p.cache.SecureHash(normalizedJSON))
	sb.WriteByte(':')
	sb.WriteString(path)

	// Include relevant options in the key using more efficient approach
	if options != nil {
		// Use single conditional writes to reduce function call overhead
		if options.StrictMode {
			sb.WriteString(":s")
		}
		if options.AllowComments {
			sb.WriteString(":c")
		}
		if options.PreserveNumbers {
			sb.WriteString(":p")
		}
		if options.MaxDepth > 0 {
			sb.WriteString(":d")
			sb.WriteString(strconv.Itoa(options.MaxDepth))
		}
	}

	return sb.String()
}

// normalizeJSONForCache normalizes JSON string for better cache hit rates with minimal overhead
func (p *Processor) normalizeJSONForCache(jsonStr string) string {
	// For very small JSON strings, use lightweight normalization
	if len(jsonStr) < 256 {
		// Simple whitespace normalization without full parsing for small JSON
		return p.lightweightJSONNormalize(jsonStr)
	}

	// For medium JSON strings, use full normalization
	if len(jsonStr) < 1024 {
		// Parse and re-encode to normalize formatting
		var data any
		if err := p.Parse(jsonStr, &data); err != nil {
			// If parsing fails, return lightweight normalized version
			return p.lightweightJSONNormalize(jsonStr)
		}

		// Re-encode with compact format to normalize
		config := DefaultEncodeConfig()
		config.Pretty = false
		config.SortKeys = true // Sort keys for consistent ordering
		if normalized, err := p.EncodeWithConfig(data, config); err == nil {
			return normalized
		}
	}

	// For large JSON strings, return original to avoid performance penalty
	return jsonStr
}

// lightweightJSONNormalize performs lightweight JSON normalization without parsing
func (p *Processor) lightweightJSONNormalize(jsonStr string) string {
	// Use string builder from pool
	sb := p.getStringBuilder()
	defer p.putStringBuilder(sb)

	// Pre-allocate capacity to reduce reallocations
	sb.Grow(len(jsonStr))

	// Simple whitespace and formatting normalization using byte operations for performance
	inString := false
	escaped := false
	bytes := []byte(jsonStr)

	for i := 0; i < len(bytes); i++ {
		b := bytes[i]

		if escaped {
			sb.WriteByte(b)
			escaped = false
			continue
		}

		if b == '\\' {
			sb.WriteByte(b)
			escaped = true
			continue
		}

		if b == '"' {
			inString = !inString
			sb.WriteByte(b)
			continue
		}

		if inString {
			sb.WriteByte(b)
			continue
		}

		// Outside string - normalize whitespace
		switch b {
		case ' ', '\t', '\n', '\r':
			// Skip whitespace outside strings
			continue
		case ':', ',':
			sb.WriteByte(b)
		default:
			sb.WriteByte(b)
		}
	}

	return sb.String()
}

// getCachedPathSegments gets parsed path segments from cache or parses and caches them
func (p *Processor) getCachedPathSegments(path string) ([]internal.PathSegment, error) {
	// Check cache first (read lock)
	p.resources.pathCacheMutex.RLock()
	if entry, exists := p.resources.pathParseCache[path]; exists {
		// Update last access time
		atomic.StoreInt64(&entry.lastAccess, time.Now().UnixNano())
		// Make a copy to avoid race conditions
		cached := entry.value.([]internal.PathSegment)
		result := make([]internal.PathSegment, len(cached))
		copy(result, cached)
		p.resources.pathCacheMutex.RUnlock()
		return result, nil
	}
	p.resources.pathCacheMutex.RUnlock()

	// Parse path (not in cache)
	parser := internal.NewPathParser()
	segments, err := parser.ParsePath(path)
	if err != nil {
		return nil, err
	}

	// Cache the result (write lock) - use double-check pattern to prevent race conditions
	p.resources.pathCacheMutex.Lock()
	defer p.resources.pathCacheMutex.Unlock()

	// Double-check: another goroutine might have cached it while we were parsing
	if entry, exists := p.resources.pathParseCache[path]; exists {
		return entry.value.([]internal.PathSegment), nil
	}

	// Check cache size to prevent memory bloat and ensure processor is still active
	if p.resources.pathParseCache != nil && len(p.resources.pathParseCache) < 1000 && atomic.LoadInt32(&p.state) == 0 {
		// Make a copy for caching
		cached := make([]internal.PathSegment, len(segments))
		copy(cached, segments)
		p.resources.pathParseCache[path] = &cacheEntry{
			value:      cached,
			lastAccess: time.Now().UnixNano(),
		}
		atomic.StoreInt64(&p.resources.pathCacheSize, int64(len(p.resources.pathParseCache)))
	}

	return segments, nil
}

// getCachedParsedJSON gets parsed JSON from cache or parses and caches it
func (p *Processor) getCachedParsedJSON(jsonStr string) (any, error) {
	// For small JSON strings, use caching
	if len(jsonStr) < 2048 {
		// Check cache first (read lock)
		p.resources.jsonCacheMutex.RLock()
		if entry, exists := p.resources.jsonParseCache[jsonStr]; exists {
			// Update last access time
			atomic.StoreInt64(&entry.lastAccess, time.Now().UnixNano())
			cached := entry.value
			p.resources.jsonCacheMutex.RUnlock()
			return cached, nil
		}
		p.resources.jsonCacheMutex.RUnlock()
	}

	// Parse JSON (not in cache or too large)
	var data any
	err := p.Parse(jsonStr, &data)
	if err != nil {
		return nil, err
	}

	// Cache the result for small JSON strings - use double-check pattern
	if len(jsonStr) < 2048 {
		p.resources.jsonCacheMutex.Lock()
		defer p.resources.jsonCacheMutex.Unlock()

		// Double-check: another goroutine might have cached it while we were parsing
		if entry, exists := p.resources.jsonParseCache[jsonStr]; exists {
			return entry.value, nil
		}

		// Check cache size to prevent memory bloat and ensure processor is still active
		if p.resources.jsonParseCache != nil && len(p.resources.jsonParseCache) < 100 && atomic.LoadInt32(&p.state) == 0 {
			p.resources.jsonParseCache[jsonStr] = &cacheEntry{
				value:      data,
				lastAccess: time.Now().UnixNano(),
			}
			atomic.StoreInt64(&p.resources.jsonCacheSize, int64(len(p.resources.jsonParseCache)))
		}
	}

	return data, nil
}

// isSimpleProperty checks if a string is a simple property name without using regex
func isSimpleProperty(s string) bool {
	if len(s) == 0 {
		return false
	}

	// First character must be letter or underscore
	first := s[0]
	if !((first >= 'a' && first <= 'z') || (first >= 'A' && first <= 'Z') || first == '_') {
		return false
	}

	// Remaining characters must be letters, digits, or underscores
	for i := 1; i < len(s); i++ {
		c := s[i]
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_') {
			return false
		}
	}

	return true
}

// isNumericIndex checks if a string represents a numeric index without using regex
func isNumericIndex(s string) bool {
	if len(s) == 0 {
		return false
	}

	start := 0
	if s[0] == '-' {
		if len(s) == 1 {
			return false
		}
		start = 1
	}

	for i := start; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}

	return true
}

// getCachedResult retrieves a cached result if available
func (p *Processor) getCachedResult(key string) (any, bool) {
	if !p.config.EnableCache {
		return nil, false
	}
	return p.cache.Get(key)
}

// setCachedResult stores a result in cache with security validation
func (p *Processor) setCachedResult(key string, result any, options ...*ProcessorOptions) {
	if !p.config.EnableCache {
		return
	}

	// Check if caching is enabled for this operation
	if len(options) > 0 && options[0] != nil && !options[0].CacheResults {
		return
	}

	// Security validation: don't cache potentially sensitive data
	if p.containsSensitiveData(result) {
		return
	}

	// Validate cache key to prevent injection
	if !p.isValidCacheKey(key) {
		return
	}

	p.cache.Set(key, result)
}

// containsSensitiveData checks if the result contains sensitive information
func (p *Processor) containsSensitiveData(result any) bool {
	if result == nil {
		return false
	}

	// Convert to string for pattern matching
	resultStr := fmt.Sprintf("%v", result)
	if len(resultStr) > 1000 { // Don't cache large results
		return true
	}

	// Check for sensitive patterns
	sensitivePatterns := []string{"password", "token", "secret", "key", "auth", "credential"}
	lowerResult := strings.ToLower(resultStr)
	for _, pattern := range sensitivePatterns {
		if strings.Contains(lowerResult, pattern) {
			return true
		}
	}
	return false
}

// isValidCacheKey validates cache key format
func (p *Processor) isValidCacheKey(key string) bool {
	if len(key) > 500 { // Prevent excessively long keys
		return false
	}
	// Check for null bytes and control characters
	for _, b := range []byte(key) {
		if b < 32 || b == 127 { // Control characters
			return false
		}
	}
	return true
}

// parseSliceComponents parses slice syntax using unified array utilities
func (p *Processor) parseSliceComponents(slicePart string) (start, end, step *int, err error) {
	arrayUtils := internal.NewArrayUtils()
	return arrayUtils.ParseSliceComponents(slicePart)
}

// SafeGet performs a type-safe get operation with comprehensive error handling
func (p *Processor) SafeGet(jsonStr, path string) TypeSafeAccessResult {
	// Validate inputs
	if jsonStr == "" {
		return TypeSafeAccessResult{Exists: false}
	}
	if path == "" {
		return TypeSafeAccessResult{Exists: false}
	}

	// Perform the get operation
	value, err := p.Get(jsonStr, path)
	if err != nil {
		return TypeSafeAccessResult{Exists: false}
	}

	// Determine the type
	var valueType string
	if value == nil {
		valueType = "null"
	} else {
		valueType = fmt.Sprintf("%T", value)
	}

	return TypeSafeAccessResult{
		Value:  value,
		Exists: true,
		Type:   valueType,
	}
}

// SafeGetTypedWithProcessor performs a type-safe get operation with generic type constraints
func SafeGetTypedWithProcessor[T any](p *Processor, jsonStr, path string) TypeSafeResult[T] {
	var zero T

	// Validate inputs
	if jsonStr == "" || path == "" {
		return TypeSafeResult[T]{Value: zero, Exists: false, Error: ErrPathNotFound}
	}

	// Perform the get operation
	value, err := p.Get(jsonStr, path)
	if err != nil {
		return TypeSafeResult[T]{Value: zero, Exists: false, Error: err}
	}

	// Type assertion with safety
	if typedValue, ok := value.(T); ok {
		return TypeSafeResult[T]{Value: typedValue, Exists: true, Error: nil}
	}

	// Attempt type conversion
	if converted, err := TypeSafeConvert[T](value); err == nil {
		return TypeSafeResult[T]{Value: converted, Exists: true, Error: nil}
	}

	return TypeSafeResult[T]{
		Value:  zero,
		Exists: true,
		Error:  fmt.Errorf("type mismatch: expected %T, got %T", zero, value),
	}
}
