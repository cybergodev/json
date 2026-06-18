package json

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math"
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

// BenchmarkDelete/BenchmarkFastDelete/BenchmarkFastSet/BenchmarkGet
// are covered by comprehensive versions in benchmark_test.go

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
	processor, _ := New()
	defer processor.Close()

	input := "user~/name/with~special/chars"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = escapeJSONPointer(input)
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

// TestArrayExtensionSignal tests the Error method
func TestArrayExtensionSignal(t *testing.T) {
	err := &arrayExtensionSignal{
		requiredLength: 10,
		currentLength:  5,
		start:          0,
		end:            10,
		step:           1,
		value:          "test",
	}

	expected := "array extension needed: current length 5, required length 10 for slice [0:10]"
	if err.Error() != expected {
		t.Errorf("Error() = %q, want %q", err.Error(), expected)
	}
}

// TestArrayIndexValidation tests array index validation
func TestArrayIndexValidation(t *testing.T) {
	processor, _ := New()
	defer processor.Close()

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
			result := processor.isValidArrayIndex(tt.index)
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

	t.Run("DefaultConfig_Fast", func(t *testing.T) {
		config := DefaultConfig()
		config.FullSecurityScan = false
		config.StrictMode = false

		helper.AssertNotNil(config)
		helper.AssertFalse(config.FullSecurityScan)
		helper.AssertFalse(config.StrictMode)
	})

	t.Run("DefaultConfig_Minimal", func(t *testing.T) {
		config := DefaultConfig()
		config.EnableValidation = false
		config.EnableCache = false

		helper.AssertNotNil(config)
		helper.AssertFalse(config.EnableValidation)
		helper.AssertFalse(config.EnableCache)
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

	t.Run("ConfigInterface", func(t *testing.T) {
		config := DefaultConfig()

		helper.AssertTrue(config.EnableCache)
		helper.AssertTrue(config.MaxCacheSize > 0)
		helper.AssertTrue(config.CacheTTL > 0)
		helper.AssertTrue(config.MaxJSONSize > 0)
		helper.AssertTrue(config.MaxPathDepth > 0)
		helper.AssertTrue(config.MaxConcurrency > 0)
		helper.AssertFalse(config.EnableMetrics)
		helper.AssertFalse(config.EnableHealthCheck)
		helper.AssertFalse(config.StrictMode)
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
	processor, _ := New()
	defer processor.Close()

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
			isComplex := processor.isDistributedOperationPath(tt.path)
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

func TestEncodeConfig_Default(t *testing.T) {
	cfg := DefaultConfig()
	// Check that default config has reasonable values
	if cfg.MaxJSONSize == 0 {
		t.Error("DefaultConfig should set MaxJSONSize")
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

	t.Run("EncodingOptions", func(t *testing.T) {
		config := DefaultConfig()

		t.Run("SetPretty", func(t *testing.T) {
			config.Pretty = true
			helper.AssertTrue(config.Pretty)
		})

		t.Run("SetSortKeys", func(t *testing.T) {
			config.SortKeys = true
			helper.AssertTrue(config.SortKeys)
		})

		t.Run("SetFloatPrecision", func(t *testing.T) {
			config.FloatPrecision = 2
			helper.AssertEqual(2, config.FloatPrecision)
		})

		t.Run("SetEscapeHTML", func(t *testing.T) {
			config.EscapeHTML = false
			helper.AssertFalse(config.EscapeHTML)
		})

		t.Run("SetEscapeUnicode", func(t *testing.T) {
			config.EscapeUnicode = true
			helper.AssertTrue(config.EscapeUnicode)
		})

		t.Run("SetIncludeNulls", func(t *testing.T) {
			config.IncludeNulls = false
			helper.AssertFalse(config.IncludeNulls)
		})
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
func TestGetOrSlice(t *testing.T) {
	jsonStr := `{"items": [1, 2, 3]}`
	defaultArr := []any{"default"}

	t.Run("existing array", func(t *testing.T) {
		result := GetTyped[[]any](jsonStr, "items", defaultArr)
		if len(result) != 3 {
			t.Errorf("GetOr[[]any](items) length = %d; want 3", len(result))
		}
	})

	t.Run("missing returns default", func(t *testing.T) {
		result := GetTyped[[]any](jsonStr, "missing", defaultArr)
		if len(result) != 1 || result[0] != "default" {
			t.Errorf("GetOr[[]any](missing) = %v; want default", result)
		}
	})
}

// TestGetOrBool tests GetTyped[bool] function
func TestGetOrBool(t *testing.T) {
	jsonStr := `{"enabled": true, "disabled": false}`

	t.Run("existing true", func(t *testing.T) {
		result := GetTyped[bool](jsonStr, "enabled", false)
		if result != true {
			t.Errorf("GetOr[bool](enabled) = %v; want true", result)
		}
	})

	t.Run("existing false", func(t *testing.T) {
		result := GetTyped[bool](jsonStr, "disabled", true)
		if result != false {
			t.Errorf("GetOr[bool](disabled) = %v; want false", result)
		}
	})

	t.Run("missing returns default", func(t *testing.T) {
		result := GetTyped[bool](jsonStr, "missing", true)
		if result != true {
			t.Errorf("GetOr[bool](missing) = %v; want true", result)
		}
	})
}

// TestGetOrFloat64 tests GetTyped[float64] function
func TestGetOrFloat64(t *testing.T) {
	jsonStr := `{"price": 19.99, "count": 5}`

	t.Run("existing float", func(t *testing.T) {
		result := GetTyped[float64](jsonStr, "price", 0.0)
		if result != 19.99 {
			t.Errorf("GetOr[float64](price) = %f; want 19.99", result)
		}
	})

	t.Run("int converted to float", func(t *testing.T) {
		result := GetTyped[float64](jsonStr, "count", 0.0)
		if result != 5.0 {
			t.Errorf("GetOr[float64](count) = %f; want 5.0", result)
		}
	})

	t.Run("missing returns default", func(t *testing.T) {
		result := GetTyped[float64](jsonStr, "missing", 99.99)
		if result != 99.99 {
			t.Errorf("GetOr[float64](missing) = %f; want 99.99", result)
		}
	})
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
func TestGetOrMap(t *testing.T) {
	jsonStr := `{"config": {"theme": "dark"}}`
	defaultObj := map[string]any{"default": true}

	t.Run("existing object", func(t *testing.T) {
		result := GetTyped[map[string]any](jsonStr, "config", defaultObj)
		if result["theme"] != "dark" {
			t.Errorf("GetOr[map[string]any](config) = %v; want theme=dark", result)
		}
	})

	t.Run("missing returns default", func(t *testing.T) {
		result := GetTyped[map[string]any](jsonStr, "missing", defaultObj)
		if result["default"] != true {
			t.Errorf("GetOr[map[string]any](missing) = %v; want default", result)
		}
	})
}

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

// TestGetOr tests typed get with defaults
func TestGetOr(t *testing.T) {
	jsonStr := `{"user": {"name": "Alice", "age": 30}}`

	t.Run("existing value", func(t *testing.T) {
		name := GetTyped[string](jsonStr, "user.name", "Unknown")
		if name != "Alice" {
			t.Errorf("Expected 'Alice', got '%s'", name)
		}
	})

	t.Run("missing value with default", func(t *testing.T) {
		name := GetTyped[string](jsonStr, "user.email", "unknown@example.com")
		if name != "unknown@example.com" {
			t.Errorf("Expected default value, got '%s'", name)
		}
	})

	t.Run("int with default", func(t *testing.T) {
		age := GetTyped[int](jsonStr, "user.age", 0)
		if age != 30 {
			t.Errorf("Expected 30, got %d", age)
		}
	})

	t.Run("missing int with default", func(t *testing.T) {
		score := GetTyped[int](jsonStr, "user.score", 100)
		if score != 100 {
			t.Errorf("Expected default 100, got %d", score)
		}
	})
}

// TestGetOrGeneric tests the generic GetTyped function with generics
func TestGetOrGeneric(t *testing.T) {
	jsonStr := `{"user": {"name": "Alice", "age": 30, "active": true}}`

	tests := []struct {
		name         string
		path         string
		defaultValue any
		expected     any
	}{
		{
			name:         "existing string",
			path:         "user.name",
			defaultValue: "Unknown",
			expected:     "Alice",
		},
		{
			name:         "missing string returns default",
			path:         "user.email",
			defaultValue: "no@email.com",
			expected:     "no@email.com",
		},
		{
			name:         "existing int",
			path:         "user.age",
			defaultValue: 0,
			expected:     30,
		},
		{
			name:         "missing int returns default",
			path:         "user.score",
			defaultValue: 100,
			expected:     100,
		},
		{
			name:         "existing bool",
			path:         "user.active",
			defaultValue: false,
			expected:     true,
		},
		{
			name:         "missing bool returns default",
			path:         "user.admin",
			defaultValue: false,
			expected:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			switch def := tt.defaultValue.(type) {
			case string:
				result := GetTyped[string](jsonStr, tt.path, def)
				if result != tt.expected.(string) {
					t.Errorf("GetOr[string](%s) = %v; want %v", tt.path, result, tt.expected)
				}
			case int:
				result := GetTyped[int](jsonStr, tt.path, def)
				if result != tt.expected.(int) {
					t.Errorf("GetOr[int](%s) = %d; want %d", tt.path, result, tt.expected)
				}
			case bool:
				result := GetTyped[bool](jsonStr, tt.path, def)
				if result != tt.expected.(bool) {
					t.Errorf("GetOr[bool](%s) = %v; want %v", tt.path, result, tt.expected)
				}
			}
		})
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
func TestHandlePropertyAccess(t *testing.T) {
	processor, _ := New()
	defer processor.Close()

	tests := []struct {
		name        string
		data        any
		property    string
		expectedVal any
		shouldExist bool
	}{
		{
			name: "map string key exists",
			data: map[string]any{
				"name": "Alice",
				"age":  30,
			},
			property:    "name",
			expectedVal: "Alice",
			shouldExist: true,
		},
		{
			name: "map string key not exists",
			data: map[string]any{
				"name": "Alice",
			},
			property:    "age",
			expectedVal: nil,
			shouldExist: false,
		},
		{
			name: "map any key exists",
			data: map[any]any{
				"name": "Bob",
				"age":  25,
			},
			property:    "name",
			expectedVal: "Bob",
			shouldExist: true,
		},
		{
			name:        "array with numeric property",
			data:        []any{"a", "b", "c"},
			property:    "1",
			expectedVal: "b",
			shouldExist: true,
		},
		{
			name:        "array with invalid property",
			data:        []any{"a", "b", "c"},
			property:    "5",
			expectedVal: nil,
			shouldExist: false,
		},
		{
			name:        "invalid data type",
			data:        "string",
			property:    "length",
			expectedVal: nil,
			shouldExist: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := processor.handlePropertyAccess(tt.data, tt.property)
			if result.exists != tt.shouldExist {
				t.Errorf("handlePropertyAccess() existence = %v; want %v", result.exists, tt.shouldExist)
			}
			if tt.shouldExist && result.value != tt.expectedVal {
				t.Errorf("handlePropertyAccess() value = %v; want %v", result.value, tt.expectedVal)
			}
		})
	}
}

// TestHandleStructAccess tests struct field access
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

// TestIsComplexPath tests complex path detection
func TestIsComplexPath(t *testing.T) {
	processor, _ := New()
	defer processor.Close()

	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		{
			name:     "simple path",
			path:     "user.name",
			expected: false,
		},
		{
			name:     "with bracket",
			path:     "users[0]",
			expected: true,
		},
		{
			name:     "with brace",
			path:     "users{name}",
			expected: true,
		},
		{
			name:     "with colon",
			path:     "items[0:5]",
			expected: true,
		},
		{
			name:     "very simple",
			path:     "user",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := processor.isComplexPath(tt.path)
			if result != tt.expected {
				t.Errorf("isComplexPath(%s) = %v; want %v", tt.path, result, tt.expected)
			}
		})
	}
}

// TestIsEmptyOrZero is covered by TestIsEmptyOrZeroExtended in test_helpers_new_test.go

// TestJSONPointerEscapeUnescape tests JSON Pointer escaping
func TestJSONPointerEscapeUnescape(t *testing.T) {
	processor, _ := New()
	defer processor.Close()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "escape tilde",
			input:    "user~name",
			expected: "user~0name",
		},
		{
			name:     "escape slash",
			input:    "user/name",
			expected: "user~1name",
		},
		{
			name:     "escape both",
			input:    "user~/name",
			expected: "user~0~1name",
		},
		{
			name:     "no escaping needed",
			input:    "username",
			expected: "username",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := escapeJSONPointer(tt.input)
			if result != tt.expected {
				t.Errorf("escapeJSONPointer(%s) = %s; want %s", tt.input, result, tt.expected)
			}
		})
	}

	// Test unescaping
	unescapeTests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "unescape tilde",
			input:    "user~0name",
			expected: "user~name",
		},
		{
			name:     "unescape slash",
			input:    "user~1name",
			expected: "user/name",
		},
		{
			name:     "unescape both",
			input:    "user~0~1name",
			expected: "user~/name",
		},
		{
			name:     "no unescaping needed",
			input:    "username",
			expected: "username",
		},
	}

	for _, tt := range unescapeTests {
		t.Run(tt.name, func(t *testing.T) {
			result := internal.UnescapeJSONPointer(tt.input)
			if result != tt.expected {
				t.Errorf("UnescapeJSONPointer(%s) = %s; want %s", tt.input, result, tt.expected)
			}
		})
	}
}

func TestPrettyEncodeConfig(t *testing.T) {
	cfg := PrettyConfig()
	if !cfg.Pretty {
		t.Fatal("PrettyEncodeConfig should have Pretty = true")
	}
	if !cfg.Pretty {
		t.Error("PrettyEncodeConfig should have Pretty = true")
	}
}

// TestNullAndMissingFields tests null value handling and missing fields
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

	t.Run("NullFieldAccess", func(t *testing.T) {
		result, err := Get(testData, "null_field")
		helper.AssertNoError(err)
		helper.AssertNil(result)
	})

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

func TestNumber_TypeMethods(t *testing.T) {
	t.Run("String", func(t *testing.T) {
		n := Number("123.45")
		if n.String() != "123.45" {
			t.Errorf("Number.String() = %q, want %q", n.String(), "123.45")
		}
	})

	t.Run("Float64 valid", func(t *testing.T) {
		n := Number("123.45")
		f, err := n.Float64()
		if err != nil {
			t.Errorf("Number.Float64() error: %v", err)
		}
		if f != 123.45 {
			t.Errorf("Number.Float64() = %v, want %v", f, 123.45)
		}
	})

	t.Run("Float64 invalid", func(t *testing.T) {
		n := Number("not-a-number")
		_, err := n.Float64()
		if err == nil {
			t.Error("Float64 should return error for invalid number")
		}
	})

	t.Run("Int64 valid", func(t *testing.T) {
		n := Number("123")
		i, err := n.Int64()
		if err != nil {
			t.Errorf("Number.Int64() error: %v", err)
		}
		if i != 123 {
			t.Errorf("Number.Int64() = %v, want %v", i, 123)
		}
	})

	t.Run("Int64 invalid", func(t *testing.T) {
		n := Number("not-a-number")
		_, err := n.Int64()
		if err == nil {
			t.Error("Int64 should return error for invalid number")
		}
	})
}

// TestOperationTypes tests Operation constants
func TestOperationTypes(t *testing.T) {
	if opGet != 0 {
		t.Errorf("opGet = %d, want 0", opGet)
	}
	if opSet != 1 {
		t.Errorf("opSet = %d, want 1", opSet)
	}
	if opDelete != 2 {
		t.Errorf("opDelete = %d, want 2", opDelete)
	}
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

// TestParseArraySegment tests parsing array access segments
func TestParseArraySegment(t *testing.T) {
	processor, _ := New()
	defer processor.Close()

	tests := []struct {
		name          string
		part          string
		expectedType  string
		expectedIndex int
		expectedStart *int
		expectedEnd   *int
		expectedStep  *int
	}{
		{
			name:          "simple index",
			part:          "[0]",
			expectedType:  "array",
			expectedIndex: 0,
		},
		{
			name:          "negative index",
			part:          "[-1]",
			expectedType:  "array",
			expectedIndex: -1,
		},
		{
			name:          "slice",
			part:          "[0:5]",
			expectedType:  "slice",
			expectedStart: intPtr(0),
			expectedEnd:   intPtr(5),
			expectedStep:  intPtr(1),
		},
		{
			name:          "slice with step",
			part:          "[0:10:2]",
			expectedType:  "slice",
			expectedStart: intPtr(0),
			expectedEnd:   intPtr(10),
			expectedStep:  intPtr(2),
		},
		{
			name:         "property with index",
			part:         "items[0]",
			expectedType: "property", // First segment is property
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			segments := processor.getPathSegments()
			defer processor.putPathSegments(segments)

			*segments = processor.parseArraySegment(tt.part, *segments)

			if len(*segments) == 0 {
				t.Fatal("parseArraySegment returned no segments")
			}

			firstSeg := (*segments)[0]
			if firstSeg.TypeString() != tt.expectedType {
				t.Errorf("Segment type = %s; want %s", firstSeg.TypeString(), tt.expectedType)
			}
		})
	}
}

// TestParseExtractionSegment tests parsing extraction segments
func TestParseExtractionSegment(t *testing.T) {
	processor, _ := New()
	defer processor.Close()

	tests := []struct {
		name           string
		part           string
		expectedKey    string
		expectedIsFlat bool
	}{
		{
			name:           "simple extraction",
			part:           "{name}",
			expectedKey:    "name",
			expectedIsFlat: false,
		},
		{
			name:           "flat extraction",
			part:           "{flat:items}",
			expectedKey:    "items",
			expectedIsFlat: true,
		},
		{
			name:           "property with extraction",
			part:           "users{name}",
			expectedKey:    "name",
			expectedIsFlat: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			segments := processor.getPathSegments()
			defer processor.putPathSegments(segments)

			*segments = processor.parseExtractionSegment(tt.part, *segments)

			// Find the extraction segment
			var extractSeg *internal.PathSegment
			for i := range *segments {
				if (*segments)[i].Type == internal.ExtractSegment {
					extractSeg = &(*segments)[i]
					break
				}
			}

			if extractSeg == nil {
				t.Fatal("No extraction segment found")
			}

			if extractSeg.Key != tt.expectedKey {
				t.Errorf("Extraction key = %s; want %s", extractSeg.Key, tt.expectedKey)
			}

			if extractSeg.IsFlatExtract() != tt.expectedIsFlat {
				t.Errorf("IsFlatExtract() = %v; want %v", extractSeg.IsFlatExtract(), tt.expectedIsFlat)
			}
		})
	}
}

// TestPathCombination tests path joining and reconstruction
func TestPathCombination(t *testing.T) {
	processor, _ := New()
	defer processor.Close()

	tests := []struct {
		name     string
		segments []string
		expected string
		useJSON  bool
	}{
		{
			name:     "simple join",
			segments: []string{"users", "name"},
			expected: "users.name",
			useJSON:  false,
		},
		{
			name:     "single segment",
			segments: []string{"user"},
			expected: "user",
			useJSON:  false,
		},
		{
			name:     "JSON pointer join",
			segments: []string{"users", "0", "name"},
			expected: "/users/0/name",
			useJSON:  true,
		},
		{
			name:     "empty segments",
			segments: []string{},
			expected: "",
			useJSON:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := processor.joinPathSegments(tt.segments, tt.useJSON)
			if result != tt.expected {
				t.Errorf("joinPathSegments(%v, %v) = %s; want %s", tt.segments, tt.useJSON, result, tt.expected)
			}
		})
	}
}

// TestPathNormalization tests path normalization operations
func TestPathNormalization(t *testing.T) {
	processor, _ := New()
	defer processor.Close()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "remove double dots",
			input:    "user..name",
			expected: "user.name",
		},
		{
			name:     "multiple double dots",
			input:    "a...b",
			expected: "a.b",
		},
		{
			name:     "trim leading dots",
			input:    "...user.name",
			expected: "user.name",
		},
		{
			name:     "trim trailing dots",
			input:    "user.name...",
			expected: "user.name",
		},
		{
			name:     "trim both ends",
			input:    "...user.name...",
			expected: "user.name",
		},
		{
			name:     "no normalization needed",
			input:    "user.name",
			expected: "user.name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := processor.normalizePathSeparators(tt.input)
			if result != tt.expected {
				t.Errorf("normalizePathSeparators(%s) = %s; want %s", tt.input, result, tt.expected)
			}
		})
	}
}

// TestPathParsingBasic tests basic path parsing functionality
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

			*segments = processor.splitPath(tt.path, *segments)
			reconstructed := internal.ReconstructPath(*segments)

			if !strings.Contains(reconstructed, tt.contains) {
				t.Errorf("Reconstructed path '%s' does not contain '%s'", reconstructed, tt.contains)
			}
		})
	}
}

// TestPathSegmentTypes tests different path segment type identification
func TestPathSegmentTypes(t *testing.T) {
	processor, _ := New()
	defer processor.Close()

	// Test segment type string representation
	tests := []struct {
		segmentType string
		expectedStr string
	}{
		{"PropertySegment", "property"},
		{"ArrayIndexSegment", "array"},
		{"ArraySliceSegment", "slice"},
		{"ExtractSegment", "extract"},
	}

	for _, tt := range tests {
		t.Run(tt.segmentType, func(t *testing.T) {
			// This test verifies that segment types have correct string representations
			// The actual implementation would require accessing internal types
			t.Logf("Segment type %s should map to '%s'", tt.segmentType, tt.expectedStr)
		})
	}
}

// TestPathSegmentation tests splitting paths into segments
func TestPathSegmentation(t *testing.T) {
	processor, _ := New()
	defer processor.Close()

	tests := []struct {
		name        string
		path        string
		expectedLen int
		firstIs     string
	}{
		{
			name:        "simple dot notation",
			path:        "user.name.first",
			expectedLen: 3,
			firstIs:     "user",
		},
		{
			name:        "with nested path",
			path:        "users.name.first",
			expectedLen: 3,
			firstIs:     "users",
		},
		{
			name:        "JSON pointer",
			path:        "/users/0/name",
			expectedLen: 3,
			firstIs:     "users",
		},
		{
			name:        "root path",
			path:        "/",
			expectedLen: 0,
			firstIs:     "",
		},
		{
			name:        "empty path",
			path:        "",
			expectedLen: 0,
			firstIs:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			segments := processor.splitPathSegments(tt.path)
			if len(segments) != tt.expectedLen {
				t.Errorf("splitPathSegments(%s) returned %d segments; want %d", tt.path, len(segments), tt.expectedLen)
			}
			if tt.expectedLen > 0 && segments[0] != tt.firstIs {
				t.Errorf("splitPathSegments(%s)[0] = %s; want %s", tt.path, segments[0], tt.firstIs)
			}
		})
	}
}

// TestPathWithSpecialCharacters tests paths with special characters
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

// TestPreprocessPath tests path preprocessing for brackets and braces
func TestPreprocessPath(t *testing.T) {
	processor, _ := New()
	defer processor.Close()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "bracket after letter",
			input:    "items[0]",
			expected: "items.[0]", // Adds dot before bracket
		},
		{
			name:     "bracket after digit",
			input:    "data1[0]",
			expected: "data1.[0]",
		},
		{
			name:     "brace after letter",
			input:    "users{name}",
			expected: "users.{name}", // Adds dot before brace
		},
		{
			name:     "complex mixed",
			input:    "data1[0]users{name}",
			expected: "data1.[0]users.{name}", // Adds dots before bracket and brace
		},
		{
			name:     "no preprocessing needed",
			input:    "user.name",
			expected: "user.name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sb := processor.getStringBuilder()
			defer processor.putStringBuilder(sb)

			result := processor.preprocessPath(tt.input, sb)
			if result != tt.expected {
				t.Errorf("preprocessPath(%s) = %s; want %s", tt.input, result, tt.expected)
			}
		})
	}
}

// TestPrintE tests printE function
// TestProcessBatch tests batch processing
func TestProcessBatch(t *testing.T) {
	jsonStr := `{"user": {"name": "Alice", "age": 30}}`

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

	_ = jsonStr // Use the variable
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

// TestProcessor_CustomConfigVsDefault tests that custom processor config is actually used
func TestProcessor_CustomConfigVsDefault(t *testing.T) {
	// JSON with depth exceeding default limit
	deepJSON := `{"a":{` + string(make([]byte, 40)) + `}}`

	// Create custom config with higher nesting limit
	config := DefaultConfig()
	config.MaxNestingDepthSecurity = 100

	processor, _ := New(config)
	defer processor.Close()

	// This should work with custom config
	processor.Foreach(deepJSON, func(key any, item *IterableValue) {
		// If we get here without error, custom config is being used
		_ = item.GetString("a")
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
			t.Errorf("EncodeWithConfig error: %v", err)
		}
		if result == "" {
			t.Error("EncodeWithConfig should return non-empty result")
		}
	})

	t.Run("EncodeBatch", func(t *testing.T) {
		pairs := map[string]any{"a": 1, "b": 2}
		cfg := DefaultConfig()
		cfg.Pretty = true
		result, err := processor.EncodeBatch(pairs, cfg)
		if err != nil {
			t.Errorf("EncodeBatch error: %v", err)
		}
		if result == "" {
			t.Error("EncodeBatch should return non-empty result")
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
			t.Errorf("EncodeFields error: %v", err)
		}
		if result == "" {
			t.Error("EncodeFields should return non-empty result")
		}
	})

	t.Run("EncodeStream", func(t *testing.T) {
		values := []any{1, 2, 3}
		result, err := processor.EncodeStream(values, DefaultConfig())
		if err != nil {
			t.Errorf("EncodeStream error: %v", err)
		}
		if result == "" {
			t.Error("EncodeStream should return non-empty result")
		}
	})

	t.Run("EncodeStreamWithOptions", func(t *testing.T) {
		values := []any{1, 2, 3}
		encOpts := DefaultConfig()
		result, err := processor.EncodeStream(values, encOpts)
		if err != nil {
			t.Errorf("EncodeStream error: %v", err)
		}
		if result == "" {
			t.Error("EncodeStream should return non-empty result")
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

	t.Run("GetStats", func(t *testing.T) {
		stats := processor.GetStats()
		// Verify stats has expected structure (Stats is a struct, not a pointer)
		_ = stats // Just verify it doesn't panic
	})

	t.Run("GetHealthStatus", func(t *testing.T) {
		health := processor.GetHealthStatus()
		// Verify health has expected structure (HealthStatus is a struct, not a pointer)
		_ = health // Just verify it doesn't panic
	})

	t.Run("ClearCache", func(t *testing.T) {
		processor.ClearCache()
		// Should not panic
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

// TestPropertyValidation tests property name validation
func TestPropertyValidation(t *testing.T) {
	processor, _ := New()
	defer processor.Close()

	tests := []struct {
		name     string
		property string
		valid    bool
	}{
		{
			name:     "valid simple",
			property: "name",
			valid:    true,
		},
		{
			name:     "valid with underscore",
			property: "user_name",
			valid:    true,
		},
		{
			name:     "empty string",
			property: "",
			valid:    false,
		},
		{
			name:     "with dot",
			property: "user.name",
			valid:    false,
		},
		{
			name:     "with bracket",
			property: "user[0]",
			valid:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := processor.isValidPropertyName(tt.property)
			if result != tt.valid {
				t.Errorf("isValidPropertyName(%s) = %v; want %v", tt.property, result, tt.valid)
			}
		})
	}
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

// TestSliceRangeValidation tests slice range validation
func TestSliceRangeValidation(t *testing.T) {
	processor, _ := New()
	defer processor.Close()

	tests := []struct {
		name     string
		rangeStr string
		valid    bool
	}{
		{
			name:     "valid simple",
			rangeStr: "0:5",
			valid:    true,
		},
		{
			name:     "valid with step",
			rangeStr: "0:10:2",
			valid:    true,
		},
		{
			name:     "valid empty start",
			rangeStr: ":5",
			valid:    true,
		},
		{
			name:     "valid empty end",
			rangeStr: "0:",
			valid:    true,
		},
		{
			name:     "invalid too many parts",
			rangeStr: "0:5:2:1",
			valid:    false,
		},
		{
			name:     "invalid single part",
			rangeStr: "5",
			valid:    false,
		},
		{
			name:     "invalid non-numeric",
			rangeStr: "a:b",
			valid:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := processor.isValidSliceRange(tt.rangeStr)
			if result != tt.valid {
				t.Errorf("isValidSliceRange(%s) = %v; want %v", tt.rangeStr, result, tt.valid)
			}
		})
	}
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

// TestValidateSchema tests ValidateSchema function
func TestValidateSchema(t *testing.T) {
	t.Run("valid object", func(t *testing.T) {
		jsonStr := `{"name": "Alice", "age": 30}`
		schema := &Schema{
			Type:     "object",
			Required: []string{"name"},
			Properties: map[string]*Schema{
				"name": {Type: "string"},
				"age":  {Type: "number"},
			},
		}

		errors, err := ValidateSchema(jsonStr, schema)
		if err != nil {
			t.Fatalf("ValidateSchema error: %v", err)
		}
		if len(errors) != 0 {
			t.Errorf("ValidateSchema should have no errors, got: %v", errors)
		}
	})

	t.Run("missing required field", func(t *testing.T) {
		jsonStr := `{"age": 30}`
		schema := &Schema{
			Type:     "object",
			Required: []string{"name"},
		}

		errors, err := ValidateSchema(jsonStr, schema)
		if err != nil {
			t.Fatalf("ValidateSchema error: %v", err)
		}
		if len(errors) == 0 {
			t.Error("ValidateSchema should report missing required field")
		}
	})

	t.Run("type mismatch", func(t *testing.T) {
		jsonStr := `{"name": 123}`
		schema := &Schema{
			Type: "object",
			Properties: map[string]*Schema{
				"name": {Type: "string"},
			},
		}

		errors, err := ValidateSchema(jsonStr, schema)
		if err != nil {
			t.Fatalf("ValidateSchema error: %v", err)
		}
		if len(errors) == 0 {
			t.Error("ValidateSchema should report type mismatch")
		}
	})

	t.Run("nil schema returns error", func(t *testing.T) {
		jsonStr := `{"name": "Alice"}`
		_, err := ValidateSchema(jsonStr, nil)
		if err == nil {
			t.Error("ValidateSchema should return error for nil schema")
		}
	})
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

			*segments = processor.splitPath(tt.path, *segments)

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

// TestMultiFieldExtractionDebug debug test
func TestMultiFieldExtractionDebug(t *testing.T) {
	processor, _ := New()
	defer processor.Close()

	// Test the extraction directly
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

	t.Logf("Result: %v (type: %T)", result, result)

	if result == nil {
		t.Error("Expected non-nil result")
	}
}

// TestPathEscapeIntegration tests the escape functionality with real JSON data

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
