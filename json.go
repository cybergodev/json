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
	"sync"
	"sync/atomic"
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
