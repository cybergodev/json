package json

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFileOperationsComprehensive(t *testing.T) {
	helper := NewTestHelper(t)
	processor := New()
	defer processor.Close()

	// Create temporary directory for tests
	tempDir, err := os.MkdirTemp("", "json_test_")
	helper.AssertNoError(err, "Should create temp directory")
	defer os.RemoveAll(tempDir)

	t.Run("LoadFromFile", func(t *testing.T) {
		// Create test file
		testData := map[string]interface{}{
			"name":  "test",
			"value": 123,
			"items": []int{1, 2, 3},
		}

		testFile := filepath.Join(tempDir, "test.json")
		jsonBytes, err := processor.Marshal(testData)
		helper.AssertNoError(err, "Should marshal test data")

		err = os.WriteFile(testFile, jsonBytes, 0644)
		helper.AssertNoError(err, "Should write test file")

		// Test loading from file
		loadedData, err := processor.LoadFromFile(testFile)
		helper.AssertNoError(err, "Should load from file")
		helper.AssertNotNil(loadedData, "Should load data")

		// Verify loaded data
		loadedMap := loadedData.(map[string]interface{})
		helper.AssertEqual("test", loadedMap["name"], "Should load name correctly")
		helper.AssertEqual(float64(123), loadedMap["value"], "Should load value correctly")
	})

	t.Run("LoadFromFileWithOptions", func(t *testing.T) {
		// Create test file with comments (if supported)
		testContent := `{
			"name": "test",
			"value": 123
		}`

		testFile := filepath.Join(tempDir, "test_options.json")
		err := os.WriteFile(testFile, []byte(testContent), 0644)
		helper.AssertNoError(err, "Should write test file")

		// Test loading with options
		options := DefaultOptions()
		loadedData, err := processor.LoadFromFile(testFile, options)
		helper.AssertNoError(err, "Should load with options")
		helper.AssertNotNil(loadedData, "Should load data with options")
	})

	t.Run("SaveToFile", func(t *testing.T) {
		// Test saving to file
		testData := map[string]interface{}{
			"saved":     true,
			"timestamp": "2024-01-01T00:00:00Z",
			"data": map[string]int{
				"count": 42,
			},
		}

		saveFile := filepath.Join(tempDir, "saved.json")
		err := processor.SaveToFile(saveFile, testData)
		helper.AssertNoError(err, "Should save to file")

		// Verify file exists and has content
		_, err = os.Stat(saveFile)
		helper.AssertNoError(err, "Saved file should exist")

		// Load and verify content
		content, err := os.ReadFile(saveFile)
		helper.AssertNoError(err, "Should read saved file")
		helper.AssertTrue(len(content) > 0, "Saved file should have content")

		// Parse and verify
		var loaded map[string]interface{}
		err = Unmarshal(content, &loaded)
		helper.AssertNoError(err, "Should parse saved content")
		helper.AssertEqual(true, loaded["saved"], "Should preserve boolean value")
	})

	t.Run("SaveToFileWithOptions", func(t *testing.T) {
		// Test saving with options
		testData := map[string]interface{}{
			"formatted": true,
			"nested": map[string]interface{}{
				"deep": "value",
			},
		}

		saveFile := filepath.Join(tempDir, "formatted.json")

		err := processor.SaveToFile(saveFile, testData, true)
		helper.AssertNoError(err, "Should save with options")

		// Verify file content
		content, err := os.ReadFile(saveFile)
		helper.AssertNoError(err, "Should read formatted file")
		helper.AssertTrue(len(content) > 0, "Should have formatted content")
	})

	t.Run("LoadFromReader", func(t *testing.T) {
		// Test loading from reader
		jsonData := `{"reader": "test", "stream": true, "count": 5}`
		reader := strings.NewReader(jsonData)

		loadedData, err := processor.LoadFromReader(reader)
		helper.AssertNoError(err, "Should load from reader")
		helper.AssertNotNil(loadedData, "Should load data from reader")

		// Verify loaded data
		loadedMap := loadedData.(map[string]interface{})
		helper.AssertEqual("test", loadedMap["reader"], "Should load from reader correctly")
		helper.AssertEqual(true, loadedMap["stream"], "Should preserve boolean from reader")
		helper.AssertEqual(float64(5), loadedMap["count"], "Should preserve number from reader")
	})

	t.Run("LoadFromReaderWithOptions", func(t *testing.T) {
		// Test loading from reader with options
		jsonData := `{"options": "enabled", "number": 12345678901234567890}`
		reader := strings.NewReader(jsonData)

		options := DefaultOptions()
		options.PreserveNumbers = true

		loadedData, err := processor.LoadFromReader(reader, options)
		helper.AssertNoError(err, "Should load from reader with options")
		helper.AssertNotNil(loadedData, "Should load data with options")
	})

	t.Run("SaveToWriter", func(t *testing.T) {
		// Test saving to writer
		testData := map[string]interface{}{
			"writer": "test",
			"output": "stream",
			"items":  []string{"a", "b", "c"},
		}

		var buf bytes.Buffer
		err := processor.SaveToWriter(&buf, testData, false)
		helper.AssertNoError(err, "Should save to writer")

		// Verify output
		output := buf.String()
		helper.AssertTrue(len(output) > 0, "Should produce output")
		helper.AssertTrue(strings.Contains(output, "writer"), "Should contain data")
		helper.AssertTrue(strings.Contains(output, "test"), "Should contain values")
	})

	t.Run("SaveToWriterWithOptions", func(t *testing.T) {
		// Test saving to writer with options
		testData := map[string]interface{}{
			"formatted": map[string]interface{}{
				"pretty": true,
				"indent": 2,
			},
		}

		var buf bytes.Buffer

		err := processor.SaveToWriter(&buf, testData, true)
		helper.AssertNoError(err, "Should save to writer with options")

		output := buf.String()
		helper.AssertTrue(len(output) > 0, "Should produce formatted output")
	})

	t.Run("DirectoryCreation", func(t *testing.T) {
		// Test automatic directory creation
		nestedDir := filepath.Join(tempDir, "nested", "deep", "path")
		testFile := filepath.Join(nestedDir, "auto_created.json")

		testData := map[string]string{
			"auto": "created",
		}

		err := processor.SaveToFile(testFile, testData)
		helper.AssertNoError(err, "Should create directories and save file")

		// Verify directory was created
		_, err = os.Stat(nestedDir)
		helper.AssertNoError(err, "Nested directory should be created")

		// Verify file was saved
		_, err = os.Stat(testFile)
		helper.AssertNoError(err, "File should be saved in created directory")
	})

	t.Run("FilePathValidation", func(t *testing.T) {
		// Test file path validation
		validPaths := []string{
			filepath.Join(tempDir, "valid.json"),
			filepath.Join(tempDir, "sub", "valid.json"),
			filepath.Join(tempDir, "file-with-dashes.json"),
			filepath.Join(tempDir, "file_with_underscores.json"),
		}

		for _, path := range validPaths {
			testData := map[string]string{"path": path}
			err := processor.SaveToFile(path, testData)
			helper.AssertNoError(err, "Should accept valid path: %s", path)
		}

		// Test invalid paths (these should be handled gracefully)
		invalidPaths := []string{
			"",                         // empty path
			"/root/no_permission.json", // likely no permission
		}

		for _, path := range invalidPaths {
			testData := map[string]string{"path": path}
			err := processor.SaveToFile(path, testData)
			if path == "" {
				helper.AssertError(err, "Should reject empty path")
			}
			// For permission errors, we just check it doesn't panic
		}
	})

	t.Run("LargeFileHandling", func(t *testing.T) {
		// Test handling of large files
		generator := NewTestDataGenerator()
		largeData := generator.GenerateComplexJSON()

		// Parse to interface{}
		var data interface{}
		err := Unmarshal([]byte(largeData), &data)
		helper.AssertNoError(err, "Should parse large data")

		// Save large file
		largeFile := filepath.Join(tempDir, "large.json")
		err = processor.SaveToFile(largeFile, data)
		helper.AssertNoError(err, "Should save large file")

		// Load large file
		loadedData, err := processor.LoadFromFile(largeFile)
		helper.AssertNoError(err, "Should load large file")
		helper.AssertNotNil(loadedData, "Should load large data")
	})

	t.Run("ConcurrentFileOperations", func(t *testing.T) {
		// Test concurrent file operations
		concurrencyTester := NewConcurrencyTester(t, 5, 10)

		concurrencyTester.Run(func(workerID, iteration int) error {
			// Each worker saves and loads its own file
			fileName := filepath.Join(tempDir, "concurrent_"+string(rune('0'+workerID))+"_"+string(rune('0'+iteration%10))+".json")

			testData := map[string]interface{}{
				"worker":    workerID,
				"iteration": iteration,
				"data":      []int{workerID, iteration},
			}

			// Save file
			err := processor.SaveToFile(fileName, testData)
			if err != nil {
				return err
			}

			// Load file
			_, err = processor.LoadFromFile(fileName)
			return err
		})
	})

	t.Run("ErrorHandling", func(t *testing.T) {
		// Test error conditions

		// Test loading non-existent file
		_, err := processor.LoadFromFile(filepath.Join(tempDir, "nonexistent.json"))
		helper.AssertError(err, "Should error on non-existent file")

		// Test loading invalid JSON file
		invalidFile := filepath.Join(tempDir, "invalid.json")
		err = os.WriteFile(invalidFile, []byte(`{"invalid": json}`), 0644)
		helper.AssertNoError(err, "Should write invalid JSON file")

		_, err = processor.LoadFromFile(invalidFile)
		helper.AssertError(err, "Should error on invalid JSON")

		// Test saving invalid data
		invalidData := make(chan int) // channels can't be marshaled
		err = processor.SaveToFile(filepath.Join(tempDir, "invalid_data.json"), invalidData)
		helper.AssertError(err, "Should error on invalid data")
	})

	t.Run("ReaderWriterEdgeCases", func(t *testing.T) {
		// Test edge cases with readers and writers

		// Test empty reader
		emptyReader := strings.NewReader("")
		_, err := processor.LoadFromReader(emptyReader)
		helper.AssertError(err, "Should error on empty reader")

		// Test invalid data (should be handled gracefully)
		invalidData := make(chan int) // channels can't be encoded
		var buf bytes.Buffer
		err = processor.SaveToWriter(&buf, invalidData, false)
		helper.AssertError(err, "Should error on invalid data")

		// Test reader with only whitespace
		whitespaceReader := strings.NewReader("   \n\t  ")
		_, err = processor.LoadFromReader(whitespaceReader)
		helper.AssertError(err, "Should error on whitespace-only reader")
	})

	t.Run("FilePermissions", func(t *testing.T) {
		// Test file permission handling
		testData := map[string]string{"permissions": "test"}

		// Save file with specific permissions
		permFile := filepath.Join(tempDir, "permissions.json")
		err := processor.SaveToFile(permFile, testData)
		helper.AssertNoError(err, "Should save file")

		// Check file exists and is readable
		info, err := os.Stat(permFile)
		helper.AssertNoError(err, "Should stat saved file")
		helper.AssertTrue(info.Size() > 0, "File should have content")

		// Load the file to verify it's readable
		_, err = processor.LoadFromFile(permFile)
		helper.AssertNoError(err, "Should load file with permissions")
	})
}
