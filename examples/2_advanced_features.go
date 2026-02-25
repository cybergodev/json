//go:build example

package main

import (
	"fmt"
	"log"

	"github.com/cybergodev/json"
)

// Advanced Features Example
//
// This example demonstrates advanced JSON operations for complex use cases.
// Ideal for developers who need to work with nested structures and bulk operations.
//
// Topics covered:
// - Complex path queries and nested extraction
// - Flat extraction from deeply nested structures
// - Iteration and transformation
// - Working with deeply nested data
// - Batch modifications
//
// For file I/O operations, see: 10_file_operations.go
// For iterator functions, see: 9_iterator_functions.go
//
// Run: go run examples/advanced_features.go

func main() {
	fmt.Println("Advanced Features - JSON Library")
	fmt.Println("=================================\n ")

	// Complex nested data structure
	complexData := `{
		"organization": "TechCorp",
		"departments": [
			{
				"name": "Engineering",
				"teams": [
					{
						"name": "Backend",
						"members": [
							{"name": "Alice", "role": "Lead", "skills": ["Go", "Python", "Docker"]},
							{"name": "Bob", "role": "Engineer", "skills": ["Java", "Kubernetes"]}
						]
					},
					{
						"name": "Frontend",
						"members": [
							{"name": "Carol", "role": "Lead", "skills": ["React", "TypeScript"]},
							{"name": "David", "role": "Engineer", "skills": ["Vue", "CSS"]}
						]
					}
				]
			},
			{
				"name": "Sales",
				"teams": [
					{
						"name": "Enterprise",
						"members": [
							{"name": "Eve", "role": "Manager", "skills": ["Negotiation", "CRM"]}
						]
					}
				]
			}
		],
		"metadata": {
			"created": "2024-01-01",
			"tags": ["tech", "startup", "innovation"]
		}
	}`

	// 1. COMPLEX PATH QUERIES
	fmt.Println("1. Complex Path Queries")
	fmt.Println("-----------------------")
	demonstrateComplexPaths(complexData)

	// 2. NESTED EXTRACTION
	fmt.Println("\n2. Nested Extraction")
	fmt.Println("--------------------")
	demonstrateExtraction(complexData)

	// 3. FLAT EXTRACTION
	fmt.Println("\n3. Flat Extraction")
	fmt.Println("------------------")
	demonstrateFlatExtraction(complexData)

	// 4. DEEP MODIFICATIONS
	fmt.Println("\n4. Deep Modifications")
	fmt.Println("---------------------")
	demonstrateDeepModifications(complexData)

	// 5. BATCH OPERATIONS
	fmt.Println("\n5. Batch Operations")
	fmt.Println("-------------------")
	demonstrateBatchOperations(complexData)

	fmt.Println("\nAdvanced features complete!")
}

func demonstrateComplexPaths(data string) {
	// Access first team of first department
	firstTeam, _ := json.GetString(data, "departments[0].teams[0].name")
	fmt.Printf("   First team: %s\n", firstTeam)

	// Access last member of last team using negative index
	lastMember, _ := json.GetString(data, "departments[0].teams[-1].members[-1].name")
	fmt.Printf("   Last member of last team: %s\n", lastMember)

	// Array slicing - get first 2 departments
	firstTwoDepts, _ := json.Get(data, "departments[0:2]{name}")
	fmt.Printf("   First two departments: %v\n", firstTwoDepts)

	// Deep nested access
	aliceRole, _ := json.GetString(data, "departments[0].teams[0].members[0].role")
	fmt.Printf("   Alice's role: %s\n", aliceRole)

	// Access skill within nested arrays
	firstSkill, _ := json.GetString(data, "departments[0].teams[0].members[0].skills[0]")
	fmt.Printf("   Alice's first skill: %s\n", firstSkill)
}

func demonstrateExtraction(data string) {
	// Extract all department names
	deptNames, _ := json.Get(data, "departments{name}")
	fmt.Printf("   Department names: %v\n", deptNames)

	// Extract all team names (nested)
	teamNames, _ := json.Get(data, "departments{teams}{name}")
	fmt.Printf("   Team names (nested): %v\n", teamNames)

	// Extract all member names from first department
	memberNames, _ := json.Get(data, "departments[0].teams{members}{name}")
	fmt.Printf("   Member names: %v\n", memberNames)

	// Extract specific fields from all members
	memberRoles, _ := json.Get(data, "departments{teams}{members}{role}")
	fmt.Printf("   Member roles: %v\n", memberRoles)
}

func demonstrateFlatExtraction(data string) {
	// Flat extraction - flattens all nested arrays into single array

	// Extract all teams (flat) from all departments
	allTeams, _ := json.Get(data, "departments{flat:teams}")
	fmt.Printf("   All teams (flat): %v\n", allTeams)

	// Extract all team names from flattened teams
	teamNames, _ := json.Get(data, "departments{flat:teams}{name}")
	fmt.Printf("   All team names (flat): %v\n", teamNames)

	// Extract all skills from all members (using chained flat extractions)
	allSkills, _ := json.Get(data, "departments{flat:teams}{flat:members}{flat:skills}")
	fmt.Printf("   All skills (flat): %v\n", allSkills)

	// Extract member names with flat extraction
	allMemberNames, _ := json.Get(data, "departments{flat:teams}{flat:members}{name}")
	fmt.Printf("   All member names (flat): %v\n", allMemberNames)
}

func demonstrateDeepModifications(data string) {
	// Modify deep nested value
	updated, _ := json.Set(data, "departments[0].teams[0].members[0].role", "Senior Lead")
	newRole, _ := json.GetString(updated, "departments[0].teams[0].members[0].role")
	fmt.Printf("   Updated role: %s\n", newRole)

	// Add new member to team
	newMember := map[string]any{
		"name":   "Frank",
		"role":   "Engineer",
		"skills": []string{"Rust", "WebAssembly"},
	}

	// Get current members and append
	members, _ := json.GetArray(data, "departments[0].teams[0].members")
	members = append(members, newMember)
	updated2, _ := json.Set(data, "departments[0].teams[0].members", members)

	// Verify addition
	allMembers, _ := json.Get(updated2, "departments[0].teams[0].members{name}")
	fmt.Printf("   Backend members after addition: %v\n", allMembers)

	// Add nested path that doesn't exist
	updated3, _ := json.SetWithAdd(data, "departments[0].budget.allocated", 1000000)
	budget, _ := json.Get(updated3, "departments[0].budget")
	fmt.Printf("   New budget path: %v\n", budget)
}

func demonstrateBatchOperations(data string) {
	// Batch update multiple deep paths
	updates := map[string]any{
		"departments[0].name":                     "Engineering & Innovation",
		"departments[0].teams[0].members[0].name": "Alice Smith",
		"metadata.tags[0]":                        "technology",
	}
	updated, _ := json.SetMultiple(data, updates)

	newDeptName, _ := json.GetString(updated, "departments[0].name")
	newMemberName, _ := json.GetString(updated, "departments[0].teams[0].members[0].name")
	newTag, _ := json.GetString(updated, "metadata.tags[0]")

	fmt.Printf("   After batch update:\n")
	fmt.Printf("   - Department: %s\n", newDeptName)
	fmt.Printf("   - Member: %s\n", newMemberName)
	fmt.Printf("   - Tag: %s\n", newTag)

	// Batch get multiple paths
	paths := []string{
		"organization",
		"departments[0].name",
		"departments[0].teams[0].name",
		"metadata.created",
	}

	results, err := json.GetMultiple(data, paths)
	if err != nil {
		log.Printf("Error: %v", err)
		return
	}

	fmt.Println("\n   Batch get results:")
	for path, value := range results {
		fmt.Printf("   - %s: %v\n", path, value)
	}

	// SetMultipleWithAdd for paths that may not exist
	newUpdates := map[string]any{
		"statistics.total_departments": 2,
		"statistics.total_teams":       3,
		"statistics.last_updated":      "2024-06-15",
	}
	updated2, _ := json.SetMultipleWithAdd(data, newUpdates)

	stats, _ := json.Get(updated2, "statistics")
	fmt.Printf("\n   New statistics section: %v\n", stats)
}
