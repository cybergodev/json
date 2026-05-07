package json

import (
	"errors"
	"fmt"
)

// Primary errors for common cases.
var (
	// ErrInvalidJSON indicates that the input is not valid JSON.
	// This error is returned when JSON parsing fails due to syntax errors,
	// malformed structures, or invalid UTF-8 encoding.
	ErrInvalidJSON = errors.New("invalid JSON format")

	// ErrPathNotFound indicates that the specified path does not exist
	// in the JSON structure. This can occur when accessing nested keys
	// that don't exist or using array indices out of bounds.
	ErrPathNotFound = errors.New("path not found")

	// ErrTypeMismatch indicates that the value at the path is not of the expected type.
	// For example, trying to get a string when the value is a number.
	ErrTypeMismatch = errors.New("type mismatch")

	// errOperationFailed indicates that a JSON operation failed.
	// The error message contains details about the specific failure.
	// Internal: wrapped in JsonsError with context before returning to callers.
	errOperationFailed = errors.New("operation failed")

	// ErrInvalidPath indicates that the path syntax is invalid.
	// Paths should use format: "key.subkey" or "array[0]".
	ErrInvalidPath = errors.New("invalid path format")

	// ErrProcessorClosed indicates that the processor has been closed
	// and cannot accept new operations. Create a new processor with New().
	ErrProcessorClosed = errors.New("processor is closed")

	// errInternalError indicates an unexpected internal error.
	// This typically indicates a bug in the library.
	// Internal: not actionable by callers; always wrapped in JsonsError.
	errInternalError = errors.New("internal error")

	// errBreak is an internal signal to stop iteration.
	// Use item.Break() to stop iteration from callback functions.
	errBreak = errors.New("iteration break")

	// ErrSizeLimit indicates that the JSON size exceeds the configured limit.
	// Increase MaxJSONSize in Config to handle larger inputs.
	ErrSizeLimit = errors.New("size limit exceeded")

	// ErrDepthLimit indicates that the JSON nesting depth exceeds the configured limit.
	// Increase MaxNestingDepth in Config for deeply nested structures.
	ErrDepthLimit = errors.New("depth limit exceeded")

	// ErrConcurrencyLimit indicates that the concurrent operation count exceeds the limit.
	// Increase MaxConcurrency in Config for high-concurrency scenarios.
	ErrConcurrencyLimit = errors.New("concurrency limit exceeded")

	// ErrSecurityViolation indicates that potentially dangerous content was detected.
	// This includes prototype pollution patterns and other security risks.
	ErrSecurityViolation = errors.New("security violation detected")

	// ErrUnsupportedPath indicates that the path operation is not supported.
	// This may occur with invalid path segments or operations.
	ErrUnsupportedPath = errors.New("unsupported path operation")

	// errCacheDisabled indicates that caching is not enabled.
	// Internal: cache configuration detail, not actionable by callers.
	errCacheDisabled = errors.New("cache is disabled")

	// ErrOperationTimeout indicates that an operation exceeded its timeout duration.
	// Consider increasing timeout or optimizing the operation.
	ErrOperationTimeout = errors.New("operation timeout")

	// ErrResourceExhausted indicates that system resources are exhausted.
	// This may indicate a memory leak or excessive resource usage.
	ErrResourceExhausted = errors.New("system resources exhausted")
)

// JsonsError represents a JSON processing error with essential context
type JsonsError struct {
	Op      string `json:"op"`      // Operation that failed
	Path    string `json:"path"`    // JSON path where error occurred
	Message string `json:"message"` // Human-readable error message
	Err     error  `json:"err"`     // Underlying error
}

func (e *JsonsError) Error() string {
	if e == nil {
		return "json: nil error"
	}
	var baseMsg string
	if e.Path != "" {
		baseMsg = fmt.Sprintf("JSON %s failed at path '%s': %s", e.Op, e.Path, e.Message)
	} else {
		baseMsg = fmt.Sprintf("JSON %s failed: %s", e.Op, e.Message)
	}

	// Include underlying error for complete error chain information
	if e.Err != nil {
		return fmt.Sprintf("%s (caused by: %v)", baseMsg, e.Err)
	}
	return baseMsg
}

// Unwrap returns the underlying error for error chain support
func (e *JsonsError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

// Is implements error matching for Go 1.13+ error handling
// Compares Op, Path, and Err fields for complete equality.
// Note: Message is intentionally excluded as it's derived from other fields.
func (e *JsonsError) Is(target error) bool {
	if e == nil {
		return target == nil
	}
	if target == nil {
		return false
	}

	// Check if target is the same type using direct type assertion
	if targetErr, ok := target.(*JsonsError); ok {
		return e.Op == targetErr.Op && e.Path == targetErr.Path && e.Err == targetErr.Err
	}

	// Check underlying error
	return errors.Is(e.Err, target)
}


// SafeError returns a client-safe error message that omits internal details.
// Use this when returning errors to external clients (HTTP responses, APIs).
// The full Error() output may include path names and internal structure
// that should not be exposed to clients (CWE-209).
//
// Example:
//
//	result, err := processor.Get(json, "users.admin.password")
//	if err != nil {
//	    // Do not expose: "JSON get failed at path 'users.admin.password': ..."
//	    // Instead return: "path not found"
//	    http.Error(w, json.SafeError(err), http.StatusBadRequest)
//	}
func SafeError(err error) string {
	if err == nil {
		return ""
	}
	var jsErr *JsonsError
	if errors.As(err, &jsErr) {
		// Return only the sentinel error message, not the path or operation
		return jsErr.Err.Error()
	}
	return err.Error()
}

// newOperationError creates a JsonsError for operation failures.
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

// newSecurityError creates a security-related error
func newSecurityError(operation, message string) error {
	return &JsonsError{Op: operation, Message: message, Err: ErrSecurityViolation}
}

// RedactedPath returns a redacted version of a path for safe inclusion in error messages.
// SECURITY: Prevents path content from leaking into logs or error responses.
func RedactedPath(path string) string {
	if len(path) > 32 {
		return path[:8] + "..." + path[len(path)-8:]
	}
	if path != "" {
		return "***"
	}
	return ""
}

// GetErrorSuggestion provides suggestions for common errors
func getErrorSuggestion(err error) string {
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

// isRetryable determines if an error is retryable
func isRetryable(err error) bool {
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

// isSecurityRelated determines if an error is security-related
func isSecurityRelated(err error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, ErrSecurityViolation)
}

// userErrorSentinels is the fixed list of user-caused errors, pre-allocated to avoid per-call allocation.
var userErrorSentinels = []error{
	ErrInvalidJSON, ErrPathNotFound, ErrTypeMismatch,
	ErrInvalidPath, ErrUnsupportedPath,
}

// isUserError determines if an error is caused by user input
func isUserError(err error) bool {
	if err == nil {
		return false
	}
	for _, userErr := range userErrorSentinels {
		if errors.Is(err, userErr) {
			return true
		}
	}
	return false
}
