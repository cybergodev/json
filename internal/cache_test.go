package internal

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// testKey builds a CacheKey from a plain string, hashing the string into
// JSONHash so distinct strings stay distinct keys (the shape the previous
// string-key API gave tests for free).
func testKey(s string) CacheKey {
	return CacheKey{JSONHash: HashStringFNV1a(s), Path: s}
}

func TestCacheManager(t *testing.T) {
	t.Run("Creation", func(t *testing.T) {
		cm := NewCacheManager(true, 100, 0)
		if cm == nil {
			t.Fatal("NewCacheManager returned nil")
		}
		if cm.shardCount == 0 {
			t.Error("Cache manager should have shards")
		}
	})

	t.Run("BasicSetGet", func(t *testing.T) {
		cm := NewCacheManager(true, 100, 0)

		key := testKey("test_key")
		value := "test_value"

		cm.Set(key, value)
		retrieved, found := cm.Get(key)

		if !found {
			t.Error("Value should be found in cache")
		}
		if retrieved != value {
			t.Errorf("Expected %v, got %v", value, retrieved)
		}
	})

	t.Run("CacheMiss", func(t *testing.T) {
		cm := NewCacheManager(true, 100, 0)

		_, found := cm.Get(testKey("nonexistent_key"))
		if found {
			t.Error("Should not find nonexistent key")
		}

		missCount := atomic.LoadInt64(&cm.missCount)
		if missCount == 0 {
			t.Error("Miss count should be incremented")
		}
	})

	t.Run("CacheHit", func(t *testing.T) {
		cm := NewCacheManager(true, 100, 0)

		cm.Set(testKey("key"), "value")
		cm.Get(testKey("key"))

		hitCount := atomic.LoadInt64(&cm.hitCount)
		if hitCount == 0 {
			t.Error("Hit count should be incremented")
		}
	})

	t.Run("TTLExpiration", func(t *testing.T) {
		cm := NewCacheManager(true, 100, 50*time.Millisecond)

		cm.Set(testKey("key"), "value")

		// Should be found immediately
		_, found := cm.Get(testKey("key"))
		if !found {
			t.Error("Value should be found before TTL expires")
		}

		// Wait for TTL to expire
		time.Sleep(100 * time.Millisecond)

		// Should not be found after TTL
		_, found = cm.Get(testKey("key"))
		if found {
			t.Error("Value should not be found after TTL expires")
		}
	})

	t.Run("ConcurrentAccess", func(t *testing.T) {
		cm := NewCacheManager(true, 1000, 0)

		var wg sync.WaitGroup
		workers := 10
		operations := 100

		// Concurrent writes
		wg.Add(workers)
		for i := 0; i < workers; i++ {
			go func(workerID int) {
				defer wg.Done()
				for j := 0; j < operations; j++ {
					key := testKey(fmt.Sprintf("key_%d", workerID*operations+j))
					cm.Set(key, workerID*operations+j)
				}
			}(i)
		}
		wg.Wait()

		// Concurrent reads
		wg.Add(workers)
		for i := 0; i < workers; i++ {
			go func(workerID int) {
				defer wg.Done()
				for j := 0; j < operations; j++ {
					key := testKey(fmt.Sprintf("key_%d", workerID*operations+j))
					cm.Get(key)
				}
			}(i)
		}
		wg.Wait()

		totalOps := int64(workers * operations)
		hitCount := atomic.LoadInt64(&cm.hitCount)
		if hitCount == 0 {
			t.Error("Should have cache hits from concurrent access")
		}
		if hitCount > totalOps {
			t.Errorf("Hit count %d exceeds total operations %d", hitCount, totalOps)
		}
	})

	t.Run("DisabledCache", func(t *testing.T) {
		cm := NewCacheManager(false, 100, 0)

		cm.Set(testKey("key"), "value")
		_, found := cm.Get(testKey("key"))

		if found {
			t.Error("Disabled cache should not store values")
		}
	})

	t.Run("MultipleValues", func(t *testing.T) {
		cm := NewCacheManager(true, 100, 0)

		testData := map[CacheKey]any{
			testKey("string"): "test",
			testKey("int"):    42,
			testKey("float"):  3.14,
			testKey("bool"):   true,
			testKey("nil"):    nil,
		}

		for k, v := range testData {
			cm.Set(k, v)
		}

		for k, expected := range testData {
			retrieved, found := cm.Get(k)
			if !found {
				t.Errorf("Key %s should be found", k.Path)
			}
			if retrieved != expected {
				t.Errorf("Key %s: expected %v, got %v", k.Path, expected, retrieved)
			}
		}
	})

	t.Run("Sharding", func(t *testing.T) {
		cm := NewCacheManager(true, 10000, 0)

		if cm.shardCount < 2 {
			t.Error("Large cache should use multiple shards")
		}

		// Verify different keys CAN go to different shards
		shard1 := cm.getShard(testKey("key1"))
		shard2 := cm.getShard(testKey("key2"))

		// Not guaranteed to be different, but with enough shards likely
		if shard1 == shard2 {
			t.Log("Keys happened to map to same shard (acceptable)")
		}
	})

	t.Run("NilConfig", func(t *testing.T) {
		cm := NewCacheManager(false, 0, 0)
		if cm == nil {
			t.Fatal("Should handle nil config")
		}

		cm.Set(testKey("key"), "value")
		_, found := cm.Get(testKey("key"))
		if found {
			t.Error("Nil config should disable caching")
		}
	})
}

func TestCacheEntry(t *testing.T) {
	t.Run("AccessTracking", func(t *testing.T) {
		cm := NewCacheManager(true, 100, 0)

		key := testKey("key")
		cm.Set(key, "value")

		// Access multiple times
		for i := 0; i < 5; i++ {
			cm.Get(key)
		}

		// Verify hit count increased
		hitCount := atomic.LoadInt64(&cm.hitCount)
		if hitCount != 5 {
			t.Errorf("Expected 5 hits, got %d", hitCount)
		}
	})
}

// TestCacheEntryCountAccuracy verifies the atomic entryCount maintained in
// CacheManager stays in lock-step with the authoritative per-shard size sum
// (GetStats().Entries) across every mutation path. entryCount drives the
// empty-cache fast-exit in DeleteByJSONHash, so a drift here would either miss
// the optimization (over-count) or, worse, skip invalidation of live entries
// (under-count → stale reads). Cross-checking against GetStats().Entries, which
// sums shard.size under read locks, catches any missed ++/-- site.
func TestCacheEntryCountAccuracy(t *testing.T) {
	// A small maxSize forces eviction, exercising the evictLRU decrement path.
	cm := NewCacheManager(true, 8, 0)

	assertCount := func(label string) {
		t.Helper()
		got := cm.EntryCount()
		want := cm.GetStats().Entries
		if got != want {
			t.Errorf("%s: EntryCount=%d but GetStats().Entries=%d", label, got, want)
		}
	}

	if cm.EntryCount() != 0 {
		t.Fatalf("fresh cache EntryCount=%d, want 0", cm.EntryCount())
	}

	// Set new entries.
	for i := 0; i < 5; i++ {
		cm.Set(testKey(fmt.Sprintf("k%d", i)), i)
	}
	assertCount("after 5 inserts")

	// Updating an existing key must NOT change the count.
	cm.Set(testKey("k0"), "updated")
	assertCount("after in-place update")

	// Inserting past maxSize triggers LRU eviction — count must still match.
	for i := 5; i < 20; i++ {
		cm.Set(testKey(fmt.Sprintf("k%d", i)), i)
	}
	assertCount("after eviction-inducing inserts")

	// DeleteByJSONHash on a populated cache removes every entry of that document.
	const docHash = uint64(0xdeadbeef)
	cm.Set(CacheKey{Op: "get", JSONHash: docHash, Path: "user"}, 1)
	cm.Set(CacheKey{Op: "parse", JSONHash: docHash}, 2)
	before := cm.EntryCount()
	cm.DeleteByJSONHash(docHash)
	if cm.EntryCount() >= before {
		t.Errorf("DeleteByJSONHash did not reduce count: before=%d after=%d", before, cm.EntryCount())
	}
	if _, ok := cm.Get(CacheKey{Op: "get", JSONHash: docHash, Path: "user"}); ok {
		t.Error("get entry of invalidated document survived")
	}
	if _, ok := cm.Get(CacheKey{Op: "parse", JSONHash: docHash}); ok {
		t.Error("parse entry of invalidated document survived")
	}
	assertCount("after DeleteByJSONHash (populated)")

	// Explicit Delete.
	cm.Delete(testKey("k0"))
	assertCount("after Delete")

	// Clear resets to zero.
	cm.Clear()
	if cm.EntryCount() != 0 {
		t.Errorf("after Clear EntryCount=%d, want 0", cm.EntryCount())
	}

	// DeleteByJSONHash on an empty cache must be a no-op (the fast-exit path)
	// and must not panic or alter the count.
	cm.DeleteByJSONHash(docHash)
	if cm.EntryCount() != 0 {
		t.Errorf("DeleteByJSONHash on empty cache changed count to %d", cm.EntryCount())
	}
}

// TestCacheEntryCountConcurrent hammers Set/Delete from many goroutines and
// confirms entryCount never drifts below zero and converges to the shard sum.
func TestCacheEntryCountConcurrent(t *testing.T) {
	cm := NewCacheManager(true, 10000, 0)
	var wg sync.WaitGroup
	for g := 0; g < 8; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for i := 0; i < 500; i++ {
				key := testKey(fmt.Sprintf("g%d-k%d", g, i))
				cm.Set(key, i)
				if i%3 == 0 {
					cm.Delete(key)
				}
			}
		}(g)
	}
	wg.Wait()

	if cm.EntryCount() < 0 {
		t.Fatalf("EntryCount went negative: %d", cm.EntryCount())
	}
	if got, want := cm.EntryCount(), cm.GetStats().Entries; got != want {
		t.Errorf("EntryCount=%d != GetStats().Entries=%d after concurrent ops", got, want)
	}
}

func BenchmarkCacheGet(b *testing.B) {
	cm := NewCacheManager(true, 1000, 0)

	key := testKey("benchmark_key")
	cm.Set(key, "benchmark_value")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cm.Get(key)
	}
}

func BenchmarkCacheSet(b *testing.B) {
	cm := NewCacheManager(true, 10000, 0)

	key := testKey("key")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cm.Set(key, i)
	}
}

func BenchmarkCacheConcurrent(b *testing.B) {
	cm := NewCacheManager(true, 10000, 0)

	keys := make([]CacheKey, 100)
	for i := range keys {
		keys[i] = testKey(fmt.Sprintf("key_%d", i))
		cm.Set(keys[i], i)
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			cm.Get(keys[i%100])
			i++
		}
	})
}

// ============================================================================
// ADDITIONAL CACHE TESTS FOR COVERAGE
// ============================================================================

func TestCacheManager_Delete(t *testing.T) {
	t.Run("delete existing key", func(t *testing.T) {
		cm := NewCacheManager(true, 100, 0)

		key := testKey("key")
		cm.Set(key, "value")
		if _, found := cm.Get(key); !found {
			t.Fatal("Value should be found before delete")
		}

		cm.Delete(key)
		if _, found := cm.Get(key); found {
			t.Error("Value should not be found after delete")
		}
	})

	t.Run("delete non-existent key", func(t *testing.T) {
		cm := NewCacheManager(true, 100, 0)

		// Should not panic
		cm.Delete(testKey("nonexistent_key"))
	})
}

func TestCacheManager_Clear(t *testing.T) {
	cm := NewCacheManager(true, 100, 0)

	// Add multiple entries
	cm.Set(testKey("key1"), "value1")
	cm.Set(testKey("key2"), "value2")
	cm.Set(testKey("key3"), "value3")

	// Clear the cache
	cm.Clear()

	// Verify all entries are gone
	_, found1 := cm.Get(testKey("key1"))
	_, found2 := cm.Get(testKey("key2"))
	_, found3 := cm.Get(testKey("key3"))

	if found1 || found2 || found3 {
		t.Error("All entries should be cleared")
	}

	// Verify entries are reset
	stats := cm.GetStats()
	if stats.Entries != 0 {
		t.Error("Entries should be 0 after clear")
	}
}

func TestCacheManager_CleanExpiredCache(t *testing.T) {
	cm := NewCacheManager(true, 100, 50*time.Millisecond)

	cm.Set(testKey("key1"), "value1")
	cm.Set(testKey("key2"), "value2")

	// Wait for TTL to expire
	time.Sleep(100 * time.Millisecond)

	// Clean expired entries
	cm.CleanExpiredCache()

	// Give some time for goroutines to complete
	time.Sleep(50 * time.Millisecond)

	// Entries should be expired
	if _, found := cm.Get(testKey("key1")); found {
		t.Error("Entry should be expired after CleanExpiredCache")
	}
}

func TestCacheManager_GetStats(t *testing.T) {
	cm := NewCacheManager(true, 100, 0)

	// Add entries and access them
	cm.Set(testKey("key1"), "value1")
	cm.Set(testKey("key2"), "value2")
	cm.Get(testKey("key1"))            // hit
	cm.Get(testKey("key1"))            // hit
	cm.Get(testKey("nonexistent_key")) // miss

	stats := cm.GetStats()

	if stats.HitCount != 2 {
		t.Errorf("HitCount = %d, want 2", stats.HitCount)
	}
	if stats.MissCount < 1 {
		t.Errorf("MissCount = %d, want at least 1", stats.MissCount)
	}
	if stats.ShardCount == 0 {
		t.Error("ShardCount should be positive")
	}
	if stats.Entries == 0 {
		t.Error("Entries should be positive")
	}
}

func TestCacheManager_Eviction(t *testing.T) {
	cm := NewCacheManager(true, 5, 0)

	// Add more entries than max size to trigger eviction
	for i := 0; i < 10; i++ {
		cm.Set(testKey(string(rune('a'+i))), i)
	}

	// Some entries should have been evicted
	stats := cm.GetStats()
	if stats.Entries > 5 {
		t.Errorf("Entries = %d, should be <= 5 due to eviction", stats.Entries)
	}
}

func TestCacheManager_VariousTypes(t *testing.T) {
	cm := NewCacheManager(true, 100, 0)

	// Test simple comparable types
	t.Run("simple types", func(t *testing.T) {
		tests := []struct {
			name  string
			key   CacheKey
			value any
		}{
			{"string", testKey("str_key"), "test_string"},
			{"int", testKey("int_key"), 42},
			{"float", testKey("float_key"), 3.14159},
			{"bool", testKey("bool_key"), true},
			{"nil", testKey("nil_key"), nil},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				cm.Set(tt.key, tt.value)
				retrieved, found := cm.Get(tt.key)
				if !found {
					t.Errorf("Key %s should be found", tt.key.Path)
				}
				if tt.value != nil && retrieved != tt.value {
					t.Errorf("Value mismatch for %s", tt.key.Path)
				}
			})
		}
	})

	// Test complex types (just verify they can be stored and retrieved)
	t.Run("complex types", func(t *testing.T) {
		// Slice
		cm.Set(testKey("slice_key"), []any{1, 2, 3})
		retrieved, found := cm.Get(testKey("slice_key"))
		if !found {
			t.Error("slice_key should be found")
		}
		if slice, ok := retrieved.([]any); !ok || len(slice) != 3 {
			t.Error("slice value type or length mismatch")
		}

		// Map
		cm.Set(testKey("map_key"), map[string]any{"a": 1})
		retrieved, found = cm.Get(testKey("map_key"))
		if !found {
			t.Error("map_key should be found")
		}
		if m, ok := retrieved.(map[string]any); !ok || m["a"] != 1 {
			t.Error("map value type or content mismatch")
		}

		// Bytes
		cm.Set(testKey("bytes_key"), []byte("test"))
		retrieved, found = cm.Get(testKey("bytes_key"))
		if !found {
			t.Error("bytes_key should be found")
		}
		if b, ok := retrieved.([]byte); !ok || string(b) != "test" {
			t.Error("bytes value type or content mismatch")
		}

		// PathSegments
		cm.Set(testKey("path_key"), []PathSegment{NewPropertySegment("test")})
		retrieved, found = cm.Get(testKey("path_key"))
		if !found {
			t.Error("path_key should be found")
		}
		if segs, ok := retrieved.([]PathSegment); !ok || len(segs) != 1 {
			t.Error("path segments value type or length mismatch")
		}
	})
}

func TestCalculateOptimalShardCount(t *testing.T) {
	tests := []struct {
		name     string
		maxSize  int
		minCount int
	}{
		{"very small", 50, 4},
		{"small", 500, 8},
		{"medium", 5000, 16},
		{"large", 50000, 32},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := calculateOptimalShardCount(tt.maxSize)
			if result < tt.minCount {
				t.Errorf("Shard count %d < minimum %d", result, tt.minCount)
			}
		})
	}
}

func TestNextPowerOf2(t *testing.T) {
	tests := []struct {
		input    int
		expected int
	}{
		{0, 1},
		{1, 1},
		{2, 2},
		{3, 4},
		{5, 8},
		{15, 16},
		{16, 16},
		{17, 32},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			result := nextPowerOf2(tt.input)
			if result != tt.expected {
				t.Errorf("nextPowerOf2(%d) = %d, want %d", tt.input, result, tt.expected)
			}
		})
	}
}
