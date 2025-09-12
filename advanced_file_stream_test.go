package json

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// TestAdvancedFileStreamOperations tests comprehensive file and stream processing functionality
func TestAdvancedFileStreamOperations(t *testing.T) {
	helper := NewTestHelper(t)

	// Create temporary directory for test files
	tempDir, err := os.MkdirTemp("", "json_test_*")
	helper.AssertNoError(err, "Should create temp directory")
	defer os.RemoveAll(tempDir)

	t.Run("BasicFileOperations", func(t *testing.T) {
		processor := New()
		defer processor.Close()

		testData := `{
			"users": [
				{"id": 1, "name": "Alice", "email": "alice@example.com"},
				{"id": 2, "name": "Bob", "email": "bob@example.com"},
				{"id": 3, "name": "Charlie", "email": "charlie@example.com"}
			],
			"metadata": {
				"total": 3,
				"created": "2024-01-01T00:00:00Z"
			}
		}`

		// Test saving to file
		testFile := filepath.Join(tempDir, "test_data.json")
		err := SaveToFile(testFile, testData, true)
		helper.AssertNoError(err, "Should save to file")

		// Verify file exists
		_, err = os.Stat(testFile)
		helper.AssertNoError(err, "File should exist")

		// Test loading from file
		loadedDataStr, err := LoadFromFile(testFile)
		helper.AssertNoError(err, "Should load from file")
		helper.AssertNotNil(loadedDataStr, "Loaded data should not be nil")

		// Test operations on loaded data
		userName, err := processor.Get(loadedDataStr, "users[0].name")
		helper.AssertNoError(err, "Should get user name from loaded data")
		helper.AssertEqual("Alice", userName, "User name should be correct")

		// Test modifying and saving back
		modifiedData, err := processor.Set(loadedDataStr, "users[0].name", "Alice Updated")
		helper.AssertNoError(err, "Should modify loaded data")

		err = SaveToFile(testFile, modifiedData, true)
		helper.AssertNoError(err, "Should save modified data")

		// Verify modification persisted
		reloadedDataStr, err := LoadFromFile(testFile)
		helper.AssertNoError(err, "Should reload modified data")

		updatedName, err := processor.Get(reloadedDataStr, "users[0].name")
		helper.AssertNoError(err, "Should get updated name")
		helper.AssertEqual("Alice Updated", updatedName, "Updated name should be correct")
	})

	t.Run("StreamProcessing", func(t *testing.T) {
		processor := New()
		defer processor.Close()

		testData := `{
			"items": [
				{"id": 1, "value": 100},
				{"id": 2, "value": 200},
				{"id": 3, "value": 300}
			]
		}`

		// Test reading from reader
		reader := strings.NewReader(testData)
		loadedData, err := processor.LoadFromReader(reader)
		helper.AssertNoError(err, "Should load from reader")

		// Convert to JSON string for Get operation
		loadedDataStr, err := processor.EncodeWithConfig(loadedData, DefaultEncodeConfig())
		helper.AssertNoError(err, "Should encode loaded data")

		value, err := processor.Get(loadedDataStr, "items[1].value")
		helper.AssertNoError(err, "Should get value from stream data")
		helper.AssertEqual(float64(200), value, "Stream data value should be correct")

		// Test writing to writer
		var buffer bytes.Buffer
		err = processor.SaveToWriter(&buffer, loadedData, true)
		helper.AssertNoError(err, "Should save to writer")

		// Verify written data
		writtenData := buffer.String()
		helper.AssertTrue(len(writtenData) > 0, "Written data should not be empty")
		helper.AssertTrue(strings.Contains(writtenData, "items"), "Written data should contain items")

		// Test round-trip through stream
		roundTripReader := strings.NewReader(writtenData)
		roundTripData, err := processor.LoadFromReader(roundTripReader)
		helper.AssertNoError(err, "Should load round-trip data")

		// Convert to JSON string for Get operation
		roundTripDataStr, err := processor.EncodeWithConfig(roundTripData, DefaultEncodeConfig())
		helper.AssertNoError(err, "Should encode round-trip data")

		roundTripValue, err := processor.Get(roundTripDataStr, "items[2].value")
		helper.AssertNoError(err, "Should get round-trip value")
		helper.AssertEqual(float64(300), roundTripValue, "Round-trip value should be correct")
	})

	t.Run("LargeFileHandling", func(t *testing.T) {
		processor := New()
		defer processor.Close()

		// Create a large JSON structure
		largeDataMap := map[string]any{
			"data": make([]map[string]any, 100), // Reduce size for testing
		}

		for i := 0; i < 100; i++ {
			largeDataMap["data"].([]map[string]any)[i] = map[string]any{
				"id":   i,
				"name": fmt.Sprintf("User%02d", i%10),
				"data": []int{i, i + 1, i + 2},
			}
		}

		// Convert to JSON string
		largeJSON, err := processor.EncodeWithConfig(largeDataMap, DefaultEncodeConfig())
		helper.AssertNoError(err, "Should encode large data")

		// Test saving large file
		largeFile := filepath.Join(tempDir, "large_data.json")
		err = SaveToFile(largeFile, largeJSON, false)
		helper.AssertNoError(err, "Should save large file")

		// Test loading large file
		loadedLargeDataStr, err := LoadFromFile(largeFile)
		helper.AssertNoError(err, "Should load large file")

		// Test operations on large data
		firstItem, err := processor.Get(loadedLargeDataStr, "data[0].name")
		helper.AssertNoError(err, "Should get first item from large data")
		helper.AssertEqual("User00", firstItem, "First item should be correct")

		lastItem, err := processor.Get(loadedLargeDataStr, "data[99].name")
		helper.AssertNoError(err, "Should get last item from large data")
		helper.AssertEqual("User09", lastItem, "Last item should be correct")

		// Test array operations on large data
		dataArray, err := processor.Get(loadedLargeDataStr, "data[10:15]")
		helper.AssertNoError(err, "Should slice large array")
		if sliceArray, ok := dataArray.([]any); ok {
			helper.AssertEqual(5, len(sliceArray), "Large array slice should have correct length")
		}
	})

	t.Run("FileErrorHandling", func(t *testing.T) {
		// Test loading non-existent file
		_, err := LoadFromFile(filepath.Join(tempDir, "nonexistent.json"))
		helper.AssertError(err, "Should error on non-existent file")

		// Test saving to invalid path (skip on Windows as it may create the path)
		if os.PathSeparator == '/' { // Unix-like systems
			err = SaveToFile("/invalid/path/file.json", `{"test": true}`, false)
			helper.AssertError(err, "Should error on invalid path")
		}

		// Test loading invalid JSON file
		invalidFile := filepath.Join(tempDir, "invalid.json")
		err = os.WriteFile(invalidFile, []byte(`{invalid json`), 0644)
		helper.AssertNoError(err, "Should write invalid JSON file")

		_, err = LoadFromFile(invalidFile)
		// LoadFromFile just reads the file, it doesn't parse JSON, so it won't error
		helper.AssertNoError(err, "LoadFromFile should succeed even with invalid JSON")

		// Test loading empty file
		emptyFile := filepath.Join(tempDir, "empty.json")
		err = os.WriteFile(emptyFile, []byte(``), 0644)
		helper.AssertNoError(err, "Should write empty file")

		_, err = LoadFromFile(emptyFile)
		helper.AssertNoError(err, "LoadFromFile should succeed with empty file")

		// Test stream error handling
		processor2 := New()
		defer processor2.Close()

		errorReader := &errorReader{}
		_, err = processor2.LoadFromReader(errorReader)
		helper.AssertError(err, "Should error on reader error")

		errorWriter := &errorWriter{}
		err = processor2.SaveToWriter(errorWriter, `{"test": true}`, false)
		helper.AssertError(err, "Should error on writer error")
	})

	t.Run("ConcurrentFileAccess", func(t *testing.T) {
		processor := New()
		defer processor.Close()

		testData := `{
			"counter": 0,
			"items": []
		}`

		concurrentFile := filepath.Join(tempDir, "concurrent.json")
		err := SaveToFile(concurrentFile, testData, false)
		helper.AssertNoError(err, "Should save initial concurrent file")

		var wg sync.WaitGroup
		var mu sync.Mutex
		errors := make([]error, 0)

		// Simulate concurrent read operations
		for i := 0; i < 5; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				
				// Read file
				dataStr, err := LoadFromFile(concurrentFile)
				if err != nil {
					mu.Lock()
					errors = append(errors, err)
					mu.Unlock()
					return
				}

				// Perform operation
				_, err = processor.Get(dataStr, "counter")
				if err != nil {
					mu.Lock()
					errors = append(errors, err)
					mu.Unlock()
				}
			}(i)
		}

		wg.Wait()

		mu.Lock()
		helper.AssertEqual(0, len(errors), "Concurrent reads should not produce errors")
		mu.Unlock()
	})

	t.Run("StreamingWithLargeData", func(t *testing.T) {
		processor := New()
		defer processor.Close()

		// Create streaming data
		streamDataMap := map[string]any{
			"stream": make([]map[string]any, 50), // Reduce size for testing
		}

		for i := 0; i < 50; i++ {
			streamDataMap["stream"].([]map[string]any)[i] = map[string]any{
				"index":     i,
				"timestamp": fmt.Sprintf("2024-01-01T00:00:%02dZ", i%60),
			}
		}

		// Convert to JSON string
		streamDataJSON, err := processor.EncodeWithConfig(streamDataMap, DefaultEncodeConfig())
		helper.AssertNoError(err, "Should encode stream data")

		// Test streaming through buffer
		reader := strings.NewReader(streamDataJSON)
		data, err := processor.LoadFromReader(reader)
		helper.AssertNoError(err, "Should load streaming data")

		// Convert to JSON string for Get operations
		dataStr, err := processor.EncodeWithConfig(data, DefaultEncodeConfig())
		helper.AssertNoError(err, "Should encode streaming data")

		// Test operations on streamed data
		firstTimestamp, err := processor.Get(dataStr, "stream[0].timestamp")
		helper.AssertNoError(err, "Should get first timestamp")
		if firstTimestamp != nil {
			helper.AssertTrue(strings.Contains(firstTimestamp.(string), "2024-01-01"), "Timestamp should be correct")
		}

		// Test array operations on streamed data
		streamSlice, err := processor.Get(dataStr, "stream[5:10]")
		helper.AssertNoError(err, "Should slice streamed array")
		if sliceArray, ok := streamSlice.([]any); ok {
			helper.AssertEqual(5, len(sliceArray), "Streamed slice should have correct length")
		}

		// Test writing back to stream
		var outputBuffer bytes.Buffer
		err = processor.SaveToWriter(&outputBuffer, data, false)
		helper.AssertNoError(err, "Should write streamed data back")

		// Verify output size
		outputSize := outputBuffer.Len()
		helper.AssertTrue(outputSize > 1000, "Output should be substantial size")
	})
}

// Helper types for error testing
type errorReader struct{}

func (r *errorReader) Read(p []byte) (n int, err error) {
	return 0, io.ErrUnexpectedEOF
}

type errorWriter struct{}

func (w *errorWriter) Write(p []byte) (n int, err error) {
	return 0, io.ErrShortWrite
}
