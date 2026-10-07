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

	// Batch size bound. Per-call cfg overrides; otherwise the processor's
	// baked MaxBatchSize applies — the same no-cfg rule as Set's CreatePaths
	// and the security limits (D-006: previously the default singleton's value,
	// so New(cfg).ProcessBatch ignored a tightened baked limit while
	// json.ProcessBatch(ops, cfg) enforced it).
	maxBatchSize := p.config.MaxBatchSize
	if len(cfg) > 0 {
		maxBatchSize = options.MaxBatchSize
	}
	if len(operations) > maxBatchSize {
		return nil, &JsonsError{
			Op:      "process_batch",
			Message: fmt.Sprintf("batch size %d exceeds maximum %d", len(operations), maxBatchSize),
			Err:     ErrSizeLimit,
		}
	}

	results := make([]BatchResult, len(operations))

	for i, op := range operations {
		result := BatchResult{ID: op.ID}

		switch op.Type {
		case opNameGet:
			result.Result, result.Error = p.Get(op.JSONStr, op.Path, cfg...)
		case opNameSet:
			result.Result, result.Error = p.Set(op.JSONStr, op.Path, op.Value, cfg...)
		case opNameDelete:
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

	// Validate JSON input. GEN-001 P1 review: honor SkipValidation like the
	// other input funnels (essential size/depth checks only).
	var jsonErr error
	if p.effectiveSkipValidation(options) {
		jsonErr = p.validateInputEssential(jsonStr)
	} else {
		jsonErr = p.validateInputForOptions(jsonStr, options)
	}
	if jsonErr != nil {
		return nil, &JsonsError{
			Op:      "warmup_cache",
			Message: "invalid JSON input for cache warmup",
			Err:     jsonErr,
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

		// Try to get the value (this will cache it if successful).
		// D-002/R8 (C1): forward the caller's cfg ONLY when one was supplied.
		// Dereferencing the no-cfg singleton into Get parses under DefaultConfig
		// values, warming the shared parse cache with float64 data on a
		// PreserveNumbers processor (the cache key cannot distinguish a
		// default-valued cfg from "no cfg").
		var getErr error
		if len(cfg) > 0 {
			_, getErr = p.Get(jsonStr, path, *options)
		} else {
			_, getErr = p.Get(jsonStr, path)
		}
		if getErr != nil {
			errorCount++
			failedPaths = append(failedPaths, path)
			lastError = &JsonsError{
				Op:      "warmup_cache",
				Path:    path,
				Message: fmt.Sprintf("failed to warmup path '%s': %v", path, getErr),
				Err:     getErr,
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
		slog.Int64("concurrent_ops", atomic.LoadInt64(&p.metrics.concurrentOps)),
	)
}

// logOperation logs a successful operation with structured logging and performance warnings
func (p *Processor) logOperation(ctx context.Context, operation, path string, duration time.Duration) {
	logger := p.getLogger()
	if logger == nil {
		return
	}

	// Use modern structured logging with typed attributes. The path is
	// sanitized exactly as logError does — a successful slow operation on
	// "users.admin.password" must not leak the sensitive key into logs.
	commonAttrs := []slog.Attr{
		slog.String("operation", operation),
		slog.String("path", sanitizePath(path)),
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
// PERFORMANCE: Returns pre-cached ID to avoid fmt.Sprintf per log call
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
//
// NOTE (P-001): no production caller — the composed-string cache keys that
// used it are gone (CacheKey structs); retained for tests.
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

// validateInputForOptions validates jsonStr against the effective security
// limits: per-call options when the caller supplied a Config, otherwise the
// processor's baked-in config.
//
// This makes per-call MaxJSONSize / MaxNestingDepthSecurity /
// FullSecurityScan / DisableDefaultPatterns actually take effect across all
// cfg-accepting operations (Get/Set/Delete/Valid/Parse/GetMultiple/
// SetMultiple/Prettify/Compact/ValidateSchema/PreParse/WarmupCache). It
// preserves prior behavior for no-cfg calls: a processor built with
// SecurityConfig still enforces its own limits when its methods are called
// without a per-call Config.
//
// The branching keys off the shared &defaultConfigSingleton sentinel — exactly
// the value prepareOptions returns when len(cfg)==0 — so the common no-cfg hot
// path stays on the cached p.securityValidator with no allocation. The per-call
// path builds a transient, cache-less validator from options (Valid is not a hot
// path; cfg'd Get/Set/Delete are less common than the no-cfg path).
func (p *Processor) validateInputForOptions(jsonStr string, options *Config) error {
	if options == &defaultConfigSingleton {
		return p.validateInput(jsonStr)
	}
	sv := newSecurityValidator(
		options.MaxJSONSize,
		maxPathLength,
		options.MaxNestingDepthSecurity,
		options.FullSecurityScan,
		options.DisableDefaultPatterns,
		options.DetectDuplicateKeys,
		toInternalPatterns(options.AdditionalDangerousPatterns),
		options.MaxObjectKeys,
		options.MaxArrayElements,
	)
	sv.cacheDisabled = true // transient one-shot validator: skip cache machinery
	return sv.ValidateJSONInput(jsonStr)
}

// validateInputForOptionsHashed is validateInputForOptions for callers that
// already hold the document's FNV-1a hash (P-001): the hash feeds the
// validation-cache lookup on BOTH branches — the processor's cached validator
// and the per-call-cfg transient validator (whose cache is disabled anyway, so
// the shared hash only replaces its internal computation).
// jsonHash MUST equal hashStringToUint64(jsonStr).
func (p *Processor) validateInputForOptionsHashed(jsonStr string, options *Config, jsonHash uint64) error {
	if options == &defaultConfigSingleton {
		return p.securityValidator.ValidateJSONInputPrehashed(jsonStr, jsonHash)
	}
	sv := newSecurityValidator(
		options.MaxJSONSize,
		maxPathLength,
		options.MaxNestingDepthSecurity,
		options.FullSecurityScan,
		options.DisableDefaultPatterns,
		options.DetectDuplicateKeys,
		toInternalPatterns(options.AdditionalDangerousPatterns),
		options.MaxObjectKeys,
		options.MaxArrayElements,
	)
	sv.cacheDisabled = true // transient one-shot validator: skip cache machinery
	return sv.ValidateJSONInputPrehashed(jsonStr, jsonHash)
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
	// Redact BEFORE truncating: the length check previously returned first,
	// so a >100-character path containing "password" etc. was logged with the
	// sensitive segment still inside the truncated prefix.
	lowerPath := strings.ToLower(path)
	// Use package-level sensitivePatterns from security.go for consistency
	for _, pattern := range sensitivePatterns {
		if strings.Contains(lowerPath, pattern) {
			return "[REDACTED_PATH]"
		}
	}
	if len(path) > 100 {
		return truncateString(path, 100)
	}
	return path
}

// sanitizeError removes potentially sensitive information from error messages
// before they are written to logs (GEN-001 must-fix #3). Two layers, mirroring
// sanitizePath:
//
//  1. Any sensitive keyword (the shared sensitivePatterns list) anywhere in the
//     message redacts the WHOLE message — a fragment of the surrounding
//     sentence could still reveal the value's context.
//  2. Otherwise, quoted spans longer than sanitizeQuoteKeepLen bytes are
//     replaced with [REDACTED]: this library's error convention embeds
//     caller-supplied data (values, pattern mismatches, paths) inside '...' or
//     "..." (e.g. "string 'x' does not match pattern 'y'"). Short spans
//     (typical identifiers like 'a' or 'user.name') are kept for debuggability;
//     longer spans are assumed to be payloads (tokens, cards, free text) and
//     masked whole.
//
// The returned error's errors.Is/As chain is unaffected — only the logged text
// is sanitized.
func sanitizeError(err error) string {
	if err == nil {
		return ""
	}
	errMsg := err.Error()

	lowerMsg := strings.ToLower(errMsg)
	for _, pattern := range sensitivePatterns {
		if strings.Contains(lowerMsg, pattern) {
			return "[REDACTED_ERROR]"
		}
	}

	errMsg = maskQuotedSpans(errMsg)
	if len(errMsg) > 200 {
		return truncateString(errMsg, 200)
	}
	return errMsg
}

// sanitizeQuoteKeepLen is the maximum quoted-span length sanitizeError keeps
// verbatim; longer spans are masked whole.
const sanitizeQuoteKeepLen = 8

// maskQuotedSpans replaces the content of '...' and "..." spans longer than
// sanitizeQuoteKeepLen bytes with [REDACTED]. Escapes are not interpreted: an
// unterminated quote masks the remainder of the string (fail-closed — a
// truncated message must not leak its tail verbatim).
func maskQuotedSpans(s string) string {
	// Fast path: nothing to mask unless a quote byte is present.
	hasQuote := false
	for i := 0; i < len(s); i++ {
		if s[i] == '\'' || s[i] == '"' {
			hasQuote = true
			break
		}
	}
	if !hasQuote {
		return s
	}

	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); {
		c := s[i]
		if c != '\'' && c != '"' {
			b.WriteByte(c)
			i++
			continue
		}
		// Find the closing quote of the same kind (no escape handling —
		// masking farther than strictly needed is the safe direction).
		end := -1
		for j := i + 1; j < len(s); j++ {
			if s[j] == c {
				end = j
				break
			}
		}
		if end == -1 {
			// Unterminated: mask the remainder and stop.
			b.WriteByte(c)
			b.WriteString("[REDACTED]")
			return b.String()
		}
		if end-i-1 > sanitizeQuoteKeepLen {
			b.WriteByte(c)
			b.WriteString("[REDACTED]")
			b.WriteByte(c)
		} else {
			b.WriteString(s[i : end+1])
		}
		i = end + 1
	}
	return b.String()
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
