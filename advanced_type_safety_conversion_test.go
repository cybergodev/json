package json

import (
	"reflect"
	"testing"
)

// TestAdvancedTypeSafetyConversion tests comprehensive type safety and conversion operations
func TestAdvancedTypeSafetyConversion(t *testing.T) {
	helper := NewTestHelper(t)

	t.Run("TypeSafeGetOperations", func(t *testing.T) {
		processor := New()
		defer processor.Close()

		testJSON := `{
			"string": "hello world",
			"number": 42.5,
			"integer": 100,
			"boolean": true,
			"null": null,
			"array": [1, 2, 3, "four", true],
			"object": {
				"nested": "value",
				"count": 10
			},
			"stringNumber": "123",
			"stringBoolean": "true",
			"stringFloat": "45.67",
			"emptyString": "",
			"zeroNumber": 0,
			"falseBoolean": false
		}`

		// Test string retrieval and type safety
		stringValue, err := processor.Get(testJSON, "string")
		helper.AssertNoError(err, "Should get string value")
		helper.AssertEqual("hello world", stringValue, "String value should match")
		helper.AssertTrue(reflect.TypeOf(stringValue).Kind() == reflect.String, "Should be string type")

		// Test number retrieval and type safety
		numberValue, err := processor.Get(testJSON, "number")
		helper.AssertNoError(err, "Should get number value")
		helper.AssertEqual(42.5, numberValue, "Number value should match")
		helper.AssertTrue(reflect.TypeOf(numberValue).Kind() == reflect.Float64, "Should be float64 type")

		// Test integer retrieval
		integerValue, err := processor.Get(testJSON, "integer")
		helper.AssertNoError(err, "Should get integer value")
		helper.AssertEqual(float64(100), integerValue, "Integer value should match")

		// Test boolean retrieval
		booleanValue, err := processor.Get(testJSON, "boolean")
		helper.AssertNoError(err, "Should get boolean value")
		helper.AssertEqual(true, booleanValue, "Boolean value should match")
		helper.AssertTrue(reflect.TypeOf(booleanValue).Kind() == reflect.Bool, "Should be bool type")

		// Test null value handling
		nullValue, err := processor.Get(testJSON, "null")
		helper.AssertNoError(err, "Should get null value")
		helper.AssertNil(nullValue, "Null value should be nil")

		// Test array retrieval
		arrayValue, err := processor.Get(testJSON, "array")
		helper.AssertNoError(err, "Should get array value")
		helper.AssertTrue(reflect.TypeOf(arrayValue).Kind() == reflect.Slice, "Should be slice type")

		// Test object retrieval
		objectValue, err := processor.Get(testJSON, "object")
		helper.AssertNoError(err, "Should get object value")
		helper.AssertTrue(reflect.TypeOf(objectValue).Kind() == reflect.Map, "Should be map type")
	})

	t.Run("TypeConversionOperations", func(t *testing.T) {
		processor := New()
		defer processor.Close()

		baseJSON := `{"data": {}}`

		conversionCases := []struct {
			name          string
			setValue      any
			expectedType  reflect.Kind
			path          string
		}{
			{"StringToString", "test", reflect.String, "data.stringValue"},
			{"IntToFloat", 42, reflect.Float64, "data.intValue"},
			{"FloatToFloat", 42.5, reflect.Float64, "data.floatValue"},
			{"BoolToBool", true, reflect.Bool, "data.boolValue"},
			{"NilToNil", nil, reflect.Invalid, "data.nilValue"},
			{"SliceToSlice", []any{1, 2, 3}, reflect.Slice, "data.sliceValue"},
			{"MapToMap", map[string]any{"key": "value"}, reflect.Map, "data.mapValue"},
		}

		for _, tc := range conversionCases {
			t.Run(tc.name, func(t *testing.T) {
				// Set the value
				result, err := processor.Set(baseJSON, tc.path, tc.setValue)
				helper.AssertNoError(err, "Should set value: "+tc.name)

				// Get the value back
				retrievedValue, err := processor.Get(result, tc.path)
				helper.AssertNoError(err, "Should get value back: "+tc.name)

				if tc.expectedType == reflect.Invalid {
					helper.AssertNil(retrievedValue, "Should be nil: "+tc.name)
				} else {
					helper.AssertNotNil(retrievedValue, "Should not be nil: "+tc.name)
					actualType := reflect.TypeOf(retrievedValue).Kind()
					helper.AssertEqual(tc.expectedType, actualType, "Type should match: "+tc.name)
				}
			})
		}
	})

	t.Run("NumericTypeHandling", func(t *testing.T) {
		processor := New()
		defer processor.Close()

		numericJSON := `{
			"int8": 127,
			"int16": 32767,
			"int32": 2147483647,
			"int64": 9223372036854775807,
			"uint8": 255,
			"uint16": 65535,
			"uint32": 4294967295,
			"float32": 3.14159,
			"float64": 3.141592653589793,
			"scientific": 1.23e10,
			"negative": -42,
			"zero": 0,
			"stringInt": "123",
			"stringFloat": "45.67",
			"stringScientific": "1.23e-4"
		}`

		numericCases := []struct {
			name     string
			path     string
			expected any
		}{
			{"Int8", "int8", float64(127)},
			{"Int16", "int16", float64(32767)},
			{"Int32", "int32", float64(2147483647)},
			{"Float32", "float32", 3.14159},
			{"Float64", "float64", 3.141592653589793},
			{"Scientific", "scientific", 1.23e10},
			{"Negative", "negative", float64(-42)},
			{"Zero", "zero", float64(0)},
		}

		for _, tc := range numericCases {
			t.Run(tc.name, func(t *testing.T) {
				value, err := processor.Get(numericJSON, tc.path)
				helper.AssertNoError(err, "Should get numeric value: "+tc.name)
				helper.AssertEqual(tc.expected, value, "Numeric value should match: "+tc.name)
				helper.AssertTrue(reflect.TypeOf(value).Kind() == reflect.Float64, "Should be float64 type: "+tc.name)
			})
		}
	})

	t.Run("StringTypeHandling", func(t *testing.T) {
		processor := New()
		defer processor.Close()

		stringJSON := `{
			"empty": "",
			"simple": "hello",
			"unicode": "🚀🌟💫⭐",
			"escaped": "Hello\nWorld\t\"Test\"",
			"numeric": "123",
			"boolean": "true",
			"null": "null",
			"whitespace": "   spaces   ",
			"multiline": "Line1\nLine2\nLine3",
			"special": "!@#$%^&*()_+-=[]{}|;':\",./special"
		}`

		stringCases := []string{
			"empty", "simple", "unicode", "escaped", "numeric", 
			"boolean", "null", "whitespace", "multiline", "special",
		}

		for _, field := range stringCases {
			t.Run(field, func(t *testing.T) {
				value, err := processor.Get(stringJSON, field)
				helper.AssertNoError(err, "Should get string value: "+field)
				helper.AssertTrue(reflect.TypeOf(value).Kind() == reflect.String, "Should be string type: "+field)
				helper.AssertNotNil(value, "String value should not be nil: "+field)
			})
		}
	})

	t.Run("BooleanTypeHandling", func(t *testing.T) {
		processor := New()
		defer processor.Close()

		booleanJSON := `{
			"true": true,
			"false": false,
			"stringTrue": "true",
			"stringFalse": "false",
			"stringYes": "yes",
			"stringNo": "no",
			"number1": 1,
			"number0": 0,
			"numberPositive": 42,
			"numberNegative": -1
		}`

		// Test actual boolean values
		trueValue, err := processor.Get(booleanJSON, "true")
		helper.AssertNoError(err, "Should get true value")
		helper.AssertEqual(true, trueValue, "True value should match")
		helper.AssertTrue(reflect.TypeOf(trueValue).Kind() == reflect.Bool, "Should be bool type")

		falseValue, err := processor.Get(booleanJSON, "false")
		helper.AssertNoError(err, "Should get false value")
		helper.AssertEqual(false, falseValue, "False value should match")
		helper.AssertTrue(reflect.TypeOf(falseValue).Kind() == reflect.Bool, "Should be bool type")

		// Test string boolean values (should remain strings)
		stringTrue, err := processor.Get(booleanJSON, "stringTrue")
		helper.AssertNoError(err, "Should get string true value")
		helper.AssertEqual("true", stringTrue, "String true should remain string")
		helper.AssertTrue(reflect.TypeOf(stringTrue).Kind() == reflect.String, "Should be string type")

		// Test numeric values (should remain numbers)
		number1, err := processor.Get(booleanJSON, "number1")
		helper.AssertNoError(err, "Should get number 1")
		helper.AssertEqual(float64(1), number1, "Number 1 should remain number")
		helper.AssertTrue(reflect.TypeOf(number1).Kind() == reflect.Float64, "Should be float64 type")
	})

	t.Run("ArrayTypeHandling", func(t *testing.T) {
		processor := New()
		defer processor.Close()

		arrayJSON := `{
			"empty": [],
			"numbers": [1, 2, 3, 4, 5],
			"strings": ["a", "b", "c"],
			"mixed": [1, "two", true, null, {"key": "value"}],
			"nested": [[1, 2], [3, 4], [5, 6]],
			"objects": [
				{"id": 1, "name": "Alice"},
				{"id": 2, "name": "Bob"}
			]
		}`

		arrayCases := []struct {
			name         string
			path         string
			expectedLen  int
			elementCheck func(arr []any) bool
		}{
			{
				"Empty", "empty", 0,
				func(arr []any) bool { return len(arr) == 0 },
			},
			{
				"Numbers", "numbers", 5,
				func(arr []any) bool {
					return len(arr) == 5 && reflect.TypeOf(arr[0]).Kind() == reflect.Float64
				},
			},
			{
				"Strings", "strings", 3,
				func(arr []any) bool {
					return len(arr) == 3 && reflect.TypeOf(arr[0]).Kind() == reflect.String
				},
			},
			{
				"Mixed", "mixed", 5,
				func(arr []any) bool {
					return len(arr) == 5 && arr[3] == nil
				},
			},
			{
				"Nested", "nested", 3,
				func(arr []any) bool {
					return len(arr) == 3 && reflect.TypeOf(arr[0]).Kind() == reflect.Slice
				},
			},
			{
				"Objects", "objects", 2,
				func(arr []any) bool {
					return len(arr) == 2 && reflect.TypeOf(arr[0]).Kind() == reflect.Map
				},
			},
		}

		for _, tc := range arrayCases {
			t.Run(tc.name, func(t *testing.T) {
				value, err := processor.Get(arrayJSON, tc.path)
				helper.AssertNoError(err, "Should get array value: "+tc.name)
				helper.AssertTrue(reflect.TypeOf(value).Kind() == reflect.Slice, "Should be slice type: "+tc.name)

				arr, ok := value.([]any)
				helper.AssertTrue(ok, "Should be []any type: "+tc.name)
				helper.AssertEqual(tc.expectedLen, len(arr), "Array length should match: "+tc.name)
				helper.AssertTrue(tc.elementCheck(arr), "Element check should pass: "+tc.name)
			})
		}
	})

	t.Run("ObjectTypeHandling", func(t *testing.T) {
		processor := New()
		defer processor.Close()

		objectJSON := `{
			"empty": {},
			"simple": {"key": "value"},
			"nested": {
				"level1": {
					"level2": {
						"value": "deep"
					}
				}
			},
			"mixed": {
				"string": "text",
				"number": 42,
				"boolean": true,
				"null": null,
				"array": [1, 2, 3],
				"object": {"nested": "value"}
			}
		}`

		objectCases := []struct {
			name        string
			path        string
			expectedLen int
			keyCheck    func(obj map[string]any) bool
		}{
			{
				"Empty", "empty", 0,
				func(obj map[string]any) bool { return len(obj) == 0 },
			},
			{
				"Simple", "simple", 1,
				func(obj map[string]any) bool {
					return len(obj) == 1 && obj["key"] == "value"
				},
			},
			{
				"Nested", "nested", 1,
				func(obj map[string]any) bool {
					return len(obj) == 1 && reflect.TypeOf(obj["level1"]).Kind() == reflect.Map
				},
			},
			{
				"Mixed", "mixed", 6,
				func(obj map[string]any) bool {
					return len(obj) == 6 && 
						reflect.TypeOf(obj["string"]).Kind() == reflect.String &&
						reflect.TypeOf(obj["number"]).Kind() == reflect.Float64 &&
						reflect.TypeOf(obj["boolean"]).Kind() == reflect.Bool &&
						obj["null"] == nil &&
						reflect.TypeOf(obj["array"]).Kind() == reflect.Slice &&
						reflect.TypeOf(obj["object"]).Kind() == reflect.Map
				},
			},
		}

		for _, tc := range objectCases {
			t.Run(tc.name, func(t *testing.T) {
				value, err := processor.Get(objectJSON, tc.path)
				helper.AssertNoError(err, "Should get object value: "+tc.name)
				helper.AssertTrue(reflect.TypeOf(value).Kind() == reflect.Map, "Should be map type: "+tc.name)

				obj, ok := value.(map[string]any)
				helper.AssertTrue(ok, "Should be map[string]any type: "+tc.name)
				helper.AssertEqual(tc.expectedLen, len(obj), "Object length should match: "+tc.name)
				helper.AssertTrue(tc.keyCheck(obj), "Key check should pass: "+tc.name)
			})
		}
	})

	t.Run("TypePreservationInOperations", func(t *testing.T) {
		processor := New()
		defer processor.Close()

		originalJSON := `{
			"data": {
				"string": "original",
				"number": 100,
				"boolean": false,
				"array": [1, 2, 3],
				"object": {"key": "value"}
			}
		}`

		// Test that types are preserved during Set operations
		typePreservationCases := []struct {
			name      string
			path      string
			newValue  any
			checkType reflect.Kind
		}{
			{"StringUpdate", "data.string", "updated", reflect.String},
			{"NumberUpdate", "data.number", 200, reflect.Float64},
			{"BooleanUpdate", "data.boolean", true, reflect.Bool},
			{"ArrayUpdate", "data.array", []any{4, 5, 6}, reflect.Slice},
			{"ObjectUpdate", "data.object", map[string]any{"newKey": "newValue"}, reflect.Map},
		}

		currentJSON := originalJSON
		for _, tc := range typePreservationCases {
			t.Run(tc.name, func(t *testing.T) {
				// Update the value
				result, err := processor.Set(currentJSON, tc.path, tc.newValue)
				helper.AssertNoError(err, "Should update value: "+tc.name)
				currentJSON = result

				// Verify the type is preserved
				retrievedValue, err := processor.Get(result, tc.path)
				helper.AssertNoError(err, "Should get updated value: "+tc.name)
				
				actualType := reflect.TypeOf(retrievedValue).Kind()
				helper.AssertEqual(tc.checkType, actualType, "Type should be preserved: "+tc.name)
			})
		}
	})

	t.Run("TypeCoercionLimits", func(t *testing.T) {
		processor := New()
		defer processor.Close()

		// Test that the library doesn't perform unexpected type coercion
		testJSON := `{
			"stringNumber": "123",
			"stringBoolean": "true",
			"numberString": 456,
			"booleanString": true
		}`

		// These should remain as their original types, not be coerced
		stringNumber, err := processor.Get(testJSON, "stringNumber")
		helper.AssertNoError(err, "Should get string number")
		helper.AssertTrue(reflect.TypeOf(stringNumber).Kind() == reflect.String, "String number should remain string")
		helper.AssertEqual("123", stringNumber, "String number value should match")

		stringBoolean, err := processor.Get(testJSON, "stringBoolean")
		helper.AssertNoError(err, "Should get string boolean")
		helper.AssertTrue(reflect.TypeOf(stringBoolean).Kind() == reflect.String, "String boolean should remain string")
		helper.AssertEqual("true", stringBoolean, "String boolean value should match")

		numberString, err := processor.Get(testJSON, "numberString")
		helper.AssertNoError(err, "Should get number string")
		helper.AssertTrue(reflect.TypeOf(numberString).Kind() == reflect.Float64, "Number should remain number")
		helper.AssertEqual(float64(456), numberString, "Number value should match")

		booleanString, err := processor.Get(testJSON, "booleanString")
		helper.AssertNoError(err, "Should get boolean string")
		helper.AssertTrue(reflect.TypeOf(booleanString).Kind() == reflect.Bool, "Boolean should remain boolean")
		helper.AssertEqual(true, booleanString, "Boolean value should match")
	})

	t.Run("ComplexTypeOperations", func(t *testing.T) {
		processor := New()
		defer processor.Close()

		complexJSON := `{
			"users": [
				{
					"id": 1,
					"name": "Alice",
					"active": true,
					"metadata": {
						"lastLogin": "2024-01-01",
						"permissions": ["read", "write"],
						"settings": {
							"theme": "dark",
							"notifications": true
						}
					}
				},
				{
					"id": 2,
					"name": "Bob",
					"active": false,
					"metadata": null
				}
			]
		}`

		// Test complex nested type access
		userName, err := processor.Get(complexJSON, "users[0].name")
		helper.AssertNoError(err, "Should get nested string")
		helper.AssertTrue(reflect.TypeOf(userName).Kind() == reflect.String, "Should be string type")
		helper.AssertEqual("Alice", userName, "Name should match")

		userActive, err := processor.Get(complexJSON, "users[0].active")
		helper.AssertNoError(err, "Should get nested boolean")
		helper.AssertTrue(reflect.TypeOf(userActive).Kind() == reflect.Bool, "Should be bool type")
		helper.AssertEqual(true, userActive, "Active status should match")

		permissions, err := processor.Get(complexJSON, "users[0].metadata.permissions")
		helper.AssertNoError(err, "Should get nested array")
		helper.AssertTrue(reflect.TypeOf(permissions).Kind() == reflect.Slice, "Should be slice type")

		settings, err := processor.Get(complexJSON, "users[0].metadata.settings")
		helper.AssertNoError(err, "Should get nested object")
		helper.AssertTrue(reflect.TypeOf(settings).Kind() == reflect.Map, "Should be map type")

		nullMetadata, err := processor.Get(complexJSON, "users[1].metadata")
		helper.AssertNoError(err, "Should get null metadata")
		helper.AssertNil(nullMetadata, "Null metadata should be nil")
	})
}
