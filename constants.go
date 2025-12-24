package json

import "time"

const (
	// Buffer and Pool Sizes - OPTIMIZED for better memory management
	DefaultBufferSize        = 1024  // Reduced from 2048
	MaxPoolBufferSize        = 16384 // Reduced from 65536
	MinPoolBufferSize        = 512   // Reduced from 1024
	DefaultPathSegmentCap    = 8     // Reduced from 16
	MaxPathSegmentCap        = 128   // Reduced from 256
	DefaultStringBuilderSize = 256   // Reduced from 512

	// Cache Sizes - OPTIMIZED for better performance/memory balance
	DefaultCacheSize     = 128 // Reduced from 256
	MaxCacheEntries      = 512 // Reduced from 1024
	CacheCleanupKeepSize = 256 // Reduced from 512

	// Operation Limits - TIGHTENED for better security
	InvalidArrayIndex        = -999999
	DefaultMaxJSONSize       = 5 * 1024 * 1024 // Reduced from 10MB to 5MB
	DefaultMaxSecuritySize   = 5 * 1024 * 1024 // Reduced from 10MB to 5MB
	DefaultMaxNestingDepth   = 32              // Reduced from 50
	DefaultMaxObjectKeys     = 5000            // Reduced from 10000
	DefaultMaxArrayElements  = 5000            // Reduced from 10000
	DefaultMaxPathDepth      = 50              // Reduced from 100
	DefaultMaxBatchSize      = 500             // Reduced from 1000
	DefaultMaxConcurrency    = 50              // Reduced from 100
	DefaultParallelThreshold = 5               // Reduced from 10

	// Timing and Intervals - OPTIMIZED for better responsiveness
	MemoryPressureCheckInterval = 25000                 // Reduced from 50000
	PoolResetInterval           = 50000                 // Reduced from 100000
	PoolResetIntervalPressure   = 25000                 // Reduced from 50000
	CacheCleanupInterval        = 15 * time.Second      // Reduced from 30s
	DeadlockCheckInterval       = 15 * time.Second      // Reduced from 30s
	DeadlockThreshold           = 15 * time.Second      // Reduced from 30s
	SlowOperationThreshold      = 50 * time.Millisecond // Reduced from 100ms

	// Retry and Timeout - OPTIMIZED for better user experience
	MaxRetries              = 2                      // Reduced from 3
	BaseRetryDelay          = 5 * time.Millisecond   // Reduced from 10ms
	DefaultOperationTimeout = 15 * time.Second       // Reduced from 30s
	AcquireSlotRetryDelay   = 500 * time.Microsecond // Reduced from 1ms

	// Path Validation - TIGHTENED for better security
	MaxPathLength          = 5000 // Reduced from 10000
	MaxSegmentLength       = 512  // Reduced from 1024
	MaxExtractionDepth     = 5    // Reduced from 10
	MaxConsecutiveColons   = 2    // Reduced from 3
	MaxConsecutiveBrackets = 3    // Reduced from 5

	// Security - ENHANCED limits
	MaxSecurityValidationSize = 5 * 1024 * 1024 // Reduced from 10MB
	MaxAllowedNestingDepth    = 32              // Reduced from 50
	MaxAllowedObjectKeys      = 5000            // Reduced from 10000
	MaxAllowedArrayElements   = 5000            // Reduced from 10000
	PathTraversalMaxLength    = 50              // Reduced from 100

	// Cache TTL - OPTIMIZED for better cache efficiency
	DefaultCacheTTL = 2 * time.Minute // Reduced from 5 minutes
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
