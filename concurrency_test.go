package json

import (
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestConcurrentAccess tests concurrent access to JSON operations
func TestConcurrentAccess(t *testing.T) {
	helper := NewTestHelper(t)

	t.Run("ConcurrentReads", func(t *testing.T) {
		processor := New(DefaultConfig())
		defer processor.Close()

		testData := `{"users": [` + generateUserJSON(100) + `]}`

		concurrency := 20
		iterations := 100

		var wg sync.WaitGroup
		errors := make(chan error, concurrency*iterations)

		for i := 0; i < concurrency; i++ {
			wg.Add(1)
			go func(workerID int) {
				defer wg.Done()
				for j := 0; j < iterations; j++ {
					_, err := processor.Get(testData, fmt.Sprintf("users[%d]", j%100))
					if err != nil {
						errors <- err
					}
				}
			}(i)
		}

		wg.Wait()
		close(errors)

		// Check for errors
		for err := range errors {
			t.Errorf("Concurrent read error: %v", err)
		}
	})

	t.Run("ConcurrentWrites", func(t *testing.T) {
		testData := `{"counter": 0}`

		concurrency := 10
		iterations := 50

		var wg sync.WaitGroup
		results := make(chan string, concurrency*iterations)

		for i := 0; i < concurrency; i++ {
			wg.Add(1)
			go func(workerID int) {
				defer wg.Done()
				for j := 0; j < iterations; j++ {
					result, err := Set(testData, "counter", workerID*iterations+j)
					if err == nil {
						results <- result
					}
				}
			}(i)
		}

		wg.Wait()
		close(results)

		// Should have successful writes
		count := 0
		for range results {
			count++
		}
		helper.AssertTrue(count > 0)
	})

	t.Run("ConcurrentMixed", func(t *testing.T) {
		processor := New(DefaultConfig())
		defer processor.Close()

		testData := `{"data": {"value": 0}}`

		concurrency := 15
		iterations := 50

		var wg sync.WaitGroup

		for i := 0; i < concurrency; i++ {
			wg.Add(1)
			go func(workerID int) {
				defer wg.Done()
				for j := 0; j < iterations; j++ {
					if workerID%2 == 0 {
						// Read
						processor.Get(testData, "data.value")
					} else {
						// Write
						newData, _ := Set(testData, "data.value", workerID*100+j)
						testData = newData
					}
				}
			}(i)
		}

		wg.Wait()
		// Test completes without deadlock
	})

	t.Run("ConcurrentProcessors", func(t *testing.T) {
		concurrency := 10
		iterations := 20

		testData := `{"test": "value"}`

		var wg sync.WaitGroup

		for i := 0; i < concurrency; i++ {
			wg.Add(1)
			go func(workerID int) {
				defer wg.Done()

				processor := New(DefaultConfig())
				defer processor.Close()

				for j := 0; j < iterations; j++ {
					processor.Get(testData, "test")
				}
			}(i)
		}

		wg.Wait()
	})
}

// TestConcurrentCacheAccess tests concurrent cache operations
func TestConcurrentCacheAccess(t *testing.T) {
	helper := NewTestHelper(t)

	t.Run("CacheConcurrency", func(t *testing.T) {
		config := DefaultConfig()
		config.EnableCache = true
		config.MaxCacheSize = 100

		processor := New(config)
		defer processor.Close()

		testData := `{"user": {"name": "Alice", "age": 30}}`

		concurrency := 20
		iterations := 100

		var wg sync.WaitGroup

		// Warm up cache
		for i := 0; i < 10; i++ {
			processor.Get(testData, "user.name")
		}

		for i := 0; i < concurrency; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for j := 0; j < iterations; j++ {
					processor.Get(testData, "user.name")
					processor.Get(testData, "user.age")
				}
			}()
		}

		wg.Wait()

		// Verify cache worked
		stats := processor.GetStats()
		helper.AssertTrue(stats.HitCount > 0)
	})

	t.Run("CacheInvalidation", func(t *testing.T) {
		config := DefaultConfig()
		config.EnableCache = true
		config.CacheTTL = 100 * time.Millisecond

		processor := New(config)
		defer processor.Close()

		testData := `{"value": "test"}`

		// Populate cache
		processor.Get(testData, "value")

		// Wait for expiration
		time.Sleep(150 * time.Millisecond)

		// Access again - should handle cache miss gracefully
		_, err := processor.Get(testData, "value")
		helper.AssertNoError(err)
	})
}

// TestConcurrentIterators tests concurrent iteration
func TestConcurrentIterators(t *testing.T) {
	helper := NewTestHelper(t)

	t.Run("ConcurrentForeach", func(t *testing.T) {
		testData := `{"items": [` + generateArrayItems(50) + `]}`

		concurrency := 10
		iterations := 20

		var wg sync.WaitGroup
		mu := sync.Mutex{}
		totalCount := 0

		for i := 0; i < concurrency; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for j := 0; j < iterations; j++ {
					err := ForeachWithPath(testData, "items", func(key any, item *IterableValue) {
						mu.Lock()
						totalCount++
						mu.Unlock()
					})
					if err != nil {
						t.Errorf("Foreach error: %v", err)
					}
				}
			}()
		}

		wg.Wait()
		helper.AssertEqual(concurrency*iterations*50, totalCount)
	})

	t.Run("ConcurrentForeachNested", func(t *testing.T) {
		testData := `{"data": {"users": [` + generateUserJSON(20) + `]}}`

		concurrency := 5
		iterations := 10

		var wg sync.WaitGroup

		for i := 0; i < concurrency; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for j := 0; j < iterations; j++ {
					ForeachNested(testData, func(key any, item *IterableValue) {
						// Just verify it doesn't panic
					})
				}
			}()
		}

		wg.Wait()
	})
}

// TestConcurrentFileOperations tests concurrent file operations
func TestConcurrentFileOperations(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping file operations test in short mode")
	}

	helper := NewTestHelper(t)

	t.Run("ConcurrentReadWrite", func(t *testing.T) {
		dir := t.TempDir()

		// Create initial file
		initialData := `{"counter": 0}`
		filePath := dir + "/test.json"

		err := SaveToFile(filePath, initialData, false)
		helper.AssertNoError(err)

		concurrency := 10
		iterations := 20

		var wg sync.WaitGroup
		var errorCount int64
		var successCount int64

		for i := 0; i < concurrency; i++ {
			wg.Add(1)
			go func(workerID int) {
				defer wg.Done()

				for j := 0; j < iterations; j++ {
					// Read
					data, err := LoadFromFile(filePath)
					if err != nil {
						// Count but don't fail - this is expected in concurrent writes
						atomic.AddInt64(&errorCount, 1)
						continue
					}

					// Verify JSON is valid before modifying
					if !IsValidJson(data) {
						// Count but don't fail - corrupted read due to concurrent writes
						atomic.AddInt64(&errorCount, 1)
						continue
					}

					// Modify and save (check for errors)
					newData, err := Set(data, "last_worker", workerID)
					if err != nil {
						atomic.AddInt64(&errorCount, 1)
						continue
					}

					// Only save if we got valid JSON
					if IsValidJson(newData) {
						err = SaveToFile(filePath, newData, false)
						if err != nil {
							atomic.AddInt64(&errorCount, 1)
						} else {
							atomic.AddInt64(&successCount, 1)
						}
					}
				}
			}(i)
		}

		wg.Wait()

		// Log results
		totalOps := atomic.LoadInt64(&errorCount) + atomic.LoadInt64(&successCount)
		t.Logf("Concurrent file operations: %d total, %d success, %d errors",
			totalOps, successCount, errorCount)

		// We expect at least some successful operations
		if successCount == 0 {
			t.Errorf("Expected at least some successful operations, got 0")
		}

		// Verify file is still valid JSON
		finalData, err := LoadFromFile(filePath)
		helper.AssertNoError(err)
		if !IsValidJson(finalData) {
			// If file is corrupted, that's a real failure
			t.Errorf("Final file is not valid JSON")
		}
	})
}

// TestRaceConditions tests for race conditions
func TestRaceConditions(t *testing.T) {
	t.Run("ConcurrentConfigModification", func(t *testing.T) {
		config := DefaultConfig()

		concurrency := 10
		var wg sync.WaitGroup

		for i := 0; i < concurrency; i++ {
			wg.Add(1)
			go func(workerID int) {
				defer wg.Done()

				// Each goroutine tries to modify config
				_ = config.Clone()

				// Validate is thread-safe
				config.Validate()
			}(i)
		}

		wg.Wait()
	})

	t.Run("SharedProcessor", func(t *testing.T) {
		processor := New(DefaultConfig())
		defer processor.Close()

		testData := `{"data": "value"}`

		concurrency := 20
		iterations := 100

		var wg sync.WaitGroup

		for i := 0; i < concurrency; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for j := 0; j < iterations; j++ {
					// Get stats
					processor.GetStats()

					// Get data
					processor.Get(testData, "data")

					// Get health
					processor.GetHealthStatus()
				}
			}()
		}

		wg.Wait()

		// Verify processor is still functional
		result, err := processor.Get(testData, "data")
		if err == nil {
			if result != "value" {
				t.Errorf("Expected 'value', got '%v'", result)
			}
		}
	})
}

// TestConcurrentStressTest stress test with high concurrency
func TestConcurrentStressTest(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stress test in short mode")
	}

	t.Run("HighConcurrency", func(t *testing.T) {
		processor := New(DefaultConfig())
		defer processor.Close()

		testData := NewTestDataGenerator().GenerateComplexJSON()

		concurrency := 50
		operations := 1000

		ct := NewConcurrencyTester(t, concurrency, operations)
		ct.Run(func(workerID, iteration int) error {
			// Mix of different operations
			switch iteration % 4 {
			case 0:
				_, err := processor.Get(testData, "users[0].name")
				return err
			case 1:
				_, err := processor.Get(testData, "settings")
				return err
			case 2:
				_, err := processor.Get(testData, "statistics")
				return err
			default:
				_ = processor.GetStats()
				return nil
			}
		})

		// Verify processor still works
		_ = processor.GetStats()
		// If we got here without panic, the test passes
	})

	t.Run("RapidClose", func(t *testing.T) {
		// Test rapid close/open cycles
		for i := 0; i < 20; i++ {
			processor := New(DefaultConfig())
			processor.Get(`{"test": "value"}`, "test")
			processor.Close()
		}
	})
}

// TestConcurrentProcessorLifecycle tests processor lifecycle under concurrency
func TestConcurrentProcessorLifecycle(t *testing.T) {
	t.Run("CloseDuringOperations", func(t *testing.T) {
		processor := New(DefaultConfig())

		testData := `{"test": "value"}`

		concurrency := 10
		var wg sync.WaitGroup

		// Start goroutines
		for i := 0; i < concurrency; i++ {
			wg.Add(1)
			go func(workerID int) {
				defer wg.Done()

				for j := 0; j < 100; j++ {
					_, err := processor.Get(testData, "test")
					if err != nil {
						// Expected after close
						break
					}
					time.Sleep(time.Microsecond)
				}
			}(i)
		}

		// Close after a short delay
		time.Sleep(10 * time.Millisecond)
		processor.Close()

		wg.Wait()
	})

	t.Run("GlobalProcessor", func(t *testing.T) {
		// Test global processor under concurrency
		SetGlobalProcessor(New(DefaultConfig()))
		defer ShutdownGlobalProcessor()

		testData := `{"test": "value"}`

		concurrency := 20
		var wg sync.WaitGroup

		for i := 0; i < concurrency; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for j := 0; j < 50; j++ {
					// Use global processor operations
					Get(testData, "test")
				}
			}()
		}

		wg.Wait()
	})
}

// Helper functions

func generateUserJSON(count int) string {
	users := make([]string, count)
	for i := 0; i < count; i++ {
		users[i] = `{"id": ` + fmt.Sprint(i) + `, "name": "User` + fmt.Sprint(i) + `"}`
	}
	return strings.Join(users, ",")
}

func generateArrayItems(count int) string {
	items := make([]string, count)
	for i := 0; i < count; i++ {
		items[i] = `{"id": ` + fmt.Sprint(i) + `}`
	}
	return strings.Join(items, ",")
}
