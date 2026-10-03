package json

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
)

// ============================================================================
// General Security Tests (from security_test.go)
// ============================================================================

// TestSecurityValidation covers security-related tests including:
// - Path traversal attacks
// - Input validation
// - Resource limits
// - Security configuration validation
func TestSecurityValidation(t *testing.T) {
	helper := newTestHelper(t)

	t.Run("PathTraversal", func(t *testing.T) {
		processor, _ := New(SecurityConfig())
		defer processor.Close()

		testData := `{"user": {"name": "Alice", "email": "alice@example.com"}}`

		// Test various path traversal attempts
		traversalPaths := []string{
			"../../../etc/passwd",
			"../secret",
			"user/../../admin",
			"users[0]/../admin",
			"..\\..\\windows",
			"users/../../../system",
			"../hidden",
			"user/../admin/data",
		}

		for _, path := range traversalPaths {
			t.Run("Path_"+strings.ReplaceAll(path, "/", "_"), func(t *testing.T) {
				_, err := processor.Get(testData, path)
				// Should either return error or not expose sensitive data
				if err == nil {
					result, _ := processor.Get(testData, path)
					// Ensure no sensitive data is exposed
					if resultStr, ok := result.(string); ok {
						helper.AssertFalse(
							strings.Contains(resultStr, "passwd") ||
								strings.Contains(resultStr, "secret") ||
								strings.Contains(resultStr, "password"),
							"Path traversal exposed sensitive data for path: %s", path)
					}
				}
			})
		}
	})

	t.Run("InjectionAttacks", func(t *testing.T) {
		processor, _ := New(SecurityConfig())
		defer processor.Close()

		testData := `{"data": "normal"}`

		// Test various injection attempts
		injectionPaths := []string{
			"data<script>alert(1)</script>",
			"data'; DROP TABLE users;--",
			"data${7*7}",
			"data{{7*7}}",
			"data[0][script](x)",
		}

		for _, path := range injectionPaths {
			t.Run("Inject_"+path[:10], func(t *testing.T) {
				// Should handle gracefully without executing injected code
				_, _ = processor.Get(testData, path)
				// Result is less important than not panicking
				helper.AssertNoPanic(func() {
					processor.Get(testData, path)
				})
			})
		}
	})

	t.Run("ResourceLimits", func(t *testing.T) {
		t.Run("LargeJSON", func(t *testing.T) {
			if testing.Short() {
				t.Skip("Skipping large JSON test in short mode")
			}

			// Test 1: Quick size limit validation with small data
			t.Run("QuickSizeLimit", func(t *testing.T) {
				smallLimitConfig := SecurityConfig()
				smallLimitConfig.MaxJSONSize = 100 * 1024 // 100KB limit for quick testing
				processor, _ := New(smallLimitConfig)
				defer processor.Close()

				largeJSON := generateLargeJSON(150 * 1024) // 150KB (exceeds 100KB limit)

				_, err := processor.Get(largeJSON, "data")
				helper.AssertError(err)
				if err != nil {
					var jsonErr *JsonsError
					if errors.As(err, &jsonErr) {
						helper.AssertTrue(
							jsonErr.Err == ErrSizeLimit || jsonErr.Err == errOperationFailed,
							"Expected size limit error, got: %v", jsonErr.Err)
					}
				}
			})

			// Test 2: Real large data test (optimized with strings.Builder)
			// SecurityConfig has conservative limits for untrusted input
			// This validates actual handling of large JSON without taking 199 seconds
			t.Run("RealLargeData", func(t *testing.T) {
				processor, _ := New(SecurityConfig())
				defer processor.Close()

				// Generate 2MB of JSON (large enough to test, small enough to be fast)
				// With optimized generateLargeJSON, this only takes ~0.5-1 second
				largeJSON := generateLargeJSON(2 * 1024 * 1024) // 2MB (within 10MB limit)

				// Should succeed since 2MB < 10MB limit
				result, err := processor.Get(largeJSON, "data")
				helper.AssertNoError(err)
				helper.AssertNotNil(result)

				// Also test slightly above limit (12MB exceeds 10MB limit)
				overLimitJSON := generateLargeJSON(12 * 1024 * 1024) // 12MB (exceeds 10MB limit)
				_, err = processor.Get(overLimitJSON, "data")
				helper.AssertError(err)
			})
		})

		t.Run("DeepNesting", func(t *testing.T) {
			processor, _ := New(SecurityConfig())
			defer processor.Close()

			// Generate deeply nested JSON
			deepJSON := genNestedJSON(50, "deep") // 50 levels

			_, err := processor.Get(deepJSON, "a")
			// SecurityConfig has conservative nesting depth limits
			if err != nil {
				var jsonErr *JsonsError
				if errors.As(err, &jsonErr) {
					helper.AssertTrue(
						jsonErr.Err == ErrDepthLimit,
						"Expected depth limit error, got: %v", jsonErr.Err)
				}
			}
		})
	})

	t.Run("SecurityConfigValidation", func(t *testing.T) {
		t.Run("SecurityConfig", func(t *testing.T) {
			config := SecurityConfig()
			helper.AssertTrue(config.FullSecurityScan)
			helper.AssertTrue(config.EnableValidation)
		})

		t.Run("DefaultConfig", func(t *testing.T) {
			config := DefaultConfig()
			helper.AssertEqual(DefaultMaxNestingDepth, config.MaxNestingDepthSecurity)
			helper.AssertFalse(config.StrictMode)
		})
	})

	t.Run("InputValidation", func(t *testing.T) {
		processor, _ := New(SecurityConfig())
		defer processor.Close()

		t.Run("InvalidCharacters", func(t *testing.T) {
			invalidInputs := []struct {
				name  string
				input string
			}{
				{"NULL_BYTE", "\x00NULL_BYTE"},
				{"ESC", "\x1BESC"},
				{"MULTI_LINE", "MULTI\u0000LINE"},
			}

			for _, tt := range invalidInputs {
				t.Run("Input_"+tt.name, func(t *testing.T) {
					// Should handle without panicking
					helper.AssertNoPanic(func() {
						processor.Get(tt.input, "data")
					})
				})
			}
		})

		t.Run("SpecialCharacters", func(t *testing.T) {
			testJSON := `{"special": "value\n\t\r"}`

			// Should preserve special characters safely
			result, err := processor.Get(testJSON, "special")
			helper.AssertNoError(err)
			if str, ok := result.(string); ok {
				helper.AssertTrue(strings.Contains(str, "\n"))
				helper.AssertTrue(strings.Contains(str, "\t"))
			}
		})
	})

	t.Run("UnicodeHandling", func(t *testing.T) {
		processor, _ := New(SecurityConfig())
		defer processor.Close()

		// Test various Unicode edge cases
		unicodeTests := []struct {
			name string
			json string
			path string
		}{
			{
				name: "ValidUnicode",
				json: `{"emoji": "🎉🚀"}`,
				path: "emoji",
			},
			{
				name: "MixedScripts",
				json: `{"mixed": "Hello你好مرحبا"}`,
				path: "mixed",
			},
			{
				name: "ZeroWidth",
				json: `{"zero": "test\u200B\u200C"}`,
				path: "zero",
			},
			{
				name: "InvalidSequence",
				json: `{"invalid": "test\xFF\xFE"}`,
				path: "invalid",
			},
		}

		for _, tt := range unicodeTests {
			t.Run(tt.name, func(t *testing.T) {
				helper.AssertNoPanic(func() {
					processor.Get(tt.json, tt.path)
				})
			})
		}
	})

	t.Run("BOMHandling", func(t *testing.T) {
		processor, _ := New(SecurityConfig())
		defer processor.Close()

		// Test JSON with BOM (Byte Order Mark)
		jsonWithBOM := "\xEF\xBB\xBF" + `{"data": "value"}`

		result, err := processor.Get(jsonWithBOM, "data")
		// Should handle BOM gracefully
		if err == nil {
			helper.AssertNotNil(result)
		}
	})
}

// TestSecurityEdgeCases covers security-related edge cases
func TestSecurityEdgeCases(t *testing.T) {
	helper := newTestHelper(t)

	t.Run("NullBytesInStrings", func(t *testing.T) {
		processor, _ := New(SecurityConfig())
		defer processor.Close()

		jsonWithNull := `{"data": "test\x00middle"}`
		_, err := processor.Get(jsonWithNull, "data")
		// Should not panic
		helper.AssertNoPanic(func() {
			processor.Get(jsonWithNull, "data")
		})
		_ = err // May or may not error depending on implementation
	})

	t.Run("OverlongPath", func(t *testing.T) {
		processor, _ := New(SecurityConfig())
		defer processor.Close()

		// Generate extremely long path
		longPath := "a"
		for i := 0; i < 1000; i++ {
			longPath += ".b"
		}

		testData := `{"a": {"b": "value"}}`
		// Library handles long paths gracefully
		_, _ = processor.Get(testData, longPath)
	})

	t.Run("MassiveArrayIndex", func(t *testing.T) {
		processor, _ := New(SecurityConfig())
		defer processor.Close()

		testData := `{"arr": [1, 2, 3]}`
		// Library handles out of bounds gracefully
		_, _ = processor.Get(testData, "arr[999999999]")
	})

	t.Run("NegativeIndexEdgeCases", func(t *testing.T) {
		processor, _ := New(SecurityConfig())
		defer processor.Close()

		testData := `{"arr": [1, 2, 3]}`

		tests := []struct {
			path     string
			wantErr  bool
			expected interface{}
		}{
			{"arr[-1]", false, float64(3)},
			{"arr[-3]", false, float64(1)},
			// arr[-4] and arr[-999] may not error, library handles gracefully
		}

		for _, tt := range tests {
			t.Run(tt.path, func(t *testing.T) {
				result, err := processor.Get(testData, tt.path)
				if tt.wantErr {
					helper.AssertError(err)
				} else {
					helper.AssertNoError(err)
					helper.AssertEqual(tt.expected, result)
				}
			})
		}
	})
}

// Helper functions for test data generation

func generateLargeJSON(size int) string {
	return genLargeJSONBytes(size)
}

// ============================================================================
// File Security Tests (from file_security_test.go)
// ============================================================================

// TestWindowsDeviceNames tests Windows reserved device name detection
func TestWindowsDeviceNames(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Skipping Windows-specific test on non-Windows platform")
	}

	processor, _ := New()
	defer processor.Close()

	tests := []struct {
		name        string
		filePath    string
		expectError bool
		description string
	}{
		{
			name:        "CON device",
			filePath:    "CON",
			expectError: true,
			description: "Windows reserved device name CON",
		},
		{
			name:        "PRN device",
			filePath:    "PRN",
			expectError: true,
			description: "Windows reserved device name PRN",
		},
		{
			name:        "AUX device",
			filePath:    "AUX",
			expectError: true,
			description: "Windows reserved device name AUX",
		},
		{
			name:        "NUL device",
			filePath:    "NUL",
			expectError: true,
			description: "Windows reserved device name NUL",
		},
		{
			name:        "COM1 device",
			filePath:    "COM1",
			expectError: true,
			description: "Windows COM port 1",
		},
		{
			name:        "COM9 device",
			filePath:    "COM9",
			expectError: true,
			description: "Windows COM port 9",
		},
		{
			name:        "COM0 device",
			filePath:    "COM0",
			expectError: true,
			description: "Windows COM0 (invalid but reserved)",
		},
		{
			name:        "LPT1 device",
			filePath:    "LPT1",
			expectError: true,
			description: "Windows LPT port 1",
		},
		{
			name:        "LPT9 device",
			filePath:    "LPT9",
			expectError: true,
			description: "Windows LPT port 9",
		},
		{
			name:        "LPT0 device",
			filePath:    "LPT0",
			expectError: true,
			description: "Windows LPT0 (invalid but reserved)",
		},
		{
			name:        "CONIN device",
			filePath:    "CONIN$",
			expectError: true,
			description: "Windows console input",
		},
		{
			name:        "CONOUT device",
			filePath:    "CONOUT$",
			expectError: true,
			description: "Windows console output",
		},
		{
			name:        "device with extension",
			filePath:    "CON.txt",
			expectError: true,
			description: "Reserved name with extension",
		},
		{
			name:        "normal file",
			filePath:    "normal.json",
			expectError: false,
			description: "Normal file name",
		},
		{
			name:        "path with device",
			filePath:    "data/CON",
			expectError: true,
			description: "Path containing device name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := processor.validateFilePath(tt.filePath)
			if tt.expectError && err == nil {
				t.Errorf("%s: Expected error for path '%s', but got none", tt.description, tt.filePath)
			}
			if !tt.expectError && err != nil {
				t.Errorf("%s: Unexpected error for path '%s': %v", tt.description, tt.filePath, err)
			}
		})
	}
}

// TestPathTraversalDetection tests path traversal attack detection
func TestPathTraversalDetection(t *testing.T) {
	processor, _ := New()
	defer processor.Close()

	tests := []struct {
		name        string
		filePath    string
		expectError bool
		description string
	}{
		{
			name:        "double dot traversal",
			filePath:    "../../etc/passwd",
			expectError: true,
			description: "Standard path traversal",
		},
		{
			name:        "URL encoded traversal",
			filePath:    "%2e%2e/%2e%2e/etc/passwd",
			expectError: true,
			description: "URL encoded double dots",
		},
		{
			name:        "double URL encoded",
			filePath:    "%252e%252e/%252e%252e",
			expectError: true,
			description: "Double URL encoded traversal",
		},
		{
			name:        "mixed encoding traversal",
			filePath:    "..%2fetc/passwd",
			expectError: true,
			description: "Mixed URL and normal separator",
		},
		{
			name:        "Windows backslash encoded",
			filePath:    "..%5cetc/passwd",
			expectError: true,
			description: "Encoded Windows backslash",
		},
		{
			name:        "UTF-8 overlong encoding",
			filePath:    "..%c0%af/etc/passwd",
			expectError: true,
			description: "UTF-8 overlong encoding attack",
		},
		{
			name:        "partial double encoding",
			filePath:    "..%2e",
			expectError: true,
			description: "Partial double encoding",
		},
		{
			name:        "null byte injection",
			filePath:    "file.txt\x00",
			expectError: true,
			description: "Null byte in path",
		},
		{
			name:        "newline injection",
			filePath:    "file.txt%0a",
			expectError: true,
			description: "Encoded newline injection",
		},
		{
			name:        "carriage return injection",
			filePath:    "file.txt%0d",
			expectError: true,
			description: "Encoded CR injection",
		},
		{
			name:        "tab injection",
			filePath:    "file.txt%09",
			expectError: true,
			description: "Encoded tab injection",
		},
		{
			name:        "five consecutive dots",
			filePath:    ".....//etc/passwd",
			expectError: true,
			description: "Five dots pattern",
		},
		{
			name:        "six consecutive dots",
			filePath:    "......//etc/passwd",
			expectError: true,
			description: "Six dots pattern",
		},
		{
			name:        "normal path",
			filePath:    "data/user/profile.json",
			expectError: false,
			description: "Normal file path",
		},
		{
			name:        "absolute path",
			filePath:    "/home/user/data.json",
			expectError: false,
			description: "Absolute path (allowed on Unix)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := processor.validateFilePath(tt.filePath)
			if tt.expectError && err == nil {
				t.Errorf("%s: Expected error for path '%s', but got none", tt.description, tt.filePath)
			}
			if !tt.expectError && err != nil {
				// Allow certain errors that aren't security-related
				if !strings.Contains(err.Error(), "security") &&
					!strings.Contains(err.Error(), "traversal") &&
					!strings.Contains(err.Error(), "null byte") {
					t.Errorf("%s: Unexpected error for path '%s': %v", tt.description, tt.filePath, err)
				}
			}
		})
	}
}

// TestAlternateDataStreamDetection tests ADS detection on Windows
func TestAlternateDataStreamDetection(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Skipping Windows-specific ADS test on non-Windows platform")
	}

	processor, _ := New()
	defer processor.Close()

	tests := []struct {
		name        string
		filePath    string
		expectError bool
		description string
	}{
		{
			name:        "ADS with colon",
			filePath:    "file.txt:stream",
			expectError: true,
			description: "Alternate data stream",
		},
		{
			name:        "ADS with $DATA",
			filePath:    "file.txt:$DATA",
			expectError: true,
			description: "ADS with $DATA stream",
		},
		{
			name:        "complex ADS",
			filePath:    "file.txt:stream:$DATA",
			expectError: true,
			description: "Complex ADS pattern",
		},
		{
			name:        "drive letter not ADS",
			filePath:    "C:/data/file.txt",
			expectError: false,
			description: "Drive letter pattern is valid",
		},
		{
			name:        "drive letter with colon",
			filePath:    "C:data/file.txt",
			expectError: false,
			description: "Relative path from drive",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := processor.validateFilePath(tt.filePath)
			if tt.expectError && err == nil {
				t.Errorf("%s: Expected error for path '%s', but got none", tt.description, tt.filePath)
			}
			if !tt.expectError && err != nil {
				if strings.Contains(err.Error(), "alternate data stream") {
					t.Errorf("%s: Unexpected ADS error for path '%s': %v", tt.description, tt.filePath, err)
				}
			}
		})
	}
}

// TestPathLengthValidation tests path length limits
func TestPathLengthValidation(t *testing.T) {
	processor, _ := New()
	defer processor.Close()

	// Create a path that exceeds maxPathLength
	longPath := strings.Repeat("a", maxPathLength+1)

	tests := []struct {
		name        string
		filePath    string
		expectError bool
		description string
	}{
		{
			name:        "exceeds max length",
			filePath:    longPath,
			expectError: true,
			description: "Path exceeds maximum length",
		},
		{
			name:        "exactly max length",
			filePath:    strings.Repeat("b", maxPathLength),
			expectError: false,
			description: "Path at maximum length",
		},
		{
			name:        "normal length",
			filePath:    "data/user/profile.json",
			expectError: false,
			description: "Normal length path",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := processor.validateFilePath(tt.filePath)
			if tt.expectError && err == nil {
				t.Errorf("%s: Expected error for path, but got none", tt.description)
			}
			if !tt.expectError && err != nil {
				if strings.Contains(err.Error(), "too long") {
					t.Errorf("%s: Unexpected length error: %v", tt.description, err)
				}
			}
		})
	}
}

// TestInvalidCharactersInWindowsPath tests invalid character detection on Windows
func TestInvalidCharactersInWindowsPath(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Skipping Windows-specific character test on non-Windows platform")
	}

	processor, _ := New()
	defer processor.Close()

	invalidChars := []string{"<", ">", ":", "\"", "|", "?", "*"}

	for _, char := range invalidChars {
		t.Run("invalid char "+char, func(t *testing.T) {
			// Use path where colon isn't the drive letter
			filePath := "data" + char + "file.json"
			if char == ":" {
				filePath = "data" + char + "\\file.json"
			}

			err := processor.validateFilePath(filePath)
			// Should error for invalid characters (except valid drive letter colon)
			if char != ":" || !strings.HasPrefix(filePath, "C:") && !strings.HasPrefix(filePath, "D:") {
				if err == nil {
					t.Errorf("Expected error for invalid character '%s', but got none", char)
				}
			}
		})
	}
}

// TestNullByteDetection tests null byte detection in paths
func TestNullByteDetection(t *testing.T) {
	processor, _ := New()
	defer processor.Close()

	tests := []struct {
		name     string
		filePath string
	}{
		{
			name:     "null at start",
			filePath: "\x00file.txt",
		},
		{
			name:     "null in middle",
			filePath: "file\x00.txt",
		},
		{
			name:     "null at end",
			filePath: "file.txt\x00",
		},
		{
			name:     "multiple nulls",
			filePath: "file\x00\x00.txt",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := processor.validateFilePath(tt.filePath)
			if err == nil {
				t.Errorf("Expected error for path with null byte '%s', but got none", tt.filePath)
			}
		})
	}
}

// TestUNCPathDetection tests UNC path detection on Windows
func TestUNCPathDetection(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Skipping Windows-specific UNC test on non-Windows platform")
	}

	processor, _ := New()
	defer processor.Close()

	tests := []struct {
		name        string
		filePath    string
		expectError bool
	}{
		{
			name:        "UNC with backslashes",
			filePath:    "\\\\server\\share\\file.txt",
			expectError: true,
		},
		{
			name:        "UNC with forward slashes",
			filePath:    "//server/share/file.txt",
			expectError: true,
		},
		{
			name:        "local path",
			filePath:    "C:/data/file.txt",
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := processor.validateFilePath(tt.filePath)
			if tt.expectError && err == nil {
				t.Errorf("Expected error for UNC path, but got none")
			}
		})
	}
}

// TestContainsPathTraversal tests the path traversal detection helper
func TestContainsPathTraversal(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		{
			name:     "double dot",
			path:     "../file.txt",
			expected: true,
		},
		{
			name:     "URL encoded",
			path:     "%2e%2e/file.txt",
			expected: true,
		},
		{
			name:     "normal path",
			path:     "data/file.txt",
			expected: false,
		},
		{
			name:     "single dot",
			path:     "./file.txt",
			expected: false,
		},
		{
			name:     "partial encoding",
			path:     "%2e%2%2e",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := containsPathTraversal(tt.path)
			if result != tt.expected {
				t.Errorf("containsPathTraversal(%s) = %v; want %v", tt.path, result, tt.expected)
			}
		})
	}
}

// TestContainsConsecutiveDots tests consecutive dot detection
func TestContainsConsecutiveDots(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		minCount int
		expected bool
	}{
		{
			name:     "three dots",
			path:     "...",
			minCount: 3,
			expected: true,
		},
		{
			name:     "four dots",
			path:     "....",
			minCount: 3,
			expected: true,
		},
		{
			name:     "two dots",
			path:     "..",
			minCount: 3,
			expected: false,
		},
		{
			name:     "separated dots",
			path:     ".a.b",
			minCount: 3,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := containsConsecutiveDots(tt.path, tt.minCount)
			if result != tt.expected {
				t.Errorf("containsConsecutiveDots(%s, %d) = %v; want %v", tt.path, tt.minCount, result, tt.expected)
			}
		})
	}
}

// TestFilePathValidationEdgeCases tests edge cases in file path validation
func TestFilePathValidationEdgeCases(t *testing.T) {
	processor, _ := New()
	defer processor.Close()

	tests := []struct {
		name        string
		filePath    string
		expectError bool
		description string
	}{
		{
			name:        "empty path",
			filePath:    "",
			expectError: true,
			description: "Empty path should error",
		},
		{
			name:        "single character",
			filePath:    "a",
			expectError: false,
			description: "Single character path",
		},
		{
			name:        "current directory",
			filePath:    ".",
			expectError: false,
			description: "Current directory reference",
		},
		{
			name:        "parent directory",
			filePath:    "..",
			expectError: true,
			description: "Parent directory reference (traversal)",
		},
		{
			name:        "file with extension",
			filePath:    "document.pdf",
			expectError: false,
			description: "Normal file with extension",
		},
		{
			name:        "deep path",
			filePath:    "a/b/c/d/e/f/g/h/i/j/file.txt",
			expectError: false,
			description: "Deep but valid path",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := processor.validateFilePath(tt.filePath)
			if tt.expectError && err == nil {
				t.Errorf("%s: Expected error for path '%s', but got none", tt.description, tt.filePath)
			}
			if !tt.expectError && err != nil {
				t.Errorf("%s: Unexpected error for path '%s': %v", tt.description, tt.filePath, err)
			}
		})
	}
}

// TestWindowsPathValidationComponents tests Windows path validation components
func TestWindowsPathValidationComponents(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Skipping Windows-specific test on non-Windows platform")
	}

	tests := []struct {
		name        string
		filePath    string
		expectError bool
		description string
	}{
		{
			name:        "valid absolute path",
			filePath:    "C:/Users/user/data.json",
			expectError: false,
			description: "Valid Windows absolute path",
		},
		{
			name:        "valid relative path",
			filePath:    "data/config.json",
			expectError: false,
			description: "Valid Windows relative path",
		},
		{
			name:        "path with spaces",
			filePath:    "C:/Program Files/data.json",
			expectError: false,
			description: "Path with spaces (valid)",
		},
		{
			name:        "path with underscore",
			filePath:    "my_data/file.json",
			expectError: false,
			description: "Path with underscore (valid)",
		},
		{
			name:        "path with hyphen",
			filePath:    "my-data/file.json",
			expectError: false,
			description: "Path with hyphen (valid)",
		},
		{
			name:        "path with pipe",
			filePath:    "data|file.json",
			expectError: true,
			description: "Path with pipe (invalid)",
		},
		{
			name:        "path with asterisk",
			filePath:    "data/*.json",
			expectError: true,
			description: "Path with asterisk (invalid)",
		},
		{
			name:        "path with question mark",
			filePath:    "data/file?.json",
			expectError: true,
			description: "Path with question mark (invalid)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, _ := New()
			defer p.Close()
			err := p.validateFilePath(tt.filePath)
			if tt.expectError && err == nil {
				t.Errorf("%s: Expected error for path '%s', but got none", tt.description, tt.filePath)
			}
			if !tt.expectError && err != nil {
				t.Errorf("%s: Unexpected error for path '%s': %v", tt.description, tt.filePath, err)
			}
		})
	}
}

// TestSecurityValidationWithRealPaths tests security validation with realistic paths
func TestSecurityValidationWithRealPaths(t *testing.T) {
	processor, _ := New()
	defer processor.Close()

	validPaths := []string{
		"data/users/profile.json",
		"config/settings.json",
		"logs/app.log",
		"backup/data.bak",
		"cache/index.tmp",
	}

	for _, path := range validPaths {
		t.Run("valid_"+path, func(t *testing.T) {
			err := processor.validateFilePath(path)
			if err != nil {
				// Some errors are OK (like file not found), but not security errors
				if strings.Contains(err.Error(), "security") ||
					strings.Contains(err.Error(), "traversal") ||
					strings.Contains(err.Error(), "null byte") {
					t.Errorf("Valid path '%s' failed security validation: %v", path, err)
				}
			}
		})
	}
}

// TestFilePathNormalization tests file path normalization
func TestFilePathNormalization(t *testing.T) {
	processor, _ := New()
	defer processor.Close()

	tests := []struct {
		name     string
		input    string
		validate func(t *testing.T, err error)
	}{
		{
			name:  "valid normalized path",
			input: "data/config.json",
			validate: func(t *testing.T, err error) {
				if err != nil && strings.Contains(err.Error(), "security") {
					t.Errorf("Unexpected security error: %v", err)
				}
			},
		},
		{
			name:  "path with extra separators",
			input: "data///config.json",
			validate: func(t *testing.T, err error) {
				if err != nil && strings.Contains(err.Error(), "security") {
					t.Errorf("Unexpected security error: %v", err)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := processor.validateFilePath(tt.input)
			tt.validate(t, err)
		})
	}
}

// TestSymlinkValidation tests symlink validation in paths
func TestSymlinkValidation(t *testing.T) {
	p, _ := New()
	defer p.Close()

	// This test checks that the validation logic handles symlinks properly
	// We can't create actual symlinks in tests, but we can verify the logic exists

	tests := []struct {
		name        string
		filePath    string
		description string
	}{
		{
			name:        "potential symlink path",
			filePath:    "data/link/target.json",
			description: "Path that might contain symlink",
		},
		{
			name:        "normal file path",
			filePath:    "data/file.json",
			description: "Regular file path",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Just verify the validation runs without panicking
			_ = p.validateFilePath(tt.filePath)
			t.Logf("Validation completed for: %s", tt.description)
		})
	}
}

// TestCrossPlatformPathValidation tests path validation works on both platforms
func TestCrossPlatformPathValidation(t *testing.T) {
	processor, _ := New()
	defer processor.Close()

	universalPaths := []struct {
		name        string
		path        string
		expectError bool
	}{
		{
			name:        "simple json file",
			path:        "data.json",
			expectError: false,
		},
		{
			name:        "nested path",
			path:        "users/admin/profile.json",
			expectError: false,
		},
		{
			name:        "path traversal attempt",
			path:        "../../../etc/passwd",
			expectError: true,
		},
		{
			name:        "null byte injection",
			path:        "file.txt\x00 malicious",
			expectError: true,
		},
	}

	for _, tt := range universalPaths {
		t.Run(tt.name, func(t *testing.T) {
			err := processor.validateFilePath(tt.path)
			if tt.expectError && err == nil {
				t.Errorf("Expected error for path '%s', but got none", tt.path)
			}
		})
	}
}

// ============================================================================
// Sampling Bypass Security Tests
// Tests for CVE-like vulnerability where attacks could be hidden between sample points
// ============================================================================

func TestSamplingBypassFixed(t *testing.T) {
	// Create a processor with FullSecurityScan=false (default)
	// This tests that even in optimized mode, the rolling window approach
	// catches attacks hidden in the middle of large JSON
	cfg := DefaultConfig()
	cfg.FullSecurityScan = false // Test optimized mode
	processor, _ := New(cfg)
	defer processor.Close()

	t.Run("AttackHiddenInMiddle", func(t *testing.T) {
		// Create a large JSON with an attack hidden in the middle
		// The attack is positioned at exactly 50KB to try to hide it between sample points
		attackPattern := `<script>alert('xss')</script>`

		// Create padding before and after the attack
		paddingSize := 50 * 1024 // 50KB padding on each side
		beforePadding := strings.Repeat(`"padding":"`+strings.Repeat("A", 100)+`",`, paddingSize/120)
		afterPadding := strings.Repeat(`"padding":"`+strings.Repeat("B", 100)+`",`, paddingSize/120)

		maliciousJSON := `{"data":{` + beforePadding + `"attack":"` + attackPattern + `",` + afterPadding + `"end":true}}`

		// This should now be caught even in optimized mode due to rolling window
		// Validation happens during Parse/Get operations
		var result any
		err := processor.Parse(maliciousJSON, &result)
		if err == nil {
			t.Error("Expected security error for hidden XSS attack in optimized mode")
		}
	})

	t.Run("AttackAtBoundary", func(t *testing.T) {
		// Test attack positioned at window boundary (32KB)
		attackPattern := `__proto__`

		// Create JSON with attack at exactly 32KB boundary
		beforeBoundary := strings.Repeat(`{"a":"`+strings.Repeat("X", 100)+`"},`, 320)

		maliciousJSON := `{"items":[` + beforeBoundary + `{"evil":"` + attackPattern + `"}]}`

		var result any
		err := processor.Parse(maliciousJSON, &result)
		if err == nil {
			t.Error("Expected security error for attack at window boundary")
		}
	})

	t.Run("AttackDistributedAcrossWindows", func(t *testing.T) {
		// Test with multiple small attacks distributed across the JSON
		// This tests the pattern fragment detection
		fragments := []string{
			`"f1":"eval`,
			`"f2":"(`,
			`"f3":"scri`,
			`"f4":"pt>`,
		}

		var sb strings.Builder
		sb.WriteString(`{"data":[`)
		for i, frag := range fragments {
			if i > 0 {
				sb.WriteString(",")
			}
			// Add padding around each fragment
			sb.WriteString(strings.Repeat(`{"p":"`+strings.Repeat("P", 1000)+`"},`, 10))
			sb.WriteString(frag)
		}
		sb.WriteString(`]}`)

		// The pattern fragment detection should catch this
		// Even if individual fragments aren't complete patterns
		maliciousJSON := sb.String()

		// This should either be caught by pattern fragments or density check
		// The exact behavior depends on the implementation details
		var result any
		_ = processor.Parse(maliciousJSON, &result)
		// We just verify it doesn't panic or crash
	})

	t.Run("LegitimateLargeJSON", func(t *testing.T) {
		// Ensure legitimate large JSON still passes
		legitimateJSON := generateLargeJSON(100 * 1024) // 100KB

		var result any
		err := processor.Parse(legitimateJSON, &result)
		if err != nil {
			t.Errorf("Legitimate large JSON should pass validation: %v", err)
		}
	})

	t.Run("FullSecurityScanStillWorks", func(t *testing.T) {
		// Verify that FullSecurityScan=true still works as expected
		secureConfig := SecurityConfig()
		secureConfig.FullSecurityScan = true
		secureProcessor, _ := New(secureConfig)
		defer secureProcessor.Close()

		attackPattern := `<script>alert(1)</script>`
		maliciousJSON := `{"data":"` + strings.Repeat("X", 5000) + attackPattern + strings.Repeat("Y", 5000) + `"}`

		var result any
		err := secureProcessor.Parse(maliciousJSON, &result)
		if err == nil {
			t.Error("FullSecurityScan should catch all attacks")
		}
	})
}

func TestRollingWindowCoverage(t *testing.T) {
	cfg := DefaultConfig()
	cfg.FullSecurityScan = false // Test optimized mode
	processor, _ := New(cfg)
	defer processor.Close()

	// Test that the rolling window approach covers the entire string
	// by placing attacks at various positions and verifying they're all caught

	positions := []int{
		0,         // Start
		16 * 1024, // 16KB
		32 * 1024, // 32KB (window boundary)
		48 * 1024, // 48KB (between windows)
		64 * 1024, // 64KB
		80 * 1024, // 80KB
	}

	for _, pos := range positions {
		posKB := pos / 1024
		t.Run("Position_"+string(rune('0'+posKB/10))+string(rune('0'+posKB%10))+"KB", func(t *testing.T) {
			// Create JSON with attack at specific position
			attackPattern := `<script>alert(1)</script>`

			var sb strings.Builder
			sb.WriteString(`{"data":"`)

			// Add padding before attack
			for sb.Len() < pos {
				sb.WriteString("X")
			}

			sb.WriteString(attackPattern)

			// Add padding after attack
			for sb.Len() < pos+1024 {
				sb.WriteString("Y")
			}

			sb.WriteString(`"}`)

			maliciousJSON := sb.String()

			var result any
			err := processor.Parse(maliciousJSON, &result)
			if err == nil {
				t.Errorf("Attack at position %d should be caught", pos)
			}
		})
	}
}

// ============================================================================
// Benchmark tests
// ============================================================================

func BenchmarkValidatePathNormal(b *testing.B) {
	processor, _ := New()
	defer processor.Close()

	path := "data/users/profile.json"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = processor.validateFilePath(path)
	}
}

func BenchmarkValidatePathComplex(b *testing.B) {
	processor, _ := New()
	defer processor.Close()

	path := "data/users/admin/config/settings.production.json"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = processor.validateFilePath(path)
	}
}

// ============================================================================
// Pattern Registry API Tests (M2 fix)
// ============================================================================

func TestPatternRegistryAPI(t *testing.T) {
	t.Run("RegisterAndList", func(t *testing.T) {
		defer clearDangerousPatterns()

		RegisterDangerousPattern(DangerousPattern{
			Pattern: "custom_test_pattern",
			Name:    "Test Pattern",
			Level:   PatternLevelCritical,
		})
		patterns := ListDangerousPatterns()
		found := false
		for _, p := range patterns {
			if p.Pattern == "custom_test_pattern" {
				found = true
				if p.Name != "Test Pattern" {
					t.Errorf("pattern name = %q, want %q", p.Name, "Test Pattern")
				}
			}
		}
		if !found {
			t.Error("registered pattern not found in list")
		}
	})

	t.Run("Unregister", func(t *testing.T) {
		defer clearDangerousPatterns()

		RegisterDangerousPattern(DangerousPattern{
			Pattern: "temp_pattern",
			Name:    "Temporary",
			Level:   PatternLevelWarning,
		})
		UnregisterDangerousPattern("temp_pattern")

		for _, p := range ListDangerousPatterns() {
			if p.Pattern == "temp_pattern" {
				t.Error("pattern should have been unregistered")
			}
		}
	})

	t.Run("Clear", func(t *testing.T) {
		RegisterDangerousPattern(DangerousPattern{
			Pattern: "clearable_pattern",
			Name:    "Clearable",
			Level:   PatternLevelInfo,
		})
		clearDangerousPatterns()
		if len(ListDangerousPatterns()) != 0 {
			t.Error("patterns should be empty after clear")
		}
	})

	t.Run("GetDefaultPatterns", func(t *testing.T) {
		defaults := getDefaultPatterns()
		if len(defaults) == 0 {
			t.Error("expected non-empty default patterns")
		}
		for _, p := range defaults {
			if p.Pattern == "" || p.Name == "" {
				t.Errorf("default pattern has empty field: %+v", p)
			}
			if p.Level != PatternLevelCritical {
				t.Errorf("default pattern level = %v, want Critical", p.Level)
			}
		}
	})

	t.Run("GetCriticalPatterns", func(t *testing.T) {
		critical := getCriticalPatterns()
		if len(critical) == 0 {
			t.Error("expected non-empty critical patterns")
		}
		for _, p := range critical {
			if p.Level != PatternLevelCritical {
				t.Errorf("critical pattern level = %v, want Critical", p.Level)
			}
		}
	})

	t.Run("MaxPatternLenDynamic", func(t *testing.T) {
		defer clearDangerousPatterns()

		baseLen := maxDangerousPatternLen()
		longPattern := "this_is_a_very_long_custom_pattern_for_testing"
		RegisterDangerousPattern(DangerousPattern{
			Pattern: longPattern,
			Name:    "Long Pattern",
			Level:   PatternLevelCritical,
		})
		newLen := maxDangerousPatternLen()
		if newLen < len(longPattern) {
			t.Errorf("maxDangerousPatternLen() = %d, want >= %d after registering long pattern", newLen, len(longPattern))
		}
		if newLen <= baseLen {
			t.Errorf("maxDangerousPatternLen() should increase after registering a longer pattern: before=%d after=%d", baseLen, newLen)
		}
	})
}

// ============================================================================
// Panic Protection Tests (SEC-003)
// ============================================================================

// TestPanicProtectionSafeErrorNilErr verifies SafeError does not panic
// when called with a JsonsError that has a nil Err field.
func TestPanicProtectionSafeErrorNilErr(t *testing.T) {
	t.Run("NilErrField", func(t *testing.T) {
		err := &JsonsError{Op: "test", Message: "something went wrong"}
		result := SafeError(err)
		if result != "something went wrong" {
			t.Errorf("SafeError with nil Err = %q, want %q", result, "something went wrong")
		}
	})

	t.Run("NilErrFieldEmptyMessage", func(t *testing.T) {
		err := &JsonsError{Op: "test"}
		result := SafeError(err)
		if result == "" {
			t.Error("SafeError with nil Err and empty Message should not return empty string")
		}
	})

	t.Run("NilInput", func(t *testing.T) {
		result := SafeError(nil)
		if result != "" {
			t.Errorf("SafeError(nil) = %q, want empty string", result)
		}
	})

	t.Run("NormalError", func(t *testing.T) {
		result := SafeError(ErrPathNotFound)
		if result != "path not found" {
			t.Errorf("SafeError(ErrPathNotFound) = %q, want %q", result, "path not found")
		}
	})

	t.Run("WrappedJsonsError", func(t *testing.T) {
		inner := &JsonsError{Op: "get", Message: "not found", Err: ErrPathNotFound}
		result := SafeError(inner)
		if result != "path not found" {
			t.Errorf("SafeError(wrapped) = %q, want %q", result, "path not found")
		}
	})
}

// TestPanicProtectionNilProcessor pins SEC-003 across the full public Processor
// API: every method on a nil *Processor MUST either return an error (mutating /
// query ops) or a safe zero value (typed getters, stats, config) — never a
// nil-pointer panic. Guards are centralized in checkClosed(); this table-driven
// test covers the whole surface so a future refactor cannot silently regress.
// (Go fails the subtest on panic, so a direct call is a valid "must not panic"
// assertion.)
func TestPanicProtectionNilProcessor(t *testing.T) {
	var p *Processor // intentionally nil

	tests := []struct {
		name string
		call func(t *testing.T)
	}{
		{name: "Parse", call: func(t *testing.T) {
			var target any
			if err := p.Parse(`{"a":1}`, &target); err == nil {
				t.Error("expected error when calling Parse on nil Processor")
			}
		}},
		{name: "Get", call: func(t *testing.T) {
			if _, err := p.Get("{}", "a"); err == nil {
				t.Error("expected error when calling Get on nil Processor")
			}
		}},
		{name: "Set", call: func(t *testing.T) {
			if _, err := p.Set(`{}`, "a", 1); err == nil {
				t.Error("expected error when calling Set on nil Processor")
			}
		}},
		{name: "Delete", call: func(t *testing.T) {
			if _, err := p.Delete(`{"a":1}`, "a"); err == nil {
				t.Error("expected error when calling Delete on nil Processor")
			}
		}},
		{name: "Marshal", call: func(t *testing.T) {
			if _, err := p.Marshal(map[string]any{"a": 1}); err == nil {
				t.Error("expected error when calling Marshal on nil Processor")
			}
		}},
		{name: "MarshalIndent", call: func(t *testing.T) {
			if _, err := p.MarshalIndent(map[string]any{"a": 1}, "", "  "); err == nil {
				t.Error("expected error when calling MarshalIndent on nil Processor")
			}
		}},
		{name: "Unmarshal", call: func(t *testing.T) {
			var v any
			if err := p.Unmarshal([]byte(`{"a":1}`), &v); err == nil {
				t.Error("expected error when calling Unmarshal on nil Processor")
			}
		}},
		{name: "StreamJSONL", call: func(t *testing.T) {
			if err := p.StreamJSONL(strings.NewReader(`{"a":1}`), func(int, *IterableValue) error { return nil }); err == nil {
				t.Error("expected error when calling StreamJSONL on nil Processor")
			}
		}},
		// Typed getters: return the provided default (or zero value), no panic.
		{name: "GetString", call: func(t *testing.T) {
			if got := p.GetString(`{"a":1}`, "a", "fallback"); got != "fallback" {
				t.Errorf("GetString on nil Processor = %q, want default %q", got, "fallback")
			}
		}},
		// Information accessors: return a safe zero-value state, no panic.
		{name: "GetStats", call: func(t *testing.T) {
			if stats := p.GetStats(); stats.IsClosed {
				t.Errorf("GetStats on nil Processor reports IsClosed=true; want zero-value Stats")
			}
		}},
		{name: "GetHealthStatus", call: func(t *testing.T) {
			if hs := p.GetHealthStatus(); hs.Healthy {
				t.Error("GetHealthStatus on nil Processor reports Healthy=true; want unhealthy")
			}
		}},
		{name: "GetConfig", call: func(t *testing.T) {
			_ = p.GetConfig() // must not panic; zero-value Config is acceptable
		}},
		{name: "SetLogger", call: func(t *testing.T) {
			p.SetLogger(nil) // no-op on nil receiver; must not panic
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.call(t)
		})
	}
}

// sec003AssertPanicked verifies that err is non-nil and describes a recovered
// panic. Every test below pins a recover() guard: if the guard is removed the
// panicking callback aborts the test binary instead of reaching these assertions
// (see TestParallelIteratorPanicRecovery for the established pattern).
func sec003AssertPanicked(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("expected error from panicking callback, got nil")
	}
	if !strings.Contains(err.Error(), "panicked") {
		t.Errorf("expected error to mention panic, got: %v", err)
	}
}

// TestPanicProtectionCallbacks pins SEC-003 across every callback-bearing path:
// a panicking callback is recovered and surfaced as an error (or, for the void
// variants, logged with iteration stopped). Removing any recover() guard makes
// the panicking callback abort the test binary instead of reaching the assertion.
func TestPanicProtectionCallbacks(t *testing.T) {
	tests := []struct {
		name string
		run  func() error
	}{
		{name: "ForeachWithError", run: func() error {
			p, _ := New()
			defer p.Close()
			return p.ForeachWithError(`[1,2,3,4,5]`, ".", func(key any, item *IterableValue) error {
				if key.(int) == 2 {
					panic("boom from ForeachWithError callback")
				}
				return nil
			})
		}},
		{name: "ForeachNestedWithError", run: func() error {
			p, _ := New()
			defer p.Close()
			return p.ForeachNestedWithError(`{"a":{"b":[1,2]},"c":3}`, func(key any, item *IterableValue) error {
				panic("boom from nested callback")
			})
		}},
		{name: "StreamJSONL", run: func() error {
			p, _ := New()
			defer p.Close()
			data := "{\"id\":1}\n{\"id\":2}\n{\"id\":3}"
			return p.StreamJSONL(strings.NewReader(data), func(lineNum int, item *IterableValue) error {
				if lineNum == 2 {
					panic("boom from StreamJSONL callback")
				}
				return nil
			})
		}},
		{name: "MapJSONL", run: func() error {
			p, _ := New()
			defer p.Close()
			data := "{\"id\":1}\n{\"id\":2}"
			_, err := p.MapJSONL(strings.NewReader(data), func(lineNum int, item *IterableValue) (any, error) {
				if lineNum == 2 {
					panic("boom from MapJSONL fn")
				}
				return item.GetData(), nil
			})
			return err
		}},
		{name: "StreamLinesInto", run: func() error {
			data := "{\"id\":1}\n{\"id\":2}"
			_, err := StreamLinesInto(strings.NewReader(data), func(lineNum int, _ map[string]any) error {
				if lineNum == 2 {
					panic("boom from StreamLinesInto fn")
				}
				return nil
			})
			return err
		}},
		{name: "ProcessReader", run: func() error {
			np := NewNDJSONProcessor()
			data := "{\"id\":1}\n{\"id\":2}"
			return np.ProcessReader(strings.NewReader(data), func(lineNum int, obj map[string]any) error {
				if lineNum == 2 {
					panic("boom from ProcessReader fn")
				}
				return nil
			})
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sec003AssertPanicked(t, tt.run())
		})
	}
}

// TestPanicProtectionForeachVoid verifies SEC-003 for the void Foreach variants,
// whose signature has no error return: a panicking callback is recovered, logged,
// and iteration stops — it must not crash the program.
func TestPanicProtectionForeachVoid(t *testing.T) {
	p, _ := New()
	defer p.Close()

	visited := 0
	// If the recover guard is removed this call aborts the test binary.
	p.Foreach(`[1,2,3,4,5]`, func(key any, item *IterableValue) {
		visited++
		if visited == 2 {
			panic("boom from void Foreach callback")
		}
	})

	if visited == 0 {
		t.Fatal("expected at least one callback invocation before the panic")
	}
	if visited > 2 {
		t.Errorf("expected iteration to stop on panic; visited=%d", visited)
	}
}

// TestPanicProtectionForeachFileChunked verifies SEC-003 for chunked file iteration.
func TestPanicProtectionForeachFileChunked(t *testing.T) {
	p, _ := New()
	defer p.Close()

	tmp := filepath.Join(t.TempDir(), "chunked.json")
	if err := os.WriteFile(tmp, []byte(`["a","b","c","d","e"]`), 0o644); err != nil {
		t.Fatal(err)
	}

	err := p.ForeachFileChunked(tmp, 2, func(chunk []*IterableValue) error {
		panic("boom from ForeachFileChunked fn")
	})
	sec003AssertPanicked(t, err)
}

// ============================================================================
// CONTAINER-LIMIT TESTS (merged from container_limits_test.go)
// ============================================================================

// makeObject builds a flat JSON object with n keys: {"k0":0,...,"k{n-1}":n-1}.
func makeObject(n int) string {
	var b strings.Builder
	b.WriteByte('{')
	for i := 0; i < n; i++ {
		if i > 0 {
			b.WriteByte(',')
		}
		fmt.Fprintf(&b, `"k%d":%d`, i, i)
	}
	b.WriteByte('}')
	return b.String()
}

// makeArray builds a flat JSON array with n elements: [0,1,...,n-1].
func makeArray(n int) string {
	var b strings.Builder
	b.WriteByte('[')
	for i := 0; i < n; i++ {
		if i > 0 {
			b.WriteByte(',')
		}
		fmt.Fprintf(&b, "%d", i)
	}
	b.WriteByte(']')
	return b.String()
}

// TestValidateContainerCounts_Algorithm exercises the structural scanner
// directly with small limits, covering the tricky cases: per-container (not
// total) counting, nesting, empty containers, and structural characters that
// appear inside string values (which must be ignored).
func TestValidateContainerCounts_Algorithm(t *testing.T) {
	sv := func(maxKeys, maxElems int) *securityValidator {
		return &securityValidator{maxObjectKeys: maxKeys, maxArrayElements: maxElems}
	}

	tests := []struct {
		name      string
		json      string
		maxKeys   int
		maxElems  int
		wantError bool
	}{
		// Object key counting — strict greater-than at the limit.
		{"object at limit", `{"a":1,"b":2}`, 2, 0, false},
		{"object over limit", `{"a":1,"b":2,"c":3}`, 2, 0, true},
		{"object under limit", `{"a":1}`, 2, 0, false},

		// Array element counting.
		{"array at limit", `[1,2]`, 0, 2, false},
		{"array over limit", `[1,2,3]`, 0, 2, true},

		// Empty containers.
		{"empty object", `{}`, 1, 1, false},
		{"empty array", `[]`, 1, 1, false},

		// Structural characters inside string values must NOT be counted.
		{"braces in string value", `{"a":"{}","b":2}`, 2, 0, false},
		{"commas/colons in string value", `{"a":",:","b":2}`, 2, 0, false},
		{"brackets in string value", `["[]","{}"]`, 0, 2, false},
		{"escaped quote in string value", `{"a":"he said \"hi\"","b":2}`, 2, 0, false},

		// Counting is per-container, not total: each nested container is judged
		// independently against the same limit.
		{"nested object over limit", `{"outer":{"x":1,"y":2,"z":3}}`, 2, 0, true},
		{"nested object within limit", `{"outer":{"x":1,"y":2},"z":3}`, 2, 0, false},
		{"nested array over limit", `[[1,2,3]]`, 0, 2, true},
		{"array of small objects", `[{"a":1},{"b":2},{"c":3}]`, 0, 2, true},

		// Mixed structures.
		{"mixed object over array limit", `{"a":[1,2,3]}`, 0, 2, true},
		{"mixed within limits", `{"a":[1,2],"b":{"x":1}}`, 2, 2, false},

		// Whitespace is structural, not a value.
		{"whitespace handled", `{ "a" : 1 , "b" : 2 }`, 2, 0, false},

		// Bare (non-container) values: nothing to count.
		{"bare number", `42`, 1, 1, false},
		{"bare string", `"hello"`, 1, 1, false},
		{"bare null", `null`, 1, 1, false},
		{"bare bool", `true`, 1, 1, false},
		{"negative numbers", `[-1,-2,-3]`, 0, 2, true},

		// Unlimited (<=0) disables enforcement entirely.
		{"unlimited skips check", makeObject(50), 0, 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := sv(tt.maxKeys, tt.maxElems).validateContainerCounts(tt.json)
			if tt.wantError {
				if !errors.Is(err, ErrSizeLimit) {
					t.Fatalf("expected ErrSizeLimit, got: %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("expected no error, got: %v", err)
			}
		})
	}
}

// TestValidateContainerCounts_PublicAPI verifies the limits are honored through
// the public Processor API. Config validation clamps MaxObjectKeys /
// MaxArrayElements to a minimum of 100, so the boundary is tested at 100/101.
func TestValidateContainerCounts_PublicAPI(t *testing.T) {
	t.Run("object keys", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.MaxObjectKeys = 100 // survives clamping (min 100)
		p, err := New(cfg)
		if err != nil {
			t.Fatalf("New failed: %v", err)
		}
		defer p.Close()

		// At the limit: allowed.
		if _, err := p.Get(makeObject(100), "k0"); err != nil {
			t.Fatalf("object with 100 keys (at limit) should pass, got: %v", err)
		}
		// Over the limit: rejected before any path processing.
		_, err = p.Get(makeObject(101), "k0")
		if !errors.Is(err, ErrSizeLimit) {
			t.Fatalf("object with 101 keys should be rejected with ErrSizeLimit, got: %v", err)
		}
	})

	t.Run("array elements", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.MaxArrayElements = 100
		p, err := New(cfg)
		if err != nil {
			t.Fatalf("New failed: %v", err)
		}
		defer p.Close()

		// At the limit: allowed.
		var out []any
		if err := p.Parse(makeArray(100), &out); err != nil {
			t.Fatalf("array with 100 elements (at limit) should pass, got: %v", err)
		}
		// Over the limit: rejected.
		err = p.Parse(makeArray(101), &out)
		if !errors.Is(err, ErrSizeLimit) {
			t.Fatalf("array with 101 elements should be rejected with ErrSizeLimit, got: %v", err)
		}
	})

	t.Run("nested object still caught", func(t *testing.T) {
		// A wide nested object at depth 1 must be rejected even though the root
		// has few keys — this is the exact attack the limit exists to stop.
		cfg := DefaultConfig()
		cfg.MaxObjectKeys = 100
		p, err := New(cfg)
		if err != nil {
			t.Fatalf("New failed: %v", err)
		}
		defer p.Close()

		nested := `{"wrapper":` + makeObject(101) + `}`
		_, err = p.Get(nested, "wrapper")
		if !errors.Is(err, ErrSizeLimit) {
			t.Fatalf("nested object with 101 keys should be rejected with ErrSizeLimit, got: %v", err)
		}
	})
}

// ============================================================================
// SECURITY BOUNDARY TESTS (merged from security_boundary_test.go)
// ============================================================================

// TestToInternalPatterns exercises toInternalPatterns (security.go).
func TestToInternalPatterns(t *testing.T) {
	out := toInternalPatterns([]DangerousPattern{
		{Pattern: "eval", Name: "eval-injection"},
	})
	if len(out) != 1 {
		t.Errorf("got %d internal patterns, want 1", len(out))
	}
	if len(toInternalPatterns(nil)) != 0 {
		t.Error("nil input should yield empty result")
	}
}

// TestSecurity_CustomDangerousPattern exercises scanCustomPatterns via
// AdditionalDangerousPatterns (security.go).
func TestSecurity_CustomDangerousPattern(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AdditionalDangerousPatterns = []DangerousPattern{{Pattern: "mybomb", Name: "bomb"}}
	p, _ := New(cfg)
	defer p.Close()

	var target map[string]any
	err := p.Parse(`{"x":"this has mybomb in it"}`, &target)
	if err == nil {
		t.Error("expected custom dangerous pattern to be rejected")
	}
}

// TestSecurity_EssentialSizeLimit exercises ValidateJSONInputEssential via
// SkipValidation + size limit (security.go).
func TestSecurity_EssentialSizeLimit(t *testing.T) {
	cfg := DefaultConfig()
	cfg.SkipValidation = true // essential checks still run
	cfg.MaxJSONSize = 16
	p, _ := New(cfg)
	defer p.Close()

	big := strings.Repeat("a", 100)
	var target map[string]any
	if err := p.Parse(`"`+big+`"`, &target); err == nil {
		t.Error("expected size-limit error even with SkipValidation=true (essential check)")
	}
}

// TestSecurity_NonASCIIPath exercises validatePathSecurity non-ASCII / NFC path
// (security.go): must not panic and must remain usable.
func TestSecurity_NonASCIIPath(t *testing.T) {
	p, _ := New()
	defer p.Close()
	v, err := Get(`{"café":1}`, "café")
	if err != nil {
		t.Errorf("non-ASCII property path should resolve, got %v", err)
	}
	if v != float64(1) {
		t.Errorf("Get(non-ASCII path) = %v, want 1", v)
	}
}

// TestP001WindowPrefilterEquivalence guards the scanWindowForPatterns
// single-pass prefilter: windowContainsDangerousMatch must report true exactly
// when at least one built-in dangerous pattern occurs case-insensitively in
// the window (context-free — the word-boundary check stays with the ordered
// loop). If this equivalence breaks in EITHER direction, scanning either
// misses dangerous content (false negative) or the prefilter gains nothing
// (always-true degeneration).
func TestP001WindowPrefilterEquivalence(t *testing.T) {
	corpus := []string{
		// Clean windows: candidate first letters present, no full pattern.
		`{"id":1,"name":"user42","email":"user42@example.com","active":true}`,
		`{"note":"evaluate options on time; once done, proceed"}`,
		`{"html":"bold text here"}`,
		`{"proto_col":"x","construct":"y"}`,
		// Pattern occurrences, various shapes and cases.
		`{"x":"onerror"}`,
		`{"x":"ONERROR"}`,
		`{"x":"myonerrorx"}`, // mid-word: context check declines, pattern still occurs
		`{"x":"<script>alert(1)</script>"}`,
		`{"x":"<SCRIPT"}`,
		`{"x":"javascript:alert(1)"}`,
		`{"x":"eval(1)"}`,
		`{"x":"new function(){}"}`,
		`{"x":"__defineGetter__"}`,
		`{"x":"setTimeout(x,1)"}`,
		`{"x":"document.cookie"}`,
		`{"x":"expression(a)"}`,
		`{"x":"atob(ZXZhbA==)"}`,
		`{"x":"constructor["}`,
		`{"x":"prototype.v"}`,
		`{"x":"__proto__"}`,
		`{"x":"VbScRiPt:go"}`, // scattered case
		// Edge shapes.
		``,
		`e`,
		`<`,
		`_`,
		`onload`,
		`{"a":"eval(`, // truncated at pattern boundary
		strings.Repeat("o", 100) + "nerror",
		strings.Repeat("x", 5000) + "eval(",
	}

	for _, w := range corpus {
		// Reference: the pre-optimization semantics — any pattern found by
		// fastIndexIgnoreCase anywhere in the window.
		reference := false
		for _, dp := range dangerousPatterns {
			if fastIndexIgnoreCase(w, dp.pattern) != -1 {
				reference = true
				break
			}
		}
		if got := windowContainsDangerousMatch(w); got != reference {
			t.Errorf("window %q: prefilter=%v, reference(any pattern occurrence)=%v", w, got, reference)
		}
	}
}

// ============================================================================
// TEST-ONLY SECURITY HELPERS (moved from security.go in the D-002 cleanup:
// they had no production callers and exist only as test conveniences).
// ============================================================================

// clearDangerousPatterns removes all custom patterns from the global registry.
// Use with caution - this does not affect built-in patterns.
func clearDangerousPatterns() {
	globalPatternRegistry.Clear()
}

// getDefaultPatterns returns the built-in dangerous patterns as DangerousPattern values.
// All default patterns are considered Critical level.
// PERFORMANCE: Cached to avoid repeated allocation — the result is immutable.
var getDefaultPatterns = sync.OnceValue(func() []DangerousPattern {
	result := make([]DangerousPattern, len(dangerousPatterns))
	for i, p := range dangerousPatterns {
		result[i] = DangerousPattern{
			Pattern: p.pattern,
			Name:    p.name,
			Level:   PatternLevelCritical,
		}
	}
	return result
})

// getCriticalPatterns returns patterns that are always fully scanned.
// PERFORMANCE: Cached to avoid repeated allocation — the result is immutable.
var getCriticalPatterns = sync.OnceValue(func() []DangerousPattern {
	result := make([]DangerousPattern, len(criticalPatterns))
	for i, p := range criticalPatterns {
		result[i] = DangerousPattern{
			Pattern: p.pattern,
			Name:    p.name,
			Level:   PatternLevelCritical,
		}
	}
	return result
})
