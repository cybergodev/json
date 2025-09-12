package main

import (
	"fmt"
	"runtime"
	"time"

	"github.com/cybergodev/json"
)

func main() {
	runPerformanceTest()
}

// PerformanceTest runs comprehensive performance tests
func runPerformanceTest() {
	fmt.Println("\n🚀 JSON Library Performance Test")
	fmt.Println("================================")

	// Test data
	testData := `{
		"users": [
			{"id": 1, "name": "Alice", "email": "alice@example.com", "active": true},
			{"id": 2, "name": "Bob", "email": "bob@example.com", "active": false},
			{"id": 3, "name": "Charlie", "email": "charlie@example.com", "active": true}
		],
		"metadata": {
			"total": 3,
			"page": 1,
			"limit": 10
		}
	}`

	// Run different performance tests
	testBasicOperations(testData)
	testPathExpressions(testData)
	testProcessorPerformance(testData)
	testConcurrentPerformance(testData)
	testMemoryUsage(testData)
}

func testBasicOperations(testData string) {
	fmt.Println("\n📊 Basic Operations Performance")

	iterations := 10000

	// Test Get operations
	start := time.Now()
	for i := 0; i < iterations; i++ {
		_, _ = json.Get(testData, "users[0].name")
	}
	duration := time.Since(start)
	fmt.Printf("  Get operations: %d ops in %v (%.0f ops/sec)\n",
		iterations, duration, float64(iterations)/duration.Seconds())

	// Test Set operations
	start = time.Now()
	for i := 0; i < iterations; i++ {
		_, _ = json.Set(testData, "metadata.page", i)
	}
	duration = time.Since(start)
	fmt.Printf("  Set operations: %d ops in %v (%.0f ops/sec)\n",
		iterations, duration, float64(iterations)/duration.Seconds())
}

func testPathExpressions(testData string) {
	fmt.Println("\n🛤️  Path Expression Performance")

	iterations := 5000

	// Test array access
	start := time.Now()
	for i := 0; i < iterations; i++ {
		_, _ = json.Get(testData, "users[0].name")
		_, _ = json.Get(testData, "users[-1].email")
	}
	duration := time.Since(start)
	fmt.Printf("  Array access: %d ops in %v (%.0f ops/sec)\n",
		iterations*2, duration, float64(iterations*2)/duration.Seconds())

	// Test extraction
	start = time.Now()
	for i := 0; i < iterations; i++ {
		_, _ = json.Get(testData, "users{name}")
	}
	duration = time.Since(start)
	fmt.Printf("  Extraction: %d ops in %v (%.0f ops/sec)\n",
		iterations, duration, float64(iterations)/duration.Seconds())
}

func testProcessorPerformance(testData string) {
	fmt.Println("\n⚙️  Processor Performance")

	processor := json.New()
	defer processor.Close()

	iterations := 10000

	// Test processor operations
	start := time.Now()
	for i := 0; i < iterations; i++ {
		_, _ = processor.Get(testData, "users[0].name")
	}
	duration := time.Since(start)
	fmt.Printf("  Processor Get: %d ops in %v (%.0f ops/sec)\n",
		iterations, duration, float64(iterations)/duration.Seconds())

	// Check cache effectiveness
	stats := processor.GetStats()
	if stats.HitCount+stats.MissCount > 0 {
		hitRatio := float64(stats.HitCount) / float64(stats.HitCount+stats.MissCount) * 100
		fmt.Printf("  Cache hit ratio: %.1f%% (hits: %d, misses: %d)\n",
			hitRatio, stats.HitCount, stats.MissCount)
	}
}

func testConcurrentPerformance(testData string) {
	fmt.Println("\n🔄 Concurrent Performance")

	const numGoroutines = 10
	const opsPerGoroutine = 1000

	start := time.Now()

	done := make(chan bool, numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func() {
			for j := 0; j < opsPerGoroutine; j++ {
				_, _ = json.Get(testData, "users[0].name")
			}
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < numGoroutines; i++ {
		<-done
	}

	duration := time.Since(start)
	totalOps := numGoroutines * opsPerGoroutine
	fmt.Printf("  Concurrent ops: %d ops in %v (%.0f ops/sec)\n",
		totalOps, duration, float64(totalOps)/duration.Seconds())
}

func testMemoryUsage(testData string) {
	fmt.Println("\n💾 Memory Usage")

	// Force garbage collection
	runtime.GC()

	var m1 runtime.MemStats
	runtime.ReadMemStats(&m1)

	// Perform operations
	iterations := 1000
	for i := 0; i < iterations; i++ {
		_, _ = json.Get(testData, "users[0].name")
		_, _ = json.Set(testData, "metadata.page", i)
	}

	// Force garbage collection again
	runtime.GC()

	var m2 runtime.MemStats
	runtime.ReadMemStats(&m2)

	fmt.Printf("  Memory allocated: %d KB\n", (m2.TotalAlloc-m1.TotalAlloc)/1024)
	fmt.Printf("  Memory per operation: %d bytes\n", (m2.TotalAlloc-m1.TotalAlloc)/(uint64(iterations)*2))
}
