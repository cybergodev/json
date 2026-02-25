package json

import (
	"bytes"
	"strings"
	"testing"

	"github.com/cybergodev/json/internal"
)

// ============================================================================
// PERFORMANCE BENCHMARK TESTS
// Tests for new optimized functions
// ============================================================================

// Benchmark FastSet vs Set
func BenchmarkFastSet_Simple(b *testing.B) {
	processor := New()
	defer processor.Close()

	jsonStr := `{"name":"test","age":30,"active":true}`
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, _ = processor.FastSet(jsonStr, "name", "updated")
	}
}

func BenchmarkSet_Simple(b *testing.B) {
	processor := New()
	defer processor.Close()

	jsonStr := `{"name":"test","age":30,"active":true}`
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, _ = processor.Set(jsonStr, "name", "updated")
	}
}

// Benchmark FastDelete vs Delete
func BenchmarkFastDelete_Simple(b *testing.B) {
	processor := New()
	defer processor.Close()

	jsonStr := `{"name":"test","age":30,"active":true}`
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, _ = processor.FastDelete(jsonStr, "name")
	}
}

func BenchmarkDelete_Simple(b *testing.B) {
	processor := New()
	defer processor.Close()

	jsonStr := `{"name":"test","age":30,"active":true}`
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, _ = processor.Delete(jsonStr, "name")
	}
}

// Benchmark BatchSetOptimized
func BenchmarkBatchSetOptimized(b *testing.B) {
	processor := New()
	defer processor.Close()

	jsonStr := `{"a":1,"b":2,"c":3,"d":4,"e":5}`
	updates := map[string]any{
		"a": 10,
		"b": 20,
		"c": 30,
	}
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, _ = processor.BatchSetOptimized(jsonStr, updates)
	}
}

// Benchmark FastGetMultiple
func BenchmarkFastGetMultiple(b *testing.B) {
	processor := New()
	defer processor.Close()

	jsonStr := `{"a":1,"b":2,"c":3,"d":4,"e":5}`
	paths := []string{"a", "b", "c", "d", "e"}
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, _ = processor.FastGetMultiple(jsonStr, paths)
	}
}

// Benchmark StreamIterator
func BenchmarkStreamIterator(b *testing.B) {
	// Create a large JSON array
	var sb strings.Builder
	sb.WriteString("[")
	for i := 0; i < 1000; i++ {
		if i > 0 {
			sb.WriteString(",")
		}
		sb.WriteString(`{"id":`)
		sb.WriteString(strings.Repeat("0", 3))
		sb.WriteString("}")
	}
	sb.WriteString("]")
	jsonData := sb.String()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		reader := strings.NewReader(jsonData)
		it := NewStreamIterator(reader)
		count := 0
		for it.Next() {
			count++
		}
	}
}

// Benchmark StreamObjectIterator
func BenchmarkStreamObjectIterator(b *testing.B) {
	// Create a large JSON object
	var sb strings.Builder
	sb.WriteString("{")
	for i := 0; i < 100; i++ {
		if i > 0 {
			sb.WriteString(",")
		}
		sb.WriteString(`"key`)
		sb.WriteString(strings.Repeat("0", 2))
		sb.WriteString(`":"value"`)
	}
	sb.WriteString("}")
	jsonData := sb.String()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		reader := strings.NewReader(jsonData)
		it := NewStreamObjectIterator(reader)
		count := 0
		for it.Next() {
			count++
		}
	}
}

// Benchmark PooledSliceIterator vs regular iteration
func BenchmarkPooledSliceIterator(b *testing.B) {
	data := make([]any, 1000)
	for i := range data {
		data[i] = i
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		it := NewPooledSliceIterator(data)
		for it.Next() {
			_ = it.Value()
		}
		it.Release()
	}
}

func BenchmarkRegularSliceIteration(b *testing.B) {
	data := make([]any, 1000)
	for i := range data {
		data[i] = i
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, v := range data {
			_ = v
		}
	}
}

// Benchmark PooledMapIterator
func BenchmarkPooledMapIterator(b *testing.B) {
	data := make(map[string]any, 100)
	for i := 0; i < 100; i++ {
		data[string(rune('a'+i%26))+string(rune('a'+i/26))] = i
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		it := NewPooledMapIterator(data)
		for it.Next() {
			_, _ = it.Key(), it.Value()
		}
		it.Release()
	}
}

func BenchmarkRegularMapIteration(b *testing.B) {
	data := make(map[string]any, 100)
	for i := 0; i < 100; i++ {
		data[string(rune('a'+i%26))+string(rune('a'+i/26))] = i
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for k, v := range data {
			_, _ = k, v
		}
	}
}

// Benchmark Path Segment Cache
func BenchmarkPathSegmentCache(b *testing.B) {
	cache := getPathCache()
	path := "users.profile.settings"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, ok := cache.Get(path); !ok {
			segments, _ := internal.ParsePath(path)
			cache.Set(path, segments)
		}
	}
}

// Benchmark Large Buffer Pool
func BenchmarkLargeBufferPool(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf := getLargeBuffer()
		*buf = append(*buf, make([]byte, 1024)...)
		putLargeBuffer(buf)
	}
}

// Benchmark Encode Buffer Pool
func BenchmarkEncodeBufferPool(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf := GetEncodeBuffer()
		buf = append(buf, make([]byte, 512)...)
		PutEncodeBuffer(buf)
	}
}

// Benchmark isSimplePropertyAccess
func BenchmarkIsSimplePropertyAccess(b *testing.B) {
	paths := []string{
		"name",
		"user",
		"profile",
		"settings",
		"data123",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, p := range paths {
			_ = isSimplePropertyAccess(p)
		}
	}
}

// Benchmark StreamingProcessor
func BenchmarkStreamingProcessor_Array(b *testing.B) {
	// Create JSON array data
	var buf bytes.Buffer
	buf.WriteString("[")
	for i := 0; i < 1000; i++ {
		if i > 0 {
			buf.WriteString(",")
		}
		buf.WriteString(`{"id":`)
		buf.WriteString(strings.Repeat("0", 3))
		buf.WriteString(`}`)
	}
	buf.WriteString("]")
	data := buf.Bytes()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sp := NewStreamingProcessor(bytes.NewReader(data), 0)
		_ = sp.StreamArray(func(index int, item any) bool {
			return true // Continue
		})
	}
}
