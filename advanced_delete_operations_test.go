package json

import (
	"testing"
)

// TestAdvancedDeleteOperations tests comprehensive Delete operations with complex paths
func TestAdvancedDeleteOperations(t *testing.T) {
	helper := NewTestHelper(t)

	t.Run("DeepNestedDeletion", func(t *testing.T) {
		processor := New()
		defer processor.Close()

		testData := `{
			"company": {
				"departments": [
					{
						"name": "Engineering",
						"manager": {
							"name": "Alice",
							"contact": {
								"email": "alice@company.com",
								"phone": "+1-555-0101",
								"address": {
									"street": "123 Main St",
									"city": "Tech City",
									"country": "USA"
								}
							}
						},
						"employees": [
							{
								"id": 1,
								"name": "Bob",
								"skills": ["Go", "Python", "Docker"],
								"projects": [
									{"name": "API Gateway", "status": "active"},
									{"name": "Database Migration", "status": "completed"}
								]
							},
							{
								"id": 2,
								"name": "Charlie",
								"skills": ["JavaScript", "React"],
								"projects": [
									{"name": "Frontend Redesign", "status": "active"}
								]
							}
						]
					},
					{
						"name": "Marketing",
						"manager": {
							"name": "Diana",
							"contact": {
								"email": "diana@company.com",
								"phone": "+1-555-0102"
							}
						},
						"employees": []
					}
				]
			}
		}`

		// Test deleting deep nested property
		result, err := processor.Delete(testData, "company.departments[0].manager.contact.phone")
		helper.AssertNoError(err, "Deep nested property deletion should work")

		// Verify phone was deleted but other properties remain
		phone, err := processor.Get(result, "company.departments[0].manager.contact.phone")
		helper.AssertNil(phone, "Phone should be deleted")

		email, err := processor.Get(result, "company.departments[0].manager.contact.email")
		helper.AssertNoError(err, "Email should still exist")
		helper.AssertEqual("alice@company.com", email, "Email should be unchanged")

		// Test deleting entire nested object
		result, err = processor.Delete(result, "company.departments[0].manager.contact.address")
		helper.AssertNoError(err, "Nested object deletion should work")

		address, err := processor.Get(result, "company.departments[0].manager.contact.address")
		helper.AssertNil(address, "Address object should be deleted")

		// Test deleting array element by index
		result, err = processor.Delete(result, "company.departments[0].employees[1]")
		helper.AssertNoError(err, "Array element deletion should work")

		employees, err := processor.Get(result, "company.departments[0].employees")
		helper.AssertNoError(err, "Employees array should still exist")
		if empArray, ok := employees.([]any); ok {
			helper.AssertEqual(1, len(empArray), "Should have one employee left")
		}

		// Test deleting property from array element
		result, err = processor.Delete(result, "company.departments[0].employees[0].skills[1]")
		helper.AssertNoError(err, "Array element property deletion should work")

		skills, err := processor.Get(result, "company.departments[0].employees[0].skills")
		helper.AssertNoError(err, "Skills array should still exist")
		if skillsArray, ok := skills.([]any); ok {
			helper.AssertEqual(2, len(skillsArray), "Should have two skills left")
		}
	})

	t.Run("ArrayDeletionOperations", func(t *testing.T) {
		processor := New()
		defer processor.Close()

		testData := `{
			"data": {
				"numbers": [10, 20, 30, 40, 50, 60, 70, 80, 90, 100],
				"matrix": [
					[1, 2, 3, 4, 5],
					[6, 7, 8, 9, 10],
					[11, 12, 13, 14, 15]
				],
				"objects": [
					{"id": 1, "name": "First", "tags": ["important", "urgent", "new"]},
					{"id": 2, "name": "Second", "tags": ["normal", "old"]},
					{"id": 3, "name": "Third", "tags": ["low", "archived"]}
				]
			}
		}`

		// Test deleting single array element
		result, err := processor.Delete(testData, "data.numbers[5]")
		helper.AssertNoError(err, "Single array element deletion should work")

		numbers, err := processor.Get(result, "data.numbers")
		helper.AssertNoError(err, "Numbers array should still exist")
		if numArray, ok := numbers.([]any); ok {
			helper.AssertEqual(9, len(numArray), "Should have 9 elements left")
		}

		// Test deleting from nested array
		result, err = processor.Delete(result, "data.matrix[1][2]")
		helper.AssertNoError(err, "Nested array element deletion should work")

		matrixRow, err := processor.Get(result, "data.matrix[1]")
		helper.AssertNoError(err, "Matrix row should still exist")
		if rowArray, ok := matrixRow.([]any); ok {
			helper.AssertEqual(4, len(rowArray), "Should have 4 elements left in row")
		}

		// Test deleting entire array
		result, err = processor.Delete(result, "data.objects[1].tags")
		helper.AssertNoError(err, "Entire array deletion should work")

		tags, err := processor.Get(result, "data.objects[1].tags")
		helper.AssertNil(tags, "Tags array should be deleted")

		// Verify object still exists
		obj, err := processor.Get(result, "data.objects[1]")
		helper.AssertNoError(err, "Object should still exist")
		helper.AssertNotNil(obj, "Object should not be nil")

		// Test deleting multiple array elements by deleting object
		result, err = processor.Delete(result, "data.objects[0]")
		helper.AssertNoError(err, "Object deletion should work")

		objects, err := processor.Get(result, "data.objects")
		helper.AssertNoError(err, "Objects array should still exist")
		if objArray, ok := objects.([]any); ok {
			helper.AssertEqual(2, len(objArray), "Should have 2 objects left")
		}
	})

	t.Run("ComplexPathDeletion", func(t *testing.T) {
		processor := New()
		defer processor.Close()

		testData := `{
			"projects": [
				{
					"name": "Project Alpha",
					"metadata": {
						"created": "2023-01-01",
						"tags": ["important", "backend"],
						"settings": {
							"notifications": true,
							"autoSave": false,
							"theme": "dark"
						}
					},
					"tasks": [
						{
							"id": 1,
							"title": "Setup Database",
							"assignees": [
								{"name": "Alice", "role": "Lead"},
								{"name": "Bob", "role": "Developer"}
							],
							"subtasks": [
								{"id": 11, "title": "Design Schema"},
								{"id": 12, "title": "Create Tables"}
							]
						},
						{
							"id": 2,
							"title": "API Development",
							"assignees": [
								{"name": "Charlie", "role": "Developer"}
							],
							"subtasks": []
						}
					]
				}
			]
		}`

		// Test deleting nested property in complex structure
		result, err := processor.Delete(testData, "projects[0].metadata.settings.autoSave")
		helper.AssertNoError(err, "Complex nested property deletion should work")

		autoSave, err := processor.Get(result, "projects[0].metadata.settings.autoSave")
		helper.AssertNil(autoSave, "AutoSave should be deleted")

		// Verify other settings remain
		theme, err := processor.Get(result, "projects[0].metadata.settings.theme")
		helper.AssertNoError(err, "Theme should still exist")
		helper.AssertEqual("dark", theme, "Theme should be unchanged")

		// Test deleting array element from nested structure
		result, err = processor.Delete(result, "projects[0].tasks[0].assignees[1]")
		helper.AssertNoError(err, "Nested array element deletion should work")

		assignees, err := processor.Get(result, "projects[0].tasks[0].assignees")
		helper.AssertNoError(err, "Assignees array should still exist")
		if assigneeArray, ok := assignees.([]any); ok {
			helper.AssertEqual(1, len(assigneeArray), "Should have 1 assignee left")
		}

		// Test deleting entire subtasks array
		result, err = processor.Delete(result, "projects[0].tasks[1].subtasks")
		helper.AssertNoError(err, "Subtasks array deletion should work")

		subtasks, err := processor.Get(result, "projects[0].tasks[1].subtasks")
		helper.AssertNil(subtasks, "Subtasks should be deleted")

		// Test deleting tag from array
		result, err = processor.Delete(result, "projects[0].metadata.tags[0]")
		helper.AssertNoError(err, "Tag deletion should work")

		tags, err := processor.Get(result, "projects[0].metadata.tags")
		helper.AssertNoError(err, "Tags array should still exist")
		if tagArray, ok := tags.([]any); ok {
			helper.AssertEqual(1, len(tagArray), "Should have 1 tag left")
			helper.AssertEqual("backend", tagArray[0], "Remaining tag should be 'backend'")
		}
	})

	t.Run("ErrorHandlingAndEdgeCases", func(t *testing.T) {
		processor := New()
		defer processor.Close()

		testData := `{
			"data": {
				"items": [1, 2, 3],
				"nested": {
					"value": 42
				},
				"empty": {},
				"nullValue": null
			}
		}`

		// Test deleting non-existent property
		result, err := processor.Delete(testData, "data.nonexistent")
		if err != nil {
			helper.AssertError(err, "Deleting non-existent property should return error")
		} else {
			// If no error, result should be unchanged
			helper.AssertEqual(testData, result, "Result should be unchanged when deleting non-existent property")
		}

		// Test deleting from empty object
		result, err = processor.Delete(testData, "data.empty.something")
		if err != nil {
			helper.AssertError(err, "Deleting from empty object should return error")
		}

		// Test deleting out of bounds array element
		result, err = processor.Delete(testData, "data.items[10]")
		if err != nil {
			helper.AssertError(err, "Deleting out of bounds element should return error")
		}

		// Test deleting with invalid path
		_, err = processor.Delete(testData, "data.items[abc]")
		helper.AssertError(err, "Invalid array index should return error")

		// Test deleting null value
		result, err = processor.Delete(testData, "data.nullValue")
		helper.AssertNoError(err, "Deleting null value should work")

		nullVal, err := processor.Get(result, "data.nullValue")
		helper.AssertNil(nullVal, "Null value should be deleted")

		// Test deleting entire nested object
		result, err = processor.Delete(testData, "data.nested")
		helper.AssertNoError(err, "Deleting nested object should work")

		nested, err := processor.Get(result, "data.nested")
		helper.AssertNil(nested, "Nested object should be deleted")

		// Verify parent object still exists
		data, err := processor.Get(result, "data")
		helper.AssertNoError(err, "Parent object should still exist")
		helper.AssertNotNil(data, "Parent object should not be nil")
	})

	t.Run("NegativeIndexDeletion", func(t *testing.T) {
		processor := New()
		defer processor.Close()

		testData := `{
			"arrays": {
				"numbers": [10, 20, 30, 40, 50],
				"nested": [
					{"id": 1, "values": [100, 200, 300]},
					{"id": 2, "values": [400, 500, 600]},
					{"id": 3, "values": [700, 800, 900]}
				]
			}
		}`

		// Test deleting with negative index
		result, err := processor.Delete(testData, "arrays.numbers[-1]")
		helper.AssertNoError(err, "Negative index deletion should work")

		numbers, err := processor.Get(result, "arrays.numbers")
		helper.AssertNoError(err, "Numbers array should still exist")
		if numArray, ok := numbers.([]any); ok {
			helper.AssertEqual(4, len(numArray), "Should have 4 elements left")
			helper.AssertEqual(float64(40), numArray[3], "Last element should be 40")
		}

		// Test deleting from nested array with negative index
		result, err = processor.Delete(result, "arrays.nested[-1].values[-2]")
		helper.AssertNoError(err, "Nested negative index deletion should work")

		values, err := processor.Get(result, "arrays.nested[2].values")
		helper.AssertNoError(err, "Values array should still exist")
		if valArray, ok := values.([]any); ok {
			helper.AssertEqual(2, len(valArray), "Should have 2 values left")
		}

		// Test deleting entire element with negative index
		result, err = processor.Delete(result, "arrays.nested[-2]")
		helper.AssertNoError(err, "Negative index object deletion should work")

		nested, err := processor.Get(result, "arrays.nested")
		helper.AssertNoError(err, "Nested array should still exist")
		if nestedArray, ok := nested.([]any); ok {
			helper.AssertEqual(2, len(nestedArray), "Should have 2 objects left")
		}
	})
}
