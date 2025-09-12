package json

import (
	"testing"
)

// TestArrayOperationsAdvanced tests advanced array operations functionality
func TestArrayOperationsAdvanced(t *testing.T) {
	helper := NewTestHelper(t)

	t.Run("NewArrayOperations", func(t *testing.T) {
		processor := New()
		defer processor.Close()

		// Test that array operations are properly initialized
		// This tests the NewArrayOperations function indirectly
		testData := `{"arr": [1, 2, 3, 4, 5]}`

		result, err := processor.Get(testData, "arr[2]")
		helper.AssertNoError(err, "Array operations should be initialized")
		helper.AssertEqual(float64(3), result, "Should get correct element")
	})

	t.Run("HandleArrayAccess", func(t *testing.T) {
		processor := New()
		defer processor.Close()

		testData := `{
			"numbers": [100, 200, 300, 400, 500],
			"strings": ["alpha", "beta", "gamma", "delta"],
			"booleans": [true, false, true, false],
			"nulls": [null, "value", null],
			"mixed": [1, "two", 3.14, true, null, {"key": "value"}]
		}`

		// Test basic array access
		result, err := processor.Get(testData, "numbers[0]")
		helper.AssertNoError(err, "Should access first element")
		helper.AssertEqual(float64(100), result, "Should get first element")

		result, err = processor.Get(testData, "numbers[4]")
		helper.AssertNoError(err, "Should access last element")
		helper.AssertEqual(float64(500), result, "Should get last element")

		// Test negative index handling
		result, err = processor.Get(testData, "numbers[-1]")
		helper.AssertNoError(err, "Should handle negative index")
		helper.AssertEqual(float64(500), result, "Should get last element with negative index")

		result, err = processor.Get(testData, "numbers[-5]")
		helper.AssertNoError(err, "Should handle negative index")
		helper.AssertEqual(float64(100), result, "Should get first element with negative index")

		// Test bounds validation
		result, err = processor.Get(testData, "numbers[10]")
		helper.AssertNoError(err, "Out of bounds should not error")
		helper.AssertNil(result, "Out of bounds should return nil")

		result, err = processor.Get(testData, "numbers[-10]")
		helper.AssertNoError(err, "Negative out of bounds should not error")
		helper.AssertNil(result, "Negative out of bounds should return nil")

		// Test different data types
		result, err = processor.Get(testData, "strings[1]")
		helper.AssertNoError(err, "Should access string element")
		helper.AssertEqual("beta", result, "Should get string element")

		result, err = processor.Get(testData, "booleans[0]")
		helper.AssertNoError(err, "Should access boolean element")
		helper.AssertEqual(true, result, "Should get boolean element")

		result, err = processor.Get(testData, "nulls[0]")
		helper.AssertNoError(err, "Should access null element")
		helper.AssertNil(result, "Should get nil for null element")

		result, err = processor.Get(testData, "mixed[5].key")
		helper.AssertNoError(err, "Should access object in mixed array")
		helper.AssertEqual("value", result, "Should get object property from mixed array")
	})

	t.Run("HandleArraySlice", func(t *testing.T) {
		processor := New()
		defer processor.Close()

		testData := `{
			"range": [0, 1, 2, 3, 4, 5, 6, 7, 8, 9],
			"letters": ["a", "b", "c", "d", "e"],
			"single": [42],
			"empty": []
		}`

		// Test basic slicing
		result, err := processor.Get(testData, "range[2:5]")
		helper.AssertNoError(err, "Basic slice should work")
		if arr, ok := result.([]any); ok {
			helper.AssertEqual(3, len(arr), "Slice should have correct length")
			helper.AssertEqual(float64(2), arr[0], "First element should be correct")
			helper.AssertEqual(float64(4), arr[2], "Last element should be correct")
		}

		// Test slice with step (if supported)
		result, err = processor.Get(testData, "range[1:8:2]")
		if err == nil {
			if arr, ok := result.([]any); ok {
				// If step is supported, verify the result
				t.Logf("Step slicing result: %v", arr)
			}
		}

		// Test negative indices in slicing
		result, err = processor.Get(testData, "range[-4:-1]")
		helper.AssertNoError(err, "Negative slice should work")
		if arr, ok := result.([]any); ok {
			helper.AssertEqual(3, len(arr), "Negative slice should have correct length")
			helper.AssertEqual(float64(6), arr[0], "First element should be correct")
		}

		// Test open-ended slices
		result, err = processor.Get(testData, "range[:3]")
		helper.AssertNoError(err, "Open start slice should work")
		if arr, ok := result.([]any); ok {
			helper.AssertEqual(3, len(arr), "Open start slice should have correct length")
			helper.AssertEqual(float64(0), arr[0], "Should start from beginning")
		}

		result, err = processor.Get(testData, "range[7:]")
		helper.AssertNoError(err, "Open end slice should work")
		if arr, ok := result.([]any); ok {
			helper.AssertEqual(3, len(arr), "Open end slice should have correct length")
			helper.AssertEqual(float64(7), arr[0], "Should start from index 7")
		}

		// Test full slice
		result, err = processor.Get(testData, "letters[:]")
		helper.AssertNoError(err, "Full slice should work")
		if arr, ok := result.([]any); ok {
			helper.AssertEqual(5, len(arr), "Full slice should have all elements")
		}

		// Test single element array
		result, err = processor.Get(testData, "single[:]")
		helper.AssertNoError(err, "Single element slice should work")
		if arr, ok := result.([]any); ok {
			helper.AssertEqual(1, len(arr), "Single element slice should have 1 element")
			helper.AssertEqual(float64(42), arr[0], "Should get the single element")
		}

		// Test empty array slice
		result, err = processor.Get(testData, "empty[:]")
		helper.AssertNoError(err, "Empty array slice should work")
		if arr, ok := result.([]any); ok {
			helper.AssertEqual(0, len(arr), "Empty array slice should have 0 elements")
		}

		// Test invalid slice ranges
		result, err = processor.Get(testData, "range[5:2]")
		helper.AssertNoError(err, "Invalid range should not error")
		if arr, ok := result.([]any); ok {
			helper.AssertEqual(0, len(arr), "Invalid range should return empty array")
		}
	})

	t.Run("ParseArrayIndex", func(t *testing.T) {
		processor := New()
		defer processor.Close()

		testData := `{"test": [10, 20, 30]}`

		// Test valid indices
		testCases := []struct {
			path     string
			expected any
		}{
			{"test[0]", float64(10)},
			{"test[1]", float64(20)},
			{"test[2]", float64(30)},
		}

		for _, tc := range testCases {
			result, err := processor.Get(testData, tc.path)
			helper.AssertNoError(err, "Valid index should work: %s", tc.path)
			helper.AssertEqual(tc.expected, result, "Should get correct value for: %s", tc.path)
		}

		// Test invalid indices (some should error, some should return nil)
		errorCases := []string{
			"test[abc]",
			"test[1.5]",
			"test[]",
		}

		for _, path := range errorCases {
			_, err := processor.Get(testData, path)
			helper.AssertError(err, "Invalid index should error: %s", path)
		}

		// Test out of bounds indices (should return nil, not error)
		outOfBoundsCases := []string{
			"test[999]",
			"test[-999]",
		}

		for _, path := range outOfBoundsCases {
			result, err := processor.Get(testData, path)
			helper.AssertNoError(err, "Out of bounds should not error: %s", path)
			helper.AssertNil(result, "Out of bounds should return nil: %s", path)
		}
	})

	t.Run("HandleNegativeIndex", func(t *testing.T) {
		processor := New()
		defer processor.Close()

		testData := `{"arr": [1, 2, 3, 4, 5]}`

		// Test all negative indices
		negativeTests := []struct {
			index    string
			expected float64
		}{
			{"-1", 5}, // last element
			{"-2", 4}, // second to last
			{"-3", 3}, // middle
			{"-4", 2}, // second
			{"-5", 1}, // first
		}

		for _, test := range negativeTests {
			path := "arr[" + test.index + "]"
			result, err := processor.Get(testData, path)
			helper.AssertNoError(err, "Negative index should work: %s", path)
			helper.AssertEqual(test.expected, result, "Should get correct value for: %s", path)
		}
	})

	t.Run("ValidateArrayBounds", func(t *testing.T) {
		processor := New()
		defer processor.Close()

		testData := `{"arr": [1, 2, 3]}`

		// Test valid bounds
		validIndices := []string{"0", "1", "2"}
		for _, index := range validIndices {
			path := "arr[" + index + "]"
			result, err := processor.Get(testData, path)
			helper.AssertNoError(err, "Valid bounds should work: %s", path)
			helper.AssertNotNil(result, "Valid bounds should return value: %s", path)
		}

		// Test invalid bounds
		invalidIndices := []string{"3", "10", "-4", "-10"}
		for _, index := range invalidIndices {
			path := "arr[" + index + "]"
			result, err := processor.Get(testData, path)
			helper.AssertNoError(err, "Invalid bounds should not error: %s", path)
			helper.AssertNil(result, "Invalid bounds should return nil: %s", path)
		}
	})

	t.Run("ComplexArrayOperations", func(t *testing.T) {
		processor := New()
		defer processor.Close()

		testData := `{
			"nested": [
				[1, 2, 3],
				[4, 5, 6],
				[7, 8, 9]
			],
			"objects": [
				{"id": 1, "values": [10, 20]},
				{"id": 2, "values": [30, 40]},
				{"id": 3, "values": [50, 60]}
			]
		}`

		// Test nested array operations
		result, err := processor.Get(testData, "nested[1][2]")
		helper.AssertNoError(err, "Nested array access should work")
		helper.AssertEqual(float64(6), result, "Should get correct nested value")

		// Test array slice of nested arrays
		result, err = processor.Get(testData, "nested[0:2]")
		helper.AssertNoError(err, "Slice of nested arrays should work")
		if arr, ok := result.([]any); ok {
			helper.AssertEqual(2, len(arr), "Should get 2 nested arrays")
		}

		// Test complex object array access
		result, err = processor.Get(testData, "objects[1].values[0]")
		helper.AssertNoError(err, "Complex object array access should work")
		helper.AssertEqual(float64(30), result, "Should get correct complex value")
	})
}
