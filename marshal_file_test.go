package json

import (
	"os"
	"path/filepath"
	"testing"
)

// Test data structures
type TestUser struct {
	Name  string `json:"name"`
	Age   int    `json:"age"`
	Email string `json:"email"`
}

type TestConfig struct {
	Database struct {
		Host     string `json:"host"`
		Port     int    `json:"port"`
		Username string `json:"username"`
	} `json:"database"`
	Features []string `json:"features"`
	Debug    bool     `json:"debug"`
}

func TestMarshalToFile(t *testing.T) {
	// Create temporary directory for tests
	tempDir := t.TempDir()

	tests := []struct {
		name     string
		path     string
		data     any
		pretty   []bool
		wantErr  bool
		validate func(t *testing.T, path string)
	}{
		{
			name: "simple struct compact",
			path: filepath.Join(tempDir, "user.json"),
			data: TestUser{Name: "John Doe", Age: 30, Email: "john@example.com"},
			validate: func(t *testing.T, path string) {
				content, err := os.ReadFile(path)
				if err != nil {
					t.Fatalf("Failed to read file: %v", err)
				}
				expected := `{"name":"John Doe","age":30,"email":"john@example.com"}`
				if string(content) != expected {
					t.Errorf("Expected %s, got %s", expected, string(content))
				}
			},
		},
		{
			name:   "simple struct pretty",
			path:   filepath.Join(tempDir, "user_pretty.json"),
			data:   TestUser{Name: "Jane Doe", Age: 25, Email: "jane@example.com"},
			pretty: []bool{true},
			validate: func(t *testing.T, path string) {
				content, err := os.ReadFile(path)
				if err != nil {
					t.Fatalf("Failed to read file: %v", err)
				}
				// Check that it contains proper indentation
				contentStr := string(content)
				if !contains(contentStr, "  \"name\"") || !contains(contentStr, "  \"age\"") {
					t.Errorf("Expected pretty formatted JSON, got %s", contentStr)
				}
			},
		},
		{
			name: "nested struct",
			path: filepath.Join(tempDir, "config.json"),
			data: TestConfig{
				Database: struct {
					Host     string `json:"host"`
					Port     int    `json:"port"`
					Username string `json:"username"`
				}{
					Host:     "localhost",
					Port:     5432,
					Username: "admin",
				},
				Features: []string{"auth", "logging", "metrics"},
				Debug:    true,
			},
			validate: func(t *testing.T, path string) {
				content, err := os.ReadFile(path)
				if err != nil {
					t.Fatalf("Failed to read file: %v", err)
				}
				// Basic validation that key fields are present
				contentStr := string(content)
				if !contains(contentStr, "localhost") || !contains(contentStr, "5432") {
					t.Errorf("Expected config data in JSON, got %s", contentStr)
				}
			},
		},
		{
			name: "map data",
			path: filepath.Join(tempDir, "map.json"),
			data: map[string]any{
				"string": "value",
				"number": 42,
				"bool":   true,
				"array":  []int{1, 2, 3},
				"nested": map[string]string{"key": "value"},
			},
			validate: func(t *testing.T, path string) {
				content, err := os.ReadFile(path)
				if err != nil {
					t.Fatalf("Failed to read file: %v", err)
				}
				contentStr := string(content)
				if !contains(contentStr, "\"string\":\"value\"") {
					t.Errorf("Expected map data in JSON, got %s", contentStr)
				}
			},
		},
		{
			name: "create nested directories",
			path: filepath.Join(tempDir, "deep", "nested", "path", "data.json"),
			data: map[string]string{"test": "data"},
			validate: func(t *testing.T, path string) {
				if _, err := os.Stat(path); os.IsNotExist(err) {
					t.Errorf("File was not created at nested path: %s", path)
				}
			},
		},
		{
			name:    "invalid path",
			path:    "",
			data:    map[string]string{"test": "data"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var err error
			if len(tt.pretty) > 0 {
				err = MarshalToFile(tt.path, tt.data, tt.pretty[0])
			} else {
				err = MarshalToFile(tt.path, tt.data)
			}

			if (err != nil) != tt.wantErr {
				t.Errorf("MarshalToFile() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && tt.validate != nil {
				tt.validate(t, tt.path)
			}
		})
	}
}

func TestUnmarshalFromFile(t *testing.T) {
	// Create temporary directory for tests
	tempDir := t.TempDir()

	// Create test files
	testFiles := map[string]string{
		"user.json": `{"name":"John Doe","age":30,"email":"john@example.com"}`,
		"config.json": `{
			"database": {
				"host": "localhost",
				"port": 5432,
				"username": "admin"
			},
			"features": ["auth", "logging", "metrics"],
			"debug": true
		}`,
		"map.json":     `{"string":"value","number":42,"bool":true,"array":[1,2,3]}`,
		"invalid.json": `{"invalid": json}`,
		"empty.json":   ``,
	}

	for filename, content := range testFiles {
		path := filepath.Join(tempDir, filename)
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to create test file %s: %v", filename, err)
		}
	}

	tests := []struct {
		name     string
		path     string
		target   any
		wantErr  bool
		validate func(t *testing.T, target any)
	}{
		{
			name:   "unmarshal to struct",
			path:   filepath.Join(tempDir, "user.json"),
			target: &TestUser{},
			validate: func(t *testing.T, target any) {
				user := target.(*TestUser)
				if user.Name != "John Doe" || user.Age != 30 || user.Email != "john@example.com" {
					t.Errorf("Expected user data, got %+v", user)
				}
			},
		},
		{
			name:   "unmarshal to map",
			path:   filepath.Join(tempDir, "map.json"),
			target: &map[string]any{},
			validate: func(t *testing.T, target any) {
				data := target.(*map[string]any)
				m := *data
				if m["string"] != "value" || m["number"].(float64) != 42 {
					t.Errorf("Expected map data, got %+v", m)
				}
			},
		},
		{
			name:   "unmarshal nested config",
			path:   filepath.Join(tempDir, "config.json"),
			target: &TestConfig{},
			validate: func(t *testing.T, target any) {
				config := target.(*TestConfig)
				if config.Database.Host != "localhost" || config.Database.Port != 5432 {
					t.Errorf("Expected config data, got %+v", config)
				}
				if len(config.Features) != 3 || config.Features[0] != "auth" {
					t.Errorf("Expected features array, got %+v", config.Features)
				}
			},
		},
		{
			name:    "file not found",
			path:    filepath.Join(tempDir, "nonexistent.json"),
			target:  &map[string]any{},
			wantErr: true,
		},
		{
			name:    "invalid JSON",
			path:    filepath.Join(tempDir, "invalid.json"),
			target:  &map[string]any{},
			wantErr: true,
		},
		{
			name:    "empty file",
			path:    filepath.Join(tempDir, "empty.json"),
			target:  &map[string]any{},
			wantErr: true,
		},
		{
			name:    "nil target",
			path:    filepath.Join(tempDir, "user.json"),
			target:  nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := UnmarshalFromFile(tt.path, tt.target)

			if (err != nil) != tt.wantErr {
				t.Errorf("UnmarshalFromFile() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && tt.validate != nil {
				tt.validate(t, tt.target)
			}
		})
	}
}

func TestProcessorMarshalToFile(t *testing.T) {
	processor := New()
	defer processor.Close()

	tempDir := t.TempDir()

	tests := []struct {
		name    string
		path    string
		data    any
		pretty  []bool
		wantErr bool
	}{
		{
			name: "processor marshal compact",
			path: filepath.Join(tempDir, "processor_user.json"),
			data: TestUser{Name: "Processor User", Age: 35, Email: "processor@example.com"},
		},
		{
			name:   "processor marshal pretty",
			path:   filepath.Join(tempDir, "processor_user_pretty.json"),
			data:   TestUser{Name: "Pretty User", Age: 28, Email: "pretty@example.com"},
			pretty: []bool{true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var err error
			if len(tt.pretty) > 0 {
				err = processor.MarshalToFile(tt.path, tt.data, tt.pretty[0])
			} else {
				err = processor.MarshalToFile(tt.path, tt.data)
			}

			if (err != nil) != tt.wantErr {
				t.Errorf("Processor.MarshalToFile() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				// Verify file was created and contains expected data
				if _, err := os.Stat(tt.path); os.IsNotExist(err) {
					t.Errorf("File was not created: %s", tt.path)
				}
			}
		})
	}
}

func TestProcessorUnmarshalFromFile(t *testing.T) {
	processor := New()
	defer processor.Close()

	tempDir := t.TempDir()

	// Create test file
	testData := TestUser{Name: "Test User", Age: 40, Email: "test@example.com"}
	testPath := filepath.Join(tempDir, "test_user.json")

	if err := processor.MarshalToFile(testPath, testData); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tests := []struct {
		name    string
		path    string
		target  any
		wantErr bool
	}{
		{
			name:   "processor unmarshal success",
			path:   testPath,
			target: &TestUser{},
		},
		{
			name:    "processor unmarshal nil target",
			path:    testPath,
			target:  nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := processor.UnmarshalFromFile(tt.path, tt.target)

			if (err != nil) != tt.wantErr {
				t.Errorf("Processor.UnmarshalFromFile() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				user := tt.target.(*TestUser)
				if user.Name != testData.Name || user.Age != testData.Age {
					t.Errorf("Expected %+v, got %+v", testData, user)
				}
			}
		})
	}
}

func TestRoundTripMarshalUnmarshal(t *testing.T) {
	tempDir := t.TempDir()

	originalData := TestConfig{
		Database: struct {
			Host     string `json:"host"`
			Port     int    `json:"port"`
			Username string `json:"username"`
		}{
			Host:     "database.example.com",
			Port:     3306,
			Username: "dbuser",
		},
		Features: []string{"caching", "monitoring", "backup"},
		Debug:    false,
	}

	testPath := filepath.Join(tempDir, "roundtrip.json")

	// Marshal to file
	if err := MarshalToFile(testPath, originalData, true); err != nil {
		t.Fatalf("MarshalToFile failed: %v", err)
	}

	// Unmarshal from file
	var loadedData TestConfig
	if err := UnmarshalFromFile(testPath, &loadedData); err != nil {
		t.Fatalf("UnmarshalFromFile failed: %v", err)
	}

	// Compare data
	if loadedData.Database.Host != originalData.Database.Host {
		t.Errorf("Host mismatch: expected %s, got %s", originalData.Database.Host, loadedData.Database.Host)
	}
	if loadedData.Database.Port != originalData.Database.Port {
		t.Errorf("Port mismatch: expected %d, got %d", originalData.Database.Port, loadedData.Database.Port)
	}
	if len(loadedData.Features) != len(originalData.Features) {
		t.Errorf("Features length mismatch: expected %d, got %d", len(originalData.Features), len(loadedData.Features))
	}
	if loadedData.Debug != originalData.Debug {
		t.Errorf("Debug mismatch: expected %t, got %t", originalData.Debug, loadedData.Debug)
	}
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > len(substr) && containsAt(s, substr)))
}

func containsAt(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
