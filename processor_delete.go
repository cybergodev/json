package json

import (
	"encoding/json"
	"fmt"
)

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
