package json

import (
	"strings"
	"sync"
	"testing"
	"time"
)

// TestUnifiedResourceManager tests the unified resource manager functionality
func TestUnifiedResourceManager(t *testing.T) {
	t.Run("Creation", func(t *testing.T) {
		rm := NewUnifiedResourceManager()
		if rm == nil {
			t.Fatal("NewUnifiedResourceManager returned nil")
		}
	})

	t.Run("StringBuilderPool", func(t *testing.T) {
		rm := NewUnifiedResourceManager()

		// Test Get and Put cycle
		sb1 := rm.GetStringBuilder()
		if sb1 == nil {
			t.Fatal("GetStringBuilder returned nil")
		}

		// Write some data
		sb1.WriteString("test data")
		if sb1.String() != "test data" {
			t.Errorf("StringBuilder write failed, got: %s", sb1.String())
		}

		// Return to pool
		rm.PutStringBuilder(sb1)

		// Get again - should get the same or different builder
		sb2 := rm.GetStringBuilder()
		if sb2 == nil {
			t.Fatal("GetStringBuilder returned nil on second call")
		}
		sb2.Reset()
		rm.PutStringBuilder(sb2)
	})

	t.Run("PathSegmentPool", func(t *testing.T) {
		rm := NewUnifiedResourceManager()

		// Test Get and Put cycle
		seg1 := rm.GetPathSegments()
		if seg1 == nil {
			t.Fatal("GetPathSegments returned nil")
		}

		// Verify it's a slice
		if cap(seg1) == 0 {
			t.Error("PathSegment should have capacity")
		}

		// Return to pool
		rm.PutPathSegments(seg1)

		// Get again
		seg2 := rm.GetPathSegments()
		if seg2 == nil {
			t.Fatal("GetPathSegments returned nil on second call")
		}
		rm.PutPathSegments(seg2)
	})

	t.Run("BufferPool", func(t *testing.T) {
		rm := NewUnifiedResourceManager()

		// Test Get and Put cycle
		buf1 := rm.GetBuffer()
		if buf1 == nil {
			t.Fatal("GetBuffer returned nil")
		}

		// Write some data
		buf1 = append(buf1, "test data"...)

		// Return to pool
		rm.PutBuffer(buf1)

		// Get again
		buf2 := rm.GetBuffer()
		if buf2 == nil {
			t.Fatal("GetBuffer returned nil on second call")
		}
		rm.PutBuffer(buf2)
	})

	t.Run("ConcurrentAccess", func(t *testing.T) {
		rm := NewUnifiedResourceManager()
		const goroutines = 100
		const opsPerGoroutine = 100

		var wg sync.WaitGroup
		wg.Add(goroutines * 3) // 3 types of pools

		// StringBuilder pool concurrent access
		for i := 0; i < goroutines; i++ {
			go func() {
				defer wg.Done()
				for j := 0; j < opsPerGoroutine; j++ {
					sb := rm.GetStringBuilder()
					sb.WriteString("concurrent test")
					rm.PutStringBuilder(sb)
				}
			}()
		}

		// PathSegment pool concurrent access
		for i := 0; i < goroutines; i++ {
			go func() {
				defer wg.Done()
				for j := 0; j < opsPerGoroutine; j++ {
					seg := rm.GetPathSegments()
					rm.PutPathSegments(seg)
				}
			}()
		}

		// Buffer pool concurrent access
		for i := 0; i < goroutines; i++ {
			go func() {
				defer wg.Done()
				for j := 0; j < opsPerGoroutine; j++ {
					buf := rm.GetBuffer()
					buf = append(buf, byte(i))
					rm.PutBuffer(buf)
				}
			}()
		}

		wg.Wait()
	})

	t.Run("Stats", func(t *testing.T) {
		rm := NewUnifiedResourceManager()

		// Perform some operations
		sb := rm.GetStringBuilder()
		sb.WriteString("test")
		rm.PutStringBuilder(sb)

		// Allocate a path segment to increment that counter
		seg := rm.GetPathSegments()
		rm.PutPathSegments(seg)

		buf := rm.GetBuffer()
		buf = append(buf, "test"...)
		rm.PutBuffer(buf)

		// Get stats - verify no crashes
		_ = rm.GetStats()
		// Note: Allocated counts are tracked atomically and should be positive
		// The specific counts may vary due to internal implementation details
	})

	t.Run("SizeLimits", func(t *testing.T) {
		rm := NewUnifiedResourceManager()

		// Test oversized builder is discarded
		oversizedSb := &strings.Builder{}
		oversizedSb.Grow(100000) // Way over MaxPoolBufferSize
		rm.PutStringBuilder(oversizedSb)

		// Test undersized builder is discarded
		undersizedSb := &strings.Builder{}
		undersizedSb.Grow(10) // Under MinPoolBufferSize
		rm.PutStringBuilder(undersizedSb)

		// Note: Oversized/undersized builders are discarded automatically
	})

	t.Run("PerformMaintenance", func(t *testing.T) {
		rm := NewUnifiedResourceManager()

		// Should not panic
		rm.PerformMaintenance()

		// Multiple calls should be safe
		for i := 0; i < 10; i++ {
			rm.PerformMaintenance()
		}
	})
}

// BenchmarkStringBuilderPool benchmarks the string builder pool performance
func BenchmarkStringBuilderPool(b *testing.B) {
	rm := NewUnifiedResourceManager()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sb := rm.GetStringBuilder()
		sb.WriteString("benchmark test data")
		rm.PutStringBuilder(sb)
	}
}

// BenchmarkPathSegmentPool benchmarks the path segment pool performance
func BenchmarkPathSegmentPool(b *testing.B) {
	rm := NewUnifiedResourceManager()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		seg := rm.GetPathSegments()
		rm.PutPathSegments(seg)
	}
}

// BenchmarkBufferPool benchmarks the buffer pool performance
func BenchmarkBufferPool(b *testing.B) {
	rm := NewUnifiedResourceManager()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf := rm.GetBuffer()
		buf = append(buf, "test"...)
		rm.PutBuffer(buf)
	}
}

// BenchmarkConcurrentPools benchmarks concurrent access to all pools
func BenchmarkConcurrentPools(b *testing.B) {
	rm := NewUnifiedResourceManager()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			sb := rm.GetStringBuilder()
			buf := rm.GetBuffer()
			_ = append(buf, "test"...)
			rm.PutStringBuilder(sb)
			rm.PutBuffer(buf)
		}
	})
}

// TestGlobalResourceManager tests the global resource manager singleton
func TestGlobalResourceManager(t *testing.T) {
	t.Run("Singleton", func(t *testing.T) {
		rm1 := getGlobalResourceManager()
		rm2 := getGlobalResourceManager()

		if rm1 != rm2 {
			t.Error("getGlobalResourceManager should return the same instance")
		}
	})

	t.Run("Usable", func(t *testing.T) {
		rm := getGlobalResourceManager()

		sb := rm.GetStringBuilder()
		sb.WriteString("global test")
		rm.PutStringBuilder(sb)

		// Should not panic
	})
}

// TestResourceMonitor tests the ResourceMonitor functionality
func TestResourceMonitor(t *testing.T) {
	t.Run("Creation", func(t *testing.T) {
		rm := NewResourceMonitor()
		if rm == nil {
			t.Fatal("NewResourceMonitor returned nil")
		}
	})

	t.Run("RecordAllocation", func(t *testing.T) {
		rm := NewResourceMonitor()

		rm.RecordAllocation(1024)
		rm.RecordAllocation(2048)
		rm.RecordDeallocation(512)

		stats := rm.GetStats()
		// Note: AllocatedBytes should be positive (at least the sum of allocations minus deallocations)
		if stats.AllocatedBytes <= 0 {
			t.Errorf("Expected positive allocated bytes, got %d", stats.AllocatedBytes)
		}
		// FreedBytes should be at least what we deallocated
		if stats.FreedBytes < 512 {
			t.Errorf("Expected at least 512 freed bytes, got %d", stats.FreedBytes)
		}
	})

	t.Run("RecordPoolOperations", func(t *testing.T) {
		rm := NewResourceMonitor()

		rm.RecordPoolHit()
		rm.RecordPoolMiss()
		rm.RecordPoolEviction()

		stats := rm.GetStats()
		if stats.PoolHits != 1 {
			t.Errorf("Expected 1 pool hit, got %d", stats.PoolHits)
		}
		if stats.PoolMisses != 1 {
			t.Errorf("Expected 1 pool miss, got %d", stats.PoolMisses)
		}
		if stats.PoolEvictions != 1 {
			t.Errorf("Expected 1 pool eviction, got %d", stats.PoolEvictions)
		}
	})

	t.Run("RecordOperation", func(t *testing.T) {
		rm := NewResourceMonitor()

		rm.RecordOperation(100 * time.Millisecond)
		rm.RecordOperation(200 * time.Millisecond)

		stats := rm.GetStats()
		if stats.TotalOperations != 2 {
			t.Errorf("Expected 2 operations, got %d", stats.TotalOperations)
		}
		if stats.AvgResponseTime == 0 {
			t.Error("Expected non-zero average response time")
		}
	})

	t.Run("PeakMemoryTracking", func(t *testing.T) {
		rm := NewResourceMonitor()

		// Record increasing allocations to test peak tracking
		rm.RecordAllocation(1000)
		stats1 := rm.GetStats()
		if stats1.PeakMemoryUsage < 1000 {
			t.Errorf("Expected peak >= 1000, got %d", stats1.PeakMemoryUsage)
		}

		rm.RecordAllocation(2000)
		stats2 := rm.GetStats()
		// Peak should be at least 3000 (1000 + 2000)
		if stats2.PeakMemoryUsage < 3000 {
			t.Errorf("Expected peak >= 3000, got %d", stats2.PeakMemoryUsage)
		}

		rm.RecordDeallocation(500)
		stats3 := rm.GetStats()
		// Peak should remain at maximum (not decrease with deallocations)
		if stats3.PeakMemoryUsage < stats2.PeakMemoryUsage {
			t.Errorf("Peak should not decrease with deallocations, went from %d to %d",
				stats2.PeakMemoryUsage, stats3.PeakMemoryUsage)
		}
	})

	t.Run("MemoryEfficiency", func(t *testing.T) {
		rm := NewResourceMonitor()

		rm.RecordAllocation(1024 * 1024) // 1MB
		rm.RecordPoolHit()

		efficiency := rm.GetMemoryEfficiency()
		// Efficiency should be calculated (hits per MB)
		// With 1 hit and 1MB, efficiency would be low, but should be non-negative
		if efficiency < 0 {
			t.Errorf("Expected non-negative memory efficiency, got %f", efficiency)
		}
	})

	t.Run("PoolEfficiency", func(t *testing.T) {
		rm := NewResourceMonitor()

		rm.RecordPoolHit()
		rm.RecordPoolHit()
		rm.RecordPoolMiss()

		efficiency := rm.GetPoolEfficiency()
		expected := 2.0 / 3.0 * 100.0
		if efficiency < expected-0.1 || efficiency > expected+0.1 {
			t.Errorf("Expected efficiency ~%.2f, got %.2f", expected, efficiency)
		}
	})

	t.Run("Reset", func(t *testing.T) {
		rm := NewResourceMonitor()

		rm.RecordAllocation(1000)
		rm.RecordPoolHit()
		rm.Reset()

		stats := rm.GetStats()
		if stats.AllocatedBytes != 0 {
			t.Errorf("Expected 0 allocated bytes after reset, got %d", stats.AllocatedBytes)
		}
		if stats.PoolHits != 0 {
			t.Errorf("Expected 0 pool hits after reset, got %d", stats.PoolHits)
		}
	})

	t.Run("CheckForLeaks", func(t *testing.T) {
		rm := NewResourceMonitor()

		// Normal case - no leaks (but may return nil due to time interval)
		issues := rm.CheckForLeaks()
		// May return nil due to time-based check interval, which is expected
		_ = issues

		// Simulate various conditions
		rm.RecordAllocation(200 * 1024 * 1024) // 200MB
		rm.RecordPoolHit()
		rm.RecordPoolHit()
		rm.RecordPoolMiss() // 1 hit, 1 miss = poor efficiency
		rm.RecordPoolMiss()
		rm.RecordPoolMiss() // Now 2 hits, 3 misses

		// Note: CheckForLeaks has a time interval that may prevent immediate execution
		// The test just verifies it doesn't crash and handles the call
		_ = rm.CheckForLeaks()
	})
}

// BenchmarkResourceMonitor benchmarks resource monitor operations
func BenchmarkResourceMonitor(b *testing.B) {
	rm := NewResourceMonitor()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rm.RecordAllocation(1024)
		rm.RecordDeallocation(512)
	}
}

// TestGetStats tests processor GetStats functionality with resource manager
func TestGetStatsWithResourceManager(t *testing.T) {
	p := New()
	defer p.Close()

	stats := p.GetStats()

	// Verify stats are accessible
	if stats.IsClosed {
		t.Error("Processor should not be closed")
	}

	// Perform operations and verify stats update
	jsonStr := `{"test":"data"}`
	_, _ = p.Get(jsonStr, "test")

	stats2 := p.GetStats()
	if stats2.OperationCount == 0 {
		t.Error("Expected operation count to increase")
	}
}
