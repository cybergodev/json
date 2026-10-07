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

// invalidateJSONCacheHashed removes all cached results associated with a
// JSON document, preventing stale cache hits after mutations (Set, Delete,
// SetMultiple). Callers that already hold the document's FNV-1a hash pass it
// directly (P-003): Set/Delete receive it from prepareOperation and
// SetMultiple computes it once alongside its validation prehash — one
// full-document scan per mutation instead of two (validation self-hash +
// invalidation re-hash). jsonHash MUST equal hashStringToUint64(jsonStr) for
// the document being invalidated; the 0 sentinel (cache disabled)
// short-circuits below before use.
func (p *Processor) invalidateJSONCacheHashed(jsonHash uint64) {
	if !p.config.EnableCache || p.cache == nil {
		return
	}

	// PERFORMANCE: Set/Delete call this on every mutation. When the cache holds
	// no entries there is nothing to invalidate, so skip the per-shard scan
	// entirely. Profiling (P-001) showed that with the default config (cache
	// enabled) this path dominated Set/Delete CPU even though Set never
	// populates the cache — the scan ran write-locked across every shard for
	// no work.
	if p.cache.EntryCount() == 0 {
		return
	}

	// P-001: cache entries are keyed by the document's FNV-1a hash, so
	// invalidation passes the hash directly — no hex-string formatting and no
	// substring matching against composed keys (see DeleteByJSONHash).
	p.cache.DeleteByJSONHash(jsonHash)
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

// createCacheKeyWithHash creates a cache key using a pre-computed document hash.
// PERFORMANCE (P-001): returns a comparable internal.CacheKey struct instead of
// building an "op:hash16:path:opts" string — key construction no longer
// allocates, and the cache shards off JSONHash without re-hashing the key.
// Uses pointer identity check for default config to avoid 40+ field comparisons.
func (p *Processor) createCacheKeyWithHash(operation string, jsonHash uint64, path string, options *Config) internal.CacheKey {
	// Determine if options are default. Pointer-identity covers the overwhelmingly
	// common case (prepareOptions hands out the shared default singleton), avoiding
	// a 40+ field configFieldsEqual scan on every cache-key build. The fallback
	// handles a caller-supplied Config that happens to equal the default.
	// A default-valued config maps to OptHash 0 whether it arrived as the
	// singleton or by value, so both produce identical keys (mirrors the
	// previous isDefault logic).
	if options == nil || options == &defaultConfigSingleton || configFieldsEqual(*options, cachedDefaultConfigValue) {
		return internal.CacheKey{Op: operation, JSONHash: jsonHash, Path: path}
	}
	// hashConfig never sees a default-valued config here, so it returns the
	// field-hash path. OptHash 0 is reserved for "default", and the field hash
	// is 2^-64-unlikely to collide with it — remap defensively.
	optHash := hashConfig(*options)
	if optHash == 0 {
		optHash = 1
	}
	return internal.CacheKey{Op: operation, JSONHash: jsonHash, Path: path, OptHash: optHash}
}

// validateAndCacheKey validates jsonStr against options and returns the
// document-keyed CacheKey for op (empty path) in one step (P-001 round 2).
//
// It exists for the non-Get cache consumers (PreParse/Prettify/Compact/Valid),
// which all follow the same validate → build-key sequence: the document hash
// needed for the key is computed once HERE and threaded into the validation
// cache lookup, so the input is FNV-scanned once per operation instead of
// twice (validation self-hash + createCacheKey).
//
// With the result cache disabled there is no key to build — validation runs
// alone (self-hashing) and the zero CacheKey is returned. Callers' subsequent
// getCachedResult/setCachedResult calls on the zero key are no-ops: their
// EnableCache guards short-circuit before touching the cache.
func (p *Processor) validateAndCacheKey(op, jsonStr string, options *Config) (internal.CacheKey, error) {
	if !p.config.EnableCache {
		return internal.CacheKey{}, p.validateInputForOptions(jsonStr, options)
	}
	jsonHash := hashStringToUint64(jsonStr)
	if err := p.validateInputForOptionsHashed(jsonStr, options, jsonHash); err != nil {
		return internal.CacheKey{}, err
	}
	return p.createCacheKeyWithHash(op, jsonHash, "", options), nil
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
func (p *Processor) getCachedResult(key internal.CacheKey) (any, bool) {
	if !p.config.EnableCache {
		return nil, false
	}
	return p.cache.Get(key)
}

// setCachedResult stores a result in cache with security validation.
// options may be nil. D-002/R9 (m11): plain *Config parameter instead of the
// variadic form — exactly one option is ever passed, and the variadic invited
// silent multi-argument misuse.
//
// P-001: the old isValidCacheKey guard (length/control-char scan of the
// composed string key) is gone — struct keys are built from internal op tags
// and the already-validated document hash, so there is no string to inject.
func (p *Processor) setCachedResult(key internal.CacheKey, result any, options *Config) {
	if !p.config.EnableCache {
		return
	}

	// Check if caching is enabled for this operation. D-002/R10: with no cfg
	// (the shared singleton), the processor's baked CacheResults applies
	// (D-006) — previously the singleton's true silently re-enabled result
	// caching on a processor built with CacheResults=false.
	cacheResults := p.config.CacheResults
	if options != nil && options != &defaultConfigSingleton {
		cacheResults = options.CacheResults
	}
	if !cacheResults {
		return
	}

	// Security validation: don't cache potentially sensitive data
	if p.containsSensitiveData(result) {
		return
	}

	p.cache.Set(key, result)
}

// setCachedResultInternal stores a result in cache without sensitive data check
// PERFORMANCE: For trusted internal results (parsed JSON, navigation results) where
// security validation already happened at input. Skips expensive sensitive data scanning.
func (p *Processor) setCachedResultInternal(key internal.CacheKey, result any) {
	if !p.config.EnableCache {
		return
	}

	p.cache.Set(key, result)
}

// invalidateCachedResult removes a cache entry by key.
// Used when a cached value has a type mismatch (corrupted entry).
func (p *Processor) invalidateCachedResult(key internal.CacheKey) {
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
