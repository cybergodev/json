package json

import (
	"testing"
)

// TestArrayPathOperations consolidates array operations, path navigation, and utilities
// Replaces: array_path_test.go, utils_iterator_test.go
func TestArrayPathOperations(t *testing.T) {
	helper := NewTestHelper(t)

	testData := `{
		"numbers": [1, 2, 3, 4, 5, 6, 7, 8, 9, 10],
		"users": [
			{"name": "Alice", "age": 25, "skills": ["Go", "Python"]},
			{"name": "Bob", "age": 30, "skills": ["Java", "React"]},
			{"name": "Charlie", "age": 35, "skills": ["C++", "Rust"]}
		],
		"matrix": [[1, 2, 3], [4, 5, 6], [7, 8, 9]],
		"empty": [],
		"mixed": [1, "string", true, null, {"key": "value"}]
	}`

	t.Run("ArrayAccess", func(t *testing.T) {
		// Nested array access
		userName, err := GetString(testData, "users[1].name")
		helper.AssertNoError(err)
		helper.AssertEqual("Bob", userName)

		// Matrix access
		matrixElement, err := GetInt(testData, "matrix[1][2]")
		helper.AssertNoError(err)
		helper.AssertEqual(6, matrixElement)

		// Negative index
		lastUserName, err := GetString(testData, "users[-1].name")
		helper.AssertNoError(err)
		helper.AssertEqual("Charlie", lastUserName)

		// Out of bounds
		outOfBounds, err := Get(testData, "numbers[100]")
		helper.AssertNoError(err)
		helper.AssertNil(outOfBounds)
	})

	t.Run("ArraySlicing", func(t *testing.T) {
		// Basic slice [start:end]
		slice, err := Get(testData, "numbers[2:5]")
		helper.AssertNoError(err)
		expected := []any{float64(3), float64(4), float64(5)}
		helper.AssertEqual(expected, slice)

		// Slice from start [start:]
		fromStart, err := Get(testData, "numbers[7:]")
		helper.AssertNoError(err)
		expectedFromStart := []any{float64(8), float64(9), float64(10)}
		helper.AssertEqual(expectedFromStart, fromStart)

		// Slice to end [:end]
		toEnd, err := Get(testData, "numbers[:3]")
		helper.AssertNoError(err)
		expectedToEnd := []any{float64(1), float64(2), float64(3)}
		helper.AssertEqual(expectedToEnd, toEnd)

		// Negative index slicing
		negSlice, err := Get(testData, "numbers[-3:]")
		helper.AssertNoError(err)
		expectedNeg := []any{float64(8), float64(9), float64(10)}
		helper.AssertEqual(expectedNeg, negSlice)
	})

	t.Run("ArrayModification", func(t *testing.T) {
		baseData := `{"numbers": [1, 2, 3, 4, 5]}`

		// Set array element
		newData, err := Set(baseData, "numbers[2]", 99)
		helper.AssertNoError(err)
		newValue, err := GetInt(newData, "numbers[2]")
		helper.AssertNoError(err)
		helper.AssertEqual(99, newValue)

		// Set with negative index
		newData2, err := Set(baseData, "numbers[-1]", 50)
		helper.AssertNoError(err)
		lastValue, err := GetInt(newData2, "numbers[-1]")
		helper.AssertNoError(err)
		helper.AssertEqual(50, lastValue)
	})

	t.Run("ArrayExpansion", func(t *testing.T) {
		processor := New()
		defer processor.Close()

		testData := `{"tags": ["tech", "startup"]}`

		// Expand array with new index
		result, err := SetWithAdd(testData, "tags[4]", "new")
		helper.AssertNoError(err)
		value, err := processor.Get(result, "tags[4]")
		helper.AssertNoError(err)
		helper.AssertEqual("new", value)

		// Verify array length
		tags, err := processor.Get(result, "tags")
		helper.AssertNoError(err)
		if arr, ok := tags.([]any); ok {
			helper.AssertEqual(5, len(arr))
			helper.AssertEqual(nil, arr[2]) // Gap filled with nil
		}
	})

	t.Run("PathExtraction", func(t *testing.T) {
		// Extract names from array
		names, err := Get(testData, "users{name}")
		helper.AssertNoError(err)
		if arr, ok := names.([]any); ok {
			helper.AssertEqual(3, len(arr))
			helper.AssertEqual("Alice", arr[0])
			helper.AssertEqual("Bob", arr[1])
			helper.AssertEqual("Charlie", arr[2])
		}
	})

	t.Run("PathValidation", func(t *testing.T) {
		helper.AssertTrue(IsValidPath("user.name"))
		helper.AssertTrue(IsValidPath("items[0]"))
		helper.AssertTrue(IsValidPath("data[1:5]"))
		helper.AssertTrue(IsValidPath("users{name}"))
		helper.AssertTrue(IsValidPath("."))
		helper.AssertFalse(IsValidPath(""))

		// Invalid paths
		_, err := Get(testData, "items[abc]")
		helper.AssertError(err)

		_, err = Get(testData, "items[]")
		helper.AssertError(err)

		// ValidatePath function
		err = ValidatePath("user.name")
		helper.AssertNoError(err)

		err = ValidatePath("")
		helper.AssertError(err)
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

	t.Run("MixedTypeArray", func(t *testing.T) {
		intValue, err := GetInt(testData, "mixed[0]")
		helper.AssertNoError(err)
		helper.AssertEqual(1, intValue)

		stringValue, err := GetString(testData, "mixed[1]")
		helper.AssertNoError(err)
		helper.AssertEqual("string", stringValue)

		boolValue, err := GetBool(testData, "mixed[2]")
		helper.AssertNoError(err)
		helper.AssertTrue(boolValue)

		nullValue, err := Get(testData, "mixed[3]")
		helper.AssertNoError(err)
		helper.AssertNil(nullValue)

		objectValue, err := Get(testData, "mixed[4].key")
		helper.AssertNoError(err)
		helper.AssertEqual("value", objectValue)
	})

	t.Run("UtilityFunctions", func(t *testing.T) {
		// DeepCopy
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

		// CompareJson
		json1 := `{"name": "test", "value": 123}`
		json2 := `{"value": 123, "name": "test"}`
		json3 := `{"name": "different", "value": 123}`

		equal, err := CompareJson(json1, json2)
		helper.AssertNoError(err)
		helper.AssertTrue(equal)

		equal, err = CompareJson(json1, json3)
		helper.AssertNoError(err)
		helper.AssertFalse(equal)

		// MergeJson
		merged, err := MergeJson(json1, `{"extra": "field"}`)
		helper.AssertNoError(err)
		helper.AssertNotNil(merged)

		name, _ := GetString(merged, "name")
		helper.AssertEqual("test", name)

		extra, _ := GetString(merged, "extra")
		helper.AssertEqual("field", extra)
	})

	t.Run("TypeSafeOperations", func(t *testing.T) {
		// TypeSafeConvert
		intVal, err := TypeSafeConvert[int](42)
		helper.AssertNoError(err)
		helper.AssertEqual(42, intVal)

		strVal, err := TypeSafeConvert[string]("test")
		helper.AssertNoError(err)
		helper.AssertEqual("test", strVal)

		boolVal, err := TypeSafeConvert[bool](true)
		helper.AssertNoError(err)
		helper.AssertTrue(boolVal)

		// SafeTypeAssert
		var value any = "test"

		safeStr, ok := SafeTypeAssert[string](value)
		helper.AssertTrue(ok)
		helper.AssertEqual("test", safeStr)

		safeInt, ok := SafeTypeAssert[int](value)
		helper.AssertFalse(ok)
		helper.AssertEqual(0, safeInt)
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

		// Test defaults
		existing := iterableValue.GetWithDefault("name", "default")
		helper.AssertEqual("Alice", existing)

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
