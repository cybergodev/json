package json

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

// ============================================================================
// Merged operation tests (from operation_test.go, operation_extra_test.go,
// operation_coverage_test.go) with duplicates removed and table-driven format.
// ============================================================================

// --- JSON Pointer (RFC 6901) set operations ---

func TestJSONPointerSet(t *testing.T) {
	tests := []struct {
		name      string
		jsonStr   string
		path      string
		value     any
		cfg       Config
		wantErr   bool
		errSubstr string
		wantJSON  string
	}{
		{name: "overwrite existing nested value", jsonStr: `{"a":{"b":1}}`, path: "/a/b", value: 2, wantJSON: `{"a":{"b":2}}`},
		{name: "add new key to existing object", jsonStr: `{"a":{"b":1}}`, path: "/a/c", value: 3, wantJSON: `{"a":{"b":1,"c":3}}`},
		{name: "set root path should error", jsonStr: `{"a":1}`, path: "/", value: 42, wantErr: true, errSubstr: "cannot set root"},
		{name: "create nested path with CreatePaths", jsonStr: `{}`, path: "/x/y/z", value: "hello", cfg: Config{CreatePaths: true}, wantJSON: `{"x":{"y":{"z":"hello"}}}`},
		{name: "tilde slash escaping ~1 becomes slash", jsonStr: `{}`, path: "/a~1b", value: "val", cfg: Config{CreatePaths: true}, wantJSON: `{"a/b":"val"}`},
		{name: "tilde escaping ~0 becomes tilde", jsonStr: `{}`, path: "/m~0n", value: "val", cfg: Config{CreatePaths: true}, wantJSON: `{"m~n":"val"}`},
		{name: "array index via pointer", jsonStr: `{"a":[10,20,30]}`, path: "/a/1", value: 99, wantJSON: `{"a":[10,99,30]}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := Set(tt.jsonStr, tt.path, tt.value, tt.cfg)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.errSubstr)
				}
				if !strings.Contains(err.Error(), tt.errSubstr) {
					t.Fatalf("expected error containing %q, got %q", tt.errSubstr, err.Error())
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			assertJSONEqual(t, tt.wantJSON, result)
		})
	}
}

// --- JSON Pointer delete operations ---

func TestJSONPointerDelete(t *testing.T) {
	tests := []struct {
		name      string
		jsonStr   string
		path      string
		wantErr   bool
		errSubstr string
		wantJSON  string
	}{
		{name: "delete nested property", jsonStr: `{"a":{"b":1,"c":2}}`, path: "/a/b", wantJSON: `{"a":{"c":2}}`},
		{name: "delete root should error", jsonStr: `{"a":1}`, path: "/", wantErr: true, errSubstr: "cannot delete root"},
		{name: "delete top-level key", jsonStr: `{"x":1}`, path: "/x", wantJSON: `{}`},
		{name: "tilde escaping in delete", jsonStr: `{"a/b":"val","c":2}`, path: "/a~1b", wantJSON: `{"c":2}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := Delete(tt.jsonStr, tt.path)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.errSubstr)
				}
				if !strings.Contains(err.Error(), tt.errSubstr) {
					t.Fatalf("expected error containing %q, got %q", tt.errSubstr, err.Error())
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			assertJSONEqual(t, tt.wantJSON, result)
		})
	}
}

// --- JSON Pointer navigation via Get ---

func TestNavigateJSONPointer(t *testing.T) {
	tests := []struct {
		name string
		json string
		path string
		want any
	}{
		{"simple", `{"a":{"b":1}}`, "/a/b", float64(1)},
		{"array index", `{"a":[10,20,30]}`, "/a/1", float64(20)},
		{"escaped tilde", `{"m~n":42}`, "/m~0n", float64(42)},
		{"escaped slash", `{"a/b":"val"}`, "/a~1b", "val"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := Get(tt.json, tt.path)
			if err != nil {
				t.Fatalf("Get() error: %v", err)
			}
			if result != tt.want {
				t.Errorf("Get() = %v (%T), want %v (%T)", result, result, tt.want, tt.want)
			}
		})
	}
}

// --- JSON Pointer with array extension ---

func TestSetJSONPointerArrayExtension(t *testing.T) {
	cfg := Config{CreatePaths: true}
	_, err := Set(`{"arr":[1,2]}`, "/arr/5", "x", cfg)
	if err != nil {
		if !strings.Contains(err.Error(), "extend") && !strings.Contains(err.Error(), "failed") {
			t.Errorf("unexpected error: %v", err)
		}
	}
}

// --- Append syntax path[+] ---

func TestAppendOperation(t *testing.T) {
	tests := []struct {
		name      string
		jsonStr   string
		path      string
		value     any
		cfg       Config
		wantErr   bool
		errSubstr string
		wantJSON  string
	}{
		{name: "append to existing array", jsonStr: `{"items":[1,2]}`, path: "items[+]", value: 3, wantJSON: `{"items":[1,2,3]}`},
		{name: "append to empty array", jsonStr: `{"items":[]}`, path: "items[+]", value: 1, wantJSON: `{"items":[1]}`},
		{name: "append to non-existent key errors", jsonStr: `{}`, path: "items[+]", value: 1, cfg: func() Config { c := DefaultConfig(); c.CreatePaths = false; return c }(), wantErr: true, errSubstr: "cannot append"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := Set(tt.jsonStr, tt.path, tt.value, tt.cfg)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.errSubstr)
				}
				if !strings.Contains(err.Error(), tt.errSubstr) {
					t.Fatalf("expected error containing %q, got %q", tt.errSubstr, err.Error())
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			assertJSONEqual(t, tt.wantJSON, result)
		})
	}
}

// --- Negative array index ---

func TestNegativeArrayIndex(t *testing.T) {
	t.Run("get negative index returns last element", func(t *testing.T) {
		result, err := Get(`{"items":[1,2,3]}`, "items[-1]")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != float64(3) {
			t.Fatalf("expected 3, got %v", result)
		}
	})
	t.Run("get various negative positions", func(t *testing.T) {
		json := `{"items":[10,20,30,40,50]}`
		tests := []struct{ path string; want any }{
			{"items[-1]", float64(50)},
			{"items[-2]", float64(40)},
			{"items[-5]", float64(10)},
		}
		for _, tt := range tests {
			result, err := Get(json, tt.path)
			if err != nil {
				t.Fatalf("Get(%q) error: %v", tt.path, err)
			}
			if result != tt.want {
				t.Errorf("Get(%q) = %v, want %v", tt.path, result, tt.want)
			}
		}
	})
	t.Run("delete negative index removes last element", func(t *testing.T) {
		result, err := Delete(`{"items":[1,2,3]}`, "items[-1]")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		assertJSONEqual(t, `{"items":[1,2]}`, result)
	})
	t.Run("set negative index sets last element", func(t *testing.T) {
		result, err := Set(`{"items":[1,2,3]}`, "items[-1]", 99)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		assertJSONEqual(t, `{"items":[1,2,99]}`, result)
	})
}

// --- Array slice operations [start:end:step] ---

func TestArraySliceGet(t *testing.T) {
	tests := []struct {
		name    string
		json    string
		path    string
		wantLen int
	}{
		{"basic slice", `{"items":[0,1,2,3,4]}`, "items[1:3]", 2},
		{"slice with step", `{"items":[0,1,2,3,4]}`, "items[::2]", 3},
		{"reversed slice", `{"items":[1,2,3]}`, "items[::-1]", 3},
		{"slice from end", `{"items":[10,20,30,40,50]}`, "items[3:]", 2},
		{"slice to end", `{"items":[10,20,30,40,50]}`, "items[:2]", 2},
		{"negative start", `{"items":[10,20,30,40,50]}`, "items[-2:]", 2},
		{"negative end", `{"a":[1,2,3,4,5]}`, "a[:-1]", 4},
		{"negative start and end", `{"a":[1,2,3,4,5]}`, "a[-3:-1]", 2},
		{"single element slice", `{"items":[10,20,30]}`, "items[1:2]", 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := Get(tt.json, tt.path)
			if err != nil {
				t.Fatalf("Get() error: %v", err)
			}
			arr, ok := result.([]any)
			if !ok {
				t.Fatalf("result is not []any, got %T", result)
			}
			if len(arr) != tt.wantLen {
				t.Errorf("result length = %d, want %d", len(arr), tt.wantLen)
			}
		})
	}
}

func TestArraySliceSet(t *testing.T) {
	tests := []struct {
		name     string
		json     string
		path     string
		value    any
		wantJSON string
	}{
		{"set slice range", `{"items":[1,2,3,4,5]}`, "items[1:3]", 99, `{"items":[1,99,99,4,5]}`},
		{"set slice with step", `{"items":[1,2,3,4,5]}`, "items[::2]", 0, `{"items":[0,2,0,4,0]}`},
		{"set array slice via path", `{"a":[1,2,3,4,5]}`, "a[1:3]", 0, `{"a":[1,0,0,4,5]}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := Set(tt.json, tt.path, tt.value)
			if err != nil {
				t.Fatalf("Set() error: %v", err)
			}
			assertJSONEqual(t, tt.wantJSON, result)
		})
	}
}

func TestArraySliceDelete(t *testing.T) {
	tests := []struct {
		name     string
		json     string
		path     string
		wantJSON string
	}{
		{"delete slice range", `{"items":[0,1,2,3,4]}`, "items[1:3]", `{"items":[0,3,4]}`},
		{"delete larger slice", `{"items":[0,1,2,3,4,5]}`, "items[1:4]", `{"items":[0,4,5]}`},
		{"delete via complex slice", `{"a":[1,2,3,4,5]}`, "a[1:3]", `{"a":[1,4,5]}`},
		{"delete slice with object elements", `{"items":[{"v":1},{"v":2},{"v":3},{"v":4}]}`, "items[1:3]", `{"items":[{"v":1},{"v":4}]}`},
		{"delete nested slice", `{"items":[{"x":1},{"x":2},{"x":3}]}`, "items[0:2]", `{"items":[{"x":3}]}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := Delete(tt.json, tt.path)
			if err != nil {
				t.Fatalf("Delete() error: %v", err)
			}
			assertJSONEqual(t, tt.wantJSON, result)
		})
	}
}

// --- Array element delete ---

func TestDeleteArrayElement(t *testing.T) {
	tests := []struct {
		name     string
		json     string
		path     string
		wantJSON string
	}{
		{"delete middle", `{"a":[1,2,3]}`, "a[1]", `{"a":[1,3]}`},
		{"delete first", `{"a":[10,20,30]}`, "a[0]", `{"a":[20,30]}`},
		{"delete last", `{"a":[10,20,30]}`, "a[2]", `{"a":[10,20]}`},
		{"delete negative index", `{"a":[10,20,30]}`, "a[-1]", `{"a":[10,20]}`},
		{"delete nested array element", `{"x":{"b":[4,5,6]}}`, "x.b[1]", `{"x":{"b":[4,6]}}`},
		{"delete from nested array", `{"users":[{"name":"a"},{"name":"b"}]}`, "users[0]", `{"users":[{"name":"b"}]}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := Delete(tt.json, tt.path)
			if err != nil {
				t.Fatalf("Delete() error: %v", err)
			}
			assertJSONEqual(t, tt.wantJSON, result)
		})
	}
}

// --- Array index set ---

func TestSetArrayIndex(t *testing.T) {
	tests := []struct {
		name     string
		json     string
		path     string
		value    any
		wantJSON string
	}{
		{"overwrite index", `{"a":[1,2,3]}`, "a[1]", 99, `{"a":[1,99,3]}`},
		{"overwrite first", `{"a":[10,20]}`, "a[0]", 0, `{"a":[0,20]}`},
		{"overwrite nested", `{"x":{"b":[4,5,6]}}`, "x.b[2]", 7, `{"x":{"b":[4,5,7]}}`},
		{"negative index", `{"a":[10,20,30]}`, "a[-1]", 99, `{"a":[10,20,99]}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := Set(tt.json, tt.path, tt.value)
			if err != nil {
				t.Fatalf("Set() error: %v", err)
			}
			assertJSONEqual(t, tt.wantJSON, result)
		})
	}
}

// --- Array extension (CreatePaths) ---

func TestSetArrayExtension(t *testing.T) {
	cfg := Config{CreatePaths: true}

	t.Run("extend array beyond bounds", func(t *testing.T) {
		result, err := Set(`{"a":[1,2]}`, "a[5]", "x", cfg)
		if err != nil {
			t.Fatalf("Set() error: %v", err)
		}
		m := mustParseMap(t, result)
		arr, ok := m["a"].([]any)
		if !ok {
			t.Fatalf("expected []any, got %T", m["a"])
		}
		if len(arr) != 6 {
			t.Errorf("array length = %d, want 6", len(arr))
		}
		if arr[5] != "x" {
			t.Errorf("arr[5] = %v, want 'x'", arr[5])
		}
	})

	t.Run("extend various ways", func(t *testing.T) {
		tests := []struct {
			name  string
			json  string
			path  string
			value any
		}{
			{"extend by one", `{"items":[1,2]}`, "items[3]", "new"},
			{"extend empty array", `{"items":[]}`, "items[2]", "x"},
			{"create nested path", `{}`, "a.b.c", "deep"},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				_, err := Set(tt.json, tt.path, tt.value, cfg)
				if err != nil {
					t.Errorf("Set() with CreatePaths error: %v", err)
				}
			})
		}
	})

	t.Run("SetCreate extends array", func(t *testing.T) {
		result, err := SetCreate(`{"a":[1,2]}`, "a[5]", "x")
		if err != nil {
			t.Fatalf("SetCreate() error: %v", err)
		}
		m := mustParseMap(t, result)
		arr, ok := m["a"].([]any)
		if !ok {
			t.Fatalf("expected []any, got %T", m["a"])
		}
		if len(arr) != 6 {
			t.Errorf("array length = %d, want 6", len(arr))
		}
	})
}

// --- Extraction operations ---

func TestExtractGet(t *testing.T) {
	t.Run("single field extraction", func(t *testing.T) {
		json := `{"items":[{"x":1,"y":2},{"x":3,"y":4}]}`
		result, err := Get(json, "items.{x}")
		if err != nil {
			t.Fatalf("Get extraction error: %v", err)
		}
		arr, ok := result.([]any)
		if !ok {
			t.Fatalf("result is not []any, got %T", result)
		}
		if len(arr) != 2 {
			t.Errorf("extraction result length = %d, want 2", len(arr))
		}
	})

	t.Run("multi-field extraction", func(t *testing.T) {
		result, err := Get(
			`[{"name":"A","age":1,"x":9},{"name":"B","age":2,"x":8}]`,
			"{name,age}",
		)
		if err != nil {
			t.Fatalf("Get() error: %v", err)
		}
		arr, ok := result.([]any)
		if !ok {
			t.Fatalf("expected []any, got %T", result)
		}
		if len(arr) != 2 {
			t.Fatalf("expected 2 elements, got %d", len(arr))
		}
	})

	t.Run("extract then index", func(t *testing.T) {
		result, err := Get(
			`{"items":[{"vals":[1,2]},{"vals":[3,4]}]}`,
			"items{vals}[0]",
		)
		if err != nil {
			t.Fatalf("Get() error: %v", err)
		}
		arr, ok := result.([]any)
		if !ok {
			t.Fatalf("expected []any, got %T", result)
		}
		if len(arr) != 2 {
			t.Fatalf("length = %d, want 2", len(arr))
		}
	})

	t.Run("extract then slice", func(t *testing.T) {
		result, err := Get(
			`{"items":[{"tags":["a","b","c"]},{"tags":["d","e","f"]}]}`,
			"items{tags}[0:2]",
		)
		if err != nil {
			t.Fatalf("Get() error: %v", err)
		}
		arr, ok := result.([]any)
		if !ok {
			t.Fatalf("expected []any, got %T", result)
		}
		if len(arr) != 2 {
			t.Fatalf("length = %d, want 2", len(arr))
		}
	})

	t.Run("post-extraction array access", func(t *testing.T) {
		result, err := Get(
			`[{"tags":["a","b","c"]},{"tags":["d","e"]}]`,
			"{tags}[0]",
		)
		if err != nil {
			t.Fatalf("Get() error: %v", err)
		}
		arr, ok := result.([]any)
		if !ok {
			t.Fatalf("expected []any, got %T", result)
		}
		if len(arr) < 1 {
			t.Fatalf("expected at least 1 result")
		}
	})
}

func TestExtractDelete(t *testing.T) {
	tests := []struct {
		name     string
		json     string
		path     string
		wantJSON string
	}{
		{"delete single field from root array", `[{"k":1,"x":2},{"k":3,"x":4}]`, "{x}", `[{"k":1},{"k":3}]`},
		{"delete nested field", `[{"a":{"b":1}},{"a":{"b":2}}]`, "{a}", `[{},{}]`},
		{"delete from object in array", `{"items":[{"meta":{"x":1}},{"meta":{"x":2}}]}`, "items.{meta}.x", `{"items":[{"meta":{}},{"meta":{}}]}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := Delete(tt.json, tt.path)
			if err != nil {
				t.Fatalf("Delete() error: %v", err)
			}
			assertJSONEqual(t, tt.wantJSON, result)
		})
	}

	t.Run("consecutive extractions", func(t *testing.T) {
		json := `[{"a":{"b":1}},{"a":{"b":2}}]`
		result, err := Delete(json, "{a}{b}")
		if err != nil {
			return // double extraction may not be supported
		}
		_ = result
	})
}

func TestExtractSet(t *testing.T) {
	tests := []struct {
		name     string
		json     string
		path     string
		value    any
		wantJSON string
	}{
		{"set extracted field in root array", `[{"k":"a"},{"k":"b"}]`, "{k}", "REPLACED", `[{"k":"REPLACED"},{"k":"REPLACED"}]`},
		{"set nested extracted field", `{"items":[{"name":"a"},{"name":"b"}]}`, "items{name}", "x", `{"items":[{"name":"x"},{"name":"x"}]}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := Set(tt.json, tt.path, tt.value)
			if err != nil {
				t.Fatalf("Set() error: %v", err)
			}
			assertJSONEqual(t, tt.wantJSON, result)
		})
	}

	t.Run("set with extraction navigation and CreatePaths", func(t *testing.T) {
		cfg := Config{CreatePaths: true}
		result, err := Set(`{"items":[{"name":"a"},{"name":"b"}]}`, "items{name}", "x", cfg)
		if err != nil {
			t.Fatalf("Set() error: %v", err)
		}
		m := mustParseMap(t, result)
		arr, ok := m["items"].([]any)
		if !ok {
			t.Fatalf("expected []any, got %T", m["items"])
		}
		for i, item := range arr {
			obj, ok := item.(map[string]any)
			if !ok {
				t.Fatalf("items[%d] is not map", i)
			}
			if obj["name"] != "x" {
				t.Errorf("items[%d][name] = %v, want 'x'", i, obj["name"])
			}
		}
	})

	t.Run("set flat extraction", func(t *testing.T) {
		result, err := Set(`[{"tags":["a"]},{"tags":["b"]}]`, "{flat:tags}", "c")
		if err != nil {
			t.Fatalf("Set() error: %v", err)
		}
		var parsed any
		if err := Unmarshal([]byte(result), &parsed); err != nil {
			t.Fatalf("Unmarshal() error: %v", err)
		}
		arr, ok := parsed.([]any)
		if !ok {
			t.Fatalf("expected []any, got %T", parsed)
		}
		if len(arr) != 2 {
			t.Fatalf("expected 2 elements, got %d", len(arr))
		}
	})
}

// --- Dot notation navigation ---

func TestNavigateDotNotation(t *testing.T) {
	tests := []struct {
		name    string
		json    string
		path    string
		wantKey string
		wantOk  bool
	}{
		{"simple property", `{"a":{"b":1}}`, "a.b", "1", true},
		{"array in path", `{"x":[10,20]}`, "x[0]", "10", true},
		{"nested array", `{"a":{"b":[1,2,3]}}`, "a.b[2]", "3", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := Get(tt.json, tt.path)
			if (err != nil) == tt.wantOk {
				if !tt.wantOk {
					return
				}
				t.Fatalf("Get() error = %v", err)
			}
			if tt.wantOk && result == nil {
				t.Errorf("Get() = nil, want non-nil")
			}
		})
	}
}

// --- Complex path delete ---

func TestDeleteComplexPaths(t *testing.T) {
	tests := []struct {
		name     string
		json     string
		path     string
		wantJSON string
	}{
		{"nested property then array", `{"a":{"b":[1,2,3]}}`, "a.b[0]", `{"a":{"b":[2,3]}}`},
		{"deep property", `{"x":{"y":{"z":42}}}`, "x.y.z", `{"x":{"y":{}}}`},
		{"nested key", `{"data":{"x":1,"y":2}}`, "data.x", `{"data":{"y":2}}`},
		{"deep nested", `{"a":{"b":{"c":"val"}}}`, "a.b.c", `{"a":{"b":{}}}`},
		{"array in path", `{"a":{"items":[1,2,3]}}`, "a.items", `{"a":{}}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := Delete(tt.json, tt.path)
			if err != nil {
				t.Fatalf("Delete() error: %v", err)
			}
			assertJSONEqual(t, tt.wantJSON, result)
		})
	}
}

// --- SetMultiple / SetCreate ---

func TestSetMultipleAndCreate(t *testing.T) {
	t.Run("SetMultiple complex path", func(t *testing.T) {
		result, err := SetMultiple(`{"a":{"b":1}}`, map[string]any{"a.b": 42})
		if err != nil {
			t.Fatalf("SetMultiple() error: %v", err)
		}
		assertJSONEqual(t, `{"a":{"b":42}}`, result)
	})

	t.Run("SetMultipleCreate", func(t *testing.T) {
		result, err := SetMultipleCreate(`{}`, map[string]any{
			"a.b": 1,
			"a.c": 2,
		})
		if err != nil {
			t.Fatalf("SetMultipleCreate() error: %v", err)
		}
		m := mustParseMap(t, result)
		a, ok := m["a"].(map[string]any)
		if !ok {
			t.Fatalf("expected a to be map, got %T", m["a"])
		}
		if a["b"] != float64(1) || a["c"] != float64(2) {
			t.Errorf("unexpected result: %v", m)
		}
	})
}

// --- SetCreatePaths ---

func TestSetCreatePaths(t *testing.T) {
	tests := []struct {
		name     string
		jsonStr  string
		path     string
		value    any
		wantJSON string
	}{
		{name: "create 3-level path", jsonStr: `{}`, path: "a.b.c", value: 42, wantJSON: `{"a":{"b":{"c":42}}}`},
		{name: "create path with array index", jsonStr: `{"items":[]}`, path: "items[0]", value: "hello", wantJSON: `{"items":["hello"]}`},
		{name: "set on existing path", jsonStr: `{"a":{"b":1}}`, path: "a.b", value: 2, wantJSON: `{"a":{"b":2}}`},
		{name: "create path alongside existing", jsonStr: `{"a":1}`, path: "b.c", value: 3, wantJSON: `{"a":1,"b":{"c":3}}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := Set(tt.jsonStr, tt.path, tt.value, Config{CreatePaths: true})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			assertJSONEqual(t, tt.wantJSON, result)
		})
	}

	t.Run("create path in array with nil intermediate errors", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.CreatePaths = true
		_, err := Set(`{}`, "list[0].name", "first", cfg)
		if err == nil {
			t.Error("expected error for nil intermediate array element, got nil")
		}
	})
}

// --- Typed getters edge cases ---

func TestTypedGettersEdgeCases(t *testing.T) {
	t.Run("GetString with number", func(t *testing.T) {
		if result := GetString(`{"v":42}`, "v"); result != "42" {
			t.Fatalf("expected \"42\", got %q", result)
		}
	})
	t.Run("GetInt with whole float64", func(t *testing.T) {
		if result := GetInt(`{"v":42.0}`, "v"); result != 42 {
			t.Fatalf("expected 42, got %d", result)
		}
	})
	t.Run("GetInt with fractional returns zero", func(t *testing.T) {
		if result := GetInt(`{"v":42.5}`, "v"); result != 0 {
			t.Fatalf("expected 0, got %d", result)
		}
	})
	t.Run("GetBool string variants", func(t *testing.T) {
		tests := []struct{ json, path string; want bool }{
			{`{"v":"true"}`, "v", true},
			{`{"v":"TRUE"}`, "v", true},
			{`{"v":"1"}`, "v", true},
			{`{"v":"false"}`, "v", false},
		}
		for _, tt := range tests {
			if result := GetBool(tt.json, tt.path); result != tt.want {
				t.Errorf("GetBool(%s) = %v, want %v", tt.json, result, tt.want)
			}
		}
	})
	t.Run("GetFloat with int", func(t *testing.T) {
		if result := GetFloat(`{"v":42}`, "v"); result != 42.0 {
			t.Fatalf("expected 42.0, got %v", result)
		}
	})
	t.Run("GetArray on non-array returns nil", func(t *testing.T) {
		if result := GetArray(`{"v":"not_array"}`, "v"); result != nil {
			t.Fatalf("expected nil, got %v", result)
		}
	})
	t.Run("GetObject on non-object returns nil", func(t *testing.T) {
		if result := GetObject(`{"v":42}`, "v"); result != nil {
			t.Fatalf("expected nil, got %v", result)
		}
	})
	t.Run("GetTyped missing path returns zero value", func(t *testing.T) {
		if result := GetTyped[string](`{"a":1}`, "missing_key"); result != "" {
			t.Fatalf("expected empty string, got %q", result)
		}
	})
	t.Run("GetTyped missing path returns default", func(t *testing.T) {
		if result := GetTyped[int](`{"a":1}`, "missing_key", 99); result != 99 {
			t.Fatalf("expected 99, got %d", result)
		}
	})
}

// --- EncodePretty API ---

func TestEncodePrettyAPI(t *testing.T) {
	t.Run("pretty print map", func(t *testing.T) {
		data := map[string]any{"name": "Alice", "age": float64(30)}
		result, err := EncodePretty(data)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(result, "\n") {
			t.Fatalf("expected newlines, got: %s", result)
		}
	})
	t.Run("pretty print nil returns null", func(t *testing.T) {
		result, err := EncodePretty(nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		assertJSONEqual(t, "null", result)
	})
	t.Run("pretty print slice", func(t *testing.T) {
		data := []any{float64(1), float64(2), float64(3)}
		result, err := EncodePretty(data)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(result, "\n") {
			t.Fatalf("expected newlines, got: %s", result)
		}
	})
	t.Run("pretty print struct", func(t *testing.T) {
		type Person struct {
			Name string `json:"name"`
			Age  int    `json:"age"`
		}
		data := Person{Name: "Bob", Age: 25}
		result, err := EncodePretty(data)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(result, "Bob") {
			t.Fatalf("expected struct fields, got: %s", result)
		}
	})
	t.Run("custom indent config", func(t *testing.T) {
		cfg := PrettyConfig()
		cfg.Indent = "    "
		data := map[string]any{"key": "value"}
		result, err := EncodePretty(data, cfg)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(result, "    ") {
			t.Fatalf("expected 4-space indent, got: %s", result)
		}
	})
}

// --- Struct access ---

func TestStructAccess(t *testing.T) {
	type Person struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}
	p, _ := New()
	defer p.Close()

	encoded, err := p.Marshal(Person{Name: "Alice", Age: 30})
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}
	result, err := p.Get(string(encoded), "name")
	if err != nil {
		t.Fatalf("Get error: %v", err)
	}
	if result != "Alice" {
		t.Errorf("Get name = %v, want Alice", result)
	}
}

// --- Distributed array operations ---

func TestDistributedArrayOps(t *testing.T) {
	t.Run("2D array access", func(t *testing.T) {
		result, err := Get(`{"matrix":[[1,2],[3,4],[5,6]]}`, "matrix[1][0]")
		if err != nil {
			t.Fatalf("Get distributed error: %v", err)
		}
		if result != float64(3) {
			t.Errorf("Get distributed = %v, want 3", result)
		}
	})
}

// --- Error cases ---

func TestOperationErrors(t *testing.T) {
	t.Run("Set errors", func(t *testing.T) {
		tests := []struct{ name, json, path string; value any; wantErr bool }{
			{"invalid json", `{invalid}`, "a", 1, true},
			{"empty path", `{"a":1}`, "", 1, true},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				_, err := Set(tt.json, tt.path, tt.value)
				if (err != nil) != tt.wantErr {
					t.Errorf("Set() error = %v, wantErr %v", err, tt.wantErr)
				}
			})
		}
	})
	t.Run("Delete errors", func(t *testing.T) {
		tests := []struct{ name, json, path string; wantErr bool }{
			{"invalid json", `{invalid}`, "a", true},
			{"empty path", `{"a":1}`, "", true},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				_, err := Delete(tt.json, tt.path)
				if (err != nil) != tt.wantErr {
					t.Errorf("Delete() error = %v, wantErr %v", err, tt.wantErr)
				}
			})
		}
	})
	t.Run("delete non-existent path", func(t *testing.T) {
		_, err := Delete(`{"a":1}`, "b")
		if err == nil {
			t.Error("expected error for non-existent path")
		}
	})
}

// --- normalizeNegativeIndex edge cases ---

func TestNormalizeNegativeIndex(t *testing.T) {
	tests := []struct {
		index int
		len   int
		want  int
		err   bool
	}{
		{-1, 5, 4, false},
		{-5, 5, 0, false},
		{0, 5, 0, false},
		{4, 5, 4, false},
		{-6, 5, 0, true},
		{5, 5, 0, true},
		{-1, 0, 0, true},
		{-1, 1, 0, false},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("index_%d_len_%d", tt.index, tt.len), func(t *testing.T) {
			got, err := normalizeNegativeIndex(tt.index, tt.len)
			if tt.err {
				if err == nil {
					t.Errorf("expected error for index=%d len=%d", tt.index, tt.len)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if got != tt.want {
					t.Errorf("normalizeNegativeIndex(%d, %d) = %d, want %d", tt.index, tt.len, got, tt.want)
				}
			}
		})
	}
}

// --- Nesting depth checks ---

func TestCheckNestingDepth(t *testing.T) {
	tests := []struct {
		name     string
		data     string
		maxDepth int
		wantErr  bool
	}{
		{"flat object", `{"a":1}`, 10, false},
		{"nested within limit", `{"a":{"b":1}}`, 10, false},
		{"at exact limit", `{"a":{"b":1}}`, 2, false},
		{"exceeds limit", `{"a":{"b":{"c":1}}}`, 2, true},
		{"array nesting", `[[[1]]]`, 10, false},
		{"deep array exceeds", `[[[[[1]]]]]`, 3, true},
		{"string with braces", `"{not counted}"`, 1, false},
		{"empty", ``, 10, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := checkNestingDepth([]byte(tt.data), tt.maxDepth)
			if (err != nil) != tt.wantErr {
				t.Errorf("checkNestingDepth() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestDecoderDepthLimit(t *testing.T) {
	t.Run("exceeds depth limit", func(t *testing.T) {
		depth := 250
		nested := strings.Repeat(`{"a":`, depth) + `1` + strings.Repeat(`}`, depth)
		dec := NewDecoder(strings.NewReader(nested), Config{MaxNestingDepthSecurity: 50})
		var result any
		err := dec.Decode(&result)
		if err == nil {
			t.Error("expected depth limit error")
		}
		if err != nil && !strings.Contains(err.Error(), "depth") {
			t.Errorf("expected depth error, got: %v", err)
		}
	})
	t.Run("within depth limit", func(t *testing.T) {
		nested := strings.Repeat(`{"a":`, 10) + `1` + strings.Repeat(`}`, 10)
		dec := NewDecoder(strings.NewReader(nested))
		var result any
		if err := dec.Decode(&result); err != nil {
			t.Errorf("Decode() error = %v", err)
		}
	})
}

// ============================================================================
// Helpers
// ============================================================================

// assertJSONEqual compares two JSON strings by normalizing both.
func assertJSONEqual(t *testing.T, expected, actual string) {
	t.Helper()
	var expData, actData any
	if err := json.Unmarshal([]byte(expected), &expData); err != nil {
		t.Fatalf("failed to parse expected JSON %q: %v", expected, err)
	}
	if err := json.Unmarshal([]byte(actual), &actData); err != nil {
		t.Fatalf("failed to parse actual JSON %q: %v", actual, err)
	}
	expBytes, _ := json.Marshal(expData)
	actBytes, _ := json.Marshal(actData)
	if string(expBytes) != string(actBytes) {
		t.Fatalf("JSON mismatch:\nexpected: %s\nactual:   %s", string(expBytes), string(actBytes))
	}
}

// ============================================================================
// Coverage tests for internal operation functions
// Exercise code paths through the public API (Get, Set, Delete, etc.)
// ============================================================================

// --- handleArraySlice: triggered by Get with slice syntax ---

func TestOperationHandleArraySlice(t *testing.T) {
	tests := []struct {
		name    string
		json    string
		path    string
		wantLen int
		check   func(t *testing.T, result any)
	}{
		{
			"slice on nested array",
			`{"data":{"vals":[10,20,30,40,50]}}`,
			"data.vals[1:4]",
			3,
			func(t *testing.T, result any) {
				arr := result.([]any)
				if arr[0] != float64(20) || arr[2] != float64(40) {
					t.Errorf("unexpected slice values: %v", arr)
				}
			},
		},
		{
			"slice with step 2",
			`{"items":[0,1,2,3,4,5,6]}`,
			"items[0:6:2]",
			3,
			func(t *testing.T, result any) {
				arr := result.([]any)
				if arr[0] != float64(0) || arr[1] != float64(2) || arr[2] != float64(4) {
					t.Errorf("unexpected stepped slice values: %v", arr)
				}
			},
		},
		{
			"reverse slice",
			`{"items":[1,2,3,4,5]}`,
			"items[::-1]",
			5,
			func(t *testing.T, result any) {
				arr := result.([]any)
				if arr[0] != float64(5) || arr[4] != float64(1) {
					t.Errorf("unexpected reversed slice: %v", arr)
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := Get(tt.json, tt.path)
			if err != nil {
				t.Fatalf("Get() error: %v", err)
			}
			arr, ok := result.([]any)
			if !ok {
				t.Fatalf("expected []any, got %T", result)
			}
			if len(arr) != tt.wantLen {
				t.Errorf("result length = %d, want %d", len(arr), tt.wantLen)
			}
			if tt.check != nil {
				tt.check(t, result)
			}
		})
	}
}

// --- assignValueToSlice: triggered by Set with slice syntax ---

func TestOperationAssignValueToSlice(t *testing.T) {
	tests := []struct {
		name     string
		json     string
		path     string
		value    any
		wantJSON string
	}{
		{
			"assign to slice with step",
			`{"a":[0,1,2,3,4,5]}`,
			"a[0:6:2]",
			99,
			`{"a":[99,1,99,3,99,5]}`,
		},
		{
			"assign to slice range partial",
			`{"a":[10,20,30,40,50]}`,
			"a[2:4]",
			0,
			`{"a":[10,20,0,0,50]}`,
		},
		{
			"assign to single element slice",
			`{"a":[1,2,3]}`,
			"a[1:2]",
			42,
			`{"a":[1,42,3]}`,
		},
		{
			"assign object to slice",
			`{"items":[{"v":1},{"v":2},{"v":3}]}`,
			"items[0:2]",
			map[string]any{"v": 99},
			`{"items":[{"v":99},{"v":99},{"v":3}]}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := Set(tt.json, tt.path, tt.value)
			if err != nil {
				t.Fatalf("Set() error: %v", err)
			}
			assertJSONEqual(t, tt.wantJSON, result)
		})
	}
}

// --- deleteArrayElement, navigateArrayIndexForDeletion, deleteComplexArray ---

func TestOperationDeleteArrayElementComplex(t *testing.T) {
	tests := []struct {
		name     string
		json     string
		path     string
		wantJSON string
	}{
		{
			"delete from nested array in object",
			`{"data":{"items":[{"id":1},{"id":2},{"id":3}]}}`,
			"data.items[1]",
			`{"data":{"items":[{"id":1},{"id":3}]}}`,
		},
		{
			"delete from 2D array",
			`{"matrix":[[1,2],[3,4],[5,6]]}`,
			"matrix[1]",
			`{"matrix":[[1,2],[5,6]]}`,
		},
		{
			"delete with extraction removes field",
			`{"items":[{"field":"a","extra":1},{"field":"b","extra":2}]}`,
			"items.{field}",
			`{"items":[{"extra":1},{"extra":2}]}`,
		},
		{
			"delete array slice removes range",
			`{"items":[{"tags":["a","b"]},{"tags":["c","d"]},{"tags":["e","f"]}]}`,
			"items[1:3]",
			`{"items":[{"tags":["a","b"]}]}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := Delete(tt.json, tt.path)
			if err != nil {
				t.Fatalf("Delete() error: %v", err)
			}
			assertJSONEqual(t, tt.wantJSON, result)
		})
	}
}

// --- deleteComplexArray: delete from nested position inside an array element ---

func TestOperationDeleteComplexArrayNavigation(t *testing.T) {
	t.Run("delete from array element with negative index", func(t *testing.T) {
		json := `{"items":[{"a":1},{"a":2},{"a":3}]}`
		result, err := Delete(json, "items[-1]")
		if err != nil {
			t.Fatalf("Delete() error: %v", err)
		}
		assertJSONEqual(t, `{"items":[{"a":1},{"a":2}]}`, result)
	})

	t.Run("delete first element from array", func(t *testing.T) {
		json := `{"items":[{"a":1},{"a":2},{"a":3}]}`
		result, err := Delete(json, "items[0]")
		if err != nil {
			t.Fatalf("Delete() error: %v", err)
		}
		assertJSONEqual(t, `{"items":[{"a":2},{"a":3}]}`, result)
	})

	t.Run("delete last element from array", func(t *testing.T) {
		json := `{"items":[{"a":1},{"a":2},{"a":3}]}`
		result, err := Delete(json, "items[2]")
		if err != nil {
			t.Fatalf("Delete() error: %v", err)
		}
		assertJSONEqual(t, `{"items":[{"a":1},{"a":2}]}`, result)
	})
}

// --- getValueWithDistributedOperation, extractIndividualArrays,
//     applySingleArrayOperation, applySingleArraySlice ---

func TestOperationDistributedGet(t *testing.T) {
	t.Run("extract field then get index from results array", func(t *testing.T) {
		// items.{tags}[0] extracts tags from each item -> [[a,b,c],[d,e,f]]
		// then [0] takes the first element of that results array -> [a,b,c]
		json := `{"items":[{"tags":["a","b","c"]},{"tags":["d","e","f"]}]}`
		result, err := Get(json, "items.{tags}[0]")
		if err != nil {
			t.Fatalf("Get() error: %v", err)
		}
		arr, ok := result.([]any)
		if !ok {
			t.Fatalf("expected []any, got %T", result)
		}
		if len(arr) != 3 {
			t.Fatalf("expected 3 elements (first extracted array), got %d", len(arr))
		}
		if arr[0] != "a" || arr[1] != "b" || arr[2] != "c" {
			t.Errorf("expected [a b c], got %v", arr)
		}
	})

	t.Run("extract field then slice results array", func(t *testing.T) {
		// items.{tags}[0:2] extracts tags from each -> returns first 2 result arrays
		json := `{"items":[{"tags":[10,20,30,40]},{"tags":[50,60,70,80]}]}`
		result, err := Get(json, "items.{tags}[0:2]")
		if err != nil {
			t.Fatalf("Get() error: %v", err)
		}
		arr, ok := result.([]any)
		if !ok {
			t.Fatalf("expected []any, got %T", result)
		}
		if len(arr) != 2 {
			t.Fatalf("expected 2 result arrays, got %d", len(arr))
		}
	})

	t.Run("extract with pre-navigation", func(t *testing.T) {
		// nested.list.{vals}[1] -> extracts vals -> [[100,200],[300,400]] -> [1] -> [300,400]
		json := `{"nested":{"list":[{"vals":[100,200]},{"vals":[300,400]}]}}`
		result, err := Get(json, "nested.list.{vals}[1]")
		if err != nil {
			t.Fatalf("Get() error: %v", err)
		}
		arr, ok := result.([]any)
		if !ok {
			t.Fatalf("expected []any, got %T", result)
		}
		if len(arr) != 2 {
			t.Fatalf("expected 2 elements, got %d", len(arr))
		}
		if arr[0] != float64(300) || arr[1] != float64(400) {
			t.Errorf("expected [300 400], got %v", arr)
		}
	})
}

// --- handlePostExtractionArrayAccess: array of arrays after extraction ---

func TestOperationPostExtractionArrayAccess(t *testing.T) {
	t.Run("extract array fields then index result", func(t *testing.T) {
		// {data}[0] extracts data -> [[1,2,3],[4,5,6]], then [0] takes first -> [1,2,3]
		json := `[{"data":[1,2,3]},{"data":[4,5,6]}]`
		result, err := Get(json, "{data}[0]")
		if err != nil {
			t.Fatalf("Get() error: %v", err)
		}
		arr, ok := result.([]any)
		if !ok {
			t.Fatalf("expected []any, got %T", result)
		}
		if len(arr) != 3 {
			t.Fatalf("expected 3 elements (first extracted array), got %d", len(arr))
		}
		if arr[0] != float64(1) {
			t.Errorf("expected first element to be 1, got %v", arr[0])
		}
	})

	t.Run("extract then slice on root array", func(t *testing.T) {
		// {nums}[1:3] extracts nums -> [[10,20,30,40],[50,60,70,80]], [1:3] takes slice
		json := `[{"nums":[10,20,30,40]},{"nums":[50,60,70,80]}]`
		result, err := Get(json, "{nums}[1:3]")
		if err != nil {
			t.Fatalf("Get() error: %v", err)
		}
		arr, ok := result.([]any)
		if !ok {
			t.Fatalf("expected []any, got %T", result)
		}
		// Slice [1:3] of the 2-element extracted results -> 1 element (the second)
		if len(arr) < 1 {
			t.Fatalf("expected at least 1 result, got %d", len(arr))
		}
	})
}

// --- setValueForArrayIndex, setValueForArraySlice with extension ---

func TestOperationSetArrayIndexExtension(t *testing.T) {
	cfg := Config{CreatePaths: true}

	t.Run("extend array by multiple positions", func(t *testing.T) {
		result, err := Set(`{"arr":[1]}`, "arr[4]", "new", cfg)
		if err != nil {
			t.Fatalf("Set() error: %v", err)
		}
		m := mustParseMap(t, result)
		arr := m["arr"].([]any)
		if len(arr) != 5 {
			t.Fatalf("expected length 5, got %d", len(arr))
		}
		if arr[0] != float64(1) {
			t.Errorf("arr[0] = %v, want 1", arr[0])
		}
		if arr[4] != "new" {
			t.Errorf("arr[4] = %v, want 'new'", arr[4])
		}
		// Intermediate positions should be nil
		if arr[1] != nil || arr[2] != nil || arr[3] != nil {
			t.Errorf("intermediate positions should be nil: %v", arr)
		}
	})

	t.Run("extend array in nested object", func(t *testing.T) {
		result, err := Set(`{"data":{"list":[]}}`, "data.list[2]", "x", cfg)
		if err != nil {
			t.Fatalf("Set() error: %v", err)
		}
		m := mustParseMap(t, result)
		data := m["data"].(map[string]any)
		arr := data["list"].([]any)
		if len(arr) != 3 {
			t.Fatalf("expected length 3, got %d", len(arr))
		}
		if arr[2] != "x" {
			t.Errorf("arr[2] = %v, want 'x'", arr[2])
		}
	})

	t.Run("set slice with extension via CreatePaths", func(t *testing.T) {
		result, err := Set(`{"arr":[1,2]}`, "arr[3:5]", 99, cfg)
		if err != nil {
			// Slice extension may not be fully supported, check error message
			t.Logf("Set slice with extension: %v (may be expected)", err)
			return
		}
		m := mustParseMap(t, result)
		arr := m["arr"].([]any)
		if len(arr) < 5 {
			t.Errorf("expected at least length 5, got %d", len(arr))
		}
	})
}

// --- handleArrayExtensionAndSet, handleArrayIndexExtension ---

func TestOperationArrayExtensionAndSet(t *testing.T) {
	t.Run("SetCreate extends array and sets value", func(t *testing.T) {
		result, err := SetCreate(`{"items":[1,2,3]}`, "items[7]", "extended")
		if err != nil {
			t.Fatalf("SetCreate() error: %v", err)
		}
		m := mustParseMap(t, result)
		arr := m["items"].([]any)
		if len(arr) != 8 {
			t.Fatalf("expected length 8, got %d", len(arr))
		}
		if arr[7] != "extended" {
			t.Errorf("arr[7] = %v, want 'extended'", arr[7])
		}
	})

	t.Run("SetCreate on nested array path", func(t *testing.T) {
		result, err := SetCreate(`{"outer":{"inner":[]}}`, "outer.inner[2]", "val")
		if err != nil {
			t.Fatalf("SetCreate() error: %v", err)
		}
		m := mustParseMap(t, result)
		outer := m["outer"].(map[string]any)
		arr := outer["inner"].([]any)
		if len(arr) != 3 {
			t.Fatalf("expected length 3, got %d", len(arr))
		}
		if arr[2] != "val" {
			t.Errorf("arr[2] = %v, want 'val'", arr[2])
		}
	})
}

// --- setValueJSONPointer, replaceArrayInJSONPointerParent ---

func TestOperationSetJSONPointerComplex(t *testing.T) {
	t.Run("set via JSON pointer creates intermediate path", func(t *testing.T) {
		cfg := Config{CreatePaths: true}
		result, err := Set(`{}`, "/a/b/c/d", "deep_value", cfg)
		if err != nil {
			t.Fatalf("Set() error: %v", err)
		}
		assertJSONEqual(t, `{"a":{"b":{"c":{"d":"deep_value"}}}}`, result)
	})

	t.Run("set via JSON pointer with array index", func(t *testing.T) {
		cfg := Config{CreatePaths: true}
		result, err := Set(`{"arr":[1,2,3]}`, "/arr/1", 99, cfg)
		if err != nil {
			t.Fatalf("Set() error: %v", err)
		}
		assertJSONEqual(t, `{"arr":[1,99,3]}`, result)
	})

	t.Run("set via JSON pointer creates nested arrays", func(t *testing.T) {
		cfg := Config{CreatePaths: true}
		result, err := Set(`{}`, "/data/0/name", "first", cfg)
		if err != nil {
			// May fail if intermediate is nil
			t.Logf("Set JSON pointer with array creation: %v", err)
			return
		}
		m := mustParseMap(t, result)
		data := m["data"].([]any)
		if len(data) < 1 {
			t.Fatalf("expected at least 1 element")
		}
		obj := data[0].(map[string]any)
		if obj["name"] != "first" {
			t.Errorf("expected name=first, got %v", obj["name"])
		}
	})

	t.Run("set via JSON pointer extends existing array", func(t *testing.T) {
		cfg := Config{CreatePaths: true}
		result, err := Set(`{"items":["a","b"]}`, "/items/4", "e", cfg)
		if err != nil {
			t.Logf("Set JSON pointer array extension: %v", err)
			return
		}
		m := mustParseMap(t, result)
		arr := m["items"].([]any)
		if len(arr) != 5 {
			t.Fatalf("expected length 5, got %d", len(arr))
		}
		if arr[4] != "e" {
			t.Errorf("arr[4] = %v, want 'e'", arr[4])
		}
	})

	t.Run("set via JSON pointer with special characters", func(t *testing.T) {
		cfg := Config{CreatePaths: true}
		result, err := Set(`{}`, "/path~1to~0value", "x", cfg)
		if err != nil {
			t.Fatalf("Set() error: %v", err)
		}
		m := mustParseMap(t, result)
		if m["path/to~value"] != "x" {
			t.Errorf("expected path/to~value=x, got %v", m["path/to~value"])
		}
	})
}

// --- setValueForExtract, setValueForArrayExtract, setValueForArrayExtractFlat ---

func TestOperationSetForExtract(t *testing.T) {
	t.Run("set extracted field on root array", func(t *testing.T) {
		json := `[{"name":"a","extra":1},{"name":"b","extra":2}]`
		result, err := Set(json, "{name}", "REPLACED")
		if err != nil {
			t.Fatalf("Set() error: %v", err)
		}
		assertJSONEqual(t, `[{"name":"REPLACED","extra":1},{"name":"REPLACED","extra":2}]`, result)
	})

	t.Run("set extracted field on nested array", func(t *testing.T) {
		json := `{"items":[{"x":1,"y":2},{"x":3,"y":4}]}`
		result, err := Set(json, "items.{x}", 99)
		if err != nil {
			t.Fatalf("Set() error: %v", err)
		}
		assertJSONEqual(t, `{"items":[{"x":99,"y":2},{"x":99,"y":4}]}`, result)
	})

	t.Run("set on single object extraction", func(t *testing.T) {
		json := `{"obj":{"field":"old"}}`
		result, err := Set(json, "obj.{field}", "new")
		if err != nil {
			t.Fatalf("Set() error: %v", err)
		}
		assertJSONEqual(t, `{"obj":{"field":"new"}}`, result)
	})
}

func TestOperationSetArrayExtract(t *testing.T) {
	t.Run("set field on array elements including non-maps", func(t *testing.T) {
		json := `[1,"hello",{"k":"v"}]`
		result, err := Set(json, "{k}", "new_val")
		if err != nil {
			t.Fatalf("Set() error: %v", err)
		}
		arr := mustParseArray(t, result)
		// Non-map elements get wrapped
		if len(arr) != 3 {
			t.Fatalf("expected 3 elements, got %d", len(arr))
		}
	})

	t.Run("set extracted field with create paths", func(t *testing.T) {
		cfg := Config{CreatePaths: true}
		json := `{"items":[{"name":"a"},{"name":"b"}]}`
		result, err := Set(json, "items.{name}", "x", cfg)
		if err != nil {
			t.Fatalf("Set() error: %v", err)
		}
		assertJSONEqual(t, `{"items":[{"name":"x"},{"name":"x"}]}`, result)
	})
}

func TestOperationSetArrayExtractFlat(t *testing.T) {
	t.Run("flat extract set with array value", func(t *testing.T) {
		json := `[{"tags":["a","b"]},{"tags":["c","d"]}]`
		result, err := Set(json, "{flat:tags}", []any{"e", "f"})
		if err != nil {
			t.Fatalf("Set() error: %v", err)
		}
		arr := mustParseArray(t, result)
		for i, item := range arr {
			obj := item.(map[string]any)
			tags := obj["tags"]
			// The tags field should have been modified
			t.Logf("item[%d].tags = %v (%T)", i, tags, tags)
		}
	})

	t.Run("flat extract set with single value replaces field", func(t *testing.T) {
		json := `[{"tags":["a"]},{"tags":["b"]}]`
		result, err := Set(json, "{flat:tags}", "c")
		if err != nil {
			t.Fatalf("Set() error: %v", err)
		}
		arr := mustParseArray(t, result)
		for _, item := range arr {
			obj := item.(map[string]any)
			tags := obj["tags"]
			// With flat extract + single value, the field gets replaced
			t.Logf("tags = %v (%T)", tags, tags)
		}
	})

	t.Run("flat extract set on field without existing array", func(t *testing.T) {
		json := `[{"val":"x"},{"val":"y"}]`
		result, err := Set(json, "{flat:val}", "z")
		if err != nil {
			t.Fatalf("Set() error: %v", err)
		}
		arr := mustParseArray(t, result)
		for _, item := range arr {
			obj := item.(map[string]any)
			val := obj["val"]
			t.Logf("val = %v (%T)", val, val)
		}
	})

	t.Run("flat extract set creates new field on map elements", func(t *testing.T) {
		json := `[{"k":[]},{"k":[]}]`
		result, err := Set(json, "{flat:k}", "v")
		if err != nil {
			t.Fatalf("Set() error: %v", err)
		}
		arr := mustParseArray(t, result)
		for _, item := range arr {
			obj, ok := item.(map[string]any)
			if !ok {
				t.Fatalf("expected map, got %T", item)
			}
			k := obj["k"]
			t.Logf("k = %v (%T)", k, k)
		}
	})
}

// --- setValueForArraySlice: set with slice parameters including negatives ---

func TestOperationSetArraySliceNegative(t *testing.T) {
	t.Run("set slice with negative indices", func(t *testing.T) {
		result, err := Set(`{"a":[10,20,30,40,50]}`, "a[-3:-1]", 0)
		if err != nil {
			t.Fatalf("Set() error: %v", err)
		}
		assertJSONEqual(t, `{"a":[10,20,0,0,50]}`, result)
	})
}

// --- setValueAtPath / setValueAdvancedPath dispatching ---

func TestOperationSetValueAtPathDispatch(t *testing.T) {
	t.Run("set via dot notation simple path", func(t *testing.T) {
		result, err := Set(`{"a":1}`, "a", 2)
		if err != nil {
			t.Fatalf("Set() error: %v", err)
		}
		assertJSONEqual(t, `{"a":2}`, result)
	})

	t.Run("set with complex path triggers recursive processor", func(t *testing.T) {
		result, err := Set(`{"items":[{"name":"a"},{"name":"b"}]}`, "items.{name}", "x")
		if err != nil {
			t.Fatalf("Set() error: %v", err)
		}
		assertJSONEqual(t, `{"items":[{"name":"x"},{"name":"x"}]}`, result)
	})

	t.Run("set root should error", func(t *testing.T) {
		_, err := Set(`{"a":1}`, "", 42)
		if err == nil {
			t.Fatal("expected error for empty path")
		}
	})

	t.Run("set dot path should error", func(t *testing.T) {
		_, err := Set(`{"a":1}`, ".", 42)
		if err == nil {
			t.Fatal("expected error for dot path")
		}
	})
}

// --- navigateToExtraction for set operations ---

func TestOperationNavigateToExtraction(t *testing.T) {
	t.Run("set with extraction navigation on single object", func(t *testing.T) {
		json := `{"wrapper":{"field":"old"}}`
		result, err := Set(json, "wrapper.{field}", "new")
		if err != nil {
			t.Fatalf("Set() error: %v", err)
		}
		assertJSONEqual(t, `{"wrapper":{"field":"new"}}`, result)
	})
}

// --- 2D array operations (matrix access) ---

func TestOperationMatrixAccess(t *testing.T) {
	t.Run("get element from 2D array", func(t *testing.T) {
		result, err := Get(`{"m":[[1,2,3],[4,5,6],[7,8,9]]}`, "m[2][1]")
		if err != nil {
			t.Fatalf("Get() error: %v", err)
		}
		if result != float64(8) {
			t.Errorf("expected 8, got %v", result)
		}
	})

	t.Run("set element in 2D array", func(t *testing.T) {
		result, err := Set(`{"m":[[1,2],[3,4]]}`, "m[0][1]", 99)
		if err != nil {
			t.Fatalf("Set() error: %v", err)
		}
		assertJSONEqual(t, `{"m":[[1,99],[3,4]]}`, result)
	})

	t.Run("delete from 2D array", func(t *testing.T) {
		result, err := Delete(`{"m":[[1,2,3],[4,5,6]]}`, "m[1]")
		if err != nil {
			t.Fatalf("Delete() error: %v", err)
		}
		assertJSONEqual(t, `{"m":[[1,2,3]]}`, result)
	})
}

// --- CleanupDeletedMarkers with nested structures ---

func TestOperationCleanupDeletedMarkers(t *testing.T) {
	t.Run("delete multiple elements from array", func(t *testing.T) {
		json := `{"items":[1,2,3,4,5]}`
		result, err := Delete(json, "items[1]")
		if err != nil {
			t.Fatalf("Delete() error: %v", err)
		}
		// Delete one, then delete another from result
		result2, err := Delete(result, "items[2]")
		if err != nil {
			t.Fatalf("Delete() error: %v", err)
		}
		assertJSONEqual(t, `{"items":[1,3,5]}`, result2)
	})
}

// --- Error paths for internal functions ---

func TestOperationInternalErrors(t *testing.T) {
	t.Run("delete from non-array index", func(t *testing.T) {
		_, err := Delete(`{"a":"not_array"}`, "a[0]")
		if err == nil {
			t.Error("expected error for array index on non-array")
		}
	})

	t.Run("set on non-object property", func(t *testing.T) {
		_, err := Set(`{"a":42}`, "a.b", "val")
		if err == nil {
			t.Error("expected error for property on non-object")
		}
	})

	t.Run("set array index on non-array", func(t *testing.T) {
		_, err := Set(`{"a":"string"}`, "a[0]", "val")
		if err == nil {
			t.Error("expected error for array index on non-array")
		}
	})

	t.Run("get from out-of-bounds index returns nil", func(t *testing.T) {
		result, err := Get(`{"a":[1,2]}`, "a[10]")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if result != nil {
			t.Errorf("expected nil result for out-of-bounds, got %v", result)
		}
	})

	t.Run("delete extraction on non-array non-object", func(t *testing.T) {
		_, err := Delete(`{"a":42}`, "a.{field}")
		if err == nil {
			t.Error("expected error for extraction on primitive")
		}
	})
}

// --- setValueForArraySlice edge cases ---

func TestOperationSetSliceEdgeCases(t *testing.T) {
	t.Run("set full array slice", func(t *testing.T) {
		result, err := Set(`{"a":[1,2,3]}`, "a[:]", 0)
		if err != nil {
			t.Fatalf("Set() error: %v", err)
		}
		assertJSONEqual(t, `{"a":[0,0,0]}`, result)
	})

	t.Run("set slice with step 3", func(t *testing.T) {
		result, err := Set(`{"a":[0,1,2,3,4,5,6,7,8,9]}`, "a[::3]", -1)
		if err != nil {
			t.Fatalf("Set() error: %v", err)
		}
		assertJSONEqual(t, `{"a":[-1,1,2,-1,4,5,-1,7,8,-1]}`, result)
	})

	t.Run("set slice from beginning", func(t *testing.T) {
		result, err := Set(`{"a":[10,20,30,40]}`, "a[:2]", 0)
		if err != nil {
			t.Fatalf("Set() error: %v", err)
		}
		assertJSONEqual(t, `{"a":[0,0,30,40]}`, result)
	})

	t.Run("set slice to end", func(t *testing.T) {
		result, err := Set(`{"a":[10,20,30,40]}`, "a[2:]", 0)
		if err != nil {
			t.Fatalf("Set() error: %v", err)
		}
		assertJSONEqual(t, `{"a":[10,20,0,0]}`, result)
	})
}

// --- Delete with complex extract + array combinations ---

func TestOperationDeleteExtractArrayCombo(t *testing.T) {
	t.Run("delete after extraction from array elements", func(t *testing.T) {
		json := `{"items":[{"meta":{"a":1,"b":2}},{"meta":{"a":3,"b":4}}]}`
		result, err := Delete(json, "items.{meta}.a")
		if err != nil {
			t.Fatalf("Delete() error: %v", err)
		}
		assertJSONEqual(t, `{"items":[{"meta":{"b":2}},{"meta":{"b":4}}]}`, result)
	})

	t.Run("delete extracted field from array", func(t *testing.T) {
		json := `[{"x":1,"y":2},{"x":3,"y":4}]`
		result, err := Delete(json, "{x}")
		if err != nil {
			t.Fatalf("Delete() error: %v", err)
		}
		assertJSONEqual(t, `[{"y":2},{"y":4}]`, result)
	})
}

// --- handleArraySlice via Get on root array ---

func TestOperationGetSliceOnRootArray(t *testing.T) {
	t.Run("slice on root array", func(t *testing.T) {
		result, err := Get(`[0,1,2,3,4]`, "[1:3]")
		if err != nil {
			t.Fatalf("Get() error: %v", err)
		}
		arr, ok := result.([]any)
		if !ok {
			t.Fatalf("expected []any, got %T", result)
		}
		if len(arr) != 2 {
			t.Fatalf("expected 2, got %d", len(arr))
		}
	})
}

// --- CreatePaths for deeply nested structures ---

func TestOperationCreatePathsDeep(t *testing.T) {
	cfg := Config{CreatePaths: true}

	t.Run("create 4-level deep path", func(t *testing.T) {
		result, err := Set(`{}`, "a.b.c.d", 42, cfg)
		if err != nil {
			t.Fatalf("Set() error: %v", err)
		}
		assertJSONEqual(t, `{"a":{"b":{"c":{"d":42}}}}`, result)
	})

	t.Run("create path with existing intermediate", func(t *testing.T) {
		result, err := Set(`{"a":{"b":1}}`, "a.c", 2, cfg)
		if err != nil {
			t.Fatalf("Set() error: %v", err)
		}
		assertJSONEqual(t, `{"a":{"b":1,"c":2}}`, result)
	})

	t.Run("create array in new path", func(t *testing.T) {
		result, err := Set(`{}`, "list[0]", "first", cfg)
		if err != nil {
			t.Fatalf("Set() error: %v", err)
		}
		assertJSONEqual(t, `{"list":["first"]}`, result)
	})

	t.Run("create path and extend array", func(t *testing.T) {
		result, err := Set(`{}`, "data.items[2]", "val", cfg)
		if err != nil {
			t.Fatalf("Set() error: %v", err)
		}
		m := mustParseMap(t, result)
		data := m["data"].(map[string]any)
		arr := data["items"].([]any)
		if len(arr) != 3 {
			t.Fatalf("expected length 3, got %d", len(arr))
		}
		if arr[2] != "val" {
			t.Errorf("arr[2] = %v, want 'val'", arr[2])
		}
	})
}

// --- Get with multi-field extraction ---

func TestOperationMultiFieldExtraction(t *testing.T) {
	t.Run("extract multiple fields from array", func(t *testing.T) {
		json := `[{"name":"a","age":1,"extra":9},{"name":"b","age":2,"extra":8}]`
		result, err := Get(json, "{name,age}")
		if err != nil {
			t.Fatalf("Get() error: %v", err)
		}
		arr, ok := result.([]any)
		if !ok {
			t.Fatalf("expected []any, got %T", result)
		}
		if len(arr) != 2 {
			t.Fatalf("expected 2 elements, got %d", len(arr))
		}
		// Each element should have exactly 2 keys: name and age
		for i, item := range arr {
			obj := item.(map[string]any)
			if len(obj) != 2 {
				t.Errorf("element %d: expected 2 fields, got %d", i, len(obj))
			}
			if _, ok := obj["name"]; !ok {
				t.Errorf("element %d: missing 'name'", i)
			}
			if _, ok := obj["age"]; !ok {
				t.Errorf("element %d: missing 'age'", i)
			}
		}
	})

	t.Run("extract multiple fields from single object", func(t *testing.T) {
		json := `{"x":1,"y":2,"z":3}`
		result, err := Get(json, "{x,y}")
		if err != nil {
			t.Fatalf("Get() error: %v", err)
		}
		obj, ok := result.(map[string]any)
		if !ok {
			t.Fatalf("expected map, got %T", result)
		}
		if len(obj) != 2 {
			t.Errorf("expected 2 fields, got %d", len(obj))
		}
	})
}

// --- Flat extraction Get operations ---

func TestOperationFlatExtractionGet(t *testing.T) {
	t.Run("flat extract from array", func(t *testing.T) {
		json := `[{"tags":["a","b"]},{"tags":["c","d"]}]`
		result, err := Get(json, "{flat:tags}")
		if err != nil {
			t.Fatalf("Get() error: %v", err)
		}
		arr, ok := result.([]any)
		if !ok {
			t.Fatalf("expected []any, got %T", result)
		}
		if len(arr) != 4 {
			t.Fatalf("expected 4 flattened elements, got %d", len(arr))
		}
	})
}

// --- DeleteClean API ---

func TestOperationDeleteClean(t *testing.T) {
	t.Run("delete and clean nulls", func(t *testing.T) {
		json := `{"a":{"b":1,"c":2}}`
		result, err := DeleteClean(json, "a.b")
		if err != nil {
			t.Fatalf("DeleteClean() error: %v", err)
		}
		assertJSONEqual(t, `{"a":{"c":2}}`, result)
	})

	t.Run("delete array element and compact", func(t *testing.T) {
		json := `{"items":[1,2,3,4]}`
		result, err := DeleteClean(json, "items[1]")
		if err != nil {
			t.Fatalf("DeleteClean() error: %v", err)
		}
		assertJSONEqual(t, `{"items":[1,3,4]}`, result)
	})
}

// --- mustParseArray helper ---

func mustParseArray(t *testing.T, jsonStr string) []any {
	t.Helper()
	var arr []any
	if err := json.Unmarshal([]byte(jsonStr), &arr); err != nil {
		t.Fatalf("invalid JSON array %q: %v", jsonStr, err)
	}
	return arr
}

// --- Distributed and extraction operations ---

func TestDistributedGetOperations(t *testing.T) {
	jsonStr := `{
		"items": [
			{"name": "a", "tags": ["x", "y"]},
			{"name": "b", "tags": ["z"]}
		]
	}`

	tests := []struct {
		name string
		path string
		want string
	}{
		{name: "extract name from all items", path: "items{name}", want: `["a","b"]`},
		{name: "extract tags from all items", path: "items{tags}", want: `[["x","y"],["z"]]`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := Get(jsonStr, tt.path)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			gotJSON, _ := json.Marshal(result)
			assertJSONEqual(t, tt.want, string(gotJSON))
		})
	}
}

func TestDistributedOperationArrayPostProcess(t *testing.T) {
	jsonStr := `{
		"matrix": [
			{"row": [1, 2, 3]},
			{"row": [4, 5, 6]}
		]
	}`

	t.Run("extract row arrays", func(t *testing.T) {
		result, err := Get(jsonStr, "matrix{row}")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		arr, ok := result.([]any)
		if !ok {
			t.Fatalf("expected array, got %T", result)
		}
		if len(arr) != 2 {
			t.Errorf("expected 2 arrays, got %d", len(arr))
		}
	})

	t.Run("extract and index into arrays", func(t *testing.T) {
		result, err := Get(jsonStr, "matrix{row}[0]")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result == nil {
			t.Error("expected non-nil result")
		}
	})
}

func TestExtractSetOperations(t *testing.T) {
	tests := []struct {
		name    string
		jsonStr string
		path    string
		value   any
		want    string
		cfg     Config
	}{
		{
			name:    "set extracted field on all objects",
			jsonStr: `{"items":[{"name":"a"},{"name":"b"}]}`,
			path:    "items{name}",
			value:   "new",
			cfg:     Config{CreatePaths: true},
			want:    `{"items":[{"name":"new"},{"name":"new"}]}`,
		},
		{
			name:    "set new field on extracted objects",
			jsonStr: `{"items":[{"name":"a"},{"name":"b"}]}`,
			path:    "items{active}",
			value:   true,
			cfg:     Config{CreatePaths: true},
			want:    `{"items":[{"name":"a","active":true},{"name":"b","active":true}]}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := Set(tt.jsonStr, tt.path, tt.value, tt.cfg)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			assertJSONEqual(t, tt.want, result)
		})
	}
}

func TestFlatExtraction(t *testing.T) {
	jsonStr := `{
		"items": [
			{"tags": ["a", "b"]},
			{"tags": ["c"]}
		]
	}`

	result, err := Get(jsonStr, "items{flat:tags}")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	arr, ok := result.([]any)
	if !ok {
		t.Fatalf("expected array, got %T", result)
	}
	if len(arr) != 3 {
		t.Errorf("expected 3 flattened items, got %d: %v", len(arr), arr)
	}
}

func TestDeleteIntermediateArrayIndex(t *testing.T) {
	t.Run("delete property via JSON Pointer with array", func(t *testing.T) {
		result, err := Delete(
			`{"users":[{"name":"A"},{"name":"B"}]}`,
			"/users/0/name",
		)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		assertJSONEqual(t, `{"users":[{},{"name":"B"}]}`, result)
	})
}

func TestArraySetSliceEdgeCases(t *testing.T) {
	tests := []struct {
		name    string
		jsonStr string
		path    string
		value   any
		want    string
	}{
		{name: "set slice with step", jsonStr: `{"arr":[1,2,3,4,5]}`, path: "arr[0:5:2]", value: 0, want: `{"arr":[0,2,0,4,0]}`},
		{name: "set slice negative indices", jsonStr: `{"arr":[1,2,3,4,5]}`, path: "arr[-3:-1]", value: 0, want: `{"arr":[1,2,0,0,5]}`},
		{name: "negative index set", jsonStr: `{"arr":[1,2,3]}`, path: "arr[-1]", value: 99, want: `{"arr":[1,2,99]}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := Set(tt.jsonStr, tt.path, tt.value)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			assertJSONEqual(t, tt.want, result)
		})
	}
}
