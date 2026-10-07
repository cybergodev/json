package json

import (
	"errors"
	"testing"
)

// helperRP creates a Processor and its recursiveProcessor, returning both.
// The caller must defer processor.Close().
func helperRP(t *testing.T) (*Processor, *recursiveProcessor) {
	t.Helper()
	processor, err := New()
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	return processor, newRecursiveProcessor(processor)
}

// ============================================================================
// newRecursiveProcessor
// ============================================================================

func TestNewRecursiveProcessor(t *testing.T) {
	processor, err := New()
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	defer processor.Close()

	rp := newRecursiveProcessor(processor)
	if rp == nil {
		t.Fatal("newRecursiveProcessor returned nil")
	}
	if rp.provider == nil {
		t.Fatal("newRecursiveProcessor: provider should not be nil")
	}
}

// ============================================================================
// ProcessRecursively — Get operation (table-driven)
// ============================================================================

func TestRecursiveProcessor_GetOperation_Table(t *testing.T) {
	processor, rp := helperRP(t)
	defer processor.Close()

	tests := []struct {
		name    string
		data    any
		path    string
		want    any
		wantErr bool
	}{
		{
			name: "simple property",
			data: map[string]any{"name": "Alice"},
			path: "name",
			want: "Alice",
		},
		{
			name: "nested property two levels",
			data: map[string]any{
				"user": map[string]any{"name": "Bob"},
			},
			path: "user.name",
			want: "Bob",
		},
		{
			name: "nested property three levels",
			data: map[string]any{
				"a": map[string]any{
					"b": map[string]any{
						"c": "deep",
					},
				},
			},
			path: "a.b.c",
			want: "deep",
		},
		{
			name: "array index",
			data: []any{10, 20, 30},
			path: "[1]",
			want: 20,
		},
		{
			name: "negative array index",
			data: []any{10, 20, 30},
			path: "[-1]",
			want: 30,
		},
		{
			name: "array slice full range",
			data: []any{0, 1, 2, 3, 4},
			path: "[1:3]",
			want: []any{1, 2},
		},
		{
			name: "array slice from start",
			data: []any{0, 1, 2, 3, 4},
			path: "[:2]",
			want: []any{0, 1},
		},
		{
			name: "array slice to end",
			data: []any{0, 1, 2, 3, 4},
			path: "[3:]",
			want: []any{3, 4},
		},
		{
			name: "object in array",
			data: map[string]any{
				"items": []any{
					map[string]any{"id": 1, "name": "first"},
					map[string]any{"id": 2, "name": "second"},
				},
			},
			path: "items[0].name",
			want: "first",
		},
		{
			name: "nested array 2D",
			data: map[string]any{
				"matrix": []any{
					[]any{1, 2, 3},
					[]any{4, 5, 6},
				},
			},
			path: "matrix[1][2]",
			want: 6,
		},
		{
			name: "empty path returns root",
			data: map[string]any{"key": "value"},
			path: "",
			want: map[string]any{"key": "value"},
		},
		{
			name: "extract single field from array",
			data: map[string]any{
				"users": []any{
					map[string]any{"id": 1, "name": "Alice"},
					map[string]any{"id": 2, "name": "Bob"},
				},
			},
			path: "users{name}",
			want: []any{"Alice", "Bob"},
		},
		{
			name: "extract from single object",
			data: map[string]any{
				"config": map[string]any{"host": "localhost", "port": 8080},
			},
			path: "config{host}",
			want: "localhost",
		},
		{
			name:    "nonexistent property",
			data:    map[string]any{"key": "value"},
			path:    "missing",
			wantErr: true,
		},
		{
			name:    "deep nonexistent path",
			data:    map[string]any{"a": map[string]any{}},
			path:    "a.b.c",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := rp.ProcessRecursively(tt.data, tt.path, opGet, nil)
			if (err != nil) != tt.wantErr {
				t.Errorf("ProcessRecursively() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}
			compareResults(t, tt.want, result)
		})
	}
}

// ============================================================================
// ProcessRecursively — Set operation (table-driven)
// ============================================================================

func TestRecursiveProcessor_SetOperation_Table(t *testing.T) {
	processor, rp := helperRP(t)
	defer processor.Close()

	tests := []struct {
		name           string
		data           any
		path           string
		value          any
		wantErr        bool
		validate       func(t *testing.T, data any)
		validateResult func(t *testing.T, result any)
	}{
		{
			name:  "set simple property",
			data:  map[string]any{"name": "old"},
			path:  "name",
			value: "new",
			validate: func(t *testing.T, data any) {
				t.Helper()
				m := data.(map[string]any)
				if m["name"] != "new" {
					t.Errorf("name = %v, want new", m["name"])
				}
			},
		},
		{
			name: "set nested property",
			data: map[string]any{
				"user": map[string]any{"age": 25},
			},
			path:  "user.age",
			value: 30,
			validate: func(t *testing.T, data any) {
				t.Helper()
				m := data.(map[string]any)
				inner := m["user"].(map[string]any)
				if inner["age"] != 30 {
					t.Errorf("age = %v, want 30", inner["age"])
				}
			},
		},
		{
			name:  "set array element",
			data:  map[string]any{"arr": []any{1, 2, 3}},
			path:  "arr[1]",
			value: 99,
			validate: func(t *testing.T, data any) {
				t.Helper()
				m := data.(map[string]any)
				arr := m["arr"].([]any)
				if arr[1] != 99 {
					t.Errorf("arr[1] = %v, want 99", arr[1])
				}
			},
		},
		{
			// GEN-001: root Set replaces the document; the new root arrives as
			// the operation result (the in-place data cannot express it).
			name:  "set root replaces document",
			data:  map[string]any{"key": "value"},
			path:  "",
			value: "newroot",
			validateResult: func(t *testing.T, result any) {
				t.Helper()
				if result != "newroot" {
					t.Errorf("root set result = %v, want newroot", result)
				}
			},
		},
		{
			name:    "set nonexistent path without createPaths",
			data:    map[string]any{},
			path:    "a.b.c",
			value:   "value",
			wantErr: true,
		},
		{
			name: "set extract field on array elements",
			data: map[string]any{
				"items": []any{
					map[string]any{"name": "a", "val": 1},
					map[string]any{"name": "b", "val": 2},
				},
			},
			path:  "items{name}",
			value: "replaced",
			validate: func(t *testing.T, data any) {
				t.Helper()
				m := data.(map[string]any)
				arr := m["items"].([]any)
				for _, item := range arr {
					im := item.(map[string]any)
					if im["name"] != "replaced" {
						t.Errorf("name = %v, want replaced", im["name"])
					}
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := rp.ProcessRecursively(tt.data, tt.path, opSet, tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("ProcessRecursively() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.validate != nil && err == nil {
				tt.validate(t, tt.data)
			}
			if tt.validateResult != nil && err == nil {
				tt.validateResult(t, result)
			}
		})
	}
}

// ============================================================================
// ProcessRecursively — Delete operation (table-driven)
// ============================================================================

func TestRecursiveProcessor_DeleteOperation_Table(t *testing.T) {
	processor, rp := helperRP(t)
	defer processor.Close()

	tests := []struct {
		name     string
		data     any
		path     string
		wantErr  bool
		validate func(t *testing.T, data any)
	}{
		{
			name: "delete simple property",
			data: map[string]any{"name": "Alice", "age": 30},
			path: "name",
			validate: func(t *testing.T, data any) {
				t.Helper()
				m := data.(map[string]any)
				if _, exists := m["name"]; exists {
					t.Error("name should have been deleted")
				}
				if m["age"] != 30 {
					t.Error("age should still exist")
				}
			},
		},
		{
			name: "delete nested property",
			data: map[string]any{
				"user": map[string]any{"name": "Alice", "email": "a@b.com"},
			},
			path: "user.email",
			validate: func(t *testing.T, data any) {
				t.Helper()
				m := data.(map[string]any)
				inner := m["user"].(map[string]any)
				if _, exists := inner["email"]; exists {
					t.Error("email should have been deleted")
				}
				if inner["name"] != "Alice" {
					t.Error("name should still exist")
				}
			},
		},
		{
			name: "delete array element by marking",
			data: map[string]any{"arr": []any{1, 2, 3}},
			path: "arr[1]",
			validate: func(t *testing.T, data any) {
				t.Helper()
				m := data.(map[string]any)
				arr := m["arr"].([]any)
				if !isDeletedMarker(arr[1]) {
					t.Error("arr[1] should be marked as deleted")
				}
			},
		},
		{
			name:    "delete root should fail",
			data:    map[string]any{"key": "value"},
			path:    "",
			wantErr: true,
		},
		{
			name: "delete nonexistent property returns nil",
			data: map[string]any{"key": "value"},
			path: "nonexistent",
			validate: func(t *testing.T, data any) {
				t.Helper()
				m := data.(map[string]any)
				if m["key"] != "value" {
					t.Error("existing key should still exist")
				}
			},
		},
		{
			name: "delete via slice on array",
			data: map[string]any{"arr": []any{10, 20, 30, 40, 50}},
			path: "arr[1:3]",
			validate: func(t *testing.T, data any) {
				t.Helper()
				m := data.(map[string]any)
				arr := m["arr"].([]any)
				if !isDeletedMarker(arr[1]) || !isDeletedMarker(arr[2]) {
					t.Error("arr[1] and arr[2] should be marked as deleted")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := rp.ProcessRecursively(tt.data, tt.path, opDelete, nil)
			if (err != nil) != tt.wantErr {
				t.Errorf("ProcessRecursively() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.validate != nil && err == nil {
				tt.validate(t, tt.data)
			}
		})
	}
}

// ============================================================================
// ProcessRecursivelyWithOptions — CreatePaths (table-driven)
// ============================================================================

func TestRecursiveProcessor_CreatePaths_Table(t *testing.T) {
	processor, rp := helperRP(t)
	defer processor.Close()

	tests := []struct {
		name     string
		data     any
		path     string
		value    any
		wantErr  bool
		validate func(t *testing.T, data any)
	}{
		{
			name:  "create nested map path",
			data:  map[string]any{},
			path:  "a.b.c",
			value: "created",
			validate: func(t *testing.T, data any) {
				t.Helper()
				m := data.(map[string]any)
				a := m["a"].(map[string]any)
				b := a["b"].(map[string]any)
				if b["c"] != "created" {
					t.Errorf("b[c] = %v, want created", b["c"])
				}
			},
		},
		{
			name:  "create path with array index creates array",
			data:  map[string]any{},
			path:  "items[0]",
			value: "first",
			validate: func(t *testing.T, data any) {
				t.Helper()
				m := data.(map[string]any)
				arr, ok := m["items"].([]any)
				if !ok {
					t.Fatal("items should be an array")
				}
				if len(arr) < 1 {
					t.Fatal("items should have at least 1 element")
				}
				if arr[0] != "first" {
					t.Errorf("items[0] = %v, want first", arr[0])
				}
			},
		},
		{
			name:  "set existing path with createPaths true",
			data:  map[string]any{"key": "old"},
			path:  "key",
			value: "new",
			validate: func(t *testing.T, data any) {
				t.Helper()
				m := data.(map[string]any)
				if m["key"] != "new" {
					t.Errorf("key = %v, want new", m["key"])
				}
			},
		},
		{
			name:    "out-of-range slice errors explicitly instead of silent clamp",
			data:    map[string]any{"arr": []any{1}},
			path:    "arr[0:5]",
			value:   99,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := rp.ProcessRecursivelyWithOptions(tt.data, tt.path, opSet, tt.value, true)
			if (err != nil) != tt.wantErr {
				t.Errorf("ProcessRecursivelyWithOptions() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.validate != nil && err == nil {
				tt.validate(t, tt.data)
			}
		})
	}
}

// TestRecursiveProcessor_SliceSet_HonorsStep is a regression test for a bug where
// the opSet branch of handleArraySliceSegmentUnified iterated with i++ and ignored
// the slice step, diverging from opDelete (which honored step) and from the
// dot-notation path. With the fix, arr[0:5:2]=v must mutate only indices 0,2,4.
func TestRecursiveProcessor_SliceSet_HonorsStep(t *testing.T) {
	processor, rp := helperRP(t)
	defer processor.Close()

	data := map[string]any{"arr": []any{1, 2, 3, 4, 5}}
	if _, err := rp.ProcessRecursivelyWithOptions(data, "arr[0:5:2]", opSet, 0, false); err != nil {
		t.Fatalf("ProcessRecursivelyWithOptions() unexpected error: %v", err)
	}

	arr := data["arr"].([]any)
	want := []any{0, 2, 0, 4, 0}
	if len(arr) != len(want) {
		t.Fatalf("arr len = %d, want %d (%v)", len(arr), len(want), arr)
	}
	for i, v := range want {
		if arr[i] != v {
			t.Errorf("arr[%d] = %v, want %v (full: %v)", i, arr[i], v, arr)
		}
	}
}

// ============================================================================
// Wildcard segment operations
// ============================================================================

func TestRecursiveProcessor_Wildcard_Table(t *testing.T) {
	processor, rp := helperRP(t)
	defer processor.Close()

	t.Run("get all array elements via wildcard", func(t *testing.T) {
		data := map[string]any{
			"items": []any{10, 20, 30},
		}
		result, err := rp.ProcessRecursively(data, "items[*]", opGet, nil)
		if err != nil {
			t.Fatalf("error: %v", err)
		}
		arr, ok := result.([]any)
		if !ok {
			t.Fatalf("expected []any, got %T", result)
		}
		if len(arr) != 3 {
			t.Errorf("len = %d, want 3", len(arr))
		}
	})

	t.Run("set all array elements via wildcard", func(t *testing.T) {
		data := map[string]any{
			"items": []any{1, 2, 3},
		}
		_, err := rp.ProcessRecursively(data, "items[*]", opSet, 99)
		if err != nil {
			t.Fatalf("error: %v", err)
		}
		arr := data["items"].([]any)
		for i, v := range arr {
			if v != 99 {
				t.Errorf("items[%d] = %v, want 99", i, v)
			}
		}
	})
}

// ============================================================================
// Extract segment — multi-field extraction
// ============================================================================

func TestRecursiveProcessor_MultiFieldExtract(t *testing.T) {
	processor, rp := helperRP(t)
	defer processor.Close()

	t.Run("extract multiple fields from array", func(t *testing.T) {
		data := map[string]any{
			"users": []any{
				map[string]any{"id": 1, "name": "Alice", "email": "a@b.com"},
				map[string]any{"id": 2, "name": "Bob", "email": "b@c.com"},
			},
		}

		result, err := rp.ProcessRecursively(data, "users{id,name}", opGet, nil)
		if err != nil {
			t.Fatalf("error: %v", err)
		}
		arr, ok := result.([]any)
		if !ok {
			t.Fatalf("expected []any, got %T", result)
		}
		if len(arr) != 2 {
			t.Fatalf("len = %d, want 2", len(arr))
		}
		first := arr[0].(map[string]any)
		if first["name"] != "Alice" || first["id"] == nil {
			t.Errorf("first element = %v, want id and name", first)
		}
		if _, hasEmail := first["email"]; hasEmail {
			t.Error("email should not be extracted")
		}
	})

	t.Run("extract multiple fields from single object", func(t *testing.T) {
		data := map[string]any{
			"config": map[string]any{"host": "localhost", "port": 8080, "debug": true},
		}

		result, err := rp.ProcessRecursively(data, "config{host,port}", opGet, nil)
		if err != nil {
			t.Fatalf("error: %v", err)
		}
		m, ok := result.(map[string]any)
		if !ok {
			t.Fatalf("expected map[string]any, got %T", result)
		}
		if m["host"] != "localhost" {
			t.Errorf("host = %v, want localhost", m["host"])
		}
		if m["port"] != 8080 {
			t.Errorf("port = %v, want 8080", m["port"])
		}
		if _, hasDebug := m["debug"]; hasDebug {
			t.Error("debug should not be extracted")
		}
	})

	t.Run("extract with nonexistent fields returns nil map", func(t *testing.T) {
		data := map[string]any{
			"items": []any{
				map[string]any{"id": 1},
			},
		}
		result, err := rp.ProcessRecursively(data, "items{nonexistent}", opGet, nil)
		if err != nil {
			t.Fatalf("error: %v", err)
		}
		arr, ok := result.([]any)
		if !ok {
			t.Fatalf("expected []any, got %T", result)
		}
		if len(arr) != 0 {
			t.Errorf("len = %d, want 0 for nonexistent field", len(arr))
		}
	})
}

// ============================================================================
// Flat extract
// ============================================================================

func TestRecursiveProcessor_FlatExtract(t *testing.T) {
	processor, rp := helperRP(t)
	defer processor.Close()

	t.Run("flat extract from array of arrays", func(t *testing.T) {
		data := map[string]any{
			"groups": []any{
				map[string]any{"tags": []any{"a", "b"}},
				map[string]any{"tags": []any{"c", "d"}},
			},
		}

		result, err := rp.ProcessRecursively(data, "groups{flat:tags}", opGet, nil)
		if err != nil {
			t.Fatalf("error: %v", err)
		}
		arr, ok := result.([]any)
		if !ok {
			t.Fatalf("expected []any, got %T", result)
		}
		if len(arr) != 4 {
			t.Errorf("len = %d, want 4 (flattened)", len(arr))
		}
	})

	t.Run("flat extract on single object with array", func(t *testing.T) {
		data := map[string]any{
			"config": map[string]any{"items": []any{1, 2, 3}},
		}

		result, err := rp.ProcessRecursively(data, "config{flat:items}", opGet, nil)
		if err != nil {
			t.Fatalf("error: %v", err)
		}
		arr, ok := result.([]any)
		if !ok {
			t.Fatalf("expected []any, got %T", result)
		}
		if len(arr) != 3 {
			t.Errorf("len = %d, want 3", len(arr))
		}
	})
}

// ============================================================================
// Nil input handling
// ============================================================================

func TestRecursiveProcessor_NilInput(t *testing.T) {
	processor, rp := helperRP(t)
	defer processor.Close()

	t.Run("nil data with get", func(t *testing.T) {
		result, err := rp.ProcessRecursively(nil, "path", opGet, nil)
		if err != nil {
			t.Errorf("get on nil data: unexpected error %v", err)
		}
		if result != nil {
			t.Errorf("get on nil data: result = %v, want nil", result)
		}
	})

	t.Run("nil data with set", func(t *testing.T) {
		_, err := rp.ProcessRecursively(nil, "path", opSet, "value")
		if err == nil {
			t.Error("expected error for set on nil data")
		}
	})

	t.Run("nil data with delete", func(t *testing.T) {
		_, err := rp.ProcessRecursively(nil, "path", opDelete, nil)
		if err == nil {
			t.Error("expected error for delete on nil data")
		}
	})
}

// ============================================================================
// Mixed type nesting (maps containing slices containing maps)
// ============================================================================

func TestRecursiveProcessor_MixedTypeNesting(t *testing.T) {
	processor, rp := helperRP(t)
	defer processor.Close()

	t.Run("get from deeply mixed structure", func(t *testing.T) {
		data := map[string]any{
			"users": []any{
				map[string]any{
					"name": "Alice",
					"addresses": []any{
						map[string]any{"city": "NYC", "zip": "10001"},
						map[string]any{"city": "LA", "zip": "90001"},
					},
				},
				map[string]any{
					"name": "Bob",
					"addresses": []any{
						map[string]any{"city": "Chicago", "zip": "60601"},
					},
				},
			},
		}

		// Get nested value through array -> object -> array -> object
		result, err := rp.ProcessRecursively(data, "users[0].addresses[1].city", opGet, nil)
		if err != nil {
			t.Fatalf("error: %v", err)
		}
		if result != "LA" {
			t.Errorf("result = %v, want LA", result)
		}
	})

	t.Run("set in mixed structure", func(t *testing.T) {
		data := map[string]any{
			"teams": []any{
				map[string]any{
					"members": []any{
						map[string]any{"name": "dev1", "role": "engineer"},
						map[string]any{"name": "dev2", "role": "manager"},
					},
				},
			},
		}

		_, err := rp.ProcessRecursively(data, "teams[0].members[1].role", opSet, "director")
		if err != nil {
			t.Fatalf("error: %v", err)
		}

		members := data["teams"].([]any)[0].(map[string]any)["members"].([]any)
		if members[1].(map[string]any)["role"] != "director" {
			t.Error("role should be updated to director")
		}
	})

	t.Run("delete from mixed structure", func(t *testing.T) {
		data := map[string]any{
			"data": []any{
				map[string]any{
					"items": []any{"a", "b", "c"},
				},
			},
		}

		_, err := rp.ProcessRecursively(data, "data[0].items[1]", opDelete, nil)
		if err != nil {
			t.Fatalf("error: %v", err)
		}

		items := data["data"].([]any)[0].(map[string]any)["items"].([]any)
		if !isDeletedMarker(items[1]) {
			t.Error("items[1] should be marked as deleted")
		}
	})

	t.Run("extract from mixed nested structure", func(t *testing.T) {
		data := map[string]any{
			"departments": []any{
				map[string]any{
					"teams": []any{
						map[string]any{"name": "Alpha", "lead": "Alice"},
						map[string]any{"name": "Beta", "lead": "Bob"},
					},
				},
			},
		}

		result, err := rp.ProcessRecursively(data, "departments.teams{name}", opGet, nil)
		if err != nil {
			t.Fatalf("error: %v", err)
		}
		if result == nil {
			t.Error("expected non-nil result")
		}
	})
}

// ============================================================================
// Empty objects and arrays
// ============================================================================

func TestRecursiveProcessor_EmptyContainers(t *testing.T) {
	processor, rp := helperRP(t)
	defer processor.Close()

	tests := []struct {
		name    string
		data    any
		path    string
		op      operation
		value   any
		wantErr bool
		check   func(t *testing.T, result any)
	}{
		{
			name:    "get from empty map returns error",
			data:    map[string]any{},
			path:    "key",
			op:      opGet,
			wantErr: true,
		},
		{
			name: "get from empty array",
			data: []any{},
			path: "[0]",
			op:   opGet,
			check: func(t *testing.T, result any) {
				t.Helper()
				if result != nil {
					t.Errorf("expected nil for out of bounds, got %v", result)
				}
			},
		},
		{
			name: "slice empty array",
			data: []any{},
			path: "[0:3]",
			op:   opGet,
			check: func(t *testing.T, result any) {
				t.Helper()
				arr, ok := result.([]any)
				if !ok {
					t.Fatalf("expected []any, got %T", result)
				}
				if len(arr) != 0 {
					t.Errorf("len = %d, want 0", len(arr))
				}
			},
		},
		{
			name:  "set on empty map",
			data:  map[string]any{},
			path:  "newkey",
			op:    opSet,
			value: "val",
			check: func(t *testing.T, result any) {
				t.Helper()
				if result != "val" {
					t.Errorf("result = %v, want val", result)
				}
			},
		},
		{
			name: "extract from empty array",
			data: map[string]any{"items": []any{}},
			path: "items{name}",
			op:   opGet,
			check: func(t *testing.T, result any) {
				t.Helper()
				arr, ok := result.([]any)
				if !ok {
					t.Fatalf("expected []any, got %T", result)
				}
				if len(arr) != 0 {
					t.Errorf("len = %d, want 0", len(arr))
				}
			},
		},
		{
			name: "delete from empty map",
			data: map[string]any{},
			path: "key",
			op:   opDelete,
			check: func(t *testing.T, result any) {
				t.Helper()
				// Should not error, just no-op
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := rp.ProcessRecursively(tt.data, tt.path, tt.op, tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.check != nil {
				tt.check(t, result)
			}
		})
	}
}

// ============================================================================
// Deep nesting traversal
// ============================================================================

func TestRecursiveProcessor_DeepNestingExtended(t *testing.T) {
	processor, rp := helperRP(t)
	defer processor.Close()

	t.Run("6 levels deep", func(t *testing.T) {
		data := map[string]any{
			"l1": map[string]any{
				"l2": map[string]any{
					"l3": map[string]any{
						"l4": map[string]any{
							"l5": map[string]any{
								"l6": "found",
							},
						},
					},
				},
			},
		}

		result, err := rp.ProcessRecursively(data, "l1.l2.l3.l4.l5.l6", opGet, nil)
		if err != nil {
			t.Fatalf("error: %v", err)
		}
		if result != "found" {
			t.Errorf("result = %v, want found", result)
		}
	})

	t.Run("deep with array nesting", func(t *testing.T) {
		data := map[string]any{
			"level1": []any{
				map[string]any{
					"level2": []any{
						map[string]any{
							"level3": "deep_value",
						},
					},
				},
			},
		}

		result, err := rp.ProcessRecursively(data, "level1[0].level2[0].level3", opGet, nil)
		if err != nil {
			t.Fatalf("error: %v", err)
		}
		if result != "deep_value" {
			t.Errorf("result = %v, want deep_value", result)
		}
	})

	t.Run("deep set and verify", func(t *testing.T) {
		data := map[string]any{
			"a": map[string]any{
				"b": map[string]any{
					"c": map[string]any{
						"d": "old",
					},
				},
			},
		}

		_, err := rp.ProcessRecursively(data, "a.b.c.d", opSet, "new")
		if err != nil {
			t.Fatalf("error: %v", err)
		}

		result, err := rp.ProcessRecursively(data, "a.b.c.d", opGet, nil)
		if err != nil {
			t.Fatalf("error: %v", err)
		}
		if result != "new" {
			t.Errorf("result = %v, want new", result)
		}
	})

	t.Run("deep delete and verify", func(t *testing.T) {
		data := map[string]any{
			"x": map[string]any{
				"y": map[string]any{
					"z": "target",
				},
			},
		}

		_, err := rp.ProcessRecursively(data, "x.y.z", opDelete, nil)
		if err != nil {
			t.Fatalf("error: %v", err)
		}

		inner := data["x"].(map[string]any)["y"].(map[string]any)
		if _, exists := inner["z"]; exists {
			t.Error("z should have been deleted")
		}
	})
}

// ============================================================================
// Array slice with step
// ============================================================================

func TestRecursiveProcessor_SliceWithStep(t *testing.T) {
	processor, rp := helperRP(t)
	defer processor.Close()

	t.Run("slice with step", func(t *testing.T) {
		data := []any{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}
		result, err := rp.ProcessRecursively(data, "[0:10:3]", opGet, nil)
		if err != nil {
			t.Fatalf("error: %v", err)
		}
		arr, ok := result.([]any)
		if !ok {
			t.Fatalf("expected []any, got %T", result)
		}
		// Step 3 from 0..10: indices 0, 3, 6, 9 -> values 0, 3, 6, 9
		if len(arr) != 4 {
			t.Errorf("len = %d, want 4", len(arr))
		}
	})
}

// ============================================================================
// Error paths and type mismatches
// ============================================================================

func TestRecursiveProcessor_ErrorPaths(t *testing.T) {
	processor, rp := helperRP(t)
	defer processor.Close()

	tests := []struct {
		name    string
		data    any
		path    string
		op      operation
		value   any
		wantErr bool
	}{
		{
			name:    "property access on non-container type",
			data:    "string_value",
			path:    "key",
			op:      opGet,
			wantErr: false, // Returns nil for get on non-container
		},
		{
			name:    "array index on string",
			data:    "string_value",
			path:    "[0]",
			op:      opGet,
			wantErr: true,
		},
		{
			name:    "set on non-container type",
			data:    42,
			path:    "key",
			op:      opSet,
			value:   "val",
			wantErr: true,
		},
		{
			name:    "out of bounds positive index",
			data:    []any{1, 2, 3},
			path:    "[100]",
			op:      opGet,
			wantErr: false, // Returns nil for out of bounds get
		},
		{
			name:    "slice on non-array type",
			data:    map[string]any{"key": "value"},
			path:    "[0:1]",
			op:      opGet,
			wantErr: false, // Returns nil for get on non-array
		},
		{
			name:    "extract from non-container",
			data:    42,
			path:    "{field}",
			op:      opGet,
			wantErr: false, // Returns nil
		},
		{
			name:    "set property on array",
			data:    []any{1, 2, 3},
			path:    "key",
			op:      opSet,
			value:   "val",
			wantErr: true,
		},
		{
			name:    "set operation on existing path",
			data:    map[string]any{"key": "val"},
			path:    "key",
			op:      opSet,
			value:   "newval",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := rp.ProcessRecursively(tt.data, tt.path, tt.op, tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// ============================================================================
// Distributed array operations
// ============================================================================

func TestRecursiveProcessor_DistributedArrayOps(t *testing.T) {
	processor, rp := helperRP(t)
	defer processor.Close()

	t.Run("extract then index distributed", func(t *testing.T) {
		data := map[string]any{
			"users": []any{
				map[string]any{"name": "Alice"},
				map[string]any{"name": "Bob"},
			},
		}

		// {name} extracts names from the array, [0] takes the first element.
		result, err := rp.ProcessRecursively(data, "users{name}[0]", opGet, nil)
		if err != nil {
			t.Fatalf("error: %v", err)
		}
		if result != "Alice" {
			t.Errorf("users{name}[0] = %#v, want \"Alice\"", result)
		}
	})

	t.Run("extract then slice", func(t *testing.T) {
		data := map[string]any{
			"items": []any{
				map[string]any{"vals": []any{1, 2, 3, 4, 5}},
				map[string]any{"vals": []any{6, 7, 8, 9, 10}},
			},
		}

		result, err := rp.ProcessRecursively(data, "items{vals}[0:2]", opGet, nil)
		if err != nil {
			t.Fatalf("error: %v", err)
		}
		// Extraction collects the vals arrays; the slice applies to the
		// COLLECTED array (selecting 2 of 2 here), not to each element.
		arr, ok := result.([]any)
		if !ok {
			t.Fatalf("result is %T, want []any", result)
		}
		if len(arr) != 2 {
			t.Fatalf("len = %d, want 2", len(arr))
		}
		first, ok := arr[0].([]any)
		if !ok || len(first) != 5 || first[0] != 1 {
			t.Errorf("arr[0] = %#v, want the unmodified [1 2 3 4 5]", arr[0])
		}
	})

	t.Run("delete via extract then slice", func(t *testing.T) {
		data := map[string]any{
			"items": []any{
				map[string]any{"tags": []any{"a", "b", "c", "d"}},
				map[string]any{"tags": []any{"e", "f", "g", "h"}},
			},
		}

		_, err := rp.ProcessRecursively(data, "items{tags}[1:3]", opDelete, nil)
		if err != nil {
			t.Fatalf("error: %v", err)
		}

		// Verify tags arrays have elements marked as deleted
		items := data["items"].([]any)
		for i, item := range items {
			tags := item.(map[string]any)["tags"].([]any)
			for j := 1; j < 3; j++ {
				if !isDeletedMarker(tags[j]) {
					t.Errorf("items[%d].tags[%d] should be deleted", i, j)
				}
			}
		}
	})
}

// ============================================================================
// Map traversal — applying operations across map values
// ============================================================================

func TestRecursiveProcessor_MapTraversal(t *testing.T) {
	processor, rp := helperRP(t)
	defer processor.Close()

	t.Run("get array index across map values", func(t *testing.T) {
		data := map[string]any{
			"users": map[string]any{
				"alice": []any{100, 200, 300},
				"bob":   []any{400, 500, 600},
			},
		}

		result, err := rp.ProcessRecursively(data, "users.alice[1]", opGet, nil)
		if err != nil {
			t.Fatalf("error: %v", err)
		}
		if result != 200 {
			t.Errorf("result = %v, want 200", result)
		}
	})

	t.Run("get property across all map values", func(t *testing.T) {
		data := map[string]any{
			"users": map[string]any{
				"u1": map[string]any{"name": "Alice", "age": 30},
				"u2": map[string]any{"name": "Bob", "age": 25},
			},
		}

		// Access "name" through "users" then each value
		result, err := rp.ProcessRecursively(data, "users.u1.name", opGet, nil)
		if err != nil {
			t.Fatalf("error: %v", err)
		}
		if result != "Alice" {
			t.Errorf("result = %v, want Alice", result)
		}
	})
}

// ============================================================================
// combineErrors
// ============================================================================

func TestRecursiveProcessor_CombineErrors(t *testing.T) {
	processor, rp := helperRP(t)
	defer processor.Close()

	t.Run("nil slice returns nil", func(t *testing.T) {
		err := rp.combineErrors(nil)
		if err != nil {
			t.Errorf("expected nil, got %v", err)
		}
	})

	t.Run("empty slice returns nil", func(t *testing.T) {
		err := rp.combineErrors([]error{})
		if err != nil {
			t.Errorf("expected nil, got %v", err)
		}
	})

	t.Run("single error", func(t *testing.T) {
		e := errors.New("test error")
		combined := rp.combineErrors([]error{e})
		if combined == nil {
			t.Error("expected non-nil error")
		}
		if combined.Error() != "test error" {
			t.Errorf("error = %v, want test error", combined)
		}
	})

	t.Run("multiple errors joined", func(t *testing.T) {
		e1 := errors.New("err1")
		e2 := errors.New("err2")
		combined := rp.combineErrors([]error{e1, e2})
		if combined == nil {
			t.Fatal("expected non-nil error")
		}
		// errors.Join produces multi-line error
		if !errors.Is(combined, e1) || !errors.Is(combined, e2) {
			t.Errorf("expected joined error containing both, got %v", combined)
		}
	})

	t.Run("nil errors filtered out", func(t *testing.T) {
		e := errors.New("real error")
		combined := rp.combineErrors([]error{nil, e, nil})
		if combined == nil {
			t.Fatal("expected non-nil error")
		}
		if !errors.Is(combined, e) {
			t.Errorf("expected error containing 'real error', got %v", combined)
		}
	})

	t.Run("all nil errors returns nil", func(t *testing.T) {
		combined := rp.combineErrors([]error{nil, nil, nil})
		if combined != nil {
			t.Errorf("expected nil, got %v", combined)
		}
	})
}

// ============================================================================
// shouldUseDistributedArrayOp
// ============================================================================

func TestRecursiveProcessor_ShouldUseDistributedArrayOp(t *testing.T) {
	processor, rp := helperRP(t)
	defer processor.Close()

	tests := []struct {
		name      string
		container []any
		want      bool
	}{
		{
			name:      "empty array",
			container: []any{},
			want:      false,
		},
		{
			name:      "simple primitives",
			container: []any{1, 2, 3},
			want:      false,
		},
		{
			name:      "simple objects not distributed",
			container: []any{map[string]any{"a": 1}, map[string]any{"b": 2}},
			want:      false,
		},
		{
			name:      "triple nested pattern",
			container: []any{[]any{[]any{1}}},
			want:      true,
		},
		{
			name:      "arrays of objects",
			container: []any{[]any{map[string]any{"a": 1}}, []any{map[string]any{"b": 2}}},
			want:      true,
		},
		{
			name:      "mixed array and non-array",
			container: []any{[]any{1}, "string"},
			want:      false,
		},
		{
			name:      "arrays of primitives only",
			container: []any{[]any{1, 2}, []any{3, 4}},
			want:      false, // no objects inside
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := rp.shouldUseDistributedArrayOp(tt.container)
			if got != tt.want {
				t.Errorf("shouldUseDistributedArrayOp() = %v, want %v", got, tt.want)
			}
		})
	}
}

// ============================================================================
// findTargetArrayForDistributedOp
// ============================================================================

func TestRecursiveProcessor_FindTargetArray(t *testing.T) {
	processor, rp := helperRP(t)
	defer processor.Close()

	tests := []struct {
		name  string
		input any
		want  int // expected length of result, -1 means nil
	}{
		{
			name:  "non-array returns nil",
			input: "string",
			want:  -1,
		},
		{
			name:  "simple array returns itself",
			input: []any{1, 2, 3},
			want:  3,
		},
		{
			name:  "single nested array unwraps",
			input: []any{[]any{map[string]any{"a": 1}, map[string]any{"b": 2}}},
			want:  2,
		},
		{
			name:  "single nested empty array",
			input: []any{[]any{}},
			want:  0,
		},
		{
			name:  "double nested array with objects",
			input: []any{[]any{[]any{map[string]any{"x": 1}}}},
			want:  1,
		},
		{
			name:  "single element not array",
			input: []any{"single"},
			want:  1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := rp.findTargetArrayForDistributedOp(tt.input)
			if tt.want == -1 {
				if result != nil {
					t.Errorf("expected nil, got %v", result)
				}
				return
			}
			if result == nil {
				t.Fatalf("expected non-nil result with length %d", tt.want)
			}
			if len(result) != tt.want {
				t.Errorf("len(result) = %d, want %d", len(result), tt.want)
			}
		})
	}
}

// ============================================================================
// deepFlattenDistributedResults
// ============================================================================

func TestRecursiveProcessor_DeepFlatten(t *testing.T) {
	processor, rp := helperRP(t)
	defer processor.Close()

	t.Run("flat values pass through", func(t *testing.T) {
		input := []any{"a", "b", "c"}
		result := rp.deepFlattenDistributedResults(input)
		if len(result) != 3 {
			t.Errorf("len = %d, want 3", len(result))
		}
	})

	t.Run("nested arrays are flattened one level", func(t *testing.T) {
		input := []any{[]any{"a", "b"}, []any{"c"}}
		result := rp.deepFlattenDistributedResults(input)
		if len(result) != 3 {
			t.Errorf("len = %d, want 3", len(result))
		}
	})

	t.Run("mixed nested and flat", func(t *testing.T) {
		input := []any{[]any{"x", "y"}, "z"}
		result := rp.deepFlattenDistributedResults(input)
		if len(result) != 3 {
			t.Errorf("len = %d, want 3", len(result))
		}
	})

	t.Run("empty input", func(t *testing.T) {
		result := rp.deepFlattenDistributedResults([]any{})
		if len(result) != 0 {
			t.Errorf("len = %d, want 0", len(result))
		}
	})
}

// ============================================================================
// deepFlattenResults (recursive variant)
// ============================================================================

func TestRecursiveProcessor_DeepFlattenResults(t *testing.T) {
	processor, rp := helperRP(t)
	defer processor.Close()

	t.Run("deeply nested arrays", func(t *testing.T) {
		input := []any{
			[]any{
				[]any{1, 2},
				[]any{3},
			},
			4,
		}
		var flattened []any
		rp.deepFlattenResults(input, &flattened)
		if len(flattened) != 4 {
			t.Errorf("len = %d, want 4", len(flattened))
		}
		expected := []any{1, 2, 3, 4}
		for i, v := range flattened {
			if v != expected[i] {
				t.Errorf("flattened[%d] = %v, want %v", i, v, expected[i])
			}
		}
	})

	t.Run("no nesting", func(t *testing.T) {
		input := []any{"a", "b"}
		var flattened []any
		rp.deepFlattenResults(input, &flattened)
		if len(flattened) != 2 {
			t.Errorf("len = %d, want 2", len(flattened))
		}
	})
}

// ============================================================================
// extractMultipleFieldsFromMap
// ============================================================================

func TestRecursiveProcessor_ExtractMultipleFieldsFromMap(t *testing.T) {
	processor, rp := helperRP(t)
	defer processor.Close()

	tests := []struct {
		name   string
		source map[string]any
		fields []string
		want   map[string]any
	}{
		{
			name:   "extract existing fields",
			source: map[string]any{"a": 1, "b": 2, "c": 3},
			fields: []string{"a", "c"},
			want:   map[string]any{"a": 1, "c": 3},
		},
		{
			name:   "extract with missing field",
			source: map[string]any{"a": 1},
			fields: []string{"a", "missing"},
			want:   map[string]any{"a": 1},
		},
		{
			name:   "all fields missing returns nil",
			source: map[string]any{"x": 1},
			fields: []string{"a", "b"},
			want:   nil,
		},
		{
			name:   "empty field name skipped",
			source: map[string]any{"a": 1, "": "empty"},
			fields: []string{"a", ""},
			want:   map[string]any{"a": 1},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := rp.extractMultipleFieldsFromMap(tt.source, tt.fields)
			if tt.want == nil {
				if result != nil {
					t.Errorf("expected nil, got %v", result)
				}
				return
			}
			if len(result) != len(tt.want) {
				t.Fatalf("len = %d, want %d", len(result), len(tt.want))
			}
			for k, v := range tt.want {
				if result[k] != v {
					t.Errorf("result[%s] = %v, want %v", k, result[k], v)
				}
			}
		})
	}
}

// ============================================================================
// Property segment on non-object types
// ============================================================================

// TestRecursiveProcessor_SegmentOnNonContainer covers applying a property or
// wildcard segment to a non-container value (an int): Get returns nil, Set
// returns an error. Previously split across PropertySegmentNonContainer and
// WildcardOnNonContainer; consolidated into one table.
func TestRecursiveProcessor_SegmentOnNonContainer(t *testing.T) {
	_, rp := helperRP(t)

	tests := []struct {
		name    string
		path    string
		op      operation
		value   any
		wantErr bool
		wantNil bool
	}{
		{"property get on int", "key", opGet, nil, false, true},
		{"property set on int", "key", opSet, "val", true, false},
		{"wildcard get on int", "*", opGet, nil, false, true},
		{"wildcard set on int", "*", opSet, "val", true, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := rp.ProcessRecursively(42, tt.path, tt.op, tt.value)
			if tt.wantErr && err == nil {
				t.Error("expected error for segment access on int")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if tt.wantNil && result != nil {
				t.Errorf("expected nil result, got %v", result)
			}
		})
	}
}

// ============================================================================
// Array slice segment on non-array types
// ============================================================================

func TestRecursiveProcessor_ArraySliceOnMap(t *testing.T) {
	processor, rp := helperRP(t)
	defer processor.Close()

	t.Run("slice on map values containing arrays", func(t *testing.T) {
		data := map[string]any{
			"outer": map[string]any{
				"a": []any{1, 2, 3, 4},
				"b": []any{5, 6, 7, 8},
			},
		}

		result, err := rp.ProcessRecursively(data, "outer.a[1:3]", opGet, nil)
		if err != nil {
			t.Fatalf("error: %v", err)
		}
		arr, ok := result.([]any)
		if !ok {
			t.Fatalf("expected []any, got %T", result)
		}
		if len(arr) != 2 {
			t.Errorf("len = %d, want 2", len(arr))
		}
	})
}

// ============================================================================
// extractMultipleFieldsFromMap edge cases
// ============================================================================

func TestRecursiveProcessor_ExtractMultipleFieldsFromMap_EdgeCases(t *testing.T) {
	processor, rp := helperRP(t)
	defer processor.Close()

	t.Run("empty source map", func(t *testing.T) {
		result := rp.extractMultipleFieldsFromMap(map[string]any{}, []string{"a"})
		if result != nil {
			t.Errorf("expected nil for empty map, got %v", result)
		}
	})

	t.Run("all empty field names", func(t *testing.T) {
		source := map[string]any{"a": 1}
		result := rp.extractMultipleFieldsFromMap(source, []string{"", ""})
		if result != nil {
			t.Errorf("expected nil for all-empty field names, got %v", result)
		}
	})
}

// ============================================================================
// compareResults is a helper for comparing results
// ============================================================================

func compareResults(t *testing.T, want, got any) {
	t.Helper()

	switch w := want.(type) {
	case []any:
		g, ok := got.([]any)
		if !ok {
			t.Errorf("expected []any, got %T", got)
			return
		}
		if len(w) != len(g) {
			t.Errorf("slice lengths differ: want %d, got %d", len(w), len(g))
			return
		}
		for i := range w {
			if w[i] != g[i] {
				t.Errorf("element[%d]: want %v, got %v", i, w[i], g[i])
			}
		}
	case map[string]any:
		g, ok := got.(map[string]any)
		if !ok {
			t.Errorf("expected map[string]any, got %T", got)
			return
		}
		if len(w) != len(g) {
			t.Errorf("map lengths differ: want %d, got %d", len(w), len(g))
			return
		}
		for k, v := range w {
			if g[k] != v {
				t.Errorf("map[%s]: want %v, got %v", k, v, g[k])
			}
		}
	default:
		if want != got {
			t.Errorf("want %v (%T), got %v (%T)", want, want, got, got)
		}
	}
}

// ============================================================================
// Additional recursive coverage tests — package-level Get/Set/Delete paths
// ============================================================================

// TestRecursive_DistributedArrayIndex tests handleArrayIndexSegmentUnified distributed paths
func TestRecursive_DistributedArrayIndex(t *testing.T) {
	t.Run("get from array of arrays", func(t *testing.T) {
		json := `{"matrix":[[1,2],[3,4],[5,6]]}`
		result, err := Get(json, "matrix[0]")
		if err != nil {
			t.Fatalf("Get error: %v", err)
		}
		// Plain indexing: the first sub-array itself.
		sub, ok := result.([]any)
		if !ok || len(sub) != 2 || sub[0] != 1.0 || sub[1] != 2.0 {
			t.Errorf("matrix[0] = %#v, want [1 2]", result)
		}
	})

	t.Run("set in distributed array", func(t *testing.T) {
		json := `{"matrix":[[1,2],[3,4]]}`
		result, err := Set(json, "matrix[0]", 99)
		if err != nil {
			t.Fatalf("Set error: %v", err)
		}
		if result == "" {
			t.Error("result should not be empty")
		}
	})

	t.Run("get from map with array values", func(t *testing.T) {
		json := `{"data":{"a":[1,2],"b":[3,4]}}`
		result, err := Get(json, "data[0]")
		if err != nil {
			t.Fatalf("Get error: %v", err)
		}
		// Should get first element from each array
		arr, ok := result.([]any)
		if !ok {
			t.Fatalf("result is %T, want []any", result)
		}
		if len(arr) != 2 {
			t.Errorf("length = %d, want 2", len(arr))
		}
	})

	t.Run("out of bounds in distributed", func(t *testing.T) {
		json := `{"matrix":[[1],[2,3]]}`
		// First sub-array has length 1, so index 1 is out of bounds
		result, err := Get(json, "matrix[1]")
		// Implementation may reject an out-of-bounds distributed index; if it
		// does, that is an acceptable outcome.
		if err != nil {
			return
		}
		arr, ok := result.([]any)
		if !ok {
			t.Fatalf("result is %T, want []any", result)
		}
		// The valid sub-array ([2,3][1]) must survive; the OOB one is skipped.
		var hasThree bool
		for _, v := range arr {
			if n, ok := v.(float64); ok && n == 3 {
				hasThree = true
			}
		}
		if !hasThree {
			t.Errorf("distributed matrix[1] = %v, want 3 present", result)
		}
	})
}

// TestRecursive_WildcardArray tests handleWildcardSegmentUnified with arrays
func TestRecursive_WildcardArray(t *testing.T) {
	t.Run("get all array elements", func(t *testing.T) {
		json := `{"items":[1,2,3]}`
		result, err := Get(json, "items[*]")
		if err != nil {
			t.Fatalf("Get error: %v", err)
		}
		arr, ok := result.([]any)
		if !ok {
			t.Fatalf("result is %T, want []any", result)
		}
		if len(arr) != 3 {
			t.Errorf("length = %d, want 3", len(arr))
		}
	})

	t.Run("set all array elements", func(t *testing.T) {
		json := `{"items":[1,2,3]}`
		result, err := Set(json, "items[*]", 0)
		if err != nil {
			t.Fatalf("Set error: %v", err)
		}
		assertJSONEqual(t, `{"items":[0,0,0]}`, result)
	})

	t.Run("delete all array elements", func(t *testing.T) {
		json := `{"items":[1,2,3]}`
		_, err := Delete(json, "items[*]")
		if err != nil {
			t.Fatalf("Delete error: %v", err)
		}
	})

	t.Run("wildcard with nested path", func(t *testing.T) {
		json := `{"items":[{"x":1},{"x":2}]}`
		result, err := Get(json, "items[*].x")
		if err != nil {
			t.Fatalf("Get error: %v", err)
		}
		arr, ok := result.([]any)
		if !ok {
			t.Fatalf("result is %T, want []any", result)
		}
		if len(arr) != 2 {
			t.Errorf("length = %d, want 2", len(arr))
		}
	})
}

// TestRecursive_WildcardMap tests handleWildcardSegmentUnified with maps
func TestRecursive_WildcardMap(t *testing.T) {
	t.Run("get all map values", func(t *testing.T) {
		json := `{"data":{"a":1,"b":2}}`
		result, err := Get(json, "data[*]")
		if err != nil {
			t.Fatalf("Get error: %v", err)
		}
		arr, ok := result.([]any)
		if !ok {
			t.Fatalf("result is %T, want []any", result)
		}
		if len(arr) != 2 {
			t.Errorf("length = %d, want 2", len(arr))
		}
	})

	t.Run("set all map values", func(t *testing.T) {
		json := `{"data":{"a":1,"b":2}}`
		result, err := Set(json, "data[*]", 9)
		if err != nil {
			t.Fatalf("Set error: %v", err)
		}
		assertJSONEqual(t, `{"data":{"a":9,"b":9}}`, result)
	})

	t.Run("delete all map values", func(t *testing.T) {
		json := `{"data":{"a":1,"b":2}}`
		result, err := Delete(json, "data[*]")
		if err != nil {
			t.Fatalf("Delete error: %v", err)
		}
		assertJSONEqual(t, `{"data":{}}`, result)
	})

	t.Run("wildcard map with nested path", func(t *testing.T) {
		json := `{"outer":{"a":{"x":1},"b":{"x":2}}}`
		result, err := Get(json, "outer[*].x")
		if err != nil {
			t.Fatalf("Get error: %v", err)
		}
		arr, ok := result.([]any)
		if !ok {
			t.Fatalf("result is %T, want []any", result)
		}
		if len(arr) != 2 {
			t.Errorf("length = %d, want 2", len(arr))
		}
	})
}

// TestRecursive_ExtractThenSlice tests handleExtractThenSlice paths
func TestRecursive_ExtractThenSlice(t *testing.T) {
	t.Run("extract and slice from array", func(t *testing.T) {
		json := `{"items":[{"name":"a","v":1},{"name":"b","v":2},{"name":"c","v":3}]}`
		result, err := Get(json, "items.{name}[0:2]")
		if err != nil {
			t.Fatalf("Get error: %v", err)
		}
		arr, ok := result.([]any)
		if !ok {
			t.Fatalf("result is %T, want []any", result)
		}
		if len(arr) != 2 {
			t.Errorf("length = %d, want 2", len(arr))
		}
	})

	t.Run("extract and delete slice", func(t *testing.T) {
		json := `{"items":[{"name":"a","v":1},{"name":"b","v":2}]}`
		result, err := Delete(json, "items.{name}[0:1]")
		// The operation may be rejected; if so, that is acceptable. If it
		// succeeds, the result must still be valid JSON with both items intact
		// (the slice delete must not corrupt the surrounding array).
		if err != nil {
			return
		}
		var got map[string]any
		if err := Unmarshal([]byte(result), &got); err != nil {
			t.Fatalf("result is not valid JSON: %v\n%s", err, result)
		}
		items, ok := got["items"].([]any)
		if !ok || len(items) != 2 {
			t.Errorf("after delete, items has %d elements, want 2; result=%s", len(items), result)
		}
	})
}

// ============================================================================
// RECURSIVE BOUNDARY TESTS (merged from operations_boundary_test.go)
// Defensive-branch coverage for recursive.go via the public Set/Get/Delete API.
// ============================================================================

// TestRecursive_ArrayIndex_Boundary exercises handleArrayIndexSegmentUnified /
// handleArraySliceSegmentUnified defensive branches.
func TestRecursive_ArrayIndex_Boundary(t *testing.T) {
	t.Run("index_on_map_no_panic", func(t *testing.T) {
		// Array index access on a non-array (object) must not panic; it surfaces
		// as a not-found result (nil and/or error) rather than crashing.
		v, err := Get(`{"a":1}`, "[0]")
		if err == nil && v != nil {
			t.Errorf("expected nil-or-error for array index on object, got (%v, %v)", v, err)
		}
	})
	t.Run("oob_index_extends_with_createPaths", func(t *testing.T) {
		// Default config has CreatePaths=true: OOB index extends the array.
		r, err := Set(`{"a":[1,2]}`, "a[5]", 99)
		if err != nil {
			t.Fatalf("Set err: %v", err)
		}
		v, _ := Get(r, "a[5]")
		if v != float64(99) {
			t.Errorf("a[5] = %v, want 99", v)
		}
	})
	t.Run("slice_delete", func(t *testing.T) {
		r, err := Delete(`{"a":[1,2,3]}`, "a[0:1]")
		if err != nil {
			t.Fatalf("Delete err: %v", err)
		}
		v, _ := Get(r, "a")
		arr, ok := v.([]any)
		if !ok || len(arr) != 2 || arr[0] != float64(2) || arr[1] != float64(3) {
			t.Errorf("after slice delete got %v, want [2 3]", v)
		}
	})
	t.Run("distributed_oob_set_extends_with_nulls", func(t *testing.T) {
		// Distributed set on a slice with an OOB index extends with nulls
		// rather than panicking or erroring.
		result, err := Set(`{"items":[[1,2],[3,4]]}`, "items[5]", 99)
		if err != nil {
			t.Fatalf("OOB distributed set: %v", err)
		}
		want := `{"items":[[1,2],[3,4],null,null,null,99]}`
		if result != want {
			t.Errorf("OOB distributed set = %s, want %s", result, want)
		}
	})
}

// TestRecursive_Extract_Boundary exercises handleMultiFieldExtractSegment /
// handleExtractThenSlice defensive branches.
func TestRecursive_Extract_Boundary(t *testing.T) {
	t.Run("multifield_delete", func(t *testing.T) {
		r, err := Delete(`{"users":[{"id":1,"name":"a","x":2}]}`, "users.{id,name}")
		if err != nil {
			t.Fatalf("Delete err: %v", err)
		}
		x, _ := Get(r, "users[0].x")
		if x != float64(2) {
			t.Errorf("x should be preserved, got %v", x)
		}
		id, _ := Get(r, "users[0].id")
		if id != nil {
			t.Errorf("id should be deleted, got %v", id)
		}
	})
	t.Run("multifield_set_with_map", func(t *testing.T) {
		r, err := Set(`{"users":[{"id":1}]}`, "users.{id,name}", map[string]any{"id": 2, "name": "b"})
		if err != nil {
			t.Fatalf("Set err: %v", err)
		}
		id, _ := Get(r, "users[0].id")
		name, _ := Get(r, "users[0].name")
		if id != float64(2) || name != "b" {
			t.Errorf("got id=%v name=%v, want 2/b", id, name)
		}
	})
	t.Run("extract_set_non_map_value", func(t *testing.T) {
		// Setting an extract with a non-map scalar completes without panicking;
		// the branch is exercised regardless of how the scalar is applied.
		if _, err := Set(`{"users":[{"id":1}]}`, "users.{id}", "not a map"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	t.Run("extract_then_slice_on_non_container", func(t *testing.T) {
		// {extract}[slice] on a non-container (JSON number) must not panic.
		_, _ = Get("42", "{tags}[0:1]")
	})
	t.Run("extract_then_slice_set", func(t *testing.T) {
		r, err := Set(`{"items":[{"tags":["a","b"]}]}`, "items{tags}[0:1]", "x")
		if err != nil {
			t.Fatalf("Set err: %v", err)
		}
		v, _ := Get(r, "items[0].tags")
		arr, ok := v.([]any)
		if !ok || len(arr) == 0 || arr[0] != "x" {
			t.Errorf("extract-then-slice set got %v, want first element x", v)
		}
	})
}

// TestRecursive_EmptyPath_Boundary exercises ProcessRecursivelyWithOptions
// empty-path handling (recursive.go): Set with an empty path replaces the
// whole document (GEN-001); Delete with an empty path must still error.
func TestRecursive_EmptyPath_Boundary(t *testing.T) {
	result, err := Set(`{"a":1}`, "", 99)
	if err != nil {
		t.Fatalf("expected root Set to replace the document, got error: %v", err)
	}
	assertJSONEqual(t, `99`, result)
	if _, err := Delete(`{"a":1}`, ""); err == nil {
		t.Error("expected error for Delete with empty path")
	}
}
