package internal

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// mockConfig implements ConfigInterface for testing
type mockConfig struct {
	cacheEnabled bool
	maxCacheSize int
	cacheTTL     time.Duration
}

func (m *mockConfig) IsCacheEnabled() bool         { return m.cacheEnabled }
func (m *mockConfig) GetMaxCacheSize() int         { return m.maxCacheSize }
func (m *mockConfig) GetCacheTTL() time.Duration   { return m.cacheTTL }
func (m *mockConfig) GetMaxJSONSize() int64        { return 10485760 }
func (m *mockConfig) GetMaxPathDepth() int         { return 100 }
func (m *mockConfig) GetMaxConcurrency() int       { return 10 }
func (m *mockConfig) IsMetricsEnabled() bool       { return false }
func (m *mockConfig) IsStrictMode() bool           { return false }
func (m *mockConfig) IsCommentsAllowed() bool      { return false }
func (m *mockConfig) ShouldPreserveNumbers() bool  { return false }
func (m *mockConfig) ShouldCreatePaths() bool      { return false }
func (m *mockConfig) ShouldCleanupNulls() bool     { return false }
func (m *mockConfig) ShouldCompactArrays() bool    { return false }
func (m *mockConfig) ShouldValidateInput() bool    { return false }
func (m *mockConfig) GetMaxNestingDepth() int      { return 100 }
func (m *mockConfig) ShouldValidateFilePath() bool { return false }

func TestCacheManager(t *testing.T) {
	t.Run("Creation", func(t *testing.T) {
		config := &mockConfig{cacheEnabled: true, maxCacheSize: 100}
		cm := NewCacheManager(config)
		if cm == nil {
			t.Fatal("NewCacheManager returned nil")
		}
		if cm.shardCount == 0 {
			t.Error("Cache manager should have shards")
		}
	})

	t.Run("BasicSetGet", func(t *testing.T) {
		config := &mockConfig{cacheEnabled: true, maxCacheSize: 100}
		cm := NewCacheManager(config)

		key := "test_key"
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
		config := &mockConfig{cacheEnabled: true, maxCacheSize: 100}
		cm := NewCacheManager(config)

		_, found := cm.Get("nonexistent_key")
		if found {
			t.Error("Should not find nonexistent key")
		}

		missCount := atomic.LoadInt64(&cm.missCount)
		if missCount == 0 {
			t.Error("Miss count should be incremented")
		}
	})

	t.Run("CacheHit", func(t *testing.T) {
		config := &mockConfig{cacheEnabled: true, maxCacheSize: 100}
		cm := NewCacheManager(config)

		cm.Set("key", "value")
		cm.Get("key")

		hitCount := atomic.LoadInt64(&cm.hitCount)
		if hitCount == 0 {
			t.Error("Hit count should be incremented")
		}
	})

	t.Run("TTLExpiration", func(t *testing.T) {
		config := &mockConfig{
			cacheEnabled: true,
			maxCacheSize: 100,
			cacheTTL:     50 * time.Millisecond,
		}
		cm := NewCacheManager(config)

		cm.Set("key", "value")

		// Should be found immediately
		_, found := cm.Get("key")
		if !found {
			t.Error("Value should be found before TTL expires")
		}

		// Wait for TTL to expire
		time.Sleep(100 * time.Millisecond)

		// Should not be found after TTL
		_, found = cm.Get("key")
		if found {
			t.Error("Value should not be found after TTL expires")
		}
	})

	t.Run("ConcurrentAccess", func(t *testing.T) {
		config := &mockConfig{cacheEnabled: true, maxCacheSize: 1000}
		cm := NewCacheManager(config)

		var wg sync.WaitGroup
		workers := 10
		operations := 100

		// Concurrent writes
		wg.Add(workers)
		for i := 0; i < workers; i++ {
			go func(workerID int) {
				defer wg.Done()
				for j := 0; j < operations; j++ {
					key := "key_" + string(rune(workerID*operations+j))
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
					key := "key_" + string(rune(workerID*operations+j))
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
		config := &mockConfig{cacheEnabled: false, maxCacheSize: 100}
		cm := NewCacheManager(config)

		cm.Set("key", "value")
		_, found := cm.Get("key")

		if found {
			t.Error("Disabled cache should not store values")
		}
	})

	t.Run("MultipleValues", func(t *testing.T) {
		config := &mockConfig{cacheEnabled: true, maxCacheSize: 100}
		cm := NewCacheManager(config)

		testData := map[string]any{
			"string": "test",
			"int":    42,
			"float":  3.14,
			"bool":   true,
			"nil":    nil,
		}

		for k, v := range testData {
			cm.Set(k, v)
		}

		for k, expected := range testData {
			retrieved, found := cm.Get(k)
			if !found {
				t.Errorf("Key %s should be found", k)
			}
			if retrieved != expected {
				t.Errorf("Key %s: expected %v, got %v", k, expected, retrieved)
			}
		}
	})

	t.Run("Sharding", func(t *testing.T) {
		config := &mockConfig{cacheEnabled: true, maxCacheSize: 10000}
		cm := NewCacheManager(config)

		if cm.shardCount < 2 {
			t.Error("Large cache should use multiple shards")
		}

		// Verify different keys go to different shards
		key1 := "key1"
		key2 := "key2"

		shard1 := cm.getShard(key1)
		shard2 := cm.getShard(key2)

		// Not guaranteed to be different, but with enough shards likely
		if shard1 == shard2 {
			t.Log("Keys happened to map to same shard (acceptable)")
		}
	})

	t.Run("NilConfig", func(t *testing.T) {
		cm := NewCacheManager(nil)
		if cm == nil {
			t.Fatal("Should handle nil config")
		}

		cm.Set("key", "value")
		_, found := cm.Get("key")
		if found {
			t.Error("Nil config should disable caching")
		}
	})
}

func TestCacheEntry(t *testing.T) {
	t.Run("AccessTracking", func(t *testing.T) {
		config := &mockConfig{cacheEnabled: true, maxCacheSize: 100}
		cm := NewCacheManager(config)

		cm.Set("key", "value")

		// Access multiple times
		for i := 0; i < 5; i++ {
			cm.Get("key")
		}

		// Verify hit count increased
		hitCount := atomic.LoadInt64(&cm.hitCount)
		if hitCount != 5 {
			t.Errorf("Expected 5 hits, got %d", hitCount)
		}
	})
}

func BenchmarkCacheGet(b *testing.B) {
	config := &mockConfig{cacheEnabled: true, maxCacheSize: 1000}
	cm := NewCacheManager(config)

	cm.Set("benchmark_key", "benchmark_value")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cm.Get("benchmark_key")
	}
}

func BenchmarkCacheSet(b *testing.B) {
	config := &mockConfig{cacheEnabled: true, maxCacheSize: 10000}
	cm := NewCacheManager(config)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cm.Set("key", i)
	}
}

func BenchmarkCacheConcurrent(b *testing.B) {
	config := &mockConfig{cacheEnabled: true, maxCacheSize: 10000}
	cm := NewCacheManager(config)

	// Pre-populate
	for i := 0; i < 100; i++ {
		cm.Set("key_"+string(rune(i)), i)
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			key := "key_" + string(rune(i%100))
			cm.Get(key)
			i++
		}
	})
}

// ============================================================================
// ADDITIONAL CACHE TESTS FOR COVERAGE
// ============================================================================

func TestCacheManager_Delete(t *testing.T) {
	t.Run("delete existing key", func(t *testing.T) {
		config := &mockConfig{cacheEnabled: true, maxCacheSize: 100}
		cm := NewCacheManager(config)

		cm.Set("key", "value")
		_, found := cm.Get("key")
		if !found {
			t.Fatal("Value should be found before delete")
		}

		cm.Delete("key")
		_, found = cm.Get("key")
		if found {
			t.Error("Value should not be found after delete")
		}
	})

	t.Run("delete non-existent key", func(t *testing.T) {
		config := &mockConfig{cacheEnabled: true, maxCacheSize: 100}
		cm := NewCacheManager(config)

		// Should not panic
		cm.Delete("nonexistent_key")
	})
}

func TestCacheManager_Clear(t *testing.T) {
	config := &mockConfig{cacheEnabled: true, maxCacheSize: 100}
	cm := NewCacheManager(config)

	// Add multiple entries
	cm.Set("key1", "value1")
	cm.Set("key2", "value2")
	cm.Set("key3", "value3")

	// Clear the cache
	cm.Clear()

	// Verify all entries are gone
	_, found1 := cm.Get("key1")
	_, found2 := cm.Get("key2")
	_, found3 := cm.Get("key3")

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
	config := &mockConfig{
		cacheEnabled: true,
		maxCacheSize: 100,
		cacheTTL:     50 * time.Millisecond,
	}
	cm := NewCacheManager(config)

	cm.Set("key1", "value1")
	cm.Set("key2", "value2")

	// Wait for TTL to expire
	time.Sleep(100 * time.Millisecond)

	// Clean expired entries
	cm.CleanExpiredCache()

	// Give some time for goroutines to complete
	time.Sleep(50 * time.Millisecond)

	// Entries should be expired
	_, found := cm.Get("key1")
	if found {
		t.Error("Entry should be expired after CleanExpiredCache")
	}
}

func TestCacheManager_GetStats(t *testing.T) {
	config := &mockConfig{cacheEnabled: true, maxCacheSize: 100}
	cm := NewCacheManager(config)

	// Add entries and access them
	cm.Set("key1", "value1")
	cm.Set("key2", "value2")
	cm.Get("key1")        // hit
	cm.Get("key1")        // hit
	cm.Get("nonexistent") // miss

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
	config := &mockConfig{cacheEnabled: true, maxCacheSize: 5}
	cm := NewCacheManager(config)

	// Add more entries than max size to trigger eviction
	for i := 0; i < 10; i++ {
		cm.Set(string(rune('a'+i)), i)
	}

	// Some entries should have been evicted
	stats := cm.GetStats()
	if stats.Entries > 5 {
		t.Errorf("Entries = %d, should be <= 5 due to eviction", stats.Entries)
	}
}

func TestCacheManager_LargeKey(t *testing.T) {
	config := &mockConfig{cacheEnabled: true, maxCacheSize: 100}
	cm := NewCacheManager(config)

	// Create a key longer than maxCacheKeyLength
	largeKey := ""
	for i := 0; i < 1500; i++ {
		largeKey += "a"
	}

	// Should not panic with large key - the key is truncated internally
	cm.Set(largeKey, "value")

	// Note: Due to key truncation, the original large key won't match
	// We're just testing that it doesn't panic and can handle large keys
}

func TestCacheManager_VariousTypes(t *testing.T) {
	config := &mockConfig{cacheEnabled: true, maxCacheSize: 100}
	cm := NewCacheManager(config)

	// Test simple comparable types
	t.Run("simple types", func(t *testing.T) {
		tests := []struct {
			name  string
			key   string
			value any
		}{
			{"string", "str_key", "test_string"},
			{"int", "int_key", 42},
			{"float", "float_key", 3.14159},
			{"bool", "bool_key", true},
			{"nil", "nil_key", nil},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				cm.Set(tt.key, tt.value)
				retrieved, found := cm.Get(tt.key)
				if !found {
					t.Errorf("Key %s should be found", tt.key)
				}
				if tt.value != nil && retrieved != tt.value {
					t.Errorf("Value mismatch for %s", tt.key)
				}
			})
		}
	})

	// Test complex types (just verify they can be stored and retrieved)
	t.Run("complex types", func(t *testing.T) {
		// Slice
		cm.Set("slice_key", []any{1, 2, 3})
		retrieved, found := cm.Get("slice_key")
		if !found {
			t.Error("slice_key should be found")
		}
		if slice, ok := retrieved.([]any); !ok || len(slice) != 3 {
			t.Error("slice value type or length mismatch")
		}

		// Map
		cm.Set("map_key", map[string]any{"a": 1})
		retrieved, found = cm.Get("map_key")
		if !found {
			t.Error("map_key should be found")
		}
		if m, ok := retrieved.(map[string]any); !ok || m["a"] != 1 {
			t.Error("map value type or content mismatch")
		}

		// Bytes
		cm.Set("bytes_key", []byte("test"))
		retrieved, found = cm.Get("bytes_key")
		if !found {
			t.Error("bytes_key should be found")
		}
		if b, ok := retrieved.([]byte); !ok || string(b) != "test" {
			t.Error("bytes value type or content mismatch")
		}

		// PathSegments
		cm.Set("path_key", []PathSegment{NewPropertySegment("test")})
		retrieved, found = cm.Get("path_key")
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

func TestTruncateCacheKey(t *testing.T) {
	t.Run("short key unchanged", func(t *testing.T) {
		key := "short_key"
		result := truncateCacheKey(key)
		if result != key {
			t.Errorf("Short key should not be truncated")
		}
	})

	t.Run("long key truncated", func(t *testing.T) {
		// Create a key longer than maxCacheKeyLength
		longKey := ""
		for i := 0; i < 1500; i++ {
			longKey += "a"
		}

		result := truncateCacheKey(longKey)
		if len(result) > maxCacheKeyLength {
			t.Errorf("Truncated key length %d > max %d", len(result), maxCacheKeyLength)
		}
		// Should contain "..." separator
		if len(result) > 0 && len(longKey) > maxCacheKeyLength {
			// Verify the key was modified
			if result == longKey {
				t.Error("Long key should be truncated")
			}
		}
	})
}
