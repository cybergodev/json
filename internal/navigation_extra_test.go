package internal

import (
	"strings"
	"testing"
)

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
