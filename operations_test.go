package json

import (
	"testing"
)

// TestBasicOperations consolidates all basic Get/Set/Delete operations
// Replaces redundant tests from core_functionality_test.go, set_operations_comprehensive_test.go, delete_operations_comprehensive_test.go
func TestBasicOperations(t *testing.T) {
	helper := NewTestHelper(t)

	t.Run("Get", func(t *testing.T) {
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

	t.Run("Set", func(t *testing.T) {
		tests := []struct {
			name      string
			json      string
			path      string
			value     interface{}
			checkPath string
			expected  interface{}
			wantErr   bool
		}{
			{"SetString", `{}`, "name", "John", "name", "John", false},
			{"SetNumber", `{}`, "age", 30, "age", float64(30), false},
			{"SetBoolean", `{}`, "active", true, "active", true, false},
			{"SetNull", `{}`, "data", nil, "data", nil, false},
			{"Overwrite", `{"name":"old"}`, "name", "new", "name", "new", false},
			{"SetNested", `{"user":{}}`, "user.name", "Alice", "user.name", "Alice", false},
			{"SetArrayElement", `{"arr":[1,2,3]}`, "arr[1]", 99, "arr[1]", float64(99), false},
			{"SetArrayNegative", `{"arr":[1,2,3]}`, "arr[-1]", 99, "arr[-1]", float64(99), false},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				result, err := Set(tt.json, tt.path, tt.value)
				if tt.wantErr {
					helper.AssertError(err)
				} else {
					helper.AssertNoError(err)
					checkVal, err := Get(result, tt.checkPath)
					helper.AssertNoError(err)
					helper.AssertEqual(tt.expected, checkVal)
				}
			})
		}
	})

	t.Run("Delete", func(t *testing.T) {
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
			{"DeleteArrayElement", `{"arr":[1,2,3,4,5]}`, "arr[2]", "arr[2]", true, false}, // Array shifts, so arr[2] still exists
			{"DeleteNonExistent", `{"name":"John"}`, "missing", "missing", false, true},    // Deleting non-existent should error
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
}
