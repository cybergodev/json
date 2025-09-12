package json

import (
	"testing"
)

// TestArrayExpansionFeatures tests the array expansion functionality added to SetWithAdd
func TestArrayExpansionFeatures(t *testing.T) {
	helper := NewTestHelper(t)

	t.Run("SingleIndexExpansion", func(t *testing.T) {
		processor := New()
		defer processor.Close()

		testData := `{
			"tags": ["technology", "startup", "innovation", "growth"]
		}`

		// Test basic index expansion
		result, err := SetWithAdd(testData, "tags[4]", "haha")
		helper.AssertNoError(err, "Basic index expansion should work")

		value, err := processor.Get(result, "tags[4]")
		helper.AssertNoError(err, "Getting expanded value should work")
		helper.AssertEqual("haha", value, "Expanded value should be correct")

		// Verify array length
		tags, err := processor.Get(result, "tags")
		helper.AssertNoError(err, "Getting tags array should work")
		if arr, ok := tags.([]any); ok {
			helper.AssertEqual(5, len(arr), "Array should be expanded to length 5")
		}

		// Test large index expansion
		result2, err := SetWithAdd(testData, "tags[10]", "future")
		helper.AssertNoError(err, "Large index expansion should work")

		value2, err := processor.Get(result2, "tags[10]")
		helper.AssertNoError(err, "Getting large expanded value should work")
		helper.AssertEqual("future", value2, "Large expanded value should be correct")

		// Verify gaps are filled with null
		tags2, err := processor.Get(result2, "tags")
		helper.AssertNoError(err, "Getting expanded tags array should work")
		if arr, ok := tags2.([]any); ok {
			helper.AssertEqual(11, len(arr), "Array should be expanded to length 11")
			helper.AssertEqual(nil, arr[5], "Gap should be filled with null")
			helper.AssertEqual(nil, arr[9], "Gap should be filled with null")
		}
	})

	t.Run("SliceExpansion", func(t *testing.T) {
		processor := New()
		defer processor.Close()

		testData := `{
			"tags": ["technology", "startup", "innovation", "growth"]
		}`

		// Test basic slice expansion
		result, err := SetWithAdd(testData, "tags[1:5]", "modified")
		helper.AssertNoError(err, "Basic slice expansion should work")

		tags, err := processor.Get(result, "tags")
		helper.AssertNoError(err, "Getting slice expanded array should work")
		if arr, ok := tags.([]any); ok {
			helper.AssertEqual(5, len(arr), "Array should be expanded to length 5")
			helper.AssertEqual("technology", arr[0], "First element should be unchanged")
			helper.AssertEqual("modified", arr[1], "Slice elements should be modified")
			helper.AssertEqual("modified", arr[4], "Last slice element should be modified")
		}

		// Test slice expansion with step
		result2, err := SetWithAdd(testData, "tags[1:10:2]", "step")
		helper.AssertNoError(err, "Step slice expansion should work")

		tags2, err := processor.Get(result2, "tags")
		helper.AssertNoError(err, "Getting step expanded array should work")
		if arr, ok := tags2.([]any); ok {
			helper.AssertEqual(10, len(arr), "Array should be expanded to length 10")
			helper.AssertEqual("technology", arr[0], "First element should be unchanged")
			helper.AssertEqual("step", arr[1], "Step element should be set")
			helper.AssertEqual("innovation", arr[2], "Non-step element should be unchanged")
			helper.AssertEqual("step", arr[3], "Step element should be set")
			helper.AssertEqual(nil, arr[4], "Gap should be filled with null")
			helper.AssertEqual("step", arr[5], "Step element should be set")
		}
	})

	t.Run("CreateNewArrays", func(t *testing.T) {
		processor := New()
		defer processor.Close()

		// Test creating new array with index
		result, err := SetWithAdd("{}", "newArray[3]", "new-item")
		helper.AssertNoError(err, "Creating new array with index should work")

		value, err := processor.Get(result, "newArray[3]")
		helper.AssertNoError(err, "Getting new array value should work")
		helper.AssertEqual("new-item", value, "New array value should be correct")

		newArray, err := processor.Get(result, "newArray")
		helper.AssertNoError(err, "Getting new array should work")
		if arr, ok := newArray.([]any); ok {
			helper.AssertEqual(4, len(arr), "New array should have length 4")
			helper.AssertEqual(nil, arr[0], "Gap should be filled with null")
			helper.AssertEqual("new-item", arr[3], "Value should be at correct index")
		}

		// Test creating new array with slice
		result2, err := SetWithAdd("{}", "newSlice[0:5]", "slice-item")
		helper.AssertNoError(err, "Creating new array with slice should work")

		newSlice, err := processor.Get(result2, "newSlice")
		helper.AssertNoError(err, "Getting new slice array should work")
		if arr, ok := newSlice.([]any); ok {
			helper.AssertEqual(5, len(arr), "New slice array should have length 5")
			for i := 0; i < 5; i++ {
				helper.AssertEqual("slice-item", arr[i], "All slice elements should be set")
			}
		}
	})

	t.Run("NestedArrayExpansion", func(t *testing.T) {
		processor := New()
		defer processor.Close()

		testData := `{
			"data": {
				"items": [{"name": "item1"}, {"name": "item2"}]
			}
		}`

		// Test nested array expansion
		result, err := SetWithAdd(testData, "data.items[5]", map[string]any{"name": "item6"})
		helper.AssertNoError(err, "Nested array expansion should work")

		value, err := processor.Get(result, "data.items[5].name")
		helper.AssertNoError(err, "Getting nested expanded value should work")
		helper.AssertEqual("item6", value, "Nested expanded value should be correct")

		items, err := processor.Get(result, "data.items")
		helper.AssertNoError(err, "Getting nested expanded array should work")
		if arr, ok := items.([]any); ok {
			helper.AssertEqual(6, len(arr), "Nested array should be expanded to length 6")
			helper.AssertEqual(nil, arr[2], "Gap should be filled with null")
			helper.AssertEqual(nil, arr[4], "Gap should be filled with null")
		}
	})

	t.Run("MixedOperations", func(t *testing.T) {
		processor := New()
		defer processor.Close()

		testData := `{"items": ["a", "b"]}`

		// First expand with slice
		step1, err := SetWithAdd(testData, "items[2:5]", "slice")
		helper.AssertNoError(err, "Slice expansion should work")

		items1, err := processor.Get(step1, "items")
		helper.AssertNoError(err, "Getting slice expanded array should work")
		if arr, ok := items1.([]any); ok {
			helper.AssertEqual(5, len(arr), "Array should be expanded to length 5")
			helper.AssertEqual("slice", arr[2], "Slice element should be set")
		}

		// Then expand with index
		step2, err := SetWithAdd(step1, "items[7]", "index")
		helper.AssertNoError(err, "Index expansion should work")

		items2, err := processor.Get(step2, "items")
		helper.AssertNoError(err, "Getting index expanded array should work")
		if arr, ok := items2.([]any); ok {
			helper.AssertEqual(8, len(arr), "Array should be expanded to length 8")
			helper.AssertEqual("index", arr[7], "Index element should be set")
			helper.AssertEqual(nil, arr[5], "Gap should be filled with null")
		}
	})
}
