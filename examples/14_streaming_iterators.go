//go:build example

package main

import (
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/cybergodev/json"
)

// Streaming Iterators Example
//
// This example demonstrates streaming and parallel iterator types
// for processing large JSON data efficiently.
//
// Topics covered:
// - StreamIterator for streaming large JSON arrays
// - StreamObjectIterator for streaming JSON objects
// - BatchIterator for chunked processing
// - ParallelIterator for concurrent processing
//
// Run: go run -tags=example examples/14_streaming_iterators.go

func main() {
	fmt.Println("JSON Library - Streaming Iterators")
	fmt.Println("===================================")

	// 1. STREAM ITERATOR
	demonstrateStreamIterator()

	// 2. STREAM OBJECT ITERATOR
	demonstrateStreamObjectIterator()

	// 3. BATCH ITERATOR
	demonstrateBatchIterator()

	// 4. PARALLEL ITERATOR
	demonstrateParallelIterator()

	fmt.Println("\nStreaming iterator examples complete!")
}

func demonstrateStreamIterator() {
	fmt.Println("1. StreamIterator (streaming array)")
	fmt.Println("------------------------------------")

	// Simulate a large JSON array stream
	arrayJSON := `[10, 20, 30, 40, 50, 60, 70, 80, 90, 100]`
	reader := strings.NewReader(arrayJSON)

	iter := json.NewStreamIterator(reader)
	fmt.Println("   Streaming array elements:")

	for iter.Next() {
		val := iter.Value()
		idx := iter.Index()
		fmt.Printf("   [%d] %v\n", idx, val)
	}

	if err := iter.Err(); err != nil {
		fmt.Printf("   Error: %v\n", err)
	}

	// Stream with objects
	objectArray := `[
		{"name": "Alice", "score": 95},
		{"name": "Bob", "score": 82},
		{"name": "Charlie", "score": 78}
	]`
	reader2 := strings.NewReader(objectArray)

	iter2 := json.NewStreamIterator(reader2)
	fmt.Println("\n   Streaming object array:")
	totalScore := 0.0

	for iter2.Next() {
		val := iter2.Value()
		if m, ok := val.(map[string]any); ok {
			name := m["name"]
			score := m["score"]
			fmt.Printf("   - %v: %v\n", name, score)
			if s, ok := score.(float64); ok {
				totalScore += s
			}
		}
	}

	fmt.Printf("   Average score: %.1f\n", totalScore/3)
}

func demonstrateStreamObjectIterator() {
	fmt.Println("\n2. StreamObjectIterator (streaming object)")
	fmt.Println("--------------------------------------------")

	// Stream a JSON object key by key
	objectJSON := `{
		"host": "localhost",
		"port": 8080,
		"debug": true,
		"database": "mydb",
		"max_conn": 100
	}`
	reader := strings.NewReader(objectJSON)

	iter := json.NewStreamObjectIterator(reader)
	fmt.Println("   Streaming object key-value pairs:")

	for iter.Next() {
		key := iter.Key()
		val := iter.Value()
		fmt.Printf("   %s = %v\n", key, val)
	}

	if err := iter.Err(); err != nil {
		fmt.Printf("   Error: %v\n", err)
	}
}

func demonstrateBatchIterator() {
	fmt.Println("\n3. BatchIterator (chunked processing)")
	fmt.Println("--------------------------------------")

	// Create data for batch processing
	data := make([]any, 10)
	for i := 0; i < 10; i++ {
		data[i] = map[string]any{
			"id":    i + 1,
			"value": fmt.Sprintf("item-%d", i+1),
		}
	}

	cfg := json.DefaultConfig()
	cfg.MaxBatchSize = 3 // Process 3 items at a time

	iter := json.NewBatchIterator(data, cfg)

	fmt.Printf("   Total items: %d, Batch size: 3\n", len(data))
	fmt.Printf("   Total batches: %d\n\n", iter.TotalBatches())

	batchNum := 0
	for iter.HasNext() {
		batch := iter.NextBatch()
		batchNum++
		fmt.Printf("   Batch %d (%d items): ", batchNum, len(batch))
		for _, item := range batch {
			if m, ok := item.(map[string]any); ok {
				fmt.Printf("[id=%v] ", m["id"])
			}
		}
		fmt.Println()
	}

	fmt.Printf("   Remaining after all batches: %d\n", iter.Remaining())

	// Reset and iterate again
	iter.Reset()
	fmt.Printf("   After Reset: HasNext=%t\n", iter.HasNext())
}

func demonstrateParallelIterator() {
	fmt.Println("\n4. ParallelIterator (concurrent processing)")
	fmt.Println("----------------------------------------------")

	// Create data for parallel processing
	data := make([]any, 20)
	for i := 0; i < 20; i++ {
		data[i] = map[string]any{
			"id":     i + 1,
			"value":  (i + 1) * 10,
			"active": i%2 == 0,
		}
	}

	cfg := json.DefaultConfig()
	cfg.MaxConcurrency = 4

	iter := json.NewParallelIterator(data, cfg)
	defer iter.Close()

	// ForEach - process all items concurrently
	fmt.Println("   ForEach (concurrent, 4 workers):")
	count := 0
	err := iter.ForEach(func(idx int, item any) error {
		count++
		return nil
	})
	if err != nil {
		fmt.Printf("   ForEach error: %v\n", err)
	}
	fmt.Printf("   Processed %d items\n", count)

	// Map - transform all items concurrently
	fmt.Println("\n   Map (concurrent transformation):")
	iter2 := json.NewParallelIterator(data, cfg)
	defer iter2.Close()

	results, err := iter2.Map(func(idx int, item any) (any, error) {
		if m, ok := item.(map[string]any); ok {
			return map[string]any{
				"id":      m["id"],
				"doubled": m["value"].(int) * 2, // safe: native Go int, not JSON-decoded
			}, nil
		}
		return nil, nil
	})
	if err != nil {
		fmt.Printf("   Map error: %v\n", err)
	} else {
		fmt.Printf("   Mapped %d items\n", len(results))
		for i := 0; i < 3 && i < len(results); i++ {
			fmt.Printf("   - %v\n", results[i])
		}
		if len(results) > 3 {
			fmt.Printf("   - ... (%d more)\n", len(results)-3)
		}
	}

	// Filter - select items concurrently
	fmt.Println("\n   Filter (concurrent selection):")
	iter3 := json.NewParallelIterator(data, cfg)
	defer iter3.Close()

	filtered := iter3.Filter(func(idx int, item any) bool {
		if m, ok := item.(map[string]any); ok {
			active, _ := m["active"].(bool)
			return active
		}
		return false
	})
	fmt.Printf("   Active items: %d (out of %d)\n", len(filtered), len(data))

	// ForEachBatch - process in batches concurrently
	// Note: batches execute in parallel, so output order is non-deterministic
	fmt.Println("\n   ForEachBatch (concurrent batches of 5):")
	iter4 := json.NewParallelIterator(data, cfg)
	defer iter4.Close()

	var batchMu sync.Mutex
	var batchResults []string
	err = iter4.ForEachBatch(5, func(batchIdx int, batch []any) error {
		batchMu.Lock()
		batchResults = append(batchResults, fmt.Sprintf("Batch %d: %d items", batchIdx+1, len(batch)))
		batchMu.Unlock()
		return nil
	})
	if err != nil {
		fmt.Printf("   ForEachBatch error: %v\n", err)
	}
	// Sort output for deterministic display
	sort.Strings(batchResults)
	for _, s := range batchResults {
		fmt.Printf("   %s\n", s)
	}
}
