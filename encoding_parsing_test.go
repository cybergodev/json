package json

import (
	"encoding/json"
	"testing"
	"time"
)

// TestEncodingParsing tests JSON encoding and parsing functionality
// Merged from: parser_encoder_test.go, encoder_extended_test.go, custom_encoder_comprehensive_test.go
func TestEncodingParsing(t *testing.T) {
	helper := NewTestHelper(t)

	t.Run("BasicEncoding", func(t *testing.T) {
		data := map[string]any{
			"name":   "Alice",
			"age":    25,
			"active": true,
			"scores": []int{85, 92, 78},
		}

		jsonStr, err := Encode(data)
		helper.AssertNoError(err, "Encode should work")
		helper.AssertTrue(len(jsonStr) > 0, "Encoded JSON should not be empty")

		// Verify we can parse it back
		name, err := GetString(jsonStr, "name")
		helper.AssertNoError(err, "Should get name from encoded JSON")
		helper.AssertEqual("Alice", name, "Name should match")

		age, err := GetInt(jsonStr, "age")
		helper.AssertNoError(err, "Should get age from encoded JSON")
		helper.AssertEqual(25, age, "Age should match")
	})

	t.Run("ComplexDataEncoding", func(t *testing.T) {
		complexData := map[string]any{
			"users": []map[string]any{
				{
					"id":   1,
					"name": "John Doe",
					"profile": map[string]any{
						"email":    "john@example.com",
						"location": "New York",
						"preferences": map[string]any{
							"theme":         "dark",
							"notifications": true,
							"languages":     []string{"en", "es"},
						},
					},
				},
				{
					"id":   2,
					"name": "Jane Smith",
					"profile": map[string]any{
						"email":    "jane@example.com",
						"location": "San Francisco",
						"preferences": map[string]any{
							"theme":         "light",
							"notifications": false,
							"languages":     []string{"en", "fr"},
						},
					},
				},
			},
			"metadata": map[string]any{
				"total":     2,
				"timestamp": time.Now().Format(time.RFC3339),
				"version":   "1.0",
			},
		}

		jsonStr, err := Encode(complexData)
		helper.AssertNoError(err, "Complex encoding should work")

		// Verify nested access works
		firstUserName, err := GetString(jsonStr, "users[0].name")
		helper.AssertNoError(err, "Should get first user name")
		helper.AssertEqual("John Doe", firstUserName, "First user name should match")

		// Verify deep nested access
		theme, err := GetString(jsonStr, "users[0].profile.preferences.theme")
		helper.AssertNoError(err, "Should get theme preference")
		helper.AssertEqual("dark", theme, "Theme should match")

		// Verify array in nested structure
		languages, err := GetArray(jsonStr, "users[0].profile.preferences.languages")
		helper.AssertNoError(err, "Should get languages array")
		helper.AssertEqual(2, len(languages), "Should have 2 languages")
	})

	t.Run("TypePreservation", func(t *testing.T) {
		data := map[string]any{
			"string":  "hello",
			"integer": 42,
			"float":   3.14,
			"boolean": true,
			"null":    nil,
			"array":   []any{1, "two", 3.0, false},
			"object":  map[string]any{"nested": "value"},
		}

		jsonStr, err := Encode(data)
		helper.AssertNoError(err, "Type preservation encoding should work")

		// Verify types are preserved
		stringVal, err := GetString(jsonStr, "string")
		helper.AssertNoError(err, "Should get string value")
		helper.AssertEqual("hello", stringVal, "String should match")

		intVal, err := GetInt(jsonStr, "integer")
		helper.AssertNoError(err, "Should get integer value")
		helper.AssertEqual(42, intVal, "Integer should match")

		floatVal, err := GetFloat64(jsonStr, "float")
		helper.AssertNoError(err, "Should get float value")
		helper.AssertEqual(3.14, floatVal, "Float should match")

		boolVal, err := GetBool(jsonStr, "boolean")
		helper.AssertNoError(err, "Should get boolean value")
		helper.AssertTrue(boolVal, "Boolean should be true")

		nullVal, err := Get(jsonStr, "null")
		helper.AssertNoError(err, "Should get null value")
		helper.AssertNil(nullVal, "Null should be nil")
	})

	t.Run("ArrayEncoding", func(t *testing.T) {
		arrays := map[string]any{
			"numbers":    []int{1, 2, 3, 4, 5},
			"strings":    []string{"a", "b", "c"},
			"mixed":      []any{1, "two", 3.0, true, nil},
			"nested":     [][]int{{1, 2}, {3, 4}, {5, 6}},
			"objects":    []map[string]any{{"id": 1, "name": "first"}, {"id": 2, "name": "second"}},
		}

		jsonStr, err := Encode(arrays)
		helper.AssertNoError(err, "Array encoding should work")

		// Verify array access
		numbers, err := GetArray(jsonStr, "numbers")
		helper.AssertNoError(err, "Should get numbers array")
		helper.AssertEqual(5, len(numbers), "Numbers array should have 5 elements")

		// Verify nested array access
		nestedFirst, err := GetArray(jsonStr, "nested[0]")
		helper.AssertNoError(err, "Should get first nested array")
		helper.AssertEqual(2, len(nestedFirst), "First nested array should have 2 elements")

		// Verify object in array
		firstName, err := GetString(jsonStr, "objects[0].name")
		helper.AssertNoError(err, "Should get first object name")
		helper.AssertEqual("first", firstName, "First object name should match")
	})
}

// TestJSONParsing tests JSON parsing functionality
func TestJSONParsing(t *testing.T) {
	helper := NewTestHelper(t)

	t.Run("BasicParsing", func(t *testing.T) {
		jsonStr := `{
			"name": "John",
			"age": 30,
			"active": true,
			"scores": [85, 92, 78]
		}`

		processor := New()
		defer processor.Close()

		var result any
		err := processor.Parse(jsonStr, &result)
		helper.AssertNoError(err, "Basic parsing should work")
		helper.AssertNotNil(result, "Parsed result should not be nil")

		// Verify we can access the parsed data
		if resultMap, ok := result.(map[string]any); ok {
			helper.AssertEqual("John", resultMap["name"], "Name should match")
			helper.AssertEqual(float64(30), resultMap["age"], "Age should match")
			helper.AssertEqual(true, resultMap["active"], "Active should match")
		} else {
			t.Errorf("Expected map[string]any, got %T", result)
		}
	})

	t.Run("ArrayParsing", func(t *testing.T) {
		jsonStr := `[1, 2, 3, "four", true, null]`

		processor := New()
		defer processor.Close()

		var result any
		err := processor.Parse(jsonStr, &result)
		helper.AssertNoError(err, "Array parsing should work")

		if resultArray, ok := result.([]any); ok {
			helper.AssertEqual(6, len(resultArray), "Array should have 6 elements")
			helper.AssertEqual(float64(1), resultArray[0], "First element should be 1")
			helper.AssertEqual("four", resultArray[3], "Fourth element should be 'four'")
			helper.AssertEqual(true, resultArray[4], "Fifth element should be true")
			helper.AssertNil(resultArray[5], "Sixth element should be nil")
		} else {
			t.Errorf("Expected []any, got %T", result)
		}
	})

	t.Run("NestedStructureParsing", func(t *testing.T) {
		jsonStr := `{
			"users": [
				{
					"id": 1,
					"profile": {
						"name": "Alice",
						"settings": {
							"theme": "dark",
							"notifications": true
						}
					}
				}
			],
			"metadata": {
				"count": 1,
				"version": "1.0"
			}
		}`

		processor := New()
		defer processor.Close()

		var result any
		err := processor.Parse(jsonStr, &result)
		helper.AssertNoError(err, "Nested structure parsing should work")

		// Verify nested access through parsed data
		if resultMap, ok := result.(map[string]any); ok {
			users, ok := resultMap["users"].([]any)
			helper.AssertTrue(ok, "Users should be an array")
			helper.AssertEqual(1, len(users), "Should have 1 user")

			firstUser, ok := users[0].(map[string]any)
			helper.AssertTrue(ok, "First user should be a map")

			profile, ok := firstUser["profile"].(map[string]any)
			helper.AssertTrue(ok, "Profile should be a map")
			helper.AssertEqual("Alice", profile["name"], "Name should match")
		}
	})
}

// TestJSONValidation tests JSON validation functionality
func TestJSONValidation(t *testing.T) {
	helper := NewTestHelper(t)

	t.Run("ValidJSONStrings", func(t *testing.T) {
		validJSONs := []string{
			`null`,
			`true`,
			`false`,
			`42`,
			`3.14`,
			`"string"`,
			`""`,
			`[]`,
			`{}`,
			`{"key": "value"}`,
			`[1, 2, 3]`,
			`{"array": [1, 2, 3], "object": {"nested": true}}`,
		}

		for i, jsonStr := range validJSONs {
			valid := Valid([]byte(jsonStr))
			helper.AssertTrue(valid, "JSON %d should be valid: %s", i, jsonStr)
		}
	})

	t.Run("InvalidJSONStrings", func(t *testing.T) {
		invalidJSONs := []string{
			`{`,
			`}`,
			`{"key": }`,
			`{key: "value"}`,
			`{"key": "value",}`,
			`[1, 2, 3,]`,
			`{"unclosed": "string}`,
			`{'single': 'quotes'}`,
			`{duplicate: "key", duplicate: "value"}`,
		}

		for i, jsonStr := range invalidJSONs {
			valid := Valid([]byte(jsonStr))
			helper.AssertFalse(valid, "JSON %d should be invalid: %s", i, jsonStr)
		}
	})
}

// TestCompatibilityWithStandardLibrary tests compatibility with encoding/json
func TestCompatibilityWithStandardLibrary(t *testing.T) {
	helper := NewTestHelper(t)

	t.Run("MarshalCompatibility", func(t *testing.T) {
		testData := map[string]any{
			"name":   "John Doe",
			"age":    30,
			"active": true,
			"scores": []int{85, 92, 78, 96, 88},
			"profile": map[string]any{
				"email":    "john@example.com",
				"location": "New York",
			},
		}

		// Our marshal
		ourResult, err := Marshal(testData)
		helper.AssertNoError(err, "Our Marshal should work")

		// Standard library marshal
		stdResult, err := json.Marshal(testData)
		helper.AssertNoError(err, "Standard Marshal should work")

		// Both should produce valid JSON
		helper.AssertTrue(Valid(ourResult), "Our result should be valid JSON")
		helper.AssertTrue(Valid(stdResult), "Standard result should be valid JSON")

		// Both should be parseable and produce equivalent data
		var ourParsed, stdParsed any
		err = json.Unmarshal(ourResult, &ourParsed)
		helper.AssertNoError(err, "Our result should be parseable by standard library")

		err = json.Unmarshal(stdResult, &stdParsed)
		helper.AssertNoError(err, "Standard result should be parseable")

		// The data should be equivalent (though order might differ)
		helper.AssertEqual(stdParsed, ourParsed, "Parsed data should be equivalent")
	})

	t.Run("UnmarshalCompatibility", func(t *testing.T) {
		jsonStr := `{
			"name": "Jane Smith",
			"age": 28,
			"active": false,
			"tags": ["developer", "golang", "json"],
			"metadata": {
				"created": "2024-01-01",
				"updated": "2024-01-15"
			}
		}`

		// Our unmarshal
		var ourResult any
		err := Unmarshal([]byte(jsonStr), &ourResult)
		helper.AssertNoError(err, "Our Unmarshal should work")

		// Standard library unmarshal
		var stdResult any
		err = json.Unmarshal([]byte(jsonStr), &stdResult)
		helper.AssertNoError(err, "Standard Unmarshal should work")

		// Results should be equivalent
		helper.AssertEqual(stdResult, ourResult, "Unmarshal results should be equivalent")
	})
}
