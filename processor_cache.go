package json

import (
	"github.com/cybergodev/json/internal"
)

// ClearCache clears all cached data
func (p *Processor) ClearCache() {
	if p.cache != nil {
		p.cache.Clear()
	}
}

// invalidateJSONCache removes all cached results associated with a JSON string.
// Called after mutation operations (Set, Delete) to prevent stale cache hits.
func (p *Processor) invalidateJSONCache(jsonStr string) {
	if !p.config.EnableCache || p.cache == nil {
		return
	}

	// PERFORMANCE: Set/Delete call this on every mutation. When the cache holds
	// no entries there is nothing to invalidate, so skip the FNV hash + hex
	// formatting + per-shard scan entirely. Profiling (P-001) showed that with
	// the default config (cache enabled) this path dominated Set/Delete CPU even
	// though Set never populates the cache — the scan ran write-locked across
	// every shard for no work.
	if p.cache.EntryCount() == 0 {
		return
	}

	jsonHash := hashStringToUint64(jsonStr)
	hashPrefix := formatUint64HexString(jsonHash)

	// Delete all entries containing this JSON hash prefix.
	// Catches parse, get, set, delete, iterate, validate, and any future operation keys.
	p.cache.DeleteByPrefix(hashPrefix)
}

// hashStringToUint64 generates a 64-bit FNV-1a hash used as cache identity for
// result/parse caches and cache invalidation.
//
// CORRECTNESS: Always hashes the FULL string. This function is the identity for
// caches that store computed results (Get/parse/encode) keyed by JSON content —
// a collision returns a WRONG cached result. A sampled variant (examining only
// first/middle/last bytes of large inputs) was removed for exactly this reason:
// two equal-length documents that differ outside the sample windows collide.
// The full scan is negligible compared to the JSON parse it guards, so
// correctness wins over the micro-optimization.
func hashStringToUint64(s string) uint64 {
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
	// Determine if options are default. Pointer-identity covers the overwhelmingly
	// common case (prepareOptions hands out the shared default singleton), avoiding
	// a 40+ field configFieldsEqual scan on every cache-key build. The fallback
	// handles a caller-supplied Config that happens to equal the default.
	isDefault := options == nil || options == &defaultConfigSingleton || configFieldsEqual(*options, cachedDefaultConfigValue)

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

// getCachedPathSegments gets parsed path segments for the recursive processor.
//
// PERFORMANCE: delegates to internal.ParsePath, whose process-wide sync.Map
// cache serves every caller (its own fast paths, iterators, path validation)
// with lock-free reads. The former processor-level "path:" cache duplicated
// that data a second time per processor AND returned a defensive copy on
// every hit — one allocation per navigation for protection against a
// mutation that no consumer performs: navigation (recursive.go) treats
// segments as read-only, the same contract all other ParsePath callers
// already rely on.
func (p *Processor) getCachedPathSegments(path string) ([]internal.PathSegment, error) {
	// D-002 (M33): honor Config.CustomPathParser. Custom parsers bypass the
	// global segment cache — it is keyed by path string alone and cannot
	// distinguish parser implementations.
	if p.config.CustomPathParser != nil {
		return parsePathGuarded(p.config.CustomPathParser, path)
	}
	return internal.ParsePath(path)
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
