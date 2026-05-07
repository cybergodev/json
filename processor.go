package json

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cybergodev/json/internal"
)

// Processor state constants for lifecycle management
const (
	processorStateActive        int32 = iota // 0: Processor is active and accepting operations
	processorStateClosing                    // 1: Processor is closing, no new operations
	processorStateClosed                     // 2: Processor is fully closed
	processorStateCloseTimedOut              // 3: Processor close timed out, resources not fully released
)

// Processor is the main JSON processing engine with thread safety and performance optimization
type Processor struct {
	config            Config
	cache             *internal.CacheManager
	state             int32
	cleanupOnce       sync.Once
	resources         *processorResources
	metrics           *processorMetrics
	resourceMonitor   *resourceMonitor
	logger            atomic.Value // *slog.Logger - thread-safe logger storage
	securityValidator *securityValidator
	// Cached recursiveProcessor for reuse across operations (performance optimization)
	recursiveProcessor *recursiveProcessor
	// Extension points for hooks
	hooks   []Hook
	hooksMu sync.Mutex // protects hooks slice for concurrent AddHook
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

// New creates a new JSON processor with the given configuration.
// If no configuration is provided, uses default configuration.
//
// Returns an error if the configuration is invalid (see Config.Validate).
// Always call Close() when done to release resources.
//
// Example:
//
//	// Using default configuration
//	processor, err := json.New()
//	if err != nil {
//	    // Handle configuration error
//	}
//	defer processor.Close()
//
//	// With custom configuration
//	cfg := json.DefaultConfig()
//	cfg.CreatePaths = true
//	cfg.EnableCache = true
//	processor, err := json.New(cfg)
//
//	// Using preset configuration
//	processor, err := json.New(json.SecurityConfig())
func New(cfg ...Config) (*Processor, error) {
	var config Config
	if len(cfg) > 0 {
		config = cfg[0]
	} else {
		config = DefaultConfig()
	}

	// Validate configuration and apply corrections for invalid values
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	p := &Processor{
		config:          config,
		cache:           internal.NewCacheManager(config.EnableCache, config.MaxCacheSize, config.CacheTTL),
		resourceMonitor: newResourceMonitor(),
		securityValidator: newSecurityValidator(
			config.MaxJSONSize,
			maxPathLength,
			config.MaxNestingDepthSecurity,
			config.FullSecurityScan,
		),
		resources: &processorResources{
			lastPoolReset:   0,
			lastMemoryCheck: 0,
			memoryPressure:  0,
		},
		metrics: &processorMetrics{
			operationWindow:      0, // Disabled by default for better performance
			concurrencySemaphore: make(chan struct{}, config.MaxConcurrency),
			enabled:              config.EnableMetrics,
		},
	}

	// Initialize logger atomically for thread safety
	p.logger.Store(slog.Default().With("component", "json-processor"))

	// Only create metrics collector if metrics are enabled
	if config.EnableMetrics {
		p.metrics.collector = internal.NewMetricsCollector()
	}

	// Initialize cached recursiveProcessor for reuse
	p.recursiveProcessor = newRecursiveProcessor(p)

	return p, nil
}

// Close closes the processor and cleans up resources.
// This method is idempotent and thread-safe.
// After Close is called, all operations on the processor will return ErrProcessorClosed.
//
// IMPORTANT: Always call Close() to release resources:
//
//	processor, err := json.New()
//	if err != nil {
//	    return err
//	}
//	defer processor.Close()
func (p *Processor) Close() error {
	if p == nil {
		return nil
	}
	p.cleanupOnce.Do(func() {
		// Mark as closing to prevent new operations
		atomic.StoreInt32(&p.state, processorStateClosing)

		// Drain the concurrency semaphore to release any waiting goroutines
		// Use context cancellation for clean goroutine termination
		if p.metrics != nil && p.metrics.concurrencySemaphore != nil {
			drainCtx, drainCancel := context.WithTimeout(context.Background(), semaphoreDrainTimeout)
			drainDone := make(chan struct{})
			go func() {
				defer close(drainDone)
				for {
					select {
					case <-drainCtx.Done():
						// Context cancelled - exit cleanly
						return
					case <-p.metrics.concurrencySemaphore:
						// Drained one slot
					default:
						// Semaphore is empty
						return
					}
				}
			}()

			select {
			case <-drainDone:
				// Drain completed - goroutine exited cleanly
				drainCancel()
			case <-drainCtx.Done():
				// Timeout on drain - cancel context to signal goroutine to exit,
				// then wait briefly for it to acknowledge
				drainCancel()
				select {
				case <-drainDone:
					// Goroutine exited after context cancellation
				case <-time.After(100 * time.Millisecond):
					// Goroutine still running - cannot wait indefinitely
					// The goroutine will exit when it checks drainCtx.Done()
				}
			}
		}

		// Safely close cache: cancels cleanup goroutines and clears data
		if p.cache != nil {
			p.cache.Close()
		}

		// Close security validator to release its cache
		if p.securityValidator != nil {
			p.securityValidator.Close()
		}

		// Reset resource tracking
		if p.resources != nil {
			atomic.StoreInt32(&p.resources.memoryPressure, 0)
			atomic.StoreInt64(&p.resources.lastMemoryCheck, 0)
			atomic.StoreInt64(&p.resources.lastPoolReset, 0)
		}

		// Release hook references to allow GC of captured closures
		p.hooksMu.Lock()
		p.hooks = nil
		p.hooksMu.Unlock()

		// NOTE: Global caches (pathTypeCache, structEncoderCache) are NOT cleared
		// here because they are shared across ALL processor instances. Clearing them
		// in individual Close() would invalidate caches for other active processors.
		// Use ShutdownGlobalProcessor() for complete cleanup at application shutdown.

		// Mark as fully closed
		atomic.StoreInt32(&p.state, processorStateClosed)
	})

	return nil
}

// IsClosed returns true if the processor has been closed or close timed out.
// In both states the processor should not accept new operations.
func (p *Processor) IsClosed() bool {
	if p == nil {
		return true
	}
	state := atomic.LoadInt32(&p.state)
	return state == processorStateClosed || state == processorStateCloseTimedOut
}

// AddHook adds an operation hook to the processor.
// Hooks are called before and after each operation.
// Multiple hooks can be added and are executed in order (Before) and reverse order (After).
//
// Example:
//
//	type LoggingHook struct{}
//	func (h *LoggingHook) Before(ctx json.HookContext) error {
//	    log.Printf("before %s", ctx.Operation)
//	    return nil
//	}
//	func (h *LoggingHook) After(ctx json.HookContext, result any, err error) (any, error) {
//	    log.Printf("after %s", ctx.Operation)
//	    return result, err
//	}
//
//	processor, err := json.New()
//	if err != nil {
//	    return err
//	}
//	defer processor.Close()
//	processor.AddHook(&LoggingHook{})
func (p *Processor) AddHook(hook Hook) {
	if p == nil {
		return
	}
	p.hooksMu.Lock()
	p.hooks = append(p.hooks, hook)
	p.hooksMu.Unlock()
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

// GetConfig returns a copy of the processor configuration
func (p *Processor) GetConfig() Config {
	if p == nil {
		return Config{}
	}
	return *p.config.Clone()
}

// SetLogger sets a custom structured logger for the processor
func (p *Processor) SetLogger(logger *slog.Logger) {
	if p == nil {
		return
	}
	if logger != nil {
		p.logger.Store(logger.With("component", "json-processor"))
	} else {
		p.logger.Store(slog.Default().With("component", "json-processor"))
	}
}

// getLogger safely retrieves the current logger (thread-safe).
// Returns slog.Default() when called on a nil Processor.
func (p *Processor) getLogger() *slog.Logger {
	if p == nil {
		return slog.Default().With("component", "json-processor")
	}
	if l, ok := p.logger.Load().(*slog.Logger); ok {
		return l
	}
	return slog.Default().With("component", "json-processor")
}

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

// configPool pools Config objects to reduce allocations in hot paths
// PERFORMANCE: Reduces ~6GB allocations from prepareOptions calls
var configPool = sync.Pool{
	New: func() any {
		cfg := DefaultConfig()
		return &cfg
	},
}

// cachedDefaultConfigPtr is a pre-validated pointer to the default config.
// PERFORMANCE: When no options are provided, return this instead of allocating
// from the pool and calling DefaultConfig() + Validate() on every operation.
var cachedDefaultConfigPtr = func() *Config {
	cfg := DefaultConfig()
	return &cfg
}()

// releaseConfig returns a pooled Config, clearing all reference-type fields first
// to prevent data leaks back into the pool.
// SECURITY: Must clear all map/slice/interface fields to avoid cross-request contamination.
func releaseConfig(cfg *Config) {
	if cfg == nil {
		return
	}
	// PERFORMANCE: Skip returning the cached default pointer to the pool
	if cfg == cachedDefaultConfigPtr {
		return
	}
	cfg.CustomEncoder = nil
	cfg.CustomPathParser = nil
	cfg.CustomEscapes = nil
	cfg.CustomTypeEncoders = nil
	cfg.CustomValidators = nil
	cfg.AdditionalDangerousPatterns = nil
	cfg.Hooks = nil
	configPool.Put(cfg)
}

// prepareOptions prepares and validates processor options.
// Accepts Config values and returns a pointer for internal use.
// PERFORMANCE: When no options are provided, returns a cached default pointer
// without allocation or validation — avoids DefaultConfig() + Validate() per operation.
// SECURITY: Clears reference fields from pooled objects to prevent leaks.
func (p *Processor) prepareOptions(cfg ...Config) (*Config, error) {
	if len(cfg) == 0 {
		// Fast path: return cached default config pointer (no allocation, no validation)
		return cachedDefaultConfigPtr, nil
	}
	c := configPool.Get().(*Config)
	c.CustomEncoder = nil
	c.CustomPathParser = nil
	c.Hooks = nil
	c.CustomEscapes = nil
	c.CustomTypeEncoders = nil
	c.CustomValidators = nil
	c.AdditionalDangerousPatterns = nil
	*c = cfg[0]
	if err := c.Validate(); err != nil {
		releaseConfig(c)
		return nil, err
	}
	return c, nil
}

// mergeOptionsWithOverride creates a new Config with overrides applied.
// Returns a Config value (not pointer) to prevent accidental mutation
// and encourage the caller to work with their own copy.
func mergeOptionsWithOverride(opts []Config, override func(*Config)) Config {
	var result Config
	if len(opts) > 0 {
		result = *(&opts[0]).Clone()
	} else {
		result = DefaultConfig()
	}
	override(&result)
	return result
}

// Delete removes a value from JSON at the specified path
func (p *Processor) Delete(jsonStr, path string, cfg ...Config) (string, error) {
	if err := p.checkClosed(); err != nil {
		return "", err
	}

	options, err := p.prepareOptions(cfg...)
	if err != nil {
		return "", err
	}
	defer releaseConfig(options)

	if err := p.validateInput(jsonStr); err != nil {
		return jsonStr, err
	}

	if err := p.validatePath(path); err != nil {
		return jsonStr, err
	}

	// Parse JSON
	var data any
	err = p.Parse(jsonStr, &data, cfg...)
	if err != nil {
		return jsonStr, err
	}

	// Determine cleanup options from prepared options and config
	cleanupNulls := options.CleanupNulls || p.config.CleanupNulls
	compactArrays := options.CompactArrays || p.config.CompactArrays

	// If compactArrays is enabled, automatically enable cleanupNulls
	if compactArrays {
		cleanupNulls = true
	}

	// Check if path contains array access - only then we need DeletedMarker cleanup
	needsMarkerCleanup := p.isArrayDeletePath(path)

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

	// Only clean up deleted markers if the path involved array operations
	// This optimization avoids unnecessary data structure recreation for map deletions
	if needsMarkerCleanup {
		data = p.cleanupDeletedMarkers(data)
	}

	// Cleanup nulls if requested
	if cleanupNulls {
		data = p.cleanupNullValuesWithReconstruction(data, compactArrays)
	}

	// Convert back to JSON string
	// PERFORMANCE: Use FastMarshalToString instead of json.Marshal to avoid
	// double allocation (bytes -> string) and leverage optimized encoder pools
	result, err := internal.FastMarshalToString(data)
	if err != nil {
		// Return original JSON instead of empty string when marshaling fails
		return jsonStr, &JsonsError{
			Op:      "delete",
			Path:    path,
			Message: "failed to marshal result",
			Err:     err,
		}
	}

	return result, nil
}

// isArrayDeletePath checks if the path involves array operations that require marker cleanup
func (p *Processor) isArrayDeletePath(path string) bool {
	// Check if path contains array bracket notation
	for i := 0; i < len(path); i++ {
		if path[i] == '[' {
			return true
		}
	}
	return false
}

// DeleteClean removes a value from JSON and cleans up null placeholders.
// This is the unified API for delete-with-cleanup operations.
//
// Example:
//
//	result, err := processor.DeleteClean(data, "users[0].profile")
func (p *Processor) DeleteClean(jsonStr, path string, cfg ...Config) (string, error) {
	cleanupOpts := mergeOptionsWithOverride(cfg, func(o *Config) {
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

	data, err := p.Get(jsonStr, ".")
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

	data, err := p.Get(jsonStr, path)
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

	data, err := p.Get(jsonStr, path)
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

	data, err := p.Get(jsonStr, path)
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

	data, err := p.Get(jsonStr, ".")
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

	data, err := p.Get(jsonStr, ".")
	if err != nil {
		return
	}

	foreachNestedOnValue(data, fn)
}

// ForeachWithError iterates over JSON arrays or objects with error-returning callback.
// The callback returns an error to signal iteration control:
//   - nil: continue iteration
//   - errBreak (via item.Break()): stop iteration without error
//   - other error: stop iteration and return the error
//
// Example:
//
//	err := processor.ForeachWithError(jsonStr, ".", func(key any, item *json.IterableValue) error {
//	    if item.GetInt("id") == targetId {
//	        return item.Break() // stop iteration
//	    }
//	    return nil // continue
//	})
func (p *Processor) ForeachWithError(jsonStr, path string, fn func(key any, item *IterableValue) error) error {
	if err := p.checkClosed(); err != nil {
		return err
	}

	data, err := p.Get(jsonStr, path)
	if err != nil {
		return err
	}

	return foreachWithIterableValueError(data, fn)
}

// ForeachNestedWithError recursively iterates over all nested JSON structures with error-returning callback.
//
// Example:
//
//	err := processor.ForeachNestedWithError(jsonStr, func(key any, item *json.IterableValue) error {
//	    fmt.Printf("Key: %v\n", key)
//	    return nil
//	})
func (p *Processor) ForeachNestedWithError(jsonStr string, fn func(key any, item *IterableValue) error) error {
	if err := p.checkClosed(); err != nil {
		return err
	}

	data, err := p.Get(jsonStr, ".")
	if err != nil {
		return err
	}

	return foreachNestedOnValueError(data, fn)
}

// SafeGet performs a type-safe get operation with comprehensive error handling.
// Accepts optional Config for controlling validation, security, and caching behavior.
func (p *Processor) SafeGet(jsonStr, path string, cfg ...Config) AccessResult {
	// Validate inputs
	if jsonStr == "" {
		return AccessResult{Exists: false}
	}
	if path == "" {
		return AccessResult{Exists: false}
	}

	// Perform the get operation
	value, err := p.Get(jsonStr, path, cfg...)
	if err != nil {
		return AccessResult{Exists: false}
	}

	// Determine the type
	var valueType string
	if value == nil {
		valueType = "null"
	} else {
		valueType = fmt.Sprintf("%T", value)
	}

	return AccessResult{
		Value:  value,
		Exists: true,
		Type:   valueType,
	}
}

// Get retrieves a value from JSON using a path expression with performance
func (p *Processor) Get(jsonStr, path string, cfg ...Config) (result any, err error) {
	// PERFORMANCE: Fast path — check nil/closed before any field access
	if err := p.checkClosed(); err != nil {
		return nil, err
	}

	// Check rate limiting for security (fast return when disabled, which is default)
	if p.metrics.operationWindow > 0 {
		if err := p.checkRateLimit(); err != nil {
			return nil, err
		}
	}

	// Increment operation counter for statistics
	p.incrementOperationCount()

	options, err := p.prepareOptions(cfg...)
	if err != nil {
		p.incrementErrorCount()
		return nil, err
	}
	defer releaseConfig(options)

	// PERFORMANCE: Metrics tracking — only allocate closures when metrics are enabled
	var metricsCollector *internal.MetricsCollector
	var startTime time.Time
	if p.metrics != nil && p.metrics.enabled {
		metricsCollector = p.metrics.collector
		if metricsCollector != nil {
			startTime = time.Now()
			metricsCollector.StartConcurrentOperation()
		}
	}

	// Cleanup metrics via defer using named return values
	// Uses success flag based on whether err was set
	defer func() {
		if metricsCollector != nil {
			metricsCollector.EndConcurrentOperation()
			if !startTime.IsZero() {
				metricsCollector.RecordOperation(time.Since(startTime), err == nil, 0)
			}
		}
	}()

	// Get context for logging
	ctx := context.Background()

	// Defer slow operation logging (no-op when logger is at default level)
	defer func() {
		if !startTime.IsZero() {
			p.logOperation(ctx, "get", path, time.Since(startTime))
		}
	}()

	// Validate input BEFORE cache lookup to prevent cache pollution
	// SECURITY: Essential checks (size + depth) are always enforced to protect the process.
	// Full validation (UTF-8, security patterns, structure) can be skipped for trusted input.
	if !options.SkipValidation {
		if err := p.validateInput(jsonStr); err != nil {
			p.incrementErrorCount()
			return nil, err
		}

		if err := p.validatePath(path); err != nil {
			p.incrementErrorCount()
			return nil, err
		}
	} else {
		// SECURITY: Always enforce essential checks even for trusted input.
		// Size and depth limits protect the process from DoS.
		if err := p.validateInputEssential(jsonStr); err != nil {
			p.incrementErrorCount()
			return nil, err
		}
	}

	// PERFORMANCE: Compute hash ONCE for entire operation, reuse for all cache keys
	// Hash is computed after validation to avoid wasted work on invalid input
	jsonHash := hashStringToUint64(jsonStr)

	// Check cache after validation
	cacheKey := p.createCacheKeyWithHash("get", jsonHash, path, options)
	if cached, ok := p.getCachedResult(cacheKey); ok {
		// Record cache hit operation
		if metricsCollector != nil {
			metricsCollector.RecordCacheHit()
		}
		// PERFORMANCE v2: Skip deep copy for JSON primitives (immutable types).
		// For parsed JSON data, only map[string]any and []any need copying.
		// This avoids the deepCopySubtree overhead for ~60% of Get results.
		switch cached.(type) {
		case nil, bool, float64, string, json.Number:
			return cached, nil
		}
		if copied, copyErr := deepCopySubtree(cached); copyErr == nil {
			return copied, nil
		}
		return cached, nil
	}

	// Record cache miss
	if metricsCollector != nil {
		metricsCollector.RecordCacheMiss()
	}

	// Try to get parsed data from cache first - reuse pre-computed hash
	parseCacheKey := p.createCacheKeyWithHash("parse", jsonHash, "", options)
	var data any

	if cachedData, ok := p.getCachedResult(parseCacheKey); ok {
		data = cachedData
	} else {
		// Parse JSON with error context
		parseErr := p.Parse(jsonStr, &data, cfg...)
		if parseErr != nil {
			p.incrementErrorCount()
			return nil, parseErr
		}

		// Cache parsed data for reuse - always cache if global cache is enabled
		// PERFORMANCE: Use setCachedResultInternal to skip sensitive data check
		// since this is trusted internal data from our parser
		if p.config.EnableCache {
			p.setCachedResultInternal(parseCacheKey, data)
		}
	}

	// Use unified recursive processor for all paths (cached instance)
	result, err = p.recursiveProcessor.ProcessRecursively(data, path, opGet, nil)
	if err != nil {
		p.incrementErrorCount()
		return nil, &JsonsError{
			Op:      "get",
			Path:    path,
			Message: err.Error(),
			Err:     err,
		}
	}

	// Cache result if enabled
	p.setCachedResult(cacheKey, result, options)

	return result, nil
}

// GetWithContext retrieves a value from JSON with context support for cancellation.
// This is the context-aware version of Get() that respects context cancellation
// and timeout deadlines.
//
// Example:
//
//	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
//	defer cancel()
//	value, err := processor.GetWithContext(ctx, jsonStr, "user.name")
func (p *Processor) GetWithContext(ctx context.Context, jsonStr, path string, cfg ...Config) (any, error) {
	if err := p.checkClosed(); err != nil {
		return nil, err
	}

	// Check for context cancellation before starting
	select {
	case <-ctx.Done():
		p.incrementErrorCount()
		p.logError(ctx, "get_with_context", path, ctx.Err())
		return nil, ctx.Err()
	default:
	}

	// Delegate to Get and check context after operation
	result, err := p.Get(jsonStr, path, cfg...)
	if err != nil {
		return nil, err
	}

	// Check context after operation
	select {
	case <-ctx.Done():
		p.logError(ctx, "get_with_context", path, ctx.Err())
		return nil, ctx.Err()
	default:
		return result, nil
	}
}

// PreParse parses a JSON string and returns a ParsedJSON object that can be reused
// for multiple Get operations. This is a performance optimization for scenarios where
// the same JSON is queried multiple times.
//
// OPTIMIZED: Pre-parsing avoids repeated JSON parsing overhead for repeated queries.
//
// Call Release() on the returned ParsedJSON when finished to free the processor reference.
//
// Example:
//
//	parsed, err := processor.PreParse(jsonStr)
//	if err != nil { return err }
//	defer parsed.Release()
//	value1, _ := processor.GetFromParsed(parsed, "path1")
//	value2, _ := processor.GetFromParsed(parsed, "path2")
func (p *Processor) PreParse(jsonStr string, cfg ...Config) (*ParsedJSON, error) {
	if err := p.checkClosed(); err != nil {
		return nil, err
	}

	options, err := p.prepareOptions(cfg...)
	if err != nil {
		return nil, err
	}
	defer releaseConfig(options)

	// Validate input
	if err := p.validateInput(jsonStr); err != nil {
		return nil, err
	}

	// Try to get from cache first
	parseCacheKey := p.createCacheKey("parse", jsonStr, "", options)
	var data any

	if cachedData, ok := p.getCachedResult(parseCacheKey); ok {
		data = cachedData
	} else {
		// Parse JSON
		parseErr := p.Parse(jsonStr, &data, cfg...)
		if parseErr != nil {
			return nil, parseErr
		}

		// Cache parsed data - PERFORMANCE: Use internal method to skip sensitive check
		if p.config.EnableCache {
			p.setCachedResultInternal(parseCacheKey, data)
		}
	}

	return &ParsedJSON{
		data:      data,
		hash:      hashStringToUint64(jsonStr),
		processor: p,
	}, nil
}

// GetFromParsed retrieves a value from a pre-parsed JSON document at the specified path.
// This is significantly faster than Get() for repeated queries on the same JSON.
//
// OPTIMIZED: Skips JSON parsing, goes directly to path navigation.
func (p *Processor) GetFromParsed(parsed *ParsedJSON, path string, cfg ...Config) (any, error) {
	if parsed == nil {
		return nil, &JsonsError{
			Op:      "get_from_parsed",
			Message: "parsed JSON is nil",
			Err:     errOperationFailed,
		}
	}

	if err := p.checkClosed(); err != nil {
		return nil, err
	}

	options, err := p.prepareOptions(cfg...)
	if err != nil {
		return nil, err
	}
	defer releaseConfig(options)

	if err := p.validatePath(path); err != nil {
		return nil, err
	}

	// Use unified recursive processor for path navigation
	result, err := p.recursiveProcessor.ProcessRecursively(parsed.data, path, opGet, nil)
	if err != nil {
		return nil, &JsonsError{
			Op:      "get_from_parsed",
			Path:    path,
			Message: err.Error(),
			Err:     err,
		}
	}

	// Cache result if enabled
	if p.config.EnableCache && options.CacheResults {
		cacheKey := p.createCacheKeyWithHash("get", parsed.hash, path, options)
		p.setCachedResult(cacheKey, result, options)
	}

	return result, nil
}

// SetFromParsed modifies a pre-parsed JSON document at the specified path.
// Returns a new ParsedJSON with the modified data (original is not modified).
//
// OPTIMIZED: Skips JSON parsing, works directly on parsed data.
func (p *Processor) SetFromParsed(parsed *ParsedJSON, path string, value any, cfg ...Config) (*ParsedJSON, error) {
	if parsed == nil {
		return nil, &JsonsError{
			Op:      "set_from_parsed",
			Message: "parsed JSON is nil",
			Err:     errOperationFailed,
		}
	}

	if err := p.checkClosed(); err != nil {
		return nil, err
	}

	options, err := p.prepareOptions(cfg...)
	if err != nil {
		return nil, err
	}
	defer releaseConfig(options)

	if err := p.validatePath(path); err != nil {
		return nil, err
	}

	// Deep copy the data before modification
	dataCopy, err := deepCopy(parsed.data)
	if err != nil {
		return nil, &JsonsError{Op: "set_from_parsed", Path: path, Err: err}
	}

	// Use unified recursive processor for path navigation and modification
	result, err := p.recursiveProcessor.ProcessRecursivelyWithOptions(dataCopy, path, opSet, value, options.CreatePaths)
	if err != nil {
		return nil, &JsonsError{
			Op:      "set_from_parsed",
			Path:    path,
			Message: err.Error(),
			Err:     err,
		}
	}

	return &ParsedJSON{
		data:      result,
		hash:      0, // New hash will be computed when needed
		processor: p,
	}, nil
}

// GetString retrieves a string value from JSON at the specified path.
// Returns defaultValue if provided, otherwise "" when: path not found, value is null, or type conversion fails.
func (p *Processor) GetString(jsonStr, path string, defaultValue ...string) string {
	return getTypedWithDefault[string](p, jsonStr, path, defaultValue...)
}

// GetInt retrieves an int value from JSON at the specified path.
// Returns defaultValue if provided, otherwise 0 when: path not found, value is null, or type conversion fails.
func (p *Processor) GetInt(jsonStr, path string, defaultValue ...int) int {
	return getTypedWithDefault[int](p, jsonStr, path, defaultValue...)
}

// GetFloat retrieves a float64 value from JSON at the specified path.
// Returns defaultValue if provided, otherwise 0.0 when: path not found, value is null, or type conversion fails.
func (p *Processor) GetFloat(jsonStr, path string, defaultValue ...float64) float64 {
	return getTypedWithDefault[float64](p, jsonStr, path, defaultValue...)
}

// GetBool retrieves a bool value from JSON at the specified path.
// Returns defaultValue if provided, otherwise false when: path not found, value is null, or type conversion fails.
func (p *Processor) GetBool(jsonStr, path string, defaultValue ...bool) bool {
	return getTypedWithDefault[bool](p, jsonStr, path, defaultValue...)
}

// GetArray retrieves an array value from JSON at the specified path.
// Returns defaultValue if provided, otherwise nil when: path not found, value is null, or type conversion fails.
func (p *Processor) GetArray(jsonStr, path string, defaultValue ...[]any) []any {
	return getTypedWithDefault[[]any](p, jsonStr, path, defaultValue...)
}

// GetObject retrieves an object value from JSON at the specified path.
// Returns defaultValue if provided, otherwise nil when: path not found, value is null, or type conversion fails.
func (p *Processor) GetObject(jsonStr, path string, defaultValue ...map[string]any) map[string]any {
	return getTypedWithDefault[map[string]any](p, jsonStr, path, defaultValue...)
}

// GetMultiple retrieves multiple values from JSON using multiple path expressions
func (p *Processor) GetMultiple(jsonStr string, paths []string, cfg ...Config) (map[string]any, error) {
	if err := p.checkClosed(); err != nil {
		return nil, err
	}

	if err := p.validateInput(jsonStr); err != nil {
		return nil, err
	}

	if len(paths) == 0 {
		return make(map[string]any), nil
	}

	options, err := p.prepareOptions(cfg...)
	if err != nil {
		return nil, err
	}
	defer releaseConfig(options)

	// Parse JSON once for all operations
	var data any
	if err := p.Parse(jsonStr, &data, cfg...); err != nil {
		return nil, err
	}

	// Sequential processing
	results := make(map[string]any, len(paths))
	var firstError error
	for _, path := range paths {
		if err := p.validatePath(path); err != nil {
			return nil, err
		}

		// Use cached recursive processor
		result, err := p.recursiveProcessor.ProcessRecursively(data, path, opGet, nil)

		if err != nil {
			results[path] = nil
			if firstError == nil {
				firstError = &JsonsError{
					Op:      "get_multiple",
					Path:    path,
					Message: err.Error(),
					Err:     err,
				}
			}
		} else {
			results[path] = result
		}
	}

	return results, firstError
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
				Err:     errOperationFailed,
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
func (p *Processor) getProcessorID() string {
	// Use the processor's memory address as a unique identifier
	return fmt.Sprintf("proc_%p", p)
}

// getPathSegments gets a path segments slice from the pool
func (p *Processor) getPathSegments() []internal.PathSegment {
	seg := internal.GetPathSegmentSlice(8)
	return *seg
}

// putPathSegments returns a path segments slice to the pool
func (p *Processor) putPathSegments(segments []internal.PathSegment) {
	if segments == nil {
		return
	}
	internal.PutPathSegmentSlice(&segments)
}

// getStringBuilder gets a string builder from the pool
func (p *Processor) getStringBuilder() *strings.Builder {
	return internal.GetStringBuilder()
}

// putStringBuilder returns a string builder to the pool
func (p *Processor) putStringBuilder(sb *strings.Builder) {
	internal.PutStringBuilder(sb)
}

// Set sets a value in JSON at the specified path
// Returns:
//   - On success: modified JSON string and nil error
//   - On failure: original unmodified JSON string and error information
func (p *Processor) Set(jsonStr, path string, value any, cfg ...Config) (string, error) {
	if err := p.checkClosed(); err != nil {
		return jsonStr, err
	}

	options, err := p.prepareOptions(cfg...)
	if err != nil {
		return jsonStr, err
	}
	defer releaseConfig(options)

	if err := p.validateInput(jsonStr); err != nil {
		return jsonStr, err
	}

	if err := p.validatePath(path); err != nil {
		return jsonStr, err
	}

	// Parse JSON
	var data any
	err = p.Parse(jsonStr, &data, cfg...)
	if err != nil {
		return jsonStr, &JsonsError{
			Op:      "set",
			Path:    path,
			Message: fmt.Sprintf("failed to parse JSON: %v", err),
			Err:     err,
		}
	}

	// Note: We directly modify the parsed data without deep copy.
	// Parse() always creates fresh data via json.Unmarshal, so the cached
	// parse results from Get() are never shared with this scope.
	// If the operation fails, the original jsonStr string is returned unchanged.

	// Determine if we should create paths
	createPaths := options.CreatePaths || p.config.CreatePaths

	// Set the value at the specified path
	err = p.setValueAtPathWithOptions(data, path, value, createPaths)
	if err != nil {
		// Return original data and detailed error information
		var setError *JsonsError
		if _, ok := err.(*rootDataTypeConversionError); ok && createPaths {
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
	// PERFORMANCE: Use FastMarshalToString instead of json.Marshal to avoid
	// double allocation (bytes -> string) and leverage optimized encoder pools
	result, err := internal.FastMarshalToString(data)
	if err != nil {
		// Return original data if marshaling fails
		return jsonStr, &JsonsError{
			Op:      "set",
			Path:    path,
			Message: "failed to marshal modified data",
			Err:     err,
		}
	}

	return result, nil
}

// SetMultiple sets multiple values in JSON using a map of path-value pairs
// Returns:
//   - On success: modified JSON string and nil error
//   - On failure: original unmodified JSON string and error information
func (p *Processor) SetMultiple(jsonStr string, updates map[string]any, cfg ...Config) (string, error) {
	if err := p.checkClosed(); err != nil {
		return jsonStr, err
	}

	// Validate input
	if len(updates) == 0 {
		return jsonStr, nil // No updates to apply
	}

	// Prepare options
	options, err := p.prepareOptions(cfg...)
	if err != nil {
		return jsonStr, err
	}
	defer releaseConfig(options)

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
	err = p.Parse(jsonStr, &data, cfg...)
	if err != nil {
		return jsonStr, &JsonsError{
			Op:      "set_multiple",
			Message: fmt.Sprintf("failed to parse JSON: %v", err),
			Err:     err,
		}
	}

	// Create a deep copy of the data for modification attempts
	dataCopy, copyErr := deepCopy(data)
	if copyErr != nil {
		return jsonStr, &JsonsError{
			Op:      "set_multiple",
			Message: fmt.Sprintf("failed to create data copy: %v", copyErr),
			Err:     copyErr,
		}
	}

	// Determine if we should create paths
	createPaths := options.CreatePaths || p.config.CreatePaths

	// Apply all updates on the copy
	var lastError error
	successCount := 0

	for path, value := range updates {
		err := p.setValueAtPathWithOptions(dataCopy, path, value, createPaths)
		if err != nil {
			// Handle root data type conversion errors
			if _, ok := err.(*rootDataTypeConversionError); ok && createPaths {
				lastError = &JsonsError{
					Op:      "set_multiple",
					Path:    path,
					Message: fmt.Sprintf("root data type conversion failed for path '%s': %v", path, err),
					Err:     err,
				}
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

	// Convert modified data back to JSON string
	// PERFORMANCE: Use FastMarshalToString instead of json.Marshal
	result, err := internal.FastMarshalToString(dataCopy)
	if err != nil {
		// Return original data if marshaling fails
		return jsonStr, &JsonsError{
			Op:      "set_multiple",
			Message: "failed to marshal modified data",
			Err:     err,
		}
	}

	return result, nil
}

// SetCreate sets a value at the specified path, creating intermediate paths as needed.
// This is the unified API for set-with-path-creation operations.
//
// Example:
//
//	result, err := processor.SetCreate(data, "users[0].profile.name", "Alice")
func (p *Processor) SetCreate(jsonStr, path string, value any, cfg ...Config) (string, error) {
	addOpts := mergeOptionsWithOverride(cfg, func(o *Config) {
		o.CreatePaths = true
	})
	return p.Set(jsonStr, path, value, addOpts)
}

// SetMultipleCreate sets multiple values, creating intermediate paths as needed.
// This is the unified API for batch set-with-path-creation operations.
//
// Example:
//
//	result, err := processor.SetMultipleCreate(data, map[string]any{"user.name": "Alice", "user.age": 30})
func (p *Processor) SetMultipleCreate(jsonStr string, updates map[string]any, cfg ...Config) (string, error) {
	addOpts := mergeOptionsWithOverride(cfg, func(o *Config) {
		o.CreatePaths = true
	})
	return p.SetMultiple(jsonStr, updates, addOpts)
}

// GetStats returns processor performance statistics
func (p *Processor) GetStats() Stats {
	if p == nil {
		return Stats{}
	}
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

// EscapeJSONPointer escapes special characters for JSON Pointer
func escapeJSONPointer(s string) string {
	return internal.EscapeJSONPointer(s)
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

// ============================================================================
// COMPILED PATH METHODS
// PERFORMANCE: Pre-parsed paths for repeated operations with zero-parse overhead
// ============================================================================

// CompilePath compiles a JSON path string into a CompiledPath for fast repeated operations
// The returned CompiledPath can be reused for multiple Get/Set/Delete operations.
// Call Release() on the returned CompiledPath when done to return it to the pool.
func (p *Processor) CompilePath(path string) (*CompiledPath, error) {
	if err := p.checkClosed(); err != nil {
		return nil, err
	}

	// Use the global compiled path cache for frequently used paths
	return internal.GetGlobalCompiledPathCache().Get(path)
}

// GetCompiled retrieves a value from JSON using a pre-compiled path.
// PERFORMANCE: Skips path parsing for faster repeated operations.
func (p *Processor) GetCompiled(jsonStr string, cp *CompiledPath) (any, error) {
	if err := p.checkClosed(); err != nil {
		return nil, err
	}

	// Parse JSON once
	var data any
	if err := json.Unmarshal([]byte(jsonStr), &data); err != nil {
		return nil, err
	}

	// Navigate using compiled path
	return cp.Get(data)
}
