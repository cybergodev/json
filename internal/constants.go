package internal

// Unified constants for the JSON library
// These constants are used across multiple files to ensure consistency
// IMPORTANT: These are the single source of truth - do not duplicate in other packages

const (
	// Depth limits for various operations
	MaxDeepMergeDepth     = 100 // Maximum depth for deep merge operations
	MaxPathParseDepth     = 100 // Maximum depth for path parsing
	MaxNestingDepth       = 200 // Maximum JSON nesting depth for security validation
	MaxSensitiveDataDepth = 10  // Maximum depth for sensitive data detection

	// Path and cache limits
	MaxPathLength     = 5000 // Maximum path length for security (single source of truth)
	MaxCacheKeyLength = 1024 // Maximum cache key length to prevent memory issues

	// ============================================================================
	// POOL SIZE LIMITS - Single source of truth for all resource pools
	// These limits prevent memory bloat while maintaining good pool hit rates
	// ============================================================================

	// Buffer pool limits (for []byte)
	MinPoolBufferSize = 256   // Minimum capacity to pool (smaller buffers not worth pooling)
	MaxPoolBufferSize = 32768 // Upper bound on pooled byte-slice capacity. The
	// encoder and HTML-escape pools retain only up to MaxPoolBufferSize/4
	// (8KB) — escaping/encoding outputs of the full 32KB are discarded — and
	// the string-builder pool uses an unrelated 16KB cutoff. Tune the per-pool
	// divisors where the pools are defined, not here.
)
