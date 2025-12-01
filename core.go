package json

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/cybergodev/json/internal"
)

// Error definitions - using sentinel errors for better performance and type safety
var (
	ErrInvalidJSON       = errors.New("invalid JSON format")
	ErrPathNotFound      = errors.New("path not found")
	ErrTypeMismatch      = errors.New("type mismatch")
	ErrSizeLimit         = errors.New("size limit exceeded")
	ErrOperationFailed   = errors.New("operation failed")
	ErrInvalidPath       = errors.New("invalid path format")
	ErrUnsupportedPath   = errors.New("unsupported path operation")
	ErrProcessorClosed   = errors.New("processor is closed")
	ErrCacheFull         = errors.New("cache is full")
	ErrCacheDisabled     = errors.New("cache is disabled")
	ErrDepthLimit        = errors.New("depth limit exceeded")
	ErrConcurrencyLimit  = errors.New("concurrency limit exceeded")
	ErrSecurityViolation = errors.New("security violation detected")
	ErrRateLimitExceeded = errors.New("rate limit exceeded")
	ErrOperationTimeout  = errors.New("operation timeout")
	ErrCircuitOpen       = errors.New("circuit breaker is open")
	ErrDeadlockDetected  = errors.New("potential deadlock detected")
	ErrResourceExhausted = errors.New("system resources exhausted")
	ErrIteratorControl   = errors.New("iterator control signal")
)

// PropertyAccessResult represents the result of a property access operation
type PropertyAccessResult struct {
	Value  any
	Exists bool
}

// RootDataTypeConversionError represents an error that signals root data type conversion is needed
type RootDataTypeConversionError struct {
	RequiredType string
	RequiredSize int
	CurrentType  string
}

func (e *RootDataTypeConversionError) Error() string {
	return fmt.Sprintf("root data type conversion required: from %s to %s (size: %d)",
		e.CurrentType, e.RequiredType, e.RequiredSize)
}

// ArrayExtensionError represents an error that signals array extension is needed
type ArrayExtensionError struct {
	CurrentLength  int
	RequiredLength int
	TargetIndex    int
	Value          any
	Message        string
	ExtendedArray  []any // For storing pre-created extended arrays
}

func (e *ArrayExtensionError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	return fmt.Sprintf("array extension required: current length %d, required length %d for index %d",
		e.CurrentLength, e.RequiredLength, e.TargetIndex)
}

// PathSegment represents a parsed path segment with its type and value
// This is an alias to the internal PathSegment for backward compatibility
type PathSegment = internal.PathSegment

// PathInfo contains parsed path information
type PathInfo struct {
	Segments     []PathSegment `json:"segments"`
	IsPointer    bool          `json:"is_pointer"`
	OriginalPath string        `json:"original_path"`
}

// JsonsError represents a JSON processing error with enhanced context
type JsonsError struct {
	Op          string         `json:"op"`          // Operation that failed
	Path        string         `json:"path"`        // JSON path where error occurred
	Message     string         `json:"message"`     // Human-readable error message
	Err         error          `json:"err"`         // Underlying error
	Context     map[string]any `json:"context"`     // Additional context information
	Suggestions []string       `json:"suggestions"` // Helpful suggestions for fixing the error
	ErrorCode   string         `json:"error_code"`  // Machine-readable error code
}

func (e *JsonsError) Error() string {
	var msg string
	if e.Path != "" {
		msg = fmt.Sprintf("JSON %s failed at path '%s': %s", e.Op, e.Path, e.Message)
	} else {
		msg = fmt.Sprintf("JSON %s failed: %s", e.Op, e.Message)
	}

	// Add error code if available
	if e.ErrorCode != "" {
		msg = fmt.Sprintf("[%s] %s", e.ErrorCode, msg)
	}

	return msg
}

// ErrorWithSuggestion returns the error message with helpful suggestions
func (e *JsonsError) ErrorWithSuggestion() string {
	baseMsg := e.Error()

	// Use explicit suggestions if available
	if len(e.Suggestions) > 0 {
		suggestions := strings.Join(e.Suggestions, "; ")
		return fmt.Sprintf("%s\nSuggestions: %s", baseMsg, suggestions)
	}

	// Fallback to legacy suggestion logic
	if e.Err != nil {
		switch e.Err {
		case ErrPathNotFound:
			return baseMsg + " (suggestion: check if the path exists or use a default value)"
		case ErrInvalidJSON:
			return baseMsg + " (suggestion: validate JSON format using json.Valid())"
		case ErrTypeMismatch:
			return baseMsg + " (suggestion: use appropriate getter method like GetString(), GetInt(), etc.)"
		case ErrSizeLimit:
			return baseMsg + " (suggestion: increase MaxJSONSize in configuration)"
		case ErrDepthLimit:
			return baseMsg + " (suggestion: increase MaxPathDepth in configuration)"
		case ErrConcurrencyLimit:
			return baseMsg + " (suggestion: increase MaxConcurrency or reduce concurrent operations)"
		case ErrCacheDisabled:
			return baseMsg + " (suggestion: enable cache by setting EnableCache to true in configuration)"
		case ErrRateLimitExceeded:
			return baseMsg + " (suggestion: reduce operation frequency or increase rate limits)"
		}
	}

	return baseMsg
}

// WithContext adds context information to the error
func (e *JsonsError) WithContext(key string, value any) *JsonsError {
	if e.Context == nil {
		e.Context = make(map[string]any)
	}
	e.Context[key] = value
	return e
}

// WithSuggestion adds a helpful suggestion to the error
func (e *JsonsError) WithSuggestion(suggestion string) *JsonsError {
	e.Suggestions = append(e.Suggestions, suggestion)
	return e
}

// WithErrorCode sets a machine-readable error code
func (e *JsonsError) WithErrorCode(code string) *JsonsError {
	e.ErrorCode = code
	return e
}

// GetDetailedInfo returns detailed error information including context
func (e *JsonsError) GetDetailedInfo() map[string]any {
	info := map[string]any{
		"operation":   e.Op,
		"path":        e.Path,
		"message":     e.Message,
		"error_code":  e.ErrorCode,
		"suggestions": e.Suggestions,
	}

	if e.Context != nil {
		info["context"] = e.Context
	}

	if e.Err != nil {
		info["underlying_error"] = e.Err.Error()
	}

	return info
}

// Unwrap returns the underlying error for error wrapping support
func (e *JsonsError) Unwrap() error {
	return e.Err
}

// Is implements error comparison for modern error handling
func (e *JsonsError) Is(target error) bool {
	if target == nil {
		return false
	}

	// Check if target is the same type
	if targetErr, ok := target.(*JsonsError); ok {
		return e.Op == targetErr.Op && e.Err == targetErr.Err
	}

	// Check if the underlying error matches
	return e.Err != nil && errors.Is(e.Err, target)
}

// InvalidUnmarshalError describes an invalid argument passed to Unmarshal.
// (The argument to Unmarshal must be a non-nil pointer.)
type InvalidUnmarshalError struct {
	Type reflect.Type
}

func (e *InvalidUnmarshalError) Error() string {
	if e.Type == nil {
		return "json: Unmarshal(nil)"
	}

	if e.Type.Kind() != reflect.Ptr {
		return "json: Unmarshal(non-pointer " + e.Type.String() + ")"
	}
	return "json: Unmarshal(nil " + e.Type.String() + ")"
}

// SyntaxError is a description of a JSON syntax error.
// Unmarshal will return a SyntaxError if the JSON can't be parsed.
type SyntaxError struct {
	msg    string // description of error
	Offset int64  // error occurred after reading Offset bytes
}

func (e *SyntaxError) Error() string { return e.msg }

// UnmarshalTypeError describes a JSON value that was
// not appropriate for a value of a specific Go type.
type UnmarshalTypeError struct {
	Value  string       // description of JSON value - "bool", "array", "number -5"
	Type   reflect.Type // type of Go value it could not be assigned to
	Offset int64        // error occurred after reading Offset bytes
	Struct string       // name of the root type containing the field
	Field  string       // the full path from root node to the value
	Err    error        // may be nil
}

func (e *UnmarshalTypeError) Error() string {
	if e.Struct != "" || e.Field != "" {
		return "json: cannot unmarshal " + e.Value + " into Go struct field " + e.Struct + "." + e.Field + " of type " + e.Type.String()
	}
	return "json: cannot unmarshal " + e.Value + " into Go value of type " + e.Type.String()
}

func (e *UnmarshalTypeError) Unwrap() error {
	return e.Err
}

// UnsupportedTypeError is returned by Marshal when attempting
// to encode an unsupported value type.
type UnsupportedTypeError struct {
	Type reflect.Type
}

func (e *UnsupportedTypeError) Error() string {
	return "json: unsupported type: " + e.Type.String()
}

// UnsupportedValueError is returned by Marshal when attempting
// to encode an unsupported value.
type UnsupportedValueError struct {
	Value reflect.Value
	Str   string
}

func (e *UnsupportedValueError) Error() string {
	return "json: unsupported value: " + e.Str
}

// MarshalerError represents an error from calling a MarshalJSON or MarshalText method.
type MarshalerError struct {
	Type       reflect.Type
	Err        error
	sourceFunc string
}

func (e *MarshalerError) Error() string {
	srcFunc := e.sourceFunc
	if srcFunc == "" {
		srcFunc = "MarshalJSON"
	}
	return "json: error calling " + srcFunc + " for type " + e.Type.String() + ": " + e.Err.Error()
}

func (e *MarshalerError) Unwrap() error { return e.Err }

// Marshaler is the interface implemented by types that
// can marshal themselves into valid JSON.
type Marshaler interface {
	MarshalJSON() ([]byte, error)
}

// Unmarshaler is the interface implemented by types
// that can unmarshal a JSON description of themselves.
// The input can be assumed to be a valid encoding of
// a JSON value. UnmarshalJSON must copy the JSON data
// if it wishes to retain the data after returning.
//
// By convention, to approximate the behavior of Unmarshal itself,
// Unmarshalers implement UnmarshalJSON([]byte("null")) as a no-op.
type Unmarshaler interface {
	UnmarshalJSON([]byte) error
}

// TextMarshaler is the interface implemented by an object that can
// marshal itself into a textual form.
//
// MarshalText encodes the receiver into UTF-8-encoded text and returns the result.
type TextMarshaler interface {
	MarshalText() (text []byte, err error)
}

// TextUnmarshaler is the interface implemented by an object that can
// unmarshal a textual representation of itself.
//
// UnmarshalText must be able to decode the form generated by MarshalText.
// UnmarshalText must copy the text if it wishes to retain the text
// after returning.
type TextUnmarshaler interface {
	UnmarshalText(text []byte) error
}

// Config holds configuration for the JSON processor
type Config struct {
	// Cache settings
	MaxCacheSize int           `json:"max_cache_size"` // Maximum number of cache entries
	CacheTTL     time.Duration `json:"cache_ttl"`      // Time-to-live for cache entries
	EnableCache  bool          `json:"enable_cache"`   // Whether to enable caching

	// Size limits
	MaxJSONSize  int64 `json:"max_json_size"`  // Maximum JSON size in bytes
	MaxPathDepth int   `json:"max_path_depth"` // Maximum path depth
	MaxBatchSize int   `json:"max_batch_size"` // Maximum batch operation size

	// Security limits (configurable)
	MaxNestingDepthSecurity   int   `json:"max_nesting_depth_security"`   // Maximum nesting depth for security validation (default: 50)
	MaxSecurityValidationSize int64 `json:"max_security_validation_size"` // Maximum size for security validation in bytes (default: 100MB)
	MaxObjectKeys             int   `json:"max_object_keys"`              // Maximum number of keys in JSON objects (default: 10000)
	MaxArrayElements          int   `json:"max_array_elements"`           // Maximum number of elements in arrays (default: 10000)

	// Concurrency settings
	MaxConcurrency    int `json:"max_concurrency"`    // Maximum concurrent operations
	ParallelThreshold int `json:"parallel_threshold"` // Threshold for parallel processing

	// Processing options
	EnableValidation bool `json:"enable_validation"` // Enable input validation
	StrictMode       bool `json:"strict_mode"`       // Enable strict parsing mode
	CreatePaths      bool `json:"create_paths"`      // Automatically create missing paths in Set operations
	CleanupNulls     bool `json:"cleanup_nulls"`     // Remove null values after deletion operations
	CompactArrays    bool `json:"compact_arrays"`    // Remove all null values from arrays (not just trailing)

	// Additional options for interface compatibility
	EnableMetrics       bool `json:"enable_metrics"`      // Enable metrics collection
	EnableHealthCheck   bool `json:"enable_health_check"` // Enable health checking
	AllowCommentsFlag   bool `json:"allow_comments"`      // Allow JSON with comments
	PreserveNumbersFlag bool `json:"preserve_numbers"`    // Preserve number format
	ValidateInput       bool `json:"validate_input"`      // Validate input
	MaxNestingDepth     int  `json:"max_nesting_depth"`   // Maximum nesting depth
	ValidateFilePath    bool `json:"validate_file_path"`  // Validate file paths
}

// ProcessorOptions provides per-operation configuration
type ProcessorOptions struct {
	Context         context.Context `json:"-"`                 // Context for cancellation and timeouts
	CacheResults    bool            `json:"cache_results"`     // Whether to cache results
	StrictMode      bool            `json:"strict_mode"`       // Enable strict mode for this operation
	MaxDepth        int             `json:"max_depth"`         // Maximum recursion depth
	AllowComments   bool            `json:"allow_comments"`    // Allow JSON with comments
	PreserveNumbers bool            `json:"preserve_numbers"`  // Preserve number format (avoid scientific notation)
	CreatePaths     bool            `json:"create_paths"`      // Override global CreatePaths setting for this operation
	CleanupNulls    bool            `json:"cleanup_nulls"`     // Remove null values after deletion operations
	CompactArrays   bool            `json:"compact_arrays"`    // Remove all null values from arrays (not just trailing)
	ContinueOnError bool            `json:"continue_on_error"` // Continue processing other operations even if some fail (for batch operations)
}

// Clone creates a deep copy of ProcessorOptions
func (opts *ProcessorOptions) Clone() *ProcessorOptions {
	if opts == nil {
		return nil
	}
	return &ProcessorOptions{
		Context:         opts.Context,
		CacheResults:    opts.CacheResults,
		StrictMode:      opts.StrictMode,
		MaxDepth:        opts.MaxDepth,
		AllowComments:   opts.AllowComments,
		PreserveNumbers: opts.PreserveNumbers,
		CreatePaths:     opts.CreatePaths,
		CleanupNulls:    opts.CleanupNulls,
		CompactArrays:   opts.CompactArrays,
		ContinueOnError: opts.ContinueOnError,
	}
}

// Stats provides processor performance statistics
type Stats struct {
	CacheSize        int64         `json:"cache_size"`        // Current cache size
	CacheMemory      int64         `json:"cache_memory"`      // Cache memory usage in bytes
	MaxCacheSize     int           `json:"max_cache_size"`    // Maximum cache size
	HitCount         int64         `json:"hit_count"`         // Cache hit count
	MissCount        int64         `json:"miss_count"`        // Cache miss count
	HitRatio         float64       `json:"hit_ratio"`         // Cache hit ratio
	CacheTTL         time.Duration `json:"cache_ttl"`         // Cache TTL
	CacheEnabled     bool          `json:"cache_enabled"`     // Whether cache is enabled
	IsClosed         bool          `json:"is_closed"`         // Whether processor is closed
	MemoryEfficiency float64       `json:"memory_efficiency"` // Memory efficiency (hits per MB)
	OperationCount   int64         `json:"operation_count"`   // Total operations performed
	ErrorCount       int64         `json:"error_count"`       // Total errors encountered
}

// DetailedStats provides comprehensive processor statistics (internal debugging)
type DetailedStats struct {
	Stats             Stats             `json:"stats"`
	state             int32             // Processor state (0=active, 1=closing, 2=closed) - private
	configSnapshot    Config            // Configuration snapshot - private
	resourcePoolStats ResourcePoolStats // Resource pool statistics - private
}

// ResourcePoolStats provides statistics about resource pools
type ResourcePoolStats struct {
	StringBuilderPoolActive bool `json:"string_builder_pool_active"` // Whether string builder pool is active
	PathSegmentPoolActive   bool `json:"path_segment_pool_active"`   // Whether path segment pool is active
}

// CacheStats provides comprehensive cache statistics
type CacheStats struct {
	HitCount         int64        `json:"hit_count"`         // Total cache hits
	MissCount        int64        `json:"miss_count"`        // Total cache misses
	TotalMemory      int64        `json:"total_memory"`      // Total memory usage in bytes
	HitRatio         float64      `json:"hit_ratio"`         // Cache hit ratio
	MemoryEfficiency float64      `json:"memory_efficiency"` // Memory efficiency (hits per MB)
	Evictions        int64        `json:"evictions"`         // Total evictions performed
	ShardCount       int          `json:"shard_count"`       // Number of cache shards
	ShardStats       []ShardStats `json:"shard_stats"`       // Per-shard statistics
}

// ShardStats provides statistics for a single cache shard
type ShardStats struct {
	Size   int64 `json:"size"`   // Number of entries in shard
	Memory int64 `json:"memory"` // Memory usage of shard in bytes
}

// ProcessorMetrics provides comprehensive processor performance metrics
type ProcessorMetrics struct {
	// Operation metrics
	TotalOperations      int64   `json:"total_operations"`
	SuccessfulOperations int64   `json:"successful_operations"`
	FailedOperations     int64   `json:"failed_operations"`
	SuccessRate          float64 `json:"success_rate"`

	// Cache metrics
	CacheHits    int64   `json:"cache_hits"`
	CacheMisses  int64   `json:"cache_misses"`
	CacheHitRate float64 `json:"cache_hit_rate"`

	// Performance metrics
	AverageProcessingTime time.Duration `json:"average_processing_time"`
	MaxProcessingTime     time.Duration `json:"max_processing_time"`
	MinProcessingTime     time.Duration `json:"min_processing_time"`

	// Memory metrics
	TotalMemoryAllocated int64 `json:"total_memory_allocated"`
	PeakMemoryUsage      int64 `json:"peak_memory_usage"`
	CurrentMemoryUsage   int64 `json:"current_memory_usage"`

	// Concurrency metrics
	ActiveConcurrentOps int64 `json:"active_concurrent_ops"`
	MaxConcurrentOps    int64 `json:"max_concurrent_ops"`

	// Runtime metrics (private - internal use only)
	runtimeMemStats runtime.MemStats
	uptime          time.Duration
	errorsByType    map[string]int64
}

// HealthStatus represents the health status of the processor
type HealthStatus struct {
	Timestamp time.Time              `json:"timestamp"`
	Healthy   bool                   `json:"healthy"`
	Checks    map[string]CheckResult `json:"checks"`
}

// CheckResult represents the result of a single health check
type CheckResult struct {
	Healthy bool   `json:"healthy"`
	Message string `json:"message"`
}

// WarmupResult represents the result of a cache warmup operation
type WarmupResult struct {
	TotalPaths  int      `json:"total_paths"`            // Total number of paths processed
	Successful  int      `json:"successful"`             // Number of successfully warmed up paths
	Failed      int      `json:"failed"`                 // Number of failed paths
	SuccessRate float64  `json:"success_rate"`           // Success rate as percentage (0-100)
	FailedPaths []string `json:"failed_paths,omitempty"` // List of paths that failed (optional)
}

// BatchOperation represents a single operation in a batch
type BatchOperation struct {
	Type    string `json:"type"`     // "get", "set", "delete", "validate"
	JSONStr string `json:"json_str"` // JSON string to operate on
	Path    string `json:"path"`     // Path for the operation
	Value   any    `json:"value"`    // Value for set operations
	ID      string `json:"id"`       // Unique identifier for this operation
}

// BatchResult represents the result of a batch operation
type BatchResult struct {
	ID     string `json:"id"`     // Operation ID
	Result any    `json:"result"` // Operation result
	Error  error  `json:"error"`  // Operation error, if any
}

// Schema represents a JSON schema for validation
type Schema struct {
	Type                 string             `json:"type,omitempty"`
	Properties           map[string]*Schema `json:"properties,omitempty"`
	Items                *Schema            `json:"items,omitempty"`
	Required             []string           `json:"required,omitempty"`
	MinLength            int                `json:"minLength,omitempty"`
	MaxLength            int                `json:"maxLength,omitempty"`
	Minimum              float64            `json:"minimum,omitempty"`
	Maximum              float64            `json:"maximum,omitempty"`
	Pattern              string             `json:"pattern,omitempty"`
	Format               string             `json:"format,omitempty"`
	AdditionalProperties bool               `json:"additionalProperties,omitempty"`

	// Enhanced validation options
	MinItems         int     `json:"minItems,omitempty"`         // Minimum array items
	MaxItems         int     `json:"maxItems,omitempty"`         // Maximum array items
	UniqueItems      bool    `json:"uniqueItems,omitempty"`      // Array items must be unique
	Enum             []any   `json:"enum,omitempty"`             // Allowed values
	Const            any     `json:"const,omitempty"`            // Constant value
	MultipleOf       float64 `json:"multipleOf,omitempty"`       // Number must be multiple of this
	ExclusiveMinimum bool    `json:"exclusiveMinimum,omitempty"` // Minimum is exclusive
	ExclusiveMaximum bool    `json:"exclusiveMaximum,omitempty"` // Maximum is exclusive
	Title            string  `json:"title,omitempty"`            // Schema title
	Description      string  `json:"description,omitempty"`      // Schema description
	Default          any     `json:"default,omitempty"`          // Default value
	Examples         []any   `json:"examples,omitempty"`         // Example values

	// Internal flags to track if constraints were explicitly set
	hasMinLength bool
	hasMaxLength bool
	hasMinimum   bool
	hasMaximum   bool
	hasMinItems  bool
	hasMaxItems  bool
}

// ValidationError represents a schema validation error
type ValidationError struct {
	Path    string `json:"path"`
	Message string `json:"message"`
}

func (ve *ValidationError) Error() string {
	if ve.Path != "" {
		return fmt.Sprintf("validation error at path '%s': %s", ve.Path, ve.Message)
	}
	return fmt.Sprintf("validation error: %s", ve.Message)
}

// TypeSafeResult represents a type-safe operation result
type TypeSafeResult[T any] struct {
	Value  T
	Exists bool
	Error  error
}

// Ok returns true if the result is valid (no error and exists)
func (r TypeSafeResult[T]) Ok() bool {
	return r.Error == nil && r.Exists
}

// Unwrap returns the value or zero value if there's an error
// For panic behavior, use UnwrapOrPanic instead
func (r TypeSafeResult[T]) Unwrap() T {
	if r.Error != nil {
		var zero T
		return zero
	}
	return r.Value
}

// UnwrapOrPanic returns the value or panics if there's an error
// Use this only when you're certain the operation succeeded
func (r TypeSafeResult[T]) UnwrapOrPanic() T {
	if r.Error != nil {
		panic(fmt.Sprintf("unwrap called on result with error: %v", r.Error))
	}
	return r.Value
}

// UnwrapOr returns the value or the provided default if there's an error or value doesn't exist
func (r TypeSafeResult[T]) UnwrapOr(defaultValue T) T {
	if r.Error != nil || !r.Exists {
		return defaultValue
	}
	return r.Value
}

// TypeSafeAccessResult represents the result of a type-safe access operation
type TypeSafeAccessResult struct {
	Value  any
	Exists bool
	Type   string
}

// AsString safely converts the result to string
func (r TypeSafeAccessResult) AsString() (string, error) {
	if !r.Exists {
		return "", ErrPathNotFound
	}
	if str, ok := r.Value.(string); ok {
		return str, nil
	}
	return fmt.Sprintf("%v", r.Value), nil
}

// AsInt safely converts the result to int
func (r TypeSafeAccessResult) AsInt() (int, error) {
	if !r.Exists {
		return 0, ErrPathNotFound
	}

	switch v := r.Value.(type) {
	case int:
		return v, nil
	case int64:
		return int(v), nil
	case float64:
		return int(v), nil
	case string:
		return strconv.Atoi(v)
	default:
		return 0, fmt.Errorf("cannot convert %T to int", v)
	}
}

// AsBool safely converts the result to bool
func (r TypeSafeAccessResult) AsBool() (bool, error) {
	if !r.Exists {
		return false, ErrPathNotFound
	}

	switch v := r.Value.(type) {
	case bool:
		return v, nil
	case string:
		return strconv.ParseBool(v)
	default:
		return false, fmt.Errorf("cannot convert %T to bool", v)
	}
}

// EncodeConfig provides advanced encoding configuration (for complex use cases)
type EncodeConfig struct {
	Pretty          bool   `json:"pretty"`
	Indent          string `json:"indent"`
	Prefix          string `json:"prefix"`
	EscapeHTML      bool   `json:"escape_html"`
	SortKeys        bool   `json:"sort_keys"`
	OmitEmpty       bool   `json:"omit_empty"`
	ValidateUTF8    bool   `json:"validate_utf8"`
	MaxDepth        int    `json:"max_depth"`
	DisallowUnknown bool   `json:"disallow_unknown"`

	// Number formatting options
	PreserveNumbers bool `json:"preserve_numbers"` // Preserve original number format and avoid type conversion
	FloatPrecision  int  `json:"float_precision"`  // Precision for float formatting (-1 for automatic)

	// Enhanced character escaping options
	DisableEscaping bool `json:"disable_escaping"` // Disable all character escaping (except quotes and backslashes)
	EscapeUnicode   bool `json:"escape_unicode"`   // Escape Unicode characters to \uXXXX format
	EscapeSlash     bool `json:"escape_slash"`     // Escape forward slashes to \/
	EscapeNewlines  bool `json:"escape_newlines"`  // Escape newlines to \n instead of literal newlines
	EscapeTabs      bool `json:"escape_tabs"`      // Escape tabs to \t instead of literal tabs

	// Null value handling
	IncludeNulls bool `json:"include_nulls"` // Include null values in output (default: true)

	// Custom escape characters
	CustomEscapes map[rune]string `json:"custom_escapes,omitempty"` // Custom character escape mappings
}

// Clone creates a deep copy of the EncodeConfig
func (c *EncodeConfig) Clone() *EncodeConfig {
	if c == nil {
		return DefaultEncodeConfig()
	}

	clone := *c

	// Deep copy CustomEscapes map
	if c.CustomEscapes != nil {
		clone.CustomEscapes = make(map[rune]string, len(c.CustomEscapes))
		for k, v := range c.CustomEscapes {
			clone.CustomEscapes[k] = v
		}
	}

	return &clone
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

// Operation represents the type of operation being performed
type Operation int

const (
	OpGet Operation = iota
	OpSet
	OpDelete
	OpValidate
)

// String returns the string representation of the operation
func (op Operation) String() string {
	switch op {
	case OpGet:
		return "get"
	case OpSet:
		return "set"
	case OpDelete:
		return "delete"
	case OpValidate:
		return "validate"
	default:
		return "unknown"
	}
}

// NavigationResult represents the result of a navigation operation
type NavigationResult struct {
	Value  any
	Exists bool
	Error  error
}

// PathSegmentInfo contains information about a parsed path segment
type PathSegmentInfo struct {
	Type    string
	Value   string
	Key     string
	Index   int
	Start   *int
	End     *int
	Step    *int
	Extract string
	IsFlat  bool
}

// OperationContext contains context information for operations
type OperationContext struct {
	Context     context.Context
	Operation   Operation
	Path        string
	Value       any
	Options     *ProcessorOptions
	StartTime   time.Time
	CreatePaths bool
}

// PathParser interface for parsing path strings
type PathParser interface {
	ParsePath(path string) ([]PathSegmentInfo, error)
	ValidatePath(path string) error
	SplitPathIntoSegments(path string) []PathSegmentInfo
	PreprocessPath(path string) string
}

// Navigator interface for basic navigation operations
type Navigator interface {
	NavigateToPath(data any, segments []PathSegmentInfo) (any, error)
	NavigateToSegment(data any, segment PathSegmentInfo) (NavigationResult, error)
	HandlePropertyAccess(data any, property string) NavigationResult
}

// ArrayOperations interface for array-related operations
type ArrayOperations interface {
	HandleArrayAccess(data any, index int) NavigationResult
	HandleArraySlice(data any, start, end, step int) NavigationResult
	ParseArrayIndex(indexStr string) int
	HandleNegativeIndex(index, length int) int
	ValidateArrayBounds(index, length int) bool
	ExtendArray(arr []any, targetLength int) []any
	SetArrayElement(arr []any, index int, value any) error
}

// ExtractionOperations interface for extraction operations
type ExtractionOperations interface {
	// Basic extraction
	HandleExtraction(data any, key string) (any, error)
	ExtractFromArray(arr []any, key string) []any
	ExtractFromObject(obj map[string]any, key string) (any, bool)

	// Deep extraction
	HandleDeepExtraction(data any, keys []string) (any, error)
	HandleConsecutiveExtractions(data any, segments []PathSegmentInfo) (any, error)
	DetectConsecutiveExtractions(segments []PathSegmentInfo) [][]PathSegmentInfo

	// Mixed operations
	HandleMixedExtractionOperations(data any, segments []PathSegmentInfo) (any, error)

	// Utility functions
	FlattenExtractionResults(data any) []any
	IsExtractionPath(path string) bool
	CountExtractions(path string) int
	ValidateExtractionSyntax(path string) error
	ExtractMultipleKeys(data any, keys []string) (map[string]any, error)
	FilterExtractionResults(data any, predicate func(any) bool) []any
}

// SetOperations interface for value setting operations
type SetOperations interface {
	SetValue(data any, path string, value any, createPaths bool) error
	SetValueWithSegments(data any, segments []PathSegmentInfo, value any, createPaths bool) error
	CreatePath(data any, segments []PathSegmentInfo) error
	HandleTypeConversion(data any, requiredType string) (any, error)
}

// DeleteOperations interface for deletion operations
type DeleteOperations interface {
	DeleteValue(data any, path string) error
	DeleteValueWithSegments(data any, segments []PathSegmentInfo) error
	MarkForDeletion(data any, path string) error
	CleanupDeletedValues(data any) any
	CompactArray(arr []any) []any
}

// ProcessorUtils interface for utility functions
type ProcessorUtils interface {
	IsArrayType(data any) bool
	IsObjectType(data any) bool
	IsEmptyContainer(data any) bool
	DeepCopy(data any) (any, error)
	GetDataType(data any) string
	ConvertToMap(data any) (map[string]any, bool)
	ConvertToArray(data any) ([]any, bool)
	ConvertToString(value any) string
	ConvertToNumber(value any) (float64, error)
}

// ModularProcessor interface that combines all operation interfaces
type ModularProcessor interface {
	// Core operations
	Get(jsonStr, path string, opts ...*ProcessorOptions) (any, error)
	Set(jsonStr, path string, value any, opts ...*ProcessorOptions) (string, error)
	Delete(jsonStr, path string, opts ...*ProcessorOptions) (string, error)

	// Batch operations
	GetMultiple(jsonStr string, paths []string, opts ...*ProcessorOptions) (map[string]any, error)
	BatchProcess(operations []BatchOperation, opts ...*ProcessorOptions) ([]BatchResult, error)

	// Validation
	Valid(jsonStr string, opts ...*ProcessorOptions) (bool, error)

	// Configuration
	SetConfig(config *ProcessorConfig)
	GetConfig() *ProcessorConfig

	// Lifecycle
	Close() error
	IsClosed() bool
}

// ProcessorConfig holds configuration for the processor
type ProcessorConfig struct {
	EnableCache      bool
	EnableMetrics    bool
	MaxConcurrency   int
	Timeout          time.Duration
	RateLimitEnabled bool
	RateLimitRPS     int
	MaxDepth         int
	MaxPathLength    int
}

// DefaultProcessorConfig returns a default configuration
func DefaultProcessorConfig() *ProcessorConfig {
	return &ProcessorConfig{
		EnableCache:      true,
		EnableMetrics:    true,
		MaxConcurrency:   100,
		Timeout:          30 * time.Second,
		RateLimitEnabled: false,
		RateLimitRPS:     1000,
		MaxDepth:         100,
		MaxPathLength:    1000,
	}
}

// CacheKey represents a cache key for operations
type CacheKey struct {
	Operation string
	JSONStr   string
	Path      string
	Options   string
}

// MetricsCollector interface for collecting metrics
type MetricsCollector interface {
	RecordOperation(duration time.Duration, success bool, cacheHit int)
	RecordCacheHit()
	RecordCacheMiss()
	StartConcurrentOperation()
	EndConcurrentOperation()
	GetStats() map[string]any
}

// ProcessorCache interface for caching operations
type ProcessorCache interface {
	Get(key CacheKey) (any, bool)
	Set(key CacheKey, value any, ttl time.Duration)
	Clear()
	Size() int
}

// RateLimiter interface for rate limiting
type RateLimiter interface {
	Allow() bool
	Wait(ctx context.Context) error
	Limit() int
}

// ErrorType represents different types of errors
type ErrorType int

const (
	ErrTypeValidation ErrorType = iota
	ErrTypeNavigation
	ErrTypeConversion
	ErrTypeBoundary
	ErrTypeTimeout
	ErrTypeRateLimit
)

// ProcessorError represents a structured error from the processor
type ProcessorError struct {
	Type      ErrorType
	Operation string
	Path      string
	Message   string
	Cause     error
}

func (e *ProcessorError) Error() string {
	if e.Cause != nil {
		return e.Message + ": " + e.Cause.Error()
	}
	return e.Message
}

func (e *ProcessorError) Unwrap() error {
	return e.Cause
}

// Special marker for deleted values
var DeletedMarker = &struct{ deleted bool }{deleted: true}

// Common error variables for the new architecture
var (
	ErrPathNotFoundNew   = &ProcessorError{Type: ErrTypeNavigation, Message: "path not found"}
	ErrInvalidPathNew    = &ProcessorError{Type: ErrTypeValidation, Message: "invalid path"}
	ErrInvalidJSONNew    = &ProcessorError{Type: ErrTypeValidation, Message: "invalid JSON"}
	ErrIndexOutOfBounds  = &ProcessorError{Type: ErrTypeBoundary, Message: "index out of bounds"}
	ErrTypeConversionNew = &ProcessorError{Type: ErrTypeConversion, Message: "type conversion failed"}
	ErrTimeoutNew        = &ProcessorError{Type: ErrTypeTimeout, Message: "operation timeout"}
	ErrRateLimitNew      = &ProcessorError{Type: ErrTypeRateLimit, Message: "rate limit exceeded"}
)

// DefaultConfig moved to config.go

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
func (c *Config) GetSecurityLimits() map[string]any {
	return map[string]any{
		"max_nesting_depth":            c.MaxNestingDepthSecurity,
		"max_security_validation_size": c.MaxSecurityValidationSize,
		"max_object_keys":              c.MaxObjectKeys,
		"max_array_elements":           c.MaxArrayElements,
		"max_json_size":                c.MaxJSONSize,
		"max_path_depth":               c.MaxPathDepth,
	}
}

// ValidateConfig moved to config.go

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

// DefaultEncodeConfig, NewPrettyConfig, NewCompactConfig moved to config.go

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

// ResourceMonitor provides enhanced resource monitoring and leak detection
type ResourceMonitor struct {
	// Memory statistics
	allocatedBytes  int64 // Total allocated bytes
	freedBytes      int64 // Total freed bytes
	peakMemoryUsage int64 // Peak memory usage

	// Pool statistics
	poolHits      int64 // Pool cache hits
	poolMisses    int64 // Pool cache misses
	poolEvictions int64 // Pool evictions

	// Goroutine tracking
	maxGoroutines     int64 // Maximum goroutines seen
	currentGoroutines int64 // Current goroutine count

	// Leak detection
	lastLeakCheck     int64 // Last leak detection check
	leakCheckInterval int64 // Interval between leak checks (seconds)

	// Performance metrics
	avgResponseTime int64 // Average response time in nanoseconds
	totalOperations int64 // Total operations processed
}

// NewResourceMonitor creates a new resource monitor
func NewResourceMonitor() *ResourceMonitor {
	return &ResourceMonitor{
		leakCheckInterval: 300, // 5 minutes
		lastLeakCheck:     time.Now().Unix(),
	}
}

// RecordAllocation records memory allocation
func (rm *ResourceMonitor) RecordAllocation(bytes int64) {
	atomic.AddInt64(&rm.allocatedBytes, bytes)

	// Update peak memory usage
	current := atomic.LoadInt64(&rm.allocatedBytes) - atomic.LoadInt64(&rm.freedBytes)
	for {
		peak := atomic.LoadInt64(&rm.peakMemoryUsage)
		if current <= peak || atomic.CompareAndSwapInt64(&rm.peakMemoryUsage, peak, current) {
			break
		}
	}
}

// RecordDeallocation records memory deallocation
func (rm *ResourceMonitor) RecordDeallocation(bytes int64) {
	atomic.AddInt64(&rm.freedBytes, bytes)
}

// RecordPoolHit records a pool cache hit
func (rm *ResourceMonitor) RecordPoolHit() {
	atomic.AddInt64(&rm.poolHits, 1)
}

// RecordPoolMiss records a pool cache miss
func (rm *ResourceMonitor) RecordPoolMiss() {
	atomic.AddInt64(&rm.poolMisses, 1)
}

// RecordPoolEviction records a pool eviction
func (rm *ResourceMonitor) RecordPoolEviction() {
	atomic.AddInt64(&rm.poolEvictions, 1)
}

// RecordOperation records an operation with timing
func (rm *ResourceMonitor) RecordOperation(duration time.Duration) {
	atomic.AddInt64(&rm.totalOperations, 1)

	// Update average response time using exponential moving average
	newTime := duration.Nanoseconds()
	for {
		oldAvg := atomic.LoadInt64(&rm.avgResponseTime)
		// Simple exponential moving average with alpha = 0.1
		newAvg := oldAvg + (newTime-oldAvg)/10
		if atomic.CompareAndSwapInt64(&rm.avgResponseTime, oldAvg, newAvg) {
			break
		}
	}
}

// CheckForLeaks performs leak detection and returns potential issues
func (rm *ResourceMonitor) CheckForLeaks() []string {
	now := time.Now().Unix()
	lastCheck := atomic.LoadInt64(&rm.lastLeakCheck)

	if now-lastCheck < rm.leakCheckInterval {
		return nil // Too soon for another check
	}

	if !atomic.CompareAndSwapInt64(&rm.lastLeakCheck, lastCheck, now) {
		return nil // Another goroutine is checking
	}

	var issues []string

	// Check memory growth
	allocated := atomic.LoadInt64(&rm.allocatedBytes)
	freed := atomic.LoadInt64(&rm.freedBytes)
	netMemory := allocated - freed

	if netMemory > 100*1024*1024 { // 100MB threshold
		issues = append(issues, "High memory usage detected")
	}

	// Check goroutine count
	currentGoroutines := int64(runtime.NumGoroutine())
	atomic.StoreInt64(&rm.currentGoroutines, currentGoroutines)

	maxGoroutines := atomic.LoadInt64(&rm.maxGoroutines)
	if currentGoroutines > maxGoroutines {
		atomic.StoreInt64(&rm.maxGoroutines, currentGoroutines)
	}

	if currentGoroutines > 1000 { // High goroutine count
		issues = append(issues, "High goroutine count detected")
	}

	// Check pool efficiency
	hits := atomic.LoadInt64(&rm.poolHits)
	misses := atomic.LoadInt64(&rm.poolMisses)

	if hits+misses > 1000 && hits < misses { // Poor pool hit rate
		issues = append(issues, "Poor pool cache efficiency")
	}

	return issues
}

// GetStats returns current resource statistics
func (rm *ResourceMonitor) GetStats() ResourceStats {
	return ResourceStats{
		AllocatedBytes:    atomic.LoadInt64(&rm.allocatedBytes),
		FreedBytes:        atomic.LoadInt64(&rm.freedBytes),
		PeakMemoryUsage:   atomic.LoadInt64(&rm.peakMemoryUsage),
		PoolHits:          atomic.LoadInt64(&rm.poolHits),
		PoolMisses:        atomic.LoadInt64(&rm.poolMisses),
		PoolEvictions:     atomic.LoadInt64(&rm.poolEvictions),
		MaxGoroutines:     atomic.LoadInt64(&rm.maxGoroutines),
		CurrentGoroutines: atomic.LoadInt64(&rm.currentGoroutines),
		AvgResponseTime:   time.Duration(atomic.LoadInt64(&rm.avgResponseTime)),
		TotalOperations:   atomic.LoadInt64(&rm.totalOperations),
	}
}

// ResourceStats represents resource usage statistics
type ResourceStats struct {
	AllocatedBytes    int64         // Total allocated bytes
	FreedBytes        int64         // Total freed bytes
	PeakMemoryUsage   int64         // Peak memory usage
	PoolHits          int64         // Pool cache hits
	PoolMisses        int64         // Pool cache misses
	PoolEvictions     int64         // Pool evictions
	MaxGoroutines     int64         // Maximum goroutines seen
	CurrentGoroutines int64         // Current goroutine count
	AvgResponseTime   time.Duration // Average response time
	TotalOperations   int64         // Total operations processed
}

// Reset resets all statistics
func (rm *ResourceMonitor) Reset() {
	atomic.StoreInt64(&rm.allocatedBytes, 0)
	atomic.StoreInt64(&rm.freedBytes, 0)
	atomic.StoreInt64(&rm.peakMemoryUsage, 0)
	atomic.StoreInt64(&rm.poolHits, 0)
	atomic.StoreInt64(&rm.poolMisses, 0)
	atomic.StoreInt64(&rm.poolEvictions, 0)
	atomic.StoreInt64(&rm.maxGoroutines, 0)
	atomic.StoreInt64(&rm.currentGoroutines, 0)
	atomic.StoreInt64(&rm.avgResponseTime, 0)
	atomic.StoreInt64(&rm.totalOperations, 0)
	atomic.StoreInt64(&rm.lastLeakCheck, time.Now().Unix())
}

// GetMemoryEfficiency returns memory efficiency as a percentage (0-100)
func (rm *ResourceMonitor) GetMemoryEfficiency() float64 {
	allocated := atomic.LoadInt64(&rm.allocatedBytes)
	freed := atomic.LoadInt64(&rm.freedBytes)

	if allocated == 0 {
		return 100.0
	}

	return float64(freed) / float64(allocated) * 100.0
}

// GetPoolEfficiency returns pool efficiency as a percentage (0-100)
func (rm *ResourceMonitor) GetPoolEfficiency() float64 {
	hits := atomic.LoadInt64(&rm.poolHits)
	misses := atomic.LoadInt64(&rm.poolMisses)
	total := hits + misses

	if total == 0 {
		return 100.0
	}

	return float64(hits) / float64(total) * 100.0
}

// ============================================================================
// Error Helper Functions (from error_helpers.go)
// ============================================================================

// newError creates a standard JsonsError with all fields
func newError(op, path, message string, err error) *JsonsError {
	return &JsonsError{
		Op:      op,
		Path:    path,
		Message: message,
		Err:     err,
	}
}

// newLimitError creates errors for size/depth/count limits
func newLimitError(op string, current, max int64, limitType string) *JsonsError {
	return &JsonsError{
		Op:      op,
		Message: fmt.Sprintf("%s %d exceeds maximum %d", limitType, current, max),
		Err:     ErrSizeLimit, // Generic limit error
	}
}

// Error helper functions moved to errors.go
