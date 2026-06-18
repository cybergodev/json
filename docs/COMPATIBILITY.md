# encoding/json Compatibility Guide

This document outlines the complete compatibility between `github.com/cybergodev/json` and Go's standard `encoding/json` package.

## Drop-in Replacement

Our library is designed as a **drop-in replacement** for `encoding/json`. Simply change your import statement:

```go
// Before
import "encoding/json"

// After
import "github.com/cybergodev/json"
```

**Most code requires no changes.** See notes below for edge cases.

### Extended Signatures (Backward-Compatible)

The following functions accept an optional `cfg ...Config` trailing parameter in addition to the standard `encoding/json` signatures. Calls without the extra argument work identically:

- `Compact(dst *bytes.Buffer, src []byte, cfg ...Config) error`
- `Indent(dst *bytes.Buffer, src []byte, prefix, indent string, cfg ...Config) error`
- `HTMLEscape(dst *bytes.Buffer, src []byte, cfg ...Config)`
- `NewEncoder(w io.Writer, cfg ...Config) *Encoder`
- `NewDecoder(r io.Reader, cfg ...Config) *Decoder`

These are backward-compatible: existing call sites compile without changes. However, **function pointer assignments** (e.g., `var fn = json.Compact`) will differ because the signature includes the optional parameter. If you assign these functions to variables, use an explicit adapter.

## Fully Compatible Functions

| Function                                                             | Status | Notes                                 |
|----------------------------------------------------------------------|--------|---------------------------------------|
| `Marshal(v any) ([]byte, error)`                                     | ✅      | Identical behavior and output         |
| `Unmarshal(data []byte, v any) error`                                | ✅      | Identical behavior and error handling |
| `MarshalIndent(v any, prefix, indent string) ([]byte, error)`        | ✅      | Same formatting rules                 |
| `Valid(data []byte) bool`                                            | ✅      | Same validation logic                 |
| `Compact(dst *bytes.Buffer, src []byte, cfg ...Config) error`                       | ✅      | Identical whitespace removal (extended with optional `Config`) |
| `Indent(dst *bytes.Buffer, src []byte, prefix, indent string, cfg ...Config) error` | ✅      | Same indentation behavior (extended with optional `Config`)    |
| `HTMLEscape(dst *bytes.Buffer, src []byte, cfg ...Config)`                          | ✅      | Same HTML escaping rules (extended with optional `Config`)     |

## Fully Compatible Types

### Streaming Types
| Type/Method                                   | Status | Notes                      |
|-----------------------------------------------|--------|----------------------------|
| `Encoder`                                     | ✅      | Complete implementation    |
| `Decoder`                                     | ✅      | Complete implementation    |
| `NewEncoder(w io.Writer, cfg ...Config) *Encoder` | ✅ | Extended with optional `Config` |
| `NewDecoder(r io.Reader, cfg ...Config) *Decoder` | ✅  | Extended with optional `Config` |
| `(*Encoder).Encode(v any) error`              | ✅      | Same encoding behavior     |
| `(*Encoder).SetEscapeHTML(on bool)`           | ✅      | Same HTML escaping control |
| `(*Encoder).SetIndent(prefix, indent string)` | ✅      | Same indentation control   |
| `(*Decoder).Decode(v any) error`              | ✅      | Same decoding behavior     |
| `(*Decoder).UseNumber()`                      | ✅      | Same number handling       |
| `(*Decoder).DisallowUnknownFields()`          | ✅      | Fully functional; implemented in this package's own streaming Decoder (not a wrapper around `encoding/json`) |
| `(*Decoder).More() bool`                      | ✅      | Same stream state checking |
| `(*Decoder).Token() (Token, error)`           | ✅      | Same token parsing         |
| `(*Decoder).Buffered() io.Reader`             | ✅      | Same buffer access         |
| `(*Decoder).InputOffset() int64`              | ✅      | Same offset tracking       |

### Token Types
| Type                                | Status | Notes                      |
|-------------------------------------|--------|----------------------------|
| `Token`                             | ✅      | Same interface             |
| `Delim`                             | ✅      | Same delimiter handling    |
| `Number`                            | ✅      | Same number representation |
| `Number.String() string`            | ✅      | Same string conversion     |
| `Number.Float64() (float64, error)` | ✅      | Same float conversion      |
| `Number.Int64() (int64, error)`     | ✅      | Same int conversion        |

## Fully Compatible Error Types

| Error Type              | Status | Notes                                   |
|-------------------------|--------|-----------------------------------------|
| `SyntaxError`           | ✅      | Same structure and offset tracking (message details may vary) |
| `UnmarshalTypeError`    | ✅      | Same type mismatch reporting            |
| `InvalidUnmarshalError` | ✅      | Same invalid target detection           |
| `UnsupportedTypeError`  | ✅      | Same unsupported type handling          |
| `UnsupportedValueError` | ✅      | Same unsupported value handling         |
| `MarshalerError`        | ✅      | Same marshaler error wrapping           |

### Extended Error Types

In addition to standard library errors, the library provides:

| Error Type       | Description                                |
|------------------|--------------------------------------------|
| `JsonsError`     | Custom error with operation context (`Op`, `Path`, `Message`, `Err`) |
| `ValidationError`| Schema validation error (`Path`, `Message`) |

**Extended Error Variables:**
| Variable | Description |
|----------|-------------|
| `ErrSizeLimit` | JSON size exceeds configured limit |
| `ErrDepthLimit` | Nesting depth exceeds configured limit |
| `ErrSecurityViolation` | Potentially dangerous content detected |
| `ErrProcessorClosed` | Operation on closed processor |
| `ErrConcurrencyLimit` | Concurrent operation count exceeds limit |
| `ErrOperationTimeout` | Operation exceeded timeout duration |

## Fully Compatible Interfaces

| Interface         | Status | Notes                                |
|-------------------|--------|--------------------------------------|
| `Marshaler`       | ✅      | Same `MarshalJSON() ([]byte, error)` |
| `Unmarshaler`     | ✅      | Same `UnmarshalJSON([]byte) error`   |
| `TextMarshaler`   | ✅      | Same `MarshalText() ([]byte, error)` |
| `TextUnmarshaler` | ✅      | Same `UnmarshalText([]byte) error`   |

> **Note:** These interfaces are method-compatible with their `encoding/json` and `encoding` counterparts. Types implementing `encoding/json.Marshaler` will satisfy this package's `Marshaler`, and vice versa, because the method signatures are identical. They are not type aliases in the Go `type X = Y` sense, but are structurally equivalent interfaces.


## Migration Examples

### Basic Usage
```go
// Works exactly the same as encoding/json
data := map[string]any{"name": "John", "age": 30}
jsonBytes, err := json.Marshal(data)
if err != nil {
    log.Fatal(err)
}

var result map[string]any
err = json.Unmarshal(jsonBytes, &result)
if err != nil {
    log.Fatal(err)
}
```

### Streaming Usage
```go
// Works exactly the same as encoding/json
var buf bytes.Buffer
encoder := json.NewEncoder(&buf)
encoder.SetIndent("", "  ")
encoder.Encode(data)

decoder := json.NewDecoder(&buf)
decoder.UseNumber()
decoder.Decode(&result)
```

### Error Handling
```go
// The top-level Unmarshal delegates to encoding/json.Unmarshal,
// so the returned error is *encoding/json.SyntaxError, NOT this
// package's *json.SyntaxError. Use errors.As for portable matching:
err := json.Unmarshal([]byte(`invalid`), &result)
if err != nil {
    // Safe: works regardless of which SyntaxError type is returned
    var syntaxErr *json.SyntaxError
    if errors.As(err, &syntaxErr) {
        fmt.Printf("Syntax error at offset %d: %v", syntaxErr.Offset, syntaxErr)
    }
}
```

> **Warning:** A direct type assertion like `err.(*json.SyntaxError)` will fail when using this package's top-level `Unmarshal`, because the actual error returned is `*encoding/json.SyntaxError` (a different Go type). Always use `errors.As` for portable error matching. This does not apply to errors returned from `Processor` methods, which use this package's own error types.

## Bonus Features

Beyond standard compatibility, our library also provides:

- **Advanced Path Operations**: `json.Get()`, `json.Set()`, `json.Delete()`
- **Type-Safe Generics**: `json.GetTyped[T]()`, `json.SafeGet()`
- **Performance Optimizations**: Caching, memory pools, string interning
- **Thread Safety**: Concurrent-safe operations with atomic operations
- **Rich Query Syntax**: Dot notation, array slicing, batch extraction
- **JSONL Support**: `json.ParseJSONL()`, `json.ToJSONL()`, `json.StreamLinesInto[T]()`
- **Advanced Encoding**: `json.EncodeStream()`, `json.EncodeBatch()`, `json.EncodeFields()`
- **File Operations**: `json.LoadFromFile()`, `json.SaveToFile()`, `json.MarshalToFile()`
- **Schema Validation**: `json.ValidateSchema()` with comprehensive schema support
- **Data Utilities**: `json.CompareJSON()`, `json.MergeJSON()` (note: `deepCopy` is unexported; deep copy is performed internally by operations like `Set` and `MergeJSON`)

## Compatibility Guarantee

We guarantee:

1. **API Compatibility**: All standard `encoding/json` public APIs are present and behave equivalently. Some functions accept an optional `cfg ...Config` trailing parameter (see Extended Signatures above).
2. **Behavioral Compatibility**: Semantically equivalent output for same input (JSON object key ordering may differ, which is compliant with JSON specification)
3. **Error Compatibility**: Same error types; messages are semantically equivalent (minor formatting details may differ). Note that top-level `Unmarshal`/`Marshal` return `*encoding/json.SyntaxError` (not this package's type); use `errors.As` for portable matching.
4. **Performance Compatibility**: Same or better performance
5. **Version Compatibility**: Requires Go 1.25.0+ (as specified in `go.mod`)

**Important Notes**:
- **Key Ordering**: JSON object key ordering is not guaranteed by the JSON specification (RFC 7159). While our library may produce different key ordering than `encoding/json` for map serialization, the output is semantically equivalent and fully compliant with JSON standards.
- **Performance**: Our library includes additional features like caching and memory pooling, which may result in slightly different performance characteristics compared to the standard library.
- **Error Messages**: While error types are identical, some error message details may vary slightly while maintaining the same semantic meaning.

## Troubleshooting

### Common Differences (All Semantically Equivalent)

1. **Map Key Ordering**: Our library may serialize map keys in a different order than `encoding/json`. This is compliant with JSON specification and doesn't affect functionality.

2. **Whitespace Handling**: Minor differences in whitespace formatting may occur, but the JSON structure remains identical.

3. **Number Precision**: Our library includes number handling that may preserve precision differently in edge cases.

### Verification Steps

If you suspect compatibility issues:

1. **Semantic Comparison**: Parse both outputs with `json.Unmarshal` and compare the resulting Go values using `reflect.DeepEqual`
2. **Functional Testing**: Verify that your application logic works the same with both libraries
3. **Performance Testing**: Measure performance differences in your specific use case

## Support

We are committed to maintaining **semantic** compatibility with `encoding/json`.
