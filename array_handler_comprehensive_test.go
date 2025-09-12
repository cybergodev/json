package json

import (
	"testing"
)

// TestArrayHandlerAdvanced tests advanced array handler functionality
func TestArrayHandlerAdvanced(t *testing.T) {
	helper := NewTestHelper(t)

	t.Run("HandleArrayAccess", func(t *testing.T) {
		processor := New()
		defer processor.Close()

		testData := `{
			"numbers": [10, 20, 30, 40, 50],
			"strings": ["a", "b", "c", "d"],
			"nested": [
				{"id": 1, "name": "first"},
				{"id": 2, "name": "second"},
				{"id": 3, "name": "third"}
			],
			"empty": [],
			"mixed": [1, "two", 3.0, true, null]
		}`

		// Test positive indices
		result, err := processor.Get(testData, "numbers[0]")
		helper.AssertNoError(err, "Should access first element")
		helper.AssertEqual(float64(10), result, "First element should be 10")

		result, err = processor.Get(testData, "numbers[4]")
		helper.AssertNoError(err, "Should access last element")
		helper.AssertEqual(float64(50), result, "Last element should be 50")

		// Test negative indices
		result, err = processor.Get(testData, "numbers[-1]")
		helper.AssertNoError(err, "Should access last element with negative index")
		helper.AssertEqual(float64(50), result, "Last element should be 50")

		result, err = processor.Get(testData, "numbers[-5]")
		helper.AssertNoError(err, "Should access first element with negative index")
		helper.AssertEqual(float64(10), result, "First element should be 10")

		// Test out of bounds
		result, err = processor.Get(testData, "numbers[10]")
		helper.AssertNoError(err, "Out of bounds should not error")
		helper.AssertNil(result, "Out of bounds should return nil")

		result, err = processor.Get(testData, "numbers[-10]")
		helper.AssertNoError(err, "Negative out of bounds should not error")
		helper.AssertNil(result, "Negative out of bounds should return nil")

		// Test nested array access
		result, err = processor.Get(testData, "nested[1].name")
		helper.AssertNoError(err, "Should access nested array element property")
		helper.AssertEqual("second", result, "Should get correct nested value")

		// Test empty array
		result, err = processor.Get(testData, "empty[0]")
		helper.AssertNoError(err, "Empty array access should not error")
		helper.AssertNil(result, "Empty array access should return nil")

		// Test mixed type array
		result, err = processor.Get(testData, "mixed[0]")
		helper.AssertNoError(err, "Should access mixed array number")
		helper.AssertEqual(float64(1), result, "Should get number from mixed array")

		result, err = processor.Get(testData, "mixed[1]")
		helper.AssertNoError(err, "Should access mixed array string")
		helper.AssertEqual("two", result, "Should get string from mixed array")

		result, err = processor.Get(testData, "mixed[3]")
		helper.AssertNoError(err, "Should access mixed array boolean")
		helper.AssertEqual(true, result, "Should get boolean from mixed array")

		result, err = processor.Get(testData, "mixed[4]")
		helper.AssertNoError(err, "Should access mixed array null")
		helper.AssertNil(result, "Should get nil from mixed array")
	})

	t.Run("HandleArraySlice", func(t *testing.T) {
		processor := New()
		defer processor.Close()

		testData := `{
			"numbers": [0, 1, 2, 3, 4, 5, 6, 7, 8, 9],
			"letters": ["a", "b", "c", "d", "e", "f", "g"],
			"empty": []
		}`

		// Test basic slicing
		result, err := processor.Get(testData, "numbers[1:4]")
		helper.AssertNoError(err, "Basic slice should work")
		if arr, ok := result.([]any); ok {
			helper.AssertEqual(3, len(arr), "Slice should have 3 elements")
			helper.AssertEqual(float64(1), arr[0], "First element should be 1")
			helper.AssertEqual(float64(3), arr[2], "Last element should be 3")
		}

		// Test slice with negative indices
		result, err = processor.Get(testData, "numbers[-3:-1]")
		helper.AssertNoError(err, "Negative slice should work")
		if arr, ok := result.([]any); ok {
			helper.AssertEqual(2, len(arr), "Negative slice should have 2 elements")
			helper.AssertEqual(float64(7), arr[0], "First element should be 7")
			helper.AssertEqual(float64(8), arr[1], "Second element should be 8")
		}

		// Test slice from start
		result, err = processor.Get(testData, "numbers[:3]")
		helper.AssertNoError(err, "Slice from start should work")
		if arr, ok := result.([]any); ok {
			helper.AssertEqual(3, len(arr), "Slice from start should have 3 elements")
			helper.AssertEqual(float64(0), arr[0], "First element should be 0")
		}

		// Test slice to end
		result, err = processor.Get(testData, "numbers[7:]")
		helper.AssertNoError(err, "Slice to end should work")
		if arr, ok := result.([]any); ok {
			helper.AssertEqual(3, len(arr), "Slice to end should have 3 elements")
			helper.AssertEqual(float64(7), arr[0], "First element should be 7")
			helper.AssertEqual(float64(9), arr[2], "Last element should be 9")
		}

		// Test full slice
		result, err = processor.Get(testData, "numbers[:]")
		helper.AssertNoError(err, "Full slice should work")
		if arr, ok := result.([]any); ok {
			helper.AssertEqual(10, len(arr), "Full slice should have all elements")
		}

		// Test empty slice
		result, err = processor.Get(testData, "empty[:]")
		helper.AssertNoError(err, "Empty array slice should work")
		if arr, ok := result.([]any); ok {
			helper.AssertEqual(0, len(arr), "Empty slice should have 0 elements")
		}

		// Test out of bounds slice
		result, err = processor.Get(testData, "numbers[5:20]")
		helper.AssertNoError(err, "Out of bounds slice should work")
		if arr, ok := result.([]any); ok {
			helper.AssertEqual(5, len(arr), "Out of bounds slice should be clamped")
			helper.AssertEqual(float64(5), arr[0], "First element should be 5")
		}
	})

	t.Run("ParseArrayIndexFromSegment", func(t *testing.T) {
		processor := New()
		defer processor.Close()

		testData := `{"arr": [10, 20, 30]}`

		// Test valid numeric indices
		result, err := processor.Get(testData, "arr[0]")
		helper.AssertNoError(err, "Valid index should work")
		helper.AssertEqual(float64(10), result, "Should get correct element")

		result, err = processor.Get(testData, "arr[2]")
		helper.AssertNoError(err, "Valid index should work")
		helper.AssertEqual(float64(30), result, "Should get correct element")

		// Test invalid indices (should return error)
		_, err = processor.Get(testData, "arr[abc]")
		helper.AssertError(err, "Invalid index should return error")

		_, err = processor.Get(testData, "arr[1.5]")
		helper.AssertError(err, "Float index should return error")

		_, err = processor.Get(testData, "arr[]")
		helper.AssertError(err, "Empty index should return error")
	})

	t.Run("ArrayAccessValue", func(t *testing.T) {
		processor := New()
		defer processor.Close()

		// Test direct array access (no property)
		testData := `[100, 200, 300, 400]`

		result, err := processor.Get(testData, "[0]")
		helper.AssertNoError(err, "Direct array access should work")
		helper.AssertEqual(float64(100), result, "Should get first element")

		result, err = processor.Get(testData, "[-1]")
		helper.AssertNoError(err, "Direct negative array access should work")
		helper.AssertEqual(float64(400), result, "Should get last element")

		// Test invalid direct access
		result, err = processor.Get(testData, "[10]")
		helper.AssertNoError(err, "Out of bounds direct access should not error")
		helper.AssertNil(result, "Out of bounds should return nil")
	})

	t.Run("ComplexArrayOperations", func(t *testing.T) {
		processor := New()
		defer processor.Close()

		testData := `{
			"matrix": [
				[1, 2, 3],
				[4, 5, 6],
				[7, 8, 9]
			],
			"objects": [
				{"values": [10, 20, 30]},
				{"values": [40, 50, 60]},
				{"values": [70, 80, 90]}
			]
		}`

		// Test nested array access
		result, err := processor.Get(testData, "matrix[1][2]")
		helper.AssertNoError(err, "Nested array access should work")
		helper.AssertEqual(float64(6), result, "Should get correct nested element")

		// Test array of objects with array properties
		result, err = processor.Get(testData, "objects[2].values[1]")
		helper.AssertNoError(err, "Complex nested access should work")
		helper.AssertEqual(float64(80), result, "Should get correct complex nested element")

		// Test slice of nested arrays
		result, err = processor.Get(testData, "matrix[0][1:3]")
		helper.AssertNoError(err, "Slice of nested array should work")
		if arr, ok := result.([]any); ok {
			helper.AssertEqual(2, len(arr), "Nested slice should have 2 elements")
			helper.AssertEqual(float64(2), arr[0], "First element should be 2")
			helper.AssertEqual(float64(3), arr[1], "Second element should be 3")
		}
	})
}
