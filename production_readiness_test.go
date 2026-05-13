package json

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

// TestNavigateToPath covers navigateToPath (0% coverage) which dispatches
// to navigateDotNotation or navigateJSONPointer.
func TestNavigateToPath(t *testing.T) {
	p, err := New()
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	defer p.Close()

	data := map[string]any{
		"name": "test",
		"nested": map[string]any{
			"value": 42,
		},
		"items": []any{"a", "b", "c"},
	}

	tests := []struct {
		name    string
		path    string
		want    any
		wantErr bool
	}{
		{name: "empty returns root", path: "", wantErr: false},
		{name: "dot returns root", path: ".", wantErr: false},
		{name: "slash returns root", path: "/", wantErr: false},
		{name: "dot property", path: "name", want: "test"},
		{name: "dot nested", path: "nested.value", want: 42},
		{name: "dot array", path: "items.1", want: "b"},
		{name: "pointer property", path: "/name", want: "test"},
		{name: "pointer nested", path: "/nested/value", want: 42},
		{name: "pointer array", path: "/items/1", want: "b"},
		{name: "missing dot", path: "missing", wantErr: true},
		{name: "missing pointer", path: "/missing", wantErr: true},
		{name: "oob pointer array", path: "/items/99", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := p.navigateToPath(data, tt.path)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error for path %q", tt.path)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error for path %q: %v", tt.path, err)
			}
			if tt.path == "" || tt.path == "." || tt.path == "/" {
				if m, ok := got.(map[string]any); !ok || m["name"] != "test" {
					t.Fatalf("expected root data, got %v", got)
				}
				return
			}
			if got != tt.want {
				t.Fatalf("path %q: expected %v, got %v", tt.path, tt.want, got)
			}
		})
	}
}

// TestNavigateJSONPointer_Escaping covers tilde/slash escape in JSON Pointer.
func TestNavigateJSONPointer_Escaping(t *testing.T) {
	p, err := New()
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	defer p.Close()

	data := map[string]any{
		"a/b": "slash_key",
		"c~d": "tilde_key",
		"e":   map[string]any{"f/g": "nested_slash"},
	}

	tests := []struct {
		name, path, want string
	}{
		{"slash", "/a~1b", "slash_key"},
		{"tilde", "/c~0d", "tilde_key"},
		{"nested", "/e/f~1g", "nested_slash"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := p.navigateJSONPointer(data, tt.path)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("expected %q, got %v", tt.want, got)
			}
		})
	}
}

// TestNavigateDotNotation_Segments covers property, array, and slice access via dot notation.
// Note: extraction paths (items{name}) are handled through a separate recursive code path
// and are tested via the public Get API in coverage_gap_test.go.
func TestNavigateDotNotation_Segments(t *testing.T) {
	p, err := New()
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	defer p.Close()

	data := map[string]any{
		"items": []any{"a", "b", "c"},
		"nested": map[string]any{
			"deep": map[string]any{
				"val": 99,
			},
		},
	}

	t.Run("property access", func(t *testing.T) {
		got, err := p.navigateDotNotation(data, "nested.deep.val")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != 99 {
			t.Fatalf("expected 99, got %v", got)
		}
	})

	t.Run("array index", func(t *testing.T) {
		got, err := p.navigateDotNotation(data, "items.1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "b" {
			t.Fatalf("expected 'b', got %v", got)
		}
	})

	t.Run("out of bounds", func(t *testing.T) {
		_, err := p.navigateDotNotation(data, "items.99999")
		if err == nil {
			t.Fatal("expected error for out-of-bounds")
		}
	})

	t.Run("missing property", func(t *testing.T) {
		_, err := p.navigateDotNotation(data, "noexist")
		if err == nil {
			t.Fatal("expected error for missing property")
		}
	})
}

// TestHandleArraySlice_Coverage covers handleArraySlice (0% coverage).
func TestHandleArraySlice_Coverage(t *testing.T) {
	p, err := New()
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	defer p.Close()

	arr := []any{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}

	tests := []struct {
		name      string
		hasStart  bool
		start     int
		hasEnd    bool
		end       int
		hasStep   bool
		step      int
		wantLen   int
		wantFirst any
	}{
		{name: "basic [2:5]", hasStart: true, start: 2, hasEnd: true, end: 5, wantLen: 3, wantFirst: 2},
		{name: "step [0:10:3]", hasStart: true, start: 0, hasEnd: true, end: 10, hasStep: true, step: 3, wantLen: 4, wantFirst: 0},
		{name: "no bounds", wantLen: 10, wantFirst: 0},
		{name: "start only", hasStart: true, start: 7, wantLen: 3, wantFirst: 7},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			seg := newArraySliceSegment(tt.start, tt.end, tt.step, tt.hasStart, tt.hasEnd, tt.hasStep)
			result := p.handleArraySlice(arr, seg)
			if !result.exists {
				t.Fatal("expected exists=true")
			}
			got, ok := result.value.([]any)
			if !ok {
				t.Fatalf("expected []any, got %T", result.value)
			}
			if len(got) != tt.wantLen {
				t.Fatalf("expected %d elements, got %d", tt.wantLen, len(got))
			}
			if tt.wantLen > 0 && got[0] != tt.wantFirst {
				t.Fatalf("first element: expected %v, got %v", tt.wantFirst, got[0])
			}
		})
	}

	t.Run("non-array", func(t *testing.T) {
		seg := newArraySliceSegment(0, 2, 1, true, true, false)
		result := p.handleArraySlice("not array", seg)
		if result.exists {
			t.Fatal("expected exists=false for non-array")
		}
	})
}

// TestAssignValueToSlice_Coverage covers assignValueToSlice (0% coverage).
func TestAssignValueToSlice_Coverage(t *testing.T) {
	p, err := New()
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	defer p.Close()

	t.Run("basic range", func(t *testing.T) {
		arr := []any{0, 1, 2, 3, 4}
		if err := p.assignValueToSlice(arr, 1, 4, 1, "x"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		for i, want := range []any{0, "x", "x", "x", 4} {
			if arr[i] != want {
				t.Fatalf("index %d: expected %v, got %v", i, want, arr[i])
			}
		}
	})

	t.Run("stepped", func(t *testing.T) {
		arr := []any{0, 1, 2, 3, 4, 5}
		if err := p.assignValueToSlice(arr, 0, 6, 2, "z"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		for i, want := range []any{"z", 1, "z", 3, "z", 5} {
			if arr[i] != want {
				t.Fatalf("index %d: expected %v, got %v", i, want, arr[i])
			}
		}
	})

	t.Run("invalid start>=end", func(t *testing.T) {
		if err := p.assignValueToSlice([]any{0, 1, 2}, 2, 1, 1, "x"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("negative start", func(t *testing.T) {
		if err := p.assignValueToSlice([]any{0, 1, 2}, -1, 2, 1, "x"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("end > len", func(t *testing.T) {
		if err := p.assignValueToSlice([]any{0, 1, 2}, 0, 10, 1, "x"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("zero step", func(t *testing.T) {
		if err := p.assignValueToSlice([]any{0, 1, 2}, 0, 2, 0, "x"); err == nil {
			t.Fatal("expected error")
		}
	})
}

// TestDeleteArrayElement_Coverage covers deleteArrayElement (0% coverage).
func TestDeleteArrayElement_Coverage(t *testing.T) {
	p, err := New()
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	defer p.Close()

	t.Run("valid index", func(t *testing.T) {
		arr := []any{0, 1, 2, 3, 4}
		container := map[string]any{"arr": arr}
		if err := p.deleteArrayElement(container["arr"], "2"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("invalid index string", func(t *testing.T) {
		arr := []any{0, 1, 2}
		if err := p.deleteArrayElement(arr, "abc"); err == nil {
			t.Fatal("expected error for invalid index")
		}
	})
}

// TestNavigateArrayIndexForDeletion_Coverage covers navigateArrayIndexForDeletion (0%).
func TestNavigateArrayIndexForDeletion_Coverage(t *testing.T) {
	p, err := New()
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	defer p.Close()

	arr := []any{10, 20, 30, 40}

	t.Run("valid", func(t *testing.T) {
		got, err := p.navigateArrayIndexForDeletion(arr, "2")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != 30 {
			t.Fatalf("expected 30, got %v", got)
		}
	})

	t.Run("negative", func(t *testing.T) {
		got, err := p.navigateArrayIndexForDeletion(arr, "-1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != 40 {
			t.Fatalf("expected 40, got %v", got)
		}
	})

	t.Run("oob negative", func(t *testing.T) {
		if _, err := p.navigateArrayIndexForDeletion(arr, "-10"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("non-array", func(t *testing.T) {
		if _, err := p.navigateArrayIndexForDeletion("str", "0"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("invalid string", func(t *testing.T) {
		if _, err := p.navigateArrayIndexForDeletion(arr, "x"); err == nil {
			t.Fatal("expected error")
		}
	})
}

// TestSetValueForArrayIndex_Coverage covers setValueForArrayIndex (0%).
func TestSetValueForArrayIndex_Coverage(t *testing.T) {
	p, err := New()
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	defer p.Close()

	t.Run("valid", func(t *testing.T) {
		arr := []any{0, 1, 2}
		if err := p.setValueForArrayIndex(arr, 1, "new", false); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if arr[1] != "new" {
			t.Fatalf("expected 'new', got %v", arr[1])
		}
	})

	t.Run("negative", func(t *testing.T) {
		arr := []any{0, 1, 2}
		if err := p.setValueForArrayIndex(arr, -1, "last", false); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if arr[2] != "last" {
			t.Fatalf("expected 'last', got %v", arr[2])
		}
	})

	t.Run("oob no create", func(t *testing.T) {
		if err := p.setValueForArrayIndex([]any{0, 1, 2}, 10, "x", false); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("oob with create returns signal", func(t *testing.T) {
		err := p.setValueForArrayIndex([]any{0, 1, 2}, 5, "x", true)
		if err == nil {
			t.Fatal("expected arrayExtensionSignal")
		}
		sig, ok := err.(*arrayExtensionSignal)
		if !ok {
			t.Fatalf("expected *arrayExtensionSignal, got %T", err)
		}
		if sig.requiredLength != 6 {
			t.Fatalf("expected requiredLength=6, got %d", sig.requiredLength)
		}
	})

	t.Run("non-array", func(t *testing.T) {
		if err := p.setValueForArrayIndex("str", 0, "x", false); err == nil {
			t.Fatal("expected error")
		}
	})
}

// TestSetValueForArraySlice_Coverage covers setValueForArraySlice (0%).
func TestSetValueForArraySlice_Coverage(t *testing.T) {
	p, err := New()
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	defer p.Close()

	t.Run("basic slice set", func(t *testing.T) {
		arr := []any{0, 1, 2, 3, 4}
		seg := newArraySliceSegment(1, 4, 1, true, true, false)
		if err := p.setValueForArraySlice(arr, seg, "x", false); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		for i, want := range []any{0, "x", "x", "x", 4} {
			if arr[i] != want {
				t.Fatalf("index %d: expected %v, got %v", i, want, arr[i])
			}
		}
	})

	t.Run("stepped slice set", func(t *testing.T) {
		arr := []any{0, 1, 2, 3, 4, 5}
		seg := newArraySliceSegment(0, 6, 2, true, true, true)
		if err := p.setValueForArraySlice(arr, seg, "z", false); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		for i, want := range []any{"z", 1, "z", 3, "z", 5} {
			if arr[i] != want {
				t.Fatalf("index %d: expected %v, got %v", i, want, arr[i])
			}
		}
	})

	t.Run("end oob no create", func(t *testing.T) {
		arr := []any{0, 1, 2}
		seg := newArraySliceSegment(0, 10, 1, true, true, false)
		if err := p.setValueForArraySlice(arr, seg, "x", false); err == nil {
			t.Fatal("expected error for end > len")
		}
	})

	t.Run("end oob with create returns signal", func(t *testing.T) {
		arr := []any{0, 1, 2}
		seg := newArraySliceSegment(0, 10, 1, true, true, false)
		err := p.setValueForArraySlice(arr, seg, "x", true)
		if err == nil {
			t.Fatal("expected arrayExtensionSignal")
		}
		if _, ok := err.(*arrayExtensionSignal); !ok {
			t.Fatalf("expected *arrayExtensionSignal, got %T", err)
		}
	})

	t.Run("non-array", func(t *testing.T) {
		seg := newArraySliceSegment(0, 2, 1, true, true, false)
		if err := p.setValueForArraySlice("str", seg, "x", false); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("invalid range", func(t *testing.T) {
		arr := []any{0, 1, 2}
		seg := newArraySliceSegment(2, 1, 1, true, true, false)
		if err := p.setValueForArraySlice(arr, seg, "x", false); err == nil {
			t.Fatal("expected error for start >= end")
		}
	})
}

// TestHandleArrayExtensionAndSet_Coverage covers handleArrayExtensionAndSet (0%).
func TestHandleArrayExtensionAndSet_Coverage(t *testing.T) {
	p, err := New()
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	defer p.Close()

	t.Run("empty segments", func(t *testing.T) {
		sig := &arrayExtensionSignal{requiredLength: 5, currentLength: 3}
		if err := p.handleArrayExtensionAndSet(map[string]any{}, nil, sig); err == nil {
			t.Fatal("expected error for empty segments")
		}
	})
}

// TestEncodeStruct_StdlibFallback covers encodeStruct's stdlib paths (15.4% coverage).
func TestEncodeStruct_StdlibFallback(t *testing.T) {
	type simple struct {
		Name  string `json:"name"`
		Value int    `json:"value"`
	}

	t.Run("default encoding", func(t *testing.T) {
		result, err := Encode(simple{Name: "test", Value: 42})
		if err != nil {
			t.Fatalf("Encode() failed: %v", err)
		}
		assertJSONEqual(t, `{"name":"test","value":42}`, result)
	})

	t.Run("pretty encoding", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.Pretty = true
		result, err := EncodeWithConfig(simple{Name: "test", Value: 42}, cfg)
		if err != nil {
			t.Fatalf("EncodeWithConfig() failed: %v", err)
		}
		if !strings.Contains(result, "\n") {
			t.Fatal("expected newlines in pretty output")
		}
		assertJSONEqual(t, `{"name":"test","value":42}`, result)
	})
}

// TestEncodeStruct_NilPointers covers encoding structs with nil pointer fields.
func TestEncodeStruct_NilPointers(t *testing.T) {
	type ptr struct {
		Name  *string `json:"name"`
		Value *int    `json:"value"`
	}

	t.Run("nil pointers", func(t *testing.T) {
		result, err := Encode(ptr{Name: nil, Value: nil})
		if err != nil {
			t.Fatalf("Encode() failed: %v", err)
		}
		assertJSONEqual(t, `{"name":null,"value":null}`, result)
	})

	t.Run("non-nil pointers", func(t *testing.T) {
		name := "hello"
		val := 99
		result, err := Encode(ptr{Name: &name, Value: &val})
		if err != nil {
			t.Fatalf("Encode() failed: %v", err)
		}
		assertJSONEqual(t, `{"name":"hello","value":99}`, result)
	})
}

// TestEncodeStruct_Nested covers encoding nested structs.
func TestEncodeStruct_Nested(t *testing.T) {
	type inner struct {
		Val int `json:"val"`
	}
	type outer struct {
		Inner inner `json:"inner"`
	}

	result, err := Encode(outer{Inner: inner{Val: 42}})
	if err != nil {
		t.Fatalf("Encode() failed: %v", err)
	}
	assertJSONEqual(t, `{"inner":{"val":42}}`, result)
}

// TestDecoderStreamingSizeLimit covers the Decoder byte-size limit (new feature).
func TestDecoderStreamingSizeLimit(t *testing.T) {
	t.Run("exceeds MaxJSONSize", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.MaxJSONSize = 5
		dec := NewDecoder(strings.NewReader(`{"key": "value"}`), cfg)
		var result any
		if err := dec.Decode(&result); err == nil {
			t.Fatal("expected size limit error")
		}
	})

	t.Run("within MaxJSONSize", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.MaxJSONSize = 1024
		dec := NewDecoder(strings.NewReader(`{"a":1}`), cfg)
		var result map[string]any
		if err := dec.Decode(&result); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result["a"] != float64(1) {
			t.Fatalf("expected a=1, got %v", result["a"])
		}
	})

	t.Run("no limit accepts large", func(t *testing.T) {
		obj := make(map[string]any)
		for i := range 100 {
			obj[fmt.Sprintf("k_%d", i)] = strings.Repeat("x", 50)
		}
		data, _ := json.Marshal(obj)
		dec := NewDecoder(strings.NewReader(string(data)))
		var result map[string]any
		if err := dec.Decode(&result); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(result) != 100 {
			t.Fatalf("expected 100 keys, got %d", len(result))
		}
	})
}

// TestMergeMode_String covers MergeMode.String() (new method).
func TestMergeMode_String(t *testing.T) {
	tests := []struct {
		mode MergeMode
		want string
	}{
		{MergeUnion, "union"},
		{MergeIntersection, "intersection"},
		{MergeDifference, "difference"},
		{MergeMode(99), "unknown(99)"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.mode.String(); got != tt.want {
				t.Fatalf("expected %q, got %q", tt.want, got)
			}
		})
	}
}
