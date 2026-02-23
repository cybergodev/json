package json

import (
	"testing"
)

// TestNewIterableValue tests creating IterableValue from different data types
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
			iv := NewIterableValue(tt.data)
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

// TestIterableValueGet tests Get method
func TestIterableValueGet(t *testing.T) {
	data := map[string]any{
		"user": map[string]any{
			"name": "Alice",
			"age":  30,
		},
		"items": []any{"a", "b", "c"},
	}

	iv := NewIterableValue(data)

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

	iv := NewIterableValue(data)

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

	iv := NewIterableValue(data)

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

	iv := NewIterableValue(data)

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

	iv := NewIterableValue(data)

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

// TestIterableValueGetArray tests GetArray method
func TestIterableValueGetArray(t *testing.T) {
	data := map[string]any{
		"items":   []any{"a", "b", "c"},
		"numbers": []any{1, 2, 3},
		"user": map[string]any{
			"tags": []any{"developer", "golang"},
		},
	}

	iv := NewIterableValue(data)

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

	iv := NewIterableValue(data)

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

	iv := NewIterableValue(data)

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

// TestIterableValueIsNull tests IsNull method
func TestIterableValueIsNull(t *testing.T) {
	data := map[string]any{
		"name":  "Alice",
		"email": nil,
		"user": map[string]any{
			"deleted": nil,
		},
	}

	iv := NewIterableValue(data)

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

	iv := NewIterableValue(data)

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
				result, ok := SafeTypeAssert[int](tt.input)
				if ok != tt.shouldSucceed {
					t.Errorf("SafeTypeAssert[int](%v) success = %v; want %v", tt.input, ok, tt.shouldSucceed)
				}
				if tt.validate != nil {
					tt.validate(t, result)
				}
			case "string":
				result, ok := SafeTypeAssert[string](tt.input)
				if ok != tt.shouldSucceed {
					t.Errorf("SafeTypeAssert[string](%v) success = %v; want %v", tt.input, ok, tt.shouldSucceed)
				}
				if tt.validate != nil {
					tt.validate(t, result)
				}
			case "float64":
				result, ok := SafeTypeAssert[float64](tt.input)
				if ok != tt.shouldSucceed {
					t.Errorf("SafeTypeAssert[float64](%v) success = %v; want %v", tt.input, ok, tt.shouldSucceed)
				}
				if tt.validate != nil {
					tt.validate(t, result)
				}
			}
		})
	}
}

// TestIteratorHasNext tests Iterator.HasNext method
func TestIteratorHasNext(t *testing.T) {
	processor := New()
	defer processor.Close()

	t.Run("array iterator", func(t *testing.T) {
		data := []any{1, 2, 3}
		it := NewIterator(processor, data, nil)

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
		it := NewIterator(processor, data, nil)

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
		it := NewIterator(processor, data, nil)

		if it.HasNext() {
			t.Error("Expected no elements in empty array")
		}
	})
}

// TestIteratorNext tests Iterator.Next method
func TestIteratorNext(t *testing.T) {
	processor := New()
	defer processor.Close()

	t.Run("array elements", func(t *testing.T) {
		data := []any{"a", "b", "c"}
		it := NewIterator(processor, data, nil)

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
		it := NewIterator(processor, data, nil)

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

// TestDefaultValues tests default value methods
func TestDefaultValues(t *testing.T) {
	data := map[string]any{
		"name": "Alice",
		"age":  30,
	}

	iv := NewIterableValue(data)

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

// TestIteratorDataState tests iterator maintains correct state
func TestIteratorDataState(t *testing.T) {
	processor := New()
	defer processor.Close()

	data := []any{1, 2, 3, 4, 5}
	it := NewIterator(processor, data, nil)

	// Check initial state
	if it.position != 0 {
		t.Errorf("Initial position = %d; want 0", it.position)
	}

	// Consume all elements
	count := 0
	for it.HasNext() {
		it.Next()
		count++
	}

	if count != 5 {
		t.Errorf("Expected 5 elements, got %d", count)
	}

	// Verify no more elements
	if it.HasNext() {
		t.Error("Expected no more elements")
	}
}

// Benchmark tests

func BenchmarkIterableValueGet(b *testing.B) {
	data := map[string]any{
		"user": map[string]any{
			"name":  "Alice",
			"age":   30,
			"email": "alice@example.com",
		},
	}
	iv := NewIterableValue(data)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = iv.Get("user.name")
	}
}

func BenchmarkSafeTypeAssert(b *testing.B) {
	input := 42

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = SafeTypeAssert[int](input)
	}
}

// Helper functions
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
			// Use deep comparison for nested elements
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
			// Use deep comparison for nested values
			if !compareValues(va[key], vb[key]) {
				return false
			}
		}
		return true
	default:
		// For other types, check if they're equal
		// Use a safe check for comparable types
		defer func() {
			if r := recover(); r != nil {
				// Not comparable, return false
			}
		}()
		return a == b
	}
}
