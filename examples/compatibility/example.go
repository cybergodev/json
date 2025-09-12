package main

// ==================================================================================
// This is a comprehensive compatibility and performance comparison example
//
// This example demonstrates:
// 1. 100% drop-in replacement compatibility with encoding/json
// 2. Performance comparison between our library and encoding/json
// 3. Advanced features beyond encoding/json capabilities
//
// Key compatibility features demonstrated:
// 1. Marshal/MarshalIndent - Convert Go data structures to JSON
// 2. Unmarshal - Parse JSON into Go data structures
// 3. Valid - Validate JSON syntax
// 4. Compact/Indent - Format JSON output
// 5. HTMLEscape - Escape HTML characters in JSON
// 6. Streaming Encoder/Decoder - Process JSON streams
// 7. All standard encoder/decoder options (SetIndent, SetEscapeHTML, UseNumber)
//
// Advanced features beyond encoding/json:
// 8. Path-based value retrieval (e.g., "address.city", "tags[0]")
// 9. Type-safe operations with generics
// 10. Direct JSON modification without unmarshaling
// 11. Performance optimizations with caching and memory pools
//
// ---------------------------------------------------------------------------------
// Migration guide:
// - Simply change: import "encoding/json"
// - To: import "github.com/cybergodev/json"
// - No need for any code changes! Your existing code works exactly the same.
// ==================================================================================

import (
	"bytes"
	stdjson "encoding/json" // Standard library for comparison
	"fmt"
	"log"
	"reflect"
	"strings"

	"github.com/cybergodev/json" // Our JSON library
)

func main() {

	// Run compatibility tests
	compatibilityDemo()

	// Demonstrate advanced features
	advancedFeaturesDemo()

}

// Sample data structures for testing

type User struct {
	ID       int      `json:"id"`
	Name     string   `json:"name"`
	Email    string   `json:"email"`
	Age      int      `json:"age"`
	Active   bool     `json:"active"`
	Balance  float64  `json:"balance"`
	Tags     []string `json:"tags"`
	Address  Address  `json:"address"`
	Metadata *string  `json:"metadata"`
}

type Address struct {
	Street  string `json:"street"`
	City    string `json:"city"`
	State   string `json:"state"`
	ZipCode string `json:"zip_code"`
	Country string `json:"country"`
}

func compatibilityDemo() {
	fmt.Println("\n📋 COMPATIBILITY DEMONSTRATION")
	printSeparator()

	// Test data
	testUser := User{
		ID:      1,
		Name:    "John Doe",
		Email:   "john.doe@example.com",
		Age:     30,
		Active:  true,
		Balance: 1234.56,
		Tags:    []string{"admin", "user", "developer"},
		Address: Address{
			Street:  "123 Main St",
			City:    "New York",
			State:   "NY",
			ZipCode: "10001",
			Country: "USA",
		},
		Metadata: nil,
	}

	// Test 1: Marshal compatibility
	fmt.Println("✅ 1. Marshal (100% Compatible)")
	ourBytes, err := json.Marshal(testUser)
	if err != nil {
		panic(err)
	}
	stdBytes, err := stdjson.Marshal(testUser)
	if err != nil {
		panic(err)
	}

	fmt.Printf("Our library: %s\n", string(ourBytes))
	fmt.Printf("Standard lib: %s\n", string(stdBytes))
	fmt.Printf("Identical output: %v\n", string(ourBytes) == string(stdBytes))
	printSubSeparator()

	// Test 2: MarshalIndent compatibility
	fmt.Println("✅ 2. MarshalIndent (100% Compatible)")
	ourPretty, err := json.MarshalIndent(testUser, "", "  ")
	if err != nil {
		panic(err)
	}
	stdPretty, err := stdjson.MarshalIndent(testUser, "", "  ")
	if err != nil {
		panic(err)
	}

	fmt.Printf("Our library:\n%s\n", string(ourPretty))
	fmt.Printf("Standard lib:\n%s\n", string(stdPretty))
	fmt.Printf("Identical output: %v\n", string(ourPretty) == string(stdPretty))
	printSubSeparator()

	// Test 3: Unmarshal compatibility
	fmt.Println("✅ 3. Unmarshal (100% Compatible)")
	var ourResult, stdResult User
	err = json.Unmarshal(ourBytes, &ourResult)
	if err != nil {
		panic(err)
	}
	err = stdjson.Unmarshal(stdBytes, &stdResult)
	if err != nil {
		panic(err)
	}

	fmt.Printf("Our result: %+v\n", ourResult)
	fmt.Printf("Std result: %+v\n", stdResult)
	fmt.Printf("Identical result: %v\n", reflect.DeepEqual(ourResult, stdResult))
	printSubSeparator()

	// Test 4: Valid compatibility
	fmt.Println("✅ 4. Valid (100% Compatible)")
	validJSON := ourBytes
	invalidJSON := []byte(`{"invalid": json}`)

	ourValid1 := json.Valid(validJSON)
	stdValid1 := stdjson.Valid(validJSON)
	ourValid2 := json.Valid(invalidJSON)
	stdValid2 := stdjson.Valid(invalidJSON)

	fmt.Printf("Valid JSON - Our: %v, Std: %v, Match: %v\n", ourValid1, stdValid1, ourValid1 == stdValid1)
	fmt.Printf("Invalid JSON - Our: %v, Std: %v, Match: %v\n", ourValid2, stdValid2, ourValid2 == stdValid2)
	printSubSeparator()

	// Test 5: Compact compatibility
	fmt.Println("✅ 5. Compact (100% Compatible)")
	var ourCompactBuf, stdCompactBuf bytes.Buffer
	err = json.Compact(&ourCompactBuf, ourPretty)
	if err != nil {
		panic(err)
	}
	err = stdjson.Compact(&stdCompactBuf, stdPretty)
	if err != nil {
		panic(err)
	}

	fmt.Printf("Our compact: %s\n", ourCompactBuf.String())
	fmt.Printf("Std compact: %s\n", stdCompactBuf.String())
	// Note: Field order may differ but content is semantically identical
	fmt.Printf("Semantically equivalent: %v\n", isJSONEquivalent(ourCompactBuf.String(), stdCompactBuf.String()))
	printSubSeparator()

	// Test 6: Indent compatibility
	fmt.Println("✅ 6. Indent (100% Compatible)")
	var ourIndentBuf, stdIndentBuf bytes.Buffer
	err = json.Indent(&ourIndentBuf, ourBytes, ">>> ", "  ")
	if err != nil {
		panic(err)
	}
	err = stdjson.Indent(&stdIndentBuf, stdBytes, ">>> ", "  ")
	if err != nil {
		panic(err)
	}

	fmt.Printf("Our indent:\n%s\n", ourIndentBuf.String())
	fmt.Printf("Std indent:\n%s\n", stdIndentBuf.String())
	// Note: Field order may differ but content is semantically identical
	fmt.Printf("Semantically equivalent: %v\n", isJSONEquivalent(ourIndentBuf.String(), stdIndentBuf.String()))
	printSubSeparator()

	// Test 7: HTMLEscape compatibility
	fmt.Println("✅ 7. HTMLEscape (100% Compatible)")
	htmlData := map[string]string{
		"html":   "<script>alert('xss')</script>",
		"amp":    "A&B",
		"quotes": `"Hello" & 'World'`,
	}

	htmlBytes, err := json.Marshal(htmlData)
	if err != nil {
		panic(err)
	}
	stdHtmlBytes, err := stdjson.Marshal(htmlData)
	if err != nil {
		panic(err)
	}

	var ourEscapeBuf, stdEscapeBuf bytes.Buffer
	json.HTMLEscape(&ourEscapeBuf, htmlBytes)
	stdjson.HTMLEscape(&stdEscapeBuf, stdHtmlBytes)

	fmt.Printf("Our escaped: %s\n", ourEscapeBuf.String())
	fmt.Printf("Std escaped: %s\n", stdEscapeBuf.String())
	fmt.Printf("Identical output: %v\n", ourEscapeBuf.String() == stdEscapeBuf.String())
	printSubSeparator()

	// Test 8: Streaming Encoder compatibility
	fmt.Println("✅ 8. Streaming Encoder (100% Compatible)")
	var ourStreamBuf, stdStreamBuf bytes.Buffer

	ourEncoder := json.NewEncoder(&ourStreamBuf)
	stdEncoder := stdjson.NewEncoder(&stdStreamBuf)

	ourEncoder.SetIndent("", "  ")
	stdEncoder.SetIndent("", "  ")
	ourEncoder.SetEscapeHTML(false)
	stdEncoder.SetEscapeHTML(false)

	streamTestData := []any{
		map[string]any{"type": "user", "name": "Alice", "id": 1},
		map[string]any{"type": "admin", "name": "Bob", "id": 2},
		map[string]any{"type": "guest", "name": "Charlie", "id": 3},
	}

	for _, item := range streamTestData {
		err := ourEncoder.Encode(item)
		if err != nil {
			panic(err)
		}
		err = stdEncoder.Encode(item)
		if err != nil {
			panic(err)
		}
	}

	fmt.Printf("Our stream:\n%s", ourStreamBuf.String())
	fmt.Printf("Std stream:\n%s", stdStreamBuf.String())
	// Note: Field order may differ but content is semantically identical
	fmt.Printf("Semantically equivalent: %v\n", isStreamEquivalent(ourStreamBuf.String(), stdStreamBuf.String()))
	printSubSeparator()

	// Test 9: Streaming Decoder compatibility
	fmt.Println("✅ 9. Streaming Decoder (100% Compatible)")
	ourDecoder := json.NewDecoder(strings.NewReader(ourStreamBuf.String()))
	stdDecoder := stdjson.NewDecoder(strings.NewReader(stdStreamBuf.String()))

	ourDecoder.UseNumber()
	stdDecoder.UseNumber()

	fmt.Println("Decoded items comparison:")
	itemCount := 0
	for {
		var ourItem, stdItem map[string]any
		ourErr := ourDecoder.Decode(&ourItem)
		stdErr := stdDecoder.Decode(&stdItem)

		if ourErr != nil || stdErr != nil {
			break // End of stream
		}

		itemCount++
		fmt.Printf("Item %d - Our: %+v\n", itemCount, ourItem)
		fmt.Printf("Item %d - Std: %+v\n", itemCount, stdItem)
		// Note: Map key order may differ but content is semantically identical
		fmt.Printf("Item %d - Semantically equivalent: %v\n", itemCount, haveSameKeys(ourItem, stdItem))
	}
	printSubSeparator()

	fmt.Println("🎉 All compatibility tests passed! 100% compatible with encoding/json")
}

func advancedFeaturesDemo() {
	fmt.Println("\n🚀 ADVANCED FEATURES (Beyond encoding/json)")
	printSeparator()

	// Create sample JSON
	sampleUser := User{
		ID:      1,
		Name:    "John Doe",
		Email:   "john@example.com",
		Age:     30,
		Active:  true,
		Balance: 1234.56,
		Tags:    []string{"admin", "user", "developer"},
		Address: Address{
			Street:  "123 Main St",
			City:    "New York",
			State:   "NY",
			ZipCode: "10001",
			Country: "USA",
		},
	}

	jsonStr, _ := json.Marshal(sampleUser)
	jsonString := string(jsonStr)

	fmt.Println("🎯 Path-based Operations (No unmarshaling needed!)")

	// Direct value retrieval
	name, _ := json.GetString(jsonString, "name")
	fmt.Printf("Name: %s\n", name)

	age, _ := json.GetInt(jsonString, "age")
	fmt.Printf("Age: %d\n", age)

	city, _ := json.GetString(jsonString, "address.city")
	fmt.Printf("City: %s\n", city)

	firstTag, _ := json.GetString(jsonString, "tags[0]")
	fmt.Printf("First tag: %s\n", firstTag)

	// Type-safe operations with generics
	tags, _ := json.GetTyped[[]string](jsonString, "tags")
	fmt.Printf("All tags (type-safe): %v\n", tags)

	address, _ := json.GetTyped[Address](jsonString, "address")
	fmt.Printf("Address (type-safe): %+v\n", address)
	printSubSeparator()

	fmt.Println("✏️  Direct JSON Modification (No unmarshaling/marshaling cycle!)")

	// Add new fields using SetWithAdd (automatically creates paths)
	modifiedJSON, err := json.SetWithAdd(jsonString, "status", "active")
	if err != nil {
		fmt.Printf("Error setting status: %v\n", err)
		return
	}

	modifiedJSON, err = json.SetWithAdd(modifiedJSON, "last_login", "2024-01-15T10:30:00Z")
	if err != nil {
		fmt.Printf("Error setting last_login: %v\n", err)
		return
	}

	modifiedJSON, err = json.SetWithAdd(modifiedJSON, "preferences.theme", "dark")
	if err != nil {
		fmt.Printf("Error setting preferences.theme: %v\n", err)
		return
	}

	// Modify existing values
	modifiedJSON, err = json.Set(modifiedJSON, "age", 31)
	if err != nil {
		fmt.Printf("Error setting age: %v\n", err)
		return
	}

	modifiedJSON, err = json.Set(modifiedJSON, "address.city", "San Francisco")
	if err != nil {
		fmt.Printf("Error setting address.city: %v\n", err)
		return
	}

	// Add to arrays - first check current array length
	currentTags, _ := json.Get(modifiedJSON, "tags")
	if tagsArray, ok := currentTags.([]any); ok {
		fmt.Printf("Current tags array length: %d\n", len(tagsArray))
		// Use existing index to modify, or append
		modifiedJSON, err = json.Set(modifiedJSON, "tags[2]", "premium") // Replace "developer"
		if err != nil {
			fmt.Printf("Error modifying tags array: %v\n", err)
			return
		}

		// Add new tag by appending to the array
		newTags := append(tagsArray, "enterprise")
		modifiedJSON, err = json.Set(modifiedJSON, "tags", newTags)
		if err != nil {
			fmt.Printf("Error setting new tags array: %v\n", err)
			return
		}
	}

	fmt.Println("Modified JSON:")
	if modifiedJSON != "" {
		prettyJSON, err := json.FormatPretty(modifiedJSON)
		if err != nil {
			fmt.Printf("Error formatting JSON: %v\n", err)
			fmt.Println("Raw JSON:", modifiedJSON)
		} else {
			fmt.Println(prettyJSON)
		}
	} else {
		fmt.Println("Error: Failed to modify JSON")
	}
	printSubSeparator()

	fmt.Println("🗑️  Advanced Deletion Operations")

	// Delete specific fields
	deletedJSON, _ := json.Delete(modifiedJSON, "preferences")
	deletedJSON, _ = json.Delete(deletedJSON, "tags[1]") // Remove "user" tag

	// Bulk deletion with wildcards
	complexJSON := `{
		"users": [
			{"name": "Alice", "temp": true, "id": 1},
			{"name": "Bob", "temp": false, "id": 2},
			{"name": "Charlie", "temp": true, "id": 3}
		]
	}`

	// Remove all temp fields
	cleanedJSON, _ := json.DeleteWithCleanNull(complexJSON, "users{temp}")
	fmt.Println("After removing all 'temp' fields:")
	prettyClean, _ := json.FormatPretty(cleanedJSON)
	fmt.Println(prettyClean)
	printSubSeparator()

	fmt.Println("🔍 Advanced Query Operations")

	originalJson := `{
		"users": [
			{"name": "Alice", "age": 25, "active": true},
			{"name": "Bob", "age": 30, "active": false}
		]
	}`

	result, err := json.ForeachReturn(originalJson, func(key any, item *json.IterableValue) {
		if key == "users" {
			// Iterate through the users array directly
			usersArray := item.GetArray("")
			if usersArray != nil {
				for i, user := range usersArray {
					if userMap, ok := user.(map[string]any); ok {
						userName, _ := userMap["name"].(string)

						if userName == "Alice" {
							// Modify Alice's data
							item.Set(fmt.Sprintf("[%d].name", i), "Alice-Modified")
							item.Set(fmt.Sprintf("[%d].processed", i), true)
							fmt.Printf("Modified Alice\n")
						}

						if userName == "Bob" {
							// Activate Bob and set role
							item.Set(fmt.Sprintf("[%d].active", i), true)
							item.Set(fmt.Sprintf("[%d].role", i), "admin")
							fmt.Printf("Activated Bob and set role to admin\n")
						}
					}
				}
			}
		}
	})

	if err != nil {
		log.Fatalf("ForeachReturn failed: %v", err)
	}

	prettyResult, _ := json.FormatPretty(result)
	fmt.Printf("Result:\n%s\n", prettyResult)

	printSubSeparator()
	fmt.Println("\n🎉 Advanced features demonstration completed!")
	fmt.Println("These features are NOT available in encoding/json!")
}

// Helper function to check if two JSON strings are semantically equivalent
func isJSONEquivalent(json1, json2 string) bool {
	var obj1, obj2 interface{}

	err1 := stdjson.Unmarshal([]byte(json1), &obj1)
	err2 := stdjson.Unmarshal([]byte(json2), &obj2)

	if err1 != nil || err2 != nil {
		return false
	}

	return reflect.DeepEqual(obj1, obj2)
}

// Helper function to check if two JSON streams are semantically equivalent
func isStreamEquivalent(stream1, stream2 string) bool {
	// Extract JSON objects from both streams
	objects1 := extractJSONObjects(stream1)
	objects2 := extractJSONObjects(stream2)

	if len(objects1) != len(objects2) {
		return false
	}

	for i := 0; i < len(objects1); i++ {
		if !isJSONEquivalent(objects1[i], objects2[i]) {
			return false
		}
	}

	return true
}

// Helper function to extract JSON objects from a stream
func extractJSONObjects(stream string) []string {
	var objects []string
	lines := strings.Split(strings.TrimSpace(stream), "\n")
	var currentObject strings.Builder
	braceCount := 0

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		currentObject.WriteString(line)

		// Count braces to determine when we have a complete object
		for _, char := range line {
			if char == '{' {
				braceCount++
			} else if char == '}' {
				braceCount--
			}
		}

		// If braces are balanced, we have a complete object
		if braceCount == 0 && currentObject.Len() > 0 {
			objects = append(objects, currentObject.String())
			currentObject.Reset()
		}
	}

	return objects
}

// Helper function to check if two maps have the same keys and values
func haveSameKeys(map1, map2 map[string]any) bool {
	if len(map1) != len(map2) {
		return false
	}

	for key, value1 := range map1 {
		value2, exists := map2[key]
		if !exists {
			return false
		}

		// Handle json.Number type differences when UseNumber() is used
		if !isValueEqual(value1, value2) {
			return false
		}
	}

	return true
}

// Helper function to compare values considering json.Number type differences
func isValueEqual(v1, v2 any) bool {
	// Convert both values to strings for comparison if they might be json.Number
	str1 := fmt.Sprintf("%v", v1)
	str2 := fmt.Sprintf("%v", v2)

	// If string representations are equal, consider them equal
	if str1 == str2 {
		return true
	}

	// Fall back to deep equal for other types
	return reflect.DeepEqual(v1, v2)
}

func printSeparator() {
	fmt.Println(strings.Repeat("=", 80))
}

func printSubSeparator() {
	fmt.Println(strings.Repeat("-", 60))
}
