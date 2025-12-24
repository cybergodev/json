//go:build examples

package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/cybergodev/json"
)

// User represents a user in our system
type User struct {
	ID       int      `json:"id"`
	Name     string   `json:"name"`
	Email    string   `json:"email"`
	Age      int      `json:"age"`
	Roles    []string `json:"roles"`
	Settings Settings `json:"settings"`
}

// Settings represents user preferences
type Settings struct {
	Theme         string `json:"theme"`
	Notifications bool   `json:"notifications"`
	Language      string `json:"language"`
}

func main() {
	fmt.Println("=== JSON File Marshal/Unmarshal Example ===")

	// Create example data
	user := User{
		ID:    1001,
		Name:  "Alice Johnson",
		Email: "alice@example.com",
		Age:   28,
		Roles: []string{"admin", "developer"},
		Settings: Settings{
			Theme:         "dark",
			Notifications: true,
			Language:      "en",
		},
	}

	// Create output directory
	outputDir := "output"
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		log.Fatalf("Failed to create output directory: %v", err)
	}

	// Example 1: Marshal to file (compact format)
	fmt.Println("\n1. Marshaling to file (compact format)...")
	compactPath := filepath.Join(outputDir, "user_compact.json")
	if err := json.MarshalToFile(compactPath, user); err != nil {
		log.Fatalf("Failed to marshal to compact file: %v", err)
	}
	fmt.Printf("✓ Saved compact JSON to: %s\n", compactPath)

	// Show compact content
	if content, err := os.ReadFile(compactPath); err == nil {
		fmt.Printf("Compact content: %s\n", string(content))
	}

	// Example 2: Marshal to file (pretty format)
	fmt.Println("\n2. Marshaling to file (pretty format)...")
	prettyPath := filepath.Join(outputDir, "user_pretty.json")
	if err := json.MarshalToFile(prettyPath, user, true); err != nil {
		log.Fatalf("Failed to marshal to pretty file: %v", err)
	}
	fmt.Printf("✓ Saved pretty JSON to: %s\n", prettyPath)

	// Show pretty content
	if content, err := os.ReadFile(prettyPath); err == nil {
		fmt.Printf("Pretty content:\n%s\n", string(content))
	}

	// Example 3: Unmarshal from file
	fmt.Println("\n3. Unmarshaling from file...")
	var loadedUser User
	if err := json.UnmarshalFromFile(prettyPath, &loadedUser); err != nil {
		log.Fatalf("Failed to unmarshal from file: %v", err)
	}
	fmt.Printf("✓ Loaded user from file: %+v\n", loadedUser)

	// Example 4: Working with maps
	fmt.Println("\n4. Working with map data...")
	configData := map[string]any{
		"app_name":  "MyApp",
		"version":   "1.2.3",
		"debug":     true,
		"max_users": 1000,
		"features":  []string{"auth", "logging", "metrics"},
		"database": map[string]any{
			"host":     "localhost",
			"port":     5432,
			"username": "admin",
		},
	}

	configPath := filepath.Join(outputDir, "config.json")
	if err := json.MarshalToFile(configPath, configData, true); err != nil {
		log.Fatalf("Failed to marshal config: %v", err)
	}
	fmt.Printf("✓ Saved config to: %s\n", configPath)

	// Load config back
	var loadedConfig map[string]any
	if err := json.UnmarshalFromFile(configPath, &loadedConfig); err != nil {
		log.Fatalf("Failed to unmarshal config: %v", err)
	}
	fmt.Printf("✓ Loaded config: app_name=%s, version=%s\n",
		loadedConfig["app_name"], loadedConfig["version"])

	// Example 5: Using processor for advanced operations
	fmt.Println("\n5. Using processor for advanced operations...")
	processor := json.New()
	defer processor.Close()

	advancedPath := filepath.Join(outputDir, "advanced.json")
	if err := processor.MarshalToFile(advancedPath, user, true); err != nil {
		log.Fatalf("Failed to marshal with processor: %v", err)
	}

	var advancedUser User
	if err := processor.UnmarshalFromFile(advancedPath, &advancedUser); err != nil {
		log.Fatalf("Failed to unmarshal with processor: %v", err)
	}
	fmt.Printf("✓ Processor operations completed successfully\n")

	// Example 6: Error handling
	fmt.Println("\n6. Error handling examples...")

	// Try to unmarshal from non-existent file
	var dummy User
	if err := json.UnmarshalFromFile("nonexistent.json", &dummy); err != nil {
		fmt.Printf("✓ Expected error for non-existent file: %v\n", err)
	}

	// Try to marshal to invalid path (empty path)
	if err := json.MarshalToFile("", user); err != nil {
		fmt.Printf("✓ Expected error for invalid path: %v\n", err)
	}

	// Try to unmarshal to nil target
	if err := json.UnmarshalFromFile(prettyPath, nil); err != nil {
		fmt.Printf("✓ Expected error for nil target: %v\n", err)
	}

	fmt.Println("\n=== Example completed successfully! ===")
	fmt.Printf("Check the '%s' directory for generated JSON files.\n", outputDir)
}
