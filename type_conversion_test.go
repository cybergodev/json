package json

import (
	"encoding/json"
	"math"
	"testing"
)

// TestConvertToInt tests integer conversion with various types
func TestConvertToInt(t *testing.T) {
	tests := []struct {
		name        string
		input       any
		expected    int
		shouldSucceed bool
	}{
		{
			name:        "from int",
			input:       42,
			expected:    42,
			shouldSucceed: true,
		},
		{
			name:        "from int8",
			input:       int8(127),
			expected:    127,
			shouldSucceed: true,
		},
		{
			name:        "from int16",
			input:       int16(1000),
			expected:    1000,
			shouldSucceed: true,
		},
		{
			name:        "from int32",
			input:       int32(50000),
			expected:    50000,
			shouldSucceed: true,
		},
		{
			name:        "from int64 in range",
			input:       int64(100000),
			expected:    100000,
			shouldSucceed: true,
		},
		{
			name:        "from int64 out of range positive",
			input:       int64(2147483648),
			expected:    0,
			shouldSucceed: false,
		},
		{
			name:        "from int64 out of range negative",
			input:       int64(-2147483649),
			expected:    0,
			shouldSucceed: false,
		},
		{
			name:        "from uint in range",
			input:       uint(100),
			expected:    100,
			shouldSucceed: true,
		},
		{
			name:        "from uint8",
			input:       uint8(255),
			expected:    255,
			shouldSucceed: true,
		},
		{
			name:        "from uint16",
			input:       uint16(1000),
			expected:    1000,
			shouldSucceed: true,
		},
		{
			name:        "from uint32 in range",
			input:       uint32(1000000),
			expected:    1000000,
			shouldSucceed: true,
		},
		{
			name:        "from float32 exact",
			input:       float32(42.0),
			expected:    42,
			shouldSucceed: true,
		},
		{
			name:        "from float32 not exact",
			input:       float32(42.5),
			expected:    0,
			shouldSucceed: false,
		},
		{
			name:        "from float64 exact",
			input:       float64(100.0),
			expected:    100,
			shouldSucceed: true,
		},
		{
			name:        "from float64 not exact",
			input:       float64(100.7),
			expected:    0,
			shouldSucceed: false,
		},
		{
			name:        "from string valid",
			input:       "123",
			expected:    123,
			shouldSucceed: true,
		},
		{
			name:        "from string invalid",
			input:       "abc",
			expected:    0,
			shouldSucceed: false,
		},
		{
			name:        "from bool true",
			input:       true,
			expected:    1,
			shouldSucceed: true,
		},
		{
			name:        "from bool false",
			input:       false,
			expected:    0,
			shouldSucceed: true,
		},
		{
			name:        "from json.Number valid",
			input:       json.Number("456"),
			expected:    456,
			shouldSucceed: true,
		},
		{
			name:        "from json.Number invalid",
			input:       json.Number("abc"),
			expected:    0,
			shouldSucceed: false,
		},
		{
			name:        "from nil",
			input:       nil,
			expected:    0,
			shouldSucceed: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, ok := ConvertToInt(tt.input)
			if ok != tt.shouldSucceed {
				t.Errorf("ConvertToInt(%v) success = %v; want %v", tt.input, ok, tt.shouldSucceed)
			}
			if ok && result != tt.expected {
				t.Errorf("ConvertToInt(%v) = %d; want %d", tt.input, result, tt.expected)
			}
		})
	}
}

// TestConvertToInt64 tests int64 conversion
func TestConvertToInt64(t *testing.T) {
	tests := []struct {
		name        string
		input       any
		expected    int64
		shouldSucceed bool
	}{
		{
			name:        "from int",
			input:       42,
			expected:    42,
			shouldSucceed: true,
		},
		{
			name:        "from int64",
			input:       int64(9223372036854775807),
			expected:    9223372036854775807,
			shouldSucceed: true,
		},
		{
			name:        "from uint64 in range",
			input:       uint64(9223372036854775807),
			expected:    9223372036854775807,
			shouldSucceed: true,
		},
		{
			name:        "from uint64 out of range",
			input:       uint64(18446744073709551615),
			expected:    0,
			shouldSucceed: false,
		},
		{
			name:        "from string",
			input:       "1234567890",
			expected:    1234567890,
			shouldSucceed: true,
		},
		{
			name:        "from float64 exact",
			input:       float64(123.0),
			expected:    123,
			shouldSucceed: true,
		},
		{
			name:        "from float64 not exact",
			input:       float64(123.5),
			expected:    0,
			shouldSucceed: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, ok := ConvertToInt64(tt.input)
			if ok != tt.shouldSucceed {
				t.Errorf("ConvertToInt64(%v) success = %v; want %v", tt.input, ok, tt.shouldSucceed)
			}
			if ok && result != tt.expected {
				t.Errorf("ConvertToInt64(%v) = %d; want %d", tt.input, result, tt.expected)
			}
		})
	}
}

// TestConvertToUint64 tests uint64 conversion
func TestConvertToUint64(t *testing.T) {
	tests := []struct {
		name        string
		input       any
		expected    uint64
		shouldSucceed bool
	}{
		{
			name:        "from uint",
			input:       uint(42),
			expected:    42,
			shouldSucceed: true,
		},
		{
			name:        "from uint64 max",
			input:       uint64(18446744073709551615),
			expected:    18446744073709551615,
			shouldSucceed: true,
		},
		{
			name:        "from positive int",
			input:       100,
			expected:    100,
			shouldSucceed: true,
		},
		{
			name:        "from negative int",
			input:       -1,
			expected:    0,
			shouldSucceed: false,
		},
		{
			name:        "from string",
			input:       "123",
			expected:    123,
			shouldSucceed: true,
		},
		{
			name:        "from negative string",
			input:       "-123",
			expected:    0,
			shouldSucceed: false,
		},
		{
			name:        "from float64 positive exact",
			input:       float64(42.0),
			expected:    42,
			shouldSucceed: true,
		},
		{
			name:        "from float64 negative",
			input:       float64(-42.0),
			expected:    0,
			shouldSucceed: false,
		},
		{
			name:        "from bool true",
			input:       true,
			expected:    1,
			shouldSucceed: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, ok := ConvertToUint64(tt.input)
			if ok != tt.shouldSucceed {
				t.Errorf("ConvertToUint64(%v) success = %v; want %v", tt.input, ok, tt.shouldSucceed)
			}
			if ok && result != tt.expected {
				t.Errorf("ConvertToUint64(%v) = %d; want %d", tt.input, result, tt.expected)
			}
		})
	}
}

// TestConvertToFloat64 tests float64 conversion
func TestConvertToFloat64(t *testing.T) {
	tests := []struct {
		name        string
		input       any
		expected    float64
		shouldSucceed bool
	}{
		{
			name:        "from float64",
			input:       3.14159,
			expected:    3.14159,
			shouldSucceed: true,
		},
		{
			name:        "from float32",
			input:       float32(2.5),
			expected:    2.5,
			shouldSucceed: true,
		},
		{
			name:        "from int",
			input:       42,
			expected:    42.0,
			shouldSucceed: true,
		},
		{
			name:        "from int64",
			input:       int64(1234567890),
			expected:    1234567890.0,
			shouldSucceed: true,
		},
		{
			name:        "from uint64",
			input:       uint64(1234567890),
			expected:    1234567890.0,
			shouldSucceed: true,
		},
		{
			name:        "from string valid",
			input:       "3.14",
			expected:    3.14,
			shouldSucceed: true,
		},
		{
			name:        "from string invalid",
			input:       "abc",
			expected:    0.0,
			shouldSucceed: false,
		},
		{
			name:        "from bool true",
			input:       true,
			expected:    1.0,
			shouldSucceed: true,
		},
		{
			name:        "from bool false",
			input:       false,
			expected:    0.0,
			shouldSucceed: true,
		},
		{
			name:        "from json.Number",
			input:       json.Number("2.71828"),
			expected:    2.71828,
			shouldSucceed: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, ok := ConvertToFloat64(tt.input)
			if ok != tt.shouldSucceed {
				t.Errorf("ConvertToFloat64(%v) success = %v; want %v", tt.input, ok, tt.shouldSucceed)
			}
			if ok && result != tt.expected {
				t.Errorf("ConvertToFloat64(%v) = %f; want %f", tt.input, result, tt.expected)
			}
		})
	}
}

// TestConvertToBool tests boolean conversion
func TestConvertToBool(t *testing.T) {
	tests := []struct {
		name        string
		input       any
		expected    bool
		shouldSucceed bool
	}{
		{
			name:        "from bool true",
			input:       true,
			expected:    true,
			shouldSucceed: true,
		},
		{
			name:        "from bool false",
			input:       false,
			expected:    false,
			shouldSucceed: true,
		},
		{
			name:        "from int non-zero",
			input:       42,
			expected:    true,
			shouldSucceed: true,
		},
		{
			name:        "from int zero",
			input:       0,
			expected:    false,
			shouldSucceed: true,
		},
		{
			name:        "from negative int",
			input:       -1,
			expected:    true,
			shouldSucceed: true,
		},
		{
			name:        "from float64 non-zero",
			input:       1.5,
			expected:    true,
			shouldSucceed: true,
		},
		{
			name:        "from float64 zero",
			input:       0.0,
			expected:    false,
			shouldSucceed: true,
		},
		{
			name:        "from string true",
			input:       "true",
			expected:    true,
			shouldSucceed: true,
		},
		{
			name:        "from string TRUE",
			input:       "TRUE",
			expected:    true,
			shouldSucceed: true,
		},
		{
			name:        "from string false",
			input:       "false",
			expected:    false,
			shouldSucceed: true,
		},
		{
			name:        "from string 1",
			input:       "1",
			expected:    true,
			shouldSucceed: true,
		},
		{
			name:        "from string 0",
			input:       "0",
			expected:    false,
			shouldSucceed: true,
		},
		{
			name:        "from string yes",
			input:       "yes",
			expected:    true,
			shouldSucceed: true,
		},
		{
			name:        "from string no",
			input:       "no",
			expected:    false,
			shouldSucceed: true,
		},
		{
			name:        "from string on",
			input:       "on",
			expected:    true,
			shouldSucceed: true,
		},
		{
			name:        "from string off",
			input:       "off",
			expected:    false,
			shouldSucceed: true,
		},
		{
			name:        "from string empty",
			input:       "",
			expected:    false,
			shouldSucceed: true,
		},
		{
			name:        "from string invalid",
			input:       "maybe",
			expected:    false,
			shouldSucceed: false,
		},
		{
			name:        "from json.Number non-zero",
			input:       json.Number("42"),
			expected:    true,
			shouldSucceed: true,
		},
		{
			name:        "from json.Number zero",
			input:       json.Number("0"),
			expected:    false,
			shouldSucceed: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, ok := ConvertToBool(tt.input)
			if ok != tt.shouldSucceed {
				t.Errorf("ConvertToBool(%v) success = %v; want %v", tt.input, ok, tt.shouldSucceed)
			}
			if ok && result != tt.expected {
				t.Errorf("ConvertToBool(%v) = %v; want %v", tt.input, result, tt.expected)
			}
		})
	}
}

// TestUnifiedTypeConversion tests generic type conversion
func TestUnifiedTypeConversion(t *testing.T) {
	tests := []struct {
		name        string
		input       any
		expected    any
		shouldSucceed bool
	}{
		{
			name:        "same type int",
			input:       42,
			expected:    42,
			shouldSucceed: true,
		},
		{
			name:        "same type string",
			input:       "hello",
			expected:    "hello",
			shouldSucceed: true,
		},
		{
			name:        "int to int64",
			input:       42,
			expected:    int64(42),
			shouldSucceed: true,
		},
		{
			name:        "int to float64",
			input:       42,
			expected:    42.0,
			shouldSucceed: true,
		},
		{
			name:        "string to int",
			input:       "123",
			expected:    123,
			shouldSucceed: true,
		},
		{
			name:        "string to bool",
			input:       "true",
			expected:    true,
			shouldSucceed: true,
		},
		{
			name:        "nil to string",
			input:       nil,
			expected:    "",
			shouldSucceed: true,
		},
		{
			name:        "nil to int",
			input:       nil,
			expected:    0,
			shouldSucceed: true,
		},
		{
			name:        "invalid conversion",
			input:       "not a number",
			expected:    0,
			shouldSucceed: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			switch tt.expected.(type) {
			case int:
				result, ok := UnifiedTypeConversion[int](tt.input)
				if ok != tt.shouldSucceed {
					t.Errorf("UnifiedTypeConversion[int](%v) success = %v; want %v", tt.input, ok, tt.shouldSucceed)
				}
				if ok && result != tt.expected.(int) {
					t.Errorf("UnifiedTypeConversion[int](%v) = %d; want %d", tt.input, result, tt.expected)
				}
			case int64:
				result, ok := UnifiedTypeConversion[int64](tt.input)
				if ok != tt.shouldSucceed {
					t.Errorf("UnifiedTypeConversion[int64](%v) success = %v; want %v", tt.input, ok, tt.shouldSucceed)
				}
				if ok && result != tt.expected.(int64) {
					t.Errorf("UnifiedTypeConversion[int64](%v) = %d; want %d", tt.input, result, tt.expected)
				}
			case float64:
				result, ok := UnifiedTypeConversion[float64](tt.input)
				if ok != tt.shouldSucceed {
					t.Errorf("UnifiedTypeConversion[float64](%v) success = %v; want %v", tt.input, ok, tt.shouldSucceed)
				}
				if ok && result != tt.expected.(float64) {
					t.Errorf("UnifiedTypeConversion[float64](%v) = %f; want %f", tt.input, result, tt.expected)
				}
			case bool:
				result, ok := UnifiedTypeConversion[bool](tt.input)
				if ok != tt.shouldSucceed {
					t.Errorf("UnifiedTypeConversion[bool](%v) success = %v; want %v", tt.input, ok, tt.shouldSucceed)
				}
				if ok && result != tt.expected.(bool) {
					t.Errorf("UnifiedTypeConversion[bool](%v) = %v; want %v", tt.input, result, tt.expected)
				}
			case string:
				result, ok := UnifiedTypeConversion[string](tt.input)
				if ok != tt.shouldSucceed {
					t.Errorf("UnifiedTypeConversion[string](%v) success = %v; want %v", tt.input, ok, tt.shouldSucceed)
				}
				if ok && result != tt.expected.(string) {
					t.Errorf("UnifiedTypeConversion[string](%v) = %s; want %s", tt.input, result, tt.expected)
				}
			}
		})
	}
}

// TestSafeConvertToInt64 tests safe int64 conversion with error handling
func TestSafeConvertToInt64(t *testing.T) {
	tests := []struct {
		name        string
		input       any
		expected    int64
		expectError bool
	}{
		{
			name:        "valid conversion",
			input:       42,
			expected:    42,
			expectError: false,
		},
		{
			name:        "invalid conversion",
			input:       "not a number",
			expected:    0,
			expectError: true,
		},
		{
			name:        "float conversion",
			input:       123.0,
			expected:    123,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := SafeConvertToInt64(tt.input)
			if tt.expectError && err == nil {
				t.Errorf("Expected error for input %v, but got none", tt.input)
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error for input %v: %v", tt.input, err)
			}
			if !tt.expectError && result != tt.expected {
				t.Errorf("SafeConvertToInt64(%v) = %d; want %d", tt.input, result, tt.expected)
			}
		})
	}
}

// TestSafeConvertToUint64 tests safe uint64 conversion with error handling
func TestSafeConvertToUint64(t *testing.T) {
	tests := []struct {
		name        string
		input       any
		expected    uint64
		expectError bool
	}{
		{
			name:        "valid conversion",
			input:       uint(42),
			expected:    42,
			expectError: false,
		},
		{
			name:        "invalid conversion",
			input:       "not a number",
			expected:    0,
			expectError: true,
		},
		{
			name:        "negative number",
			input:       -1,
			expected:    0,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := SafeConvertToUint64(tt.input)
			if tt.expectError && err == nil {
				t.Errorf("Expected error for input %v, but got none", tt.input)
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error for input %v: %v", tt.input, err)
			}
			if !tt.expectError && result != tt.expected {
				t.Errorf("SafeConvertToUint64(%v) = %d; want %d", tt.input, result, tt.expected)
			}
		})
	}
}

// TestFormatNumber tests number formatting
func TestFormatNumber(t *testing.T) {
	tests := []struct {
		name     string
		input    any
		expected string
	}{
		{
			name:     "int",
			input:    42,
			expected: "42",
		},
		{
			name:     "int64",
			input:    int64(1234567890),
			expected: "1234567890",
		},
		{
			name:     "uint64",
			input:    uint64(18446744073709551615),
			expected: "18446744073709551615",
		},
		{
			name:     "float64",
			input:    3.14159,
			expected: "3.14159",
		},
		{
			name:     "json.Number",
			input:    json.Number("2.71828"),
			expected: "2.71828",
		},
		{
			name:     "other type",
			input:    "custom",
			expected: "custom",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatNumber(tt.input)
			if result != tt.expected {
				t.Errorf("FormatNumber(%v) = %s; want %s", tt.input, result, tt.expected)
			}
		})
	}
}

// TestConvertToString tests string conversion
func TestConvertToString(t *testing.T) {
	tests := []struct {
		name     string
		input    any
		expected string
	}{
		{
			name:     "from string",
			input:    "hello",
			expected: "hello",
		},
		{
			name:     "from []byte",
			input:    []byte("world"),
			expected: "world",
		},
		{
			name:     "from json.Number",
			input:    json.Number("123"),
			expected: "123",
		},
		{
			name:     "from int",
			input:    42,
			expected: "42",
		},
		{
			name:     "from bool",
			input:    true,
			expected: "true",
		},
		{
			name:     "from float64",
			input:    3.14,
			expected: "3.14",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ConvertToString(tt.input)
			if result != tt.expected {
				t.Errorf("ConvertToString(%v) = %s; want %s", tt.input, result, tt.expected)
			}
		})
	}
}

// TestTypeConversionBoundaryConditions tests edge cases in type conversion
func TestTypeConversionBoundaryConditions(t *testing.T) {
	t.Run("max int64", func(t *testing.T) {
		result, ok := ConvertToInt64(int64(math.MaxInt64))
		if !ok || result != math.MaxInt64 {
			t.Errorf("Failed to convert max int64")
		}
	})

	t.Run("min int64", func(t *testing.T) {
		result, ok := ConvertToInt64(int64(math.MinInt64))
		if !ok || result != math.MinInt64 {
			t.Errorf("Failed to convert min int64")
		}
	})

	t.Run("max uint64", func(t *testing.T) {
		result, ok := ConvertToUint64(uint64(math.MaxUint64))
		if !ok || result != math.MaxUint64 {
			t.Errorf("Failed to convert max uint64")
		}
	})

	t.Run("max float64", func(t *testing.T) {
		result, ok := ConvertToFloat64(math.MaxFloat64)
		if !ok || result != math.MaxFloat64 {
			t.Errorf("Failed to convert max float64")
		}
	})

	t.Run("infinity float64", func(t *testing.T) {
		result, ok := ConvertToFloat64(math.Inf(1))
		if !ok || result != math.Inf(1) {
			t.Errorf("Failed to convert infinity")
		}
	})

	t.Run("NaN float64", func(t *testing.T) {
		result, ok := ConvertToFloat64(math.NaN())
		if !ok || !math.IsNaN(result) {
			t.Errorf("Failed to convert NaN")
		}
	})
}

// Benchmark tests

func BenchmarkConvertToInt(b *testing.B) {
	input := 42
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = ConvertToInt(input)
	}
}

func BenchmarkConvertToInt64(b *testing.B) {
	input := int64(42)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = ConvertToInt64(input)
	}
}

func BenchmarkConvertToFloat64(b *testing.B) {
	input := 3.14
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = ConvertToFloat64(input)
	}
}

func BenchmarkUnifiedTypeConversion(b *testing.B) {
	input := 42
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = UnifiedTypeConversion[int64](input)
	}
}
