//go:build example

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
// - GetTyped[T] - generic method with default values
// - Practical use cases
//
// Run: go run -tags=example examples/11_with_defaults.go

func main() {
	fmt.Println("JSON Library - With Defaults")
	fmt.Println("================================")

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

	// 1. GETTYPED[T] - RECOMMENDED
	demonstrateGetTyped(partialData, completeData)

	// 2. TYPED DEFAULTS IN PRACTICE
	demonstrateTypedDefaultsPractice(partialData, completeData)

	// 3. PRACTICAL USE CASES
	demonstratePracticalCases()

	fmt.Println("\nWith defaults examples complete!")
}

func demonstrateGetTyped(partialData, completeData string) {
	fmt.Println("1. GetTyped[T] (Recommended)")
	fmt.Println("------------------------------")

	// String with default - RECOMMENDED approach
	email := json.GetTyped(partialData, "user.email", "no-email@example.com")
	fmt.Printf("   user.email: %s\n", email)

	missingPhone := json.GetTyped(partialData, "user.phone", "N/A")
	fmt.Printf("   user.phone (missing): %s\n", missingPhone)

	// Int with default
	age := json.GetTyped(partialData, "user.age", 0)
	fmt.Printf("   user.age (missing): %d\n", age)

	completeAge := json.GetTyped(completeData, "user.age", 0)
	fmt.Printf("   user.age (from complete): %d\n", completeAge)

	// Bool with default
	notifications := json.GetTyped(partialData, "settings.notifications", false)
	fmt.Printf("   settings.notifications (missing): %t\n", notifications)

	// Float with default
	score := json.GetTyped(partialData, "user.score", 100.0)
	fmt.Printf("   user.score (missing): %.1f\n", score)

	// Array with default
	tags := json.GetTyped[[]any](partialData, "user.tags", []any{})
	fmt.Printf("   user.tags (missing): %v (length: %d)\n", tags, len(tags))
}

func demonstrateTypedDefaultsPractice(partialData, completeData string) {
	fmt.Println("\n2. GetTyped[T] (generic, type-safe)")
	fmt.Println("-------------------------------------")

	// Missing field with default
	missingPath := "user.age"
	defaultAge := 18

	age := json.GetTyped(partialData, missingPath, defaultAge)
	fmt.Printf("   Missing field '%s': %v (default: %d)\n", missingPath, age, defaultAge)

	// Existing field (returns actual value, not default)
	existingPath := "user.name"
	defaultName := "Unknown"

	name := json.GetTyped(partialData, existingPath, defaultName)
	fmt.Printf("   Existing field '%s': %v (default ignored)\n", existingPath, name)

	// Nested path with default
	missingNested := "settings.notifications"
	defaultNotif := false

	notifications := json.GetTyped(partialData, missingNested, defaultNotif)
	fmt.Printf("   Missing nested '%s': %v (default: %t)\n", missingNested, notifications, defaultNotif)

	// Show difference with complete data
	fmt.Println("\n   With complete data:")
	completeAge := json.GetTyped(completeData, missingPath, defaultAge)
	fmt.Printf("   Field '%s': %v (actual value, default ignored)\n", missingPath, completeAge)
}

func demonstratePracticalCases() {
	fmt.Println("\n3. Practical Use Cases")
	fmt.Println("-----------------------")

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

	// Extract with defaults using GetTyped[T] (recommended)
	config := Config{
		Host:         json.GetTyped(configJSON, "server.host", "0.0.0.0"),
		Port:         json.GetTyped(configJSON, "server.port", 8080),
		Debug:        json.GetTyped(configJSON, "debug", false),
		MaxConn:      json.GetTyped(configJSON, "max_connections", 100),
		ReadTimeout:  json.GetTyped(configJSON, "read_timeout", 30),
		WriteTimeout: json.GetTyped(configJSON, "write_timeout", 30),
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

	// Extract with defaults for optional fields using GetTyped[T]
	status := json.GetTyped(apiResponse, "status", "unknown")
	productID := json.GetTyped(apiResponse, "data.id", 0)
	name := json.GetTyped(apiResponse, "data.name", "Unnamed Product")
	description := json.GetTyped(apiResponse, "data.description", "No description available")
	price := json.GetTyped(apiResponse, "data.price", 0.0)

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
		"new_ui":        json.GetTyped(featuresJSON, "new_ui", false),
		"beta_features": json.GetTyped(featuresJSON, "beta_features", false),
		"experimental":  json.GetTyped(featuresJSON, "experimental", false),
		"analytics":     json.GetTyped(featuresJSON, "analytics", true),
		"notifications": json.GetTyped(featuresJSON, "notifications", true),
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
