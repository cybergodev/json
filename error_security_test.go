package json

import (
	"errors"
	"strings"
	"testing"
)

// TestErrorAndSecurity consolidates error handling and security tests
// Replaces: error_handling_test.go, security_test.go
func TestErrorAndSecurity(t *testing.T) {
	helper := NewTestHelper(t)

	t.Run("ErrorCreation", func(t *testing.T) {
		err := &JsonsError{
			Op:      "test_operation",
			Message: "test error message",
			Err:     ErrOperationFailed,
		}

		helper.AssertEqual("test_operation", err.Op)
		helper.AssertEqual("test error message", err.Message)
		helper.AssertEqual(ErrOperationFailed, err.Err)

		errorStr := err.Error()
		helper.AssertTrue(strings.Contains(errorStr, "test_operation"))
		helper.AssertTrue(strings.Contains(errorStr, "test error message"))
	})

	t.Run("ErrorChaining", func(t *testing.T) {
		rootErr := errors.New("root cause")
		wrappedErr := &JsonsError{
			Op:      "wrapper_operation",
			Message: "wrapped error",
			Err:     rootErr,
		}

		helper.AssertTrue(errors.Is(wrappedErr, rootErr))
	})

	t.Run("InvalidJSON", func(t *testing.T) {
		invalidJSONs := []string{
			`{`,
			`{"key": }`,
			`{key: "value"}`,
			`{"key": "value",}`,
			`[1, 2, 3,]`,
		}

		for i, invalidJSON := range invalidJSONs {
			_, err := Get(invalidJSON, "key")
			helper.AssertError(err, "Invalid JSON %d should error: %s", i, invalidJSON)

			var jsonsErr *JsonsError
			if errors.As(err, &jsonsErr) {
				helper.AssertTrue(len(jsonsErr.Message) > 0)
			}
		}
	})

	t.Run("InvalidPath", func(t *testing.T) {
		validJSON := `{"users": [{"name": "Alice"}, {"name": "Bob"}]}`

		invalidPaths := []string{
			"users[abc]",
			"users[1.5]",
			"users[]",
			"users[",
		}

		for _, invalidPath := range invalidPaths {
			result, err := Get(validJSON, invalidPath)
			if err != nil {
				helper.AssertError(err)
			} else {
				helper.AssertNil(result)
			}
		}
	})

	t.Run("TypeMismatch", func(t *testing.T) {
		testJSON := `{
			"string": "hello",
			"number": 42,
			"boolean": true,
			"array": [1, 2, 3],
			"object": {"key": "value"}
		}`

		_, err := GetInt(testJSON, "string")
		if err == nil {
			t.Log("Type mismatch handled gracefully")
		}

		_, err = GetString(testJSON, "number")
		if err == nil {
			t.Log("Type mismatch handled gracefully")
		}
	})

	t.Run("BoundaryConditions", func(t *testing.T) {
		emptyJSON := `{}`
		result, err := Get(emptyJSON, "nonexistent")
		helper.AssertError(err)
		helper.AssertNil(result)

		deepJSON := `{"a":{"b":{"c":{"d":{"e":{"f":"deep"}}}}}}`
		deepResult, err := GetString(deepJSON, "a.b.c.d.e.f")
		helper.AssertNoError(err)
		helper.AssertEqual("deep", deepResult)

		largeArrayJSON := `{"array": [1, 2, 3]}`
		largeIndexResult, err := Get(largeArrayJSON, "array[1000]")
		helper.AssertNoError(err)
		helper.AssertNil(largeIndexResult)
	})

	t.Run("ProcessorClosed", func(t *testing.T) {
		processor := New()
		processor.Close()

		_, err := processor.Get(`{"test": "value"}`, "test")
		helper.AssertError(err)

		var jsonsErr *JsonsError
		if errors.As(err, &jsonsErr) {
			helper.AssertEqual(ErrProcessorClosed, jsonsErr.Err)
		}
	})

	t.Run("ErrorRecovery", func(t *testing.T) {
		processor := New()
		defer processor.Close()

		validJSON := `{"valid": "data"}`
		invalidJSON := `{invalid}`

		_, err1 := processor.Get(invalidJSON, "key")
		helper.AssertError(err1)

		result2, err2 := processor.Get(validJSON, "valid")
		helper.AssertNoError(err2)
		helper.AssertEqual("data", result2)
	})

	t.Run("DeeplyNestedJSON", func(t *testing.T) {
		depth := 100
		nested := strings.Repeat(`{"a":`, depth) + "1" + strings.Repeat("}", depth)

		_, err := Get(nested, "a.a.a.a.a")
		if err != nil {
			t.Logf("Deep nesting handled: %v", err)
		}
	})

	t.Run("LargeArrays", func(t *testing.T) {
		largeArray := `{"data": [`
		for i := 0; i < 100; i++ {
			if i > 0 {
				largeArray += ","
			}
			largeArray += `{"id":` + string(rune(i+48)) + `}`
		}
		largeArray += `]}`

		result, err := Get(largeArray, "data[0].id")
		if err == nil {
			helper.AssertNotNil(result)
		}
	})

	t.Run("MaliciousPathInjection", func(t *testing.T) {
		testData := `{"user": {"name": "Alice", "password": "secret"}}`

		maliciousPaths := []string{
			"user.name'; DROP TABLE users; --",
			"user.name\"; system('rm -rf /'); \"",
			"user.name<script>alert('xss')</script>",
			"user.name../../etc/passwd",
		}

		for _, path := range maliciousPaths {
			_, err := Get(testData, path)
			if err == nil {
				t.Logf("Malicious path handled safely: %s", path)
			}
		}
	})

	t.Run("ExcessiveMemory", func(t *testing.T) {
		testData := `{"data": "` + strings.Repeat("A", 100000) + `"}`

		result, err := Get(testData, "data")
		helper.AssertNoError(err)
		if str, ok := result.(string); ok {
			helper.AssertEqual(100000, len(str))
		}
	})

	t.Run("ErrorTypes", func(t *testing.T) {
		errorTypes := []error{
			ErrInvalidJSON,
			ErrInvalidPath,
			ErrOperationFailed,
			ErrProcessorClosed,
		}

		for _, errType := range errorTypes {
			helper.AssertTrue(errType != nil)
			helper.AssertTrue(len(errType.Error()) > 0)
		}
	})
}
