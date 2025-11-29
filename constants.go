package json

import "time"

// =============================================================================
// CONSTANTS - Centralized configuration values
// =============================================================================

const (
	// Buffer and Pool Sizes
	DefaultBufferSize        = 1024  // Default buffer size for string builders
	MaxPoolBufferSize        = 16384 // Maximum buffer size to keep in pool
	MinPoolBufferSize        = 512   // Minimum buffer size to keep in pool
	DefaultPathSegmentCap    = 8     // Default capacity for path segment slices
	MaxPathSegmentCap        = 128   // Maximum path segment capacity to pool
	DefaultStringBuilderSize = 256   // Initial size for string builders

	// Cache Sizes
	DefaultCacheSize     = 128  // Default cache size for path/JSON parsing
	MaxCacheEntries      = 512  // Maximum cache entries before cleanup
	CacheCleanupKeepSize = 256  // Number of entries to keep after cleanup (MaxCacheEntries/2)

	// Operation Limits
	InvalidArrayIndex         = -999999           // Marker for invalid array index
	DefaultMaxJSONSize        = 10 * 1024 * 1024  // 10MB default max JSON size
	DefaultMaxSecuritySize    = 10 * 1024 * 1024  // 10MB for security validation
	DefaultMaxNestingDepth    = 50                // Maximum nesting depth for security
	DefaultMaxObjectKeys      = 10000             // Maximum object keys
	DefaultMaxArrayElements   = 10000             // Maximum array elements
	DefaultMaxPathDepth       = 100               // Maximum path depth
	DefaultMaxBatchSize       = 1000              // Maximum batch operation size
	DefaultMaxConcurrency     = 100               // Maximum concurrent operations
	DefaultParallelThreshold  = 10                // Threshold for parallel processing

	// Timing and Intervals
	MemoryPressureCheckInterval = 50000              // Check memory every N operations
	PoolResetInterval           = 100000             // Reset pools every N operations
	PoolResetIntervalPressure   = 50000              // Reset pools under memory pressure
	CacheCleanupInterval        = 30 * time.Second   // Cleanup stale cache entries
	DeadlockCheckInterval       = 30 * time.Second   // Check for deadlocks
	DeadlockThreshold           = 30 * time.Second   // Consider operation deadlocked after this
	SlowOperationThreshold      = 100 * time.Millisecond // Log warning for slow operations

	// Retry and Timeout
	MaxRetries              = 3                    // Maximum retry attempts
	BaseRetryDelay          = 10 * time.Millisecond // Base delay for exponential backoff
	DefaultOperationTimeout = 30 * time.Second     // Default timeout for operations
	AcquireSlotRetryDelay   = 1 * time.Millisecond // Delay between slot acquisition retries

	// Path Validation
	MaxPathLength         = 4096 // Maximum path length
	MaxSegmentLength      = 1024 // Maximum segment length
	MaxExtractionDepth    = 10   // Maximum extraction nesting depth
	MaxConsecutiveColons  = 3    // Maximum consecutive colons in path
	MaxConsecutiveBrackets = 5   // Maximum consecutive brackets

	// Security
	MaxSecurityValidationSize = 10 * 1024 * 1024 // 10MB max for security validation
	MaxAllowedNestingDepth    = 50               // Maximum allowed nesting depth
	MaxAllowedObjectKeys      = 10000            // Maximum allowed object keys
	MaxAllowedArrayElements   = 10000            // Maximum allowed array elements
	PathTraversalMaxLength    = 100              // Maximum path length for sanitization display

	// Cache TTL
	DefaultCacheTTL = 5 * time.Minute // Default cache time-to-live
)

// Error codes for machine-readable error identification
const (
	ErrCodeInvalidJSON       = "ERR_INVALID_JSON"
	ErrCodePathNotFound      = "ERR_PATH_NOT_FOUND"
	ErrCodeTypeMismatch      = "ERR_TYPE_MISMATCH"
	ErrCodeSizeLimit         = "ERR_SIZE_LIMIT"
	ErrCodeDepthLimit        = "ERR_DEPTH_LIMIT"
	ErrCodeSecurityViolation = "ERR_SECURITY_VIOLATION"
	ErrCodeRateLimit         = "ERR_RATE_LIMIT"
	ErrCodeTimeout           = "ERR_TIMEOUT"
	ErrCodeConcurrencyLimit  = "ERR_CONCURRENCY_LIMIT"
	ErrCodeProcessorClosed   = "ERR_PROCESSOR_CLOSED"
)
