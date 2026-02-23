package json

import (
	"encoding/json"
	"testing"
)

// ============================================================================
// IterableValue Core Tests (from iterator_core_test.go)
// ============================================================================

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

// ============================================================================
// IterableValue Path Navigation Tests (from iterator_path_test.go)
// ============================================================================

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

// ============================================================================
// Benchmark tests
// ============================================================================

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
