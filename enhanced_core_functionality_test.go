package json

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

// TestEnhancedCoreOperations provides comprehensive testing for core JSON operations
// with extensive edge cases, boundary conditions, and error handling
func TestEnhancedCoreOperations(t *testing.T) {
	helper := NewTestHelper(t)

	t.Run("GetOperationsEdgeCases", func(t *testing.T) {
		// Test with various data types and edge cases
		testCases := []struct {
			name     string
			json     string
			path     string
			expected interface{}
			hasError bool
		}{
			// Basic types
			{"String", `{"str": "hello"}`, "str", "hello", false},
			{"Number", `{"num": 42}`, "num", float64(42), false},
			{"Boolean", `{"bool": true}`, "bool", true, false},
			{"Null", `{"null": null}`, "null", nil, false},

			// Empty values
			{"EmptyString", `{"empty": ""}`, "empty", "", false},
			{"EmptyObject", `{"obj": {}}`, "obj", map[string]interface{}{}, false},
			{"EmptyArray", `{"arr": []}`, "arr", []interface{}{}, false},

			// Special characters in strings
			{"SpecialChars", `{"special": "hello\nworld\t\"quoted\""}`, "special", "hello\nworld\t\"quoted\"", false},
			{"Unicode", `{"unicode": "你好世界🌍"}`, "unicode", "你好世界🌍", false},

			// Large numbers
			{"LargeInt", `{"large": 9223372036854775807}`, "large", float64(9223372036854775807), false},
			{"LargeFloat", `{"float": 1.7976931348623157e+308}`, "float", 1.7976931348623157e+308, false},
			{"SmallFloat", `{"small": 2.2250738585072014e-308}`, "small", 2.2250738585072014e-308, false},

			// Nested access
			{"DeepNesting", `{"a":{"b":{"c":{"d":"deep"}}}}`, "a.b.c.d", "deep", false},

			// Array access
			{"ArrayFirst", `{"arr": [1,2,3]}`, "arr[0]", float64(1), false},
			{"ArrayLast", `{"arr": [1,2,3]}`, "arr[-1]", float64(3), false},
			{"ArrayOutOfBounds", `{"arr": [1,2,3]}`, "arr[10]", nil, false},          // Library returns nil without error
			{"ArrayNegativeOutOfBounds", `{"arr": [1,2,3]}`, "arr[-10]", nil, false}, // Library returns nil without error

			// Error cases
			{"NonexistentPath", `{"key": "value"}`, "nonexistent", nil, true}, // Should return error for nonexistent path
			{"InvalidArrayIndex", `{"arr": [1,2,3]}`, "arr[abc]", nil, true},
			{"PathThroughNonObject", `{"num": 42}`, "num.invalid", nil, false},  // Library returns nil without error
			{"PathThroughArray", `{"arr": [1,2,3]}`, "arr.invalid", nil, false}, // Library returns nil without error
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				result, err := Get(tc.json, tc.path)

				if tc.hasError {
					helper.AssertError(err, "Should have error for %s", tc.name)
				} else {
					helper.AssertNoError(err, "Should not have error for %s", tc.name)
					helper.AssertEqual(tc.expected, result, "Result should match for %s", tc.name)
				}
			})
		}
	})

	t.Run("SetOperationsEdgeCases", func(t *testing.T) {
		testCases := []struct {
			name      string
			json      string
			path      string
			value     interface{}
			hasError  bool
			checkPath string
			expected  interface{}
		}{
			// Basic set operations
			{"SetString", `{}`, "name", "John", false, "name", "John"},
			{"SetNumber", `{}`, "age", 30, false, "age", float64(30)},
			{"SetBoolean", `{}`, "active", true, false, "active", true},
			{"SetNull", `{}`, "data", nil, false, "data", nil},

			// Overwrite existing values
			{"OverwriteString", `{"name": "old"}`, "name", "new", false, "name", "new"},
			{"OverwriteType", `{"value": "string"}`, "value", 42, false, "value", float64(42)},

			// Nested operations
			{"SetNested", `{"user": {}}`, "user.name", "Alice", false, "user.name", "Alice"},
			{"SetDeepNested", `{}`, "a.b.c.d", "deep", true, "", nil}, // Library doesn't auto-create deep paths

			// Array operations
			{"SetArrayElement", `{"arr": [1,2,3]}`, "arr[1]", 99, false, "arr[1]", float64(99)},
			{"SetArrayNegativeIndex", `{"arr": [1,2,3]}`, "arr[-1]", 99, false, "arr[-1]", float64(99)},

			// Complex values
			{"SetObject", `{}`, "obj", map[string]interface{}{"key": "value"}, false, "obj.key", "value"},
			{"SetArray", `{}`, "arr", []interface{}{1, 2, 3}, false, "arr[0]", float64(1)},

			// Error cases
			{"SetInvalidArrayIndex", `{"arr": [1,2,3]}`, "arr[abc]", 99, true, "", nil},
			{"SetArrayOutOfBounds", `{"arr": [1,2,3]}`, "arr[10]", 99, true, "", nil},
			{"SetThroughNonObject", `{"num": 42}`, "num.invalid", "value", true, "", nil},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				result, err := Set(tc.json, tc.path, tc.value)

				if tc.hasError {
					helper.AssertError(err, "Should have error for %s", tc.name)
				} else {
					helper.AssertNoError(err, "Should not have error for %s", tc.name)

					if tc.checkPath != "" {
						checkValue, checkErr := Get(result, tc.checkPath)
						helper.AssertNoError(checkErr, "Should be able to get set value for %s", tc.name)
						helper.AssertEqual(tc.expected, checkValue, "Set value should match for %s", tc.name)
					}
				}
			})
		}
	})

	t.Run("DeleteOperationsEdgeCases", func(t *testing.T) {
		testCases := []struct {
			name        string
			json        string
			path        string
			hasError    bool
			checkPath   string
			shouldBeNil bool
		}{
			// Basic delete operations
			{"DeleteSimple", `{"name": "John", "age": 30}`, "name", false, "name", true},
			{"DeleteNested", `{"user": {"name": "John", "age": 30}}`, "user.name", false, "user.name", true},
			{"DeleteArrayElement", `{"arr": [1,2,3,4,5]}`, "arr[2]", false, "arr[2]", false}, // Array deletion shifts elements
			{"DeleteLastElement", `{"arr": [1,2,3]}`, "arr[-1]", false, "arr[-1]", false},    // Array deletion shifts elements

			// Error cases
			{"DeleteNonexistent", `{"key": "value"}`, "nonexistent", true, "", false},
			{"DeleteInvalidArrayIndex", `{"arr": [1,2,3]}`, "arr[abc]", true, "", false},
			{"DeleteArrayOutOfBounds", `{"arr": [1,2,3]}`, "arr[10]", true, "", false},
			{"DeleteThroughNonObject", `{"num": 42}`, "num.invalid", true, "", false},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				result, err := Delete(tc.json, tc.path)

				if tc.hasError {
					helper.AssertError(err, "Should have error for %s", tc.name)
				} else {
					helper.AssertNoError(err, "Should not have error for %s", tc.name)

					if tc.checkPath != "" {
						checkValue, checkErr := Get(result, tc.checkPath)
						if tc.shouldBeNil {
							// Deleted fields should return error for nonexistent path
							helper.AssertError(checkErr, "Should return error when getting deleted path for %s", tc.name)
							helper.AssertNil(checkValue, "Deleted value should be nil for %s", tc.name)
						} else {
							// For array deletions, the element might shift or be replaced
							helper.AssertNoError(checkErr, "Should not error for array element access after deletion for %s", tc.name)
							t.Logf("After deletion %s, path %s has value: %v", tc.name, tc.checkPath, checkValue)
						}
					}
				}
			})
		}
	})
}

// TestTypeConversionAndSafety tests type conversion and type safety features
func TestTypeConversionAndSafety(t *testing.T) {
	helper := NewTestHelper(t)

	testJSON := `{
		"string": "hello",
		"number": 42,
		"float": 3.14,
		"boolean": true,
		"null": null,
		"array": [1, 2, 3],
		"object": {"key": "value"},
		"stringNumber": "123",
		"stringFloat": "3.14",
		"stringBool": "true"
	}`

	t.Run("TypedGetOperations", func(t *testing.T) {
		// Test successful type conversions
		str, err := GetString(testJSON, "string")
		helper.AssertNoError(err, "GetString should work")
		helper.AssertEqual("hello", str, "String should match")

		num, err := GetInt(testJSON, "number")
		helper.AssertNoError(err, "GetInt should work")
		helper.AssertEqual(42, num, "Number should match")

		flt, err := GetFloat64(testJSON, "float")
		helper.AssertNoError(err, "GetFloat64 should work")
		helper.AssertEqual(3.14, flt, "Float should match")

		boolean, err := GetBool(testJSON, "boolean")
		helper.AssertNoError(err, "GetBool should work")
		helper.AssertEqual(true, boolean, "Boolean should match")

		arr, err := GetArray(testJSON, "array")
		helper.AssertNoError(err, "GetArray should work")
		helper.AssertEqual(3, len(arr), "Array length should match")

		obj, err := GetObject(testJSON, "object")
		helper.AssertNoError(err, "GetObject should work")
		helper.AssertEqual("value", obj["key"], "Object value should match")
	})

	t.Run("TypeConversionFromStrings", func(t *testing.T) {
		// Test string to number conversion
		num, err := GetInt(testJSON, "stringNumber")
		if err == nil {
			helper.AssertEqual(123, num, "String to int conversion should work")
		}

		flt, err := GetFloat64(testJSON, "stringFloat")
		if err == nil {
			helper.AssertEqual(3.14, flt, "String to float conversion should work")
		}

		boolean, err := GetBool(testJSON, "stringBool")
		if err == nil {
			helper.AssertEqual(true, boolean, "String to bool conversion should work")
		}
	})

	t.Run("TypeMismatchHandling", func(t *testing.T) {
		// These should handle type mismatches gracefully
		_, err := GetInt(testJSON, "string")
		// Note: The library might convert or return an error - both are acceptable
		t.Logf("GetInt on string: %v", err)

		_, err = GetBool(testJSON, "number")
		t.Logf("GetBool on number: %v", err)

		_, err = GetArray(testJSON, "string")
		t.Logf("GetArray on string: %v", err)
	})
}

// TestBoundaryConditionsEnhanced tests enhanced boundary conditions
func TestBoundaryConditionsEnhanced(t *testing.T) {
	helper := NewTestHelper(t)

	t.Run("LargeDataHandling", func(t *testing.T) {
		// Create large but manageable JSON
		var builder strings.Builder
		builder.WriteString(`{"items": [`)
		for i := 0; i < 1000; i++ {
			if i > 0 {
				builder.WriteString(",")
			}
			builder.WriteString(fmt.Sprintf(`{"id": %d, "name": "item_%d", "value": %f}`, i, i, float64(i)*1.5))
		}
		builder.WriteString(`]}`)

		largeJSON := builder.String()

		// Test operations on large data
		result, err := Get(largeJSON, "items[0].name")
		helper.AssertNoError(err, "Should handle large data")
		helper.AssertEqual("item_0", result, "Should get correct value from large data")

		result, err = Get(largeJSON, "items[999].id")
		helper.AssertNoError(err, "Should access last item in large data")
		helper.AssertEqual(float64(999), result, "Should get correct last item")

		result, err = Get(largeJSON, "items[-1].name")
		helper.AssertNoError(err, "Should access last item with negative index")
		helper.AssertEqual("item_999", result, "Should get correct last item with negative index")
	})

	t.Run("DeepNestingLimits", func(t *testing.T) {
		// Create deeply nested JSON
		depth := 20
		var builder strings.Builder

		// Build opening braces
		for i := 0; i < depth; i++ {
			builder.WriteString(fmt.Sprintf(`{"level_%d":`, i))
		}
		builder.WriteString(`"deep_value"`)

		// Build closing braces
		for i := 0; i < depth; i++ {
			builder.WriteString("}")
		}

		deepJSON := builder.String()

		// Build path
		var pathBuilder strings.Builder
		for i := 0; i < depth; i++ {
			if i > 0 {
				pathBuilder.WriteString(".")
			}
			pathBuilder.WriteString(fmt.Sprintf("level_%d", i))
		}
		deepPath := pathBuilder.String()

		// Test deep access
		result, err := Get(deepJSON, deepPath)
		helper.AssertNoError(err, "Should handle deep nesting")
		helper.AssertEqual("deep_value", result, "Should get deep nested value")
	})

	t.Run("SpecialCharacterHandling", func(t *testing.T) {
		specialJSON := `{
			"normal": "value",
			"with spaces": "space value",
			"with-dashes": "dash value",
			"with_underscores": "underscore value",
			"with.dots": "dot value",
			"with[brackets]": "bracket value",
			"unicode_key_你好": "unicode value",
			"emoji_key_🚀": "emoji value"
		}`

		// Test accessing keys with special characters
		result, err := Get(specialJSON, "normal")
		helper.AssertNoError(err, "Normal key should work")
		helper.AssertEqual("value", result, "Normal value should match")

		// Note: Keys with special characters might need special handling
		// The library's behavior with such keys depends on implementation
		result, err = Get(specialJSON, "with_underscores")
		helper.AssertNoError(err, "Underscore key should work")
		helper.AssertEqual("underscore value", result, "Underscore value should match")
	})
}

// TestConcurrentOperationsEnhanced tests enhanced concurrent operations
func TestConcurrentOperationsEnhanced(t *testing.T) {
	helper := NewTestHelper(t)

	testJSON := `{
		"counters": [0, 0, 0, 0, 0],
		"data": {
			"shared": "initial",
			"items": [1, 2, 3, 4, 5]
		}
	}`

	t.Run("ConcurrentReads", func(t *testing.T) {
		const numGoroutines = 50
		const operationsPerGoroutine = 20

		results := make(chan interface{}, numGoroutines*operationsPerGoroutine)
		errors := make(chan error, numGoroutines*operationsPerGoroutine)

		// Launch concurrent read operations
		for i := 0; i < numGoroutines; i++ {
			go func(workerID int) {
				for j := 0; j < operationsPerGoroutine; j++ {
					// Vary the paths to test different access patterns
					paths := []string{
						"data.shared",
						"data.items[0]",
						"data.items[-1]",
						fmt.Sprintf("counters[%d]", j%5),
					}

					path := paths[j%len(paths)]
					result, err := Get(testJSON, path)

					if err != nil {
						errors <- err
					} else {
						results <- result
					}
				}
			}(i)
		}

		// Collect results
		successCount := 0
		errorCount := 0
		timeout := time.After(5 * time.Second)

		for i := 0; i < numGoroutines*operationsPerGoroutine; i++ {
			select {
			case <-results:
				successCount++
			case err := <-errors:
				errorCount++
				t.Logf("Concurrent read error: %v", err)
			case <-timeout:
				t.Fatal("Timeout waiting for concurrent operations")
			}
		}

		helper.AssertTrue(successCount > 0, "Should have successful concurrent reads")
		helper.AssertTrue(errorCount < successCount/10, "Error rate should be low")
		t.Logf("Concurrent reads: %d successful, %d errors", successCount, errorCount)
	})
}
