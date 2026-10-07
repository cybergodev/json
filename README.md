# cybergodev/json

[![Go Version](https://img.shields.io/badge/Go-1.25+-00ADD8.svg)](https://golang.org)
[![GoDoc](https://pkg.go.dev/badge/github.com/cybergodev/json.svg)](https://pkg.go.dev/github.com/cybergodev/json)
[![MIT License](https://img.shields.io/badge/License-MIT-green.svg)](https://opensource.org/licenses/MIT)
[![Thread Safe](https://img.shields.io/badge/Thread_Safe-Yes-brightgreen.svg)](#)
[![Security](https://img.shields.io/badge/Security-Hardened-red.svg)](docs/SECURITY.md)

> A high-performance, thread-safe Go JSON processing library with 100% `encoding/json` compatibility.
> Powerful path syntax, type safety, streaming processing, production-grade performance.

**[中文文档](README_zh-CN.md)** | **[www.cybergo.dev/json](https://www.cybergo.dev/json)**

---

## Table of Contents

- [Why cybergodev/json](#why-cybergodevjson)
- [Features](#features)
- [Installation](#installation)
- [Quick Start](#quick-start)
- [Path Syntax Reference](#path-syntax-reference)
- [Core API](#core-api)
  - [Data Retrieval](#data-retrieval) · [Parsing](#parsing) · [Data Modification](#data-modification)
  - [Encoding and Formatting](#encoding-and-formatting) · [File Operations](#file-operations) · [JSON Utilities](#json-utilities)
- [Configuration](#configuration)
- [Advanced Features](#advanced-features)
- [Common Use Cases](#common-use-cases)
- [Performance Monitoring](#performance-monitoring)
- [Migrating from encoding/json](#migrating-from-encodingjson)
- [Security Configuration](#security-configuration)
- [Example Code](#example-code)
- [Documentation](#documentation)
- [License](#license)

---

## Why cybergodev/json

| Feature | encoding/json | cybergodev/json |
|--------|---------------|-----------------|
| Path-based access | Manual unmarshal | `json.Get(data, "users[0].name")` |
| Negative index | - | `items[-1]` gets last element |
| Flatten nested arrays | - | `users{flat:tags}` |
| JSON Pointer (RFC 6901) | - | `/users/0/name` |
| Type-safe defaults | - | `GetString(data, "path", "default")` |
| Streaming large files | - | Built-in streaming processors |
| Memory pooling | - | `sync.Pool` for hot paths |
| Path caching | - | Smart cache with TTL |
| Batch operations | - | `ProcessBatch()` for bulk work |
| 100% Compatibility | Native | Drop-in replacement (signatures extended with optional config) |

---

## Features

- **100% Compatible** - Drop-in replacement for `encoding/json`, all standard function signatures supported with optional config extension
- **Powerful Paths** - Dot notation, array slicing, field extraction, JSON Pointer (RFC 6901)
- **High Performance** - Smart caching, memory pooling, optimized hot paths
- **Type Safe** - Generics support with `GetTyped[T]`, built-in defaults, `AccessResult` type conversion
- **Feature Rich** - Batch operations, streaming, file I/O, schema validation, deep merge, JSONL
- **Production Ready** - Thread-safe, comprehensive error handling, security hardened, health monitoring

---

## Installation

```bash
go get github.com/cybergodev/json
```

**Requirements**: Go 1.25 or later

**Import**:

```go
import "github.com/cybergodev/json"
```

---

## Quick Start

```go
package main

import (
    "fmt"
    "github.com/cybergodev/json"
)

func main() {
    data := `{"user": {"name": "Alice", "age": 28, "tags": ["premium", "verified"]}}`

    // Simple field access (returns value directly, no error)
    name := json.GetString(data, "user.name")
    fmt.Println(name) // "Alice"

    // Type-safe retrieval with generics
    age := json.GetTyped[int](data, "user.age", 0)
    fmt.Println(age) // 28

    // With default value (no panic on missing path)
    email := json.GetTyped[string](data, "user.email", "unknown@example.com")
    fmt.Println(email) // "unknown@example.com"

    // Negative indexing (last element)
    lastTag, _ := json.Get(data, "user.tags[-1]")
    fmt.Println(lastTag) // "verified"

    // Modify data
    updated, _ := json.Set(data, "user.age", 29)
    newAge := json.GetInt(updated, "user.age")
    fmt.Println(newAge) // 29

    // 100% encoding/json compatible
    bytes, _ := json.Marshal(map[string]any{"status": "ok"})
    fmt.Println(string(bytes)) // {"status":"ok"}
}
```

---

## Path Syntax Reference

| Syntax | Description | Example |
|--------|-------------|---------|
| `.property` | Access property | `user.name` -> "Alice" |
| `[n]` | Array index | `items[0]` -> first element |
| `[-n]` | Negative index (from end) | `items[-1]` -> last element |
| `[start:end]` | Array slice | `items[1:3]` -> elements 1-2 |
| `[start:end:step]` | Slice with step | `items[::2]` -> every other element |
| `[+]` | Append to array | `items[+]` -> append position |
| `{field}` | Extract field from all elements | `users{name}` -> ["Alice", "Bob"] |
| `{flat:field}` | Flatten nested arrays | `users{flat:tags}` -> merge all tags |
| `{f1,f2,...}` | Multi-field extraction (subset object) | `address{city,zip}` -> `{"city":...,"zip":...}` |
| `/pointer` | JSON Pointer (RFC 6901) | `/users/0/name` -> "Alice" |

---

## Core API

### Data Retrieval

```go
// Basic getters - return value directly, accept optional default
// When path is missing or type mismatches: returns zero value, or default if provided
json.Get(data, "user.name")            // (any, error)
json.GetString(data, "user.name")      // string
json.GetInt(data, "user.age")          // int
json.GetFloat(data, "user.score")      // float64
json.GetBool(data, "user.active")      // bool
json.GetArray(data, "user.tags")       // []any
json.GetObject(data, "user.profile")   // map[string]any

// Type-safe generic retrieval
json.GetTyped[string](data, "user.name", "default")
json.GetTyped[[]int](data, "numbers", nil)
json.GetTyped[User](data, "user", User{})  // custom struct

// Typed getters with defaults
json.GetString(data, "user.name", "Anonymous")
json.GetInt(data, "user.age", 0)
json.GetBool(data, "user.active", false)
json.GetFloat(data, "user.score", 0.0)
json.GetTyped[[]any](data, "user.tags", []any{})

// Safe access with result type and type conversion
result := json.SafeGet(data, "user.age")
if result.Ok() {
    age, _ := result.AsInt()
    fmt.Println(age)
}

// Batch retrieval
results, err := json.GetMultiple(data, []string{"user.name", "user.age"})

// Context-aware retrieval (supports timeout/cancellation)
value, err := json.GetWithContext(ctx, data, "user.name")
```

### Parsing

```go
// Parse JSON to any (uses default processor)
data, err := json.ParseAny(jsonStr)

// Parse JSON into typed target
var user User
err = json.Parse(jsonStr, &user)

// With configuration
data, err = json.ParseAny(jsonStr, json.SecurityConfig())
```

### Data Modification

```go
// Basic set - returns modified JSON on success, original data on failure
result, err := json.Set(data, "user.name", "Bob")

// Auto-create paths with config
cfg := json.DefaultConfig()
cfg.CreatePaths = true
result, err := json.Set(data, "user.profile.level", "gold", cfg)

// Append to array
result, _ := json.Set(data, "user.tags[+]", "new-tag")

// Batch set
result, _ := json.SetMultiple(data, map[string]any{
    "user.name": "Bob",
    "user.age":  30,
})

// Delete
result, err := json.Delete(data, "user.temp")

// Delete with null cleanup
result, err = json.DeleteClean(data, "user.temp")

// Set with auto-create (creates intermediate paths)
result, err = json.SetCreate(data, "user.profile.level", "gold")

// Batch set with auto-create
result, err = json.SetMultipleCreate(data, map[string]any{
    "user.profile.level": "gold",
    "user.profile.badge": "star",
})
```

### Encoding and Formatting

```go
// Standard encoding (100% compatible)
bytes, _ := json.Marshal(data)
json.Unmarshal(bytes, &target)
bytes, _ := json.MarshalIndent(data, "", "  ")

// Quick formatting
pretty, _ := json.Prettify(jsonStr)       // pretty print

// Compact/minify using encoding/json compatible buffer API
var buf bytes.Buffer
json.Compact(&buf, []byte(jsonStr))
compact := buf.String()

// Or the string-in/string-out form (compact a JSON string directly)
compact, _ = json.CompactString(jsonStr)

// Encoding with config — Encode is the canonical string-returning encoder
// (EncodeWithConfig is a deprecated alias kept for compatibility)
cfg := json.DefaultConfig()
cfg.Pretty = true
cfg.SortKeys = true
result, _ := json.Encode(data, cfg)

// Preset configs
result, _ = json.Encode(data, json.PrettyConfig())

// Quick pretty encoding
result, _ = json.EncodePretty(data)

// Validate with configuration
valid, err := json.ValidWithConfig(jsonStr, json.SecurityConfig())
```

### File Operations

```go
// Load and save (package-level functions)
jsonStr, _ := json.LoadFromFile("data.json")
json.SaveToFile("output.json", data, json.PrettyConfig())

// Load from reader (with size limiting)
jsonStr, _ = json.LoadFromReader(reader)

// Struct/Map serialization
json.MarshalToFile("user.json", user)
json.UnmarshalFromFile("user.json", &user)

// Write to any io.Writer
json.SaveToWriter(writer, data, cfg)

// File iteration (process without loading entire file)
err := json.ForeachFile("data.json", func(key any, item *json.IterableValue) error {
    fmt.Println(item.GetString("name"))
    return nil
})

err = json.ForeachFileChunked("large.json", 100, func(chunk []*json.IterableValue) error {
    // Process 100 items at a time
    return nil
})

// Processor-based file operations with full config support
processor, _ := json.New(json.DefaultConfig())
defer processor.Close()
jsonStr, _ = processor.LoadFromFile("data.json")
_ = processor.SaveToFile("output.json", data, json.PrettyConfig())
```

### JSON Utilities

```go
// Compare two JSON strings
equal, _  := json.CompareJSON(json1, json2)

// Union merge (default) - combines all keys
merged, _ := json.MergeJSON(json1, json2)

// Intersection merge - only common keys
cfg := json.DefaultConfig()
cfg.MergeMode = json.MergeIntersection
merged, _ = json.MergeJSON(json1, json2, cfg)

// Difference merge - keys in json1 but not json2
cfg.MergeMode = json.MergeDifference
merged, _ = json.MergeJSON(json1, json2, cfg)

// Merge multiple JSON objects
merged, _ = json.MergeMany([]string{json1, json2, json3})
```

---

## Configuration

### Custom Configuration

```go
cfg := json.DefaultConfig()     // start from safe defaults
cfg.EnableCache = true
cfg.MaxCacheSize = 256
cfg.CacheTTL = 5 * time.Minute
cfg.MaxConcurrency = 50
cfg.CreatePaths = true          // auto-create paths on Set
cfg.CleanupNulls = true         // cleanup nulls after Delete

processor, err := json.New(cfg)
if err != nil {
    // handle configuration error
}
defer processor.Close()

// Use processor methods
result, _ := processor.Get(jsonStr, "user.name")
stats := processor.GetStats()
health := processor.GetHealthStatus()
processor.ClearCache()
```

### Preset Configurations

```go
// Choose a preset based on your use case:
cfg := json.DefaultConfig()   // balanced defaults
// cfg := json.SecurityConfig()  // for untrusted input
// cfg := json.PrettyConfig()    // for pretty output
```

---

## Advanced Features

### Multi-field Extraction

Extract a subset of fields from an object (or from every object in an array) using
comma-separated names inside `{}`. Unlisted fields are dropped; fields that don't
exist are silently skipped. Whitespace around the names is tolerated
(`{a, b, c}` is normalized to `{a,b,c}`).

```go
data := `{
    "address": {
        "city": "Metro City",
        "zip": "12345",
        "country": "Sample Country",
        "street": "1 Main Street"
    }
}`

// Extract only "city" and "zip" into a new object (country and street are dropped)
subset, _ := json.Get(data, "address{city,zip}")
// subset -> map[string]any{"city": "Metro City", "zip": "12345"}

// GetObject / GetTyped are the typed getters for the resulting map:
addr := json.GetObject(data, "address{city,zip}")
fmt.Println(addr["city"], addr["zip"]) // Metro City 12345
```

It applies to every element when the target is an array:

```go
arr := `{"users":[
    {"id":1,"name":"Alice","email":"a@x.com"},
    {"id":2,"name":"Bob","email":"b@x.com"}
]}`
names, _ := json.Get(arr, "users{id,name}")
// names -> [{"id":1,"name":"Alice"},{"id":2,"name":"Bob"}]
```

> **Note** — multi-field extraction returns an *object* (`map[string]any`), so prefer
> `Get` / `GetObject`. Calling `GetString` on it falls back to Go's default formatting:
> `json.GetString(data, "address{city,zip}", "")` returns `"map[city:Metro City zip:12345]"`,
> not a JSON string. A single field, `{city}`, is different — it returns the raw value
> (`"Metro City"`), not a wrapped object.

### Iteration

```go
// Basic iteration (error-returning form; the void Foreach is deprecated
// because it drops errors)
err := json.ForeachWithError(data, ".", func(key any, item *json.IterableValue) error {
    name := item.GetString("name")
    fmt.Printf("Key: %v, Name: %s\n", key, name)
    return nil
})

// With path
err = json.ForeachWithPath(data, "users", func(key any, item *json.IterableValue) {
    name := item.GetString("name")
    fmt.Printf("Key: %v, Name: %s\n", key, name)
})

// Nested iteration (specify nested field path)
err = json.ForeachNestedWithError(data, func(key any, item *json.IterableValue) error {
    item.ForeachNested("items", func(nestedKey any, nestedItem *json.IterableValue) {
        fmt.Printf("Nested: %v\n", nestedItem.Get("id"))
    })
    return nil
})

// Error-returning callback: return a non-nil error to abort iteration early.
// (For a plain callback use ForeachWithPath; for file input use ForeachFile.)
err = json.ForeachWithError(data, "users", func(key any, item *json.IterableValue) error {
    if item.IsNull("id") {
        log.Printf("warning: missing id at key %v", key)
    }
    return nil
})

// Iterator control (break / continue)
_ = json.ForeachWithPathAndControl(data, "users", func(key any, value any) json.IteratorControl {
    if key.(int) > 5 {
        return json.IteratorBreak
    }
    return json.IteratorNormal
})

// Iterate and return original JSON string
result, err := json.ForeachReturn(data, func(key any, item *json.IterableValue) {
    // iterate over all elements; original JSON string is returned unchanged
})
```

### Batch Operations

```go
data := `{"user": {"name": "Alice", "age": 28, "temp": "value"}}`

operations := []json.BatchOperation{
    {Type: "get", JSONStr: data, Path: "user.name", ID: "op1"},
    {Type: "set", JSONStr: data, Path: "user.age", Value: 25},
    {Type: "delete", JSONStr: data, Path: "user.temp"},
}
results, err := json.ProcessBatch(operations)
```

### Schema Validation

```go
schema := &json.Schema{
    Type:     "object",
    Required: []string{"name", "email"},
    Properties: map[string]*json.Schema{
        "name":  {Type: "string", MinLength: 1, MaxLength: 100},
        "email": {Type: "string", Format: "email"},
        "age":   {Type: "integer", Minimum: 0, Maximum: 150},
    },
}

validationErrors, err := json.ValidateSchema(jsonStr, schema)
```

### PreParse and CompiledPath

```go
processor, _ := json.New(json.DefaultConfig())
defer processor.Close()

// Pre-parse once, query many times (avoids re-parsing)
parsed, _ := processor.PreParse(jsonStr)
name, _    := processor.GetFromParsed(parsed, "user.name")
age, _     := processor.GetFromParsed(parsed, "user.age")
updated, _ := processor.SetFromParsed(parsed, "user.age", 30)

// Compile a path for fast repeated access
compiled, _ := processor.CompilePath("user.profile.settings.theme")
value, _    := processor.GetCompiled(jsonStr, compiled)
```

### Encode Utilities

```go
// EncodeStream - encode slice as JSON array
streamJSON, _ := json.EncodeStream(users, json.PrettyConfig())

// EncodeBatch - encode key-value pairs as JSON object
batchJSON, _ := json.EncodeBatch(pairs, cfg)

// EncodeFields - encode only specific fields (filter sensitive data)
fieldsJSON, _ := json.EncodeFields(user, []string{"id", "name", "email"}, cfg)
```

### JSONL Processing

```go
// Convert between JSON array and JSONL
jsonlData, _    := json.ToJSONL(records)          // []any -> JSONL bytes
jsonlString, _  := json.ToJSONLString(records)    // []any -> JSONL string
records, _      := json.ParseJSONL(jsonlData)     // JSONL bytes -> []any

// Stream JSONL from reader
processor, _ := json.New(json.DefaultConfig())
defer processor.Close()
err := processor.StreamJSONL(reader, func(lineNum int, item *json.IterableValue) error {
    fmt.Printf("Line %d: %s\n", lineNum, item.GetString("id"))
    return nil
})

// JSONL writer
writer := json.NewJSONLWriter(bufWriter)
writer.Write(record1)
writer.Write(record2)
writer.WriteAll(records)
stats := writer.Stats() // LinesProcessed, BytesWritten

// NDJSON file processing (StreamJSONLFile — NDJSONProcessor is deprecated)
proc, _ := json.New(json.DefaultConfig())
defer proc.Close()
err = proc.StreamJSONLFile("data.ndjson", func(lineNum int, item *json.IterableValue) error {
    fmt.Printf("Line %d: %v\n", lineNum, item.Get("id")) // item.GetData() for the map
    return nil
})

// JSONL filter, map, reduce (available as package-level functions too)
filtered, _ := json.FilterJSONL(reader, func(item *json.IterableValue) bool {
    return item.GetBool("active")
})
mapped, _   := json.MapJSONL(reader, func(lineNum int, item *json.IterableValue) (any, error) {
    return map[string]any{"id": item.GetString("id")}, nil
})
result, _   := json.ReduceJSONL(reader, 0, func(acc any, item *json.IterableValue) any {
    count := item.GetInt("count")
    return acc.(int) + count
})

// Additional JSONL utilities
items, _    := json.CollectJSONL(reader)       // collect all items
first, _, _ := json.FirstJSONL(reader, func(item *json.IterableValue) bool {
    return item.GetBool("target")              // find first matching item
})
err = json.ForeachJSONL(reader, func(lineNum int, item *json.IterableValue) error {
    fmt.Printf("Line %d: %s\n", lineNum, item.GetString("id"))
    return nil
})
```

### Streaming Iterators

```go
// Stream large JSON arrays without loading into memory
reader := strings.NewReader(largeJSONArray)
iter := json.NewStreamIterator(reader)
for iter.Next() {
    item := iter.Value()
    // process item
}

// Stream JSON objects (key-value pairs)
objIter := json.NewStreamObjectIterator(reader)
for objIter.Next() {
    key, value := objIter.Key(), objIter.Value()
}

// Batch processing with in-memory data
batchIter := json.NewBatchIterator(items, json.DefaultConfig())
for batchIter.HasNext() {
    batch := batchIter.NextBatch()
}
```

### Parallel Processing

```go
// ParallelIterator - parallel processing with worker pool
items := []any{"a", "b", "c", "d"}
iter := json.NewParallelIterator(items)

// Process items in parallel
err := iter.ForEach(func(idx int, item any) error {
    // process each item concurrently
    return nil
})

// Process in batches
err = iter.ForEachBatch(2, func(batchIdx int, batch []any) error {
    // process batch of items
    return nil
})

// Parallel JSONL streaming
processor, _ := json.New(json.DefaultConfig())
defer processor.Close()
err = processor.StreamJSONLParallel(reader, 4, func(lineNum int, item *json.IterableValue) error {
    // Process item with 4 parallel workers
    return nil
})
```

### Hooks

```go
processor, _ := json.New(json.DefaultConfig())
defer processor.Close()

// Logging hook - takes any type with Info(string, ...any) method
processor.AddHook(json.LoggingHook(slog.Default()))

// Timing hook - takes any type with Record(op string, duration time.Duration) method
// Define your recorder:
//
//	type MetricsRecorder struct{}
//	func (m *MetricsRecorder) Record(op string, d time.Duration) { /* record */ }
//
processor.AddHook(json.TimingHook(&MetricsRecorder{}))

// Validation hook - takes func(jsonStr, path string) error
processor.AddHook(json.ValidationHook(func(jsonStr, path string) error {
    if len(path) > 100 {
        return fmt.Errorf("path too long: %s", path)
    }
    return nil
}))

// Error hook - takes func(ctx json.HookContext, err error) error
processor.AddHook(json.ErrorHook(func(ctx json.HookContext, err error) error {
    log.Printf("operation %s on path %s failed: %v", ctx.Operation, ctx.Path, err)
    return err // return original or transformed error
}))

// Custom hook using HookFunc
processor.AddHook(json.HookFunc{
    BeforeFn: func(ctx json.HookContext) error {
        fmt.Printf("before: %s %s\n", ctx.Operation, ctx.Path)
        return nil
    },
    AfterFn: func(ctx json.HookContext, result any, err error) (any, error) {
        fmt.Printf("after: %s (err=%v)\n", ctx.Operation, err)
        return result, err
    },
})
```

---

## Common Use Cases

### API Response Processing

```go
apiResponse := `{
    "status": "success",
    "data": {
        "users": [{"id": 1, "name": "Alice", "permissions": ["read", "write"]}],
        "pagination": {"total": 25, "page": 1}
    }
}`

// Quick extraction
status := json.GetString(apiResponse, "status")
total  := json.GetInt(apiResponse, "data.pagination.total")

// Extract all user names
names, _ := json.Get(apiResponse, "data.users{name}")
// Result: ["Alice"]

// Flatten all permissions
permissions, _ := json.Get(apiResponse, "data.users{flat:permissions}")
// Result: ["read", "write"]

// JSON Pointer access
name, _ := json.Get(apiResponse, "/data/users/0/name")
// Result: "Alice"
```

### Configuration Management

```go
config := `{
    "database": {"host": "localhost", "port": 5432},
    "cache": {"enabled": true}
}`

// Type-safe with defaults
dbHost        := json.GetString(config, "database.host", "localhost")
dbPort        := json.GetInt(config, "database.port", 5432)
cacheEnabled  := json.GetBool(config, "cache.enabled", false)

// Dynamic update
updated, _ := json.SetMultiple(config, map[string]any{
    "database.host": "prod-db.example.com",
    "cache.ttl":     3600,
})
```

### Multi-Source Data Merge

```go
// Merge configs from multiple sources
defaults := `{"timeout": 30, "retries": 3, "debug": false}`
file     := `{"timeout": 60, "debug": true}`
env      := `{"retries": 5}`

// Union merge (default behavior)
merged, _ := json.MergeMany([]string{defaults, file, env})
// Result: {"timeout": 60, "retries": 5, "debug": true}
```

---

## Performance Monitoring

```go
// Package-level monitoring
stats := json.GetStats()
fmt.Printf("Operations: %d\n", stats.OperationCount)
fmt.Printf("Cache Hit Rate: %.2f%%\n", stats.HitRatio*100)

health := json.GetHealthStatus()
fmt.Printf("Health Status: %v\n", health.Healthy)

// Cache management
json.ClearCache()

// Cache warmup - preload paths for faster access
paths := []string{"user.name", "user.age", "user.profile"}
result, _ := json.WarmupCache(jsonStr, paths)

// Processor-level monitoring
processor, _ := json.New(json.DefaultConfig())
defer processor.Close()
stats := processor.GetStats()
health := processor.GetHealthStatus()
processor.ClearCache()
```

### Benchmarks

The library ships with a benchmark suite that compares hot paths — the fast
encoder, path-based `Get`, array slicing, and field extraction — against
`encoding/json`. Run it to measure performance on your own hardware:

```bash
# Full benchmark suite with allocation stats
go test -run='^$' -bench=. -benchmem ./...
```

---

## Migrating from encoding/json

Simply change the import:

```go
// Before
import "encoding/json"

// After
import "github.com/cybergodev/json"
```

All standard functions are fully compatible:
- `json.Marshal()` / `json.Unmarshal()`
- `json.MarshalIndent()`
- `json.NewEncoder()` / `json.NewDecoder()`
- `json.Valid()`
- `json.Compact()` / `json.Indent()` / `json.HTMLEscape()`

Compatible types: `Encoder`, `Decoder`, `Number`, `Token`, `Delim`, `SyntaxError`, `UnmarshalTypeError`, `InvalidUnmarshalError`, `UnsupportedTypeError`, `UnsupportedValueError`, `MarshalerError`.

**Note**: `RawMessage` is not currently re-exported. Use `encoding/json.RawMessage` if needed.

See [Compatibility Guide](docs/COMPATIBILITY.md) for full details.

---

## Security Configuration

```go
// For handling untrusted JSON input
secureConfig := json.SecurityConfig()
// Features:
// - Full security scanning enabled
// - Conservative size limits (max 10MB)
// - Strict mode validation
// - Prototype pollution protection
// - Path traversal protection

processor, _ := json.New(secureConfig)
defer processor.Close()
```

### Security Utilities

```go
// Register custom dangerous patterns
json.RegisterDangerousPattern(json.DangerousPattern{
    Pattern: "eval\\(",
    Name:    "eval-injection",
    Level:   json.PatternLevelCritical,
})

// List all registered patterns
patterns := json.ListDangerousPatterns()

// Safe error reporting (no internal details leaked)
safeMsg := json.SafeError(err)

// Redact sensitive paths in logs
redacted := json.RedactedPath("user.password")
```

### Error Handling

```go
// Sentinel errors for programmatic checks
errors.Is(err, json.ErrInvalidJSON)
errors.Is(err, json.ErrPathNotFound)
errors.Is(err, json.ErrTypeMismatch)
errors.Is(err, json.ErrSizeLimit)
errors.Is(err, json.ErrSecurityViolation)

// Structured error with context
var jsonErr *json.JsonsError
if errors.As(err, &jsonErr) {
    fmt.Printf("op=%s path=%s: %s\n", jsonErr.Op, jsonErr.Path, jsonErr.Message)
}
```

See [Security Guide](docs/SECURITY.md) for detailed security best practices.

---

## Example Code

| File | Description |
|------|-------------|
| [1_basic_usage](examples/1_basic_usage/main.go) | Core operations |
| [2_advanced_features](examples/2_advanced_features/main.go) | Complex paths, nested extraction |
| [3_production_ready](examples/3_production_ready/main.go) | Thread-safe patterns |
| [4_error_handling](examples/4_error_handling/main.go) | Error handling patterns |
| [5_encoding_options](examples/5_encoding_options/main.go) | Encoding configuration |
| [6_validation](examples/6_validation/main.go) | Schema validation |
| [7_type_conversion](examples/7_type_conversion/main.go) | Type conversion, Result[T] |
| [8_helper_functions](examples/8_helper_functions/main.go) | Helper utilities |
| [9_iterator_functions](examples/9_iterator_functions/main.go) | Iteration patterns |
| [10_file_operations](examples/10_file_operations/main.go) | File I/O |
| [11_with_defaults](examples/11_with_defaults/main.go) | Default value handling |
| [12_advanced_delete](examples/12_advanced_delete/main.go) | Delete operations |
| [13_batch_operations](examples/13_batch_operations/main.go) | Batch processing and caching |
| [14_streaming_iterators](examples/14_streaming_iterators/main.go) | Streaming iterators |
| [15_jsonl_processing](examples/15_jsonl_processing/main.go) | JSONL format processing |
| [16_hooks_and_security](examples/16_hooks_and_security/main.go) | Hooks (AddHook + Config.Hooks), security |
| [17_advanced_patterns](examples/17_advanced_patterns/main.go) | PreParse, CompiledPath, package-level helpers |

```bash
# Run individual examples (build tag required)
go run -tags=example ./examples/1_basic_usage
go run -tags=example ./examples/2_advanced_features

# Compile-check all examples at once (same gate CI runs)
go vet -tags example ./examples/...
```

---

## Documentation

- **[API Reference](docs/API_REFERENCE.md)** - Complete API documentation
- **[Security Guide](docs/SECURITY.md)** - Security best practices
- **[Quick Reference](docs/QUICK_REFERENCE.md)** - Common patterns at a glance
- **[Compatibility](docs/COMPATIBILITY.md)** - encoding/json compatibility details
- **[pkg.go.dev](https://pkg.go.dev/github.com/cybergodev/json)** - GoDoc

---

## License

MIT License - See [LICENSE](LICENSE) file for details.
