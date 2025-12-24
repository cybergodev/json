package json

import (
	"errors"
	"fmt"
)

// Core error definitions - simplified and optimized for performance
var (
	// Primary errors for common cases
	ErrInvalidJSON     = errors.New("invalid JSON format")
	ErrPathNotFound    = errors.New("path not found")
	ErrTypeMismatch    = errors.New("type mismatch")
	ErrOperationFailed = errors.New("operation failed")
	ErrInvalidPath     = errors.New("invalid path format")
	ErrProcessorClosed = errors.New("processor is closed")

	// Limit-related errors
	ErrSizeLimit        = errors.New("size limit exceeded")
	ErrDepthLimit       = errors.New("depth limit exceeded")
	ErrConcurrencyLimit = errors.New("concurrency limit exceeded")

	// Security and validation errors
	ErrSecurityViolation = errors.New("security violation detected")
	ErrUnsupportedPath   = errors.New("unsupported path operation")

	// Resource and performance errors
	ErrCacheFull         = errors.New("cache is full")
	ErrCacheDisabled     = errors.New("cache is disabled")
	ErrOperationTimeout  = errors.New("operation timeout")
	ErrResourceExhausted = errors.New("system resources exhausted")

	// Control flow errors (internal use)
	ErrIteratorControl = errors.New("iterator control signal")
)

// JsonsError represents a JSON processing error with essential context
type JsonsError struct {
	Op      string `json:"op"`      // Operation that failed
	Path    string `json:"path"`    // JSON path where error occurred
	Message string `json:"message"` // Human-readable error message
	Err     error  `json:"err"`     // Underlying error
}

func (e *JsonsError) Error() string {
	if e.Path != "" {
		return fmt.Sprintf("JSON %s failed at path '%s': %s", e.Op, e.Path, e.Message)
	}
	return fmt.Sprintf("JSON %s failed: %s", e.Op, e.Message)
}

// Unwrap returns the underlying error for error chain support
func (e *JsonsError) Unwrap() error {
	return e.Err
}

// Is implements error matching for Go 1.13+ error handling
func (e *JsonsError) Is(target error) bool {
	if target == nil {
		return false
	}

	// Check if target is the same type
	if targetErr, ok := target.(*JsonsError); ok {
		return e.Op == targetErr.Op && e.Err == targetErr.Err
	}

	// Check underlying error
	return errors.Is(e.Err, target)
}

// Error helper functions for creating consistent error messages

// newOperationError creates a JsonsError for operation failures
func newOperationError(operation, message string, err error) error {
	return &JsonsError{
		Op:      operation,
		Message: message,
		Err:     err,
	}
}

// newPathError creates a JsonsError for path-related errors
func newPathError(path, message string, err error) error {
	return &JsonsError{
		Op:      "path_validation",
		Path:    path,
		Message: message,
		Err:     err,
	}
}

// newSizeLimitError creates a JsonsError for size limit violations
func newSizeLimitError(operation string, actual, limit int64) error {
	return &JsonsError{
		Op:      operation,
		Message: fmt.Sprintf("size %d exceeds limit %d", actual, limit),
		Err:     ErrSizeLimit,
	}
}

// newDepthLimitError creates a JsonsError for depth limit violations
func newDepthLimitError(operation string, actual, limit int) error {
	return &JsonsError{
		Op:      operation,
		Message: fmt.Sprintf("depth %d exceeds limit %d", actual, limit),
		Err:     ErrDepthLimit,
	}
}

// newSecurityError creates a JsonsError for security violations
func newSecurityError(operation, message string) error {
	return &JsonsError{
		Op:      operation,
		Message: message,
		Err:     ErrSecurityViolation,
	}
}

// ErrorClassifier helps classify errors for better handling
type ErrorClassifier struct{}

// NewErrorClassifier creates a new error classifier
func NewErrorClassifier() *ErrorClassifier {
	return &ErrorClassifier{}
}

// IsRetryable determines if an error is retryable
func (ec *ErrorClassifier) IsRetryable(err error) bool {
	if err == nil {
		return false
	}

	// Check for specific error types that are retryable
	switch {
	case errors.Is(err, ErrOperationTimeout):
		return true
	case errors.Is(err, ErrConcurrencyLimit):
		return true
	case errors.Is(err, ErrResourceExhausted):
		return true
	default:
		return false
	}
}

// IsSecurityError determines if an error is security-related
func (ec *ErrorClassifier) IsSecurityError(err error) bool {
	return errors.Is(err, ErrSecurityViolation)
}

// IsUserError determines if an error is caused by user input
func (ec *ErrorClassifier) IsUserError(err error) bool {
	switch {
	case errors.Is(err, ErrInvalidJSON):
		return true
	case errors.Is(err, ErrInvalidPath):
		return true
	case errors.Is(err, ErrPathNotFound):
		return true
	case errors.Is(err, ErrTypeMismatch):
		return true
	default:
		return false
	}
}

// GetErrorSuggestion provides helpful suggestions for common errors
func (ec *ErrorClassifier) GetErrorSuggestion(err error) string {
	switch {
	case errors.Is(err, ErrInvalidJSON):
		return "Validate JSON format using json.Valid() or check for syntax errors"
	case errors.Is(err, ErrPathNotFound):
		return "Check if the path exists or use a default value with GetWithDefault()"
	case errors.Is(err, ErrTypeMismatch):
		return "Use appropriate getter method like GetString(), GetInt(), etc."
	case errors.Is(err, ErrSizeLimit):
		return "Reduce input size or increase MaxJSONSize in configuration"
	case errors.Is(err, ErrDepthLimit):
		return "Reduce nesting depth or increase MaxNestingDepth in configuration"
	case errors.Is(err, ErrConcurrencyLimit):
		return "Reduce concurrent operations or increase MaxConcurrency"
	case errors.Is(err, ErrSecurityViolation):
		return "Review input for security issues like path traversal attempts"
	default:
		return "Check the error message for specific details"
	}
}

// WrapError wraps an error with additional context
func WrapError(err error, op, message string) error {
	if err == nil {
		return nil
	}
	return &JsonsError{
		Op:      op,
		Message: message,
		Err:     err,
	}
}

// WrapPathError wraps an error with path context
func WrapPathError(err error, op, path, message string) error {
	if err == nil {
		return nil
	}
	return &JsonsError{
		Op:      op,
		Path:    path,
		Message: message,
		Err:     err,
	}
}
