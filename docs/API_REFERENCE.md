# API Reference - cybergodev/json

> Complete API Reference for github.com/cybergodev/json library

---

## Table of Contents

- [Core Functions](#core-functions)
- [encoding/json Streaming Types](#encodingjson-streaming-types)
- [Data Retrieval (Get)](#data-retrieval-get)
- [Data Modification (Set)](#data-modification-set)
- [Data Deletion (Delete)](#data-deletion-delete)
- [Encoding Functions](#encoding-functions)
- [Decoding Functions](#decoding-functions)
- [File Operations](#file-operations)
- [Iteration Functions](#iteration-functions)
- [Streaming Processing](#streaming-processing)
- [JSONL Support](#jsonl-support)
- [Validation Functions](#validation-functions)
- [Type Conversion](#type-conversion)
- [Data Utilities](#data-utilities)
- [Batch Processing](#batch-processing)
- [Cache Operations](#cache-operations)
- [Security Functions](#security-functions)
- [Utility Functions](#utility-functions)
- [Hook Constructors](#hook-constructors)
- [Processor Management](#processor-management)
- [Configuration](#configuration)
- [Processor Type](#processor-type)
- [Error Types](#error-types)
- [IterableValue Type](#iterablevalue-type)
- [AccessResult Type](#accessresult-type)
- [Result[T] Type](#resultt-type)
- [Schema Type](#schema-type)
- [Iterator Control](#iterator-control)
- [Additional Types](#additional-types)
- [Extension Interfaces](#extension-interfaces)
- [Path Expression Syntax](#path-expression-syntax)

---

## Core Functions

### Marshal

```go
func Marshal(value any, cfg ...Config) ([]byte, error)
```

Encodes a Go value to JSON. Signature-compatible with `encoding/json.Marshal`
and byte-compatible for typical values, with one deliberate difference (D-002):
the encoded output is capped at `Config.MaxJSONSize` (default 100MB) and returns
`ErrSizeLimit` instead of succeeding — stdlib has no such cap.

**Parameters:**
- `value` - Any Go value to encode

**Returns:**
- `[]byte` - JSON encoded bytes
- `error` - Encoding error if any

**Example:**
```go
data := map[string]any{"name": "Alice", "age": 30}
jsonBytes, err := json.Marshal(data)
```

---

### Unmarshal

```go
func Unmarshal(data []byte, value any, cfg ...Config) error
```

Decodes JSON bytes into a Go value. Signature-compatible with
`encoding/json.Unmarshal`, but the no-config call applies this library's default
security hardening — deliberate differences from the standard library (D-002):
input larger than `Config.MaxJSONSize` (default 100MB) → `ErrSizeLimit`;
dangerous-pattern substrings in string values (e.g. `"__proto__"`, `"<script"`)
→ security violation (stdlib accepts them as plain data); invalid UTF-8 in
string values → rejected (stdlib replaces it with U+FFFD). Pass
`SkipValidation: true` for stdlib-exact behavior on trusted input.

**Parameters:**
- `data` - JSON bytes to decode
- `value` - Pointer to target value

**Returns:**
- `error` - Decoding error if any

**Example:**
```go
var result map[string]any
err := json.Unmarshal(jsonBytes, &result)
```

---

### MarshalIndent

```go
func MarshalIndent(v any, prefix, indent string, cfg ...Config) ([]byte, error)
```

Encodes a Go value to formatted JSON with indentation.

**Parameters:**
- `v` - Any Go value to encode
- `prefix` - Prefix for each line
- `indent` - Indentation string

**Example:**
```go
jsonBytes, err := json.MarshalIndent(data, "", "  ")
```

---

### Valid

```go
func Valid(data []byte, cfg ...Config) bool
```

Reports whether data is valid JSON. Signature-compatible with
`encoding/json.Valid`, but the no-config call is not purely syntactic (D-002):
it routes through default security validation, so syntactically valid JSON can
still return `false` when it exceeds `MaxJSONSize`, contains invalid UTF-8, or
carries a dangerous pattern in a string value. For a syntax-only check use
`encoding/json.Valid` (or `ValidWithConfig` with `SkipValidation: true`).

**Parameters:**
- `data` - JSON bytes to validate

**Returns:**
- `bool` - true if valid JSON

---

## encoding/json Streaming Types

### Encoder

```go
type Encoder struct { /* unexported fields */ }

func NewEncoder(w io.Writer, cfg ...Config) *Encoder
func (enc *Encoder) Encode(v any) error
func (enc *Encoder) SetEscapeHTML(on bool)
func (enc *Encoder) SetIndent(prefix, indent string)
```

Writes JSON values to an output stream. Compatible with `encoding/json.Encoder` (byte-identical output for typical values), with one deliberate difference (D-002): the encoded output is capped at `Config.MaxJSONSize` (default 100MB) and returns `ErrSizeLimit` instead of succeeding. The optional `cfg` parameter allows customization of encoding behavior (e.g., `EscapeHTML`, `Pretty`, `Prefix`, `Indent`).

**Example:**
```go
var buf bytes.Buffer
enc := json.NewEncoder(&buf)
enc.SetIndent("", "  ")
err := enc.Encode(map[string]any{"name": "Alice", "age": 30})
if err != nil {
    log.Fatal(err)
}
fmt.Println(buf.String())
```

---

### Decoder

```go
type Decoder struct { /* unexported fields */ }

func NewDecoder(r io.Reader, cfg ...Config) *Decoder
func (dec *Decoder) Decode(v any) error
func (dec *Decoder) UseNumber()
func (dec *Decoder) DisallowUnknownFields()
func (dec *Decoder) Buffered() io.Reader
func (dec *Decoder) InputOffset() int64
func (dec *Decoder) More() bool
func (dec *Decoder) Token() (Token, error)
```

Reads and decodes JSON values from an input stream. Compatible with `encoding/json.Decoder` (see [Security-Hardening Differences](../COMPATIBILITY.md) in the compatibility guide for `Unmarshal`-level input checks). Known limitation: `DisallowUnknownFields()` (and cfg `DisallowUnknown`) is enforced only on the `UseNumber()` path — on the default decode path it is currently a no-op. The optional `cfg` parameter allows customization of decoding behavior (e.g., `MaxNestingDepthSecurity`, `MaxJSONSize`).

**Example:**
```go
reader := strings.NewReader(`{"name": "Alice"} {"name": "Bob"}`)
dec := json.NewDecoder(reader)
for dec.More() {
    var obj map[string]any
    if err := dec.Decode(&obj); err != nil {
        break
    }
    fmt.Println(obj["name"])
}
```

---

### Number, Token, and Delim

```go
type Number string
func (n Number) String() string
func (n Number) Float64() (float64, error)
func (n Number) Int64() (int64, error)
func (n Number) MarshalJSON() ([]byte, error)

type Token any
type Delim rune
func (d Delim) String() string
```

`Number` represents a JSON number literal, preserving exact representation. `Token` holds a value of one of these types: `Delim`, `bool`, `float64`, `Number`, `string`, or `nil`. `Delim` represents one of the four JSON delimiters `[ ] { }`.

Use `Decoder.UseNumber()` to decode numbers as `Number` instead of `float64`:

```go
dec := json.NewDecoder(reader)
dec.UseNumber()
var obj map[string]any
dec.Decode(&obj)
if n, ok := obj["count"].(json.Number); ok {
    val, _ := n.Int64()
    fmt.Println(val)
}
```

---

## Data Retrieval (Get)

### Get

```go
func Get(jsonStr, path string, cfg ...Config) (any, error)
```

Retrieves a value from JSON at the specified path.

**Parameters:**
- `jsonStr` - JSON string
- `path` - Path expression (e.g., "user.name", "items[0]")
- `cfg` - Optional configuration

**Returns:**
- `any` - The value at the path
- `error` - Path or parsing error

**Example:**
```go
value, err := json.Get(data, "users[0].name")
```

---

### GetString

```go
func GetString(jsonStr, path string, defaultValue ...string) string
```

Retrieves a string value from JSON at the specified path. Returns the zero value if the path is not found, the value is null, or type conversion fails, or the provided `defaultValue` if given.

**Example:**
```go
name := json.GetString(data, "user.name", "unknown")
```

---

### GetInt

```go
func GetInt(jsonStr, path string, defaultValue ...int) int
```

Retrieves an integer value from JSON at the specified path. Returns the zero value if the path is not found, the value is null, or type conversion fails, or the provided `defaultValue` if given.

**Example:**
```go
age := json.GetInt(data, "user.age", 0)
```

---

### GetFloat

```go
func GetFloat(jsonStr, path string, defaultValue ...float64) float64
```

Retrieves a float64 value from JSON at the specified path. Returns the zero value if the path is not found, the value is null, or type conversion fails, or the provided `defaultValue` if given.

**Example:**
```go
price := json.GetFloat(data, "product.price", 0.0)
```

---

### GetBool

```go
func GetBool(jsonStr, path string, defaultValue ...bool) bool
```

Retrieves a boolean value from JSON at the specified path. Returns the zero value if the path is not found, the value is null, or type conversion fails, or the provided `defaultValue` if given.

**Example:**
```go
active := json.GetBool(data, "user.active", false)
```

---

### GetArray

```go
func GetArray(jsonStr, path string, defaultValue ...[]any) []any
```

Retrieves an array value from JSON at the specified path. Returns `nil` if the path is not found, the value is null, or type conversion fails, or the provided `defaultValue` if given.

**Example:**
```go
items := json.GetArray(data, "items", []any{})
```

---

### GetObject

```go
func GetObject(jsonStr, path string, defaultValue ...map[string]any) map[string]any
```

Retrieves an object value from JSON at the specified path. Returns `nil` if the path is not found, the value is null, or type conversion fails, or the provided `defaultValue` if given.

**Example:**
```go
profile := json.GetObject(data, "user.profile")
```

---

### SafeGet

```go
func SafeGet(jsonStr, path string, cfg ...Config) AccessResult
```

Performs a type-safe get returning an `AccessResult` with conversion methods (`AsString`, `AsInt`, `AsFloat64`, `AsBool`, `AsStringConverted`, `Unwrap`, `UnwrapOr`). Accepts optional `Config` for validation, security, and caching.

**Example:**
```go
result := json.SafeGet(data, "user.age")
if result.Ok() {
    age, _ := result.AsInt()
}
```

---

### GetTyped[T]

```go
func GetTyped[T any](jsonStr, path string, defaultValue ...T) T
```

Retrieves a typed value from JSON at the specified path using generics. Returns the zero value if the path is not found, the value is null, or type conversion fails, or the provided `defaultValue` if given.

**Example:**
```go
name := json.GetTyped[string](data, "user.name", "Anonymous")
age := json.GetTyped[int](data, "user.age", 0)
```

---

### GetWithContext

```go
func GetWithContext(ctx context.Context, jsonStr, path string, cfg ...Config) (any, error)
```

Retrieves a value from JSON with boundary-level context checks. Context is checked before and after the operation, NOT during parsing/navigation. For large JSON documents, the operation may not respond to cancellation mid-parse.

**Parameters:**
- `ctx` - Context for timeout/cancellation
- `jsonStr` - JSON string
- `path` - Path expression
- `cfg` - Optional configuration

**Example:**
```go
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()
value, err := json.GetWithContext(ctx, data, "users[0].name")
```

---

### GetMultiple

```go
func GetMultiple(jsonStr string, paths []string, cfg ...Config) (map[string]any, error)
```

Retrieves multiple values from JSON at the specified paths in a single operation.

**Example:**
```go
paths := []string{"user.name", "user.age", "user.email"}
results, err := json.GetMultiple(data, paths)
name := results["user.name"]
```

---

## Data Modification (Set)

### Set

```go
func Set(jsonStr, path string, value any, cfg ...Config) (string, error)
```

Sets a value in JSON at the specified path.

**Parameters:**
- `jsonStr` - JSON string
- `path` - Path expression
- `value` - Value to set
- `cfg` - Optional configuration (auto-create paths; `CreatePaths` defaults to `true`)

**Returns:**
- `string` - Modified JSON string
- `error` - Error if operation failed

**Safety Guarantee:** On failure, returns the original unmodified JSON string.

**Example:**
```go
// Basic set
result, err := json.Set(data, "user.name", "Alice")

// Auto-create nested paths
cfg := json.DefaultConfig()
cfg.CreatePaths = true // redundant: CreatePaths defaults to true
result, err = json.Set(data, "new.nested.path", "value", cfg)
```

---

### Array Append with [+]

The `[+]` syntax allows appending values to arrays in a single operation:

```go
// Append single value to array
result, err := json.Set(data, "items[+]", "new-item")

// Append object to nested array
newMember := map[string]any{
    "name": "Charlie",
    "role": "Developer",
}
result, err = json.Set(data, "departments[0].teams[0].members[+]", newMember)

// Append multiple values (slice expansion)
moreItems := []any{4, 5, 6}
result, err = json.Set(data, "numbers[+]", moreItems)
// Result: numbers becomes [1, 2, 3, 4, 5, 6]
```

**Comparison with old approach:**
```go
// OLD WAY: 3 operations
members := json.GetArray(data, "users")              // Step 1: Get
members = append(members, newUser)                   // Step 2: Append
result, _ := json.Set(data, "users", members)        // Step 3: Set back

// NEW WAY: 1 operation
result, _ = json.Set(data, "users[+]", newUser)       // Single operation!
```

---

### SetMultiple

```go
func SetMultiple(jsonStr string, updates map[string]any, cfg ...Config) (string, error)
```

Sets multiple values in a single operation.

**Example:**
```go
updates := map[string]any{
    "user.name": "Bob",
    "user.age":  30,
}
result, err := json.SetMultiple(data, updates)
```

---

### SetCreate

```go
func SetCreate(jsonStr, path string, value any, cfg ...Config) (string, error)
```

Sets a value in JSON at the specified path, automatically creating intermediate paths. Equivalent to calling `Set` with `CreatePaths: true`.

**Example:**
```go
result, err := json.SetCreate(data, "new.nested.path", "value")
```

---

### SetMultipleCreate

```go
func SetMultipleCreate(jsonStr string, updates map[string]any, cfg ...Config) (string, error)
```

Sets multiple values in a single operation, automatically creating intermediate paths. Equivalent to calling `SetMultiple` with `CreatePaths: true`.

**Example:**
```go
updates := map[string]any{
    "user.profile.name": "Bob",
    "user.profile.age":  30,
}
result, err := json.SetMultipleCreate(data, updates)
```

---

## Data Deletion (Delete)

### Delete

```go
func Delete(jsonStr, path string, cfg ...Config) (string, error)
```

Deletes a value from JSON at the specified path.

**Parameters:**
- `jsonStr` - JSON string
- `path` - Path expression
- `cfg` - Optional configuration (use `CleanupNulls: true` to remove null values)

**Example:**
```go
result, err := json.Delete(data, "user.temp")

// With null cleanup
cfg := json.DefaultConfig()
cfg.CleanupNulls = true
result, err = json.Delete(data, "user.temp", cfg)
```

---

### DeleteClean

```go
func DeleteClean(jsonStr, path string, cfg ...Config) (string, error)
```

Deletes a value from JSON and removes null values. Equivalent to calling `Delete` with `CleanupNulls: true` **and** `CompactArrays: true` (cleans up null placeholders and empty array slots).

**Example:**
```go
result, err := json.DeleteClean(data, "user.temp")
```

---

## Encoding Functions

### Encode

```go
func Encode(value any, cfg ...Config) (string, error)
```

Converts any Go value to JSON string. This is the canonical string-returning
encoder (D-005 decision D1): the optional trailing `Config` selects the encoding
behavior for this call; omitted, the default configuration applies. `Marshal` is
the `[]byte`-returning `encoding/json` drop-in.

**Example:**
```go
result, err := json.Encode(data)                    // compact (default)
result, err = json.Encode(data, json.PrettyConfig()) // pretty
```

---

### EncodePretty

```go
func EncodePretty(value any, cfg ...Config) (string, error)
```

Converts any Go value to pretty-printed JSON string.

**Example:**
```go
result, err := json.EncodePretty(data)
```

---

### EncodeWithConfig

```go
func EncodeWithConfig(value any, cfg ...Config) (string, error)
```

Converts any Go value to JSON string with custom configuration.

> **Deprecated:** functionally identical to `Encode` — the name predates `Encode`
> accepting an optional trailing `Config`. Use `Encode(value, cfg)`. Kept for the
> lifetime of v1; will not be removed.

**Example:**
```go
cfg := json.PrettyConfig()
result, err := json.EncodeWithConfig(data, cfg) // → prefer json.Encode(data, cfg)
```

---

### Prettify

```go
func Prettify(jsonStr string, cfg ...Config) (string, error)
```

Formats JSON string with pretty indentation.

---

### EncodeBatch

```go
func EncodeBatch(pairs map[string]any, cfg ...Config) (string, error)
```

Encodes a map of key-value pairs as a JSON object.

**Example:**
```go
result, err := json.EncodeBatch(map[string]any{"name": "Alice", "age": 30})
```

---

### EncodeFields

```go
func EncodeFields(value any, fields []string, cfg ...Config) (string, error)
```

Encodes only the specified fields of a struct to JSON. Fields are matched against the serialized JSON key names (i.e. `json` tag names), not Go struct field names; a tagged struct must pass the tag names.

**Example:**
```go
result, err := json.EncodeFields(user, []string{"Name", "Email"})
// For a struct with tags (e.g. `json:"name"`), pass the tag names instead:
// json.EncodeFields(user, []string{"name", "email"})
```

---

### EncodeStream

```go
func EncodeStream(values any, cfg ...Config) (string, error)
```

Encodes a slice of values as a JSON array — a passthrough alias of `Encode`; non-slice values are not wrapped into an array.

**Example:**
```go
result, err := json.EncodeStream([]User{user1, user2})
```

---

### Compact

```go
func Compact(dst *bytes.Buffer, src []byte, cfg ...Config) error
```

Appends compacted JSON to dst. 100% compatible with encoding/json.Compact. Accepts optional Config.

---

### CompactString

```go
func CompactString(jsonStr string, cfg ...Config) (string, error)
```

Compacts a JSON string and returns the result as a string. This is the string-in/string-out
mirror of `Processor.Compact`. Accepts optional Config.

---

### Indent

```go
func Indent(dst *bytes.Buffer, src []byte, prefix, indent string, cfg ...Config) error
```

Appends indented JSON to dst. 100% compatible with encoding/json.Indent. Accepts optional Config.

---

### HTMLEscape

```go
func HTMLEscape(dst *bytes.Buffer, src []byte, cfg ...Config)
```

Appends HTML-escaped JSON to dst. 100% compatible with encoding/json.HTMLEscape. Accepts optional Config.

---

## Decoding Functions

### Parse

```go
func Parse(jsonStr string, target any, cfg ...Config) error
```

Parses a JSON string into a Go value. Similar to `Unmarshal` but accepts a JSON string instead of bytes. The `target` must be a non-nil pointer.

**Parameters:**
- `jsonStr` - JSON string to parse
- `target` - Pointer to target value (must be non-nil)
- `cfg` - Optional configuration

**Returns:**
- `error` - Parsing error if any

**Example:**
```go
var result map[string]any
err := json.Parse(jsonStr, &result)

// With security configuration
cfg := json.SecurityConfig()
var data map[string]any
err = json.Parse(untrustedInput, &data, cfg)
```

---

### ParseAny

```go
func ParseAny(jsonStr string, cfg ...Config) (any, error)
```

Parses a JSON string and returns the result as `any`. Unlike `Parse`, does not require a target pointer.

**Example:**
```go
result, err := json.ParseAny(`{"name": "Alice", "age": 30}`)
if err != nil {
    log.Fatal(err)
}
m := result.(map[string]any)
fmt.Println(m["name"])
```

---

## File Operations

### LoadFromFile

```go
func LoadFromFile(filePath string, cfg ...Config) (string, error)
```

Loads JSON data from a file.

**Example:**
```go
data, err := json.LoadFromFile("config.json")
```

---

### SaveToFile

```go
func SaveToFile(filePath string, data any, cfg ...Config) error
```

Saves JSON data to a file.

**Example:**
```go
cfg := json.PrettyConfig()
err := json.SaveToFile("output.json", data, cfg)
```

---

### MarshalToFile

```go
func MarshalToFile(filePath string, data any, cfg ...Config) error
```

Marshals data to JSON and writes to a file.

---

### UnmarshalFromFile

```go
func UnmarshalFromFile(filePath string, v any, cfg ...Config) error
```

Reads JSON from a file and unmarshals it into v.

---

### LoadFromReader

```go
func LoadFromReader(reader io.Reader, cfg ...Config) (string, error)
```

Loads JSON data from an `io.Reader`.

**Example:**
```go
file, _ := os.Open("data.json")
defer file.Close()
data, err := json.LoadFromReader(file)
```

---

### SaveToWriter

```go
func SaveToWriter(writer io.Writer, data any, cfg ...Config) error
```

Writes JSON data to an `io.Writer`.

**Example:**
```go
var buf bytes.Buffer
cfg := json.PrettyConfig()
err := json.SaveToWriter(&buf, data, cfg)
```

---

## Iteration Functions

### Foreach

```go
func Foreach(jsonStr string, fn func(key any, item *IterableValue), cfg ...Config)
```

Iterates over JSON arrays or objects (read-only).

> **Deprecated:** `Foreach` drops errors — an invalid document or a closed
> processor silently skips the callback entirely (D-005 decision D2). Use
> `ForeachWithError`, which returns the error. Kept for the lifetime of v1.

**Example:**
```go
// Prefer the error-returning form (exact replacement iterates the root: "."):
err := json.ForeachWithError(data, ".", func(key any, item *json.IterableValue) error {
    name := item.GetString("name")
    fmt.Printf("Key: %v, Name: %s\n", key, name)
    return nil
})
```

---

### ForeachWithPath

```go
func ForeachWithPath(jsonStr, path string, fn func(key any, item *IterableValue), cfg ...Config) error
```

Iterates over a specific path in JSON.

---

### ForeachNested

```go
func ForeachNested(jsonStr string, fn func(key any, item *IterableValue), cfg ...Config)
```

Recursively iterates through all nested levels.

> **Deprecated:** drops errors the same way `Foreach` does (D-005 decision D2).
> Use `ForeachNestedWithError`. Kept for the lifetime of v1.

---

### ForeachWithPathAndControl

```go
func ForeachWithPathAndControl(jsonStr, path string, fn func(key any, value any) IteratorControl, cfg ...Config) error
```

Iterates with early termination support using the exported `IteratorControl` type.

**Note:** For user-facing iteration with control, prefer `ForeachWithPath` with `IterableValue`:

```go
json.ForeachWithPath(data, "users", func(key any, item *json.IterableValue) {
    name := item.GetString("name")
    if name == "target" {
        // Use item.Break() for early termination in file operations
        return
    }
})
```

---

### ForeachReturn

```go
func ForeachReturn(jsonStr string, fn func(key any, item *IterableValue), cfg ...Config) (string, error)
```

Iterates over JSON and returns the JSON string. Both the package-level function and the Processor method deep-copy the data, run the callback, and re-encode the result, so the returned string reflects mutations made in the callback through the container returned by `item.GetData()` (e.g., mutating the underlying map/slice). `*IterableValue` itself exposes no setter methods.

**Note:** For explicit path-based modifications, use `json.Set()`.

---

### ForeachWithPathAndIterator

```go
func ForeachWithPathAndIterator(jsonStr, path string, fn func(key any, item *IterableValue, currentPath string) IteratorControl, cfg ...Config) error
```

Iterates over a specific path with automatic path tracking and flow control. The callback receives the current full path as the third argument.

**Example:**
```go
err := json.ForeachWithPathAndIterator(data, "users", func(key any, item *json.IterableValue, currentPath string) json.IteratorControl {
    fmt.Printf("Path: %s, Name: %s\n", currentPath, item.GetString("name"))
    return json.IteratorNormal
})
```

---

### ForeachWithError

```go
func ForeachWithError(jsonStr, path string, fn func(key any, item *IterableValue) error, cfg ...Config) error
```

Iterates over a specific path with an error-returning callback. Stops on first error returned.

---

### ForeachNestedWithError

```go
func ForeachNestedWithError(jsonStr string, fn func(key any, item *IterableValue) error, cfg ...Config) error
```

Recursively iterates through all nested levels with an error-returning callback. Stops on first error returned.

---

### ForeachFile

```go
func ForeachFile(filePath string, fn func(key any, item *IterableValue) error, cfg ...Config) error
```

Iterates over a JSON file. The whole file is loaded and parsed first (memory is O(file size)); for memory-bounded streaming of JSONL input use `StreamJSONLFile`. Supports early termination via `item.Break()`.

**Example:**
```go
err := json.ForeachFile("data.json", func(key any, item *json.IterableValue) error {
    fmt.Println(item.GetString("name"))
    return nil
})
```

---

### ForeachFileWithPath

```go
func ForeachFileWithPath(filePath, path string, fn func(key any, item *IterableValue) error, cfg ...Config) error
```

Iterates over a specific path within a JSON file.

---

### ForeachFileChunked

```go
func ForeachFileChunked(filePath string, chunkSize int, fn func(chunk []*IterableValue) error, cfg ...Config) error
```

Iterates over a JSON file in chunks; chunks are delivered after the whole file is loaded and parsed (memory is O(file size)); for memory-bounded streaming use `StreamJSONLChunked`.

---

### ForeachFileNested

```go
func ForeachFileNested(filePath string, fn func(key any, item *IterableValue) error, cfg ...Config) error
```

Recursively iterates through all nested levels in a JSON file.

---

### ForeachJSONL

```go
func ForeachJSONL(reader io.Reader, fn func(lineNum int, item *IterableValue) error, cfg ...Config) error
```

Iterates over JSONL/NDJSON lines from a reader.

---

## Streaming Processing

### StreamJSONL

```go
func StreamJSONL(reader io.Reader, fn func(lineNum int, item *IterableValue) error, cfg ...Config) error
```

Streams JSONL data from a reader with callback processing.

---

### StreamJSONLParallel

```go
func StreamJSONLParallel(reader io.Reader, workers int, fn func(lineNum int, item *IterableValue) error, cfg ...Config) error
```

Streams JSONL data with parallel worker processing.

---

### StreamJSONLParallelWithContext

```go
func StreamJSONLParallelWithContext(ctx context.Context, reader io.Reader, workers int, fn func(lineNum int, item *IterableValue) error, cfg ...Config) error
```

Streams JSONL data with parallel workers and context cancellation support.

---

### StreamJSONLChunked

```go
func StreamJSONLChunked(reader io.Reader, chunkSize int, fn func(chunk []*IterableValue) error, cfg ...Config) error
```

Streams JSONL data in chunks for batch processing.

---

### MapJSONL

```go
func MapJSONL(reader io.Reader, fn func(lineNum int, item *IterableValue) (any, error), cfg ...Config) ([]any, error)
```

Maps each JSONL line to a transformed value.

---

### ReduceJSONL

```go
func ReduceJSONL(reader io.Reader, initial any, fn func(acc any, item *IterableValue) any, cfg ...Config) (any, error)
```

Reduces JSONL lines to a single accumulated value.

---

### FilterJSONL

```go
func FilterJSONL(reader io.Reader, predicate func(item *IterableValue) bool, cfg ...Config) ([]*IterableValue, error)
```

Filters JSONL lines by a predicate function.

---

### StreamJSONLFile

```go
func StreamJSONLFile(filename string, fn func(lineNum int, item *IterableValue) error, cfg ...Config) error
```

Streams JSONL data from a file with callback processing. Convenience wrapper for file-based JSONL processing.

**Example:**
```go
err := json.StreamJSONLFile("data.jsonl", func(lineNum int, item *json.IterableValue) error {
    fmt.Printf("Line %d: %v\n", lineNum, item.GetData())
    return nil
})
```

---

### CollectJSONL

```go
func CollectJSONL(reader io.Reader, cfg ...Config) ([]*IterableValue, error)
```

Collects all JSONL items from a reader into a slice. Useful for loading JSONL data into memory.

**Example:**
```go
items, err := json.CollectJSONL(strings.NewReader(jsonlData))
for _, item := range items {
    fmt.Println(item.GetString("name"))
}
```

---

### FirstJSONL

```go
func FirstJSONL(reader io.Reader, predicate func(item *IterableValue) bool, cfg ...Config) (*IterableValue, bool, error)
```

Returns the first JSONL item that matches the predicate. Stops scanning after the first match for efficiency.

**Example:**
```go
item, found, err := json.FirstJSONL(reader, func(item *json.IterableValue) bool {
    return item.GetInt("score") >= 90
})
if found {
    fmt.Printf("First match: %s\n", item.GetString("name"))
}
```

---

## JSONL Support

### ParseJSONL

```go
func ParseJSONL(data []byte, cfg ...Config) ([]any, error)
```

Parses JSONL data into a slice of values. Supports Config for processing options.

**Example:**
```go
// Basic usage
data, err := json.ParseJSONL(jsonlBytes)

// With config
cfg := json.DefaultConfig()
cfg.JSONLSkipComments = true
cfg.JSONLContinueOnErr = true
data, err := json.ParseJSONL(jsonlBytes, cfg)
```

---

### ToJSONL

```go
func ToJSONL(data []any, cfg ...Config) ([]byte, error)
func ToJSONLString(data []any, cfg ...Config) (string, error)
```

Converts a slice to JSONL format. Supports Config for encoding options.

**Example:**
```go
// Basic usage
jsonl, err := json.ToJSONL(data)

// With config
cfg := json.DefaultConfig()
cfg.EscapeHTML = true
jsonl, err := json.ToJSONL(data, cfg)
```

---

### NewJSONLWriter

```go
func NewJSONLWriter(writer io.Writer, cfg ...Config) *JSONLWriter
```

Creates a new JSONL writer with optional Config — only `Config.EscapeHTML` is honored.

---

### StreamLinesInto[T]

```go
func StreamLinesInto[T any](reader io.Reader, fn func(lineNum int, data T) error, cfg ...Config) ([]T, error)
```

Type-safe streaming of JSONL lines.

---

## Validation Functions

### ValidateSchema

```go
func ValidateSchema(jsonStr string, schema *Schema, cfg ...Config) ([]ValidationError, error)
```

Validates JSON data against a schema.

**Example:**
```go
// Build schemas with NewSchema: length/range constraints
// (MinLength, MaxLength, Minimum, Maximum, MinItems, MaxItems)
// are enforced only via its pointer fields — a struct literal cannot set the
// internal "constraint present" flags, so such constraints are silently
// skipped. MultipleOf is the exception: it is a plain non-pointer field,
// enabled by any non-zero value (MultipleOf > 0) — no pointer construction
// needed.
minLen, zero := 1, 0.0
schema := json.NewSchema(json.SchemaConfig{
    Type: "object",
    Properties: map[string]*json.Schema{
        "name": json.NewSchema(json.SchemaConfig{Type: "string", MinLength: &minLen}),
        "age":  json.NewSchema(json.SchemaConfig{Type: "number", Minimum: &zero}),
    },
    Required: []string{"name"},
})
errors, err := json.ValidateSchema(data, schema)
```

---

### IsValidJSON

> **Note:** This function is unexported (`isValidJSON`). Use `json.Valid([]byte(jsonStr))` for public access.

---

### IsValidPath

> **Note:** No `isValidPath` helper exists in this package (not even unexported). JSON paths are validated internally before every operation — syntax, bracket and array-index checks, depth/length limits, and security checks (null bytes, traversal, zero-width characters). There is no public path-validation function; `json.Valid([]byte(jsonStr))` validates JSON *content*, not paths.

---

### ValidWithConfig

```go
func ValidWithConfig(jsonStr string, cfg ...Config) (bool, error)
```

Validates a JSON string with optional configuration. Unlike `Valid` which accepts `[]byte`, this accepts a string and supports Config options.

**Example:**
```go
ok, err := json.ValidWithConfig(jsonStr, json.SecurityConfig())
if err != nil || !ok {
    log.Fatal("invalid JSON")
}
```

---

## Type Conversion

> **Note:** The following type conversion functions are unexported (internal). Use `GetTyped[T]`, `GetString`, `GetInt`, etc. for type-safe access, or use `AccessResult` methods (`AsInt()`, `AsFloat64()`, `AsBool()`, `AsString()`) from `SafeGet`.

| Function | Signature | Description |
|----------|-----------|-------------|
| `convertToInt` | `(value any) (int, bool)` | Safely converts any value to int |
| `convertToFloat64` | `(value any) (float64, bool)` | Safely converts any value to float64 |
| `convertToBool` | `(value any) (bool, bool)` | Safely converts any value to bool |
| `convertToString` | `(value any) string` | Converts any value to string |

---

## Data Utilities

### DeepCopy

> **Note:** This function is unexported (`deepCopy`). Deep copy is performed internally when needed by operations like `Get` (cache isolation) and the `Foreach*` family.

---

### CompareJSON

```go
func CompareJSON(json1, json2 string, cfg ...Config) (bool, error)
```

Compares two JSON strings for semantic equality.

---

### MergeJSON

```go
func MergeJSON(json1, json2 string, cfg ...Config) (string, error)
```

Merges two JSON objects using deep merge strategy. Uses `Config.MergeMode` to specify the merge strategy (defaults to MergeUnion).

**Merge Modes (Config.MergeMode):**
| Mode | Description |
|------|-------------|
| `MergeUnion` | Combines all keys from both objects (default) |
| `MergeIntersection` | Only keys present in both objects |
| `MergeDifference` | Only keys in first object but not in second |

**Example:**
```go
// Union merge (default)
result, err := json.MergeJSON(a, b)

// Intersection merge (only common keys)
cfg := json.DefaultConfig()
cfg.MergeMode = json.MergeIntersection
result, err := json.MergeJSON(a, b, cfg)

// Difference merge (keys only in first)
cfg.MergeMode = json.MergeDifference
result, err := json.MergeJSON(a, b, cfg)
```

---

### MergeMany

```go
func MergeMany(jsons []string, cfg ...Config) (string, error)
```

Merges multiple JSON objects using the unified Config pattern. Uses `Config.MergeMode` to determine the merge strategy (default: `MergeUnion`).

**Example:**
```go
// Union merge (default)
result, err := json.MergeMany([]string{config1, config2, config3})

// Intersection merge with config
cfg := json.DefaultConfig()
cfg.MergeMode = json.MergeIntersection
result, err := json.MergeMany([]string{config1, config2, config3}, cfg)
```

---

## Batch Processing

### ProcessBatch

```go
func ProcessBatch(operations []BatchOperation, cfg ...Config) ([]BatchResult, error)
```

Processes multiple operations in a single batch call. Each `BatchOperation` specifies a `Type` ("get", "set", "delete", "validate"), `JSONStr`, `Path`, `Value`, and `ID`.

**Example:**
```go
ops := []json.BatchOperation{
    {Type: "get", JSONStr: data, Path: "user.name", ID: "1"},
    {Type: "set", JSONStr: data, Path: "user.age", Value: 30, ID: "2"},
}
results, err := json.ProcessBatch(ops)
```

---

## Cache Operations

### WarmupCache

```go
func WarmupCache(jsonStr string, paths []string, cfg ...Config) (*WarmupResult, error)
```

Pre-warms the cache for specified paths. Returns a `WarmupResult` with success/failure counts.

**Example:**
```go
result, err := json.WarmupCache(data, []string{"user.name", "user.age"})
fmt.Printf("Warmed %d/%d paths\n", result.Successful, result.TotalPaths)
```

---

### ClearCache

```go
func ClearCache()
```

Clears the global processor cache.

---

### GetStats

```go
func GetStats() Stats
```

Returns processor performance statistics (package-level).

---

### GetHealthStatus

```go
func GetHealthStatus() HealthStatus
```

Returns processor health status (package-level).

---

## Security Functions

### RegisterDangerousPattern

```go
func RegisterDangerousPattern(pattern DangerousPattern)
```

Registers a custom dangerous pattern for security validation.

---

### UnregisterDangerousPattern

```go
func UnregisterDangerousPattern(pattern string)
```

Unregisters a dangerous pattern by its pattern string.

---

### ListDangerousPatterns

```go
func ListDangerousPatterns() []DangerousPattern
```

Returns all registered dangerous patterns.

---

## Utility Functions

### SafeError

```go
func SafeError(err error) string
```

Returns a best-effort client-safe message by stripping the wrapper's Op/Path context. Paths may still appear in inner error messages (e.g. validation errors embed the offending path) — redact explicitly if required.

---

### RedactedPath

```go
func RedactedPath(path string) string
```

Returns a redacted version of a path for safe logging.

---

## Hook Constructors

### LoggingHook

```go
func LoggingHook(logger interface{ Info(msg string, args ...any) }) Hook
```

Creates a hook that logs operations.

---

### TimingHook

```go
func TimingHook(recorder interface{ Record(op string, duration time.Duration) }) Hook
```

Creates a hook that records operation timing.

---

### ValidationHook

```go
func ValidationHook(validator func(jsonStr, path string) error) Hook
```

Creates a hook that validates data before operations.

---

### ErrorHook

```go
func ErrorHook(handler func(ctx HookContext, err error) error) Hook
```

Creates a hook that handles operation errors.

---

## Processor Management

### SetGlobalProcessor

```go
func SetGlobalProcessor(processor *Processor)
```

Sets the global processor used by package-level functions.

---

### ShutdownGlobalProcessor

```go
func ShutdownGlobalProcessor()
```

Shuts down and releases the global processor.

---

## Configuration

### DefaultConfig

```go
func DefaultConfig() Config
```

Returns the default configuration with balanced settings.

**Default Values:**
| Setting | Value |
|---------|-------|
| MaxJSONSize | 100MB |
| MaxPathDepth | 50 |
| MaxNestingDepthSecurity | 200 |
| MaxSecurityValidationSize | 10MB |
| MaxObjectKeys | 100,000 |
| MaxArrayElements | 100,000 |
| MaxConcurrency | 50 |
| MaxBatchSize | 2,000 |
| MaxCacheSize | 128 |
| ParallelThreshold | 10 |
| EnableValidation | true |
| EnableCache | true |
| CacheResults | true |
| CacheTTL | 5 minutes |
| StrictMode | false |
| ValidateInput | true |
| ValidateFilePath | true |
| CreatePaths | true |
| IncludeNulls | true |
| EscapeHTML | true |
| ValidateUTF8 | true |
| MaxDepth | 100 |
| FloatPrecision | -1 (auto) |
| FullSecurityScan | false |

---

### SecurityConfig

```go
func SecurityConfig() Config
```

Returns a configuration with enhanced security settings for untrusted input.

**Security Values:**
| Setting | Value |
|---------|-------|
| MaxJSONSize | 10MB |
| MaxPathDepth | 30 |
| MaxNestingDepthSecurity | 30 |
| MaxSecurityValidationSize | 10MB |
| MaxObjectKeys | 5,000 |
| MaxArrayElements | 5,000 |
| MaxCacheSize | 256 |
| CacheTTL | 3 minutes |
| EnableCache | true |
| EnableValidation | true |
| FullSecurityScan | true |
| StrictMode | true |

---

### PrettyConfig

```go
func PrettyConfig() Config
```

Returns a configuration for pretty-printed JSON output.

---

### Config Methods

```go
func (c *Config) Clone() *Config
func (c *Config) Validate() error
func (c *Config) ValidateWithWarnings() []ConfigWarning
func (c *Config) AddHook(hook Hook)
func (c *Config) AddValidator(validator Validator) // Deprecated: never executed; use AddHook
func (c *Config) AddDangerousPattern(pattern DangerousPattern)
```

### Schema Helpers

```go
func DefaultSchema() *Schema
func DefaultSchemaConfig() SchemaConfig
func NewSchema(cfg SchemaConfig) *Schema
// NewSchemaWithConfig(cfg) — deprecated alias of NewSchema (D-005); kept for v1
```

---

## Processor Type

### New

```go
func New(cfg ...Config) (*Processor, error)
```

Creates a new Processor with optional configuration.

**Example:**
```go
processor, err := json.New()
if err != nil {
    log.Fatal(err)
}
defer processor.Close()

// Or with configuration
processor, err = json.New(json.DefaultConfig())
if err != nil {
    log.Fatal(err)
}
defer processor.Close()
```

---

### Processor Methods

```go
// Core operations
func (p *Processor) Get(jsonStr, path string, cfg ...Config) (any, error)
func (p *Processor) GetString(jsonStr, path string, defaultValue ...string) string
func (p *Processor) GetInt(jsonStr, path string, defaultValue ...int) int
func (p *Processor) GetFloat(jsonStr, path string, defaultValue ...float64) float64
func (p *Processor) GetBool(jsonStr, path string, defaultValue ...bool) bool
func (p *Processor) GetArray(jsonStr, path string, defaultValue ...[]any) []any
func (p *Processor) GetObject(jsonStr, path string, defaultValue ...map[string]any) map[string]any
func (p *Processor) GetMultiple(jsonStr string, paths []string, cfg ...Config) (map[string]any, error)
func (p *Processor) GetWithContext(ctx context.Context, jsonStr, path string, cfg ...Config) (any, error)
func (p *Processor) SafeGet(jsonStr, path string, cfg ...Config) AccessResult
func (p *Processor) GetFromParsed(parsed *ParsedJSON, path string, cfg ...Config) (any, error)
func (p *Processor) GetCompiled(jsonStr string, cp *CompiledPath) (any, error)
func (p *Processor) CompilePath(path string) (*CompiledPath, error)
func (p *Processor) PreParse(jsonStr string, cfg ...Config) (*ParsedJSON, error)

func (p *Processor) Set(jsonStr, path string, value any, cfg ...Config) (string, error)
func (p *Processor) SetMultiple(jsonStr string, updates map[string]any, cfg ...Config) (string, error)
func (p *Processor) SetCreate(jsonStr, path string, value any, cfg ...Config) (string, error)
func (p *Processor) SetMultipleCreate(jsonStr string, updates map[string]any, cfg ...Config) (string, error)
func (p *Processor) SetFromParsed(parsed *ParsedJSON, path string, value any, cfg ...Config) (*ParsedJSON, error)

func (p *Processor) Delete(jsonStr, path string, cfg ...Config) (string, error)
func (p *Processor) DeleteClean(jsonStr, path string, cfg ...Config) (string, error)

// Encoding/Decoding
func (p *Processor) Marshal(value any, cfg ...Config) ([]byte, error)
func (p *Processor) Unmarshal(data []byte, value any, cfg ...Config) error
func (p *Processor) MarshalIndent(value any, prefix, indent string, cfg ...Config) ([]byte, error)
// Encode is the canonical string-returning encoder (D-005 D1);
// EncodeWithConfig is a deprecated alias kept for v1.
func (p *Processor) Encode(value any, cfg ...Config) (string, error)
func (p *Processor) EncodeWithConfig(value any, cfg ...Config) (string, error)
func (p *Processor) EncodeBatch(pairs map[string]any, cfg ...Config) (string, error)
func (p *Processor) EncodeFields(value any, fields []string, cfg ...Config) (string, error)
func (p *Processor) EncodeStream(values any, cfg ...Config) (string, error)
func (p *Processor) EncodePretty(value any, cfg ...Config) (string, error)
func (p *Processor) Prettify(jsonStr string, cfg ...Config) (string, error)
func (p *Processor) Compact(jsonStr string, cfg ...Config) (string, error)
func (p *Processor) CompactString(jsonStr string, cfg ...Config) (string, error) // D-005: mirror of json.CompactString
func (p *Processor) CompactBuffer(dst *bytes.Buffer, src []byte, cfg ...Config) error
func (p *Processor) Indent(dst *bytes.Buffer, src []byte, prefix, indent string, cfg ...Config) error
func (p *Processor) HTMLEscape(dst *bytes.Buffer, src []byte, cfg ...Config)

// Parsing/Validation
func (p *Processor) Parse(jsonStr string, target any, cfg ...Config) error
func (p *Processor) ParseAny(jsonStr string, cfg ...Config) (any, error)
func (p *Processor) Valid(jsonStr string, cfg ...Config) (bool, error)
func (p *Processor) ValidBytes(data []byte) bool
func (p *Processor) ValidateSchema(jsonStr string, schema *Schema, cfg ...Config) ([]ValidationError, error)

// Comparison & Merge
func (p *Processor) CompareJSON(json1, json2 string, cfg ...Config) (bool, error)
func (p *Processor) MergeJSON(json1, json2 string, cfg ...Config) (string, error)
func (p *Processor) MergeMany(jsons []string, cfg ...Config) (string, error)

// File operations
func (p *Processor) LoadFromFile(filePath string, cfg ...Config) (string, error)
func (p *Processor) LoadFromReader(reader io.Reader, cfg ...Config) (string, error)
func (p *Processor) SaveToFile(filePath string, data any, cfg ...Config) error
func (p *Processor) SaveToWriter(writer io.Writer, data any, cfg ...Config) error
func (p *Processor) MarshalToFile(path string, data any, cfg ...Config) error
func (p *Processor) UnmarshalFromFile(path string, v any, cfg ...Config) error

// Batch/Cache
func (p *Processor) ProcessBatch(operations []BatchOperation, cfg ...Config) ([]BatchResult, error)
func (p *Processor) ClearCache()
func (p *Processor) WarmupCache(jsonStr string, paths []string, cfg ...Config) (*WarmupResult, error)
func (p *Processor) GetStats() Stats
func (p *Processor) GetHealthStatus() HealthStatus

// Iteration
// Deprecated (D-005 D2, error-dropping void forms): Foreach, ForeachNested —
// use ForeachWithError / ForeachNestedWithError. Kept for the lifetime of v1.
func (p *Processor) Foreach(jsonStr string, fn func(key any, item *IterableValue), cfg ...Config)
func (p *Processor) ForeachWithPath(jsonStr, path string, fn func(key any, item *IterableValue), cfg ...Config) error
func (p *Processor) ForeachWithPathAndControl(jsonStr, path string, fn func(key any, value any) IteratorControl, cfg ...Config) error
func (p *Processor) ForeachWithPathAndIterator(jsonStr, path string, fn func(key any, item *IterableValue, currentPath string) IteratorControl, cfg ...Config) error
func (p *Processor) ForeachReturn(jsonStr string, fn func(key any, item *IterableValue), cfg ...Config) (string, error)
func (p *Processor) ForeachNested(jsonStr string, fn func(key any, item *IterableValue), cfg ...Config)
func (p *Processor) ForeachWithError(jsonStr, path string, fn func(key any, item *IterableValue) error, cfg ...Config) error
func (p *Processor) ForeachNestedWithError(jsonStr string, fn func(key any, item *IterableValue) error, cfg ...Config) error

// File Iteration
func (p *Processor) ForeachFile(filePath string, fn func(key any, item *IterableValue) error, cfg ...Config) error
func (p *Processor) ForeachFileWithPath(filePath, path string, fn func(key any, item *IterableValue) error, cfg ...Config) error
func (p *Processor) ForeachFileChunked(filePath string, chunkSize int, fn func(chunk []*IterableValue) error, cfg ...Config) error
func (p *Processor) ForeachFileNested(filePath string, fn func(key any, item *IterableValue) error, cfg ...Config) error

// JSONL Streaming (D-005 Phase 2: the Processor methods accept the same
// optional trailing Config as their package-level counterparts — per-call
// override of the processor's JSONL settings)
func (p *Processor) StreamJSONL(reader io.Reader, fn func(lineNum int, item *IterableValue) error, cfg ...Config) error
func (p *Processor) StreamJSONLParallel(reader io.Reader, workers int, fn func(lineNum int, item *IterableValue) error, cfg ...Config) error
func (p *Processor) StreamJSONLParallelWithContext(ctx context.Context, reader io.Reader, workers int, fn func(lineNum int, item *IterableValue) error, cfg ...Config) error
func (p *Processor) StreamJSONLChunked(reader io.Reader, chunkSize int, fn func(chunk []*IterableValue) error, cfg ...Config) error
func (p *Processor) ForeachJSONL(reader io.Reader, fn func(lineNum int, item *IterableValue) error, cfg ...Config) error
func (p *Processor) MapJSONL(reader io.Reader, fn func(lineNum int, item *IterableValue) (any, error), cfg ...Config) ([]any, error)
func (p *Processor) ReduceJSONL(reader io.Reader, initial any, fn func(acc any, item *IterableValue) any, cfg ...Config) (any, error)
func (p *Processor) FilterJSONL(reader io.Reader, predicate func(item *IterableValue) bool, cfg ...Config) ([]*IterableValue, error)
func (p *Processor) StreamJSONLFile(filename string, fn func(lineNum int, item *IterableValue) error, cfg ...Config) error
func (p *Processor) CollectJSONL(reader io.Reader, cfg ...Config) ([]*IterableValue, error)
func (p *Processor) FirstJSONL(reader io.Reader, predicate func(item *IterableValue) bool, cfg ...Config) (*IterableValue, bool, error)
func (p *Processor) ParseJSONL(data []byte, cfg ...Config) ([]any, error)
func (p *Processor) ToJSONL(data []any, cfg ...Config) ([]byte, error)
func (p *Processor) ToJSONLString(data []any, cfg ...Config) (string, error)

// Lifecycle
func (p *Processor) Close() error
func (p *Processor) IsClosed() bool
func (p *Processor) GetConfig() Config
func (p *Processor) SetLogger(logger *slog.Logger)
func (p *Processor) AddHook(hook Hook)
```

---

## Error Types

### Standard Errors (encoding/json compatible)

```go
type SyntaxError struct {
    Offset int64
}
type UnmarshalTypeError struct { ... }
type InvalidUnmarshalError struct { ... }
type UnsupportedTypeError struct { ... }
type UnsupportedValueError struct { ... }
type MarshalerError struct { ... }
```

---

### Extended Error Types

```go
type JsonsError struct {
    Op      string  // Operation name
    Path    string  // JSON path
    Message string  // Error message
    Err     error   // Underlying error
}
```

---

### Error Variables

```go
var ErrInvalidJSON       = errors.New("invalid JSON format")
var ErrPathNotFound      = errors.New("path not found")
var ErrTypeMismatch      = errors.New("type mismatch")
var ErrSizeLimit         = errors.New("size limit exceeded")
var ErrDepthLimit        = errors.New("depth limit exceeded")
var ErrSecurityViolation = errors.New("security violation detected")
var ErrInvalidPath       = errors.New("invalid path format")
var ErrProcessorClosed   = errors.New("processor is closed")
var ErrConcurrencyLimit  = errors.New("concurrency limit exceeded")  // Returned by governed operations (Get/Set/Delete/...) when MaxConcurrency is reached. Increase MaxConcurrency in Config for high-concurrency scenarios.
var ErrOperationTimeout  = errors.New("operation timeout")             // Deprecated: Reserved for future use; not currently returned by any operation.
var ErrUnsupportedPath   = errors.New("unsupported path operation")    // Deprecated: not currently returned by any operation (D-002 audit); unsupported segments surface as ErrInvalidPath or ErrTypeMismatch. Reserved for future use.
var ErrResourceExhausted = errors.New("system resources exhausted")    // Deprecated: Reserved for future use; not currently returned by any operation.
```

---

### Error Handling Example

```go
result, err := json.Get(data, "user.name")
if err != nil {
    // Check specific error types
    var jsonsErr *json.JsonsError
    if errors.As(err, &jsonsErr) {
        fmt.Printf("Operation: %s, Path: %s\n", jsonsErr.Op, jsonsErr.Path)
    }

    // Check error codes
    if errors.Is(err, json.ErrPathNotFound) {
        // Handle missing path
    }
}
```

---

## IterableValue Type

```go
type IterableValue struct {
    // Contains methods for safe data access during iteration
}

func (iv *IterableValue) GetData() any
func (iv *IterableValue) Break() error

func (iv *IterableValue) Get(path string) any
func (iv *IterableValue) GetString(key string) string
func (iv *IterableValue) GetStringWithDefault(key string, defaultValue string) string
func (iv *IterableValue) GetInt(key string) int
func (iv *IterableValue) GetIntWithDefault(key string, defaultValue int) int
func (iv *IterableValue) GetFloat64(key string) float64
func (iv *IterableValue) GetFloat64WithDefault(key string, defaultValue float64) float64
func (iv *IterableValue) GetBool(key string) bool
func (iv *IterableValue) GetBoolWithDefault(key string, defaultValue bool) bool
func (iv *IterableValue) GetArray(key string) []any
func (iv *IterableValue) GetObject(key string) map[string]any
func (iv *IterableValue) GetWithDefault(key string, defaultValue any) any
func (iv *IterableValue) Exists(key string) bool
func (iv *IterableValue) IsNull(key string) bool
func (iv *IterableValue) IsNullData() bool
func (iv *IterableValue) IsEmpty(key string) bool
func (iv *IterableValue) IsEmptyData() bool
func (iv *IterableValue) ForeachNested(path string, fn func(key any, item *IterableValue))
func (iv *IterableValue) Release()
```

---

## AccessResult Type

```go
type AccessResult struct {
    Value  any
    Exists bool
    Type   string  // Runtime type info (for debugging)
}

func (r AccessResult) Ok() bool
func (r AccessResult) Unwrap() any
func (r AccessResult) UnwrapOr(defaultValue any) any
func (r AccessResult) AsString() (string, error)
func (r AccessResult) AsStringConverted() (string, error)
func (r AccessResult) AsInt() (int, error)
func (r AccessResult) AsFloat64() (float64, error)
func (r AccessResult) AsBool() (bool, error)
```

---

## Result[T] Type

```go
type Result[T any] struct {
    Value  T
    Exists bool
    Error  error
}

func (r Result[T]) Ok() bool
func (r Result[T]) Unwrap() T
func (r Result[T]) UnwrapOr(defaultValue T) T
```

---

## Schema Type

```go
type Schema struct {
    Type                 string             `json:"type,omitempty"`
    Properties           map[string]*Schema `json:"properties,omitempty"`
    Items                *Schema            `json:"items,omitempty"`
    Required             []string           `json:"required,omitempty"`
    MinLength            int                `json:"minLength,omitempty"`
    MaxLength            int                `json:"maxLength,omitempty"`
    Minimum              float64            `json:"minimum,omitempty"`
    Maximum              float64            `json:"maximum,omitempty"`
    ExclusiveMinimum     bool               `json:"exclusiveMinimum,omitempty"`
    ExclusiveMaximum     bool               `json:"exclusiveMaximum,omitempty"`
    MultipleOf           float64            `json:"multipleOf,omitempty"`
    Pattern              string             `json:"pattern,omitempty"`
    Format               string             `json:"format,omitempty"`
    AdditionalProperties bool               `json:"additionalProperties,omitempty"`
    MinItems             int                `json:"minItems,omitempty"`
    MaxItems             int                `json:"maxItems,omitempty"`
    UniqueItems          bool               `json:"uniqueItems,omitempty"`
    Enum                 []any              `json:"enum,omitempty"`
    Const                any                `json:"const,omitempty"`
    Title                string             `json:"title,omitempty"`
    Description          string             `json:"description,omitempty"`
    Default              any                `json:"default,omitempty"`
    Examples             []any              `json:"examples,omitempty"`
}

type ValidationError struct {
    Path    string `json:"path"`
    Message string `json:"message"`
}
```

---

## Iterator Control

The `IteratorControl` type provides iteration flow control with three constants:

```go
type IteratorControl int

const (
    IteratorNormal   IteratorControl = iota  // Default/normal
    IteratorContinue                          // Continue iteration
    IteratorBreak                             // Stop iteration
)
```

All three constants are exported. For user-facing iteration with early termination, use `ForeachFile`, `ForeachFileChunked`, `ForeachFileNested`, or `ForeachFileWithPath` with `IterableValue.Break()`:

```go
processor, err := json.New()
if err != nil {
    log.Fatal(err)
}
defer processor.Close()

err = processor.ForeachFile("data.json", func(key any, item *json.IterableValue) error {
    if item.GetInt("id") == targetId {
        return item.Break() // stop iteration
    }
    return nil // continue
})
```

**Note:** The `IteratorControl` constants (`IteratorNormal`, `IteratorContinue`, `IteratorBreak`) are exported but primarily used internally. For most use cases, prefer the `IterableValue.Break()` method for iteration control.

---

## Additional Types

### Iterator

```go
type Iterator struct { /* unexported fields */ }

func NewIterator(data any, cfg ...Config) *Iterator
func (it *Iterator) HasNext() bool
func (it *Iterator) Next() (any, bool)
func (it *Iterator) Reset()
func (it *Iterator) ResetWith(data any)
```

A general-purpose iterator for traversing arrays and objects.

**Example:**
```go
data, _ := json.ParseAny(`{"name": "Alice", "age": 30}`)
iter := json.NewIterator(data)
for iter.HasNext() {
    value, ok := iter.Next()
    if !ok {
        break
    }
    fmt.Println(value)
}
```

---

### BatchIterator

```go
type BatchIterator struct { /* unexported fields */ }

func NewBatchIterator(data []any, cfg ...Config) *BatchIterator
func (it *BatchIterator) NextBatch() []any
func (it *BatchIterator) HasNext() bool
func (it *BatchIterator) Reset()
func (it *BatchIterator) TotalBatches() int
func (it *BatchIterator) CurrentIndex() int
func (it *BatchIterator) Remaining() int
```

Processes arrays in batches for efficient bulk operations. The batch size is `Config.MaxBatchSize` (default: 2,000); when no `Config` is provided, `DefaultConfig()` supplies it.

**Example:**
```go
data := make([]any, 1000)
iter := json.NewBatchIterator(data)
for iter.HasNext() {
    batch := iter.NextBatch()
    // Process batch...
}
```

---

### NDJSONProcessor

```go
type NDJSONProcessor struct { /* unexported fields */ }

func NewNDJSONProcessor(cfg ...Config) *NDJSONProcessor
func (p *NDJSONProcessor) ProcessFile(filename string, fn func(lineNum int, obj map[string]any) error) error
func (p *NDJSONProcessor) ProcessReader(reader io.Reader, fn func(lineNum int, obj map[string]any) error) error
```

A processor for NDJSON/JSONL files with line-by-line processing.

> **Deprecated** (D-005): duplicates the `StreamJSONL` family with a
> `map[string]any` callback. Use `Processor.StreamJSONL` / `StreamJSONLFile`
> (same limits, same JSONL config knobs, `*IterableValue` callback with typed
> access; `item.GetData()` yields the decoded map). Kept for the lifetime of v1.

---

### ParsedJSON

```go
type ParsedJSON struct { /* unexported fields */ }

func (p *ParsedJSON) Data() any
func (p *ParsedJSON) Release()
```

A pre-parsed JSON document for reuse across multiple operations. Create with `Processor.PreParse`, then use `Processor.GetFromParsed` and `Processor.SetFromParsed` for efficient repeated access. Call `Release()` when done to drop the reference so the parsed tree can be garbage-collected.

**PreParse Example:**
```go
// Pre-parse once, query many times
parsed, err := processor.PreParse(largeJSON)
if err != nil {
    log.Fatal(err)
}

// Multiple fast queries on the same parsed document
name, _ := processor.GetFromParsed(parsed, "user.name")
age, _ := processor.GetFromParsed(parsed, "user.age")
```

---

### CompiledPath

```go
type CompiledPath = internal.CompiledPath
```

A pre-parsed path for fast repeated operations. Create with `Processor.CompilePath`.

---

### JSONLWriter

```go
type JSONLWriter struct { /* unexported fields */ }

func NewJSONLWriter(writer io.Writer, cfg ...Config) *JSONLWriter
func (w *JSONLWriter) Write(data any) error
func (w *JSONLWriter) WriteAll(data []any) error
func (w *JSONLWriter) WriteRaw(line []byte) error
func (w *JSONLWriter) Err() error
func (w *JSONLWriter) Stats() JSONLStats
```

A streaming JSONL writer; the optional Config controls `EscapeHTML` only — Pretty/Indent/CustomEscapes/FloatPrecision/MaxJSONSize are ignored. For full config-driven encoding per line, build lines with `Encode` and write them via `WriteRaw`.

---

### JSONLStats

```go
type JSONLStats struct {
    LinesProcessed int64
    BytesWritten   int64
}
```

Statistics from JSONLWriter operations.

---

### BatchOperation

```go
type BatchOperation struct {
    Type    string  // "get", "set", "delete", or "validate"
    JSONStr string
    Path    string
    Value   any
    ID      string
}
```

---

### BatchResult

```go
type BatchResult struct {
    ID     string
    Result any
    Error  error
}
```

---

### Stats

```go
type Stats struct {
    CacheSize        int64
    CacheMemory      int64
    MaxCacheSize     int
    HitCount         int64
    MissCount        int64
    HitRatio         float64
    CacheTTL         time.Duration
    CacheEnabled     bool
    IsClosed         bool
    MemoryEfficiency float64
    OperationCount   int64
    ErrorCount       int64
}
```

---

### HealthStatus

```go
type HealthStatus struct {
    Timestamp time.Time
    Healthy   bool
    Checks    map[string]CheckResult
}
```

---

### CheckResult

```go
type CheckResult struct {
    Healthy bool
    Message string
}
```

---

### WarmupResult

```go
type WarmupResult struct {
    TotalPaths  int
    Successful  int
    Failed      int
    SuccessRate float64
    FailedPaths []string
}
```

---

### ConfigWarning

```go
type ConfigWarning struct {
    Field    string
    OldValue any
    NewValue any
    Reason   string
}
```

Returned by `Config.ValidateWithWarnings()` when configuration values are auto-corrected.

---

### SchemaConfig

```go
type SchemaConfig struct {
    Type                 string
    Properties           map[string]*Schema
    Items                *Schema
    Required             []string
    MinLength            *int
    MaxLength            *int
    Minimum              *float64
    Maximum              *float64
    Pattern              string
    Format               string
    AdditionalProperties *bool
    MinItems             *int
    MaxItems             *int
    UniqueItems          bool
    Enum                 []any
    Const                any
    MultipleOf           *float64
    ExclusiveMinimum     *bool
    ExclusiveMaximum     *bool
    Title                string
    Description          string
    Default              any
    Examples             []any
}
```

Configuration for creating schemas with optional pointer-typed fields. Unlike `Schema`, numeric and boolean constraints use pointer types (`*int`, `*float64`, `*bool`) so that zero values are distinguishable from "not set". Use `DefaultSchemaConfig()` for defaults and `NewSchema(cfg)` to create (`NewSchemaWithConfig` is a deprecated alias, D-005).

---

### PathSegment

```go
type PathSegment = internal.PathSegment
```

A segment of a parsed JSON path. Used with custom `PathParser` implementations.

---

### StreamIterator

```go
type StreamIterator struct { /* unexported fields */ }

func NewStreamIterator(reader io.Reader, cfg ...Config) *StreamIterator
func (it *StreamIterator) Next() bool
func (it *StreamIterator) Value() any
func (it *StreamIterator) Index() int
func (it *StreamIterator) Err() error
```

Provides memory-efficient iteration over large JSON arrays. Processes elements one at a time without loading the entire array into memory. The optional `cfg` parameter allows customization (e.g., `BufferSize`).

**Example:**
```go
reader := strings.NewReader(`[1, 2, 3, 4, 5]`)
iter := json.NewStreamIterator(reader)
for iter.Next() {
    fmt.Printf("Index %d: %v\n", iter.Index(), iter.Value())
}
if err := iter.Err(); err != nil {
    log.Fatal(err)
}
```

---

### ParallelIterator

```go
type ParallelIterator struct { /* unexported fields */ }

func NewParallelIterator(data []any, cfg ...Config) *ParallelIterator
func (pi *ParallelIterator) ForEach(fn func(index int, value any) error) error
func (pi *ParallelIterator) ForEachWithContext(ctx context.Context, fn func(index int, value any) error) error
func (pi *ParallelIterator) ForEachBatch(batchSize int, fn func(int, []any) error) error
func (pi *ParallelIterator) ForEachBatchWithContext(ctx context.Context, batchSize int, fn func(int, []any) error) error
func (pi *ParallelIterator) Map(transform func(int, any) (any, error)) ([]any, error)
func (pi *ParallelIterator) Filter(predicate func(int, any) bool) []any
func (pi *ParallelIterator) Close()
```

Processes arrays in parallel using worker goroutines. The optional `cfg` parameter controls worker count via `Config.MaxConcurrency`. Remember to call `Close()` to release worker goroutines when done.

**Example:**
```go
data := []any{1, 2, 3, 4, 5}
iter := json.NewParallelIterator(data)
err := iter.ForEach(func(idx int, val any) error {
    fmt.Printf("Worker processing index %d: %v\n", idx, val)
    return nil
})
```

---

### StreamObjectIterator

```go
type StreamObjectIterator struct { /* unexported fields */ }

func NewStreamObjectIterator(reader io.Reader, cfg ...Config) *StreamObjectIterator
func (soi *StreamObjectIterator) Next() bool
func (soi *StreamObjectIterator) Key() string
func (soi *StreamObjectIterator) Value() any
func (soi *StreamObjectIterator) Err() error
```

Provides memory-efficient iteration over a JSON object's key/value pairs.
Processes one entry at a time without loading the entire object into memory.
The optional `cfg` parameter allows customization (e.g., `BufferSize`).

**Example:**
```go
reader := strings.NewReader(`{"name": "Alice", "age": 30, "active": true}`)
iter := json.NewStreamObjectIterator(reader)
for iter.Next() {
    fmt.Printf("Key %q = %v\n", iter.Key(), iter.Value())
}
if err := iter.Err(); err != nil {
    log.Fatal(err)
}
```

---

## Extension Interfaces

These exported interfaces and types support customizing encoding, validation,
hooking, and path parsing. The live extension points are `Hooks` (registered
via `Config.AddHook` / `Processor.AddHook`), `AdditionalDangerousPatterns`
(via `Config.AddDangerousPattern`), and `CustomPathParser`.

> **Deprecated (not wired):** `CustomEncoder`, `CustomTypeEncoders`
> (`TypeEncoder`), and `CustomValidators` (`Validator`, `AddValidator`) are
> never consulted by the encoding or operation pipelines — setting them has
> no effect. They are retained only for v1 API compatibility. Use `Hook`
> (`Before`) for pre-operation checks, and implement `json.Marshaler` /
> `encoding.TextMarshaler` on your types for custom encoding.

```go
// CustomEncoder — Deprecated: not wired; never invoked by the encoding
// pipeline. Retained for v1 compatibility.
type CustomEncoder interface {
    Encode(value any) (string, error)
}

// TypeEncoder — Deprecated: not wired; Config.CustomTypeEncoders is never
// consulted. Retained for v1 compatibility.
type TypeEncoder interface {
    Encode(v reflect.Value) (string, error)
}

// Validator — Deprecated: not wired; validators registered via
// Config.CustomValidators / AddValidator are never executed.
// Use Hook (Before) for pre-operation checks.
type Validator interface {
    Validate(jsonStr string) error
}

// PathParser parses a path string into segments.
// Implement to provide custom path syntax support.
type PathParser interface {
    ParsePath(path string) ([]PathSegment, error)
}
```

### PatternLevel

`PatternLevel` represents the severity of a `DangerousPattern`. Used by
`RegisterDangerousPattern` and `Config.AddDangerousPattern`.

```go
type PatternLevel int

const (
    PatternLevelCritical PatternLevel = iota  // Always blocks the operation
    PatternLevelWarning                       // Blocks in strict mode; warns otherwise
    PatternLevelInfo                          // Logs but never blocks
)
```

### Hook, HookContext, and HookFunc

```go
// HookContext provides context for operation hooks.
type HookContext struct {
    Operation string    // "get", "set", "delete", "get_multiple", "set_multiple"
    JSONStr   string    // Input JSON (may contain sensitive data — do not log)
    Path      string
    Value     any
    Config    *Config
    StartTime time.Time
}

// Hook intercepts operations before/after execution.
type Hook interface {
    Before(ctx HookContext) error
    After(ctx HookContext, result any, err error) (any, error)
}

// HookFunc adapts ordinary functions into a Hook (either BeforeFn or AfterFn
// may be nil). Useful for hooks that only need one side.
type HookFunc struct {
    BeforeFn func(ctx HookContext) error
    AfterFn  func(ctx HookContext, result any, err error) (any, error)
}
```

Register hooks via `Config.AddHook(hook)` or `Processor.AddHook(hook)`.
Convenience constructors (`LoggingHook`, `TimingHook`, `ValidationHook`,
`ErrorHook`) are documented under [Hook Constructors](#hook-constructors).

> **Security:** `HookContext.JSONStr` may contain sensitive data. Use only
> `Operation` and `Path` for logging — never log the raw `JSONStr`.

---

## Path Expression Syntax

| Syntax | Description | Example | Result |
|--------|-------------|---------|--------|
| `.` | Property access | `user.name` | Get user's name |
| `[n]` | Array index | `items[0]` | First element |
| `[-n]` | Negative index | `items[-1]` | Last element |
| `[start:end]` | Array slice | `items[1:3]` | Elements at index 1-2 |
| `[::step]` | Step slice | `numbers[::2]` | Every other element |
| `{field}` | Batch extract | `users{name}` | All user names |
| `{flat:field}` | Flatten extract | `users{flat:skills}` | All skills flattened |
| `[+]` | Array append (Set only) | `items[+]` | Append value to array |
| `*` / `[*]` | Wildcard | `items[*]` | All array elements / all object keys |
| `\.` `\\` `\[` `\]` `\{` `\}` | Escaped literal characters | `user\.name` | Key literally named `user.name` |
| `/a/b` | JSON Pointer style | `/user/name` | Same as `user.name` |

> **Note:** To access the entire JSON document, pass an empty path `""`
> (e.g., `json.Get(data, "")` returns the full document). The `$` symbol is
> **not** supported as a root reference and will return a "path not found" error.

---

**For more information, see:**
- [Compatibility Guide](./COMPATIBILITY.md)
- [Security Guide](./SECURITY.md)
- [Quick Reference](./QUICK_REFERENCE.md)
