package json

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cybergodev/json/internal"
)

// Processor is the main JSON processing engine with thread safety and performance optimization
type Processor struct {
	config          *Config
	cache           *internal.CacheManager
	state           int32
	cleanupOnce     sync.Once
	resources       *processorResources
	metrics         *processorMetrics
	resourceMonitor *ResourceMonitor
	logger          *slog.Logger
}

type processorResources struct {
	lastPoolReset   int64
	lastMemoryCheck int64
	memoryPressure  int32
}

type processorMetrics struct {
	operationCount       int64
	errorCount           int64
	lastOperationTime    int64
	operationWindow      int64
	concurrencySemaphore chan struct{}
	collector            *internal.MetricsCollector
	enabled              bool // Flag to enable/disable metrics collection
}

// New creates a new JSON processor with optimized configuration.
// If no configuration is provided, uses default configuration.
// This function follows the explicit config pattern as required by the design guidelines.
func New(config ...*Config) *Processor {
	var cfg *Config
	if len(config) > 0 && config[0] != nil {
		cfg = config[0].Clone() // Use clone to prevent external modifications
	} else {
		cfg = DefaultConfig()
	}

	// Validate configuration and use defaults for invalid values
	if err := ValidateConfig(cfg); err != nil {
		// Log the error but continue with corrected config instead of returning broken processor
		cfg = DefaultConfig()
	}

	p := &Processor{
		config:          cfg,
		cache:           internal.NewCacheManager(cfg),
		resourceMonitor: NewResourceMonitor(),
		logger:          slog.Default().With("component", "json-processor"),
		resources: &processorResources{
			lastPoolReset:   0,
			lastMemoryCheck: 0,
			memoryPressure:  0,
		},
		metrics: &processorMetrics{
			operationWindow:      0,
			concurrencySemaphore: make(chan struct{}, cfg.MaxConcurrency),
			enabled:              cfg.EnableMetrics,
		},
	}

	// Only create metrics collector if metrics are enabled
	if cfg.EnableMetrics {
		p.metrics.collector = internal.NewMetricsCollector()
	}

	return p
}

// Close closes the processor and cleans up resources
func (p *Processor) Close() error {
	p.cleanupOnce.Do(func() {
		atomic.StoreInt32(&p.state, 1)

		if p.cache != nil {
			p.cache.Clear()
		}

		// Reset resource tracking
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

// ClearCache clears all cached data
func (p *Processor) ClearCache() {
	if p.cache != nil {
		p.cache.Clear()
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

// hashString generates a fast hash for cache keys using FNV-1a
func hashString(s string) string {
	h := fnv.New64a()
	h.Write([]byte(s))
	return fmt.Sprintf("%x", h.Sum64())
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
	sb.WriteString(hashString(normalizedJSON))
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
	segments, err := internal.ParsePath(path)
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
	return internal.ParseSliceComponents(slicePart)
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

// DeleteWithCleanNull removes a value from JSON and cleans up null values
// Returns:
//   - On success: modified JSON string and nil error
//   - On failure: original unmodified JSON string and error information
func (p *Processor) DeleteWithCleanNull(jsonStr, path string, opts ...*ProcessorOptions) (string, error) {
	cleanupOpts := mergeOptionsWithOverride(opts, func(o *ProcessorOptions) {
		o.CleanupNulls = true
		o.CompactArrays = true
	})
	return p.Delete(jsonStr, path, cleanupOpts)
}

// Foreach iterates over JSON arrays or objects using this processor
func (p *Processor) Foreach(jsonStr string, fn func(key any, item *IterableValue)) {
	if err := p.checkClosed(); err != nil {
		return
	}

	data, err := p.Get(jsonStr, ".", nil)
	if err != nil {
		return
	}

	foreachWithIterableValue(data, fn)
}

// ForeachWithPath iterates over JSON arrays or objects at a specific path using this processor
// This allows using custom processor configurations (security limits, nesting depth, etc.)
func (p *Processor) ForeachWithPath(jsonStr, path string, fn func(key any, item *IterableValue)) error {
	if err := p.checkClosed(); err != nil {
		return err
	}

	data, err := p.Get(jsonStr, path, nil)
	if err != nil {
		return err
	}

	foreachWithIterableValue(data, fn)
	return nil
}

// ForeachWithPathAndIterator iterates over JSON at a path with path information
func (p *Processor) ForeachWithPathAndIterator(jsonStr, path string, fn func(key any, item *IterableValue, currentPath string) IteratorControl) error {
	if err := p.checkClosed(); err != nil {
		return err
	}

	data, err := p.Get(jsonStr, path, nil)
	if err != nil {
		return err
	}

	return foreachWithPathIterableValue(data, "", fn)
}

// ForeachWithPathAndControl iterates with control over iteration flow
func (p *Processor) ForeachWithPathAndControl(jsonStr, path string, fn func(key any, value any) IteratorControl) error {
	if err := p.checkClosed(); err != nil {
		return err
	}

	data, err := p.Get(jsonStr, path, nil)
	if err != nil {
		return err
	}

	return foreachOnValue(data, fn)
}

// ForeachReturn iterates over JSON arrays or objects and returns the JSON string
// This is useful for iteration with transformation purposes
func (p *Processor) ForeachReturn(jsonStr string, fn func(key any, item *IterableValue)) (string, error) {
	if err := p.checkClosed(); err != nil {
		return "", err
	}

	data, err := p.Get(jsonStr, ".", nil)
	if err != nil {
		return "", err
	}

	// Execute the iteration
	foreachWithIterableValue(data, fn)

	// Return the original JSON string
	return jsonStr, nil
}

// ForeachNested recursively iterates over all nested JSON structures
// This method traverses through all nested objects and arrays
func (p *Processor) ForeachNested(jsonStr string, fn func(key any, item *IterableValue)) {
	if err := p.checkClosed(); err != nil {
		return
	}

	data, err := p.Get(jsonStr, ".", nil)
	if err != nil {
		return
	}

	foreachNestedOnValue(data, fn)
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

// GetString retrieves a string value from JSON at the specified path
func (p *Processor) GetString(jsonStr, path string, opts ...*ProcessorOptions) (string, error) {
	return GetTypedWithProcessor[string](p, jsonStr, path, opts...)
}

// GetInt retrieves an int value from JSON at the specified path
func (p *Processor) GetInt(jsonStr, path string, opts ...*ProcessorOptions) (int, error) {
	return GetTypedWithProcessor[int](p, jsonStr, path, opts...)
}

// GetFloat64 retrieves a float64 value from JSON at the specified path
func (p *Processor) GetFloat64(jsonStr, path string, opts ...*ProcessorOptions) (float64, error) {
	return GetTypedWithProcessor[float64](p, jsonStr, path, opts...)
}

// GetBool retrieves a bool value from JSON at the specified path
func (p *Processor) GetBool(jsonStr, path string, opts ...*ProcessorOptions) (bool, error) {
	return GetTypedWithProcessor[bool](p, jsonStr, path, opts...)
}

// GetArray retrieves an array value from JSON at the specified path
func (p *Processor) GetArray(jsonStr, path string, opts ...*ProcessorOptions) ([]any, error) {
	return GetTypedWithProcessor[[]any](p, jsonStr, path, opts...)
}

// GetObject retrieves an object value from JSON at the specified path
func (p *Processor) GetObject(jsonStr, path string, opts ...*ProcessorOptions) (map[string]any, error) {
	return GetTypedWithProcessor[map[string]any](p, jsonStr, path, opts...)
}

// GetWithDefault retrieves a value from JSON with a default fallback
func (p *Processor) GetWithDefault(jsonStr, path string, defaultValue any, opts ...*ProcessorOptions) any {
	value, err := p.Get(jsonStr, path, opts...)
	if err != nil || value == nil {
		return defaultValue
	}
	return value
}

// GetStringWithDefault retrieves a string value from JSON with a default fallback
func (p *Processor) GetStringWithDefault(jsonStr, path, defaultValue string, opts ...*ProcessorOptions) string {
	value, err := p.GetString(jsonStr, path, opts...)
	if err != nil {
		return defaultValue
	}
	return value
}

// GetIntWithDefault retrieves an int value from JSON with a default fallback
func (p *Processor) GetIntWithDefault(jsonStr, path string, defaultValue int, opts ...*ProcessorOptions) int {
	value, err := p.GetInt(jsonStr, path, opts...)
	if err != nil {
		return defaultValue
	}
	return value
}

// GetFloat64WithDefault retrieves a float64 value from JSON with a default fallback
func (p *Processor) GetFloat64WithDefault(jsonStr, path string, defaultValue float64, opts ...*ProcessorOptions) float64 {
	value, err := p.GetFloat64(jsonStr, path, opts...)
	if err != nil {
		return defaultValue
	}
	return value
}

// GetBoolWithDefault retrieves a bool value from JSON with a default fallback
func (p *Processor) GetBoolWithDefault(jsonStr, path string, defaultValue bool, opts ...*ProcessorOptions) bool {
	value, err := p.GetBool(jsonStr, path, opts...)
	if err != nil {
		return defaultValue
	}
	return value
}

// GetArrayWithDefault retrieves an array value from JSON with a default fallback
func (p *Processor) GetArrayWithDefault(jsonStr, path string, defaultValue []any, opts ...*ProcessorOptions) []any {
	value, err := p.GetArray(jsonStr, path, opts...)
	if err != nil {
		return defaultValue
	}
	return value
}

// GetObjectWithDefault retrieves an object value from JSON with a default fallback
func (p *Processor) GetObjectWithDefault(jsonStr, path string, defaultValue map[string]any, opts ...*ProcessorOptions) map[string]any {
	value, err := p.GetObject(jsonStr, path, opts...)
	if err != nil {
		return defaultValue
	}
	return value
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

// executeWithConcurrencyControl executes an operation with timeout control
func (p *Processor) executeWithConcurrencyControl(
	operation func() error,
	timeout time.Duration,
) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	errChan := make(chan error, 1)
	go func() {
		errChan <- operation()
	}()

	select {
	case err := <-errChan:
		return err
	case <-ctx.Done():
		return fmt.Errorf("operation timeout after %v", timeout)
	}
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

// getPathSegments gets a path segments slice from the unified resource manager
func (p *Processor) getPathSegments() []PathSegment {
	return getGlobalResourceManager().GetPathSegments()
}

// putPathSegments returns a path segments slice to the unified resource manager
func (p *Processor) putPathSegments(segments []PathSegment) {
	getGlobalResourceManager().PutPathSegments(segments)
}

// getStringBuilder gets a string builder from the unified resource manager
func (p *Processor) getStringBuilder() *strings.Builder {
	return getGlobalResourceManager().GetStringBuilder()
}

// putStringBuilder returns a string builder to the unified resource manager
func (p *Processor) putStringBuilder(sb *strings.Builder) {
	getGlobalResourceManager().PutStringBuilder(sb)
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

	// Perform unified resource manager maintenance
	getGlobalResourceManager().PerformMaintenance()

	// Perform leak detection
	if p.resourceMonitor != nil {
		if issues := p.resourceMonitor.CheckForLeaks(); len(issues) > 0 {
			for _, issue := range issues {
				p.logger.Warn("Resource issue detected", "issue", issue)
			}
		}
	}
}

// isClosing returns true if the processor is in the process of closing
func (p *Processor) isClosing() bool {
	state := atomic.LoadInt32(&p.state)
	return state == 1 || state == 2
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
	// Pre-allocate with capacity for potential failures (typically few)
	failedPaths := make([]string, 0, min(len(updates), 10))

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

// SetWithAdd sets a value with automatic path creation
// Returns:
//   - On success: modified JSON string and nil error
//   - On failure: original unmodified JSON string and error information
func (p *Processor) SetWithAdd(jsonStr, path string, value any, opts ...*ProcessorOptions) (string, error) {
	addOpts := mergeOptionsWithOverride(opts, func(o *ProcessorOptions) {
		o.CreatePaths = true
	})
	return p.Set(jsonStr, path, value, addOpts)
}

// SetMultipleWithAdd sets multiple values with automatic path creation
// Returns:
//   - On success: modified JSON string and nil error
//   - On failure: original unmodified JSON string and error information
func (p *Processor) SetMultipleWithAdd(jsonStr string, updates map[string]any, opts ...*ProcessorOptions) (string, error) {
	addOpts := mergeOptionsWithOverride(opts, func(o *ProcessorOptions) {
		o.CreatePaths = true
	})
	return p.SetMultiple(jsonStr, updates, addOpts)
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
		OperationCount:   atomic.LoadInt64(&p.metrics.operationCount),
		ErrorCount:       atomic.LoadInt64(&p.metrics.errorCount),
	}
}

// getDetailedStats returns detailed performance statistics
func (p *Processor) getDetailedStats() DetailedStats {
	stats := p.GetStats()
	resourceStats := getGlobalResourceManager().GetStats()

	return DetailedStats{
		Stats:          stats,
		state:          atomic.LoadInt32(&p.state),
		configSnapshot: *p.config,
		resourcePoolStats: ResourcePoolStats{
			StringBuilderPoolActive: resourceStats.AllocatedBuilders > 0,
			PathSegmentPoolActive:   resourceStats.AllocatedSegments > 0,
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

// processorUtils provides utility functions for JSON processing
type processorUtils struct {
	// String builder pool for efficient string operations
	stringBuilderPool *stringBuilderPool
}

// NewProcessorUtils creates a new processor utils instance
func NewProcessorUtils() *processorUtils {
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
	return internal.EscapeJSONPointer(s)
}

// UnescapeJSONPointer unescapes JSON Pointer special characters
func UnescapeJSONPointer(s string) string {
	return internal.UnescapeJSONPointer(s)
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
	return internal.MergeObjects(obj1, obj2)
}

// flattenArray flattens a nested array structure (internal use)
func flattenArray(arr []any) []any {
	return internal.FlattenArray(arr)
}

// uniqueArray removes duplicate values from an array (internal use)
func uniqueArray(arr []any) []any {
	return internal.UniqueArray(arr)
}

// reverseArray reverses an array in place (internal use)
func reverseArray(arr []any) {
	internal.ReverseArray(arr)
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

// validateInput validates JSON input string with optimized security checks
func (p *Processor) validateInput(jsonString string) error {
	// Use the unified security validator
	validator := NewSecurityValidator(
		p.config.MaxJSONSize,
		MaxPathLength,
		p.config.MaxNestingDepthSecurity,
	)

	return validator.ValidateJSONInput(jsonString)
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

// isValidNumberStart checks if character can start a valid JSON number
func isValidNumberStart(c byte) bool {
	return (c >= '0' && c <= '9') || c == '-'
}

// validatePath validates a JSON path string with enhanced security and efficiency
func (p *Processor) validatePath(path string) error {
	// Use the unified security validator
	validator := NewSecurityValidator(
		p.config.MaxJSONSize,
		MaxPathLength,
		p.config.MaxNestingDepthSecurity,
	)

	return validator.ValidatePathInput(path)
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
