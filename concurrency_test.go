package json

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
)

// TestConcurrency consolidates all concurrency and thread safety tests
// Replaces: concurrency_test.go (keeping existing comprehensive tests)
func TestConcurrency(t *testing.T) {
	helper := NewTestHelper(t)
	generator := NewTestDataGenerator()

	t.Run("ConcurrentReads", func(t *testing.T) {
		jsonStr := generator.GenerateComplexJSON()
		concurrencyTester := NewConcurrencyTester(t, 20, 100)

		concurrencyTester.Run(func(workerID, iteration int) error {
			paths := []string{
				"users[0].name",
				"users[1].name",
				"settings.appName",
				"statistics.totalUsers",
			}
			path := paths[workerID%len(paths)]
			_, err := Get(jsonStr, path)
			return err
		})
	})

	t.Run("ConcurrentWrites", func(t *testing.T) {
		originalJSON := `{"counters": {"a": 0, "b": 0, "c": 0}}`
		var results []string
		var resultsMutex sync.Mutex

		concurrencyTester := NewConcurrencyTester(t, 10, 50)
		concurrencyTester.Run(func(workerID, iteration int) error {
			counterKey := fmt.Sprintf("counters.%c", 'a'+workerID%3)
			result, err := Set(originalJSON, counterKey, workerID*1000+iteration)
			if err != nil {
				return err
			}
			resultsMutex.Lock()
			results = append(results, result)
			resultsMutex.Unlock()
			return nil
		})

		helper.AssertTrue(len(results) > 0)
	})

	t.Run("ConcurrentMixedOperations", func(t *testing.T) {
		baseJSON := `{
			"data": {"counter": 0, "name": "test"},
			"array": [1, 2, 3, 4, 5]
		}`

		var operations int64
		concurrencyTester := NewConcurrencyTester(t, 15, 100)

		concurrencyTester.Run(func(workerID, iteration int) error {
			atomic.AddInt64(&operations, 1)
			switch iteration % 3 {
			case 0:
				_, err := Get(baseJSON, "data.name")
				return err
			case 1:
				_, err := Set(baseJSON, "data.counter", iteration)
				return err
			case 2:
				_, err := Get(baseJSON, "array[2]")
				return err
			}
			return nil
		})

		helper.AssertTrue(atomic.LoadInt64(&operations) > 0)
	})

	t.Run("SharedProcessor", func(t *testing.T) {
		processor := New()
		defer processor.Close()

		jsonStr := generator.GenerateComplexJSON()
		concurrencyTester := NewConcurrencyTester(t, 25, 200)

		concurrencyTester.Run(func(workerID, iteration int) error {
			switch iteration % 3 {
			case 0:
				_, err := processor.Get(jsonStr, "users[0].name")
				return err
			case 1:
				_, err := GetInt(jsonStr, "statistics.totalUsers")
				return err
			case 2:
				_, err := Set(jsonStr, fmt.Sprintf("worker_%d", workerID), iteration)
				return err
			}
			return nil
		})
	})

	t.Run("MultipleProcessors", func(t *testing.T) {
		const numProcessors = 10
		const operationsPerProcessor = 100

		jsonStr := `{"test": "value", "number": 42}`
		var wg sync.WaitGroup
		var totalOps, totalErrors int64

		for i := 0; i < numProcessors; i++ {
			wg.Add(1)
			go func(processorID int) {
				defer wg.Done()
				processor := New()
				defer processor.Close()

				for j := 0; j < operationsPerProcessor; j++ {
					atomic.AddInt64(&totalOps, 1)
					_, err := processor.Get(jsonStr, "test")
					if err != nil {
						atomic.AddInt64(&totalErrors, 1)
					}
				}
			}(i)
		}

		wg.Wait()
		helper.AssertEqual(int64(numProcessors*operationsPerProcessor), atomic.LoadInt64(&totalOps))
		helper.AssertEqual(int64(0), atomic.LoadInt64(&totalErrors))
	})

	t.Run("CacheThreadSafety", func(t *testing.T) {
		config := DefaultConfig()
		config.EnableCache = true
		config.MaxCacheSize = 100

		processor := New(config)
		defer processor.Close()

		jsonStr := `{"cached": "value", "number": 123}`
		const numWorkers = 15
		const operationsPerWorker = 100

		var wg sync.WaitGroup
		for i := 0; i < numWorkers; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for j := 0; j < operationsPerWorker; j++ {
					if j%2 == 0 {
						processor.Get(jsonStr, "cached")
					} else {
						processor.Get(jsonStr, fmt.Sprintf("dynamic_%d", j))
					}
				}
			}()
		}

		wg.Wait()
		stats := processor.GetStats()
		helper.AssertTrue(stats.HitCount+stats.MissCount > 0)
	})

	t.Run("RaceConditions", func(t *testing.T) {
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
					result, err := Set(jsonStr, "counter", id*1000+j)
					if err == nil {
						results <- result
					}
				}
			}(i)
		}

		wg.Wait()
		close(results)

		resultCount := 0
		for range results {
			resultCount++
		}

		helper.AssertEqual(numGoroutines*operationsPerGoroutine, resultCount)
	})

	t.Run("StateConsistency", func(t *testing.T) {
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
		helper.AssertEqual(int64(numWorkers*100), atomic.LoadInt64(&consistentReads))
	})
}
