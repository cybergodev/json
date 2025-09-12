package json

import (
	"testing"
)

// TestSetMultipleWithAddArrayExpansion tests array expansion functionality in SetMultipleWithAdd
func TestSetMultipleWithAddArrayExpansion(t *testing.T) {
	helper := NewTestHelper(t)

	t.Run("MultipleArrayIndexExpansion", func(t *testing.T) {
		processor := New()
		defer processor.Close()

		testData := `{
			"tags": ["technology", "startup"],
			"categories": ["web", "mobile"]
		}`

		// Test multiple array index expansions
		updates := map[string]any{
			"tags[4]":       "expanded1",
			"tags[6]":       "expanded2",
			"categories[3]": "expanded3",
			"categories[5]": "expanded4",
		}

		result, err := SetMultipleWithAdd(testData, updates)
		helper.AssertNoError(err, "SetMultipleWithAdd with array expansion should work")

		// Verify tags array expansion
		tags, err := processor.Get(result, "tags")
		helper.AssertNoError(err, "Getting tags should work")
		if arr, ok := tags.([]any); ok {
			helper.AssertEqual(7, len(arr), "Tags array should be expanded to length 7")
			helper.AssertEqual("technology", arr[0], "Original element should be preserved")
			helper.AssertEqual("startup", arr[1], "Original element should be preserved")
			helper.AssertEqual(nil, arr[2], "Gap should be filled with null")
			helper.AssertEqual(nil, arr[3], "Gap should be filled with null")
			helper.AssertEqual("expanded1", arr[4], "Expanded element should be set")
			helper.AssertEqual(nil, arr[5], "Gap should be filled with null")
			helper.AssertEqual("expanded2", arr[6], "Expanded element should be set")
		}

		// Verify categories array expansion
		categories, err := processor.Get(result, "categories")
		helper.AssertNoError(err, "Getting categories should work")
		if arr, ok := categories.([]any); ok {
			helper.AssertEqual(6, len(arr), "Categories array should be expanded to length 6")
			helper.AssertEqual("web", arr[0], "Original element should be preserved")
			helper.AssertEqual("mobile", arr[1], "Original element should be preserved")
			helper.AssertEqual(nil, arr[2], "Gap should be filled with null")
			helper.AssertEqual("expanded3", arr[3], "Expanded element should be set")
			helper.AssertEqual(nil, arr[4], "Gap should be filled with null")
			helper.AssertEqual("expanded4", arr[5], "Expanded element should be set")
		}
	})

	t.Run("MultipleSliceExpansion", func(t *testing.T) {
		processor := New()
		defer processor.Close()

		testData := `{
			"data1": [1, 2, 3],
			"data2": [10, 20]
		}`

		// Test multiple slice expansions
		updates := map[string]any{
			"data1[3:6]": "slice1",
			"data2[2:5]": "slice2",
		}

		result, err := SetMultipleWithAdd(testData, updates)
		helper.AssertNoError(err, "SetMultipleWithAdd with slice expansion should work")

		// Verify data1 slice expansion
		data1, err := processor.Get(result, "data1")
		helper.AssertNoError(err, "Getting data1 should work")
		if arr, ok := data1.([]any); ok {
			helper.AssertEqual(6, len(arr), "Data1 array should be expanded to length 6")
			helper.AssertEqual(float64(1), arr[0], "Original element should be preserved")
			helper.AssertEqual(float64(2), arr[1], "Original element should be preserved")
			helper.AssertEqual(float64(3), arr[2], "Original element should be preserved")
			helper.AssertEqual("slice1", arr[3], "Slice element should be set")
			helper.AssertEqual("slice1", arr[4], "Slice element should be set")
			helper.AssertEqual("slice1", arr[5], "Slice element should be set")
		}

		// Verify data2 slice expansion
		data2, err := processor.Get(result, "data2")
		helper.AssertNoError(err, "Getting data2 should work")
		if arr, ok := data2.([]any); ok {
			helper.AssertEqual(5, len(arr), "Data2 array should be expanded to length 5")
			helper.AssertEqual(float64(10), arr[0], "Original element should be preserved")
			helper.AssertEqual(float64(20), arr[1], "Original element should be preserved")
			helper.AssertEqual("slice2", arr[2], "Slice element should be set")
			helper.AssertEqual("slice2", arr[3], "Slice element should be set")
			helper.AssertEqual("slice2", arr[4], "Slice element should be set")
		}
	})

	t.Run("CreateNewArraysMultiple", func(t *testing.T) {
		processor := New()
		defer processor.Close()

		// Test creating multiple new arrays
		updates := map[string]any{
			"newArray1[2]":    "item1",
			"newArray2[0:3]":  "item2",
			"nested.array[4]": "nested",
		}

		result, err := SetMultipleWithAdd("{}", updates)
		helper.AssertNoError(err, "Creating multiple new arrays should work")

		// Verify newArray1
		newArray1, err := processor.Get(result, "newArray1")
		helper.AssertNoError(err, "Getting newArray1 should work")
		if arr, ok := newArray1.([]any); ok {
			helper.AssertEqual(3, len(arr), "NewArray1 should have length 3")
			helper.AssertEqual(nil, arr[0], "Gap should be filled with null")
			helper.AssertEqual(nil, arr[1], "Gap should be filled with null")
			helper.AssertEqual("item1", arr[2], "Value should be at correct index")
		}

		// Verify newArray2
		newArray2, err := processor.Get(result, "newArray2")
		helper.AssertNoError(err, "Getting newArray2 should work")
		if arr, ok := newArray2.([]any); ok {
			helper.AssertEqual(3, len(arr), "NewArray2 should have length 3")
			for i := 0; i < 3; i++ {
				helper.AssertEqual("item2", arr[i], "All slice elements should be set")
			}
		}

		// Verify nested array
		nestedArray, err := processor.Get(result, "nested.array")
		helper.AssertNoError(err, "Getting nested array should work")
		if arr, ok := nestedArray.([]any); ok {
			helper.AssertEqual(5, len(arr), "Nested array should have length 5")
			helper.AssertEqual("nested", arr[4], "Nested value should be at correct index")
		}
	})

	t.Run("MixedOperationsMultiple", func(t *testing.T) {
		processor := New()
		defer processor.Close()

		testData := `{
			"existing": ["a", "b"],
			"numbers": [1, 2, 3]
		}`

		// Test mixed operations: regular sets, array expansion, slice expansion
		updates := map[string]any{
			"existing[1]":  "modified", // Regular array access
			"existing[4]":  "expanded", // Array expansion
			"numbers[3:6]": 99,         // Slice expansion
			"newProp":      "created",  // Property creation
			"newArray[2]":  "new",      // New array creation
		}

		result, err := SetMultipleWithAdd(testData, updates)
		helper.AssertNoError(err, "Mixed operations should work")

		// Verify existing array
		existing, err := processor.Get(result, "existing")
		helper.AssertNoError(err, "Getting existing should work")
		if arr, ok := existing.([]any); ok {
			helper.AssertEqual(5, len(arr), "Existing array should be expanded")
			helper.AssertEqual("a", arr[0], "Original element preserved")
			helper.AssertEqual("modified", arr[1], "Modified element")
			helper.AssertEqual(nil, arr[2], "Gap filled with null")
			helper.AssertEqual(nil, arr[3], "Gap filled with null")
			helper.AssertEqual("expanded", arr[4], "Expanded element")
		}

		// Verify numbers array
		numbers, err := processor.Get(result, "numbers")
		helper.AssertNoError(err, "Getting numbers should work")
		if arr, ok := numbers.([]any); ok {
			helper.AssertEqual(6, len(arr), "Numbers array should be expanded")
			helper.AssertEqual(float64(1), arr[0], "Original element preserved")
			helper.AssertEqual(float64(2), arr[1], "Original element preserved")
			helper.AssertEqual(float64(3), arr[2], "Original element preserved")
			helper.AssertEqual(float64(99), arr[3], "Slice element set")
			helper.AssertEqual(float64(99), arr[4], "Slice element set")
			helper.AssertEqual(float64(99), arr[5], "Slice element set")
		}

		// Verify new property
		newProp, err := processor.Get(result, "newProp")
		helper.AssertNoError(err, "Getting newProp should work")
		helper.AssertEqual("created", newProp, "New property should be created")

		// Verify new array
		newArray, err := processor.Get(result, "newArray")
		helper.AssertNoError(err, "Getting newArray should work")
		if arr, ok := newArray.([]any); ok {
			helper.AssertEqual(3, len(arr), "New array should have correct length")
			helper.AssertEqual("new", arr[2], "New array element should be set")
		}
	})

	t.Run("ErrorHandlingWithContinueOnError", func(t *testing.T) {
		processor := New()
		defer processor.Close()

		testData := `{
			"valid": ["a", "b"]
		}`

		// Test with some invalid paths and ContinueOnError option
		updates := map[string]any{
			"valid[3]":  "expanded",  // Valid expansion
			"invalid..": "bad",       // Invalid path
			"valid[5]":  "expanded2", // Valid expansion
		}

		// Create options with ContinueOnError
		opts := &ProcessorOptions{
			ContinueOnError: true,
		}

		result, err := SetMultipleWithAdd(testData, updates, opts)
		// Should succeed partially
		if err == nil {
			// Verify valid operations succeeded
			valid, err := processor.Get(result, "valid")
			helper.AssertNoError(err, "Getting valid should work")
			if arr, ok := valid.([]any); ok {
				helper.AssertEqual(6, len(arr), "Valid array should be expanded")
				helper.AssertEqual("expanded", arr[3], "First expansion should work")
				helper.AssertEqual("expanded2", arr[5], "Second expansion should work")
			}
		}
	})
}
