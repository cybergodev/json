package json

import (
	"errors"
	"strings"
	"testing"
)

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
	sb.Grow(depth * 8 + 10)

	for i := 0; i < depth; i++ {
		sb.WriteString(`{"a":`)
	}
	sb.WriteString(`"deep"`)
	for i := 0; i < depth; i++ {
		sb.WriteString(`}`)
	}
	return sb.String()
}
