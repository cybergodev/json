package json

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ============================================================================
// FILE OPERATIONS TESTS
// Tests for file.go: LoadFromFile, SaveToFile, LargeFileProcessor,
// ChunkedReader, NDJSONProcessor, ChunkedWriter, SamplingReader, LazyParser
// ============================================================================

// TestLoadFromFile tests the LoadFromFile method
func TestLoadFromFile(t *testing.T) {
	processor := New()
	defer processor.Close()

	t.Run("ValidFile", func(t *testing.T) {
		tempDir := t.TempDir()
		filePath := filepath.Join(tempDir, "test.json")
		testData := `{"name":"test","value":123}`

		err := os.WriteFile(filePath, []byte(testData), 0644)
		if err != nil {
			t.Fatalf("Failed to write test file: %v", err)
		}

		loaded, err := processor.LoadFromFile(filePath)
		if err != nil {
			t.Errorf("LoadFromFile failed: %v", err)
		}
		if loaded != testData {
			t.Errorf("Loaded data = %q, want %q", loaded, testData)
		}
	})

	t.Run("LoadFromFileAsData", func(t *testing.T) {
		tempDir := t.TempDir()
		filePath := filepath.Join(tempDir, "test_data.json")
		testData := `{"name":"test","nested":{"key":"value"}}`

		err := os.WriteFile(filePath, []byte(testData), 0644)
		if err != nil {
			t.Fatalf("Failed to write test file: %v", err)
		}

		data, err := processor.LoadFromFileAsData(filePath)
		if err != nil {
			t.Errorf("LoadFromFileAsData failed: %v", err)
		}

		result, ok := data.(map[string]interface{})
		if !ok {
			t.Errorf("Expected map[string]interface{}, got %T", data)
		}
		if result["name"] != "test" {
			t.Errorf("name = %v, want test", result["name"])
		}
	})

	t.Run("NonExistentFile", func(t *testing.T) {
		_, err := processor.LoadFromFile("/non/existent/file.json")
		if err == nil {
			t.Error("Expected error for non-existent file")
		}
	})

	t.Run("EmptyFilePath", func(t *testing.T) {
		_, err := processor.LoadFromFile("")
		if err == nil {
			t.Error("Expected error for empty file path")
		}
	})
}

// TestSaveToFile tests the SaveToFile method
func TestSaveToFile(t *testing.T) {
	processor := New()
	defer processor.Close()

	t.Run("SaveAndLoad", func(t *testing.T) {
		tempDir := t.TempDir()
		filePath := filepath.Join(tempDir, "save_test.json")
		testData := map[string]interface{}{"name": "test", "value": 123}

		err := processor.SaveToFile(filePath, testData, false)
		if err != nil {
			t.Errorf("SaveToFile failed: %v", err)
		}

		loaded, err := processor.LoadFromFile(filePath)
		if err != nil {
			t.Errorf("LoadFromFile failed: %v", err)
		}

		var parsed map[string]interface{}
		if err := json.Unmarshal([]byte(loaded), &parsed); err != nil {
			t.Errorf("Failed to parse loaded data: %v", err)
		}

		if parsed["name"] != "test" {
			t.Errorf("name = %v, want test", parsed["name"])
		}
	})

	t.Run("PrettyPrint", func(t *testing.T) {
		tempDir := t.TempDir()
		filePath := filepath.Join(tempDir, "pretty_test.json")
		testData := map[string]interface{}{"name": "test", "nested": map[string]interface{}{"key": "value"}}

		err := processor.SaveToFile(filePath, testData, true)
		if err != nil {
			t.Errorf("SaveToFile with pretty failed: %v", err)
		}

		content, err := os.ReadFile(filePath)
		if err != nil {
			t.Errorf("Failed to read file: %v", err)
		}

		if !strings.Contains(string(content), "\n") {
			t.Error("Expected pretty-printed JSON with newlines")
		}
	})

	t.Run("CreateDirectory", func(t *testing.T) {
		tempDir := t.TempDir()
		filePath := filepath.Join(tempDir, "subdir", "nested", "test.json")
		testData := map[string]interface{}{"created": true}

		err := processor.SaveToFile(filePath, testData, false)
		if err != nil {
			t.Errorf("SaveToFile with directory creation failed: %v", err)
		}

		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			t.Error("File was not created")
		}
	})

	t.Run("SaveStringData", func(t *testing.T) {
		tempDir := t.TempDir()
		filePath := filepath.Join(tempDir, "string_test.json")
		testData := `{"already":"json"}`

		err := processor.SaveToFile(filePath, testData, false)
		if err != nil {
			t.Errorf("SaveToFile with string data failed: %v", err)
		}
	})
}

// TestLoadFromReader tests the LoadFromReader method
func TestLoadFromReader(t *testing.T) {
	processor := New()
	defer processor.Close()

	t.Run("ValidReader", func(t *testing.T) {
		testData := `{"name":"reader_test"}`
		reader := strings.NewReader(testData)

		loaded, err := processor.LoadFromReader(reader)
		if err != nil {
			t.Errorf("LoadFromReader failed: %v", err)
		}
		if loaded != testData {
			t.Errorf("Loaded data = %q, want %q", loaded, testData)
		}
	})

	t.Run("LoadFromReaderAsData", func(t *testing.T) {
		testData := `{"key":"value","number":42}`
		reader := strings.NewReader(testData)

		data, err := processor.LoadFromReaderAsData(reader)
		if err != nil {
			t.Errorf("LoadFromReaderAsData failed: %v", err)
		}

		_, ok := data.(map[string]interface{})
		if !ok {
			t.Errorf("Expected map[string]interface{}, got %T", data)
		}
	})
}

// TestSaveToWriter tests the SaveToWriter method
func TestSaveToWriter(t *testing.T) {
	processor := New()
	defer processor.Close()

	t.Run("SaveToWriter", func(t *testing.T) {
		testData := map[string]interface{}{"writer": "test"}
		var buf bytes.Buffer

		err := processor.SaveToWriter(&buf, testData, false)
		if err != nil {
			t.Errorf("SaveToWriter failed: %v", err)
		}

		if buf.Len() == 0 {
			t.Error("Expected non-empty buffer")
		}
	})

	t.Run("SaveToWriterPretty", func(t *testing.T) {
		testData := map[string]interface{}{"pretty": true}
		var buf bytes.Buffer

		err := processor.SaveToWriter(&buf, testData, true)
		if err != nil {
			t.Errorf("SaveToWriter with pretty failed: %v", err)
		}

		if !strings.Contains(buf.String(), "\n") {
			t.Error("Expected pretty-printed JSON")
		}
	})
}

// TestMarshalToFile tests the MarshalToFile method
func TestMarshalToFile(t *testing.T) {
	processor := New()
	defer processor.Close()

	t.Run("MarshalAndUnmarshal", func(t *testing.T) {
		tempDir := t.TempDir()
		filePath := filepath.Join(tempDir, "marshal_test.json")

		type TestStruct struct {
			Name  string `json:"name"`
			Value int    `json:"value"`
		}

		testData := TestStruct{Name: "marshal_test", Value: 42}

		err := processor.MarshalToFile(filePath, testData, false)
		if err != nil {
			t.Errorf("MarshalToFile failed: %v", err)
		}

		var loaded TestStruct
		err = processor.UnmarshalFromFile(filePath, &loaded)
		if err != nil {
			t.Errorf("UnmarshalFromFile failed: %v", err)
		}

		if loaded.Name != testData.Name || loaded.Value != testData.Value {
			t.Errorf("Loaded = %+v, want %+v", loaded, testData)
		}
	})

	t.Run("MarshalPretty", func(t *testing.T) {
		tempDir := t.TempDir()
		filePath := filepath.Join(tempDir, "marshal_pretty.json")

		testData := map[string]interface{}{"pretty": true}

		err := processor.MarshalToFile(filePath, testData, true)
		if err != nil {
			t.Errorf("MarshalToFile with pretty failed: %v", err)
		}

		content, err := os.ReadFile(filePath)
		if err != nil {
			t.Errorf("Failed to read file: %v", err)
		}

		if !strings.Contains(string(content), "\n") {
			t.Error("Expected pretty-printed JSON")
		}
	})
}

// ============================================================================
// LARGE FILE PROCESSOR TESTS
// ============================================================================

// TestLargeFileProcessor tests the LargeFileProcessor
func TestLargeFileProcessor(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping large file tests in short mode")
	}

	t.Run("ProcessFile", func(t *testing.T) {
		tempDir := t.TempDir()
		filePath := filepath.Join(tempDir, "large_test.json")

		// Create a large JSON array
		var items []map[string]interface{}
		for i := 0; i < 100; i++ {
			items = append(items, map[string]interface{}{
				"id":    i,
				"name":  fmt.Sprintf("Item%d", i),
				"value": i * 10,
			})
		}
		data, _ := json.Marshal(items)
		os.WriteFile(filePath, data, 0644)

		config := DefaultLargeFileConfig()
		config.ChunkSize = 1024
		lfp := NewLargeFileProcessor(config)

		count := 0
		err := lfp.ProcessFile(filePath, func(item interface{}) error {
			count++
			return nil
		})

		if err != nil {
			t.Errorf("ProcessFile failed: %v", err)
		}
		if count != 100 {
			t.Errorf("Processed %d items, want 100", count)
		}
	})

	t.Run("ProcessFileChunked", func(t *testing.T) {
		tempDir := t.TempDir()
		filePath := filepath.Join(tempDir, "chunked_test.json")

		// Create a large JSON array
		var items []map[string]interface{}
		for i := 0; i < 50; i++ {
			items = append(items, map[string]interface{}{
				"id":    i,
				"value": i * 2,
			})
		}
		data, _ := json.Marshal(items)
		os.WriteFile(filePath, data, 0644)

		config := DefaultLargeFileConfig()
		lfp := NewLargeFileProcessor(config)

		chunks := 0
		totalItems := 0
		err := lfp.ProcessFileChunked(filePath, 10, func(chunk []interface{}) error {
			chunks++
			totalItems += len(chunk)
			return nil
		})

		if err != nil {
			t.Errorf("ProcessFileChunked failed: %v", err)
		}
		if chunks < 5 {
			t.Errorf("Expected at least 5 chunks, got %d", chunks)
		}
		if totalItems != 50 {
			t.Errorf("Total items = %d, want 50", totalItems)
		}
	})
}

// TestDefaultLargeFileConfig tests the DefaultLargeFileConfig function
func TestDefaultLargeFileConfig(t *testing.T) {
	config := DefaultLargeFileConfig()

	if config.ChunkSize <= 0 {
		t.Error("ChunkSize should be positive")
	}
	if config.MaxMemory <= 0 {
		t.Error("MaxMemory should be positive")
	}
	if config.BufferSize <= 0 {
		t.Error("BufferSize should be positive")
	}
}

// ============================================================================
// CHUNKED READER TESTS
// ============================================================================

// TestChunkedReader tests the ChunkedReader
func TestChunkedReader(t *testing.T) {
	t.Run("ReadArray", func(t *testing.T) {
		jsonData := `[{"id":1},{"id":2},{"id":3}]`
		reader := strings.NewReader(jsonData)
		cr := NewChunkedReader(reader, 1024)

		count := 0
		err := cr.ReadArray(func(item interface{}) bool {
			count++
			return true
		})

		if err != nil {
			t.Errorf("ReadArray failed: %v", err)
		}
		if count != 3 {
			t.Errorf("Read %d items, want 3", count)
		}
	})

	t.Run("ReadArrayEarlyStop", func(t *testing.T) {
		jsonData := `[1,2,3,4,5]`
		reader := strings.NewReader(jsonData)
		cr := NewChunkedReader(reader, 1024)

		count := 0
		err := cr.ReadArray(func(item interface{}) bool {
			count++
			return count < 2 // Stop after 2 items
		})

		if err != nil {
			t.Errorf("ReadArray failed: %v", err)
		}
		if count != 2 {
			t.Errorf("Read %d items, want 2", count)
		}
	})

	t.Run("ReadObject", func(t *testing.T) {
		jsonData := `{"key1":"value1","key2":"value2","key3":"value3"}`
		reader := strings.NewReader(jsonData)
		cr := NewChunkedReader(reader, 1024)

		count := 0
		err := cr.ReadObject(func(key string, value interface{}) bool {
			count++
			return true
		})

		if err != nil {
			t.Errorf("ReadObject failed: %v", err)
		}
		if count != 3 {
			t.Errorf("Read %d items, want 3", count)
		}
	})

	t.Run("NotAnArray", func(t *testing.T) {
		jsonData := `{"not":"an array"}`
		reader := strings.NewReader(jsonData)
		cr := NewChunkedReader(reader, 1024)

		count := 0
		err := cr.ReadArray(func(item interface{}) bool {
			count++
			return true
		})

		// ReadArray expects an array, so non-array should return an error
		if err == nil {
			t.Error("Expected error for non-array input")
		}
		// No items should be processed
		if count != 0 {
			t.Errorf("Read %d items, want 0", count)
		}
	})
}

// ============================================================================
// NDJSON PROCESSOR TESTS
// ============================================================================

// TestNDJSONProcessor tests the NDJSONProcessor
func TestNDJSONProcessor(t *testing.T) {
	t.Run("ProcessFile", func(t *testing.T) {
		tempDir := t.TempDir()
		filePath := filepath.Join(tempDir, "test.ndjson")

		// Create NDJSON file
		content := `{"id":1,"name":"first"}
{"id":2,"name":"second"}
{"id":3,"name":"third"}
`
		os.WriteFile(filePath, []byte(content), 0644)

		np := NewNDJSONProcessor(1024)

		lines := 0
		err := np.ProcessFile(filePath, func(lineNum int, obj map[string]interface{}) error {
			lines++
			return nil
		})

		if err != nil {
			t.Errorf("ProcessFile failed: %v", err)
		}
		if lines != 3 {
			t.Errorf("Processed %d lines, want 3", lines)
		}
	})

	t.Run("ProcessReader", func(t *testing.T) {
		content := `{"id":1}
{"id":2}
`
		reader := strings.NewReader(content)

		np := NewNDJSONProcessor(1024)

		lines := 0
		err := np.ProcessReader(reader, func(lineNum int, obj map[string]interface{}) error {
			lines++
			return nil
		})

		if err != nil {
			t.Errorf("ProcessReader failed: %v", err)
		}
		if lines != 2 {
			t.Errorf("Processed %d lines, want 2", lines)
		}
	})

	t.Run("SkipInvalidLines", func(t *testing.T) {
		content := `{"id":1}
invalid json
{"id":2}
`
		reader := strings.NewReader(content)

		np := NewNDJSONProcessor(1024)

		lines := 0
		err := np.ProcessReader(reader, func(lineNum int, obj map[string]interface{}) error {
			lines++
			return nil
		})

		if err != nil {
			t.Errorf("ProcessReader failed: %v", err)
		}
		// Should only process valid lines
		if lines != 2 {
			t.Errorf("Processed %d lines, want 2 (skipping invalid)", lines)
		}
	})

	t.Run("SkipEmptyLines", func(t *testing.T) {
		content := `{"id":1}

{"id":2}
`
		reader := strings.NewReader(content)

		np := NewNDJSONProcessor(1024)

		lines := 0
		err := np.ProcessReader(reader, func(lineNum int, obj map[string]interface{}) error {
			lines++
			return nil
		})

		if err != nil {
			t.Errorf("ProcessReader failed: %v", err)
		}
		if lines != 2 {
			t.Errorf("Processed %d lines, want 2 (skipping empty)", lines)
		}
	})
}

// ============================================================================
// CHUNKED WRITER TESTS
// ============================================================================

// TestChunkedWriter tests the ChunkedWriter
func TestChunkedWriter(t *testing.T) {
	t.Run("WriteArray", func(t *testing.T) {
		var buf bytes.Buffer
		cw := NewChunkedWriter(&buf, 1024, true)

		for i := 0; i < 5; i++ {
			if err := cw.WriteItem(map[string]interface{}{"id": i}); err != nil {
				t.Errorf("WriteItem failed: %v", err)
			}
		}

		if err := cw.Flush(true); err != nil {
			t.Errorf("Flush failed: %v", err)
		}

		result := buf.String()
		if result[0] != '[' || result[len(result)-1] != ']' {
			t.Errorf("Result should be a JSON array, got: %s", result)
		}

		if cw.Count() != 5 {
			t.Errorf("Count = %d, want 5", cw.Count())
		}
	})

	t.Run("WriteObject", func(t *testing.T) {
		var buf bytes.Buffer
		cw := NewChunkedWriter(&buf, 1024, false)

		for i := 0; i < 3; i++ {
			if err := cw.WriteKeyValue(fmt.Sprintf("key%d", i), i); err != nil {
				t.Errorf("WriteKeyValue failed: %v", err)
			}
		}

		if err := cw.Flush(true); err != nil {
			t.Errorf("Flush failed: %v", err)
		}

		result := buf.String()
		if result[0] != '{' || result[len(result)-1] != '}' {
			t.Errorf("Result should be a JSON object, got: %s", result)
		}
	})

	t.Run("AutoFlush", func(t *testing.T) {
		var buf bytes.Buffer
		// Small chunk size to trigger auto-flush
		cw := NewChunkedWriter(&buf, 50, true)

		for i := 0; i < 100; i++ {
			if err := cw.WriteItem(map[string]interface{}{"id": i, "data": strings.Repeat("x", 100)}); err != nil {
				t.Errorf("WriteItem failed: %v", err)
			}
		}

		// Buffer should have content even before final flush
		if buf.Len() == 0 {
			t.Error("Expected auto-flush to write data")
		}
	})
}

// ============================================================================
// SAMPLING READER TESTS
// ============================================================================

// TestSamplingReader tests the SamplingReader
func TestSamplingReader(t *testing.T) {
	t.Run("SampleArray", func(t *testing.T) {
		// Create array with 100 items
		var items []interface{}
		for i := 0; i < 100; i++ {
			items = append(items, map[string]interface{}{"id": i})
		}
		data, _ := json.Marshal(items)
		reader := strings.NewReader(string(data))

		sr := NewSamplingReader(reader, 10)

		samples := 0
		err := sr.Sample(func(index int, item interface{}) bool {
			samples++
			return true
		})

		if err != nil {
			t.Errorf("Sample failed: %v", err)
		}
		if samples > 10 {
			t.Errorf("Sampled %d items, should be at most 10", samples)
		}

		if sr.TotalRead() != 100 {
			t.Errorf("TotalRead = %d, want 100", sr.TotalRead())
		}
	})

	t.Run("SampleSingleValue", func(t *testing.T) {
		data := `{"not":"an array"}`
		reader := strings.NewReader(data)

		sr := NewSamplingReader(reader, 10)

		samples := 0
		err := sr.Sample(func(index int, item interface{}) bool {
			samples++
			return true
		})

		// Sample expects an array, so non-array should return an error
		if err == nil {
			t.Error("Expected error for non-array input")
		}
		// No items should be processed
		if samples != 0 {
			t.Errorf("Sampled %d items, want 0", samples)
		}
	})
}

// ============================================================================
// LAZY PARSER TESTS
// ============================================================================

// TestLazyParser tests the LazyParser
func TestLazyParser(t *testing.T) {
	t.Run("LazyParsing", func(t *testing.T) {
		data := []byte(`{"name":"test","nested":{"key":"value"}}`)
		lp := NewLazyParser(data)

		// Should not be parsed yet
		if lp.IsParsed() {
			t.Error("Should not be parsed yet")
		}

		// Get triggers parsing
		result, err := lp.Get("name")
		if err != nil {
			t.Errorf("Get failed: %v", err)
		}
		if result != "test" {
			t.Errorf("Get = %v, want test", result)
		}

		// Should be parsed now
		if !lp.IsParsed() {
			t.Error("Should be parsed now")
		}
	})

	t.Run("GetNested", func(t *testing.T) {
		data := []byte(`{"nested":{"key":"value"}}`)
		lp := NewLazyParser(data)

		result, err := lp.Get("nested.key")
		if err != nil {
			t.Errorf("Get nested failed: %v", err)
		}
		if result != "value" {
			t.Errorf("Get = %v, want value", result)
		}
	})

	t.Run("GetAll", func(t *testing.T) {
		data := []byte(`{"a":1,"b":2}`)
		lp := NewLazyParser(data)

		all, err := lp.GetAll()
		if err != nil {
			t.Errorf("GetAll failed: %v", err)
		}
		if all["a"].(float64) != 1 || all["b"].(float64) != 2 {
			t.Errorf("GetAll = %v, want {a:1, b:2}", all)
		}
	})

	t.Run("Raw", func(t *testing.T) {
		data := []byte(`{"test":"data"}`)
		lp := NewLazyParser(data)

		raw := lp.Raw()
		if string(raw) != string(data) {
			t.Errorf("Raw = %s, want %s", raw, data)
		}
	})

	t.Run("InvalidJSON", func(t *testing.T) {
		data := []byte(`{invalid json}`)
		lp := NewLazyParser(data)

		_, err := lp.Get("key")
		if err == nil {
			t.Error("Expected error for invalid JSON")
		}
	})

	t.Run("RootPath", func(t *testing.T) {
		data := []byte(`{"name":"test"}`)
		lp := NewLazyParser(data)

		result, err := lp.Get(".")
		if err != nil {
			t.Errorf("Get root failed: %v", err)
		}
		_, ok := result.(map[string]interface{})
		if !ok {
			t.Errorf("Expected map, got %T", result)
		}
	})
}

// ============================================================================
// STREAMING PROCESSOR FILE TESTS
// ============================================================================

// TestStreamingProcessorWithFile tests streaming with file-like data
func TestStreamingProcessorWithFile(t *testing.T) {
	t.Run("StreamLargeArray", func(t *testing.T) {
		// Create a large array
		var buf bytes.Buffer
		buf.WriteString("[")
		for i := 0; i < 1000; i++ {
			if i > 0 {
				buf.WriteString(",")
			}
			buf.WriteString(fmt.Sprintf(`{"id":%d}`, i))
		}
		buf.WriteString("]")

		sp := NewStreamingProcessor(bytes.NewReader(buf.Bytes()), 0)

		count := 0
		err := sp.StreamArray(func(index int, item interface{}) bool {
			count++
			return true
		})

		if err != nil {
			t.Errorf("StreamArray failed: %v", err)
		}
		if count != 1000 {
			t.Errorf("Streamed %d items, want 1000", count)
		}
	})

	t.Run("StreamWithEarlyStop", func(t *testing.T) {
		var buf bytes.Buffer
		buf.WriteString("[")
		for i := 0; i < 100; i++ {
			if i > 0 {
				buf.WriteString(",")
			}
			buf.WriteString(fmt.Sprintf(`%d`, i))
		}
		buf.WriteString("]")

		sp := NewStreamingProcessor(bytes.NewReader(buf.Bytes()), 0)

		count := 0
		err := sp.StreamArray(func(index int, item interface{}) bool {
			count++
			return count < 10 // Stop after 10
		})

		if err != nil {
			t.Errorf("StreamArray failed: %v", err)
		}
		if count != 10 {
			t.Errorf("Streamed %d items, want 10", count)
		}
	})
}
