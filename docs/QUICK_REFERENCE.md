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

// Get string
str, err := json.GetString(data, "user.name")

// Get integer
num, err := json.GetInt(data, "user.age")

// Get boolean
flag, err := json.GetBool(data, "user.active")

// Get float
price, err := json.GetFloat64(data, "product.price")

// Get array
arr, err := json.GetArray(data, "items")

// Get object
obj, err := json.GetObject(data, "user.profile")
```

### Retrieval with Default Values

```go
// String (default: "Anonymous")
name := json.GetStringWithDefault(data, "user.name", "Anonymous")

// Integer (default: 0)
age := json.GetIntWithDefault(data, "user.age", 0)

// Boolean (default: false)
active := json.GetBoolWithDefault(data, "user.active", false)
```

### Type-Safe Retrieval (Generics)

```go
// Get string
name, err := json.GetTyped[string](data, "user.name")

// Get integer slice
numbers, err := json.GetTyped[[]int](data, "scores")

// Get custom type
users, err := json.GetTyped[[]User](data, "users")
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

// Auto-create paths
result, err := json.SetWithAdd(data, "user.profile.city", "NYC")
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
result, err := json.SetMultipleWithAdd(data, updates)
```

---

## 🗑️ Data Deletion (Delete)

```go
// Delete field
result, err := json.Delete(data, "user.temp")

// Delete and cleanup null values
result, err := json.DeleteWithCleanNull(data, "user.temp")
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
modifiedJson, err := json.ForeachReturn(data, func(key any, item *json.IterableValue) {
    // Modify data
    if item.GetString("status") == "inactive" {
        item.Set("status", "active")
    }
})
```

### Nested Iteration (Read-only)

```go
// Recursively iterate through all nested levels
json.ForeachNested(data, func(key any, item *json.IterableValue, path string) {
    fmt.Printf("Path: %s, Value: %v\n", path, item.GetAny(""))
})
```

### Iteration with Flow Control

```go
// Iterate with early termination support
json.ForeachWithPathAndControl(data, "users", func(key any, value any) json.IteratorControl {
    // Process each item
    if shouldStop {
        return json.IteratorBreak  // Stop iteration
    }
    return json.IteratorContinue  // Continue to next item
})
```

### Iteration with Path Information

```go
// Iterate with detailed path tracking
json.ForeachWithPathAndIterator(data, "data.users", func(key any, item *json.IterableValue, currentPath string) json.IteratorControl {
    name := item.GetString("name")
    fmt.Printf("User at %s: %s\n", currentPath, name)
    return json.IteratorContinue
})
```

### Streaming Iteration

```go
// Stream array elements without loading entire JSON
processor := json.NewStreamingProcessor(reader, 64*1024)
err := processor.StreamArray(func(index int, item any) bool {
    fmt.Printf("Item %d: %v\n", index, item)
    return true  // continue
})
```

### Complete Foreach Functions List

| Function | Description | Use Case |
|----------|-------------|----------|
| `Foreach(data, callback)` | Basic iteration | Simple read-only traversal |
| `ForeachNested(data, callback)` | Recursive iteration | All nested levels |
| `ForeachWithPath(data, path, callback)` | Path-specific iteration | Specific JSON subset |
| `ForeachWithPathAndControl(data, path, callback)` | With flow control | Early termination |
| `ForeachWithPathAndIterator(data, path, callback)` | With path info | Path tracking |
| `ForeachReturn(data, callback)` | Modify and return | Data transformation |

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

// Load from Reader (requires processor)
processor := json.New()
defer processor.Close()

file, _ := os.Open("data.json")
defer file.Close()
data, err := processor.LoadFromReader(file)
```

### Write Files

```go
// Save to file (pretty format)
err := json.SaveToFile("output.json", data, true)

// Save to file (compact format)
err := json.SaveToFile("output.json", data, false)

// Save to Writer (requires processor)
processor := json.New()
defer processor.Close()

var buffer bytes.Buffer
err = processor.SaveToWriter(&buffer, data, true)
```

---

## ⚙️ Configuration

### Create Processor

```go
// Use default configuration
processor := json.New()
defer processor.Close()

// Use custom configuration
config := &json.Config{
    EnableCache:               true,
    MaxCacheSize:              128,                 // Default cache entry count
    CacheTTL:                  5 * time.Minute,     // Default cache TTL
    MaxJSONSize:               100 * 1024 * 1024,   // 100MB (default)
    MaxPathDepth:              50,                  // Default path depth
    MaxConcurrency:            50,                  // Default max concurrency
    ParallelThreshold:         10,                  // Default parallel threshold
    MaxBatchSize:              2000,                // Default batch size
    MaxNestingDepthSecurity:   200,                 // Default nesting depth
    MaxSecurityValidationSize: 10 * 1024 * 1024,    // 10MB validation size
    MaxObjectKeys:             100000,              // Default max object keys
    MaxArrayElements:          100000,              // Default max array elements
    EnableValidation:          true,
    ValidateInput:             true,
    ValidateFilePath:          true,
}
processor := json.New(config)
defer processor.Close()

// Use predefined configurations
processor := json.New(json.HighSecurityConfig())  // For untrusted input
processor := json.New(json.LargeDataConfig())     // For large JSON files
processor := json.New(json.DefaultConfig())       // Same as json.New()
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

processor := json.New(json.DefaultConfig())
errors, err := processor.ValidateSchema(data, schema)

// Check validation errors
for _, verr := range errors {
    fmt.Printf("Error at %s: %s\n", verr.Path, verr.Message)
}
```

### Basic Validation

```go
// Validate JSON
if json.Valid([]byte(jsonStr)) {
    fmt.Println("Valid JSON")
}

// Quick validation check
if json.IsValidJSON(jsonStr) {
    fmt.Println("Valid JSON")
}

// Validate path expression
if json.IsValidPath("user.profile.name") {
    fmt.Println("Valid path")
}
```

---

## ❌ Error Handling

### Recommended Patterns

```go
// 1. Check errors
result, err := json.GetString(data, "user.name")
if err != nil {
    log.Printf("Get failed: %v", err)
    return err
}

// 2. Use default values
name := json.GetStringWithDefault(data, "user.name", "Anonymous")

// 3. Type checking
if errors.Is(err, json.ErrTypeMismatch) {
    // Handle type mismatch
}

// 4. Check specific error types
var jsonsErr *json.JsonsError
if errors.As(err, &jsonsErr) {
    fmt.Printf("Operation: %s, Path: %s\n", jsonsErr.Op, jsonsErr.Path)
}

// 5. Type-safe result handling
result := json.TypeSafeResult[string]{}
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
- ✅ Use HighSecurityConfig() for untrusted input

### Common Pitfalls
- ⚠️ Note the difference between null and missing fields
- ⚠️ Array indices start at 0
- ⚠️ Negative indices start at -1 (last element)
- ⚠️ ForeachWithPath is read-only, cannot modify data
- ⚠️ Set operations return original data on failure

### Type Conversion Utilities

```go
// Safe type conversion
intVal, ok := json.ConvertToInt(value)
floatVal, ok := json.ConvertToFloat64(value)
boolVal, ok := json.ConvertToBool(value)
strVal := json.ConvertToString(value)

// Generic type conversion
result, ok := json.UnifiedTypeConversion[int](value)
result, err := json.TypeSafeConvert[string](value)

// Fast conversion (for hot paths)
strVal, ok := json.FastToString(value)
intVal, ok := json.FastToInt(value)
floatVal, ok := json.FastToFloat64(value)
boolVal, ok := json.FastToBool(value)
```

---

## 🔄 JSONL (JSON Lines) Support

```go
// Parse JSONL data
jsonlData := `{"name":"Alice","age":25}
{"name":"Bob","age":30}
{"name":"Carol","age":28}`
results, err := json.ParseJSONL([]byte(jsonlData))

// Stream processing for large files
processor := json.NewJSONLProcessor(reader)
err := processor.StreamLines(func(lineNum int, data any) bool {
    fmt.Printf("Line %d: %v\n", lineNum, data)
    return true  // continue
})

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
writer.Write(map[string]any{"event": "login", "user": "alice"})

// Convert slice to JSONL
data := []any{
    map[string]any{"id": 1, "name": "Alice"},
    map[string]any{"id": 2, "name": "Bob"},
}
jsonlBytes, err := json.ToJSONL(data)
```

---

## 🌊 Streaming Processing

```go
// Create streaming processor for large JSON arrays
processor := json.NewStreamingProcessor(reader, 64*1024) // 64KB buffer

// Stream array elements
err := processor.StreamArray(func(index int, item any) bool {
    fmt.Printf("Item %d: %v\n", index, item)
    return true  // continue
})

// Stream object key-value pairs
err := processor.StreamObject(func(key string, value any) bool {
    fmt.Printf("Key: %s, Value: %v\n", key, value)
    return true
})

// Chunked processing for batch operations
err := processor.StreamArrayChunked(100, func(chunk []any) error {
    // Process 100 items at a time
    return nil
})

// Stream transformations
filtered, err := json.StreamArrayFilter(reader, func(item any) bool {
    return item.(map[string]any)["active"] == true
})

transformed, err := json.StreamArrayMap(reader, func(item any) any {
    item.(map[string]any)["processed"] = true
    return item
})

// Pagination support
page, err := json.StreamArraySkip(reader, 10)  // Skip first 10
page, err := json.StreamArrayTake(reader, 10)  // Take first 10
```

---

**Quick start, efficient development!** 🚀

