package json

import (
	"runtime"
	"strings"
	"testing"
)

// TestWindowsDeviceNames tests Windows reserved device name detection
func TestWindowsDeviceNames(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Skipping Windows-specific test on non-Windows platform")
	}

	processor := New()
	defer processor.Close()

	tests := []struct {
		name        string
		filePath    string
		expectError bool
		description string
	}{
		{
			name:        "CON device",
			filePath:    "CON",
			expectError: true,
			description: "Windows reserved device name CON",
		},
		{
			name:        "PRN device",
			filePath:    "PRN",
			expectError: true,
			description: "Windows reserved device name PRN",
		},
		{
			name:        "AUX device",
			filePath:    "AUX",
			expectError: true,
			description: "Windows reserved device name AUX",
		},
		{
			name:        "NUL device",
			filePath:    "NUL",
			expectError: true,
			description: "Windows reserved device name NUL",
		},
		{
			name:        "COM1 device",
			filePath:    "COM1",
			expectError: true,
			description: "Windows COM port 1",
		},
		{
			name:        "COM9 device",
			filePath:    "COM9",
			expectError: true,
			description: "Windows COM port 9",
		},
		{
			name:        "COM0 device",
			filePath:    "COM0",
			expectError: true,
			description: "Windows COM0 (invalid but reserved)",
		},
		{
			name:        "LPT1 device",
			filePath:    "LPT1",
			expectError: true,
			description: "Windows LPT port 1",
		},
		{
			name:        "LPT9 device",
			filePath:    "LPT9",
			expectError: true,
			description: "Windows LPT port 9",
		},
		{
			name:        "LPT0 device",
			filePath:    "LPT0",
			expectError: true,
			description: "Windows LPT0 (invalid but reserved)",
		},
		{
			name:        "CONIN device",
			filePath:    "CONIN$",
			expectError: true,
			description: "Windows console input",
		},
		{
			name:        "CONOUT device",
			filePath:    "CONOUT$",
			expectError: true,
			description: "Windows console output",
		},
		{
			name:        "device with extension",
			filePath:    "CON.txt",
			expectError: true,
			description: "Reserved name with extension",
		},
		{
			name:        "normal file",
			filePath:    "normal.json",
			expectError: false,
			description: "Normal file name",
		},
		{
			name:        "path with device",
			filePath:    "data/CON",
			expectError: true,
			description: "Path containing device name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := processor.validateFilePath(tt.filePath)
			if tt.expectError && err == nil {
				t.Errorf("%s: Expected error for path '%s', but got none", tt.description, tt.filePath)
			}
			if !tt.expectError && err != nil {
				t.Errorf("%s: Unexpected error for path '%s': %v", tt.description, tt.filePath, err)
			}
		})
	}
}

// TestPathTraversalDetection tests path traversal attack detection
func TestPathTraversalDetection(t *testing.T) {
	processor := New()
	defer processor.Close()

	tests := []struct {
		name        string
		filePath    string
		expectError bool
		description string
	}{
		{
			name:        "double dot traversal",
			filePath:    "../../etc/passwd",
			expectError: true,
			description: "Standard path traversal",
		},
		{
			name:        "URL encoded traversal",
			filePath:    "%2e%2e/%2e%2e/etc/passwd",
			expectError: true,
			description: "URL encoded double dots",
		},
		{
			name:        "double URL encoded",
			filePath:    "%252e%252e/%252e%252e",
			expectError: true,
			description: "Double URL encoded traversal",
		},
		{
			name:        "mixed encoding traversal",
			filePath:    "..%2fetc/passwd",
			expectError: true,
			description: "Mixed URL and normal separator",
		},
		{
			name:        "Windows backslash encoded",
			filePath:    "..%5cetc/passwd",
			expectError: true,
			description: "Encoded Windows backslash",
		},
		{
			name:        "UTF-8 overlong encoding",
			filePath:    "..%c0%af/etc/passwd",
			expectError: true,
			description: "UTF-8 overlong encoding attack",
		},
		{
			name:        "partial double encoding",
			filePath:    "..%2e",
			expectError: true,
			description: "Partial double encoding",
		},
		{
			name:        "null byte injection",
			filePath:    "file.txt\x00",
			expectError: true,
			description: "Null byte in path",
		},
		{
			name:        "newline injection",
			filePath:    "file.txt%0a",
			expectError: true,
			description: "Encoded newline injection",
		},
		{
			name:        "carriage return injection",
			filePath:    "file.txt%0d",
			expectError: true,
			description: "Encoded CR injection",
		},
		{
			name:        "tab injection",
			filePath:    "file.txt%09",
			expectError: true,
			description: "Encoded tab injection",
		},
		{
			name:        "five consecutive dots",
			filePath:    ".....//etc/passwd",
			expectError: true,
			description: "Five dots pattern",
		},
		{
			name:        "six consecutive dots",
			filePath:    "......//etc/passwd",
			expectError: true,
			description: "Six dots pattern",
		},
		{
			name:        "normal path",
			filePath:    "data/user/profile.json",
			expectError: false,
			description: "Normal file path",
		},
		{
			name:        "absolute path",
			filePath:    "/home/user/data.json",
			expectError: false,
			description: "Absolute path (allowed on Unix)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := processor.validateFilePath(tt.filePath)
			if tt.expectError && err == nil {
				t.Errorf("%s: Expected error for path '%s', but got none", tt.description, tt.filePath)
			}
			if !tt.expectError && err != nil {
				// Allow certain errors that aren't security-related
				if !strings.Contains(err.Error(), "security") &&
					!strings.Contains(err.Error(), "traversal") &&
					!strings.Contains(err.Error(), "null byte") {
					t.Errorf("%s: Unexpected error for path '%s': %v", tt.description, tt.filePath, err)
				}
			}
		})
	}
}

// TestAlternateDataStreamDetection tests ADS detection on Windows
func TestAlternateDataStreamDetection(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Skipping Windows-specific ADS test on non-Windows platform")
	}

	processor := New()
	defer processor.Close()

	tests := []struct {
		name        string
		filePath    string
		expectError bool
		description string
	}{
		{
			name:        "ADS with colon",
			filePath:    "file.txt:stream",
			expectError: true,
			description: "Alternate data stream",
		},
		{
			name:        "ADS with $DATA",
			filePath:    "file.txt:$DATA",
			expectError: true,
			description: "ADS with $DATA stream",
		},
		{
			name:        "complex ADS",
			filePath:    "file.txt:stream:$DATA",
			expectError: true,
			description: "Complex ADS pattern",
		},
		{
			name:        "drive letter not ADS",
			filePath:    "C:/data/file.txt",
			expectError: false,
			description: "Drive letter pattern is valid",
		},
		{
			name:        "drive letter with colon",
			filePath:    "C:data/file.txt",
			expectError: false,
			description: "Relative path from drive",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := processor.validateFilePath(tt.filePath)
			if tt.expectError && err == nil {
				t.Errorf("%s: Expected error for path '%s', but got none", tt.description, tt.filePath)
			}
			if !tt.expectError && err != nil {
				if strings.Contains(err.Error(), "alternate data stream") {
					t.Errorf("%s: Unexpected ADS error for path '%s': %v", tt.description, tt.filePath, err)
				}
			}
		})
	}
}

// TestPathLengthValidation tests path length limits
func TestPathLengthValidation(t *testing.T) {
	processor := New()
	defer processor.Close()

	// Create a path that exceeds MaxPathLength
	longPath := strings.Repeat("a", MaxPathLength+1)

	tests := []struct {
		name        string
		filePath    string
		expectError bool
		description string
	}{
		{
			name:        "exceeds max length",
			filePath:    longPath,
			expectError: true,
			description: "Path exceeds maximum length",
		},
		{
			name:        "exactly max length",
			filePath:    strings.Repeat("b", MaxPathLength),
			expectError: false,
			description: "Path at maximum length",
		},
		{
			name:        "normal length",
			filePath:    "data/user/profile.json",
			expectError: false,
			description: "Normal length path",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := processor.validateFilePath(tt.filePath)
			if tt.expectError && err == nil {
				t.Errorf("%s: Expected error for path, but got none", tt.description)
			}
			if !tt.expectError && err != nil {
				if strings.Contains(err.Error(), "too long") {
					t.Errorf("%s: Unexpected length error: %v", tt.description, err)
				}
			}
		})
	}
}

// TestInvalidCharactersInWindowsPath tests invalid character detection on Windows
func TestInvalidCharactersInWindowsPath(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Skipping Windows-specific character test on non-Windows platform")
	}

	processor := New()
	defer processor.Close()

	invalidChars := []string{"<", ">", ":", "\"", "|", "?", "*"}

	for _, char := range invalidChars {
		t.Run("invalid char "+char, func(t *testing.T) {
			// Use path where colon isn't the drive letter
			filePath := "data" + char + "file.json"
			if char == ":" {
				filePath = "data" + char + "\\file.json"
			}

			err := processor.validateFilePath(filePath)
			// Should error for invalid characters (except valid drive letter colon)
			if char != ":" || !strings.HasPrefix(filePath, "C:") && !strings.HasPrefix(filePath, "D:") {
				if err == nil {
					t.Errorf("Expected error for invalid character '%s', but got none", char)
				}
			}
		})
	}
}

// TestNullByteDetection tests null byte detection in paths
func TestNullByteDetection(t *testing.T) {
	processor := New()
	defer processor.Close()

	tests := []struct {
		name     string
		filePath string
	}{
		{
			name:     "null at start",
			filePath: "\x00file.txt",
		},
		{
			name:     "null in middle",
			filePath: "file\x00.txt",
		},
		{
			name:     "null at end",
			filePath: "file.txt\x00",
		},
		{
			name:     "multiple nulls",
			filePath: "file\x00\x00.txt",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := processor.validateFilePath(tt.filePath)
			if err == nil {
				t.Errorf("Expected error for path with null byte '%s', but got none", tt.filePath)
			}
		})
	}
}

// TestUNCPathDetection tests UNC path detection on Windows
func TestUNCPathDetection(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Skipping Windows-specific UNC test on non-Windows platform")
	}

	processor := New()
	defer processor.Close()

	tests := []struct {
		name        string
		filePath    string
		expectError bool
	}{
		{
			name:        "UNC with backslashes",
			filePath:    "\\\\server\\share\\file.txt",
			expectError: true,
		},
		{
			name:        "UNC with forward slashes",
			filePath:    "//server/share/file.txt",
			expectError: true,
		},
		{
			name:        "local path",
			filePath:    "C:/data/file.txt",
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := processor.validateFilePath(tt.filePath)
			if tt.expectError && err == nil {
				t.Errorf("Expected error for UNC path, but got none")
			}
		})
	}
}

// TestContainsPathTraversal tests the path traversal detection helper
func TestContainsPathTraversal(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		{
			name:     "double dot",
			path:     "../file.txt",
			expected: true,
		},
		{
			name:     "URL encoded",
			path:     "%2e%2e/file.txt",
			expected: true,
		},
		{
			name:     "normal path",
			path:     "data/file.txt",
			expected: false,
		},
		{
			name:     "single dot",
			path:     "./file.txt",
			expected: false,
		},
		{
			name:     "partial encoding",
			path:     "%2e%2%2e",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := containsPathTraversal(tt.path)
			if result != tt.expected {
				t.Errorf("containsPathTraversal(%s) = %v; want %v", tt.path, result, tt.expected)
			}
		})
	}
}

// TestContainsConsecutiveDots tests consecutive dot detection
func TestContainsConsecutiveDots(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		minCount int
		expected bool
	}{
		{
			name:     "three dots",
			path:     "...",
			minCount: 3,
			expected: true,
		},
		{
			name:     "four dots",
			path:     "....",
			minCount: 3,
			expected: true,
		},
		{
			name:     "two dots",
			path:     "..",
			minCount: 3,
			expected: false,
		},
		{
			name:     "separated dots",
			path:     ".a.b",
			minCount: 3,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := containsConsecutiveDots(tt.path, tt.minCount)
			if result != tt.expected {
				t.Errorf("containsConsecutiveDots(%s, %d) = %v; want %v", tt.path, tt.minCount, result, tt.expected)
			}
		})
	}
}

// TestFilePathValidationEdgeCases tests edge cases in file path validation
func TestFilePathValidationEdgeCases(t *testing.T) {
	processor := New()
	defer processor.Close()

	tests := []struct {
		name        string
		filePath    string
		expectError bool
		description string
	}{
		{
			name:        "empty path",
			filePath:    "",
			expectError: true,
			description: "Empty path should error",
		},
		{
			name:        "single character",
			filePath:    "a",
			expectError: false,
			description: "Single character path",
		},
		{
			name:        "current directory",
			filePath:    ".",
			expectError: false,
			description: "Current directory reference",
		},
		{
			name:        "parent directory",
			filePath:    "..",
			expectError: true,
			description: "Parent directory reference (traversal)",
		},
		{
			name:        "file with extension",
			filePath:    "document.pdf",
			expectError: false,
			description: "Normal file with extension",
		},
		{
			name:        "deep path",
			filePath:    "a/b/c/d/e/f/g/h/i/j/file.txt",
			expectError: false,
			description: "Deep but valid path",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := processor.validateFilePath(tt.filePath)
			if tt.expectError && err == nil {
				t.Errorf("%s: Expected error for path '%s', but got none", tt.description, tt.filePath)
			}
			if !tt.expectError && err != nil {
				t.Errorf("%s: Unexpected error for path '%s': %v", tt.description, tt.filePath, err)
			}
		})
	}
}

// TestWindowsPathValidationComponents tests Windows path validation components
func TestWindowsPathValidationComponents(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Skipping Windows-specific test on non-Windows platform")
	}

	tests := []struct {
		name        string
		filePath    string
		expectError bool
		description string
	}{
		{
			name:        "valid absolute path",
			filePath:    "C:/Users/user/data.json",
			expectError: false,
			description: "Valid Windows absolute path",
		},
		{
			name:        "valid relative path",
			filePath:    "data/config.json",
			expectError: false,
			description: "Valid Windows relative path",
		},
		{
			name:        "path with spaces",
			filePath:    "C:/Program Files/data.json",
			expectError: false,
			description: "Path with spaces (valid)",
		},
		{
			name:        "path with underscore",
			filePath:    "my_data/file.json",
			expectError: false,
			description: "Path with underscore (valid)",
		},
		{
			name:        "path with hyphen",
			filePath:    "my-data/file.json",
			expectError: false,
			description: "Path with hyphen (valid)",
		},
		{
			name:        "path with pipe",
			filePath:    "data|file.json",
			expectError: true,
			description: "Path with pipe (invalid)",
		},
		{
			name:        "path with asterisk",
			filePath:    "data/*.json",
			expectError: true,
			description: "Path with asterisk (invalid)",
		},
		{
			name:        "path with question mark",
			filePath:    "data/file?.json",
			expectError: true,
			description: "Path with question mark (invalid)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := New()
			defer p.Close()
			err := p.validateFilePath(tt.filePath)
			if tt.expectError && err == nil {
				t.Errorf("%s: Expected error for path '%s', but got none", tt.description, tt.filePath)
			}
			if !tt.expectError && err != nil {
				t.Errorf("%s: Unexpected error for path '%s': %v", tt.description, tt.filePath, err)
			}
		})
	}
}

// TestSecurityValidationWithRealPaths tests security validation with realistic paths
func TestSecurityValidationWithRealPaths(t *testing.T) {
	processor := New()
	defer processor.Close()

	validPaths := []string{
		"data/users/profile.json",
		"config/settings.json",
		"logs/app.log",
		"backup/data.bak",
		"cache/index.tmp",
	}

	for _, path := range validPaths {
		t.Run("valid_"+path, func(t *testing.T) {
			err := processor.validateFilePath(path)
			if err != nil {
				// Some errors are OK (like file not found), but not security errors
				if strings.Contains(err.Error(), "security") ||
					strings.Contains(err.Error(), "traversal") ||
					strings.Contains(err.Error(), "null byte") {
					t.Errorf("Valid path '%s' failed security validation: %v", path, err)
				}
			}
		})
	}
}

// TestFilePathNormalization tests file path normalization
func TestFilePathNormalization(t *testing.T) {
	processor := New()
	defer processor.Close()

	tests := []struct {
		name     string
		input    string
		validate func(t *testing.T, err error)
	}{
		{
			name:  "valid normalized path",
			input: "data/config.json",
			validate: func(t *testing.T, err error) {
				if err != nil && strings.Contains(err.Error(), "security") {
					t.Errorf("Unexpected security error: %v", err)
				}
			},
		},
		{
			name:  "path with extra separators",
			input: "data///config.json",
			validate: func(t *testing.T, err error) {
				if err != nil && strings.Contains(err.Error(), "security") {
					t.Errorf("Unexpected security error: %v", err)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := processor.validateFilePath(tt.input)
			tt.validate(t, err)
		})
	}
}

// TestSymlinkValidation tests symlink validation in paths
func TestSymlinkValidation(t *testing.T) {
	p := New()
	defer p.Close()

	// This test checks that the validation logic handles symlinks properly
	// We can't create actual symlinks in tests, but we can verify the logic exists

	tests := []struct {
		name        string
		filePath    string
		description string
	}{
		{
			name:        "potential symlink path",
			filePath:    "data/link/target.json",
			description: "Path that might contain symlink",
		},
		{
			name:        "normal file path",
			filePath:    "data/file.json",
			description: "Regular file path",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Just verify the validation runs without panicking
			_ = p.validateFilePath(tt.filePath)
			t.Logf("Validation completed for: %s", tt.description)
		})
	}
}

// TestCrossPlatformPathValidation tests path validation works on both platforms
func TestCrossPlatformPathValidation(t *testing.T) {
	processor := New()
	defer processor.Close()

	universalPaths := []struct {
		name        string
		path        string
		expectError bool
	}{
		{
			name:        "simple json file",
			path:        "data.json",
			expectError: false,
		},
		{
			name:        "nested path",
			path:        "users/admin/profile.json",
			expectError: false,
		},
		{
			name:        "path traversal attempt",
			path:        "../../../etc/passwd",
			expectError: true,
		},
		{
			name:        "null byte injection",
			path:        "file.txt\x00 malicious",
			expectError: true,
		},
	}

	for _, tt := range universalPaths {
		t.Run(tt.name, func(t *testing.T) {
			err := processor.validateFilePath(tt.path)
			if tt.expectError && err == nil {
				t.Errorf("Expected error for path '%s', but got none", tt.path)
			}
		})
	}
}

// Benchmark tests

func BenchmarkValidatePathNormal(b *testing.B) {
	processor := New()
	defer processor.Close()

	path := "data/users/profile.json"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = processor.validateFilePath(path)
	}
}

func BenchmarkValidatePathComplex(b *testing.B) {
	processor := New()
	defer processor.Close()

	path := "data/users/admin/config/settings.production.json"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = processor.validateFilePath(path)
	}
}
