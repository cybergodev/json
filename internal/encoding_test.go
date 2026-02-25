package internal

import (
	"bytes"
	"testing"
)

// TestMarshalJSON tests the MarshalJSON function
func TestMarshalJSON(t *testing.T) {
	tests := []struct {
		name     string
		value    any
		pretty   bool
		prefix   string
		indent   string
		wantErr  bool
		contains string
	}{
		{
			name:     "simple object compact",
			value:    map[string]any{"name": "test"},
			pretty:   false,
			contains: `"name"`,
		},
		{
			name:     "simple object pretty",
			value:    map[string]any{"name": "test"},
			pretty:   true,
			prefix:   "",
			indent:   "  ",
			contains: "\n",
		},
		{
			name:     "array compact",
			value:    []any{1, 2, 3},
			pretty:   false,
			contains: "[",
		},
		{
			name:     "array pretty",
			value:    []any{1, 2, 3},
			pretty:   true,
			prefix:   "  ",
			indent:   "  ",
			contains: "\n",
		},
		{
			name:     "nil value",
			value:    nil,
			pretty:   false,
			contains: "null",
		},
		{
			name:     "boolean true",
			value:    true,
			pretty:   false,
			contains: "true",
		},
		{
			name:     "boolean false",
			value:    false,
			pretty:   false,
			contains: "false",
		},
		{
			name:     "number",
			value:    42,
			pretty:   false,
			contains: "42",
		},
		{
			name:     "string",
			value:    "hello",
			pretty:   false,
			contains: `"hello"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := MarshalJSON(tt.value, tt.pretty, tt.prefix, tt.indent)
			if (err != nil) != tt.wantErr {
				t.Errorf("MarshalJSON() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.contains != "" && !bytes.Contains([]byte(result), []byte(tt.contains)) {
				t.Errorf("MarshalJSON() = %v, want to contain %v", result, tt.contains)
			}
		})
	}
}

// TestEncoderBuffer tests the buffer pool operations
func TestEncoderBuffer(t *testing.T) {
	t.Run("Get and Put", func(t *testing.T) {
		buf := GetEncoderBuffer()
		if buf == nil {
			t.Fatal("GetEncoderBuffer returned nil")
		}

		buf.WriteString("test data")
		if buf.String() != "test data" {
			t.Errorf("Buffer content = %q, want %q", buf.String(), "test data")
		}

		PutEncoderBuffer(buf)
	})

	t.Run("Reset on reuse", func(t *testing.T) {
		buf1 := GetEncoderBuffer()
		buf1.WriteString("first data")
		PutEncoderBuffer(buf1)

		buf2 := GetEncoderBuffer()
		// Buffer should be reset
		if buf2.Len() != 0 {
			t.Errorf("Buffer length = %d, want 0", buf2.Len())
		}
		PutEncoderBuffer(buf2)
	})

	t.Run("Nil buffer", func(t *testing.T) {
		// Should not panic
		PutEncoderBuffer(nil)
	})

	t.Run("Oversized buffer discarded", func(t *testing.T) {
		buf := GetEncoderBuffer()
		// Grow beyond max size
		buf.Grow(16 * 1024)
		buf.WriteString("oversized")
		// This should be discarded, not pooled
		PutEncoderBuffer(buf)
	})
}

// TestByteSlice tests the byte slice pool operations
func TestByteSlice(t *testing.T) {
	t.Run("Get and Put", func(t *testing.T) {
		slice := GetByteSlice()
		if slice == nil {
			t.Fatal("GetByteSlice returned nil")
		}

		*slice = append(*slice, "test"...)
		if string(*slice) != "test" {
			t.Errorf("Slice content = %q, want %q", string(*slice), "test")
		}

		PutByteSlice(slice)
	})

	t.Run("Nil slice", func(t *testing.T) {
		// Should not panic
		PutByteSlice(nil)
	})

	t.Run("Reset on reuse", func(t *testing.T) {
		slice1 := GetByteSlice()
		*slice1 = append(*slice1, "data"...)
		PutByteSlice(slice1)

		slice2 := GetByteSlice()
		// Slice should be reset
		if len(*slice2) != 0 {
			t.Errorf("Slice length = %d, want 0", len(*slice2))
		}
		PutByteSlice(slice2)
	})
}

// TestStringToBytes tests string to bytes conversion
func TestStringToBytes(t *testing.T) {
	tests := []string{
		"",
		"hello",
		"test with spaces",
		"unicode: 你好世界",
	}

	for _, tt := range tests {
		t.Run(tt, func(t *testing.T) {
			result := StringToBytes(tt)
			if string(result) != tt {
				t.Errorf("StringToBytes(%q) = %q, want %q", tt, string(result), tt)
			}
		})
	}
}

// TestContainsAnyByte tests the ContainsAnyByte function
func TestContainsAnyByte(t *testing.T) {
	tests := []struct {
		s        string
		chars    string
		expected bool
	}{
		{"hello", "aeiou", true},
		{"rhythm", "aeiou", false},
		{"test", "xyz", false},
		{"", "abc", false},
		{"abc", "", false},
		{"hello world", " ", true},
		{"12345", "0123456789", true},
		{`test"value`, `"`, true},
		{`test\value`, `\`, true},
	}

	for _, tt := range tests {
		t.Run(tt.s+"_"+tt.chars, func(t *testing.T) {
			if got := ContainsAnyByte(tt.s, tt.chars); got != tt.expected {
				t.Errorf("ContainsAnyByte(%q, %q) = %v, want %v", tt.s, tt.chars, got, tt.expected)
			}
		})
	}
}

// TestIsValidNumberString tests number string validation
func TestIsValidNumberString(t *testing.T) {
	tests := []struct {
		s        string
		expected bool
	}{
		{"123", true},
		{"-123", true},
		{"12.34", true},
		{"-12.34", true},
		{"0", true},
		{"0.0", true},
		{"", false},
		{"abc", false},
		{"12.34.56", false},
		{"--123", false},
		{"123abc", false},
		{".5", true},
		{"1e10", true},
		{"1E-5", true},
		{"+123", true}, // json.Number accepts leading +
	}

	for _, tt := range tests {
		t.Run(tt.s, func(t *testing.T) {
			if got := IsValidNumberString(tt.s); got != tt.expected {
				t.Errorf("IsValidNumberString(%q) = %v, want %v", tt.s, got, tt.expected)
			}
		})
	}
}

// TestParseIntFast tests fast integer parsing
func TestParseIntFast(t *testing.T) {
	tests := []struct {
		s        string
		expected int
		ok       bool
	}{
		{"0", 0, true},
		{"1", 1, true},
		{"9", 9, true},
		{"10", 10, true},
		{"99", 99, true},
		{"100", 100, true},
		{"12345", 12345, true},
		{"-1", -1, true},
		{"-99", -99, true},
		{"-123", -123, true},
		{"", 0, false},
		{"-", 0, false},
		{"abc", 0, false},
		{"12a", 0, false},
		{"a12", 0, false},
		{"--1", 0, false},
		{"1.5", 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.s, func(t *testing.T) {
			got, ok := ParseIntFast(tt.s)
			if ok != tt.ok {
				t.Errorf("ParseIntFast(%q) ok = %v, want %v", tt.s, ok, tt.ok)
			}
			if ok && got != tt.expected {
				t.Errorf("ParseIntFast(%q) = %d, want %d", tt.s, got, tt.expected)
			}
		})
	}
}

// TestIntToStringFast tests fast integer to string conversion
func TestIntToStringFast(t *testing.T) {
	tests := []struct {
		n        int
		expected string
	}{
		{0, "0"},
		{1, "1"},
		{9, "9"},
		{10, "10"},
		{50, "50"},
		{99, "99"},
		{100, "100"},
		{1000, "1000"},
		{-1, "-1"},
		{-99, "-99"},
		{-100, "-100"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if got := IntToStringFast(tt.n); got != tt.expected {
				t.Errorf("IntToStringFast(%d) = %q, want %q", tt.n, got, tt.expected)
			}
		})
	}
}

// TestEncodeFast tests the EncodeFast function
func TestEncodeFast(t *testing.T) {
	tests := []struct {
		name     string
		value    any
		expected string
		handled  bool
	}{
		{"nil", nil, "null", true},
		{"bool true", true, "true", true},
		{"bool false", false, "false", true},
		{"int", 42, "42", true},
		{"int8", int8(127), "127", true},
		{"int16", int16(1000), "1000", true},
		{"int32", int32(50000), "50000", true},
		{"int64", int64(1000000), "1000000", true},
		{"uint", uint(42), "42", true},
		{"uint8", uint8(255), "255", true},
		{"uint16", uint16(1000), "1000", true},
		{"uint32", uint32(50000), "50000", true},
		{"uint64", uint64(1000000), "1000000", true},
		{"float32", float32(3.14), "", true},
		{"float64", float64(3.14159), "", true},
		{"string", "hello", `"hello"`, true},
		{"string with escapes", `hello"world`, `"hello\"world"`, true},
		{"string with newline", "hello\nworld", `"hello\nworld"`, true},
		{"string with tab", "hello\tworld", `"hello\tworld"`, true},
		{"string with backslash", `hello\world`, `"hello\\world"`, true},
		{"string with carriage return", "hello\rworld", `"hello\rworld"`, true},
		{"string with form feed", "hello\fworld", `"hello\fworld"`, true},
		{"string with backspace", "hello\bworld", `"hello\bworld"`, true},
		{"string with control char", "hello\x01world", `"hello\u0001world"`, true},
		{"map", map[string]any{"key": "value"}, "", false},
		{"slice", []int{1, 2, 3}, "", false},
		{"struct", struct{ Name string }{"test"}, "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			handled := EncodeFast(tt.value, &buf)

			if handled != tt.handled {
				t.Errorf("EncodeFast(%v) handled = %v, want %v", tt.value, handled, tt.handled)
			}

			if handled && tt.expected != "" {
				if buf.String() != tt.expected {
					t.Errorf("EncodeFast(%v) = %q, want %q", tt.value, buf.String(), tt.expected)
				}
			}
		})
	}
}

// TestWriteEscapedStringFast tests string escaping
func TestWriteEscapedStringFast(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"simple", "hello", "hello"},
		{"quote", `hello"world`, `hello\"world`},
		{"backslash", `hello\world`, `hello\\world`},
		{"newline", "hello\nworld", `hello\nworld`},
		{"tab", "hello\tworld", `hello\tworld`},
		{"carriage return", "hello\rworld", `hello\rworld`},
		{"form feed", "hello\fworld", `hello\fworld`},
		{"backspace", "hello\bworld", `hello\bworld`},
		{"multiple escapes", "a\"b\nc\td", `a\"b\nc\td`},
		{"control char", "\x01", `\u0001`},
		{"unicode", "hello世界", "hello世界"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			writeEscapedStringFast(&buf, tt.input)
			if buf.String() != tt.expected {
				t.Errorf("writeEscapedStringFast(%q) = %q, want %q", tt.input, buf.String(), tt.expected)
			}
		})
	}
}

// Benchmark tests for encoding functions
func BenchmarkMarshalJSON_Compact(b *testing.B) {
	data := map[string]any{"name": "test", "value": 42}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = MarshalJSON(data, false, "", "")
	}
}

func BenchmarkMarshalJSON_Pretty(b *testing.B) {
	data := map[string]any{"name": "test", "value": 42}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = MarshalJSON(data, true, "", "  ")
	}
}

func BenchmarkParseIntFast(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = ParseIntFast("12345")
	}
}

func BenchmarkIntToStringFast(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = IntToStringFast(50)
	}
}

func BenchmarkEncodeFast(b *testing.B) {
	var buf bytes.Buffer
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf.Reset()
		_ = EncodeFast("test string", &buf)
	}
}
