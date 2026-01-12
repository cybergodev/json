//go:build ignore

package main

import (
	"fmt"

	"github.com/cybergodev/json"
)

// With Defaults Example
//
// This example demonstrates using default values with JSON operations
// to handle missing or null values gracefully.
//
// Topics covered:
// - GetWithDefault for any type
// - GetTypedWithDefault for type-safe defaults
// - Type-specific WithDefault methods
// - Practical use cases
//
// Run: go run examples/11_with_defaults.go

func main() {
	fmt.Println("🎯 JSON Library - With Defaults")
	fmt.Println("===============================\n ")

	// Sample data with some missing/optional fields
	partialData := `{
		"user": {
			"name": "Alice",
			"email": "alice@example.com"
		},
		"settings": {
			"theme": "dark"
		}
	}`

	completeData := `{
		"user": {
			"name": "Bob",
			"email": "bob@example.com",
			"age": 30
		},
		"settings": {
			"theme": "light",
			"notifications": true,
			"language": "en"
		}
	}`

	// 1. GETWITHDEFAULT
	fmt.Println("1️⃣  GetWithDefault (any type)")
	fmt.Println("─────────────────────────────")
	demonstrateGetWithDefault(partialData, completeData)

	// 2. GETTYPEDWITHDEFAULT
	fmt.Println("\n2️⃣  GetTypedWithDefault (type-safe)")
	fmt.Println("──────────────────────────────────")
	demonstrateGetTypedWithDefault(partialData, completeData)

	// 3. TYPE-SPECIFIC WITHDEFAULT
	fmt.Println("\n3️⃣  Type-Specific WithDefault Methods")
	fmt.Println("────────────────────────────────────")
	demonstrateTypeSpecificDefaults(partialData, completeData)

	// 4. PRACTICAL USE CASES
	fmt.Println("\n4️⃣  Practical Use Cases")
	fmt.Println("──────────────────────")
	demonstratePracticalCases()

	fmt.Println("\n✅ With defaults examples complete!")
}

func demonstrateGetWithDefault(partialData, completeData string) {
	// Missing field with default
	missingPath := "user.age"
	defaultAge := 18

	age := json.GetWithDefault(partialData, missingPath, defaultAge)
	fmt.Printf("   Missing field '%s': %v (default: %d)\n", missingPath, age, defaultAge)

	// Existing field (returns actual value, not default)
	existingPath := "user.name"
	defaultName := "Unknown"

	name := json.GetWithDefault(partialData, existingPath, defaultName)
	fmt.Printf("   Existing field '%s': %v (default ignored)\n", existingPath, name)

	// Nested path with default
	missingNested := "settings.notifications"
	defaultNotif := false

	notifications := json.GetWithDefault(partialData, missingNested, defaultNotif)
	fmt.Printf("   Missing nested '%s': %v (default: %t)\n", missingNested, notifications, defaultNotif)

	// Show difference with complete data
	fmt.Println("\n   With complete data:")
	completeAge := json.GetWithDefault(completeData, missingPath, defaultAge)
	fmt.Printf("   Field '%s': %v (actual value, default ignored)\n", missingPath, completeAge)
}

func demonstrateGetTypedWithDefault(partialData, completeData string) {
	// String with default
	email := json.GetTypedWithDefault(partialData, "user.email", "no-email@example.com")
	fmt.Printf("   user.email: %s\n", email)

	missingEmail := json.GetTypedWithDefault(partialData, "user.phone", "N/A")
	fmt.Printf("   user.phone (missing): %s\n", missingEmail)

	// Int with default
	age := json.GetTypedWithDefault(partialData, "user.age", 0)
	fmt.Printf("   user.age (missing): %d\n", age)

	completeAge := json.GetTypedWithDefault(completeData, "user.age", 0)
	fmt.Printf("   user.age (from complete): %d\n", completeAge)

	// Bool with default
	notifications := json.GetTypedWithDefault(partialData, "settings.notifications", false)
	fmt.Printf("   settings.notifications (missing): %t\n", notifications)

	completeNotif := json.GetTypedWithDefault(completeData, "settings.notifications", false)
	fmt.Printf("   settings.notifications (from complete): %t\n", completeNotif)

	// Float with default
	score := json.GetTypedWithDefault(partialData, "user.score", 100.0)
	fmt.Printf("   user.score (missing): %.1f\n", score)

	// Array with default
	tags := json.GetTypedWithDefault[[]interface{}](partialData, "user.tags", []interface{}{})
	fmt.Printf("   user.tags (missing): %v (length: %d)\n", tags, len(tags))
}

func demonstrateTypeSpecificDefaults(partialData, completeData string) {
	// String with default
	name := json.GetStringWithDefault(partialData, "user.name", "Anonymous")
	fmt.Printf("   GetStringWithDefault - name: %s\n", name)

	missingPhone := json.GetStringWithDefault(partialData, "user.phone", "N/A")
	fmt.Printf("   GetStringWithDefault - phone: %s\n", missingPhone)

	// Int with default
	age := json.GetIntWithDefault(partialData, "user.age", 18)
	fmt.Printf("   GetIntWithDefault - age: %d\n", age)

	completeAge := json.GetIntWithDefault(completeData, "user.age", 18)
	fmt.Printf("   GetIntWithDefault - age (complete): %d\n", completeAge)

	// Float64 with default
	price := json.GetFloat64WithDefault(partialData, "product.price", 0.0)
	fmt.Printf("   GetFloat64WithDefault - price: %.2f\n", price)

	// Bool with default
	active := json.GetBoolWithDefault(partialData, "user.active", true)
	fmt.Printf("   GetBoolWithDefault - active: %t\n", active)

	// Array with default
	tags := json.GetArrayWithDefault(partialData, "user.tags", []interface{}{})
	fmt.Printf("   GetArrayWithDefault - tags: %v\n", tags)

	// Object with default
	settings := json.GetObjectWithDefault(partialData, "settings", map[string]interface{}{})
	fmt.Printf("   GetObjectWithDefault - settings: %v\n", settings)

	missingSettings := json.GetObjectWithDefault(partialData, "preferences", map[string]interface{}{})
	fmt.Printf("   GetObjectWithDefault - preferences (missing): %v\n", missingSettings)
}

func demonstratePracticalCases() {
	// Use case 1: Configuration with sensible defaults
	configJSON := `{
		"server": {
			"host": "localhost"
		}
	}`

	fmt.Println("   Use Case 1: Configuration defaults")

	type Config struct {
		Host         string
		Port         int
		Debug        bool
		MaxConn      int
		ReadTimeout  int
		WriteTimeout int
	}

	// Extract with defaults
	config := Config{
		Host:         json.GetStringWithDefault(configJSON, "server.host", "0.0.0.0"),
		Port:         json.GetIntWithDefault(configJSON, "server.port", 8080),
		Debug:        json.GetBoolWithDefault(configJSON, "debug", false),
		MaxConn:      json.GetIntWithDefault(configJSON, "max_connections", 100),
		ReadTimeout:  json.GetIntWithDefault(configJSON, "read_timeout", 30),
		WriteTimeout: json.GetIntWithDefault(configJSON, "write_timeout", 30),
	}

	fmt.Printf("   Config: %+v\n", config)

	// Use case 2: API response handling
	fmt.Println("\n   Use Case 2: API response with optional fields")

	apiResponse := `{
		"status": "success",
		"data": {
			"id": 123,
			"name": "Product Name"
		}
	}`

	// Extract with defaults for optional fields
	status := json.GetStringWithDefault(apiResponse, "status", "unknown")
	productID := json.GetIntWithDefault(apiResponse, "data.id", 0)
	name := json.GetStringWithDefault(apiResponse, "data.name", "Unnamed Product")
	description := json.GetStringWithDefault(apiResponse, "data.description", "No description available")
	price := json.GetFloat64WithDefault(apiResponse, "data.price", 0.0)

	fmt.Printf("   Status: %s\n", status)
	fmt.Printf("   Product: %s (ID: %d)\n", name, productID)
	fmt.Printf("   Description: %s\n", description)
	fmt.Printf("   Price: $%.2f\n", price)

	// Use case 3: Feature flags
	fmt.Println("\n   Use Case 3: Feature flags with defaults")

	featuresJSON := `{
		"new_ui": true
	}`

	features := map[string]bool{
		"new_ui":        json.GetBoolWithDefault(featuresJSON, "new_ui", false),
		"beta_features": json.GetBoolWithDefault(featuresJSON, "beta_features", false),
		"experimental":  json.GetBoolWithDefault(featuresJSON, "experimental", false),
		"analytics":     json.GetBoolWithDefault(featuresJSON, "analytics", true),
		"notifications": json.GetBoolWithDefault(featuresJSON, "notifications", true),
	}

	fmt.Println("   Feature flags:")
	for name, enabled := range features {
		status := "disabled"
		if enabled {
			status = "enabled"
		}
		fmt.Printf("   - %s: %s\n", name, status)
	}
}
