//go:build example

package main

import (
	"fmt"
	"strings"

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
// - NDJSONProcessor for file processing
// - Processor JSONL streaming methods
//
// Run: go run -tags=example examples/15_jsonl_processing.go

func main() {
	fmt.Println("JSON Library - JSONL Processing")
	fmt.Println("================================")

	// 1. JSONL CONVERSION
	demonstrateJSONLConversion()

	// 2. JSONL WRITER
	demonstrateJSONLWriter()

	// 3. PROCESSOR JSONL METHODS
	demonstrateProcessorJSONL()

	// 4. NDJSON PROCESSOR
	demonstrateNDJSONProcessor()

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

	// Write individual records
	_ = writer.Write(map[string]any{"event": "click", "target": "button"})
	_ = writer.Write(map[string]any{"event": "scroll", "target": "page"})
	_ = writer.Write(map[string]any{"event": "submit", "target": "form"})

	// Write raw JSON line
	_ = writer.WriteRaw([]byte(`{"event":"hover","target":"link"}`))

	fmt.Printf("   Written JSONL:\n   %s", buf.String())

	// Get statistics
	stats := writer.Stats()
	fmt.Printf("   Stats: lines=%d, bytes=%d\n", stats.LinesProcessed, stats.BytesWritten)

	// WriteAll for batch writing
	var buf2 strings.Builder
	writer2 := json.NewJSONLWriter(&buf2)
	_ = writer2.WriteAll([]any{
		map[string]any{"batch": 1, "count": 10},
		map[string]any{"batch": 2, "count": 20},
	})
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
		return acc.(int) + item.GetInt("score")
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
	}
}

func demonstrateNDJSONProcessor() {
	fmt.Println("\n4. NDJSONProcessor")
	fmt.Println("-------------------")

	// NDJSONProcessor processes JSONL from io.Reader
	ndprocessor := json.NewNDJSONProcessor()

	jsonlData := `{"type":"log","level":"info","msg":"started"}
{"type":"log","level":"warn","msg":"slow query"}
{"type":"log","level":"error","msg":"connection failed"}
{"type":"log","level":"info","msg":"recovered"}`

	reader := strings.NewReader(jsonlData)

	err := ndprocessor.ProcessReader(reader, func(lineNum int, obj map[string]any) error {
		level, _ := obj["level"].(string)
		msg, _ := obj["msg"].(string)
		fmt.Printf("   [%d] %-5s %s\n", lineNum, level, msg)
		return nil
	})
	if err != nil {
		fmt.Printf("   ProcessReader error: %v\n", err)
	}

	// CollectJSONL - collect all items
	fmt.Println("\n   CollectJSONL (collect all items):")
	processor, err := json.New(json.DefaultConfig())
	if err != nil {
		fmt.Printf("   New error: %v\n", err)
		return
	}
	defer processor.Close()

	reader2 := strings.NewReader(jsonlData)
	items, err := processor.CollectJSONL(reader2)
	if err != nil {
		fmt.Printf("   CollectJSONL error: %v\n", err)
	}
	fmt.Printf("   Collected %d items\n", len(items))
}
