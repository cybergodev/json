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
// - ErrorClassifier for error categorization
// - Error suggestions for common issues
// - Retry logic for recoverable errors
// - Error wrapping with context
//
// Run: go run examples/4_error_handling.go

func main() {
	fmt.Println("🚨 JSON Library - Error Handling")
	fmt.Println("================================\n ")

	// 1. STRUCTURED ERRORS
	fmt.Println("1️⃣  Structured Errors (JsonsError)")
	fmt.Println("──────────────────────────────────")
	demonstrateStructuredErrors()

	// 2. ERROR CLASSIFICATION
	fmt.Println("\n2️⃣  Error Classification")
	fmt.Println("─────────────────────────")
	demonstrateErrorClassification()

	// 3. ERROR SUGGESTIONS
	fmt.Println("\n3️⃣  Error Suggestions")
	fmt.Println("──────────────────────")
	demonstrateErrorSuggestions()

	// 4. RETRY LOGIC
	fmt.Println("\n4️⃣  Retry Logic")
	fmt.Println("───────────────")
	demonstrateRetryLogic()

	// 5. ERROR WRAPPING
	fmt.Println("\n5️⃣  Error Wrapping")
	fmt.Println("─────────────────")
	demonstrateErrorWrapping()

	fmt.Println("\n✅ Error handling complete!")
}

func demonstrateStructuredErrors() {
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
	classifier := json.NewErrorClassifier()

	testCases := []struct {
		name string
		err  error
	}{
		{"Invalid JSON", json.ErrInvalidJSON},
		{"Path not found", json.ErrPathNotFound},
		{"Type mismatch", json.ErrTypeMismatch},
		{"Security violation", json.ErrSecurityViolation},
		{"Operation timeout", json.ErrOperationTimeout},
	}

	fmt.Println("   Error classification results:")
	for _, tc := range testCases {
		isSecurity := classifier.IsSecurityRelated(tc.err)
		isUser := classifier.IsUserError(tc.err)
		isRetryable := classifier.IsRetryable(tc.err)

		fmt.Printf("   [%s]\n", tc.name)
		fmt.Printf("     Security-related: %t\n", isSecurity)
		fmt.Printf("     User error: %t\n", isUser)
		fmt.Printf("     Retryable: %t\n", isRetryable)
	}
}

func demonstrateErrorSuggestions() {
	classifier := json.NewErrorClassifier()

	// Simulate various errs
	errs := []error{
		json.ErrInvalidJSON,
		json.ErrPathNotFound,
		json.ErrTypeMismatch,
		json.ErrInvalidPath,
		json.ErrSizeLimit,
		json.ErrDepthLimit,
		json.ErrSecurityViolation,
	}

	fmt.Println("   Error suggestions:")
	for _, err := range errs {
		suggestion := classifier.GetErrorSuggestion(err)
		fmt.Printf("\n   [%v]\n", err)
		fmt.Printf("   💡 Suggestion: %s\n", suggestion)
	}
}

func demonstrateRetryLogic() {
	classifier := json.NewErrorClassifier()

	// Simulate errors and check retry ability
	testErrors := []struct {
		name string
		err  error
	}{
		{"Timeout error", json.ErrOperationTimeout},
		{"Concurrency limit", json.ErrConcurrencyLimit},
		{"Invalid JSON", json.ErrInvalidJSON},
		{"Path not found", json.ErrPathNotFound},
	}

	fmt.Println("   Retry decision for each error:")
	for _, test := range testErrors {
		retryable := classifier.IsRetryable(test.err)
		action := "Skip retry"
		if retryable {
			action = "Attempt retry"
		}
		fmt.Printf("   [%s]: %s\n", test.name, action)
	}
}

func demonstrateErrorWrapping() {
	// Wrap errors with additional context
	baseErr := json.ErrPathNotFound

	// Use WrapError to add context
	wrapped1 := json.WrapError(baseErr, "get_user", "failed to retrieve user data")
	fmt.Printf("   Wrapped error 1: %v\n", wrapped1)

	// Use WrapPathError to add path context
	wrapped2 := json.WrapPathError(baseErr, "get_field", "user.profile.email", "email field not found")
	fmt.Printf("   Wrapped error 2: %v\n", wrapped2)

	// Unwrap to get original error
	if errors.Is(baseErr, errors.Unwrap(wrapped1)) {
		fmt.Println("\n   ✓ Successfully unwrapped to original error")
	}

	// Error matching with errors.Is
	if errors.Is(wrapped1, json.ErrPathNotFound) {
		fmt.Println("   ✓ Error matches using errors.Is()")
	}
}
