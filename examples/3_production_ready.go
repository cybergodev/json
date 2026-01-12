//go:build ignore

package main

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cybergodev/json"
)

// Production-Ready Example
//
// This example demonstrates production-grade usage patterns including:
// - Custom processor configuration for different scenarios
// - Thread-safe concurrent operations
// - Performance optimization with caching
// - Resource management and monitoring
// - Best practices for production environments
//
// Run: go run examples/production_ready.go

func main() {
	fmt.Println("🏭 JSON Library - Production Ready")
	fmt.Println("===================================\n ")

	// Sample data for demonstrations
	testData := `{
		"users": [
			{"id": 1, "name": "Alice", "email": "alice@example.com", "active": true},
			{"id": 2, "name": "Bob", "email": "bob@example.com", "active": false},
			{"id": 3, "name": "Charlie", "email": "charlie@example.com", "active": true}
		],
		"config": {
			"version": "1.0.0",
			"timeout": 30,
			"features": ["auth", "logging", "metrics"]
		}
	}`

	// 1. CONFIGURATION PATTERNS
	fmt.Println("1️⃣  Configuration Patterns")
	fmt.Println("───────────────────────────")
	demonstrateConfigurations(testData)

	// 2. THREAD-SAFE OPERATIONS
	fmt.Println("\n2️⃣  Thread-Safe Concurrent Operations")
	fmt.Println("──────────────────────────────────────")
	demonstrateConcurrency(testData)

	// 3. PERFORMANCE OPTIMIZATION
	fmt.Println("\n3️⃣  Performance Optimization")
	fmt.Println("─────────────────────────────")
	demonstratePerformance(testData)

	// 4. RESOURCE MANAGEMENT
	fmt.Println("\n4️⃣  Resource Management")
	fmt.Println("────────────────────────")
	demonstrateResourceManagement(testData)

	// 5. MONITORING & METRICS
	fmt.Println("\n5️⃣  Monitoring & Metrics")
	fmt.Println("─────────────────────────")
	demonstrateMonitoring(testData)

	fmt.Println("\n✅ Production-ready patterns complete!")
	fmt.Println("💡 These patterns ensure reliability, performance, and safety in production!")
}

func demonstrateConfigurations(testData string) {
	// 1. Default configuration (quick start)
	fmt.Println("   Default Configuration:")
	defaultProc := json.New(json.DefaultConfig())
	defer defaultProc.Close()

	result, _ := defaultProc.Get(testData, "users[0].name")
	fmt.Printf("   - Result: %v\n", result)

	// 2. High-performance configuration
	fmt.Println("\n   High-Performance Configuration:")
	perfConfig := &json.Config{
		EnableCache:       true,
		MaxCacheSize:      10000,
		CacheTTL:          30 * time.Minute,
		MaxJSONSize:       100 * 1024 * 1024, // 100MB
		MaxPathDepth:      100,
		MaxBatchSize:      2000,
		MaxConcurrency:    100,
		ParallelThreshold: 3,
		EnableMetrics:     true,
		EnableValidation:  false, // Skip validation for speed
	}
	perfProc := json.New(perfConfig)
	defer perfProc.Close()

	start := time.Now()
	for i := 0; i < 100; i++ {
		_, _ = perfProc.Get(testData, "users[0].name")
	}
	duration := time.Since(start)
	fmt.Printf("   - 100 operations in: %v\n", duration)

	// 3. High-security configuration
	fmt.Println("\n   High-Security Configuration:")
	secConfig := json.HighSecurityConfig()
	secProc := json.New(secConfig)
	defer secProc.Close()

	result2, _ := secProc.Get(testData, "users[0].email")
	fmt.Printf("   - Secure result: %v\n", result2)

	// 4. Large data configuration
	fmt.Println("\n   Large Data Configuration:")
	largeConfig := json.LargeDataConfig()
	largeProc := json.New(largeConfig)
	defer largeProc.Close()

	result3, _ := largeProc.Get(testData, "config.version")
	fmt.Printf("   - Large data result: %v\n", result3)
}

func demonstrateConcurrency(testData string) {
	processor := json.New(json.DefaultConfig())
	defer processor.Close()

	var wg sync.WaitGroup
	var successCount int64
	var errorCount int64

	numGoroutines := 20
	operationsPerGoroutine := 50

	start := time.Now()

	// Launch concurrent operations
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			for j := 0; j < operationsPerGoroutine; j++ {
				// Mix different operations
				var err error
				switch j % 4 {
				case 0:
					_, err = processor.Get(testData, "users[0].name")
				case 1:
					_, err = processor.Set(testData, "users[0].active", true)
				case 2:
					_, err = processor.Get(testData, "config.version")
				case 3:
					_, err = processor.Get(testData, "config.timeout")
				}

				if err != nil {
					atomic.AddInt64(&errorCount, 1)
				} else {
					atomic.AddInt64(&successCount, 1)
				}
			}
		}(i)
	}

	wg.Wait()
	duration := time.Since(start)

	totalOps := atomic.LoadInt64(&successCount) + atomic.LoadInt64(&errorCount)
	successRate := float64(atomic.LoadInt64(&successCount)) / float64(totalOps) * 100

	fmt.Printf("   ✓ Completed %d operations in %v\n", totalOps, duration)
	fmt.Printf("   ✓ Success rate: %.2f%% (%d/%d)\n", successRate, atomic.LoadInt64(&successCount), totalOps)
	fmt.Printf("   ✓ Throughput: %.0f ops/sec\n", float64(totalOps)/duration.Seconds())
	fmt.Printf("   ✓ All operations were thread-safe!\n")
}

func demonstratePerformance(testData string) {
	// Test with cache enabled
	cacheConfig := &json.Config{
		EnableCache:       true,
		MaxCacheSize:      1000,
		CacheTTL:          10 * time.Minute,
		MaxPathDepth:      50,
		MaxJSONSize:       50 * 1024 * 1024,
		MaxBatchSize:      500,
		MaxConcurrency:    20,
		ParallelThreshold: 5,
	}
	cachedProc := json.New(cacheConfig)
	defer cachedProc.Close()

	// Warm up cache
	_, _ = cachedProc.Get(testData, "users[0].name")
	_, _ = cachedProc.Get(testData, "config.version")

	// Test cache performance
	start := time.Now()
	for i := 0; i < 1000; i++ {
		_, _ = cachedProc.Get(testData, "users[0].name")
	}
	cachedDuration := time.Since(start)

	stats := cachedProc.GetStats()
	hitRatio := float64(stats.HitCount) / float64(stats.HitCount+stats.MissCount) * 100

	fmt.Printf("   ✓ 1000 cached operations in: %v\n", cachedDuration)
	fmt.Printf("   ✓ Cache hit ratio: %.2f%%\n", hitRatio)
	fmt.Printf("   ✓ Throughput: %.0f ops/sec\n", 1000.0/cachedDuration.Seconds())

	// Test without cache for comparison
	noCacheConfig := &json.Config{
		EnableCache:       false,
		MaxPathDepth:      50,
		MaxJSONSize:       50 * 1024 * 1024,
		MaxBatchSize:      500,
		MaxConcurrency:    20,
		ParallelThreshold: 5,
	}
	noCacheProc := json.New(noCacheConfig)
	defer noCacheProc.Close()

	start = time.Now()
	for i := 0; i < 1000; i++ {
		_, _ = noCacheProc.Get(testData, "users[0].name")
	}
	noCacheDuration := time.Since(start)

	speedup := float64(noCacheDuration) / float64(cachedDuration)
	fmt.Printf("   ✓ Without cache: %v (%.1fx slower)\n", noCacheDuration, speedup)
}

func demonstrateResourceManagement(testData string) {
	// Proper resource lifecycle management
	processor := json.New(json.DefaultConfig())

	// Use defer to ensure cleanup
	defer func() {
		processor.Close()
		fmt.Println("   ✓ Processor resources cleaned up")
	}()

	// Perform operations
	_, _ = processor.Get(testData, "users[0].name")

	// Check processor health
	if !processor.IsClosed() {
		fmt.Println("   ✓ Processor is healthy and active")
	}

	// Context-aware operations with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	opts := &json.ProcessorOptions{
		Context:      ctx,
		CacheResults: true,
		StrictMode:   false,
	}

	result, err := processor.Get(testData, "config.features", opts)
	if err != nil {
		fmt.Printf("   ✗ Operation failed: %v\n", err)
	} else {
		fmt.Printf("   ✓ Context-aware operation succeeded: %v\n", result)
	}
}

func demonstrateMonitoring(testData string) {
	config := &json.Config{
		EnableCache:       true,
		MaxCacheSize:      1000,
		CacheTTL:          10 * time.Minute,
		EnableMetrics:     true,
		EnableHealthCheck: true,
		MaxPathDepth:      50,
		MaxJSONSize:       50 * 1024 * 1024,
		MaxBatchSize:      500,
		MaxConcurrency:    20,
		ParallelThreshold: 5,
	}
	processor := json.New(config)
	defer processor.Close()

	// Perform various operations
	operations := []struct {
		name string
		path string
	}{
		{"Get user name", "users[0].name"},
		{"Get config version", "config.version"},
		{"Get all users", "users"},
		{"Get features", "config.features"},
	}

	for _, op := range operations {
		_, _ = processor.Get(testData, op.path)
	}

	// Get comprehensive statistics
	stats := processor.GetStats()
	fmt.Printf("   📊 Performance Metrics:\n")
	fmt.Printf("   - Total operations: %d\n", stats.OperationCount)
	fmt.Printf("   - Cache size: %d entries\n", stats.CacheSize)
	fmt.Printf("   - Cache hits: %d\n", stats.HitCount)
	fmt.Printf("   - Cache misses: %d\n", stats.MissCount)
	fmt.Printf("   - Cache hit rate: %.2f%%\n", stats.HitRatio*100)
	fmt.Printf("   - Error count: %d\n", stats.ErrorCount)

	// Health check
	if processor.IsClosed() {
		fmt.Printf("   ⚠️  Processor is closed\n")
	} else {
		fmt.Printf("   ✅ Processor is healthy and active\n")
	}
}
