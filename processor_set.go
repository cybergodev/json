package json

import (
	"encoding/json"
	"fmt"
)

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
