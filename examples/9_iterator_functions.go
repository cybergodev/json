//go:build ignore

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

	// NOTE: IterableValue is designed to be used within iteration callbacks
	// where it's automatically created with the correct internal state.
	// This example shows how to use it properly.

	// Instead of manually creating IterableValue, iterate over the specific path
	fmt.Println("   Using ForeachWithPath to access IterableValue API:")

	err := json.ForeachWithPath(data, "users[0]", func(key any, item *json.IterableValue) {
		// Now we can use all the IterableValue methods
		name := item.GetString("name")
		email := item.GetString("email")
		active := item.GetBool("active")
		id := item.GetInt("id")

		fmt.Printf("   - GetString: name=%s, email=%s\n", name, email)
		fmt.Printf("   - GetInt: id=%d\n", id)
		fmt.Printf("   - GetBool: active=%t\n", active)

		// GetWithDefault
		nonExistent := item.GetStringWithDefault("nonexistent", "default value")
		fmt.Printf("   - GetStringWithDefault: %s\n", nonExistent)

		// Check existence
		fmt.Println("\n   Checking field existence:")
		fmt.Printf("   - Exists('name'): %t\n", item.Exists("name"))
		fmt.Printf("   - Exists('missing'): %t\n", item.Exists("missing"))

		// Check for null
		fmt.Println("\n   Null checks:")
		fmt.Printf("   - IsNull('name'): %t\n", item.IsNull("name"))
		fmt.Printf("   - IsNull('missing'): %t\n", item.IsNull("missing"))

		// Check for empty
		fmt.Println("\n   Empty checks:")
		fmt.Printf("   - IsEmpty('email'): %t\n", item.IsEmpty("email"))
		roles := item.GetArray("roles")
		fmt.Printf("   - IsEmpty('roles'): %t (length: %d)\n", item.IsEmpty("roles"), len(roles))

		// GetArray
		fmt.Println("\n   GetArray:")
		roles = item.GetArray("roles")
		fmt.Printf("   - Roles: %v\n", roles)
	})

	if err != nil {
		fmt.Printf("   Error: %v\n", err)
	}

	// Access settings object
	fmt.Println("\n   Accessing nested object:")
	err = json.ForeachWithPath(data, "settings", func(key any, item *json.IterableValue) {
		theme := item.GetString("theme")
		notifications := item.GetBool("notifications")
		language := item.GetString("language")

		fmt.Printf("   - Theme: %s\n", theme)
		fmt.Printf("   - Notifications: %t\n", notifications)
		fmt.Printf("   - Language: %s\n", language)
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
