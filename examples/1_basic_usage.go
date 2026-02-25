//go:build example

package main

import (
	"fmt"
	"log"

	"github.com/cybergodev/json"
)

// Basic Usage Example
//
// This example demonstrates the essential features for getting started with the cybergodev/json library.
// Perfect for developers who want to quickly understand the core functionality.
//
// Topics covered:
// - Basic Get/Set operations
// - Type-safe retrieval (GetString, GetInt, GetBool, etc.)
// - Array operations and indexing
// - Batch operations (GetMultiple, SetMultiple)
// - 100% encoding/json compatibility
//
// For advanced delete operations, see: 12_advanced_delete.go
// For advanced features, see: 2_advanced_features.go
// For production patterns, see: 3_production_ready.go
//
// Run: go run examples/basic_usage.go

func main() {
	fmt.Println("Basic Usage - JSON Library")
	fmt.Println("===========================\n ")

	// Sample JSON data
	sampleData := `{
		"user": {
			"id": 1001,
			"name": "Alice Johnson",
			"email": "alice@example.com",
			"age": 28,
			"active": true,
			"balance": 1250.75,
			"tags": ["premium", "verified", "developer"]
		},
		"settings": {
			"theme": "dark",
			"notifications": true,
			"language": "en"
		}
	}`

	// 1. BASIC GET OPERATIONS
	fmt.Println("1. Basic Get Operations")
	fmt.Println("-----------------------")
	demonstrateGet(sampleData)

	// 2. TYPE-SAFE OPERATIONS
	fmt.Println("\n2. Type-Safe Operations")
	fmt.Println("-----------------------")
	demonstrateTypeSafe(sampleData)

	// 3. SET OPERATIONS
	fmt.Println("\n3. Set Operations")
	fmt.Println("-----------------")
	demonstrateSet(sampleData)

	// 4. ARRAY OPERATIONS
	fmt.Println("\n4. Array Operations")
	fmt.Println("-------------------")
	demonstrateArrays(sampleData)

	// 5. BATCH OPERATIONS
	fmt.Println("\n5. Batch Operations")
	fmt.Println("-------------------")
	demonstrateBatch(sampleData)

	// 6. ENCODING/JSON COMPATIBILITY
	fmt.Println("\n6. encoding/json Compatibility")
	fmt.Println("------------------------------")
	demonstrateCompatibility()

	fmt.Println("\nBasic usage complete!")
	fmt.Println("Next steps:")
	fmt.Println("   - Check 2_advanced_features.go for complex operations")
	fmt.Println("   - Check 3_production_ready.go for production best practices")
	fmt.Println("   - Check 12_advanced_delete.go for advanced delete operations")
}

func demonstrateGet(data string) {
	// Simple field access
	name, _ := json.Get(data, "user.name")
	fmt.Printf("   Name: %v\n", name)

	// Nested field access
	theme, _ := json.Get(data, "settings.theme")
	fmt.Printf("   Theme: %v\n", theme)

	// Array element access
	firstTag, _ := json.Get(data, "user.tags[0]")
	fmt.Printf("   First tag: %v\n", firstTag)

	// Negative index (last element)
	lastTag, _ := json.Get(data, "user.tags[-1]")
	fmt.Printf("   Last tag: %v\n", lastTag)

	// Non-existent path returns nil
	missing, _ := json.Get(data, "user.phone")
	fmt.Printf("   Missing path (user.phone): %v\n", missing)
}

func demonstrateTypeSafe(data string) {
	// Type-safe getters with automatic conversion
	name, _ := json.GetString(data, "user.name")
	fmt.Printf("   Name (string): %s\n", name)

	age, _ := json.GetInt(data, "user.age")
	fmt.Printf("   Age (int): %d\n", age)

	balance, _ := json.GetFloat64(data, "user.balance")
	fmt.Printf("   Balance (float64): %.2f\n", balance)

	active, _ := json.GetBool(data, "user.active")
	fmt.Printf("   Active (bool): %t\n", active)

	tags, _ := json.GetArray(data, "user.tags")
	fmt.Printf("   Tags (array): %v\n", tags)

	settings, _ := json.GetObject(data, "settings")
	fmt.Printf("   Settings (object): %v\n", settings)

	// Generic GetTyped for custom types
	id, _ := json.GetTyped[int](data, "user.id")
	fmt.Printf("   ID (generic): %d\n", id)
}

func demonstrateSet(data string) {
	// Set simple field
	updated, _ := json.Set(data, "user.age", 29)
	newAge, _ := json.GetInt(updated, "user.age")
	fmt.Printf("   Updated age: %d\n", newAge)

	// Set nested field
	updated2, _ := json.Set(data, "settings.theme", "light")
	newTheme, _ := json.GetString(updated2, "settings.theme")
	fmt.Printf("   Updated theme: %s\n", newTheme)

	// SetWithAdd creates paths automatically
	updated3, _ := json.SetWithAdd(data, "user.premium.level", "gold")
	level, _ := json.GetString(updated3, "user.premium.level")
	fmt.Printf("   New premium level (auto-created): %s\n", level)

	// Set array element
	updated4, _ := json.Set(data, "user.tags[0]", "VIP")
	firstTag, _ := json.GetString(updated4, "user.tags[0]")
	fmt.Printf("   Updated first tag: %s\n", firstTag)
}

func demonstrateArrays(data string) {
	// Array slicing
	firstTwo, _ := json.Get(data, "user.tags[0:2]")
	fmt.Printf("   First two tags: %v\n", firstTwo)

	// Extract all values from array
	allTags, _ := json.GetArray(data, "user.tags")
	fmt.Printf("   All tags: %v, Count: %d\n", allTags, len(allTags))

	// Array with negative indices
	lastTwo, _ := json.Get(data, "user.tags[-2:]")
	fmt.Printf("   Last two tags: %v\n", lastTwo)
}

func demonstrateBatch(data string) {
	// Batch get multiple paths
	paths := []string{"user.name", "user.age", "settings.theme"}
	results, err := json.GetMultiple(data, paths)
	if err != nil {
		log.Printf("Error: %v", err)
		return
	}
	fmt.Printf("   Batch get results: %v\n", results)

	// Batch set multiple values
	updates := map[string]any{
		"user.age":       30,
		"settings.theme": "auto",
		"user.active":    false,
	}
	updated, _ := json.SetMultiple(data, updates)

	// Verify updates
	newAge, _ := json.GetInt(updated, "user.age")
	newTheme, _ := json.GetString(updated, "settings.theme")
	newActive, _ := json.GetBool(updated, "user.active")
	fmt.Printf("   After batch set - Age: %d, Theme: %s, Active: %t\n",
		newAge, newTheme, newActive)

	// SetMultipleWithAdd for paths that may not exist
	newUpdates := map[string]any{
		"user.stats.logins":    100,
		"user.stats.lastLogin": "2024-06-15",
	}
	updated2, _ := json.SetMultipleWithAdd(data, newUpdates)
	logins, _ := json.GetInt(updated2, "user.stats.logins")
	fmt.Printf("   New stats.logins: %d\n", logins)
}

func demonstrateCompatibility() {
	// 100% compatible with encoding/json
	type User struct {
		Name   string   `json:"name"`
		Age    int      `json:"age"`
		Active bool     `json:"active"`
		Tags   []string `json:"tags"`
	}

	user := User{
		Name:   "Bob Smith",
		Age:    35,
		Active: true,
		Tags:   []string{"admin", "moderator"},
	}

	// Marshal (same as encoding/json)
	jsonBytes, err := json.Marshal(user)
	if err != nil {
		log.Printf("Marshal error: %v", err)
		return
	}
	fmt.Printf("   Marshaled: %s\n", string(jsonBytes))

	// Unmarshal (same as encoding/json)
	var decoded User
	err = json.Unmarshal(jsonBytes, &decoded)
	if err != nil {
		log.Printf("Unmarshal error: %v", err)
		return
	}
	fmt.Printf("   Unmarshaled: %+v\n", decoded)

	// MarshalIndent (same as encoding/json)
	prettyJSON, _ := json.MarshalIndent(user, "", "  ")
	fmt.Printf("   Pretty JSON:\n%s\n", string(prettyJSON))

	// Valid (same as encoding/json)
	valid := json.Valid(jsonBytes)
	fmt.Printf("   JSON valid: %t\n", valid)
}
