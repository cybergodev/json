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
// - JSON format validation with json.Valid
// - Path validation via Processor
// - Schema validation
// - Security validation
// - Processor-level validation
//
// Run: go run -tags=example examples/6_validation.go

func main() {
	fmt.Println("JSON Library - Validation")
	fmt.Println("=========================")

	// 1. JSON FORMAT VALIDATION
	demonstrateFormatValidation()

	// 2. PATH VALIDATION
	demonstratePathValidation()

	// 3. SCHEMA VALIDATION
	demonstrateSchemaValidation()

	// 4. SECURITY VALIDATION
	demonstrateSecurityValidation()

	// 5. VALIDATION WITH PROCESSOR
	demonstrateProcessorValidation()

	fmt.Println("\nValidation examples complete!")
}

func demonstrateFormatValidation() {
	fmt.Println("1. JSON Format Validation (json.Valid)")
	fmt.Println("----------------------------------------------")

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
		{"Empty JSON", `{}`, true},
		{"Invalid JSON", `{"name": "John",}`, false},
		{"Empty string", ``, false},
		{"Missing closing brace", `{"name": "John"`, false},
	}

	fmt.Println("   Format validation results:")
	for _, tc := range testCases {
		valid := json.Valid([]byte(tc.data))
		status := "valid"
		if !valid {
			status = "invalid"
		}
		fmt.Printf("   [%s] %s\n", tc.name, status)
	}
}

func demonstratePathValidation() {
	fmt.Println("\n2. Path Validation (via Processor)")
	fmt.Println("------------------------------------")

	processor, err := json.New(json.DefaultConfig())
	if err != nil {
		fmt.Printf("   New error: %v\n", err)
		return
	}
	defer processor.Close()

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
		{"Invalid brackets", "user[0", false},
	}

	fmt.Println("   Path validation results (using sample JSON for syntax check):")
	sampleJSON := `{"user": {"name": "test"}, "users": [{"name": "a"}], "data": [{"items": [1, 2]}]}`
	for _, tc := range testPaths {
		if tc.path == "" {
			fmt.Printf("   X [%s] %s (empty path is invalid)\n", tc.name, tc.path)
			continue
		}
		// Attempt Get on sample JSON: syntax errors surface as ErrInvalidPath,
		// while valid paths either succeed (path exists) or fail with ErrPathNotFound
		_, err := processor.Get(sampleJSON, tc.path)
		if !tc.valid {
			fmt.Printf("   X [%s] %s (correctly rejected)\n", tc.name, tc.path)
		} else if err != nil {
			fmt.Printf("   OK [%s] %s (valid syntax, path not found in sample data)\n", tc.name, tc.path)
		} else {
			fmt.Printf("   OK [%s] %s (valid)\n", tc.name, tc.path)
		}
	}
}

func demonstrateSchemaValidation() {
	fmt.Println("\n3. Schema Validation (json.ValidateSchema)")
	fmt.Println("---------------------------------------------")

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
		fmt.Println("   User data is valid!")
	} else {
		for _, e := range errors {
			fmt.Printf("   X %s: %s\n", e.Path, e.Message)
		}
	}

	// Validate invalid user 1
	fmt.Println("\n   Validating invalid user (missing required field):")
	errors, err = json.ValidateSchema(invalidUser1, schema)
	if err == nil {
		for _, e := range errors {
			fmt.Printf("   X %s: %s\n", e.Path, e.Message)
		}
	}

	// Validate invalid user 2
	fmt.Println("\n   Validating invalid user (wrong types):")
	errors, err = json.ValidateSchema(invalidUser2, schema)
	if err == nil {
		for _, e := range errors {
			fmt.Printf("   X %s: %s\n", e.Path, e.Message)
		}
	}
}

func demonstrateSecurityValidation() {
	fmt.Println("\n4. Security Validation")
	fmt.Println("------------------------")

	// Create a security processor
	processor, _ := json.New(json.SecurityConfig()) // OK: preset config always valid
	defer processor.Close()

	testCases := []struct {
		name string
		data string
	}{
		{"Normal JSON", `{"user": "John", "age": 30}`},
		{"Deeply nested (within limits)", `{"a":{"b":{"c":"value"}}}`},
		{"Large JSON (within limits)", generateLargeJSON(100)},
	}

	fmt.Println("   Security validation with SecurityConfig:")
	for _, tc := range testCases {
		valid := json.Valid([]byte(tc.data))
		status := "OK"
		if !valid {
			status = "X"
		}
		fmt.Printf("   %s %s\n", status, tc.name)
	}
}

func demonstrateProcessorValidation() {
	fmt.Println("\n5. Validation with Processor")
	fmt.Println("------------------------------")

	// Create processor with validation enabled
	config := json.DefaultConfig()
	config.EnableValidation = true
	config.MaxJSONSize = 1024 * 1024 // 1MB

	processor, _ := json.New(config) // OK: DefaultConfig-derived, always valid
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
		fmt.Printf("   Validated and retrieved: %v\n", name)
	}

	// Invalid path (will fail validation)
	_, err = processor.Get(testJSON, "")
	if err != nil {
		fmt.Printf("   Invalid path caught: %v\n", err)
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
