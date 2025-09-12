package json

import (
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestAdvancedConcurrencyPerformance tests comprehensive concurrency safety and performance
func TestAdvancedConcurrencyPerformance(t *testing.T) {
	helper := NewTestHelper(t)

	t.Run("ConcurrentProcessorOperations", func(t *testing.T) {
		processor := New()
		defer processor.Close()

		testData := `{
			"users": [
				{"id": 1, "name": "Alice", "score": 95, "active": true},
				{"id": 2, "name": "Bob", "score": 87, "active": false},
				{"id": 3, "name": "Charlie", "score": 92, "active": true},
				{"id": 4, "name": "Diana", "score": 88, "active": true},
				{"id": 5, "name": "Eve", "score": 94, "active": false}
			],
			"metadata": {
				"total": 5,
				"updated": "2024-01-01T00:00:00Z"
			}
		}`

		const numGoroutines = 50
		const operationsPerGoroutine = 100
		var wg sync.WaitGroup
		var errors int64
		var successCount int64

		// Test concurrent Get operations
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(goroutineID int) {
				defer wg.Done()
				for j := 0; j < operationsPerGoroutine; j++ {
					// Vary the operations to test different code paths
					switch j % 4 {
					case 0:
						_, err := processor.Get(testData, "users[0].name")
						if err != nil {
							atomic.AddInt64(&errors, 1)
						} else {
							atomic.AddInt64(&successCount, 1)
						}
					case 1:
						_, err := processor.Get(testData, "users[1:3]")
						if err != nil {
							atomic.AddInt64(&errors, 1)
						} else {
							atomic.AddInt64(&successCount, 1)
						}
					case 2:
						_, err := processor.Get(testData, "metadata.total")
						if err != nil {
							atomic.AddInt64(&errors, 1)
						} else {
							atomic.AddInt64(&successCount, 1)
						}
					case 3:
						_, err := processor.Get(testData, "users[-1].score")
						if err != nil {
							atomic.AddInt64(&errors, 1)
						} else {
							atomic.AddInt64(&successCount, 1)
						}
					}
				}
			}(i)
		}

		wg.Wait()

		totalOperations := int64(numGoroutines * operationsPerGoroutine)
		helper.AssertEqual(int64(0), errors, "Should have no errors in concurrent operations")
		helper.AssertEqual(totalOperations, successCount, "All operations should succeed")
	})

	t.Run("ConcurrentSetOperations", func(t *testing.T) {
		processor := New()
		defer processor.Close()

		baseData := `{
			"counters": {
				"a": 0,
				"b": 0,
				"c": 0
			},
			"arrays": {
				"items": []
			}
		}`

		const numGoroutines = 20
		const operationsPerGoroutine = 50
		var wg sync.WaitGroup
		var errors int64
		var mu sync.Mutex
		results := make([]string, 0, numGoroutines*operationsPerGoroutine)

		// Test concurrent Set operations
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(goroutineID int) {
				defer wg.Done()
				for j := 0; j < operationsPerGoroutine; j++ {
					// Each goroutine works on different paths to avoid conflicts
					path := fmt.Sprintf("counters.%c", 'a'+goroutineID%3)
					value := goroutineID*operationsPerGoroutine + j

					result, err := processor.Set(baseData, path, value)
					if err != nil {
						atomic.AddInt64(&errors, 1)
					} else {
						mu.Lock()
						results = append(results, result)
						mu.Unlock()
					}
				}
			}(i)
		}

		wg.Wait()

		helper.AssertEqual(int64(0), errors, "Should have no errors in concurrent Set operations")
		helper.AssertEqual(numGoroutines*operationsPerGoroutine, len(results), "Should have all results")
	})

	t.Run("ConcurrentDeleteOperations", func(t *testing.T) {
		processor := New()
		defer processor.Close()

		// Create test data with many items to delete
		testData := `{
			"items": [
				{"id": 1, "temp": true},
				{"id": 2, "temp": false},
				{"id": 3, "temp": true},
				{"id": 4, "temp": false},
				{"id": 5, "temp": true},
				{"id": 6, "temp": false},
				{"id": 7, "temp": true},
				{"id": 8, "temp": false},
				{"id": 9, "temp": true},
				{"id": 10, "temp": false}
			]
		}`

		const numGoroutines = 10
		var wg sync.WaitGroup
		var errors int64
		var mu sync.Mutex
		results := make([]string, 0, numGoroutines)

		// Test concurrent Delete operations on different paths
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(goroutineID int) {
				defer wg.Done()
				
				// Each goroutine deletes a different item
				path := fmt.Sprintf("items[%d].temp", goroutineID)
				result, err := processor.Delete(testData, path)
				if err != nil {
					atomic.AddInt64(&errors, 1)
				} else {
					mu.Lock()
					results = append(results, result)
					mu.Unlock()
				}
			}(i)
		}

		wg.Wait()

		helper.AssertEqual(int64(0), errors, "Should have no errors in concurrent Delete operations")
		helper.AssertEqual(numGoroutines, len(results), "Should have all delete results")
	})

	t.Run("ConcurrentForeachOperations", func(t *testing.T) {
		processor := New()
		defer processor.Close()

		testData := `{
			"data": [
				{"value": 1, "category": "A"},
				{"value": 2, "category": "B"},
				{"value": 3, "category": "A"},
				{"value": 4, "category": "C"},
				{"value": 5, "category": "B"}
			]
		}`

		const numGoroutines = 15
		var wg sync.WaitGroup
		var errors int64
		var totalProcessed int64

		// Test concurrent Foreach operations
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(goroutineID int) {
				defer wg.Done()
				
				var localCount int64
				err := processor.ForeachWithPath(testData, "data", func(key any, value *IterableValue) {
					atomic.AddInt64(&localCount, 1)
					// Simulate some processing
					if val := value.GetInt("value"); val > 0 {
						// Processing successful
					}
				})
				
				if err != nil {
					atomic.AddInt64(&errors, 1)
				} else {
					atomic.AddInt64(&totalProcessed, localCount)
				}
			}(i)
		}

		wg.Wait()

		helper.AssertEqual(int64(0), errors, "Should have no errors in concurrent Foreach operations")
		expectedTotal := int64(numGoroutines * 5) // 5 items per foreach
		helper.AssertEqual(expectedTotal, totalProcessed, "Should process all items across all goroutines")
	})

	t.Run("ProcessorResourceManagement", func(t *testing.T) {
		const numProcessors = 20
		const operationsPerProcessor = 50

		var wg sync.WaitGroup
		var errors int64
		var successCount int64

		testData := `{"test": {"value": 42, "array": [1, 2, 3, 4, 5]}}`

		// Test creating and using multiple processors concurrently
		for i := 0; i < numProcessors; i++ {
			wg.Add(1)
			go func(processorID int) {
				defer wg.Done()
				
				processor := New()
				defer processor.Close()

				for j := 0; j < operationsPerProcessor; j++ {
					switch j % 3 {
					case 0:
						_, err := processor.Get(testData, "test.value")
						if err != nil {
							atomic.AddInt64(&errors, 1)
						} else {
							atomic.AddInt64(&successCount, 1)
						}
					case 1:
						_, err := processor.Set(testData, "test.temp", processorID)
						if err != nil {
							atomic.AddInt64(&errors, 1)
						} else {
							atomic.AddInt64(&successCount, 1)
						}
					case 2:
						_, err := processor.Get(testData, "test.array[2]")
						if err != nil {
							atomic.AddInt64(&errors, 1)
						} else {
							atomic.AddInt64(&successCount, 1)
						}
					}
				}
			}(i)
		}

		wg.Wait()

		totalOperations := int64(numProcessors * operationsPerProcessor)
		helper.AssertEqual(int64(0), errors, "Should have no errors with multiple processors")
		helper.AssertEqual(totalOperations, successCount, "All operations should succeed")
	})

	t.Run("MemoryLeakDetection", func(t *testing.T) {
		// Force garbage collection before test
		runtime.GC()
		runtime.GC()
		
		var m1, m2 runtime.MemStats
		runtime.ReadMemStats(&m1)

		// Perform many operations that could potentially leak memory
		const iterations = 1000
		testData := `{
			"large_array": [` + func() string {
				result := ""
				for i := 0; i < 100; i++ {
					if i > 0 {
						result += ","
					}
					result += fmt.Sprintf(`{"id": %d, "data": "item_%d"}`, i, i)
				}
				return result
			}() + `]
		}`

		processor := New()
		defer processor.Close()

		for i := 0; i < iterations; i++ {
			// Perform various operations
			_, _ = processor.Get(testData, "large_array[50].data")
			_, _ = processor.Set(testData, "temp", i)
			_, _ = processor.Delete(testData, "temp")
			
			// Clear cache periodically
			if i%100 == 0 {
				processor.ClearCache()
			}
		}

		// Force garbage collection after test
		runtime.GC()
		runtime.GC()
		runtime.ReadMemStats(&m2)

		// Check memory growth (should be reasonable)
		memoryGrowth := m2.Alloc - m1.Alloc
		helper.AssertTrue(memoryGrowth < 50*1024*1024, fmt.Sprintf("Memory growth should be reasonable, got %d bytes", memoryGrowth))
	})

	t.Run("HighLoadStressTest", func(t *testing.T) {
		processor := New()
		defer processor.Close()

		// Create complex test data
		complexData := `{
			"users": [` + func() string {
				result := ""
				for i := 0; i < 200; i++ {
					if i > 0 {
						result += ","
					}
					result += fmt.Sprintf(`{
						"id": %d,
						"name": "User%d",
						"profile": {
							"age": %d,
							"scores": [%d, %d, %d],
							"active": %t
						}
					}`, i, i, 20+i%50, i%100, (i+1)%100, (i+2)%100, i%2 == 0)
				}
				return result
			}() + `]
		}`

		const numGoroutines = 100
		const operationsPerGoroutine = 200
		var wg sync.WaitGroup
		var errors int64
		var successCount int64

		startTime := time.Now()

		// High load test with many concurrent operations
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(goroutineID int) {
				defer wg.Done()
				
				for j := 0; j < operationsPerGoroutine; j++ {
					userIndex := (goroutineID + j) % 200
					
					switch j % 5 {
					case 0:
						_, err := processor.Get(complexData, fmt.Sprintf("users[%d].name", userIndex))
						if err != nil {
							atomic.AddInt64(&errors, 1)
						} else {
							atomic.AddInt64(&successCount, 1)
						}
					case 1:
						_, err := processor.Get(complexData, fmt.Sprintf("users[%d].profile.scores[1]", userIndex))
						if err != nil {
							atomic.AddInt64(&errors, 1)
						} else {
							atomic.AddInt64(&successCount, 1)
						}
					case 2:
						_, err := processor.Get(complexData, fmt.Sprintf("users[%d:].profile.active", userIndex))
						if err != nil {
							atomic.AddInt64(&errors, 1)
						} else {
							atomic.AddInt64(&successCount, 1)
						}
					case 3:
						_, err := processor.Set(complexData, fmt.Sprintf("users[%d].temp", userIndex), goroutineID)
						if err != nil {
							atomic.AddInt64(&errors, 1)
						} else {
							atomic.AddInt64(&successCount, 1)
						}
					case 4:
						_, err := processor.Get(complexData, "users[-1].profile.age")
						if err != nil {
							atomic.AddInt64(&errors, 1)
						} else {
							atomic.AddInt64(&successCount, 1)
						}
					}
				}
			}(i)
		}

		wg.Wait()
		duration := time.Since(startTime)

		totalOperations := int64(numGoroutines * operationsPerGoroutine)
		helper.AssertEqual(int64(0), errors, "Should have no errors in high load test")
		helper.AssertEqual(totalOperations, successCount, "All operations should succeed")

		// Performance check - should complete within reasonable time
		helper.AssertTrue(duration < 30*time.Second, fmt.Sprintf("High load test should complete within 30 seconds, took %v", duration))
		
		// Calculate operations per second
		opsPerSecond := float64(totalOperations) / duration.Seconds()
		helper.AssertTrue(opsPerSecond > 1000, fmt.Sprintf("Should achieve at least 1000 ops/sec, got %.2f", opsPerSecond))
		
		t.Logf("High load test completed: %d operations in %v (%.2f ops/sec)", totalOperations, duration, opsPerSecond)
	})
}
