package json

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cybergodev/json/internal"
)

// Merged from: api_test.go, core_test.go, operation_test.go, comprehensive_test.go, path_test.go

// ============================================================================
// Test Helper Functions
// ============================================================================

// intPtr returns a pointer to an int
func intPtr(i int) *int {
	return &i
}

// getProp gets a property from a map
func getProp(data any, key string) any {
	if m, ok := data.(map[string]any); ok {
		return m[key]
	}
	return nil
}

// slicesEqual compares two slices for equality
func slicesEqual(a, b []any) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// ============================================================================
// SafeGet API Tests (migrated from api_access_test.go)
// ============================================================================

func TestSafeGetAPI(t *testing.T) {
	tests := []struct {
		name      string
		jsonStr   string
		path      string
		wantExist bool
		wantType  string
	}{
		{"string value", `{"name":"Alice"}`, "name", true, "string"},
		{"number value", `{"age":30}`, "age", true, "float64"},
		{"bool value", `{"active":true}`, "active", true, "bool"},
		{"null value", `{"val":null}`, "val", true, "null"},
		{"nested path", `{"a":{"b":1}}`, "a.b", true, "float64"},
		{"array index", `{"a":[1,2]}`, "a[0]", true, "float64"},
		{"missing path", `{"a":1}`, "missing", false, ""},
		{"empty json", "", "path", false, ""},
		{"empty path", `{"a":1}`, "", false, ""},
		{"invalid json", "invalid", "x", false, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SafeGet(tt.jsonStr, tt.path)
			if result.Exists != tt.wantExist {
				t.Errorf("SafeGet(%q, %q).Exists = %v, want %v", tt.jsonStr, tt.path, result.Exists, tt.wantExist)
			}
			if tt.wantExist && result.Type != tt.wantType {
				t.Errorf("SafeGet(%q, %q).Type = %q, want %q", tt.jsonStr, tt.path, result.Type, tt.wantType)
			}
		})
	}
}

// ============================================================================
// Benchmark Tests
// ============================================================================

func BenchmarkHandleArrayAccess(b *testing.B) {
	processor, _ := New()
	defer processor.Close()

	arr := make([]any, 1000)
	for i := 0; i < 1000; i++ {
		arr[i] = i
	}

	segment := internal.PathSegment{
		Type:  internal.ArrayIndexSegment,
		Index: 500,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = processor.handleArrayAccess(arr, segment)
	}
}

func BenchmarkHandlePropertyAccess(b *testing.B) {
	processor, _ := New()
	defer processor.Close()

	data := map[string]any{
		"name":  "Alice",
		"age":   30,
		"email": "alice@example.com",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = processor.handlePropertyAccess(data, "name")
	}
}

func BenchmarkJSONPointerEscape(b *testing.B) {
	input := "user~/name/with~special/chars"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = internal.EscapeJSONPointer(input)
	}
}

// BenchmarkMarshal is covered by comprehensive versions in benchmark_test.go

func BenchmarkPathParsingComplex(b *testing.B) {
	processor, _ := New()
	defer processor.Close()

	path := "users[0:5]{name}.first"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = processor.parsePath(path)
	}
}

func BenchmarkPathParsingSimple(b *testing.B) {
	processor, _ := New()
	defer processor.Close()

	path := "user.name.first"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = processor.parsePath(path)
	}
}

func BenchmarkPerformArraySlice(b *testing.B) {
	processor, _ := New()
	defer processor.Close()

	arr := make([]any, 1000)
	for i := 0; i < 1000; i++ {
		arr[i] = i
	}

	start := intPtr(100)
	end := intPtr(900)
	step := intPtr(2)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = internal.PerformArraySlice(arr, start, end, step)
	}
}

// BenchmarkSet is covered by comprehensive versions in benchmark_test.go

// TestArrayExtensionSignal was removed with the arrayExtensionSignal type
// (D-002: the signal path was unreachable from Set).

// TestArrayIndexValidation tests array index validation
func TestArrayIndexValidation(t *testing.T) {
	tests := []struct {
		name  string
		index string
		valid bool
	}{
		{
			name:  "valid positive",
			index: "0",
			valid: true,
		},
		{
			name:  "valid negative",
			index: "-1",
			valid: true,
		},
		{
			name:  "empty",
			index: "",
			valid: false,
		},
		{
			name:  "invalid text",
			index: "abc",
			valid: false,
		},
		{
			name:  "with decimal",
			index: "1.5",
			valid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := internal.IsValidArrayIndex(tt.index)
			if result != tt.valid {
				t.Errorf("isValidArrayIndex(%s) = %v; want %v", tt.index, result, tt.valid)
			}
		})
	}
}

// TestArraySliceEdgeCases tests edge cases for array slicing
func TestArraySliceEdgeCases(t *testing.T) {
	jsonStr := `{
		"items": [0, 1, 2, 3, 4, 5, 6, 7, 8, 9]
	}`

	processor, _ := New()
	defer processor.Close()

	tests := []struct {
		name     string
		path     string
		expected []any
	}{
		{
			name:     "slice from start",
			path:     "items[:5]",
			expected: []any{0.0, 1.0, 2.0, 3.0, 4.0},
		},
		{
			name:     "slice to end",
			path:     "items[5:]",
			expected: []any{5.0, 6.0, 7.0, 8.0, 9.0},
		},
		{
			name:     "full slice",
			path:     "items[:]",
			expected: []any{0.0, 1.0, 2.0, 3.0, 4.0, 5.0, 6.0, 7.0, 8.0, 9.0},
		},
		{
			name:     "negative indices",
			path:     "items[-3:]",
			expected: []any{7.0, 8.0, 9.0},
		},
		{
			name:     "step slice",
			path:     "items[0:10:2]",
			expected: []any{0.0, 2.0, 4.0, 6.0, 8.0},
		},
		{
			name:     "empty result",
			path:     "items[10:15]",
			expected: []any{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := processor.Get(jsonStr, tt.path)
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			arr, ok := result.([]any)
			if !ok {
				t.Fatalf("Expected []any, got %T", result)
			}
			if !slicesEqual(arr, tt.expected) {
				t.Errorf("Get(%s) = %v; want %v", tt.path, arr, tt.expected)
			}
		})
	}
}

// TestArraySliceOperations tests array slice operations
func TestArraySliceOperations(t *testing.T) {
	processor, _ := New()
	defer processor.Close()

	t.Run("get array slice", func(t *testing.T) {
		jsonStr := `{"items": [1, 2, 3, 4, 5]}`
		result, err := processor.Get(jsonStr, "items[1:3]")
		if err != nil {
			t.Fatalf("Get slice error: %v", err)
		}
		arr, ok := result.([]any)
		if !ok {
			t.Fatalf("Expected array, got %T", result)
		}
		if len(arr) != 2 {
			t.Errorf("Slice length = %d, want 2", len(arr))
		}
	})

	t.Run("set array slice", func(t *testing.T) {
		jsonStr := `{"items": [1, 2, 3, 4, 5]}`
		result, err := processor.Set(jsonStr, "items[0:2]", []any{10, 20})
		if err != nil {
			t.Fatalf("Set slice error: %v", err)
		}
		// Verify the slice was updated
		var parsed map[string]any
		if err := json.Unmarshal([]byte(result), &parsed); err != nil {
			t.Fatalf("Failed to parse result: %v", err)
		}
		items, ok := parsed["items"].([]any)
		if !ok {
			t.Fatal("items should be an array")
		}
		// Check first two items were replaced
		if len(items) != 5 {
			t.Errorf("Expected 5 items, got %d", len(items))
		}
	})
}

// TestBoundaryConditions tests edge cases and boundary conditions
func TestBoundaryConditions(t *testing.T) {
	helper := newTestHelper(t)

	t.Run("EmptyValues", func(t *testing.T) {
		t.Run("EmptyString", func(t *testing.T) {
			testData := `{"empty": "", "normal": "value"}`

			result, err := Get(testData, "empty")
			helper.AssertNoError(err)
			helper.AssertEqual("", result)

			withDefault := GetTyped[string](testData, "missing", "default")
			helper.AssertEqual("default", withDefault)
		})

		t.Run("EmptyArray", func(t *testing.T) {
			testData := `{"empty": [], "normal": [1, 2, 3]}`

			result := GetArray(testData, "empty", nil)
			helper.AssertEqual(0, len(result))
		})

		t.Run("EmptyObject", func(t *testing.T) {
			testData := `{"empty": {}, "normal": {"key": "value"}}`

			result := GetObject(testData, "empty", nil)
			helper.AssertEqual(0, len(result))
		})

		t.Run("NullValue", func(t *testing.T) {
			testData := `{"null_value": null, "string": "value"}`

			result, err := Get(testData, "null_value")
			helper.AssertNoError(err)
			helper.AssertNil(result)
		})
	})

	t.Run("NumericBoundaries", func(t *testing.T) {
		processor, _ := New(DefaultConfig())
		defer processor.Close()

		t.Run("MaxInt64", func(t *testing.T) {
			testData := `{"max": 9223372036854775807}`

			result, err := processor.Get(testData, "max")
			helper.AssertNoError(err)

			if num, ok := result.(float64); ok {
				helper.AssertTrue(num == float64(math.MaxInt64) || num == float64(int64(math.MaxInt64)))
			}
		})

		t.Run("MinInt64", func(t *testing.T) {
			testData := `{"min": -9223372036854775808}`

			result, err := processor.Get(testData, "min")
			helper.AssertNoError(err)

			if num, ok := result.(float64); ok {
				helper.AssertTrue(num <= float64(math.MinInt64))
			}
		})

		t.Run("MaxFloat", func(t *testing.T) {
			testData := `{"max_float": 1.7976931348623157e+308}`

			result, err := processor.Get(testData, "max_float")
			helper.AssertNoError(err)

			if num, ok := result.(float64); ok {
				helper.AssertTrue(num > 1.79e308 || num == math.MaxFloat64)
			}
		})

		t.Run("VerySmallFloat", func(t *testing.T) {
			testData := `{"small": 1e-323}`

			result, err := processor.Get(testData, "small")
			helper.AssertNoError(err)

			if num, ok := result.(float64); ok {
				helper.AssertTrue(num >= 0 && num < 1e-300)
			}
		})

		t.Run("ZeroValues", func(t *testing.T) {
			testData := `{"int_zero": 0, "float_zero": 0.0, "bool_false": false}`

			intZero := GetInt(testData, "int_zero", 0)
			helper.AssertEqual(0, intZero)

			floatZero := GetFloat(testData, "float_zero", 0.0)
			helper.AssertEqual(0.0, floatZero)

			boolFalse := GetBool(testData, "bool_false", false)
			helper.AssertFalse(boolFalse)
		})
	})

	t.Run("StringBoundaries", func(t *testing.T) {
		t.Run("VeryLongString", func(t *testing.T) {
			longString := strings.Repeat("a", 100000)
			testData := `{"long": "` + longString + `"}`

			result, err := Get(testData, "long")
			helper.AssertNoError(err)

			if str, ok := result.(string); ok {
				helper.AssertEqual(100000, len(str))
			}
		})

		t.Run("SpecialCharacters", func(t *testing.T) {
			specialChars := `\"\n\r\t\b\f\/\\`
			testData := `{"special": "` + specialChars + `"}`

			result, err := Get(testData, "special")
			helper.AssertNoError(err)

			if str, ok := result.(string); ok {
				helper.AssertTrue(len(str) > 0)
			}
		})

		t.Run("UnicodeEscape", func(t *testing.T) {
			testData := `{"unicode": "\u0048\u0065\u006c\u006c\u006f"}`

			result, err := Get(testData, "unicode")
			helper.AssertNoError(err)

			if str, ok := result.(string); ok {
				helper.AssertEqual("Hello", str)
			}
		})

		t.Run("Emoji", func(t *testing.T) {
			testData := `{"emoji": "🎉🚀💻"}`

			result, err := Get(testData, "emoji")
			helper.AssertNoError(err)

			if str, ok := result.(string); ok {
				helper.AssertTrue(len(str) > 0)
			}
		})
	})

	t.Run("ArrayBoundaries", func(t *testing.T) {
		t.Run("SingleElement", func(t *testing.T) {
			testData := `{"single": [1]}`

			first, _ := Get(testData, "single[0]")
			helper.AssertEqual(float64(1), first)

			last, _ := Get(testData, "single[-1]")
			helper.AssertEqual(float64(1), last)
		})

		t.Run("LargeArray", func(t *testing.T) {
			elements := make([]string, 1000)
			for i := 0; i < 1000; i++ {
				elements[i] = fmt.Sprint(i)
			}
			testData := `{"large": [` + strings.Join(elements, ",") + `]}`

			result := GetArray(testData, "large", nil)
			helper.AssertEqual(1000, len(result))
		})

		t.Run("ArraySlicing", func(t *testing.T) {
			testData := `{"arr": [0, 1, 2, 3, 4, 5, 6, 7, 8, 9]}`

			tests := []struct {
				path     string
				expected []interface{}
			}{
				{"arr[0:3]", []interface{}{float64(0), float64(1), float64(2)}},
				{"arr[5:]", []interface{}{float64(5), float64(6), float64(7), float64(8), float64(9)}},
				{"arr[:3]", []interface{}{float64(0), float64(1), float64(2)}},
				{"arr[-3:]", []interface{}{float64(7), float64(8), float64(9)}},
			}

			for _, tt := range tests {
				t.Run(tt.path, func(t *testing.T) {
					result, err := Get(testData, tt.path)
					helper.AssertNoError(err)

					if arr, ok := result.([]interface{}); ok {
						helper.AssertEqual(len(tt.expected), len(arr))
						for i, exp := range tt.expected {
							helper.AssertEqual(exp, arr[i])
						}
					}
				})
			}
		})
	})

	t.Run("ObjectBoundaries", func(t *testing.T) {
		t.Run("ManyKeys", func(t *testing.T) {
			pairs := make([]string, 100)
			for i := 0; i < 100; i++ {
				pairs[i] = `"key` + fmt.Sprint(i) + `": ` + fmt.Sprint(i)
			}
			testData := `{"many": {` + strings.Join(pairs, ",") + `}}`

			result := GetObject(testData, "many", nil)
			helper.AssertEqual(100, len(result))
		})

		t.Run("DeeplyNested", func(t *testing.T) {
			testData := `{"root": {"level1": {"level2": {"level3": {"level4": {"deep": "value"}}}}}}`

			result, err := Get(testData, "root.level1.level2.level3.level4.deep")
			helper.AssertNoError(err)
			helper.AssertEqual("value", result)
		})

		t.Run("MixedTypes", func(t *testing.T) {
			testData := `{
				"string": "text",
				"number": 42,
				"float": 3.14,
				"bool": true,
				"null": null,
				"array": [1, 2, 3],
				"object": {"nested": "value"}
			}`

			_, err := Get(testData, "string")
			helper.AssertNoError(err)

			_, err = Get(testData, "number")
			helper.AssertNoError(err)

			_, err = Get(testData, "float")
			helper.AssertNoError(err)

			_, err = Get(testData, "bool")
			helper.AssertNoError(err)

			_, err = Get(testData, "null")
			helper.AssertNoError(err)

			_, err = Get(testData, "array")
			helper.AssertNoError(err)

			_, err = Get(testData, "object")
			helper.AssertNoError(err)
		})
	})

	t.Run("WhitespaceHandling", func(t *testing.T) {
		t.Run("LotsOfWhitespace", func(t *testing.T) {
			whitespaceJSON := `{` + strings.Repeat(" \t\n", 1000) + `"key"` + strings.Repeat(" \t\n", 1000) + `:` + strings.Repeat(" \t\n", 1000) + `"value"` + strings.Repeat(" \t\n", 1000) + `}`

			result, err := Get(whitespaceJSON, "key")
			helper.AssertNoError(err)
			helper.AssertEqual("value", result)
		})

		t.Run("NoWhitespace", func(t *testing.T) {
			noSpaceJSON := `{"key":"value"}`
			result, err := Get(noSpaceJSON, "key")
			helper.AssertNoError(err)
			helper.AssertEqual("value", result)
		})
	})

	t.Run("BooleanBoundaries", func(t *testing.T) {
		testData := `{"true": true, "false": false}`

		trueVal := GetBool(testData, "true", false)
		helper.AssertTrue(trueVal)

		falseVal := GetBool(testData, "false", true)
		helper.AssertFalse(falseVal)

		conversionTests := []struct {
			value    interface{}
			expected bool
		}{
			{1, true},
			{0, false},
			{-1, true},
			{1.0, true},
			{0.0, false},
			{"true", true},
			{"false", false},
			{"1", true},
			{"0", false},
		}

		for _, tt := range conversionTests {
			name := "true"
			if !tt.expected {
				name = "false"
			}
			t.Run("Convert_"+name, func(t *testing.T) {
				result, ok := convertToBool(tt.value)
				helper.AssertTrue(ok)
				helper.AssertEqual(tt.expected, result)
			})
		}
	})

	t.Run("TimestampEdgeCases", func(t *testing.T) {
		testData := `{"timestamp": "2024-01-15T10:30:00Z", "epoch": 1705319400}`

		timestamp := GetString(testData, "timestamp", "")
		helper.AssertEqual("2024-01-15T10:30:00Z", timestamp)

		epoch := GetInt(testData, "epoch", 0)
		helper.AssertEqual(1705319400, epoch)
	})
}

// TestBufferCompatibility tests buffer-based operations
func TestBufferCompatibility(t *testing.T) {
	jsonStr := `{"name": "Alice", "age": 30}`
	src := []byte(jsonStr)

	// Test Compact
	var compactBuf bytes.Buffer
	err := Compact(&compactBuf, src)
	if err != nil {
		t.Fatalf("Compact failed: %v", err)
	}

	// Test Indent
	var indentBuf bytes.Buffer
	err = Indent(&indentBuf, src, "", "  ")
	if err != nil {
		t.Fatalf("Indent failed: %v", err)
	}
	if !strings.Contains(indentBuf.String(), "\n") {
		t.Error("Expected indented output")
	}

	// Test HTMLEscape
	// Note: Using characters that don't trigger security validation
	var escapeBuf bytes.Buffer
	htmlJSON := []byte(`{"html": "<div>Content & more</div>"}`)
	HTMLEscape(&escapeBuf, htmlJSON)
	escaped := escapeBuf.String()
	// HTML entities should be escaped
	// Standard library escapes < to \u003c, > to \u003e, & to \u0026
	if !strings.Contains(escaped, "\\u003c") && !strings.Contains(escaped, "\\u003e") && !strings.Contains(escaped, "\\u0026") {
		t.Logf("Actual escaped output: %s", escaped)
		// Check that raw HTML characters are not present
		if strings.Contains(escaped, "<div>") {
			t.Error("Expected HTML to be escaped but found raw <div>")
		}
	}
}

// TestCompactBuffer tests compactBuffer function
func TestCompactBuffer(t *testing.T) {
	prettyJSON := `{
		"name": "Alice",
		"age": 30
	}`

	var buf bytes.Buffer
	err := Compact(&buf, []byte(prettyJSON))
	if err != nil {
		t.Fatalf("CompactBuffer error: %v", err)
	}

	result := buf.String()
	if strings.Contains(result, "\n") || strings.Contains(result, "  ") {
		t.Errorf("CompactBuffer should remove whitespace, got: %s", result)
	}
	if !strings.Contains(result, `"name"`) || !strings.Contains(result, `"Alice"`) {
		t.Errorf("CompactBuffer lost data, got: %s", result)
	}
}

// TestComplexPathWithBracesAndBrackets tests paths with both braces and brackets
func TestComplexPathWithBracesAndBrackets(t *testing.T) {
	jsonStr := `{
		"users": [
			{"name": "Alice", "tags": ["a", "b", "c"]},
			{"name": "Bob", "tags": ["d", "e", "f"]}
		]
	}`

	processor, _ := New()
	defer processor.Close()

	tests := []struct {
		name        string
		path        string
		expectError bool
	}{
		{
			name:        "extraction then array",
			path:        "users{tags}[0]",
			expectError: false,
		},
		{
			name:        "array then extraction",
			path:        "users[0]{name}",
			expectError: false,
		},
		{
			name:        "slice then extraction",
			path:        "users[0:1]{name}",
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := processor.Get(jsonStr, tt.path)
			if tt.expectError && err == nil {
				t.Errorf("Expected error for path '%s', but got none", tt.path)
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error for path '%s': %v", tt.path, err)
			}
		})
	}
}

// TestConfiguration tests configuration creation, validation, and cloning
func TestConfiguration(t *testing.T) {
	helper := newTestHelper(t)

	t.Run("DefaultConfig", func(t *testing.T) {
		config := DefaultConfig()

		helper.AssertNotNil(config)
		helper.AssertTrue(config.EnableCache)
		helper.AssertEqual(defaultCacheSize, config.MaxCacheSize)
		helper.AssertEqual(DefaultCacheTTL, config.CacheTTL)
		helper.AssertEqual(int64(DefaultMaxJSONSize), config.MaxJSONSize)
		helper.AssertEqual(DefaultMaxPathDepth, config.MaxPathDepth)
		helper.AssertFalse(config.StrictMode)
		helper.AssertFalse(config.EnableMetrics)
		helper.AssertFalse(config.EnableHealthCheck)
	})

	t.Run("SecurityConfig", func(t *testing.T) {
		config := SecurityConfig()

		helper.AssertNotNil(config)
		helper.AssertTrue(config.FullSecurityScan)
		helper.AssertTrue(config.EnableValidation)
	})

	t.Run("ConfigClone", func(t *testing.T) {
		original := DefaultConfig()
		original.EnableCache = false
		original.StrictMode = true

		cloned := (&original).Clone()

		helper.AssertNotNil(cloned)
		helper.AssertEqual(original.EnableCache, cloned.EnableCache)
		helper.AssertEqual(original.StrictMode, cloned.StrictMode)

		// Modify clone should not affect original
		cloned.EnableCache = true
		helper.AssertFalse(original.EnableCache)
	})

	t.Run("ConfigValidate", func(t *testing.T) {
		t.Run("ValidConfig", func(t *testing.T) {
			config := DefaultConfig()
			err := config.Validate()
			helper.AssertNoError(err)
		})

		t.Run("NilConfig", func(t *testing.T) {
			var config *Config
			err := config.Validate()
			helper.AssertError(err)
		})

		t.Run("NegativeCacheSize", func(t *testing.T) {
			config := DefaultConfig()
			config.MaxCacheSize = -1
			err := config.Validate()
			helper.AssertNoError(err)
			// Negative cache size should be clamped to 0 and cache disabled
			helper.AssertEqual(0, config.MaxCacheSize)
			helper.AssertFalse(config.EnableCache)
		})

		t.Run("ZeroValuesGetOrs", func(t *testing.T) {
			config := &Config{}
			err := config.Validate()
			helper.AssertNoError(err)

			// Should have defaults applied
			helper.AssertTrue(config.MaxJSONSize > 0)
			helper.AssertTrue(config.MaxPathDepth > 0)
			helper.AssertTrue(config.MaxConcurrency > 0)
		})

		t.Run("ClampValues", func(t *testing.T) {
			config := DefaultConfig()
			config.MaxJSONSize = 200 * 1024 * 1024 // Over max
			config.MaxPathDepth = 300              // Over max

			config.Validate()

			// Should be clamped
			helper.AssertTrue(config.MaxJSONSize <= 100*1024*1024)
			helper.AssertTrue(config.MaxPathDepth <= 200)
		})
	})

}

// TestConfigurationEdgeCases tests configuration edge cases
func TestConfigurationEdgeCases(t *testing.T) {
	helper := newTestHelper(t)

	t.Run("ExtremeCacheSizes", func(t *testing.T) {
		config := DefaultConfig()

		t.Run("ZeroCacheSize", func(t *testing.T) {
			config.MaxCacheSize = 0
			err := config.Validate()
			helper.AssertNoError(err)
			// Cache might still be enabled but with 0 size
			_ = config.EnableCache
		})

		t.Run("VeryLargeCacheSize", func(t *testing.T) {
			config.MaxCacheSize = 10000
			config.Validate()
			helper.AssertTrue(config.MaxCacheSize <= 2000)
		})
	})

	t.Run("ExtremeConcurrency", func(t *testing.T) {
		config := DefaultConfig()

		t.Run("ZeroConcurrency", func(t *testing.T) {
			config.MaxConcurrency = 0
			config.Validate()
			helper.AssertTrue(config.MaxConcurrency > 0)
		})

		t.Run("VeryHighConcurrency", func(t *testing.T) {
			config.MaxConcurrency = 500
			config.Validate()
			helper.AssertTrue(config.MaxConcurrency <= 200)
		})
	})

	t.Run("CacheTTL", func(t *testing.T) {
		config := DefaultConfig()

		t.Run("ZeroTTL", func(t *testing.T) {
			config.CacheTTL = 0
			config.Validate()
			helper.AssertTrue(config.CacheTTL > 0)
		})

		t.Run("VeryShortTTL", func(t *testing.T) {
			config.CacheTTL = 1 * time.Nanosecond
			helper.AssertNoError(config.Validate())
		})

		t.Run("VeryLongTTL", func(t *testing.T) {
			config.CacheTTL = 24 * time.Hour
			helper.AssertNoError(config.Validate())
		})
	})

	t.Run("BooleanFlags", func(t *testing.T) {
		config := DefaultConfig()

		// Test all boolean flags can be set
		flags := []struct {
			name string
			set  func(bool)
			get  func() bool
		}{
			{"EnableCache", func(b bool) { config.EnableCache = b }, func() bool { return config.EnableCache }},
			{"EnableValidation", func(b bool) { config.EnableValidation = b }, func() bool { return config.EnableValidation }},
			{"StrictMode", func(b bool) { config.StrictMode = b }, func() bool { return config.StrictMode }},
			{"CreatePaths", func(b bool) { config.CreatePaths = b }, func() bool { return config.CreatePaths }},
			{"CleanupNulls", func(b bool) { config.CleanupNulls = b }, func() bool { return config.CleanupNulls }},
			{"CompactArrays", func(b bool) { config.CompactArrays = b }, func() bool { return config.CompactArrays }},
			{"EnableMetrics", func(b bool) { config.EnableMetrics = b }, func() bool { return config.EnableMetrics }},
			{"EnableHealthCheck", func(b bool) { config.EnableHealthCheck = b }, func() bool { return config.EnableHealthCheck }},
			{"AllowComments", func(b bool) { config.AllowComments = b }, func() bool { return config.AllowComments }},
			{"PreserveNumbers", func(b bool) { config.PreserveNumbers = b }, func() bool { return config.PreserveNumbers }},
			{"ValidateInput", func(b bool) { config.ValidateInput = b }, func() bool { return config.ValidateInput }},
			{"ValidateFilePath", func(b bool) { config.ValidateFilePath = b }, func() bool { return config.ValidateFilePath }},
		}

		for _, flag := range flags {
			t.Run(flag.name+"_True", func(t *testing.T) {
				flag.set(true)
				helper.AssertTrue(flag.get())
			})

			t.Run(flag.name+"_False", func(t *testing.T) {
				flag.set(false)
				helper.AssertFalse(flag.get())
			})
		}
	})
}

// TestConfigurationIntegration tests configuration with processor
func TestConfigurationIntegration(t *testing.T) {
	helper := newTestHelper(t)

	t.Run("ProcessorWithConfig", func(t *testing.T) {
		config := DefaultConfig()
		config.EnableCache = true
		config.EnableMetrics = true

		processor, _ := New(config)
		defer processor.Close()

		testData := `{"test": "value"}`

		result, err := processor.Get(testData, "test")
		helper.AssertNoError(err)
		helper.AssertEqual("value", result)

		stats := processor.GetStats()
		helper.AssertTrue(stats.CacheEnabled)
		helper.AssertEqual(config.MaxCacheSize, stats.MaxCacheSize)
	})

	t.Run("SecurityProcessor", func(t *testing.T) {
		processor, _ := New(SecurityConfig())
		defer processor.Close()

		// Test that security limits are enforced
		deepJSON := genNestedJSON(30, "deep")

		_, err := processor.Get(deepJSON, "a")
		// Should error due to depth limit
		if err != nil {
			var jsonErr *JsonsError
			if errors.As(err, &jsonErr) {
				helper.AssertEqual(ErrDepthLimit, jsonErr.Err)
			}
		}
	})

	t.Run("LargeDataProcessor", func(t *testing.T) {
		// Use SecurityConfig with adjusted limits for large data
		config := SecurityConfig()
		config.MaxJSONSize = 100 * 1024 * 1024 // 100MB
		config.MaxNestingDepthSecurity = 100
		config.MaxSecurityValidationSize = 500 * 1024 * 1024
		config.MaxObjectKeys = 50000
		config.MaxArrayElements = 50000
		config.MaxPathDepth = 200
		processor, _ := New(config)
		defer processor.Close()

		// Test with large array
		largeArrayData := genJSONArraySize(10000)

		result := GetArray(largeArrayData, "items", nil)
		helper.AssertTrue(len(result) > 0)
	})

	t.Run("SecurityProcessor_WebAPI", func(t *testing.T) {
		processor, _ := New(SecurityConfig())
		defer processor.Close()

		testData := `{"user": "test", "data": {"id": 123}}`

		// Should work with normal data
		result, err := processor.Get(testData, "user")
		helper.AssertNoError(err)
		helper.AssertEqual("test", result)

		// Verify cache is enabled
		stats := processor.GetStats()
		helper.AssertTrue(stats.CacheEnabled)
	})

	t.Run("DefaultProcessor_Fast", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.FullSecurityScan = false
		processor, _ := New(cfg)
		defer processor.Close()

		testData := `{"items": [1, 2, 3, 4, 5]}`

		result := processor.GetArray(testData, "items", nil)
		helper.AssertEqual(5, len(result))
	})

	t.Run("DefaultProcessor_Minimal", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.EnableCache = false
		processor, _ := New(cfg)
		defer processor.Close()

		testData := `{"test": "value"}`

		// Should work without validation overhead
		result, err := processor.Get(testData, "test")
		helper.AssertNoError(err)
		helper.AssertEqual("value", result)

		// Verify cache is disabled
		stats := processor.GetStats()
		helper.AssertFalse(stats.CacheEnabled)
	})
}

// TestDeleteWithCleanupNullsOption tests deletion with null cleanup using Config
func TestDeleteWithCleanupNullsOption(t *testing.T) {
	jsonStr := `{
		"user": {
			"name": "Alice",
			"age": 30,
			"email": null
		},
		"posts": [
			{"title": "Post 1", "content": null},
			{"title": "Post 2", "content": "Content"}
		]
	}`

	tests := []struct {
		name     string
		path     string
		contains []string
		excludes []string
	}{
		{
			name:     "delete and clean nulls",
			path:     "user.email",
			contains: []string{"name", "age"},
			excludes: []string{"email", "null"},
		},
		{
			name:     "delete entire array",
			path:     "posts",
			contains: []string{"user", "name"},                // Should still have user data
			excludes: []string{"Post 1", "Post 2", "content"}, // All posts content should be gone
		},
	}

	cfg := DefaultConfig()
	cfg.CleanupNulls = true

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := Delete(jsonStr, tt.path, cfg)
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			for _, str := range tt.contains {
				if !strings.Contains(result, str) {
					t.Errorf("Expected result to contain '%s'", str)
				}
			}
			for _, str := range tt.excludes {
				if strings.Contains(result, str) {
					t.Errorf("Expected result to not contain '%s'", str)
				}
			}
		})
	}
}

func TestDelim_TypeMethods(t *testing.T) {
	tests := []struct {
		delim    Delim
		expected string
	}{
		{Delim('['), "["},
		{Delim(']'), "]"},
		{Delim('{'), "{"},
		{Delim('}'), "}"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if tt.delim.String() != tt.expected {
				t.Errorf("Delim.String() = %q, want %q", tt.delim.String(), tt.expected)
			}
		})
	}
}

// TestDistributedOperationPath tests distributed operation patterns
func TestDistributedOperationPath(t *testing.T) {
	// Test that distributed operation paths are detected
	tests := []struct {
		name            string
		path            string
		shouldBeComplex bool
	}{
		{
			name:            "simple extraction",
			path:            "data{items}",
			shouldBeComplex: false, // Not distributed - extraction alone is not distributed
		},
		{
			name:            "extraction with array",
			path:            "data{items}[0]",
			shouldBeComplex: true, // Extraction followed by array access IS distributed
		},
		{
			name:            "extraction with property",
			path:            "data{items}:field",
			shouldBeComplex: true, // Extraction followed by property IS distributed
		},
		{
			name:            "simple path",
			path:            "data.items",
			shouldBeComplex: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isComplex := internal.IsExtractionPath(tt.path)
			if isComplex != tt.shouldBeComplex {
				t.Errorf("isDistributedOperationPath(%s) = %v; want %v", tt.path, isComplex, tt.shouldBeComplex)
			}
		})
	}
}

// TestEncodeBatch tests batch encoding
func TestEncodeBatch(t *testing.T) {
	pairs := map[string]any{
		"user1": map[string]any{"name": "Alice"},
		"user2": map[string]any{"name": "Bob"},
	}

	result, err := EncodeBatch(pairs, DefaultConfig())
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Check that it's a JSON object
	if !strings.Contains(result, "{") || !strings.Contains(result, "}") {
		t.Error("Expected object wrapper")
	}
	if !strings.Contains(result, "user1") || !strings.Contains(result, "user2") {
		t.Error("Expected keys to be present")
	}
}

// TestEncodeFields tests selective field encoding
func TestEncodeFields(t *testing.T) {
	type User struct {
		Name     string `json:"name"`
		Age      int    `json:"age"`
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	user := User{
		Name:     "Alice",
		Age:      30,
		Email:    "alice@example.com",
		Password: "secret123",
	}

	fields := []string{"name", "email"}

	result, err := EncodeFields(user, fields, DefaultConfig())
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Check that only specified fields are present
	if !strings.Contains(result, "name") || !strings.Contains(result, "email") {
		t.Error("Expected specified fields to be present")
	}
	if strings.Contains(result, "password") {
		t.Error("Expected password to be excluded")
	}
	if strings.Contains(result, "age") {
		t.Error("Expected age to be excluded")
	}
}

// TestEncodeStream tests stream encoding
func TestEncodeStream(t *testing.T) {
	values := []any{
		map[string]any{"name": "Alice"},
		map[string]any{"name": "Bob"},
		map[string]any{"name": "Charlie"},
	}

	tests := []struct {
		name        string
		pretty      bool
		expectError bool
		validate    func(t *testing.T, result string)
	}{
		{
			name:        "compact stream",
			pretty:      false,
			expectError: false,
			validate: func(t *testing.T, result string) {
				if !strings.Contains(result, "[") || !strings.Contains(result, "]") {
					t.Error("Expected array wrapper")
				}
			},
		},
		{
			name:        "pretty stream",
			pretty:      true,
			expectError: false,
			validate: func(t *testing.T, result string) {
				if !strings.Contains(result, "\n") {
					t.Error("Expected pretty output with newlines")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := DefaultConfig()
			opts.Pretty = tt.pretty
			result, err := EncodeStream(values, opts)
			if tt.expectError && err == nil {
				t.Errorf("Expected error, but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
			if tt.validate != nil {
				tt.validate(t, result)
			}
		})
	}
}

func TestEncoderDecoder(t *testing.T) {
	t.Run("NewEncoder", func(t *testing.T) {
		var buf bytes.Buffer
		enc := NewEncoder(&buf)
		if enc == nil {
			t.Error("NewEncoder returned nil")
		}
	})

	t.Run("Encoder.Encode", func(t *testing.T) {
		var buf bytes.Buffer
		enc := NewEncoder(&buf)
		err := enc.Encode(map[string]any{"key": "value"})
		if err != nil {
			t.Errorf("Encode error: %v", err)
		}
		if !strings.Contains(buf.String(), "key") {
			t.Error("Encoded output should contain 'key'")
		}
	})

	t.Run("Encoder.SetEscapeHTML", func(t *testing.T) {
		var buf bytes.Buffer
		enc := NewEncoder(&buf)
		enc.SetEscapeHTML(true)
		err := enc.Encode(map[string]any{"html": "<script>"})
		if err != nil {
			t.Errorf("Encode error: %v", err)
		}
		if !strings.Contains(buf.String(), "\\u003c") {
			t.Error("HTML should be escaped")
		}
	})

	t.Run("Encoder.SetIndent", func(t *testing.T) {
		var buf bytes.Buffer
		enc := NewEncoder(&buf)
		enc.SetIndent("", "  ")
		err := enc.Encode(map[string]any{"key": "value"})
		if err != nil {
			t.Errorf("Encode error: %v", err)
		}
		if !strings.Contains(buf.String(), "  ") {
			t.Error("Output should be indented")
		}
	})

	t.Run("NewDecoder", func(t *testing.T) {
		r := strings.NewReader(`{"key": "value"}`)
		dec := NewDecoder(r)
		if dec == nil {
			t.Error("NewDecoder returned nil")
		}
	})

	t.Run("Decoder.Decode", func(t *testing.T) {
		r := strings.NewReader(`{"key": "value"}`)
		dec := NewDecoder(r)
		var result map[string]any
		err := dec.Decode(&result)
		if err != nil {
			t.Errorf("Decode error: %v", err)
		}
		if result["key"] != "value" {
			t.Errorf("result[key] = %v, want 'value'", result["key"])
		}
	})

	t.Run("Decoder.UseNumber", func(t *testing.T) {
		r := strings.NewReader(`{"num": 123}`)
		dec := NewDecoder(r)
		dec.UseNumber()
		var result map[string]any
		err := dec.Decode(&result)
		if err != nil {
			t.Errorf("Decode error: %v", err)
		}
		if _, ok := result["num"].(Number); !ok {
			t.Errorf("result[num] should be Number type, got %T", result["num"])
		}
	})

	t.Run("Decoder.DisallowUnknownFields", func(t *testing.T) {
		r := strings.NewReader(`{"unknown": "field"}`)
		dec := NewDecoder(r)
		dec.DisallowUnknownFields()
		var result struct {
			Known string `json:"known"`
		}
		_ = dec.Decode(&result) // Just verify it doesn't panic
	})

	t.Run("Decoder.Buffered", func(t *testing.T) {
		r := strings.NewReader(`{"key": "value"}`)
		dec := NewDecoder(r)
		var result map[string]any
		dec.Decode(&result)
		buffered := dec.Buffered()
		if buffered == nil {
			t.Error("Buffered should not return nil")
		}
	})

	t.Run("Decoder.InputOffset", func(t *testing.T) {
		r := strings.NewReader(`{"key": "value"}`)
		dec := NewDecoder(r)
		var result map[string]any
		dec.Decode(&result)
		offset := dec.InputOffset()
		if offset == 0 {
			t.Log("InputOffset returned 0")
		}
	})

	t.Run("Decoder.More", func(t *testing.T) {
		r := strings.NewReader(`{"a":1}{"b":2}`)
		dec := NewDecoder(r)

		var result map[string]any
		dec.Decode(&result)
		if !dec.More() {
			t.Error("More should return true for second JSON")
		}
		dec.Decode(&result)
		if dec.More() {
			t.Error("More should return false after all JSON consumed")
		}
	})

	t.Run("Decoder.Decode nil", func(t *testing.T) {
		r := strings.NewReader(`{"key": "value"}`)
		dec := NewDecoder(r)
		err := dec.Decode(nil)
		if err == nil {
			t.Error("Decode(nil) should return error")
		}
	})

	t.Run("Decoder.Decode non-pointer", func(t *testing.T) {
		r := strings.NewReader(`{"key": "value"}`)
		dec := NewDecoder(r)
		var result map[string]any
		err := dec.Decode(result) // Not a pointer
		if err == nil {
			t.Error("Decode(non-pointer) should return error")
		}
	})
}

// TestEncodingConfiguration tests encoding configuration
func TestEncodingConfiguration(t *testing.T) {
	helper := newTestHelper(t)

	t.Run("DefaultEncodeConfig", func(t *testing.T) {
		config := DefaultConfig()

		helper.AssertNotNil(config)
		helper.AssertFalse(config.Pretty)
		helper.AssertEqual("  ", config.Indent)
		helper.AssertEqual("", config.Prefix)
		helper.AssertTrue(config.EscapeHTML)
		helper.AssertFalse(config.SortKeys)
		helper.AssertTrue(config.ValidateUTF8)
		helper.AssertEqual(100, config.MaxDepth)
		helper.AssertFalse(config.DisallowUnknown)
		helper.AssertFalse(config.PreserveNumbers)
		helper.AssertEqual(-1, config.FloatPrecision)
		helper.AssertFalse(config.DisableEscaping)
		helper.AssertFalse(config.EscapeUnicode)
		helper.AssertFalse(config.EscapeSlash)
		helper.AssertTrue(config.EscapeNewlines)
		helper.AssertTrue(config.EscapeTabs)
		helper.AssertTrue(config.IncludeNulls)
	})

	t.Run("PrettyEncodeConfig", func(t *testing.T) {
		config := PrettyConfig()

		helper.AssertTrue(config.Pretty)
		helper.AssertEqual("  ", config.Indent)
	})
}

// TestExtractSyntaxComplex tests complex extraction syntax scenarios
func TestExtractSyntaxComplex(t *testing.T) {
	jsonStr := `{
		"users": [
			{"name": "Alice", "age": 30, "address": {"city": "NYC"}},
			{"name": "Bob", "age": 25, "address": {"city": "LA"}},
			{"name": "Charlie", "age": 35, "address": {"city": "SF"}}
		],
		"departments": [
			{"name": "Engineering", "head": "Alice"},
			{"name": "Sales", "head": "Bob"}
		]
	}`

	processor, _ := New()
	defer processor.Close()

	tests := []struct {
		name     string
		path     string
		validate func(t *testing.T, result any, err error)
	}{
		{
			name: "simple extraction",
			path: "users{name}",
			validate: func(t *testing.T, result any, err error) {
				if err != nil {
					t.Fatalf("Unexpected error: %v", err)
				}
				arr, ok := result.([]any)
				if !ok || len(arr) != 3 {
					t.Errorf("Expected array of 3 names, got: %v", result)
				}
			},
		},
		{
			name: "flat extraction",
			path: "users{flat:address.city}",
			validate: func(t *testing.T, result any, err error) {
				if err != nil {
					t.Fatalf("Unexpected error: %v", err)
				}
				// Flat extraction should flatten nested arrays
				if result == nil {
					t.Error("Expected result, got nil")
				}
			},
		},
		{
			name: "extraction with array access",
			path: "users{name}[0]",
			validate: func(t *testing.T, result any, err error) {
				if err != nil {
					t.Fatalf("Unexpected error: %v", err)
				}
				// Should return first name
				if result == nil {
					t.Error("Expected result, got nil")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := processor.Get(jsonStr, tt.path)
			tt.validate(t, result, err)
		})
	}
}

// TestExtractionOperations tests extraction operations
func TestExtractionOperations(t *testing.T) {
	processor, _ := New()
	defer processor.Close()

	t.Run("extract array field", func(t *testing.T) {
		jsonStr := `{"users": [{"name": "Alice", "age": 30}, {"name": "Bob", "age": 25}]}`
		result, err := processor.Get(jsonStr, "users{name}")
		if err != nil {
			t.Fatalf("Get extract error: %v", err)
		}
		// Verify extraction result
		arr, ok := result.([]any)
		if !ok {
			t.Fatalf("Expected array, got %T", result)
		}
		if len(arr) != 2 {
			t.Errorf("Expected 2 extracted items, got %d", len(arr))
		}
	})
}

// TestMultiFieldExtraction tests multi-field extraction syntax {field1,field2}
func TestMultiFieldExtraction(t *testing.T) {
	processor, _ := New()
	defer processor.Close()

	t.Run("extract multiple fields from single object", func(t *testing.T) {
		jsonStr := `{"user": {"id": 1001, "name": "Alice", "email": "alice@example.com", "password": "secret"}}`
		result, err := processor.Get(jsonStr, "user{id,name}")
		if err != nil {
			t.Fatalf("Get error: %v", err)
		}

		obj, ok := result.(map[string]any)
		if !ok {
			t.Fatalf("Expected map[string]any, got %T", result)
		}

		// Should have exactly 2 fields
		if len(obj) != 2 {
			t.Errorf("Expected 2 fields, got %d", len(obj))
		}

		// Verify id field
		if id, exists := obj["id"]; !exists {
			t.Error("Missing 'id' field")
		} else if id.(float64) != 1001 {
			t.Errorf("Expected id=1001, got %v", id)
		}

		// Verify name field
		if name, exists := obj["name"]; !exists {
			t.Error("Missing 'name' field")
		} else if name != "Alice" {
			t.Errorf("Expected name='Alice', got %v", name)
		}

		// Should NOT have email or password
		if _, exists := obj["email"]; exists {
			t.Error("Should not have 'email' field")
		}
		if _, exists := obj["password"]; exists {
			t.Error("Should not have 'password' field")
		}
	})

	t.Run("extract multiple fields with whitespace", func(t *testing.T) {
		jsonStr := `{"data": {"a": 1, "b": 2, "c": 3}}`
		result, err := processor.Get(jsonStr, "data{a, b, c}")
		if err != nil {
			t.Fatalf("Get error: %v", err)
		}

		obj, ok := result.(map[string]any)
		if !ok {
			t.Fatalf("Expected map[string]any, got %T", result)
		}

		if len(obj) != 3 {
			t.Errorf("Expected 3 fields, got %d", len(obj))
		}
	})

	t.Run("extract multiple fields from array of objects", func(t *testing.T) {
		jsonStr := `{"users": [
			{"id": 1, "name": "Alice", "email": "a@example.com"},
			{"id": 2, "name": "Bob", "email": "b@example.com"}
		]}`
		result, err := processor.Get(jsonStr, "users{id,name}")
		if err != nil {
			t.Fatalf("Get error: %v", err)
		}

		arr, ok := result.([]any)
		if !ok {
			t.Fatalf("Expected []any, got %T", result)
		}

		if len(arr) != 2 {
			t.Fatalf("Expected 2 items, got %d", len(arr))
		}

		// Verify first item
		obj1, ok := arr[0].(map[string]any)
		if !ok {
			t.Fatalf("Expected map, got %T", arr[0])
		}
		if len(obj1) != 2 {
			t.Errorf("Expected 2 fields in first item, got %d", len(obj1))
		}

		// Verify second item
		obj2, ok := arr[1].(map[string]any)
		if !ok {
			t.Fatalf("Expected map, got %T", arr[1])
		}
		if len(obj2) != 2 {
			t.Errorf("Expected 2 fields in second item, got %d", len(obj2))
		}
	})

	t.Run("extract with non-existent field", func(t *testing.T) {
		jsonStr := `{"data": {"a": 1, "b": 2}}`
		result, err := processor.Get(jsonStr, "data{a,nonexistent}")
		if err != nil {
			t.Fatalf("Get error: %v", err)
		}

		obj, ok := result.(map[string]any)
		if !ok {
			t.Fatalf("Expected map[string]any, got %T", result)
		}

		// Should only have 'a' field
		if len(obj) != 1 {
			t.Errorf("Expected 1 field, got %d", len(obj))
		}
		if _, exists := obj["a"]; !exists {
			t.Error("Missing 'a' field")
		}
	})

	t.Run("single field extraction returns value (backward compatible)", func(t *testing.T) {
		jsonStr := `{"user": {"id": 1001, "name": "Alice"}}`
		result, err := processor.Get(jsonStr, "user{id}")
		if err != nil {
			t.Fatalf("Get error: %v", err)
		}

		// Single field should return the value, not a map
		id, ok := result.(float64)
		if !ok {
			t.Fatalf("Expected numeric value, got %T", result)
		}
		if id != 1001 {
			t.Errorf("Expected id=1001, got %v", id)
		}
	})

	t.Run("extract three fields", func(t *testing.T) {
		jsonStr := `{"user": {"id": 1, "name": "Test", "email": "test@example.com", "age": 30}}`
		result, err := processor.Get(jsonStr, "user{id,name,email}")
		if err != nil {
			t.Fatalf("Get error: %v", err)
		}

		obj, ok := result.(map[string]any)
		if !ok {
			t.Fatalf("Expected map[string]any, got %T", result)
		}

		if len(obj) != 3 {
			t.Errorf("Expected 3 fields, got %d: %v", len(obj), obj)
		}

		// Verify all fields present
		if _, exists := obj["id"]; !exists {
			t.Error("Missing 'id' field")
		}
		if _, exists := obj["name"]; !exists {
			t.Error("Missing 'name' field")
		}
		if _, exists := obj["email"]; !exists {
			t.Error("Missing 'email' field")
		}
		if _, exists := obj["age"]; exists {
			t.Error("Should not have 'age' field")
		}
	})
}

// TestCompactString tests compact formatting via processor
func TestCompactString(t *testing.T) {
	prettyJSON := `{
		"user": {
			"name": "Alice",
			"age": 30
		}
	}`

	p, err := New()
	if err != nil {
		t.Fatalf("Failed to create processor: %v", err)
	}
	defer p.Close()

	result, err := p.Compact(prettyJSON)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Check that it's compact
	if strings.Contains(result, "\n") {
		t.Error("Expected compact output to not contain newlines")
	}
}

// TestPrettify tests pretty formatting
func TestPrettify(t *testing.T) {
	compactJSON := `{"user":{"name":"Alice","age":30},"settings":{"theme":"dark"}}`

	result, err := Prettify(compactJSON)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Check for indentation
	if !strings.Contains(result, "\n") {
		t.Error("Expected formatted output to contain newlines")
	}
	if !strings.Contains(result, "  ") {
		t.Error("Expected formatted output to contain indentation")
	}
}

// TestForwardSlice tests forward slicing logic
func TestForwardSlice(t *testing.T) {
	processor, _ := New()
	defer processor.Close()

	arr := []any{0.0, 1.0, 2.0, 3.0, 4.0}

	tests := []struct {
		name     string
		start    int
		end      int
		step     int
		expected []any
	}{
		{
			name:     "normal slice",
			start:    1,
			end:      4,
			step:     1,
			expected: []any{1.0, 2.0, 3.0},
		},
		{
			name:     "with step",
			start:    0,
			end:      5,
			step:     2,
			expected: []any{0.0, 2.0, 4.0},
		},
		{
			name:     "start negative (Python-style: -1 means last element)",
			start:    -1,
			end:      5,
			step:     1,
			expected: []any{4.0},
		},
		{
			name:     "start negative from beginning",
			start:    -5,
			end:      4,
			step:     1,
			expected: []any{0.0, 1.0, 2.0, 3.0},
		},
		{
			name:     "end beyond length",
			start:    2,
			end:      10,
			step:     1,
			expected: []any{2.0, 3.0, 4.0},
		},
		{
			name:     "empty result",
			start:    3,
			end:      1,
			step:     1,
			expected: []any{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := internal.PerformArraySlice(arr, &tt.start, &tt.end, &tt.step)
			if !slicesEqual(result, tt.expected) {
				t.Errorf("forwardSlice(%d, %d, %d) = %v; want %v", tt.start, tt.end, tt.step, result, tt.expected)
			}
		})
	}
}

// TestGetOrSlice tests GetTyped[[]any] function
// TestGetTypedDefaults consolidates the former GetOr/GetOrGeneric/GetOrSlice/
// GetOrBool/GetOrFloat64/GetOrMap tests. GetTyped[T] returns the stored value
// when found (with best-effort numeric conversion) and the default when the
// path is missing or the value is type-incompatible.
func TestGetTypedDefaults(t *testing.T) {
	cases := []struct {
		name     string
		jsonStr  string
		path     string
		def      any
		expected any
	}{
		// string
		{"string existing", `{"user":{"name":"Alice","age":30}}`, "user.name", "Unknown", "Alice"},
		{"string missing returns default", `{"user":{"name":"Alice"}}`, "user.email", "no@email.com", "no@email.com"},
		// int
		{"int existing", `{"user":{"age":30}}`, "user.age", 0, 30},
		{"int missing returns default", `{"user":{}}`, "user.score", 100, 100},
		// float64
		{"float existing", `{"price":19.99}`, "price", 0.0, 19.99},
		{"int converted to float", `{"count":5}`, "count", 0.0, 5.0},
		{"float missing returns default", `{}`, "missing", 99.99, 99.99},
		// bool
		{"bool existing true", `{"enabled":true}`, "enabled", false, true},
		{"bool existing false over default", `{"disabled":false}`, "disabled", true, false},
		{"bool missing returns default", `{}`, "missing", true, true},
		// []any
		{"slice existing", `{"items":[1,2,3]}`, "items", []any{"default"}, []any{1.0, 2.0, 3.0}},
		{"slice missing returns default", `{}`, "missing", []any{"default"}, []any{"default"}},
		// map[string]any
		{"map existing", `{"config":{"theme":"dark"}}`, "config", map[string]any{"default": true}, map[string]any{"theme": "dark"}},
		{"map missing returns default", `{}`, "missing", map[string]any{"default": true}, map[string]any{"default": true}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			switch def := tc.def.(type) {
			case string:
				got := GetTyped[string](tc.jsonStr, tc.path, def)
				if want := tc.expected.(string); got != want {
					t.Errorf("GetTyped[string](%s) = %q, want %q", tc.path, got, want)
				}
			case int:
				got := GetTyped[int](tc.jsonStr, tc.path, def)
				if want := tc.expected.(int); got != want {
					t.Errorf("GetTyped[int](%s) = %d, want %d", tc.path, got, want)
				}
			case float64:
				got := GetTyped[float64](tc.jsonStr, tc.path, def)
				if want := tc.expected.(float64); got != want {
					t.Errorf("GetTyped[float64](%s) = %v, want %v", tc.path, got, want)
				}
			case bool:
				got := GetTyped[bool](tc.jsonStr, tc.path, def)
				if want := tc.expected.(bool); got != want {
					t.Errorf("GetTyped[bool](%s) = %v, want %v", tc.path, got, want)
				}
			case []any:
				got := GetTyped[[]any](tc.jsonStr, tc.path, def)
				want := tc.expected.([]any)
				if len(got) != len(want) {
					t.Fatalf("slice length = %d, want %d (got=%v)", len(got), len(want), got)
				}
				for i := range want {
					if got[i] != want[i] {
						t.Errorf("slice[%d] = %v, want %v", i, got[i], want[i])
					}
				}
			case map[string]any:
				got := GetTyped[map[string]any](tc.jsonStr, tc.path, def)
				want := tc.expected.(map[string]any)
				if len(got) != len(want) {
					t.Fatalf("map size = %d, want %d (got=%v)", len(got), len(want), got)
				}
				for k, v := range want {
					if got[k] != v {
						t.Errorf("map[%q] = %v, want %v", k, got[k], v)
					}
				}
			default:
				t.Fatalf("unsupported default type %T", tc.def)
			}
		})
	}
}

// TestGetHealthStatus tests health status retrieval
func TestGetHealthStatus(t *testing.T) {
	status := GetHealthStatus()

	// Just verify we can get health status without panicking
	// The actual healthy status depends on whether metrics are initialized
	if status.Timestamp.IsZero() {
		t.Error("Expected non-zero timestamp")
	}

	if len(status.Checks) == 0 {
		t.Error("Expected some health checks to be present")
	}

	// At minimum, memory check should be present
	if _, ok := status.Checks["memory"]; !ok {
		t.Error("Expected memory check to be present")
	}

	t.Logf("Health status: %+v", status)
}

// TestGetMultiple tests retrieving multiple values at once
func TestGetMultiple(t *testing.T) {
	jsonStr := `{
		"user": {
			"name": "Alice",
			"age": 30,
			"email": "alice@example.com"
		},
		"settings": {
			"theme": "dark",
			"language": "en"
		}
	}`

	tests := []struct {
		name        string
		paths       []string
		expectedLen int
		expectError bool
	}{
		{
			name:        "multiple paths",
			paths:       []string{"user.name", "user.age", "settings.theme"},
			expectedLen: 3,
			expectError: false,
		},
		{
			name:        "single path",
			paths:       []string{"user.name"},
			expectedLen: 1,
			expectError: false,
		},
		{
			name:        "empty paths",
			paths:       []string{},
			expectedLen: 0,
			expectError: false,
		},
		{
			name:        "mixed valid and invalid",
			paths:       []string{"user.name", "invalid.path", "settings.theme"},
			expectedLen: 3, // GetMultiple returns entries for all paths, including nil for invalid ones
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := GetMultiple(jsonStr, tt.paths)
			if tt.expectError && err == nil {
				t.Errorf("Expected error, but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
			if !tt.expectError && len(result) != tt.expectedLen {
				t.Errorf("Result length = %d; want %d", len(result), tt.expectedLen)
			}
		})
	}
}

// TestGetOrMap tests GetTyped[map[string]any] function
// TestGetStats tests statistics retrieval from the global processor.
func TestGetStats(t *testing.T) {
	stats := GetStats()

	// All counters are non-negative by contract.
	if stats.CacheSize < 0 || stats.HitCount < 0 || stats.MissCount < 0 || stats.OperationCount < 0 {
		t.Errorf("Stats has a negative counter: %+v", stats)
	}
	// HitRatio, when reported, is a probability in [0, 1].
	if stats.HitRatio < 0 || stats.HitRatio > 1 {
		t.Errorf("HitRatio out of [0,1]: %v", stats.HitRatio)
	}
}

// TestGlobalProcessor_BasicFunctionality tests basic global processor operations
func TestGlobalProcessor_BasicFunctionality(t *testing.T) {
	// Reset global processor state
	ShutdownGlobalProcessor()

	t.Run("GetOrProcessorCreatesProcessor", func(t *testing.T) {
		p := getDefaultProcessor()
		if p == nil {
			t.Fatal("getDefaultProcessor returned nil")
		}
		if p.IsClosed() {
			t.Error("Default processor should not be closed")
		}
	})

	t.Run("GetOrProcessorReturnsSameInstance", func(t *testing.T) {
		p1 := getDefaultProcessor()
		p2 := getDefaultProcessor()

		if p1 != p2 {
			t.Error("getDefaultProcessor should return the same instance")
		}
	})

	// Cleanup
	ShutdownGlobalProcessor()
}

// TestGlobalProcessor_ConcurrentAccess tests concurrent access to global processor
func TestGlobalProcessor_ConcurrentAccess(t *testing.T) {
	// Reset state
	ShutdownGlobalProcessor()

	t.Run("ConcurrentGetOrProcessor", func(t *testing.T) {
		const goroutines = 100
		var wg sync.WaitGroup
		processors := make(chan *Processor, goroutines)

		for i := 0; i < goroutines; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				p := getDefaultProcessor()
				processors <- p
			}()
		}

		wg.Wait()
		close(processors)

		// All goroutines should get the same processor instance
		var firstProcessor *Processor
		count := 0
		for p := range processors {
			count++
			if firstProcessor == nil {
				firstProcessor = p
			} else if firstProcessor != p {
				t.Error("All goroutines should get the same processor instance")
				break
			}
		}

		if count != goroutines {
			t.Errorf("Received %d processors, want %d", count, goroutines)
		}
	})

	t.Run("ConcurrentSetAndGet", func(t *testing.T) {
		ShutdownGlobalProcessor()

		const goroutines = 50
		var wg sync.WaitGroup

		for i := 0; i < goroutines; i++ {
			wg.Add(2)

			// Goroutine that sets processor
			go func(id int) {
				defer wg.Done()
				config := DefaultConfig()
				config.MaxCacheSize = 100 + id
				p, _ := New(config)
				SetGlobalProcessor(p)
			}(i)

			// Goroutine that gets processor
			go func() {
				defer wg.Done()
				_ = getDefaultProcessor()
			}()
		}

		wg.Wait()
	})

	// Cleanup
	ShutdownGlobalProcessor()
}

// TestGlobalProcessor_IntegrationWithStats tests that global processor stats work correctly
func TestGlobalProcessor_IntegrationWithStats(t *testing.T) {
	// Reset state
	ShutdownGlobalProcessor()

	// Perform some operations
	jsonStr := `{"test":"value"}`
	for i := 0; i < 10; i++ {
		_, _ = Get(jsonStr, "test")
	}

	// Get stats
	stats := GetStats()

	if stats.IsClosed {
		t.Error("Global processor should not be closed")
	}

	// OperationCount should be at least 10
	if stats.OperationCount < 10 {
		t.Errorf("OperationCount = %d, want at least 10", stats.OperationCount)
	}

	// Cleanup
	ShutdownGlobalProcessor()
}

// TestGlobalProcessor_Lifecycle tests the complete lifecycle of global processor
func TestGlobalProcessor_Lifecycle(t *testing.T) {
	t.Run("CompleteLifecycle", func(t *testing.T) {
		// Start fresh
		ShutdownGlobalProcessor()

		// 1. Get default processor (creates new)
		p1 := getDefaultProcessor()
		if p1 == nil {
			t.Fatal("Expected non-nil processor")
		}

		// 2. Use the processor
		jsonStr := `{"key":"value"}`
		result, err := Get(jsonStr, "key")
		if err != nil {
			t.Errorf("Get failed: %v", err)
		}
		if result != "value" {
			t.Errorf("result = %v, want value", result)
		}

		// 3. Set a custom processor
		customConfig := DefaultConfig()
		customConfig.MaxCacheSize = 256
		p2, _ := New(customConfig)
		SetGlobalProcessor(p2)

		// 4. Verify custom processor is used
		if getDefaultProcessor() != p2 {
			t.Error("Custom processor should be active")
		}

		// 5. Shutdown
		ShutdownGlobalProcessor()

		// 6. Verify new processor is created after shutdown
		p3 := getDefaultProcessor()
		if p3 == p2 {
			t.Error("New processor should be created after shutdown")
		}

		// Final cleanup
		ShutdownGlobalProcessor()
	})
}

// TestGlobalProcessor_ReplaceClosesOld tests that replacing global processor closes the old one
func TestGlobalProcessor_ReplaceClosesOld(t *testing.T) {
	// Reset state
	ShutdownGlobalProcessor()

	// Create first processor
	p1, _ := New(DefaultConfig())
	SetGlobalProcessor(p1)

	// Create second processor
	p2, _ := New(DefaultConfig())
	SetGlobalProcessor(p2)

	// Give it time to close
	time.Sleep(10 * time.Millisecond)

	// Old processor should be closed
	if !p1.IsClosed() {
		t.Error("Old processor should be closed when replaced")
	}

	// New processor should be the global processor
	if getDefaultProcessor() != p2 {
		t.Error("New processor should be the global processor")
	}

	// Cleanup
	ShutdownGlobalProcessor()
}

// TestGlobalProcessor_ThreadSafety tests thread safety of global processor operations
func TestGlobalProcessor_ThreadSafety(t *testing.T) {
	// Reset state
	ShutdownGlobalProcessor()

	const iterations = 100
	const goroutines = 20

	var wg sync.WaitGroup
	errors := make(chan error, goroutines*iterations)

	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				// Mix of operations
				switch i % 5 {
				case 0:
					_ = getDefaultProcessor()
				case 1:
					_, err := Get(`{"test":"value"}`, "test")
					if err != nil {
						errors <- err
					}
				case 2:
					_, err := Set(`{"test":"old"}`, "test", "new")
					if err != nil {
						errors <- err
					}
				case 3:
					// Occasionally set a new processor
					if i%20 == 0 {
						config := DefaultConfig()
						config.MaxCacheSize = 100 + goroutineID
						p, _ := New(config)
						SetGlobalProcessor(p)
					}
				case 4:
					// Occasionally check stats
					_ = GetStats()
				}
			}
		}(g)
	}

	wg.Wait()
	close(errors)

	for err := range errors {
		t.Errorf("Concurrent operation error: %v", err)
	}

	// Cleanup
	ShutdownGlobalProcessor()
}

// TestGlobalProcessor_WithPackageFunctions tests that package functions work with global processor
func TestGlobalProcessor_WithPackageFunctions(t *testing.T) {
	// Reset state
	ShutdownGlobalProcessor()

	t.Run("PackageGetUsesGlobalProcessor", func(t *testing.T) {
		testData := `{"name":"test","value":123}`

		result, err := Get(testData, "name")
		if err != nil {
			t.Errorf("Get failed: %v", err)
		}
		if result != "test" {
			t.Errorf("result = %v, want test", result)
		}
	})

	t.Run("PackageSetUsesGlobalProcessor", func(t *testing.T) {
		testData := `{"name":"old"}`

		result, err := Set(testData, "name", "new")
		if err != nil {
			t.Errorf("Set failed: %v", err)
		}
		if result != `{"name":"new"}` {
			t.Errorf("result = %s, want {\"name\":\"new\"}", result)
		}
	})

	t.Run("PackageDeleteUsesGlobalProcessor", func(t *testing.T) {
		testData := `{"name":"test","value":123}`

		result, err := Delete(testData, "name")
		if err != nil {
			t.Errorf("Delete failed: %v", err)
		}
		if result != `{"value":123}` {
			t.Errorf("result = %s, want {\"value\":123}", result)
		}
	})

	// Cleanup
	ShutdownGlobalProcessor()
}

// TestHTMLEscape tests HTMLEscape function
func TestHTMLEscape(t *testing.T) {
	htmlContent := `{"html":"<script>alert('xss')</script>","amp":"a & b"}`

	var buf bytes.Buffer
	HTMLEscape(&buf, []byte(htmlContent))

	result := buf.String()
	// Check that HTML characters are escaped
	if strings.Contains(result, "<script>") {
		t.Errorf("HTMLEscape should escape HTML, got: %s", result)
	}
}

// TestHandleArrayAccess tests array access handling
func TestHandleArrayAccess(t *testing.T) {
	processor, _ := New()
	defer processor.Close()

	jsonStr := `{
		"items": [10, 20, 30, 40, 50],
		"nested": {
			"arr": [1, 2, 3]
		}
	}`

	var data any
	if err := processor.Parse(jsonStr, &data); err != nil {
		t.Fatalf("Failed to parse test data: %v", err)
	}

	tests := []struct {
		name        string
		data        any
		segment     internal.PathSegment
		expectedVal any
		shouldExist bool
	}{
		{
			name: "valid positive index",
			data: getProp(data, "items"),
			segment: internal.PathSegment{
				Type:  internal.ArrayIndexSegment,
				Index: 2,
			},
			expectedVal: 30.0,
			shouldExist: true,
		},
		{
			name: "negative index",
			data: getProp(data, "items"),
			segment: internal.PathSegment{
				Type:  internal.ArrayIndexSegment,
				Index: -1,
			},
			expectedVal: 50.0,
			shouldExist: true,
		},
		{
			name: "out of bounds positive",
			data: getProp(data, "items"),
			segment: internal.PathSegment{
				Type:  internal.ArrayIndexSegment,
				Index: 10,
			},
			expectedVal: nil,
			shouldExist: false,
		},
		{
			name: "out of bounds negative",
			data: getProp(data, "items"),
			segment: internal.PathSegment{
				Type:  internal.ArrayIndexSegment,
				Index: -10,
			},
			expectedVal: nil,
			shouldExist: false,
		},
		{
			name: "with property key",
			data: getProp(getProp(data, "nested"), "arr"),
			segment: internal.PathSegment{
				Type:  internal.ArrayIndexSegment,
				Index: 1,
				Key:   "",
			},
			expectedVal: 2.0,
			shouldExist: true,
		},
		{
			name: "invalid data type",
			data: "not an array",
			segment: internal.PathSegment{
				Type:  internal.ArrayIndexSegment,
				Index: 0,
			},
			expectedVal: nil,
			shouldExist: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := processor.handleArrayAccess(tt.data, tt.segment)
			if result.exists != tt.shouldExist {
				t.Errorf("handleArrayAccess() existence = %v; want %v", result.exists, tt.shouldExist)
			}
			if tt.shouldExist && result.value != tt.expectedVal {
				t.Errorf("handleArrayAccess() value = %v; want %v", result.value, tt.expectedVal)
			}
		})
	}
}

// TestHandleExtraction tests field extraction from objects/arrays
func TestHandleExtraction(t *testing.T) {
	processor, _ := New()
	defer processor.Close()

	tests := []struct {
		name        string
		data        any
		segment     internal.PathSegment
		expectedLen int
		expectError bool
	}{
		{
			name: "extract from array",
			data: []any{
				map[string]any{"name": "Alice", "age": 30},
				map[string]any{"name": "Bob", "age": 25},
				map[string]any{"name": "Charlie", "age": 35},
			},
			segment: internal.PathSegment{
				Type: internal.ExtractSegment,
				Key:  "name",
			},
			expectedLen: 3,
			expectError: false,
		},
		{
			name: "extract from single object",
			data: map[string]any{
				"name": "Alice",
				"age":  30,
			},
			segment: internal.PathSegment{
				Type: internal.ExtractSegment,
				Key:  "name",
			},
			expectedLen: 0, // Single value, not an array
			expectError: false,
		},
		{
			name: "extract missing field",
			data: []any{
				map[string]any{"age": 30},
				map[string]any{"age": 25},
			},
			segment: internal.PathSegment{
				Type: internal.ExtractSegment,
				Key:  "name",
			},
			expectedLen: 0,
			expectError: false,
		},
		{
			name: "flat extraction",
			data: []any{
				map[string]any{"items": []any{1, 2}},
				map[string]any{"items": []any{3, 4}},
			},
			segment: internal.PathSegment{
				Type:  internal.ExtractSegment,
				Key:   "items",
				Flags: internal.FlagIsFlat,
			},
			expectedLen: 4, // Flattened: [1, 2, 3, 4]
			expectError: false,
		},
		{
			name:        "invalid data type",
			data:        "not extractable",
			segment:     internal.PathSegment{Type: internal.ExtractSegment, Key: "name"},
			expectedLen: 0,
			expectError: false,
		},
		{
			name: "empty key",
			data: []any{
				map[string]any{"name": "Alice"},
			},
			segment: internal.PathSegment{
				Type: internal.ExtractSegment,
				Key:  "",
			},
			expectedLen: 0,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := processor.handleExtraction(tt.data, tt.segment)
			if tt.expectError && err == nil {
				t.Errorf("Expected error, but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
			if tt.expectedLen > 0 {
				arr, ok := result.([]any)
				if !ok {
					t.Errorf("Expected []any result, got %T", result)
				} else if len(arr) != tt.expectedLen {
					t.Errorf("Result length = %d; want %d", len(arr), tt.expectedLen)
				}
			}
		})
	}
}

// TestHandlePropertyAccess tests property access handling
func TestHandleStructAccess(t *testing.T) {
	processor, _ := New()
	defer processor.Close()

	type TestStruct struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}

	t.Run("access struct field", func(t *testing.T) {
		data := TestStruct{Name: "test", Age: 30}
		jsonBytes, err := processor.Marshal(data)
		if err != nil {
			t.Fatalf("Marshal error: %v", err)
		}

		result, err := processor.Get(string(jsonBytes), "name")
		if err != nil {
			t.Fatalf("Get error: %v", err)
		}
		if result != "test" {
			t.Errorf("Get struct field = %v, want test", result)
		}
	})

	t.Run("access nested struct", func(t *testing.T) {
		type Nested struct {
			Value string `json:"value"`
		}
		type Outer struct {
			Nested Nested `json:"nested"`
		}

		data := Outer{Nested: Nested{Value: "nested_value"}}
		jsonBytes, err := processor.Marshal(data)
		if err != nil {
			t.Fatalf("Marshal error: %v", err)
		}

		result, err := processor.Get(string(jsonBytes), "nested.value")
		if err != nil {
			t.Fatalf("Get error: %v", err)
		}
		if result != "nested_value" {
			t.Errorf("Get nested field = %v, want nested_value", result)
		}
	})
}

// TestIndent tests Indent function
func TestIndent(t *testing.T) {
	compactJSON := `{"name":"Alice","age":30}`

	var buf bytes.Buffer
	err := Indent(&buf, []byte(compactJSON), "", "  ")
	if err != nil {
		t.Fatalf("Indent error: %v", err)
	}

	result := buf.String()
	if !strings.Contains(result, "\n") {
		t.Errorf("Indent should add newlines, got: %s", result)
	}
	if !strings.Contains(result, "  ") {
		t.Errorf("Indent should add indentation, got: %s", result)
	}
}

// TestIsEmptyOrZero is covered by TestIsEmptyOrZeroExtended in test_helpers_new_test.go

func TestNullAndMissingFields(t *testing.T) {
	helper := newTestHelper(t)

	testData := `{
		"null_field": null,
		"string_field": "value",
		"nested": {
			"null_nested": null,
			"valid_nested": "data"
		},
		"array_with_nulls": [1, null, 3, null, 5]
	}`

	t.Run("NestedNullAccess", func(t *testing.T) {
		result, err := Get(testData, "nested.null_nested")
		helper.AssertNoError(err)
		helper.AssertNil(result)
	})

	t.Run("ArrayNullAccess", func(t *testing.T) {
		result, err := Get(testData, "array_with_nulls[1]")
		helper.AssertNoError(err)
		helper.AssertNil(result)
	})

	t.Run("NullWithDefault", func(t *testing.T) {
		result := GetTyped[string](testData, "null_field", "default")
		helper.AssertEqual("default", result)

		result = GetTyped[string](testData, "missing_field", "default")
		helper.AssertEqual("default", result)

		result = GetTyped[string](testData, "string_field", "default")
		helper.AssertEqual("value", result)
	})

	t.Run("IsNullMethod", func(t *testing.T) {
		Foreach(testData, func(key any, item *IterableValue) {
			if key == "null_field" {
				helper.AssertTrue(item.IsNull(""))
			}
		})
	})
}

// TestParseArrayIndex tests the internal ParseArrayIndex function
func TestParseArrayIndex(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expectedIdx int
		expectedOK  bool
	}{
		{"Valid index", "5", 5, true},
		{"Invalid index", "abc", 0, false},
		{"Empty string", "", 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, ok := internal.ParseArrayIndex(tt.input)
			if result != tt.expectedIdx || ok != tt.expectedOK {
				t.Errorf("ParseArrayIndex(%q) = (%d, %v), want (%d, %v)", tt.input, result, ok, tt.expectedIdx, tt.expectedOK)
			}
		})
	}
}

// splitPathSegments and joinPathSegments mirror the removed (*Processor) helpers
// of the same names. Kept here purely to exercise the simple dot/JSON-pointer
// segment semantics covered by TestPathCombination and TestPathSegmentation;
// production code uses the richer internal path parser.
func TestPathParsingBasic(t *testing.T) {
	processor, _ := New()
	defer processor.Close()

	tests := []struct {
		name        string
		path        string
		expectError bool
		description string
	}{
		{
			name:        "simple property",
			path:        "user.name",
			expectError: false,
			description: "Simple dot-notation path",
		},
		{
			name:        "array index",
			path:        "users[0]",
			expectError: false,
			description: "Path with array index",
		},
		{
			name:        "nested array",
			path:        "data.users[0].name",
			expectError: false,
			description: "Nested path with array",
		},
		{
			name:        "array slice",
			path:        "items[0:5]",
			expectError: false,
			description: "Path with array slice",
		},
		{
			name:        "extraction",
			path:        "users{name}",
			expectError: false,
			description: "Path with field extraction",
		},
		{
			name:        "empty path",
			path:        "",
			expectError: false,
			description: "Empty path (root)",
		},
		{
			name:        "root path",
			path:        ".",
			expectError: false,
			description: "Root path notation",
		},
		{
			name:        "JSON pointer",
			path:        "/users/0/name",
			expectError: false,
			description: "JSON Pointer format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Just verify the path can be parsed without error
			segments, err := processor.parsePath(tt.path)
			if tt.expectError && err == nil {
				t.Errorf("Expected error for path '%s', but got none", tt.path)
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error for path '%s': %v", tt.path, err)
			}
			if !tt.expectError && len(segments) == 0 && tt.path != "" && tt.path != "." {
				t.Errorf("Expected segments for path '%s', got none", tt.path)
			}
		})
	}
}

// TestPathParsingBoundaryConditions tests edge cases in path parsing
func TestPathParsingBoundaryConditions(t *testing.T) {
	processor, _ := New()
	defer processor.Close()

	tests := []struct {
		name        string
		path        string
		expectError bool
		description string
	}{
		{
			name:        "very deep nesting",
			path:        "a.b.c.d.e.f.g.h.i.j.k.l.m.n.o.p",
			expectError: false,
			description: "Very deep path nesting",
		},
		{
			name:        "negative array index",
			path:        "items[-1]",
			expectError: false,
			description: "Negative array index",
		},
		{
			name:        "slice with step",
			path:        "items[0:10:2]",
			expectError: false,
			description: "Slice with step parameter",
		},
		{
			name:        "flat extraction",
			path:        "users{flat:items}",
			expectError: false,
			description: "Flat extraction syntax",
		},
		{
			name:        "consecutive extractions",
			path:        "data{users}{name}",
			expectError: false,
			description: "Consecutive extraction operations",
		},
		{
			name:        "complex nested slice",
			path:        "data.items[0:5].subarray[1:3]",
			expectError: false,
			description: "Multiple slice operations",
		},
		{
			name:        "mixed operations",
			path:        "users[0:5]{name}.first",
			expectError: false,
			description: "Mixed slice and extraction",
		},
		{
			name:        "special characters in property",
			path:        "user_profile.name",
			expectError: false,
			description: "Property with underscore",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := processor.parsePath(tt.path)
			if tt.expectError && err == nil {
				t.Errorf("Expected error for path '%s', but got none", tt.path)
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error for path '%s': %v", tt.path, err)
			}
		})
	}
}

// TestPathReconstruction tests reconstructing paths from segments
func TestPathReconstruction(t *testing.T) {
	processor, _ := New()
	defer processor.Close()

	// This tests the reconstructPath helper
	tests := []struct {
		name     string
		path     string
		contains string
	}{
		{
			name:     "simple path",
			path:     "user.name",
			contains: "user",
		},
		{
			name:     "with array",
			path:     "users[0].name",
			contains: "[0]",
		},
		{
			name:     "with extraction",
			path:     "users{name}",
			contains: "{name}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			segments := processor.getPathSegments()
			defer processor.putPathSegments(segments)

			var splitErr error
			*segments, splitErr = processor.splitPath(tt.path, *segments)
			if splitErr != nil {
				t.Fatalf("splitPath: %v", splitErr)
			}
			reconstructed := internal.ReconstructPath(*segments)

			if !strings.Contains(reconstructed, tt.contains) {
				t.Errorf("Reconstructed path '%s' does not contain '%s'", reconstructed, tt.contains)
			}
		})
	}
}

// TestPathSegmentTypes tests different path segment type identification
// TestPathSegmentation tests splitting paths into segments
func TestPathWithSpecialCharacters(t *testing.T) {
	jsonStr := `{
		"user_name": "value1",
		"user-name": "value2",
		"userName": "value3",
		"user name": "value4",
		"user@name": "value5"
	}`

	processor, _ := New()
	defer processor.Close()

	tests := []struct {
		name     string
		path     string
		expected string
	}{
		{
			name:     "underscore",
			path:     "user_name",
			expected: "value1",
		},
		{
			name:     "hyphen",
			path:     "user-name",
			expected: "value2",
		},
		{
			name:     "camelCase",
			path:     "userName",
			expected: "value3",
		},
		{
			name:     "JSON pointer with space",
			path:     "/user name",
			expected: "value4",
		},
		{
			name:     "JSON pointer with @",
			path:     "/user@name",
			expected: "value5",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := processor.Get(jsonStr, tt.path)
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			if result != tt.expected {
				t.Errorf("Get(%s) = %v; want %s", tt.path, result, tt.expected)
			}
		})
	}
}

// TestProcessBatch tests batch processing
func TestProcessBatch(t *testing.T) {
	operations := []BatchOperation{
		{Type: "get", Path: "user.name"},
		{Type: "get", Path: "user.age"},
		{Type: "set", Path: "user.email", Value: "alice@example.com"},
	}

	results, err := ProcessBatch(operations)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(results) != len(operations) {
		t.Errorf("Expected %d results, got %d", len(operations), len(results))
	}
}

func TestProcessor_BatchOperations(t *testing.T) {
	processor, _ := New()
	defer processor.Close()

	t.Run("SetMultiple", func(t *testing.T) {
		jsonStr := `{"a": 1}`
		updates := map[string]any{
			"b": 2,
			"c": 3,
		}
		result, err := processor.SetMultiple(jsonStr, updates)
		if err != nil {
			t.Errorf("SetMultiple error: %v", err)
		}

		val, _ := processor.Get(result, "b")
		if val != 2.0 {
			t.Errorf("SetMultiple failed, b = %v, want 2", val)
		}
	})

	t.Run("SetMultipleWithCreatePaths", func(t *testing.T) {
		jsonStr := `{}`
		updates := map[string]any{
			"a.b": 1,
			"c.d": 2,
		}
		cfg := DefaultConfig()
		cfg.CreatePaths = true
		result, err := processor.SetMultiple(jsonStr, updates, cfg)
		if err != nil {
			t.Errorf("SetMultiple error: %v", err)
		}

		val, _ := processor.Get(result, "a.b")
		if val != 1.0 {
			t.Errorf("SetMultiple failed, a.b = %v, want 1", val)
		}
	})

	t.Run("GetMultiple", func(t *testing.T) {
		jsonStr := `{"a": 1, "b": 2, "c": 3}`
		paths := []string{"a", "b"}
		result, err := processor.GetMultiple(jsonStr, paths)
		if err != nil {
			t.Errorf("GetMultiple error: %v", err)
		}
		if result["a"] != 1.0 {
			t.Errorf("result[a] = %v, want 1", result["a"])
		}
	})
}

func TestProcessor_CompiledPath(t *testing.T) {
	processor, _ := New()
	defer processor.Close()

	t.Run("CompilePath", func(t *testing.T) {
		cp, err := processor.CompilePath("user.name")
		if err != nil {
			t.Errorf("CompilePath error: %v", err)
		}
		if cp == nil {
			t.Fatal("CompilePath returned nil")
		}
	})

	t.Run("GetCompiled", func(t *testing.T) {
		cp, _ := processor.CompilePath("user.name")
		jsonStr := `{"user": {"name": "John"}}`
		result, err := processor.GetCompiled(jsonStr, cp)
		if err != nil {
			t.Errorf("GetCompiled error: %v", err)
		}
		if result != "John" {
			t.Errorf("GetCompiled = %v, want 'John'", result)
		}
	})
}

func TestProcessor_EncodeWithOptions(t *testing.T) {
	processor, _ := New()
	defer processor.Close()

	t.Run("EncodeWithOptions", func(t *testing.T) {
		data := map[string]any{"key": "value"}
		encOpts := DefaultConfig()
		encOpts.Pretty = true
		result, err := processor.EncodeWithConfig(data, encOpts)
		if err != nil {
			t.Fatalf("EncodeWithConfig error: %v", err)
		}
		if !strings.Contains(result, "value") {
			t.Errorf("result should contain the value, got %q", result)
		}
		if !strings.Contains(result, "\n") {
			t.Errorf("Pretty encoding should produce newlines, got %q", result)
		}
	})

	t.Run("EncodeBatch", func(t *testing.T) {
		pairs := map[string]any{"a": 1, "b": 2}
		cfg := DefaultConfig()
		cfg.Pretty = true
		result, err := processor.EncodeBatch(pairs, cfg)
		if err != nil {
			t.Fatalf("EncodeBatch error: %v", err)
		}
		if !strings.Contains(result, "\"a\"") || !strings.Contains(result, "\"b\"") {
			t.Errorf("result should contain both keys, got %q", result)
		}
	})

	t.Run("EncodeFields", func(t *testing.T) {
		data := struct {
			Name  string `json:"name"`
			Age   int    `json:"age"`
			Email string `json:"email"`
		}{
			Name:  "John",
			Age:   30,
			Email: "john@example.com",
		}
		fields := []string{"name", "age"}
		result, err := processor.EncodeFields(data, fields, DefaultConfig())
		if err != nil {
			t.Fatalf("EncodeFields error: %v", err)
		}
		if !strings.Contains(result, "John") {
			t.Errorf("result should contain selected field 'name', got %q", result)
		}
		// Only the selected fields should be present; email must be excluded.
		if strings.Contains(result, "john@example.com") {
			t.Errorf("result should exclude unselected field 'email', got %q", result)
		}
	})

	t.Run("EncodeStream", func(t *testing.T) {
		values := []any{1, 2, 3}
		result, err := processor.EncodeStream(values, DefaultConfig())
		if err != nil {
			t.Fatalf("EncodeStream error: %v", err)
		}
		for _, want := range []string{"1", "2", "3"} {
			if !strings.Contains(result, want) {
				t.Errorf("result should contain %s, got %q", want, result)
			}
		}
	})

	t.Run("EncodeStreamWithOptions", func(t *testing.T) {
		values := []any{1, 2, 3}
		encOpts := DefaultConfig()
		result, err := processor.EncodeStream(values, encOpts)
		if err != nil {
			t.Fatalf("EncodeStream error: %v", err)
		}
		if !strings.Contains(result, "2") {
			t.Errorf("result should contain streamed value 2, got %q", result)
		}
	})
}

func TestProcessor_ErrorHandling(t *testing.T) {
	processor, _ := New()
	defer processor.Close()

	t.Run("Get invalid path", func(t *testing.T) {
		jsonStr := `{"key": "value"}`
		_, err := processor.Get(jsonStr, "nonexistent.path")
		if err == nil {
			t.Error("Get should return error for invalid path")
		}
	})

	t.Run("Set on non-object", func(t *testing.T) {
		jsonStr := `{"key": "string_value"}`
		_, err := processor.Set(jsonStr, "key.nested", "value")
		if err == nil {
			t.Error("Set should return error when parent is not an object")
		}
	})

	t.Run("Parse invalid JSON", func(t *testing.T) {
		invalidJSON := `{invalid json}`
		_, err := processor.Get(invalidJSON, "key")
		if err == nil {
			t.Error("Get should return error for invalid JSON")
		}
	})

	t.Run("DeleteWithCleanupNulls", func(t *testing.T) {
		jsonStr := `{"a": {"b": {"c": 1}}}`
		cfg := DefaultConfig()
		cfg.CleanupNulls = true
		result, err := Delete(jsonStr, "a.b.c", cfg)
		if err != nil {
			t.Errorf("Delete with CleanupNulls error: %v", err)
		}
		// Verify the result is valid JSON
		if !json.Valid([]byte(result)) {
			t.Error("Delete with CleanupNulls should return valid JSON")
		}
	})
}

// TestProcessor_ForeachMethods tests Processor's Foreach methods
func TestProcessor_ForeachMethods(t *testing.T) {
	jsonStr := `{
		"users": [
			{"name": "Alice", "age": 25},
			{"name": "Bob", "age": 30},
			{"name": "Charlie", "age": 35}
		]
	}`

	t.Run("Foreach", func(t *testing.T) {
		processor, _ := New(DefaultConfig())
		defer processor.Close()

		count := 0
		processor.Foreach(jsonStr, func(key any, item *IterableValue) {
			count++
		})

		// Foreach on root should iterate over "users" key
		if count == 0 {
			t.Error("Foreach should have iterated over at least one item")
		}
	})

	t.Run("ForeachWithPath", func(t *testing.T) {
		processor, _ := New(DefaultConfig())
		defer processor.Close()

		count := 0
		names := []string{}

		err := processor.ForeachWithPath(jsonStr, "users", func(key any, item *IterableValue) {
			count++
			name := item.GetString("name")
			names = append(names, name)
		})

		if err != nil {
			t.Errorf("ForeachWithPath error: %v", err)
		}

		if count != 3 {
			t.Errorf("ForeachWithPath count = %d, want 3", count)
		}

		expectedNames := []string{"Alice", "Bob", "Charlie"}
		if len(names) != 3 {
			t.Errorf("Names count = %d, want 3", len(names))
		} else {
			for i, name := range names {
				if name != expectedNames[i] {
					t.Errorf("names[%d] = %q, want %q", i, name, expectedNames[i])
				}
			}
		}
	})

	t.Run("ForeachWithPathDeepNesting", func(t *testing.T) {
		// Create a processor with custom nesting depth limit
		config := DefaultConfig()
		config.MaxNestingDepthSecurity = 50

		processor, _ := New(config)
		defer processor.Close()

		// Deeply nested JSON structure with array at the end
		deepJSON := `{
			"level1": {
				"level2": {
					"level3": {
						"level4": {
							"items": [
								{"value": "deep1"},
								{"value": "deep2"}
							]
						}
					}
				}
			}
		}`

		found := false
		err := processor.ForeachWithPath(deepJSON, "level1.level2.level3.level4.items", func(key any, item *IterableValue) {
			value := item.GetString("value")
			if value == "deep1" || value == "deep2" {
				found = true
			}
		})

		if err != nil {
			t.Errorf("ForeachWithPath on deep structure error: %v", err)
		}

		if !found {
			t.Error("Expected to find deep values")
		}
	})

	t.Run("ForeachWithPathAndControl", func(t *testing.T) {
		processor, _ := New(DefaultConfig())
		defer processor.Close()

		count := 0
		err := processor.ForeachWithPathAndControl(jsonStr, "users", func(key any, value any) IteratorControl {
			count++
			// Continue iteration
			return IteratorContinue
		})

		if err != nil {
			t.Errorf("ForeachWithPathAndControl error: %v", err)
		}

		if count != 3 {
			t.Errorf("ForeachWithPathAndControl count = %d, want 3", count)
		}
	})

	t.Run("ForeachWithPathBreakEarly", func(t *testing.T) {
		processor, _ := New(DefaultConfig())
		defer processor.Close()

		count := 0
		err := processor.ForeachWithPathAndControl(jsonStr, "users", func(key any, value any) IteratorControl {
			count++
			// Break after first item
			if count == 1 {
				return IteratorBreak
			}
			return IteratorContinue
		})

		if err != nil {
			t.Errorf("ForeachWithPathAndControl error: %v", err)
		}

		if count != 1 {
			t.Errorf("ForeachWithPathAndControl with break count = %d, want 1", count)
		}
	})

	t.Run("ForeachWithPathAndIterator", func(t *testing.T) {
		processor, _ := New(DefaultConfig())
		defer processor.Close()

		paths := []string{}
		err := processor.ForeachWithPathAndIterator(jsonStr, "users", func(key any, item *IterableValue, currentPath string) IteratorControl {
			paths = append(paths, currentPath)
			return IteratorContinue
		})

		if err != nil {
			t.Errorf("ForeachWithPathAndIterator error: %v", err)
		}

		if len(paths) != 3 {
			t.Errorf("ForeachWithPathAndIterator paths count = %d, want 3", len(paths))
		}
	})
}

// TestProcessor_ForeachNested tests the ForeachNested method
func TestProcessor_ForeachNested(t *testing.T) {
	jsonStr := `{
		"user": {
			"name": "Alice",
			"age": 25,
			"address": {
				"city": "NYC"
			}
		}
	}`

	processor, _ := New(DefaultConfig())
	defer processor.Close()

	count := 0
	processor.ForeachNested(jsonStr, func(key any, item *IterableValue) {
		count++
	})

	// ForeachNested recursively iterates, so count should be > 5
	if count < 5 {
		t.Errorf("ForeachNested count = %d, want at least 5 (it's recursive)", count)
	}
}

// TestProcessor_ForeachReturn tests the ForeachReturn method
func TestProcessor_ForeachReturn(t *testing.T) {
	jsonStr := `{"items": [1, 2, 3]}`

	processor, _ := New(DefaultConfig())
	defer processor.Close()

	count := 0
	result, err := processor.ForeachReturn(jsonStr, func(key any, item *IterableValue) {
		count++
	})

	if err != nil {
		t.Errorf("ForeachReturn error: %v", err)
	}

	if count != 1 {
		t.Errorf("ForeachReturn count = %d, want 1 (just 'items' key)", count)
	}

	if result == "" {
		t.Error("ForeachReturn should return a non-empty JSON string")
	}

	// Result should be valid JSON representing the same data
	var parsedResult, parsedOriginal any
	if err := json.Unmarshal([]byte(result), &parsedResult); err != nil {
		t.Errorf("ForeachReturn result is not valid JSON: %v", err)
	}
	if err := json.Unmarshal([]byte(jsonStr), &parsedOriginal); err != nil {
		t.Fatalf("Test input is not valid JSON: %v", err)
	}
}

// TestProcessor_ForeachVsPackageLevel tests that Processor methods work independently
func TestProcessor_ForeachVsPackageLevel(t *testing.T) {
	jsonStr := `{"items": [1, 2, 3]}`

	t.Run("ProcessorMethod", func(t *testing.T) {
		processor, _ := New(DefaultConfig())
		defer processor.Close()

		count := 0
		err := processor.ForeachWithPath(jsonStr, "items", func(key any, item *IterableValue) {
			count++
		})

		if err != nil {
			t.Errorf("Processor.ForeachWithPath error: %v", err)
		}

		if count != 3 {
			t.Errorf("Processor.ForeachWithPath count = %d, want 3", count)
		}
	})

	t.Run("PackageLevelFunction", func(t *testing.T) {
		count := 0
		err := ForeachWithPath(jsonStr, "items", func(key any, item *IterableValue) {
			count++
		})

		if err != nil {
			t.Errorf("ForeachWithPath error: %v", err)
		}

		if count != 3 {
			t.Errorf("ForeachWithPath count = %d, want 3", count)
		}
	})
}

func TestProcessor_Iterators(t *testing.T) {
	processor, _ := New()
	defer processor.Close()

	t.Run("Foreach", func(t *testing.T) {
		jsonStr := `{"a": 1, "b": 2}`
		count := 0
		processor.Foreach(jsonStr, func(key any, item *IterableValue) {
			count++
		})
		if count != 2 {
			t.Errorf("Foreach visited %d items, want 2", count)
		}
	})

	t.Run("ForeachWithPath", func(t *testing.T) {
		jsonStr := `{"items": [1, 2, 3]}`
		count := 0
		err := processor.ForeachWithPath(jsonStr, "items[*]", func(key any, item *IterableValue) {
			count++
		})
		if err != nil {
			t.Errorf("ForeachWithPath error: %v", err)
		}
		if count != 3 {
			t.Errorf("ForeachWithPath visited %d items, want 3", count)
		}
	})

	t.Run("ForeachReturn", func(t *testing.T) {
		jsonStr := `{"a": 1, "b": 2}`
		result, err := processor.ForeachReturn(jsonStr, func(key any, item *IterableValue) {
			// Just iterate
		})
		if err != nil {
			t.Errorf("ForeachReturn error: %v", err)
		}
		if result == "" {
			t.Error("ForeachReturn should return result")
		}
	})

	t.Run("ForeachNested", func(t *testing.T) {
		jsonStr := `{"a": 1, "b": {"c": 2}}`
		count := 0
		processor.ForeachNested(jsonStr, func(key any, item *IterableValue) {
			count++
		})
		if count < 2 {
			t.Errorf("ForeachNested visited %d items, expected at least 2", count)
		}
	})
}

func TestProcessor_ParseAndValidate(t *testing.T) {
	processor, _ := New()
	defer processor.Close()

	t.Run("Parse", func(t *testing.T) {
		jsonStr := `{"key": "value"}`
		var result map[string]any
		err := processor.Parse(jsonStr, &result)
		if err != nil {
			t.Errorf("Parse error: %v", err)
		}
		if result["key"] != "value" {
			t.Errorf("result[key] = %v, want 'value'", result["key"])
		}
	})

	t.Run("Valid", func(t *testing.T) {
		jsonStr := `{"key": "value"}`
		valid, err := processor.Valid(jsonStr)
		if err != nil {
			t.Errorf("Valid error: %v", err)
		}
		if !valid {
			t.Error("Valid should return true for valid JSON")
		}
	})

	t.Run("ValidBytes", func(t *testing.T) {
		data := []byte(`{"key": "value"}`)
		if !processor.ValidBytes(data) {
			t.Error("ValidBytes should return true for valid JSON")
		}
	})
}

func TestProcessor_PreParse(t *testing.T) {
	processor, _ := New()
	defer processor.Close()

	t.Run("PreParse", func(t *testing.T) {
		jsonStr := `{"key": "value"}`
		parsed, err := processor.PreParse(jsonStr)
		if err != nil {
			t.Errorf("PreParse error: %v", err)
		}
		if parsed == nil {
			t.Fatal("PreParse returned nil")
		}
	})

	t.Run("GetFromParsed", func(t *testing.T) {
		jsonStr := `{"key": "value", "nested": {"a": 1}}`
		parsed, _ := processor.PreParse(jsonStr)
		result, err := processor.GetFromParsed(parsed, "nested.a")
		if err != nil {
			t.Errorf("GetFromParsed error: %v", err)
		}
		if result != 1.0 {
			t.Errorf("GetFromParsed = %v, want 1", result)
		}
	})

	t.Run("SetFromParsed", func(t *testing.T) {
		jsonStr := `{"key": "value"}`
		parsed, _ := processor.PreParse(jsonStr)
		newParsed, err := processor.SetFromParsed(parsed, "key", "new_value")
		if err != nil {
			t.Errorf("SetFromParsed error: %v", err)
		}
		if newParsed == nil {
			t.Fatal("SetFromParsed returned nil")
		}
	})
}

func TestProcessor_SafeGet(t *testing.T) {
	processor, _ := New()
	defer processor.Close()

	t.Run("SafeGet found", func(t *testing.T) {
		jsonStr := `{"key": "value"}`
		result := processor.SafeGet(jsonStr, "key")
		if !result.Exists {
			t.Error("SafeGet should find key")
		}
		if result.Value != "value" {
			t.Errorf("SafeGet = %v, want 'value'", result.Value)
		}
	})

	t.Run("SafeGet not found", func(t *testing.T) {
		jsonStr := `{"key": "value"}`
		result := processor.SafeGet(jsonStr, "nonexistent")
		if result.Exists {
			t.Error("SafeGet should not find nonexistent key")
		}
	})
}

func TestProcessor_State(t *testing.T) {
	t.Run("GetConfig", func(t *testing.T) {
		processor, _ := New()
		defer processor.Close()

		cfg := processor.GetConfig()
		// Check that config has valid values
		if cfg.MaxJSONSize == 0 {
			t.Fatal("GetConfig returned zero config")
		}
	})

	t.Run("IsClosed not closed", func(t *testing.T) {
		processor, _ := New()
		if processor.IsClosed() {
			t.Error("Processor should not be closed")
		}
		processor.Close()
	})

	t.Run("IsClosed closed", func(t *testing.T) {
		processor, _ := New()
		processor.Close()
		if !processor.IsClosed() {
			t.Error("Processor should be closed")
		}
	})
}

func TestProcessor_StatsAndHealth(t *testing.T) {
	processor, _ := New()
	defer processor.Close()

	// Drive at least one operation so the operation counter is non-zero.
	if _, err := processor.Get(`{"k":"v"}`, "k"); err != nil {
		t.Fatalf("setup Get failed: %v", err)
	}

	t.Run("GetStats", func(t *testing.T) {
		stats := processor.GetStats()
		if stats.OperationCount < 1 {
			t.Errorf("OperationCount = %d, want >= 1 after a Get", stats.OperationCount)
		}
		if stats.IsClosed {
			t.Error("IsClosed = true, want false on an open processor")
		}
	})

	t.Run("GetHealthStatus", func(t *testing.T) {
		health := processor.GetHealthStatus()
		if len(health.Checks) == 0 {
			t.Fatal("expected populated health checks, got none")
		}
		// The aggregate Healthy flag depends on metrics-collector warmup and may
		// be false on a fresh processor; verify the pipeline returns structured
		// per-check results with at least one healthy dimension.
		var anyHealthy bool
		for _, c := range health.Checks {
			if c.Healthy {
				anyHealthy = true
				break
			}
		}
		if !anyHealthy {
			t.Errorf("expected at least one healthy check, got %v", health.Checks)
		}
	})

	t.Run("ClearCache", func(t *testing.T) {
		// ClearCache must not panic and must leave the processor usable.
		processor.ClearCache()
		if _, err := processor.Get(`{"k":"v"}`, "k"); err != nil {
			t.Errorf("Get after ClearCache failed: %v", err)
		}
	})
}

func TestProcessor_WildcardAndExtraction(t *testing.T) {
	processor, _ := New()
	defer processor.Close()

	t.Run("Get with wildcard", func(t *testing.T) {
		jsonStr := `{"users": [{"name": "John"}, {"name": "Jane"}]}`
		result, err := processor.Get(jsonStr, "users[*].name")
		if err != nil {
			t.Errorf("Get error: %v", err)
		}
		arr, ok := result.([]any)
		if !ok {
			t.Fatalf("Result is not an array")
		}
		if len(arr) != 2 {
			t.Errorf("Wildcard result length = %d, want 2", len(arr))
		}
	})

	t.Run("Get with extraction", func(t *testing.T) {
		jsonStr := `{"users": [{"name": "John", "email": "john@example.com"}, {"name": "Jane", "email": "jane@example.com"}]}`
		result, err := processor.Get(jsonStr, "users{name}")
		if err != nil {
			t.Errorf("Get error: %v", err)
		}
		// Verify extraction result
		arr, ok := result.([]any)
		if !ok {
			t.Fatalf("Expected array, got %T", result)
		}
		if len(arr) != 2 {
			t.Errorf("Expected 2 extracted names, got %d", len(arr))
		}
	})
}

// TestReverseSlice tests reverse slicing logic
func TestReverseSlice(t *testing.T) {
	processor, _ := New()
	defer processor.Close()

	arr := []any{0.0, 1.0, 2.0, 3.0, 4.0}

	tests := []struct {
		name     string
		start    int
		end      int
		step     int
		expected []any
	}{
		{
			name:     "reverse slice",
			start:    4,
			end:      0,
			step:     -1,
			expected: []any{4.0, 3.0, 2.0, 1.0},
		},
		{
			name:     "reverse with step (end=-1 normalizes to last index, so empty)",
			start:    4,
			end:      -1,
			step:     -2,
			expected: []any{},
		},
		{
			name:     "reverse to before beginning (end=-6 normalizes to -1, before index 0)",
			start:    4,
			end:      -6,
			step:     -2,
			expected: []any{4.0, 2.0, 0.0},
		},
		{
			name:     "start beyond length",
			start:    10,
			end:      0,
			step:     -1,
			expected: []any{4.0, 3.0, 2.0, 1.0},
		},
		{
			name:     "empty result",
			start:    0,
			end:      4,
			step:     -1,
			expected: []any{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := internal.PerformArraySlice(arr, &tt.start, &tt.end, &tt.step)
			if !slicesEqual(result, tt.expected) {
				t.Errorf("reverseSlice(%d, %d, %d) = %v; want %v", tt.start, tt.end, tt.step, result, tt.expected)
			}
		})
	}
}

// TestSetGlobalProcessor tests SetGlobalProcessor function
func TestSetGlobalProcessor(t *testing.T) {
	// Reset state
	ShutdownGlobalProcessor()

	t.Run("SetCustomProcessor", func(t *testing.T) {
		customConfig := DefaultConfig()
		customConfig.MaxCacheSize = 500
		customProcessor, _ := New(customConfig)

		SetGlobalProcessor(customProcessor)

		p := getDefaultProcessor()
		if p != customProcessor {
			t.Error("getDefaultProcessor should return the custom processor")
		}
	})

	t.Run("SetNilProcessorDoesNothing", func(t *testing.T) {
		originalProcessor := getDefaultProcessor()

		SetGlobalProcessor(nil)

		p := getDefaultProcessor()
		if p != originalProcessor {
			t.Error("Setting nil should not change the global processor")
		}
	})

	// Cleanup
	ShutdownGlobalProcessor()
}

// TestSetMultiple tests setting multiple values at once
func TestSetMultiple(t *testing.T) {
	jsonStr := `{"user": {"name": "Alice", "age": 30}}`

	tests := []struct {
		name        string
		updates     map[string]any
		expectError bool
		validate    func(t *testing.T, result string)
	}{
		{
			name: "multiple updates",
			updates: map[string]any{
				"user.name":  "Bob",
				"user.age":   35,
				"user.email": "bob@example.com",
			},
			expectError: false,
			validate: func(t *testing.T, result string) {
				if !strings.Contains(result, "Bob") {
					t.Error("Expected name to be updated to Bob")
				}
			},
		},
		{
			name:        "empty updates",
			updates:     map[string]any{},
			expectError: false,
			validate: func(t *testing.T, result string) {
				if !strings.Contains(result, "Alice") {
					t.Error("Expected original data to remain")
				}
			},
		},
		{
			name: "single update",
			updates: map[string]any{
				"user.name": "Charlie",
			},
			expectError: false,
			validate: func(t *testing.T, result string) {
				if !strings.Contains(result, "Charlie") {
					t.Error("Expected name to be updated to Charlie")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := SetMultiple(jsonStr, tt.updates)
			if tt.expectError && err == nil {
				t.Errorf("Expected error, but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
			if tt.validate != nil {
				tt.validate(t, result)
			}
		})
	}
}

// TestShutdownGlobalProcessor tests ShutdownGlobalProcessor function
func TestShutdownGlobalProcessor(t *testing.T) {
	t.Run("ShutdownClosesProcessor", func(t *testing.T) {
		p := getDefaultProcessor()

		ShutdownGlobalProcessor()

		// Give it time to close
		time.Sleep(10 * time.Millisecond)

		if !p.IsClosed() {
			t.Error("Processor should be closed after shutdown")
		}
	})

	t.Run("MultipleShutdownsAreSafe", func(t *testing.T) {
		// Create a new processor
		_ = getDefaultProcessor()

		// Multiple shutdowns should not panic
		ShutdownGlobalProcessor()
		ShutdownGlobalProcessor()
		ShutdownGlobalProcessor()
	})

	t.Run("GetAfterShutdownCreatesNewProcessor", func(t *testing.T) {
		ShutdownGlobalProcessor()

		p1 := getDefaultProcessor()
		ShutdownGlobalProcessor()
		p2 := getDefaultProcessor()

		if p1 == p2 {
			t.Error("New processor should be created after shutdown")
		}

		// Cleanup
		ShutdownGlobalProcessor()
	})
}

// TestStandardLibraryCompatibility tests encoding/json compatibility
func TestStandardLibraryCompatibility(t *testing.T) {
	// Test Marshal
	data := map[string]any{"name": "Alice", "age": 30}
	jsonBytes, err := Marshal(data)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}
	if len(jsonBytes) == 0 {
		t.Error("Expected non-empty JSON output")
	}

	// Test Unmarshal
	var result map[string]any
	err = Unmarshal(jsonBytes, &result)
	if err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
	if result["name"] != "Alice" {
		t.Errorf("Expected name 'Alice', got '%v'", result["name"])
	}

	// Test MarshalIndent
	indented, err := MarshalIndent(data, "", "  ")
	if err != nil {
		t.Fatalf("MarshalIndent failed: %v", err)
	}
	if !strings.Contains(string(indented), "\n") {
		t.Error("Expected indented output to contain newlines")
	}

	// Test Valid
	if !Valid(jsonBytes) {
		t.Error("Valid() returned false for valid JSON")
	}
	invalidJSON := []byte("{invalid}")
	if Valid(invalidJSON) {
		t.Error("Valid() returned true for invalid JSON")
	}
}

// TestWarmupCache tests cache warmup functionality
func TestWarmupCache(t *testing.T) {
	jsonStr := `{
		"users": [
			{"name": "Alice", "age": 30},
			{"name": "Bob", "age": 25}
		],
		"settings": {
			"theme": "dark",
			"language": "en"
		}
	}`

	paths := []string{
		"users[0].name",
		"users[1].name",
		"settings.theme",
	}

	result, err := WarmupCache(jsonStr, paths)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if result == nil {
		t.Error("Expected warmup result, got nil")
	}
}

// TestMultiFieldPathParsing tests parsing of multi-field extraction paths
func TestMultiFieldPathParsing(t *testing.T) {
	processor, _ := New()
	defer processor.Close()

	tests := []struct {
		name       string
		path       string
		expectLen  int
		expectType string
		expectKey  string
	}{
		{
			name:       "single_field_extract",
			path:       "user{id}",
			expectLen:  2,
			expectType: "extract",
			expectKey:  "id",
		},
		{
			name:       "multi_field_extract",
			path:       "user{id,name}",
			expectLen:  2,
			expectType: "extract",
			expectKey:  "id,name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			segments := processor.getPathSegments()
			defer processor.putPathSegments(segments)

			var splitErr error
			*segments, splitErr = processor.splitPath(tt.path, *segments)
			if splitErr != nil {
				t.Fatalf("splitPath: %v", splitErr)
			}

			if len(*segments) != tt.expectLen {
				t.Errorf("Expected %d segments, got %d", tt.expectLen, len(*segments))
				for i, s := range *segments {
					t.Logf("  Segment %d: Type=%v Key=%q", i, s.Type, s.Key)
				}
				return
			}

			lastSeg := (*segments)[len(*segments)-1]
			if lastSeg.TypeString() != tt.expectType {
				t.Errorf("Expected type %s, got %s", tt.expectType, lastSeg.TypeString())
			}
			if lastSeg.Key != tt.expectKey {
				t.Errorf("Expected key %q, got %q", tt.expectKey, lastSeg.Key)
			}
		})
	}
}

// TestMultiFieldExtractionDebug drives handleExtraction directly with a
// pre-parsed map and an extract segment. This map-input path reaches
// array-extraction branches in operation_array.go that the Get("...{...}")
// JSON-string path does not exercise, so it is retained for coverage.
func TestMultiFieldExtractionDebug(t *testing.T) {
	processor, _ := New()
	defer processor.Close()

	data := map[string]any{
		"id":   float64(1001),
		"name": "Alice",
	}

	segment := internal.PathSegment{
		Type: internal.ExtractSegment,
		Key:  "id,name",
	}

	result, err := processor.handleExtraction(data, segment)
	if err != nil {
		t.Fatalf("handleExtraction error: %v", err)
	}

	if result == nil {
		t.Error("Expected non-nil result")
	}
}

// TestPathEscapeIntegration tests the escape functionality with real JSON data
func TestPathEscapeIntegration(t *testing.T) {
	processor, _ := New()
	defer processor.Close()

	t.Run("escaped dot in property name", func(t *testing.T) {
		// JSON with a property name containing a literal dot
		jsonStr := `{"user.name": "Alice"}`

		// Access using escaped dot notation (backslash-dot)
		result := GetString(jsonStr, `user\.name`, "")
		if result != "Alice" {
			t.Errorf("GetString(%q) = %q, want Alice", `user\.name`, result)
		}
	})

	t.Run("multiple escaped dots", func(t *testing.T) {
		jsonStr := `{"a.b.c": "nested"}`

		// Access with multiple escaped dots
		result := GetString(jsonStr, `a\.b\.c`, "")
		if result != "nested" {
			t.Errorf("GetString(%q) = %q, want nested", `a\.b\.c`, result)
		}
	})

	t.Run("mixed escaped and normal", func(t *testing.T) {
		jsonStr := `{"user.name": {"first": "John", "last": "Doe"}}`

		// Access with escaped dot then normal property
		result := GetString(jsonStr, `user\.name.first`, "")
		if result != "John" {
			t.Errorf("GetString(%q) = %q, want John", `user\.name.first`, result)
		}
	})

	t.Run("set with escaped property", func(t *testing.T) {
		jsonStr := `{"user.name": "Bob"}`

		// Set value with escaped property
		result, err := Set(jsonStr, `user\.name`, "Alice")
		if err != nil {
			t.Fatalf("Set error: %v", err)
		}

		// Verify the set worked
		name := GetString(result, `user\.name`, "")
		if name != "Alice" {
			t.Errorf("Set did not update value correctly, got %q, want Alice", name)
		}
	})

	t.Run("JSON Pointer alternative", func(t *testing.T) {
		// Same test using JSON Pointer format (alternative approach)
		jsonStr := `{"user.name": "Alice"}`

		// Access using JSON Pointer format
		result := GetString(jsonStr, "/user.name", "")
		if result != "Alice" {
			t.Errorf("GetString(%q) = %q, want Alice", "/user.name", result)
		}
	})
}

// ============================================================================
// BARE-NUMERIC INDEX TESTS (merged from bare_numeric_test.go)
// ============================================================================

// TestBareIndexMatchesBracket is the core invariant of the bare-index feature:
// for every supported array path, the bare form ("0", "-1", "*", "*.prop")
// returns the same result as the bracket form ("[0]", "[-1]", "[*]", "[*].prop").
func TestBareIndexMatchesBracket(t *testing.T) {
	intArr := `[10,20,30]`
	objArr := `[{"name_cn":"万国数据","symbol":"GDS.US"},{"name_cn":"极氪","symbol":"ZK.US"}]`

	cases := []struct {
		name string
		json string
		bare string
		brkt string
	}{
		{"int 0", intArr, "0", "[0]"},
		{"int -1", intArr, "-1", "[-1]"},
		{"int *", intArr, "*", "[*]"},
		{"object 0", objArr, "0", "[0]"},
		{"object 0.symbol", objArr, "0.symbol", "[0].symbol"},
		{"object -1.symbol", objArr, "-1.symbol", "[-1].symbol"},
		{"object *.symbol", objArr, "*.symbol", "[*].symbol"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			bare, errB := Get(c.json, c.bare)
			brkt, errK := Get(c.json, c.brkt)
			if (errB != nil) != (errK != nil) {
				t.Errorf("errors differ: bare %q err=%v, bracket %q err=%v", c.bare, errB, c.brkt, errK)
			}
			if !reflect.DeepEqual(bare, brkt) {
				t.Errorf("bare %q = %#v, bracket %q = %#v", c.bare, bare, c.brkt, brkt)
			}
		})
	}
}

// TestBareIndexObjectKeyCompat verifies backward compatibility for objects
// that use numeric-string keys (user chose "array-first, object numeric-key
// compat"): the value is reachable via the bare numeric path AND via "[N]".
func TestBareIndexObjectKeyCompat(t *testing.T) {
	cases := []struct {
		name string
		json string
		path string
		want any
	}{
		{`bare "0" on {"0":"x"}`, `{"0":"x"}`, "0", "x"},
		{`bracket "[0]" on {"0":"x"}`, `{"0":"x"}`, "[0]", "x"},
		{`recurse "0.a"`, `{"0":{"a":1}}`, "0.a", float64(1)},
		{`bare "-1" on {"-1":"neg"}`, `{"-1":"neg"}`, "-1", "neg"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := Get(c.json, c.path)
			if err != nil {
				t.Fatalf("Get(%q) error: %v", c.path, err)
			}
			if !reflect.DeepEqual(got, c.want) {
				t.Errorf("Get(%q) = %#v, want %#v", c.path, got, c.want)
			}
		})
	}
}

// TestBareIndexSetGuard verifies Set with a bare numeric path is unchanged.
func TestBareIndexSetGuard(t *testing.T) {
	out, err := Set(`[10,20,30]`, "0", 99)
	if err != nil {
		t.Fatalf("Set error: %v", err)
	}
	if out != `[99,20,30]` {
		t.Errorf(`Set([10,20,30], "0", 99) = %s, want [99,20,30]`, out)
	}
}

// TestBareIndexUserScenario is the exact case from the bug report: getting an
// array element via a bare numeric index.
func TestBareIndexUserScenario(t *testing.T) {
	jsonStr := `[` +
		`{"name_cn":"万国数据","name_en":"GDS Holdings Limited","name_hk":"万国数据","symbol":"GDS.US"},` +
		`{"name_cn":"极氪","name_en":"ZEEKR Intelligent Technology Holding Limited","name_hk":"極氪","symbol":"ZK.US"}` +
		`]`

	if got := GetString(jsonStr, "0.symbol"); got != "GDS.US" {
		t.Errorf(`GetString(_, "0.symbol") = %q, want "GDS.US"`, got)
	}
	if got := GetString(jsonStr, "1.symbol"); got != "ZK.US" {
		t.Errorf(`GetString(_, "1.symbol") = %q, want "ZK.US"`, got)
	}
	if got := GetArray(jsonStr, "*.symbol"); !reflect.DeepEqual(got, []any{"GDS.US", "ZK.US"}) {
		t.Errorf(`GetArray(_, "*.symbol") = %#v, want ["GDS.US","ZK.US"]`, got)
	}
}

// ============================================================================
// SHARED-RESULT CACHE TESTS (merged from shared_cache_test.go)
// ============================================================================

// TestGet_DefaultCacheCopiesOnHit verifies the default (safe) behavior: two
// cache-hit Gets return independent deep copies, so mutating one cannot affect
// the other. This guards against regressing the default safety guarantee.
func TestGet_DefaultCacheCopiesOnHit(t *testing.T) {
	p, _ := New()
	defer p.Close()

	jsonStr := `{"items":[1,2,3]}`

	// Prime the result cache (first call is a cache miss).
	_, _ = p.Get(jsonStr, "items")

	// Two cache-hit Gets must each deep-copy the cached slice.
	a, err := p.Get(jsonStr, "items")
	if err != nil {
		t.Fatalf("Get #1: %v", err)
	}
	b, err := p.Get(jsonStr, "items")
	if err != nil {
		t.Fatalf("Get #2: %v", err)
	}

	aArr, aOK := a.([]any)
	bArr, bOK := b.([]any)
	if !aOK || !bOK {
		t.Fatalf("expected []any, got %T and %T", a, b)
	}
	if reflect.ValueOf(aArr).Pointer() == reflect.ValueOf(bArr).Pointer() {
		t.Error("default cache: two cache-hit Gets share a backing array; expected independent deep copies")
	}
}

// TestGet_SharedCacheReturnsSameReference verifies that with
// CacheSharedResults enabled, cache-hit Gets skip the defensive deep copy and
// return the shared cached value directly (pointer identity).
func TestGet_SharedCacheReturnsSameReference(t *testing.T) {
	cfg := DefaultConfig()
	cfg.CacheSharedResults = true
	p, _ := New(cfg)
	defer p.Close()

	jsonStr := `{"items":[1,2,3]}`

	// Prime the result cache.
	_, _ = p.Get(jsonStr, "items")

	a, err := p.Get(jsonStr, "items")
	if err != nil {
		t.Fatalf("Get #1: %v", err)
	}
	b, err := p.Get(jsonStr, "items")
	if err != nil {
		t.Fatalf("Get #2: %v", err)
	}

	aArr, aOK := a.([]any)
	bArr, bOK := b.([]any)
	if !aOK || !bOK {
		t.Fatalf("expected []any, got %T and %T", a, b)
	}
	if reflect.ValueOf(aArr).Pointer() != reflect.ValueOf(bArr).Pointer() {
		t.Error("CacheSharedResults: two cache-hit Gets returned distinct backing arrays; expected a shared reference")
	}
}

// TestGet_SharedCacheStillReturnsCorrectValue ensures the opt-in fast path
// returns the right value, not just any reference.
func TestGet_SharedCacheStillReturnsCorrectValue(t *testing.T) {
	cfg := DefaultConfig()
	cfg.CacheSharedResults = true
	p, _ := New(cfg)
	defer p.Close()

	jsonStr := `{"user":{"name":"Alice","roles":["admin","user"]}}`

	roles, err := p.Get(jsonStr, "user.roles")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	arr := roles.([]any)
	if len(arr) != 2 || arr[0] != "admin" || arr[1] != "user" {
		t.Errorf("unexpected roles: %v", arr)
	}

	name, err := p.Get(jsonStr, "user.name")
	if err != nil {
		t.Fatalf("Get name: %v", err)
	}
	if name != "Alice" {
		t.Errorf("unexpected name: %v", name)
	}
}

// TestGetFromParsed_SharedCacheSkipsCopy mirrors the Get behavior for the
// pre-parsed read path.
func TestGetFromParsed_SharedCacheSkipsCopy(t *testing.T) {
	cfg := DefaultConfig()
	cfg.CacheSharedResults = true
	p, _ := New(cfg)
	defer p.Close()

	jsonStr := `{"items":[1,2,3]}`
	parsed, err := p.PreParse(jsonStr)
	if err != nil {
		t.Fatalf("PreParse: %v", err)
	}
	defer parsed.Release()

	a, err := p.GetFromParsed(parsed, "items")
	if err != nil {
		t.Fatalf("GetFromParsed #1: %v", err)
	}
	b, err := p.GetFromParsed(parsed, "items")
	if err != nil {
		t.Fatalf("GetFromParsed #2: %v", err)
	}
	aArr := a.([]any)
	bArr := b.([]any)
	if reflect.ValueOf(aArr).Pointer() != reflect.ValueOf(bArr).Pointer() {
		t.Error("CacheSharedResults: GetFromParsed returned distinct backing arrays; expected a shared reference")
	}
}

// TestCacheSharedResults_InConfigFieldRegistry guards against the
// maintainability hazard of forgetting to register a new Config field in
// configFieldList: two configs differing only in CacheSharedResults must
// compare unequal and hash differently, so their cache keys never collide.
func TestCacheSharedResults_InConfigFieldRegistry(t *testing.T) {
	a := DefaultConfig()
	b := DefaultConfig()
	b.CacheSharedResults = true

	if configFieldsEqual(a, b) {
		t.Error("configFieldsEqual must distinguish CacheSharedResults (field missing from configFieldList?)")
	}
	if hashConfig(a) == hashConfig(b) {
		t.Error("hashConfig must differ for CacheSharedResults (field missing from configFieldList?)")
	}

	// isDefaultConfig must reject the shared-results config as non-default.
	if isDefaultConfig(b) {
		t.Error("isDefaultConfig must return false when CacheSharedResults is true")
	}
}

// ============================================================================
// PATH BOUNDARY TESTS (merged from path_boundary_test.go)
// ============================================================================

// TestPreservingUnmarshal_Boundary exercises preservingUnmarshal (path.go)
// across target types: any, map, slice, struct, and invalid JSON.
func TestPreservingUnmarshal_Boundary(t *testing.T) {
	t.Run("non_preserve_into_any", func(t *testing.T) {
		var v any
		if err := preservingUnmarshal([]byte(`{"a":1}`), &v, false, false); err != nil {
			t.Fatalf("err: %v", err)
		}
	})
	t.Run("preserve_into_any", func(t *testing.T) {
		var v any
		if err := preservingUnmarshal([]byte(`{"a":42}`), &v, true, false); err != nil {
			t.Fatalf("err: %v", err)
		}
	})
	t.Run("preserve_into_map", func(t *testing.T) {
		var m map[string]any
		if err := preservingUnmarshal([]byte(`{"a":42}`), &m, true, false); err != nil {
			t.Fatalf("err: %v", err)
		}
	})
	t.Run("preserve_into_slice", func(t *testing.T) {
		var s []any
		if err := preservingUnmarshal([]byte(`[1,2,3]`), &s, true, false); err != nil {
			t.Fatalf("err: %v", err)
		}
	})
	t.Run("preserve_into_struct", func(t *testing.T) {
		type S struct {
			A int `json:"a"`
		}
		var s S
		if err := preservingUnmarshal([]byte(`{"a":42}`), &s, true, false); err != nil {
			t.Fatalf("err: %v", err)
		}
		if s.A != 42 {
			t.Errorf("A = %d, want 42", s.A)
		}
	})
	t.Run("invalid_json", func(t *testing.T) {
		var v any
		if err := preservingUnmarshal([]byte("not json"), &v, true, false); err == nil {
			t.Error("expected error for invalid JSON")
		}
	})
}

// ============================================================================
// PROCESSOR CLOSED-STATE BOUNDARY (merged from processor_boundary_test.go)
// ============================================================================

// TestProcessor_ClosedState_Boundary verifies the closed-processor error guards
// on the iterator/JSONL methods (checkClosed branches) — coverage that
// coverage_test.go's TestProcessorClosedState (Get/Set/Delete/Marshal only) lacks.
func TestProcessor_ClosedState_Boundary(t *testing.T) {
	p, _ := New()
	p.Close()

	noop := func(key any, item *IterableValue) {}
	noopLine := func(lineNum int, item *IterableValue) error { return nil }
	reduceFn := func(acc any, item *IterableValue) any { return acc }

	if err := p.ForeachWithPath(`{"a":1}`, "a", noop); err == nil {
		t.Error("expected error from ForeachWithPath on closed processor")
	}
	if _, err := p.ForeachReturn(`{"a":1}`, noop); err == nil {
		t.Error("expected error from ForeachReturn on closed processor")
	}
	if _, err := p.CompilePath("a"); err == nil {
		t.Error("expected error from CompilePath on closed processor")
	}
	if _, err := p.GetCompiled(`{"a":1}`, nil); err == nil {
		t.Error("expected error from GetCompiled on closed processor")
	}
	if err := p.ForeachJSONL(strings.NewReader(`{"a":1}`), noopLine); err == nil {
		t.Error("expected error from ForeachJSONL on closed processor")
	}
	if _, err := p.ReduceJSONL(strings.NewReader(`{"a":1}`), nil, reduceFn); err == nil {
		t.Error("expected error from ReduceJSONL on closed processor")
	}
	if _, err := p.CollectJSONL(strings.NewReader(`{"a":1}`)); err == nil {
		t.Error("expected error from CollectJSONL on closed processor")
	}
}
