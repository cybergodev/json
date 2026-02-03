package json

import (
	"testing"

	"github.com/cybergodev/json/internal"
)

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

// TestPerformArraySlice tests array slice operations
func TestPerformArraySlice(t *testing.T) {
	processor := New()
	defer processor.Close()

	arr := []any{0.0, 1.0, 2.0, 3.0, 4.0, 5.0, 6.0, 7.0, 8.0, 9.0}

	tests := []struct {
		name          string
		start         *int
		end           *int
		step          *int
		expected      []any
		description   string
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
			name: "array with numeric property",
			data: []any{"a", "b", "c"},
			property:    "1",
			expectedVal: "b",
			shouldExist: true,
		},
		{
			name: "array with invalid property",
			data: []any{"a", "b", "c"},
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
				Type:   internal.ExtractSegment,
				Key:    "items",
				IsFlat: true,
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
			name:         "slice",
			part:         "[0:5]",
			expectedType: "slice",
			expectedStart: intPtr(0),
			expectedEnd:   intPtr(5),
			expectedStep:  intPtr(1),
		},
		{
			name:         "slice with step",
			part:         "[0:10:2]",
			expectedType: "slice",
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
		name         string
		part         string
		expectedKey  string
		expectedIsFlat bool
	}{
		{
			name:         "simple extraction",
			part:         "{name}",
			expectedKey:  "name",
			expectedIsFlat: false,
		},
		{
			name:         "flat extraction",
			part:         "{flat:items}",
			expectedKey:  "items",
			expectedIsFlat: true,
		},
		{
			name:         "property with extraction",
			part:         "users{name}",
			expectedKey:  "name",
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

			if extractSeg.IsFlat != tt.expectedIsFlat {
				t.Errorf("IsFlat = %v; want %v", extractSeg.IsFlat, tt.expectedIsFlat)
			}
		})
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

// Benchmark tests

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

// Helper functions

func intPtr(i int) *int {
	return &i
}

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

func getProp(data any, key string) any {
	if m, ok := data.(map[string]any); ok {
		return m[key]
	}
	return nil
}
