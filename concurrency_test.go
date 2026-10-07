package json

// P-002 concurrency-audit regression tests.
//
// F1 under review: cachedMaxPatternLen (security.go) is a lock-free cached
// maximum pattern length that the rolling-window scanner uses for its overlap
// (scanWithRollingWindow: overlapSize = maxDangerousPatternLen() + 8). The
// pre-fix code invalidated the cache with a bare atomic.Store in the public
// register/unregister wrappers while recomputeMaxPatternLen read the registry
// and published its result WITHOUT the registry lock — a last-writer-wins race
// where a recompute that read a pre-registration snapshot could overwrite a
// newer invalidation with a stale, smaller length. Until the next registry
// event, the overlap was too small for a globally-registered long pattern,
// reopening the window-boundary straddle gap fixed in D-002 round 6.
//
// The fix serializes both sides through the registry RWMutex: Add/Remove/Clear
// invalidate while holding the write lock, and recompute reads + stores under
// the read lock.

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestP002_RecomputeSerializedWithRegistryWrites deterministically verifies the
// fix mechanism: recomputeMaxPatternLen must acquire the registry read lock, so
// while a writer holds globalPatternRegistry.mu (exactly what Add does during
// RegisterDangerousPattern), a concurrent recompute cannot run to completion
// and publish a result computed from a snapshot taken before the write.
//
// The pre-fix recompute took no lock, completed immediately, and failed this
// test — which is precisely the interleaving that let a stale length overwrite
// a fresh invalidation.
func TestP002_RecomputeSerializedWithRegistryWrites(t *testing.T) {
	const patLen = 200
	longPattern := DangerousPattern{
		Pattern: "p002_serialize" + strings.Repeat("x", patLen),
		Name:    "P-002 serialization probe",
		Level:   PatternLevelCritical,
	}
	RegisterDangerousPattern(longPattern)
	defer UnregisterDangerousPattern(longPattern.Pattern)

	// Reset the cache so the concurrent recompute below cannot be satisfied by
	// a previously cached value.
	atomic.StoreInt64(&cachedMaxPatternLen, 0)

	done := make(chan int, 1)

	// Simulate Add() mid-mutation: hold the registry write lock.
	globalPatternRegistry.mu.Lock()
	go func() { done <- recomputeMaxPatternLen() }()

	select {
	case <-done:
		// Release the write lock BEFORE failing so later tests in the package
		// are not deadlocked behind this one's fatal path.
		globalPatternRegistry.mu.Unlock()
		t.Fatal("recomputeMaxPatternLen completed while the registry write lock was held — " +
			"its result is not serialized against registration, so a stale length can " +
			"overwrite a newer invalidation (P-002 F1)")
	case <-time.After(50 * time.Millisecond):
		// Blocked on the read lock, as required.
	}

	globalPatternRegistry.mu.Unlock()

	select {
	case got := <-done:
		if got < len(longPattern.Pattern) {
			t.Fatalf("recomputed max pattern length = %d, want >= %d (registered pattern visible after lock release)",
				got, len(longPattern.Pattern))
		}
	case <-time.After(5 * time.Second):
		t.Fatal("recomputeMaxPatternLen did not finish after the write lock was released")
	}
}

// TestP002_PatternLenCacheConcurrentChurn exercises the invalidation/recompute
// pair under concurrent load. The race detector (run with -race) validates the
// lock discipline; the post-churn assertion pins the observable contract: once
// registration activity has settled, the cached length must cover the
// registered pattern.
func TestP002_PatternLenCacheConcurrentChurn(t *testing.T) {
	const patLen = 150
	pattern := DangerousPattern{
		Pattern: "p002_churn" + strings.Repeat("y", patLen),
		Name:    "P-002 churn probe",
		Level:   PatternLevelCritical,
	}
	RegisterDangerousPattern(pattern)
	defer UnregisterDangerousPattern(pattern.Pattern)

	const readers = 4
	var wg sync.WaitGroup
	stop := make(chan struct{})

	// Readers keep maxDangerousPatternLen() hot, forcing recomputes after each
	// invalidation.
	for range readers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
				}
				_ = maxDangerousPatternLen()
			}
		}()
	}

	// Churner invalidates via repeated register/unregister cycles.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for range 2000 {
			UnregisterDangerousPattern(pattern.Pattern)
			RegisterDangerousPattern(pattern)
		}
		// Leave the pattern registered.
		RegisterDangerousPattern(pattern)
	}()

	// Wait specifically for the churner by letting all goroutines observe stop
	// only after churn completes: close stop, then wg.Wait covers everyone.
	// The churner's final Register is its last statement, so after wg.Wait()
	// the registry holds the pattern and the cache holds either 0 (invalidated)
	// or a value >= patLen.
	close(stop)
	wg.Wait()

	if got := maxDangerousPatternLen(); got < len(pattern.Pattern) {
		t.Fatalf("post-churn maxDangerousPatternLen() = %d, want >= %d", got, len(pattern.Pattern))
	}
}

// -----------------------------------------------------------------------------
// P-002 round 2: concurrency stress for shared surfaces whose safety was
// established by code review but lacked dedicated concurrent coverage. The
// race detector is the primary oracle; functional assertions only pin
// contracts that must hold under any interleaving.
// -----------------------------------------------------------------------------

// TestP002_ConfigProcessorCacheChurn drives getProcessorWithConfig across MORE
// distinct configs than the cache limit (64), so concurrent calls interleave
// LoadOrStore insertion, stale-entry CompareAndSwap/CompareAndDelete,
// maybeEvictConfigCache, and asyncCloseProcessor of evicted instances.
//
// An evicted-and-closed processor may legitimately surface in a caller's hand
// (eviction protects only the caller's own key — see maybeEvictConfigCache);
// ErrProcessorClosed on the functional probe is therefore tolerated and simply
// proves eviction ran.
func TestP002_ConfigProcessorCacheChurn(t *testing.T) {
	const distinctConfigs = 80 // > configProcessorCacheLimit (64) to force eviction
	const callsPerGoroutine = 20

	cfgs := make([]Config, distinctConfigs)
	for i := range cfgs {
		cfgs[i] = DefaultConfig()
		cfgs[i].MaxCacheSize = 9000 + i
	}

	var wg sync.WaitGroup
	for g := range 8 {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for i := range callsPerGoroutine {
				idx := (g*callsPerGoroutine + i) % distinctConfigs
				p, err := getProcessorWithConfig(cfgs[idx])
				if err != nil {
					t.Errorf("getProcessorWithConfig(config %d): %v", idx, err)
					return
				}
				if _, err := p.Get(`{"ok":true}`, "ok"); err != nil && !errors.Is(err, ErrProcessorClosed) {
					t.Errorf("Get via cached processor %d: %v", idx, err)
					return
				}
			}
		}(g)
	}
	wg.Wait()
}

// TestP002_GlobalProcessorLifecycleChurn races package-level operations
// against SetGlobalProcessor and ShutdownGlobalProcessor. Individual calls may
// fail transiently (ErrProcessorClosed / ErrConcurrencyLimit / nil-processor
// fallback) — that is the documented churn behavior; the assertions are: no
// panic, and package-level calls recover once churn settles.
func TestP002_GlobalProcessorLifecycleChurn(t *testing.T) {
	done := make(chan struct{})
	var wg sync.WaitGroup

	for range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-done:
					return
				default:
				}
				_, _ = Get(`{"a":1}`, "a")
				_, _ = Set(`{"a":1}`, "a", 2)
			}
		}()
	}

	for range 6 {
		p, err := New(DefaultConfig())
		if err != nil {
			close(done)
			wg.Wait()
			t.Fatalf("New(DefaultConfig()): %v", err)
		}
		SetGlobalProcessor(p)
	}
	close(done)
	wg.Wait()

	ShutdownGlobalProcessor()

	// Post-churn contract: a fresh default processor is created on demand.
	v, err := Get(`{"k":"v"}`, "k")
	if err != nil || v != "v" {
		t.Fatalf("post-churn Get = %v, %v; want \"v\", <nil>", v, err)
	}
}

// TestP002_ValidateCacheCloseChurn hammers a single processor's validation
// cache (isValidationCached read path, cacheValidationWithKey lazy creation,
// LRU eviction) while Close() runs concurrently: securityValidator.Close()
// flips cacheDisabled under the write lock against in-flight validations, and
// Processor.Close drains governed ops under the same churn.
func TestP002_ValidateCacheCloseChurn(t *testing.T) {
	p, err := New(DefaultConfig())
	if err != nil {
		t.Fatalf("New(DefaultConfig()): %v", err)
	}

	var stop atomic.Bool
	var wg sync.WaitGroup
	for w := range 6 {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for i := 0; !stop.Load(); i++ {
				doc := fmt.Sprintf(`{"w%d":%d,"pad":"%s"}`, w, i%50, strings.Repeat("p", 16))
				if _, err := p.Get(doc, fmt.Sprintf("w%d", w)); err != nil &&
					!errors.Is(err, ErrProcessorClosed) {
					t.Errorf("worker %d Get: %v", w, err)
					return
				}
			}
		}(w)
	}

	// Let the workers warm the validation cache, then close mid-flight.
	time.Sleep(2 * time.Millisecond)
	_ = p.Close()
	stop.Store(true)
	wg.Wait()
}

// TestP002_PathTypeCacheEvictChurn fills the sharded path-type cache past its
// per-shard limit (16 shards × 256 entries) from multiple goroutines while a
// concurrent clearer mirrors ShutdownGlobalProcessor's clearPathTypeCache,
// then pins the classification contract.
func TestP002_PathTypeCacheEvictChurn(t *testing.T) {
	var wg sync.WaitGroup
	for g := range 8 {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for i := range 900 {
				simple := fmt.Sprintf("g%d_i%d_field", g, i)
				if getPathType(simple) != pathTypeSimple {
					t.Errorf("getPathType(%q) != simple", simple)
					return
				}
				if getPathType(simple+".nested[0]") != pathTypeComplex {
					t.Errorf("getPathType(%q.nested[0]) != complex", simple)
					return
				}
			}
		}(g)
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		for range 5 {
			clearPathTypeCache()
			time.Sleep(time.Millisecond)
		}
	}()

	wg.Wait()

	if getPathType("plain") != pathTypeSimple || getPathType("a.b") != pathTypeComplex {
		t.Fatal("path type classification broken after churn")
	}
}

// TestP002_PreParseSharedTreeContract documents and guards the P-002
// resolution for PreParse/ParsedJSON.Data(): the tree behind Data() is the
// parse-cache entry itself (zero-copy — copying it was measured at ~47x the
// hit-path cost, BenchmarkPreParse_Large 1.3ms vs 28µs, an unacceptable
// regression for this API's stated purpose), so its contract is DO NOT
// MUTATE. What MUST hold — and what this test enforces — is that every
// supported value-extraction path hands out safe copies, so callers never
// need to touch the shared tree to get work done:
//  1. GetFromParsed results are independent copies (default config): caller
//     mutation of an extracted value cannot poison the cache.
//  2. Concurrent read-only use (PreParse + GetFromParsed + Data() reads)
//     stays race-clean under -race.
func TestP002_PreParseSharedTreeContract(t *testing.T) {
	const src = `{"user":{"name":"Alice","roles":["a","b"]},"n":1}`

	p, err := New()
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer p.Close()

	parsed, err := p.PreParse(src)
	if err != nil {
		t.Fatalf("PreParse: %v", err)
	}

	// 1. Mutating an EXTRACTED result must not poison the shared tree.
	extracted, err := p.GetFromParsed(parsed, "user")
	if err != nil {
		t.Fatalf("GetFromParsed(user): %v", err)
	}
	user := extracted.(map[string]any)
	user["name"] = "MALLORY"
	user["roles"].([]any)[0] = "pwned"

	name, err := p.GetFromParsed(parsed, "user.name")
	if err != nil {
		t.Fatalf("GetFromParsed(name): %v", err)
	}
	if name != "Alice" {
		t.Errorf("cache poisoned via mutated GetFromParsed result: got %v, want Alice", name)
	}
	role, err := p.GetFromParsed(parsed, "user.roles[0]")
	if err != nil {
		t.Fatalf("GetFromParsed(roles[0]): %v", err)
	}
	if role != "a" {
		t.Errorf("nested slice poisoned via mutated result: got %v, want a", role)
	}

	// 2. Concurrent READ-ONLY use of the same shared tree is race-clean.
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 50 {
				pj, err := p.PreParse(src)
				if err != nil {
					t.Errorf("concurrent PreParse: %v", err)
					return
				}
				v, err := p.GetFromParsed(pj, "user.name")
				if err != nil {
					t.Errorf("concurrent GetFromParsed: %v", err)
					return
				}
				if v != "Alice" {
					t.Errorf("concurrent read saw poisoned value: got %v, want Alice", v)
					return
				}
				_ = pj.Data() // read-only Data() access is part of the contract
			}
		}()
	}
	wg.Wait()

	// CacheSharedResults=true opts into sharing of extracted RESULTS as well;
	// the mode must keep working (correct values).
	cfg := DefaultConfig()
	cfg.CacheSharedResults = true
	ps, err := New(cfg)
	if err != nil {
		t.Fatalf("New(shared): %v", err)
	}
	defer ps.Close()
	sp, err := ps.PreParse(src)
	if err != nil {
		t.Fatalf("shared PreParse: %v", err)
	}
	sname, err := ps.GetFromParsed(sp, "user.name")
	if err != nil {
		t.Fatalf("shared GetFromParsed: %v", err)
	}
	if sname != "Alice" {
		t.Errorf("shared mode returned %v, want Alice", sname)
	}
}

// TestP002_JSONLParallelEarlyReturnsJoinWorkers guards the defer-based
// close(jobs)+wg.Wait() restructure in StreamJSONLParallelWithContext
// (P-002 MEDIUM): every early-return path (memory limit, nesting error, parse
// error) must close the jobs channel and join the workers. Pre-fix, a panic in
// the feed loop skipped close(jobs) and leaked every worker permanently; the
// explicit close+wait pairs were also a double-close hazard waiting to happen
// on any future refactor. A regression hangs (workers never joined → blocked
// send) or panics (double close), failing the test either way.
func TestP002_JSONLParallelEarlyReturnsJoinWorkers(t *testing.T) {
	p, err := New()
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer p.Close()

	lines := strings.Repeat("{\"a\":1}\n", 50)
	noop := func(int, *IterableValue) error { return nil }

	// Memory-limit early return (workers spawned, fed at least one job).
	cfg := DefaultConfig()
	cfg.JSONLMaxMemory = 8
	err = p.StreamJSONLParallel(strings.NewReader(lines), 4, noop, cfg)
	if !errors.Is(err, ErrSizeLimit) {
		t.Fatalf("mem-limit path: got %v, want ErrSizeLimit", err)
	}

	// Parse-error early return with JSONLContinueOnErr disabled.
	bad := "{\"a\":1}\n{not json at all}\n"
	err = p.StreamJSONLParallel(strings.NewReader(bad), 4, noop)
	if err == nil {
		t.Fatal("parse-error path: expected an error, got nil")
	}

	// Nesting-error early return (one deep line beyond the default cap of 200).
	deep := "{\"a\":1}\n" + strings.Repeat("[", 300) + strings.Repeat("]", 300) + "\n"
	err = p.StreamJSONLParallel(strings.NewReader(deep), 4, noop)
	if err == nil {
		t.Fatal("nesting path: expected an error, got nil")
	}

	// Normal completion still joins and returns nil.
	if err := p.StreamJSONLParallel(strings.NewReader(lines), 4, noop); err != nil {
		t.Fatalf("normal path: %v", err)
	}
}

// TestGEN001_IterableValuePoolConcurrentRelease is a regression test for the
// GEN-001 P0-1 audit finding: a callback calling Release() Put the value back
// to the shared pool, another goroutine's Get() re-initialized it, and the
// original iteration loop's putIterableValue then nulled the foreign data and
// Put the same pointer a second time — one object handed to two goroutines,
// plus a data race on data/released (plain bool check-then-act; an atomic CAS
// variant re-raced via ABA after the re-Get reset the flag). The fix is the
// single-putter invariant: Release only clears data, and only the owning
// loop Puts. Run with -race.
func TestGEN001_IterableValuePoolConcurrentRelease(t *testing.T) {
	p, err := New()
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer p.Close()

	const docs = 8
	const items = 200
	jsonStr := `{"items":[` + strings.Repeat(`{"id":7},`, items-1) + `{"id":7}]}`

	var wg sync.WaitGroup
	errs := make(chan error, docs)
	for g := 0; g < docs; g++ {
		wg.Add(1)
		go func(releaseEvery int) {
			defer wg.Done()
			i := 0
			err := p.ForeachWithError(jsonStr, "items", func(key any, item *IterableValue) error {
				if got := item.GetInt("id"); got != 7 {
					return fmt.Errorf("item %v: id = %d, want 7", key, got)
				}
				if releaseEvery > 0 && i%releaseEvery == 0 {
					item.Release()
					item.Release() // double Release must be a no-op, not a double Put
				}
				i++
				return nil
			})
			if err != nil {
				errs <- err
			}
		}(g) // g == 0: never release; the others release every g-th item
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}
}

// TestGEN001Review_SkipValidationZeroCacheKeyInert is a regression test for a
// defect the P1 review caught in the SkipValidation wiring: under
// SkipValidation, validateAndCacheKey returns the zero CacheKey while
// EnableCache may still be true — PreParse's cache read/write on that zero
// key would collide every skip-mode document onto one entry and serve the
// wrong document's parse tree. The zero key is now inert in every cache
// accessor, so skip-mode documents simply bypass the cache.
func TestGEN001Review_SkipValidationZeroCacheKeyInert(t *testing.T) {
	cfg := DefaultConfig()
	cfg.SkipValidation = true
	cfg.EnableCache = true
	p, err := New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() { _ = p.Close() }) // best-effort cleanup of the test processor

	docA := `{"v":"A","evil":"<script>a</script>"}`
	docB := `{"v":"B","evil":"<script>b</script>"}`

	for round := 0; round < 3; round++ {
		// PreParse + GetFromParsed must never cross-contaminate documents.
		pa, err := p.PreParse(docA)
		if err != nil {
			t.Fatalf("round %d PreParse(A): %v", round, err)
		}
		pb, err := p.PreParse(docB)
		if err != nil {
			t.Fatalf("round %d PreParse(B): %v", round, err)
		}
		if v, err := p.GetFromParsed(pa, "v"); err != nil || v != "A" {
			t.Fatalf("round %d GetFromParsed(A): v=%v err=%v, want A", round, v, err)
		}
		if v, err := p.GetFromParsed(pb, "v"); err != nil || v != "B" {
			t.Fatalf("round %d GetFromParsed(B): v=%v err=%v, want B", round, v, err)
		}

		// Direct Get and GetMultiple under skip must stay correct too.
		if v, err := p.Get(docA, "v"); err != nil || v != "A" {
			t.Fatalf("round %d Get(A): v=%v err=%v", round, v, err)
		}
		res, err := p.GetMultiple(docB, []string{"v"})
		if err != nil || res["v"] != "B" {
			t.Fatalf("round %d GetMultiple(B): res=%v err=%v", round, res, err)
		}
	}
}
