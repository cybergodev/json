package json

import "time"

// DefaultConfig returns the default configuration with optimized settings
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
		EnableMetrics:             false, // Disabled by default for better performance
		EnableHealthCheck:         false, // Disabled by default for better performance
		AllowCommentsFlag:         false,
		PreserveNumbersFlag:       false,
		ValidateInput:             true,
		MaxNestingDepth:           DefaultMaxNestingDepth,
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
func HighSecurityConfig() *Config {
	config := DefaultConfig()
	// More restrictive security settings
	config.MaxNestingDepthSecurity = 20                 // Very restrictive nesting
	config.MaxSecurityValidationSize = 10 * 1024 * 1024 // 10MB limit
	config.MaxObjectKeys = 1000                         // Fewer keys allowed
	config.MaxArrayElements = 1000                      // Fewer array elements
	config.MaxJSONSize = 5 * 1024 * 1024                // 5MB JSON limit
	config.MaxPathDepth = 20                            // Shallow path depth
	config.EnableValidation = true                      // Force validation
	config.StrictMode = true                            // Enable strict mode
	return config
}

// LargeDataConfig returns a configuration optimized for processing large JSON datasets
func LargeDataConfig() *Config {
	config := DefaultConfig()
	// Optimized settings for large data processing
	config.MaxNestingDepthSecurity = 100                 // Allow deeper nesting for complex data
	config.MaxSecurityValidationSize = 500 * 1024 * 1024 // 500MB validation limit
	config.MaxObjectKeys = 50000                         // Support large objects
	config.MaxArrayElements = 50000                      // Support large arrays
	config.MaxJSONSize = 100 * 1024 * 1024               // 100MB JSON processing limit
	config.MaxPathDepth = 200                            // Support deep path traversal
	return config
}

// DefaultEncodeConfig returns default encoding configuration
func DefaultEncodeConfig() *EncodeConfig {
	return &EncodeConfig{
		Pretty:          false,
		Indent:          "  ",
		Prefix:          "",
		EscapeHTML:      true,
		SortKeys:        false,
		OmitEmpty:       false,
		ValidateUTF8:    true,
		MaxDepth:        100,
		DisallowUnknown: false,
		PreserveNumbers: false,
		FloatPrecision:  -1,
		DisableEscaping: false,
		EscapeUnicode:   false,
		EscapeSlash:     false,
		EscapeNewlines:  true,
		EscapeTabs:      true,
		IncludeNulls:    true,
		CustomEscapes:   nil,
	}
}

// NewPrettyConfig returns configuration for pretty-printed JSON
func NewPrettyConfig() *EncodeConfig {
	config := DefaultEncodeConfig()
	config.Pretty = true
	config.Indent = "  "
	return config
}

// NewCompactConfig returns configuration for compact JSON
func NewCompactConfig() *EncodeConfig {
	config := DefaultEncodeConfig()
	config.Pretty = false
	return config
}

// Clone creates a deep copy of the configuration
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
	clampInt(&c.MaxNestingDepth, 10, 100)
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
func (c *Config) AllowComments() bool          { return c.AllowCommentsFlag }
func (c *Config) PreserveNumbers() bool        { return c.PreserveNumbersFlag }
func (c *Config) ShouldCreatePaths() bool      { return c.CreatePaths }
func (c *Config) ShouldCleanupNulls() bool     { return c.CleanupNulls }
func (c *Config) ShouldCompactArrays() bool    { return c.CompactArrays }
func (c *Config) ShouldValidateInput() bool    { return c.ValidateInput }
func (c *Config) GetMaxNestingDepth() int      { return c.MaxNestingDepth }
func (c *Config) ShouldValidateFilePath() bool { return c.ValidateFilePath }
