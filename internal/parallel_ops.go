package internal

import (
	"runtime"
	"sync"
	"sync/atomic"
)

// ============================================================================
// PARALLEL BATCH OPERATIONS
// Provides parallel processing for large JSON datasets
// PERFORMANCE: Scales with CPU cores for CPU-bound operations
// ============================================================================

// ParallelConfig holds configuration for parallel operations
type ParallelConfig struct {
	Workers     int // Number of worker goroutines
	BatchSize   int // Items per batch
	MinParallel int // Minimum items to trigger parallel processing
}

// DefaultParallelConfig returns the default parallel configuration
func DefaultParallelConfig() ParallelConfig {
	workers := runtime.NumCPU()
	if workers < 2 {
		workers = 2
	}
	if workers > 16 {
		workers = 16 // Cap at 16 workers
	}
	return ParallelConfig{
		Workers:     workers,
		BatchSize:   100,
		MinParallel: 1000,
	}
}

// ParallelProcessor handles parallel batch operations
type ParallelProcessor struct {
	config ParallelConfig
	pool   chan struct{} // Semaphore for limiting concurrent workers
}

// NewParallelProcessor creates a new parallel processor
func NewParallelProcessor(config ParallelConfig) *ParallelProcessor {
	if config.Workers <= 0 {
		config.Workers = runtime.NumCPU()
	}
	if config.BatchSize <= 0 {
		config.BatchSize = 100
	}
	if config.MinParallel <= 0 {
		config.MinParallel = 1000
	}

	return &ParallelProcessor{
		config: config,
		pool:   make(chan struct{}, config.Workers),
	}
}

// DefaultParallelProcessor is the default parallel processor
var DefaultParallelProcessor = NewParallelProcessor(DefaultParallelConfig())

// ============================================================================
// PARALLEL MAP OPERATIONS
// ============================================================================

// ParallelMapResult represents a result from parallel map processing
type ParallelMapResult struct {
	Key   string
	Value any
	Index int
}

// ParallelMap processes map entries in parallel
func (pp *ParallelProcessor) ParallelMap(m map[string]any, fn func(key string, value any) (any, error)) (map[string]any, error) {
	if len(m) < pp.config.MinParallel {
		// Process sequentially for small maps
		result := make(map[string]any, len(m))
		for k, v := range m {
			val, err := fn(k, v)
			if err != nil {
				return nil, err
			}
			result[k] = val
		}
		return result, nil
	}

	// Process in parallel
	result := make(map[string]any, len(m))
	var mu sync.Mutex
	var firstErr error

	// Collect keys for batching
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}

	var wg sync.WaitGroup
	var errCount int32

	// Process in batches
	for i := 0; i < len(keys); i += pp.config.BatchSize {
		end := i + pp.config.BatchSize
		if end > len(keys) {
			end = len(keys)
		}
		batch := keys[i:end]

		wg.Add(1)
		go func(batchKeys []string) {
			defer wg.Done()

			// Acquire semaphore
			pp.pool <- struct{}{}
			defer func() { <-pp.pool }()

			for _, k := range batchKeys {
				// Check if we should stop
				if atomic.LoadInt32(&errCount) > 0 {
					return
				}

				val, err := fn(k, m[k])
				if err != nil {
					if atomic.CompareAndSwapInt32(&errCount, 0, 1) {
						firstErr = err
					}
					return
				}

				mu.Lock()
				result[k] = val
				mu.Unlock()
			}
		}(batch)
	}

	wg.Wait()
	return result, firstErr
}

// ============================================================================
// PARALLEL SLICE OPERATIONS
// ============================================================================

// ParallelSliceResult represents a result from parallel slice processing
type ParallelSliceResult struct {
	Index int
	Value any
}

// ParallelSlice processes slice elements in parallel
func (pp *ParallelProcessor) ParallelSlice(arr []any, fn func(index int, value any) (any, error)) ([]any, error) {
	if len(arr) < pp.config.MinParallel {
		// Process sequentially for small slices
		result := make([]any, len(arr))
		for i, v := range arr {
			val, err := fn(i, v)
			if err != nil {
				return nil, err
			}
			result[i] = val
		}
		return result, nil
	}

	// Process in parallel
	result := make([]any, len(arr))
	var firstErr error
	var wg sync.WaitGroup
	var errCount int32

	// Process in batches
	for i := 0; i < len(arr); i += pp.config.BatchSize {
		end := i + pp.config.BatchSize
		if end > len(arr) {
			end = len(arr)
		}
		start := i

		wg.Add(1)
		go func(batchStart, batchEnd int) {
			defer wg.Done()

			// Acquire semaphore
			pp.pool <- struct{}{}
			defer func() { <-pp.pool }()

			for j := batchStart; j < batchEnd; j++ {
				// Check if we should stop
				if atomic.LoadInt32(&errCount) > 0 {
					return
				}

				val, err := fn(j, arr[j])
				if err != nil {
					if atomic.CompareAndSwapInt32(&errCount, 0, 1) {
						firstErr = err
					}
					return
				}
				result[j] = val
			}
		}(start, end)
	}

	wg.Wait()
	return result, firstErr
}

// ============================================================================
// PARALLEL FOREACH OPERATIONS
// ============================================================================

// ParallelForEach iterates over slice elements in parallel
// The function is called concurrently, ensure thread safety
func (pp *ParallelProcessor) ParallelForEach(arr []any, fn func(index int, value any) error) error {
	if len(arr) < pp.config.MinParallel {
		for i, v := range arr {
			if err := fn(i, v); err != nil {
				return err
			}
		}
		return nil
	}

	var wg sync.WaitGroup
	var firstErr error
	var errCount int32

	for i := 0; i < len(arr); i += pp.config.BatchSize {
		end := i + pp.config.BatchSize
		if end > len(arr) {
			end = len(arr)
		}
		start := i

		wg.Add(1)
		go func(batchStart, batchEnd int) {
			defer wg.Done()

			pp.pool <- struct{}{}
			defer func() { <-pp.pool }()

			for j := batchStart; j < batchEnd; j++ {
				if atomic.LoadInt32(&errCount) > 0 {
					return
				}

				if err := fn(j, arr[j]); err != nil {
					if atomic.CompareAndSwapInt32(&errCount, 0, 1) {
						firstErr = err
					}
					return
				}
			}
		}(start, end)
	}

	wg.Wait()
	return firstErr
}

// ParallelForEachMap iterates over map entries in parallel
func (pp *ParallelProcessor) ParallelForEachMap(m map[string]any, fn func(key string, value any) error) error {
	if len(m) < pp.config.MinParallel {
		for k, v := range m {
			if err := fn(k, v); err != nil {
				return err
			}
		}
		return nil
	}

	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}

	var wg sync.WaitGroup
	var firstErr error
	var errCount int32

	for i := 0; i < len(keys); i += pp.config.BatchSize {
		end := i + pp.config.BatchSize
		if end > len(keys) {
			end = len(keys)
		}
		batch := keys[i:end]

		wg.Add(1)
		go func(batchKeys []string) {
			defer wg.Done()

			pp.pool <- struct{}{}
			defer func() { <-pp.pool }()

			for _, k := range batchKeys {
				if atomic.LoadInt32(&errCount) > 0 {
					return
				}

				if err := fn(k, m[k]); err != nil {
					if atomic.CompareAndSwapInt32(&errCount, 0, 1) {
						firstErr = err
					}
					return
				}
			}
		}(batch)
	}

	wg.Wait()
	return firstErr
}

// ============================================================================
// WORKER POOL
// ============================================================================

// WorkerPool manages a pool of worker goroutines
type WorkerPool struct {
	tasks     chan func()
	workers   int
	wg        sync.WaitGroup
	stopChan  chan struct{}
	stopped   atomic.Bool
	taskCount int32      // Tracks the number of pending tasks
	doneCond  *sync.Cond // Condition variable for efficient waiting
}

// NewWorkerPool creates a new worker pool
func NewWorkerPool(workers int) *WorkerPool {
	if workers <= 0 {
		workers = runtime.NumCPU()
	}

	wp := &WorkerPool{
		tasks:    make(chan func(), workers*2),
		workers:  workers,
		stopChan: make(chan struct{}),
		doneCond: sync.NewCond(&sync.Mutex{}),
	}

	// Start workers
	for i := 0; i < workers; i++ {
		wp.wg.Add(1)
		go wp.worker()
	}

	return wp
}

// worker is the main worker loop
func (wp *WorkerPool) worker() {
	defer wp.wg.Done()

	for {
		select {
		case task := <-wp.tasks:
			task()
			// Decrement task count and signal if all tasks are done
			if atomic.AddInt32(&wp.taskCount, -1) == 0 {
				wp.doneCond.Broadcast()
			}
		case <-wp.stopChan:
			return
		}
	}
}

// Submit adds a task to the pool
func (wp *WorkerPool) Submit(task func()) {
	// Don't accept new tasks if pool is stopped
	if wp.stopped.Load() {
		return
	}

	// Increment task count before submitting
	atomic.AddInt32(&wp.taskCount, 1)

	select {
	case wp.tasks <- task:
		// Task submitted successfully
	default:
		// Channel is full, run synchronously (only if not stopped)
		if !wp.stopped.Load() {
			task()
			// Decrement task count since we ran it synchronously
			if atomic.AddInt32(&wp.taskCount, -1) == 0 {
				wp.doneCond.Broadcast()
			}
		} else {
			// Pool stopped, decrement the count we added
			if atomic.AddInt32(&wp.taskCount, -1) == 0 {
				wp.doneCond.Broadcast()
			}
		}
	}
}

// Stop stops the worker pool
func (wp *WorkerPool) Stop() {
	wp.stopped.Store(true)
	close(wp.stopChan)
	wp.wg.Wait()
}

// Wait waits for all tasks to complete
// PERFORMANCE: Uses condition variable instead of busy-wait for efficient CPU usage
func (wp *WorkerPool) Wait() {
	// Fast path: no tasks pending
	if atomic.LoadInt32(&wp.taskCount) <= 0 {
		return
	}

	// Wait for all tasks to complete using condition variable
	wp.doneCond.L.Lock()
	for atomic.LoadInt32(&wp.taskCount) > 0 {
		wp.doneCond.Wait()
	}
	wp.doneCond.L.Unlock()
}

// ============================================================================
// CHUNKED PROCESSING
// ============================================================================

// ChunkProcessor processes data in chunks for memory efficiency
type ChunkProcessor struct {
	chunkSize int
}

// NewChunkProcessor creates a new chunk processor
func NewChunkProcessor(chunkSize int) *ChunkProcessor {
	if chunkSize <= 0 {
		chunkSize = 1000
	}
	return &ChunkProcessor{chunkSize: chunkSize}
}

// ProcessSlice processes a slice in chunks
func (cp *ChunkProcessor) ProcessSlice(arr []any, fn func(chunk []any) error) error {
	for i := 0; i < len(arr); i += cp.chunkSize {
		end := i + cp.chunkSize
		if end > len(arr) {
			end = len(arr)
		}

		if err := fn(arr[i:end]); err != nil {
			return err
		}
	}
	return nil
}

// ProcessMap processes a map in chunks
func (cp *ChunkProcessor) ProcessMap(m map[string]any, fn func(chunk map[string]any) error) error {
	chunk := make(map[string]any, cp.chunkSize)
	count := 0

	for k, v := range m {
		chunk[k] = v
		count++

		if count >= cp.chunkSize {
			if err := fn(chunk); err != nil {
				return err
			}
			chunk = make(map[string]any, cp.chunkSize)
			count = 0
		}
	}

	// Process remaining items
	if len(chunk) > 0 {
		if err := fn(chunk); err != nil {
			return err
		}
	}

	return nil
}

// ============================================================================
// PARALLEL FILTER AND TRANSFORM
// ============================================================================

// ParallelFilter filters slice elements in parallel
func (pp *ParallelProcessor) ParallelFilter(arr []any, predicate func(value any) bool) []any {
	if len(arr) < pp.config.MinParallel {
		result := make([]any, 0)
		for _, v := range arr {
			if predicate(v) {
				result = append(result, v)
			}
		}
		return result
	}

	// Each goroutine collects its own results to avoid lock contention
	type localResult struct {
		values []any
	}

	results := make([]localResult, pp.config.Workers)
	for i := range results {
		results[i].values = make([]any, 0, len(arr)/pp.config.Workers)
	}

	var wg sync.WaitGroup
	chunkSize := (len(arr) + pp.config.Workers - 1) / pp.config.Workers

	for worker := 0; worker < pp.config.Workers; worker++ {
		start := worker * chunkSize
		if start >= len(arr) {
			break
		}
		end := start + chunkSize
		if end > len(arr) {
			end = len(arr)
		}

		wg.Add(1)
		go func(workerID, startIdx, endIdx int) {
			defer wg.Done()
			for i := startIdx; i < endIdx; i++ {
				if predicate(arr[i]) {
					results[workerID].values = append(results[workerID].values, arr[i])
				}
			}
		}(worker, start, end)
	}

	wg.Wait()

	// Combine results
	totalLen := 0
	for _, r := range results {
		totalLen += len(r.values)
	}

	combined := make([]any, 0, totalLen)
	for _, r := range results {
		combined = append(combined, r.values...)
	}

	return combined
}

// ParallelTransform transforms slice elements in parallel
func (pp *ParallelProcessor) ParallelTransform(arr []any, transform func(value any) any) []any {
	if len(arr) < pp.config.MinParallel {
		result := make([]any, len(arr))
		for i, v := range arr {
			result[i] = transform(v)
		}
		return result
	}

	result := make([]any, len(arr))
	var wg sync.WaitGroup

	chunkSize := (len(arr) + pp.config.Workers - 1) / pp.config.Workers

	for worker := 0; worker < pp.config.Workers; worker++ {
		start := worker * chunkSize
		if start >= len(arr) {
			break
		}
		end := start + chunkSize
		if end > len(arr) {
			end = len(arr)
		}

		wg.Add(1)
		go func(startIdx, endIdx int) {
			defer wg.Done()
			for i := startIdx; i < endIdx; i++ {
				result[i] = transform(arr[i])
			}
		}(start, end)
	}

	wg.Wait()
	return result
}
