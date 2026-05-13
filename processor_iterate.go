package json

import (
	"github.com/cybergodev/json/internal"
)

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

// ForeachReturn iterates over JSON arrays or objects and returns the modified JSON string.
// The callback can modify IterableValue fields via Set method; changes are reflected in the result.
func (p *Processor) ForeachReturn(jsonStr string, fn func(key any, item *IterableValue)) (string, error) {
	if err := p.checkClosed(); err != nil {
		return "", err
	}

	data, err := p.Get(jsonStr, ".")
	if err != nil {
		return "", err
	}

	// Deep copy to avoid modifying cached parse data.
	// Get() may return a reference to cached parsed data for cache misses,
	// and modifying it in-place would corrupt subsequent cache lookups.
	dataCopy, copyErr := deepCopySubtree(data)
	if copyErr != nil {
		return jsonStr, copyErr
	}

	foreachWithIterableValue(dataCopy, fn)

	result, err := internal.FastMarshalToString(dataCopy)
	if err != nil {
		return jsonStr, err
	}
	return result, nil
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
// SECURITY: Applies all configured validation (size, depth, patterns) before parsing.
func (p *Processor) GetCompiled(jsonStr string, cp *CompiledPath) (any, error) {
	if err := p.checkClosed(); err != nil {
		return nil, err
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
