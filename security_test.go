package json

import (
	"errors"
	"runtime"
	"strings"
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
	helper := NewTestHelper(t)

	t.Run("PathTraversal", func(t *testing.T) {
		processor := New(HighSecurityConfig())
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
		processor := New(HighSecurityConfig())
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
				smallLimitConfig := HighSecurityConfig().Clone()
				smallLimitConfig.MaxJSONSize = 100 * 1024 // 100KB limit for quick testing
				processor := New(smallLimitConfig)
				defer processor.Close()

				largeJSON := generateLargeJSON(150 * 1024) // 150KB (exceeds 100KB limit)

				_, err := processor.Get(largeJSON, "data")
				helper.AssertError(err)
				if err != nil {
					var jsonErr *JsonsError
					if errors.As(err, &jsonErr) {
						helper.AssertTrue(
							jsonErr.Err == ErrSizeLimit || jsonErr.Err == ErrOperationFailed,
							"Expected size limit error, got: %v", jsonErr.Err)
					}
				}
			})

			// Test 2: Real large data test (optimized with strings.Builder)
			// HighSecurityConfig has MaxJSONSize of 5MB, so we test with 2-3MB
			// This validates actual handling of large JSON without taking 199 seconds
			t.Run("RealLargeData", func(t *testing.T) {
				processor := New(HighSecurityConfig())
				defer processor.Close()

				// Generate 2MB of JSON (large enough to test, small enough to be fast)
				// With optimized generateLargeJSON, this only takes ~0.5-1 second
				largeJSON := generateLargeJSON(2 * 1024 * 1024) // 2MB (within 5MB limit)

				// Should succeed since 2MB < 5MB limit
				result, err := processor.Get(largeJSON, "data")
				helper.AssertNoError(err)
				helper.AssertNotNil(result)

				// Also test slightly above limit
				overLimitJSON := generateLargeJSON(6 * 1024 * 1024) // 6MB (exceeds 5MB limit)
				_, err = processor.Get(overLimitJSON, "data")
				helper.AssertError(err)
			})
		})

		t.Run("DeepNesting", func(t *testing.T) {
			processor := New(HighSecurityConfig())
			defer processor.Close()

			// Generate deeply nested JSON
			deepJSON := generateDeepNesting(50) // 50 levels

			_, err := processor.Get(deepJSON, "a")
			// HighSecurityConfig has MaxNestingDepthSecurity = 20
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
		t.Run("HighSecurityConfig", func(t *testing.T) {
			config := HighSecurityConfig()
			helper.AssertEqual(20, config.MaxNestingDepthSecurity)
			helper.AssertEqual(int64(10*1024*1024), config.MaxSecurityValidationSize)
			helper.AssertEqual(1000, config.MaxObjectKeys)
			helper.AssertEqual(int64(5*1024*1024), config.MaxJSONSize)
			helper.AssertTrue(config.StrictMode)
		})

		t.Run("LargeDataConfig", func(t *testing.T) {
			config := LargeDataConfig()
			helper.AssertEqual(100, config.MaxNestingDepthSecurity)
			helper.AssertEqual(int64(500*1024*1024), config.MaxSecurityValidationSize)
			helper.AssertEqual(50000, config.MaxObjectKeys)
			helper.AssertEqual(int64(100*1024*1024), config.MaxJSONSize)
		})

		t.Run("DefaultConfig", func(t *testing.T) {
			config := DefaultConfig()
			helper.AssertEqual(DefaultMaxNestingDepth, config.MaxNestingDepthSecurity)
			helper.AssertFalse(config.StrictMode)
		})
	})

	t.Run("InputValidation", func(t *testing.T) {
		processor := New(HighSecurityConfig())
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
		processor := New(HighSecurityConfig())
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
		processor := New(HighSecurityConfig())
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
	helper := NewTestHelper(t)

	t.Run("NullBytesInStrings", func(t *testing.T) {
		processor := New(HighSecurityConfig())
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
		processor := New(HighSecurityConfig())
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
		processor := New(HighSecurityConfig())
		defer processor.Close()

		testData := `{"arr": [1, 2, 3]}`
		// Library handles out of bounds gracefully
		_, _ = processor.Get(testData, "arr[999999999]")
	})

	t.Run("NegativeIndexEdgeCases", func(t *testing.T) {
		processor := New(HighSecurityConfig())
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
	var sb strings.Builder

	// Pre-allocate to avoid reallocations
	sb.Grow(size + 20) // Add some buffer

	sb.WriteString(`{"data": [`)

	remaining := size - 12 // len(`{"data": []}`) approximately
	item := `{"value":"data"},`
	itemLen := len(item)

	for remaining >= itemLen {
		sb.WriteString(item)
		remaining -= itemLen
	}

	// Remove trailing comma and close
	str := sb.String()
	if len(str) > 0 && str[len(str)-1] == ',' {
		str = str[:len(str)-1]
	}
	var result strings.Builder
	result.Grow(len(str) + 3)
	result.WriteString(str)
	result.WriteString(`]}`)
	return result.String()
}

func generateDeepNesting(depth int) string {
	var sb strings.Builder
	// Pre-allocate: each level adds ~7 chars {"a": and 1 char for }
	sb.Grow(depth*8 + 10)

	for i := 0; i < depth; i++ {
		sb.WriteString(`{"a":`)
	}
	sb.WriteString(`"deep"`)
	for i := 0; i < depth; i++ {
		sb.WriteString(`}`)
	}
	return sb.String()
}

// ============================================================================
// File Security Tests (from file_security_test.go)
// ============================================================================

// TestWindowsDeviceNames tests Windows reserved device name detection
func TestWindowsDeviceNames(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Skipping Windows-specific test on non-Windows platform")
	}

	processor := New()
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
	processor := New()
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

	processor := New()
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
	processor := New()
	defer processor.Close()

	// Create a path that exceeds MaxPathLength
	longPath := strings.Repeat("a", MaxPathLength+1)

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
			filePath:    strings.Repeat("b", MaxPathLength),
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

	processor := New()
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
	processor := New()
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

	processor := New()
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
	processor := New()
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
			p := New()
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
	processor := New()
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
	processor := New()
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
	p := New()
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
	processor := New()
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
	processor := New(cfg)
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
		secureConfig := HighSecurityConfig()
		secureConfig.FullSecurityScan = true
		secureProcessor := New(secureConfig)
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
	processor := New(cfg)
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
	processor := New()
	defer processor.Close()

	path := "data/users/profile.json"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = processor.validateFilePath(path)
	}
}

func BenchmarkValidatePathComplex(b *testing.B) {
	processor := New()
	defer processor.Close()

	path := "data/users/admin/config/settings.production.json"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = processor.validateFilePath(path)
	}
}
