package json

import (
	"fmt"
	"math/rand"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"
)

// TestHelper provides utilities for testing JSON operations
type TestHelper struct {
	t *testing.T
}

// TestProcessorResourcePools tests processor resource pool functionality
func TestProcessorResourcePools(processor *Processor) bool {
	// Test string builder pool
	sb := processor.getStringBuilder()
	if sb == nil {
		return false
	}
	processor.putStringBuilder(sb)

	// Test path segments pool
	segments := processor.getPathSegments()
	if segments == nil {
		return false
	}
	processor.putPathSegments(segments)

	return true
}

// NewTestHelper creates a new test helper
func NewTestHelper(t *testing.T) *TestHelper {
	return &TestHelper{t: t}
}

// AssertEqual checks if two values are equal
func (h *TestHelper) AssertEqual(expected, actual any, msgAndArgs ...any) {
	h.t.Helper()
	if !reflect.DeepEqual(expected, actual) {
		msg := "Values are not equal"
		if len(msgAndArgs) > 0 {
			msg = fmt.Sprintf(msgAndArgs[0].(string), msgAndArgs[1:]...)
		}
		h.t.Errorf("%s\nExpected: %v (%T)\nActual: %v (%T)", msg, expected, expected, actual, actual)
	}
}

// AssertNotEqual checks if two values are not equal
func (h *TestHelper) AssertNotEqual(expected, actual any, msgAndArgs ...any) {
	h.t.Helper()
	if reflect.DeepEqual(expected, actual) {
		msg := "Values should not be equal"
		if len(msgAndArgs) > 0 {
			msg = fmt.Sprintf(msgAndArgs[0].(string), msgAndArgs[1:]...)
		}
		h.t.Errorf("%s\nBoth values: %v (%T)", msg, expected, expected)
	}
}

// AssertNoError checks that error is nil
func (h *TestHelper) AssertNoError(err error, msgAndArgs ...any) {
	h.t.Helper()
	if err != nil {
		msg := "Expected no error"
		if len(msgAndArgs) > 0 {
			msg = fmt.Sprintf(msgAndArgs[0].(string), msgAndArgs[1:]...)
		}
		h.t.Errorf("%s, but got: %v", msg, err)
	}
}

// AssertError checks that error is not nil
func (h *TestHelper) AssertError(err error, msgAndArgs ...any) {
	h.t.Helper()
	if err == nil {
		msg := "Expected an error"
		if len(msgAndArgs) > 0 {
			msg = fmt.Sprintf(msgAndArgs[0].(string), msgAndArgs[1:]...)
		}
		h.t.Error(msg + ", but got nil")
	}
}

// AssertErrorContains checks that error contains specific text
func (h *TestHelper) AssertErrorContains(err error, contains string, msgAndArgs ...any) {
	h.t.Helper()
	if err == nil {
		msg := "Expected an error"
		if len(msgAndArgs) > 0 {
			msg = fmt.Sprintf(msgAndArgs[0].(string), msgAndArgs[1:]...)
		}
		h.t.Error(msg + ", but got nil")
		return
	}
	if !strings.Contains(err.Error(), contains) {
		msg := fmt.Sprintf("Expected error to contain '%s'", contains)
		if len(msgAndArgs) > 0 {
			msg = fmt.Sprintf(msgAndArgs[0].(string), msgAndArgs[1:]...)
		}
		h.t.Errorf("%s, but got: %v", msg, err)
	}
}

// AssertPanic checks that function panics
func (h *TestHelper) AssertPanic(fn func(), msgAndArgs ...any) {
	h.t.Helper()
	defer func() {
		if r := recover(); r == nil {
			msg := "Expected function to panic"
			if len(msgAndArgs) > 0 {
				msg = fmt.Sprintf(msgAndArgs[0].(string), msgAndArgs[1:]...)
			}
			h.t.Error(msg + ", but it didn't")
		}
	}()
	fn()
}

// AssertNoPanic checks that function doesn't panic
func (h *TestHelper) AssertNoPanic(fn func(), msgAndArgs ...any) {
	h.t.Helper()
	defer func() {
		if r := recover(); r != nil {
			msg := "Expected function not to panic"
			if len(msgAndArgs) > 0 {
				msg = fmt.Sprintf(msgAndArgs[0].(string), msgAndArgs[1:]...)
			}
			h.t.Errorf("%s, but it panicked with: %v", msg, r)
		}
	}()
	fn()
}

// AssertTrue checks that condition is true
func (h *TestHelper) AssertTrue(condition bool, msgAndArgs ...any) {
	h.t.Helper()
	if !condition {
		msg := "Expected condition to be true"
		if len(msgAndArgs) > 0 {
			msg = fmt.Sprintf(msgAndArgs[0].(string), msgAndArgs[1:]...)
		}
		h.t.Error(msg)
	}
}

// AssertFalse checks that condition is false
func (h *TestHelper) AssertFalse(condition bool, msgAndArgs ...any) {
	h.t.Helper()
	if condition {
		msg := "Expected condition to be false"
		if len(msgAndArgs) > 0 {
			msg = fmt.Sprintf(msgAndArgs[0].(string), msgAndArgs[1:]...)
		}
		h.t.Error(msg)
	}
}

// AssertNotNil checks that value is not nil
func (h *TestHelper) AssertNotNil(value any, msgAndArgs ...any) {
	h.t.Helper()
	if value == nil {
		msg := "Expected value to be not nil"
		if len(msgAndArgs) > 0 {
			msg = fmt.Sprintf(msgAndArgs[0].(string), msgAndArgs[1:]...)
		}
		h.t.Error(msg)
	}
}

// AssertNil checks that value is nil
func (h *TestHelper) AssertNil(value any, msgAndArgs ...any) {
	h.t.Helper()
	if value != nil {
		msg := "Expected value to be nil"
		if len(msgAndArgs) > 0 {
			msg = fmt.Sprintf(msgAndArgs[0].(string), msgAndArgs[1:]...)
		}
		h.t.Errorf("%s, but got: %v (%T)", msg, value, value)
	}
}

// TestDataGenerator generates test data for various scenarios
type TestDataGenerator struct {
	rand *rand.Rand
}

// NewTestDataGenerator creates a new test data generator
func NewTestDataGenerator() *TestDataGenerator {
	return &TestDataGenerator{
		rand: rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// GenerateSimpleJSON generates simple JSON structures
func (g *TestDataGenerator) GenerateSimpleJSON() string {
	templates := []string{
		`{"name":"John","age":30}`,
		`{"active":true,"score":95.5}`,
		`{"items":[1,2,3,4,5]}`,
		`{"user":{"name":"Alice","email":"alice@example.com"}}`,
		`{"data":null,"empty":""}`,
	}
	return templates[g.rand.Intn(len(templates))]
}

// GenerateComplexJSON generates complex nested JSON structures
func (g *TestDataGenerator) GenerateComplexJSON() string {
	return `{
		"users": [
			{
				"id": 1,
				"name": "Alice Johnson",
				"email": "alice@example.com",
				"profile": {
					"age": 28,
					"location": "New York",
					"preferences": {
						"theme": "dark",
						"notifications": true,
						"languages": ["en", "es", "fr"]
					}
				},
				"roles": ["admin", "user"],
				"metadata": {
					"created": "2023-01-15T10:30:00Z",
					"lastLogin": "2024-01-20T14:45:00Z",
					"loginCount": 156
				}
			},
			{
				"id": 2,
				"name": "Bob Smith",
				"email": "bob@example.com",
				"profile": {
					"age": 35,
					"location": "San Francisco",
					"preferences": {
						"theme": "light",
						"notifications": false,
						"languages": ["en"]
					}
				},
				"roles": ["user"],
				"metadata": {
					"created": "2023-03-22T09:15:00Z",
					"lastLogin": "2024-01-19T16:20:00Z",
					"loginCount": 89
				}
			}
		],
		"settings": {
			"appName": "TestApp",
			"version": "1.2.3",
			"features": {
				"authentication": true,
				"logging": true,
				"caching": false
			},
			"limits": {
				"maxUsers": 1000,
				"maxRequests": 10000,
				"timeout": 30.5
			}
		},
		"statistics": {
			"totalUsers": 2,
			"activeUsers": 1,
			"metrics": [
				{"name": "cpu", "value": 45.2, "unit": "%"},
				{"name": "memory", "value": 78.9, "unit": "%"},
				{"name": "disk", "value": 23.1, "unit": "%"}
			]
		}
	}`
}

// GenerateArrayJSON generates JSON with various array structures
func (g *TestDataGenerator) GenerateArrayJSON() string {
	return `{
		"numbers": [1, 2, 3, 4, 5, 6, 7, 8, 9, 10],
		"strings": ["apple", "banana", "cherry", "date", "elderberry"],
		"mixed": [1, "two", 3.0, true, null, {"nested": "object"}],
		"nested": [
			[1, 2, 3],
			[4, 5, 6],
			[7, 8, 9]
		],
		"objects": [
			{"id": 1, "name": "Item 1", "active": true},
			{"id": 2, "name": "Item 2", "active": false},
			{"id": 3, "name": "Item 3", "active": true}
		],
		"empty": [],
		"nulls": [null, null, null]
	}`
}

// GenerateInvalidJSON generates invalid JSON for error testing
func (g *TestDataGenerator) GenerateInvalidJSON() []string {
	return []string{
		`{invalid json}`,
		`{"unclosed": "string}`,
		`{"trailing": "comma",}`,
		// Note: JSON allows duplicate keys, last one wins, so this is actually valid
		// `{"duplicate": 1, "duplicate": 2}`,
		`{unquoted: "key"}`,
		`{"number": 123.45.67}`,
		`{"array": [1, 2, 3,]}`,
		`{"nested": {"unclosed": }`,
		``,
		`null extra content`,
	}
}

// MemoryTracker tracks memory usage during tests
type MemoryTracker struct {
	initialStats runtime.MemStats
	name         string
}

// NewMemoryTracker creates a new memory tracker
func NewMemoryTracker(name string) *MemoryTracker {
	var stats runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&stats)
	return &MemoryTracker{
		initialStats: stats,
		name:         name,
	}
}

// Report reports memory usage
func (mt *MemoryTracker) Report(t *testing.T) {
	var stats runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&stats)

	allocDiff := int64(stats.Alloc) - int64(mt.initialStats.Alloc)
	totalAllocDiff := int64(stats.TotalAlloc) - int64(mt.initialStats.TotalAlloc)

	t.Logf("Memory usage for %s:", mt.name)
	t.Logf("  Current alloc diff: %d bytes", allocDiff)
	t.Logf("  Total alloc diff: %d bytes", totalAllocDiff)
	t.Logf("  Mallocs: %d", stats.Mallocs-mt.initialStats.Mallocs)
	t.Logf("  Frees: %d", stats.Frees-mt.initialStats.Frees)
}

// ConcurrencyTester helps test concurrent operations
type ConcurrencyTester struct {
	t           *testing.T
	concurrency int
	iterations  int
}

// NewConcurrencyTester creates a new concurrency tester
func NewConcurrencyTester(t *testing.T, concurrency, iterations int) *ConcurrencyTester {
	return &ConcurrencyTester{
		t:           t,
		concurrency: concurrency,
		iterations:  iterations,
	}
}

// Run runs concurrent test operations
func (ct *ConcurrencyTester) Run(operation func(workerID, iteration int) error) {
	ct.t.Helper()

	done := make(chan error, ct.concurrency)

	for i := 0; i < ct.concurrency; i++ {
		go func(workerID int) {
			for j := 0; j < ct.iterations; j++ {
				if err := operation(workerID, j); err != nil {
					done <- fmt.Errorf("worker %d, iteration %d: %w", workerID, j, err)
					return
				}
			}
			done <- nil
		}(i)
	}

	for i := 0; i < ct.concurrency; i++ {
		if err := <-done; err != nil {
			ct.t.Errorf("Concurrent operation failed: %v", err)
		}
	}
}

// BenchmarkHelper provides utilities for benchmark tests
type BenchmarkHelper struct {
	b *testing.B
}

// NewBenchmarkHelper creates a new benchmark helper
func NewBenchmarkHelper(b *testing.B) *BenchmarkHelper {
	return &BenchmarkHelper{b: b}
}

// MeasureMemory measures memory allocations during benchmark
func (bh *BenchmarkHelper) MeasureMemory(fn func()) {
	bh.b.Helper()
	bh.b.ReportAllocs()

	var before, after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)

	fn()

	runtime.GC()
	runtime.ReadMemStats(&after)

	bh.b.ReportMetric(float64(after.Alloc-before.Alloc), "bytes/op")
	bh.b.ReportMetric(float64(after.Mallocs-before.Mallocs), "allocs/op")
}
