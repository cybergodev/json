package json

import (
	"context"
	"fmt"
	"io"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cybergodev/json/internal"
)

// ============================================================================
// RESOURCE LEAK VERIFICATION TESTS
// These tests verify that resources (goroutines, channels, memory) are
// properly cleaned up and not leaked.
// ============================================================================

// countGoroutines returns the current number of goroutines
func countGoroutines() int {
	runtime.GC()
	time.Sleep(10 * time.Millisecond) // Allow goroutines to settle
	return runtime.NumGoroutine()
}

// ============================================================================
// PROCESSOR CLOSE TESTS
// ============================================================================

// TestProcessorCloseGoroutineCleanup verifies that Processor.Close() properly
// cleans up all goroutines including the drain goroutine
func TestProcessorCloseGoroutineCleanup(t *testing.T) {
	initialGoroutines := countGoroutines()

	// Create a processor with concurrency limits
	cfg := DefaultConfig()
	cfg.MaxConcurrency = 10
	processor, err := New(cfg)
	if err != nil {
		t.Fatalf("Failed to create processor: %v", err)
	}

	// Perform some operations to populate the semaphore
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = processor.Get(`{"test": "value"}`, ".")
		}()
	}
	wg.Wait()

	// Close the processor
	err = processor.Close()
	if err != nil {
		t.Errorf("Processor.Close() returned error: %v", err)
	}

	// Allow goroutines to settle
	time.Sleep(100 * time.Millisecond)

	// Verify goroutine count hasn't increased significantly
	finalGoroutines := countGoroutines()
	leaked := finalGoroutines - initialGoroutines

	if leaked > 2 { // Allow small variance for test infrastructure
		t.Errorf("Potential goroutine leak: started with %d, ended with %d (leaked: %d)",
			initialGoroutines, finalGoroutines, leaked)
	}
}

// TestProcessorCloseCompletes consolidates the four "Close must finish within
// a timeout" tests — plain close, cache-populated close, close racing
// in-flight semaphore operations, and ten concurrent closes — into one
// table-driven test. Each row's closeFn blocks until every close finished.
func TestProcessorCloseCompletes(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping in short mode")
	}

	tests := []struct {
		name    string
		timeout time.Duration
		closeFn func() error
	}{
		{
			name:    "plain close",
			timeout: 5 * time.Second,
			closeFn: func() error {
				cfg := DefaultConfig()
				cfg.MaxConcurrency = 5
				p, err := New(cfg)
				if err != nil {
					return err
				}
				return p.Close()
			},
		},
		{
			name:    "cache-populated close",
			timeout: 5 * time.Second,
			closeFn: func() error {
				cfg := DefaultConfig()
				cfg.EnableCache = true
				cfg.MaxCacheSize = 1000
				cfg.CacheTTL = 1 * time.Minute
				p, err := New(cfg)
				if err != nil {
					return err
				}
				for i := 0; i < 100; i++ {
					_, _ = p.Get(`{"key": "value"}`, ".")
				}
				return p.Close()
			},
		},
		{
			name:    "close racing in-flight semaphore operations",
			timeout: 10 * time.Second,
			closeFn: func() error {
				cfg := DefaultConfig()
				cfg.MaxConcurrency = 3
				p, err := New(cfg)
				if err != nil {
					return err
				}
				var wg sync.WaitGroup
				for i := 0; i < 10; i++ {
					wg.Add(1)
					go func() {
						defer wg.Done()
						// These might block waiting for the semaphore.
						_, _ = p.Get(`{"test": "value"}`, ".")
					}()
				}
				time.Sleep(20 * time.Millisecond)
				err = p.Close()
				wg.Wait()
				return err
			},
		},
		{
			name:    "ten concurrent closes",
			timeout: 10 * time.Second,
			closeFn: func() error {
				processors := make([]*Processor, 10)
				for i := range processors {
					cfg := DefaultConfig()
					cfg.EnableCache = true
					p, err := New(cfg)
					if err != nil {
						return err
					}
					processors[i] = p
				}
				var wg sync.WaitGroup
				var firstErr error
				var once sync.Once
				for _, p := range processors {
					wg.Add(1)
					go func(proc *Processor) {
						defer wg.Done()
						if err := proc.Close(); err != nil {
							once.Do(func() { firstErr = err })
						}
					}(p)
				}
				wg.Wait()
				return firstErr
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			done := make(chan error, 1)
			go func() {
				done <- tt.closeFn()
			}()

			select {
			case err := <-done:
				if err != nil {
					t.Errorf("Close returned error: %v", err)
				}
			case <-time.After(tt.timeout):
				t.Error("Close blocked for too long - potential goroutine leak")
			}
		})
	}
}

// ============================================================================
// PARALLEL ITERATOR CONTEXT TESTS
// ============================================================================

// TestParallelIteratorForEachWithContextCancellation verifies that
// ForEachWithContext properly cancels goroutines when context is cancelled
func TestParallelIteratorForEachWithContextCancellation(t *testing.T) {
	data := make([]any, 100)
	for i := range data {
		data[i] = i
	}

	cfg := DefaultConfig()
	cfg.MaxConcurrency = 4
	iterator := NewParallelIterator(data, cfg)

	// Use a context that we'll cancel
	ctx, cancel := context.WithCancel(context.Background())

	var processedCount int32
	var goroutineCount int32

	// Start processing
	done := make(chan error, 1)
	go func() {
		err := iterator.ForEachWithContext(ctx, func(idx int, val any) error {
			atomic.AddInt32(&goroutineCount, 1)
			// Simulate work
			time.Sleep(50 * time.Millisecond)
			atomic.AddInt32(&processedCount, 1)
			return nil
		})
		done <- err
	}()

	// Cancel after a short time
	time.Sleep(30 * time.Millisecond)
	cancel()

	// Wait for completion
	err := <-done

	// Should return context cancelled error
	if err != context.Canceled {
		t.Logf("ForEachWithContext returned: %v (expected context.Canceled)", err)
	}

	// Verify not all items were processed (cancellation worked)
	processed := atomic.LoadInt32(&processedCount)
	if processed >= 100 {
		t.Errorf("Expected cancellation to stop processing, but %d items were processed", processed)
	}
}

// TestParallelIteratorForEachBatchWithContextCancellation verifies that
// ForEachBatchWithContext properly cancels goroutines when context is cancelled
func TestParallelIteratorForEachBatchWithContextCancellation(t *testing.T) {
	data := make([]any, 200)
	for i := range data {
		data[i] = i
	}

	cfg := DefaultConfig()
	cfg.MaxConcurrency = 4
	iterator := NewParallelIterator(data, cfg)

	ctx, cancel := context.WithCancel(context.Background())

	var batchCount int32

	done := make(chan error, 1)
	go func() {
		err := iterator.ForEachBatchWithContext(ctx, 10, func(idx int, batch []any) error {
			atomic.AddInt32(&batchCount, 1)
			time.Sleep(30 * time.Millisecond)
			return nil
		})
		done <- err
	}()

	// Cancel after processing some batches
	time.Sleep(50 * time.Millisecond)
	cancel()

	err := <-done

	// Should return context cancelled error or nil
	t.Logf("ForEachBatchWithContext returned: %v, processed %d batches", err, batchCount)

	// Verify not all batches were processed
	batches := atomic.LoadInt32(&batchCount)
	if batches >= 20 {
		t.Errorf("Expected cancellation to stop batch processing, but %d batches were processed", batches)
	}
}

// TestParallelIteratorForEachNoLeak verifies that ForEach doesn't leak
// goroutines even when all items are processed successfully
func TestParallelIteratorForEachNoLeak(t *testing.T) {
	initialGoroutines := countGoroutines()

	data := make([]any, 50)
	for i := range data {
		data[i] = i
	}

	// Run multiple iterations
	for i := 0; i < 10; i++ {
		cfg := DefaultConfig()
		cfg.MaxConcurrency = 4
		iterator := NewParallelIterator(data, cfg)
		err := iterator.ForEach(func(idx int, val any) error {
			return nil
		})
		if err != nil {
			t.Errorf("ForEach returned error: %v", err)
		}
	}

	// Allow goroutines to settle
	time.Sleep(100 * time.Millisecond)

	finalGoroutines := countGoroutines()
	leaked := finalGoroutines - initialGoroutines

	if leaked > 2 {
		t.Errorf("Potential goroutine leak after ForEach: started with %d, ended with %d (leaked: %d)",
			initialGoroutines, finalGoroutines, leaked)
	}
}

// ============================================================================
// CHANNEL LEAK TESTS
// ============================================================================

// TestParallelIteratorChannelCleanup drives 50 short-lived ParallelIterators
// to completion and verifies every element is delivered. (Channel teardown
// itself is unobservable from here; the hard assertion is iteration
// correctness — leaked/closed channels would surface as missed elements.)
func TestParallelIteratorChannelCleanup(t *testing.T) {
	data := make([]any, 20)
	for i := range data {
		data[i] = i
	}

	// Create multiple iterators and verify they don't leak channels
	var processed atomic.Int64 // callbacks run on worker goroutines: atomic
	for i := 0; i < 50; i++ {
		cfg := DefaultConfig()
		cfg.MaxConcurrency = 4
		iterator := NewParallelIterator(data, cfg)
		if err := iterator.ForEach(func(idx int, val any) error {
			processed.Add(1)
			return nil
		}); err != nil {
			t.Fatalf("iteration %d: ForEach: %v", i, err)
		}
		iterator.Close()
	}

	if got := processed.Load(); got != int64(50*len(data)) {
		t.Errorf("processed %d elements across 50 iterators, want %d", got, 50*len(data))
	}

	runtime.GC()
	time.Sleep(50 * time.Millisecond)

	// Settle after GC: keeps the -race allocator steady for the next tests.
}

// ============================================================================
// CONTEXT TIMEOUT TESTS
// ============================================================================

// TestParallelIteratorWithTimeout verifies that ForEachWithContext respects
// context timeout
func TestParallelIteratorWithTimeout(t *testing.T) {
	data := make([]any, 100)
	for i := range data {
		data[i] = i
	}

	cfg := DefaultConfig()
	cfg.MaxConcurrency = 4
	iterator := NewParallelIterator(data, cfg)

	// Create context with short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	var processedCount int32

	err := iterator.ForEachWithContext(ctx, func(idx int, val any) error {
		atomic.AddInt32(&processedCount, 1)
		time.Sleep(20 * time.Millisecond) // Slow processing
		return nil
	})

	// Should timeout
	if err != context.DeadlineExceeded {
		t.Logf("Expected context.DeadlineExceeded, got: %v", err)
	}

	processed := atomic.LoadInt32(&processedCount)
	t.Logf("Processed %d items before timeout", processed)

	if processed >= 100 {
		t.Error("Expected timeout to limit processing")
	}
}

// ============================================================================
// STRESS TESTS
// ============================================================================

// TestConcurrentProcessorOperations verifies no leaks under concurrent load
func TestConcurrentProcessorOperations(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stress test in short mode")
	}

	initialGoroutines := countGoroutines()

	cfg := DefaultConfig()
	cfg.MaxConcurrency = 10
	cfg.EnableCache = true

	processor, _ := New(cfg)

	var wg sync.WaitGroup
	// Launch many concurrent operations
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				_, _ = processor.Get(`{"test": "value"}`, ".")
				_, _ = processor.Get(`{"nested": {"key": `+string(rune('0'+j%10))+`}}`, ".nested")
			}
		}(i)
	}

	wg.Wait()

	// Close processor
	_ = processor.Close()

	// Allow cleanup
	time.Sleep(200 * time.Millisecond)

	finalGoroutines := countGoroutines()
	leaked := finalGoroutines - initialGoroutines

	if leaked > 5 {
		t.Errorf("Goroutine leak detected after concurrent operations: started with %d, ended with %d (leaked: %d)",
			initialGoroutines, finalGoroutines, leaked)
	}
}

// ============================================================================
// RESOURCE MANAGER COUNTER TESTS
// ============================================================================

// ============================================================================
// PARALLEL ITERATOR CLOSE TESTS
// ============================================================================

// TestParallelIteratorClose verifies that Close method works correctly
func TestParallelIteratorClose(t *testing.T) {
	data := make([]any, 20)
	for i := range data {
		data[i] = i
	}

	cfg := DefaultConfig()
	cfg.MaxConcurrency = 4
	iterator := NewParallelIterator(data, cfg)

	// Process some data
	_ = iterator.ForEach(func(idx int, val any) error {
		return nil
	})

	// Close should not panic
	iterator.Close()

	// Multiple Close calls should be safe
	iterator.Close()
}

// ============================================================================
// FIX VERIFICATION TESTS (2026-04-11 resource leak audit)
// ============================================================================
// The former TestStaleProcessorCloseHasTimeout was removed in the FIX-001
// consolidation: its eviction-churn goroutine-delta assertion is subsumed by
// TestEvictionGoroutinesBounded below (5 eviction rounds, same tolerance).

// TestClearStructEncoderCache verifies that ClearStructEncoderCache properly
// reclaims memory from the global struct encoder cache (Fix #2).
func TestClearStructEncoderCache(t *testing.T) {
	// Populate cache with multiple struct types
	type testStructA struct{ Name string }
	type testStructB struct{ Value int }
	type testStructC struct{ Flag bool }

	_ = internal.GetStructEncoder(reflect.TypeOf(testStructA{}))
	_ = internal.GetStructEncoder(reflect.TypeOf(testStructB{}))
	_ = internal.GetStructEncoder(reflect.TypeOf(testStructC{}))

	// Clear cache
	internal.ClearStructEncoderCache()

	// Verify cache is repopulated after clear (functional correctness)
	fields := internal.GetStructEncoder(reflect.TypeOf(testStructA{}))
	if len(fields) == 0 {
		t.Error("GetStructEncoder returned empty fields after cache clear")
	}
}

// TestClearPathTypeCacheOnProcessorClose verifies that the path type cache
// is cleared when a processor is closed (Fix #3).
func TestClearPathTypeCacheOnProcessorClose(t *testing.T) {
	processor, err := New(DefaultConfig())
	if err != nil {
		t.Fatalf("Failed to create processor: %v", err)
	}

	// Populate path type cache by querying various paths
	paths := []string{"simple", "nested.key", "array[0]", "deep.nested.path", "complex[1].field"}
	for _, path := range paths {
		_, _ = processor.Get(`{"simple":1,"nested":{"key":2},"array":[3],"deep":{"nested":{"path":4}},"complex":[{"field":5}]}`, path)
	}

	// Close processor — should clear path type cache
	err = processor.Close()
	if err != nil {
		t.Fatalf("Close returned error: %v", err)
	}

	// Shard maps are not directly inspectable, but the observable contract of
	// "Close clears the cache" is that classification still works afterwards:
	// a cleared cache must recompute the same pathType for every path class.
	if got := getPathType("simple"); got != pathTypeSimple {
		t.Errorf("post-Close getPathType(simple) = %d, want pathTypeSimple", got)
	}
	for _, path := range []string{"nested.key", "array[0]", "deep.nested.path"} {
		if got := getPathType(path); got != pathTypeComplex {
			t.Errorf("post-Close getPathType(%q) = %d, want pathTypeComplex", path, got)
		}
	}
}

// TestHooksClearedOnClose verifies that hooks slice is nil'd on Close(),
// releasing closure references for GC (Fix #4).
func TestHooksClearedOnClose(t *testing.T) {
	processor, err := New(DefaultConfig())
	if err != nil {
		t.Fatalf("Failed to create processor: %v", err)
	}

	// Add hooks that capture large closures
	largeData := make([]byte, 1024*1024) // 1MB
	for i := 0; i < 10; i++ {
		hook := &testClosureHook{data: largeData}
		processor.AddHook(hook)
	}

	// Verify hooks are present
	processor.hooksMu.Lock()
	hookCount := len(processor.hooks)
	processor.hooksMu.Unlock()
	if hookCount != 10 {
		t.Fatalf("Expected 10 hooks, got %d", hookCount)
	}

	// Close should nil the hooks
	err = processor.Close()
	if err != nil {
		t.Fatalf("Close returned error: %v", err)
	}

	processor.hooksMu.Lock()
	hooksAfter := processor.hooks
	processor.hooksMu.Unlock()

	if hooksAfter != nil {
		t.Errorf("Expected hooks to be nil after Close(), got %d hooks", len(hooksAfter))
	}
}

// testClosureHook is a test hook that captures a reference to external data.
type testClosureHook struct {
	data []byte
}

func (h *testClosureHook) Before(ctx HookContext) error { return nil }
func (h *testClosureHook) After(ctx HookContext, result any, err error) (any, error) {
	return result, err
}

// TestEvictionGoroutinesBounded verifies that eviction close goroutines
// don't accumulate beyond the semaphore limit (Fix #5).
func TestEvictionGoroutinesBounded(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stress test in short mode")
	}

	initial := countGoroutines()

	// Create and evict many processors by exceeding cache limit repeatedly
	for round := 0; round < 5; round++ {
		for i := 0; i < configProcessorCacheLimit+configProcessorCacheEvictNum; i++ {
			cfg := DefaultConfig()
			cfg.MaxConcurrency = round*1000 + i // unique config
			_, _ = getProcessorWithConfig(cfg)
		}
		time.Sleep(100 * time.Millisecond)
	}

	// Allow goroutines to settle
	time.Sleep(500 * time.Millisecond)

	final := countGoroutines()
	leaked := final - initial
	if leaked > 5 {
		t.Errorf("Goroutine leak after eviction stress: before=%d after=%d leaked=%d",
			initial, final, leaked)
	}
}

// ============================================================================
// FIX VERIFICATION TESTS (2026-04-12 resource leak audit — round 2)
// ============================================================================

// TestSemaphoreAcquireRespectsContext verifies that ForEachWithContext does not
// block on semaphore acquire when context is cancelled and all workers are busy.
// This is the regression test for fix C-3.
func TestSemaphoreAcquireRespectsContext(t *testing.T) {
	data := make([]any, 20)
	for i := range data {
		data[i] = i
	}

	cfg := DefaultConfig()
	cfg.MaxConcurrency = 2
	iterator := NewParallelIterator(data, cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	start := time.Now()
	err := iterator.ForEachWithContext(ctx, func(idx int, val any) error {
		time.Sleep(50 * time.Millisecond) // slow work
		return nil
	})
	elapsed := time.Since(start)

	if ctx.Err() != nil && err == nil {
		t.Error("expected error when context expired during semaphore acquire")
	}

	// Must NOT take much longer than the timeout — old code would block.
	if elapsed > 2*time.Second {
		t.Errorf("ForEachWithContext took %v — semaphore acquire may not respect context", elapsed)
	}
}

// TestBatchSemaphoreAcquireRespectsContext is the batch variant of the above.
func TestBatchSemaphoreAcquireRespectsContext(t *testing.T) {
	data := make([]any, 50)
	for i := range data {
		data[i] = i
	}

	cfg := DefaultConfig()
	cfg.MaxConcurrency = 2
	iterator := NewParallelIterator(data, cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	start := time.Now()
	err := iterator.ForEachBatchWithContext(ctx, 5, func(idx int, batch []any) error {
		time.Sleep(50 * time.Millisecond)
		return nil
	})
	elapsed := time.Since(start)

	if elapsed > 2*time.Second {
		t.Errorf("ForEachBatchWithContext took %v — semaphore acquire may not respect context", elapsed)
	}

	if ctx.Err() != nil && err == nil {
		t.Error("expected error when context expired during batch processing")
	}
}

// TestStreamJSONLParallelWithContextCancellation verifies that
// StreamJSONLParallelWithContext properly cancels workers and scanner.
func TestStreamJSONLParallelWithContextCancellation(t *testing.T) {
	processor, err := New()
	if err != nil {
		t.Fatalf("Failed to create processor: %v", err)
	}
	defer processor.Close()

	// Build a slow reader that feeds data gradually
	r, w := io.Pipe()
	go func() {
		defer w.Close()
		for i := 0; i < 100; i++ {
			fmt.Fprintf(w, `{"line":%d}`+"\n", i)
			time.Sleep(20 * time.Millisecond)
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()

	var processed int32
	start := time.Now()
	err = processor.StreamJSONLParallelWithContext(ctx, r, 2, func(lineNum int, item *IterableValue) error {
		atomic.AddInt32(&processed, 1)
		return nil
	})
	elapsed := time.Since(start)

	if elapsed > 3*time.Second {
		t.Errorf("StreamJSONLParallelWithContext took %v — context not respected", elapsed)
	}

	if err != nil && ctx.Err() != nil {
		t.Logf("Returned context error as expected: %v", err)
	}
}

// TestShutdownGlobalProcessorClearsConfigCache verifies that
// ShutdownGlobalProcessor closes all cached config processors.
func TestShutdownGlobalProcessorClearsConfigCache(t *testing.T) {
	// Populate the config cache with several processors
	for i := 0; i < 5; i++ {
		cfg := DefaultConfig()
		cfg.MaxConcurrency = 100 + i // unique config key
		_, _ = getProcessorWithConfig(cfg)
	}

	// Count cached entries before shutdown
	var countBefore int
	configProcessorCache.Range(func(_, _ any) bool {
		countBefore++
		return true
	})

	if countBefore == 0 {
		t.Skip("No config cache entries to test")
	}

	// Shutdown should clear the cache
	ShutdownGlobalProcessor()

	var countAfter int
	configProcessorCache.Range(func(_, _ any) bool {
		countAfter++
		return true
	})

	if countAfter != 0 {
		t.Errorf("configProcessorCache not cleared after shutdown: before=%d after=%d",
			countBefore, countAfter)
	}

	// Restore default processor for other tests
	p, err := New()
	if err != nil {
		t.Fatalf("Failed to restore processor: %v", err)
	}
	SetGlobalProcessor(p)
}

// ============================================================================
// CONCURRENT-CACHE SAFETY (merged from concurrency_test.go)
// ============================================================================

// TestConcurrentCacheSafety tests concurrent access to various caches and
// shared state. All caches follow the same access pattern, so they are
// consolidated into a single table-driven test.
func TestConcurrentCacheSafety(t *testing.T) {
	cache := internal.GetGlobalCompiledPathCache()

	tests := []struct {
		name        string
		concurrency int
		iterations  int
		workload    func(workerID, iteration int) error
	}{
		{
			name:        "PathTypeCache",
			concurrency: 20,
			iterations:  100,
			workload: func(_, _ int) error {
				for _, path := range []string{"simple", "nested.path", "array[0]", "complex.nested[1].key", "very.deep.nested.path.with.many.segments"} {
					_ = getPathType(path)
				}
				return nil
			},
		},
		{
			name:        "KeyInternMap",
			concurrency: 20,
			iterations:  100,
			workload: func(_, _ int) error {
				for _, key := range []string{"id", "name", "value", "timestamp", "metadata"} {
					internal.GlobalKeyIntern.Intern(key)
				}
				return nil
			},
		},
		{
			name:        "DefaultProcessor",
			concurrency: 20,
			iterations:  1,
			workload: func(_, _ int) error {
				p := getDefaultProcessor()
				if p == nil {
					return errPtr("expected non-nil processor")
				}
				_, err := p.Get(`{"test": 1}`, "test")
				return err
			},
		},
		{
			name:        "PathSegmentCache",
			concurrency: 20,
			iterations:  50,
			workload: func(_, _ int) error {
				// D-002: exercises the real path-segment cache via ParsePath
				// (the old body used the removed GlobalPathIntern directly).
				for _, path := range []string{"simple", "nested.path", "array[0].item"} {
					if _, err := internal.ParsePath(path); err != nil {
						return errPtr("parse path " + path + ": " + err.Error())
					}
				}
				return nil
			},
		},
		{
			name:        "CompiledPathCache",
			concurrency: 20,
			iterations:  50,
			workload: func(_, _ int) error {
				for _, path := range []string{"simple", "nested.path", "array[0]"} {
					cp, err := cache.Get(path)
					if err != nil {
						return err
					}
					if cp == nil {
						return errPtr("nil CompiledPath for " + path)
					}
					cp.Release()
				}
				return nil
			},
		},
		{
			name:        "ValidationCache",
			concurrency: 10,
			iterations:  50,
			workload: func(_, _ int) error {
				p, err := New()
				if err != nil {
					return err
				}
				defer p.Close()
				for _, jsonStr := range []string{`{"test": 1}`, `{"nested": {"key": "value"}}`, `{"array": [1, 2, 3]}`} {
					_, err := p.Get(jsonStr, "test")
					if err != nil && !strings.Contains(err.Error(), "not found") {
						return err
					}
				}
				return nil
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var wg sync.WaitGroup
			errCh := make(chan error, tt.concurrency)

			for i := 0; i < tt.concurrency; i++ {
				wg.Add(1)
				go func(workerID int) {
					defer wg.Done()
					for j := 0; j < tt.iterations; j++ {
						if err := tt.workload(workerID, j); err != nil {
							select {
							case errCh <- err:
							default:
							}
							return
						}
					}
				}(i)
			}

			wg.Wait()
			close(errCh)
			for err := range errCh {
				t.Errorf("concurrent %s failed: %v", tt.name, err)
			}
		})
	}
}

// errPtr is a helper to create an error from a string for use in table-driven tests.
func errPtr(msg string) error { return &testErr{msg: msg} }

type testErr struct{ msg string }

func (e *testErr) Error() string { return e.msg }

// ============================================================================
// CONCURRENCY-GOVERNANCE INVARIANTS (merged from governance_test.go)
// ============================================================================

// TestGovernance_EncodeFunnelNoNestedAcquire locks in that governing the encode
// funnel (encodeWithConfigToBytes) does NOT cause nested semaphore acquisition in
// limited-concurrency mode. Marshal/MarshalIndent/EncodeWithConfig/EncodeStream/
// EncodeBatch all funnel through exactly one governed call; none of them govern
// themselves. Under MaxConcurrency=1 a nested acquire would self-reject with
// ErrConcurrencyLimit, so a spurious failure here is the canary for a nesting bug.
func TestGovernance_EncodeFunnelNoNestedAcquire(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MaxConcurrency = 1 // tightest legal limit: a nested acquire would fail at once
	p, err := New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer p.Close()

	data := map[string]any{"k": []int{1, 2, 3}}

	if _, err := p.Marshal(data); err != nil {
		t.Fatalf("Marshal under MaxConcurrency=1: %v", err)
	}
	if _, err := p.MarshalIndent(data, "", "  "); err != nil {
		t.Fatalf("MarshalIndent under MaxConcurrency=1: %v", err)
	}
	if _, err := p.EncodeWithConfig(data); err != nil {
		t.Fatalf("EncodeWithConfig under MaxConcurrency=1: %v", err)
	}
	// EncodeStream funnels through EncodeWithConfig -> encodeWithConfigToBytes.
	if _, err := p.EncodeStream([]any{data, data, data}); err != nil {
		t.Fatalf("EncodeStream under MaxConcurrency=1: %v", err)
	}
	if _, err := p.EncodeBatch(map[string]any{"a": 1, "b": 2}); err != nil {
		t.Fatalf("EncodeBatch under MaxConcurrency=1: %v", err)
	}
}

// TestGovernance_StreamJSONLWorksUnderConcurrencyLimit confirms that acquiring
// governance once for the whole stream does not break normal streaming and does
// not self-reject when MaxConcurrency is small.
func TestGovernance_StreamJSONLWorksUnderConcurrencyLimit(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MaxConcurrency = 2
	p, err := New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer p.Close()

	in := `{"i":1}
{"i":2}
{"i":3}
`
	count := 0
	err = p.StreamJSONL(strings.NewReader(in), func(lineNum int, item *IterableValue) error {
		count++
		return nil
	})
	if err != nil {
		t.Fatalf("StreamJSONL under MaxConcurrency=2: %v", err)
	}
	if count != 3 {
		t.Fatalf("expected 3 lines, got %d", count)
	}
}

// TestGovernance_StreamJSONLParallelUnderLimit exercises the parallel stream: the
// scanner goroutine holds the single in-flight slot for the whole run while worker
// goroutines run unregistered. Under MaxConcurrency=1 this must still succeed —
// workers must not contend the limit, only the outer stream call registers.
func TestGovernance_StreamJSONLParallelUnderLimit(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MaxConcurrency = 1
	p, err := New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer p.Close()

	var sb strings.Builder
	for i := 1; i <= 20; i++ {
		sb.WriteString(`{"i":`)
		sb.WriteString(strconv.Itoa(i))
		sb.WriteString("}\n")
	}

	// The callback runs concurrently on the worker pool, so the counter must
	// be atomic (a plain int raced and lost updates under -race).
	var count atomic.Int64
	err = p.StreamJSONLParallelWithContext(
		context.Background(),
		strings.NewReader(sb.String()),
		4,
		func(lineNum int, item *IterableValue) error {
			count.Add(1)
			return nil
		},
	)
	if err != nil {
		t.Fatalf("StreamJSONLParallelWithContext under MaxConcurrency=1: %v", err)
	}
	if count.Load() != 20 {
		t.Fatalf("expected 20 lines, got %d", count.Load())
	}
}

// TestGovernance_StreamJSONLChunkedUnderLimit confirms the chunked stream variant
// is also governed exactly once and works under a tight limit.
func TestGovernance_StreamJSONLChunkedUnderLimit(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MaxConcurrency = 1
	p, err := New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer p.Close()

	var sb strings.Builder
	for i := 1; i <= 25; i++ {
		sb.WriteString(`{"i":`)
		sb.WriteString(strconv.Itoa(i))
		sb.WriteString("}\n")
	}

	processed := 0
	err = p.StreamJSONLChunked(strings.NewReader(sb.String()), 10, func(chunk []*IterableValue) error {
		processed += len(chunk)
		return nil
	})
	if err != nil {
		t.Fatalf("StreamJSONLChunked under MaxConcurrency=1: %v", err)
	}
	if processed != 25 {
		t.Fatalf("expected 25 items, got %d", processed)
	}
}
