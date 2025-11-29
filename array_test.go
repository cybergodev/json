package json

import (
	"testing"
)

// TestArrayOperations tests all array operations comprehensively
func TestArrayOperations(t *testing.T) {
	helper := NewTestHelper(t)

	testData := `{
		"numbers": [1, 2, 3, 4, 5, 6, 7, 8, 9, 10],
		"users": [
			{"name": "Alice", "age": 25, "skills": ["Go", "Python"]},
			{"name": "Bob", "age": 30, "skills": ["Java", "React"]},
			{"name": "Charlie", "age": 35, "skills": ["C++", "Rust"]},
			{"name": "Diana", "age": 28, "skills": ["Vue", "JavaScript"]}
		],
		"matrix": [
			[1, 2, 3],
			[4, 5, 6],
			[7, 8, 9]
		],
		"empty": [],
		"mixed": [1, "string", true, null, {"key": "value"}]
	}`

	t.Run("BasicArrayAccess", func(t *testing.T) {
		// Positive index
		first, err := GetInt(testData, "numbers[0]")
		helper.AssertNoError(err)
		helper.AssertEqual(1, first)

		// Negative index
		last, err := GetInt(testData, "numbers[-1]")
		helper.AssertNoError(err)
		helper.AssertEqual(10, last)

		// Out of bounds
		outOfBounds, err := Get(testData, "numbers[100]")
		helper.AssertNoError(err)
		helper.AssertNil(outOfBounds)

		negOutOfBounds, err := Get(testData, "numbers[-100]")
		helper.AssertNoError(err)
		helper.AssertNil(negOutOfBounds)
	})

	t.Run("NestedArrayAccess", func(t *testing.T) {
		userName, err := GetString(testData, "users[1].name")
		helper.AssertNoError(err)
		helper.AssertEqual("Bob", userName)

		matrixElement, err := GetInt(testData, "matrix[1][2]")
		helper.AssertNoError(err)
		helper.AssertEqual(6, matrixElement)

		lastUserName, err := GetString(testData, "users[-1].name")
		helper.AssertNoError(err)
		helper.AssertEqual("Diana", lastUserName)
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

		// Full slice [:]
		fullSlice, err := Get(testData, "numbers[:]")
		helper.AssertNoError(err)
		if fullArray, ok := fullSlice.([]any); ok {
			helper.AssertEqual(10, len(fullArray))
		}
	})

	t.Run("ArraySlicingWithStep", func(t *testing.T) {
		// Slice with step [start:end:step]
		stepSlice, err := Get(testData, "numbers[1:8:2]")
		helper.AssertNoError(err)
		expectedStep := []any{float64(2), float64(4), float64(6), float64(8)}
		helper.AssertEqual(expectedStep, stepSlice)

		// Slice with step from start [start::step]
		stepFromStart, err := Get(testData, "numbers[::3]")
		helper.AssertNoError(err)
		expectedStepFromStart := []any{float64(1), float64(4), float64(7), float64(10)}
		helper.AssertEqual(expectedStepFromStart, stepFromStart)

		// Reverse slice with negative step
		reverseSlice, err := Get(testData, "numbers[::-1]")
		helper.AssertNoError(err)
		if reverseArray, ok := reverseSlice.([]any); ok {
			helper.AssertEqual(10, len(reverseArray))
			helper.AssertEqual(float64(10), reverseArray[0])
			helper.AssertEqual(float64(1), reverseArray[9])
		}
	})

	t.Run("NegativeIndexSlicing", func(t *testing.T) {
		negStartSlice, err := Get(testData, "numbers[-3:]")
		helper.AssertNoError(err)
		expectedNegStart := []any{float64(8), float64(9), float64(10)}
		helper.AssertEqual(expectedNegStart, negStartSlice)

		negEndSlice, err := Get(testData, "numbers[:-2]")
		helper.AssertNoError(err)
		if negEndArray, ok := negEndSlice.([]any); ok {
			helper.AssertEqual(8, len(negEndArray))
			helper.AssertEqual(float64(8), negEndArray[7])
		}

		bothNegSlice, err := Get(testData, "numbers[-5:-2]")
		helper.AssertNoError(err)
		expectedBothNeg := []any{float64(6), float64(7), float64(8)}
		helper.AssertEqual(expectedBothNeg, bothNegSlice)
	})

	t.Run("EmptyArrayHandling", func(t *testing.T) {
		emptyAccess, err := Get(testData, "empty[0]")
		helper.AssertNoError(err)
		helper.AssertNil(emptyAccess)

		emptySlice, err := Get(testData, "empty[0:5]")
		helper.AssertNoError(err)
		if emptySliceArray, ok := emptySlice.([]any); ok {
			helper.AssertEqual(0, len(emptySliceArray))
		} else {
			helper.AssertNil(emptySlice)
		}
	})

	t.Run("MixedTypeArrayHandling", func(t *testing.T) {
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
}

// TestArrayModification tests array modification operations
func TestArrayModification(t *testing.T) {
	helper := NewTestHelper(t)

	baseData := `{
		"numbers": [1, 2, 3, 4, 5],
		"users": [
			{"name": "Alice", "active": false},
			{"name": "Bob", "active": false},
			{"name": "Charlie", "active": false}
		]
	}`

	t.Run("SetArrayElement", func(t *testing.T) {
		newData, err := Set(baseData, "numbers[2]", 99)
		helper.AssertNoError(err)

		newValue, err := GetInt(newData, "numbers[2]")
		helper.AssertNoError(err)
		helper.AssertEqual(99, newValue)

		firstValue, err := GetInt(newData, "numbers[0]")
		helper.AssertNoError(err)
		helper.AssertEqual(1, firstValue)
	})

	t.Run("SetNestedArrayElement", func(t *testing.T) {
		newData, err := Set(baseData, "users[1].active", true)
		helper.AssertNoError(err)

		newValue, err := GetBool(newData, "users[1].active")
		helper.AssertNoError(err)
		helper.AssertTrue(newValue)

		firstUserActive, err := GetBool(newData, "users[0].active")
		helper.AssertNoError(err)
		helper.AssertFalse(firstUserActive)
	})

	t.Run("SetMultipleArrayElements", func(t *testing.T) {
		data1, err := Set(baseData, "users[0].active", true)
		helper.AssertNoError(err)

		data2, err := Set(data1, "users[2].active", true)
		helper.AssertNoError(err)

		firstActive, err := GetBool(data2, "users[0].active")
		helper.AssertNoError(err)
		helper.AssertTrue(firstActive)

		thirdActive, err := GetBool(data2, "users[2].active")
		helper.AssertNoError(err)
		helper.AssertTrue(thirdActive)

		secondActive, err := GetBool(data2, "users[1].active")
		helper.AssertNoError(err)
		helper.AssertFalse(secondActive)
	})

	t.Run("SetArrayElementWithNegativeIndex", func(t *testing.T) {
		newData, err := Set(baseData, "numbers[-1]", 50)
		helper.AssertNoError(err)

		lastValue, err := GetInt(newData, "numbers[-1]")
		helper.AssertNoError(err)
		helper.AssertEqual(50, lastValue)

		lastValuePos, err := GetInt(newData, "numbers[4]")
		helper.AssertNoError(err)
		helper.AssertEqual(50, lastValuePos)
	})
}

// TestArrayExpansion tests array expansion functionality
func TestArrayExpansion(t *testing.T) {
	helper := NewTestHelper(t)

	t.Run("SingleIndexExpansion", func(t *testing.T) {
		processor := New()
		defer processor.Close()

		testData := `{"tags": ["technology", "startup", "innovation", "growth"]}`

		result, err := SetWithAdd(testData, "tags[4]", "haha")
		helper.AssertNoError(err)

		value, err := processor.Get(result, "tags[4]")
		helper.AssertNoError(err)
		helper.AssertEqual("haha", value)

		tags, err := processor.Get(result, "tags")
		helper.AssertNoError(err)
		if arr, ok := tags.([]any); ok {
			helper.AssertEqual(5, len(arr))
		}

		// Large index expansion
		result2, err := SetWithAdd(testData, "tags[10]", "future")
		helper.AssertNoError(err)

		value2, err := processor.Get(result2, "tags[10]")
		helper.AssertNoError(err)
		helper.AssertEqual("future", value2)

		tags2, err := processor.Get(result2, "tags")
		helper.AssertNoError(err)
		if arr, ok := tags2.([]any); ok {
			helper.AssertEqual(11, len(arr))
			helper.AssertEqual(nil, arr[5])
			helper.AssertEqual(nil, arr[9])
		}
	})

	t.Run("SliceExpansion", func(t *testing.T) {
		processor := New()
		defer processor.Close()

		testData := `{"tags": ["technology", "startup", "innovation", "growth"]}`

		result, err := SetWithAdd(testData, "tags[1:5]", "modified")
		helper.AssertNoError(err)

		tags, err := processor.Get(result, "tags")
		helper.AssertNoError(err)
		if arr, ok := tags.([]any); ok {
			helper.AssertEqual(5, len(arr))
			helper.AssertEqual("technology", arr[0])
			helper.AssertEqual("modified", arr[1])
			helper.AssertEqual("modified", arr[4])
		}
	})

	t.Run("CreateNewArrays", func(t *testing.T) {
		processor := New()
		defer processor.Close()

		result, err := SetWithAdd("{}", "newArray[3]", "new-item")
		helper.AssertNoError(err)

		value, err := processor.Get(result, "newArray[3]")
		helper.AssertNoError(err)
		helper.AssertEqual("new-item", value)

		newArray, err := processor.Get(result, "newArray")
		helper.AssertNoError(err)
		if arr, ok := newArray.([]any); ok {
			helper.AssertEqual(4, len(arr))
			helper.AssertEqual(nil, arr[0])
			helper.AssertEqual("new-item", arr[3])
		}
	})

	t.Run("NestedArrayExpansion", func(t *testing.T) {
		processor := New()
		defer processor.Close()

		testData := `{"data": {"items": [{"name": "item1"}, {"name": "item2"}]}}`

		result, err := SetWithAdd(testData, "data.items[5]", map[string]any{"name": "item6"})
		helper.AssertNoError(err)

		value, err := processor.Get(result, "data.items[5].name")
		helper.AssertNoError(err)
		helper.AssertEqual("item6", value)

		items, err := processor.Get(result, "data.items")
		helper.AssertNoError(err)
		if arr, ok := items.([]any); ok {
			helper.AssertEqual(6, len(arr))
			helper.AssertEqual(nil, arr[2])
			helper.AssertEqual(nil, arr[4])
		}
	})
}

// TestArrayInternalOperations tests internal array handler operations
func TestArrayInternalOperations(t *testing.T) {
	helper := NewTestHelper(t)
	processor := New()
	defer processor.Close()

	t.Run("DirectArrayAccess", func(t *testing.T) {
		testData := `[100, 200, 300, 400]`

		result, err := processor.Get(testData, "[0]")
		helper.AssertNoError(err)
		helper.AssertEqual(float64(100), result)

		result, err = processor.Get(testData, "[-1]")
		helper.AssertNoError(err)
		helper.AssertEqual(float64(400), result)

		result, err = processor.Get(testData, "[10]")
		helper.AssertNoError(err)
		helper.AssertNil(result)
	})

	t.Run("InvalidArrayIndices", func(t *testing.T) {
		testData := `{"arr": [10, 20, 30]}`

		_, err := processor.Get(testData, "arr[abc]")
		helper.AssertError(err)

		_, err = processor.Get(testData, "arr[1.5]")
		helper.AssertError(err)

		_, err = processor.Get(testData, "arr[]")
		helper.AssertError(err)
	})

	t.Run("ComplexNestedArrays", func(t *testing.T) {
		testData := `{
			"matrix": [[1, 2, 3], [4, 5, 6], [7, 8, 9]],
			"objects": [
				{"values": [10, 20, 30]},
				{"values": [40, 50, 60]},
				{"values": [70, 80, 90]}
			]
		}`

		result, err := processor.Get(testData, "matrix[1][2]")
		helper.AssertNoError(err)
		helper.AssertEqual(float64(6), result)

		result, err = processor.Get(testData, "objects[2].values[1]")
		helper.AssertNoError(err)
		helper.AssertEqual(float64(80), result)

		result, err = processor.Get(testData, "matrix[0][1:3]")
		helper.AssertNoError(err)
		if arr, ok := result.([]any); ok {
			helper.AssertEqual(2, len(arr))
			helper.AssertEqual(float64(2), arr[0])
			helper.AssertEqual(float64(3), arr[1])
		}
	})
}
