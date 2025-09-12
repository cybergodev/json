package json

import (
	"strings"
	"testing"
)

func TestEncoderComprehensive(t *testing.T) {
	helper := NewTestHelper(t)
	processor := New()
	defer processor.Close()

	t.Run("BasicEncoding", func(t *testing.T) {
		// Test basic encoding functions
		data := map[string]interface{}{
			"string":  "test",
			"number":  123,
			"boolean": true,
			"null":    nil,
		}

		// Test ToJsonString
		result, err := processor.ToJsonString(data)
		helper.AssertNoError(err, "Should encode to JSON string")
		helper.AssertTrue(len(result) > 0, "Should produce JSON string")
		helper.AssertTrue(strings.Contains(result, "test"), "Should contain string value")

		// Test Marshal
		bytes, err := processor.Marshal(data)
		helper.AssertNoError(err, "Should marshal successfully")
		helper.AssertTrue(len(bytes) > 0, "Should produce bytes")

		// Test Unmarshal
		var unmarshaled map[string]interface{}
		err = Unmarshal(bytes, &unmarshaled)
		helper.AssertNoError(err, "Should unmarshal successfully")
		helper.AssertEqual("test", unmarshaled["string"], "Should preserve string value")
	})

	t.Run("PrettyEncoding", func(t *testing.T) {
		// Test pretty printing
		data := map[string]interface{}{
			"nested": map[string]interface{}{
				"array": []int{1, 2, 3},
				"value": "test",
			},
		}

		// Test ToJsonStringPretty
		pretty, err := processor.ToJsonStringPretty(data)
		helper.AssertNoError(err, "Should encode pretty JSON")
		helper.AssertTrue(strings.Contains(pretty, "\n"), "Should contain newlines")
		helper.AssertTrue(strings.Contains(pretty, "  "), "Should contain indentation")

		// Test MarshalIndent
		bytes, err := processor.MarshalIndent(data, "", "  ")
		helper.AssertNoError(err, "Should marshal with indent")
		result := string(bytes)
		helper.AssertTrue(strings.Contains(result, "\n"), "Should be formatted")
	})

	t.Run("StandardEncoding", func(t *testing.T) {
		// Test standard encoding
		data := map[string]interface{}{
			"test": "value",
		}

		result, err := processor.ToJsonStringStandard(data)
		helper.AssertNoError(err, "Should encode standard JSON")
		helper.AssertTrue(len(result) > 0, "Should produce standard JSON")
		helper.AssertFalse(strings.Contains(result, "\n"), "Should be compact")
	})

	t.Run("EncodingWithOptions", func(t *testing.T) {
		// Test encoding with custom options
		data := map[string]interface{}{
			"html":    "<script>alert('test')</script>",
			"unicode": "测试",
			"number":  123.456,
		}

		config := DefaultEncodeConfig()
		config.EscapeHTML = true
		config.Indent = "  "

		result, err := processor.EncodeWithOptions(data, config)
		helper.AssertNoError(err, "Should encode with options")
		helper.AssertTrue(len(result) > 0, "Should produce result")
	})

	t.Run("StreamEncoding", func(t *testing.T) {
		// Test stream encoding
		data := []map[string]interface{}{
			{"id": 1, "name": "first"},
			{"id": 2, "name": "second"},
			{"id": 3, "name": "third"},
		}

		result, err := processor.EncodeStream(data, false)
		helper.AssertNoError(err, "Should encode stream")
		helper.AssertTrue(len(result) > 0, "Should produce stream output")
	})

	t.Run("StreamEncodingWithOptions", func(t *testing.T) {
		// Test stream encoding with options
		data := map[string]interface{}{
			"formatted": true,
			"data":      []int{1, 2, 3},
		}

		config := DefaultEncodeConfig()
		config.Indent = "\t"

		result, err := processor.EncodeStreamWithOptions(data, config)
		helper.AssertNoError(err, "Should encode stream with options")
		helper.AssertTrue(len(result) > 0, "Should produce encoded output")
	})

	t.Run("BatchEncoding", func(t *testing.T) {
		// Test batch encoding
		items := map[string]interface{}{
			"user1": "alice",
			"user2": "bob",
			"count": 42,
		}

		result, err := processor.EncodeBatch(items, false)
		helper.AssertNoError(err, "Should encode batch")
		helper.AssertTrue(len(result) > 0, "Should encode batch result")
	})

	t.Run("FieldEncoding", func(t *testing.T) {
		// Test field-specific encoding
		data := map[string]interface{}{
			"public":    "visible",
			"private":   "hidden",
			"sensitive": "secret",
		}

		fields := []string{"public", "private"}
		result, err := processor.EncodeFields(data, fields, false)
		helper.AssertNoError(err, "Should encode specific fields")
		helper.AssertTrue(strings.Contains(result, "visible"), "Should include public field")
		helper.AssertTrue(strings.Contains(result, "hidden"), "Should include private field")
		helper.AssertFalse(strings.Contains(result, "secret"), "Should exclude sensitive field")
	})

	t.Run("EncodingWithConfig", func(t *testing.T) {
		// Test encoding with custom configuration
		data := map[string]interface{}{
			"number":  123.456789,
			"string":  "test value",
			"boolean": true,
			"array":   []int{1, 2, 3, 4, 5},
		}

		config := DefaultEncodeConfig()
		config.Pretty = true
		config.SortKeys = true
		config.EscapeHTML = false

		result, err := processor.EncodeWithConfig(data, config)
		helper.AssertNoError(err, "Should encode with config")
		helper.AssertTrue(len(result) > 0, "Should produce configured output")
	})

	t.Run("EncodingWithTags", func(t *testing.T) {
		// Test encoding with struct tags
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
		helper.AssertNoError(err, "Should encode with tags")
		helper.AssertTrue(strings.Contains(result, "visible"), "Should include public field")
		helper.AssertFalse(strings.Contains(result, "hidden"), "Should exclude private field")
		helper.AssertTrue(strings.Contains(result, "renamed_field"), "Should use renamed field")
		helper.AssertFalse(strings.Contains(result, "omit_empty"), "Should omit empty field")
	})

	t.Run("CustomEncoderUsage", func(t *testing.T) {
		// Test custom encoder
		config := DefaultEncodeConfig()
		config.Indent = "    "
		encoder := NewCustomEncoder(config)
		defer encoder.Close()

		data := map[string]interface{}{
			"custom": "encoding",
			"nested": map[string]int{"value": 42},
		}

		result, err := encoder.Encode(data)
		helper.AssertNoError(err, "Should encode with custom encoder")
		helper.AssertTrue(len(result) > 0, "Should produce encoded output")
	})

	t.Run("EncodingErrorHandling", func(t *testing.T) {
		// Test error conditions
		invalidData := make(chan int) // channels can't be encoded

		_, err := Marshal(invalidData)
		helper.AssertError(err, "Should handle invalid data")

		// Test encoding with nil
		result, err := processor.ToJsonString(nil)
		helper.AssertNoError(err, "Should encode nil")
		helper.AssertEqual("null", result, "Should encode nil as null")
	})

	t.Run("LargeDataEncoding", func(t *testing.T) {
		// Test encoding large data structures
		generator := NewTestDataGenerator()
		largeData := generator.GenerateComplexJSON()

		// Parse it back to interface{}
		var data interface{}
		err := Unmarshal([]byte(largeData), &data)
		helper.AssertNoError(err, "Should parse large data")

		// Re-encode it
		result, err := processor.Marshal(data)
		helper.AssertNoError(err, "Should encode large data")
		helper.AssertTrue(len(result) > 100, "Should produce substantial output")
	})

	t.Run("ConcurrentEncoding", func(t *testing.T) {
		// Test concurrent encoding
		concurrencyTester := NewConcurrencyTester(t, 5, 20)

		concurrencyTester.Run(func(workerID, iteration int) error {
			data := map[string]interface{}{
				"worker":    workerID,
				"iteration": iteration,
				"timestamp": "2024-01-01T00:00:00Z",
			}

			_, err := processor.Marshal(data)
			return err
		})
	})

	t.Run("MemoryEfficiency", func(t *testing.T) {
		// Test memory efficiency with repeated encoding
		data := map[string]interface{}{
			"test": "memory efficiency",
			"data": []int{1, 2, 3, 4, 5},
		}

		// Encode multiple times to test buffer reuse
		for i := 0; i < 100; i++ {
			result, err := processor.Marshal(data)
			helper.AssertNoError(err, "Should encode efficiently")
			helper.AssertTrue(len(result) > 0, "Should produce result")
		}
	})
}

func TestEncodingFormats(t *testing.T) {
	helper := NewTestHelper(t)
	processor := New()
	defer processor.Close()

	t.Run("CompactFormat", func(t *testing.T) {
		data := map[string]interface{}{
			"compact": true,
			"nested":  map[string]int{"value": 123},
		}

		compactConfig := NewCompactConfig()
		result, err := processor.EncodeWithConfig(data, compactConfig)
		helper.AssertNoError(err, "Should encode compact")
		helper.AssertFalse(strings.Contains(result, "\n"), "Should be compact")
		helper.AssertFalse(strings.Contains(result, "  "), "Should have no extra spaces")
	})

	t.Run("PrettyFormat", func(t *testing.T) {
		data := map[string]interface{}{
			"pretty": true,
			"nested": map[string]int{"value": 123},
		}

		prettyConfig := NewPrettyConfig()
		result, err := processor.EncodeWithConfig(data, prettyConfig)
		helper.AssertNoError(err, "Should encode pretty")
		helper.AssertTrue(strings.Contains(result, "\n"), "Should have newlines")
		helper.AssertTrue(strings.Contains(result, "  "), "Should have indentation")
	})

	t.Run("StandardFormat", func(t *testing.T) {
		data := map[string]interface{}{
			"standard": true,
		}

		result, err := processor.ToJsonString(data)
		helper.AssertNoError(err, "Should encode in standard format")
		helper.AssertTrue(len(result) > 0, "Should produce output")
	})
}
