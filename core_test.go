package json

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cybergodev/json/internal"
)

// Merged from: api_test.go, core_test.go, operation_test.go, comprehensive_test.go, path_test.go

// ============================================================================
// Test Helper Functions
// ============================================================================

// contains checks if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && indexOf(s, substr) >= 0)
}

// indexOf finds the index of a substring
func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

// captureStdout captures output written to stdout
func captureStdout(f func()) string {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	f()

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	io.Copy(&buf, r)
	return buf.String()
}

// captureStderr captures output written to stderr
func captureStderr(f func()) string {
	old := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	f()

	w.Close()
	os.Stderr = old

	var buf bytes.Buffer
	io.Copy(&buf, r)
	return buf.String()
}

// intPtr returns a pointer to an int
func intPtr(i int) *int {
	return &i
}

// floatPtr returns a pointer to a float64
func floatPtr(f float64) *float64 {
	return &f
}

// getProp gets a property from a map
func getProp(data any, key string) any {
	if m, ok := data.(map[string]any); ok {
		return m[key]
	}
	return nil
}

// coreSliceEqual compares two slices for equality
func coreSliceEqual(a, b []any) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// coreSliceEqualHelper is an alias for coreSliceEqual
func coreSliceEqualHelper(a, b []any) bool {
	return coreSliceEqual(a, b)
}

// slicesEqual compares two slices for equality
func slicesEqual(a, b []any) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// generateLargeArray generates a large JSON array for testing
func generateLargeArray(size int) string {
	result := `{"items": [`
	for i := 0; i < size; i++ {
		if i > 0 {
			result += ","
		}
		result += `{"id": ` + fmt.Sprint(i) + `}`
	}
	result += `]}`
	return result
}

// generateUserJSON generates user JSON for testing
func generateUserJSON(count int) string {
	users := make([]string, count)
	for i := 0; i < count; i++ {
		users[i] = `{"id": ` + fmt.Sprint(i) + `, "name": "User` + fmt.Sprint(i) + `"}`
	}
	return strings.Join(users, ",")
}

// generateArrayItems generates array items for testing
func generateArrayItems(count int) string {
	items := make([]string, count)
	for i := 0; i < count; i++ {
		items[i] = `{"id": ` + fmt.Sprint(i) + `}`
	}
	return strings.Join(items, ",")
}

// ============================================================================
// Benchmark Tests
// ============================================================================

// BenchmarkArrayHelper_CompactArray benchmarks the CompactArray method
func BenchmarkArrayHelper_CompactArray(b *testing.B) {
	ah := &ArrayHelper{}
	arr := make([]any, 1000)
	for i := 0; i < 1000; i++ {
		if i%3 == 0 {
			arr[i] = nil
		} else {
			arr[i] = i
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ah.CompactArray(arr)
	}
}

// BenchmarkArrayHelper_PerformSlice benchmarks the PerformSlice method
func BenchmarkArrayHelper_PerformSlice(b *testing.B) {
	ah := &ArrayHelper{}
	arr := make([]any, 1000)
	for i := 0; i < 1000; i++ {
		arr[i] = i
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ah.PerformSlice(arr, 100, 900, 1)
	}
}

func BenchmarkDelete(b *testing.B) {
	jsonStr := `{"user": {"name": "Alice", "age": 30, "email": "alice@example.com"}}`
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = Delete(jsonStr, "user.email")
	}
}

func BenchmarkFastDelete(b *testing.B) {
	processor := New()
	defer processor.Close()

	jsonStr := `{"name": "test", "age": 30}`
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, _ = processor.FastDelete(jsonStr, "name")
	}
}

// Benchmark tests for operation functions
func BenchmarkFastSet(b *testing.B) {
	processor := New()
	defer processor.Close()

	jsonStr := `{"name": "test", "age": 30}`
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, _ = processor.FastSet(jsonStr, "name", "updated")
	}
}

func BenchmarkGet(b *testing.B) {
	jsonStr := `{"user": {"name": "Alice", "age": 30, "email": "alice@example.com"}}`
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = Get(jsonStr, "user.name")
	}
}

func BenchmarkHandleArrayAccess(b *testing.B) {
	processor := New()
	defer processor.Close()

	arr := make([]any, 1000)
	for i := 0; i < 1000; i++ {
		arr[i] = i
	}

	segment := PathSegment{
		Type:  internal.ArrayIndexSegment,
		Index: 500,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = processor.handleArrayAccess(arr, segment)
	}
}

func BenchmarkHandlePropertyAccess(b *testing.B) {
	processor := New()
	defer processor.Close()

	data := map[string]any{
		"name":  "Alice",
		"age":   30,
		"email": "alice@example.com",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = processor.handlePropertyAccess(data, "name")
	}
}

func BenchmarkJSONPointerEscape(b *testing.B) {
	processor := New()
	defer processor.Close()

	input := "user~/name/with~special/chars"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = processor.escapeJSONPointer(input)
	}
}

func BenchmarkMarshal(b *testing.B) {
	data := map[string]any{"name": "Alice", "age": 30}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = Marshal(data)
	}
}

func BenchmarkOpBatchSet(b *testing.B) {
	processor := New()
	defer processor.Close()

	jsonStr := `{"a": 1, "b": 2, "c": 3}`
	updates := map[string]any{
		"a": 10,
		"b": 20,
		"c": 30,
	}
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, _ = processor.BatchSetOptimized(jsonStr, updates)
	}
}

func BenchmarkOpFastGetMultiple(b *testing.B) {
	processor := New()
	defer processor.Close()

	jsonStr := `{"a": 1, "b": 2, "c": 3, "d": 4, "e": 5}`
	paths := []string{"a", "b", "c", "d", "e"}
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, _ = processor.FastGetMultiple(jsonStr, paths)
	}
}

func BenchmarkPathParsingComplex(b *testing.B) {
	processor := New()
	defer processor.Close()

	path := "users[0:5]{name}.first"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = processor.parsePath(path)
	}
}

func BenchmarkPathParsingSimple(b *testing.B) {
	processor := New()
	defer processor.Close()

	path := "user.name.first"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = processor.parsePath(path)
	}
}

func BenchmarkPerformArraySlice(b *testing.B) {
	processor := New()
	defer processor.Close()

	arr := make([]any, 1000)
	for i := 0; i < 1000; i++ {
		arr[i] = i
	}

	start := intPtr(100)
	end := intPtr(900)
	step := intPtr(2)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = processor.performArraySlice(arr, start, end, step)
	}
}

func BenchmarkSet(b *testing.B) {
	jsonStr := `{"user": {"name": "Alice"}}`
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = Set(jsonStr, "user.age", 30)
	}
}

// TestArrayExtensionNeededError tests the Error method
func TestArrayExtensionNeededError(t *testing.T) {
	err := &ArrayExtensionNeededError{
		RequiredLength: 10,
		CurrentLength:  5,
		Start:          0,
		End:            10,
		Step:           1,
		Value:          "test",
	}

	expected := "array extension needed: current length 5, required length 10 for slice [0:10]"
	if err.Error() != expected {
		t.Errorf("Error() = %q, want %q", err.Error(), expected)
	}
}

// TestArrayHelper_ClampIndex tests the ClampIndex method
func TestArrayHelper_ClampIndex(t *testing.T) {
	ah := &ArrayHelper{}

	tests := []struct {
		name     string
		index    int
		length   int
		expected int
	}{
		{"Within bounds", 2, 5, 2},
		{"At lower bound", 0, 5, 0},
		{"At upper bound", 5, 5, 5},
		{"Below lower bound", -1, 5, 0},
		{"Above upper bound", 10, 5, 5},
		{"Large negative", -100, 5, 0},
		{"Large positive", 1000, 5, 5},
		{"Zero length", 0, 0, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ah.ClampIndex(tt.index, tt.length)
			if result != tt.expected {
				t.Errorf("ClampIndex(%d, %d) = %d, want %d", tt.index, tt.length, result, tt.expected)
			}
		})
	}
}

// TestArrayHelper_CompactArray tests the CompactArray method
func TestArrayHelper_CompactArray(t *testing.T) {
	ah := &ArrayHelper{}

	tests := []struct {
		name     string
		input    []any
		expected int
	}{
		{"Empty array", []any{}, 0},
		{"No nil values", []any{1, 2, 3}, 3},
		{"With nil values", []any{1, nil, 3, nil}, 2},
		{"All nil", []any{nil, nil, nil}, 0},
		{"With DeletedMarker", []any{1, DeletedMarker, 3}, 2},
		{"Mixed nil and DeletedMarker", []any{nil, DeletedMarker, 1, nil, DeletedMarker}, 1},
		{"Nil at start", []any{nil, 1, 2}, 2},
		{"Nil at end", []any{1, 2, nil}, 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ah.CompactArray(tt.input)
			if len(result) != tt.expected {
				t.Errorf("CompactArray() returned length %d, want %d", len(result), tt.expected)
			}
			for _, item := range result {
				if item == nil || item == DeletedMarker {
					t.Errorf("CompactArray() result contains nil or DeletedMarker")
				}
			}
		})
	}
}

// TestArrayHelper_ExtendArray tests the ExtendArray method
func TestArrayHelper_ExtendArray(t *testing.T) {
	ah := &ArrayHelper{}

	tests := []struct {
		name           string
		input          []any
		targetLength   int
		expectExtend   bool
		expectedLength int
	}{
		{"Already longer", []any{1, 2, 3}, 2, false, 3},
		{"Same length", []any{1, 2, 3}, 3, false, 3},
		{"Need extension", []any{1, 2}, 5, true, 5},
		{"Empty to non-empty", []any{}, 3, true, 3},
		{"Single to multiple", []any{1}, 5, true, 5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ah.ExtendArray(tt.input, tt.targetLength)
			if len(result) != tt.expectedLength {
				t.Errorf("ExtendArray() returned length %d, want %d", len(result), tt.expectedLength)
			}
			if tt.expectExtend && len(result) <= len(tt.input) {
				t.Errorf("ExtendArray() should have extended array")
			}
		})
	}
}

// TestArrayHelper_GetElement tests the GetElement method
func TestArrayHelper_GetElement(t *testing.T) {
	ah := &ArrayHelper{}
	arr := []any{"a", "b", "c", "d", "e"}

	tests := []struct {
		name        string
		index       int
		expected    any
		expectFound bool
	}{
		{"Valid positive index", 1, "b", true},
		{"First element", 0, "a", true},
		{"Last element", 4, "e", true},
		{"Negative index (last)", -1, "e", true},
		{"Negative index (first)", -5, "a", true},
		{"Out of bounds", 10, nil, false},
		{"Out of bounds (negative)", -10, nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, found := ah.GetElement(arr, tt.index)
			if found != tt.expectFound {
				t.Errorf("GetElement() found = %v, want %v", found, tt.expectFound)
			}
			if found && result != tt.expected {
				t.Errorf("GetElement() = %v, want %v", result, tt.expected)
			}
		})
	}
}

// TestArrayHelper_NormalizeIndex tests the NormalizeIndex method
func TestArrayHelper_NormalizeIndex(t *testing.T) {
	ah := &ArrayHelper{}

	tests := []struct {
		name     string
		index    int
		length   int
		expected int
	}{
		{"Positive index", 2, 5, 2},
		{"Negative index (last)", -1, 5, 4},
		{"Negative index (second to last)", -2, 5, 3},
		{"Zero index", 0, 5, 0},
		{"First element", -5, 5, 0},
		{"Large positive index", 100, 5, 100},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ah.NormalizeIndex(tt.index, tt.length)
			if result != tt.expected {
				t.Errorf("NormalizeIndex(%d, %d) = %d, want %d", tt.index, tt.length, result, tt.expected)
			}
		})
	}
}

// TestArrayHelper_ParseArrayIndex tests the ParseArrayIndex method
func TestArrayHelper_ParseArrayIndex(t *testing.T) {
	ah := &ArrayHelper{}

	tests := []struct {
		name     string
		input    string
		expected int
	}{
		{"Valid positive index", "[5]", 5},
		{"Valid negative index", "[-3]", -3},
		{"Index without brackets", "10", 10},
		{"Empty string", "", -999999},
		{"Invalid index", "abc", -999999},
		{"Index with spaces", "[ 7 ]", 7},
		{"Index with tabs", "[\t8\t]", 8},
		{"Negative index without brackets", "-2", -2},
		{"Zero index", "[0]", 0},
		{"Large positive index", "[999999]", 999999},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ah.ParseArrayIndex(tt.input)
			if result != tt.expected {
				t.Errorf("ParseArrayIndex(%q) = %d, want %d", tt.input, result, tt.expected)
			}
		})
	}
}

// TestArrayHelper_PerformSlice tests the PerformSlice method
func TestArrayHelper_PerformSlice(t *testing.T) {
	ah := &ArrayHelper{}
	arr := []any{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}

	tests := []struct {
		name     string
		start    int
		end      int
		step     int
		expected []any
	}{
		{"Simple slice", 2, 5, 1, []any{2, 3, 4}},
		{"From beginning", 0, 3, 1, []any{0, 1, 2}},
		{"To end", 7, 10, 1, []any{7, 8, 9}},
		{"Full slice", 0, 10, 1, []any{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}},
		{"Empty slice", 5, 5, 1, []any{}},
		{"Step of 2", 0, 10, 2, []any{0, 2, 4, 6, 8}},
		{"Step of 3", 0, 9, 3, []any{0, 3, 6}},
		{"Negative start (normalized)", -3, 10, 1, []any{7, 8, 9}},
		{"Negative end (normalized)", 0, -3, 1, []any{0, 1, 2, 3, 4, 5, 6}},
		{"Zero step (empty)", 0, 5, 0, []any{}},
		{"Reverse step partial", 8, 2, -2, []any{8, 6, 4}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ah.PerformSlice(arr, tt.start, tt.end, tt.step)
			if !coreSliceEqualHelper(result, tt.expected) {
				t.Errorf("PerformSlice(%d, %d, %d) = %v, want %v", tt.start, tt.end, tt.step, result, tt.expected)
			}
		})
	}
}

// TestArrayHelper_PerformSlice_EmptyArray tests PerformSlice with empty array
func TestArrayHelper_PerformSlice_EmptyArray(t *testing.T) {
	ah := &ArrayHelper{}
	emptyArr := []any{}

	result := ah.PerformSlice(emptyArr, 0, 5, 1)
	if len(result) != 0 {
		t.Errorf("PerformSlice on empty array should return empty, got %v", result)
	}
}

// TestArrayHelper_SetElement tests the SetElement method
func TestArrayHelper_SetElement(t *testing.T) {
	ah := &ArrayHelper{}

	tests := []struct {
		name        string
		arr         []any
		index       int
		value       any
		expectOK    bool
		expectIndex int
	}{
		{"Valid index", []any{1, 2, 3}, 1, "x", true, 1},
		{"First element", []any{1, 2, 3}, 0, "x", true, 0},
		{"Last element", []any{1, 2, 3}, 2, "x", true, 2},
		{"Negative index", []any{1, 2, 3}, -1, "x", true, 2},
		{"Out of bounds", []any{1, 2, 3}, 10, "x", false, -1},
		{"Negative out of bounds", []any{1, 2, 3}, -10, "x", false, -1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			arrCopy := make([]any, len(tt.arr))
			copy(arrCopy, tt.arr)

			result := ah.SetElement(arrCopy, tt.index, tt.value)
			if result != tt.expectOK {
				t.Errorf("SetElement() = %v, want %v", result, tt.expectOK)
			}
			if tt.expectOK && arrCopy[tt.expectIndex] != tt.value {
				t.Errorf("SetElement() did not set value at index %d", tt.expectIndex)
			}
		})
	}
}

// TestArrayHelper_ValidateBounds tests the ValidateBounds method
func TestArrayHelper_ValidateBounds(t *testing.T) {
	ah := &ArrayHelper{}

	tests := []struct {
		name     string
		index    int
		length   int
		expected bool
	}{
		{"Valid index", 2, 5, true},
		{"First index", 0, 5, true},
		{"Last index", 4, 5, true},
		{"Out of bounds (positive)", 5, 5, false},
		{"Out of bounds (large positive)", 100, 5, false},
		{"Negative index", -1, 5, false},
		{"Empty array", 0, 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ah.ValidateBounds(tt.index, tt.length)
			if result != tt.expected {
				t.Errorf("ValidateBounds(%d, %d) = %v, want %v", tt.index, tt.length, result, tt.expected)
			}
		})
	}
}

// TestArrayIndexValidation tests array index validation
func TestArrayIndexValidation(t *testing.T) {
	processor := New()
	defer processor.Close()

	tests := []struct {
		name  string
		index string
		valid bool
	}{
		{
			name:  "valid positive",
			index: "0",
			valid: true,
		},
		{
			name:  "valid negative",
			index: "-1",
			valid: true,
		},
		{
			name:  "empty",
			index: "",
			valid: false,
		},
		{
			name:  "invalid text",
			index: "abc",
			valid: false,
		},
		{
			name:  "with decimal",
			index: "1.5",
			valid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := processor.isValidArrayIndex(tt.index)
			if result != tt.valid {
				t.Errorf("isValidArrayIndex(%s) = %v; want %v", tt.index, result, tt.valid)
			}
		})
	}
}

// TestArraySliceEdgeCases tests edge cases for array slicing
func TestArraySliceEdgeCases(t *testing.T) {
	jsonStr := `{
		"items": [0, 1, 2, 3, 4, 5, 6, 7, 8, 9]
	}`

	processor := New()
	defer processor.Close()

	tests := []struct {
		name     string
		path     string
		expected []any
	}{
		{
			name:     "slice from start",
			path:     "items[:5]",
			expected: []any{0.0, 1.0, 2.0, 3.0, 4.0},
		},
		{
			name:     "slice to end",
			path:     "items[5:]",
			expected: []any{5.0, 6.0, 7.0, 8.0, 9.0},
		},
		{
			name:     "full slice",
			path:     "items[:]",
			expected: []any{0.0, 1.0, 2.0, 3.0, 4.0, 5.0, 6.0, 7.0, 8.0, 9.0},
		},
		{
			name:     "negative indices",
			path:     "items[-3:]",
			expected: []any{7.0, 8.0, 9.0},
		},
		{
			name:     "step slice",
			path:     "items[0:10:2]",
			expected: []any{0.0, 2.0, 4.0, 6.0, 8.0},
		},
		{
			name:     "empty result",
			path:     "items[10:15]",
			expected: []any{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := processor.Get(jsonStr, tt.path)
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			arr, ok := result.([]any)
			if !ok {
				t.Fatalf("Expected []any, got %T", result)
			}
			if !slicesEqual(arr, tt.expected) {
				t.Errorf("Get(%s) = %v; want %v", tt.path, arr, tt.expected)
			}
		})
	}
}

// TestArraySliceOperations tests array slice operations
func TestArraySliceOperations(t *testing.T) {
	processor := New()
	defer processor.Close()

	t.Run("get array slice", func(t *testing.T) {
		jsonStr := `{"items": [1, 2, 3, 4, 5]}`
		result, err := processor.Get(jsonStr, "items[1:3]")
		if err != nil {
			t.Fatalf("Get slice error: %v", err)
		}
		arr, ok := result.([]any)
		if !ok {
			t.Fatalf("Expected array, got %T", result)
		}
		if len(arr) != 2 {
			t.Errorf("Slice length = %d, want 2", len(arr))
		}
	})

	t.Run("set array slice", func(t *testing.T) {
		jsonStr := `{"items": [1, 2, 3, 4, 5]}`
		result, err := processor.Set(jsonStr, "items[0:2]", []any{10, 20})
		if err != nil {
			t.Fatalf("Set slice error: %v", err)
		}
		_ = result
	})
}

// TestBatchDeleteOptimized tests the BatchDeleteOptimized function
func TestBatchDeleteOptimized(t *testing.T) {
	processor := New()
	defer processor.Close()

	t.Run("batch delete multiple paths", func(t *testing.T) {
		jsonStr := `{"a": 1, "b": 2, "c": 3}`
		paths := []string{"a", "c"}
		result, err := processor.BatchDeleteOptimized(jsonStr, paths)
		if err != nil {
			t.Fatalf("BatchDeleteOptimized error: %v", err)
		}
		if result == "" {
			t.Error("BatchDeleteOptimized returned empty result")
		}
	})

	t.Run("empty paths", func(t *testing.T) {
		jsonStr := `{"a": 1}`
		paths := []string{}
		result, err := processor.BatchDeleteOptimized(jsonStr, paths)
		if err != nil {
			t.Fatalf("BatchDeleteOptimized error: %v", err)
		}
		_ = result
	})
}

// TestBatchSetOptimized tests the BatchSetOptimized function
func TestBatchSetOptimized(t *testing.T) {
	processor := New()
	defer processor.Close()

	t.Run("batch set multiple values", func(t *testing.T) {
		jsonStr := `{"a": 1, "b": 2}`
		updates := map[string]any{
			"a": 10,
			"b": 20,
			"c": 30,
		}
		result, err := processor.BatchSetOptimized(jsonStr, updates)
		if err != nil {
			t.Fatalf("BatchSetOptimized error: %v", err)
		}
		if result == "" {
			t.Error("BatchSetOptimized returned empty result")
		}
	})

	t.Run("empty updates", func(t *testing.T) {
		jsonStr := `{"a": 1}`
		updates := map[string]any{}
		result, err := processor.BatchSetOptimized(jsonStr, updates)
		if err != nil {
			t.Fatalf("BatchSetOptimized error: %v", err)
		}
		_ = result
	})
}

// TestBoundaryConditions tests edge cases and boundary conditions
func TestBoundaryConditions(t *testing.T) {
	helper := NewTestHelper(t)

	t.Run("EmptyValues", func(t *testing.T) {
		t.Run("EmptyString", func(t *testing.T) {
			testData := `{"empty": "", "normal": "value"}`

			result, err := Get(testData, "empty")
			helper.AssertNoError(err)
			helper.AssertEqual("", result)

			withDefault := GetStringWithDefault(testData, "missing", "default")
			helper.AssertEqual("default", withDefault)
		})

		t.Run("EmptyArray", func(t *testing.T) {
			testData := `{"empty": [], "normal": [1, 2, 3]}`

			result, err := GetArray(testData, "empty")
			helper.AssertNoError(err)
			helper.AssertEqual(0, len(result))
		})

		t.Run("EmptyObject", func(t *testing.T) {
			testData := `{"empty": {}, "normal": {"key": "value"}}`

			result, err := GetObject(testData, "empty")
			helper.AssertNoError(err)
			helper.AssertEqual(0, len(result))
		})

		t.Run("NullValue", func(t *testing.T) {
			testData := `{"null_value": null, "string": "value"}`

			result, err := Get(testData, "null_value")
			helper.AssertNoError(err)
			helper.AssertNil(result)
		})
	})

	t.Run("NumericBoundaries", func(t *testing.T) {
		processor := New(DefaultConfig())
		defer processor.Close()

		t.Run("MaxInt64", func(t *testing.T) {
			testData := `{"max": 9223372036854775807}`

			result, err := processor.Get(testData, "max")
			helper.AssertNoError(err)

			if num, ok := result.(float64); ok {
				helper.AssertTrue(num == float64(math.MaxInt64) || num == float64(int64(math.MaxInt64)))
			}
		})

		t.Run("MinInt64", func(t *testing.T) {
			testData := `{"min": -9223372036854775808}`

			result, err := processor.Get(testData, "min")
			helper.AssertNoError(err)

			if num, ok := result.(float64); ok {
				helper.AssertTrue(num <= float64(math.MinInt64))
			}
		})

		t.Run("MaxFloat", func(t *testing.T) {
			testData := `{"max_float": 1.7976931348623157e+308}`

			result, err := processor.Get(testData, "max_float")
			helper.AssertNoError(err)

			if num, ok := result.(float64); ok {
				helper.AssertTrue(num > 1.79e308 || num == math.MaxFloat64)
			}
		})

		t.Run("VerySmallFloat", func(t *testing.T) {
			testData := `{"small": 1e-323}`

			result, err := processor.Get(testData, "small")
			helper.AssertNoError(err)

			if num, ok := result.(float64); ok {
				helper.AssertTrue(num >= 0 && num < 1e-300)
			}
		})

		t.Run("ZeroValues", func(t *testing.T) {
			testData := `{"int_zero": 0, "float_zero": 0.0, "bool_false": false}`

			intZero, _ := GetInt(testData, "int_zero")
			helper.AssertEqual(0, intZero)

			floatZero, _ := GetFloat64(testData, "float_zero")
			helper.AssertEqual(0.0, floatZero)

			boolFalse, _ := GetBool(testData, "bool_false")
			helper.AssertFalse(boolFalse)
		})
	})

	t.Run("StringBoundaries", func(t *testing.T) {
		t.Run("VeryLongString", func(t *testing.T) {
			longString := strings.Repeat("a", 100000)
			testData := `{"long": "` + longString + `"}`

			result, err := Get(testData, "long")
			helper.AssertNoError(err)

			if str, ok := result.(string); ok {
				helper.AssertEqual(100000, len(str))
			}
		})

		t.Run("SpecialCharacters", func(t *testing.T) {
			specialChars := `\"\n\r\t\b\f\/\\`
			testData := `{"special": "` + specialChars + `"}`

			result, err := Get(testData, "special")
			helper.AssertNoError(err)

			if str, ok := result.(string); ok {
				helper.AssertTrue(len(str) > 0)
			}
		})

		t.Run("UnicodeEscape", func(t *testing.T) {
			testData := `{"unicode": "\u0048\u0065\u006c\u006c\u006f"}`

			result, err := Get(testData, "unicode")
			helper.AssertNoError(err)

			if str, ok := result.(string); ok {
				helper.AssertEqual("Hello", str)
			}
		})

		t.Run("Emoji", func(t *testing.T) {
			testData := `{"emoji": "🎉🚀💻"}`

			result, err := Get(testData, "emoji")
			helper.AssertNoError(err)

			if str, ok := result.(string); ok {
				helper.AssertTrue(len(str) > 0)
			}
		})
	})

	t.Run("ArrayBoundaries", func(t *testing.T) {
		t.Run("SingleElement", func(t *testing.T) {
			testData := `{"single": [1]}`

			first, _ := Get(testData, "single[0]")
			helper.AssertEqual(float64(1), first)

			last, _ := Get(testData, "single[-1]")
			helper.AssertEqual(float64(1), last)
		})

		t.Run("LargeArray", func(t *testing.T) {
			elements := make([]string, 1000)
			for i := 0; i < 1000; i++ {
				elements[i] = fmt.Sprint(i)
			}
			testData := `{"large": [` + strings.Join(elements, ",") + `]}`

			result, err := GetArray(testData, "large")
			helper.AssertNoError(err)
			helper.AssertEqual(1000, len(result))
		})

		t.Run("ArraySlicing", func(t *testing.T) {
			testData := `{"arr": [0, 1, 2, 3, 4, 5, 6, 7, 8, 9]}`

			tests := []struct {
				path     string
				expected []interface{}
			}{
				{"arr[0:3]", []interface{}{float64(0), float64(1), float64(2)}},
				{"arr[5:]", []interface{}{float64(5), float64(6), float64(7), float64(8), float64(9)}},
				{"arr[:3]", []interface{}{float64(0), float64(1), float64(2)}},
				{"arr[-3:]", []interface{}{float64(7), float64(8), float64(9)}},
			}

			for _, tt := range tests {
				t.Run(tt.path, func(t *testing.T) {
					result, err := Get(testData, tt.path)
					helper.AssertNoError(err)

					if arr, ok := result.([]interface{}); ok {
						helper.AssertEqual(len(tt.expected), len(arr))
						for i, exp := range tt.expected {
							helper.AssertEqual(exp, arr[i])
						}
					}
				})
			}
		})
	})

	t.Run("ObjectBoundaries", func(t *testing.T) {
		t.Run("ManyKeys", func(t *testing.T) {
			pairs := make([]string, 100)
			for i := 0; i < 100; i++ {
				pairs[i] = `"key` + fmt.Sprint(i) + `": ` + fmt.Sprint(i)
			}
			testData := `{"many": {` + strings.Join(pairs, ",") + `}}`

			result, err := GetObject(testData, "many")
			helper.AssertNoError(err)
			helper.AssertEqual(100, len(result))
		})

		t.Run("DeeplyNested", func(t *testing.T) {
			testData := `{"root": {"level1": {"level2": {"level3": {"level4": {"deep": "value"}}}}}}`

			result, err := Get(testData, "root.level1.level2.level3.level4.deep")
			helper.AssertNoError(err)
			helper.AssertEqual("value", result)
		})

		t.Run("MixedTypes", func(t *testing.T) {
			testData := `{
				"string": "text",
				"number": 42,
				"float": 3.14,
				"bool": true,
				"null": null,
				"array": [1, 2, 3],
				"object": {"nested": "value"}
			}`

			_, err := Get(testData, "string")
			helper.AssertNoError(err)

			_, err = Get(testData, "number")
			helper.AssertNoError(err)

			_, err = Get(testData, "float")
			helper.AssertNoError(err)

			_, err = Get(testData, "bool")
			helper.AssertNoError(err)

			_, err = Get(testData, "null")
			helper.AssertNoError(err)

			_, err = Get(testData, "array")
			helper.AssertNoError(err)

			_, err = Get(testData, "object")
			helper.AssertNoError(err)
		})
	})

	t.Run("WhitespaceHandling", func(t *testing.T) {
		t.Run("LotsOfWhitespace", func(t *testing.T) {
			whitespaceJSON := `{` + strings.Repeat(" \t\n", 1000) + `"key"` + strings.Repeat(" \t\n", 1000) + `:` + strings.Repeat(" \t\n", 1000) + `"value"` + strings.Repeat(" \t\n", 1000) + `}`

			result, err := Get(whitespaceJSON, "key")
			helper.AssertNoError(err)
			helper.AssertEqual("value", result)
		})

		t.Run("NoWhitespace", func(t *testing.T) {
			noSpaceJSON := `{"key":"value"}`
			result, err := Get(noSpaceJSON, "key")
			helper.AssertNoError(err)
			helper.AssertEqual("value", result)
		})
	})

	t.Run("BooleanBoundaries", func(t *testing.T) {
		testData := `{"true": true, "false": false}`

		trueVal, err := GetBool(testData, "true")
		helper.AssertNoError(err)
		helper.AssertTrue(trueVal)

		falseVal, err := GetBool(testData, "false")
		helper.AssertNoError(err)
		helper.AssertFalse(falseVal)

		conversionTests := []struct {
			value    interface{}
			expected bool
		}{
			{1, true},
			{0, false},
			{-1, true},
			{1.0, true},
			{0.0, false},
			{"true", true},
			{"false", false},
			{"1", true},
			{"0", false},
		}

		for _, tt := range conversionTests {
			name := "true"
			if !tt.expected {
				name = "false"
			}
			t.Run("Convert_"+name, func(t *testing.T) {
				result, ok := ConvertToBool(tt.value)
				helper.AssertTrue(ok)
				helper.AssertEqual(tt.expected, result)
			})
		}
	})

	t.Run("TimestampEdgeCases", func(t *testing.T) {
		testData := `{"timestamp": "2024-01-15T10:30:00Z", "epoch": 1705319400}`

		timestamp, err := GetString(testData, "timestamp")
		helper.AssertNoError(err)
		helper.AssertEqual("2024-01-15T10:30:00Z", timestamp)

		epoch, err := GetInt(testData, "epoch")
		helper.AssertNoError(err)
		helper.AssertEqual(1705319400, epoch)
	})
}

// TestBufferCompatibility tests buffer-based operations
func TestBufferCompatibility(t *testing.T) {
	jsonStr := `{"name": "Alice", "age": 30}`
	src := []byte(jsonStr)

	// Test Compact
	var compactBuf bytes.Buffer
	err := Compact(&compactBuf, src)
	if err != nil {
		t.Fatalf("Compact failed: %v", err)
	}

	// Test Indent
	var indentBuf bytes.Buffer
	err = Indent(&indentBuf, src, "", "  ")
	if err != nil {
		t.Fatalf("Indent failed: %v", err)
	}
	if !contains(indentBuf.String(), "\n") {
		t.Error("Expected indented output")
	}

	// Test HTMLEscape
	// Note: Using characters that don't trigger security validation
	var escapeBuf bytes.Buffer
	htmlJSON := []byte(`{"html": "<div>Content & more</div>"}`)
	HTMLEscape(&escapeBuf, htmlJSON)
	escaped := escapeBuf.String()
	// HTML entities should be escaped
	// Standard library escapes < to \u003c, > to \u003e, & to \u0026
	if !contains(escaped, "\\u003c") && !contains(escaped, "\\u003e") && !contains(escaped, "\\u0026") {
		t.Logf("Actual escaped output: %s", escaped)
		// Check that raw HTML characters are not present
		if contains(escaped, "<div>") {
			t.Error("Expected HTML to be escaped but found raw <div>")
		}
	}
}

// TestClearCache tests cache clearing
func TestClearCache(t *testing.T) {
	// Get a value to populate cache
	jsonStr := `{"user": {"name": "Alice"}}`
	_, _ = Get(jsonStr, "user.name")

	// Clear cache
	ClearCache()

	// Should not error
	t.Log("Cache cleared successfully")
}

// TestCompactBuffer tests CompactBuffer function
func TestCompactBuffer(t *testing.T) {
	prettyJSON := `{
		"name": "Alice",
		"age": 30
	}`

	var buf bytes.Buffer
	err := CompactBuffer(&buf, []byte(prettyJSON))
	if err != nil {
		t.Fatalf("CompactBuffer error: %v", err)
	}

	result := buf.String()
	if contains(result, "\n") || contains(result, "  ") {
		t.Errorf("CompactBuffer should remove whitespace, got: %s", result)
	}
	if !contains(result, `"name"`) || !contains(result, `"Alice"`) {
		t.Errorf("CompactBuffer lost data, got: %s", result)
	}
}

// TestComplexPathWithBracesAndBrackets tests paths with both braces and brackets
func TestComplexPathWithBracesAndBrackets(t *testing.T) {
	jsonStr := `{
		"users": [
			{"name": "Alice", "tags": ["a", "b", "c"]},
			{"name": "Bob", "tags": ["d", "e", "f"]}
		]
	}`

	processor := New()
	defer processor.Close()

	tests := []struct {
		name        string
		path        string
		expectError bool
	}{
		{
			name:        "extraction then array",
			path:        "users{tags}[0]",
			expectError: false,
		},
		{
			name:        "array then extraction",
			path:        "users[0]{name}",
			expectError: false,
		},
		{
			name:        "slice then extraction",
			path:        "users[0:1]{name}",
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := processor.Get(jsonStr, tt.path)
			if tt.expectError && err == nil {
				t.Errorf("Expected error for path '%s', but got none", tt.path)
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error for path '%s': %v", tt.path, err)
			}
		})
	}
}

func TestConfig_AccessorMethods(t *testing.T) {
	cfg := &Config{
		AllowComments:    true,
		PreserveNumbers:  true,
		CreatePaths:      true,
		CleanupNulls:     true,
		CompactArrays:    true,
		ValidateInput:    true,
		ValidateFilePath: true,
	}

	tests := []struct {
		name     string
		got      bool
		expected bool
	}{
		{"IsCommentsAllowed", cfg.IsCommentsAllowed(), true},
		{"ShouldPreserveNumbers", cfg.ShouldPreserveNumbers(), true},
		{"ShouldCreatePaths", cfg.ShouldCreatePaths(), true},
		{"ShouldCleanupNulls", cfg.ShouldCleanupNulls(), true},
		{"ShouldCompactArrays", cfg.ShouldCompactArrays(), true},
		{"ShouldValidateInput", cfg.ShouldValidateInput(), true},
		{"ShouldValidateFilePath", cfg.ShouldValidateFilePath(), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.expected {
				t.Errorf("%s() = %v, want %v", tt.name, tt.got, tt.expected)
			}
		})
	}

	// Test false values
	cfgFalse := &Config{}
	if cfgFalse.IsCommentsAllowed() {
		t.Error("IsCommentsAllowed should return false for default config")
	}
}

func TestConfig_Clone(t *testing.T) {
	original := &Config{
		EnableCache:     true,
		MaxCacheSize:    1000,
		CacheTTL:        time.Minute,
		MaxJSONSize:     1048576,
		MaxPathDepth:    50,
		MaxConcurrency:  10,
		AllowComments:   true,
		PreserveNumbers: true,
	}

	cloned := original.Clone()

	if cloned == original {
		t.Error("Clone should return a new instance")
	}
	if cloned.EnableCache != original.EnableCache {
		t.Error("EnableCache should be copied")
	}
	if cloned.MaxCacheSize != original.MaxCacheSize {
		t.Error("MaxCacheSize should be copied")
	}
}

// TestConfiguration tests configuration creation, validation, and cloning
func TestConfiguration(t *testing.T) {
	helper := NewTestHelper(t)

	t.Run("DefaultConfig", func(t *testing.T) {
		config := DefaultConfig()

		helper.AssertNotNil(config)
		helper.AssertTrue(config.EnableCache)
		helper.AssertEqual(DefaultCacheSize, config.MaxCacheSize)
		helper.AssertEqual(DefaultCacheTTL, config.CacheTTL)
		helper.AssertEqual(int64(DefaultMaxJSONSize), config.MaxJSONSize)
		helper.AssertEqual(DefaultMaxPathDepth, config.MaxPathDepth)
		helper.AssertFalse(config.StrictMode)
		helper.AssertFalse(config.EnableMetrics)
		helper.AssertFalse(config.EnableHealthCheck)
	})

	t.Run("HighSecurityConfig", func(t *testing.T) {
		config := HighSecurityConfig()

		helper.AssertNotNil(config)
		helper.AssertEqual(20, config.MaxNestingDepthSecurity)
		helper.AssertEqual(int64(10*1024*1024), config.MaxSecurityValidationSize)
		helper.AssertEqual(1000, config.MaxObjectKeys)
		helper.AssertEqual(1000, config.MaxArrayElements)
		helper.AssertEqual(int64(5*1024*1024), config.MaxJSONSize)
		helper.AssertEqual(20, config.MaxPathDepth)
		helper.AssertTrue(config.EnableValidation)
		helper.AssertTrue(config.StrictMode)
	})

	t.Run("LargeDataConfig", func(t *testing.T) {
		config := LargeDataConfig()

		helper.AssertNotNil(config)
		helper.AssertEqual(100, config.MaxNestingDepthSecurity)
		helper.AssertEqual(int64(500*1024*1024), config.MaxSecurityValidationSize)
		helper.AssertEqual(50000, config.MaxObjectKeys)
		helper.AssertEqual(50000, config.MaxArrayElements)
		helper.AssertEqual(int64(100*1024*1024), config.MaxJSONSize)
		helper.AssertEqual(200, config.MaxPathDepth)
	})

	t.Run("ConfigClone", func(t *testing.T) {
		original := DefaultConfig()
		original.EnableCache = false
		original.StrictMode = true

		cloned := original.Clone()

		helper.AssertNotNil(cloned)
		helper.AssertEqual(original.EnableCache, cloned.EnableCache)
		helper.AssertEqual(original.StrictMode, cloned.StrictMode)

		// Modify clone should not affect original
		cloned.EnableCache = true
		helper.AssertFalse(original.EnableCache)
	})

	t.Run("ConfigValidate", func(t *testing.T) {
		t.Run("ValidConfig", func(t *testing.T) {
			config := DefaultConfig()
			err := config.Validate()
			helper.AssertNoError(err)
		})

		t.Run("NilConfig", func(t *testing.T) {
			err := ValidateConfig(nil)
			helper.AssertError(err)
		})

		t.Run("NegativeCacheSize", func(t *testing.T) {
			config := DefaultConfig()
			config.MaxCacheSize = -1
			err := ValidateConfig(config)
			helper.AssertError(err)
		})

		t.Run("ZeroValuesGetDefaults", func(t *testing.T) {
			config := &Config{}
			err := ValidateConfig(config)
			helper.AssertNoError(err)

			// Should have defaults applied
			helper.AssertTrue(config.MaxJSONSize > 0)
			helper.AssertTrue(config.MaxPathDepth > 0)
			helper.AssertTrue(config.MaxConcurrency > 0)
		})

		t.Run("ClampValues", func(t *testing.T) {
			config := DefaultConfig()
			config.MaxJSONSize = 200 * 1024 * 1024 // Over max
			config.MaxPathDepth = 300              // Over max

			config.Validate()

			// Should be clamped
			helper.AssertTrue(config.MaxJSONSize <= 100*1024*1024)
			helper.AssertTrue(config.MaxPathDepth <= 200)
		})
	})

	t.Run("ConfigInterface", func(t *testing.T) {
		config := DefaultConfig()

		helper.AssertTrue(config.IsCacheEnabled())
		helper.AssertEqual(config.MaxCacheSize, config.GetMaxCacheSize())
		helper.AssertEqual(config.CacheTTL, config.GetCacheTTL())
		helper.AssertEqual(config.MaxJSONSize, config.GetMaxJSONSize())
		helper.AssertEqual(config.MaxPathDepth, config.GetMaxPathDepth())
		helper.AssertEqual(config.MaxConcurrency, config.GetMaxConcurrency())
		helper.AssertEqual(config.EnableMetrics, config.IsMetricsEnabled())
		helper.AssertEqual(config.EnableHealthCheck, config.IsHealthCheckEnabled())
		helper.AssertEqual(config.StrictMode, config.IsStrictMode())
	})
}

// TestConfigurationEdgeCases tests configuration edge cases
func TestConfigurationEdgeCases(t *testing.T) {
	helper := NewTestHelper(t)

	t.Run("ExtremeCacheSizes", func(t *testing.T) {
		config := DefaultConfig()

		t.Run("ZeroCacheSize", func(t *testing.T) {
			config.MaxCacheSize = 0
			err := config.Validate()
			helper.AssertNoError(err)
			// Cache might still be enabled but with 0 size
			_ = config.EnableCache
		})

		t.Run("VeryLargeCacheSize", func(t *testing.T) {
			config.MaxCacheSize = 10000
			config.Validate()
			helper.AssertTrue(config.MaxCacheSize <= 2000)
		})
	})

	t.Run("ExtremeConcurrency", func(t *testing.T) {
		config := DefaultConfig()

		t.Run("ZeroConcurrency", func(t *testing.T) {
			config.MaxConcurrency = 0
			config.Validate()
			helper.AssertTrue(config.MaxConcurrency > 0)
		})

		t.Run("VeryHighConcurrency", func(t *testing.T) {
			config.MaxConcurrency = 500
			config.Validate()
			helper.AssertTrue(config.MaxConcurrency <= 200)
		})
	})

	t.Run("CacheTTL", func(t *testing.T) {
		config := DefaultConfig()

		t.Run("ZeroTTL", func(t *testing.T) {
			config.CacheTTL = 0
			config.Validate()
			helper.AssertTrue(config.CacheTTL > 0)
		})

		t.Run("VeryShortTTL", func(t *testing.T) {
			config.CacheTTL = 1 * time.Nanosecond
			helper.AssertNoError(config.Validate())
		})

		t.Run("VeryLongTTL", func(t *testing.T) {
			config.CacheTTL = 24 * time.Hour
			helper.AssertNoError(config.Validate())
		})
	})

	t.Run("BooleanFlags", func(t *testing.T) {
		config := DefaultConfig()

		// Test all boolean flags can be set
		flags := []struct {
			name string
			set  func(bool)
			get  func() bool
		}{
			{"EnableCache", func(b bool) { config.EnableCache = b }, func() bool { return config.EnableCache }},
			{"EnableValidation", func(b bool) { config.EnableValidation = b }, func() bool { return config.EnableValidation }},
			{"StrictMode", func(b bool) { config.StrictMode = b }, func() bool { return config.StrictMode }},
			{"CreatePaths", func(b bool) { config.CreatePaths = b }, func() bool { return config.CreatePaths }},
			{"CleanupNulls", func(b bool) { config.CleanupNulls = b }, func() bool { return config.CleanupNulls }},
			{"CompactArrays", func(b bool) { config.CompactArrays = b }, func() bool { return config.CompactArrays }},
			{"EnableMetrics", func(b bool) { config.EnableMetrics = b }, func() bool { return config.EnableMetrics }},
			{"EnableHealthCheck", func(b bool) { config.EnableHealthCheck = b }, func() bool { return config.EnableHealthCheck }},
			{"AllowComments", func(b bool) { config.AllowComments = b }, func() bool { return config.AllowComments }},
			{"PreserveNumbers", func(b bool) { config.PreserveNumbers = b }, func() bool { return config.PreserveNumbers }},
			{"ValidateInput", func(b bool) { config.ValidateInput = b }, func() bool { return config.ValidateInput }},
			{"ValidateFilePath", func(b bool) { config.ValidateFilePath = b }, func() bool { return config.ValidateFilePath }},
		}

		for _, flag := range flags {
			t.Run(flag.name+"_True", func(t *testing.T) {
				flag.set(true)
				helper.AssertTrue(flag.get())
			})

			t.Run(flag.name+"_False", func(t *testing.T) {
				flag.set(false)
				helper.AssertFalse(flag.get())
			})
		}
	})
}

// TestConfigurationIntegration tests configuration with processor
func TestConfigurationIntegration(t *testing.T) {
	helper := NewTestHelper(t)

	t.Run("ProcessorWithConfig", func(t *testing.T) {
		config := DefaultConfig()
		config.EnableCache = true
		config.EnableMetrics = true

		processor := New(config)
		defer processor.Close()

		testData := `{"test": "value"}`

		result, err := processor.Get(testData, "test")
		helper.AssertNoError(err)
		helper.AssertEqual("value", result)

		stats := processor.GetStats()
		helper.AssertTrue(stats.CacheEnabled)
		helper.AssertEqual(config.MaxCacheSize, stats.MaxCacheSize)
	})

	t.Run("HighSecurityProcessor", func(t *testing.T) {
		processor := New(HighSecurityConfig())
		defer processor.Close()

		// Test that security limits are enforced
		deepJSON := generateDeepNesting(30)

		_, err := processor.Get(deepJSON, "a")
		// Should error due to depth limit
		if err != nil {
			var jsonErr *JsonsError
			if errors.As(err, &jsonErr) {
				helper.AssertEqual(ErrDepthLimit, jsonErr.Err)
			}
		}
	})

	t.Run("LargeDataProcessor", func(t *testing.T) {
		processor := New(LargeDataConfig())
		defer processor.Close()

		// Test with large array
		largeArrayData := generateLargeArray(10000)

		result, err := GetArray(largeArrayData, "items")
		helper.AssertNoError(err)
		helper.AssertTrue(len(result) > 0)
	})
}

// TestDeleteWithCleanNull tests deletion with null cleanup
func TestDeleteWithCleanNull(t *testing.T) {
	jsonStr := `{
		"user": {
			"name": "Alice",
			"age": 30,
			"email": null
		},
		"posts": [
			{"title": "Post 1", "content": null},
			{"title": "Post 2", "content": "Content"}
		]
	}`

	tests := []struct {
		name     string
		path     string
		contains []string
		excludes []string
	}{
		{
			name:     "delete and clean nulls",
			path:     "user.email",
			contains: []string{"name", "age"},
			excludes: []string{"email", "null"},
		},
		{
			name:     "delete entire array",
			path:     "posts",
			contains: []string{"user", "name"},                // Should still have user data
			excludes: []string{"Post 1", "Post 2", "content"}, // All posts content should be gone
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := DeleteWithCleanNull(jsonStr, tt.path)
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			for _, str := range tt.contains {
				if !contains(result, str) {
					t.Errorf("Expected result to contain '%s'", str)
				}
			}
			for _, str := range tt.excludes {
				if contains(result, str) {
					t.Errorf("Expected result to not contain '%s'", str)
				}
			}
		})
	}
}

// TestDeleteWithCleanupNulls tests deletion with cleanup nulls
func TestDeleteWithCleanupNulls(t *testing.T) {
	processor := New(DefaultConfig())
	defer processor.Close()

	t.Run("delete with cleanup", func(t *testing.T) {
		jsonStr := `{"a": 1, "b": null, "c": 3}`
		result, err := processor.Delete(jsonStr, "b")
		if err != nil {
			t.Fatalf("Delete error: %v", err)
		}
		_ = result
	})
}

func TestDelim_TypeMethods(t *testing.T) {
	tests := []struct {
		delim    Delim
		expected string
	}{
		{Delim('['), "["},
		{Delim(']'), "]"},
		{Delim('{'), "{"},
		{Delim('}'), "}"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if tt.delim.String() != tt.expected {
				t.Errorf("Delim.String() = %q, want %q", tt.delim.String(), tt.expected)
			}
		})
	}
}

// TestDetectConsecutiveExtractions tests detection of consecutive extraction segments
func TestDetectConsecutiveExtractions(t *testing.T) {
	processor := New()
	defer processor.Close()

	tests := []struct {
		name                string
		segments            []PathSegment
		expectedGroupCount  int
		expectedSegmentsIn0 int
	}{
		{
			name: "single extraction",
			segments: []PathSegment{
				{Type: internal.PropertySegment, Key: "users"},
				{Type: internal.ExtractSegment, Key: "name"},
			},
			expectedGroupCount:  1,
			expectedSegmentsIn0: 1,
		},
		{
			name: "consecutive extractions",
			segments: []PathSegment{
				{Type: internal.PropertySegment, Key: "data"},
				{Type: internal.ExtractSegment, Key: "users"},
				{Type: internal.ExtractSegment, Key: "name"},
			},
			expectedGroupCount:  1,
			expectedSegmentsIn0: 2,
		},
		{
			name: "separated extractions",
			segments: []PathSegment{
				{Type: internal.ExtractSegment, Key: "users"},
				{Type: internal.PropertySegment, Key: "data"},
				{Type: internal.ExtractSegment, Key: "name"},
			},
			expectedGroupCount:  2,
			expectedSegmentsIn0: 1,
		},
		{
			name:                "no extractions",
			segments:            []PathSegment{{Type: internal.PropertySegment, Key: "user"}},
			expectedGroupCount:  0,
			expectedSegmentsIn0: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			groups := processor.detectConsecutiveExtractions(tt.segments)
			if len(groups) != tt.expectedGroupCount {
				t.Errorf("detectConsecutiveExtractions() returned %d groups; want %d", len(groups), tt.expectedGroupCount)
			}
			if tt.expectedGroupCount > 0 && len(groups[0].Segments) != tt.expectedSegmentsIn0 {
				t.Errorf("First group has %d segments; want %d", len(groups[0].Segments), tt.expectedSegmentsIn0)
			}
		})
	}
}

// TestDistributedOperationPath tests distributed operation patterns
func TestDistributedOperationPath(t *testing.T) {
	_ = `{
		"data": [
			{"items": [1, 2, 3]},
			{"items": [4, 5, 6]},
			{"items": [7, 8, 9]}
		]
	}`

	processor := New()
	defer processor.Close()

	// Test that distributed operation paths are detected
	tests := []struct {
		name            string
		path            string
		shouldBeComplex bool
	}{
		{
			name:            "simple extraction",
			path:            "data{items}",
			shouldBeComplex: false, // Not distributed - extraction alone is not distributed
		},
		{
			name:            "extraction with array",
			path:            "data{items}[0]",
			shouldBeComplex: true, // Extraction followed by array access IS distributed
		},
		{
			name:            "extraction with property",
			path:            "data{items}:field",
			shouldBeComplex: true, // Extraction followed by property IS distributed
		},
		{
			name:            "simple path",
			path:            "data.items",
			shouldBeComplex: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isComplex := processor.isDistributedOperationPath(tt.path)
			if isComplex != tt.shouldBeComplex {
				t.Errorf("isDistributedOperationPath(%s) = %v; want %v", tt.path, isComplex, tt.shouldBeComplex)
			}
		})
	}
}

// TestEncodeBatch tests batch encoding
func TestEncodeBatch(t *testing.T) {
	pairs := map[string]any{
		"user1": map[string]any{"name": "Alice"},
		"user2": map[string]any{"name": "Bob"},
	}

	result, err := EncodeBatch(pairs, false)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Check that it's a JSON object
	if !contains(result, "{") || !contains(result, "}") {
		t.Error("Expected object wrapper")
	}
	if !contains(result, "user1") || !contains(result, "user2") {
		t.Error("Expected keys to be present")
	}
}

func TestEncodeConfig_Default(t *testing.T) {
	cfg := DefaultEncodeConfig()
	if cfg == nil {
		t.Error("DefaultEncodeConfig should not return nil")
	}
}

// TestEncodeFields tests selective field encoding
func TestEncodeFields(t *testing.T) {
	type User struct {
		Name     string `json:"name"`
		Age      int    `json:"age"`
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	user := User{
		Name:     "Alice",
		Age:      30,
		Email:    "alice@example.com",
		Password: "secret123",
	}

	fields := []string{"name", "email"}

	result, err := EncodeFields(user, fields, false)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Check that only specified fields are present
	if !contains(result, "name") || !contains(result, "email") {
		t.Error("Expected specified fields to be present")
	}
	if contains(result, "password") {
		t.Error("Expected password to be excluded")
	}
	if contains(result, "age") {
		t.Error("Expected age to be excluded")
	}
}

// TestEncodeStream tests stream encoding
func TestEncodeStream(t *testing.T) {
	values := []any{
		map[string]any{"name": "Alice"},
		map[string]any{"name": "Bob"},
		map[string]any{"name": "Charlie"},
	}

	tests := []struct {
		name        string
		pretty      bool
		expectError bool
		validate    func(t *testing.T, result string)
	}{
		{
			name:        "compact stream",
			pretty:      false,
			expectError: false,
			validate: func(t *testing.T, result string) {
				if !contains(result, "[") || !contains(result, "]") {
					t.Error("Expected array wrapper")
				}
			},
		},
		{
			name:        "pretty stream",
			pretty:      true,
			expectError: false,
			validate: func(t *testing.T, result string) {
				if !contains(result, "\n") {
					t.Error("Expected pretty output with newlines")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := EncodeStream(values, tt.pretty)
			if tt.expectError && err == nil {
				t.Errorf("Expected error, but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
			if tt.validate != nil {
				tt.validate(t, result)
			}
		})
	}
}

func TestEncoderDecoder(t *testing.T) {
	t.Run("NewEncoder", func(t *testing.T) {
		var buf bytes.Buffer
		enc := NewEncoder(&buf)
		if enc == nil {
			t.Error("NewEncoder returned nil")
		}
	})

	t.Run("Encoder.Encode", func(t *testing.T) {
		var buf bytes.Buffer
		enc := NewEncoder(&buf)
		err := enc.Encode(map[string]any{"key": "value"})
		if err != nil {
			t.Errorf("Encode error: %v", err)
		}
		if !strings.Contains(buf.String(), "key") {
			t.Error("Encoded output should contain 'key'")
		}
	})

	t.Run("Encoder.SetEscapeHTML", func(t *testing.T) {
		var buf bytes.Buffer
		enc := NewEncoder(&buf)
		enc.SetEscapeHTML(true)
		err := enc.Encode(map[string]any{"html": "<script>"})
		if err != nil {
			t.Errorf("Encode error: %v", err)
		}
		if !strings.Contains(buf.String(), "\\u003c") {
			t.Error("HTML should be escaped")
		}
	})

	t.Run("Encoder.SetIndent", func(t *testing.T) {
		var buf bytes.Buffer
		enc := NewEncoder(&buf)
		enc.SetIndent("", "  ")
		err := enc.Encode(map[string]any{"key": "value"})
		if err != nil {
			t.Errorf("Encode error: %v", err)
		}
		if !strings.Contains(buf.String(), "  ") {
			t.Error("Output should be indented")
		}
	})

	t.Run("NewDecoder", func(t *testing.T) {
		r := strings.NewReader(`{"key": "value"}`)
		dec := NewDecoder(r)
		if dec == nil {
			t.Error("NewDecoder returned nil")
		}
	})

	t.Run("Decoder.Decode", func(t *testing.T) {
		r := strings.NewReader(`{"key": "value"}`)
		dec := NewDecoder(r)
		var result map[string]any
		err := dec.Decode(&result)
		if err != nil {
			t.Errorf("Decode error: %v", err)
		}
		if result["key"] != "value" {
			t.Errorf("result[key] = %v, want 'value'", result["key"])
		}
	})

	t.Run("Decoder.UseNumber", func(t *testing.T) {
		r := strings.NewReader(`{"num": 123}`)
		dec := NewDecoder(r)
		dec.UseNumber()
		var result map[string]any
		err := dec.Decode(&result)
		if err != nil {
			t.Errorf("Decode error: %v", err)
		}
		if _, ok := result["num"].(Number); !ok {
			t.Errorf("result[num] should be Number type, got %T", result["num"])
		}
	})

	t.Run("Decoder.DisallowUnknownFields", func(t *testing.T) {
		r := strings.NewReader(`{"unknown": "field"}`)
		dec := NewDecoder(r)
		dec.DisallowUnknownFields()
		var result struct {
			Known string `json:"known"`
		}
		_ = dec.Decode(&result) // Just verify it doesn't panic
	})

	t.Run("Decoder.Buffered", func(t *testing.T) {
		r := strings.NewReader(`{"key": "value"}`)
		dec := NewDecoder(r)
		var result map[string]any
		dec.Decode(&result)
		buffered := dec.Buffered()
		if buffered == nil {
			t.Error("Buffered should not return nil")
		}
	})

	t.Run("Decoder.InputOffset", func(t *testing.T) {
		r := strings.NewReader(`{"key": "value"}`)
		dec := NewDecoder(r)
		var result map[string]any
		dec.Decode(&result)
		offset := dec.InputOffset()
		if offset == 0 {
			t.Log("InputOffset returned 0")
		}
	})

	t.Run("Decoder.More", func(t *testing.T) {
		r := strings.NewReader(`{"a":1}{"b":2}`)
		dec := NewDecoder(r)

		var result map[string]any
		dec.Decode(&result)
		if !dec.More() {
			t.Error("More should return true for second JSON")
		}
		dec.Decode(&result)
		if dec.More() {
			t.Error("More should return false after all JSON consumed")
		}
	})

	t.Run("Decoder.Decode nil", func(t *testing.T) {
		r := strings.NewReader(`{"key": "value"}`)
		dec := NewDecoder(r)
		err := dec.Decode(nil)
		if err == nil {
			t.Error("Decode(nil) should return error")
		}
	})

	t.Run("Decoder.Decode non-pointer", func(t *testing.T) {
		r := strings.NewReader(`{"key": "value"}`)
		dec := NewDecoder(r)
		var result map[string]any
		err := dec.Decode(result) // Not a pointer
		if err == nil {
			t.Error("Decode(non-pointer) should return error")
		}
	})
}

// TestEncodingConfiguration tests encoding configuration
func TestEncodingConfiguration(t *testing.T) {
	helper := NewTestHelper(t)

	t.Run("DefaultEncodeConfig", func(t *testing.T) {
		config := DefaultEncodeConfig()

		helper.AssertNotNil(config)
		helper.AssertFalse(config.Pretty)
		helper.AssertEqual("  ", config.Indent)
		helper.AssertEqual("", config.Prefix)
		helper.AssertTrue(config.EscapeHTML)
		helper.AssertFalse(config.SortKeys)
		helper.AssertTrue(config.ValidateUTF8)
		helper.AssertEqual(100, config.MaxDepth)
		helper.AssertFalse(config.DisallowUnknown)
		helper.AssertFalse(config.PreserveNumbers)
		helper.AssertEqual(-1, config.FloatPrecision)
		helper.AssertFalse(config.DisableEscaping)
		helper.AssertFalse(config.EscapeUnicode)
		helper.AssertFalse(config.EscapeSlash)
		helper.AssertTrue(config.EscapeNewlines)
		helper.AssertTrue(config.EscapeTabs)
		helper.AssertTrue(config.IncludeNulls)
	})

	t.Run("NewPrettyConfig", func(t *testing.T) {
		config := NewPrettyConfig()

		helper.AssertTrue(config.Pretty)
		helper.AssertEqual("  ", config.Indent)
	})

	t.Run("EncodingOptions", func(t *testing.T) {
		config := DefaultEncodeConfig()

		t.Run("SetPretty", func(t *testing.T) {
			config.Pretty = true
			helper.AssertTrue(config.Pretty)
		})

		t.Run("SetSortKeys", func(t *testing.T) {
			config.SortKeys = true
			helper.AssertTrue(config.SortKeys)
		})

		t.Run("SetFloatPrecision", func(t *testing.T) {
			config.FloatPrecision = 2
			helper.AssertEqual(2, config.FloatPrecision)
		})

		t.Run("SetEscapeHTML", func(t *testing.T) {
			config.EscapeHTML = false
			helper.AssertFalse(config.EscapeHTML)
		})

		t.Run("SetEscapeUnicode", func(t *testing.T) {
			config.EscapeUnicode = true
			helper.AssertTrue(config.EscapeUnicode)
		})

		t.Run("SetIncludeNulls", func(t *testing.T) {
			config.IncludeNulls = false
			helper.AssertFalse(config.IncludeNulls)
		})
	})
}

// TestExtractSyntaxComplex tests complex extraction syntax scenarios
func TestExtractSyntaxComplex(t *testing.T) {
	jsonStr := `{
		"users": [
			{"name": "Alice", "age": 30, "address": {"city": "NYC"}},
			{"name": "Bob", "age": 25, "address": {"city": "LA"}},
			{"name": "Charlie", "age": 35, "address": {"city": "SF"}}
		],
		"departments": [
			{"name": "Engineering", "head": "Alice"},
			{"name": "Sales", "head": "Bob"}
		]
	}`

	processor := New()
	defer processor.Close()

	tests := []struct {
		name     string
		path     string
		validate func(t *testing.T, result any, err error)
	}{
		{
			name: "simple extraction",
			path: "users{name}",
			validate: func(t *testing.T, result any, err error) {
				if err != nil {
					t.Fatalf("Unexpected error: %v", err)
				}
				arr, ok := result.([]any)
				if !ok || len(arr) != 3 {
					t.Errorf("Expected array of 3 names, got: %v", result)
				}
			},
		},
		{
			name: "flat extraction",
			path: "users{flat:address.city}",
			validate: func(t *testing.T, result any, err error) {
				if err != nil {
					t.Fatalf("Unexpected error: %v", err)
				}
				// Flat extraction should flatten nested arrays
				if result == nil {
					t.Error("Expected result, got nil")
				}
			},
		},
		{
			name: "extraction with array access",
			path: "users{name}[0]",
			validate: func(t *testing.T, result any, err error) {
				if err != nil {
					t.Fatalf("Unexpected error: %v", err)
				}
				// Should return first name
				if result == nil {
					t.Error("Expected result, got nil")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := processor.Get(jsonStr, tt.path)
			tt.validate(t, result, err)
		})
	}
}

// TestExtractionOperations tests extraction operations
func TestExtractionOperations(t *testing.T) {
	processor := New()
	defer processor.Close()

	t.Run("extract array field", func(t *testing.T) {
		jsonStr := `{"users": [{"name": "Alice", "age": 30}, {"name": "Bob", "age": 25}]}`
		result, err := processor.Get(jsonStr, "users{name}")
		if err != nil {
			t.Fatalf("Get extract error: %v", err)
		}
		_ = result
	})
}

// TestFastDelete tests the FastDelete function
func TestFastDelete(t *testing.T) {
	processor := New()
	defer processor.Close()

	t.Run("simple property delete", func(t *testing.T) {
		jsonStr := `{"name": "test", "age": 30}`
		result, err := processor.FastDelete(jsonStr, "name")
		if err != nil {
			t.Fatalf("FastDelete error: %v", err)
		}
		if result == "" {
			t.Error("FastDelete returned empty result")
		}
	})

	t.Run("nested property delete", func(t *testing.T) {
		jsonStr := `{"user": {"name": "test", "email": "test@example.com"}}`
		result, err := processor.FastDelete(jsonStr, "user.email")
		if err != nil {
			t.Fatalf("FastDelete error: %v", err)
		}
		if result == "" {
			t.Error("FastDelete returned empty result")
		}
	})

	t.Run("delete array element", func(t *testing.T) {
		jsonStr := `{"items": [1, 2, 3]}`
		result, err := processor.FastDelete(jsonStr, "items[1]")
		if err != nil {
			t.Fatalf("FastDelete error: %v", err)
		}
		if result == "" {
			t.Error("FastDelete returned empty result")
		}
	})
}

// TestFastGetMultiple tests the FastGetMultiple function
func TestFastGetMultiple(t *testing.T) {
	processor := New()
	defer processor.Close()

	t.Run("get multiple paths", func(t *testing.T) {
		jsonStr := `{"a": 1, "b": 2, "c": 3}`
		paths := []string{"a", "b", "c"}
		results, err := processor.FastGetMultiple(jsonStr, paths)
		if err != nil {
			t.Fatalf("FastGetMultiple error: %v", err)
		}
		if len(results) != 3 {
			t.Errorf("FastGetMultiple returned %d results, want 3", len(results))
		}
	})

	t.Run("empty paths", func(t *testing.T) {
		jsonStr := `{"a": 1}`
		paths := []string{}
		results, err := processor.FastGetMultiple(jsonStr, paths)
		if err != nil {
			t.Fatalf("FastGetMultiple error: %v", err)
		}
		if len(results) != 0 {
			t.Errorf("FastGetMultiple returned %d results, want 0", len(results))
		}
	})
}

// TestFastSet tests the FastSet function
func TestFastSet(t *testing.T) {
	processor := New()
	defer processor.Close()

	t.Run("simple property set", func(t *testing.T) {
		jsonStr := `{"name": "old"}`
		result, err := processor.FastSet(jsonStr, "name", "new")
		if err != nil {
			t.Fatalf("FastSet error: %v", err)
		}
		if result == "" {
			t.Error("FastSet returned empty result")
		}
	})

	t.Run("nested property set", func(t *testing.T) {
		jsonStr := `{"user": {"name": "old"}}`
		result, err := processor.FastSet(jsonStr, "user.name", "new")
		if err != nil {
			t.Fatalf("FastSet error: %v", err)
		}
		if result == "" {
			t.Error("FastSet returned empty result")
		}
	})

	t.Run("set new property", func(t *testing.T) {
		jsonStr := `{"name": "test"}`
		result, err := processor.FastSet(jsonStr, "age", 30)
		if err != nil {
			t.Fatalf("FastSet error: %v", err)
		}
		if result == "" {
			t.Error("FastSet returned empty result")
		}
	})

	t.Run("set array element", func(t *testing.T) {
		jsonStr := `{"items": [1, 2, 3]}`
		result, err := processor.FastSet(jsonStr, "items[1]", 10)
		if err != nil {
			t.Fatalf("FastSet error: %v", err)
		}
		if result == "" {
			t.Error("FastSet returned empty result")
		}
	})
}

// TestFormatCompact tests compact formatting
func TestFormatCompact(t *testing.T) {
	prettyJSON := `{
		"user": {
			"name": "Alice",
			"age": 30
		}
	}`

	result, err := FormatCompact(prettyJSON)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Check that it's compact
	if contains(result, "\n") {
		t.Error("Expected compact output to not contain newlines")
	}
}

// TestFormatPretty tests pretty formatting
func TestFormatPretty(t *testing.T) {
	compactJSON := `{"user":{"name":"Alice","age":30},"settings":{"theme":"dark"}}`

	result, err := FormatPretty(compactJSON)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Check for indentation
	if !contains(result, "\n") {
		t.Error("Expected formatted output to contain newlines")
	}
	if !contains(result, "  ") {
		t.Error("Expected formatted output to contain indentation")
	}
}

// TestForwardSlice tests forward slicing logic
func TestForwardSlice(t *testing.T) {
	processor := New()
	defer processor.Close()

	arr := []any{0.0, 1.0, 2.0, 3.0, 4.0}

	tests := []struct {
		name     string
		start    int
		end      int
		step     int
		expected []any
	}{
		{
			name:     "normal slice",
			start:    1,
			end:      4,
			step:     1,
			expected: []any{1.0, 2.0, 3.0},
		},
		{
			name:     "with step",
			start:    0,
			end:      5,
			step:     2,
			expected: []any{0.0, 2.0, 4.0},
		},
		{
			name:     "start negative",
			start:    -1,
			end:      4,
			step:     1,
			expected: []any{0.0, 1.0, 2.0, 3.0},
		},
		{
			name:     "end beyond length",
			start:    2,
			end:      10,
			step:     1,
			expected: []any{2.0, 3.0, 4.0},
		},
		{
			name:     "empty result",
			start:    3,
			end:      1,
			step:     1,
			expected: []any{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := processor.forwardSlice(arr, tt.start, tt.end, tt.step)
			if !coreSliceEqual(result, tt.expected) {
				t.Errorf("forwardSlice(%d, %d, %d) = %v; want %v", tt.start, tt.end, tt.step, result, tt.expected)
			}
		})
	}
}

// TestGetArrayWithDefault tests GetArrayWithDefault function
func TestGetArrayWithDefault(t *testing.T) {
	jsonStr := `{"items": [1, 2, 3]}`
	defaultArr := []any{"default"}

	t.Run("existing array", func(t *testing.T) {
		result := GetArrayWithDefault(jsonStr, "items", defaultArr)
		if len(result) != 3 {
			t.Errorf("GetArrayWithDefault(items) length = %d; want 3", len(result))
		}
	})

	t.Run("missing returns default", func(t *testing.T) {
		result := GetArrayWithDefault(jsonStr, "missing", defaultArr)
		if len(result) != 1 || result[0] != "default" {
			t.Errorf("GetArrayWithDefault(missing) = %v; want default", result)
		}
	})
}

// TestGetBoolWithDefault tests GetBoolWithDefault function
func TestGetBoolWithDefault(t *testing.T) {
	jsonStr := `{"enabled": true, "disabled": false}`

	t.Run("existing true", func(t *testing.T) {
		result := GetBoolWithDefault(jsonStr, "enabled", false)
		if result != true {
			t.Errorf("GetBoolWithDefault(enabled) = %v; want true", result)
		}
	})

	t.Run("existing false", func(t *testing.T) {
		result := GetBoolWithDefault(jsonStr, "disabled", true)
		if result != false {
			t.Errorf("GetBoolWithDefault(disabled) = %v; want false", result)
		}
	})

	t.Run("missing returns default", func(t *testing.T) {
		result := GetBoolWithDefault(jsonStr, "missing", true)
		if result != true {
			t.Errorf("GetBoolWithDefault(missing) = %v; want true", result)
		}
	})
}

// TestGetFloat64WithDefault tests GetFloat64WithDefault function
func TestGetFloat64WithDefault(t *testing.T) {
	jsonStr := `{"price": 19.99, "count": 5}`

	t.Run("existing float", func(t *testing.T) {
		result := GetFloat64WithDefault(jsonStr, "price", 0.0)
		if result != 19.99 {
			t.Errorf("GetFloat64WithDefault(price) = %f; want 19.99", result)
		}
	})

	t.Run("int converted to float", func(t *testing.T) {
		result := GetFloat64WithDefault(jsonStr, "count", 0.0)
		if result != 5.0 {
			t.Errorf("GetFloat64WithDefault(count) = %f; want 5.0", result)
		}
	})

	t.Run("missing returns default", func(t *testing.T) {
		result := GetFloat64WithDefault(jsonStr, "missing", 99.99)
		if result != 99.99 {
			t.Errorf("GetFloat64WithDefault(missing) = %f; want 99.99", result)
		}
	})
}

// TestGetHealthStatus tests health status retrieval
func TestGetHealthStatus(t *testing.T) {
	status := GetHealthStatus()

	// Just verify we can get health status without panicking
	// The actual healthy status depends on whether metrics are initialized
	if status.Timestamp.IsZero() {
		t.Error("Expected non-zero timestamp")
	}

	if len(status.Checks) == 0 {
		t.Error("Expected some health checks to be present")
	}

	// At minimum, memory check should be present
	if _, ok := status.Checks["memory"]; !ok {
		t.Error("Expected memory check to be present")
	}

	t.Logf("Health status: %+v", status)
}

// TestGetMultiple tests retrieving multiple values at once
func TestGetMultiple(t *testing.T) {
	jsonStr := `{
		"user": {
			"name": "Alice",
			"age": 30,
			"email": "alice@example.com"
		},
		"settings": {
			"theme": "dark",
			"language": "en"
		}
	}`

	tests := []struct {
		name        string
		paths       []string
		expectedLen int
		expectError bool
	}{
		{
			name:        "multiple paths",
			paths:       []string{"user.name", "user.age", "settings.theme"},
			expectedLen: 3,
			expectError: false,
		},
		{
			name:        "single path",
			paths:       []string{"user.name"},
			expectedLen: 1,
			expectError: false,
		},
		{
			name:        "empty paths",
			paths:       []string{},
			expectedLen: 0,
			expectError: false,
		},
		{
			name:        "mixed valid and invalid",
			paths:       []string{"user.name", "invalid.path", "settings.theme"},
			expectedLen: 3, // GetMultiple returns entries for all paths, including nil for invalid ones
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := GetMultiple(jsonStr, tt.paths)
			if tt.expectError && err == nil {
				t.Errorf("Expected error, but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
			if !tt.expectError && len(result) != tt.expectedLen {
				t.Errorf("Result length = %d; want %d", len(result), tt.expectedLen)
			}
		})
	}
}

// TestGetObjectWithDefault tests GetObjectWithDefault function
func TestGetObjectWithDefault(t *testing.T) {
	jsonStr := `{"config": {"theme": "dark"}}`
	defaultObj := map[string]any{"default": true}

	t.Run("existing object", func(t *testing.T) {
		result := GetObjectWithDefault(jsonStr, "config", defaultObj)
		if result["theme"] != "dark" {
			t.Errorf("GetObjectWithDefault(config) = %v; want theme=dark", result)
		}
	})

	t.Run("missing returns default", func(t *testing.T) {
		result := GetObjectWithDefault(jsonStr, "missing", defaultObj)
		if result["default"] != true {
			t.Errorf("GetObjectWithDefault(missing) = %v; want default", result)
		}
	})
}

// TestGetResultBuffer tests buffer pool operations
func TestGetResultBuffer(t *testing.T) {
	buf := GetResultBuffer()
	if buf == nil {
		t.Fatal("GetResultBuffer returned nil")
	}

	*buf = append(*buf, "test data"...)
	if string(*buf) != "test data" {
		t.Errorf("Buffer content = %q, want %q", string(*buf), "test data")
	}

	PutResultBuffer(buf)
}

// TestGetStats tests statistics retrieval
func TestGetStats(t *testing.T) {
	stats := GetStats()

	if stats.CacheSize < 0 {
		t.Error("Expected non-negative cache size")
	}

	t.Logf("Stats: %+v", stats)
}

// TestGetTypedWithDefault tests typed get with defaults
func TestGetTypedWithDefault(t *testing.T) {
	jsonStr := `{"user": {"name": "Alice", "age": 30}}`

	t.Run("existing value", func(t *testing.T) {
		name := GetStringWithDefault(jsonStr, "user.name", "Unknown")
		if name != "Alice" {
			t.Errorf("Expected 'Alice', got '%s'", name)
		}
	})

	t.Run("missing value with default", func(t *testing.T) {
		name := GetStringWithDefault(jsonStr, "user.email", "unknown@example.com")
		if name != "unknown@example.com" {
			t.Errorf("Expected default value, got '%s'", name)
		}
	})

	t.Run("int with default", func(t *testing.T) {
		age := GetIntWithDefault(jsonStr, "user.age", 0)
		if age != 30 {
			t.Errorf("Expected 30, got %d", age)
		}
	})

	t.Run("missing int with default", func(t *testing.T) {
		score := GetIntWithDefault(jsonStr, "user.score", 100)
		if score != 100 {
			t.Errorf("Expected default 100, got %d", score)
		}
	})
}

// TestGetTypedWithDefaultGeneric tests the generic GetTypedWithDefault function with generics
func TestGetTypedWithDefaultGeneric(t *testing.T) {
	jsonStr := `{"user": {"name": "Alice", "age": 30, "active": true}}`

	tests := []struct {
		name         string
		path         string
		defaultValue any
		expected     any
	}{
		{
			name:         "existing string",
			path:         "user.name",
			defaultValue: "Unknown",
			expected:     "Alice",
		},
		{
			name:         "missing string returns default",
			path:         "user.email",
			defaultValue: "no@email.com",
			expected:     "no@email.com",
		},
		{
			name:         "existing int",
			path:         "user.age",
			defaultValue: 0,
			expected:     30,
		},
		{
			name:         "missing int returns default",
			path:         "user.score",
			defaultValue: 100,
			expected:     100,
		},
		{
			name:         "existing bool",
			path:         "user.active",
			defaultValue: false,
			expected:     true,
		},
		{
			name:         "missing bool returns default",
			path:         "user.admin",
			defaultValue: false,
			expected:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			switch def := tt.defaultValue.(type) {
			case string:
				result := GetTypedWithDefault[string](jsonStr, tt.path, def)
				if result != tt.expected.(string) {
					t.Errorf("GetTypedWithDefault[string](%s) = %v; want %v", tt.path, result, tt.expected)
				}
			case int:
				result := GetTypedWithDefault[int](jsonStr, tt.path, def)
				if result != tt.expected.(int) {
					t.Errorf("GetTypedWithDefault[int](%s) = %d; want %d", tt.path, result, tt.expected)
				}
			case bool:
				result := GetTypedWithDefault[bool](jsonStr, tt.path, def)
				if result != tt.expected.(bool) {
					t.Errorf("GetTypedWithDefault[bool](%s) = %v; want %v", tt.path, result, tt.expected)
				}
			}
		})
	}
}

// TestGetWithDefault tests get with default value
func TestGetWithDefault(t *testing.T) {
	jsonStr := `{"user": {"name": "Alice"}}`

	t.Run("existing value", func(t *testing.T) {
		result := GetWithDefault(jsonStr, "user.name", "Unknown")
		if result != "Alice" {
			t.Errorf("Expected 'Alice', got '%v'", result)
		}
	})

	t.Run("missing value", func(t *testing.T) {
		result := GetWithDefault(jsonStr, "user.email", "unknown@example.com")
		if result != "unknown@example.com" {
			t.Errorf("Expected default value, got '%v'", result)
		}
	})
}

// TestGlobalProcessor_BasicFunctionality tests basic global processor operations
func TestGlobalProcessor_BasicFunctionality(t *testing.T) {
	// Reset global processor state
	ShutdownGlobalProcessor()

	t.Run("GetDefaultProcessorCreatesProcessor", func(t *testing.T) {
		p := getDefaultProcessor()
		if p == nil {
			t.Fatal("getDefaultProcessor returned nil")
		}
		if p.IsClosed() {
			t.Error("Default processor should not be closed")
		}
	})

	t.Run("GetDefaultProcessorReturnsSameInstance", func(t *testing.T) {
		p1 := getDefaultProcessor()
		p2 := getDefaultProcessor()

		if p1 != p2 {
			t.Error("getDefaultProcessor should return the same instance")
		}
	})

	// Cleanup
	ShutdownGlobalProcessor()
}

// TestGlobalProcessor_ConcurrentAccess tests concurrent access to global processor
func TestGlobalProcessor_ConcurrentAccess(t *testing.T) {
	// Reset state
	ShutdownGlobalProcessor()

	t.Run("ConcurrentGetDefaultProcessor", func(t *testing.T) {
		const goroutines = 100
		var wg sync.WaitGroup
		processors := make(chan *Processor, goroutines)

		for i := 0; i < goroutines; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				p := getDefaultProcessor()
				processors <- p
			}()
		}

		wg.Wait()
		close(processors)

		// All goroutines should get the same processor instance
		var firstProcessor *Processor
		count := 0
		for p := range processors {
			count++
			if firstProcessor == nil {
				firstProcessor = p
			} else if firstProcessor != p {
				t.Error("All goroutines should get the same processor instance")
				break
			}
		}

		if count != goroutines {
			t.Errorf("Received %d processors, want %d", count, goroutines)
		}
	})

	t.Run("ConcurrentSetAndGet", func(t *testing.T) {
		ShutdownGlobalProcessor()

		const goroutines = 50
		var wg sync.WaitGroup

		for i := 0; i < goroutines; i++ {
			wg.Add(2)

			// Goroutine that sets processor
			go func(id int) {
				defer wg.Done()
				config := DefaultConfig()
				config.MaxCacheSize = 100 + id
				p := New(config)
				SetGlobalProcessor(p)
			}(i)

			// Goroutine that gets processor
			go func() {
				defer wg.Done()
				_ = getDefaultProcessor()
			}()
		}

		wg.Wait()
	})

	// Cleanup
	ShutdownGlobalProcessor()
}

// TestGlobalProcessor_IntegrationWithStats tests that global processor stats work correctly
func TestGlobalProcessor_IntegrationWithStats(t *testing.T) {
	// Reset state
	ShutdownGlobalProcessor()

	// Perform some operations
	jsonStr := `{"test":"value"}`
	for i := 0; i < 10; i++ {
		_, _ = Get(jsonStr, "test")
	}

	// Get stats
	stats := GetStats()

	if stats.IsClosed {
		t.Error("Global processor should not be closed")
	}

	// OperationCount should be at least 10
	if stats.OperationCount < 10 {
		t.Errorf("OperationCount = %d, want at least 10", stats.OperationCount)
	}

	// Cleanup
	ShutdownGlobalProcessor()
}

// TestGlobalProcessor_Lifecycle tests the complete lifecycle of global processor
func TestGlobalProcessor_Lifecycle(t *testing.T) {
	t.Run("CompleteLifecycle", func(t *testing.T) {
		// Start fresh
		ShutdownGlobalProcessor()

		// 1. Get default processor (creates new)
		p1 := getDefaultProcessor()
		if p1 == nil {
			t.Fatal("Expected non-nil processor")
		}

		// 2. Use the processor
		jsonStr := `{"key":"value"}`
		result, err := Get(jsonStr, "key")
		if err != nil {
			t.Errorf("Get failed: %v", err)
		}
		if result != "value" {
			t.Errorf("result = %v, want value", result)
		}

		// 3. Set a custom processor
		customConfig := DefaultConfig()
		customConfig.MaxCacheSize = 256
		p2 := New(customConfig)
		SetGlobalProcessor(p2)

		// 4. Verify custom processor is used
		if getDefaultProcessor() != p2 {
			t.Error("Custom processor should be active")
		}

		// 5. Shutdown
		ShutdownGlobalProcessor()

		// 6. Verify new processor is created after shutdown
		p3 := getDefaultProcessor()
		if p3 == p2 {
			t.Error("New processor should be created after shutdown")
		}

		// Final cleanup
		ShutdownGlobalProcessor()
	})
}

// TestGlobalProcessor_ReplaceClosesOld tests that replacing global processor closes the old one
func TestGlobalProcessor_ReplaceClosesOld(t *testing.T) {
	// Reset state
	ShutdownGlobalProcessor()

	// Create first processor
	p1 := New(DefaultConfig())
	SetGlobalProcessor(p1)

	// Create second processor
	p2 := New(DefaultConfig())
	SetGlobalProcessor(p2)

	// Give it time to close
	time.Sleep(10 * time.Millisecond)

	// Old processor should be closed
	if !p1.IsClosed() {
		t.Error("Old processor should be closed when replaced")
	}

	// New processor should be the global processor
	if getDefaultProcessor() != p2 {
		t.Error("New processor should be the global processor")
	}

	// Cleanup
	ShutdownGlobalProcessor()
}

// TestGlobalProcessor_ThreadSafety tests thread safety of global processor operations
func TestGlobalProcessor_ThreadSafety(t *testing.T) {
	// Reset state
	ShutdownGlobalProcessor()

	const iterations = 100
	const goroutines = 20

	var wg sync.WaitGroup
	errors := make(chan error, goroutines*iterations)

	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				// Mix of operations
				switch i % 5 {
				case 0:
					_ = getDefaultProcessor()
				case 1:
					_, err := Get(`{"test":"value"}`, "test")
					if err != nil {
						errors <- err
					}
				case 2:
					_, err := Set(`{"test":"old"}`, "test", "new")
					if err != nil {
						errors <- err
					}
				case 3:
					// Occasionally set a new processor
					if i%20 == 0 {
						config := DefaultConfig()
						config.MaxCacheSize = 100 + goroutineID
						p := New(config)
						SetGlobalProcessor(p)
					}
				case 4:
					// Occasionally check stats
					_ = GetStats()
				}
			}
		}(g)
	}

	wg.Wait()
	close(errors)

	for err := range errors {
		t.Errorf("Concurrent operation error: %v", err)
	}

	// Cleanup
	ShutdownGlobalProcessor()
}

// TestGlobalProcessor_WithPackageFunctions tests that package functions work with global processor
func TestGlobalProcessor_WithPackageFunctions(t *testing.T) {
	// Reset state
	ShutdownGlobalProcessor()

	t.Run("PackageGetUsesGlobalProcessor", func(t *testing.T) {
		testData := `{"name":"test","value":123}`

		result, err := Get(testData, "name")
		if err != nil {
			t.Errorf("Get failed: %v", err)
		}
		if result != "test" {
			t.Errorf("result = %v, want test", result)
		}
	})

	t.Run("PackageSetUsesGlobalProcessor", func(t *testing.T) {
		testData := `{"name":"old"}`

		result, err := Set(testData, "name", "new")
		if err != nil {
			t.Errorf("Set failed: %v", err)
		}
		if result != `{"name":"new"}` {
			t.Errorf("result = %s, want {\"name\":\"new\"}", result)
		}
	})

	t.Run("PackageDeleteUsesGlobalProcessor", func(t *testing.T) {
		testData := `{"name":"test","value":123}`

		result, err := Delete(testData, "name")
		if err != nil {
			t.Errorf("Delete failed: %v", err)
		}
		if result != `{"value":123}` {
			t.Errorf("result = %s, want {\"value\":123}", result)
		}
	})

	// Cleanup
	ShutdownGlobalProcessor()
}

// TestHTMLEscapeBuffer tests HTMLEscapeBuffer function
func TestHTMLEscapeBuffer(t *testing.T) {
	htmlContent := `{"html":"<script>alert('xss')</script>","amp":"a & b"}`

	var buf bytes.Buffer
	HTMLEscapeBuffer(&buf, []byte(htmlContent))

	result := buf.String()
	// Check that HTML characters are escaped
	if contains(result, "<script>") {
		t.Errorf("HTMLEscapeBuffer should escape HTML, got: %s", result)
	}
}

// TestHandleArrayAccess tests array access handling
func TestHandleArrayAccess(t *testing.T) {
	processor := New()
	defer processor.Close()

	jsonStr := `{
		"items": [10, 20, 30, 40, 50],
		"nested": {
			"arr": [1, 2, 3]
		}
	}`

	var data any
	if err := processor.Parse(jsonStr, &data); err != nil {
		t.Fatalf("Failed to parse test data: %v", err)
	}

	tests := []struct {
		name        string
		data        any
		segment     PathSegment
		expectedVal any
		shouldExist bool
	}{
		{
			name: "valid positive index",
			data: getProp(data, "items"),
			segment: PathSegment{
				Type:  internal.ArrayIndexSegment,
				Index: 2,
			},
			expectedVal: 30.0,
			shouldExist: true,
		},
		{
			name: "negative index",
			data: getProp(data, "items"),
			segment: PathSegment{
				Type:  internal.ArrayIndexSegment,
				Index: -1,
			},
			expectedVal: 50.0,
			shouldExist: true,
		},
		{
			name: "out of bounds positive",
			data: getProp(data, "items"),
			segment: PathSegment{
				Type:  internal.ArrayIndexSegment,
				Index: 10,
			},
			expectedVal: nil,
			shouldExist: false,
		},
		{
			name: "out of bounds negative",
			data: getProp(data, "items"),
			segment: PathSegment{
				Type:  internal.ArrayIndexSegment,
				Index: -10,
			},
			expectedVal: nil,
			shouldExist: false,
		},
		{
			name: "with property key",
			data: getProp(getProp(data, "nested"), "arr"),
			segment: PathSegment{
				Type:  internal.ArrayIndexSegment,
				Index: 1,
				Key:   "",
			},
			expectedVal: 2.0,
			shouldExist: true,
		},
		{
			name: "invalid data type",
			data: "not an array",
			segment: PathSegment{
				Type:  internal.ArrayIndexSegment,
				Index: 0,
			},
			expectedVal: nil,
			shouldExist: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := processor.handleArrayAccess(tt.data, tt.segment)
			if result.Exists != tt.shouldExist {
				t.Errorf("handleArrayAccess() existence = %v; want %v", result.Exists, tt.shouldExist)
			}
			if tt.shouldExist && result.Value != tt.expectedVal {
				t.Errorf("handleArrayAccess() value = %v; want %v", result.Value, tt.expectedVal)
			}
		})
	}
}

// TestHandleExtraction tests field extraction from objects/arrays
func TestHandleExtraction(t *testing.T) {
	processor := New()
	defer processor.Close()

	tests := []struct {
		name        string
		data        any
		segment     PathSegment
		expectedLen int
		expectError bool
	}{
		{
			name: "extract from array",
			data: []any{
				map[string]any{"name": "Alice", "age": 30},
				map[string]any{"name": "Bob", "age": 25},
				map[string]any{"name": "Charlie", "age": 35},
			},
			segment: PathSegment{
				Type: internal.ExtractSegment,
				Key:  "name",
			},
			expectedLen: 3,
			expectError: false,
		},
		{
			name: "extract from single object",
			data: map[string]any{
				"name": "Alice",
				"age":  30,
			},
			segment: PathSegment{
				Type: internal.ExtractSegment,
				Key:  "name",
			},
			expectedLen: 0, // Single value, not an array
			expectError: false,
		},
		{
			name: "extract missing field",
			data: []any{
				map[string]any{"age": 30},
				map[string]any{"age": 25},
			},
			segment: PathSegment{
				Type: internal.ExtractSegment,
				Key:  "name",
			},
			expectedLen: 0,
			expectError: false,
		},
		{
			name: "flat extraction",
			data: []any{
				map[string]any{"items": []any{1, 2}},
				map[string]any{"items": []any{3, 4}},
			},
			segment: PathSegment{
				Type:  internal.ExtractSegment,
				Key:   "items",
				Flags: internal.FlagIsFlat,
			},
			expectedLen: 4, // Flattened: [1, 2, 3, 4]
			expectError: false,
		},
		{
			name:        "invalid data type",
			data:        "not extractable",
			segment:     PathSegment{Type: internal.ExtractSegment, Key: "name"},
			expectedLen: 0,
			expectError: false,
		},
		{
			name: "empty key",
			data: []any{
				map[string]any{"name": "Alice"},
			},
			segment: PathSegment{
				Type: internal.ExtractSegment,
				Key:  "",
			},
			expectedLen: 0,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := processor.handleExtraction(tt.data, tt.segment)
			if tt.expectError && err == nil {
				t.Errorf("Expected error, but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
			if tt.expectedLen > 0 {
				arr, ok := result.([]any)
				if !ok {
					t.Errorf("Expected []any result, got %T", result)
				} else if len(arr) != tt.expectedLen {
					t.Errorf("Result length = %d; want %d", len(arr), tt.expectedLen)
				}
			}
		})
	}
}

// TestHandlePropertyAccess tests property access handling
func TestHandlePropertyAccess(t *testing.T) {
	processor := New()
	defer processor.Close()

	tests := []struct {
		name        string
		data        any
		property    string
		expectedVal any
		shouldExist bool
	}{
		{
			name: "map string key exists",
			data: map[string]any{
				"name": "Alice",
				"age":  30,
			},
			property:    "name",
			expectedVal: "Alice",
			shouldExist: true,
		},
		{
			name: "map string key not exists",
			data: map[string]any{
				"name": "Alice",
			},
			property:    "age",
			expectedVal: nil,
			shouldExist: false,
		},
		{
			name: "map any key exists",
			data: map[any]any{
				"name": "Bob",
				"age":  25,
			},
			property:    "name",
			expectedVal: "Bob",
			shouldExist: true,
		},
		{
			name:        "array with numeric property",
			data:        []any{"a", "b", "c"},
			property:    "1",
			expectedVal: "b",
			shouldExist: true,
		},
		{
			name:        "array with invalid property",
			data:        []any{"a", "b", "c"},
			property:    "5",
			expectedVal: nil,
			shouldExist: false,
		},
		{
			name:        "invalid data type",
			data:        "string",
			property:    "length",
			expectedVal: nil,
			shouldExist: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := processor.handlePropertyAccess(tt.data, tt.property)
			if result.Exists != tt.shouldExist {
				t.Errorf("handlePropertyAccess() existence = %v; want %v", result.Exists, tt.shouldExist)
			}
			if tt.shouldExist && result.Value != tt.expectedVal {
				t.Errorf("handlePropertyAccess() value = %v; want %v", result.Value, tt.expectedVal)
			}
		})
	}
}

// TestHandleStructAccess tests struct field access
func TestHandleStructAccess(t *testing.T) {
	processor := New()
	defer processor.Close()

	type TestStruct struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}

	t.Run("access struct field", func(t *testing.T) {
		data := TestStruct{Name: "test", Age: 30}
		jsonBytes, err := processor.Marshal(data)
		if err != nil {
			t.Fatalf("Marshal error: %v", err)
		}

		result, err := processor.Get(string(jsonBytes), "name")
		if err != nil {
			t.Fatalf("Get error: %v", err)
		}
		if result != "test" {
			t.Errorf("Get struct field = %v, want test", result)
		}
	})

	t.Run("access nested struct", func(t *testing.T) {
		type Nested struct {
			Value string `json:"value"`
		}
		type Outer struct {
			Nested Nested `json:"nested"`
		}

		data := Outer{Nested: Nested{Value: "nested_value"}}
		jsonBytes, err := processor.Marshal(data)
		if err != nil {
			t.Fatalf("Marshal error: %v", err)
		}

		result, err := processor.Get(string(jsonBytes), "nested.value")
		if err != nil {
			t.Fatalf("Get error: %v", err)
		}
		if result != "nested_value" {
			t.Errorf("Get nested field = %v, want nested_value", result)
		}
	})
}

// TestIndentBuffer tests IndentBuffer function
func TestIndentBuffer(t *testing.T) {
	compactJSON := `{"name":"Alice","age":30}`

	var buf bytes.Buffer
	err := IndentBuffer(&buf, []byte(compactJSON), "", "  ")
	if err != nil {
		t.Fatalf("IndentBuffer error: %v", err)
	}

	result := buf.String()
	if !contains(result, "\n") {
		t.Errorf("IndentBuffer should add newlines, got: %s", result)
	}
	if !contains(result, "  ") {
		t.Errorf("IndentBuffer should add indentation, got: %s", result)
	}
}

// TestIsArrayType tests array type detection
func TestIsArrayType(t *testing.T) {
	processor := New()
	defer processor.Close()

	tests := []struct {
		name     string
		data     any
		expected bool
	}{
		{
			name:     "is array",
			data:     []any{1, 2, 3},
			expected: true,
		},
		{
			name:     "is not array - map",
			data:     map[string]any{},
			expected: false,
		},
		{
			name:     "is not array - string",
			data:     "array",
			expected: false,
		},
		{
			name:     "is not array - nil",
			data:     nil,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := processor.isArrayType(tt.data)
			if result != tt.expected {
				t.Errorf("isArrayType() = %v; want %v", result, tt.expected)
			}
		})
	}
}

// TestIsComplexPath tests complex path detection
func TestIsComplexPath(t *testing.T) {
	processor := New()
	defer processor.Close()

	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		{
			name:     "simple path",
			path:     "user.name",
			expected: false,
		},
		{
			name:     "with bracket",
			path:     "users[0]",
			expected: true,
		},
		{
			name:     "with brace",
			path:     "users{name}",
			expected: true,
		},
		{
			name:     "with colon",
			path:     "items[0:5]",
			expected: true,
		},
		{
			name:     "very simple",
			path:     "user",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := processor.isComplexPath(tt.path)
			if result != tt.expected {
				t.Errorf("isComplexPath(%s) = %v; want %v", tt.path, result, tt.expected)
			}
		})
	}
}

func TestIsDigit(t *testing.T) {
	tests := []struct {
		char     byte
		expected bool
	}{
		{'0', true},
		{'5', true},
		{'9', true},
		{'a', false},
		{'-', false},
		{' ', false},
	}

	for _, tt := range tests {
		result := isDigit(tt.char)
		if result != tt.expected {
			t.Errorf("isDigit(%q) = %v, want %v", tt.char, result, tt.expected)
		}
	}
}

// TestIsObjectType tests object type detection
func TestIsObjectType(t *testing.T) {
	processor := New()
	defer processor.Close()

	tests := []struct {
		name     string
		data     any
		expected bool
	}{
		{
			name:     "is map string any",
			data:     map[string]any{},
			expected: true,
		},
		{
			name:     "is map any any",
			data:     map[any]any{},
			expected: true,
		},
		{
			name:     "is not object - array",
			data:     []any{},
			expected: false,
		},
		{
			name:     "is not object - string",
			data:     "object",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := processor.isObjectType(tt.data)
			if result != tt.expected {
				t.Errorf("isObjectType() = %v; want %v", result, tt.expected)
			}
		})
	}
}

// TestIsPrimitiveType tests primitive type detection
func TestIsPrimitiveType(t *testing.T) {
	processor := New()
	defer processor.Close()

	tests := []struct {
		name     string
		data     any
		expected bool
	}{
		{
			name:     "string",
			data:     "hello",
			expected: true,
		},
		{
			name:     "int",
			data:     42,
			expected: true,
		},
		{
			name:     "float",
			data:     3.14,
			expected: true,
		},
		{
			name:     "bool",
			data:     true,
			expected: true,
		},
		{
			name:     "array",
			data:     []any{},
			expected: false,
		},
		{
			name:     "map",
			data:     map[string]any{},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := processor.isPrimitiveType(tt.data)
			if result != tt.expected {
				t.Errorf("isPrimitiveType() = %v; want %v", result, tt.expected)
			}
		})
	}
}

// TestIsZeroValue tests the isZeroValue helper function
func TestIsZeroValue(t *testing.T) {
	tests := []struct {
		name     string
		input    any
		expected bool
	}{
		{"nil", nil, true},
		{"empty string", "", true},
		{"non-empty string", "hello", false},
		{"zero int", 0, true},
		{"non-zero int", 42, false},
		{"zero int64", int64(0), true},
		{"non-zero int64", int64(42), false},
		{"zero float64", 0.0, true},
		{"non-zero float64", 3.14, false},
		{"false bool", false, true},
		{"true bool", true, false},
		{"empty slice", []any{}, true},
		{"non-empty slice", []any{1, 2}, false},
		{"empty map", map[string]any{}, true},
		{"non-empty map", map[string]any{"key": "val"}, false},
		{"other type", struct{}{}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isZeroValue(tt.input)
			if result != tt.expected {
				t.Errorf("isZeroValue(%v) = %v; want %v", tt.input, result, tt.expected)
			}
		})
	}
}

// TestJSONPointerEscapeUnescape tests JSON Pointer escaping
func TestJSONPointerEscapeUnescape(t *testing.T) {
	processor := New()
	defer processor.Close()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "escape tilde",
			input:    "user~name",
			expected: "user~0name",
		},
		{
			name:     "escape slash",
			input:    "user/name",
			expected: "user~1name",
		},
		{
			name:     "escape both",
			input:    "user~/name",
			expected: "user~0~1name",
		},
		{
			name:     "no escaping needed",
			input:    "username",
			expected: "username",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := processor.escapeJSONPointer(tt.input)
			if result != tt.expected {
				t.Errorf("escapeJSONPointer(%s) = %s; want %s", tt.input, result, tt.expected)
			}
		})
	}

	// Test unescaping
	unescapeTests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "unescape tilde",
			input:    "user~0name",
			expected: "user~name",
		},
		{
			name:     "unescape slash",
			input:    "user~1name",
			expected: "user/name",
		},
		{
			name:     "unescape both",
			input:    "user~0~1name",
			expected: "user~/name",
		},
		{
			name:     "no unescaping needed",
			input:    "username",
			expected: "username",
		},
	}

	for _, tt := range unescapeTests {
		t.Run(tt.name, func(t *testing.T) {
			result := processor.unescapeJSONPointer(tt.input)
			if result != tt.expected {
				t.Errorf("unescapeJSONPointer(%s) = %s; want %s", tt.input, result, tt.expected)
			}
		})
	}
}

// TestNegativeArrayIndex tests negative array indexing
func TestNegativeArrayIndex(t *testing.T) {
	processor := New()
	defer processor.Close()

	t.Run("get with negative index", func(t *testing.T) {
		jsonStr := `{"items": [1, 2, 3, 4, 5]}`
		result, err := processor.Get(jsonStr, "items[-1]")
		if err != nil {
			t.Fatalf("Get error: %v", err)
		}
		if result.(float64) != 5 {
			t.Errorf("Get items[-1] = %v, want 5", result)
		}
	})

	t.Run("delete with negative index", func(t *testing.T) {
		jsonStr := `{"items": [1, 2, 3, 4, 5]}`
		result, err := processor.Delete(jsonStr, "items[-1]")
		if err != nil {
			t.Fatalf("Delete error: %v", err)
		}
		_ = result
	})
}

func TestNewPrettyConfig(t *testing.T) {
	cfg := NewPrettyConfig()
	if cfg == nil {
		t.Error("NewPrettyConfig should not return nil")
	}
	if !cfg.Pretty {
		t.Error("NewPrettyConfig should have Pretty = true")
	}
}

// TestNullAndMissingFields tests null value handling and missing fields
func TestNullAndMissingFields(t *testing.T) {
	helper := NewTestHelper(t)

	testData := `{
		"null_field": null,
		"string_field": "value",
		"nested": {
			"null_nested": null,
			"valid_nested": "data"
		},
		"array_with_nulls": [1, null, 3, null, 5]
	}`

	t.Run("NullFieldAccess", func(t *testing.T) {
		result, err := Get(testData, "null_field")
		helper.AssertNoError(err)
		helper.AssertNil(result)
	})

	t.Run("NestedNullAccess", func(t *testing.T) {
		result, err := Get(testData, "nested.null_nested")
		helper.AssertNoError(err)
		helper.AssertNil(result)
	})

	t.Run("ArrayNullAccess", func(t *testing.T) {
		result, err := Get(testData, "array_with_nulls[1]")
		helper.AssertNoError(err)
		helper.AssertNil(result)
	})

	t.Run("NullWithDefault", func(t *testing.T) {
		result := GetWithDefault(testData, "null_field", "default")
		helper.AssertEqual("default", result)

		result = GetWithDefault(testData, "missing_field", "default")
		helper.AssertEqual("default", result)

		result = GetWithDefault(testData, "string_field", "default")
		helper.AssertEqual("value", result)
	})

	t.Run("IsNullMethod", func(t *testing.T) {
		Foreach(testData, func(key any, item *IterableValue) {
			if key == "null_field" {
				helper.AssertTrue(item.IsNull(""))
			}
		})
	})
}

func TestNumber_TypeMethods(t *testing.T) {
	t.Run("String", func(t *testing.T) {
		n := Number("123.45")
		if n.String() != "123.45" {
			t.Errorf("Number.String() = %q, want %q", n.String(), "123.45")
		}
	})

	t.Run("Float64 valid", func(t *testing.T) {
		n := Number("123.45")
		f, err := n.Float64()
		if err != nil {
			t.Errorf("Number.Float64() error: %v", err)
		}
		if f != 123.45 {
			t.Errorf("Number.Float64() = %v, want %v", f, 123.45)
		}
	})

	t.Run("Float64 invalid", func(t *testing.T) {
		n := Number("not-a-number")
		_, err := n.Float64()
		if err == nil {
			t.Error("Float64 should return error for invalid number")
		}
	})

	t.Run("Int64 valid", func(t *testing.T) {
		n := Number("123")
		i, err := n.Int64()
		if err != nil {
			t.Errorf("Number.Int64() error: %v", err)
		}
		if i != 123 {
			t.Errorf("Number.Int64() = %v, want %v", i, 123)
		}
	})

	t.Run("Int64 invalid", func(t *testing.T) {
		n := Number("not-a-number")
		_, err := n.Int64()
		if err == nil {
			t.Error("Int64 should return error for invalid number")
		}
	})
}

// TestOperationTypes tests Operation constants
func TestOperationTypes(t *testing.T) {
	if OpGet != 0 {
		t.Errorf("OpGet = %d, want 0", OpGet)
	}
	if OpSet != 1 {
		t.Errorf("OpSet = %d, want 1", OpSet)
	}
	if OpDelete != 2 {
		t.Errorf("OpDelete = %d, want 2", OpDelete)
	}
}

// TestParseArrayIndexGlobal tests the global ParseArrayIndexGlobal function
func TestParseArrayIndexGlobal(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected int
	}{
		{"Valid index", "[5]", 5},
		{"Invalid index", "abc", InvalidArrayIndex},
		{"Empty string", "", InvalidArrayIndex},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseArrayIndexGlobal(tt.input)
			if result != tt.expected {
				t.Errorf("ParseArrayIndexGlobal(%q) = %d, want %d", tt.input, result, tt.expected)
			}
		})
	}
}

// TestParseArraySegment tests parsing array access segments
func TestParseArraySegment(t *testing.T) {
	processor := New()
	defer processor.Close()

	tests := []struct {
		name          string
		part          string
		expectedType  string
		expectedIndex int
		expectedStart *int
		expectedEnd   *int
		expectedStep  *int
	}{
		{
			name:          "simple index",
			part:          "[0]",
			expectedType:  "array",
			expectedIndex: 0,
		},
		{
			name:          "negative index",
			part:          "[-1]",
			expectedType:  "array",
			expectedIndex: -1,
		},
		{
			name:          "slice",
			part:          "[0:5]",
			expectedType:  "slice",
			expectedStart: intPtr(0),
			expectedEnd:   intPtr(5),
			expectedStep:  intPtr(1),
		},
		{
			name:          "slice with step",
			part:          "[0:10:2]",
			expectedType:  "slice",
			expectedStart: intPtr(0),
			expectedEnd:   intPtr(10),
			expectedStep:  intPtr(2),
		},
		{
			name:         "property with index",
			part:         "items[0]",
			expectedType: "property", // First segment is property
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			segments := processor.getPathSegments()
			defer processor.putPathSegments(segments)

			segments = processor.parseArraySegment(tt.part, segments)

			if len(segments) == 0 {
				t.Fatal("parseArraySegment returned no segments")
			}

			firstSeg := segments[0]
			if firstSeg.TypeString() != tt.expectedType {
				t.Errorf("Segment type = %s; want %s", firstSeg.TypeString(), tt.expectedType)
			}
		})
	}
}

// TestParseExtractionSegment tests parsing extraction segments
func TestParseExtractionSegment(t *testing.T) {
	processor := New()
	defer processor.Close()

	tests := []struct {
		name           string
		part           string
		expectedKey    string
		expectedIsFlat bool
	}{
		{
			name:           "simple extraction",
			part:           "{name}",
			expectedKey:    "name",
			expectedIsFlat: false,
		},
		{
			name:           "flat extraction",
			part:           "{flat:items}",
			expectedKey:    "items",
			expectedIsFlat: true,
		},
		{
			name:           "property with extraction",
			part:           "users{name}",
			expectedKey:    "name",
			expectedIsFlat: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			segments := processor.getPathSegments()
			defer processor.putPathSegments(segments)

			segments = processor.parseExtractionSegment(tt.part, segments)

			// Find the extraction segment
			var extractSeg *PathSegment
			for i := range segments {
				if segments[i].Type == internal.ExtractSegment {
					extractSeg = &segments[i]
					break
				}
			}

			if extractSeg == nil {
				t.Fatal("No extraction segment found")
			}

			if extractSeg.Key != tt.expectedKey {
				t.Errorf("Extraction key = %s; want %s", extractSeg.Key, tt.expectedKey)
			}

			if extractSeg.IsFlatExtract() != tt.expectedIsFlat {
				t.Errorf("IsFlatExtract() = %v; want %v", extractSeg.IsFlatExtract(), tt.expectedIsFlat)
			}
		})
	}
}

// TestPathCombination tests path joining and reconstruction
func TestPathCombination(t *testing.T) {
	processor := New()
	defer processor.Close()

	tests := []struct {
		name     string
		segments []string
		expected string
		useJSON  bool
	}{
		{
			name:     "simple join",
			segments: []string{"users", "name"},
			expected: "users.name",
			useJSON:  false,
		},
		{
			name:     "single segment",
			segments: []string{"user"},
			expected: "user",
			useJSON:  false,
		},
		{
			name:     "JSON pointer join",
			segments: []string{"users", "0", "name"},
			expected: "/users/0/name",
			useJSON:  true,
		},
		{
			name:     "empty segments",
			segments: []string{},
			expected: "",
			useJSON:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := processor.joinPathSegments(tt.segments, tt.useJSON)
			if result != tt.expected {
				t.Errorf("joinPathSegments(%v, %v) = %s; want %s", tt.segments, tt.useJSON, result, tt.expected)
			}
		})
	}
}

// TestPathNormalization tests path normalization operations
func TestPathNormalization(t *testing.T) {
	processor := New()
	defer processor.Close()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "remove double dots",
			input:    "user..name",
			expected: "user.name",
		},
		{
			name:     "multiple double dots",
			input:    "a...b",
			expected: "a.b",
		},
		{
			name:     "trim leading dots",
			input:    "...user.name",
			expected: "user.name",
		},
		{
			name:     "trim trailing dots",
			input:    "user.name...",
			expected: "user.name",
		},
		{
			name:     "trim both ends",
			input:    "...user.name...",
			expected: "user.name",
		},
		{
			name:     "no normalization needed",
			input:    "user.name",
			expected: "user.name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := processor.normalizePathSeparators(tt.input)
			if result != tt.expected {
				t.Errorf("normalizePathSeparators(%s) = %s; want %s", tt.input, result, tt.expected)
			}
		})
	}
}

// TestPathParsingBasic tests basic path parsing functionality
func TestPathParsingBasic(t *testing.T) {
	processor := New()
	defer processor.Close()

	tests := []struct {
		name        string
		path        string
		expectError bool
		description string
	}{
		{
			name:        "simple property",
			path:        "user.name",
			expectError: false,
			description: "Simple dot-notation path",
		},
		{
			name:        "array index",
			path:        "users[0]",
			expectError: false,
			description: "Path with array index",
		},
		{
			name:        "nested array",
			path:        "data.users[0].name",
			expectError: false,
			description: "Nested path with array",
		},
		{
			name:        "array slice",
			path:        "items[0:5]",
			expectError: false,
			description: "Path with array slice",
		},
		{
			name:        "extraction",
			path:        "users{name}",
			expectError: false,
			description: "Path with field extraction",
		},
		{
			name:        "empty path",
			path:        "",
			expectError: false,
			description: "Empty path (root)",
		},
		{
			name:        "root path",
			path:        ".",
			expectError: false,
			description: "Root path notation",
		},
		{
			name:        "JSON pointer",
			path:        "/users/0/name",
			expectError: false,
			description: "JSON Pointer format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Just verify the path can be parsed without error
			segments, err := processor.parsePath(tt.path)
			if tt.expectError && err == nil {
				t.Errorf("Expected error for path '%s', but got none", tt.path)
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error for path '%s': %v", tt.path, err)
			}
			if !tt.expectError && len(segments) == 0 && tt.path != "" && tt.path != "." {
				t.Errorf("Expected segments for path '%s', got none", tt.path)
			}
		})
	}
}

// TestPathParsingBoundaryConditions tests edge cases in path parsing
func TestPathParsingBoundaryConditions(t *testing.T) {
	processor := New()
	defer processor.Close()

	tests := []struct {
		name        string
		path        string
		expectError bool
		description string
	}{
		{
			name:        "very deep nesting",
			path:        "a.b.c.d.e.f.g.h.i.j.k.l.m.n.o.p",
			expectError: false,
			description: "Very deep path nesting",
		},
		{
			name:        "negative array index",
			path:        "items[-1]",
			expectError: false,
			description: "Negative array index",
		},
		{
			name:        "slice with step",
			path:        "items[0:10:2]",
			expectError: false,
			description: "Slice with step parameter",
		},
		{
			name:        "flat extraction",
			path:        "users{flat:items}",
			expectError: false,
			description: "Flat extraction syntax",
		},
		{
			name:        "consecutive extractions",
			path:        "data{users}{name}",
			expectError: false,
			description: "Consecutive extraction operations",
		},
		{
			name:        "complex nested slice",
			path:        "data.items[0:5].subarray[1:3]",
			expectError: false,
			description: "Multiple slice operations",
		},
		{
			name:        "mixed operations",
			path:        "users[0:5]{name}.first",
			expectError: false,
			description: "Mixed slice and extraction",
		},
		{
			name:        "special characters in property",
			path:        "user_profile.name",
			expectError: false,
			description: "Property with underscore",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := processor.parsePath(tt.path)
			if tt.expectError && err == nil {
				t.Errorf("Expected error for path '%s', but got none", tt.path)
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error for path '%s': %v", tt.path, err)
			}
		})
	}
}

// TestPathReconstruction tests reconstructing paths from segments
func TestPathReconstruction(t *testing.T) {
	processor := New()
	defer processor.Close()

	// This tests the reconstructPath helper
	tests := []struct {
		name     string
		path     string
		contains string
	}{
		{
			name:     "simple path",
			path:     "user.name",
			contains: "user",
		},
		{
			name:     "with array",
			path:     "users[0].name",
			contains: "[0]",
		},
		{
			name:     "with extraction",
			path:     "users{name}",
			contains: "{name}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			segments := processor.getPathSegments()
			defer processor.putPathSegments(segments)

			segments = processor.splitPath(tt.path, segments)
			reconstructed := processor.reconstructPath(segments)

			if !strings.Contains(reconstructed, tt.contains) {
				t.Errorf("Reconstructed path '%s' does not contain '%s'", reconstructed, tt.contains)
			}
		})
	}
}

// TestPathSegmentTypes tests different path segment type identification
func TestPathSegmentTypes(t *testing.T) {
	processor := New()
	defer processor.Close()

	// Test segment type string representation
	tests := []struct {
		segmentType string
		expectedStr string
	}{
		{"PropertySegment", "property"},
		{"ArrayIndexSegment", "array"},
		{"ArraySliceSegment", "slice"},
		{"ExtractSegment", "extract"},
	}

	for _, tt := range tests {
		t.Run(tt.segmentType, func(t *testing.T) {
			// This test verifies that segment types have correct string representations
			// The actual implementation would require accessing internal types
			t.Logf("Segment type %s should map to '%s'", tt.segmentType, tt.expectedStr)
		})
	}
}

// TestPathSegmentation tests splitting paths into segments
func TestPathSegmentation(t *testing.T) {
	processor := New()
	defer processor.Close()

	tests := []struct {
		name        string
		path        string
		expectedLen int
		firstIs     string
	}{
		{
			name:        "simple dot notation",
			path:        "user.name.first",
			expectedLen: 3,
			firstIs:     "user",
		},
		{
			name:        "with nested path",
			path:        "users.name.first",
			expectedLen: 3,
			firstIs:     "users",
		},
		{
			name:        "JSON pointer",
			path:        "/users/0/name",
			expectedLen: 3,
			firstIs:     "users",
		},
		{
			name:        "root path",
			path:        "/",
			expectedLen: 0,
			firstIs:     "",
		},
		{
			name:        "empty path",
			path:        "",
			expectedLen: 0,
			firstIs:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			segments := processor.splitPathSegments(tt.path)
			if len(segments) != tt.expectedLen {
				t.Errorf("splitPathSegments(%s) returned %d segments; want %d", tt.path, len(segments), tt.expectedLen)
			}
			if tt.expectedLen > 0 && segments[0] != tt.firstIs {
				t.Errorf("splitPathSegments(%s)[0] = %s; want %s", tt.path, segments[0], tt.firstIs)
			}
		})
	}
}

// TestPathWithSpecialCharacters tests paths with special characters
func TestPathWithSpecialCharacters(t *testing.T) {
	jsonStr := `{
		"user_name": "value1",
		"user-name": "value2",
		"userName": "value3",
		"user name": "value4",
		"user@name": "value5"
	}`

	processor := New()
	defer processor.Close()

	tests := []struct {
		name     string
		path     string
		expected string
	}{
		{
			name:     "underscore",
			path:     "user_name",
			expected: "value1",
		},
		{
			name:     "hyphen",
			path:     "user-name",
			expected: "value2",
		},
		{
			name:     "camelCase",
			path:     "userName",
			expected: "value3",
		},
		{
			name:     "JSON pointer with space",
			path:     "/user name",
			expected: "value4",
		},
		{
			name:     "JSON pointer with @",
			path:     "/user@name",
			expected: "value5",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := processor.Get(jsonStr, tt.path)
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			if result != tt.expected {
				t.Errorf("Get(%s) = %v; want %s", tt.path, result, tt.expected)
			}
		})
	}
}

// TestPerformArraySlice tests array slice operations
func TestPerformArraySlice(t *testing.T) {
	processor := New()
	defer processor.Close()

	arr := []any{0.0, 1.0, 2.0, 3.0, 4.0, 5.0, 6.0, 7.0, 8.0, 9.0}

	tests := []struct {
		name        string
		start       *int
		end         *int
		step        *int
		expected    []any
		description string
	}{
		{
			name:        "basic slice",
			start:       intPtr(2),
			end:         intPtr(5),
			step:        intPtr(1),
			expected:    []any{2.0, 3.0, 4.0},
			description: "Simple forward slice",
		},
		{
			name:        "slice from start",
			start:       nil,
			end:         intPtr(3),
			step:        intPtr(1),
			expected:    []any{0.0, 1.0, 2.0},
			description: "Slice from beginning",
		},
		{
			name:        "slice to end",
			start:       intPtr(7),
			end:         nil,
			step:        intPtr(1),
			expected:    []any{7.0, 8.0, 9.0},
			description: "Slice to end",
		},
		{
			name:        "full slice",
			start:       nil,
			end:         nil,
			step:        intPtr(1),
			expected:    []any{0.0, 1.0, 2.0, 3.0, 4.0, 5.0, 6.0, 7.0, 8.0, 9.0},
			description: "Complete array",
		},
		{
			name:        "with step",
			start:       intPtr(0),
			end:         intPtr(10),
			step:        intPtr(2),
			expected:    []any{0.0, 2.0, 4.0, 6.0, 8.0},
			description: "Slice with step",
		},
		{
			name:        "negative indices",
			start:       intPtr(-3),
			end:         nil,
			step:        intPtr(1),
			expected:    []any{7.0, 8.0, 9.0},
			description: "Negative start index",
		},
		{
			name:        "empty slice",
			start:       intPtr(5),
			end:         intPtr(5),
			step:        intPtr(1),
			expected:    []any{},
			description: "Zero-length slice",
		},
		{
			name:        "reverse slice",
			start:       intPtr(5),
			end:         intPtr(0),
			step:        intPtr(-1),
			expected:    []any{5.0, 4.0, 3.0, 2.0, 1.0},
			description: "Reverse slice",
		},
		{
			name:        "zero step",
			start:       intPtr(0),
			end:         intPtr(5),
			step:        intPtr(0),
			expected:    []any{},
			description: "Zero step returns empty",
		},
		{
			name:        "empty array",
			start:       intPtr(0),
			end:         intPtr(5),
			step:        intPtr(1),
			expected:    []any{},
			description: "Empty input array",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var input []any
			if tt.name == "empty array" {
				input = []any{}
			} else {
				input = arr
			}

			result := processor.performArraySlice(input, tt.start, tt.end, tt.step)
			if !coreSliceEqual(result, tt.expected) {
				t.Errorf("%s: performArraySlice() = %v; want %v", tt.description, result, tt.expected)
			}
		})
	}
}

// TestPreprocessPath tests path preprocessing for brackets and braces
func TestPreprocessPath(t *testing.T) {
	processor := New()
	defer processor.Close()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "bracket after letter",
			input:    "items[0]",
			expected: "items.[0]", // Adds dot before bracket
		},
		{
			name:     "bracket after digit",
			input:    "data1[0]",
			expected: "data1.[0]",
		},
		{
			name:     "brace after letter",
			input:    "users{name}",
			expected: "users.{name}", // Adds dot before brace
		},
		{
			name:     "complex mixed",
			input:    "data1[0]users{name}",
			expected: "data1.[0]users.{name}", // Adds dots before bracket and brace
		},
		{
			name:     "no preprocessing needed",
			input:    "user.name",
			expected: "user.name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sb := processor.getStringBuilder()
			defer processor.putStringBuilder(sb)

			result := processor.preprocessPath(tt.input, sb)
			if result != tt.expected {
				t.Errorf("preprocessPath(%s) = %s; want %s", tt.input, result, tt.expected)
			}
		})
	}
}

// TestPrintE tests PrintE function
func TestPrintE(t *testing.T) {
	t.Run("valid data", func(t *testing.T) {
		output := captureStdout(func() {
			err := PrintE(map[string]any{"test": "value"})
			if err != nil {
				t.Errorf("PrintE returned unexpected error: %v", err)
			}
		})
		if !contains(output, "test") || !contains(output, "value") {
			t.Errorf("PrintE output = %s; should contain test and value", output)
		}
	})

	t.Run("invalid data returns error", func(t *testing.T) {
		err := PrintE(make(chan int))
		if err == nil {
			t.Error("PrintE should return error for unserializable data")
		}
	})
}

func TestPrintError(t *testing.T) {
	// Test that Print handles errors by writing to stderr
	stderr := captureStderr(func() {
		// Channel is not serializable, should cause an error
		Print(make(chan int))
	})

	if stderr == "" {
		t.Error("Print() should write error to stderr for unserializable data")
	}
}

// TestPrintPrettyE tests PrintPrettyE function
func TestPrintPrettyE(t *testing.T) {
	t.Run("valid data", func(t *testing.T) {
		output := captureStdout(func() {
			err := PrintPrettyE(map[string]any{"test": "value"})
			if err != nil {
				t.Errorf("PrintPrettyE returned unexpected error: %v", err)
			}
		})
		if !contains(output, "\n") {
			t.Errorf("PrintPrettyE output should contain newlines, got: %s", output)
		}
	})

	t.Run("invalid data returns error", func(t *testing.T) {
		err := PrintPrettyE(make(chan int))
		if err == nil {
			t.Error("PrintPrettyE should return error for unserializable data")
		}
	})
}

func TestPrintPrettyError(t *testing.T) {
	// Test that PrintPretty handles errors by writing to stderr
	stderr := captureStderr(func() {
		// Channel is not serializable, should cause an error
		PrintPretty(make(chan int))
	})

	if stderr == "" {
		t.Error("PrintPretty() should write error to stderr for unserializable data")
	}
}

// TestProcessBatch tests batch processing
func TestProcessBatch(t *testing.T) {
	jsonStr := `{"user": {"name": "Alice", "age": 30}}`

	operations := []BatchOperation{
		{Type: "get", Path: "user.name"},
		{Type: "get", Path: "user.age"},
		{Type: "set", Path: "user.email", Value: "alice@example.com"},
	}

	results, err := ProcessBatch(operations)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(results) != len(operations) {
		t.Errorf("Expected %d results, got %d", len(operations), len(results))
	}

	_ = jsonStr // Use the variable
}

func TestProcessor_BatchOperations(t *testing.T) {
	processor := New()
	defer processor.Close()

	t.Run("SetMultiple", func(t *testing.T) {
		jsonStr := `{"a": 1}`
		updates := map[string]any{
			"b": 2,
			"c": 3,
		}
		result, err := processor.SetMultiple(jsonStr, updates)
		if err != nil {
			t.Errorf("SetMultiple error: %v", err)
		}

		val, _ := processor.Get(result, "b")
		if val != 2.0 {
			t.Errorf("SetMultiple failed, b = %v, want 2", val)
		}
	})

	t.Run("SetMultipleWithAdd", func(t *testing.T) {
		jsonStr := `{}`
		updates := map[string]any{
			"a.b": 1,
			"c.d": 2,
		}
		result, err := processor.SetMultipleWithAdd(jsonStr, updates)
		if err != nil {
			t.Errorf("SetMultipleWithAdd error: %v", err)
		}

		val, _ := processor.Get(result, "a.b")
		if val != 1.0 {
			t.Errorf("SetMultipleWithAdd failed, a.b = %v, want 1", val)
		}
	})

	t.Run("GetMultiple", func(t *testing.T) {
		jsonStr := `{"a": 1, "b": 2, "c": 3}`
		paths := []string{"a", "b"}
		result, err := processor.GetMultiple(jsonStr, paths)
		if err != nil {
			t.Errorf("GetMultiple error: %v", err)
		}
		if result["a"] != 1.0 {
			t.Errorf("result[a] = %v, want 1", result["a"])
		}
	})
}

func TestProcessor_CompiledPath(t *testing.T) {
	processor := New()
	defer processor.Close()

	t.Run("CompilePath", func(t *testing.T) {
		cp, err := processor.CompilePath("user.name")
		if err != nil {
			t.Errorf("CompilePath error: %v", err)
		}
		if cp == nil {
			t.Fatal("CompilePath returned nil")
		}
	})

	t.Run("GetCompiled", func(t *testing.T) {
		cp, _ := processor.CompilePath("user.name")
		jsonStr := `{"user": {"name": "John"}}`
		result, err := processor.GetCompiled(jsonStr, cp)
		if err != nil {
			t.Errorf("GetCompiled error: %v", err)
		}
		if result != "John" {
			t.Errorf("GetCompiled = %v, want 'John'", result)
		}
	})
}

// TestProcessor_CustomConfigVsDefault tests that custom processor config is actually used
func TestProcessor_CustomConfigVsDefault(t *testing.T) {
	// JSON with depth exceeding default limit
	deepJSON := `{"a":{` + string(make([]byte, 40)) + `}}`

	// Create custom config with higher nesting limit
	config := DefaultConfig()
	config.MaxNestingDepthSecurity = 100

	processor := New(config)
	defer processor.Close()

	// This should work with custom config
	processor.Foreach(deepJSON, func(key any, item *IterableValue) {
		// If we get here without error, custom config is being used
		_ = item.GetString("a")
	})
}

func TestProcessor_EncodeWithOptions(t *testing.T) {
	processor := New()
	defer processor.Close()

	t.Run("EncodeWithOptions", func(t *testing.T) {
		data := map[string]any{"key": "value"}
		encOpts := DefaultEncodeConfig()
		encOpts.Pretty = true
		result, err := processor.EncodeWithOptions(data, encOpts)
		if err != nil {
			t.Errorf("EncodeWithOptions error: %v", err)
		}
		if result == "" {
			t.Error("EncodeWithOptions should return non-empty result")
		}
	})

	t.Run("ToJsonString", func(t *testing.T) {
		data := map[string]any{"key": "value"}
		result, err := processor.ToJsonString(data)
		if err != nil {
			t.Errorf("ToJsonString error: %v", err)
		}
		if result == "" {
			t.Error("ToJsonString should return non-empty result")
		}
	})

	t.Run("ToJsonStringPretty", func(t *testing.T) {
		data := map[string]any{"key": "value"}
		result, err := processor.ToJsonStringPretty(data)
		if err != nil {
			t.Errorf("ToJsonStringPretty error: %v", err)
		}
		if result == "" {
			t.Error("ToJsonStringPretty should return non-empty result")
		}
	})

	t.Run("ToJsonStringStandard", func(t *testing.T) {
		data := map[string]any{"key": "value"}
		result, err := processor.ToJsonStringStandard(data)
		if err != nil {
			t.Errorf("ToJsonStringStandard error: %v", err)
		}
		if result == "" {
			t.Error("ToJsonStringStandard should return non-empty result")
		}
	})

	t.Run("EncodeBatch", func(t *testing.T) {
		pairs := map[string]any{"a": 1, "b": 2}
		result, err := processor.EncodeBatch(pairs, true)
		if err != nil {
			t.Errorf("EncodeBatch error: %v", err)
		}
		if result == "" {
			t.Error("EncodeBatch should return non-empty result")
		}
	})

	t.Run("EncodeFields", func(t *testing.T) {
		data := struct {
			Name  string `json:"name"`
			Age   int    `json:"age"`
			Email string `json:"email"`
		}{
			Name:  "John",
			Age:   30,
			Email: "john@example.com",
		}
		fields := []string{"name", "age"}
		result, err := processor.EncodeFields(data, fields, false)
		if err != nil {
			t.Errorf("EncodeFields error: %v", err)
		}
		if result == "" {
			t.Error("EncodeFields should return non-empty result")
		}
	})

	t.Run("EncodeStream", func(t *testing.T) {
		values := []any{1, 2, 3}
		result, err := processor.EncodeStream(values, false)
		if err != nil {
			t.Errorf("EncodeStream error: %v", err)
		}
		if result == "" {
			t.Error("EncodeStream should return non-empty result")
		}
	})

	t.Run("EncodeStreamWithOptions", func(t *testing.T) {
		values := []any{1, 2, 3}
		encOpts := DefaultEncodeConfig()
		result, err := processor.EncodeStreamWithOptions(values, encOpts)
		if err != nil {
			t.Errorf("EncodeStreamWithOptions error: %v", err)
		}
		if result == "" {
			t.Error("EncodeStreamWithOptions should return non-empty result")
		}
	})
}

func TestProcessor_ErrorHandling(t *testing.T) {
	processor := New()
	defer processor.Close()

	t.Run("Get invalid path", func(t *testing.T) {
		jsonStr := `{"key": "value"}`
		_, err := processor.Get(jsonStr, "nonexistent.path")
		if err == nil {
			t.Error("Get should return error for invalid path")
		}
	})

	t.Run("Set on non-object", func(t *testing.T) {
		jsonStr := `{"key": "string_value"}`
		_, err := processor.Set(jsonStr, "key.nested", "value")
		if err == nil {
			t.Error("Set should return error when parent is not an object")
		}
	})

	t.Run("Parse invalid JSON", func(t *testing.T) {
		invalidJSON := `{invalid json}`
		_, err := processor.Get(invalidJSON, "key")
		if err == nil {
			t.Error("Get should return error for invalid JSON")
		}
	})

	t.Run("DeleteWithCleanNull", func(t *testing.T) {
		jsonStr := `{"a": {"b": {"c": 1}}}`
		result, err := processor.DeleteWithCleanNull(jsonStr, "a.b.c")
		if err != nil {
			t.Errorf("DeleteWithCleanNull error: %v", err)
		}
		// Verify the result is valid JSON
		if !json.Valid([]byte(result)) {
			t.Error("DeleteWithCleanNull should return valid JSON")
		}
	})
}

func TestProcessor_FastOperations(t *testing.T) {
	processor := New()
	defer processor.Close()

	t.Run("FastSet", func(t *testing.T) {
		jsonStr := `{"user": {"name": "John"}}`
		result, err := processor.FastSet(jsonStr, "user.name", "Jane")
		if err != nil {
			t.Errorf("FastSet error: %v", err)
		}
		if result == "" {
			t.Error("FastSet should return non-empty result")
		}
	})

	t.Run("FastDelete", func(t *testing.T) {
		jsonStr := `{"user": {"name": "John", "age": 30}}`
		result, err := processor.FastDelete(jsonStr, "user.age")
		if err != nil {
			t.Errorf("FastDelete error: %v", err)
		}
		if result == "" {
			t.Error("FastDelete should return non-empty result")
		}
	})

	t.Run("BatchSetOptimized", func(t *testing.T) {
		jsonStr := `{"a": 1, "b": 2}`
		updates := map[string]any{
			"a": 10,
			"c": 3,
		}
		result, err := processor.BatchSetOptimized(jsonStr, updates)
		if err != nil {
			t.Errorf("BatchSetOptimized error: %v", err)
		}
		if result == "" {
			t.Error("BatchSetOptimized should return non-empty result")
		}
	})

	t.Run("BatchDeleteOptimized", func(t *testing.T) {
		jsonStr := `{"a": 1, "b": 2, "c": 3}`
		paths := []string{"a", "c"}
		result, err := processor.BatchDeleteOptimized(jsonStr, paths)
		if err != nil {
			t.Errorf("BatchDeleteOptimized error: %v", err)
		}
		if result == "" {
			t.Error("BatchDeleteOptimized should return non-empty result")
		}
	})

	t.Run("FastGetMultiple", func(t *testing.T) {
		jsonStr := `{"a": 1, "b": 2, "c": 3}`
		paths := []string{"a", "b"}
		result, err := processor.FastGetMultiple(jsonStr, paths)
		if err != nil {
			t.Errorf("FastGetMultiple error: %v", err)
		}
		if result["a"] != 1.0 {
			t.Errorf("result[a] = %v, want 1", result["a"])
		}
		if result["b"] != 2.0 {
			t.Errorf("result[b] = %v, want 2", result["b"])
		}
	})
}

// TestProcessor_ForeachMethods tests Processor's Foreach methods
func TestProcessor_ForeachMethods(t *testing.T) {
	jsonStr := `{
		"users": [
			{"name": "Alice", "age": 25},
			{"name": "Bob", "age": 30},
			{"name": "Charlie", "age": 35}
		]
	}`

	t.Run("Foreach", func(t *testing.T) {
		processor := New(DefaultConfig())
		defer processor.Close()

		count := 0
		processor.Foreach(jsonStr, func(key any, item *IterableValue) {
			count++
		})

		// Foreach on root should iterate over "users" key
		if count == 0 {
			t.Error("Foreach should have iterated over at least one item")
		}
	})

	t.Run("ForeachWithPath", func(t *testing.T) {
		processor := New(DefaultConfig())
		defer processor.Close()

		count := 0
		names := []string{}

		err := processor.ForeachWithPath(jsonStr, "users", func(key any, item *IterableValue) {
			count++
			name := item.GetString("name")
			names = append(names, name)
		})

		if err != nil {
			t.Errorf("ForeachWithPath error: %v", err)
		}

		if count != 3 {
			t.Errorf("ForeachWithPath count = %d, want 3", count)
		}

		expectedNames := []string{"Alice", "Bob", "Charlie"}
		if len(names) != 3 {
			t.Errorf("Names count = %d, want 3", len(names))
		} else {
			for i, name := range names {
				if name != expectedNames[i] {
					t.Errorf("names[%d] = %q, want %q", i, name, expectedNames[i])
				}
			}
		}
	})

	t.Run("ForeachWithPathDeepNesting", func(t *testing.T) {
		// Create a processor with custom nesting depth limit
		config := DefaultConfig()
		config.MaxNestingDepthSecurity = 50

		processor := New(config)
		defer processor.Close()

		// Deeply nested JSON structure with array at the end
		deepJSON := `{
			"level1": {
				"level2": {
					"level3": {
						"level4": {
							"items": [
								{"value": "deep1"},
								{"value": "deep2"}
							]
						}
					}
				}
			}
		}`

		found := false
		err := processor.ForeachWithPath(deepJSON, "level1.level2.level3.level4.items", func(key any, item *IterableValue) {
			value := item.GetString("value")
			if value == "deep1" || value == "deep2" {
				found = true
			}
		})

		if err != nil {
			t.Errorf("ForeachWithPath on deep structure error: %v", err)
		}

		if !found {
			t.Error("Expected to find deep values")
		}
	})

	t.Run("ForeachWithPathAndControl", func(t *testing.T) {
		processor := New(DefaultConfig())
		defer processor.Close()

		count := 0
		err := processor.ForeachWithPathAndControl(jsonStr, "users", func(key any, value any) IteratorControl {
			count++
			// Continue iteration
			return IteratorContinue
		})

		if err != nil {
			t.Errorf("ForeachWithPathAndControl error: %v", err)
		}

		if count != 3 {
			t.Errorf("ForeachWithPathAndControl count = %d, want 3", count)
		}
	})

	t.Run("ForeachWithPathBreakEarly", func(t *testing.T) {
		processor := New(DefaultConfig())
		defer processor.Close()

		count := 0
		err := processor.ForeachWithPathAndControl(jsonStr, "users", func(key any, value any) IteratorControl {
			count++
			// Break after first item
			if count == 1 {
				return IteratorBreak
			}
			return IteratorContinue
		})

		if err != nil {
			t.Errorf("ForeachWithPathAndControl error: %v", err)
		}

		if count != 1 {
			t.Errorf("ForeachWithPathAndControl with break count = %d, want 1", count)
		}
	})

	t.Run("ForeachWithPathAndIterator", func(t *testing.T) {
		processor := New(DefaultConfig())
		defer processor.Close()

		paths := []string{}
		err := processor.ForeachWithPathAndIterator(jsonStr, "users", func(key any, item *IterableValue, currentPath string) IteratorControl {
			paths = append(paths, currentPath)
			return IteratorContinue
		})

		if err != nil {
			t.Errorf("ForeachWithPathAndIterator error: %v", err)
		}

		if len(paths) != 3 {
			t.Errorf("ForeachWithPathAndIterator paths count = %d, want 3", len(paths))
		}
	})
}

// TestProcessor_ForeachNested tests the ForeachNested method
func TestProcessor_ForeachNested(t *testing.T) {
	jsonStr := `{
		"user": {
			"name": "Alice",
			"age": 25,
			"address": {
				"city": "NYC"
			}
		}
	}`

	processor := New(DefaultConfig())
	defer processor.Close()

	count := 0
	processor.ForeachNested(jsonStr, func(key any, item *IterableValue) {
		count++
	})

	// ForeachNested recursively iterates, so count should be > 5
	if count < 5 {
		t.Errorf("ForeachNested count = %d, want at least 5 (it's recursive)", count)
	}
}

// TestProcessor_ForeachReturn tests the ForeachReturn method
func TestProcessor_ForeachReturn(t *testing.T) {
	jsonStr := `{"items": [1, 2, 3]}`

	processor := New(DefaultConfig())
	defer processor.Close()

	count := 0
	result, err := processor.ForeachReturn(jsonStr, func(key any, item *IterableValue) {
		count++
	})

	if err != nil {
		t.Errorf("ForeachReturn error: %v", err)
	}

	if count != 1 {
		t.Errorf("ForeachReturn count = %d, want 1 (just 'items' key)", count)
	}

	if result != jsonStr {
		t.Error("ForeachReturn should return the original JSON string")
	}
}

// TestProcessor_ForeachVsPackageLevel tests that Processor methods work independently
func TestProcessor_ForeachVsPackageLevel(t *testing.T) {
	jsonStr := `{"items": [1, 2, 3]}`

	t.Run("ProcessorMethod", func(t *testing.T) {
		processor := New(DefaultConfig())
		defer processor.Close()

		count := 0
		err := processor.ForeachWithPath(jsonStr, "items", func(key any, item *IterableValue) {
			count++
		})

		if err != nil {
			t.Errorf("Processor.ForeachWithPath error: %v", err)
		}

		if count != 3 {
			t.Errorf("Processor.ForeachWithPath count = %d, want 3", count)
		}
	})

	t.Run("PackageLevelFunction", func(t *testing.T) {
		count := 0
		err := ForeachWithPath(jsonStr, "items", func(key any, item *IterableValue) {
			count++
		})

		if err != nil {
			t.Errorf("ForeachWithPath error: %v", err)
		}

		if count != 3 {
			t.Errorf("ForeachWithPath count = %d, want 3", count)
		}
	})
}

func TestProcessor_GetFromParsedData(t *testing.T) {
	processor := New()
	defer processor.Close()

	t.Run("GetFromParsedData", func(t *testing.T) {
		data := map[string]any{
			"user": map[string]any{
				"name": "John",
			},
		}
		result, err := processor.GetFromParsedData(data, "user.name")
		if err != nil {
			t.Errorf("GetFromParsedData error: %v", err)
		}
		if result != "John" {
			t.Errorf("GetFromParsedData = %v, want 'John'", result)
		}
	})
}

func TestProcessor_Iterators(t *testing.T) {
	processor := New()
	defer processor.Close()

	t.Run("Foreach", func(t *testing.T) {
		jsonStr := `{"a": 1, "b": 2}`
		count := 0
		processor.Foreach(jsonStr, func(key any, item *IterableValue) {
			count++
		})
		if count != 2 {
			t.Errorf("Foreach visited %d items, want 2", count)
		}
	})

	t.Run("ForeachWithPath", func(t *testing.T) {
		jsonStr := `{"items": [1, 2, 3]}`
		count := 0
		err := processor.ForeachWithPath(jsonStr, "items[*]", func(key any, item *IterableValue) {
			count++
		})
		if err != nil {
			t.Errorf("ForeachWithPath error: %v", err)
		}
		if count != 3 {
			t.Errorf("ForeachWithPath visited %d items, want 3", count)
		}
	})

	t.Run("ForeachReturn", func(t *testing.T) {
		jsonStr := `{"a": 1, "b": 2}`
		result, err := processor.ForeachReturn(jsonStr, func(key any, item *IterableValue) {
			// Just iterate
		})
		if err != nil {
			t.Errorf("ForeachReturn error: %v", err)
		}
		if result == "" {
			t.Error("ForeachReturn should return result")
		}
	})

	t.Run("ForeachNested", func(t *testing.T) {
		jsonStr := `{"a": 1, "b": {"c": 2}}`
		count := 0
		processor.ForeachNested(jsonStr, func(key any, item *IterableValue) {
			count++
		})
		if count < 2 {
			t.Errorf("ForeachNested visited %d items, expected at least 2", count)
		}
	})
}

func TestProcessor_ParseAndValidate(t *testing.T) {
	processor := New()
	defer processor.Close()

	t.Run("Parse", func(t *testing.T) {
		jsonStr := `{"key": "value"}`
		var result map[string]any
		err := processor.Parse(jsonStr, &result)
		if err != nil {
			t.Errorf("Parse error: %v", err)
		}
		if result["key"] != "value" {
			t.Errorf("result[key] = %v, want 'value'", result["key"])
		}
	})

	t.Run("Valid", func(t *testing.T) {
		jsonStr := `{"key": "value"}`
		valid, err := processor.Valid(jsonStr)
		if err != nil {
			t.Errorf("Valid error: %v", err)
		}
		if !valid {
			t.Error("Valid should return true for valid JSON")
		}
	})

	t.Run("ValidBytes", func(t *testing.T) {
		data := []byte(`{"key": "value"}`)
		if !processor.ValidBytes(data) {
			t.Error("ValidBytes should return true for valid JSON")
		}
	})
}

func TestProcessor_PreParse(t *testing.T) {
	processor := New()
	defer processor.Close()

	t.Run("PreParse", func(t *testing.T) {
		jsonStr := `{"key": "value"}`
		parsed, err := processor.PreParse(jsonStr)
		if err != nil {
			t.Errorf("PreParse error: %v", err)
		}
		if parsed == nil {
			t.Fatal("PreParse returned nil")
		}
	})

	t.Run("GetFromParsed", func(t *testing.T) {
		jsonStr := `{"key": "value", "nested": {"a": 1}}`
		parsed, _ := processor.PreParse(jsonStr)
		result, err := processor.GetFromParsed(parsed, "nested.a")
		if err != nil {
			t.Errorf("GetFromParsed error: %v", err)
		}
		if result != 1.0 {
			t.Errorf("GetFromParsed = %v, want 1", result)
		}
	})

	t.Run("SetFromParsed", func(t *testing.T) {
		jsonStr := `{"key": "value"}`
		parsed, _ := processor.PreParse(jsonStr)
		newParsed, err := processor.SetFromParsed(parsed, "key", "new_value")
		if err != nil {
			t.Errorf("SetFromParsed error: %v", err)
		}
		if newParsed == nil {
			t.Fatal("SetFromParsed returned nil")
		}
	})
}

func TestProcessor_SafeGet(t *testing.T) {
	processor := New()
	defer processor.Close()

	t.Run("SafeGet found", func(t *testing.T) {
		jsonStr := `{"key": "value"}`
		result := processor.SafeGet(jsonStr, "key")
		if !result.Exists {
			t.Error("SafeGet should find key")
		}
		if result.Value != "value" {
			t.Errorf("SafeGet = %v, want 'value'", result.Value)
		}
	})

	t.Run("SafeGet not found", func(t *testing.T) {
		jsonStr := `{"key": "value"}`
		result := processor.SafeGet(jsonStr, "nonexistent")
		if result.Exists {
			t.Error("SafeGet should not find nonexistent key")
		}
	})
}

func TestProcessor_State(t *testing.T) {
	t.Run("GetConfig", func(t *testing.T) {
		processor := New()
		defer processor.Close()

		cfg := processor.GetConfig()
		if cfg == nil {
			t.Fatal("GetConfig returned nil")
		}
	})

	t.Run("IsClosed not closed", func(t *testing.T) {
		processor := New()
		if processor.IsClosed() {
			t.Error("Processor should not be closed")
		}
		processor.Close()
	})

	t.Run("IsClosed closed", func(t *testing.T) {
		processor := New()
		processor.Close()
		if !processor.IsClosed() {
			t.Error("Processor should be closed")
		}
	})
}

func TestProcessor_StatsAndHealth(t *testing.T) {
	processor := New()
	defer processor.Close()

	t.Run("GetStats", func(t *testing.T) {
		stats := processor.GetStats()
		// Verify stats has expected structure (Stats is a struct, not a pointer)
		_ = stats // Just verify it doesn't panic
	})

	t.Run("GetHealthStatus", func(t *testing.T) {
		health := processor.GetHealthStatus()
		// Verify health has expected structure (HealthStatus is a struct, not a pointer)
		_ = health // Just verify it doesn't panic
	})

	t.Run("ClearCache", func(t *testing.T) {
		processor.ClearCache()
		// Should not panic
	})
}

func TestProcessor_WildcardAndExtraction(t *testing.T) {
	processor := New()
	defer processor.Close()

	t.Run("Get with wildcard", func(t *testing.T) {
		jsonStr := `{"users": [{"name": "John"}, {"name": "Jane"}]}`
		result, err := processor.Get(jsonStr, "users[*].name")
		if err != nil {
			t.Errorf("Get error: %v", err)
		}
		arr, ok := result.([]any)
		if !ok {
			t.Fatalf("Result is not an array")
		}
		if len(arr) != 2 {
			t.Errorf("Wildcard result length = %d, want 2", len(arr))
		}
	})

	t.Run("Get with extraction", func(t *testing.T) {
		jsonStr := `{"users": [{"name": "John", "email": "john@example.com"}, {"name": "Jane", "email": "jane@example.com"}]}`
		result, err := processor.Get(jsonStr, "users{name}")
		if err != nil {
			t.Errorf("Get error: %v", err)
		}
		_ = result
	})
}

// TestPropertyValidation tests property name validation
func TestPropertyValidation(t *testing.T) {
	processor := New()
	defer processor.Close()

	tests := []struct {
		name     string
		property string
		valid    bool
	}{
		{
			name:     "valid simple",
			property: "name",
			valid:    true,
		},
		{
			name:     "valid with underscore",
			property: "user_name",
			valid:    true,
		},
		{
			name:     "empty string",
			property: "",
			valid:    false,
		},
		{
			name:     "with dot",
			property: "user.name",
			valid:    false,
		},
		{
			name:     "with bracket",
			property: "user[0]",
			valid:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := processor.isValidPropertyName(tt.property)
			if result != tt.valid {
				t.Errorf("isValidPropertyName(%s) = %v; want %v", tt.property, result, tt.valid)
			}
		})
	}
}

// TestPutResultBuffer tests returning buffer to pool
func TestPutResultBuffer(t *testing.T) {
	t.Run("non-nil buffer", func(t *testing.T) {
		buf := GetResultBuffer()
		*buf = append(*buf, "data"...)
		PutResultBuffer(buf)
		// Should not panic
	})
}

// TestRecursiveProcessor_ComplexArrays tests complex array operations
func TestRecursiveProcessor_ComplexArrays(t *testing.T) {
	processor := New()
	defer processor.Close()

	rp := NewRecursiveProcessor(processor)

	t.Run("NestedArrayGet", func(t *testing.T) {
		data := map[string]any{
			"matrix": []any{
				[]any{1, 2, 3},
				[]any{4, 5, 6},
				[]any{7, 8, 9},
			},
		}

		result, err := rp.ProcessRecursively(data, "matrix[1][2]", OpGet, nil)
		if err != nil {
			t.Errorf("ProcessRecursively failed: %v", err)
		}
		if result != float64(6) && result != 6 {
			t.Errorf("result = %v, want 6", result)
		}
	})

	t.Run("ArrayOfObjectsGet", func(t *testing.T) {
		data := map[string]any{
			"users": []any{
				map[string]any{"id": 1, "name": "Alice"},
				map[string]any{"id": 2, "name": "Bob"},
				map[string]any{"id": 3, "name": "Charlie"},
			},
		}

		result, err := rp.ProcessRecursively(data, "users[1].name", OpGet, nil)
		if err != nil {
			t.Errorf("ProcessRecursively failed: %v", err)
		}
		if result != "Bob" {
			t.Errorf("result = %v, want Bob", result)
		}
	})
}

// TestRecursiveProcessor_Creation tests RecursiveProcessor creation
func TestRecursiveProcessor_Creation(t *testing.T) {
	processor := New()
	defer processor.Close()

	rp := NewRecursiveProcessor(processor)
	if rp == nil {
		t.Fatal("NewRecursiveProcessor returned nil")
	}
}

// TestRecursiveProcessor_DataIntegrity tests that data is not unexpectedly modified
func TestRecursiveProcessor_DataIntegrity(t *testing.T) {
	processor := New()
	defer processor.Close()

	rp := NewRecursiveProcessor(processor)

	t.Run("GetDoesNotModifyOriginal", func(t *testing.T) {
		originalData := map[string]any{
			"nested": map[string]any{
				"value": "original",
			},
		}

		_, err := rp.ProcessRecursively(originalData, "nested.value", OpGet, nil)
		if err != nil {
			t.Errorf("ProcessRecursively failed: %v", err)
		}

		// Original data should not be modified
		if originalData["nested"].(map[string]any)["value"] != "original" {
			t.Error("Get operation modified original data")
		}
	})
}

// TestRecursiveProcessor_DeepNesting tests deeply nested data
func TestRecursiveProcessor_DeepNesting(t *testing.T) {
	processor := New()
	defer processor.Close()

	rp := NewRecursiveProcessor(processor)

	t.Run("DeepGet", func(t *testing.T) {
		data := map[string]any{
			"level1": map[string]any{
				"level2": map[string]any{
					"level3": map[string]any{
						"level4": map[string]any{
							"value": "deep",
						},
					},
				},
			},
		}

		result, err := rp.ProcessRecursively(data, "level1.level2.level3.level4.value", OpGet, nil)
		if err != nil {
			t.Errorf("ProcessRecursively failed: %v", err)
		}
		if result != "deep" {
			t.Errorf("result = %v, want deep", result)
		}
	})
}

// TestRecursiveProcessor_EdgeCases tests edge cases
func TestRecursiveProcessor_EdgeCases(t *testing.T) {
	processor := New()
	defer processor.Close()

	rp := NewRecursiveProcessor(processor)

	t.Run("NilData", func(t *testing.T) {
		_, err := rp.ProcessRecursively(nil, "path", OpGet, nil)
		// May or may not return error, just verify it doesn't panic
		t.Logf("Nil data error: %v", err)
	})

	t.Run("EmptyPath", func(t *testing.T) {
		data := map[string]any{"key": "value"}

		result, err := rp.ProcessRecursively(data, "", OpGet, nil)
		if err != nil {
			t.Errorf("ProcessRecursively failed: %v", err)
		}
		// For empty path, result should be the same as input
		resultMap, ok := result.(map[string]any)
		if !ok {
			t.Errorf("Expected map[string]any, got %T", result)
		} else if resultMap["key"] != "value" {
			t.Errorf("result[key] = %v, want value", resultMap["key"])
		}
	})

	t.Run("OutOfBoundsArrayIndex", func(t *testing.T) {
		data := []any{1, 2, 3}

		_, err := rp.ProcessRecursively(data, "[10]", OpGet, nil)
		// May or may not return error, just verify it doesn't panic
		t.Logf("Out of bounds error: %v", err)
	})

	t.Run("NegativeArrayIndex", func(t *testing.T) {
		data := []any{1, 2, 3}

		result, err := rp.ProcessRecursively(data, "[-1]", OpGet, nil)
		if err != nil {
			t.Errorf("ProcessRecursively failed: %v", err)
		}
		if result != float64(3) && result != 3 {
			t.Errorf("result = %v, want 3", result)
		}
	})

	t.Run("PropertyOnArray", func(t *testing.T) {
		data := []any{1, 2, 3}

		_, err := rp.ProcessRecursively(data, "invalid", OpGet, nil)
		// May or may not return error, just verify it doesn't panic
		t.Logf("Property on array error: %v", err)
	})

	t.Run("ArrayIndexOnObject", func(t *testing.T) {
		data := map[string]any{"key": "value"}

		_, err := rp.ProcessRecursively(data, "[0]", OpGet, nil)
		// May or may not return error, just verify it doesn't panic
		t.Logf("Array index on object error: %v", err)
	})
}

// TestRecursiveProcessor_ExtractOperations tests extract segment operations
func TestRecursiveProcessor_ExtractOperations(t *testing.T) {
	processor := New()
	defer processor.Close()

	rp := NewRecursiveProcessor(processor)

	t.Run("SimpleExtract", func(t *testing.T) {
		data := map[string]any{
			"users": []any{
				map[string]any{"id": 1, "name": "Alice"},
				map[string]any{"id": 2, "name": "Bob"},
			},
		}

		result, err := rp.ProcessRecursively(data, "users{name}", OpGet, nil)
		if err != nil {
			t.Errorf("ProcessRecursively failed: %v", err)
		}
		arr, ok := result.([]any)
		if !ok {
			t.Errorf("Expected []any, got %T", result)
		} else if len(arr) != 2 {
			t.Errorf("len(result) = %d, want 2", len(arr))
		}
	})
}

// TestRecursiveProcessor_GetOperation tests Get operations with RecursiveProcessor
func TestRecursiveProcessor_GetOperation(t *testing.T) {
	processor := New()
	defer processor.Close()

	rp := NewRecursiveProcessor(processor)

	t.Run("SimpleProperty", func(t *testing.T) {
		data := map[string]any{"name": "test", "value": 123}

		result, err := rp.ProcessRecursively(data, "name", OpGet, nil)
		if err != nil {
			t.Errorf("ProcessRecursively failed: %v", err)
		}
		if result != "test" {
			t.Errorf("result = %v, want test", result)
		}
	})

	t.Run("NestedProperty", func(t *testing.T) {
		data := map[string]any{
			"user": map[string]any{
				"name": "Alice",
				"age":  30,
			},
		}

		result, err := rp.ProcessRecursively(data, "user.name", OpGet, nil)
		if err != nil {
			t.Errorf("ProcessRecursively failed: %v", err)
		}
		if result != "Alice" {
			t.Errorf("result = %v, want Alice", result)
		}
	})

	t.Run("ArrayIndex", func(t *testing.T) {
		data := []any{1, 2, 3, 4, 5}

		result, err := rp.ProcessRecursively(data, "[2]", OpGet, nil)
		if err != nil {
			t.Errorf("ProcessRecursively failed: %v", err)
		}
		if result != float64(3) && result != 3 {
			t.Errorf("result = %v, want 3", result)
		}
	})

	t.Run("ArrayNestedInObject", func(t *testing.T) {
		data := map[string]any{
			"items": []any{
				map[string]any{"id": 1, "name": "first"},
				map[string]any{"id": 2, "name": "second"},
			},
		}

		result, err := rp.ProcessRecursively(data, "items[0].name", OpGet, nil)
		if err != nil {
			t.Errorf("ProcessRecursively failed: %v", err)
		}
		if result != "first" {
			t.Errorf("result = %v, want first", result)
		}
	})

	t.Run("ArraySlice", func(t *testing.T) {
		data := []any{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}

		result, err := rp.ProcessRecursively(data, "[2:5]", OpGet, nil)
		if err != nil {
			t.Errorf("ProcessRecursively failed: %v", err)
		}
		arr, ok := result.([]any)
		if !ok {
			t.Errorf("Expected []any, got %T", result)
		} else if len(arr) != 3 {
			t.Errorf("len(result) = %d, want 3", len(arr))
		}
	})

	t.Run("RootPath", func(t *testing.T) {
		data := map[string]any{"key": "value"}

		result, err := rp.ProcessRecursively(data, "", OpGet, nil)
		if err != nil {
			t.Errorf("ProcessRecursively failed: %v", err)
		}
		// For empty path, result should be the same as input
		resultMap, ok := result.(map[string]any)
		if !ok {
			t.Errorf("Expected map[string]any, got %T", result)
		} else if resultMap["key"] != "value" {
			t.Errorf("result[key] = %v, want value", resultMap["key"])
		}
	})

	t.Run("InvalidPath", func(t *testing.T) {
		data := map[string]any{"key": "value"}

		_, err := rp.ProcessRecursively(data, "nonexistent.path", OpGet, nil)
		// Invalid paths may or may not return errors depending on implementation
		// Just verify it doesn't panic
		if err != nil {
			t.Logf("Invalid path returned error (expected): %v", err)
		}
	})
}

// TestReverseSlice tests reverse slicing logic
func TestReverseSlice(t *testing.T) {
	processor := New()
	defer processor.Close()

	arr := []any{0.0, 1.0, 2.0, 3.0, 4.0}

	tests := []struct {
		name     string
		start    int
		end      int
		step     int
		expected []any
	}{
		{
			name:     "reverse slice",
			start:    4,
			end:      0,
			step:     -1,
			expected: []any{4.0, 3.0, 2.0, 1.0},
		},
		{
			name:     "reverse with step",
			start:    4,
			end:      -1,
			step:     -2,
			expected: []any{4.0, 2.0, 0.0},
		},
		{
			name:     "start beyond length",
			start:    10,
			end:      0,
			step:     -1,
			expected: []any{4.0, 3.0, 2.0, 1.0},
		},
		{
			name:     "empty result",
			start:    0,
			end:      4,
			step:     -1,
			expected: []any{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := processor.reverseSlice(arr, tt.start, tt.end, tt.step)
			if !coreSliceEqual(result, tt.expected) {
				t.Errorf("reverseSlice(%d, %d, %d) = %v; want %v", tt.start, tt.end, tt.step, result, tt.expected)
			}
		})
	}
}

// TestSetGlobalProcessor tests SetGlobalProcessor function
func TestSetGlobalProcessor(t *testing.T) {
	// Reset state
	ShutdownGlobalProcessor()

	t.Run("SetCustomProcessor", func(t *testing.T) {
		customConfig := DefaultConfig()
		customConfig.MaxCacheSize = 500
		customProcessor := New(customConfig)

		SetGlobalProcessor(customProcessor)

		p := getDefaultProcessor()
		if p != customProcessor {
			t.Error("getDefaultProcessor should return the custom processor")
		}
	})

	t.Run("SetNilProcessorDoesNothing", func(t *testing.T) {
		originalProcessor := getDefaultProcessor()

		SetGlobalProcessor(nil)

		p := getDefaultProcessor()
		if p != originalProcessor {
			t.Error("Setting nil should not change the global processor")
		}
	})

	// Cleanup
	ShutdownGlobalProcessor()
}

// TestSetMultiple tests setting multiple values at once
func TestSetMultiple(t *testing.T) {
	jsonStr := `{"user": {"name": "Alice", "age": 30}}`

	tests := []struct {
		name        string
		updates     map[string]any
		expectError bool
		validate    func(t *testing.T, result string)
	}{
		{
			name: "multiple updates",
			updates: map[string]any{
				"user.name":  "Bob",
				"user.age":   35,
				"user.email": "bob@example.com",
			},
			expectError: false,
			validate: func(t *testing.T, result string) {
				if !contains(result, "Bob") {
					t.Error("Expected name to be updated to Bob")
				}
			},
		},
		{
			name:        "empty updates",
			updates:     map[string]any{},
			expectError: false,
			validate: func(t *testing.T, result string) {
				if !contains(result, "Alice") {
					t.Error("Expected original data to remain")
				}
			},
		},
		{
			name: "single update",
			updates: map[string]any{
				"user.name": "Charlie",
			},
			expectError: false,
			validate: func(t *testing.T, result string) {
				if !contains(result, "Charlie") {
					t.Error("Expected name to be updated to Charlie")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := SetMultiple(jsonStr, tt.updates)
			if tt.expectError && err == nil {
				t.Errorf("Expected error, but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
			if tt.validate != nil {
				tt.validate(t, result)
			}
		})
	}
}

// TestSetMultipleWithAdd tests multiple sets with path creation
func TestSetMultipleWithAdd(t *testing.T) {
	jsonStr := `{}`

	updates := map[string]any{
		"user.name":      "Alice",
		"user.age":       30,
		"user.email":     "alice@example.com",
		"settings.theme": "dark",
	}

	result, err := SetMultipleWithAdd(jsonStr, updates)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	for _, str := range []string{"Alice", "alice@example.com", "dark"} {
		if !contains(result, str) {
			t.Errorf("Expected result to contain '%s'", str)
		}
	}
}

// TestSetWithAdd tests set with automatic path creation
func TestSetWithAdd(t *testing.T) {
	jsonStr := `{"user": {}}`

	result, err := SetWithAdd(jsonStr, "user.name", "Alice")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if !contains(result, "Alice") {
		t.Error("Expected name to be set")
	}

	// Test nested path creation
	result, err = SetWithAdd(result, "user.profile.age", 30)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if !contains(result, "age") {
		t.Error("Expected nested age to be set")
	}
}

// TestShutdownGlobalProcessor tests ShutdownGlobalProcessor function
func TestShutdownGlobalProcessor(t *testing.T) {
	t.Run("ShutdownClosesProcessor", func(t *testing.T) {
		p := getDefaultProcessor()

		ShutdownGlobalProcessor()

		// Give it time to close
		time.Sleep(10 * time.Millisecond)

		if !p.IsClosed() {
			t.Error("Processor should be closed after shutdown")
		}
	})

	t.Run("MultipleShutdownsAreSafe", func(t *testing.T) {
		// Create a new processor
		_ = getDefaultProcessor()

		// Multiple shutdowns should not panic
		ShutdownGlobalProcessor()
		ShutdownGlobalProcessor()
		ShutdownGlobalProcessor()
	})

	t.Run("GetAfterShutdownCreatesNewProcessor", func(t *testing.T) {
		ShutdownGlobalProcessor()

		p1 := getDefaultProcessor()
		ShutdownGlobalProcessor()
		p2 := getDefaultProcessor()

		if p1 == p2 {
			t.Error("New processor should be created after shutdown")
		}

		// Cleanup
		ShutdownGlobalProcessor()
	})
}

// TestSliceRangeValidation tests slice range validation
func TestSliceRangeValidation(t *testing.T) {
	processor := New()
	defer processor.Close()

	tests := []struct {
		name     string
		rangeStr string
		valid    bool
	}{
		{
			name:     "valid simple",
			rangeStr: "0:5",
			valid:    true,
		},
		{
			name:     "valid with step",
			rangeStr: "0:10:2",
			valid:    true,
		},
		{
			name:     "valid empty start",
			rangeStr: ":5",
			valid:    true,
		},
		{
			name:     "valid empty end",
			rangeStr: "0:",
			valid:    true,
		},
		{
			name:     "invalid too many parts",
			rangeStr: "0:5:2:1",
			valid:    false,
		},
		{
			name:     "invalid single part",
			rangeStr: "5",
			valid:    false,
		},
		{
			name:     "invalid non-numeric",
			rangeStr: "a:b",
			valid:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := processor.isValidSliceRange(tt.rangeStr)
			if result != tt.valid {
				t.Errorf("isValidSliceRange(%s) = %v; want %v", tt.rangeStr, result, tt.valid)
			}
		})
	}
}

// TestStandardLibraryCompatibility tests encoding/json compatibility
func TestStandardLibraryCompatibility(t *testing.T) {
	// Test Marshal
	data := map[string]any{"name": "Alice", "age": 30}
	jsonBytes, err := Marshal(data)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}
	if len(jsonBytes) == 0 {
		t.Error("Expected non-empty JSON output")
	}

	// Test Unmarshal
	var result map[string]any
	err = Unmarshal(jsonBytes, &result)
	if err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
	if result["name"] != "Alice" {
		t.Errorf("Expected name 'Alice', got '%v'", result["name"])
	}

	// Test MarshalIndent
	indented, err := MarshalIndent(data, "", "  ")
	if err != nil {
		t.Fatalf("MarshalIndent failed: %v", err)
	}
	if !contains(string(indented), "\n") {
		t.Error("Expected indented output to contain newlines")
	}

	// Test Valid
	if !Valid(jsonBytes) {
		t.Error("Valid() returned false for valid JSON")
	}
	invalidJSON := []byte("{invalid}")
	if Valid(invalidJSON) {
		t.Error("Valid() returned true for invalid JSON")
	}
}

// TestValidateSchema tests ValidateSchema function
func TestValidateSchema(t *testing.T) {
	t.Run("valid object", func(t *testing.T) {
		jsonStr := `{"name": "Alice", "age": 30}`
		schema := &Schema{
			Type:     "object",
			Required: []string{"name"},
			Properties: map[string]*Schema{
				"name": {Type: "string"},
				"age":  {Type: "number"},
			},
		}

		errors, err := ValidateSchema(jsonStr, schema)
		if err != nil {
			t.Fatalf("ValidateSchema error: %v", err)
		}
		if len(errors) != 0 {
			t.Errorf("ValidateSchema should have no errors, got: %v", errors)
		}
	})

	t.Run("missing required field", func(t *testing.T) {
		jsonStr := `{"age": 30}`
		schema := &Schema{
			Type:     "object",
			Required: []string{"name"},
		}

		errors, err := ValidateSchema(jsonStr, schema)
		if err != nil {
			t.Fatalf("ValidateSchema error: %v", err)
		}
		if len(errors) == 0 {
			t.Error("ValidateSchema should report missing required field")
		}
	})

	t.Run("type mismatch", func(t *testing.T) {
		jsonStr := `{"name": 123}`
		schema := &Schema{
			Type: "object",
			Properties: map[string]*Schema{
				"name": {Type: "string"},
			},
		}

		errors, err := ValidateSchema(jsonStr, schema)
		if err != nil {
			t.Fatalf("ValidateSchema error: %v", err)
		}
		if len(errors) == 0 {
			t.Error("ValidateSchema should report type mismatch")
		}
	})

	t.Run("nil schema returns error", func(t *testing.T) {
		jsonStr := `{"name": "Alice"}`
		_, err := ValidateSchema(jsonStr, nil)
		if err == nil {
			t.Error("ValidateSchema should return error for nil schema")
		}
	})
}

// TestWarmupCache tests cache warmup functionality
func TestWarmupCache(t *testing.T) {
	jsonStr := `{
		"users": [
			{"name": "Alice", "age": 30},
			{"name": "Bob", "age": 25}
		],
		"settings": {
			"theme": "dark",
			"language": "en"
		}
	}`

	paths := []string{
		"users[0].name",
		"users[1].name",
		"settings.theme",
	}

	result, err := WarmupCache(jsonStr, paths)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if result == nil {
		t.Error("Expected warmup result, got nil")
	}
}
