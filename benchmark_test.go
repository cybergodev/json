package json

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/cybergodev/json/internal"
)

// ============================================================================
// COMPREHENSIVE PERFORMANCE BENCHMARKS
// Tests for all optimization areas
// ============================================================================

// ----------------------------------------------------------------------------
// FAST ENCODER BENCHMARKS
// ----------------------------------------------------------------------------

func BenchmarkFastEncoder_String(b *testing.B) {
	encoder := internal.GetEncoder()
	defer internal.PutEncoder(encoder)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		encoder.Reset()
		encoder.EncodeString("hello world")
	}
}

func BenchmarkStdLib_String(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		json.Marshal("hello world")
	}
}

func BenchmarkFastEncoder_Int(b *testing.B) {
	encoder := internal.GetEncoder()
	defer internal.PutEncoder(encoder)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		encoder.Reset()
		encoder.EncodeInt(12345)
	}
}

func BenchmarkStdLib_Int(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		json.Marshal(12345)
	}
}

func BenchmarkFastEncoder_SimpleMap(b *testing.B) {
	data := map[string]any{
		"name":   "test",
		"age":    30,
		"active": true,
	}

	encoder := internal.GetEncoder()
	defer internal.PutEncoder(encoder)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		encoder.Reset()
		encoder.EncodeMap(data)
	}
}

func BenchmarkStdLib_SimpleMap(b *testing.B) {
	data := map[string]any{
		"name":   "test",
		"age":    30,
		"active": true,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		json.Marshal(data)
	}
}

func BenchmarkFastEncoder_SimpleArray(b *testing.B) {
	data := []any{1, 2, 3, 4, 5, "test", true}

	encoder := internal.GetEncoder()
	defer internal.PutEncoder(encoder)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		encoder.Reset()
		encoder.EncodeArray(data)
	}
}

func BenchmarkStdLib_SimpleArray(b *testing.B) {
	data := []any{1, 2, 3, 4, 5, "test", true}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		json.Marshal(data)
	}
}

// ----------------------------------------------------------------------------
// STRING INTERNING BENCHMARKS
// ----------------------------------------------------------------------------

func BenchmarkStringIntern_Single(b *testing.B) {
	keys := []string{"name", "age", "active", "email", "phone"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, key := range keys {
			internal.InternKey(key)
		}
	}
}

func BenchmarkStringIntern_Bytes(b *testing.B) {
	keys := [][]byte{
		[]byte("name"),
		[]byte("age"),
		[]byte("active"),
		[]byte("email"),
		[]byte("phone"),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, key := range keys {
			internal.InternKeyBytes(key)
		}
	}
}

func BenchmarkStringIntern_Batch(b *testing.B) {
	keys := make([]string, 1000)
	for i := range keys {
		keys[i] = fmt.Sprintf("key%d", i%100)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		internal.BatchInternKeys(keys)
	}
}

// ----------------------------------------------------------------------------
// LARGE JSON BENCHMARKS
// ----------------------------------------------------------------------------

func generateLargeJSONArray(size int) string {
	var sb strings.Builder
	sb.WriteString("[")
	for i := 0; i < size; i++ {
		if i > 0 {
			sb.WriteString(",")
		}
		sb.WriteString(fmt.Sprintf(`{"id":%d,"name":"user%d","email":"user%d@example.com","active":true,"score":%.2f}`,
			i, i, i, float64(i)*1.5))
	}
	sb.WriteString("]")
	return sb.String()
}

func generateLargeJSONObject(size int) string {
	var sb strings.Builder
	sb.WriteString("{")
	for i := 0; i < size; i++ {
		if i > 0 {
			sb.WriteString(",")
		}
		sb.WriteString(fmt.Sprintf(`"key%d":{"value":%d,"label":"Label %d"}`,
			i, i, i))
	}
	sb.WriteString("}")
	return sb.String()
}

func BenchmarkLargeJSONArray_Parse_1000(b *testing.B) {
	jsonStr := generateLargeJSONArray(1000)
	processor := New()
	defer processor.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = processor.Get(jsonStr, ".")
	}
}

func BenchmarkLargeJSONArray_Parse_10000(b *testing.B) {
	jsonStr := generateLargeJSONArray(10000)
	processor := New()
	defer processor.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = processor.Get(jsonStr, ".")
	}
}

func BenchmarkLargeJSONObject_Parse_1000(b *testing.B) {
	jsonStr := generateLargeJSONObject(1000)
	processor := New()
	defer processor.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = processor.Get(jsonStr, ".")
	}
}

// ----------------------------------------------------------------------------
// ITERATOR BENCHMARKS
// ----------------------------------------------------------------------------

func BenchmarkIterator_SmallArray(b *testing.B) {
	processor := New()
	defer processor.Close()

	jsonStr := `[1,2,3,4,5,6,7,8,9,10]`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		data, _ := processor.Get(jsonStr, ".")
		if arr, ok := data.([]any); ok {
			it := NewPooledSliceIterator(arr)
			for it.Next() {
				_ = it.Value()
			}
			it.Release()
		}
	}
}

func BenchmarkIterator_LargeArray(b *testing.B) {
	processor := New()
	defer processor.Close()

	jsonStr := generateLargeJSONArray(1000)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		data, _ := processor.Get(jsonStr, ".")
		if arr, ok := data.([]any); ok {
			it := NewPooledSliceIterator(arr)
			for it.Next() {
				_ = it.Value()
			}
			it.Release()
		}
	}
}

func BenchmarkIterator_SmallObject(b *testing.B) {
	processor := New()
	defer processor.Close()

	jsonStr := `{"a":1,"b":2,"c":3,"d":4,"e":5}`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		data, _ := processor.Get(jsonStr, ".")
		if obj, ok := data.(map[string]any); ok {
			it := NewPooledMapIterator(obj)
			for it.Next() {
				_, _ = it.Key(), it.Value()
			}
			it.Release()
		}
	}
}

func BenchmarkIterator_LargeObject(b *testing.B) {
	processor := New()
	defer processor.Close()

	jsonStr := generateLargeJSONObject(100)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		data, _ := processor.Get(jsonStr, ".")
		if obj, ok := data.(map[string]any); ok {
			it := NewPooledMapIterator(obj)
			for it.Next() {
				_, _ = it.Key(), it.Value()
			}
			it.Release()
		}
	}
}

// ----------------------------------------------------------------------------
// STREAMING BENCHMARKS
// ----------------------------------------------------------------------------

func BenchmarkStreaming_Array_1000(b *testing.B) {
	jsonData := []byte(generateLargeJSONArray(1000))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sp := NewStreamingProcessor(bytes.NewReader(jsonData), 0)
		_ = sp.StreamArray(func(index int, item any) bool {
			return true
		})
	}
}

func BenchmarkStreaming_Array_10000(b *testing.B) {
	jsonData := []byte(generateLargeJSONArray(10000))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sp := NewStreamingProcessor(bytes.NewReader(jsonData), 0)
		_ = sp.StreamArray(func(index int, item any) bool {
			return true
		})
	}
}

func BenchmarkStreamIterator_1000(b *testing.B) {
	jsonData := generateLargeJSONArray(1000)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		reader := strings.NewReader(jsonData)
		it := NewStreamIterator(reader)
		for it.Next() {
			_ = it.Value()
		}
	}
}

func BenchmarkStreamIterator_10000(b *testing.B) {
	jsonData := generateLargeJSONArray(10000)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		reader := strings.NewReader(jsonData)
		it := NewStreamIterator(reader)
		for it.Next() {
			_ = it.Value()
		}
	}
}

// ----------------------------------------------------------------------------
// PATH PARSING BENCHMARKS
// ----------------------------------------------------------------------------

func BenchmarkPathParsing_Simple(b *testing.B) {
	path := "user.name"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = internal.ParsePath(path)
	}
}

func BenchmarkPathParsing_Complex(b *testing.B) {
	path := "users[0].profile.settings.theme"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = internal.ParsePath(path)
	}
}

func BenchmarkPathParsing_Slice(b *testing.B) {
	path := "data.items[10:20:2].value"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = internal.ParsePath(path)
	}
}

func BenchmarkPathParsing_Extract(b *testing.B) {
	path := "users{name}.emails{flat:address}"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = internal.ParsePath(path)
	}
}

func BenchmarkPathParsing_WithCache(b *testing.B) {
	path := "users.profile.settings.theme"
	cache := getPathCache()

	// Pre-populate cache
	segments, _ := internal.ParsePath(path)
	cache.Set(path, segments)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, ok := cache.Get(path); !ok {
			segments, _ := internal.ParsePath(path)
			cache.Set(path, segments)
		}
	}
}

// ----------------------------------------------------------------------------
// NUMBER PARSING BENCHMARKS
// ----------------------------------------------------------------------------

func BenchmarkFastParseInt_Single(b *testing.B) {
	str := "42"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = internal.FastParseInt([]byte(str))
	}
}

func BenchmarkFastParseInt_Large(b *testing.B) {
	str := "1234567890"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = internal.FastParseInt([]byte(str))
	}
}

func BenchmarkIntToStringFast_Small(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = internal.IntToStringFast(i % 100)
	}
}

func BenchmarkIntToStringFast_Large(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = internal.IntToStringFast(1234567890)
	}
}

// ----------------------------------------------------------------------------
// CONCURRENT ACCESS BENCHMARKS
// ----------------------------------------------------------------------------

func BenchmarkConcurrent_Get(b *testing.B) {
	processor := New()
	defer processor.Close()

	jsonStr := `{"a":1,"b":2,"c":3,"d":4,"e":5}`

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, _ = processor.Get(jsonStr, "a")
		}
	})
}

func BenchmarkConcurrent_Set(b *testing.B) {
	processor := New()
	defer processor.Close()

	jsonStr := `{"a":1,"b":2,"c":3,"d":4,"e":5}`

	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			_, _ = processor.Set(jsonStr, "a", i)
			i++
		}
	})
}

func BenchmarkConcurrent_Marshal(b *testing.B) {
	processor := New()
	defer processor.Close()

	data := map[string]any{
		"name":   "test",
		"age":    30,
		"active": true,
	}

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, _ = processor.Marshal(data)
		}
	})
}

// ----------------------------------------------------------------------------
// DEEP NESTING BENCHMARKS
// ----------------------------------------------------------------------------

func generateDeepNestedJSON(depth int) string {
	var sb strings.Builder
	for i := 0; i < depth; i++ {
		sb.WriteString(fmt.Sprintf(`{"level%d":`, i))
	}
	sb.WriteString(`"value"`)
	for i := 0; i < depth; i++ {
		sb.WriteString("}")
	}
	return sb.String()
}

func BenchmarkDeepNesting_Parse_10(b *testing.B) {
	jsonStr := generateDeepNestedJSON(10)
	processor := New()
	defer processor.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = processor.Get(jsonStr, ".")
	}
}

func BenchmarkDeepNesting_Parse_50(b *testing.B) {
	jsonStr := generateDeepNestedJSON(50)
	processor := New()
	defer processor.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = processor.Get(jsonStr, ".")
	}
}

func BenchmarkDeepNesting_Navigate_10(b *testing.B) {
	jsonStr := generateDeepNestedJSON(10)
	processor := New()
	defer processor.Close()

	path := "level0.level1.level2.level3.level4.level5.level6.level7.level8.level9"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = processor.Get(jsonStr, path)
	}
}

// ----------------------------------------------------------------------------
// BATCH OPERATION BENCHMARKS
// ----------------------------------------------------------------------------

func BenchmarkBatchSet_Small(b *testing.B) {
	processor := New()
	defer processor.Close()

	jsonStr := `{"a":1,"b":2,"c":3}`
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

func BenchmarkBatchSet_Large(b *testing.B) {
	processor := New()
	defer processor.Close()

	// Generate large JSON
	var sb strings.Builder
	sb.WriteString("{")
	for i := 0; i < 100; i++ {
		if i > 0 {
			sb.WriteString(",")
		}
		sb.WriteString(fmt.Sprintf(`"key%d":%d`, i, i))
	}
	sb.WriteString("}")
	jsonStr := sb.String()

	// Generate updates
	updates := make(map[string]any, 50)
	for i := 0; i < 50; i++ {
		updates[fmt.Sprintf("key%d", i)] = i * 10
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = processor.BatchSetOptimized(jsonStr, updates)
	}
}

func BenchmarkFastGetMultiple_Small(b *testing.B) {
	processor := New()
	defer processor.Close()

	jsonStr := `{"a":1,"b":2,"c":3,"d":4,"e":5}`
	paths := []string{"a", "b", "c", "d", "e"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = processor.FastGetMultiple(jsonStr, paths)
	}
}

func BenchmarkFastGetMultiple_Large(b *testing.B) {
	processor := New()
	defer processor.Close()

	// Generate large JSON
	var sb strings.Builder
	sb.WriteString("{")
	for i := 0; i < 100; i++ {
		if i > 0 {
			sb.WriteString(",")
		}
		sb.WriteString(fmt.Sprintf(`"key%d":%d`, i, i))
	}
	sb.WriteString("}")
	jsonStr := sb.String()

	// Generate paths
	paths := make([]string, 50)
	for i := 0; i < 50; i++ {
		paths[i] = fmt.Sprintf("key%d", i)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = processor.FastGetMultiple(jsonStr, paths)
	}
}

// ----------------------------------------------------------------------------
// BUFFER POOL BENCHMARKS
// ----------------------------------------------------------------------------

func BenchmarkBufferPool_GetPut(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf := internal.GetEncoderBuffer()
		buf.WriteString("test")
		internal.PutEncoderBuffer(buf)
	}
}

func BenchmarkBufferPool_Parallel(b *testing.B) {
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			buf := internal.GetEncoderBuffer()
			buf.WriteString("test")
			internal.PutEncoderBuffer(buf)
		}
	})
}

// ----------------------------------------------------------------------------
// COMPARISON BENCHMARKS (vs encoding/json)
// ----------------------------------------------------------------------------

func BenchmarkComparison_Marshal_Simple(b *testing.B) {
	data := map[string]any{
		"name":   "test",
		"age":    30,
		"active": true,
	}

	b.Run("cybergodev/json", func(b *testing.B) {
		processor := New()
		defer processor.Close()

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = processor.Marshal(data)
		}
	})

	b.Run("encoding/json", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = json.Marshal(data)
		}
	})
}

func BenchmarkComparison_Unmarshal_Simple(b *testing.B) {
	jsonStr := `{"name":"test","age":30,"active":true}`

	b.Run("cybergodev/json", func(b *testing.B) {
		processor := New()
		defer processor.Close()

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			var result map[string]any
			_ = processor.Parse(jsonStr, &result)
		}
	})

	b.Run("encoding/json", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			var result map[string]any
			_ = json.Unmarshal([]byte(jsonStr), &result)
		}
	})
}

func BenchmarkComparison_Marshal_Large(b *testing.B) {
	data := make(map[string]any, 100)
	for i := 0; i < 100; i++ {
		data[fmt.Sprintf("key%d", i)] = map[string]any{
			"value": i,
			"label": fmt.Sprintf("Label %d", i),
		}
	}

	b.Run("cybergodev/json", func(b *testing.B) {
		processor := New()
		defer processor.Close()

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = processor.Marshal(data)
		}
	})

	b.Run("encoding/json", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = json.Marshal(data)
		}
	})
}

// ----------------------------------------------------------------------------
// ITERABLE VALUE BENCHMARKS
// ----------------------------------------------------------------------------

func BenchmarkIterableValue_Get(b *testing.B) {
	data := map[string]any{
		"name": "test",
		"age":  30,
		"nested": map[string]any{
			"value": 42,
		},
	}
	iv := NewIterableValue(data)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = iv.Get("name")
	}
}

func BenchmarkIterableValue_GetNested(b *testing.B) {
	data := map[string]any{
		"level1": map[string]any{
			"level2": map[string]any{
				"level3": map[string]any{
					"value": 42,
				},
			},
		},
	}
	iv := NewIterableValue(data)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = iv.Get("level1.level2.level3.value")
	}
}

func BenchmarkIterableValue_GetTyped(b *testing.B) {
	data := map[string]any{
		"name":   "test",
		"age":    30,
		"price":  99.99,
		"active": true,
	}
	iv := NewIterableValue(data)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = iv.GetString("name")
		_ = iv.GetInt("age")
		_ = iv.GetFloat64("price")
		_ = iv.GetBool("active")
	}
}

// ----------------------------------------------------------------------------
// ADDITIONAL PERFORMANCE BENCHMARKS (from performance_test.go)
// ----------------------------------------------------------------------------

// BenchmarkFastSet_Simple compares FastSet vs Set performance
func BenchmarkFastSet_Simple(b *testing.B) {
	processor := New()
	defer processor.Close()

	jsonStr := `{"name":"test","age":30,"active":true}`
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, _ = processor.FastSet(jsonStr, "name", "updated")
	}
}

// BenchmarkSet_Simple for comparison with FastSet
func BenchmarkSet_Simple(b *testing.B) {
	processor := New()
	defer processor.Close()

	jsonStr := `{"name":"test","age":30,"active":true}`
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, _ = processor.Set(jsonStr, "name", "updated")
	}
}

// BenchmarkFastDelete_Simple compares FastDelete vs Delete performance
func BenchmarkFastDelete_Simple(b *testing.B) {
	processor := New()
	defer processor.Close()

	jsonStr := `{"name":"test","age":30,"active":true}`
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, _ = processor.FastDelete(jsonStr, "name")
	}
}

// BenchmarkDelete_Simple for comparison with FastDelete
func BenchmarkDelete_Simple(b *testing.B) {
	processor := New()
	defer processor.Close()

	jsonStr := `{"name":"test","age":30,"active":true}`
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, _ = processor.Delete(jsonStr, "name")
	}
}

// BenchmarkBatchSetOptimized benchmarks the BatchSetOptimized method
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

// BenchmarkFastGetMultiple benchmarks the FastGetMultiple method
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

// BenchmarkPooledSliceIterator benchmarks pooled slice iterator
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

// BenchmarkRegularSliceIteration for comparison with pooled iterator
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

// BenchmarkPooledMapIterator benchmarks pooled map iterator
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

// BenchmarkRegularMapIteration for comparison with pooled iterator
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

// BenchmarkLargeBufferPool benchmarks large buffer pool operations
func BenchmarkLargeBufferPool(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf := getLargeBuffer()
		*buf = append(*buf, make([]byte, 1024)...)
		putLargeBuffer(buf)
	}
}

// BenchmarkEncodeBufferPool benchmarks encode buffer pool operations
func BenchmarkEncodeBufferPool(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf := GetEncodeBuffer()
		buf = append(buf, make([]byte, 512)...)
		PutEncodeBuffer(buf)
	}
}

// BenchmarkIsSimplePropertyAccess benchmarks simple property detection
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

// BenchmarkStreamingProcessor_Array benchmarks streaming processor for arrays
func BenchmarkStreamingProcessor_Array(b *testing.B) {
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
			return true
		})
	}
}
