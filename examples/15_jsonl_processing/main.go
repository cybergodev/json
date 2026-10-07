//go:build example

package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"

	"github.com/cybergodev/json"
)

// JSONL Processing Example
//
// This example demonstrates JSON Lines (JSONL/NDJSON) processing capabilities
// for streaming line-delimited JSON data.
//
// Topics covered:
// - JSONLWriter for writing JSONL output
// - ParseJSONL and ToJSONL conversion
// - StreamJSONL as the NDJSONProcessor replacement (deprecated, D-005)
// - Processor JSONL streaming methods
// - Package-level streaming: StreamJSONL, ForeachJSONL, StreamJSONLChunked,
//   StreamJSONLFile, StreamJSONLParallel, StreamJSONLParallelWithContext,
//   StreamLinesInto[T], and the JSONL mirror family (FilterJSONL, MapJSONL,
//   ReduceJSONL, FirstJSONL, CollectJSONL)
//
// Run: go run -tags=example ./examples/15_jsonl_processing

func main() {
	fmt.Println("JSON Library - JSONL Processing")
	fmt.Println("================================")

	// 1. JSONL CONVERSION
	demonstrateJSONLConversion()

	// 2. JSONL WRITER
	demonstrateJSONLWriter()

	// 3. PROCESSOR JSONL METHODS
	demonstrateProcessorJSONL()

	// 4. NDJSON PROCESSING (StreamJSONL — NDJSONProcessor replacement)
	demonstrateNDJSONReplacement()

	// 5. PACKAGE-LEVEL STREAMING
	demonstratePackageStreaming()

	fmt.Println("\nJSONL processing examples complete!")
}

func demonstrateJSONLConversion() {
	fmt.Println("1. JSONL Conversion (ParseJSONL / ToJSONL)")
	fmt.Println("--------------------------------------------")

	// Convert data to JSONL format
	records := []any{
		map[string]any{"id": 1, "name": "Alice", "active": true},
		map[string]any{"id": 2, "name": "Bob", "active": false},
		map[string]any{"id": 3, "name": "Charlie", "active": true},
	}

	// ToJSONL converts a slice to JSONL bytes
	jsonlBytes, err := json.ToJSONL(records)
	if err != nil {
		fmt.Printf("   ToJSONL error: %v\n", err)
		return
	}
	fmt.Println("   ToJSONL output:")
	fmt.Printf("   %s", string(jsonlBytes))

	// ToJSONLString returns a string
	jsonlStr, err := json.ToJSONLString(records)
	if err != nil {
		fmt.Printf("   ToJSONLString error: %v\n", err)
		return
	}
	fmt.Printf("\n   ToJSONLString output (length: %d chars):\n", len(jsonlStr))

	// ParseJSONL parses JSONL back to a slice
	parsed, err := json.ParseJSONL(jsonlBytes)
	if err != nil {
		fmt.Printf("   ParseJSONL error: %v\n", err)
		return
	}
	fmt.Printf("   ParseJSONL parsed %d records:\n", len(parsed))
	for _, item := range parsed {
		fmt.Printf("   - %v\n", item)
	}
}

func demonstrateJSONLWriter() {
	fmt.Println("\n2. JSONLWriter")
	fmt.Println("---------------")

	var buf strings.Builder
	writer := json.NewJSONLWriter(&buf)

	// Write individual records. The target is an in-memory strings.Builder, so
	// write failures are not expected in this demo — errors are best-effort.
	_ = writer.Write(map[string]any{"event": "click", "target": "button"}) // best-effort
	_ = writer.Write(map[string]any{"event": "scroll", "target": "page"})  // best-effort
	_ = writer.Write(map[string]any{"event": "submit", "target": "form"})  // best-effort

	// Write raw JSON line
	_ = writer.WriteRaw([]byte(`{"event":"hover","target":"link"}`)) // best-effort

	fmt.Printf("   Written JSONL:\n   %s", buf.String())

	// Get statistics
	stats := writer.Stats()
	fmt.Printf("   Stats: lines=%d, bytes=%d\n", stats.LinesProcessed, stats.BytesWritten)

	// WriteAll for batch writing
	var buf2 strings.Builder
	writer2 := json.NewJSONLWriter(&buf2)
	if err := writer2.WriteAll([]any{
		map[string]any{"batch": 1, "count": 10},
		map[string]any{"batch": 2, "count": 20},
	}); err != nil {
		fmt.Printf("   WriteAll error: %v\n", err)
		return
	}
	fmt.Printf("   WriteAll output:\n   %s", buf2.String())
}

func demonstrateProcessorJSONL() {
	fmt.Println("\n3. Processor JSONL Methods")
	fmt.Println("---------------------------")

	processor, err := json.New(json.DefaultConfig())
	if err != nil {
		fmt.Printf("   New error: %v\n", err)
		return
	}
	defer processor.Close()

	jsonlData := `{"id":1,"name":"Alice","score":95}
{"id":2,"name":"Bob","score":82}
{"id":3,"name":"Charlie","score":78}
{"id":4,"name":"Diana","score":91}
{"id":5,"name":"Eve","score":67}`

	reader := strings.NewReader(jsonlData)

	// StreamJSONL - iterate over each line
	fmt.Println("   StreamJSONL (iterate all lines):")
	lineCount := 0
	err = processor.StreamJSONL(reader, func(lineNum int, item *json.IterableValue) error {
		name := item.GetString("name")
		score := item.GetInt("score")
		fmt.Printf("   Line %d: %s (score=%d)\n", lineNum, name, score)
		lineCount++
		return nil
	})
	if err != nil {
		fmt.Printf("   StreamJSONL error: %v\n", err)
	}
	fmt.Printf("   Total lines processed: %d\n", lineCount)

	// FilterJSONL - filter records
	fmt.Println("\n   FilterJSONL (score >= 90):")
	reader2 := strings.NewReader(jsonlData)
	filtered, err := processor.FilterJSONL(reader2, func(item *json.IterableValue) bool {
		return item.GetInt("score") >= 90
	})
	if err != nil {
		fmt.Printf("   FilterJSONL error: %v\n", err)
	}
	for _, item := range filtered {
		fmt.Printf("   - %v\n", item.GetData())
	}

	// MapJSONL - transform records
	fmt.Println("\n   MapJSONL (add grade field):")
	reader3 := strings.NewReader(jsonlData)
	mapped, err := processor.MapJSONL(reader3, func(lineNum int, item *json.IterableValue) (any, error) {
		score := item.GetInt("score")
		grade := "C"
		if score >= 90 {
			grade = "A"
		} else if score >= 80 {
			grade = "B"
		}
		return map[string]any{
			"name":  item.GetString("name"),
			"score": score,
			"grade": grade,
		}, nil
	})
	if err != nil {
		fmt.Printf("   MapJSONL error: %v\n", err)
	}
	for _, item := range mapped {
		fmt.Printf("   - %v\n", item)
	}

	// ReduceJSONL - aggregate
	fmt.Println("\n   ReduceJSONL (sum scores):")
	reader4 := strings.NewReader(jsonlData)
	totalScore, err := processor.ReduceJSONL(reader4, 0, func(acc any, item *json.IterableValue) any {
		// acc starts as the int 0 above and stays an int, but guard the
		// assertion so a future change to the seed can't panic at runtime.
		sum, _ := acc.(int)
		return sum + item.GetInt("score")
	})
	if err != nil {
		fmt.Printf("   ReduceJSONL error: %v\n", err)
	}
	fmt.Printf("   Total score: %v\n", totalScore)

	// FirstJSONL - find first matching record
	fmt.Println("\n   FirstJSONL (first with score >= 90):")
	reader5 := strings.NewReader(jsonlData)
	first, found, err := processor.FirstJSONL(reader5, func(item *json.IterableValue) bool {
		return item.GetInt("score") >= 90
	})
	if err != nil {
		fmt.Printf("   FirstJSONL error: %v\n", err)
	}
	if found {
		fmt.Printf("   Found: %v\n", first.GetData())
	} else {
		fmt.Println("   No matching record found")
	}
}

func demonstrateNDJSONReplacement() {
	fmt.Println("\n4. NDJSON Processing (StreamJSONL)")
	fmt.Println("----------------------------------")

	// NDJSONProcessor is deprecated (D-005): it duplicated the StreamJSONL
	// family with a map[string]any callback. A Processor plus StreamJSONL is
	// the replacement — same limits, same JSONL config knobs, plus typed
	// access through *json.IterableValue.
	jsonlData := `{"type":"log","level":"info","msg":"started"}
{"type":"log","level":"warn","msg":"slow query"}
{"type":"log","level":"error","msg":"connection failed"}
{"type":"log","level":"info","msg":"recovered"}`

	processor, err := json.New(json.DefaultConfig())
	if err != nil {
		fmt.Printf("   New error: %v\n", err)
		return
	}
	defer processor.Close()

	// StreamJSONL replaces NDJSONProcessor.ProcessReader; item.GetData()
	// yields the decoded map when map[string]any access is preferred.
	err = processor.StreamJSONL(strings.NewReader(jsonlData), func(lineNum int, item *json.IterableValue) error {
		fmt.Printf("   [%d] %-5s %s\n", lineNum, item.GetString("level"), item.GetString("msg"))
		return nil
	})
	if err != nil {
		fmt.Printf("   StreamJSONL error: %v\n", err)
	}

	// CollectJSONL - collect all items
	fmt.Println("\n   CollectJSONL (collect all items):")

	reader2 := strings.NewReader(jsonlData)
	items, err := processor.CollectJSONL(reader2)
	if err != nil {
		fmt.Printf("   CollectJSONL error: %v\n", err)
	}
	fmt.Printf("   Collected %d items\n", len(items))
}

// LogRecord is the typed struct used by the StreamLinesInto demo below.
type LogRecord struct {
	Level string `json:"level"`
	Msg   string `json:"msg"`
}

func demonstratePackageStreaming() {
	fmt.Println("\n5. Package-Level Streaming (no Processor needed)")
	fmt.Println("--------------------------------------------------")

	jsonlData := `{"level":"info","msg":"service started"}
{"level":"warn","msg":"slow query"}
{"level":"error","msg":"connection failed"}
{"level":"info","msg":"recovered"}`

	// ForeachJSONL is the package-level streaming form (an alias pair of
	// Processor.StreamJSONL). Callback contract is the same: nil continues,
	// item.Break() stops cleanly, an error aborts and propagates.
	err := json.ForeachJSONL(strings.NewReader(jsonlData), func(lineNum int, item *json.IterableValue) error {
		fmt.Printf("   [%d] %-5s %s\n", lineNum, item.GetString("level"), item.GetString("msg"))
		return nil
	})
	if err != nil {
		fmt.Printf("   ForeachJSONL error: %v\n", err)
	}

	// StreamJSONLChunked processes fixed-size chunks — bounded memory for
	// large logs.
	fmt.Println("\n   StreamJSONLChunked (chunk size 2):")
	err = json.StreamJSONLChunked(strings.NewReader(jsonlData), 2, func(chunk []*json.IterableValue) error {
		levels := make([]string, 0, len(chunk))
		for _, item := range chunk {
			levels = append(levels, item.GetString("level"))
		}
		fmt.Printf("   - chunk: %v\n", levels)
		return nil
	})
	if err != nil {
		fmt.Printf("   StreamJSONLChunked error: %v\n", err)
	}

	// StreamJSONLFile streams a JSONL file directly from disk.
	file, err := os.CreateTemp("", "example-*.jsonl")
	if err != nil {
		fmt.Printf("   CreateTemp error: %v\n", err)
		return
	}
	defer os.Remove(file.Name())
	if _, err := file.WriteString(jsonlData); err != nil {
		fmt.Printf("   Write error: %v\n", err)
		file.Close()
		return
	}
	file.Close()

	lineCount := 0
	if err := json.StreamJSONLFile(file.Name(), func(lineNum int, item *json.IterableValue) error {
		lineCount++
		return nil
	}); err != nil {
		fmt.Printf("   StreamJSONLFile error: %v\n", err)
	}
	fmt.Printf("\n   StreamJSONLFile: %d lines from %s\n", lineCount, filepath.Base(file.Name()))

	// StreamJSONLParallel fans lines out to N workers. The callback runs
	// concurrently — shared state must be synchronized (here: atomic counter).
	var processed atomic.Int64
	err = json.StreamJSONLParallel(strings.NewReader(jsonlData), 2, func(lineNum int, item *json.IterableValue) error {
		processed.Add(1)
		return nil
	})
	if err != nil {
		fmt.Printf("   StreamJSONLParallel error: %v\n", err)
	}
	fmt.Printf("   StreamJSONLParallel: %d lines on 2 workers\n", processed.Load())

	// Package-level mirrors of the Processor JSONL family (section 3):
	// StreamJSONL, FilterJSONL, MapJSONL, ReduceJSONL, FirstJSONL and
	// CollectJSONL — same contracts, no Processor construction needed.
	fmt.Println("\n   Package-level JSONL mirrors (no Processor):")

	err = json.StreamJSONL(strings.NewReader(jsonlData), func(lineNum int, item *json.IterableValue) error {
		fmt.Printf("   StreamJSONL line %d: [%s] %s\n", lineNum, item.GetString("level"), item.GetString("msg"))
		return nil
	})
	if err != nil {
		fmt.Printf("   StreamJSONL error: %v\n", err)
	}

	errorLevel, err := json.FilterJSONL(strings.NewReader(jsonlData), func(item *json.IterableValue) bool {
		return item.GetString("level") == "error"
	})
	if err != nil {
		fmt.Printf("   FilterJSONL error: %v\n", err)
	} else {
		fmt.Printf("   FilterJSONL (level=error): %d record(s)\n", len(errorLevel))
	}

	mapped, err := json.MapJSONL(strings.NewReader(jsonlData), func(lineNum int, item *json.IterableValue) (any, error) {
		return strings.ToUpper(item.GetString("level")), nil
	})
	if err != nil {
		fmt.Printf("   MapJSONL error: %v\n", err)
	} else {
		fmt.Printf("   MapJSONL (uppercased levels): %v\n", mapped)
	}

	count, err := json.ReduceJSONL(strings.NewReader(jsonlData), 0, func(acc any, item *json.IterableValue) any {
		sum, _ := acc.(int) // seed is the int 0 above; guard against a changed seed
		return sum + 1
	})
	if err != nil {
		fmt.Printf("   ReduceJSONL error: %v\n", err)
	} else {
		fmt.Printf("   ReduceJSONL (line count): %v\n", count)
	}

	first, found, err := json.FirstJSONL(strings.NewReader(jsonlData), func(item *json.IterableValue) bool {
		return item.GetString("level") == "error"
	})
	if err != nil {
		fmt.Printf("   FirstJSONL error: %v\n", err)
	} else if found {
		fmt.Printf("   FirstJSONL (level=error): %s\n", first.GetString("msg"))
	}

	collected, err := json.CollectJSONL(strings.NewReader(jsonlData))
	if err != nil {
		fmt.Printf("   CollectJSONL error: %v\n", err)
	} else {
		fmt.Printf("   CollectJSONL: %d items\n", len(collected))
	}

	// StreamJSONLParallelWithContext adds context cancellation to the
	// parallel fan-out above — cancel and the returned error matches
	// context.Canceled.
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // pre-cancelled: the run aborts immediately
	err = json.StreamJSONLParallelWithContext(ctx, strings.NewReader(jsonlData), 2,
		func(lineNum int, item *json.IterableValue) error { return nil })
	fmt.Printf("   StreamJSONLParallelWithContext (cancelled): err=%v\n", err)
	fmt.Printf("     classified as context.Canceled: %t\n", errors.Is(err, context.Canceled))

	// StreamLinesInto[T]: generic, fully typed — each line unmarshals straight
	// into your struct and the collected results are returned.
	records, err := json.StreamLinesInto[LogRecord](strings.NewReader(jsonlData), func(lineNum int, rec LogRecord) error {
		_ = rec // per-line side effect (e.g. write to a typed sink)
		return nil
	})
	if err != nil {
		fmt.Printf("   StreamLinesInto error: %v\n", err)
		return
	}
	fmt.Printf("   StreamLinesInto[LogRecord]: %d typed records, first=%+v\n",
		len(records), records[0])
}
