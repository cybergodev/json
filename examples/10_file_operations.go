//go:build ignore

package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/cybergodev/json"
)

// File Operations Example
//
// This example demonstrates file I/O operations for JSON data.
// Learn about reading, writing, and processing JSON files.
//
// Topics covered:
// - LoadFromFile and SaveToFile
// - MarshalToFile and UnmarshalFromFile
// - Automatic directory creation
// - Pretty vs compact file output
//
// Run: go run examples/10_file_operations.go

func main() {
	fmt.Println("📁 JSON Library - File Operations")
	fmt.Println("=================================\n ")

	// Create temporary directory for examples
	tempDir, err := os.MkdirTemp("", "json-file-ops-*")
	if err != nil {
		fmt.Printf("Failed to create temp dir: %v\n", err)
		return
	}
	defer os.RemoveAll(tempDir)

	fmt.Printf("Using temp directory: %s\n\n", tempDir)

	// 1. SAVE TO FILE
	fmt.Println("1️⃣  Save to File")
	fmt.Println("─────────────────")
	demonstrateSaveToFile(tempDir)

	// 2. LOAD FROM FILE
	fmt.Println("\n2️⃣  Load from File")
	fmt.Println("───────────────────")
	demonstrateLoadFromFile(tempDir)

	// 3. MARSHAL TO FILE
	fmt.Println("\n3️⃣  Marshal to File")
	fmt.Println("────────────────────")
	demonstrateMarshalToFile(tempDir)

	// 4. UNMARSHAL FROM FILE
	fmt.Println("\n4️⃣  Unmarshal from File")
	fmt.Println("───────────────────────")
	demonstrateUnmarshalFromFile(tempDir)

	// 5. READ-MODIFY-WRITE
	fmt.Println("\n5️⃣  Read-Modify-Write Pattern")
	fmt.Println("────────────────────────────")
	demonstrateReadModifyWrite(tempDir)

	fmt.Println("\n✅ File operations examples complete!")
}

func demonstrateSaveToFile(tempDir string) {
	// Sample data
	config := map[string]interface{}{
		"version": "1.0.0",
		"server": map[string]interface{}{
			"host": "localhost",
			"port": 8080,
		},
		"features": []string{"auth", "logging", "metrics"},
	}

	// Save with pretty formatting
	prettyPath := filepath.Join(tempDir, "config_pretty.json")
	err := json.SaveToFile(prettyPath, config, true)
	if err != nil {
		fmt.Printf("   Error saving pretty JSON: %v\n", err)
		return
	}
	fmt.Printf("   ✓ Saved pretty JSON to: %s\n", filepath.Base(prettyPath))

	// Save with compact formatting
	compactPath := filepath.Join(tempDir, "config_compact.json")
	err = json.SaveToFile(compactPath, config, false)
	if err != nil {
		fmt.Printf("   Error saving compact JSON: %v\n", err)
		return
	}
	fmt.Printf("   ✓ Saved compact JSON to: %s\n", filepath.Base(compactPath))

	// Show the difference
	fmt.Println("\n   Pretty file content:")
	prettyContent, _ := os.ReadFile(prettyPath)
	fmt.Println("   " + string(prettyContent))

	fmt.Println("\n   Compact file content:")
	compactContent, _ := os.ReadFile(compactPath)
	fmt.Printf("   %s\n", string(compactContent))
}

func demonstrateLoadFromFile(tempDir string) {
	// First create a file
	data := `{
		"user": "Alice",
		"age": 30,
		"active": true
	}`
	filePath := filepath.Join(tempDir, "user.json")
	os.WriteFile(filePath, []byte(data), 0644)

	// Load from file
	jsonStr, err := json.LoadFromFile(filePath)
	if err != nil {
		fmt.Printf("   Error loading file: %v\n", err)
		return
	}
	fmt.Printf("   ✓ Loaded from: %s\n", filepath.Base(filePath))

	// Process the loaded JSON
	name, _ := json.GetString(jsonStr, "user")
	age, _ := json.GetInt(jsonStr, "age")
	active, _ := json.GetBool(jsonStr, "active")

	fmt.Printf("   User: %s, Age: %d, Active: %t\n", name, age, active)
}

func demonstrateMarshalToFile(tempDir string) {
	type User struct {
		ID     int      `json:"id"`
		Name   string   `json:"name"`
		Email  string   `json:"email"`
		Tags   []string `json:"tags"`
		Active bool     `json:"active"`
	}

	user := User{
		ID:     1,
		Name:   "Bob Smith",
		Email:  "bob@example.com",
		Tags:   []string{"developer", "golang"},
		Active: true,
	}

	// Marshal to file with pretty formatting
	filePath := filepath.Join(tempDir, "user_marshal.json")
	err := json.MarshalToFile(filePath, user, true)
	if err != nil {
		fmt.Printf("   Error marshaling to file: %v\n", err)
		return
	}
	fmt.Printf("   ✓ Marshaled struct to: %s\n", filepath.Base(filePath))

	// Show file content
	content, _ := os.ReadFile(filePath)
	fmt.Println("\n   File content:")
	fmt.Println("   " + string(content))
}

func demonstrateUnmarshalFromFile(tempDir string) {
	// First create a file with JSON data
	data := `{
		"id": 2,
		"name": "Charlie",
		"email": "charlie@example.com",
		"tags": ["designer", "ui"],
		"active": true
	}`
	filePath := filepath.Join(tempDir, "user_unmarshal.json")
	os.WriteFile(filePath, []byte(data), 0644)

	// Unmarshal into struct
	type User struct {
		ID     int      `json:"id"`
		Name   string   `json:"name"`
		Email  string   `json:"email"`
		Tags   []string `json:"tags"`
		Active bool     `json:"active"`
	}

	var user User
	err := json.UnmarshalFromFile(filePath, &user)
	if err != nil {
		fmt.Printf("   Error unmarshaling from file: %v\n", err)
		return
	}
	fmt.Printf("   ✓ Unmarshaled from: %s\n", filepath.Base(filePath))
	fmt.Printf("   User: %+v\n", user)

	// Also can unmarshal into map
	var userMap map[string]interface{}
	err = json.UnmarshalFromFile(filePath, &userMap)
	if err == nil {
		fmt.Printf("\n   As map: %+v\n", userMap)
	}
}

func demonstrateReadModifyWrite(tempDir string) {
	// Create initial config file
	initialConfig := `{
		"version": "1.0.0",
		"server": {
			"host": "localhost",
			"port": 8080
		},
		"debug": false
	}`
	configPath := filepath.Join(tempDir, "config.json")
	os.WriteFile(configPath, []byte(initialConfig), 0644)

	fmt.Println("   Initial config:")
	content, _ := os.ReadFile(configPath)
	fmt.Println("   " + string(content))

	// Load, modify, and save
	fmt.Println("\n   Performing modifications:")

	// Load from file
	configStr, err := json.LoadFromFile(configPath)
	if err != nil {
		fmt.Printf("   Error loading: %v\n", err)
		return
	}

	// Modify values
	updated, _ := json.Set(configStr, "version", "1.1.0")
	updated, _ = json.Set(updated, "server.port", 9090)
	updated, _ = json.Set(updated, "debug", true)

	// Add new field
	updated, _ = json.SetWithAdd(updated, "server.ssl", true)

	// Save back
	err = json.SaveToFile(configPath, updated, true)
	if err != nil {
		fmt.Printf("   Error saving: %v\n", err)
		return
	}

	fmt.Println("   ✓ Modified and saved back to file")

	fmt.Println("\n   Updated config:")
	content, _ = os.ReadFile(configPath)
	fmt.Println("   " + string(content))
}
