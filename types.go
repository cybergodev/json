package json

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"runtime"
	"strconv"
	"strings"
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

// =============================================================================
// ENCODING/JSON COMPATIBILITY ERROR TYPES
// =============================================================================

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

// =============================================================================
// STANDARD INTERFACES FOR ENCODING/JSON COMPATIBILITY
// =============================================================================

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
	EnableMetrics       bool          `json:"enable_metrics"`        // Enable metrics collection
	EnableHealthCheck   bool          `json:"enable_health_check"`   // Enable health checking
	AllowCommentsFlag   bool          `json:"allow_comments"`        // Allow JSON with comments
	PreserveNumbersFlag bool          `json:"preserve_numbers"`      // Preserve number format
	ValidateInput       bool          `json:"validate_input"`        // Validate input
	MaxNestingDepth     int           `json:"max_nesting_depth"`     // Maximum nesting depth
	EnableRateLimit     bool          `json:"enable_rate_limit"`     // Enable rate limiting
	RateLimitPerSec     int           `json:"rate_limit_per_sec"`    // Rate limit per second
	ValidateFilePath    bool          `json:"validate_file_path"`    // Validate file paths
	EnableResourcePools bool          `json:"enable_resource_pools"` // Enable resource pools
	MaxPoolSize         int           `json:"max_pool_size"`         // Maximum pool size
	PoolCleanupInterval time.Duration `json:"pool_cleanup_interval"` // Pool cleanup interval
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

// cacheEntry represents a cached value with metadata
// type cacheEntry struct {
// 	data      any       // The cached data
// 	timestamp time.Time // When the entry was created
// 	hits      int64     // Number of times this entry was accessed (atomic)
// 	size      int32     // Estimated size in bytes
// }
//
// // isExpired checks if the cache entry has expired
// func (e *cacheEntry) isExpired(ttl time.Duration) bool {
// 	return time.Since(e.timestamp) > ttl
// }
//
// // incrementHits atomically increments the hit counter
// func (e *cacheEntry) incrementHits() {
// 	atomic.AddInt64(&e.hits, 1)
// }
//
// // getHits atomically gets the hit count
// func (e *cacheEntry) getHits() int64 {
// 	return atomic.LoadInt64(&e.hits)
// }

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
	state             int32             `json:"state"`               // Processor state (0=active, 1=closing, 2=closed) - private
	configSnapshot    Config            `json:"config_snapshot"`     // Configuration snapshot - private
	resourcePoolStats ResourcePoolStats `json:"resource_pool_stats"` // Resource pool statistics - private
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
	runtimeMemStats runtime.MemStats `json:"runtime_mem_stats"`
	uptime          time.Duration    `json:"uptime"`
	errorsByType    map[string]int64 `json:"errors_by_type"`
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

// Unwrap returns the value or panics if there's an error
func (r TypeSafeResult[T]) Unwrap() T {
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

func (c *Config) IsCacheEnabled() bool                  { return c.EnableCache }
func (c *Config) GetMaxCacheSize() int                  { return c.MaxCacheSize }
func (c *Config) GetCacheTTL() time.Duration            { return c.CacheTTL }
func (c *Config) GetMaxJSONSize() int64                 { return c.MaxJSONSize }
func (c *Config) GetMaxPathDepth() int                  { return c.MaxPathDepth }
func (c *Config) GetMaxConcurrency() int                { return c.MaxConcurrency }
func (c *Config) IsMetricsEnabled() bool                { return c.EnableMetrics }
func (c *Config) IsHealthCheckEnabled() bool            { return c.EnableHealthCheck }
func (c *Config) IsStrictMode() bool                    { return c.StrictMode }
func (c *Config) AllowComments() bool                   { return c.AllowCommentsFlag }
func (c *Config) PreserveNumbers() bool                 { return c.PreserveNumbersFlag }
func (c *Config) ShouldCreatePaths() bool               { return c.CreatePaths }
func (c *Config) ShouldCleanupNulls() bool              { return c.CleanupNulls }
func (c *Config) ShouldCompactArrays() bool             { return c.CompactArrays }
func (c *Config) ShouldValidateInput() bool             { return c.ValidateInput }
func (c *Config) GetMaxNestingDepth() int               { return c.MaxNestingDepth }
func (c *Config) IsRateLimitEnabled() bool              { return c.EnableRateLimit }
func (c *Config) GetRateLimitPerSec() int               { return c.RateLimitPerSec }
func (c *Config) ShouldValidateFilePath() bool          { return c.ValidateFilePath }
func (c *Config) AreResourcePoolsEnabled() bool         { return c.EnableResourcePools }
func (c *Config) GetMaxPoolSize() int                   { return c.MaxPoolSize }
func (c *Config) GetPoolCleanupInterval() time.Duration { return c.PoolCleanupInterval }
