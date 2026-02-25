package internal

import (
	"strings"
	"testing"
)

// ============================================================================
// NAVIGATION FUNCTION TESTS
// ============================================================================

// TestNeedsPathPreprocessing tests the NeedsPathPreprocessing function
func TestNeedsPathPreprocessing(t *testing.T) {
	tests := []struct {
		path     string
		expected bool
	}{
		{"name", false},
		{"user.name", false},
		{"user[0]", true},
		{"users[0].name", true},
		{"data{key}", true},
		{"", false},
		{"a.b.c", false},
		{"a[0].b[1]", true},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := NeedsPathPreprocessing(tt.path)
			if result != tt.expected {
				t.Errorf("NeedsPathPreprocessing(%q) = %v, want %v", tt.path, result, tt.expected)
			}
		})
	}
}

// TestNeedsDotBefore tests the NeedsDotBefore function
func TestNeedsDotBefore(t *testing.T) {
	tests := []struct {
		char     rune
		expected bool
	}{
		{'a', true},
		{'Z', true},
		{'5', true},
		{'_', true},
		{']', true},
		{'}', true},
		{'.', false},
		{'[', false},
		{' ', false},
		{'-', false},
	}

	for _, tt := range tests {
		t.Run(string(tt.char), func(t *testing.T) {
			result := NeedsDotBefore(tt.char)
			if result != tt.expected {
				t.Errorf("NeedsDotBefore(%q) = %v, want %v", tt.char, result, tt.expected)
			}
		})
	}
}

// TestNeedsDotBeforeByte tests the NeedsDotBeforeByte function
func TestNeedsDotBeforeByte(t *testing.T) {
	tests := []struct {
		char     byte
		expected bool
	}{
		{'a', true},
		{'Z', true},
		{'5', true},
		{'_', true},
		{']', true},
		{'}', true},
		{'.', false},
		{'[', false},
		{' ', false},
		{'-', false},
	}

	for _, tt := range tests {
		t.Run(string(tt.char), func(t *testing.T) {
			result := NeedsDotBeforeByte(tt.char)
			if result != tt.expected {
				t.Errorf("NeedsDotBeforeByte(%q) = %v, want %v", tt.char, result, tt.expected)
			}
		})
	}
}

// TestIsComplexPath tests the IsComplexPath function
func TestIsComplexPath(t *testing.T) {
	tests := []struct {
		path     string
		expected bool
	}{
		{"name", false},
		{"user.name", false},
		{"user[0]", true},
		{"data{key}", true},
		{"slice[1:5]", true},
		{"filter{.price > 10}", true},
		{"a:b", true},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := IsComplexPath(tt.path)
			if result != tt.expected {
				t.Errorf("IsComplexPath(%q) = %v, want %v", tt.path, result, tt.expected)
			}
		})
	}
}

// ============================================================================
// ARRAY OPERATIONS TESTS
// ============================================================================

// TestGetPooledSlice tests slice pooling
func TestGetPooledSlice(t *testing.T) {
	slice := GetPooledSlice()
	if slice == nil {
		t.Fatal("GetPooledSlice returned nil")
	}
	if len(*slice) != 0 {
		t.Errorf("Pooled slice length = %d, want 0", len(*slice))
	}

	// Add some items
	*slice = append(*slice, 1, 2, 3)

	// Return to pool
	PutPooledSlice(slice)

	// Get again - should be reset
	slice2 := GetPooledSlice()
	if len(*slice2) != 0 {
		t.Errorf("Pooled slice should be reset, length = %d", len(*slice2))
	}

	PutPooledSlice(slice2)
	PutPooledSlice(nil) // Should not panic
}

// TestCompactArrayOptimized tests array compaction
func TestCompactArrayOptimized(t *testing.T) {
	tests := []struct {
		name     string
		input    []any
		expected int
	}{
		{"empty array", []any{}, 0},
		{"all nil", []any{nil, nil, nil}, 0},
		{"mixed", []any{1, nil, "test", "", []any{}, map[string]any{}}, 2},
		{"all valid", []any{1, "test", true}, 3},
		{"with empty string", []any{"", "value"}, 1},
		{"with empty slice", []any{[]any{}, []any{1}}, 1},
		{"with empty map", []any{map[string]any{}, map[string]any{"key": "val"}}, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CompactArrayOptimized(tt.input)
			if len(result) != tt.expected {
				t.Errorf("CompactArrayOptimized returned %d items, want %d", len(result), tt.expected)
			}
		})
	}
}

// TestFilterArrayOptimized tests array filtering
func TestFilterArrayOptimized(t *testing.T) {
	input := []any{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}

	result := FilterArrayOptimized(input, func(item any) bool {
		if num, ok := item.(int); ok {
			return num%2 == 0
		}
		return false
	})

	if len(result) != 5 {
		t.Errorf("FilterArrayOptimized returned %d items, want 5", len(result))
	}

	// Verify all results are even
	for _, item := range result {
		if num, ok := item.(int); ok {
			if num%2 != 0 {
				t.Errorf("FilterArrayOptimized returned odd number: %v", item)
			}
		}
	}

	// Test empty input
	emptyResult := FilterArrayOptimized([]any{}, func(item any) bool { return true })
	if len(emptyResult) != 0 {
		t.Errorf("FilterArrayOptimized with empty input returned %d items, want 0", len(emptyResult))
	}
}

// TestMapArrayOptimized tests array mapping
func TestMapArrayOptimized(t *testing.T) {
	input := []any{1, 2, 3}

	result := MapArrayOptimized(input, func(item any) any {
		if num, ok := item.(int); ok {
			return num * 2
		}
		return item
	})

	if len(result) != 3 {
		t.Fatalf("MapArrayOptimized returned %d items, want 3", len(result))
	}

	expected := []any{2, 4, 6}
	for i, item := range result {
		if item != expected[i] {
			t.Errorf("MapArrayOptimized[%d] = %v, want %v", i, item, expected[i])
		}
	}

	// Test empty input
	emptyResult := MapArrayOptimized([]any{}, func(item any) any { return item })
	if len(emptyResult) != 0 {
		t.Errorf("MapArrayOptimized with empty input returned %d items, want 0", len(emptyResult))
	}
}

// TestisNilOrEmptyFast tests the isNilOrEmptyFast helper
func TestIsNilOrEmptyFast(t *testing.T) {
	tests := []struct {
		name     string
		input    any
		expected bool
	}{
		{"nil", nil, true},
		{"empty string", "", true},
		{"non-empty string", "test", false},
		{"empty slice", []any{}, true},
		{"non-empty slice", []any{1}, false},
		{"empty map", map[string]any{}, true},
		{"non-empty map", map[string]any{"key": "val"}, false},
		{"int", 42, false},
		{"bool", true, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isNilOrEmptyFast(tt.input)
			if result != tt.expected {
				t.Errorf("isNilOrEmptyFast(%v) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

// ============================================================================
// PATH SEGMENT TESTS
// ============================================================================

// TestPathSegment_HasStart tests the HasStart method
func TestPathSegment_HasStart(t *testing.T) {
	// Array slice segment with start
	seg := NewArraySliceSegment(1, 5, 1, true, true, false)
	if !seg.HasStart() {
		t.Error("ArraySliceSegment should have start")
	}

	// Array slice segment without start
	seg2 := NewArraySliceSegment(0, 5, 1, false, true, false)
	if seg2.HasStart() {
		t.Error("ArraySliceSegment should not have start")
	}

	// Property segment (not applicable)
	propSeg := NewPropertySegment("test")
	if propSeg.HasStart() {
		t.Error("PropertySegment should not have start")
	}
}

// TestPathSegment_HasEnd tests the HasEnd method
func TestPathSegment_HasEnd(t *testing.T) {
	// Array slice segment with end
	seg := NewArraySliceSegment(1, 5, 1, true, true, false)
	if !seg.HasEnd() {
		t.Error("ArraySliceSegment should have end")
	}

	// Array slice segment without end
	seg2 := NewArraySliceSegment(1, 0, 1, true, false, false)
	if seg2.HasEnd() {
		t.Error("ArraySliceSegment should not have end")
	}
}

// TestPathSegment_HasStep tests the HasStep method
func TestPathSegment_HasStep(t *testing.T) {
	// Array slice segment with step
	seg := NewArraySliceSegment(1, 5, 2, true, true, true)
	if !seg.HasStep() {
		t.Error("ArraySliceSegment should have step")
	}

	// Array slice segment without step
	seg2 := NewArraySliceSegment(1, 5, 0, true, true, false)
	if seg2.HasStep() {
		t.Error("ArraySliceSegment should not have step")
	}
}

// TestPathSegment_TypeString tests the TypeString method
func TestPathSegment_TypeString(t *testing.T) {
	tests := []struct {
		name     string
		seg      PathSegment
		expected string
	}{
		{"property", NewPropertySegment("test"), "property"},
		{"array index", NewArrayIndexSegment(0), "array"},
		{"array slice", NewArraySliceSegment(0, 1, 1, true, true, true), "slice"},
		{"extract", NewExtractSegment("key"), "extract"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.seg.TypeString(); got != tt.expected {
				t.Errorf("TypeString() = %q, want %q", got, tt.expected)
			}
		})
	}
}

// TestPathSegment_IsFlatExtract tests the IsFlatExtract method
func TestPathSegment_IsFlatExtract(t *testing.T) {
	// Regular extract
	seg := NewExtractSegment("email")
	if seg.IsFlatExtract() {
		t.Error("Regular extract should not be flat")
	}

	// Flat extract
	flatSeg := NewExtractSegment("flat:email")
	if !flatSeg.IsFlatExtract() {
		t.Error("Flat extract should be flat")
	}

	// Property segment
	propSeg := NewPropertySegment("test")
	if propSeg.IsFlatExtract() {
		t.Error("PropertySegment should not be flat extract")
	}
}

// TestNewPropertySegment tests creating property segments
func TestNewPropertySegment(t *testing.T) {
	seg := NewPropertySegment("userName")

	if seg.Type != PropertySegment {
		t.Errorf("Type = %v, want PropertySegment", seg.Type)
	}
	if seg.Key != "userName" {
		t.Errorf("Key = %q, want %q", seg.Key, "userName")
	}
}

// TestNewArrayIndexSegment tests creating array index segments
func TestNewArrayIndexSegment(t *testing.T) {
	seg := NewArrayIndexSegment(42)

	if seg.Type != ArrayIndexSegment {
		t.Errorf("Type = %v, want ArrayIndexSegment", seg.Type)
	}
	if seg.Index != 42 {
		t.Errorf("Index = %d, want 42", seg.Index)
	}

	// Test negative index
	negSeg := NewArrayIndexSegment(-1)
	if !negSeg.IsNegativeIndex() {
		t.Error("Negative index should be marked as negative")
	}
}

// TestNewArraySliceSegment tests creating array slice segments
func TestNewArraySliceSegment(t *testing.T) {
	seg := NewArraySliceSegment(1, 10, 2, true, true, true)

	if seg.Type != ArraySliceSegment {
		t.Errorf("Type = %v, want ArraySliceSegment", seg.Type)
	}
	if seg.Index != 1 {
		t.Errorf("Start (Index) = %d, want 1", seg.Index)
	}
	if seg.End != 10 {
		t.Errorf("End = %d, want 10", seg.End)
	}
	if seg.Step != 2 {
		t.Errorf("Step = %d, want 2", seg.Step)
	}
}

// TestNewExtractSegment tests creating extract segments
func TestNewExtractSegment(t *testing.T) {
	// Regular extract
	seg := NewExtractSegment("email")

	if seg.Type != ExtractSegment {
		t.Errorf("Type = %v, want ExtractSegment", seg.Type)
	}
	if seg.Key != "email" {
		t.Errorf("Key = %q, want %q", seg.Key, "email")
	}
}

// TestPathSegmentType_String tests the String method for all segment types
func TestPathSegmentType_String(t *testing.T) {
	tests := []struct {
		segType  PathSegmentType
		expected string
	}{
		{PropertySegment, "property"},
		{ArrayIndexSegment, "array"},
		{ArraySliceSegment, "slice"},
		{WildcardSegment, "wildcard"},
		{RecursiveSegment, "recursive"},
		{FilterSegment, "filter"},
		{ExtractSegment, "extract"},
		{PathSegmentType(99), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if got := tt.segType.String(); got != tt.expected {
				t.Errorf("String() = %q, want %q", got, tt.expected)
			}
		})
	}
}

// TestPathSegment_GetMethods tests the GetStart, GetEnd, GetStep methods
func TestPathSegment_GetMethods(t *testing.T) {
	seg := NewArraySliceSegment(1, 10, 2, true, true, true)

	start, hasStart := seg.GetStart()
	if !hasStart || start != 1 {
		t.Errorf("GetStart() = (%d, %v), want (1, true)", start, hasStart)
	}

	end, hasEnd := seg.GetEnd()
	if !hasEnd || end != 10 {
		t.Errorf("GetEnd() = (%d, %v), want (10, true)", end, hasEnd)
	}

	step, hasStep := seg.GetStep()
	if !hasStep || step != 2 {
		t.Errorf("GetStep() = (%d, %v), want (2, true)", step, hasStep)
	}

	// Test without values
	seg2 := NewArraySliceSegment(0, 0, 0, false, false, false)

	_, hasStart2 := seg2.GetStart()
	if hasStart2 {
		t.Error("GetStart should return false when not set")
	}

	_, hasEnd2 := seg2.GetEnd()
	if hasEnd2 {
		t.Error("GetEnd should return false when not set")
	}

	_, hasStep2 := seg2.GetStep()
	if hasStep2 {
		t.Error("GetStep should return false when not set")
	}
}

// ============================================================================
// PREPROCESS PATH TESTS
// ============================================================================

// TestPreprocessPath tests the PreprocessPath function
func TestPreprocessPath(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"simple path", "user.name", "user.name"},
		{"bracket after property", "user[0]", "user.[0]"},
		{"multiple brackets", "users[0].name", "users.[0].name"},
		{"extraction", "data{key}", "data.{key}"},
		{"complex path", "users[0].posts{title}", "users.[0].posts.{title}"},
		{"already has dot", "user.[0]", "user.[0]"},
		{"empty string", "", ""},
	}

	var sb strings.Builder
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := PreprocessPath(tt.input, &sb)
			if result != tt.expected {
				t.Errorf("PreprocessPath(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

// TestPreprocessPathNonASCII tests PreprocessPath with non-ASCII characters
func TestPreprocessPathNonASCII(t *testing.T) {
	var sb strings.Builder

	// Test with non-ASCII characters - the function handles non-ASCII in the slow path
	input := "用户[0]"
	result := PreprocessPath(input, &sb)
	// The function adds a dot before [ when the previous character is alphanumeric (including non-ASCII letters)
	// Since '户' is a non-ASCII character, the behavior depends on the implementation
	t.Logf("PreprocessPath(%q) = %q", input, result)
	// The actual behavior: non-ASCII chars are not considered alphanumeric by the simple check
	// So the dot is NOT added for non-ASCII
	expected := "用户[0]" // No dot added because '户' is not ASCII alphanumeric
	if result != expected {
		t.Errorf("PreprocessPath(%q) = %q, want %q", input, result, expected)
	}
}

// ============================================================================
// ESCAPE/UNESCAPE JSON POINTER TESTS
// ============================================================================

// TestEscapeJSONPointer tests EscapeJSONPointer function
func TestEscapeJSONPointer(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"simple", "simple"},
		{"with~tilde", "with~0tilde"},
		{"with/slash", "with~1slash"},
		{"~test/path~", "~0test~1path~0"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := EscapeJSONPointer(tt.input)
			if result != tt.expected {
				t.Errorf("EscapeJSONPointer(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

// TestUnescapeJSONPointer tests UnescapeJSONPointer function
func TestUnescapeJSONPointer(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"simple", "simple"},
		{"with~0tilde", "with~tilde"},
		{"with~1slash", "with/slash"},
		{"~0test~1path~0", "~test/path~"},
		{"", ""},
		{"~2", "~2"}, // Invalid escape, should remain unchanged
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := UnescapeJSONPointer(tt.input)
			if result != tt.expected {
				t.Errorf("UnescapeJSONPointer(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

// ============================================================================
// PARSE PATH TESTS
// ============================================================================

// TestParsePath tests the ParsePath function
func TestParsePath(t *testing.T) {
	tests := []struct {
		name        string
		path        string
		expectCount int
		expectError bool
	}{
		{"empty path", "", 0, false},
		{"simple property", "name", 1, false},
		{"nested path", "user.name", 2, false},
		{"array index", "users[0]", 2, false},
		{"json pointer", "/users/0/name", 3, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			segments, err := ParsePath(tt.path)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if len(segments) != tt.expectCount {
				t.Errorf("Got %d segments, want %d", len(segments), tt.expectCount)
			}
		})
	}
}
