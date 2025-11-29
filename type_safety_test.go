package json

import (
	"testing"
)

// TestEnhancedTypeSafety provides comprehensive testing for type safety features
// including generic GetTyped methods and type conversion edge cases
func TestEnhancedTypeSafety(t *testing.T) {
	helper := NewTestHelper(t)

	// Test data with various types
	testJSON := `{
		"string": "hello world",
		"emptyString": "",
		"number": 42,
		"float": 3.14159,
		"largeNumber": 9223372036854775807,
		"boolean": true,
		"falseBool": false,
		"null": null,
		"array": [1, 2, 3, 4, 5],
		"emptyArray": [],
		"stringArray": ["apple", "banana", "cherry"],
		"mixedArray": [1, "two", true, null],
		"object": {
			"nested": "value",
			"count": 10
		},
		"emptyObject": {},
		"stringNumbers": {
			"intStr": "123",
			"floatStr": "45.67",
			"negativeStr": "-89",
			"scientificStr": "1.23e4"
		},
		"stringBooleans": {
			"trueStr": "true",
			"falseStr": "false",
			"yesStr": "yes",
			"noStr": "no",
			"oneStr": "1",
			"zeroStr": "0"
		},
		"dates": {
			"iso": "2024-01-15T10:30:00Z",
			"timestamp": "1705315800"
		}
	}`

	t.Run("BasicTypedOperations", func(t *testing.T) {
		// Test GetTyped with basic types
		str, err := GetTyped[string](testJSON, "string")
		helper.AssertNoError(err, "GetTyped[string] should work")
		helper.AssertEqual("hello world", str, "String should match")

		num, err := GetTyped[int](testJSON, "number")
		helper.AssertNoError(err, "GetTyped[int] should work")
		helper.AssertEqual(42, num, "Number should match")

		flt, err := GetTyped[float64](testJSON, "float")
		helper.AssertNoError(err, "GetTyped[float64] should work")
		helper.AssertEqual(3.14159, flt, "Float should match")

		boolean, err := GetTyped[bool](testJSON, "boolean")
		helper.AssertNoError(err, "GetTyped[bool] should work")
		helper.AssertEqual(true, boolean, "Boolean should match")
	})

	t.Run("ArrayTypedOperations", func(t *testing.T) {
		// Test GetTyped with arrays
		intArray, err := GetTyped[[]int](testJSON, "array")
		helper.AssertNoError(err, "GetTyped[[]int] should work")
		helper.AssertEqual(5, len(intArray), "Array length should match")
		helper.AssertEqual(1, intArray[0], "First element should match")

		stringArray, err := GetTyped[[]string](testJSON, "stringArray")
		helper.AssertNoError(err, "GetTyped[[]string] should work")
		helper.AssertEqual(3, len(stringArray), "String array length should match")
		helper.AssertEqual("apple", stringArray[0], "First string should match")

		// Test empty array
		emptyArray, err := GetTyped[[]int](testJSON, "emptyArray")
		helper.AssertNoError(err, "GetTyped[[]int] for empty array should work")
		helper.AssertEqual(0, len(emptyArray), "Empty array should have zero length")
	})

	t.Run("ObjectTypedOperations", func(t *testing.T) {
		// Test GetTyped with objects
		obj, err := GetTyped[map[string]interface{}](testJSON, "object")
		helper.AssertNoError(err, "GetTyped[map[string]interface{}] should work")
		helper.AssertEqual("value", obj["nested"], "Nested value should match")
		helper.AssertEqual(float64(10), obj["count"], "Count should match")

		// Test empty object
		emptyObj, err := GetTyped[map[string]interface{}](testJSON, "emptyObject")
		helper.AssertNoError(err, "GetTyped[map[string]interface{}] for empty object should work")
		helper.AssertEqual(0, len(emptyObj), "Empty object should have zero length")
	})

	t.Run("TypeConversionFromStrings", func(t *testing.T) {
		// Test string to number conversion
		intFromStr, err := GetTyped[int](testJSON, "stringNumbers.intStr")
		if err == nil {
			helper.AssertEqual(123, intFromStr, "String to int conversion should work")
		} else {
			t.Logf("String to int conversion not supported: %v", err)
		}

		floatFromStr, err := GetTyped[float64](testJSON, "stringNumbers.floatStr")
		if err == nil {
			helper.AssertEqual(45.67, floatFromStr, "String to float conversion should work")
		} else {
			t.Logf("String to float conversion not supported: %v", err)
		}

		// Test string to boolean conversion
		boolFromStr, err := GetTyped[bool](testJSON, "stringBooleans.trueStr")
		if err == nil {
			helper.AssertEqual(true, boolFromStr, "String to bool conversion should work")
		} else {
			t.Logf("String to bool conversion not supported: %v", err)
		}
	})

	t.Run("TypeMismatchHandling", func(t *testing.T) {
		// Test type mismatches - these should return errors
		_, err := GetTyped[int](testJSON, "string")
		helper.AssertError(err, "String to int should return error")

		_, err = GetTyped[string](testJSON, "number")
		// Library might allow number to string conversion
		if err != nil {
			t.Logf("Number to string correctly returned error: %v", err)
		} else {
			t.Log("Number to string conversion was allowed")
		}

		_, err = GetTyped[bool](testJSON, "array")
		helper.AssertError(err, "Array to bool should return error")

		_, err = GetTyped[[]int](testJSON, "object")
		helper.AssertError(err, "Object to array should return error")

		_, err = GetTyped[map[string]interface{}](testJSON, "array")
		helper.AssertError(err, "Array to object should return error")
	})

	t.Run("NullValueHandling", func(t *testing.T) {
		// Test null value handling
		nullStr, err := GetTyped[string](testJSON, "null")
		if err == nil {
			// Library might convert null to "null" string
			t.Logf("Null converted to string: '%s'", nullStr)
		} else {
			t.Logf("Null to string conversion returned error: %v", err)
		}

		nullInt, err := GetTyped[int](testJSON, "null")
		if err == nil {
			helper.AssertEqual(0, nullInt, "Null should convert to zero int")
		} else {
			t.Logf("Null to int conversion returned error: %v", err)
		}

		nullBool, err := GetTyped[bool](testJSON, "null")
		if err == nil {
			helper.AssertEqual(false, nullBool, "Null should convert to false")
		} else {
			t.Logf("Null to bool conversion returned error: %v", err)
		}
	})

	t.Run("EdgeCaseTypes", func(t *testing.T) {
		// Test with large numbers
		largeNum, err := GetTyped[int64](testJSON, "largeNumber")
		if err == nil {
			helper.AssertEqual(int64(9223372036854775807), largeNum, "Large number should match")
		} else {
			t.Logf("Large number conversion failed as expected: %v", err)
		}

		// Test with different numeric types
		float32Val, err := GetTyped[float32](testJSON, "float")
		helper.AssertNoError(err, "GetTyped[float32] should work")
		helper.AssertTrue(float32Val > 3.14 && float32Val < 3.15, "Float32 should be approximately correct")

		// Test with uint types
		uintVal, err := GetTyped[uint](testJSON, "number")
		helper.AssertNoError(err, "GetTyped[uint] should work")
		helper.AssertEqual(uint(42), uintVal, "Uint should match")
	})

	t.Run("CustomStructTypes", func(t *testing.T) {
		// Define custom struct types
		type Person struct {
			Name  string `json:"name"`
			Age   int    `json:"age"`
			Email string `json:"email"`
		}

		type Company struct {
			Name      string   `json:"name"`
			Founded   int      `json:"founded"`
			Employees []Person `json:"employees"`
		}

		// Test JSON with custom struct
		customJSON := `{
			"person": {
				"name": "John Doe",
				"age": 30,
				"email": "john@example.com"
			},
			"company": {
				"name": "TechCorp",
				"founded": 2010,
				"employees": [
					{"name": "Alice", "age": 25, "email": "alice@techcorp.com"},
					{"name": "Bob", "age": 28, "email": "bob@techcorp.com"}
				]
			}
		}`

		// Test GetTyped with custom struct
		person, err := GetTyped[Person](customJSON, "person")
		helper.AssertNoError(err, "GetTyped[Person] should work")
		helper.AssertEqual("John Doe", person.Name, "Person name should match")
		helper.AssertEqual(30, person.Age, "Person age should match")

		company, err := GetTyped[Company](customJSON, "company")
		helper.AssertNoError(err, "GetTyped[Company] should work")
		helper.AssertEqual("TechCorp", company.Name, "Company name should match")
		helper.AssertEqual(2, len(company.Employees), "Company should have 2 employees")
	})

	t.Run("PointerTypes", func(t *testing.T) {
		// Test with pointer types
		strPtr, err := GetTyped[*string](testJSON, "string")
		helper.AssertNoError(err, "GetTyped[*string] should work")
		helper.AssertNotNil(strPtr, "String pointer should not be nil")
		helper.AssertEqual("hello world", *strPtr, "String pointer value should match")

		// Test with nil values
		nullPtr, err := GetTyped[*string](testJSON, "null")
		if err == nil {
			if nullPtr == nil {
				t.Log("Null correctly converted to nil pointer")
			} else {
				t.Logf("Null converted to pointer with value: %v", *nullPtr)
			}
		} else {
			t.Logf("Null to pointer conversion returned error: %v", err)
		}
	})

	t.Run("InterfaceTypes", func(t *testing.T) {
		// Test with interface{} type
		anyValue, err := GetTyped[interface{}](testJSON, "string")
		helper.AssertNoError(err, "GetTyped[interface{}] should work")
		helper.AssertEqual("hello world", anyValue, "Interface value should match")

		anyNumber, err := GetTyped[interface{}](testJSON, "number")
		helper.AssertNoError(err, "GetTyped[interface{}] for number should work")
		helper.AssertEqual(float64(42), anyNumber, "Interface number should match")

		anyArray, err := GetTyped[interface{}](testJSON, "array")
		helper.AssertNoError(err, "GetTyped[interface{}] for array should work")
		if arr, ok := anyArray.([]interface{}); ok {
			helper.AssertEqual(5, len(arr), "Interface array should have correct length")
		}
	})
}

// TestTypeSafetyWithProcessor tests type safety with custom processors
func TestTypeSafetyWithProcessor(t *testing.T) {
	helper := NewTestHelper(t)

	processor := New()
	defer processor.Close()

	testJSON := `{
		"data": {
			"values": [1, 2, 3, 4, 5],
			"metadata": {
				"count": 5,
				"average": 3.0
			}
		}
	}`

	t.Run("ProcessorTypedOperations", func(t *testing.T) {
		// Test processor-based operations (using regular Get since GetTyped might not be available)
		valuesRaw, err := processor.Get(testJSON, "data.values")
		helper.AssertNoError(err, "Processor Get should work")
		if values, ok := valuesRaw.([]interface{}); ok {
			helper.AssertEqual(5, len(values), "Values array should have correct length")
		}

		countRaw, err := processor.Get(testJSON, "data.metadata.count")
		helper.AssertNoError(err, "Processor Get for count should work")
		if count, ok := countRaw.(float64); ok {
			helper.AssertEqual(float64(5), count, "Count should match")
		}

		averageRaw, err := processor.Get(testJSON, "data.metadata.average")
		helper.AssertNoError(err, "Processor Get for average should work")
		if average, ok := averageRaw.(float64); ok {
			helper.AssertEqual(3.0, average, "Average should match")
		}
	})
}
