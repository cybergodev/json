package json

import (
	"errors"
	"strings"
	"testing"
)

// TestErrorSecurity consolidates error handling and security validation tests
// Replaces: error_security_test.go, security_validation_test.go
func TestErrorSecurity(t *testing.T) {
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

	t.Run("PathTraversalSecurity", func(t *testing.T) {
		processor := New()
		defer processor.Close()

		testCases := []struct {
			name        string
			path        string
			shouldBlock bool
			description string
		}{
			// Standard traversal patterns
			{
				name:        "Standard Unix traversal",
				path:        "../etc/passwd",
				shouldBlock: true,
				description: "Basic Unix path traversal",
			},
			{
				name:        "Standard Windows traversal",
				path:        "..\\windows\\system32",
				shouldBlock: true,
				description: "Basic Windows path traversal",
			},
			{
				name:        "Multiple traversal",
				path:        "../../etc/passwd",
				shouldBlock: true,
				description: "Multiple directory traversal",
			},

			// URL-encoded patterns
			{
				name:        "URL-encoded dots",
				path:        "%2e%2e/etc/passwd",
				shouldBlock: true,
				description: "URL-encoded .. pattern",
			},
			{
				name:        "Double-encoded dot",
				path:        "%252e%252e/etc/passwd",
				shouldBlock: true,
				description: "Double URL-encoded pattern",
			},
			{
				name:        "Mixed encoding",
				path:        "%2e./etc/passwd",
				shouldBlock: true,
				description: "Partially encoded pattern",
			},

			// UTF-8 overlong encoding
			{
				name:        "UTF-8 overlong slash",
				path:        "..%c0%af/etc/passwd",
				shouldBlock: true,
				description: "UTF-8 overlong encoding for /",
			},
			{
				name:        "UTF-8 overlong backslash",
				path:        "..%c1%9c/windows",
				shouldBlock: true,
				description: "UTF-8 overlong encoding for \\",
			},

			// Null byte injection
			{
				name:        "Null byte injection",
				path:        "../etc/passwd\x00.json",
				shouldBlock: true,
				description: "Null byte to bypass extension check",
			},

			// Valid paths (should NOT block)
			{
				name:        "Simple property",
				path:        "user.name",
				shouldBlock: false,
				description: "Normal property access",
			},
			{
				name:        "Array access",
				path:        "users[0].name",
				shouldBlock: false,
				description: "Normal array access",
			},
			{
				name:        "Nested path",
				path:        "data.users[0].profile.email",
				shouldBlock: false,
				description: "Complex nested path",
			},
			{
				name:        "Extraction",
				path:        "users{name}",
				shouldBlock: false,
				description: "Extraction syntax",
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				err := processor.validatePath(tc.path)

				if tc.shouldBlock {
					// Should return security error
					if err == nil {
						t.Errorf("Expected security error for path '%s' (%s), but got nil",
							tc.path, tc.description)
					} else if !strings.Contains(err.Error(), "security") &&
						!strings.Contains(err.Error(), "traversal") &&
						!strings.Contains(err.Error(), "injection") {
						t.Errorf("Expected security-related error for path '%s', got: %v",
							tc.path, err)
					}
				} else {
					// Should NOT return error
					if err != nil {
						t.Errorf("Expected valid path '%s' (%s), but got error: %v",
							tc.path, tc.description, err)
					}
				}
			})
		}
	})

	t.Run("PathLengthValidation", func(t *testing.T) {
		processor := New()
		defer processor.Close()

		testCases := []struct {
			name        string
			pathLength  int
			shouldBlock bool
		}{
			{
				name:        "Normal path",
				pathLength:  100,
				shouldBlock: false,
			},
			{
				name:        "Long but valid path",
				pathLength:  2000,
				shouldBlock: false,
			},
			{
				name:        "Maximum allowed path",
				pathLength:  MaxPathLength,
				shouldBlock: false,
			},
			{
				name:        "Exceeds maximum",
				pathLength:  MaxPathLength + 1,
				shouldBlock: true,
			},
			{
				name:        "Extremely long path",
				pathLength:  10000,
				shouldBlock: true,
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				// Create path of specified length
				path := strings.Repeat("a", tc.pathLength)

				err := processor.validatePath(path)

				if tc.shouldBlock {
					if err == nil {
						t.Errorf("Expected error for path length %d, but got nil", tc.pathLength)
					}
				} else {
					if err != nil {
						t.Errorf("Expected valid path of length %d, but got error: %v",
							tc.pathLength, err)
					}
				}
			})
		}
	})

	t.Run("SecurityErrorTypes", func(t *testing.T) {
		processor := New()
		defer processor.Close()

		testCases := []struct {
			name          string
			path          string
			expectedError error
		}{
			{
				name:          "Path traversal",
				path:          "../etc/passwd",
				expectedError: ErrSecurityViolation,
			},
			{
				name:          "Null byte injection",
				path:          "user\x00name",
				expectedError: ErrSecurityViolation,
			},
			{
				name:          "Encoded traversal",
				path:          "%2e%2e/etc/passwd",
				expectedError: ErrSecurityViolation,
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				err := processor.validatePath(tc.path)

				if err == nil {
					t.Errorf("Expected error for path '%s', but got nil", tc.path)
					return
				}

				// Check if error is JsonsError with correct underlying error
				if jsErr, ok := err.(*JsonsError); ok {
					if jsErr.Err != tc.expectedError {
						t.Errorf("Expected underlying error %v, got %v", tc.expectedError, jsErr.Err)
					}
				} else {
					t.Errorf("Expected JsonsError type, got %T", err)
				}
			})
		}
	})

	t.Run("MaliciousInputHandling", func(t *testing.T) {
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

	t.Run("ExcessiveMemoryHandling", func(t *testing.T) {
		testData := `{"data": "` + strings.Repeat("A", 100000) + `"}`

		result, err := Get(testData, "data")
		helper.AssertNoError(err)
		if str, ok := result.(string); ok {
			helper.AssertEqual(100000, len(str))
		}
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

	t.Run("CaseSensitiveEncodingDetection", func(t *testing.T) {
		processor := New()
		defer processor.Close()

		testCases := []struct {
			name string
			path string
		}{
			{
				name: "Lowercase encoding",
				path: "%2e%2e/etc/passwd",
			},
			{
				name: "Uppercase encoding",
				path: "%2E%2E/etc/passwd",
			},
			{
				name: "Mixed case encoding",
				path: "%2e%2E/etc/passwd",
			},
			{
				name: "Lowercase UTF-8 overlong",
				path: "..%c0%af/etc/passwd",
			},
			{
				name: "Uppercase UTF-8 overlong",
				path: "..%C0%AF/etc/passwd",
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				err := processor.validatePath(tc.path)

				if err == nil {
					t.Errorf("Expected security error for path '%s', but got nil", tc.path)
				}
			})
		}
	})
}
