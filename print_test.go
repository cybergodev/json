package json

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
)

// captureStdout captures output written to stdout
func captureStdout(f func()) string {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	f()

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	io.Copy(&buf, r)
	return buf.String()
}

// captureStderr captures output written to stderr
func captureStderr(f func()) string {
	old := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	f()

	w.Close()
	os.Stderr = old

	var buf bytes.Buffer
	io.Copy(&buf, r)
	return buf.String()
}

func TestPrint(t *testing.T) {
	tests := []struct {
		name            string
		data            any
		contains        []string
		noIndentation   bool // Check that output has no indentation spaces
		startsWithBrace bool // If true, verify output starts with { not "
	}{
		{
			name: "simple map",
			data: map[string]any{
				"monitoring": true,
				"debug":      false,
			},
			contains:       []string{`"debug":false`, `"monitoring":true`},
			noIndentation: true,
		},
		{
			name: "nested object",
			data: map[string]any{
				"monitoring": true,
				"database": map[string]any{
					"name": "myDb",
					"port": "5432",
					"ssl":  true,
				},
				"debug":    false,
				"features": []string{"caching"},
			},
			contains:       []string{`"database"`, `"name":"myDb"`, `"port":"5432"`, `"ssl":true`, `"debug":false`, `"features":["caching"]`, `"monitoring":true`},
			noIndentation: true,
		},
		{
			name:     "string",
			data:     "hello",
			contains: []string{`"hello"` + "\n"},
		},
		{
			name:     "number",
			data:     42,
			contains: []string{`42` + "\n"},
		},
		{
			name:     "bool",
			data:     true,
			contains: []string{`true` + "\n"},
		},
		{
			name:           "JSON string - no double encoding",
			data:           `{"name":"John","age":30}`,
			contains:       []string{`"name":"John"`, `"age":30`},
			noIndentation:  true,
			startsWithBrace: true, // Should start with {, not "
		},
		{
			name:           "JSON []byte - no double encoding",
			data:           []byte(`{"name":"Jane","active":true}`),
			contains:       []string{`"name":"Jane"`, `"active":true`},
			noIndentation:  true,
			startsWithBrace: true, // Should start with {, not "
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := captureStdout(func() {
				Print(tt.data)
			})

			// Check for expected content
			for _, s := range tt.contains {
				if !contains(output, s) {
					t.Errorf("Print() output = %q, should contain %q", output, s)
				}
			}

			// For compact format, there should be no indentation (two spaces)
			if tt.noIndentation && bytes.Contains([]byte(output), []byte("  ")) {
				// Strip trailing newline first
				stripped := bytes.TrimRight([]byte(output), "\n")
				if bytes.Contains(stripped, []byte("  ")) {
					t.Errorf("Print() compact output should not have indentation, got: %q", output)
				}
			}

			// For JSON string inputs, verify no double-encoding
			// The output should start with { or [, not " (which would indicate it was encoded as a string literal)
			if tt.startsWithBrace {
				stripped := bytes.TrimRight([]byte(output), "\n")
				// Check if it starts with " (quote) - that would mean double-encoded
				if bytes.HasPrefix(stripped, []byte("\"")) {
					t.Errorf("Print() should not double-encode JSON string. Output starts with quote: %q", output)
				}
				// Verify it starts with { (opening brace for JSON object)
				if !bytes.HasPrefix(stripped, []byte("{")) {
					t.Errorf("Print() JSON string output should start with {. Got: %q", output)
				}
			}
		})
	}
}

// Note: contains() helper is already defined in package_api_test.go

func TestPrintPretty(t *testing.T) {
	tests := []struct {
		name            string
		data            any
		contains        []string
		mustHaveNewline bool
	}{
		{
			name: "simple map",
			data: map[string]any{
				"monitoring": true,
				"debug":      false,
			},
			contains:        []string{`"debug": false`, `"monitoring": true`},
			mustHaveNewline: true,
		},
		{
			name: "nested object",
			data: map[string]any{
				"monitoring": true,
				"database": map[string]any{
					"name": "myDb",
					"port": "5432",
					"ssl":  true,
				},
				"debug":    false,
				"features": []string{"caching"},
			},
			contains:        []string{`"database"`, `"name": "myDb"`, `"port": "5432"`, `"ssl": true`, `"debug": false`, `"features"`, `"caching"`, `"monitoring": true`},
			mustHaveNewline: true,
		},
		{
			name:            "JSON string - no double encoding, pretty format",
			data:            `{"name":"John","age":30}`,
			contains:        []string{`"name": "John"`, `"age": 30`},
			mustHaveNewline: true,
		},
		{
			name:            "JSON []byte - no double encoding, pretty format",
			data:            []byte(`{"name":"Jane","active":true}`),
			contains:        []string{`"name": "Jane"`, `"active": true`},
			mustHaveNewline: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := captureStdout(func() {
				PrintPretty(tt.data)
			})

			// Check for newlines (pretty formatting)
			if tt.mustHaveNewline && !bytes.Contains([]byte(output), []byte("\n")) {
				t.Errorf("PrintPretty() output should contain newlines, got: %q", output)
			}

			// Check for indentation
			if tt.mustHaveNewline && !bytes.Contains([]byte(output), []byte("  ")) {
				t.Errorf("PrintPretty() output should contain indentation, got: %q", output)
			}

			// Check for expected content
			for _, s := range tt.contains {
				if !contains(output, s) {
					t.Errorf("PrintPretty() output = %q, should contain %q", output, s)
				}
			}

			// For JSON string inputs, verify no double-encoding
			if strings.Contains(tt.name, "no double encoding") {
				// Output should NOT be a JSON string (i.e., should not start with "{\n  \"" which would indicate it was escaped)
				stripped := bytes.TrimRight([]byte(output), "\n")
				if bytes.HasPrefix(stripped, []byte("{\n  \"")) || bytes.HasPrefix(stripped, []byte("{\"")) {
					// This is actually valid for the pretty case - we need to check the whole structure
					// The key is that we should see the actual JSON keys, not an escaped string
					if !contains(output, `"name":`) {
						t.Errorf("PrintPretty() should not double-encode JSON string, got: %q", output)
					}
				}
			}
		})
	}
}

func TestPrintError(t *testing.T) {
	// Test that Print handles errors by writing to stderr
	stderr := captureStderr(func() {
		// Channel is not serializable, should cause an error
		Print(make(chan int))
	})

	if stderr == "" {
		t.Error("Print() should write error to stderr for unserializable data")
	}
}

func TestPrintPrettyError(t *testing.T) {
	// Test that PrintPretty handles errors by writing to stderr
	stderr := captureStderr(func() {
		// Channel is not serializable, should cause an error
		PrintPretty(make(chan int))
	})

	if stderr == "" {
		t.Error("PrintPretty() should write error to stderr for unserializable data")
	}
}
