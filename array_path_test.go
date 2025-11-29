package json

import (
	"testing"
)

// TestArrayAndPath consolidates array operations and path navigation tests
// Replaces: array_test.go, path_navigation_test.go
func TestArrayAndPath(t *testing.T) {
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
}
