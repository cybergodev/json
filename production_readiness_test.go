package json

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

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

	t.Run("oob with create returns error", func(t *testing.T) {
		// D-002: arrayExtensionSignal was removed (unreachable from Set);
		// out-of-bounds is now a plain error on every path.
		if err := p.setValueForArrayIndex([]any{0, 1, 2}, 5, "x", true); err == nil {
			t.Fatal("expected error")
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

	t.Run("end oob with create returns error", func(t *testing.T) {
		arr := []any{0, 1, 2}
		seg := newArraySliceSegment(0, 10, 1, true, true, false)
		if err := p.setValueForArraySlice(arr, seg, "x", true); err == nil {
			t.Fatal("expected error")
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

// TestHandleArrayExtensionAndSet_Coverage was removed with the
// arrayExtensionSignal machinery (D-002): the dispatch was unreachable — Set
// intercepts createPaths index/slice finals via setValueForArrayIndexWithExtension.

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
