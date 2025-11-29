package json

import (
	"testing"
)

// TestCore consolidates basic JSON operations (Get/Set/Delete) and type conversions
// Replaces: operations_test.go, type_operations_test.go
func TestCore(t *testing.T) {
	helper := NewTestHelper(t)

	t.Run("BasicGet", func(t *testing.T) {
		testData := `{
			"string": "hello",
			"number": 42,
			"float": 3.14,
			"boolean": true,
			"null": null,
			"empty": "",
			"nested": {"deep": {"value": "found"}},
			"array": [1, 2, 3, 4, 5],
			"unicode": "你好世界🌍"
		}`

		tests := []struct {
			name     string
			path     string
			expected interface{}
			wantErr  bool
		}{
			{"String", "string", "hello", false},
			{"Number", "number", float64(42), false},
			{"Float", "float", 3.14, false},
			{"Boolean", "boolean", true, false},
			{"Null", "null", nil, false},
			{"EmptyString", "empty", "", false},
			{"DeepNested", "nested.deep.value", "found", false},
			{"ArrayIndex", "array[0]", float64(1), false},
			{"ArrayNegative", "array[-1]", float64(5), false},
			{"Unicode", "unicode", "你好世界🌍", false},
			{"NonExistent", "missing", nil, true},
			{"ArrayOutOfBounds", "array[100]", nil, false},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				result, err := Get(testData, tt.path)
				if tt.wantErr {
					helper.AssertError(err)
				} else {
					helper.AssertNoError(err)
					helper.AssertEqual(tt.expected, result)
				}
			})
		}
	})

	t.Run("BasicSet", func(t *testing.T) {
		tests := []struct {
			name      string
			json      string
			path      string
			value     interface{}
			checkPath string
			expected  interface{}
		}{
			{"SetString", `{}`, "name", "John", "name", "John"},
			{"SetNumber", `{}`, "age", 30, "age", float64(30)},
			{"SetBoolean", `{}`, "active", true, "active", true},
			{"SetNull", `{}`, "data", nil, "data", nil},
			{"Overwrite", `{"name":"old"}`, "name", "new", "name", "new"},
			{"SetNested", `{"user":{}}`, "user.name", "Alice", "user.name", "Alice"},
			{"SetArrayElement", `{"arr":[1,2,3]}`, "arr[1]", 99, "arr[1]", float64(99)},
			{"SetArrayNegative", `{"arr":[1,2,3]}`, "arr[-1]", 99, "arr[-1]", float64(99)},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				result, err := Set(tt.json, tt.path, tt.value)
				helper.AssertNoError(err)
				checkVal, err := Get(result, tt.checkPath)
				helper.AssertNoError(err)
				helper.AssertEqual(tt.expected, checkVal)
			})
		}
	})

	t.Run("BasicDelete", func(t *testing.T) {
		tests := []struct {
			name        string
			json        string
			path        string
			checkPath   string
			shouldExist bool
			wantErr     bool
		}{
			{"DeleteSimple", `{"name":"John","age":30}`, "name", "name", false, false},
			{"DeleteNested", `{"user":{"name":"Alice","age":25}}`, "user.name", "user.name", false, false},
			{"DeleteArrayElement", `{"arr":[1,2,3,4,5]}`, "arr[2]", "arr[2]", true, false},
			{"DeleteNonExistent", `{"name":"John"}`, "missing", "missing", false, true},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				result, err := Delete(tt.json, tt.path)
				if tt.wantErr {
					helper.AssertError(err)
				} else {
					helper.AssertNoError(err)
					checkVal, _ := Get(result, tt.checkPath)
					if tt.shouldExist {
						helper.AssertNotNil(checkVal)
					} else {
						helper.AssertNil(checkVal)
					}
				}
			})
		}
	})

	t.Run("TypeConversion", func(t *testing.T) {
		t.Run("ToInt", func(t *testing.T) {
			tests := []struct {
				input   any
				want    int
				success bool
			}{
				{42, 42, true},
				{int64(100), 100, true},
				{42.0, 42, true},
				{42.5, 0, false},
				{"123", 123, true},
				{"abc", 0, false},
				{true, 1, true},
				{false, 0, true},
			}

			for _, tt := range tests {
				result, ok := ConvertToInt(tt.input)
				helper.AssertEqual(tt.success, ok)
				if ok {
					helper.AssertEqual(tt.want, result)
				}
			}
		})

		t.Run("ToFloat64", func(t *testing.T) {
			tests := []struct {
				input   any
				want    float64
				success bool
			}{
				{42.5, 42.5, true},
				{42, 42.0, true},
				{"3.14", 3.14, true},
				{"abc", 0, false},
				{true, 1.0, true},
			}

			for _, tt := range tests {
				result, ok := ConvertToFloat64(tt.input)
				helper.AssertEqual(tt.success, ok)
				if ok {
					helper.AssertEqual(tt.want, result)
				}
			}
		})

		t.Run("ToString", func(t *testing.T) {
			tests := []struct {
				input any
				want  string
			}{
				{"hello", "hello"},
				{42, "42"},
				{3.14, "3.14"},
				{true, "true"},
				{false, "false"},
			}

			for _, tt := range tests {
				result, ok := ConvertToString(tt.input)
				helper.AssertTrue(ok)
				helper.AssertEqual(tt.want, result)
			}
		})

		t.Run("ToBool", func(t *testing.T) {
			tests := []struct {
				input   any
				want    bool
				success bool
			}{
				{true, true, true},
				{false, false, true},
				{1, true, true},
				{0, false, true},
				{"true", true, true},
				{"false", false, true},
				{"invalid", false, false},
			}

			for _, tt := range tests {
				result, ok := ConvertToBool(tt.input)
				helper.AssertEqual(tt.success, ok)
				if ok {
					helper.AssertEqual(tt.want, result)
				}
			}
		})
	})

	t.Run("TypedGet", func(t *testing.T) {
		testJSON := `{
			"string": "hello",
			"number": 42,
			"float": 3.14,
			"boolean": true,
			"array": [1, 2, 3],
			"object": {"key": "value"}
		}`

		str, err := GetTyped[string](testJSON, "string")
		helper.AssertNoError(err)
		helper.AssertEqual("hello", str)

		num, err := GetTyped[int](testJSON, "number")
		helper.AssertNoError(err)
		helper.AssertEqual(42, num)

		flt, err := GetTyped[float64](testJSON, "float")
		helper.AssertNoError(err)
		helper.AssertEqual(3.14, flt)

		boolean, err := GetTyped[bool](testJSON, "boolean")
		helper.AssertNoError(err)
		helper.AssertTrue(boolean)

		arr, err := GetTyped[[]int](testJSON, "array")
		helper.AssertNoError(err)
		helper.AssertEqual(3, len(arr))

		obj, err := GetTyped[map[string]any](testJSON, "object")
		helper.AssertNoError(err)
		helper.AssertEqual("value", obj["key"])
	})
}
