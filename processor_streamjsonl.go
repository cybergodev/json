package json

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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

// jsonlScanLimits resolves the effective scanner buffer capacity and maximum
// line size for a JSONL engine from its Config. All five scanner sites
// (StreamJSONL / Parallel / Chunked, NDJSONProcessor.ProcessReader,
// StreamLinesInto) share this helper so the same input meets the same limits
// regardless of entry point (D-002: the NDJSON engine previously fell back to
// MaxJSONSize → 100MB while the StreamJSONL family fell back to 1MB, and the
// token cap was off by one between engines — one accepted a line the other
// rejected with bufio.ErrTooLong).
//
// The token cap is maxLine+1 so a line of EXACTLY JSONLMaxLineSize bytes is
// accepted, matching MaxJSONSize's "exceeds" semantics elsewhere.
func jsonlScanLimits(cfg *Config) (bufCap, maxToken int) {
	bufCap = cfg.JSONLBufferSize
	if bufCap <= 0 {
		bufCap = defaultScannerBufSize
	}
	maxLine := cfg.JSONLMaxLineSize
	if maxLine <= 0 {
		maxLine = defaultMaxLineSize
	}
	// bufio.Scanner's effective token cap is max(cap(buf), max): clamp the
	// initial capacity so a JSONLMaxLineSize smaller than the buffer size is
	// actually enforced.
	if bufCap > maxLine+1 {
		bufCap = maxLine + 1
	}
	return bufCap, maxLine + 1
}

// skipJSONLLine reports whether a JSONL line must be skipped before parsing.
// Whitespace-only lines are ALWAYS skipped — such a line can never be a valid
// JSONL record, and the engines previously disagreed on them (NDJSON skipped
// only zero-length lines, StreamJSONL errored unless JSONLSkipEmpty was set,
// which made the same file succeed or fail depending on the entry point;
// D-002). Comment lines are skipped when JSONLSkipComments is configured.
func skipJSONLLine(line []byte, cfg *Config) bool {
	if len(bytes.TrimSpace(line)) == 0 {
		return true
	}
	return shouldSkipJSONLLineFromConfig(line, cfg)
}

// decodeJSONLLine parses one already-depth-checked JSONL line into a fresh
// any, honoring the engine's effective PreserveNumbers setting (D-002/R9 m4:
// the engines previously hard-coded stdlib float64 semantics, ignoring both
// the baked and the per-call config — the same divergence class C1 fixed for
// the parse funnel).
func decodeJSONLLine(line []byte, preserveNumbers bool) (any, error) {
	if !preserveNumbers {
		var data any
		if err := json.Unmarshal(line, &data); err != nil {
			return nil, err
		}
		return data, nil
	}
	return newNumberPreservingDecoder(true).DecodeToAny(string(line))
}

// decodeJSONLObject parses one JSONL line into a map[string]any, honoring
// PreserveNumbers (see decodeJSONLLine). A valid-JSON-but-not-an-object line
// fails with the same condition encoding/json reports for a map destination.
func decodeJSONLObject(line []byte, preserveNumbers bool) (map[string]any, error) {
	if !preserveNumbers {
		var obj map[string]any
		if err := json.Unmarshal(line, &obj); err != nil {
			return nil, err
		}
		return obj, nil
	}
	data, err := newNumberPreservingDecoder(true).DecodeToAny(string(line))
	if err != nil {
		return nil, err
	}
	obj, ok := data.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("json: cannot unmarshal %s into Go value of type map[string]any", jsonKindOfLiteral(line))
	}
	return obj, nil
}

// resolveJSONLOptions returns the Config governing a JSONL/stream operation.
//
// With no cfg it returns &p.config — the processor's baked configuration,
// exactly what these engines read before per-call Config support existed, so
// no-cfg behavior is unchanged. A supplied cfg is deep-copied and validated
// (clamped) the same way New(cfg) bakes it, then used for this call only —
// the replace semantics every other cfg-accepting Processor method follows.
//
// Not drawn from configPool: the pointer is held for the whole stream, so
// pool discipline would have to span every return path of three engines (plus
// panic recovery) and a future refactor could return &p.config itself to the
// pool. One Clone per explicit-cfg call is negligible next to stream I/O.
func (p *Processor) resolveJSONLOptions(cfg ...Config) (*Config, error) {
	if len(cfg) == 0 {
		return &p.config, nil
	}
	c := cfg[0].Clone()
	if err := c.Validate(); err != nil {
		return nil, err
	}
	return c, nil
}

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
//
// MEMORY LIMIT (D-002 doc): the total-bytes cap falls back
// JSONLMaxMemory → MaxMemory; when both are 0 (the default) this reader is
// NOT bounded in total bytes — set JSONLMaxMemory for untrusted streams
// (StreamIterator, by contrast, always applies DefaultMaxJSONSize).
//
// The optional trailing Config overrides the processor's JSONL settings
// (buffer/line sizes, memory limit, nesting cap, JSONLSkipComments,
// JSONLContinueOnErr) for this call only; omitted, the baked configuration
// applies.
func (p *Processor) StreamJSONL(reader io.Reader, fn func(lineNum int, item *IterableValue) error, cfg ...Config) (err error) {
	// SAFETY (SEC-003): a panicking user callback (or any unexpected panic during
	// the stream) must not crash the program. Recover and surface as an error.
	// Registered before beginGovernedOp so the governance release (endGovernedOp)
	// still runs on panic — defers unwind LIFO, so endGovernedOp fires first.
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("jsonl callback panicked: %v", r)
		}
	}()
	// Concurrency governance for the full stream duration. A StreamJSONL call on a
	// config-cached processor can run for many seconds, so per-op governance (as
	// Get/Set provide) would not protect it: between lines activeOps drops to zero and
	// a concurrent eviction Close() could tear the processor down mid-stream.
	// Registering once at entry pins the processor until the stream completes. This
	// method unmarshals each line via the stdlib directly (not p.Unmarshal/p.Parse),
	// so the acquisition is never nested under another governed op.
	if err := p.beginGovernedOp(); err != nil {
		return err
	}
	defer p.endGovernedOp()

	// Per-call Config (D-005 Phase 2): a supplied cfg's JSONL settings replace
	// the baked ones for this call only; resolveJSONLOptions keeps the no-cfg
	// path on &p.config, byte-identical to the previous behavior.
	opts, err := p.resolveJSONLOptions(cfg...)
	if err != nil {
		return err
	}

	// Determine effective memory limit for JSONL processing
	memLimit := opts.JSONLMaxMemory
	if memLimit <= 0 && opts.MaxMemory > 0 {
		memLimit = opts.MaxMemory
	}

	bufSize, maxToken := jsonlScanLimits(opts)
	// SECURITY: per-line nesting cap to prevent stack overflow from deeply nested
	// JSONL payloads (mirrors NDJSONProcessor.ProcessReader in file.go).
	maxDepth := opts.MaxNestingDepthSecurity
	if maxDepth <= 0 {
		maxDepth = DefaultMaxNestingDepth
	}

	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, bufSize), maxToken)

	lineNum := 0
	var totalBytes int64

	for scanner.Scan() {
		lineNum++

		line := scanner.Bytes()

		// Skip lines based on config (blank lines always, comments when configured)
		if skipJSONLLine(line, opts) {
			continue
		}

		// Track memory usage if limit is configured
		if memLimit > 0 {
			totalBytes += int64(len(line))
			if totalBytes > memLimit {
				// D-002/R8 (m2): carry the ErrSizeLimit sentinel like
				// NDJSONProcessor (file.go) so errors.Is works uniformly across
				// the JSONL family.
				return &JsonsError{
					Op:      "stream_jsonl",
					Message: fmt.Sprintf("jsonl memory limit exceeded: processed %d bytes (limit %d bytes at line %d)", totalBytes, memLimit, lineNum),
					Err:     ErrSizeLimit,
				}
			}
		}

		// SECURITY: per-line nesting check before unmarshaling (prevents stack overflow
		// from deeply nested payloads). JSONLContinueOnErr downgrades depth and
		// parse failures to skips, matching NDJSONProcessor.ProcessReader and
		// StreamLinesInto (D-002: this knob was previously ignored here).
		if err := checkNestingDepth(line, maxDepth); err != nil {
			if opts.JSONLContinueOnErr {
				continue
			}
			return fmt.Errorf("line %d: %w", lineNum, err)
		}

		// Parse JSON line (honors PreserveNumbers — D-002/R9 m4)
		data, err := decodeJSONLLine(line, opts.PreserveNumbers)
		if err != nil {
			if opts.JSONLContinueOnErr {
				continue
			}
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
func (p *Processor) StreamJSONLParallel(reader io.Reader, workers int, fn func(lineNum int, item *IterableValue) error, cfg ...Config) error {
	return p.StreamJSONLParallelWithContext(context.Background(), reader, workers, fn, cfg...)
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
//
// The optional trailing Config overrides the processor's JSONL settings for
// this call only; omitted, the baked configuration applies.
func (p *Processor) StreamJSONLParallelWithContext(ctx context.Context, reader io.Reader, workers int, fn func(lineNum int, item *IterableValue) error, cfg ...Config) (retErr error) {
	// Concurrency governance for the full parallel stream (see StreamJSONL for the
	// rationale: pinning once at entry beats per-line governance, which leaves the
	// processor unprotected between lines). The in-flight slot is held by this
	// (scanner) goroutine for the whole run; worker goroutines do not register
	// separately. Calls json.Unmarshal directly, so never nested under another op.
	if err := p.beginGovernedOp(); err != nil {
		return err
	}
	defer p.endGovernedOp()

	// Per-call Config (D-005 Phase 2): see StreamJSONL.
	opts, err := p.resolveJSONLOptions(cfg...)
	if err != nil {
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
			// SAFETY: a panic inside the user callback (or any unexpected panic) must
			// not tear down the process; convert it to an error reported to the caller.
			defer func() {
				if r := recover(); r != nil {
					if atomic.CompareAndSwapInt32(&errCount, 0, 1) {
						e := fmt.Errorf("jsonl worker panicked: %v", r)
						firstErr.Store(&e)
					}
				}
			}()
			for job := range jobs {
				// RESOURCE FIX: Check context cancellation in workers
				select {
				case <-ctx.Done():
					return
				default:
				}
				if atomic.LoadInt32(&errCount) > 0 {
					// Another worker hit an error or the consumer requested a break.
					// Exit immediately instead of `continue`-ing to drain the rest of
					// the (bounded) jobs channel performing no useful work. The feed
					// loop breaks on errCount and closes(jobs), so other workers still
					// range-out cleanly; defer wg.Done() runs on this return.
					return
				}
				item := newIterableValue(job.data)
				if jobErr := fn(job.lineNum, item); jobErr != nil {
					if errors.Is(jobErr, errBreak) {
						// Clean stop: signal the feed loop and other workers to
						// drain via errCount, mirroring serial StreamJSONL where
						// errBreak maps to a nil return. Do NOT store firstErr,
						// so the final return is nil (not an error). CAS keeps a
						// real error dominant if one was already recorded.
						atomic.CompareAndSwapInt32(&errCount, 0, 1)
					} else if atomic.CompareAndSwapInt32(&errCount, 0, 1) {
						firstErr.Store(&jobErr)
					}
				}
			}
		}()
	}

	// Feed jobs — respect context cancellation during scan
	lineNum := 0
	parBufSize, parMaxToken := jsonlScanLimits(opts)
	// Total-bytes cap, mirroring the serial/chunked engines (D-002/R10: the
	// parallel engine was the only JSONL reader without it — JSONLMaxMemory
	// falls back to MaxMemory; zero disables accounting).
	memLimit := opts.JSONLMaxMemory
	if memLimit <= 0 && opts.MaxMemory > 0 {
		memLimit = opts.MaxMemory
	}
	var totalBytes int64
	// SECURITY: per-line nesting cap to prevent stack overflow from deeply nested
	// JSONL payloads. The feed loop parses each line before dispatching to workers,
	// so the check belongs here (the overflow would happen in this goroutine).
	maxDepth := opts.MaxNestingDepthSecurity
	if maxDepth <= 0 {
		maxDepth = DefaultMaxNestingDepth
	}
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, parBufSize), parMaxToken)

	// P-002: close(jobs) and wg.Wait() run on EVERY exit path — including a
	// panic in the feed loop below (it has no recover, unlike the workers).
	// Previously the explicit close+wait pairs covered only the known returns:
	// a feed-side panic skipped close(jobs) and left every worker blocked
	// forever on `for job := range jobs`. The wait must also happen BEFORE
	// firstErr is read (workers publish their error before wg.Done), so the
	// post-loop error selection lives in this defer, preserving the original
	// priority — worker error > scanner error > ctx cancellation — and never
	// overriding a more specific error the body already chose to return.
	defer func() {
		close(jobs)
		wg.Wait()
		if retErr != nil {
			return
		}
		if storedErr := firstErr.Load(); storedErr != nil {
			retErr = *storedErr
			return
		}
		if serr := scanner.Err(); serr != nil {
			retErr = serr
			return
		}
		retErr = ctx.Err()
	}()

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

		// Skip lines based on config (blank lines always, comments when configured)
		if skipJSONLLine(line, opts) {
			continue
		}

		// Total-bytes cap (see memLimit above; ErrSizeLimit sentinel like the
		// serial/chunked engines — D-002/R9 m2).
		if memLimit > 0 {
			totalBytes += int64(len(line))
			if totalBytes > memLimit {
				// close(jobs)+wg.Wait() run via the defers registered above.
				return &JsonsError{
					Op:      "stream_jsonl_parallel",
					Message: fmt.Sprintf("jsonl memory limit exceeded: processed %d bytes (limit %d bytes at line %d)", totalBytes, memLimit, lineNum),
					Err:     ErrSizeLimit,
				}
			}
		}

		// SECURITY: per-line nesting check before unmarshaling (prevents stack overflow
		// from deeply nested payloads). JSONLContinueOnErr downgrades depth and
		// parse failures to skips, matching the serial/chunked engines (D-002).
		if err := checkNestingDepth(line, maxDepth); err != nil {
			if opts.JSONLContinueOnErr {
				continue
			}
			return fmt.Errorf("line %d: %w", lineNum, err)
		}

		// Parse JSON line (honors PreserveNumbers — D-002/R9 m4)
		data, err := decodeJSONLLine(line, opts.PreserveNumbers)
		if err != nil {
			if opts.JSONLContinueOnErr {
				continue
			}
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

	// Feed loop complete (normal end, ctx cancellation, or worker error).
	// close(jobs), wg.Wait(), and the error selection (firstErr > scanner
	// error > ctx.Err) run in the defer registered above — the wait must
	// precede reading firstErr, and the deferred form also covers a panic
	// in this feed loop (P-002).
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
//
// The optional trailing Config overrides the processor's JSONL settings for
// this call only; omitted, the baked configuration applies.
func (p *Processor) StreamJSONLChunked(reader io.Reader, chunkSize int, fn func(chunk []*IterableValue) error, cfg ...Config) (err error) {
	// SAFETY (SEC-003): a panicking user callback must not crash the program.
	// Registered first so the pool-cleanup and governance defers (registered later)
	// still run on panic — defers unwind LIFO, so they fire before this recover.
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("jsonl chunk callback panicked: %v", r)
		}
	}()
	// Concurrency governance for the full chunked stream (see StreamJSONL for the
	// rationale). Calls json.Unmarshal directly, so never nested under another op.
	if err := p.beginGovernedOp(); err != nil {
		return err
	}
	defer p.endGovernedOp()

	// Per-call Config (D-005 Phase 2): see StreamJSONL.
	opts, err := p.resolveJSONLOptions(cfg...)
	if err != nil {
		return err
	}

	if chunkSize <= 0 {
		chunkSize = 1000
	}

	// Determine effective memory limit for JSONL processing
	memLimit := opts.JSONLMaxMemory
	if memLimit <= 0 && opts.MaxMemory > 0 {
		memLimit = opts.MaxMemory
	}

	var chunk []*IterableValue
	// Return any accumulated pool objects on every return path, including the
	// early-return error paths below (memory limit, nesting, parse, scanner).
	// The flush points reset chunk after returning their objects, so this defer
	// only fires when we bail out with a partially filled chunk.
	defer func() {
		releaseIterableValues(chunk)
	}()

	chunkBufSize, chunkMaxToken := jsonlScanLimits(opts)
	// SECURITY: per-line nesting cap to prevent stack overflow from deeply nested
	// JSONL payloads (mirrors NDJSONProcessor.ProcessReader in file.go).
	maxDepth := opts.MaxNestingDepthSecurity
	if maxDepth <= 0 {
		maxDepth = DefaultMaxNestingDepth
	}
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, chunkBufSize), chunkMaxToken)

	lineNum := 0
	var totalBytes int64

	for scanner.Scan() {
		lineNum++

		line := scanner.Bytes()

		// Skip lines based on config (blank lines always, comments when configured)
		if skipJSONLLine(line, opts) {
			continue
		}

		// Track memory usage if limit is configured
		if memLimit > 0 {
			totalBytes += int64(len(line))
			if totalBytes > memLimit {
				// D-002/R8 (m2): ErrSizeLimit sentinel, matching NDJSONProcessor
				// and the serial engine above.
				return &JsonsError{
					Op:      "stream_jsonl_chunked",
					Message: fmt.Sprintf("jsonl memory limit exceeded: processed %d bytes (limit %d bytes at line %d)", totalBytes, memLimit, lineNum),
					Err:     ErrSizeLimit,
				}
			}
		}

		// SECURITY: per-line nesting check before unmarshaling (prevents stack overflow
		// from deeply nested payloads). JSONLContinueOnErr downgrades depth and
		// parse failures to skips, matching the serial/parallel engines (D-002).
		if err := checkNestingDepth(line, maxDepth); err != nil {
			if opts.JSONLContinueOnErr {
				continue
			}
			return fmt.Errorf("line %d: %w", lineNum, err)
		}

		// Parse JSON line (honors PreserveNumbers — D-002/R9 m4)
		data, err := decodeJSONLLine(line, opts.PreserveNumbers)
		if err != nil {
			if opts.JSONLContinueOnErr {
				continue
			}
			return fmt.Errorf("line %d: %w", lineNum, err)
		}

		item := newIterableValue(data)
		chunk = append(chunk, item)

		if len(chunk) >= chunkSize {
			if err := fn(chunk); err != nil {
				releaseIterableValues(chunk)
				chunk = chunk[:0]
				return err
			}
			releaseIterableValues(chunk)
			chunk = chunk[:0]
		}
	}

	if err := scanner.Err(); err != nil {
		return err
	}

	// Process remaining chunk
	if len(chunk) > 0 {
		if err := fn(chunk); err != nil {
			releaseIterableValues(chunk)
			chunk = chunk[:0]
			return err
		}
		releaseIterableValues(chunk)
		chunk = chunk[:0]
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
//
// The optional trailing Config is forwarded to StreamJSONL (per-call override
// of the processor's JSONL settings).
func (p *Processor) ForeachJSONL(reader io.Reader, fn func(lineNum int, item *IterableValue) error, cfg ...Config) error {
	if err := p.checkClosed(); err != nil {
		return err
	}

	return p.StreamJSONL(reader, fn, cfg...)
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
//
// The optional trailing Config is forwarded to StreamJSONL (per-call override
// of the processor's JSONL settings).
func (p *Processor) MapJSONL(reader io.Reader, fn func(lineNum int, item *IterableValue) (any, error), cfg ...Config) ([]any, error) {
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
	}, cfg...)

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
//
// The optional trailing Config is forwarded to StreamJSONL (per-call override
// of the processor's JSONL settings).
func (p *Processor) ReduceJSONL(reader io.Reader, initial any, fn func(acc any, item *IterableValue) any, cfg ...Config) (any, error) {
	if err := p.checkClosed(); err != nil {
		return initial, err
	}

	acc := initial

	err := p.StreamJSONL(reader, func(_ int, item *IterableValue) error {
		acc = fn(acc, item)
		return nil
	}, cfg...)

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
//
// The optional trailing Config is forwarded to StreamJSONL (per-call override
// of the processor's JSONL settings).
func (p *Processor) FilterJSONL(reader io.Reader, predicate func(item *IterableValue) bool, cfg ...Config) ([]*IterableValue, error) {
	if err := p.checkClosed(); err != nil {
		return nil, err
	}

	var results []*IterableValue

	err := p.StreamJSONL(reader, func(_ int, item *IterableValue) error {
		if predicate(item) {
			results = append(results, item)
		}
		return nil
	}, cfg...)

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
//
// The optional trailing Config is forwarded to StreamJSONL (per-call override
// of the processor's JSONL settings).
func (p *Processor) StreamJSONLFile(filename string, fn func(lineNum int, item *IterableValue) error, cfg ...Config) error {
	if err := p.checkClosed(); err != nil {
		return err
	}

	// SECURITY: Validate file path to prevent path traversal attacks; cfg is
	// forwarded so a per-call AllowedFileDirs override applies here too
	// (GEN-001 review follow-up).
	if err := p.validateFilePath(filename, cfg...); err != nil {
		return err
	}

	// Open the symlink-resolved location validated above (GEN-001 TOCTOU
	// narrowing; see openValidatedFile).
	file, err := openValidatedFile(filename)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer func() { _ = file.Close() }() // best-effort cleanup

	return p.StreamJSONL(file, fn, cfg...)
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
//
// The optional trailing Config is forwarded to StreamJSONL (per-call override
// of the processor's JSONL settings).
func (p *Processor) CollectJSONL(reader io.Reader, cfg ...Config) ([]*IterableValue, error) {
	if err := p.checkClosed(); err != nil {
		return nil, err
	}

	var items []*IterableValue

	err := p.StreamJSONL(reader, func(_ int, item *IterableValue) error {
		items = append(items, item)
		return nil
	}, cfg...)

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
//
// The optional trailing Config is forwarded to StreamJSONL (per-call override
// of the processor's JSONL settings).
func (p *Processor) FirstJSONL(reader io.Reader, predicate func(item *IterableValue) bool, cfg ...Config) (*IterableValue, bool, error) {
	if err := p.checkClosed(); err != nil {
		return nil, false, err
	}

	var result *IterableValue
	found := false

	err := p.StreamJSONL(reader, func(_ int, item *IterableValue) error {
		if predicate(item) {
			result = item
			found = true
			return errBreak
		}
		return nil
	}, cfg...)

	if err != nil {
		return nil, false, err
	}

	return result, found, nil
}

// ============================================================================
// Package-level JSONL wrappers (dual-layer design)
// Delegate to a processor for convenience. Each accepts an optional trailing
// Config: when omitted it uses the default processor (behavior unchanged); when
// supplied it selects a config-cached processor whose baked-in JSONL settings
// (workers, buffer/line sizes, memory limits) reflect cfg. Explicit parameters
// (e.g. StreamJSONLParallel's workers) still take precedence over cfg fields.
// The Processor methods these wrap also accept their own trailing Config
// (D-005 Phase 2); the package level keeps the baked-processor route because
// it reuses the config-keyed processor cache across repeated calls.
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
func StreamJSONL(reader io.Reader, fn func(lineNum int, item *IterableValue) error, cfg ...Config) error {
	p, err := processorForCfg(cfg...)
	if err != nil {
		return err
	}
	return p.StreamJSONL(reader, fn)
}

// StreamJSONLParallel processes JSONL data in parallel with multiple workers.
//
// Example:
//
//	err := json.StreamJSONLParallel(reader, 4, func(lineNum int, item *json.IterableValue) error {
//		// Process each item in parallel
//		return nil
//	})
func StreamJSONLParallel(reader io.Reader, workers int, fn func(lineNum int, item *IterableValue) error, cfg ...Config) error {
	p, err := processorForCfg(cfg...)
	if err != nil {
		return err
	}
	return p.StreamJSONLParallel(reader, workers, fn)
}

// StreamJSONLParallelWithContext processes JSONL data in parallel with context support
// for cancellation. See Processor.StreamJSONLParallelWithContext for details.
func StreamJSONLParallelWithContext(ctx context.Context, reader io.Reader, workers int, fn func(lineNum int, item *IterableValue) error, cfg ...Config) error {
	p, err := processorForCfg(cfg...)
	if err != nil {
		return err
	}
	return p.StreamJSONLParallelWithContext(ctx, reader, workers, fn)
}

// StreamJSONLChunked processes JSONL data in chunks for memory-efficient processing.
//
// Example:
//
//	err := json.StreamJSONLChunked(reader, 1000, func(chunk []*json.IterableValue) error {
//		// Process chunk of 1000 items
//		return nil
//	})
func StreamJSONLChunked(reader io.Reader, chunkSize int, fn func(chunk []*IterableValue) error, cfg ...Config) error {
	p, err := processorForCfg(cfg...)
	if err != nil {
		return err
	}
	return p.StreamJSONLChunked(reader, chunkSize, fn)
}

// ForeachJSONL iterates over JSONL data with IterableValue callback.
//
// Example:
//
//	err := json.ForeachJSONL(reader, func(lineNum int, item *json.IterableValue) error {
//		fmt.Printf("Line: %d, Value: %v\n", lineNum, item.GetData())
//		return nil
//	})
func ForeachJSONL(reader io.Reader, fn func(lineNum int, item *IterableValue) error, cfg ...Config) error {
	p, err := processorForCfg(cfg...)
	if err != nil {
		return err
	}
	return p.ForeachJSONL(reader, fn)
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
func MapJSONL(reader io.Reader, fn func(lineNum int, item *IterableValue) (any, error), cfg ...Config) ([]any, error) {
	p, err := processorForCfg(cfg...)
	if err != nil {
		return nil, err
	}
	return p.MapJSONL(reader, fn)
}

// ReduceJSONL reduces JSONL data to a single aggregated result using a reducer function.
//
// Example:
//
//	totalAge, err := json.ReduceJSONL(reader, 0, func(acc any, item *json.IterableValue) any {
//		return acc.(int64) + int64(item.GetInt("age"))
//	})
func ReduceJSONL(reader io.Reader, initial any, fn func(acc any, item *IterableValue) any, cfg ...Config) (any, error) {
	// Note: Cannot use withProcessor because it returns zero-value on error,
	// but ReduceJSONL must return the initial accumulator on error.
	p, err := processorForCfg(cfg...)
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
func FilterJSONL(reader io.Reader, predicate func(item *IterableValue) bool, cfg ...Config) ([]*IterableValue, error) {
	p, err := processorForCfg(cfg...)
	if err != nil {
		return nil, err
	}
	return p.FilterJSONL(reader, predicate)
}

// StreamJSONLFile streams JSONL data from a file with IterableValue callback.
//
// Example:
//
//	err := json.StreamJSONLFile("data.jsonl", func(lineNum int, item *json.IterableValue) error {
//		fmt.Printf("Line %d: %v\n", lineNum, item.GetData())
//		return nil
//	})
func StreamJSONLFile(filename string, fn func(lineNum int, item *IterableValue) error, cfg ...Config) error {
	p, err := processorForCfg(cfg...)
	if err != nil {
		return err
	}
	return p.StreamJSONLFile(filename, fn)
}

// CollectJSONL collects all JSONL items into a slice.
//
// Example:
//
//	items, err := json.CollectJSONL(reader)
//	for _, item := range items {
//		fmt.Println(item.GetString("name"))
//	}
func CollectJSONL(reader io.Reader, cfg ...Config) ([]*IterableValue, error) {
	p, err := processorForCfg(cfg...)
	if err != nil {
		return nil, err
	}
	return p.CollectJSONL(reader)
}

// FirstJSONL returns the first JSONL item that matches a predicate.
//
// Example:
//
//	user, found, err := json.FirstJSONL(reader, func(item *json.IterableValue) bool {
//		return item.GetString("name") == "Alice"
//	})
func FirstJSONL(reader io.Reader, predicate func(item *IterableValue) bool, cfg ...Config) (*IterableValue, bool, error) {
	// Note: Cannot use withProcessor because it only supports (T, error) return,
	// but FirstJSONL returns (*IterableValue, bool, error).
	p, err := processorForCfg(cfg...)
	if err != nil {
		return nil, false, err
	}
	return p.FirstJSONL(reader, predicate)
}
