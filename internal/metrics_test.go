package internal

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestMetricsCollector(t *testing.T) {
	t.Run("Creation", func(t *testing.T) {
		mc := NewMetricsCollector()
		if mc == nil {
			t.Fatal("NewMetricsCollector returned nil")
		}
		if mc.startTime.IsZero() {
			t.Error("Start time should be set")
		}
	})

	t.Run("RecordOperation", func(t *testing.T) {
		mc := NewMetricsCollector()

		duration := 100 * time.Millisecond
		mc.RecordOperation(duration, true, 1024)

		totalOps := atomic.LoadInt64(&mc.totalOperations)
		if totalOps != 1 {
			t.Errorf("Expected 1 operation, got %d", totalOps)
		}

		successOps := atomic.LoadInt64(&mc.successfulOps)
		if successOps != 1 {
			t.Errorf("Expected 1 successful operation, got %d", successOps)
		}
	})

	t.Run("RecordFailedOperation", func(t *testing.T) {
		mc := NewMetricsCollector()

		mc.RecordOperation(50*time.Millisecond, false, 0)

		failedOps := atomic.LoadInt64(&mc.failedOps)
		if failedOps != 1 {
			t.Errorf("Expected 1 failed operation, got %d", failedOps)
		}
	})

	t.Run("TimingMetrics", func(t *testing.T) {
		mc := NewMetricsCollector()

		durations := []time.Duration{
			100 * time.Millisecond,
			200 * time.Millisecond,
			50 * time.Millisecond,
		}

		for _, d := range durations {
			mc.RecordOperation(d, true, 0)
		}

		maxTime := atomic.LoadInt64(&mc.maxProcessingTime)
		minTime := atomic.LoadInt64(&mc.minProcessingTime)

		if maxTime < (200 * time.Millisecond).Nanoseconds() {
			t.Error("Max time should be at least 200ms")
		}
		if minTime > (50 * time.Millisecond).Nanoseconds() {
			t.Error("Min time should be at most 50ms")
		}
	})

	t.Run("CacheMetrics", func(t *testing.T) {
		mc := NewMetricsCollector()

		mc.RecordCacheHit()
		mc.RecordCacheHit()
		mc.RecordCacheMiss()

		hits := atomic.LoadInt64(&mc.cacheHits)
		misses := atomic.LoadInt64(&mc.cacheMisses)

		if hits != 2 {
			t.Errorf("Expected 2 cache hits, got %d", hits)
		}
		if misses != 1 {
			t.Errorf("Expected 1 cache miss, got %d", misses)
		}
	})

	t.Run("ConcurrencyMetrics", func(t *testing.T) {
		mc := NewMetricsCollector()

		mc.StartConcurrentOperation()
		mc.StartConcurrentOperation()
		mc.StartConcurrentOperation()

		active := atomic.LoadInt64(&mc.activeConcurrentOps)
		if active != 3 {
			t.Errorf("Expected 3 active operations, got %d", active)
		}

		mc.EndConcurrentOperation()
		active = atomic.LoadInt64(&mc.activeConcurrentOps)
		if active != 2 {
			t.Errorf("Expected 2 active operations after end, got %d", active)
		}

		maxConcurrent := atomic.LoadInt64(&mc.maxConcurrentOps)
		if maxConcurrent < 3 {
			t.Errorf("Max concurrent should be at least 3, got %d", maxConcurrent)
		}
	})

	t.Run("ErrorTracking", func(t *testing.T) {
		mc := NewMetricsCollector()

		mc.RecordError("ParseError")
		mc.RecordError("ParseError")
		mc.RecordError("ValidationError")

		parseErrors := int64(0)
		if val, ok := mc.errorsByType.Load("ParseError"); ok {
			parseErrors = val.(int64)
		}

		if parseErrors != 2 {
			t.Errorf("Expected 2 parse errors, got %d", parseErrors)
		}

		validationErrors := int64(0)
		if val, ok := mc.errorsByType.Load("ValidationError"); ok {
			validationErrors = val.(int64)
		}

		if validationErrors != 1 {
			t.Errorf("Expected 1 validation error, got %d", validationErrors)
		}
	})

	t.Run("ConcurrentRecording", func(t *testing.T) {
		mc := NewMetricsCollector()

		var wg sync.WaitGroup
		workers := 10
		operations := 100

		wg.Add(workers)
		for i := 0; i < workers; i++ {
			go func() {
				defer wg.Done()
				for j := 0; j < operations; j++ {
					mc.RecordOperation(time.Millisecond, true, 100)
					mc.RecordCacheHit()
				}
			}()
		}
		wg.Wait()

		totalOps := atomic.LoadInt64(&mc.totalOperations)
		expected := int64(workers * operations)
		if totalOps != expected {
			t.Errorf("Expected %d operations, got %d", expected, totalOps)
		}

		cacheHits := atomic.LoadInt64(&mc.cacheHits)
		if cacheHits != expected {
			t.Errorf("Expected %d cache hits, got %d", expected, cacheHits)
		}
	})

	t.Run("MemoryTracking", func(t *testing.T) {
		mc := NewMetricsCollector()

		mc.RecordOperation(time.Millisecond, true, 1024)
		mc.RecordOperation(time.Millisecond, true, 2048)

		totalMemory := atomic.LoadInt64(&mc.totalMemoryAllocated)
		if totalMemory != 3072 {
			t.Errorf("Expected 3072 bytes allocated, got %d", totalMemory)
		}
	})

	t.Run("GetMetrics", func(t *testing.T) {
		mc := NewMetricsCollector()

		mc.RecordOperation(100*time.Millisecond, true, 1024)
		mc.RecordOperation(200*time.Millisecond, false, 512)
		mc.RecordCacheHit()
		mc.RecordCacheMiss()

		metrics := mc.GetMetrics()

		if metrics.TotalOperations != 2 {
			t.Errorf("Expected 2 total operations in metrics")
		}
		if metrics.SuccessfulOps != 1 {
			t.Errorf("Expected 1 successful operation in metrics")
		}
		if metrics.FailedOps != 1 {
			t.Errorf("Expected 1 failed operation in metrics")
		}
	})
}

func BenchmarkRecordOperation(b *testing.B) {
	mc := NewMetricsCollector()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mc.RecordOperation(time.Millisecond, true, 100)
	}
}

func BenchmarkConcurrentMetrics(b *testing.B) {
	mc := NewMetricsCollector()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			mc.RecordOperation(time.Millisecond, true, 100)
			mc.RecordCacheHit()
		}
	})
}
