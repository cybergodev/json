package internal

import (
	"reflect"
	"testing"
)

// ============================================================================
// DeepMerge TESTS
// ============================================================================

func TestDeepMerge_Primitives(t *testing.T) {
	tests := []struct {
		name     string
		base     any
		override any
		expected any
	}{
		{"nil override", "hello", nil, nil},
		{"string override", "hello", "world", "world"},
		{"int override", 1, 2, 2},
		{"bool override", true, false, false},
		{"float override", 1.5, 2.5, 2.5},
		{"nil base", nil, "value", "value"},
		{"both nil", nil, nil, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DeepMerge(tt.base, tt.override)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestDeepMerge_Objects(t *testing.T) {
	tests := []struct {
		name     string
		base     map[string]any
		override map[string]any
		expected map[string]any
	}{
		{
			name:     "empty override",
			base:     map[string]any{"a": 1},
			override: map[string]any{},
			expected: map[string]any{"a": 1},
		},
		{
			name:     "add new key",
			base:     map[string]any{"a": 1},
			override: map[string]any{"b": 2},
			expected: map[string]any{"a": 1, "b": 2},
		},
		{
			name:     "override existing key",
			base:     map[string]any{"a": 1, "b": 2},
			override: map[string]any{"b": 3},
			expected: map[string]any{"a": 1, "b": 3},
		},
		{
			name:     "nested merge",
			base:     map[string]any{"nested": map[string]any{"a": 1}},
			override: map[string]any{"nested": map[string]any{"b": 2}},
			expected: map[string]any{"nested": map[string]any{"a": 1, "b": 2}},
		},
		{
			name:     "nested override",
			base:     map[string]any{"nested": map[string]any{"a": 1}},
			override: map[string]any{"nested": "replaced"},
			expected: map[string]any{"nested": "replaced"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DeepMerge(tt.base, tt.override)
			resultMap, ok := result.(map[string]any)
			if !ok {
				t.Fatalf("expected map, got %T", result)
			}

			for key, expectedValue := range tt.expected {
				if !reflect.DeepEqual(resultMap[key], expectedValue) {
					t.Errorf("key %s: expected %v, got %v", key, expectedValue, resultMap[key])
				}
			}
		})
	}
}

func TestDeepMerge_Arrays(t *testing.T) {
	tests := []struct {
		name     string
		base     []any
		override []any
		checkLen int
	}{
		{
			name:     "empty arrays",
			base:     []any{},
			override: []any{},
			checkLen: 0,
		},
		{
			name:     "add elements",
			base:     []any{1, 2},
			override: []any{3, 4},
			checkLen: 4,
		},
		{
			name:     "deduplicate same elements",
			base:     []any{1, 2, 3},
			override: []any{2, 3, 4},
			checkLen: 4, // 1,2,3,4
		},
		{
			name:     "string deduplication",
			base:     []any{"a", "b"},
			override: []any{"b", "c"},
			checkLen: 3, // a,b,c
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DeepMerge(tt.base, tt.override)
			resultArr, ok := result.([]any)
			if !ok {
				t.Fatalf("expected array, got %T", result)
			}
			if len(resultArr) != tt.checkLen {
				t.Errorf("expected length %d, got %d", tt.checkLen, len(resultArr))
			}
		})
	}
}

func TestDeepMix_MixedTypes(t *testing.T) {
	// Object with array
	base := map[string]any{
		"arr": []any{1, 2},
	}
	override := map[string]any{
		"arr": []any{2, 3},
	}

	result := DeepMerge(base, override)
	resultMap := result.(map[string]any)
	arr := resultMap["arr"].([]any)

	// Should have 1,2,3 (deduplicated)
	if len(arr) != 3 {
		t.Errorf("expected 3 elements, got %d: %v", len(arr), arr)
	}
}

func TestDeepMerge_DeepNesting(t *testing.T) {
	base := map[string]any{
		"level1": map[string]any{
			"level2": map[string]any{
				"value": "base",
			},
		},
	}
	override := map[string]any{
		"level1": map[string]any{
			"level2": map[string]any{
				"value": "override",
			},
		},
	}

	result := DeepMerge(base, override)
	resultMap := result.(map[string]any)
	level1 := resultMap["level1"].(map[string]any)
	level2 := level1["level2"].(map[string]any)

	if level2["value"] != "override" {
		t.Errorf("expected 'override', got %v", level2["value"])
	}
}

// ============================================================================
// ArrayItemKey TESTS
// ============================================================================

func TestArrayItemKey(t *testing.T) {
	tests := []struct {
		name     string
		input    any
		expected string
	}{
		{"string", "hello", "s:hello"},
		{"int as float64", float64(42), "n:42"},
		{"float", float64(3.14), "n:3.14"},
		{"bool true", true, "b:true"},
		{"bool false", false, "b:false"},
		{"nil", nil, "null"},
		{"empty object", map[string]any{}, "o:{}"},
		{"simple object", map[string]any{"a": 1}, "o:{\"a\":1}"},
		{"empty array", []any{}, "a:[]"},
		{"simple array", []any{1, 2}, "a:[1,2]"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ArrayItemKey(tt.input)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestArrayItemKey_NestedStructures(t *testing.T) {
	// Nested object
	obj := map[string]any{
		"nested": map[string]any{"key": "value"},
	}
	result := ArrayItemKey(obj)
	if result == "" || result[0] != 'o' {
		t.Errorf("expected object key starting with 'o:', got %q", result)
	}

	// Nested array
	arr := []any{[]any{1, 2}, 3}
	result = ArrayItemKey(arr)
	if result == "" || result[0] != 'a' {
		t.Errorf("expected array key starting with 'a:', got %q", result)
	}
}

func TestArrayItemKey_DefaultType(t *testing.T) {
	// Test with a type not explicitly handled
	type CustomType struct {
		Value int
	}
	input := CustomType{Value: 42}
	result := ArrayItemKey(input)

	if result == "" {
		t.Error("expected non-empty result for custom type")
	}
}

// ============================================================================
// FormatNumberForDedup TESTS
// ============================================================================

func TestFormatNumberForDedup(t *testing.T) {
	tests := []struct {
		name     string
		input    float64
		expected string
	}{
		{"zero", 0, "0"},
		{"positive integer", 42, "42"},
		{"negative integer", -42, "-42"},
		{"float", 3.14, "3.14"},
		{"negative float", -3.14, "-3.14"},
		{"large integer", 1000000, "1000000"},
		{"scientific notation", 1e10, "10000000000"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatNumberForDedup(tt.input)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

// ============================================================================
// IsJSONPointerPath TESTS
// ============================================================================

// ============================================================================
// IsDotNotationPath TESTS
// ============================================================================

// ============================================================================
// IsArrayPath TESTS
// ============================================================================

func TestIsArrayPath(t *testing.T) {
	tests := []struct {
		path     string
		expected bool
	}{
		{"", false},
		{"users", false},
		{"users[0]", true},
		{"data.items[0].name", true},
		{"items[]", true},
		{"no.array", false},
		{"only[bracket", false},
		{"only]bracket", false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := IsArrayPath(tt.path)
			if result != tt.expected {
				t.Errorf("IsArrayPath(%q) = %v, expected %v", tt.path, result, tt.expected)
			}
		})
	}
}

// ============================================================================
// IsSlicePath TESTS
// ============================================================================

func TestIsSlicePath(t *testing.T) {
	tests := []struct {
		path     string
		expected bool
	}{
		{"", false},
		{"users", false},
		{"users[0]", false},
		{"items[0:10]", true},
		{"items[:5]", true},
		{"items[2:]", true},
		{"items[1:5:2]", true},
		{"no.slice", false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := IsSlicePath(tt.path)
			if result != tt.expected {
				t.Errorf("IsSlicePath(%q) = %v, expected %v", tt.path, result, tt.expected)
			}
		})
	}
}

// ============================================================================
// IsExtractionPath TESTS
// ============================================================================

func TestIsExtractionPath(t *testing.T) {
	tests := []struct {
		path     string
		expected bool
	}{
		{"", false},
		{"users", false},
		{"items{name}", false},        // single extraction, not multi-container
		{"items[:]{name,age}", false}, // single extraction
		{"{field}", false},            // single extraction
		{"no.extraction", false},
		{"items{name}[0]", true},         // extraction followed by array access
		{"items{name}:field", true},      // extraction followed by property
		{"data{items}{other}", true},     // extraction followed by extraction
		{"data{items}{flat:name}", true}, // flat extraction
		{"only{bracket", false},
		{"only}bracket", false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := IsExtractionPath(tt.path)
			if result != tt.expected {
				t.Errorf("IsExtractionPath(%q) = %v, expected %v", tt.path, result, tt.expected)
			}
		})
	}
}

// ============================================================================
// IsJSONObject TESTS
// ============================================================================

// ============================================================================
// IsJSONArray TESTS
// ============================================================================

func TestIsJSONArray(t *testing.T) {
	tests := []struct {
		name     string
		input    any
		expected bool
	}{
		{"[]any", []any{1, 2, 3}, true},
		{"empty slice", []any{}, true},
		{"map", map[string]any{"a": 1}, false},
		{"string", "hello", false},
		{"int", 42, false},
		{"nil", nil, false},
		{"[]int", []int{1, 2}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsJSONArray(tt.input)
			if result != tt.expected {
				t.Errorf("IsJSONArray(%v) = %v, expected %v", tt.input, result, tt.expected)
			}
		})
	}
}

// ============================================================================
// IsJSONPrimitive TESTS
// ============================================================================

func TestIsJSONPrimitive(t *testing.T) {
	tests := []struct {
		name     string
		input    any
		expected bool
	}{
		{"string", "hello", true},
		{"int", 42, true},
		{"int32", int32(42), true},
		{"int64", int64(42), true},
		{"float32", float32(3.14), true},
		{"float64", 3.14, true},
		{"bool true", true, true},
		{"bool false", false, true},
		{"nil", nil, true},
		{"map", map[string]any{"a": 1}, false},
		{"slice", []any{1, 2}, false},
		{"struct", struct{ Name string }{"test"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsJSONPrimitive(tt.input)
			if result != tt.expected {
				t.Errorf("IsJSONPrimitive(%v) = %v, expected %v", tt.input, result, tt.expected)
			}
		})
	}
}

// ============================================================================
// TryConvertToArray TESTS
// ============================================================================

// ============================================================================
// IndexIgnoreCase TESTS
// ============================================================================

func TestIndexIgnoreCase(t *testing.T) {
	tests := []struct {
		name     string
		s        string
		pattern  string
		expected int
	}{
		{"exact match", "hello", "hello", 0},
		{"case insensitive string", "Hello", "hello", 0},
		{"mixed case string", "HeLLo WoRLD", "world", 6},
		{"not found", "hello", "xyz", -1},
		{"pattern longer", "hi", "hello", -1},
		{"empty string", "", "hello", -1},
		{"substring at start", "abcdef", "abc", 0},
		{"substring at end", "abcdef", "def", 3},
		{"substring in middle", "abcdef", "cde", 2},
		{"multiple occurrences", "hello hello", "hello", 0},
		// Note: pattern should be lowercase for matching to work correctly
		// The function expects lowercase patterns
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IndexIgnoreCase(tt.s, tt.pattern)
			if result != tt.expected {
				t.Errorf("IndexIgnoreCase(%q, %q) = %d, expected %d", tt.s, tt.pattern, result, tt.expected)
			}
		})
	}
}

func TestIndexIgnoreCase_SpecialCases(t *testing.T) {
	// Pattern should be lowercase for matching to work
	result := IndexIgnoreCase("hello world", "world")
	if result != 6 {
		t.Errorf("expected 6, got %d", result)
	}

	// String with special characters
	result = IndexIgnoreCase("test-123-abc", "123")
	if result != 5 {
		t.Errorf("expected 5, got %d", result)
	}

	// Uppercase pattern matches: both sides are folded. The old matcher folded
	// only the subject, so any caller-supplied pattern containing an uppercase
	// letter (Config.AdditionalDangerousPatterns, global registry) could never
	// be detected — a silent false negative in security scanning.
	result = IndexIgnoreCase("hello world", "WORLD")
	if result != 6 {
		t.Errorf("expected 6 for uppercase pattern, got %d", result)
	}
}

// ============================================================================
// IsMatchPatternIgnoreCase TESTS
// ============================================================================

func TestIsMatchPatternIgnoreCase(t *testing.T) {
	// Both s and pattern are folded before comparison — matching is
	// case-insensitive in both directions, as the name promises.
	tests := []struct {
		s        string
		pattern  string
		expected bool
	}{
		{"hello", "hello", true},
		{"Hello", "hello", true},
		{"HELLO", "hello", true},
		{"hello", "HELLO", true}, // pattern is folded too
		{"HeLLo", "hello", true},
		{"hello", "world", false},
		{"hello", "hell", false},
		{"", "", true},
		{"A", "a", true},
		{"a", "A", true}, // pattern is folded too
		{"ABC", "abc", true},
	}

	for _, tt := range tests {
		t.Run(tt.s+"_"+tt.pattern, func(t *testing.T) {
			result := IsMatchPatternIgnoreCase(tt.s, tt.pattern)
			if result != tt.expected {
				t.Errorf("IsMatchPatternIgnoreCase(%q, %q) = %v, expected %v", tt.s, tt.pattern, result, tt.expected)
			}
		})
	}
}

// ============================================================================
// Integration TESTS
// ============================================================================

func TestDeepMerge_ComplexScenario(t *testing.T) {
	base := map[string]any{
		"user": map[string]any{
			"name": "John",
			"age":  30,
			"tags": []any{"admin", "user"},
		},
		"settings": map[string]any{
			"theme": "dark",
		},
	}

	override := map[string]any{
		"user": map[string]any{
			"age":  31,
			"tags": []any{"user", "moderator"},
		},
		"settings": map[string]any{
			"language": "en",
		},
	}

	result := DeepMerge(base, override)
	resultMap := result.(map[string]any)

	user := resultMap["user"].(map[string]any)
	if user["name"] != "John" {
		t.Error("base name should be preserved")
	}
	if user["age"] != 31 {
		t.Error("age should be overridden")
	}

	settings := resultMap["settings"].(map[string]any)
	if settings["theme"] != "dark" {
		t.Error("theme should be preserved")
	}
	if settings["language"] != "en" {
		t.Error("language should be added")
	}

	tags := user["tags"].([]any)
	// Should have admin, user, moderator (deduplicated)
	if len(tags) != 3 {
		t.Errorf("expected 3 tags, got %d: %v", len(tags), tags)
	}
}

// ============================================================================
// Benchmark TESTS
// ============================================================================

func BenchmarkDeepMerge(b *testing.B) {
	base := map[string]any{
		"a": 1,
		"b": map[string]any{"c": 2},
		"d": []any{1, 2, 3},
	}
	override := map[string]any{
		"a": 10,
		"b": map[string]any{"e": 3},
		"d": []any{3, 4, 5},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = DeepMerge(base, override)
	}
}

func BenchmarkArrayItemKey(b *testing.B) {
	items := []any{
		"string",
		float64(42),
		true,
		nil,
		map[string]any{"key": "value"},
		[]any{1, 2, 3},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, item := range items {
			_ = ArrayItemKey(item)
		}
	}
}

func BenchmarkIndexIgnoreCase(b *testing.B) {
	s := "This is a test string with some content to search through"
	pattern := "CONTENT"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = IndexIgnoreCase(s, pattern)
	}
}

// ============================================================================
// DeepMergeWithMode TESTS - Union Mode
// ============================================================================

func TestDeepMergeWithMode_Union_Objects(t *testing.T) {
	tests := []struct {
		name     string
		base     map[string]any
		override map[string]any
		expected map[string]any
	}{
		{
			name:     "empty objects",
			base:     map[string]any{},
			override: map[string]any{},
			expected: map[string]any{},
		},
		{
			name:     "add new keys",
			base:     map[string]any{"a": 1},
			override: map[string]any{"b": 2},
			expected: map[string]any{"a": 1, "b": 2},
		},
		{
			name:     "override existing keys",
			base:     map[string]any{"a": 1, "b": 2},
			override: map[string]any{"b": 3, "c": 4},
			expected: map[string]any{"a": 1, "b": 3, "c": 4},
		},
		{
			name:     "nested objects",
			base:     map[string]any{"nested": map[string]any{"a": 1}},
			override: map[string]any{"nested": map[string]any{"b": 2}},
			expected: map[string]any{"nested": map[string]any{"a": 1, "b": 2}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DeepMergeWithMode(tt.base, tt.override, MergeUnion)
			resultMap, ok := result.(map[string]any)
			if !ok {
				t.Fatalf("expected map, got %T", result)
			}
			if !reflect.DeepEqual(resultMap, tt.expected) {
				t.Errorf("expected %v, got %v", tt.expected, resultMap)
			}
		})
	}
}

func TestDeepMergeWithMode_Union_Arrays(t *testing.T) {
	tests := []struct {
		name     string
		base     []any
		override []any
		expected []any
	}{
		{
			name:     "empty arrays",
			base:     []any{},
			override: []any{},
			expected: []any{},
		},
		{
			name:     "combine different elements",
			base:     []any{1, 2},
			override: []any{3, 4},
			expected: []any{1, 2, 3, 4},
		},
		{
			name:     "deduplicate same elements",
			base:     []any{1, 2, 3},
			override: []any{2, 3, 4},
			expected: []any{1, 2, 3, 4},
		},
		{
			name:     "string deduplication",
			base:     []any{"a", "b"},
			override: []any{"b", "c"},
			expected: []any{"a", "b", "c"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DeepMergeWithMode(tt.base, tt.override, MergeUnion)
			resultArr, ok := result.([]any)
			if !ok {
				t.Fatalf("expected array, got %T", result)
			}
			if !reflect.DeepEqual(resultArr, tt.expected) {
				t.Errorf("expected %v, got %v", tt.expected, resultArr)
			}
		})
	}
}

// ============================================================================
// DeepMergeWithMode TESTS - Intersection Mode
// ============================================================================

func TestDeepMergeWithMode_Intersection_Objects(t *testing.T) {
	tests := []struct {
		name     string
		base     map[string]any
		override map[string]any
		expected map[string]any
	}{
		{
			name:     "no common keys",
			base:     map[string]any{"a": 1},
			override: map[string]any{"b": 2},
			expected: map[string]any{},
		},
		{
			name:     "some common keys",
			base:     map[string]any{"a": 1, "b": 2, "c": 3},
			override: map[string]any{"b": 20, "c": 30, "d": 40},
			expected: map[string]any{"b": 20, "c": 30},
		},
		{
			name:     "all common keys",
			base:     map[string]any{"a": 1, "b": 2},
			override: map[string]any{"a": 10, "b": 20},
			expected: map[string]any{"a": 10, "b": 20},
		},
		{
			name: "nested objects intersection",
			base: map[string]any{
				"common": map[string]any{"a": 1, "b": 2},
				"onlyA":  1,
			},
			override: map[string]any{
				"common": map[string]any{"a": 10, "c": 3},
				"onlyB":  2,
			},
			expected: map[string]any{
				"common": map[string]any{"a": 10}, // only "a" is common in nested
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DeepMergeWithMode(tt.base, tt.override, MergeIntersection)
			resultMap, ok := result.(map[string]any)
			if !ok {
				t.Fatalf("expected map, got %T", result)
			}
			if !reflect.DeepEqual(resultMap, tt.expected) {
				t.Errorf("expected %v, got %v", tt.expected, resultMap)
			}
		})
	}
}

func TestDeepMergeWithMode_Intersection_Arrays(t *testing.T) {
	tests := []struct {
		name     string
		base     []any
		override []any
		expected []any
	}{
		{
			name:     "no common elements",
			base:     []any{1, 2},
			override: []any{3, 4},
			expected: []any{},
		},
		{
			name:     "some common elements",
			base:     []any{1, 2, 3},
			override: []any{2, 3, 4},
			expected: []any{2, 3},
		},
		{
			name:     "all common elements",
			base:     []any{1, 2},
			override: []any{1, 2},
			expected: []any{1, 2},
		},
		{
			name:     "strings intersection",
			base:     []any{"a", "b", "c"},
			override: []any{"b", "c", "d"},
			expected: []any{"b", "c"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DeepMergeWithMode(tt.base, tt.override, MergeIntersection)
			resultArr, ok := result.([]any)
			if !ok {
				t.Fatalf("expected array, got %T", result)
			}
			if !reflect.DeepEqual(resultArr, tt.expected) {
				t.Errorf("expected %v, got %v", tt.expected, resultArr)
			}
		})
	}
}

// ============================================================================
// DeepMergeWithMode TESTS - Difference Mode
// ============================================================================

func TestDeepMergeWithMode_Difference_Objects(t *testing.T) {
	tests := []struct {
		name     string
		base     map[string]any
		override map[string]any
		expected map[string]any
	}{
		{
			name:     "no overlap - all from base",
			base:     map[string]any{"a": 1, "b": 2},
			override: map[string]any{"c": 3, "d": 4},
			expected: map[string]any{"a": 1, "b": 2},
		},
		{
			name:     "some overlap",
			base:     map[string]any{"a": 1, "b": 2, "c": 3},
			override: map[string]any{"b": 2, "d": 4},
			expected: map[string]any{"a": 1, "c": 3},
		},
		{
			name:     "full overlap - empty result",
			base:     map[string]any{"a": 1, "b": 2},
			override: map[string]any{"a": 1, "b": 2},
			expected: map[string]any{},
		},
		{
			name: "nested objects difference",
			base: map[string]any{
				"onlyA":  1,
				"common": map[string]any{"a": 1, "b": 2},
			},
			override: map[string]any{
				"onlyB":  2,
				"common": map[string]any{"a": 1, "c": 3},
			},
			expected: map[string]any{
				"onlyA":  1,
				"common": map[string]any{"b": 2}, // only "b" is not in override.common
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DeepMergeWithMode(tt.base, tt.override, MergeDifference)
			resultMap, ok := result.(map[string]any)
			if !ok {
				t.Fatalf("expected map, got %T", result)
			}
			if !reflect.DeepEqual(resultMap, tt.expected) {
				t.Errorf("expected %v, got %v", tt.expected, resultMap)
			}
		})
	}
}

func TestDeepMergeWithMode_Difference_Arrays(t *testing.T) {
	tests := []struct {
		name     string
		base     []any
		override []any
		expected []any
	}{
		{
			name:     "no overlap - all from base",
			base:     []any{1, 2},
			override: []any{3, 4},
			expected: []any{1, 2},
		},
		{
			name:     "some overlap",
			base:     []any{1, 2, 3},
			override: []any{2, 3, 4},
			expected: []any{1},
		},
		{
			name:     "full overlap - empty result",
			base:     []any{1, 2},
			override: []any{1, 2},
			expected: []any{},
		},
		{
			name:     "strings difference",
			base:     []any{"a", "b", "c"},
			override: []any{"b", "d"},
			expected: []any{"a", "c"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DeepMergeWithMode(tt.base, tt.override, MergeDifference)
			resultArr, ok := result.([]any)
			if !ok {
				t.Fatalf("expected array, got %T", result)
			}
			if !reflect.DeepEqual(resultArr, tt.expected) {
				t.Errorf("expected %v, got %v", tt.expected, resultArr)
			}
		})
	}
}

// ============================================================================
// DeepMergeWithMode BENCHMARKS
// ============================================================================

func BenchmarkDeepMergeWithMode_Union(b *testing.B) {
	base := map[string]any{
		"a": 1,
		"b": map[string]any{"c": 2},
		"d": []any{1, 2, 3},
	}
	override := map[string]any{
		"a": 10,
		"b": map[string]any{"e": 3},
		"d": []any{3, 4, 5},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = DeepMergeWithMode(base, override, MergeUnion)
	}
}

func BenchmarkDeepMergeWithMode_Intersection(b *testing.B) {
	base := map[string]any{
		"a": 1,
		"b": map[string]any{"c": 2},
		"d": []any{1, 2, 3},
	}
	override := map[string]any{
		"a": 10,
		"b": map[string]any{"e": 3},
		"d": []any{3, 4, 5},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = DeepMergeWithMode(base, override, MergeIntersection)
	}
}

func BenchmarkDeepMergeWithMode_Difference(b *testing.B) {
	base := map[string]any{
		"a": 1,
		"b": map[string]any{"c": 2},
		"d": []any{1, 2, 3},
	}
	override := map[string]any{
		"a": 10,
		"b": map[string]any{"e": 3},
		"d": []any{3, 4, 5},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = DeepMergeWithMode(base, override, MergeDifference)
	}
}

// ============================================================================
// Additional helpers coverage tests
// ============================================================================

// TestCleanupNullValues tests CleanupNullValues function
func TestCleanupNullValues(t *testing.T) {
	t.Run("object with nulls", func(t *testing.T) {
		input := map[string]any{"a": nil, "b": 1, "c": nil, "d": "hello"}
		result := CleanupNullValues(input, false)
		m := result.(map[string]any)
		if _, exists := m["a"]; exists {
			t.Error("null key 'a' should be removed")
		}
		if _, exists := m["c"]; exists {
			t.Error("null key 'c' should be removed")
		}
		if m["b"] != 1 || m["d"] != "hello" {
			t.Error("non-null values should be preserved")
		}
	})

	t.Run("nested object with nulls", func(t *testing.T) {
		input := map[string]any{
			"outer": map[string]any{
				"inner": nil,
				"keep":  42,
			},
		}
		result := CleanupNullValues(input, false)
		outer := result.(map[string]any)["outer"].(map[string]any)
		if _, exists := outer["inner"]; exists {
			t.Error("nested null key should be removed")
		}
		if outer["keep"] != 42 {
			t.Error("nested non-null value should be preserved")
		}
	})

	t.Run("array with nulls and compact", func(t *testing.T) {
		input := map[string]any{
			"items": []any{1, nil, 3, nil, 5},
		}
		result := CleanupNullValues(input, true)
		m := result.(map[string]any)
		arr := m["items"].([]any)
		for _, v := range arr {
			if v == nil {
				t.Error("null items should be removed when compactArrays=true")
			}
		}
	})

	t.Run("array with nulls no compact", func(t *testing.T) {
		input := map[string]any{
			"items": []any{1, nil, 3},
		}
		result := CleanupNullValues(input, false)
		m := result.(map[string]any)
		arr := m["items"].([]any)
		if len(arr) != 3 {
			t.Errorf("array length = %d, want 3 (nulls preserved without compact)", len(arr))
		}
	})
}

// TestConvertNumbersToFloat tests ConvertNumbersToFloat function
func TestConvertNumbersToFloat(t *testing.T) {
	t.Run("basic conversion", func(t *testing.T) {
		input := map[string]any{"int": 42, "float": 3.14, "str": "hello"}
		result := ConvertNumbersToFloat(input)
		m := result.(map[string]any)
		if m["str"] != "hello" {
			t.Error("string should be preserved")
		}
	})

	t.Run("nested conversion", func(t *testing.T) {
		input := map[string]any{
			"data": map[string]any{
				"val": 100,
			},
		}
		result := ConvertNumbersToFloat(input)
		m := result.(map[string]any)
		data := m["data"].(map[string]any)
		// Non-json.Number values are returned as-is
		if data["val"] != 100 {
			t.Errorf("nested int should be preserved, got %v", data["val"])
		}
	})

	t.Run("array conversion", func(t *testing.T) {
		input := []any{1, 2, 3}
		result := ConvertNumbersToFloat(input)
		arr := result.([]any)
		if len(arr) != 3 {
			t.Errorf("array length = %d, want 3", len(arr))
		}
	})
}

// TestMatchBytesIgnoreCase tests matchBytesIgnoreCase function
func TestMatchBytesIgnoreCase(t *testing.T) {
	tests := []struct {
		name string
		a, b string
		want bool
	}{
		{"same case 8 chars", "testtest", "testtest", true},
		{"different case 8 chars", "TESTTEST", "testtest", true},
		{"mixed case 8 chars", "TestTest", "testtest", true},
		{"no match", "testXXXX", "testtest", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchBytesIgnoreCase(tt.a, tt.b)
			if got != tt.want {
				t.Errorf("matchBytesIgnoreCase(%q, %q) = %v, want %v", tt.a, tt.b, got, tt.want)
			}
		})
	}
}
