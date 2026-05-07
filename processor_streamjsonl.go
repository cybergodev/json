package json

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
	"sync/atomic"
)

// Scanner configuration constants for JSONL processing
const (
	// defaultScannerBufSize is the initial buffer size for JSONL scanners (64KB)
	defaultScannerBufSize = 64 * 1024
	// defaultMaxLineSize is the maximum line size for JSONL scanners (1MB)
	defaultMaxLineSize = 1024 * 1024
)

// StreamJSONL streams JSONL data from a reader with IterableValue callback support.
//
// This method provides line-by-line processing of JSONL (NDJSON) files with
// full IterableValue support for type-safe data access.
//
// Example:
//
//	processor, _ := json.New()
//	defer processor.Close()
//
//	err := processor.StreamJSONL(reader, func(lineNum int, item *json.IterableValue) error {
//		name := item.GetString("name")
//		age := item.GetInt("age")
//		fmt.Printf("Line %d: name=%s, age=%d\n", lineNum, name, age)
//		return nil // continue processing
//		// return item.Break() // to stop iteration
//	})
func (p *Processor) StreamJSONL(reader io.Reader, fn func(lineNum int, item *IterableValue) error) error {
	if err := p.checkClosed(); err != nil {
		return err
	}

	// Determine effective memory limit for JSONL processing
	memLimit := p.config.JSONLMaxMemory
	if memLimit <= 0 && p.config.MaxMemory > 0 {
		memLimit = p.config.MaxMemory
	}

	bufSize := p.config.JSONLBufferSize
	if bufSize <= 0 {
		bufSize = defaultScannerBufSize
	}
	maxLine := p.config.JSONLMaxLineSize
	if maxLine <= 0 {
		maxLine = defaultMaxLineSize
	}

	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, bufSize), maxLine)

	lineNum := 0
	var totalBytes int64

	for scanner.Scan() {
		lineNum++

		line := scanner.Bytes()

		// Skip lines based on config (empty lines, comments)
		if shouldSkipJSONLLineFromConfig(line, &p.config) {
			continue
		}

		// Track memory usage if limit is configured
		if memLimit > 0 {
			totalBytes += int64(len(line))
			if totalBytes > memLimit {
				return fmt.Errorf("jsonl memory limit exceeded: processed %d bytes (limit %d bytes at line %d)", totalBytes, memLimit, lineNum)
			}
		}

		// Parse JSON line
		var data any
		if err := json.Unmarshal(line, &data); err != nil {
			return fmt.Errorf("line %d: %w", lineNum, err)
		}

		// Create IterableValue and call user callback
		item := newIterableValue(data)
		if err := fn(lineNum, item); err != nil {
			if errors.Is(err, errBreak) {
				return nil // Clean stop
			}
			return err
		}
	}

	if err := scanner.Err(); err != nil {
		return err
	}

	return nil
}

// StreamJSONLParallel processes JSONL data in parallel with multiple workers.
// This method provides parallel processing of JSONL files with configurable worker count.
//
// Example:
//
//	processor, _ := json.New()
//	defer processor.Close()
//
//	err := processor.StreamJSONLParallel(reader, 4, func(lineNum int, item *json.IterableValue) error {
//		// Process each item in parallel
//		return nil
//	})
func (p *Processor) StreamJSONLParallel(reader io.Reader, workers int, fn func(lineNum int, item *IterableValue) error) error {
	return p.StreamJSONLParallelWithContext(context.Background(), reader, workers, fn)
}

// StreamJSONLParallelWithContext processes JSONL data in parallel with context support
// for cancellation. Workers and the scanner goroutine respect context cancellation.
// RESOURCE FIX: Added context parameter to prevent goroutine leaks when reader/fn blocks.
//
// Example:
//
//	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
//	defer cancel()
//	err := processor.StreamJSONLParallelWithContext(ctx, reader, 4, func(lineNum int, item *json.IterableValue) error {
//	    return nil
//	})
func (p *Processor) StreamJSONLParallelWithContext(ctx context.Context, reader io.Reader, workers int, fn func(lineNum int, item *IterableValue) error) error {
	if err := p.checkClosed(); err != nil {
		return err
	}

	if workers <= 0 {
		workers = 4
	}

	// Job structure for parallel processing
	type job struct {
		lineNum int
		data    any
	}

	jobs := make(chan job, workers*2)

	// Error handling
	var firstErr atomic.Pointer[error]
	var errCount int32
	var wg sync.WaitGroup

	// Start workers
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for job := range jobs {
				// RESOURCE FIX: Check context cancellation in workers
				select {
				case <-ctx.Done():
					return
				default:
				}
				if atomic.LoadInt32(&errCount) > 0 {
					continue
				}
				item := newIterableValue(job.data)
				if jobErr := fn(job.lineNum, item); jobErr != nil {
					if !errors.Is(jobErr, errBreak) {
						if atomic.CompareAndSwapInt32(&errCount, 0, 1) {
							firstErr.Store(&jobErr)
						}
					}
				}
			}
		}()
	}

	// Feed jobs — respect context cancellation during scan
	lineNum := 0
	parBufSize := p.config.JSONLBufferSize
	if parBufSize <= 0 {
		parBufSize = defaultScannerBufSize
	}
	parMaxLine := p.config.JSONLMaxLineSize
	if parMaxLine <= 0 {
		parMaxLine = defaultMaxLineSize
	}
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, parBufSize), parMaxLine)

feedLoop:
	for scanner.Scan() {
		// RESOURCE FIX: Check context on each iteration
		select {
		case <-ctx.Done():
			break feedLoop
		default:
		}

		lineNum++

		line := scanner.Bytes()

		// Skip lines based on config (empty lines, comments)
		if shouldSkipJSONLLineFromConfig(line, &p.config) {
			continue
		}

		// Parse JSON line
		var data any
		if err := json.Unmarshal(line, &data); err != nil {
			close(jobs)
			wg.Wait()
			return fmt.Errorf("line %d: %w", lineNum, err)
		}

		// Check if error occurred before sending
		if atomic.LoadInt32(&errCount) > 0 {
			break
		}

		// RESOURCE FIX: Select on ctx.Done() when sending to jobs channel
		// to prevent blocking if all workers are busy and context is cancelled.
		select {
		case jobs <- job{lineNum: lineNum, data: data}:
		case <-ctx.Done():
			break feedLoop
		}
	}

	close(jobs)
	wg.Wait()

	if ctx.Err() != nil {
		return ctx.Err()
	}

	if err := scanner.Err(); err != nil {
		return err
	}

	if storedErr := firstErr.Load(); storedErr != nil {
		return *storedErr
	}

	return nil
}

// StreamJSONLChunked processes JSONL data in chunks for memory-efficient processing
// This method provides chunked processing of JSONL files with configurable chunk size
//
// Example:
//
//	processor, _ := json.New()
//	defer processor.Close()
//
//	err := processor.StreamJSONLChunked(reader, 1000, func(chunk []*IterableValue) error {
//		// Process chunk of 1000 items
//		return nil
//	})
func (p *Processor) StreamJSONLChunked(reader io.Reader, chunkSize int, fn func(chunk []*IterableValue) error) error {
	if err := p.checkClosed(); err != nil {
		return err
	}

	if chunkSize <= 0 {
		chunkSize = 1000
	}

	// Determine effective memory limit for JSONL processing
	memLimit := p.config.JSONLMaxMemory
	if memLimit <= 0 && p.config.MaxMemory > 0 {
		memLimit = p.config.MaxMemory
	}

	var chunk []*IterableValue

	chunkBufSize := p.config.JSONLBufferSize
	if chunkBufSize <= 0 {
		chunkBufSize = defaultScannerBufSize
	}
	chunkMaxLine := p.config.JSONLMaxLineSize
	if chunkMaxLine <= 0 {
		chunkMaxLine = defaultMaxLineSize
	}
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, chunkBufSize), chunkMaxLine)

	lineNum := 0
	var totalBytes int64

	for scanner.Scan() {
		lineNum++

		line := scanner.Bytes()

		// Skip lines based on config (empty lines, comments)
		if shouldSkipJSONLLineFromConfig(line, &p.config) {
			continue
		}

		// Track memory usage if limit is configured
		if memLimit > 0 {
			totalBytes += int64(len(line))
			if totalBytes > memLimit {
				return fmt.Errorf("jsonl memory limit exceeded: processed %d bytes (limit %d bytes at line %d)", totalBytes, memLimit, lineNum)
			}
		}

		// Parse JSON line
		var data any
		if err := json.Unmarshal(line, &data); err != nil {
			return fmt.Errorf("line %d: %w", lineNum, err)
		}

		item := newIterableValue(data)
		chunk = append(chunk, item)

		if len(chunk) >= chunkSize {
			if err := fn(chunk); err != nil {
				for i := range chunk {
					iterableValuePool.Put(chunk[i])
				}
				return err
			}
			for i := range chunk {
				iterableValuePool.Put(chunk[i])
			}
			chunk = chunk[:0]
		}
	}

	if err := scanner.Err(); err != nil {
		return err
	}

	// Process remaining chunk
	if len(chunk) > 0 {
		if err := fn(chunk); err != nil {
			for i := range chunk {
				iterableValuePool.Put(chunk[i])
			}
			return err
		}
		for i := range chunk {
			iterableValuePool.Put(chunk[i])
		}
	}

	return nil
}

// ForeachJSONL iterates over JSONL data with IterableValue callback (similar to Foreach)
//
// Example:
//
//	processor, _ := json.New()
//	defer processor.Close()
//
//	err := processor.ForeachJSONL(reader, func(lineNum int, item *json.IterableValue) error {
//		fmt.Printf("Line: %d, Value: %v\n", lineNum, item.GetData())
//		return nil
//	})
func (p *Processor) ForeachJSONL(reader io.Reader, fn func(lineNum int, item *IterableValue) error) error {
	if err := p.checkClosed(); err != nil {
		return err
	}

	return p.StreamJSONL(reader, fn)
}

// MapJSONL maps JSONL data into a new format using a mapping function
//
// Example:
//
//	processor, _ := json.New()
//	defer processor.Close()
//
//	result, err := processor.MapJSONL(reader, func(lineNum int, item *json.IterableValue) (any, error) {
//		// Transform each item
//		return map[string]any{
//			"name": item.GetString("name"),
//			"age":  item.GetInt("age"),
//		}, nil
//	})
func (p *Processor) MapJSONL(reader io.Reader, fn func(lineNum int, item *IterableValue) (any, error)) ([]any, error) {
	if err := p.checkClosed(); err != nil {
		return nil, err
	}

	var results []any

	err := p.StreamJSONL(reader, func(lineNum int, item *IterableValue) error {
		value, err := fn(lineNum, item)
		if err != nil {
			return err
		}
		results = append(results, value)
		return nil
	})

	if err != nil {
		return nil, err
	}

	return results, nil
}

// ReduceJSONL reduces JSONL data to a single aggregated result using a reducer function
// The accumulator starts with the initial value and is updated by the reducer function.
//
// Example:
//
//	processor, _ := json.New()
//	defer processor.Close()
//
//	totalAge, err := processor.ReduceJSONL(reader, 0, func(acc any, item *json.IterableValue) any {
//		return acc.(int64) + int64(item.GetInt("age"))
//	})
func (p *Processor) ReduceJSONL(reader io.Reader, initial any, fn func(acc any, item *IterableValue) any) (any, error) {
	if err := p.checkClosed(); err != nil {
		return initial, err
	}

	acc := initial

	err := p.StreamJSONL(reader, func(lineNum int, item *IterableValue) error {
		acc = fn(acc, item)
		return nil
	})

	if err != nil {
		return initial, err
	}

	return acc, nil
}

// FilterJSONL filters JSONL data based on a predicate function
//
// Example:
//
//	processor, _ := json.New()
//	defer processor.Close()
//
//	adults, err := processor.FilterJSONL(reader, func(item *json.IterableValue) bool {
//		return item.GetInt("age") >= 18
//	})
func (p *Processor) FilterJSONL(reader io.Reader, predicate func(item *IterableValue) bool) ([]*IterableValue, error) {
	if err := p.checkClosed(); err != nil {
		return nil, err
	}

	var results []*IterableValue

	err := p.StreamJSONL(reader, func(lineNum int, item *IterableValue) error {
		if predicate(item) {
			results = append(results, item)
		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	return results, nil
}

// StreamJSONLFile streams JSONL data from a file with IterableValue callback
//
// Example:
//
//	processor, _ := json.New()
//	defer processor.Close()
//
//	err := processor.StreamJSONLFile("data.jsonl", func(lineNum int, item *json.IterableValue) error {
//		fmt.Printf("Line %d: %v\n", lineNum, item.GetData())
//		return nil
//	})
func (p *Processor) StreamJSONLFile(filename string, fn func(lineNum int, item *IterableValue) error) error {
	if err := p.checkClosed(); err != nil {
		return err
	}

	// SECURITY: Validate file path to prevent path traversal attacks
	if err := p.validateFilePath(filename); err != nil {
		return err
	}

	// Use os.Open to read file
	file, err := os.Open(filename)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	return p.StreamJSONL(file, fn)
}

// CollectJSONL collects all JSONL items into a slice
//
// Example:
//
//	processor, _ := json.New()
//	defer processor.Close()
//
//	items, err := processor.CollectJSONL(reader)
//	for _, item := range items {
//		fmt.Println(item.GetString("name"))
//	}
func (p *Processor) CollectJSONL(reader io.Reader) ([]*IterableValue, error) {
	if err := p.checkClosed(); err != nil {
		return nil, err
	}

	var items []*IterableValue

	err := p.StreamJSONL(reader, func(lineNum int, item *IterableValue) error {
		items = append(items, item)
		return nil
	})

	if err != nil {
		return nil, err
	}

	return items, nil
}

// FirstJSONL returns the first JSONL item that matches a predicate
//
// Example:
//
//	processor, _ := json.New()
//	defer processor.Close()
//
//	user, found, err := processor.FirstJSONL(reader, func(item *json.IterableValue) bool {
//		return item.GetString("name") == "Alice"
//	})
func (p *Processor) FirstJSONL(reader io.Reader, predicate func(item *IterableValue) bool) (*IterableValue, bool, error) {
	if err := p.checkClosed(); err != nil {
		return nil, false, err
	}

	var result *IterableValue
	found := false

	err := p.StreamJSONL(reader, func(lineNum int, item *IterableValue) error {
		if predicate(item) {
			result = item
			found = true
			return errBreak
		}
		return nil
	})

	if err != nil {
		return nil, false, err
	}

	return result, found, nil
}

// ============================================================================
// Package-level JSONL wrappers (dual-layer design)
// Delegate to the default processor for convenience
// ============================================================================

// StreamJSONL streams JSONL data from a reader with IterableValue callback support.
//
// Example:
//
//	err := json.StreamJSONL(reader, func(lineNum int, item *json.IterableValue) error {
//		name := item.GetString("name")
//		fmt.Printf("Line %d: name=%s\n", lineNum, name)
//		return nil // continue processing
//	})
func StreamJSONL(reader io.Reader, fn func(lineNum int, item *IterableValue) error) error {
	return withProcessorError(func(p *Processor) error {
		return p.StreamJSONL(reader, fn)
	})
}

// StreamJSONLParallel processes JSONL data in parallel with multiple workers.
//
// Example:
//
//	err := json.StreamJSONLParallel(reader, 4, func(lineNum int, item *json.IterableValue) error {
//		// Process each item in parallel
//		return nil
//	})
func StreamJSONLParallel(reader io.Reader, workers int, fn func(lineNum int, item *IterableValue) error) error {
	return withProcessorError(func(p *Processor) error {
		return p.StreamJSONLParallel(reader, workers, fn)
	})
}

// StreamJSONLParallelWithContext processes JSONL data in parallel with context support
// for cancellation. See Processor.StreamJSONLParallelWithContext for details.
func StreamJSONLParallelWithContext(ctx context.Context, reader io.Reader, workers int, fn func(lineNum int, item *IterableValue) error) error {
	return withProcessorError(func(p *Processor) error {
		return p.StreamJSONLParallelWithContext(ctx, reader, workers, fn)
	})
}

// StreamJSONLChunked processes JSONL data in chunks for memory-efficient processing.
//
// Example:
//
//	err := json.StreamJSONLChunked(reader, 1000, func(chunk []*json.IterableValue) error {
//		// Process chunk of 1000 items
//		return nil
//	})
func StreamJSONLChunked(reader io.Reader, chunkSize int, fn func(chunk []*IterableValue) error) error {
	return withProcessorError(func(p *Processor) error {
		return p.StreamJSONLChunked(reader, chunkSize, fn)
	})
}

// ForeachJSONL iterates over JSONL data with IterableValue callback.
//
// Example:
//
//	err := json.ForeachJSONL(reader, func(lineNum int, item *json.IterableValue) error {
//		fmt.Printf("Line: %d, Value: %v\n", lineNum, item.GetData())
//		return nil
//	})
func ForeachJSONL(reader io.Reader, fn func(lineNum int, item *IterableValue) error) error {
	return withProcessorError(func(p *Processor) error {
		return p.ForeachJSONL(reader, fn)
	})
}

// MapJSONL maps JSONL data into a new format using a mapping function.
//
// Example:
//
//	result, err := json.MapJSONL(reader, func(lineNum int, item *json.IterableValue) (any, error) {
//		return map[string]any{
//			"name": item.GetString("name"),
//			"age":  item.GetInt("age"),
//		}, nil
//	})
func MapJSONL(reader io.Reader, fn func(lineNum int, item *IterableValue) (any, error)) ([]any, error) {
	return withProcessor(func(p *Processor) ([]any, error) {
		return p.MapJSONL(reader, fn)
	})
}

// ReduceJSONL reduces JSONL data to a single aggregated result using a reducer function.
//
// Example:
//
//	totalAge, err := json.ReduceJSONL(reader, 0, func(acc any, item *json.IterableValue) any {
//		return acc.(int64) + int64(item.GetInt("age"))
//	})
func ReduceJSONL(reader io.Reader, initial any, fn func(acc any, item *IterableValue) any) (any, error) {
	// Note: Cannot use withProcessor because it returns zero-value on error,
	// but ReduceJSONL must return the initial accumulator on error.
	p, err := getProcessorOrFail()
	if err != nil {
		return initial, err
	}
	return p.ReduceJSONL(reader, initial, fn)
}

// FilterJSONL filters JSONL data based on a predicate function.
//
// Example:
//
//	adults, err := json.FilterJSONL(reader, func(item *json.IterableValue) bool {
//		return item.GetInt("age") >= 18
//	})
func FilterJSONL(reader io.Reader, predicate func(item *IterableValue) bool) ([]*IterableValue, error) {
	return withProcessor(func(p *Processor) ([]*IterableValue, error) {
		return p.FilterJSONL(reader, predicate)
	})
}

// StreamJSONLFile streams JSONL data from a file with IterableValue callback.
//
// Example:
//
//	err := json.StreamJSONLFile("data.jsonl", func(lineNum int, item *json.IterableValue) error {
//		fmt.Printf("Line %d: %v\n", lineNum, item.GetData())
//		return nil
//	})
func StreamJSONLFile(filename string, fn func(lineNum int, item *IterableValue) error) error {
	return withProcessorError(func(p *Processor) error {
		return p.StreamJSONLFile(filename, fn)
	})
}

// CollectJSONL collects all JSONL items into a slice.
//
// Example:
//
//	items, err := json.CollectJSONL(reader)
//	for _, item := range items {
//		fmt.Println(item.GetString("name"))
//	}
func CollectJSONL(reader io.Reader) ([]*IterableValue, error) {
	return withProcessor(func(p *Processor) ([]*IterableValue, error) {
		return p.CollectJSONL(reader)
	})
}

// FirstJSONL returns the first JSONL item that matches a predicate.
//
// Example:
//
//	user, found, err := json.FirstJSONL(reader, func(item *json.IterableValue) bool {
//		return item.GetString("name") == "Alice"
//	})
func FirstJSONL(reader io.Reader, predicate func(item *IterableValue) bool) (*IterableValue, bool, error) {
	// Note: Cannot use withProcessor because it only supports (T, error) return,
	// but FirstJSONL returns (*IterableValue, bool, error).
	p, err := getProcessorOrFail()
	if err != nil {
		return nil, false, err
	}
	return p.FirstJSONL(reader, predicate)
}
