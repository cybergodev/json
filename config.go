package json

import (
	"fmt"
	"time"
)

// DefaultConfig returns a default configuration for the JSON processor
func DefaultConfig() *Config {
	return &Config{
		MaxCacheSize:      1000,
		CacheTTL:          5 * time.Minute,
		EnableCache:       true,
		MaxJSONSize:       10 * 1024 * 1024, // 10MB
		MaxPathDepth:      100,
		MaxBatchSize:      1000,
		MaxConcurrency:    50,
		ParallelThreshold: 10,
		EnableValidation:  true,
		StrictMode:        false,
		CreatePaths:       false, // Conservative default - don't auto-create paths

		// Security configuration with safe defaults
		MaxNestingDepthSecurity:   50,                // Maximum nesting depth for security validation
		MaxSecurityValidationSize: 100 * 1024 * 1024, // 100MB for security validation
		MaxObjectKeys:             10000,             // Maximum keys per object
		MaxArrayElements:          10000,             // Maximum array elements
	}
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
// Use with caution in production environments and ensure adequate system resources
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

// DefaultOptions returns default processor options
func DefaultOptions() *ProcessorOptions {
	return &ProcessorOptions{
		CacheResults:    true,
		StrictMode:      false,
		MaxDepth:        50,
		AllowComments:   false,
		PreserveNumbers: false, // Disable number preservation for encoding/json compatibility
		CreatePaths:     false, // Conservative default - don't auto-create paths
		CleanupNulls:    false, // Conservative default - don't auto-cleanup nulls
		CompactArrays:   false, // Conservative default - don't auto-compact arrays
		ContinueOnError: false, // Conservative default - fail fast on errors
	}
}

// DefaultSchema returns a default schema configuration
func DefaultSchema() *Schema {
	return &Schema{
		Type:                 "",
		Properties:           make(map[string]*Schema),
		Items:                nil,
		Required:             []string{},
		MinLength:            0,
		MaxLength:            0,
		Minimum:              0,
		Maximum:              0,
		Pattern:              "",
		Format:               "",
		AdditionalProperties: true, // Allow additional properties by default
		hasMinLength:         false,
		hasMaxLength:         false,
		hasMinimum:           false,
		hasMaximum:           false,
	}
}

// SetMinLength sets the minimum length constraint
func (s *Schema) SetMinLength(minLength int) *Schema {
	s.MinLength = minLength
	s.hasMinLength = true
	return s
}

// SetMaxLength sets the maximum length constraint
func (s *Schema) SetMaxLength(maxLength int) *Schema {
	s.MaxLength = maxLength
	s.hasMaxLength = true
	return s
}

// SetMinimum sets the minimum value constraint
func (s *Schema) SetMinimum(minimum float64) *Schema {
	s.Minimum = minimum
	s.hasMinimum = true
	return s
}

// SetMaximum sets the maximum value constraint
func (s *Schema) SetMaximum(maximum float64) *Schema {
	s.Maximum = maximum
	s.hasMaximum = true
	return s
}

// HasMinLength returns true if MinLength constraint is explicitly set
func (s *Schema) HasMinLength() bool {
	return s.hasMinLength
}

// HasMaxLength returns true if MaxLength constraint is explicitly set
func (s *Schema) HasMaxLength() bool {
	return s.hasMaxLength
}

// HasMinimum returns true if Minimum constraint is explicitly set
func (s *Schema) HasMinimum() bool {
	return s.hasMinimum
}

// HasMaximum returns true if Maximum constraint is explicitly set
func (s *Schema) HasMaximum() bool {
	return s.hasMaximum
}

// SetMinItems sets the minimum items constraint for arrays
func (s *Schema) SetMinItems(minItems int) *Schema {
	s.MinItems = minItems
	s.hasMinItems = true
	return s
}

// SetMaxItems sets the maximum items constraint for arrays
func (s *Schema) SetMaxItems(maxItems int) *Schema {
	s.MaxItems = maxItems
	s.hasMaxItems = true
	return s
}

// HasMinItems returns true if MinItems constraint is explicitly set
func (s *Schema) HasMinItems() bool {
	return s.hasMinItems
}

// HasMaxItems returns true if MaxItems constraint is explicitly set
func (s *Schema) HasMaxItems() bool {
	return s.hasMaxItems
}

// SetExclusiveMinimum sets the exclusive minimum flag
func (s *Schema) SetExclusiveMinimum(exclusive bool) *Schema {
	s.ExclusiveMinimum = exclusive
	return s
}

// SetExclusiveMaximum sets the exclusive maximum flag
func (s *Schema) SetExclusiveMaximum(exclusive bool) *Schema {
	s.ExclusiveMaximum = exclusive
	return s
}

// GetSecurityLimits returns a summary of current security limits
func (c *Config) GetSecurityLimits() map[string]interface{} {
	return map[string]interface{}{
		"max_nesting_depth":            c.MaxNestingDepthSecurity,
		"max_security_validation_size": c.MaxSecurityValidationSize,
		"max_object_keys":              c.MaxObjectKeys,
		"max_array_elements":           c.MaxArrayElements,
		"max_json_size":                c.MaxJSONSize,
		"max_path_depth":               c.MaxPathDepth,
	}
}

// ValidateConfig validates processor configuration with comprehensive checks
func ValidateConfig(config *Config) error {
	if config == nil {
		return fmt.Errorf("config cannot be nil")
	}

	// Valid cache settings
	if config.MaxCacheSize < 0 {
		return fmt.Errorf("MaxCacheSize cannot be negative: %d", config.MaxCacheSize)
	}
	if config.MaxCacheSize > 1000000 {
		return fmt.Errorf("MaxCacheSize too large (max 1,000,000): %d", config.MaxCacheSize)
	}
	if config.CacheTTL < 0 {
		return fmt.Errorf("CacheTTL cannot be negative: %v", config.CacheTTL)
	}
	if config.CacheTTL > 24*time.Hour {
		return fmt.Errorf("CacheTTL too large (max 24h): %v", config.CacheTTL)
	}

	// Valid size limits
	if config.MaxJSONSize < 0 {
		return fmt.Errorf("MaxJSONSize cannot be negative: %d", config.MaxJSONSize)
	}
	if config.MaxJSONSize > 1024*1024*1024 { // 1GB limit
		return fmt.Errorf("MaxJSONSize too large (max 1GB): %d", config.MaxJSONSize)
	}
	if config.MaxPathDepth < 1 {
		return fmt.Errorf("MaxPathDepth must be at least 1: %d", config.MaxPathDepth)
	}
	if config.MaxPathDepth > 1000 {
		return fmt.Errorf("MaxPathDepth too large (max 1000): %d", config.MaxPathDepth)
	}
	if config.MaxBatchSize < 0 {
		return fmt.Errorf("MaxBatchSize cannot be negative: %d", config.MaxBatchSize)
	}
	if config.MaxBatchSize > 100000 {
		return fmt.Errorf("MaxBatchSize too large (max 100,000): %d", config.MaxBatchSize)
	}

	// Valid concurrency settings
	if config.MaxConcurrency < 1 {
		return fmt.Errorf("MaxConcurrency must be at least 1: %d", config.MaxConcurrency)
	}
	if config.MaxConcurrency > 1000 {
		return fmt.Errorf("MaxConcurrency too large (max 1000): %d", config.MaxConcurrency)
	}
	if config.ParallelThreshold < 1 {
		return fmt.Errorf("ParallelThreshold must be at least 1: %d", config.ParallelThreshold)
	}

	// Validate security limits with autocorrection
	if config.MaxNestingDepthSecurity <= 0 {
		config.MaxNestingDepthSecurity = 50 // Set safe default
	}
	if config.MaxNestingDepthSecurity > 1000 {
		return fmt.Errorf("MaxNestingDepthSecurity too high (%d), maximum allowed is 1000", config.MaxNestingDepthSecurity)
	}

	if config.MaxSecurityValidationSize <= 0 {
		config.MaxSecurityValidationSize = 100 * 1024 * 1024 // Set safe default
	}
	if config.MaxSecurityValidationSize > 1024*1024*1024 { // 1GB limit
		return fmt.Errorf("MaxSecurityValidationSize too high (%d), maximum allowed is 1GB", config.MaxSecurityValidationSize)
	}

	if config.MaxObjectKeys <= 0 {
		config.MaxObjectKeys = 10000 // Set safe default
	}
	if config.MaxObjectKeys > 1000000 { // 1M keys limit
		return fmt.Errorf("MaxObjectKeys too high (%d), maximum allowed is 1,000,000", config.MaxObjectKeys)
	}

	if config.MaxArrayElements <= 0 {
		config.MaxArrayElements = 10000 // Set safe default
	}
	if config.MaxArrayElements > 1000000 { // 1M elements limit
		return fmt.Errorf("MaxArrayElements too high (%d), maximum allowed is 1,000,000", config.MaxArrayElements)
	}

	return nil
}

// ValidateOptions validates processor options with enhanced checks
func ValidateOptions(options *ProcessorOptions) error {
	if options == nil {
		return fmt.Errorf("options cannot be nil")
	}

	if options.MaxDepth < 0 {
		return fmt.Errorf("MaxDepth cannot be negative: %d", options.MaxDepth)
	}
	if options.MaxDepth > 1000 {
		return fmt.Errorf("MaxDepth too large (max 1000): %d", options.MaxDepth)
	}

	return nil
}

// =============================================================================
// ENCODING OPTIONS AND FUNCTIONAL OPTIONS
// =============================================================================

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
		PreserveNumbers: false, // Disable number preservation for encoding/json compatibility
		FloatPrecision:  -1,    // Automatic precision
		DisableEscaping: false,
		EscapeUnicode:   false,
		EscapeSlash:     false,
		EscapeNewlines:  true,
		EscapeTabs:      true,
		IncludeNulls:    true, // Include null values by default
		CustomEscapes:   nil,
	}
}

// =============================================================================
// CONVENIENT CONFIGURATION BUILDERS
// =============================================================================

// NewPrettyConfig creates a configuration for pretty-printed JSON
func NewPrettyConfig() *EncodeConfig {
	return &EncodeConfig{
		Pretty:          true,
		Indent:          "  ",
		Prefix:          "",
		EscapeHTML:      true,
		SortKeys:        true,
		OmitEmpty:       false,
		ValidateUTF8:    true,
		MaxDepth:        100,
		DisallowUnknown: false,
		PreserveNumbers: true, // Enable number preservation for pretty printing
		FloatPrecision:  -1,   // Automatic precision
		DisableEscaping: false,
		EscapeUnicode:   false,
		EscapeSlash:     false,
		EscapeNewlines:  true,
		EscapeTabs:      true,
		IncludeNulls:    true, // Include null values by default
		CustomEscapes:   nil,
	}
}

// NewCompactConfig creates a configuration for compact JSON
func NewCompactConfig() *EncodeConfig {
	return &EncodeConfig{
		Pretty:          false,
		Indent:          "",
		Prefix:          "",
		EscapeHTML:      true,
		SortKeys:        false,
		OmitEmpty:       false,
		ValidateUTF8:    true,
		MaxDepth:        100,
		DisallowUnknown: false,
		PreserveNumbers: false, // Disable number preservation for encoding/json compatibility
		FloatPrecision:  -1,    // Automatic precision
		DisableEscaping: false,
		EscapeUnicode:   false,
		EscapeSlash:     false,
		EscapeNewlines:  true,
		EscapeTabs:      true,
		IncludeNulls:    true, // Include null values by default
		CustomEscapes:   nil,
	}
}

// NewReadableConfig creates a configuration for human-readable JSON with minimal escaping
func NewReadableConfig() *EncodeConfig {
	return &EncodeConfig{
		Pretty:          false,
		Indent:          "  ",
		Prefix:          "",
		EscapeHTML:      false,
		SortKeys:        false,
		OmitEmpty:       false,
		ValidateUTF8:    true,
		MaxDepth:        100,
		DisallowUnknown: false,
		DisableEscaping: true,
		EscapeUnicode:   false,
		EscapeSlash:     false,
		EscapeNewlines:  true,
		EscapeTabs:      true,
		IncludeNulls:    true, // Include null values by default
		CustomEscapes:   nil,
		PreserveNumbers: false, // Disable number preservation for encoding/json compatibility
	}
}

// NewWebSafeConfig creates a configuration for web-safe JSON
func NewWebSafeConfig() *EncodeConfig {
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
		DisableEscaping: false,
		EscapeUnicode:   false,
		EscapeSlash:     true,
		EscapeNewlines:  true,
		EscapeTabs:      true,
		IncludeNulls:    true, // Include null values by default
		CustomEscapes:   nil,
	}
}

// NewCleanConfig creates a configuration that omits null and empty values
func NewCleanConfig() *EncodeConfig {
	return &EncodeConfig{
		Pretty:          true,
		Indent:          "  ",
		Prefix:          "",
		EscapeHTML:      true,
		SortKeys:        true,
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
		IncludeNulls:    false, // Exclude null values for clean output
		CustomEscapes:   nil,
	}
}
