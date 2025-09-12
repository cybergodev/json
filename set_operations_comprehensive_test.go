package json

import (
	"testing"
)

// TestSetOperationsComprehensive tests set operations functionality
func TestSetOperationsComprehensive(t *testing.T) {
	helper := NewTestHelper(t)

	t.Run("BasicSetOperations", func(t *testing.T) {
		processor := New()
		defer processor.Close()

		testData := `{
			"name": "John",
			"age": 30,
			"city": "New York"
		}`

		// Test simple property setting
		result, err := processor.Set(testData, "age", 35)
		helper.AssertNoError(err, "Simple property setting should work")

		// Verify the property was updated
		age, err := processor.Get(result, "age")
		helper.AssertNoError(err, "Getting updated property should work")
		helper.AssertEqual(float64(35), age, "Property should be updated")

		// Test new property creation
		result, err = processor.Set(testData, "email", "john@example.com")
		helper.AssertNoError(err, "New property creation should work")

		email, err := processor.Get(result, "email")
		helper.AssertNoError(err, "Getting new property should work")
		helper.AssertEqual("john@example.com", email, "New property should be set")

		// Test setting different data types
		result, err = processor.Set(testData, "active", true)
		helper.AssertNoError(err, "Setting boolean should work")

		active, err := processor.Get(result, "active")
		helper.AssertNoError(err, "Getting boolean should work")
		helper.AssertEqual(true, active, "Boolean should be set correctly")

		// Test setting null value
		result, err = processor.Set(testData, "middle_name", nil)
		helper.AssertNoError(err, "Setting null should work")

		middleName, err := processor.Get(result, "middle_name")
		helper.AssertNoError(err, "Getting null value should work")
		helper.AssertNil(middleName, "Null value should be set correctly")
	})

	t.Run("NestedPropertySetting", func(t *testing.T) {
		processor := New()
		defer processor.Close()

		testData := `{
			"user": {
				"profile": {
					"name": "Alice",
					"age": 25
				},
				"settings": {
					"theme": "light",
					"notifications": true
				}
			}
		}`

		// Test nested property setting
		result, err := processor.Set(testData, "user.profile.age", 26)
		helper.AssertNoError(err, "Nested property setting should work")

		age, err := processor.Get(result, "user.profile.age")
		helper.AssertNoError(err, "Getting nested property should work")
		helper.AssertEqual(float64(26), age, "Nested property should be updated")

		// Test deep nested property setting
		result, err = processor.Set(testData, "user.settings.notifications", false)
		helper.AssertNoError(err, "Deep nested property setting should work")

		notifications, err := processor.Get(result, "user.settings.notifications")
		helper.AssertNoError(err, "Getting deep nested property should work")
		helper.AssertEqual(false, notifications, "Deep nested property should be updated")

		// Test creating new nested structure (should fail)
		_, err = processor.Set(testData, "user.contact.email", "alice@example.com")
		helper.AssertError(err, "Creating new nested structure should fail")
	})

	t.Run("ArrayElementSetting", func(t *testing.T) {
		processor := New()
		defer processor.Close()

		testData := `{
			"numbers": [1, 2, 3, 4, 5],
			"users": [
				{"name": "Alice", "age": 25},
				{"name": "Bob", "age": 30}
			]
		}`

		// Test array element setting by index
		result, err := processor.Set(testData, "numbers[2]", 99)
		helper.AssertNoError(err, "Array element setting should work")

		element, err := processor.Get(result, "numbers[2]")
		helper.AssertNoError(err, "Getting updated array element should work")
		helper.AssertEqual(float64(99), element, "Array element should be updated")

		// Test negative index setting
		result, err = processor.Set(testData, "numbers[-1]", 100)
		helper.AssertNoError(err, "Negative index setting should work")

		lastElement, err := processor.Get(result, "numbers[-1]")
		helper.AssertNoError(err, "Getting last element should work")
		helper.AssertEqual(float64(100), lastElement, "Last element should be updated")

		// Test setting object property in array
		result, err = processor.Set(testData, "users[0].age", 26)
		helper.AssertNoError(err, "Setting object property in array should work")

		age, err := processor.Get(result, "users[0].age")
		helper.AssertNoError(err, "Getting object property in array should work")
		helper.AssertEqual(float64(26), age, "Object property in array should be updated")

		// Test adding new property to object in array
		result, err = processor.Set(testData, "users[1].email", "bob@example.com")
		helper.AssertNoError(err, "Adding property to object in array should work")

		email, err := processor.Get(result, "users[1].email")
		helper.AssertNoError(err, "Getting new property in array object should work")
		helper.AssertEqual("bob@example.com", email, "New property in array object should be set")
	})

	t.Run("ArrayBoundsHandling", func(t *testing.T) {
		processor := New()
		defer processor.Close()

		testData := `{
			"items": [1, 2, 3]
		}`

		// Test setting beyond array bounds (should fail)
		_, err := processor.Set(testData, "items[5]", 6)
		helper.AssertError(err, "Setting beyond array bounds should fail")

		// Test setting within bounds
		result, err := processor.Set(testData, "items[2]", 99)
		helper.AssertNoError(err, "Setting within bounds should work")

		element, err := processor.Get(result, "items[2]")
		helper.AssertNoError(err, "Getting updated element should work")
		helper.AssertEqual(float64(99), element, "Element should be updated")
	})

	t.Run("ExtractionSetting", func(t *testing.T) {
		processor := New()
		defer processor.Close()

		testData := `{
			"users": [
				{"name": "Alice", "age": 25, "active": true},
				{"name": "Bob", "age": 30, "active": false},
				{"name": "Charlie", "age": 35, "active": true}
			]
		}`

		// Test setting property for all extracted objects
		result, err := processor.Set(testData, "users{active}", true)
		helper.AssertNoError(err, "Extraction setting should work")

		// Verify all users are now active
		users, err := processor.Get(result, "users")
		helper.AssertNoError(err, "Getting users after extraction setting should work")
		if arr, ok := users.([]any); ok {
			for i, user := range arr {
				if userObj, ok := user.(map[string]any); ok {
					active, hasActive := userObj["active"]
					helper.AssertTrue(hasActive, "User %d should have active property", i)
					helper.AssertEqual(true, active, "User %d should be active", i)
				}
			}
		}

		// Test setting new property for all extracted objects
		result, err = processor.Set(testData, "users{department}", "Engineering")
		helper.AssertNoError(err, "Setting new property via extraction should work")

		users, err = processor.Get(result, "users")
		helper.AssertNoError(err, "Getting users after new property setting should work")
		if arr, ok := users.([]any); ok {
			for i, user := range arr {
				if userObj, ok := user.(map[string]any); ok {
					dept, hasDept := userObj["department"]
					helper.AssertTrue(hasDept, "User %d should have department property", i)
					helper.AssertEqual("Engineering", dept, "User %d should be in Engineering", i)
				}
			}
		}
	})

	t.Run("ComplexPathSetting", func(t *testing.T) {
		processor := New()
		defer processor.Close()

		testData := `{
			"company": {
				"departments": [
					{
						"name": "Engineering",
						"employees": [
							{"name": "Alice", "salary": 80000},
							{"name": "Bob", "salary": 75000}
						]
					},
					{
						"name": "Marketing",
						"employees": [
							{"name": "Charlie", "salary": 60000}
						]
					}
				]
			}
		}`

		// Test complex nested setting
		result, err := processor.Set(testData, "company.departments[0].employees[1].salary", 85000)
		helper.AssertNoError(err, "Complex nested setting should work")

		salary, err := processor.Get(result, "company.departments[0].employees[1].salary")
		helper.AssertNoError(err, "Getting complex nested property should work")
		helper.AssertEqual(float64(85000), salary, "Complex nested property should be updated")

		// Test extraction with complex path
		result, err = processor.Set(testData, "company.departments{employees}{bonus}", 5000)
		helper.AssertNoError(err, "Complex extraction setting should work")

		// Verify all employees got bonus
		departments, err := processor.Get(result, "company.departments")
		helper.AssertNoError(err, "Getting departments after complex setting should work")
		if deptArr, ok := departments.([]any); ok {
			for _, dept := range deptArr {
				if deptObj, ok := dept.(map[string]any); ok {
					if employees, ok := deptObj["employees"].([]any); ok {
						for _, emp := range employees {
							if empObj, ok := emp.(map[string]any); ok {
								bonus, hasBonus := empObj["bonus"]
								helper.AssertTrue(hasBonus, "Employee should have bonus")
								helper.AssertEqual(float64(5000), bonus, "Employee bonus should be 5000")
							}
						}
					}
				}
			}
		}
	})

	t.Run("SetWithDifferentTypes", func(t *testing.T) {
		processor := New()
		defer processor.Close()

		testData := `{"value": "original"}`

		// Test setting string
		result, err := processor.Set(testData, "value", "new string")
		helper.AssertNoError(err, "Setting string should work")
		value, _ := processor.Get(result, "value")
		helper.AssertEqual("new string", value, "String should be set")

		// Test setting number
		result, err = processor.Set(testData, "value", 42)
		helper.AssertNoError(err, "Setting number should work")
		value, _ = processor.Get(result, "value")
		helper.AssertEqual(float64(42), value, "Number should be set")

		// Test setting boolean
		result, err = processor.Set(testData, "value", false)
		helper.AssertNoError(err, "Setting boolean should work")
		value, _ = processor.Get(result, "value")
		helper.AssertEqual(false, value, "Boolean should be set")

		// Test setting array
		result, err = processor.Set(testData, "value", []any{1, 2, 3})
		helper.AssertNoError(err, "Setting array should work")
		value, _ = processor.Get(result, "value")
		if arr, ok := value.([]any); ok {
			helper.AssertEqual(3, len(arr), "Array should be set with correct length")
		}

		// Test setting object
		result, err = processor.Set(testData, "value", map[string]any{"key": "value"})
		helper.AssertNoError(err, "Setting object should work")
		value, _ = processor.Get(result, "value")
		if obj, ok := value.(map[string]any); ok {
			helper.AssertEqual("value", obj["key"], "Object should be set correctly")
		}
	})

	t.Run("EdgeCaseSetting", func(t *testing.T) {
		processor := New()
		defer processor.Close()

		// Test setting on empty object
		emptyObj := `{}`
		result, err := processor.Set(emptyObj, "newProp", "value")
		helper.AssertNoError(err, "Setting on empty object should work")

		newProp, err := processor.Get(result, "newProp")
		helper.AssertNoError(err, "Getting new property should work")
		helper.AssertEqual("value", newProp, "New property should be set")

		// Test setting with invalid path
		testData := `{"test": "value"}`
		_, err = processor.Set(testData, "invalid[path", "value")
		helper.AssertError(err, "Invalid path should return error")

		// Test setting on array out of bounds (should fail)
		arrayData := `{"arr": [1, 2, 3]}`
		_, err = processor.Set(arrayData, "arr[10]", "value")
		helper.AssertError(err, "Setting out of bounds index should return error")

		_, err = processor.Set(arrayData, "arr[-10]", "value")
		helper.AssertError(err, "Setting far negative index should return error")
	})
}
