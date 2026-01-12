package json

import "time"

// ConfigInterface defines the interface for configuration objects
type ConfigInterface interface {
	IsCacheEnabled() bool
	GetMaxCacheSize() int
	GetCacheTTL() time.Duration
	GetMaxJSONSize() int64
	GetMaxPathDepth() int
	GetMaxConcurrency() int
	IsMetricsEnabled() bool
	IsStrictMode() bool
	AllowComments() bool
	PreserveNumbers() bool
	ShouldCreatePaths() bool
	ShouldCleanupNulls() bool
	ShouldCompactArrays() bool
	ShouldValidateInput() bool
	GetMaxNestingDepth() int
}

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
	DefaultMaxJSONSize       = 100 * 1024 * 1024 // 100MB - reasonable for most use cases
	DefaultMaxSecuritySize   = 10 * 1024 * 1024
	DefaultMaxNestingDepth   = 200
	DefaultMaxObjectKeys     = 100000
	DefaultMaxArrayElements  = 100000
	DefaultMaxPathDepth      = 50
	DefaultMaxBatchSize      = 2000
	DefaultMaxConcurrency    = 50
	DefaultParallelThreshold = 10

	// Timing and Intervals - Optimized for responsiveness
	MemoryPressureCheckInterval = 30 * time.Second
	PoolResetInterval           = 60 * time.Second
	PoolResetIntervalPressure   = 30 * time.Second
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

	// Cache TTL
	DefaultCacheTTL = 5 * time.Minute

	// JSON processing thresholds
	SmallJSONThreshold  = 256  // Threshold for lightweight JSON normalization
	MediumJSONThreshold = 1024 // Threshold for full JSON normalization

	// Cache key constants
	CacheKeyHashLength   = 32   // Length for cache key hash
	SmallJSONCacheLimit  = 2048 // Limit for caching small JSON strings
	EstimatedKeyOverhead = 32   // Estimated overhead for cache key generation
	LargeJSONKeyOverhead = 64   // Overhead for large JSON cache keys
	MaxCacheKeyLength    = 500  // Maximum allowed cache key length

	// Validation constants
	ValidationBOMPrefix = "\uFEFF" // UTF-8 BOM prefix to detect and remove
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
