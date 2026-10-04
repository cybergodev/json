//go:build example

package main

import (
	"errors"
	"fmt"
	"strings"

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

	// 6. PER-CALL CONFIG ENFORCEMENT (PHASE 2)
	demonstratePerCallConfigEnforcement()

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

	// ValidWithConfig is the error-returning variant: it reports WHY a
	// document was rejected instead of just false. Here a syntactically valid
	// document is declined because it exceeds a per-call size limit.
	valid, err := json.ValidWithConfig(`{"user": "John"}`, json.SecurityConfig())
	if err != nil {
		fmt.Printf("   ValidWithConfig (SecurityConfig): rejected -> %v\n", err)
	} else {
		fmt.Printf("   ValidWithConfig (SecurityConfig): valid=%t\n", valid)
	}
}

func demonstratePathValidation() {
	fmt.Println("\n2. Path Syntax Validation (via Processor)")
	fmt.Println("-------------------------------------------")

	processor, err := json.New(json.DefaultConfig())
	if err != nil {
		fmt.Printf("   New error: %v\n", err)
		return
	}
	defer processor.Close()

	sampleJSON := `{"user": {"name": "test"}, "users": [{"name": "a"}], "data": [{"items": [1, 2]}]}`

	// The package exposes no standalone path validator; path syntax is validated
	// implicitly by operations. Issuing a Get against sample data classifies each
	// path by the error it returns:
	//   - nil                 => valid syntax AND the path exists in the sample
	//   - ErrPathNotFound     => valid syntax, but the path is absent here
	//   - any other error     => invalid path syntax (rejected by the parser)
	testPaths := []string{
		".",                // root
		"user.name",        // simple property
		"users[0]",         // array index
		"data[0].items[1]", // nested array
		"users{name}",      // extraction
		"data[0].missing",  // valid syntax, not present in the sample
		"user[",            // invalid: missing closing bracket
		"users[0",          // invalid: missing closing bracket
	}

	fmt.Println("   Classifying each path against sample data:")
	for _, path := range testPaths {
		_, err := processor.Get(sampleJSON, path)
		switch {
		case err == nil:
			fmt.Printf("   OK  %-18q valid syntax, path exists\n", path)
		case errors.Is(err, json.ErrPathNotFound):
			fmt.Printf("   --  %-18q valid syntax, not in sample\n", path)
		default:
			fmt.Printf("   X   %-18q invalid syntax (rejected)\n", path)
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

	// reportSchema validates data against the schema and prints a consistent
	// result for every case: valid, the list of validation errors, or a hard
	// validation error. (The result var is named verrs/verr to avoid shadowing
	// the imported "errors" package.)
	reportSchema := func(label, data string) {
		fmt.Printf("   %s:\n", label)
		verrs, verr := json.ValidateSchema(data, schema)
		if verr != nil {
			fmt.Printf("   X validation error: %v\n", verr)
			return
		}
		if len(verrs) == 0 {
			fmt.Println("   ✓ valid")
			return
		}
		for _, e := range verrs {
			fmt.Printf("   X %s: %s\n", e.Path, e.Message)
		}
	}

	reportSchema("Validating valid user", validUser)
	fmt.Println()
	reportSchema("Validating invalid user (missing required field)", invalidUser1)
	fmt.Println()
	reportSchema("Validating invalid user (wrong types)", invalidUser2)

	// Alternative construction: instead of a &json.Schema{...} literal, build
	// one through the Config pattern — start from DefaultSchemaConfig(), set
	// fields, and let NewSchema assemble the Schema (nil maps become
	// empty, pointer fields become "present" flags).
	minLen, maxLen := 2, 50
	schemaCfg := json.DefaultSchemaConfig()
	schemaCfg.Type = "object"
	schemaCfg.Required = []string{"name", "email"}
	schemaCfg.Properties = map[string]*json.Schema{
		"name":  {Type: "string", MinLength: minLen, MaxLength: maxLen},
		"email": {Type: "string", Format: "email"},
	}
	cfgBuilt := json.NewSchema(schemaCfg)
	verrs, verr := json.ValidateSchema(validUser, cfgBuilt)
	if verr == nil && len(verrs) == 0 {
		fmt.Println("\n   ✓ Config-pattern schema (NewSchema) validates the same data")
	}

	// DefaultSchema is the permissive starting point: no type constraint, no
	// required fields, additional properties allowed — extend it for quick checks.
	ds := json.DefaultSchema()
	fmt.Printf("\n   DefaultSchema: permissive base — %d required, additionalProperties=%t\n",
		len(ds.Required), ds.AdditionalProperties)
}

func demonstrateSecurityValidation() {
	fmt.Println("\n4. Security Validation (SecurityConfig enforcement)")
	fmt.Println("----------------------------------------------------")

	// SecurityConfig hardens every operation on the processor: input size,
	// nesting depth, and dangerous-pattern checks. json.Valid (section 1)
	// inspects syntax only — Processor.Valid runs the full security pipeline
	// and explains rejections.
	processor, _ := json.New(json.SecurityConfig()) // OK: preset config always valid
	defer processor.Close()

	normal := `{"user": "John", "age": 30}`
	valid, err := processor.Valid(normal)
	fmt.Printf("   Normal JSON:               valid=%t err=%v\n", valid, err)

	// SecurityConfig caps nesting depth at 30; a 50-level document is
	// rejected before parsing, classifiable via errors.Is.
	tooDeep := strings.Repeat(`{"a":`, 50) + `1` + strings.Repeat(`}`, 50)
	valid, err = processor.Valid(tooDeep)
	fmt.Printf("   Nesting depth 50 (max 30): valid=%t\n", valid)
	fmt.Printf("     rejected: %v\n", err)
	fmt.Printf("     classified as ErrDepthLimit: %t\n", errors.Is(err, json.ErrDepthLimit))

	// Size cap works the same way — lower SecurityConfig's MaxJSONSize for a
	// tighter processor.
	tightCfg := json.SecurityConfig()
	tightCfg.MaxJSONSize = 512
	tight, _ := json.New(tightCfg) // OK: SecurityConfig-derived, always valid
	defer tight.Close()
	_, err = tight.Get(generateLargeJSON(50), "field0")
	fmt.Printf("   Size cap (512 bytes):      rejected=%t\n", err != nil)
	fmt.Printf("     classified as ErrSizeLimit: %t\n", errors.Is(err, json.ErrSizeLimit))
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

	// A malformed path is rejected by the parser and surfaces as an error
	_, err = processor.Get(testJSON, "user.name[")
	if err != nil {
		fmt.Printf("   Invalid path caught: %v\n", err)
	}

	// ValidBytes: the []byte quick-validity check — boolean only, no reason.
	fmt.Printf("   ValidBytes(testJSON): %t\n", processor.ValidBytes([]byte(testJSON)))
}

func demonstratePerCallConfigEnforcement() {
	fmt.Println("\n6. Per-call Config Enforcement (Phase 2)")
	fmt.Println("-------------------------------------------")

	// The optional trailing Config on package-level functions (Get/Set/Delete/...)
	// is now truly enforced — it used to be silently ignored. A deep-but-legal
	// document is accepted under the default nesting limit (200) and rejected
	// under a tighter per-call limit, with no Processor required.
	deep := strings.Repeat(`{"a":`, 50) + `1` + strings.Repeat(`}`, 50)

	if _, err := json.Get(deep, "a"); err != nil {
		fmt.Printf("   default cfg (depth 200): rejected -> %v\n", err)
	} else {
		fmt.Println("   default cfg (depth 200): accepted")
	}

	tight := json.DefaultConfig()
	tight.MaxNestingDepthSecurity = 15
	if _, err := json.Get(deep, "a", tight); err != nil {
		fmt.Printf("   tight cfg   (depth 15) : rejected -> %v\n", err)
	} else {
		fmt.Println("   tight cfg   (depth 15) : accepted")
	}

	fmt.Println("   => the trailing cfg now governs security limits on a single call")
}

// Helper function to generate large JSON for testing
func generateLargeJSON(size int) string {
	var b strings.Builder
	b.WriteByte('{')
	for i := 0; i < size; i++ {
		if i > 0 {
			b.WriteByte(',')
		}
		fmt.Fprintf(&b, "\"field%d\": \"value%d\"", i, i)
	}
	b.WriteByte('}')
	return b.String()
}
