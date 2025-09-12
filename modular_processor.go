package json

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// modularProcessor implements the ModularProcessor interface using composition
type modularProcessor struct {
	// Core modules
	pathParser PathParser
	navigator  Navigator
	arrayOps   ArrayOperations
	extractOps ExtractionOperations
	setOps     SetOperations
	deleteOps  DeleteOperations
	utils      ProcessorUtils

	// Configuration and state
	config  *ProcessorConfig
	cache   ProcessorCache
	metrics MetricsCollector
	limiter RateLimiter

	// Synchronization
	mu     sync.RWMutex
	closed bool
}

// NewModularProcessor creates a new modular processor instance
func NewModularProcessor(config *ProcessorConfig) ModularProcessor {
	if config == nil {
		config = DefaultProcessorConfig()
	}

	utils := NewProcessorUtils()
	pathParser := NewPathParser()
	navigator := NewNavigator(pathParser, utils)
	arrayOps := NewArrayOperations(utils)
	extractionOps := NewExtractionOperations(utils)
	setOps := NewSetOperations(utils, pathParser, navigator, arrayOps)
	deleteOps := NewDeleteOperations(utils, pathParser, navigator, arrayOps)

	return &modularProcessor{
		pathParser: pathParser,
		navigator:  navigator,
		arrayOps:   arrayOps,
		extractOps: extractionOps,
		setOps:     setOps,
		deleteOps:  deleteOps,
		utils:      utils,
		config:     config,
		closed:     false,
	}
}

// Get retrieves a value from JSON using a path expression
func (mp *modularProcessor) Get(jsonStr, path string, opts ...*ProcessorOptions) (any, error) {
	if err := mp.checkClosed(); err != nil {
		return nil, err
	}

	// Parse options
	options := mp.prepareOptions(opts...)

	// Start timing
	startTime := time.Now()
	defer func() {
		if mp.metrics != nil {
			mp.metrics.RecordOperation(time.Since(startTime), true, 0)
		}
	}()

	// Rate limiting check
	if mp.limiter != nil && !mp.limiter.Allow() {
		return nil, ErrRateLimitNew
	}

	// Validate inputs
	if err := mp.validateInput(jsonStr); err != nil {
		return nil, err
	}

	if err := mp.pathParser.ValidatePath(path); err != nil {
		return nil, &ProcessorError{
			Type:      ErrTypeValidation,
			Operation: "get",
			Path:      path,
			Message:   "invalid path",
			Cause:     err,
		}
	}

	// Check cache first
	if mp.cache != nil {
		cacheKey := mp.createCacheKey("get", jsonStr, path, options)
		if cached, ok := mp.cache.Get(cacheKey); ok {
			if mp.metrics != nil {
				mp.metrics.RecordCacheHit()
			}
			return cached, nil
		}
		if mp.metrics != nil {
			mp.metrics.RecordCacheMiss()
		}
	}

	// Parse JSON
	var data any
	if err := json.Unmarshal([]byte(jsonStr), &data); err != nil {
		return nil, &ProcessorError{
			Type:      ErrTypeValidation,
			Operation: "get",
			Path:      path,
			Message:   "invalid JSON",
			Cause:     err,
		}
	}

	// Parse path into segments
	segments, err := mp.pathParser.ParsePath(path)
	if err != nil {
		return nil, &ProcessorError{
			Type:      ErrTypeValidation,
			Operation: "get",
			Path:      path,
			Message:   "failed to parse path",
			Cause:     err,
		}
	}

	// Navigate to the target value
	result, err := mp.navigator.NavigateToPath(data, segments)
	if err != nil {
		return nil, &ProcessorError{
			Type:      ErrTypeNavigation,
			Operation: "get",
			Path:      path,
			Message:   "navigation failed",
			Cause:     err,
		}
	}

	// Cache the result
	if mp.cache != nil {
		cacheKey := mp.createCacheKey("get", jsonStr, path, options)
		mp.cache.Set(cacheKey, result, mp.config.Timeout)
	}

	return result, nil
}

// Set sets a value in JSON at the specified path
func (mp *modularProcessor) Set(jsonStr, path string, value any, opts ...*ProcessorOptions) (string, error) {
	if err := mp.checkClosed(); err != nil {
		return "", err
	}

	// Parse options
	options := mp.prepareOptions(opts...)

	// Validate inputs
	if err := mp.validateInput(jsonStr); err != nil {
		return "", err
	}

	if err := mp.pathParser.ValidatePath(path); err != nil {
		return "", &ProcessorError{
			Type:      ErrTypeValidation,
			Operation: "set",
			Path:      path,
			Message:   "invalid path",
			Cause:     err,
		}
	}

	// Parse JSON
	var data any
	if err := json.Unmarshal([]byte(jsonStr), &data); err != nil {
		return "", &ProcessorError{
			Type:      ErrTypeValidation,
			Operation: "set",
			Path:      path,
			Message:   "invalid JSON",
			Cause:     err,
		}
	}

	// Parse path into segments
	segments, err := mp.pathParser.ParsePath(path)
	if err != nil {
		return "", &ProcessorError{
			Type:      ErrTypeValidation,
			Operation: "set",
			Path:      path,
			Message:   "failed to parse path",
			Cause:     err,
		}
	}

	// Set the value using the appropriate operations module
	createPaths := options.CreatePaths
	if mp.setOps != nil {
		err = mp.setOps.SetValueWithSegments(data, segments, value, createPaths)
	} else {
		// Fallback to basic implementation
		err = mp.setValueBasic(data, segments, value, createPaths)
	}

	if err != nil {
		return "", &ProcessorError{
			Type:      ErrTypeNavigation,
			Operation: "set",
			Path:      path,
			Message:   "failed to set value",
			Cause:     err,
		}
	}

	// Marshal back to JSON
	resultBytes, err := json.Marshal(data)
	if err != nil {
		return "", &ProcessorError{
			Type:      ErrTypeConversion,
			Operation: "set",
			Path:      path,
			Message:   "failed to marshal result",
			Cause:     err,
		}
	}

	return string(resultBytes), nil
}

// Delete deletes a value from JSON at the specified path
func (mp *modularProcessor) Delete(jsonStr, path string, opts ...*ProcessorOptions) (string, error) {
	if err := mp.checkClosed(); err != nil {
		return "", err
	}

	// Validate inputs
	if err := mp.validateInput(jsonStr); err != nil {
		return jsonStr, err
	}

	if err := mp.pathParser.ValidatePath(path); err != nil {
		return jsonStr, &ProcessorError{
			Type:      ErrTypeValidation,
			Operation: "delete",
			Path:      path,
			Message:   "invalid path",
			Cause:     err,
		}
	}

	// Parse JSON
	var data any
	if err := json.Unmarshal([]byte(jsonStr), &data); err != nil {
		return jsonStr, &ProcessorError{
			Type:      ErrTypeValidation,
			Operation: "delete",
			Path:      path,
			Message:   "invalid JSON",
			Cause:     err,
		}
	}

	// Delete the value using the delete operations module
	if mp.deleteOps != nil {
		err := mp.deleteOps.DeleteValue(data, path)
		if err != nil {
			// Return original JSON instead of empty string when deletion fails
			return jsonStr, &ProcessorError{
				Type:      ErrTypeNavigation,
				Operation: "delete",
				Path:      path,
				Message:   "failed to delete value",
				Cause:     err,
			}
		}
	} else {
		// Return original JSON instead of empty string when delete operations not implemented
		return jsonStr, &ProcessorError{
			Type:      ErrTypeNavigation,
			Operation: "delete",
			Path:      path,
			Message:   "delete operations not implemented",
		}
	}

	// Marshal back to JSON
	resultBytes, err := json.Marshal(data)
	if err != nil {
		// Return original JSON instead of empty string when marshaling fails
		return jsonStr, &ProcessorError{
			Type:      ErrTypeConversion,
			Operation: "delete",
			Path:      path,
			Message:   "failed to marshal result",
			Cause:     err,
		}
	}

	return string(resultBytes), nil
}

// GetMultiple retrieves multiple values from JSON using multiple paths
func (mp *modularProcessor) GetMultiple(jsonStr string, paths []string, opts ...*ProcessorOptions) (map[string]any, error) {
	results := make(map[string]any)

	for _, path := range paths {
		result, err := mp.Get(jsonStr, path, opts...)
		if err != nil {
			results[path] = nil
		} else {
			results[path] = result
		}
	}

	return results, nil
}

// BatchProcess processes multiple operations in batch
func (mp *modularProcessor) BatchProcess(operations []BatchOperation, opts ...*ProcessorOptions) ([]BatchResult, error) {
	results := make([]BatchResult, len(operations))

	for i, op := range operations {
		result := BatchResult{ID: op.ID}

		switch op.Type {
		case "get":
			result.Result, result.Error = mp.Get(op.JSONStr, op.Path, opts...)
		case "set":
			result.Result, result.Error = mp.Set(op.JSONStr, op.Path, op.Value, opts...)
		case "delete":
			result.Result, result.Error = mp.Delete(op.JSONStr, op.Path, opts...)
		case "validate":
			result.Result, result.Error = mp.Valid(op.JSONStr, opts...)
		default:
			result.Error = fmt.Errorf("unknown operation type: %s", op.Type)
		}

		results[i] = result
	}

	return results, nil
}

// Valid validates JSON format
func (mp *modularProcessor) Valid(jsonStr string, opts ...*ProcessorOptions) (bool, error) {
	var data any
	err := json.Unmarshal([]byte(jsonStr), &data)
	return err == nil, err
}

// SetConfig updates the processor configuration
func (mp *modularProcessor) SetConfig(config *ProcessorConfig) {
	mp.mu.Lock()
	defer mp.mu.Unlock()
	mp.config = config
}

// GetConfig returns the current processor configuration
func (mp *modularProcessor) GetConfig() *ProcessorConfig {
	mp.mu.RLock()
	defer mp.mu.RUnlock()
	return mp.config
}

// Close closes the processor and releases resources
func (mp *modularProcessor) Close() error {
	mp.mu.Lock()
	defer mp.mu.Unlock()

	if mp.closed {
		return nil
	}

	mp.closed = true

	// Clear cache if present
	if mp.cache != nil {
		mp.cache.Clear()
	}

	return nil
}

// IsClosed returns whether the processor is closed
func (mp *modularProcessor) IsClosed() bool {
	mp.mu.RLock()
	defer mp.mu.RUnlock()
	return mp.closed
}

// Helper methods

// checkClosed checks if the processor is closed
func (mp *modularProcessor) checkClosed() error {
	mp.mu.RLock()
	defer mp.mu.RUnlock()
	if mp.closed {
		return &ProcessorError{
			Type:    ErrTypeValidation,
			Message: "processor is closed",
		}
	}
	return nil
}

// prepareOptions prepares and validates processor options
func (mp *modularProcessor) prepareOptions(opts ...*ProcessorOptions) *ProcessorOptions {
	if len(opts) == 0 {
		return &ProcessorOptions{}
	}
	return opts[0]
}

// validateInput validates JSON input string
func (mp *modularProcessor) validateInput(jsonStr string) error {
	if jsonStr == "" {
		return &ProcessorError{
			Type:    ErrTypeValidation,
			Message: "empty JSON string",
		}
	}
	return nil
}

// createCacheKey creates a cache key for the operation
func (mp *modularProcessor) createCacheKey(operation, jsonStr, path string, options *ProcessorOptions) CacheKey {
	return CacheKey{
		Operation: operation,
		JSONStr:   jsonStr,
		Path:      path,
		Options:   fmt.Sprintf("%+v", options),
	}
}

// setValueBasic provides a basic implementation for setting values
func (mp *modularProcessor) setValueBasic(data any, segments []PathSegmentInfo, value any, createPaths bool) error {
	if len(segments) == 0 {
		return fmt.Errorf("no segments to process")
	}

	// Navigate to parent segments
	current := data
	for i, segment := range segments[:len(segments)-1] {
		result, err := mp.navigator.NavigateToSegment(current, segment)
		if err != nil {
			return fmt.Errorf("failed to navigate to segment %d: %w", i, err)
		}

		if !result.Exists {
			if createPaths {
				// Create missing path segment
				newContainer, err := mp.createContainerForSegment(segments, i+1)
				if err != nil {
					return fmt.Errorf("failed to create container for segment %d: %w", i, err)
				}

				// Set the new container in the current object
				if err := mp.setValueInContainer(current, segment, newContainer); err != nil {
					return fmt.Errorf("failed to set container in segment %d: %w", i, err)
				}
				current = newContainer
			} else {
				return fmt.Errorf("path not found at segment %d", i)
			}
		} else {
			current = result.Value
		}
	}

	// Set the final value
	finalSegment := segments[len(segments)-1]
	return mp.setValueInContainer(current, finalSegment, value)
}

// createContainerForSegment creates an appropriate container for the next segment
func (mp *modularProcessor) createContainerForSegment(segments []PathSegmentInfo, nextIndex int) (any, error) {
	if nextIndex >= len(segments) {
		return make(map[string]any), nil
	}

	nextSegment := segments[nextIndex]
	switch nextSegment.Type {
	case "array", "slice":
		return make([]any, 0), nil
	default:
		return make(map[string]any), nil
	}
}

// setValueInContainer sets a value in a container based on segment type
func (mp *modularProcessor) setValueInContainer(container any, segment PathSegmentInfo, value any) error {
	switch segment.Type {
	case "property":
		return mp.setPropertyValue(container, segment.Key, value)
	case "array":
		return mp.setArrayValue(container, segment.Index, value)
	default:
		return fmt.Errorf("unsupported segment type for setting: %s", segment.Type)
	}
}

// setPropertyValue sets a property value in an object
func (mp *modularProcessor) setPropertyValue(container any, key string, value any) error {
	switch obj := container.(type) {
	case map[string]any:
		obj[key] = value
		return nil
	case map[any]any:
		obj[key] = value
		return nil
	default:
		return fmt.Errorf("cannot set property on type %T", container)
	}
}

// setArrayValue sets a value at an array index
func (mp *modularProcessor) setArrayValue(container any, index int, value any) error {
	arr, ok := container.([]any)
	if !ok {
		return fmt.Errorf("cannot set array value on type %T", container)
	}

	// Handle negative indices
	normalizedIndex := mp.arrayOps.HandleNegativeIndex(index, len(arr))

	// Check if we need to extend the array
	if normalizedIndex >= len(arr) {
		// Extend array to accommodate the index
		extended := mp.arrayOps.ExtendArray(arr, normalizedIndex+1)
		// Copy back to original slice (this is a limitation of this approach)
		copy(arr, extended)
		if len(extended) > len(arr) {
			return fmt.Errorf("cannot extend array in place")
		}
	}

	return mp.arrayOps.SetArrayElement(arr, normalizedIndex, value)
}
