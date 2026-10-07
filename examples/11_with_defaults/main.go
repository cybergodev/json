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
// - GetTyped[T] — type-safe read returning T directly (no error)
// - Practical use cases
//
// Run: go run -tags=example ./examples/11_with_defaults

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

	// 1. GETTYPED[T]
	demonstrateGetTyped(partialData, completeData)

	// 2. PRACTICAL USE CASES
	demonstratePracticalCases()

	fmt.Println("\nWith defaults examples complete!")
}

func demonstrateGetTyped(partialData, completeData string) {
	fmt.Println("1. GetTyped[T] (type-safe, with default)")
	fmt.Println("-----------------------------------------")

	// GetTyped[T] returns T directly — not (T, error). When the path is missing,
	// the value is null, or conversion fails, it returns the default (or the
	// zero value of T if no default is given). That makes it ideal for optional
	// fields: no error handling is needed for the common "missing" case.

	// String
	fmt.Println("   Strings:")
	fmt.Printf("   - user.email:           %s\n", json.GetTyped(partialData, "user.email", "no-email@example.com"))
	fmt.Printf("   - user.phone (missing): %s\n", json.GetTyped(partialData, "user.phone", "N/A"))

	// Int — missing returns the default; present returns the actual value.
	fmt.Println("\n   Ints (default vs actual):")
	fmt.Printf("   - user.age (missing, default 18):        %d\n", json.GetTyped(partialData, "user.age", 18))
	fmt.Printf("   - user.age (complete, default ignored): %d\n", json.GetTyped(completeData, "user.age", 18))

	// Bool / Float / Array
	fmt.Println("\n   Bool / Float / Array:")
	fmt.Printf("   - settings.notifications (missing): %t\n", json.GetTyped(partialData, "settings.notifications", false))
	fmt.Printf("   - user.score (missing):             %.1f\n", json.GetTyped(partialData, "user.score", 100.0))
	tags := json.GetTyped[[]any](partialData, "user.tags", []any{})
	fmt.Printf("   - user.tags (missing):               %v (len %d)\n", tags, len(tags))
}

func demonstratePracticalCases() {
	fmt.Println("\n2. Practical Use Cases")
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
