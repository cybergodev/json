package json

import (
	"strings"
	"testing"
)

// TestEncodingValidation consolidates encoding, parsing, validation, and formatting tests
// Replaces: encoding_validation_test.go, marshal_file_test.go
func TestEncodingValidation(t *testing.T) {
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

		// IsValidJson function
		helper.AssertTrue(IsValidJson(`{"valid": true}`))
		helper.AssertFalse(IsValidJson(`{invalid}`))
	})

	t.Run("FormattingOperations", func(t *testing.T) {
		data := map[string]any{"name": "test"}

		// MarshalIndent
		result, err := MarshalIndent(data, "", "  ")
		helper.AssertNoError(err)
		helper.AssertTrue(len(result) > 0)

		// EncodePretty
		prettyResult, err := EncodePretty(data)
		helper.AssertNoError(err)
		helper.AssertTrue(strings.Contains(prettyResult, "\n"))

		// EncodeCompact
		compactResult, err := EncodeCompact(data)
		helper.AssertNoError(err)
		helper.AssertFalse(strings.Contains(compactResult, "\n"))

		// FormatPretty
		jsonStr := `{"name":"test"}`
		prettyFormatted, err := FormatPretty(jsonStr)
		helper.AssertNoError(err)
		helper.AssertTrue(strings.Contains(prettyFormatted, "\n"))

		// FormatCompact
		compactFormatted, err := FormatCompact(jsonStr)
		helper.AssertNoError(err)
		helper.AssertFalse(strings.Contains(compactFormatted, "\n"))
	})

	t.Run("FileOperations", func(t *testing.T) {
		tempDir := t.TempDir()

		type TestUser struct {
			Name  string `json:"name"`
			Age   int    `json:"age"`
			Email string `json:"email"`
		}

		testUser := TestUser{Name: "John Doe", Age: 30, Email: "john@example.com"}

		// MarshalToFile
		compactFile := tempDir + "/user.json"
		err := MarshalToFile(compactFile, testUser)
		helper.AssertNoError(err)

		// UnmarshalFromFile
		var loadedUser TestUser
		err = UnmarshalFromFile(compactFile, &loadedUser)
		helper.AssertNoError(err)
		helper.AssertEqual(testUser.Name, loadedUser.Name)
	})
}
