package internal

import (
	"strings"
	"testing"
)

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
