package json

import (
	"sync"
	"time"
)

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
	IsCommentsAllowed() bool
	ShouldPreserveNumbers() bool
	ShouldCreatePaths() bool
	ShouldCleanupNulls() bool
	ShouldCompactArrays() bool
	ShouldValidateInput() bool
	GetMaxNestingDepth() int
}

// Configuration constants with optimized defaults for production workloads.
const (
	// Buffer and Pool Sizes - Optimized for production workloads
	DefaultBufferSize        = 1024
	MaxPoolBufferSize        = 32768 // 32KB max for better buffer reuse
	MinPoolBufferSize        = 256   // 256B min for efficiency
	DefaultPathSegmentCap    = 8
	MaxPathSegmentCap        = 32 // Reduced from 128
	DefaultStringBuilderSize = 256

	// Cache Sizes - Balanced for performance and memory
	DefaultCacheSize     = 128
	MaxCacheEntries      = 512
	CacheCleanupKeepSize = 256

	// Operation Limits - Secure defaults with reasonable headroom
	// InvalidArrayIndex is a sentinel value indicating an invalid or out-of-bounds array index
	InvalidArrayIndex        = -999999
	DefaultMaxJSONSize       = 100 * 1024 * 1024 // 100MB
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
	// Note: MaxPathLength (5000) is also defined in internal/constants.go for internal use
	// Both definitions must be kept in sync
	MaxPathLength    = 5000
	MaxSegmentLength = 1024

	// Cache TTL
	DefaultCacheTTL = 5 * time.Minute

	// JSON processing thresholds
	SmallJSONThreshold  = 256  // Threshold for lightweight JSON normalization
	MediumJSONThreshold = 1024 // Threshold for full JSON normalization

	// Cache key constants - OPTIMIZED: Increased limits for better cache hit rate
	CacheKeyHashLength   = 32      // Length for cache key hash
	SmallJSONCacheLimit  = 2048    // Limit for caching small JSON strings (fast path)
	MediumJSONCacheLimit = 51200   // Limit for caching medium JSON strings (50KB)
	LargeJSONCacheLimit  = 1048576 // Limit for caching large JSON strings (1MB) - OPTIMIZED: increased for better performance
	EstimatedKeyOverhead = 32      // Estimated overhead for cache key generation
	LargeJSONKeyOverhead = 64      // Overhead for large JSON cache keys
	MaxCacheKeyLength    = 500     // Maximum allowed cache key length

	// Validation constants
	ValidationBOMPrefix = "\uFEFF" // UTF-8 BOM prefix to detect and remove
)

// Error codes for machine-readable error identification.
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

// DefaultConfig returns the default configuration.
// Creates a new instance each time to allow modifications without affecting other callers.
// PERFORMANCE NOTE: For read-only access in hot paths, cache the result.
func DefaultConfig() *Config {
	return &Config{
		MaxCacheSize:              DefaultCacheSize,
		CacheTTL:                  DefaultCacheTTL,
		EnableCache:               true,
		MaxJSONSize:               DefaultMaxJSONSize,
		MaxPathDepth:              DefaultMaxPathDepth,
		MaxBatchSize:              DefaultMaxBatchSize,
		MaxNestingDepthSecurity:   DefaultMaxNestingDepth,
		MaxSecurityValidationSize: DefaultMaxSecuritySize,
		MaxObjectKeys:             DefaultMaxObjectKeys,
		MaxArrayElements:          DefaultMaxArrayElements,
		MaxConcurrency:            DefaultMaxConcurrency,
		ParallelThreshold:         DefaultParallelThreshold,
		EnableValidation:          true,
		StrictMode:                false,
		CreatePaths:               false,
		CleanupNulls:              false,
		CompactArrays:             false,
		EnableMetrics:             false,
		EnableHealthCheck:         false,
		AllowComments:             false,
		PreserveNumbers:           false,
		ValidateInput:             true,
		ValidateFilePath:          true,
	}
}

// ValidateConfig validates configuration values and applies corrections
func ValidateConfig(config *Config) error {
	if config == nil {
		return newOperationError("validate_config", "config cannot be nil", ErrOperationFailed)
	}

	if config.MaxCacheSize < 0 {
		return newOperationError("validate_config", "MaxCacheSize cannot be negative", ErrOperationFailed)
	}

	// Apply defaults for invalid values
	if config.MaxJSONSize <= 0 {
		config.MaxJSONSize = DefaultMaxJSONSize
	}
	if config.MaxPathDepth <= 0 {
		config.MaxPathDepth = DefaultMaxPathDepth
	}
	if config.MaxConcurrency <= 0 {
		config.MaxConcurrency = DefaultMaxConcurrency
	}
	if config.MaxNestingDepthSecurity <= 0 {
		config.MaxNestingDepthSecurity = DefaultMaxNestingDepth
	}
	if config.MaxObjectKeys <= 0 {
		config.MaxObjectKeys = DefaultMaxObjectKeys
	}
	if config.MaxArrayElements <= 0 {
		config.MaxArrayElements = DefaultMaxArrayElements
	}

	return nil
}

// HighSecurityConfig returns a configuration with enhanced security settings
// for processing untrusted input from external sources.
//
// SECURITY: This configuration enables FullSecurityScan by default, which
// disables sampling-based validation and performs complete security scanning
// on all JSON input. Use this for public APIs, authentication endpoints,
// financial data processing, or any scenario with untrusted input.
func HighSecurityConfig() *Config {
	config := DefaultConfig()
	config.MaxNestingDepthSecurity = 20
	config.MaxSecurityValidationSize = 10 * 1024 * 1024
	config.MaxObjectKeys = 1000
	config.MaxArrayElements = 1000
	config.MaxJSONSize = 5 * 1024 * 1024
	config.MaxPathDepth = 20
	config.EnableValidation = true
	config.StrictMode = true
	config.FullSecurityScan = true // SECURITY: Enable full scan for maximum protection
	return config
}

// LargeDataConfig returns a configuration optimized for large JSON datasets
func LargeDataConfig() *Config {
	config := DefaultConfig()
	config.MaxNestingDepthSecurity = 100
	config.MaxSecurityValidationSize = 500 * 1024 * 1024
	config.MaxObjectKeys = 50000
	config.MaxArrayElements = 50000
	config.MaxJSONSize = 100 * 1024 * 1024
	config.MaxPathDepth = 200
	return config
}

// WebAPIConfig returns a configuration optimized for web API handlers.
// This configuration provides a balance between security and performance
// for public-facing APIs that receive JSON input from external clients.
//
// Key characteristics:
//   - Full security scan enabled for all input
//   - Moderate limits suitable for typical API payloads
//   - Strict mode enabled for predictable parsing
//   - Caching enabled for repeated operations
//
// Use this for: REST APIs, GraphQL endpoints, webhooks, public APIs.
func WebAPIConfig() *Config {
	config := DefaultConfig()
	// Security settings - moderate but comprehensive
	config.MaxNestingDepthSecurity = 50
	config.MaxSecurityValidationSize = 10 * 1024 * 1024 // 10MB
	config.MaxObjectKeys = 5000
	config.MaxArrayElements = 5000
	config.MaxJSONSize = 10 * 1024 * 1024 // 10MB max payload
	config.MaxPathDepth = 30
	config.FullSecurityScan = true
	config.StrictMode = true
	config.EnableValidation = true
	// Performance settings
	config.EnableCache = true
	config.MaxCacheSize = 256
	config.CacheTTL = 3 * time.Minute
	return config
}

// FastConfig returns a configuration optimized for trusted internal services.
// This configuration maximizes performance by reducing security overhead
// and should ONLY be used when you trust the source of JSON data.
//
// Key characteristics:
//   - Sampling-based security validation (faster but less thorough)
//   - Larger limits for trusted internal data
//   - Caching enabled with larger cache
//   - Non-strict mode for flexible parsing
//
// SECURITY WARNING: Do NOT use this configuration for:
//   - Public APIs
//   - User-submitted data
//   - External webhooks
//   - Any untrusted input
//
// Use this for: Internal microservices, config files, trusted data pipelines.
func FastConfig() *Config {
	config := DefaultConfig()
	// Relaxed limits for trusted internal data
	config.MaxNestingDepthSecurity = 150
	config.MaxSecurityValidationSize = 100 * 1024 * 1024 // 100MB
	config.MaxObjectKeys = 100000
	config.MaxArrayElements = 100000
	config.MaxJSONSize = 50 * 1024 * 1024 // 50MB
	config.MaxPathDepth = 100
	// Performance optimizations
	config.FullSecurityScan = false // Use sampling-based validation
	config.StrictMode = false
	config.EnableCache = true
	config.MaxCacheSize = 512
	config.CacheTTL = 10 * time.Minute
	config.MaxConcurrency = 100
	return config
}

// MinimalConfig returns a configuration with minimal overhead for maximum performance.
// This configuration disables most safety features and should only be used in
// controlled environments where you have complete control over the JSON data.
//
// Key characteristics:
//   - Security validation disabled
//   - Caching disabled
//   - Maximum limits
//   - No strict mode
//
// SECURITY WARNING: This configuration provides NO protection against:
//   - Malformed JSON attacks
//   - Deeply nested payload attacks
//   - Memory exhaustion attacks
//   - Prototype pollution attempts
//
// Use this for: Benchmarking, testing, trusted in-memory processing.
func MinimalConfig() *Config {
	config := DefaultConfig()
	// Maximum limits
	config.MaxNestingDepthSecurity = 200
	config.MaxSecurityValidationSize = 500 * 1024 * 1024 // 500MB
	config.MaxObjectKeys = 100000
	config.MaxArrayElements = 100000
	config.MaxJSONSize = 200 * 1024 * 1024 // 200MB
	config.MaxPathDepth = 200
	// Disable overhead features
	config.EnableValidation = false
	config.FullSecurityScan = false
	config.StrictMode = false
	config.EnableCache = false
	config.MaxCacheSize = 0
	config.EnableMetrics = false
	config.EnableHealthCheck = false
	return config
}

// defaultEncodeConfigPool caches encode config objects to reduce allocations
// PERFORMANCE: Avoids repeated struct allocations for hot encoding paths
var defaultEncodeConfigPool = &sync.Pool{
	New: func() any {
		return &EncodeConfig{
			Pretty:          false,
			Indent:          "  ",
			Prefix:          "",
			EscapeHTML:      true,
			SortKeys:        false,
			ValidateUTF8:    true,
			MaxDepth:        100,
			DisallowUnknown: false,
			PreserveNumbers: false,
			FloatPrecision:  -1,
			FloatTruncate:   false,
			DisableEscaping: false,
			EscapeUnicode:   false,
			EscapeSlash:     false,
			EscapeNewlines:  true,
			EscapeTabs:      true,
			IncludeNulls:    true,
			CustomEscapes:   nil,
		}
	},
}

// DefaultEncodeConfig returns default encoding configuration
// PERFORMANCE: Uses sync.Pool to reduce allocations in hot paths
func DefaultEncodeConfig() *EncodeConfig {
	cfg := defaultEncodeConfigPool.Get().(*EncodeConfig)
	// Reset to defaults in case caller modified it
	*cfg = EncodeConfig{
		Pretty:          false,
		Indent:          "  ",
		Prefix:          "",
		EscapeHTML:      true,
		SortKeys:        false,
		ValidateUTF8:    true,
		MaxDepth:        100,
		DisallowUnknown: false,
		PreserveNumbers: false,
		FloatPrecision:  -1,
		FloatTruncate:   false,
		DisableEscaping: false,
		EscapeUnicode:   false,
		EscapeSlash:     false,
		EscapeNewlines:  true,
		EscapeTabs:      true,
		IncludeNulls:    true,
		CustomEscapes:   nil,
	}
	return cfg
}

// PutEncodeConfig returns an EncodeConfig to the pool
// PERFORMANCE: Call this after using DefaultEncodeConfig to reduce GC pressure
func PutEncodeConfig(cfg *EncodeConfig) {
	if cfg != nil {
		cfg.CustomEscapes = nil // Clear potential reference
		defaultEncodeConfigPool.Put(cfg)
	}
}

// NewPrettyConfig returns configuration for pretty-printed JSON
func NewPrettyConfig() *EncodeConfig {
	cfg := DefaultEncodeConfig()
	cfg.Pretty = true
	cfg.Indent = "  "
	return cfg
}

// Clone creates a copy of the configuration.
// Note: Config currently contains only value types (int, bool, time.Duration),
// so a shallow copy is sufficient. If reference types (slices, maps, pointers)
// are added in the future, this method must be updated to perform deep copying.
func (c *Config) Clone() *Config {
	if c == nil {
		return DefaultConfig()
	}

	clone := *c
	return &clone
}

// Validate validates the configuration and applies corrections
func (c *Config) Validate() error {
	// Clamp int64 values
	clampInt64 := func(value *int64, min, max int64) {
		if *value <= 0 {
			*value = min
		} else if *value > max {
			*value = max
		}
	}

	// Clamp int values
	clampInt := func(value *int, min, max int) {
		if *value <= 0 {
			*value = min
		} else if *value > max {
			*value = max
		}
	}

	clampInt64(&c.MaxJSONSize, 1024*1024, 100*1024*1024)
	clampInt(&c.MaxPathDepth, 10, 200)
	clampInt(&c.MaxNestingDepthSecurity, 10, 200)
	clampInt(&c.MaxConcurrency, 1, 200)
	clampInt(&c.ParallelThreshold, 1, 50)

	if c.MaxCacheSize < 0 {
		c.MaxCacheSize = 0
		c.EnableCache = false
	} else if c.MaxCacheSize > 2000 {
		c.MaxCacheSize = 2000
	}

	if c.CacheTTL <= 0 {
		c.CacheTTL = DefaultCacheTTL
	}

	return nil
}

// ConfigInterface implementation methods
func (c *Config) IsCacheEnabled() bool         { return c.EnableCache }
func (c *Config) GetMaxCacheSize() int         { return c.MaxCacheSize }
func (c *Config) GetCacheTTL() time.Duration   { return c.CacheTTL }
func (c *Config) GetMaxJSONSize() int64        { return c.MaxJSONSize }
func (c *Config) GetMaxPathDepth() int         { return c.MaxPathDepth }
func (c *Config) GetMaxConcurrency() int       { return c.MaxConcurrency }
func (c *Config) IsMetricsEnabled() bool       { return c.EnableMetrics }
func (c *Config) IsHealthCheckEnabled() bool   { return c.EnableHealthCheck }
func (c *Config) IsStrictMode() bool           { return c.StrictMode }
func (c *Config) IsCommentsAllowed() bool      { return c.AllowComments }
func (c *Config) ShouldPreserveNumbers() bool  { return c.PreserveNumbers }
func (c *Config) ShouldCreatePaths() bool      { return c.CreatePaths }
func (c *Config) ShouldCleanupNulls() bool     { return c.CleanupNulls }
func (c *Config) ShouldCompactArrays() bool    { return c.CompactArrays }
func (c *Config) ShouldValidateInput() bool    { return c.ValidateInput }
func (c *Config) GetMaxNestingDepth() int      { return c.MaxNestingDepthSecurity }
func (c *Config) ShouldValidateFilePath() bool { return c.ValidateFilePath }
