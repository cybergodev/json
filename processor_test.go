package json

import (
	"testing"
	"time"
)

// TestProcessor consolidates processor-specific functionality
// Replaces: processor_core_comprehensive_test.go
func TestProcessor(t *testing.T) {
	helper := NewTestHelper(t)

	t.Run("GetMultiple", func(t *testing.T) {
		processor := New()
		defer processor.Close()

		testData := `{
			"user": {"name": "John", "age": 30, "email": "john@example.com"},
			"items": [1, 2, 3, 4, 5],
			"active": true
		}`

		paths := []string{"user.name", "user.age", "active", "items[0]"}
		results, err := processor.GetMultiple(testData, paths)
		helper.AssertNoError(err)
		helper.AssertEqual(4, len(results))
		helper.AssertEqual("John", results["user.name"])
		helper.AssertEqual(float64(30), results["user.age"])
		helper.AssertEqual(true, results["active"])
		helper.AssertEqual(float64(1), results["items[0]"])
	})

	t.Run("SetMultiple", func(t *testing.T) {
		processor := New()
		defer processor.Close()

		testData := `{"user": {"name": "John", "age": 30}, "items": [1, 2, 3]}`
		result := testData
		var err error

		result, err = processor.Set(result, "user.name", "Jane")
		helper.AssertNoError(err)
		result, err = processor.Set(result, "user.age", 25)
		helper.AssertNoError(err)
		result, err = processor.Set(result, "items[0]", 10)
		helper.AssertNoError(err)

		name, _ := processor.Get(result, "user.name")
		helper.AssertEqual("Jane", name)
		age, _ := processor.Get(result, "user.age")
		helper.AssertEqual(float64(25), age)
		item, _ := processor.Get(result, "items[0]")
		helper.AssertEqual(float64(10), item)
	})

	t.Run("ProcessBatch", func(t *testing.T) {
		processor := New()
		defer processor.Close()

		operations := []BatchOperation{
			{Type: "get", JSONStr: `{"name": "John", "age": 30}`, Path: "name", ID: "op1"},
			{Type: "set", JSONStr: `{"name": "John", "age": 30}`, Path: "age", Value: 35, ID: "op2"},
			{Type: "delete", JSONStr: `{"name": "John", "age": 30, "city": "NYC"}`, Path: "city", ID: "op3"},
		}

		results, err := processor.ProcessBatch(operations)
		helper.AssertNoError(err)
		helper.AssertEqual(3, len(results))
		helper.AssertEqual("John", results[0].Result)
	})

	t.Run("Configuration", func(t *testing.T) {
		config := DefaultConfig()
		config.MaxJSONSize = 1024 * 1024
		processor := New(config)
		defer processor.Close()

		testData := `{"test": "value"}`
		result, err := processor.Get(testData, "test")
		helper.AssertNoError(err)
		helper.AssertEqual("value", result)
	})

	t.Run("Stats", func(t *testing.T) {
		processor := New()
		defer processor.Close()

		testData := `{"test": "value"}`
		_, _ = processor.Get(testData, "test")
		_, _ = processor.Set(testData, "new", "value")

		stats := processor.GetStats()
		helper.AssertTrue(stats.OperationCount > 0)
	})

	t.Run("HealthStatus", func(t *testing.T) {
		processor := New()
		defer processor.Close()

		health := processor.GetHealthStatus()
		helper.AssertNotNil(health)
		helper.AssertTrue(len(health.Checks) > 0)

		timeDiff := time.Since(health.Timestamp)
		helper.AssertTrue(timeDiff < time.Minute)
	})

	t.Run("CacheOperations", func(t *testing.T) {
		processor := New()
		defer processor.Close()

		testData := `{"test": "value", "number": 42}`
		_, _ = processor.Get(testData, "test")
		_, _ = processor.Get(testData, "number")

		processor.ClearCache()

		samplePaths := []string{"test", "number"}
		_, _ = processor.WarmupCache(testData, samplePaths)

		result, err := processor.Get(testData, "test")
		helper.AssertNoError(err)
		helper.AssertEqual("value", result)
	})

	t.Run("Lifecycle", func(t *testing.T) {
		processor := New()
		helper.AssertFalse(processor.IsClosed())

		processor.Close()
		helper.AssertTrue(processor.IsClosed())

		_, err := processor.Get(`{"test": "value"}`, "test")
		helper.AssertError(err)
	})

	t.Run("MemoryManagement", func(t *testing.T) {
		processor := New()
		defer processor.Close()

		testData := `{"test": "value"}`
		for i := 0; i < 1000; i++ {
			_, _ = processor.Get(testData, "test")
			if i%100 == 0 {
				processor.ClearCache()
			}
		}

		result, err := processor.Get(testData, "test")
		helper.AssertNoError(err)
		helper.AssertEqual("value", result)
	})
}
