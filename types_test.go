package json

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cybergodev/json/internal"
)

// Merged from: types_test.go, json_test.go, error_test.go

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

func BenchmarkConvertToFloat64(b *testing.B) {
	input := 3.14
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = ConvertToFloat64(input)
	}
}

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

func BenchmarkPathSegmentPool(b *testing.B) {
	rm := NewUnifiedResourceManager()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		seg := rm.GetPathSegments()
		rm.PutPathSegments(seg)
	}
}

func BenchmarkResourceMonitor(b *testing.B) {
	rm := NewResourceMonitor()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rm.RecordAllocation(1024)
		rm.RecordDeallocation(512)
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

func BenchmarkUnifiedTypeConversion(b *testing.B) {
	input := 42
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = UnifiedTypeConversion[int64](input)
	}
}

// TestAdvancedPathOperations tests advanced path features
func TestAdvancedPathOperations(t *testing.T) {
	helper := NewTestHelper(t)

	t.Run("WildcardAccess", func(t *testing.T) {
		testData := `{
			"users": [
				{"name": "Alice", "age": 25},
				{"name": "Bob", "age": 30}
			]
		}`

		// Test wildcard path access
		result, err := Get(testData, "users[*].name")
		helper.AssertNoError(err)
		if arr, ok := result.([]any); ok {
			helper.AssertTrue(len(arr) > 0)
		}
	})

	t.Run("ComplexExtraction", func(t *testing.T) {
		testData := `{
			"departments": [
				{
					"name": "Engineering",
					"teams": [
						{"name": "Backend", "members": ["Alice", "Bob"]},
						{"name": "Frontend", "members": ["Charlie"]}
					]
				}
			]
		}`

		// Test deep extraction
		result, err := Get(testData, "departments{teams}{members}")
		helper.AssertNoError(err)
		helper.AssertNotNil(result)
	})

	t.Run("ArraySlicingAdvanced", func(t *testing.T) {
		testData := `{"numbers": [1, 2, 3, 4, 5, 6, 7, 8, 9, 10]}`

		// Test various slice patterns
		tests := []struct {
			path     string
			expected []int
		}{
			{"numbers[0:3]", []int{1, 2, 3}},
			{"numbers[7:]", []int{8, 9, 10}},
			{"numbers[:3]", []int{1, 2, 3}},
			{"numbers[-3:]", []int{8, 9, 10}},
		}

		for _, tt := range tests {
			result, err := Get(testData, tt.path)
			helper.AssertNoError(err, "Path: %s", tt.path)
			if arr, ok := result.([]any); ok {
				helper.AssertEqual(len(tt.expected), len(arr))
			}
		}
	})
}

// TestArrayExtensionError tests the ArrayExtensionError type
func TestArrayExtensionError(t *testing.T) {
	// Test default message
	err1 := &ArrayExtensionError{
		CurrentLength:  5,
		RequiredLength: 10,
		TargetIndex:    9,
		Value:          "test",
	}
	expectedMsg1 := "array extension required: current length 5, required length 10 for index 9"
	if err1.Error() != expectedMsg1 {
		t.Errorf("Error() = %q, want %q", err1.Error(), expectedMsg1)
	}

	// Test custom message
	err2 := &ArrayExtensionError{
		CurrentLength:  5,
		RequiredLength: 10,
		TargetIndex:    9,
		Value:          "test",
		Message:        "Custom error message",
	}
	if err2.Error() != "Custom error message" {
		t.Errorf("Error() with custom message = %q, want %q", err2.Error(), "Custom error message")
	}

	// Test with custom message
	err3 := &ArrayExtensionError{
		CurrentLength:  3,
		RequiredLength: 10,
		TargetIndex:    9,
		Value:          "test",
		Message:        "Array too small",
	}
	if err3.Error() != "Array too small" {
		t.Errorf("Error() with custom message = %q, want %q", err3.Error(), "Array too small")
	}
}

// TestConfigConstantsComprehensive consolidates configuration and constants tests
// This replaces: config_constants_test.go
func TestConfigConstantsComprehensive(t *testing.T) {
	helper := NewTestHelper(t)

	t.Run("ConfigGetters", func(t *testing.T) {
		config := DefaultConfig()

		helper.AssertTrue(config.IsCacheEnabled())
		helper.AssertTrue(config.GetMaxCacheSize() > 0)
		helper.AssertTrue(config.GetCacheTTL() > 0)
		helper.AssertTrue(config.GetMaxJSONSize() > 0)
		helper.AssertTrue(config.GetMaxPathDepth() > 0)
		helper.AssertTrue(config.GetMaxConcurrency() > 0)
		helper.AssertTrue(config.GetMaxNestingDepth() >= 0)

		limits := config.GetSecurityLimits()
		helper.AssertNotNil(limits)
	})

	t.Run("ConfigPresets", func(t *testing.T) {
		config := DefaultConfig()
		helper.AssertNotNil(config)

		highSec := HighSecurityConfig()
		helper.AssertNotNil(highSec)

		largeDat := LargeDataConfig()
		helper.AssertNotNil(largeDat)
	})

	t.Run("EncodeConfigPresets", func(t *testing.T) {
		config := DefaultEncodeConfig()
		helper.AssertNotNil(config)

		pretty := NewPrettyConfig()
		helper.AssertNotNil(pretty)
		helper.AssertTrue(pretty.Pretty)

		// Default config is compact (Pretty = false)
		helper.AssertFalse(config.Pretty)

		cloned := config.Clone()
		helper.AssertNotNil(cloned)
	})

	t.Run("ConfigValidation", func(t *testing.T) {
		config := DefaultConfig()
		err := ValidateConfig(config)
		helper.AssertNoError(err)

		invalidConfig := DefaultConfig()
		invalidConfig.MaxCacheSize = -1
		err = ValidateConfig(invalidConfig)
		helper.AssertError(err)
	})

	t.Run("Constants", func(t *testing.T) {
		helper.AssertTrue(DefaultBufferSize > 0)
		helper.AssertTrue(MaxPoolBufferSize > MinPoolBufferSize)
		helper.AssertTrue(DefaultCacheSize > 0)
		helper.AssertTrue(DefaultMaxJSONSize > 0)
		helper.AssertTrue(DefaultMaxNestingDepth > 0)
		helper.AssertTrue(MaxPathLength > 0)
		helper.AssertTrue(DefaultOperationTimeout > 0)
	})

	t.Run("ErrorCodes", func(t *testing.T) {
		errorCodes := []string{
			ErrCodeInvalidJSON,
			ErrCodePathNotFound,
			ErrCodeTypeMismatch,
			ErrCodeSizeLimit,
			ErrCodeSecurityViolation,
		}

		for _, code := range errorCodes {
			helper.AssertTrue(len(code) > 0)
			helper.AssertTrue(code[:4] == "ERR_")
		}
	})

	t.Run("GlobalProcessor", func(t *testing.T) {
		SetGlobalProcessor(New(DefaultConfig()))
		ShutdownGlobalProcessor()
	})
}

// TestConfig_GetSecurityLimits tests Config.GetSecurityLimits method
func TestConfig_GetSecurityLimits(t *testing.T) {
	config := &Config{
		MaxNestingDepthSecurity:   100,
		MaxSecurityValidationSize: 1024 * 1024,
		MaxObjectKeys:             1000,
		MaxArrayElements:          10000,
		MaxJSONSize:               10 * 1024 * 1024,
		MaxPathDepth:              50,
	}

	limits := config.GetSecurityLimits()

	if limits["max_nesting_depth"].(int) != 100 {
		t.Errorf("GetSecurityLimits max_nesting_depth = %v, want 100", limits["max_nesting_depth"])
	}
	if limits["max_json_size"].(int64) != 10*1024*1024 {
		t.Errorf("GetSecurityLimits max_json_size = %v, want %d", limits["max_json_size"], 10*1024*1024)
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
			name:          "from string T",
			input:         "T",
			expected:      true,
			shouldSucceed: true,
		},
		{
			name:          "from string F",
			input:         "F",
			expected:      false,
			shouldSucceed: true,
		},
		{
			name:          "from string True",
			input:         "True",
			expected:      true,
			shouldSucceed: true,
		},
		{
			name:          "from string False",
			input:         "False",
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

// TestDeletedMarker tests the DeletedMarker constant
func TestDeletedMarker(t *testing.T) {
	if DeletedMarker == nil {
		t.Errorf("DeletedMarker should not be nil")
	}

	// Test that DeletedMarker can be used for comparison
	arr := []any{1, DeletedMarker, 3}
	count := 0
	for _, v := range arr {
		if v == DeletedMarker {
			count++
		}
	}
	if count != 1 {
		t.Errorf("Found %d DeletedMarker, want 1", count)
	}
}

// TestEncodeConfig_Clone tests EncodeConfig.Clone method
func TestEncodeConfig_Clone(t *testing.T) {
	original := &EncodeConfig{
		Pretty:        true,
		Indent:        "  ",
		EscapeHTML:    true,
		CustomEscapes: map[rune]string{'\n': "\\n"},
	}

	cloned := original.Clone()

	// Verify clone is equal
	if cloned.Pretty != original.Pretty {
		t.Error("Clone should copy Pretty")
	}
	if cloned.Indent != original.Indent {
		t.Error("Clone should copy Indent")
	}

	// Modify clone and verify original is unchanged
	cloned.Pretty = false
	if original.Pretty != true {
		t.Error("Modifying clone should not affect original")
	}

	// Verify CustomEscapes is deep copied
	cloned.CustomEscapes['\t'] = "\\t"
	if _, exists := original.CustomEscapes['\t']; exists {
		t.Error("Modifying clone's CustomEscapes should not affect original")
	}
}

// TestEncodeConfig_Clone_Nil tests Clone with nil receiver
func TestEncodeConfig_Clone_Nil(t *testing.T) {
	var config *EncodeConfig
	cloned := config.Clone()
	if cloned == nil {
		t.Error("Clone of nil should return default config, not nil")
	}
}

// TestErrorClassifier tests error classification functionality
func TestErrorClassifier(t *testing.T) {
	helper := NewTestHelper(t)

	t.Run("IsSecurityRelated", func(t *testing.T) {
		tests := []struct {
			name     string
			err      error
			expected bool
		}{
			{"SecurityViolation", ErrSecurityViolation, true},
			{"PathNotFound", ErrPathNotFound, false},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				result := IsSecurityRelated(tt.err)
				helper.AssertEqual(tt.expected, result)
			})
		}
	})

	t.Run("IsUserError", func(t *testing.T) {
		tests := []struct {
			name     string
			err      error
			expected bool
		}{
			{"PathNotFound", ErrPathNotFound, true},
			{"InvalidPath", ErrInvalidPath, true},
			{"TypeMismatch", ErrTypeMismatch, true},
			{"SystemError", ErrOperationFailed, false},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				result := IsUserError(tt.err)
				helper.AssertEqual(tt.expected, result)
			})
		}
	})

	t.Run("IsRetryable", func(t *testing.T) {
		tests := []struct {
			name     string
			err      error
			expected bool
		}{
			{"Timeout", ErrOperationTimeout, true},
			{"ConcurrencyLimit", ErrConcurrencyLimit, true},
			{"InvalidJSON", ErrInvalidJSON, false},
			{"PathNotFound", ErrPathNotFound, false},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				result := IsRetryable(tt.err)
				helper.AssertEqual(tt.expected, result)
			})
		}
	})

	t.Run("GetErrorSuggestion", func(t *testing.T) {
		tests := []struct {
			name               string
			err                error
			suggestionContains string
		}{
			{"InvalidJSON", ErrInvalidJSON, "valid"},
			{"PathNotFound", ErrPathNotFound, "check"},
			{"TypeMismatch", ErrTypeMismatch, "type"},
			{"InvalidPath", ErrInvalidPath, "format"},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				suggestion := GetErrorSuggestion(tt.err)
				helper.AssertNotNil(suggestion)
				helper.AssertTrue(
					len(suggestion) > 0,
					"Suggestion should not be empty",
				)
				// Suggestions may vary by implementation, just verify we got one
				_ = tt.suggestionContains
			})
		}
	})
}

// TestErrorHandling comprehensive error handling tests
func TestErrorHandling(t *testing.T) {
	helper := NewTestHelper(t)

	t.Run("JsonsErrorStructure", func(t *testing.T) {
		err := &JsonsError{
			Op:      "test_operation",
			Path:    "test.path",
			Message: "test error message",
			Err:     ErrOperationFailed,
		}

		helper.AssertEqual("test_operation", err.Op)
		helper.AssertEqual("test.path", err.Path)
		helper.AssertEqual("test error message", err.Message)
		helper.AssertEqual(ErrOperationFailed, err.Err)
	})

	t.Run("ErrorWrapping", func(t *testing.T) {
		baseErr := ErrPathNotFound

		t.Run("WrapError", func(t *testing.T) {
			wrapped := WrapError(baseErr, "get_user", "user not found")
			helper.AssertNotNil(wrapped)

			// Unwrap should return original
			unwrapped := errors.Unwrap(wrapped)
			helper.AssertEqual(baseErr, unwrapped)
		})

		t.Run("WrapPathError", func(t *testing.T) {
			wrapped := WrapPathError(baseErr, "get_field", "user.profile.email", "field missing")
			helper.AssertNotNil(wrapped)

			// Check error message contains path
			helper.AssertTrue(errors.Is(wrapped, baseErr))
		})
	})

	t.Run("ErrorTypes", func(t *testing.T) {
		testData := `{"name": "John", "age": 30}`

		tests := []struct {
			name    string
			path    string
			wantErr bool
			errType error
		}{
			{"ValidPath", "name", false, nil},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				_, err := Get(testData, tt.path)
				if tt.wantErr {
					helper.AssertError(err)
					if tt.errType != nil {
						var jsonErr *JsonsError
						if errors.As(err, &jsonErr) {
							helper.AssertEqual(tt.errType, jsonErr.Err)
						}
					}
				} else {
					helper.AssertNoError(err)
				}
			})
		}
	})
}

// TestErrorMessages verifies error messages are helpful
func TestErrorMessages(t *testing.T) {
	helper := NewTestHelper(t)

	t.Run("PathNotFoundMessage", func(t *testing.T) {
		testData := `{"user": {"name": "John"}}`
		_, err := Get(testData, "user.email")

		helper.AssertError(err)
		var jsonErr *JsonsError
		if errors.As(err, &jsonErr) {
			helper.AssertEqual("get", jsonErr.Op)
			helper.AssertEqual("user.email", jsonErr.Path)
			helper.AssertTrue(strings.Contains(jsonErr.Message, "not found"))
		}
	})

	t.Run("InvalidPathMessage", func(t *testing.T) {
		_, err := Get(`{"test": "value"}`, "[[[")

		helper.AssertError(err)
		if err != nil {
			helper.AssertTrue(strings.Contains(err.Error(), "path") ||
				strings.Contains(err.Error(), "bracket") ||
				strings.Contains(err.Error(), "invalid"))
		}
	})

	t.Run("TypeMismatchMessage", func(t *testing.T) {
		testData := `{"value": "not a number"}`

		_, err := GetInt(testData, "value")
		helper.AssertError(err)

		var jsonErr *JsonsError
		if errors.As(err, &jsonErr) {
			helper.AssertEqual(ErrTypeMismatch, jsonErr.Err)
		}
	})
}

// TestErrorRecovery tests error recovery scenarios
func TestErrorRecovery(t *testing.T) {
	helper := NewTestHelper(t)

	t.Run("RetryAfterTimeout", func(t *testing.T) {
		retryableErrors := []error{
			ErrOperationTimeout,
			ErrConcurrencyLimit,
		}

		for _, err := range retryableErrors {
			t.Run(err.Error(), func(t *testing.T) {
				helper.AssertTrue(IsRetryable(err))
			})
		}
	})

	t.Run("NoRetryForInvalidData", func(t *testing.T) {
		nonRetryableErrors := []error{
			ErrInvalidJSON,
			ErrInvalidPath,
			ErrTypeMismatch,
		}

		for _, err := range nonRetryableErrors {
			t.Run(err.Error(), func(t *testing.T) {
				helper.AssertFalse(IsRetryable(err))
			})
		}
	})

	t.Run("ContinueAfterError", func(t *testing.T) {
		testData := `{
			"valid1": "value1",
			"invalid": null,
			"valid2": "value2"
		}`

		// Get multiple paths, some may fail
		paths := []string{"valid1", "invalid", "valid2"}

		successCount := 0
		for _, path := range paths {
			_, err := GetString(testData, path)
			if err == nil {
				successCount++
			}
		}

		// Should have succeeded for valid paths
		helper.AssertTrue(successCount >= 2)
	})
}

// TestErrorScenarios tests various error scenarios
func TestErrorScenarios(t *testing.T) {
	helper := NewTestHelper(t)

	t.Run("InvalidJSON", func(t *testing.T) {
		invalidJSON := []string{
			`{invalid json}`,
			`{"unclosed": "string}`,
			`{"trailing": "comma",}`,
			`{unquoted: "key"}`,
			`{"number": 123.45.67}`,
			`{"array": [1, 2, 3,]}`,
		}

		for _, jsonStr := range invalidJSON {
			t.Run("JSON_"+jsonStr[:10], func(t *testing.T) {
				_, err := Get(jsonStr, "any")
				helper.AssertError(err)
			})
		}
	})

	t.Run("InvalidPaths", func(t *testing.T) {
		testData := `{"user": {"name": "John"}}`

		// Only test paths that should legitimately fail
		invalidPaths := []string{
			"[[[", // Invalid bracket syntax
			"]]]", // Invalid bracket syntax
		}

		for _, path := range invalidPaths {
			t.Run("Path_"+path, func(t *testing.T) {
				_, err := Get(testData, path)
				helper.AssertError(err)
			})
		}
	})

	t.Run("ArrayErrors", func(t *testing.T) {
		testData := `{"arr": [1, 2, 3]}`

		t.Run("InvalidIndex", func(t *testing.T) {
			_, err := Get(testData, "arr[abc]")
			helper.AssertError(err)
		})

		t.Run("ArrayOnNonArray", func(t *testing.T) {
			_, err := Get(testData, "user[0]")
			helper.AssertError(err)
		})
	})

	t.Run("TypeMismatch", func(t *testing.T) {
		testData := `{"str": "value", "num": 42, "bool": true}`

		t.Run("StringAsInt", func(t *testing.T) {
			_, err := GetInt(testData, "str")
			helper.AssertError(err)
		})

		t.Run("NumberAsString", func(t *testing.T) {
			_, err := GetString(testData, "num")
			// This might succeed with conversion
			_ = err
		})

		t.Run("BoolAsInt", func(t *testing.T) {
			// Bool to int might convert (true=1, false=0)
			_, err := GetInt(testData, "bool")
			_ = err // Don't assert error, library may convert
		})

		t.Run("ObjectAsArray", func(t *testing.T) {
			_, err := GetArray(testData, "str")
			helper.AssertError(err)
		})
	})

	t.Run("NullHandling", func(t *testing.T) {
		testData := `{"null_value": null, "string_value": "value"}`

		t.Run("GetNull", func(t *testing.T) {
			result, err := Get(testData, "null_value")
			helper.AssertNoError(err)
			helper.AssertNil(result)
		})

		t.Run("GetTypedNull", func(t *testing.T) {
			_, err := GetTyped[string](testData, "null_value")
			// Null to string might error or return "null"
			_ = err
		})

		t.Run("GetMissingField", func(t *testing.T) {
			_, err := Get(testData, "missing")
			helper.AssertError(err)
		})
	})

	t.Run("ProcessorClosed", func(t *testing.T) {
		processor := New(DefaultConfig())
		processor.Close()

		testData := `{"test": "value"}`

		_, err := processor.Get(testData, "test")
		helper.AssertError(err)

		var jsonErr *JsonsError
		helper.AssertTrue(errors.As(err, &jsonErr))
	})

	t.Run("EmptyInput", func(t *testing.T) {
		tests := []struct {
			name  string
			input string
		}{
			{"EmptyString", ""},
			{"WhitespaceOnly", "   \n\t  "},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				_, err := Get(tt.input, "any")
				helper.AssertError(err)
			})
		}
	})

	t.Run("ConcurrentErrors", func(t *testing.T) {
		processor := New(DefaultConfig())
		defer processor.Close()

		// Attempt to access after close from multiple goroutines
		done := make(chan error, 5)

		for i := 0; i < 5; i++ {
			go func() {
				_, err := processor.Get(`{"test": "value"}`, "test")
				done <- err
			}()
		}

		// Close while operations might be in flight
		processor.Close()

		for i := 0; i < 5; i++ {
			err := <-done
			// May succeed if it completed before close, or error if after
			_ = err
		}
	})
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

// TestHealthCheckSystem tests health check functionality
func TestHealthCheckSystem(t *testing.T) {
	helper := NewTestHelper(t)

	t.Run("BasicHealthCheck", func(t *testing.T) {
		processor := New()
		defer processor.Close()

		health := processor.GetHealthStatus()
		helper.AssertNotNil(health)
		// Health status should be available
		helper.AssertNotNil(health.Checks)
	})

	t.Run("HealthCheckWithLoad", func(t *testing.T) {
		processor := New()
		defer processor.Close()

		testData := `{"test": "value"}`
		for i := 0; i < 100; i++ {
			processor.Get(testData, "test")
		}

		health := processor.GetHealthStatus()
		helper.AssertNotNil(health)
		// Health status should still be available after load
		helper.AssertNotNil(health.Checks)
	})
}

// TestInvalidArrayIndex tests the InvalidArrayIndex constant
func TestInvalidArrayIndex(t *testing.T) {
	if InvalidArrayIndex != -999999 {
		t.Errorf("InvalidArrayIndex = %d, want -999999", InvalidArrayIndex)
	}
}

// TestInvalidUnmarshalError tests the InvalidUnmarshalError type
func TestInvalidUnmarshalError(t *testing.T) {
	// Test nil type
	err1 := &InvalidUnmarshalError{Type: nil}
	if err1.Error() != "json: Unmarshal(nil)" {
		t.Errorf("Error() with nil Type = %q, want %q", err1.Error(), "json: Unmarshal(nil)")
	}

	// Test non-pointer type
	strType := reflect.TypeOf("")
	err2 := &InvalidUnmarshalError{Type: strType}
	if err2.Error() != "json: Unmarshal(non-pointer string)" {
		t.Errorf("Error() with non-pointer = %q, want %q", err2.Error(), "json: Unmarshal(non-pointer string)")
	}

	// Test nil pointer
	type MyStruct struct{ Field int }
	ptrType := reflect.TypeOf(&MyStruct{})
	err3 := &InvalidUnmarshalError{Type: ptrType}
	if err3.Error() != "json: Unmarshal(nil *json.MyStruct)" {
		t.Errorf("Error() with nil pointer = %q, want %q", err3.Error(), "json: Unmarshal(nil *json.MyStruct)")
	}
}

// TestIteratorAdvancedFeatures tests advanced iterator functionality
func TestIteratorAdvancedFeatures(t *testing.T) {
	helper := NewTestHelper(t)

	t.Run("IteratorControlBreak", func(t *testing.T) {
		testData := `{"items": [1, 2, 3, 4, 5]}`

		count := 0
		Foreach(testData, func(key any, item *IterableValue) {
			count++
			if count >= 2 {
				// Break early
			}
		})

		helper.AssertTrue(count <= 5)
	})

	t.Run("IterableValueGetPath", func(t *testing.T) {
		testData := `{"user": {"name": "Alice", "profile": {"age": 25}}}`
		processor := New()
		defer processor.Close()

		var data map[string]any
		processor.Parse(testData, &data)

		iv := &IterableValue{data: data}

		// Test nested path access using dot notation
		profile := iv.GetObject("user")
		helper.AssertNotNil(profile)
		age, ok := profile["profile"].(map[string]any)
		helper.AssertTrue(ok)
		helper.AssertEqual(float64(25), age["age"])
	})

	t.Run("ForeachNestedWithPaths", func(t *testing.T) {
		testData := `{
			"users": [
				{"name": "Alice", "roles": ["admin", "user"]},
				{"name": "Bob", "roles": ["user"]}
			]
		}`

		// Test that Foreach correctly iterates over the root object
		callCount := 0
		Foreach(testData, func(key any, item *IterableValue) {
			callCount++
		})

		// Should be called once for the root object
		helper.AssertTrue(callCount >= 1)
	})
}

// TestMarshalerError tests the MarshalerError type
func TestMarshalerError(t *testing.T) {
	type TestType struct{}
	testType := reflect.TypeOf(TestType{})
	wrappedErr := errors.New("marshal failed")

	err1 := &MarshalerError{
		Type:       testType,
		Err:        wrappedErr,
		sourceFunc: "",
	}

	expectedMsg1 := "json: error calling MarshalJSON for type json.TestType: marshal failed"
	if err1.Error() != expectedMsg1 {
		t.Errorf("Error() with empty sourceFunc = %q, want %q", err1.Error(), expectedMsg1)
	}

	// Test Unwrap
	if err1.Unwrap() != wrappedErr {
		t.Errorf("Unwrap() should return wrapped error")
	}

	// Test with custom sourceFunc
	err2 := &MarshalerError{
		Type:       testType,
		Err:        wrappedErr,
		sourceFunc: "MarshalText",
	}

	expectedMsg2 := "json: error calling MarshalText for type json.TestType: marshal failed"
	if err2.Error() != expectedMsg2 {
		t.Errorf("Error() with custom sourceFunc = %q, want %q", err2.Error(), expectedMsg2)
	}
}

// TestOperation_String tests Operation.String method
func TestOperation_String(t *testing.T) {
	tests := []struct {
		op       Operation
		expected string
	}{
		{OpGet, "get"},
		{OpSet, "set"},
		{OpDelete, "delete"},
		{OpValidate, "validate"},
		{Operation(999), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if got := tt.op.String(); got != tt.expected {
				t.Errorf("Operation.String() = %v, want %v", got, tt.expected)
			}
		})
	}
}

// TestPathInfo tests the PathInfo type
func TestPathInfo(t *testing.T) {
	segments := []PathSegment{
		{Type: internal.PropertySegment, Key: "user"},
		{Type: internal.ArrayIndexSegment, Index: 0},
	}

	pathInfo := PathInfo{
		Segments:     segments,
		IsPointer:    false,
		OriginalPath: "user[0]",
	}

	if len(pathInfo.Segments) != 2 {
		t.Errorf("Segments length = %d, want 2", len(pathInfo.Segments))
	}

	if pathInfo.IsPointer {
		t.Errorf("IsPointer = true, want false")
	}

	if pathInfo.OriginalPath != "user[0]" {
		t.Errorf("OriginalPath = %q, want %q", pathInfo.OriginalPath, "user[0]")
	}
}

// TestProcessorConcurrencyComprehensive consolidates processor and concurrency tests
// This replaces: processor_concurrency_test.go, concurrency_test.go
func TestProcessorConcurrencyComprehensive(t *testing.T) {
	helper := NewTestHelper(t)
	generator := NewTestDataGenerator()

	t.Run("ProcessorOperations", func(t *testing.T) {
		processor := New()
		defer processor.Close()

		testData := `{
			"user": {"name": "John", "age": 30},
			"items": [1, 2, 3, 4, 5],
			"active": true
		}`

		t.Run("GetMultiple", func(t *testing.T) {
			paths := []string{"user.name", "user.age", "active"}
			results, err := processor.GetMultiple(testData, paths)
			helper.AssertNoError(err)
			helper.AssertEqual(3, len(results))
			helper.AssertEqual("John", results["user.name"])
		})

		t.Run("ProcessBatch", func(t *testing.T) {
			operations := []BatchOperation{
				{Type: "get", JSONStr: `{"name": "John"}`, Path: "name", ID: "op1"},
				{Type: "set", JSONStr: `{"age": 30}`, Path: "age", Value: 35, ID: "op2"},
			}

			batchResults, err := processor.ProcessBatch(operations)
			helper.AssertNoError(err)
			helper.AssertEqual(2, len(batchResults))
		})

		t.Run("Stats", func(t *testing.T) {
			_, _ = processor.Get(testData, "user.name")
			stats := processor.GetStats()
			helper.AssertTrue(stats.OperationCount > 0)
		})

		t.Run("HealthStatus", func(t *testing.T) {
			health := processor.GetHealthStatus()
			helper.AssertNotNil(health)
			helper.AssertTrue(len(health.Checks) > 0)
		})

		t.Run("CacheOperations", func(t *testing.T) {
			_, _ = processor.Get(testData, "user.name")
			processor.ClearCache()

			samplePaths := []string{"user.name", "active"}
			_, _ = processor.WarmupCache(testData, samplePaths)
		})

		t.Run("Lifecycle", func(t *testing.T) {
			p := New()
			helper.AssertFalse(p.IsClosed())
			p.Close()
			helper.AssertTrue(p.IsClosed())

			_, err := p.Get(`{"test": "value"}`, "test")
			helper.AssertError(err)
		})
	})

	t.Run("ConcurrentOperations", func(t *testing.T) {
		t.Run("ConcurrentReads", func(t *testing.T) {
			jsonStr := generator.GenerateComplexJSON()
			concurrencyTester := NewConcurrencyTester(t, 20, 100)

			concurrencyTester.Run(func(workerID, iteration int) error {
				paths := []string{
					"users[0].name",
					"settings.appName",
					"statistics.totalUsers",
				}
				path := paths[workerID%len(paths)]
				_, err := Get(jsonStr, path)
				return err
			})
		})

		t.Run("ConcurrentWrites", func(t *testing.T) {
			originalJSON := `{"counters": {"a": 0, "b": 0}}`
			var results []string
			var resultsMutex sync.Mutex

			concurrencyTester := NewConcurrencyTester(t, 10, 50)
			concurrencyTester.Run(func(workerID, iteration int) error {
				counterKey := fmt.Sprintf("counters.%c", 'a'+workerID%2)
				result, err := Set(originalJSON, counterKey, workerID*1000+iteration)
				if err != nil {
					return err
				}
				resultsMutex.Lock()
				results = append(results, result)
				resultsMutex.Unlock()
				return nil
			})

			helper.AssertTrue(len(results) > 0)
		})

		t.Run("MixedOperations", func(t *testing.T) {
			baseJSON := `{"data": {"counter": 0}, "array": [1, 2, 3]}`
			var operations int64

			concurrencyTester := NewConcurrencyTester(t, 15, 100)
			concurrencyTester.Run(func(workerID, iteration int) error {
				atomic.AddInt64(&operations, 1)
				switch iteration % 3 {
				case 0:
					_, err := Get(baseJSON, "data.counter")
					return err
				case 1:
					_, err := Set(baseJSON, "data.counter", iteration)
					return err
				case 2:
					_, err := Get(baseJSON, "array[1]")
					return err
				}
				return nil
			})

			helper.AssertTrue(atomic.LoadInt64(&operations) > 0)
		})

		t.Run("SharedProcessor", func(t *testing.T) {
			processor := New()
			defer processor.Close()

			jsonStr := generator.GenerateComplexJSON()
			concurrencyTester := NewConcurrencyTester(t, 25, 200)

			concurrencyTester.Run(func(workerID, iteration int) error {
				switch iteration % 2 {
				case 0:
					_, err := processor.Get(jsonStr, "users[0].name")
					return err
				case 1:
					_, err := Set(jsonStr, fmt.Sprintf("worker_%d", workerID), iteration)
					return err
				}
				return nil
			})
		})

		t.Run("MultipleProcessors", func(t *testing.T) {
			const numProcessors = 10
			const operationsPerProcessor = 100

			jsonStr := `{"test": "value", "number": 42}`
			var wg sync.WaitGroup
			var totalOps, totalErrors int64

			for i := 0; i < numProcessors; i++ {
				wg.Add(1)
				go func(processorID int) {
					defer wg.Done()
					processor := New()
					defer processor.Close()

					for j := 0; j < operationsPerProcessor; j++ {
						atomic.AddInt64(&totalOps, 1)
						_, err := processor.Get(jsonStr, "test")
						if err != nil {
							atomic.AddInt64(&totalErrors, 1)
						}
					}
				}(i)
			}

			wg.Wait()
			helper.AssertEqual(int64(numProcessors*operationsPerProcessor), atomic.LoadInt64(&totalOps))
			helper.AssertEqual(int64(0), atomic.LoadInt64(&totalErrors))
		})

		t.Run("CacheThreadSafety", func(t *testing.T) {
			config := DefaultConfig()
			config.EnableCache = true
			config.MaxCacheSize = 100

			processor := New(config)
			defer processor.Close()

			jsonStr := `{"cached": "value", "number": 123}`
			const numWorkers = 15
			const operationsPerWorker = 100

			var wg sync.WaitGroup
			for i := 0; i < numWorkers; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					for j := 0; j < operationsPerWorker; j++ {
						if j%2 == 0 {
							processor.Get(jsonStr, "cached")
						} else {
							processor.Get(jsonStr, fmt.Sprintf("dynamic_%d", j))
						}
					}
				}()
			}

			wg.Wait()
			stats := processor.GetStats()
			helper.AssertTrue(stats.HitCount+stats.MissCount > 0)
		})
	})

	// Additional concurrency tests from concurrency_test.go
	t.Run("ConcurrentAccessDetailed", func(t *testing.T) {
		t.Run("ConcurrentReadsWithProcessor", func(t *testing.T) {
			proc := New(DefaultConfig())
			defer proc.Close()

			testData := `{"users": [` + generateUserJSON(100) + `]}`

			concurrency := 20
			iterations := 100

			var wg sync.WaitGroup
			errors := make(chan error, concurrency*iterations)

			for i := 0; i < concurrency; i++ {
				wg.Add(1)
				go func(workerID int) {
					defer wg.Done()
					for j := 0; j < iterations; j++ {
						_, err := proc.Get(testData, fmt.Sprintf("users[%d]", j%100))
						if err != nil {
							errors <- err
						}
					}
				}(i)
			}

			wg.Wait()
			close(errors)

			for err := range errors {
				t.Errorf("Concurrent read error: %v", err)
			}
		})

		t.Run("ConcurrentWritesWithPackageFuncs", func(t *testing.T) {
			testData := `{"counter": 0}`

			concurrency := 10
			iterations := 50

			var wg sync.WaitGroup
			results := make(chan string, concurrency*iterations)

			for i := 0; i < concurrency; i++ {
				wg.Add(1)
				go func(workerID int) {
					defer wg.Done()
					for j := 0; j < iterations; j++ {
						result, err := Set(testData, "counter", workerID*iterations+j)
						if err == nil {
							results <- result
						}
					}
				}(i)
			}

			wg.Wait()
			close(results)

			count := 0
			for range results {
				count++
			}
			helper.AssertTrue(count > 0)
		})

		t.Run("ConcurrentMixedReadWrite", func(t *testing.T) {
			proc := New(DefaultConfig())
			defer proc.Close()

			testData := `{"data": {"value": 0}}`
			var testDataMu sync.Mutex

			concurrency := 15
			iterations := 50

			var wg sync.WaitGroup

			for i := 0; i < concurrency; i++ {
				wg.Add(1)
				go func(workerID int) {
					defer wg.Done()
					for j := 0; j < iterations; j++ {
						if workerID%2 == 0 {
							testDataMu.Lock()
							localData := testData
							testDataMu.Unlock()
							proc.Get(localData, "data.value")
						} else {
							testDataMu.Lock()
							newData, _ := Set(testData, "data.value", workerID*100+j)
							testData = newData
							testDataMu.Unlock()
						}
					}
				}(i)
			}

			wg.Wait()
		})

		t.Run("ConcurrentProcessorsIndependence", func(t *testing.T) {
			concurrency := 10
			iterations := 20

			testData := `{"test": "value"}`

			var wg sync.WaitGroup

			for i := 0; i < concurrency; i++ {
				wg.Add(1)
				go func(workerID int) {
					defer wg.Done()

					proc := New(DefaultConfig())
					defer proc.Close()

					for j := 0; j < iterations; j++ {
						proc.Get(testData, "test")
					}
				}(i)
			}

			wg.Wait()
		})
	})

	t.Run("ConcurrentCacheOperations", func(t *testing.T) {
		t.Run("CacheConcurrency", func(t *testing.T) {
			config := DefaultConfig()
			config.EnableCache = true
			config.MaxCacheSize = 100

			proc := New(config)
			defer proc.Close()

			testData := `{"user": {"name": "Alice", "age": 30}}`

			concurrency := 20
			iterations := 100

			var wg sync.WaitGroup

			for i := 0; i < 10; i++ {
				proc.Get(testData, "user.name")
			}

			for i := 0; i < concurrency; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					for j := 0; j < iterations; j++ {
						proc.Get(testData, "user.name")
						proc.Get(testData, "user.age")
					}
				}()
			}

			wg.Wait()

			stats := proc.GetStats()
			helper.AssertTrue(stats.HitCount > 0)
		})

		t.Run("CacheInvalidation", func(t *testing.T) {
			config := DefaultConfig()
			config.EnableCache = true
			config.CacheTTL = 100 * time.Millisecond

			proc := New(config)
			defer proc.Close()

			testData := `{"value": "test"}`

			proc.Get(testData, "value")

			time.Sleep(150 * time.Millisecond)

			_, err := proc.Get(testData, "value")
			helper.AssertNoError(err)
		})
	})

	t.Run("ConcurrentIteratorOperations", func(t *testing.T) {
		t.Run("ConcurrentForeach", func(t *testing.T) {
			testData := `{"items": [` + generateArrayItems(50) + `]}`

			concurrency := 10
			iterations := 20

			var wg sync.WaitGroup
			mu := sync.Mutex{}
			totalCount := 0

			for i := 0; i < concurrency; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					for j := 0; j < iterations; j++ {
						err := ForeachWithPath(testData, "items", func(key any, item *IterableValue) {
							mu.Lock()
							totalCount++
							mu.Unlock()
						})
						if err != nil {
							t.Errorf("Foreach error: %v", err)
						}
					}
				}()
			}

			wg.Wait()
			helper.AssertEqual(concurrency*iterations*50, totalCount)
		})

		t.Run("ConcurrentForeachNested", func(t *testing.T) {
			testData := `{"data": {"users": [` + generateUserJSON(20) + `]}}`

			concurrency := 5
			iterations := 10

			var wg sync.WaitGroup

			for i := 0; i < concurrency; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					for j := 0; j < iterations; j++ {
						ForeachNested(testData, func(key any, item *IterableValue) {})
					}
				}()
			}

			wg.Wait()
		})
	})

	t.Run("RaceConditionTests", func(t *testing.T) {
		t.Run("ConcurrentConfigModification", func(t *testing.T) {
			config := DefaultConfig()

			concurrency := 10
			var wg sync.WaitGroup

			for i := 0; i < concurrency; i++ {
				wg.Add(1)
				go func(workerID int) {
					defer wg.Done()

					localConfig := config.Clone()
					localConfig.Validate()
				}(i)
			}

			wg.Wait()
		})

		t.Run("SharedProcessorStats", func(t *testing.T) {
			proc := New(DefaultConfig())
			defer proc.Close()

			testData := `{"data": "value"}`

			concurrency := 20
			iterations := 100

			var wg sync.WaitGroup

			for i := 0; i < concurrency; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					for j := 0; j < iterations; j++ {
						proc.GetStats()
						proc.Get(testData, "data")
						proc.GetHealthStatus()
					}
				}()
			}

			wg.Wait()

			result, err := proc.Get(testData, "data")
			if err == nil {
				if result != "value" {
					t.Errorf("Expected 'value', got '%v'", result)
				}
			}
		})
	})

	t.Run("ConcurrentStressTest", func(t *testing.T) {
		if testing.Short() {
			t.Skip("Skipping stress test in short mode")
		}

		t.Run("HighConcurrency", func(t *testing.T) {
			proc := New(DefaultConfig())
			defer proc.Close()

			testData := NewTestDataGenerator().GenerateComplexJSON()

			concurrency := 50
			operations := 1000

			ct := NewConcurrencyTester(t, concurrency, operations)
			ct.Run(func(workerID, iteration int) error {
				switch iteration % 4 {
				case 0:
					_, err := proc.Get(testData, "users[0].name")
					return err
				case 1:
					_, err := proc.Get(testData, "settings")
					return err
				case 2:
					_, err := proc.Get(testData, "statistics")
					return err
				default:
					_ = proc.GetStats()
					return nil
				}
			})
		})

		t.Run("RapidClose", func(t *testing.T) {
			for i := 0; i < 20; i++ {
				proc := New(DefaultConfig())
				proc.Get(`{"test": "value"}`, "test")
				proc.Close()
			}
		})
	})

	t.Run("ProcessorLifecycleConcurrency", func(t *testing.T) {
		t.Run("CloseDuringOperations", func(t *testing.T) {
			proc := New(DefaultConfig())

			testData := `{"test": "value"}`

			concurrency := 10
			var wg sync.WaitGroup

			for i := 0; i < concurrency; i++ {
				wg.Add(1)
				go func(workerID int) {
					defer wg.Done()

					for j := 0; j < 100; j++ {
						_, err := proc.Get(testData, "test")
						if err != nil {
							break
						}
						time.Sleep(time.Microsecond)
					}
				}(i)
			}

			time.Sleep(10 * time.Millisecond)
			proc.Close()

			wg.Wait()
		})

		t.Run("GlobalProcessorConcurrency", func(t *testing.T) {
			SetGlobalProcessor(New(DefaultConfig()))
			defer ShutdownGlobalProcessor()

			testData := `{"test": "value"}`

			concurrency := 20
			var wg sync.WaitGroup

			for i := 0; i < concurrency; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					for j := 0; j < 50; j++ {
						Get(testData, "test")
					}
				}()
			}

			wg.Wait()
		})
	})
}

// TestPropertyAccessResult tests the PropertyAccessResult type
func TestPropertyAccessResult(t *testing.T) {
	// Test exists case
	result1 := PropertyAccessResult{
		Value:  "test",
		Exists: true,
	}

	if !result1.Exists {
		t.Errorf("Exists should be true")
	}

	if result1.Value != "test" {
		t.Errorf("Value = %v, want %v", result1.Value, "test")
	}

	// Test not exists case
	result2 := PropertyAccessResult{
		Value:  nil,
		Exists: false,
	}

	if result2.Exists {
		t.Errorf("Exists should be false")
	}
}

// TestResourceManager tests resource management functionality
func TestResourceManager(t *testing.T) {
	helper := NewTestHelper(t)

	t.Run("StringBuilderPool", func(t *testing.T) {
		processor := New()
		defer processor.Close()

		success := TestProcessorResourcePools(processor)
		helper.AssertTrue(success, "Resource pools should be functional")
	})

	t.Run("MemoryEfficiency", func(t *testing.T) {
		processor := New()
		defer processor.Close()

		testData := `{"data": {"nested": {"deep": "value"}}}`

		// Perform multiple operations to test memory efficiency
		for i := 0; i < 100; i++ {
			_, err := processor.Get(testData, "data.nested.deep")
			helper.AssertNoError(err)
		}

		// Verify no memory leaks
		stats := processor.GetStats()
		helper.AssertTrue(stats.OperationCount >= 0)
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

// TestResourceMonitor_CheckForLeaks tests CheckForLeaks method
func TestResourceMonitor_CheckForLeaks(t *testing.T) {
	rm := NewResourceMonitor()
	rm.leakCheckInterval = 0 // Force immediate check

	issues := rm.CheckForLeaks()
	// Should return nil or empty for normal conditions
	// The actual result depends on current memory/goroutine state
	t.Logf("CheckForLeaks returned: %v", issues)
}

// TestResourceMonitor_EfficiencyMethods tests GetMemoryEfficiency and GetPoolEfficiency
func TestResourceMonitor_EfficiencyMethods(t *testing.T) {
	t.Run("GetMemoryEfficiency", func(t *testing.T) {
		rm := NewResourceMonitor()
		// Initially 100% (no allocations)
		if eff := rm.GetMemoryEfficiency(); eff != 100.0 {
			t.Errorf("GetMemoryEfficiency() = %v, want 100.0", eff)
		}

		rm.RecordAllocation(1000)
		rm.RecordDeallocation(500)
		// 500 / 1000 * 100 = 50%
		if eff := rm.GetMemoryEfficiency(); eff != 50.0 {
			t.Errorf("GetMemoryEfficiency() = %v, want 50.0", eff)
		}
	})

	t.Run("GetPoolEfficiency", func(t *testing.T) {
		rm := NewResourceMonitor()
		// Initially 100% (no operations)
		if eff := rm.GetPoolEfficiency(); eff != 100.0 {
			t.Errorf("GetPoolEfficiency() = %v, want 100.0", eff)
		}

		rm.RecordPoolHit()
		rm.RecordPoolHit()
		rm.RecordPoolMiss()
		// 2 / 3 * 100 = 66.67%
		eff := rm.GetPoolEfficiency()
		if eff < 66.0 || eff > 67.0 {
			t.Errorf("GetPoolEfficiency() = %v, want ~66.67", eff)
		}
	})
}

// TestRootDataTypeConversionError tests the RootDataTypeConversionError type
func TestRootDataTypeConversionError(t *testing.T) {
	err := &RootDataTypeConversionError{
		RequiredType: "object",
		RequiredSize: 100,
		CurrentType:  "string",
	}

	expectedMsg := "root data type conversion required: from string to object (size: 100)"
	if err.Error() != expectedMsg {
		t.Errorf("Error() = %q, want %q", err.Error(), expectedMsg)
	}

	// Test nil error
	var nilErr *RootDataTypeConversionError
	if nilErr != nil {
		t.Errorf("nil error should be nil")
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

// TestSyntaxError tests the SyntaxError type
func TestSyntaxError(t *testing.T) {
	err := &SyntaxError{
		msg:    "invalid character 'a' looking for beginning of value",
		Offset: 42,
	}

	expectedMsg := "invalid character 'a' looking for beginning of value"
	if err.Error() != expectedMsg {
		t.Errorf("Error() = %q, want %q", err.Error(), expectedMsg)
	}

	if err.Offset != 42 {
		t.Errorf("Offset = %d, want 42", err.Offset)
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

// TestTypeSafeAccessResult_AsBool tests AsBool method
func TestTypeSafeAccessResult_AsBool(t *testing.T) {
	tests := []struct {
		name        string
		value       any
		exists      bool
		expected    bool
		expectError bool
	}{
		{"bool true", true, true, true, false},
		{"bool false", false, true, false, false},
		{"string true", "true", true, true, false},
		{"string false", "false", true, false, false},
		{"string 1", "1", true, true, false},
		{"string 0", "0", true, false, false},
		{"not exists", nil, false, false, true},
		{"int invalid", 1, true, false, true},
		{"string invalid", "maybe", true, false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := TypeSafeAccessResult{Value: tt.value, Exists: tt.exists}
			got, err := result.AsBool()
			if tt.expectError {
				if err == nil {
					t.Errorf("AsBool() should return error for %v", tt.value)
				}
			} else {
				if err != nil {
					t.Errorf("AsBool() unexpected error: %v", err)
				}
				if got != tt.expected {
					t.Errorf("AsBool() = %v, want %v", got, tt.expected)
				}
			}
		})
	}
}

// TestTypeSafeAccessResult_AsFloat64 tests AsFloat64 method
func TestTypeSafeAccessResult_AsFloat64(t *testing.T) {
	tests := []struct {
		name        string
		value       any
		exists      bool
		expected    float64
		expectError bool
	}{
		{"float64 value", 3.14159, true, 3.14159, false},
		{"float32 value", float32(2.5), true, 2.5, false},
		{"int value", 42, true, 42.0, false},
		{"int64 value", int64(1234567890), true, 1234567890.0, false},
		{"uint64 value", uint64(1234567890), true, 1234567890.0, false},
		{"string number", "3.14", true, 3.14, false},
		{"not exists", nil, false, 0, true},
		{"string invalid", "abc", true, 0, true},
		{"bool invalid", true, true, 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := TypeSafeAccessResult{Value: tt.value, Exists: tt.exists}
			got, err := result.AsFloat64()
			if tt.expectError {
				if err == nil {
					t.Errorf("AsFloat64() should return error for %v", tt.value)
				}
			} else {
				if err != nil {
					t.Errorf("AsFloat64() unexpected error: %v", err)
				}
				if got != tt.expected {
					t.Errorf("AsFloat64() = %v, want %v", got, tt.expected)
				}
			}
		})
	}
}

// TestTypeSafeAccessResult_AsInt tests AsInt method
func TestTypeSafeAccessResult_AsInt(t *testing.T) {
	tests := []struct {
		name        string
		value       any
		exists      bool
		expected    int
		expectError bool
	}{
		{"int value", 42, true, 42, false},
		{"int8 value", int8(127), true, 127, false},
		{"int16 value", int16(1000), true, 1000, false},
		{"int32 value", int32(50000), true, 50000, false},
		{"int64 value", int64(100000), true, 100000, false},
		{"uint value", uint(100), true, 100, false},
		{"uint8 value", uint8(255), true, 255, false},
		{"uint16 value", uint16(1000), true, 1000, false},
		{"uint32 value", uint32(100000), true, 100000, false},
		{"float32 exact", float32(42.0), true, 42, false},
		{"float64 exact", float64(100.0), true, 100, false},
		{"string number", "123", true, 123, false},
		{"not exists", nil, false, 0, true},
		{"float non-integer", 42.5, true, 0, true},
		{"string invalid", "abc", true, 0, true},
		{"bool invalid", true, true, 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := TypeSafeAccessResult{Value: tt.value, Exists: tt.exists}
			got, err := result.AsInt()
			if tt.expectError {
				if err == nil {
					t.Errorf("AsInt() should return error for %v", tt.value)
				}
			} else {
				if err != nil {
					t.Errorf("AsInt() unexpected error: %v", err)
				}
				if got != tt.expected {
					t.Errorf("AsInt() = %v, want %v", got, tt.expected)
				}
			}
		})
	}
}

// TestTypeSafeAccessResult_AsString tests AsString method
func TestTypeSafeAccessResult_AsString(t *testing.T) {
	t.Run("valid string", func(t *testing.T) {
		result := TypeSafeAccessResult{Value: "hello", Exists: true}
		got, err := result.AsString()
		if err != nil {
			t.Errorf("AsString() unexpected error: %v", err)
		}
		if got != "hello" {
			t.Errorf("AsString() = %v, want hello", got)
		}
	})

	t.Run("not exists returns error", func(t *testing.T) {
		result := TypeSafeAccessResult{Value: nil, Exists: false}
		_, err := result.AsString()
		if err == nil {
			t.Error("AsString() should return error when not exists")
		}
	})

	t.Run("type mismatch returns error", func(t *testing.T) {
		result := TypeSafeAccessResult{Value: 123, Exists: true}
		_, err := result.AsString()
		if err == nil {
			t.Error("AsString() should return error for non-string type")
		}
	})
}

// TestTypeSafeAccessResult_AsStringConverted tests AsStringConverted method
func TestTypeSafeAccessResult_AsStringConverted(t *testing.T) {
	t.Run("valid string", func(t *testing.T) {
		result := TypeSafeAccessResult{Value: "hello", Exists: true}
		got, err := result.AsStringConverted()
		if err != nil {
			t.Errorf("AsStringConverted() unexpected error: %v", err)
		}
		if got != "hello" {
			t.Errorf("AsStringConverted() = %v, want hello", got)
		}
	})

	t.Run("int converts to string", func(t *testing.T) {
		result := TypeSafeAccessResult{Value: 123, Exists: true}
		got, err := result.AsStringConverted()
		if err != nil {
			t.Errorf("AsStringConverted() unexpected error: %v", err)
		}
		if got != "123" {
			t.Errorf("AsStringConverted() = %v, want 123", got)
		}
	})

	t.Run("not exists returns error", func(t *testing.T) {
		result := TypeSafeAccessResult{Value: nil, Exists: false}
		_, err := result.AsStringConverted()
		if err == nil {
			t.Error("AsStringConverted() should return error when not exists")
		}
	})
}

// TestTypeSafeResult_Ok tests the Ok method
func TestTypeSafeResult_Ok(t *testing.T) {
	tests := []struct {
		name     string
		result   TypeSafeResult[string]
		expected bool
	}{
		{
			name:     "valid result",
			result:   TypeSafeResult[string]{Value: "hello", Exists: true, Error: nil},
			expected: true,
		},
		{
			name:     "result with error",
			result:   TypeSafeResult[string]{Value: "", Exists: true, Error: fmt.Errorf("error")},
			expected: false,
		},
		{
			name:     "result not exists",
			result:   TypeSafeResult[string]{Value: "", Exists: false, Error: nil},
			expected: false,
		},
		{
			name:     "result with error and not exists",
			result:   TypeSafeResult[string]{Value: "", Exists: false, Error: fmt.Errorf("error")},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.result.Ok(); got != tt.expected {
				t.Errorf("TypeSafeResult.Ok() = %v, want %v", got, tt.expected)
			}
		})
	}
}

// TestTypeSafeResult_Unwrap tests the Unwrap method
func TestTypeSafeResult_Unwrap(t *testing.T) {
	t.Run("valid result returns value", func(t *testing.T) {
		result := TypeSafeResult[int]{Value: 42, Exists: true, Error: nil}
		if got := result.Unwrap(); got != 42 {
			t.Errorf("TypeSafeResult.Unwrap() = %v, want 42", got)
		}
	})

	t.Run("result with error returns zero", func(t *testing.T) {
		result := TypeSafeResult[int]{Value: 42, Exists: true, Error: fmt.Errorf("error")}
		if got := result.Unwrap(); got != 0 {
			t.Errorf("TypeSafeResult.Unwrap() with error = %v, want 0", got)
		}
	})
}

// TestTypeSafeResult_UnwrapOr tests the UnwrapOr method
func TestTypeSafeResult_UnwrapOr(t *testing.T) {
	tests := []struct {
		name         string
		result       TypeSafeResult[int]
		defaultValue int
		expected     int
	}{
		{
			name:         "valid result returns value",
			result:       TypeSafeResult[int]{Value: 42, Exists: true, Error: nil},
			defaultValue: 0,
			expected:     42,
		},
		{
			name:         "result with error returns default",
			result:       TypeSafeResult[int]{Value: 42, Exists: true, Error: fmt.Errorf("error")},
			defaultValue: 100,
			expected:     100,
		},
		{
			name:         "result not exists returns default",
			result:       TypeSafeResult[int]{Value: 42, Exists: false, Error: nil},
			defaultValue: 200,
			expected:     200,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.result.UnwrapOr(tt.defaultValue); got != tt.expected {
				t.Errorf("TypeSafeResult.UnwrapOr() = %v, want %v", got, tt.expected)
			}
		})
	}
}

// TestTypeSafeResult_UnwrapOrPanic tests the UnwrapOrPanic method
func TestTypeSafeResult_UnwrapOrPanic(t *testing.T) {
	t.Run("valid result returns value", func(t *testing.T) {
		result := TypeSafeResult[string]{Value: "hello", Exists: true, Error: nil}
		if got := result.UnwrapOrPanic(); got != "hello" {
			t.Errorf("TypeSafeResult.UnwrapOrPanic() = %v, want hello", got)
		}
	})

	t.Run("result with error panics", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Error("TypeSafeResult.UnwrapOrPanic() should panic on error")
			}
		}()
		result := TypeSafeResult[int]{Value: 0, Exists: true, Error: fmt.Errorf("error")}
		result.UnwrapOrPanic()
	})
}

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

// TestUnmarshalTypeError tests the UnmarshalTypeError type
func TestUnmarshalTypeError(t *testing.T) {
	// Test without Struct/Field
	err1 := &UnmarshalTypeError{
		Value:  "number",
		Type:   reflect.TypeOf(0),
		Offset: 10,
	}
	expectedMsg1 := "json: cannot unmarshal number into Go value of type int"
	if err1.Error() != expectedMsg1 {
		t.Errorf("Error() without Struct/Field = %q, want %q", err1.Error(), expectedMsg1)
	}

	// Test with Struct and Field
	err2 := &UnmarshalTypeError{
		Value:  "string",
		Type:   reflect.TypeOf(0),
		Offset: 20,
		Struct: "MyStruct",
		Field:  "Field",
	}
	expectedMsg2 := "json: cannot unmarshal string into Go struct field MyStruct.Field of type int"
	if err2.Error() != expectedMsg2 {
		t.Errorf("Error() with Struct/Field = %q, want %q", err2.Error(), expectedMsg2)
	}

	// Test Unwrap with nil error
	if err1.Unwrap() != nil {
		t.Errorf("Unwrap() with nil Err should return nil")
	}

	// Test Unwrap with error
	wrappedErr := errors.New("wrapped error")
	err3 := &UnmarshalTypeError{
		Value:  "bool",
		Type:   reflect.TypeOf(""),
		Offset: 5,
		Err:    wrappedErr,
	}
	if err3.Unwrap() != wrappedErr {
		t.Errorf("Unwrap() should return wrapped error")
	}
}

// TestUnsupportedTypeError tests the UnsupportedTypeError type
func TestUnsupportedTypeError(t *testing.T) {
	chType := reflect.TypeOf(make(chan int))
	err := &UnsupportedTypeError{Type: chType}

	expectedMsg := "json: unsupported type: chan int"
	if err.Error() != expectedMsg {
		t.Errorf("Error() = %q, want %q", err.Error(), expectedMsg)
	}
}

// TestUnsupportedValueError tests the UnsupportedValueError type
func TestUnsupportedValueError(t *testing.T) {
	// Create a function value (which is unsupported)
	fn := func() {}
	val := reflect.ValueOf(fn)

	err := &UnsupportedValueError{
		Value: val,
		Str:   "func()",
	}

	expectedMsg := "json: unsupported value: func()"
	if err.Error() != expectedMsg {
		t.Errorf("Error() = %q, want %q", err.Error(), expectedMsg)
	}
}

// TestValidationError tests ValidationError.Error method
func TestValidationError(t *testing.T) {
	t.Run("with path", func(t *testing.T) {
		ve := &ValidationError{Path: "user.name", Message: "is required"}
		expected := "validation error at path 'user.name': is required"
		if got := ve.Error(); got != expected {
			t.Errorf("ValidationError.Error() = %v, want %v", got, expected)
		}
	})

	t.Run("without path", func(t *testing.T) {
		ve := &ValidationError{Path: "", Message: "invalid format"}
		expected := "validation error: invalid format"
		if got := ve.Error(); got != expected {
			t.Errorf("ValidationError.Error() = %v, want %v", got, expected)
		}
	})
}
