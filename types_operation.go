package json

import (
	"context"
	"fmt"
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

// ValidateOptions validates processor options with enhanced checks
func ValidateOptions(options *ProcessorOptions) error {
	if options == nil {
		return &JsonsError{
			Op:      "validate_options",
			Message: "options cannot be nil",
			Err:     ErrOperationFailed,
		}
	}

	if options.MaxDepth < 0 {
		return &JsonsError{
			Op:      "validate_options",
			Message: fmt.Sprintf("MaxDepth cannot be negative: %d", options.MaxDepth),
			Err:     ErrOperationFailed,
		}
	}
	if options.MaxDepth > 1000 {
		return &JsonsError{
			Op:      "validate_options",
			Message: fmt.Sprintf("MaxDepth too large (max 1000): %d", options.MaxDepth),
			Err:     ErrDepthLimit,
		}
	}

	return nil
}
