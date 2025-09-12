package json

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

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

	// Check if this is a complex path that needs special handling
	if p.needsLegacyComplexHandling(path) {
		// Use legacy complex path handling for compatibility
		result, err := p.navigateToPath(data, path)
		if err != nil {
			if err == ErrPathNotFound {
				// Record successful operation with null result
				if p.metrics != nil && p.metrics.collector != nil {
					p.metrics.collector.RecordOperation(time.Since(startTime), true, 0)
				}
				return nil, nil
			}
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

		// Record successful operation
		if p.metrics != nil && p.metrics.collector != nil {
			p.metrics.collector.RecordOperation(time.Since(startTime), true, 0)
		}
		return result, nil
	}

	// Note: Distributed operations are now handled by the unified recursive processor
	// which has better support for complex multi-layer distributed operations

	// Check if this needs legacy complex handling
	if p.needsLegacyComplexHandling(path) {
		result, err := p.navigateToPath(data, path)
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

	// Use unified recursive processor for simple paths
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

// needsLegacyComplexHandling is defined in navigation_utils.go

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

	// TODO: Implement parallel processing
	// if options.EnableParallel && len(paths) > 1 {
	//     return p.getMultipleParallel(data, paths, options)
	// }

	// Sequential processing
	results := make(map[string]any, len(paths))
	for _, path := range paths {
		if err := p.validatePath(path); err != nil {
			return nil, err
		}

		// Use the same logic as Get method for consistency
		var result any
		var err error

		if p.needsLegacyComplexHandling(path) {
			result, err = p.navigateToPath(data, path)
		} else {
			unifiedProcessor := NewRecursiveProcessor(p)
			result, err = unifiedProcessor.ProcessRecursively(data, path, OpGet, nil)
		}

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
				// Use the same logic as Get method for consistency
				var result any
				var err error

				if p.needsLegacyComplexHandling(currentPath) {
					result, err = p.navigateToPath(data, currentPath)
				} else {
					unifiedProcessor := NewRecursiveProcessor(p)
					result, err = unifiedProcessor.ProcessRecursively(data, currentPath, OpGet, nil)
				}

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
	if updates == nil || len(updates) == 0 {
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
	failedPaths := make([]string, 0)

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
