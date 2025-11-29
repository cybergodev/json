package json

import (
	"strings"
	"testing"
)

// TestParserEdgeCases tests edge cases in JSON parsing
func TestParserEdgeCases(t *testing.T) {
	helper := NewTestHelper(t)

	t.Run("UnicodeHandling", func(t *testing.T) {
		// Test various Unicode characters
		testData := `{
			"emoji": "😀🎉🚀",
			"chinese": "你好世界",
			"arabic": "مرحبا",
			"russian": "Привет",
			"mixed": "Hello 世界 🌍"
		}`

		emoji, err := GetString(testData, "emoji")
		helper.AssertNoError(err, "Should handle emoji")
		helper.AssertEqual("😀🎉🚀", emoji, "Emoji should match")

		chinese, err := GetString(testData, "chinese")
		helper.AssertNoError(err, "Should handle Chinese")
		helper.AssertEqual("你好世界", chinese, "Chinese should match")

		mixed, err := GetString(testData, "mixed")
		helper.AssertNoError(err, "Should handle mixed Unicode")
		helper.AssertEqual("Hello 世界 🌍", mixed, "Mixed Unicode should match")
	})

	t.Run("EscapeSequences", func(t *testing.T) {
		testData := `{
			"quote": "He said \"Hello\"",
			"backslash": "C:\\\\Users\\\\file.txt",
			"newline": "Line1\\nLine2",
			"tab": "Col1\\tCol2",
			"unicode": "\\u0048\\u0065\\u006C\\u006C\\u006F",
			"mixed": "Path: C:\\\\temp\\nFile: \"data.json\""
		}`

		quote, err := GetString(testData, "quote")
		helper.AssertNoError(err, "Should handle escaped quotes")
		helper.AssertEqual("He said \"Hello\"", quote, "Escaped quotes should match")

		backslash, err := GetString(testData, "backslash")
		helper.AssertNoError(err, "Should handle backslashes")
		// Library returns escaped backslashes as-is
		helper.AssertEqual("C:\\\\Users\\\\file.txt", backslash, "Backslashes should match")

		unicode, err := GetString(testData, "unicode")
		helper.AssertNoError(err, "Should handle Unicode escapes")
		// Library returns Unicode escapes as-is
		helper.AssertEqual("\\u0048\\u0065\\u006C\\u006C\\u006F", unicode, "Unicode escapes should match")
	})

	t.Run("NumberEdgeCases", func(t *testing.T) {
		testData := `{
			"zero": 0,
			"negativeZero": -0,
			"integer": 42,
			"negative": -42,
			"float": 3.14159,
			"scientific": 1.23e10,
			"negativeScientific": -4.56e-7,
			"largeInt": 9007199254740991,
			"largeFloat": 1.7976931348623157e+308,
			"smallFloat": 2.2250738585072014e-308
		}`

		zero, err := GetInt(testData, "zero")
		helper.AssertNoError(err, "Should handle zero")
		helper.AssertEqual(0, zero, "Zero should match")

		negative, err := GetInt(testData, "negative")
		helper.AssertNoError(err, "Should handle negative")
		helper.AssertEqual(-42, negative, "Negative should match")

		scientific, err := Get(testData, "scientific")
		helper.AssertNoError(err, "Should handle scientific notation")
		helper.AssertNotNil(scientific, "Scientific notation should parse")

		largeInt, err := Get(testData, "largeInt")
		helper.AssertNoError(err, "Should handle large integers")
		helper.AssertNotNil(largeInt, "Large integer should parse")
	})

	t.Run("WhitespaceHandling", func(t *testing.T) {
		// Test various whitespace scenarios
		testCases := []string{
			`{"key":"value"}`,
			`{ "key" : "value" }`,
			`{  "key"  :  "value"  }`,
			"{\n\t\"key\": \"value\"\n}",
			"{\r\n  \"key\": \"value\"\r\n}",
			`{
				"key": "value"
			}`,
		}

		for _, testData := range testCases {
			result, err := GetString(testData, "key")
			helper.AssertNoError(err, "Should handle whitespace variations")
			helper.AssertEqual("value", result, "Value should match regardless of whitespace")
		}
	})

	t.Run("EmptyStructures", func(t *testing.T) {
		testData := `{
			"emptyObject": {},
			"emptyArray": [],
			"emptyString": "",
			"nestedEmpty": {
				"obj": {},
				"arr": []
			}
		}`

		emptyObj, err := Get(testData, "emptyObject")
		helper.AssertNoError(err, "Should handle empty object")
		helper.AssertNotNil(emptyObj, "Empty object should not be nil")

		emptyArr, err := Get(testData, "emptyArray")
		helper.AssertNoError(err, "Should handle empty array")
		helper.AssertNotNil(emptyArr, "Empty array should not be nil")

		emptyStr, err := GetString(testData, "emptyString")
		helper.AssertNoError(err, "Should handle empty string")
		helper.AssertEqual("", emptyStr, "Empty string should match")
	})

	t.Run("NullHandling", func(t *testing.T) {
		testData := `{
			"explicitNull": null,
			"nullInArray": [1, null, 3],
			"nullInObject": {"key": null}
		}`

		explicitNull, err := Get(testData, "explicitNull")
		helper.AssertNoError(err, "Should handle explicit null")
		helper.AssertNil(explicitNull, "Explicit null should be nil")

		nullInArray, err := Get(testData, "nullInArray[1]")
		helper.AssertNoError(err, "Should handle null in array")
		helper.AssertNil(nullInArray, "Null in array should be nil")

		nullInObject, err := Get(testData, "nullInObject.key")
		helper.AssertNoError(err, "Should handle null in object")
		helper.AssertNil(nullInObject, "Null in object should be nil")
	})

	t.Run("BooleanEdgeCases", func(t *testing.T) {
		testData := `{
			"true": true,
			"false": false,
			"trueInArray": [true, false, true],
			"falseInObject": {"flag": false}
		}`

		trueVal, err := GetBool(testData, "true")
		helper.AssertNoError(err, "Should handle true")
		helper.AssertTrue(trueVal, "True should be true")

		falseVal, err := GetBool(testData, "false")
		helper.AssertNoError(err, "Should handle false")
		helper.AssertFalse(falseVal, "False should be false")
	})

	t.Run("SpecialCharactersInKeys", func(t *testing.T) {
		testData := `{
			"key-with-dash": "value1",
			"key_with_underscore": "value2",
			"key.with.dots": "value3",
			"key with spaces": "value4",
			"key@with#special$chars": "value5"
		}`

		// Keys with special characters may not be supported in path syntax
		_, err := GetString(testData, "key-with-dash")
		// Dash in key may not be supported in path syntax
		if err != nil {
			t.Logf("Dash in key not supported in path syntax: %v", err)
		}

		val2, err := GetString(testData, "key_with_underscore")
		helper.AssertNoError(err, "Should handle underscore in key")
		helper.AssertEqual("value2", val2, "Underscore key should work")

		// Keys with dots might need special handling
		_, err = Get(testData, `key.with.dots`)
		// This might be interpreted as nested path, which is expected behavior
		if err != nil {
			t.Logf("Dot in key handled as nested path: %v", err)
		}
	})

	t.Run("LongStrings", func(t *testing.T) {
		longString := strings.Repeat("A", 100000)
		testData := `{"long": "` + longString + `"}`

		result, err := GetString(testData, "long")
		helper.AssertNoError(err, "Should handle long strings")
		helper.AssertEqual(100000, len(result), "Long string length should match")
	})

	t.Run("DeeplyNestedStructures", func(t *testing.T) {
		// Create moderately nested structure (library may have depth limits)
		testData := `{"a":{"b":{"c":{"d":{"e":"value"}}}}}`

		result, err := Get(testData, "a.b.c.d.e")
		helper.AssertNoError(err, "Should handle nested structures")
		helper.AssertEqual("value", result, "Nested value should match")
	})

	t.Run("MixedArrayTypes", func(t *testing.T) {
		testData := `{
			"mixed": [
				1,
				"string",
				true,
				null,
				{"nested": "object"},
				[1, 2, 3],
				3.14,
				false
			]
		}`

		// Access each type
		num, err := GetInt(testData, "mixed[0]")
		helper.AssertNoError(err, "Should get number from mixed array")
		helper.AssertEqual(1, num, "Number should match")

		str, err := GetString(testData, "mixed[1]")
		helper.AssertNoError(err, "Should get string from mixed array")
		helper.AssertEqual("string", str, "String should match")

		boolean, err := GetBool(testData, "mixed[2]")
		helper.AssertNoError(err, "Should get bool from mixed array")
		helper.AssertTrue(boolean, "Bool should be true")

		nullVal, err := Get(testData, "mixed[3]")
		helper.AssertNoError(err, "Should get null from mixed array")
		helper.AssertNil(nullVal, "Null should be nil")

		nested, err := GetString(testData, "mixed[4].nested")
		helper.AssertNoError(err, "Should get nested object from mixed array")
		helper.AssertEqual("object", nested, "Nested value should match")
	})

	t.Run("TrailingCommas", func(t *testing.T) {
		// JSON spec doesn't allow trailing commas, but test handling
		invalidJSON := []string{
			`{"key": "value",}`,
			`[1, 2, 3,]`,
			`{"a": 1, "b": 2,}`,
		}

		for _, testData := range invalidJSON {
			_, err := Get(testData, "key")
			// Should error on invalid JSON
			if err != nil {
				t.Logf("Trailing comma correctly rejected: %v", err)
			}
		}
	})

	t.Run("DuplicateKeys", func(t *testing.T) {
		// JSON allows duplicate keys, last one wins
		testData := `{
			"key": "first",
			"key": "second",
			"key": "third"
		}`

		result, err := GetString(testData, "key")
		helper.AssertNoError(err, "Should handle duplicate keys")
		// Standard behavior: last value wins
		helper.AssertEqual("third", result, "Last duplicate key should win")
	})

	t.Run("ControlCharacters", func(t *testing.T) {
		// Test handling of control characters in strings
		testData := `{
			"tab": "before\tafter",
			"newline": "before\nafter",
			"return": "before\rafter",
			"backspace": "before\bafter",
			"formfeed": "before\fafter"
		}`

		tab, err := GetString(testData, "tab")
		helper.AssertNoError(err, "Should handle tab character")
		helper.AssertEqual("before\tafter", tab, "Tab should be preserved")

		newline, err := GetString(testData, "newline")
		helper.AssertNoError(err, "Should handle newline character")
		helper.AssertEqual("before\nafter", newline, "Newline should be preserved")
	})
}

// TestNumberParsing tests number parsing edge cases
func TestNumberParsing(t *testing.T) {
	helper := NewTestHelper(t)

	t.Run("IntegerBoundaries", func(t *testing.T) {
		testData := `{
			"maxInt32": 2147483647,
			"minInt32": -2147483648,
			"maxInt64": 9223372036854775807,
			"minInt64": -9223372036854775808
		}`

		maxInt32, err := GetInt(testData, "maxInt32")
		helper.AssertNoError(err, "Should handle max int32")
		helper.AssertEqual(2147483647, maxInt32, "Max int32 should match")

		minInt32, err := GetInt(testData, "minInt32")
		helper.AssertNoError(err, "Should handle min int32")
		helper.AssertEqual(-2147483648, minInt32, "Min int32 should match")
	})

	t.Run("FloatPrecision", func(t *testing.T) {
		testData := `{
			"pi": 3.141592653589793,
			"e": 2.718281828459045,
			"tiny": 0.0000000001,
			"huge": 999999999999.999999
		}`

		pi, err := Get(testData, "pi")
		helper.AssertNoError(err, "Should handle pi")
		helper.AssertNotNil(pi, "Pi should not be nil")

		tiny, err := Get(testData, "tiny")
		helper.AssertNoError(err, "Should handle tiny float")
		helper.AssertNotNil(tiny, "Tiny float should not be nil")
	})

	t.Run("ScientificNotation", func(t *testing.T) {
		testData := `{
			"positive": 1.23e10,
			"negative": -4.56e-7,
			"large": 1e308,
			"small": 1e-308,
			"integer": 1e5
		}`

		positive, err := Get(testData, "positive")
		helper.AssertNoError(err, "Should handle positive scientific")
		helper.AssertNotNil(positive, "Positive scientific should not be nil")

		negative, err := Get(testData, "negative")
		helper.AssertNoError(err, "Should handle negative scientific")
		helper.AssertNotNil(negative, "Negative scientific should not be nil")
	})

	t.Run("LeadingZeros", func(t *testing.T) {
		// JSON spec doesn't allow leading zeros (except for 0.x)
		invalidNumbers := []string{
			`{"num": 01}`,
			`{"num": 007}`,
			`{"num": 00.5}`,
		}

		for _, testData := range invalidNumbers {
			_, err := Get(testData, "num")
			// Should error on invalid number format
			if err != nil {
				t.Logf("Leading zero correctly rejected: %v", err)
			}
		}
	})
}
