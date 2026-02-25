package internal

// Unified constants for the JSON library
// These constants are used across multiple files to ensure consistency

const (
	// Depth limits for various operations
	MaxDeepMergeDepth = 100  // Maximum depth for deep merge operations
	MaxPathParseDepth = 100  // Maximum depth for path parsing
	MaxNestingDepth   = 200  // Maximum JSON nesting depth for security validation
	MaxPathLength     = 5000 // Maximum path length for security validation
	MaxCacheKeyLength = 1024 // Maximum cache key length to prevent memory issues
)
