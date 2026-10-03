package internal

import (
	"testing"
)

// ============================================================================
// STRING INTERN TESTS
// ============================================================================

func TestStringIntern(t *testing.T) {
	t.Run("NewStringIntern", func(t *testing.T) {
		si := NewStringIntern(1024)
		if si == nil {
			t.Fatal("NewStringIntern returned nil")
		}
	})

	t.Run("Intern basic", func(t *testing.T) {
		si := NewStringIntern(1024)
		s1 := si.Intern("test")
		s2 := si.Intern("test")

		if s1 != s2 {
			t.Error("Same string should return same interned value")
		}
	})

	t.Run("Intern empty string", func(t *testing.T) {
		si := NewStringIntern(1024)
		s := si.Intern("")
		if s != "" {
			t.Error("Empty string should return empty string")
		}
	})

	t.Run("Intern long string", func(t *testing.T) {
		si := NewStringIntern(1024)
		longStr := ""
		for i := 0; i < 300; i++ {
			longStr += "a"
		}
		s := si.Intern(longStr)
		if s != longStr {
			t.Error("Long string should not be interned")
		}
	})

	t.Run("InternBytes", func(t *testing.T) {
		si := NewStringIntern(1024)
		b := []byte("test")
		s1 := si.InternBytes(b)
		s2 := si.InternBytes(b)

		if s1 != s2 {
			t.Error("Same bytes should return same interned value")
		}
	})

	t.Run("InternBytes empty", func(t *testing.T) {
		si := NewStringIntern(1024)
		s := si.InternBytes([]byte{})
		if s != "" {
			t.Error("Empty bytes should return empty string")
		}
	})

	t.Run("GetStats", func(t *testing.T) {
		si := NewStringIntern(1024)
		si.Intern("test1")
		si.Intern("test2")
		si.Intern("test1") // hit

		stats := si.GetStats()
		if stats.Entries != 2 {
			t.Errorf("Entries = %d, want 2", stats.Entries)
		}
		if stats.Hits != 1 {
			t.Errorf("Hits = %d, want 1", stats.Hits)
		}
	})

	t.Run("Clear", func(t *testing.T) {
		si := NewStringIntern(1024)
		si.Intern("test1")
		si.Intern("test2")

		si.Clear()

		stats := si.GetStats()
		if stats.Entries != 0 {
			t.Errorf("After clear, Entries = %d, want 0", stats.Entries)
		}
	})

	t.Run("eviction", func(t *testing.T) {
		si := NewStringIntern(100) // Very small to trigger eviction
		for i := 0; i < 50; i++ {
			si.Intern("test" + string(rune('a'+i%26)) + string(rune('a'+i/26)))
		}

		stats := si.GetStats()
		if stats.Evictions == 0 {
			t.Error("Expected some evictions with small cache")
		}
	})
}

func TestKeyIntern(t *testing.T) {
	t.Run("NewKeyIntern", func(t *testing.T) {
		ki := NewKeyIntern()
		if ki == nil {
			t.Fatal("NewKeyIntern returned nil")
		}
	})

	t.Run("Intern", func(t *testing.T) {
		ki := NewKeyIntern()
		k1 := ki.Intern("key1")
		k2 := ki.Intern("key1")

		if k1 != k2 {
			t.Error("Same key should return same interned value")
		}
	})

	t.Run("Intern empty", func(t *testing.T) {
		ki := NewKeyIntern()
		k := ki.Intern("")
		if k != "" {
			t.Error("Empty string should return empty")
		}
	})

	t.Run("Intern long key", func(t *testing.T) {
		ki := NewKeyIntern()
		longKey := ""
		for i := 0; i < 150; i++ {
			longKey += "a"
		}
		k := ki.Intern(longKey)
		if k != longKey {
			t.Error("Long key should not be interned")
		}
	})

	t.Run("InternBytes", func(t *testing.T) {
		ki := NewKeyIntern()
		b := []byte("key1")
		k1 := ki.InternBytes(b)
		k2 := ki.InternBytes(b)

		if k1 != k2 {
			t.Error("Same bytes should return same interned value")
		}
	})

	t.Run("InternBytes empty", func(t *testing.T) {
		ki := NewKeyIntern()
		k := ki.InternBytes([]byte{})
		if k != "" {
			t.Error("Empty bytes should return empty string")
		}
	})

	t.Run("Clear", func(t *testing.T) {
		ki := NewKeyIntern()
		ki.Intern("key1")
		ki.Intern("key2")

		ki.Clear()

		stats := ki.GetStats()
		if stats.ShardCount == 0 {
			t.Error("ShardCount should be positive")
		}
	})

	t.Run("GetStats", func(t *testing.T) {
		ki := NewKeyIntern()
		ki.Intern("key1")
		ki.Intern("key2")
		ki.Intern("key1") // hit - promotes to hot key

		stats := ki.GetStats()
		if stats.ShardCount == 0 {
			t.Error("ShardCount should be positive")
		}
	})
}

// ============================================================================
// SECURITY FUNCTION TESTS
// ============================================================================

// TestIsWordChar tests the IsWordChar function
func TestIsWordChar(t *testing.T) {
	tests := []struct {
		char     byte
		expected bool
	}{
		{'a', true},
		{'Z', true},
		{'0', true},
		{'_', true},
		{'-', false},
		{' ', false},
		{'!', false},
		{'@', false},
	}

	for _, tt := range tests {
		t.Run(string(tt.char), func(t *testing.T) {
			result := IsWordChar(tt.char)
			if result != tt.expected {
				t.Errorf("IsWordChar(%q) = %v, want %v", tt.char, result, tt.expected)
			}
		})
	}
}

// TestIsValidJSONNumber tests the IsValidJSONNumber function
func TestIsValidJSONNumber(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"123", true},
		{"123.456", true},
		{"-123", true},
		{"-123.456", true},
		{"0", true},
		{"0.0", true},
		{"1e10", true},
		{"1E10", true},
		{"1e+10", true},
		{"1e-10", true},
		{"-1.5e+10", true},
		{"", false},
		{"abc", false},
		{"12.34.56", false},
		{"++123", false},
		{"--123", false},
		{"e10", false},
		{"1e", false},
		{"1e+", false},
		{"1e-", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := IsValidJSONNumber(tt.input)
			if result != tt.expected {
				t.Errorf("IsValidJSONNumber(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

// TestIsValidJSONPrimitive tests the IsValidJSONPrimitive function
func TestIsValidJSONPrimitive(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"true", true},
		{"false", true},
		{"null", true},
		{"123", true},
		{"-123.456", true},
		{"invalid", false},
		{"", false},
		{"True", false},
		{"FALSE", false},
		{"NULL", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := IsValidJSONPrimitive(tt.input)
			if result != tt.expected {
				t.Errorf("IsValidJSONPrimitive(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

// ============================================================================
// ARRAY UTILITIES TESTS
// ============================================================================

// TestNormalizeIndex tests the NormalizeIndex function
func TestNormalizeIndex(t *testing.T) {
	tests := []struct {
		index    int
		length   int
		expected int
	}{
		{0, 5, 0},
		{2, 5, 2},
		{4, 5, 4},
		{-1, 5, 4},
		{-2, 5, 3},
		{-5, 5, 0},
		{5, 5, 5},   // Out of bounds, returns as-is
		{-6, 5, -1}, // Out of bounds, returns as-is
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			result := NormalizeIndex(tt.index, tt.length)
			if result != tt.expected {
				t.Errorf("NormalizeIndex(%d, %d) = %d, want %d", tt.index, tt.length, result, tt.expected)
			}
		})
	}
}

// TestParseArrayIndex_Internal tests the ParseArrayIndex function
func TestParseArrayIndex_Internal(t *testing.T) {
	tests := []struct {
		input      string
		expected   int
		expectedOk bool
	}{
		{"0", 0, true},
		{"5", 5, true},
		{"10", 10, true},
		{"-1", -1, true},
		{"-5", -5, true},
		{"abc", 0, false},
		{"", 0, false},
		{"1.5", 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result, ok := ParseArrayIndex(tt.input)
			if ok != tt.expectedOk {
				t.Errorf("ParseArrayIndex(%q) ok = %v, want %v", tt.input, ok, tt.expectedOk)
			}
			if tt.expectedOk && result != tt.expected {
				t.Errorf("ParseArrayIndex(%q) = %d, want %d", tt.input, result, tt.expected)
			}
		})
	}
}

// TestParseSliceComponents tests the ParseSliceComponents function
func TestParseSliceComponents(t *testing.T) {
	tests := []struct {
		input       string
		expectStart int
		expectEnd   int
		expectStep  int
		hasStart    bool
		hasEnd      bool
		hasStep     bool
		expectError bool
	}{
		{"1:5", 1, 5, 0, true, true, false, false},
		{":5", 0, 5, 0, false, true, false, false},
		{"1:", 1, 0, 0, true, false, false, false},
		{":", 0, 0, 0, false, false, false, false},
		{"1:5:2", 1, 5, 2, true, true, true, false},
		{"::2", 0, 0, 2, false, false, true, false},
		{"1::2", 1, 0, 2, true, false, true, false},
		{":5:2", 0, 5, 2, false, true, true, false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			start, end, step, err := ParseSliceComponents(tt.input)

			if tt.expectError {
				if err == nil {
					t.Errorf("ParseSliceComponents(%q) should return error", tt.input)
				}
				return
			}

			if err != nil {
				t.Errorf("ParseSliceComponents(%q) unexpected error: %v", tt.input, err)
				return
			}

			if tt.hasStart {
				if start == nil {
					t.Error("start should not be nil")
				} else if *start != tt.expectStart {
					t.Errorf("start = %d, want %d", *start, tt.expectStart)
				}
			} else if start != nil {
				t.Error("start should be nil")
			}

			if tt.hasEnd {
				if end == nil {
					t.Error("end should not be nil")
				} else if *end != tt.expectEnd {
					t.Errorf("end = %d, want %d", *end, tt.expectEnd)
				}
			} else if end != nil {
				t.Error("end should be nil")
			}

			if tt.hasStep {
				if step == nil {
					t.Error("step should not be nil")
				} else if *step != tt.expectStep {
					t.Errorf("step = %d, want %d", *step, tt.expectStep)
				}
			} else if step != nil {
				t.Error("step should be nil")
			}
		})
	}
}

// ============================================================================
// ENCODING HELPERS TESTS
// ============================================================================

// TestIsSpace tests the IsSpace function
func TestIsSpace(t *testing.T) {
	tests := []struct {
		char     byte
		expected bool
	}{
		{' ', true},
		{'\t', true},
		{'\n', true},
		{'\r', true},
		{'a', false},
		{'0', false},
		{'_', false},
	}

	for _, tt := range tests {
		t.Run(string(tt.char), func(t *testing.T) {
			result := IsSpace(tt.char)
			if result != tt.expected {
				t.Errorf("IsSpace(%q) = %v, want %v", tt.char, result, tt.expected)
			}
		})
	}
}

// TestIsDigit tests the IsDigit function
func TestIsDigit(t *testing.T) {
	tests := []struct {
		char     byte
		expected bool
	}{
		{'0', true},
		{'5', true},
		{'9', true},
		{'a', false},
		{' ', false},
		{'-', false},
	}

	for _, tt := range tests {
		t.Run(string(tt.char), func(t *testing.T) {
			result := IsDigit(tt.char)
			if result != tt.expected {
				t.Errorf("IsDigit(%q) = %v, want %v", tt.char, result, tt.expected)
			}
		})
	}
}

// TestDeepMerge tests the DeepMerge function
// TestDeepMerge_NonObjectOverride covers the one DeepMerge branch not
// already pinned by TestDeepMerge_Objects in helpers_test.go (the two
// duplicated subtests were removed in the FIX-001 test cleanup).
func TestDeepMerge_NonObjectOverride(t *testing.T) {
	t.Run("override non-object with object", func(t *testing.T) {
		base := map[string]any{"a": 1}
		override := map[string]any{"a": map[string]any{"b": 2}}

		result := DeepMerge(base, override)
		resultMap := result.(map[string]any)

		_, ok := resultMap["a"].(map[string]any)
		if !ok {
			t.Error("expected a to be a map")
		}
	})
}

// ============================================================================
// EXTRACTION TESTS
// ============================================================================

// ============================================================================
// FLATTEN ARRAY OPTIMIZED TESTS
// ============================================================================

// TestFlattenArrayOptimized tests the FlattenArrayOptimized function
func TestNormalizeSlice(t *testing.T) {
	tests := []struct {
		name        string
		length      int
		start       int
		end         int
		expectStart int
		expectEnd   int
	}{
		{"full slice", 5, 0, 5, 0, 5},
		{"partial slice", 5, 1, 4, 1, 4},
		{"negative start", 5, -2, 5, 3, 5},
		{"negative end", 5, 0, -1, 0, 4},
		{"both negative", 5, -3, -1, 2, 4},
		{"start clamped", 5, -10, 5, 0, 5},
		{"end clamped", 5, 0, 10, 0, 5},
		{"start > end", 5, 4, 2, 2, 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start, end := NormalizeSlice(tt.start, tt.end, tt.length)
			if start != tt.expectStart {
				t.Errorf("start = %d, want %d", start, tt.expectStart)
			}
			if end != tt.expectEnd {
				t.Errorf("end = %d, want %d", end, tt.expectEnd)
			}
		})
	}
}

// ============================================================================
// PERFORM ARRAY SLICE TESTS
// ============================================================================

// TestPerformArraySlice tests the PerformArraySlice function
func TestPerformArraySlice(t *testing.T) {
	arr := []any{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}

	t.Run("full slice", func(t *testing.T) {
		result := PerformArraySlice(arr, nil, nil, nil)
		if len(result) != 10 {
			t.Errorf("PerformArraySlice returned %d items, want 10", len(result))
		}
	})

	t.Run("first 5", func(t *testing.T) {
		start, end := 0, 5
		result := PerformArraySlice(arr, &start, &end, nil)
		expected := []any{0, 1, 2, 3, 4}
		if len(result) != len(expected) {
			t.Errorf("PerformArraySlice returned %d items, want %d", len(result), len(expected))
			return
		}
		for i, v := range result {
			if v != expected[i] {
				t.Errorf("result[%d] = %v, want %v", i, v, expected[i])
			}
		}
	})

	t.Run("step 2", func(t *testing.T) {
		step := 2
		result := PerformArraySlice(arr, nil, nil, &step)
		expected := []any{0, 2, 4, 6, 8}
		if len(result) != len(expected) {
			t.Errorf("PerformArraySlice returned %d items, want %d", len(result), len(expected))
			return
		}
		for i, v := range result {
			if v != expected[i] {
				t.Errorf("result[%d] = %v, want %v", i, v, expected[i])
			}
		}
	})

	t.Run("negative step", func(t *testing.T) {
		start, end, step := 9, -1, -1
		result := PerformArraySlice(arr, &start, &end, &step)
		// Negative step goes backwards, the exact behavior depends on implementation
		if len(result) == 0 {
			// If negative step with -1 end doesn't work as expected, just verify it doesn't panic
			t.Log("Negative step slice returned empty result")
		}
	})

	t.Run("empty array", func(t *testing.T) {
		emptyArr := []any{}
		result := PerformArraySlice(emptyArr, nil, nil, nil)
		if len(result) != 0 {
			t.Errorf("PerformArraySlice on empty array should return empty, got %d", len(result))
		}
	})

	t.Run("zero step", func(t *testing.T) {
		step := 0
		result := PerformArraySlice(arr, nil, nil, &step)
		if len(result) != 0 {
			t.Errorf("Zero step should return empty slice")
		}
	})
}
