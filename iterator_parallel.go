package json

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
)

// ============================================================================
// PARALLEL ITERATOR - Concurrent processing for CPU-bound operations
// PERFORMANCE: Parallelizes processing across multiple goroutines
// ============================================================================

// ParallelIterator processes arrays in parallel using worker goroutines
type ParallelIterator struct {
	data   []any
	sem    chan struct{}
	done   chan struct{}
	closed atomic.Bool
}

// NewParallelIterator creates a new parallel iterator.
// The optional cfg parameter allows customization using the unified Config pattern.
// When config is provided, cfg.MaxConcurrency is used as the worker count.
//
// Example:
//
//	// Default settings (workers = 4)
//	iter := json.NewParallelIterator(data)
//
//	// With custom worker count
//	cfg := json.DefaultConfig()
//	cfg.MaxConcurrency = 8
//	iter := json.NewParallelIterator(data, cfg)
func NewParallelIterator(data []any, cfg ...Config) *ParallelIterator {
	var config Config
	if len(cfg) > 0 {
		config = cfg[0]
	} else {
		config = DefaultConfig()
	}

	workers := config.MaxConcurrency
	if workers <= 0 {
		workers = 4 // Default worker count
	}
	if workers > len(data) {
		workers = len(data)
		if workers == 0 {
			workers = 1
		}
	}
	return &ParallelIterator{
		data: data,
		sem:  make(chan struct{}, workers),
		done: make(chan struct{}),
	}
}

// ForEach processes each element in parallel using the provided function
// The function receives the index and value of each element
// Returns the first error encountered, or nil if all operations succeed
func (it *ParallelIterator) ForEach(fn func(int, any) error) error {
	return it.ForEachWithContext(context.Background(), fn)
}

// forEachParallel is the shared dispatch/worker skeleton behind
// ForEachWithContext (items = it.data, T = any) and ForEachBatchWithContext
// (items = batches, T = []any). Both public methods previously carried a
// verbatim copy of this loop — including the panic-recovery block — which
// would silently drift apart on any concurrency fix. panicLabel keeps each
// caller's historical panic message.
//
// Semantics (unchanged from the pre-dedup implementations):
//   - workers stop early once any error is recorded (hasError gate)
//   - ctx cancellation returns ctx.Err(); iterator Close returns nil
//   - a callback panic is converted to an error instead of crashing
func forEachParallel[T any](ctx context.Context, it *ParallelIterator, items []T, fn func(idx int, val T) error, panicLabel string) error {
	errCh := make(chan error, 1)
	var wg sync.WaitGroup
	var hasError int32

	for i, item := range items {
		// Check context cancellation and close signal
		select {
		case <-ctx.Done():
			wg.Wait()
			return ctx.Err()
		case <-it.done:
			wg.Wait()
			return nil
		default:
		}

		// Check if we already have an error
		if atomic.LoadInt32(&hasError) == 1 {
			break
		}

		select {
		case it.sem <- struct{}{}: // Acquire semaphore
		case <-ctx.Done():
			wg.Wait()
			return ctx.Err()
		case <-it.done:
			wg.Wait()
			return nil
		}
		wg.Add(1)

		go func(idx int, val T) {
			defer wg.Done()
			defer func() { <-it.sem }()
			// SAFETY (SEC-003): a panic inside the user callback fn must not tear
			// down the process; convert it to an error reported to the caller.
			// Mirrors the JSONL worker recovery in processor_streamjsonl.go.
			defer func() {
				if r := recover(); r != nil {
					if atomic.CompareAndSwapInt32(&hasError, 0, 1) {
						select {
						case errCh <- fmt.Errorf("%s panicked: %v", panicLabel, r):
						default:
						}
					}
				}
			}()

			select {
			case <-ctx.Done():
				return
			case <-it.done:
				return
			default:
			}

			if atomic.LoadInt32(&hasError) == 1 {
				return
			}

			if err := fn(idx, val); err != nil {
				if atomic.CompareAndSwapInt32(&hasError, 0, 1) {
					select {
					case errCh <- err:
					default:
					}
				}
			}
		}(i, item)
	}

	wg.Wait()

	select {
	case err := <-errCh:
		return err
	default:
		return nil
	}
}

// ForEachWithContext processes each element in parallel with context support for cancellation
// The function receives the index and value of each element
// Returns the first error encountered, or ctx.Err() if context is cancelled
// RESOURCE FIX: Added context support for graceful goroutine termination
func (it *ParallelIterator) ForEachWithContext(ctx context.Context, fn func(int, any) error) error {
	return forEachParallel(ctx, it, it.data, fn, "parallel iterator worker")
}

// ForEachBatch processes elements in batches in parallel
// Each batch is processed by a single goroutine
//
// Errors:
//   - the first error returned by fn across all batches
//   - nil if all batches succeed, or if the iterator has been closed
func (it *ParallelIterator) ForEachBatch(batchSize int, fn func(int, []any) error) error {
	return it.ForEachBatchWithContext(context.Background(), batchSize, fn)
}

// ForEachBatchWithContext processes elements in batches with context support for cancellation
// Each batch is processed by a single goroutine
// RESOURCE FIX: Added context support for graceful goroutine termination
//
// Errors:
//   - the first error returned by fn across all batches
//   - ctx.Err() if the context is cancelled before completion
//   - nil if all batches succeed, or if the iterator has been closed
func (it *ParallelIterator) ForEachBatchWithContext(ctx context.Context, batchSize int, fn func(int, []any) error) error {
	if batchSize <= 0 {
		batchSize = 100
	}

	// Create batches
	batches := make([][]any, 0)
	for i := 0; i < len(it.data); i += batchSize {
		end := i + batchSize
		if end > len(it.data) {
			end = len(it.data)
		}
		batches = append(batches, it.data[i:end])
	}

	return forEachParallel(ctx, it, batches, fn, "parallel iterator batch worker")
}

// Map applies a transformation function to each element in parallel
// Returns a new slice with the transformed values
//
// Errors:
//   - the first error returned by transform (returned with a nil slice)
//   - nil if all elements transform successfully
func (it *ParallelIterator) Map(transform func(int, any) (any, error)) ([]any, error) {
	result := make([]any, len(it.data))

	err := it.ForEach(func(idx int, val any) error {
		transformed, err := transform(idx, val)
		if err != nil {
			return err
		}

		// Each worker writes a distinct index; concurrent writes to distinct
		// slice elements do not race (separate memory locations). ForEach
		// joins all workers via wg.Wait before returning, so every write is
		// visible to the caller — no mutex required.
		result[idx] = transformed

		return nil
	})

	if err != nil {
		return nil, err
	}

	return result, nil
}

// Filter filters elements in parallel using a predicate function.
// Returns a new slice with elements that pass the predicate, preserving the
// input order (results land in preallocated slots keyed by index, like Map —
// appending under a mutex would return completion order instead).
func (it *ParallelIterator) Filter(predicate func(int, any) bool) []any {
	slots := make([]any, len(it.data))
	kept := make([]bool, len(it.data))

	if err := it.ForEach(func(idx int, val any) error {
		if predicate(idx, val) {
			slots[idx] = val
			kept[idx] = true
		}
		return nil
	}); err != nil {
		// SEC-003: ForEach returns a non-nil error only when a worker recovered a
		// panic (see the recover in ForEachWithContext). Filter has no error return,
		// so log rather than silently swallow — keeps the panic visible without
		// crashing the program.
		slog.Error("parallel filter recovered a panic", slog.Any("error", err))
	}

	result := make([]any, 0, len(slots))
	for i, k := range kept {
		if k {
			result = append(result, slots[i])
		}
	}
	return result
}

// Close releases resources associated with the ParallelIterator.
// Signals any running goroutines to stop and waits for them to finish.
// Safe to call from multiple goroutines — uses CAS to prevent double-close panic.
func (it *ParallelIterator) Close() {
	if it.closed.CompareAndSwap(false, true) {
		close(it.done)
	}
}
