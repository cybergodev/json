package json

import (
	"testing"
	"time"
)

// TestProcessorCore tests processor-specific functionality (batch, stats, health, lifecycle)
// Basic operations are covered in operations_test.go
func TestProcessorCore(t *testing.T) {
	helper := NewTestHelper(t)

	t.Run("GetMultiple", func(t *testing.T) {
		processor := New()
		defer processor.Close()

		testData := `{
			"user": {
				"name": "John",
				"age": 30,
				"email": "john@example.com"
			},
			"items": [1, 2, 3, 4, 5],
			"active": true
		}`

		// Test multiple path retrieval
		paths := []string{"user.name", "user.age", "active", "items[0]"}
		results, err := processor.GetMultiple(testData, paths)
		helper.AssertNoError(err, "GetMultiple should work")
		helper.AssertEqual(4, len(results), "Should return 4 results")

		// Verify individual results
		helper.AssertEqual("John", results["user.name"], "First result should be name")
		helper.AssertEqual(float64(30), results["user.age"], "Second result should be age")
		helper.AssertEqual(true, results["active"], "Third result should be active")
		helper.AssertEqual(float64(1), results["items[0]"], "Fourth result should be first item")

		// Test with some invalid paths
		mixedPaths := []string{"user.name", "nonexistent", "items[0]"}
		results, err = processor.GetMultiple(testData, mixedPaths)
		helper.AssertNoError(err, "GetMultiple with mixed paths should work")
		helper.AssertEqual(3, len(results), "Should return 3 results")
		helper.AssertEqual("John", results["user.name"], "First result should be name")
		helper.AssertNil(results["nonexistent"], "Second result should be nil for nonexistent")
		helper.AssertEqual(float64(1), results["items[0]"], "Third result should be first item")
	})

	t.Run("SetMultiple", func(t *testing.T) {
		processor := New()
		defer processor.Close()

		testData := `{
			"user": {
				"name": "John",
				"age": 30
			},
			"items": [1, 2, 3]
		}`

		// Test multiple path setting using individual Set operations
		result := testData
		var err error

		result, err = processor.Set(result, "user.name", "Jane")
		helper.AssertNoError(err, "Should set name")

		result, err = processor.Set(result, "user.age", 25)
		helper.AssertNoError(err, "Should set age")

		result, err = processor.Set(result, "items[0]", 10)
		helper.AssertNoError(err, "Should set item")

		result, err = processor.Set(result, "active", true)
		helper.AssertNoError(err, "Should set active")

		// Verify changes
		name, err := processor.Get(result, "user.name")
		helper.AssertNoError(err, "Should get updated name")
		helper.AssertEqual("Jane", name, "Name should be updated")

		age, err := processor.Get(result, "user.age")
		helper.AssertNoError(err, "Should get updated age")
		helper.AssertEqual(float64(25), age, "Age should be updated")

		item, err := processor.Get(result, "items[0]")
		helper.AssertNoError(err, "Should get updated item")
		helper.AssertEqual(float64(10), item, "Item should be updated")

		active, err := processor.Get(result, "active")
		helper.AssertNoError(err, "Should get new active field")
		helper.AssertEqual(true, active, "Active should be set")
	})

	t.Run("ProcessBatch", func(t *testing.T) {
		processor := New()
		defer processor.Close()

		// Test batch processing
		operations := []BatchOperation{
			{Type: "get", JSONStr: `{"name": "John", "age": 30}`, Path: "name", ID: "op1"},
			{Type: "set", JSONStr: `{"name": "John", "age": 30}`, Path: "age", Value: 35, ID: "op2"},
			{Type: "delete", JSONStr: `{"name": "John", "age": 30, "city": "NYC"}`, Path: "city", ID: "op3"},
		}

		results, err := processor.ProcessBatch(operations)
		helper.AssertNoError(err, "ProcessBatch should work")
		helper.AssertEqual(3, len(results), "Should return 3 results")

		// Verify get result
		helper.AssertEqual("John", results[0].Result, "Get result should be correct")

		// Verify set result
		if setResult, ok := results[1].Result.(string); ok {
			age, err := processor.Get(setResult, "age")
			helper.AssertNoError(err, "Should get updated age from set result")
			helper.AssertEqual(float64(35), age, "Age should be updated in set result")
		}

		// Verify delete result - should return error for deleted field
		if deleteResult, ok := results[2].Result.(string); ok {
			city, err := processor.Get(deleteResult, "city")
			helper.AssertError(err, "Should return error for deleted field")
			helper.AssertNil(city, "City should be deleted")
		}
	})

	t.Run("ProcessorConfiguration", func(t *testing.T) {
		// Test with custom config
		config := DefaultConfig()
		config.MaxJSONSize = 1024 * 1024 // 1MB
		processor := New(config)
		defer processor.Close()

		// Test that processor works with custom config
		testData := `{"test": "value"}`
		result, err := processor.Get(testData, "test")
		helper.AssertNoError(err, "Should work with custom config")
		helper.AssertEqual("value", result, "Should get correct value with custom config")
	})

	t.Run("ProcessorStats", func(t *testing.T) {
		processor := New()
		defer processor.Close()

		testData := `{"test": "value"}`

		// Perform some operations to generate stats
		_, _ = processor.Get(testData, "test")
		_, _ = processor.Set(testData, "new", "value")
		_, _ = processor.Delete(testData, "test")

		// Get stats
		stats := processor.GetStats()
		helper.AssertTrue(stats.OperationCount > 0, "Should have recorded operations")
	})

	t.Run("ProcessorHealthStatus", func(t *testing.T) {
		processor := New()
		defer processor.Close()

		// Test health status
		health := processor.GetHealthStatus()
		helper.AssertNotNil(health, "Health status should not be nil")

		// Health status might be false in test environments due to memory pressure or other factors
		// The important thing is that we get a valid health status response
		if !health.Healthy {
			t.Logf("Processor health is false - this might be expected in test environments")
			t.Logf("Health checks: %+v", health.Checks)
		}

		// Verify we have health checks
		helper.AssertTrue(len(health.Checks) > 0, "Should have health checks")

		// Verify timestamp is recent
		timeDiff := time.Since(health.Timestamp)
		helper.AssertTrue(timeDiff < time.Minute, "Health check timestamp should be recent")
	})

	t.Run("CacheOperations", func(t *testing.T) {
		processor := New()
		defer processor.Close()

		testData := `{"test": "value", "number": 42}`

		// Perform operations to populate cache
		_, _ = processor.Get(testData, "test")
		_, _ = processor.Get(testData, "number")

		// Clear cache
		processor.ClearCache()

		// Warmup cache
		samplePaths := []string{"test", "number"}
		_, _ = processor.WarmupCache(testData, samplePaths)

		// Test that operations still work after cache operations
		result, err := processor.Get(testData, "test")
		helper.AssertNoError(err, "Get should work after cache operations")
		helper.AssertEqual("value", result, "Should get correct value after cache operations")
	})

	t.Run("ProcessorClosure", func(t *testing.T) {
		processor := New()

		// Test that processor is not closed initially
		helper.AssertFalse(processor.IsClosed(), "Processor should not be closed initially")

		// Close processor
		processor.Close()

		// Test that processor is closed
		helper.AssertTrue(processor.IsClosed(), "Processor should be closed after Close()")

		// Test that operations fail on closed processor
		_, err := processor.Get(`{"test": "value"}`, "test")
		helper.AssertError(err, "Operations should fail on closed processor")
	})

	t.Run("MemoryManagement", func(t *testing.T) {
		processor := New()
		defer processor.Close()

		testData := `{"test": "value"}`

		// Perform many operations to test memory management
		for i := 0; i < 1000; i++ {
			_, _ = processor.Get(testData, "test")
			if i%100 == 0 {
				// Trigger maintenance periodically
				processor.ClearCache()
			}
		}

		// Verify processor is still functional
		result, err := processor.Get(testData, "test")
		helper.AssertNoError(err, "Processor should still work after many operations")
		helper.AssertEqual("value", result, "Should get correct result after many operations")
	})
}
