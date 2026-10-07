package json

import (
	"strings"
	"testing"
)

// ============================================================================
// TestProcessorSetCreate
// ============================================================================

func TestProcessorSetCreate(t *testing.T) {
	p, err := New()
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer p.Close()

	t.Run("CreateNestedPathInEmptyObject", func(t *testing.T) {
		result, err := p.SetCreate(`{}`, "a.b.c", 42)
		if err != nil {
			t.Fatalf("SetCreate error: %v", err)
		}

		m := mustParseMap(t, result)
		a, ok := m["a"].(map[string]any)
		if !ok {
			t.Fatalf("expected a to be a map, got %T", m["a"])
		}
		b, ok := a["b"].(map[string]any)
		if !ok {
			t.Fatalf("expected a.b to be a map, got %T", a["b"])
		}
		if b["c"] != float64(42) {
			t.Errorf("expected a.b.c = 42, got %v", b["c"])
		}
	})

	t.Run("CreatePathAlongsideExistingData", func(t *testing.T) {
		result, err := p.SetCreate(`{"x":1}`, "y.z", "hello")
		if err != nil {
			t.Fatalf("SetCreate error: %v", err)
		}

		m := mustParseMap(t, result)
		if m["x"] != float64(1) {
			t.Errorf("expected x=1, got %v", m["x"])
		}
		y, ok := m["y"].(map[string]any)
		if !ok {
			t.Fatalf("expected y to be a map, got %T", m["y"])
		}
		if y["z"] != "hello" {
			t.Errorf("expected y.z = 'hello', got %v", y["z"])
		}
	})

	t.Run("OverwriteExistingPath", func(t *testing.T) {
		result, err := p.SetCreate(`{"a":1}`, "a", 99)
		if err != nil {
			t.Fatalf("SetCreate error: %v", err)
		}

		m := mustParseMap(t, result)
		if m["a"] != float64(99) {
			t.Errorf("expected a=99, got %v", m["a"])
		}
	})

	t.Run("OverwriteNestedExistingPath", func(t *testing.T) {
		result, err := p.SetCreate(`{"a":{"b":"old"}}`, "a.b", "new")
		if err != nil {
			t.Fatalf("SetCreate error: %v", err)
		}

		m := mustParseMap(t, result)
		a := m["a"].(map[string]any)
		if a["b"] != "new" {
			t.Errorf("expected a.b='new', got %v", a["b"])
		}
	})

	t.Run("InvalidJSONReturnsError", func(t *testing.T) {
		_, err := p.SetCreate(`{invalid}`, "a.b", 1)
		if err == nil {
			t.Error("expected error for invalid JSON")
		}
	})

	t.Run("EmptyPathReplacesDocument", func(t *testing.T) {
		// GEN-001: a root path replaces the whole document with the value.
		result, err := p.SetCreate(`{"a":1}`, "", 42)
		if err != nil {
			t.Fatalf("expected root replacement, got error: %v", err)
		}
		if result != "42" {
			t.Errorf("expected document replaced with 42, got %s", result)
		}
	})

	t.Run("DeepNestedPathCreation", func(t *testing.T) {
		result, err := p.SetCreate(`{}`, "a.b.c.d.e", "deep")
		if err != nil {
			t.Fatalf("SetCreate error: %v", err)
		}

		assertContains(t, result, `"deep"`)
		m := mustParseMap(t, result)
		// Walk down the chain: a -> b -> c -> d -> e
		current := m
		for _, key := range []string{"a", "b", "c", "d"} {
			next, ok := current[key].(map[string]any)
			if !ok {
				t.Fatalf("expected %s to be a map, got %T", key, current[key])
			}
			current = next
		}
		if current["e"] != "deep" {
			t.Errorf("expected a.b.c.d.e = 'deep', got %v", current["e"])
		}
	})

	t.Run("SetCreateArrayValue", func(t *testing.T) {
		result, err := p.SetCreate(`{}`, "items", []any{1, 2, 3})
		if err != nil {
			t.Fatalf("SetCreate error: %v", err)
		}

		m := mustParseMap(t, result)
		arr, ok := m["items"].([]any)
		if !ok {
			t.Fatalf("expected items to be an array, got %T", m["items"])
		}
		if len(arr) != 3 {
			t.Errorf("expected 3 items, got %d", len(arr))
		}
	})

	t.Run("SetCreateNullValue", func(t *testing.T) {
		result, err := p.SetCreate(`{}`, "a", nil)
		if err != nil {
			t.Fatalf("SetCreate error: %v", err)
		}

		m := mustParseMap(t, result)
		if m["a"] != nil {
			t.Errorf("expected a=null, got %v", m["a"])
		}
	})

	t.Run("SetCreateBoolValue", func(t *testing.T) {
		result, err := p.SetCreate(`{}`, "active", true)
		if err != nil {
			t.Fatalf("SetCreate error: %v", err)
		}

		m := mustParseMap(t, result)
		if m["active"] != true {
			t.Errorf("expected active=true, got %v", m["active"])
		}
	})

	t.Run("SetCreateWithConfigOverride", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.SkipValidation = true

		result, err := p.SetCreate(`{"x":1}`, "y", 2, cfg)
		if err != nil {
			t.Fatalf("SetCreate with config error: %v", err)
		}

		m := mustParseMap(t, result)
		if m["x"] != float64(1) {
			t.Errorf("expected x=1, got %v", m["x"])
		}
		if m["y"] != float64(2) {
			t.Errorf("expected y=2, got %v", m["y"])
		}
	})
}

// ============================================================================
// TestProcessorSetMultipleCreate
// ============================================================================

func TestProcessorSetMultipleCreate(t *testing.T) {
	p, err := New()
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer p.Close()

	t.Run("CreateNestedPaths", func(t *testing.T) {
		updates := map[string]any{
			"a.x": 1,
			"a.y": 2,
		}

		result, err := p.SetMultipleCreate(`{}`, updates)
		if err != nil {
			t.Fatalf("SetMultipleCreate error: %v", err)
		}

		m := mustParseMap(t, result)
		a, ok := m["a"].(map[string]any)
		if !ok {
			t.Fatalf("expected a to be a map, got %T", m["a"])
		}
		if a["x"] != float64(1) {
			t.Errorf("expected a.x=1, got %v", a["x"])
		}
		if a["y"] != float64(2) {
			t.Errorf("expected a.y=2, got %v", a["y"])
		}
	})

	t.Run("EmptyUpdatesReturnsOriginal", func(t *testing.T) {
		original := `{"a":1}`
		result, err := p.SetMultipleCreate(original, map[string]any{})
		if err != nil {
			t.Fatalf("SetMultipleCreate error: %v", err)
		}

		m := mustParseMap(t, result)
		if m["a"] != float64(1) {
			t.Errorf("expected a=1, got %v", m["a"])
		}
	})

	t.Run("MultipleDeepPaths", func(t *testing.T) {
		updates := map[string]any{
			"user.name":         "Alice",
			"user.age":          30,
			"user.address.city": "Wonderland",
		}

		result, err := p.SetMultipleCreate(`{}`, updates)
		if err != nil {
			t.Fatalf("SetMultipleCreate error: %v", err)
		}

		m := mustParseMap(t, result)
		user, ok := m["user"].(map[string]any)
		if !ok {
			t.Fatalf("expected user to be a map, got %T", m["user"])
		}
		if user["name"] != "Alice" {
			t.Errorf("expected user.name='Alice', got %v", user["name"])
		}
		if user["age"] != float64(30) {
			t.Errorf("expected user.age=30, got %v", user["age"])
		}
		addr, ok := user["address"].(map[string]any)
		if !ok {
			t.Fatalf("expected user.address to be a map, got %T", user["address"])
		}
		if addr["city"] != "Wonderland" {
			t.Errorf("expected user.address.city='Wonderland', got %v", addr["city"])
		}
	})

	t.Run("PreservesExistingData", func(t *testing.T) {
		updates := map[string]any{
			"b": 2,
		}

		result, err := p.SetMultipleCreate(`{"a":1}`, updates)
		if err != nil {
			t.Fatalf("SetMultipleCreate error: %v", err)
		}

		m := mustParseMap(t, result)
		if m["a"] != float64(1) {
			t.Errorf("expected existing a=1 preserved, got %v", m["a"])
		}
		if m["b"] != float64(2) {
			t.Errorf("expected b=2, got %v", m["b"])
		}
	})

	t.Run("InvalidJSONReturnsError", func(t *testing.T) {
		updates := map[string]any{"a": 1}
		_, err := p.SetMultipleCreate(`not json`, updates)
		if err == nil {
			t.Error("expected error for invalid JSON")
		}
	})

	t.Run("OverwriteExistingValues", func(t *testing.T) {
		updates := map[string]any{
			"x": "updated",
		}

		result, err := p.SetMultipleCreate(`{"x":"original"}`, updates)
		if err != nil {
			t.Fatalf("SetMultipleCreate error: %v", err)
		}

		m := mustParseMap(t, result)
		if m["x"] != "updated" {
			t.Errorf("expected x='updated', got %v", m["x"])
		}
	})

	t.Run("MixedTypesInUpdates", func(t *testing.T) {
		updates := map[string]any{
			"str":   "hello",
			"num":   42,
			"flag":  true,
			"null":  nil,
			"array": []any{1, 2, 3},
		}

		result, err := p.SetMultipleCreate(`{}`, updates)
		if err != nil {
			t.Fatalf("SetMultipleCreate error: %v", err)
		}

		m := mustParseMap(t, result)
		if m["str"] != "hello" {
			t.Errorf("expected str='hello', got %v", m["str"])
		}
		if m["num"] != float64(42) {
			t.Errorf("expected num=42, got %v", m["num"])
		}
		if m["flag"] != true {
			t.Errorf("expected flag=true, got %v", m["flag"])
		}
		if m["null"] != nil {
			t.Errorf("expected null=nil, got %v", m["null"])
		}
		arr, ok := m["array"].([]any)
		if !ok || len(arr) != 3 {
			t.Errorf("expected array of length 3, got %v", m["array"])
		}
	})

	t.Run("SetMultipleCreateWithConfig", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.SkipValidation = true

		updates := map[string]any{"z": 99}
		result, err := p.SetMultipleCreate(`{"a":1}`, updates, cfg)
		if err != nil {
			t.Fatalf("SetMultipleCreate with config error: %v", err)
		}

		m := mustParseMap(t, result)
		if m["a"] != float64(1) {
			t.Errorf("expected a=1, got %v", m["a"])
		}
		if m["z"] != float64(99) {
			t.Errorf("expected z=99, got %v", m["z"])
		}
	})
}

// ============================================================================
// TestProcessorDeleteClean
// ============================================================================

func TestProcessorDeleteClean(t *testing.T) {
	p, err := New()
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer p.Close()

	t.Run("DeleteKeyNoNullGaps", func(t *testing.T) {
		result, err := p.DeleteClean(`{"a":1,"b":2,"c":3}`, "b")
		if err != nil {
			t.Fatalf("DeleteClean error: %v", err)
		}

		m := mustParseMap(t, result)

		// b should be gone entirely, not null.
		if _, exists := m["b"]; exists {
			t.Error("key 'b' should not exist in result")
		}
		if m["a"] != float64(1) {
			t.Errorf("expected a=1, got %v", m["a"])
		}
		if m["c"] != float64(3) {
			t.Errorf("expected c=3, got %v", m["c"])
		}

		// Ensure no "null" appears in the raw JSON string.
		if strings.Contains(result, "null") {
			t.Errorf("result should not contain 'null': %s", result)
		}
	})

	t.Run("DeleteArrayElementCompactsArray", func(t *testing.T) {
		result, err := p.DeleteClean(`{"arr":[1,2,3]}`, "arr[1]")
		if err != nil {
			t.Fatalf("DeleteClean error: %v", err)
		}

		m := mustParseMap(t, result)
		arr, ok := m["arr"].([]any)
		if !ok {
			t.Fatalf("expected arr to be an array, got %T", m["arr"])
		}

		// Array should be compacted: [1,3] not [1,null,3]
		if len(arr) != 2 {
			t.Fatalf("expected array length 2, got %d: %v", len(arr), arr)
		}
		if arr[0] != float64(1) {
			t.Errorf("expected arr[0]=1, got %v", arr[0])
		}
		if arr[1] != float64(3) {
			t.Errorf("expected arr[1]=3, got %v", arr[1])
		}

		// Ensure no null in the raw JSON string.
		if strings.Contains(result, "null") {
			t.Errorf("result should not contain 'null' (compact array expected): %s", result)
		}
	})

	t.Run("NonExistentPathReturnsError", func(t *testing.T) {
		_, err := p.DeleteClean(`{"a":1}`, "nonexistent")
		if err == nil {
			t.Error("expected error for non-existent path")
		}
	})

	t.Run("DeleteNestedKeyCleansUp", func(t *testing.T) {
		result, err := p.DeleteClean(
			`{"user":{"name":"Alice","email":"alice@example.com","temp":null}}`,
			"user.temp",
		)
		if err != nil {
			t.Fatalf("DeleteClean error: %v", err)
		}

		// The temp field was already null, so delete should remove it cleanly.
		m := mustParseMap(t, result)
		user := m["user"].(map[string]any)
		if _, exists := user["temp"]; exists {
			t.Error("user.temp should not exist after delete")
		}
		if user["name"] != "Alice" {
			t.Errorf("expected user.name='Alice', got %v", user["name"])
		}
	})

	t.Run("DeleteFromEmptyObject", func(t *testing.T) {
		_, err := p.DeleteClean(`{}`, "a")
		if err == nil {
			t.Error("expected error when deleting from empty object")
		}
	})

	t.Run("InvalidJSONReturnsError", func(t *testing.T) {
		_, err := p.DeleteClean(`{invalid}`, "a")
		if err == nil {
			t.Error("expected error for invalid JSON")
		}
	})

	t.Run("DeleteFirstArrayElement", func(t *testing.T) {
		result, err := p.DeleteClean(`{"arr":[10,20,30]}`, "arr[0]")
		if err != nil {
			t.Fatalf("DeleteClean error: %v", err)
		}

		m := mustParseMap(t, result)
		arr := m["arr"].([]any)
		if len(arr) != 2 {
			t.Fatalf("expected array length 2, got %d", len(arr))
		}
		if arr[0] != float64(20) {
			t.Errorf("expected arr[0]=20, got %v", arr[0])
		}
		if arr[1] != float64(30) {
			t.Errorf("expected arr[1]=30, got %v", arr[1])
		}
	})

	t.Run("DeleteLastArrayElement", func(t *testing.T) {
		result, err := p.DeleteClean(`{"arr":[10,20,30]}`, "arr[2]")
		if err != nil {
			t.Fatalf("DeleteClean error: %v", err)
		}

		m := mustParseMap(t, result)
		arr := m["arr"].([]any)
		if len(arr) != 2 {
			t.Fatalf("expected array length 2, got %d", len(arr))
		}
		if arr[0] != float64(10) {
			t.Errorf("expected arr[0]=10, got %v", arr[0])
		}
		if arr[1] != float64(20) {
			t.Errorf("expected arr[1]=20, got %v", arr[1])
		}
	})

	t.Run("DeleteCleanWithConfigOverride", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.SkipValidation = true

		result, err := p.DeleteClean(`{"a":1,"b":2,"c":3}`, "b", cfg)
		if err != nil {
			t.Fatalf("DeleteClean with config error: %v", err)
		}

		m := mustParseMap(t, result)
		if _, exists := m["b"]; exists {
			t.Error("key 'b' should not exist after delete")
		}
		if m["a"] != float64(1) {
			t.Errorf("expected a=1, got %v", m["a"])
		}
		if m["c"] != float64(3) {
			t.Errorf("expected c=3, got %v", m["c"])
		}
	})

	t.Run("DeleteAllKeysFromObject", func(t *testing.T) {
		// Deleting all keys one by one should eventually leave an empty object.
		input := `{"a":1,"b":2,"c":3}`

		result, err := p.DeleteClean(input, "a")
		if err != nil {
			t.Fatalf("DeleteClean 'a' error: %v", err)
		}
		result, err = p.DeleteClean(result, "b")
		if err != nil {
			t.Fatalf("DeleteClean 'b' error: %v", err)
		}
		result, err = p.DeleteClean(result, "c")
		if err != nil {
			t.Fatalf("DeleteClean 'c' error: %v", err)
		}

		m := mustParseMap(t, result)
		if len(m) != 0 {
			t.Errorf("expected empty object, got %v", m)
		}
	})
}
