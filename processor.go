package json

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"reflect"
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
// Simplified: removed duplicate caching, now uses internal.CacheManager exclusively
type processorResources struct {
	stringBuilderPool *sync.Pool
	pathSegmentPool   *sync.Pool
	lastPoolReset     int64
	lastMemoryCheck   int64
	memoryPressure    int32
}

// processorMetrics consolidates all metrics and performance tracking
type processorMetrics struct {
	operationCount       int64
	errorCount           int64
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
		// Return a processor with error state instead of panicking
		// This allows graceful error handling in production
		p := &Processor{
			config: DefaultConfig(),
			state:  2, // closed state
		}
		return p
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
				New: func() any {
					sb := &strings.Builder{}
					sb.Grow(DefaultStringBuilderSize)
					return sb
				},
			},
			pathSegmentPool: &sync.Pool{
				New: func() any {
					return make([]PathSegment, 0, DefaultPathSegmentCap)
				},
			},
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
		return make([]PathSegment, 0, DefaultPathSegmentCap)
	}

	segments := p.resources.pathSegmentPool.Get().([]PathSegment)
	return segments[:0] // Reset length but keep capacity
}

// putPathSegments returns a path segments slice to the pool
func (p *Processor) putPathSegments(segments []PathSegment) {
	if p.resources.pathSegmentPool != nil && segments != nil && !p.isClosing() {
		if cap(segments) <= MaxPathSegmentCap && cap(segments) >= DefaultPathSegmentCap {
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

	// Perform leak detection
	if p.resourceMonitor != nil {
		if issues := p.resourceMonitor.CheckForLeaks(); len(issues) > 0 {
			for _, issue := range issues {
				p.logger.Warn("Resource issue detected", "issue", issue)
			}
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

func calculateSuccessRateInternal(successful, total int64) float64 {
	if total == 0 {
		return 0.0
	}
	return float64(successful) / float64(total) * 100.0
}

func calculateHitRatioInternal(hits, misses int64) float64 {
	total := hits + misses
	if total == 0 {
		return 0.0
	}
	return float64(hits) / float64(total) * 100.0
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

// getDetailedStats returns detailed performance statistics
func (p *Processor) getDetailedStats() DetailedStats {
	stats := p.GetStats()

	return DetailedStats{
		Stats:          stats,
		state:          atomic.LoadInt32(&p.state),
		configSnapshot: *p.config,
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
		SuccessRate:           calculateSuccessRateInternal(internalMetrics.SuccessfulOps, internalMetrics.TotalOperations),
		CacheHits:             internalMetrics.CacheHits,
		CacheMisses:           internalMetrics.CacheMisses,
		CacheHitRate:          calculateHitRatioInternal(internalMetrics.CacheHits, internalMetrics.CacheMisses),
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

// logError logs an error with structured logging
func (p *Processor) logError(ctx context.Context, operation, path string, err error) {
	if p.logger == nil {
		return
	}

	errorType := "unknown"
	var jsonErr *JsonsError
	if errors.As(err, &jsonErr) && jsonErr.Err != nil {
		errorType = jsonErr.Err.Error()
	}

	if p.metrics != nil {
		p.metrics.collector.RecordError(errorType)
	}

	sanitizedPath := sanitizePath(path)
	sanitizedError := sanitizeError(err)

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
func sanitizePath(path string) string {
	if len(path) > 100 {
		return truncateString(path, 100)
	}
	// Remove potential sensitive patterns but keep structure
	// Use case-insensitive matching for better security
	lowerPath := strings.ToLower(path)
	sensitivePatterns := []string{
		"password", "passwd", "pwd",
		"token", "bearer",
		"key", "apikey", "api_key", "api-key",
		"secret", "credential", "cred",
		"auth", "authorization",
		"session", "cookie",
	}
	for _, pattern := range sensitivePatterns {
		if strings.Contains(lowerPath, pattern) {
			return "[REDACTED_PATH]"
		}
	}
	return path
}

// sanitizeError removes potentially sensitive information from error messages
func sanitizeError(err error) string {
	if err == nil {
		return ""
	}
	errMsg := err.Error()
	if len(errMsg) > 200 {
		return truncateString(errMsg, 200)
	}
	return errMsg
}

// truncateString efficiently truncates a string with ellipsis
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

// logOperation logs a successful operation with structured logging and performance warnings
func (p *Processor) logOperation(ctx context.Context, operation, path string, duration time.Duration) {
	if p.logger == nil {
		return
	}

	// Use modern structured logging with typed attributes
	commonAttrs := []slog.Attr{
		slog.String("operation", operation),
		slog.String("path", path),
		slog.Int64("duration_ms", duration.Milliseconds()),
		slog.Int64("operation_count", atomic.LoadInt64(&p.metrics.operationCount)),
		slog.String("processor_id", p.getProcessorID()),
	}

	if duration > SlowOperationThreshold {
		// Log as warning for slow operations
		attrs := append(commonAttrs, slog.Int64("threshold_ms", SlowOperationThreshold.Milliseconds()))
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

// validateInput validates JSON input string with optimized security checks
func (p *Processor) validateInput(jsonString string) error {
	// Fast path: check empty string
	if jsonString == "" {
		return newOperationError("validate_input", "JSON string cannot be empty", ErrInvalidJSON)
	}

	jsonSize := int64(len(jsonString))

	// Size validation with configured limits
	maxSize := p.config.MaxJSONSize
	if maxSize <= 0 {
		maxSize = DefaultMaxJSONSize
	}
	if jsonSize > maxSize {
		return newSizeLimitError("validate_input", jsonSize, maxSize)
	}

	// UTF-8 validation (optimized: only check if not ASCII-only)
	if !isASCIIOnly(jsonString) && !utf8.ValidString(jsonString) {
		return newOperationError("validate_input", "JSON string contains invalid UTF-8 sequences", ErrInvalidJSON)
	}

	// BOM detection (optimized: single check)
	if jsonSize >= 3 && jsonString[0] == '\xEF' && jsonString[1] == '\xBB' && jsonString[2] == '\xBF' {
		return newOperationError("validate_input", "JSON string contains UTF-8 BOM which is not allowed", ErrInvalidJSON)
	}

	// Combined security and structure validation (single pass)
	return p.validateJSONSecurityAndStructure(jsonString)
}

// isASCIIOnly checks if string contains only ASCII characters (fast path optimization)
func isASCIIOnly(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] > 127 {
			return false
		}
	}
	return true
}

// validateJSONSecurityAndStructure performs combined security and structure validation in a single pass
// This optimization reduces redundant iterations over the JSON string
func (p *Processor) validateJSONSecurityAndStructure(jsonString string) error {
	// Get security limits from configuration
	maxDepth := p.config.MaxNestingDepthSecurity
	if maxDepth <= 0 {
		maxDepth = DefaultMaxNestingDepth
	}

	// Trim whitespace for structure validation
	trimmed := strings.TrimSpace(jsonString)
	if len(trimmed) == 0 {
		return newOperationError("validate_input", "empty JSON after trimming whitespace", ErrInvalidJSON)
	}

	// Validate first and last characters for basic structure
	firstChar := trimmed[0]
	lastChar := trimmed[len(trimmed)-1]

	// Quick structure validation based on first character
	switch firstChar {
	case '{':
		if lastChar != '}' {
			return newOperationError("validate_input", "JSON object not properly closed", ErrInvalidJSON)
		}
	case '[':
		if lastChar != ']' {
			return newOperationError("validate_input", "JSON array not properly closed", ErrInvalidJSON)
		}
	case '"':
		if lastChar != '"' || len(trimmed) < 2 {
			return newOperationError("validate_input", "JSON string not properly closed", ErrInvalidJSON)
		}
		return nil // String values don't need depth checking
	case 't', 'f', 'n':
		// Primitive values (true, false, null) - no depth checking needed
		return nil
	default:
		// Numbers - no depth checking needed
		if !isValidNumberStart(firstChar) {
			return newOperationError("validate_input", fmt.Sprintf("invalid JSON start character: %c", firstChar), ErrInvalidJSON)
		}
		return nil
	}

	// Depth validation (only for objects and arrays)
	braceDepth := 0
	bracketDepth := 0

	for _, char := range trimmed {
		switch char {
		case '{':
			braceDepth++
			if braceDepth > maxDepth {
				return newDepthLimitError("validate_security", braceDepth, maxDepth)
			}
		case '}':
			braceDepth--
		case '[':
			bracketDepth++
			if bracketDepth > maxDepth {
				return newDepthLimitError("validate_security", bracketDepth, maxDepth)
			}
		case ']':
			bracketDepth--
		}
	}

	return nil
}

// isValidNumberStart checks if character can start a valid JSON number
func isValidNumberStart(c byte) bool {
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
		return newSecurityError("validate_path", "null byte injection detected in path")
	}

	// Check for excessively long paths
	if len(path) > MaxPathLength {
		return newPathError(path, fmt.Sprintf("path too long: %d characters (max: %d)", len(path), MaxPathLength), ErrInvalidPath)
	}

	// Enhanced path traversal detection with bypass protection
	// Check for standard traversal patterns
	if strings.Contains(path, "..") {
		return newSecurityError("validate_path", "path traversal pattern detected")
	}

	// Check for URL-encoded traversal patterns (case-insensitive)
	normalized := strings.ToLower(path)
	encodedPatterns := []string{
		"%2e%2e", // URL-encoded ..
		"%2e.",   // Partially encoded (mixed)
		".%2e",   // Partially encoded (mixed)
		"%252e",  // Double-encoded .
		"%c0%af", // UTF-8 overlong encoding for /
		"%c1%9c", // UTF-8 overlong encoding for \
	}
	for _, pattern := range encodedPatterns {
		if strings.Contains(normalized, pattern) {
			return newSecurityError("validate_path", "encoded path traversal pattern detected")
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

// getCachedPathSegments gets parsed path segments using unified cache
func (p *Processor) getCachedPathSegments(path string) ([]internal.PathSegment, error) {
	// Use unified cache manager
	if p.config.EnableCache {
		cacheKey := "path:" + path
		if cached, ok := p.cache.Get(cacheKey); ok {
			if segments, ok := cached.([]internal.PathSegment); ok {
				// Make a copy to avoid race conditions
				result := make([]internal.PathSegment, len(segments))
				copy(result, segments)
				return result, nil
			}
		}
	}

	// Parse path
	parser := internal.NewPathParser()
	segments, err := parser.ParsePath(path)
	if err != nil {
		return nil, err
	}

	// Cache the result using unified cache
	if p.config.EnableCache && atomic.LoadInt32(&p.state) == 0 {
		cacheKey := "path:" + path
		cached := make([]internal.PathSegment, len(segments))
		copy(cached, segments)
		p.cache.Set(cacheKey, cached)
	}

	return segments, nil
}

// getCachedParsedJSON gets parsed JSON using unified cache
func (p *Processor) getCachedParsedJSON(jsonStr string) (any, error) {
	// Only cache small JSON strings
	if len(jsonStr) < 2048 && p.config.EnableCache {
		cacheKey := "json:" + jsonStr
		if cached, ok := p.cache.Get(cacheKey); ok {
			return cached, nil
		}
	}

	// Parse JSON
	var data any
	err := p.Parse(jsonStr, &data)
	if err != nil {
		return nil, err
	}

	// Cache small JSON strings using unified cache
	if len(jsonStr) < 2048 && p.config.EnableCache && atomic.LoadInt32(&p.state) == 0 {
		cacheKey := "json:" + jsonStr
		p.cache.Set(cacheKey, data)
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

// Get retrieves a value from JSON using a path expression with performance
func (p *Processor) Get(jsonStr, path string, opts ...*ProcessorOptions) (any, error) {
	// Check rate limiting for security
	if err := p.checkRateLimit(); err != nil {
		return nil, err
	}

	// Record operation start time for metrics
	startTime := time.Now()

	// Increment operation counter for statistics
	p.incrementOperationCount()

	// Record concurrent operation start
	if p.metrics != nil && p.metrics.collector != nil {
		p.metrics.collector.StartConcurrentOperation()
		defer p.metrics.collector.EndConcurrentOperation()
	}

	if err := p.checkClosed(); err != nil {
		p.incrementErrorCount()
		if p.metrics != nil && p.metrics.collector != nil {
			p.metrics.collector.RecordOperation(time.Since(startTime), false, 0)
		}
		return nil, err
	}

	options, err := p.prepareOptions(opts...)
	if err != nil {
		p.incrementErrorCount()
		return nil, err
	}

	// Get context from options or use background
	ctx := context.Background()
	if options.Context != nil {
		ctx = options.Context
	}

	// Check for context cancellation
	select {
	case <-ctx.Done():
		p.incrementErrorCount()
		p.logError(ctx, "get", path, ctx.Err())
		return nil, ctx.Err()
	default:
	}

	// Continue with the rest of the method...
	defer func() {
		duration := time.Since(startTime)
		p.logOperation(ctx, "get", path, duration)
	}()

	if err := p.validateInput(jsonStr); err != nil {
		p.incrementErrorCount()
		if p.metrics != nil && p.metrics.collector != nil {
			p.metrics.collector.RecordOperation(time.Since(startTime), false, 0)
		}
		return nil, err
	}

	if err := p.validatePath(path); err != nil {
		p.incrementErrorCount()
		if p.metrics != nil && p.metrics.collector != nil {
			p.metrics.collector.RecordOperation(time.Since(startTime), false, 0)
		}
		return nil, err
	}

	// Check cache first with optimized key generation
	cacheKey := p.createCacheKey("get", jsonStr, path, options)
	if cached, ok := p.getCachedResult(cacheKey); ok {
		// Record cache hit operation
		if p.metrics != nil && p.metrics.collector != nil {
			p.metrics.collector.RecordOperation(time.Since(startTime), true, 0)
			p.metrics.collector.RecordCacheHit()
		}
		return cached, nil
	}

	// Record cache miss
	if p.metrics != nil && p.metrics.collector != nil {
		p.metrics.collector.RecordCacheMiss()
	}

	// Try to get parsed data from cache first
	parseCacheKey := p.createCacheKey("parse", jsonStr, "", options)
	var data any

	if cachedData, ok := p.getCachedResult(parseCacheKey); ok {
		data = cachedData
	} else {
		// Parse JSON with error context
		var parseErr error
		parseErr = p.Parse(jsonStr, &data, opts...)
		if parseErr != nil {
			p.incrementErrorCount()
			if p.metrics != nil && p.metrics.collector != nil {
				p.metrics.collector.RecordOperation(time.Since(startTime), false, 0)
			}
			return nil, parseErr
		}

		// Cache parsed data for reuse
		if options.CacheResults && p.config.EnableCache {
			p.setCachedResult(parseCacheKey, data, options)
		}
	}

	// Use unified recursive processor for all paths
	unifiedProcessor := NewRecursiveProcessor(p)
	result, err := unifiedProcessor.ProcessRecursively(data, path, OpGet, nil)
	if err != nil {
		p.incrementErrorCount()
		if p.metrics != nil && p.metrics.collector != nil {
			p.metrics.collector.RecordOperation(time.Since(startTime), false, 0)
		}
		return nil, &JsonsError{
			Op:      "get",
			Path:    path,
			Message: err.Error(),
			Err:     err,
		}
	}

	// Cache result if enabled
	p.setCachedResult(cacheKey, result, options)

	// Record successful operation
	if p.metrics != nil && p.metrics.collector != nil {
		p.metrics.collector.RecordOperation(time.Since(startTime), true, 0)
	}

	return result, nil
}

// GetMultiple retrieves multiple values from JSON using multiple path expressions
func (p *Processor) GetMultiple(jsonStr string, paths []string, opts ...*ProcessorOptions) (map[string]any, error) {
	if err := p.checkClosed(); err != nil {
		return nil, err
	}

	if err := p.validateInput(jsonStr); err != nil {
		return nil, err
	}

	if len(paths) == 0 {
		return make(map[string]any), nil
	}

	_, err := p.prepareOptions(opts...)
	if err != nil {
		return nil, err
	}

	// Parse JSON once for all operations
	var data any
	if err := p.Parse(jsonStr, &data, opts...); err != nil {
		return nil, err
	}

	// Sequential processing
	results := make(map[string]any, len(paths))
	for _, path := range paths {
		if err := p.validatePath(path); err != nil {
			return nil, err
		}

		// Use unified processor for all paths
		unifiedProcessor := NewRecursiveProcessor(p)
		result, err := unifiedProcessor.ProcessRecursively(data, path, OpGet, nil)

		if err != nil {
			// Continue with other paths, store error as result
			results[path] = nil
		} else {
			results[path] = result
		}
	}

	return results, nil
}

// getMultipleParallel processes multiple paths in parallel
func (p *Processor) getMultipleParallel(data any, paths []string, _ *ProcessorOptions) (map[string]any, error) {
	results := make(map[string]any, len(paths)) // Pre-allocate with known size
	var mu sync.RWMutex                         // Use RWMutex for better read performance
	var wg sync.WaitGroup

	// Create semaphore to limit concurrency
	semaphore := make(chan struct{}, p.config.MaxConcurrency)

	// Create context with timeout to prevent goroutine leaks
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	for _, path := range paths {
		wg.Add(1)
		go func(currentPath string) {
			defer wg.Done()

			// Check for context cancellation
			select {
			case <-ctx.Done():
				mu.Lock()
				results[currentPath] = nil
				mu.Unlock()
				return
			case semaphore <- struct{}{}:
				// Acquired semaphore, continue
			}
			defer func() { <-semaphore }()

			// Valid path
			if err := p.validatePath(currentPath); err != nil {
				mu.Lock()
				results[currentPath] = nil
				mu.Unlock()
				return
			}

			// Navigate to path with context check
			select {
			case <-ctx.Done():
				mu.Lock()
				results[currentPath] = nil
				mu.Unlock()
				return
			default:
				// Use unified recursive processor for consistency
				var result any
				var err error

				unifiedProcessor := NewRecursiveProcessor(p)
				result, err = unifiedProcessor.ProcessRecursively(data, currentPath, OpGet, nil)

				// Store result
				mu.Lock()
				if err != nil {
					results[currentPath] = nil
				} else {
					results[currentPath] = result
				}
				mu.Unlock()
			}
		}(path)
	}

	// Wait for completion or timeout
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return results, nil
	case <-ctx.Done():
		return results, &JsonsError{
			Op:      "get_multiple_parallel",
			Message: "operation timed out",
			Err:     ErrOperationFailed,
		}
	}
}

// Set sets a value in JSON at the specified path
// Returns:
//   - On success: modified JSON string and nil error
//   - On failure: original unmodified JSON string and error information
func (p *Processor) Set(jsonStr, path string, value any, opts ...*ProcessorOptions) (string, error) {
	if err := p.checkClosed(); err != nil {
		return jsonStr, err
	}

	options, err := p.prepareOptions(opts...)
	if err != nil {
		return jsonStr, err
	}

	if err := p.validateInput(jsonStr); err != nil {
		return jsonStr, err
	}

	if err := p.validatePath(path); err != nil {
		return jsonStr, err
	}

	// Parse JSON
	var data any
	err = p.Parse(jsonStr, &data, opts...)
	if err != nil {
		return jsonStr, &JsonsError{
			Op:      "set",
			Path:    path,
			Message: fmt.Sprintf("failed to parse JSON: %v", err),
			Err:     err,
		}
	}

	// Create a deep copy of the data for modification attempts
	// This ensures we don't modify the original data if the operation fails
	var dataCopy any
	if copyBytes, marshalErr := json.Marshal(data); marshalErr == nil {
		if unmarshalErr := json.Unmarshal(copyBytes, &dataCopy); unmarshalErr != nil {
			return jsonStr, &JsonsError{
				Op:      "set",
				Path:    path,
				Message: fmt.Sprintf("failed to create data copy: %v", unmarshalErr),
				Err:     unmarshalErr,
			}
		}
	} else {
		return jsonStr, &JsonsError{
			Op:      "set",
			Path:    path,
			Message: fmt.Sprintf("failed to create data copy: %v", marshalErr),
			Err:     marshalErr,
		}
	}

	// Determine if we should create paths
	createPaths := options.CreatePaths || p.config.CreatePaths

	// Set the value at the specified path on the copy
	err = p.setValueAtPathWithOptions(dataCopy, path, value, createPaths)
	if err != nil {
		// Return original data and detailed error information
		var setError *JsonsError
		if _, ok := err.(*RootDataTypeConversionError); ok && createPaths {
			setError = &JsonsError{
				Op:      "set",
				Path:    path,
				Message: fmt.Sprintf("root data type conversion failed: %v", err),
				Err:     err,
			}
		} else {
			setError = &JsonsError{
				Op:      "set",
				Path:    path,
				Message: fmt.Sprintf("set operation failed: %v", err),
				Err:     err,
			}
		}
		return jsonStr, setError
	}

	// Convert modified data back to JSON string
	resultBytes, err := json.Marshal(dataCopy)
	if err != nil {
		// Return original data if marshaling fails
		return jsonStr, &JsonsError{
			Op:      "set",
			Path:    path,
			Message: fmt.Sprintf("failed to marshal modified data: %v", err),
			Err:     ErrOperationFailed,
		}
	}

	return string(resultBytes), nil
}

// SetMultiple sets multiple values in JSON using a map of path-value pairs
// Returns:
//   - On success: modified JSON string and nil error
//   - On failure: original unmodified JSON string and error information
func (p *Processor) SetMultiple(jsonStr string, updates map[string]any, opts ...*ProcessorOptions) (string, error) {
	if err := p.checkClosed(); err != nil {
		return jsonStr, err
	}

	// Validate input
	if len(updates) == 0 {
		return jsonStr, nil // No updates to apply
	}

	// Prepare options
	options, err := p.prepareOptions(opts...)
	if err != nil {
		return jsonStr, err
	}

	// Validate JSON input
	if err := p.validateInput(jsonStr); err != nil {
		return jsonStr, err
	}

	// Validate all paths before processing
	for path := range updates {
		if err := p.validatePath(path); err != nil {
			return jsonStr, &JsonsError{
				Op:      "set_multiple",
				Path:    path,
				Message: fmt.Sprintf("invalid path '%s': %v", path, err),
				Err:     err,
			}
		}
	}

	// Parse JSON
	var data any
	err = p.Parse(jsonStr, &data, opts...)
	if err != nil {
		return jsonStr, &JsonsError{
			Op:      "set_multiple",
			Message: fmt.Sprintf("failed to parse JSON: %v", err),
			Err:     err,
		}
	}

	// Create a deep copy of the data for modification attempts
	var dataCopy any
	if copyBytes, marshalErr := json.Marshal(data); marshalErr == nil {
		if unmarshalErr := json.Unmarshal(copyBytes, &dataCopy); unmarshalErr != nil {
			return jsonStr, &JsonsError{
				Op:      "set_multiple",
				Message: fmt.Sprintf("failed to create data copy: %v", unmarshalErr),
				Err:     unmarshalErr,
			}
		}
	} else {
		return jsonStr, &JsonsError{
			Op:      "set_multiple",
			Message: fmt.Sprintf("failed to create data copy: %v", marshalErr),
			Err:     marshalErr,
		}
	}

	// Determine if we should create paths
	createPaths := options.CreatePaths || p.config.CreatePaths

	// Apply all updates on the copy
	var lastError error
	successCount := 0
	failedPaths := make([]string, 0)

	for path, value := range updates {
		err := p.setValueAtPathWithOptions(dataCopy, path, value, createPaths)
		if err != nil {
			// Handle root data type conversion errors
			if _, ok := err.(*RootDataTypeConversionError); ok && createPaths {
				lastError = &JsonsError{
					Op:      "set_multiple",
					Path:    path,
					Message: fmt.Sprintf("root data type conversion failed for path '%s': %v", path, err),
					Err:     err,
				}
				failedPaths = append(failedPaths, path)
				if !options.ContinueOnError {
					return jsonStr, lastError
				}
			} else {
				lastError = &JsonsError{
					Op:      "set_multiple",
					Path:    path,
					Message: fmt.Sprintf("failed to set path '%s': %v", path, err),
					Err:     err,
				}
				failedPaths = append(failedPaths, path)
				if !options.ContinueOnError {
					return jsonStr, lastError
				}
			}
		} else {
			successCount++
		}
	}

	// If no updates were successful and we have errors, return original data and error
	if successCount == 0 && lastError != nil {
		return jsonStr, &JsonsError{
			Op:      "set_multiple",
			Message: fmt.Sprintf("all %d updates failed, last error: %v", len(updates), lastError),
			Err:     lastError,
		}
	}

	// If some updates failed but we're continuing on error, log the failures
	if len(failedPaths) > 0 && options.ContinueOnError {
		// Could log warnings here if logger is available
		// For now, we continue silently as requested
	}

	// Convert modified data back to JSON string
	resultBytes, err := json.Marshal(dataCopy)
	if err != nil {
		// Return original data if marshaling fails
		return jsonStr, &JsonsError{
			Op:      "set_multiple",
			Message: fmt.Sprintf("failed to marshal modified data: %v", err),
			Err:     ErrOperationFailed,
		}
	}

	return string(resultBytes), nil
}

// Delete removes a value from JSON at the specified path
func (p *Processor) Delete(jsonStr, path string, opts ...*ProcessorOptions) (string, error) {
	if err := p.checkClosed(); err != nil {
		return "", err
	}

	_, err := p.prepareOptions(opts...)
	if err != nil {
		return "", err
	}

	if err := p.validateInput(jsonStr); err != nil {
		return jsonStr, err
	}

	if err := p.validatePath(path); err != nil {
		return jsonStr, err
	}

	// Parse JSON
	var data any
	err = p.Parse(jsonStr, &data, opts...)
	if err != nil {
		return jsonStr, err
	}

	// Get the effective options
	var effectiveOpts *ProcessorOptions
	if len(opts) > 0 && opts[0] != nil {
		effectiveOpts = opts[0]
	} else {
		effectiveOpts = &ProcessorOptions{}
	}

	// Determine cleanup options
	cleanupNulls := effectiveOpts.CleanupNulls || p.config.CleanupNulls
	compactArrays := effectiveOpts.CompactArrays || p.config.CompactArrays

	// If compactArrays is enabled, automatically enable cleanupNulls
	if compactArrays {
		cleanupNulls = true
	}

	// Delete the value at the specified path
	err = p.deleteValueAtPath(data, path)
	if err != nil {
		// For any deletion error, return the original JSON unchanged instead of empty string
		// This includes "path not found", "property not found", and other deletion errors
		return jsonStr, &JsonsError{
			Op:      "delete",
			Path:    path,
			Message: err.Error(),
			Err:     err,
		}
	}

	// Clean up deleted markers from the data (this handles array element removal)
	data = p.cleanupDeletedMarkers(data)

	// Cleanup nulls if requested
	if cleanupNulls {
		data = p.cleanupNullValuesWithReconstruction(data, compactArrays)
	}

	// Convert back to JSON string
	resultBytes, err := json.Marshal(data)
	if err != nil {
		// Return original JSON instead of empty string when marshaling fails
		return jsonStr, &JsonsError{
			Op:      "delete",
			Path:    path,
			Message: fmt.Sprintf("failed to marshal result: %v", err),
			Err:     ErrOperationFailed,
		}
	}

	return string(resultBytes), nil
}

// ProcessBatch processes multiple operations in a single batch
func (p *Processor) ProcessBatch(operations []BatchOperation, opts ...*ProcessorOptions) ([]BatchResult, error) {
	if err := p.checkClosed(); err != nil {
		return nil, err
	}

	_, err := p.prepareOptions(opts...)
	if err != nil {
		return nil, err
	}

	if len(operations) > p.config.MaxBatchSize {
		return nil, &JsonsError{
			Op:      "process_batch",
			Message: fmt.Sprintf("batch size %d exceeds maximum %d", len(operations), p.config.MaxBatchSize),
			Err:     ErrSizeLimit,
		}
	}

	results := make([]BatchResult, len(operations))

	for i, op := range operations {
		result := BatchResult{ID: op.ID}

		switch op.Type {
		case "get":
			result.Result, result.Error = p.Get(op.JSONStr, op.Path, opts...)
		case "set":
			result.Result, result.Error = p.Set(op.JSONStr, op.Path, op.Value, opts...)
		case "delete":
			result.Result, result.Error = p.Delete(op.JSONStr, op.Path, opts...)
		case "validate":
			result.Result, result.Error = p.Valid(op.JSONStr, opts...)
		default:
			result.Error = fmt.Errorf("unknown operation type: %s", op.Type)
		}

		results[i] = result
	}

	return results, nil
}

// processorUtils implements the ProcessorUtils interface
type processorUtils struct {
	// String builder pool for efficient string operations
	stringBuilderPool *stringBuilderPool
}

// NewProcessorUtils creates a new processor utils instance
func NewProcessorUtils() ProcessorUtils {
	return &processorUtils{
		stringBuilderPool: newStringBuilderPool(),
	}
}

// IsArrayType checks if the data is an array type
func (u *processorUtils) IsArrayType(data any) bool {
	switch data.(type) {
	case []any:
		return true
	default:
		return false
	}
}

// IsObjectType checks if the data is an object type
func (u *processorUtils) IsObjectType(data any) bool {
	switch data.(type) {
	case map[string]any, map[any]any:
		return true
	default:
		return false
	}
}

// IsEmptyContainer checks if a container (object or array) is empty
func (u *processorUtils) IsEmptyContainer(data any) bool {
	switch v := data.(type) {
	case map[string]any:
		return len(v) == 0
	case map[any]any:
		return len(v) == 0
	case []any:
		// Check if all elements are nil
		for _, item := range v {
			if item != nil {
				return false
			}
		}
		return true
	default:
		return false
	}
}

// DeepCopy creates a deep copy of the data structure
func (u *processorUtils) DeepCopy(data any) (any, error) {
	// Use JSON marshal/unmarshal for deep copy
	jsonBytes, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal data for deep copy: %w", err)
	}

	var result any
	err = json.Unmarshal(jsonBytes, &result)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal data for deep copy: %w", err)
	}

	return result, nil
}

// GetDataType returns a string representation of the data type
func (u *processorUtils) GetDataType(data any) string {
	if data == nil {
		return "null"
	}

	switch data.(type) {
	case map[string]any:
		return "object"
	case map[any]any:
		return "object"
	case []any:
		return "array"
	case string:
		return "string"
	case float64:
		return "number"
	case int:
		return "number"
	case bool:
		return "boolean"
	default:
		return reflect.TypeOf(data).String()
	}
}

// ConvertToMap converts data to map[string]any if possible
func (u *processorUtils) ConvertToMap(data any) (map[string]any, bool) {
	switch v := data.(type) {
	case map[string]any:
		return v, true
	case map[any]any:
		// Convert map[any]any to map[string]any
		result := make(map[string]any)
		for key, value := range v {
			if strKey, ok := key.(string); ok {
				result[strKey] = value
			} else {
				result[fmt.Sprintf("%v", key)] = value
			}
		}
		return result, true
	default:
		return nil, false
	}
}

// ConvertToArray converts data to []any if possible
func (u *processorUtils) ConvertToArray(data any) ([]any, bool) {
	switch v := data.(type) {
	case []any:
		return v, true
	default:
		// Try to convert using reflection
		rv := reflect.ValueOf(data)
		if rv.Kind() == reflect.Slice {
			result := make([]any, rv.Len())
			for i := 0; i < rv.Len(); i++ {
				result[i] = rv.Index(i).Interface()
			}
			return result, true
		}
		return nil, false
	}
}

// stringBuilderPool provides a pool of string builders for efficient string operations
type stringBuilderPool struct {
	pool sync.Pool
}

// newStringBuilderPool creates a new string builder pool
func newStringBuilderPool() *stringBuilderPool {
	return &stringBuilderPool{
		pool: sync.Pool{
			New: func() any {
				return &strings.Builder{}
			},
		},
	}
}

// Get gets a string builder from the pool
func (p *stringBuilderPool) Get() *strings.Builder {
	return p.pool.Get().(*strings.Builder)
}

// Put returns a string builder to the pool
func (p *stringBuilderPool) Put(sb *strings.Builder) {
	sb.Reset()
	p.pool.Put(sb)
}

// ParseInt parses a string to integer with error handling
func ParseInt(s string) (int, error) {
	return strconv.Atoi(s)
}

// ParseFloat parses a string to float64 with error handling
func ParseFloat(s string) (float64, error) {
	return strconv.ParseFloat(s, 64)
}

// ParseBool parses a string to boolean with error handling
func ParseBool(s string) (bool, error) {
	return strconv.ParseBool(s)
}

// IsNumeric checks if a string represents a numeric value
func IsNumeric(s string) bool {
	_, err := strconv.ParseFloat(s, 64)
	return err == nil
}

// IsInteger checks if a string represents an integer value
func IsInteger(s string) bool {
	_, err := strconv.Atoi(s)
	return err == nil
}

// NormalizeIndex normalizes an array index (handles negative indices)
func NormalizeIndex(index, length int) int {
	if index < 0 {
		return length + index
	}
	return index
}

// IsValidIndex checks if an index is valid for an array of given length
func IsValidIndex(index, length int) bool {
	normalizedIndex := NormalizeIndex(index, length)
	return normalizedIndex >= 0 && normalizedIndex < length
}

// ClampIndex clamps an index to valid bounds for an array
func ClampIndex(index, length int) int {
	if index < 0 {
		return 0
	}
	if index >= length {
		return length - 1
	}
	return index
}

// SanitizeKey sanitizes a key for safe use in maps
func SanitizeKey(key string) string {
	// Remove any null bytes or other problematic characters
	return strings.ReplaceAll(key, "\x00", "")
}

// EscapeJSONPointer escapes special characters for JSON Pointer
func EscapeJSONPointer(s string) string {
	s = strings.ReplaceAll(s, "~", "~0")
	s = strings.ReplaceAll(s, "/", "~1")
	return s
}

// UnescapeJSONPointer unescapes JSON Pointer special characters
func UnescapeJSONPointer(s string) string {
	s = strings.ReplaceAll(s, "~1", "/")
	s = strings.ReplaceAll(s, "~0", "~")
	return s
}

// IsContainer checks if the data is a container type (map or slice)
func IsContainer(data any) bool {
	switch data.(type) {
	case map[string]any, map[any]any, []any:
		return true
	default:
		return false
	}
}

// GetContainerSize returns the size of a container
func GetContainerSize(data any) int {
	switch v := data.(type) {
	case map[string]any:
		return len(v)
	case map[any]any:
		return len(v)
	case []any:
		return len(v)
	default:
		return 0
	}
}

// CreateEmptyContainer creates an empty container of the specified type
func CreateEmptyContainer(containerType string) any {
	switch containerType {
	case "object":
		return make(map[string]any)
	case "array":
		return make([]any, 0)
	default:
		return make(map[string]any) // Default to object
	}
}

// mergeObjects merges two objects, with the second object taking precedence (internal use)
func mergeObjects(obj1, obj2 map[string]any) map[string]any {
	result := make(map[string]any)

	// Copy from first object
	for k, v := range obj1 {
		result[k] = v
	}

	// Override with second object
	for k, v := range obj2 {
		result[k] = v
	}

	return result
}

// flattenArray flattens a nested array structure (internal use)
func flattenArray(arr []any) []any {
	var result []any

	for _, item := range arr {
		if subArr, ok := item.([]any); ok {
			result = append(result, flattenArray(subArr)...)
		} else {
			result = append(result, item)
		}
	}

	return result
}

// uniqueArray removes duplicate values from an array (internal use)
func uniqueArray(arr []any) []any {
	seen := make(map[string]bool)
	var result []any

	for _, item := range arr {
		key := fmt.Sprintf("%v", item)
		if !seen[key] {
			seen[key] = true
			result = append(result, item)
		}
	}

	return result
}

// reverseArray reverses an array in place (internal use)
func reverseArray(arr []any) {
	for i, j := 0, len(arr)-1; i < j; i, j = i+1, j-1 {
		arr[i], arr[j] = arr[j], arr[i]
	}
}

// ConvertToString converts a value to string
func (u *processorUtils) ConvertToString(value any) string {
	if value == nil {
		return ""
	}

	switch v := value.(type) {
	case string:
		return v
	case int:
		return strconv.Itoa(v)
	case int64:
		return strconv.FormatInt(v, 10)
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(v)
	default:
		return fmt.Sprintf("%v", v)
	}
}

// ConvertToNumber converts a value to a number (float64)
func (u *processorUtils) ConvertToNumber(value any) (float64, error) {
	switch v := value.(type) {
	case float64:
		return v, nil
	case int:
		return float64(v), nil
	case int64:
		return float64(v), nil
	case string:
		return strconv.ParseFloat(v, 64)
	default:
		return 0, fmt.Errorf("cannot convert %T to number", value)
	}
}

// modularProcessor implements the ModularProcessor interface using composition
type modularProcessor struct {
	// Core modules
	pathParser PathParser
	navigator  Navigator
	arrayOps   ArrayOperations
	extractOps ExtractionOperations
	setOps     SetOperations
	deleteOps  DeleteOperations
	utils      ProcessorUtils

	// Configuration and state
	config  *ProcessorConfig
	cache   ProcessorCache
	metrics MetricsCollector
	limiter RateLimiter

	// Synchronization
	mu     sync.RWMutex
	closed bool
}

// NewModularProcessor creates a new modular processor instance
func NewModularProcessor(config *ProcessorConfig) ModularProcessor {
	if config == nil {
		config = DefaultProcessorConfig()
	}

	utils := NewProcessorUtils()
	pathParser := NewPathParser()
	navigator := NewNavigator(pathParser, utils)
	arrayOps := NewArrayOperations(utils)
	extractionOps := NewExtractionOperations(utils)
	setOps := NewSetOperations(utils, pathParser, navigator, arrayOps)
	deleteOps := NewDeleteOperations(utils, pathParser, navigator, arrayOps)

	return &modularProcessor{
		pathParser: pathParser,
		navigator:  navigator,
		arrayOps:   arrayOps,
		extractOps: extractionOps,
		setOps:     setOps,
		deleteOps:  deleteOps,
		utils:      utils,
		config:     config,
		closed:     false,
	}
}

// Get retrieves a value from JSON using a path expression
func (mp *modularProcessor) Get(jsonStr, path string, opts ...*ProcessorOptions) (any, error) {
	if err := mp.checkClosed(); err != nil {
		return nil, err
	}

	// Parse options
	options := mp.prepareOptions(opts...)

	// Start timing
	startTime := time.Now()
	defer func() {
		if mp.metrics != nil {
			mp.metrics.RecordOperation(time.Since(startTime), true, 0)
		}
	}()

	// Rate limiting check
	if mp.limiter != nil && !mp.limiter.Allow() {
		return nil, ErrRateLimitNew
	}

	// Validate inputs
	if err := mp.validateInput(jsonStr); err != nil {
		return nil, err
	}

	if err := mp.pathParser.ValidatePath(path); err != nil {
		return nil, &ProcessorError{
			Type:      ErrTypeValidation,
			Operation: "get",
			Path:      path,
			Message:   "invalid path",
			Cause:     err,
		}
	}

	// Check cache first
	if mp.cache != nil {
		cacheKey := mp.createCacheKey("get", jsonStr, path, options)
		if cached, ok := mp.cache.Get(cacheKey); ok {
			if mp.metrics != nil {
				mp.metrics.RecordCacheHit()
			}
			return cached, nil
		}
		if mp.metrics != nil {
			mp.metrics.RecordCacheMiss()
		}
	}

	// Parse JSON
	var data any
	if err := json.Unmarshal([]byte(jsonStr), &data); err != nil {
		return nil, &ProcessorError{
			Type:      ErrTypeValidation,
			Operation: "get",
			Path:      path,
			Message:   "invalid JSON",
			Cause:     err,
		}
	}

	// Parse path into segments
	segments, err := mp.pathParser.ParsePath(path)
	if err != nil {
		return nil, &ProcessorError{
			Type:      ErrTypeValidation,
			Operation: "get",
			Path:      path,
			Message:   "failed to parse path",
			Cause:     err,
		}
	}

	// Navigate to the target value
	result, err := mp.navigator.NavigateToPath(data, segments)
	if err != nil {
		return nil, &ProcessorError{
			Type:      ErrTypeNavigation,
			Operation: "get",
			Path:      path,
			Message:   "navigation failed",
			Cause:     err,
		}
	}

	// Cache the result
	if mp.cache != nil {
		cacheKey := mp.createCacheKey("get", jsonStr, path, options)
		mp.cache.Set(cacheKey, result, mp.config.Timeout)
	}

	return result, nil
}

// Set sets a value in JSON at the specified path
func (mp *modularProcessor) Set(jsonStr, path string, value any, opts ...*ProcessorOptions) (string, error) {
	if err := mp.checkClosed(); err != nil {
		return "", err
	}

	// Parse options
	options := mp.prepareOptions(opts...)

	// Validate inputs
	if err := mp.validateInput(jsonStr); err != nil {
		return "", err
	}

	if err := mp.pathParser.ValidatePath(path); err != nil {
		return "", &ProcessorError{
			Type:      ErrTypeValidation,
			Operation: "set",
			Path:      path,
			Message:   "invalid path",
			Cause:     err,
		}
	}

	// Parse JSON
	var data any
	if err := json.Unmarshal([]byte(jsonStr), &data); err != nil {
		return "", &ProcessorError{
			Type:      ErrTypeValidation,
			Operation: "set",
			Path:      path,
			Message:   "invalid JSON",
			Cause:     err,
		}
	}

	// Parse path into segments
	segments, err := mp.pathParser.ParsePath(path)
	if err != nil {
		return "", &ProcessorError{
			Type:      ErrTypeValidation,
			Operation: "set",
			Path:      path,
			Message:   "failed to parse path",
			Cause:     err,
		}
	}

	// Set the value using the appropriate operations module
	createPaths := options.CreatePaths
	if mp.setOps != nil {
		err = mp.setOps.SetValueWithSegments(data, segments, value, createPaths)
	} else {
		// Fallback to basic implementation
		err = mp.setValueBasic(data, segments, value, createPaths)
	}

	if err != nil {
		return "", &ProcessorError{
			Type:      ErrTypeNavigation,
			Operation: "set",
			Path:      path,
			Message:   "failed to set value",
			Cause:     err,
		}
	}

	// Marshal back to JSON
	resultBytes, err := json.Marshal(data)
	if err != nil {
		return "", &ProcessorError{
			Type:      ErrTypeConversion,
			Operation: "set",
			Path:      path,
			Message:   "failed to marshal result",
			Cause:     err,
		}
	}

	return string(resultBytes), nil
}

// Delete deletes a value from JSON at the specified path
func (mp *modularProcessor) Delete(jsonStr, path string, opts ...*ProcessorOptions) (string, error) {
	if err := mp.checkClosed(); err != nil {
		return "", err
	}

	// Validate inputs
	if err := mp.validateInput(jsonStr); err != nil {
		return jsonStr, err
	}

	if err := mp.pathParser.ValidatePath(path); err != nil {
		return jsonStr, &ProcessorError{
			Type:      ErrTypeValidation,
			Operation: "delete",
			Path:      path,
			Message:   "invalid path",
			Cause:     err,
		}
	}

	// Parse JSON
	var data any
	if err := json.Unmarshal([]byte(jsonStr), &data); err != nil {
		return jsonStr, &ProcessorError{
			Type:      ErrTypeValidation,
			Operation: "delete",
			Path:      path,
			Message:   "invalid JSON",
			Cause:     err,
		}
	}

	// Delete the value using the delete operations module
	if mp.deleteOps != nil {
		err := mp.deleteOps.DeleteValue(data, path)
		if err != nil {
			// Return original JSON instead of empty string when deletion fails
			return jsonStr, &ProcessorError{
				Type:      ErrTypeNavigation,
				Operation: "delete",
				Path:      path,
				Message:   "failed to delete value",
				Cause:     err,
			}
		}
	} else {
		// Return original JSON instead of empty string when delete operations not implemented
		return jsonStr, &ProcessorError{
			Type:      ErrTypeNavigation,
			Operation: "delete",
			Path:      path,
			Message:   "delete operations not implemented",
		}
	}

	// Marshal back to JSON
	resultBytes, err := json.Marshal(data)
	if err != nil {
		// Return original JSON instead of empty string when marshaling fails
		return jsonStr, &ProcessorError{
			Type:      ErrTypeConversion,
			Operation: "delete",
			Path:      path,
			Message:   "failed to marshal result",
			Cause:     err,
		}
	}

	return string(resultBytes), nil
}

// GetMultiple retrieves multiple values from JSON using multiple paths
func (mp *modularProcessor) GetMultiple(jsonStr string, paths []string, opts ...*ProcessorOptions) (map[string]any, error) {
	results := make(map[string]any)

	for _, path := range paths {
		result, err := mp.Get(jsonStr, path, opts...)
		if err != nil {
			results[path] = nil
		} else {
			results[path] = result
		}
	}

	return results, nil
}

// BatchProcess processes multiple operations in batch
func (mp *modularProcessor) BatchProcess(operations []BatchOperation, opts ...*ProcessorOptions) ([]BatchResult, error) {
	results := make([]BatchResult, len(operations))

	for i, op := range operations {
		result := BatchResult{ID: op.ID}

		switch op.Type {
		case "get":
			result.Result, result.Error = mp.Get(op.JSONStr, op.Path, opts...)
		case "set":
			result.Result, result.Error = mp.Set(op.JSONStr, op.Path, op.Value, opts...)
		case "delete":
			result.Result, result.Error = mp.Delete(op.JSONStr, op.Path, opts...)
		case "validate":
			result.Result, result.Error = mp.Valid(op.JSONStr, opts...)
		default:
			result.Error = fmt.Errorf("unknown operation type: %s", op.Type)
		}

		results[i] = result
	}

	return results, nil
}

// Valid validates JSON format
func (mp *modularProcessor) Valid(jsonStr string, opts ...*ProcessorOptions) (bool, error) {
	var data any
	err := json.Unmarshal([]byte(jsonStr), &data)
	return err == nil, err
}

// SetConfig updates the processor configuration
func (mp *modularProcessor) SetConfig(config *ProcessorConfig) {
	mp.mu.Lock()
	defer mp.mu.Unlock()
	mp.config = config
}

// GetConfig returns the current processor configuration
func (mp *modularProcessor) GetConfig() *ProcessorConfig {
	mp.mu.RLock()
	defer mp.mu.RUnlock()
	return mp.config
}

// Close closes the processor and releases resources
func (mp *modularProcessor) Close() error {
	mp.mu.Lock()
	defer mp.mu.Unlock()

	if mp.closed {
		return nil
	}

	mp.closed = true

	// Clear cache if present
	if mp.cache != nil {
		mp.cache.Clear()
	}

	return nil
}

// IsClosed returns whether the processor is closed
func (mp *modularProcessor) IsClosed() bool {
	mp.mu.RLock()
	defer mp.mu.RUnlock()
	return mp.closed
}

// Helper methods

// checkClosed checks if the processor is closed
func (mp *modularProcessor) checkClosed() error {
	mp.mu.RLock()
	defer mp.mu.RUnlock()
	if mp.closed {
		return &ProcessorError{
			Type:    ErrTypeValidation,
			Message: "processor is closed",
		}
	}
	return nil
}

// prepareOptions prepares and validates processor options
func (mp *modularProcessor) prepareOptions(opts ...*ProcessorOptions) *ProcessorOptions {
	if len(opts) == 0 {
		return &ProcessorOptions{}
	}
	return opts[0]
}

// validateInput validates JSON input string
func (mp *modularProcessor) validateInput(jsonStr string) error {
	if jsonStr == "" {
		return &ProcessorError{
			Type:    ErrTypeValidation,
			Message: "empty JSON string",
		}
	}
	return nil
}

// createCacheKey creates a cache key for the operation
func (mp *modularProcessor) createCacheKey(operation, jsonStr, path string, options *ProcessorOptions) CacheKey {
	return CacheKey{
		Operation: operation,
		JSONStr:   jsonStr,
		Path:      path,
		Options:   fmt.Sprintf("%+v", options),
	}
}

// setValueBasic provides a basic implementation for setting values
func (mp *modularProcessor) setValueBasic(data any, segments []PathSegmentInfo, value any, createPaths bool) error {
	if len(segments) == 0 {
		return fmt.Errorf("no segments to process")
	}

	// Navigate to parent segments
	current := data
	for i, segment := range segments[:len(segments)-1] {
		result, err := mp.navigator.NavigateToSegment(current, segment)
		if err != nil {
			return fmt.Errorf("failed to navigate to segment %d: %w", i, err)
		}

		if !result.Exists {
			if createPaths {
				// Create missing path segment
				newContainer, err := mp.createContainerForSegment(segments, i+1)
				if err != nil {
					return fmt.Errorf("failed to create container for segment %d: %w", i, err)
				}

				// Set the new container in the current object
				if err := mp.setValueInContainer(current, segment, newContainer); err != nil {
					return fmt.Errorf("failed to set container in segment %d: %w", i, err)
				}
				current = newContainer
			} else {
				return fmt.Errorf("path not found at segment %d", i)
			}
		} else {
			current = result.Value
		}
	}

	// Set the final value
	finalSegment := segments[len(segments)-1]
	return mp.setValueInContainer(current, finalSegment, value)
}

// createContainerForSegment creates an appropriate container for the next segment
func (mp *modularProcessor) createContainerForSegment(segments []PathSegmentInfo, nextIndex int) (any, error) {
	if nextIndex >= len(segments) {
		return make(map[string]any), nil
	}

	nextSegment := segments[nextIndex]
	switch nextSegment.Type {
	case "array", "slice":
		return make([]any, 0), nil
	default:
		return make(map[string]any), nil
	}
}

// setValueInContainer sets a value in a container based on segment type
func (mp *modularProcessor) setValueInContainer(container any, segment PathSegmentInfo, value any) error {
	switch segment.Type {
	case "property":
		return mp.setPropertyValue(container, segment.Key, value)
	case "array":
		return mp.setArrayValue(container, segment.Index, value)
	default:
		return fmt.Errorf("unsupported segment type for setting: %s", segment.Type)
	}
}

// setPropertyValue sets a property value in an object
func (mp *modularProcessor) setPropertyValue(container any, key string, value any) error {
	switch obj := container.(type) {
	case map[string]any:
		obj[key] = value
		return nil
	case map[any]any:
		obj[key] = value
		return nil
	default:
		return fmt.Errorf("cannot set property on type %T", container)
	}
}

// setArrayValue sets a value at an array index
func (mp *modularProcessor) setArrayValue(container any, index int, value any) error {
	arr, ok := container.([]any)
	if !ok {
		return fmt.Errorf("cannot set array value on type %T", container)
	}

	// Handle negative indices
	normalizedIndex := mp.arrayOps.HandleNegativeIndex(index, len(arr))

	// Check if we need to extend the array
	if normalizedIndex >= len(arr) {
		// Extend array to accommodate the index
		extended := mp.arrayOps.ExtendArray(arr, normalizedIndex+1)
		// Copy back to original slice (this is a limitation of this approach)
		copy(arr, extended)
		if len(extended) > len(arr) {
			return fmt.Errorf("cannot extend array in place")
		}
	}

	return mp.arrayOps.SetArrayElement(arr, normalizedIndex, value)
}

// RecursiveProcessor implements true recursive processing for all operations
type RecursiveProcessor struct {
	processor  *Processor
	arrayUtils *internal.ArrayUtils
}

// NewRecursiveProcessor creates a new unified recursive processor
func NewRecursiveProcessor(p *Processor) *RecursiveProcessor {
	return &RecursiveProcessor{
		processor:  p,
		arrayUtils: internal.NewArrayUtils(),
	}
}

// Use Operation types from interfaces.go

// ProcessRecursively performs recursive processing for any operation
func (urp *RecursiveProcessor) ProcessRecursively(data any, path string, operation Operation, value any) (any, error) {
	return urp.ProcessRecursivelyWithOptions(data, path, operation, value, false)
}

// ProcessRecursivelyWithOptions performs recursive processing with path creation options
func (urp *RecursiveProcessor) ProcessRecursivelyWithOptions(data any, path string, operation Operation, value any, createPaths bool) (any, error) {
	// Parse path into segments using cached parsing
	segments, err := urp.processor.getCachedPathSegments(path)
	if err != nil {
		return nil, fmt.Errorf("failed to parse path '%s': %w", path, err)
	}

	if len(segments) == 0 {
		switch operation {
		case OpGet:
			return data, nil
		case OpSet:
			return nil, fmt.Errorf("cannot set root value")
		case OpDelete:
			return nil, fmt.Errorf("cannot delete root value")
		}
	}

	// Start recursive processing from root
	result, err := urp.processRecursivelyAtSegmentsWithOptions(data, segments, 0, operation, value, createPaths)
	if err != nil {
		return nil, err
	}

	// Check if any segment in the path was a flat extraction
	// If so, we need special handling to apply flattening and subsequent operations correctly
	if operation == OpGet {
		// Find the LAST flat segment, not the first one
		// This is important for paths like orders{flat:items}{flat:tags}[0:3]
		flatSegmentIndex := -1
		for i, segment := range segments {
			if segment.Type == internal.ExtractSegment && segment.IsFlat {
				flatSegmentIndex = i // Keep updating to find the last one
			}
		}

		if flatSegmentIndex >= 0 {
			// Check if there are any operations after the flat extraction
			hasPostFlatOps := flatSegmentIndex+1 < len(segments)

			if hasPostFlatOps {
				// There are operations after flat extraction - need special handling
				// Process the path in two phases:
				// Phase 1: Process up to and including the flat segment
				// Phase 2: Apply flattening and then process remaining segments

				// Step 1: Process up to and including the flat segment
				preFlatSegments := segments[:flatSegmentIndex+1]
				preFlatResult, err := urp.processRecursivelyAtSegmentsWithOptions(data, preFlatSegments, 0, operation, value, createPaths)
				if err != nil {
					return nil, err
				}

				// Step 2: Apply flattening to the pre-flat result
				var flattened []any
				if resultArray, ok := preFlatResult.([]any); ok {
					urp.deepFlattenResults(resultArray, &flattened)
				} else {
					flattened = []any{preFlatResult}
				}

				// Step 3: Process remaining segments on the flattened result
				postFlatSegments := segments[flatSegmentIndex+1:]
				if len(postFlatSegments) > 0 {
					finalResult, err := urp.processRecursivelyAtSegmentsWithOptions(flattened, postFlatSegments, 0, operation, value, createPaths)
					if err != nil {
						return nil, err
					}
					return finalResult, nil
				}

				return flattened, nil
			} else {
				// No operations after flat extraction - the flat extraction should have been handled
				// during normal processing, so just return the result as-is
				return result, nil
			}
		}
	}

	return result, nil
}

// processRecursivelyAtSegments recursively processes path segments for any operation
func (urp *RecursiveProcessor) processRecursivelyAtSegments(data any, segments []internal.PathSegment, segmentIndex int, operation Operation, value any) (any, error) {
	return urp.processRecursivelyAtSegmentsWithOptions(data, segments, segmentIndex, operation, value, false)
}

// processRecursivelyAtSegmentsWithOptions recursively processes path segments with path creation options
func (urp *RecursiveProcessor) processRecursivelyAtSegmentsWithOptions(data any, segments []internal.PathSegment, segmentIndex int, operation Operation, value any, createPaths bool) (any, error) {
	// Base case: no more segments to process
	if segmentIndex >= len(segments) {
		switch operation {
		case OpGet:
			return data, nil
		case OpSet:
			return nil, fmt.Errorf("cannot set value: no target segment")
		case OpDelete:
			return nil, fmt.Errorf("cannot delete value: no target segment")
		}
	}

	// Check for extract-then-slice pattern
	if segmentIndex < len(segments)-1 {
		currentSegment := segments[segmentIndex]
		nextSegment := segments[segmentIndex+1]

		// Special handling for {extract}[slice] pattern
		if currentSegment.Type == internal.ExtractSegment && nextSegment.Type == internal.ArraySliceSegment {
			return urp.handleExtractThenSlice(data, currentSegment, nextSegment, segments, segmentIndex, operation, value)
		}
	}

	currentSegment := segments[segmentIndex]
	isLastSegment := segmentIndex == len(segments)-1

	switch currentSegment.Type {
	case internal.PropertySegment:
		return urp.handlePropertySegmentUnified(data, currentSegment, segments, segmentIndex, isLastSegment, operation, value, createPaths)

	case internal.ArrayIndexSegment:
		return urp.handleArrayIndexSegmentUnified(data, currentSegment, segments, segmentIndex, isLastSegment, operation, value, createPaths)

	case internal.ArraySliceSegment:
		return urp.handleArraySliceSegmentUnified(data, currentSegment, segments, segmentIndex, isLastSegment, operation, value, createPaths)

	case internal.ExtractSegment:
		return urp.handleExtractSegmentUnified(data, currentSegment, segments, segmentIndex, isLastSegment, operation, value, createPaths)

	case internal.WildcardSegment:
		return urp.handleWildcardSegmentUnified(data, currentSegment, segments, segmentIndex, isLastSegment, operation, value, createPaths)

	default:
		return nil, fmt.Errorf("unsupported segment type: %v", currentSegment.Type)
	}
}

// handlePropertySegmentUnified handles property access segments for all operations
func (urp *RecursiveProcessor) handlePropertySegmentUnified(data any, segment internal.PathSegment, segments []internal.PathSegment, segmentIndex int, isLastSegment bool, operation Operation, value any, createPaths bool) (any, error) {
	switch container := data.(type) {
	case map[string]any:
		if isLastSegment {
			switch operation {
			case OpGet:
				if val, exists := container[segment.Key]; exists {
					return val, nil
				}
				// Property doesn't exist - return ErrPathNotFound as documented
				return nil, ErrPathNotFound
			case OpSet:
				container[segment.Key] = value
				return value, nil
			case OpDelete:
				delete(container, segment.Key)
				return nil, nil
			}
		}

		// Recursively process next segment
		if nextValue, exists := container[segment.Key]; exists {
			return urp.processRecursivelyAtSegmentsWithOptions(nextValue, segments, segmentIndex+1, operation, value, createPaths)
		}

		// Handle path creation for Set operations
		if operation == OpSet && createPaths {
			// Create missing path segment
			nextSegment := segments[segmentIndex+1]
			var newContainer any

			switch nextSegment.Type {
			case internal.ArrayIndexSegment:
				// For array index, create array with sufficient size
				requiredSize := nextSegment.Index + 1
				if requiredSize < 0 {
					requiredSize = 1
				}
				newContainer = make([]any, requiredSize)
			case internal.ArraySliceSegment:
				// For array slice, create array with sufficient size based on slice end
				requiredSize := 0
				if nextSegment.End != nil {
					requiredSize = *nextSegment.End
				}
				if requiredSize <= 0 {
					requiredSize = 1
				}
				newContainer = make([]any, requiredSize)
			default:
				newContainer = make(map[string]any)
			}

			container[segment.Key] = newContainer
			return urp.processRecursivelyAtSegmentsWithOptions(newContainer, segments, segmentIndex+1, operation, value, createPaths)
		}

		// Path doesn't exist and we're not creating paths
		if operation == OpSet {
			return nil, fmt.Errorf("path not found: %s", segment.Key)
		}

		// For Get operation, return ErrPathNotFound as documented
		if operation == OpGet {
			return nil, ErrPathNotFound
		}

		return nil, nil // Property doesn't exist for Delete

	case []any:
		// Apply property access to each array element recursively
		var results []any
		var errors []error

		for i, item := range container {
			result, err := urp.handlePropertySegmentUnified(item, segment, segments, segmentIndex, isLastSegment, operation, value, createPaths)
			if err != nil {
				errors = append(errors, err)
				continue
			}

			if operation == OpGet && result != nil {
				results = append(results, result)
			} else if operation == OpSet {
				container[i] = item // Item was modified in place
			}
		}

		if operation == OpGet {
			if len(results) == 0 {
				return nil, nil
			}
			return results, nil
		}

		return nil, urp.combineErrors(errors)

	default:
		if operation == OpGet {
			return nil, nil // Cannot access property on non-object/array
		}
		return nil, fmt.Errorf("cannot access property '%s' on type %T", segment.Key, data)
	}
}

// handleArrayIndexSegmentUnified handles array index access segments for all operations
func (urp *RecursiveProcessor) handleArrayIndexSegmentUnified(data any, segment internal.PathSegment, segments []internal.PathSegment, segmentIndex int, isLastSegment bool, operation Operation, value any, createPaths bool) (any, error) {
	switch container := data.(type) {
	case []any:
		// Determine if this should be a distributed operation based on actual data structure
		// A distributed operation is needed when we have nested arrays that need individual processing
		shouldUseDistributed := segment.IsDistributed && urp.shouldUseDistributedArrayOperation(container)

		if shouldUseDistributed {
			// For distributed operations, apply the index to each element in the container
			var results []any
			var errors []error

			for _, item := range container {
				// Find the actual target array for distributed operation
				targetArray := urp.findTargetArrayForDistributedOperation(item)
				if targetArray != nil {
					// Apply index operation to this array
					index := urp.arrayUtils.NormalizeIndex(segment.Index, len(targetArray))
					if index < 0 || index >= len(targetArray) {
						if operation == OpGet {
							continue // Skip out of bounds items
						}
						errors = append(errors, fmt.Errorf("array index %d out of bounds (length %d)", segment.Index, len(targetArray)))
						continue
					}

					if isLastSegment {
						switch operation {
						case OpGet:
							// Get the result from the target array
							result := targetArray[index]

							// For distributed array operations, unwrap single element results for flattening
							// This mimics the behavior of the original getValueWithDistributedOperation
							if !segment.IsSlice {
								// For index operations (not slice), add the result directly
								// This will be a single value like "Alice", not an array
								results = append(results, result)
							} else {
								// For slice operations, add the result as-is (could be an array)
								results = append(results, result)
							}
						case OpSet:
							targetArray[index] = value
						case OpDelete:
							targetArray[index] = deletedMarker
						}
					} else {
						// Recursively process next segment
						result, err := urp.processRecursivelyAtSegmentsWithOptions(targetArray[index], segments, segmentIndex+1, operation, value, createPaths)
						if err != nil {
							errors = append(errors, err)
							continue
						}
						if operation == OpGet && result != nil {
							results = append(results, result)
						}
					}
				}
			}

			if operation == OpGet {
				// For distributed array operations, flatten the results to match expected behavior
				// This mimics the behavior of the original getValueWithDistributedOperation
				if isLastSegment && !segment.IsSlice {
					// Return flattened results for distributed array index operations
					return results, nil
				}
				return results, nil
			}
			return nil, urp.combineErrors(errors)
		}

		// Non-distributed operation - standard array index access
		index := urp.arrayUtils.NormalizeIndex(segment.Index, len(container))
		if index < 0 || index >= len(container) {
			if operation == OpGet {
				return nil, nil // Index out of bounds
			}
			if operation == OpSet && createPaths && index >= 0 {
				// Array extension required
				return nil, fmt.Errorf("array extension required for index %d on array length %d", index, len(container))
			}
			return nil, fmt.Errorf("array index %d out of bounds (length %d)", segment.Index, len(container))
		}

		if isLastSegment {
			switch operation {
			case OpGet:
				return container[index], nil
			case OpSet:
				container[index] = value
				return value, nil
			case OpDelete:
				// Mark for deletion (will be cleaned up later)
				container[index] = deletedMarker
				return nil, nil
			}
		}

		// Recursively process next segment
		return urp.processRecursivelyAtSegmentsWithOptions(container[index], segments, segmentIndex+1, operation, value, createPaths)

	case map[string]any:
		// Apply array index to each map value recursively
		var results []any
		var errors []error

		for key, mapValue := range container {
			result, err := urp.handleArrayIndexSegmentUnified(mapValue, segment, segments, segmentIndex, isLastSegment, operation, value, createPaths)
			if err != nil {
				errors = append(errors, err)
				continue
			}

			if operation == OpGet && result != nil {
				results = append(results, result)
			} else if operation == OpSet {
				container[key] = mapValue // Value was modified in place
			}
		}

		if operation == OpGet {
			if len(results) == 0 {
				return nil, nil
			}
			return results, nil
		}

		return nil, urp.combineErrors(errors)

	default:
		// Cannot perform array index access on non-array types
		return nil, fmt.Errorf("cannot access array index [%d] on type %T", segment.Index, data)
	}
}

// handleArraySliceSegmentUnified handles array slice segments for all operations
func (urp *RecursiveProcessor) handleArraySliceSegmentUnified(data any, segment internal.PathSegment, segments []internal.PathSegment, segmentIndex int, isLastSegment bool, operation Operation, value any, createPaths bool) (any, error) {
	switch container := data.(type) {
	case []any:
		// Check if this should be a distributed operation
		shouldUseDistributed := segment.IsDistributed && urp.shouldUseDistributedArrayOperation(container)

		if shouldUseDistributed {
			// Distributed slice operation - apply slice to each array element
			var results []any
			var errors []error

			for _, item := range container {
				targetArray := urp.findTargetArrayForDistributedOperation(item)
				if targetArray == nil {
					continue // Skip non-array items
				}

				var startVal, endVal int
				if segment.Start != nil {
					startVal = *segment.Start
				} else {
					startVal = 0
				}
				if segment.End != nil {
					endVal = *segment.End
				} else {
					endVal = len(targetArray)
				}

				if isLastSegment {
					switch operation {
					case OpGet:
						// Use the array utils for proper slicing with step support
						startPtr := &startVal
						endPtr := &endVal
						if segment.Start == nil {
							startPtr = nil
						}
						if segment.End == nil {
							endPtr = nil
						}
						sliceResult := urp.arrayUtils.PerformArraySlice(targetArray, startPtr, endPtr, segment.Step)
						results = append(results, sliceResult)
					case OpSet:
						// For distributed set operations on slices, we need special handling
						return nil, fmt.Errorf("distributed set operations on slices not yet supported")
					case OpDelete:
						// For distributed delete operations on slices, we need special handling
						return nil, fmt.Errorf("distributed delete operations on slices not yet supported")
					}
				} else {
					// Recursively process next segment on sliced result
					startPtr := &startVal
					endPtr := &endVal
					if segment.Start == nil {
						startPtr = nil
					}
					if segment.End == nil {
						endPtr = nil
					}
					sliceResult := urp.arrayUtils.PerformArraySlice(targetArray, startPtr, endPtr, segment.Step)

					result, err := urp.processRecursivelyAtSegmentsWithOptions(sliceResult, segments, segmentIndex+1, operation, value, createPaths)
					if err != nil {
						errors = append(errors, err)
						continue
					}
					if operation == OpGet && result != nil {
						results = append(results, result)
					}
				}
			}

			if len(errors) > 0 {
				return nil, urp.combineErrors(errors)
			}

			if operation == OpGet {
				return results, nil
			}
			return nil, nil
		}

		// Non-distributed slice operation
		var startVal, endVal int
		if segment.Start != nil {
			startVal = *segment.Start
		} else {
			startVal = 0
		}
		if segment.End != nil {
			endVal = *segment.End
		} else {
			endVal = len(container)
		}

		start, end := urp.arrayUtils.NormalizeSlice(startVal, endVal, len(container))

		if isLastSegment {
			switch operation {
			case OpGet:
				// Use the array utils for proper slicing with step support
				startPtr := &startVal
				endPtr := &endVal
				if segment.Start == nil {
					startPtr = nil
				}
				if segment.End == nil {
					endPtr = nil
				}
				return urp.arrayUtils.PerformArraySlice(container, startPtr, endPtr, segment.Step), nil
			case OpSet:
				// Check if we need to extend the array for slice assignment
				if end > len(container) && createPaths {
					// For array slice extension, we need to fall back to legacy handling
					// because the unified processor can't modify parent references directly
					return nil, fmt.Errorf("array slice extension required: use legacy handling for path with slice [%d:%d] on array length %d", start, end, len(container))
				}

				// Set value to all elements in slice
				for i := start; i < end && i < len(container); i++ {
					container[i] = value
				}
				return value, nil
			case OpDelete:
				// Mark elements in slice for deletion
				for i := start; i < end && i < len(container); i++ {
					container[i] = deletedMarker
				}
				return nil, nil
			}
		}

		// For non-last segments, we need to decide whether to:
		// 1. Apply slice first, then process remaining segments on each sliced element
		// 2. Process remaining segments on each element, then apply slice to results

		// The correct behavior depends on the context:
		// If this slice comes after an extraction, we should slice the extracted results
		// If this slice comes before further processing, we should slice first then process

		// Apply slice first, then process remaining segments
		slicedContainer := container[start:end]

		if len(slicedContainer) == 0 {
			return []any{}, nil
		}

		// Process remaining segments on each sliced element
		var results []any
		var errors []error

		for i, item := range slicedContainer {
			result, err := urp.processRecursivelyAtSegmentsWithOptions(item, segments, segmentIndex+1, operation, value, createPaths)
			if err != nil {
				errors = append(errors, err)
				continue
			}

			if operation == OpGet && result != nil {
				results = append(results, result)
			} else if operation == OpSet {
				slicedContainer[i] = item // Item was modified in place
			}
		}

		if operation == OpGet {
			return results, nil
		}

		return nil, urp.combineErrors(errors)

	case map[string]any:
		// Apply array slice to each map value recursively
		var results []any
		var errors []error

		for key, mapValue := range container {
			result, err := urp.handleArraySliceSegmentUnified(mapValue, segment, segments, segmentIndex, isLastSegment, operation, value, createPaths)
			if err != nil {
				errors = append(errors, err)
				continue
			}

			if operation == OpGet && result != nil {
				// Preserve structure for map values - don't flatten
				results = append(results, result)
			} else if operation == OpSet {
				container[key] = mapValue // Value was modified in place
			}
		}

		if operation == OpGet {
			return results, nil
		}

		return nil, urp.combineErrors(errors)

	default:
		if operation == OpGet {
			return nil, nil // Cannot slice non-array
		}
		return nil, fmt.Errorf("cannot slice type %T", data)
	}
}

// handleExtractSegmentUnified handles extraction segments for all operations
func (urp *RecursiveProcessor) handleExtractSegmentUnified(data any, segment internal.PathSegment, segments []internal.PathSegment, segmentIndex int, isLastSegment bool, operation Operation, value any, createPaths bool) (any, error) {
	// Check for special flat extraction syntax - use the IsFlat flag from parsing
	isFlat := segment.IsFlat
	actualKey := segment.Key
	if isFlat {
		// The key should already be cleaned by the parser, but double-check
		actualKey = strings.TrimPrefix(actualKey, "flat:")
	}

	switch container := data.(type) {
	case []any:
		// Extract from each array element
		var results []any
		var errors []error

		for i, item := range container {
			if itemMap, ok := item.(map[string]any); ok {
				if isLastSegment {
					switch operation {
					case OpGet:
						if val, exists := itemMap[actualKey]; exists {
							if isFlat {
								// Flatten the result if it's an array
								if valArray, ok := val.([]any); ok {
									results = append(results, valArray...)
								} else {
									results = append(results, val)
								}
							} else {
								results = append(results, val)
							}
						}
					case OpSet:
						itemMap[actualKey] = value
					case OpDelete:
						delete(itemMap, actualKey)
					}
				} else {
					// For non-last segments, we need to handle array operations specially
					if extractedValue, exists := itemMap[actualKey]; exists {
						if operation == OpGet {
							// Check if the next segment is an array operation
							nextSegmentIndex := segmentIndex + 1
							if nextSegmentIndex < len(segments) && segments[nextSegmentIndex].Type == internal.ArrayIndexSegment {
								// For array operations following extraction, collect values first
								results = append(results, extractedValue)
							} else {
								// For non-array operations, process recursively
								result, err := urp.processRecursivelyAtSegmentsWithOptions(extractedValue, segments, segmentIndex+1, operation, value, createPaths)
								if err != nil {
									errors = append(errors, err)
									continue
								}
								if result != nil {
									results = append(results, result)
								}
							}
						} else if operation == OpDelete {
							// For Delete operations on extraction paths, check if this is the last extraction
							// followed by array/slice operation
							nextSegmentIndex := segmentIndex + 1
							isLastExtraction := true

							// Check if there are more extraction segments after this one
							for i := nextSegmentIndex; i < len(segments); i++ {
								if segments[i].Type == internal.ExtractSegment {
									isLastExtraction = false
									break
								}
							}

							if isLastExtraction && nextSegmentIndex < len(segments) {
								nextSegment := segments[nextSegmentIndex]
								if nextSegment.Type == internal.ArrayIndexSegment || nextSegment.Type == internal.ArraySliceSegment {
									// For delete operations like {tasks}[0], we need to check if the extracted value is an array
									// If it's an array, delete from the array; if it's a scalar, delete the field
									if _, isArray := extractedValue.([]any); isArray {
										// The extracted value is an array, apply the array operation to it
										_, err := urp.processRecursivelyAtSegmentsWithOptions(extractedValue, segments, segmentIndex+1, operation, value, createPaths)
										if err != nil {
											errors = append(errors, err)
											continue
										}
									} else {
										// The extracted value is a scalar, delete the field itself
										// This matches the expected behavior for scalar fields like {name}[0]
										delete(itemMap, actualKey)
									}
								} else {
									// For other delete operations, process recursively
									_, err := urp.processRecursivelyAtSegmentsWithOptions(extractedValue, segments, segmentIndex+1, operation, value, createPaths)
									if err != nil {
										errors = append(errors, err)
										continue
									}
								}
							} else {
								// For other delete operations, process recursively
								_, err := urp.processRecursivelyAtSegmentsWithOptions(extractedValue, segments, segmentIndex+1, operation, value, createPaths)
								if err != nil {
									errors = append(errors, err)
									continue
								}
							}
						} else {
							// For Set operations, always process recursively
							_, err := urp.processRecursivelyAtSegmentsWithOptions(extractedValue, segments, segmentIndex+1, operation, value, createPaths)
							if err != nil {
								errors = append(errors, err)
								continue
							}
							if operation == OpSet {
								container[i] = item // Item was modified in place
							}
						}
					}
				}
			}
		}

		if operation == OpGet {
			// If this is not the last segment and we have collected results for array operations
			if !isLastSegment && len(results) > 0 {
				nextSegmentIndex := segmentIndex + 1
				if nextSegmentIndex < len(segments) && segments[nextSegmentIndex].Type == internal.ArrayIndexSegment {
					// Process the collected results with the remaining segments
					result, err := urp.processRecursivelyAtSegmentsWithOptions(results, segments, nextSegmentIndex, operation, value, createPaths)
					if err != nil {
						return nil, err
					}

					// For distributed array operations, apply deep flattening to match expected behavior
					// This flattens nested arrays from distributed operations like {name}[0]
					if resultArray, ok := result.([]any); ok {
						// Check if the next segment is an array index operation (not slice)
						nextSegment := segments[nextSegmentIndex]
						if nextSegment.Type == internal.ArrayIndexSegment && !nextSegment.IsSlice {
							// For array index operations, apply deep flattening
							flattened := urp.deepFlattenDistributedResults(resultArray)
							return flattened, nil
						}
					}
					return result, nil
				}
			}

			// Apply flattening if this was a flat extraction
			if isFlat && len(results) > 0 {
				var flattened []any
				urp.deepFlattenResults(results, &flattened)
				return flattened, nil
			}

			// For distributed operations that end with array index operations, apply deep flattening
			// This handles cases like {name}[0] where we want ["Alice", "David", "Frank"] not [["Alice", "David"], ["Frank"]]
			// Only apply this for paths that have multiple extraction segments followed by array operations
			if len(results) > 0 && len(segments) > 0 {
				lastSegment := segments[len(segments)-1]
				if lastSegment.Type == internal.ArrayIndexSegment && !lastSegment.IsSlice {
					// Count extraction segments to determine if deep flattening is needed
					extractionCount := 0
					for _, seg := range segments {
						if seg.Type == internal.ExtractSegment {
							extractionCount++
						}
					}

					// Only apply deep flattening for multi-level extractions like {teams}{members}{name}[0]
					// Don't apply it for simple extractions like {name} which should preserve structure
					if extractionCount >= 3 {
						flattened := urp.deepFlattenDistributedResults(results)
						return flattened, nil
					}
				}
			}

			return results, nil
		}

		return nil, urp.combineErrors(errors)

	case map[string]any:
		if isLastSegment {
			switch operation {
			case OpGet:
				if val, exists := container[actualKey]; exists {
					if isFlat {
						// Flatten the result if it's an array
						if valArray, ok := val.([]any); ok {
							return valArray, nil // Return flattened array
						}
					}
					return val, nil
				}
				return nil, nil
			case OpSet:
				container[actualKey] = value
				return value, nil
			case OpDelete:
				delete(container, actualKey)
				return nil, nil
			}
		}

		// Recursively process extracted value
		if extractedValue, exists := container[actualKey]; exists {
			return urp.processRecursivelyAtSegmentsWithOptions(extractedValue, segments, segmentIndex+1, operation, value, createPaths)
		}

		return nil, nil

	default:
		if operation == OpGet {
			return nil, nil // Cannot extract from non-object/array
		}
		return nil, fmt.Errorf("cannot extract from type %T", data)
	}
}

// handleWildcardSegmentUnified handles wildcard segments for all operations
func (urp *RecursiveProcessor) handleWildcardSegmentUnified(data any, segment internal.PathSegment, segments []internal.PathSegment, segmentIndex int, isLastSegment bool, operation Operation, value any, createPaths bool) (any, error) {
	switch container := data.(type) {
	case []any:
		if isLastSegment {
			switch operation {
			case OpGet:
				return container, nil
			case OpSet:
				// Set value to all array elements
				for i := range container {
					container[i] = value
				}
				return value, nil
			case OpDelete:
				// Mark all array elements for deletion
				for i := range container {
					container[i] = deletedMarker
				}
				return nil, nil
			}
		}

		// Recursively process all array elements
		var results []any
		var errors []error

		for i, item := range container {
			result, err := urp.processRecursivelyAtSegmentsWithOptions(item, segments, segmentIndex+1, operation, value, createPaths)
			if err != nil {
				errors = append(errors, err)
				continue
			}

			if operation == OpGet && result != nil {
				// Preserve structure - don't flatten unless explicitly requested
				results = append(results, result)
			} else if operation == OpSet {
				container[i] = item // Item was modified in place
			}
		}

		if operation == OpGet {
			return results, nil
		}

		return nil, urp.combineErrors(errors)

	case map[string]any:
		if isLastSegment {
			switch operation {
			case OpGet:
				var results []any
				for _, val := range container {
					results = append(results, val)
				}
				return results, nil
			case OpSet:
				// Set value to all map entries
				for key := range container {
					container[key] = value
				}
				return value, nil
			case OpDelete:
				// Delete all map entries
				for key := range container {
					delete(container, key)
				}
				return nil, nil
			}
		}

		// Recursively process all map values
		var results []any
		var errs []error

		for key, mapValue := range container {
			result, err := urp.processRecursivelyAtSegmentsWithOptions(mapValue, segments, segmentIndex+1, operation, value, createPaths)
			if err != nil {
				errs = append(errs, err)
				continue
			}

			if operation == OpGet && result != nil {
				// Preserve structure - don't flatten unless explicitly requested
				results = append(results, result)
			} else if operation == OpSet {
				container[key] = mapValue // Value was modified in place
			}
		}

		if operation == OpGet {
			return results, nil
		}

		return nil, urp.combineErrors(errs)

	default:
		if operation == OpGet {
			return nil, nil // Cannot wildcard non-container
		}
		return nil, fmt.Errorf("cannot apply wildcard to type %T", data)
	}
}

// handleExtractThenSlice handles the special case of {extract}[slice] pattern
func (urp *RecursiveProcessor) handleExtractThenSlice(data any, extractSegment, sliceSegment internal.PathSegment, segments []internal.PathSegment, segmentIndex int, operation Operation, value any) (any, error) {
	// For Delete operations on {extract}[slice] patterns, we need to apply the slice operation
	// to each extracted array individually, not to the collection of extracted results
	if operation == OpDelete {
		return urp.handleExtractThenSliceDelete(data, extractSegment, sliceSegment, segments, segmentIndex, value)
	}

	// For Get operations, use the original logic
	var extractedResults []any

	switch container := data.(type) {
	case []any:
		// Extract from each array element
		for _, item := range container {
			if itemMap, ok := item.(map[string]any); ok {
				if val, exists := itemMap[extractSegment.Key]; exists {
					extractedResults = append(extractedResults, val)
				}
			}
		}
	case map[string]any:
		// Extract from single object
		if val, exists := container[extractSegment.Key]; exists {
			extractedResults = append(extractedResults, val)
		}
	default:
		return nil, fmt.Errorf("cannot extract from type %T", data)
	}

	// Now apply the slice to the extracted results
	if len(extractedResults) > 0 {
		var startVal, endVal int
		if sliceSegment.Start != nil {
			startVal = *sliceSegment.Start
		} else {
			startVal = 0
		}
		if sliceSegment.End != nil {
			endVal = *sliceSegment.End
		} else {
			endVal = len(extractedResults)
		}

		start, end := urp.arrayUtils.NormalizeSlice(startVal, endVal, len(extractedResults))

		// Check if this is the last operation (extract + slice)
		isLastOperation := segmentIndex+2 >= len(segments)

		if isLastOperation {
			// Final result: slice the extracted data
			if start >= len(extractedResults) || end <= 0 || start >= end {
				return []any{}, nil
			}
			return extractedResults[start:end], nil
		} else {
			// More segments to process: slice first, then continue processing
			if start >= len(extractedResults) || end <= 0 || start >= end {
				return []any{}, nil
			}

			slicedData := extractedResults[start:end]

			// Process remaining segments on each sliced element
			var results []any
			var errs []error

			for _, item := range slicedData {
				result, err := urp.processRecursivelyAtSegmentsWithOptions(item, segments, segmentIndex+2, operation, value, false)
				if err != nil {
					errs = append(errs, err)
					continue
				}

				if operation == OpGet && result != nil {
					results = append(results, result)
				}
			}

			if operation == OpGet {
				return results, nil
			}

			return nil, urp.combineErrors(errs)
		}
	}

	// No extraction results
	return []any{}, nil
}

// handleExtractThenSliceDelete handles Delete operations for {extract}[slice] patterns
func (urp *RecursiveProcessor) handleExtractThenSliceDelete(data any, extractSegment, sliceSegment internal.PathSegment, segments []internal.PathSegment, segmentIndex int, value any) (any, error) {
	switch container := data.(type) {
	case []any:
		// Apply slice deletion to each extracted array
		var errs []error
		for _, item := range container {
			if itemMap, ok := item.(map[string]any); ok {
				if extractedValue, exists := itemMap[extractSegment.Key]; exists {
					if extractedArray, isArray := extractedValue.([]any); isArray {
						// Apply slice deletion to this array
						err := urp.applySliceDeletion(extractedArray, sliceSegment)
						if err != nil {
							errs = append(errs, err)
							continue
						}
						// Update the array in the map
						itemMap[extractSegment.Key] = extractedArray
					}
				}
			}
		}
		return nil, urp.combineErrors(errs)
	case map[string]any:
		// Apply slice deletion to single extracted array
		if extractedValue, exists := container[extractSegment.Key]; exists {
			if extractedArray, isArray := extractedValue.([]any); isArray {
				err := urp.applySliceDeletion(extractedArray, sliceSegment)
				if err != nil {
					return nil, err
				}
				container[extractSegment.Key] = extractedArray
			}
		}
		return nil, nil
	default:
		return nil, fmt.Errorf("cannot extract from type %T", data)
	}
}

// applySliceDeletion applies slice deletion to an array
func (urp *RecursiveProcessor) applySliceDeletion(arr []any, sliceSegment internal.PathSegment) error {
	var startVal, endVal int
	if sliceSegment.Start != nil {
		startVal = *sliceSegment.Start
	} else {
		startVal = 0
	}
	if sliceSegment.End != nil {
		endVal = *sliceSegment.End
	} else {
		endVal = len(arr)
	}

	start, end := urp.arrayUtils.NormalizeSlice(startVal, endVal, len(arr))

	// Mark elements in slice for deletion
	for i := start; i < end && i < len(arr); i++ {
		arr[i] = deletedMarker
	}

	return nil
}

// combineErrors combines multiple errors into a single error using modern Go 1.24+ patterns
func (urp *RecursiveProcessor) combineErrors(errs []error) error {
	if len(errs) == 0 {
		return nil
	}

	// Filter out nil errors
	var validErrors []error
	for _, err := range errs {
		if err != nil {
			validErrors = append(validErrors, err)
		}
	}

	if len(validErrors) == 0 {
		return nil
	}

	// Use errors.Join() for modern error composition (Go 1.20+)
	return errors.Join(validErrors...)
}

// findTargetArrayForDistributedOperation finds the actual target array for distributed operations
// This handles nested array structures that may result from extraction operations
func (urp *RecursiveProcessor) findTargetArrayForDistributedOperation(item any) []any {
	// If item is directly an array, return it
	if arr, ok := item.([]any); ok {
		// Check if this array contains only one element that is also an array
		// This handles the case where extraction creates nested structures like [[[members]]]
		if len(arr) == 1 {
			if nestedArr, ok := arr[0].([]any); ok {
				// Check if the nested array contains objects (actual data)
				// vs another level of nesting
				if len(nestedArr) > 0 {
					if _, ok := nestedArr[0].(map[string]any); ok {
						// This is the target array containing objects
						return nestedArr
					} else if _, ok := nestedArr[0].([]any); ok {
						// Another level of nesting, recurse
						return urp.findTargetArrayForDistributedOperation(nestedArr)
					} else {
						// This is the target array containing primitive values (like strings)
						return nestedArr
					}
				}
				// Return the nested array even if empty
				return nestedArr
			}
		}
		// Return the array as-is if it doesn't match the nested pattern
		return arr
	}

	// If item is not an array, return nil
	return nil
}

// deepFlattenDistributedResults performs deep flattening of distributed operation results
// This handles nested array structures like [["Alice", "David"], ["Frank"]] -> ["Alice", "David", "Frank"]
func (urp *RecursiveProcessor) deepFlattenDistributedResults(results []any) []any {
	var flattened []any

	for _, item := range results {
		if itemArray, ok := item.([]any); ok {
			// Recursively flatten nested arrays
			for _, nestedItem := range itemArray {
				if nestedArray, ok := nestedItem.([]any); ok {
					// Another level of nesting, flatten it
					flattened = append(flattened, nestedArray...)
				} else {
					// This is a leaf value, add it directly
					flattened = append(flattened, nestedItem)
				}
			}
		} else {
			// This is a leaf value, add it directly
			flattened = append(flattened, item)
		}
	}

	return flattened
}

// deepFlattenResults recursively flattens nested arrays into a single flat array
// This is used for flat: extraction syntax to completely flatten all nested structures
func (urp *RecursiveProcessor) deepFlattenResults(results []any, flattened *[]any) {
	for _, result := range results {
		if resultArray, ok := result.([]any); ok {
			// Recursively flatten nested arrays
			urp.deepFlattenResults(resultArray, flattened)
		} else {
			// Add non-array items directly
			*flattened = append(*flattened, result)
		}
	}
}

// shouldUseDistributedArrayOperation determines if an array operation should be distributed
// based on the actual data structure
func (urp *RecursiveProcessor) shouldUseDistributedArrayOperation(container []any) bool {
	// If the container is empty, no distributed operation needed
	if len(container) == 0 {
		return false
	}

	// For extraction results, we typically have nested structures like:
	// - [[[item1, item2]]] for field extraction results
	// - [[[array1], [array2]]] for array field extraction results

	// Check if this looks like an extraction result structure
	// Extraction results typically have nested arrays at multiple levels
	for _, item := range container {
		if arr, ok := item.([]any); ok {
			// If we have nested arrays, this is likely an extraction result
			// that needs distributed operation
			if len(arr) == 1 {
				if _, ok := arr[0].([]any); ok {
					// This is a nested structure like [[items]]
					// Use distributed operation regardless of content type
					return true
				}
			}
			// If the array has multiple elements, this is definitely an extraction result
			// that needs distributed operation (like string arrays from field extraction)
			if len(arr) > 0 {
				// This handles cases like {name}[0] where we have string arrays
				// and need to apply the index operation to each string array
				return true
			}
		}
	}

	// Default to normal indexing for simple cases
	return false
}

// ============================================================================
// Note: ProcessorEnhancements removed - incomplete experimental code
// Use standard Processor with appropriate Config settings for security
// ============================================================================
