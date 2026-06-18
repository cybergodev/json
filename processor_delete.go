package json

import (
	"github.com/cybergodev/json/internal"
)

// Delete removes a value from JSON at the specified path
func (p *Processor) Delete(jsonStr, path string, cfg ...Config) (string, error) {
	options, err := p.prepareOperation(jsonStr, path, cfg...)
	if err != nil {
		// Return the original input on failure, matching every other error path
		// in this method and the contract documented by Set/SetMultiple.
		return jsonStr, err
	}
	// Release in reverse-acquire order: options first, then governance slot.
	defer p.endGovernedOp()
	defer releaseConfig(options)

	// Determine cleanup options from prepared options and config
	cleanupNulls := options.CleanupNulls || p.config.CleanupNulls
	compactArrays := options.CompactArrays || p.config.CompactArrays

	// PERFORMANCE: Fast path for simple property delete without cache or cleanup.
	if isSimplePropertyAccess(path) && !p.config.EnableCache && len(cfg) == 0 && !cleanupNulls {
		m, isObj, err := unmarshalRootObject(jsonStr)
		if err != nil {
			return jsonStr, newOperationPathError("delete", path, err.Error(), ErrInvalidJSON)
		}
		if isObj {
			if _, exists := m[path]; !exists {
				return jsonStr, newOperationPathError("delete", path, "path not found", ErrPathNotFound)
			}
			delete(m, path)
			result, err := internal.FastMarshalToString(m)
			if err != nil {
				return jsonStr, newOperationPathError("delete", path, "failed to marshal result", err)
			}
			return result, nil
		}
		// Not an object — fall through to full recursive processor
	}

	// Parse JSON using unified helper
	data, err := p.parseJSON(jsonStr, "delete", path, options, cfg...)
	if err != nil {
		return jsonStr, err
	}

	// If compactArrays is enabled, automatically enable cleanupNulls
	if compactArrays {
		cleanupNulls = true
	}

	// Check if path contains array access - only then we need DeletedMarker cleanup
	needsMarkerCleanup := p.isArrayDeletePath(path)

	// Delete the value at the specified path
	err = p.deleteValueAtPath(data, path)
	if err != nil {
		return jsonStr, &JsonsError{
			Op:      "delete",
			Path:    path,
			Message: err.Error(),
			Err:     err,
		}
	}

	// Only clean up deleted markers if the path involved array operations
	if needsMarkerCleanup {
		data = p.cleanupDeletedMarkers(data)
	}

	// Invalidate cached results for this JSON string since the data changed
	p.invalidateJSONCache(jsonStr)

	// Cleanup nulls if requested
	if cleanupNulls {
		data = p.cleanupNullValuesWithReconstruction(data, compactArrays)
	}

	// Convert back to JSON string
	result, err := internal.FastMarshalToString(data)
	if err != nil {
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
	for i := 0; i < len(path); i++ {
		if path[i] == '[' {
			return true
		}
	}
	return false
}

// DeleteClean removes a value from JSON and cleans up null placeholders.
func (p *Processor) DeleteClean(jsonStr, path string, cfg ...Config) (string, error) {
	cleanupOpts := mergeOptionsWithOverride(cfg, func(o *Config) {
		o.CleanupNulls = true
		o.CompactArrays = true
	})
	return p.Delete(jsonStr, path, cleanupOpts)
}
