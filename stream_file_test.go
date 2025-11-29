package json

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestStreamAndFile consolidates stream processing and file operations
// Replaces: stream_processing_comprehensive_test.go, file_operations_comprehensive_test.go, file_path_validation_test.go
func TestStreamAndFile(t *testing.T) {
	helper := NewTestHelper(t)

	t.Run("StreamDecoder", func(t *testing.T) {
		jsonData := `{"name": "test", "value": 123, "active": true}`
		reader := strings.NewReader(jsonData)
		decoder := NewDecoder(reader)

		var result map[string]interface{}
		err := decoder.Decode(&result)
		helper.AssertNoError(err)
		helper.AssertEqual("test", result["name"])
		helper.AssertEqual(float64(123), result["value"])
		helper.AssertEqual(true, result["active"])
	})

	t.Run("StreamDecoderWithNumbers", func(t *testing.T) {
		jsonData := `{"bigNumber": 12345678901234567890, "decimal": 123.456}`
		reader := strings.NewReader(jsonData)
		decoder := NewDecoder(reader)
		decoder.UseNumber()

		var result map[string]interface{}
		err := decoder.Decode(&result)
		helper.AssertNoError(err)
		helper.AssertNotNil(result["bigNumber"])
		helper.AssertNotNil(result["decimal"])
	})

	t.Run("StreamEncoder", func(t *testing.T) {
		var buf bytes.Buffer
		encoder := NewEncoder(&buf)

		data := map[string]interface{}{
			"name":  "test",
			"value": 123,
		}

		err := encoder.Encode(data)
		helper.AssertNoError(err)
		helper.AssertTrue(buf.Len() > 0)
	})

	t.Run("StreamToken", func(t *testing.T) {
		jsonData := `{"array": [1, 2, 3], "object": {"nested": true}}`
		reader := strings.NewReader(jsonData)
		decoder := NewDecoder(reader)

		token, err := decoder.Token()
		helper.AssertNoError(err)
		helper.AssertNotNil(token)

		hasMore := decoder.More()
		helper.AssertTrue(hasMore)
	})

	t.Run("FileOperations", func(t *testing.T) {
		processor := New()
		defer processor.Close()

		tempDir, err := os.MkdirTemp("", "json_test_")
		helper.AssertNoError(err)
		defer os.RemoveAll(tempDir)

		// Test save
		testData := map[string]interface{}{
			"name":  "test",
			"value": 123,
			"items": []int{1, 2, 3},
		}

		testFile := filepath.Join(tempDir, "test.json")
		err = processor.SaveToFile(testFile, testData)
		helper.AssertNoError(err)

		// Test load
		loadedData, err := processor.LoadFromFile(testFile)
		helper.AssertNoError(err)
		helper.AssertNotNil(loadedData)

		loadedMap := loadedData.(map[string]interface{})
		helper.AssertEqual("test", loadedMap["name"])
		helper.AssertEqual(float64(123), loadedMap["value"])
	})

	t.Run("FilePathValidation", func(t *testing.T) {
		processor := New()
		defer processor.Close()

		validPaths := []string{
			"dev_test/file.json",
			"config/production.json",
			"test.json",
		}

		for _, path := range validPaths {
			err := processor.validateFilePath(path)
			if err != nil {
				t.Logf("Path validation for '%s': %v", path, err)
			}
		}
	})

	t.Run("LoadWithOptions", func(t *testing.T) {
		processor := New()
		defer processor.Close()

		tempDir, err := os.MkdirTemp("", "json_test_")
		helper.AssertNoError(err)
		defer os.RemoveAll(tempDir)

		testContent := `{"name": "test", "value": 123}`
		testFile := filepath.Join(tempDir, "test_options.json")
		err = os.WriteFile(testFile, []byte(testContent), 0644)
		helper.AssertNoError(err)

		options := DefaultOptions()
		loadedData, err := processor.LoadFromFile(testFile, options)
		helper.AssertNoError(err)
		helper.AssertNotNil(loadedData)
	})

	t.Run("BufferedReading", func(t *testing.T) {
		jsonData := `{"test": "value"}{"another": "object"}`
		reader := strings.NewReader(jsonData)
		decoder := NewDecoder(reader)

		var first map[string]interface{}
		err := decoder.Decode(&first)
		helper.AssertNoError(err)

		buffered := decoder.Buffered()
		helper.AssertNotNil(buffered)

		var second map[string]interface{}
		err = decoder.Decode(&second)
		helper.AssertNoError(err)
	})
}
