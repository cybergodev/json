// Package json provides a high-performance, thread-safe JSON processing library
// with 100% encoding/json compatibility and advanced path operations.
//
// The package uses an internal package for implementation details:
//
//   - internal: Private implementation including path parsing, navigation, extraction,
//     caching, array utilities, security helpers, and encoding utilities
//
// Most users can simply import the root package:
//
//	import "github.com/cybergodev/json"
//
// # Basic Usage
//
// Simple operations (100% compatible with encoding/json):
//
//	data, err := json.Marshal(value)
//	err = json.Unmarshal(data, &target)
//
// Advanced path operations:
//
//	value, err := json.Get(`{"user":{"name":"John"}}`, "user.name")
//	result, err := json.Set(`{"user":{}}`, "user.age", 30)
//
// Type-safe operations:
//
//	name, err := json.GetString(jsonStr, "user.name")
//	age, err := json.GetInt(jsonStr, "user.age")
//
// Advanced processor for complex operations:
//
//	processor := json.New() // Use default config
//	defer processor.Close()
//	value, err := processor.Get(jsonStr, "complex.path[0].field")
//
// # Configuration
//
// Use NewWithConfig for custom configuration:
//
//	cfg := json.NewConfig()
//	cfg.EnableCache = true
//	processor := json.NewWithConfig(cfg)
//	defer processor.Close()
//
// # Key Features
//
//   - 100% encoding/json compatibility - drop-in replacement
//   - High-performance path operations with smart caching
//   - Thread-safe concurrent operations
//   - Type-safe generic operations with Go 1.24+ features
//   - Memory-efficient resource pooling
//   - Production-ready error handling and validation
//
// # Package Structure
//
// The package is organized with all public API in the root package:
//
//   - Core types: Processor, Config, ProcessorOptions, EncodeConfig
//   - Path types: PathSegment, PathInfo, PropertyAccessResult
//   - Error types: JsonsError, various error constructors
//   - Encoding types: Number, NumberPreservingDecoder, CustomEncoder
//
// Implementation details are in the internal/ package:
//
//   - Path parsing and navigation utilities
//   - Extraction and segment handling
//   - Cache and array utilities
//   - Security and encoding helpers
//
// # Core Types Organization
//
// Core types are organized in the following files:
//
//   - types.go: All type definitions (Config, ProcessorOptions, Stats, PathSegment, etc.)
//   - processor.go: Processor struct and all methods
//   - ops.go: Internal operation implementations
//   - path.go: Path parsing and navigation
//   - encoding.go: JSON encoding/decoding
//   - api.go: Package-level API functions
//   - file.go: File operations
//   - iterator.go: Iteration utilities
//   - recursive.go: Recursive processing
package json
