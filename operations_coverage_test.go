package json

import (
	"strings"
	"testing"
)

// TestOperationsCoverage tests uncovered operation functions
func TestOperationsCoverage(t *testing.T) {
	helper := NewTestHelper(t)
	processor := New(DefaultConfig())
	defer processor.Close()

	t.Run("GetArray", func(t *testing.T) {
		jsonStr := `{"items":[1,2,3]}`
		arr, err := GetArray(jsonStr, "items")
		helper.AssertNoError(err, "GetArray should not error")
		helper.AssertEqual(len(arr), 3, "Array length should be 3")
	})

	t.Run("GetObject", func(t *testing.T) {
		jsonStr := `{"user":{"name":"test"}}`
		obj, err := GetObject(jsonStr, "user")
		helper.AssertNoError(err, "GetObject should not error")
		helper.AssertNotNil(obj, "Object should not be nil")
	})

	t.Run("GetWithDefault", func(t *testing.T) {
		jsonStr := `{"value":"test"}`
		val := GetWithDefault(jsonStr, "value", "default")
		helper.AssertEqual(val, "test", "Should get value")

		val = GetWithDefault(jsonStr, "missing", "default")
		helper.AssertEqual(val, "default", "Should get default")
	})

	t.Run("GetTypedWithDefault", func(t *testing.T) {
		jsonStr := `{"value":42}`
		val := GetTypedWithDefault[int](jsonStr, "value", 0)
		helper.AssertEqual(val, 42, "Should get typed value")

		val = GetTypedWithDefault[int](jsonStr, "missing", 99)
		helper.AssertEqual(val, 99, "Should get default")
	})

	t.Run("GetStringWithDefault", func(t *testing.T) {
		jsonStr := `{"value":"test"}`
		val := GetStringWithDefault(jsonStr, "value", "default")
		helper.AssertEqual(val, "test", "Should get value")

		val = GetStringWithDefault(jsonStr, "missing", "default")
		helper.AssertEqual(val, "default", "Should get default")
	})

	t.Run("GetIntWithDefault", func(t *testing.T) {
		jsonStr := `{"value":42}`
		val := GetIntWithDefault(jsonStr, "value", 0)
		helper.AssertEqual(val, 42, "Should get value")

		val = GetIntWithDefault(jsonStr, "missing", 99)
		helper.AssertEqual(val, 99, "Should get default")
	})

	t.Run("GetFloat64WithDefault", func(t *testing.T) {
		jsonStr := `{"value":3.14}`
		val := GetFloat64WithDefault(jsonStr, "value", 0.0)
		helper.AssertEqual(val, 3.14, "Should get value")

		val = GetFloat64WithDefault(jsonStr, "missing", 9.99)
		helper.AssertEqual(val, 9.99, "Should get default")
	})

	t.Run("GetBoolWithDefault", func(t *testing.T) {
		jsonStr := `{"value":true}`
		val := GetBoolWithDefault(jsonStr, "value", false)
		helper.AssertTrue(val, "Should get value")

		val = GetBoolWithDefault(jsonStr, "missing", true)
		helper.AssertTrue(val, "Should get default")
	})

	t.Run("GetArrayWithDefault", func(t *testing.T) {
		jsonStr := `{"items":[1,2,3]}`
		arr := GetArrayWithDefault(jsonStr, "items", []any{})
		helper.AssertEqual(len(arr), 3, "Should get array")

		arr = GetArrayWithDefault(jsonStr, "missing", []any{99})
		helper.AssertEqual(len(arr), 1, "Should get default")
	})

	t.Run("GetObjectWithDefault", func(t *testing.T) {
		jsonStr := `{"user":{"name":"test"}}`
		obj := GetObjectWithDefault(jsonStr, "user", map[string]any{})
		helper.AssertNotNil(obj, "Should get object")

		obj = GetObjectWithDefault(jsonStr, "missing", map[string]any{"default": true})
		helper.AssertEqual(obj["default"], true, "Should get default")
	})

	t.Run("GetMultiple", func(t *testing.T) {
		jsonStr := `{"name":"test","age":30,"city":"NYC"}`
		paths := []string{"name", "age", "city"}
		results, err := GetMultiple(jsonStr, paths)
		helper.AssertNoError(err, "Should not error")
		helper.AssertEqual(len(results), 3, "Should get all values")
	})

	t.Run("SetMultiple", func(t *testing.T) {
		jsonStr := `{"name":"test"}`
		updates := map[string]any{
			"age":  30,
			"city": "NYC",
		}
		result, err := SetMultiple(jsonStr, updates)
		helper.AssertNoError(err, "Should not error")
		helper.AssertTrue(len(result) > 0, "Result should not be empty")
	})

	t.Run("SetMultipleWithAdd", func(t *testing.T) {
		jsonStr := `{"name":"test"}`
		updates := map[string]any{
			"items[0]": "first",
		}
		result, err := SetMultipleWithAdd(jsonStr, updates)
		helper.AssertNoError(err, "Should not error")
		helper.AssertTrue(len(result) > 0, "Result should not be empty")
	})

	t.Run("DeleteWithCleanNull", func(t *testing.T) {
		jsonStr := `{"name":"test","age":30}`
		result, err := DeleteWithCleanNull(jsonStr, "age")
		helper.AssertNoError(err, "Should not error")
		helper.AssertFalse(strings.Contains(result, "age"), "Should not contain deleted field")
	})

	t.Run("MarshalIndent", func(t *testing.T) {
		data := map[string]any{"name": "test"}
		result, err := MarshalIndent(data, "", "  ")
		helper.AssertNoError(err, "Should not error")
		helper.AssertTrue(len(result) > 0, "Result should not be empty")
	})

	t.Run("EncodePretty", func(t *testing.T) {
		data := map[string]any{"name": "test"}
		result, err := EncodePretty(data)
		helper.AssertNoError(err, "Should not error")
		helper.AssertTrue(strings.Contains(result, "\n"), "Should be pretty printed")
	})

	t.Run("EncodeCompact", func(t *testing.T) {
		data := map[string]any{"name": "test"}
		result, err := EncodeCompact(data)
		helper.AssertNoError(err, "Should not error")
		helper.AssertFalse(strings.Contains(result, "\n"), "Should be compact")
	})

	t.Run("FormatPretty", func(t *testing.T) {
		jsonStr := `{"name":"test"}`
		result, err := FormatPretty(jsonStr)
		helper.AssertNoError(err, "Should not error")
		helper.AssertTrue(strings.Contains(result, "\n"), "Should be pretty printed")
	})

	t.Run("FormatCompact", func(t *testing.T) {
		jsonStr := `{
			"name": "test"
		}`
		result, err := FormatCompact(jsonStr)
		helper.AssertNoError(err, "Should not error")
		helper.AssertFalse(strings.Contains(result, "\n"), "Should be compact")
	})
}
