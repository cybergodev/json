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

			// SecurityConfig caps nesting at 30, so 50 levels must be rejected.
			_, err := processor.Get(deepJSON, "a")
			helper.AssertError(err)
			if !errors.Is(err, ErrDepthLimit) {
				t.Errorf("50-deep JSON under SecurityConfig: expected ErrDepthLimit, got %v", err)
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
				// Interpreted literal: real invalid UTF-8 bytes, not the
				// four ASCII characters \xFF\xFE.
				json: "{\"invalid\": \"test\xFF\xFE\"}",
				path: "invalid",
			},
		}

		for _, tt := range unicodeTests {
			t.Run(tt.name, func(t *testing.T) {
				// Valid Unicode round-trips without error; invalid UTF-8
				// sequences are rejected by input validation.
				_, err := processor.Get(tt.json, tt.path)
				if tt.name == "InvalidSequence" {
					helper.AssertError(err)
				} else {
					helper.AssertNoError(err)
				}
			})
		}
	})

	t.Run("BOMHandling", func(t *testing.T) {
		processor, _ := New(SecurityConfig())
		defer processor.Close()

		// Test JSON with BOM (Byte Order Mark)
		jsonWithBOM := "\xEF\xBB\xBF" + `{"data": "value"}`

		// A leading BOM is rejected, not silently stripped.
		_, err := processor.Get(jsonWithBOM, "data")
		helper.AssertError(err)
	})
}

// TestSecurityEdgeCases covers security-related edge cases
func TestSecurityEdgeCases(t *testing.T) {
	helper := newTestHelper(t)

	t.Run("NullBytesInStrings", func(t *testing.T) {
		processor, _ := New(SecurityConfig())
		defer processor.Close()

		// Interpreted literal: a real NUL byte inside a string value.
		jsonWithNull := "{\"data\": \"test\x00middle\"}"
		_, err := processor.Get(jsonWithNull, "data")
		if err == nil {
			t.Error("NUL byte inside a string value: expected rejection, got nil error")
		}
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
		// The over-long path must be rejected, not silently navigated.
		if _, err := processor.Get(testData, longPath); err == nil {
			t.Error("1001-segment path: expected rejection, got nil error")
		}
	})

	t.Run("MassiveArrayIndex", func(t *testing.T) {
		processor, _ := New(SecurityConfig())
		defer processor.Close()

		testData := `{"arr": [1, 2, 3]}`
		// The index is beyond the reasonable-range guard and is rejected as
		// an invalid path rather than silently resolving to nil.
		v, err := processor.Get(testData, "arr[999999999]")
		if err == nil {
			t.Error("massive array index: expected rejection, got nil error")
		}
		if v != nil {
			t.Errorf("massive array index: got %v, want nil", v)
		}
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

		// The malformed fragment values also make this invalid JSON, so the
		// parse must fail either way — never a silent success or a panic.
		var result any
		if err := processor.Parse(maliciousJSON, &result); err == nil {
			t.Error("distributed fragment payload: expected rejection, got nil error")
		}
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

// panickingPathParser implements PathParser by panicking, pinning the
// parsePathGuarded recover in path.go / processor_cache.go.
type panickingPathParser struct{}

func (panickingPathParser) ParsePath(path string) ([]PathSegment, error) {
	panic("boom from CustomPathParser")
}

// panickingValidator implements Validator by panicking, pinning the
// validationChain recover in interfaces.go.
type panickingValidator struct{}

func (panickingValidator) Validate(jsonStr string) error {
	panic("boom from Validator")
}

// panickingHook implements Hook with a panicking Before, pinning the
// hookChain.executeBefore recover in interfaces.go.
type panickingHook struct{}

func (panickingHook) Before(HookContext) error { panic("boom from Hook.Before") }

func (panickingHook) After(_ HookContext, result any, err error) (any, error) {
	return result, err
}

// TestPanicProtectionExtensionPoints pins SEC-003 across the user-implemented
// extension interfaces: a panicking CustomPathParser, Validator, or Hook is
// recovered and surfaced as an error (or, for the chain, stops execution)
// rather than crashing the program.
func TestPanicProtectionExtensionPoints(t *testing.T) {
	t.Run("CustomPathParser via Get", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.CustomPathParser = panickingPathParser{}
		p, err := New(cfg)
		if err != nil {
			t.Fatal(err)
		}
		defer p.Close()

		_, err = p.Get(`{"a":1}`, "a")
		sec003AssertPanicked(t, err)
	})

	t.Run("CustomPathParser via Set", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.CustomPathParser = panickingPathParser{}
		p, err := New(cfg)
		if err != nil {
			t.Fatal(err)
		}
		defer p.Close()

		_, err = p.Set(`{"a":1}`, "a", 2)
		sec003AssertPanicked(t, err)
	})

	t.Run("validationChain", func(t *testing.T) {
		chain := validationChain{panickingValidator{}}
		sec003AssertPanicked(t, chain.Validate(`{}`))
	})

	t.Run("Hook Before via Get", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.AddHook(panickingHook{})
		p, err := New(cfg)
		if err != nil {
			t.Fatal(err)
		}
		defer p.Close()

		_, err = p.Get(`{"a":1}`, "a")
		sec003AssertPanicked(t, err)
	})
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
// single-pass scanner's existence mode: scanWindowPatterns(w, nil) must
// report true exactly when at least one built-in dangerous pattern occurs
// case-insensitively in the window (context-free — the word-boundary check
// stays with the ordered loop). If this equivalence breaks in EITHER
// direction, scanning either misses dangerous content (false negative) or
// the pass gains nothing (always-true degeneration).
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
		if got := scanWindowPatterns(w, nil); got != reference {
			t.Errorf("window %q: prefilter=%v, reference(any pattern occurrence)=%v", w, got, reference)
		}
	}
}

// TestP003FirstOccurrenceRecording guards the recording mode of
// scanWindowPatterns: the position filed for every built-in pattern must equal
// what a per-pattern fastIndexIgnoreCase scan reports (its FIRST occurrence).
// scanWindowForPatterns builds its ordered reporting loop on these positions,
// so a drift here would change which occurrence the word-context check sees.
func TestP003FirstOccurrenceRecording(t *testing.T) {
	corpus := []string{
		// Clean window: nothing recorded.
		`{"id":1,"name":"user42","note":"evaluate options on time"}`,
		// Single occurrences, several patterns, mixed case.
		`{"x":"ONERROR"}`,
		`{"x":"<script>alert(1)</script>"}`,
		`{"x":"setTimeout(f,10)"}`,
		// Same pattern twice: only the FIRST position may be recorded.
		`{"a":"onerror later","b":"earlier onerror"}`,
		// Position order vs pattern order: "atob(" occurs EARLIER in the
		// window than "__proto__", but both must be recorded independently.
		`{"a":"atob(x)","b":"__proto__"}`,
		// First occurrence in benign word context, second standalone.
		`{"x":"myonerrorx and onerror"}`,
		// Edge shapes.
		``,
		`onload`,
		`{"a":"eval(`,
		strings.Repeat("x", 5000) + "eval(",
	}

	for _, w := range corpus {
		first := make([]int32, len(dangerousPatterns))
		for i := range first {
			first[i] = -1
		}
		scanWindowPatterns(w, first)

		for i, dp := range dangerousPatterns {
			want := fastIndexIgnoreCase(w, dp.pattern)
			if got := int(first[i]); got != want {
				t.Errorf("window %q pattern %q: recorded first=%d, fastIndexIgnoreCase=%d",
					w, dp.pattern, got, want)
			}
		}
	}
}

// TestP003ScanWindowErrorEquivalence pins scanWindowForPatterns to the
// pre-P-003 shape (existence prefilter, then one fastIndexIgnoreCase rescan
// per pattern): both must select the SAME error — same pattern (list order,
// not position order) and same message — or both return nil. GEN-001 P0-3
// closed the historical quirk that only a pattern's FIRST occurrence got the
// word-context check (a benign first occurrence shielded a dangerous second
// one); both shapes now context-check every occurrence.
// TestGEN001_FirstOccurrenceShieldingClosed is a pipeline-level regression
// test for the GEN-001 P0-3 fix: a benign word-internal first occurrence of
// a dangerous pattern must not shield a later standalone occurrence. The
// pre-fix scanner context-checked only the FIRST occurrence per pattern, so
// this payload passed validation.
func TestGEN001_FirstOccurrenceShieldingClosed(t *testing.T) {
	shielded := []string{
		`{"x":"myonerrorx then onerror"}`,
		`{"a":"evaluate it","b":"eval(1)"}`,
		`{"a":"myatobx","b":"atob(ZXZhbA==)"}`,
	}
	for _, in := range shielded {
		var v any
		if err := Unmarshal([]byte(in), &v); err == nil {
			t.Errorf("Unmarshal(%s) = nil error, want security violation", in)
		}
		if Valid([]byte(in)) {
			t.Errorf("Valid(%s) = true, want false", in)
		}
	}
	// Benign word-internal occurrences alone must still pass.
	for _, in := range []string{`{"x":"myonerrorx"}`, `{"x":"evaluate the options"}`} {
		var v any
		if err := Unmarshal([]byte(in), &v); err != nil {
			t.Errorf("Unmarshal(%s) = %v, want nil", in, err)
		}
	}
}

func TestP003ScanWindowErrorEquivalence(t *testing.T) {
	sv := newSecurityValidator(
		100*1024*1024, // maxJSONSize
		1024,          // maxPathLength
		200,           // maxNestingDepth
		false,         // fullSecurityScan
		false,         // disableDefaultPatterns
		false,         // detectDuplicateKeys
		nil,           // additionalPatterns
		100000,        // maxObjectKeys
		100000,        // maxArrayElements
	)
	// scanCustomPatterns reads the global registry live; keep it empty so the
	// comparison isolates the built-in path.
	clearDangerousPatterns()
	defer clearDangerousPatterns()

	legacyErr := func(w string) error {
		if scanWindowPatterns(w, nil) {
			for _, dp := range dangerousPatterns {
				if idx := fastIndexIgnoreCase(w, dp.pattern); idx != -1 {
					// GEN-001 P0-3: every occurrence context-checked — matching
					// the fixed production semantics.
					if sv.indexInDangerousContext(w, dp.pattern, idx) >= 0 {
						return newSecurityError("validate_json_security", fmt.Sprintf("dangerous pattern: %s", dp.name))
					}
				}
			}
		}
		return nil
	}

	corpus := []string{
		// Clean.
		`{"id":1,"note":"plain"}`,
		// Dangerous in context: each must error identically on both paths.
		`{"x":"<script>alert(1)</script>"}`,
		`{"x":"javascript:alert(1)"}`,
		`{"x":"eval(1)"}`,
		`{"x":"__proto__"}`,
		`{"x":"constructor[0]"}`,
		`{"x":"prototype.x"}`,
		`{"x":"document.cookie"}`,
		`{"x":"__defineGetter__"}`,
		// Occurs but word context declines: both paths return nil.
		`{"x":"myonerrorx"}`,
		`{"x":"evaluate the options"}`,
		// GEN-001 P0-3 regression: the benign FIRST occurrence must NOT shield
		// the SECOND standalone one — both paths must now error.
		`{"x":"myonerrorx then onerror"}`,
		// Position vs list order: "atob(" appears before "__proto__", but
		// __proto__ (list index 0) wins the error on both paths.
		`{"a":"atob(x)","b":"__proto__"}`,
		// Case variations.
		`{"x":"OnErRoR=y"}`,
		`{"x":"VbScRiPt:go"}`,
		// Truncated at pattern boundary.
		`{"a":"eval(`,
		// Standalone edge shapes.
		``,
		`onerror`,
		`<svg`,
	}

	for _, w := range corpus {
		want := legacyErr(w)
		got := sv.scanWindowForPatterns(w)

		switch {
		case want == nil && got == nil:
			// agree: benign
		case want == nil || got == nil:
			t.Errorf("window %q: legacy error=%v, new error=%v", w, want, got)
		case want.Error() != got.Error():
			t.Errorf("window %q: legacy error=%q, new error=%q", w, want.Error(), got.Error())
		}
	}
}

// TestP003SensitivePatternScan guards the single-pass sensitive-pattern scan:
// the first-byte-bucketed walk must report true exactly when the previous
// per-pattern strings.Contains loop did (same lowercasing, same existence
// semantics), for benign keys/values, occurrences in any position or case,
// boundary lengths, and strings dense in bucket-first-bytes.
func TestP003SensitivePatternScan(t *testing.T) {
	sv := newSecurityValidator(
		100*1024*1024, 1024, 200,
		false, false, false, nil, 100000, 100000,
	)
	defer sv.Close()

	reference := func(s string) bool {
		if len(s) < minSensitivePatternLen {
			return false
		}
		if !isLowercaseASCII(s) {
			s = strings.ToLower(s)
		}
		for _, p := range sensitivePatterns {
			if strings.Contains(s, p) {
				return true
			}
		}
		return false
	}

	corpus := []string{
		// Benign keys and values.
		"user", "name", "email", "description", "note about nothing special",
		"created_at", "isActive", "plain data 42",
		strings.Repeat("description_", 200),
		// Occurrences: prefix, suffix, mid-word, mixed case, exact pattern.
		"password", "user_password", "PASSWORD", "paSSwordX", "x-password-y",
		"bearer token", "X-API-KEY", "authorization", "social_security_number",
		"aws_secret", "session_id", "creditcard", "jwt", "cvv",
		// Substring traps: benign words CONTAINING a short pattern.
		"pinned", "authentication", "secretsauce", "keyed",
		// Boundary lengths: below/above the shortest pattern (3).
		"s", "cv", "jw", "pwd", "pw",
		// Non-ASCII that lowercases differently than the pattern.
		"PÄSSWORD", "pässword",
		"",
	}

	for _, s := range corpus {
		want := reference(s)
		if got := sv.containsSensitivePatterns(s); got != want {
			t.Errorf("containsSensitivePatterns(%q) = %v, reference(per-pattern loop) = %v", s, got, want)
		}
	}
}

// TestP003StructureLimits pins validateStructureLimits to the pre-P-003
// two-pass shape: nesting scan to completion, then the container scan only
// when nesting passed. The merged single pass must select the SAME error for
// every (validator, document) pair — including the precedence rule that a
// nesting violation beats a container violation even when the container one
// occurs EARLIER in the text, the first-textual-violation order within each
// class, the end-of-scan unbalanced check, and the large-input-only bracket
// anomaly checks (consecutive opens / total brackets, ≥64KB).
func TestP003StructureLimits(t *testing.T) {
	validators := []struct {
		name                string
		maxNestingDepth     int
		maxObjectKeys       int
		maxArrayElements    int
		detectDuplicateKeys bool
	}{
		{"defaults", 200, 100000, 100000, false},
		{"tight-depth", 3, 100000, 100000, false},
		{"tight-keys", 200, 5, 100000, false},
		{"tight-elements", 200, 100000, 5, false},
		{"dup-detection", 200, 100000, 100000, true},
		{"unlimited-containers", 200, -1, -1, false},
		{"zero-depth-defaults-to-100", 0, 100000, 100000, false},
	}

	pad := strings.Repeat(" ", securityNestingValidationThreshold) // pushes docs over 64KB
	oversizedObj := `{"k0":0,"k1":1,"k2":2,"k3":3,"k4":4,"k5":5,"k6":6,"k7":7,"k8":8,"k9":9}`

	docs := []string{
		// Clean documents of various shapes.
		`{}`,
		`[]`,
		`[[[]]]`,
		`{"a":[1,2,{"b":3}],"s":"x\"y\\z","t":true}`,
		`{"brackets":"[not a bracket] {also not}","esc":"\\"quoted\\""}`,
		`  [ 1 , 2 ]  `,
		`"just a string"`,
		`123`,
		`null`,
		// Nesting-class violations.
		strings.Repeat("[", 4),         // exceeds tight-depth(3); unbalanced
		`{"a":{"b":{"c":{"d":1}}}}`,    // depth 4 via objects
		strings.Repeat("[", 101),       // exceeds zero-depth fallback (100)
		pad + strings.Repeat("[", 101), // large path: consecutive opens fire first
		strings.Repeat("[]", 500001),   // large path: total brackets > 1M
		// Unbalanced (end-of-scan, nesting class).
		`[`,
		`]`,
		`{]`,
		`{"a":1]`,
		// Container-class violations.
		oversizedObj,                // 10 keys > tight-keys(5)
		`[1,2,3,4,5,6]`,             // 6 elements > tight-elements(5)
		`{"a":1,"a":2}`,             // duplicate key (dup-detection validator)
		`{"a":1,"a":2,"b":3,"c":4}`, // dup fires before oversized-pop
		// Both classes: container violation textually EARLIER, nesting later —
		// the nesting error must win (the container scan never used to run).
		oversizedObj + strings.Repeat("[", 4),
		// Clean large documents.
		`{"pad":"` + strings.Repeat("a", securityNestingValidationThreshold) + `"}`,
		pad + `[[[[]]]]`,
	}

	for _, v := range validators {
		sv := newSecurityValidator(
			100*1024*1024, 1024, v.maxNestingDepth,
			false, false, v.detectDuplicateKeys, nil,
			v.maxObjectKeys, v.maxArrayElements,
		)
		reference := func(doc string) error {
			if err := sv.validateNestingDepth(doc); err != nil {
				return err
			}
			return sv.validateContainerCounts(doc)
		}

		for _, doc := range docs {
			want := reference(doc)
			got := sv.validateStructureLimits(doc)

			switch {
			case want == nil && got == nil:
				// agree: valid
			case want == nil || got == nil:
				t.Errorf("validator %s doc %.40q: reference=%v, merged=%v", v.name, doc, want, got)
			case want.Error() != got.Error():
				t.Errorf("validator %s doc %.40q: reference=%q, merged=%q", v.name, doc, want.Error(), got.Error())
			}
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

// validateNestingDepth and validateContainerCounts below are the pre-P-003
// two-pass structural scans, preserved verbatim (moved from security.go when
// validateStructureLimits merged them into one pass) as the REFERENCE
// implementation TestP003StructureLimits checks the merged walk against:
// reference = validateNestingDepth to completion, then validateContainerCounts
// only if nesting passed. They have no production callers. Their direct test
// (TestValidateContainerCounts* table) also keeps exercising them unchanged.
func (sv *securityValidator) validateNestingDepth(jsonStr string) error {
	// SECURITY: Validate nesting depth for all inputs regardless of size.
	// Use a faster scan for small JSON (< 64KB) by only checking depth,
	// and full scan for larger inputs that also track total brackets.
	// Small but deeply nested JSON can still cause stack overflow during processing.
	if len(jsonStr) < securityNestingValidationThreshold {
		// Fast path for small JSON: only check max depth, no bracket counting
		depth := 0
		inString := false
		escaped := false
		maxCheckDepth := sv.maxNestingDepth
		if maxCheckDepth <= 0 {
			maxCheckDepth = 100
		}
		for i := 0; i < len(jsonStr); i++ {
			c := jsonStr[i]
			if escaped {
				escaped = false
				continue
			}
			if inString {
				if c == byte(0x5c) {
					escaped = true
				} else if c == '"' {
					inString = false
				}
				continue
			}
			switch c {
			case '"':
				inString = true
			case '{', '[':
				depth++
				if depth > maxCheckDepth {
					return newOperationError("validate_nesting_depth",
						fmt.Sprintf("nesting depth %d exceeds maximum %d", depth, maxCheckDepth), ErrDepthLimit)
				}
			case '}', ']':
				depth--
			}
			// A backslash outside a string (unreachable as an escape marker) is
			// simply ignored, matching the parser's tolerance for stray bytes —
			// malformed JSON is rejected downstream by encoding/json anyway.
		}
		if depth != 0 {
			return newOperationError("validate_nesting_depth",
				"unbalanced brackets in JSON structure", ErrInvalidJSON)
		}
		return nil
	}

	depth := 0
	inString := false
	escaped := false
	maxCheckDepth := sv.maxNestingDepth
	if maxCheckDepth <= 0 {
		maxCheckDepth = 100 // Default max depth
	}

	// SECURITY: Track total bracket count to prevent DoS attacks
	// Attackers can create shallow but massive bracket structures
	// Set limit high enough for normal use but prevent excessive structures
	totalBrackets := 0
	maxTotalBrackets := securityMaxTotalBrackets

	// SECURITY: Track consecutive opening brackets for anomaly detection
	consecutiveOpens := 0
	maxConsecutiveOpens := securityMaxConsecutiveOpens

	// Use byte-level iteration for better performance
	// Check all JSON regardless of size to prevent depth-based attacks
	for i := 0; i < len(jsonStr); i++ {
		c := jsonStr[i]

		if escaped {
			escaped = false
			continue
		}

		switch c {
		case '\\':
			if inString {
				escaped = true
			}
		case '"':
			inString = !inString
		case '{', '[':
			if !inString {
				depth++
				totalBrackets++
				consecutiveOpens++

				// SECURITY: Check for too many consecutive opens (potential attack)
				if consecutiveOpens > maxConsecutiveOpens {
					return newOperationError("validate_nesting_depth",
						fmt.Sprintf("too many consecutive opening brackets at position %d", i), ErrDepthLimit)
				}

				if depth > maxCheckDepth {
					return newOperationError("validate_nesting_depth",
						fmt.Sprintf("nesting depth %d exceeds maximum %d", depth, maxCheckDepth), ErrDepthLimit)
				}

				// SECURITY: Check total bracket count
				if totalBrackets > maxTotalBrackets {
					return newOperationError("validate_nesting_depth",
						fmt.Sprintf("total bracket count %d exceeds maximum %d", totalBrackets, maxTotalBrackets), ErrDepthLimit)
				}
			}
		case '}', ']':
			if !inString {
				depth--
				totalBrackets++
				consecutiveOpens = 0 // Reset on closing bracket
			}
		default:
			consecutiveOpens = 0 // Reset on non-bracket character
		}
	}

	// SECURITY: Check for unbalanced brackets
	if depth != 0 {
		return newOperationError("validate_nesting_depth",
			"unbalanced brackets in JSON structure", ErrInvalidJSON)
	}

	return nil
}

// validateContainerCounts is the pre-P-003 container scan (reference for
// TestP003StructureLimits; see the comment above validateNestingDepth).
func (sv *securityValidator) validateContainerCounts(jsonStr string) error {
	maxKeys := sv.maxObjectKeys
	maxElements := sv.maxArrayElements
	// Both unlimited — nothing to enforce. (Config validation clamps these to
	// >=100, so this is a defensive guard for the unlimited sentinel.)
	// Duplicate-key detection reuses this same walk, so it must also proceed.
	if maxKeys <= 0 && maxElements <= 0 && !sv.detectDuplicateKeys {
		return nil
	}

	detectDup := sv.detectDuplicateKeys

	stack := make([]containerFrame, 0, 32)

	inString := false
	escaped := false

	for i := 0; i < len(jsonStr); i++ {
		c := jsonStr[i]

		if escaped {
			escaped = false
			continue
		}
		if inString {
			switch c {
			case '\\':
				escaped = true
			case '"':
				inString = false
				// Duplicate-key check (GEN-001): a string just closed in key
				// position of an object frame. Containers cannot open inside
				// a string, so the frame seen here is the one that was on top
				// when the key opened. Keys are compared as raw bytes: two
				// spellings that differ only by escape encoding (e.g. "a" vs
				// "a") are treated as distinct — the underlying parse is
				// still last-wins for such pairs.
				if detectDup && len(stack) > 0 {
					top := &stack[len(stack)-1]
					if !top.isArray && top.keyStart >= 0 {
						key := jsonStr[top.keyStart:i]
						top.keyStart = -1
						if top.keySet == nil {
							top.keySet = make(map[string]struct{}, 8)
						}
						if _, dup := top.keySet[key]; dup {
							return newOperationError("validate_container_counts",
								fmt.Sprintf("duplicate object key %q", key), ErrDuplicateKey)
						}
						top.keySet[key] = struct{}{}
					}
				}
			}
			continue
		}

		switch c {
		case '"':
			inString = true
			if detectDup && len(stack) > 0 {
				top := &stack[len(stack)-1]
				// In an object, a string opening while expecting a child is a
				// KEY (a value string only follows ':', which clears
				// expectingChild). Array strings are values — not tracked.
				if !top.isArray && top.expectingChild {
					top.keyStart = i + 1
				}
			}
			noteValueStart(stack)
		case '{', '[':
			// A container open is itself a value start in its parent...
			noteValueStart(stack)
			// ...then descend into the new container.
			stack = append(stack, containerFrame{
				isArray:        c == '[',
				expectingChild: true,
				keyStart:       -1,
			})
		case '}', ']':
			if len(stack) > 0 {
				frame := stack[len(stack)-1]
				stack = stack[:len(stack)-1]
				if frame.isArray {
					if maxElements > 0 && frame.count > maxElements {
						return newOperationError("validate_container_counts",
							fmt.Sprintf("array has %d elements, exceeds maximum %d", frame.count, maxElements),
							ErrSizeLimit)
					}
				} else {
					if maxKeys > 0 && frame.count > maxKeys {
						return newOperationError("validate_container_counts",
							fmt.Sprintf("object has %d keys, exceeds maximum %d", frame.count, maxKeys),
							ErrSizeLimit)
					}
				}
			}
		case ',':
			if len(stack) > 0 {
				stack[len(stack)-1].expectingChild = true
			}
		case ':':
			// Object key/value separator. The key was already counted as a value
			// start; nothing to do. (A ':' outside an object is malformed JSON
			// and is rejected by the parser downstream.)
		default:
			// Whitespace is structural; any other byte is the leading byte of a
			// primitive value (digit, '-', 't'/'f'/'n', etc.).
			if !isSpace(c) {
				noteValueStart(stack)
			}
		}
	}

	return nil
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

// TestValidateFilePath_Matrix consolidates the eight identical-scaffold
// validateFilePath tables (device names, traversal, ADS, path length, null
// bytes, UNC, edge cases, Windows components) plus the former standalone
// invalid-chars / real-paths / normalization / cross-platform tables into one
// table-driven test. Rows flagged windows=true replace the per-function
// runtime.GOOS skips and are skipped silently on other platforms.
func TestValidateFilePath_Matrix(t *testing.T) {
	processor, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer processor.Close()

	rows := []struct {
		group       string
		windows     bool
		name        string
		filePath    string
		expectError bool
	}{
		{
			group:       "device-names",
			windows:     true,
			name:        "CON device",
			filePath:    "CON",
			expectError: true,
		},
		{
			group:       "device-names",
			windows:     true,
			name:        "PRN device",
			filePath:    "PRN",
			expectError: true,
		},
		{
			group:       "device-names",
			windows:     true,
			name:        "AUX device",
			filePath:    "AUX",
			expectError: true,
		},
		{
			group:       "device-names",
			windows:     true,
			name:        "NUL device",
			filePath:    "NUL",
			expectError: true,
		},
		{
			group:       "device-names",
			windows:     true,
			name:        "COM1 device",
			filePath:    "COM1",
			expectError: true,
		},
		{
			group:       "device-names",
			windows:     true,
			name:        "COM9 device",
			filePath:    "COM9",
			expectError: true,
		},
		{
			group:       "device-names",
			windows:     true,
			name:        "COM0 device",
			filePath:    "COM0",
			expectError: true,
		},
		{
			group:       "device-names",
			windows:     true,
			name:        "LPT1 device",
			filePath:    "LPT1",
			expectError: true,
		},
		{
			group:       "device-names",
			windows:     true,
			name:        "LPT9 device",
			filePath:    "LPT9",
			expectError: true,
		},
		{
			group:       "device-names",
			windows:     true,
			name:        "LPT0 device",
			filePath:    "LPT0",
			expectError: true,
		},
		{
			group:       "device-names",
			windows:     true,
			name:        "CONIN device",
			filePath:    "CONIN$",
			expectError: true,
		},
		{
			group:       "device-names",
			windows:     true,
			name:        "CONOUT device",
			filePath:    "CONOUT$",
			expectError: true,
		},
		{
			group:       "device-names",
			windows:     true,
			name:        "device with extension",
			filePath:    "CON.txt",
			expectError: true,
		},
		{
			group:       "device-names",
			windows:     true,
			name:        "normal file",
			filePath:    "normal.json",
			expectError: false,
		},
		{
			group:       "device-names",
			windows:     true,
			name:        "path with device",
			filePath:    "data/CON",
			expectError: true,
		},
		{
			group:       "traversal",
			windows:     false,
			name:        "double dot traversal",
			filePath:    "../../etc/passwd",
			expectError: true,
		},
		{
			group:       "traversal",
			windows:     false,
			name:        "URL encoded traversal",
			filePath:    "%2e%2e/%2e%2e/etc/passwd",
			expectError: true,
		},
		{
			group:       "traversal",
			windows:     false,
			name:        "double URL encoded",
			filePath:    "%252e%252e/%252e%252e",
			expectError: true,
		},
		{
			group:       "traversal",
			windows:     false,
			name:        "mixed encoding traversal",
			filePath:    "..%2fetc/passwd",
			expectError: true,
		},
		{
			group:       "traversal",
			windows:     false,
			name:        "Windows backslash encoded",
			filePath:    "..%5cetc/passwd",
			expectError: true,
		},
		{
			group:       "traversal",
			windows:     false,
			name:        "UTF-8 overlong encoding",
			filePath:    "..%c0%af/etc/passwd",
			expectError: true,
		},
		{
			group:       "traversal",
			windows:     false,
			name:        "partial double encoding",
			filePath:    "..%2e",
			expectError: true,
		},
		{
			group:       "traversal",
			windows:     false,
			name:        "null byte injection",
			filePath:    "file.txt\x00",
			expectError: true,
		},
		{
			group:       "traversal",
			windows:     false,
			name:        "newline injection",
			filePath:    "file.txt%0a",
			expectError: true,
		},
		{
			group:       "traversal",
			windows:     false,
			name:        "carriage return injection",
			filePath:    "file.txt%0d",
			expectError: true,
		},
		{
			group:       "traversal",
			windows:     false,
			name:        "tab injection",
			filePath:    "file.txt%09",
			expectError: true,
		},
		{
			group:       "traversal",
			windows:     false,
			name:        "five consecutive dots",
			filePath:    ".....//etc/passwd",
			expectError: true,
		},
		{
			group:       "traversal",
			windows:     false,
			name:        "six consecutive dots",
			filePath:    "......//etc/passwd",
			expectError: true,
		},
		{
			group:       "traversal",
			windows:     false,
			name:        "normal path",
			filePath:    "data/user/profile.json",
			expectError: false,
		},
		{
			group:       "traversal",
			windows:     false,
			name:        "absolute path",
			filePath:    "/home/user/data.json",
			expectError: false,
		},
		{
			group:       "alternate-data-stream",
			windows:     true,
			name:        "ADS with colon",
			filePath:    "file.txt:stream",
			expectError: true,
		},
		{
			group:       "alternate-data-stream",
			windows:     true,
			name:        "ADS with $DATA",
			filePath:    "file.txt:$DATA",
			expectError: true,
		},
		{
			group:       "alternate-data-stream",
			windows:     true,
			name:        "complex ADS",
			filePath:    "file.txt:stream:$DATA",
			expectError: true,
		},
		{
			group:       "alternate-data-stream",
			windows:     true,
			name:        "drive letter not ADS",
			filePath:    "C:/data/file.txt",
			expectError: false,
		},
		{
			group:       "alternate-data-stream",
			windows:     true,
			name:        "drive letter with colon",
			filePath:    "C:data/file.txt",
			expectError: false,
		},
		{
			group:       "path-length",
			windows:     false,
			name:        "exceeds max length",
			filePath:    strings.Repeat("a", maxPathLength+1),
			expectError: true,
		},
		{
			group:       "path-length",
			windows:     false,
			name:        "exactly max length",
			filePath:    strings.Repeat("b", maxPathLength),
			expectError: false,
		},
		{
			group:       "path-length",
			windows:     false,
			name:        "normal length",
			filePath:    "data/user/profile.json",
			expectError: false,
		},
		{
			group:       "null-bytes",
			windows:     false,
			name:        "null at start",
			filePath:    "\x00file.txt",
			expectError: true,
		},
		{
			group:       "null-bytes",
			windows:     false,
			name:        "null in middle",
			filePath:    "file\x00.txt",
			expectError: true,
		},
		{
			group:       "null-bytes",
			windows:     false,
			name:        "null at end",
			filePath:    "file.txt\x00",
			expectError: true,
		},
		{
			group:       "null-bytes",
			windows:     false,
			name:        "multiple nulls",
			filePath:    "file\x00\x00.txt",
			expectError: true,
		},
		{
			group:       "unc",
			windows:     true,
			name:        "UNC with backslashes",
			filePath:    "\\\\server\\share\\file.txt",
			expectError: true,
		},
		{
			group:       "unc",
			windows:     true,
			name:        "UNC with forward slashes",
			filePath:    "//server/share/file.txt",
			expectError: true,
		},
		{
			group:       "unc",
			windows:     true,
			name:        "local path",
			filePath:    "C:/data/file.txt",
			expectError: false,
		},
		{
			group:       "edge-cases",
			windows:     false,
			name:        "empty path",
			filePath:    "",
			expectError: true,
		},
		{
			group:       "edge-cases",
			windows:     false,
			name:        "single character",
			filePath:    "a",
			expectError: false,
		},
		{
			group:       "edge-cases",
			windows:     false,
			name:        "current directory",
			filePath:    ".",
			expectError: false,
		},
		{
			group:       "edge-cases",
			windows:     false,
			name:        "parent directory",
			filePath:    "..",
			expectError: true,
		},
		{
			group:       "edge-cases",
			windows:     false,
			name:        "file with extension",
			filePath:    "document.pdf",
			expectError: false,
		},
		{
			group:       "edge-cases",
			windows:     false,
			name:        "deep path",
			filePath:    "a/b/c/d/e/f/g/h/i/j/file.txt",
			expectError: false,
		},
		{
			group:       "components",
			windows:     true,
			name:        "valid absolute path",
			filePath:    "C:/Users/user/data.json",
			expectError: false,
		},
		{
			group:       "components",
			windows:     true,
			name:        "valid relative path",
			filePath:    "data/config.json",
			expectError: false,
		},
		{
			group:       "components",
			windows:     true,
			name:        "path with spaces",
			filePath:    "C:/Program Files/data.json",
			expectError: false,
		},
		{
			group:       "components",
			windows:     true,
			name:        "path with underscore",
			filePath:    "my_data/file.json",
			expectError: false,
		},
		{
			group:       "components",
			windows:     true,
			name:        "path with hyphen",
			filePath:    "my-data/file.json",
			expectError: false,
		},
		{
			group:       "components",
			windows:     true,
			name:        "path with pipe",
			filePath:    "data|file.json",
			expectError: true,
		},
		{
			group:       "components",
			windows:     true,
			name:        "path with asterisk",
			filePath:    "data/*.json",
			expectError: true,
		},
		{
			group:       "components",
			windows:     true,
			name:        "path with question mark",
			filePath:    "data/file?.json",
			expectError: true,
		},
		{
			group:       "windows-invalid-chars",
			windows:     true,
			name:        "path with less-than",
			filePath:    "data<file.json",
			expectError: true,
		},
		{
			group:       "windows-invalid-chars",
			windows:     true,
			name:        "path with greater-than",
			filePath:    "data>file.json",
			expectError: true,
		},
		{
			group:       "windows-invalid-chars",
			windows:     true,
			name:        "path with quote",
			filePath:    `data"file.json`,
			expectError: true,
		},
		{
			group:       "windows-invalid-chars",
			windows:     true,
			name:        "colon is not a drive letter",
			filePath:    `data:\file.json`,
			expectError: true,
		},
		{
			group:       "normalization",
			name:        "extra separators",
			filePath:    "data///config.json",
			expectError: false,
		},
	}

	for _, tt := range rows {
		if tt.windows && runtime.GOOS != "windows" {
			continue
		}
		t.Run(tt.group+"/"+tt.name, func(t *testing.T) {
			err := processor.validateFilePath(tt.filePath)
			if tt.expectError && err == nil {
				t.Errorf("path %q: expected rejection, got nil error", tt.filePath)
			}
			if !tt.expectError && err != nil {
				t.Errorf("path %q: unexpected error: %v", tt.filePath, err)
			}
		})
	}
}
