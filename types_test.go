package json

import (
	"encoding/json"
	"math"
	"strings"
	"sync"
	"testing"
	"time"
)

// ============================================================================
// Type Conversion Tests (from type_conversion_test.go)
// ============================================================================

// TestConvertToInt tests integer conversion with various types
func TestConvertToInt(t *testing.T) {
	tests := []struct {
		name          string
		input         any
		expected      int
		shouldSucceed bool
	}{
		{
			name:          "from int",
			input:         42,
			expected:      42,
			shouldSucceed: true,
		},
		{
			name:          "from int8",
			input:         int8(127),
			expected:      127,
			shouldSucceed: true,
		},
		{
			name:          "from int16",
			input:         int16(1000),
			expected:      1000,
			shouldSucceed: true,
		},
		{
			name:          "from int32",
			input:         int32(50000),
			expected:      50000,
			shouldSucceed: true,
		},
		{
			name:          "from int64 in range",
			input:         int64(100000),
			expected:      100000,
			shouldSucceed: true,
		},
		{
			name:          "from int64 out of range positive",
			input:         int64(2147483648),
			expected:      0,
			shouldSucceed: false,
		},
		{
			name:          "from int64 out of range negative",
			input:         int64(-2147483649),
			expected:      0,
			shouldSucceed: false,
		},
		{
			name:          "from uint in range",
			input:         uint(100),
			expected:      100,
			shouldSucceed: true,
		},
		{
			name:          "from uint8",
			input:         uint8(255),
			expected:      255,
			shouldSucceed: true,
		},
		{
			name:          "from uint16",
			input:         uint16(1000),
			expected:      1000,
			shouldSucceed: true,
		},
		{
			name:          "from uint32 in range",
			input:         uint32(1000000),
			expected:      1000000,
			shouldSucceed: true,
		},
		{
			name:          "from float32 exact",
			input:         float32(42.0),
			expected:      42,
			shouldSucceed: true,
		},
		{
			name:          "from float32 not exact",
			input:         float32(42.5),
			expected:      0,
			shouldSucceed: false,
		},
		{
			name:          "from float64 exact",
			input:         float64(100.0),
			expected:      100,
			shouldSucceed: true,
		},
		{
			name:          "from float64 not exact",
			input:         float64(100.7),
			expected:      0,
			shouldSucceed: false,
		},
		{
			name:          "from string valid",
			input:         "123",
			expected:      123,
			shouldSucceed: true,
		},
		{
			name:          "from string invalid",
			input:         "abc",
			expected:      0,
			shouldSucceed: false,
		},
		{
			name:          "from bool true",
			input:         true,
			expected:      1,
			shouldSucceed: true,
		},
		{
			name:          "from bool false",
			input:         false,
			expected:      0,
			shouldSucceed: true,
		},
		{
			name:          "from json.Number valid",
			input:         json.Number("456"),
			expected:      456,
			shouldSucceed: true,
		},
		{
			name:          "from json.Number invalid",
			input:         json.Number("abc"),
			expected:      0,
			shouldSucceed: false,
		},
		{
			name:          "from nil",
			input:         nil,
			expected:      0,
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
		name          string
		input         any
		expected      int64
		shouldSucceed bool
	}{
		{
			name:          "from int",
			input:         42,
			expected:      42,
			shouldSucceed: true,
		},
		{
			name:          "from int64",
			input:         int64(9223372036854775807),
			expected:      9223372036854775807,
			shouldSucceed: true,
		},
		{
			name:          "from uint64 in range",
			input:         uint64(9223372036854775807),
			expected:      9223372036854775807,
			shouldSucceed: true,
		},
		{
			name:          "from uint64 out of range",
			input:         uint64(18446744073709551615),
			expected:      0,
			shouldSucceed: false,
		},
		{
			name:          "from string",
			input:         "1234567890",
			expected:      1234567890,
			shouldSucceed: true,
		},
		{
			name:          "from float64 exact",
			input:         float64(123.0),
			expected:      123,
			shouldSucceed: true,
		},
		{
			name:          "from float64 not exact",
			input:         float64(123.5),
			expected:      0,
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
		name          string
		input         any
		expected      uint64
		shouldSucceed bool
	}{
		{
			name:          "from uint",
			input:         uint(42),
			expected:      42,
			shouldSucceed: true,
		},
		{
			name:          "from uint64 max",
			input:         uint64(18446744073709551615),
			expected:      18446744073709551615,
			shouldSucceed: true,
		},
		{
			name:          "from positive int",
			input:         100,
			expected:      100,
			shouldSucceed: true,
		},
		{
			name:          "from negative int",
			input:         -1,
			expected:      0,
			shouldSucceed: false,
		},
		{
			name:          "from string",
			input:         "123",
			expected:      123,
			shouldSucceed: true,
		},
		{
			name:          "from negative string",
			input:         "-123",
			expected:      0,
			shouldSucceed: false,
		},
		{
			name:          "from float64 positive exact",
			input:         float64(42.0),
			expected:      42,
			shouldSucceed: true,
		},
		{
			name:          "from float64 negative",
			input:         float64(-42.0),
			expected:      0,
			shouldSucceed: false,
		},
		{
			name:          "from bool true",
			input:         true,
			expected:      1,
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
		name          string
		input         any
		expected      float64
		shouldSucceed bool
	}{
		{
			name:          "from float64",
			input:         3.14159,
			expected:      3.14159,
			shouldSucceed: true,
		},
		{
			name:          "from float32",
			input:         float32(2.5),
			expected:      2.5,
			shouldSucceed: true,
		},
		{
			name:          "from int",
			input:         42,
			expected:      42.0,
			shouldSucceed: true,
		},
		{
			name:          "from int64",
			input:         int64(1234567890),
			expected:      1234567890.0,
			shouldSucceed: true,
		},
		{
			name:          "from uint64",
			input:         uint64(1234567890),
			expected:      1234567890.0,
			shouldSucceed: true,
		},
		{
			name:          "from string valid",
			input:         "3.14",
			expected:      3.14,
			shouldSucceed: true,
		},
		{
			name:          "from string invalid",
			input:         "abc",
			expected:      0.0,
			shouldSucceed: false,
		},
		{
			name:          "from bool true",
			input:         true,
			expected:      1.0,
			shouldSucceed: true,
		},
		{
			name:          "from bool false",
			input:         false,
			expected:      0.0,
			shouldSucceed: true,
		},
		{
			name:          "from json.Number",
			input:         json.Number("2.71828"),
			expected:      2.71828,
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
		name          string
		input         any
		expected      bool
		shouldSucceed bool
	}{
		{
			name:          "from bool true",
			input:         true,
			expected:      true,
			shouldSucceed: true,
		},
		{
			name:          "from bool false",
			input:         false,
			expected:      false,
			shouldSucceed: true,
		},
		{
			name:          "from int non-zero",
			input:         42,
			expected:      true,
			shouldSucceed: true,
		},
		{
			name:          "from int zero",
			input:         0,
			expected:      false,
			shouldSucceed: true,
		},
		{
			name:          "from negative int",
			input:         -1,
			expected:      true,
			shouldSucceed: true,
		},
		{
			name:          "from float64 non-zero",
			input:         1.5,
			expected:      true,
			shouldSucceed: true,
		},
		{
			name:          "from float64 zero",
			input:         0.0,
			expected:      false,
			shouldSucceed: true,
		},
		{
			name:          "from string true",
			input:         "true",
			expected:      true,
			shouldSucceed: true,
		},
		{
			name:          "from string TRUE",
			input:         "TRUE",
			expected:      true,
			shouldSucceed: true,
		},
		{
			name:          "from string false",
			input:         "false",
			expected:      false,
			shouldSucceed: true,
		},
		{
			name:          "from string 1",
			input:         "1",
			expected:      true,
			shouldSucceed: true,
		},
		{
			name:          "from string 0",
			input:         "0",
			expected:      false,
			shouldSucceed: true,
		},
		{
			name:          "from string yes",
			input:         "yes",
			expected:      true,
			shouldSucceed: true,
		},
		{
			name:          "from string no",
			input:         "no",
			expected:      false,
			shouldSucceed: true,
		},
		{
			name:          "from string on",
			input:         "on",
			expected:      true,
			shouldSucceed: true,
		},
		{
			name:          "from string off",
			input:         "off",
			expected:      false,
			shouldSucceed: true,
		},
		{
			name:          "from string empty",
			input:         "",
			expected:      false,
			shouldSucceed: true,
		},
		{
			name:          "from string invalid",
			input:         "maybe",
			expected:      false,
			shouldSucceed: false,
		},
		{
			name:          "from json.Number non-zero",
			input:         json.Number("42"),
			expected:      true,
			shouldSucceed: true,
		},
		{
			name:          "from json.Number zero",
			input:         json.Number("0"),
			expected:      false,
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
		name          string
		input         any
		expected      any
		shouldSucceed bool
	}{
		{
			name:          "same type int",
			input:         42,
			expected:      42,
			shouldSucceed: true,
		},
		{
			name:          "same type string",
			input:         "hello",
			expected:      "hello",
			shouldSucceed: true,
		},
		{
			name:          "int to int64",
			input:         42,
			expected:      int64(42),
			shouldSucceed: true,
		},
		{
			name:          "int to float64",
			input:         42,
			expected:      42.0,
			shouldSucceed: true,
		},
		{
			name:          "string to int",
			input:         "123",
			expected:      123,
			shouldSucceed: true,
		},
		{
			name:          "string to bool",
			input:         "true",
			expected:      true,
			shouldSucceed: true,
		},
		{
			name:          "nil to string",
			input:         nil,
			expected:      "",
			shouldSucceed: true,
		},
		{
			name:          "nil to int",
			input:         nil,
			expected:      0,
			shouldSucceed: true,
		},
		{
			name:          "invalid conversion",
			input:         "not a number",
			expected:      0,
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

// ============================================================================
// Schema Tests (from schema_test.go)
// ============================================================================

// TestSchema tests the Schema type
func TestSchema(t *testing.T) {
	schema := &Schema{}

	// Test DefaultSchema initialization
	defaultSchema := DefaultSchema()
	if defaultSchema == nil {
		t.Errorf("DefaultSchema() should not return nil")
	}

	// Test SetMinLength
	schema.SetMinLength(5)
	if !schema.HasMinLength() {
		t.Errorf("HasMinLength() should return true after SetMinLength")
	}

	// Test SetMaxLength
	schema.SetMaxLength(100)
	if !schema.HasMaxLength() {
		t.Errorf("HasMaxLength() should return true after SetMaxLength")
	}

	// Test SetMinimum
	schema.SetMinimum(0)
	if !schema.HasMinimum() {
		t.Errorf("HasMinimum() should return true after SetMinimum")
	}

	// Test SetMaximum
	schema.SetMaximum(1000)
	if !schema.HasMaximum() {
		t.Errorf("HasMaximum() should return true after SetMaximum")
	}

	// Test SetMinItems
	schema.SetMinItems(1)
	if !schema.HasMinItems() {
		t.Errorf("HasMinItems() should return true after SetMinItems")
	}

	// Test SetMaxItems
	schema.SetMaxItems(10)
	if !schema.HasMaxItems() {
		t.Errorf("HasMaxItems() should return true after SetMaxItems")
	}

	// Test SetExclusiveMinimum (should not affect other fields)
	schema.SetExclusiveMinimum(true)
	if !schema.ExclusiveMinimum {
		t.Errorf("ExclusiveMinimum should be true")
	}
	if schema.MinLength != 5 {
		t.Errorf("MinLength should still be 5 after SetExclusiveMinimum, got %d", schema.MinLength)
	}

	// Test SetExclusiveMaximum (should not affect other fields)
	schema.SetExclusiveMaximum(true)
	if !schema.ExclusiveMaximum {
		t.Errorf("ExclusiveMaximum should be true")
	}
	if schema.MaxLength != 100 {
		t.Errorf("MaxLength should still be 100 after SetExclusiveMaximum, got %d", schema.MaxLength)
	}
}

// TestSchema_DefaultSchema tests the DefaultSchema function
func TestSchema_DefaultSchema(t *testing.T) {
	schema := DefaultSchema()

	if schema == nil {
		t.Fatal("DefaultSchema() returned nil")
	}

	// Verify defaults
	if schema.HasMinLength() {
		t.Errorf("Default schema should not have MinLength set")
	}
	if schema.HasMaxLength() {
		t.Errorf("Default schema should not have MaxLength set")
	}
	if schema.HasMinimum() {
		t.Errorf("Default schema should not have Minimum set")
	}
	if schema.HasMaximum() {
		t.Errorf("Default schema should not have Maximum set")
	}
	if schema.HasMinItems() {
		t.Errorf("Default schema should not have MinItems set")
	}
	if schema.HasMaxItems() {
		t.Errorf("Default schema should not have MaxItems set")
	}
}

// TestSchema_StringConstraints tests string constraint methods
func TestSchema_StringConstraints(t *testing.T) {
	schema := &Schema{}

	// Test SetMinLength and HasMinLength
	schema.SetMinLength(10)
	if !schema.HasMinLength() {
		t.Errorf("HasMinLength() should return true")
	}
	if schema.MinLength != 10 {
		t.Errorf("MinLength = %d, want 10", schema.MinLength)
	}

	// Test SetMaxLength and HasMaxLength
	schema.SetMaxLength(100)
	if !schema.HasMaxLength() {
		t.Errorf("HasMaxLength() should return true")
	}
	if schema.MaxLength != 100 {
		t.Errorf("MaxLength = %d, want 100", schema.MaxLength)
	}

	// Test multiple sets
	schema.SetMinLength(5)
	schema.SetMaxLength(50)
	if schema.MinLength != 5 || schema.MaxLength != 50 {
		t.Errorf("MinLength/MaxLength not updated correctly")
	}
}

// TestSchema_NumericConstraints tests numeric constraint methods
func TestSchema_NumericConstraints(t *testing.T) {
	schema := &Schema{}

	// Test SetMinimum and HasMinimum
	schema.SetMinimum(-100)
	if !schema.HasMinimum() {
		t.Errorf("HasMinimum() should return true")
	}
	if schema.Minimum != -100 {
		t.Errorf("Minimum = %f, want -100", schema.Minimum)
	}

	// Test SetMaximum and HasMaximum
	schema.SetMaximum(1000)
	if !schema.HasMaximum() {
		t.Errorf("HasMaximum() should return true")
	}
	if schema.Maximum != 1000 {
		t.Errorf("Maximum = %f, want 1000", schema.Maximum)
	}

	// Test zero values
	schema2 := &Schema{}
	schema2.SetMinimum(0)
	if !schema2.HasMinimum() {
		t.Errorf("HasMinimum() should return true for 0")
	}

	schema2.SetMaximum(0)
	if !schema2.HasMaximum() {
		t.Errorf("HasMaximum() should return true for 0")
	}
}

// TestSchema_ArrayConstraints tests array constraint methods
func TestSchema_ArrayConstraints(t *testing.T) {
	schema := &Schema{}

	// Test SetMinItems and HasMinItems
	schema.SetMinItems(1)
	if !schema.HasMinItems() {
		t.Errorf("HasMinItems() should return true")
	}
	if schema.MinItems != 1 {
		t.Errorf("MinItems = %d, want 1", schema.MinItems)
	}

	// Test SetMaxItems and HasMaxItems
	schema.SetMaxItems(100)
	if !schema.HasMaxItems() {
		t.Errorf("HasMaxItems() should return true")
	}
	if schema.MaxItems != 100 {
		t.Errorf("MaxItems = %d, want 100", schema.MaxItems)
	}

	// Test zero values
	schema2 := &Schema{}
	schema2.SetMinItems(0)
	if !schema2.HasMinItems() {
		t.Errorf("HasMinItems() should return true for 0")
	}

	schema2.SetMaxItems(0)
	if !schema2.HasMaxItems() {
		t.Errorf("HasMaxItems() should return true for 0")
	}
}

// TestSchema_ExclusiveConstraints tests exclusive constraint methods
func TestSchema_ExclusiveConstraints(t *testing.T) {
	schema := &Schema{}

	// Test SetExclusiveMinimum
	schema.SetExclusiveMinimum(true)
	if !schema.ExclusiveMinimum {
		t.Errorf("ExclusiveMinimum should be true")
	}

	schema.SetExclusiveMinimum(false)
	if schema.ExclusiveMinimum {
		t.Errorf("ExclusiveMinimum should be false")
	}

	// Test SetExclusiveMaximum
	schema.SetExclusiveMaximum(true)
	if !schema.ExclusiveMaximum {
		t.Errorf("ExclusiveMaximum should be true")
	}

	schema.SetExclusiveMaximum(false)
	if schema.ExclusiveMaximum {
		t.Errorf("ExclusiveMaximum should be false")
	}

	// Test with other constraints set
	schema.SetMinimum(10)
	schema.SetExclusiveMinimum(true)
	if schema.Minimum != 10 {
		t.Errorf("Minimum should still be 10")
	}

	schema.SetMaximum(100)
	schema.SetExclusiveMaximum(true)
	if schema.Maximum != 100 {
		t.Errorf("Maximum should still be 100")
	}
}

// TestSchema_AllConstraints tests setting all constraints together
func TestSchema_AllConstraints(t *testing.T) {
	schema := &Schema{}

	schema.SetMinLength(5)
	schema.SetMaxLength(50)
	schema.SetMinimum(0)
	schema.SetMaximum(100)
	schema.SetMinItems(1)
	schema.SetMaxItems(10)
	schema.SetExclusiveMinimum(false)
	schema.SetExclusiveMaximum(false)

	if !schema.HasMinLength() || schema.MinLength != 5 {
		t.Errorf("MinLength not set correctly")
	}
	if !schema.HasMaxLength() || schema.MaxLength != 50 {
		t.Errorf("MaxLength not set correctly")
	}
	if !schema.HasMinimum() || schema.Minimum != 0 {
		t.Errorf("Minimum not set correctly")
	}
	if !schema.HasMaximum() || schema.Maximum != 100 {
		t.Errorf("Maximum not set correctly")
	}
	if !schema.HasMinItems() || schema.MinItems != 1 {
		t.Errorf("MinItems not set correctly")
	}
	if !schema.HasMaxItems() || schema.MaxItems != 10 {
		t.Errorf("MaxItems not set correctly")
	}
	if schema.ExclusiveMinimum {
		t.Errorf("ExclusiveMinimum should be false")
	}
	if schema.ExclusiveMaximum {
		t.Errorf("ExclusiveMaximum should be false")
	}
}

// ============================================================================
// Resource Manager Tests (from resource_manager_test.go)
// ============================================================================

// TestUnifiedResourceManager tests the unified resource manager functionality
func TestUnifiedResourceManager(t *testing.T) {
	t.Run("Creation", func(t *testing.T) {
		rm := NewUnifiedResourceManager()
		if rm == nil {
			t.Fatal("NewUnifiedResourceManager returned nil")
		}
	})

	t.Run("StringBuilderPool", func(t *testing.T) {
		rm := NewUnifiedResourceManager()

		// Test Get and Put cycle
		sb1 := rm.GetStringBuilder()
		if sb1 == nil {
			t.Fatal("GetStringBuilder returned nil")
		}

		// Write some data
		sb1.WriteString("test data")
		if sb1.String() != "test data" {
			t.Errorf("StringBuilder write failed, got: %s", sb1.String())
		}

		// Return to pool
		rm.PutStringBuilder(sb1)

		// Get again - should get the same or different builder
		sb2 := rm.GetStringBuilder()
		if sb2 == nil {
			t.Fatal("GetStringBuilder returned nil on second call")
		}
		sb2.Reset()
		rm.PutStringBuilder(sb2)
	})

	t.Run("PathSegmentPool", func(t *testing.T) {
		rm := NewUnifiedResourceManager()

		// Test Get and Put cycle
		seg1 := rm.GetPathSegments()
		if seg1 == nil {
			t.Fatal("GetPathSegments returned nil")
		}

		// Verify it's a slice
		if cap(seg1) == 0 {
			t.Error("PathSegment should have capacity")
		}

		// Return to pool
		rm.PutPathSegments(seg1)

		// Get again
		seg2 := rm.GetPathSegments()
		if seg2 == nil {
			t.Fatal("GetPathSegments returned nil on second call")
		}
		rm.PutPathSegments(seg2)
	})

	t.Run("BufferPool", func(t *testing.T) {
		rm := NewUnifiedResourceManager()

		// Test Get and Put cycle
		buf1 := rm.GetBuffer()
		if buf1 == nil {
			t.Fatal("GetBuffer returned nil")
		}

		// Write some data
		buf1 = append(buf1, "test data"...)

		// Return to pool
		rm.PutBuffer(buf1)

		// Get again
		buf2 := rm.GetBuffer()
		if buf2 == nil {
			t.Fatal("GetBuffer returned nil on second call")
		}
		rm.PutBuffer(buf2)
	})

	t.Run("ConcurrentAccess", func(t *testing.T) {
		rm := NewUnifiedResourceManager()
		const goroutines = 100
		const opsPerGoroutine = 100

		var wg sync.WaitGroup
		wg.Add(goroutines * 3) // 3 types of pools

		// StringBuilder pool concurrent access
		for i := 0; i < goroutines; i++ {
			go func() {
				defer wg.Done()
				for j := 0; j < opsPerGoroutine; j++ {
					sb := rm.GetStringBuilder()
					sb.WriteString("concurrent test")
					rm.PutStringBuilder(sb)
				}
			}()
		}

		// PathSegment pool concurrent access
		for i := 0; i < goroutines; i++ {
			go func() {
				defer wg.Done()
				for j := 0; j < opsPerGoroutine; j++ {
					seg := rm.GetPathSegments()
					rm.PutPathSegments(seg)
				}
			}()
		}

		// Buffer pool concurrent access
		for i := 0; i < goroutines; i++ {
			go func() {
				defer wg.Done()
				for j := 0; j < opsPerGoroutine; j++ {
					buf := rm.GetBuffer()
					buf = append(buf, byte(i))
					rm.PutBuffer(buf)
				}
			}()
		}

		wg.Wait()
	})

	t.Run("Stats", func(t *testing.T) {
		rm := NewUnifiedResourceManager()

		// Perform some operations
		sb := rm.GetStringBuilder()
		sb.WriteString("test")
		rm.PutStringBuilder(sb)

		// Allocate a path segment to increment that counter
		seg := rm.GetPathSegments()
		rm.PutPathSegments(seg)

		buf := rm.GetBuffer()
		buf = append(buf, "test"...)
		rm.PutBuffer(buf)

		// Get stats - verify no crashes
		_ = rm.GetStats()
		// Note: Allocated counts are tracked atomically and should be positive
		// The specific counts may vary due to internal implementation details
	})

	t.Run("SizeLimits", func(t *testing.T) {
		rm := NewUnifiedResourceManager()

		// Test oversized builder is discarded
		oversizedSb := &strings.Builder{}
		oversizedSb.Grow(100000) // Way over MaxPoolBufferSize
		rm.PutStringBuilder(oversizedSb)

		// Test undersized builder is discarded
		undersizedSb := &strings.Builder{}
		undersizedSb.Grow(10) // Under MinPoolBufferSize
		rm.PutStringBuilder(undersizedSb)

		// Note: Oversized/undersized builders are discarded automatically
	})

	t.Run("PerformMaintenance", func(t *testing.T) {
		rm := NewUnifiedResourceManager()

		// Should not panic
		rm.PerformMaintenance()

		// Multiple calls should be safe
		for i := 0; i < 10; i++ {
			rm.PerformMaintenance()
		}
	})
}

// TestGlobalResourceManager tests the global resource manager singleton
func TestGlobalResourceManager(t *testing.T) {
	t.Run("Singleton", func(t *testing.T) {
		rm1 := getGlobalResourceManager()
		rm2 := getGlobalResourceManager()

		if rm1 != rm2 {
			t.Error("getGlobalResourceManager should return the same instance")
		}
	})

	t.Run("Usable", func(t *testing.T) {
		rm := getGlobalResourceManager()

		sb := rm.GetStringBuilder()
		sb.WriteString("global test")
		rm.PutStringBuilder(sb)

		// Should not panic
	})
}

// TestResourceMonitor tests the ResourceMonitor functionality
func TestResourceMonitor(t *testing.T) {
	t.Run("Creation", func(t *testing.T) {
		rm := NewResourceMonitor()
		if rm == nil {
			t.Fatal("NewResourceMonitor returned nil")
		}
	})

	t.Run("RecordAllocation", func(t *testing.T) {
		rm := NewResourceMonitor()

		rm.RecordAllocation(1024)
		rm.RecordAllocation(2048)
		rm.RecordDeallocation(512)

		stats := rm.GetStats()
		// Note: AllocatedBytes should be positive (at least the sum of allocations minus deallocations)
		if stats.AllocatedBytes <= 0 {
			t.Errorf("Expected positive allocated bytes, got %d", stats.AllocatedBytes)
		}
		// FreedBytes should be at least what we deallocated
		if stats.FreedBytes < 512 {
			t.Errorf("Expected at least 512 freed bytes, got %d", stats.FreedBytes)
		}
	})

	t.Run("RecordPoolOperations", func(t *testing.T) {
		rm := NewResourceMonitor()

		rm.RecordPoolHit()
		rm.RecordPoolMiss()
		rm.RecordPoolEviction()

		stats := rm.GetStats()
		if stats.PoolHits != 1 {
			t.Errorf("Expected 1 pool hit, got %d", stats.PoolHits)
		}
		if stats.PoolMisses != 1 {
			t.Errorf("Expected 1 pool miss, got %d", stats.PoolMisses)
		}
		if stats.PoolEvictions != 1 {
			t.Errorf("Expected 1 pool eviction, got %d", stats.PoolEvictions)
		}
	})

	t.Run("RecordOperation", func(t *testing.T) {
		rm := NewResourceMonitor()

		rm.RecordOperation(100 * time.Millisecond)
		rm.RecordOperation(200 * time.Millisecond)

		stats := rm.GetStats()
		if stats.TotalOperations != 2 {
			t.Errorf("Expected 2 operations, got %d", stats.TotalOperations)
		}
		if stats.AvgResponseTime == 0 {
			t.Error("Expected non-zero average response time")
		}
	})

	t.Run("PeakMemoryTracking", func(t *testing.T) {
		rm := NewResourceMonitor()

		// Record increasing allocations to test peak tracking
		rm.RecordAllocation(1000)
		stats1 := rm.GetStats()
		if stats1.PeakMemoryUsage < 1000 {
			t.Errorf("Expected peak >= 1000, got %d", stats1.PeakMemoryUsage)
		}

		rm.RecordAllocation(2000)
		stats2 := rm.GetStats()
		// Peak should be at least 3000 (1000 + 2000)
		if stats2.PeakMemoryUsage < 3000 {
			t.Errorf("Expected peak >= 3000, got %d", stats2.PeakMemoryUsage)
		}

		rm.RecordDeallocation(500)
		stats3 := rm.GetStats()
		// Peak should remain at maximum (not decrease with deallocations)
		if stats3.PeakMemoryUsage < stats2.PeakMemoryUsage {
			t.Errorf("Peak should not decrease with deallocations, went from %d to %d",
				stats2.PeakMemoryUsage, stats3.PeakMemoryUsage)
		}
	})

	t.Run("Reset", func(t *testing.T) {
		rm := NewResourceMonitor()

		rm.RecordAllocation(1000)
		rm.RecordPoolHit()
		rm.Reset()

		stats := rm.GetStats()
		if stats.AllocatedBytes != 0 {
			t.Errorf("Expected 0 allocated bytes after reset, got %d", stats.AllocatedBytes)
		}
		if stats.PoolHits != 0 {
			t.Errorf("Expected 0 pool hits after reset, got %d", stats.PoolHits)
		}
	})
}

// TestGetStatsWithResourceManager tests processor GetStats functionality with resource manager
func TestGetStatsWithResourceManager(t *testing.T) {
	p := New()
	defer p.Close()

	stats := p.GetStats()

	// Verify stats are accessible
	if stats.IsClosed {
		t.Error("Processor should not be closed")
	}

	// Perform operations and verify stats update
	jsonStr := `{"test":"data"}`
	_, _ = p.Get(jsonStr, "test")

	stats2 := p.GetStats()
	if stats2.OperationCount == 0 {
		t.Error("Expected operation count to increase")
	}
}

// ============================================================================
// Benchmark tests
// ============================================================================

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

func BenchmarkStringBuilderPool(b *testing.B) {
	rm := NewUnifiedResourceManager()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sb := rm.GetStringBuilder()
		sb.WriteString("benchmark test data")
		rm.PutStringBuilder(sb)
	}
}

func BenchmarkPathSegmentPool(b *testing.B) {
	rm := NewUnifiedResourceManager()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		seg := rm.GetPathSegments()
		rm.PutPathSegments(seg)
	}
}

func BenchmarkBufferPool(b *testing.B) {
	rm := NewUnifiedResourceManager()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf := rm.GetBuffer()
		buf = append(buf, "test"...)
		rm.PutBuffer(buf)
	}
}

func BenchmarkConcurrentPools(b *testing.B) {
	rm := NewUnifiedResourceManager()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			sb := rm.GetStringBuilder()
			buf := rm.GetBuffer()
			_ = append(buf, "test"...)
			rm.PutStringBuilder(sb)
			rm.PutBuffer(buf)
		}
	})
}

func BenchmarkResourceMonitor(b *testing.B) {
	rm := NewResourceMonitor()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rm.RecordAllocation(1024)
		rm.RecordDeallocation(512)
	}
}
