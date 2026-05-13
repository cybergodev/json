package json

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/cybergodev/json/internal"
)

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

	// Determine the type (nil check first — fmt.Sprintf("%T", nil) returns "<nil>", not "null")
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

	// Enforce concurrency limit
	if err := p.acquireSemaphore(); err != nil {
		return nil, err
	}
	defer p.releaseSemaphore()

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

	// Defer slow operation logging (no-op when logger is at default level)
	// PERF: ctx is created lazily only when slow operation is detected
	defer func() {
		if !startTime.IsZero() {
			p.logOperation(context.Background(), "get", path, time.Since(startTime))
		}
	}()

	// Validate input using unified helper (handles SkipValidation internally)
	if err := p.validateOperationInput(jsonStr, path, options); err != nil {
		p.incrementErrorCount()
		return nil, err
	}

	// PERFORMANCE: Fast path for simple property access without cache overhead.
	// Bypasses hash computation, cache key creation, and recursive processor
	// for the most common case: single-key lookup on a JSON object.
	if isSimplePropertyAccess(path) && !p.config.EnableCache && len(cfg) == 0 {
		var data any
		if parseErr := json.Unmarshal(internal.StringToBytes(jsonStr), &data); parseErr != nil {
			p.incrementErrorCount()
			return nil, &JsonsError{
				Op:      "get",
				Path:    path,
				Message: parseErr.Error(),
				Err:     ErrInvalidJSON,
			}
		}
		if m, ok := data.(map[string]any); ok {
			if val, exists := m[path]; exists {
				return val, nil
			}
			return nil, ErrPathNotFound
		}
		// Not an object — fall through to full recursive processor
	}

	// PERFORMANCE: Skip hash and cache operations when cache is disabled
	if !p.config.EnableCache {
		data, parseErr := p.parseJSON(jsonStr, "get", path, options, cfg...)
		if parseErr != nil {
			p.incrementErrorCount()
			return nil, parseErr
		}

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
		return result, nil
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
		copied, copyErr := deepCopySubtree(cached)
		if copyErr != nil {
			p.incrementErrorCount()
			return nil, &JsonsError{
				Op:      "get",
				Path:    path,
				Message: fmt.Sprintf("cache copy failed: %v", copyErr),
				Err:     copyErr,
			}
		}
		return copied, nil
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
		parseErr := p.Parse(jsonStr, &data, *options)
		if parseErr != nil {
			p.incrementErrorCount()
			return nil, parseErr
		}

		// Cache parsed data for reuse - always cache if global cache is enabled
		// PERFORMANCE: Use setCachedResultInternal to skip sensitive data check
		// since this is trusted internal data from our parser
		p.setCachedResultInternal(parseCacheKey, data)
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

// GetWithContext retrieves a value from JSON with boundary-level context checks.
// Context is checked before and after the operation, NOT during parsing/navigation.
// For large JSON documents, the operation may not respond to cancellation mid-parse.
// This is the context-aware version of Get() that supports timeout deadlines.
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
		parseErr := p.Parse(jsonStr, &data, *options)
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

	// Protect cached parsed data from caller mutation
	result = safeCopyResult(result)

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
	return getTypedWithDefault(p, jsonStr, path, defaultValue...)
}

// GetInt retrieves an int value from JSON at the specified path.
// Returns defaultValue if provided, otherwise 0 when: path not found, value is null, or type conversion fails.
func (p *Processor) GetInt(jsonStr, path string, defaultValue ...int) int {
	return getTypedWithDefault(p, jsonStr, path, defaultValue...)
}

// GetFloat retrieves a float64 value from JSON at the specified path.
// Returns defaultValue if provided, otherwise 0.0 when: path not found, value is null, or type conversion fails.
func (p *Processor) GetFloat(jsonStr, path string, defaultValue ...float64) float64 {
	return getTypedWithDefault(p, jsonStr, path, defaultValue...)
}

// GetBool retrieves a bool value from JSON at the specified path.
// Returns defaultValue if provided, otherwise false when: path not found, value is null, or type conversion fails.
func (p *Processor) GetBool(jsonStr, path string, defaultValue ...bool) bool {
	return getTypedWithDefault(p, jsonStr, path, defaultValue...)
}

// GetArray retrieves an array value from JSON at the specified path.
// Returns defaultValue if provided, otherwise nil when: path not found, value is null, or type conversion fails.
func (p *Processor) GetArray(jsonStr, path string, defaultValue ...[]any) []any {
	return getTypedWithDefault(p, jsonStr, path, defaultValue...)
}

// GetObject retrieves an object value from JSON at the specified path.
// Returns defaultValue if provided, otherwise nil when: path not found, value is null, or type conversion fails.
func (p *Processor) GetObject(jsonStr, path string, defaultValue ...map[string]any) map[string]any {
	return getTypedWithDefault(p, jsonStr, path, defaultValue...)
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
	if err := p.Parse(jsonStr, &data, *options); err != nil {
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
