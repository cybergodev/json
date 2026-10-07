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

			if err := e.EncodeFloat(tt.input, tt.bits); err != nil {
				t.Fatalf("EncodeFloat(%v) errored: %v", tt.input, err)
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
	// D-002/R11 (m4): NaN/Inf are not representable in JSON (RFC 8259);
	// EncodeFloat rejects them like encoding/json's UnsupportedValueError
	// instead of emitting "null" (the pre-R11 behavior).
	t.Run("NaN", func(t *testing.T) {
		e := GetEncoder()
		defer PutEncoder(e)

		if err := e.EncodeFloat(math.NaN(), 64); err == nil {
			t.Error("expected error for NaN, got nil")
		}
	})

	t.Run("PositiveInf", func(t *testing.T) {
		e := GetEncoder()
		defer PutEncoder(e)

		if err := e.EncodeFloat(math.Inf(1), 64); err == nil {
			t.Error("expected error for +Inf, got nil")
		}
	})

	t.Run("NegativeInf", func(t *testing.T) {
		e := GetEncoder()
		defer PutEncoder(e)

		if err := e.EncodeFloat(math.Inf(-1), 64); err == nil {
			t.Error("expected error for -Inf, got nil")
		}
	})

	t.Run("Float32NaN", func(t *testing.T) {
		e := GetEncoder()
		defer PutEncoder(e)

		if err := e.EncodeFloat(float64(float32(math.NaN())), 32); err == nil {
			t.Error("expected error for float32 NaN, got nil")
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
	if err := e.EncodeFloatSlice(input); err != nil {
		t.Fatalf("EncodeFloatSlice errored: %v", err)
	}

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
	if err := e.EncodeFloat32Slice(input); err != nil {
		t.Fatalf("EncodeFloat32Slice errored: %v", err)
	}

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
	if err := e.EncodeTime(tm); err != nil {
		t.Fatalf("EncodeTime errored: %v", err)
	}

	expected := `"2024-01-15T10:30:00Z"`
	if string(e.buf) != expected {
		t.Errorf("expected %s, got %s", expected, string(e.buf))
	}
}

func TestFastEncoder_EncodeTime_YearOutOfRange(t *testing.T) {
	// D-002/R11 (M3): RFC3339 cannot represent years outside [0,9999];
	// encoding/json errors for them, so the fast path must too instead of
	// emitting an invalid 5-digit-year timestamp with a nil error.
	for _, tc := range []struct {
		name string
		tm   time.Time
	}{
		{"year 10000", time.Date(10000, 1, 1, 0, 0, 0, 0, time.UTC)},
		{"negative year", time.Date(-1, 1, 1, 0, 0, 0, 0, time.UTC)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := GetEncoder()
			defer PutEncoder(e)
			if err := e.EncodeTime(tc.tm); err == nil {
				t.Error("expected year-out-of-range error, got nil")
			}
		})
	}
	t.Run("boundary years ok", func(t *testing.T) {
		e := GetEncoder()
		defer PutEncoder(e)
		if err := e.EncodeTime(time.Date(9999, 12, 31, 23, 59, 59, 0, time.UTC)); err != nil {
			t.Errorf("year 9999 should encode: %v", err)
		}
	})
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
	// Test positive overflow detection
	_, err := FastParseInt([]byte("99999999999999999999999999999999999999999999"))
	if err != strconv.ErrRange {
		t.Errorf("expected ErrRange for huge positive, got %v", err)
	}

	// Test negative overflow detection
	_, err = FastParseInt([]byte("-99999999999999999999999999999999999999999999"))
	if err != strconv.ErrRange {
		t.Errorf("expected ErrRange for huge negative, got %v", err)
	}
}

func TestFastParseInt_BoundaryValues(t *testing.T) {
	// SECURITY FIX: Test boundary values for overflow detection
	tests := []struct {
		name        string
		input       string
		expected    int64
		expectError error
	}{
		// MaxInt64 boundary
		{"MaxInt64", "9223372036854775807", math.MaxInt64, nil},
		{"MaxInt64+1", "9223372036854775808", 0, strconv.ErrRange},
		{"MaxInt64+10", "9223372036854775817", 0, strconv.ErrRange},

		// MinInt64 boundary (the critical case that was broken)
		{"MinInt64", "-9223372036854775808", math.MinInt64, nil},
		{"MinInt64-1", "-9223372036854775809", 0, strconv.ErrRange},
		{"MinInt64-10", "-9223372036854775818", 0, strconv.ErrRange},

		// Near boundary values
		{"near MaxInt64", "9223372036854775806", math.MaxInt64 - 1, nil},
		{"near MinInt64", "-9223372036854775807", math.MinInt64 + 1, nil},

		// Edge case: 10-digit numbers near overflow threshold
		{"threshold positive", "922337203685477580", 922337203685477580, nil},
		{"threshold negative", "-922337203685477580", -922337203685477580, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := FastParseInt([]byte(tt.input))
			if tt.expectError != nil {
				if err != tt.expectError {
					t.Errorf("expected error %v, got %v", tt.expectError, err)
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
		Name   string `json:"name"`
		Age    int    `json:"age"`
		Hidden string `json:"-"` // "-" keeps struct field name
		_      string // unexported, should be ignored (blank identifier to avoid unused warning)
		Omit   string `json:"omit,omitempty"`
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

// ============================================================================
// POOL STATE-LEAK REGRESSION
// ============================================================================

// TestFastEncoderPoolStateNeverLeaks locks the pool's no-leak invariant:
// neither a per-instance depth cap nor a residual nesting depth (elevated
// when an encode fails mid-container and skips its leaveContainer pairs)
// may survive a pool round-trip. Checkout (GetEncoder/GetEncoderWithSize)
// and return (PutEncoder) both run resetState; this test fails if either
// side regresses.
//
// The assertions inspect the pointer AFTER PutEncoder returns it to the
// pool, so they do not depend on sync.Pool's LIFO identity guarantees.
func TestFastEncoderPoolStateNeverLeaks(t *testing.T) {
	t.Run("return clears cap and depth", func(t *testing.T) {
		e := GetEncoder()
		e.SetMaxEncodeDepth(3)
		e.depth = 2 // simulate residual nesting from a failed encode
		PutEncoder(e)
		if e.depth != 0 || e.maxDepthCap != 0 {
			t.Errorf("returned encoder still dirty: depth=%d maxDepthCap=%d", e.depth, e.maxDepthCap)
		}
	})

	t.Run("checkout yields clean state", func(t *testing.T) {
		// Seed a dirty encoder into the pool first; the single-goroutine
		// LIFO pool hands the same object back on the next Get.
		dirty := GetEncoder()
		dirty.SetMaxEncodeDepth(3)
		dirty.depth = 2
		PutEncoder(dirty)

		for range 3 {
			e := GetEncoder()
			if e.depth != 0 {
				t.Errorf("checkout depth = %d, want 0", e.depth)
			}
			if got := e.effectiveMaxDepth(); got != MaxNestingDepth {
				t.Errorf("checkout effectiveMaxDepth = %d, want %d (stale cap leaked)", got, MaxNestingDepth)
			}
			PutEncoder(e)
		}
	})

	t.Run("depth residual after mid-container error is cleaned", func(t *testing.T) {
		e := GetEncoder()
		err := e.EncodeValue(map[string]any{"k": make(chan int)}) // unencodable value inside a container
		if err == nil {
			t.Fatal("expected encode error for chan value")
		}
		// e.depth may legitimately be elevated here (the error skips the
		// leaveContainer pairs). The pool round-trip must clear it.
		PutEncoder(e)
		if e.depth != 0 {
			t.Errorf("depth = %d after PutEncoder, want 0", e.depth)
		}
	})

	t.Run("size-tiered checkouts are clean", func(t *testing.T) {
		for _, hint := range []int{2048, 8192, 100_000} {
			e := GetEncoderWithSize(hint)
			e.SetMaxEncodeDepth(5)
			e.depth = 1
			PutEncoder(e)
			if e.depth != 0 || e.maxDepthCap != 0 {
				t.Errorf("hint %d: returned encoder dirty (depth=%d cap=%d)", hint, e.depth, e.maxDepthCap)
			}
			fresh := GetEncoderWithSize(hint)
			if fresh.depth != 0 || fresh.effectiveMaxDepth() != MaxNestingDepth {
				t.Errorf("hint %d: checkout dirty (depth=%d maxDepth=%d)", hint, fresh.depth, fresh.effectiveMaxDepth())
			}
			PutEncoder(fresh)
		}
	})
}

// ===========================================================================
// Fast-encoder boundary tests (consolidated from fast_encoder_boundary_test.go)
// ===========================================================================
// ============================================================================
// Boundary tests for internal/fast_encoder.go low-coverage paths.
// ============================================================================

// --- ClearStructEncoderCache (fast_encoder.go:1435, 0% coverage) ---

func TestClearStructEncoderCache(t *testing.T) {
	type cached struct {
		A int    `json:"a"`
		B string `json:"b"`
	}
	// Encoding a struct populates the struct-encoder cache via getEncodeFn.
	if _, err := FastMarshal(cached{A: 1, B: "x"}); err != nil {
		t.Fatalf("FastMarshal err: %v", err)
	}
	// Clearing must not panic and must leave the cache usable.
	ClearStructEncoderCache()
	if _, err := FastMarshal(cached{A: 2, B: "y"}); err != nil {
		t.Fatalf("FastMarshal after clear err: %v", err)
	}
}

// --- isEmptyValue (fast_encoder.go:1580, 0% coverage) ---

func TestIsEmptyValue(t *testing.T) {
	tests := []struct {
		name string
		v    any
		want bool
	}{
		{"zero int", 0, true},
		{"nonzero int", 42, false},
		{"empty string", "", true},
		{"nonempty string", "x", false},
		{"false", false, true},
		{"true", true, false},
		{"nil pointer", (*int)(nil), true},
		{"non-nil pointer", ptrInt(7), false},
		{"zero float", float64(0), true},
		{"nan", math.NaN(), false},
		{"empty slice", []int{}, true},
		{"nonempty slice", []int{1}, false},
		{"empty map", map[string]int{}, true},
		{"nonempty map", map[string]int{"a": 1}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isEmptyValue(reflect.ValueOf(tt.v)); got != tt.want {
				t.Errorf("isEmptyValue(%v) = %v, want %v", tt.v, got, tt.want)
			}
		})
	}
}

func ptrInt(n int) *int { return &n }

// --- FastParseFloat special-value rejection (fast_encoder.go:1298, 69%) ---

func TestFastParseFloat_SpecialValues(t *testing.T) {
	tests := []struct {
		in      string
		wantErr bool
	}{
		{"1.5", false},
		{"42", false},
		{"-3.14", false},
		{"nan", true},
		{"NaN", true},
		{"inf", true},
		{"Infinity", true},
		{"-inf", true},
		{"+nan", true},
		{"-Inf", true},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			_, err := FastParseFloat([]byte(tt.in))
			if tt.wantErr && err == nil {
				t.Errorf("FastParseFloat(%q): expected error, got nil", tt.in)
			}
			if !tt.wantErr && err != nil {
				t.Errorf("FastParseFloat(%q): unexpected error: %v", tt.in, err)
			}
		})
	}
}

// --- getEncodeFn diverse field types (fast_encoder.go:1444, 5% coverage) ---

func TestFastEncoder_DiverseFieldTypes(t *testing.T) {
	type diverse struct {
		I8  int8    `json:"i8"`
		I16 int16   `json:"i16"`
		I32 int32   `json:"i32"`
		I64 int64   `json:"i64"`
		U8  uint8   `json:"u8"`
		U16 uint16  `json:"u16"`
		U32 uint32  `json:"u32"`
		U64 uint64  `json:"u64"`
		F32 float32 `json:"f32"`
		F64 float64 `json:"f64"`
		B   bool    `json:"b"`
		S   string  `json:"s"`
	}
	v := diverse{
		I8: -8, I16: -16, I32: -32, I64: -64,
		U8: 8, U16: 16, U32: 32, U64: 64,
		F32: 1.5, F64: 2.5, B: true, S: "hi",
	}
	got, err := FastMarshal(v)
	if err != nil {
		t.Fatalf("FastMarshal err: %v", err)
	}
	want, _ := json.Marshal(v)
	if string(got) != string(want) {
		t.Errorf("diverse types mismatch:\n got %s\nwant %s", got, want)
	}
}

// --- EncodeArray error propagation (fast_encoder.go:980, 69%) ---

func TestFastEncoder_EncodeArray_ErrorPropagation(t *testing.T) {
	e := GetEncoder()
	defer PutEncoder(e)
	// An invalid json.Number inside an array forces EncodeValue to error,
	// which EncodeArray must propagate.
	err := e.EncodeArray([]any{json.Number("not-a-number")})
	if err == nil {
		t.Error("expected EncodeArray to propagate error for invalid json.Number")
	}
}

// --- GetStructEncoder + getEncodeFn (fast_encoder.go:1385/1444, 5% coverage) ---
//
// GetStructEncoder is exported public API; calling it builds per-field
// encoders via getEncodeFn for every field type, and populates the cache that
// ClearStructEncoderCache empties.

func TestGetStructEncoder_DiverseFields(t *testing.T) {
	type nested struct {
		X int `json:"x"`
	}
	type fields struct {
		I8  int8           `json:"i8"`
		I64 int64          `json:"i64"`
		U8  uint8          `json:"u8"`
		U64 uint64         `json:"u64"`
		F32 float32        `json:"f32"`
		F64 float64        `json:"f64"`
		B   bool           `json:"b"`
		S   string         `json:"s"`
		Sl  []int          `json:"sl"`
		M   map[string]int `json:"m"`
		P   *int           `json:"p"`
		N   nested         `json:"n"`
	}
	ft := reflect.TypeOf(fields{})
	info := GetStructEncoder(ft)
	if len(info) != 12 {
		t.Fatalf("GetStructEncoder returned %d fields, want 12", len(info))
	}
	// Cached lookup returns the same entry without rebuilding.
	if info2 := GetStructEncoder(ft); len(info2) != len(info) {
		t.Error("cached GetStructEncoder returned a different field count")
	}
	// Clear empties the cache; a fresh rebuild still yields the same shape.
	ClearStructEncoderCache()
	if info3 := GetStructEncoder(ft); len(info3) != 12 {
		t.Fatalf("after clear, GetStructEncoder returned %d fields, want 12", len(info3))
	}
}

// --- EncodeValue top-level type branches (fast_encoder.go:155, 71% coverage) ---

func TestFastEncoder_TopLevelValues(t *testing.T) {
	now := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name string
		v    any
	}{
		{"int8", int8(8)},
		{"int16", int16(16)},
		{"int32", int32(32)},
		{"uint8", uint8(8)},
		{"uint16", uint16(16)},
		{"uint32", uint32(32)},
		{"float32", float32(1.5)},
		{"[]int", []int{1, 2, 3}},
		{"[]int64", []int64{1, 2}},
		{"[]float64", []float64{1.5, 2.5}},
		{"[]byte", []byte("hello")},
		{"map[string]int64", map[string]int64{"a": 1}},
		{"map[string]float64", map[string]float64{"a": 1.5}},
		{"time", now},
		{"json.Number", json.Number("42")},
		{"json.RawMessage", json.RawMessage(`{"k":1}`)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := FastMarshal(tt.v)
			if err != nil {
				t.Fatalf("FastMarshal(%s) err: %v", tt.name, err)
			}
			want, werr := json.Marshal(tt.v)
			if werr != nil {
				t.Fatalf("stdlib marshal(%s) err: %v", tt.name, werr)
			}
			if string(got) != string(want) {
				t.Errorf("%s mismatch:\n got %s\nwant %s", tt.name, got, want)
			}
		})
	}
}

// --- EncodeStringSlice large pre-allocation (fast_encoder.go:1008, 58%) ---

func TestFastEncoder_EncodeStringSlice_Large(t *testing.T) {
	e := GetEncoder()
	defer PutEncoder(e)
	// >8 elements exercises the large-slice pre-allocation path.
	big := make([]string, 20)
	for i := range big {
		big[i] = "x"
	}
	e.EncodeStringSlice(big)
	want, _ := json.Marshal(big)
	if string(e.buf) != string(want) {
		t.Errorf("EncodeStringSlice mismatch:\n got %s\nwant %s", e.buf, want)
	}
}

// --- EncodeBase64 buffer growth (fast_encoder.go:1123, 70%) ---

func TestFastEncoder_EncodeBase64_Large(t *testing.T) {
	e := GetEncoder()
	defer PutEncoder(e)
	// Larger than the encoder's initial 512-byte buffer forces capacity growth.
	big := make([]byte, 1024)
	for i := range big {
		big[i] = byte(i)
	}
	e.EncodeBase64(big)
	want, _ := json.Marshal(big)
	if string(e.buf) != string(want) {
		t.Errorf("EncodeBase64 mismatch:\n got %s\nwant %s", e.buf, want)
	}
}

// --- getEncodeFn closure bodies (fast_encoder.go:1678, 24% coverage) ---
//
// GetStructEncoder builds per-field encoders via getEncodeFn, but the closures
// only execute when the field is encoded. Encode every field kind directly —
// populated AND zero values (nil pointer/map/slice/[]byte hit the "null"
// branches) — and compare against encoding/json. Types without a specialized
// encoder get nil EncodeFn; the caller then uses EncodeValue, mirroring the
// reflect.Struct consumer.

func TestGetEncodeFn_FieldClosures(t *testing.T) {
	type inner struct {
		X int `json:"x"`
	}
	type diverse struct {
		I8    int8               `json:"i8"`
		I64   int64              `json:"i64"`
		U8    uint8              `json:"u8"`
		U64   uint64             `json:"u64"`
		Up    uintptr            `json:"up"`
		F32   float32            `json:"f32"`
		F64   float64            `json:"f64"`
		B     bool               `json:"b"`
		S     string             `json:"s"`
		Bs    []byte             `json:"bs"`
		Sl    []int              `json:"sl"`
		MSS   map[string]string  `json:"mss"`
		MSI   map[string]int     `json:"msi"`
		MSI64 map[string]int64   `json:"msi64"`
		MSF   map[string]float64 `json:"msf"`
		P     *int               `json:"p"`
		N     inner              `json:"n"`
	}
	n := 7
	zero := diverse{}
	full := diverse{
		I8: -8, I64: -64, U8: 8, U64: 64, Up: 3, F32: 1.5, F64: 2.5,
		B: true, S: "hi",
		Bs:    []byte("ab"),
		Sl:    []int{1},
		MSS:   map[string]string{"k": "v"},
		MSI:   map[string]int{"k": 1},
		MSI64: map[string]int64{"k": 1},
		MSF:   map[string]float64{"k": 1.5},
		P:     &n,
		N:     inner{X: 9},
	}

	for _, v := range []diverse{full, zero} {
		rv := reflect.ValueOf(v)
		rt := rv.Type()
		fields := GetStructEncoder(rt)
		if len(fields) != rt.NumField() {
			t.Fatalf("GetStructEncoder returned %d fields, want %d", len(fields), rt.NumField())
		}

		for _, f := range fields {
			fv := rv.Field(f.Index)
			want, werr := json.Marshal(fv.Interface())
			if werr != nil {
				t.Fatalf("stdlib marshal field %s err: %v", f.Name, werr)
			}

			e := GetEncoder()
			var err error
			if f.EncodeFn != nil {
				err = f.EncodeFn(e, fv)
			} else {
				// Production contract for types without a specialized encoder.
				err = e.EncodeValue(fv.Interface())
			}
			got := string(e.Bytes())
			PutEncoder(e)

			if err != nil {
				t.Errorf("field %s (%s): %v", f.Name, f.Type, err)
				continue
			}
			if got != string(want) {
				t.Errorf("field %s (%s) mismatch:\n got %s\nwant %s", f.Name, f.Type, got, want)
			}
		}
	}
}
