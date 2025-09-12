package json

import (
	"testing"
)

// TestAdvancedPathNavigation tests comprehensive path parsing and navigation functionality
func TestAdvancedPathNavigation(t *testing.T) {
	helper := NewTestHelper(t)

	t.Run("ComplexPathParsing", func(t *testing.T) {
		processor := New()
		defer processor.Close()

		testData := `{
			"users": [
				{
					"profile": {
						"personal": {
							"name": "Alice",
							"contacts": [
								{"type": "email", "value": "alice@example.com"},
								{"type": "phone", "value": "+1-555-0101"}
							]
						},
						"professional": {
							"company": "TechCorp",
							"projects": [
								{"name": "API Gateway", "status": "active"},
								{"name": "Database Migration", "status": "completed"}
							]
						}
					}
				},
				{
					"profile": {
						"personal": {
							"name": "Bob",
							"contacts": [
								{"type": "email", "value": "bob@example.com"}
							]
						},
						"professional": {
							"company": "DataInc",
							"projects": [
								{"name": "Analytics Dashboard", "status": "active"}
							]
						}
					}
				}
			]
		}`

		// Test deep nested path navigation
		name, err := processor.Get(testData, "users[0].profile.personal.name")
		helper.AssertNoError(err, "Deep nested path should work")
		helper.AssertEqual("Alice", name, "Deep nested value should be correct")

		// Test array access within nested structure
		email, err := processor.Get(testData, "users[0].profile.personal.contacts[0].value")
		helper.AssertNoError(err, "Array access in nested structure should work")
		helper.AssertEqual("alice@example.com", email, "Nested array value should be correct")

		// Test complex path with multiple array accesses
		projectStatus, err := processor.Get(testData, "users[1].profile.professional.projects[0].status")
		helper.AssertNoError(err, "Multiple array access should work")
		helper.AssertEqual("active", projectStatus, "Complex path value should be correct")

		// Test negative index navigation
		lastContact, err := processor.Get(testData, "users[0].profile.personal.contacts[-1].type")
		helper.AssertNoError(err, "Negative index navigation should work")
		helper.AssertEqual("phone", lastContact, "Negative index value should be correct")
	})

	t.Run("PathSyntaxVariations", func(t *testing.T) {
		processor := New()
		defer processor.Close()

		testData := `{
			"data": {
				"items": [
					{"id": 1, "values": [10, 20, 30]},
					{"id": 2, "values": [40, 50, 60]},
					{"id": 3, "values": [70, 80, 90]}
				],
				"metadata": {
					"total": 9,
					"categories": ["A", "B", "C"]
				}
			}
		}`

		// Test dot notation
		total, err := processor.Get(testData, "data.metadata.total")
		helper.AssertNoError(err, "Dot notation should work")
		helper.AssertEqual(float64(9), total, "Dot notation value should be correct")

		// Test standard array access
		value, err := processor.Get(testData, "data.items[1].values[2]")
		helper.AssertNoError(err, "Standard array access should work")
		helper.AssertEqual(float64(60), value, "Array access value should be correct")

		// Test alternative path format
		total2, err := processor.Get(testData, "data.metadata.total")
		helper.AssertNoError(err, "Alternative path should work")
		helper.AssertEqual(float64(9), total2, "Alternative path value should be correct")

		// Test array slicing
		slice, err := processor.Get(testData, "data.items[0:2]")
		helper.AssertNoError(err, "Array slicing should work")
		if sliceArray, ok := slice.([]any); ok {
			helper.AssertEqual(2, len(sliceArray), "Slice should have correct length")
		}

		// Test array slicing with step
		categories, err := processor.Get(testData, "data.metadata.categories[::2]")
		helper.AssertNoError(err, "Array slicing with step should work")
		if catArray, ok := categories.([]any); ok {
			helper.AssertEqual(2, len(catArray), "Stepped slice should have correct length")
			helper.AssertEqual("A", catArray[0], "First stepped element should be correct")
			helper.AssertEqual("C", catArray[1], "Second stepped element should be correct")
		}
	})

	t.Run("ExtractionPathNavigation", func(t *testing.T) {
		processor := New()
		defer processor.Close()

		testData := `{
			"departments": [
				{
					"name": "Engineering",
					"employees": [
						{"name": "Alice", "skills": ["Go", "Python"], "level": "Senior"},
						{"name": "Bob", "skills": ["JavaScript", "React"], "level": "Mid"}
					]
				},
				{
					"name": "Marketing",
					"employees": [
						{"name": "Charlie", "skills": ["Analytics", "SEO"], "level": "Senior"},
						{"name": "Diana", "skills": ["Content", "Social"], "level": "Junior"}
					]
				}
			]
		}`

		// Test basic extraction
		names, err := processor.Get(testData, "departments{employees}{name}")
		helper.AssertNoError(err, "Basic extraction should work")
		if nameArray, ok := names.([]any); ok {
			helper.AssertTrue(len(nameArray) >= 2, "Should extract names from multiple departments")
		}

		// Test extraction with array access
		firstDeptEmployees, err := processor.Get(testData, "departments[0]{employees}")
		helper.AssertNoError(err, "Extraction with array access should work")
		if empArray, ok := firstDeptEmployees.([]any); ok {
			helper.AssertEqual(2, len(empArray), "Should extract employees from first department")
		}

		// Test flat extraction
		allSkills, err := processor.Get(testData, "departments{flat:employees}{skills}")
		helper.AssertNoError(err, "Flat extraction should work")
		if skillsArray, ok := allSkills.([]any); ok {
			helper.AssertTrue(len(skillsArray) >= 4, "Should flatten and extract all skills")
		}

		// Test extraction followed by array access
		firstEmployeeName, err := processor.Get(testData, "departments{employees}[0].name")
		helper.AssertNoError(err, "Extraction followed by array access should work")
		helper.AssertNotNil(firstEmployeeName, "Should get first employee from each department")
	})

	t.Run("PathValidationAndErrorHandling", func(t *testing.T) {
		processor := New()
		defer processor.Close()

		testData := `{
			"data": {
				"items": [1, 2, 3],
				"info": {"count": 3}
			}
		}`

		// Test invalid array index
		_, err := processor.Get(testData, "data.items[abc]")
		helper.AssertError(err, "Invalid array index should return error")

		// Test out of bounds access
		result, err := processor.Get(testData, "data.items[10]")
		if err == nil {
			helper.AssertNil(result, "Out of bounds access should return nil")
		}

		// Test invalid property access
		result, err = processor.Get(testData, "data.nonexistent.property")
		if err == nil {
			helper.AssertNil(result, "Invalid property access should return nil")
		}

		// Test malformed path
		_, err = processor.Get(testData, "data.items[")
		helper.AssertError(err, "Malformed path should return error")

		// Test empty path
		result, err = processor.Get(testData, "")
		helper.AssertNoError(err, "Empty path should return root")
		helper.AssertNotNil(result, "Empty path should return data")

		// Test path with invalid characters
		_, err = processor.Get(testData, "data..items")
		helper.AssertError(err, "Path with double dots should return error")
	})

	t.Run("SpecialCharacterHandling", func(t *testing.T) {
		processor := New()
		defer processor.Close()

		testData := `{
			"userinfo": {
				"first_name": "John",
				"lastname": "Doe",
				"emaildomain": "john@example.com",
				"datawithspaces": "test value",
				"unicode_data": "中文测试",
				"emoji_key": "emoji value"
			},
			"arraydata": [
				{"keywithdash": "value1"},
				{"key_with_underscore": "value2"}
			]
		}`

		// Test property with underscore
		firstName, err := processor.Get(testData, "userinfo.first_name")
		helper.AssertNoError(err, "Property with underscore should work")
		helper.AssertEqual("John", firstName, "Underscore property value should be correct")

		// Test simple property access
		lastName, err := processor.Get(testData, "userinfo.lastname")
		helper.AssertNoError(err, "Simple property should work")
		helper.AssertEqual("Doe", lastName, "Simple property value should be correct")

		// Test property with complex content
		email, err := processor.Get(testData, "userinfo.emaildomain")
		helper.AssertNoError(err, "Property with complex content should work")
		helper.AssertEqual("john@example.com", email, "Complex content should be correct")

		// Test Unicode content
		unicodeValue, err := processor.Get(testData, "userinfo.unicode_data")
		helper.AssertNoError(err, "Unicode content should work")
		helper.AssertEqual("中文测试", unicodeValue, "Unicode content should be correct")

		// Test emoji content
		emojiValue, err := processor.Get(testData, "userinfo.emoji_key")
		helper.AssertNoError(err, "Emoji content should work")
		helper.AssertEqual("emoji value", emojiValue, "Emoji content should be correct")

		// Test array with underscore properties
		underscoreValue, err := processor.Get(testData, "arraydata[1].key_with_underscore")
		helper.AssertNoError(err, "Array with underscore property should work")
		helper.AssertEqual("value2", underscoreValue, "Array underscore property value should be correct")
	})

	t.Run("PathNormalizationAndOptimization", func(t *testing.T) {
		processor := New()
		defer processor.Close()

		testData := `{
			"level1": {
				"level2": {
					"level3": {
						"data": [
							{"value": 100},
							{"value": 200},
							{"value": 300}
						]
					}
				}
			}
		}`

		// Test standard path
		path1 := "level1.level2.level3.data[1].value"

		value1, err1 := processor.Get(testData, path1)

		helper.AssertNoError(err1, "Standard path should work")
		helper.AssertEqual(float64(200), value1, "Standard path value should be correct")

		// Test path with redundant segments
		value3, err3 := processor.Get(testData, "level1.level2.level3.data[1].value")
		helper.AssertNoError(err3, "Path should work")
		helper.AssertEqual(float64(200), value3, "Path value should be correct")
	})

	t.Run("JSONPointerNavigation", func(t *testing.T) {
		processor := New()
		defer processor.Close()

		testData := `{
			"foo": ["bar", "baz"],
			"": {
				"": "empty key value"
			},
			"a/b": 1,
			"c%d": 2,
			"e^f": 3,
			"g|h": 4,
			"i\\j": 5,
			"k\"l": 6,
			" ": 7,
			"m~n": 8
		}`

		// Test basic JSON Pointer
		value, err := processor.Get(testData, "/foo/0")
		helper.AssertNoError(err, "JSON Pointer should work")
		helper.AssertEqual("bar", value, "JSON Pointer value should be correct")

		// Test simple JSON Pointer paths
		arrayValue, err := processor.Get(testData, "foo[1]")
		helper.AssertNoError(err, "Array access should work")
		helper.AssertEqual("baz", arrayValue, "Array value should be correct")

		// Test accessing properties with special characters using standard notation
		spaceValue, err := processor.Get(testData, " ")
		if err == nil {
			helper.AssertEqual(float64(7), spaceValue, "Space key value should be correct")
		}

		// Test other special character properties
		percentValue, err := processor.Get(testData, "c%d")
		if err == nil {
			helper.AssertEqual(float64(2), percentValue, "Percent key value should be correct")
		}
	})

	t.Run("PathPerformanceAndCaching", func(t *testing.T) {
		processor := New()
		defer processor.Close()

		// Create a simple nested structure for performance testing
		testData := `{
			"performance": {
				"data": {
					"item0": {"id": 0, "values": [10, 20, 30]},
					"item1": {"id": 1, "values": [11, 21, 31]},
					"item2": {"id": 2, "values": [12, 22, 32]},
					"item3": {"id": 3, "values": [13, 23, 33]},
					"item4": {"id": 4, "values": [14, 24, 34]},
					"item5": {"id": 5, "values": [15, 25, 35]}
				}
			}
		}`

		// Test repeated access to same path (should benefit from caching)
		path := "performance.data.item5.values[1]"

		// First access
		value1, err1 := processor.Get(testData, path)
		helper.AssertNoError(err1, "First access should work")
		helper.AssertEqual(float64(25), value1, "First access value should be correct")

		// Second access (should be faster due to caching)
		value2, err2 := processor.Get(testData, path)
		helper.AssertNoError(err2, "Second access should work")
		helper.AssertEqual(value1, value2, "Cached value should be same")

		// Test accessing different items
		path3 := "performance.data.item3.values[2]"
		value3, err3 := processor.Get(testData, path3)
		helper.AssertNoError(err3, "Different path should work")
		helper.AssertEqual(float64(33), value3, "Different path value should be correct")
	})
}
