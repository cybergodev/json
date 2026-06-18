//go:build example

package main

import (
	"fmt"

	"github.com/cybergodev/json"
)

// Type Conversion Example
//
// This example demonstrates type conversion capabilities in the cybergodev/json library.
// Learn about type-safe generic operations, automatic conversions, and deep copy.
//
// Topics covered:
// - Type-safe generic operations (GetTyped[T])
// - Automatic type conversion from JSON
// - Default values with GetTyped[T]
// - Deep copy via marshal/unmarshal cycle
//
// Run: go run -tags=example examples/7_type_conversion.go

func main() {
	fmt.Println("JSON Library - Type Conversion")
	fmt.Println("===============================")

	// 1. TYPE-SAFE GENERICS
	demonstrateTypeSafeGenerics()

	// 2. AUTOMATIC CONVERSION
	demonstrateAutomaticConversion()

	// 3. DEFAULT VALUES
	demonstrateDefaultValues()

	// 4. NUMBER HANDLING
	demonstrateNumberHandling()

	// 5. DEEP COPY
	demonstrateDeepCopy()

	fmt.Println("\nType conversion examples complete!")
}

func demonstrateTypeSafeGenerics() {
	fmt.Println("1. Type-Safe Generic Operations (GetTyped[T])")
	fmt.Println("------------------------------------------------")

	testJSON := `{
		"name": "Alice",
		"age": 30,
		"score": 95.5,
		"active": true,
		"tags": ["go", "json", "developer"]
	}`

	// Get as string
	name := json.GetTyped[string](testJSON, "name", "")
	fmt.Printf("   Name (string): %s\n", name)

	// Get as int
	age := json.GetTyped[int](testJSON, "age", 0)
	fmt.Printf("   Age (int): %d\n", age)

	// Get as float64
	score := json.GetTyped[float64](testJSON, "score", 0.0)
	fmt.Printf("   Score (float64): %.1f\n", score)

	// Get as bool
	active := json.GetTyped[bool](testJSON, "active", false)
	fmt.Printf("   Active (bool): %t\n", active)

	// Get as array
	tags := json.GetTyped[[]any](testJSON, "tags", nil)
	fmt.Printf("   Tags ([]any): %v\n", tags)

	// Get as object
	user := json.GetTyped[map[string]any](testJSON, "", nil)
	fmt.Printf("   Full user (map): %v\n", user)
}

func demonstrateAutomaticConversion() {
	fmt.Println("\n2. Automatic Type Conversion")
	fmt.Println("-------------------------------")

	testJSON := `{
		"intString": "42",
		"floatString": "3.14",
		"boolString": "true",
		"actualInt": 100,
		"actualFloat": 2.718,
		"actualBool": false
	}`

	// String to int (automatic conversion)
	intVal := json.GetInt(testJSON, "intString", 0)
	fmt.Printf("   String '42' -> int: %d\n", intVal)

	// String to float64
	floatVal := json.GetFloat(testJSON, "floatString", 0.0)
	fmt.Printf("   String '3.14' -> float64: %.2f\n", floatVal)

	// String to bool
	boolVal := json.GetBool(testJSON, "boolString", false)
	fmt.Printf("   String 'true' -> bool: %t\n", boolVal)

	// Type-preserving retrieval
	fmt.Println("\n   Type-preserving retrieval:")
	intVal2 := json.GetInt(testJSON, "actualInt", 0)
	fmt.Printf("   int 100 -> int: %d\n", intVal2)

	floatVal2 := json.GetFloat(testJSON, "actualFloat", 0.0)
	fmt.Printf("   float64 2.718 -> float64: %.3f\n", floatVal2)
}

func demonstrateDefaultValues() {
	fmt.Println("\n3. Default Values (GetTyped[T])")
	fmt.Println("-----------------------------------")

	partialData := `{
		"user": {
			"name": "Alice",
			"email": "alice@example.com"
		}
	}`

	// Existing field returns actual value
	email := json.GetTyped(partialData, "user.email", "no-email@example.com")
	fmt.Printf("   user.email: %s\n", email)

	// Missing field returns default
	phone := json.GetTyped(partialData, "user.phone", "N/A")
	fmt.Printf("   user.phone (missing): %s\n", phone)

	age := json.GetTyped(partialData, "user.age", 0)
	fmt.Printf("   user.age (missing): %d\n", age)

	score := json.GetTyped(partialData, "user.score", 100.0)
	fmt.Printf("   user.score (missing): %.1f\n", score)

	tags := json.GetTyped[[]any](partialData, "user.tags", []any{})
	fmt.Printf("   user.tags (missing): %v (length: %d)\n", tags, len(tags))
}

func demonstrateNumberHandling() {
	fmt.Println("\n4. JSON Number Handling")
	fmt.Println("-------------------------")

	numberJSON := `{
		"integer": 42,
		"largeInteger": 9007199254740992,
		"float": 3.14159,
		"scientific": 1.23e10,
		"negative": -123.45,
		"zero": 0
	}`

	// Get as int64 for large integers
	largeInt := json.GetInt(numberJSON, "largeInteger", 0)
	fmt.Printf("   Large integer: %d\n", largeInt)

	// Get as float64 for decimals
	floatVal := json.GetFloat(numberJSON, "float", 0.0)
	fmt.Printf("   Float value: %.5f\n", floatVal)

	// Get as any for generic handling
	anyVal, _ := json.Get(numberJSON, "integer")
	fmt.Printf("   As any: %v (type: %T)\n", anyVal, anyVal)

	// Number type for precise numeric handling
	fmt.Println("\n   Number type (json.Number):")
	num := json.Number("3.14159")
	fmt.Printf("   Number.String():  %s\n", num.String())
	if f, err := num.Float64(); err == nil {
		fmt.Printf("   Number.Float64(): %.5f\n", f)
	}
	// Int64() parses only integer literals; a fractional value is rejected
	// rather than truncated, so callers can detect the precision loss.
	if i, err := num.Int64(); err != nil {
		fmt.Printf("   Number.Int64():   error (not an integer): %v\n", err)
	} else {
		fmt.Printf("   Number.Int64():   %d\n", i)
	}

	intNum := json.Number("42")
	if i, err := intNum.Int64(); err == nil {
		fmt.Printf("   Number('42').Int64(): %d\n", i)
	}
}

func demonstrateDeepCopy() {
	fmt.Println("\n5. Deep Copy via Marshal/Unmarshal Cycle")
	fmt.Println("-------------------------------------")

	original := map[string]any{
		"name": "Bob",
		"age":  25,
		"address": map[string]any{
			"street": "123 Main St",
			"city":   "Springfield",
		},
		"hobbies": []any{"reading", "coding"},
	}

	fmt.Println("   Deep copy using marshal/unmarshal cycle:")

	// Marshal original to JSON
	jsonBytes, err := json.Marshal(original)
	if err != nil {
		fmt.Printf("   Error marshaling: %v\n", err)
		return
	}

	// Unmarshal into new variable to create deep copy
	var copied map[string]any
	if err := json.Unmarshal(jsonBytes, &copied); err != nil {
		fmt.Printf("   Error unmarshaling: %v\n", err)
		return
	}

	fmt.Printf("   Original: %v\n", original)
	fmt.Printf("   Copy:     %v\n", copied)

	// Modify copy to prove it's deep
	if addr, ok := copied["address"].(map[string]any); ok {
		addr["city"] = "New City"
	}
	if hobbies, ok := copied["hobbies"].([]any); ok {
		hobbies[0] = "writing"
	}

	fmt.Println("\n   After modifying copy:")
	fmt.Printf("   Original address city: %v\n", original["address"].(map[string]any)["city"])
	fmt.Printf("   Copy address city:     %v\n", copied["address"].(map[string]any)["city"])

	fmt.Println("\n   Original is unchanged (deep copy successful)")
}
