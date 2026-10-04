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
//	name := json.GetString(jsonStr, "user.name", "")
//	age := json.GetInt(jsonStr, "user.age", 0)
//
// Advanced processor for complex operations:
//
//	processor, err := json.New() // Use default config
//	if err != nil {
//	    // handle error
//	}
//	defer processor.Close()
//	value, err := processor.Get(jsonStr, "complex.path[0].field")
//
// # Configuration
//
// Use DefaultConfig and optional parameters for custom configuration:
//
//	cfg := json.DefaultConfig()
//	cfg.EnableCache = true
//	processor, err := json.New(cfg)
//	if err != nil {
//	    // handle error
//	}
//	defer processor.Close()
//
// # Package-level API and Processor Mirror
//
// Every operation exists in two equivalent forms: a package-level function and
// a Processor method. The package-level form is a thin mirror of the Processor
// form — json.Foo(args, cfg) behaves like p.Foo(args, cfg) on a processor with
// the matching configuration:
//
//	// Package-level (uses a cached processor keyed by cfg)
//	v, err := json.Get(data, "user.name", json.SecurityConfig())
//
//	// Processor-level (explicit processor you own)
//	p, err := json.New(json.SecurityConfig())
//	defer p.Close()
//	v, err = p.Get(data, "user.name")
//
// Both accept an optional trailing Config (cfg ...Config) so the encoding/json
// drop-in signatures (Marshal, Unmarshal, MarshalIndent, Valid) stay
// byte-compatible while still allowing configuration. When cfg is supplied, its
// security limits (MaxJSONSize, MaxNestingDepthSecurity, FullSecurityScan) and
// encoding options take effect on that call; when omitted, the processor's own
// configuration applies. The JSONL/stream family (StreamJSONL, MapJSONL,
// ReduceJSONL, ...) follows the same convention on both layers (D-005): the
// package-level form selects a config-cached processor, the Processor methods
// take the per-call Config directly.
//
// Canonical names (D-005): Encode is the canonical string-returning encoder —
// EncodeWithConfig is a deprecated alias retained for compatibility. Likewise
// NewSchema replaces NewSchemaWithConfig, the error-returning ForeachWithError
// / ForeachNestedWithError replace the void Foreach / ForeachNested (which drop
// errors), and the StreamJSONL family replaces NDJSONProcessor. Deprecated
// symbols are kept for the lifetime of v1.
//
// Four intentional exceptions to the trailing-cfg convention:
//
//   - Typed getters (GetTyped, GetString, GetInt, ...) take a variadic default
//     value instead of cfg (Go allows only one variadic parameter). Use SafeGet
//     or New(cfg).GetXxx for Config-controlled typed reads.
//   - Convenience variants (SetCreate, SetMultipleCreate, DeleteClean) are
//     shorthand that force a flag (CreatePaths, or CleanupNulls+CompactArrays).
//   - Valid returns a single bool (encoding/json drop-in); ValidWithConfig
//     returns (bool, error) for callers that need the failure reason. Both take
//     cfg; the name difference is historical.
//   - CompareJSON's package-level no-cfg path skips security validation and
//     compares via the standard-library encoder (a documented fast path);
//     the Processor method always validates. Pass a cfg — or call the method —
//     to validate untrusted input.
//
// # Key Features
//
//   - 100% encoding/json compatibility - drop-in replacement
//   - High-performance path operations with smart caching
//   - Thread-safe concurrent operations
//   - Type-safe generic operations with Go generics
//   - Memory-efficient resource pooling
//   - Production-ready error handling and validation
//
// # Package Structure
//
// The package is organized with all public API in the root package:
//
//   - Core types: Processor, Config
//   - Error types: JsonsError, various error constructors
//   - Encoding types: Number
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
//   - types.go: All type definitions (Config, Stats, Schema, Result[T], etc.)
//   - processor.go: Processor struct and constructor
//   - processor_lifecycle.go: Lifecycle (Close, IsClosed, hooks, logger)
//   - processor_get.go: Get operations and typed getters (GetString, GetInt, ...)
//   - processor_set.go: Set operations (Set, SetMultiple, SetCreate, ...)
//   - processor_delete.go: Delete operations (Delete, DeleteClean)
//   - processor_iterate.go: Foreach iteration operations
//   - processor_cache.go: Cache management and cache keys
//   - processor_stats.go: Batch processing, cache warmup, stats, health
//   - processor_streamjsonl.go: JSONL streaming operations
//   - operation.go: Internal operation type definitions
//   - operation_set.go: Set value-at-path implementation
//   - operation_delete.go: Delete value-at-path implementation
//   - operation_array.go: Array access, slice, and extraction operations
//   - recursive.go: Unified recursive processing, segment handlers
//   - path.go: Path parsing and navigation
//   - encoding.go: JSON encoding/decoding, streaming, schema validation
//   - encoding_format.go: JSON formatting (Prettify, Compact, Indent, HTMLEscape)
//   - encoding_schema.go: Schema validation logic
//   - api.go: Package-level API functions and delegation helpers
//   - file.go: File operations and NDJSON processing
//   - iterator.go: Iteration utilities
//   - iterator_stream.go: Streaming iterators (StreamIterator, StreamObjectIterator)
//   - iterator_parallel.go: Parallel iterator
//   - iterable_value.go: IterableValue type for safe iteration access
//   - security.go: Security validation and dangerous pattern detection
//   - helpers.go: Type conversion, deep copy, merge, compare utilities
//   - errors.go: Error types, sentinels, and classification
//   - interfaces.go: Extension interfaces (hooks, custom encoders, validators)
//   - json.go: Global processor, JSONL support, and StreamLinesInto
package json
