package json

import (
	"context"
	"sync"
	"sync/atomic"
)

// ============================================================================
// PARALLEL ITERATOR - Concurrent processing for CPU-bound operations
// PERFORMANCE: Parallelizes processing across multiple goroutines
// ============================================================================

// ParallelIterator processes arrays in parallel using worker goroutines
type ParallelIterator struct {
	data    []any
	workers int
	sem     chan struct{}
	done    chan struct{}
	closed  atomic.Bool
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
//
//	// Legacy pattern (backward compatible)
//	iter := json.NewParallelIteratorWithWorkers(data, 8)
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
		data:    data,
		workers: workers,
		sem:     make(chan struct{}, workers),
		done:    make(chan struct{}),
	}
}

// ForEach processes each element in parallel using the provided function
// The function receives the index and value of each element
// Returns the first error encountered, or nil if all operations succeed
func (it *ParallelIterator) ForEach(fn func(int, any) error) error {
	return it.ForEachWithContext(context.Background(), fn)
}

// ForEachWithContext processes each element in parallel with context support for cancellation
// The function receives the index and value of each element
// Returns the first error encountered, or ctx.Err() if context is cancelled
// RESOURCE FIX: Added context support for graceful goroutine termination
func (it *ParallelIterator) ForEachWithContext(ctx context.Context, fn func(int, any) error) error {
	errCh := make(chan error, 1)
	var wg sync.WaitGroup
	var hasError int32

	for i, item := range it.data {
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

		go func(idx int, val any) {
			defer wg.Done()
			defer func() { <-it.sem }()

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

// ForEachBatch processes elements in batches in parallel
// Each batch is processed by a single goroutine
func (it *ParallelIterator) ForEachBatch(batchSize int, fn func(int, []any) error) error {
	return it.ForEachBatchWithContext(context.Background(), batchSize, fn)
}

// ForEachBatchWithContext processes elements in batches with context support for cancellation
// Each batch is processed by a single goroutine
// RESOURCE FIX: Added context support for graceful goroutine termination
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

	errCh := make(chan error, 1)
	var wg sync.WaitGroup
	var hasError int32

	for batchIdx, batch := range batches {
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

		go func(idx int, b []any) {
			defer wg.Done()
			defer func() { <-it.sem }()

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

			if err := fn(idx, b); err != nil {
				if atomic.CompareAndSwapInt32(&hasError, 0, 1) {
					select {
					case errCh <- err:
					default:
					}
				}
			}
		}(batchIdx, batch)
	}

	wg.Wait()

	select {
	case err := <-errCh:
		return err
	default:
		return nil
	}
}

// Map applies a transformation function to each element in parallel
// Returns a new slice with the transformed values
func (it *ParallelIterator) Map(transform func(int, any) (any, error)) ([]any, error) {
	result := make([]any, len(it.data))
	var mu sync.Mutex

	err := it.ForEach(func(idx int, val any) error {
		transformed, err := transform(idx, val)
		if err != nil {
			return err
		}

		mu.Lock()
		result[idx] = transformed
		mu.Unlock()

		return nil
	})

	if err != nil {
		return nil, err
	}

	return result, nil
}

// Filter filters elements in parallel using a predicate function
// Returns a new slice with elements that pass the predicate
func (it *ParallelIterator) Filter(predicate func(int, any) bool) []any {
	var mu sync.Mutex
	result := make([]any, 0)

	it.ForEach(func(idx int, val any) error {
		if predicate(idx, val) {
			mu.Lock()
			result = append(result, val)
			mu.Unlock()
		}
		return nil
	})

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
