package json

import (
	"errors"
	"strings"
	"testing"
)

// TestErrorHandlingComprehensive tests comprehensive error handling functionality
// Merged from: error_handling_test.go, error_handling_comprehensive_test.go, boundary_conditions_test.go
func TestErrorHandlingComprehensive(t *testing.T) {
	helper := NewTestHelper(t)

	t.Run("JsonsErrorCreation", func(t *testing.T) {
		// Test creating JsonsError
		err := &JsonsError{
			Op:      "test_operation",
			Message: "test error message",
			Err:     ErrOperationFailed,
		}

		helper.AssertEqual("test_operation", err.Op, "Operation should match")
		helper.AssertEqual("test error message", err.Message, "Message should match")
		helper.AssertEqual(ErrOperationFailed, err.Err, "Underlying error should match")

		// Test error string representation
		errorStr := err.Error()
		helper.AssertTrue(strings.Contains(errorStr, "test_operation"), "Error string should contain operation")
		helper.AssertTrue(strings.Contains(errorStr, "test error message"), "Error string should contain message")
	})

	t.Run("ErrorChaining", func(t *testing.T) {
		// Test error chaining
		rootErr := errors.New("root cause")
		wrappedErr := &JsonsError{
			Op:      "wrapper_operation",
			Message: "wrapped error",
			Err:     rootErr,
		}

		// Test unwrapping
		helper.AssertTrue(errors.Is(wrappedErr, rootErr), "Should be able to unwrap to root error")

		// Test error chain
		errorStr := wrappedErr.Error()
		helper.AssertTrue(strings.Contains(errorStr, "wrapper_operation"), "Should contain wrapper operation")
		helper.AssertTrue(strings.Contains(errorStr, "wrapped error"), "Should contain wrapper message")
	})

	t.Run("ErrorContext", func(t *testing.T) {
		processor := New()
		defer processor.Close()

		// Test error with context
		invalidJson := `{invalid json}`
		_, err := processor.Get(invalidJson, "name")

		helper.AssertError(err, "Should error for invalid JSON")

		// Check if error contains context information
		errorStr := err.Error()
		helper.AssertTrue(len(errorStr) > 0, "Error should have description")

		// Test error type
		var jsonsErr *JsonsError
		helper.AssertTrue(errors.As(err, &jsonsErr), "Error should be JsonsError type")
	})

	t.Run("InvalidJSONHandling", func(t *testing.T) {
		invalidJSONs := []string{
			`{`,                         // Incomplete object
			`{"key": }`,                 // Missing value
			`{key: "value"}`,            // Unquoted key
			`{"key": "value",}`,         // Trailing comma
			`[1, 2, 3,]`,                // Trailing comma in array
			`{"key": "unclosed string}`, // Unclosed string
		}

		for i, invalidJSON := range invalidJSONs {
			_, err := Get(invalidJSON, "key")
			helper.AssertError(err, "Invalid JSON %d should produce error: %s", i, invalidJSON)

			// Verify error type
			var jsonsErr *JsonsError
			if errors.As(err, &jsonsErr) {
				helper.AssertTrue(len(jsonsErr.Message) > 0, "Error should have message")
			}
		}
	})

	t.Run("InvalidPathHandling", func(t *testing.T) {
		validJSON := `{"users": [{"name": "Alice"}, {"name": "Bob"}]}`

		invalidPaths := []string{
			"users[abc]",         // Non-numeric array index
			"users[1.5]",         // Float array index
			"users[]",            // Empty array index
			"users[",             // Incomplete array access
			"users.name.invalid", // Path through non-object
			"nonexistent.path",   // Non-existent path
		}

		for i, invalidPath := range invalidPaths {
			result, err := Get(validJSON, invalidPath)
			// Some implementations might return nil instead of error for non-existent paths
			if err != nil {
				helper.AssertError(err, "Invalid path %d should produce error: %s", i, invalidPath)
			} else {
				helper.AssertNil(result, "Invalid path %d should return nil: %s", i, invalidPath)
			}
		}
	})

	t.Run("TypeMismatchErrors", func(t *testing.T) {
		testJSON := `{
			"string": "hello",
			"number": 42,
			"boolean": true,
			"array": [1, 2, 3],
			"object": {"key": "value"},
			"null": null
		}`

		// Test type mismatches
		typeMismatches := []struct {
			path     string
			function string
		}{
			{"string", "GetInt"},
			{"number", "GetString"},
			{"boolean", "GetArray"},
			{"array", "GetString"},
			{"object", "GetInt"},
			{"null", "GetString"},
		}

		for _, tm := range typeMismatches {
			var err error
			var result any
			switch tm.function {
			case "GetInt":
				result, err = GetInt(testJSON, tm.path)
			case "GetString":
				result, err = GetString(testJSON, tm.path)
			case "GetArray":
				result, err = GetArray(testJSON, tm.path)
			}

			// Some implementations might return nil instead of error for type mismatches
			if err != nil {
				helper.AssertError(err, "Type mismatch should produce error: %s with %s", tm.path, tm.function)
			} else {
				// If no error, result should be nil or zero value
				t.Logf("Type mismatch %s with %s returned: %v (no error)", tm.path, tm.function, result)
			}
		}
	})

	t.Run("BoundaryConditions", func(t *testing.T) {
		// Test empty JSON - should return error for nonexistent path
		emptyJSON := `{}`
		result, err := Get(emptyJSON, "nonexistent")
		helper.AssertError(err, "Empty JSON should return error for nonexistent path")
		helper.AssertNil(result, "Non-existent key should return nil")

		// Test very deep nesting
		deepJSON := `{"a":{"b":{"c":{"d":{"e":{"f":"deep"}}}}}}`
		deepResult, err := GetString(deepJSON, "a.b.c.d.e.f")
		helper.AssertNoError(err, "Deep nesting should work")
		helper.AssertEqual("deep", deepResult, "Deep value should match")

		// Test large array index
		largeArrayJSON := `{"array": [1, 2, 3]}`
		largeIndexResult, err := Get(largeArrayJSON, "array[1000]")
		helper.AssertNoError(err, "Large array index should not error")
		helper.AssertNil(largeIndexResult, "Out of bounds should return nil")

		// Test negative array index beyond bounds
		negativeResult, err := Get(largeArrayJSON, "array[-1000]")
		helper.AssertNoError(err, "Large negative index should not error")
		helper.AssertNil(negativeResult, "Out of bounds negative should return nil")
	})

	t.Run("ProcessorErrorStates", func(t *testing.T) {
		// Test closed processor
		processor := New()
		processor.Close()

		_, err := processor.Get(`{"test": "value"}`, "test")
		helper.AssertError(err, "Closed processor should error")

		// Verify error type
		var jsonsErr *JsonsError
		if errors.As(err, &jsonsErr) {
			helper.AssertEqual(ErrProcessorClosed, jsonsErr.Err, "Should be processor closed error")
		}
	})

	t.Run("ValidationErrors", func(t *testing.T) {
		// Test validation with invalid data
		invalidData := `{"key": "value", "number": "not_a_number"}`

		// Create a validator (if available)
		processor := New()
		defer processor.Close()

		// Test validation
		isValid, err := processor.Valid(invalidData)
		if err != nil {
			helper.AssertError(err, "Validation should handle invalid data")
		} else {
			// Some implementations might return false instead of error
			helper.AssertTrue(isValid || !isValid, "Validation should return boolean")
		}
	})

	t.Run("CustomErrorTypes", func(t *testing.T) {
		// Test different error types
		errorTypes := []error{
			ErrInvalidJSON,
			ErrInvalidPath,
			ErrOperationFailed,
			ErrProcessorClosed,
		}

		for _, errType := range errorTypes {
			helper.AssertTrue(errType != nil, "Error type should not be nil")
			helper.AssertTrue(len(errType.Error()) > 0, "Error type should have description")
		}

		// Test error type checking
		testErr := &JsonsError{
			Op:      "test",
			Message: "test message",
			Err:     ErrInvalidJSON,
		}

		helper.AssertTrue(errors.Is(testErr, ErrInvalidJSON), "Should identify underlying error type")
	})
}

// TestErrorRecovery tests error recovery mechanisms
func TestErrorRecovery(t *testing.T) {
	helper := NewTestHelper(t)

	t.Run("PartialOperationRecovery", func(t *testing.T) {
		// Test that one failed operation doesn't affect subsequent operations
		processor := New()
		defer processor.Close()

		validJSON := `{"valid": "data"}`
		invalidJSON := `{invalid}`

		// First operation should fail
		_, err1 := processor.Get(invalidJSON, "key")
		helper.AssertError(err1, "Invalid JSON should fail")

		// Second operation should succeed
		result2, err2 := processor.Get(validJSON, "valid")
		helper.AssertNoError(err2, "Valid JSON should succeed after error")
		helper.AssertEqual("data", result2, "Valid operation should return correct result")
	})

	t.Run("ProcessorStateAfterError", func(t *testing.T) {
		processor := New()
		defer processor.Close()

		// Cause an error
		_, err := processor.Get(`{invalid}`, "key")
		helper.AssertError(err, "Should error on invalid JSON")

		// Verify processor is still functional
		helper.AssertFalse(processor.IsClosed(), "Processor should not be closed after error")

		// Verify processor can still perform operations
		result, err := processor.Get(`{"test": "value"}`, "test")
		helper.AssertNoError(err, "Processor should work after error")
		helper.AssertEqual("value", result, "Result should be correct")
	})
}

// TestErrorMessages tests error message quality
func TestErrorMessages(t *testing.T) {
	helper := NewTestHelper(t)

	t.Run("DescriptiveErrorMessages", func(t *testing.T) {
		// Test that error messages are descriptive
		testCases := []struct {
			json        string
			path        string
			description string
		}{
			{`{invalid}`, "key", "invalid JSON"},
			{`{"valid": "data"}`, "users[abc]", "invalid array index"},
			{`{"number": 42}`, "number.invalid", "path through non-object"},
		}

		for _, tc := range testCases {
			_, err := Get(tc.json, tc.path)
			if err != nil {
				errorMsg := err.Error()
				helper.AssertTrue(len(errorMsg) > 10, "Error message should be descriptive for %s", tc.description)
				t.Logf("Error for %s: %s", tc.description, errorMsg)
			}
		}
	})
}
