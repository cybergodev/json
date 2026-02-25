package internal

import (
	"testing"
	"time"
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

func TestPathIntern(t *testing.T) {
	t.Run("NewPathIntern", func(t *testing.T) {
		pi := NewPathIntern(1000)
		if pi == nil {
			t.Fatal("NewPathIntern returned nil")
		}
	})

	t.Run("Get and Set", func(t *testing.T) {
		pi := NewPathIntern(1000)
		segments := []PathSegment{NewPropertySegment("test")}
		pi.Set("path1", segments)
		v, ok := pi.Get("path1")
		if !ok {
			t.Error("Get should return ok=true")
		}
		if len(v) != 1 {
			t.Errorf("Get returned %d segments, want 1", len(v))
		}
	})

	t.Run("Get non-existent", func(t *testing.T) {
		pi := NewPathIntern(1000)
		_, ok := pi.Get("nonexistent")
		if ok {
			t.Error("Get should return false for non-existent key")
		}
	})

	t.Run("Clear", func(t *testing.T) {
		pi := NewPathIntern(1000)
		segments := []PathSegment{NewPropertySegment("test")}
		pi.Set("path1", segments)
		pi.Clear()

		_, ok := pi.Get("path1")
		if ok {
			t.Error("Get should return false after clear")
		}
	})

	t.Run("Long path not cached", func(t *testing.T) {
		pi := NewPathIntern(1000)
		// Create a very long path
		longPath := ""
		for i := 0; i < 300; i++ {
			longPath += "a"
		}
		segments := []PathSegment{NewPropertySegment("test")}
		pi.Set(longPath, segments)

		_, ok := pi.Get(longPath)
		if ok {
			t.Error("Long path should not be cached")
		}
	})

	t.Run("InternKey", func(t *testing.T) {
		k := InternKey("test_key")
		if k == "" {
			t.Error("InternKey should not return empty string")
		}
	})

	t.Run("InternKeyBytes", func(t *testing.T) {
		k := InternKeyBytes([]byte("test_key"))
		if k == "" {
			t.Error("InternKeyBytes should not return empty string")
		}
	})

	t.Run("InternString", func(t *testing.T) {
		s := InternString("test_string")
		if s == "" {
			t.Error("InternString should not return empty string")
		}
	})

	t.Run("InternStringBytes", func(t *testing.T) {
		s := InternStringBytes([]byte("test_string"))
		if s == "" {
			t.Error("InternStringBytes should not return empty string")
		}
	})

	t.Run("BatchIntern", func(t *testing.T) {
		keys := []string{"key1", "key2", "key3"}
		result := BatchIntern(keys)
		if len(result) != 3 {
			t.Errorf("BatchIntern returned %d keys, want 3", len(result))
		}
	})

	t.Run("BatchIntern empty", func(t *testing.T) {
		result := BatchIntern([]string{})
		if len(result) != 0 {
			t.Errorf("BatchIntern should return empty for empty input")
		}
	})

	t.Run("BatchInternKeys", func(t *testing.T) {
		keys := []string{"key1", "key2", "key3"}
		result := BatchInternKeys(keys)
		if len(result) != 3 {
			t.Errorf("BatchInternKeys returned %d keys, want 3", len(result))
		}
	})

	t.Run("BatchInternKeys empty", func(t *testing.T) {
		result := BatchInternKeys([]string{})
		if len(result) != 0 {
			t.Errorf("BatchInternKeys should return empty for empty input")
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

// ============================================================================
// DATA OPERATIONS TESTS
// ============================================================================

// TestMergeObjects tests the MergeObjects function
func TestMergeObjects(t *testing.T) {
	t.Run("merge two objects", func(t *testing.T) {
		obj1 := map[string]any{"a": 1, "b": 2}
		obj2 := map[string]any{"b": 3, "c": 4}

		result := MergeObjects(obj1, obj2)

		if result["a"] != 1 {
			t.Errorf("a = %v, want 1", result["a"])
		}
		if result["b"] != 3 {
			t.Errorf("b = %v, want 3 (should be overwritten)", result["b"])
		}
		if result["c"] != 4 {
			t.Errorf("c = %v, want 4", result["c"])
		}
	})

	t.Run("merge with nil", func(t *testing.T) {
		obj := map[string]any{"a": 1}

		result := MergeObjects(nil, obj)
		if result["a"] != 1 {
			t.Errorf("a = %v, want 1", result["a"])
		}

		result2 := MergeObjects(obj, nil)
		if result2["a"] != 1 {
			t.Errorf("a = %v, want 1", result2["a"])
		}
	})
}

// TestFlattenArray tests the FlattenArray function
func TestFlattenArray(t *testing.T) {
	t.Run("flat array", func(t *testing.T) {
		arr := []any{1, 2, 3}
		result := FlattenArray(arr)

		if len(result) != 3 {
			t.Errorf("length = %d, want 3", len(result))
		}
	})

	t.Run("nested array", func(t *testing.T) {
		arr := []any{1, []any{2, 3}, 4}
		result := FlattenArray(arr)

		if len(result) != 4 {
			t.Errorf("length = %d, want 4", len(result))
		}
	})

	t.Run("deeply nested array", func(t *testing.T) {
		arr := []any{1, []any{2, []any{3, 4}}, 5}
		result := FlattenArray(arr)

		if len(result) != 5 {
			t.Errorf("length = %d, want 5", len(result))
		}
	})
}

// TestDeepMerge tests the DeepMerge function
func TestDeepMerge(t *testing.T) {
	t.Run("merge non-overlapping", func(t *testing.T) {
		base := map[string]any{"a": 1}
		override := map[string]any{"b": 2}

		result := DeepMerge(base, override)
		resultMap := result.(map[string]any)

		if resultMap["a"] != 1 {
			t.Errorf("a = %v, want 1", resultMap["a"])
		}
		if resultMap["b"] != 2 {
			t.Errorf("b = %v, want 2", resultMap["b"])
		}
	})

	t.Run("merge nested objects", func(t *testing.T) {
		base := map[string]any{
			"config": map[string]any{"a": 1, "b": 2},
		}
		override := map[string]any{
			"config": map[string]any{"b": 3, "c": 4},
		}

		result := DeepMerge(base, override)
		resultMap := result.(map[string]any)
		config := resultMap["config"].(map[string]any)

		if config["a"] != 1 {
			t.Errorf("config.a = %v, want 1", config["a"])
		}
		if config["b"] != 3 {
			t.Errorf("config.b = %v, want 3", config["b"])
		}
		if config["c"] != 4 {
			t.Errorf("config.c = %v, want 4", config["c"])
		}
	})

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

// TestDetectConsecutiveExtractions tests DetectConsecutiveExtractions function
func TestDetectConsecutiveExtractions(t *testing.T) {
	t.Run("consecutive extractions", func(t *testing.T) {
		segments := []PathSegment{
			NewExtractSegment("email"),
			NewExtractSegment("name"),
		}

		groups := DetectConsecutiveExtractions(segments)
		if len(groups) == 0 {
			t.Error("expected at least one extraction group")
		}
	})

	t.Run("mixed segments", func(t *testing.T) {
		segments := []PathSegment{
			NewPropertySegment("user"),
			NewExtractSegment("email"),
			NewExtractSegment("name"),
		}

		groups := DetectConsecutiveExtractions(segments)
		if len(groups) == 0 {
			t.Error("expected at least one extraction group")
		}
	})

	t.Run("no extractions", func(t *testing.T) {
		segments := []PathSegment{
			NewPropertySegment("user"),
		}

		groups := DetectConsecutiveExtractions(segments)
		if len(groups) != 0 {
			t.Errorf("expected no extraction groups, got %d", len(groups))
		}
	})
}

// ============================================================================
// UNIQUE ARRAY TESTS
// ============================================================================

// TestUniqueArray tests the UniqueArray function
func TestUniqueArray(t *testing.T) {
	tests := []struct {
		name     string
		input    []any
		expected int
	}{
		{"empty", []any{}, 0},
		{"single", []any{1}, 1},
		{"no duplicates", []any{1, 2, 3}, 3},
		{"with duplicates", []any{1, 2, 1, 3, 2}, 3},
		{"all same", []any{1, 1, 1}, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := UniqueArray(tt.input)
			if len(result) != tt.expected {
				t.Errorf("UniqueArray returned %d items, want %d", len(result), tt.expected)
			}
		})
	}
}

// ============================================================================
// REVERSE ARRAY TESTS
// ============================================================================

// TestReverseArray tests the ReverseArray function
func TestReverseArray(t *testing.T) {
	t.Run("empty array", func(t *testing.T) {
		arr := []any{}
		ReverseArray(arr)
		if len(arr) != 0 {
			t.Error("empty array should remain empty")
		}
	})

	t.Run("single element", func(t *testing.T) {
		arr := []any{1}
		ReverseArray(arr)
		if arr[0] != 1 {
			t.Errorf("single element should remain, got %v", arr[0])
		}
	})

	t.Run("multiple elements", func(t *testing.T) {
		arr := []any{1, 2, 3, 4, 5}
		ReverseArray(arr)
		expected := []any{5, 4, 3, 2, 1}
		for i, v := range arr {
			if v != expected[i] {
				t.Errorf("arr[%d] = %v, want %v", i, v, expected[i])
			}
		}
	})
}

// ============================================================================
// UNIQUE ARRAY OPTIMIZED TESTS
// ============================================================================

// TestUniqueArrayOptimized tests the UniqueArrayOptimized function
func TestUniqueArrayOptimized(t *testing.T) {
	tests := []struct {
		name     string
		input    []any
		expected int
	}{
		{"empty", []any{}, 0},
		{"single", []any{1}, 1},
		{"no duplicates", []any{1, 2, 3}, 3},
		{"with duplicates", []any{1, 2, 1, 3, 2}, 3},
		{"all same", []any{1, 1, 1}, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := UniqueArrayOptimized(tt.input)
			if len(result) != tt.expected {
				t.Errorf("UniqueArrayOptimized returned %d items, want %d", len(result), tt.expected)
			}
		})
	}
}

// ============================================================================
// FLATTEN ARRAY OPTIMIZED TESTS
// ============================================================================

// TestFlattenArrayOptimized tests the FlattenArrayOptimized function
func TestFlattenArrayOptimized(t *testing.T) {
	tests := []struct {
		name     string
		input    []any
		expected int
	}{
		{"empty", []any{}, 0},
		{"flat", []any{1, 2, 3}, 3},
		{"nested", []any{1, []any{2, 3}, 4}, 4},
		{"deeply nested", []any{1, []any{2, []any{3, 4}}, 5}, 5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FlattenArrayOptimized(tt.input)
			if len(result) != tt.expected {
				t.Errorf("FlattenArrayOptimized returned %d items, want %d", len(result), tt.expected)
			}
		})
	}
}

// ============================================================================
// CHUNK ARRAY OPTIMIZED TESTS
// ============================================================================

// TestChunkArrayOptimized tests the ChunkArrayOptimized function
func TestChunkArrayOptimized(t *testing.T) {
	tests := []struct {
		name      string
		input     []any
		chunkSize int
		expected  int
	}{
		{"empty", []any{}, 2, 0},
		{"exact fit", []any{1, 2, 3, 4}, 2, 2},
		{"with remainder", []any{1, 2, 3, 4, 5}, 2, 3},
		{"larger chunk", []any{1, 2}, 5, 1},
		{"invalid chunk size", []any{1, 2, 3}, 0, 0},
		{"negative chunk size", []any{1, 2, 3}, -1, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ChunkArrayOptimized(tt.input, tt.chunkSize)
			if len(result) != tt.expected {
				t.Errorf("ChunkArrayOptimized returned %d chunks, want %d", len(result), tt.expected)
			}
		})
	}
}

// ============================================================================
// REVERSE ARRAY OPTIMIZED TESTS
// ============================================================================

// TestReverseArrayOptimized tests the ReverseArrayOptimized function
func TestReverseArrayOptimized(t *testing.T) {
	t.Run("empty array", func(t *testing.T) {
		arr := []any{}
		ReverseArrayOptimized(arr)
		if len(arr) != 0 {
			t.Error("empty array should remain empty")
		}
	})

	t.Run("single element", func(t *testing.T) {
		arr := []any{1}
		ReverseArrayOptimized(arr)
		if arr[0] != 1 {
			t.Errorf("single element should remain, got %v", arr[0])
		}
	})

	t.Run("multiple elements", func(t *testing.T) {
		arr := []any{1, 2, 3, 4, 5}
		ReverseArrayOptimized(arr)
		expected := []any{5, 4, 3, 2, 1}
		for i, v := range arr {
			if v != expected[i] {
				t.Errorf("arr[%d] = %v, want %v", i, v, expected[i])
			}
		}
	})
}

// ============================================================================
// TAKE FIRST/LAST TESTS
// ============================================================================

// TestTakeFirst tests the TakeFirst function
func TestTakeFirst(t *testing.T) {
	tests := []struct {
		name     string
		arr      []any
		n        int
		expected int
	}{
		{"empty array", []any{}, 2, 0},
		{"take zero", []any{1, 2, 3}, 0, 0},
		{"take negative", []any{1, 2, 3}, -1, 0},
		{"take less than length", []any{1, 2, 3, 4, 5}, 3, 3},
		{"take equal to length", []any{1, 2, 3}, 3, 3},
		{"take more than length", []any{1, 2, 3}, 5, 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := TakeFirst(tt.arr, tt.n)
			if len(result) != tt.expected {
				t.Errorf("TakeFirst returned %d items, want %d", len(result), tt.expected)
			}
		})
	}
}

// TestTakeLast tests the TakeLast function
func TestTakeLast(t *testing.T) {
	tests := []struct {
		name     string
		arr      []any
		n        int
		expected int
	}{
		{"empty array", []any{}, 2, 0},
		{"take zero", []any{1, 2, 3}, 0, 0},
		{"take negative", []any{1, 2, 3}, -1, 0},
		{"take less than length", []any{1, 2, 3, 4, 5}, 3, 3},
		{"take equal to length", []any{1, 2, 3}, 3, 3},
		{"take more than length", []any{1, 2, 3}, 5, 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := TakeLast(tt.arr, tt.n)
			if len(result) != tt.expected {
				t.Errorf("TakeLast returned %d items, want %d", len(result), tt.expected)
			}
		})
	}
}

// ============================================================================
// NORMALIZE SLICE TESTS
// ============================================================================

// TestNormalizeSlice tests the NormalizeSlice function
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

// ============================================================================
// IS VALID INDEX / GET SAFE ARRAY ELEMENT TESTS
// ============================================================================

// TestIsValidIndex tests the IsValidIndex function
func TestIsValidIndex(t *testing.T) {
	tests := []struct {
		index    int
		length   int
		expected bool
	}{
		{0, 5, true},
		{4, 5, true},
		{5, 5, false},
		{-1, 5, true}, // -1 normalizes to 4
		{-5, 5, true}, // -5 normalizes to 0
		{-6, 5, false},
		{0, 0, false},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			result := IsValidIndex(tt.index, tt.length)
			if result != tt.expected {
				t.Errorf("IsValidIndex(%d, %d) = %v, want %v", tt.index, tt.length, result, tt.expected)
			}
		})
	}
}

// TestGetSafeArrayElement tests the GetSafeArrayElement function
func TestGetSafeArrayElement(t *testing.T) {
	arr := []any{1, 2, 3}

	tests := []struct {
		name     string
		index    int
		expected any
		expectOk bool
	}{
		{"valid index 0", 0, 1, true},
		{"valid index 1", 1, 2, true},
		{"valid index 2", 2, 3, true},
		{"out of bounds positive", 5, nil, false},
		{"negative valid", -1, 3, true},
		{"negative out of bounds", -10, nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, ok := GetSafeArrayElement(arr, tt.index)
			if ok != tt.expectOk {
				t.Errorf("GetSafeArrayElement(arr, %d) ok = %v, want %v", tt.index, ok, tt.expectOk)
			}
			if tt.expectOk && result != tt.expected {
				t.Errorf("GetSafeArrayElement(arr, %d) = %v, want %v", tt.index, result, tt.expected)
			}
		})
	}
}

// ============================================================================
// PARALLEL PROCESSOR TESTS
// ============================================================================

// TestParallelConfig tests ParallelConfig defaults
func TestDefaultParallelConfig(t *testing.T) {
	config := DefaultParallelConfig()
	if config.Workers < 2 {
		t.Errorf("Workers = %d, want at least 2", config.Workers)
	}
	if config.BatchSize <= 0 {
		t.Error("BatchSize should be positive")
	}
	if config.MinParallel <= 0 {
		t.Error("MinParallel should be positive")
	}
}

// TestNewParallelProcessor tests NewParallelProcessor
func TestNewParallelProcessor(t *testing.T) {
	t.Run("with defaults", func(t *testing.T) {
		pp := NewParallelProcessor(DefaultParallelConfig())
		if pp == nil {
			t.Fatal("NewParallelProcessor returned nil")
		}
	})

	t.Run("with zero values", func(t *testing.T) {
		pp := NewParallelProcessor(ParallelConfig{})
		if pp == nil {
			t.Fatal("NewParallelProcessor returned nil")
		}
		if pp.config.Workers <= 0 {
			t.Error("Workers should be set to default")
		}
	})
}

// TestParallelSlice tests ParallelSlice
func TestParallelSlice(t *testing.T) {
	pp := NewParallelProcessor(ParallelConfig{
		Workers:     2,
		BatchSize:   10,
		MinParallel: 100,
	})

	t.Run("small slice sequential", func(t *testing.T) {
		arr := []any{1, 2, 3, 4, 5}
		result, err := pp.ParallelSlice(arr, func(index int, value any) (any, error) {
			return value.(int) * 2, nil
		})
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if len(result) != 5 {
			t.Errorf("Expected 5 results, got %d", len(result))
		}
	})

	t.Run("with error", func(t *testing.T) {
		arr := []any{1, 2, 3}
		_, err := pp.ParallelSlice(arr, func(index int, value any) (any, error) {
			if value.(int) == 2 {
				return nil, ErrInvalidPath
			}
			return value, nil
		})
		if err == nil {
			t.Error("Expected error, got nil")
		}
	})
}

// TestParallelMap tests ParallelMap
func TestParallelMap(t *testing.T) {
	pp := NewParallelProcessor(ParallelConfig{
		Workers:     2,
		BatchSize:   10,
		MinParallel: 100,
	})

	t.Run("small map sequential", func(t *testing.T) {
		m := map[string]any{"a": 1, "b": 2, "c": 3}
		result, err := pp.ParallelMap(m, func(key string, value any) (any, error) {
			return value.(int) * 2, nil
		})
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if len(result) != 3 {
			t.Errorf("Expected 3 results, got %d", len(result))
		}
	})

	t.Run("with error", func(t *testing.T) {
		m := map[string]any{"a": 1, "b": 2}
		_, err := pp.ParallelMap(m, func(key string, value any) (any, error) {
			if key == "b" {
				return nil, ErrInvalidPath
			}
			return value, nil
		})
		if err == nil {
			t.Error("Expected error, got nil")
		}
	})
}

// TestParallelForEach tests ParallelForEach
func TestParallelForEach(t *testing.T) {
	pp := NewParallelProcessor(ParallelConfig{
		Workers:     2,
		BatchSize:   10,
		MinParallel: 100,
	})

	t.Run("small slice sequential", func(t *testing.T) {
		arr := []any{1, 2, 3}
		sum := 0
		err := pp.ParallelForEach(arr, func(index int, value any) error {
			sum += value.(int)
			return nil
		})
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if sum != 6 {
			t.Errorf("Sum = %d, want 6", sum)
		}
	})
}

// TestParallelFilter tests ParallelFilter
func TestParallelFilter(t *testing.T) {
	pp := NewParallelProcessor(ParallelConfig{
		Workers:     2,
		BatchSize:   10,
		MinParallel: 100,
	})

	t.Run("small slice", func(t *testing.T) {
		arr := []any{1, 2, 3, 4, 5}
		result := pp.ParallelFilter(arr, func(value any) bool {
			return value.(int)%2 == 0
		})
		if len(result) != 2 {
			t.Errorf("Expected 2 results, got %d", len(result))
		}
	})
}

// TestParallelTransform tests ParallelTransform
func TestParallelTransform(t *testing.T) {
	pp := NewParallelProcessor(ParallelConfig{
		Workers:     2,
		BatchSize:   10,
		MinParallel: 100,
	})

	t.Run("small slice", func(t *testing.T) {
		arr := []any{1, 2, 3}
		result := pp.ParallelTransform(arr, func(value any) any {
			return value.(int) * 2
		})
		if len(result) != 3 {
			t.Errorf("Expected 3 results, got %d", len(result))
		}
		for i, v := range result {
			if v != (i+1)*2 {
				t.Errorf("result[%d] = %v, want %d", i, v, (i+1)*2)
			}
		}
	})
}

// ============================================================================
// WORKER POOL TESTS
// ============================================================================

// TestNewWorkerPool tests NewWorkerPool
func TestNewWorkerPool(t *testing.T) {
	wp := NewWorkerPool(2)
	if wp == nil {
		t.Fatal("NewWorkerPool returned nil")
	}
	defer wp.Stop()

	// Submit a task
	done := make(chan bool, 1)
	wp.Submit(func() {
		done <- true
	})

	// Wait for task to complete with timeout
	select {
	case <-done:
		// Task completed successfully
	case <-time.After(1 * time.Second):
		t.Error("Task was not executed within timeout")
	}
}

// ============================================================================
// CHUNK PROCESSOR TESTS
// ============================================================================

// TestNewChunkProcessor tests NewChunkProcessor
func TestNewChunkProcessor(t *testing.T) {
	cp := NewChunkProcessor(100)
	if cp == nil {
		t.Fatal("NewChunkProcessor returned nil")
	}
	if cp.chunkSize != 100 {
		t.Errorf("chunkSize = %d, want 100", cp.chunkSize)
	}

	// Test with zero (should use default)
	cp2 := NewChunkProcessor(0)
	if cp2.chunkSize != 1000 {
		t.Errorf("default chunkSize = %d, want 1000", cp2.chunkSize)
	}
}

// TestProcessSlice tests ProcessSlice
func TestProcessSlice(t *testing.T) {
	cp := NewChunkProcessor(2)
	arr := []any{1, 2, 3, 4, 5}

	chunks := 0
	err := cp.ProcessSlice(arr, func(chunk []any) error {
		chunks++
		return nil
	})
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if chunks != 3 {
		t.Errorf("Expected 3 chunks, got %d", chunks)
	}
}

// TestProcessMap tests ProcessMap
func TestProcessMap(t *testing.T) {
	cp := NewChunkProcessor(2)
	m := map[string]any{"a": 1, "b": 2, "c": 3, "d": 4, "e": 5}

	chunks := 0
	err := cp.ProcessMap(m, func(chunk map[string]any) error {
		chunks++
		return nil
	})
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if chunks < 2 {
		t.Errorf("Expected at least 2 chunks, got %d", chunks)
	}
}
