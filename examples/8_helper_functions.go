//go:build example

package main

import (
	"fmt"

	"github.com/cybergodev/json"
)

// Helper Functions Example
//
// This example demonstrates useful helper functions in the cybergodev/json library
// for validation, comparison, merging, and data manipulation.
//
// Topics covered:
// - IsValidJSON and IsValidPath
// - DeepCopy for cloning JSON data
// - CompareJson for JSON comparison
// - MergeJson for combining JSON objects
// - FormatPretty and FormatCompact
//
// Run: go run examples/8_helper_functions.go

func main() {
	fmt.Println("🛠️  JSON Library - Helper Functions")
	fmt.Println("===================================\n ")

	// 1. JSON VALIDATION
	fmt.Println("1️⃣  JSON Validation")
	fmt.Println("───────────────────")
	demonstrateValidation()

	// 2. PATH VALIDATION
	fmt.Println("\n2️⃣  Path Validation")
	fmt.Println("───────────────────")
	demonstratePathValidation()

	// 3. DEEP COPY
	fmt.Println("\n3️⃣  Deep Copy")
	fmt.Println("─────────────")
	demonstrateDeepCopyData()

	// 4. JSON COMPARISON
	fmt.Println("\n4️⃣  JSON Comparison")
	fmt.Println("───────────────────")
	demonstrateComparison()

	// 5. JSON MERGE
	fmt.Println("\n5️⃣  JSON Merge")
	fmt.Println("───────────────")
	demonstrateMerge()

	// 6. FORMATTING
	fmt.Println("\n6️⃣  Formatting")
	fmt.Println("─────────────")
	demonstrateFormatting()

	fmt.Println("\n✅ Helper functions examples complete!")
}

func demonstrateValidation() {
	testCases := []struct {
		name string
		data string
	}{
		{"Valid object", `{"name": "John", "age": 30}`},
		{"Valid array", `[1, 2, 3, 4]`},
		{"Valid string", `"hello world"`},
		{"Valid number", `42.5`},
		{"Valid boolean", `true`},
		{"Invalid - missing brace", `{"name": "John"`},
		{"Invalid - trailing comma", `{"name": "John",}`},
		{"Invalid - unquoted key", `{name: "John"}`},
		{"Empty string", ``},
	}

	fmt.Println("   IsValidJSON results:")
	for _, tc := range testCases {
		valid := json.IsValidJSON(tc.data)
		status := "✓"
		if !valid {
			status = "✗"
		}
		fmt.Printf("   %s [%s]\n", status, tc.name)
	}
}

func demonstratePathValidation() {
	testPaths := []struct {
		name string
		path string
	}{
		{"Root", "."},
		{"Simple property", "user.name"},
		{"Nested property", "data.settings.theme"},
		{"Array index", "users[0]"},
		{"Nested array", "data[0].items[1].name"},
		{"Extraction", "users{name}"},
		{"Slice", "items[0:5]"},
		{"Empty", ""},
		{"Path traversal", "../secret"},
		{"Invalid brackets", "data[0"},
		{"Double dots", "data..field"},
	}

	fmt.Println("   IsValidPath results:")
	for _, tc := range testPaths {
		valid := json.IsValidPath(tc.path)
		status := "✓"
		if !valid {
			status = "✗"
		}
		fmt.Printf("   %s [%s] '%s'\n", status, tc.name, tc.path)
	}
}

func demonstrateDeepCopyData() {
	// Original complex data
	original := map[string]interface{}{
		"user": "Alice",
		"age":  30,
		"address": map[string]interface{}{
			"street":  "123 Main St",
			"city":    "Springfield",
			"country": "USA",
		},
		"hobbies": []interface{}{"reading", "coding", "gaming"},
	}

	fmt.Println("   Creating deep copy:")

	// Create deep copy
	copied, err := json.DeepCopy(original)
	if err != nil {
		fmt.Printf("   Error creating copy: %v\n", err)
		return
	}

	fmt.Println("   ✓ Deep copy created successfully")

	// Modify the copy
	if copiedMap, ok := copied.(map[string]interface{}); ok {
		// Change nested value
		if addr, ok := copiedMap["address"].(map[string]interface{}); ok {
			addr["city"] = "New York"
		}
		// Change array value
		if hobbies, ok := copiedMap["hobbies"].([]interface{}); ok {
			hobbies[0] = "writing"
		}
	}

	// Verify original is unchanged
	originalCity := original["address"].(map[string]interface{})["city"]
	copiedCity := copied.(map[string]interface{})["address"].(map[string]interface{})["city"]

	fmt.Printf("\n   Original city: %s\n", originalCity)
	fmt.Printf("   Copy city:     %s\n", copiedCity)
	fmt.Println("\n   ✓ Original data unchanged (deep copy works!)")
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
			fmt.Printf("   ✗ [%s] Error: %v\n", tc.name, err)
			continue
		}

		status := "✓"
		if equal != tc.equal {
			status = "✗"
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

	fmt.Println("\n   ✓ Formatting reversible!")
}
