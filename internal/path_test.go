package internal

import (
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
		{"invalid path with traversal", "../../../etc/passwd", true, 0},
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

func TestCompilePathUnsafe(t *testing.T) {
	t.Run("valid path", func(t *testing.T) {
		cp, err := CompilePathUnsafe("user.name")
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
			return
		}
		defer cp.Release()

		if cp.Len() != 2 {
			t.Errorf("Len() = %d, want 2", cp.Len())
		}
	})
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

	t.Run("Hash", func(t *testing.T) {
		hash := cp.Hash()
		if hash == 0 {
			t.Error("Hash() should not return 0")
		}
		// Same path should produce same hash
		cp2, _ := CompilePath("user.profile.name")
		defer cp2.Release()
		if cp.Hash() != cp2.Hash() {
			t.Error("Same paths should produce same hash")
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

func TestCompiledPath_GetFromRaw(t *testing.T) {
	raw := []byte(`{"user": {"name": "John"}}`)

	cp, err := CompilePath("user.name")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	defer cp.Release()

	result, err := cp.GetFromRaw(raw)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
		return
	}
	if result != "John" {
		t.Errorf("GetFromRaw() = %v, want 'John'", result)
	}
}

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

		// Should be the same cached path
		if cp1 != cp2 {
			t.Error("Should return cached path")
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
		{"empty", "", false},
		{"simple", "name", false},
		{"nested", "user.name", false},
		{"with array", "users[0]", false},
		{"deep nested", "a.b.c.d.e.f.g", false},
		{"too long", string(make([]byte, 1001)), true},
		{"null byte", "user\x00name", true},
		{"control char", "user\x01name", true},
		{"backslash", "user\\name", true},
		{"template injection", "${var}", true},
		{"double brace", "{{template}}", true},
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
