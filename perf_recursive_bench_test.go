package json

import "testing"

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
