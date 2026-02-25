// Package json provides a high-performance, thread-safe JSON processing library
// with 100% encoding/json compatibility and advanced path operations.
//
// Key Features:
//   - 100% encoding/json compatibility - drop-in replacement
//   - High-performance path operations with smart caching
//   - Thread-safe concurrent operations
//   - Type-safe generic operations with Go 1.24+ features
//   - Memory-efficient resource pooling
//   - Production-ready error handling and validation
//
// Basic Usage:
//
//	// Simple operations (100% compatible with encoding/json)
//	data, err := json.Marshal(value)
//	err = json.Unmarshal(data, &target)
//
//	// Advanced path operations
//	value, err := json.Get(`{"user":{"name":"John"}}`, "user.name")
//	result, err := json.Set(`{"user":{}}`, "user.age", 30)
//
//	// Type-safe operations
//	name, err := json.GetString(jsonStr, "user.name")
//	age, err := json.GetInt(jsonStr, "user.age")
//
//	// Advanced processor for complex operations
//	processor := json.New() // Use default config
//	defer processor.Close()
//	value, err := processor.Get(jsonStr, "complex.path[0].field")
package json

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"sync"
	"sync/atomic"

	"github.com/cybergodev/json/internal"
)

var (
	defaultProcessor   atomic.Pointer[Processor]
	defaultProcessorMu sync.Mutex
	// defaultProcessorOnce provides thread-safe lazy initialization using Go 1.24+ sync.OnceValue
	defaultProcessorOnce sync.Once
)

// getDefaultProcessorFn is a thread-safe function that returns the default processor
// Uses sync.OnceValue pattern for efficient lazy initialization
func getDefaultProcessor() *Processor {
	// Fast path: check if processor exists and is not closed
	if p := defaultProcessor.Load(); p != nil && !p.IsClosed() {
		return p
	}

	// Slow path: need to create or replace processor
	defaultProcessorMu.Lock()
	defer defaultProcessorMu.Unlock()

	// Double-check after acquiring lock
	if p := defaultProcessor.Load(); p != nil && !p.IsClosed() {
		return p
	}

	// Create new processor
	p := New()
	defaultProcessor.Store(p)
	return p
}

// SetGlobalProcessor sets a custom global processor (thread-safe)
func SetGlobalProcessor(processor *Processor) {
	if processor == nil {
		return
	}

	defaultProcessorMu.Lock()
	defer defaultProcessorMu.Unlock()

	if old := defaultProcessor.Swap(processor); old != nil {
		old.Close()
	}
}

// ShutdownGlobalProcessor shuts down the global processor
func ShutdownGlobalProcessor() {
	defaultProcessorMu.Lock()
	defer defaultProcessorMu.Unlock()

	if old := defaultProcessor.Swap(nil); old != nil {
		old.Close()
	}
}

// Package-level API functions have been refactored into separate files:
//
//   - api_get.go    : Get*, GetTyped*, GetString*, etc.
//   - api_set.go    : Set, SetWithAdd, SetMultiple*, etc.
//   - api_delete.go : Delete, DeleteWithCleanNull
//   - api_encode.go : Marshal, Unmarshal, Encode, Format*, Print*, etc.
//   - api_file.go   : LoadFromFile, SaveToFile, MarshalToFile, etc.
//   - api_batch.go  : ProcessBatch, WarmupCache, ClearCache, GetStats, etc.
//
// All functions remain in package json and maintain 100% API compatibility.

// ============================================================================
// JSONL (JSON LINES) SUPPORT
// Efficient processing for line-delimited JSON format
// Commonly used for: logs, data pipelines, streaming data
// ============================================================================

// JSONLConfig holds configuration for JSONL processing
type JSONLConfig struct {
	BufferSize    int  // Buffer size for reading (default: 64KB)
	MaxLineSize   int  // Maximum line size (default: 1MB)
	SkipEmpty     bool // Skip empty lines (default: true)
	SkipComments  bool // Skip lines starting with # or // (default: false)
	ContinueOnErr bool // Continue processing on parse errors (default: false)
}

// DefaultJSONLConfig returns the default JSONL configuration
func DefaultJSONLConfig() JSONLConfig {
	return JSONLConfig{
		BufferSize:    64 * 1024,   // 64KB
		MaxLineSize:   1024 * 1024, // 1MB
		SkipEmpty:     true,
		SkipComments:  false,
		ContinueOnErr: false,
	}
}

// JSONLProcessor processes JSON Lines format data
// PERFORMANCE: Uses buffered reading and object pooling for efficiency
type JSONLProcessor struct {
	scanner    *bufio.Scanner
	config     JSONLConfig
	lineNum    int
	err        error
	processor  *Processor
	stopped    atomic.Bool
	bytesRead  int64
	linesCount int64
}

// jsonlProcessorPool for reusing JSONL processors
var jsonlProcessorPool = sync.Pool{
	New: func() any {
		return &JSONLProcessor{}
	},
}

// NewJSONLProcessor creates a new JSONL processor with default configuration
func NewJSONLProcessor(reader io.Reader) *JSONLProcessor {
	return NewJSONLProcessorWithConfig(reader, DefaultJSONLConfig())
}

// NewJSONLProcessorWithConfig creates a new JSONL processor with custom configuration
func NewJSONLProcessorWithConfig(reader io.Reader, config JSONLConfig) *JSONLProcessor {
	if config.BufferSize <= 0 {
		config.BufferSize = 64 * 1024
	}
	if config.MaxLineSize <= 0 {
		config.MaxLineSize = 1024 * 1024
	}

	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, config.BufferSize), config.MaxLineSize)

	p := jsonlProcessorPool.Get().(*JSONLProcessor)
	p.scanner = scanner
	p.config = config
	p.lineNum = 0
	p.err = nil
	p.processor = getDefaultProcessor()
	p.stopped.Store(false)
	p.bytesRead = 0
	p.linesCount = 0

	return p
}

// StreamLines processes JSONL data line by line
// The callback function receives the line number and parsed data
// Return false from the callback to stop iteration
func (j *JSONLProcessor) StreamLines(fn func(lineNum int, data any) bool) error {
	for j.scanner.Scan() {
		if j.stopped.Load() {
			break
		}

		j.lineNum++
		line := j.scanner.Bytes()
		j.bytesRead += int64(len(line))

		// Skip empty lines if configured
		if j.config.SkipEmpty && len(line) == 0 {
			continue
		}

		// Skip comments if configured
		if j.config.SkipComments && len(line) > 0 {
			if line[0] == '#' || (len(line) > 1 && line[0] == '/' && line[1] == '/') {
				continue
			}
		}

		var data any
		if err := json.Unmarshal(line, &data); err != nil {
			if j.config.ContinueOnErr {
				continue
			}
			return fmt.Errorf("line %d: %w", j.lineNum, err)
		}

		j.linesCount++

		if !fn(j.lineNum, data) {
			break
		}
	}

	if err := j.scanner.Err(); err != nil {
		j.err = err
		return err
	}

	return nil
}

// StreamLinesInto processes JSONL data into a slice of typed values
// Uses generics for type-safe processing
func StreamLinesInto[T any](reader io.Reader, fn func(lineNum int, data T) error) ([]T, error) {
	return StreamLinesIntoWithConfig(reader, DefaultJSONLConfig(), fn)
}

// StreamLinesIntoWithConfig processes JSONL data into a slice of typed values with config
func StreamLinesIntoWithConfig[T any](reader io.Reader, config JSONLConfig, fn func(lineNum int, data T) error) ([]T, error) {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, config.BufferSize), config.MaxLineSize)

	var results []T
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := scanner.Bytes()

		if config.SkipEmpty && len(line) == 0 {
			continue
		}

		if config.SkipComments && len(line) > 0 {
			if line[0] == '#' || (len(line) > 1 && line[0] == '/' && line[1] == '/') {
				continue
			}
		}

		var data T
		if err := json.Unmarshal(line, &data); err != nil {
			if config.ContinueOnErr {
				continue
			}
			return nil, fmt.Errorf("line %d: %w", lineNum, err)
		}

		if fn != nil {
			if err := fn(lineNum, data); err != nil {
				return nil, err
			}
		}

		results = append(results, data)
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return results, nil
}

// StreamLinesParallel processes JSONL data in parallel using worker pool
// PERFORMANCE: Parallel processing for CPU-bound operations on JSONL data
func (j *JSONLProcessor) StreamLinesParallel(fn func(lineNum int, data any) error, workers int) error {
	if workers <= 0 {
		workers = 4
	}

	// Channel for distributing work
	type lineJob struct {
		lineNum int
		data    any
	}
	jobs := make(chan lineJob, workers*2)

	// Error handling
	var firstErr error
	var errCount int32
	var wg sync.WaitGroup

	// Start workers
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for job := range jobs {
				if atomic.LoadInt32(&errCount) > 0 {
					continue
				}
				if err := fn(job.lineNum, job.data); err != nil {
					if atomic.CompareAndSwapInt32(&errCount, 0, 1) {
						firstErr = err
					}
				}
			}
		}()
	}

	// Feed jobs
	for j.scanner.Scan() {
		if j.stopped.Load() {
			break
		}

		j.lineNum++
		line := j.scanner.Bytes()
		j.bytesRead += int64(len(line))

		if j.config.SkipEmpty && len(line) == 0 {
			continue
		}

		if j.config.SkipComments && len(line) > 0 {
			if line[0] == '#' || (len(line) > 1 && line[0] == '/' && line[1] == '/') {
				continue
			}
		}

		var data any
		if err := json.Unmarshal(line, &data); err != nil {
			if j.config.ContinueOnErr {
				continue
			}
			close(jobs)
			wg.Wait()
			return fmt.Errorf("line %d: %w", j.lineNum, err)
		}

		j.linesCount++

		// Wait if error occurred
		if atomic.LoadInt32(&errCount) > 0 {
			break
		}

		jobs <- lineJob{lineNum: j.lineNum, data: data}
	}

	close(jobs)
	wg.Wait()

	if err := j.scanner.Err(); err != nil {
		j.err = err
		return err
	}

	return firstErr
}

// Stop stops the JSONL processor
func (j *JSONLProcessor) Stop() {
	j.stopped.Store(true)
}

// Err returns any error encountered during processing
func (j *JSONLProcessor) Err() error {
	return j.err
}

// Stats returns processing statistics
type JSONLStats struct {
	LinesProcessed int64
	BytesRead      int64
	CurrentLine    int
}

// GetStats returns current processing statistics
func (j *JSONLProcessor) GetStats() JSONLStats {
	return JSONLStats{
		LinesProcessed: j.linesCount,
		BytesRead:      j.bytesRead,
		CurrentLine:    j.lineNum,
	}
}

// Release returns the processor to the pool
func (j *JSONLProcessor) Release() {
	j.scanner = nil
	j.err = nil
	j.processor = nil
	j.lineNum = 0
	j.bytesRead = 0
	j.linesCount = 0
	j.stopped.Store(false)
	jsonlProcessorPool.Put(j)
}

// ============================================================================
// JSONL WRITER - For writing JSONL format
// ============================================================================

// JSONLWriter writes JSON Lines format to an io.Writer
type JSONLWriter struct {
	writer   io.Writer
	encoder  *json.Encoder
	lineNum  int
	err      error
	bytesOut int64
}

// NewJSONLWriter creates a new JSONL writer
func NewJSONLWriter(writer io.Writer) *JSONLWriter {
	encoder := json.NewEncoder(writer)
	encoder.SetEscapeHTML(false) // Performance: skip HTML escaping
	return &JSONLWriter{
		writer:  writer,
		encoder: encoder,
	}
}

// Write writes a single JSON value as a line
func (w *JSONLWriter) Write(data any) error {
	if w.err != nil {
		return w.err
	}

	if err := w.encoder.Encode(data); err != nil {
		w.err = err
		return err
	}

	w.lineNum++
	return nil
}

// WriteAll writes multiple JSON values as lines
func (w *JSONLWriter) WriteAll(data []any) error {
	for _, d := range data {
		if err := w.Write(d); err != nil {
			return err
		}
	}
	return nil
}

// WriteRaw writes a raw JSON line (already encoded)
func (w *JSONLWriter) WriteRaw(line []byte) error {
	if w.err != nil {
		return w.err
	}

	n, err := w.writer.Write(line)
	if err != nil {
		w.err = err
		return err
	}
	w.bytesOut += int64(n)

	// Add newline if not present
	if len(line) == 0 || line[len(line)-1] != '\n' {
		if _, err := w.writer.Write([]byte{'\n'}); err != nil {
			w.err = err
			return err
		}
		w.bytesOut++
	}

	w.lineNum++
	return nil
}

// Err returns any error encountered during writing
func (w *JSONLWriter) Err() error {
	return w.err
}

// Stats returns writing statistics
func (w *JSONLWriter) Stats() JSONLStats {
	return JSONLStats{
		LinesProcessed: int64(w.lineNum),
		BytesRead:      w.bytesOut,
	}
}

// ============================================================================
// UTILITY FUNCTIONS
// ============================================================================

// ParseJSONL parses JSONL data from a byte slice
func ParseJSONL(data []byte) ([]any, error) {
	processor := NewJSONLProcessor(bytes.NewReader(data))
	defer processor.Release()

	var results []any
	err := processor.StreamLines(func(lineNum int, data any) bool {
		results = append(results, data)
		return true
	})

	return results, err
}

// ParseJSONLInto parses JSONL data into typed values
func ParseJSONLInto[T any](data []byte) ([]T, error) {
	return StreamLinesInto[T](bytes.NewReader(data), nil)
}

// ToJSONL converts a slice of values to JSONL format
func ToJSONL(data []any) ([]byte, error) {
	if len(data) == 0 {
		return []byte{}, nil
	}

	// Estimate buffer size
	estimatedSize := len(data) * 64 // Rough estimate
	if estimatedSize > 64*1024 {
		estimatedSize = 64 * 1024
	}

	buf := internal.GetEncoderBuffer()
	defer internal.PutEncoderBuffer(buf)
	buf.Reset()

	// Grow buffer if needed
	if buf.Cap() < estimatedSize {
		buf.Grow(estimatedSize - buf.Cap())
	}

	for _, item := range data {
		encoded, err := json.Marshal(item)
		if err != nil {
			return nil, err
		}
		buf.Write(encoded)
		buf.WriteByte('\n')
	}

	result := make([]byte, buf.Len())
	copy(result, buf.Bytes())
	return result, nil
}

// ToJSONLString converts a slice of values to JSONL format string
func ToJSONLString(data []any) (string, error) {
	result, err := ToJSONL(data)
	if err != nil {
		return "", err
	}
	return string(result), nil
}
