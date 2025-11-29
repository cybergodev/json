package json

import "fmt"

// Unified error creation functions - reduced from 6 to 3 functions

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

// newSecurityError creates security violation errors
func newSecurityError(op, message string) *JsonsError {
	return &JsonsError{
		Op:      op,
		Message: message,
		Err:     ErrSecurityViolation,
	}
}

// Legacy compatibility wrappers (inline, no function call overhead)
func newValidationError(op, path, message string, err error) *JsonsError {
	return newError(op, path, message, err)
}

func newOperationError(op, message string, err error) *JsonsError {
	return newError(op, "", message, err)
}

func newPathError(path, message string, err error) *JsonsError {
	return newError("validate_path", path, message, err)
}

func newSizeLimitError(op string, size, maxSize int64) *JsonsError {
	return newLimitError(op, size, maxSize, "size")
}

func newDepthLimitError(op string, depth, maxDepth int) *JsonsError {
	return newLimitError(op, int64(depth), int64(maxDepth), "depth")
}
