package json

import (
	"sync/atomic"

	"github.com/cybergodev/json/internal"
)

// ClearCache clears all cached data
func (p *Processor) ClearCache() {
	if p.cache != nil {
		p.cache.Clear()
	}
}

// hashStringToUint64 generates a fast 64-bit hash using FNV-1a.
// Delegates to internal package for consistent implementation.
// PERFORMANCE: For large strings (> 4KB), uses sampling to avoid full scan.
func hashStringToUint64(s string) uint64 {
	if len(s) > largeStringHashThreshold {
		return internal.HashStringFNV1aSampled(s)
	}
	return internal.HashStringFNV1a(s)
}

// createCacheKey creates a cache key with optimized efficiency
// Uses direct hash values instead of hex strings for better performance
func (p *Processor) createCacheKey(operation, jsonStr, path string, options *Config) string {
	jsonHash := hashStringToUint64(jsonStr)
	return p.createCacheKeyWithHash(operation, jsonHash, path, options)
}

// createCacheKeyWithHash creates a cache key using a pre-computed hash
// PERFORMANCE: Allows hash reuse across multiple cache key creations.
// Uses pointer identity check for default config to avoid 40+ field comparisons.
func (p *Processor) createCacheKeyWithHash(operation string, jsonHash uint64, path string, options *Config) string {
	// Determine if options are default — pointer identity is the fastest check
	isDefault := options == nil || options == cachedDefaultConfigPtr

	// Use a fixed-size array buffer for small keys to avoid allocations
	// Most cache keys are < 128 bytes
	var buf [128]byte

	// Try to use stack-allocated buffer
	estimatedLen := len(operation) + 1 + 16 + 1 + len(path) + 16 // op:hash16:path:opts
	if estimatedLen < len(buf) && isDefault {
		// Fast path: use stack buffer (covers >99% of real-world cases)
		n := copy(buf[:], operation)
		buf[n] = ':'
		n++
		n += formatUint64Hex(buf[n:], jsonHash)
		buf[n] = ':'
		n++
		n += copy(buf[n:], path)
		return string(buf[:n])
	}

	// Slow path: use string builder for larger keys or non-default options
	sb := p.getStringBuilder()
	defer p.putStringBuilder(sb)

	sb.Grow(estimatedLen + 32)
	sb.WriteString(operation)
	sb.WriteByte(':')
	sb.WriteString(formatUint64HexString(jsonHash))
	sb.WriteByte(':')
	sb.WriteString(path)

	// Include all options that affect output using config hash.
	// Ensures different configs never share cached results.
	// PERFORMANCE: Skip hash computation for default config (common case)
	if !isDefault {
		optHash := hashConfig(*options)
		sb.WriteByte(':')
		sb.WriteString(formatUint64HexString(optHash))
	}

	return sb.String()
}

// formatUint64Hex formats a uint64 as hex without allocation
func formatUint64Hex(buf []byte, v uint64) int {
	const hexChars = "0123456789abcdef"
	for i := 15; i >= 0; i-- {
		buf[i] = hexChars[v&0xF]
		v >>= 4
	}
	return 16
}

// formatUint64HexString formats a uint64 as a hex string
func formatUint64HexString(v uint64) string {
	var buf [16]byte
	formatUint64Hex(buf[:], v)
	return string(buf[:])
}

// createSimpleCacheKey creates a simple "prefix:data" format cache key
// Uses stack-allocated buffer for small keys to avoid heap allocation
func createSimpleCacheKey(prefix, data string) string {
	totalLen := len(prefix) + 1 + len(data) // prefix + ":" + data

	// Use stack-allocated buffer for small keys (up to 256 bytes)
	const maxStackKeySize = 256
	if totalLen <= maxStackKeySize {
		var buf [maxStackKeySize]byte
		n := copy(buf[:], prefix)
		buf[n] = ':'
		n++
		n += copy(buf[n:], data)
		return string(buf[:n])
	}

	// Fall back to heap allocation for large keys
	return prefix + ":" + data
}

// getCachedPathSegments gets parsed path segments using unified cache
// PERFORMANCE: Creates cache key once and reuses for both lookup and storage
// PERFORMANCE: Returns cached segments directly (immutable after creation)
func (p *Processor) getCachedPathSegments(path string) ([]internal.PathSegment, error) {
	// Use unified cache manager
	if p.config.EnableCache {
		// PERFORMANCE: Create cache key once for both lookup and storage
		cacheKey := createSimpleCacheKey("path", path)
		if cached, ok := p.cache.Get(cacheKey); ok {
			if segments, ok := cached.([]internal.PathSegment); ok {
				// PERFORMANCE: Return cached segments directly - they are immutable after creation
				return segments, nil
			}
		}

		// Parse path
		segments, err := internal.ParsePath(path)
		if err != nil {
			return nil, err
		}

		// Cache the result using unified cache - reuse the cache key
		if atomic.LoadInt32(&p.state) == processorStateActive {
			cached := make([]internal.PathSegment, len(segments))
			copy(cached, segments)
			p.cache.Set(cacheKey, cached)
		}

		return segments, nil
	}

	// Parse path without caching
	segments, err := internal.ParsePath(path)
	if err != nil {
		return nil, err
	}

	return segments, nil
}

// getCachedResult retrieves a cached result if available
func (p *Processor) getCachedResult(key string) (any, bool) {
	if !p.config.EnableCache {
		return nil, false
	}
	return p.cache.Get(key)
}

// setCachedResult stores a result in cache with security validation
func (p *Processor) setCachedResult(key string, result any, options ...*Config) {
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

// setCachedResultInternal stores a result in cache without sensitive data check
// PERFORMANCE: For trusted internal results (parsed JSON, navigation results) where
// security validation already happened at input. Skips expensive sensitive data scanning.
func (p *Processor) setCachedResultInternal(key string, result any) {
	if !p.config.EnableCache {
		return
	}

	// Validate cache key to prevent injection
	if !p.isValidCacheKey(key) {
		return
	}

	p.cache.Set(key, result)
}

// invalidateCachedResult removes a cache entry by key.
// Used when a cached value has a type mismatch (corrupted entry).
func (p *Processor) invalidateCachedResult(key string) {
	if !p.config.EnableCache {
		return
	}
	p.cache.Delete(key)
}

// containsSensitiveData checks if the result contains sensitive information
// SECURITY: Delegates to securityValidator for consistent detection logic
func (p *Processor) containsSensitiveData(result any) bool {
	return p.securityValidator.ContainsSensitiveData(result)
}

// isValidCacheKey validates cache key format
// Delegates to internal package for consistent implementation
func (p *Processor) isValidCacheKey(key string) bool {
	return internal.IsValidCacheKey(key)
}
