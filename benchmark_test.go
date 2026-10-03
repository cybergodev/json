package json

import (
	"encoding/json"
	"fmt"
	"strconv"
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
// LARGE JSON BENCHMARKS
// ----------------------------------------------------------------------------

func generateLargeJSONArray(size int) string {
	return genJSONArrayRaw(size)
}

func generateLargeJSONObject(size int) string {
	return genJSONObject(size)
}

func BenchmarkLargeJSONArray_Parse_1000(b *testing.B) {
	jsonStr := generateLargeJSONArray(1000)
	processor, _ := New()
	defer processor.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = processor.Get(jsonStr, ".")
	}
}

func BenchmarkLargeJSONArray_Parse_10000(b *testing.B) {
	jsonStr := generateLargeJSONArray(10000)
	processor, _ := New()
	defer processor.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = processor.Get(jsonStr, ".")
	}
}

func BenchmarkLargeJSONObject_Parse_1000(b *testing.B) {
	jsonStr := generateLargeJSONObject(1000)
	processor, _ := New()
	defer processor.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = processor.Get(jsonStr, ".")
	}
}

// BenchmarkLargeJSONArray_Parse_1000_SharedCache mirrors the default-config
// benchmark above but enables Config.CacheSharedResults. On cache hits the
// defensive deep copy (the dominant cost of the default benchmark) is skipped,
// so repeated root Gets return shared references directly.
func BenchmarkLargeJSONArray_Parse_1000_SharedCache(b *testing.B) {
	jsonStr := generateLargeJSONArray(1000)
	cfg := DefaultConfig()
	cfg.CacheSharedResults = true
	processor, _ := New(cfg)
	defer processor.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = processor.Get(jsonStr, ".")
	}
}

// BenchmarkLargeJSONObject_Parse_1000_SharedCache mirrors the object variant.
func BenchmarkLargeJSONObject_Parse_1000_SharedCache(b *testing.B) {
	jsonStr := generateLargeJSONObject(1000)
	cfg := DefaultConfig()
	cfg.CacheSharedResults = true
	processor, _ := New(cfg)
	defer processor.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = processor.Get(jsonStr, ".")
	}
}

// ----------------------------------------------------------------------------
// ITERATOR BENCHMARKS
// ----------------------------------------------------------------------------

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
	processor, _ := New()
	defer processor.Close()

	jsonStr := `{"a":1,"b":2,"c":3,"d":4,"e":5}`

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, _ = processor.Get(jsonStr, "a")
		}
	})
}

func BenchmarkConcurrent_Set(b *testing.B) {
	processor, _ := New()
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
	processor, _ := New()
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
	return genNestedJSONDynamicKeys(depth)
}

func BenchmarkDeepNesting_Parse_10(b *testing.B) {
	jsonStr := generateDeepNestedJSON(10)
	processor, _ := New()
	defer processor.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = processor.Get(jsonStr, ".")
	}
}

func BenchmarkDeepNesting_Parse_50(b *testing.B) {
	jsonStr := generateDeepNestedJSON(50)
	processor, _ := New()
	defer processor.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = processor.Get(jsonStr, ".")
	}
}

func BenchmarkDeepNesting_Navigate_10(b *testing.B) {
	jsonStr := generateDeepNestedJSON(10)
	processor, _ := New()
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
		processor, _ := New()
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
		processor, _ := New()
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
		processor, _ := New()
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
	iv := newIterableValue(data)

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
	iv := newIterableValue(data)

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
	iv := newIterableValue(data)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = iv.GetString("name")
		_ = iv.GetInt("age")
		_ = iv.GetFloat64("price")
		_ = iv.GetBool("active")
	}
}

// ----------------------------------------------------------------------------
// ADDITIONAL PERFORMANCE BENCHMARKS
// ----------------------------------------------------------------------------

// BenchmarkSet_Simple for comparison with FastSet
func BenchmarkSet_Simple(b *testing.B) {
	processor, _ := New()
	defer processor.Close()

	jsonStr := `{"name":"test","age":30,"active":true}`
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, _ = processor.Set(jsonStr, "name", "updated")
	}
}

// BenchmarkDelete_Simple for comparison with FastDelete
func BenchmarkDelete_Simple(b *testing.B) {
	processor, _ := New()
	defer processor.Close()

	jsonStr := `{"name":"test","age":30,"active":true}`
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, _ = processor.Delete(jsonStr, "name")
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

// ============================================================================
// BENCHMARKS FOR THE RECURSIVE PROCESSOR (wildcard / extract / array ops)
//
// These paths exercise the per-handler []error and []any slice allocations in
// recursive.go — the controllable allocation surface identified by pprof
// (the dominant cost, encoding/json parsing, is not controllable).
// ============================================================================

// BenchmarkGet_PropertyOverArray exercises handlePropertySegmentUnified's []any
// (distributed) branch: Get("name") over a root array of objects collects one
// result per element and allocates a local []error buffer per call.
func BenchmarkGet_PropertyOverArray_100(b *testing.B) {
	jsonStr := generateLargeJSONArray(100)
	processor, _ := New()
	defer processor.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = processor.Get(jsonStr, "name")
	}
}

// BenchmarkGet_ExtractArray exercises handleExtractSegmentUnified's []any branch
// via the {field} extraction syntax over a root array of objects.
func BenchmarkGet_ExtractArray_100(b *testing.B) {
	jsonStr := generateLargeJSONArray(100)
	processor, _ := New()
	defer processor.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = processor.Get(jsonStr, "{name}")
	}
}

// BenchmarkGet_ArraySlice exercises handleArraySliceSegmentUnified's non-distributed
// []any branch, which slices the array and then iterates the remaining (none) segments.
func BenchmarkGet_ArraySlice_100(b *testing.B) {
	jsonStr := generateLargeJSONArray(100)
	processor, _ := New()
	defer processor.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = processor.Get(jsonStr, "[0:50]")
	}
}

// ----------------------------------------------------------------------------
// PerformArraySlice A/B micro-benchmark (legacy vs optimized unit-step path)
// ----------------------------------------------------------------------------

// performArraySliceLegacy is the pre-optimization implementation: it always
// builds a temporary []int index slice via PerformArraySliceIndices, then
// appends element-by-element. Kept here only to A/B against the optimized
// internal.PerformArraySlice (unit-step fast path uses make+copy, no []int).
func performArraySliceLegacy(arr []any, start, end, step *int) []any {
	indices := internal.PerformArraySliceIndices(len(arr), start, end, step)
	result := make([]any, 0, len(indices))
	for _, i := range indices {
		result = append(result, arr[i])
	}
	return result
}

// BenchmarkPerformArraySlice_AB compares the legacy (temp []int + append loop)
// implementation against the optimized unit-step path (make + copy) on a
// 1000-element []any sliced [0:500]. Isolates the slice function from parsing
// and deep-copy so the allocation/CPU delta is visible.
func BenchmarkPerformArraySlice_AB(b *testing.B) {
	arr := make([]any, 1000)
	for i := range arr {
		arr[i] = i
	}
	start, end := 0, 500
	var step *int // nil step → unit step (fast path)

	b.Run("legacy", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_ = performArraySliceLegacy(arr, &start, &end, step)
		}
	})

	b.Run("optimized", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_ = internal.PerformArraySlice(arr, &start, &end, step)
		}
	})
}

// ----------------------------------------------------------------------------
// Foreach path-build A/B micro-benchmark (per-element buffer vs reused buffer)
// ----------------------------------------------------------------------------

// buildPathLegacy mirrors the pre-optimization per-element path construction
// that foreachWithPathIterableValue used: a fresh []byte is appended from nil
// each element, then copied to a string — 2 allocations per element.
func buildPathLegacy(currentPath string, i int) string {
	var buf []byte
	buf = append(buf, currentPath...)
	buf = append(buf, '[')
	buf = strconv.AppendInt(buf, int64(i), 10)
	buf = append(buf, ']')
	return string(buf)
}

// benchmarkPathSink prevents dead-code elimination of string(buf) so the
// per-element string allocation (which the real callback consumes) is measured.
var benchmarkPathSink string

// BenchmarkForeachPathBuild_AB isolates the per-element path-string construction
// in foreachWithPathIterableValue (iterator.go). "legacy" allocates a fresh
// []byte per element (the old code); "reused" reuses one buffer across all
// elements (the optimization now in place). Only the string(buf) conversion
// allocates per element in the reused path.
func BenchmarkForeachPathBuild_AB(b *testing.B) {
	const n = 1000
	currentPath := "users"

	b.Run("legacy", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			for j := 0; j < n; j++ {
				benchmarkPathSink = buildPathLegacy(currentPath, j)
			}
		}
	})

	b.Run("reused", func(b *testing.B) {
		b.ReportAllocs()
		buf := make([]byte, 0, 64)
		for i := 0; i < b.N; i++ {
			for j := 0; j < n; j++ {
				buf = buf[:0]
				buf = append(buf, currentPath...)
				buf = append(buf, '[')
				buf = strconv.AppendInt(buf, int64(j), 10)
				buf = append(buf, ']')
				benchmarkPathSink = string(buf)
			}
		}
	})
}

// ============================================================================
// P-001 OPTIMIZATION BENCHMARKS
// These exercise paths that the benchmarks above cannot see: distinct inputs
// (the validation cache is bypassed, so the security scan actually runs) and
// the encode paths changed by the P-001 round (single-pass bytes fast path,
// single-key map encoding, SetMultiple without defensive deep copy).
// ============================================================================

// genDistinctInputs builds n distinct small JSON objects. More than
// securityCacheHighWatermark (8000) entries are produced so a cycling
// benchmark reader misses the validation cache on (nearly) every iteration —
// the steady state of real-world workloads that never repeat an input.
func genDistinctInputs(n int) []string {
	inputs := make([]string, n)
	for i := 0; i < n; i++ {
		inputs[i] = fmt.Sprintf(`{"id":%d,"name":"user%d","note":"plain data %d","active":true}`, i, i, i)
	}
	return inputs
}

// BenchmarkGet_DistinctInputs measures Get over never-repeated inputs: the
// validation scan, hashing, parse, and navigation all run on every iteration
// (the validation cache cannot short-circuit).
func BenchmarkGet_DistinctInputs(b *testing.B) {
	inputs := genDistinctInputs(securityCacheHighWatermark + 200)
	processor, _ := New()
	defer processor.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = processor.Get(inputs[i%len(inputs)], "name")
	}
}

// BenchmarkValidateInput_Distinct isolates the input validation pipeline
// (UTF-8, security pattern scan, structure, depth, container counts) on
// distinct inputs, without parse/cache/navigation costs.
func BenchmarkValidateInput_Distinct(b *testing.B) {
	inputs := genDistinctInputs(securityCacheHighWatermark + 200)
	processor, _ := New()
	defer processor.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = processor.validateInput(inputs[i%len(inputs)])
	}
}

// BenchmarkSetMultiple_MultiKey covers SetMultiple with several updates:
// path validation, single parse, in-place application, and re-encoding.
func BenchmarkSetMultiple_MultiKey(b *testing.B) {
	processor, _ := New()
	defer processor.Close()

	jsonStr := `{"a":1,"b":2,"c":3,"nested":{"x":10,"y":20}}`
	updates := map[string]any{
		"a":        100,
		"c":        false,
		"nested.x": 42,
		"nested.z": "new",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = processor.SetMultiple(jsonStr, updates)
	}
}

// BenchmarkEncodeWithConfig_SimpleMap covers the encodeWithConfigToBytes fast
// path (governance + config handling + FastEncoder single-pass bytes output).
func BenchmarkEncodeWithConfig_SimpleMap(b *testing.B) {
	processor, _ := New()
	defer processor.Close()

	data := map[string]any{
		"name":   "test",
		"age":    30,
		"active": true,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = processor.EncodeWithConfig(data)
	}
}

// BenchmarkToJSONL_100 covers batch JSONL assembly: 100 items encoded
// straight into the shared output buffer.
func BenchmarkToJSONL_100(b *testing.B) {
	data := make([]any, 100)
	for i := range data {
		data[i] = map[string]any{"id": i, "name": fmt.Sprintf("user%d", i)}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = ToJSONL(data)
	}
}

// BenchmarkFastEncoder_SingleKeyMap isolates the single-key map encode case
// (the most common result shape after Set/Delete): forEachSortedEntry must
// take the allocation-free direct range path.
func BenchmarkFastEncoder_SingleKeyMap(b *testing.B) {
	data := map[string]any{"name": "updated"}

	encoder := internal.GetEncoder()
	defer internal.PutEncoder(encoder)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		encoder.Reset()
		_ = encoder.EncodeValue(data)
	}
}

// ----------------------------------------------------------------------------
// Security scan A/B micro-benchmark (legacy per-pattern scan vs prefilter)
// ----------------------------------------------------------------------------

// scanWindowLegacy mirrors the pre-P-001 scan shape: one full
// fastIndexIgnoreCase pass per pattern over the window, ~27 scans total on a
// clean window. Kept here only to A/B against windowContainsDangerousMatch.
func scanWindowLegacy(window string) bool {
	for _, dp := range dangerousPatterns {
		if fastIndexIgnoreCase(window, dp.pattern) != -1 {
			return true
		}
	}
	return false
}

// BenchmarkScanWindow_AB isolates the built-in-pattern scan stage of input
// validation on a clean window (the common case): legacy scans the window
// once per pattern; the prefilter makes one pass and skips the pattern loop
// entirely when nothing matches.
func BenchmarkScanWindow_AB(b *testing.B) {
	window := `{"id":12345,"name":"user42","email":"user42@example.com","note":"plain descriptive text","active":true,"score":12.5,"tags":["a","b","c"]}`

	b.Run("legacy", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			if scanWindowLegacy(window) {
				b.Fatal("clean window must not match")
			}
		}
	})

	b.Run("prefilter", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			if windowContainsDangerousMatch(window) {
				b.Fatal("clean window must not match")
			}
		}
	})
}
