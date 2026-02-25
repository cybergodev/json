package internal

import (
	"errors"
	"runtime"
	"sync/atomic"
	"testing"
	"time"
)

// ============================================================================
// DefaultParallelConfig TESTS
// ============================================================================

func TestDefaultParallelConfig_Extended(t *testing.T) {
	config := DefaultParallelConfig()

	cpuCount := runtime.NumCPU()
	expectedWorkers := cpuCount
	if expectedWorkers < 2 {
		expectedWorkers = 2
	}
	if expectedWorkers > 16 {
		expectedWorkers = 16
	}

	if config.Workers != expectedWorkers {
		t.Errorf("expected Workers %d, got %d", expectedWorkers, config.Workers)
	}
	if config.BatchSize != 100 {
		t.Errorf("expected BatchSize 100, got %d", config.BatchSize)
	}
	if config.MinParallel != 1000 {
		t.Errorf("expected MinParallel 1000, got %d", config.MinParallel)
	}
}

// ============================================================================
// NewParallelProcessor TESTS
// ============================================================================

func TestNewParallelProcessor_DefaultConfig(t *testing.T) {
	config := ParallelConfig{} // Zero values
	pp := NewParallelProcessor(config)

	if pp == nil {
		t.Fatal("NewParallelProcessor returned nil")
	}
	if pp.config.Workers <= 0 {
		t.Error("Workers should be set to a positive value")
	}
	if pp.config.BatchSize != 100 {
		t.Errorf("expected default BatchSize 100, got %d", pp.config.BatchSize)
	}
	if pp.config.MinParallel != 1000 {
		t.Errorf("expected default MinParallel 1000, got %d", pp.config.MinParallel)
	}
	if pp.pool == nil {
		t.Error("pool should not be nil")
	}
}

func TestNewParallelProcessor_CustomConfig(t *testing.T) {
	config := ParallelConfig{
		Workers:     4,
		BatchSize:   50,
		MinParallel: 100,
	}
	pp := NewParallelProcessor(config)

	if pp.config.Workers != 4 {
		t.Errorf("expected Workers 4, got %d", pp.config.Workers)
	}
	if pp.config.BatchSize != 50 {
		t.Errorf("expected BatchSize 50, got %d", pp.config.BatchSize)
	}
	if pp.config.MinParallel != 100 {
		t.Errorf("expected MinParallel 100, got %d", pp.config.MinParallel)
	}
}

func TestNewParallelProcessor_NegativeValues(t *testing.T) {
	config := ParallelConfig{
		Workers:     -1,
		BatchSize:   -1,
		MinParallel: -1,
	}
	pp := NewParallelProcessor(config)

	if pp.config.Workers <= 0 {
		t.Error("Workers should be set to a positive value even with negative input")
	}
	if pp.config.BatchSize != 100 {
		t.Errorf("expected default BatchSize 100, got %d", pp.config.BatchSize)
	}
	if pp.config.MinParallel != 1000 {
		t.Errorf("expected default MinParallel 1000, got %d", pp.config.MinParallel)
	}
}

func TestDefaultParallelProcessor_Extended(t *testing.T) {
	if DefaultParallelProcessor == nil {
		t.Fatal("DefaultParallelProcessor is nil")
	}
}

// ============================================================================
// ParallelMap TESTS
// ============================================================================

func TestParallelMap_SmallMap(t *testing.T) {
	config := ParallelConfig{
		Workers:     2,
		BatchSize:   10,
		MinParallel: 100, // Higher than our test data size
	}
	pp := NewParallelProcessor(config)

	// Small map should be processed sequentially
	input := map[string]any{
		"a": 1,
		"b": 2,
		"c": 3,
	}

	result, err := pp.ParallelMap(input, func(key string, value any) (any, error) {
		return value.(int) * 2, nil
	})

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(result) != 3 {
		t.Errorf("expected 3 results, got %d", len(result))
	}
	if result["a"].(int) != 2 || result["b"].(int) != 4 || result["c"].(int) != 6 {
		t.Errorf("unexpected results: %v", result)
	}
}

func TestParallelMap_LargeMap(t *testing.T) {
	config := ParallelConfig{
		Workers:     2,
		BatchSize:   10,
		MinParallel: 10, // Lower than our test data size
	}
	pp := NewParallelProcessor(config)

	// Large map should be processed in parallel
	input := make(map[string]any)
	for i := 0; i < 50; i++ {
		input[string(rune('a'+i%26))+string(rune('0'+i/26))] = i
	}

	result, err := pp.ParallelMap(input, func(key string, value any) (any, error) {
		return value.(int) * 2, nil
	})

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(result) != 50 {
		t.Errorf("expected 50 results, got %d", len(result))
	}
}

func TestParallelMap_WithError(t *testing.T) {
	config := ParallelConfig{
		Workers:     2,
		BatchSize:   10,
		MinParallel: 10,
	}
	pp := NewParallelProcessor(config)

	input := make(map[string]any)
	for i := 0; i < 50; i++ {
		input[string(rune('a'+i%26))+string(rune('0'+i/26))] = i
	}

	expectedErr := errors.New("test error")
	result, err := pp.ParallelMap(input, func(key string, value any) (any, error) {
		if value.(int) == 25 {
			return nil, expectedErr
		}
		return value.(int) * 2, nil
	})

	if err != expectedErr {
		t.Errorf("expected error, got %v", err)
	}
	// result may be partial on error, just verify error was returned
	_ = result
}

func TestParallelMap_EmptyMap(t *testing.T) {
	pp := NewParallelProcessor(ParallelConfig{})

	result, err := pp.ParallelMap(map[string]any{}, func(key string, value any) (any, error) {
		return value, nil
	})

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("expected empty result, got %d", len(result))
	}
}

// ============================================================================
// ParallelSlice TESTS
// ============================================================================

func TestParallelSlice_SmallSlice(t *testing.T) {
	config := ParallelConfig{
		Workers:     2,
		BatchSize:   10,
		MinParallel: 100, // Higher than our test data size
	}
	pp := NewParallelProcessor(config)

	input := []any{1, 2, 3, 4, 5}
	result, err := pp.ParallelSlice(input, func(index int, value any) (any, error) {
		return value.(int) * 2, nil
	})

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(result) != 5 {
		t.Errorf("expected 5 results, got %d", len(result))
	}
	for i, v := range result {
		if v.(int) != (i+1)*2 {
			t.Errorf("expected %d at index %d, got %d", (i+1)*2, i, v)
		}
	}
}

func TestParallelSlice_LargeSlice(t *testing.T) {
	config := ParallelConfig{
		Workers:     2,
		BatchSize:   10,
		MinParallel: 10, // Lower than our test data size
	}
	pp := NewParallelProcessor(config)

	size := 100
	input := make([]any, size)
	for i := 0; i < size; i++ {
		input[i] = i
	}

	result, err := pp.ParallelSlice(input, func(index int, value any) (any, error) {
		return value.(int) * 2, nil
	})

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(result) != size {
		t.Errorf("expected %d results, got %d", size, len(result))
	}
	for i, v := range result {
		if v.(int) != i*2 {
			t.Errorf("expected %d at index %d, got %d", i*2, i, v)
		}
	}
}

func TestParallelSlice_WithError(t *testing.T) {
	config := ParallelConfig{
		Workers:     2,
		BatchSize:   10,
		MinParallel: 10,
	}
	pp := NewParallelProcessor(config)

	size := 100
	input := make([]any, size)
	for i := 0; i < size; i++ {
		input[i] = i
	}

	expectedErr := errors.New("test error")
	result, err := pp.ParallelSlice(input, func(index int, value any) (any, error) {
		if index == 50 {
			return nil, expectedErr
		}
		return value.(int) * 2, nil
	})

	if err != expectedErr {
		t.Errorf("expected error, got %v", err)
	}
	_ = result // result may be partial on error
}

func TestParallelSlice_EmptySlice(t *testing.T) {
	pp := NewParallelProcessor(ParallelConfig{})

	result, err := pp.ParallelSlice([]any{}, func(index int, value any) (any, error) {
		return value, nil
	})

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("expected empty result, got %d", len(result))
	}
}

// ============================================================================
// ParallelForEach TESTS
// ============================================================================

func TestParallelForEach_SmallSlice(t *testing.T) {
	config := ParallelConfig{
		Workers:     2,
		BatchSize:   10,
		MinParallel: 100, // Higher than our test data size
	}
	pp := NewParallelProcessor(config)

	var sum atomic.Int64
	input := []any{1, 2, 3, 4, 5}
	err := pp.ParallelForEach(input, func(index int, value any) error {
		sum.Add(int64(value.(int)))
		return nil
	})

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if sum.Load() != 15 {
		t.Errorf("expected sum 15, got %d", sum.Load())
	}
}

func TestParallelForEach_LargeSlice(t *testing.T) {
	config := ParallelConfig{
		Workers:     2,
		BatchSize:   10,
		MinParallel: 10, // Lower than our test data size
	}
	pp := NewParallelProcessor(config)

	var count atomic.Int64
	size := 100
	input := make([]any, size)
	for i := 0; i < size; i++ {
		input[i] = i
	}

	err := pp.ParallelForEach(input, func(index int, value any) error {
		count.Add(1)
		return nil
	})

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if count.Load() != int64(size) {
		t.Errorf("expected count %d, got %d", size, count.Load())
	}
}

func TestParallelForEach_WithError(t *testing.T) {
	config := ParallelConfig{
		Workers:     2,
		BatchSize:   10,
		MinParallel: 10,
	}
	pp := NewParallelProcessor(config)

	size := 100
	input := make([]any, size)
	for i := 0; i < size; i++ {
		input[i] = i
	}

	expectedErr := errors.New("test error")
	err := pp.ParallelForEach(input, func(index int, value any) error {
		if index == 50 {
			return expectedErr
		}
		return nil
	})

	if err != expectedErr {
		t.Errorf("expected error, got %v", err)
	}
}

func TestParallelForEach_EmptySlice(t *testing.T) {
	pp := NewParallelProcessor(ParallelConfig{})

	err := pp.ParallelForEach([]any{}, func(index int, value any) error {
		return nil
	})

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

// ============================================================================
// ParallelForEachMap TESTS
// ============================================================================

func TestParallelForEachMap_SmallMap(t *testing.T) {
	config := ParallelConfig{
		Workers:     2,
		BatchSize:   10,
		MinParallel: 100, // Higher than our test data size
	}
	pp := NewParallelProcessor(config)

	var sum atomic.Int64
	input := map[string]any{
		"a": 1,
		"b": 2,
		"c": 3,
	}

	err := pp.ParallelForEachMap(input, func(key string, value any) error {
		sum.Add(int64(value.(int)))
		return nil
	})

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if sum.Load() != 6 {
		t.Errorf("expected sum 6, got %d", sum.Load())
	}
}

func TestParallelForEachMap_LargeMap(t *testing.T) {
	config := ParallelConfig{
		Workers:     2,
		BatchSize:   10,
		MinParallel: 10, // Lower than our test data size
	}
	pp := NewParallelProcessor(config)

	var count atomic.Int64
	size := 50
	input := make(map[string]any)
	for i := 0; i < size; i++ {
		input[string(rune('a'+i%26))+string(rune('0'+i/26))] = i
	}

	err := pp.ParallelForEachMap(input, func(key string, value any) error {
		count.Add(1)
		return nil
	})

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if count.Load() != int64(size) {
		t.Errorf("expected count %d, got %d", size, count.Load())
	}
}

func TestParallelForEachMap_WithError(t *testing.T) {
	config := ParallelConfig{
		Workers:     2,
		BatchSize:   10,
		MinParallel: 10,
	}
	pp := NewParallelProcessor(config)

	size := 50
	input := make(map[string]any)
	for i := 0; i < size; i++ {
		input[string(rune('a'+i%26))+string(rune('0'+i/26))] = i
	}

	expectedErr := errors.New("test error")
	err := pp.ParallelForEachMap(input, func(key string, value any) error {
		if value.(int) == 25 {
			return expectedErr
		}
		return nil
	})

	if err != expectedErr {
		t.Errorf("expected error, got %v", err)
	}
}

func TestParallelForEachMap_EmptyMap(t *testing.T) {
	pp := NewParallelProcessor(ParallelConfig{})

	err := pp.ParallelForEachMap(map[string]any{}, func(key string, value any) error {
		return nil
	})

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

// ============================================================================
// WorkerPool TESTS
// ============================================================================

func TestNewWorkerPool_DefaultWorkers(t *testing.T) {
	wp := NewWorkerPool(0) // 0 should use default
	defer wp.Stop()

	if wp == nil {
		t.Fatal("NewWorkerPool returned nil")
	}
	if wp.workers <= 0 {
		t.Error("workers should be positive")
	}
}

func TestNewWorkerPool_CustomWorkers(t *testing.T) {
	wp := NewWorkerPool(4)
	defer wp.Stop()

	if wp.workers != 4 {
		t.Errorf("expected 4 workers, got %d", wp.workers)
	}
}

func TestWorkerPool_Submit(t *testing.T) {
	wp := NewWorkerPool(2)
	defer wp.Stop()

	var counter atomic.Int64

	// Submit tasks
	for i := 0; i < 10; i++ {
		wp.Submit(func() {
			counter.Add(1)
		})
	}

	// Wait for tasks to complete
	time.Sleep(100 * time.Millisecond)

	if counter.Load() != 10 {
		t.Errorf("expected counter 10, got %d", counter.Load())
	}
}

func TestWorkerPool_SubmitFullChannel(t *testing.T) {
	// Create pool with small buffer
	wp := NewWorkerPool(1)
	defer wp.Stop()

	var counter atomic.Int64

	// Submit many tasks to fill channel
	for i := 0; i < 100; i++ {
		wp.Submit(func() {
			counter.Add(1)
			time.Sleep(10 * time.Millisecond) // Slow task
		})
	}

	// All tasks should complete (some run synchronously when channel full)
	time.Sleep(500 * time.Millisecond)

	if counter.Load() != 100 {
		t.Errorf("expected counter 100, got %d", counter.Load())
	}
}

func TestWorkerPool_Stop(t *testing.T) {
	wp := NewWorkerPool(2)

	var counter atomic.Int64

	// Submit tasks
	for i := 0; i < 5; i++ {
		wp.Submit(func() {
			counter.Add(1)
		})
	}

	// Stop should wait for workers to finish
	wp.Stop()

	// After stop, no more tasks should be processed
	wp.Submit(func() {
		counter.Add(100) // This should not run
	})

	time.Sleep(50 * time.Millisecond)

	// counter should be 5 (original tasks), not 105
	if counter.Load() > 10 {
		t.Errorf("tasks ran after stop: counter = %d", counter.Load())
	}
}

func TestWorkerPool_Wait(t *testing.T) {
	wp := NewWorkerPool(2)
	defer wp.Stop()

	// Submit tasks
	for i := 0; i < 10; i++ {
		wp.Submit(func() {
			time.Sleep(10 * time.Millisecond)
		})
	}

	// Wait should return when tasks are drained
	wp.Wait()
}

// ============================================================================
// ChunkProcessor TESTS
// ============================================================================

func TestNewChunkProcessor_DefaultSize(t *testing.T) {
	cp := NewChunkProcessor(0) // 0 should use default

	if cp == nil {
		t.Fatal("NewChunkProcessor returned nil")
	}
	if cp.chunkSize != 1000 {
		t.Errorf("expected default chunkSize 1000, got %d", cp.chunkSize)
	}
}

func TestNewChunkProcessor_CustomSize(t *testing.T) {
	cp := NewChunkProcessor(500)

	if cp.chunkSize != 500 {
		t.Errorf("expected chunkSize 500, got %d", cp.chunkSize)
	}
}

func TestChunkProcessor_ProcessSlice(t *testing.T) {
	cp := NewChunkProcessor(10)

	// Create slice of 25 items
	input := make([]any, 25)
	for i := 0; i < 25; i++ {
		input[i] = i
	}

	var chunkCount atomic.Int64
	var totalItems atomic.Int64

	err := cp.ProcessSlice(input, func(chunk []any) error {
		chunkCount.Add(1)
		totalItems.Add(int64(len(chunk)))
		return nil
	})

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Should have 3 chunks: 10, 10, 5
	if chunkCount.Load() != 3 {
		t.Errorf("expected 3 chunks, got %d", chunkCount.Load())
	}
	if totalItems.Load() != 25 {
		t.Errorf("expected 25 total items, got %d", totalItems.Load())
	}
}

func TestChunkProcessor_ProcessSlice_Empty(t *testing.T) {
	cp := NewChunkProcessor(10)

	called := false
	err := cp.ProcessSlice([]any{}, func(chunk []any) error {
		called = true
		return nil
	})

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if called {
		t.Error("function should not be called for empty slice")
	}
}

func TestChunkProcessor_ProcessSlice_WithError(t *testing.T) {
	cp := NewChunkProcessor(10)

	input := make([]any, 25)
	for i := 0; i < 25; i++ {
		input[i] = i
	}

	expectedErr := errors.New("chunk error")
	err := cp.ProcessSlice(input, func(chunk []any) error {
		if len(chunk) > 0 && chunk[0].(int) >= 10 {
			return expectedErr
		}
		return nil
	})

	if err != expectedErr {
		t.Errorf("expected error, got %v", err)
	}
}

func TestChunkProcessor_ProcessMap(t *testing.T) {
	cp := NewChunkProcessor(10)

	// Create map with 25 items
	input := make(map[string]any)
	for i := 0; i < 25; i++ {
		input[string(rune('a'+i%26))+string(rune('0'+i/26))] = i
	}

	var chunkCount atomic.Int64
	var totalItems atomic.Int64

	err := cp.ProcessMap(input, func(chunk map[string]any) error {
		chunkCount.Add(1)
		totalItems.Add(int64(len(chunk)))
		return nil
	})

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Should have 3 chunks: 10, 10, 5
	if chunkCount.Load() != 3 {
		t.Errorf("expected 3 chunks, got %d", chunkCount.Load())
	}
	if totalItems.Load() != 25 {
		t.Errorf("expected 25 total items, got %d", totalItems.Load())
	}
}

func TestChunkProcessor_ProcessMap_Empty(t *testing.T) {
	cp := NewChunkProcessor(10)

	called := false
	err := cp.ProcessMap(map[string]any{}, func(chunk map[string]any) error {
		called = true
		return nil
	})

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if called {
		t.Error("function should not be called for empty map")
	}
}

func TestChunkProcessor_ProcessMap_WithError(t *testing.T) {
	cp := NewChunkProcessor(10)

	input := make(map[string]any)
	for i := 0; i < 25; i++ {
		input[string(rune('a'+i%26))+string(rune('0'+i/26))] = i
	}

	expectedErr := errors.New("chunk error")
	callCount := atomic.Int64{}
	err := cp.ProcessMap(input, func(chunk map[string]any) error {
		callCount.Add(1)
		if callCount.Load() >= 2 {
			return expectedErr
		}
		return nil
	})

	if err != expectedErr {
		t.Errorf("expected error, got %v", err)
	}
}

func TestChunkProcessor_ProcessMap_ExactChunkSize(t *testing.T) {
	cp := NewChunkProcessor(10)

	// Create map with exactly 10 items (one chunk)
	input := make(map[string]any)
	for i := 0; i < 10; i++ {
		input[string(rune('a'+i))] = i
	}

	var chunkCount atomic.Int64
	err := cp.ProcessMap(input, func(chunk map[string]any) error {
		chunkCount.Add(1)
		return nil
	})

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if chunkCount.Load() != 1 {
		t.Errorf("expected 1 chunk, got %d", chunkCount.Load())
	}
}

// ============================================================================
// ParallelFilter TESTS
// ============================================================================

func TestParallelFilter_SmallSlice(t *testing.T) {
	config := ParallelConfig{
		Workers:     2,
		BatchSize:   10,
		MinParallel: 100, // Higher than our test data size
	}
	pp := NewParallelProcessor(config)

	input := []any{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	result := pp.ParallelFilter(input, func(value any) bool {
		return value.(int)%2 == 0 // Keep even numbers
	})

	if len(result) != 5 {
		t.Errorf("expected 5 results, got %d", len(result))
	}
	for _, v := range result {
		if v.(int)%2 != 0 {
			t.Errorf("expected only even numbers, got %d", v)
		}
	}
}

func TestParallelFilter_LargeSlice(t *testing.T) {
	config := ParallelConfig{
		Workers:     2,
		BatchSize:   10,
		MinParallel: 10, // Lower than our test data size
	}
	pp := NewParallelProcessor(config)

	size := 100
	input := make([]any, size)
	for i := 0; i < size; i++ {
		input[i] = i
	}

	result := pp.ParallelFilter(input, func(value any) bool {
		return value.(int) < 25 // Keep first 25
	})

	if len(result) != 25 {
		t.Errorf("expected 25 results, got %d", len(result))
	}
}

func TestParallelFilter_AllPass(t *testing.T) {
	pp := NewParallelProcessor(ParallelConfig{MinParallel: 10})

	input := []any{1, 2, 3, 4, 5}
	result := pp.ParallelFilter(input, func(value any) bool {
		return true
	})

	if len(result) != 5 {
		t.Errorf("expected 5 results, got %d", len(result))
	}
}

func TestParallelFilter_NonePass(t *testing.T) {
	pp := NewParallelProcessor(ParallelConfig{MinParallel: 10})

	input := []any{1, 2, 3, 4, 5}
	result := pp.ParallelFilter(input, func(value any) bool {
		return false
	})

	if len(result) != 0 {
		t.Errorf("expected 0 results, got %d", len(result))
	}
}

func TestParallelFilter_EmptySlice(t *testing.T) {
	pp := NewParallelProcessor(ParallelConfig{})

	result := pp.ParallelFilter([]any{}, func(value any) bool {
		return true
	})

	if len(result) != 0 {
		t.Errorf("expected empty result, got %d", len(result))
	}
}

// ============================================================================
// ParallelTransform TESTS
// ============================================================================

func TestParallelTransform_SmallSlice(t *testing.T) {
	config := ParallelConfig{
		Workers:     2,
		BatchSize:   10,
		MinParallel: 100, // Higher than our test data size
	}
	pp := NewParallelProcessor(config)

	input := []any{1, 2, 3, 4, 5}
	result := pp.ParallelTransform(input, func(value any) any {
		return value.(int) * 2
	})

	if len(result) != 5 {
		t.Errorf("expected 5 results, got %d", len(result))
	}
	for i, v := range result {
		if v.(int) != (i+1)*2 {
			t.Errorf("expected %d at index %d, got %d", (i+1)*2, i, v)
		}
	}
}

func TestParallelTransform_LargeSlice(t *testing.T) {
	config := ParallelConfig{
		Workers:     2,
		BatchSize:   10,
		MinParallel: 10, // Lower than our test data size
	}
	pp := NewParallelProcessor(config)

	size := 100
	input := make([]any, size)
	for i := 0; i < size; i++ {
		input[i] = i
	}

	result := pp.ParallelTransform(input, func(value any) any {
		return value.(int) * 2
	})

	if len(result) != size {
		t.Errorf("expected %d results, got %d", size, len(result))
	}
	for i, v := range result {
		if v.(int) != i*2 {
			t.Errorf("expected %d at index %d, got %d", i*2, i, v)
		}
	}
}

func TestParallelTransform_EmptySlice(t *testing.T) {
	pp := NewParallelProcessor(ParallelConfig{})

	result := pp.ParallelTransform([]any{}, func(value any) any {
		return value
	})

	if len(result) != 0 {
		t.Errorf("expected empty result, got %d", len(result))
	}
}

func TestParallelTransform_TypeChange(t *testing.T) {
	pp := NewParallelProcessor(ParallelConfig{MinParallel: 100}) // Force sequential

	input := []any{1, 2, 3}
	result := pp.ParallelTransform(input, func(value any) any {
		return string(rune('a' + value.(int) - 1))
	})

	if result[0].(string) != "a" || result[1].(string) != "b" || result[2].(string) != "c" {
		t.Errorf("unexpected results: %v", result)
	}
}

// ============================================================================
// Integration TESTS
// ============================================================================

func TestParallelProcessor_Integration(t *testing.T) {
	config := ParallelConfig{
		Workers:     4,
		BatchSize:   25,
		MinParallel: 50,
	}
	pp := NewParallelProcessor(config)

	// Create test data
	size := 100
	input := make([]any, size)
	for i := 0; i < size; i++ {
		input[i] = i
	}

	// Transform
	transformed, err := pp.ParallelSlice(input, func(index int, value any) (any, error) {
		return value.(int) * 2, nil
	})
	if err != nil {
		t.Fatalf("transform error: %v", err)
	}

	// Filter
	filtered := pp.ParallelFilter(transformed, func(value any) bool {
		return value.(int) < 100
	})

	// Should have 50 items (0-49 * 2 = 0-98)
	if len(filtered) != 50 {
		t.Errorf("expected 50 filtered results, got %d", len(filtered))
	}
}

// ============================================================================
// Concurrent Safety TESTS
// ============================================================================

func TestParallelProcessor_ConcurrentSafety(t *testing.T) {
	pp := NewParallelProcessor(ParallelConfig{
		Workers:     4,
		BatchSize:   10,
		MinParallel: 10,
	})

	// Run multiple parallel operations concurrently
	done := make(chan bool, 10)

	for i := 0; i < 5; i++ {
		go func() {
			input := make(map[string]any)
			for j := 0; j < 50; j++ {
				input[string(rune('a'+j%26))] = j
			}
			_, _ = pp.ParallelMap(input, func(key string, value any) (any, error) {
				return value.(int) * 2, nil
			})
			done <- true
		}()

		go func() {
			input := make([]any, 50)
			for j := 0; j < 50; j++ {
				input[j] = j
			}
			_ = pp.ParallelFilter(input, func(value any) bool {
				return value.(int)%2 == 0
			})
			done <- true
		}()
	}

	// Wait for all operations
	for i := 0; i < 10; i++ {
		<-done
	}
}

// ============================================================================
// Benchmark TESTS
// ============================================================================

func BenchmarkParallelMap(b *testing.B) {
	pp := NewParallelProcessor(ParallelConfig{
		Workers:     4,
		BatchSize:   100,
		MinParallel: 100,
	})

	input := make(map[string]any)
	for i := 0; i < 1000; i++ {
		input[string(rune('a'+i%26))+string(rune('0'+i/26))] = i
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = pp.ParallelMap(input, func(key string, value any) (any, error) {
			return value.(int) * 2, nil
		})
	}
}

func BenchmarkParallelSlice(b *testing.B) {
	pp := NewParallelProcessor(ParallelConfig{
		Workers:     4,
		BatchSize:   100,
		MinParallel: 100,
	})

	input := make([]any, 1000)
	for i := 0; i < 1000; i++ {
		input[i] = i
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = pp.ParallelSlice(input, func(index int, value any) (any, error) {
			return value.(int) * 2, nil
		})
	}
}

func BenchmarkParallelFilter(b *testing.B) {
	pp := NewParallelProcessor(ParallelConfig{
		Workers:     4,
		BatchSize:   100,
		MinParallel: 100,
	})

	input := make([]any, 1000)
	for i := 0; i < 1000; i++ {
		input[i] = i
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = pp.ParallelFilter(input, func(value any) bool {
			return value.(int)%2 == 0
		})
	}
}

func BenchmarkWorkerPool(b *testing.B) {
	wp := NewWorkerPool(4)
	defer wp.Stop()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		wp.Submit(func() {
			// Do nothing
		})
	}
}

func BenchmarkChunkProcessor(b *testing.B) {
	cp := NewChunkProcessor(100)

	input := make([]any, 1000)
	for i := 0; i < 1000; i++ {
		input[i] = i
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = cp.ProcessSlice(input, func(chunk []any) error {
			return nil
		})
	}
}
