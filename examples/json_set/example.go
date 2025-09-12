package main

// ==================================================================================
// This is a comprehensive example of JSON Set functionality
//
// This example demonstrates different approaches to setting JSON data:
// 1. Basic Set operations - setting values at specific paths
// 2. SetWithAdd operations - setting values with automatic path creation
// 3. Custom Set operations with ProcessorOptions - fine-grained control
// 4. Array element setting - setting specific array elements by index
// 5. Array range setting - setting ranges of array elements
// 6. Nested object creation - creating complex nested structures
// 7. Batch setting operations - setting multiple values efficiently
// 8. Type-specific setting - setting different data types (strings, numbers, booleans, arrays, objects)
//
// The example uses various JSON structures to show how Set operations work with:
// - Simple objects and arrays
// - Complex nested structures
// - Path expressions with wildcards ({})
// - Array indexing and slicing
// - Automatic path creation vs. strict mode
//
// Key features demonstrated:
// - Difference between Set() and SetWithAdd() behaviors
// - Handling of non-existent paths
// - Creating nested structures automatically
// - Setting array elements and ranges
// - Batch operations for efficiency
// - Error handling and validation
// ==================================================================================

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/cybergodev/json"
)

// Sample JSON data for basic Set examples
const basicJsonStr = `{
	"user": {
		"name": "John",
		"age": 30,
		"email": "john@example.com"
	},
	"settings": {
		"theme": "dark",
		"notifications": true
	}
}`

// Sample JSON data for array Set examples
const arrayJsonStr = `{
	"users": [
		{"id": 1, "name": "Alice", "age": 25, "active": true},
		{"id": 2, "name": "Bob", "age": 30, "active": false},
		{"id": 3, "name": "Charlie", "age": 35, "active": true}
	],
	"tags": ["admin", "user", "guest"],
	"metadata": {
		"total": 3,
		"version": "1.0"
	}
}`

// Sample JSON data for complex nested Set examples
const complexJsonStr = `{
	"company": {
		"name": "TechCorp",
		"departments": [
			{
				"name": "Engineering",
				"budget": 1000000,
				"employees": [
					{"name": "John", "salary": 80000, "skills": ["Go", "Python"]},
					{"name": "Jane", "salary": 85000, "skills": ["JavaScript", "React"]}
				]
			},
			{
				"name": "Marketing",
				"budget": 500000,
				"employees": [
					{"name": "Mary", "salary": 70000, "skills": ["SEO", "Content"]},
					{"name": "Mike", "salary": 72000, "skills": ["Analytics", "PPC"]}
				]
			}
		]
	}
}`

// Empty JSON for path creation examples
const emptyJsonStr = `{}`

func main() {

	// j := map[string]any{
	// 	"user": []any{
	// 		map[string]any{"id": 1, "name": "John", "friend": []any{
	// 			map[string]any{"name": "Alice"},
	// 			map[string]any{"name": "Bob"},
	// 		}},
	// 		map[string]any{"id": 1, "name": "David", "friend": []any{
	// 			map[string]any{"name": "Susan"},
	// 			map[string]any{"name": "Carol"},
	// 		}},
	// 	},
	// }
	//
	// en, err := json.Encode(j)
	// if err != nil {
	// 	return
	// }
	//
	// fmt.Println(en)
	// return

	// Basic Set examples
	printLines("=== Basic Set Examples ===")
	basicSetOperations()
	setWithAddOperations()

	// Custom options Set
	printLines("=== Custom Options Set ===")
	setWithCustomOptions()

	// Array Set examples
	printLines("=== Array Set Examples ===")
	setArrayElements()
	setArrayRanges()

	// Path creation examples
	printLines("=== Path Creation Examples ===")
	createNestedPaths()

	// Complex nested Set examples
	printLines("=== Complex Nested Set Examples ===")
	setNestedElements()

	// Batch operations example
	printLines("=== Batch Operations Example ===")
	batchSetOperations()

	// Type-specific examples
	printLines("=== Type-Specific Set Examples ===")
	setDifferentTypes()

	// Error handling examples
	printLines("=== Error Handling Examples ===")
	errorHandlingExamples()
}

// Basic Set operations using the standard Set() function
func basicSetOperations() {

	// Set existing field
	fmt.Println("\nSetting existing field 'user.age' to 31:")
	result, err := json.Set(basicJsonStr, "user.age", 31)
	if err != nil {
		log.Println("Error:", err)
	} else {
		fmt.Println("Note: Existing field value is updated")
		fmt.Println("Result:\n", result)
	}

	// Try to set non-existent field with explicit no-create option
	fmt.Println("\nTrying to set non-existent field 'user.city' (with CreatePaths=false):")
	opts := &json.ProcessorOptions{
		CreatePaths: false,
	}
	result2, err := json.Set(basicJsonStr, "user.city", "New York", opts)
	if err != nil {
		fmt.Printf("Expected error: %v\n", err)
		fmt.Println("Note: Set() with CreatePaths=false requires existing paths")
	} else {
		fmt.Println("Note: Path was created (library may default to CreatePaths=true)")
		fmt.Println("Result:\n", result2)
	}

	// Set nested existing field
	fmt.Println("\nSetting nested field 'settings.theme' to 'light':")
	result3, err := json.Set(basicJsonStr, "settings.theme", "light")
	if err != nil {
		log.Println("Error:", err)
	} else {
		fmt.Println("Note: Nested field successfully updated")
		fmt.Println("Result:\n", result3)
	}
}

// SetWithAdd operations that automatically create paths
func setWithAddOperations() {

	// SetWithAdd creates paths automatically
	fmt.Println("\nUsing SetWithAdd to create new field 'user.city':")
	result, err := json.SetWithAdd(basicJsonStr, "user.city", "New York")
	if err != nil {
		log.Println("Error:", err)
	} else {
		fmt.Println("Note: SetWithAdd automatically creates missing paths")
		fmt.Println("Result:\n", result)
	}

	// Create deeply nested path
	fmt.Println("\nCreating deeply nested path 'user.profile.social.twitter':")
	result2, err := json.SetWithAdd(basicJsonStr, "user.profile.social.twitter", "@john_doe")
	if err != nil {
		log.Println("Error:", err)
	} else {
		fmt.Println("Note: Multiple levels of nesting created automatically")
		fmt.Println("Result:\n", result2)
	}

	// Add new top-level object
	fmt.Println("\nAdding new top-level object 'preferences':")
	preferences := map[string]any{
		"language": "en",
		"timezone": "UTC",
		"currency": "USD",
	}
	result3, err := json.SetWithAdd(basicJsonStr, "preferences", preferences)
	if err != nil {
		log.Println("Error:", err)
	} else {
		fmt.Println("Note: Complex objects can be added as values")
		fmt.Println("Result:\n", result3)
	}
}

// Set with custom ProcessorOptions for fine-grained control
func setWithCustomOptions() {

	// Set with CreatePaths option
	fmt.Println("\nUsing Set() with CreatePaths option:")
	opts := &json.ProcessorOptions{
		CreatePaths: true,
	}
	result, err := json.Set(basicJsonStr, "user.address.street", "123 Main St", opts)
	if err != nil {
		log.Println("Error:", err)
	} else {
		fmt.Println("Note: CreatePaths option allows path creation with Set()")
		fmt.Println("Result:\n", result)
	}

	// Set with StrictMode option (test with existing path)
	fmt.Println("\nUsing Set() with StrictMode option:")
	opts2 := &json.ProcessorOptions{
		StrictMode:  true,
		CreatePaths: false,
	}
	result2, err := json.Set(basicJsonStr, "user.age", 32, opts2)
	if err != nil {
		fmt.Printf("Error in strict mode: %v\n", err)
	} else {
		fmt.Println("Note: StrictMode allows updates to existing paths")
		fmt.Println("Result:\n", result2)
	}

	// Test StrictMode with non-existent path
	fmt.Println("\nTesting StrictMode with non-existent path:")
	result2b, err := json.Set(basicJsonStr, "user.nonexistent", "value", opts2)
	if err != nil {
		fmt.Printf("Expected error in strict mode: %v\n", err)
		fmt.Println("Note: StrictMode prevents setting non-existent paths")
	} else {
		fmt.Println("Note: Library may not fully implement StrictMode yet")
		fmt.Println("Unexpected success:\n", result2b)
	}

	// Set with multiple options
	fmt.Println("\nUsing Set() with multiple options:")
	opts3 := &json.ProcessorOptions{
		CreatePaths:   true,
		StrictMode:    false,
		CleanupNulls:  true,
		CompactArrays: true,
	}
	result3, err := json.Set(basicJsonStr, "user.metadata.lastLogin", time.Now().Format(time.RFC3339), opts3)
	if err != nil {
		log.Println("Error:", err)
	} else {
		fmt.Println("Note: Multiple options provide fine-grained control")
		fmt.Println("Result:\n", result3)
	}
}

// Set specific array elements by index
func setArrayElements() {

	// Set array element by positive index
	fmt.Println("\nSetting user at index 1 (Bob's age to 31):")
	result, err := json.Set(arrayJsonStr, "users[1].age", 31)
	if err != nil {
		log.Println("Error:", err)
	} else {
		fmt.Println("Note: Array element updated by index")
		fmt.Println("Result:\n", result)
	}

	// Set array element by negative index
	fmt.Println("\nSetting last user (index -1) active status to false:")
	result2, err := json.Set(arrayJsonStr, "users[-1].active", false)
	if err != nil {
		log.Println("Error:", err)
	} else {
		fmt.Println("Note: Negative indexing works from the end of array")
		fmt.Println("Result:\n", result2)
	}

	// Set simple array element
	fmt.Println("\nSetting tag at index 1 to 'moderator':")
	result3, err := json.Set(arrayJsonStr, "tags[1]", "moderator")
	if err != nil {
		log.Println("Error:", err)
	} else {
		fmt.Println("Note: Simple array elements can be updated")
		fmt.Println("Result:\n", result3)
	}

	// Add new array element by extending the array manually
	fmt.Println("\nAdding new user by extending array:")

	// First get the current users array
	currentUsers, err := json.Get(arrayJsonStr, "users")
	if err != nil {
		log.Println("Error getting current users:", err)
		return
	}

	// Convert to slice and append new user
	if usersArray, ok := currentUsers.([]any); ok {
		newUser := map[string]any{
			"id":     4,
			"name":   "David",
			"age":    28,
			"active": true,
		}
		extendedUsers := append(usersArray, newUser)

		// Set the entire array back
		result4, err := json.Set(arrayJsonStr, "users", extendedUsers)
		if err != nil {
			log.Println("Error:", err)
		} else {
			fmt.Println("Note: Arrays can be extended by replacing the entire array")
			fmt.Println("Result:\n", result4)
		}
	}
}

// Set ranges of array elements
func setArrayRanges() {

	// Set range of array elements
	fmt.Println("\nSetting active status for users[0:2] to false:")
	result, err := json.Set(arrayJsonStr, "users[0:2]{active}", false)
	if err != nil {
		log.Println("Error:", err)
	} else {
		fmt.Println("Note: Range operations affect multiple elements")
		fmt.Println("Result:\n", result)
	}

	// Set from index to end
	fmt.Println("\nSetting age for users[1:] to 35:")
	result2, err := json.Set(arrayJsonStr, "users[1:]{age}", 35)
	if err != nil {
		log.Println("Error:", err)
	} else {
		fmt.Println("Note: Open-ended range affects from index to end")
		fmt.Println("Result:\n", result2)
	}

	// Set using wildcard notation
	fmt.Println("\nSetting all users' active status using wildcard:")
	result3, err := json.Set(arrayJsonStr, "users{active}", true)
	if err != nil {
		log.Println("Error:", err)
	} else {
		fmt.Println("Note: Wildcard {} notation affects all array elements")
		fmt.Println("Result:\n", result3)
	}

	// Replace entire array
	fmt.Println("\nReplacing entire tags array:")
	newTags := []string{"admin", "moderator", "user", "guest", "visitor"}
	result4, err := json.Set(arrayJsonStr, "tags", newTags)
	if err != nil {
		log.Println("Error:", err)
	} else {
		fmt.Println("Note: Entire arrays can be replaced")
		fmt.Println("Result:\n", result4)
	}
}

// Create nested paths from empty JSON
func createNestedPaths() {
	fmt.Println("Starting with empty JSON:\n", emptyJsonStr)

	// Create simple nested path
	fmt.Println("\nCreating simple nested path 'user.name':")
	result, err := json.SetWithAdd(emptyJsonStr, "user.name", "Alice")
	if err != nil {
		log.Println("Error:", err)
	} else {
		fmt.Println("Note: Basic nested structure created")
		fmt.Println("Result:\n", result)
	}

	// Create complex nested structure
	fmt.Println("\nCreating complex nested structure:")
	workingJson := emptyJsonStr

	// Step by step creation
	workingJson, _ = json.SetWithAdd(workingJson, "company.name", "TechCorp")
	workingJson, _ = json.SetWithAdd(workingJson, "company.founded", 2020)
	workingJson, _ = json.SetWithAdd(workingJson, "company.employees[0].name", "John")
	workingJson, _ = json.SetWithAdd(workingJson, "company.employees[0].position", "Developer")
	workingJson, _ = json.SetWithAdd(workingJson, "company.employees[0].skills", []string{"Go", "Python", "JavaScript"})
	workingJson, _ = json.SetWithAdd(workingJson, "company.employees[1].name", "Jane")
	workingJson, _ = json.SetWithAdd(workingJson, "company.employees[1].position", "Designer")
	workingJson, _ = json.SetWithAdd(workingJson, "company.employees[1].skills", []string{"UI/UX", "Figma", "CSS"})
	workingJson, _ = json.SetWithAdd(workingJson, "company.address.street", "123 Tech Street")
	workingJson, _ = json.SetWithAdd(workingJson, "company.address.city", "San Francisco")
	workingJson, _ = json.SetWithAdd(workingJson, "company.address.country", "USA")

	prettyResult, err := json.FormatPretty(workingJson)
	if err != nil {
		fmt.Println("Error formatting result:", err)
		fmt.Println("Raw result:", workingJson)
	} else {
		fmt.Println("Final complex structure:\n", prettyResult)
	}
	fmt.Println("Note: Complex nested structures built step by step")

	// Create structure with single operation
	fmt.Println("\nCreating structure with single complex object:")
	complexObject := map[string]any{
		"department": "Engineering",
		"budget":     1000000,
		"projects": []map[string]any{
			{
				"name":     "Project Alpha",
				"status":   "active",
				"priority": "high",
				"team":     []string{"Alice", "Bob", "Charlie"},
			},
			{
				"name":     "Project Beta",
				"status":   "planning",
				"priority": "medium",
				"team":     []string{"David", "Eve"},
			},
		},
	}

	result2, err := json.SetWithAdd(emptyJsonStr, "organization.engineering", complexObject)
	if err != nil {
		log.Println("Error:", err)
	} else {
		fmt.Println("Note: Complex structures can be set in single operations")
		fmt.Println("Result:\n", result2)
	}
}

// Set elements in complex nested structures
func setNestedElements() {

	// Set nested field in array element
	fmt.Println("\nSetting salary for first Engineering employee:")
	result, err := json.Set(complexJsonStr, "company.departments[0].employees[0].salary", 85000)
	if err != nil {
		log.Println("Error:", err)
	} else {
		fmt.Println("Note: Deep nested array element updated")
		fmt.Println("Result:\n", result)
	}

	// Set using wildcard to update all employees
	fmt.Println("\nAdding 'remote' skill to all employees using wildcard:")

	result2, err := json.Set(complexJsonStr, "company.departments{employees}{skills}", []string{"Remote Work"})
	if err != nil {
		log.Println("Error:", err)
	} else {
		fmt.Println("Note: Wildcard notation updates all matching elements")
		fmt.Println("Result:\n", result2)
	}

	// Add new department by extending the departments array
	fmt.Println("\nAdding new HR department:")

	// Get current departments array
	currentDepts, err := json.Get(complexJsonStr, "company.departments")
	if err != nil {
		log.Println("Error getting departments:", err)
		return
	}

	if deptsArray, ok := currentDepts.([]any); ok {
		hrDept := map[string]any{
			"name":   "Human Resources",
			"budget": 300000,
			"employees": []map[string]any{
				{
					"name":   "Sarah",
					"salary": 65000,
					"skills": []string{"Recruitment", "Employee Relations"},
				},
			},
		}

		// Extend the departments array
		extendedDepts := append(deptsArray, hrDept)
		result3, err := json.Set(complexJsonStr, "company.departments", extendedDepts)
		if err != nil {
			log.Println("Error:", err)
		} else {
			fmt.Println("Note: Arrays can be extended by replacing the entire array")
			fmt.Println("Result:\n", result3)
		}
	}

	// Update company-level information
	fmt.Println("\nUpdating company information:")
	workingJson := complexJsonStr
	workingJson, _ = json.Set(workingJson, "company.name", "TechCorp International")
	workingJson, _ = json.SetWithAdd(workingJson, "company.founded", 2015)
	workingJson, _ = json.SetWithAdd(workingJson, "company.headquarters", "San Francisco, CA")
	workingJson, _ = json.SetWithAdd(workingJson, "company.totalEmployees", 150)

	fmt.Println("Updated company information:\n", workingJson)
	fmt.Println("Note: Multiple updates can be chained together")
}

// Demonstrate batch Set operations for efficiency
func batchSetOperations() {

	// Batch update user information
	fmt.Println("\nPerforming batch updates on user data:")
	workingJson := arrayJsonStr

	// Update multiple fields for each user
	workingJson, _ = json.Set(workingJson, "users[0].age", 26)
	workingJson, _ = json.SetWithAdd(workingJson, "users[0].department", "Engineering")
	workingJson, _ = json.SetWithAdd(workingJson, "users[0].lastLogin", "2024-01-15T10:30:00Z")

	workingJson, _ = json.Set(workingJson, "users[1].age", 31)
	workingJson, _ = json.SetWithAdd(workingJson, "users[1].department", "Marketing")
	workingJson, _ = json.SetWithAdd(workingJson, "users[1].lastLogin", "2024-01-14T15:45:00Z")

	workingJson, _ = json.Set(workingJson, "users[2].age", 36)
	workingJson, _ = json.SetWithAdd(workingJson, "users[2].department", "Sales")
	workingJson, _ = json.SetWithAdd(workingJson, "users[2].lastLogin", "2024-01-16T09:15:00Z")

	// Add metadata
	workingJson, _ = json.Set(workingJson, "metadata.total", 3)
	workingJson, _ = json.Set(workingJson, "metadata.version", "2.0")
	workingJson, _ = json.SetWithAdd(workingJson, "metadata.lastUpdated", time.Now().Format(time.RFC3339))
	workingJson, _ = json.SetWithAdd(workingJson, "metadata.updatedBy", "admin")

	fmt.Println("Note: Multiple Set operations can be chained for batch updates")
	fmt.Println("Result after batch operations:\n", workingJson)

	// Demonstrate bulk field addition using wildcards
	fmt.Println("\nAdding 'status' field to all users using wildcard:")
	result2, err := json.Set(workingJson, "users{status}", "active")
	if err != nil {
		log.Println("Error:", err)
	} else {
		fmt.Println("Note: Wildcard operations are efficient for bulk updates")
		fmt.Println("Result:\n", result2)
	}
}

// Demonstrate setting different data types
func setDifferentTypes() {
	fmt.Println("Starting with empty JSON for type demonstrations:", emptyJsonStr)

	workingJson := emptyJsonStr

	// Set string values
	fmt.Println("\nSetting string values:")
	workingJson, _ = json.SetWithAdd(workingJson, "data.string", "Hello, World!")
	workingJson, _ = json.SetWithAdd(workingJson, "data.emptyString", "")
	workingJson, _ = json.SetWithAdd(workingJson, "data.unicodeString", "Hello 世界 🌍")

	// Set numeric values
	fmt.Println("Setting numeric values:")
	workingJson, _ = json.SetWithAdd(workingJson, "data.integer", 42)
	workingJson, _ = json.SetWithAdd(workingJson, "data.float", 3.14159)
	workingJson, _ = json.SetWithAdd(workingJson, "data.negative", -100)
	workingJson, _ = json.SetWithAdd(workingJson, "data.zero", 0)

	// Set boolean values
	fmt.Println("Setting boolean values:")
	workingJson, _ = json.SetWithAdd(workingJson, "data.boolTrue", true)
	workingJson, _ = json.SetWithAdd(workingJson, "data.boolFalse", false)

	// Set null value
	fmt.Println("Setting null value:")
	workingJson, _ = json.SetWithAdd(workingJson, "data.nullValue", nil)

	// Set array values
	fmt.Println("Setting array values:")
	workingJson, _ = json.SetWithAdd(workingJson, "data.stringArray", []string{"apple", "banana", "cherry"})
	workingJson, _ = json.SetWithAdd(workingJson, "data.numberArray", []int{1, 2, 3, 4, 5})
	workingJson, _ = json.SetWithAdd(workingJson, "data.mixedArray", []any{"string", 42, true, nil})
	workingJson, _ = json.SetWithAdd(workingJson, "data.emptyArray", []any{})

	// Set object values
	fmt.Println("Setting object values:")
	simpleObject := map[string]any{
		"name": "Simple Object",
		"id":   1,
	}
	workingJson, _ = json.SetWithAdd(workingJson, "data.simpleObject", simpleObject)

	nestedObject := map[string]any{
		"level1": map[string]any{
			"level2": map[string]any{
				"level3": "deep value",
				"array":  []int{1, 2, 3},
			},
		},
	}
	workingJson, _ = json.SetWithAdd(workingJson, "data.nestedObject", nestedObject)

	// Set empty object
	workingJson, _ = json.SetWithAdd(workingJson, "data.emptyObject", map[string]any{})

	fmt.Println("Note: All JSON data types are supported")
	fmt.Println("Final result with all data types:\n", workingJson)

	// Demonstrate type conversion behavior
	fmt.Println("\nDemonstrating type updates:")

	// Change string to number
	workingJson, _ = json.Set(workingJson, "data.string", 123)

	// Change number to boolean
	workingJson, _ = json.Set(workingJson, "data.integer", false)

	// Change boolean to array
	workingJson, _ = json.Set(workingJson, "data.boolTrue", []string{"was", "boolean", "now", "array"})

	fmt.Println("Note: Values can be changed to different types")
	fmt.Println("After type changes:\n", workingJson)
}

// Demonstrate error handling scenarios
func errorHandlingExamples() {
	fmt.Println("Demonstrating error handling scenarios:")

	// Invalid JSON
	fmt.Println("\n1. Invalid JSON input:")
	invalidJson := `{"invalid": json}`
	_, err := json.Set(invalidJson, "field", "value")
	if err != nil {
		fmt.Printf("Expected error for invalid JSON: %v\n", err)
	}

	// Invalid path syntax
	fmt.Println("\n2. Invalid path syntax:")
	_, err = json.Set(basicJsonStr, "invalid..path", "value")
	if err != nil {
		fmt.Printf("Expected error for invalid path: %v\n", err)
	}

	// Path not found without CreatePaths
	fmt.Println("\n3. Path not found without CreatePaths:")
	_, err = json.Set(basicJsonStr, "nonexistent.deeply.nested.path", "value")
	if err != nil {
		fmt.Printf("Expected error for non-existent path: %v\n", err)
	}

	// Array index out of bounds
	fmt.Println("\n4. Array index out of bounds:")
	_, err = json.Set(arrayJsonStr, "users[10].name", "OutOfBounds")
	if err != nil {
		fmt.Printf("Expected error for out of bounds index: %v\n", err)
	}

	// Successful operations that don't error
	fmt.Println("\n5. Operations that succeed gracefully:")

	// Setting same value (idempotent)
	result, err := json.Set(basicJsonStr, "user.name", "John")
	if err != nil {
		fmt.Printf("Unexpected error: %v\n", err)
	} else {
		fmt.Println("Setting same value succeeds (idempotent)")
	}

	// Setting with SetWithAdd on existing path
	result, err = json.SetWithAdd(basicJsonStr, "user.name", "Jane")
	if err != nil {
		fmt.Printf("Unexpected error: %v\n", err)
	} else {
		name, _ := json.GetString(result, "user.name")
		fmt.Printf("SetWithAdd on existing path succeeds, new value: %s\n", name)
	}

	// Setting root level field
	result, err = json.SetWithAdd(basicJsonStr, "newRootField", "root value")
	if err != nil {
		fmt.Printf("Unexpected error: %v\n", err)
	} else {
		fmt.Println("Setting root level field succeeds")
	}

	fmt.Println("\nNote: Proper error handling is essential for robust applications")
}

var line = strings.Repeat("----", 20)

func printLines(title string) {
	fmt.Println(line)
	fmt.Println(title)
}
