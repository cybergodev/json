package json

import (
	"fmt"
	"math"
	"strings"
	"testing"
)

// TestBoundaryConditions tests edge cases and boundary conditions
func TestBoundaryConditions(t *testing.T) {
	helper := NewTestHelper(t)

	t.Run("EmptyValues", func(t *testing.T) {
		t.Run("EmptyString", func(t *testing.T) {
			testData := `{"empty": "", "normal": "value"}`

			result, err := Get(testData, "empty")
			helper.AssertNoError(err)
			helper.AssertEqual("", result)

			// Empty with default
			withDefault := GetStringWithDefault(testData, "missing", "default")
			helper.AssertEqual("default", withDefault)
		})

		t.Run("EmptyArray", func(t *testing.T) {
			testData := `{"empty": [], "normal": [1, 2, 3]}`

			result, err := GetArray(testData, "empty")
			helper.AssertNoError(err)
			helper.AssertEqual(0, len(result))
		})

		t.Run("EmptyObject", func(t *testing.T) {
			testData := `{"empty": {}, "normal": {"key": "value"}}`

			result, err := GetObject(testData, "empty")
			helper.AssertNoError(err)
			helper.AssertEqual(0, len(result))
		})

		t.Run("NullValue", func(t *testing.T) {
			testData := `{"null_value": null, "string": "value"}`

			result, err := Get(testData, "null_value")
			helper.AssertNoError(err)
			helper.AssertNil(result)
		})
	})

	t.Run("NumericBoundaries", func(t *testing.T) {
		processor := New(DefaultConfig())
		defer processor.Close()

		t.Run("MaxInt64", func(t *testing.T) {
			testData := `{"max": 9223372036854775807}`

			result, err := processor.Get(testData, "max")
			helper.AssertNoError(err)

			if num, ok := result.(float64); ok {
				helper.AssertTrue(num == float64(math.MaxInt64) || num == float64(int64(math.MaxInt64)))
			}
		})

		t.Run("MinInt64", func(t *testing.T) {
			testData := `{"min": -9223372036854775808}`

			result, err := processor.Get(testData, "min")
			helper.AssertNoError(err)

			if num, ok := result.(float64); ok {
				// JSON numbers are typically float64, so min int64 might lose precision
				helper.AssertTrue(num <= float64(math.MinInt64))
			}
		})

		t.Run("MaxFloat", func(t *testing.T) {
			testData := `{"max_float": 1.7976931348623157e+308}`

			result, err := processor.Get(testData, "max_float")
			helper.AssertNoError(err)

			if num, ok := result.(float64); ok {
				helper.AssertTrue(num > 1.79e308 || num == math.MaxFloat64)
			}
		})

		t.Run("VerySmallFloat", func(t *testing.T) {
			testData := `{"small": 1e-323}`

			result, err := processor.Get(testData, "small")
			helper.AssertNoError(err)

			if num, ok := result.(float64); ok {
				helper.AssertTrue(num >= 0 && num < 1e-300)
			}
		})

		t.Run("ZeroValues", func(t *testing.T) {
			testData := `{"int_zero": 0, "float_zero": 0.0, "bool_false": false}`

			intZero, _ := GetInt(testData, "int_zero")
			helper.AssertEqual(0, intZero)

			floatZero, _ := GetFloat64(testData, "float_zero")
			helper.AssertEqual(0.0, floatZero)

			boolFalse, _ := GetBool(testData, "bool_false")
			helper.AssertFalse(boolFalse)
		})
	})

	t.Run("StringBoundaries", func(t *testing.T) {
		t.Run("VeryLongString", func(t *testing.T) {
			longString := strings.Repeat("a", 100000)
			testData := `{"long": "` + longString + `"}`

			result, err := Get(testData, "long")
			helper.AssertNoError(err)

			if str, ok := result.(string); ok {
				helper.AssertEqual(100000, len(str))
			}
		})

		t.Run("SpecialCharacters", func(t *testing.T) {
			specialChars := `\"\n\r\t\b\f\/\\`
			testData := `{"special": "` + specialChars + `"}`

			result, err := Get(testData, "special")
			helper.AssertNoError(err)

			if str, ok := result.(string); ok {
				helper.AssertTrue(len(str) > 0)
			}
		})

		t.Run("UnicodeEscape", func(t *testing.T) {
			testData := `{"unicode": "\u0048\u0065\u006c\u006c\u006f"}`

			result, err := Get(testData, "unicode")
			helper.AssertNoError(err)

			if str, ok := result.(string); ok {
				helper.AssertEqual("Hello", str)
			}
		})

		t.Run("Emoji", func(t *testing.T) {
			testData := `{"emoji": "🎉🚀💻"}`

			result, err := Get(testData, "emoji")
			helper.AssertNoError(err)

			if str, ok := result.(string); ok {
				helper.AssertTrue(len(str) > 0)
			}
		})
	})

	t.Run("ArrayBoundaries", func(t *testing.T) {
		t.Run("SingleElement", func(t *testing.T) {
			testData := `{"single": [1]}`

			first, _ := Get(testData, "single[0]")
			helper.AssertEqual(float64(1), first)

			last, _ := Get(testData, "single[-1]")
			helper.AssertEqual(float64(1), last)
		})

		t.Run("LargeArray", func(t *testing.T) {
			// Generate large array JSON
			elements := make([]string, 1000)
			for i := 0; i < 1000; i++ {
				elements[i] = fmt.Sprint(i)
			}
			testData := `{"large": [` + strings.Join(elements, ",") + `]}`

			result, err := GetArray(testData, "large")
			helper.AssertNoError(err)
			helper.AssertEqual(1000, len(result))
		})

		t.Run("ArraySlicing", func(t *testing.T) {
			testData := `{"arr": [0, 1, 2, 3, 4, 5, 6, 7, 8, 9]}`

			tests := []struct {
				path     string
				expected []interface{}
			}{
				{"arr[0:3]", []interface{}{float64(0), float64(1), float64(2)}},
				{"arr[5:]", []interface{}{float64(5), float64(6), float64(7), float64(8), float64(9)}},
				{"arr[:3]", []interface{}{float64(0), float64(1), float64(2)}},
				{"arr[-3:]", []interface{}{float64(7), float64(8), float64(9)}},
			}

			for _, tt := range tests {
				t.Run(tt.path, func(t *testing.T) {
					result, err := Get(testData, tt.path)
					helper.AssertNoError(err)

					if arr, ok := result.([]interface{}); ok {
						helper.AssertEqual(len(tt.expected), len(arr))
						for i, exp := range tt.expected {
							helper.AssertEqual(exp, arr[i])
						}
					}
				})
			}
		})
	})

	t.Run("ObjectBoundaries", func(t *testing.T) {
		t.Run("ManyKeys", func(t *testing.T) {
			// Generate object with many keys
			pairs := make([]string, 100)
			for i := 0; i < 100; i++ {
				pairs[i] = `"key` + fmt.Sprint(i) + `": ` + fmt.Sprint(i)
			}
			testData := `{"many": {` + strings.Join(pairs, ",") + `}}`

			result, err := GetObject(testData, "many")
			helper.AssertNoError(err)
			helper.AssertEqual(100, len(result))
		})

		t.Run("DeeplyNested", func(t *testing.T) {
			// Generate deeply nested structure
			testData := `{"root": {"level1": {"level2": {"level3": {"level4": {"deep": "value"}}}}}}`

			result, err := Get(testData, "root.level1.level2.level3.level4.deep")
			helper.AssertNoError(err)
			helper.AssertEqual("value", result)
		})

		t.Run("MixedTypes", func(t *testing.T) {
			testData := `{
				"string": "text",
				"number": 42,
				"float": 3.14,
				"bool": true,
				"null": null,
				"array": [1, 2, 3],
				"object": {"nested": "value"}
			}`

			// Verify all types are accessible
			_, err := Get(testData, "string")
			helper.AssertNoError(err)

			_, err = Get(testData, "number")
			helper.AssertNoError(err)

			_, err = Get(testData, "float")
			helper.AssertNoError(err)

			_, err = Get(testData, "bool")
			helper.AssertNoError(err)

			_, err = Get(testData, "null")
			helper.AssertNoError(err)

			_, err = Get(testData, "array")
			helper.AssertNoError(err)

			_, err = Get(testData, "object")
			helper.AssertNoError(err)
		})
	})

	t.Run("WhitespaceHandling", func(t *testing.T) {
		t.Run("LotsOfWhitespace", func(t *testing.T) {
			whitespaceJSON := `{` + strings.Repeat(" \t\n", 1000) + `"key"` + strings.Repeat(" \t\n", 1000) + `:` + strings.Repeat(" \t\n", 1000) + `"value"` + strings.Repeat(" \t\n", 1000) + `}`

			result, err := Get(whitespaceJSON, "key")
			helper.AssertNoError(err)
			helper.AssertEqual("value", result)
		})

		t.Run("NoWhitespace", func(t *testing.T) {
			noSpaceJSON := `{"key":"value"}`
			result, err := Get(noSpaceJSON, "key")
			helper.AssertNoError(err)
			helper.AssertEqual("value", result)
		})
	})

	t.Run("BooleanBoundaries", func(t *testing.T) {
		testData := `{"true": true, "false": false}`

		trueVal, err := GetBool(testData, "true")
		helper.AssertNoError(err)
		helper.AssertTrue(trueVal)

		falseVal, err := GetBool(testData, "false")
		helper.AssertNoError(err)
		helper.AssertFalse(falseVal)

		// Test conversion from various values
		conversionTests := []struct {
			value    interface{}
			expected bool
		}{
			{1, true},
			{0, false},
			{-1, true},
			{1.0, true},
			{0.0, false},
			{"true", true},
			{"false", false},
			{"1", true},
			{"0", false},
		}

		for _, tt := range conversionTests {
			name := "true"
			if !tt.expected {
				name = "false"
			}
			t.Run("Convert_"+name, func(t *testing.T) {
				result, ok := ConvertToBool(tt.value)
				helper.AssertTrue(ok)
				helper.AssertEqual(tt.expected, result)
			})
		}
	})

	t.Run("TimestampEdgeCases", func(t *testing.T) {
		testData := `{"timestamp": "2024-01-15T10:30:00Z", "epoch": 1705319400}`

		timestamp, err := GetString(testData, "timestamp")
		helper.AssertNoError(err)
		helper.AssertEqual("2024-01-15T10:30:00Z", timestamp)

		epoch, err := GetInt(testData, "epoch")
		helper.AssertNoError(err)
		helper.AssertEqual(1705319400, epoch)
	})
}

// TestNullAndMissingFields tests null value handling and missing fields
func TestNullAndMissingFields(t *testing.T) {
	helper := NewTestHelper(t)

	testData := `{
		"null_field": null,
		"string_field": "value",
		"nested": {
			"null_nested": null,
			"valid_nested": "data"
		},
		"array_with_nulls": [1, null, 3, null, 5]
	}`

	t.Run("NullFieldAccess", func(t *testing.T) {
		result, err := Get(testData, "null_field")
		helper.AssertNoError(err)
		helper.AssertNil(result)
	})

	t.Run("NestedNullAccess", func(t *testing.T) {
		result, err := Get(testData, "nested.null_nested")
		helper.AssertNoError(err)
		helper.AssertNil(result)
	})

	t.Run("ArrayNullAccess", func(t *testing.T) {
		result, err := Get(testData, "array_with_nulls[1]")
		helper.AssertNoError(err)
		helper.AssertNil(result)
	})

	t.Run("NullWithDefault", func(t *testing.T) {
		// Null field should return default
		result := GetWithDefault(testData, "null_field", "default")
		helper.AssertEqual("default", result)

		// Missing field should return default
		result = GetWithDefault(testData, "missing_field", "default")
		helper.AssertEqual("default", result)

		// Valid field should return actual value
		result = GetWithDefault(testData, "string_field", "default")
		helper.AssertEqual("value", result)
	})

	t.Run("IsNullMethod", func(t *testing.T) {
		// Iterate through data to test IsNull
		Foreach(testData, func(key any, item *IterableValue) {
			if key == "null_field" {
				helper.AssertTrue(item.IsNull(""))
			}
		})
	})
}

// TestTypeConversionBoundaryCases tests type conversion edge cases
func TestTypeConversionBoundaryCases(t *testing.T) {
	helper := NewTestHelper(t)

	t.Run("IntConversion", func(t *testing.T) {
		tests := []struct {
			input    interface{}
			expected int
			ok       bool
		}{
			{int(42), 42, true},
			{int8(42), 42, true},
			{int16(42), 42, true},
			{int32(42), 42, true},
			{int64(42), 42, true},
			{uint(42), 42, true},
			{uint8(42), 42, true},
			{uint16(42), 42, true},
			{uint32(42), 42, true},
			{uint64(42), 42, true},
			{float32(42.0), 42, true},
			{float64(42.0), 42, true},
			{"42", 42, true},
			{true, 1, true},
			{false, 0, true},
			{"not a number", 0, false},
			{3.14, 0, false}, // Non-integer float
			{nil, 0, false},
		}

		for _, tt := range tests {
			t.Run("Input_"+string(rune(tt.expected)), func(t *testing.T) {
				result, ok := ConvertToInt(tt.input)
				helper.AssertEqual(tt.ok, ok)
				if tt.ok {
					helper.AssertEqual(tt.expected, result)
				}
			})
		}
	})

	t.Run("FloatConversion", func(t *testing.T) {
		tests := []struct {
			input    interface{}
			expected float64
			ok       bool
		}{
			{float64(3.14), 3.14, true},
			{float32(3.14), 3.14, true},
			{int(42), 42.0, true},
			{int64(42), 42.0, true},
			{uint(42), 42.0, true},
			{"3.14", 3.14, true},
			{true, 1.0, true},
			{false, 0.0, true},
			{"not a number", 0.0, false},
			{nil, 0.0, false},
		}

		for _, tt := range tests {
			t.Run("Input_"+string(rune(tt.expected)), func(t *testing.T) {
				result, ok := ConvertToFloat64(tt.input)
				helper.AssertEqual(tt.ok, ok)
				if tt.ok {
					// Use approximate comparison for floats
					diff := tt.expected - result
					if diff < 0 {
						diff = -diff
					}
					helper.AssertTrue(diff < 0.0001, "Float values should be approximately equal")
				}
			})
		}
	})

	t.Run("StringConversion", func(t *testing.T) {
		tests := []struct {
			input    interface{}
			expected string
		}{
			{"hello", "hello"},
			{int(42), "42"},
			{float64(3.14), "3.14"},
			{true, "true"},
			{false, "false"},
			{[]byte("bytes"), "bytes"},
		}

		for _, tt := range tests {
			t.Run("Input_"+tt.expected[:2], func(t *testing.T) {
				result := ConvertToString(tt.input)
				helper.AssertEqual(tt.expected, result)
			})
		}
	})
}
