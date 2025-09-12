package main

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/cybergodev/json"
)

func main() {
	fmt.Println("🚀 JSON File Operations Examples")
	fmt.Println("===================")

	// 0. Create sample data
	createSampleFiles()

	// 1. Basic file operations
	basicFileOperations()

	// 2. Stream operations
	streamOperations()

	// 3. Batch file processing
	batchFileProcessing()

	// 4. File operations with options
	fileOperationsWithOptions()

	// 5. Error handling examples
	errorHandlingExample()

	// Clean up sample files
	cleanupSampleFiles()
}

// Create sample files
func createSampleFiles() {
	// Configuration file
	config := map[string]any{
		"server": map[string]any{
			"host": "localhost",
			"port": 3000,
			"ssl":  false,
		},
		"database": map[string]any{
			"host":     "db.example.com",
			"port":     5432,
			"name":     "myapp",
			"username": "admin",
		},
		"cache": map[string]any{
			"enabled": true,
			"ttl":     300,
			"size":    1000,
		},
	}

	// User data
	users := map[string]any{
		"users": []map[string]any{
			{
				"id":     1,
				"name":   "Alice",
				"email":  "alice@example.com",
				"active": true,
				"roles":  []string{"admin", "user"},
			},
			{
				"id":     2,
				"name":   "Bob",
				"email":  "bob@example.com",
				"active": false,
				"roles":  []string{"user"},
			},
		},
	}

	// Save sample files
	json.SaveToFile("config.json", config, true)
	json.SaveToFile("users.json", users, true)

	// Create a large file for streaming operation demonstration
	largeData := map[string]any{
		"data": make([]map[string]any, 1000),
	}
	for i := 0; i < 1000; i++ {
		largeData["data"].([]map[string]any)[i] = map[string]any{
			"id":    i,
			"value": fmt.Sprintf("item_%d", i),
		}
	}
	json.SaveToFile("large_data.json", largeData, false) // Save in compact format
}

// 1. Basic file operations
func basicFileOperations() {
	fmt.Println("\n1️⃣ Basic File Operations")
	fmt.Println("---------------")

	// 🚀 Load file
	fmt.Println("📖 Load JSON from file:")
	config, err := json.LoadFromFile("config.json")
	if err != nil {
		log.Printf("Failed to load config file: %v", err)
		return
	}

	// Display loaded data
	serverHost, _ := json.GetString(config, "server.host")
	serverPort, _ := json.GetInt(config, "server.port")
	fmt.Printf("   Server config: %s:%d\n", serverHost, serverPort)

	// Modify configuration
	fmt.Println("\n🔧 Modify configuration:")
	updated, err := json.Set(config, "server.port", 8080)
	if err != nil {
		log.Printf("Set operation failed: %v", err)
		return
	}

	updated, err = json.SetWithAdd(updated, "server.tls.enabled", true)
	if err != nil {
		log.Printf("SetWithAdd operation failed: %v", err)
		return
	}

	updated, err = json.SetWithAdd(updated, "server.tls.cert_path", "/etc/ssl/cert.pem")
	if err != nil {
		log.Printf("SetWithAdd operation failed: %v", err)
		return
	}

	newPort, _ := json.GetInt(updated, "server.port")
	tlsEnabled, _ := json.GetBool(updated, "server.tls.enabled")
	fmt.Printf("   New port: %d\n", newPort)
	fmt.Printf("   TLS enabled: %v\n", tlsEnabled)

	// 🚀 Save file
	fmt.Println("\n💾 Save to file:")

	// Save pretty version
	err = json.SaveToFile("config_updated.json", updated, true)
	if err != nil {
		log.Printf("Failed to save pretty config: %v", err)
	} else {
		fmt.Println("   ✅ Pretty version saved to config_updated.json")
	}

	// Save compact version
	err = json.SaveToFile("config_compact.json", updated, false)
	if err != nil {
		log.Printf("Failed to save compact config: %v", err)
	} else {
		fmt.Println("   ✅ Compact version saved to config_compact.json")
	}
}

// 2. Stream operations
func streamOperations() {
	fmt.Println("\n2️⃣ Stream Operations")
	fmt.Println("------------")

	// 🚀 Load from Reader
	fmt.Println("📖 Load large file from Reader:")
	file, err := os.Open("large_data.json")
	if err != nil {
		log.Printf("Failed to open large file: %v", err)
		return
	}
	defer file.Close()

	// Use processor to load data from Reader
	processor := json.New()
	defer processor.Close()

	data, err := processor.LoadFromReader(file)
	if err != nil {
		log.Printf("Failed to load from Reader: %v", err)
		return
	}

	// Convert data to JSON string for using package-level functions
	jsonStr, err := json.Marshal(data)
	if err != nil {
		log.Printf("Failed to serialize data: %v", err)
		return
	}
	jsonString := string(jsonStr)

	// Get data statistics
	dataArray, _ := json.Get(jsonString, "data")
	if arr, ok := dataArray.([]any); ok {
		fmt.Printf("   Loaded %d records\n", len(arr))
	}

	// Process data - add timestamp
	processed, _ := json.SetWithAdd(jsonString, "metadata.processed_at", "2030-01-01T12:00:00Z")
	processed, _ = json.SetWithAdd(processed, "metadata.total_records", len(dataArray.([]any)))

	// 🚀 Save to Writer
	fmt.Println("\n💾 Save to Writer:")

	// Method 1: Write JSON string directly to Buffer
	var buffer1 bytes.Buffer
	_, err = buffer1.WriteString(processed)
	if err != nil {
		log.Printf("Failed to write to Buffer: %v", err)
		return
	}
	fmt.Printf("   ✅ Method 1 - Write string directly, Buffer size: %d bytes\n", buffer1.Len())

	// Method 2: Use SaveToWriter to save parsed data object
	var processedData any
	err = processor.Parse(processed, &processedData)
	if err != nil {
		log.Printf("Failed to parse JSON: %v", err)
		return
	}

	var buffer2 bytes.Buffer
	err = processor.SaveToWriter(&buffer2, processedData, false) // Compact output
	if err != nil {
		log.Printf("SaveToWriter failed: %v", err)
		return
	}
	fmt.Printf("   ✅ Method 2 - SaveToWriter, Buffer size: %d bytes\n", buffer2.Len())

	// Save Buffer content to file (using Method 1 result)
	err = os.WriteFile("processed_data.json", buffer1.Bytes(), 0644)
	if err != nil {
		log.Printf("Failed to write file: %v", err)
	} else {
		fmt.Println("   ✅ Processed data saved to processed_data.json")
	}
}

// 3. Batch file processing
func batchFileProcessing() {
	fmt.Println("\n3️⃣ Batch File Processing")
	fmt.Println("---------------")

	// 🚀 Batch process multiple files
	fmt.Println("📁 Batch process config files:")

	configFiles := []string{
		"config.json",
		"users.json",
	}

	allConfigs := make(map[string]any)

	for _, filename := range configFiles {
		fmt.Printf("   Processing file: %s\n", filename)

		// Load file
		config, err := json.LoadFromFile(filename)
		if err != nil {
			log.Printf("   ❌ Failed to load %s: %v", filename, err)
			continue
		}

		var cfg any
		_ = json.Unmarshal([]byte(config), &cfg)

		// Extract config name (remove .json extension)
		configName := strings.TrimSuffix(filename, ".json")
		allConfigs[configName] = cfg

		fmt.Printf("   ✅ %s loaded successfully\n", filename)
	}

	// Add merge metadata
	allConfigs["metadata"] = map[string]any{
		"merged_at":     "2024-01-01T12:00:00Z",
		"source_files":  configFiles,
		"total_configs": len(configFiles),
	}

	// 🚀 Save merged configuration
	fmt.Println("\n💾 Save merged configuration:")
	err := json.SaveToFile("merged_config.json", allConfigs, true)
	if err != nil {
		log.Printf("Failed to save merged config: %v", err)
	} else {
		fmt.Println("   ✅ Merged configuration saved to merged_config.json")

		// Display merge result statistics
		if metadata, ok := allConfigs["metadata"].(map[string]any); ok {
			if totalConfigs, ok := metadata["total_configs"].(int); ok {
				fmt.Printf("   📊 Merged %d configuration files\n", totalConfigs)
			}
		}
	}
}

// 4. File operations with options
func fileOperationsWithOptions() {
	fmt.Println("\n4️⃣ File Operations with Options")
	fmt.Println("------------------")

	// 🚀 Load file using strict mode
	fmt.Println("🔒 Strict mode loading:")
	strictOpts := &json.ProcessorOptions{
		StrictMode:      true,
		MaxDepth:        10,
		AllowComments:   false,
		PreserveNumbers: true,
	}

	// Use processor for loading with options
	processor := json.New()
	defer processor.Close()

	config, err := processor.LoadFromFile("config.json", strictOpts)
	if err != nil {
		log.Printf("Strict mode loading failed: %v", err)
		return
	}

	fmt.Println("   ✅ Strict mode loading successful")

	// 🚀 Load file using flexible mode
	fmt.Println("\n🔓 Flexible mode loading:")
	flexibleOpts := &json.ProcessorOptions{
		StrictMode:      false,
		MaxDepth:        50,
		AllowComments:   true,
		CreatePaths:     true,
		CleanupNulls:    true,
		ContinueOnError: true,
	}

	config2, err := processor.LoadFromFile("config.json", flexibleOpts)
	if err != nil {
		log.Printf("Flexible mode loading failed: %v", err)
		return
	}

	fmt.Println("   ✅ Flexible mode loading successful")

	// Compare results from both modes
	configStr1, _ := json.Marshal(config)
	configStr2, _ := json.Marshal(config2)
	host1, _ := json.GetString(string(configStr1), "server.host")
	host2, _ := json.GetString(string(configStr2), "server.host")
	fmt.Printf("   Strict mode result: %s\n", host1)
	fmt.Printf("   Flexible mode result: %s\n", host2)
}

// 5. Error handling examples
func errorHandlingExample() {
	fmt.Println("\n5️⃣ Error Handling Examples")
	fmt.Println("---------------")

	// Try to load non-existent file
	fmt.Println("❌ Load non-existent file:")
	_, err := json.LoadFromFile("nonexistent.json")
	if err != nil {
		fmt.Printf("   Expected error: %v\n", err)
	}

	// Try to save to invalid path
	fmt.Println("\n❌ Save to invalid path:")
	testData := map[string]any{"test": "value"}
	err = json.SaveToFile("/invalid/path/file.json", testData, true)
	if err != nil {
		fmt.Printf("   Expected error: %v\n", err)
	}

	// Correct error handling approach
	fmt.Println("\n✅ Correct error handling:")
	configStr, err := json.LoadFromFile("config.json")
	if err != nil {
		log.Printf("Failed to load config, using default config: %v", err)
		// Use default configuration
		defaultConfig := map[string]any{
			"server": map[string]any{
				"host": "localhost",
				"port": 3000,
			},
		}
		// Convert default config to JSON string
		defaultBytes, _ := json.Marshal(defaultConfig)
		configStr = string(defaultBytes)
	}

	host, _ := json.GetString(configStr, "server.host")
	fmt.Printf("   Server host: %s\n", host)
}

// Clean up sample files
func cleanupSampleFiles() {
	fmt.Println("\n🧹 Clean up generated sample files")
	fmt.Println("---------------")

	files := []string{
		"config.json",
		"users.json",
		"large_data.json",
		"config_updated.json",
		"config_compact.json",
		"processed_data.json",
		"merged_config.json",
	}

	for _, file := range files {
		if err := os.Remove(file); err == nil {
			fmt.Printf("   ✅ Deleted: %s\n", file)
		}
	}
}
