package json

import (
	"testing"
)

// TestArrayOperationsComprehensive tests all array operations comprehensively
// Merged from: array_path_test.go, set_multiple_test.go, and related array tests
func TestArrayOperationsComprehensive(t *testing.T) {
	helper := NewTestHelper(t)

	testData := `{
		"numbers": [1, 2, 3, 4, 5, 6, 7, 8, 9, 10],
		"users": [
			{"name": "Alice", "age": 25, "skills": ["Go", "Python"]},
			{"name": "Bob", "age": 30, "skills": ["Java", "React"]},
			{"name": "Charlie", "age": 35, "skills": ["C++", "Rust"]},
			{"name": "Diana", "age": 28, "skills": ["Vue", "JavaScript"]}
		],
		"matrix": [
			[1, 2, 3],
			[4, 5, 6],
			[7, 8, 9]
		],
		"empty": [],
		"mixed": [1, "string", true, null, {"key": "value"}]
	}`

	t.Run("BasicArrayAccess", func(t *testing.T) {
		// Test positive index
		first, err := GetInt(testData, "numbers[0]")
		helper.AssertNoError(err, "Should get first element")
		helper.AssertEqual(1, first, "First element should be 1")

		// Test negative index
		last, err := GetInt(testData, "numbers[-1]")
		helper.AssertNoError(err, "Should get last element")
		helper.AssertEqual(10, last, "Last element should be 10")

		// Test out of bounds (should return nil, not error)
		outOfBounds, err := Get(testData, "numbers[100]")
		helper.AssertNoError(err, "Out of bounds should not error")
		helper.AssertNil(outOfBounds, "Out of bounds should return nil")

		// Test negative out of bounds
		negOutOfBounds, err := Get(testData, "numbers[-100]")
		helper.AssertNoError(err, "Negative out of bounds should not error")
		helper.AssertNil(negOutOfBounds, "Negative out of bounds should return nil")
	})

	t.Run("NestedArrayAccess", func(t *testing.T) {
		// Test nested object in array
		userName, err := GetString(testData, "users[1].name")
		helper.AssertNoError(err, "Should get nested object property")
		helper.AssertEqual("Bob", userName, "User name should match")

		// Test nested array in array
		matrixElement, err := GetInt(testData, "matrix[1][2]")
		helper.AssertNoError(err, "Should get matrix element")
		helper.AssertEqual(6, matrixElement, "Matrix element should be 6")

		// Test nested array access with negative index
		lastUserName, err := GetString(testData, "users[-1].name")
		helper.AssertNoError(err, "Should get last user name")
		helper.AssertEqual("Diana", lastUserName, "Last user name should be Diana")
	})

	t.Run("ArraySlicing", func(t *testing.T) {
		// Test basic slice [start:end]
		slice, err := Get(testData, "numbers[2:5]")
		helper.AssertNoError(err, "Should get array slice")
		expected := []any{float64(3), float64(4), float64(5)}
		helper.AssertEqual(expected, slice, "Slice should match")

		// Test slice from start [start:]
		fromStart, err := Get(testData, "numbers[7:]")
		helper.AssertNoError(err, "Should get slice from start")
		expectedFromStart := []any{float64(8), float64(9), float64(10)}
		helper.AssertEqual(expectedFromStart, fromStart, "Slice from start should match")

		// Test slice to end [:end]
		toEnd, err := Get(testData, "numbers[:3]")
		helper.AssertNoError(err, "Should get slice to end")
		expectedToEnd := []any{float64(1), float64(2), float64(3)}
		helper.AssertEqual(expectedToEnd, toEnd, "Slice to end should match")

		// Test full slice [:]
		fullSlice, err := Get(testData, "numbers[:]")
		helper.AssertNoError(err, "Should get full slice")
		if fullArray, ok := fullSlice.([]any); ok {
			helper.AssertEqual(10, len(fullArray), "Full slice should have all elements")
		}
	})

	t.Run("ArraySlicingWithStep", func(t *testing.T) {
		// Test slice with step [start:end:step]
		stepSlice, err := Get(testData, "numbers[1:8:2]")
		helper.AssertNoError(err, "Should get slice with step")
		expectedStep := []any{float64(2), float64(4), float64(6), float64(8)}
		helper.AssertEqual(expectedStep, stepSlice, "Step slice should match")

		// Test slice with step from start [start::step]
		stepFromStart, err := Get(testData, "numbers[::3]")
		helper.AssertNoError(err, "Should get step slice from start")
		expectedStepFromStart := []any{float64(1), float64(4), float64(7), float64(10)}
		helper.AssertEqual(expectedStepFromStart, stepFromStart, "Step slice from start should match")

		// Test slice with negative step (reverse)
		reverseSlice, err := Get(testData, "numbers[::-1]")
		helper.AssertNoError(err, "Should get reverse slice")
		if reverseArray, ok := reverseSlice.([]any); ok {
			helper.AssertEqual(10, len(reverseArray), "Reverse slice should have all elements")
			helper.AssertEqual(float64(10), reverseArray[0], "First element should be 10")
			helper.AssertEqual(float64(1), reverseArray[9], "Last element should be 1")
		}
	})

	t.Run("NegativeIndexSlicing", func(t *testing.T) {
		// Test negative start index
		negStartSlice, err := Get(testData, "numbers[-3:]")
		helper.AssertNoError(err, "Should get slice with negative start")
		expectedNegStart := []any{float64(8), float64(9), float64(10)}
		helper.AssertEqual(expectedNegStart, negStartSlice, "Negative start slice should match")

		// Test negative end index
		negEndSlice, err := Get(testData, "numbers[:-2]")
		helper.AssertNoError(err, "Should get slice with negative end")
		if negEndArray, ok := negEndSlice.([]any); ok {
			helper.AssertEqual(8, len(negEndArray), "Negative end slice should have 8 elements")
			helper.AssertEqual(float64(8), negEndArray[7], "Last element should be 8")
		}

		// Test both negative indices
		bothNegSlice, err := Get(testData, "numbers[-5:-2]")
		helper.AssertNoError(err, "Should get slice with both negative indices")
		expectedBothNeg := []any{float64(6), float64(7), float64(8)}
		helper.AssertEqual(expectedBothNeg, bothNegSlice, "Both negative slice should match")
	})

	t.Run("EmptyArrayHandling", func(t *testing.T) {
		// Test access to empty array
		emptyAccess, err := Get(testData, "empty[0]")
		helper.AssertNoError(err, "Empty array access should not error")
		helper.AssertNil(emptyAccess, "Empty array access should return nil")

		// Test slice of empty array
		emptySlice, err := Get(testData, "empty[0:5]")
		helper.AssertNoError(err, "Empty array slice should not error")
		if emptySliceArray, ok := emptySlice.([]any); ok {
			helper.AssertEqual(0, len(emptySliceArray), "Empty array slice should be empty")
		} else {
			helper.AssertNil(emptySlice, "Empty array slice should be nil or empty")
		}
	})

	t.Run("MixedTypeArrayHandling", func(t *testing.T) {
		// Test accessing different types in mixed array
		intValue, err := GetInt(testData, "mixed[0]")
		helper.AssertNoError(err, "Should get int from mixed array")
		helper.AssertEqual(1, intValue, "Int value should match")

		stringValue, err := GetString(testData, "mixed[1]")
		helper.AssertNoError(err, "Should get string from mixed array")
		helper.AssertEqual("string", stringValue, "String value should match")

		boolValue, err := GetBool(testData, "mixed[2]")
		helper.AssertNoError(err, "Should get bool from mixed array")
		helper.AssertTrue(boolValue, "Bool value should be true")

		nullValue, err := Get(testData, "mixed[3]")
		helper.AssertNoError(err, "Should get null from mixed array")
		helper.AssertNil(nullValue, "Null value should be nil")

		objectValue, err := Get(testData, "mixed[4].key")
		helper.AssertNoError(err, "Should get object property from mixed array")
		helper.AssertEqual("value", objectValue, "Object value should match")
	})
}

// TestArrayModificationOperations tests array modification operations
func TestArrayModificationOperations(t *testing.T) {
	helper := NewTestHelper(t)

	baseData := `{
		"numbers": [1, 2, 3, 4, 5],
		"users": [
			{"name": "Alice", "active": false},
			{"name": "Bob", "active": false},
			{"name": "Charlie", "active": false}
		]
	}`

	t.Run("SetArrayElement", func(t *testing.T) {
		// Test setting array element by index
		newData, err := Set(baseData, "numbers[2]", 99)
		helper.AssertNoError(err, "Should set array element")

		newValue, err := GetInt(newData, "numbers[2]")
		helper.AssertNoError(err, "Should get modified array element")
		helper.AssertEqual(99, newValue, "Modified value should match")

		// Verify other elements unchanged
		firstValue, err := GetInt(newData, "numbers[0]")
		helper.AssertNoError(err, "Should get first element")
		helper.AssertEqual(1, firstValue, "First element should be unchanged")
	})

	t.Run("SetNestedArrayElement", func(t *testing.T) {
		// Test setting nested object property in array
		newData, err := Set(baseData, "users[1].active", true)
		helper.AssertNoError(err, "Should set nested array element property")

		newValue, err := GetBool(newData, "users[1].active")
		helper.AssertNoError(err, "Should get modified nested property")
		helper.AssertTrue(newValue, "Modified nested value should be true")

		// Verify other elements unchanged
		firstUserActive, err := GetBool(newData, "users[0].active")
		helper.AssertNoError(err, "Should get first user active status")
		helper.AssertFalse(firstUserActive, "First user should still be inactive")
	})

	t.Run("SetMultipleArrayElements", func(t *testing.T) {
		// Test setting multiple array elements
		data1, err := Set(baseData, "users[0].active", true)
		helper.AssertNoError(err, "Should set first user active")

		data2, err := Set(data1, "users[2].active", true)
		helper.AssertNoError(err, "Should set third user active")

		// Verify both changes
		firstActive, err := GetBool(data2, "users[0].active")
		helper.AssertNoError(err, "Should get first user status")
		helper.AssertTrue(firstActive, "First user should be active")

		thirdActive, err := GetBool(data2, "users[2].active")
		helper.AssertNoError(err, "Should get third user status")
		helper.AssertTrue(thirdActive, "Third user should be active")

		// Verify middle user unchanged
		secondActive, err := GetBool(data2, "users[1].active")
		helper.AssertNoError(err, "Should get second user status")
		helper.AssertFalse(secondActive, "Second user should still be inactive")
	})

	t.Run("SetArrayElementWithNegativeIndex", func(t *testing.T) {
		// Test setting array element with negative index
		newData, err := Set(baseData, "numbers[-1]", 50)
		helper.AssertNoError(err, "Should set last array element")

		lastValue, err := GetInt(newData, "numbers[-1]")
		helper.AssertNoError(err, "Should get last element")
		helper.AssertEqual(50, lastValue, "Last element should be modified")

		// Also verify by positive index
		lastValuePos, err := GetInt(newData, "numbers[4]")
		helper.AssertNoError(err, "Should get last element by positive index")
		helper.AssertEqual(50, lastValuePos, "Last element should match")
	})
}
