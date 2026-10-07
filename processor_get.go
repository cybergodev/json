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
	// Concurrency governance: register the in-flight op (so Close() can drain it
	// via waitForActiveOps) and acquire the semaphore. Released by endGovernedOp.
	if err := p.beginGovernedOp(); err != nil {
		return nil, err
	}
	defer p.endGovernedOp()

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

	// Per-call custom parser: delegate (see delegateForPerCallParser).
	if q, derr := p.delegateForPerCallParser(options); derr != nil || q != nil {
		if derr != nil {
			p.incrementErrorCount()
			return nil, derr
		}
		defer q.Close()
		return q.Get(jsonStr, path)
	}

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

	// Defer slow operation logging. Only active when EnableMetrics=true —
	// startTime is set solely by the metrics prologue above (D-002/R8 m8: the
	// old comment claimed logger-level gating, which is the SECOND gate; the
	// log level then decides visibility: Debug normally, Warn above
	// slowOperationThreshold).
	// PERF: ctx is created lazily only when slow operation is detected
	defer func() {
		if !startTime.IsZero() {
			p.logOperation(context.Background(), "get", path, time.Since(startTime))
		}
	}()

	// Run registered hooks around the operation. A Before hook may abort the
	// operation by returning an error; an After hook may observe or transform the
	// result/error. This defer is registered last so it unwinds first, letting
	// hooks see the raw result before metrics/logging cleanup run. Per-call
	// cfg.Hooks are merged with the processor's hooks (hooksForOptions).
	hc := p.hooksForOptions(options)
	if len(hc) > 0 {
		hookCtx := HookContext{
			Operation: opNameGet,
			JSONStr:   jsonStr,
			Path:      path,
			Config:    options,
			StartTime: time.Now(),
		}
		if hookErr := hc.executeBefore(hookCtx); hookErr != nil {
			p.incrementErrorCount()
			return nil, hookErr
		}
		defer func() {
			result, err = hc.executeAfter(hookCtx, result, err)
		}()
	}

	// PERFORMANCE (P-001): hash the document ONCE when the result cache is
	// enabled and share it with the validation cache and both cache keys below —
	// previously the input was FNV-scanned by ValidateJSONInput and again by
	// Get. With the cache disabled Get never needs a hash, so validation
	// computes its own exactly as before.
	jsonHash := uint64(0)
	if p.config.EnableCache {
		jsonHash = hashStringToUint64(jsonStr)
	}
	// Validate input using unified helper (handles SkipValidation internally)
	if err := p.validateOperationInputHashed(jsonStr, path, options, jsonHash, p.config.EnableCache); err != nil {
		p.incrementErrorCount()
		return nil, err
	}

	// PERFORMANCE: Fast path for simple property access without cache overhead.
	// Bypasses hash computation, cache key creation, and recursive processor
	// for the most common case: single-key lookup on a JSON object.
	//
	// PreserveNumbers must be off for this path: unmarshalRootObject uses stdlib
	// json.Unmarshal which always yields float64, so a big-integer property would
	// lose precision here. When PreserveNumbers is on, fall through to parseJSON
	// (which routes to p.Parse and preserves json.Number).
	if isSimplePropertyAccess(path) && !p.config.EnableCache && !p.config.PreserveNumbers && len(cfg) == 0 &&
		p.config.CustomPathParser == nil { // custom syntax: never simple (D-002/M33)
		m, isObj, parseErr := unmarshalRootObject(jsonStr)
		if parseErr != nil {
			p.incrementErrorCount()
			return nil, &JsonsError{
				Op:      "get",
				Path:    path,
				Message: parseErr.Error(),
				Err:     ErrInvalidJSON,
			}
		}
		if isObj {
			if val, exists := m[path]; exists {
				return val, nil
			}
			return nil, ErrPathNotFound
		}
		// Not an object — fall through to full recursive processor
	}

	// PERFORMANCE: Skip hash and cache operations when cache is disabled
	if !p.config.EnableCache {
		data, parseErr := p.parseJSON(jsonStr, "get", path, options)
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

	// Check cache after validation. jsonHash was computed once above (P-001)
	// and shared with the validation-cache lookup.
	cacheKey := p.createCacheKeyWithHash("get", jsonHash, path, options)
	if cached, ok := p.getCachedResult(cacheKey); ok {
		// Record cache hit operation
		if metricsCollector != nil {
			metricsCollector.RecordCacheHit()
		}
		// PERFORMANCE: Skip deep copy for JSON primitives (immutable types).
		// For parsed JSON data, only map[string]any and []any need copying.
		// This avoids the deepCopySubtree overhead for ~60% of Get results.
		// D-002/R8 (m1): the library's Number is an immutable string-kind leaf
		// too (aligned with safeCopyResult) — include it so PreserveNumbers
		// hits skip a pointless copy.
		switch cached.(type) {
		case nil, bool, float64, string, json.Number, Number:
			return cached, nil
		}
		// PERFORMANCE: When CacheSharedResults is enabled, return the cached
		// value directly. The caller has opted into the "do not mutate" contract
		// (see Config.CacheSharedResults), so the defensive deep copy — the
		// dominant cost of repeated Gets on large results — is skipped entirely.
		if p.config.CacheSharedResults {
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
		// Parse JSON with error context. parseJSON (not p.Parse): the input was
		// already validated by validateOperationInput, and dereferencing options
		// into p.Parse would build a transient securityValidator and re-run the
		// full uncached validation on every parse-cache miss (P-001) — the same
		// helper the simple-property fast path above uses.
		var parseErr error
		data, parseErr = p.parseJSON(jsonStr, "get", path, options)
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

	// Cache result if enabled. The result aliases the shared parse tree (or
	// the cached parse entry), so the caller must receive an independent copy —
	// symmetric with the hit path above. Without this, the FIRST caller held
	// the very map/slice stored in the get: cache (and aliased into the
	// parse: cache), and any mutation poisoned every subsequent hit (D-002).
	p.setCachedResult(cacheKey, result, options)
	if p.config.CacheSharedResults {
		// Caller opted into the shared, do-not-mutate contract (see hit path).
		return result, nil
	}
	switch result.(type) {
	case nil, bool, float64, string, json.Number, Number:
		// Immutable JSON primitives — no copy needed (mirrors hit path,
		// including the library's Number; D-002/R8 m1).
		return result, nil
	}
	copied, copyErr := deepCopySubtree(result)
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
// SHARED DATA (P-002): the tree behind Data() is the parse-cache entry itself
// (zero-copy) and may be read concurrently by other PreParse/GetFromParsed
// callers on the same processor. It MUST NOT be mutated — mutation poisons the
// cache for every reader. GetFromParsed remains safe for value extraction by
// default (results are copied unless Config.CacheSharedResults is set); for
// mutating workflows use Get (which deep-copies results) or copy what Data()
// returns before changing it.
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
	// D-002/R9 (m3): governance — PreParse reads/writes the parse cache and
	// validator. parseJSON (its heavy path) is ungoverned, so no nesting.
	if err := p.beginGovernedOp(); err != nil {
		return nil, err
	}
	defer p.endGovernedOp()

	options, err := p.prepareOptions(cfg...)
	if err != nil {
		return nil, err
	}
	defer releaseConfig(options)

	// Validate input and build the parse-cache key in one step (P-001: one FNV
	// scan of the document instead of two — validation self-hash + key build).
	parseCacheKey, err := p.validateAndCacheKey("parse", jsonStr, options)
	if err != nil {
		return nil, err
	}
	var data any

	if cachedData, ok := p.getCachedResult(parseCacheKey); ok {
		data = cachedData
	} else {
		// Parse JSON. parseJSON (not p.Parse): the input was validated above,
		// so the sentinel-dereferencing p.Parse call would re-validate through
		// a transient securityValidator on every parse-cache miss (P-001).
		var parseErr error
		data, parseErr = p.parseJSON(jsonStr, "pre_parse", "", options)
		if parseErr != nil {
			return nil, parseErr
		}

		// Cache parsed data - PERFORMANCE: Use internal method to skip sensitive check
		if p.config.EnableCache {
			p.setCachedResultInternal(parseCacheKey, data)
		}
	}

	// P-002: the tree is returned AS-IS (zero-copy) — this is what makes
	// PreParse a performance optimization. It is shared with the parse cache
	// and concurrent GetFromParsed readers; see the Data() contract. Copying
	// here was measured at ~47x the hit-path cost on a 100KB document
	// (BenchmarkPreParse_Large 1.3ms vs 28µs), an unacceptable regression for
	// the API's stated purpose.
	return &ParsedJSON{
		data: data,
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

	// D-002/R9 (m3): governance — navigation-only, but registered so Close()
	// drains it like every other op (recursiveProcessor and the validator it
	// reads are processor state). ProcessRecursively is ungoverned: no nesting.
	if err := p.beginGovernedOp(); err != nil {
		return nil, err
	}
	defer p.endGovernedOp()

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

	// Protect cached parsed data from caller mutation.
	// PERFORMANCE: Skip the copy when CacheSharedResults is enabled (caller has
	// opted into the "do not mutate" contract — see Config.CacheSharedResults).
	if !p.config.CacheSharedResults {
		result = safeCopyResult(result)
	}

	// NOTE: no cache write here. Processor.Get looks up CacheKey{Op: "get", ...}
	// keys built from the document hash, but ParsedJSON no longer carries a
	// content hash, so any key built here could never be read back — it would
	// only pollute the cache and evict live entries.

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

	// D-002/R9 (m3): governance — see GetFromParsed (mutation variant).
	if err := p.beginGovernedOp(); err != nil {
		return nil, err
	}
	defer p.endGovernedOp()

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

	// opSet mutates dataCopy in place (the same contract Set relies on); the
	// value returned by ProcessRecursivelyWithOptions is the assigned value, not
	// the modified document. Returning `result` here was a bug: it made the new
	// ParsedJSON hold only the set value, so a follow-up GetFromParsed could not
	// read any other path. The modified root lives in dataCopy.
	//
	// D-002/R10: no-cfg resolves CreatePaths from the baked config (mirrors
	// Set) — the singleton's true previously re-enabled path creation on a
	// processor built with CreatePaths=false.
	createPaths := p.config.CreatePaths
	if options != &defaultConfigSingleton {
		createPaths = options.CreatePaths
	}
	_, err = p.recursiveProcessor.ProcessRecursivelyWithOptions(dataCopy, path, opSet, value, createPaths)
	if err != nil {
		return nil, &JsonsError{
			Op:      "set_from_parsed",
			Path:    path,
			Message: err.Error(),
			Err:     err,
		}
	}

	return &ParsedJSON{
		data: dataCopy,
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
func (p *Processor) GetMultiple(jsonStr string, paths []string, cfg ...Config) (results map[string]any, err error) {
	// D-002/R11 (M1): concurrency governance — GetMultiple was the one batch
	// read never registered as an in-flight op, so Close() could complete (and
	// release resources) mid-run while every other read/mutation drains, and
	// MaxConcurrency / MaxOperationsPerSecond did not apply to it. parseJSON,
	// validateInputForOptions, and the recursive engine are ungoverned callees,
	// so this acquire never nests.
	if err := p.beginGovernedOp(); err != nil {
		return nil, err
	}
	defer p.endGovernedOp()

	// Rate limiting, matching Get (D-002). No-op unless operationWindow > 0
	// (disabled by default).
	if p.metrics.operationWindow > 0 {
		if err := p.checkRateLimit(); err != nil {
			return nil, err
		}
	}

	options, err := p.prepareOptions(cfg...)
	if err != nil {
		p.incrementErrorCount() // mirrors Get's prepareOptions-failure accounting
		return nil, err
	}
	defer releaseConfig(options)

	// Count the operation for stats — see Set for the rationale. Get has
	// always counted; GetMultiple previously did not.
	p.incrementOperationCount()

	// Metrics timing + slow-operation logging, matching Get (D-002/R11 M1
	// 回查): batch reads were the last operations absent from
	// RecordOperation/GetStats accounting and from slow-op warnings. Both are
	// no-ops unless EnableMetrics is set. Registered BEFORE the hooks defer so
	// hooks unwind first and observe the raw result, mirroring Get.
	var metricsCollector *internal.MetricsCollector
	var startTime time.Time
	if p.metrics != nil && p.metrics.enabled {
		metricsCollector = p.metrics.collector
		if metricsCollector != nil {
			startTime = time.Now()
			metricsCollector.StartConcurrentOperation()
		}
	}
	defer func() {
		if metricsCollector != nil {
			metricsCollector.EndConcurrentOperation()
			if !startTime.IsZero() {
				metricsCollector.RecordOperation(time.Since(startTime), err == nil, 0)
			}
		}
	}()
	defer func() {
		if !startTime.IsZero() {
			p.logOperation(context.Background(), "get_multiple", fmt.Sprintf("(%d paths)", len(paths)), time.Since(startTime))
		}
	}()

	// Run registered hooks around the batch operation (D-002): GetMultiple
	// previously ran NO hooks, so audit/transform coverage silently
	// disappeared for batch reads while Get had it. Before may abort; After
	// observes or transforms the results map.
	hc := p.hooksForOptions(options)
	if len(hc) > 0 {
		hookCtx := HookContext{
			Operation: "get_multiple",
			JSONStr:   jsonStr,
			Path:      fmt.Sprintf("(%d paths)", len(paths)),
			Config:    options,
			StartTime: time.Now(),
		}
		if hookErr := hc.executeBefore(hookCtx); hookErr != nil {
			p.incrementErrorCount()
			return nil, hookErr
		}
		defer func() {
			// Coerce like executeAfterString: a non-map After result is a
			// no-op on the result (original kept); the error still propagates.
			r, e := hc.executeAfter(hookCtx, results, err)
			if m, ok := r.(map[string]any); ok {
				results = m
			}
			err = e
		}()
	}

	if err := p.validateInputForOptions(jsonStr, options); err != nil {
		p.incrementErrorCount()
		return nil, err
	}

	if len(paths) == 0 {
		return make(map[string]any), nil
	}

	// Parse JSON once for all operations. parseJSON (not p.Parse): the input
	// was validated above; p.Parse would re-validate through a transient
	// securityValidator per call (P-001).
	data, err := p.parseJSON(jsonStr, "get_multiple", "", options)
	if err != nil {
		p.incrementErrorCount()
		return nil, err
	}

	// Sequential processing
	results = make(map[string]any, len(paths))
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

	if firstError != nil {
		p.incrementErrorCount()
	}
	return results, firstError
}
