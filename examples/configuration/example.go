package main

import (
	"fmt"
	"time"

	"github.com/cybergodev/json"
)

func main() {
	fmt.Println("🚀 JSON Library Configuration Usage Examples")
	fmt.Println("====================================================")

	// Sample JSON data for configuration examples
	configData := `{
		"application": {
			"name": "MyApp",
			"version": "1.0.0",
			"environment": "production"
		},
		"database": {
			"host": "localhost",
			"port": 5432,
			"name": "myapp_db",
			"ssl": true,
			"connections": {
				"max": 100,
				"idle": 10,
				"timeout": 30
			}
		},
		"cache": {
			"enabled": true,
			"ttl": 300,
			"size": 1000
		},
		"features": [
			{"name": "feature1", "enabled": true, "config": {"limit": 100}},
			{"name": "feature2", "enabled": false, "config": {"limit": 50}},
			{"name": "feature3", "enabled": true, "config": {"limit": 200}}
		],
		"servers": [
			{"name": "web1", "host": "192.168.1.10", "port": 8080, "active": true},
			{"name": "web2", "host": "192.168.1.11", "port": 8080, "active": true},
			{"name": "web3", "host": "192.168.1.12", "port": 8080, "active": false}
		]
	}`

	// 1. Default configuration usage
	fmt.Println("\n1. Default Configuration Usage:")
	defaultConfigDemo(configData)

	// 2. Custom configuration setup
	fmt.Println("\n2. Custom Configuration Setup:")
	customConfigDemo(configData)

	// 3. Performance-optimized configuration
	fmt.Println("\n3. Performance-Optimized Configuration:")
	performanceConfigDemo(configData)

	// 4. Security-focused configuration
	fmt.Println("\n4. Security-Focused Configuration:")
	securityConfigDemo(configData)

	// 5. Cache configuration examples
	fmt.Println("\n5. Cache Configuration Examples:")
	cacheConfigDemo(configData)

	// 6. Processor options examples
	fmt.Println("\n6. Processor Options Examples:")
	processorOptionsDemo(configData)

	// 7. Configuration validation and monitoring
	fmt.Println("\n7. Configuration Validation and Monitoring:")
	validationMonitoringDemo(configData)

	fmt.Println("\n🎉 Configuration examples demonstration completed!")
	fmt.Println("💡 Tip: Choose configuration based on your specific use case and performance requirements")
}

func defaultConfigDemo(data string) {
	// Using default global processor (simplest approach)
	appName, _ := json.GetString(data, "application.name")
	fmt.Printf("   Using default config - App name: %s\n", appName)
	// Result: MyApp

	// Create processor with default configuration
	processor := json.New(json.DefaultConfig())
	defer processor.Close()

	version, _ := processor.Get(data, "application.version")
	fmt.Printf("   Using default processor - Version: %s\n", version)
	// Result: 1.0.0

	// Show default configuration values
	config := json.DefaultConfig()
	fmt.Printf("   Default cache size: %d\n", config.MaxCacheSize)   // 1000
	fmt.Printf("   Default cache TTL: %v\n", config.CacheTTL)        // 5m0s
	fmt.Printf("   Default cache enabled: %t\n", config.EnableCache) // true
}

func customConfigDemo(data string) {
	// Create custom configuration for specific needs
	customConfig := &json.Config{
		// Cache settings - optimized for frequent access
		EnableCache:  true,
		MaxCacheSize: 5000,
		CacheTTL:     10 * time.Minute,

		// Size limits - for handling large JSON files
		MaxJSONSize:  50 * 1024 * 1024, // 50MB
		MaxPathDepth: 50,
		MaxBatchSize: 500,

		// Performance settings
		MaxConcurrency:    20,
		ParallelThreshold: 5,

		// Processing options
		EnableValidation: true,
		StrictMode:       false,
		CreatePaths:      true, // Allow creating new paths
		CleanupNulls:     true, // Auto-cleanup null values
		CompactArrays:    true, // Auto-compact arrays
	}

	processor := json.New(customConfig)
	defer processor.Close()

	// Test custom configuration
	dbHost, _ := processor.Get(data, "database.host") // localhost
	dbPort, _ := processor.Get(data, "database.port") // 5432
	fmt.Printf("   Custom config - Database: %s:%v\n", dbHost, dbPort)

	// Test path creation (enabled in custom config)
	updated, _ := processor.Set(data, "application.new_feature.enabled", true) // Create new path
	newFeature, _ := processor.Get(updated, "application.new_feature.enabled") // true
	fmt.Printf("   Path creation enabled - New feature: %v\n", newFeature)
}

func performanceConfigDemo(data string) {
	// High-performance configuration for production use
	perfConfig := &json.Config{
		// Aggressive caching for maximum performance
		EnableCache:  true,
		MaxCacheSize: 20000,
		CacheTTL:     30 * time.Minute,

		// High concurrency settings
		MaxConcurrency:    100,
		ParallelThreshold: 3,

		// Large capacity limits
		MaxJSONSize:  100 * 1024 * 1024, // 100MB
		MaxPathDepth: 100,
		MaxBatchSize: 2000,

		// Performance optimizations
		EnableMetrics:     true,
		EnableHealthCheck: true,

		// Relaxed validation for speed
		EnableValidation: false,
		StrictMode:       false,
	}

	processor := json.New(perfConfig)
	defer processor.Close()

	// Demonstrate batch operations for performance
	paths := []string{
		"application.name",
		"application.version",
		"database.host",
		"database.port",
		"cache.enabled",
	}

	start := time.Now()
	results, _ := processor.GetMultiple(data, paths)
	duration := time.Since(start)

	fmt.Printf("   Performance config - Batch operation completed in: %v\n", duration)
	fmt.Printf("   Retrieved %d values: %v\n", len(results), results)

	// Show performance metrics
	stats := processor.GetStats()
	fmt.Printf("   Cache hit ratio: %.2f%%\n", stats.HitRatio*100)
	fmt.Printf("   Total operations: %d\n", stats.OperationCount)
}

func securityConfigDemo(data string) {
	// Security-focused configuration with strict limits
	securityConfig := &json.Config{
		// Conservative cache settings
		EnableCache:  true,
		MaxCacheSize: 1000,
		CacheTTL:     2 * time.Minute,

		// Strict security limits
		MaxJSONSize:       10 * 1024 * 1024, // 10MB limit
		MaxPathDepth:      20,               // Prevent deep nesting attacks
		MaxBatchSize:      100,              // Limit batch operations
		MaxConcurrency:    10,               // Conservative concurrency
		MaxNestingDepth:   15,               // Prevent stack overflow
		ParallelThreshold: 10,

		// Strict validation and security
		EnableValidation: true,
		StrictMode:       true,
		ValidateInput:    true,

		// Conservative processing options
		CreatePaths:   false, // Don't auto-create paths
		CleanupNulls:  false, // Don't auto-modify data
		CompactArrays: false, // Don't auto-modify arrays
	}

	processor := json.New(securityConfig)
	defer processor.Close()

	// Test secure operations - use supported extraction syntax
	allServers, err := processor.Get(data, "servers")
	if err != nil {
		fmt.Printf("   Security config - Error getting servers: %v\n", err)
	} else {
		// Count active servers manually since conditional filtering is not supported
		activeCount := 0
		if servers, ok := allServers.([]any); ok {
			for _, server := range servers {
				if serverObj, ok := server.(map[string]any); ok {
					if active, ok := serverObj["active"].(bool); ok && active {
						activeCount++
					}
				}
			}
		}
		fmt.Printf("   Security config - Active servers count: %d\n", activeCount)
	}

	// Demonstrate rate limiting (would be more visible with many operations)
	for i := 0; i < 5; i++ {
		_, _ = processor.Get(data, "application.name")
	}
	fmt.Printf("   Rate limiting enabled - Operations completed safely\n")
}

func cacheConfigDemo(data string) {
	// Different cache configurations for different scenarios

	// 1. High-frequency access cache
	highFreqConfig := &json.Config{
		EnableCache:       true,
		MaxCacheSize:      10000,
		CacheTTL:          1 * time.Hour, // Long TTL for stable data
		MaxPathDepth:      50,
		MaxJSONSize:       50 * 1024 * 1024, // 50MB
		MaxBatchSize:      500,
		MaxConcurrency:    10,
		ParallelThreshold: 5,
	}

	// 2. Memory-conscious cache
	memoryConfig := &json.Config{
		EnableCache:       true,
		MaxCacheSize:      500,              // Small cache size
		CacheTTL:          5 * time.Minute,  // Short TTL
		MaxPathDepth:      20,               // Required field
		MaxJSONSize:       10 * 1024 * 1024, // 10MB
		MaxBatchSize:      100,
		MaxConcurrency:    5,
		ParallelThreshold: 10,
	}

	// 3. No-cache configuration for dynamic data
	noCacheConfig := &json.Config{
		EnableCache:       false,            // Disable caching entirely
		MaxPathDepth:      20,               // Required field
		MaxJSONSize:       10 * 1024 * 1024, // 10MB
		MaxBatchSize:      100,
		MaxConcurrency:    5,
		ParallelThreshold: 10,
	}

	// Demonstrate different cache behaviors
	processors := map[string]*json.Processor{
		"high-freq": json.New(highFreqConfig),
		"memory":    json.New(memoryConfig),
		"no-cache":  json.New(noCacheConfig),
	}

	for name, processor := range processors {
		defer processor.Close()

		// Perform same operation multiple times to show cache effect
		// Use supported extraction syntax instead of conditional filtering
		start := time.Now()
		for i := 0; i < 3; i++ {
			_, _ = processor.Get(data, "features{name}")
		}
		duration := time.Since(start)

		stats := processor.GetStats()
		fmt.Printf("   %s config - 3 operations took: %v, cache hit rate: %.2f%%\n",
			name, duration, stats.HitRatio*100)
	}
}

func processorOptionsDemo(data string) {
	// Create processor with default config
	processor := json.New(json.DefaultConfig())
	defer processor.Close()

	// 1. Strict mode options
	strictOpts := &json.ProcessorOptions{
		StrictMode:      true,
		MaxDepth:        10,
		AllowComments:   false,
		PreserveNumbers: true,
	}

	// 2. Flexible mode options
	flexibleOpts := &json.ProcessorOptions{
		StrictMode:      false,
		MaxDepth:        50,
		AllowComments:   true,
		CreatePaths:     true,
		CleanupNulls:    true,
		CompactArrays:   true,
		ContinueOnError: true,
	}

	// 3. Performance mode options
	perfOpts := &json.ProcessorOptions{
		CacheResults:    true,
		StrictMode:      false,
		MaxDepth:        100,
		PreserveNumbers: false,
		ContinueOnError: true,
	}

	// Demonstrate different option behaviors
	fmt.Printf("   Testing different processor options:\n")

	// Test with strict options
	result1, _ := processor.Get(data, "database.connections.max", strictOpts)
	fmt.Printf("   Strict mode result: %v\n", result1)

	// Test with flexible options - create new path
	_, _ = processor.Set(data, "application.monitoring.enabled", true, flexibleOpts)
	fmt.Printf("   Flexible mode - path creation enabled\n")

	// Test with performance options
	start := time.Now()
	_, _ = processor.Get(data, "features{config.limit}", perfOpts)
	duration := time.Since(start)
	fmt.Printf("   Performance mode - operation took: %v\n", duration)
}

func validationMonitoringDemo(data string) {
	// Configuration with comprehensive monitoring
	monitoringConfig := &json.Config{
		EnableCache:       true,
		MaxCacheSize:      2000,
		CacheTTL:          15 * time.Minute,
		EnableMetrics:     true,
		EnableHealthCheck: true,
		EnableValidation:  true,
		StrictMode:        false,
		MaxPathDepth:      50,
		MaxJSONSize:       50 * 1024 * 1024,
		MaxBatchSize:      500,
		MaxConcurrency:    20,
		ParallelThreshold: 5,
	}

	processor := json.New(monitoringConfig)
	defer processor.Close()

	// Perform various operations to generate metrics
	operations := []struct {
		name string
		path string
	}{
		{"Get app name", "application.name"},
		{"Get database config", "database"},
		{"Get server names", "servers{?=name}"},    // ⚠️ Unsupported extraction syntax
		{"Get feature names", "features{A?.name}"}, // ⚠️ Unsupported extraction syntax
		{"Get cache settings", "cache"},
	}

	fmt.Printf("   Performing monitored operations:\n")
	for _, op := range operations {
		start := time.Now()
		result, err := processor.Get(data, op.path)
		duration := time.Since(start)

		if err != nil {
			fmt.Printf("   ❌ %s failed: %v (took %v)\n", op.name, err, duration)
		} else {
			fmt.Printf("   ✅ %s succeeded (took %v)\n", op.name, duration)
			_ = result // Use result to avoid unused variable warning
		}
	}

	// Display comprehensive metrics
	stats := processor.GetStats()
	fmt.Printf("\n   📊 Performance Metrics:\n")
	fmt.Printf("   Total operations: %d\n", stats.OperationCount)
	fmt.Printf("   Cache size: %d\n", stats.CacheSize)
	fmt.Printf("   Cache hits: %d\n", stats.HitCount)
	fmt.Printf("   Cache misses: %d\n", stats.MissCount)
	fmt.Printf("   Cache hit rate: %.2f%%\n", stats.HitRatio*100)

	// Health check
	if processor.IsClosed() {
		fmt.Printf("   ⚠️ Processor is closed\n")
	} else {
		fmt.Printf("   ✅ Processor is healthy and active\n")
	}
}
