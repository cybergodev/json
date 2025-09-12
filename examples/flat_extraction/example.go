package main

// ==================================================================================
// This is a comprehensive example of flat extraction and array operations
//
// This example demonstrates the difference between regular extraction and flat extraction
// when combined with array operations:
// 1. Regular extraction - returns nested arrays that preserve structure
// 2. Flat extraction - returns flattened single arrays for simplified access
// 3. Distributed array operations - applies operations to each extracted array
// 4. Flat array operations - applies operations to the flattened result
// 5. Complex nested scenarios with multiple extraction levels
// 6. Performance comparisons between different approaches
// 7. Set operations on both regular and flat extractions
// 8. Edge cases and error handling
//
// Key concepts:
// - {field} vs {flat:field} extraction behavior
// - Distributed operations: {extraction}[operation] on multiple arrays
// - Flat operations: {flat:extraction}[operation] on single flattened array
// - When to use each approach for different use cases
// ==================================================================================

import (
	"fmt"
	"log"
	"strings"

	"github.com/cybergodev/json"
)

// Sample JSON data with nested arrays for demonstration
const companyData = `{
	"company": {
		"name": "TechCorp",
		"departments": [{
			"name": "Engineering",
			"teams": [{
				"name": "Backend",
				"members": [
					{"name": "Alice", "skills": ["Go", "Python", "Docker"], "level": "Senior", "years": 5},
					{"name": "Bob", "skills": ["Java", "Spring", "Kubernetes"], "level": "Junior", "years": 2}
				]
			}, {
				"name": "Frontend", 
				"members": [
					{"name": "Carol", "skills": ["React", "TypeScript", "CSS"], "level": "Mid", "years": 3},
					{"name": "Dave", "skills": ["Vue", "JavaScript", "HTML"], "level": "Senior", "years": 4}
				]
			}]
		}, {
			"name": "Marketing",
			"teams": [{
				"name": "Digital",
				"members": [
					{"name": "Eve", "skills": ["SEO", "Analytics", "Content"], "level": "Senior", "years": 6},
					{"name": "Frank", "skills": ["PPC", "Social Media", "Design"], "level": "Mid", "years": 3}
				]
			}]
		}]
	}
}`

// Sample data with simpler structure for basic examples
const simpleData = `{
	"products": [
		{"name": "Laptop", "tags": ["electronics", "computers"], "prices": [999, 1299, 1599]},
		{"name": "Phone", "tags": ["electronics", "mobile"], "prices": [699, 899, 1199]},
		{"name": "Tablet", "tags": ["electronics", "portable"], "prices": [399, 599, 799]}
	]
}`

func main() {
	// Basic flat extraction examples
	printSection("Basic Flat Extraction Examples")
	basicFlatExtraction()

	// Flat vs regular extraction comparison
	printSection("Flat vs Regular Extraction Comparison")
	flatVsRegularComparison()

	// Array operations on flat extractions
	printSection("Array Operations on Flat Extractions")
	flatArrayOperations()

	// Distributed array operations
	printSection("Distributed Array Operations")
	distributedArrayOperations()

	// Complex nested scenarios
	printSection("Complex Nested Scenarios")
	complexNestedScenarios()

	// Set operations examples
	printSection("Set Operations Examples")
	setOperationsExamples()

	// Performance and use case guidance
	printSection("Performance and Use Case Guidance")
	performanceGuidance()
}

// Basic flat extraction examples
func basicFlatExtraction() {
	fmt.Println("\nBasic flat extraction examples:")

	// Regular extraction - preserves structure
	fmt.Println("1. Regular extraction (preserves structure):")
	regularTags, err := json.Get(simpleData, "products{tags}")
	if err != nil {
		log.Println("Error getting regular tags:", err)
	} else {
		fmt.Printf("   products{tags}: %v\n", regularTags)
		// Result: [[electronics computers] [electronics mobile] [electronics portable]]
	}

	// Flat extraction - flattens into single array
	fmt.Println("\n2. Flat extraction (flattens into single array):")
	flatTags, err := json.Get(simpleData, "products{flat:tags}")
	if err != nil {
		log.Println("Error getting flat tags:", err)
	} else {
		fmt.Printf("   products{flat:tags}: %v\n", flatTags)
		// Result: [electronics computers electronics mobile electronics portable]
	}

	// Regular extraction with prices
	fmt.Println("\n3. Regular vs flat extraction with numeric arrays:")
	regularPrices, err := json.Get(simpleData, "products{prices}")
	if err != nil {
		log.Println("Error getting regular prices:", err)
	} else {
		fmt.Printf("   products{prices}: %v\n", regularPrices)
		// Result: [[999 1299 1599] [699 899 1199] [399 599 799]]
	}

	flatPrices, err := json.Get(simpleData, "products{flat:prices}")
	if err != nil {
		log.Println("Error getting flat prices:", err)
	} else {
		fmt.Printf("   products{flat:prices}: %v\n", flatPrices)
		// Result: [999 1299 1599 699 899 1199 399 599 799]
	}
}

// Compare flat vs regular extraction behavior
func flatVsRegularComparison() {
	fmt.Println("\nFlat vs regular extraction comparison:")

	// Complex nested extraction
	fmt.Println("1. Complex nested extraction:")

	// Regular extraction - nested structure preserved
	regularSkills, err := json.Get(companyData, "company.departments{teams}{members}{skills}")
	if err != nil {
		log.Println("Error getting regular skills:", err)
	} else {
		fmt.Printf("   Regular {skills}: %v\n", regularSkills)
		// Result: [[[[Go Python Docker] [Java Spring Kubernetes]] [[React TypeScript CSS] [Vue JavaScript HTML]]] [[[SEO Analytics Content] [PPC Social Media Design]]]]
	}

	// Flat extraction - completely flattened
	flatSkills, err := json.Get(companyData, "company.departments{flat:teams}{flat:members}{flat:skills}")
	if err != nil {
		log.Println("Error getting flat skills:", err)
	} else {
		fmt.Printf("   Flat {flat:skills}: %v\n", flatSkills)
		// Result: [Go Python Docker Java Spring Kubernetes React TypeScript CSS Vue JavaScript HTML SEO Analytics Content PPC Social Media Design]
	}

	// Show structure differences
	fmt.Println("\n2. Structure analysis:")
	if regularArr, ok := regularSkills.([]any); ok {
		fmt.Printf("   Regular extraction: %d departments\n", len(regularArr)) // 2
		if dept0, ok := regularArr[0].([]any); ok {
			fmt.Printf("   First department: %d teams\n", len(dept0)) // 2
			if team0, ok := dept0[0].([]any); ok {
				fmt.Printf("   First team: %d members\n", len(team0)) // 2
			}
		}
	}

	if flatArr, ok := flatSkills.([]any); ok {
		fmt.Printf("   Flat extraction: %d total skills\n", len(flatArr)) // 18
	}
}

// Array operations on flat extractions
func flatArrayOperations() {
	fmt.Println("\nArray operations on flat extractions:")

	// Basic array operations on flat extractions
	fmt.Println("1. Basic array operations on flat extractions:")

	// Get first element from flattened array
	firstSkill, err := json.Get(companyData, "company.departments{teams}{members}{flat:skills}[0]")
	if err != nil {
		log.Println("Error getting first skill:", err)
	} else {
		fmt.Printf("   First skill {flat:skills}[0]: %v\n", firstSkill)
		// Result: "Go"
	}

	// Get last element from flattened array
	lastSkill, err := json.Get(companyData, "company.departments{teams}{members}{flat:skills}[-1]")
	if err != nil {
		log.Println("Error getting last skill:", err)
	} else {
		fmt.Printf("   Last skill {flat:skills}[-1]: %v\n", lastSkill)
		// Result: "Design"
	}

	// Get slice from flattened array
	middleSkills, err := json.Get(companyData, "company.departments{teams}{members}{flat:skills}[3:8]")
	if err != nil {
		log.Println("Error getting middle skills:", err)
	} else {
		fmt.Printf("   Middle skills {flat:skills}[3:8]: %v\n", middleSkills)
		// Result: [Java Spring Kubernetes React TypeScript]
	}

	// Compare with simple data
	fmt.Println("\n2. Simple data flat operations:")

	firstTag, err := json.Get(simpleData, "products{flat:tags}[0]")
	if err != nil {
		log.Println("Error getting first tag:", err)
	} else {
		fmt.Printf("   First tag {flat:tags}[0]: %v\n", firstTag)
		// Result: "electronics"
	}

	uniqueTags, err := json.Get(simpleData, "products{flat:tags}[0:3]")
	if err != nil {
		log.Println("Error getting unique tags:", err)
	} else {
		fmt.Printf("   First 3 tags {flat:tags}[0:3]: %v\n", uniqueTags)
		// Result: [electronics computers electronics]
	}
}

// Distributed array operations (non-flat)
func distributedArrayOperations() {
	fmt.Println("\nDistributed array operations (non-flat):")

	// Distributed operations apply to each individual array
	fmt.Println("1. Distributed operations on skills arrays:")

	// Get first skill from each person
	firstFromEach, err := json.Get(companyData, "company.departments{teams}{members}{skills}[0]")
	if err != nil {
		log.Println("Error getting first from each:", err)
	} else {
		fmt.Printf("   First skill from each person {skills}[0]: %v\n", firstFromEach)
		// Result: [Go Java React Vue SEO PPC]
	}

	// Get last skill from each person
	lastFromEach, err := json.Get(companyData, "company.departments{teams}{members}{skills}[-1]")
	if err != nil {
		log.Println("Error getting last from each:", err)
	} else {
		fmt.Printf("   Last skill from each person {skills}[-1]: %v\n", lastFromEach)
		// Result: [Docker Kubernetes CSS HTML Content Design]
	}

	// Get skill slices from each person
	slicesFromEach, err := json.Get(companyData, "company.departments{teams}{members}{skills}[0:2]")
	if err != nil {
		log.Println("Error getting slices from each:", err)
	} else {
		fmt.Printf("   First 2 skills from each person {skills}[0:2]: %v\n", slicesFromEach)
		// Result: [[[[Go Python Docker] [Java Spring Kubernetes]] [[React TypeScript CSS] [Vue JavaScript HTML]]] [[[SEO Analytics Content] [PPC Social Media Design]]]]
	}

	// Compare with simple data
	fmt.Println("\n2. Distributed operations on simple data:")

	firstPriceFromEach, err := json.Get(simpleData, "products{prices}[0]")
	if err != nil {
		log.Println("Error getting first price from each:", err)
	} else {
		fmt.Printf("   First price from each product {prices}[0]: %v\n", firstPriceFromEach)
		// Result: [999 699 399]
	}
}

// Complex nested scenarios
func complexNestedScenarios() {
	fmt.Println("\nComplex nested scenarios:")

	// Multi-level extraction with flat
	fmt.Println("1. Multi-level extraction scenarios:")

	// Extract all member names (regular)
	memberNames, err := json.Get(companyData, "company.departments{teams}{members}{name}")
	if err != nil {
		log.Println("Error getting member names:", err)
	} else {
		fmt.Printf("   Member names {name}: %v\n", memberNames)
		// Result: [[[Alice Bob] [Carol Dave]] [[Eve Frank]]]
	}

	// Extract all member names (flat)
	flatMemberNames, err := json.Get(companyData, "company.departments{flat:teams}{flat:members}{flat:name}")
	if err != nil {
		log.Println("Error getting flat member names:", err)
	} else {
		fmt.Printf("   Flat member names {flat:name}: %v\n", flatMemberNames)
		// Result: [Alice Bob Carol Dave Eve Frank]
	}

	// Complex array operations
	fmt.Println("\n2. Complex array operations:")

	// Get first member from each team (distributed)
	firstMembers, err := json.Get(companyData, "company.departments{teams}{members}[0]")
	if err != nil {
		log.Println("Error getting first members:", err)
	} else {
		fmt.Printf("   First member from each team {members}[0]: %v\n", firstMembers)
		// Result: First member object from each team
	}

	// Get first name from flattened list
	firstFlatName, err := json.Get(companyData, "company.departments{teams}{members}{flat:name}[0]")
	if err != nil {
		log.Println("Error getting first flat name:", err)
	} else {
		fmt.Printf("   First name from flat list {flat:name}[0]: %v\n", firstFlatName)
		// Result: "Alice"
	}

	// Get last 3 names from flattened list
	lastThreeNames, err := json.Get(companyData, "company.departments{teams}{members}{flat:name}[-3:]")
	if err != nil {
		log.Println("Error getting last three names:", err)
	} else {
		fmt.Printf("   Last 3 names {flat:name}[-3:]: %v\n", lastThreeNames)
		// Result: [Dave Eve Frank]
	}
}

// Set operations examples
func setOperationsExamples() {
	fmt.Println("\nSet operations examples:")

	// Set operations on distributed arrays
	fmt.Println("1. Set operations on distributed arrays:")

	// Set first skill for each person
	setResult1, err := json.Set(companyData, "company.departments{teams}{members}{skills}[0]", "NewSkill")
	if err != nil {
		log.Println("Error setting distributed skills:", err)
	} else {
		// Verify the change
		modifiedSkills, _ := json.Get(setResult1, "company.departments{teams}{members}{skills}[0]")
		fmt.Printf("   After setting {skills}[0] = 'NewSkill': %v\n", modifiedSkills)
		// Result: [NewSkill NewSkill NewSkill NewSkill NewSkill NewSkill]
	}

	// Set operations on flat extractions
	fmt.Println("\n2. Set operations on flat extractions:")

	// Set first skill in flattened array
	setResult2, err := json.Set(companyData, "company.departments{teams}{members}{flat:skills}[0]", "FlatNewSkill")
	if err != nil {
		log.Println("Error setting flat skill:", err)
	} else {
		// Verify the change
		modifiedFlatSkill, _ := json.Get(setResult2, "company.departments{teams}{members}{flat:skills}[0]")
		fmt.Printf("   After setting {flat:skills}[0] = 'FlatNewSkill': %v\n", modifiedFlatSkill)
		// Result: "FlatNewSkill"

		// Check what happened to the original structure
		modifiedStructure, _ := json.Get(setResult2, "company.departments[0].teams[0].members[0].skills[0]")
		fmt.Printf("   Alice's first skill after flat set: %v\n", modifiedStructure)
		// Result: "FlatNewSkill"
	}

	// Set multiple values using array
	fmt.Println("\n3. Set multiple values using array:")

	setResult3, err := json.Set(companyData, "company.departments{teams}{members}[0]{skills}",
		[]string{"Rust", "TypeScript", "Swift", "Kotlin", "C++", "C#"})
	if err != nil {
		log.Println("Error setting array values:", err)
	} else {
		modifiedArraySkills, _ := json.Get(setResult3, "company.departments{teams}{members}[0]{skills}")
		fmt.Printf("   After setting array values: %v\n", modifiedArraySkills)
		// Result: [[Rust TypeScript Swift Kotlin C++ C# Rust TypeScript Swift Kotlin C++ C#] [Rust TypeScript Swift Kotlin C++ C#]]
	}
}

// Performance and use case guidance
func performanceGuidance() {
	fmt.Println("\nPerformance and use case guidance:")

	fmt.Println("1. When to use flat extraction:")
	fmt.Println("   ✅ When you need all values in a single array")
	fmt.Println("   ✅ When you want to apply array operations to the combined result")
	fmt.Println("   ✅ When you need to count total elements across all arrays")
	fmt.Println("   ✅ When you want to find unique values across all arrays")
	fmt.Println("   ✅ When you need simple iteration over all values")

	fmt.Println("\n2. When to use distributed operations:")
	fmt.Println("   ✅ When you need to apply the same operation to each individual array")
	fmt.Println("   ✅ When you want to preserve the relationship between arrays and their sources")
	fmt.Println("   ✅ When you need to modify specific positions in each array")
	fmt.Println("   ✅ When you want to get corresponding elements from multiple arrays")

	// Practical examples
	fmt.Println("\n3. Practical examples:")

	// Use case 1: Count total skills
	totalSkillsFlat, _ := json.Get(companyData, "company.departments{teams}{members}{flat:skills}")
	if flatArr, ok := totalSkillsFlat.([]any); ok {
		fmt.Printf("   Total skills count (using flat): %d\n", len(flatArr))
	}

	// Use case 2: Get primary skill for each person
	primarySkills, _ := json.Get(companyData, "company.departments{teams}{members}{skills}[0]")
	fmt.Printf("   Primary skills (using distributed): %v\n", primarySkills)

	// Use case 3: Get all unique skills
	allSkillsFlat, _ := json.Get(companyData, "company.departments{teams}{members}{flat:skills}")
	if flatArr, ok := allSkillsFlat.([]any); ok {
		uniqueSkills := make(map[string]bool)
		for _, skill := range flatArr {
			if skillStr, ok := skill.(string); ok {
				uniqueSkills[skillStr] = true
			}
		}
		fmt.Printf("   Unique skills count: %d\n", len(uniqueSkills))
	}

	fmt.Println("\n4. Performance tips:")
	fmt.Println("   💡 Use flat extraction for aggregation operations")
	fmt.Println("   💡 Use distributed operations for element-wise operations")
	fmt.Println("   💡 Flat extraction is more memory efficient for large datasets")
	fmt.Println("   💡 Distributed operations preserve data relationships")
}

var sectionLine = strings.Repeat("=", 60)

func printSection(title string) {
	fmt.Println()
	fmt.Println(sectionLine)
	fmt.Println(title)
	fmt.Println(sectionLine)
}
