package json

import (
	"testing"
)

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

// TestArrayHelper_NormalizeIndex tests the NormalizeIndex method
func TestArrayHelper_NormalizeIndex(t *testing.T) {
	ah := &ArrayHelper{}

	tests := []struct {
		name       string
		index      int
		length     int
		expected   int
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
			// Verify no nil or DeletedMarker in result
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
		name             string
		input            []any
		targetLength     int
		expectExtend     bool
		expectedLength   int
	}{
		{"Already longer", []any{1, 2, 3}, 2, false, 3},  // Returns original array when already longer
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
			// Copy array to avoid modifying original
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
		{"Reverse step", 9, -1, -1, []any{}},  // Normalized end becomes 9, same as start, so empty
		{"Reverse step partial", 8, 2, -2, []any{8, 6, 4}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ah.PerformSlice(arr, tt.start, tt.end, tt.step)
			if !sliceEqual(result, tt.expected) {
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

// Helper function to compare slices
func sliceEqual(a, b []any) bool {
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
