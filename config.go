package json

// DefaultConfig returns the default configuration
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
		EnableMetrics:             true,
		EnableHealthCheck:         true,
		AllowCommentsFlag:         false,
		PreserveNumbersFlag:       false,
		ValidateInput:             true,
		MaxNestingDepth:           DefaultMaxNestingDepth,
		ValidateFilePath:          true,
	}
}

// ValidateConfig validates configuration values
func ValidateConfig(config *Config) error {
	if config == nil {
		return newOperationError("validate_config", "config cannot be nil", ErrOperationFailed)
	}

	if config.MaxCacheSize < 0 {
		return newOperationError("validate_config", "MaxCacheSize cannot be negative", ErrOperationFailed)
	}

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
