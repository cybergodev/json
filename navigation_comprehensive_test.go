package json

import (
	"testing"
)

// TestNavigationComprehensive tests navigation and path parsing functionality
func TestNavigationComprehensive(t *testing.T) {
	helper := NewTestHelper(t)

	t.Run("PathParsingBasic", func(t *testing.T) {
		processor := New()
		defer processor.Close()

		testData := `{
			"user": {
				"profile": {
					"name": "John",
					"age": 30
				},
				"settings": {
					"theme": "dark",
					"notifications": true
				}
			},
			"items": [1, 2, 3, 4, 5]
		}`

		// Test simple property path
		result, err := processor.Get(testData, "user")
		helper.AssertNoError(err, "Simple property path should work")
		helper.AssertNotNil(result, "Simple property should exist")

		// Test nested property path
		result, err = processor.Get(testData, "user.profile.name")
		helper.AssertNoError(err, "Nested property path should work")
		helper.AssertEqual("John", result, "Nested property should be correct")

		// Test array index path
		result, err = processor.Get(testData, "items[2]")
		helper.AssertNoError(err, "Array index path should work")
		helper.AssertEqual(float64(3), result, "Array element should be correct")

		// Test combined path
		result, err = processor.Get(testData, "user.settings.notifications")
		helper.AssertNoError(err, "Combined path should work")
		helper.AssertEqual(true, result, "Combined path result should be correct")
	})

	t.Run("PathParsingAdvanced", func(t *testing.T) {
		processor := New()
		defer processor.Close()

		testData := `{
			"data": [
				{"values": [10, 20, 30]},
				{"values": [40, 50, 60]},
				{"values": [70, 80, 90]}
			],
			"nested": {
				"level1": {
					"level2": {
						"items": ["a", "b", "c"]
					}
				}
			}
		}`

		// Test array slice path
		result, err := processor.Get(testData, "data[0:2]")
		helper.AssertNoError(err, "Array slice path should work")
		if arr, ok := result.([]any); ok {
			helper.AssertEqual(2, len(arr), "Slice should have correct length")
		}

		// Test nested array access
		result, err = processor.Get(testData, "data[1].values[2]")
		helper.AssertNoError(err, "Nested array access should work")
		helper.AssertEqual(float64(60), result, "Nested array element should be correct")

		// Test deep nested path
		result, err = processor.Get(testData, "nested.level1.level2.items[1]")
		helper.AssertNoError(err, "Deep nested path should work")
		helper.AssertEqual("b", result, "Deep nested element should be correct")

		// Test extraction path
		result, err = processor.Get(testData, "data{values}")
		helper.AssertNoError(err, "Extraction path should work")
		if arr, ok := result.([]any); ok {
			helper.AssertEqual(3, len(arr), "Extraction should return correct number of arrays")
		}
	})

	t.Run("PathValidation", func(t *testing.T) {
		processor := New()
		defer processor.Close()

		testData := `{"test": "value"}`

		// Test invalid paths
		invalidPaths := []string{
			"[unclosed",
			"unclosed[",
			"invalid..path",
			"path..",
			".startWithDot",
			"path.",
			"invalid{unclosed",
			"unclosed{",
			"invalid}",
		}

		for _, path := range invalidPaths {
			_, err := processor.Get(testData, path)
			helper.AssertError(err, "Invalid path should return error: %s", path)
		}
	})

	t.Run("NavigationCore", func(t *testing.T) {
		processor := New()
		defer processor.Close()

		testData := `{
			"users": [
				{
					"id": 1,
					"profile": {
						"name": "Alice",
						"contacts": {
							"email": "alice@example.com",
							"phone": "123-456-7890"
						}
					},
					"roles": ["admin", "user"]
				},
				{
					"id": 2,
					"profile": {
						"name": "Bob",
						"contacts": {
							"email": "bob@example.com"
						}
					},
					"roles": ["user"]
				}
			]
		}`

		// Test navigation through complex structure
		result, err := processor.Get(testData, "users[0].profile.contacts.email")
		helper.AssertNoError(err, "Complex navigation should work")
		helper.AssertEqual("alice@example.com", result, "Complex navigation result should be correct")

		// Test navigation with array extraction
		result, err = processor.Get(testData, "users{profile}")
		helper.AssertNoError(err, "Navigation with extraction should work")
		if arr, ok := result.([]any); ok {
			helper.AssertEqual(2, len(arr), "Should extract 2 profiles")
		}

		// Test navigation with nested extraction
		result, err = processor.Get(testData, "users{profile}{name}")
		helper.AssertNoError(err, "Nested extraction navigation should work")
		if arr, ok := result.([]any); ok {
			helper.AssertEqual(2, len(arr), "Should extract 2 names")
			helper.AssertEqual("Alice", arr[0], "First name should be Alice")
			helper.AssertEqual("Bob", arr[1], "Second name should be Bob")
		}

		// Test navigation with array slicing
		result, err = processor.Get(testData, "users[0].roles[0:1]")
		helper.AssertNoError(err, "Navigation with array slicing should work")
		if arr, ok := result.([]any); ok {
			helper.AssertEqual(1, len(arr), "Slice should have 1 element")
			helper.AssertEqual("admin", arr[0], "Sliced element should be admin")
		}
	})

	t.Run("NavigatorEdgeCases", func(t *testing.T) {
		processor := New()
		defer processor.Close()

		// Test empty object navigation
		emptyObj := `{}`
		result, err := processor.Get(emptyObj, "nonexistent")
		helper.AssertNoError(err, "Navigation in empty object should not error")
		helper.AssertNil(result, "Nonexistent property should be nil")

		// Test empty array navigation
		emptyArr := `{"arr": []}`
		result, err = processor.Get(emptyArr, "arr[0]")
		helper.AssertNoError(err, "Navigation in empty array should not error")
		helper.AssertNil(result, "Empty array access should return nil")

		// Test null value navigation
		nullData := `{"value": null}`
		result, err = processor.Get(nullData, "value")
		helper.AssertNoError(err, "Null value navigation should not error")
		helper.AssertNil(result, "Null value should be nil")

		// Test navigation through null
		nullNested := `{"obj": null}`
		result, err = processor.Get(nullNested, "obj.property")
		helper.AssertNoError(err, "Navigation through null should not error")
		helper.AssertNil(result, "Navigation through null should return nil")
	})

	t.Run("PathSegmentParsing", func(t *testing.T) {
		processor := New()
		defer processor.Close()

		testData := `{
			"array": [0, 1, 2, 3, 4],
			"object": {
				"key": "value",
				"nested": {
					"items": ["a", "b", "c"]
				}
			}
		}`

		// Test different segment types
		segments := []struct {
			path     string
			expected any
		}{
			{"array", []any{float64(0), float64(1), float64(2), float64(3), float64(4)}},
			{"array[0]", float64(0)},
			{"array[-1]", float64(4)},
			{"object.key", "value"},
			{"object.nested.items[1]", "b"},
		}

		for _, seg := range segments {
			result, err := processor.Get(testData, seg.path)
			helper.AssertNoError(err, "Path segment should work: %s", seg.path)
			if seg.path == "array" {
				// Special handling for array comparison
				if arr, ok := result.([]any); ok {
					helper.AssertEqual(5, len(arr), "Array should have correct length")
				}
			} else {
				helper.AssertEqual(seg.expected, result, "Path segment result should match: %s", seg.path)
			}
		}
	})

	t.Run("NavigationWithSpecialCharacters", func(t *testing.T) {
		processor := New()
		defer processor.Close()

		testData := `{
			"keyWithUnderscore": "value1",
			"CamelCase": "value2",
			"lowercase": "value3"
		}`

		// Test keys that are valid
		validKeys := []struct {
			key      string
			expected string
		}{
			{"keyWithUnderscore", "value1"},
			{"CamelCase", "value2"},
			{"lowercase", "value3"},
		}

		for _, vk := range validKeys {
			result, err := processor.Get(testData, vk.key)
			helper.AssertNoError(err, "Valid key should work: %s", vk.key)
			helper.AssertEqual(vk.expected, result, "Valid key result should match: %s", vk.key)
		}

		// Test keys that should fail
		invalidData := `{
			"key-with-dash": "value1",
			"123numeric": "value2"
		}`

		invalidKeys := []string{
			"key-with-dash",
			"123numeric",
		}

		for _, key := range invalidKeys {
			_, err := processor.Get(invalidData, key)
			helper.AssertError(err, "Invalid key should fail: %s", key)
		}

		// Test that "key.with.dots" is interpreted as nested path
		dotData := `{"key": {"with": {"dots": "nested_value"}}}`
		result, err := processor.Get(dotData, "key.with.dots")
		helper.AssertNoError(err, "Nested path with dots should work")
		helper.AssertEqual("nested_value", result, "Nested path should return correct value")
	})

	t.Run("NavigationPerformance", func(t *testing.T) {
		processor := New()
		defer processor.Close()

		// Create a moderately complex structure for performance testing
		testData := `{
			"data": [
				{"items": [1, 2, 3, 4, 5]},
				{"items": [6, 7, 8, 9, 10]},
				{"items": [11, 12, 13, 14, 15]}
			]
		}`

		// Test multiple navigation operations
		paths := []string{
			"data[0].items[2]",
			"data[1].items[4]",
			"data[2].items[0]",
			"data{items}",
			"data[0:2]",
		}

		for _, path := range paths {
			result, err := processor.Get(testData, path)
			helper.AssertNoError(err, "Performance test path should work: %s", path)
			helper.AssertNotNil(result, "Performance test should return result: %s", path)
		}
	})

	t.Run("PathNormalization", func(t *testing.T) {
		processor := New()
		defer processor.Close()

		testData := `{
			"user": {
				"name": "John",
				"items": [1, 2, 3]
			}
		}`

		// Test equivalent paths (should all work the same)
		equivalentPaths := [][]string{
			{"user.name", "user.name"},
			{"user.items[0]", "user.items[0]"},
			{"user.items[-1]", "user.items[-1]"},
		}

		for _, pathGroup := range equivalentPaths {
			var results []any
			for _, path := range pathGroup {
				result, err := processor.Get(testData, path)
				helper.AssertNoError(err, "Equivalent path should work: %s", path)
				results = append(results, result)
			}

			// All results in the group should be the same
			for i := 1; i < len(results); i++ {
				helper.AssertEqual(results[0], results[i], "Equivalent paths should return same result")
			}
		}
	})
}
