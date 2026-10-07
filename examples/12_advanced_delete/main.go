//go:build example

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
// - Array element deletion
// - Nested path deletion
// - DeleteClean — delete plus a sweep of null/empty values (no Config needed)
// - Batch deletion and sanitization patterns
//
// Run: go run -tags=example ./examples/12_advanced_delete

// deleteAndShow deletes path from data and prints the before→after result.
// The demo operates on author-controlled valid data, so an error is reported
// and the demo continues rather than aborting.
func deleteAndShow(data, path string) {
	result, err := json.Delete(data, path)
	if err != nil {
		fmt.Printf("   Error deleting %s: %v\n", path, err)
		return
	}
	fmt.Printf("\n   After deleting %s:\n   %s\n", path, result)
}

// deleteAll applies Delete for every path, threading each result into the
// next call. Errors are reported per path and skipped so the demo continues.
func deleteAll(data string, paths []string) string {
	for _, field := range paths {
		updated, err := json.Delete(data, field)
		if err != nil {
			fmt.Printf("   Error deleting %s: %v\n", field, err)
			continue
		}
		data = updated
	}
	return data
}

func main() {
	fmt.Println("JSON Library - Advanced Delete Operations")
	fmt.Println("==============================================")

	// 1. BASIC DELETE
	demonstrateBasicDelete()

	// 2. ARRAY DELETION
	demonstrateArrayDelete()

	// 3. DELETE WITH CLEANUP
	demonstrateDeleteWithCleanup()

	// 4. NESTED DELETION
	demonstrateNestedDelete()

	// 5. BATCH DELETION
	demonstrateBatchDelete()

	// 6. PRACTICAL USE CASES
	demonstratePracticalUseCases()

	fmt.Println("\nAdvanced delete operations complete!")
}

func demonstrateBasicDelete() {
	fmt.Println("1. Basic Delete Operations")
	fmt.Println("--------------------------")

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
	fmt.Println("\n2. Array Element Deletion")
	fmt.Println("-------------------------")

	data := `{
		"items": ["apple", "banana", "cherry", "date", "elderberry"],
		"numbers": [10, 20, 30, 40, 50]
	}`

	fmt.Println("   Original:")
	fmt.Println("   " + data)

	// Delete first / last / middle items (each operates on the original data).
	deleteAndShow(data, "items[0]")
	deleteAndShow(data, "numbers[-1]")
	deleteAndShow(data, "items[2]")
}

func demonstrateDeleteWithCleanup() {
	fmt.Println("\n3. Delete with Cleanup (DeleteClean)")
	fmt.Println("-------------------------------------")

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

	// Regular Delete removes the targeted key but leaves any null siblings behind.
	regularDelete, err := json.Delete(data, "user.email")
	if err != nil {
		fmt.Printf("   Error: %v\n", err)
		return
	}
	fmt.Println("\n   After regular Delete (user.email):")
	fmt.Println("   " + regularDelete)

	// DeleteClean = Delete with CleanupNulls+CompactArrays: removes the key AND
	// sweeps up null/empty values left behind. No manual Config needed.
	cleanDelete, err := json.DeleteClean(data, "user.phone")
	if err != nil {
		fmt.Printf("   Error: %v\n", err)
		return
	}
	fmt.Println("\n   After DeleteClean (user.phone):")
	fmt.Println("   " + cleanDelete)

	cleanDelete2, err := json.DeleteClean(data, "user.address")
	if err != nil {
		fmt.Printf("   Error: %v\n", err)
		return
	}
	fmt.Println("\n   After DeleteClean (user.address):")
	fmt.Println("   " + cleanDelete2)

	fmt.Println("\n   Key differences:")
	fmt.Println("   - Delete:      removes the targeted key (sibling nulls remain)")
	fmt.Println("   - DeleteClean: removes it AND sweeps up null/empty values")

	// Show cleanup of null values on a small object.
	dataWithNulls := `{
		"a": 1,
		"b": null,
		"c": 2,
		"d": null
	}`

	fmt.Println("\n   Cleanup demonstration:")
	fmt.Println("   Original: " + dataWithNulls)

	cleaned, err := json.DeleteClean(dataWithNulls, "b")
	if err != nil {
		fmt.Printf("   Error: %v\n", err)
		return
	}
	fmt.Println("   After DeleteClean('b'): " + cleaned)
}

func demonstrateNestedDelete() {
	fmt.Println("\n4. Nested Path Deletion")
	fmt.Println("-----------------------")

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

	// Delete nested credential / object / section (each from the original data).
	deleteAndShow(data, "config.database.credentials.password")
	deleteAndShow(data, "config.api")
	deleteAndShow(data, "config.database")
}

func demonstrateBatchDelete() {
	fmt.Println("\n5. Batch Deletion")
	fmt.Println("-----------------")

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

	deleted := deleteAll(data, sensitiveFields)

	fmt.Println("\n   After batch deleting sensitive fields:")
	fmt.Println("   " + deleted)
}

func demonstratePracticalUseCases() {
	fmt.Println("\n6. Practical Use Cases")
	fmt.Println("----------------------")

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
	sanitized := deleteAll(userData, []string{
		"user.password",
		"user.credit_card",
		"user.ssn",
	})

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

	// A single DeleteClean removes the targeted field AND sweeps every other
	// null value in the document — so one call cleans the whole response.
	// (Looping DeleteClean over each null field would fail: the first call
	// removes them all, leaving nothing for the later calls to find.)
	cleaned, err := json.DeleteClean(apiResponse, "data.description")
	if err != nil {
		fmt.Printf("   Error cleaning response: %v\n", err)
		return
	}

	fmt.Println("\n   Cleaned response (one DeleteClean swept all nulls):")
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

	// Remove specific optional fields one by one with plain Delete. Unlike
	// DeleteClean (use case 2), this removes only the listed fields and leaves
	// any other nulls untouched.
	cleanedForm := deleteAll(formData, []string{"user.bio", "user.website", "user.twitter"})

	fmt.Println("\n   Cleaned (only provided fields):")
	fmt.Println("   " + cleanedForm)
}
