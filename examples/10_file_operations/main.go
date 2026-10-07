//go:build example

package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

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
// - Reader/Writer based I/O (LoadFromReader, SaveToWriter)
// - Iterating JSON directly from files (ForeachFile family)
// - Pretty vs compact file output
//
// Run: go run -tags=example ./examples/10_file_operations

// User is the sample struct shared by the marshal/unmarshal demos below.
type User struct {
	ID     int      `json:"id"`
	Name   string   `json:"name"`
	Email  string   `json:"email"`
	Tags   []string `json:"tags"`
	Active bool     `json:"active"`
}

// readFile reads a file for display. Each file read here was just written
// above, so a read failure is unexpected; surface it inline instead of
// silently printing an empty string.
func readFile(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return fmt.Sprintf("<read error: %v>", err)
	}
	return string(b)
}

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

	// 6. READER/WRITER I/O
	demonstrateReaderWriter()

	// 7. FILE ITERATION
	demonstrateFileIteration(tempDir)

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
	fmt.Println("   " + readFile(prettyPath))

	fmt.Println("\n   Compact file content:")
	fmt.Printf("   %s\n", readFile(compactPath))
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
	fmt.Println("\n   File content:")
	fmt.Println("   " + readFile(filePath))
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
	fmt.Println("   " + readFile(configPath))

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
	updated, err = json.Set(updated, "debug", true)
	if err != nil {
		fmt.Printf("   Error setting debug: %v\n", err)
		return
	}

	// SetCreate adds a new nested field, auto-creating "server.ssl" — no Config needed.
	updated, err = json.SetCreate(updated, "server.ssl", true)
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
	fmt.Println("   " + readFile(configPath))
}

func demonstrateReaderWriter() {
	fmt.Println("\n6. Reader/Writer I/O (LoadFromReader / SaveToWriter)")
	fmt.Println("------------------------------------------------------")

	// LoadFromReader reads JSON from any io.Reader — network bodies, pipes,
	// embedded assets — and returns it as a string (files: LoadFromFile).
	r := strings.NewReader(`{"source": "reader", "port": 5432}`)
	jsonStr, err := json.LoadFromReader(r)
	if err != nil {
		fmt.Printf("   LoadFromReader error: %v\n", err)
		return
	}
	port := json.GetInt(jsonStr, "port", 0)
	fmt.Printf("   LoadFromReader: source=%s, port=%d\n",
		json.GetString(jsonStr, "source", ""), port)

	// SaveToWriter encodes any value to any io.Writer — no temp file needed.
	var buf bytes.Buffer
	if err := json.SaveToWriter(&buf, map[string]any{"saved": "to writer", "ok": true}); err != nil {
		fmt.Printf("   SaveToWriter error: %v\n", err)
		return
	}
	fmt.Printf("   SaveToWriter: %s\n", buf.String())
}

func demonstrateFileIteration(tempDir string) {
	fmt.Println("\n7. File Iteration (ForeachFile family)")
	fmt.Println("----------------------------------------")

	// A JSON array on disk — the ForeachFile family streams it without
	// loading more than one element into the callback at a time.
	usersPath := filepath.Join(tempDir, "users.json")
	usersJSON := `[
		{"id": 1, "name": "Alice", "active": true},
		{"id": 2, "name": "Bob", "active": false},
		{"id": 3, "name": "Carol", "active": true}
	]`
	if err := os.WriteFile(usersPath, []byte(usersJSON), 0644); err != nil {
		fmt.Printf("   Error writing users file: %v\n", err)
		return
	}

	// ForeachFile: iterate the top-level array element by element. Returning
	// item.Break() stops without an error; any other error aborts and returns.
	fmt.Println("   ForeachFile over users.json:")
	err := json.ForeachFile(usersPath, func(key any, item *json.IterableValue) error {
		fmt.Printf("   - [%v] id=%d name=%s\n", key, item.GetInt("id"), item.GetString("name"))
		return nil
	})
	if err != nil {
		fmt.Printf("   ForeachFile error: %v\n", err)
	}

	// ForeachFileChunked: same file, processed in fixed-size batches — the
	// building block for bulk-loading large datasets.
	fmt.Println("\n   ForeachFileChunked (batch size 2):")
	err = json.ForeachFileChunked(usersPath, 2, func(chunk []*json.IterableValue) error {
		ids := make([]int, 0, len(chunk)) // ids collected for display
		for _, item := range chunk {
			ids = append(ids, item.GetInt("id"))
		}
		fmt.Printf("   - batch: ids=%v\n", ids)
		return nil
	})
	if err != nil {
		fmt.Printf("   ForeachFileChunked error: %v\n", err)
	}

	// ForeachFileWithPath: navigate into the file's JSON first, then iterate —
	// the file holds an object, we iterate its "users" array.
	objectPath := filepath.Join(tempDir, "data_object.json")
	if err := os.WriteFile(objectPath, []byte(`{"users": [{"id": 10}, {"id": 20}]}`), 0644); err != nil {
		fmt.Printf("   Error writing object file: %v\n", err)
		return
	}
	fmt.Println("\n   ForeachFileWithPath(data_object.json, \"users\"):")
	err = json.ForeachFileWithPath(objectPath, "users", func(key any, item *json.IterableValue) error {
		fmt.Printf("   - users[%v]: id=%d\n", key, item.GetInt("id"))
		return nil
	})
	if err != nil {
		fmt.Printf("   ForeachFileWithPath error: %v\n", err)
	}

	// ForeachFileNested: recursive walk of every value in the file.
	count := 0
	err = json.ForeachFileNested(usersPath, func(key any, item *json.IterableValue) error {
		count++
		return nil
	})
	if err != nil {
		fmt.Printf("   ForeachFileNested error: %v\n", err)
		return
	}
	fmt.Printf("\n   ForeachFileNested visited %d values in users.json\n", count)
}
