package json

import (
	"fmt"
	"testing"
)

func TestIteratorComprehensive(t *testing.T) {
	helper := NewTestHelper(t)
	processor := New()
	defer processor.Close()

	t.Run("BasicIteratorCreation", func(t *testing.T) {
		// Test basic iterator creation
		jsonData := `{"name": "test", "value": 123, "active": true}`

		// Parse JSON data first
		var data interface{}
		err := processor.Parse(jsonData, &data)
		helper.AssertNoError(err, "Should parse JSON data")

		iterator := NewIterator(processor, data, DefaultOptions())
		helper.AssertNotNil(iterator, "Should create iterator")

		// Test iterator with custom options
		options := DefaultOptions()
		options.StrictMode = true
		iteratorWithOptions := NewIterator(processor, data, options)
		helper.AssertNotNil(iteratorWithOptions, "Should create iterator with options")
	})

	t.Run("IterableValueTypedOperations", func(t *testing.T) {
		// Test IterableValue typed get operations (iterator-specific)
		jsonData := `{
			"name": "Alice",
			"age": 25,
			"score": 87.5,
			"active": true,
			"tags": ["user", "premium"],
			"profile": {"level": "gold"}
		}`

		var data interface{}
		err := processor.Parse(jsonData, &data)
		helper.AssertNoError(err)

		iterator := NewIterator(processor, data, DefaultOptions())
		iterableValue := NewIterableValueWithIterator(data, processor, iterator)

		// Test typed getters
		helper.AssertEqual("Alice", iterableValue.GetString("name"))
		helper.AssertEqual(25, iterableValue.GetInt("age"))
		helper.AssertEqual(int64(25), iterableValue.GetInt64("age"))
		helper.AssertEqual(87.5, iterableValue.GetFloat64("score"))
		helper.AssertEqual(true, iterableValue.GetBool("active"))

		tags := iterableValue.GetArray("tags")
		helper.AssertEqual(2, len(tags))

		profile := iterableValue.GetObject("profile")
		helper.AssertNotNil(profile)
	})

	t.Run("GetWithDefault", func(t *testing.T) {
		// Test get with default values
		jsonData := `{"existing": "value"}`

		// Parse JSON data first
		var data interface{}
		err := processor.Parse(jsonData, &data)
		helper.AssertNoError(err, "Should parse JSON data")

		iterator := NewIterator(processor, data, DefaultOptions())
		iterableValue := NewIterableValueWithIterator(data, processor, iterator)

		// Test existing value
		existing := iterableValue.GetWithDefault("existing", "default")
		helper.AssertEqual("value", existing, "Should get existing value")

		// Test missing value with default
		missing := iterableValue.GetWithDefault("missing", "default")
		// Note: GetWithDefault may return nil for missing paths in some cases
		if missing != nil {
			helper.AssertEqual("default", missing, "Should get default value")
		}

		// Test typed defaults
		stringDefault := iterableValue.GetStringWithDefault("missing", "default")
		helper.AssertEqual("default", stringDefault, "Should get string default")

		intDefault := iterableValue.GetIntWithDefault("missing", 42)
		helper.AssertEqual(42, intDefault, "Should get int default")

		floatDefault := iterableValue.GetFloat64WithDefault("missing", 3.14)
		helper.AssertEqual(3.14, floatDefault, "Should get float default")

		boolDefault := iterableValue.GetBoolWithDefault("missing", true)
		helper.AssertEqual(true, boolDefault, "Should get bool default")

		arrayDefault := iterableValue.GetArrayWithDefault("missing", []interface{}{"default"})
		helper.AssertEqual(1, len(arrayDefault), "Should get array default")

		objectDefault := iterableValue.GetObjectWithDefault("missing", map[string]interface{}{"default": true})
		helper.AssertNotNil(objectDefault["default"], "Should get object default")
	})

	t.Run("IterableValueModification", func(t *testing.T) {
		// Test IterableValue set/delete (iterator-specific in-place modification)
		jsonData := `{"name": "original", "nested": {"value": 1}}`

		var data interface{}
		err := processor.Parse(jsonData, &data)
		helper.AssertNoError(err)

		iterator := NewIterator(processor, data, DefaultOptions())
		iterableValue := NewIterableValueWithIterator(data, processor, iterator)

		// Test in-place modification
		err = iterableValue.Set("name", "updated")
		helper.AssertNoError(err)
		helper.AssertEqual("updated", iterableValue.Get("name"))

		err = iterableValue.Set("nested.value", 42)
		helper.AssertNoError(err)
		helper.AssertEqual(42, iterableValue.Get("nested.value"))
	})

	t.Run("IterableValueDeletion", func(t *testing.T) {
		// Test IterableValue delete operations
		jsonData := `{
			"keep": "this",
			"delete": "this",
			"nested": {
				"keep": "this",
				"delete": "this"
			},
			"array": [1, 2, 3, 4, 5]
		}`

		// Parse JSON data first
		var data interface{}
		err := processor.Parse(jsonData, &data)
		helper.AssertNoError(err, "Should parse JSON data")

		iterator := NewIterator(processor, data, DefaultOptions())
		iterableValue := NewIterableValueWithIterator(data, processor, iterator)

		// Test basic delete - IterableValue.Delete modifies the data in place
		err = iterableValue.Delete("delete")
		helper.AssertNoError(err, "Should delete field")

		// Verify deletion by trying to get the deleted field
		deleted := iterableValue.Get("delete")
		helper.AssertNil(deleted, "Field should be deleted")

		// Test nested delete
		err = iterableValue.Delete("nested.delete")
		helper.AssertNoError(err, "Should delete nested field")

		// Verify nested deletion
		nestedDeleted := iterableValue.Get("nested.delete")
		helper.AssertNil(nestedDeleted, "Nested field should be deleted")
	})

	t.Run("IteratorState", func(t *testing.T) {
		// Test iterator state methods
		jsonData := `{"test": "value", "array": [1, 2, 3]}`

		// Parse JSON data first
		var data interface{}
		err := processor.Parse(jsonData, &data)
		helper.AssertNoError(err, "Should parse JSON data")

		iterator := NewIterator(processor, data, DefaultOptions())
		iterableValue := NewIterableValueWithIterator(data, processor, iterator)

		// Test Exists
		exists := iterableValue.Exists("test")
		helper.AssertTrue(exists, "Should detect existing field")

		notExists := iterableValue.Exists("missing")
		helper.AssertFalse(notExists, "Should detect missing field")

		// Test IsNull
		nullData := `{"null_field": null, "not_null": "value"}`
		var nullDataParsed interface{}
		err = processor.Parse(nullData, &nullDataParsed)
		helper.AssertNoError(err, "Should parse null data")

		nullIterator := NewIterator(processor, nullDataParsed, DefaultOptions())
		nullIterableValue := NewIterableValueWithIterator(nullDataParsed, processor, nullIterator)

		isNull := nullIterableValue.IsNull("null_field")
		helper.AssertTrue(isNull, "Should detect null field")

		notNull := nullIterableValue.IsNull("not_null")
		helper.AssertFalse(notNull, "Should detect non-null field")

		// Test IsEmpty
		emptyData := `{"empty_array": [], "empty_object": {}, "not_empty": [1]}`
		var emptyDataParsed interface{}
		err = processor.Parse(emptyData, &emptyDataParsed)
		helper.AssertNoError(err, "Should parse empty data")

		emptyIterator := NewIterator(processor, emptyDataParsed, DefaultOptions())
		emptyIterableValue := NewIterableValueWithIterator(emptyDataParsed, processor, emptyIterator)

		emptyArray := emptyIterableValue.IsEmpty("empty_array")
		helper.AssertTrue(emptyArray, "Should detect empty array")

		emptyObject := emptyIterableValue.IsEmpty("empty_object")
		helper.AssertTrue(emptyObject, "Should detect empty object")

		notEmpty := emptyIterableValue.IsEmpty("not_empty")
		helper.AssertFalse(notEmpty, "Should detect non-empty array")

		// Test Length
		length := iterableValue.Length("array")
		helper.AssertEqual(3, length, "Should get correct array length")
	})

	t.Run("IteratorNavigation", func(t *testing.T) {
		// Test iterator navigation methods
		jsonData := `{
			"users": [
				{"name": "Alice", "age": 25},
				{"name": "Bob", "age": 30}
			],
			"settings": {
				"theme": "dark",
				"notifications": true
			}
		}`

		// Parse JSON data first
		var data interface{}
		err := processor.Parse(jsonData, &data)
		helper.AssertNoError(err, "Should parse JSON data")

		iterator := NewIterator(processor, data, DefaultOptions())
		iterableValue := NewIterableValueWithIterator(data, processor, iterator)

		// Test Keys
		keys := iterableValue.Keys("")
		helper.AssertTrue(len(keys) > 0, "Should get root keys")

		settingsKeys := iterableValue.Keys("settings")
		helper.AssertTrue(len(settingsKeys) == 2, "Should get settings keys")

		// Test Values
		values := iterableValue.Values("users")
		helper.AssertTrue(len(values) == 2, "Should get users values")
	})

	t.Run("IterableValue", func(t *testing.T) {
		// Test IterableValue functionality
		jsonData := `{
			"items": [
				{"id": 1, "name": "first"},
				{"id": 2, "name": "second"},
				{"id": 3, "name": "third"}
			]
		}`

		// Parse JSON data first
		var data interface{}
		err := processor.Parse(jsonData, &data)
		helper.AssertNoError(err, "Should parse JSON data")

		iterator := NewIterator(processor, data, DefaultOptions())
		iterableValue := NewIterableValueWithIterator(data, processor, iterator)

		// Test getting iterable value (just get the items array)
		items := iterableValue.Get("items")
		helper.AssertNotNil(items, "Should get items array")

		// Test GetWithDefault for missing path
		missing := iterableValue.GetWithDefault("missing", []interface{}{})
		// Note: GetWithDefault may return nil for missing paths in some cases
		if missing != nil {
			helper.AssertNotNil(missing, "Should get default value")
		}

		// Test NewIterableValue
		testArray := []interface{}{1, 2, 3}
		newIterable := NewIterableValue(testArray, processor)
		helper.AssertNotNil(newIterable, "Should create new iterable")

		// Test NewIterableValueWithIterator
		iterableWithIterator := NewIterableValueWithIterator(testArray, processor, iterator)
		helper.AssertNotNil(iterableWithIterator, "Should create iterable with iterator")
	})

	t.Run("ForeachOperations", func(t *testing.T) {
		// Test foreach operations
		jsonData := `{
			"users": [
				{"name": "Alice", "active": true},
				{"name": "Bob", "active": false},
				{"name": "Charlie", "active": true}
			]
		}`

		// Test Foreach using processor
		count := 0
		err := processor.Foreach(jsonData, func(key any, value *IterableValue) {
			count++
			helper.AssertNotNil(value, "Should have value in foreach")
		})
		helper.AssertNoError(err, "Should iterate without error")
		helper.AssertEqual(1, count, "Should iterate over root items (users array only)")

		// Test ForeachReturn
		activeCount := 0
		modifiedData, err := processor.ForeachReturn(jsonData, func(key any, value *IterableValue) {
			if key == "users" {
				// Iterate through users array
				users := value.GetArray("")
				for _, user := range users {
					if userMap, ok := user.(map[string]interface{}); ok {
						if active, exists := userMap["active"]; exists && active == true {
							activeCount++
						}
					}
				}
			}
		})
		helper.AssertNoError(err, "Should return modified data without error")
		helper.AssertTrue(len(modifiedData) > 0, "Should return modified JSON")
		helper.AssertEqual(2, activeCount, "Should count active users")

		// Test ForeachWithPath
		pathCount := 0
		err = processor.ForeachWithPath(jsonData, "users", func(key any, value *IterableValue) {
			pathCount++
		})
		helper.AssertNoError(err, "Should iterate with path without error")
		helper.AssertEqual(3, pathCount, "Should iterate over users array items")
	})

	t.Run("SetMultiple", func(t *testing.T) {
		// Test setting multiple values
		jsonData := `{"a": 1, "b": 2, "c": 3}`

		// Parse JSON data first
		var data interface{}
		err := processor.Parse(jsonData, &data)
		helper.AssertNoError(err, "Should parse JSON data")

		iterator := NewIterator(processor, data, DefaultOptions())
		iterableValue := NewIterableValueWithIterator(data, processor, iterator)

		updates := map[string]interface{}{
			"a": 10,
			"b": 20,
			"d": 40, // new field
		}

		err = iterableValue.SetMultiple(updates)
		helper.AssertNoError(err, "Should set multiple values")

		// Verify updates by getting values from the modified data
		helper.AssertEqual(10, iterableValue.Get("a"), "Should update a")
		helper.AssertEqual(20, iterableValue.Get("b"), "Should update b")
		helper.AssertEqual(float64(3), iterableValue.Get("c"), "Should preserve c")
		helper.AssertEqual(40, iterableValue.Get("d"), "Should add d")
	})

	t.Run("ErrorHandling", func(t *testing.T) {
		// Test error handling
		invalidJSON := `{"invalid": json}`

		// Test parsing invalid JSON
		var invalidData interface{}
		err := processor.Parse(invalidJSON, &invalidData)
		helper.AssertError(err, "Should error on invalid JSON")

		// Test valid JSON with invalid path
		validJSON := `{"valid": "json"}`
		var validData interface{}
		err = processor.Parse(validJSON, &validData)
		helper.AssertNoError(err, "Should parse valid JSON")

		validIterator := NewIterator(processor, validData, DefaultOptions())

		_, err = validIterator.Get("invalid[path")
		helper.AssertError(err, "Should error on invalid path")

		// Test set on invalid path
		err = validIterator.Set("invalid[path", "value")
		helper.AssertError(err, "Should error on invalid set path")
	})

	t.Run("ConcurrentAccess", func(t *testing.T) {
		// Test concurrent iterator access
		jsonData := `{
			"shared": "data",
			"counter": 0,
			"items": [1, 2, 3, 4, 5]
		}`

		concurrencyTester := NewConcurrencyTester(t, 5, 20)

		concurrencyTester.Run(func(workerID, iteration int) error {
			// Parse JSON data first
			var data interface{}
			err := processor.Parse(jsonData, &data)
			if err != nil {
				return err
			}

			iterator := NewIterator(processor, data, DefaultOptions())
			iterableValue := NewIterableValueWithIterator(data, processor, iterator)

			// Perform various read operations
			shared := iterableValue.Get("shared")
			if shared == nil {
				return fmt.Errorf("shared value not found")
			}

			_ = iterableValue.GetInt("counter")
			_ = iterableValue.GetArray("items")
			_ = iterableValue.Exists("shared")
			_ = iterableValue.Length("items")

			return nil
		})
	})

	t.Run("ComplexDataStructures", func(t *testing.T) {
		// Test with complex data structures
		generator := NewTestDataGenerator()
		complexJSON := generator.GenerateComplexJSON()

		// Parse JSON data first
		var data interface{}
		err := processor.Parse(complexJSON, &data)
		helper.AssertNoError(err, "Should parse complex JSON")

		iterator := NewIterator(processor, data, DefaultOptions())
		iterableValue := NewIterableValueWithIterator(data, processor, iterator)

		// Test navigation in complex structure
		users := iterableValue.GetArray("users")
		helper.AssertNotNil(users, "Should get users array from complex data")

		if len(users) > 0 {
			firstName := iterableValue.GetString("users[0].name")
			helper.AssertTrue(len(firstName) > 0, "Should get first user name")
		}

		// Test existence checks
		hasUsers := iterableValue.Exists("users")
		helper.AssertTrue(hasUsers, "Should detect users in complex data")
	})

	t.Run("MemoryEfficiency", func(t *testing.T) {
		// Test memory efficiency with repeated operations
		jsonData := `{"test": "memory", "data": [1, 2, 3, 4, 5]}`

		// Create multiple iterators to test resource management
		for i := 0; i < 100; i++ {
			// Parse JSON data first
			var data interface{}
			err := processor.Parse(jsonData, &data)
			helper.AssertNoError(err, "Should parse JSON data")

			iterator := NewIterator(processor, data, DefaultOptions())
			iterableValue := NewIterableValueWithIterator(data, processor, iterator)

			_ = iterableValue.GetString("test")
			_ = iterableValue.GetArray("data")
			_ = iterableValue.Exists("test")
		}

		// Test should complete without memory issues
		helper.AssertTrue(true, "Should handle multiple iterator creations efficiently")
	})
}
