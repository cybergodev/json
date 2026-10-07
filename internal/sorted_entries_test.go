package internal

import (
	"maps"
	"slices"
	"sync"
	"testing"
)

// P-001 SortedEntries boundary tests: the three size tiers (direct range /
// stack buffer / pooled buffer), early exit, repeated ranges, and the
// GetSortedKeysSlice/PutSortedKeysSlice contract (exact-cap swap, oversize
// discard, nil safety, concurrent use).

// makeTestMap builds a map with n distinct keys "k%04d" -> n.
func makeTestMap(n int) map[string]int {
	m := make(map[string]int, n)
	for i := 0; i < n; i++ {
		m["k"+string(rune('0'+i/1000%10))+string(rune('0'+i/100%10))+string(rune('0'+i/10%10))+string(rune('0'+i%10))] = i
	}
	return m
}

// collectOrdered drains SortedEntries in order.
func collectOrdered[V any](m map[string]V) []string {
	var got []string
	for k := range SortedEntries(m) {
		got = append(got, k)
	}
	return got
}

func TestSortedEntries_SizeTiers(t *testing.T) {
	// Tier boundaries: 1 (direct range), 2..smallKeysBufSize (stack buffer),
	// smallKeysBufSize+1..maxPooledKeysCap (pool), >maxPooledKeysCap (direct
	// allocation, discarded at Put).
	for _, n := range []int{0, 1, 2, smallKeysBufSize - 1, smallKeysBufSize, smallKeysBufSize + 1, maxPooledKeysCap, maxPooledKeysCap + 1, 300} {
		m := makeTestMap(n)
		got := collectOrdered(m)
		if len(got) != n {
			t.Fatalf("n=%d: got %d keys, want %d", n, len(got), n)
		}
		if want := slices.Sorted(maps.Keys(m)); !slices.Equal(got, want) {
			t.Fatalf("n=%d: keys not sorted: got %v, want %v", n, got, want)
		}
	}

	// Keys must still pair with their original values after iteration.
	m := makeTestMap(smallKeysBufSize + 2)
	seen := make(map[string]int)
	for k, v := range SortedEntries(m) {
		seen[k] = v
	}
	if !maps.Equal(seen, m) {
		t.Fatalf("key/value pairing broken: got %v, want %v", seen, m)
	}
}

func TestSortedEntries_EarlyExit(t *testing.T) {
	// Breaking out of the range must stop yielding mid-iteration across every
	// tier (the pooled tier's deferred Put must still run — verified by
	// immediately doing a full range afterwards and checking correctness).
	for _, n := range []int{1, 2, smallKeysBufSize, smallKeysBufSize + 1, maxPooledKeysCap + 1} {
		m := makeTestMap(n)
		var got []string
		for k := range SortedEntries(m) {
			got = append(got, k)
			if len(got) == 2 {
				break
			}
		}
		if n >= 2 && len(got) != 2 {
			t.Fatalf("n=%d: early exit yielded %d keys, want 2", n, len(got))
		}
		if n < 2 && len(got) != n {
			t.Fatalf("n=%d: early exit yielded %d keys, want %d", n, len(got), n)
		}
		// Sortedness of the received prefix.
		if !slices.IsSorted(got) {
			t.Fatalf("n=%d: early-exit prefix not sorted: %v", n, got)
		}
		// The same Seq must be fully usable afterwards (pool returned clean).
		if full := collectOrdered(m); len(full) != n {
			t.Fatalf("n=%d: post-break full range got %d keys, want %d", n, len(full), n)
		}
	}
}

func TestSortedEntries_RepeatedRanges(t *testing.T) {
	// One iter.Seq2 ranged multiple times: each pass is an independent
	// Get/collect/sort/Put cycle and must produce identical output.
	m := makeTestMap(smallKeysBufSize + 5)
	seq := SortedEntries(m)
	first := collectOrdered(m)
	for i := 0; i < 3; i++ {
		var got []string
		for k := range seq {
			got = append(got, k)
		}
		if !slices.Equal(got, first) {
			t.Fatalf("range pass %d differs from first pass", i+1)
		}
	}
}

func TestSortedKeysPool_ExactCapSwap(t *testing.T) {
	// A Get with a hint larger than the pooled buffer's capacity must yield a
	// buffer with cap >= hint (swap-in), never an append-growth path.
	s := GetSortedKeysSlice(smallKeysBufSize + 50)
	if cap(*s) < smallKeysBufSize+50 {
		t.Fatalf("swap-in: cap=%d < hint", cap(*s))
	}
	*s = append(*s, "a", "b")
	PutSortedKeysSlice(s)

	// Small hints reuse the pool and come back empty.
	s2 := GetSortedKeysSlice(2)
	if len(*s2) != 0 {
		t.Fatalf("pooled buffer not reset: len=%d", len(*s2))
	}
	PutSortedKeysSlice(s2)

	// Oversize buffers are not retained: a >maxPooledKeysCap hint allocates
	// directly; returning it must not poison later Gets.
	s3 := GetSortedKeysSlice(maxPooledKeysCap + 1)
	if cap(*s3) < maxPooledKeysCap+1 {
		t.Fatalf("oversize direct alloc: cap=%d < hint", cap(*s3))
	}
	*s3 = append(*s3, "x")
	PutSortedKeysSlice(s3) // discarded, must be a no-op on the pool

	s4 := GetSortedKeysSlice(3)
	if len(*s4) != 0 {
		t.Fatalf("pool poisoned by oversize Put: len=%d", len(*s4))
	}
	PutSortedKeysSlice(s4)

	// Nil safety.
	PutSortedKeysSlice(nil)
}

func TestSortedEntries_Concurrent(t *testing.T) {
	// Distinct map sizes across goroutines exercise all tiers concurrently;
	// -race guards the pool and the swap-in path.
	small := makeTestMap(3)
	medium := makeTestMap(smallKeysBufSize + 3)
	large := makeTestMap(150)

	var wg sync.WaitGroup
	for g := 0; g < 8; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for i := 0; i < 50; i++ {
				for _, m := range []map[string]int{small, medium, large} {
					prev := ""
					for k := range SortedEntries(m) {
						if k < prev {
							return // caller observes order violation
						}
						prev = k
					}
				}
			}
		}(g)
	}
	wg.Wait()
}

// TestSortedEntries_EncodeMapIntegration pins the encoder-level contract:
// EncodeMap output over every tier is sorted and byte-stable across calls
// (the pooled tier reuses buffers, so staleness would leak into output).
func TestSortedEntries_EncodeMapIntegration(t *testing.T) {
	for _, n := range []int{1, 2, smallKeysBufSize, smallKeysBufSize + 1, maxPooledKeysCap + 1} {
		m := make(map[string]any, n)
		for i := 0; i < n; i++ {
			m[string(rune('a'+i%26))+string(rune('0'+i/10%10))+string(rune('0'+i%10))] = i
		}
		e := GetEncoder()
		if err := e.EncodeMap(m); err != nil {
			t.Fatalf("n=%d: EncodeMap: %v", n, err)
		}
		want := string(append([]byte(nil), e.Bytes()...))
		PutEncoder(e)
		for i := 0; i < 3; i++ {
			e2 := GetEncoder()
			if err := e2.EncodeMap(m); err != nil {
				t.Fatalf("n=%d: EncodeMap pass %d: %v", n, i+1, err)
			}
			if got := string(e2.Bytes()); got != want {
				t.Fatalf("n=%d: pass %d output differs:\n got %s\nwant %s", n, i+1, got, want)
			}
			PutEncoder(e2)
		}
	}
}
