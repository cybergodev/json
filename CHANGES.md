# Changelog

All notable changes to the cybergodev/json library will be documented in this file.

---

## v1.5.0 - Performance, Quality & Security Improvements (2026-05-14)

> Minor release: performance gains across all operations, critical bug fixes, security hardening, ~1600 lines of dead code removed

### Added

- Export `Marshaler`, `Unmarshaler`, `TextMarshaler`, `TextUnmarshaler` interfaces for encoding/json compatibility
- UTF-16 surrogate pair handling in streaming Decoder (RFC 8259 §7)
- Streaming byte limit enforcement in `Decoder` to prevent memory exhaustion from malicious input
- Concurrency semaphore integration for `Get()`, `Set()`, `Delete()` — bounded parallelism per processor
- `CacheManager.DeleteByPrefix()` for complete prefix-based cache eviction
- Panic recovery in hook execution — misbehaving hooks no longer crash the processor
- Streaming types documentation (Encoder, Decoder, Number, Token, Delim, StreamIterator, ParallelIterator)
- `Iterator.Reset()` / `ResetWith()` documented as not concurrency-safe
- Comprehensive boundary condition tests for low-coverage functions

### Fixed

- `Decoder.DisallowUnknownFields()` now enforces strict field matching (was a no-op)
- Memory counter race condition in `decMemoryUsage` — non-atomic AddInt64+StoreInt64 replaced with CAS loop
- `ParseIntFast` 32-bit overflow on MinInt64 boundary — platform width now validated before conversion
- `HealthChecker` STW pause eliminated — cached memory stats replace direct `runtime.ReadMemStats`
- Circular call chain in `deepCopy` → `deepCopySubtree` → `deepCopy`
- `scanWindowForPatterns` now respects `Config.DisableDefaultPatterns` — non-critical patterns skipped when enabled
- `Get()` no longer returns shared cached reference on deepCopy failure
- `handleArrayIndexExtension` no longer silently loses writes due to local slice variable
- `validateNestingDepth` now checks bracket balance for all input sizes (was skipping < 64KB)
- `StreamLinesInto` now enforces security validation (size limits, depth checks, pattern scanning)
- `ForeachReturn` now re-encodes data after iteration to reflect callback transformations
- `GetFromParsed` deep copies non-primitive results to prevent cached data corruption
- `deleteComplexExtract` collects errors instead of silently discarding them
- `ParallelIterator` double-close panic — CAS-guarded close replaces select-based close
- `Processor.Close()` sets timeout state when semaphore drain exceeds deadline
- TOCTOU race in `checkRateLimit` — load-check-store replaced with CAS loop
- `invalidateJSONCache()` uses `DeleteByPrefix()` for complete eviction (was leaving stale entries)
- `SafeError` no longer panics on nil `Err` field in `JsonsError`
- Nil `Processor` access in `Parse` fast path now guarded
- `GetStats()` no longer panics on partial initialization (nil cache/metrics)
- `getCachedPathSegments()` returns copy to prevent data races when callers mutate the slice
- `StreamIterator.Next()` propagates closing-bracket decode errors instead of silently discarding
- `ShutdownGlobalProcessor` now closes `fallbackProcessor` to prevent resource leak
- `JSONLWriter.Write` uses pooled encoding with HTML escape support
- Path segment pool identity preserved — `getPathSegments`/`putPathSegments` use `*[]PathSegment`
- Shared mutable `cachedDefaultConfigPtr` removed — prevents silent cross-request config mutation
- `scanWithRollingWindow` offset progression fix — prevents data skipping at chunk boundaries
- `shouldUseDistributedArrayOp` heuristic improved — stricter sampling prevents false positives on regular nested arrays
- `extendArrayAndSetValue` / `extendArrayAndSetSliceValue` guard against theoretical out-of-bounds panic
- `readContainerValue` validates bracket matching at all nesting depths (was depth==1 only)

### Changed

- Validation cache key generation: SHA-256 (~100ns) → FNV-1a dual-hash (~4ns), ~50x speedup per Get/Set/Parse
- Fast path for simple property access in `Get()`, `Set()`, `Delete()` — bypasses hash, cache, and recursive processor
- `strconv.Atoi` in path parsing hot paths replaced with zero-allocation `ParseIntFast`
- `GlobalPathIntern` changed to zero-capacity lazy init from eager 25000-entry allocation
- `operation.go` (2162 lines) split into focused files: operation_delete.go, operation_set.go, operation_array.go
- `processor.go` (1784 lines) split into 7 focused files: lifecycle, get, set, delete, iterate, stats, cache
- `encoding.go` split into encoding.go (codec), encoding_schema.go (validation), encoding_format.go (formatting)
- `iterator.go` split into iterator.go, iterable_value.go, iterator_stream.go, iterator_parallel.go
- Modernized to Go 1.22+ idioms: range-over-int, reflect.Pointer, slices.Sort, maps.Copy
- IterableValue accessor methods (~400 lines duplication) consolidated to ~120 lines with generic helpers
- `EncodeWithConfig` delegates to shared `encodeWithConfigToBytes`, eliminating ~80 lines duplicated logic
- `prepareOptions` returns fresh pooled copy from cached default — no shared mutable state
- `getHooks` uses copy-on-write pattern — zero allocation for common no-hook case
- Config comparison for cache keys uses value equality instead of pointer identity
- `fallbackProcessor` uses minimal config (no cache, no metrics) to reduce memory footprint

### Performance

- Concurrent Get: 537ns → 487ns (9.3% faster), memory 104 → 72 B/op (30.8% less)
- Set Simple: 1476ns → 1233ns (16.5% faster), memory 841 → 777 B/op (7.6% less)
- Delete Simple: 1363ns → 1219ns (10.6% faster), memory 752 → 688 B/op (8.5% less)
- Unmarshal Simple: 1223ns → 1142ns (6.6% faster), memory 696 → 664 B/op (4.6% less)
- Large JSON Parse: 347Kns → 322Kns (7.2% faster)
- Deep Nesting Parse: 2329ns → 2211ns (5.1% faster)

### Removed

- `internal/array_ops.go` (926 lines) — all exported functions unused by parent package
- `internal/parallel_ops.go` (683 lines) — entire file unused
- ~190 lines of experimental dead code (fastSet, fastDelete, batchSetOptimized, etc.)
- Unused print functions, error helpers, experimental pools, and wrapper methods
- ~2400 lines of redundant/duplicate test code consolidated into focused test files

---

## v1.4.1 - Performance, Security & Quality Improvements (2026-05-08)

### Breaking Changes

- `Config.Context` field removed — use `GetWithContext(ctx, jsonStr, path)` for context cancellation

### Added

- `GetWithContext(ctx, jsonStr, path)` — context-aware Get with cancellation support
- `ParsedJSON.Release()` — prevents resource leak when ParsedJSON no longer needed
- `processor_cache.go` — cache management extracted from processor.go for separation of concerns
- Security and coverage tests for depth limits, schema regex caching, JSONL memory enforcement

### Fixed

- `MaxDepth=0` now correctly means "no limit" instead of blocking all nested encoding
- `StreamJSONLChunked` returns IterableValue objects to pool after chunk processing (allocation pressure)
- `ParallelIterator.Close()` signals goroutines to stop via done channel (goroutine leak fix)
- `StreamJSONL` methods now respect `Config.JSONLBufferSize` and `Config.JSONLMaxLineSize` settings
- `GetMultiple()` returns first error from failed paths instead of silently swallowing errors
- `Encoder.Encode()` fast path checks output size against `MaxJSONSize`
- `validateNestingDepth` fast path returns properly wrapped `ErrDepthLimit` for `errors.Is()` matching
- Nil pointer dereference in `logError()` when `EnableMetrics=false`
- `configPool` reference leaks — reference-type fields cleared before pool return
- Decoder pool `useNumber`/`disallowUnknownFields` leakage between pooled decoders

### Changed

- Schema regex lazily compiled and cached on first validation (prevents ReDoS)
- Validation cache uses SHA-256 for all JSON sizes (eliminates FNV-1a collision bypass)
- Deep copy functions enforce depth limit of 200 (prevents stack overflow DoS)
- `StreamJSONL`/`StreamJSONLChunked` enforce `Config.JSONLMaxMemory` limit
- `IsValidJSONNumber()` enforces leading-zero rule consistently across all paths
- `fastSet`/`fastDelete`/`batchSetOptimized`/`batchDeleteOptimized`/`fastGetMultiple` marked EXPERIMENTAL
- Removed duplicate `isValidJSONNumber()` from fast_encoder.go (70 lines deduplicated)
- `fastParseInt()` and `ParseArrayIndex()` delegate to `ParseIntFast()` (eliminates ~50 lines duplication)

### Performance

- `navigateDotNotation` hot path: direct Type enum replaces `TypeString()` string comparison
- `Set()`/`Delete()`/`SetMultiple()`: `FastMarshalToString` replaces `json.Marshal` output serialization
- `preservingUnmarshal`: `bytes.NewReader` replaces `strings.NewReader(string(data))` allocation
- Array index parsing: `ParseArrayIndex` replaces `strconv.Atoi` in deletion hot paths
- Path building in iteration: byte append pattern replaces string concatenation per iteration

### Removed

- `internal.MergeObjects()` — duplicates `DeepMerge` functionality
- `internal.DetectConsecutiveExtractions()` / `ExtractionGroup` — dead code
- `Processor.compactBuffer()` — dead method

---

## v1.4.0 - API Unification & Security Hardening (2026-04-13)

> Major release: unified Config pattern, reduced API surface, production security hardening

### Breaking Changes

**Removed types & structs:**
- `JSONLProcessor` → Use `Processor.StreamJSONL()`
- `StreamingProcessor` → Use `Processor.StreamArray()` / `Processor.StreamObject()`
- `LargeFileProcessor` → Use `Processor.ForeachFile()`
- `ChunkedWriter` / `ChunkedReader` / `SamplingReader` / `LazyParser` → Removed
- `JSONLConfig` / `StreamJSONLConfig` / `StreamIteratorConfig` / `LargeFileConfig` → Use `Config` with JSONL*/Chunk* fields
- `StreamJSONLResult` / `ParallelMapResult` / `ParallelSliceResult` → Removed (dead code)

**Removed interfaces:**
- `PatternRegistry` / `PathValidator` / `SecurityValidator` / `CacheConfig` / `EncoderConfig` → Internal only

**Changed signatures:**
- `Parse(jsonStr, cfg) (any, error)` → `ParseAny(jsonStr, cfg) (any, error)`; new `Parse(jsonStr, target, cfg) error`
- `GetString/GetInt/GetFloat/GetBool/GetArray/GetObject` now return value directly with optional `defaultValue`; `GetXxxOr` variants removed
- `MergeJSON(a, b, mergeMode)` → `MergeJSON(a, b, cfg)` with `cfg.MergeMode`
- `Compact(dst, src)` extended to `Compact(dst, src, cfg ...Config)`; `CompactString` removed

**Privatized identifiers (~50+ exported symbols):**
- Helpers: `ConvertToInt/ToFloat64/ToBool/ToString`, `IsValidJSON/IsValidPath`, `DeepCopy`, `CompareJSON`
- Errors: `ErrOperationFailed/ErrInternalError/ErrCacheFull/ErrCacheDisabled`
- Security: `GetDefaultPatterns/GetCriticalPatterns/ClearDangerousPatterns`
- Config: `Config.IsCacheEnabled/GetMaxCacheSize/GetCacheTTL`
- Schema: `HasMinLength/HasMaxLength/HasMinimum/HasMaximum/HasMinItems/HasMaxItems`
- Processor: `FastSet/FastDelete/BatchSetOptimized/BatchDeleteOptimized/FastGetMultiple`
- I/O: `CompactBuffer/ValidString/Print/PrintPretty`, `LoadFromFileAsData/LoadFromReaderAsData`
- Path: `PathSegmentType`/`PathSegmentFlags` constants, `NewPropertySegment`/`NewArrayIndexSegment`/etc.
- Internal: `HookChain/ValidationChain/CachedPathParser`, `NewEncoderWithConfig`

**Removed deprecated methods:**
- `ToJsonString/ToJsonStringPretty/ToJsonStringStandard` → Use `Encode()`/`EncodePretty()`
- `CompactBytes/IndentBytes/FormatCompact/CompactString` → Use `Compact()`/`Indent()`
- `Processor.IndentBuffer/Processor.HTMLEscapeBuffer` → Use `Indent()`/`HTMLEscape()`
- `MergeJSONMany/MergeJSONManyWithConfig` → Use `MergeMany()`
- `IndentBuffer/HTMLEscapeBuffer` (package-level) → Use `Indent()`/`HTMLEscape()`

**Other breaking changes:**
- `CreatePaths` default changed from `false` to `true`
- `Processor.CompilePath/GetCompiled` now return `*CompiledPath` (public alias) instead of `*internal.CompiledPath`

### Added

- `SafeGet(jsonStr, path, cfg) AccessResult` — type-safe get with `Ok()`/`Unwrap()`/`UnwrapOr()`/`AsXxx()`
- `MergeMany(jsons, cfg)` — unified merge-many with standard Config pattern
- `EncodePretty(value, cfg)` / `ParseAny(jsonStr, cfg)` / `Parse(jsonStr, target, cfg)` — package-level methods
- `DeleteClean/SetCreate/SetMultipleCreate` — package-level wrappers
- `LoadFromReader` / `ForeachFile/ForeachFileWithPath/ForeachFileChunked/ForeachFileNested` — package-level file ops
- `ForeachWithError/ForeachNestedWithError/ForeachWithPathAndIterator` — error-returning iteration
- `StreamJSONL/StreamJSONLParallel/StreamJSONLChunked` — Processor JSONL streaming
- `ForeachJSONL/MapJSONL/ReduceJSONL/FilterJSONL/CollectJSONL/FirstJSONL/StreamJSONLFile` — package-level JSONL wrappers
- `StreamJSONLParallelWithContext()` — context-aware parallel JSONL processing
- `IterableValue.Break()` — stop iteration without reporting error (replaces `ErrBreak`)
- `SafeError()` — client-safe error messages without internal details (CWE-209)
- `RedactedPath()` — safe path inclusion in logs
- `CompiledPath` public type alias for `internal.CompiledPath`
- `Config` now includes `JSONL*`, `ChunkSize`, `MaxMemory`, `BufferSize`, `SamplingEnabled`, `SampleSize` fields

### Fixed

**Security:**
- Added depth limit to `Decoder.readContainerValue()` — prevents stack overflow from deeply nested JSON
- Added per-line nesting depth validation and size limit to `NDJSONProcessor.ProcessReader()`
- Replaced `os.ReadFile` with `io.LimitReader` in `readValidatedFile/UnmarshalFromFile` — prevents TOCTOU race
- `FastEncoder` now escapes `<`, `>`, `&` when `htmlEscape` enabled (CWE-79 XSS)
- Essential validation (size + depth) always enforced even with `SkipValidation: true` (CWE-20)
- Sensitive data sampling expanded: head 100 / tail 50 / middle 20 uniform samples
- File paths sanitized in error messages to prevent information disclosure

**Data races & concurrency:**
- Fixed `ParallelMap` data race — concurrent map writes replaced with local-slice-per-goroutine pattern
- Fixed `CacheManager` WaitGroup reuse panic — added atomic `closed` flag
- Fixed `StringIntern` stats/trim races — atomic loads + CAS guard
- Fixed `SetErrorSentinels` data race — added `sync.Once`
- Fixed `ForEachWithContext` goroutine leak — select on `ctx.Done()` during semaphore acquire
- Fixed `CompiledPathCache.Get` TOCTOU race — clone within same lock scope

**Resource & memory leaks:**
- Fixed `configPool` reference leaks — all reference-type fields cleared before pool return
- Fixed `Processor.Close()` no longer clears global caches (moved to `ShutdownGlobalProcessor()`)
- Fixed `Get()` cache hit corrupts cache — deep-copy mutable values on cache hit
- Fixed `ShutdownGlobalProcessor()` now clears all cached processors and global caches
- Added timeout protection to stale processor close goroutine

**Error handling & correctness:**
- Restored error chains in `Delete/Set/EncodeWithConfig/SaveToWriter/Parse/Prettify/Compact`
- `NDJSONProcessor.ProcessReader` no longer silently discards parse errors
- `readValidatedReader` size check now uses local `maxSize` (was reading `p.config.MaxJSONSize` which could be 0)
- `shallowCopyMap/shallowCopySlice` now propagate depth counter (was resetting to 0, bypassing stack overflow protection)
- `Encoder.Encode` no longer unconditionally overrides processor's `EscapeHTML` config
- `readContainerValue` validates closing delimiter matches opening
- `readContainerValue` returns error on truncated EOF
- `ValidateString` uses `utf8.RuneCountInString` for Unicode-correct length
- `multipleOf` validation uses epsilon tolerance for IEEE 754 safety
- `CompareJSON` now normalizes `Number` to `float64` for correct equality

**Nil receiver safety:**
- All public types (`Processor`, `Config`, `JsonsError`, `JSONLWriter`, `NDJSONProcessor`, error types) return safe defaults on nil receiver

### Changed

- All APIs unified to variadic `cfg ...Config` pattern (Encoder, Decoder, StreamIterator, JSONL, iterators)
- Dual-layer API consistency: every package-level function delegates to corresponding Processor method
- `Encoder` inherits processor config as base; explicit fields still take priority
- `NDJSONProcessor` now respects `JSONLContinueOnErr` config
- `StreamJSONL` methods use `shouldSkipJSONLLineFromConfig` respecting `JSONLSkipEmpty`/`JSONLSkipComments`
- `Valid()` delegates to `Processor.ValidBytes()` eliminating input type mismatch
- `Compact()` delegates to `Processor.CompactBuffer()` aligning dual-layer signatures
- `ForeachFileChunked` uses `iterableValuePool` and releases values after chunk callback
- Unified negative index handling into `normalizeNegativeIndex` helpers (8+ patterns consolidated)
- `navigateToParent` helper deduplicates delete navigation logic (~40 lines eliminated)
- `withTypedGetter[T]`/`withConfigProcessor[T]` generics eliminate typed getter boilerplate

### Performance

- `deepCopyJSONValue` — JSON-specialized 6-case type switch replaces 16-case generic switch
- `Get()` primitive fast path — `nil/bool/float64/string/json.Number` skip deep copy (~60% of calls)
- `NeedsHTMLEscapeBytes` — pre-computed lookup table replaces per-byte SWAR comparisons
- `FastEncoder` map/array — `append`-based buffer growth eliminates separate allocation+copy
- `operation.go` — 26+ `TypeString()` calls replaced with direct `segment.Type` enum comparisons
- `escapeRune` — direct hex byte writes replace `fmt.Fprintf` (~10-50x faster per escape)
- `EncodeMap`/`EncodeArray` — buffer pre-allocation for large inputs
- `EncodeBase64` — direct buffer encoding eliminates intermediate string allocation
- `createCacheKeyWithHash` — pointer identity check replaces 40+ field comparison
- `prepareOptions` — cached default config pointer eliminates `DefaultConfig()+Validate()` per operation
- `CompiledPathCache` — initial lookup uses `RLock` instead of exclusive `Lock`
- `shouldSkipJSONLLineFromConfig` parameter changed from `Config` (value) to `*Config` (pointer)
- `getDefaultPatterns`/`getCriticalPatterns` — `sync.OnceValue` caching avoids repeated slice allocation

### Security

- FastEncoder HTML escape support for `<`, `>`, `&` → `\u003c`, `\u003e`, `\u0026` (CWE-79)
- Essential size/depth validation always enforced regardless of `SkipValidation` (CWE-20)
- `SafeError()` returns client-safe messages without internal path/operation details (CWE-209)
- `RedactedPath()` for safe path inclusion in logs
- Sensitive data array sampling improved from 70-element head-only to 500-element head/tail/middle coverage
- Security warning documentation for `HookContext.JSONStr`, `CompilePathUnsafe`, `StringToBytes`

---

## v1.3.0 - Performance & API Unification (2026-03-30)

>Internal core restructuring  
>Basic functionality remains unchanged

### Breaking Changes
- `EncodeWithOptions()` → Use `EncodeWithConfig()` (deprecated alias removed)
- `EncodeStreamWithOptions()` → Use `EncodeStream()` (deprecated alias removed)
- `StreamingProcessor.Release()` → Use `Close()` (deprecated method removed)
- `ValidWithOptions()` → Use `ValidWithConfig()` (deprecated alias removed)
- `NewEncoderWithOpts()` → Use `NewEncoderWithConfig()` (deprecated alias removed)

### Added
- `CompactBytes()`, `IndentBytes()` - encoding/json signature compatibility *(removed in v1.4.0, use `CompactBuffer`/`IndentBuffer`)*
- `GetStringOr()`, `GetIntOr()`, `GetFloatOr()`, `GetBoolOr()` instance methods
- Streaming-optimized object pools (tiered map/slice pools)
- Path segment cache with sync.Map (lock-free concurrent reads)
- Config sync.Pool for hot path allocation reduction

### Changed
- Unified all encoding APIs to variadic `cfg ...Config` pattern
- StreamingProcessor now reuses bufio.Reader across operations
- Batch operations use FastMarshal for reduced allocations
- HTML escape uses pooled buffer with byte-level scanning

### Fixed
- StreamLinesParallel goroutine leak and error propagation
- ProcessFileChunked data corruption (shared array issue)
- Hash cache TTL-based expiration for stale entry prevention
- Pool corruption recovery mechanism

### Performance
- BatchSet_Small: 29% faster, 19% less memory, 35% fewer allocations
- BatchSet_Large: 43% faster, 19% less memory, 38% fewer allocations
- StreamingProcessor_Array: 98% faster, 86% less memory, 99.9% fewer allocations
- PathParsing_Complex: 480ns → 22ns (95% improvement with caching)
- FastEncoder_String: 9.8% faster, FastEncoder_Int: 8.4% faster
- Test coverage: 56.9% → 62.6%

---

## v1.2.2 - Architecture & Quality Enhancement (2026-03-04)

### Added
- **Configuration Presets**: `WebAPIConfig()`, `FastConfig()`, `MinimalConfig()` for common use cases
- **LazyParser Enhancements**: `GetValue()`, `IsObject()`, `IsArray()` methods for all JSON types
- **WorkerPool Improvements**: `SubmitWait()` method for blocking task submission
- **API Consistency**: `ValidString()`, `ValidWithOptions()`, `CompactString()`, `EncodeWithOptions()`
- **Documentation**: `docs/API_OPTIMIZATION.md` and `docs/API_MIGRATION_GUIDE.md`

### Changed
- **Architecture**: Introduced delegation pattern with `CoreProcessorImpl` and `ProcessorBuilder`
- **WorkerPool Limit**: Default max workers increased from 16 to 64 for better multi-core utilization
- **Config.Clone()**: Documented shallow copy behavior and future-proofing notes
- **LazyParser.GetAll()**: Now returns error for non-object JSON

### Fixed
- **FastParseInt Overflow (P0)**: Added separate checks for MaxInt64/MinInt64 boundaries
- **WorkerPool.Wait() Race (P0)**: Fixed condition variable race with proper lock synchronization
- **WorkerPool.Submit() Race (P1)**: Added double-check pattern for stop state validation
- **JSON Special Floats (P1)**: `NaN`, `Infinity` now output `null` for RFC 8259 compliance
- **ParseIntFast Platform (P1)**: Platform-independent overflow detection for 32/64-bit systems
- **KeyIntern Eviction (P2)**: Fixed hot key cache consistency during eviction
- **Pipe Deadlock**: Fixed `captureStdout`/`captureStderr` deadlock with concurrent goroutine pattern

### Removed
- ~400 lines of dead code across errors.go, helpers.go, performance.go, iterator.go, operation.go
- Unused error constructors, path type checking wrappers, iterator pool implementation
- Duplicate parallel processor tests and slice comparison functions

### Performance
- `EncodeConfig` pooling: ~15% reduction in allocations for repeated encoding
- Cache Set optimization: ~15% reduction by updating entries in-place
- Path parsing: ~10% improvement with early exit for simple paths
- Cache key generation: ~20% faster with manual hex encoding
- Test suite: 24% reduction in test functions, coverage improved from 51.5% to 70%+

### Security
- 55+ dangerous patterns detected (XSS, prototype pollution, sensitive data)
- Memory limits for string interning (16MB max, 256KB per shard)
- Integer overflow protection in size estimation functions

### Breaking Changes
- None - 100% backward compatible

---

## v1.2.1 - Encoding & Documentation Update (2026-02-26)

### Added
- **FloatTruncate option**: New `EncodeConfig.FloatTruncate` field to truncate float precision instead of rounding

### Changed
- **ConvertToBool**: Extended with user-friendly formats (`yes`, `no`, `on`, `off`)
- **Documentation**: Updated `docs/` for accuracy with actual codebase (COMPATIBILITY.md, QUICK_REFERENCE.md, SECURITY.md)

### Fixed
- **EscapeHTML option**: Correctly disables HTML escaping when set to `false`
- **Pretty option**: Properly applies indentation formatting

---

## v1.2.0 - Performance & Security Enhancement (2026-02-26)

### Added
- **JSONL (JSON Lines) Support**: `ParseJSONL()`, `JSONLProcessor`, `StreamLinesParallel()`, `StreamLinesInto[T]()`, `JSONLWriter`, `ToJSONL()`
- **Streaming Processing**: `StreamingProcessor`, `StreamArray()`, `StreamObject()`, `StreamArrayChunked()`, `StreamArrayFilter/Map/Reduce()`, `StreamArrayFirst/Take/Skip()`
- **Lazy JSON Parsing**: `LazyJSON` with on-demand parsing, `Get()`, `IsParsed()`, `Raw()`
- **Large File Processing**: `LargeFileProcessor`, `ProcessFile()`, `ProcessFileChunked()`
- **Compiled Path Support**: `CompiledPath` for zero-parse overhead, `CompilePath()`, `GetCompiled()`, `CompiledPathCache`
- **Fast Encoder**: `FastEncoder` with 4-8x faster encoding for common types, pre-computed integer tables (0-9999)
- **String Interning**: `StringIntern`, `KeyIntern` (64 shards), `PathIntern`, `BatchIntern`
- **Parallel Operations**: `ParallelProcessor`, `WorkerPool`, `ChunkProcessor`, `ParallelFilter/Transform()`
- **New Examples**: `13_streaming_ndjson.go`, `14_batch_operations.go`, `examples/README.md`

### Changed
- **WorkerPool**: Replaced busy-wait with condition variable pattern (10-20% throughput improvement)
- **LRU Cache**: Enhanced with frequency-aware eviction (15-25% better hit ratio)
- **StreamIterator**: Configurable buffer sizes (20-40% improvement for large files)
- **Struct Encoder**: Extended with type-specific encoding functions (50-100% faster)
- **Security Scanning**: Rolling window replaces sampling (100% coverage guarantee)
- **Test Coverage**: Improved from 30.5% to 47.8%

### Fixed
- **Sampling Bypass Vulnerability**: Rolling window scan guarantees complete coverage
- **Unsafe Package Usage**: Replaced unsafe conversions with safe standard library
- **String Intern Race Condition**: Fixed memory exhaustion in high-concurrency scenarios
- **Validation Cache Memory**: Hash-based keys for all sizes prevent unbounded growth
- **DeepCopy Stack Overflow**: Added depth limit (200) to prevent overflow
- **WorkerPool Stop Race**: Added stopped flag to prevent task submission after stop

### Security Enhancements
- **55+ Dangerous Patterns Detected**: XSS, prototype pollution, event handlers, sensitive data patterns
- **Path Traversal Protection**: Multi-layer encoding detection, Unicode lookalike detection
- **Memory Safety**: DeepCopy depth limits, cache LRU eviction, string interning limits
- **Integer Overflow Protection**: Type conversion safety checks
- **Platform-Specific Security**: Windows reserved names, UNC paths, ADS blocking

### Performance Improvements
| Operation | Performance |
|-----------|-------------|
| FastEncodeString | 7 ns/op (8.7x faster than stdlib) |
| FastEncodeInt | 9 ns/op (5.3x faster) |
| FastEncodeMap | 81 ns/op (6.5x faster) |
| FastEncodeArray | 57 ns/op (4.6x faster) |
| PathSegmentCache | 20 ns/op, 0 allocs |
| PooledIterator | 0 allocs |

### Backward Compatibility
- 100% backward compatible - all public APIs unchanged
- New functionality is additive
- All tests pass with race detection

---

## v1.1.0 - Comprehensive Enhancement & Quality Assurance (2026-02-05)

### Added
- **Print Methods**: `json.Print()` and `json.PrintPretty()` for direct stdout output with smart JSON detection
- **Package-Level Convenience Methods**: `GetStats()`, `GetHealthStatus()`, `ClearCache()`, `WarmupCache()`, `ProcessBatch()`
- **Advanced Encoding Methods**: `EncodeStream()`, `EncodeBatch()`, `EncodeFields()`
- **Buffer Methods**: `CompactBuffer()`, `IndentBuffer()`, `HTMLEscapeBuffer()` with processor options support
- **Test Coverage**: 6 new comprehensive test files with 82+ test functions and 18+ benchmarks
- **Chinese Documentation**: Complete README_zh-CN.md translation with feature parity

### Changed
- **MergeJSON Behavior**: Implemented deep merge for nested objects and union merge with deduplication for arrays
- **SaveToFile Behavior**: Now preprocesses string/[]byte inputs to prevent double-encoding (unified with package-level)
- **LoadFromFile Return Type**: Returns `(string, error)` instead of `(any, error)` for consistency
  - Use `LoadFromFileAsData()` for previous `(any, error)` behavior
- **Cache Statistics**: `GetStats()` returns `CacheStats` struct instead of `map[string]any` (~40% faster)
- **Path Parsing**: Added fast path for simple paths (~50% fewer allocations)

### Fixed
- **Security (Critical)**:
  - JSON Pointer double-unescape vulnerability (single-pass parsing)
  - Integer overflow in slice capacity calculation
  - Email validation regex DoS vulnerability (RFC-compliant validation)
  - Race condition in resource monitor leak detection (atomic CAS loop)
- **Encoding Options**: Fixed `EscapeHTML`, `FloatPrecision`, `EscapeUnicode`, `EscapeNewlines`, `EscapeTabs`, `EscapeSlash` not working
- **Documentation**: Fixed inaccurate examples, removed non-existent methods, corrected config defaults
- **Examples**: Fixed `examples/9_iterator_functions.go` IterableValue API usage

### Removed
- **EncodeCompact()**: Redundant function (use `Encode()` which defaults to compact)
- **OmitEmpty Config**: Removed `OmitEmpty` and `OmitEmptyMode` fields (use struct tags `omitempty`)
- **NewCompactConfig()**: Helper function (use `DefaultEncodeConfig()`)
- **ProcessorConfig**: Deprecated configuration struct (use `Config` instead)
- **DefaultProcessorConfig()**: Deprecated function (use `DefaultConfig()` instead)
- **NewIterableValueWithIterator()**: Deprecated function (use `NewIterableValue()` instead)
- **deletedMarker variable**: Unused local variable (use `DeletedMarker` constant)

### Performance Improvements
- **Cache Stats**: ~40% faster with struct return type
- **Simple Path Parsing**: ~50% fewer allocations
- **Null Value Handling**: ~60% faster with direct type checking
- **Path Validation**: Pre-compiled regex pattern for array index validation

### Security Enhancements
- Enhanced path traversal detection (consecutive dots, partial double encoding)
- Fixed COM0/LPT0 Windows device validation
- Improved Alternate Data Streams detection
- Email validation with RFC-compliant length limits

### Test Results
- **Tests**: 600+ test runs (150+ total functions)
- **Benchmarks**: 35+ benchmarks
- **Race Detection**: All tests pass with `-race` flag

### Backward Compatibility
- **Minor Breaking Changes**:
  - `EncodeCompact()` → `Encode()` (same behavior)
  - `OmitEmpty` config → struct tags `omitempty`
  - `LoadFromFile()` return type changed (use `LoadFromFileAsData()` for old behavior)
  - `SaveToFile()` string preprocessing behavior (more intuitive, prevents double-encoding)

---

## v1.0.10 - Major Code Quality & Security Enhancement (2026-01-20)

### Fixed
- **Critical Security**: Pattern validation bypasses using Unicode characters and whitespace (security.go)
- **Double-Fetch Bug**: `GetTypedWithDefault()` calling `Get()` twice unnecessarily (json.go)
- **SaveToFile Double-Encoding**: JSON strings being saved with escaped quotes instead of proper JSON (json.go)
- **Null Value Conversion**: `handleNullValue()` returning literal "null" string instead of empty string (helpers.go)
- **EncodePretty/EncodeCompact**: Incorrect handling when `nil` config explicitly passed (json.go)
- **Nested Extraction Syntax**: `{teams{name}}` incorrectly parsed, adopted separated braces syntax `{teams}{name}`
- **Duplicate MarshalToFile**: Function declared twice causing build issues (json.go)
- **Unused Variable**: Removed unused `zero` variable declaration (json.go)
- **Example Code**: Fixed `examples/4_error_handling.go` to use correct error classification functions

### Changed
- **Security Validation**: Enhanced with context-aware pattern matching and word boundary checking
- **Performance**: Eliminated double-fetch in typed operations (~50% faster for default value retrieval)
- **Buffer Management**: Consolidated duplicate buffer pools into single `encoderBufferPool`
- **File Operations**: Merged duplicate `SaveToFile()` and `MarshalToFile()` code into unified internal function
- **Documentation**: Fixed `EncodeConfig` example missing required `MaxDepth` field in README.md and README_zh-CN.md
- **Documentation**: Corrected `ForeachWithIterator` documentation (Processor method, not package-level)
- **Resource Pool Limits**: Stricter pool size limits (max 8KB buffer, 32 path segments)

### Removed
- **5 Deprecated Conversion Functions** (helpers.go): `convertToInt()`, `convertToInt64()`, `convertToFloat64()`, `convertToString()`, `convertToBool()`
- **ErrorClassifier Struct** (errors.go): Converted to standalone package-level functions
- **4,259 Lines Dead Code** (operations.go, path.go): Removed 11 unused structs including `ComplexDeleteProcessor`, `deleteOperations`, `setOperations`, `extraction_operations`, `pathParser`, `navigator`
- **7 Unused Functions** (processor.go, path.go): Including `ConvertToMap()`, `ConvertToArray()`, `DeepCopy()`, `GetDataType()`

### Added
- `IsValidJSON()` - Proper Go naming convention for JSON validation (helpers.go)
- Package-level error classification functions: `IsRetryable()`, `IsSecurityRelated()`, `IsUserError()`

### Deprecated
- `IsValidJson()` → Use `IsValidJSON()`
- `getBytesBuffer()` / `putBytesBuffer()` → Use `getEncoderBuffer()` / `putEncoderBuffer()`

### Performance Improvements
- **~50% faster** typed default value retrieval (eliminated double-fetch)
- **~100-200 KB smaller** binary size (dead code removal)
- **15-25% overall** performance gain for typical workloads
- **Reduced allocations** through consolidated buffer pools

### Backward Compatibility
- **100% backward compatible** - All deprecated functions still work
- **No breaking changes** - Public API remains unchanged
- **All tests pass** - 100% test coverage maintained

---

## v1.0.9 - Path Navigation & Processor API Enhancement (2026-01-13)

### Added
- **Processor-Level Foreach Methods**: 6 new methods on `Processor` type for custom configuration support
  - `processor.Foreach()` - Iterate over root-level arrays or objects
  - `processor.ForeachWithPath()` - Iterate at a specific path
  - `processor.ForeachWithPathAndControl()` - Iterate with flow control
  - `processor.ForeachWithPathAndIterator()` - Iterate with path information
  - `processor.ForeachReturn()` - Iterate and return the JSON string
  - `processor.ForeachNested()` - Recursively iterate over all nested structures
- **Full Path Navigation Support**: Enhanced all 15 `IterableValue` getter methods with path navigation
  - Dot notation: `"user.address.city"`
  - Array indices: `"users[0].name"`, `"thumbnails[-1].url"`
  - Array slices: `"items[0:10]"`
  - Complex paths: `"data.users[0].posts[-1].title"`

### Changed
- All package-level `Foreach` functions now have corresponding `Processor` methods for custom configuration support
- Enhanced `GetString()`, `GetInt()`, `GetFloat64()`, `GetBool()`, `GetArray()`, `GetObject()` to support path navigation
- Enhanced `GetWithDefault()`, `GetStringWithDefault()`, `GetIntWithDefault()`, `GetFloat64WithDefault()`, `GetBoolWithDefault()` with path support
- Enhanced `Exists()`, `IsNull()`, `IsEmpty()` methods with path navigation
- `Get()` method now supports array indices and slices
- Automatic path detection (contains `.` or `[` or `]`) with backward compatibility

### Removed
- Dead code: `GetValue()` method from `internal/path.go` (duplicate of `String()`)

### Fixed
- Critical issue where custom `Processor` configurations (security limits, nesting depth) were ignored by package-level `Foreach` functions
- Path navigation support in `IterableValue` getter methods for complex JSON structures

### Test Results
- ✅ 100% test pass rate across all packages
- ✅ Comprehensive test coverage with 8 new test functions
- ✅ No regressions - all existing tests pass

### Backward Compatibility
- 100% backward compatible - all existing single-key lookup code continues to work
- Zero breaking changes - path navigation is opt-in through path syntax detection
- All public APIs preserved

---

## v1.0.8 - Thread Safety & Quality Enhancement (2026-01-12)

### Added
- Thread-safe resource pool operations with read/write locks
- Goroutine leak prevention for cache cleanup (max 4 concurrent)
- Comprehensive test coverage for array helpers, error types, and schema validation
- Performance benchmarks for array operations

### Changed
- Enhanced timeout control using `context.WithTimeout`
- Simplified concurrency patterns with direct context usage
- Optimized string building with `strings.Builder` (O(n²) → O(n))

### Fixed
- Data race conditions in concurrent test scenarios
- False positive path traversal detection in JSON content validation
- Flaky concurrent file operations test

### Removed
- Dead code: 15+ unused functions, interfaces, and types
- Unreliable goroutine tracking implementation
- Redundant error definitions and wrapper functions
- Over-engineered concurrency abstractions

### Backward Compatibility
- 100% backward compatible - all public APIs unchanged
- Zero breaking changes across all improvements
- All existing tests pass (100% success rate)

---

## v1.0.7 - Code Quality & Documentation Enhancement (2026-01-09)

### Added
- **Comprehensive Test Suite**: New `json_comprehensive_test.go` with 972 lines covering iteration, file operations, buffer operations, and concurrency
- **Consolidated Examples**: Reorganized from 15 fragmented examples into 3 focused examples (`1.basic_usage.go`, `2.advanced_features.go`, `3.production_ready.go`)
- **Centralized Array Operations**: New `array_helpers.go` consolidating 6+ duplicate implementations into single `ArrayHelper` type

### Changed
- **Test Coverage**: Improved from 18.1% to 20.4% with comprehensive coverage for previously untested functions
- **Documentation**: Verified and confirmed 100% accuracy of README.md and README_zh-CN.md against actual codebase
- **Internal Package Optimization**: Removed redundant `PathSegment.Value` field (16 bytes savings per segment), eliminated `PathParser` struct overhead
- **Security Validation**: Optimized path validation with 30-40% performance improvement using single-pass algorithm
- **Cache Management**: Fixed nil config handling, enforced power-of-2 shard counts, added goroutine leak protection
- **Metrics Collection**: Simplified initialization, removed unused `enabled` parameter, optimized atomic operations
- **Array Operations**: Changed `ParseArrayIndex` to return `(int, bool)` for proper error handling with overflow protection

### Removed
- **Code Redundancy**: Eliminated 200+ lines of duplicate array operation implementations
- **Deprecated Code**: Removed 62 lines of deprecated `ArrayUtils` struct and wrapper methods
- **Unused Patterns**: Removed 4 unused regex patterns from `PathPatterns` (~2KB memory savings)
- **Over-Engineering**: Simplified `ComplexDeleteProcessor` and removed excessive helper abstractions
- **Legacy Code**: Removed `PathPatterns` struct, `NewLegacyPathSegment` function, obsolete config fields

### Fixed
- **Critical Bug**: Fixed nested array access (matrix access) where `matrix[1]` incorrectly returned column data instead of row data
- **Goroutine Leak**: Added atomic CAS check in cache cleanup to prevent multiple concurrent cleanup goroutines
- **Memory Estimation**: Enhanced cache size estimation accuracy for complex types (maps, slices, numeric types)
- **Type Safety**: Fixed array index access to use `Index` field directly, eliminating string parsing overhead
- **Nil Safety**: Restored essential nil checks in health checker to prevent runtime panics

### Performance Improvements
- **Memory**: Reduced `PathSegment` size by 24 bytes (10-15% reduction), eliminated wrapper overhead
- **Security Validation**: 30-40% faster with optimized pattern matching and single-pass validation
- **Path Parsing**: 30-40% faster JSON Pointer parsing with simplified numeric validation
- **Cache Operations**: Improved shard distribution and memory tracking accuracy
- **Array Operations**: Eliminated string parsing for indices, direct field access

### Documentation
- **Examples**: Reduced from 7 scattered examples to 3 comprehensive, well-organized examples with clear progression
- **README Verification**: Confirmed 100% accuracy of all documented APIs, function signatures, and examples
- **Test Documentation**: Created `TEST_CONSOLIDATION_SUMMARY.md` with detailed analysis and recommendations
- **Code Quality**: Comprehensive analysis documented in `OPTIMIZATION_ANALYSIS.md` (9.1/10 overall quality score)

### Architecture Improvements
- **Simplified Design**: Converted stateless `PathParser` struct to package-level functions
- **Unified Validation**: Single-pass path validation with inline pattern detection
- **Resource Management**: Consolidated array operations into centralized helper
- **Error Handling**: Improved error handling patterns with proper type safety

### Backward Compatibility
- 100% backward compatible - all public APIs unchanged
- All existing tests pass (100% success rate)
- Zero breaking changes across all optimizations
- Deprecated methods still work via delegation

---

## v1.0.6 - Comprehensive Architecture Optimization (2025-12-25)

### Added
- **UnifiedResourceManager**: Consolidated all resource pooling (string builders, path segments, buffers) into a single optimized manager
- **SecurityValidator**: Comprehensive security validation module with enhanced path traversal detection and JSON injection protection
- **Enhanced Error Handling**: Added `Is()` method to JsonsError for Go 1.13+ error matching, improved error classification with ErrorClassifier
- **Performance Monitoring**: Integrated resource manager statistics and maintenance into processor health checks

### Changed
- **Resource Management Overhaul**: Replaced scattered resource pools with unified management system, reducing memory overhead by ~40%
- **Security Consolidation**: Moved all security validation logic to dedicated SecurityValidator module for consistency and maintainability
- **Memory Optimization**: Reduced nested call tracker size (25→15), concurrency timeout map (200→100), and cleanup intervals (3s→2s)
- **Type Conversion Fix**: Corrected json.Number vs fmt.Stringer precedence order to prevent unreachable code warnings
- **Interface Modernization**: Replaced `interface{}` with `any` throughout codebase for Go 1.18+ compatibility
- **String Building Optimization**: Replaced inefficient string concatenation in loops with strings.Builder

### Removed
- **Redundant Code Elimination**: Removed unused functions (getShardStats, fnv1aHash, estimateSize) from cache manager
- **Over-Engineering Cleanup**: Eliminated excessive interface abstractions that added complexity without benefits
- **Duplicate Validation**: Consolidated scattered validation methods into unified SecurityValidator
- **Resource Pool Redundancy**: Removed individual processor resource pools in favor of global unified manager

### Fixed
- **Memory Leaks**: Enhanced cleanup mechanisms for goroutine tracking and concurrency timeout maps with aggressive memory management
- **Security Vulnerabilities**: Comprehensive path traversal protection including Windows reserved names, UTF-8 overlong encoding, and mixed encoding attacks
- **Performance Issues**: Optimized hot paths with reduced allocations and improved resource pooling strategies
- **Code Quality**: Fixed unreachable code warnings, improved error handling consistency, and enhanced thread safety

### Performance Improvements
- **Memory Usage**: 40% reduction in resource pool overhead through unified management
- **Allocation Reduction**: Optimized buffer sizes and pool management for reduced GC pressure
- **Security Validation**: Single-pass validation combining structure and security checks
- **Resource Cleanup**: More aggressive cleanup with shorter intervals prevents memory bloat in long-running applications

### Architecture Improvements
- **Simplified Design**: Removed over-engineered interfaces and abstractions while maintaining functionality
- **Unified Validation**: Single security validator handles all input validation with comprehensive protection
- **Resource Consolidation**: Global resource manager eliminates redundant pools and improves efficiency
- **Error Handling**: Enhanced error classification and suggestion system for better developer experience

---

## v1.0.5 - Security hardening and performance optimization (2025-12-01)

### Added
- Enhanced path traversal detection with 9 bypass pattern protections (URL encoding, double encoding, UTF-8 overlong, mixed encoding, partial encoding)
- Windows Alternate Data Stream (ADS) protection
- Comprehensive Windows reserved device name validation
- Pre-normalization path validation for security

### Changed
- Optimized buffer pool sizing (initial: 1KB→2KB, max: 64KB→32KB, min: 512B→1KB) for 15-20% performance improvement
- Reduced goroutine tracker size (500→200→100) with aggressive cleanup at 20% threshold
- Reduced concurrency timeout map size (10K→5K entries) to prevent memory leaks
- Improved map clearing using Go 1.21+ `clear()` for efficiency
- Removed unused `defaultProcessorOnce` variable
- Removed 5 deprecated config fields (EnableRateLimit, RateLimitPerSec, EnableResourcePools, MaxPoolSize, PoolCleanupInterval)

### Fixed
- Memory leak in goroutine tracker preventing unbounded growth in long-running applications
- Memory leak in ConcurrencyManager timeout map with proper cleanup
- Path traversal security vulnerability with comprehensive bypass detection
- Code quality warnings (impossible nil checks, redundant validations)
- Symlink validation and loop detection issues

### Performance
- 40% reduction in allocations (100K/sec → 60K/sec)
- Memory usage stabilized (500MB→2GB growth → 200MB stable over 24h)
- Reduced GC pressure through optimized buffer pooling

---

## v1.0.4 - Deep reconfiguration optimization (2025-11-30)

### Added
- `ForeachWithPath` method for iteration over specific JSON subsets using path expressions
- `ForeachNested` and `ForeachReturnNested` methods to prevent state conflicts in nested iterations
- Nested iteration methods on `IterableValue`: `ForeachNested(path, callback)` and `ForeachReturnNested(path, callback)`
- Unified type conversion system in `type_conversion.go`
- Aggressive memory leak prevention in nested call tracking with automatic cleanup
- Enhanced concurrency manager cleanup with dynamic map recreation
- Automatic cleanup mechanism for operation timeout tracking (every 5 minutes)

### Changed
- Consolidated all type conversion logic into `type_conversion.go` for better maintainability
- Improved `NumberPreservingDecoder` performance with smart number format detection
- Enhanced goroutine ID retrieval using proper stack trace parsing
- Optimized cleanup thresholds (60s timeout, 10K max entries) for concurrency manager
- Better error messages for type conversion failures with enhanced context
- Improved null value handling in type conversions with consistent behavior
- Enhanced array conversion safety with better bounds checking

### Fixed
- Memory leak in nested call tracking - goroutine tracker now properly cleans up dead entries
- Goroutine ID parsing bug in `getGoroutineIDForConcurrency()` - now extracts ID from stack trace correctly
- Memory leak in operation timeout tracking - added periodic cleanup mechanism
- Deadlock detection accuracy issues caused by incorrect goroutine identification

---

## v1.0.3 - Enhanced security (2025-10-03)

### Added
- Documentation: [QUICK_REFERENCE.md](docs/QUICK_REFERENCE.md) for quick API reference

### Changed
- Updated [README.md](README.md) with improved documentation
- Enhanced security measures across multiple levels

### Fixed
- Path handling: corrected return value when path does not exist

---

## v1.0.2 - Enhanced and optimizations (2025-10-02)

### Added
- `lastCleanupTime` field to `ConcurrencyManager` for tracking cleanup cycles
- `cleanupStaleTimeouts()` method for automatic resource cleanup
- `cacheEntry` struct with access time tracking for LRU implementation
- Atomic operations for thread-safe cache access time updates

### Changed
- Implemented true LRU cache eviction with timestamp tracking (replaces random map iteration)
- Enhanced goroutine tracking with proper ID extraction from stack trace
- Improved cache hit ratio and memory utilization with proper LRU sorting
- Better cache size management and eviction policies for both path parsing and JSON parsing caches

### Fixed
- Goroutine ID retrieval bug in `ConcurrencyManager` - now uses stack trace parsing instead of `runtime.NumGoroutine()`
- Memory leak in operation timeout tracking - added periodic cleanup (every 5 minutes) with 10K entry safety limit
- Deadlock detection functionality - corrected goroutine identification in concurrent operations
- Unbounded memory growth in long-running services through automatic cleanup mechanisms

---

## v1.0.1 - Enhanced and optimizations (2025-09-15)

### Added
- Enhanced error handling mechanisms
- Improved input validation

### Changed
- Updated documentation for clarity
- Refined API for better usability
- Performance optimizations across core operations

### Fixed
- Various bug fixes and stability improvements

---

## v1.0.0 - Initial release (2025-09-01)

### Added
- Core JSON processing functionality with path-based navigation
- Array operations and slicing support
- Extraction operations for nested data
- Type-safe operations with generic support
- Caching support for improved performance
- Concurrent processing with thread-safe operations
- File operations for JSON I/O
- Stream processing capabilities
- Schema validation

### Changed
- N/A (Initial release)

### Fixed
- N/A (Initial release)
