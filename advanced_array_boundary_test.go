package json

import (
	"testing"
)

// TestAdvancedArrayBoundaryOperations tests comprehensive array boundary conditions and error handling
func TestAdvancedArrayBoundaryOperations(t *testing.T) {
	helper := NewTestHelper(t)

	t.Run("ArrayIndexBoundaryConditions", func(t *testing.T) {
		processor := New()
		defer processor.Close()

		testData := `{
			"numbers": [10, 20, 30, 40, 50],
			"empty": [],
			"single": [100],
			"nested": [
				[1, 2, 3],
				[4, 5, 6],
				[7, 8, 9]
			]
		}`

		// Test valid index access
		value, err := processor.Get(testData, "numbers[2]")
		helper.AssertNoError(err, "Valid index should work")
		helper.AssertEqual(float64(30), value, "Valid index value should be correct")

		// Test first element
		first, err := processor.Get(testData, "numbers[0]")
		helper.AssertNoError(err, "First element access should work")
		helper.AssertEqual(float64(10), first, "First element should be correct")

		// Test last element with valid index
		last, err := processor.Get(testData, "numbers[4]")
		helper.AssertNoError(err, "Last element access should work")
		helper.AssertEqual(float64(50), last, "Last element should be correct")

		// Test out of bounds positive index
		outOfBounds, err := processor.Get(testData, "numbers[10]")
		if err == nil {
			helper.AssertNil(outOfBounds, "Out of bounds should return nil")
		}

		// Test negative index (last element)
		negativeIndex, err := processor.Get(testData, "numbers[-1]")
		helper.AssertNoError(err, "Negative index should work")
		helper.AssertEqual(float64(50), negativeIndex, "Negative index should return last element")

		// Test negative index (second to last)
		negativeIndex2, err := processor.Get(testData, "numbers[-2]")
		helper.AssertNoError(err, "Negative index -2 should work")
		helper.AssertEqual(float64(40), negativeIndex2, "Negative index -2 should return second to last")

		// Test negative index out of bounds
		negativeOutOfBounds, err := processor.Get(testData, "numbers[-10]")
		if err == nil {
			helper.AssertNil(negativeOutOfBounds, "Negative out of bounds should return nil")
		}

		// Test empty array access
		emptyAccess, err := processor.Get(testData, "empty[0]")
		if err == nil {
			helper.AssertNil(emptyAccess, "Empty array access should return nil")
		}

		// Test single element array
		singleElement, err := processor.Get(testData, "single[0]")
		helper.AssertNoError(err, "Single element access should work")
		helper.AssertEqual(float64(100), singleElement, "Single element should be correct")

		// Test single element array out of bounds
		singleOutOfBounds, err := processor.Get(testData, "single[1]")
		if err == nil {
			helper.AssertNil(singleOutOfBounds, "Single array out of bounds should return nil")
		}
	})

	t.Run("ArraySliceBoundaryConditions", func(t *testing.T) {
		processor := New()
		defer processor.Close()

		testData := `{
			"data": [0, 1, 2, 3, 4, 5, 6, 7, 8, 9],
			"short": [10, 20],
			"empty": []
		}`

		// Test normal slice
		slice1, err := processor.Get(testData, "data[2:5]")
		helper.AssertNoError(err, "Normal slice should work")
		if sliceArray, ok := slice1.([]any); ok {
			helper.AssertEqual(3, len(sliceArray), "Normal slice should have correct length")
			helper.AssertEqual(float64(2), sliceArray[0], "First slice element should be correct")
			helper.AssertEqual(float64(4), sliceArray[2], "Last slice element should be correct")
		}

		// Test slice from start
		sliceFromStart, err := processor.Get(testData, "data[:3]")
		helper.AssertNoError(err, "Slice from start should work")
		if sliceArray, ok := sliceFromStart.([]any); ok {
			helper.AssertEqual(3, len(sliceArray), "Slice from start should have correct length")
			helper.AssertEqual(float64(0), sliceArray[0], "First element should be correct")
		}

		// Test slice to end
		sliceToEnd, err := processor.Get(testData, "data[7:]")
		helper.AssertNoError(err, "Slice to end should work")
		if sliceArray, ok := sliceToEnd.([]any); ok {
			helper.AssertEqual(3, len(sliceArray), "Slice to end should have correct length")
			helper.AssertEqual(float64(9), sliceArray[2], "Last element should be correct")
		}

		// Test full slice
		fullSlice, err := processor.Get(testData, "data[:]")
		helper.AssertNoError(err, "Full slice should work")
		if sliceArray, ok := fullSlice.([]any); ok {
			helper.AssertEqual(10, len(sliceArray), "Full slice should have correct length")
		}

		// Test slice with step
		stepSlice, err := processor.Get(testData, "data[::2]")
		helper.AssertNoError(err, "Step slice should work")
		if sliceArray, ok := stepSlice.([]any); ok {
			helper.AssertEqual(5, len(sliceArray), "Step slice should have correct length")
			helper.AssertEqual(float64(0), sliceArray[0], "First stepped element should be correct")
			helper.AssertEqual(float64(8), sliceArray[4], "Last stepped element should be correct")
		}

		// Test slice with negative indices
		negativeSlice, err := processor.Get(testData, "data[-3:-1]")
		helper.AssertNoError(err, "Negative slice should work")
		if sliceArray, ok := negativeSlice.([]any); ok {
			helper.AssertEqual(2, len(sliceArray), "Negative slice should have correct length")
			helper.AssertEqual(float64(7), sliceArray[0], "First negative slice element should be correct")
		}

		// Test slice out of bounds (should be clamped)
		outOfBoundsSlice, err := processor.Get(testData, "data[5:20]")
		helper.AssertNoError(err, "Out of bounds slice should work")
		if sliceArray, ok := outOfBoundsSlice.([]any); ok {
			helper.AssertEqual(5, len(sliceArray), "Out of bounds slice should be clamped")
		}

		// Test reverse slice (start > end)
		reverseSlice, err := processor.Get(testData, "data[5:2]")
		helper.AssertNoError(err, "Reverse slice should work")
		if sliceArray, ok := reverseSlice.([]any); ok {
			helper.AssertEqual(0, len(sliceArray), "Reverse slice should be empty")
		}

		// Test slice on short array
		shortSlice, err := processor.Get(testData, "short[0:5]")
		helper.AssertNoError(err, "Slice on short array should work")
		if sliceArray, ok := shortSlice.([]any); ok {
			helper.AssertEqual(2, len(sliceArray), "Short array slice should be clamped")
		}

		// Test slice on empty array
		emptySlice, err := processor.Get(testData, "empty[0:5]")
		helper.AssertNoError(err, "Slice on empty array should work")
		if sliceArray, ok := emptySlice.([]any); ok {
			helper.AssertEqual(0, len(sliceArray), "Empty array slice should be empty")
		}
	})

	t.Run("NestedArrayBoundaryConditions", func(t *testing.T) {
		processor := New()
		defer processor.Close()

		testData := `{
			"matrix": [
				[1, 2, 3],
				[4, 5, 6],
				[7, 8, 9]
			],
			"jagged": [
				[1, 2],
				[3, 4, 5, 6],
				[7]
			],
			"mixed": [
				[1, 2, 3],
				{"key": "value"},
				[4, 5]
			]
		}`

		// Test valid nested access
		value, err := processor.Get(testData, "matrix[1][2]")
		helper.AssertNoError(err, "Valid nested access should work")
		helper.AssertEqual(float64(6), value, "Nested value should be correct")

		// Test nested access with negative indices
		negativeNested, err := processor.Get(testData, "matrix[-1][-1]")
		helper.AssertNoError(err, "Negative nested access should work")
		helper.AssertEqual(float64(9), negativeNested, "Negative nested value should be correct")

		// Test nested out of bounds
		nestedOutOfBounds, err := processor.Get(testData, "matrix[1][5]")
		if err == nil {
			helper.AssertNil(nestedOutOfBounds, "Nested out of bounds should return nil")
		}

		// Test jagged array access
		jaggedValue, err := processor.Get(testData, "jagged[1][3]")
		helper.AssertNoError(err, "Jagged array access should work")
		helper.AssertEqual(float64(6), jaggedValue, "Jagged array value should be correct")

		// Test jagged array out of bounds
		jaggedOutOfBounds, err := processor.Get(testData, "jagged[2][1]")
		if err == nil {
			helper.AssertNil(jaggedOutOfBounds, "Jagged array out of bounds should return nil")
		}

		// Test mixed array types
		mixedObject, err := processor.Get(testData, "mixed[1].key")
		helper.AssertNoError(err, "Mixed array object access should work")
		helper.AssertEqual("value", mixedObject, "Mixed array object value should be correct")

		// Test accessing array element as object (should fail gracefully)
		invalidAccess, err := processor.Get(testData, "mixed[0].key")
		if err == nil {
			helper.AssertNil(invalidAccess, "Invalid object access on array should return nil")
		}
	})

	t.Run("ArrayModificationBoundaryConditions", func(t *testing.T) {
		processor := New()
		defer processor.Close()

		testData := `{
			"numbers": [1, 2, 3, 4, 5],
			"empty": [],
			"nested": [[1, 2], [3, 4]]
		}`

		// Test setting valid index
		result, err := processor.Set(testData, "numbers[2]", 99)
		helper.AssertNoError(err, "Setting valid index should work")

		value, err := processor.Get(result, "numbers[2]")
		helper.AssertNoError(err, "Getting modified value should work")
		helper.AssertEqual(float64(99), value, "Modified value should be correct")

		// Test setting with negative index
		result2, err := processor.Set(testData, "numbers[-1]", 88)
		helper.AssertNoError(err, "Setting negative index should work")

		lastValue, err := processor.Get(result2, "numbers[4]")
		helper.AssertNoError(err, "Getting last modified value should work")
		helper.AssertEqual(float64(88), lastValue, "Last modified value should be correct")

		// Test setting out of bounds with SetWithAdd (should work with auto-expansion)
		result6, err := SetWithAdd(testData, "numbers[10]", 77)
		helper.AssertNoError(err, "Setting out of bounds with SetWithAdd should work (auto-expansion)")
		if err == nil {
			expandedValue, err := processor.Get(result6, "numbers[10]")
			helper.AssertNoError(err, "Getting expanded value should work")
			helper.AssertEqual(float64(77), expandedValue, "Expanded value should be correct")

			// Verify array was expanded with null values in between
			expandedArray, err := processor.Get(result6, "numbers")
			helper.AssertNoError(err, "Getting expanded array should work")
			if arr, ok := expandedArray.([]any); ok {
				helper.AssertEqual(11, len(arr), "Array should be expanded to length 11")
				helper.AssertEqual(nil, arr[5], "Gap should be filled with null")
				helper.AssertEqual(nil, arr[9], "Gap should be filled with null")
			}
		}

		// Test setting in empty array with SetWithAdd (should work with auto-expansion)
		result7, err := SetWithAdd(testData, "empty[0]", 123)
		helper.AssertNoError(err, "Setting in empty array with SetWithAdd should work (auto-expansion)")
		if err == nil {
			emptyValue, err := processor.Get(result7, "empty[0]")
			helper.AssertNoError(err, "Getting value from expanded empty array should work")
			helper.AssertEqual(float64(123), emptyValue, "Value in expanded empty array should be correct")
		}

		// Test setting adjacent index (should work)
		result3, err := SetWithAdd(testData, "numbers[5]", 66)
		if err == nil {
			extendedValue, err := processor.Get(result3, "numbers[5]")
			if err == nil {
				helper.AssertEqual(float64(66), extendedValue, "Adjacent index value should be correct")
			}
		}

		// Test setting nested array element
		result5, err := processor.Set(testData, "nested[0][1]", 999)
		helper.AssertNoError(err, "Setting nested array element should work")

		nestedValue, err := processor.Get(result5, "nested[0][1]")
		helper.AssertNoError(err, "Getting nested modified value should work")
		helper.AssertEqual(float64(999), nestedValue, "Nested modified value should be correct")
	})

	t.Run("ArrayDeletionBoundaryConditions", func(t *testing.T) {
		processor := New()
		defer processor.Close()

		testData := `{
			"numbers": [1, 2, 3, 4, 5],
			"single": [100],
			"nested": [[1, 2], [3, 4], [5, 6]]
		}`

		// Test deleting valid index
		result, err := processor.Delete(testData, "numbers[2]")
		helper.AssertNoError(err, "Deleting valid index should work")

		// Verify deletion
		deletedValue, err := processor.Get(result, "numbers[2]")
		if err == nil {
			// After deletion, index 2 should now contain the next element (4)
			helper.AssertEqual(float64(4), deletedValue, "After deletion, next element should shift")
		}

		// Test deleting with negative index
		result2, err := processor.Delete(testData, "numbers[-1]")
		helper.AssertNoError(err, "Deleting negative index should work")

		// Verify array length decreased
		remainingArray, err := processor.Get(result2, "numbers")
		if err == nil {
			if arr, ok := remainingArray.([]any); ok {
				helper.AssertEqual(4, len(arr), "Array length should decrease after deletion")
			}
		}

		// Test deleting out of bounds (should return error)
		_, err = processor.Delete(testData, "numbers[10]")
		helper.AssertError(err, "Deleting out of bounds should return error")

		// Test deleting negative out of bounds (should return error)
		_, err = processor.Delete(testData, "numbers[-10]")
		helper.AssertError(err, "Deleting negative out of bounds should return error")

		// Test deleting from single element array
		result4, err := processor.Delete(testData, "single[0]")
		helper.AssertNoError(err, "Deleting from single element array should work")

		singleArray, err := processor.Get(result4, "single")
		if err == nil {
			if arr, ok := singleArray.([]any); ok {
				helper.AssertEqual(0, len(arr), "Single element array should become empty")
			}
		}

		// Test deleting nested array element
		result5, err := processor.Delete(testData, "nested[1][0]")
		helper.AssertNoError(err, "Deleting nested array element should work")

		nestedArray, err := processor.Get(result5, "nested[1]")
		if err == nil {
			if arr, ok := nestedArray.([]any); ok {
				helper.AssertEqual(1, len(arr), "Nested array length should decrease")
				helper.AssertEqual(float64(4), arr[0], "Remaining element should be correct")
			}
		}
	})
}
