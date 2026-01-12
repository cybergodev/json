//go:build ignore

package main

import (
	"fmt"

	"github.com/cybergodev/json"
)

// Advanced Delete Operations Example
//
// This example demonstrates advanced delete operations including
// cleanup of null values and array compaction.
//
// Topics covered:
// - Basic Delete operations
// - DeleteWithCleanNull for cleanup
// - Array element deletion
// - Nested path deletion
// - Cleanup options
//
// Run: go run examples/12_advanced_delete.go

func main() {
	fmt.Println("🗑️  JSON Library - Advanced Delete Operations")
	fmt.Println("==========================================\n ")

	// 1. BASIC DELETE
	fmt.Println("1️⃣  Basic Delete Operations")
	fmt.Println("───────────────────────────")
	demonstrateBasicDelete()

	// 2. ARRAY DELETION
	fmt.Println("\n2️⃣  Array Element Deletion")
	fmt.Println("─────────────────────────")
	demonstrateArrayDelete()

	// 3. DELETE WITH CLEANUP
	fmt.Println("\n3️⃣  Delete with Cleanup (DeleteWithCleanNull)")
	fmt.Println("──────────────────────────────────────────────")
	demonstrateDeleteWithCleanup()

	// 4. NESTED DELETION
	fmt.Println("\n4️⃣  Nested Path Deletion")
	fmt.Println("─────────────────────────")
	demonstrateNestedDelete()

	// 5. BATCH DELETION
	fmt.Println("\n5️⃣  Batch Deletion")
	fmt.Println("──────────────────")
	demonstrateBatchDelete()

	// 6. PRACTICAL USE CASES
	fmt.Println("\n6️⃣  Practical Use Cases")
	fmt.Println("──────────────────────")
	demonstratePracticalUseCases()

	fmt.Println("\n✅ Advanced delete operations complete!")
}

func demonstrateBasicDelete() {
	data := `{
		"user": {
			"name": "Alice",
			"email": "alice@example.com",
			"password": "secret123",
			"age": 30
		},
		"metadata": {
			"created": "2024-01-01",
			"updated": "2024-01-15",
			"version": 2
		}
	}`

	fmt.Println("   Original:")
	fmt.Println("   " + data)

	// Delete password field
	deleted, err := json.Delete(data, "user.password")
	if err != nil {
		fmt.Printf("   Error: %v\n", err)
		return
	}

	fmt.Println("\n   After deleting user.password:")
	fmt.Println("   " + deleted)

	// Verify deletion
	password, _ := json.Get(deleted, "user.password")
	fmt.Printf("\n   user.password value: %v (should be nil/missing)\n", password)
}

func demonstrateArrayDelete() {
	data := `{
		"items": ["apple", "banana", "cherry", "date", "elderberry"],
		"numbers": [10, 20, 30, 40, 50]
	}`

	fmt.Println("   Original:")
	fmt.Println("   " + data)

	// Delete first item
	deleted, _ := json.Delete(data, "items[0]")
	fmt.Println("\n   After deleting items[0]:")
	fmt.Println("   " + deleted)

	// Delete last item
	deleted2, _ := json.Delete(data, "numbers[-1]")
	fmt.Println("\n   After deleting numbers[-1]:")
	fmt.Println("   " + deleted2)

	// Delete middle item
	deleted3, _ := json.Delete(data, "items[2]")
	fmt.Println("\n   After deleting items[2]:")
	fmt.Println("   " + deleted3)
}

func demonstrateDeleteWithCleanup() {
	data := `{
		"user": {
			"name": "Bob",
			"email": null,
			"phone": null,
			"age": 25,
			"address": null
		},
		"settings": {
			"theme": "dark",
			"notifications": null
		}
	}`

	fmt.Println("   Original (with null values):")
	fmt.Println("   " + data)

	// Regular delete - leaves null
	regularDelete, _ := json.Delete(data, "user.email")
	fmt.Println("\n   After regular Delete (user.email):")
	fmt.Println("   " + regularDelete)

	// DeleteWithCleanNull - removes and cleans up
	cleanDelete, _ := json.DeleteWithCleanNull(data, "user.phone")
	fmt.Println("\n   After DeleteWithCleanNull (user.phone):")
	fmt.Println("   " + cleanDelete)

	fmt.Println("\n   Key differences:")
	fmt.Println("   - Delete: removes field, may leave null in its place")
	fmt.Println("   - DeleteWithCleanNull: removes field and cleans up nulls")

	// Show cleanup of null values
	dataWithNulls := `{
		"a": 1,
		"b": null,
		"c": 2,
		"d": null
	}`

	fmt.Println("\n   Cleanup demonstration:")
	fmt.Println("   Original: " + dataWithNulls)

	cleaned, _ := json.DeleteWithCleanNull(dataWithNulls, "b")
	fmt.Println("   After DeleteWithCleanNull('b'): " + cleaned)
}

func demonstrateNestedDelete() {
	data := `{
		"config": {
			"database": {
				"host": "localhost",
				"port": 5432,
				"credentials": {
					"username": "admin",
					"password": "secret"
				}
			},
			"api": {
				"key": "abc123",
				"secret": "xyz789"
			}
		}
	}`

	fmt.Println("   Original:")
	fmt.Println("   " + data)

	// Delete nested credential
	deleted1, _ := json.Delete(data, "config.database.credentials.password")
	fmt.Println("\n   After deleting config.database.credentials.password:")
	fmt.Println("   " + deleted1)

	// Delete entire nested object
	deleted2, _ := json.Delete(data, "config.api")
	fmt.Println("\n   After deleting config.api:")
	fmt.Println("   " + deleted2)

	// Delete entire database section
	deleted3, _ := json.Delete(data, "config.database")
	fmt.Println("\n   After deleting config.database:")
	fmt.Println("   " + deleted3)
}

func demonstrateBatchDelete() {
	data := `{
		"user": {
			"id": 1,
			"name": "Alice",
			"email": "alice@example.com",
			"password": "secret",
			"ssn": "123-45-6789",
			"credit_card": "4111-1111-1111-1111"
		},
		"metadata": {
			"internal_id": "ABC123",
			"debug_info": "detailed trace",
			"created_at": "2024-01-01"
		}
	}`

	fmt.Println("   Original:")
	fmt.Println("   " + data)

	// Delete sensitive fields
	sensitiveFields := []string{
		"user.password",
		"user.ssn",
		"user.credit_card",
		"metadata.internal_id",
		"metadata.debug_info",
	}

	deleted := data
	for _, field := range sensitiveFields {
		var err error
		deleted, err = json.Delete(deleted, field)
		if err != nil {
			fmt.Printf("   Error deleting %s: %v\n", field, err)
		}
	}

	fmt.Println("\n   After batch deleting sensitive fields:")
	fmt.Println("   " + deleted)
}

func demonstratePracticalUseCases() {
	// Use case 1: Sanitize user data for logging
	fmt.Println("   Use Case 1: Sanitize user data for logging")

	userData := `{
		"user": {
			"id": 123,
			"username": "Alice",
			"email": "alice@example.com",
			"password": "SecretPass123!",
			"credit_card": "4111111111111111",
			"ssn": "123-45-6789"
		}
	}`

	fmt.Println("   Original (sensitive):")
	fmt.Println("   " + userData)

	// Sanitize for logging
	sanitized := userData
	sensitiveFields := []string{
		"user.password",
		"user.credit_card",
		"user.ssn",
	}

	for _, field := range sensitiveFields {
		sanitized, _ = json.Delete(sanitized, field)
	}

	fmt.Println("\n   Sanitized for logging:")
	fmt.Println("   " + sanitized)

	// Use case 2: Clean up null values from API response
	fmt.Println("\n\n   Use Case 2: Clean up API response")

	apiResponse := `{
		"data": {
			"id": 1,
			"name": "Product",
			"description": null,
			"category": null,
			"price": 29.99,
			"discount": null
		}
	}`

	fmt.Println("   API response (with nulls):")
	fmt.Println("   " + apiResponse)

	// Clean up nulls by deleting and using cleanup
	cleaned := apiResponse
	nullFields := []string{"data.description", "data.category", "data.discount"}

	for _, field := range nullFields {
		cleaned, _ = json.DeleteWithCleanNull(cleaned, field)
	}

	fmt.Println("\n   Cleaned response:")
	fmt.Println("   " + cleaned)

	// Use case 3: Remove optional fields that weren't provided
	fmt.Println("\n\n   Use Case 3: Remove unset optional fields")

	formData := `{
		"user": {
			"username": "bob",
			"email": "bob@example.com",
			"bio": null,
			"website": null,
			"twitter": null
		}
	}`

	fmt.Println("   Form submission (unset fields as null):")
	fmt.Println("   " + formData)

	// Remove null optional fields
	cleanedForm := formData
	optionalFields := []string{"user.bio", "user.website", "user.twitter"}

	for _, field := range optionalFields {
		cleanedForm, _ = json.DeleteWithCleanNull(cleanedForm, field)
	}

	fmt.Println("\n   Cleaned (only provided fields):")
	fmt.Println("   " + cleanedForm)
}
