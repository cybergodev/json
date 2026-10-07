package internal

import (
	"errors"
	"testing"
)

func TestPathSegment(t *testing.T) {
	t.Run("PropertySegment", func(t *testing.T) {
		seg := NewPropertySegment("name")

		if seg.Type != PropertySegment {
			t.Error("Type should be PropertySegment")
		}
		if seg.Key != "name" {
			t.Errorf("Expected key 'name', got '%s'", seg.Key)
		}
		if seg.TypeString() != "property" {
			t.Errorf("Expected type string 'property', got '%s'", seg.TypeString())
		}
	})

	t.Run("ArrayIndexSegment", func(t *testing.T) {
		seg := NewArrayIndexSegment(5)

		if seg.Type != ArrayIndexSegment {
			t.Error("Type should be ArrayIndexSegment")
		}
		if seg.Index != 5 {
			t.Errorf("Expected index 5, got %d", seg.Index)
		}
		if seg.TypeString() != "array" {
			t.Errorf("Expected type string 'array', got '%s'", seg.TypeString())
		}
	})

	t.Run("ArraySliceSegment", func(t *testing.T) {
		start := 1
		end := 5
		step := 2

		seg := NewArraySliceSegment(start, end, step, true, true, true)

		if seg.Type != ArraySliceSegment {
			t.Error("Type should be ArraySliceSegment")
		}
		if !seg.HasStart() || seg.Index != 1 {
			t.Error("Start should be 1")
		}
		if !seg.HasEnd() || seg.End != 5 {
			t.Error("End should be 5")
		}
		if !seg.HasStep() || seg.Step != 2 {
			t.Error("Step should be 2")
		}
		if seg.TypeString() != "slice" {
			t.Errorf("Expected type string 'slice', got '%s'", seg.TypeString())
		}
	})

	t.Run("ExtractSegment", func(t *testing.T) {
		seg := NewExtractSegment("email")

		if seg.Type != ExtractSegment {
			t.Error("Type should be ExtractSegment")
		}
		if seg.Key != "email" {
			t.Errorf("Expected key 'email', got '%s'", seg.Key)
		}
		if seg.IsFlatExtract() {
			t.Error("Should not be flat extraction")
		}
	})

	t.Run("FlatExtractSegment", func(t *testing.T) {
		seg := NewExtractSegment("flat:email")

		if seg.Type != ExtractSegment {
			t.Error("Type should be ExtractSegment")
		}
		if seg.Key != "email" {
			t.Errorf("Expected key 'email', got '%s'", seg.Key)
		}
		if !seg.IsFlatExtract() {
			t.Error("Should be flat extraction")
		}
	})

}

func TestPathSegmentType(t *testing.T) {
	tests := []struct {
		segmentType PathSegmentType
		expected    string
	}{
		{PropertySegment, "property"},
		{ArrayIndexSegment, "array"},
		{ArraySliceSegment, "slice"},
		{WildcardSegment, "wildcard"},
		{RecursiveSegment, "recursive"},
		{FilterSegment, "filter"},
		{ExtractSegment, "extract"},
	}

	for _, tt := range tests {
		result := tt.segmentType.String()
		if result != tt.expected {
			t.Errorf("Expected '%s', got '%s'", tt.expected, result)
		}
	}
}

// ============================================================================
// COMPILED PATH TESTS
// ============================================================================

func TestCompilePath(t *testing.T) {
	tests := []struct {
		name        string
		path        string
		expectError bool
		segmentLen  int
	}{
		{"empty path", "", false, 0},
		{"simple property", "name", false, 1},
		{"nested path", "user.name", false, 2},
		{"array access", "users[0]", false, 2},
		{"json pointer", "/users/0/name", false, 3},
		// Note: Security validation (traversal, injection) is done by caller in security package
		// ValidatePath focuses on syntax only
		{"dot notation", "data.field", false, 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cp, err := CompilePath(tt.path)
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
			defer cp.Release()

			if cp.Len() != tt.segmentLen {
				t.Errorf("Len() = %d, want %d", cp.Len(), tt.segmentLen)
			}
			if cp.Path() != tt.path {
				t.Errorf("Path() = %q, want %q", cp.Path(), tt.path)
			}
		})
	}
}

func TestCompiledPath_Methods(t *testing.T) {
	cp, err := CompilePath("user.profile.name")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	defer cp.Release()

	t.Run("Segments", func(t *testing.T) {
		segs := cp.Segments()
		if len(segs) != 3 {
			t.Errorf("Segments() returned %d segments, want 3", len(segs))
		}
	})

	t.Run("String", func(t *testing.T) {
		if cp.String() != "user.profile.name" {
			t.Errorf("String() = %q, want %q", cp.String(), "user.profile.name")
		}
	})

	t.Run("IsEmpty", func(t *testing.T) {
		if cp.IsEmpty() {
			t.Error("Non-empty path should not be empty")
		}

		emptyCp, _ := CompilePath("")
		defer emptyCp.Release()
		if !emptyCp.IsEmpty() {
			t.Error("Empty path should be empty")
		}
	})
}

func TestCompiledPath_Get(t *testing.T) {
	data := map[string]any{
		"user": map[string]any{
			"name": "John",
			"age":  30,
			"tags": []any{"a", "b", "c"},
		},
	}

	tests := []struct {
		name        string
		path        string
		expected    any
		expectError bool
	}{
		{"simple get", "user.name", "John", false},
		{"nested get", "user.age", 30, false},
		{"array access", "user.tags[0]", "a", false},
		{"non-existent", "user.missing", nil, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cp, err := CompilePath(tt.path)
			if err != nil {
				t.Errorf("CompilePath error: %v", err)
				return
			}
			defer cp.Release()

			result, err := cp.Get(data)
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
			if result != tt.expected {
				t.Errorf("Get() = %v, want %v", result, tt.expected)
			}
		})
	}
}

// TestCompiledPath_Exists (restored header; GetFromRaw test removed above).
func TestCompiledPath_Exists(t *testing.T) {
	data := map[string]any{
		"user": map[string]any{
			"name": "John",
		},
	}

	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		{"existing", "user.name", true},
		{"non-existing", "user.missing", false},
		{"nested non-existing", "missing.path", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cp, err := CompilePath(tt.path)
			if err != nil {
				t.Errorf("CompilePath error: %v", err)
				return
			}
			defer cp.Release()

			result := cp.Exists(data)
			if result != tt.expected {
				t.Errorf("Exists() = %v, want %v", result, tt.expected)
			}
		})
	}
}

// ============================================================================
// COMPILED PATH CACHE TESTS
// ============================================================================

func TestCompiledPathCache(t *testing.T) {
	t.Run("Get and cache", func(t *testing.T) {
		cache := NewCompiledPathCache(100)

		// First get should compile and cache
		cp1, err := cache.Get("user.name")
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
			return
		}

		// Second get should return cached
		cp2, err := cache.Get("user.name")
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
			return
		}

		// Should return equivalent cached path (copies are independent but equal)
		if cp1.Path() != cp2.Path() || cp1.Len() != cp2.Len() {
			t.Error("Should return equivalent cached path")
		}
	})

	t.Run("Clear", func(t *testing.T) {
		cache := NewCompiledPathCache(100)
		cache.Get("user.name")

		cache.Clear()

		if cache.Size() != 0 {
			t.Errorf("Size() = %d, want 0 after clear", cache.Size())
		}
	})

	t.Run("Size", func(t *testing.T) {
		cache := NewCompiledPathCache(100)

		cache.Get("path1")
		cache.Get("path2")
		cache.Get("path3")

		if cache.Size() != 3 {
			t.Errorf("Size() = %d, want 3", cache.Size())
		}
	})

	t.Run("eviction", func(t *testing.T) {
		cache := NewCompiledPathCache(2)

		// Add more than max
		cache.Get("path1")
		cache.Get("path2")
		cache.Get("path3")

		// Size should be at most max (after eviction)
		if cache.Size() > 2 {
			t.Errorf("Size() = %d, should be <= 2 after eviction", cache.Size())
		}
	})
}

func TestGetGlobalCompiledPathCache(t *testing.T) {
	cache := GetGlobalCompiledPathCache()
	if cache == nil {
		t.Error("GetGlobalCompiledPathCache returned nil")
	}
}

// ============================================================================
// PATH ERROR TESTS
// ============================================================================

func TestCompiledPathError(t *testing.T) {
	t.Run("with path", func(t *testing.T) {
		err := NewPathError("user", "key not found", ErrPathNotFound)
		if err == nil {
			t.Fatal("NewPathError returned nil")
		}

		errStr := err.Error()
		if errStr == "" {
			t.Error("Error() should not be empty")
		}

		unwrapped := err.(*CompiledPathError).Unwrap()
		if unwrapped != ErrPathNotFound {
			t.Error("Unwrap should return underlying error")
		}
	})

	t.Run("without path", func(t *testing.T) {
		err := NewPathError("", "generic error", ErrTypeMismatch)
		errStr := err.Error()
		if errStr == "" {
			t.Error("Error() should not be empty")
		}
	})
}

// ============================================================================
// PATH VALIDATION TESTS
// ============================================================================

func TestValidatePath(t *testing.T) {
	tests := []struct {
		name        string
		path        string
		expectError bool
	}{
		// SYNTAX TESTS: ValidatePath focuses on syntax validation only
		// SECURITY TESTS: Security validation is tested in security package
		{"empty", "", false},
		{"simple", "name", false},
		{"nested", "user.name", false},
		{"with array", "users[0]", false},
		{"deep nested", "a.b.c.d.e.f.g", false},
		// Security tests moved to security package - ValidatePath only does syntax
		// {"too long", string(make([]byte, 1001)), true},      // Security: length check
		// {"null byte", "user\x00name", true},                  // Security: control char
		// {"control char", "user\x01name", true},               // Security: control char
		// {"backslash", "user\\name", true},                    // Security: traversal
		// {"template injection", "${var}", true},               // Security: injection
		// {"double brace", "{{template}}", true},               // Security: injection
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePath(tt.path)
			if tt.expectError {
				if err == nil {
					t.Error("Expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}

// ============================================================================
// PATH SEGMENT STRING TESTS
// ============================================================================

func TestPathSegment_String(t *testing.T) {
	tests := []struct {
		name     string
		segment  PathSegment
		expected string
	}{
		{"property", NewPropertySegment("name"), "name"},
		{"array index", NewArrayIndexSegment(5), "[5]"},
		{"negative index", NewArrayIndexSegment(-1), "[-1]"},
		{"array slice", NewArraySliceSegment(1, 5, 2, true, true, true), "[1:5:2]"},
		{"wildcard", PathSegment{Type: WildcardSegment, Flags: FlagIsWildcard}, "[*]"},
		{"extract", NewExtractSegment("email"), "{email}"},
		{"flat extract", NewExtractSegment("flat:email"), "{flat:email}"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.segment.String()
			if result != tt.expected {
				t.Errorf("String() = %q, want %q", result, tt.expected)
			}
		})
	}
}

// ============================================================================
// ARRAY ACCESS TESTS
// ============================================================================

func TestPathSegment_IsArrayAccess(t *testing.T) {
	tests := []struct {
		name     string
		segment  PathSegment
		expected bool
	}{
		{"property", NewPropertySegment("name"), false},
		{"array index", NewArrayIndexSegment(0), true},
		{"array slice", NewArraySliceSegment(0, 1, 1, true, true, false), true},
		{"wildcard", PathSegment{Type: WildcardSegment, Flags: FlagIsWildcard}, true},
		{"extract", NewExtractSegment("field"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.segment.IsArrayAccess()
			if result != tt.expected {
				t.Errorf("IsArrayAccess() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestPathSegment_GetArrayIndex(t *testing.T) {
	t.Run("positive index", func(t *testing.T) {
		seg := NewArrayIndexSegment(2)
		idx, err := seg.GetArrayIndex(5)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
			return
		}
		if idx != 2 {
			t.Errorf("GetArrayIndex() = %d, want 2", idx)
		}
	})

	t.Run("negative index", func(t *testing.T) {
		seg := NewArrayIndexSegment(-1)
		idx, err := seg.GetArrayIndex(5)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
			return
		}
		if idx != 4 {
			t.Errorf("GetArrayIndex() = %d, want 4", idx)
		}
	})

	t.Run("out of bounds", func(t *testing.T) {
		seg := NewArrayIndexSegment(10)
		_, err := seg.GetArrayIndex(5)
		if err == nil {
			t.Error("Expected error for out of bounds index")
		}
	})

	t.Run("wrong segment type", func(t *testing.T) {
		seg := NewPropertySegment("name")
		_, err := seg.GetArrayIndex(5)
		if err == nil {
			t.Error("Expected error for non-array segment")
		}
	})
}

func TestParseAndValidateArrayIndex(t *testing.T) {
	tests := []struct {
		input    string
		length   int
		expected int
		ok       bool
	}{
		{"0", 5, 0, true},
		{"2", 5, 2, true},
		{"-1", 5, 4, true},
		{"10", 5, 0, false},  // out of bounds
		{"-10", 5, 0, false}, // out of bounds
		{"abc", 5, 0, false}, // invalid
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			idx, ok := ParseAndValidateArrayIndex(tt.input, tt.length)
			if ok != tt.ok {
				t.Errorf("ok = %v, want %v", ok, tt.ok)
				return
			}
			if tt.ok && idx != tt.expected {
				t.Errorf("index = %d, want %d", idx, tt.expected)
			}
		})
	}
}

// ============================================================================
// Additional path coverage tests
// ============================================================================

// TestParseComplexSegment tests ParseComplexSegment function
func TestParseComplexSegment(t *testing.T) {
	t.Run("array index", func(t *testing.T) {
		segs, err := ParseComplexSegment("[0]")
		if err != nil {
			t.Fatalf("ParseComplexSegment error: %v", err)
		}
		if len(segs) == 0 || segs[0].Type != ArrayIndexSegment {
			t.Errorf("first segment type = %v, want ArrayIndexSegment", segs[0].Type)
		}
	})

	t.Run("array slice", func(t *testing.T) {
		segs, err := ParseComplexSegment("[1:3]")
		if err != nil {
			t.Fatalf("ParseComplexSegment error: %v", err)
		}
		if len(segs) == 0 || segs[0].Type != ArraySliceSegment {
			t.Errorf("first segment type = %v, want ArraySliceSegment", segs[0].Type)
		}
	})

	t.Run("wildcard", func(t *testing.T) {
		segs, err := ParseComplexSegment("[*]")
		if err != nil {
			t.Fatalf("ParseComplexSegment error: %v", err)
		}
		if len(segs) == 0 || segs[0].Type != WildcardSegment {
			t.Errorf("first segment type = %v, want WildcardSegment", segs[0].Type)
		}
	})

	t.Run("extract", func(t *testing.T) {
		segs, err := ParseComplexSegment("{name}")
		if err != nil {
			t.Fatalf("ParseComplexSegment error: %v", err)
		}
		if len(segs) == 0 || segs[0].Type != ExtractSegment {
			t.Errorf("first segment type = %v, want ExtractSegment", segs[0].Type)
		}
	})

	t.Run("slice with step", func(t *testing.T) {
		segs, err := ParseComplexSegment("[::2]")
		if err != nil {
			t.Fatalf("ParseComplexSegment error: %v", err)
		}
		if len(segs) == 0 || segs[0].Type != ArraySliceSegment {
			t.Errorf("first segment type = %v, want ArraySliceSegment", segs[0].Type)
		}
	})

	t.Run("negative slice", func(t *testing.T) {
		segs, err := ParseComplexSegment("[-2:]")
		if err != nil {
			t.Fatalf("ParseComplexSegment error: %v", err)
		}
		if len(segs) == 0 || segs[0].Type != ArraySliceSegment {
			t.Errorf("first segment type = %v, want ArraySliceSegment", segs[0].Type)
		}
	})
}

// TestNewExtractSegmentWithFlat tests NewExtractSegmentWithFlat
func TestNewExtractSegmentWithFlat(t *testing.T) {
	seg := NewExtractSegmentWithFlat("field", true)
	if seg.Type != ExtractSegment {
		t.Errorf("type = %v, want ExtractSegment", seg.Type)
	}
	if seg.Key != "field" {
		t.Errorf("key = %q, want 'field'", seg.Key)
	}
	if seg.Flags&FlagIsFlat == 0 {
		t.Error("expected FlagIsFlat to be set")
	}
}

// TestIsValidFieldName tests the isValidFieldName function
func TestIsValidFieldName(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"valid simple", "name", true},
		{"valid with underscore", "field_name", true},
		{"valid with dash", "field-name", true},
		{"valid with numbers", "field123", true},
		{"empty string", "", false},
		{"dash first char", "-name", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isValidFieldName(tt.input)
			if got != tt.want {
				t.Errorf("isValidFieldName(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

// TestHasEscapeSequence tests the hasEscapeSequence function
func TestHasEscapeSequence(t *testing.T) {
	tests := []struct {
		name string
		path string
		want bool
	}{
		{"simple", "user.name", false},
		{"escaped dot", "user\\.name", true},
		{"escaped backslash", "user\\\\name", true},
		{"non-escape char after backslash", "user\\name", false},
		{"mixed", "user\\.name.first", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hasEscape := HasEscapeSequence(tt.path)
			if hasEscape != tt.want {
				t.Errorf("HasEscapeSequence(%q) = %v, want %t", tt.path, hasEscape, tt.want)
			}
		})
	}
}

// TestUnescapePathSegment tests the UnescapePathSegment function
func TestUnescapePathSegment(t *testing.T) {
	tests := []struct {
		name string
		path string
		want string
	}{
		{"simple", "user.name", "user.name"},
		{"escaped dot", "user\\.name", "user.name"},
		{"escaped backslash", "user\\name", "user\\name"},
		{"mixed", "user\\.name.first", "user.name.first"},
		{"multiple escapes", "a\\.b\\.c", "a.b.c"},
		{"trailing backslash", "test\\", "test\\"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			unescaped := UnescapePathSegment(tt.path)
			if unescaped != tt.want {
				t.Errorf("UnescapePathSegment(%q) = %q, want %q", tt.path, unescaped, tt.want)
			}
		})
	}
}

// TestValidateNumericIndex exercises validateNumericIndex boundaries directly:
// sign handling, single/multi-digit fast paths, non-digits, overflow guards,
// and inclusive range bounds around maxIndex.
func TestValidateNumericIndex(t *testing.T) {
	const max = 5
	tests := []struct {
		name    string
		input   string
		maxIdx  int
		wantErr bool
	}{
		{"empty", "", max, true},
		{"bare minus", "-", max, true},
		{"single digit zero", "0", max, false},
		{"single digit negative zero", "-0", max, false},
		{"single digit max", "5", max, false},
		{"single digit non-digit", "a", max, true},
		{"multi digit in range", "12", 20, false},
		{"multi digit negative in range", "-12", 20, false},
		{"multi digit above range", "12", max, true},
		{"negative below range", "-12", max, true},
		{"negative at range bound", "-5", max, false},
		{"non-digit inside digits", "1a2", 20, true},
		{"non-digit after minus", "-a", 20, true},
		{"overflow guard", "99999999999999999999", 20, true},
		{"negative overflow guard", "-99999999999999999999", 20, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateNumericIndex(tt.input, tt.maxIdx)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateNumericIndex(%q, %d) error = %v, wantErr %v", tt.input, tt.maxIdx, err, tt.wantErr)
			}
		})
	}
}

// ===========================================================================
// Compiled-path boundary tests (consolidated from compiled_path_boundary_test.go)
// ===========================================================================
// ============================================================================
// Boundary tests for internal/compiled_path.go low-coverage paths.
// House style: plain assertions, t.Run subtests, section headers.
// ============================================================================

// --- applySlice (compiled_path.go:256, 0% coverage) ---

func TestApplySlice_Boundary(t *testing.T) {
	arr := []any{0, 1, 2, 3, 4}

	t.Run("basic_range", func(t *testing.T) {
		seg := &PathSegment{Index: 1, End: 3, Flags: FlagHasStart | FlagHasEnd}
		got, err := applySlice(arr, seg)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		if len(got) != 2 || got[0] != 1 || got[1] != 2 {
			t.Fatalf("got %v want [1 2]", got)
		}
	})

	t.Run("reverse_step_default_bounds", func(t *testing.T) {
		seg := &PathSegment{Step: -1, Flags: FlagHasStep}
		got, err := applySlice(arr, seg)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		if len(got) != 5 || got[0] != 4 || got[4] != 0 {
			t.Fatalf("got %v want [4 3 2 1 0]", got)
		}
	})

	t.Run("reverse_step_with_range", func(t *testing.T) {
		seg := &PathSegment{Index: 3, End: 0, Step: -1, Flags: FlagHasStart | FlagHasEnd | FlagHasStep}
		got, err := applySlice(arr, seg)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		if len(got) != 3 || got[0] != 3 || got[2] != 1 {
			t.Fatalf("got %v want [3 2 1]", got)
		}
	})

	t.Run("zero_step_error", func(t *testing.T) {
		seg := &PathSegment{Step: 0, Flags: FlagHasStep}
		if _, err := applySlice(arr, seg); err == nil {
			t.Fatal("expected error for zero slice step")
		}
	})

	t.Run("start_ge_end_empty", func(t *testing.T) {
		seg := &PathSegment{Index: 3, End: 1, Flags: FlagHasStart | FlagHasEnd}
		got, err := applySlice(arr, seg)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		if len(got) != 0 {
			t.Fatalf("got %v want empty", got)
		}
	})

	t.Run("negative_step_start_le_end_empty", func(t *testing.T) {
		seg := &PathSegment{Index: 1, End: 3, Step: -1, Flags: FlagHasStart | FlagHasEnd | FlagHasStep}
		got, err := applySlice(arr, seg)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		if len(got) != 0 {
			t.Fatalf("got %v want empty (start<=end with negative step)", got)
		}
	})

	t.Run("negative_indices", func(t *testing.T) {
		seg := &PathSegment{Index: -2, End: -1, Flags: FlagHasStart | FlagHasEnd}
		got, err := applySlice(arr, seg)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		if len(got) != 1 || got[0] != 3 {
			t.Fatalf("got %v want [3]", got)
		}
	})

	t.Run("clamped_end_beyond_length", func(t *testing.T) {
		seg := &PathSegment{Index: 0, End: 100, Flags: FlagHasStart | FlagHasEnd}
		got, err := applySlice(arr, seg)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		if len(got) != 5 {
			t.Fatalf("got %v want full array", got)
		}
	})

	t.Run("positive_step", func(t *testing.T) {
		seg := &PathSegment{Index: 0, End: 5, Step: 2, Flags: FlagHasStart | FlagHasEnd | FlagHasStep}
		got, err := applySlice(arr, seg)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		if len(got) != 3 || got[0] != 0 || got[1] != 2 || got[2] != 4 {
			t.Fatalf("got %v want [0 2 4]", got)
		}
	})
}

// --- CompiledPathError.Is (compiled_path.go:375, 0% coverage) ---

func TestCompiledPathError_Is(t *testing.T) {
	t.Run("matches_sentinel", func(t *testing.T) {
		e := &CompiledPathError{Path: "a", Message: "missing", Err: ErrPathNotFound}
		if !errors.Is(e, ErrPathNotFound) {
			t.Error("expected errors.Is(e, ErrPathNotFound) == true")
		}
	})
	t.Run("no_match", func(t *testing.T) {
		e := &CompiledPathError{Path: "a", Message: "missing", Err: ErrPathNotFound}
		if errors.Is(e, ErrTypeMismatch) {
			t.Error("expected errors.Is(e, ErrTypeMismatch) == false")
		}
	})
}

// --- CompiledPath.navigate error branches (compiled_path.go:181, 46% coverage) ---

func TestCompiledPath_Navigate_Boundary(t *testing.T) {
	mustCompile := func(path string) *CompiledPath {
		t.Helper()
		cp, err := CompilePath(path)
		if err != nil {
			t.Fatalf("CompilePath(%q) err: %v", path, err)
		}
		return cp
	}

	t.Run("nil_current", func(t *testing.T) {
		cp := mustCompile("a")
		if _, err := cp.Get(nil); err == nil {
			t.Error("expected error navigating into nil")
		}
	})
	t.Run("property_on_non_object", func(t *testing.T) {
		cp := mustCompile("a")
		if _, err := cp.Get("not an object"); err == nil {
			t.Error("expected type-mismatch error on property access of string")
		}
	})
	t.Run("missing_key", func(t *testing.T) {
		cp := mustCompile("a")
		if _, err := cp.Get(map[string]any{}); err == nil {
			t.Error("expected path-not-found error for missing key")
		}
	})
	t.Run("index_on_non_array", func(t *testing.T) {
		cp := mustCompile("[0]")
		if _, err := cp.Get(42); err == nil {
			t.Error("expected type-mismatch error for index on non-array")
		}
	})
	t.Run("index_out_of_bounds", func(t *testing.T) {
		cp := mustCompile("[5]")
		if _, err := cp.Get([]any{1, 2, 3}); err == nil {
			t.Error("expected out-of-bounds error")
		}
	})
	t.Run("negative_index", func(t *testing.T) {
		cp := mustCompile("-1")
		v, err := cp.Get([]any{1, 2, 3})
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		if v != 3 {
			t.Fatalf("got %v want 3", v)
		}
	})
	t.Run("slice_on_non_array", func(t *testing.T) {
		cp := mustCompile("[0:2]")
		if _, err := cp.Get("not an array"); err == nil {
			t.Error("expected type-mismatch error for slice on non-array")
		}
	})
	t.Run("slice_on_array", func(t *testing.T) {
		cp := mustCompile("[0:2]")
		v, err := cp.Get([]any{0, 1, 2, 3, 4})
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		got, ok := v.([]any)
		if !ok || len(got) != 2 || got[0] != 0 || got[1] != 1 {
			t.Fatalf("got %v", v)
		}
	})
	t.Run("wildcard_on_non_container", func(t *testing.T) {
		cp := mustCompile("*")
		if _, err := cp.Get(42); err == nil {
			t.Error("expected type-mismatch error for wildcard on non-container")
		}
	})
	t.Run("wildcard_on_map", func(t *testing.T) {
		cp := mustCompile("*")
		v, err := cp.Get(map[string]any{"a": 1, "b": 2})
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		got, ok := v.([]any)
		if !ok || len(got) != 2 {
			t.Fatalf("wildcard on map got %v", v)
		}
	})
	t.Run("wildcard_on_array", func(t *testing.T) {
		cp := mustCompile("*")
		v, err := cp.Get([]any{1, 2, 3})
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		got, ok := v.([]any)
		if !ok || len(got) != 3 {
			t.Fatalf("wildcard on array got %v", v)
		}
	})
}

// --- CompilePath error branch (compiled_path.go) ---

func TestCompilePath_InvalidPath(t *testing.T) {
	// Empty brackets -> ValidatePath rejects "empty array index".
	if _, err := CompilePath("a[]"); err == nil {
		t.Error("expected error for path with empty brackets")
	}
}
