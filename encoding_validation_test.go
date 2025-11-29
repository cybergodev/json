package json

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestEncodingAndValidation consolidates encoding, parsing, and validation tests
// Replaces: encoding_operations_test.go, validation_comprehensive_test.go, parser_edge_test.go
func TestEncodingAndValidation(t *testing.T) {
	helper := NewTestHelper(t)

	t.Run("BasicEncoding", func(t *testing.T) {
		data := map[string]any{
			"name":   "Alice",
			"age":    25,
			"active": true,
			"scores": []int{85, 92, 78},
		}

		jsonStr, err := Encode(data)
		helper.AssertNoError(err)
		helper.AssertTrue(len(jsonStr) > 0)

		name, err := GetString(jsonStr, "name")
		helper.AssertNoError(err)
		helper.AssertEqual("Alice", name)
	})

	t.Run("MarshalUnmarshal", func(t *testing.T) {
		processor := New()
		defer processor.Close()

		data := map[string]any{
			"string":  "test",
			"number":  123,
			"boolean": true,
			"null":    nil,
		}

		bytes, err := processor.Marshal(data)
		helper.AssertNoError(err)

		var unmarshaled map[string]any
		err = Unmarshal(bytes, &unmarshaled)
		helper.AssertNoError(err)
		helper.AssertEqual("test", unmarshaled["string"])
	})

	t.Run("PrettyEncoding", func(t *testing.T) {
		processor := New()
		defer processor.Close()

		data := map[string]any{
			"nested": map[string]any{
				"array": []int{1, 2, 3},
				"value": "test",
			},
		}

		pretty, err := processor.ToJsonStringPretty(data)
		helper.AssertNoError(err)
		helper.AssertTrue(strings.Contains(pretty, "\n"))
		helper.AssertTrue(strings.Contains(pretty, "  "))
	})

	t.Run("TypePreservation", func(t *testing.T) {
		data := map[string]any{
			"string":  "hello",
			"integer": 42,
			"float":   3.14,
			"boolean": true,
			"null":    nil,
			"array":   []any{1, "two", 3.0, false},
		}

		jsonStr, err := Encode(data)
		helper.AssertNoError(err)

		stringVal, _ := GetString(jsonStr, "string")
		helper.AssertEqual("hello", stringVal)

		intVal, _ := GetInt(jsonStr, "integer")
		helper.AssertEqual(42, intVal)

		floatVal, _ := GetFloat64(jsonStr, "float")
		helper.AssertEqual(3.14, floatVal)

		boolVal, _ := GetBool(jsonStr, "boolean")
		helper.AssertTrue(boolVal)

		nullVal, _ := Get(jsonStr, "null")
		helper.AssertNil(nullVal)
	})

	t.Run("JSONValidation", func(t *testing.T) {
		validJSONs := []string{
			`null`,
			`true`,
			`42`,
			`"string"`,
			`[]`,
			`{}`,
			`{"key": "value"}`,
			`[1, 2, 3]`,
		}

		for _, jsonStr := range validJSONs {
			valid := Valid([]byte(jsonStr))
			helper.AssertTrue(valid, "Should be valid: %s", jsonStr)
		}

		invalidJSONs := []string{
			`{`,
			`{"key": }`,
			`{key: "value"}`,
			`{"key": "value",}`,
			`[1, 2, 3,]`,
		}

		for _, jsonStr := range invalidJSONs {
			valid := Valid([]byte(jsonStr))
			helper.AssertFalse(valid, "Should be invalid: %s", jsonStr)
		}
	})

	t.Run("StandardLibraryCompatibility", func(t *testing.T) {
		testData := map[string]any{
			"name":   "John Doe",
			"age":    30,
			"active": true,
			"scores": []int{85, 92, 78},
		}

		ourResult, err := Marshal(testData)
		helper.AssertNoError(err)

		stdResult, err := json.Marshal(testData)
		helper.AssertNoError(err)

		helper.AssertTrue(Valid(ourResult))
		helper.AssertTrue(Valid(stdResult))

		var ourParsed, stdParsed any
		err = json.Unmarshal(ourResult, &ourParsed)
		helper.AssertNoError(err)
		err = json.Unmarshal(stdResult, &stdParsed)
		helper.AssertNoError(err)

		helper.AssertEqual(stdParsed, ourParsed)
	})

	t.Run("SchemaValidation", func(t *testing.T) {
		schema := DefaultSchema()
		schema.Type = "object"
		schema.Properties = map[string]*Schema{
			"name": {
				Type:      "string",
				MinLength: 1,
				MaxLength: 50,
			},
			"age": {
				Type:    "number",
				Minimum: 0,
				Maximum: 150,
			},
		}
		schema.Required = []string{"name"}

		validJSON := `{"name": "John Doe", "age": 30}`
		errors, err := ValidateSchema(validJSON, schema)
		helper.AssertNoError(err)
		helper.AssertEqual(0, len(errors))

		invalidJSON := `{"age": 30}`
		errors, err = ValidateSchema(invalidJSON, schema)
		helper.AssertNoError(err)
		helper.AssertTrue(len(errors) > 0)
	})

	t.Run("UnicodeHandling", func(t *testing.T) {
		testData := `{
			"emoji": "😀🎉🚀",
			"chinese": "你好世界",
			"mixed": "Hello 世界 🌍"
		}`

		emoji, err := GetString(testData, "emoji")
		helper.AssertNoError(err)
		helper.AssertEqual("😀🎉🚀", emoji)

		chinese, err := GetString(testData, "chinese")
		helper.AssertNoError(err)
		helper.AssertEqual("你好世界", chinese)

		mixed, err := GetString(testData, "mixed")
		helper.AssertNoError(err)
		helper.AssertEqual("Hello 世界 🌍", mixed)
	})

	t.Run("NumberEdgeCases", func(t *testing.T) {
		testData := `{
			"zero": 0,
			"negative": -42,
			"float": 3.14159,
			"scientific": 1.23e10,
			"largeInt": 9007199254740991
		}`

		zero, err := GetInt(testData, "zero")
		helper.AssertNoError(err)
		helper.AssertEqual(0, zero)

		negative, err := GetInt(testData, "negative")
		helper.AssertNoError(err)
		helper.AssertEqual(-42, negative)

		scientific, err := Get(testData, "scientific")
		helper.AssertNoError(err)
		helper.AssertNotNil(scientific)
	})

	t.Run("WhitespaceHandling", func(t *testing.T) {
		testCases := []string{
			`{"key":"value"}`,
			`{ "key" : "value" }`,
			`{  "key"  :  "value"  }`,
			"{\n\t\"key\": \"value\"\n}",
		}

		for _, testCase := range testCases {
			value, err := GetString(testCase, "key")
			helper.AssertNoError(err)
			helper.AssertEqual("value", value)
		}
	})
}
