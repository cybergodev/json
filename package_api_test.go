package json

import (
	"bytes"
	"testing"
)

// TestGetMultiple tests retrieving multiple values at once
func TestGetMultiple(t *testing.T) {
	jsonStr := `{
		"user": {
			"name": "Alice",
			"age": 30,
			"email": "alice@example.com"
		},
		"settings": {
			"theme": "dark",
			"language": "en"
		}
	}`

	tests := []struct {
		name        string
		paths       []string
		expectedLen int
		expectError bool
	}{
		{
			name:        "multiple paths",
			paths:       []string{"user.name", "user.age", "settings.theme"},
			expectedLen: 3,
			expectError: false,
		},
		{
			name:        "single path",
			paths:       []string{"user.name"},
			expectedLen: 1,
			expectError: false,
		},
		{
			name:        "empty paths",
			paths:       []string{},
			expectedLen: 0,
			expectError: false,
		},
		{
			name:        "mixed valid and invalid",
			paths:       []string{"user.name", "invalid.path", "settings.theme"},
			expectedLen: 3, // GetMultiple returns entries for all paths, including nil for invalid ones
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := GetMultiple(jsonStr, tt.paths)
			if tt.expectError && err == nil {
				t.Errorf("Expected error, but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
			if !tt.expectError && len(result) != tt.expectedLen {
				t.Errorf("Result length = %d; want %d", len(result), tt.expectedLen)
			}
		})
	}
}

// TestSetMultiple tests setting multiple values at once
func TestSetMultiple(t *testing.T) {
	jsonStr := `{"user": {"name": "Alice", "age": 30}}`

	tests := []struct {
		name        string
		updates     map[string]any
		expectError bool
		validate    func(t *testing.T, result string)
	}{
		{
			name: "multiple updates",
			updates: map[string]any{
				"user.name":  "Bob",
				"user.age":   35,
				"user.email": "bob@example.com",
			},
			expectError: false,
			validate: func(t *testing.T, result string) {
				if !contains(result, "Bob") {
					t.Error("Expected name to be updated to Bob")
				}
			},
		},
		{
			name:        "empty updates",
			updates:     map[string]any{},
			expectError: false,
			validate: func(t *testing.T, result string) {
				if !contains(result, "Alice") {
					t.Error("Expected original data to remain")
				}
			},
		},
		{
			name: "single update",
			updates: map[string]any{
				"user.name": "Charlie",
			},
			expectError: false,
			validate: func(t *testing.T, result string) {
				if !contains(result, "Charlie") {
					t.Error("Expected name to be updated to Charlie")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := SetMultiple(jsonStr, tt.updates)
			if tt.expectError && err == nil {
				t.Errorf("Expected error, but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
			if tt.validate != nil {
				tt.validate(t, result)
			}
		})
	}
}

// TestDeleteWithCleanNull tests deletion with null cleanup
func TestDeleteWithCleanNull(t *testing.T) {
	jsonStr := `{
		"user": {
			"name": "Alice",
			"age": 30,
			"email": null
		},
		"posts": [
			{"title": "Post 1", "content": null},
			{"title": "Post 2", "content": "Content"}
		]
	}`

	tests := []struct {
		name     string
		path     string
		contains []string
		excludes []string
	}{
		{
			name:     "delete and clean nulls",
			path:     "user.email",
			contains: []string{"name", "age"},
			excludes: []string{"email", "null"},
		},
		{
			name:     "delete entire array",
			path:     "posts",
			contains: []string{"user", "name"},                // Should still have user data
			excludes: []string{"Post 1", "Post 2", "content"}, // All posts content should be gone
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := DeleteWithCleanNull(jsonStr, tt.path)
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			for _, str := range tt.contains {
				if !contains(result, str) {
					t.Errorf("Expected result to contain '%s'", str)
				}
			}
			for _, str := range tt.excludes {
				if contains(result, str) {
					t.Errorf("Expected result to not contain '%s'", str)
				}
			}
		})
	}
}

// TestFormatPretty tests pretty formatting
func TestFormatPretty(t *testing.T) {
	compactJSON := `{"user":{"name":"Alice","age":30},"settings":{"theme":"dark"}}`

	result, err := FormatPretty(compactJSON)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Check for indentation
	if !contains(result, "\n") {
		t.Error("Expected formatted output to contain newlines")
	}
	if !contains(result, "  ") {
		t.Error("Expected formatted output to contain indentation")
	}
}

// TestFormatCompact tests compact formatting
func TestFormatCompact(t *testing.T) {
	prettyJSON := `{
		"user": {
			"name": "Alice",
			"age": 30
		}
	}`

	result, err := FormatCompact(prettyJSON)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Check that it's compact
	if contains(result, "\n") {
		t.Error("Expected compact output to not contain newlines")
	}
}

// TestEncodeStream tests stream encoding
func TestEncodeStream(t *testing.T) {
	values := []any{
		map[string]any{"name": "Alice"},
		map[string]any{"name": "Bob"},
		map[string]any{"name": "Charlie"},
	}

	tests := []struct {
		name        string
		pretty      bool
		expectError bool
		validate    func(t *testing.T, result string)
	}{
		{
			name:        "compact stream",
			pretty:      false,
			expectError: false,
			validate: func(t *testing.T, result string) {
				if !contains(result, "[") || !contains(result, "]") {
					t.Error("Expected array wrapper")
				}
			},
		},
		{
			name:        "pretty stream",
			pretty:      true,
			expectError: false,
			validate: func(t *testing.T, result string) {
				if !contains(result, "\n") {
					t.Error("Expected pretty output with newlines")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := EncodeStream(values, tt.pretty)
			if tt.expectError && err == nil {
				t.Errorf("Expected error, but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
			if tt.validate != nil {
				tt.validate(t, result)
			}
		})
	}
}

// TestEncodeBatch tests batch encoding
func TestEncodeBatch(t *testing.T) {
	pairs := map[string]any{
		"user1": map[string]any{"name": "Alice"},
		"user2": map[string]any{"name": "Bob"},
	}

	result, err := EncodeBatch(pairs, false)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Check that it's a JSON object
	if !contains(result, "{") || !contains(result, "}") {
		t.Error("Expected object wrapper")
	}
	if !contains(result, "user1") || !contains(result, "user2") {
		t.Error("Expected keys to be present")
	}
}

// TestEncodeFields tests selective field encoding
func TestEncodeFields(t *testing.T) {
	type User struct {
		Name     string `json:"name"`
		Age      int    `json:"age"`
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	user := User{
		Name:     "Alice",
		Age:      30,
		Email:    "alice@example.com",
		Password: "secret123",
	}

	fields := []string{"name", "email"}

	result, err := EncodeFields(user, fields, false)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Check that only specified fields are present
	if !contains(result, "name") || !contains(result, "email") {
		t.Error("Expected specified fields to be present")
	}
	if contains(result, "password") {
		t.Error("Expected password to be excluded")
	}
	if contains(result, "age") {
		t.Error("Expected age to be excluded")
	}
}

// TestProcessBatch tests batch processing
func TestProcessBatch(t *testing.T) {
	jsonStr := `{"user": {"name": "Alice", "age": 30}}`

	operations := []BatchOperation{
		{Type: "get", Path: "user.name"},
		{Type: "get", Path: "user.age"},
		{Type: "set", Path: "user.email", Value: "alice@example.com"},
	}

	results, err := ProcessBatch(operations)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(results) != len(operations) {
		t.Errorf("Expected %d results, got %d", len(operations), len(results))
	}

	_ = jsonStr // Use the variable
}

// TestWarmupCache tests cache warmup functionality
func TestWarmupCache(t *testing.T) {
	jsonStr := `{
		"users": [
			{"name": "Alice", "age": 30},
			{"name": "Bob", "age": 25}
		],
		"settings": {
			"theme": "dark",
			"language": "en"
		}
	}`

	paths := []string{
		"users[0].name",
		"users[1].name",
		"settings.theme",
	}

	result, err := WarmupCache(jsonStr, paths)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if result == nil {
		t.Error("Expected warmup result, got nil")
	}
}

// TestGetTypedWithDefault tests typed get with defaults
func TestGetTypedWithDefault(t *testing.T) {
	jsonStr := `{"user": {"name": "Alice", "age": 30}}`

	t.Run("existing value", func(t *testing.T) {
		name := GetStringWithDefault(jsonStr, "user.name", "Unknown")
		if name != "Alice" {
			t.Errorf("Expected 'Alice', got '%s'", name)
		}
	})

	t.Run("missing value with default", func(t *testing.T) {
		name := GetStringWithDefault(jsonStr, "user.email", "unknown@example.com")
		if name != "unknown@example.com" {
			t.Errorf("Expected default value, got '%s'", name)
		}
	})

	t.Run("int with default", func(t *testing.T) {
		age := GetIntWithDefault(jsonStr, "user.age", 0)
		if age != 30 {
			t.Errorf("Expected 30, got %d", age)
		}
	})

	t.Run("missing int with default", func(t *testing.T) {
		score := GetIntWithDefault(jsonStr, "user.score", 100)
		if score != 100 {
			t.Errorf("Expected default 100, got %d", score)
		}
	})
}

// TestGetWithDefault tests get with default value
func TestGetWithDefault(t *testing.T) {
	jsonStr := `{"user": {"name": "Alice"}}`

	t.Run("existing value", func(t *testing.T) {
		result := GetWithDefault(jsonStr, "user.name", "Unknown")
		if result != "Alice" {
			t.Errorf("Expected 'Alice', got '%v'", result)
		}
	})

	t.Run("missing value", func(t *testing.T) {
		result := GetWithDefault(jsonStr, "user.email", "unknown@example.com")
		if result != "unknown@example.com" {
			t.Errorf("Expected default value, got '%v'", result)
		}
	})
}

// TestStandardLibraryCompatibility tests encoding/json compatibility
func TestStandardLibraryCompatibility(t *testing.T) {
	// Test Marshal
	data := map[string]any{"name": "Alice", "age": 30}
	jsonBytes, err := Marshal(data)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}
	if len(jsonBytes) == 0 {
		t.Error("Expected non-empty JSON output")
	}

	// Test Unmarshal
	var result map[string]any
	err = Unmarshal(jsonBytes, &result)
	if err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
	if result["name"] != "Alice" {
		t.Errorf("Expected name 'Alice', got '%v'", result["name"])
	}

	// Test MarshalIndent
	indented, err := MarshalIndent(data, "", "  ")
	if err != nil {
		t.Fatalf("MarshalIndent failed: %v", err)
	}
	if !contains(string(indented), "\n") {
		t.Error("Expected indented output to contain newlines")
	}

	// Test Valid
	if !Valid(jsonBytes) {
		t.Error("Valid() returned false for valid JSON")
	}
	invalidJSON := []byte("{invalid}")
	if Valid(invalidJSON) {
		t.Error("Valid() returned true for invalid JSON")
	}
}

// TestBufferCompatibility tests buffer-based operations
func TestBufferCompatibility(t *testing.T) {
	jsonStr := `{"name": "Alice", "age": 30}`
	src := []byte(jsonStr)

	// Test Compact
	var compactBuf bytes.Buffer
	err := Compact(&compactBuf, src)
	if err != nil {
		t.Fatalf("Compact failed: %v", err)
	}

	// Test Indent
	var indentBuf bytes.Buffer
	err = Indent(&indentBuf, src, "", "  ")
	if err != nil {
		t.Fatalf("Indent failed: %v", err)
	}
	if !contains(indentBuf.String(), "\n") {
		t.Error("Expected indented output")
	}

	// Test HTMLEscape
	// Note: Using characters that don't trigger security validation
	var escapeBuf bytes.Buffer
	htmlJSON := []byte(`{"html": "<div>Content & more</div>"}`)
	HTMLEscape(&escapeBuf, htmlJSON)
	escaped := escapeBuf.String()
	// HTML entities should be escaped
	// Standard library escapes < to \u003c, > to \u003e, & to \u0026
	if !contains(escaped, "\\u003c") && !contains(escaped, "\\u003e") && !contains(escaped, "\\u0026") {
		t.Logf("Actual escaped output: %s", escaped)
		// Check that raw HTML characters are not present
		if contains(escaped, "<div>") {
			t.Error("Expected HTML to be escaped but found raw <div>")
		}
	}
}

// TestSetWithAdd tests set with automatic path creation
func TestSetWithAdd(t *testing.T) {
	jsonStr := `{"user": {}}`

	result, err := SetWithAdd(jsonStr, "user.name", "Alice")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if !contains(result, "Alice") {
		t.Error("Expected name to be set")
	}

	// Test nested path creation
	result, err = SetWithAdd(result, "user.profile.age", 30)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if !contains(result, "age") {
		t.Error("Expected nested age to be set")
	}
}

// TestSetMultipleWithAdd tests multiple sets with path creation
func TestSetMultipleWithAdd(t *testing.T) {
	jsonStr := `{}`

	updates := map[string]any{
		"user.name":      "Alice",
		"user.age":       30,
		"user.email":     "alice@example.com",
		"settings.theme": "dark",
	}

	result, err := SetMultipleWithAdd(jsonStr, updates)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	for _, str := range []string{"Alice", "alice@example.com", "dark"} {
		if !contains(result, str) {
			t.Errorf("Expected result to contain '%s'", str)
		}
	}
}

// TestClearCache tests cache clearing
func TestClearCache(t *testing.T) {
	// Get a value to populate cache
	jsonStr := `{"user": {"name": "Alice"}}`
	_, _ = Get(jsonStr, "user.name")

	// Clear cache
	ClearCache()

	// Should not error
	t.Log("Cache cleared successfully")
}

// TestGetStats tests statistics retrieval
func TestGetStats(t *testing.T) {
	stats := GetStats()

	if stats.CacheSize < 0 {
		t.Error("Expected non-negative cache size")
	}

	t.Logf("Stats: %+v", stats)
}

// TestGetHealthStatus tests health status retrieval
func TestGetHealthStatus(t *testing.T) {
	status := GetHealthStatus()

	// Just verify we can get health status without panicking
	// The actual healthy status depends on whether metrics are initialized
	if status.Timestamp.IsZero() {
		t.Error("Expected non-zero timestamp")
	}

	if len(status.Checks) == 0 {
		t.Error("Expected some health checks to be present")
	}

	// At minimum, memory check should be present
	if _, ok := status.Checks["memory"]; !ok {
		t.Error("Expected memory check to be present")
	}

	t.Logf("Health status: %+v", status)
}

// Helper functions
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && indexOf(s, substr) >= 0)
}

func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

// Benchmark tests

func BenchmarkGet(b *testing.B) {
	jsonStr := `{"user": {"name": "Alice", "age": 30, "email": "alice@example.com"}}`
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = Get(jsonStr, "user.name")
	}
}

func BenchmarkSet(b *testing.B) {
	jsonStr := `{"user": {"name": "Alice"}}`
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = Set(jsonStr, "user.age", 30)
	}
}

func BenchmarkDelete(b *testing.B) {
	jsonStr := `{"user": {"name": "Alice", "age": 30, "email": "alice@example.com"}}`
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = Delete(jsonStr, "user.email")
	}
}

func BenchmarkMarshal(b *testing.B) {
	data := map[string]any{"name": "Alice", "age": 30}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = Marshal(data)
	}
}
