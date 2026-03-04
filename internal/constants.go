package internal

// Unified constants for the JSON library
// These constants are used across multiple files to ensure consistency
// IMPORTANT: These are the single source of truth - do not duplicate in other packages

const (
	// Depth limits for various operations
	MaxDeepMergeDepth = 100 // Maximum depth for deep merge operations
	MaxPathParseDepth = 100 // Maximum depth for path parsing
	MaxNestingDepth   = 200 // Maximum JSON nesting depth for security validation

	// Path and cache limits
	// Note: MaxPathLength is also defined in config.go for the public API
	// Both should be kept in sync (value: 5000)
	MaxCacheKeyLength = 1024 // Maximum cache key length to prevent memory issues
)
