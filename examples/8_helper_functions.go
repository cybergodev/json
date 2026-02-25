//go:build example

package main

import (
	"fmt"

	"github.com/cybergodev/json"
)

// Helper Functions Example
//
// This example demonstrates useful helper functions in the cybergodev/json library
// for comparison, merging, and data manipulation.
//
// Topics covered:
// - CompareJson for JSON comparison
// - MergeJson for combining JSON objects
// - FormatPretty and FormatCompact for formatting
// - Print and PrintPretty for quick output
//
// For JSON validation, see: 6_validation.go
// For DeepCopy, see: 7_type_conversion.go
//
// Run: go run examples/8_helper_functions.go

func main() {
	fmt.Println("Helper Functions - JSON Library")
	fmt.Println("================================\n ")

	// 1. JSON COMPARISON
	fmt.Println("1. JSON Comparison (CompareJson)")
	fmt.Println("--------------------------------")
	demonstrateComparison()

	// 2. JSON MERGE
	fmt.Println("\n2. JSON Merge (MergeJson)")
	fmt.Println("--------------------------")
	demonstrateMerge()

	// 3. FORMATTING
	fmt.Println("\n3. Formatting (FormatPretty/FormatCompact)")
	fmt.Println("-------------------------------------------")
	demonstrateFormatting()

	// 4. QUICK PRINT
	fmt.Println("\n4. Quick Print Functions")
	fmt.Println("------------------------")
	demonstratePrint()

	fmt.Println("\nHelper functions examples complete!")
}

func demonstrateComparison() {
	testCases := []struct {
		name  string
		json1 string
		json2 string
		equal bool
	}{
		{
			"Identical objects",
			`{"name": "John", "age": 30}`,
			`{"name": "John", "age": 30}`,
			true,
		},
		{
			"Different order (same data)",
			`{"name": "John", "age": 30}`,
			`{"age": 30, "name": "John"}`,
			true,
		},
		{
			"Different values",
			`{"name": "John", "age": 30}`,
			`{"name": "John", "age": 31}`,
			false,
		},
		{
			"Missing field",
			`{"name": "John", "age": 30}`,
			`{"name": "John"}`,
			false,
		},
		{
			"Arrays same order",
			`[1, 2, 3]`,
			`[1, 2, 3]`,
			true,
		},
		{
			"Arrays different order",
			`[1, 2, 3]`,
			`[3, 2, 1]`,
			false,
		},
	}

	fmt.Println("   CompareJson results:")
	for _, tc := range testCases {
		equal, err := json.CompareJson(tc.json1, tc.json2)
		if err != nil {
			fmt.Printf("   [ERROR] %s: %v\n", tc.name, err)
			continue
		}

		status := "[PASS]"
		if equal != tc.equal {
			status = "[FAIL]"
		}
		fmt.Printf("   %s [%s] equal=%v\n", status, tc.name, equal)
	}
}

func demonstrateMerge() {
	// Base configuration
	baseConfig := `{
		"database": {
			"host": "localhost",
			"port": 5432,
			"name": "myDb"
		},
		"features": ["auth", "logging"],
		"debug": false
	}`

	// Override configuration
	overrideConfig := `{
		"database": {
			"host": "prod-server",
			"ssl": true
		},
		"features": ["caching"],
		"monitoring": true
	}`

	fmt.Println("   MergeJson demonstration:")
	fmt.Println("\n   Base config:")
	fmt.Println(baseConfig)

	fmt.Println("\n   Override config:")
	fmt.Println(overrideConfig)

	// Merge
	merged, err := json.MergeJson(baseConfig, overrideConfig)
	if err != nil {
		fmt.Printf("   Error merging: %v\n", err)
		return
	}

	fmt.Println("\n   Merged result:")
	fmt.Println(merged)

	// Verify merge results
	fmt.Println("\n   Verification:")
	host, _ := json.GetString(merged, "database.host")
	fmt.Printf("   - database.host: %s (from override)\n", host)

	port, _ := json.GetInt(merged, "database.port")
	fmt.Printf("   - database.port: %d (from base)\n", port)

	ssl, _ := json.GetBool(merged, "database.ssl")
	fmt.Printf("   - database.ssl: %t (from override)\n", ssl)

	debug, _ := json.GetBool(merged, "debug")
	fmt.Printf("   - debug: %t (from base)\n", debug)

	monitoring, _ := json.GetBool(merged, "monitoring")
	fmt.Printf("   - monitoring: %t (from override)\n", monitoring)
}

func demonstrateFormatting() {
	compactJSON := `{"name":"John","age":30,"address":{"city":"NYC","zip":"10001"},"active":true}`

	fmt.Println("   Format formatting:")
	fmt.Println("\n   Original (compact):")
	fmt.Println(compactJSON)

	// Format as pretty
	pretty, err := json.FormatPretty(compactJSON)
	if err != nil {
		fmt.Printf("   Error: %v\n", err)
		return
	}

	fmt.Println("\n   FormatPretty result:")
	fmt.Println(pretty)

	// Format as compact
	compact, err := json.FormatCompact(pretty)
	if err != nil {
		fmt.Printf("   Error: %v\n", err)
		return
	}

	fmt.Println("\n   FormatCompact result:")
	fmt.Println(compact)

	fmt.Println("\n   Formatting is reversible!")
}

func demonstratePrint() {
	data := map[string]any{
		"user":    "Alice",
		"age":     30,
		"active":  true,
		"tags":    []string{"go", "json"},
		"balance": 1250.75,
	}

	fmt.Println("   Print (compact, single line):")
	json.Print(data)

	fmt.Println("\n   PrintPretty (formatted for readability):")
	json.PrintPretty(data)

	fmt.Println("\n   PrintE and PrintPrettyE return errors for programmatic use:")
	if err := json.PrintE(data); err != nil {
		fmt.Printf("   PrintE error: %v\n", err)
	}
}
