package json

import "time"

const (
	// Buffer and Pool Sizes
	DefaultBufferSize        = 2048
	MaxPoolBufferSize        = 65536
	MinPoolBufferSize        = 1024
	DefaultPathSegmentCap    = 16
	MaxPathSegmentCap        = 256
	DefaultStringBuilderSize = 512

	// Cache Sizes
	DefaultCacheSize     = 256
	MaxCacheEntries      = 1024
	CacheCleanupKeepSize = 512

	// Operation Limits
	InvalidArrayIndex        = -999999
	DefaultMaxJSONSize       = 10 * 1024 * 1024
	DefaultMaxSecuritySize   = 10 * 1024 * 1024
	DefaultMaxNestingDepth   = 50
	DefaultMaxObjectKeys     = 10000
	DefaultMaxArrayElements  = 10000
	DefaultMaxPathDepth      = 100
	DefaultMaxBatchSize      = 1000
	DefaultMaxConcurrency    = 100
	DefaultParallelThreshold = 10

	// Timing and Intervals
	MemoryPressureCheckInterval = 50000
	PoolResetInterval           = 100000
	PoolResetIntervalPressure   = 50000
	CacheCleanupInterval        = 30 * time.Second
	DeadlockCheckInterval       = 30 * time.Second
	DeadlockThreshold           = 30 * time.Second
	SlowOperationThreshold      = 100 * time.Millisecond

	// Retry and Timeout
	MaxRetries              = 3
	BaseRetryDelay          = 10 * time.Millisecond
	DefaultOperationTimeout = 30 * time.Second
	AcquireSlotRetryDelay   = 1 * time.Millisecond

	// Path Validation
	MaxPathLength          = 10000 // Maximum path length for security
	MaxSegmentLength       = 1024
	MaxExtractionDepth     = 10
	MaxConsecutiveColons   = 3
	MaxConsecutiveBrackets = 5

	// Security
	MaxSecurityValidationSize = 10 * 1024 * 1024
	MaxAllowedNestingDepth    = 50
	MaxAllowedObjectKeys      = 10000
	MaxAllowedArrayElements   = 10000
	PathTraversalMaxLength    = 100

	// Cache TTL
	DefaultCacheTTL = 5 * time.Minute
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
