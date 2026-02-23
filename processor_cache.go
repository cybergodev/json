package json

import (
	"fmt"
	"hash/fnv"
	"strconv"
	"strings"
	"sync/atomic"

	"github.com/cybergodev/json/internal"
)

// ClearCache clears all cached data
func (p *Processor) ClearCache() {
	if p.cache != nil {
		p.cache.Clear()
	}
}

// WarmupCache pre-loads commonly used paths into cache to improve first-access performance
func (p *Processor) WarmupCache(jsonStr string, paths []string, opts ...*ProcessorOptions) (*WarmupResult, error) {
	if err := p.checkClosed(); err != nil {
		return nil, err
	}

	if !p.config.EnableCache {
		return nil, &JsonsError{
			Op:      "warmup_cache",
			Message: "cache is disabled, cannot warmup cache",
			Err:     ErrCacheDisabled,
		}
	}

	if len(paths) == 0 {
		return &WarmupResult{
			TotalPaths:  0,
			Successful:  0,
			Failed:      0,
			SuccessRate: 100.0,
			FailedPaths: nil,
		}, nil // Nothing to warmup
	}

	// Validate JSON input
	if err := p.validateInput(jsonStr); err != nil {
		return nil, &JsonsError{
			Op:      "warmup_cache",
			Message: "invalid JSON input for cache warmup",
			Err:     err,
		}
	}

	// Prepare options
	options, err := p.prepareOptions(opts...)
	if err != nil {
		return nil, &JsonsError{
			Op:      "warmup_cache",
			Message: "invalid options for cache warmup",
			Err:     err,
		}
	}

	// Track warmup statistics
	successCount := 0
	errorCount := 0
	var lastError error
	var failedPaths []string

	// Preload each path into cache
	for _, path := range paths {
		// Validate path
		if err := p.validatePath(path); err != nil {
			errorCount++
			failedPaths = append(failedPaths, path)
			lastError = &JsonsError{
				Op:      "warmup_cache",
				Path:    path,
				Message: fmt.Sprintf("invalid path '%s' for cache warmup: %v", path, err),
				Err:     err,
			}
			continue
		}

		// Try to get the value (this will cache it if successful)
		_, err := p.Get(jsonStr, path, options)
		if err != nil {
			errorCount++
			failedPaths = append(failedPaths, path)
			lastError = &JsonsError{
				Op:      "warmup_cache",
				Path:    path,
				Message: fmt.Sprintf("failed to warmup path '%s': %v", path, err),
				Err:     err,
			}
		} else {
			successCount++
		}
	}

	// Create warmup result
	successRate := 100.0
	if len(paths) > 0 {
		successRate = float64(successCount) / float64(len(paths)) * 100
	}

	result := &WarmupResult{
		TotalPaths:  len(paths),
		Successful:  successCount,
		Failed:      errorCount,
		SuccessRate: successRate,
		FailedPaths: failedPaths,
	}

	// Return error if all paths failed
	if successCount == 0 && errorCount > 0 {
		return result, &JsonsError{
			Op:      "warmup_cache",
			Message: fmt.Sprintf("cache warmup failed for all %d paths, last error: %v", len(paths), lastError),
			Err:     lastError,
		}
	}

	return result, nil
}

// warmupCacheWithSampleData pre-loads paths using sample JSON data for better cache preparation
func (p *Processor) warmupCacheWithSampleData(sampleData map[string]string, opts ...*ProcessorOptions) (*WarmupResult, error) {
	if err := p.checkClosed(); err != nil {
		return nil, err
	}

	if !p.config.EnableCache {
		return nil, &JsonsError{
			Op:      "warmup_cache_sample",
			Message: "cache is disabled, cannot warmup cache",
			Err:     ErrCacheDisabled,
		}
	}

	if len(sampleData) == 0 {
		return &WarmupResult{
			TotalPaths:  0,
			Successful:  0,
			Failed:      0,
			SuccessRate: 100.0,
			FailedPaths: nil,
		}, nil // Nothing to warmup
	}

	totalPaths := 0
	successCount := 0
	errorCount := 0
	var lastError error
	var allFailedPaths []string

	// Process each JSON sample with its associated paths
	for jsonStr, pathsStr := range sampleData {
		// Parse paths (comma-separated)
		paths := strings.Split(pathsStr, ",")
		for i, path := range paths {
			paths[i] = strings.TrimSpace(path)
		}

		totalPaths += len(paths)

		// Warmup cache for this JSON with these paths
		result, err := p.WarmupCache(jsonStr, paths, opts...)
		if err != nil {
			errorCount += len(paths)
			lastError = err
			// Add all paths as failed if the entire warmup failed
			allFailedPaths = append(allFailedPaths, paths...)
		} else if result != nil {
			successCount += result.Successful
			errorCount += result.Failed
			allFailedPaths = append(allFailedPaths, result.FailedPaths...)
		}
	}

	// Create warmup result
	successRate := 100.0
	if totalPaths > 0 {
		successRate = float64(successCount) / float64(totalPaths) * 100
	}

	result := &WarmupResult{
		TotalPaths:  totalPaths,
		Successful:  successCount,
		Failed:      errorCount,
		SuccessRate: successRate,
		FailedPaths: allFailedPaths,
	}

	// Return error if all operations failed
	if successCount == 0 && errorCount > 0 {
		return result, &JsonsError{
			Op:      "warmup_cache_sample",
			Message: fmt.Sprintf("sample data cache warmup failed for all %d paths, last error: %v", totalPaths, lastError),
			Err:     lastError,
		}
	}

	return result, nil
}

// hashString generates a fast hash for cache keys using FNV-1a
func hashString(s string) string {
	h := fnv.New64a()
	h.Write([]byte(s))
	return fmt.Sprintf("%x", h.Sum64())
}

// createCacheKey creates a cache key with optimized efficiency using resource pools
func (p *Processor) createCacheKey(operation, jsonStr, path string, options *ProcessorOptions) string {
	// Get string builder from pool for memory efficiency
	sb := p.getStringBuilder()
	defer p.putStringBuilder(sb)

	// Pre-allocate capacity more accurately to reduce reallocations
	estimatedSize := len(operation) + len(path) + 32 // Reduced overhead estimation
	if len(jsonStr) < 1024 {
		estimatedSize += 32 // Hash size for small JSON
	} else {
		estimatedSize += 64 // Hash size for larger JSON
	}
	sb.Grow(estimatedSize)

	sb.WriteString(operation)
	sb.WriteByte(':')

	// Optimize JSON normalization for better cache hit rates
	normalizedJSON := p.normalizeJSONForCache(jsonStr)
	sb.WriteString(hashString(normalizedJSON))
	sb.WriteByte(':')
	sb.WriteString(path)

	// Include relevant options in the key using more efficient approach
	if options != nil {
		// Use single conditional writes to reduce function call overhead
		if options.StrictMode {
			sb.WriteString(":s")
		}
		if options.AllowComments {
			sb.WriteString(":c")
		}
		if options.PreserveNumbers {
			sb.WriteString(":p")
		}
		if options.MaxDepth > 0 {
			sb.WriteString(":d")
			sb.WriteString(strconv.Itoa(options.MaxDepth))
		}
	}

	return sb.String()
}

// normalizeJSONForCache normalizes JSON string for better cache hit rates with minimal overhead
func (p *Processor) normalizeJSONForCache(jsonStr string) string {
	// For very small JSON strings, use lightweight normalization
	if len(jsonStr) < 256 {
		// Simple whitespace normalization without full parsing for small JSON
		return p.lightweightJSONNormalize(jsonStr)
	}

	// For medium JSON strings, use full normalization
	if len(jsonStr) < 1024 {
		// Parse and re-encode to normalize formatting
		var data any
		if err := p.Parse(jsonStr, &data); err != nil {
			// If parsing fails, return lightweight normalized version
			return p.lightweightJSONNormalize(jsonStr)
		}

		// Re-encode with compact format to normalize
		config := DefaultEncodeConfig()
		config.Pretty = false
		config.SortKeys = true // Sort keys for consistent ordering
		if normalized, err := p.EncodeWithConfig(data, config); err == nil {
			return normalized
		}
	}

	// For large JSON strings, return original to avoid performance penalty
	return jsonStr
}

// lightweightJSONNormalize performs lightweight JSON normalization without parsing
func (p *Processor) lightweightJSONNormalize(jsonStr string) string {
	// Use string builder from pool
	sb := p.getStringBuilder()
	defer p.putStringBuilder(sb)

	// Pre-allocate capacity to reduce reallocations
	sb.Grow(len(jsonStr))

	// Simple whitespace and formatting normalization using byte operations for performance
	inString := false
	escaped := false
	bytes := []byte(jsonStr)

	for i := 0; i < len(bytes); i++ {
		b := bytes[i]

		if escaped {
			sb.WriteByte(b)
			escaped = false
			continue
		}

		if b == '\\' {
			sb.WriteByte(b)
			escaped = true
			continue
		}

		if b == '"' {
			inString = !inString
			sb.WriteByte(b)
			continue
		}

		if inString {
			sb.WriteByte(b)
			continue
		}

		// Outside string - normalize whitespace
		switch b {
		case ' ', '\t', '\n', '\r':
			// Skip whitespace outside strings
			continue
		case ':', ',':
			sb.WriteByte(b)
		default:
			sb.WriteByte(b)
		}
	}

	return sb.String()
}

// getCachedPathSegments gets parsed path segments using unified cache
func (p *Processor) getCachedPathSegments(path string) ([]internal.PathSegment, error) {
	// Use unified cache manager
	if p.config.EnableCache {
		cacheKey := "path:" + path
		if cached, ok := p.cache.Get(cacheKey); ok {
			if segments, ok := cached.([]internal.PathSegment); ok {
				// Make a copy to avoid race conditions
				result := make([]internal.PathSegment, len(segments))
				copy(result, segments)
				return result, nil
			}
		}
	}

	// Parse path
	segments, err := internal.ParsePath(path)
	if err != nil {
		return nil, err
	}

	// Cache the result using unified cache
	if p.config.EnableCache && atomic.LoadInt32(&p.state) == 0 {
		cacheKey := "path:" + path
		cached := make([]internal.PathSegment, len(segments))
		copy(cached, segments)
		p.cache.Set(cacheKey, cached)
	}

	return segments, nil
}

// getCachedParsedJSON gets parsed JSON using unified cache
func (p *Processor) getCachedParsedJSON(jsonStr string) (any, error) {
	// Only cache small JSON strings
	if len(jsonStr) < 2048 && p.config.EnableCache {
		cacheKey := "json:" + jsonStr
		if cached, ok := p.cache.Get(cacheKey); ok {
			return cached, nil
		}
	}

	// Parse JSON
	var data any
	err := p.Parse(jsonStr, &data)
	if err != nil {
		return nil, err
	}

	// Cache small JSON strings using unified cache
	if len(jsonStr) < 2048 && p.config.EnableCache && atomic.LoadInt32(&p.state) == 0 {
		cacheKey := "json:" + jsonStr
		p.cache.Set(cacheKey, data)
	}

	return data, nil
}

// getCachedResult retrieves a cached result if available
func (p *Processor) getCachedResult(key string) (any, bool) {
	if !p.config.EnableCache {
		return nil, false
	}
	return p.cache.Get(key)
}

// setCachedResult stores a result in cache with security validation
func (p *Processor) setCachedResult(key string, result any, options ...*ProcessorOptions) {
	if !p.config.EnableCache {
		return
	}

	// Check if caching is enabled for this operation
	if len(options) > 0 && options[0] != nil && !options[0].CacheResults {
		return
	}

	// Security validation: don't cache potentially sensitive data
	if p.containsSensitiveData(result) {
		return
	}

	// Validate cache key to prevent injection
	if !p.isValidCacheKey(key) {
		return
	}

	p.cache.Set(key, result)
}

// containsSensitiveData checks if the result contains sensitive information
func (p *Processor) containsSensitiveData(result any) bool {
	if result == nil {
		return false
	}

	// Convert to string for pattern matching
	resultStr := fmt.Sprintf("%v", result)
	if len(resultStr) > 1000 { // Don't cache large results
		return true
	}

	// Check for sensitive patterns
	sensitivePatterns := []string{"password", "token", "secret", "key", "auth", "credential"}
	lowerResult := strings.ToLower(resultStr)
	for _, pattern := range sensitivePatterns {
		if strings.Contains(lowerResult, pattern) {
			return true
		}
	}
	return false
}

// isValidCacheKey validates cache key format
func (p *Processor) isValidCacheKey(key string) bool {
	if len(key) > 500 { // Prevent excessively long keys
		return false
	}
	// Check for null bytes and control characters
	for _, b := range []byte(key) {
		if b < 32 || b == 127 { // Control characters
			return false
		}
	}
	return true
}

// parseSliceComponents parses slice syntax using unified array utilities
func (p *Processor) parseSliceComponents(slicePart string) (start, end, step *int, err error) {
	return internal.ParseSliceComponents(slicePart)
}
