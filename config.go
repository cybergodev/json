package json

import (
	"errors"
	"maps"
	"reflect"
	"time"

	"github.com/cybergodev/json/internal"
)

// Configuration constants with optimized defaults for production workloads.
const (
	// Cache Sizes - Balanced for performance and memory (internal)
	defaultCacheSize = 128

	// DefaultMaxJSONSize is the maximum allowed JSON input size (100MB).
	// JSON documents larger than this will be rejected with ErrSizeLimit.
	// Adjust via Config.MaxJSONSize if processing larger documents.
	DefaultMaxJSONSize = 100 * 1024 * 1024

	// DefaultMaxNestingDepth is the maximum allowed JSON nesting depth (200).
	// Deeply nested JSON may indicate malicious input.
	// Adjust via Config.MaxNestingDepthSecurity if needed.
	DefaultMaxNestingDepth = 200

	// DefaultMaxPathDepth is the maximum allowed path depth (50).
	// Limits the number of segments in a path like "a.b.c.d...".
	// Adjust via Config.MaxPathDepth if needed.
	DefaultMaxPathDepth = 50

	// DefaultMaxDepth is the default JSON encoding/decoding nesting depth (100).
	// Used by Config.MaxDepth. Aligns with the internal.ValidatePath array-index
	// cap so that path-derived allocations stay bounded.
	DefaultMaxDepth = 100

	// maxArrayExtension is the largest array length that Set/CreatePaths may
	// allocate in a single extension (e.g. Set(`{"a":[]}`, "a/99999999", v)).
	// SECURITY/STABILITY: bounds memory amplification from user-supplied indices
	// on paths that bypass full validation (SkipValidation, *FromParsed).
	// Matches internal.ValidatePath's maxArrayIndex policy.
	maxArrayExtension = 1000000

	// DefaultMaxConcurrency is the maximum concurrent operations (50).
	// Limits parallel operations to prevent resource exhaustion.
	// Adjust via Config.MaxConcurrency for high-throughput scenarios.
	DefaultMaxConcurrency = 50

	// DefaultMaxSecuritySize is the maximum size for security validation (10MB).
	// JSON documents larger than this use sampling-based security checks.
	DefaultMaxSecuritySize = 10 * 1024 * 1024

	// DefaultMaxObjectKeys is the maximum number of keys in an object (100000).
	// Objects with more keys will be rejected to prevent memory exhaustion.
	DefaultMaxObjectKeys = 100000

	// DefaultMaxArrayElements is the maximum number of elements in an array (100000).
	// Arrays with more elements will be rejected to prevent memory exhaustion.
	DefaultMaxArrayElements = 100000

	// DefaultMaxBatchSize is the maximum number of operations in a batch (2000).
	// Larger batches will be rejected to prevent memory exhaustion.
	DefaultMaxBatchSize = 2000

	// DefaultParallelThreshold is the threshold for parallel processing (10).
	// Operations below this count use sequential processing.
	DefaultParallelThreshold = 10

	// Timing and Intervals - Optimized for responsiveness (internal)
	slowOperationThreshold = 100 * time.Millisecond

	// Processor lifecycle timeouts (internal)
	closeOperationTimeout = 5 * time.Second // Timeout waiting for active operations during Close()

	// maxPathLength is the maximum allowed path string length.
	// Paths longer than this will be rejected for security reasons.
	maxPathLength = internal.MaxPathLength

	// DefaultCacheTTL is the default cache entry time-to-live (5 minutes).
	// Cached values expire after this duration.
	// Adjust via Config.CacheTTL if needed.
	DefaultCacheTTL = 5 * time.Minute

	// Validation constants (internal)
	validationBOMPrefix = "\uFEFF" // UTF-8 BOM prefix to detect and remove
)

// DefaultConfig returns the default configuration.
// Creates a new instance each time to allow modifications without affecting other callers.
func DefaultConfig() Config {
	return Config{
		// Cache Settings
		MaxCacheSize: defaultCacheSize,
		CacheTTL:     DefaultCacheTTL,
		EnableCache:  true,
		CacheResults: true,

		// Size Limits
		MaxJSONSize:  DefaultMaxJSONSize,
		MaxPathDepth: DefaultMaxPathDepth,
		MaxBatchSize: DefaultMaxBatchSize,

		// Security Limits
		MaxNestingDepthSecurity:   DefaultMaxNestingDepth,
		MaxSecurityValidationSize: DefaultMaxSecuritySize,
		MaxObjectKeys:             DefaultMaxObjectKeys,
		MaxArrayElements:          DefaultMaxArrayElements,
		FullSecurityScan:          false,

		// Concurrency
		MaxConcurrency:    DefaultMaxConcurrency,
		ParallelThreshold: DefaultParallelThreshold,

		// Processing Options
		EnableValidation: true,
		StrictMode:       false,
		CreatePaths:      true,
		CleanupNulls:     false,
		CompactArrays:    false,
		ContinueOnError:  false,

		// Input/Output Options
		AllowComments:    false,
		PreserveNumbers:  false,
		ValidateInput:    true,
		ValidateFilePath: true,
		SkipValidation:   false,

		// Encoding Options
		Pretty:          false,
		Indent:          "  ",
		Prefix:          "",
		EscapeHTML:      true,
		SortKeys:        false,
		ValidateUTF8:    true,
		MaxDepth:        DefaultMaxDepth,
		DisallowUnknown: false,
		FloatPrecision:  -1,
		FloatTruncate:   false,
		DisableEscaping: false,
		EscapeUnicode:   false,
		EscapeSlash:     false,
		EscapeNewlines:  true,
		EscapeTabs:      true,
		IncludeNulls:    true,
		CustomEscapes:   nil,

		// Observability
		EnableMetrics:     false,
		EnableHealthCheck: false,

		// Large File Processing
		ChunkSize:       1024 * 1024,       // 1MB chunks
		MaxMemory:       100 * 1024 * 1024, // 100MB max
		BufferSize:      64 * 1024,         // 64KB buffer
		SamplingEnabled: true,
		SampleSize:      1000,

		// JSONL (JSON Lines) Configuration
		JSONLBufferSize:    64 * 1024,         // 64KB buffer
		JSONLMaxLineSize:   1024 * 1024,       // 1MB max line
		JSONLSkipEmpty:     true,              // Skip empty lines by default
		JSONLSkipComments:  false,             // Don't skip comments by default
		JSONLContinueOnErr: false,             // Stop on errors by default
		JSONLWorkers:       4,                 // 4 parallel workers
		JSONLChunkSize:     1000,              // 1000 lines per chunk
		JSONLMaxMemory:     100 * 1024 * 1024, // 100MB max memory

		// Merge Options
		MergeMode: MergeUnion, // Default: union merge
	}
}

// cachedDefaultConfigValue is a package-level cached default config value.
// PERFORMANCE: Avoids DefaultConfig() allocation when no config is provided.
// This is a value type; callers receive a copy and should not mutate the original.
var cachedDefaultConfigValue = DefaultConfig()

// getConfigOrDefault extracts configuration from variadic parameter.
// Internal helper to reduce code duplication across the codebase.
// Returns the first config if provided, otherwise returns a cached default.
// PERFORMANCE: Uses cached value to avoid repeated DefaultConfig() allocation.
func getConfigOrDefault(cfg ...Config) Config {
	if len(cfg) > 0 {
		return cfg[0]
	}
	return cachedDefaultConfigValue
}

// Clone creates a copy of the configuration.
// Performs a deep copy of reference types (maps, slices).
// Returns a pointer to avoid unnecessary copying of the large Config struct.
//
// NOTE: Interface fields (CustomEncoder, CustomPathParser) are shallow-copied
// as they typically contain stateless or singleton implementations.
// CustomTypeEncoders, CustomValidators, AdditionalDangerousPatterns, and Hooks are
// deep-copied as they may be modified independently.
func (c *Config) Clone() *Config {
	if c == nil {
		return nil
	}
	clone := *c

	// Deep copy CustomEscapes map
	if c.CustomEscapes != nil {
		clone.CustomEscapes = make(map[rune]string, len(c.CustomEscapes))
		maps.Copy(clone.CustomEscapes, c.CustomEscapes)
	}

	// Deep copy CustomTypeEncoders map
	if c.CustomTypeEncoders != nil {
		clone.CustomTypeEncoders = make(map[reflect.Type]TypeEncoder, len(c.CustomTypeEncoders))
		maps.Copy(clone.CustomTypeEncoders, c.CustomTypeEncoders)
	}

	// Deep copy CustomValidators slice
	if c.CustomValidators != nil {
		clone.CustomValidators = make([]Validator, len(c.CustomValidators))
		copy(clone.CustomValidators, c.CustomValidators)
	}

	// Deep copy AdditionalDangerousPatterns slice
	if c.AdditionalDangerousPatterns != nil {
		clone.AdditionalDangerousPatterns = make([]DangerousPattern, len(c.AdditionalDangerousPatterns))
		copy(clone.AdditionalDangerousPatterns, c.AdditionalDangerousPatterns)
	}

	// Deep copy Hooks slice
	if c.Hooks != nil {
		clone.Hooks = make([]Hook, len(c.Hooks))
		copy(clone.Hooks, c.Hooks)
	}

	return &clone
}

// Validate validates the configuration and applies corrections.
// IMPORTANT: This method MUTATES the config in place. Invalid values are silently
// clamped to valid ranges. Use ValidateWithWarnings to see what changed.
// Returns nil after corrections are applied — use ValidateWithWarnings for visibility.
func (c *Config) Validate() error {
	if c == nil {
		return errors.New("config cannot be nil")
	}

	c.ValidateWithWarnings()
	return nil
}

// ConfigWarning represents a configuration modification made during validation.
type ConfigWarning struct {
	Field    string // The field that was modified
	OldValue any    // The original value (may be nil for invalid values)
	NewValue any    // The corrected value
	Reason   string // Why the modification was made
}

// ValidateWithWarnings validates the configuration and returns warnings for any
// modifications made. This is useful for debugging configuration issues or
// informing users about automatic adjustments.
//
// Example:
//
//	cfg := json.DefaultConfig()
//	cfg.MaxJSONSize = -1 // Invalid value
//	warnings := cfg.ValidateWithWarnings()
//	for _, w := range warnings {
//	    fmt.Printf("%s: %s\n", w.Field, w.Reason)
//	}
func (c *Config) ValidateWithWarnings() []ConfigWarning {
	if c == nil {
		return []ConfigWarning{{Field: "Config", Reason: "config cannot be nil"}}
	}

	var warnings []ConfigWarning

	// Helper to record clamped int64 values
	// SEMANTIC: Values <= 0 are considered invalid and set to minimum
	// This is appropriate for fields where 0 has no meaningful interpretation
	// (e.g., sizes, counts, limits that must be positive)
	checkInt64Clamp := func(ptr *int64, min, max int64, fieldName string) {
		original := *ptr
		if original <= 0 {
			*ptr = min
			warnings = append(warnings, ConfigWarning{
				Field:    fieldName,
				OldValue: original,
				NewValue: min,
				Reason:   "value was invalid, set to minimum",
			})
		} else if original > max {
			*ptr = max
			warnings = append(warnings, ConfigWarning{
				Field:    fieldName,
				OldValue: original,
				NewValue: max,
				Reason:   "value exceeded maximum",
			})
		}
	}

	// Helper to record clamped int values
	// SEMANTIC: Values <= 0 are considered invalid and set to minimum
	// This is appropriate for fields where 0 has no meaningful interpretation
	checkIntClamp := func(ptr *int, min, max int, fieldName string) {
		original := *ptr
		if original <= 0 {
			*ptr = min
			warnings = append(warnings, ConfigWarning{
				Field:    fieldName,
				OldValue: original,
				NewValue: min,
				Reason:   "value was invalid, set to minimum",
			})
		} else if original > max {
			*ptr = max
			warnings = append(warnings, ConfigWarning{
				Field:    fieldName,
				OldValue: original,
				NewValue: max,
				Reason:   "value exceeded maximum",
			})
		}
	}

	// Size and depth limits
	checkInt64Clamp(&c.MaxJSONSize, 1024*1024, 100*1024*1024, "MaxJSONSize")
	checkIntClamp(&c.MaxPathDepth, 10, 200, "MaxPathDepth")
	checkIntClamp(&c.MaxNestingDepthSecurity, 10, 200, "MaxNestingDepthSecurity")
	checkIntClamp(&c.MaxConcurrency, 1, 200, "MaxConcurrency")
	checkIntClamp(&c.ParallelThreshold, 1, 50, "ParallelThreshold")

	// Security limits
	checkIntClamp(&c.MaxObjectKeys, 100, 100000, "MaxObjectKeys")
	checkIntClamp(&c.MaxArrayElements, 100, 100000, "MaxArrayElements")
	checkInt64Clamp(&c.MaxSecurityValidationSize, 1024*1024, 100*1024*1024, "MaxSecurityValidationSize")

	// Cache settings
	// SEMANTIC: MaxCacheSize uses < 0 check because 0 is valid (means "disabled")
	// This differs from checkIntClamp which treats <= 0 as invalid
	if c.MaxCacheSize < 0 {
		warnings = append(warnings, ConfigWarning{
			Field:    "MaxCacheSize",
			OldValue: c.MaxCacheSize,
			NewValue: 0,
			Reason:   "negative cache size is invalid, disabled caching",
		})
		c.MaxCacheSize = 0
		c.EnableCache = false
	} else if c.MaxCacheSize > 2000 {
		warnings = append(warnings, ConfigWarning{
			Field:    "MaxCacheSize",
			OldValue: c.MaxCacheSize,
			NewValue: 2000,
			Reason:   "cache size exceeded maximum",
		})
		c.MaxCacheSize = 2000
	}

	// SEMANTIC: CacheTTL uses <= 0 check because 0 duration has no meaning
	// A zero TTL would mean "immediately expire" which is not useful
	if c.CacheTTL <= 0 {
		warnings = append(warnings, ConfigWarning{
			Field:    "CacheTTL",
			OldValue: c.CacheTTL,
			NewValue: DefaultCacheTTL,
			Reason:   "invalid TTL, set to default",
		})
		c.CacheTTL = DefaultCacheTTL
	}

	// Encoding options
	// MaxDepth=0 means no limit, positive values set a hard cap, negative is invalid
	if c.MaxDepth < 0 || c.MaxDepth > 1000 {
		warnings = append(warnings, ConfigWarning{
			Field:    "MaxDepth",
			OldValue: c.MaxDepth,
			NewValue: DefaultMaxDepth,
			Reason:   "value out of valid range [0, 1000]",
		})
		c.MaxDepth = DefaultMaxDepth
	}
	// SEMANTIC: FloatPrecision uses -1 as sentinel for "default precision"
	// Range [-1, 15] where -1=auto, 0-15=explicit precision
	if c.FloatPrecision < -1 || c.FloatPrecision > 15 {
		warnings = append(warnings, ConfigWarning{
			Field:    "FloatPrecision",
			OldValue: c.FloatPrecision,
			NewValue: -1,
			Reason:   "value out of valid range [-1, 15]",
		})
		c.FloatPrecision = -1
	}

	// Batch size limits
	checkIntClamp(&c.MaxBatchSize, 10, 10000, "MaxBatchSize")

	// Large file processing limits
	checkInt64Clamp(&c.ChunkSize, 64*1024, 100*1024*1024, "ChunkSize")
	checkInt64Clamp(&c.MaxMemory, 10*1024*1024, 1024*1024*1024, "MaxMemory")
	checkIntClamp(&c.BufferSize, 4*1024, 1024*1024, "BufferSize")
	checkIntClamp(&c.SampleSize, 100, 10000, "SampleSize")

	// JSONL configuration limits
	checkIntClamp(&c.JSONLBufferSize, 4*1024, 1024*1024, "JSONLBufferSize")
	checkIntClamp(&c.JSONLMaxLineSize, 1024, 100*1024*1024, "JSONLMaxLineSize")
	checkIntClamp(&c.JSONLWorkers, 1, 64, "JSONLWorkers")
	checkIntClamp(&c.JSONLChunkSize, 100, 10000, "JSONLChunkSize")
	checkInt64Clamp(&c.JSONLMaxMemory, 10*1024*1024, 1024*1024*1024, "JSONLMaxMemory")

	return warnings
}

// =============================================================================
// API Unification - Config presets for common scenarios
// =============================================================================

// SecurityConfig returns a configuration with enhanced security settings
// for processing untrusted input from external sources.
//
// This is the recommended configuration for:
//   - Public APIs and web services
//   - User-submitted data
//   - External webhooks
//   - Authentication endpoints
//   - Financial data processing
//
// Key characteristics:
//   - Full security scan enabled for all input
//   - Strict mode enabled for predictable parsing
//   - Conservative limits for untrusted payloads
//   - Caching enabled for repeated operations
//
// This function provides a single entry point for security-focused configuration.
func SecurityConfig() Config {
	config := DefaultConfig()
	// Security settings - conservative limits for untrusted input
	config.MaxNestingDepthSecurity = 30
	config.MaxSecurityValidationSize = 10 * 1024 * 1024 // 10MB
	config.MaxObjectKeys = 5000
	config.MaxArrayElements = 5000
	config.MaxJSONSize = 10 * 1024 * 1024 // 10MB max payload
	config.MaxPathDepth = 30
	// Security features enabled
	config.FullSecurityScan = true
	config.StrictMode = true
	config.EnableValidation = true
	// Performance settings
	config.EnableCache = true
	config.MaxCacheSize = 256
	config.CacheTTL = 3 * time.Minute
	return config
}

// =============================================================================
// Unified Config Presets - Use these for common scenarios
// =============================================================================

// PrettyConfig returns a Config for pretty-printed JSON output.
//
// Example:
//
//	result, err := json.Encode(data, json.PrettyConfig())
func PrettyConfig() Config {
	cfg := DefaultConfig()
	cfg.Pretty = true
	cfg.Indent = "  "
	return cfg
}
