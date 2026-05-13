# Quick Reference Guide

> Quick reference guide for cybergodev/json library - Common features at a glance

---

## 📦 Installation

```bash
go get github.com/cybergodev/json
```

---

## 🚀 Basic Usage

### Import Library

```go
import "github.com/cybergodev/json"
```

---

## 📖 Data Retrieval (Get)

### Basic Type Retrieval

```go
// Get any type
value, err := json.Get(data, "path")

// Get string (with default value)
str := json.GetString(data, "user.name", "unknown")

// Get integer (with default value)
num := json.GetInt(data, "user.age", 0)

// Get boolean (with default value)
flag := json.GetBool(data, "user.active", false)

// Get float (with default value)
price := json.GetFloat(data, "product.price", 0.0)

// Get array (with default value)
arr := json.GetArray(data, "items", nil)

// Get object
obj := json.GetObject(data, "user.profile")
```

### Retrieval with Default Values

```go
// Recommended: Use GetTyped[T] for type-safe defaults
name := json.GetTyped[string](data, "user.name", "Anonymous")
age := json.GetTyped[int](data, "user.age", 0)
active := json.GetTyped[bool](data, "user.active", false)
price := json.GetTyped[float64](data, "product.price", 0.0)
tags := json.GetTyped[[]any](data, "user.tags", []any{})
settings := json.GetTyped[map[string]any](data, "settings", map[string]any{})
```

### Type-Safe Retrieval (Generics)

```go
// Get string (with optional default)
name := json.GetTyped[string](data, "user.name")

// Get integer slice
numbers := json.GetTyped[[]int](data, "scores")

// Get custom type (with default)
users := json.GetTyped[[]User](data, "users", []User{})
```

### Batch Retrieval

```go
paths := []string{"user.name", "user.age", "user.email"}
results, err := json.GetMultiple(data, paths)

// Access results
name := results["user.name"]
age := results["user.age"]
```

---

## ✏️ Data Modification (Set)

### Basic Setting

```go
// Set single value
result, err := json.Set(data, "user.name", "Alice")

// Auto-create paths using Config
cfg := json.DefaultConfig()
cfg.CreatePaths = true
result, err := json.Set(data, "user.profile.city", "NYC", cfg)
```

### Batch Setting

```go
updates := map[string]any{
    "user.name": "Bob",
    "user.age":  30,
    "user.active": true,
}
result, err := json.SetMultiple(data, updates)

// Batch setting with auto-create paths
cfg := json.DefaultConfig()
cfg.CreatePaths = true
result, err := json.SetMultiple(data, updates, cfg)
```

---

## 🗑️ Data Deletion (Delete)

```go
// Delete field
result, err := json.Delete(data, "user.temp")

// Delete and cleanup null values
cfg := json.DefaultConfig()
cfg.CleanupNulls = true
result, err := json.Delete(data, "user.temp", cfg)
```

---

## 🔄 Data Iteration (Foreach)

### Basic Iteration (Read-only)

```go
json.Foreach(data, func(key any, item *json.IterableValue) {
    name := item.GetString("name")
    age := item.GetInt("age")
    fmt.Printf("Key: %v, Name: %s, Age: %d\n", key, name, age)
})
```

### Path Iteration (Read-only)

```go
json.ForeachWithPath(data, "users", func(key any, user *json.IterableValue) {
    name := user.GetString("name")
    fmt.Printf("User %v: %s\n", key, name)
})
```

### Iterate and Modify

```go
// Note: ForeachReturn iterates for reading. Use json.Set() for modifications.
// Collect paths that need modification during iteration
var pathsToUpdate []string
json.ForeachWithPath(data, "users", func(key any, item *json.IterableValue) {
    if item.GetString("status") == "inactive" {
        pathsToUpdate = append(pathsToUpdate, fmt.Sprintf("users[%d].status", key))
    }
})

// Apply modifications using json.Set
result := data
cfg := json.DefaultConfig()
for _, path := range pathsToUpdate {
    result, _ = json.Set(result, path, "active", cfg)
}
```

### Nested Iteration (Read-only)

```go
// Recursively iterate through all nested levels
json.ForeachNested(data, func(key any, item *json.IterableValue) {
    fmt.Printf("Key: %v, Value: %v\n", key, item.Get(""))
})
```

### Iteration with Flow Control

```go
// For early termination, use file-based iteration with IterableValue.Break()
processor, err := json.New()
if err != nil {
    log.Fatal(err)
}
defer processor.Close()

err = processor.ForeachFile("data.json", func(key any, item *json.IterableValue) error {
    if item.GetInt("id") == targetId {
        return item.Break() // Stop iteration
    }
    return nil // Continue
})
```

### Iteration with Path Tracking

```go
// Use ForeachWithPathAndIterator for automatic path tracking
err := json.ForeachWithPathAndIterator(data, "users", func(key any, item *json.IterableValue, currentPath string) json.IteratorControl {
    name := item.GetString("name")
    fmt.Printf("User at %s: %s\n", currentPath, name)
    return json.IteratorContinue // continue (same as IteratorNormal)
    // return json.IteratorBreak   // stop iteration early
})

// Or use manual path tracking with ForeachWithPath
basePath := "data.users"
json.ForeachWithPath(data, basePath, func(key any, item *json.IterableValue) {
    currentPath := fmt.Sprintf("%s[%v]", basePath, key)
    name := item.GetString("name")
    fmt.Printf("User at %s: %s\n", currentPath, name)
})
```

### Complete Foreach Functions List

| Function | Description | Use Case |
|----------|-------------|----------|
| `Foreach(data, callback)` | Basic iteration | Simple read-only traversal |
| `ForeachNested(data, callback)` | Recursive iteration | All nested levels |
| `ForeachWithPath(data, path, callback)` | Path-specific iteration | Specific JSON subset |
| `ForeachWithPathAndControl(data, path, callback)` | With flow control | Early termination |
| `ForeachWithPathAndIterator(data, path, callback)` | With path tracking | Path-aware iteration |
| `ForeachReturn(data, callback)` | Returns original JSON string | Read-only iteration |
| `ForeachWithError(data, path, callback)` | Error-returning callback | Error-aware iteration |
| `ForeachNestedWithError(data, callback)` | Recursive with errors | Nested error-aware iteration |
| `ForeachFile(path, callback)` | File-based iteration | Large file processing |
| `ForeachFileWithPath(path, jsonPath, callback)` | File + path iteration | File subset processing |
| `ForeachFileChunked(path, chunkSize, callback)` | Chunked file iteration | Memory-efficient file processing |
| `ForeachFileNested(path, callback)` | Nested file iteration | Nested file traversal |
| `ForeachJSONL(reader, callback)` | JSONL stream iteration | JSONL/NDJSON processing |

**Note:** ForeachReturn iterates over data and returns the JSON string. Use `json.Set()` for modifications.

---

## 🎯 Path Expressions

### Basic Syntax

| Syntax         | Description     | Example              | Result              |
|----------------|-----------------|----------------------|---------------------|
| `.`            | Property access | `user.name`          | Get user's name     |
| `[n]`          | Array index     | `users[0]`           | First user          |
| `[-n]`         | Negative index  | `users[-1]`          | Last user           |
| `[start:end]`  | Array slice     | `users[1:3]`         | Users at index 1-2  |
| `[::step]`     | Step slice      | `numbers[::2]`       | Every other element |
| `{field}`      | Batch extract   | `users{name}`        | All user names      |
| `{flat:field}` | Flatten extract | `users{flat:skills}` | All skills (flat)   |

### Path Examples

```go
data := `{
  "users": [
    {"name": "Alice", "skills": ["Go", "Python"]},
    {"name": "Bob", "skills": ["Java", "React"]}
  ]
}`

// Get first user
json.Get(data, "users[0]")
// Result: {"name": "Alice", "skills": ["Go", "Python"]}

// Get last user
json.Get(data, "users[-1]")
// Result: {"name": "Bob", "skills": ["Java", "React"]}

// Get all user names
json.Get(data, "users{name}")
// Result: ["Alice", "Bob"]

// Get all skills (flattened)
json.Get(data, "users{flat:skills}")
// Result: ["Go", "Python", "Java", "React"]
```

---

## 📁 File Operations

### Read Files

```go
// Load from file
data, err := json.LoadFromFile("config.json")

// Load from Reader (package-level function, no processor needed)
file, _ := os.Open("data.json")
defer file.Close()
data, err := json.LoadFromReader(file)

// Or via Processor
processor, err := json.New()
if err != nil {
    log.Fatal(err)
}
defer processor.Close()
data, err = processor.LoadFromReader(file)
```

### Write Files

```go
// Save to file (pretty format)
cfg := json.DefaultConfig()
cfg.Pretty = true
err := json.SaveToFile("output.json", data, cfg)

// Save to file (compact format)
err := json.SaveToFile("output.json", data, json.DefaultConfig())

// Save to Writer (package-level function, no processor needed)
var buffer bytes.Buffer
cfg := json.DefaultConfig()
cfg.Pretty = true
err = json.SaveToWriter(&buffer, data, cfg)

// Or via Processor
err = processor.SaveToWriter(&buffer, data, cfg)
```

---

## ⚙️ Configuration

### Create Processor

```go
// Use default configuration
processor, err := json.New()
if err != nil {
    log.Fatal(err)
}
defer processor.Close()

// Use predefined configurations
processor, err = json.New(json.DefaultConfig())    // Same as json.New()
processor, err = json.New(json.SecurityConfig())   // For untrusted input
processor, err = json.New(json.PrettyConfig())     // For pretty output
```

### Custom Configuration

```go
// Start with defaults and modify as needed
config := json.DefaultConfig()
config.EnableCache = true // Already true by default; shown for clarity
config.MaxCacheSize = 128
config.CacheTTL = 5 * time.Minute
config.MaxJSONSize = 100 * 1024 * 1024   // 100MB
config.MaxPathDepth = 50
config.CreatePaths = true  // For Set operations
config.CleanupNulls = true // For Delete operations
processor, err := json.New(config)
if err != nil {
    log.Fatal(err)
}
defer processor.Close()

// Security configuration for untrusted input
secureCfg := json.SecurityConfig()  // Pre-configured for security
var result map[string]any
err = json.Parse(untrustedInput, &result, secureCfg)

// For operation-specific settings (without creating processor)
cfg := json.DefaultConfig()
cfg.CreatePaths = true
result, err := json.Set(data, "new.nested.path", "value", cfg)
```

### Common Configuration Patterns

```go
// Pattern 1: Auto-create paths for Set operations
cfg := json.DefaultConfig()
cfg.CreatePaths = true
result, err := json.Set(data, "new.path", value, cfg)

// Pattern 2: Cleanup nulls after Delete
cfg := json.DefaultConfig()
cfg.CleanupNulls = true
result, err := json.Delete(data, "path", cfg)

// Pattern 3: Pretty output for encoding
cfg := json.DefaultConfig()
cfg.Pretty = true
cfg.Indent = "  "
result, err := json.Encode(data, cfg)

// Pattern 4: Compact output (no nulls)
cfg := json.DefaultConfig()
cfg.IncludeNulls = false
result, err := json.Encode(data, cfg)
```

### Performance Monitoring

```go
// Get statistics
stats := processor.GetStats()
fmt.Printf("Operations: %d\n", stats.OperationCount)
fmt.Printf("Cache hit ratio: %.2f%%\n", stats.HitRatio*100)

// Get health status
health := processor.GetHealthStatus()
fmt.Printf("Health status: %v\n", health.Healthy)
```

---

## 🛡️ Data Validation

### JSON Schema Validation

```go
schema := &json.Schema{
    Type: "object",
    Properties: map[string]*json.Schema{
        "name": {Type: "string", MinLength: 1, MaxLength: 100},
        "age":  {Type: "number", Minimum: 0, Maximum: 150},
        "email": {Type: "string", Format: "email"},
    },
    Required: []string{"name", "age"},
}

processor, err := json.New(json.DefaultConfig())
if err != nil {
    log.Fatal(err)
}
defer processor.Close()

validationErrors, err := processor.ValidateSchema(data, schema)

// Check validation errors
for _, verr := range validationErrors {
    fmt.Printf("Error at %s: %s\n", verr.Path, verr.Message)
}
```

### Basic Validation

```go
// Validate JSON
if json.Valid([]byte(jsonStr)) {
    fmt.Println("Valid JSON")
}

// Note: isValidJSON and isValidPath are unexported (internal).
// Use json.Valid([]byte(jsonStr)) for public JSON validation.
// Path operations validate paths internally before execution.
```

---

## ❌ Error Handling

### Recommended Patterns

```go
// 1. Use Get for error handling
result, err := json.Get(data, "user.name")
if err != nil {
    log.Printf("Get failed: %v", err)
    return err
}
name := result.(string)

// 2. Use default values (recommended: GetTyped[T])
name := json.GetTyped[string](data, "user.name", "Anonymous")
age := json.GetTyped[int](data, "user.age", 0)

// 3. Type checking
if errors.Is(err, json.ErrTypeMismatch) {
    // Handle type mismatch
}

// 4. Check specific error types
var jsonsErr *json.JsonsError
if errors.As(err, &jsonsErr) {
    fmt.Printf("Operation: %s, Path: %s\n", jsonsErr.Op, jsonsErr.Path)
}

// 5. Type-safe result handling (construct manually with Value and Exists)
result := json.Result[string]{Value: "hello", Exists: true}
if result.Ok() {
    fmt.Println(result.Value)
}
value := result.UnwrapOr("default")
```

### Set Operations Safety Guarantee

```go
// Success: Returns modified data
result, err := json.Set(data, "user.name", "Alice")
if err == nil {
    // result is modified JSON
}

// Failure: Returns original data (data never corrupted)
result, err := json.Set(data, "invalid[path", "value")
if err != nil {
    // result still contains valid original data
}
```

---

## 💡 Tips

### Performance Optimization
- ✅ Use caching for repeated queries (enabled by default)
- ✅ Batch operations are better than multiple single operations
- ✅ Configure size limits appropriately for your use case
- ✅ Use streaming processors for large JSON files
- ✅ Use `SkipValidation` option for trusted input only

### Best Practices
- ✅ Use type-safe GetTyped methods for compile-time checking
- ✅ Use default values for potentially missing fields
- ✅ Enable validation in production (enabled by default)
- ✅ Use defer processor.Close() to release resources
- ✅ Use SecurityConfig() for untrusted input

### Common Pitfalls
- ⚠️ Note the difference between null and missing fields
- ⚠️ Array indices start at 0
- ⚠️ Negative indices start at -1 (last element)
- ⚠️ ForeachWithPath is read-only, cannot modify data
- ⚠️ Set operations return original data on failure

### Type Conversion Utilities

> **Note:** These type conversion functions are unexported (internal). Use `GetTyped[T]`, `GetString`, `GetInt`, etc. for type-safe access.

```go
// Use typed Get functions instead:
str := json.GetString(data, "user.name", "default")
num := json.GetInt(data, "user.age", 0)
flag := json.GetBool(data, "user.active", false)
price := json.GetFloat(data, "product.price", 0.0)

// Or use SafeGet with AccessResult methods:
result := json.SafeGet(data, "user.age")
age, err := result.AsInt()
```

---

## 🔄 JSONL (JSON Lines) Support

```go
// Parse JSONL data
jsonlData := `{"name":"Alice","age":25}
{"name":"Bob","age":30}
{"name":"Carol","age":28}`
results, err := json.ParseJSONL([]byte(jsonlData))

// Type-safe streaming
type User struct {
    Name string `json:"name"`
    Age  int    `json:"age"`
}
users, err := json.StreamLinesInto[User](reader, func(lineNum int, user User) error {
    fmt.Printf("User: %s, Age: %d\n", user.Name, user.Age)
    return nil
})

// Write JSONL output
writer := json.NewJSONLWriter(outputWriter)
err = writer.Write(map[string]any{"event": "login", "user": "alice"})
if err != nil {
    log.Fatal(err)
}

// Convert slice to JSONL
data := []any{
    map[string]any{"id": 1, "name": "Alice"},
    map[string]any{"id": 2, "name": "Bob"},
}
jsonlBytes, err := json.ToJSONL(data)
```

---

**Quick start, efficient development!** 🚀

