package json

import (
	"context"
	"fmt"
	"reflect"
	"runtime"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/cybergodev/json/internal"
)

// PropertyAccessResult represents the result of a property access operation
type PropertyAccessResult struct {
	Value  any
	Exists bool
	Path   string // Path that was accessed
	Error  error
}

// RootDataTypeConversionError signals that root data type conversion is needed
type RootDataTypeConversionError struct {
	RequiredType string
	RequiredSize int
	CurrentType  string
}

func (e *RootDataTypeConversionError) Error() string {
	return fmt.Sprintf("root data type conversion required: from %s to %s (size: %d)",
		e.CurrentType, e.RequiredType, e.RequiredSize)
}

// ArrayExtensionError signals that array extension is needed
type ArrayExtensionError struct {
	CurrentLength  int
	RequiredLength int
	TargetIndex    int
	Value          any
	Message        string
}

func (e *ArrayExtensionError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	return fmt.Sprintf("array extension required: current length %d, required length %d for index %d",
		e.CurrentLength, e.RequiredLength, e.TargetIndex)
}

// PathSegment represents a parsed path segment with its type and value
type PathSegment = internal.PathSegment

// PathInfo contains parsed path information
type PathInfo struct {
	Segments     []PathSegment `json:"segments"`
	IsPointer    bool          `json:"is_pointer"`
	OriginalPath string        `json:"original_path"`
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
	MaxCacheSize int           `json:"max_cache_size"`
	CacheTTL     time.Duration `json:"cache_ttl"`
	EnableCache  bool          `json:"enable_cache"`

	// Size limits
	MaxJSONSize  int64 `json:"max_json_size"`
	MaxPathDepth int   `json:"max_path_depth"`
	MaxBatchSize int   `json:"max_batch_size"`

	// Security limits
	MaxNestingDepthSecurity   int   `json:"max_nesting_depth"`
	MaxSecurityValidationSize int64 `json:"max_security_validation_size"`
	MaxObjectKeys             int   `json:"max_object_keys"`
	MaxArrayElements          int   `json:"max_array_elements"`

	// Concurrency
	MaxConcurrency    int `json:"max_concurrency"`
	ParallelThreshold int `json:"parallel_threshold"`

	// Processing
	EnableValidation bool `json:"enable_validation"`
	StrictMode       bool `json:"strict_mode"`
	CreatePaths      bool `json:"create_paths"`
	CleanupNulls     bool `json:"cleanup_nulls"`
	CompactArrays    bool `json:"compact_arrays"`

	// Additional options
	EnableMetrics     bool `json:"enable_metrics"`
	EnableHealthCheck bool `json:"enable_health_check"`
	AllowComments     bool `json:"allow_comments"`
	PreserveNumbers   bool `json:"preserve_numbers"`
	ValidateInput     bool `json:"validate_input"`
	ValidateFilePath  bool `json:"validate_file_path"`
}

// ProcessorOptions provides per-operation configuration
type ProcessorOptions struct {
	Context         context.Context `json:"-"`
	CacheResults    bool            `json:"cache_results"`
	StrictMode      bool            `json:"strict_mode"`
	MaxDepth        int             `json:"max_depth"`
	AllowComments   bool            `json:"allow_comments"`
	PreserveNumbers bool            `json:"preserve_numbers"`
	CreatePaths     bool            `json:"create_paths"`
	CleanupNulls    bool            `json:"cleanup_nulls"`
	CompactArrays   bool            `json:"compact_arrays"`
	ContinueOnError bool            `json:"continue_on_error"`
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
	CacheSize        int64         `json:"cache_size"`
	CacheMemory      int64         `json:"cache_memory"`
	MaxCacheSize     int           `json:"max_cache_size"`
	HitCount         int64         `json:"hit_count"`
	MissCount        int64         `json:"miss_count"`
	HitRatio         float64       `json:"hit_ratio"`
	CacheTTL         time.Duration `json:"cache_ttl"`
	CacheEnabled     bool          `json:"cache_enabled"`
	IsClosed         bool          `json:"is_closed"`
	MemoryEfficiency float64       `json:"memory_efficiency"`
	OperationCount   int64         `json:"operation_count"`
	ErrorCount       int64         `json:"error_count"`
}

// DetailedStats provides comprehensive processor statistics (internal debugging)
type DetailedStats struct {
	Stats             Stats             `json:"stats"`
	state             int32             `json:"-"` // Processor state (0=active, 1=closing, 2=closed)
	configSnapshot    Config            `json:"config_snapshot"`
	resourcePoolStats ResourcePoolStats `json:"resource_pool_stats"`
}

// ResourcePoolStats provides statistics about resource pools
type ResourcePoolStats struct {
	StringBuilderPoolActive bool `json:"string_builder_pool_active"` // Whether string builder pool is active
	PathSegmentPoolActive   bool `json:"path_segment_pool_active"`   // Whether path segment pool is active
}

// CacheStats provides comprehensive cache statistics
type CacheStats struct {
	HitCount         int64        `json:"hit_count"`
	MissCount        int64        `json:"miss_count"`
	TotalMemory      int64        `json:"total_memory"`
	HitRatio         float64      `json:"hit_ratio"`
	MemoryEfficiency float64      `json:"memory_efficiency"`
	Evictions        int64        `json:"evictions"`
	ShardCount       int          `json:"shard_count"`
	ShardStats       []ShardStats `json:"shard_stats"`
}

// ShardStats provides statistics for a single cache shard
type ShardStats struct {
	Size   int64 `json:"size"`
	Memory int64 `json:"memory"`
}

// ProcessorMetrics provides comprehensive processor performance metrics
type ProcessorMetrics struct {
	TotalOperations      int64   `json:"total_operations"`
	SuccessfulOperations int64   `json:"successful_operations"`
	FailedOperations     int64   `json:"failed_operations"`
	SuccessRate          float64 `json:"success_rate"`
	CacheHits            int64   `json:"cache_hits"`
	CacheMisses          int64   `json:"cache_misses"`
	CacheHitRate         float64 `json:"cache_hit_rate"`
	AverageProcessingTime time.Duration `json:"average_processing_time"`
	MaxProcessingTime     time.Duration `json:"max_processing_time"`
	MinProcessingTime     time.Duration `json:"min_processing_time"`
	TotalMemoryAllocated  int64 `json:"total_memory_allocated"`
	PeakMemoryUsage       int64 `json:"peak_memory_usage"`
	CurrentMemoryUsage    int64 `json:"current_memory_usage"`
	ActiveConcurrentOps   int64 `json:"active_concurrent_ops"`
	MaxConcurrentOps      int64 `json:"max_concurrent_ops"`
	runtimeMemStats       runtime.MemStats `json:"-"`
	uptime                time.Duration     `json:"-"`
	errorsByType          map[string]int64  `json:"-"`
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
	TotalPaths  int      `json:"total_paths"`
	Successful  int      `json:"successful"`
	Failed      int      `json:"failed"`
	SuccessRate float64  `json:"success_rate"`
	FailedPaths []string `json:"failed_paths,omitempty"`
}

// BatchOperation represents a single operation in a batch
type BatchOperation struct {
	Type    string `json:"type"`
	JSONStr string `json:"json_str"`
	Path    string `json:"path"`
	Value   any    `json:"value"`
	ID      string `json:"id"`
}

// BatchResult represents the result of a batch operation
type BatchResult struct {
	ID     string `json:"id"`
	Result any    `json:"result"`
	Error  error  `json:"error"`
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
	MinItems             int                `json:"minItems,omitempty"`
	MaxItems             int                `json:"maxItems,omitempty"`
	UniqueItems          bool               `json:"uniqueItems,omitempty"`
	Enum                 []any              `json:"enum,omitempty"`
	Const                any                `json:"const,omitempty"`
	MultipleOf           float64            `json:"multipleOf,omitempty"`
	ExclusiveMinimum     bool               `json:"exclusiveMinimum,omitempty"`
	ExclusiveMaximum     bool               `json:"exclusiveMaximum,omitempty"`
	Title                string             `json:"title,omitempty"`
	Description          string             `json:"description,omitempty"`
	Default              any                `json:"default,omitempty"`
	Examples             []any              `json:"examples,omitempty"`

	// Internal flags
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
	ValidateUTF8    bool   `json:"validate_utf8"`
	MaxDepth        int    `json:"max_depth"`
	DisallowUnknown bool   `json:"disallow_unknown"`

	// Number formatting
	PreserveNumbers bool `json:"preserve_numbers"`
	FloatPrecision  int  `json:"float_precision"`

	// Character escaping
	DisableEscaping bool `json:"disable_escaping"`
	EscapeUnicode   bool `json:"escape_unicode"`
	EscapeSlash     bool `json:"escape_slash"`
	EscapeNewlines  bool `json:"escape_newlines"`
	EscapeTabs      bool `json:"escape_tabs"`

	// Null handling
	IncludeNulls  bool            `json:"include_nulls"`
	CustomEscapes map[rune]string `json:"custom_escapes,omitempty"`
}

// Clone creates a deep copy of the EncodeConfig
func (c *EncodeConfig) Clone() *EncodeConfig {
	if c == nil {
		return DefaultEncodeConfig()
	}

	clone := *c

	if c.CustomEscapes != nil {
		clone.CustomEscapes = make(map[rune]string, len(c.CustomEscapes))
		for k, v := range c.CustomEscapes {
			clone.CustomEscapes[k] = v
		}
	}

	return &clone
}

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

// CacheKey represents a cache key for operations
type CacheKey struct {
	Operation string
	JSONStr   string
	Path      string
	Options   string
}

// RateLimiter interface for rate limiting (reserved for future use)
type RateLimiter interface {
	Allow() bool
	Wait(ctx context.Context) error
	Limit() int
}

// DeletedMarker is a special marker for deleted values
var DeletedMarker = &struct{ deleted bool }{deleted: true}

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
		AdditionalProperties: true,
		MinItems:             0,
		MaxItems:             0,
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

// NewReadableConfig creates a configuration for human-readable JSON with minimal escaping
func NewReadableConfig() *EncodeConfig {
	return &EncodeConfig{
		Pretty:          false,
		Indent:          "  ",
		Prefix:          "",
		EscapeHTML:      false,
		SortKeys:        false,
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

// ResourceMonitor provides resource monitoring and leak detection
type ResourceMonitor struct {
	allocatedBytes  int64
	freedBytes      int64
	peakMemoryUsage int64
	poolHits        int64
	poolMisses      int64
	poolEvictions   int64
	maxGoroutines    int64
	currentGoroutines int64
	lastLeakCheck    int64
	leakCheckInterval int64
	avgResponseTime   int64
	totalOperations  int64
}

// NewResourceMonitor creates a new resource monitor
func NewResourceMonitor() *ResourceMonitor {
	return &ResourceMonitor{
		leakCheckInterval: 300, // 5 minutes
		lastLeakCheck:     time.Now().Unix(),
	}
}

func (rm *ResourceMonitor) RecordAllocation(bytes int64) {
	atomic.AddInt64(&rm.allocatedBytes, bytes)

	current := atomic.LoadInt64(&rm.allocatedBytes) - atomic.LoadInt64(&rm.freedBytes)
	for {
		peak := atomic.LoadInt64(&rm.peakMemoryUsage)
		if current <= peak || atomic.CompareAndSwapInt64(&rm.peakMemoryUsage, peak, current) {
			break
		}
	}
}

func (rm *ResourceMonitor) RecordDeallocation(bytes int64) {
	atomic.AddInt64(&rm.freedBytes, bytes)
}

func (rm *ResourceMonitor) RecordPoolHit() {
	atomic.AddInt64(&rm.poolHits, 1)
}

func (rm *ResourceMonitor) RecordPoolMiss() {
	atomic.AddInt64(&rm.poolMisses, 1)
}

func (rm *ResourceMonitor) RecordPoolEviction() {
	atomic.AddInt64(&rm.poolEvictions, 1)
}

func (rm *ResourceMonitor) RecordOperation(duration time.Duration) {
	atomic.AddInt64(&rm.totalOperations, 1)

	newTime := duration.Nanoseconds()
	for {
		oldAvg := atomic.LoadInt64(&rm.avgResponseTime)
		newAvg := oldAvg + (newTime-oldAvg)/10
		if atomic.CompareAndSwapInt64(&rm.avgResponseTime, oldAvg, newAvg) {
			break
		}
	}
}

func (rm *ResourceMonitor) CheckForLeaks() []string {
	for {
		now := time.Now().Unix()
		lastCheck := atomic.LoadInt64(&rm.lastLeakCheck)

		if now-lastCheck < rm.leakCheckInterval {
			return nil
		}

		if atomic.CompareAndSwapInt64(&rm.lastLeakCheck, lastCheck, now) {
			break
		}
	}

	var issues []string

	allocated := atomic.LoadInt64(&rm.allocatedBytes)
	freed := atomic.LoadInt64(&rm.freedBytes)
	netMemory := allocated - freed

	if netMemory > 100*1024*1024 {
		issues = append(issues, "High memory usage detected")
	}

	currentGoroutines := int64(runtime.NumGoroutine())
	atomic.StoreInt64(&rm.currentGoroutines, currentGoroutines)

	maxGoroutines := atomic.LoadInt64(&rm.maxGoroutines)
	if currentGoroutines > maxGoroutines {
		atomic.StoreInt64(&rm.maxGoroutines, currentGoroutines)
	}

	if currentGoroutines > 1000 {
		issues = append(issues, "High goroutine count detected")
	}

	hits := atomic.LoadInt64(&rm.poolHits)
	misses := atomic.LoadInt64(&rm.poolMisses)

	if hits+misses > 1000 && hits < misses {
		issues = append(issues, "Poor pool cache efficiency")
	}

	return issues
}

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
	AllocatedBytes    int64         `json:"allocated_bytes"`
	FreedBytes        int64         `json:"freed_bytes"`
	PeakMemoryUsage   int64         `json:"peak_memory_usage"`
	PoolHits          int64         `json:"pool_hits"`
	PoolMisses        int64         `json:"pool_misses"`
	PoolEvictions     int64         `json:"pool_evictions"`
	MaxGoroutines     int64         `json:"max_goroutines"`
	CurrentGoroutines int64         `json:"current_goroutines"`
	AvgResponseTime   time.Duration `json:"avg_response_time"`
	TotalOperations   int64         `json:"total_operations"`
}

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

func (rm *ResourceMonitor) GetMemoryEfficiency() float64 {
	allocated := atomic.LoadInt64(&rm.allocatedBytes)
	freed := atomic.LoadInt64(&rm.freedBytes)

	if allocated == 0 {
		return 100.0
	}

	return float64(freed) / float64(allocated) * 100.0
}

func (rm *ResourceMonitor) GetPoolEfficiency() float64 {
	hits := atomic.LoadInt64(&rm.poolHits)
	misses := atomic.LoadInt64(&rm.poolMisses)
	total := hits + misses

	if total == 0 {
		return 100.0
	}

	return float64(hits) / float64(total) * 100.0
}
