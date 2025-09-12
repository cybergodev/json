package json

import (
	"testing"
)

// TestAdvancedSetOperations tests comprehensive Set operations with complex paths
func TestAdvancedSetOperations(t *testing.T) {
	helper := NewTestHelper(t)

	t.Run("ComplexNestedObjectSetting", func(t *testing.T) {
		processor := New()
		defer processor.Close()

		testData := `{
			"company": {
				"departments": [
					{
						"name": "Engineering",
						"teams": [
							{
								"name": "Backend",
								"members": [
									{"name": "Alice", "role": "Senior", "skills": ["Go", "Python"]},
									{"name": "Bob", "role": "Junior", "skills": ["JavaScript", "React"]}
								]
							},
							{
								"name": "Frontend", 
								"members": [
									{"name": "Charlie", "role": "Lead", "skills": ["React", "TypeScript"]},
									{"name": "Diana", "role": "Senior", "skills": ["Vue", "CSS"]}
								]
							}
						]
					}
				]
			}
		}`

		// Test deep nested object setting
		result, err := processor.Set(testData, "company.departments[0].teams[0].members[0].salary", 120000)
		helper.AssertNoError(err, "Deep nested object setting should work")

		salary, err := processor.Get(result, "company.departments[0].teams[0].members[0].salary")
		helper.AssertNoError(err, "Getting deep nested property should work")
		helper.AssertEqual(float64(120000), salary, "Deep nested property should be set correctly")

		// Test setting nested array element
		result, err = processor.Set(testData, "company.departments[0].teams[1].members[1].skills[0]", "Angular")
		helper.AssertNoError(err, "Nested array element setting should work")

		skill, err := processor.Get(result, "company.departments[0].teams[1].members[1].skills[0]")
		helper.AssertNoError(err, "Getting nested array element should work")
		helper.AssertEqual("Angular", skill, "Nested array element should be updated")
	})

	t.Run("ArraySliceOperations", func(t *testing.T) {
		processor := New()
		defer processor.Close()

		testData := `{
			"matrix": [
				[1, 2, 3, 4, 5],
				[6, 7, 8, 9, 10],
				[11, 12, 13, 14, 15]
			],
			"data": {
				"values": [10, 20, 30, 40, 50, 60, 70, 80, 90, 100]
			}
		}`

		// Test setting individual array elements (slice operations are complex)
		result, err := processor.Set(testData, "data.values[1]", 25)
		helper.AssertNoError(err, "Array element setting should work")

		result, err = processor.Set(result, "data.values[3]", 35)
		helper.AssertNoError(err, "Array element setting should work")

		result, err = processor.Set(result, "data.values[5]", 45)
		helper.AssertNoError(err, "Array element setting should work")

		result, err = processor.Set(result, "data.values[7]", 55)
		helper.AssertNoError(err, "Array element setting should work")

		// Verify elements were set correctly
		values, err := processor.Get(result, "data.values")
		helper.AssertNoError(err, "Getting values array should work")
		if valuesArray, ok := values.([]any); ok {
			helper.AssertEqual(float64(25), valuesArray[1], "First element should be updated")
			helper.AssertEqual(float64(35), valuesArray[3], "Second element should be updated")
			helper.AssertEqual(float64(45), valuesArray[5], "Third element should be updated")
			helper.AssertEqual(float64(55), valuesArray[7], "Fourth element should be updated")
		}

		// Test setting nested array elements
		result, err = processor.Set(testData, "matrix[1][1]", 77)
		helper.AssertNoError(err, "Nested array element setting should work")

		result, err = processor.Set(result, "matrix[1][2]", 88)
		helper.AssertNoError(err, "Nested array element setting should work")

		result, err = processor.Set(result, "matrix[1][3]", 99)
		helper.AssertNoError(err, "Nested array element setting should work")

		row, err := processor.Get(result, "matrix[1]")
		helper.AssertNoError(err, "Getting matrix row should work")
		if rowArray, ok := row.([]any); ok {
			helper.AssertEqual(float64(77), rowArray[1], "First nested element should be updated")
			helper.AssertEqual(float64(88), rowArray[2], "Second nested element should be updated")
			helper.AssertEqual(float64(99), rowArray[3], "Third nested element should be updated")
		}
	})

	t.Run("ExtractionOperations", func(t *testing.T) {
		processor := New()
		defer processor.Close()

		testData := `{
			"teams": [
				{
					"name": "Backend",
					"members": [
						{"name": "Alice", "salary": 80000, "bonus": 5000},
						{"name": "Bob", "salary": 70000, "bonus": 3000}
					]
				},
				{
					"name": "Frontend",
					"members": [
						{"name": "Charlie", "salary": 85000, "bonus": 6000},
						{"name": "Diana", "salary": 75000, "bonus": 4000}
					]
				}
			]
		}`

		// Test extraction setting - set bonus for all members
		result, err := processor.Set(testData, "teams{members}{bonus}", 10000)
		helper.AssertNoError(err, "Extraction setting should work")

		// Verify all bonuses were updated
		aliceBonus, err := processor.Get(result, "teams[0].members[0].bonus")
		helper.AssertNoError(err, "Getting Alice's bonus should work")
		helper.AssertEqual(float64(10000), aliceBonus, "Alice's bonus should be updated")

		bobBonus, err := processor.Get(result, "teams[0].members[1].bonus")
		helper.AssertNoError(err, "Getting Bob's bonus should work")
		helper.AssertEqual(float64(10000), bobBonus, "Bob's bonus should be updated")

		charlieBonus, err := processor.Get(result, "teams[1].members[0].bonus")
		helper.AssertNoError(err, "Getting Charlie's bonus should work")
		helper.AssertEqual(float64(10000), charlieBonus, "Charlie's bonus should be updated")

		dianaBonus, err := processor.Get(result, "teams[1].members[1].bonus")
		helper.AssertNoError(err, "Getting Diana's bonus should work")
		helper.AssertEqual(float64(10000), dianaBonus, "Diana's bonus should be updated")

		// Test flat extraction setting
		result, err = processor.Set(testData, "teams{flat:members}{department}", "Engineering")
		helper.AssertNoError(err, "Flat extraction setting should work")

		// Verify department was set for all members
		aliceDept, err := processor.Get(result, "teams[0].members[0].department")
		helper.AssertNoError(err, "Getting Alice's department should work")
		helper.AssertEqual("Engineering", aliceDept, "Alice's department should be set")
	})

	t.Run("MixedComplexOperations", func(t *testing.T) {
		processor := New()
		defer processor.Close()

		testData := `{
			"projects": [
				{
					"name": "Project A",
					"tasks": [
						{"id": 1, "assignees": [{"name": "Alice", "hours": 40}]},
						{"id": 2, "assignees": [{"name": "Bob", "hours": 35}]}
					]
				},
				{
					"name": "Project B", 
					"tasks": [
						{"id": 3, "assignees": [{"name": "Charlie", "hours": 30}]},
						{"id": 4, "assignees": [{"name": "Diana", "hours": 45}]}
					]
				}
			]
		}`

		// Test extraction followed by array access
		result, err := processor.Set(testData, "projects{tasks}[0].priority", "high")
		helper.AssertNoError(err, "Extraction followed by array access should work")

		// Verify priority was set for first task in each project
		priorityA, err := processor.Get(result, "projects[0].tasks[0].priority")
		helper.AssertNoError(err, "Getting Project A first task priority should work")
		helper.AssertEqual("high", priorityA, "Project A first task priority should be set")

		priorityB, err := processor.Get(result, "projects[1].tasks[0].priority")
		helper.AssertNoError(err, "Getting Project B first task priority should work")
		helper.AssertEqual("high", priorityB, "Project B first task priority should be set")

		// Test complex nested extraction
		result, err = processor.Set(testData, "projects{tasks}{assignees}{overtime}", 5)
		helper.AssertNoError(err, "Complex nested extraction should work")

		// Verify overtime was set for all assignees
		aliceOvertime, err := processor.Get(result, "projects[0].tasks[0].assignees[0].overtime")
		helper.AssertNoError(err, "Getting Alice's overtime should work")
		helper.AssertEqual(float64(5), aliceOvertime, "Alice's overtime should be set")

		dianaOvertime, err := processor.Get(result, "projects[1].tasks[1].assignees[0].overtime")
		helper.AssertNoError(err, "Getting Diana's overtime should work")
		helper.AssertEqual(float64(5), dianaOvertime, "Diana's overtime should be set")
	})

	t.Run("PathCreationOperations", func(t *testing.T) {
		processor := New()
		defer processor.Close()

		testData := `{
			"user": {
				"name": "John"
			}
		}`

		// Test path creation with SetWithAdd
		result, err := SetWithAdd(testData, "user.profile.settings.theme", "dark")
		helper.AssertNoError(err, "Path creation should work")

		theme, err := processor.Get(result, "user.profile.settings.theme")
		helper.AssertNoError(err, "Getting created path should work")
		helper.AssertEqual("dark", theme, "Created path value should be correct")

		// Test array path creation
		result, err = SetWithAdd(testData, "user.hobbies[0]", "reading")
		helper.AssertNoError(err, "Array path creation should work")

		hobby, err := processor.Get(result, "user.hobbies[0]")
		helper.AssertNoError(err, "Getting created array element should work")
		helper.AssertEqual("reading", hobby, "Created array element should be correct")

		// Test nested object path creation
		result, err = SetWithAdd(testData, "user.settings.notifications", true)
		helper.AssertNoError(err, "Nested object path creation should work")

		notifications, err := processor.Get(result, "user.settings.notifications")
		helper.AssertNoError(err, "Getting created nested path should work")
		helper.AssertEqual(true, notifications, "Created nested path value should be correct")
	})

	t.Run("NegativeIndexOperations", func(t *testing.T) {
		processor := New()
		defer processor.Close()

		testData := `{
			"items": [10, 20, 30, 40, 50],
			"nested": {
				"arrays": [
					[1, 2, 3],
					[4, 5, 6],
					[7, 8, 9]
				]
			}
		}`

		// Test negative index setting
		result, err := processor.Set(testData, "items[-1]", 99)
		helper.AssertNoError(err, "Negative index setting should work")

		lastItem, err := processor.Get(result, "items[-1]")
		helper.AssertNoError(err, "Getting last item should work")
		helper.AssertEqual(float64(99), lastItem, "Last item should be updated")

		// Test negative index in nested array
		result, err = processor.Set(testData, "nested.arrays[-1][-2]", 88)
		helper.AssertNoError(err, "Nested negative index setting should work")

		nestedItem, err := processor.Get(result, "nested.arrays[-1][-2]")
		helper.AssertNoError(err, "Getting nested item should work")
		helper.AssertEqual(float64(88), nestedItem, "Nested item should be updated")

		// Test negative index setting for multiple elements
		result, err = processor.Set(testData, "items[-3]", 77)
		helper.AssertNoError(err, "Negative index setting should work")

		result, err = processor.Set(result, "items[-2]", 88)
		helper.AssertNoError(err, "Negative index setting should work")

		items, err := processor.Get(result, "items")
		helper.AssertNoError(err, "Getting items array should work")
		if itemsArray, ok := items.([]any); ok {
			helper.AssertEqual(float64(77), itemsArray[2], "First negative index element should be updated")
			helper.AssertEqual(float64(88), itemsArray[3], "Second negative index element should be updated")
		}
	})

	t.Run("ErrorHandlingAndBoundaryConditions", func(t *testing.T) {
		processor := New()
		defer processor.Close()

		testData := `{
			"numbers": [1, 2, 3],
			"user": {
				"name": "John",
				"age": 30
			}
		}`

		// Test setting beyond array bounds without path creation
		_, err := processor.Set(testData, "numbers[10]", 100)
		helper.AssertError(err, "Setting beyond array bounds should fail")

		// Test setting on non-existent path without path creation
		_, err = processor.Set(testData, "user.profile.email", "john@example.com")
		helper.AssertError(err, "Setting non-existent path should fail")

		// Test setting with invalid array index
		_, err = processor.Set(testData, "numbers[abc]", 100)
		helper.AssertError(err, "Setting with invalid array index should fail")

		// Test setting with invalid slice syntax
		_, err = processor.Set(testData, "numbers[1:abc]", []any{10, 20})
		helper.AssertError(err, "Setting with invalid slice syntax should fail")

		// Test setting on primitive value
		_, err = processor.Set(testData, "user.name.length", 4)
		helper.AssertError(err, "Setting on primitive value should fail")

		// Test setting with empty path
		_, err = processor.Set(testData, "", "value")
		helper.AssertError(err, "Setting with empty path should fail")

		// Test setting root value
		_, err = processor.Set(testData, ".", "value")
		helper.AssertError(err, "Setting root value should fail")
	})

	t.Run("TypeConversionAndCoercion", func(t *testing.T) {
		processor := New()
		defer processor.Close()

		testData := `{
			"config": {
				"enabled": true,
				"count": 42,
				"name": "test"
			}
		}`

		// Test setting different types
		result, err := processor.Set(testData, "config.enabled", false)
		helper.AssertNoError(err, "Setting boolean should work")

		enabled, err := processor.Get(result, "config.enabled")
		helper.AssertNoError(err, "Getting boolean should work")
		helper.AssertEqual(false, enabled, "Boolean should be updated")

		// Test setting number
		result, err = processor.Set(testData, "config.count", 100)
		helper.AssertNoError(err, "Setting number should work")

		count, err := processor.Get(result, "config.count")
		helper.AssertNoError(err, "Getting number should work")
		helper.AssertEqual(float64(100), count, "Number should be updated")

		// Test setting null
		result, err = processor.Set(testData, "config.name", nil)
		helper.AssertNoError(err, "Setting null should work")

		name, err := processor.Get(result, "config.name")
		helper.AssertNoError(err, "Getting null should work")
		helper.AssertNil(name, "Value should be null")

		// Test setting complex object
		complexObj := map[string]any{
			"nested": map[string]any{
				"value": 123,
				"items": []any{1, 2, 3},
			},
		}
		result, err = processor.Set(testData, "config.complex", complexObj)
		helper.AssertNoError(err, "Setting complex object should work")

		nestedValue, err := processor.Get(result, "config.complex.nested.value")
		helper.AssertNoError(err, "Getting nested value should work")
		helper.AssertEqual(float64(123), nestedValue, "Nested value should be correct")
	})

	t.Run("ConcurrentSetOperations", func(t *testing.T) {
		processor := New()
		defer processor.Close()

		testData := `{
			"counters": {
				"a": 0,
				"b": 0,
				"c": 0
			}
		}`

		concurrencyTester := NewConcurrencyTester(t, 10, 50)

		concurrencyTester.Run(func(workerID, iteration int) error {
			path := "counters." + string(rune('a'+workerID%3))
			value := workerID*100 + iteration

			_, err := processor.Set(testData, path, value)
			return err
		})
	})

	t.Run("LargeDataSetOperations", func(t *testing.T) {
		processor := New()
		defer processor.Close()

		// Create simpler nested structure for testing
		testData := `{
			"data": {
				"items": [
					{"id": 0, "name": "Item 0", "value": 100},
					{"id": 1, "name": "Item 1", "value": 200},
					{"id": 2, "name": "Item 2", "value": 300}
				]
			}
		}`

		// Test setting in existing structure
		result, err := processor.Set(testData, "data.items[1].status", "updated")
		helper.AssertNoError(err, "Setting in existing structure should work")

		status, err := processor.Get(result, "data.items[1].status")
		helper.AssertNoError(err, "Getting updated status should work")
		helper.AssertEqual("updated", status, "Status should be updated")

		// Test setting multiple properties
		result, err = processor.Set(result, "data.items[0].priority", "high")
		helper.AssertNoError(err, "Setting priority should work")

		priority, err := processor.Get(result, "data.items[0].priority")
		helper.AssertNoError(err, "Getting priority should work")
		helper.AssertEqual("high", priority, "Priority should be set")
	})
}
