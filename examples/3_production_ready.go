//go:build example

package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
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
// - The global processor (SetGlobalProcessor / ShutdownGlobalProcessor)
// - Health status reporting
//
// Run: go run -tags=example examples/3_production_ready.go

func main() {
	fmt.Println("JSON Library - Production Ready")
	fmt.Println("================================")

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
	demonstrateConfigurations(testData)

	// 2. THREAD-SAFE OPERATIONS
	demonstrateConcurrency(testData)

	// 3. PERFORMANCE OPTIMIZATION
	demonstratePerformance(testData)

	// 4. RESOURCE MANAGEMENT
	demonstrateResourceManagement(testData)

	// 5. MONITORING & METRICS
	demonstrateMonitoring(testData)

	// 6. GLOBAL PROCESSOR & HEALTH
	demonstrateGlobalProcessor(testData)

	fmt.Println("\nProduction-ready patterns complete!")
}

func demonstrateConfigurations(testData string) {
	fmt.Println("1. Configuration Patterns")
	fmt.Println("--------------------------")

	// 1. Default configuration (quick start)
	fmt.Println("   Default Configuration:")
	defaultProc, err := json.New(json.DefaultConfig())
	if err != nil {
		fmt.Printf("   New error: %v\n", err)
		return
	}
	defer defaultProc.Close()

	result, _ := defaultProc.Get(testData, "users[0].name")
	fmt.Printf("   - Result: %v\n", result)

	// 2. High-performance configuration
	fmt.Println("\n   High-Performance Configuration:")
	perfConfig := json.DefaultConfig()
	perfConfig.MaxCacheSize = 10000
	perfConfig.CacheTTL = 30 * time.Minute
	perfConfig.ParallelThreshold = 3
	perfConfig.EnableMetrics = true
	perfConfig.EnableValidation = false // Skip validation for speed
	perfProc, _ := json.New(perfConfig) // OK: DefaultConfig-derived, always valid
	defer perfProc.Close()

	// Benchmark loop: errors discarded to avoid skewing the measurement.
	start := time.Now()
	for i := 0; i < 100; i++ {
		_, _ = perfProc.Get(testData, "users[0].name")
	}
	duration := time.Since(start)
	fmt.Printf("   - 100 operations in: %v\n", duration)

	// 3. Security configuration
	fmt.Println("\n   Security Configuration:")
	secConfig := json.SecurityConfig()
	secProc, _ := json.New(secConfig) // OK: SecurityConfig-derived, always valid
	defer secProc.Close()

	result2, _ := secProc.Get(testData, "users[0].email")
	fmt.Printf("   - Secure result: %v\n", result2)

	// 4. Large data configuration (use SecurityConfig with adjusted limits)
	fmt.Println("\n   Large Data Configuration:")
	largeConfig := json.SecurityConfig()
	largeConfig.MaxJSONSize = 100 * 1024 * 1024 // 100MB
	largeConfig.MaxNestingDepthSecurity = 100
	largeProc, _ := json.New(largeConfig) // OK: SecurityConfig-derived, always valid
	defer largeProc.Close()

	result3, _ := largeProc.Get(testData, "config.version")
	fmt.Printf("   - Large data result: %v\n", result3)

	// 5. Config self-validation: invalid values are clamped in place;
	// ValidateWithWarnings reports every correction New() will bake in.
	fmt.Println("\n   Config Validation (ValidateWithWarnings):")
	checked := json.DefaultConfig()
	checked.MaxCacheSize = -5 // invalid: negative
	for _, w := range checked.ValidateWithWarnings() {
		fmt.Printf("   - %s: %v -> %v (%s)\n", w.Field, w.OldValue, w.NewValue, w.Reason)
	}
}

func demonstrateConcurrency(testData string) {
	fmt.Println("\n2. Thread-Safe Concurrent Operations")
	fmt.Println("--------------------------------------")

	processor, _ := json.New(json.DefaultConfig()) // OK: DefaultConfig always valid
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

	fmt.Printf("   Completed %d operations in %v\n", totalOps, duration)
	fmt.Printf("   Success rate: %.2f%% (%d/%d)\n", successRate, atomic.LoadInt64(&successCount), totalOps)
	fmt.Printf("   Throughput: %.0f ops/sec\n", float64(totalOps)/duration.Seconds())
	fmt.Printf("   All operations were thread-safe!\n")
}

func demonstratePerformance(testData string) {
	fmt.Println("\n3. Performance Optimization")
	fmt.Println("---------------------------")

	// Note: benchmark loops discard errors to avoid skewing measurements.
	// Test with cache enabled
	cacheConfig := json.DefaultConfig()
	cacheConfig.EnableCache = true
	cacheConfig.MaxCacheSize = 1000
	cacheConfig.CacheTTL = 10 * time.Minute
	cachedProc, _ := json.New(cacheConfig) // OK: DefaultConfig-derived
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

	fmt.Printf("   1000 cached operations in: %v\n", cachedDuration)
	fmt.Printf("   Cache hit ratio: %.2f%%\n", stats.HitRatio*100)
	fmt.Printf("   Throughput: %.0f ops/sec\n", 1000.0/cachedDuration.Seconds())

	// Test without cache for comparison
	noCacheConfig := json.DefaultConfig()
	noCacheConfig.EnableCache = false
	noCacheProc, _ := json.New(noCacheConfig) // OK: DefaultConfig-derived
	defer noCacheProc.Close()

	start = time.Now()
	for i := 0; i < 1000; i++ {
		_, _ = noCacheProc.Get(testData, "users[0].name")
	}
	noCacheDuration := time.Since(start)

	speedup := float64(noCacheDuration) / float64(cachedDuration)
	fmt.Printf("   Without cache: %v (%.1fx slower)\n", noCacheDuration, speedup)
}

func demonstrateResourceManagement(testData string) {
	fmt.Println("\n4. Resource Management")
	fmt.Println("------------------------")

	// Proper resource lifecycle management
	processor, _ := json.New(json.DefaultConfig()) // OK: DefaultConfig always valid

	// Use defer to ensure cleanup
	defer func() {
		processor.Close()
		fmt.Println("   Processor resources cleaned up")
	}()

	// Perform an operation (result not needed; exercising the processor).
	_, _ = processor.Get(testData, "users[0].name")

	// Check processor health
	if !processor.IsClosed() {
		fmt.Println("   Processor is healthy and active")
	}

	// Context-aware operations with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Using unified Config for operation
	cfg := json.DefaultConfig()
	cfg.CacheResults = true
	cfg.StrictMode = false

	result, err := processor.GetWithContext(ctx, testData, "config.features", cfg)
	if err != nil {
		fmt.Printf("   Operation failed: %v\n", err)
	} else {
		fmt.Printf("   Context-aware operation succeeded: %v\n", result)
	}

	// The package-level mirror runs the same context-aware read on the
	// shared global processor — no Processor instance needed.
	if _, err := json.GetWithContext(ctx, testData, "config.version"); err != nil {
		fmt.Printf("   Package-level GetWithContext failed: %v\n", err)
	} else {
		fmt.Println("   Package-level GetWithContext succeeded")
	}
}

func demonstrateMonitoring(testData string) {
	fmt.Println("\n5. Monitoring & Metrics")
	fmt.Println("-----------------------------")

	config := json.DefaultConfig()
	config.EnableCache = true
	config.MaxCacheSize = 1000
	config.CacheTTL = 10 * time.Minute
	config.EnableMetrics = true
	config.EnableHealthCheck = true
	processor, _ := json.New(config) // OK: DefaultConfig-derived
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

	// Exercise each path to populate metrics (results discarded).
	for _, op := range operations {
		_, _ = processor.Get(testData, op.path)
	}

	// Get comprehensive statistics
	stats := processor.GetStats()
	fmt.Printf("   Performance Metrics:\n")
	fmt.Printf("   - Total operations: %d\n", stats.OperationCount)
	fmt.Printf("   - Cache size: %d entries\n", stats.CacheSize)
	fmt.Printf("   - Cache hits: %d\n", stats.HitCount)
	fmt.Printf("   - Cache misses: %d\n", stats.MissCount)
	fmt.Printf("   - Cache hit rate: %.2f%%\n", stats.HitRatio*100)
	fmt.Printf("   - Error count: %d\n", stats.ErrorCount)

	// Health check
	if processor.IsClosed() {
		fmt.Printf("   Processor is closed\n")
	} else {
		fmt.Printf("   Processor is healthy and active\n")
	}
}

func demonstrateGlobalProcessor(testData string) {
	fmt.Println("\n6. Global Processor & Health")
	fmt.Println("------------------------------")

	// Inspect and adjust a live processor: GetConfig returns the effective
	// Config (after defaults), SetLogger wires a *slog.Logger for internal
	// diagnostics. io.Discard keeps the example output clean — point it at a
	// real handler in production. EnableMetrics feeds the health checks below.
	monitorCfg := json.DefaultConfig()
	monitorCfg.EnableMetrics = true
	processor, _ := json.New(monitorCfg) // OK: DefaultConfig-derived, always valid
	defer processor.Close()
	fmt.Printf("   GetConfig(): cache=%t, maxCacheSize=%d\n",
		processor.GetConfig().EnableCache, processor.GetConfig().MaxCacheSize)
	processor.SetLogger(slog.New(slog.NewTextHandler(io.Discard, nil)))

	// Package-level functions (Get, Set, ...) run on a shared global processor.
	// SetGlobalProcessor replaces it with your own — e.g. one whose Config is
	// tuned for your workload. The previous global is closed automatically.
	json.SetGlobalProcessor(processor)
	fmt.Println("   SetGlobalProcessor: package-level ops now use our processor")

	_, _ = json.Get(testData, "users[0].name") // best-effort: exercising the global

	// Package-level GetStats reports the global processor's counters.
	globalStats := json.GetStats()
	fmt.Printf("   Global GetStats(): operations=%d, errors=%d\n",
		globalStats.OperationCount, globalStats.ErrorCount)

	// Package-level GetHealthStatus mirrors that for the global's health.
	fmt.Printf("   Global GetHealthStatus(): healthy=%t\n",
		json.GetHealthStatus().Healthy)

	// Processor-level health check reports each subsystem individually.
	health := processor.GetHealthStatus()
	fmt.Printf("   HealthStatus: healthy=%t, checks=%d\n", health.Healthy, len(health.Checks))
	for name, check := range health.Checks {
		fmt.Printf("   - %-12s healthy=%t %s\n", name, check.Healthy, check.Message)
	}

	// At application shutdown, close the global processor cleanly. The next
	// package-level call would lazily create a fresh default one.
	json.ShutdownGlobalProcessor()
	fmt.Println("   ShutdownGlobalProcessor: global closed")
}
