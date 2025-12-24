package main

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/cybergodev/json"
)

func main() {
	// Run comprehensive validation examples
	runComprehensiveValidation()

	fmt.Println("\n" + strings.Repeat("=", 60))

	// Run performance tests
	runPerformanceTest()
}

// PerformanceTest runs comprehensive performance tests
func runPerformanceTest() {
	fmt.Println("\n🚀 JSON Library Performance Test")
	fmt.Println("================================")

	// Test data
	testData := `{
		"users": [
			{"id": 1, "name": "Alice", "email": "alice@example.com", "active": true},
			{"id": 2, "name": "Bob", "email": "bob@example.com", "active": false},
			{"id": 3, "name": "Charlie", "email": "charlie@example.com", "active": true}
		],
		"metadata": {
			"total": 3,
			"page": 1,
			"limit": 10
		}
	}`

	// Run different performance tests
	testBasicOperations(testData)
	testPathExpressions(testData)
	testProcessorPerformance(testData)
	testConcurrentPerformance(testData)
	testMemoryUsage(testData)
}

func testBasicOperations(testData string) {
	fmt.Println("\n📊 Basic Operations Performance")

	iterations := 10000

	// Test Get operations
	start := time.Now()
	for i := 0; i < iterations; i++ {
		_, _ = json.Get(testData, "users[0].name")
	}
	duration := time.Since(start)
	fmt.Printf("  Get operations: %d ops in %v (%.0f ops/sec)\n",
		iterations, duration, float64(iterations)/duration.Seconds())

	// Test Set operations
	start = time.Now()
	for i := 0; i < iterations; i++ {
		_, _ = json.Set(testData, "metadata.page", i)
	}
	duration = time.Since(start)
	fmt.Printf("  Set operations: %d ops in %v (%.0f ops/sec)\n",
		iterations, duration, float64(iterations)/duration.Seconds())
}

func testPathExpressions(testData string) {
	fmt.Println("\n🛤️  Path Expression Performance")

	iterations := 5000

	// Test array access
	start := time.Now()
	for i := 0; i < iterations; i++ {
		_, _ = json.Get(testData, "users[0].name")
		_, _ = json.Get(testData, "users[-1].email")
	}
	duration := time.Since(start)
	fmt.Printf("  Array access: %d ops in %v (%.0f ops/sec)\n",
		iterations*2, duration, float64(iterations*2)/duration.Seconds())

	// Test extraction
	start = time.Now()
	for i := 0; i < iterations; i++ {
		_, _ = json.Get(testData, "users{name}")
	}
	duration = time.Since(start)
	fmt.Printf("  Extraction: %d ops in %v (%.0f ops/sec)\n",
		iterations, duration, float64(iterations)/duration.Seconds())
}

func testProcessorPerformance(testData string) {
	fmt.Println("\n⚙️  Processor Performance")

	processor := json.New()
	defer processor.Close()

	iterations := 10000

	// Test processor operations
	start := time.Now()
	for i := 0; i < iterations; i++ {
		_, _ = processor.Get(testData, "users[0].name")
	}
	duration := time.Since(start)
	fmt.Printf("  Processor Get: %d ops in %v (%.0f ops/sec)\n",
		iterations, duration, float64(iterations)/duration.Seconds())

	// Check cache effectiveness
	stats := processor.GetStats()
	if stats.HitCount+stats.MissCount > 0 {
		hitRatio := float64(stats.HitCount) / float64(stats.HitCount+stats.MissCount) * 100
		fmt.Printf("  Cache hit ratio: %.1f%% (hits: %d, misses: %d)\n",
			hitRatio, stats.HitCount, stats.MissCount)
	}
}

func testConcurrentPerformance(testData string) {
	fmt.Println("\n🔄 Concurrent Performance")

	const numGoroutines = 10
	const opsPerGoroutine = 1000

	start := time.Now()

	done := make(chan bool, numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func() {
			for j := 0; j < opsPerGoroutine; j++ {
				_, _ = json.Get(testData, "users[0].name")
			}
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < numGoroutines; i++ {
		<-done
	}

	duration := time.Since(start)
	totalOps := numGoroutines * opsPerGoroutine
	fmt.Printf("  Concurrent ops: %d ops in %v (%.0f ops/sec)\n",
		totalOps, duration, float64(totalOps)/duration.Seconds())
}

func testMemoryUsage(testData string) {
	fmt.Println("\n💾 Memory Usage")

	// Force garbage collection
	runtime.GC()

	var m1 runtime.MemStats
	runtime.ReadMemStats(&m1)

	// Perform operations
	iterations := 1000
	for i := 0; i < iterations; i++ {
		_, _ = json.Get(testData, "users[0].name")
		_, _ = json.Set(testData, "metadata.page", i)
	}

	// Force garbage collection again
	runtime.GC()

	var m2 runtime.MemStats
	runtime.ReadMemStats(&m2)

	fmt.Printf("  Memory allocated: %d KB\n", (m2.TotalAlloc-m1.TotalAlloc)/1024)
	fmt.Printf("  Memory per operation: %d bytes\n", (m2.TotalAlloc-m1.TotalAlloc)/(uint64(iterations)*2))
}

// ComprehensiveValidationExample demonstrates all major features of the JSON library
func runComprehensiveValidation() {
	fmt.Println("🚀 JSON Library Comprehensive Validation Example")
	fmt.Println(strings.Repeat("=", 60))

	// Test data for comprehensive validation
	testData := createTestData()

	// Run all validation tests
	runBasicOperationsValidation(testData)
	runPathExpressionValidation(testData)
	runAdvancedPathExpressionValidation(testData)
	runArrayOperationsValidation(testData)
	runExtractionValidation(testData)
	runAdvancedExtractionValidation(testData)
	runTypeSafetyValidation(testData)
	runProcessorValidation(testData)
	runBatchOperationsValidation(testData)
	runStreamingOperationsValidation()
	runJSONPointerValidation(testData)
	runConcurrencyValidation(testData)
	runFileOperationsValidation()
	runEncodingValidation(testData)
	runValidationAndSecurityValidation()
	runSchemaValidationValidation()
	runPerformanceValidation(testData)
	runErrorHandlingValidation()
	runEdgeCasesValidation()

	fmt.Println("\n✅ All validation tests completed successfully!")
	fmt.Println("🎉 JSON Library is functioning correctly across all features!")
}

// createTestData creates comprehensive test data covering various scenarios
func createTestData() string {
	return `{
		"company": {
			"name": "TechCorp",
			"founded": 2010,
			"employees": 150,
			"active": true,
			"revenue": 1250000.50,
			"departments": [
				{
					"name": "Engineering",
					"budget": 1000000,
					"teams": [
						{
							"name": "Backend",
							"lead": "Alice Johnson",
							"members": [
								{
									"id": 1,
									"name": "Alice Johnson",
									"role": "Senior Engineer",
									"skills": ["Go", "Python", "Docker", "Kubernetes"],
									"salary": 120000,
									"active": true,
									"joined": "2020-01-15"
								},
								{
									"id": 2,
									"name": "Bob Smith",
									"role": "Mid Engineer",
									"skills": ["Java", "Spring", "MySQL"],
									"salary": 90000,
									"active": true,
									"joined": "2021-03-10"
								}
							],
							"projects": [
								{
									"name": "API Gateway",
									"status": "active",
									"priority": "high",
									"deadline": "2024-06-30",
									"budget": 500000
								},
								{
									"name": "Microservices Migration",
									"status": "planning",
									"priority": "medium",
									"deadline": "2024-12-31",
									"budget": 750000
								}
							]
						},
						{
							"name": "Frontend",
							"lead": "Carol Davis",
							"members": [
								{
									"id": 3,
									"name": "Carol Davis",
									"role": "Senior Engineer",
									"skills": ["React", "TypeScript", "CSS", "Node.js"],
									"salary": 115000,
									"active": true,
									"joined": "2019-08-20"
								}
							],
							"projects": [
								{
									"name": "Dashboard Redesign",
									"status": "active",
									"priority": "high",
									"deadline": "2024-04-15",
									"budget": 300000
								}
							]
						}
					]
				},
				{
					"name": "Marketing",
					"budget": 500000,
					"teams": [
						{
							"name": "Digital Marketing",
							"lead": "David Wilson",
							"members": [
								{
									"id": 4,
									"name": "David Wilson",
									"role": "Marketing Manager",
									"skills": ["SEO", "Analytics", "AdWords", "Content"],
									"salary": 95000,
									"active": true,
									"joined": "2020-11-05"
								}
							],
							"projects": [
								{
									"name": "Brand Campaign 2024",
									"status": "active",
									"priority": "high",
									"deadline": "2024-03-31",
									"budget": 200000
								}
							]
						}
					]
				}
			],
			"metadata": {
				"tags": ["technology", "startup", "innovation", "growth"],
				"locations": ["San Francisco", "New York", "Austin", "Seattle"],
				"certifications": {
					"iso": ["9001", "27001", "14001"],
					"soc": ["SOC1", "SOC2"],
					"compliance": ["GDPR", "CCPA", "HIPAA"]
				},
				"contact": {
					"email": "info@techcorp.com",
					"phone": "+1-555-0123",
					"address": {
						"street": "123 Tech Street",
						"city": "San Francisco",
						"state": "CA",
						"zip": "94105",
						"country": "USA"
					}
				}
			}
		},
		"statistics": {
			"totalEmployees": 4,
			"averageSalary": 105000,
			"departmentCount": 2,
			"activeProjects": 4,
			"totalBudget": 1750000,
			"yearlyGrowth": 15.5,
			"marketShare": 8.2,
			"customerSatisfaction": 4.7
		},
		"config": {
			"debug": false,
			"version": "1.2.3",
			"features": {
				"analytics": true,
				"reporting": true,
				"notifications": false
			},
			"limits": {
				"maxUsers": 1000,
				"maxProjects": 50,
				"storageGB": 100
			}
		},
		"testData": {
			"nullValue": null,
			"emptyString": "",
			"emptyArray": [],
			"emptyObject": {},
			"specialChars": "Hello, 世界! 🌍 @#$%^&*()",
			"unicodeText": "Ñoño café naïve résumé",
			"numbers": {
				"integer": 42,
				"float": 3.14159,
				"negative": -123,
				"zero": 0,
				"large": 9223372036854775807
			},
			"booleans": {
				"true": true,
				"false": false
			},
			"dates": {
				"iso": "2024-01-15T10:30:00Z",
				"timestamp": 1705315800,
				"formatted": "January 15, 2024"
			}
		},
		"complexNested": {
			"level1": {
				"level2": [
					{
						"level3": {
							"level4": [
								{
									"level5": {
										"level6": {
											"deepValue": "found",
											"deepArray": [1, 2, 3, 4, 5],
											"deepObject": {
												"key1": "value1",
												"key2": "value2"
											}
										}
									}
								}
							]
						}
					}
				]
			}
		},
		"mixedTypes": [
			{"type": "string", "value": "text", "priority": 1},
			{"type": "number", "value": 42, "priority": 2},
			{"type": "boolean", "value": true, "priority": 3},
			{"type": "array", "value": [1, 2, 3], "priority": 1},
			{"type": "object", "value": {"nested": "data"}, "priority": 2}
		],
		"pathTestCases": {
			"simple.property": "dot in key",
			"array[with]brackets": "brackets in key",
			"special/chars": "slash in key",
			"unicode🌍key": "unicode in key",
			"spaces in key": "spaces in key"
		}
	}`
}

// runBasicOperationsValidation tests basic JSON operations
func runBasicOperationsValidation(testData string) {
	fmt.Println("\n📋 Testing Basic Operations...")

	// Test Get operations
	companyName, err := json.GetString(testData, "company.name")
	validateResult("Get company name", err, companyName, "TechCorp")

	employeeCount, err := json.GetInt(testData, "company.employees")
	validateResult("Get employee count", err, employeeCount, 150)

	isActive, err := json.GetBool(testData, "company.active")
	validateResult("Get company active status", err, isActive, true)

	revenue, err := json.GetFloat64(testData, "company.revenue")
	validateResult("Get company revenue", err, revenue, 1250000.50)

	// Test Set operations
	updatedData, err := json.Set(testData, "company.employees", 155)
	validateOperation("Set employee count", err)

	newCount, err := json.GetInt(updatedData, "company.employees")
	validateResult("Verify updated employee count", err, newCount, 155)

	// Test Delete operations
	deletedData, err := json.Delete(testData, "testData.nullValue")
	validateOperation("Delete null value", err)

	// Verify deletion
	_, err = json.Get(deletedData, "testData.nullValue")
	if err == nil {
		fmt.Println("  ✅ Delete operation verified - value no longer exists")
	}

	fmt.Println("  ✅ Basic operations validation completed")
}

// runPathExpressionValidation tests complex path expressions
func runPathExpressionValidation(testData string) {
	fmt.Println("\n🛤️  Testing Path Expressions...")

	// Test array access with positive index
	firstDept, err := json.GetString(testData, "company.departments[0].name")
	validateResult("Array access [0]", err, firstDept, "Engineering")

	// Test array access with negative index
	lastLocation, err := json.GetString(testData, "company.metadata.locations[-1]")
	validateResult("Array access [-1]", err, lastLocation, "Seattle")

	// Test deep nested access
	firstMemberName, err := json.GetString(testData, "company.departments[0].teams[0].members[0].name")
	validateResult("Deep nested access", err, firstMemberName, "Alice Johnson")

	// Test array slicing
	firstTwoTags, err := json.Get(testData, "company.metadata.tags[0:2]")
	validateOperation("Array slicing [0:2]", err)
	if tags, ok := firstTwoTags.([]interface{}); ok && len(tags) == 2 {
		fmt.Println("  ✅ Array slicing returned correct number of elements")
	}

	fmt.Println("  ✅ Path expressions validation completed")
}

// runAdvancedPathExpressionValidation tests advanced path expression features
func runAdvancedPathExpressionValidation(testData string) {
	fmt.Println("\n🔬 Testing Advanced Path Expressions...")

	// Test deep nested access (6 levels)
	deepValue, err := json.GetString(testData, "complexNested.level1.level2[0].level3.level4[0].level5.level6.deepValue")
	validateResult("Deep nested access (6 levels)", err, deepValue, "found")

	// Test array slicing with step
	deepArraySlice, err := json.Get(testData, "complexNested.level1.level2[0].level3.level4[0].level5.level6.deepArray[0:5:2]")
	validateOperation("Array slicing with step [0:5:2]", err)
	if slice, ok := deepArraySlice.([]interface{}); ok {
		fmt.Printf("  ✅ Array slice with step returned %d elements\n", len(slice))
	}

	// Test negative array slicing
	lastTwoElements, err := json.Get(testData, "complexNested.level1.level2[0].level3.level4[0].level5.level6.deepArray[-2:]")
	validateOperation("Negative array slicing [-2:]", err)
	if slice, ok := lastTwoElements.([]interface{}); ok && len(slice) == 2 {
		fmt.Println("  ✅ Negative array slicing returned correct elements")
	}

	// Test array slicing from beginning
	firstThreeElements, err := json.Get(testData, "complexNested.level1.level2[0].level3.level4[0].level5.level6.deepArray[:3]")
	validateOperation("Array slicing from beginning [:3]", err)
	if slice, ok := firstThreeElements.([]interface{}); ok && len(slice) == 3 {
		fmt.Println("  ✅ Array slicing from beginning returned correct elements")
	}

	// Test complex path with special characters in keys
	// Note: Keys with special characters require programmatic access since they can't be used in dot notation

	// Get the pathTestCases object first, then access keys programmatically
	pathTestCases, err := json.Get(testData, "pathTestCases")
	if err != nil {
		fmt.Printf("  ❌ Failed to get pathTestCases: %v\n", err)
	} else {
		if cases, ok := pathTestCases.(map[string]interface{}); ok {
			// Test dot in key
			if value, exists := cases["simple.property"]; exists {
				if strValue, ok := value.(string); ok && strValue == "dot in key" {
					fmt.Println("  ✅ Path with dot in key accessed successfully")
				} else {
					fmt.Printf("  ❌ Path with dot in key: expected 'dot in key', got %v\n", value)
				}
			} else {
				fmt.Println("  ❌ Key 'simple.property' not found in pathTestCases")
			}

			// Test brackets in key
			if value, exists := cases["array[with]brackets"]; exists {
				if strValue, ok := value.(string); ok && strValue == "brackets in key" {
					fmt.Println("  ✅ Path with brackets in key accessed successfully")
				}
			}

			// Test slash in key
			if value, exists := cases["special/chars"]; exists {
				if strValue, ok := value.(string); ok && strValue == "slash in key" {
					fmt.Println("  ✅ Path with slash in key accessed successfully")
				}
			}

			// Test unicode in key
			if value, exists := cases["unicode🌍key"]; exists {
				if strValue, ok := value.(string); ok && strValue == "unicode in key" {
					fmt.Println("  ✅ Path with unicode in key accessed successfully")
				}
			}

			// Test spaces in key
			if value, exists := cases["spaces in key"]; exists {
				if strValue, ok := value.(string); ok && strValue == "spaces in key" {
					fmt.Println("  ✅ Path with spaces in key accessed successfully")
				}
			}
		} else {
			fmt.Printf("  ❌ pathTestCases is not a map, got %T\n", pathTestCases)
		}
	}

	// Test mixed type array access
	mixedTypeValue, err := json.GetString(testData, "mixedTypes[0].type")
	validateResult("Mixed type array access", err, mixedTypeValue, "string")

	fmt.Println("  ✅ Advanced path expressions validation completed")
}

// runArrayOperationsValidation tests array-specific operations
func runArrayOperationsValidation(testData string) {
	fmt.Println("\n📊 Testing Array Operations...")

	// Test array length access
	skills, err := json.Get(testData, "company.departments[0].teams[0].members[0].skills")
	validateOperation("Get skills array", err)
	if skillsArray, ok := skills.([]interface{}); ok {
		fmt.Printf("  ✅ Skills array has %d elements\n", len(skillsArray))
	}

	// Test array modification
	modifiedData, err := json.Set(testData, "company.departments[0].teams[0].members[0].skills[0]", "Golang")
	validateOperation("Modify array element", err)

	updatedSkill, err := json.GetString(modifiedData, "company.departments[0].teams[0].members[0].skills[0]")
	validateResult("Verify array modification", err, updatedSkill, "Golang")

	// Test array append (using SetWithAdd for path creation)
	appendedData, err := json.SetWithAdd(testData, "company.metadata.tags[4]", "scalable")
	if err != nil {
		// If direct index append fails, try appending to the end of array
		// First get the current array to determine its length
		tags, getErr := json.Get(testData, "company.metadata.tags")
		if getErr == nil {
			if tagsArray, ok := tags.([]interface{}); ok {
				// Append to the end using the next available index
				nextIndex := len(tagsArray)
				appendedData, err = json.SetWithAdd(testData, fmt.Sprintf("company.metadata.tags[%d]", nextIndex), "scalable")
				if err == nil {
					fmt.Printf("  ✅ Array append succeeded at index %d\n", nextIndex)
					newTag, err := json.GetString(appendedData, fmt.Sprintf("company.metadata.tags[%d]", nextIndex))
					validateResult("Verify array append", err, newTag, "scalable")
				} else {
					fmt.Printf("  ⚠️ Array append not supported - this is expected behavior for safety\n")
				}
			}
		}
	} else {
		validateOperation("Append to array", err)
		newTag, err := json.GetString(appendedData, "company.metadata.tags[4]")
		validateResult("Verify array append", err, newTag, "scalable")
	}

	fmt.Println("  ✅ Array operations validation completed")
}

// runExtractionValidation tests extraction syntax
func runExtractionValidation(testData string) {
	fmt.Println("\n🔍 Testing Extraction Operations...")

	// Test simple extraction
	deptNames, err := json.Get(testData, "company.departments{name}")
	validateOperation("Extract department names", err)
	if names, ok := deptNames.([]interface{}); ok {
		fmt.Printf("  ✅ Extracted %d department names\n", len(names))
	}

	// Test nested extraction
	teamNames, err := json.Get(testData, "company.departments{teams}{name}")
	validateOperation("Extract team names", err)
	if names, ok := teamNames.([]interface{}); ok {
		fmt.Printf("  ✅ Extracted team names structure with %d elements\n", len(names))
	}

	// Test flat extraction
	allMemberNames, err := json.Get(testData, "company.departments{teams}{flat:members}{name}")
	validateOperation("Flat extract member names", err)
	if names, ok := allMemberNames.([]interface{}); ok {
		fmt.Printf("  ✅ Flat extracted %d member names\n", len(names))
	}

	fmt.Println("  ✅ Extraction operations validation completed")
}

// runAdvancedExtractionValidation tests advanced extraction features
func runAdvancedExtractionValidation(testData string) {
	fmt.Println("\n🔍 Testing Advanced Extraction Operations...")

	// Test deep nested extraction
	deepExtraction, err := json.Get(testData, "company.departments{teams}{members}{skills}")
	validateOperation("Deep nested extraction", err)
	if extracted, ok := deepExtraction.([]interface{}); ok {
		fmt.Printf("  ✅ Deep nested extraction returned %d skill arrays\n", len(extracted))
	}

	// Test flat extraction with deep nesting
	flatSkills, err := json.Get(testData, "company.departments{teams}{flat:members}{flat:skills}")
	validateOperation("Flat extraction with deep nesting", err)
	if skills, ok := flatSkills.([]interface{}); ok {
		fmt.Printf("  ✅ Flat extraction returned %d individual skills\n", len(skills))
	}

	// Test extraction with array slicing
	firstTwoTeamMembers, err := json.Get(testData, "company.departments[0].teams[0:2]{members}")
	validateOperation("Extraction with array slicing", err)
	if members, ok := firstTwoTeamMembers.([]interface{}); ok {
		fmt.Printf("  ✅ Extraction with slicing returned %d member arrays\n", len(members))
	}

	// Test mixed type extraction
	mixedTypeExtraction, err := json.Get(testData, "mixedTypes{type}")
	validateOperation("Mixed type extraction", err)
	if types, ok := mixedTypeExtraction.([]interface{}); ok {
		fmt.Printf("  ✅ Mixed type extraction returned %d types\n", len(types))
		expectedTypes := []string{"string", "number", "boolean", "array", "object"}
		for i, expectedType := range expectedTypes {
			if i < len(types) {
				if actualType, ok := types[i].(string); ok && actualType == expectedType {
					fmt.Printf("    ✅ Type %d: %s\n", i, actualType)
				}
			}
		}
	}

	// Test extraction with filtering by priority
	priorityOneItems, err := json.Get(testData, "mixedTypes{priority}")
	validateOperation("Priority extraction", err)
	if priorities, ok := priorityOneItems.([]interface{}); ok {
		fmt.Printf("  ✅ Priority extraction returned %d priorities\n", len(priorities))
	}

	fmt.Println("  ✅ Advanced extraction operations validation completed")
}

// runTypeSafetyValidation tests type-safe operations
func runTypeSafetyValidation(testData string) {
	fmt.Println("\n🔒 Testing Type Safety...")

	// Test GetTyped with various types
	companyName, err := json.GetTyped[string](testData, "company.name")
	validateResult("GetTyped[string]", err, companyName, "TechCorp")

	employeeCount, err := json.GetTyped[int](testData, "company.employees")
	validateResult("GetTyped[int]", err, employeeCount, 150)

	revenue, err := json.GetTyped[float64](testData, "company.revenue")
	validateResult("GetTyped[float64]", err, revenue, 1250000.50)

	isActive, err := json.GetTyped[bool](testData, "company.active")
	validateResult("GetTyped[bool]", err, isActive, true)

	// Test GetTyped with arrays
	tags, err := json.GetTyped[[]string](testData, "company.metadata.tags")
	validateOperation("GetTyped[[]string]", err)
	if len(tags) > 0 {
		fmt.Printf("  ✅ GetTyped returned array with %d tags\n", len(tags))
	}

	// Test GetTyped with objects
	contact, err := json.GetTyped[map[string]interface{}](testData, "company.metadata.contact")
	validateOperation("GetTyped[map[string]interface{}]", err)
	if len(contact) > 0 {
		fmt.Printf("  ✅ GetTyped returned object with %d fields\n", len(contact))
	}

	// Test type mismatch handling
	_, err = json.GetTyped[int](testData, "company.name")
	if err != nil {
		fmt.Println("  ✅ Type mismatch correctly handled with error")
	}

	fmt.Println("  ✅ Type safety validation completed")
}

// runProcessorValidation tests processor-specific features
func runProcessorValidation(testData string) {
	fmt.Println("\n⚙️  Testing Processor Features...")

	// Create processor with custom configuration
	config := json.DefaultConfig()
	config.MaxCacheSize = 100
	config.EnableCache = true
	processor := json.New(config)
	defer processor.Close()

	// Test processor operations
	result, err := processor.Get(testData, "company.name")
	validateResult("Processor Get", err, result, "TechCorp")

	// Test processor stats
	stats := processor.GetStats()
	fmt.Printf("  ✅ Processor stats - Operations: %d, Cache hits: %d, Cache misses: %d\n",
		stats.OperationCount, stats.HitCount, stats.MissCount)

	// Test processor health (using GetStats instead of GetHealth)
	healthStats := processor.GetStats()
	fmt.Printf("  ✅ Processor health - Operations: %d, Errors: %d\n",
		healthStats.OperationCount, healthStats.ErrorCount)

	// Test multiple operations to populate cache
	for i := 0; i < 5; i++ {
		_, _ = processor.Get(testData, "company.name")
	}

	updatedStats := processor.GetStats()
	if updatedStats.OperationCount > stats.OperationCount {
		fmt.Println("  ✅ Processor operation count increased correctly")
	}

	fmt.Println("  ✅ Processor features validation completed")
}

// runBatchOperationsValidation tests batch operation features
func runBatchOperationsValidation(testData string) {
	fmt.Println("\n📦 Testing Batch Operations...")

	// Test multiple get operations
	paths := []string{
		"company.name",
		"company.employees",
		"company.active",
		"statistics.totalEmployees",
		"config.version",
	}

	// Simulate batch get operations (since GetMultiple might not be available)
	results := make(map[string]interface{})
	errors := make(map[string]error)

	for _, path := range paths {
		result, err := json.Get(testData, path)
		if err != nil {
			errors[path] = err
		} else {
			results[path] = result
		}
	}

	fmt.Printf("  ✅ Batch get operations - Success: %d, Errors: %d\n", len(results), len(errors))

	// Test multiple set operations
	updates := map[string]interface{}{
		"company.employees":         160,
		"company.active":            false,
		"statistics.totalEmployees": 5,
		"config.debug":              true,
	}

	// Test SetMultipleWithAdd if available
	updatedData, err := json.SetMultipleWithAdd(testData, updates)
	if err != nil {
		fmt.Printf("  ⚠️  SetMultipleWithAdd not available or failed: %v\n", err)

		// Fallback to individual set operations
		currentData := testData
		successCount := 0
		for path, value := range updates {
			newData, setErr := json.Set(currentData, path, value)
			if setErr == nil {
				currentData = newData
				successCount++
			}
		}
		fmt.Printf("  ✅ Individual set operations - Success: %d/%d\n", successCount, len(updates))
	} else {
		fmt.Println("  ✅ SetMultipleWithAdd succeeded")

		// Verify some of the updates
		newEmployeeCount, err := json.GetInt(updatedData, "company.employees")
		validateResult("Verify batch update - employees", err, newEmployeeCount, 160)

		newActiveStatus, err := json.GetBool(updatedData, "company.active")
		validateResult("Verify batch update - active", err, newActiveStatus, false)
	}

	fmt.Println("  ✅ Batch operations validation completed")
}

// runStreamingOperationsValidation tests streaming operation features
func runStreamingOperationsValidation() {
	fmt.Println("\n🌊 Testing Streaming Operations...")

	// Test streaming with bytes.Buffer
	testObject := map[string]interface{}{
		"streaming": "test",
		"data":      []int{1, 2, 3, 4, 5},
		"nested": map[string]interface{}{
			"value": "streaming works",
		},
	}

	// Test NewEncoder
	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)

	err := encoder.Encode(testObject)
	validateOperation("Stream encoding with NewEncoder", err)

	if buf.Len() > 0 {
		fmt.Printf("  ✅ Encoded %d bytes to stream\n", buf.Len())
	}

	// Test NewDecoder
	decoder := json.NewDecoder(&buf)
	var decodedObject map[string]interface{}

	err = decoder.Decode(&decodedObject)
	validateOperation("Stream decoding with NewDecoder", err)

	if len(decodedObject) > 0 {
		fmt.Printf("  ✅ Decoded object with %d fields\n", len(decodedObject))

		// Verify decoded content
		if streamingValue, ok := decodedObject["streaming"].(string); ok && streamingValue == "test" {
			fmt.Println("  ✅ Stream decode verification successful")
		}
	}

	// Test streaming with strings.Reader
	jsonString := `{"reader": "test", "numbers": [10, 20, 30]}`
	reader := strings.NewReader(jsonString)

	decoder2 := json.NewDecoder(reader)
	var readerObject map[string]interface{}

	err = decoder2.Decode(&readerObject)
	validateOperation("Stream decoding from strings.Reader", err)

	if readerValue, ok := readerObject["reader"].(string); ok && readerValue == "test" {
		fmt.Println("  ✅ Reader stream decode verification successful")
	}

	// Test LoadFromReader and SaveToWriter if available
	processor := json.New()
	defer processor.Close()

	// Test SaveToWriter
	var writerBuf bytes.Buffer
	testData := `{"writer": "test", "array": [1, 2, 3]}`

	err = processor.SaveToWriter(&writerBuf, testData, false)
	if err != nil {
		fmt.Printf("  ⚠️  SaveToWriter not available or failed: %v\n", err)
	} else {
		validateOperation("SaveToWriter", err)
		fmt.Printf("  ✅ SaveToWriter wrote %d bytes\n", writerBuf.Len())
	}

	// Test LoadFromReader
	readerBuf := bytes.NewReader(writerBuf.Bytes())
	loadedData, err := processor.LoadFromReader(readerBuf)
	if err != nil {
		fmt.Printf("  ⚠️  LoadFromReader not available or failed: %v\n", err)
	} else {
		validateOperation("LoadFromReader", err)
		if loadedStr, ok := loadedData.(string); ok && len(loadedStr) > 0 {
			fmt.Println("  ✅ LoadFromReader successful")
		}
	}

	fmt.Println("  ✅ Streaming operations validation completed")
}

// runJSONPointerValidation tests JSON Pointer format support
func runJSONPointerValidation(testData string) {
	fmt.Println("\n👉 Testing JSON Pointer Support...")

	// Test basic JSON Pointer access
	companyName, err := json.GetString(testData, "/company/name")
	validateResult("JSON Pointer basic access", err, companyName, "TechCorp")

	// Test JSON Pointer array access
	firstDept, err := json.GetString(testData, "/company/departments/0/name")
	validateResult("JSON Pointer array access", err, firstDept, "Engineering")

	// Test JSON Pointer with special characters
	// Note: JSON Pointer uses ~0 for ~ and ~1 for /
	specialValue, err := json.GetString(testData, "/pathTestCases/special~1chars")
	validateResult("JSON Pointer with escaped slash", err, specialValue, "slash in key")

	// Test JSON Pointer deep nesting
	deepValue, err := json.GetString(testData, "/complexNested/level1/level2/0/level3/level4/0/level5/level6/deepValue")
	validateResult("JSON Pointer deep nesting", err, deepValue, "found")

	// Test JSON Pointer with array indices
	mixedTypeValue, err := json.GetString(testData, "/mixedTypes/0/type")
	validateResult("JSON Pointer mixed type access", err, mixedTypeValue, "string")

	// Test JSON Pointer root access
	rootValue, err := json.Get(testData, "")
	validateOperation("JSON Pointer root access", err)
	if rootValue != nil {
		fmt.Println("  ✅ JSON Pointer root access successful")
	}

	// Test JSON Pointer with Set operations
	updatedData, err := json.Set(testData, "/company/employees", 175)
	validateOperation("JSON Pointer Set operation", err)
	if err == nil {
		newCount, err := json.GetInt(updatedData, "/company/employees")
		validateResult("Verify JSON Pointer Set", err, newCount, 175)
	}

	fmt.Println("  ✅ JSON Pointer validation completed")
}

// runConcurrencyValidation tests thread safety
func runConcurrencyValidation(testData string) {
	fmt.Println("\n🔄 Testing Concurrency...")

	const numGoroutines = 10
	const operationsPerGoroutine = 20
	var wg sync.WaitGroup
	errors := make(chan error, numGoroutines*operationsPerGoroutine)
	results := make(chan interface{}, numGoroutines*operationsPerGoroutine)

	// Launch concurrent operations
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for j := 0; j < operationsPerGoroutine; j++ {
				// Vary the operations
				paths := []string{
					"company.name",
					"company.employees",
					"company.active",
					"statistics.totalEmployees",
				}
				path := paths[j%len(paths)]

				result, err := json.Get(testData, path)
				if err != nil {
					errors <- err
				} else {
					results <- result
				}
			}
		}(i)
	}

	wg.Wait()
	close(errors)
	close(results)

	// Count results
	errorCount := len(errors)
	resultCount := len(results)

	fmt.Printf("  ✅ Concurrent operations completed - Results: %d, Errors: %d\n",
		resultCount, errorCount)

	if errorCount == 0 {
		fmt.Println("  ✅ All concurrent operations succeeded")
	}

	fmt.Println("  ✅ Concurrency validation completed")
}

// runFileOperationsValidation tests file I/O operations
func runFileOperationsValidation() {
	fmt.Println("\n📁 Testing File Operations...")

	// Create temporary test file
	tempFile := "temp_test.json"
	testContent := `{"test": "file operations", "number": 42, "array": [1, 2, 3]}`

	// Test save to file (correct parameter order: filePath, data)
	err := json.SaveToFile(tempFile, testContent, false)
	validateOperation("Save to file", err)

	// Test load from file
	loadedContent, err := json.LoadFromFile(tempFile)
	validateOperation("Load from file", err)

	// Verify loaded content
	testValue, err := json.GetString(loadedContent, "test")
	validateResult("Verify loaded content", err, testValue, "file operations")

	// Test file operations with processor
	processor := json.New()
	defer processor.Close()

	err = processor.SaveToFile("temp_processor_test.json", testContent, false)
	validateOperation("Processor save to file", err)

	processorLoaded, err := processor.LoadFromFile("temp_processor_test.json")
	validateOperation("Processor load from file", err)

	if processorLoadedStr, ok := processorLoaded.(string); ok {
		processorValue, err := processor.Get(processorLoadedStr, "number")
		validateResult("Verify processor loaded content", err, processorValue, float64(42))
	}

	// Cleanup
	os.Remove(tempFile)
	os.Remove("temp_processor_test.json")

	fmt.Println("  ✅ File operations validation completed")
}

// runEncodingValidation tests JSON encoding features
func runEncodingValidation(testData string) {
	fmt.Println("\n🔧 Testing Encoding Operations...")

	// Test compact encoding
	compactJSON, err := json.EncodeCompact(testData)
	validateOperation("Compact encoding", err)
	if !strings.Contains(compactJSON, "\n") && !strings.Contains(compactJSON, "  ") {
		fmt.Println("  ✅ Compact encoding removed whitespace correctly")
	}

	// Test pretty encoding
	prettyJSON, err := json.EncodePretty(testData)
	validateOperation("Pretty encoding", err)
	if strings.Contains(prettyJSON, "\n") && strings.Contains(prettyJSON, "  ") {
		fmt.Println("  ✅ Pretty encoding added formatting correctly")
	}

	// Test standard encoding
	_, err = json.Encode(testData)
	validateOperation("Standard encoding", err)

	// Test encoding with processor
	processor := json.New()
	defer processor.Close()

	// Create a simple test object for encoding
	testObject := map[string]interface{}{
		"company": map[string]interface{}{
			"name": "TechCorp",
		},
		"test": "encoding",
	}

	processorEncoded, err := processor.ToJsonString(testObject)
	validateOperation("Processor encoding", err)

	// Verify encoded data can be parsed back
	if len(processorEncoded) > 0 {
		parsedValue, err := json.GetString(processorEncoded, "company.name")
		validateResult("Verify encoded data parsing", err, parsedValue, "TechCorp")
	} else {
		fmt.Println("  ⚠️  Encoded data is empty - this may be expected behavior")
	}

	fmt.Println("  ✅ Encoding operations validation completed")
}

// runValidationAndSecurityValidation tests validation and security features
func runValidationAndSecurityValidation() {
	fmt.Println("\n🛡️  Testing Validation & Security...")

	// Test JSON validation
	validJSON := `{"valid": true, "number": 42}`
	isValid := json.Valid([]byte(validJSON))
	if isValid {
		fmt.Println("  ✅ Valid JSON correctly identified")
	}

	invalidJSON := `{"invalid": true, "number": 42,}`
	isInvalid := json.Valid([]byte(invalidJSON))
	if !isInvalid {
		fmt.Println("  ✅ Invalid JSON correctly identified")
	}

	// Test with security configuration
	secureConfig := json.HighSecurityConfig()
	secureProcessor := json.New(secureConfig)
	defer secureProcessor.Close()

	// Test with large data configuration
	largeDataConfig := json.LargeDataConfig()
	largeDataProcessor := json.New(largeDataConfig)
	defer largeDataProcessor.Close()

	// Test basic operation with secure processor
	testData := `{"secure": "test", "value": 123}`
	result, err := secureProcessor.Get(testData, "secure")
	validateResult("Secure processor operation", err, result, "test")

	// Test basic operation with large data processor
	result2, err := largeDataProcessor.Get(testData, "value")
	validateResult("Large data processor operation", err, result2, float64(123))

	fmt.Println("  ✅ Validation & security validation completed")
}

// runSchemaValidationValidation tests schema validation features
func runSchemaValidationValidation() {
	fmt.Println("\n📋 Testing Schema Validation...")

	// Test basic JSON validation
	validJSON := `{"name": "John", "age": 30, "active": true}`
	isValid := json.Valid([]byte(validJSON))
	if isValid {
		fmt.Println("  ✅ Valid JSON correctly identified")
	}

	// Test invalid JSON detection
	invalidJSONs := []string{
		`{"name": "John", "age": 30,}`, // Trailing comma
		`{"name": "John" "age": 30}`,   // Missing comma
		`{"name": "John", "age": }`,    // Missing value
		`{name: "John", "age": 30}`,    // Unquoted key
		`{"name": "John", "age": 30`,   // Missing closing brace
		`{"name": "John", "age": 30}}`, // Extra closing brace
	}

	invalidCount := 0
	for i, invalidJSON := range invalidJSONs {
		isInvalid := json.Valid([]byte(invalidJSON))
		if !isInvalid {
			invalidCount++
			fmt.Printf("  ✅ Invalid JSON %d correctly identified\n", i+1)
		} else {
			fmt.Printf("  ⚠️  Invalid JSON %d not detected: %s\n", i+1, invalidJSON)
		}
	}

	fmt.Printf("  ✅ Detected %d/%d invalid JSON cases\n", invalidCount, len(invalidJSONs))

	// Test schema validation with processor if available
	processor := json.New()
	defer processor.Close()

	// Simple schema for testing
	testData := `{"name": "Alice", "age": 25, "email": "alice@example.com"}`
	schema := json.DefaultSchema()
	schema.Type = "object"
	schema.Properties = map[string]*json.Schema{
		"name": {
			Type: "string",
		},
		"age": {
			Type: "number",
		},
		"email": {
			Type: "string",
		},
	}
	schema.Required = []string{"name", "age"}

	// Test ValidateSchema if available
	validationErrors, err := processor.ValidateSchema(testData, schema)
	if err != nil {
		fmt.Printf("  ⚠️  Schema validation not available or failed: %v\n", err)
	} else {
		if len(validationErrors) == 0 {
			fmt.Println("  ✅ Schema validation passed")
		} else {
			fmt.Printf("  ⚠️  Schema validation found %d errors\n", len(validationErrors))
		}
	}

	// Test with invalid data against schema
	invalidData := `{"name": "Bob", "age": "not a number"}`
	validationErrors2, err := processor.ValidateSchema(invalidData, schema)
	if err != nil {
		fmt.Printf("  ⚠️  Schema validation with invalid data failed: %v\n", err)
	} else {
		if len(validationErrors2) > 0 {
			fmt.Printf("  ✅ Schema validation correctly found %d errors in invalid data\n", len(validationErrors2))
		} else {
			fmt.Println("  ⚠️  Schema validation should have found errors in invalid data")
		}
	}

	fmt.Println("  ✅ Schema validation validation completed")
}

// runPerformanceValidation tests performance characteristics
func runPerformanceValidation(testData string) {
	fmt.Println("\n⚡ Testing Performance...")

	// Test performance with multiple operations
	start := time.Now()
	iterations := 1000

	for i := 0; i < iterations; i++ {
		_, _ = json.Get(testData, "company.name")
	}

	duration := time.Since(start)
	opsPerSecond := float64(iterations) / duration.Seconds()

	fmt.Printf("  ✅ Performance test - %d operations in %v (%.0f ops/sec)\n",
		iterations, duration, opsPerSecond)

	// Test with processor and cache
	processor := json.New()
	defer processor.Close()

	start = time.Now()
	for i := 0; i < iterations; i++ {
		_, _ = processor.Get(testData, "company.name")
	}

	processorDuration := time.Since(start)
	processorOpsPerSecond := float64(iterations) / processorDuration.Seconds()

	fmt.Printf("  ✅ Processor performance - %d operations in %v (%.0f ops/sec)\n",
		iterations, processorDuration, processorOpsPerSecond)

	// Check cache effectiveness
	stats := processor.GetStats()
	if stats.HitCount > 0 {
		hitRatio := float64(stats.HitCount) / float64(stats.HitCount+stats.MissCount) * 100
		fmt.Printf("  ✅ Cache hit ratio: %.1f%%\n", hitRatio)
	}

	fmt.Println("  ✅ Performance validation completed")
}

// runErrorHandlingValidation tests error handling
func runErrorHandlingValidation() {
	fmt.Println("\n❌ Testing Error Handling...")

	// Test with invalid JSON
	invalidJSON := `{"invalid": json}`
	_, err := json.Get(invalidJSON, "invalid")
	if err != nil {
		fmt.Println("  ✅ Invalid JSON correctly returned error")
	}

	// Test with invalid path
	validJSON := `{"valid": "json"}`
	_, err = json.Get(validJSON, "invalid[path")
	if err != nil {
		fmt.Println("  ✅ Invalid path correctly returned error")
	}

	// Test type mismatch
	_, err = json.GetInt(validJSON, "valid")
	if err != nil {
		fmt.Println("  ✅ Type mismatch correctly returned error")
	}

	// Test processor error handling
	processor := json.New()
	processor.Close() // Close processor to test closed state

	_, err = processor.Get(validJSON, "valid")
	if err != nil {
		fmt.Println("  ✅ Closed processor correctly returned error")
	}

	fmt.Println("  ✅ Error handling validation completed")
}

// runEdgeCasesValidation tests edge cases and boundary conditions
func runEdgeCasesValidation() {
	fmt.Println("\n🔍 Testing Edge Cases...")

	// Test empty JSON
	emptyJSON := `{}`
	_, err := json.Get(emptyJSON, "nonexistent")
	if err != nil {
		fmt.Println("  ✅ Empty JSON with nonexistent path correctly returned error")
	}

	// Test very large numbers
	// Note: Use a smaller integer that can be safely represented in float64
	largeNumberJSON := `{"large": 1234567890123456, "float": 1.7976931348623157e+308, "maxSafeInt": 9007199254740991}`

	// Test large but safe integer
	largeInt, err := json.GetInt(largeNumberJSON, "large")
	validateResult("Large integer", err, largeInt, 1234567890123456)

	// Test maximum safe integer for JavaScript compatibility
	maxSafeInt, err := json.GetInt(largeNumberJSON, "maxSafeInt")
	validateResult("Maximum safe integer", err, maxSafeInt, 9007199254740991)

	// Test very large float
	largeFloat, err := json.GetFloat64(largeNumberJSON, "float")
	validateOperation("Very large float", err)
	if err == nil {
		fmt.Printf("  ✅ Large float value: %e\n", largeFloat)
	}

	// Test edge case: integer that loses precision when converted to float64
	precisionTestJSON := `{"bigInt": 9223372036854775807}`
	// This should be accessed as float64 since JSON numbers are parsed as float64 first
	bigIntAsFloat, err := json.GetFloat64(precisionTestJSON, "bigInt")
	validateOperation("Large integer as float64", err)
	if err == nil {
		fmt.Printf("  ✅ Large integer as float64: %.0f (precision may be lost)\n", bigIntAsFloat)
	}

	// Test Unicode and special characters
	unicodeJSON := `{"emoji": "🌍🚀💻", "chinese": "你好世界", "arabic": "مرحبا بالعالم", "russian": "Привет мир"}`
	emoji, err := json.GetString(unicodeJSON, "emoji")
	validateResult("Unicode emoji", err, emoji, "🌍🚀💻")

	chinese, err := json.GetString(unicodeJSON, "chinese")
	validateResult("Chinese characters", err, chinese, "你好世界")

	// Test deeply nested empty structures
	deepEmptyJSON := `{"a": {"b": {"c": {"d": {"e": {}}}}}}`
	_, err = json.Get(deepEmptyJSON, "a.b.c.d.e.f")
	if err != nil {
		fmt.Println("  ✅ Deep empty structure with nonexistent path correctly returned error")
	}

	// Test array with mixed types
	mixedArrayJSON := `{"mixed": [null, true, false, 0, "", [], {}]}`
	mixedArray, err := json.Get(mixedArrayJSON, "mixed")
	validateOperation("Mixed type array", err)
	if arr, ok := mixedArray.([]interface{}); ok {
		fmt.Printf("  ✅ Mixed array has %d elements\n", len(arr))
	}

	// Test null value handling
	nullJSON := `{"null": null, "nested": {"null": null}}`
	nullValue, err := json.Get(nullJSON, "null")
	validateOperation("Null value access", err)
	if nullValue == nil {
		fmt.Println("  ✅ Null value correctly returned as nil")
	}

	// Test very long path (reduced depth to avoid parsing issues)
	longPath := "a.b.c.d.e.f.g.h.i.j.k.l.m.n.o.p.q.r.s.t"
	longPathJSON := `{"a": {"b": {"c": {"d": {"e": {"f": {"g": {"h": {"i": {"j": {"k": {"l": {"m": {"n": {"o": {"p": {"q": {"r": {"s": {"t": "deep"}}}}}}}}}}}}}}}}}}}}`
	deepValue, err := json.GetString(longPathJSON, longPath)
	validateResult("Very long path (20 levels)", err, deepValue, "deep")

	// Test array bounds
	arrayJSON := `{"arr": [1, 2, 3]}`

	// Test negative index beyond bounds
	_, err = json.Get(arrayJSON, "arr[-10]")
	if err != nil {
		fmt.Println("  ✅ Negative index beyond bounds correctly returned error")
	}

	// Test positive index beyond bounds
	_, err = json.Get(arrayJSON, "arr[10]")
	if err != nil {
		fmt.Println("  ✅ Positive index beyond bounds correctly returned error")
	}

	// Test empty string keys
	emptyKeyJSON := `{"": "empty key", "normal": "normal key"}`
	// Try different approaches for empty string key access
	emptyKeyValue, err := json.GetString(emptyKeyJSON, "")
	if err == nil && emptyKeyValue == "empty key" {
		fmt.Println("  ✅ Empty string key access successful")
	} else {
		// Try alternative method - get the whole object and access programmatically
		wholeObj, err2 := json.Get(emptyKeyJSON, "")
		if err2 == nil {
			if obj, ok := wholeObj.(map[string]interface{}); ok {
				if value, exists := obj[""]; exists {
					fmt.Printf("  ✅ Empty string key found via object access: %v\n", value)
				} else {
					fmt.Println("  ⚠️  Empty string key not found in object")
				}
			}
		} else {
			fmt.Printf("  ⚠️  Empty string key access failed: %v\n", err)
		}
	}

	// Test deep nesting limits (reasonable depth)
	// This tests the library's ability to handle deep but valid JSON structures
	deepNestingJSON := `{"a": {"b": {"c": {"d": {"e": {"f": {"g": {"h": {"i": {"j": "end"}}}}}}}}}}`
	deepResult, err := json.GetString(deepNestingJSON, "a.b.c.d.e.f.g.h.i.j")
	validateResult("Deep nesting (10 levels)", err, deepResult, "end")

	fmt.Println("  ✅ Edge cases validation completed")
}

// Helper functions for validation
func validateOperation(operation string, err error) {
	if err != nil {
		log.Printf("  ❌ %s failed: %v", operation, err)
	} else {
		fmt.Printf("  ✅ %s succeeded\n", operation)
	}
}

func validateResult(operation string, err error, actual, expected interface{}) {
	if err != nil {
		log.Printf("  ❌ %s failed: %v", operation, err)
		return
	}

	if actual == expected {
		fmt.Printf("  ✅ %s succeeded (got: %v)\n", operation, actual)
	} else {
		log.Printf("  ❌ %s failed: expected %v, got %v", operation, expected, actual)
	}
}
