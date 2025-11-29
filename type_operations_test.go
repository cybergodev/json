package json

import (
	"encoding/json"
	"testing"
)

// TestTypeOperations provides comprehensive testing for type conversion and type safety
// Merged from: type_conversion_test.go, type_safety_test.go, and parts of core_functionality_test.go
func TestTypeOperations(t *testing.T) {
	helper := NewTestHelper(t)

	t.Run("TypeConversion", func(t *testing.T) {
		t.Run("ConvertToInt", func(t *testing.T) {
			tests := []struct {
				name     string
				input    any
				expected int
				success  bool
			}{
				{"int", 42, 42, true},
				{"int8", int8(10), 10, true},
				{"int16", int16(100), 100, true},
				{"int32", int32(1000), 1000, true},
				{"int64", int64(10000), 10000, true},
				{"uint8", uint8(5), 5, true},
				{"uint16", uint16(50), 50, true},
				{"float64_whole", 42.0, 42, true},
				{"float64_decimal", 42.5, 0, false},
				{"string_valid", "123", 123, true},
				{"string_invalid", "abc", 0, false},
				{"bool_true", true, 1, true},
				{"bool_false", false, 0, true},
				{"json_number", json.Number("456"), 456, true},
				{"overflow_int64", int64(9999999999), 0, false},
			}

			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					result, ok := ConvertToInt(tt.input)
					helper.AssertEqual(tt.success, ok)
					if ok {
						helper.AssertEqual(tt.expected, result)
					}
				})
			}
		})

		t.Run("ConvertToInt64", func(t *testing.T) {
			tests := []struct {
				name     string
				input    any
				expected int64
				success  bool
			}{
				{"int", 42, 42, true},
				{"int64_max", int64(9223372036854775807), 9223372036854775807, true},
				{"float64_whole", 42.0, 42, true},
				{"float64_decimal", 42.5, 0, false},
				{"string_valid", "123456789", 123456789, true},
				{"string_invalid", "not_a_number", 0, false},
				{"bool_true", true, 1, true},
				{"bool_false", false, 0, true},
				{"json_number", json.Number("789"), 789, true},
			}

			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					result, ok := ConvertToInt64(tt.input)
					helper.AssertEqual(tt.success, ok)
					if ok {
						helper.AssertEqual(tt.expected, result)
					}
				})
			}
		})

		t.Run("ConvertToUint64", func(t *testing.T) {
			tests := []struct {
				name     string
				input    any
				expected uint64
				success  bool
			}{
				{"uint", uint(42), 42, true},
				{"uint64_max", uint64(18446744073709551615), 18446744073709551615, true},
				{"int_positive", 100, 100, true},
				{"int_negative", -10, 0, false},
				{"float64_positive", 42.0, 42, true},
				{"float64_negative", -42.0, 0, false},
				{"string_valid", "12345", 12345, true},
				{"string_invalid", "abc", 0, false},
				{"bool_true", true, 1, true},
				{"bool_false", false, 0, true},
			}

			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					result, ok := ConvertToUint64(tt.input)
					helper.AssertEqual(tt.success, ok)
					if ok {
						helper.AssertEqual(tt.expected, result)
					}
				})
			}
		})

		t.Run("ConvertToFloat64", func(t *testing.T) {
			tests := []struct {
				name     string
				input    any
				expected float64
				success  bool
			}{
				{"float64", 42.5, 42.5, true},
				{"float32", float32(3.14), float64(float32(3.14)), true},
				{"int", 42, 42.0, true},
				{"int64", int64(100), 100.0, true},
				{"uint64", uint64(200), 200.0, true},
				{"string_valid", "3.14159", 3.14159, true},
				{"string_invalid", "not_a_float", 0, false},
				{"bool_true", true, 1.0, true},
				{"bool_false", false, 0.0, true},
				{"json_number", json.Number("2.718"), 2.718, true},
			}

			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					result, ok := ConvertToFloat64(tt.input)
					helper.AssertEqual(tt.success, ok)
					if ok {
						helper.AssertEqual(tt.expected, result)
					}
				})
			}
		})

		t.Run("ConvertToString", func(t *testing.T) {
			tests := []struct {
				name     string
				input    any
				expected string
				success  bool
			}{
				{"string", "hello", "hello", true},
				{"int", 42, "42", true},
				{"int64", int64(123), "123", true},
				{"float64", 3.14, "3.14", true},
				{"bool_true", true, "true", true},
				{"bool_false", false, "false", true},
				{"json_number", json.Number("456"), "456", true},
			}

			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					result, ok := ConvertToString(tt.input)
					helper.AssertEqual(tt.success, ok)
					if ok {
						helper.AssertEqual(tt.expected, result)
					}
				})
			}
		})

		t.Run("ConvertToBool", func(t *testing.T) {
			tests := []struct {
				name     string
				input    any
				expected bool
				success  bool
			}{
				{"bool_true", true, true, true},
				{"bool_false", false, false, true},
				{"int_nonzero", 42, true, true},
				{"int_zero", 0, false, true},
				{"int64_nonzero", int64(100), true, true},
				{"int64_zero", int64(0), false, true},
				{"float64_nonzero", 3.14, true, true},
				{"float64_zero", 0.0, false, true},
				{"string_true", "true", true, true},
				{"string_false", "false", false, true},
				{"string_1", "1", true, true},
				{"string_0", "0", false, true},
				{"string_invalid", "maybe", false, false},
			}

			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					result, ok := ConvertToBool(tt.input)
					helper.AssertEqual(tt.success, ok)
					if ok {
						helper.AssertEqual(tt.expected, result)
					}
				})
			}
		})

		t.Run("SafeConversions", func(t *testing.T) {
			result, err := SafeConvertToInt64(42)
			helper.AssertNoError(err)
			helper.AssertEqual(int64(42), result)

			_, err = SafeConvertToInt64("invalid")
			helper.AssertError(err)

			result2, err := SafeConvertToUint64(uint(42))
			helper.AssertNoError(err)
			helper.AssertEqual(uint64(42), result2)

			_, err = SafeConvertToUint64(-10)
			helper.AssertError(err)
		})

		t.Run("FormatNumber", func(t *testing.T) {
			tests := []struct {
				name     string
				input    any
				expected string
			}{
				{"int", 42, "42"},
				{"int64", int64(123), "123"},
				{"float64", 3.14, "3.14"},
				{"json_number", json.Number("456.789"), "456.789"},
				{"string", "already_string", "already_string"},
			}

			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					result := FormatNumber(tt.input)
					helper.AssertEqual(tt.expected, result)
				})
			}
		})
	})

	t.Run("TypeSafety", func(t *testing.T) {
		testJSON := `{
			"string": "hello world",
			"emptyString": "",
			"number": 42,
			"float": 3.14159,
			"largeNumber": 9223372036854775807,
			"boolean": true,
			"falseBool": false,
			"null": null,
			"array": [1, 2, 3, 4, 5],
			"emptyArray": [],
			"stringArray": ["apple", "banana", "cherry"],
			"object": {
				"nested": "value",
				"count": 10
			},
			"emptyObject": {}
		}`

		t.Run("BasicTypedOperations", func(t *testing.T) {
			str, err := GetTyped[string](testJSON, "string")
			helper.AssertNoError(err)
			helper.AssertEqual("hello world", str)

			num, err := GetTyped[int](testJSON, "number")
			helper.AssertNoError(err)
			helper.AssertEqual(42, num)

			flt, err := GetTyped[float64](testJSON, "float")
			helper.AssertNoError(err)
			helper.AssertEqual(3.14159, flt)

			boolean, err := GetTyped[bool](testJSON, "boolean")
			helper.AssertNoError(err)
			helper.AssertEqual(true, boolean)
		})

		t.Run("ArrayTypedOperations", func(t *testing.T) {
			intArray, err := GetTyped[[]int](testJSON, "array")
			helper.AssertNoError(err)
			helper.AssertEqual(5, len(intArray))
			helper.AssertEqual(1, intArray[0])

			stringArray, err := GetTyped[[]string](testJSON, "stringArray")
			helper.AssertNoError(err)
			helper.AssertEqual(3, len(stringArray))
			helper.AssertEqual("apple", stringArray[0])

			emptyArray, err := GetTyped[[]int](testJSON, "emptyArray")
			helper.AssertNoError(err)
			helper.AssertEqual(0, len(emptyArray))
		})

		t.Run("ObjectTypedOperations", func(t *testing.T) {
			obj, err := GetTyped[map[string]any](testJSON, "object")
			helper.AssertNoError(err)
			helper.AssertEqual("value", obj["nested"])
			helper.AssertEqual(float64(10), obj["count"])

			emptyObj, err := GetTyped[map[string]any](testJSON, "emptyObject")
			helper.AssertNoError(err)
			helper.AssertEqual(0, len(emptyObj))
		})

		t.Run("TypeMismatchHandling", func(t *testing.T) {
			_, err := GetTyped[int](testJSON, "string")
			helper.AssertError(err)

			_, err = GetTyped[bool](testJSON, "array")
			helper.AssertError(err)

			_, err = GetTyped[[]int](testJSON, "object")
			helper.AssertError(err)

			_, err = GetTyped[map[string]any](testJSON, "array")
			helper.AssertError(err)
		})

		t.Run("EdgeCaseTypes", func(t *testing.T) {
			largeNum, err := GetTyped[int64](testJSON, "largeNumber")
			if err == nil {
				helper.AssertEqual(int64(9223372036854775807), largeNum)
			}

			float32Val, err := GetTyped[float32](testJSON, "float")
			helper.AssertNoError(err)
			helper.AssertTrue(float32Val > 3.14 && float32Val < 3.15)

			uintVal, err := GetTyped[uint](testJSON, "number")
			helper.AssertNoError(err)
			helper.AssertEqual(uint(42), uintVal)
		})

		t.Run("CustomStructTypes", func(t *testing.T) {
			type Person struct {
				Name  string `json:"name"`
				Age   int    `json:"age"`
				Email string `json:"email"`
			}

			customJSON := `{
				"person": {
					"name": "John Doe",
					"age": 30,
					"email": "john@example.com"
				}
			}`

			person, err := GetTyped[Person](customJSON, "person")
			helper.AssertNoError(err)
			helper.AssertEqual("John Doe", person.Name)
			helper.AssertEqual(30, person.Age)
		})
	})
}
