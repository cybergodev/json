//go:build example

package main

import (
	"errors"
	"fmt"

	"github.com/cybergodev/json"
)

// Error Handling Example
//
// This example demonstrates comprehensive error handling in the cybergodev/json library.
// Learn about error classification, suggestions, and structured error information.
//
// Topics covered:
// - JsonsError structured error information
// - Error classification functions
// - Error suggestions for common issues
// - Retry logic for recoverable errors
// - Error wrapping with context
//
// Run: go run -tags=example examples/4_error_handling.go

func main() {
	fmt.Println("JSON Library - Error Handling")
	fmt.Println("================================")

	// 1. STRUCTURED ERRORS
	demonstrateStructuredErrors()

	// 2. ERROR CLASSIFICATION
	demonstrateErrorClassification()

	// 3. ERROR SUGGESTIONS
	demonstrateErrorSuggestions()

	// 4. RETRY LOGIC
	demonstrateRetryLogic()

	// 5. ERROR WRAPPING
	demonstrateErrorWrapping()

	fmt.Println("\nError handling complete!")
}

func demonstrateStructuredErrors() {
	fmt.Println("1. Structured Errors (JsonsError)")
	fmt.Println("----------------------------------")

	// Invalid JSON example
	invalidJSON := `{"name": "John", "age": }`

	_, err := json.Get(invalidJSON, "name")
	if err != nil {
		// Check if it's a JsonsError
		var jsonErr *json.JsonsError
		if errors.As(err, &jsonErr) {
			fmt.Printf("   Structured error detected:\n")
			fmt.Printf("   - Operation: %s\n", jsonErr.Op)
			fmt.Printf("   - Path: %s\n", jsonErr.Path)
			fmt.Printf("   - Message: %s\n", jsonErr.Message)
			fmt.Printf("   - Underlying error: %v\n", jsonErr.Err)
		}
	}

	// Path not found example
	validJSON := `{"user": {"name": "Alice"}}`
	_, err = json.Get(validJSON, "user.email")
	if err != nil {
		var jsonErr *json.JsonsError
		if errors.As(err, &jsonErr) {
			fmt.Printf("\n   Path error details:\n")
			fmt.Printf("   - Operation: %s\n", jsonErr.Op)
			fmt.Printf("   - Missing path: %s\n", jsonErr.Path)
		}
	}
}

func demonstrateErrorClassification() {
	fmt.Println("\n2. Error Classification")
	fmt.Println("-------------------------")

	testCases := []struct {
		name string
		err  error
	}{
		{"Invalid JSON", json.ErrInvalidJSON},
		{"Path not found", json.ErrPathNotFound},
		{"Type mismatch", json.ErrTypeMismatch},
		{"Security violation", json.ErrSecurityViolation},
		{"Size limit", json.ErrSizeLimit},
	}

	fmt.Println("   Error classification results:")
	for _, tc := range testCases {
		isSecurity := errors.Is(tc.err, json.ErrSecurityViolation)
		isUser := errors.Is(tc.err, json.ErrInvalidJSON) || errors.Is(tc.err, json.ErrPathNotFound) ||
			errors.Is(tc.err, json.ErrTypeMismatch) || errors.Is(tc.err, json.ErrInvalidPath) ||
			errors.Is(tc.err, json.ErrUnsupportedPath)
		// Retryable: the caller can raise a Config limit (MaxJSONSize /
		// MaxNestingDepthSecurity) and retry the same input successfully.
		isRetryable := errors.Is(tc.err, json.ErrSizeLimit) || errors.Is(tc.err, json.ErrDepthLimit)

		fmt.Printf("   [%s]\n", tc.name)
		fmt.Printf("     Security-related: %t\n", isSecurity)
		fmt.Printf("     User error: %t\n", isUser)
		fmt.Printf("     Retryable: %t\n", isRetryable)
	}
}

func demonstrateErrorSuggestions() {
	fmt.Println("\n3. Error Suggestions")
	fmt.Println("----------------------")

	// Simulate various errors
	errs := []error{
		json.ErrInvalidJSON,
		json.ErrPathNotFound,
		json.ErrTypeMismatch,
		json.ErrInvalidPath,
		json.ErrSizeLimit,
		json.ErrDepthLimit,
		json.ErrSecurityViolation,
	}

	fmt.Println("   Error suggestions (use errors.Is for matching):")
	for _, err := range errs {
		fmt.Printf("\n   [%v]\n", err)
		if errors.Is(err, json.ErrInvalidJSON) {
			fmt.Printf("   Suggestion: Check JSON syntax - ensure proper quotes, brackets, and commas\n")
		} else if errors.Is(err, json.ErrPathNotFound) {
			fmt.Printf("   Suggestion: Verify the path exists in the JSON structure\n")
		} else {
			fmt.Printf("   Suggestion: Check the error message for specific details\n")
		}
	}
}

func demonstrateRetryLogic() {
	fmt.Println("\n4. Retry Logic")
	fmt.Println("---------------")

	// This library is synchronous and local, so a true "transient fault" rarely
	// occurs. "Retryable" here means the caller can change something and try
	// again: size/depth limits are configurable — raise them in Config and retry
	// the same input. Malformed input, a missing path, or a security violation
	// is permanent and will not succeed no matter how many times it is retried.
	testErrors := []struct {
		name string
		err  error
	}{
		{"Invalid JSON", json.ErrInvalidJSON},
		{"Path not found", json.ErrPathNotFound},
		{"Size limit", json.ErrSizeLimit},
		{"Depth limit", json.ErrDepthLimit},
		{"Security violation", json.ErrSecurityViolation},
	}

	fmt.Println("   Retry decision for each error:")
	for _, test := range testErrors {
		configurable := errors.Is(test.err, json.ErrSizeLimit) || errors.Is(test.err, json.ErrDepthLimit)
		action := "Skip retry (permanent)"
		if configurable {
			action = "Retry with raised Config limit"
		}
		fmt.Printf("   [%s]: %s\n", test.name, action)
	}
}

func demonstrateErrorWrapping() {
	fmt.Println("\n5. Error Wrapping")
	fmt.Println("-----------------")

	// Wrap errors with additional context
	baseErr := json.ErrPathNotFound

	// WrapError and WrapPathError are internal helpers; users receive JsonsError
	// from library operations. To create wrapped errors, use fmt.Errorf with %w:
	wrapped1 := fmt.Errorf("get_user: failed to retrieve user data: %w", baseErr)
	fmt.Printf("   Wrapped error 1: %v\n", wrapped1)

	wrapped2 := fmt.Errorf("get_field [user.profile.email]: email field not found: %w", baseErr)
	fmt.Printf("   Wrapped error 2: %v\n", wrapped2)

	// Unwrap to get original error
	unwrapped := errors.Unwrap(wrapped1)
	if errors.Is(unwrapped, baseErr) {
		fmt.Println("\n   ✓ Successfully unwrapped to original error")
	}

	// Error matching with errors.Is
	if errors.Is(wrapped1, json.ErrPathNotFound) {
		fmt.Println("   ✓ Error matches using errors.Is()")
	}
}
