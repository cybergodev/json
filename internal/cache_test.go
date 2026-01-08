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
func (m *mockConfig) AllowComments() bool          { return false }
func (m *mockConfig) PreserveNumbers() bool        { return false }
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
