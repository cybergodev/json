package json

import (
	"testing"
)

// TestCoreJSONOperations tests all core JSON operations comprehensively
// Merged from: core_test.go, comprehensive_core_test.go, processor_test.go
func TestCoreJSONOperations(t *testing.T) {
	helper := NewTestHelper(t)

	jsonStr := `{
		"name": "John Doe",
		"age": 30,
		"active": true,
		"scores": [85, 92, 78],
		"profile": {
			"email": "john@example.com",
			"location": "New York",
			"preferences": {
				"notifications": true,
				"languages": ["en", "es", "fr"]
			}
		}
	}`

	t.Run("GetOperations", func(t *testing.T) {
		// Test string retrieval
		name, err := GetString(jsonStr, "name")
		helper.AssertNoError(err, "GetString should work")
		helper.AssertEqual("John Doe", name, "Name should match")

		// Test nested string retrieval
		email, err := GetString(jsonStr, "profile.email")
		helper.AssertNoError(err, "GetString nested should work")
		helper.AssertEqual("john@example.com", email, "Email should match")

		// Test integer retrieval
		age, err := GetInt(jsonStr, "age")
		helper.AssertNoError(err, "GetInt should work")
		helper.AssertEqual(30, age, "Age should match")

		// Test array element
		score, err := GetInt(jsonStr, "scores[0]")
		helper.AssertNoError(err, "GetInt array should work")
		helper.AssertEqual(85, score, "First score should match")

		// Test boolean retrieval
		active, err := GetBool(jsonStr, "active")
		helper.AssertNoError(err, "GetBool should work")
		helper.AssertTrue(active, "Active should be true")

		// Test nested boolean
		notifications, err := GetBool(jsonStr, "profile.preferences.notifications")
		helper.AssertNoError(err, "GetBool nested should work")
		helper.AssertTrue(notifications, "Notifications should be true")

		// Test array retrieval
		scores, err := GetArray(jsonStr, "scores")
		helper.AssertNoError(err, "GetArray should work")
		helper.AssertEqual(3, len(scores), "Should have 3 scores")

		// Test nested array
		languages, err := GetArray(jsonStr, "profile.preferences.languages")
		helper.AssertNoError(err, "GetArray nested should work")
		helper.AssertEqual(3, len(languages), "Should have 3 languages")
	})

	t.Run("SetOperations", func(t *testing.T) {
		// Test setting simple value
		newJSON, err := Set(jsonStr, "city", "San Francisco")
		helper.AssertNoError(err, "Set should work")

		city, err := GetString(newJSON, "city")
		helper.AssertNoError(err, "Get after Set should work")
		helper.AssertEqual("San Francisco", city, "City should match")

		// Test setting nested value
		newJSON, err = Set(newJSON, "profile.phone", "555-1234")
		helper.AssertNoError(err, "Set nested should work")

		phone, err := GetString(newJSON, "profile.phone")
		helper.AssertNoError(err, "Get nested after Set should work")
		helper.AssertEqual("555-1234", phone, "Phone should match")

		// Test setting array element
		newJSON, err = Set(newJSON, "scores[0]", 95)
		helper.AssertNoError(err, "Set array element should work")

		newScore, err := GetInt(newJSON, "scores[0]")
		helper.AssertNoError(err, "Get array element after Set should work")
		helper.AssertEqual(95, newScore, "New score should match")
	})

	t.Run("DeleteOperations", func(t *testing.T) {
		// Test deleting simple field
		newJSON, err := Delete(jsonStr, "age")
		helper.AssertNoError(err, "Delete should work")

		result, err := Get(newJSON, "age")
		helper.AssertError(err, "Get deleted field should return error")
		helper.AssertNil(result, "Deleted field should be nil")

		// Test deleting nested field
		newJSON, err = Delete(newJSON, "profile.location")
		helper.AssertNoError(err, "Delete nested should work")

		result, err = Get(newJSON, "profile.location")
		helper.AssertError(err, "Get deleted nested field should return error")
		helper.AssertNil(result, "Deleted nested field should be nil")

		// Test deleting array element
		newJSON, err = Delete(newJSON, "scores[1]")
		helper.AssertNoError(err, "Delete array element should work")

		scores, err := GetArray(newJSON, "scores")
		helper.AssertNoError(err, "Get array after delete should work")
		helper.AssertEqual(2, len(scores), "Array should have 2 elements after delete")
	})
}

// TestArrayOperations tests array-specific operations
func TestArrayOperations(t *testing.T) {
	helper := NewTestHelper(t)

	jsonStr := `{
		"numbers": [1, 2, 3, 4, 5, 6, 7, 8, 9, 10],
		"users": [
			{"name": "Alice", "age": 25},
			{"name": "Bob", "age": 30},
			{"name": "Charlie", "age": 35}
		]
	}`

	t.Run("ArrayIndex", func(t *testing.T) {
		// Positive index
		first, err := GetInt(jsonStr, "numbers[0]")
		helper.AssertNoError(err, "Should get first element")
		helper.AssertEqual(1, first, "First element should be 1")

		// Negative index
		last, err := GetInt(jsonStr, "numbers[-1]")
		helper.AssertNoError(err, "Should get last element")
		helper.AssertEqual(10, last, "Last element should be 10")

		// Nested array access
		userName, err := GetString(jsonStr, "users[1].name")
		helper.AssertNoError(err, "Should get nested array element")
		helper.AssertEqual("Bob", userName, "User name should match")
	})

	t.Run("ArraySlicing", func(t *testing.T) {
		// Basic slice
		slice, err := Get(jsonStr, "numbers[1:4]")
		helper.AssertNoError(err, "Should get array slice")
		expected := []any{float64(2), float64(3), float64(4)}
		helper.AssertEqual(expected, slice, "Slice should match")

		// Slice with step
		stepSlice, err := Get(jsonStr, "numbers[::2]")
		helper.AssertNoError(err, "Should get array slice with step")
		if stepResult, ok := stepSlice.([]any); ok {
			helper.AssertEqual(5, len(stepResult), "Step slice should have 5 elements")
		}

		// Negative slice
		negSlice, err := Get(jsonStr, "numbers[-3:]")
		helper.AssertNoError(err, "Should get negative slice")
		expected = []any{float64(8), float64(9), float64(10)}
		helper.AssertEqual(expected, negSlice, "Negative slice should match")
	})
}

// TestJSONEncoding tests JSON encoding
func TestJSONEncoding(t *testing.T) {
	helper := NewTestHelper(t)

	t.Run("BasicEncoding", func(t *testing.T) {
		data := map[string]any{
			"name":   "Alice",
			"age":    25,
			"active": true,
		}

		jsonStr, err := Encode(data)
		helper.AssertNoError(err, "Encode should work")

		// Verify we can parse it back
		name, err := GetString(jsonStr, "name")
		helper.AssertNoError(err, "Should get name from encoded JSON")
		helper.AssertEqual("Alice", name, "Name should match")
	})
}

// TestProcessor tests processor functionality
func TestProcessor(t *testing.T) {
	helper := NewTestHelper(t)

	t.Run("ProcessorBasic", func(t *testing.T) {
		processor := New()
		defer processor.Close()

		jsonStr := `{"test": "value", "number": 42}`

		value, err := processor.Get(jsonStr, "test")
		helper.AssertNoError(err, "Processor Get should work")
		helper.AssertEqual("value", value, "Value should match")

		number, err := processor.Get(jsonStr, "number")
		helper.AssertNoError(err, "Processor Get number should work")
		helper.AssertEqual(float64(42), number, "Number should match")
	})

	t.Run("ProcessorWithConfig", func(t *testing.T) {
		config := DefaultConfig()
		config.EnableCache = true
		config.MaxCacheSize = 100

		processor := New(config)
		defer processor.Close()

		jsonStr := `{"cached": "data"}`

		// First access
		value1, err := processor.Get(jsonStr, "cached")
		helper.AssertNoError(err, "First access should work")

		// Second access (should use cache)
		value2, err := processor.Get(jsonStr, "cached")
		helper.AssertNoError(err, "Second access should work")

		helper.AssertEqual(value1, value2, "Cached values should be equal")
	})
}

// TestPathExpressions tests comprehensive path expression features
func TestPathExpressions(t *testing.T) {
	helper := NewTestHelper(t)

	complexData := `{
		"company": {
			"name": "TechCorp",
			"departments": [
				{
					"name": "Engineering",
					"budget": 1000000,
					"teams": [
						{
							"name": "Backend",
							"members": [
								{"name": "Alice", "salary": 120000, "skills": ["Go", "Python"]},
								{"name": "Bob", "salary": 90000, "skills": ["Java", "React"]}
							]
						},
						{
							"name": "Frontend",
							"members": [
								{"name": "Charlie", "salary": 85000, "skills": ["React", "TypeScript"]},
								{"name": "Diana", "salary": 95000, "skills": ["Vue", "JavaScript"]}
							]
						}
					]
				},
				{
					"name": "Marketing",
					"budget": 500000,
					"teams": [
						{
							"name": "Digital",
							"members": [
								{"name": "Eve", "salary": 75000, "skills": ["SEO", "Content"]}
							]
						}
					]
				}
			]
		},
		"metadata": {
			"tags": ["technology", "tech", "startup"],
			"founded": 2020
		}
	}`

	t.Run("BasicPathAccess", func(t *testing.T) {
		// Test simple dot notation
		companyName, err := GetString(complexData, "company.name")
		helper.AssertNoError(err, "Should get company name")
		helper.AssertEqual("TechCorp", companyName, "Company name should match")

		// Test array index access
		firstDeptName, err := GetString(complexData, "company.departments[0].name")
		helper.AssertNoError(err, "Should get first department name")
		helper.AssertEqual("Engineering", firstDeptName, "First department should be Engineering")

		// Test negative array index
		lastTag, err := GetString(complexData, "metadata.tags[-1]")
		helper.AssertNoError(err, "Should get last tag")
		helper.AssertEqual("startup", lastTag, "Last tag should be startup")
	})

	t.Run("ArraySlicingOperations", func(t *testing.T) {
		// Test basic slice
		firstTwoTags, err := Get(complexData, "metadata.tags[0:2]")
		helper.AssertNoError(err, "Should get first two tags")
		expected := []any{"technology", "tech"}
		helper.AssertEqual(expected, firstTwoTags, "First two tags should match")

		// Test slice with step
		everyOtherTag, err := Get(complexData, "metadata.tags[::2]")
		helper.AssertNoError(err, "Should get every other tag")
		expectedStep := []any{"technology", "startup"}
		helper.AssertEqual(expectedStep, everyOtherTag, "Every other tag should match")

		// Test negative slice
		lastTwoTags, err := Get(complexData, "metadata.tags[-2:]")
		helper.AssertNoError(err, "Should get last two tags")
		expectedLast := []any{"tech", "startup"}
		helper.AssertEqual(expectedLast, lastTwoTags, "Last two tags should match")
	})

	t.Run("ExtractionSyntax", func(t *testing.T) {
		// Test basic extraction
		allDeptNames, err := Get(complexData, "company.departments{name}")
		helper.AssertNoError(err, "Should extract all department names")
		expectedNames := []any{"Engineering", "Marketing"}
		helper.AssertEqual(expectedNames, allDeptNames, "Department names should match")

		// Test nested extraction
		allTeamNames, err := Get(complexData, "company.departments{teams}{name}")
		helper.AssertNoError(err, "Should extract all team names")
		if teamNames, ok := allTeamNames.([]any); ok {
			helper.AssertTrue(len(teamNames) > 0, "Should have team names")
			// Verify structure: should be array of arrays
			if firstDeptTeams, ok := teamNames[0].([]any); ok {
				helper.AssertEqual("Backend", firstDeptTeams[0], "First team should be Backend")
				helper.AssertEqual("Frontend", firstDeptTeams[1], "Second team should be Frontend")
			}
		}
	})

	t.Run("FlatExtractionSyntax", func(t *testing.T) {
		// Test flat extraction
		allMemberNames, err := Get(complexData, "company.departments{flat:teams}{flat:members}{name}")
		helper.AssertNoError(err, "Should extract all member names with flat syntax")
		if names, ok := allMemberNames.([]any); ok {
			expectedCount := 5 // Alice, Bob, Charlie, Diana, Eve
			helper.AssertEqual(expectedCount, len(names), "Should have 5 member names")
			helper.AssertEqual("Alice", names[0], "First member should be Alice")
		}

		// Test flat extraction of skills
		allSkills, err := Get(complexData, "company.departments{flat:teams}{flat:members}{flat:skills}")
		helper.AssertNoError(err, "Should extract all skills with flat syntax")
		if skills, ok := allSkills.([]any); ok {
			helper.AssertTrue(len(skills) >= 8, "Should have at least 8 skills")
		}
	})
}
