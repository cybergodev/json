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
// - Mutation with ForeachReturn
// - Error-returning variants (ForeachWithError, ForeachNestedWithError)
// - Per-element paths with ForeachWithPathAndIterator
//
// Run: go run -tags=example examples/9_iterator_functions.go

func main() {
	fmt.Println("JSON Library - Iterator Functions")
	fmt.Println("===================================")

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
	demonstrateSimpleIteration(sampleData)

	// 2. ITERATION WITH PATH
	demonstrateIterationWithPath(sampleData)

	// 3. NESTED ITERATION
	demonstrateNestedIteration(sampleData)

	// 4. ITERABLE VALUE API
	demonstrateIterableValueAPI(sampleData)

	// 5. TRANSFORMATION
	demonstrateTransformation(sampleData)

	// 6. MUTATION WITH FOREACHRETURN
	demonstrateMutation(sampleData)

	// 7. ERROR-RETURNING VARIANTS
	demonstrateErrorIterators(sampleData)

	// 8. PER-ELEMENT PATHS
	demonstratePathIteration(sampleData)

	fmt.Println("\nIterator functions examples complete!")
}

func demonstrateSimpleIteration(data string) {
	fmt.Println("1. Simple Iteration (Foreach)")
	fmt.Println("--------------------------------")

	fmt.Println("   Iterating over entire JSON:")

	// ForeachWithError instead of the deprecated void Foreach, so iteration
	// failures surface instead of silently skipping the callback.
	err := json.ForeachWithError(data, ".", func(key any, item *json.IterableValue) error {
		// Top-level iteration
		fmt.Printf("   Key: %v, Type: %T\n", key, item.Get(""))
		return nil
	})
	if err != nil {
		fmt.Printf("   Error: %v\n", err)
	}
}

func demonstrateIterationWithPath(data string) {
	fmt.Println("\n2. Iteration with Path (ForeachWithPath)")
	fmt.Println("------------------------------------------")

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
	fmt.Println("\n3. Nested Iteration (ForeachNested)")
	fmt.Println("---------------------------------------")

	fmt.Println("   Recursively iterating all values:")

	// ForeachNestedWithError instead of the deprecated void ForeachNested.
	count := 0
	err := json.ForeachNestedWithError(data, func(key any, item *json.IterableValue) error {
		count++
		return nil
	})
	if err != nil {
		fmt.Printf("   Error: %v\n", err)
	}

	fmt.Printf("   Total values visited (including nested): %d\n", count)

	// Count specific types
	fmt.Println("\n   Counting by type:")

	// JSON numbers always decode to float64, so the numeric case matches that.
	numCount := 0
	strCount := 0
	boolCount := 0

	err = json.ForeachNestedWithError(data, func(key any, item *json.IterableValue) error {
		switch item.Get("").(type) {
		case float64:
			numCount++
		case string:
			strCount++
		case bool:
			boolCount++
		}
		return nil
	})
	if err != nil {
		fmt.Printf("   Error: %v\n", err)
	}

	fmt.Printf("   Numbers: %d, Strings: %d, Booleans: %d\n", numCount, strCount, boolCount)
}

func demonstrateIterableValueAPI(data string) {
	fmt.Println("\n4. IterableValue API")
	fmt.Println("----------------------")

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
	fmt.Println("\n5. Data Transformation with Iteration")
	fmt.Println("----------------------------------------")

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

	// Find all active developers (one line per matching user).
	fmt.Println("\n   Finding all active developers:")
	err = json.ForeachWithPath(data, "users", func(key any, item *json.IterableValue) {
		if !item.GetBool("active") {
			return
		}
		for _, role := range item.GetArray("roles") {
			if roleStr, ok := role.(string); ok && roleStr == "developer" {
				// This break only exits the inner range — it avoids printing the
				// same user twice if they hold "developer" more than once. It does
				// NOT stop the outer iteration. For early termination, see below.
				fmt.Printf("   - %s (%s) is an active developer\n",
					item.GetString("name"), item.GetString("email"))
				break
			}
		}
	})
	if err != nil {
		fmt.Printf("   Error: %v\n", err)
	}

	// Find the FIRST active developer and stop early with IteratorBreak.
	// ForeachWithPathAndControl hands the callback a flow-control return value:
	// IteratorBreak stops iteration entirely (unlike `break`, which would only
	// exit an inner loop). The callback receives the raw value (not an
	// IterableValue), so we type-assert the user map directly.
	fmt.Println("\n   Finding first active developer (early exit with IteratorBreak):")
	err = json.ForeachWithPathAndControl(data, "users", func(key any, value any) json.IteratorControl {
		m, ok := value.(map[string]any)
		if !ok {
			return json.IteratorContinue
		}
		if active, _ := m["active"].(bool); !active {
			return json.IteratorContinue
		}
		roles, _ := m["roles"].([]any)
		for _, r := range roles {
			if rs, _ := r.(string); rs == "developer" {
				name, _ := m["name"].(string)
				email, _ := m["email"].(string)
				fmt.Printf("   - First match: %s (%s)\n", name, email)
				return json.IteratorBreak
			}
		}
		return json.IteratorContinue
	})
	if err != nil {
		fmt.Printf("   Error: %v\n", err)
	}
}

func demonstrateMutation(data string) {
	fmt.Println("\n6. Mutation with ForeachReturn")
	fmt.Println("----------------------------------")

	// ForeachReturn re-marshals the iterated data after the callback, so
	// mutations made through item.GetData() — a reference to the working
	// copy — show up in the returned JSON. IterableValue itself is read-only
	// (no Set), so the pattern is: assert GetData() to map/slice and write
	// through it. Replacing a top-level scalar in place is not possible.
	result, err := json.ForeachReturn(data, func(key any, item *json.IterableValue) {
		switch key {
		case "settings": // flip a nested map entry
			if settings, ok := item.GetData().(map[string]any); ok {
				settings["theme"] = "light"
			}
		case "users": // stamp every array element
			if users, ok := item.GetData().([]any); ok {
				for _, u := range users {
					if m, ok := u.(map[string]any); ok {
						m["active"] = true
					}
				}
			}
		}
	})
	if err != nil {
		fmt.Printf("   Error: %v\n", err)
		return
	}
	theme := json.GetString(result, "settings.theme", "")
	fmt.Printf("   settings.theme after mutation: %s\n", theme)
	fmt.Printf("   users[1].active after mutation: %t\n", json.GetBool(result, "users[1].active", false))
}

func demonstrateErrorIterators(data string) {
	fmt.Println("\n7. Error-Returning Variants")
	fmt.Println("------------------------------")

	// ForeachWithError: the callback returns an error to control iteration.
	// item.Break() stops WITHOUT reporting an error; any other error stops
	// and propagates. (Plain Foreach callbacks cannot stop or fail.)
	stoppedAt := ""
	err := json.ForeachWithError(data, "users", func(key any, item *json.IterableValue) error {
		if !item.GetBool("active") {
			stoppedAt = item.GetString("name")
			return item.Break() // stop, err stays nil
		}
		return nil // continue
	})
	fmt.Printf("   ForeachWithError: err=%v, stopped at %q via Break()\n", err, stoppedAt)

	// ForeachNestedWithError: recursive traversal with the same contract —
	// a non-Break error aborts the whole walk and is returned to the caller.
	err = json.ForeachNestedWithError(data, func(key any, item *json.IterableValue) error {
		if s, ok := item.GetData().(string); ok && s == "dark" {
			return fmt.Errorf("unexpected theme %q at key %v", s, key)
		}
		return nil
	})
	fmt.Printf("   ForeachNestedWithError propagated: %v\n", err)
}

func demonstratePathIteration(data string) {
	fmt.Println("\n8. Per-Element Paths (ForeachWithPathAndIterator)")
	fmt.Println("---------------------------------------------------")

	// Like ForeachWithPathAndControl, but the callback receives an
	// IterableValue AND the path of the current element (relative to the
	// iterated path — "[0]", "[1]", ... for arrays), plus IteratorControl.
	count := 0
	err := json.ForeachWithPathAndIterator(data, "users", func(key any, item *json.IterableValue, currentPath string) json.IteratorControl {
		fmt.Printf("   - path %-4s name=%-8s (active=%t)\n",
			currentPath, item.GetString("name"), item.GetBool("active"))
		count++
		if count >= 2 {
			return json.IteratorBreak
		}
		return json.IteratorContinue
	})
	if err != nil {
		fmt.Printf("   Error: %v\n", err)
		return
	}
	fmt.Printf("   Visited %d elements before IteratorBreak\n", count)
}
