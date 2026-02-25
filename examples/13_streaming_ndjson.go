//go:build example

package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/cybergodev/json"
)

// Streaming and NDJSON Processing Example
//
// This example demonstrates streaming processing for large JSON data
// and NDJSON (Newline-Delimited JSON) file processing.
//
// Topics covered:
// - StreamingProcessor for large JSON arrays
// - StreamArray and StreamObject for element-by-element processing
// - StreamArrayChunked for batch processing
// - StreamArrayFilter, StreamArrayMap, StreamArrayReduce
// - NDJSONProcessor for line-delimited JSON files
// - LazyJSON for deferred parsing
// - LargeFileProcessor for memory-efficient file handling
//
// Run: go run examples/13_streaming_ndjson.go

func main() {
	fmt.Println("Streaming & NDJSON Processing - JSON Library")
	fmt.Println("=============================================\n ")

	// 1. STREAMING PROCESSOR BASICS
	fmt.Println("1. Streaming Processor Basics")
	fmt.Println("------------------------------")
	demonstrateStreamingProcessor()

	// 2. STREAMING TRANSFORMATIONS
	fmt.Println("\n2. Streaming Transformations")
	fmt.Println("-----------------------------")
	demonstrateStreamingTransformations()

	// 3. CHUNKED PROCESSING
	fmt.Println("\n3. Chunked Processing")
	fmt.Println("----------------------")
	demonstrateChunkedProcessing()

	// 4. NDJSON PROCESSING
	fmt.Println("\n4. NDJSON Processing")
	fmt.Println("--------------------")
	demonstrateNDJSON()

	// 5. LAZY JSON
	fmt.Println("\n5. Lazy JSON (Deferred Parsing)")
	fmt.Println("--------------------------------")
	demonstrateLazyJSON()

	// 6. LARGE FILE PROCESSING
	fmt.Println("\n6. Large File Processing")
	fmt.Println("------------------------")
	demonstrateLargeFileProcessing()

	fmt.Println("\nStreaming & NDJSON processing examples complete!")
}

func demonstrateStreamingProcessor() {
	// Large JSON array - simulating a large data stream
	largeArray := `[
		{"id": 1, "name": "Alice", "active": true},
		{"id": 2, "name": "Bob", "active": false},
		{"id": 3, "name": "Charlie", "active": true},
		{"id": 4, "name": "Diana", "active": true},
		{"id": 5, "name": "Eve", "active": false}
	]`

	reader := strings.NewReader(largeArray)
	processor := json.NewStreamingProcessor(reader, 0)

	fmt.Println("   Streaming array elements:")

	err := processor.StreamArray(func(index int, item any) bool {
		// Process each element one at a time
		if obj, ok := item.(map[string]any); ok {
			name := obj["name"]
			active := obj["active"]
			fmt.Printf("   [%d] %v (active: %v)\n", index, name, active)
		}
		return true // Continue processing
	})

	if err != nil {
		fmt.Printf("   Error: %v\n", err)
	}

	// Get statistics
	stats := processor.GetStats()
	fmt.Printf("\n   Stats: %d items processed\n", stats.ItemsProcessed)
}

func demonstrateStreamingTransformations() {
	data := `[
		{"name": "Alice", "score": 85},
		{"name": "Bob", "score": 92},
		{"name": "Charlie", "score": 78},
		{"name": "Diana", "score": 95},
		{"name": "Eve", "score": 88}
	]`

	// Filter: Get only high scores (>= 90)
	reader := strings.NewReader(data)
	filtered, err := json.StreamArrayFilter(reader, func(item any) bool {
		if obj, ok := item.(map[string]any); ok {
			if score, ok := obj["score"].(float64); ok {
				return score >= 90
			}
		}
		return false
	})
	if err != nil {
		fmt.Printf("   Filter error: %v\n", err)
	} else {
		fmt.Printf("   Filtered (score >= 90): %d items\n", len(filtered))
		for _, item := range filtered {
			if obj, ok := item.(map[string]any); ok {
				fmt.Printf("   - %v: %v\n", obj["name"], obj["score"])
			}
		}
	}

	// Map: Transform names to uppercase
	reader = strings.NewReader(data)
	transformed, err := json.StreamArrayMap(reader, func(item any) any {
		if obj, ok := item.(map[string]any); ok {
			newObj := make(map[string]any)
			for k, v := range obj {
				newObj[k] = v
			}
			if name, ok := newObj["name"].(string); ok {
				newObj["name"] = strings.ToUpper(name)
			}
			return newObj
		}
		return item
	})
	if err != nil {
		fmt.Printf("   Map error: %v\n", err)
	} else {
		fmt.Println("\n   Transformed (uppercase names):")
		for _, item := range transformed {
			if obj, ok := item.(map[string]any); ok {
				fmt.Printf("   - %v\n", obj["name"])
			}
		}
	}

	// Reduce: Sum all scores
	reader = strings.NewReader(data)
	sum, err := json.StreamArrayReduce(reader, 0.0, func(acc any, item any) any {
		if obj, ok := item.(map[string]any); ok {
			if score, ok := obj["score"].(float64); ok {
				return acc.(float64) + score
			}
		}
		return acc
	})
	if err != nil {
		fmt.Printf("   Reduce error: %v\n", err)
	} else {
		fmt.Printf("\n   Total score (reduce): %.0f\n", sum)
	}

	// Count: Get array length without storing elements
	reader = strings.NewReader(data)
	count, err := json.StreamArrayCount(reader)
	if err != nil {
		fmt.Printf("   Count error: %v\n", err)
	} else {
		fmt.Printf("   Array count: %d\n", count)
	}

	// First: Find first matching element
	reader = strings.NewReader(data)
	first, found, err := json.StreamArrayFirst(reader, func(item any) bool {
		if obj, ok := item.(map[string]any); ok {
			return obj["name"] == "Charlie"
		}
		return false
	})
	if err != nil {
		fmt.Printf("   First error: %v\n", err)
	} else if found {
		fmt.Printf("   Found 'Charlie': %v\n", first)
	}

	// Take: Get first N elements
	reader = strings.NewReader(data)
	taken, err := json.StreamArrayTake(reader, 3)
	if err != nil {
		fmt.Printf("   Take error: %v\n", err)
	} else {
		fmt.Printf("   First 3 elements: %d items\n", len(taken))
	}
}

func demonstrateChunkedProcessing() {
	data := `[1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15]`

	reader := strings.NewReader(data)
	processor := json.NewStreamingProcessor(reader, 0)

	// Process in chunks of 5
	chunkNum := 0
	err := processor.StreamArrayChunked(5, func(chunk []any) error {
		chunkNum++
		fmt.Printf("   Chunk %d: %v\n", chunkNum, chunk)
		return nil
	})

	if err != nil {
		fmt.Printf("   Error: %v\n", err)
	}

	// Object streaming
	objectData := `{"a": 1, "b": 2, "c": 3, "d": 4, "e": 5}`
	reader = strings.NewReader(objectData)
	processor = json.NewStreamingProcessor(reader, 0)

	fmt.Println("\n   Streaming object:")
	err = processor.StreamObject(func(key string, value any) bool {
		fmt.Printf("   - %s: %v\n", key, value)
		return true
	})

	if err != nil {
		fmt.Printf("   Error: %v\n", err)
	}
}

func demonstrateNDJSON() {
	// Create a temporary NDJSON file
	tempDir, err := os.MkdirTemp("", "json-ndjson-*")
	if err != nil {
		fmt.Printf("   Error creating temp dir: %v\n", err)
		return
	}
	defer os.RemoveAll(tempDir)

	// NDJSON content (each line is a valid JSON object)
	ndjsonContent := `{"type": "user", "id": 1, "name": "Alice"}
{"type": "user", "id": 2, "name": "Bob"}
{"type": "event", "id": 3, "action": "login", "userId": 1}
{"type": "event", "id": 4, "action": "purchase", "userId": 2}
{"type": "user", "id": 5, "name": "Charlie"}
`

	ndjsonPath := filepath.Join(tempDir, "data.ndjson")
	err = os.WriteFile(ndjsonPath, []byte(ndjsonContent), 0644)
	if err != nil {
		fmt.Printf("   Error writing NDJSON: %v\n", err)
		return
	}

	// Process NDJSON file
	processor := json.NewNDJSONProcessor(64 * 1024)

	userCount := 0
	eventCount := 0

	err = processor.ProcessFile(ndjsonPath, func(lineNum int, obj map[string]any) error {
		switch obj["type"] {
		case "user":
			userCount++
			fmt.Printf("   [Line %d] User: %v\n", lineNum, obj["name"])
		case "event":
			eventCount++
			fmt.Printf("   [Line %d] Event: %v\n", lineNum, obj["action"])
		}
		return nil
	})

	if err != nil {
		fmt.Printf("   Error processing NDJSON: %v\n", err)
	}

	fmt.Printf("\n   Summary: %d users, %d events\n", userCount, eventCount)

	// Process from reader
	fmt.Println("\n   Processing from reader:")
	reader := strings.NewReader(ndjsonContent)
	err = processor.ProcessReader(reader, func(lineNum int, obj map[string]any) error {
		if lineNum <= 2 { // Just show first 2 lines
			fmt.Printf("   [%d] type=%v\n", lineNum, obj["type"])
		}
		return nil
	})

	if err != nil {
		fmt.Printf("   Error: %v\n", err)
	}
}

func demonstrateLazyJSON() {
	// LazyJSON defers parsing until a value is accessed
	rawJSON := []byte(`{"user": {"name": "Alice", "age": 30}, "active": true}`)

	lazy := json.NewLazyJSON(rawJSON)

	// Check if parsed (not yet)
	fmt.Printf("   Is parsed (before access): %v\n", lazy.IsParsed())

	// Access a value - this triggers parsing
	name, err := lazy.Get("user.name")
	if err != nil {
		fmt.Printf("   Error: %v\n", err)
	} else {
		fmt.Printf("   user.name: %v\n", name)
	}

	// Now it's parsed
	fmt.Printf("   Is parsed (after access): %v\n", lazy.IsParsed())

	// Subsequent accesses are fast (no re-parsing)
	age, _ := lazy.Get("user.age")
	active, _ := lazy.Get("active")
	fmt.Printf("   user.age: %v, active: %v\n", age, active)

	// Get raw bytes
	fmt.Printf("   Raw JSON length: %d bytes\n", len(lazy.Raw()))

	// Force parse and get all data
	parsed, err := lazy.Parse()
	if err != nil {
		fmt.Printf("   Parse error: %v\n", err)
	} else {
		fmt.Printf("   Parsed data: %v\n", parsed)
	}
}

func demonstrateLargeFileProcessing() {
	// Create a temporary large JSON file
	tempDir, err := os.MkdirTemp("", "json-large-*")
	if err != nil {
		fmt.Printf("   Error creating temp dir: %v\n", err)
		return
	}
	defer os.RemoveAll(tempDir)

	// Create a JSON array with many items
	var buffer bytes.Buffer
	buffer.WriteString("[")
	for i := 0; i < 100; i++ {
		if i > 0 {
			buffer.WriteString(",")
		}
		buffer.WriteString(fmt.Sprintf(`{"id":%d,"value":"item%d"}`, i, i))
	}
	buffer.WriteString("]")

	largeFilePath := filepath.Join(tempDir, "large.json")
	err = os.WriteFile(largeFilePath, buffer.Bytes(), 0644)
	if err != nil {
		fmt.Printf("   Error writing large file: %v\n", err)
		return
	}

	// Process with LargeFileProcessor
	config := json.DefaultLargeFileConfig()
	config.ChunkSize = 1024 // Small chunks for demo

	lfp := json.NewLargeFileProcessor(config)

	count := 0
	err = lfp.ProcessFile(largeFilePath, func(item any) error {
		count++
		return nil
	})

	if err != nil {
		fmt.Printf("   Error processing large file: %v\n", err)
	} else {
		fmt.Printf("   Processed %d items from large file\n", count)
	}

	// Process in chunks
	fmt.Println("\n   Chunked processing:")
	chunkCount := 0
	err = lfp.ProcessFileChunked(largeFilePath, 25, func(chunk []any) error {
		chunkCount++
		fmt.Printf("   Processed chunk %d with %d items\n", chunkCount, len(chunk))
		return nil
	})

	if err != nil {
		fmt.Printf("   Error: %v\n", err)
	}

	// ChunkedReader for more control
	fmt.Println("\n   Using ChunkedReader:")
	file, err := os.Open(largeFilePath)
	if err != nil {
		fmt.Printf("   Error opening file: %v\n", err)
		return
	}
	defer file.Close()

	reader := json.NewChunkedReader(file, 1024)
	crCount := 0
	err = reader.ReadArray(func(item any) bool {
		crCount++
		return crCount < 5 // Stop after 5 items for demo
	})

	if err != nil {
		fmt.Printf("   Error: %v\n", err)
	} else {
		fmt.Printf("   Read %d items with ChunkedReader\n", crCount)
	}
}
