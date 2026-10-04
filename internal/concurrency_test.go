package internal

import (
	"fmt"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// P-002 concurrency stress for internal shared structures. The race detector
// is the primary oracle; assertions only pin invariants that must hold under
// any interleaving. (Clear-vs-Intern churn for the global interners is already
// covered by TestInternClearConcurrent; these tests target the remaining
// write-hot internal paths.)

// TestP002_CacheManagerTTLExpiryChurn exercises Get's RLock→Lock upgrade path
// (TTL-expired entries are re-validated after the upgrade — the TOCTOU double
// check), Set's periodic-cleanup spawn (wg.Add under the closed flag), and
// CleanExpiredCache, all concurrently under a 2ms TTL.
func TestP002_CacheManagerTTLExpiryChurn(t *testing.T) {
	cm := NewCacheManager(true, 512, 2*time.Millisecond)
	defer cm.Close()

	keys := make([]string, 64)
	for i := range keys {
		keys[i] = "k" + strconv.Itoa(i)
	}

	stop := make(chan struct{})
	var wg sync.WaitGroup

	wg.Add(1)
	go func() { // writer: refreshes entries and triggers periodic cleanup
		defer wg.Done()
		for {
			for _, k := range keys {
				select {
				case <-stop:
					return
				default:
				}
				cm.Set(k, k)
			}
		}
	}()

	wg.Add(1)
	go func() { // reader: hits, expiries, and the lock-upgrade double-check
		defer wg.Done()
		for {
			for _, k := range keys {
				select {
				case <-stop:
					return
				default:
				}
				if v, ok := cm.Get(k); ok && v.(string) != k {
					t.Errorf("Get(%q) = %v; want %q", k, v, k)
					return
				}
			}
		}
	}()

	wg.Add(1)
	go func() { // deleter + explicit shard cleanups
		defer wg.Done()
		i := 0
		for {
			select {
			case <-stop:
				return
			default:
			}
			cm.Delete(keys[i%len(keys)])
			cm.CleanExpiredCache()
			i++
		}
	}()

	time.Sleep(40 * time.Millisecond)
	close(stop)
	wg.Wait()
}

// TestP002_ParsePathEvictChurn drives the process-wide path-segment cache past
// pathCacheMaxSize (10,000) from concurrent goroutines, racing
// setCachedPathSegments, evictPathCacheEntries, and hot-key reads. Afterwards
// the size counter must be non-negative and within one eviction slack of the
// limit, and a hot path must still parse correctly.
func TestP002_ParsePathEvictChurn(t *testing.T) {
	hot := "user.profile.name"
	var wg sync.WaitGroup
	for g := range 8 {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for i := range 1400 {
				path := fmt.Sprintf("g%d.n%d.arr[%d].tail", g, i, i%7)
				if _, err := ParsePath(path); err != nil {
					t.Errorf("ParsePath(%q): %v", path, err)
					return
				}
				if _, err := ParsePath(hot); err != nil {
					t.Errorf("ParsePath(hot): %v", err)
					return
				}
			}
		}(g)
	}
	wg.Wait()

	size := atomic.LoadInt64(&pathCacheSize)
	if size < 0 || size > pathCacheMaxSize+64 {
		t.Fatalf("pathCacheSize = %d after churn; want [0, %d]", size, pathCacheMaxSize+64)
	}
	segs, err := ParsePath(hot)
	if err != nil || len(segs) != 3 {
		t.Fatalf("ParsePath(hot) = %v, %v; want 3 segments, <nil>", segs, err)
	}
}

// TestP002_MetricsCollectorResetChurn races RecordOperation/RecordError/
// GetMetrics against Reset: Reset replaces errorsByType (a sync.Map field) and
// startTime under errorsMu, while RecordError holds RLock and GetMetrics reads
// the multi-word time.Time under RLock — the torn-read fix's exact interleaving.
func TestP002_MetricsCollectorResetChurn(t *testing.T) {
	mc := NewMetricsCollector()
	stop := make(chan struct{})
	var wg sync.WaitGroup

	for w := range 3 {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for i := 0; ; i++ {
				select {
				case <-stop:
					return
				default:
				}
				mc.RecordOperation(time.Duration(i%100)*time.Microsecond, i%4 != 0, 64)
				mc.RecordError("e" + strconv.Itoa(w))
			}
		}(w)
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
			}
			_ = mc.GetMetrics()
			mc.Reset()
		}
	}()

	time.Sleep(30 * time.Millisecond)
	close(stop)
	wg.Wait()
	_ = mc.GetMetrics() // final read must not panic
}

// TestP002_KeyInternTrimChurn crosses maxHotKeys (10,000) with concurrent
// interns so trimHotCache's CAS single-flight and evictShardLocked's
// hotKeys-first deletion run concurrently with promotion. Only structural
// invariants are asserted: interning is idempotent and the hot-key counter
// never goes negative.
func TestP002_KeyInternTrimChurn(t *testing.T) {
	ki := NewKeyIntern()
	var wg sync.WaitGroup
	for g := range 8 {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for i := range 1500 {
				_ = ki.Intern(fmt.Sprintf("k%d_%d", g, i))
			}
		}(g)
	}
	wg.Wait()

	if s := ki.Intern("stable"); s != "stable" {
		t.Fatalf("Intern(stable) = %q; want %q", s, "stable")
	}
	if got := atomic.LoadInt64(&ki.hotKeyCount); got < 0 {
		t.Fatalf("hotKeyCount = %d; want >= 0", got)
	}
}
