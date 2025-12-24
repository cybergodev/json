package json

import "time"

const (
	// Buffer and Pool Sizes - Optimized for production workloads
	DefaultBufferSize        = 1024
	MaxPoolBufferSize        = 16384
	MinPoolBufferSize        = 512
	DefaultPathSegmentCap    = 8
	MaxPathSegmentCap        = 128
	DefaultStringBuilderSize = 256

	// Cache Sizes - Balanced for performance and memory
	DefaultCacheSize     = 128
	MaxCacheEntries      = 512
	CacheCleanupKeepSize = 256

	// Operation Limits - Secure defaults with reasonable headroom
	InvalidArrayIndex        = -999999
	DefaultMaxJSONSize       = 10 * 1024 * 1024 // 10MB - reasonable for most use cases
	DefaultMaxSecuritySize   = 10 * 1024 * 1024
	DefaultMaxNestingDepth   = 32
	DefaultMaxObjectKeys     = 10000
	DefaultMaxArrayElements  = 10000
	DefaultMaxPathDepth      = 50
	DefaultMaxBatchSize      = 1000
	DefaultMaxConcurrency    = 50
	DefaultParallelThreshold = 10

	// Timing and Intervals - Optimized for responsiveness
	MemoryPressureCheckInterval = 30000
	PoolResetInterval           = 60000
	PoolResetIntervalPressure   = 30000
	CacheCleanupInterval        = 30 * time.Second
	DeadlockCheckInterval       = 30 * time.Second
	DeadlockThreshold           = 30 * time.Second
	SlowOperationThreshold      = 100 * time.Millisecond

	// Retry and Timeout - Production-ready settings
	MaxRetries              = 3
	BaseRetryDelay          = 10 * time.Millisecond
	DefaultOperationTimeout = 30 * time.Second
	AcquireSlotRetryDelay   = 1 * time.Millisecond

	// Path Validation - Secure but flexible
	MaxPathLength          = 5000
	MaxSegmentLength       = 1024
	MaxExtractionDepth     = 10
	MaxConsecutiveColons   = 3
	MaxConsecutiveBrackets = 3

	// Security constants (aliases for backward compatibility)
	MaxSecurityValidationSize = DefaultMaxSecuritySize
	MaxAllowedNestingDepth    = DefaultMaxNestingDepth
	MaxAllowedObjectKeys      = DefaultMaxObjectKeys
	MaxAllowedArrayElements   = DefaultMaxArrayElements

	// Cache TTL - Default cache time-to-live
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
	ErrCodeOperationFailed   = "ERR_OPERATION_FAILED"
	ErrCodeTimeout           = "ERR_TIMEOUT"
	ErrCodeConcurrencyLimit  = "ERR_CONCURRENCY_LIMIT"
	ErrCodeProcessorClosed   = "ERR_PROCESSOR_CLOSED"
	ErrCodeRateLimit         = "ERR_RATE_LIMIT"
)
