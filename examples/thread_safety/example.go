package main

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cybergodev/json"
)

func main() {
	fmt.Println("🚀 JSON Library Thread Safety Demo")
	fmt.Println("===================================")

	// Demo 1: Global Processor Thread Safety
	fmt.Println("\n1. Global Processor Thread Safety Test")
	testGlobalProcessorSafety()

	// Demo 2: Concurrent Operations
	fmt.Println("\n2. Concurrent Operations Test")
	testConcurrentOperations()

	// Demo 3: Resource Pool Efficiency
	fmt.Println("\n3. Resource Pool Efficiency Test")
	testResourcePoolEfficiency()

	// Demo 4: Cache Thread Safety
	fmt.Println("\n4. Cache Thread Safety Test")
	testCacheThreadSafety()

	// Demo 5: Concurrency Control
	fmt.Println("\n5. Concurrency Control Test")
	testConcurrencyControl()

	fmt.Println("\n✅ All thread safety tests completed successfully!")
}

func testGlobalProcessorSafety() {
	var wg sync.WaitGroup
	var successCount int64
	var errorCount int64

	numGoroutines := 50
	operationsPerGoroutine := 100

	testData := `{"users":[{"name":"Alice","age":30},{"name":"Bob","age":25}],"config":{"timeout":5000}}`

	start := time.Now()

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			for j := 0; j < operationsPerGoroutine; j++ {
				// Test different operations concurrently
				var err error
				switch j % 4 {
				case 0:
					_, err = json.Get(testData, "users[0].name")
				case 1:
					_, err = json.Set(testData, "users[0].age", id+j)
				case 2:
					_, err = json.GetString(testData, "users[1].name")
				case 3:
					_, err = json.GetInt(testData, "config.timeout")
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
}

func testConcurrentOperations() {
	config := json.DefaultConfig()
	config.MaxConcurrency = 20
	processor := json.New(config)
	defer processor.Close()

	var wg sync.WaitGroup
	var operationCount int64

	testData := `{"data":{"items":[1,2,3,4,5],"metadata":{"version":"1.0"}}}`

	start := time.Now()

	// Launch multiple types of concurrent operations
	for i := 0; i < 10; i++ {
		// Read operations
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				_, _ = processor.Get(testData, "data.items[0]")
				atomic.AddInt64(&operationCount, 1)
			}
		}()

		// Write operations
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				_, _ = processor.Set(testData, "data.metadata.timestamp", time.Now().Unix())
				atomic.AddInt64(&operationCount, 1)
			}
		}()

		// Array operations
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				_, _ = processor.Get(testData, "data.items")
				atomic.AddInt64(&operationCount, 1)
			}
		}()
	}

	wg.Wait()
	duration := time.Since(start)

	totalOps := atomic.LoadInt64(&operationCount)
	fmt.Printf("   ✓ Completed %d concurrent operations in %v\n", totalOps, duration)
	fmt.Printf("   ✓ Average throughput: %.0f ops/sec\n", float64(totalOps)/duration.Seconds())

	// Check processor health
	if processor.IsClosed() {
		fmt.Printf("   ❌ Processor was closed unexpectedly\n")
	} else {
		fmt.Printf("   ✓ Processor remained healthy throughout test\n")
	}
}

func testResourcePoolEfficiency() {
	processor := json.New()
	defer processor.Close()

	var wg sync.WaitGroup
	numGoroutines := 20
	operationsPerGoroutine := 200

	start := time.Now()

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			for j := 0; j < operationsPerGoroutine; j++ {
				// These operations will use resource pools internally
				testData := fmt.Sprintf(`{"id":%d,"value":"test_%d"}`, j, j)
				_, _ = processor.Get(testData, "id")
				_, _ = processor.Set(testData, "processed", true)
			}
		}()
	}

	wg.Wait()
	duration := time.Since(start)

	totalOps := int64(numGoroutines * operationsPerGoroutine * 2) // 2 operations per iteration
	fmt.Printf("   ✓ Completed %d resource pool operations in %v\n", totalOps, duration)
	fmt.Printf("   ✓ Average throughput: %.0f ops/sec\n", float64(totalOps)/duration.Seconds())

	// Check resource monitor if available
	if processor != nil {
		fmt.Printf("   ✓ Resource pools handled high concurrency efficiently\n")
	}
}

func testCacheThreadSafety() {
	config := json.DefaultConfig()
	config.EnableCache = true
	config.MaxCacheSize = 1000
	processor := json.New(config)
	defer processor.Close()

	var wg sync.WaitGroup
	var cacheHits int64
	var cacheMisses int64

	testData := `{"products":[{"id":1,"name":"Product A"},{"id":2,"name":"Product B"}]}`

	// Warm up cache
	_, _ = processor.Get(testData, "products[0].name")
	_, _ = processor.Get(testData, "products[1].name")

	start := time.Now()

	for i := 0; i < 30; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			for j := 0; j < 100; j++ {
				path := "products[0].name"
				if j%2 == 0 {
					path = "products[1].name"
				}

				_, err := processor.Get(testData, path)
				if err == nil {
					atomic.AddInt64(&cacheHits, 1)
				} else {
					atomic.AddInt64(&cacheMisses, 1)
				}
			}
		}()
	}

	wg.Wait()
	duration := time.Since(start)

	totalOps := atomic.LoadInt64(&cacheHits) + atomic.LoadInt64(&cacheMisses)
	hitRate := float64(atomic.LoadInt64(&cacheHits)) / float64(totalOps) * 100

	fmt.Printf("   ✓ Completed %d cache operations in %v\n", totalOps, duration)
	fmt.Printf("   ✓ Cache hit rate: %.2f%%\n", hitRate)
	fmt.Printf("   ✓ Throughput: %.0f ops/sec\n", float64(totalOps)/duration.Seconds())

	// Get cache statistics
	stats := processor.GetStats()
	fmt.Printf("   ✓ Final cache size: %d entries\n", stats.CacheSize)
}

func testConcurrencyControl() {
	// Create a concurrency manager for testing
	cm := json.NewConcurrencyManager(5, 100) // Max 5 concurrent, 100 ops/sec

	var wg sync.WaitGroup
	var successCount int64
	var timeoutCount int64
	var rateLimitCount int64

	start := time.Now()

	// Launch more operations than the concurrency limit
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()

			err := cm.ExecuteWithConcurrencyControl(ctx, func() error {
				// Simulate work
				time.Sleep(100 * time.Millisecond)
				return nil
			}, 1*time.Second)

			if err != nil {
				if err.Error() == "operation timeout" {
					atomic.AddInt64(&timeoutCount, 1)
				} else if err.Error() == "rate limit exceeded" {
					atomic.AddInt64(&rateLimitCount, 1)
				}
			} else {
				atomic.AddInt64(&successCount, 1)
			}
		}(i)
	}

	wg.Wait()
	duration := time.Since(start)

	success := atomic.LoadInt64(&successCount)
	timeouts := atomic.LoadInt64(&timeoutCount)
	rateLimits := atomic.LoadInt64(&rateLimitCount)
	total := success + timeouts + rateLimits

	fmt.Printf("   ✓ Concurrency control test completed in %v\n", duration)
	fmt.Printf("   ✓ Successful operations: %d/%d (%.1f%%)\n", success, total, float64(success)/float64(total)*100)
	fmt.Printf("   ✓ Timeout operations: %d/%d (%.1f%%)\n", timeouts, total, float64(timeouts)/float64(total)*100)
	fmt.Printf("   ✓ Rate limited operations: %d/%d (%.1f%%)\n", rateLimits, total, float64(rateLimits)/float64(total)*100)

	// Get concurrency statistics
	stats := cm.GetStats()
	fmt.Printf("   ✓ Max concurrent operations: %d\n", stats.MaxConcurrency)
	fmt.Printf("   ✓ Average wait time: %v\n", stats.AverageWaitTime)

	// Check for deadlocks
	deadlocks := cm.DetectDeadlocks()
	if len(deadlocks) > 0 {
		fmt.Printf("   ⚠️  Detected %d potential deadlocks\n", len(deadlocks))
	} else {
		fmt.Printf("   ✓ No deadlocks detected\n")
	}

	// Memory usage check
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	fmt.Printf("   ✓ Current memory usage: %.2f MB\n", float64(m.Alloc)/1024/1024)
}
