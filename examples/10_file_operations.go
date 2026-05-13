//go:build example

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
// Run: go run -tags=example examples/10_file_operations.go

func main() {
	fmt.Println("JSON Library - File Operations")
	fmt.Println("=================================")

	// Create temporary directory for examples
	tempDir, err := os.MkdirTemp("", "json-file-ops-*")
	if err != nil {
		fmt.Printf("Failed to create temp dir: %v\n", err)
		return
	}
	defer os.RemoveAll(tempDir)

	fmt.Printf("Using temp directory: %s\n\n", tempDir)

	// 1. SAVE TO FILE
	demonstrateSaveToFile(tempDir)

	// 2. LOAD FROM FILE
	demonstrateLoadFromFile(tempDir)

	// 3. MARSHAL TO FILE
	demonstrateMarshalToFile(tempDir)

	// 4. UNMARSHAL FROM FILE
	demonstrateUnmarshalFromFile(tempDir)

	// 5. READ-MODIFY-WRITE
	demonstrateReadModifyWrite(tempDir)

	fmt.Println("\nFile operations examples complete!")
}

func demonstrateSaveToFile(tempDir string) {
	fmt.Println("1. Save to File")
	fmt.Println("-----------------")

	// Sample data
	config := map[string]any{
		"version": "1.0.0",
		"server": map[string]any{
			"host": "localhost",
			"port": 8080,
		},
		"features": []string{"auth", "logging", "metrics"},
	}

	// Save with pretty formatting
	prettyPath := filepath.Join(tempDir, "config_pretty.json")
	opts := json.DefaultConfig()
	opts.Pretty = true
	err := json.SaveToFile(prettyPath, config, opts)
	if err != nil {
		fmt.Printf("   Error saving pretty JSON: %v\n", err)
		return
	}
	fmt.Printf("   ✓ Saved pretty JSON to: %s\n", filepath.Base(prettyPath))

	// Save with compact formatting
	compactPath := filepath.Join(tempDir, "config_compact.json")
	err = json.SaveToFile(compactPath, config, json.DefaultConfig())
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
	fmt.Println("\n2. Load from File")
	fmt.Println("-------------------")

	// First create a file
	data := `{
		"user": "Alice",
		"age": 30,
		"active": true
	}`
	filePath := filepath.Join(tempDir, "user.json")
	if err := os.WriteFile(filePath, []byte(data), 0644); err != nil {
		fmt.Printf("   Error writing file: %v\n", err)
		return
	}

	// Load from file
	jsonStr, err := json.LoadFromFile(filePath)
	if err != nil {
		fmt.Printf("   Error loading file: %v\n", err)
		return
	}
	fmt.Printf("   ✓ Loaded from: %s\n", filepath.Base(filePath))

	// Process the loaded JSON
	name := json.GetString(jsonStr, "user", "")
	age := json.GetInt(jsonStr, "age", 0)
	active := json.GetBool(jsonStr, "active", false)

	fmt.Printf("   User: %s, Age: %d, Active: %t\n", name, age, active)
}

func demonstrateMarshalToFile(tempDir string) {
	fmt.Println("\n3. Marshal to File")
	fmt.Println("--------------------")

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
	opts := json.DefaultConfig()
	opts.Pretty = true
	err := json.MarshalToFile(filePath, user, opts)
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
	fmt.Println("\n4.  Unmarshal from File")
	fmt.Println("-----------------------")

	// First create a file with JSON data
	data := `{
		"id": 2,
		"name": "Charlie",
		"email": "charlie@example.com",
		"tags": ["designer", "ui"],
		"active": true
	}`
	filePath := filepath.Join(tempDir, "user_unmarshal.json")
	if err := os.WriteFile(filePath, []byte(data), 0644); err != nil {
		fmt.Printf("   Error writing file: %v\n", err)
		return
	}

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
	var userMap map[string]any
	err = json.UnmarshalFromFile(filePath, &userMap)
	if err == nil {
		fmt.Printf("\n   As map: %+v\n", userMap)
	}
}

func demonstrateReadModifyWrite(tempDir string) {
	fmt.Println("\n5.  Read-Modify-Write Pattern")
	fmt.Println("----------------------------")

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
	if err := os.WriteFile(configPath, []byte(initialConfig), 0644); err != nil {
		fmt.Printf("   Error writing config file: %v\n", err)
		return
	}

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
	updated, err := json.Set(configStr, "version", "1.1.0")
	if err != nil {
		fmt.Printf("   Error setting version: %v\n", err)
		return
	}
	updated, err = json.Set(updated, "server.port", 9090)
	if err != nil {
		fmt.Printf("   Error setting port: %v\n", err)
		return
	}
	updated, _ = json.Set(updated, "debug", true) // OK: valid path, valid value

	// Add new field with automatic path creation using Config
	cfg := json.DefaultConfig()
	cfg.CreatePaths = true
	updated, err = json.Set(updated, "server.ssl", true, cfg)
	if err != nil {
		fmt.Printf("   Error setting ssl: %v\n", err)
		return
	}

	// Save back
	opts := json.DefaultConfig()
	opts.Pretty = true
	err = json.SaveToFile(configPath, updated, opts)
	if err != nil {
		fmt.Printf("   Error saving: %v\n", err)
		return
	}

	fmt.Println("   ✓ Modified and saved back to file")

	fmt.Println("\n   Updated config:")
	content, _ = os.ReadFile(configPath)
	fmt.Println("   " + string(content))
}
