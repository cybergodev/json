package main

import (
	"fmt"

	"github.com/cybergodev/json"
)

func main() {
	fmt.Println("🚀 JSON Library Basic Usage Examples")
	fmt.Println("====================================================")

	// Basic example data
	basicData := `{
		"company": "TechCorp",
		"users": [
			{"name": "Alice", "age": 25, "skills": ["Go", "Python"]},
			{"name": "Bob", "age": 30, "skills": ["Java", "React"]},
			{"name": "Charlie", "age": 35, "skills": ["C++", "Rust"]}
		],
		"config": {
			"debug": true,
			"version": "1.0.0"
		}
	}`

	// 1. Basic Get operations
	fmt.Println("\n1. Basic Get Operations:")
	basicGetOperations(basicData)

	// 2. Type-safe operations
	fmt.Println("\n2. Type-Safe Operations:")
	typeSafeOperations(basicData)

	// 3. Array operations
	fmt.Println("\n3. Array Operations:")
	arrayOperations(basicData)

	// 4. Extraction operations
	fmt.Println("\n4. Extraction Operations:")
	extractionOperations(basicData)

	// 5. Basic Set operations
	fmt.Println("\n5. Basic Set Operations:")
	basicSetOperations(basicData)

	// 6. Basic Delete operations
	fmt.Println("\n6. Basic Delete Operations:")
	basicDeleteOperations(basicData)

	// 7. Batch operations
	fmt.Println("\n7. Batch Operations:")
	batchOperations(basicData)

	fmt.Println("\n🎉 Basic examples demonstration completed!")
	fmt.Println("💡 Tip: Check other examples directories for more advanced features")
}

func basicGetOperations(data string) {
	// Simple field access
	company, _ := json.Get(data, "company")
	fmt.Printf("   Company name: %v\n", company)
	// Result: TechCorp

	// Nested field access
	version, _ := json.Get(data, "config.version")
	fmt.Printf("   Version: %v\n", version)
	// Result: 1.0.0

	// Array element access
	firstUser, _ := json.Get(data, "users[0]")
	fmt.Printf("   First user: %v\n", firstUser)
	// Result: map[name:Alice age:25 skills:[Go Python]]
}

func typeSafeOperations(data string) {
	// Type-safe get operations
	company, _ := json.GetString(data, "company")
	fmt.Printf("   Company name (string): %s\n", company)
	// Result: TechCorp

	age, _ := json.GetInt(data, "users[0].age")
	fmt.Printf("   Alice's age (int): %d\n", age)
	// Result: 25

	debug, _ := json.GetBool(data, "config.debug")
	fmt.Printf("   Debug mode (bool): %t\n", debug)
	// Result: true

	skills, _ := json.GetArray(data, "users[0].skills")
	fmt.Printf("   Alice's skills (array): %v\n", skills)
	// Result: [Go Python]
}

func arrayOperations(data string) {
	// Array index operations
	firstName, _ := json.GetString(data, "users[0].name")
	fmt.Printf("   First user: %s\n", firstName)
	// Result: Alice

	lastName, _ := json.GetString(data, "users[-1].name")
	fmt.Printf("   Last user: %s\n", lastName)
	// Result: Charlie

	// Array slice operations - get usernames more clearly
	firstTwoNames, _ := json.Get(data, "users[0:2]{name}")
	fmt.Printf("   First two user names: %v\n", firstTwoNames)
	// Result: [Alice Bob]

	lastTwoNames, _ := json.Get(data, "users[-2:]{name}")
	fmt.Printf("   Last two user names: %v\n", lastTwoNames)
	// Result: [Bob Charlie]
}

func extractionOperations(data string) {
	// Extract all usernames
	allNames, _ := json.Get(data, "users{name}")
	fmt.Printf("   All user names: %v\n", allNames)
	// Result: [Alice Bob Charlie]

	// Extract all ages
	allAges, _ := json.Get(data, "users{age}")
	fmt.Printf("   All ages: %v\n", allAges)
	// Result: [25 30 35]

	// Flat extraction of all skills
	allSkills, _ := json.Get(data, "users{flat:skills}")
	fmt.Printf("   All skills (flattened): %v\n", allSkills)
	// Result: [Go Python Java React C++ Rust]
}

func basicSetOperations(data string) {
	// Set simple field
	updated, _ := json.Set(data, "company", "NewTechCorp")
	newCompany, _ := json.GetString(updated, "company")
	fmt.Printf("   Updated company name: %s\n", newCompany)
	// Result: NewTechCorp

	// Set nested field
	updated2, _ := json.Set(data, "config.debug", false)
	newDebug, _ := json.GetBool(updated2, "config.debug")
	fmt.Printf("   Updated debug mode: %t\n", newDebug)
	// Result: false

	// Set array element
	updated3, _ := json.Set(data, "users[0].age", 26)
	newAge, _ := json.GetInt(updated3, "users[0].age")
	fmt.Printf("   Updated Alice's age: %d\n", newAge)
	// Result: 26
}

func basicDeleteOperations(data string) {
	// Delete simple field
	updated, _ := json.Delete(data, "config.debug")
	debugValue, _ := json.Get(updated, "config.debug")
	exists := debugValue != nil
	fmt.Printf("   Debug field exists after deletion: %t\n", exists)
	// Result: false

	// Delete array element
	updated2, _ := json.Delete(data, "users[0]")
	remainingUsers, _ := json.Get(updated2, "users{name}")
	fmt.Printf("   Remaining users after deleting first: %v\n", remainingUsers)
	// Result: [Bob Charlie]
}

func batchOperations(data string) {
	// Batch get operations
	paths := []string{"company", "config.version", "users[0].name"}
	results, _ := json.GetMultiple(data, paths)
	fmt.Printf("   Batch get results: %v\n", results)
	// Result: map[company:TechCorp config.version:1.0.0 users[0].name:Alice]

	// Batch set operations
	updates := map[string]any{
		"company":        "SuperTechCorp",
		"config.version": "2.0.0",
		"users[0].age":   27,
	}
	updated, _ := json.SetMultiple(data, updates)

	// Verify batch set results
	newCompany, _ := json.GetString(updated, "company")
	newVersion, _ := json.GetString(updated, "config.version")
	newAge, _ := json.GetInt(updated, "users[0].age")

	fmt.Printf("   After batch set - Company: %s, Version: %s, Alice's age: %d\n",
		newCompany, newVersion, newAge)
	// Result: SuperTechCorp, 2.0.0, 27
}
