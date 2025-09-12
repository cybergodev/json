package json

import (
	"testing"
)

// TestDeepExtractionComprehensive tests deep extraction functionality
func TestDeepExtractionComprehensive(t *testing.T) {
	helper := NewTestHelper(t)

	t.Run("HandleDeepExtraction", func(t *testing.T) {
		processor := New()
		defer processor.Close()

		testData := `{
			"departments": [
				{
					"name": "Engineering",
					"teams": [
						{
							"name": "Backend",
							"members": [
								{"name": "Alice", "role": "Senior"},
								{"name": "Bob", "role": "Junior"}
							]
						},
						{
							"name": "Frontend", 
							"members": [
								{"name": "Charlie", "role": "Senior"},
								{"name": "Diana", "role": "Mid"}
							]
						}
					]
				},
				{
					"name": "Marketing",
					"teams": [
						{
							"name": "Digital",
							"members": [
								{"name": "Eve", "role": "Manager"},
								{"name": "Frank", "role": "Analyst"}
							]
						}
					]
				}
			]
		}`

		// Test consecutive extractions
		result, err := processor.Get(testData, "departments{teams}{members}{name}")
		helper.AssertNoError(err, "Consecutive extractions should work")
		if arr, ok := result.([]any); ok {
			helper.AssertTrue(len(arr) > 0, "Should extract member names")
			t.Logf("Extracted names: %v", arr)
		}

		// Test mixed extraction operations
		result, err = processor.Get(testData, "departments{teams}[0].name")
		helper.AssertNoError(err, "Mixed extraction with array access should work")
		t.Logf("Mixed extraction result: %v", result)

		// Test flat extraction
		result, err = processor.Get(testData, "departments{flat:teams}{flat:members}{name}")
		helper.AssertNoError(err, "Flat extraction should work")
		if arr, ok := result.([]any); ok {
			t.Logf("Flat extraction result: %v", arr)
		}
	})

	t.Run("PerformConsecutiveExtractions", func(t *testing.T) {
		processor := New()
		defer processor.Close()

		testData := `{
			"groups": [
				{
					"items": [
						{"values": [1, 2, 3]},
						{"values": [4, 5, 6]}
					]
				},
				{
					"items": [
						{"values": [7, 8, 9]},
						{"values": [10, 11, 12]}
					]
				}
			]
		}`

		// Test multiple consecutive extractions
		result, err := processor.Get(testData, "groups{items}{values}")
		helper.AssertNoError(err, "Multiple consecutive extractions should work")
		if arr, ok := result.([]any); ok {
			helper.AssertTrue(len(arr) > 0, "Should extract values")
			t.Logf("Consecutive extraction result: %v", arr)
		}
	})

	t.Run("HandlePostExtractionSlice", func(t *testing.T) {
		processor := New()
		defer processor.Close()

		testData := `{
			"data": [
				{"items": ["a", "b", "c", "d", "e"]},
				{"items": ["f", "g", "h", "i", "j"]},
				{"items": ["k", "l", "m", "n", "o"]}
			]
		}`

		// Test extraction followed by slicing
		result, err := processor.Get(testData, "data{items}[0:2]")
		helper.AssertNoError(err, "Post-extraction slice should work")
		if arr, ok := result.([]any); ok {
			helper.AssertTrue(len(arr) >= 2, "Should have at least 2 items after slice")
			t.Logf("Post-extraction slice result: %v", arr)
		}

		// Test extraction with negative slice
		result, err = processor.Get(testData, "data{items}[-2:]")
		helper.AssertNoError(err, "Post-extraction negative slice should work")
		if arr, ok := result.([]any); ok {
			t.Logf("Post-extraction negative slice result: %v", arr)
		}
	})

	t.Run("HandleMixedExtractionOperations", func(t *testing.T) {
		processor := New()
		defer processor.Close()

		testData := `{
			"categories": [
				{
					"name": "Electronics",
					"products": [
						{"name": "Phone", "price": 500},
						{"name": "Laptop", "price": 1000}
					]
				},
				{
					"name": "Books",
					"products": [
						{"name": "Novel", "price": 15},
						{"name": "Textbook", "price": 80}
					]
				}
			]
		}`

		// Test extraction mixed with property access
		result, err := processor.Get(testData, "categories{products}[0].name")
		helper.AssertNoError(err, "Mixed extraction with property access should work")
		t.Logf("Mixed extraction result: %v", result)

		// Test extraction mixed with array slicing
		result, err = processor.Get(testData, "categories{products}[0:1]")
		helper.AssertNoError(err, "Mixed extraction with slicing should work")
		if arr, ok := result.([]any); ok {
			t.Logf("Mixed extraction with slice result: %v", arr)
		}
	})

	t.Run("FlatExtraction", func(t *testing.T) {
		processor := New()
		defer processor.Close()

		testData := `{
			"nested": [
				{
					"level1": [
						{"level2": [{"value": "a"}, {"value": "b"}]},
						{"level2": [{"value": "c"}, {"value": "d"}]}
					]
				},
				{
					"level1": [
						{"level2": [{"value": "e"}, {"value": "f"}]},
						{"level2": [{"value": "g"}, {"value": "h"}]}
					]
				}
			]
		}`

		// Test flat extraction to flatten nested structures
		result, err := processor.Get(testData, "nested{flat:level1}{flat:level2}{value}")
		helper.AssertNoError(err, "Flat extraction should work")
		if arr, ok := result.([]any); ok {
			helper.AssertTrue(len(arr) >= 8, "Should flatten all nested values")
			t.Logf("Flat extraction result: %v", arr)
		}

		// Test mixed flat and structured extraction
		result, err = processor.Get(testData, "nested{level1}{flat:level2}{value}")
		helper.AssertNoError(err, "Mixed flat and structured extraction should work")
		if arr, ok := result.([]any); ok {
			t.Logf("Mixed flat/structured extraction result: %v", arr)
		}
	})

	t.Run("ExtractFromArray", func(t *testing.T) {
		processor := New()
		defer processor.Close()

		testData := `{
			"users": [
				{"profile": {"name": "Alice", "age": 25}},
				{"profile": {"name": "Bob", "age": 30}},
				{"profile": {"name": "Charlie", "age": 35}}
			]
		}`

		// Test extraction from array
		result, err := processor.Get(testData, "users{profile}")
		helper.AssertNoError(err, "Extract from array should work")
		if arr, ok := result.([]any); ok {
			helper.AssertEqual(3, len(arr), "Should extract 3 profiles")
			t.Logf("Extract from array result: %v", arr)
		}

		// Test nested extraction from array
		result, err = processor.Get(testData, "users{profile}{name}")
		helper.AssertNoError(err, "Nested extract from array should work")
		if arr, ok := result.([]any); ok {
			helper.AssertEqual(3, len(arr), "Should extract 3 names")
			helper.AssertEqual("Alice", arr[0], "First name should be Alice")
		}
	})

	t.Run("ExtractFromObject", func(t *testing.T) {
		processor := New()
		defer processor.Close()

		testData := `{
			"company": {
				"departments": {
					"engineering": {"head": "Alice", "count": 10},
					"marketing": {"head": "Bob", "count": 5},
					"sales": {"head": "Charlie", "count": 8}
				}
			}
		}`

		// Test extraction from object
		result, err := processor.Get(testData, "company.departments{head}")
		helper.AssertNoError(err, "Extract from object should work")
		if arr, ok := result.([]any); ok {
			helper.AssertEqual(3, len(arr), "Should extract 3 heads")
			t.Logf("Extract from object result: %v", arr)
		}
	})

	t.Run("IsDeepExtractionPath", func(t *testing.T) {
		processor := New()
		defer processor.Close()

		// Test paths that should be recognized as deep extraction
		deepExtractionPaths := []string{
			"a{b}{c}",
			"users{profile}{name}",
			"data{items}[0]",
			"nested{flat:values}",
		}

		for _, path := range deepExtractionPaths {
			// Test that these paths work (indirect test of isDeepExtractionPath)
			testData := `{"a": [{"b": [{"c": "value"}]}]}`
			_, err := processor.Get(testData, path)
			// Some paths might not match the test data, but they shouldn't crash
			t.Logf("Testing deep extraction path: %s, error: %v", path, err)
		}
	})

	t.Run("NavigateWithDeepExtraction", func(t *testing.T) {
		processor := New()
		defer processor.Close()

		testData := `{
			"root": [
				{
					"branch": [
						{"leaf": "value1"},
						{"leaf": "value2"}
					]
				},
				{
					"branch": [
						{"leaf": "value3"},
						{"leaf": "value4"}
					]
				}
			]
		}`

		// Test navigation with deep extraction
		result, err := processor.Get(testData, "root{branch}{leaf}")
		helper.AssertNoError(err, "Navigation with deep extraction should work")
		if arr, ok := result.([]any); ok {
			helper.AssertTrue(len(arr) >= 2, "Should extract leaf values")
			t.Logf("Navigation with deep extraction result: %v", arr)
		}
	})

	t.Run("EdgeCases", func(t *testing.T) {
		processor := New()
		defer processor.Close()

		// Test empty arrays
		emptyData := `{"empty": []}`
		result, err := processor.Get(emptyData, "empty{field}")
		helper.AssertNoError(err, "Empty array extraction should not error")
		if arr, ok := result.([]any); ok {
			helper.AssertEqual(0, len(arr), "Empty array extraction should return empty array")
		}

		// Test missing fields
		missingData := `{"items": [{"name": "test"}]}`
		result, err = processor.Get(missingData, "items{missing}")
		helper.AssertNoError(err, "Missing field extraction should not error")
		if arr, ok := result.([]any); ok {
			t.Logf("Missing field extraction result: %v", arr)
		}

		// Test null values
		nullData := `{"items": [null, {"value": "test"}, null]}`
		result, err = processor.Get(nullData, "items{value}")
		helper.AssertNoError(err, "Null value extraction should not error")
		if arr, ok := result.([]any); ok {
			t.Logf("Null value extraction result: %v", arr)
		}
	})
}
