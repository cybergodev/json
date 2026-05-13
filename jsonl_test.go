package json

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

// ============================================================================
// JSONL CONFIG TESTS - Using unified Config struct
// ============================================================================

func TestJSONLConfigDefaults(t *testing.T) {
	config := DefaultConfig()

	if config.JSONLBufferSize <= 0 {
		t.Error("JSONLBufferSize should be positive")
	}
	if config.JSONLMaxLineSize <= 0 {
		t.Error("JSONLMaxLineSize should be positive")
	}
	if !config.JSONLSkipEmpty {
		t.Error("JSONLSkipEmpty should be true by default")
	}
	if config.JSONLSkipComments {
		t.Error("JSONLSkipComments should be false by default")
	}
	if config.JSONLContinueOnErr {
		t.Error("JSONLContinueOnErr should be false by default")
	}
}

func TestShouldSkipJSONLLineFromConfig(t *testing.T) {
	tests := []struct {
		name     string
		line     []byte
		config   Config
		expected bool
	}{
		{
			name:     "empty_line_skip_true",
			line:     []byte{},
			config:   Config{JSONLSkipEmpty: true},
			expected: true,
		},
		{
			name:     "empty_line_skip_false",
			line:     []byte{},
			config:   Config{JSONLSkipEmpty: false},
			expected: false,
		},
		{
			name:     "hash_comment_skip",
			line:     []byte("# this is a comment"),
			config:   Config{JSONLSkipComments: true},
			expected: true,
		},
		{
			name:     "double_slash_comment_skip",
			line:     []byte("// this is a comment"),
			config:   Config{JSONLSkipComments: true},
			expected: true,
		},
		{
			name:     "comment_skip_disabled",
			line:     []byte("# this is a comment"),
			config:   Config{JSONLSkipComments: false},
			expected: false,
		},
		{
			name:     "normal_json_not_skipped",
			line:     []byte(`{"key":"value"}`),
			config:   DefaultConfig(),
			expected: false,
		},
		{
			name:     "single_slash_not_comment",
			line:     []byte(`/path/to/file`),
			config:   Config{JSONLSkipComments: true},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := shouldSkipJSONLLineFromConfig(tt.line, &tt.config)
			if result != tt.expected {
				t.Errorf("shouldSkipJSONLLineFromConfig() = %v, want %v", result, tt.expected)
			}
		})
	}
}

// ============================================================================
// PROCESSOR.StreamJSONL TESTS
// ============================================================================

func TestProcessor_StreamJSONL(t *testing.T) {
	processor, err := New()
	if err != nil {
		t.Fatalf("Failed to create processor: %v", err)
	}
	defer processor.Close()

	input := `{"name":"Alice"}
{"name":"Bob"}
{"name":"Charlie"}`

	var count int
	var names []string

	err = processor.StreamJSONL(strings.NewReader(input), func(lineNum int, item *IterableValue) error {
		count++
		names = append(names, item.GetString("name"))
		return nil
	})

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if count != 3 {
		t.Errorf("Expected 3 lines, got %d", count)
	}
	if len(names) != 3 || names[0] != "Alice" || names[1] != "Bob" || names[2] != "Charlie" {
		t.Errorf("Names mismatch: %v", names)
	}
}

func TestProcessor_StreamJSONL_EarlyStop(t *testing.T) {
	processor, err := New()
	if err != nil {
		t.Fatalf("Failed to create processor: %v", err)
	}
	defer processor.Close()

	input := "{\"a\":1}\n{\"b\":2}\n{\"c\":3}\n{\"d\":4}\n{\"e\":5}"

	var count int
	err = processor.StreamJSONL(strings.NewReader(input), func(lineNum int, item *IterableValue) error {
		count++
		if lineNum >= 3 {
			return item.Break() // Stop after 3 lines
		}
		return nil
	})

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if count != 3 {
		t.Errorf("Expected 3 lines processed, got %d", count)
	}
}

func TestProcessor_StreamJSONLParallel(t *testing.T) {
	processor, err := New()
	if err != nil {
		t.Fatalf("Failed to create processor: %v", err)
	}
	defer processor.Close()

	// Create input with 100 lines
	var lines []string
	for i := 0; i < 100; i++ {
		lines = append(lines, `{"id":`+string(rune('0'+i%10))+`}`)
	}
	input := strings.Join(lines, "\n")

	var count int32
	err = processor.StreamJSONLParallel(strings.NewReader(input), 4, func(lineNum int, item *IterableValue) error {
		atomic.AddInt32(&count, 1)
		return nil
	})

	if err != nil {
		t.Errorf("StreamJSONLParallel error: %v", err)
	}
	if count != 100 {
		t.Errorf("Expected 100 lines, got %d", count)
	}
}

func TestProcessor_StreamJSONLParallel_WithError(t *testing.T) {
	processor, err := New()
	if err != nil {
		t.Fatalf("Failed to create processor: %v", err)
	}
	defer processor.Close()

	input := "{\"a\":1}\n{\"b\":2}\n{\"c\":3}"

	err = processor.StreamJSONLParallel(strings.NewReader(input), 2, func(lineNum int, item *IterableValue) error {
		if lineNum == 2 {
			return errOperationFailed
		}
		return nil
	})

	if err == nil {
		t.Error("Expected error from worker")
	}
}

func TestProcessor_StreamJSONLChunked(t *testing.T) {
	processor, err := New()
	if err != nil {
		t.Fatalf("Failed to create processor: %v", err)
	}
	defer processor.Close()

	input := "{\"a\":1}\n{\"b\":2}\n{\"c\":3}\n{\"d\":4}\n{\"e\":5}"

	var chunkCalls int
	var totalItems int

	err = processor.StreamJSONLChunked(strings.NewReader(input), 2, func(chunk []*IterableValue) error {
		chunkCalls++
		totalItems += len(chunk)
		return nil
	})

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if chunkCalls != 3 { // 2+2+1
		t.Errorf("Expected 3 chunk calls, got %d", chunkCalls)
	}
	if totalItems != 5 {
		t.Errorf("Expected 5 total items, got %d", totalItems)
	}
}

func TestProcessor_ForeachJSONL(t *testing.T) {
	processor, err := New()
	if err != nil {
		t.Fatalf("Failed to create processor: %v", err)
	}
	defer processor.Close()

	input := `{"id":1}
{"id":2}
{"id":3}`

	var ids []int
	err = processor.ForeachJSONL(strings.NewReader(input), func(lineNum int, item *IterableValue) error {
		ids = append(ids, item.GetInt("id"))
		return nil
	})

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if len(ids) != 3 {
		t.Errorf("Expected 3 items, got %d", len(ids))
	}
}

func TestProcessor_MapJSONL(t *testing.T) {
	processor, err := New()
	if err != nil {
		t.Fatalf("Failed to create processor: %v", err)
	}
	defer processor.Close()

	input := `{"name":"Alice","age":30}
{"name":"Bob","age":25}`

	results, err := processor.MapJSONL(strings.NewReader(input), func(lineNum int, item *IterableValue) (any, error) {
		return map[string]any{
			"name":     item.GetString("name"),
			"ageYears": item.GetInt("age"),
		}, nil
	})

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("Expected 2 results, got %d", len(results))
	}
}

func TestProcessor_ReduceJSONL(t *testing.T) {
	processor, err := New()
	if err != nil {
		t.Fatalf("Failed to create processor: %v", err)
	}
	defer processor.Close()

	input := `{"value":10}
{"value":20}
{"value":30}`

	result, err := processor.ReduceJSONL(strings.NewReader(input), 0, func(acc any, item *IterableValue) any {
		return acc.(int) + item.GetInt("value")
	})

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if result.(int) != 60 {
		t.Errorf("Expected 60, got %v", result)
	}
}

func TestProcessor_FilterJSONL(t *testing.T) {
	processor, err := New()
	if err != nil {
		t.Fatalf("Failed to create processor: %v", err)
	}
	defer processor.Close()

	input := `{"age":25}
{"age":15}
{"age":30}
{"age":10}`

	results, err := processor.FilterJSONL(strings.NewReader(input), func(item *IterableValue) bool {
		return item.GetInt("age") >= 18
	})

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("Expected 2 adults, got %d", len(results))
	}
}

func TestProcessor_CollectJSONL(t *testing.T) {
	processor, err := New()
	if err != nil {
		t.Fatalf("Failed to create processor: %v", err)
	}
	defer processor.Close()

	input := `{"a":1}
{"b":2}
{"c":3}`

	items, err := processor.CollectJSONL(strings.NewReader(input))

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if len(items) != 3 {
		t.Errorf("Expected 3 items, got %d", len(items))
	}
}

func TestProcessor_FirstJSONL(t *testing.T) {
	processor, err := New()
	if err != nil {
		t.Fatalf("Failed to create processor: %v", err)
	}
	defer processor.Close()

	input := `{"name":"Alice","id":1}
{"name":"Bob","id":2}
{"name":"Charlie","id":3}`

	item, found, err := processor.FirstJSONL(strings.NewReader(input), func(item *IterableValue) bool {
		return item.GetString("name") == "Bob"
	})

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if !found {
		t.Error("Expected to find Bob")
	}
	if item.GetInt("id") != 2 {
		t.Errorf("Expected id 2, got %d", item.GetInt("id"))
	}
}

// ============================================================================
// JSONL WRITER TESTS
// ============================================================================

func TestNewJSONLWriter(t *testing.T) {
	var buf bytes.Buffer
	writer := NewJSONLWriter(&buf)

	if writer == nil {
		t.Fatal("NewJSONLWriter returned nil")
	}
}

func TestJSONLWriter_Write(t *testing.T) {
	tests := []struct {
		name     string
		data     any
		wantErr  bool
		checkOut func(t *testing.T, output string)
	}{
		{
			name:    "simple_map",
			data:    map[string]any{"key": "value"},
			wantErr: false,
			checkOut: func(t *testing.T, output string) {
				if !strings.Contains(output, `"key"`) || !strings.Contains(output, `"value"`) {
					t.Errorf("Output should contain key and value, got: %s", output)
				}
			},
		},
		{
			name:    "simple_array",
			data:    []any{1, 2, 3},
			wantErr: false,
			checkOut: func(t *testing.T, output string) {
				if !strings.Contains(output, "[") {
					t.Errorf("Output should be an array, got: %s", output)
				}
			},
		},
		{
			name:    "nested_object",
			data:    map[string]any{"inner": map[string]any{"deep": "value"}},
			wantErr: false,
			checkOut: func(t *testing.T, output string) {
				if !strings.Contains(output, "inner") || !strings.Contains(output, "deep") {
					t.Errorf("Output should contain nested keys, got: %s", output)
				}
			},
		},
		{
			name:    "primitive_string",
			data:    "hello",
			wantErr: false,
			checkOut: func(t *testing.T, output string) {
				if !strings.Contains(output, "hello") {
					t.Errorf("Output should contain string, got: %s", output)
				}
			},
		},
		{
			name:    "primitive_number",
			data:    42,
			wantErr: false,
			checkOut: func(t *testing.T, output string) {
				if !strings.Contains(output, "42") {
					t.Errorf("Output should contain number, got: %s", output)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			writer := NewJSONLWriter(&buf)

			err := writer.Write(tt.data)
			if tt.wantErr {
				if err == nil {
					t.Error("Expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				tt.checkOut(t, buf.String())
			}
		})
	}
}

func TestJSONLWriter_WriteAll(t *testing.T) {
	var buf bytes.Buffer
	writer := NewJSONLWriter(&buf)

	data := []any{
		map[string]any{"id": 1},
		map[string]any{"id": 2},
		map[string]any{"id": 3},
	}

	err := writer.WriteAll(data)
	if err != nil {
		t.Errorf("WriteAll error: %v", err)
	}

	output := buf.String()
	lines := strings.Count(output, "\n")
	if lines != 3 {
		t.Errorf("Expected 3 lines, got %d", lines)
	}
}

func TestJSONLWriter_WriteRaw(t *testing.T) {
	tests := []struct {
		name     string
		line     []byte
		expected string
	}{
		{
			name:     "with_newline",
			line:     []byte("{\"key\":\"value\"}\n"),
			expected: "{\"key\":\"value\"}\n",
		},
		{
			name:     "without_newline",
			line:     []byte("{\"key\":\"value\"}"),
			expected: "{\"key\":\"value\"}\n",
		},
		{
			name:     "empty_line",
			line:     []byte{},
			expected: "\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			writer := NewJSONLWriter(&buf)

			err := writer.WriteRaw(tt.line)
			if err != nil {
				t.Errorf("WriteRaw error: %v", err)
			}

			if buf.String() != tt.expected {
				t.Errorf("Output = %q, want %q", buf.String(), tt.expected)
			}
		})
	}
}

func TestJSONLWriter_Stats(t *testing.T) {
	var buf bytes.Buffer
	writer := NewJSONLWriter(&buf)

	_ = writer.Write(map[string]any{"a": 1})
	_ = writer.Write(map[string]any{"b": 2})
	_ = writer.WriteRaw([]byte("{\"c\":3}"))

	stats := writer.Stats()

	if stats.LinesProcessed != 3 {
		t.Errorf("LinesProcessed = %d, want 3", stats.LinesProcessed)
	}
}

func TestJSONLWriter_Err(t *testing.T) {
	var buf bytes.Buffer
	writer := NewJSONLWriter(&buf)

	// First write succeeds
	_ = writer.Write(map[string]any{"a": 1})

	// No error yet
	if writer.Err() != nil {
		t.Errorf("Err() = %v, want nil", writer.Err())
	}
}

// ============================================================================
// UTILITY FUNCTION TESTS
// ============================================================================

func TestParseJSONL(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		expectedCount int
		wantErr       bool
	}{
		{
			name:          "basic",
			input:         "{\"a\":1}\n{\"b\":2}\n{\"c\":3}",
			expectedCount: 3,
			wantErr:       false,
		},
		{
			name:          "with_empty_lines",
			input:         "{\"a\":1}\n\n{\"b\":2}",
			expectedCount: 2,
			wantErr:       false,
		},
		{
			name:          "empty_input",
			input:         "",
			expectedCount: 0,
			wantErr:       false,
		},
		{
			name:          "invalid_json",
			input:         "{\"a\":1}\n{invalid}\n{\"c\":3}",
			expectedCount: 0,
			wantErr:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results, err := ParseJSONL([]byte(tt.input))

			if tt.wantErr {
				if err == nil {
					t.Error("Expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if len(results) != tt.expectedCount {
					t.Errorf("Expected %d results, got %d", tt.expectedCount, len(results))
				}
			}
		})
	}
}

func TestToJSONL(t *testing.T) {
	tests := []struct {
		name          string
		data          []any
		expectedLines int
		wantErr       bool
	}{
		{
			name:          "basic",
			data:          []any{map[string]any{"a": 1}, map[string]any{"b": 2}},
			expectedLines: 2,
			wantErr:       false,
		},
		{
			name:          "empty",
			data:          []any{},
			expectedLines: 0,
			wantErr:       false,
		},
		{
			name:          "with_primitives",
			data:          []any{"hello", 42, true},
			expectedLines: 3,
			wantErr:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ToJSONL(tt.data)

			if tt.wantErr {
				if err == nil {
					t.Error("Expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}

				if tt.expectedLines == 0 {
					if len(result) != 0 {
						t.Errorf("Expected empty result, got %d bytes", len(result))
					}
				} else {
					lines := strings.Count(string(result), "\n")
					if lines != tt.expectedLines {
						t.Errorf("Expected %d lines, got %d", tt.expectedLines, lines)
					}
				}
			}
		})
	}
}

func TestToJSONLString(t *testing.T) {
	data := []any{
		map[string]any{"name": "Alice"},
		map[string]any{"name": "Bob"},
	}

	result, err := ToJSONLString(data)
	if err != nil {
		t.Fatalf("ToJSONLString error: %v", err)
	}

	lines := strings.Count(result, "\n")
	if lines != 2 {
		t.Errorf("Expected 2 lines, got %d", lines)
	}
}

// ============================================================================
// STREAM LINES INTO TESTS
// ============================================================================

func TestStreamLinesInto(t *testing.T) {
	type Item struct {
		ID    int    `json:"id"`
		Value string `json:"value"`
	}

	input := "{\"id\":1,\"value\":\"first\"}\n{\"id\":2,\"value\":\"second\"}"

	results, err := StreamLinesInto[Item](strings.NewReader(input), func(lineNum int, data Item) error {
		if data.ID <= 0 {
			return ErrInvalidJSON
		}
		return nil
	})

	if err != nil {
		t.Fatalf("StreamLinesInto error: %v", err)
	}

	if len(results) != 2 {
		t.Errorf("Expected 2 results, got %d", len(results))
	}
	if results[0].Value != "first" {
		t.Errorf("results[0].Value = %q, want %q", results[0].Value, "first")
	}
}

func TestStreamLinesInto_CallbackError(t *testing.T) {
	type Data struct {
		Value int `json:"value"`
	}

	input := "{\"value\":1}\n{\"value\":2}\n{\"value\":3}"

	_, err := StreamLinesInto[Data](strings.NewReader(input), func(lineNum int, data Data) error {
		if data.Value == 2 {
			return errOperationFailed
		}
		return nil
	})

	if err == nil {
		t.Error("Expected error from callback")
	}
}

// ============================================================================
// ITERABLEVALUE INTEGRATION TESTS
// ============================================================================

func TestStreamJSONL_IterableValueMethods(t *testing.T) {
	processor, err := New()
	if err != nil {
		t.Fatalf("Failed to create processor: %v", err)
	}
	defer processor.Close()

	input := `{"name":"Alice","age":30,"active":true,"score":95.5,"tags":["a","b"],"meta":{"city":"NYC"}}
{"name":"Bob","age":25,"active":false,"score":88.0,"tags":["c"],"meta":{"city":"LA"}}`

	var results []map[string]any

	err = processor.StreamJSONL(strings.NewReader(input), func(lineNum int, item *IterableValue) error {
		result := map[string]any{
			"name":   item.GetString("name"),
			"age":    item.GetInt("age"),
			"active": item.GetBool("active"),
			"score":  item.GetFloat64("score"),
			"city":   item.GetString("meta.city"),
			"tag0":   item.GetString("tags[0]"),
		}
		results = append(results, result)
		return nil
	})

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("Expected 2 results, got %d", len(results))
	}

	// Check first result
	if results[0]["name"] != "Alice" {
		t.Errorf("Expected name Alice, got %v", results[0]["name"])
	}
	if results[0]["age"] != 30 {
		t.Errorf("Expected age 30, got %v", results[0]["age"])
	}
	if results[0]["city"] != "NYC" {
		t.Errorf("Expected city NYC, got %v", results[0]["city"])
	}
}

// TestProcessor_FirstJSONL_NotFound tests FirstJSONL when no match exists
func TestProcessor_FirstJSONL_NotFound(t *testing.T) {
	processor, err := New()
	if err != nil {
		t.Fatalf("Failed to create processor: %v", err)
	}
	defer processor.Close()

	input := `{"name":"Alice"}
{"name":"Bob"}`

	item, found, err := processor.FirstJSONL(strings.NewReader(input), func(item *IterableValue) bool {
		return false // never match
	})

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if found {
		t.Error("Expected found=false when no match")
	}
	if item != nil {
		t.Errorf("Expected nil item, got %v", item)
	}
}

// TestProcessor_JSONLEdgeCases tests edge cases for JSONL methods
func TestProcessor_JSONLEdgeCases(t *testing.T) {
	processor, err := New()
	if err != nil {
		t.Fatalf("Failed to create processor: %v", err)
	}
	defer processor.Close()

	t.Run("empty input", func(t *testing.T) {
		err := processor.StreamJSONL(strings.NewReader(""), func(lineNum int, item *IterableValue) error {
			t.Error("should not be called on empty input")
			return nil
		})
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
	})

	t.Run("blank lines skipped", func(t *testing.T) {
		input := "\n\n{\"a\":1}\n\n{\"b\":2}\n\n"
		var count int
		err := processor.StreamJSONL(strings.NewReader(input), func(lineNum int, item *IterableValue) error {
			count++
			return nil
		})
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if count != 2 {
			t.Errorf("Expected 2 items, got %d", count)
		}
	})

	t.Run("malformed JSON returns error", func(t *testing.T) {
		input := "{\"valid\":1}\n{bad json}"
		err := processor.StreamJSONL(strings.NewReader(input), func(lineNum int, item *IterableValue) error {
			return nil
		})
		if err == nil {
			t.Error("Expected error for malformed JSON")
		}
	})

	t.Run("ForeachJSONL error propagation", func(t *testing.T) {
		input := "{\"a\":1}\n{\"a\":2}"
		err := processor.ForeachJSONL(strings.NewReader(input), func(lineNum int, item *IterableValue) error {
			return fmt.Errorf("stop at line %d", lineNum)
		})
		if err == nil || !strings.Contains(err.Error(), "stop at line") {
			t.Errorf("Expected propagated error, got %v", err)
		}
	})

	t.Run("CollectJSONL empty", func(t *testing.T) {
		items, err := processor.CollectJSONL(strings.NewReader(""))
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if len(items) != 0 {
			t.Errorf("Expected 0 items, got %d", len(items))
		}
	})

	t.Run("MapJSONL error", func(t *testing.T) {
		input := "{\"a\":1}"
		_, err := processor.MapJSONL(strings.NewReader(input), func(lineNum int, item *IterableValue) (any, error) {
			return nil, fmt.Errorf("map error")
		})
		if err == nil || !strings.Contains(err.Error(), "map error") {
			t.Errorf("Expected map error, got %v", err)
		}
	})
}

// ============================================================================
// Package-level JSONL delegation tests — consolidated from 10+ duplicates.
// Each TestPackageLevel_* previously duplicated the corresponding
// TestProcessor_* test. Now we verify delegation with a single table.
// ============================================================================

func TestPackageLevel_JSONLDelegation(t *testing.T) {
	input := `{"name":"Alice","age":30}
{"name":"Bob","age":25}`

	t.Run("StreamJSONL", func(t *testing.T) {
		var count int
		err := StreamJSONL(strings.NewReader(input), func(lineNum int, item *IterableValue) error {
			count++
			return nil
		})
		if err != nil {
			t.Fatalf("StreamJSONL error: %v", err)
		}
		if count != 2 {
			t.Errorf("count = %d, want 2", count)
		}
	})

	t.Run("ForeachJSONL", func(t *testing.T) {
		var count int
		err := ForeachJSONL(strings.NewReader(input), func(lineNum int, item *IterableValue) error {
			count++
			return nil
		})
		if err != nil {
			t.Fatalf("ForeachJSONL error: %v", err)
		}
		if count != 2 {
			t.Errorf("count = %d, want 2", count)
		}
	})

	t.Run("CollectJSONL", func(t *testing.T) {
		results, err := CollectJSONL(strings.NewReader(input))
		if err != nil {
			t.Fatalf("CollectJSONL error: %v", err)
		}
		if len(results) != 2 {
			t.Errorf("results = %d, want 2", len(results))
		}
	})

	t.Run("MapJSONL", func(t *testing.T) {
		results, err := MapJSONL(strings.NewReader(input), func(lineNum int, item *IterableValue) (any, error) {
			return item.GetData(), nil
		})
		if err != nil {
			t.Fatalf("MapJSONL error: %v", err)
		}
		if len(results) != 2 {
			t.Errorf("results = %d, want 2", len(results))
		}
	})

	t.Run("ReduceJSONL", func(t *testing.T) {
		result, err := ReduceJSONL(strings.NewReader(input), 0, func(acc any, item *IterableValue) any {
			return acc.(int) + item.GetInt("age")
		})
		if err != nil {
			t.Fatalf("ReduceJSONL error: %v", err)
		}
		if result != 55 {
			t.Errorf("result = %v, want 55", result)
		}
	})

	t.Run("FilterJSONL", func(t *testing.T) {
		results, err := FilterJSONL(strings.NewReader(input), func(item *IterableValue) bool {
			return item.GetInt("age") >= 30
		})
		if err != nil {
			t.Fatalf("FilterJSONL error: %v", err)
		}
		if len(results) != 1 {
			t.Errorf("results = %d, want 1", len(results))
		}
	})

	t.Run("FirstJSONL found", func(t *testing.T) {
		item, found, err := FirstJSONL(strings.NewReader(input), func(item *IterableValue) bool {
			return item.GetString("name") == "Bob"
		})
		if err != nil {
			t.Fatalf("FirstJSONL error: %v", err)
		}
		if !found || item == nil {
			t.Fatal("FirstJSONL should find Bob")
		}
	})

	t.Run("FirstJSONL not found", func(t *testing.T) {
		item, found, err := FirstJSONL(strings.NewReader(input), func(item *IterableValue) bool {
			return false
		})
		if err != nil {
			t.Fatalf("FirstJSONL error: %v", err)
		}
		if found || item != nil {
			t.Error("FirstJSONL should not find anything")
		}
	})

	t.Run("StreamJSONLParallel", func(t *testing.T) {
		var mu sync.Mutex
		var count int
		err := StreamJSONLParallel(strings.NewReader(input), 2, func(lineNum int, item *IterableValue) error {
			mu.Lock()
			count++
			mu.Unlock()
			return nil
		})
		if err != nil {
			t.Fatalf("StreamJSONLParallel error: %v", err)
		}
		if count != 2 {
			t.Errorf("count = %d, want 2", count)
		}
	})

	t.Run("StreamJSONLParallelWithContext", func(t *testing.T) {
		ctx := context.Background()
		var count atomic.Int32
		err := StreamJSONLParallelWithContext(ctx, strings.NewReader(input), 2, func(lineNum int, item *IterableValue) error {
			count.Add(1)
			return nil
		})
		if err != nil {
			t.Fatalf("StreamJSONLParallelWithContext error: %v", err)
		}
		if count.Load() != 2 {
			t.Errorf("count = %d, want 2", count.Load())
		}
	})

	t.Run("StreamJSONLChunked", func(t *testing.T) {
		var chunkCalls int
		err := StreamJSONLChunked(strings.NewReader(input), 1, func(chunk []*IterableValue) error {
			chunkCalls++
			return nil
		})
		if err != nil {
			t.Fatalf("StreamJSONLChunked error: %v", err)
		}
		if chunkCalls != 2 {
			t.Errorf("chunkCalls = %d, want 2", chunkCalls)
		}
	})
}

func TestPackageLevel_StreamJSONLFile(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/test.jsonl"
	content := "{\"x\":1}\n{\"x\":2}\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("WriteFile error: %v", err)
	}
	var count int
	err := StreamJSONLFile(path, func(lineNum int, item *IterableValue) error {
		count++
		return nil
	})
	if err != nil {
		t.Fatalf("StreamJSONLFile error: %v", err)
	}
	if count != 2 {
		t.Errorf("count = %d, want 2", count)
	}
}
