package json

import (
	"strings"
	"testing"
)

// TestPathTraversalSecurity validates enhanced path traversal protection
func TestPathTraversalSecurity(t *testing.T) {
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
		{
			name:        "Property with dots in name",
			path:        "config.api.endpoint",
			shouldBlock: false,
			description: "Multiple dots in valid path",
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
}

// TestPathLengthValidation validates path length limits
func TestPathLengthValidation(t *testing.T) {
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
			pathLength:  5000,
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
			pathLength:  50000,
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
}

// TestSecurityErrorTypes validates that security violations return appropriate error types
func TestSecurityErrorTypes(t *testing.T) {
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
}

// TestCaseSensitiveEncodingDetection validates case-insensitive encoding detection
func TestCaseSensitiveEncodingDetection(t *testing.T) {
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
}

// BenchmarkPathSecurityValidation benchmarks the security validation performance
func BenchmarkPathSecurityValidation(b *testing.B) {
	processor := New()
	defer processor.Close()

	testPaths := []string{
		"user.name",                    // Valid simple path
		"users[0].profile.email",       // Valid complex path
		"../etc/passwd",                // Traversal attack
		"%2e%2e/etc/passwd",            // Encoded attack
		"data.config.settings.timeout", // Valid nested path
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		path := testPaths[i%len(testPaths)]
		_ = processor.validatePath(path)
	}
}
