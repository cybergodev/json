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
func TestIsNilOrEmpty(t *testing.T) {
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
			result := IsNilOrEmpty(tt.input)
			if result != tt.expected {
				t.Errorf("IsNilOrEmpty(%v) = %v, want %v", tt.input, result, tt.expected)
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

// TestParsePathBareNumericAndWildcard verifies that a single-segment numeric
// path ("0", "-1") parses to ArrayIndexSegment and a bare "*" parses to
// WildcardSegment — the core of the bare-index feature where "0" == "[0]"
// and "*" == "[*]".
func TestParsePathBareNumericAndWildcard(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		wantType PathSegmentType
		wantKey  string // for PropertySegment
		wantIdx  int    // for ArrayIndexSegment
		wantNeg  bool   // for ArrayIndexSegment negative flag
	}{
		{"bare zero", "0", ArrayIndexSegment, "", 0, false},
		{"bare positive", "42", ArrayIndexSegment, "", 42, false},
		{"bare negative", "-1", ArrayIndexSegment, "", -1, true},
		{"bare wildcard", "*", WildcardSegment, "", 0, false},
		{"property name", "name", PropertySegment, "name", 0, false},
		{"alphanumeric not int", "a1", PropertySegment, "a1", 0, false},
		{"underscore key", "_id", PropertySegment, "_id", 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			segments, err := ParsePath(tt.path)
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			if len(segments) != 1 {
				t.Fatalf("Got %d segments, want 1", len(segments))
			}
			s := segments[0]
			if s.Type != tt.wantType {
				t.Errorf("segment type = %v, want %v", s.Type, tt.wantType)
			}
			switch tt.wantType {
			case ArrayIndexSegment:
				if s.Index != tt.wantIdx {
					t.Errorf("index = %d, want %d", s.Index, tt.wantIdx)
				}
				if s.IsNegativeIndex() != tt.wantNeg {
					t.Errorf("negative flag = %v, want %v", s.IsNegativeIndex(), tt.wantNeg)
				}
			case WildcardSegment:
				if !s.IsWildcardSegment() {
					t.Errorf("expected wildcard flag set")
				}
			case PropertySegment:
				if s.Key != tt.wantKey {
					t.Errorf("key = %q, want %q", s.Key, tt.wantKey)
				}
			}
		})
	}
}

// ===========================================================================
// Navigation boundary tests (consolidated from navigation_boundary_test.go)
// ===========================================================================

// ============================================================================
// Boundary tests for internal/navigation.go low-coverage parse paths.
// ============================================================================

// --- IsExtractionSegment (navigation.go:135, 0% coverage) ---

func TestIsExtractionSegment(t *testing.T) {
	if !IsExtractionSegment(PathSegment{Type: ExtractSegment}) {
		t.Error("expected ExtractSegment to be an extraction segment")
	}
	if IsExtractionSegment(PathSegment{Type: PropertySegment}) {
		t.Error("expected PropertySegment to NOT be an extraction segment")
	}
	if IsExtractionSegment(PathSegment{Type: ArrayIndexSegment}) {
		t.Error("expected ArrayIndexSegment to NOT be an extraction segment")
	}
}

// --- ParseArraySegment (navigation.go:163, 0% coverage) ---

func TestParseArraySegment(t *testing.T) {
	t.Run("property_then_index", func(t *testing.T) {
		segs := ParseArraySegment("items[0]", nil)
		if len(segs) != 2 || segs[0].Type != PropertySegment || segs[0].Key != "items" ||
			segs[1].Type != ArrayIndexSegment || segs[1].Index != 0 {
			t.Fatalf("got %+v", segs)
		}
	})
	t.Run("bare_slice", func(t *testing.T) {
		segs := ParseArraySegment("[1:3]", nil)
		if len(segs) != 1 || segs[0].Type != ArraySliceSegment || !segs[0].HasStart() || !segs[0].HasEnd() {
			t.Fatalf("got %+v", segs)
		}
		if segs[0].Index != 1 || segs[0].End != 3 {
			t.Fatalf("slice bounds got %+v", segs[0])
		}
	})
	t.Run("append", func(t *testing.T) {
		segs := ParseArraySegment("[+]", nil)
		if len(segs) != 1 || segs[0].Type != AppendSegment {
			t.Fatalf("got %+v", segs)
		}
	})
	t.Run("slice_with_step", func(t *testing.T) {
		segs := ParseArraySegment("[0:5:2]", nil)
		if len(segs) != 1 || segs[0].Type != ArraySliceSegment || !segs[0].HasStep() || segs[0].Step != 2 {
			t.Fatalf("got %+v", segs)
		}
	})
	t.Run("no_close_bracket_treated_as_property", func(t *testing.T) {
		// Missing ']' -> falls back to a single property segment.
		segs := ParseArraySegment("items[0", nil)
		if len(segs) != 1 || segs[0].Type != PropertySegment || segs[0].Key != "items[0" {
			t.Fatalf("got %+v", segs)
		}
	})
	t.Run("chained_after_bracket", func(t *testing.T) {
		// "a[0].b" -> property a, index 0, then ".b" recurses via ParsePathSegment.
		segs := ParseArraySegment("a[0].b", nil)
		if len(segs) != 3 {
			t.Fatalf("got %d segments: %+v", len(segs), segs)
		}
	})
}

// --- ParseExtractionSegment (navigation.go:250, 0% coverage) ---

func TestParseExtractionSegment(t *testing.T) {
	t.Run("simple_extract", func(t *testing.T) {
		segs := ParseExtractionSegment("{key}", nil)
		if len(segs) != 1 || segs[0].Type != ExtractSegment || segs[0].Key != "key" {
			t.Fatalf("got %+v", segs)
		}
	})
	t.Run("flat_extract", func(t *testing.T) {
		segs := ParseExtractionSegment("{flat:tags}", nil)
		if len(segs) != 1 || segs[0].Type != ExtractSegment || segs[0].Key != "tags" {
			t.Fatalf("got %+v", segs)
		}
		if segs[0].Flags&FlagIsFlat == 0 {
			t.Error("expected FlagIsFlat set for {flat:tags}")
		}
	})
	t.Run("prefix_then_extract", func(t *testing.T) {
		segs := ParseExtractionSegment("items{id}", nil)
		if len(segs) != 2 || segs[0].Type != PropertySegment || segs[0].Key != "items" ||
			segs[1].Type != ExtractSegment || segs[1].Key != "id" {
			t.Fatalf("got %+v", segs)
		}
	})
	t.Run("no_close_brace_treated_as_property", func(t *testing.T) {
		segs := ParseExtractionSegment("{key", nil)
		if len(segs) != 1 || segs[0].Type != PropertySegment || segs[0].Key != "{key" {
			t.Fatalf("got %+v", segs)
		}
	})
}

// --- ParsePathSegment dispatch (navigation.go:140, 56% coverage) ---

func TestParsePathSegment_Dispatch(t *testing.T) {
	t.Run("array_dispatch", func(t *testing.T) {
		segs := ParsePathSegment("items[0]", nil)
		if len(segs) != 2 || segs[1].Type != ArrayIndexSegment {
			t.Fatalf("got %+v", segs)
		}
	})
	t.Run("extraction_dispatch", func(t *testing.T) {
		segs := ParsePathSegment("{key}", nil)
		if len(segs) != 1 || segs[0].Type != ExtractSegment {
			t.Fatalf("got %+v", segs)
		}
	})
	t.Run("numeric_index", func(t *testing.T) {
		segs := ParsePathSegment("42", nil)
		if len(segs) != 1 || segs[0].Type != ArrayIndexSegment || segs[0].Index != 42 {
			t.Fatalf("got %+v", segs)
		}
	})
	t.Run("property", func(t *testing.T) {
		segs := ParsePathSegment("name", nil)
		if len(segs) != 1 || segs[0].Type != PropertySegment || segs[0].Key != "name" {
			t.Fatalf("got %+v", segs)
		}
	})
}

// --- SplitPathIntoSegments escaped-dot slow path (navigation.go:299) ---

func TestSplitPathIntoSegments_EscapedDot(t *testing.T) {
	// `a\.b.c` -> two segments: "a.b" and "c" (escaped dot does not split).
	segs := SplitPathIntoSegments(`a\.b.c`, nil)
	if len(segs) != 2 {
		t.Fatalf("got %d segments: %+v", len(segs), segs)
	}
}

func TestSplitPathIntoSegments(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		wantLen int
	}{
		{"simple", "a.b.c", 3},
		{"single", "name", 1},
		{"empty", "", 0},
		{"trailing dot", "a.b.", 2},
		{"leading dot", ".a.b", 2},
		{"consecutive dots", "a..b", 2},
		{"escaped dot", `a\.b`, 1},
		{"escaped backslash", `a\\b`, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			segs := SplitPathIntoSegments(tt.path, nil)
			if len(segs) != tt.wantLen {
				t.Errorf("got %d segments, want %d: %+v", len(segs), tt.wantLen, segs)
			}
		})
	}
}

func TestIsValidArrayIndex(t *testing.T) {
	tests := []struct {
		idx  string
		want bool
	}{
		{"0", true},
		{"42", true},
		{"-1", true},
		{"", false},
		{"abc", false},
		{"1.5", false},
	}

	for _, tt := range tests {
		t.Run(tt.idx, func(t *testing.T) {
			if got := IsValidArrayIndex(tt.idx); got != tt.want {
				t.Errorf("IsValidArrayIndex(%q) = %v, want %v", tt.idx, got, tt.want)
			}
		})
	}
}

func TestIsArrayType(t *testing.T) {
	tests := []struct {
		val  any
		want bool
	}{
		{[]any{1, 2, 3}, true},
		{map[string]any{"a": 1}, false},
		{"hello", false},
		{nil, false},
		{42, false},
	}

	for i, tt := range tests {
		if got := IsArrayType(tt.val); got != tt.want {
			t.Errorf("IsArrayType[%d] = %v, want %v", i, got, tt.want)
		}
	}
}

func TestIsObjectType(t *testing.T) {
	tests := []struct {
		val  any
		want bool
	}{
		{map[string]any{"a": 1}, true},
		{map[any]any{"a": 1}, true},
		{[]any{1}, false},
		{nil, false},
		{"hello", false},
	}

	for i, tt := range tests {
		if got := IsObjectType(tt.val); got != tt.want {
			t.Errorf("IsObjectType[%d] = %v, want %v", i, got, tt.want)
		}
	}
}

func TestReconstructPath(t *testing.T) {
	tests := []struct {
		name string
		segs []PathSegment
		want string
	}{
		{"empty", nil, ""},
		{"single", []PathSegment{{Type: PropertySegment, Key: "a"}}, "a"},
		{"multiple", []PathSegment{
			{Type: PropertySegment, Key: "a"},
			{Type: PropertySegment, Key: "b"},
		}, "a.b"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ReconstructPath(tt.segs); got != tt.want {
				t.Errorf("ReconstructPath() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestIsValidCacheKey(t *testing.T) {
	tests := []struct {
		name string
		key  string
		want bool
	}{
		{"valid", "user.name", true},
		{"empty", "", false},
		{"too long", strings.Repeat("a", MaxCacheKeyLength+1), false},
		{"at limit", strings.Repeat("a", MaxCacheKeyLength), true},
		{"control char", "key\x00name", false},
		{"tab", "key\tname", false},
		{"unicode", "user.名前", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsValidCacheKey(tt.key); got != tt.want {
				t.Errorf("IsValidCacheKey() = %v, want %v", got, tt.want)
			}
		})
	}
}
