package internal

import "time"

// ConfigInterface defines the interface for configuration objects
type ConfigInterface interface {

	// Cache configuration
	IsCacheEnabled() bool
	GetMaxCacheSize() int
	GetCacheTTL() time.Duration

	// Performance configuration
	GetMaxJSONSize() int64
	GetMaxPathDepth() int
	GetMaxConcurrency() int
	IsMetricsEnabled() bool
	IsHealthCheckEnabled() bool

	// Parsing configuration
	IsStrictMode() bool
	AllowComments() bool
	PreserveNumbers() bool

	// Operation configuration
	ShouldCreatePaths() bool
	ShouldCleanupNulls() bool
	ShouldCompactArrays() bool

	// Security configuration
	ShouldValidateInput() bool
	GetMaxNestingDepth() int
	IsRateLimitEnabled() bool
	GetRateLimitPerSec() int
	ShouldValidateFilePath() bool

	// Memory management
	AreResourcePoolsEnabled() bool
	GetMaxPoolSize() int
	GetPoolCleanupInterval() time.Duration
}

// ProcessorOptionsInterface defines the interface for processor options
type ProcessorOptionsInterface interface {
	GetContext() interface{} // Returns context.Context but as interface{} to avoid import
	ShouldCacheResults() bool
	IsStrictMode() bool
	GetMaxDepth() int
	AllowComments() bool
	PreserveNumbers() bool
	ShouldCreatePaths() bool
	ShouldCleanupNulls() bool
	ShouldCompactArrays() bool
}

// EncodeConfigInterface defines the interface for encoding configuration
type EncodeConfigInterface interface {
	// Formatting options
	IsPretty() bool
	GetIndent() string
	GetPrefix() string
	ShouldSortKeys() bool
	ShouldEscapeHTML() bool

	// Output options
	IsCompactOutput() bool
	HasTrailingComma() bool

	// Number formatting
	ShouldPreserveNumbers() bool
	GetFloatPrecision() int

	// String handling
	ShouldEscapeUnicode() bool
	GetQuoteStyle() int

	// Advanced options
	ShouldOmitEmpty() bool
	ShouldIncludeNulls() bool
	ShouldValidateUTF8() bool
	GetMaxDepth() int
	GetBufferSize() int
}

// StatsInterface defines the interface for statistics
type StatsInterface interface {
	GetCacheSize() int64
	GetCacheMemory() int64
	GetMaxCacheSize() int
	GetHitCount() int64
	GetMissCount() int64
	GetHitRatio() float64
	GetCacheTTL() time.Duration
	IsCacheEnabled() bool
	GetOperationCount() int64
	GetErrorCount() int64
}

// MetricsInterface defines the interface for metrics collection
type MetricsInterface interface {
	RecordOperation(duration time.Duration, success bool, memoryUsed int64)
	RecordCacheHit()
	RecordCacheMiss()
	StartConcurrentOperation()
	EndConcurrentOperation()
	RecordError(errorType string)
	GetMetrics() interface{} // Returns Metrics but as interface{} to avoid circular dependency
	Reset()
	GetSummary() string
}

// HealthCheckerInterface defines the interface for health checking
type HealthCheckerInterface interface {
	CheckHealth() interface{} // Returns HealthStatus but as interface{} to avoid circular dependency
}

// CacheInterface defines the interface for cache operations
type CacheInterface interface {
	Get(key string) (any, bool)
	Set(key string, value any)
	ClearCache()
	GetStats() interface{} // Returns CacheStats but as interface{} to avoid circular dependency
}

// PathParserInterface defines the interface for path parsing (internal use)
type PathParserInterface interface {
	ParsePath(path string) ([]interface{}, error) // Returns []PathSegment but as []interface{}
	ValidatePath(path string) error
	SplitPathIntoSegments(path string) []interface{} // Returns []PathSegmentInfo but as []interface{}
}
