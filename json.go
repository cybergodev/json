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
)

var (
	defaultProcessor   *Processor
	defaultProcessorMu sync.RWMutex
)

func getDefaultProcessor() *Processor {
	defaultProcessorMu.RLock()
	if defaultProcessor != nil && !defaultProcessor.IsClosed() {
		p := defaultProcessor
		defaultProcessorMu.RUnlock()
		return p
	}
	defaultProcessorMu.RUnlock()

	defaultProcessorMu.Lock()
	defer defaultProcessorMu.Unlock()

	if defaultProcessor == nil || defaultProcessor.IsClosed() {
		defaultProcessor = New()
	}

	return defaultProcessor
}

// SetGlobalProcessor sets a custom global processor (thread-safe)
func SetGlobalProcessor(processor *Processor) {
	if processor == nil {
		return
	}

	defaultProcessorMu.Lock()
	defer defaultProcessorMu.Unlock()

	if defaultProcessor != nil {
		defaultProcessor.Close()
	}

	defaultProcessor = processor
}

// ShutdownGlobalProcessor shuts down the global processor
func ShutdownGlobalProcessor() {
	defaultProcessorMu.Lock()
	defer defaultProcessorMu.Unlock()

	if defaultProcessor != nil {
		defaultProcessor.Close()
		defaultProcessor = nil
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
