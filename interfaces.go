package json

import (
	"context"
	"time"
)

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
	NeedsLegacyHandling(path string) bool
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
