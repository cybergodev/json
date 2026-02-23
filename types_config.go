package json

import (
	"context"
	"runtime"
	"time"
)

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
	TotalOperations       int64            `json:"total_operations"`
	SuccessfulOperations  int64            `json:"successful_operations"`
	FailedOperations      int64            `json:"failed_operations"`
	SuccessRate           float64          `json:"success_rate"`
	CacheHits             int64            `json:"cache_hits"`
	CacheMisses           int64            `json:"cache_misses"`
	CacheHitRate          float64          `json:"cache_hit_rate"`
	AverageProcessingTime time.Duration    `json:"average_processing_time"`
	MaxProcessingTime     time.Duration    `json:"max_processing_time"`
	MinProcessingTime     time.Duration    `json:"min_processing_time"`
	TotalMemoryAllocated  int64            `json:"total_memory_allocated"`
	PeakMemoryUsage       int64            `json:"peak_memory_usage"`
	CurrentMemoryUsage    int64            `json:"current_memory_usage"`
	ActiveConcurrentOps   int64            `json:"active_concurrent_ops"`
	MaxConcurrentOps      int64            `json:"max_concurrent_ops"`
	runtimeMemStats       runtime.MemStats `json:"-"`
	uptime                time.Duration    `json:"-"`
	errorsByType          map[string]int64 `json:"-"`
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
