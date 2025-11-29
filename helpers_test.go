package json

import (
	"testing"
)

// TestHelpers tests helper functions that are not covered by other tests
func TestHelpers(t *testing.T) {
	helper := NewTestHelper(t)

	t.Run("PathValidation", func(t *testing.T) {
		// Test IsValidPath
		helper.AssertTrue(IsValidPath("user.name"))
		helper.AssertTrue(IsValidPath("items[0]"))
		helper.AssertTrue(IsValidPath("data[1:5]"))
		helper.AssertTrue(IsValidPath("users{name}"))
		helper.AssertTrue(IsValidPath("."))
		helper.AssertFalse(IsValidPath(""))

		// Test ValidatePath
		err := ValidatePath("user.name")
		helper.AssertNoError(err)

		err = ValidatePath("")
		helper.AssertError(err)

		err = ValidatePath("invalid[")
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

		// Modify original
		original["name"] = "modified"
		original["nested"].(map[string]any)["value"] = 999

		// Check copy is unchanged
		copiedMap := copied.(map[string]any)
		helper.AssertEqual("test", copiedMap["name"])
		// Value might be json.Number or int, just check it's not 999
		nestedValue := copiedMap["nested"].(map[string]any)["value"]
		helper.AssertNotEqual(999, nestedValue)
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

		// Verify merged result
		name, err := GetString(merged, "name")
		helper.AssertNoError(err)
		helper.AssertEqual("test", name)

		value, err := GetInt(merged, "value")
		helper.AssertNoError(err)
		helper.AssertEqual(456, value) // json2 should override

		extra, err := GetString(merged, "extra")
		helper.AssertNoError(err)
		helper.AssertEqual("field", extra)
	})

	t.Run("TypeSafeConvert", func(t *testing.T) {
		// Test int conversion
		intVal, err := TypeSafeConvert[int](42)
		helper.AssertNoError(err)
		helper.AssertEqual(42, intVal)

		// Test string conversion
		strVal, err := TypeSafeConvert[string]("test")
		helper.AssertNoError(err)
		helper.AssertEqual("test", strVal)

		strVal, err = TypeSafeConvert[string](123)
		helper.AssertNoError(err)
		helper.AssertEqual("123", strVal)

		// Test bool conversion
		boolVal, err := TypeSafeConvert[bool](true)
		helper.AssertNoError(err)
		helper.AssertEqual(true, boolVal)
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

	t.Run("GenericGetters", func(t *testing.T) {
		testData := `{"number": 42, "text": "hello", "flag": true}`

		// Test GetNumeric
		num, err := GetNumeric[int](testData, "number")
		helper.AssertNoError(err)
		helper.AssertEqual(42, num)

		// Test GetOrdered
		text := GetOrdered[string](testData, "text", "default")
		helper.AssertEqual("hello", text)

		missing := GetOrdered[string](testData, "missing", "default")
		helper.AssertEqual("default", missing)

		// Test GetJSONValue
		flag, err := GetJSONValue[bool](testData, "flag")
		helper.AssertNoError(err)
		helper.AssertEqual(true, flag)
	})
}
