package json

import (
	"testing"
)

// TestUtilsAndIterator consolidates utility functions and iterator tests
// Replaces: utils_comprehensive_test.go, iterator_comprehensive_test.go, helpers_test.go
func TestUtilsAndIterator(t *testing.T) {
	helper := NewTestHelper(t)

	t.Run("PathValidation", func(t *testing.T) {
		helper.AssertTrue(IsValidPath("user.name"))
		helper.AssertTrue(IsValidPath("items[0]"))
		helper.AssertTrue(IsValidPath("data[1:5]"))
		helper.AssertTrue(IsValidPath("users{name}"))
		helper.AssertTrue(IsValidPath("."))
		helper.AssertFalse(IsValidPath(""))

		err := ValidatePath("user.name")
		helper.AssertNoError(err)

		err = ValidatePath("")
		helper.AssertError(err)
	})

	t.Run("JSONValidation", func(t *testing.T) {
		helper.AssertTrue(IsValidJson(`{"valid": true}`))
		helper.AssertTrue(IsValidJson(`[1, 2, 3]`))
		helper.AssertTrue(IsValidJson(`"string"`))
		helper.AssertTrue(IsValidJson(`123`))
		helper.AssertTrue(IsValidJson(`true`))
		helper.AssertTrue(IsValidJson(`null`))

		helper.AssertFalse(IsValidJson(`{invalid}`))
		helper.AssertFalse(IsValidJson(`[1, 2,]`))
		helper.AssertFalse(IsValidJson(``))
	})

	t.Run("PathTypeDetection", func(t *testing.T) {
		helper.AssertTrue(isJSONPointerPath("/users/0/name"))
		helper.AssertFalse(isJSONPointerPath("users.name"))

		helper.AssertTrue(isDotNotationPath("users.name"))
		helper.AssertFalse(isDotNotationPath("/users/name"))

		helper.AssertTrue(isArrayPath("items[0]"))
		helper.AssertTrue(isArrayPath("users[5].name"))
		helper.AssertFalse(isArrayPath("simple.path"))

		helper.AssertTrue(isSlicePath("items[1:3]"))
		helper.AssertTrue(isSlicePath("items[:5]"))
		helper.AssertFalse(isSlicePath("items[0]"))

		helper.AssertTrue(isExtractionPath("users{name}"))
		helper.AssertFalse(isExtractionPath("items[0]"))
	})

	t.Run("JSONTypeDetection", func(t *testing.T) {
		processor := New()
		defer processor.Close()

		var objData interface{}
		err := processor.Parse(`{"name": "test"}`, &objData)
		helper.AssertNoError(err)
		helper.AssertTrue(isJsonObject(objData))

		var arrData interface{}
		err = processor.Parse(`[1, 2, 3]`, &arrData)
		helper.AssertNoError(err)
		helper.AssertTrue(isJsonArray(arrData))

		helper.AssertTrue(isJsonPrimitive("string"))
		helper.AssertTrue(isJsonPrimitive(123))
		helper.AssertTrue(isJsonPrimitive(true))
		helper.AssertTrue(isJsonPrimitive(nil))
	})

	t.Run("DeepCopy", func(t *testing.T) {
		original := map[string]any{
			"name": "test",
			"nested": map[string]any{
				"value": 123,
			},
			"array": []any{1, 2, 3},
		}

		copied, err := DeepCopy(original)
		helper.AssertNoError(err)
		helper.AssertNotNil(copied)

		original["name"] = "modified"
		copiedMap := copied.(map[string]any)
		helper.AssertEqual("test", copiedMap["name"])
	})

	t.Run("CompareJson", func(t *testing.T) {
		json1 := `{"name": "test", "value": 123}`
		json2 := `{"value": 123, "name": "test"}`
		json3 := `{"name": "different", "value": 123}`

		equal, err := CompareJson(json1, json2)
		helper.AssertNoError(err)
		helper.AssertTrue(equal)

		equal, err = CompareJson(json1, json3)
		helper.AssertNoError(err)
		helper.AssertFalse(equal)
	})

	t.Run("MergeJson", func(t *testing.T) {
		json1 := `{"name": "test", "value": 123}`
		json2 := `{"value": 456, "extra": "field"}`

		merged, err := MergeJson(json1, json2)
		helper.AssertNoError(err)
		helper.AssertNotNil(merged)

		name, _ := GetString(merged, "name")
		helper.AssertEqual("test", name)

		value, _ := GetInt(merged, "value")
		helper.AssertEqual(456, value)

		extra, _ := GetString(merged, "extra")
		helper.AssertEqual("field", extra)
	})

	t.Run("TypeSafeConvert", func(t *testing.T) {
		intVal, err := TypeSafeConvert[int](42)
		helper.AssertNoError(err)
		helper.AssertEqual(42, intVal)

		strVal, err := TypeSafeConvert[string]("test")
		helper.AssertNoError(err)
		helper.AssertEqual("test", strVal)

		boolVal, err := TypeSafeConvert[bool](true)
		helper.AssertNoError(err)
		helper.AssertTrue(boolVal)
	})

	t.Run("SafeTypeAssert", func(t *testing.T) {
		var value any = "test"

		strVal, ok := SafeTypeAssert[string](value)
		helper.AssertTrue(ok)
		helper.AssertEqual("test", strVal)

		intVal, ok := SafeTypeAssert[int](value)
		helper.AssertFalse(ok)
		helper.AssertEqual(0, intVal)
	})

	t.Run("Iterator", func(t *testing.T) {
		processor := New()
		defer processor.Close()

		jsonData := `{"name": "test", "value": 123, "active": true}`

		var data interface{}
		err := processor.Parse(jsonData, &data)
		helper.AssertNoError(err)

		iterator := NewIterator(processor, data, DefaultOptions())
		helper.AssertNotNil(iterator)

		options := DefaultOptions()
		options.StrictMode = true
		iteratorWithOptions := NewIterator(processor, data, options)
		helper.AssertNotNil(iteratorWithOptions)
	})

	t.Run("IterableValue", func(t *testing.T) {
		processor := New()
		defer processor.Close()

		jsonData := `{
			"name": "Alice",
			"age": 25,
			"score": 87.5,
			"active": true,
			"tags": ["user", "premium"]
		}`

		var data interface{}
		err := processor.Parse(jsonData, &data)
		helper.AssertNoError(err)

		iterator := NewIterator(processor, data, DefaultOptions())
		iterableValue := NewIterableValueWithIterator(data, processor, iterator)

		helper.AssertEqual("Alice", iterableValue.GetString("name"))
		helper.AssertEqual(25, iterableValue.GetInt("age"))
		helper.AssertEqual(87.5, iterableValue.GetFloat64("score"))
		helper.AssertEqual(true, iterableValue.GetBool("active"))

		tags := iterableValue.GetArray("tags")
		helper.AssertEqual(2, len(tags))
	})

	t.Run("GetWithDefault", func(t *testing.T) {
		processor := New()
		defer processor.Close()

		jsonData := `{"existing": "value"}`

		var data interface{}
		err := processor.Parse(jsonData, &data)
		helper.AssertNoError(err)

		iterator := NewIterator(processor, data, DefaultOptions())
		iterableValue := NewIterableValueWithIterator(data, processor, iterator)

		existing := iterableValue.GetWithDefault("existing", "default")
		helper.AssertEqual("value", existing)

		stringDefault := iterableValue.GetStringWithDefault("missing", "default")
		helper.AssertEqual("default", stringDefault)

		intDefault := iterableValue.GetIntWithDefault("missing", 42)
		helper.AssertEqual(42, intDefault)

		floatDefault := iterableValue.GetFloat64WithDefault("missing", 3.14)
		helper.AssertEqual(3.14, floatDefault)

		boolDefault := iterableValue.GetBoolWithDefault("missing", true)
		helper.AssertEqual(true, boolDefault)
	})
}
