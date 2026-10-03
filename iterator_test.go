package json

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
)

// Merged from: iterator_test.go, performance_test.go

// ============================================================================
// Test Helper Functions
// ============================================================================

// compareValues compares two values for equality
func compareValues(a, b any) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}

	switch va := a.(type) {
	case string:
		vb, ok := b.(string)
		return ok && va == vb
	case int:
		vb, ok := b.(int)
		return ok && va == vb
	case float64:
		vb, ok := b.(float64)
		return ok && va == vb
	case bool:
		vb, ok := b.(bool)
		return ok && va == vb
	case []any:
		vb, ok := b.([]any)
		if !ok || len(va) != len(vb) {
			return false
		}
		for i := range va {
			if !compareValues(va[i], vb[i]) {
				return false
			}
		}
		return true
	case map[string]any:
		vb, ok := b.(map[string]any)
		if !ok || len(va) != len(vb) {
			return false
		}
		for key := range va {
			if !compareValues(va[key], vb[key]) {
				return false
			}
		}
		return true
	default:
		return fmt.Sprintf("%v", a) == fmt.Sprintf("%v", b)
	}
}

// ============================================================================
// Benchmark Tests
// ============================================================================

func BenchmarkIterableValueGet(b *testing.B) {
	data := map[string]any{
		"user": map[string]any{
			"name":  "Alice",
			"age":   30,
			"email": "alice@example.com",
		},
	}
	iv := newIterableValue(data)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = iv.Get("user.name")
	}
}

func BenchmarkSafeTypeAssert(b *testing.B) {
	input := 42

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = safeTypeAssert[int](input)
	}
}

// TestDefaultValues tests default value methods
func TestDefaultValues(t *testing.T) {
	data := map[string]any{
		"name": "Alice",
		"age":  30,
	}

	iv := newIterableValue(data)

	t.Run("GetStringWithDefault", func(t *testing.T) {
		result := iv.GetStringWithDefault("name", "Unknown")
		if result != "Alice" {
			t.Errorf("Expected 'Alice', got '%s'", result)
		}

		result = iv.GetStringWithDefault("email", "unknown@example.com")
		if result != "unknown@example.com" {
			t.Errorf("Expected default, got '%s'", result)
		}
	})

	t.Run("GetIntWithDefault", func(t *testing.T) {
		result := iv.GetIntWithDefault("age", 0)
		if result != 30 {
			t.Errorf("Expected 30, got %d", result)
		}

		result = iv.GetIntWithDefault("score", 100)
		if result != 100 {
			t.Errorf("Expected default 100, got %d", result)
		}
	})

	t.Run("GetFloat64WithDefault", func(t *testing.T) {
		result := iv.GetFloat64WithDefault("age", 0.0)
		if result != 30.0 {
			t.Errorf("Expected 30.0, got %f", result)
		}

		result = iv.GetFloat64WithDefault("price", 9.99)
		if result != 9.99 {
			t.Errorf("Expected default 9.99, got %f", result)
		}
	})

	t.Run("GetBoolWithDefault", func(t *testing.T) {
		result := iv.GetBoolWithDefault("active", false)
		if result != false {
			t.Errorf("Expected default false, got %v", result)
		}
	})

	t.Run("GetWithDefault", func(t *testing.T) {
		result := iv.GetWithDefault("name", "Unknown")
		if result != "Alice" {
			t.Errorf("Expected 'Alice', got %v", result)
		}

		result = iv.GetWithDefault("missing", "default_value")
		if result != "default_value" {
			t.Errorf("Expected default, got %v", result)
		}
	})
}

// TestIsSimplePropertyAccess tests simple property access detection
func TestIsSimplePropertyAccess(t *testing.T) {
	tests := []struct {
		path     string
		expected bool
	}{
		{"name", true},
		{"user_name", true},
		{"userName", true},
		{"user123", true},
		{"", false},
		{"user.name", false},
		{"user[0]", false},
		{"user-name", false},
		{"user name", false},
		{strings.Repeat("a", 65), false}, // too long
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := isSimplePropertyAccess(tt.path)
			if result != tt.expected {
				t.Errorf("isSimplePropertyAccess(%q) = %v, want %v", tt.path, result, tt.expected)
			}
		})
	}
}

// TestIterableValueExists tests Exists method
func TestIterableValueExists(t *testing.T) {
	data := map[string]any{
		"name":  "Alice",
		"age":   30,
		"email": nil,
		"user": map[string]any{
			"active": true,
		},
	}

	iv := newIterableValue(data)

	tests := []struct {
		name     string
		key      string
		expected bool
	}{
		{
			name:     "existing key",
			key:      "name",
			expected: true,
		},
		{
			name:     "null value exists",
			key:      "email",
			expected: true,
		},
		{
			name:     "nested path exists",
			key:      "user.active",
			expected: true,
		},
		{
			name:     "missing key",
			key:      "missing",
			expected: false,
		},
		{
			name:     "invalid path",
			key:      "user.invalid",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := iv.Exists(tt.key)
			if result != tt.expected {
				t.Errorf("Exists(%s) = %v; want %v", tt.key, result, tt.expected)
			}
		})
	}
}

// TestIterableValueGet tests Get method
func TestIterableValueGet(t *testing.T) {
	data := map[string]any{
		"user": map[string]any{
			"name": "Alice",
			"age":  30,
		},
		"items": []any{"a", "b", "c"},
	}

	iv := newIterableValue(data)

	tests := []struct {
		name     string
		path     string
		expected any
	}{
		{
			name:     "simple property",
			path:     "user",
			expected: map[string]any{"name": "Alice", "age": 30},
		},
		{
			name:     "nested property",
			path:     "user.name",
			expected: "Alice",
		},
		{
			name:     "array index",
			path:     "items[0]",
			expected: "a",
		},
		{
			name:     "root path",
			path:     ".",
			expected: data,
		},
		{
			name:     "empty path",
			path:     "",
			expected: data,
		},
		{
			name:     "invalid path",
			path:     "invalid.path",
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := iv.Get(tt.path)
			if !compareValues(result, tt.expected) {
				t.Errorf("Get(%s) = %v; want %v", tt.path, result, tt.expected)
			}
		})
	}
}

// TestIterableValueGetArray tests GetArray method
func TestIterableValueGetArray(t *testing.T) {
	data := map[string]any{
		"items":   []any{"a", "b", "c"},
		"numbers": []any{1, 2, 3},
		"user": map[string]any{
			"tags": []any{"developer", "golang"},
		},
	}

	iv := newIterableValue(data)

	tests := []struct {
		name        string
		key         string
		expectedLen int
	}{
		{
			name:        "existing array",
			key:         "items",
			expectedLen: 3,
		},
		{
			name:        "nested array",
			key:         "user.tags",
			expectedLen: 2,
		},
		{
			name:        "not an array",
			key:         "user",
			expectedLen: 0,
		},
		{
			name:        "missing key",
			key:         "missing",
			expectedLen: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := iv.GetArray(tt.key)
			if tt.expectedLen > 0 {
				if result == nil {
					t.Errorf("GetArray(%s) returned nil", tt.key)
				} else if len(result) != tt.expectedLen {
					t.Errorf("GetArray(%s) length = %d; want %d", tt.key, len(result), tt.expectedLen)
				}
			} else if result != nil {
				t.Errorf("GetArray(%s) = %v; want nil", tt.key, result)
			}
		})
	}
}

// TestIterableValueGetBool tests GetBool method
func TestIterableValueGetBool(t *testing.T) {
	data := map[string]any{
		"active":   true,
		"age":      30,
		"enabled":  "true",
		"verified": 1,
		"user": map[string]any{
			"admin": true,
		},
	}

	iv := newIterableValue(data)

	tests := []struct {
		name     string
		key      string
		expected bool
	}{
		{
			name:     "existing bool",
			key:      "active",
			expected: true,
		},
		{
			name:     "convert non-zero int",
			key:      "age",
			expected: true,
		},
		{
			name:     "convert string true",
			key:      "enabled",
			expected: true,
		},
		{
			name:     "nested path",
			key:      "user.admin",
			expected: true,
		},
		{
			name:     "missing key",
			key:      "missing",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := iv.GetBool(tt.key)
			if result != tt.expected {
				t.Errorf("GetBool(%s) = %v; want %v", tt.key, result, tt.expected)
			}
		})
	}
}

// TestIterableValueGetFloat64 tests GetFloat64 method
func TestIterableValueGetFloat64(t *testing.T) {
	data := map[string]any{
		"price":  19.99,
		"age":    30,
		"rating": "4.5",
		"user": map[string]any{
			"score": 95.5,
		},
	}

	iv := newIterableValue(data)

	tests := []struct {
		name     string
		key      string
		expected float64
	}{
		{
			name:     "existing float",
			key:      "price",
			expected: 19.99,
		},
		{
			name:     "convert int",
			key:      "age",
			expected: 30.0,
		},
		{
			name:     "convert string",
			key:      "rating",
			expected: 4.5,
		},
		{
			name:     "nested path",
			key:      "user.score",
			expected: 95.5,
		},
		{
			name:     "missing key",
			key:      "missing",
			expected: 0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := iv.GetFloat64(tt.key)
			if result != tt.expected {
				t.Errorf("GetFloat64(%s) = %f; want %f", tt.key, result, tt.expected)
			}
		})
	}
}

// TestIterableValueGetInt tests GetInt method
func TestIterableValueGetInt(t *testing.T) {
	data := map[string]any{
		"age":    30,
		"score":  95.5,
		"count":  "100",
		"active": true,
		"user": map[string]any{
			"id": 42,
		},
	}

	iv := newIterableValue(data)

	tests := []struct {
		name     string
		key      string
		expected int
	}{
		{
			name:     "existing int",
			key:      "age",
			expected: 30,
		},
		{
			name:     "convert float",
			key:      "score",
			expected: 0, // Float can't convert cleanly to int
		},
		{
			name:     "convert string",
			key:      "count",
			expected: 100,
		},
		{
			name:     "convert bool true",
			key:      "active",
			expected: 1,
		},
		{
			name:     "nested path",
			key:      "user.id",
			expected: 42,
		},
		{
			name:     "missing key",
			key:      "missing",
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := iv.GetInt(tt.key)
			if result != tt.expected {
				t.Errorf("GetInt(%s) = %d; want %d", tt.key, result, tt.expected)
			}
		})
	}
}

// TestIterableValueGetObject tests GetObject method
func TestIterableValueGetObject(t *testing.T) {
	data := map[string]any{
		"user": map[string]any{
			"name": "Alice",
			"age":  30,
		},
		"settings": map[string]any{
			"theme": "dark",
		},
	}

	iv := newIterableValue(data)

	tests := []struct {
		name        string
		key         string
		expectValue bool
	}{
		{
			name:        "existing object",
			key:         "user",
			expectValue: true,
		},
		{
			name:        "nested object",
			key:         "settings",
			expectValue: true,
		},
		{
			name:        "not an object",
			key:         "items",
			expectValue: false,
		},
		{
			name:        "missing key",
			key:         "missing",
			expectValue: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := iv.GetObject(tt.key)
			if tt.expectValue {
				if result == nil {
					t.Errorf("GetObject(%s) returned nil", tt.key)
				}
			} else if result != nil {
				t.Errorf("GetObject(%s) = %v; want nil", tt.key, result)
			}
		})
	}
}

// TestIterableValueGetString tests GetString method
func TestIterableValueGetString(t *testing.T) {
	data := map[string]any{
		"name":   "Alice",
		"age":    30,
		"active": true,
		"user": map[string]any{
			"email": "alice@example.com",
		},
	}

	iv := newIterableValue(data)

	tests := []struct {
		name     string
		key      string
		expected string
	}{
		{
			name:     "existing string",
			key:      "name",
			expected: "Alice",
		},
		{
			name:     "convert int",
			key:      "age",
			expected: "30",
		},
		{
			name:     "convert bool",
			key:      "active",
			expected: "true",
		},
		{
			name:     "nested path",
			key:      "user.email",
			expected: "alice@example.com",
		},
		{
			name:     "missing key",
			key:      "missing",
			expected: "",
		},
		{
			name:     "path not found",
			key:      "user.invalid",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := iv.GetString(tt.key)
			if result != tt.expected {
				t.Errorf("GetString(%s) = %s; want %s", tt.key, result, tt.expected)
			}
		})
	}
}

// TestIterableValueIsEmpty tests IsEmpty method
func TestIterableValueIsEmpty(t *testing.T) {
	data := map[string]any{
		"name":    "",
		"items":   []any{},
		"profile": map[string]any{},
		"active":  true,
		"user": map[string]any{
			"tags": []any{},
		},
	}

	iv := newIterableValue(data)

	tests := []struct {
		name     string
		key      string
		expected bool
	}{
		{
			name:     "empty string",
			key:      "name",
			expected: true,
		},
		{
			name:     "empty array",
			key:      "items",
			expected: true,
		},
		{
			name:     "empty object",
			key:      "profile",
			expected: true,
		},
		{
			name:     "non-empty bool",
			key:      "active",
			expected: false,
		},
		{
			name:     "nested empty array",
			key:      "user.tags",
			expected: true,
		},
		{
			name:     "missing key",
			key:      "missing",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := iv.IsEmpty(tt.key)
			if result != tt.expected {
				t.Errorf("IsEmpty(%s) = %v; want %v", tt.key, result, tt.expected)
			}
		})
	}
}

// TestIterableValueIsNull tests IsNull method
func TestIterableValueIsNull(t *testing.T) {
	data := map[string]any{
		"name":  "Alice",
		"email": nil,
		"user": map[string]any{
			"deleted": nil,
		},
	}

	iv := newIterableValue(data)

	tests := []struct {
		name     string
		key      string
		expected bool
	}{
		{
			name:     "non-null value",
			key:      "name",
			expected: false,
		},
		{
			name:     "null value",
			key:      "email",
			expected: true,
		},
		{
			name:     "nested null",
			key:      "user.deleted",
			expected: true,
		},
		{
			name:     "missing key",
			key:      "missing",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := iv.IsNull(tt.key)
			if result != tt.expected {
				t.Errorf("IsNull(%s) = %v; want %v", tt.key, result, tt.expected)
			}
		})
	}
}

// TestIterableValue_BackwardCompatibility tests that simple key lookup still works
func TestIterableValue_BackwardCompatibility(t *testing.T) {
	jsonStr := `{
		"name": "Test",
		"value": 42,
		"flag": true,
		"items": [1, 2, 3]
	}`

	var data map[string]any
	if err := json.Unmarshal([]byte(jsonStr), &data); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	iv := &IterableValue{data: data}

	// Test simple key access (without dots)
	if name := iv.GetString("name"); name != "Test" {
		t.Errorf("GetString(name) = %q, want 'Test'", name)
	}

	if value := iv.GetInt("value"); value != 42 {
		t.Errorf("GetInt(value) = %d, want 42", value)
	}

	if flag := iv.GetBool("flag"); !flag {
		t.Errorf("GetBool(flag) = false, want true")
	}

	items := iv.GetArray("items")
	if items == nil || len(items) != 3 {
		t.Errorf("GetArray(items) = %v, want array of length 3", items)
	}
}

// TestIterableValue_EdgeCases tests edge cases and error conditions
func TestIterableValue_EdgeCases(t *testing.T) {
	jsonStr := `{
		"emptyArray": [],
		"emptyObject": {},
		"nullField": null,
		"array": [1, 2, 3]
	}`

	var data map[string]any
	if err := json.Unmarshal([]byte(jsonStr), &data); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	iv := &IterableValue{data: data}

	t.Run("EmptyArray", func(t *testing.T) {
		arr := iv.GetArray("emptyArray")
		if arr == nil || len(arr) != 0 {
			t.Errorf("GetArray(emptyArray) = %v, want empty array", arr)
		}

		if !iv.IsEmpty("emptyArray") {
			t.Error("IsEmpty(emptyArray) should return true")
		}
	})

	t.Run("EmptyObject", func(t *testing.T) {
		obj := iv.GetObject("emptyObject")
		if obj == nil || len(obj) != 0 {
			t.Errorf("GetObject(emptyObject) = %v, want empty object", obj)
		}
	})

	t.Run("NonExistentPath", func(t *testing.T) {
		if iv.Exists("nonexistent.path") {
			t.Error("Exists(nonexistent.path) should return false")
		}

		if val := iv.GetString("nonexistent.path"); val != "" {
			t.Errorf("GetString(nonexistent.path) = %q, want empty string", val)
		}
	})

	t.Run("InvalidArrayIndex", func(t *testing.T) {
		if val := iv.GetString("array[10]"); val != "" {
			t.Errorf("GetString(array[10]) = %q, want empty string (out of bounds)", val)
		}

		if val := iv.GetString("array[-10]"); val != "" {
			t.Errorf("GetString(array[-10]) = %q, want empty string (out of bounds)", val)
		}
	})
}

// TestIterableValue_ForeachNestedWithPath tests ForeachNested with path navigation
func TestIterableValue_ForeachNestedWithPath(t *testing.T) {
	jsonStr := `{
		"users": [
			{"name": "Alice", "age": 25},
			{"name": "Bob", "age": 30},
			{"name": "Charlie", "age": 35}
		]
	}`

	var data map[string]any
	if err := json.Unmarshal([]byte(jsonStr), &data); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	iv := &IterableValue{data: data}

	t.Run("ForeachOverArray", func(t *testing.T) {
		usersArray := iv.GetArray("users")
		if usersArray == nil || len(usersArray) != 3 {
			t.Fatalf("GetArray(users) = %v, want array of length 3", usersArray)
		}

		count := 0
		for _, user := range usersArray {
			userIV := &IterableValue{data: user}
			count++
			name := userIV.GetString("name")
			if name == "" {
				t.Errorf("Expected non-empty name at index %d", count-1)
			}
		}

		if count != 3 {
			t.Errorf("Iterated over %d users, want 3", count)
		}
	})

	t.Run("ForeachNestedRecursive", func(t *testing.T) {
		// Test that ForeachNested recursively iterates over all nested values
		count := 0
		iv.ForeachNested("users", func(key any, item *IterableValue) {
			count++
		})

		// ForeachNested recursively iterates, so count should be > 3
		if count < 3 {
			t.Errorf("ForeachRecursive count = %d, want at least 3 (it's recursive)", count)
		}
	})
}

// TestIterableValue_MixedPathAndKeyAccess tests mixing path and key access
func TestIterableValue_MixedPathAndKeyAccess(t *testing.T) {
	jsonStr := `{
		"data": {
			"user": {
				"name": "Test User",
				"settings": {
					"theme": "dark"
				}
			}
		}
	}`

	var data map[string]any
	if err := json.Unmarshal([]byte(jsonStr), &data); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	iv := &IterableValue{data: data}

	// Access with full path
	if name := iv.GetString("data.user.name"); name != "Test User" {
		t.Errorf("GetString(data.user.name) = %q, want 'Test User'", name)
	}

	// Access nested object then use simple key
	userObj := iv.GetObject("data.user")
	if userObj == nil {
		t.Fatal("GetObject(data.user) returned nil")
	}

	userIV := &IterableValue{data: userObj}
	if name := userIV.GetString("name"); name != "Test User" {
		t.Errorf("Nested GetString(name) = %q, want 'Test User'", name)
	}

	if theme := userIV.GetString("settings.theme"); theme != "dark" {
		t.Errorf("Nested GetString(settings.theme) = %q, want 'dark'", theme)
	}
}

// TestIterableValue_PathNavigation tests the path navigation functionality
func TestIterableValue_PathNavigation(t *testing.T) {
	jsonStr := `{
		"user": {
			"name": "John Doe",
			"age": 30,
			"active": true,
			"score": 95.5,
			"address": {
				"city": "New York",
				"zip": "10001"
			},
			"hobbies": ["reading", "gaming", "coding"],
			"posts": [
				{
					"id": 1,
					"title": "First Post",
					"tags": ["intro", "hello"]
				},
				{
					"id": 2,
					"title": "Second Post",
					"tags": ["update", "news"]
				}
			]
		},
		"thumbnails": [
			{"url": "small.jpg", "width": 100},
			{"url": "medium.jpg", "width": 300},
			{"url": "large.jpg", "width": 800}
		]
	}`

	var data map[string]any
	if err := json.Unmarshal([]byte(jsonStr), &data); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	iv := &IterableValue{data: data}

	t.Run("SimplePropertyAccess", func(t *testing.T) {
		tests := []struct {
			name     string
			path     string
			expected string
		}{
			{"Single key", "user.name", "John Doe"},
			{"Nested path", "user.address.city", "New York"},
			{"Deep nested", "user.address.zip", "10001"},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				result := iv.GetString(tt.path)
				if result != tt.expected {
					t.Errorf("GetString(%q) = %q, want %q", tt.path, result, tt.expected)
				}
			})
		}
	})

	t.Run("ArrayIndexAccess", func(t *testing.T) {
		tests := []struct {
			name     string
			path     string
			expected string
		}{
			{"First element", "user.hobbies[0]", "reading"},
			{"Second element", "user.hobbies[1]", "gaming"},
			{"Last element", "user.hobbies[2]", "coding"},
			{"Nested array", "user.posts[0].title", "First Post"},
			{"Nested array deep", "user.posts[1].tags[0]", "update"},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				result := iv.GetString(tt.path)
				if result != tt.expected {
					t.Errorf("GetString(%q) = %q, want %q", tt.path, result, tt.expected)
				}
			})
		}
	})

	t.Run("NegativeArrayIndex", func(t *testing.T) {
		tests := []struct {
			name     string
			path     string
			expected string
		}{
			{"Last element with -1", "user.hobbies[-1]", "coding"},
			{"Second to last with -2", "user.hobbies[-2]", "gaming"},
			{"First from end", "user.posts[-1].title", "Second Post"},
			{"Thumbnail last", "thumbnails[-1].url", "large.jpg"},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				result := iv.GetString(tt.path)
				if result != tt.expected {
					t.Errorf("GetString(%q) = %q, want %q", tt.path, result, tt.expected)
				}
			})
		}
	})

	t.Run("TypeSpecificGetters", func(t *testing.T) {
		t.Run("GetInt", func(t *testing.T) {
			if age := iv.GetInt("user.age"); age != 30 {
				t.Errorf("GetInt(user.age) = %d, want 30", age)
			}
			if postID := iv.GetInt("user.posts[0].id"); postID != 1 {
				t.Errorf("GetInt(user.posts[0].id) = %d, want 1", postID)
			}
		})

		t.Run("GetFloat64", func(t *testing.T) {
			if score := iv.GetFloat64("user.score"); score != 95.5 {
				t.Errorf("GetFloat64(user.score) = %f, want 95.5", score)
			}
		})

		t.Run("GetBool", func(t *testing.T) {
			if active := iv.GetBool("user.active"); !active {
				t.Errorf("GetBool(user.active) = false, want true")
			}
		})

		t.Run("GetArray", func(t *testing.T) {
			hobbies := iv.GetArray("user.hobbies")
			if hobbies == nil || len(hobbies) != 3 {
				t.Errorf("GetArray(user.hobbies) = %v, want array of length 3", hobbies)
			}
		})

		t.Run("GetObject", func(t *testing.T) {
			address := iv.GetObject("user.address")
			if address == nil {
				t.Errorf("GetObject(user.address) = nil, want map")
			}
		})
	})

	t.Run("DefaultValues", func(t *testing.T) {
		t.Run("GetStringWithDefault", func(t *testing.T) {
			if val := iv.GetStringWithDefault("user.nonexistent", "default"); val != "default" {
				t.Errorf("GetStringWithDefault(nonexistent) = %q, want 'default'", val)
			}
			if val := iv.GetStringWithDefault("user.name", "default"); val != "John Doe" {
				t.Errorf("GetStringWithDefault(user.name) = %q, want 'John Doe'", val)
			}
		})

		t.Run("GetIntWithDefault", func(t *testing.T) {
			if val := iv.GetIntWithDefault("user.nonexistent", 99); val != 99 {
				t.Errorf("GetIntWithDefault(nonexistent) = %d, want 99", val)
			}
			if val := iv.GetIntWithDefault("user.age", 99); val != 30 {
				t.Errorf("GetIntWithDefault(user.age) = %d, want 30", val)
			}
		})
	})

	t.Run("ExistsAndNull", func(t *testing.T) {
		t.Run("Exists", func(t *testing.T) {
			if !iv.Exists("user.name") {
				t.Error("Exists(user.name) = false, want true")
			}
			if iv.Exists("user.nonexistent") {
				t.Error("Exists(user.nonexistent) = true, want false")
			}
		})

		t.Run("IsNull", func(t *testing.T) {
			if iv.IsNull("user.name") {
				t.Error("IsNull(user.name) = true, want false")
			}
			// Note: we can't test null values in this JSON as we don't have any
		})
	})

	t.Run("GetGeneric", func(t *testing.T) {
		t.Run("GetString", func(t *testing.T) {
			if val := iv.Get("user.name"); val != "John Doe" {
				t.Errorf("Get(user.name) = %v, want 'John Doe'", val)
			}
		})

		t.Run("GetNested", func(t *testing.T) {
			if val := iv.Get("user.address.city"); val != "New York" {
				t.Errorf("Get(user.address.city) = %v, want 'New York'", val)
			}
		})

		t.Run("GetArrayElement", func(t *testing.T) {
			if val := iv.Get("user.hobbies[0]"); val != "reading" {
				t.Errorf("Get(user.hobbies[0]) = %v, want 'reading'", val)
			}
		})
	})
}

// TestIterableValue_RealWorldScenario tests real-world JSON parsing scenarios
func TestIterableValue_RealWorldScenario(t *testing.T) {
	// Simulate YouTube API response structure
	jsonStr := `{
		"contents": {
			"twoColumnBrowseResultsRenderer": {
				"tabs": [
					{
						"tabRenderer": {
							"title": "Home",
							"selected": false
						}
					},
					{
						"tabRenderer": {
							"title": "Videos",
							"selected": true,
							"content": {
								"richGridRenderer": {
									"contents": [
										{
											"richItemRenderer": {
												"content": {
													"videoRenderer": {
														"videoId": "abc123",
														"title": {
															"runs": [{"text": "Test Video"}]
														},
														"thumbnail": {
															"thumbnails": [
																{"url": "thumb1.jpg"},
																{"url": "thumb2.jpg"}
															]
														}
													}
												}
											}
										}
									]
								}
							}
						}
					}
				]
			}
		}
	}`

	var data map[string]any
	if err := json.Unmarshal([]byte(jsonStr), &data); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	iv := &IterableValue{data: data}

	t.Run("DeepNestedExtraction", func(t *testing.T) {
		videoID := iv.GetString("contents.twoColumnBrowseResultsRenderer.tabs[1].tabRenderer.content.richGridRenderer.contents[0].richItemRenderer.content.videoRenderer.videoId")
		if videoID != "abc123" {
			t.Errorf("Video ID = %q, want 'abc123'", videoID)
		}

		title := iv.GetString("contents.twoColumnBrowseResultsRenderer.tabs[1].tabRenderer.content.richGridRenderer.contents[0].richItemRenderer.content.videoRenderer.title.runs[0].text")
		if title != "Test Video" {
			t.Errorf("Title = %q, want 'Test Video'", title)
		}

		thumbnailURL := iv.GetString("contents.twoColumnBrowseResultsRenderer.tabs[1].tabRenderer.content.richGridRenderer.contents[0].richItemRenderer.content.videoRenderer.thumbnail.thumbnails[-1].url")
		if thumbnailURL != "thumb2.jpg" {
			t.Errorf("Thumbnail URL = %q, want 'thumb2.jpg'", thumbnailURL)
		}
	})

	t.Run("TabSelection", func(t *testing.T) {
		selected := iv.GetBool("contents.twoColumnBrowseResultsRenderer.tabs[1].tabRenderer.selected")
		if !selected {
			t.Error("Second tab should be selected")
		}

		notSelected := iv.GetBool("contents.twoColumnBrowseResultsRenderer.tabs[0].tabRenderer.selected")
		if notSelected {
			t.Error("First tab should not be selected")
		}
	})
}

// TestIteratorHasNext tests Iterator.HasNext method
func TestIteratorHasNext(t *testing.T) {
	processor, _ := New()
	defer processor.Close()

	t.Run("array iterator", func(t *testing.T) {
		data := []any{1, 2, 3}
		it := NewIterator(data)

		count := 0
		for it.HasNext() {
			it.Next()
			count++
		}

		if count != 3 {
			t.Errorf("Expected 3 iterations, got %d", count)
		}
	})

	t.Run("object iterator", func(t *testing.T) {
		data := map[string]any{"a": 1, "b": 2, "c": 3}
		it := NewIterator(data)

		count := 0
		for it.HasNext() {
			it.Next()
			count++
		}

		if count != 3 {
			t.Errorf("Expected 3 iterations, got %d", count)
		}
	})

	t.Run("empty array", func(t *testing.T) {
		data := []any{}
		it := NewIterator(data)

		if it.HasNext() {
			t.Error("Expected no elements in empty array")
		}
	})
}

// TestIteratorNext tests Iterator.Next method
func TestIteratorNext(t *testing.T) {
	processor, _ := New()
	defer processor.Close()

	t.Run("array elements", func(t *testing.T) {
		data := []any{"a", "b", "c"}
		it := NewIterator(data)

		expected := []any{"a", "b", "c"}
		for i := 0; i < len(expected); i++ {
			result, ok := it.Next()
			if !ok {
				t.Errorf("Expected element at index %d", i)
			}
			if result != expected[i] {
				t.Errorf("Element %d = %v; want %v", i, result, expected[i])
			}
		}

		// Should return false after exhausting
		_, ok := it.Next()
		if ok {
			t.Error("Expected false after exhaustion")
		}
	})

	t.Run("object values", func(t *testing.T) {
		data := map[string]any{"a": 1, "b": 2}
		it := NewIterator(data)

		count := 0
		for it.HasNext() {
			_, ok := it.Next()
			if !ok {
				t.Error("Expected valid element")
			}
			count++
		}

		if count != 2 {
			t.Errorf("Expected 2 iterations, got %d", count)
		}
	})
}

// TestNewIterableValue tests creating IterableValue from different data types (internal)
func TestNewIterableValue(t *testing.T) {
	tests := []struct {
		name  string
		data  any
		valid bool
	}{
		{
			name:  "from map",
			data:  map[string]any{"name": "Alice"},
			valid: true,
		},
		{
			name:  "from array",
			data:  []any{1, 2, 3},
			valid: true,
		},
		{
			name:  "from string",
			data:  "hello",
			valid: true,
		},
		{
			name:  "from nil",
			data:  nil,
			valid: true,
		},
		{
			name:  "from int",
			data:  42,
			valid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			iv := newIterableValue(tt.data)
			if iv == nil {
				t.Error("Expected non-nil IterableValue")
			}
			// Just verify the data was set; equality checks fail for maps
			if tt.valid && iv.data == nil && tt.data != nil {
				t.Error("Expected data to be set")
			}
		})
	}
}

// TestSafeTypeAssert tests SafeTypeAssert function
func TestSafeTypeAssert(t *testing.T) {
	tests := []struct {
		name          string
		input         any
		targetType    string
		shouldSucceed bool
		validate      func(t *testing.T, result any)
	}{
		{
			name:          "same type int",
			input:         42,
			targetType:    "int",
			shouldSucceed: true,
			validate: func(t *testing.T, result any) {
				if result.(int) != 42 {
					t.Errorf("Expected 42, got %v", result)
				}
			},
		},
		{
			name:          "same type string",
			input:         "hello",
			targetType:    "string",
			shouldSucceed: true,
			validate: func(t *testing.T, result any) {
				if result.(string) != "hello" {
					t.Errorf("Expected 'hello', got %v", result)
				}
			},
		},
		{
			name:          "nil value",
			input:         nil,
			targetType:    "string",
			shouldSucceed: false,
		},
		{
			name:          "int to float64 (convertible)",
			input:         42,
			targetType:    "float64",
			shouldSucceed: true,
			validate: func(t *testing.T, result any) {
				if result.(float64) != 42.0 {
					t.Errorf("Expected 42.0, got %v", result)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			switch tt.targetType {
			case "int":
				result, ok := safeTypeAssert[int](tt.input)
				if ok != tt.shouldSucceed {
					t.Errorf("safeTypeAssert[int](%v) success = %v; want %v", tt.input, ok, tt.shouldSucceed)
				}
				if tt.validate != nil {
					tt.validate(t, result)
				}
			case "string":
				result, ok := safeTypeAssert[string](tt.input)
				if ok != tt.shouldSucceed {
					t.Errorf("safeTypeAssert[string](%v) success = %v; want %v", tt.input, ok, tt.shouldSucceed)
				}
				if tt.validate != nil {
					tt.validate(t, result)
				}
			case "float64":
				result, ok := safeTypeAssert[float64](tt.input)
				if ok != tt.shouldSucceed {
					t.Errorf("safeTypeAssert[float64](%v) success = %v; want %v", tt.input, ok, tt.shouldSucceed)
				}
				if tt.validate != nil {
					tt.validate(t, result)
				}
			}
		})
	}
}

// TestIterableValueIsNullAndEmptyData tests IsNullData and IsEmptyData (companion methods)
func TestIterableValueIsNullAndEmptyData(t *testing.T) {
	t.Run("IsNullData", func(t *testing.T) {
		if !newIterableValue(nil).IsNullData() {
			t.Error("IsNullData should return true for nil data")
		}
		if newIterableValue(map[string]any{"a": 1}).IsNullData() {
			t.Error("IsNullData should return false for non-nil data")
		}
	})

	t.Run("IsEmptyData", func(t *testing.T) {
		tests := []struct {
			data any
			want bool
		}{
			{nil, true},
			{[]any{}, true},
			{map[string]any{}, true},
			{"", true},
			{[]any{1}, false},
			{map[string]any{"a": 1}, false},
			{"x", false},
			{42, false},
		}
		for _, tt := range tests {
			iv := newIterableValue(tt.data)
			if got := iv.IsEmptyData(); got != tt.want {
				t.Errorf("IsEmptyData(%v) = %v, want %v", tt.data, got, tt.want)
			}
		}
	})
}

// TestIterableValueForeachNested tests ForeachNested iteration
func TestIterableValueForeachNested(t *testing.T) {
	t.Run("iterate array", func(t *testing.T) {
		data := map[string]any{
			"items": []any{1, 2, 3},
		}
		iv := newIterableValue(data)

		var count int
		iv.ForeachNested("items", func(key any, item *IterableValue) {
			count++
		})
		if count != 3 {
			t.Fatalf("expected 3 iterations, got %d", count)
		}
	})

	t.Run("iterate root", func(t *testing.T) {
		data := map[string]any{"x": 1}
		iv := newIterableValue(data)

		count := 0
		iv.ForeachNested("", func(key any, item *IterableValue) {
			count++
		})
		if count != 1 {
			t.Fatalf("expected 1 iteration, got %d", count)
		}
	})

	t.Run("invalid path", func(t *testing.T) {
		data := map[string]any{"a": 1}
		iv := newIterableValue(data)

		count := 0
		iv.ForeachNested("missing.path", func(key any, item *IterableValue) {
			count++
		})
		if count != 0 {
			t.Fatalf("expected 0 iterations for invalid path, got %d", count)
		}
	})
}

// TestIterableValueWithDefaultComplexPath tests *WithDefault methods with complex paths
func TestIterableValueWithDefaultComplexPath(t *testing.T) {
	data := map[string]any{
		"user": map[string]any{
			"name":   "Alice",
			"age":    float64(30),
			"active": true,
			"score":  float64(95.5),
		},
	}
	iv := newIterableValue(data)

	t.Run("GetStringWithDefault complex", func(t *testing.T) {
		if got := iv.GetStringWithDefault("user.name", "N/A"); got != "Alice" {
			t.Errorf("got %q, want Alice", got)
		}
		if got := iv.GetStringWithDefault("user.missing", "N/A"); got != "N/A" {
			t.Errorf("got %q, want N/A", got)
		}
	})

	t.Run("GetIntWithDefault complex", func(t *testing.T) {
		if got := iv.GetIntWithDefault("user.age", 0); got != 30 {
			t.Errorf("got %d, want 30", got)
		}
		if got := iv.GetIntWithDefault("user.missing", -1); got != -1 {
			t.Errorf("got %d, want -1", got)
		}
	})

	t.Run("GetFloat64WithDefault complex", func(t *testing.T) {
		if got := iv.GetFloat64WithDefault("user.score", 0); got != 95.5 {
			t.Errorf("got %f, want 95.5", got)
		}
		if got := iv.GetFloat64WithDefault("user.missing", -1.0); got != -1.0 {
			t.Errorf("got %f, want -1", got)
		}
	})

	t.Run("GetBoolWithDefault complex", func(t *testing.T) {
		if got := iv.GetBoolWithDefault("user.active", false); got != true {
			t.Errorf("got %v, want true", got)
		}
		if got := iv.GetBoolWithDefault("user.missing", true); got != true {
			t.Errorf("got %v, want true (default)", got)
		}
	})

	t.Run("GetWithDefault complex nil data", func(t *testing.T) {
		iv := newIterableValue("string data")
		if got := iv.GetWithDefault("any.key", "fallback"); got != "fallback" {
			t.Errorf("got %v, want fallback", got)
		}
	})
}

// ============================================================================
// Additional coverage tests for low-coverage functions
// ============================================================================

// TestForeachWithError tests the package-level ForeachWithError function.
func TestForeachWithError(t *testing.T) {
	tests := []struct {
		name        string
		jsonStr     string
		path        string
		expectErr   bool
		callbackErr bool
	}{
		{
			name:      "successful iteration over array",
			jsonStr:   `{"items":[1,2,3]}`,
			path:      "items",
			expectErr: false,
		},
		{
			name:        "error propagation from callback",
			jsonStr:     `{"items":[10,20,30]}`,
			path:        "items",
			expectErr:   true,
			callbackErr: true,
		},
		{
			name:      "invalid JSON returns error",
			jsonStr:   "{invalid}",
			path:      ".",
			expectErr: true,
		},
		{
			name:      "invalid path returns error",
			jsonStr:   `{"items":[1,2,3]}`,
			path:      "nonexistent",
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var collected []any
			callCount := 0
			err := ForeachWithError(tt.jsonStr, tt.path, func(key any, item *IterableValue) error {
				callCount++
				if tt.callbackErr && callCount > 1 {
					return fmt.Errorf("callback error at item %d", callCount)
				}
				collected = append(collected, item.GetData())
				return nil
			})

			if tt.expectErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if len(collected) != 3 {
					t.Errorf("collected %d items, want 3", len(collected))
				}
			}
		})
	}
}

// TestForeachNestedWithErrorCoverage tests the package-level ForeachNestedWithError function.
func TestForeachNestedWithErrorCoverage(t *testing.T) {
	tests := []struct {
		name        string
		jsonStr     string
		expectCount int
		expectErr   bool
		callbackErr bool
	}{
		{
			name:        "successful nested iteration",
			jsonStr:     `{"a":1,"b":{"c":2}}`,
			expectCount: 3,
			expectErr:   false,
		},
		{
			name:        "error propagation from callback",
			jsonStr:     `{"items":[1,2,3]}`,
			expectCount: 1,
			expectErr:   true,
			callbackErr: true,
		},
		{
			name:        "flat object iteration",
			jsonStr:     `{"x":10,"y":20}`,
			expectCount: 2,
			expectErr:   false,
		},
		{
			name:      "invalid JSON returns error",
			jsonStr:   "bad json",
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			count := 0
			err := ForeachNestedWithError(tt.jsonStr, func(key any, item *IterableValue) error {
				count++
				if tt.callbackErr {
					return fmt.Errorf("nested callback error")
				}
				return nil
			})

			if tt.expectErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if tt.expectCount > 0 && count != tt.expectCount {
					t.Errorf("callback called %d times, want %d", count, tt.expectCount)
				}
			}
		})
	}
}

// TestForeachWithPathAndIterator tests the package-level ForeachWithPathAndIterator function.
func TestForeachWithPathAndIterator(t *testing.T) {
	tests := []struct {
		name        string
		jsonStr     string
		path        string
		expectPaths []string
		breakEarly  bool
	}{
		{
			name:        "path tracking over array",
			jsonStr:     `{"items":[{"id":1},{"id":2},{"id":3}]}`,
			path:        "items",
			expectPaths: []string{"[0]", "[1]", "[2]"},
		},
		{
			name:        "break control stops iteration",
			jsonStr:     `{"items":[10,20,30,40,50]}`,
			path:        "items",
			expectPaths: []string{"[0]"},
			breakEarly:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var paths []string
			err := ForeachWithPathAndIterator(tt.jsonStr, tt.path, func(key any, item *IterableValue, currentPath string) IteratorControl {
				paths = append(paths, currentPath)
				if tt.breakEarly {
					return IteratorBreak
				}
				return IteratorNormal
			})

			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if len(paths) != len(tt.expectPaths) {
				t.Errorf("got %d paths %v, want %d paths %v", len(paths), paths, len(tt.expectPaths), tt.expectPaths)
			}
		})
	}
}

// TestBatchIteratorTotalBatches tests TotalBatches with edge cases including zero batch size.
func TestBatchIteratorTotalBatches(t *testing.T) {
	tests := []struct {
		name      string
		dataLen   int
		batchSize int
		expected  int
	}{
		{
			name:      "normal division with remainder",
			dataLen:   5,
			batchSize: 2,
			expected:  3,
		},
		{
			name:      "exact division",
			dataLen:   6,
			batchSize: 3,
			expected:  2,
		},
		{
			name:      "single element",
			dataLen:   1,
			batchSize: 5,
			expected:  1,
		},
		{
			name:      "empty data",
			dataLen:   0,
			batchSize: 5,
			expected:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := make([]any, tt.dataLen)
			for i := range data {
				data[i] = i + 1
			}
			cfg := DefaultConfig()
			cfg.MaxBatchSize = tt.batchSize
			it := NewBatchIterator(data, cfg)

			total := it.TotalBatches()
			if total != tt.expected {
				t.Errorf("TotalBatches() = %d, want %d", total, tt.expected)
			}
		})
	}

	t.Run("batch size zero returns zero", func(t *testing.T) {
		data := []any{1, 2, 3}
		it := &BatchIterator{
			data:      data,
			batchSize: 0,
			current:   0,
		}
		total := it.TotalBatches()
		if total != 0 {
			t.Errorf("TotalBatches() with batchSize=0 = %d, want 0", total)
		}
	})

	t.Run("negative batch size returns zero", func(t *testing.T) {
		data := []any{1, 2, 3}
		it := &BatchIterator{
			data:      data,
			batchSize: -1,
			current:   0,
		}
		total := it.TotalBatches()
		if total != 0 {
			t.Errorf("TotalBatches() with batchSize=-1 = %d, want 0", total)
		}
	})
}

// TestParallelIteratorCloseCoverage verifies Close does not panic in various states.
func TestParallelIteratorCloseCoverage(t *testing.T) {
	t.Run("close without processing", func(t *testing.T) {
		data := []any{1, 2, 3}
		it := NewParallelIterator(data)
		it.Close()
	})

	t.Run("close after processing", func(t *testing.T) {
		data := []any{1, 2, 3}
		it := NewParallelIterator(data)
		err := it.ForEach(func(i int, v any) error {
			return nil
		})
		if err != nil {
			t.Fatalf("ForEach error: %v", err)
		}
		it.Close()
	})

	t.Run("double close does not panic", func(t *testing.T) {
		data := []any{1, 2}
		it := NewParallelIterator(data)
		it.Close()
		it.Close()
	})
}

// TestStreamIteratorSingleValue tests StreamIterator.Next with non-array input.
func TestStreamIteratorSingleValue(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		expectVal any
	}{
		{
			name:      "single string value",
			input:     `"hello"`,
			expectVal: "hello",
		},
		{
			name:      "single number value",
			input:     "42",
			expectVal: float64(42),
		},
		{
			name:      "single boolean false",
			input:     "false",
			expectVal: false,
		},
		{
			name:      "single boolean true",
			input:     "true",
			expectVal: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			it := NewStreamIterator(strings.NewReader(tt.input))

			if !it.Next() {
				t.Fatalf("first Next() returned false, want true")
			}

			val := it.Value()
			if !compareValues(val, tt.expectVal) {
				t.Errorf("Value() = %v (%T), want %v (%T)", val, val, tt.expectVal, tt.expectVal)
			}

			if it.Next() {
				t.Error("second Next() returned true, want false")
			}
		})
	}
}

// TestGetBoolWithDefault tests GetBoolWithDefault through Foreach iteration and direct usage.
func TestGetBoolWithDefault(t *testing.T) {
	tests := []struct {
		name       string
		jsonStr    string
		key        string
		defaultVal bool
		expected   []bool
	}{
		{
			name:       "bool values in array items",
			jsonStr:    `{"items":[{"active":true},{"active":false}]}`,
			key:        "active",
			defaultVal: false,
			expected:   []bool{true, false},
		},
		{
			name:       "missing key returns default true",
			jsonStr:    `{"items":[{"name":"a"},{"name":"b"}]}`,
			key:        "active",
			defaultVal: true,
			expected:   []bool{true, true},
		},
		{
			name:       "coerced string yes returns true",
			jsonStr:    `{"items":[{"active":"yes"},{"active":"no"}]}`,
			key:        "active",
			defaultVal: false,
			expected:   []bool{true, false},
		},
		{
			name:       "non-zero number coerced to true",
			jsonStr:    `{"items":[{"active":42}]}`,
			key:        "active",
			defaultVal: false,
			expected:   []bool{true},
		},
		{
			name:       "non-coercible string returns default",
			jsonStr:    `{"items":[{"active":"maybe"}]}`,
			key:        "active",
			defaultVal: false,
			expected:   []bool{false},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var results []bool
			ForeachWithPath(tt.jsonStr, "items", func(key any, item *IterableValue) {
				got := item.GetBoolWithDefault(tt.key, tt.defaultVal)
				results = append(results, got)
			})

			if len(results) != len(tt.expected) {
				t.Fatalf("got %d results, want %d", len(results), len(tt.expected))
			}
			for i, got := range results {
				if got != tt.expected[i] {
					t.Errorf("result[%d] = %v, want %v", i, got, tt.expected[i])
				}
			}
		})
	}

	t.Run("complex path missing key returns default", func(t *testing.T) {
		data := map[string]any{
			"user": map[string]any{
				"name": "Alice",
			},
		}
		iv := newIterableValue(data)
		if got := iv.GetBoolWithDefault("user.active", false); got != false {
			t.Errorf("got %v, want false (default for missing nested key)", got)
		}
	})

	t.Run("complex path existing key returns value", func(t *testing.T) {
		data := map[string]any{
			"user": map[string]any{
				"active": true,
			},
		}
		iv := newIterableValue(data)
		if got := iv.GetBoolWithDefault("user.active", false); got != true {
			t.Errorf("got %v, want true", got)
		}
	})
}

// TestForeachWithPathAndControl tests break and continue control flow.
func TestForeachWithPathAndControl(t *testing.T) {
	tests := []struct {
		name       string
		jsonStr    string
		path       string
		breakAfter int
		expectErr  bool
	}{
		{
			name:       "break after first item",
			jsonStr:    `{"items":[1,2,3,4,5]}`,
			path:       "items",
			breakAfter: 1,
		},
		{
			name:       "iterate all without break",
			jsonStr:    `{"items":[1,2,3]}`,
			path:       "items",
			breakAfter: 0,
		},
		{
			name:      "invalid path returns error",
			jsonStr:   `{"items":[1,2]}`,
			path:      "nonexistent",
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			count := 0
			err := ForeachWithPathAndControl(tt.jsonStr, tt.path, func(key any, value any) IteratorControl {
				count++
				if tt.breakAfter > 0 && count >= tt.breakAfter {
					return IteratorBreak
				}
				return IteratorNormal
			})

			if tt.expectErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if tt.breakAfter > 0 && count != tt.breakAfter {
				t.Errorf("callback called %d times, want %d (break after)", count, tt.breakAfter)
			}
		})
	}

	t.Run("break returns nil error", func(t *testing.T) {
		err := ForeachWithPathAndControl(`{"items":[1,2,3]}`, "items", func(key any, value any) IteratorControl {
			return IteratorBreak
		})
		if err != nil {
			t.Errorf("IteratorBreak should return nil error, got: %v", err)
		}
	})

	t.Run("IteratorContinue does not stop iteration", func(t *testing.T) {
		count := 0
		err := ForeachWithPathAndControl(`{"items":[10,20,30]}`, "items", func(key any, value any) IteratorControl {
			count++
			return IteratorContinue
		})
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if count != 3 {
			t.Errorf("IteratorContinue: callback called %d times, want 3", count)
		}
	})

	t.Run("non-iterable value returns error", func(t *testing.T) {
		err := ForeachWithPathAndControl(`{"value":42}`, "value", func(key any, value any) IteratorControl {
			return IteratorNormal
		})
		if err == nil {
			t.Error("expected error for non-iterable value, got nil")
		}
	})

	t.Run("iterate over object keys", func(t *testing.T) {
		var keys []string
		err := ForeachWithPathAndControl(`{"data":{"a":1,"b":2,"c":3}}`, "data", func(key any, value any) IteratorControl {
			keys = append(keys, key.(string))
			return IteratorNormal
		})
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if len(keys) != 3 {
			t.Errorf("got %d keys, want 3", len(keys))
		}
	})
}

// ============================================================================
// ParallelIterator TESTS (merged from parallel_iterator_test.go)
// ============================================================================

func TestParallelIteratorForEach(t *testing.T) {
	t.Run("AllItems", func(t *testing.T) {
		data := make([]any, 10)
		for i := range data {
			data[i] = i
		}

		cfg := DefaultConfig()
		cfg.MaxConcurrency = 4
		iter := NewParallelIterator(data, cfg)
		defer iter.Close()

		var processed int64
		err := iter.ForEach(func(idx int, val any) error {
			atomic.AddInt64(&processed, 1)
			return nil
		})

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if processed != 10 {
			t.Errorf("expected 10 items processed, got %d", processed)
		}
	})

	t.Run("WithError", func(t *testing.T) {
		data := make([]any, 10)
		for i := range data {
			data[i] = i
		}

		cfg := DefaultConfig()
		cfg.MaxConcurrency = 4
		iter := NewParallelIterator(data, cfg)
		defer iter.Close()

		testErr := errors.New("test error")
		err := iter.ForEach(func(idx int, val any) error {
			if idx == 5 {
				return testErr
			}
			return nil
		})

		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !errors.Is(err, testErr) {
			t.Errorf("expected testErr, got %v", err)
		}
	})

	t.Run("EmptyData", func(t *testing.T) {
		data := []any{}

		iter := NewParallelIterator(data)
		defer iter.Close()

		err := iter.ForEach(func(idx int, val any) error {
			return nil
		})

		if err != nil {
			t.Fatalf("unexpected error for empty data: %v", err)
		}
	})
}

func TestParallelIteratorMap(t *testing.T) {
	t.Run("MultiplyByTwo", func(t *testing.T) {
		data := []any{float64(1), float64(2), float64(3), float64(4), float64(5)}
		cfg := DefaultConfig()
		cfg.MaxConcurrency = 4
		iter := NewParallelIterator(data, cfg)
		defer iter.Close()

		result, err := iter.Map(func(idx int, val any) (any, error) {
			return val.(float64) * 2, nil
		})

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(result) != 5 {
			t.Fatalf("expected 5 results, got %d", len(result))
		}
		expected := []float64{2, 4, 6, 8, 10}
		for i, v := range result {
			if v.(float64) != expected[i] {
				t.Errorf("result[%d] = %v, want %v", i, v, expected[i])
			}
		}
	})

	t.Run("ErrorOnOneElement", func(t *testing.T) {
		data := []any{float64(1), float64(2), float64(3)}
		cfg := DefaultConfig()
		cfg.MaxConcurrency = 2
		iter := NewParallelIterator(data, cfg)
		defer iter.Close()

		testErr := errors.New("transform error")
		result, err := iter.Map(func(idx int, val any) (any, error) {
			if idx == 1 {
				return nil, testErr
			}
			return val, nil
		})

		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !errors.Is(err, testErr) {
			t.Errorf("expected testErr, got %v", err)
		}
		if result != nil {
			t.Errorf("expected nil result on error, got %v", result)
		}
	})
}

func TestParallelIteratorFilter(t *testing.T) {
	t.Run("KeepEvenValues", func(t *testing.T) {
		data := []any{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
		cfg := DefaultConfig()
		cfg.MaxConcurrency = 4
		iter := NewParallelIterator(data, cfg)
		defer iter.Close()

		result := iter.Filter(func(idx int, val any) bool {
			return val.(int)%2 == 0
		})

		if len(result) != 5 {
			t.Fatalf("expected 5 results, got %d", len(result))
		}
		expected := []int{2, 4, 6, 8, 10}
		// Filter preserves order of insertion, but parallel execution means
		// order is not guaranteed. Check by presence instead.
		resultSet := map[int]bool{}
		for _, v := range result {
			resultSet[v.(int)] = true
		}
		for _, e := range expected {
			if !resultSet[e] {
				t.Errorf("expected %d in result", e)
			}
		}
	})
}

func TestParallelIteratorForEachBatch(t *testing.T) {
	t.Run("BatchSizes", func(t *testing.T) {
		data := make([]any, 10)
		for i := range data {
			data[i] = i
		}

		cfg := DefaultConfig()
		cfg.MaxConcurrency = 4
		iter := NewParallelIterator(data, cfg)
		defer iter.Close()

		var batchCalls int64
		var totalItems int64

		err := iter.ForEachBatch(3, func(batchIdx int, batch []any) error {
			atomic.AddInt64(&batchCalls, 1)
			for range batch {
				atomic.AddInt64(&totalItems, 1)
			}
			return nil
		})

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if batchCalls != 4 {
			t.Errorf("expected 4 batch calls (3+3+3+1), got %d", batchCalls)
		}
		if totalItems != 10 {
			t.Errorf("expected 10 total items, got %d", totalItems)
		}
	})
}

// TestParallelIteratorPanicRecovery verifies SEC-003: a panic inside a
// user-provided callback is recovered and surfaced as an error rather than
// crashing the process. Mirrors the guarantee already provided by the JSONL
// worker (see processor_streamjsonl.go). If the recover guard is removed, the
// panic propagates and the test binary aborts — so this test pins the guard.
func TestParallelIteratorPanicRecovery(t *testing.T) {
	data := make([]any, 10)
	for i := range data {
		data[i] = i
	}

	t.Run("ForEach", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.MaxConcurrency = 4
		iter := NewParallelIterator(data, cfg)
		defer iter.Close()

		err := iter.ForEach(func(idx int, val any) error {
			if idx == 3 {
				panic("boom from ForEach callback")
			}
			return nil
		})
		if err == nil {
			t.Fatal("expected error from panicking callback, got nil")
		}
		if !strings.Contains(err.Error(), "panicked") {
			t.Errorf("expected error to mention panic, got: %v", err)
		}
	})

	t.Run("ForEachBatch", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.MaxConcurrency = 4
		iter := NewParallelIterator(data, cfg)
		defer iter.Close()

		err := iter.ForEachBatch(3, func(batchIdx int, batch []any) error {
			if batchIdx == 1 {
				panic("boom from ForEachBatch callback")
			}
			return nil
		})
		if err == nil {
			t.Fatal("expected error from panicking callback, got nil")
		}
		if !strings.Contains(err.Error(), "panicked") {
			t.Errorf("expected error to mention panic, got: %v", err)
		}
	})
}

// ============================================================================
// StreamIterator / StreamObjectIterator / BatchIterator TESTS
// (merged from stream_iterator_test.go)
// ============================================================================

func TestStreamIterator(t *testing.T) {
	t.Run("IterateArray", func(t *testing.T) {
		input := `[1,2,3]`
		iter := NewStreamIterator(strings.NewReader(input))

		var values []any
		var indices []int
		for iter.Next() {
			values = append(values, iter.Value())
			indices = append(indices, iter.Index())
		}

		if err := iter.Err(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(values) != 3 {
			t.Fatalf("expected 3 values, got %d", len(values))
		}
		// JSON numbers decode as float64
		expectedValues := []any{float64(1), float64(2), float64(3)}
		for i, v := range values {
			if v.(float64) != expectedValues[i].(float64) {
				t.Errorf("values[%d] = %v, want %v", i, v, expectedValues[i])
			}
		}
		expectedIndices := []int{0, 1, 2}
		for i, idx := range indices {
			if idx != expectedIndices[i] {
				t.Errorf("indices[%d] = %d, want %d", i, idx, expectedIndices[i])
			}
		}
	})

	t.Run("EmptyArray", func(t *testing.T) {
		input := `[]`
		iter := NewStreamIterator(strings.NewReader(input))

		if iter.Next() {
			t.Error("expected Next() to return false for empty array")
		}
		if err := iter.Err(); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("InvalidJSON", func(t *testing.T) {
		input := `not json`
		iter := NewStreamIterator(strings.NewReader(input))

		if iter.Next() {
			t.Error("expected Next() to return false for invalid JSON")
		}
		if err := iter.Err(); err == nil {
			t.Error("expected Err() to return non-nil for invalid JSON")
		}
	})

	t.Run("ValueBeforeNext", func(t *testing.T) {
		input := `[1,2,3]`
		iter := NewStreamIterator(strings.NewReader(input))

		val := iter.Value()
		if val != nil {
			t.Errorf("expected Value() before Next() to return nil, got %v", val)
		}
	})

	t.Run("ErrAfterCleanIteration", func(t *testing.T) {
		input := `[1,2,3]`
		iter := NewStreamIterator(strings.NewReader(input))

		for iter.Next() {
			// drain all elements
		}

		if err := iter.Err(); err != nil {
			t.Errorf("expected Err() to be nil after clean iteration, got %v", err)
		}
	})

	t.Run("LargeArray", func(t *testing.T) {
		var sb strings.Builder
		sb.WriteByte('[')
		const n = 1000
		for i := 0; i < n; i++ {
			if i > 0 {
				sb.WriteByte(',')
			}
			sb.WriteString("1")
		}
		sb.WriteByte(']')

		iter := NewStreamIterator(strings.NewReader(sb.String()))

		count := 0
		for iter.Next() {
			count++
		}

		if err := iter.Err(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if count != n {
			t.Errorf("expected %d elements, got %d", n, count)
		}
	})
}

func TestStreamObjectIterator(t *testing.T) {
	t.Run("IterateObject", func(t *testing.T) {
		input := `{"a":1,"b":2}`
		iter := NewStreamObjectIterator(strings.NewReader(input))

		keys := map[string]any{}
		for iter.Next() {
			keys[iter.Key()] = iter.Value()
		}

		if err := iter.Err(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(keys) != 2 {
			t.Fatalf("expected 2 keys, got %d", len(keys))
		}
		if v, ok := keys["a"]; !ok || v.(float64) != 1 {
			t.Errorf("key 'a' = %v, want 1", v)
		}
		if v, ok := keys["b"]; !ok || v.(float64) != 2 {
			t.Errorf("key 'b' = %v, want 2", v)
		}
	})

	t.Run("EmptyObject", func(t *testing.T) {
		input := `{}`
		iter := NewStreamObjectIterator(strings.NewReader(input))

		if iter.Next() {
			t.Error("expected Next() to return false for empty object")
		}
		if err := iter.Err(); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("InvalidJSON", func(t *testing.T) {
		input := `not json`
		iter := NewStreamObjectIterator(strings.NewReader(input))

		if iter.Next() {
			t.Error("expected Next() to return false for invalid JSON")
		}
		if err := iter.Err(); err == nil {
			t.Error("expected Err() to return non-nil for invalid JSON")
		}
	})

	t.Run("KeyValueBeforeNext", func(t *testing.T) {
		input := `{"a":1}`
		iter := NewStreamObjectIterator(strings.NewReader(input))

		if key := iter.Key(); key != "" {
			t.Errorf("expected Key() before Next() to return empty string, got %q", key)
		}
		if val := iter.Value(); val != nil {
			t.Errorf("expected Value() before Next() to return nil, got %v", val)
		}
	})

	t.Run("NestedValues", func(t *testing.T) {
		input := `{"str":"hello","num":42,"bool":true,"arr":[1,2],"obj":{"nested":1}}`
		iter := NewStreamObjectIterator(strings.NewReader(input))

		result := map[string]any{}
		for iter.Next() {
			result[iter.Key()] = iter.Value()
		}

		if err := iter.Err(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if v, ok := result["str"]; !ok || v.(string) != "hello" {
			t.Errorf("key 'str' = %v, want 'hello'", v)
		}
		if v, ok := result["num"]; !ok || v.(float64) != 42 {
			t.Errorf("key 'num' = %v, want 42", v)
		}
		if v, ok := result["bool"]; !ok || v.(bool) != true {
			t.Errorf("key 'bool' = %v, want true", v)
		}
		if v, ok := result["arr"]; !ok {
			t.Errorf("key 'arr' missing")
		} else if arr, ok := v.([]any); !ok || len(arr) != 2 {
			t.Errorf("key 'arr' = %v, want array of 2 elements", v)
		}
		if v, ok := result["obj"]; !ok {
			t.Errorf("key 'obj' missing")
		} else if obj, ok := v.(map[string]any); !ok || obj["nested"].(float64) != 1 {
			t.Errorf("key 'obj' = %v, want object with nested=1", v)
		}
	})
}

func TestBatchIterator(t *testing.T) {
	t.Run("ExactMultiple", func(t *testing.T) {
		data := []any{float64(1), float64(2), float64(3), float64(4), float64(5), float64(6)}
		cfg := DefaultConfig()
		cfg.MaxBatchSize = 3
		iter := NewBatchIterator(data, cfg)

		batch1 := iter.NextBatch()
		if len(batch1) != 3 {
			t.Fatalf("batch 1: expected 3 items, got %d", len(batch1))
		}

		batch2 := iter.NextBatch()
		if len(batch2) != 3 {
			t.Fatalf("batch 2: expected 3 items, got %d", len(batch2))
		}

		if iter.HasNext() {
			t.Error("expected HasNext() to be false after all batches consumed")
		}

		batch3 := iter.NextBatch()
		if batch3 != nil {
			t.Errorf("expected nil after all batches, got %v", batch3)
		}
	})

	t.Run("NonMultiple", func(t *testing.T) {
		data := []any{1, 2, 3, 4, 5, 6, 7}
		cfg := DefaultConfig()
		cfg.MaxBatchSize = 3
		iter := NewBatchIterator(data, cfg)

		batchSizes := []int{}
		for iter.HasNext() {
			batch := iter.NextBatch()
			batchSizes = append(batchSizes, len(batch))
		}

		expected := []int{3, 3, 1}
		if len(batchSizes) != len(expected) {
			t.Fatalf("expected %d batches, got %d", len(expected), len(batchSizes))
		}
		for i, size := range batchSizes {
			if size != expected[i] {
				t.Errorf("batch %d: size=%d, want %d", i, size, expected[i])
			}
		}
	})

	t.Run("EmptyData", func(t *testing.T) {
		data := []any{}
		cfg := DefaultConfig()
		cfg.MaxBatchSize = 3
		iter := NewBatchIterator(data, cfg)

		batch := iter.NextBatch()
		if batch != nil {
			t.Errorf("expected nil for empty data, got %v", batch)
		}
		if iter.HasNext() {
			t.Error("expected HasNext() to be false for empty data")
		}
	})

	t.Run("SingleItem", func(t *testing.T) {
		data := []any{"only"}
		cfg := DefaultConfig()
		cfg.MaxBatchSize = 3
		iter := NewBatchIterator(data, cfg)

		batch := iter.NextBatch()
		if len(batch) != 1 {
			t.Fatalf("expected 1 item, got %d", len(batch))
		}
		if batch[0] != "only" {
			t.Errorf("expected 'only', got %v", batch[0])
		}
		if iter.HasNext() {
			t.Error("expected HasNext() to be false after single item")
		}
	})

	t.Run("Reset", func(t *testing.T) {
		data := []any{1, 2, 3, 4}
		cfg := DefaultConfig()
		cfg.MaxBatchSize = 2
		iter := NewBatchIterator(data, cfg)

		// Consume first batch
		batch1 := iter.NextBatch()
		if len(batch1) != 2 {
			t.Fatalf("first batch: expected 2 items, got %d", len(batch1))
		}

		// Reset
		iter.Reset()

		if !iter.HasNext() {
			t.Error("expected HasNext() to be true after Reset()")
		}
		if iter.CurrentIndex() != 0 {
			t.Errorf("expected CurrentIndex()=0 after Reset(), got %d", iter.CurrentIndex())
		}
		if iter.Remaining() != 4 {
			t.Errorf("expected Remaining()=4 after Reset(), got %d", iter.Remaining())
		}

		// Consume again from start
		batchAfterReset := iter.NextBatch()
		if len(batchAfterReset) != 2 {
			t.Fatalf("batch after reset: expected 2 items, got %d", len(batchAfterReset))
		}
		if batchAfterReset[0] != 1 || batchAfterReset[1] != 2 {
			t.Errorf("batch after reset: expected [1,2], got %v", batchAfterReset)
		}
	})

	t.Run("TotalBatches", func(t *testing.T) {
		tests := []struct {
			name      string
			dataLen   int
			batchSize int
			expected  int
		}{
			{"exact_multiple", 6, 3, 2},
			{"non_multiple", 7, 3, 3},
			{"single_batch", 3, 5, 1},
			{"empty", 0, 3, 0},
			{"one_item", 1, 5, 1},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				data := make([]any, tt.dataLen)
				cfg := DefaultConfig()
				cfg.MaxBatchSize = tt.batchSize
				iter := NewBatchIterator(data, cfg)

				total := iter.TotalBatches()
				if total != tt.expected {
					t.Errorf("TotalBatches() = %d, want %d", total, tt.expected)
				}
			})
		}
	})

	t.Run("CurrentIndexAndRemaining", func(t *testing.T) {
		data := []any{1, 2, 3, 4, 5}
		cfg := DefaultConfig()
		cfg.MaxBatchSize = 2
		iter := NewBatchIterator(data, cfg)

		if idx := iter.CurrentIndex(); idx != 0 {
			t.Errorf("initial CurrentIndex() = %d, want 0", idx)
		}
		if rem := iter.Remaining(); rem != 5 {
			t.Errorf("initial Remaining() = %d, want 5", rem)
		}

		iter.NextBatch() // consumes 2 items

		if idx := iter.CurrentIndex(); idx != 2 {
			t.Errorf("after 1 batch CurrentIndex() = %d, want 2", idx)
		}
		if rem := iter.Remaining(); rem != 3 {
			t.Errorf("after 1 batch Remaining() = %d, want 3", rem)
		}

		iter.NextBatch() // consumes 2 more items

		if idx := iter.CurrentIndex(); idx != 4 {
			t.Errorf("after 2 batches CurrentIndex() = %d, want 4", idx)
		}
		if rem := iter.Remaining(); rem != 1 {
			t.Errorf("after 2 batches Remaining() = %d, want 1", rem)
		}

		iter.NextBatch() // consumes last item

		if idx := iter.CurrentIndex(); idx != 5 {
			t.Errorf("after 3 batches CurrentIndex() = %d, want 5", idx)
		}
		if rem := iter.Remaining(); rem != 0 {
			t.Errorf("after 3 batches Remaining() = %d, want 0", rem)
		}
	})
}

// TestIteratorReset verifies Reset clears iteration state and cached map keys,
// leaving an iterator that yields nothing until ResetWith supplies new data.
func TestIteratorReset(t *testing.T) {
	it := NewIterator(map[string]any{"a": 1, "b": 2})

	// Consume one element so initKeysOnce caches the key slice.
	if _, ok := it.Next(); !ok {
		t.Fatal("first Next() on fresh iterator should succeed")
	}
	if len(it.keys) == 0 {
		t.Fatal("map iteration should have cached keys after first Next()")
	}

	it.Reset()

	if it.data != nil || it.position != 0 || it.keys != nil {
		t.Errorf("Reset should clear data/position/keys, got data=%v position=%d keys=%v",
			it.data, it.position, it.keys)
	}
	if it.HasNext() {
		t.Error("HasNext after Reset should be false")
	}
	if _, ok := it.Next(); ok {
		t.Error("Next after Reset should return ok=false")
	}
}

// TestIteratorResetWith verifies iterator reuse on new data and, critically,
// that the sync.Once reset makes initKeysOnce re-collect keys for the new map
// instead of serving stale keys from the previous structure.
func TestIteratorResetWith(t *testing.T) {
	it := NewIterator(map[string]any{"a": 1, "b": 2, "c": 3})
	for it.HasNext() {
		it.Next()
	}

	// Reuse on an array: keys stay unused, elements come in order.
	it.ResetWith([]any{"x", "y"})
	var got []any
	for it.HasNext() {
		v, ok := it.Next()
		if !ok {
			t.Fatal("Next returned ok=false while HasNext was true")
		}
		got = append(got, v)
	}
	if len(got) != 2 || got[0] != "x" || got[1] != "y" {
		t.Errorf("ResetWith(array) iteration = %v, want [x y]", got)
	}

	// Reuse on a different map: keys must be re-initialized for the new data.
	it.ResetWith(map[string]any{"k": 42})
	if v, ok := it.Next(); !ok || v != 42 {
		t.Errorf("ResetWith(map) first Next = (%v, %v), want (42, true)", v, ok)
	}
	if len(it.keys) != 1 || it.keys[0] != "k" {
		t.Errorf("ResetWith should re-collect keys for the new map, got %v", it.keys)
	}
	if it.HasNext() {
		t.Error("single-key map should be exhausted after one Next()")
	}
}

// TestForeachReturn covers the package-level ForeachReturn wrapper: iteration
// over top-level keys, the documented contract that mutations to an item's
// underlying data are reflected in the returned string, and the invalid-JSON
// error path.
func TestForeachReturn(t *testing.T) {
	jsonStr := `{"items":[1,2,3],"meta":{"name":"list"}}`

	count := 0
	result, err := ForeachReturn(jsonStr, func(key any, item *IterableValue) {
		count++
		if key == "meta" {
			// IterableValue is read-only; mutate the object it wraps.
			if m, ok := item.GetData().(map[string]any); ok {
				m["name"] = "renamed"
			}
		}
	})
	if err != nil {
		t.Fatalf("ForeachReturn error: %v", err)
	}
	if count != 2 {
		t.Errorf("callback ran %d times, want 2 (top-level keys)", count)
	}
	if result == "" {
		t.Fatal("ForeachReturn should return a non-empty JSON string")
	}
	var check map[string]any
	if err := json.Unmarshal([]byte(result), &check); err != nil {
		t.Fatalf("ForeachReturn result is not valid JSON: %v (result=%s)", err, result)
	}
	meta, ok := check["meta"].(map[string]any)
	if !ok || meta["name"] != "renamed" {
		t.Errorf(`result["meta"]["name"] = %v, want "renamed" (mutation not reflected)`, check["meta"])
	}
	if items, ok := check["items"].([]any); !ok || len(items) != 3 {
		t.Errorf(`result["items"] = %v, want the original 3-element array`, check["items"])
	}

	if _, err := ForeachReturn(`{invalid`, func(key any, item *IterableValue) {}); err == nil {
		t.Error("ForeachReturn on invalid JSON should return an error")
	} else if !errors.Is(err, ErrInvalidJSON) {
		t.Errorf("ForeachReturn on invalid JSON should wrap ErrInvalidJSON, got %v", err)
	}
}
