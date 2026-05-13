//go:build example

package main

import (
	"bytes"
	"fmt"

	"github.com/cybergodev/json"
)

// Helper Functions Example
//
// This example demonstrates useful helper functions in the cybergodev/json library
// for comparison, merging, and data manipulation.
//
// Topics covered:
// - CompareJSON for JSON comparison
// - MergeJSON for combining JSON objects
// - Prettify and Compact for formatting
// - Encode and EncodePretty for JSON output
//
// For JSON validation, see: 6_validation.go
// For DeepCopy, see: 7_type_conversion.go
//
// Run: go run -tags=example examples/8_helper_functions.go

func main() {
	fmt.Println("JSON Library - Helper Functions")
	fmt.Println("===================================")

	// 1. JSON COMPARISON
	demonstrateComparison()

	// 2. JSON MERGE
	demonstrateMerge()

	// 3. FORMATTING
	demonstrateFormatting()

	// 4. ENCODE FUNCTIONS
	demonstratePrint()

	fmt.Println("\nHelper functions examples complete!")
}

func demonstrateComparison() {
	fmt.Println("1. JSON Comparison (CompareJSON)")
	fmt.Println("--------------------------------")

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

	fmt.Println("   CompareJSON results:")
	for _, tc := range testCases {
		equal, err := json.CompareJSON(tc.json1, tc.json2)
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
	fmt.Println("\n2. JSON Merge (MergeJSON)")
	fmt.Println("--------------------------")

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

	fmt.Println("   MergeJSON demonstration:")
	fmt.Println("\n   Base config:")
	fmt.Println(baseConfig)

	fmt.Println("\n   Override config:")
	fmt.Println(overrideConfig)

	// Union merge (default)
	merged, err := json.MergeJSON(baseConfig, overrideConfig)
	if err != nil {
		fmt.Printf("   Error merging: %v\n", err)
		return
	}

	fmt.Println("\n   Merged result (Union - default):")
	fmt.Println(merged)

	// Verify merge results
	fmt.Println("\n   Verification:")
	host := json.GetString(merged, "database.host", "")
	fmt.Printf("   - database.host: %s (from override)\n", host)

	port := json.GetInt(merged, "database.port", 0)
	fmt.Printf("   - database.port: %d (from base)\n", port)

	ssl := json.GetBool(merged, "database.ssl", false)
	fmt.Printf("   - database.ssl: %t (from override)\n", ssl)

	debug := json.GetBool(merged, "debug", false)
	fmt.Printf("   - debug: %t (from base)\n", debug)

	monitoring := json.GetBool(merged, "monitoring", false)
	fmt.Printf("   - monitoring: %t (from override)\n", monitoring)

	// Demonstrate different merge modes
	fmt.Println("\n   Merge Modes:")

	// Intersection merge - only common keys
	intersectCfg := json.DefaultConfig()
	intersectCfg.MergeMode = json.MergeIntersection
	intersected, _ := json.MergeJSON(baseConfig, overrideConfig, intersectCfg)
	fmt.Println("\n   Intersection (common keys only):")
	fmt.Println(intersected)

	// Difference merge - keys only in base
	diffCfg := json.DefaultConfig()
	diffCfg.MergeMode = json.MergeDifference
	diff, _ := json.MergeJSON(baseConfig, overrideConfig, diffCfg)
	fmt.Println("\n   Difference (keys only in base):")
	fmt.Println(diff)

	// MergeMany - merge multiple JSON strings at once
	overrides := []string{
		`{"database": {"ssl": true}}`,
		`{"features": ["caching", "metrics"]}`,
		`{"monitoring": true}`,
	}
	mergedMany, err := json.MergeMany(overrides)
	if err != nil {
		fmt.Printf("   MergeMany error: %v\n", err)
	} else {
		fmt.Println("\n   MergeMany (3 JSONs merged):")
		fmt.Println(mergedMany)
	}
}

func demonstrateFormatting() {
	fmt.Println("\n3. Formatting (Prettify/Compact)")
	fmt.Println("-------------------------------------------")

	compactJSON := `{"name":"John","age":30,"address":{"city":"NYC","zip":"10001"},"active":true}`

	fmt.Println("   Format formatting:")
	fmt.Println("\n   Original (compact):")
	fmt.Println(compactJSON)

	// Format as pretty
	pretty, err := json.Prettify(compactJSON)
	if err != nil {
		fmt.Printf("   Error: %v\n", err)
		return
	}

	fmt.Println("\n   Prettify result:")
	fmt.Println(pretty)

	// Format as compact
	var buf bytes.Buffer
	err = json.Compact(&buf, []byte(pretty))
	if err != nil {
		fmt.Printf("   Error: %v\n", err)
		return
	}
	compact := buf.String()

	fmt.Println("\n   Compact result:")
	fmt.Println(compact)

	fmt.Println("\n   Formatting is reversible!")
}

func demonstratePrint() {
	fmt.Println("\n4. Encode Functions")
	fmt.Println("-------------------")

	data := map[string]any{
		"user":    "Alice",
		"age":     30,
		"active":  true,
		"tags":    []string{"go", "json"},
		"balance": 1250.75,
	}

	fmt.Println("   Encode (compact, single line):")
	result, _ := json.Encode(data)
	fmt.Println(result)

	fmt.Println("\n   EncodePretty (formatted for readability):")
	pretty, _ := json.EncodePretty(data)
	fmt.Println(pretty)

	fmt.Println("\n   Encode with error handling:")
	if encoded, err := json.Encode(data); err != nil {
		fmt.Printf("   Encode error: %v\n", err)
	} else {
		fmt.Println(encoded)
	}
}
