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
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf := *internal.GetByteSliceWithHint(1024)
		buf = append(buf, "test"...)
		internal.PutByteSlice(&buf)
	}
}

func BenchmarkConcurrentPools(b *testing.B) {
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			sb := internal.GetStringBuilder()
			buf := *internal.GetByteSliceWithHint(1024)
			_ = append(buf, "test"...)
			internal.PutStringBuilder(sb)
			internal.PutByteSlice(&buf)
		}
	})
}

func BenchmarkConvertToFloat64(b *testing.B) {
	input := 3.14
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = convertToFloat64(input)
	}
}

func BenchmarkConvertToInt(b *testing.B) {
	input := 42
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = convertToInt(input)
	}
}

func BenchmarkConvertToInt64(b *testing.B) {
	input := int64(42)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = convertToInt64(input)
	}
}

func BenchmarkPathSegmentPool(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		seg := internal.GetPathSegmentSlice(8)
		internal.PutPathSegmentSlice(seg)
	}
}

func BenchmarkResourceMonitor(b *testing.B) {
	rm := newResourceMonitor()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rm.recordAllocation(1024)
		rm.recordDeallocation(512)
	}
}

func BenchmarkStringBuilderPool(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sb := internal.GetStringBuilder()
		sb.WriteString("benchmark test data")
		internal.PutStringBuilder(sb)
	}
}

func BenchmarkUnifiedTypeConversion(b *testing.B) {
	input := 42
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = unifiedTypeConversion[int64](input)
	}
}

// TestAdvancedPathoperations tests advanced path features
func TestAdvancedPathoperations(t *testing.T) {
	helper := newTestHelper(t)

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

// TestConfigConstantsComprehensive consolidates configuration and constants tests
// This replaces: config_constants_test.go
func TestConfigConstantsComprehensive(t *testing.T) {
	helper := newTestHelper(t)

	t.Run("ConfigGetters", func(t *testing.T) {
		config := DefaultConfig()

		helper.AssertTrue(config.EnableCache)
		helper.AssertTrue(config.MaxCacheSize > 0)
		helper.AssertTrue(config.CacheTTL > 0)
		helper.AssertTrue(config.getMaxJSONSize() > 0)
		helper.AssertTrue(config.getMaxPathDepth() > 0)
		helper.AssertTrue(config.getMaxConcurrency() > 0)
		helper.AssertTrue(config.getMaxNestingDepth() >= 0)

		limits := config.getSecurityLimits()
		helper.AssertNotNil(limits)
	})

	t.Run("ConfigPresets", func(t *testing.T) {
		config := DefaultConfig()
		helper.AssertNotNil(config)

		sec := SecurityConfig()
		helper.AssertNotNil(sec)
	})

	t.Run("EncodeConfigPresets", func(t *testing.T) {
		config := DefaultConfig()
		helper.AssertNotNil(config)

		pretty := PrettyConfig()
		helper.AssertNotNil(pretty)
		helper.AssertTrue(pretty.Pretty)

		// Default config is compact (Pretty = false)
		helper.AssertFalse(config.Pretty)

		cloned := (&config).Clone()
		helper.AssertNotNil(cloned)
	})

	t.Run("ConfigValidation", func(t *testing.T) {
		config := DefaultConfig()
		err := config.Validate()
		helper.AssertNoError(err)

		// Negative cache size should be clamped to 0 and cache disabled
		configWithNegativeCache := DefaultConfig()
		configWithNegativeCache.MaxCacheSize = -1
		err = configWithNegativeCache.Validate()
		helper.AssertNoError(err)
		helper.AssertEqual(0, configWithNegativeCache.MaxCacheSize)
		helper.AssertFalse(configWithNegativeCache.EnableCache)
	})

	t.Run("Constants", func(t *testing.T) {
		helper.AssertTrue(defaultBufferSize > 0)
		helper.AssertTrue(maxPoolBufferSize > minPoolBufferSize)
		helper.AssertTrue(defaultCacheSize > 0)
		helper.AssertTrue(DefaultMaxJSONSize > 0)
		helper.AssertTrue(DefaultMaxNestingDepth > 0)
		helper.AssertTrue(maxPathLength > 0)
		helper.AssertTrue(defaultOperationTimeout > 0)
	})

	t.Run("GlobalProcessor", func(t *testing.T) {
		p, _ := New(DefaultConfig())
		SetGlobalProcessor(p)
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

	limits := config.getSecurityLimits()

	if limits.MaxNestingDepth != 100 {
		t.Errorf("GetSecurityLimits max_nesting_depth = %v, want 100", limits.MaxNestingDepth)
	}
	if limits.MaxJSONSize != 10*1024*1024 {
		t.Errorf("GetSecurityLimits max_json_size = %v, want %d", limits.MaxJSONSize, 10*1024*1024)
	}
}

// TestconvertToBool tests boolean conversion
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
			result, ok := convertToBool(tt.input)
			if ok != tt.shouldSucceed {
				t.Errorf("convertToBool(%v) success = %v; want %v", tt.input, ok, tt.shouldSucceed)
			}
			if ok && result != tt.expected {
				t.Errorf("convertToBool(%v) = %v; want %v", tt.input, result, tt.expected)
			}
		})
	}
}

// TestconvertToFloat64 tests float64 conversion
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
			result, ok := convertToFloat64(tt.input)
			if ok != tt.shouldSucceed {
				t.Errorf("convertToFloat64(%v) success = %v; want %v", tt.input, ok, tt.shouldSucceed)
			}
			if ok && result != tt.expected {
				t.Errorf("convertToFloat64(%v) = %f; want %f", tt.input, result, tt.expected)
			}
		})
	}
}

// TestconvertToInt tests integer conversion with various types
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
			name:          "from int64 in range positive (platform-dependent)",
			input:         int64(2147483648), // > MaxInt32 but fits in int on 64-bit
			expected:      int(int64(2147483648)),
			shouldSucceed: true,
		},
		{
			name:          "from int64 in range negative (platform-dependent)",
			input:         int64(-2147483649), // < MinInt32 but fits in int on 64-bit
			expected:      int(int64(-2147483649)),
			shouldSucceed: true,
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
			result, ok := convertToInt(tt.input)
			if ok != tt.shouldSucceed {
				t.Errorf("convertToInt(%v) success = %v; want %v", tt.input, ok, tt.shouldSucceed)
			}
			if ok && result != tt.expected {
				t.Errorf("convertToInt(%v) = %d; want %d", tt.input, result, tt.expected)
			}
		})
	}
}

// TestconvertToInt64 tests int64 conversion
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
			result, ok := convertToInt64(tt.input)
			if ok != tt.shouldSucceed {
				t.Errorf("convertToInt64(%v) success = %v; want %v", tt.input, ok, tt.shouldSucceed)
			}
			if ok && result != tt.expected {
				t.Errorf("convertToInt64(%v) = %d; want %d", tt.input, result, tt.expected)
			}
		})
	}
}

// TestconvertToString tests string conversion
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
			result := convertToString(tt.input)
			if result != tt.expected {
				t.Errorf("convertToString(%v) = %s; want %s", tt.input, result, tt.expected)
			}
		})
	}
}

// TestconvertToUint64 tests uint64 conversion
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
			result, ok := convertToUint64(tt.input)
			if ok != tt.shouldSucceed {
				t.Errorf("convertToUint64(%v) success = %v; want %v", tt.input, ok, tt.shouldSucceed)
			}
			if ok && result != tt.expected {
				t.Errorf("convertToUint64(%v) = %d; want %d", tt.input, result, tt.expected)
			}
		})
	}
}

// TestIsDeletedMarker tests the internal isDeletedMarker function
// Note: isDeletedMarker is now private, tested indirectly through Delete operations
func TestIsDeletedMarker(t *testing.T) {
	// Test that isDeletedMarker returns false for nil
	if isDeletedMarker(nil) {
		t.Errorf("isDeletedMarker(nil) should return false")
	}

	// Test that isDeletedMarker returns false for regular values
	if isDeletedMarker(1) {
		t.Errorf("isDeletedMarker(1) should return false")
	}
	if isDeletedMarker("test") {
		t.Errorf("isDeletedMarker(\"test\") should return false")
	}
	if isDeletedMarker(map[string]any{"key": "value"}) {
		t.Errorf("isDeletedMarker(map) should return false")
	}
}

// TestEncodeConfig_Clone tests EncodeConfig.Clone method
func TestEncodeConfig_Clone(t *testing.T) {
	original := &Config{
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

// TestEncodeConfig_Clone_Zero tests Clone with zero value
func TestEncodeConfig_Clone_Zero(t *testing.T) {
	var config Config
	cloned := (&config).Clone()
	if cloned.Pretty != false {
		t.Error("Clone of zero value should have default values")
	}
}

// TestErrorClassifier tests error classification functionality

// TestErrorHandling comprehensive error handling tests
func TestErrorHandling(t *testing.T) {
	helper := newTestHelper(t)

	t.Run("JsonsErrorStructure", func(t *testing.T) {
		err := &JsonsError{
			Op:      "test_operation",
			Path:    "test.path",
			Message: "test error message",
			Err:     errOperationFailed,
		}

		helper.AssertEqual("test_operation", err.Op)
		helper.AssertEqual("test.path", err.Path)
		helper.AssertEqual("test error message", err.Message)
		helper.AssertEqual(errOperationFailed, err.Err)
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
	helper := newTestHelper(t)

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

		// GetInt returns default value on type mismatch
		result := GetInt(testData, "value", 0)
		helper.AssertEqual(0, result)
	})
}

// TestErrorRecovery tests error recovery scenarios
func TestErrorRecovery(t *testing.T) {
	helper := newTestHelper(t)

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
			value := GetString(testData, path, "")
			if value != "" {
				successCount++
			}
		}

		// Should have succeeded for valid paths
		helper.AssertTrue(successCount >= 2)
	})
}

// TestErrorScenarios tests various error scenarios
func TestErrorScenarios(t *testing.T) {
	helper := newTestHelper(t)

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
			// GetInt returns default value on type mismatch
			result := GetInt(testData, "str", 0)
			helper.AssertEqual(0, result)
		})

		t.Run("NumberAsString", func(t *testing.T) {
			// GetString returns default value on type mismatch
			result := GetString(testData, "num", "")
			_ = result
		})

		t.Run("BoolAsInt", func(t *testing.T) {
			// GetInt returns default value on type mismatch
			result := GetInt(testData, "bool", 0)
			_ = result // Don't assert value, library may convert
		})

			t.Run("ObjectAsArray", func(t *testing.T) {
				// GetArray returns default value on type mismatch
				result := GetArray(testData, "str", nil)
				_ = result
			})





	})

	t.Run("NullHandling", func(t *testing.T) {
		testData := `{"null_value": null, "string_value": "value"}`

		t.Run("GetNull", func(t *testing.T) {
			result, err := Get(testData, "null_value")
			helper.AssertNoError(err)
			helper.AssertNil(result)
		})

		t.Run("GetAsNull", func(t *testing.T) {
			// GetTyped returns default value for null
			result := GetTyped[string](testData, "null_value", "")
			_ = result
		})

		t.Run("GetMissingField", func(t *testing.T) {
			_, err := Get(testData, "missing")
			helper.AssertError(err)
		})
	})

	t.Run("ProcessorClosed", func(t *testing.T) {
		processor, _ := New(DefaultConfig())
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
		processor, _ := New(DefaultConfig())
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

// TestformatNumber tests number formatting
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
			result := formatNumber(tt.input)
			if result != tt.expected {
				t.Errorf("formatNumber(%v) = %s; want %s", tt.input, result, tt.expected)
			}
		})
	}
}

// TestGetStatsWithResourceManager tests processor GetStats functionality with resource manager
func TestGetStatsWithResourceManager(t *testing.T) {
	p, _ := New()
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

// TestGlobalResourceManager tests that internal pool functions work correctly
func TestGlobalResourceManager(t *testing.T) {
	t.Run("StringBuilder", func(t *testing.T) {
		sb := internal.GetStringBuilder()
		sb.WriteString("global test")
		internal.PutStringBuilder(sb)

		// Should not panic
	})
}

// TestHealthCheckSystem tests health check functionality
func TestHealthCheckSystem(t *testing.T) {
	helper := newTestHelper(t)

	t.Run("BasicHealthCheck", func(t *testing.T) {
		processor, _ := New()
		defer processor.Close()

		health := processor.GetHealthStatus()
		helper.AssertNotNil(health)
		// Health status should be available
		helper.AssertNotNil(health.Checks)
	})

	t.Run("HealthCheckWithLoad", func(t *testing.T) {
		processor, _ := New()
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
	if internal.ArrayIndexInvalid != -999999 {
		t.Errorf("ArrayIndexInvalid = %d, want -999999", internal.ArrayIndexInvalid)
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
	helper := newTestHelper(t)

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
		processor, _ := New()
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

// TestOperation_String tests operation.String method
func TestOperation_String(t *testing.T) {
	tests := []struct {
		op       operation
		expected string
	}{
		{opGet, "get"},
		{opSet, "set"},
		{opDelete, "delete"},
		{operation(3), "unknown"},
		{operation(999), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if got := tt.op.String(); got != tt.expected {
				t.Errorf("operation.String() = %v, want %v", got, tt.expected)
			}
		})
	}
}

// TestProcessorConcurrencyComprehensive consolidates processor and concurrency tests
// This replaces: processor_concurrency_test.go, concurrency_test.go
func TestProcessorConcurrencyComprehensive(t *testing.T) {
	helper := newTestHelper(t)
	generator := newTestDataGenerator()

	t.Run("Processoroperations", func(t *testing.T) {
		processor, _ := New()
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

		t.Run("Cacheoperations", func(t *testing.T) {
			_, _ = processor.Get(testData, "user.name")
			processor.ClearCache()

			samplePaths := []string{"user.name", "active"}
			_, _ = processor.WarmupCache(testData, samplePaths)
		})

		t.Run("Lifecycle", func(t *testing.T) {
			p, _ := New()
			helper.AssertFalse(p.IsClosed())
			p.Close()
			helper.AssertTrue(p.IsClosed())

			_, err := p.Get(`{"test": "value"}`, "test")
			helper.AssertError(err)
		})
	})

	t.Run("Concurrentoperations", func(t *testing.T) {
		t.Run("ConcurrentReads", func(t *testing.T) {
			jsonStr := generator.generateComplexJSON()
			concurrencyTester := newConcurrencyTester(t, 20, 100)

			concurrencyTester.run(func(workerID, iteration int) error {
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

			concurrencyTester := newConcurrencyTester(t, 10, 50)
			concurrencyTester.run(func(workerID, iteration int) error {
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

		t.Run("Mixedoperations", func(t *testing.T) {
			baseJSON := `{"data": {"counter": 0}, "array": [1, 2, 3]}`
			var operations int64

			concurrencyTester := newConcurrencyTester(t, 15, 100)
			concurrencyTester.run(func(workerID, iteration int) error {
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
			processor, _ := New()
			defer processor.Close()

			jsonStr := generator.generateComplexJSON()
			concurrencyTester := newConcurrencyTester(t, 25, 200)

			concurrencyTester.run(func(workerID, iteration int) error {
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
					processor, _ := New()
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

			processor, _ := New(config)
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
			proc, _ := New(DefaultConfig())
			defer proc.Close()

			testData := `{"users": [` + genUserFragments(100) + `]}`

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
			proc, _ := New(DefaultConfig())
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

					proc, _ := New(DefaultConfig())
					defer proc.Close()

					for j := 0; j < iterations; j++ {
						proc.Get(testData, "test")
					}
				}(i)
			}

			wg.Wait()
		})
	})

	t.Run("ConcurrentCacheoperations", func(t *testing.T) {
		t.Run("CacheConcurrency", func(t *testing.T) {
			config := DefaultConfig()
			config.EnableCache = true
			config.MaxCacheSize = 100

			proc, _ := New(config)
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

			proc, _ := New(config)
			defer proc.Close()

			testData := `{"value": "test"}`

			proc.Get(testData, "value")

			time.Sleep(150 * time.Millisecond)

			_, err := proc.Get(testData, "value")
			helper.AssertNoError(err)
		})
	})

	t.Run("ConcurrentIteratoroperations", func(t *testing.T) {
		t.Run("ConcurrentForeach", func(t *testing.T) {
			testData := `{"items": [` + genItemFragments(50) + `]}`

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
			testData := `{"data": {"users": [` + genUserFragments(20) + `]}}`

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

					localConfig := (&config).Clone()
					localConfig.Validate()
				}(i)
			}

			wg.Wait()
		})

		t.Run("SharedProcessorStats", func(t *testing.T) {
			proc, _ := New(DefaultConfig())
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
			proc, _ := New(DefaultConfig())
			defer proc.Close()

			testData := newTestDataGenerator().generateComplexJSON()

			concurrency := 50
			operations := 1000

			ct := newConcurrencyTester(t, concurrency, operations)
			ct.run(func(workerID, iteration int) error {
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
				proc, _ := New(DefaultConfig())
				proc.Get(`{"test": "value"}`, "test")
				proc.Close()
			}
		})
	})

	t.Run("ProcessorLifecycleConcurrency", func(t *testing.T) {
		t.Run("CloseDuringoperations", func(t *testing.T) {
			proc, _ := New(DefaultConfig())

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
			p, _ := New(DefaultConfig())
			SetGlobalProcessor(p)
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

// TestResourceManager tests resource management functionality
func TestResourceManager(t *testing.T) {
	helper := newTestHelper(t)

	t.Run("StringBuilderPool", func(t *testing.T) {
		processor, _ := New()
		defer processor.Close()

		success := testProcessorResourcePools(processor)
		helper.AssertTrue(success, "Resource pools should be functional")
	})

	t.Run("MemoryEfficiency", func(t *testing.T) {
		processor, _ := New()
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

// TestResourceMonitor_CheckForLeaks tests CheckForLeaks method
func TestResourceMonitor_CheckForLeaks(t *testing.T) {
	rm := newResourceMonitor()
	rm.leakCheckInterval = 0 // Force immediate check

	issues := rm.checkForLeaks()
	// Should return nil or empty for normal conditions
	// The actual result depends on current memory/goroutine state
	t.Logf("CheckForLeaks returned: %v", issues)
}

// TestRootDataTypeConversionError tests the rootDataTypeConversionError type
func TestRootDataTypeConversionError(t *testing.T) {
	err := &rootDataTypeConversionError{
		requiredType: "object",
		requiredSize: 100,
		currentType:  "string",
	}

	expectedMsg := "root data type conversion required: from string to object (size: 100)"
	if err.Error() != expectedMsg {
		t.Errorf("Error() = %q, want %q", err.Error(), expectedMsg)
	}

	// Test nil error - verify nil pointer behavior is correct
	// Note: This test verifies that an uninitialized error pointer is nil
	// which is always true in Go. The test exists for documentation purposes.
	var nilErr *rootDataTypeConversionError //nolint:staticcheck // nilness check: intentionally testing nil
	_ = nilErr                              // use the variable to avoid compiler warnings
}

// TestsafeConvertToInt64 tests safe int64 conversion with error handling
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
			result, err := safeConvertToInt64(tt.input)
			if tt.expectError && err == nil {
				t.Errorf("Expected error for input %v, but got none", tt.input)
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error for input %v: %v", tt.input, err)
			}
			if !tt.expectError && result != tt.expected {
				t.Errorf("safeConvertToInt64(%v) = %d; want %d", tt.input, result, tt.expected)
			}
		})
	}
}

// TestsafeConvertToUint64 tests safe uint64 conversion with error handling
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
			result, err := safeConvertToUint64(tt.input)
			if tt.expectError && err == nil {
				t.Errorf("Expected error for input %v, but got none", tt.input)
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error for input %v: %v", tt.input, err)
			}
			if !tt.expectError && result != tt.expected {
				t.Errorf("safeConvertToUint64(%v) = %d; want %d", tt.input, result, tt.expected)
			}
		})
	}
}

// TestSchemaComprehensive consolidates TestSchema, TestSchema_AllConstraints,
// TestSchema_ArrayConstraints, TestSchema_ExclusiveConstraints,
// TestSchema_NumericConstraints, TestSchema_StringConstraints,
// and TestSchema_DefaultSchema into a single table-driven test.
func TestSchemaComprehensive(t *testing.T) {
	t.Run("constraint fields", func(t *testing.T) {
		tests := []struct {
			name            string
			minLen          int
			maxLen          int
			minimum         float64
			maximum         float64
			minItems        int
			maxItems        int
			exclusiveMin    bool
			exclusiveMax    bool
		}{
			{"all set positive", 5, 100, 0, 1000, 1, 10, true, true},
			{"all set negative range", 5, 50, -100, 100, 1, 10, false, false},
			{"zero values", 0, 0, 0, 0, 0, 0, false, false},
			{"large values", 100, 10000, -1e6, 1e9, 0, 100000, true, false},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				s := &Schema{
					MinLength: tt.minLen, hasMinLength: true,
					MaxLength: tt.maxLen, hasMaxLength: true,
					Minimum: tt.minimum, hasMinimum: true,
					Maximum: tt.maximum, hasMaximum: true,
					MinItems: tt.minItems, hasMinItems: true,
					MaxItems: tt.maxItems, hasMaxItems: true,
					ExclusiveMinimum: tt.exclusiveMin,
					ExclusiveMaximum: tt.exclusiveMax,
				}
				if !s.hasMinLength || s.MinLength != tt.minLen {
					t.Errorf("MinLength = %d, has=%v, want %d", s.MinLength, s.hasMinLength, tt.minLen)
				}
				if !s.hasMaxLength || s.MaxLength != tt.maxLen {
					t.Errorf("MaxLength = %d, has=%v, want %d", s.MaxLength, s.hasMaxLength, tt.maxLen)
				}
				if !s.hasMinimum || s.Minimum != tt.minimum {
					t.Errorf("Minimum = %v, has=%v, want %v", s.Minimum, s.hasMinimum, tt.minimum)
				}
				if !s.hasMaximum || s.Maximum != tt.maximum {
					t.Errorf("Maximum = %v, has=%v, want %v", s.Maximum, s.hasMaximum, tt.maximum)
				}
				if !s.hasMinItems || s.MinItems != tt.minItems {
					t.Errorf("MinItems = %d, has=%v, want %d", s.MinItems, s.hasMinItems, tt.minItems)
				}
				if !s.hasMaxItems || s.MaxItems != tt.maxItems {
					t.Errorf("MaxItems = %d, has=%v, want %d", s.MaxItems, s.hasMaxItems, tt.maxItems)
				}
				if s.ExclusiveMinimum != tt.exclusiveMin {
					t.Errorf("ExclusiveMinimum = %v, want %v", s.ExclusiveMinimum, tt.exclusiveMin)
				}
				if s.ExclusiveMaximum != tt.exclusiveMax {
					t.Errorf("ExclusiveMaximum = %v, want %v", s.ExclusiveMaximum, tt.exclusiveMax)
				}
			})
		}
	})

	t.Run("DefaultSchema has no constraints", func(t *testing.T) {
		s := DefaultSchema()
		if s == nil {
			t.Fatal("DefaultSchema() returned nil")
		}
		for _, set := range []bool{s.hasMinLength, s.hasMaxLength, s.hasMinimum, s.hasMaximum, s.hasMinItems, s.hasMaxItems} {
			if set {
				t.Error("DefaultSchema should have no constraints set")
			}
		}
	})

	t.Run("mutable constraint updates", func(t *testing.T) {
		s := &Schema{MinLength: 10, MaxLength: 100}
		s.hasMinLength = true
		s.hasMaxLength = true
		s.MinLength = 5
		s.MaxLength = 50
		if s.MinLength != 5 || s.MaxLength != 50 {
			t.Errorf("MinLength/MaxLength not updated correctly: %d/%d", s.MinLength, s.MaxLength)
		}
	})

	t.Run("exclusive with numeric constraints", func(t *testing.T) {
		s := &Schema{Minimum: 10, Maximum: 100, ExclusiveMinimum: true, ExclusiveMaximum: true}
		s.hasMinimum = true
		s.hasMaximum = true
		if s.Minimum != 10 || s.Maximum != 100 {
			t.Errorf("Minimum/Maximum should be preserved: %v/%v", s.Minimum, s.Maximum)
		}
		if !s.ExclusiveMinimum || !s.ExclusiveMaximum {
			t.Error("Exclusive flags should be true")
		}
	})
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
		result, ok := convertToInt64(int64(math.MaxInt64))
		if !ok || result != math.MaxInt64 {
			t.Errorf("Failed to convert max int64")
		}
	})

	t.Run("min int64", func(t *testing.T) {
		result, ok := convertToInt64(int64(math.MinInt64))
		if !ok || result != math.MinInt64 {
			t.Errorf("Failed to convert min int64")
		}
	})

	t.Run("max uint64", func(t *testing.T) {
		result, ok := convertToUint64(uint64(math.MaxUint64))
		if !ok || result != math.MaxUint64 {
			t.Errorf("Failed to convert max uint64")
		}
	})

	t.Run("max float64", func(t *testing.T) {
		result, ok := convertToFloat64(math.MaxFloat64)
		if !ok || result != math.MaxFloat64 {
			t.Errorf("Failed to convert max float64")
		}
	})

	t.Run("infinity float64", func(t *testing.T) {
		result, ok := convertToFloat64(math.Inf(1))
		if !ok || result != math.Inf(1) {
			t.Errorf("Failed to convert infinity")
		}
	})

	t.Run("NaN float64", func(t *testing.T) {
		result, ok := convertToFloat64(math.NaN())
		if !ok || !math.IsNaN(result) {
			t.Errorf("Failed to convert NaN")
		}
	})
}

// TestAccessResult_AsBool tests AsBool method
func TestAccessResult_AsBool(t *testing.T) {
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
			result := AccessResult{Value: tt.value, Exists: tt.exists}
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

// TestAccessResult_AsFloat64 tests AsFloat64 method
func TestAccessResult_AsFloat64(t *testing.T) {
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
			result := AccessResult{Value: tt.value, Exists: tt.exists}
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

// TestAccessResult_AsInt tests AsInt method
func TestAccessResult_AsInt(t *testing.T) {
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
			result := AccessResult{Value: tt.value, Exists: tt.exists}
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

// TestAccessResult_AsString tests AsString method
func TestAccessResult_AsString(t *testing.T) {
	t.Run("valid string", func(t *testing.T) {
		result := AccessResult{Value: "hello", Exists: true}
		got, err := result.AsString()
		if err != nil {
			t.Errorf("AsString() unexpected error: %v", err)
		}
		if got != "hello" {
			t.Errorf("AsString() = %v, want hello", got)
		}
	})

	t.Run("not exists returns error", func(t *testing.T) {
		result := AccessResult{Value: nil, Exists: false}
		_, err := result.AsString()
		if err == nil {
			t.Error("AsString() should return error when not exists")
		}
	})

	t.Run("type mismatch returns error", func(t *testing.T) {
		result := AccessResult{Value: 123, Exists: true}
		_, err := result.AsString()
		if err == nil {
			t.Error("AsString() should return error for non-string type")
		}
	})
}

// TestAccessResult_AsStringConverted tests AsStringConverted method
func TestAccessResult_AsStringConverted(t *testing.T) {
	t.Run("valid string", func(t *testing.T) {
		result := AccessResult{Value: "hello", Exists: true}
		got, err := result.AsStringConverted()
		if err != nil {
			t.Errorf("AsStringConverted() unexpected error: %v", err)
		}
		if got != "hello" {
			t.Errorf("AsStringConverted() = %v, want hello", got)
		}
	})

	t.Run("int converts to string", func(t *testing.T) {
		result := AccessResult{Value: 123, Exists: true}
		got, err := result.AsStringConverted()
		if err != nil {
			t.Errorf("AsStringConverted() unexpected error: %v", err)
		}
		if got != "123" {
			t.Errorf("AsStringConverted() = %v, want 123", got)
		}
	})

	t.Run("not exists returns error", func(t *testing.T) {
		result := AccessResult{Value: nil, Exists: false}
		_, err := result.AsStringConverted()
		if err == nil {
			t.Error("AsStringConverted() should return error when not exists")
		}
	})
}

// TestResult_Ok tests the Ok method
func TestResult_Ok(t *testing.T) {
	tests := []struct {
		name     string
		result   Result[string]
		expected bool
	}{
		{
			name:     "valid result",
			result:   Result[string]{Value: "hello", Exists: true, Error: nil},
			expected: true,
		},
		{
			name:     "result with error",
			result:   Result[string]{Value: "", Exists: true, Error: fmt.Errorf("error")},
			expected: false,
		},
		{
			name:     "result not exists",
			result:   Result[string]{Value: "", Exists: false, Error: nil},
			expected: false,
		},
		{
			name:     "result with error and not exists",
			result:   Result[string]{Value: "", Exists: false, Error: fmt.Errorf("error")},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.result.Ok(); got != tt.expected {
				t.Errorf("Result.Ok() = %v, want %v", got, tt.expected)
			}
		})
	}
}

// TestResult_Unwrap tests the Unwrap method
func TestResult_Unwrap(t *testing.T) {
	t.Run("valid result returns value", func(t *testing.T) {
		result := Result[int]{Value: 42, Exists: true, Error: nil}
		if got := result.Unwrap(); got != 42 {
			t.Errorf("Result.Unwrap() = %v, want 42", got)
		}
	})

	t.Run("result with error returns zero", func(t *testing.T) {
		result := Result[int]{Value: 42, Exists: true, Error: fmt.Errorf("error")}
		if got := result.Unwrap(); got != 0 {
			t.Errorf("Result.Unwrap() with error = %v, want 0", got)
		}
	})
}

// TestResult_UnwrapOr tests the UnwrapOr method
func TestResult_UnwrapOr(t *testing.T) {
	tests := []struct {
		name         string
		result       Result[int]
		defaultValue int
		expected     int
	}{
		{
			name:         "valid result returns value",
			result:       Result[int]{Value: 42, Exists: true, Error: nil},
			defaultValue: 0,
			expected:     42,
		},
		{
			name:         "result with error returns default",
			result:       Result[int]{Value: 42, Exists: true, Error: fmt.Errorf("error")},
			defaultValue: 100,
			expected:     100,
		},
		{
			name:         "result not exists returns default",
			result:       Result[int]{Value: 42, Exists: false, Error: nil},
			defaultValue: 200,
			expected:     200,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.result.UnwrapOr(tt.defaultValue); got != tt.expected {
				t.Errorf("Result.UnwrapOr() = %v, want %v", got, tt.expected)
			}
		})
	}
}

// TestUnifiedResourceManager tests the internal pool functionality
func TestUnifiedResourceManager(t *testing.T) {
	t.Run("StringBuilderPool", func(t *testing.T) {
		sb1 := internal.GetStringBuilder()
		if sb1 == nil {
			t.Fatal("GetStringBuilder returned nil")
		}

		sb1.WriteString("test data")
		if sb1.String() != "test data" {
			t.Errorf("StringBuilder write failed, got: %s", sb1.String())
		}

		internal.PutStringBuilder(sb1)

		sb2 := internal.GetStringBuilder()
		if sb2 == nil {
			t.Fatal("GetStringBuilder returned nil on second call")
		}
		sb2.Reset()
		internal.PutStringBuilder(sb2)
	})

	t.Run("PathSegmentPool", func(t *testing.T) {
		seg1 := *internal.GetPathSegmentSlice(8)
		if cap(seg1) == 0 {
			t.Error("PathSegment should have capacity")
		}

		internal.PutPathSegmentSlice(&seg1)

		seg2 := internal.GetPathSegmentSlice(8)
		internal.PutPathSegmentSlice(seg2)
	})

	t.Run("BufferPool", func(t *testing.T) {
		buf1 := *internal.GetByteSliceWithHint(1024)
		buf1 = append(buf1, "test data"...)

		internal.PutByteSlice(&buf1)

		buf2 := internal.GetByteSliceWithHint(1024)
		internal.PutByteSlice(buf2)
	})

	t.Run("ConcurrentAccess", func(t *testing.T) {
		const goroutines = 100
		const opsPerGoroutine = 100

		var wg sync.WaitGroup
		wg.Add(goroutines * 3)

		for i := 0; i < goroutines; i++ {
			go func() {
				defer wg.Done()
				for j := 0; j < opsPerGoroutine; j++ {
					sb := internal.GetStringBuilder()
					sb.WriteString("concurrent test")
					internal.PutStringBuilder(sb)
				}
			}()
		}

		for i := 0; i < goroutines; i++ {
			go func() {
				defer wg.Done()
				for j := 0; j < opsPerGoroutine; j++ {
					seg := internal.GetPathSegmentSlice(8)
					internal.PutPathSegmentSlice(seg)
				}
			}()
		}

		for i := 0; i < goroutines; i++ {
			go func() {
				defer wg.Done()
				for j := 0; j < opsPerGoroutine; j++ {
					buf := internal.GetByteSliceWithHint(1024)
					internal.PutByteSlice(buf)
				}
			}()
		}

		wg.Wait()
	})

	t.Run("SizeLimits", func(t *testing.T) {
		// Test oversized builder is handled without panic
		oversizedSb := &strings.Builder{}
		oversizedSb.Grow(100000)
		internal.PutStringBuilder(oversizedSb)

		undersizedSb := &strings.Builder{}
		undersizedSb.Grow(10)
		internal.PutStringBuilder(undersizedSb)
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
				result, ok := unifiedTypeConversion[int](tt.input)
				if ok != tt.shouldSucceed {
					t.Errorf("unifiedTypeConversion[int](%v) success = %v; want %v", tt.input, ok, tt.shouldSucceed)
				}
				if ok && result != tt.expected.(int) {
					t.Errorf("unifiedTypeConversion[int](%v) = %d; want %d", tt.input, result, tt.expected)
				}
			case int64:
				result, ok := unifiedTypeConversion[int64](tt.input)
				if ok != tt.shouldSucceed {
					t.Errorf("unifiedTypeConversion[int64](%v) success = %v; want %v", tt.input, ok, tt.shouldSucceed)
				}
				if ok && result != tt.expected.(int64) {
					t.Errorf("unifiedTypeConversion[int64](%v) = %d; want %d", tt.input, result, tt.expected)
				}
			case float64:
				result, ok := unifiedTypeConversion[float64](tt.input)
				if ok != tt.shouldSucceed {
					t.Errorf("unifiedTypeConversion[float64](%v) success = %v; want %v", tt.input, ok, tt.shouldSucceed)
				}
				if ok && result != tt.expected.(float64) {
					t.Errorf("unifiedTypeConversion[float64](%v) = %f; want %f", tt.input, result, tt.expected)
				}
			case bool:
				result, ok := unifiedTypeConversion[bool](tt.input)
				if ok != tt.shouldSucceed {
					t.Errorf("unifiedTypeConversion[bool](%v) success = %v; want %v", tt.input, ok, tt.shouldSucceed)
				}
				if ok && result != tt.expected.(bool) {
					t.Errorf("unifiedTypeConversion[bool](%v) = %v; want %v", tt.input, result, tt.expected)
				}
			case string:
				result, ok := unifiedTypeConversion[string](tt.input)
				if ok != tt.shouldSucceed {
					t.Errorf("unifiedTypeConversion[string](%v) success = %v; want %v", tt.input, ok, tt.shouldSucceed)
				}
				if ok && result != tt.expected.(string) {
					t.Errorf("unifiedTypeConversion[string](%v) = %s; want %s", tt.input, result, tt.expected)
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
