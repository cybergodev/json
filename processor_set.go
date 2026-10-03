package json

import (
	"errors"
	"fmt"
	"maps"
	"slices"
	"time"

	"github.com/cybergodev/json/internal"
)

// Set sets a value in JSON at the specified path
// Returns:
//   - On success: modified JSON string and nil error
//   - On failure: original unmodified JSON string and error information
func (p *Processor) Set(jsonStr, path string, value any, cfg ...Config) (result string, err error) {
	options, err := p.prepareOperation(jsonStr, path, cfg...)
	if err != nil {
		// Match Get's accounting: lifecycle rejections (closed processor,
		// concurrency limit) are not operation errors, but option and
		// input/path validation failures are.
		if !errors.Is(err, ErrProcessorClosed) && !errors.Is(err, ErrConcurrencyLimit) {
			p.incrementErrorCount()
		}
		return jsonStr, err
	}
	// Release in reverse-acquire order: options first, then governance slot.
	// Defers run LIFO, so endGovernedOp (registered first) runs last.
	defer p.endGovernedOp()
	defer releaseConfig(options)

	// Per-call custom parser: delegate (see delegateForPerCallParser).
	if q, derr := p.delegateForPerCallParser(options); derr != nil || q != nil {
		if derr != nil {
			p.incrementErrorCount()
			return jsonStr, derr
		}
		defer q.Close()
		return q.Set(jsonStr, path, value)
	}

	// Rate limiting, matching Get (D-002): the limit previously guarded only
	// reads, so writes bypassed it entirely — one more way the five
	// operations' prologues had drifted. No-op unless operationWindow > 0
	// (disabled by default).
	if p.metrics.operationWindow > 0 {
		if err := p.checkRateLimit(); err != nil {
			return jsonStr, err
		}
	}

	// Count the operation for stats. Get has always incremented the counters;
	// mutations previously went unreported, so GetStats() undercounted every
	// write. Failure paths below increment the error counter, as Get does.
	p.incrementOperationCount()

	// Metrics timing, matching Get (D-002): write latency/errors were absent
	// from RecordOperation, so GetStats/HealthChecker misrepresented
	// write-heavy workloads. No-op unless EnableMetrics is set.
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

	// Run registered hooks around the operation. A Before hook may abort; an
	// After hook may observe or transform the result/error. Registered last so
	// it unwinds first (hooks see the raw result). Per-call cfg.Hooks are
	// merged with the processor's hooks (hooksForOptions).
	hc := p.hooksForOptions(options)
	if len(hc) > 0 {
		hookCtx := HookContext{
			Operation: opNameSet,
			JSONStr:   jsonStr,
			Path:      path,
			Value:     value,
			Config:    options,
			StartTime: time.Now(),
		}
		if hookErr := hc.executeBefore(hookCtx); hookErr != nil {
			p.incrementErrorCount()
			return jsonStr, hookErr
		}
		defer func() {
			result, err = hc.executeAfterString(hookCtx, result, err)
		}()
	}

	data, err := p.parseJSON(jsonStr, "set", path, options)
	if err != nil {
		p.incrementErrorCount()
		return jsonStr, err
	}

	// Note: We directly modify the parsed data without deep copy.
	// Parse() always creates fresh data via json.Unmarshal, so the cached
	// parse results from Get() are never shared with this scope.
	// If the operation fails, the original jsonStr string is returned unchanged.

	// Determine if we should create paths. A per-call Config (when supplied)
	// fully overrides the processor's setting — including disabling CreatePaths
	// when the processor default has it on. When no Config is supplied, the
	// processor's own setting applies (not the global default singleton, which
	// would silently re-enable CreatePaths on a processor built with it off).
	createPaths := p.config.CreatePaths
	if len(cfg) > 0 {
		createPaths = options.CreatePaths
	}

	// Set the value at the specified path
	err = p.setValueAtPathWithOptions(data, path, value, createPaths)
	if err != nil {
		p.incrementErrorCount()
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

	// Invalidate cached results for this JSON string since the data changed
	p.invalidateJSONCache(jsonStr)

	// Convert modified data back to JSON string
	// PERFORMANCE: Use FastMarshalToString instead of json.Marshal to avoid
	// double allocation (bytes -> string) and leverage optimized encoder pools
	result, err = internal.FastMarshalToString(data)
	if err != nil {
		p.incrementErrorCount()
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
//   - With ContinueOnError=true and partial failures: the modified JSON
//     string (successes applied) and an errors.Join of every failed path
func (p *Processor) SetMultiple(jsonStr string, updates map[string]any, cfg ...Config) (result string, err error) {
	// Concurrency governance first (includes the closed-check via beginGovernedOp),
	// matching Set/Delete ordering. Previously SetMultiple only did a single
	// checkClosed() at entry, so it was neither concurrency-limited nor drained
	// by Close() — an inconsistency with Set/Delete.
	if err := p.beginGovernedOp(); err != nil {
		return jsonStr, err
	}
	defer p.endGovernedOp()

	// Validate input
	if len(updates) == 0 {
		return jsonStr, nil // No updates to apply
	}

	// Prepare options
	options, err := p.prepareOptions(cfg...)
	if err != nil {
		p.incrementErrorCount() // mirrors Get's prepareOptions-failure accounting
		return jsonStr, err
	}
	defer releaseConfig(options)

	// Batch size bound, matching ProcessBatch (D-002): an unbounded updates
	// map is the same memory-exhaustion vector MaxBatchSize exists for, but
	// SetMultiple never enforced it.
	if len(updates) > options.MaxBatchSize {
		p.incrementErrorCount()
		return jsonStr, &JsonsError{
			Op:      "set_multiple",
			Message: fmt.Sprintf("batch size %d exceeds maximum %d (Config.MaxBatchSize)", len(updates), options.MaxBatchSize),
			Err:     ErrSizeLimit,
		}
	}

	// Count the operation for stats — see Set for the rationale (mutations
	// previously went unreported, undercounting GetStats). Error returns below
	// increment the error counter, as Get does.
	p.incrementOperationCount()

	// Per-call custom parser: delegate (see delegateForPerCallParser).
	if q, derr := p.delegateForPerCallParser(options); derr != nil || q != nil {
		if derr != nil {
			p.incrementErrorCount()
			return jsonStr, derr
		}
		defer q.Close()
		return q.SetMultiple(jsonStr, updates)
	}

	// Run registered hooks around the batch operation (D-002): SetMultiple
	// previously ran NO hooks, so audit/transform coverage silently
	// disappeared for batch writes while Set/Delete had it. Before may abort;
	// After observes or transforms the (string) result.
	hc := p.hooksForOptions(options)
	if len(hc) > 0 {
		hookCtx := HookContext{
			Operation: "set_multiple",
			JSONStr:   jsonStr,
			Path:      fmt.Sprintf("(%d updates)", len(updates)),
			Config:    options,
			StartTime: time.Now(),
		}
		if hookErr := hc.executeBefore(hookCtx); hookErr != nil {
			p.incrementErrorCount()
			return jsonStr, hookErr
		}
		defer func() {
			result, err = hc.executeAfterString(hookCtx, result, err)
		}()
	}

	// Validate JSON input. Honor SkipValidation (essential DoS checks only) to
	// stay consistent with Set/Delete, which route through validateOperationInput.
	// Previously this called validateInputForOptions unconditionally, silently
	// ignoring SkipValidation — a behavioral divergence from Set.
	if options.SkipValidation {
		if err := p.validateInputEssential(jsonStr); err != nil {
			p.incrementErrorCount()
			return jsonStr, err
		}
	} else {
		if err := p.validateInputForOptions(jsonStr, options); err != nil {
			p.incrementErrorCount()
			return jsonStr, err
		}
	}

	// Validate all paths before processing. Path SYNTAX is always validated —
	// matching validateOperationInput's policy — so a malformed index can never
	// silently corrupt data even under SkipValidation. Under SkipValidation the
	// cheaper syntax-only internal.ValidatePath is used, as in the single-path path.
	// Iterate paths in sorted order (see sortedMapKeys): map iteration order
	// is randomized per call, which made the FIRST reported invalid path —
	// and therefore the returned error — nondeterministic.
	// The sorted path slice is computed once and reused by the application
	// loop below (it previously sorted the same keys a second time).
	sortedPaths := slices.Sorted(maps.Keys(updates))
	for _, path := range sortedPaths {
		var pathErr error
		if options.SkipValidation {
			pathErr = internal.ValidatePath(path)
		} else {
			pathErr = p.validatePath(path)
		}
		if pathErr != nil {
			p.incrementErrorCount()
			return jsonStr, &JsonsError{
				Op:      "set_multiple",
				Path:    path,
				Message: fmt.Sprintf("invalid path '%s': %v", path, pathErr),
				Err:     pathErr,
			}
		}
	}

	// Parse JSON
	var data any
	err = p.Parse(jsonStr, &data, *options)
	if err != nil {
		p.incrementErrorCount()
		return jsonStr, &JsonsError{
			Op:      "set_multiple",
			Message: fmt.Sprintf("failed to parse JSON: %v", err),
			Err:     err,
		}
	}

	// The parsed data is modified directly, without a deep copy — the same
	// contract Set documents. p.Parse always builds a fresh tree via
	// json.Unmarshal, so no cached parse result (Get's "parse:" entries) is
	// ever shared with this scope, and on failure the original jsonStr is
	// returned unchanged. The former defensive deepCopy doubled the memory
	// traffic of every SetMultiple call for no isolation benefit.

	// Determine if we should create paths. A per-call Config (when supplied)
	// fully overrides the processor's setting — including disabling CreatePaths
	// when the processor default has it on. When no Config is supplied, the
	// processor's own setting applies (not the global default singleton, which
	// would silently re-enable CreatePaths on a processor built with it off).
	createPaths := p.config.CreatePaths
	if len(cfg) > 0 {
		createPaths = options.CreatePaths
	}

	// Apply all updates on the parsed data
	//
	// failures accumulates per-path errors under ContinueOnError so a partial
	// batch is REPORTED (D-002): previously a 1-of-N failure with
	// ContinueOnError=true returned (modifiedJSON, nil), contradicting the
	// method contract and hiding that some updates never landed.
	var failures []error
	var lastError error
	successCount := 0

	// Sorted application order (see sortedMapKeys): updates are applied
	// sequentially to the same tree, so with overlapping keys (e.g. "a" and
	// "a.b") the final document previously depended on random map order.
	// Ascending order is deterministic: "a" is set before "a.b", so the
	// deeper path always lands in the freshly created container.
	for _, path := range sortedPaths {
		value := updates[path]
		err := p.setValueAtPathWithOptions(data, path, value, createPaths)
		if err != nil {
			// Handle root data type conversion errors
			if _, ok := err.(*rootDataTypeConversionError); ok && createPaths {
				lastError = &JsonsError{
					Op:      "set_multiple",
					Path:    path,
					Message: fmt.Sprintf("root data type conversion failed for path '%s': %v", path, err),
					Err:     err,
				}
				failures = append(failures, lastError)
				if !options.ContinueOnError {
					p.incrementErrorCount()
					return jsonStr, lastError
				}
			} else {
				lastError = &JsonsError{
					Op:      "set_multiple",
					Path:    path,
					Message: fmt.Sprintf("failed to set path '%s': %v", path, err),
					Err:     err,
				}
				failures = append(failures, lastError)
				if !options.ContinueOnError {
					p.incrementErrorCount()
					return jsonStr, lastError
				}
			}
		} else {
			successCount++
		}
	}

	// If no updates were successful and we have errors, return original data and error
	if successCount == 0 && lastError != nil {
		p.incrementErrorCount()
		return jsonStr, &JsonsError{
			Op:      "set_multiple",
			Message: fmt.Sprintf("all %d updates failed, last error: %v", len(updates), lastError),
			Err:     lastError,
		}
	}

	// Invalidate cached results for this JSON string since the data changed.
	// Mirrors Set(): without this, a subsequent Get on the same jsonStr could be
	// served stale parse/get results from cache.
	p.invalidateJSONCache(jsonStr)

	// Convert modified data back to JSON string
	// PERFORMANCE: Use FastMarshalToString instead of json.Marshal
	result, err = internal.FastMarshalToString(data)
	if err != nil {
		p.incrementErrorCount()
		// Return original data if marshaling fails
		return jsonStr, &JsonsError{
			Op:      "set_multiple",
			Message: "failed to marshal modified data",
			Err:     err,
		}
	}

	// Partial failure under ContinueOnError: successes were applied, so the
	// modified document is returned — but the caller must be able to detect
	// that some updates failed (see failures above). errors.Join carries
	// every failed path as a *JsonsError (D-002).
	if len(failures) > 0 {
		p.incrementErrorCount()
		return result, errors.Join(failures...)
	}

	return result, nil
}

// SetCreate sets a value at the specified path, creating intermediate paths as
// needed. It is a convenience wrapper: SetCreate(s, p, v, cfg) is exactly
// Set(s, p, v, cfg') where cfg' is cfg with CreatePaths forced to true.
//
// Example:
//
//	result, err := processor.SetCreate(data, "users[0].profile.name", "Alice")
func (p *Processor) SetCreate(jsonStr, path string, value any, cfg ...Config) (string, error) {
	addOpts := p.mergeOptionsWithOverride(cfg, func(o *Config) {
		o.CreatePaths = true
	})
	return p.Set(jsonStr, path, value, addOpts)
}

// SetMultipleCreate sets multiple values, creating intermediate paths as needed.
// It is a convenience wrapper: SetMultipleCreate(s, u, cfg) is exactly
// SetMultiple(s, u, cfg') where cfg' is cfg with CreatePaths forced to true.
//
// Example:
//
//	result, err := processor.SetMultipleCreate(data, map[string]any{"user.name": "Alice", "user.age": 30})
func (p *Processor) SetMultipleCreate(jsonStr string, updates map[string]any, cfg ...Config) (string, error) {
	addOpts := p.mergeOptionsWithOverride(cfg, func(o *Config) {
		o.CreatePaths = true
	})
	return p.SetMultiple(jsonStr, updates, addOpts)
}
