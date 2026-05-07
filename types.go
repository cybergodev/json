package json

import (
	"fmt"
	"reflect"
	"runtime"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/cybergodev/json/internal"
)

// Config holds all configuration for the JSON processor.
// Start with DefaultConfig() and modify as needed.
//
// Example:
//
//	cfg := json.DefaultConfig()
//	cfg.CreatePaths = true
//	cfg.Pretty = true
//	result, err := json.Set(data, "path", value, cfg)
type Config struct {
	// ===== Cache Settings =====
	MaxCacheSize int           `json:"max_cache_size"`
	CacheTTL     time.Duration `json:"cache_ttl"`
	EnableCache  bool          `json:"enable_cache"`
	CacheResults bool          `json:"cache_results"` // Per-operation caching

	// ===== Size Limits =====
	MaxJSONSize  int64 `json:"max_json_size"`
	MaxPathDepth int   `json:"max_path_depth"`
	MaxBatchSize int   `json:"max_batch_size"`

	// ===== Security Limits =====
	MaxNestingDepthSecurity   int   `json:"max_nesting_depth"`
	MaxSecurityValidationSize int64 `json:"max_security_validation_size"`
	MaxObjectKeys             int   `json:"max_object_keys"`
	MaxArrayElements          int   `json:"max_array_elements"`
	// FullSecurityScan enables full (non-sampling) security validation for all JSON input.
	//
	// When false (default): Large JSON (>4KB) uses optimized scanning with:
	//   - Rolling window scan (32KB windows) over the entire JSON content
	//   - Suspicious character density sampling (4KB beginning, middle, and end regions)
	//   - Critical patterns (__proto__, constructor, prototype) always fully scanned
	//   - High suspicious density triggers automatic full scan
	//
	// When true: All JSON is fully scanned regardless of size.
	//
	// SECURITY RECOMMENDATION: Enable FullSecurityScan when:
	//   - Processing untrusted input from external sources
	//   - Handling sensitive data (authentication, financial, personal)
	//   - Building public-facing APIs or web services
	//   - Compliance requirements mandate full content inspection
	//
	// PERFORMANCE NOTE: Full scanning adds ~10-30% overhead for JSON >100KB.
	// For trusted internal services with large JSON payloads, sampling mode is acceptable.
	FullSecurityScan bool `json:"full_security_scan"`

	// ===== Concurrency =====
	MaxConcurrency    int `json:"max_concurrency"`
	ParallelThreshold int `json:"parallel_threshold"`

	// ===== Processing Options =====
	EnableValidation bool `json:"enable_validation"`
	StrictMode       bool `json:"strict_mode"`
	CreatePaths      bool `json:"create_paths"`
	CleanupNulls     bool `json:"cleanup_nulls"`
	CompactArrays    bool `json:"compact_arrays"`
	ContinueOnError  bool `json:"continue_on_error"` // Continue on batch errors

	// ===== Input/Output Options =====
	AllowComments    bool `json:"allow_comments"`
	PreserveNumbers  bool `json:"preserve_numbers"`
	ValidateInput    bool `json:"validate_input"`
	ValidateFilePath bool `json:"validate_file_path"`
	SkipValidation   bool `json:"skip_validation"` // Skip validation for trusted input

	// ===== Encoding Options =====
	Pretty          bool            `json:"pretty"`
	Indent          string          `json:"indent"`
	Prefix          string          `json:"prefix"`
	EscapeHTML      bool            `json:"escape_html"`
	SortKeys        bool            `json:"sort_keys"`
	ValidateUTF8    bool            `json:"validate_utf8"`
	MaxDepth        int             `json:"max_depth"`
	DisallowUnknown bool            `json:"disallow_unknown"`
	FloatPrecision  int             `json:"float_precision"`
	FloatTruncate   bool            `json:"float_truncate"`
	DisableEscaping bool            `json:"disable_escaping"`
	EscapeUnicode   bool            `json:"escape_unicode"`
	EscapeSlash     bool            `json:"escape_slash"`
	EscapeNewlines  bool            `json:"escape_newlines"`
	EscapeTabs      bool            `json:"escape_tabs"`
	IncludeNulls    bool            `json:"include_nulls"`
	CustomEscapes   map[rune]string `json:"custom_escapes,omitempty"`

	// ===== Observability =====
	EnableMetrics     bool `json:"enable_metrics"`
	EnableHealthCheck bool `json:"enable_health_check"`

	// ===== Large File Processing =====
	// ChunkSize is the size of each chunk when processing large files.
	// Default: 1MB (1024 * 1024 bytes)
	ChunkSize int64 `json:"chunk_size"`

	// MaxMemory is the maximum memory to use for large file processing.
	// Default: 100MB (100 * 1024 * 1024 bytes)
	MaxMemory int64 `json:"max_memory"`

	// BufferSize is the buffer size for reading large files.
	// Default: 64KB (64 * 1024 bytes)
	BufferSize int `json:"buffer_size"`

	// SamplingEnabled enables sampling for very large files.
	// When true, only a subset of data is validated for security.
	// Default: true
	SamplingEnabled bool `json:"sampling_enabled"`

	// SampleSize is the number of samples to take when sampling is enabled.
	// Default: 1000
	SampleSize int `json:"sample_size"`

	// ===== JSONL (JSON Lines) Configuration =====
	// These settings control JSONL/NDJSON file processing

	// JSONLBufferSize is the buffer size for reading JSONL files.
	// Default: 64KB (64 * 1024 bytes)
	JSONLBufferSize int `json:"jsonl_buffer_size"`

	// JSONLMaxLineSize is the maximum allowed line size for JSONL files.
	// Default: 1MB (1024 * 1024 bytes)
	JSONLMaxLineSize int `json:"jsonl_max_line_size"`

	// JSONLSkipEmpty skips empty lines when processing JSONL files.
	// Default: true
	JSONLSkipEmpty bool `json:"jsonl_skip_empty"`

	// JSONLSkipComments skips lines starting with # or //.
	// Default: false
	JSONLSkipComments bool `json:"jsonl_skip_comments"`

	// JSONLContinueOnErr continues processing on parse errors.
	// Default: false
	JSONLContinueOnErr bool `json:"jsonl_continue_on_err"`

	// JSONLWorkers is the number of parallel workers for JSONL processing.
	// Default: 4
	JSONLWorkers int `json:"jsonl_workers"`

	// JSONLChunkSize is the chunk size for batched JSONL processing.
	// Default: 1000
	JSONLChunkSize int `json:"jsonl_chunk_size"`

	// JSONLMaxMemory is the maximum memory for JSONL file processing in bytes.
	// Default: 100MB
	JSONLMaxMemory int64 `json:"jsonl_max_memory"`

	// ===== Merge Options =====
	// MergeMode controls how JSON documents are merged by MergeJSON and MergeMany.
	// Default: MergeUnion (combine all keys/elements)
	MergeMode MergeMode `json:"merge_mode"`

	// ===== Extension Points =====

	// CustomEncoder replaces the default encoder entirely.
	// If set, Encode operations use this encoder instead of the built-in one.
	CustomEncoder CustomEncoder

	// CustomTypeEncoders provides encoding for specific types.
	// Keys are reflect.Type values; values implement TypeEncoder.
	CustomTypeEncoders map[reflect.Type]TypeEncoder

	// CustomValidators run before operations.
	// All validators must pass for the operation to proceed.
	CustomValidators []Validator

	// AdditionalDangerousPatterns adds security patterns beyond defaults.
	// These are checked in addition to built-in patterns.
	// SECURITY: Critical patterns (__proto__, constructor[, prototype.) are always
	// enforced and cannot be disabled.
	AdditionalDangerousPatterns []DangerousPattern

	// DisableDefaultPatterns disables built-in warning-level security patterns
	// (HTML tags, event handlers, etc.) but has no effect on critical patterns.
	// SECURITY: Critical patterns (__proto__, constructor[, prototype.) are always
	// enforced regardless of this setting. These patterns are too dangerous to disable.
	DisableDefaultPatterns bool

	// Hooks provide before/after interception for operations.
	Hooks []Hook

	// CustomPathParser replaces the default path parser.
	// If set, path parsing uses this parser instead of the built-in one.
	CustomPathParser PathParser
}

// getSecurityLimits returns a summary of current security limits
func (c *Config) getSecurityLimits() map[string]any {
	if c == nil {
		return map[string]any{}
	}
	return map[string]any{
		"max_nesting_depth":            c.MaxNestingDepthSecurity,
		"max_security_validation_size": c.MaxSecurityValidationSize,
		"max_object_keys":              c.MaxObjectKeys,
		"max_array_elements":           c.MaxArrayElements,
		"max_json_size":                c.MaxJSONSize,
		"max_path_depth":               c.MaxPathDepth,
	}
}

// AddHook adds an operation hook to the configuration.
// Hooks are executed in order for Before and in reverse order for After.
func (c *Config) AddHook(hook Hook) {
	if c == nil {
		return
	}
	c.Hooks = append(c.Hooks, hook)
}

// AddValidator adds a custom validator to the configuration.
// Validators are executed in order; all must pass for operations to proceed.
func (c *Config) AddValidator(validator Validator) {
	if c == nil {
		return
	}
	c.CustomValidators = append(c.CustomValidators, validator)
}

// AddDangerousPattern adds a security pattern to the configuration.
func (c *Config) AddDangerousPattern(pattern DangerousPattern) {
	if c == nil {
		return
	}
	c.AdditionalDangerousPatterns = append(c.AdditionalDangerousPatterns, pattern)
}

// ParsedJSON represents a pre-parsed JSON document that can be reused for multiple operations.
// This is a performance optimization for scenarios where the same JSON is queried multiple times.
// OPTIMIZED: Pre-parsing avoids repeated JSON parsing overhead for repeated queries.
type ParsedJSON struct {
	data      any
	hash      uint64
	processor *Processor
}

// Data returns the underlying parsed data
func (p *ParsedJSON) Data() any {
	if p == nil {
		return nil
	}
	return p.data
}

// Release releases resources held by ParsedJSON.
// After calling Release, Data() returns nil and the processor reference is cleared.
// Call this when finished with a pre-parsed JSON document to allow GC of the processor.
func (p *ParsedJSON) Release() {
	if p == nil {
		return
	}
	p.data = nil
	p.processor = nil
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

// marshaler is the interface implemented by types that
// can marshal themselves into valid JSON.
type marshaler interface {
	MarshalJSON() ([]byte, error)
}

// InvalidUnmarshalError describes an invalid argument passed to Unmarshal.
// (The argument to Unmarshal must be a non-nil pointer.)
type InvalidUnmarshalError struct {
	Type reflect.Type
}

func (e *InvalidUnmarshalError) Error() string {
	if e == nil {
		return "json: Unmarshal(nil)"
	}
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

func (e *SyntaxError) Error() string {
	if e == nil {
		return "json: nil syntax error"
	}
	return e.msg
}

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
	if e == nil {
		return "json: cannot unmarshal into nil target"
	}
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
	if e == nil {
		return "json: unsupported type: nil"
	}
	return "json: unsupported type: " + e.Type.String()
}

// UnsupportedValueError is returned by Marshal when attempting
// to encode an unsupported value.
type UnsupportedValueError struct {
	Value reflect.Value
	Str   string
}

func (e *UnsupportedValueError) Error() string {
	if e == nil {
		return "json: unsupported value: nil"
	}
	return "json: unsupported value: " + e.Str
}

// MarshalerError represents an error from calling a MarshalJSON or MarshalText method.
type MarshalerError struct {
	Type       reflect.Type
	Err        error
	sourceFunc string
}

func (e *MarshalerError) Error() string {
	if e == nil {
		return "json: nil marshaler error"
	}
	srcFunc := e.sourceFunc
	if srcFunc == "" {
		srcFunc = "MarshalJSON"
	}
	return "json: error calling " + srcFunc + " for type " + e.Type.String() + ": " + e.Err.Error()
}

func (e *MarshalerError) Unwrap() error { return e.Err }

// deletedMarker is a special sentinel value used to mark array elements
// for deletion. It is compared by pointer identity (using ==).
//
// SECURITY: This is an unexported struct pointer to prevent external modification.
// The zero-size struct{}{} is used because we only need unique pointer identity.
//
// Naming Convention: Uses camelCase (unexported) to prevent external packages from
// accessing this internal implementation detail. The "Marker" suffix indicates this
// is a sentinel value for marking items, not a data container.
//
// IMPORTANT: Do not reassign this variable. Use IsDeletedMarker() for comparisons.
var deletedMarker = &struct{}{} // deleted marker - empty struct for pointer identity

// isDeletedMarker checks if a value is the deleted marker sentinel.
// This is the internal function for checking deleted markers.
func isDeletedMarker(v any) bool {
	return v == deletedMarker
}

// propertyAccessResult represents the result of a property access operation
type propertyAccessResult struct {
	value  any
	exists bool
}

// ============================================================================
// INTERNAL ERROR TYPES
// These are unexported types used internally for control flow during operations.
// They should not be used directly by external code.
// ============================================================================

// rootDataTypeConversionError signals that root data type conversion is needed
type rootDataTypeConversionError struct {
	requiredType string
	requiredSize int
	currentType  string
}

func (e *rootDataTypeConversionError) Error() string {
	return fmt.Sprintf("root data type conversion required: from %s to %s (size: %d)",
		e.currentType, e.requiredType, e.requiredSize)
}

// pathInfo contains parsed path information.
// This is an internal type used for path parsing.
// Uses internal.PathSegment directly to avoid redundant type alias.
type pathInfo struct {
	segments     []internal.PathSegment
	isPointer    bool
	originalPath string
}

// Resource monitoring thresholds (internal)
const (
	// highMemoryThreshold is the threshold for high memory usage warning (100MB)
	highMemoryThreshold = 100 * 1024 * 1024
	// highGoroutineThreshold is the threshold for high goroutine count warning
	highGoroutineThreshold = 1000
	// minPoolOperationsForEfficiencyCheck is the minimum operations before checking pool efficiency
	minPoolOperationsForEfficiencyCheck = 1000
)

// resourceMonitor provides resource monitoring and leak detection
type resourceMonitor struct {
	allocatedBytes    int64
	freedBytes        int64
	peakMemoryUsage   int64
	poolHits          int64
	poolMisses        int64
	poolEvictions     int64
	maxGoroutines     int64
	currentGoroutines int64
	lastLeakCheck     int64
	leakCheckInterval int64
	avgResponseTime   int64
	totalOperations   int64
}

// newResourceMonitor creates a new resource monitor
func newResourceMonitor() *resourceMonitor {
	return &resourceMonitor{
		leakCheckInterval: 300, // 5 minutes
		lastLeakCheck:     time.Now().Unix(),
	}
}

// RecordAllocation records an allocation of the specified size.
// Note: the peak memory calculation uses a snapshot of allocated/freed counters,
// so it is approximate under high concurrency. This is acceptable for monitoring.
// FIX: Limited CAS retries to prevent unbounded loops under high contention.
// After maxCASRetries, we accept a slightly stale peak value for better throughput.
func (rm *resourceMonitor) recordAllocation(bytes int64) {
	// Atomically update allocation counter
	atomic.AddInt64(&rm.allocatedBytes, bytes)

	// FIX: Limit CAS retries to prevent unbounded loops under high contention
	// This is a reasonable trade-off: we accept slightly stale peak values
	// in exchange for better throughput and avoiding goroutine starvation
	const maxCASRetries = 3

	for i := 0; i < maxCASRetries; i++ {
		allocated := atomic.LoadInt64(&rm.allocatedBytes)
		freed := atomic.LoadInt64(&rm.freedBytes)
		current := allocated - freed

		peak := atomic.LoadInt64(&rm.peakMemoryUsage)
		if current <= peak {
			return // Current is not higher than peak, no update needed
		}

		if atomic.CompareAndSwapInt64(&rm.peakMemoryUsage, peak, current) {
			return // Successfully updated peak
		}
		// CAS failed - another goroutine updated peak, retry with fresh values
	}
	// After maxCASRetries, accept the current peak value
	// This is acceptable for monitoring purposes where exact precision is not critical
}

// RecordDeallocation records a deallocation of the specified size
func (rm *resourceMonitor) recordDeallocation(bytes int64) {
	atomic.AddInt64(&rm.freedBytes, bytes)
}

// RecordPoolHit records a pool cache hit
func (rm *resourceMonitor) recordPoolHit() {
	atomic.AddInt64(&rm.poolHits, 1)
}

// RecordPoolMiss records a pool cache miss
func (rm *resourceMonitor) recordPoolMiss() {
	atomic.AddInt64(&rm.poolMisses, 1)
}

// RecordPoolEviction records a pool eviction
func (rm *resourceMonitor) recordPoolEviction() {
	atomic.AddInt64(&rm.poolEvictions, 1)
}

// RecordOperation records an operation with its duration.
// FIX: Limited CAS retries to prevent unbounded loops under high contention.
// After maxCASRetries, we accept a slightly stale average for better throughput.
func (rm *resourceMonitor) recordOperation(duration time.Duration) {
	atomic.AddInt64(&rm.totalOperations, 1)

	newTime := duration.Nanoseconds()
	const maxCASRetries = 3

	for i := 0; i < maxCASRetries; i++ {
		oldAvg := atomic.LoadInt64(&rm.avgResponseTime)
		newAvg := oldAvg + (newTime-oldAvg)/10
		if atomic.CompareAndSwapInt64(&rm.avgResponseTime, oldAvg, newAvg) {
			return
		}
	}
	// After maxCASRetries, accept slightly stale average — acceptable for monitoring
}

// CheckForLeaks checks for potential resource leaks.
// FIX: Limited CAS retries to prevent unbounded loops under high contention.
func (rm *resourceMonitor) checkForLeaks() []string {
	const maxCASRetries = 3

	// Try to update lastLeakCheck timestamp via CAS; proceed if interval elapsed.
	casSucceeded := false
	for i := 0; i < maxCASRetries; i++ {
		now := time.Now().Unix()
		lastCheck := atomic.LoadInt64(&rm.lastLeakCheck)

		if now-lastCheck < rm.leakCheckInterval {
			return nil
		}

		if atomic.CompareAndSwapInt64(&rm.lastLeakCheck, lastCheck, now) {
			casSucceeded = true
			break
		}
		// CAS failed - another goroutine updated, retry with fresh values
	}
	// After maxCASRetries without successful CAS, another goroutine won and will run the check.
	if !casSucceeded {
		return nil
	}

	var issues []string

	allocated := atomic.LoadInt64(&rm.allocatedBytes)
	freed := atomic.LoadInt64(&rm.freedBytes)
	netMemory := allocated - freed

	if netMemory > highMemoryThreshold {
		issues = append(issues, "High memory usage detected")
	}

	currentGoroutines := int64(runtime.NumGoroutine())
	atomic.StoreInt64(&rm.currentGoroutines, currentGoroutines)

	maxGoroutines := atomic.LoadInt64(&rm.maxGoroutines)
	if currentGoroutines > maxGoroutines {
		atomic.StoreInt64(&rm.maxGoroutines, currentGoroutines)
	}

	if currentGoroutines > highGoroutineThreshold {
		issues = append(issues, "High goroutine count detected")
	}

	hits := atomic.LoadInt64(&rm.poolHits)
	misses := atomic.LoadInt64(&rm.poolMisses)

	if hits+misses > minPoolOperationsForEfficiencyCheck && hits < misses {
		issues = append(issues, "Poor pool cache efficiency")
	}

	return issues
}

// Reset resets all resource statistics
func (rm *resourceMonitor) reset() {
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

// Schema represents a JSON schema for validation.
// Supports a subset of JSON Schema Draft 7 for validating JSON structures.
//
// Example:
//
//	schema := &json.Schema{
//	    Type:     "object",
//	    Required: []string{"name", "email"},
//	    Properties: map[string]*json.Schema{
//	        "name":  {Type: "string", MinLength: 1},
//	        "email": {Type: "string", Format: "email"},
//	        "age":   {Type: "integer", Minimum: 0},
//	    },
//	}
//	err := processor.ValidateSchema(jsonStr, schema)
type Schema struct {
	// Type specifies the JSON type: "object", "array", "string", "number", "integer", "boolean", "null".
	Type string `json:"type,omitempty"`

	// Properties defines the schema for each property when Type is "object".
	Properties map[string]*Schema `json:"properties,omitempty"`

	// Items defines the schema for array elements when Type is "array".
	Items *Schema `json:"items,omitempty"`

	// Required lists property names that must be present (for objects).
	Required []string `json:"required,omitempty"`

	// MinLength is the minimum string length (for strings).
	MinLength int `json:"minLength,omitempty"`

	// MaxLength is the maximum string length (for strings).
	MaxLength int `json:"maxLength,omitempty"`

	// Minimum is the minimum numeric value (for numbers/integers).
	Minimum float64 `json:"minimum,omitempty"`

	// Maximum is the maximum numeric value (for numbers/integers).
	Maximum float64 `json:"maximum,omitempty"`

	// Pattern is a regex pattern that the string must match (for strings).
	Pattern string `json:"pattern,omitempty"`

	// Format specifies a semantic format: "email", "uri", "date", "date-time", etc.
	Format string `json:"format,omitempty"`

	// AdditionalProperties controls whether extra properties are allowed (for objects).
	AdditionalProperties bool `json:"additionalProperties,omitempty"`

	// MinItems is the minimum number of items (for arrays).
	MinItems int `json:"minItems,omitempty"`

	// MaxItems is the maximum number of items (for arrays).
	MaxItems int `json:"maxItems,omitempty"`

	// UniqueItems requires all array elements to be unique (for arrays).
	UniqueItems bool `json:"uniqueItems,omitempty"`

	// Enum restricts the value to one of the specified values.
	Enum []any `json:"enum,omitempty"`

	// Const requires the value to equal this exact value.
	Const any `json:"const,omitempty"`

	// MultipleOf requires the value to be a multiple of this number.
	MultipleOf float64 `json:"multipleOf,omitempty"`

	// ExclusiveMinimum excludes the minimum value itself (for numbers).
	ExclusiveMinimum bool `json:"exclusiveMinimum,omitempty"`

	// ExclusiveMaximum excludes the maximum value itself (for numbers).
	ExclusiveMaximum bool `json:"exclusiveMaximum,omitempty"`

	// Title is a human-readable title for the schema.
	Title string `json:"title,omitempty"`

	// Description is a human-readable description of the schema.
	Description string `json:"description,omitempty"`

	// Default is the default value for the property.
	Default any `json:"default,omitempty"`

	// Examples provides example values for documentation.
	Examples []any `json:"examples,omitempty"`

	// Internal flags for tracking which constraints are explicitly set
	hasMinLength bool
	hasMaxLength bool
	hasMinimum   bool
	hasMaximum   bool
	hasMinItems  bool
	hasMaxItems  bool

	// compiledPattern is the lazily pre-compiled regex for Pattern.
	// Prevents recompilation on every validateString call (ReDoS mitigation).
	compiledPattern any
	patternCompiled bool
}

// ValidationError represents a schema validation error.
// It includes the path where the error occurred and a descriptive message.
//
// Example:
//
//	err := processor.ValidateSchema(jsonStr, schema)
//	if err != nil {
//	    var valErr *json.ValidationError
//	    if errors.As(err, &valErr) {
//	        fmt.Printf("Error at %s: %s\n", valErr.Path, valErr.Message)
//	    }
//	}
type ValidationError struct {
	// Path is the JSON path where the validation error occurred.
	Path string `json:"path"`
	// Message describes the validation failure.
	Message string `json:"message"`
}

func (ve *ValidationError) Error() string {
	if ve.Path != "" {
		return fmt.Sprintf("validation error at path '%s': %s", ve.Path, ve.Message)
	}
	return fmt.Sprintf("validation error: %s", ve.Message)
}

// =============================================================================
// Unified Result Type - Result[T]
// =============================================================================

// Result represents a type-safe operation result with comprehensive error handling.
// This is the unified type for all type-safe operations.
//
// Example:
//
//	result := json.GetResult[string](data, "user.name")
//	if result.Ok() {
//	    name := result.Unwrap()
//	}
//	// Or with default
//	name := json.GetResult[string](data, "user.name").UnwrapOr("unknown")
type Result[T any] struct {
	Value  T     // The result value (exported for backward compatibility)
	Exists bool  // Whether the path exists
	Error  error // Error if any
}

// Ok returns true if the result is valid (no error and exists).
func (r Result[T]) Ok() bool { return r.Error == nil && r.Exists }

// Unwrap returns the value or zero value if there's an error or value doesn't exist.
// For panic behavior, use Must() instead.
func (r Result[T]) Unwrap() T {
	if r.Error != nil || !r.Exists {
		var zero T
		return zero
	}
	return r.Value
}

// UnwrapOr returns the value or the provided default if there's an error or value doesn't exist.
func (r Result[T]) UnwrapOr(defaultValue T) T {
	if r.Error != nil || !r.Exists {
		return defaultValue
	}
	return r.Value
}

// =============================================================================
// AccessResult - For dynamic type access with conversion methods
// =============================================================================

// AccessResult represents the result of a dynamic access operation.
// It extends Result[any] with type conversion methods for safe type handling.
//
// Example:
//
//	result := processor.SafeGet(data, "user.age")
//	age, err := result.AsInt()
//	name, err := result.AsString()
type AccessResult struct {
	Value  any    // The result value (exported for backward compatibility)
	Exists bool   // Whether the path exists
	Type   string // Runtime type info (for debugging)
}

// Ok returns true if the value exists.
func (r AccessResult) Ok() bool { return r.Exists }

// Unwrap returns the value or nil if it doesn't exist.
func (r AccessResult) Unwrap() any {
	if !r.Exists {
		return nil
	}
	return r.Value
}

// UnwrapOr returns the value or the provided default if it doesn't exist.
func (r AccessResult) UnwrapOr(defaultValue any) any {
	if !r.Exists {
		return defaultValue
	}
	return r.Value
}

// AsString safely converts the result to string.
// Returns ErrTypeMismatch if the value is not a string type.
// Use AsStringConverted() for explicit type conversion with formatting.
func (r AccessResult) AsString() (string, error) {
	if !r.Exists {
		return "", ErrPathNotFound
	}
	if str, ok := r.Value.(string); ok {
		return str, nil
	}
	// SECURITY: Return error for non-string types instead of silent conversion
	return "", fmt.Errorf("cannot convert %T to string: type mismatch", r.Value)
}

// AsStringConverted converts the result to string using fmt.Sprintf formatting.
// Use this when you explicitly want string representation of any type.
// For strict type checking, use AsString() instead.
func (r AccessResult) AsStringConverted() (string, error) {
	if !r.Exists {
		return "", ErrPathNotFound
	}
	if str, ok := r.Value.(string); ok {
		return str, nil
	}
	// Explicit conversion requested - use formatting
	return fmt.Sprintf("%v", r.Value), nil
}

// AsInt safely converts the result to int with overflow and precision checks.
// Unlike convertToInt, this method is stricter and does NOT convert bool to int.
// Use convertToInt directly if you need more permissive conversion.
func (r AccessResult) AsInt() (int, error) {
	if !r.Exists {
		return 0, ErrPathNotFound
	}

	// Strict type check: bool should not convert to int
	switch r.Value.(type) {
	case bool:
		return 0, fmt.Errorf("cannot convert bool to int")
	}

	result, ok := convertToInt(r.Value)
	if !ok {
		return 0, fmt.Errorf("cannot convert %T to int", r.Value)
	}
	return result, nil
}

// AsFloat64 safely converts the result to float64 with precision checks.
// Unlike convertToFloat64, this method is stricter and does NOT convert bool to float64.
// Use convertToFloat64 directly if you need more permissive conversion.
func (r AccessResult) AsFloat64() (float64, error) {
	if !r.Exists {
		return 0, ErrPathNotFound
	}

	// Strict type check: bool should not convert to float64
	switch r.Value.(type) {
	case bool:
		return 0, fmt.Errorf("cannot convert bool to float64")
	}

	result, ok := convertToFloat64(r.Value)
	if !ok {
		return 0, fmt.Errorf("cannot convert %T to float64", r.Value)
	}
	return result, nil
}

// AsBool safely converts the result to bool.
// Unlike ConvertToBool, this method is stricter and only accepts bool and string types.
// Use ConvertToBool directly if you need more permissive conversion (e.g., int to bool).
func (r AccessResult) AsBool() (bool, error) {
	if !r.Exists {
		return false, ErrPathNotFound
	}

	switch v := r.Value.(type) {
	case bool:
		return v, nil
	case string:
		result, err := strconv.ParseBool(v)
		if err != nil {
			return false, fmt.Errorf("cannot convert %q to bool: %w", v, err)
		}
		return result, nil
	default:
		return false, fmt.Errorf("cannot convert %T to bool", r.Value)
	}
}

// DefaultSchema returns a default schema configuration.
// All zero values are omitted for brevity; only non-zero defaults are set.
func DefaultSchema() *Schema {
	return &Schema{
		Properties:           make(map[string]*Schema),
		Required:             []string{},
		AdditionalProperties: true,
	}
}

// SchemaConfig provides configuration options for creating a Schema.
// This follows the Config pattern as required by the design guidelines.
type SchemaConfig struct {
	Type                 string
	Properties           map[string]*Schema
	Items                *Schema
	Required             []string
	MinLength            *int
	MaxLength            *int
	Minimum              *float64
	Maximum              *float64
	Pattern              string
	Format               string
	AdditionalProperties *bool
	MinItems             *int
	MaxItems             *int
	UniqueItems          bool
	Enum                 []any
	Const                any
	MultipleOf           *float64
	ExclusiveMinimum     *bool
	ExclusiveMaximum     *bool
	Title                string
	Description          string
	Default              any
	Examples             []any
}

// DefaultSchemaConfig returns the default configuration for creating a Schema.
// This follows the unified Config pattern as required by the design guidelines.
//
// Example:
//
//	cfg := json.DefaultSchemaConfig()
//	cfg.Type = "object"
//	cfg.Required = []string{"name", "email"}
//	schema := json.NewSchemaWithConfig(cfg)
func DefaultSchemaConfig() SchemaConfig {
	return SchemaConfig{
		AdditionalProperties: ptrBool(true),
	}
}

// ptrBool returns a pointer to a bool value.
// This is a helper function for SchemaConfig optional fields.
func ptrBool(v bool) *bool { return &v }

// NewSchemaWithConfig creates a new Schema with the provided configuration.
// This is the recommended way to create configured Schema instances.
func NewSchemaWithConfig(cfg SchemaConfig) *Schema {
	s := &Schema{
		Type:        cfg.Type,
		Properties:  cfg.Properties,
		Items:       cfg.Items,
		Required:    cfg.Required,
		Pattern:     cfg.Pattern,
		Format:      cfg.Format,
		UniqueItems: cfg.UniqueItems,
		Enum:        cfg.Enum,
		Const:       cfg.Const,
		Title:       cfg.Title,
		Description: cfg.Description,
		Default:     cfg.Default,
		Examples:    cfg.Examples,
	}

	if cfg.Properties == nil {
		s.Properties = make(map[string]*Schema)
	}
	if cfg.Required == nil {
		s.Required = []string{}
	}

	// Set optional fields with their has* flags
	if cfg.MinLength != nil {
		s.MinLength = *cfg.MinLength
		s.hasMinLength = true
	}
	if cfg.MaxLength != nil {
		s.MaxLength = *cfg.MaxLength
		s.hasMaxLength = true
	}
	if cfg.Minimum != nil {
		s.Minimum = *cfg.Minimum
		s.hasMinimum = true
	}
	if cfg.Maximum != nil {
		s.Maximum = *cfg.Maximum
		s.hasMaximum = true
	}
	if cfg.AdditionalProperties != nil {
		s.AdditionalProperties = *cfg.AdditionalProperties
	} else {
		s.AdditionalProperties = true
	}
	if cfg.MinItems != nil {
		s.MinItems = *cfg.MinItems
		s.hasMinItems = true
	}
	if cfg.MaxItems != nil {
		s.MaxItems = *cfg.MaxItems
		s.hasMaxItems = true
	}
	if cfg.MultipleOf != nil {
		s.MultipleOf = *cfg.MultipleOf
	}
	if cfg.ExclusiveMinimum != nil {
		s.ExclusiveMinimum = *cfg.ExclusiveMinimum
	}
	if cfg.ExclusiveMaximum != nil {
		s.ExclusiveMaximum = *cfg.ExclusiveMaximum
	}

	return s
}

// ============================================================================
// COMPILED PATH
// Pre-parsed JSON path for fast repeated operations.
// ============================================================================

// CompiledPath represents a pre-parsed JSON path for fast repeated operations.
// Use Processor.CompilePath() to create instances.
//
// Example:
//
//	cp, err := processor.CompilePath("users[0].name")
//	defer cp.Release()
//	value, err := processor.GetCompiled(jsonStr, cp)
type CompiledPath = internal.CompiledPath

// ============================================================================
// MERGE CONFIGURATION
// Configuration types for JSON merge operations with union/intersection/difference modes.
// ============================================================================

// MergeMode defines the merge strategy for combining JSON objects and arrays.
// This is a type alias to internal.MergeMode to ensure consistency across the codebase.
type MergeMode = internal.MergeMode

// Merge mode constants - re-exported from internal package for public API
const (
	// MergeUnion performs union merge - combines all keys/elements (default)
	// For objects: all keys from both, conflicts resolved by override value
	// For arrays: all elements from both with deduplication
	MergeUnion = internal.MergeUnion

	// MergeIntersection performs intersection merge - only common keys/elements
	// For objects: only keys present in both, values from override
	// For arrays: only elements present in both arrays
	MergeIntersection = internal.MergeIntersection

	// MergeDifference performs difference merge - keys/elements only in base
	// For objects: keys in base but not in override
	// For arrays: elements in base but not in override
	MergeDifference = internal.MergeDifference
)
