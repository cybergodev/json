package json

import "fmt"

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
