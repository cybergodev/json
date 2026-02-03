//go:build example

package main

import (
	"fmt"

	"github.com/cybergodev/json"
)

// Iterator Functions Example
//
// This example demonstrates powerful iteration capabilities for JSON data.
// Learn about different iteration patterns and the IterableValue API.
//
// Topics covered:
// - Foreach for simple iteration
// - ForeachWithPath for targeted iteration
// - ForeachNested for recursive iteration
// - IterableValue API methods
// - IteratorControl for flow control
//
// Run: go run examples/9_iterator_functions.go

func main() {
	fmt.Println("🔁 JSON Library - Iterator Functions")
	fmt.Println("===================================\n ")

	// Sample data
	sampleData := `{
		"users": [
			{
				"id": 1,
				"name": "Alice",
				"email": "alice@example.com",
				"active": true,
				"roles": ["admin", "developer"]
			},
			{
				"id": 2,
				"name": "Bob",
				"email": "bob@example.com",
				"active": false,
				"roles": ["user"]
			},
			{
				"id": 3,
				"name": "Charlie",
				"email": "charlie@example.com",
				"active": true,
				"roles": ["developer", "designer"]
			}
		],
		"settings": {
			"theme": "dark",
			"notifications": true,
			"language": "en"
		}
	}`

	// 1. SIMPLE ITERATION
	fmt.Println("1️⃣  Simple Iteration (Foreach)")
	fmt.Println("────────────────────────────────")
	demonstrateSimpleIteration(sampleData)

	// 2. ITERATION WITH PATH
	fmt.Println("\n2️⃣  Iteration with Path (ForeachWithPath)")
	fmt.Println("──────────────────────────────────────────")
	demonstrateIterationWithPath(sampleData)

	// 3. NESTED ITERATION
	fmt.Println("\n3️⃣  Nested Iteration (ForeachNested)")
	fmt.Println("───────────────────────────────────────")
	demonstrateNestedIteration(sampleData)

	// 4. ITERABLE VALUE API
	fmt.Println("\n4️⃣  IterableValue API")
	fmt.Println("──────────────────────")
	demonstrateIterableValueAPI(sampleData)

	// 5. TRANSFORMATION
	fmt.Println("\n5️⃣  Data Transformation with Iteration")
	fmt.Println("────────────────────────────────────────")
	demonstrateTransformation(sampleData)

	fmt.Println("\n✅ Iterator functions examples complete!")
}

func demonstrateSimpleIteration(data string) {
	fmt.Println("   Iterating over entire JSON:")

	json.Foreach(data, func(key any, item *json.IterableValue) {
		// Top-level iteration
		fmt.Printf("   Key: %v, Type: %T\n", key, item.Get(""))
	})
}

func demonstrateIterationWithPath(data string) {
	fmt.Println("   Iterating over users array:")

	err := json.ForeachWithPath(data, "users", func(key any, item *json.IterableValue) {
		// Get user details
		name := item.GetString("name")
		email := item.GetString("email")
		active := item.GetBool("active")

		status := "active"
		if !active {
			status = "inactive"
		}

		fmt.Printf("   [%d] %s (%s) - %s\n", key, name, email, status)
	})

	if err != nil {
		fmt.Printf("   Error: %v\n", err)
	}

	// Iterate over roles of first user
	fmt.Println("\n   Iterating over roles of first user:")
	err = json.ForeachWithPath(data, "users[0].roles", func(key any, item *json.IterableValue) {
		role := item.Get("")
		if roleStr, ok := role.(string); ok {
			fmt.Printf("   - Role: %s\n", roleStr)
		}
	})

	if err != nil {
		fmt.Printf("   Error: %v\n", err)
	}
}

func demonstrateNestedIteration(data string) {
	fmt.Println("   Recursively iterating all values:")

	count := 0
	json.ForeachNested(data, func(key any, item *json.IterableValue) {
		count++
	})

	fmt.Printf("   Total values visited (including nested): %d\n", count)

	// Count specific types
	fmt.Println("\n   Counting by type:")

	intCount := 0
	strCount := 0
	boolCount := 0

	json.ForeachNested(data, func(key any, item *json.IterableValue) {
		val := item.Get("")
		switch val.(type) {
		case int, int64, float64:
			intCount++
		case string:
			strCount++
		case bool:
			boolCount++
		}
	})

	fmt.Printf("   Numbers: %d, Strings: %d, Booleans: %d\n", intCount, strCount, boolCount)
}

func demonstrateIterableValueAPI(data string) {
	fmt.Println("   IterableValue convenience methods:")

	// IMPORTANT: IterableValue works correctly when iterating over arrays
	// where each item is a complete object, NOT when iterating over object fields

	fmt.Println("   Using ForeachWithPath on users array:")

	err := json.ForeachWithPath(data, "users", func(key any, item *json.IterableValue) {
		// Now we can use all the IterableValue methods
		name := item.GetString("name")
		email := item.GetString("email")
		active := item.GetBool("active")
		id := item.GetInt("id")

		fmt.Printf("   - [%d] GetString: name=%s, email=%s\n", key, name, email)
		fmt.Printf("   - [%d] GetInt: id=%d\n", key, id)
		fmt.Printf("   - [%d] GetBool: active=%t\n", key, active)

		// GetWithDefault
		nonExistent := item.GetStringWithDefault("nonexistent", "default value")
		fmt.Printf("   - [%d] GetStringWithDefault: %s\n", key, nonExistent)

		// Check existence
		fmt.Printf("   - [%d] Exists('name'): %t\n", key, item.Exists("name"))
		fmt.Printf("   - [%d] Exists('missing'): %t\n", key, item.Exists("missing"))

		// Check for null
		fmt.Printf("   - [%d] IsNull('name'): %t\n", key, item.IsNull("name"))
		fmt.Printf("   - [%d] IsNull('missing'): %t\n", key, item.IsNull("missing"))

		// Check for empty
		fmt.Printf("   - [%d] IsEmpty('email'): %t\n", key, item.IsEmpty("email"))

		// GetArray
		roles := item.GetArray("roles")
		fmt.Printf("   - [%d] GetArray('roles'): %v (length: %d)\n\n", key, roles, len(roles))
	})

	if err != nil {
		fmt.Printf("   Error: %v\n", err)
	}

	// Access settings object
	fmt.Println("\n   Accessing nested object:")
	err = json.ForeachWithPath(data, "settings", func(key any, item *json.IterableValue) {
		// For object iteration, we can still use Get method
		val := item.Get("")
		fmt.Printf("   - [%v]: %v\n", key, val)
	})

	if err != nil {
		fmt.Printf("   Error: %v\n", err)
	}
}

func demonstrateTransformation(data string) {
	fmt.Println("   Building summary using iteration:")

	// Count active/inactive users
	activeCount := 0
	inactiveCount := 0
	rolesMap := make(map[string]int)

	// Iterate over users
	err := json.ForeachWithPath(data, "users", func(key any, item *json.IterableValue) {
		active := item.GetBool("active")
		if active {
			activeCount++
		} else {
			inactiveCount++
		}

		// Collect roles
		roles := item.GetArray("roles")
		for _, role := range roles {
			if roleStr, ok := role.(string); ok {
				rolesMap[roleStr]++
			}
		}
	})

	if err != nil {
		fmt.Printf("   Error: %v\n", err)
		return
	}

	fmt.Printf("   Active users: %d\n", activeCount)
	fmt.Printf("   Inactive users: %d\n", inactiveCount)
	fmt.Println("\n   Role distribution:")
	for role, count := range rolesMap {
		fmt.Printf("   - %s: %d\n", role, count)
	}

	// Find specific user
	fmt.Println("\n   Finding user by criteria:")
	err = json.ForeachWithPath(data, "users", func(key any, item *json.IterableValue) {
		name := item.GetString("name")
		active := item.GetBool("active")

		// Find active developers
		if active {
			roles := item.GetArray("roles")
			for _, role := range roles {
				if roleStr, ok := role.(string); ok && roleStr == "developer" {
					email := item.GetString("email")
					fmt.Printf("   - %s (%s) is an active developer\n", name, email)
					break
				}
			}
		}
	})

	if err != nil {
		fmt.Printf("   Error: %v\n", err)
	}
}
