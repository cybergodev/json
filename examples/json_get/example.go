package main

// ==================================================================================
// This is a comprehensive example of JSON retrieval functionality
//
// This example demonstrates different approaches to getting JSON data:
// 1. Basic value retrieval using json.Get() - returns any type
// 2. Type-safe retrieval using json.GetString(), json.GetInt(), etc.
// 3. Generic type-safe retrieval using json.GetTyped[T]()
// 4. Array element access - getting specific array elements by index
// 5. Array slice access - getting ranges of array elements
// 6. Nested object access - accessing deeply nested properties
// 7. Path expressions with wildcards - bulk data extraction using {}
// 8. Multiple path retrieval - getting multiple values in one operation
// 9. Custom processor usage - advanced configuration and caching
// 10. Error handling and edge cases
//
// The example uses nested JSON structure with arrays and objects to show how retrieval
// works with complex path expressions like "a{g}{name}" which extracts all "name" fields
// within nested "g" arrays inside the "a" array.
//
// Key features demonstrated:
// - Bulk extraction using path expressions with wildcards ({})
// - Type-safe operations with automatic type conversion
// - Array indexing including negative indices
// - Array slicing with start:end notation
// - Nested property access with dot notation
// - JSON Pointer format support
// - Batch operations for performance optimization
// - Custom processor configuration for advanced use cases
// ==================================================================================

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/cybergodev/json"
)

// Sample JSON data for basic retrieval examples
const jsonStr = `{
		"a":[
			{"g":[{"name":"11","value":100},null,{"name":"22","value":200},null]},
			{"g":[{"name":"aa","value":300},null,{"name":"bb","value":400},null,{"name":"cc","value":500},null]}
		],
		"init":"initialized",
		"count":42,
		"active":true,
		"metadata":null
	}`

type User struct {
	ID     int     `json:"id"`
	Name   string  `json:"name"`
	Age    int     `json:"age"`
	Active bool    `json:"active"`
	Salary float64 `json:"salary"`
}

// Sample JSON data for array retrieval examples
const arrayJsonStr = `{
	"users": [
		{"id": 1, "name": "Alice", "age": 25, "active": true, "salary": 75000.50},
		{"id": 2, "name": "Bob", "age": 30, "active": false, "salary": 80000.75},
		{"id": 3, "name": "Charlie", "age": 35, "active": true, "salary": 90000.00},
		{"id": 4, "name": "David", "age": 28, "active": false, "salary": 85000.25},
		{"id": 5, "name": "Eve", "age": 32, "active": true, "salary": 95000.00}
	],
	"metadata": {
		"total": 5,
		"version": "1.0",
		"created": "2024-01-01T00:00:00Z",
		"tags": ["production", "api", "v1"]
	}
}`

// Sample JSON data for complex nested retrieval examples
const complexJsonStr = `{
	"company": {
		"name": "TechCorp",
		"departments": [
			{
				"name": "Engineering",
				"budget": 1000000,
				"employees": [
					{"name": "John", "salary": 80000, "skills": ["Go", "Python", "Docker"]},
					{"name": "Jane", "salary": 85000, "skills": ["JavaScript", "React", "Node.js"]},
					{"name": "Jim", "salary": 75000, "skills": ["Java", "Spring", "Kubernetes"]}
				],
				"projects": [
					{"name": "ProjectAlpha", "status": "active", "priority": 1},
					{"name": "ProjectBeta", "status": "completed", "priority": 2}
				]
			},
			{
				"name": "Marketing",
				"budget": 500000,
				"employees": [
					{"name": "Mary", "salary": 70000, "skills": ["SEO", "Content", "Analytics"]},
					{"name": "Mike", "salary": 72000, "skills": ["Social Media", "PPC", "Design"]}
				],
				"projects": [
					{"name": "CampaignX", "status": "planning", "priority": 1}
				]
			}
		],
		"config": {
			"debug": true,
			"cache_ttl": 3600,
			"features": {
				"auth": true,
				"logging": true,
				"metrics": false
			}
		}
	}
}`

func main() {
	// Basic retrieval examples
	printLines("=== Basic Retrieval Examples ===")
	basicRetrieval()
	typeSafeRetrieval()

	// Array access examples
	printLines("=== Array Access Examples ===")
	arrayElementAccess()
	arraySliceAccess()

	// Nested object access examples
	printLines("=== Nested Object Access Examples ===")
	nestedObjectAccess()

	// Path expression examples
	printLines("=== Path Expression Examples ===")
	pathExpressionRetrieval()

	// Multiple path retrieval examples
	printLines("=== Multiple Path Retrieval Examples ===")
	multiplePathRetrieval()

	// Advanced processor examples
	printLines("=== Advanced Processor Examples ===")
	advancedProcessorUsage()

	// Error handling examples
	printLines("=== Error Handling Examples ===")
	errorHandlingExamples()

	// Performance optimization examples
	printLines("=== Performance Optimization Examples ===")
	performanceOptimization()
}

// Basic value retrieval using json.Get()
func basicRetrieval() {
	fmt.Println("\nBasic value retrieval using json.Get():")

	// Get string value
	initValue, err := json.Get(jsonStr, "init")
	if err != nil {
		log.Println("Error getting init:", err)
	} else {
		fmt.Printf("init value: %v (type: %T)\n", initValue, initValue)
		// Result: initialized (type: string)
	}

	// Get integer value
	count, err := json.Get(jsonStr, "count")
	if err != nil {
		log.Println("Error getting count:", err)
	} else {
		fmt.Printf("count value: %v (type: %T)\n", count, count)
		// Result: 42 (type: float64)
	}

	// Get boolean value
	active, err := json.Get(jsonStr, "active")
	if err != nil {
		log.Println("Error getting active:", err)
	} else {
		fmt.Printf("active value: %v (type: %T)\n", active, active)
		// Result: true (type: bool)
	}

	// Get null value
	metadata, err := json.Get(jsonStr, "metadata")
	if err != nil {
		log.Println("Error getting metadata:", err)
	} else {
		fmt.Printf("metadata value: %v (type: %T)\n", metadata, metadata)
		// Result: <nil> (type: <nil>)
	}

	// Get nested object
	firstG, err := json.Get(jsonStr, "a[0].g")
	if err != nil {
		log.Println("Error getting a[0].g:", err)
	} else {
		fmt.Printf("a[0].g value: %v (type: %T)\n", firstG, firstG)
		// Result: [map[name:11 value:100] <nil> map[name:22 value:200] <nil>] (type: []interface {})
	}

	// Get specific nested value
	firstName, err := json.Get(jsonStr, "a[0].g[0].name")
	if err != nil {
		log.Println("Error getting a[0].g[0].name:", err)
	} else {
		fmt.Printf("a[0].g[0].name value: %v (type: %T)\n", firstName, firstName)
		// Result: 11 (type: string)
	}
}

// Type-safe retrieval using specific getter functions
func typeSafeRetrieval() {
	fmt.Println(line)
	fmt.Println("\nType-safe retrieval using specific getter functions:")

	// String retrieval
	userName, err := json.GetString(arrayJsonStr, "users[0].name")
	if err != nil {
		log.Println("Error getting user name:", err)
	} else {
		fmt.Printf("User name: %s\n", userName)
		// Result: Alice
	}

	// Integer retrieval
	userAge, err := json.GetInt(arrayJsonStr, "users[1].age")
	if err != nil {
		log.Println("Error getting user age:", err)
	} else {
		fmt.Printf("User age: %d\n", userAge)
		// Result: 30
	}

	// Float retrieval
	userSalary, err := json.GetFloat64(arrayJsonStr, "users[2].salary")
	if err != nil {
		log.Println("Error getting user salary:", err)
	} else {
		fmt.Printf("User salary: %.2f\n", userSalary)
		// Result: 90000.00
	}

	// Boolean retrieval
	userActive, err := json.GetBool(arrayJsonStr, "users[0].active")
	if err != nil {
		log.Println("Error getting user active status:", err)
	} else {
		fmt.Printf("User active: %t\n", userActive)
		// Result: true
	}

	// Array retrieval
	tags, err := json.GetArray(arrayJsonStr, "metadata.tags")
	if err != nil {
		log.Println("Error getting tags:", err)
	} else {
		fmt.Printf("Tags: %v\n", tags)
		// Result: [production api v1]
	}

	// Object retrieval
	metadata, err := json.GetObject(arrayJsonStr, "metadata")
	if err != nil {
		log.Println("Error getting metadata object:", err)
	} else {
		fmt.Printf("Metadata: %v\n", metadata)
		// Result: map[created:2024-01-01T00:00:00Z tags:[production api v1] total:5 version:1.0]
	}

	// Generic type-safe retrieval
	fmt.Println("\nGeneric type-safe retrieval using json.GetTyped[T]():")

	// Get typed array of strings
	typedTags, err := json.GetTyped[[]string](arrayJsonStr, "metadata.tags")
	if err != nil {
		log.Println("Error getting typed tags:", err)
	} else {
		fmt.Printf("Typed tags: %v (type: %T)\n", typedTags, typedTags)
		// Result: [production api v1] (type: []string)
	}

	// Get typed integer
	typedTotal, err := json.GetTyped[int](arrayJsonStr, "metadata.total")
	if err != nil {
		log.Println("Error getting typed total:", err)
	} else {
		fmt.Printf("Typed total: %d (type: %T)\n", typedTotal, typedTotal)
		// Result: 5 (type: int)
	}

	// Get typed struct
	typedStruct, err := json.GetTyped[[]User](arrayJsonStr, "users")
	if err != nil {
		log.Println("Error getting typed struct:", err)
	} else {
		fmt.Printf("Typed struct: %v (type: %T)\n", typedStruct, typedStruct)
		// Result: [{1 Alice 25 true 75000.5} {2 Bob 30 false 80000.75} {3 Charlie 35 true 90000} {4 David 28 false 85000.25} {5 Eve 32 true 95000}] (type: []main.User)
	}

}

// Array element access using index notation
func arrayElementAccess() {
	fmt.Println("\nArray element access using index notation:")

	// Access first user
	fmt.Println("Accessing first user (index 0):")
	firstUser, err := json.Get(arrayJsonStr, "users[0]")
	if err != nil {
		log.Println("Error getting first user:", err)
	} else {
		fmt.Println("First user, `users[0]`:", firstUser)
		// Result: map[active:true age:25 id:1 name:Alice salary:75000.5]
	}

	// Access last user using negative index
	fmt.Println("\nAccessing last user (negative index -1):")
	lastUser, err := json.GetString(arrayJsonStr, "users[-1]")
	if err != nil {
		log.Println("Error getting last user:", err)
	} else {
		fmt.Printf("Last user name, `users[-1]`: %s\n", lastUser)
		// Result: map[active:true age:32 id:5 name:Eve salary:95000]
	}

	// Access specific field from array element
	fmt.Println("\nAccessing specific fields from array elements:")
	secondUserAge, err := json.GetInt(arrayJsonStr, "users[1].age")
	if err != nil {
		log.Println("Error getting second user age:", err)
	} else {
		fmt.Printf("Second user age, `users[1].age`: %d\n", secondUserAge)
		// Result: 30
	}

	thirdUserSalary, err := json.GetFloat64(arrayJsonStr, "users[2].salary")
	if err != nil {
		log.Println("Error getting third user salary:", err)
	} else {
		fmt.Printf("Third user salary, `users[2].salary`: %.2f\n", thirdUserSalary)
		// Result: 90000.00
	}

	// Access nested array element
	fmt.Println("\nAccessing nested array elements:")
	firstTag, err := json.GetString(arrayJsonStr, "metadata.tags[0]")
	if err != nil {
		log.Println("Error getting first tag:", err)
	} else {
		fmt.Printf("First tag, `metadata.tags[0]`: %s\n", firstTag)
		// Result: production
	}

	lastTag, err := json.GetString(arrayJsonStr, "metadata.tags[-1]")
	if err != nil {
		log.Println("Error getting last tag:", err)
	} else {
		fmt.Printf("Last tag, `metadata.tags[-1]`: %s\n", lastTag)
		// Result: v1
	}
}

// Array slice access using range notation
func arraySliceAccess() {
	fmt.Println(line)
	fmt.Println("Array slice access using range notation:")

	// Get first 3 users
	fmt.Println("Getting first 3 users (users[0:3]):")
	firstThreeUsers, err := json.Get(arrayJsonStr, "users[0:3]")
	if err != nil {
		log.Println("Error getting first three users:", err)
	} else {
		fmt.Println("First 3 users, `users[0:3]`:", firstThreeUsers)
		// Result:
		// [
		// 	 map[active:true age:25 id:1 name:Alice salary:75000.5]
		// 	 map[active:false age:30 id:2 name:Bob salary:80000.75]
		// 	 map[active:true age:35 id:3 name:Charlie salary:90000]
		// ]
	}

	// Get users from index 2 to end
	fmt.Println("\nGetting users from index 2 to end (users[2:]):")
	usersFromTwo, err := json.Get(arrayJsonStr, "users[2:]")
	if err != nil {
		log.Println("Error getting users from index 2:", err)
	} else {
		fmt.Println("Users from index 2 to end, `users[2:]`:", usersFromTwo)
		// Result:
		// [
		// 	 map[active:true age:35 id:3 name:Charlie salary:90000]
		// 	 map[active:false age:28 id:4 name:David salary:85000.25]
		// 	 map[active:true age:32 id:5 name:Eve salary:95000]
		// ]
	}

	// Get last 2 users
	fmt.Println("\nGetting last 2 users (users[-2:]):")
	lastTwoUsers, err := json.Get(arrayJsonStr, "users[-2:]")
	if err != nil {
		log.Println("Error getting last two users:", err)
	} else {
		fmt.Println("Last 2 users, `users[-2:]`:", lastTwoUsers)
		// Result:
		// [
		// 	 map[active:false age:28 id:4 name:David salary:85000.25]
		// 	 map[active:true age:32 id:5 name:Eve salary:95000]
		// ]
	}

	// Get middle users (excluding first and last)
	fmt.Println("\nGetting middle users (users[2:-1]):")
	middleUsers, err := json.Get(arrayJsonStr, "users[2:-1]")
	if err != nil {
		log.Println("Error getting middle users:", err)
	} else {
		fmt.Println("Middle users, `users[2:-1]`:", middleUsers)
		// Result:
		// [
		// 	 map[active:true age:35 id:3 name:Charlie salary:90000]
		// 	 map[active:false age:28 id:4 name:David salary:85000.25]]
		// ]
	}
}

// Nested object access using dot notation
func nestedObjectAccess() {
	fmt.Println("\nNested object access using dot notation:")

	// Access company name
	companyName, err := json.GetString(complexJsonStr, "company.name")
	if err != nil {
		log.Println("Error getting company name:", err)
	} else {
		fmt.Printf("Company name, `company.name`: %s\n", companyName)
		// Result: TechCorp
	}

	// Access nested configuration
	debugMode, err := json.GetBool(complexJsonStr, "company.config.debug")
	if err != nil {
		log.Println("Error getting debug mode:", err)
	} else {
		fmt.Printf("Debug mode, `company.config.debug`: %t\n", debugMode)
		// Result: true
	}

	cacheTTL, err := json.GetInt(complexJsonStr, "company.config.cache_ttl")
	if err != nil {
		log.Println("Error getting cache TTL:", err)
	} else {
		fmt.Printf("Cache TTL, `company.config.cache_ttl`: %d seconds\n", cacheTTL)
		// Result: 3600
	}

	// Access deeply nested features
	authEnabled, err := json.GetBool(complexJsonStr, "company.config.features.auth")
	if err != nil {
		log.Println("Error getting auth feature:", err)
	} else {
		fmt.Printf("Auth enabled, `company.config.features.auth`: %t\n", authEnabled)
		// Result: true
	}

	loggingEnabled, err := json.GetBool(complexJsonStr, "company.config.features.logging")
	if err != nil {
		log.Println("Error getting logging feature:", err)
	} else {
		fmt.Printf("Logging enabled, `company.config.features.logging`: %t\n", loggingEnabled)
		// Result: true
	}

	// Access array elements within nested objects
	fmt.Println("\nAccessing array elements within nested objects:")

	firstDeptName, err := json.GetString(complexJsonStr, "company.departments[0].name")
	if err != nil {
		log.Println("Error getting first department name:", err)
	} else {
		fmt.Printf("First department, `company.departments[0].name`: %s\n", firstDeptName)
		// Result: Engineering
	}

	firstDeptBudget, err := json.GetInt(complexJsonStr, "company.departments[0].budget")
	if err != nil {
		log.Println("Error getting first department budget:", err)
	} else {
		fmt.Printf("First department budget, `company.departments[0].budget`: $%d\n", firstDeptBudget)
		// Result: $1000000
	}

	// Access employee information
	firstEmployee, err := json.GetString(complexJsonStr, "company.departments[0].employees[0].name")
	if err != nil {
		log.Println("Error getting first employee name:", err)
	} else {
		fmt.Printf("First employee, `company.departments[0].employees[0].name`: %s\n", firstEmployee)
		// Result: John
	}

	firstEmployeeSalary, err := json.GetInt(complexJsonStr, "company.departments[0].employees[0].salary")
	if err != nil {
		log.Println("Error getting first employee salary:", err)
	} else {
		fmt.Printf("First employee salary, `company.departments[0].employees[0].salary`: $%d\n", firstEmployeeSalary)
		// Result: $80000
	}

	// Access skills array
	firstEmployeeSkills, err := json.GetArray(complexJsonStr, "company.departments[0].employees[0].skills")
	if err != nil {
		log.Println("Error getting first employee skills:", err)
	} else {
		fmt.Printf("First employee skills, `company.departments[0].employees[0].skills`: %v\n", firstEmployeeSkills)
		// Result: [Go Python Docker]
	}

	// Access specific skill
	firstSkill, err := json.GetString(complexJsonStr, "company.departments[0].employees[0].skills[0]")
	if err != nil {
		log.Println("Error getting first skill:", err)
	} else {
		fmt.Printf("First skill, `company.departments[0].employees[0].skills[0]`: %s\n", firstSkill)
		// Result: Go
	}
}

// Path expression retrieval using wildcard notation
func pathExpressionRetrieval() {
	fmt.Println("\nPath expression retrieval using wildcard notation:")

	// Extract all names using path expression
	fmt.Println("Extracting all names using 'a{g}{name}':")
	allNames, err := json.Get(jsonStr, "a{g}{name}")
	if err != nil {
		log.Println("Error getting all names:", err)
	} else {
		fmt.Printf("All names, `a{g}{name}`: %v\n", allNames)
		// Result: [[11 22] [aa bb cc]]
	}

	// Extract all values using path expression
	fmt.Println("\nExtracting all values using 'a{g}{value}':")
	allValues, err := json.Get(jsonStr, "a{g}{value}")
	if err != nil {
		log.Println("Error getting all values:", err)
	} else {
		fmt.Printf("All values, `a{g}{value}`: %v\n", allValues)
		// Result: [[100 200] [300 400 500]]
	}

	// Complex path expressions with array data
	fmt.Println("\nComplex path expressions with array data:")

	// Extract all usernames
	fmt.Println("Extracting all user names using 'users{name}':")
	userNames, err := json.Get(arrayJsonStr, "users{name}")
	if err != nil {
		log.Println("Error getting user names:", err)
	} else {
		fmt.Printf("User names, `users{name}`: %v\n", userNames)
		// Result: [Alice Bob Charlie David Eve]
	}

	// Extract all user IDs
	fmt.Println("\nExtracting all user IDs using 'users{id}':")
	userIDs, err := json.Get(arrayJsonStr, "users{id}")
	if err != nil {
		log.Println("Error getting user IDs:", err)
	} else {
		fmt.Printf("User IDs, `users{id}`: %v\n", userIDs)
		// Result: [1 2 3 4 5]
	}

	// Extract all salaries
	fmt.Println("\nExtracting all salaries using 'users{salary}':")
	salaries, err := json.Get(arrayJsonStr, "users{salary}")
	if err != nil {
		log.Println("Error getting salaries:", err)
	} else {
		fmt.Printf("Salaries, `users{salary}`: %v\n", salaries)
		// Result: [75000.5 80000.75 90000 85000.25 95000]
	}

	// Complex nested path expressions
	fmt.Println("\nComplex nested path expressions:")

	// Extract all employee names from all departments
	fmt.Println("Extracting all employee names using 'company.departments{employees}{name}':")
	employeeNames, err := json.Get(complexJsonStr, "company.departments{employees}{name}")
	if err != nil {
		log.Println("Error getting employee names:", err)
	} else {
		fmt.Printf("Employee names, `company.departments{employees}{name}`: %v\n", employeeNames)
		// Result: [[John Jane Jim] [Mary Mike]]
	}

	// Extract all skills from all employees
	fmt.Println("\nExtracting all skills using 'company.departments{employees}{flat:skills}':")
	allSkills, err := json.Get(complexJsonStr, "company.departments{employees}{flat:skills}")
	if err != nil {
		log.Println("Error getting all skills:", err)
	} else {
		fmt.Printf("All skills, `company.departments{employees}{flat:skills}`: %v\n", allSkills)
		// Result:
		// [
		// 	  [Go Python Docker JavaScript React Node.js Java Spring Kubernetes]
		// 	  [SEO Content Analytics Social Media PPC Design]
		// ]
	}

	// Extract project names
	fmt.Println("\nExtracting all project names using 'company.departments{projects}{name}':")
	projectNames, err := json.Get(complexJsonStr, "company.departments{projects}{name}")
	if err != nil {
		log.Println("Error getting project names:", err)
	} else {
		fmt.Printf("Project names, `company.departments{projects}{name}`: %v\n", projectNames)
		// Result: [[ProjectAlpha ProjectBeta] [CampaignX]]
	}
}

// Multiple path retrieval for batch operations
func multiplePathRetrieval() {
	fmt.Println("\nMultiple path retrieval for batch operations:")

	// Define multiple paths to retrieve
	paths := []string{
		"metadata.total",
		"metadata.version",
		"metadata.created",
		"users[0].name",
		"users[0].age",
		"users[-1].name",
		"users[-1].salary",
		"metadata.tags[0]",
		"metadata.tags[-1]",
	}

	// Get multiple values in one operation
	results, err := json.GetMultiple(arrayJsonStr, paths)
	if err != nil {
		log.Println("Error getting multiple values:", err)
	} else {
		fmt.Println("\nResults:")
		for path, value := range results {
			fmt.Printf("  %s: %s (type: %T)\n", path, formatDisplayValue(value), value)
		}
		fmt.Println("Note: GetMultiple() is more efficient than individual Get() calls")
	}

	// Complex multiple path retrieval
	fmt.Println("\nComplex multiple path retrieval:")
	complexPaths := []string{
		"company.name",
		"company.departments[0].name",
		"company.departments[0].budget",
		"company.departments[1].name",
		"company.departments[1].budget",
		"company.config.debug",
		"company.config.cache_ttl",
		"company.config.features.auth",
		"company.config.features.logging",
	}

	complexResults, err := json.GetMultiple(complexJsonStr, complexPaths)
	if err != nil {
		log.Println("Error getting complex multiple values:", err)
	} else {
		fmt.Println("Complex results:")
		for path, value := range complexResults {
			fmt.Printf("  %s: %s\n", path, formatDisplayValue(value))
		}
	}
}

// Advanced processor usage with custom configuration
func advancedProcessorUsage() {
	fmt.Println("Advanced processor usage with custom configuration:")

	// Create a custom processor with specific configuration
	config := json.DefaultConfig()
	config.EnableCache = true
	config.MaxCacheSize = 100
	config.CacheTTL = 300            // 5 minutes
	config.MaxJSONSize = 1024 * 1024 // 1MB

	processor := json.New(config)
	defer processor.Close()

	fmt.Println("Created custom processor with caching enabled")

	// Use the processor for operations
	fmt.Println("\nUsing custom processor for operations:")

	// First call - will be cached
	start := time.Now()
	result1, err := processor.Get(complexJsonStr, "company.departments{employees}{name}")
	duration1 := time.Since(start)
	if err != nil {
		log.Println("Error with processor get:", err)
	} else {
		fmt.Printf("First call result: %v (took: %v)\n", result1, duration1)
	}

	// Second call - should be faster due to caching
	start = time.Now()
	result2, err := processor.Get(complexJsonStr, "company.departments{employees}{name}")
	duration2 := time.Since(start)
	if err != nil {
		log.Println("Error with processor get:", err)
	} else {
		fmt.Printf("Second call result: %v (took: %v)\n", result2, duration2)

		// Calculate performance improvement with proper handling of very small durations
		if duration2.Nanoseconds() > 0 {
			improvement := float64(duration1.Nanoseconds()) / float64(duration2.Nanoseconds())
			if improvement > 1000 {
				fmt.Printf("Performance improvement: >1000x faster (second call was cached)\n")
			} else {
				fmt.Printf("Performance improvement: %.2fx faster\n", improvement)
			}
		} else {
			fmt.Printf("Performance improvement: Second call was instantaneous (cached)\n")
		}
	}

	// Get processor statistics
	stats := processor.GetStats()
	fmt.Printf("\nProcessor statistics:\n")
	fmt.Printf("  Cache size: %d\n", stats.CacheSize)
	fmt.Printf("  Cache hits: %d\n", stats.HitCount)
	fmt.Printf("  Cache misses: %d\n", stats.MissCount)
	fmt.Printf("  Hit ratio: %.2f%%\n", stats.HitRatio*100)
	fmt.Printf("  Operations: %d\n", stats.OperationCount)

	// Demonstrate processor options
	fmt.Println("\nUsing processor with custom options:")
	opts := &json.ProcessorOptions{
		CacheResults: false, // Disable caching for this operation
		StrictMode:   true,  // Enable strict mode
	}

	result3, err := processor.Get(complexJsonStr, "company.departments[0].employees[0].skills", opts)
	if err != nil {
		log.Println("Error with options:", err)
	} else {
		fmt.Printf("Result with custom options: %v\n", result3)
	}
}

// Error handling examples
func errorHandlingExamples() {
	fmt.Println("Error handling examples:")

	// Invalid JSON
	fmt.Println("\n1. Invalid JSON:")
	invalidJSON := `{"name": "John", "age":}`
	_, err := json.Get(invalidJSON, "name")
	if err != nil {
		fmt.Printf("Expected error for invalid JSON: %v\n", err)
	}

	// Non-existent path
	fmt.Println("\n2. Non-existent path:")
	result, err := json.Get(arrayJsonStr, "nonexistent.field")
	if err != nil {
		fmt.Printf("Error for non-existent path: %v\n", err)
	} else {
		fmt.Printf("Result for non-existent path: %v (should be nil)\n", result)
	}

	// Invalid array index
	fmt.Println("\n3. Invalid array index:")
	_, err = json.Get(arrayJsonStr, "users[100]")
	if err != nil {
		fmt.Printf("Expected error for invalid array index: %v\n", err)
	}

	// Invalid path syntax
	fmt.Println("\n4. Invalid path syntax:")
	_, err = json.Get(arrayJsonStr, "users[invalid]")
	if err != nil {
		fmt.Printf("Expected error for invalid path syntax: %v\n", err)
	}

	// Type conversion errors
	fmt.Println("\n5. Type conversion errors:")
	_, err = json.GetInt(arrayJsonStr, "users[0].name") // name is string, not int
	if err != nil {
		fmt.Printf("Expected error for type conversion: %v\n", err)
	}

	// Null value handling
	fmt.Println("\n6. Null value handling:")
	nullJSON := `{"value": null, "number": 0, "empty": ""}`

	nullValue, err := json.Get(nullJSON, "value")
	if err != nil {
		fmt.Printf("Error getting null value: %v\n", err)
	} else {
		fmt.Printf("Null value: %v (type: %T)\n", nullValue, nullValue)
	}

	// Getting string from null should return empty string
	nullString, err := json.GetString(nullJSON, "value")
	if err != nil {
		fmt.Printf("Error getting null as string: %v\n", err)
	} else {
		fmt.Printf("Null as string: '%s'\n", nullString)
	}

	// Getting int from null should return zero
	nullInt, err := json.GetInt(nullJSON, "value")
	if err != nil {
		fmt.Printf("Error getting null as int: %v\n", err)
	} else {
		fmt.Printf("Null as int: %d\n", nullInt)
	}
}

// Performance optimization examples
func performanceOptimization() {
	fmt.Println("Performance optimization examples:")

	// Batch operations vs individual operations
	fmt.Println("\n1. Batch vs Individual Operations:")

	paths := []string{
		"users[0].name", "users[0].age", "users[0].salary",
		"users[1].name", "users[1].age", "users[1].salary",
		"users[2].name", "users[2].age", "users[2].salary",
	}

	// Individual operations
	start := time.Now()
	for _, path := range paths {
		_, err := json.Get(arrayJsonStr, path)
		if err != nil {
			log.Printf("Error getting %s: %v", path, err)
		}
	}
	individualDuration := time.Since(start)

	// Batch operation
	start = time.Now()
	_, err := json.GetMultiple(arrayJsonStr, paths)
	if err != nil {
		log.Printf("Error in batch operation: %v", err)
	}
	batchDuration := time.Since(start)

	fmt.Printf("Individual operations took: %v\n", individualDuration)
	fmt.Printf("Batch operation took: %v\n", batchDuration)
	if batchDuration.Nanoseconds() > 0 {
		improvement := float64(individualDuration.Nanoseconds()) / float64(batchDuration.Nanoseconds())
		if improvement > 1000 {
			fmt.Printf("Batch is >1000x faster\n")
		} else {
			fmt.Printf("Batch is %.2fx faster\n", improvement)
		}
	} else {
		fmt.Printf("Batch operation was instantaneous\n")
	}

	// Path expressions vs multiple individual gets
	fmt.Println("\n2. Path Expressions vs Individual Gets:")

	// Individual gets for all names
	start = time.Now()
	for i := 0; i < 5; i++ {
		path := fmt.Sprintf("users[%d].name", i)
		_, err := json.Get(arrayJsonStr, path)
		if err != nil {
			log.Printf("Error getting %s: %v", path, err)
		}
	}
	individualNamesDuration := time.Since(start)

	// Path expression for all names
	start = time.Now()
	_, err = json.Get(arrayJsonStr, "users{name}")
	if err != nil {
		log.Printf("Error with path expression: %v", err)
	}
	pathExpressionDuration := time.Since(start)

	fmt.Printf("Individual name gets took: %v\n", individualNamesDuration)
	fmt.Printf("Path expression took: %v\n", pathExpressionDuration)
	if pathExpressionDuration.Nanoseconds() > 0 {
		improvement := float64(individualNamesDuration.Nanoseconds()) / float64(pathExpressionDuration.Nanoseconds())
		if improvement > 1000 {
			fmt.Printf("Path expression is >1000x faster\n")
		} else {
			fmt.Printf("Path expression is %.2fx faster\n", improvement)
		}
	} else {
		fmt.Printf("Path expression was instantaneous\n")
	}

	// Processor reuse vs creating new processors
	fmt.Println("\n3. Processor Reuse vs New Processors:")

	// Creating new processors each time
	start = time.Now()
	for i := 0; i < 10; i++ {
		processor := json.New()
		_, err := processor.Get(arrayJsonStr, "users[0].name")
		if err != nil {
			log.Printf("Error with new processor: %v", err)
		}
		processor.Close()
	}
	newProcessorDuration := time.Since(start)

	// Reusing single processor
	processor := json.New()
	defer processor.Close()

	start = time.Now()
	for i := 0; i < 10; i++ {
		_, err := processor.Get(arrayJsonStr, "users[0].name")
		if err != nil {
			log.Printf("Error with reused processor: %v", err)
		}
	}
	reusedProcessorDuration := time.Since(start)

	fmt.Printf("New processors each time took: %v\n", newProcessorDuration)
	fmt.Printf("Reused processor took: %v\n", reusedProcessorDuration)
	if reusedProcessorDuration.Nanoseconds() > 0 {
		improvement := float64(newProcessorDuration.Nanoseconds()) / float64(reusedProcessorDuration.Nanoseconds())
		if improvement > 1000 {
			fmt.Printf("Processor reuse is >1000x faster\n")
		} else {
			fmt.Printf("Processor reuse is %.2fx faster\n", improvement)
		}
	} else {
		fmt.Printf("Reused processor was instantaneous\n")
	}

	fmt.Println("\nPerformance Tips:")
	fmt.Println("- Use GetMultiple() for batch operations")
	fmt.Println("- Use path expressions with {} for bulk data extraction")
	fmt.Println("- Reuse processors instead of creating new ones")
	fmt.Println("- Enable caching for repeated operations")
	fmt.Println("- Use type-safe getters (GetString, GetInt, etc.) when possible")
}

var line = strings.Repeat("----", 20)

func printLines(title string) {
	fmt.Println(line)
	fmt.Println(title)
}

// formatDisplayValue formats a value for display, avoiding scientific notation for numbers
func formatDisplayValue(value any) string {
	switch v := value.(type) {
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return fmt.Sprintf("%d", v)
	case float32, float64:
		// Use FormatNumber to avoid scientific notation
		return json.FormatNumber(v)
	case string:
		return v
	case bool:
		return fmt.Sprintf("%t", v)
	case nil:
		return "<nil>"
	default:
		return fmt.Sprintf("%v", v)
	}
}
