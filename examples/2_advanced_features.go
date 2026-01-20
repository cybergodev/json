//go:build example

package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/cybergodev/json"
)

// Advanced Features Example
//
// This example demonstrates advanced JSON operations for complex use cases.
// Ideal for developers who need to work with nested structures, file I/O, and bulk operations.
//
// Topics covered:
// - Complex path queries and nested extraction
// - Flat extraction from deeply nested structures
// - File I/O operations (read/write JSON files)
// - Iteration and transformation
// - Working with deeply nested data
//
// Run: go run examples/advanced_features.go

func main() {
	fmt.Println("🔧 JSON Library - Advanced Features")
	fmt.Println("====================================\n ")

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
	fmt.Println("1️⃣  Complex Path Queries")
	fmt.Println("─────────────────────────")
	demonstrateComplexPaths(complexData)

	// 2. NESTED EXTRACTION
	fmt.Println("\n2️⃣  Nested Extraction")
	fmt.Println("──────────────────────")
	demonstrateExtraction(complexData)

	// 3. FLAT EXTRACTION
	fmt.Println("\n3️⃣  Flat Extraction")
	fmt.Println("────────────────────")
	demonstrateFlatExtraction(complexData)

	// 4. ITERATION & TRANSFORMATION
	fmt.Println("\n4️⃣  Iteration & Transformation")
	fmt.Println("───────────────────────────────")
	demonstrateIteration(complexData)

	// 5. FILE OPERATIONS
	fmt.Println("\n5️⃣  File Operations")
	fmt.Println("────────────────────")
	demonstrateFileOps()

	// 6. DEEP MODIFICATIONS
	fmt.Println("\n6️⃣  Deep Modifications")
	fmt.Println("───────────────────────")
	demonstrateDeepModifications(complexData)

	fmt.Println("\n✅ Advanced features complete!")
	fmt.Println("💡 These features enable powerful JSON manipulation without unmarshaling!")
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
}

func demonstrateFlatExtraction(data string) {
	// Flat extraction - flattens all nested arrays into single array
	// Note: For complex nested structures with multiple extraction levels,
	// using flat: at each level separately is recommended

	// Extract all teams (flat) from all departments
	allTeams, _ := json.Get(data, "departments{flat:teams}")
	fmt.Printf("   All teams (flat): %v\n", allTeams)

	// Extract all team names from flattened teams
	teamNames, _ := json.Get(data, "departments{flat:teams}{name}")
	fmt.Printf("   All team names (flat): %v\n", teamNames)

	// Extract all skills from all members (using chained flat extractions)
	// This demonstrates extracting from deeply nested structures
	allSkills, _ := json.Get(data, "departments{flat:teams}{flat:members}{flat:skills}")
	fmt.Printf("   All skills by department: %v\n", allSkills)
}

func demonstrateIteration(data string) {
	// Iterate over departments
	departments, err := json.GetArray(data, "departments")
	if err != nil {
		log.Printf("Error: %v", err)
		return
	}

	fmt.Println("   Departments:")
	for i, dept := range departments {
		if deptMap, ok := dept.(map[string]any); ok {
			fmt.Printf("   [%d] %v\n", i, deptMap["name"])
		}
	}

	// Use ForeachWithPath to iterate over specific paths
	fmt.Println("\n   Iterating over first department teams:")
	err = json.ForeachWithPath(data, "departments[0].teams", func(key any, item *json.IterableValue) {
		teamName := item.GetString("name")
		fmt.Printf("   - Team: %s\n", teamName)
	})
	if err != nil {
		log.Printf("Iteration error: %v", err)
	}

	// Use ForeachNested for recursive iteration
	fmt.Println("\n   Nested iteration of first department:")
	count := 0
	json.ForeachNested(data, func(key any, item *json.IterableValue) {
		// Access value safely using Get method
		val := item.Get("")
		if str, ok := val.(string); ok && str != "" {
			count++
		}
	})
	fmt.Printf("   Found %d non-empty string values in nested structure\n", count)
}

func demonstrateFileOps() {
	// Create temporary directory
	tempDir, err := os.MkdirTemp("", "json-example-*")
	if err != nil {
		log.Printf("Failed to create temp dir: %v", err)
		return
	}
	defer os.RemoveAll(tempDir)

	// Sample data
	config := map[string]any{
		"version": "1.0.0",
		"server": map[string]any{
			"host": "localhost",
			"port": 8080,
		},
		"features": []string{"auth", "logging", "metrics"},
	}

	// Marshal to JSON
	jsonBytes, _ := json.MarshalIndent(config, "", "  ")

	// Write to file
	filePath := filepath.Join(tempDir, "config.json")
	err = os.WriteFile(filePath, jsonBytes, 0644)
	if err != nil {
		log.Printf("Write error: %v", err)
		return
	}
	fmt.Printf("   ✓ Written to: %s\n", filePath)

	// Read from file
	content, err := os.ReadFile(filePath)
	if err != nil {
		log.Printf("Read error: %v", err)
		return
	}

	// Extract values using path operations
	version, _ := json.GetString(string(content), "version")
	port, _ := json.GetInt(string(content), "server.port")
	fmt.Printf("   ✓ Read from file - Version: %s, Port: %d\n", version, port)

	// Modify and write back
	updated, _ := json.Set(string(content), "server.port", 9090)
	os.WriteFile(filePath, []byte(updated), 0644)
	fmt.Printf("   ✓ Modified and saved back to file\n")
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

	// Batch update multiple deep paths
	updates := map[string]any{
		"departments[0].name":                     "Engineering & Innovation",
		"departments[0].teams[0].members[0].name": "Alice Smith",
		"metadata.tags[0]":                        "technology",
	}
	updated3, _ := json.SetMultiple(data, updates)

	newDeptName, _ := json.GetString(updated3, "departments[0].name")
	newMemberName, _ := json.GetString(updated3, "departments[0].teams[0].members[0].name")
	newTag, _ := json.GetString(updated3, "metadata.tags[0]")

	fmt.Printf("   After batch update:\n")
	fmt.Printf("   - Department: %s\n", newDeptName)
	fmt.Printf("   - Member: %s\n", newMemberName)
	fmt.Printf("   - Tag: %s\n", newTag)
}
