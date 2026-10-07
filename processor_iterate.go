package json

import (
	"fmt"

	"github.com/cybergodev/json/internal"
)

// iterRoot resolves the iteration root for the Foreach* family: closed-check,
// Get at path, then a deep copy so mutation callbacks cannot corrupt cached
// parse data. If the (practically unreachable) copy fails, the error is
// returned instead of the original data: with CacheSharedResults=true Get
// returns the cached tree itself, and handing it to mutation callbacks would
// poison it for concurrent readers (P-002) — mirroring safeCopyResult's
// "return nil rather than alias" discipline.
func (p *Processor) iterRoot(jsonStr, path string, cfg ...Config) (any, error) {
	if err := p.checkClosed(); err != nil {
		return nil, err
	}
	data, err := p.Get(jsonStr, path, cfg...)
	if err != nil {
		return nil, err
	}
	dataCopy, copyErr := deepCopySubtree(data)
	if copyErr == nil {
		return dataCopy, nil
	}
	return nil, fmt.Errorf("foreach: failed to isolate iteration root from cached data: %w", copyErr)
}

// Foreach iterates over JSON arrays or objects using this processor
//
// Deprecated: Foreach drops errors — an invalid document, a failed path
// resolution, or a closed processor silently skips the callback entirely.
// Use ForeachWithError, which returns the error (its callback must return
// nil to continue):
//
//	err := p.ForeachWithError(jsonStr, ".", func(key any, item *json.IterableValue) error {
//	    // ...
//	    return nil
//	})
//
// Foreach will not be removed within v1 (per D-005 the module stays on v1.x).
func (p *Processor) Foreach(jsonStr string, fn func(key any, item *IterableValue), cfg ...Config) {
	data, err := p.iterRoot(jsonStr, ".", cfg...)
	if err != nil {
		return
	}
	foreachWithIterableValue(data, fn)
}

// ForeachWithPath iterates over JSON arrays or objects at a specific path using this processor
// This allows using custom processor configurations (security limits, nesting depth, etc.)
func (p *Processor) ForeachWithPath(jsonStr, path string, fn func(key any, item *IterableValue), cfg ...Config) error {
	data, err := p.iterRoot(jsonStr, path, cfg...)
	if err != nil {
		return err
	}
	foreachWithIterableValue(data, fn)
	return nil
}

// ForeachWithPathAndIterator iterates over JSON at a path with path information
func (p *Processor) ForeachWithPathAndIterator(jsonStr, path string, fn func(key any, item *IterableValue, currentPath string) IteratorControl, cfg ...Config) error {
	data, err := p.iterRoot(jsonStr, path, cfg...)
	if err != nil {
		return err
	}
	return foreachWithPathIterableValue(data, "", fn)
}

// ForeachWithPathAndControl iterates with control over iteration flow
func (p *Processor) ForeachWithPathAndControl(jsonStr, path string, fn func(key any, value any) IteratorControl, cfg ...Config) error {
	data, err := p.iterRoot(jsonStr, path, cfg...)
	if err != nil {
		return err
	}
	return foreachOnValue(data, fn)
}

// ForeachReturn iterates over JSON arrays or objects and returns the modified JSON string.
// The callback can mutate the iterated containers via the value returned by
// item.GetData() (a reference into the working copy): changes to maps/slices
// are reflected in the marshaled result. Replacing scalars in place is not
// possible through the IterableValue itself.
func (p *Processor) ForeachReturn(jsonStr string, fn func(key any, item *IterableValue), cfg ...Config) (string, error) {
	data, err := p.iterRoot(jsonStr, ".", cfg...)
	if err != nil {
		return "", err
	}
	foreachWithIterableValue(data, fn)
	result, err := internal.FastMarshalToString(data)
	if err != nil {
		return jsonStr, err
	}
	// D-002/R11 (M2, option A): like Set/Delete, the re-encoded output honors
	// MaxJSONSize (a mutation callback can grow the document past the limit).
	// cfg arrives raw here (no prepareOptions validation), so only a positive
	// per-call value overrides the baked limit.
	maxSize := p.config.MaxJSONSize
	if len(cfg) > 0 && cfg[0].MaxJSONSize > 0 {
		maxSize = cfg[0].MaxJSONSize
	}
	if err := p.checkMutationOutputSize(result, maxSize, "foreach_return", ""); err != nil {
		return jsonStr, err
	}
	return result, nil
}

// ForeachNested recursively iterates over all nested JSON structures
// This method traverses through all nested objects and arrays
//
// Deprecated: ForeachNested drops errors the same way Foreach does — a failed
// parse or closed processor silently skips the callback. Use
// ForeachNestedWithError, which returns the error (its callback must return
// nil to continue). ForeachNested will not be removed within v1 (per D-005
// the module stays on v1.x).
func (p *Processor) ForeachNested(jsonStr string, fn func(key any, item *IterableValue), cfg ...Config) {
	data, err := p.iterRoot(jsonStr, ".", cfg...)
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
func (p *Processor) ForeachWithError(jsonStr, path string, fn func(key any, item *IterableValue) error, cfg ...Config) error {
	data, err := p.iterRoot(jsonStr, path, cfg...)
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
func (p *Processor) ForeachNestedWithError(jsonStr string, fn func(key any, item *IterableValue) error, cfg ...Config) error {
	data, err := p.iterRoot(jsonStr, ".", cfg...)
	if err != nil {
		return err
	}
	return foreachNestedOnValueError(data, fn)
}

// ============================================================================
// COMPILED PATH METHODS
// PERFORMANCE: Pre-parsed paths for repeated operations with zero-parse overhead
// ============================================================================

// CompilePath compiles a JSON path string into a CompiledPath for fast repeated operations.
// The returned CompiledPath can be reused for multiple GetCompiled operations
// (Set/Delete variants do not exist yet).
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
// SECURITY: Applies all configured validation (size, depth, patterns) before parsing.
func (p *Processor) GetCompiled(jsonStr string, cp *CompiledPath) (any, error) {
	if err := p.checkClosed(); err != nil {
		return nil, err
	}

	// Guard the parameter before navigation: a nil *CompiledPath would panic
	// inside cp.Get (nil receiver dereferences cp.segments). The closed-
	// processor check above returned early for this case, masking the panic
	// in tests — on an active processor it crashed.
	if cp == nil {
		return nil, &JsonsError{
			Op:      "get_compiled",
			Message: "compiled path is nil",
			Err:     errOperationFailed,
		}
	}

	if err := p.validateInput(jsonStr); err != nil {
		return nil, err
	}

	var data any
	if err := p.Parse(jsonStr, &data); err != nil {
		return nil, err
	}

	return cp.Get(data)
}
