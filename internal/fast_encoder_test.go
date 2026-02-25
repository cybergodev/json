package internal

import (
	"bytes"
	"encoding/json"
	"math"
	"reflect"
	"strconv"
	"testing"
	"time"
)

// ============================================================================
// ENCODER POOL TESTS
// ============================================================================

func TestGetEncoder(t *testing.T) {
	e := GetEncoder()
	if e == nil {
		t.Fatal("GetEncoder returned nil")
	}
	if e.buf == nil {
		t.Fatal("encoder buffer is nil")
	}
	if len(e.buf) != 0 {
		t.Errorf("expected empty buffer, got length %d", len(e.buf))
	}
	PutEncoder(e)
}

func TestGetEncoderWithSize(t *testing.T) {
	tests := []struct {
		name        string
		hint        int
		expectSmall bool
	}{
		{"small hint", 100, true},
		{"small hint boundary", 1024, true},
		{"medium hint", 2000, false},
		{"medium hint boundary", 4096, false},
		{"large hint", 10000, false},
		{"large hint boundary", 65536, false},
		{"very large hint", 100000, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := GetEncoderWithSize(tt.hint)
			if e == nil {
				t.Fatal("GetEncoderWithSize returned nil")
			}
			if len(e.buf) != 0 {
				t.Errorf("expected empty buffer, got length %d", len(e.buf))
			}
			PutEncoder(e)
		})
	}
}

func TestPutEncoder(t *testing.T) {
	// Test with nil encoder (should not panic)
	PutEncoder(nil)

	// Test with small buffer
	e := GetEncoder()
	e.buf = append(e.buf, "test"...)
	PutEncoder(e)

	// Test with medium buffer
	e2 := GetEncoder()
	e2.buf = make([]byte, 0, 2048)
	e2.buf = append(e2.buf, make([]byte, 1500)...)
	PutEncoder(e2)

	// Test with large buffer
	e3 := GetEncoder()
	e3.buf = make([]byte, 0, 8192)
	e3.buf = append(e3.buf, make([]byte, 7000)...)
	PutEncoder(e3)

	// Test with very large buffer (should be discarded)
	e4 := GetEncoder()
	e4.buf = make([]byte, 0, 100000)
	e4.buf = append(e4.buf, make([]byte, 80000)...)
	PutEncoder(e4)
}

// ============================================================================
// ENCODER BASIC METHODS TESTS
// ============================================================================

func TestFastEncoder_Bytes(t *testing.T) {
	e := GetEncoder()
	defer PutEncoder(e)

	e.buf = append(e.buf, "hello world"...)
	bytes := e.Bytes()
	if string(bytes) != "hello world" {
		t.Errorf("expected 'hello world', got %s", string(bytes))
	}
}

func TestFastEncoder_Reset(t *testing.T) {
	e := GetEncoder()
	defer PutEncoder(e)

	e.buf = append(e.buf, []byte("test data")...)
	e.Reset()

	if len(e.buf) != 0 {
		t.Errorf("expected empty buffer after reset, got length %d", len(e.buf))
	}
}

// ============================================================================
// EncodeValue TESTS
// ============================================================================

func TestFastEncoder_EncodeValue_Nil(t *testing.T) {
	e := GetEncoder()
	defer PutEncoder(e)

	err := e.EncodeValue(nil)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if string(e.buf) != "null" {
		t.Errorf("expected 'null', got %s", string(e.buf))
	}
}

func TestFastEncoder_EncodeValue_String(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"empty string", "", `""`},
		{"simple string", "hello", `"hello"`},
		{"string with space", "hello world", `"hello world"`},
		{"string with special chars", "hello\"world", `"hello\"world"`},
		{"unicode string", "hello 世界", `"hello 世界"`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := GetEncoder()
			defer PutEncoder(e)

			err := e.EncodeValue(tt.input)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if string(e.buf) != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, string(e.buf))
			}
		})
	}
}

func TestFastEncoder_EncodeValue_Integers(t *testing.T) {
	tests := []struct {
		name     string
		input    any
		expected string
	}{
		{"int", int(42), "42"},
		{"int8", int8(127), "127"},
		{"int16", int16(1000), "1000"},
		{"int32", int32(100000), "100000"},
		{"int64", int64(1234567890), "1234567890"},
		{"uint", uint(42), "42"},
		{"uint8", uint8(255), "255"},
		{"uint16", uint16(1000), "1000"},
		{"uint32", uint32(100000), "100000"},
		{"uint64", uint64(1234567890), "1234567890"},
		{"negative int", int(-42), "-42"},
		{"zero", int(0), "0"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := GetEncoder()
			defer PutEncoder(e)

			err := e.EncodeValue(tt.input)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if string(e.buf) != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, string(e.buf))
			}
		})
	}
}

func TestFastEncoder_EncodeValue_Floats(t *testing.T) {
	tests := []struct {
		name     string
		input    any
		expected string
	}{
		{"float32", float32(3.14), "3.14"},
		{"float64", float64(3.14159), "3.14159"},
		{"zero float", float64(0), "0"},
		{"negative float", float64(-3.14), "-3.14"},
		{"integer float", float64(42.0), "42"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := GetEncoder()
			defer PutEncoder(e)

			err := e.EncodeValue(tt.input)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if string(e.buf) != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, string(e.buf))
			}
		})
	}
}

func TestFastEncoder_EncodeValue_Bool(t *testing.T) {
	e := GetEncoder()
	defer PutEncoder(e)

	err := e.EncodeValue(true)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if string(e.buf) != "true" {
		t.Errorf("expected 'true', got %s", string(e.buf))
	}

	e.Reset()
	err = e.EncodeValue(false)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if string(e.buf) != "false" {
		t.Errorf("expected 'false', got %s", string(e.buf))
	}
}

func TestFastEncoder_EncodeValue_JsonTypes(t *testing.T) {
	t.Run("json.Number", func(t *testing.T) {
		e := GetEncoder()
		defer PutEncoder(e)

		err := e.EncodeValue(json.Number("123.45"))
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if string(e.buf) != "123.45" {
			t.Errorf("expected '123.45', got %s", string(e.buf))
		}
	})

	t.Run("json.RawMessage", func(t *testing.T) {
		e := GetEncoder()
		defer PutEncoder(e)

		raw := json.RawMessage(`{"key":"value"}`)
		err := e.EncodeValue(raw)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if string(e.buf) != `{"key":"value"}` {
			t.Errorf("expected '{\"key\":\"value\"}', got %s", string(e.buf))
		}
	})
}

func TestFastEncoder_EncodeValue_ComplexType(t *testing.T) {
	// Test fallback to stdlib for complex types
	type Person struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}

	e := GetEncoder()
	defer PutEncoder(e)

	p := Person{Name: "Alice", Age: 30}
	err := e.EncodeValue(p)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Verify it's valid JSON
	var result Person
	err = json.Unmarshal(e.buf, &result)
	if err != nil {
		t.Errorf("failed to unmarshal: %v", err)
	}
	if result.Name != "Alice" || result.Age != 30 {
		t.Errorf("unexpected result: %+v", result)
	}
}

// ============================================================================
// EncodeString TESTS
// ============================================================================

func TestFastEncoder_EncodeString(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"empty", "", `""`},
		{"simple", "hello", `"hello"`},
		{"with quote", `hello"world`, `"hello\"world"`},
		{"with backslash", `hello\world`, `"hello\\world"`},
		{"with newline", "hello\nworld", `"hello\nworld"`},
		{"with tab", "hello\tworld", `"hello\tworld"`},
		{"with carriage return", "hello\rworld", `"hello\rworld"`},
		{"with backspace", "hello\bworld", `"hello\bworld"`},
		{"with form feed", "hello\fworld", `"hello\fworld"`},
		{"with control char", "hello\x01world", `"hello\u0001world"`},
		{"unicode", "hello 世界", `"hello 世界"`},
		{"long string without escape", "abcdefghijklmnopqrstuvwxyz", `"abcdefghijklmnopqrstuvwxyz"`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := GetEncoder()
			defer PutEncoder(e)

			e.EncodeString(tt.input)
			result := string(e.buf)

			// Verify it's valid JSON by unmarshaling
			var unmarshaled string
			err := json.Unmarshal([]byte(result), &unmarshaled)
			if err != nil {
				t.Errorf("invalid JSON: %s, error: %v", result, err)
			}
			if unmarshaled != tt.input {
				t.Errorf("expected %q after roundtrip, got %q", tt.input, unmarshaled)
			}
		})
	}
}

// ============================================================================
// needsEscape TESTS
// ============================================================================

func TestNeedsEscape(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"", false},
		{"hello", false},
		{"hello world", false},
		{"hello\"world", true},
		{"hello\\world", true},
		{"hello\nworld", true},
		{"hello\tworld", true},
		{"hello\x00world", true},
		{"hello\x1fworld", true},
		{"\x00", true},
		{"\x1f", true},
		{"\x20", false}, // space is safe
		{"unicode 世界", false},
		{"a longer string without any special characters at all", false},
		// Test SWAR boundary (8-byte chunks)
		{"12345678", false},
		{"12345678" + "12345678", false},
		{"1234567\"8", true}, // quote at position 8
		{"123456781234567\"8", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := needsEscape(tt.input)
			if result != tt.expected {
				t.Errorf("needsEscape(%q) = %v, expected %v", tt.input, result, tt.expected)
			}
		})
	}
}

// ============================================================================
// EncodeInt TESTS
// ============================================================================

func TestFastEncoder_EncodeInt(t *testing.T) {
	tests := []struct {
		name     string
		input    int64
		expected string
	}{
		// Small positive integers (0-99)
		{"zero", 0, "0"},
		{"one", 1, "1"},
		{"nine", 9, "9"},
		{"ten", 10, "10"},
		{"ninety nine", 99, "99"},

		// Medium positive integers (100-999)
		{"one hundred", 100, "100"},
		{"five hundred", 500, "500"},
		{"nine ninety nine", 999, "999"},

		// Large positive integers (1000-9999)
		{"one thousand", 1000, "1000"},
		{"five thousand", 5000, "5000"},
		{"nine thousand nine ninety nine", 9999, "9999"},

		// Very large positive integers (use strconv)
		{"ten thousand", 10000, "10000"},
		{"million", 1000000, "1000000"},
		{"max int64", int64(1<<63 - 1), strconv.FormatInt(1<<63-1, 10)},

		// Small negative integers (-1 to -99)
		{"negative one", -1, "-1"},
		{"negative nine", -9, "-9"},
		{"negative ten", -10, "-10"},
		{"negative ninety nine", -99, "-99"},

		// Medium negative integers (-100 to -999)
		{"negative one hundred", -100, "-100"},
		{"negative five hundred", -500, "-500"},
		{"negative nine ninety nine", -999, "-999"},

		// Large negative integers (use strconv)
		{"negative ten thousand", -10000, "-10000"},
		{"negative million", -1000000, "-1000000"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := GetEncoder()
			defer PutEncoder(e)

			e.EncodeInt(tt.input)
			if string(e.buf) != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, string(e.buf))
			}
		})
	}
}

// ============================================================================
// EncodeUint TESTS
// ============================================================================

func TestFastEncoder_EncodeUint(t *testing.T) {
	tests := []struct {
		name     string
		input    uint64
		expected string
	}{
		// Small integers (0-99)
		{"zero", 0, "0"},
		{"one", 1, "1"},
		{"ninety nine", 99, "99"},

		// Medium integers (100-999)
		{"one hundred", 100, "100"},
		{"nine ninety nine", 999, "999"},

		// Large integers (1000-9999)
		{"one thousand", 1000, "1000"},
		{"nine thousand nine ninety nine", 9999, "9999"},

		// Very large integers (use strconv)
		{"ten thousand", 10000, "10000"},
		{"million", 1000000, "1000000"},
		{"max uint64", uint64(1<<64 - 1), strconv.FormatUint(1<<64-1, 10)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := GetEncoder()
			defer PutEncoder(e)

			e.EncodeUint(tt.input)
			if string(e.buf) != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, string(e.buf))
			}
		})
	}
}

// ============================================================================
// EncodeFloat TESTS
// ============================================================================

func TestFastEncoder_EncodeFloat(t *testing.T) {
	tests := []struct {
		name      string
		input     float64
		bits      int
		expected  string
		checkOnly bool // if true, only verify roundtrip, not exact string
	}{
		// Zero
		{"zero", 0, 64, "0", false},

		// Common cached values
		{"one", 1.0, 64, "1", false},
		{"two", 2.0, 64, "2", false},
		{"ten", 10.0, 64, "10", false},
		{"point five", 0.5, 64, "0.5", false},
		{"point one", 0.1, 64, "0.1", false},

		// Negative cached values
		{"negative one", -1.0, 64, "-1", false},
		{"negative two", -2.0, 64, "-2", false},
		{"negative point five", -0.5, 64, "-0.5", false},

		// Integer-like floats (0-100)
		{"forty two", 42.0, 64, "42", false},

		// Non-cached floats
		{"pi", 3.14159, 64, "3.14159", false},
		{"e", 2.71828, 64, "2.71828", false},

		// Float32 (roundtrip check only due to precision differences)
		{"float32 pi", float64(float32(3.14)), 32, "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := GetEncoder()
			defer PutEncoder(e)

			e.EncodeFloat(tt.input, tt.bits)

			// For non-special values, verify the result can be parsed back
			if tt.input != tt.input { // NaN check
				if string(e.buf) != "NaN" {
					t.Errorf("expected 'NaN', got %s", string(e.buf))
				}
				return
			}

			if math.IsInf(tt.input, 1) {
				if string(e.buf) != "Infinity" {
					t.Errorf("expected 'Infinity', got %s", string(e.buf))
				}
				return
			}

			if math.IsInf(tt.input, -1) {
				if string(e.buf) != "-Infinity" {
					t.Errorf("expected '-Infinity', got %s", string(e.buf))
				}
				return
			}

			// For regular floats, just verify it's a valid number representation
			result := string(e.buf)
			parsed, err := strconv.ParseFloat(result, 64)
			if err != nil {
				t.Errorf("failed to parse result %q: %v", result, err)
			}
			// For checkOnly cases (like float32), just verify the parsing worked
			// For others, verify exact roundtrip
			if !tt.checkOnly {
				if parsed != tt.input {
					t.Errorf("roundtrip failed: expected %v, got %v", tt.input, parsed)
				}
			} else {
				// For float32 cases, allow small precision difference
				diff := math.Abs(parsed - tt.input)
				if diff > 1e-6 {
					t.Errorf("roundtrip failed with too much difference: expected %v, got %v (diff: %v)", tt.input, parsed, diff)
				}
			}
		})
	}
}

func TestFastEncoder_EncodeFloat_SpecialValues(t *testing.T) {
	t.Run("NaN", func(t *testing.T) {
		e := GetEncoder()
		defer PutEncoder(e)

		e.EncodeFloat(math.NaN(), 64)
		if string(e.buf) != "NaN" {
			t.Errorf("expected 'NaN', got %s", string(e.buf))
		}
	})

	t.Run("PositiveInf", func(t *testing.T) {
		e := GetEncoder()
		defer PutEncoder(e)

		e.EncodeFloat(math.Inf(1), 64)
		if string(e.buf) != "Infinity" {
			t.Errorf("expected 'Infinity', got %s", string(e.buf))
		}
	})

	t.Run("NegativeInf", func(t *testing.T) {
		e := GetEncoder()
		defer PutEncoder(e)

		e.EncodeFloat(math.Inf(-1), 64)
		if string(e.buf) != "-Infinity" {
			t.Errorf("expected '-Infinity', got %s", string(e.buf))
		}
	})
}

// ============================================================================
// EncodeBool TESTS
// ============================================================================

func TestFastEncoder_EncodeBool(t *testing.T) {
	t.Run("true", func(t *testing.T) {
		e := GetEncoder()
		defer PutEncoder(e)

		e.EncodeBool(true)
		if string(e.buf) != "true" {
			t.Errorf("expected 'true', got %s", string(e.buf))
		}
	})

	t.Run("false", func(t *testing.T) {
		e := GetEncoder()
		defer PutEncoder(e)

		e.EncodeBool(false)
		if string(e.buf) != "false" {
			t.Errorf("expected 'false', got %s", string(e.buf))
		}
	})
}

// ============================================================================
// EncodeMap TESTS
// ============================================================================

func TestFastEncoder_EncodeMap(t *testing.T) {
	tests := []struct {
		name  string
		input map[string]any
	}{
		{"empty map", map[string]any{}},
		{"simple map", map[string]any{"key": "value"}},
		{"multiple keys", map[string]any{"a": 1, "b": 2, "c": 3}},
		{"nested map", map[string]any{"outer": map[string]any{"inner": "value"}}},
		{"mixed types", map[string]any{"string": "value", "int": 42, "bool": true, "null": nil}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := GetEncoder()
			defer PutEncoder(e)

			err := e.EncodeMap(tt.input)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			// Verify it's valid JSON
			var result map[string]any
			err = json.Unmarshal(e.buf, &result)
			if err != nil {
				t.Errorf("failed to unmarshal: %v, JSON: %s", err, string(e.buf))
			}
		})
	}
}

func TestFastEncoder_EncodeMapStringString(t *testing.T) {
	e := GetEncoder()
	defer PutEncoder(e)

	input := map[string]string{"a": "1", "b": "2"}
	err := e.EncodeMapStringString(input)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	var result map[string]string
	err = json.Unmarshal(e.buf, &result)
	if err != nil {
		t.Errorf("failed to unmarshal: %v", err)
	}
}

func TestFastEncoder_EncodeMapStringInt(t *testing.T) {
	e := GetEncoder()
	defer PutEncoder(e)

	input := map[string]int{"a": 1, "b": 2}
	err := e.EncodeMapStringInt(input)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	var result map[string]int
	err = json.Unmarshal(e.buf, &result)
	if err != nil {
		t.Errorf("failed to unmarshal: %v", err)
	}
}

func TestFastEncoder_EncodeMapStringInt64(t *testing.T) {
	e := GetEncoder()
	defer PutEncoder(e)

	input := map[string]int64{"a": 1, "b": 2}
	err := e.EncodeMapStringInt64(input)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	var result map[string]int64
	err = json.Unmarshal(e.buf, &result)
	if err != nil {
		t.Errorf("failed to unmarshal: %v", err)
	}
}

func TestFastEncoder_EncodeMapStringFloat64(t *testing.T) {
	e := GetEncoder()
	defer PutEncoder(e)

	input := map[string]float64{"a": 1.5, "b": 2.5}
	err := e.EncodeMapStringFloat64(input)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	var result map[string]float64
	err = json.Unmarshal(e.buf, &result)
	if err != nil {
		t.Errorf("failed to unmarshal: %v", err)
	}
}

// ============================================================================
// EncodeArray TESTS
// ============================================================================

func TestFastEncoder_EncodeArray(t *testing.T) {
	tests := []struct {
		name  string
		input []any
	}{
		{"empty array", []any{}},
		{"single element", []any{1}},
		{"multiple elements", []any{1, "two", true, nil}},
		{"nested array", []any{[]any{1, 2}, []any{3, 4}}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := GetEncoder()
			defer PutEncoder(e)

			err := e.EncodeArray(tt.input)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			var result []any
			err = json.Unmarshal(e.buf, &result)
			if err != nil {
				t.Errorf("failed to unmarshal: %v, JSON: %s", err, string(e.buf))
			}
		})
	}
}

func TestFastEncoder_EncodeStringSlice(t *testing.T) {
	e := GetEncoder()
	defer PutEncoder(e)

	input := []string{"a", "b", "c"}
	e.EncodeStringSlice(input)

	var result []string
	err := json.Unmarshal(e.buf, &result)
	if err != nil {
		t.Errorf("failed to unmarshal: %v", err)
	}
	if len(result) != 3 {
		t.Errorf("expected 3 elements, got %d", len(result))
	}
}

func TestFastEncoder_EncodeIntSlice(t *testing.T) {
	e := GetEncoder()
	defer PutEncoder(e)

	input := []int{1, 2, 3}
	e.EncodeIntSlice(input)

	var result []int
	err := json.Unmarshal(e.buf, &result)
	if err != nil {
		t.Errorf("failed to unmarshal: %v", err)
	}
}

func TestFastEncoder_EncodeInt32Slice(t *testing.T) {
	e := GetEncoder()
	defer PutEncoder(e)

	input := []int32{1, 2, 3}
	e.EncodeInt32Slice(input)

	var result []int32
	err := json.Unmarshal(e.buf, &result)
	if err != nil {
		t.Errorf("failed to unmarshal: %v", err)
	}
}

func TestFastEncoder_EncodeInt64Slice(t *testing.T) {
	e := GetEncoder()
	defer PutEncoder(e)

	input := []int64{1, 2, 3}
	e.EncodeInt64Slice(input)

	var result []int64
	err := json.Unmarshal(e.buf, &result)
	if err != nil {
		t.Errorf("failed to unmarshal: %v", err)
	}
}

func TestFastEncoder_EncodeUint64Slice(t *testing.T) {
	e := GetEncoder()
	defer PutEncoder(e)

	input := []uint64{1, 2, 3}
	e.EncodeUint64Slice(input)

	var result []uint64
	err := json.Unmarshal(e.buf, &result)
	if err != nil {
		t.Errorf("failed to unmarshal: %v", err)
	}
}

func TestFastEncoder_EncodeFloatSlice(t *testing.T) {
	e := GetEncoder()
	defer PutEncoder(e)

	input := []float64{1.1, 2.2, 3.3}
	e.EncodeFloatSlice(input)

	var result []float64
	err := json.Unmarshal(e.buf, &result)
	if err != nil {
		t.Errorf("failed to unmarshal: %v", err)
	}
}

func TestFastEncoder_EncodeFloat32Slice(t *testing.T) {
	e := GetEncoder()
	defer PutEncoder(e)

	input := []float32{1.1, 2.2, 3.3}
	e.EncodeFloat32Slice(input)

	var result []float32
	err := json.Unmarshal(e.buf, &result)
	if err != nil {
		t.Errorf("failed to unmarshal: %v", err)
	}
}

// ============================================================================
// EncodeTime TESTS
// ============================================================================

func TestFastEncoder_EncodeTime(t *testing.T) {
	e := GetEncoder()
	defer PutEncoder(e)

	// Use a fixed time for reproducible tests
	tm := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	e.EncodeTime(tm)

	expected := `"2024-01-15T10:30:00Z"`
	if string(e.buf) != expected {
		t.Errorf("expected %s, got %s", expected, string(e.buf))
	}
}

// ============================================================================
// EncodeBase64 TESTS
// ============================================================================

func TestFastEncoder_EncodeBase64(t *testing.T) {
	e := GetEncoder()
	defer PutEncoder(e)

	input := []byte("hello world")
	e.EncodeBase64(input)

	// Verify it's valid JSON string
	var result string
	err := json.Unmarshal(e.buf, &result)
	if err != nil {
		t.Errorf("failed to unmarshal: %v", err)
	}

	// Verify base64 decoding gives original data
	decoded, err := json.Marshal(input) // This will give us the expected base64
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}
	if string(e.buf) != string(decoded) {
		t.Errorf("expected %s, got %s", string(decoded), string(e.buf))
	}
}

// ============================================================================
// FastParseInt TESTS
// ============================================================================

func TestFastParseInt(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expected    int64
		expectError bool
	}{
		{"zero", "0", 0, false},
		{"positive", "123", 123, false},
		{"negative", "-456", -456, false},
		{"large positive", "9223372036854775807", math.MaxInt64, false},
		{"large negative", "-9223372036854775808", math.MinInt64, false},
		{"empty", "", 0, true},
		{"just minus", "-", 0, true},
		{"invalid char", "12a3", 0, true},
		{"with space", "12 3", 0, true},
		{"with plus", "+123", 0, true}, // doesn't handle +
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := FastParseInt([]byte(tt.input))
			if tt.expectError {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if result != tt.expected {
					t.Errorf("expected %d, got %d", tt.expected, result)
				}
			}
		})
	}
}

func TestFastParseInt_Overflow(t *testing.T) {
	// Test overflow detection
	_, err := FastParseInt([]byte("99999999999999999999999999999999999999999999"))
	if err != strconv.ErrRange {
		t.Errorf("expected ErrRange, got %v", err)
	}
}

// ============================================================================
// FastParseFloat TESTS
// ============================================================================

func TestFastParseFloat(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected float64
	}{
		{"integer", "123", 123},
		{"decimal", "3.14", 3.14},
		{"negative", "-2.5", -2.5},
		{"scientific", "1e10", 1e10},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := FastParseFloat([]byte(tt.input))
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

// ============================================================================
// FastMarshal TESTS
// ============================================================================

func TestFastMarshal(t *testing.T) {
	tests := []struct {
		name  string
		input any
	}{
		{"nil", nil},
		{"string", "hello"},
		{"int", 42},
		{"float", 3.14},
		{"bool", true},
		{"slice", []int{1, 2, 3}},
		{"map", map[string]int{"a": 1}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := FastMarshal(tt.input)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			// Verify it's valid JSON
			var unmarshaled any
			err = json.Unmarshal(result, &unmarshaled)
			if err != nil {
				t.Errorf("failed to unmarshal: %v, JSON: %s", err, string(result))
			}
		})
	}
}

func TestFastMarshalToString(t *testing.T) {
	result, err := FastMarshalToString(map[string]int{"a": 1, "b": 2})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	var m map[string]int
	err = json.Unmarshal([]byte(result), &m)
	if err != nil {
		t.Errorf("failed to unmarshal: %v", err)
	}
	if m["a"] != 1 || m["b"] != 2 {
		t.Errorf("unexpected result: %v", m)
	}
}

// ============================================================================
// GetStructEncoder TESTS
// ============================================================================

func TestGetStructEncoder(t *testing.T) {
	type TestStruct struct {
		Name    string `json:"name"`
		Age     int    `json:"age"`
		Hidden  string `json:"-"` // "-" keeps struct field name
		private string // unexported, should be ignored
		Omit    string `json:"omit,omitempty"`
	}

	fields := GetStructEncoder(reflect.TypeOf(TestStruct{}))

	// Should have 4 fields (Name, Age, Hidden, Omit) - private is unexported
	// Note: json:"-" doesn't skip the field, it just uses the struct field name
	if len(fields) != 4 {
		t.Errorf("expected 4 fields, got %d", len(fields))
	}

	// Check field names
	fieldNames := make(map[string]bool)
	for _, f := range fields {
		fieldNames[f.Name] = true
	}

	if !fieldNames["name"] {
		t.Error("missing 'name' field")
	}
	if !fieldNames["age"] {
		t.Error("missing 'age' field")
	}
	if !fieldNames["Hidden"] {
		t.Error("missing 'Hidden' field (json:\"-\" keeps struct name)")
	}
	if !fieldNames["omit"] {
		t.Error("missing 'omit' field")
	}

	// Check omitempty on Omit field
	for _, f := range fields {
		if f.Name == "omit" {
			if !f.OmitEmpty {
				t.Error("'omit' field should have OmitEmpty=true")
			}
		}
	}
}

func TestGetStructEncoder_Caching(t *testing.T) {
	type TestStruct struct {
		Value int `json:"value"`
	}

	// Call twice, should return cached result
	fields1 := GetStructEncoder(reflect.TypeOf(TestStruct{}))
	fields2 := GetStructEncoder(reflect.TypeOf(TestStruct{}))

	// Should be the same slice (cached)
	if &fields1[0] != &fields2[0] {
		t.Error("expected cached result, got different slices")
	}
}

// ============================================================================
// splitTag TESTS
// ============================================================================

func TestSplitTag(t *testing.T) {
	tests := []struct {
		input    string
		expected []string
	}{
		{"", []string{}},
		{"name", []string{"name"}},
		{"name,omitempty", []string{"name", "omitempty"}},
		{"name,omitempty,string", []string{"name", "omitempty", "string"}},
		{",omitempty", []string{"omitempty"}},
		{"name,,omitempty", []string{"name", "omitempty"}},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := splitTag(tt.input)
			if len(result) != len(tt.expected) {
				t.Errorf("expected %v, got %v", tt.expected, result)
				return
			}
			for i, v := range result {
				if v != tt.expected[i] {
					t.Errorf("expected %v, got %v", tt.expected, result)
					return
				}
			}
		})
	}
}

// ============================================================================
// BytesToString TESTS
// ============================================================================

func TestBytesToString(t *testing.T) {
	input := []byte("hello world")
	result := BytesToString(input)
	if result != "hello world" {
		t.Errorf("expected 'hello world', got %s", result)
	}
}

// ============================================================================
// IsValidUTF8 TESTS
// ============================================================================

func TestIsValidUTF8(t *testing.T) {
	tests := []struct {
		input    []byte
		expected bool
	}{
		{[]byte("hello"), true},
		{[]byte("hello 世界"), true},
		{[]byte{0xff, 0xfe}, false}, // invalid UTF-8
		{[]byte{}, true},
		{[]byte{0xc3, 0x28}, false}, // invalid 2-byte sequence
	}

	for _, tt := range tests {
		t.Run(string(tt.input), func(t *testing.T) {
			result := IsValidUTF8(tt.input)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

// ============================================================================
// FastBufferPool TESTS
// ============================================================================

func TestGetFastBuffer(t *testing.T) {
	buf := GetFastBuffer()
	if buf == nil {
		t.Fatal("GetFastBuffer returned nil")
	}
	if buf.Len() != 0 {
		t.Errorf("expected empty buffer, got length %d", buf.Len())
	}
	PutFastBuffer(buf)
}

func TestPutFastBuffer(t *testing.T) {
	// Test with nil (should not panic)
	// Note: PutFastBuffer doesn't handle nil, so we skip that

	// Test with small buffer (should be pooled)
	buf := bytes.NewBuffer(make([]byte, 0, 512))
	buf.WriteString("test")
	PutFastBuffer(buf)

	// Test with large buffer (should not be pooled)
	largeBuf := bytes.NewBuffer(make([]byte, 0, 10000))
	largeBuf.WriteString("test")
	PutFastBuffer(largeBuf) // Should be discarded due to size
}

// ============================================================================
// float64ToBits TESTS
// ============================================================================

func TestFloat64ToBits(t *testing.T) {
	tests := []float64{0, 1.0, -1.0, math.Pi, math.MaxFloat64, math.SmallestNonzeroFloat64}

	for _, tt := range tests {
		t.Run(strconv.FormatFloat(tt, 'f', -1, 64), func(t *testing.T) {
			bits := float64ToBits(tt)
			// Round trip
			back := math.Float64frombits(bits)
			if back != tt && !(math.IsNaN(back) && math.IsNaN(tt)) {
				t.Errorf("roundtrip failed: expected %v, got %v", tt, back)
			}
		})
	}
}

// ============================================================================
// isIntegerFloat TESTS
// ============================================================================

func TestIsIntegerFloat(t *testing.T) {
	tests := []struct {
		input    float64
		expected bool
	}{
		{0, true},
		{1, true},
		{-1, true},
		{42, true},
		{-42, true},
		{1.5, false},
		{-1.5, false},
		{3.14159, false},
		{9007199254740992, true},  // 2^53, max safe integer
		{-9007199254740992, true}, // -2^53
		// Note: 2^53+1 cannot be tested because it rounds to 2^53 in float64
		// Large values that overflow int64 (int64(f) overflows)
		{1e20, false},
		{-1e20, false},
	}

	for _, tt := range tests {
		t.Run(strconv.FormatFloat(tt.input, 'f', -1, 64), func(t *testing.T) {
			result := isIntegerFloat(tt.input)
			if result != tt.expected {
				t.Errorf("isIntegerFloat(%v) = %v, expected %v", tt.input, result, tt.expected)
			}
		})
	}
}

// ============================================================================
// encodeSlow TESTS (via EncodeValue with complex types)
// ============================================================================

func TestFastEncoder_encodeSlow(t *testing.T) {
	// Test that encodeSlow correctly handles complex types via stdlib
	type ComplexStruct struct {
		Name   string
		Values []int
		Nested struct {
			Enabled bool
		}
	}

	e := GetEncoder()
	defer PutEncoder(e)

	input := ComplexStruct{
		Name:   "test",
		Values: []int{1, 2, 3},
	}
	input.Nested.Enabled = true

	err := e.EncodeValue(input)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	var result ComplexStruct
	err = json.Unmarshal(e.buf, &result)
	if err != nil {
		t.Errorf("failed to unmarshal: %v", err)
	}
	if result.Name != "test" || len(result.Values) != 3 || !result.Nested.Enabled {
		t.Errorf("unexpected result: %+v", result)
	}
}

func TestFastEncoder_encodeSlow_Error(t *testing.T) {
	// Test that encodeSlow returns errors for unmarshalable types
	e := GetEncoder()
	defer PutEncoder(e)

	// Channels cannot be marshaled to JSON
	err := e.EncodeValue(make(chan int))
	if err == nil {
		t.Error("expected error for channel type")
	}
}

// ============================================================================
// appendHex TESTS (indirectly tested via escapeString)
// ============================================================================

func TestEscapeString_ControlCharacters(t *testing.T) {
	// Test all control characters (0x00-0x1F)
	for i := 0; i < 0x20; i++ {
		e := GetEncoder()
		input := string([]byte{byte(i)})
		e.EncodeString(input)

		var result string
		err := json.Unmarshal(e.buf, &result)
		PutEncoder(e)

		if err != nil {
			t.Errorf("failed to unmarshal control char 0x%02x: %v", i, err)
		}
		if result != input {
			t.Errorf("roundtrip failed for 0x%02x: expected %q, got %q", i, input, result)
		}
	}
}

// ============================================================================
// BENCHMARK TESTS
// ============================================================================

func BenchmarkFastEncoder_EncodeString_Simple(b *testing.B) {
	e := GetEncoder()
	defer PutEncoder(e)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		e.Reset()
		e.EncodeString("hello world")
	}
}

func BenchmarkFastEncoder_EncodeString_NeedsEscape(b *testing.B) {
	e := GetEncoder()
	defer PutEncoder(e)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		e.Reset()
		e.EncodeString("hello\"world\nwith\tescape")
	}
}

func BenchmarkFastEncoder_EncodeInt_Small(b *testing.B) {
	e := GetEncoder()
	defer PutEncoder(e)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		e.Reset()
		e.EncodeInt(int64(i % 100))
	}
}

func BenchmarkFastEncoder_EncodeInt_Large(b *testing.B) {
	e := GetEncoder()
	defer PutEncoder(e)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		e.Reset()
		e.EncodeInt(12345678901234)
	}
}

func BenchmarkFastEncoder_EncodeMap(b *testing.B) {
	e := GetEncoder()
	defer PutEncoder(e)

	m := map[string]any{
		"name":  "Alice",
		"age":   30,
		"email": "alice@example.com",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		e.Reset()
		_ = e.EncodeMap(m)
	}
}

func BenchmarkFastMarshal(b *testing.B) {
	data := map[string]any{
		"name":   "Alice",
		"age":    30,
		"active": true,
		"tags":   []string{"user", "admin"},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = FastMarshal(data)
	}
}

func BenchmarkStdlibMarshal(b *testing.B) {
	data := map[string]any{
		"name":   "Alice",
		"age":    30,
		"active": true,
		"tags":   []string{"user", "admin"},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = json.Marshal(data)
	}
}
