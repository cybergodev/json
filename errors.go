package json

import (
	"errors"
	"fmt"
	"time"
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
	var targetErr *JsonsError
	if errors.As(target, &targetErr) {
		return e.Op == targetErr.Op && e.Err == targetErr.Err
	}

	// Check underlying error
	return errors.Is(e.Err, target)
}

// Error helper functions for creating consistent error messages

// newOperationError creates a JsonsError for operation failures
func newOperationError(operation, message string, err error) error {
	return &JsonsError{Op: operation, Message: message, Err: err}
}

// newPathError creates a JsonsError for path-related errors
func newPathError(path, message string, err error) error {
	return &JsonsError{Op: "path_operation", Path: path, Message: message, Err: err}
}

// newOperationPathError creates a JsonsError with both operation and path context
func newOperationPathError(operation, path, message string, err error) error {
	return &JsonsError{Op: operation, Path: path, Message: message, Err: err}
}

// newSizeLimitError creates a JsonsError for size limit violations
func newSizeLimitError(operation string, actual, limit int64) error {
	return &JsonsError{Op: operation, Message: fmt.Sprintf("size %d exceeds limit %d", actual, limit), Err: ErrSizeLimit}
}

// newDepthLimitError creates a JsonsError for depth limit violations
func newDepthLimitError(operation, path string, actual, limit int) error {
	return &JsonsError{Op: operation, Path: path, Message: fmt.Sprintf("depth %d exceeds limit %d", actual, limit), Err: ErrDepthLimit}
}

// newConcurrencyLimitError creates a JsonsError for concurrency limit violations
func newConcurrencyLimitError(operation string, current, limit int) error {
	return &JsonsError{Op: operation, Message: fmt.Sprintf("concurrent operations %d exceeds limit %d", current, limit), Err: ErrConcurrencyLimit}
}

// newSecurityError creates a security-related error
func newSecurityError(operation, message string) error {
	return &JsonsError{Op: operation, Message: message, Err: ErrSecurityViolation}
}

// newTimeoutError creates a timeout error
func newTimeoutError(operation, path string, duration time.Duration) error {
	return &JsonsError{Op: operation, Path: path, Message: fmt.Sprintf("operation timed out after %v", duration), Err: ErrOperationTimeout}
}

// IsRetryable determines if an error is retryable
func IsRetryable(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, ErrOperationTimeout) || errors.Is(err, ErrConcurrencyLimit) {
		return true
	}
	var jsErr *JsonsError
	if errors.As(err, &jsErr) {
		switch jsErr.Op {
		case "cache_operation", "concurrent_operation":
			return true
		}
	}
	return false
}

// IsSecurityRelated determines if an error is security-related
func IsSecurityRelated(err error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, ErrSecurityViolation)
}

// IsUserError determines if an error is caused by user input
func IsUserError(err error) bool {
	if err == nil {
		return false
	}
	userErrors := []error{
		ErrInvalidJSON, ErrPathNotFound, ErrTypeMismatch,
		ErrInvalidPath, ErrUnsupportedPath,
	}
	for _, userErr := range userErrors {
		if errors.Is(err, userErr) {
			return true
		}
	}
	return false
}

// GetErrorSuggestion provides suggestions for common errors
func GetErrorSuggestion(err error) string {
	if err == nil {
		return ""
	}
	if errors.Is(err, ErrInvalidJSON) {
		return "Check JSON syntax - ensure proper quotes, brackets, and commas"
	}
	if errors.Is(err, ErrPathNotFound) {
		return "Verify the path exists in the JSON structure"
	}
	if errors.Is(err, ErrTypeMismatch) {
		return "Check that the path points to the expected data type"
	}
	if errors.Is(err, ErrInvalidPath) {
		return "Use valid path syntax: 'key.subkey', 'array[0]', or 'object{field}'"
	}
	if errors.Is(err, ErrSizeLimit) {
		return "Reduce JSON size or increase MaxJSONSize in configuration"
	}
	if errors.Is(err, ErrDepthLimit) {
		return "Reduce nesting depth or increase MaxNestingDepth in configuration"
	}
	if errors.Is(err, ErrConcurrencyLimit) {
		return "Reduce concurrent operations or increase MaxConcurrency in configuration"
	}
	if errors.Is(err, ErrSecurityViolation) {
		return "Input contains potentially dangerous patterns - review and sanitize"
	}
	return "Check the error message for specific details"
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
