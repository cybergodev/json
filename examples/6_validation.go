//go:build example

package main

import (
	"fmt"

	"github.com/cybergodev/json"
)

// Validation Example
//
// This example demonstrates JSON validation capabilities including schema validation,
// security validation, and path validation.
//
// Topics covered:
// - JSON schema validation
// - Security validation (nesting depth, size limits)
// - Path validation
// - JSON format validation
// - Custom validation rules
//
// Run: go run examples/6_validation.go

func main() {
	fmt.Println("✓ JSON Library - Validation")
	fmt.Println("===========================\n ")

	// 1. JSON FORMAT VALIDATION
	fmt.Println("1️⃣  JSON Format Validation")
	fmt.Println("─────────────────────────")
	demonstrateFormatValidation()

	// 2. PATH VALIDATION
	fmt.Println("\n2️⃣  Path Validation")
	fmt.Println("───────────────────")
	demonstrateValidationPath()

	// 3. SCHEMA VALIDATION
	fmt.Println("\n3️⃣  Schema Validation")
	fmt.Println("────────────────────")
	demonstrateSchemaValidation()

	// 4. SECURITY VALIDATION
	fmt.Println("\n4️⃣  Security Validation")
	fmt.Println("───────────────────────")
	demonstrateSecurityValidation()

	// 5. VALIDATION WITH PROCESSOR
	fmt.Println("\n5️⃣  Validation with Processor")
	fmt.Println("────────────────────────────")
	demonstrateProcessorValidation()

	fmt.Println("\n✅ Validation examples complete!")
}

func demonstrateFormatValidation() {
	testCases := []struct {
		name  string
		data  string
		valid bool
	}{
		{"Valid object", `{"name": "John", "age": 30}`, true},
		{"Valid array", `[1, 2, 3]`, true},
		{"Valid string", `"hello"`, true},
		{"Valid number", `42`, true},
		{"Valid boolean", `true`, true},
		{"Invalid JSON", `{"name": "John",}`, false},
		{"Empty string", ``, false},
		{"Missing closing brace", `{"name": "John"`, false},
	}

	fmt.Println("   Format validation results:")
	for _, tc := range testCases {
		valid := json.IsValidJSON(tc.data)
		status := "✓ valid"
		if !valid {
			status = "✗ invalid"
		}
		fmt.Printf("   [%s] %s\n", tc.name, status)
	}
}

func demonstrateValidationPath() {
	testPaths := []struct {
		name  string
		path  string
		valid bool
	}{
		{"Root path", ".", true},
		{"Simple property", "user.name", true},
		{"Array index", "users[0]", true},
		{"Nested array", "data[0].items[1]", true},
		{"Extraction", "users{name}", true},
		{"Empty path", "", false},
		{"Path traversal", "../etc/passwd", false},
		{"Invalid brackets", "user[0", false},
	}

	fmt.Println("   Path validation results:")
	for _, tc := range testPaths {
		valid := json.IsValidPath(tc.path)
		status := "✓"
		if !valid {
			status = "✗"
		}
		fmt.Printf("   %s [%s] %s\n", status, tc.name, tc.path)
	}

	// Detailed validation with error messages
	fmt.Println("\n   Detailed validation:")
	invalidPath := "users..name"
	if err := json.ValidatePath(invalidPath); err != nil {
		fmt.Printf("   Path '%s' error: %v\n", invalidPath, err)
	}
}

func demonstrateSchemaValidation() {
	// Create a schema for user data
	schema := &json.Schema{
		Type:     "object",
		Required: []string{"name", "email"},
		Properties: map[string]*json.Schema{
			"name": {
				Type:      "string",
				MinLength: 2,
				MaxLength: 50,
			},
			"email": {
				Type:   "string",
				Format: "email",
			},
			"age": {
				Type:    "number",
				Minimum: 0,
				Maximum: 150,
			},
			"tags": {
				Type:     "array",
				MinItems: 0,
				MaxItems: 10,
			},
		},
	}

	// Valid user data
	validUser := `{
		"name": "John Doe",
		"email": "john@example.com",
		"age": 30,
		"tags": ["developer", "golang"]
	}`

	// Invalid user data (missing required field)
	invalidUser1 := `{
		"name": "Jane Doe",
		"age": 25
	}`

	// Invalid user data (wrong type)
	invalidUser2 := `{
		"name": "Bob",
		"email": "not-an-email",
		"age": "thirty"
	}`

	// Validate valid user
	fmt.Println("   Validating valid user:")
	errors, err := json.ValidateSchema(validUser, schema)
	if err != nil {
		fmt.Printf("   Validation error: %v\n", err)
	} else if len(errors) == 0 {
		fmt.Println("   ✓ User data is valid!")
	} else {
		for _, e := range errors {
			fmt.Printf("   ✗ %s: %s\n", e.Path, e.Message)
		}
	}

	// Validate invalid user 1
	fmt.Println("\n   Validating invalid user (missing required field):")
	errors, err = json.ValidateSchema(invalidUser1, schema)
	if err == nil {
		for _, e := range errors {
			fmt.Printf("   ✗ %s: %s\n", e.Path, e.Message)
		}
	}

	// Validate invalid user 2
	fmt.Println("\n   Validating invalid user (wrong types):")
	errors, err = json.ValidateSchema(invalidUser2, schema)
	if err == nil {
		for _, e := range errors {
			fmt.Printf("   ✗ %s: %s\n", e.Path, e.Message)
		}
	}
}

func demonstrateSecurityValidation() {
	// Create a high-security processor
	processor := json.New(json.HighSecurityConfig())
	defer processor.Close()

	testCases := []struct {
		name  string
		data  string
		valid bool
	}{
		{
			"Normal JSON",
			`{"user": "John", "age": 30}`,
			true,
		},
		{
			"Deeply nested (within limits)",
			`{"a":{"b":{"c":"value"}}}`,
			true,
		},
		{
			"Large JSON (within limits)",
			generateLargeJSON(100),
			true,
		},
	}

	fmt.Println("   Security validation with HighSecurityConfig:")
	for _, tc := range testCases {
		valid := json.IsValidJSON(tc.data)
		status := "✓"
		if !valid {
			status = "✗"
		}
		fmt.Printf("   %s %s\n", status, tc.name)
	}
}

func demonstrateProcessorValidation() {
	// Create processor with validation enabled
	config := json.DefaultConfig()
	config.EnableValidation = true
	config.MaxJSONSize = 1024 * 1024 // 1MB

	processor := json.New(config)
	defer processor.Close()

	testJSON := `{
		"user": {
			"name": "Alice",
			"email": "alice@example.com",
			"preferences": {
				"theme": "dark",
				"notifications": true
			}
		}
	}`

	// Get with validation
	name, err := processor.Get(testJSON, "user.name")
	if err != nil {
		fmt.Printf("   Error: %v\n", err)
	} else {
		fmt.Printf("   ✓ Validated and retrieved: %v\n", name)
	}

	// Invalid path (will fail validation)
	_, err = processor.Get(testJSON, "")
	if err != nil {
		fmt.Printf("   ✓ Invalid path caught: %v\n", err)
	}
}

// Helper function to generate large JSON for testing
func generateLargeJSON(size int) string {
	result := "{"
	for i := 0; i < size; i++ {
		if i > 0 {
			result += ","
		}
		result += fmt.Sprintf("\"field%d\": \"value%d\"", i, i)
	}
	result += "}"
	return result
}
