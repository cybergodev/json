package json

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
)

// TestConcurrency tests all concurrency and thread safety scenarios
// Consolidated from: concurrency_test.go, thread_safety_test.go, performance_concurrency_test.go
func TestConcurrency(t *testing.T) {
	helper := NewTestHelper(t)
	generator := NewTestDataGenerator()

	t.Run("ConcurrentReads", func(t *testing.T) {
		jsonStr := generator.GenerateComplexJSON()

		concurrencyTester := NewConcurrencyTester(t, 20, 100)

		concurrencyTester.Run(func(workerID, iteration int) error {
			// Different workers access different paths
			paths := []string{
				"users[0].name",
				"users[1].name",
				"settings.appName",
				"statistics.totalUsers",
				"users[0].profile.age",
				"users[0].profile.preferences.theme",
			}

			path := paths[workerID%len(paths)]
			_, err := Get(jsonStr, path)
			return err
		})
	})

	t.Run("ConcurrentWrites", func(t *testing.T) {
		originalJSON := `{"counters": {"a": 0, "b": 0, "c": 0, "d": 0, "e": 0}}`

		var results []string
		var resultsMutex sync.Mutex

		concurrencyTester := NewConcurrencyTester(t, 10, 50)

		concurrencyTester.Run(func(workerID, iteration int) error {
			// Each worker modifies a different counter
			counterKey := fmt.Sprintf("counters.%c", 'a'+workerID%5)

			result, err := Set(originalJSON, counterKey, workerID*1000+iteration)
			if err != nil {
				return err
			}

			resultsMutex.Lock()
			results = append(results, result)
			resultsMutex.Unlock()

			return nil
		})

		// Verify we got results from all operations
		helper.AssertTrue(len(results) > 0, "Should have results from concurrent writes")
	})

	t.Run("ConcurrentMixedOperations", func(t *testing.T) {
		baseJSON := `{
			"data": {"counter": 0, "name": "test"},
			"array": [1, 2, 3, 4, 5],
			"nested": {"level1": {"level2": "value"}}
		}`

		var operations int64
		var errors int64

		concurrencyTester := NewConcurrencyTester(t, 15, 100)

		concurrencyTester.Run(func(workerID, iteration int) error {
			atomic.AddInt64(&operations, 1)

			// Mix different operations
			switch iteration % 4 {
			case 0:
				_, err := Get(baseJSON, "data.name")
				if err != nil {
					atomic.AddInt64(&errors, 1)
				}
				return err
			case 1:
				_, err := Set(baseJSON, "data.counter", iteration)
				if err != nil {
					atomic.AddInt64(&errors, 1)
				}
				return err
			case 2:
				_, err := Get(baseJSON, "array[2]")
				if err != nil {
					atomic.AddInt64(&errors, 1)
				}
				return err
			case 3:
				_, err := Delete(baseJSON, "nested.level1.level2")
				if err != nil {
					atomic.AddInt64(&errors, 1)
				}
				return err
			}
			return nil
		})

		totalOps := atomic.LoadInt64(&operations)
		totalErrors := atomic.LoadInt64(&errors)

		t.Logf("Total operations: %d, Errors: %d, Success rate: %.2f%%",
			totalOps, totalErrors, float64(totalOps-totalErrors)/float64(totalOps)*100)

		helper.AssertTrue(totalOps > 0, "Should have performed operations")
	})

	t.Run("ConcurrentProcessorUsage", func(t *testing.T) {
		config := DefaultConfig()
		config.EnableCache = true
		config.MaxCacheSize = 1000

		processor := New(config)
		defer processor.Close()

		jsonStr := generator.GenerateComplexJSON()

		concurrencyTester := NewConcurrencyTester(t, 25, 200)

		concurrencyTester.Run(func(workerID, iteration int) error {
			// Mix of operations using the same processor
			switch iteration % 4 {
			case 0:
				_, err := processor.Get(jsonStr, "users[0].name")
				return err
			case 1:
				_, err := GetInt(jsonStr, "statistics.totalUsers")
				return err
			case 2:
				_, err := GetArray(jsonStr, "users")
				return err
			case 3:
				_, err := Set(jsonStr, fmt.Sprintf("worker_%d", workerID), iteration)
				return err
			}
			return nil
		})
	})
}

// TestProcessorConcurrency tests processor-specific concurrency
func TestProcessorConcurrency(t *testing.T) {
	helper := NewTestHelper(t)

	t.Run("MultipleProcessorsConcurrent", func(t *testing.T) {
		const numProcessors = 10
		const operationsPerProcessor = 100

		jsonStr := `{"test": "value", "number": 42, "array": [1, 2, 3]}`

		var wg sync.WaitGroup
		var totalOperations int64
		var totalErrors int64

		for i := 0; i < numProcessors; i++ {
			wg.Add(1)
			go func(processorID int) {
				defer wg.Done()

				processor := New()
				defer processor.Close()

				for j := 0; j < operationsPerProcessor; j++ {
					atomic.AddInt64(&totalOperations, 1)

					_, err := processor.Get(jsonStr, "test")
					if err != nil {
						atomic.AddInt64(&totalErrors, 1)
						t.Errorf("Processor %d operation %d failed: %v", processorID, j, err)
					}
				}
			}(i)
		}

		wg.Wait()

		operations := atomic.LoadInt64(&totalOperations)
		errors := atomic.LoadInt64(&totalErrors)

		helper.AssertEqual(int64(numProcessors*operationsPerProcessor), operations, "Should complete all operations")
		helper.AssertEqual(int64(0), errors, "Should have no errors")
	})

	t.Run("SharedProcessorConcurrency", func(t *testing.T) {
		processor := New()
		defer processor.Close()

		jsonStr := `{"shared": "data", "counter": 0}`

		const numWorkers = 20
		const operationsPerWorker = 50

		var wg sync.WaitGroup
		var successCount int64

		for i := 0; i < numWorkers; i++ {
			wg.Add(1)
			go func(workerID int) {
				defer wg.Done()

				for j := 0; j < operationsPerWorker; j++ {
					// Perform read operation
					_, err := processor.Get(jsonStr, "shared")
					if err == nil {
						atomic.AddInt64(&successCount, 1)
					}
				}
			}(i)
		}

		wg.Wait()

		expectedOperations := int64(numWorkers * operationsPerWorker)
		actualSuccess := atomic.LoadInt64(&successCount)

		helper.AssertEqual(expectedOperations, actualSuccess, "All operations should succeed")
	})
}

// TestCacheThreadSafety tests cache thread safety
func TestCacheThreadSafety(t *testing.T) {
	helper := NewTestHelper(t)

	t.Run("ConcurrentCacheAccess", func(t *testing.T) {
		config := DefaultConfig()
		config.EnableCache = true
		config.MaxCacheSize = 100

		processor := New(config)
		defer processor.Close()

		jsonStr := `{"cached": "value", "number": 123}`

		const numWorkers = 15
		const operationsPerWorker = 100

		var wg sync.WaitGroup
		var cacheHits int64
		var cacheMisses int64

		// Pre-warm cache
		processor.Get(jsonStr, "cached")

		for i := 0; i < numWorkers; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()

				for j := 0; j < operationsPerWorker; j++ {
					// Alternate between cached and non-cached paths
					if j%2 == 0 {
						_, err := processor.Get(jsonStr, "cached")
						if err == nil {
							atomic.AddInt64(&cacheHits, 1)
						}
					} else {
						_, err := processor.Get(jsonStr, fmt.Sprintf("dynamic_%d", j))
						if err != nil {
							// Expected for non-existent paths
							atomic.AddInt64(&cacheMisses, 1)
						}
					}
				}
			}()
		}

		wg.Wait()

		// Check cache statistics
		stats := processor.GetStats()
		cacheHits = stats.HitCount
		cacheMisses = stats.MissCount

		t.Logf("Cache performance - Hits: %d, Misses: %d, Hit Ratio: %.2f%%",
			cacheHits, cacheMisses, stats.HitRatio*100)

		helper.AssertEqual(true, cacheHits+cacheMisses > 0, "Should have cache activity")
	})
}

// TestIteratorConcurrency tests concurrent iteration
func TestIteratorConcurrency(t *testing.T) {
	helper := NewTestHelper(t)

	t.Run("ConcurrentForeach", func(t *testing.T) {
		jsonStr := `{
			"data": [1, 2, 3, 4, 5, 6, 7, 8, 9, 10],
			"metadata": {"count": 10, "processed": false}
		}`

		const numWorkers = 10
		var wg sync.WaitGroup
		var processedCount int64

		for i := 0; i < numWorkers; i++ {
			wg.Add(1)
			go func(workerID int) {
				defer wg.Done()

				// Simplified foreach test - just count array elements
				dataArray, err := GetArray(jsonStr, "data")
				if err == nil {
					for range dataArray {
						atomic.AddInt64(&processedCount, 1)
					}
				}

				if err != nil {
					t.Errorf("Worker %d foreach failed: %v", workerID, err)
				}
			}(i)
		}

		wg.Wait()

		// Each worker processes 10 items
		expectedTotal := int64(numWorkers * 10)
		actualProcessed := atomic.LoadInt64(&processedCount)

		helper.AssertEqual(expectedTotal, actualProcessed, "Should process all items from all workers")
	})
}

// TestRaceConditions tests for race conditions
func TestRaceConditions(t *testing.T) {
	helper := NewTestHelper(t)

	t.Run("NoRaceInBasicOperations", func(t *testing.T) {
		jsonStr := `{"test": "value", "counter": 0}`

		const numGoroutines = 100
		const operationsPerGoroutine = 50

		var wg sync.WaitGroup
		results := make(chan string, numGoroutines*operationsPerGoroutine)

		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()

				for j := 0; j < operationsPerGoroutine; j++ {
					// Perform operation
					result, err := Set(jsonStr, "counter", id*1000+j)
					if err != nil {
						t.Errorf("Goroutine %d operation %d failed: %v", id, j, err)
						continue
					}
					results <- result
				}
			}(i)
		}

		wg.Wait()
		close(results)

		// Count results
		resultCount := 0
		for range results {
			resultCount++
		}

		expectedResults := numGoroutines * operationsPerGoroutine
		helper.AssertEqual(expectedResults, resultCount, "Should get result from each operation")
	})

	t.Run("ProcessorStateConsistency", func(t *testing.T) {
		processor := New()
		defer processor.Close()

		jsonStr := `{"state": "initial"}`

		const numWorkers = 20
		var wg sync.WaitGroup
		var consistentReads int64

		for i := 0; i < numWorkers; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()

				for j := 0; j < 100; j++ {
					value, err := processor.Get(jsonStr, "state")
					if err == nil && value == "initial" {
						atomic.AddInt64(&consistentReads, 1)
					}
				}
			}()
		}

		wg.Wait()

		expectedReads := int64(numWorkers * 100)
		actualConsistentReads := atomic.LoadInt64(&consistentReads)

		helper.AssertEqual(expectedReads, actualConsistentReads, "All reads should be consistent")
	})
}
