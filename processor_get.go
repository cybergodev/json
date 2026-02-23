package json

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// SafeGet performs a type-safe get operation with comprehensive error handling
func (p *Processor) SafeGet(jsonStr, path string) TypeSafeAccessResult {
	// Validate inputs
	if jsonStr == "" {
		return TypeSafeAccessResult{Exists: false}
	}
	if path == "" {
		return TypeSafeAccessResult{Exists: false}
	}

	// Perform the get operation
	value, err := p.Get(jsonStr, path)
	if err != nil {
		return TypeSafeAccessResult{Exists: false}
	}

	// Determine the type
	var valueType string
	if value == nil {
		valueType = "null"
	} else {
		valueType = fmt.Sprintf("%T", value)
	}

	return TypeSafeAccessResult{
		Value:  value,
		Exists: true,
		Type:   valueType,
	}
}

// SafeGetTypedWithProcessor performs a type-safe get operation with generic type constraints
func SafeGetTypedWithProcessor[T any](p *Processor, jsonStr, path string) TypeSafeResult[T] {
	var zero T

	// Validate inputs
	if jsonStr == "" || path == "" {
		return TypeSafeResult[T]{Value: zero, Exists: false, Error: ErrPathNotFound}
	}

	// Perform the get operation
	value, err := p.Get(jsonStr, path)
	if err != nil {
		return TypeSafeResult[T]{Value: zero, Exists: false, Error: err}
	}

	// Type assertion with safety
	if typedValue, ok := value.(T); ok {
		return TypeSafeResult[T]{Value: typedValue, Exists: true, Error: nil}
	}

	// Attempt type conversion
	if converted, err := TypeSafeConvert[T](value); err == nil {
		return TypeSafeResult[T]{Value: converted, Exists: true, Error: nil}
	}

	return TypeSafeResult[T]{
		Value:  zero,
		Exists: true,
		Error:  fmt.Errorf("type mismatch: expected %T, got %T", zero, value),
	}
}

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

	// Use unified recursive processor for all paths
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

// GetString retrieves a string value from JSON at the specified path
func (p *Processor) GetString(jsonStr, path string, opts ...*ProcessorOptions) (string, error) {
	return GetTypedWithProcessor[string](p, jsonStr, path, opts...)
}

// GetInt retrieves an int value from JSON at the specified path
func (p *Processor) GetInt(jsonStr, path string, opts ...*ProcessorOptions) (int, error) {
	return GetTypedWithProcessor[int](p, jsonStr, path, opts...)
}

// GetFloat64 retrieves a float64 value from JSON at the specified path
func (p *Processor) GetFloat64(jsonStr, path string, opts ...*ProcessorOptions) (float64, error) {
	return GetTypedWithProcessor[float64](p, jsonStr, path, opts...)
}

// GetBool retrieves a bool value from JSON at the specified path
func (p *Processor) GetBool(jsonStr, path string, opts ...*ProcessorOptions) (bool, error) {
	return GetTypedWithProcessor[bool](p, jsonStr, path, opts...)
}

// GetArray retrieves an array value from JSON at the specified path
func (p *Processor) GetArray(jsonStr, path string, opts ...*ProcessorOptions) ([]any, error) {
	return GetTypedWithProcessor[[]any](p, jsonStr, path, opts...)
}

// GetObject retrieves an object value from JSON at the specified path
func (p *Processor) GetObject(jsonStr, path string, opts ...*ProcessorOptions) (map[string]any, error) {
	return GetTypedWithProcessor[map[string]any](p, jsonStr, path, opts...)
}

// GetWithDefault retrieves a value from JSON with a default fallback
func (p *Processor) GetWithDefault(jsonStr, path string, defaultValue any, opts ...*ProcessorOptions) any {
	value, err := p.Get(jsonStr, path, opts...)
	if err != nil || value == nil {
		return defaultValue
	}
	return value
}

// GetStringWithDefault retrieves a string value from JSON with a default fallback
func (p *Processor) GetStringWithDefault(jsonStr, path, defaultValue string, opts ...*ProcessorOptions) string {
	value, err := p.GetString(jsonStr, path, opts...)
	if err != nil {
		return defaultValue
	}
	return value
}

// GetIntWithDefault retrieves an int value from JSON with a default fallback
func (p *Processor) GetIntWithDefault(jsonStr, path string, defaultValue int, opts ...*ProcessorOptions) int {
	value, err := p.GetInt(jsonStr, path, opts...)
	if err != nil {
		return defaultValue
	}
	return value
}

// GetFloat64WithDefault retrieves a float64 value from JSON with a default fallback
func (p *Processor) GetFloat64WithDefault(jsonStr, path string, defaultValue float64, opts ...*ProcessorOptions) float64 {
	value, err := p.GetFloat64(jsonStr, path, opts...)
	if err != nil {
		return defaultValue
	}
	return value
}

// GetBoolWithDefault retrieves a bool value from JSON with a default fallback
func (p *Processor) GetBoolWithDefault(jsonStr, path string, defaultValue bool, opts ...*ProcessorOptions) bool {
	value, err := p.GetBool(jsonStr, path, opts...)
	if err != nil {
		return defaultValue
	}
	return value
}

// GetArrayWithDefault retrieves an array value from JSON with a default fallback
func (p *Processor) GetArrayWithDefault(jsonStr, path string, defaultValue []any, opts ...*ProcessorOptions) []any {
	value, err := p.GetArray(jsonStr, path, opts...)
	if err != nil {
		return defaultValue
	}
	return value
}

// GetObjectWithDefault retrieves an object value from JSON with a default fallback
func (p *Processor) GetObjectWithDefault(jsonStr, path string, defaultValue map[string]any, opts ...*ProcessorOptions) map[string]any {
	value, err := p.GetObject(jsonStr, path, opts...)
	if err != nil {
		return defaultValue
	}
	return value
}

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

	// Sequential processing
	results := make(map[string]any, len(paths))
	for _, path := range paths {
		if err := p.validatePath(path); err != nil {
			return nil, err
		}

		// Use unified processor for all paths
		unifiedProcessor := NewRecursiveProcessor(p)
		result, err := unifiedProcessor.ProcessRecursively(data, path, OpGet, nil)

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
				// Use unified recursive processor for consistency
				var result any
				var err error

				unifiedProcessor := NewRecursiveProcessor(p)
				result, err = unifiedProcessor.ProcessRecursively(data, currentPath, OpGet, nil)

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
