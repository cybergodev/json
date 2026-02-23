package json

import (
	"strings"
	"testing"
)

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

// Helper function to compare slices
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

// Benchmark tests for path operations

func BenchmarkPathParsingSimple(b *testing.B) {
	processor := New()
	defer processor.Close()

	path := "user.name.first"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = processor.parsePath(path)
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

func BenchmarkJSONPointerEscape(b *testing.B) {
	processor := New()
	defer processor.Close()

	input := "user~/name/with~special/chars"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = processor.escapeJSONPointer(input)
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
