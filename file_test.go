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
// NDJSONProcessor, SamplingReader, LazyParser
// ============================================================================

// TestLoadFromFile tests the LoadFromFile method
func TestLoadFromFile(t *testing.T) {
	processor, _ := New()
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

		data, err := processor.loadFromFileAsData(filePath)
		if err != nil {
			t.Errorf("loadFromFileAsData failed: %v", err)
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
	processor, _ := New()
	defer processor.Close()

	t.Run("SaveAndLoad", func(t *testing.T) {
		tempDir := t.TempDir()
		filePath := filepath.Join(tempDir, "save_test.json")
		testData := map[string]interface{}{"name": "test", "value": 123}

		cfg := DefaultConfig()
		cfg.Pretty = false
		err := processor.SaveToFile(filePath, testData, cfg)
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

		cfg := DefaultConfig()
		cfg.Pretty = true
		err := processor.SaveToFile(filePath, testData, cfg)
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

		cfg := DefaultConfig()
		cfg.Pretty = false
		err := processor.SaveToFile(filePath, testData, cfg)
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

		cfg := DefaultConfig()
		cfg.Pretty = false
		err := processor.SaveToFile(filePath, testData, cfg)
		if err != nil {
			t.Errorf("SaveToFile with string data failed: %v", err)
		}
	})
}

// TestLoadFromReader tests the LoadFromReader method
func TestLoadFromReader(t *testing.T) {
	processor, _ := New()
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

		data, err := processor.loadFromReaderAsData(reader)
		if err != nil {
			t.Errorf("loadFromReaderAsData failed: %v", err)
		}

		_, ok := data.(map[string]interface{})
		if !ok {
			t.Errorf("Expected map[string]interface{}, got %T", data)
		}
	})
}

// TestSaveToWriter tests the SaveToWriter method
func TestSaveToWriter(t *testing.T) {
	processor, _ := New()
	defer processor.Close()

	t.Run("SaveToWriter", func(t *testing.T) {
		testData := map[string]interface{}{"writer": "test"}
		var buf bytes.Buffer

		cfg := DefaultConfig()
		cfg.Pretty = false
		err := processor.SaveToWriter(&buf, testData, cfg)
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

		cfg := DefaultConfig()
		cfg.Pretty = true
		err := processor.SaveToWriter(&buf, testData, cfg)
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
	processor, _ := New()
	defer processor.Close()

	t.Run("MarshalAndUnmarshal", func(t *testing.T) {
		tempDir := t.TempDir()
		filePath := filepath.Join(tempDir, "marshal_test.json")

		type TestStruct struct {
			Name  string `json:"name"`
			Value int    `json:"value"`
		}

		testData := TestStruct{Name: "marshal_test", Value: 42}

		cfg := DefaultConfig()
		cfg.Pretty = false
		err := processor.MarshalToFile(filePath, testData, cfg)
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

		cfg := DefaultConfig()
		cfg.Pretty = true
		err := processor.MarshalToFile(filePath, testData, cfg)
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

// TestDefaultConfigLargeFile tests that DefaultConfig has proper large file settings
func TestDefaultConfigLargeFile(t *testing.T) {
	config := DefaultConfig()

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

		np := NewNDJSONProcessor()

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

		np := NewNDJSONProcessor()

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

		cfg := DefaultConfig()
		cfg.JSONLContinueOnErr = true
		np := NewNDJSONProcessor(cfg)

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

		np := NewNDJSONProcessor()

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
// FOREACHFILE TESTS
// Tests for file.go: ForeachFile, ForeachFileWithPath, ForeachFileChunked
// ============================================================================

func TestForeachFile(t *testing.T) {
	processor, _ := New()
	defer processor.Close()

	t.Run("BasicIteration", func(t *testing.T) {
		tempDir := t.TempDir()
		filePath := filepath.Join(tempDir, "test.json")
		testData := `[{"id":1,"name":"Alice"},{"id":2,"name":"Bob"},{"id":3,"name":"Charlie"}]`

		err := os.WriteFile(filePath, []byte(testData), 0644)
		if err != nil {
			t.Fatalf("Failed to write test file: %v", err)
		}

		var ids []int
		var names []string
		err = processor.ForeachFile(filePath, func(key any, item *IterableValue) error {
			ids = append(ids, item.GetInt("id"))
			names = append(names, item.GetString("name"))
			return nil
		})

		if err != nil {
			t.Errorf("ForeachFile failed: %v", err)
		}
		if len(ids) != 3 {
			t.Errorf("Expected 3 items, got %d", len(ids))
		}
		if names[0] != "Alice" {
			t.Errorf("Expected first name 'Alice', got '%s'", names[0])
		}
	})

	t.Run("BreakIteration", func(t *testing.T) {
		tempDir := t.TempDir()
		filePath := filepath.Join(tempDir, "test.json")
		testData := `[{"id":1},{"id":2},{"id":3},{"id":4},{"id":5}]`

		err := os.WriteFile(filePath, []byte(testData), 0644)
		if err != nil {
			t.Fatalf("Failed to write test file: %v", err)
		}

		count := 0
		err = processor.ForeachFile(filePath, func(key any, item *IterableValue) error {
			count++
			if item.GetInt("id") == 2 {
				return item.Break()
			}
			return nil
		})

		if err != nil {
			t.Errorf("ForeachFile failed: %v", err)
		}
		if count != 2 {
			t.Errorf("Expected 2 iterations (break at id=2), got %d", count)
		}
	})

	t.Run("ObjectIteration", func(t *testing.T) {
		tempDir := t.TempDir()
		filePath := filepath.Join(tempDir, "test.json")
		testData := `{"users":{"alice":{"age":30},"bob":{"age":25}}}`

		err := os.WriteFile(filePath, []byte(testData), 0644)
		if err != nil {
			t.Fatalf("Failed to write test file: %v", err)
		}

		var keys []string
		var ages []int
		err = processor.ForeachFileWithPath(filePath, "users", func(key any, item *IterableValue) error {
			keys = append(keys, key.(string))
			ages = append(ages, item.GetInt("age"))
			return nil
		})

		if err != nil {
			t.Errorf("ForeachFileWithPath failed: %v", err)
		}
		if len(keys) != 2 {
			t.Errorf("Expected 2 items, got %d", len(keys))
		}
	})
}

func TestForeachFileChunked(t *testing.T) {
	processor, _ := New()
	defer processor.Close()

	t.Run("BasicChunked", func(t *testing.T) {
		tempDir := t.TempDir()
		filePath := filepath.Join(tempDir, "test.json")

		// Create array with 10 items
		testData := `[`
		for i := 1; i <= 10; i++ {
			if i > 1 {
				testData += ","
			}
			testData += fmt.Sprintf(`{"id":%d}`, i)
		}
		testData += `]`

		err := os.WriteFile(filePath, []byte(testData), 0644)
		if err != nil {
			t.Fatalf("Failed to write test file: %v", err)
		}

		batches := 0
		totalItems := 0
		err = processor.ForeachFileChunked(filePath, 3, func(chunk []*IterableValue) error {
			batches++
			totalItems += len(chunk)
			return nil
		})

		if err != nil {
			t.Errorf("ForeachFileChunked failed: %v", err)
		}
		// 10 items with chunk size 3 = 4 batches (3, 3, 3, 1)
		if batches != 4 {
			t.Errorf("Expected 4 batches, got %d", batches)
		}
		if totalItems != 10 {
			t.Errorf("Expected 10 total items, got %d", totalItems)
		}
	})

	t.Run("BreakInChunk", func(t *testing.T) {
		tempDir := t.TempDir()
		filePath := filepath.Join(tempDir, "test.json")
		testData := `[{"id":1},{"id":2},{"id":3},{"id":4},{"id":5}]`

		err := os.WriteFile(filePath, []byte(testData), 0644)
		if err != nil {
			t.Fatalf("Failed to write test file: %v", err)
		}

		batches := 0
		err = processor.ForeachFileChunked(filePath, 2, func(chunk []*IterableValue) error {
			batches++
			if batches == 2 {
				return errOperationFailed // use any non-break error to stop
			}
			return nil
		})

		if err == nil {
			t.Error("Expected error from callback")
		}
		if batches != 2 {
			t.Errorf("Expected 2 batches before error, got %d", batches)
		}
	})

	// Non-array root must be rejected (folded from TestForeachFileChunked_NonArray).
	t.Run("NonArrayRoot", func(t *testing.T) {
		dir := t.TempDir()
		path := dir + "/obj.json"
		if err := os.WriteFile(path, []byte(`{"key": "value"}`), 0o644); err != nil {
			t.Fatalf("WriteFile error: %v", err)
		}

		if err := processor.ForeachFileChunked(path, 10, func(chunk []*IterableValue) error {
			return nil
		}); err == nil {
			t.Error("expected error for non-array root in ForeachFileChunked")
		}
	})

	// chunkSize <= 0 falls back to the default (folded from
	// TestForeachFileChunked_DefaultChunkSize).
	t.Run("DefaultChunkSize", func(t *testing.T) {
		dir := t.TempDir()
		path := dir + "/arr.json"
		items := make([]string, 150)
		for i := range items {
			items[i] = fmt.Sprintf(`{"id": %d}`, i)
		}
		content := "[" + strings.Join(items, ",") + "]"
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("WriteFile error: %v", err)
		}

		var totalItems int
		err := processor.ForeachFileChunked(path, 0, func(chunk []*IterableValue) error {
			totalItems += len(chunk)
			return nil
		})
		if err != nil {
			t.Fatalf("ForeachFileChunked error: %v", err)
		}
		if totalItems != 150 {
			t.Errorf("totalItems = %d, want 150", totalItems)
		}
	})
}

// ============================================================================
// Additional file coverage tests
// ============================================================================

// TestForeachFileNested tests the ForeachFileNested processor method
func TestForeachFileNested(t *testing.T) {
	p, _ := New()
	defer p.Close()

	dir := t.TempDir()
	path := dir + "/nested.json"
	content := `{"users": [{"name": "Alice", "age": 30}, {"name": "Bob", "age": 25}]}`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("WriteFile error: %v", err)
	}

	var count int
	err := p.ForeachFileNested(path, func(key any, item *IterableValue) error {
		count++
		return nil
	})
	if err != nil {
		t.Fatalf("ForeachFileNested error: %v", err)
	}
	if count == 0 {
		t.Error("ForeachFileNested should iterate at least once")
	}
}

// TestPackageLevel_SaveToFile tests the package-level SaveToFile wrapper
func TestPackageLevel_SaveToFile(t *testing.T) {
	tempDir := t.TempDir()

	t.Run("DefaultConfig", func(t *testing.T) {
		filePath := filepath.Join(tempDir, "pkg_default.json")
		if err := SaveToFile(filePath, map[string]any{"name": "test", "value": 123}); err != nil {
			t.Fatalf("SaveToFile failed: %v", err)
		}
		content, err := os.ReadFile(filePath)
		if err != nil {
			t.Fatalf("reading saved file: %v", err)
		}
		var back map[string]any
		if err := json.Unmarshal(content, &back); err != nil {
			t.Fatalf("saved content is not valid JSON: %v (content=%s)", err, content)
		}
		if back["name"] != "test" || back["value"] != float64(123) {
			t.Errorf("round-trip mismatch: got %v, want name=test value=123", back)
		}
	})

	t.Run("PrettyConfig", func(t *testing.T) {
		filePath := filepath.Join(tempDir, "pkg_pretty.json")
		if err := SaveToFile(filePath, map[string]any{"name": "test"}, PrettyConfig()); err != nil {
			t.Fatalf("SaveToFile with PrettyConfig failed: %v", err)
		}
		content, err := os.ReadFile(filePath)
		if err != nil {
			t.Fatalf("reading saved file: %v", err)
		}
		if !strings.Contains(string(content), "\n") {
			t.Errorf("PrettyConfig output should be multi-line, got %q", content)
		}
	})

	t.Run("WriteError", func(t *testing.T) {
		// Saving onto a directory path must surface the write error, not panic.
		if err := SaveToFile(tempDir, map[string]any{"a": 1}); err == nil {
			t.Error("SaveToFile to a directory path should fail")
		}
	})
}

// TestForeachFilePackageFunctions covers the package-level ForeachFile family
// wrappers in file.go (previously 0% coverage): happy path per function plus
// error propagation from a nonexistent file.
func TestForeachFilePackageFunctions(t *testing.T) {
	dir := t.TempDir()
	arrPath := filepath.Join(dir, "arr.json")
	if err := os.WriteFile(arrPath, []byte(`[{"n":1},{"n":2},{"n":3}]`), 0o644); err != nil {
		t.Fatal(err)
	}
	missing := filepath.Join(dir, "missing.json")

	t.Run("ForeachFile", func(t *testing.T) {
		count := 0
		if err := ForeachFile(arrPath, func(key any, item *IterableValue) error {
			count++
			return nil
		}); err != nil {
			t.Fatalf("ForeachFile failed: %v", err)
		}
		if count != 3 {
			t.Errorf("ForeachFile iterated %d items, want 3", count)
		}
		if err := ForeachFile(missing, func(key any, item *IterableValue) error { return nil }); err == nil {
			t.Error("ForeachFile on missing file should fail")
		}
	})

	t.Run("ForeachFileWithPath", func(t *testing.T) {
		objPath := filepath.Join(dir, "obj.json")
		if err := os.WriteFile(objPath, []byte(`{"users":[{"n":"a"}]}`), 0o644); err != nil {
			t.Fatal(err)
		}
		count := 0
		if err := ForeachFileWithPath(objPath, "users", func(key any, item *IterableValue) error {
			count++
			return nil
		}); err != nil {
			t.Fatalf("ForeachFileWithPath failed: %v", err)
		}
		if count != 1 {
			t.Errorf("ForeachFileWithPath iterated %d items, want 1", count)
		}
	})

	t.Run("ForeachFileChunked", func(t *testing.T) {
		chunks := 0
		items := 0
		if err := ForeachFileChunked(arrPath, 2, func(chunk []*IterableValue) error {
			chunks++
			items += len(chunk)
			return nil
		}); err != nil {
			t.Fatalf("ForeachFileChunked failed: %v", err)
		}
		if chunks != 2 || items != 3 {
			t.Errorf("ForeachFileChunked got %d chunks / %d items, want 2/3", chunks, items)
		}
	})

	t.Run("ForeachFileNested", func(t *testing.T) {
		count := 0
		if err := ForeachFileNested(arrPath, func(key any, item *IterableValue) error {
			count++
			return nil
		}); err != nil {
			t.Fatalf("ForeachFileNested failed: %v", err)
		}
		if count == 0 {
			t.Error("ForeachFileNested should iterate at least once")
		}
	})
}
