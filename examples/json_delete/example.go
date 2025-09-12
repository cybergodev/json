package main

// ==================================================================================
// This is a comprehensive example of JSON deletion functionality
//
// This example demonstrates different approaches to deleting JSON data:
// 1. Standard deletion using json.Delete() - removes target values but retains null placeholders
// 2. Clean deletion using json.DeleteWithCleanNull() - removes target values and cleans up null values
// 3. Custom deletion with ProcessorOptions - fine-grained control over deletion behavior
// 4. Array element deletion - deleting specific array elements by index
// 5. Array range deletion - deleting ranges of array elements
// 6. Conditional deletion using ForeachReturn - deleting based on conditions
// 7. Multiple path deletion - deleting multiple paths in one operation
//
// The example uses nested JSON structure with arrays and objects to show how deletion
// works with complex path expressions like "a{g}{name}" which targets all "name" fields
// within nested "g" arrays inside the "a" array.
//
// Key features demonstrated:
// - Bulk deletion using path expressions with wildcards ({})
// - Difference between standard and clean deletion behaviors
// - Handling of null values in arrays after deletion operations
// - Array index and range deletion
// - Conditional deletion based on value matching
// - Custom deletion options for fine-grained control
// ==================================================================================

import (
	"fmt"
	"log"
	"strings"

	"github.com/cybergodev/json"
)

// Sample JSON data for basic deletion examples
const jsonStr = `{"a":[
		{"g":[{"name":"11"},null,{"name":"22"},null]},
		{"g":[{"name":"aa"},null,{"name":"bb"},null,{"name":"cc"},null]}],
		"init":null
	}`

// Sample JSON data for array deletion examples
const arrayJsonStr = `{
	"users": [
		{"id": 1, "name": "Alice", "age": 25, "active": true},
		{"id": 2, "name": "Bob", "age": 30, "active": false},
		{"id": 3, "name": "Charlie", "age": 35, "active": true},
		{"id": 4, "name": "David", "age": 28, "active": false},
		{"id": 5, "name": "Eve", "age": 32, "active": true}
	],
	"metadata": {
		"total": 5,
		"version": "1.0",
		"temp_data": null
	}
}`

// Sample JSON data for complex nested deletion examples
const complexJsonStr = `{
	"company": {
		"departments": [
			{
				"name": "Engineering",
				"employees": [
					{"name": "John", "salary": 80000, "temp": true},
					{"name": "Jane", "salary": 85000, "temp": false},
					{"name": "Jim", "salary": 75000, "temp": true}
				]
			},
			{
				"name": "Marketing",
				"employees": [
					{"name": "Mary", "salary": 70000, "temp": false},
					{"name": "Mike", "salary": 72000, "temp": true}
				]
			}
		],
		"config": {
			"debug": true,
			"cache": null,
			"temp_settings": {"timeout": 30}
		}
	}
}`

func main() {
	// Basic deletion examples
	printLines("=== Basic Deletion Examples ===")
	deletionDefault()
	deleteWithCleanNull()

	// Custom options deletion
	printLines("=== Custom Options Deletion ===")
	deleteWithCustomOptions()

	// Array deletion examples
	printLines("=== Array Deletion Examples ===")
	deleteArrayElements()
	deleteArrayRanges()

	// Conditional deletion examples
	printLines("=== Conditional Deletion Examples ===")
	deleteConditionally()

	// Complex nested deletion examples
	printLines("=== Complex Nested Deletion Examples ===")
	deleteNestedElements()

	// Multiple operations example
	printLines("=== Multiple Operations Example ===")
	multipleDeletionOperations()
}

// Default delete while retaining null placeholders
func deletionDefault() {

	fmt.Println("\nDeleting 'a{g}{name}' with standard Delete():")

	result, err := json.Delete(jsonStr, "a{g}{name}")
	if err != nil {
		log.Println("Error:", err)
	} else {
		fmt.Println("Note: Empty objects remain, null values are preserved")
		fmt.Println("Result:", result)
		// Result: {"a":[{"g":[{},null,{},null]},{"g":[{},null,{},null,{},null]}]},"init":null}
	}
}

// Delete the data and clean up the null values
func deleteWithCleanNull() {

	fmt.Println("\nDeleting 'a{g}{name}' with DeleteWithCleanNull():")

	result, err := json.DeleteWithCleanNull(jsonStr, "a{g}{name}")
	if err != nil {
		log.Println("Error:", err)
	} else {
		fmt.Println("Note: Null values are cleaned up, arrays are compacted")
		fmt.Println("Result:", result)
		// Result: {"a":[{"g":[{},{}]},{"g":[{},{},{}]}]}
	}
}

// Delete with custom ProcessorOptions for fine-grained control
func deleteWithCustomOptions() {

	fmt.Println("\nDeleting 'init' field with custom options:")

	// Create custom options
	opts := &json.ProcessorOptions{
		CleanupNulls:  true,
		CompactArrays: false, // Keep array structure but clean nulls
	}

	result, err := json.Delete(jsonStr, "init", opts)
	if err != nil {
		log.Println("Error:", err)
	} else {
		fmt.Println("Note: Only the 'init' field is removed, arrays keep their structure")
		fmt.Println("Result:", result)
		// Result: {"a":[{"g":[{"name":"11"},null,{"name":"22"},null]},{"g":[{"name":"aa"},null,{"name":"bb"},null,{"name":"cc"},null]}]}
	}

	// Another example with different options
	fmt.Println("\nDeleting 'a{g}{name}' with CleanupNulls=false, CompactArrays=true:")
	opts2 := &json.ProcessorOptions{
		CleanupNulls:  false,
		CompactArrays: true,
	}

	result2, err := json.Delete(jsonStr, "a{g}{name}", opts2)
	if err != nil {
		log.Println("Error:", err)
	} else {
		fmt.Println("Note: Arrays are compacted but other nulls remain")
		fmt.Println("Result:", result2)
		// Result: {"a":[{"g":[{},{}]},{"g":[{},{},{}]}]}
	}
}

// Delete specific array elements by index
func deleteArrayElements() {

	// Delete the second user (index 1)
	fmt.Println("\nDeleting user at index 1 (Bob):")
	result, err := json.Delete(arrayJsonStr, "users[1]")
	if err != nil {
		log.Println("Error:", err)
	} else {
		fmt.Println("Note: Bob is removed, array indices are automatically adjusted")
		fmt.Println("Result:\n", result)
		// Result: {"users":[{"id":1,"name":"Alice","age":25,"active":true,"salary":75000.5},{"id":3,"name":"Charlie","age":35,"active":true,"salary":90000},{"id":4,"name":"David","age":28,"active":false,"salary":85000.25},{"id":5,"name":"Eve","age":
	}

	// Delete the last user (negative index)
	fmt.Println("\nDeleting the last user using negative index [-1]:")
	result2, err := json.Delete(arrayJsonStr, "users[-1]")
	if err != nil {
		log.Println("Error:", err)
	} else {
		fmt.Println("Note: Last user (Eve) is removed using negative indexing")
		fmt.Println("Result:\n", result2)
		// Result: {"users":[{"id":1,"name":"Alice","age":25,"active":true,"salary":75000.5},{"id":2,"name":"Bob","age":30,"active":false,"salary":80000.75},{"id":3,"name":"Charlie","age":35,"active":true,"salary":90000},{"id":4,"name":"David","age":2
	}
}

// Delete ranges of array elements
func deleteArrayRanges() {

	// Delete users from index 1 to 3 (Bob, Charlie, David)
	fmt.Println("\nDeleting users[1:4] (Bob, Charlie, David):")
	result, err := json.DeleteWithCleanNull(arrayJsonStr, "users[1:4]")
	if err != nil {
		log.Println("Error:", err)
	} else {
		fmt.Println("Note: Range deletion removes multiple elements at once")
		fmt.Println("Result:\n", result)
		// Result: {"metadata":{"total":5,"version":"1.0"},"users":[{"active":true,"age":25,"id":1,"name":"Alice"},{"active":true,"age":32,"id":5,"name":"Eve"}]}
	}

	// Delete from index 2 to end
	fmt.Println("\nDeleting users[2:] (from Charlie to end):")
	result2, err := json.DeleteWithCleanNull(arrayJsonStr, "users[2:]")
	if err != nil {
		log.Println("Error:", err)
	} else {
		fmt.Println("Note: Open-ended range deletes from index to end of array")
		fmt.Println("Result:\n", result2)
		// Result: {"metadata":{"total":5,"version":"1.0"},"users":[{"active":true,"age":25,"id":1,"name":"Alice"},{"active":false,"age":30,"id":2,"name":"Bob"}]}
	}
}

// Delete elements conditionally using ForeachReturn
func deleteConditionally() {

	// Demonstrate bulk deletion using path expressions first (simpler approach)
	fmt.Println("\nBulk deletion using path expressions:")
	fmt.Println("Removing 'id' field from all users:")
	result1, err := json.DeleteWithCleanNull(arrayJsonStr, "users{id}")
	if err != nil {
		log.Println("Error:", err)
	} else {
		fmt.Println("Note: All 'id' fields removed from users array using path expression")
		fmt.Println("Result:\n", result1)
		// Result: {"metadata":{"total":5,"version":"1.0"},"users":[{"active":true,"age":25,"name":"Alice"},{"active":false,"age":30,"name":"Bob"},{"active":true,"age":35,"name":"Charlie"},{"active":false,"age":28,"name":"David"},{"active":true,"age":32,"name":"Eve"}]}
	}

	// Remove multiple fields at once
	fmt.Println("\nRemoving multiple fields using separate operations:")
	fmt.Println("First remove 'age' field from all users:")
	result2, err := json.DeleteWithCleanNull(arrayJsonStr, "users{age}")
	if err != nil {
		log.Println("Error:", err)
	} else {
		fmt.Println("Then remove 'id' field from all users:")
		result3, err := json.DeleteWithCleanNull(result2, "users{id}")
		if err != nil {
			log.Println("Error:", err)
		} else {
			fmt.Println("Final result:\n", result3)
			fmt.Println("Note: Multiple field deletions can be chained")
			// Result: {"metadata":{"total":5,"version":"1.0"},"users":[{"active":true,"name":"Alice"},{"active":false,"name":"Bob"},{"active":true,"name":"Charlie"},{"active":false,"name":"David"},{"active":true,"name":"Eve"}]}
		}
	}

	// Demonstrate conditional deletion using ForeachReturn for array elements
	fmt.Println("\nConditional deletion using ForeachReturn:")
	fmt.Println("Deleting users with specific conditions:")

	// Create a working copy for conditional deletion
	workingJson := arrayJsonStr

	// Delete inactive users by iterating through the users array
	result4, err := json.ForeachReturn(workingJson, func(key any, item *json.IterableValue) {
		// We're looking for array elements in the users array
		if keyStr, ok := key.(string); ok && keyStr == "users" {
			// This is the users array, but we need to work with individual elements
			return
		}

		// Check if this is a user object (has required fields)
		if item.GetString("name") != "" && item.GetInt("id") > 0 {
			// This looks like a user object
			if item.GetBool("active") == false {
				// Delete inactive user
				err := item.Delete("")
				if err != nil {
					log.Printf("Failed to delete user: %v", err)
				} else {
					fmt.Printf("Deleted inactive user: %s\n", item.GetString("name"))
				}
			}
		}
	})

	if err != nil {
		log.Println("Error:", err)
	} else {
		fmt.Println("Result after conditional deletion:\n", result4)
		fmt.Println("Note: Inactive users have been removed")
	}

	// Show a simpler approach for filtering
	fmt.Println("\nSimpler approach - Delete specific array elements by index:")
	fmt.Println("Delete users at indices 1 and 3 (Bob and David - the inactive ones):")

	// Delete in reverse order to maintain indices
	tempResult, err := json.Delete(arrayJsonStr, "users[3]") // Delete David first
	if err != nil {
		log.Println("Error deleting user 3:", err)
	} else {
		finalResult, err := json.Delete(tempResult, "users[1]") // Then delete Bob
		if err != nil {
			log.Println("Error deleting user 1:", err)
		} else {
			fmt.Println("Result:\n", finalResult)
			fmt.Println("Note: Specific inactive users removed by index")
		}
	}
}

// Delete elements from complex nested structures
func deleteNestedElements() {

	// Delete all temporary employees
	fmt.Println("\nDeleting all temporary employees (temp: true):")
	result, err := json.ForeachReturn(complexJsonStr, func(key any, item *json.IterableValue) {
		if key == "employees" {
			// This is an employees array, check each employee
			_, err := json.ForeachReturn(item.GetString(""), func(empKey any, emp *json.IterableValue) {
				if emp.GetBool("temp") == true {
					err := emp.Delete("")
					if err != nil {
						log.Printf("Failed to delete employee: %v", err)
					} else {
						fmt.Printf("Deleted temp employee: %s\n", emp.GetString("name"))
					}
				}
			})
			if err != nil {
				log.Printf("Error processing employees: %v", err)
			}
		}
	})

	if err != nil {
		log.Println("Error:", err)
	} else {
		fmt.Println("Note: All temporary employees are removed")
		fmt.Println("Result:\n", result)
	}

	// Delete specific nested paths
	fmt.Println("\nDeleting nested paths using path expressions:")

	// Delete all temp fields from employees
	result2, err := json.DeleteWithCleanNull(complexJsonStr, "company.departments{employees}{temp}")
	if err != nil {
		log.Println("Error:", err)
	} else {
		fmt.Println("After deleting all 'temp' fields from employees:\n", result2)
	}

	// Delete debug config and temp_settings
	result3, err := json.DeleteWithCleanNull(result2, "company.config.debug")
	if err != nil {
		log.Println("Error:", err)
	} else {
		result3, err = json.DeleteWithCleanNull(result3, "company.config.temp_settings")
		if err != nil {
			log.Println("Error:", err)
		} else {
			fmt.Println("\nNote: Multiple nested deletions clean up the structure")
			fmt.Println("After deleting debug and temp_settings:\n", result3)
		}
	}
}

// Demonstrate multiple deletion operations in sequence
func multipleDeletionOperations() {
	// Step 1: Remove inactive users
	fmt.Println("\nStep 1: Remove inactive users")
	step1Result, err := json.ForeachReturn(arrayJsonStr, func(key any, item *json.IterableValue) {
		// Check if this is the "users" key
		if keyStr, ok := key.(string); ok && keyStr == "users" {
			// Use ForeachReturnNested to iterate through the users array
			err := item.ForeachReturnNested("", func(userKey any, user *json.IterableValue) {
				// Check if this user is inactive
				if user.GetBool("active") == false {
					fmt.Printf("Deleting inactive user: %s (id: %d)\n", user.GetString("name"), user.GetInt("id"))
					err := user.Delete("")
					if err != nil {
						log.Printf("Failed to delete user: %v", err)
					}
				}
			})
			if err != nil {
				log.Printf("Error processing users array: %v", err)
			}
		}
	})
	if err != nil {
		log.Println("Error in step 1:", err)
		return
	}

	fmt.Println("Result after removing inactive users:")
	fmt.Println(step1Result)

	// Step 2: Remove age field from remaining users
	fmt.Println("\nStep 2: Remove age field from remaining users")
	step2Result, err := json.Delete(step1Result, "users{age}")
	if err != nil {
		log.Println("Error in step 2:", err)
		return
	}

	fmt.Println("Result after removing age fields:")
	fmt.Println(step2Result)

	// Step 3: Remove metadata.temp_data
	fmt.Println("\nStep 3: Remove metadata.temp_data")
	step3Result, err := json.DeleteWithCleanNull(step2Result, "metadata.temp_data")
	if err != nil {
		log.Println("Error in step 3:", err)
		return
	}

	fmt.Println("Result after removing temp_data:")
	fmt.Println(step3Result)

	// Step 4: Add a summary field
	fmt.Println("\nStep 4: Add summary information")
	finalResult, err := json.SetWithAdd(step3Result, "summary.active_users", 3)
	if err != nil {
		log.Println("Error in step 4:", err)
		return
	}

	finalResult, err = json.SetWithAdd(finalResult, "summary.last_updated", "2024-01-01")
	if err != nil {
		log.Println("Error adding last_updated:", err)
		return
	}

	fmt.Println("\nFinal result after all operations:")
	fmt.Println(finalResult)
	fmt.Println("Note: Multiple operations can be chained to transform JSON data")

	// Demonstrate error handling
	fmt.Println("\nDemonstrating error handling:")
	_, err = json.Delete(arrayJsonStr, "invalid..path")
	if err != nil {
		fmt.Printf("Expected error for invalid path: %v\n", err)
	}

	// Demonstrate deletion of non-existent path (should not error)
	_, err = json.Delete(arrayJsonStr, "nonexistent.field")
	if err != nil {
		fmt.Printf("Unexpected error: %v\n", err)
	} else {
		fmt.Println("Deleting non-existent path succeeds (no-op)")
	}
}

var line = strings.Repeat("----", 20)

func printLines(title string) {
	fmt.Println(line)
	fmt.Println(title)
}
