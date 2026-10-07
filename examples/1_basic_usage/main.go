//go:build example

package main

import (
	"bytes"
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
// - Parsing strings with Parse and ParseAny
// - 100% encoding/json compatibility
//
// For advanced delete operations, see: ./examples/12_advanced_delete
// For advanced features, see: ./examples/2_advanced_features
// For production patterns, see: ./examples/3_production_ready
//
// Run: go run -tags=example ./examples/1_basic_usage

func main() {
	fmt.Println("Basic Usage - JSON Library")
	fmt.Println("===========================")

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
	demonstrateGet(sampleData)

	// 2. TYPE-SAFE OPERATIONS
	demonstrateTypeSafe(sampleData)

	// 3. SET OPERATIONS
	demonstrateSet(sampleData)

	// 4. ARRAY OPERATIONS
	demonstrateArrays(sampleData)

	// 5. BATCH OPERATIONS
	demonstrateBatch(sampleData)

	// 6. ENCODING/JSON COMPATIBILITY
	demonstrateCompatibility()

	// 7. STREAMING ENCODER/DECODER
	demonstrateStreaming()

	// 8. STRING PARSING
	demonstrateParsing()

	fmt.Println("\nBasic usage complete!")
}

func demonstrateGet(data string) {
	fmt.Println("1. Basic Get Operations")
	fmt.Println("-----------------------")

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
	fmt.Println("\n2. Type-Safe Operations")
	fmt.Println("-----------------------")

	// Type-safe getters with automatic conversion and default values
	name := json.GetString(data, "user.name", "")
	fmt.Printf("   Name (string): %s\n", name)

	age := json.GetInt(data, "user.age", 0)
	fmt.Printf("   Age (int): %d\n", age)

	balance := json.GetFloat(data, "user.balance", 0.0)
	fmt.Printf("   Balance (float64): %.2f\n", balance)

	active := json.GetBool(data, "user.active", false)
	fmt.Printf("   Active (bool): %t\n", active)

	tags := json.GetArray(data, "user.tags", nil)
	fmt.Printf("   Tags (array): %v\n", tags)

	settings := json.GetObject(data, "settings", nil)
	fmt.Printf("   Settings (object): %v\n", settings)

	// Generic GetTyped for custom types with default value
	id := json.GetTyped[int](data, "user.id", 0)
	fmt.Printf("   ID (generic): %d\n", id)
}

func demonstrateSet(data string) {
	fmt.Println("\n3. Set Operations")
	fmt.Println("-----------------")

	// Set simple field
	updated, _ := json.Set(data, "user.age", 29)
	newAge := json.GetInt(updated, "user.age", 0)
	fmt.Printf("   Updated age: %d\n", newAge)

	// Set nested field
	updated2, _ := json.Set(data, "settings.theme", "light")
	newTheme := json.GetString(updated2, "settings.theme", "")
	fmt.Printf("   Updated theme: %s\n", newTheme)

	// SetCreate auto-creates intermediate paths (equivalent to Set with
	// Config{CreatePaths: true}, but without the manual Config).
	updated3, _ := json.SetCreate(data, "user.premium.level", "gold")
	level := json.GetString(updated3, "user.premium.level", "")
	fmt.Printf("   New premium level (auto-created): %s\n", level)

	// Set array element
	updated4, _ := json.Set(data, "user.tags[0]", "VIP")
	firstTag := json.GetString(updated4, "user.tags[0]", "")
	fmt.Printf("   Updated first tag: %s\n", firstTag)

	// Append array element
	updated5, _ := json.Set(data, "user.tags[+]", "Testers")
	lastTag := json.GetString(updated5, "user.tags[-1]", "")
	fmt.Printf("   Append tag: %s\n", lastTag)
}

func demonstrateArrays(data string) {
	fmt.Println("\n4. Array Operations")
	fmt.Println("-------------------")

	// Array slicing
	firstTwo, _ := json.Get(data, "user.tags[0:2]")
	fmt.Printf("   First two tags: %v\n", firstTwo)

	// Extract all values from array
	allTags := json.GetArray(data, "user.tags", nil)
	fmt.Printf("   All tags: %v, Count: %d\n", allTags, len(allTags))

	// Array with negative indices
	lastTwo, _ := json.Get(data, "user.tags[-2:]")
	fmt.Printf("   Last two tags: %v\n", lastTwo)
}

func demonstrateBatch(data string) {
	fmt.Println("\n5. Batch Operations")
	fmt.Println("-------------------")

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
	newAge := json.GetInt(updated, "user.age", 0)
	newTheme := json.GetString(updated, "settings.theme", "")
	newActive := json.GetBool(updated, "user.active", false)
	fmt.Printf("   After batch set - Age: %d, Theme: %s, Active: %t\n",
		newAge, newTheme, newActive)

	// SetMultipleCreate auto-creates intermediate paths for every update.
	newUpdates := map[string]any{
		"user.stats.logins":    100,
		"user.stats.lastLogin": "2024-06-15",
	}
	updated2, _ := json.SetMultipleCreate(data, newUpdates)
	logins := json.GetInt(updated2, "user.stats.logins", 0)
	fmt.Printf("   New stats.logins: %d\n", logins)
}

func demonstrateCompatibility() {
	fmt.Println("\n6. encoding/json Compatibility")
	fmt.Println("------------------------------")

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

func demonstrateStreaming() {
	fmt.Println("\n7. Streaming Encoder/Decoder")
	fmt.Println("----------------------------")

	type Item struct {
		Name  string  `json:"name"`
		Price float64 `json:"price"`
	}

	items := []Item{
		{Name: "Laptop", Price: 999.99},
		{Name: "Mouse", Price: 29.99},
	}

	// Encode multiple objects to a buffer
	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)
	for _, item := range items {
		if err := encoder.Encode(item); err != nil {
			log.Printf("Encode error: %v", err)
		}
	}
	fmt.Printf("   Encoded %d objects:\n%s", len(items), buf.String())

	// Decode multiple objects from a buffer
	decoder := json.NewDecoder(&buf)
	count := 0
	for decoder.More() {
		var item Item
		if err := decoder.Decode(&item); err != nil {
			log.Printf("Decode error: %v", err)
			break
		}
		fmt.Printf("   Decoded: %+v\n", item)
		count++
	}
	fmt.Printf("   Decoded %d objects\n", count)
}

func demonstrateParsing() {
	fmt.Println("\n8. Parsing Strings (Parse / ParseAny)")
	fmt.Println("-------------------------------------")

	type Address struct {
		City string `json:"city"`
		Zip  string `json:"zip"`
	}
	type Profile struct {
		Name    string   `json:"name"`
		Tags    []string `json:"tags"`
		Address Address  `json:"address"`
	}

	jsonStr := `{
		"name": "Carol",
		"tags": ["admin", "ops"],
		"address": {"city": "NYC", "zip": "10001"}
	}`

	// Parse decodes a JSON string into a typed target (string-in, unlike
	// Unmarshal which takes []byte).
	var profile Profile
	if err := json.Parse(jsonStr, &profile); err != nil {
		log.Printf("Parse error: %v", err)
		return
	}
	fmt.Printf("   Parse into struct: %s / %v / %v\n",
		profile.Name, profile.Tags, profile.Address)

	// ParseAny decodes into any — map[string]any for objects, []any for arrays.
	data, err := json.ParseAny(jsonStr)
	if err != nil {
		log.Printf("ParseAny error: %v", err)
		return
	}
	fmt.Printf("   ParseAny: %T with %d top-level keys\n", data, len(data.(map[string]any)))

	// Both accept an optional Config (e.g. SecurityConfig for untrusted input).
	safe, err := json.ParseAny(jsonStr, json.SecurityConfig())
	if err != nil {
		log.Printf("ParseAny error: %v", err)
		return
	}
	fmt.Printf("   ParseAny with SecurityConfig: %v\n", safe.(map[string]any)["name"])
}
