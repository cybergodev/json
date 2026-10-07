package internal

import (
	"strings"
	"sync"
)

// ============================================================================
// PERFORMANCE OPTIMIZATION POOLS
// These pools reduce heap allocations in hot paths
// ============================================================================

const (
	// Pool size thresholds
	// largeSliceSize is the hint above which path-segment slices are allocated
	// directly instead of from a pool.
	largeSliceSize = 128
)

// ----------------------------------------------------------------------------
// STRING BUILDER POOL - For string building operations
// PERFORMANCE: Reduces allocations in string concatenation
// ----------------------------------------------------------------------------

var stringBuilderPool = sync.Pool{
	New: func() any {
		sb := &strings.Builder{}
		sb.Grow(256)
		return sb
	},
}

// GetStringBuilder retrieves a pooled strings.Builder
func GetStringBuilder() *strings.Builder {
	sb := stringBuilderPool.Get().(*strings.Builder)
	sb.Reset()
	return sb
}

// PutStringBuilder returns a strings.Builder to the pool
func PutStringBuilder(sb *strings.Builder) {
	if sb == nil {
		return
	}
	// Don't pool very large builders
	if sb.Cap() > 16*1024 {
		return
	}
	sb.Reset()
	stringBuilderPool.Put(sb)
}

// ----------------------------------------------------------------------------
// SORTED KEYS POOL - For SortedEntries' multi-key collect-and-sort path
// ----------------------------------------------------------------------------

// smallKeysBufSize is the map width below which SortedEntries collects keys
// into a fixed-size stack array instead of the pool: small maps dominate real
// JSON (nested two/three-key objects), a stack array is deterministically
// allocation-free, and pool Get/defer/Put traffic per nested map costs more
// than the allocation it avoids (see SortedEntries).
const smallKeysBufSize = 8

// maxPooledKeysCap bounds the key-slice capacity retained by the pool. Wider
// maps allocate directly and are dropped after use, so one huge map cannot pin
// a huge buffer in the pool. At 256 keys a retained buffer costs at most ~4KB,
// which keeps even the wide-map steady state (reused exact-cap buffer) inside
// the pool while bounding pinned memory.
const maxPooledKeysCap = 256

// sortedKeysPool pools []string buffers used to collect map keys before
// sorting (SortedEntries' multi-key path). Profiling (P-001) showed that
// allocation was ~30% of ALL allocated objects on the Set/Delete path, where
// every result encode sorts the keys of a multi-key map.
//
// New hands out a small buffer deliberately: GetSortedKeysSlice swaps in an
// exact-capacity buffer whenever the pooled one is smaller than the caller's
// hint, so buffers in circulation are gradually sized to actual workloads
// instead of every first-time Get pinning a max-size (4KB) buffer per P.
var sortedKeysPool = sync.Pool{
	New: func() any {
		s := make([]string, 0, 16)
		return &s
	},
}

// GetSortedKeysSlice retrieves a pooled []string buffer for key collection.
// hint is the expected number of keys; hints above maxPooledKeysCap allocate
// directly instead of from the pool.
func GetSortedKeysSlice(hint int) *[]string {
	if hint > maxPooledKeysCap {
		s := make([]string, 0, hint)
		return &s
	}
	s := sortedKeysPool.Get().(*[]string)
	if cap(*s) < hint {
		// The pooled buffer is smaller than needed: appending would regrow it
		// through several copy rounds, and after every GC pool clear that
		// churn repeats (P-001: it measurably slowed 100-key map encodes).
		// Swap in one exact-sized buffer instead.
		ns := make([]string, 0, hint)
		s = &ns
	} else {
		*s = (*s)[:0]
	}
	return s
}

// PutSortedKeysSlice returns a key buffer to the pool. Buffers that grew past
// maxPooledKeysCap are discarded.
func PutSortedKeysSlice(s *[]string) {
	if s == nil {
		return
	}
	if cap(*s) > maxPooledKeysCap {
		return // Don't pool very large key buffers
	}
	*s = (*s)[:0]
	sortedKeysPool.Put(s)
}

// ----------------------------------------------------------------------------
// PATH SEGMENT SLICE POOL - For path parsing results
// ----------------------------------------------------------------------------

var (
	// smallPathPool pools small []PathSegment slices (cap 4)
	smallPathPool = sync.Pool{
		New: func() any {
			s := make([]PathSegment, 0, 4)
			return &s
		},
	}

	// mediumPathPool pools medium []PathSegment slices (cap 8)
	mediumPathPool = sync.Pool{
		New: func() any {
			s := make([]PathSegment, 0, 8)
			return &s
		},
	}

	// largePathPool pools large []PathSegment slices (cap 16)
	largePathPool = sync.Pool{
		New: func() any {
			s := make([]PathSegment, 0, 16)
			return &s
		},
	}
)

// GetPathSegmentSlice retrieves a pooled []PathSegment slice
func GetPathSegmentSlice(hint int) *[]PathSegment {
	// SECURITY: For hints larger than pool capacity, allocate directly — a
	// pooled cap-16 slice would immediately regrow and be dropped at Put
	// (cap > 32 is not pooled), churning allocations for deep paths. Hints in
	// (16, largeSliceSize] still take the cap-16 large pool and may regrow by
	// append; the threshold trades that churn against extra pooling, and the
	// only production caller today passes 8.
	if hint > largeSliceSize {
		s := make([]PathSegment, 0, hint)
		return &s
	}
	var s *[]PathSegment
	switch {
	case hint <= 4:
		s = smallPathPool.Get().(*[]PathSegment)
	case hint <= 8:
		s = mediumPathPool.Get().(*[]PathSegment)
	default:
		s = largePathPool.Get().(*[]PathSegment)
	}
	*s = (*s)[:0]
	return s
}

// PutPathSegmentSlice returns a []PathSegment slice to the pool
func PutPathSegmentSlice(s *[]PathSegment) {
	if s == nil {
		return
	}
	c := cap(*s)
	if c > 32 {
		return // Don't pool very large slices
	}
	*s = (*s)[:0]
	switch {
	case c <= 4:
		smallPathPool.Put(s)
	case c <= 8:
		mediumPathPool.Put(s)
	default:
		largePathPool.Put(s)
	}
}
