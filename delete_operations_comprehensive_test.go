package json

import (
	"testing"
)

// TestDeleteOperationsComprehensive tests delete operations functionality
func TestDeleteOperationsComprehensive(t *testing.T) {
	helper := NewTestHelper(t)

	t.Run("BasicDeleteOperations", func(t *testing.T) {
		processor := New()
		defer processor.Close()

		testData := `{
			"name": "John",
			"age": 30,
			"city": "New York",
			"hobbies": ["reading", "swimming", "coding"],
			"profile": {
				"email": "john@example.com",
				"phone": "123-456-7890",
				"address": {
					"street": "123 Main St",
					"zip": "10001"
				}
			}
		}`

		// Test simple property deletion
		result, err := processor.Delete(testData, "age")
		helper.AssertNoError(err, "Simple property deletion should work")

		// Verify the property was deleted - should return error for nonexistent path
		deletedValue, err := processor.Get(result, "age")
		helper.AssertError(err, "Getting deleted property should return error")
		helper.AssertNil(deletedValue, "Deleted property should be nil")

		// Verify other properties still exist
		name, err := processor.Get(result, "name")
		helper.AssertNoError(err, "Other properties should still exist")
		helper.AssertEqual("John", name, "Other properties should be unchanged")

		// Test nested property deletion
		result, err = processor.Delete(testData, "profile.phone")
		helper.AssertNoError(err, "Nested property deletion should work")

		deletedPhone, err := processor.Get(result, "profile.phone")
		helper.AssertError(err, "Getting deleted nested property should return error")
		helper.AssertNil(deletedPhone, "Deleted nested property should be nil")

		// Test deep nested property deletion
		result, err = processor.Delete(testData, "profile.address.zip")
		helper.AssertNoError(err, "Deep nested property deletion should work")

		deletedZip, err := processor.Get(result, "profile.address.zip")
		helper.AssertError(err, "Getting deleted deep nested property should return error")
		helper.AssertNil(deletedZip, "Deleted deep nested property should be nil")
	})

	t.Run("ArrayElementDeletion", func(t *testing.T) {
		processor := New()
		defer processor.Close()

		testData := `{
			"numbers": [1, 2, 3, 4, 5],
			"users": [
				{"name": "Alice", "age": 25},
				{"name": "Bob", "age": 30},
				{"name": "Charlie", "age": 35}
			]
		}`

		// Test array element deletion by index
		result, err := processor.Delete(testData, "numbers[2]")
		helper.AssertNoError(err, "Array element deletion should work")

		// Verify array length changed
		numbers, err := processor.Get(result, "numbers")
		helper.AssertNoError(err, "Getting modified array should work")
		if arr, ok := numbers.([]any); ok {
			helper.AssertEqual(4, len(arr), "Array should have one less element")
			// Verify the element was actually removed (3 should be gone)
			helper.AssertEqual(float64(1), arr[0], "First element should be unchanged")
			helper.AssertEqual(float64(2), arr[1], "Second element should be unchanged")
			helper.AssertEqual(float64(4), arr[2], "Third element should now be 4")
		}

		// Test negative index deletion
		result, err = processor.Delete(testData, "numbers[-1]")
		helper.AssertNoError(err, "Negative index deletion should work")

		numbers, err = processor.Get(result, "numbers")
		helper.AssertNoError(err, "Getting array after negative index deletion should work")
		if arr, ok := numbers.([]any); ok {
			helper.AssertEqual(4, len(arr), "Array should have one less element")
			// Last element (5) should be gone
			helper.AssertEqual(float64(4), arr[len(arr)-1], "Last element should now be 4")
		}

		// Test object in array deletion
		result, err = processor.Delete(testData, "users[1]")
		helper.AssertNoError(err, "Object in array deletion should work")

		users, err := processor.Get(result, "users")
		helper.AssertNoError(err, "Getting users after deletion should work")
		if arr, ok := users.([]any); ok {
			helper.AssertEqual(2, len(arr), "Users array should have one less element")
		}
	})

	t.Run("ArraySliceDeletion", func(t *testing.T) {
		processor := New()
		defer processor.Close()

		testData := `{
			"items": [0, 1, 2, 3, 4, 5, 6, 7, 8, 9],
			"words": ["apple", "banana", "cherry", "date", "elderberry"]
		}`

		// Test slice deletion
		result, err := processor.Delete(testData, "items[2:5]")
		helper.AssertNoError(err, "Array slice deletion should work")

		items, err := processor.Get(result, "items")
		helper.AssertNoError(err, "Getting items after slice deletion should work")
		if arr, ok := items.([]any); ok {
			helper.AssertEqual(7, len(arr), "Array should have 3 fewer elements")
			// Elements 2, 3, 4 should be deleted
			helper.AssertEqual(float64(0), arr[0], "First element should be unchanged")
			helper.AssertEqual(float64(1), arr[1], "Second element should be unchanged")
			helper.AssertEqual(float64(5), arr[2], "Third element should now be 5")
		}

		// Test open-ended slice deletion
		result, err = processor.Delete(testData, "words[2:]")
		helper.AssertNoError(err, "Open-ended slice deletion should work")

		words, err := processor.Get(result, "words")
		helper.AssertNoError(err, "Getting words after slice deletion should work")
		if arr, ok := words.([]any); ok {
			helper.AssertEqual(2, len(arr), "Array should have only first 2 elements")
			helper.AssertEqual("apple", arr[0], "First element should be unchanged")
			helper.AssertEqual("banana", arr[1], "Second element should be unchanged")
		}
	})

	t.Run("ExtractionDeletion", func(t *testing.T) {
		processor := New()
		defer processor.Close()

		testData := `{
			"users": [
				{"name": "Alice", "email": "alice@example.com", "age": 25},
				{"name": "Bob", "email": "bob@example.com", "age": 30},
				{"name": "Charlie", "email": "charlie@example.com", "age": 35}
			],
			"products": [
				{"name": "Phone", "price": 500, "category": "Electronics"},
				{"name": "Book", "price": 20, "category": "Education"}
			]
		}`

		// Test extraction deletion (delete email from all users)
		result, err := processor.Delete(testData, "users{email}")
		helper.AssertNoError(err, "Extraction deletion should work")

		// Verify emails were deleted from all users
		users, err := processor.Get(result, "users")
		helper.AssertNoError(err, "Getting users after extraction deletion should work")
		if arr, ok := users.([]any); ok {
			for i, user := range arr {
				if userObj, ok := user.(map[string]any); ok {
					_, hasEmail := userObj["email"]
					helper.AssertFalse(hasEmail, "User %d should not have email after deletion", i)

					// Verify other properties still exist
					_, hasName := userObj["name"]
					helper.AssertTrue(hasName, "User %d should still have name", i)
				}
			}
		}

		// Test extraction with array operation deletion
		result, err = processor.Delete(testData, "users[0:2]{age}")
		helper.AssertNoError(err, "Extraction with array operation deletion should work")

		users, err = processor.Get(result, "users")
		helper.AssertNoError(err, "Getting users after complex deletion should work")
		if arr, ok := users.([]any); ok {
			// First two users should not have age, third should still have it
			for i := 0; i < 2; i++ {
				if userObj, ok := arr[i].(map[string]any); ok {
					_, hasAge := userObj["age"]
					helper.AssertFalse(hasAge, "User %d should not have age after deletion", i)
				}
			}
			if len(arr) > 2 {
				if userObj, ok := arr[2].(map[string]any); ok {
					_, hasAge := userObj["age"]
					helper.AssertTrue(hasAge, "User 2 should still have age")
				}
			}
		}
	})

	t.Run("ComplexPathDeletion", func(t *testing.T) {
		processor := New()
		defer processor.Close()

		testData := `{
			"company": {
				"departments": [
					{
						"name": "Engineering",
						"employees": [
							{"name": "Alice", "salary": 80000, "benefits": {"health": true, "dental": true}},
							{"name": "Bob", "salary": 75000, "benefits": {"health": true, "dental": false}}
						]
					},
					{
						"name": "Marketing", 
						"employees": [
							{"name": "Charlie", "salary": 60000, "benefits": {"health": true, "dental": true}}
						]
					}
				]
			}
		}`

		// Test complex nested deletion
		result, err := processor.Delete(testData, "company.departments[0].employees[1].benefits.dental")
		helper.AssertNoError(err, "Complex nested deletion should work")

		// Verify specific nested property was deleted - should return error for nonexistent path
		dental, err := processor.Get(result, "company.departments[0].employees[1].benefits.dental")
		helper.AssertError(err, "Getting deleted complex nested property should return error")
		helper.AssertNil(dental, "Complex nested property should be deleted")

		// Test extraction from nested structure
		result, err = processor.Delete(testData, "company.departments{employees}{salary}")
		helper.AssertNoError(err, "Nested extraction deletion should work")

		// Verify salaries were deleted from all employees in all departments
		departments, err := processor.Get(result, "company.departments")
		helper.AssertNoError(err, "Getting departments after nested extraction deletion should work")
		if deptArr, ok := departments.([]any); ok {
			for _, dept := range deptArr {
				if deptObj, ok := dept.(map[string]any); ok {
					if employees, ok := deptObj["employees"].([]any); ok {
						for _, emp := range employees {
							if empObj, ok := emp.(map[string]any); ok {
								_, hasSalary := empObj["salary"]
								helper.AssertFalse(hasSalary, "Employee should not have salary after deletion")
							}
						}
					}
				}
			}
		}
	})

	t.Run("EdgeCaseDeletion", func(t *testing.T) {
		processor := New()
		defer processor.Close()

		// Test deletion from empty object
		emptyObj := `{}`
		_, err := processor.Delete(emptyObj, "nonexistent")
		helper.AssertError(err, "Deleting from empty object should return error")

		// Test deletion from empty array
		emptyArr := `{"arr": []}`
		_, err = processor.Delete(emptyArr, "arr[0]")
		helper.AssertError(err, "Deleting from empty array should return error")

		// Test deletion of nonexistent property
		testData := `{"name": "test", "value": 42}`
		_, err = processor.Delete(testData, "nonexistent")
		helper.AssertError(err, "Deleting nonexistent property should return error")

		// Test deletion with invalid path
		_, err = processor.Delete(testData, "invalid[path")
		helper.AssertError(err, "Invalid path should return error")
	})

	t.Run("DeleteWithCleanup", func(t *testing.T) {
		processor := New()
		defer processor.Close()

		testData := `{
			"user": {
				"profile": {
					"name": "John",
					"email": "john@example.com"
				},
				"settings": {
					"theme": "dark"
				}
			}
		}`

		// Test deletion that might leave empty containers
		result, err := processor.Delete(testData, "user.profile.email")
		helper.AssertNoError(err, "Deletion should work")

		// Verify the structure is maintained
		profile, err := processor.Get(result, "user.profile")
		helper.AssertNoError(err, "Profile should still exist")
		helper.AssertNotNil(profile, "Profile should not be nil")

		if profileObj, ok := profile.(map[string]any); ok {
			_, hasName := profileObj["name"]
			helper.AssertTrue(hasName, "Name should still exist in profile")

			_, hasEmail := profileObj["email"]
			helper.AssertFalse(hasEmail, "Email should be deleted from profile")
		}
	})
}
