package internal

import (
	"errors"
	"testing"
)

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
