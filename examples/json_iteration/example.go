package main

// ==================================================================================
// This is an example of JSON iteration functionality
//
// This example demonstrates comprehensive JSON iteration capabilities including:
// 1. Iterating over JSON objects and arrays using json.Foreach()
// 2. Accessing nested values with path notation (e.g., "address.contacts[-2:]{status}")
// 3. Array slicing operations (e.g., "strings[::2]", "numbers[-3:]")
// 4. Type-safe value retrieval (GetString, GetInt, GetBool, GetFloat64)
// 5. Flow control within iterations (Continue, Break)
// 6. Modifying JSON data during iteration with json.ForeachReturn()
// 7. Setting, adding, and deleting values within nested structures
// 8. Automatic type conversion and generic type handling
// 9. Utility methods for data validation and analysis (Exists, IsNull, IsEmpty, Length, Keys, Values)
// 10. Nested iteration with isolated processor states
//
// This example covers the following scenarios:
// - Basic object iteration with data access and type conversion
// - Array iteration with conditional flow control
// - Object iteration with data modification and result return
// - Array iteration with element manipulation and result return
// - Deep nested iteration with multi-level data transformations
// - Comprehensive utility method usage for data validation and structure analysis
// ==================================================================================

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/cybergodev/json"
)

func main() {

	printLines("=== Iterative JSON Object example ===")
	iterableJsonObject()

	printLines("=== Iterative JSON Array example ===")
	iterableJsonArray()

	printLines("=== Iterative Object and return result example ===")
	iterableObjectAndReturn()

	printLines("=== Iterative Array and return result example ===")
	iterableArrayAndReturn()

	printLines("=== Nested iteration example ===")
	iterationNested()

	printLines("=== Deep nested example ===")
	deepNestedExample()

	printLines("=== Utility Methods Examples ===")
	utilityMethodsExample()
}

// Iterative JSON Object Example
func iterableJsonObject() {
	jsonStr := `{
		"name": "John Doe",
		"age": 30,
		"active": true,
		"score": 95.5,
		"metadata": null,
		"users": [
			{"name": "Alice", "age": 30, "city": "New York"},
			{"name": "Bob", "age": 25, "city": "Los Angeles"},
			{"name": "Carol", "age": 35, "city": "Chicago"},
			{"name": "David", "age": 28, "city": "Houston"},
			{"name": "Eve", "age": 32, "city": "Phoenix"},
			{"name": "Frank", "age": 29, "city": "Philadelphia"},
			{"name": "Grace", "age": 31, "city": "San Antonio"},
			{"name": "Henry", "age": 27, "city": "San Diego"}
		],
		"hobbies": ["reading", "coding", "music"],
		"strings": ["a", "b", "c", "d", "e"],
		"numbers": [1, 2, 3, 4, 5],
		"address": {
			"street": "123 Main St",
			"city": "New York",
			"zipcode": "10001",
			"contacts": [
				{"name": "UserA", "status": "active"},
				{"name": "UserB", "status": "completed"},
				{"name": "UserC", "status": "ignore"},
				{"name": "UserD", "status": "active"},
				{"name": "UserE", "status": "pass"}
			]
		}
	}`

	type Contacts struct {
		Name   string `json:"name"`
		Status string `json:"status"`
	}

	// Demonstrate IterableValue operations in the root context
	_ = json.Foreach(jsonStr, func(key any, item *json.IterableValue) {
		keyStr := key.(string)

		// Only process the root object (when key is a top-level field)
		if keyStr == "name" {

			fmt.Printf("🎯 Current field value: %s\n", item.GetString(""))
			fmt.Println("\n📋 Direct JSON Path Operations (outside iteration):")

			// Type safety with direct JSON operations
			fmt.Printf("Name: %s\n", item.GetString("name"))      // Result: John Doe
			fmt.Printf("Age: %d\n", item.GetInt("age"))           // Result: 30
			fmt.Printf("Active: %v\n", item.GetBool("active"))    // Result: true
			fmt.Printf("Score: %.1f\n", item.GetFloat64("score")) // Result: 95.5

			// Advanced array slicing operations
			fmt.Println("\n🔍 Advanced Array Slicing:")

			// Every 2nd element from strings array
			fmt.Printf("strings[::2]: %v\n", item.Get("strings[::2]"))
			// Result: [a c e]

			// Contacts slice [1:3]
			fmt.Printf("address.contacts[1:3]: %v\n", item.Get("address.contacts[1:3]"))
			// Result: [map[name:UserB status:completed] map[name:UserC status:ignore]]

			// Last contact's name
			fmt.Printf("address.contacts[-1].name: %s\n", item.GetString("address.contacts[-1].name"))
			// Result: UserE

			// Last 3 numbers
			fmt.Printf("numbers[-3:]: %v\n", item.Get("numbers[-3:]"))
			// Result: [3 4 5]

			// Extract status from last 2 contacts
			fmt.Printf("address.contacts[-2:]{status}: %v\n", item.Get("address.contacts[-2:]{status}"))
			// Result: [active pass]

			// Type conversion
			fmt.Printf("address.zipcode (as int): %d\n", item.GetInt("address.zipcode"))
			// Result: 10001

			// Type-safe operations with generics
			contacts, _ := json.GetTyped[[]Contacts](jsonStr, "address.contacts[0:3]")
			fmt.Printf("address.contacts[0:3] (typed): %+v\n", contacts)
			// Result: [{Name:UserA Status:active} {Name:UserB Status:completed} {Name:UserC Status:ignore}]
		}
	})

}

// Iterative JSON Array Example
func iterableJsonArray() {
	jsonStr := `[
		{"id": 1, "name": "Alice", "age": 25, "status": "active"},
		{"id": 2, "name": "Bob", "age": 30, "status": "inactive"},
		{"id": 3, "name": "Carol", "age": 35, "status": "active"},
		{"id": 4, "name": "David", "age": 40, "status": "banned"},
		{"id": 5, "name": "Eve", "age": 28, "status": "active"}
	]`

	_ = json.Foreach(jsonStr, func(key any, item *json.IterableValue) {

		keyInt := key.(int)

		if keyInt == 1 {

			fmt.Println(item.GetInt("id"), item.Get("name")) // Result: 2 Bob

			item.Continue() // Skip this iteration
			fmt.Println("This info not be printed")
		}

		if item.GetString("status") == "banned" {

			fmt.Println(item.GetInt("id"), item.Get("status")) // Result: 4 banned

			item.Break() // End the iteration
			fmt.Println("This info not be printed")
		}

	})
}

// Iterative Object and return result example ===
func iterableObjectAndReturn() {
	jsonStr := `{
		"name": "John Doe",
		"age": 30,
		"active": true,
		"score": 95.5,
		"aaaa": {"aaaa": 99999},
		"deprecated": "doesn't make sense",
		"deprecatedArray": [1,2,3],
		"deprecatedObject": {
			"street": "123 Main St",
			"city": "New York",
			"country": "unknown"
		},
		"metadata": null,
		"strings": ["a", "b", "c", "d", "e"],
		"numbers": [1, 2, 3, 4, 5],
		"address": {
			"street": "123 Main St",
			"city": "New York",
			"zipcode": "10001",
			"contacts": [
				{"name": "UserA", "status": "active"},
				{"name": "UserB", "status": "completed"},
				{"name": "UserC", "status": "ignore"},
				{"name": "UserD", "status": "ignore"},
				{"name": "UserE", "status": "pass"}
			]
		}
	}`

	result, _ := json.ForeachReturn(jsonStr, func(key any, item *json.IterableValue) {
		keyStr := key.(string)

		if keyStr == "name" {
			_ = item.SetWithAdd("backup.temps[2:5]", []string{"reading", "coding", "music"})
			_ = item.SetWithAdd("hobbies", []string{"reading", "coding", "music"})

			_ = item.Set("address.zipcode", 10001)
			_ = item.Set("address.contacts[-2:]{status}", "active")

			_ = item.Delete("address.contacts[0:3]") // Removes elements 0,1,2 from contacts array
		}

		if keyStr == "deprecated" {
			// Use the empty path to delete the current item
			_ = item.Delete("")
		}

		if keyStr == "aaaa" {
			_ = item.Delete("aaaa")
		}

		if keyStr == "deprecatedArray" {
			_ = item.Delete("")
		}

		if keyStr == "deprecatedObject" {
			_ = item.Delete("country")
		}

	})

	fmt.Println(json.FormatPretty(result))
}

// Iterative operation and return result example
func iterableArrayAndReturn() {
	jsonStr := `[
		{"id": 1, "name": "Alice", "age": 25, "status": "active"},
		{"id": 2, "name": "Bob", "age": 30, "status": "inactive"},
		{"id": 3, "name": "Carol", "age": 35, "status": "active"},
		{"id": 4, "name": "David", "age": 40, "status": "banned"},
		{"id": 5, "name": "Eve", "age": 28, "status": "active"}
	]`

	result, _ := json.ForeachReturn(jsonStr, func(key any, item *json.IterableValue) {

		id := item.GetInt("id")
		name := item.GetString("name")

		if id == 1 {
			_ = item.Delete("age")
		}

		if name == "Bob" {
			_ = item.Set("level", "vip")
			_ = item.Set("birthday", time.Now().Format("2006-01-02"))
		}

		if name == "Carol" {
			_ = item.SetWithAdd("other.score", []int{11, 22, 33})
			_ = item.SetWithAdd("other.hobby", []string{"reading", "coding", "music"})
		}

		if item.GetString("status") == "banned" {
			_ = item.Delete("")
		}
	})

	fmt.Println(json.FormatPretty(result))
}

// Nested iteration example
// When complex operations need to be performed on JSON
func iterationNested() {
	complexData := `{
		"company": {
			"departments": [
				{
					"name": "Engineering",
					"employees": [
						{"name": "Alice", "salary": 90000, "level": "senior"},
						{"name": "Bob", "salary": 75000, "level": "junior"}
					]
				},
				{
					"name": "Marketing",
					"employees": [
						{"name": "Carol", "salary": 80000, "level": "senior"},
						{"name": "Dave", "salary": 65000, "level": "junior"}
					]
				}
			]
		}
	}`

	var marketingEmployees []string
	var totalSalaryByDept = make(map[string]int)

	editJson, _ := json.ForeachReturn(complexData, func(key any, item *json.IterableValue) {
		if key == "company" {
			// Use the ForeachReturnNested() method for nested iteration modification
			err := item.ForeachReturnNested("departments", func(deptKey any, deptItem *json.IterableValue) {
				deptName := deptItem.GetString("name")
				fmt.Printf("Processing department: %s\n", deptName)

				// Add department metadata
				deptItem.Set("status", "active")
				deptItem.Set("id", fmt.Sprintf("dept-%v", deptKey))

				// Process employees with nested modifications
				var deptTotalSalary int
				err := deptItem.ForeachReturnNested("employees", func(empKey any, empItem *json.IterableValue) {
					empName := empItem.GetString("name")
					empSalary := empItem.GetInt("salary")
					empLevel := empItem.GetString("level")

					fmt.Printf("Processing employee: %s\n", empName)

					// Add employee metadata
					empItem.Set("id", fmt.Sprintf("emp-%s-%v", deptName, empKey))
					empItem.Set("department", deptName)
					empItem.Set("status", "active")

					// Apply salary adjustments based on level
					var newSalary int
					if empLevel == "senior" {
						newSalary = empSalary + 5000 // Senior bonus
						empItem.Set("bonus", 5000)
					} else {
						newSalary = empSalary + 2000 // Junior bonus
						empItem.Set("bonus", 2000)
					}
					empItem.Set("salary", newSalary)
					empItem.Set("adjusted", true)

					deptTotalSalary += newSalary

					// Collect marketing employees
					if deptName == "Marketing" {
						marketingEmployees = append(marketingEmployees, empName)
					}
				})

				if err != nil {
					fmt.Printf("Error processing employees: %v\n", err)
				}

				// Add department summary
				deptItem.Set("total_salary", deptTotalSalary)
				deptItem.Set("employee_count", len(deptItem.GetArray("employees")))
				totalSalaryByDept[deptName] = deptTotalSalary
			})

			if err != nil {
				fmt.Printf("Error processing departments: %v\n", err)
			}
		}
	})

	// Display results
	fmt.Println("\nResults nested modifications:")
	prettyJson, _ := json.FormatPretty(editJson)
	fmt.Println(prettyJson)

	fmt.Printf("\nMarketing employees: %v\n", marketingEmployees)
	fmt.Println("\nTotal salary by department:")
	for dept, total := range totalSalaryByDept {
		fmt.Printf("  %s: $%d\n", dept, total)
	}
}

// Deep nested example
func deepNestedExample() {
	complexJson := `{
		"company": {
			"departments": [
				{
					"name": "Engineering",
					"teams": [
						{
							"name": "Backend",
							"members": [
								{"name": "Alice", "level": "Senior", "salary": 90000}
							]
						}
					]
				}
			]
		}
	}`

	result, err := json.ForeachReturn(complexJson, func(key any, item *json.IterableValue) {
		if key == "company" {
			// First-level nesting: Processing Department
			err := item.ForeachReturnNested("departments", func(deptKey any, deptItem *json.IterableValue) {
				deptName := deptItem.GetString("name")
				fmt.Printf("Processing department: %s\n", deptName)

				// Add department metadata
				deptItem.Set("id", fmt.Sprintf("dept-%v", deptKey))
				deptItem.Set("budget", 1000000)

				// Second level nesting: Project team
				err := deptItem.ForeachReturnNested("teams", func(teamKey any, teamItem *json.IterableValue) {
					teamName := teamItem.GetString("name")
					fmt.Printf("Processing team: %s\n", teamName)

					// Add team metadata
					teamItem.Set("id", fmt.Sprintf("team-%v", teamKey))
					teamItem.Set("size", len(teamItem.GetArray("members")))

					// Third-level nesting: Handling members
					err := teamItem.ForeachReturnNested("members", func(memberKey any, memberItem *json.IterableValue) {
						memberName := memberItem.GetString("name")
						fmt.Printf("Processing member: %s\n", memberName)

						// Add member metadata
						memberItem.Set("id", fmt.Sprintf("member-%v", memberKey))
						memberItem.Set("department", deptName)
						memberItem.Set("team", teamName)

						// Adjust salary according to the level
						level := memberItem.GetString("level")
						salary := memberItem.GetInt("salary")
						if level == "Senior" {
							newSalary := salary + 5000
							memberItem.Set("salary", newSalary)
							memberItem.Set("bonus", 5000)
							fmt.Printf("Applied senior bonus: $%d -> $%d\n", salary, newSalary)
						}
					})

					if err != nil {
						log.Printf("Member processing error: %v", err)
					}
				})

				if err != nil {
					log.Printf("Team processing error: %v", err)
				}
			})

			if err != nil {
				log.Printf("Department processing error: %v", err)
			}
		}
	})

	if err != nil {
		log.Fatalf("Deep nested ForeachReturn failed: %v", err)
	}

	prettyResult, _ := json.FormatPretty(result)
	fmt.Printf("\nFinal result:\n%s\n", prettyResult)
}

// Utility methods example - demonstrates Exists, IsNull, IsEmpty, Length, Keys, Values
func utilityMethodsExample() {
	jsonStr := `{
		"user": {
			"name": "John Doe",
			"age": 30,
			"email": "john@example.com",
			"profile": {
				"bio": "",
				"skills": [],
				"preferences": {
					"theme": "dark",
					"language": "en",
					"notifications": true
				},
				"metadata": null,
				"tags": ["developer", "golang", "json"]
			},
			"projects": [
				{"name": "Project A", "status": "active", "priority": 1},
				{"name": "Project B", "status": "completed", "priority": 2},
				{"name": "Project C", "status": "pending", "priority": 3}
			],
			"settings": {},
			"temp_data": null,
			"empty_array": [],
			"empty_string": "",
			"zero_number": 0,
			"false_boolean": false
		},
		"company": {
			"departments": [
				{
					"name": "Engineering",
					"employees": [
						{"name": "Alice", "role": "Senior Developer"},
						{"name": "Bob", "role": "Junior Developer"}
					],
					"budget": 1000000,
					"active": true
				},
				{
					"name": "Marketing",
					"employees": [],
					"budget": 500000,
					"active": false
				}
			],
			"metadata": {
				"founded": "2020",
				"location": "San Francisco",
				"size": "medium"
			}
		}
	}`

	fmt.Println("=== Demonstrating Utility Methods ===")

	_ = json.Foreach(jsonStr, func(key any, item *json.IterableValue) {
		keyStr, ok := key.(string)
		if !ok {
			return
		}

		if keyStr == "user" {
			fmt.Println("\n--- Testing on 'user' object ---")

			// Exists() method - check if paths exist
			fmt.Println("1. Exists() method:")
			fmt.Printf("   user.name exists: %t\n", item.Exists("name"))
			fmt.Printf("   user.email exists: %t\n", item.Exists("email"))
			fmt.Printf("   user.phone exists: %t\n", item.Exists("phone"))
			fmt.Printf("   user.profile.bio exists: %t\n", item.Exists("profile.bio"))
			fmt.Printf("   user.profile.nonexistent exists: %t\n", item.Exists("profile.nonexistent"))
			fmt.Printf("   user.projects[0].name exists: %t\n", item.Exists("projects[0].name"))
			fmt.Printf("   user.projects[10].name exists: %t\n", item.Exists("projects[10].name"))

			// IsNull() method - check for null values
			fmt.Println("\n2. IsNull() method:")
			fmt.Printf("   user.name is null: %t\n", item.IsNull("name"))
			fmt.Printf("   user.profile.metadata is null: %t\n", item.IsNull("profile.metadata"))
			fmt.Printf("   user.temp_data is null: %t\n", item.IsNull("temp_data"))
			fmt.Printf("   user.nonexistent is null: %t\n", item.IsNull("nonexistent"))

			// IsEmpty() method - check for empty values
			fmt.Println("\n3. IsEmpty() method:")
			fmt.Printf("   user.profile.bio is empty: %t\n", item.IsEmpty("profile.bio"))
			fmt.Printf("   user.profile.skills is empty: %t\n", item.IsEmpty("profile.skills"))
			fmt.Printf("   user.settings is empty: %t\n", item.IsEmpty("settings"))
			fmt.Printf("   user.empty_array is empty: %t\n", item.IsEmpty("empty_array"))
			fmt.Printf("   user.empty_string is empty: %t\n", item.IsEmpty("empty_string"))
			fmt.Printf("   user.name is empty: %t\n", item.IsEmpty("name"))
			fmt.Printf("   user.zero_number is empty: %t\n", item.IsEmpty("zero_number"))
			fmt.Printf("   user.false_boolean is empty: %t\n", item.IsEmpty("false_boolean"))

			// Length() method - get length of arrays, objects, strings
			fmt.Println("\n4. Length() method:")
			fmt.Printf("   user.name length: %d\n", item.Length("name"))
			fmt.Printf("   user.profile.skills length: %d\n", item.Length("profile.skills"))
			fmt.Printf("   user.profile.tags length: %d\n", item.Length("profile.tags"))
			fmt.Printf("   user.projects length: %d\n", item.Length("projects"))
			fmt.Printf("   user.profile.preferences length: %d\n", item.Length("profile.preferences"))
			fmt.Printf("   user.settings length: %d\n", item.Length("settings"))
			fmt.Printf("   user.nonexistent length: %d\n", item.Length("nonexistent"))

			// Keys() method - get object keys
			fmt.Println("\n5. Keys() method:")
			fmt.Printf("   user keys: %v\n", item.Keys(""))
			fmt.Printf("   user.profile keys: %v\n", item.Keys("profile"))
			fmt.Printf("   user.profile.preferences keys: %v\n", item.Keys("profile.preferences"))
			fmt.Printf("   user.projects[0] keys: %v\n", item.Keys("projects[0]"))
			fmt.Printf("   user.settings keys: %v\n", item.Keys("settings"))
			fmt.Printf("   user.profile.skills keys (array): %v\n", item.Keys("profile.skills"))

			// Values() method - get object/array values
			fmt.Println("\n6. Values() method:")
			profileValues := item.Values("profile.preferences")
			fmt.Printf("   user.profile.preferences values: %v\n", profileValues)

			tagsValues := item.Values("profile.tags")
			fmt.Printf("   user.profile.tags values: %v\n", tagsValues)

			projectsValues := item.Values("projects")
			fmt.Printf("   user.projects values count: %d\n", len(projectsValues))

			emptyValues := item.Values("settings")
			fmt.Printf("   user.settings values: %v\n", emptyValues)
		}

		if keyStr == "company" {
			fmt.Println("\n--- Testing on 'company' object ---")

			// Complex path testing
			fmt.Println("7. Complex path testing:")
			fmt.Printf("   company.departments exists: %t\n", item.Exists("departments"))
			fmt.Printf("   company.departments[0].name exists: %t\n", item.Exists("departments[0].name"))
			fmt.Printf("   company.departments[0].employees length: %d\n", item.Length("departments[0].employees"))
			fmt.Printf("   company.departments[1].employees length: %d\n", item.Length("departments[1].employees"))
			fmt.Printf("   company.departments[0].employees is empty: %t\n", item.IsEmpty("departments[0].employees"))
			fmt.Printf("   company.departments[1].employees is empty: %t\n", item.IsEmpty("departments[1].employees"))

			// Get keys of nested objects
			fmt.Printf("   company.departments[0] keys: %v\n", item.Keys("departments[0]"))
			fmt.Printf("   company.metadata keys: %v\n", item.Keys("metadata"))

			// Array operations
			deptValues := item.Values("departments")
			fmt.Printf("   company.departments values count: %d\n", len(deptValues))
		}
	})

	fmt.Println("\n=== Advanced Utility Methods Usage ===")

	// Demonstrate utility methods in practical scenarios
	_ = json.Foreach(jsonStr, func(key any, item *json.IterableValue) {
		keyStr, ok := key.(string)
		if !ok {
			return
		}

		if keyStr == "user" {
			fmt.Println("\n--- Practical Usage Scenarios ---")

			// Scenario 1: Validation and conditional processing
			fmt.Println("8. Validation and conditional processing:")
			if item.Exists("email") && !item.IsEmpty("email") {
				fmt.Printf("   ✓ User has valid email: %s\n", item.GetString("email"))
			}

			if item.Exists("profile.bio") && item.IsEmpty("profile.bio") {
				fmt.Println("   ⚠ User bio is empty - might need to prompt for completion")
			}

			if item.IsNull("temp_data") {
				fmt.Println("   ℹ temp_data is null - safe to ignore")
			}

			// Scenario 2: Dynamic processing based on content
			fmt.Println("\n9. Dynamic processing based on content:")
			skillsLength := item.Length("profile.skills")
			if skillsLength == 0 {
				fmt.Println("   📝 User has no skills listed - recommend adding some")
			} else {
				fmt.Printf("   👍 User has %d skills listed\n", skillsLength)
			}

			projectsLength := item.Length("projects")
			if projectsLength > 0 {
				fmt.Printf("   🚀 User is working on %d projects\n", projectsLength)

				// Check each project
				for i := 0; i < projectsLength; i++ {
					projectPath := fmt.Sprintf("projects[%d]", i)
					if item.Exists(projectPath + ".status") {
						status := item.GetString(projectPath + ".status")
						name := item.GetString(projectPath + ".name")
						fmt.Printf("      - %s: %s\n", name, status)
					}
				}
			}

			// Scenario 3: Object structure analysis
			fmt.Println("\n10. Object structure analysis:")
			userKeys := item.Keys("")
			fmt.Printf("   User object has %d top-level fields: %v\n", len(userKeys), userKeys)

			if item.Exists("profile") {
				profileKeys := item.Keys("profile")
				fmt.Printf("   Profile has %d fields: %v\n", len(profileKeys), profileKeys)

				// Analyze each profile field
				for _, profileKey := range profileKeys {
					profilePath := "profile." + profileKey
					if item.IsNull(profilePath) {
						fmt.Printf("      - %s: null\n", profileKey)
					} else if item.IsEmpty(profilePath) {
						fmt.Printf("      - %s: empty\n", profileKey)
					} else {
						length := item.Length(profilePath)
						if length > 0 {
							fmt.Printf("      - %s: has %d items/chars\n", profileKey, length)
						} else {
							fmt.Printf("      - %s: has value\n", profileKey)
						}
					}
				}
			}
		}
	})
}

var line = strings.Repeat("----", 20)

func printLines(title string) {
	fmt.Println(line)
	fmt.Println(title)
}
