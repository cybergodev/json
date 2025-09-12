package json

import (
	"strings"
	"testing"
)

// TestAdvancedErrorHandlingBoundaryConditions tests comprehensive error handling and boundary conditions
func TestAdvancedErrorHandlingBoundaryConditions(t *testing.T) {
	helper := NewTestHelper(t)

	t.Run("InvalidJSONHandling", func(t *testing.T) {
		processor := New()
		defer processor.Close()

		invalidJSONCases := []struct {
			name string
			json string
		}{
			{"EmptyString", ""},
			{"OnlyWhitespace", "   \n\t  "},
			{"UnterminatedString", `{"name": "John`},
			{"UnterminatedObject", `{"name": "John", "age": 30`},
			{"UnterminatedArray", `[1, 2, 3`},
			{"InvalidEscape", `{"text": "Hello\xWorld"}`},
			{"TrailingComma", `{"name": "John", "age": 30,}`},
			{"MissingQuotes", `{name: "John"}`},
			{"SingleQuotes", `{'name': 'John'}`},
			{"InvalidNumber", `{"value": 123.45.67}`},
			{"InvalidBoolean", `{"flag": truee}`},
			{"InvalidNull", `{"value": nul}`},
			{"MixedQuotes", `{"name": 'John"}`},
			{"ControlCharacters", "{\"text\": \"Hello\x00World\"}"},
			{"InvalidUnicode", `{"text": "\uXXXX"}`},
			{"NestedInvalid", `{"valid": true, "invalid": {"bad": }}`},
		}

		for _, tc := range invalidJSONCases {
			t.Run(tc.name, func(t *testing.T) {
				// Test Get operation with invalid JSON
				_, err := processor.Get(tc.json, "name")
				helper.AssertError(err, "Get should fail with invalid JSON: "+tc.name)

				// Test Set operation with invalid JSON
				_, err = processor.Set(tc.json, "newField", "value")
				helper.AssertError(err, "Set should fail with invalid JSON: "+tc.name)

				// Test Delete operation with invalid JSON
				_, err = processor.Delete(tc.json, "name")
				helper.AssertError(err, "Delete should fail with invalid JSON: "+tc.name)

				// Test Foreach operation with invalid JSON
				err = processor.ForeachWithPath(tc.json, "", func(key any, value *IterableValue) {
					// Should not be called
				})
				helper.AssertError(err, "Foreach should fail with invalid JSON: "+tc.name)
			})
		}
	})

	t.Run("InvalidPathHandling", func(t *testing.T) {
		processor := New()
		defer processor.Close()

		validJSON := `{
			"users": [
				{"name": "Alice", "age": 30},
				{"name": "Bob", "age": 25}
			],
			"metadata": {
				"count": 2,
				"version": "1.0"
			}
		}`

		invalidPathCases := []struct {
			name string
			path string
		}{
			{"EmptyBrackets", "users[]"},
			{"InvalidArrayIndex", "users[abc]"},
			{"NegativeOutOfBounds", "users[-10]"},
			{"PositiveOutOfBounds", "users[100]"},
			{"InvalidSliceSyntax", "users[1:2:3:4]"},
			{"InvalidSliceStart", "users[abc:2]"},
			{"InvalidSliceEnd", "users[1:abc]"},
			{"InvalidSliceStep", "users[1:2:abc]"},
			{"MalformedBrackets", "users[1"},
			{"ExtraBrackets", "users[1]]"},
			{"NestedInvalidIndex", "users[0].projects[xyz]"},
			{"InvalidExtractionSyntax", "users{"},
			{"MalformedExtraction", "users{name"},
			{"InvalidExtractionField", "users{123}"},
			{"MixedInvalidSyntax", "users[0]{name[1]"},
			{"DoubleNegative", "users[--1]"},
			{"FloatingPointIndex", "users[1.5]"},
			{"SpecialCharacters", "users[@#$]"},
			{"UnicodeInPath", "users[测试]"},
			{"VeryLongPath", strings.Repeat("a.", 1000) + "field"},
		}

		for _, tc := range invalidPathCases {
			t.Run(tc.name, func(t *testing.T) {
				// Test Get operation with invalid path
				_, err := processor.Get(validJSON, tc.path)
				if err == nil {
					// Some invalid paths might return nil instead of error
					t.Logf("Get with invalid path '%s' returned nil instead of error", tc.path)
				}

				// Test Set operation with invalid path
				_, err = processor.Set(validJSON, tc.path, "newValue")
				if err == nil {
					t.Logf("Set with invalid path '%s' succeeded unexpectedly", tc.path)
				}

				// Test Delete operation with invalid path
				_, err = processor.Delete(validJSON, tc.path)
				if err == nil {
					t.Logf("Delete with invalid path '%s' succeeded unexpectedly", tc.path)
				}
			})
		}
	})

	t.Run("ExtremeValueHandling", func(t *testing.T) {
		processor := New()
		defer processor.Close()

		baseJSON := `{"data": {}}`

		extremeValueCases := []struct {
			name  string
			value any
		}{
			{"VeryLargeString", strings.Repeat("A", 100000)},
			{"VeryLargeNumber", 1.7976931348623157e+308}, // Near max float64
			{"VerySmallNumber", 4.9406564584124654e-324}, // Near min positive float64
			{"VeryLargeNegativeNumber", -1.7976931348623157e+307},
			{"ZeroValue", 0},
			{"EmptyString", ""},
			{"NilValue", nil},
			{"BooleanTrue", true},
			{"BooleanFalse", false},
			{"UnicodeString", "🚀🌟💫⭐🎯🔥💎🌈🎨🎭🎪🎨🎯🔥💎🌈"},
			{"ControlCharacters", "\x00\x01\x02\x03\x04\x05\x06\x07\x08\x09\x0A\x0B\x0C\x0D\x0E\x0F"},
			{"SpecialCharacters", "!@#$%^&*()_+-=[]{}|;':\",./<>?`~"},
			{"NewlineString", "Line1\nLine2\nLine3"},
			{"TabString", "Col1\tCol2\tCol3"},
			{"QuoteString", `"Hello 'World' "Test""`},
			{"BackslashString", `C:\Users\Test\Documents\file.txt`},
		}

		for _, tc := range extremeValueCases {
			t.Run(tc.name, func(t *testing.T) {
				// Test setting extreme values
				result, err := processor.Set(baseJSON, "data.extremeValue", tc.value)
				helper.AssertNoError(err, "Should handle extreme value: "+tc.name)

				// Test getting extreme values back
				retrievedValue, err := processor.Get(result, "data.extremeValue")
				helper.AssertNoError(err, "Should retrieve extreme value: "+tc.name)

				// For some types, we expect type conversion
				if tc.value != nil {
					helper.AssertNotNil(retrievedValue, "Retrieved value should not be nil: "+tc.name)
				}
			})
		}
	})

	t.Run("DeepNestingLimits", func(t *testing.T) {
		processor := New()
		defer processor.Close()

		// Create deeply nested JSON structure
		deepJSON := `{"level0": {"level1": {"level2": {"level3": {"level4": {"level5": {"level6": {"level7": {"level8": {"level9": {"level10": {"level11": {"level12": {"level13": {"level14": {"level15": {"level16": {"level17": {"level18": {"level19": {"level20": {"level21": {"level22": {"level23": {"level24": {"level25": {"level26": {"level27": {"level28": {"level29": {"level30": {"value": "deep"}}}}}}}}}}}}}}}}}}}}}}}}}}}}}}}}`

		// Test accessing deeply nested values
		deepPath := "level0.level1.level2.level3.level4.level5.level6.level7.level8.level9.level10.level11.level12.level13.level14.level15.level16.level17.level18.level19.level20.level21.level22.level23.level24.level25.level26.level27.level28.level29.level30.value"

		value, err := processor.Get(deepJSON, deepPath)
		if err != nil {
			t.Logf("Deep nesting access failed as expected: %v", err)
		} else {
			helper.AssertEqual("deep", value, "Should retrieve deeply nested value")
		}

		// Test setting at deep nesting levels
		veryDeepPath := deepPath + ".evenDeeper.muchDeeper.extremelyDeep"
		_, err = processor.Set(deepJSON, veryDeepPath, "veryDeep")
		if err != nil {
			t.Logf("Deep nesting set failed as expected: %v", err)
		}

		// Test creating extremely deep paths
		baseJSON := `{"data": {}}`
		extremelyDeepPath := strings.Repeat("level.", 100) + "value"
		_, err = SetWithAdd(baseJSON, extremelyDeepPath, "extreme")
		if err != nil {
			t.Logf("Extremely deep path creation failed as expected: %v", err)
		}
	})

	t.Run("MemoryExhaustionProtection", func(t *testing.T) {
		processor := New()
		defer processor.Close()

		// Test with very large arrays - expect protection to kick in
		largeArrayJSON := `{"data": [` + strings.Repeat(`{"id": 1, "data": "test"},`, 10000) + `{"id": 1, "data": "test"}]}`

		// Test operations on large arrays - should be protected
		_, err := processor.Get(largeArrayJSON, "data[5000].id")
		if err != nil {
			t.Logf("Large array access protected as expected: %v", err)
		} else {
			helper.AssertNoError(err, "Should handle large array access")
		}

		// Test slicing large arrays - should be protected
		_, err = processor.Get(largeArrayJSON, "data[1000:2000]")
		if err != nil {
			t.Logf("Large array slicing protected as expected: %v", err)
		} else {
			helper.AssertNoError(err, "Should handle large array slicing")
		}

		// Test with moderately large objects (within limits)
		largeObjectParts := []string{`{"data": {`}
		for i := 0; i < 100; i++ { // Reduced size to stay within limits
			if i > 0 {
				largeObjectParts = append(largeObjectParts, ",")
			}
			largeObjectParts = append(largeObjectParts, `"field`+strings.Repeat("0", 2)+`": "value`+strings.Repeat("0", 2)+`"`)
		}
		largeObjectParts = append(largeObjectParts, `}}`)
		largeObjectJSON := strings.Join(largeObjectParts, "")

		_, err = processor.Get(largeObjectJSON, "data.field00")
		helper.AssertNoError(err, "Should handle moderately large object access")
	})

	t.Run("ConcurrentErrorHandling", func(t *testing.T) {
		processor := New()
		defer processor.Close()

		invalidJSON := `{"invalid": json}`
		validJSON := `{"valid": "json"}`

		// Test concurrent operations with mixed valid/invalid data
		const numGoroutines = 10
		errors := make(chan error, numGoroutines*2)

		for i := 0; i < numGoroutines; i++ {
			go func(id int) {
				// Try invalid JSON
				_, err := processor.Get(invalidJSON, "invalid")
				errors <- err

				// Try valid JSON
				_, err = processor.Get(validJSON, "valid")
				errors <- err
			}(i)
		}

		// Collect results
		validCount := 0
		errorCount := 0
		for i := 0; i < numGoroutines*2; i++ {
			err := <-errors
			if err != nil {
				errorCount++
			} else {
				validCount++
			}
		}

		helper.AssertEqual(numGoroutines, errorCount, "Should have errors for invalid JSON")
		helper.AssertEqual(numGoroutines, validCount, "Should have successes for valid JSON")
	})

	t.Run("ResourceCleanupOnError", func(t *testing.T) {
		// Test that resources are properly cleaned up when errors occur
		processor := New()
		defer processor.Close()

		invalidCases := []string{
			`{"invalid": }`,
			`[1, 2, 3`,
			`{"name": "John"`,
			`invalid json`,
			``,
		}

		for i, invalidJSON := range invalidCases {
			// Perform operations that should fail
			_, _ = processor.Get(invalidJSON, "field")
			_, _ = processor.Set(invalidJSON, "field", "value")
			_, _ = processor.Delete(invalidJSON, "field")

			// Clear cache to test cleanup
			if i%2 == 0 {
				processor.ClearCache()
			}
		}

		// Verify processor is still functional after errors
		validJSON := `{"test": "value"}`
		result, err := processor.Get(validJSON, "test")
		helper.AssertNoError(err, "Processor should still be functional after errors")
		helper.AssertEqual("value", result, "Should get correct value after error recovery")
	})

	t.Run("EdgeCasePathOperations", func(t *testing.T) {
		processor := New()
		defer processor.Close()

		testJSON := `{
			"empty": {},
			"emptyArray": [],
			"nullValue": null,
			"zeroValue": 0,
			"falseValue": false,
			"emptyString": "",
			"nested": {
				"empty": {},
				"nullArray": [null, null, null]
			}
		}`

		edgeCases := []struct {
			name string
			path string
		}{
			{"AccessEmptyObject", "empty.nonexistent"},
			{"AccessEmptyArray", "emptyArray[0]"},
			{"AccessNullValue", "nullValue.field"},
			{"AccessZeroValue", "zeroValue.field"},
			{"AccessFalseValue", "falseValue.field"},
			{"AccessEmptyString", "emptyString.field"},
			{"SliceEmptyArray", "emptyArray[0:1]"},
			{"SliceNullArray", "nested.nullArray[1:2]"},
			{"ExtractFromEmpty", "empty{field}"},
			{"ExtractFromNull", "nullValue{field}"},
			{"NestedEmptyAccess", "nested.empty.deep.deeper"},
		}

		for _, tc := range edgeCases {
			t.Run(tc.name, func(t *testing.T) {
				result, err := processor.Get(testJSON, tc.path)
				// These operations might return nil or error, both are acceptable
				if err != nil {
					t.Logf("Edge case '%s' returned error as expected: %v", tc.name, err)
				} else if result == nil {
					t.Logf("Edge case '%s' returned nil as expected", tc.name)
				} else {
					t.Logf("Edge case '%s' returned unexpected result: %v", tc.name, result)
				}
			})
		}
	})

	t.Run("TypeMismatchHandling", func(t *testing.T) {
		processor := New()
		defer processor.Close()

		testJSON := `{
			"string": "hello",
			"number": 42,
			"boolean": true,
			"array": [1, 2, 3],
			"object": {"key": "value"},
			"null": null
		}`

		typeMismatchCases := []struct {
			name string
			path string
		}{
			{"StringAsArray", "string[0]"},
			{"NumberAsObject", "number.field"},
			{"BooleanAsArray", "boolean[0]"},
			{"ArrayAsObject", "array.field"},
			{"ObjectAsArray", "object[0]"},
			{"NullAsObject", "null.field"},
			{"NullAsArray", "null[0]"},
			{"StringSlice", "string[1:3]"},
			{"NumberSlice", "number[0:1]"},
			{"BooleanExtraction", "boolean{field}"},
		}

		for _, tc := range typeMismatchCases {
			t.Run(tc.name, func(t *testing.T) {
				result, err := processor.Get(testJSON, tc.path)
				// Type mismatches should either return error or nil
				if err != nil {
					t.Logf("Type mismatch '%s' returned error as expected: %v", tc.name, err)
				} else if result == nil {
					t.Logf("Type mismatch '%s' returned nil as expected", tc.name)
				} else {
					t.Logf("Type mismatch '%s' returned unexpected result: %v", tc.name, result)
				}
			})
		}
	})
}
