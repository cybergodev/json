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
// Run: go run -tags=example examples/13_batch_operations.go

func main() {
	fmt.Println("JSON Library - Batch Operations")
	fmt.Println("==================================")

	// 1. PROCESSBATCH FOR MIXED OPERATIONS
	demonstrateProcessBatch()

	// 2. CACHE WARMUP
	demonstrateCacheWarmup()

	// 3. BULK PROCESSOR
	demonstrateBulkProcessor()

	// 4. ENCODE STREAM/BATCH/FIELDS
	demonstrateEncodeFunctions()

	// 5. PERFORMANCE COMPARISON
	demonstrateBatchPerformance()

	fmt.Println("\nBatch operations examples complete!")
}

func demonstrateProcessBatch() {
	fmt.Println("1. ProcessBatch for Mixed Operations")
	fmt.Println("-------------------------------------")

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

	// Define batch operations with JSONStr for each operation.
	// Each operation is independent — a "set" does not affect subsequent "get"s
	// because each BatchOperation carries its own JSONStr.
	operations := []json.BatchOperation{
		{Type: "get", JSONStr: jsonStr, Path: "user.name", ID: "op1"},
		{Type: "get", JSONStr: jsonStr, Path: "user.age", ID: "op2"},
		{Type: "get", JSONStr: jsonStr, Path: "settings.theme", ID: "op3"},
		{Type: "set", JSONStr: jsonStr, Path: "user.age", Value: 31, ID: "op4"},
		{Type: "get", JSONStr: jsonStr, Path: "nonexistent", ID: "op5"},
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
	processor, err := json.New(json.DefaultConfig())
	if err != nil {
		fmt.Printf("   New error: %v\n", err)
		return
	}
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
	fmt.Println("\n2. Cache Warmup")
	fmt.Println("----------------")

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
	name := json.GetString(jsonStr, "users[0].name", "")
	fmt.Printf("   - users[0].name: %s\n", name)

	version := json.GetString(jsonStr, "config.version", "")
	fmt.Printf("   - config.version: %s\n", version)

	// Clear cache when done with this data
	json.ClearCache()
	fmt.Println("   Cache cleared")
}

func demonstrateBulkProcessor() {
	fmt.Println("\n3. Bulk Operations with GetMultiple")
	fmt.Println("------------------------------------")

	processor, err := json.New(json.DefaultConfig())
	if err != nil {
		fmt.Printf("   New error: %v\n", err)
		return
	}
	defer processor.Close()

	jsonStr := `{
		"items": [
			{"id": 1, "value": "a"},
			{"id": 2, "value": "b"},
			{"id": 3, "value": "c"}
		],
		"metadata": {"count": 3}
	}`

	// Use GetMultiple for efficient bulk retrieval
	paths := []string{
		"items[0].id",
		"items[0].value",
		"items[1].id",
		"items[1].value",
		"metadata.count",
	}

	// GetMultiple is optimized for batch operations
	results, err := json.GetMultiple(jsonStr, paths)
	if err != nil {
		fmt.Printf("   GetMultiple error: %v\n", err)
		return
	}

	fmt.Println("   GetMultiple results:")
	for path, value := range results {
		fmt.Printf("   - %s: %v\n", path, value)
	}

	// Processor-level batch operations
	fmt.Println("\n   Using Processor.GetMultiple:")
	procResults, err := processor.GetMultiple(jsonStr, paths)
	if err != nil {
		fmt.Printf("   Processor GetMultiple error: %v\n", err)
		return
	}

	for path, value := range procResults {
		fmt.Printf("   - %s: %v\n", path, value)
	}
}

func demonstrateEncodeFunctions() {
	fmt.Println("\n4. Encode Stream/Batch/Fields")
	fmt.Println("------------------------------")

	// EncodeStream - encode a slice as JSON array
	users := []map[string]any{
		{"id": 1, "name": "Alice"},
		{"id": 2, "name": "Bob"},
		{"id": 3, "name": "Charlie"},
	}

	opts := json.DefaultConfig()
	opts.Pretty = true

	streamJSON, err := json.EncodeStream(users, opts)
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

	batchJSON, err := json.EncodeBatch(pairs, opts)
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
	fieldsJSON, err := json.EncodeFields(user, fieldsToEncode, opts)
	if err != nil {
		fmt.Printf("   EncodeFields error: %v\n", err)
	} else {
		fmt.Println("   EncodeFields result (password excluded):")
		fmt.Println(fieldsJSON)
	}
}

func demonstrateBatchPerformance() {
	fmt.Println("\n5. Performance Comparison")
	fmt.Println("--------------------------")

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
	// Note: benchmark loops discard errors to avoid skewing measurements.
	fmt.Println("   Performance comparison (1000 operations):")

	// Without cache optimization
	configNoCache := json.DefaultConfig()
	configNoCache.EnableCache = false
	procNoCache, _ := json.New(configNoCache) // OK: config validated above
	defer procNoCache.Close()

	start := time.Now()
	for i := 0; i < 1000; i++ {
		_, _ = procNoCache.Get(jsonStr, "user.name")
	}
	noCacheDuration := time.Since(start)

	// With cache optimization
	configCache := json.DefaultConfig()
	configCache.EnableCache = true
	configCache.MaxCacheSize = 1000
	configCache.CacheTTL = 10 * time.Minute
	procCache, _ := json.New(configCache) // OK: config validated above
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
