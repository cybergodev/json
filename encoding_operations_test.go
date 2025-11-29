package json

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// TestEncodingOperations provides comprehensive testing for JSON encoding, decoding, and validation
// Merged from: encoding_test.go and validation parts from other test files
func TestEncodingOperations(t *testing.T) {
	helper := NewTestHelper(t)

	t.Run("Encoding", func(t *testing.T) {
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

			age, err := GetInt(jsonStr, "age")
			helper.AssertNoError(err)
			helper.AssertEqual(25, age)
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
			helper.AssertTrue(len(bytes) > 0)

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

			bytes, err := processor.MarshalIndent(data, "", "  ")
			helper.AssertNoError(err)
			helper.AssertTrue(strings.Contains(string(bytes), "\n"))
		})

		t.Run("CompactEncoding", func(t *testing.T) {
			processor := New()
			defer processor.Close()

			data := map[string]any{
				"compact": true,
				"nested":  map[string]int{"value": 123},
			}

			compactConfig := NewCompactConfig()
			result, err := processor.EncodeWithConfig(data, compactConfig)
			helper.AssertNoError(err)
			helper.AssertFalse(strings.Contains(result, "\n"))
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
				},
				"metadata": map[string]any{
					"total":     1,
					"timestamp": time.Now().Format(time.RFC3339),
				},
			}

			jsonStr, err := Encode(complexData)
			helper.AssertNoError(err)

			firstUserName, err := GetString(jsonStr, "users[0].name")
			helper.AssertNoError(err)
			helper.AssertEqual("John Doe", firstUserName)

			theme, err := GetString(jsonStr, "users[0].profile.preferences.theme")
			helper.AssertNoError(err)
			helper.AssertEqual("dark", theme)
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

			stringVal, err := GetString(jsonStr, "string")
			helper.AssertNoError(err)
			helper.AssertEqual("hello", stringVal)

			intVal, err := GetInt(jsonStr, "integer")
			helper.AssertNoError(err)
			helper.AssertEqual(42, intVal)

			floatVal, err := GetFloat64(jsonStr, "float")
			helper.AssertNoError(err)
			helper.AssertEqual(3.14, floatVal)

			boolVal, err := GetBool(jsonStr, "boolean")
			helper.AssertNoError(err)
			helper.AssertTrue(boolVal)

			nullVal, err := Get(jsonStr, "null")
			helper.AssertNoError(err)
			helper.AssertNil(nullVal)
		})

		t.Run("EncodingWithOptions", func(t *testing.T) {
			processor := New()
			defer processor.Close()

			data := map[string]any{
				"html":    "<script>alert('test')</script>",
				"unicode": "测试",
				"number":  123.456,
			}

			config := DefaultEncodeConfig()
			config.EscapeHTML = true
			config.Indent = "  "

			result, err := processor.EncodeWithOptions(data, config)
			helper.AssertNoError(err)
			helper.AssertTrue(len(result) > 0)
		})

		t.Run("CustomEncoder", func(t *testing.T) {
			config := DefaultEncodeConfig()
			config.Indent = "    "
			encoder := NewCustomEncoder(config)
			defer encoder.Close()

			data := map[string]any{
				"custom": "encoding",
				"nested": map[string]int{"value": 42},
			}

			result, err := encoder.Encode(data)
			helper.AssertNoError(err)
			helper.AssertTrue(len(result) > 0)
		})

		t.Run("EncodingErrorHandling", func(t *testing.T) {
			processor := New()
			defer processor.Close()

			invalidData := make(chan int)
			_, err := Marshal(invalidData)
			helper.AssertError(err)

			result, err := processor.ToJsonString(nil)
			helper.AssertNoError(err)
			helper.AssertEqual("null", result)
		})

		t.Run("StructWithTags", func(t *testing.T) {
			processor := New()
			defer processor.Close()

			type TestStruct struct {
				PublicField  string `json:"public"`
				PrivateField string `json:"-"`
				RenamedField string `json:"renamed_field"`
				OmitEmpty    string `json:"omit_empty,omitempty"`
			}

			data := TestStruct{
				PublicField:  "visible",
				PrivateField: "hidden",
				RenamedField: "renamed",
				OmitEmpty:    "",
			}

			result, err := processor.EncodeWithTags(data, false)
			helper.AssertNoError(err)
			helper.AssertTrue(strings.Contains(result, "visible"))
			helper.AssertFalse(strings.Contains(result, "hidden"))
			helper.AssertTrue(strings.Contains(result, "renamed_field"))
			helper.AssertFalse(strings.Contains(result, "omit_empty"))
		})

		t.Run("ConcurrentEncoding", func(t *testing.T) {
			processor := New()
			defer processor.Close()

			concurrencyTester := NewConcurrencyTester(t, 5, 20)
			concurrencyTester.Run(func(workerID, iteration int) error {
				data := map[string]any{
					"worker":    workerID,
					"iteration": iteration,
				}
				_, err := processor.Marshal(data)
				return err
			})
		})
	})

	t.Run("Parsing", func(t *testing.T) {
		t.Run("BasicParsing", func(t *testing.T) {
			jsonStr := `{"name": "John", "age": 30, "active": true}`

			processor := New()
			defer processor.Close()

			var result any
			err := processor.Parse(jsonStr, &result)
			helper.AssertNoError(err)
			helper.AssertNotNil(result)

			if resultMap, ok := result.(map[string]any); ok {
				helper.AssertEqual("John", resultMap["name"])
				helper.AssertEqual(float64(30), resultMap["age"])
				helper.AssertEqual(true, resultMap["active"])
			}
		})

		t.Run("ArrayParsing", func(t *testing.T) {
			jsonStr := `[1, 2, 3, "four", true, null]`

			processor := New()
			defer processor.Close()

			var result any
			err := processor.Parse(jsonStr, &result)
			helper.AssertNoError(err)

			if resultArray, ok := result.([]any); ok {
				helper.AssertEqual(6, len(resultArray))
				helper.AssertEqual(float64(1), resultArray[0])
				helper.AssertEqual("four", resultArray[3])
				helper.AssertEqual(true, resultArray[4])
				helper.AssertNil(resultArray[5])
			}
		})

		t.Run("NestedStructureParsing", func(t *testing.T) {
			jsonStr := `{
				"users": [{"id": 1, "profile": {"name": "Alice"}}],
				"metadata": {"count": 1}
			}`

			processor := New()
			defer processor.Close()

			var result any
			err := processor.Parse(jsonStr, &result)
			helper.AssertNoError(err)

			if resultMap, ok := result.(map[string]any); ok {
				users, ok := resultMap["users"].([]any)
				helper.AssertTrue(ok)
				helper.AssertEqual(1, len(users))
			}
		})
	})

	t.Run("Validation", func(t *testing.T) {
		t.Run("ValidJSON", func(t *testing.T) {
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
		})

		t.Run("InvalidJSON", func(t *testing.T) {
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
	})

	t.Run("StandardLibraryCompatibility", func(t *testing.T) {
		t.Run("MarshalCompatibility", func(t *testing.T) {
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

		t.Run("UnmarshalCompatibility", func(t *testing.T) {
			jsonStr := `{"name": "Jane", "age": 28, "tags": ["dev", "go"]}`

			var ourResult any
			err := Unmarshal([]byte(jsonStr), &ourResult)
			helper.AssertNoError(err)

			var stdResult any
			err = json.Unmarshal([]byte(jsonStr), &stdResult)
			helper.AssertNoError(err)

			helper.AssertEqual(stdResult, ourResult)
		})
	})
}
