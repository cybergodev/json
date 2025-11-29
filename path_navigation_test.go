package json

import (
	"testing"
)

// TestPathNavigation tests path parsing and navigation functionality
func TestPathNavigation(t *testing.T) {
	helper := NewTestHelper(t)

	testData := `{
		"company": {
			"name": "TechCorp",
			"departments": [
				{
					"name": "Engineering",
					"teams": [
						{
							"name": "Backend",
							"members": [
								{"name": "Alice", "skills": ["Go", "Python"], "level": "Senior"},
								{"name": "Bob", "skills": ["Java", "Spring"], "level": "Mid"}
							]
						}
					]
				}
			]
		},
		"items": [1, 2, 3, 4, 5]
	}`

	t.Run("BasicPathAccess", func(t *testing.T) {
		name, err := GetString(testData, "company.name")
		helper.AssertNoError(err)
		helper.AssertEqual("TechCorp", name)

		deptName, err := GetString(testData, "company.departments[0].name")
		helper.AssertNoError(err)
		helper.AssertEqual("Engineering", deptName)
	})

	t.Run("ArrayAccess", func(t *testing.T) {
		first, err := GetInt(testData, "items[0]")
		helper.AssertNoError(err)
		helper.AssertEqual(1, first)

		last, err := GetInt(testData, "items[-1]")
		helper.AssertNoError(err)
		helper.AssertEqual(5, last)

		outOfBounds, err := Get(testData, "items[100]")
		helper.AssertNoError(err)
		helper.AssertNil(outOfBounds)
	})

	t.Run("ArraySlicing", func(t *testing.T) {
		slice, err := Get(testData, "items[1:4]")
		helper.AssertNoError(err)
		expected := []any{float64(2), float64(3), float64(4)}
		helper.AssertEqual(expected, slice)

		fromStart, err := Get(testData, "items[3:]")
		helper.AssertNoError(err)
		if arr, ok := fromStart.([]any); ok {
			helper.AssertEqual(2, len(arr))
		}

		toEnd, err := Get(testData, "items[:2]")
		helper.AssertNoError(err)
		if arr, ok := toEnd.([]any); ok {
			helper.AssertEqual(2, len(arr))
		}
	})

	t.Run("NestedArrayAccess", func(t *testing.T) {
		memberName, err := GetString(testData, "company.departments[0].teams[0].members[0].name")
		helper.AssertNoError(err)
		helper.AssertEqual("Alice", memberName)

		skills, err := GetArray(testData, "company.departments[0].teams[0].members[0].skills")
		helper.AssertNoError(err)
		helper.AssertEqual(2, len(skills))
	})

	t.Run("Extraction", func(t *testing.T) {
		names, err := Get(testData, "company.departments[0].teams[0].members{name}")
		helper.AssertNoError(err)
		if arr, ok := names.([]any); ok {
			helper.AssertEqual(2, len(arr))
			helper.AssertEqual("Alice", arr[0])
			helper.AssertEqual("Bob", arr[1])
		}
	})

	t.Run("PathValidation", func(t *testing.T) {
		helper.AssertTrue(IsValidPath("user.name"))
		helper.AssertTrue(IsValidPath("items[0]"))
		helper.AssertTrue(IsValidPath("data[1:5]"))
		helper.AssertTrue(IsValidPath("users{name}"))
		helper.AssertFalse(IsValidPath(""))
		helper.AssertTrue(IsValidPath("."))
	})

	t.Run("PathTypeDetection", func(t *testing.T) {
		helper.AssertEqual("root", getPathType("."))
		helper.AssertEqual("dot_notation_simple", getPathType("user.name"))
		helper.AssertEqual("dot_notation_complex", getPathType("items[0]"))
		helper.AssertEqual("dot_notation_complex", getPathType("data[1:5]"))
		helper.AssertEqual("dot_notation_complex", getPathType("users{name}"))
	})

	t.Run("InvalidPaths", func(t *testing.T) {
		_, err := Get(testData, "items[abc]")
		helper.AssertError(err)

		_, err = Get(testData, "items[]")
		helper.AssertError(err)

		_, err = Get(testData, "nonexistent.path")
		helper.AssertError(err)
	})
}

// TestDeepExtraction tests deep extraction functionality
func TestDeepExtraction(t *testing.T) {
	helper := NewTestHelper(t)

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
							{"name": "Eve", "role": "Manager"}
						]
					}
				]
			}
		]
	}`

	t.Run("ConsecutiveExtractions", func(t *testing.T) {
		processor := New()
		defer processor.Close()

		result, err := processor.Get(testData, "departments{teams}{members}{name}")
		helper.AssertNoError(err)
		if arr, ok := result.([]any); ok {
			helper.AssertTrue(len(arr) > 0)
		}
	})

	t.Run("MixedExtractionAndArray", func(t *testing.T) {
		processor := New()
		defer processor.Close()

		result, err := processor.Get(testData, "departments{teams}[0].name")
		helper.AssertNoError(err)
		helper.AssertNotNil(result)
	})

	t.Run("ExtractionFromArray", func(t *testing.T) {
		processor := New()
		defer processor.Close()

		result, err := processor.Get(testData, "departments[0].teams{name}")
		helper.AssertNoError(err)
		if arr, ok := result.([]any); ok {
			helper.AssertEqual(2, len(arr))
		}
	})
}

// TestPathExpressionPerformance tests performance of path expressions
func TestPathExpressionPerformance(t *testing.T) {
	helper := NewTestHelper(t)
	generator := NewTestDataGenerator()

	t.Run("SimplePathPerformance", func(t *testing.T) {
		jsonData := generator.GenerateComplexJSON()

		for i := 0; i < 100; i++ {
			_, err := GetString(jsonData, "users[0].name")
			helper.AssertNoError(err)
		}
	})

	t.Run("ComplexPathPerformance", func(t *testing.T) {
		jsonData := generator.GenerateComplexJSON()

		for i := 0; i < 100; i++ {
			_, err := Get(jsonData, "users[0].profile.preferences.languages")
			helper.AssertNoError(err)
		}
	})
}
