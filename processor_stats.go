package json

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync/atomic"
	"time"

	"github.com/cybergodev/json/internal"
)

// checkClosed returns an error if the processor is closed or closing.
// Returns ErrProcessorClosed when called on a nil Processor to prevent
// nil-pointer panics on every public method that delegates here.
func (p *Processor) checkClosed() error {
	if p == nil {
		return &JsonsError{Op: "check_closed", Message: "processor is nil", Err: ErrProcessorClosed}
	}
	state := atomic.LoadInt32(&p.state)
	if state != processorStateActive {
		msg := "processor is closed"
		if state == processorStateClosing {
			msg = "processor is closing"
		}
		return &JsonsError{Op: "check_closed", Message: msg, Err: ErrProcessorClosed}
	}
	return nil
}

// ProcessBatch processes multiple operations in a single batch
func (p *Processor) ProcessBatch(operations []BatchOperation, cfg ...Config) ([]BatchResult, error) {
	if err := p.checkClosed(); err != nil {
		return nil, err
	}

	options, err := p.prepareOptions(cfg...)
	if err != nil {
		return nil, err
	}
	defer releaseConfig(options)

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
			result.Result, result.Error = p.Get(op.JSONStr, op.Path, cfg...)
		case "set":
			result.Result, result.Error = p.Set(op.JSONStr, op.Path, op.Value, cfg...)
		case "delete":
			result.Result, result.Error = p.Delete(op.JSONStr, op.Path, cfg...)
		case "validate":
			valid, err := p.Valid(op.JSONStr, cfg...)
			result.Result = map[string]any{"valid": valid}
			result.Error = err
		default:
			result.Error = fmt.Errorf("unknown operation type: %s", op.Type)
		}

		results[i] = result
	}

	return results, nil
}

// WarmupCache pre-loads commonly used paths into cache to improve first-access performance
func (p *Processor) WarmupCache(jsonStr string, paths []string, cfg ...Config) (*WarmupResult, error) {
	if err := p.checkClosed(); err != nil {
		return nil, err
	}

	if !p.config.EnableCache {
		return nil, &JsonsError{
			Op:      "warmup_cache",
			Message: "cache is disabled, cannot warmup cache",
			Err:     errCacheDisabled,
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
	options, err := p.prepareOptions(cfg...)
	if err != nil {
		return nil, &JsonsError{
			Op:      "warmup_cache",
			Message: "invalid options for cache warmup",
			Err:     err,
		}
	}
	defer releaseConfig(options)

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
		// options is guaranteed non-nil after prepareOptions()
		_, err := p.Get(jsonStr, path, *options)
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

// GetStats returns processor performance statistics
func (p *Processor) GetStats() Stats {
	if p == nil {
		return Stats{}
	}

	var cacheStats internal.CacheStats
	if p.cache != nil {
		cacheStats = p.cache.GetStats()
	}

	var opCount, errCount int64
	if p.metrics != nil {
		opCount = atomic.LoadInt64(&p.metrics.operationCount)
		errCount = atomic.LoadInt64(&p.metrics.errorCount)
	}

	return Stats{
		CacheSize:        cacheStats.Entries,
		CacheMemory:      cacheStats.TotalMemory,
		MaxCacheSize:     p.config.MaxCacheSize,
		HitCount:         cacheStats.HitCount,
		MissCount:        cacheStats.MissCount,
		HitRatio:         cacheStats.HitRatio,
		CacheTTL:         p.config.CacheTTL,
		CacheEnabled:     p.config.EnableCache,
		IsClosed:         p.IsClosed(),
		MemoryEfficiency: cacheStats.MemoryEfficiency,
		OperationCount:   opCount,
		ErrorCount:       errCount,
	}
}

// GetHealthStatus returns the current health status
func (p *Processor) GetHealthStatus() HealthStatus {
	if p == nil {
		return HealthStatus{
			Timestamp: time.Now(),
			Healthy:   false,
			Checks: map[string]CheckResult{
				"processor": {
					Healthy: false,
					Message: "processor is nil",
				},
			},
		}
	}
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

	healthChecker := internal.NewHealthChecker(p.metrics.collector, nil)
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

// incrementOperationCount atomically increments the operation counter with rate limiting
func (p *Processor) incrementOperationCount() {
	atomic.AddInt64(&p.metrics.operationCount, 1)
}

// checkRateLimit checks if the operation rate is within acceptable limits.
// Uses CAS loop to prevent TOCTOU race where concurrent goroutines both
// pass the rate check before either updates the timestamp.
func (p *Processor) checkRateLimit() error {
	if p.metrics.operationWindow <= 0 {
		return nil
	}

	const maxCASRetries = 3
	now := time.Now().UnixNano()

	for range maxCASRetries {
		lastOp := atomic.LoadInt64(&p.metrics.lastOperationTime)
		if lastOp > 0 && now-lastOp < int64(time.Second)/p.metrics.operationWindow {
			return &JsonsError{
				Op:      "rate_limit",
				Message: "operation rate limit exceeded",
				Err:     errOperationFailed,
			}
		}
		if atomic.CompareAndSwapInt64(&p.metrics.lastOperationTime, lastOp, now) {
			return nil
		}
	}
	// After maxCASRetries, another goroutine won the update — allow this operation
	return nil
}

// incrementErrorCount atomically increments the error counter with optional logging
func (p *Processor) incrementErrorCount() {
	atomic.AddInt64(&p.metrics.errorCount, 1)
}

// logError logs an error with structured logging
func (p *Processor) logError(ctx context.Context, operation, path string, err error) {
	logger := p.getLogger()
	if logger == nil {
		return
	}

	errorType := "unknown"
	var jsonErr *JsonsError
	if errors.As(err, &jsonErr) && jsonErr.Err != nil {
		errorType = jsonErr.Err.Error()
	}

	if p.metrics != nil && p.metrics.collector != nil {
		p.metrics.collector.RecordError(errorType)
	}

	sanitizedPath := sanitizePath(path)
	sanitizedError := sanitizeError(err)

	logger.ErrorContext(ctx, "JSON operation failed",
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

// logOperation logs a successful operation with structured logging and performance warnings
func (p *Processor) logOperation(ctx context.Context, operation, path string, duration time.Duration) {
	logger := p.getLogger()
	if logger == nil {
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

	if duration > slowOperationThreshold {
		// Log as warning for slow operations
		attrs := append(commonAttrs, slog.Int64("threshold_ms", slowOperationThreshold.Milliseconds()))
		logger.LogAttrs(ctx, slog.LevelWarn, "Slow JSON operation detected", attrs...)
	} else {
		// Log as debug for normal operations
		logger.LogAttrs(ctx, slog.LevelDebug, "JSON operation completed", commonAttrs...)
	}
}

// getProcessorID returns a unique identifier for this processor instance
// PERFORMANCE v2: Returns pre-cached ID to avoid fmt.Sprintf per log call
func (p *Processor) getProcessorID() string {
	return p.processorID
}

// getPathSegments gets a path segments slice from the pool.
// Returns a pointer to preserve pool identity for correct return via putPathSegments.
func (p *Processor) getPathSegments() *[]internal.PathSegment {
	return internal.GetPathSegmentSlice(8)
}

// putPathSegments returns a path segments slice to the pool.
func (p *Processor) putPathSegments(segments *[]internal.PathSegment) {
	if segments == nil {
		return
	}
	internal.PutPathSegmentSlice(segments)
}

// getStringBuilder gets a string builder from the pool
func (p *Processor) getStringBuilder() *strings.Builder {
	return internal.GetStringBuilder()
}

// putStringBuilder returns a string builder to the pool
func (p *Processor) putStringBuilder(sb *strings.Builder) {
	internal.PutStringBuilder(sb)
}

// validateInput validates JSON input string with optimized security checks
func (p *Processor) validateInput(jsonString string) error {
	return p.securityValidator.ValidateJSONInput(jsonString)
}

// validateInputEssential performs only essential safety checks (size + depth).
// SECURITY: These checks protect the process from DoS and must always be enforced,
// even when SkipValidation is true for trusted input.
func (p *Processor) validateInputEssential(jsonString string) error {
	return p.securityValidator.ValidateJSONInputEssential(jsonString)
}

// validatePath validates a JSON path string with enhanced security and efficiency
func (p *Processor) validatePath(path string) error {
	// Use the cached security validator instead of creating a new one each time
	return p.securityValidator.ValidatePathInput(path)
}

// sanitizePath removes potentially sensitive information from paths
func sanitizePath(path string) string {
	if len(path) > 100 {
		return truncateString(path, 100)
	}
	// Remove potential sensitive patterns but keep structure
	// Use case-insensitive matching for better security
	lowerPath := strings.ToLower(path)
	// Use package-level sensitivePatterns from security.go for consistency
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

// EscapeJSONPointer escapes special characters for JSON Pointer
func escapeJSONPointer(s string) string {
	return internal.EscapeJSONPointer(s)
}
