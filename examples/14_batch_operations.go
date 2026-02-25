//go:build example

package main

import (
	"fmt"
	"time"

	"github.com/cybergodev/json"
)

// Batch Operations Example
//
// This example demonstrates batch processing capabilities and cache management
// for high-performance JSON operations.
//
// Topics covered:
// - ProcessBatch for mixed batch operations
// - BatchOperation types (get, set, delete)
// - WarmupCache for pre-populating cache
// - BulkProcessor for efficient bulk operations
// - EncodeStream, EncodeBatch, EncodeFields for encoding
// - Path cache warmup
// - Performance optimization techniques
//
// Run: go run examples/14_batch_operations.go

func main() {
	fmt.Println("Batch Operations - JSON Library")
	fmt.Println("================================\n ")

	// 1. PROCESSBATCH FOR MIXED OPERATIONS
	fmt.Println("1. ProcessBatch for Mixed Operations")
	fmt.Println("-------------------------------------")
	demonstrateProcessBatch()

	// 2. CACHE WARMUP
	fmt.Println("\n2. Cache Warmup")
	fmt.Println("----------------")
	demonstrateCacheWarmup()

	// 3. BULK PROCESSOR
	fmt.Println("\n3. Bulk Processor")
	fmt.Println("------------------")
	demonstrateBulkProcessor()

	// 4. ENCODE STREAM/BATCH/FIELDS
	fmt.Println("\n4. Encode Stream/Batch/Fields")
	fmt.Println("------------------------------")
	demonstrateEncodeFunctions()

	// 5. PERFORMANCE COMPARISON
	fmt.Println("\n5. Performance Comparison")
	fmt.Println("--------------------------")
	demonstrateBatchPerformance()

	fmt.Println("\nBatch operations examples complete!")
}

func demonstrateProcessBatch() {
	jsonStr := `{
		"user": {
			"name": "Alice",
			"age": 30,
			"email": "alice@example.com"
		},
		"settings": {
			"theme": "dark",
			"notifications": true
		},
		"version": 1
	}`

	// Define batch operations
	operations := []json.BatchOperation{
		{Type: "get", Path: "user.name", ID: "op1"},
		{Type: "get", Path: "user.age", ID: "op2"},
		{Type: "get", Path: "settings.theme", ID: "op3"},
		{Type: "set", Path: "user.age", Value: 31, ID: "op4"},
		{Type: "get", Path: "user.age", ID: "op5"},
		{Type: "get", Path: "nonexistent", ID: "op6"},
	}

	// Execute batch
	results, err := json.ProcessBatch(operations)
	if err != nil {
		fmt.Printf("   Batch error: %v\n", err)
		return
	}

	fmt.Println("   Batch results:")
	for _, result := range results {
		if result.Error != nil {
			fmt.Printf("   [%s] Error: %v\n", result.ID, result.Error)
		} else {
			fmt.Printf("   [%s] Result: %v\n", result.ID, result.Result)
		}
	}

	// Using with processor for more control
	processor := json.New(json.DefaultConfig())
	defer processor.Close()

	// Note: BatchOperation also supports JSONStr field for different JSON inputs
	operations2 := []json.BatchOperation{
		{Type: "get", JSONStr: jsonStr, Path: "user.name", ID: "get_name"},
		{Type: "get", JSONStr: jsonStr, Path: "user.email", ID: "get_email"},
	}

	results2, err := processor.ProcessBatch(operations2)
	if err != nil {
		fmt.Printf("   Processor batch error: %v\n", err)
		return
	}

	fmt.Println("\n   Processor batch results:")
	for _, result := range results2 {
		fmt.Printf("   [%s] %v\n", result.ID, result.Result)
	}
}

func demonstrateCacheWarmup() {
	jsonStr := `{
		"users": [
			{"id": 1, "name": "Alice", "active": true},
			{"id": 2, "name": "Bob", "active": false}
		],
		"config": {
			"version": "1.0",
			"debug": false
		}
	}`

	// Define frequently accessed paths
	commonPaths := []string{
		"users[0].name",
		"users[0].id",
		"users[0].active",
		"config.version",
		"config.debug",
	}

	// Warmup cache for better first-access performance
	result, err := json.WarmupCache(jsonStr, commonPaths)
	if err != nil {
		fmt.Printf("   Warmup error: %v\n", err)
		return
	}

	fmt.Printf("   Cache warmup results:\n")
	fmt.Printf("   - Total paths: %d\n", result.TotalPaths)
	fmt.Printf("   - Successful: %d\n", result.Successful)
	fmt.Printf("   - Failed: %d\n", result.Failed)
	fmt.Printf("   - Success rate: %.1f%%\n", result.SuccessRate*100)

	if len(result.FailedPaths) > 0 {
		fmt.Printf("   - Failed paths: %v\n", result.FailedPaths)
	}

	// Now access the warmed paths - should be faster
	fmt.Println("\n   Accessing warmed paths:")
	name, _ := json.GetString(jsonStr, "users[0].name")
	fmt.Printf("   - users[0].name: %s\n", name)

	version, _ := json.GetString(jsonStr, "config.version")
	fmt.Printf("   - config.version: %s\n", version)

	// Path cache warmup (for repeated path parsing)
	json.WarmupPathCache(commonPaths)
	fmt.Println("\n   Path cache warmed for common paths")

	// Clear cache when done with this data
	json.ClearCache()
	fmt.Println("   Cache cleared")
}

func demonstrateBulkProcessor() {
	processor := json.New(json.DefaultConfig())
	defer processor.Close()

	// Create bulk processor with batch size
	bulkProcessor := json.NewBulkProcessor(processor, 100)

	jsonStr := `{
		"items": [
			{"id": 1, "value": "a"},
			{"id": 2, "value": "b"},
			{"id": 3, "value": "c"}
		],
		"metadata": {"count": 3}
	}`

	// Bulk get multiple paths efficiently
	paths := []string{
		"items[0].id",
		"items[0].value",
		"items[1].id",
		"items[1].value",
		"metadata.count",
	}

	results, err := bulkProcessor.BulkGet(jsonStr, paths)
	if err != nil {
		fmt.Printf("   Bulk get error: %v\n", err)
		return
	}

	fmt.Println("   Bulk get results:")
	for path, value := range results {
		fmt.Printf("   - %s: %v\n", path, value)
	}

	// BulkProcessor parses JSON once and reuses for all lookups
	fmt.Printf("\n   BulkProcessor batch size: 100\n")
}

func demonstrateEncodeFunctions() {
	// EncodeStream - encode a slice as JSON array
	users := []map[string]any{
		{"id": 1, "name": "Alice"},
		{"id": 2, "name": "Bob"},
		{"id": 3, "name": "Charlie"},
	}

	streamJSON, err := json.EncodeStream(users, true)
	if err != nil {
		fmt.Printf("   EncodeStream error: %v\n", err)
	} else {
		fmt.Println("   EncodeStream result:")
		fmt.Println(streamJSON)
	}

	// EncodeBatch - encode key-value pairs as JSON object
	pairs := map[string]any{
		"user1": map[string]any{"name": "Alice", "active": true},
		"user2": map[string]any{"name": "Bob", "active": false},
		"count": 2,
	}

	batchJSON, err := json.EncodeBatch(pairs, true)
	if err != nil {
		fmt.Printf("   EncodeBatch error: %v\n", err)
	} else {
		fmt.Println("   EncodeBatch result:")
		fmt.Println(batchJSON)
	}

	// EncodeFields - encode only specific fields
	type User struct {
		ID       int    `json:"id"`
		Name     string `json:"name"`
		Email    string `json:"email"`
		Password string `json:"password"`
		Role     string `json:"role"`
	}

	user := User{
		ID:       1,
		Name:     "Alice",
		Email:    "alice@example.com",
		Password: "secret123", // Should not be exposed
		Role:     "admin",
	}

	// Only encode safe fields
	fieldsToEncode := []string{"id", "name", "email", "role"}
	fieldsJSON, err := json.EncodeFields(user, fieldsToEncode, true)
	if err != nil {
		fmt.Printf("   EncodeFields error: %v\n", err)
	} else {
		fmt.Println("   EncodeFields result (password excluded):")
		fmt.Println(fieldsJSON)
	}
}

func demonstrateBatchPerformance() {
	jsonStr := `{
		"user": {
			"id": 1001,
			"name": "Alice Johnson",
			"email": "alice@example.com",
			"profile": {
				"age": 28,
				"location": "NYC"
			}
		},
		"settings": {
			"theme": "dark",
			"notifications": true
		}
	}`

	// Performance comparison: with and without cache
	fmt.Println("   Performance comparison (1000 operations):")

	// Without cache optimization
	configNoCache := &json.Config{
		EnableCache:  false,
		MaxJSONSize:  10 * 1024 * 1024,
		MaxBatchSize: 1000,
	}
	procNoCache := json.New(configNoCache)
	defer procNoCache.Close()

	start := time.Now()
	for i := 0; i < 1000; i++ {
		_, _ = procNoCache.Get(jsonStr, "user.name")
	}
	noCacheDuration := time.Since(start)

	// With cache optimization
	configCache := &json.Config{
		EnableCache:  true,
		MaxCacheSize: 1000,
		CacheTTL:     10 * time.Minute,
		MaxJSONSize:  10 * 1024 * 1024,
		MaxBatchSize: 1000,
	}
	procCache := json.New(configCache)
	defer procCache.Close()

	start = time.Now()
	for i := 0; i < 1000; i++ {
		_, _ = procCache.Get(jsonStr, "user.name")
	}
	cacheDuration := time.Since(start)

	// Get stats from cached processor
	stats := procCache.GetStats()

	fmt.Printf("   Without cache: %v\n", noCacheDuration)
	fmt.Printf("   With cache:    %v\n", cacheDuration)

	speedup := float64(noCacheDuration) / float64(cacheDuration)
	if speedup > 1 {
		fmt.Printf("   Cache speedup: %.1fx faster\n", speedup)
	}

	fmt.Printf("\n   Cache statistics:\n")
	fmt.Printf("   - Cache size: %d\n", stats.CacheSize)
	fmt.Printf("   - Cache hits: %d\n", stats.HitCount)
	fmt.Printf("   - Cache misses: %d\n", stats.MissCount)
	fmt.Printf("   - Hit ratio: %.2f%%\n", stats.HitRatio*100)

	// GetMultiple vs individual Get
	fmt.Println("\n   GetMultiple vs individual Get:")

	paths := []string{"user.name", "user.email", "settings.theme", "user.profile.age"}

	// Individual gets
	start = time.Now()
	for i := 0; i < 100; i++ {
		for _, path := range paths {
			_, _ = procCache.Get(jsonStr, path)
		}
	}
	individualDuration := time.Since(start)

	// Batch get
	start = time.Now()
	for i := 0; i < 100; i++ {
		_, _ = json.GetMultiple(jsonStr, paths)
	}
	batchDuration := time.Since(start)

	fmt.Printf("   Individual Get (4 paths x 100): %v\n", individualDuration)
	fmt.Printf("   GetMultiple (4 paths x 100):    %v\n", batchDuration)
}
