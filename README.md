# 🚀 cybergodev/json - High-Performance Go JSON Processing Library

[![Go Version](https://img.shields.io/badge/Go-1.24+-blue.svg)](https://golang.org)
[![pkg.go.dev](https://pkg.go.dev/badge/github.com/cybergodev/json.svg)](https://pkg.go.dev/github.com/cybergodev/json)
[![MIT license](https://img.shields.io/badge/license-MIT-brightgreen.svg)](https://opensource.org/licenses/MIT)
[![Performance](https://img.shields.io/badge/performance-high%20performance-green.svg)](https://github.com/cybergodev/json)
[![Thread Safe](https://img.shields.io/badge/thread%20safe-yes-brightgreen.svg)](https://github.com/cybergodev/json)

> A high-performance, feature-rich Go JSON processing library with 100% `encoding/json` compatibility, providing powerful path operations, type safety, performance optimization, and rich advanced features.

#### **[📖 中文文档](README_zh-CN.md)** - User guide

---

## 🏆 Core Advantages

- **🔄 Full Compatibility** - 100% compatible with standard `encoding/json`, zero learning curve, drop-in replacement
- **🎯 Powerful Paths** - Support for complex path expressions, complete complex data operations in one line
- **🚀 High Performance** - Smart caching, concurrent safety, memory optimization, production-ready performance
- **🛡️ Type Safety** - Generic support, compile-time checking, intelligent type conversion
- **🔧 Feature Rich** - Batch operations, data validation, file operations, performance monitoring
- **🏗️ Production Ready** - Thread-safe, error handling, security configuration, monitoring metrics

### 🎯 Use Cases

- **🌐 API Data Processing** - Fast extraction and transformation of complex response data
- **⚙️ Configuration Management** - Dynamic configuration reading and batch updates
- **📊 Data Analysis** - Statistics and analysis of large amounts of JSON data
- **🔄 Microservice Communication** - Data exchange and format conversion between services
- **📝 Log Processing** - Parsing and analysis of structured logs

---

## 📋 Basic Path Syntax

| Syntax             | Description     | Example              | Result                     |
|--------------------|-----------------|----------------------|----------------------------|
| `.`                | Property access | `user.name`          | Get user's name property   |
| `[n]`              | Array index     | `users[0]`           | Get first user             |
| `[-n]`             | Negative index  | `users[-1]`          | Get last user              |
| `[start:end:step]` | Array slice     | `users[1:3]`         | Get users at index 1-2     |
| `{field}`          | Batch extract   | `users{name}`        | Extract all user names     |
| `{flat:field}`     | Flatten extract | `users{flat:skills}` | Flatten extract all skills |

## 🚀 Quick Start

### Installation

```bash
go get github.com/cybergodev/json
```

### Basic Usage

```go
package main

import (
    "fmt"
    "github.com/cybergodev/json"
)

func main() {
    // 1. Full compatibility with standard library
    data := map[string]any{"name": "Alice", "age": 25}
    jsonBytes, err := json.Marshal(data)

    var result map[string]any
    json.Unmarshal(jsonBytes, &result)

    // 2. Powerful path operations (enhanced features)
    jsonStr := `{"user":{"profile":{"name":"Alice","age":25}}}`

    name, err := json.GetString(jsonStr, "user.profile.name")
    if err != nil {
        fmt.Printf("Error: %v\n", err)
        return
    }
    fmt.Println(name) // "Alice"

    age, err := json.GetInt(jsonStr, "user.profile.age")
    if err != nil {
        fmt.Printf("Error: %v\n", err)
        return
    }
    fmt.Println(age) // 25
}
```

### Path Operations Example

```go
// Complex JSON data
complexData := `{
  "users": [
    {"name": "Alice", "skills": ["Go", "Python"], "active": true},
    {"name": "Bob", "skills": ["Java", "React"], "active": false}
  ]
}`

// Get all usernames
names, err := json.Get(complexData, "users{name}")
if err != nil {
    fmt.Printf("Error: %v\n", err)
    return
}
// Result: ["Alice", "Bob"]

// Get all skills (flattened)
skills, err := json.Get(complexData, "users{flat:skills}")
if err != nil {
    fmt.Printf("Error: %v\n", err)
    return
}
// Result: ["Go", "Python", "Java", "React"]

// Batch get multiple values
paths := []string{"users[0].name", "users[1].name", "users{active}"}
results, err := json.GetMultiple(complexData, paths)
if err != nil {
    fmt.Printf("Error: %v\n", err)
    return
}
```


---

## ⚡ Core Features

### Data Retrieval

```go
// Basic retrieval
json.Get(data, "user.name")          // Get any type
json.GetString(data, "user.name")    // Get string
json.GetInt(data, "user.age")        // Get integer
json.GetFloat64(data, "user.score")  // Get float64
json.GetBool(data, "user.active")    // Get boolean
json.GetArray(data, "user.tags")     // Get array
json.GetObject(data, "user.profile") // Get object

// Type-safe retrieval
json.GetTyped[string](data, "user.name") // Generic type safety
json.GetTyped[[]User](data, "users")     // Custom types

// Retrieval with default values
json.GetWithDefault(data, "user.name", "Anonymous")
json.GetStringWithDefault(data, "user.name", "Anonymous")
json.GetIntWithDefault(data, "user.age", 0)
json.GetFloat64WithDefault(data, "user.score", 0.0)
json.GetBoolWithDefault(data, "user.active", false)
json.GetArrayWithDefault(data, "user.tags", []any{})
json.GetObjectWithDefault(data, "user.profile", map[string]any{})

// Batch retrieval
paths := []string{"user.name", "user.age", "user.email"}
results, err := json.GetMultiple(data, paths)
```

### Data Modification

```go
// Basic setting - returns modified data on success, original data on failure
data := `{"user":{"name":"Bob","age":25}}`
result, err := json.Set(data, "user.name", "Alice")
// result => {"user":{"name":"Alice","age":25}}

// Auto-create paths
data := `{}`
result, err := json.SetWithAdd(data, "user.name", "Alice")
// result => {"user":{"name":"Alice"}}

// Batch setting
updates := map[string]any{
    "user.name": "Bob",
    "user.age":  30,
    "user.active": true,
}
result, err := json.SetMultiple(data, updates)
result, err := json.SetMultipleWithAdd(data, updates) // With auto-create paths
// Same behavior: success = modified data, failure = original data
```

### Data Deletion

```go
json.Delete(data, "user.temp") // Delete field
json.DeleteWithCleanNull(data, "user.temp") // Delete and cleanup nulls
```

### Data Iteration

```go
// Basic iteration - read-only traversal
json.Foreach(data, func (key any, item *json.IterableValue) {
    name := item.GetString("name")
    fmt.Printf("Key: %v, Name: %s\n", key, name)
})

// Advanced iteration variants
json.ForeachNested(data, callback)                            // Recursively iterate all nested levels
json.ForeachWithPath(data, "data.users", callback)            // Iterate specific path
json.ForeachReturn(data, callback)                            // Modify and return modified JSON

// Iterate with control flow - supports early termination
json.ForeachWithPathAndControl(data, "data.users", func(key any, value any) json.IteratorControl {
    // Process each item
    if shouldStop {
        return json.IteratorBreak  // Stop iteration
    }
    return json.IteratorContinue  // Continue to next item
})

// Iterate with path information tracking
json.ForeachWithPathAndIterator(data, "data.users", func(key any, item *json.IterableValue, currentPath string) json.IteratorControl {
    name := item.GetString("name")
    fmt.Printf("User at %s: %s\n", currentPath, name)
    return json.IteratorContinue
})

// Complete Foreach functions list:
// - Foreach(data, callback) - Basic iteration
// - ForeachNested(data, callback) - Recursive iteration
// - ForeachWithPath(data, path, callback) - Path-specific iteration
// - ForeachWithPathAndControl(data, path, callback) - With flow control
// - ForeachWithPathAndIterator(data, path, callback) - With path info
// - ForeachReturn(data, callback) - Modify and return
```

### JSON Encoding & Formatting

```go
// Standard encoding (100% compatible with encoding/json)
bytes, err := json.Marshal(data)
err = json.Unmarshal(bytes, &target)
bytes, err := json.MarshalIndent(data, "", "  ")

// Advanced encoding with configuration
config := &json.EncodeConfig{
    Pretty:       true,
    SortKeys:     true,
    EscapeHTML:   false,
    MaxDepth:     10,  // Optional: maximum encoding depth (overriding default of 100)
}
jsonStr, err := json.Encode(data, config)           // Encode with custom config (config is optional, uses defaults if nil)
jsonStr, err := json.EncodePretty(data, config)     // Encode with pretty formatting

// Formatting operations
pretty, err := json.FormatPretty(jsonStr)
compact, err := json.FormatCompact(jsonStr)

// Print operations (direct output to stdout)
// Smart JSON detection: string/[]byte inputs are checked for validity first
json.Print(data)           // Print compact JSON to stdout
json.PrintPretty(data)     // Print pretty JSON to stdout

// Print examples
data := map[string]any{
    "monitoring": true,
    "database": map[string]any{
        "name": "myDb",
        "port": "5432",
        "ssl":  true,
    },
}

// Print Go value as compact JSON
json.Print(data)
// Output: {"monitoring":true,"database":{"name":"myDb","port":"5432","ssl":true}}

// Print Go value as pretty JSON
json.PrintPretty(data)
// Output:
// {
//   "database": {
//     "name": "myDb",
//     "port": "5432",
//     "ssl": true
//   },
//   "monitoring": true
// }

// Print JSON string directly (no double-encoding)
jsonStr := `{"name":"John","age":30}`
json.Print(jsonStr)
// Output: {"name":"John","age":30}

// Buffer operations (encoding/json compatible)
json.Compact(dst, src)
json.Indent(dst, src, prefix, indent)
json.HTMLEscape(dst, src)

// Advanced buffer operations with processor options
json.CompactBuffer(dst, src, opts)   // With custom processor options
json.IndentBuffer(dst, src, prefix, indent, opts)
json.HTMLEscapeBuffer(dst, src, opts)

// Advanced encoding methods
// Encode stream - encode multiple values as JSON array stream
users := []map[string]any{
    {"name": "Alice", "age": 25},
    {"name": "Bob", "age": 30},
}
stream, err := json.EncodeStream(users, false)  // compact format

// Encode batch - encode multiple key-value pairs as JSON object
pairs := map[string]any{
    "user1": map[string]any{"name": "Alice", "age": 25},
    "user2": map[string]any{"name": "Bob", "age": 30},
}
batch, err := json.EncodeBatch(pairs, true)  // pretty format

// Encode fields - encode only specified fields from a struct
type User struct {
    Name  string `json:"name"`
    Age   int    `json:"age"`
    Email string `json:"email"`
}
user := User{Name: "Alice", Age: 25, Email: "alice@example.com"}
fields, err := json.EncodeFields(user, []string{"name", "age"}, true)
// Output: {"name":"Alice","age":25}
```

### File Operations

```go
// Load and save JSON files
jsonStr, err := json.LoadFromFile("data.json")
err = json.SaveToFile("output.json", data, true) // pretty format

// Marshal/Unmarshal with files
err = json.MarshalToFile("user.json", user)
err = json.MarshalToFile("user_pretty.json", user, true)
err = json.UnmarshalFromFile("user.json", &loadedUser)

// Stream operations
data, err := processor.LoadFromReader(reader)
err = processor.SaveToWriter(writer, data, true)
```

### Type Conversion & Utilities

```go
// Safe type conversion
intVal, ok := json.ConvertToInt(value)
floatVal, ok := json.ConvertToFloat64(value)
boolVal, ok := json.ConvertToBool(value)
strVal := json.ConvertToString(value)

// Generic type conversion
result, ok := json.UnifiedTypeConversion[int](value)
result, err := json.TypeSafeConvert[string](value)

// JSON comparison and merging
equal, err := json.CompareJson(json1, json2)
merged, err := json.MergeJson(json1, json2)
copy, err := json.DeepCopy(data)
```

### Processor Management

```go
// Create processor with configuration
config := &json.Config{
    EnableCache:      true,
    MaxCacheSize:     5000,
    MaxJSONSize:      50 * 1024 * 1024,
    MaxConcurrency:   100,
    EnableValidation: true,
}
processor := json.New(config)
defer processor.Close()

// Processor operations
result, err := processor.Get(jsonStr, path)
stats := processor.GetStats()
health := processor.GetHealthStatus()
processor.ClearCache()

// Cache warmup
paths := []string{"user.name", "user.age", "user.profile"}
warmupResult, err := processor.WarmupCache(jsonStr, paths)

// Global processor management
json.SetGlobalProcessor(processor)
json.ShutdownGlobalProcessor()
```

### Package-Level Convenience Methods

The library provides convenient package-level methods that use a default processor:

```go
// Performance monitoring (uses default processor)
stats := json.GetStats()
fmt.Printf("Total operations: %d\n", stats.OperationCount)
fmt.Printf("Cache hit ratio: %.2f%%\n", stats.HitRatio*100)
fmt.Printf("Cache memory usage: %d bytes\n", stats.CacheMemory)

// Health monitoring
health := json.GetHealthStatus()
fmt.Printf("System healthy: %v\n", health.Healthy)

// Cache management
json.ClearCache()  // Clear all cached data

// Cache warmup - pre-load commonly used paths
paths := []string{"user.name", "user.age", "user.profile"}
warmupResult, err := json.WarmupCache(jsonStr, paths)

// Batch processing - execute multiple operations efficiently
operations := []json.BatchOperation{
    {Type: "get", Path: "user.name"},
    {Type: "set", Path: "user.age", Value: 25},
    {Type: "delete", Path: "user.temp"},
}
results, err := json.ProcessBatch(operations)
```

### Complex Path Examples

```go
complexData := `{
  "company": {
    "departments": [
      {
        "name": "Engineering",
        "teams": [
          {
            "name": "Backend",
            "members": [
              {"name": "Alice", "skills": ["Go", "Python"], "level": "Senior"},
              {"name": "Bob", "skills": ["Java", "Spring"], "level": "Mid"}
            ]
          }
        ]
      }
    ]
  }
}`

// Multi-level nested extraction
allMembers, err := json.Get(complexData, "company.departments{teams}{flat:members}")
// Result: [Alice's data, Bob's data]

// Extract specific fields
allNames, err := json.Get(complexData, "company.departments{teams}{flat:members}{name}")
// Result: ["Alice", "Bob"]

// Flatten skills extraction
allSkills, err := json.Get(complexData, "company.departments{teams}{flat:members}{flat:skills}")
// Result: ["Go", "Python", "Java", "Spring"]
```

### Array Operations

```go
arrayData := `{
  "numbers": [1, 2, 3, 4, 5, 6, 7, 8, 9, 10],
  "users": [
    {"name": "Alice", "age": 25},
    {"name": "Bob", "age": 30}
  ]
}`

// Array indexing and slicing
first, err := json.GetInt(arrayData, "numbers[0]")           // 1
last, err := json.GetInt(arrayData, "numbers[-1]")           // 10 (negative index)
slice, err := json.Get(arrayData, "numbers[1:4]")            // [2, 3, 4]
everyOther, err := json.Get(arrayData, "numbers[::2]")       // [1, 3, 5, 7, 9]
reverseEveryOther, err := json.Get(arrayData, "numbers[::-2]")  // [10, 8, 6, 4, 2]

// Nested array access
ages, err := json.Get(arrayData, "users{age}") // [25, 30]
```

---

## 🔧 Configuration Options

### Processor Configuration

The `json.New()` function now supports optional configuration parameters:

```go
// 1. No parameters - uses default configuration
processor1 := json.New()
defer processor1.Close()

// 2. Custom configuration
customConfig := &json.Config{
    // Cache settings
    EnableCache:      true,             // Enable cache
    MaxCacheSize:     128,              // Cache entry count (default)
    CacheTTL:         5 * time.Minute,  // Cache expiration time (default)

    // Size limits
    MaxJSONSize:      100 * 1024 * 1024, // 100MB JSON size limit (default)
    MaxPathDepth:     50,                // Path depth limit (default)
    MaxBatchSize:     2000,              // Batch operation size limit

    // Concurrency settings
    MaxConcurrency:   50,   // Maximum concurrency (default)
    ParallelThreshold: 10,   // Parallel processing threshold (default)

    // Processing options
    EnableValidation: true,  // Enable validation
    StrictMode:       false, // Non-strict mode
    CreatePaths:      true,  // Auto-create paths
    CleanupNulls:     true,  // Cleanup null values
}

processor2 := json.New(customConfig)
defer processor2.Close()

// 3. Predefined configurations
// HighSecurityConfig: For processing untrusted JSON with strict validation limits
secureProcessor := json.New(json.HighSecurityConfig())
defer secureProcessor.Close()

// LargeDataConfig: For handling large JSON files with optimized performance
largeDataProcessor := json.New(json.LargeDataConfig())
defer largeDataProcessor.Close()
```

### Operation Options

```go
opts := &json.ProcessorOptions{
    CreatePaths:     true,  // Auto-create paths
    CleanupNulls:    true,  // Cleanup null values
    CompactArrays:   true,  // Compact arrays
    ContinueOnError: false, // Continue on error
    MaxDepth:        50,    // Maximum depth
}

result, err := json.Get(data, "path", opts)
```

### Performance Monitoring

```go
processor := json.New(json.DefaultConfig())
defer processor.Close()

// Get statistics after operations
stats := processor.GetStats()
fmt.Printf("Total operations: %d\n", stats.OperationCount)
fmt.Printf("Cache hit rate: %.2f%%\n", stats.HitRatio*100)
fmt.Printf("Cache memory usage: %d bytes\n", stats.CacheMemory)

// Get health status
health := processor.GetHealthStatus()
fmt.Printf("System health: %v\n", health.Healthy)
```

---

## 📁 File Operations

### Basic File Operations

```go
// Load JSON from file
data, err := json.LoadFromFile("example.json")

// Save to file (pretty format)
err = json.SaveToFile("output_pretty.json", data, true)

// Save to file (compact format)
err = json.SaveToFile("output.json", data, false)

// Load from Reader (using processor)
processor := json.New()
defer processor.Close()

file, err := os.Open("large_data.json")
if err != nil {
    log.Fatal(err)
}
defer file.Close()

data, err := processor.LoadFromReader(file)

// Save to Writer (using processor)
var buffer bytes.Buffer
err = processor.SaveToWriter(&buffer, data, true)
```

### Marshal/Unmarshal File Operations

```go
// Marshal data to file (compact format by default)
user := map[string]any{
    "name": "Alice",
    "age":  30,
    "email": "alice@example.com",
}
err := json.MarshalToFile("user.json", user)

// Marshal data to file (pretty format)
err = json.MarshalToFile("user_pretty.json", user, true)

// Unmarshal data from file
var loadedUser map[string]any
err = json.UnmarshalFromFile("user.json", &loadedUser)

// Works with structs too
type User struct {
    Name  string `json:"name"`
    Age   int    `json:"age"`
    Email string `json:"email"`
}

var person User
err = json.UnmarshalFromFile("user.json", &person)

// Using processor for advanced options
processor := json.New()
defer processor.Close()

err = processor.MarshalToFile("advanced.json", user, true)
err = processor.UnmarshalFromFile("advanced.json", &loadedUser, opts...)
```

### Batch File Processing

```go
configFiles := []string{
    "database.json",
    "cache.json",
    "logging.json",
}

allConfigs := make(map[string]any)

for _, filename := range configFiles {
    config, err := json.LoadFromFile(filename)
    if err != nil {
        log.Printf("Loading %s failed: %v", filename, err)
        continue
    }

    configName := strings.TrimSuffix(filename, ".json")
    allConfigs[configName] = config
}

// Save merged configuration
err = json.SaveToFile("merged_config.json", allConfigs, true)
if err != nil {
    log.Printf("Saving merged config failed: %v", err)
    return
}
```

---

### Security Configuration

```go
// Security configuration
secureConfig := &json.Config{
    MaxJSONSize:              10 * 1024 * 1024, // 10MB JSON size limit
    MaxPathDepth:             50,                // Path depth limit
    MaxNestingDepthSecurity:  100,               // Object nesting depth limit
    MaxArrayElements:         10000,             // Array element count limit
    MaxObjectKeys:            1000,              // Object key count limit
    ValidateInput:            true,              // Input validation
    EnableValidation:         true,              // Enable validation
    StrictMode:               true,              // Strict mode
}

processor := json.New(secureConfig)
defer processor.Close()
```

---

## 🎯 Use Cases

### Example - API Response Processing

```go
// Typical REST API response
apiResponse := `{
    "status": "success",
    "code": 200,
    "data": {
        "users": [
            {
                "id": 1,
                "profile": {
                    "name": "Alice Johnson",
                    "email": "alice@example.com"
                },
                "permissions": ["read", "write", "admin"],
                "metadata": {
                    "created_at": "2023-01-15T10:30:00Z",
                    "tags": ["premium", "verified"]
                }
            }
        ],
        "pagination": {
            "page": 1,
            "total": 25
        }
    }
}`

// Quick extraction of key information
status, err := json.GetString(apiResponse, "status")
if err != nil {
    fmt.Printf("Error: %v\n", err)
    return
}
// Result: success

code, err := json.GetInt(apiResponse, "code")
if err != nil {
    fmt.Printf("Error: %v\n", err)
    return
}
// Result: 200

// Get pagination information
totalUsers, err := json.GetInt(apiResponse, "data.pagination.total")
if err != nil {
    fmt.Printf("Error: %v\n", err)
    return
}
// Result: 25

currentPage, err := json.GetInt(apiResponse, "data.pagination.page")
if err != nil {
    fmt.Printf("Error: %v\n", err)
    return
}
// Result: 1

// Batch extract user information
userNames, err := json.Get(apiResponse, "data.users.profile.name")
if err != nil {
    fmt.Printf("Error: %v\n", err)
    return
}
// Result: ["Alice Johnson"]

userEmails, err := json.Get(apiResponse, "data.users.profile.email")
if err != nil {
    fmt.Printf("Error: %v\n", err)
    return
}
// Result: ["alice@example.com"]

// Flatten extract all permissions
allPermissions, err := json.Get(apiResponse, "data.users{flat:permissions}")
if err != nil {
    fmt.Printf("Error: %v\n", err)
    return
}
// Result: ["read", "write", "admin"]
```

### Example - Configuration File Management

```go
// Multi-environment configuration file
configJSON := `{
    "app": {
        "name": "MyApplication",
        "version": "1.2.3"
    },
    "environments": {
        "development": {
            "database": {
                "host": "localhost",
                "port": 5432,
                "name": "myapp_dev"
            },
            "cache": {
                "enabled": true,
                "host": "localhost",
                "port": 6379
            }
        },
        "production": {
            "database": {
                "host": "prod-db.example.com",
                "port": 5432,
                "name": "myapp_prod"
            },
            "cache": {
                "enabled": true,
                "host": "prod-cache.example.com",
                "port": 6379
            }
        }
    }
}`

// Type-safe configuration retrieval
dbHost := json.GetStringWithDefault(configJSON, "environments.production.database.host", "localhost")
dbPort := json.GetIntWithDefault(configJSON, "environments.production.database.port", 5432)
cacheEnabled := json.GetBoolWithDefault(configJSON, "environments.production.cache.enabled", false)

fmt.Printf("Production database: %s:%d\n", dbHost, dbPort)
fmt.Printf("Cache enabled: %v\n", cacheEnabled)

// Dynamic configuration updates
updates := map[string]any{
    "app.version": "1.2.4",
    "environments.production.cache.ttl": 10800, // 3 hours
}

newConfig, err := json.SetMultiple(configJSON, updates)
if err != nil {
    fmt.Printf("Error updating config: %v\n", err)
    return
}
```

### Example - Data Analysis Processing

```go
// Log and monitoring data
analyticsData := `{
    "events": [
        {
            "type": "request",
            "user_id": "user_123",
            "endpoint": "/api/users",
            "status_code": 200,
            "response_time": 45
        },
        {
            "type": "error",
            "user_id": "user_456",
            "endpoint": "/api/orders",
            "status_code": 500,
            "response_time": 5000
        }
    ]
}`

// Extract all event types
eventTypes, err := json.Get(analyticsData, "events.type")
if err != nil {
    fmt.Printf("Error: %v\n", err)
    return
}
// Result: ["request", "error"]

// Extract all status codes
statusCodes, err := json.Get(analyticsData, "events.status_code")
if err != nil {
    fmt.Printf("Error: %v\n", err)
    return
}
// Result: [200, 500]

// Extract all response times
responseTimes, err := json.GetTyped[[]int](analyticsData, "events.response_time")
if err != nil {
    fmt.Printf("Error: %v\n", err)
    return
}
// Result: [45, 5000]

// Calculate average response time
times := responseTimes
var total float64
for _, t := range times {
    total += t
}

avgTime := total / float64(len(times))
fmt.Printf("Average response time: %.2f ms\n", avgTime)
```

---

## Set Operations - Data Safety Guarantee

All Set operations follow a **safe-by-default** pattern that ensures your data is never corrupted:

```go
// ✅ Success: Returns modified data
result, err := json.Set(data, "user.name", "Alice")
if err == nil {
    // result contains successfully modified JSON
    fmt.Println("Data updated:", result)
}

// ❌ Failure: Returns original unmodified data
result, err := json.Set(data, "invalid[path", "value")
if err != nil {
    // result still contains valid original data
    // Your original data is NEVER corrupted
    fmt.Printf("Set failed: %v\n", err)
    fmt.Println("Original data preserved:", result)
}
```

**Key Benefits**:
- 🔒 **Data Integrity**: Original data never corrupted on error
- ✅ **Safe Fallback**: Always have valid JSON to work with
- 🎯 **Predictable**: Consistent behavior across all operations

---

## 📦 Advanced Features

### JSONL (JSON Lines) Support

The library provides comprehensive support for JSON Lines format, commonly used for logs, data pipelines, and streaming data:

```go
// Parse JSONL data
jsonlData := `{"name":"Alice","age":25}
{"name":"Bob","age":30}
{"name":"Carol","age":28}`

// Parse into slice
results, err := json.ParseJSONL([]byte(jsonlData))

// Stream processing for large files
processor := json.NewJSONLProcessor(reader)
err := processor.StreamLines(func(lineNum int, data any) bool {
    fmt.Printf("Line %d: %v\n", lineNum, data)
    return true // continue processing
})

// Parallel processing for CPU-bound operations
err := processor.StreamLinesParallel(func(lineNum int, data any) error {
    // Process each line in parallel
    return nil
}, 4) // 4 workers

// Type-safe streaming with generics
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
writer.Write(map[string]any{"event": "logout", "user": "bob"})

// Convert slice to JSONL
data := []any{
    map[string]any{"id": 1, "name": "Alice"},
    map[string]any{"id": 2, "name": "Bob"},
}
jsonlBytes, err := json.ToJSONL(data)
```

### Streaming Processing

For large JSON files, use streaming processors to avoid loading everything into memory:

```go
// Create streaming processor
processor := json.NewStreamingProcessor(reader, 64*1024) // 64KB buffer

// Stream array elements one at a time
err := processor.StreamArray(func(index int, item any) bool {
    fmt.Printf("Item %d: %v\n", index, item)
    return true // continue
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

// Memory-efficient array counting
count, err := json.StreamArrayCount(reader)

// Get first matching element (stops early)
first, found, err := json.StreamArrayFirst(reader, func(item any) bool {
    return item.(map[string]any)["priority"] == "high"
})

// Pagination support
page2, err := json.StreamArraySkip(reader, 10)  // Skip first 10
page, err := json.StreamArrayTake(reader, 10)   // Take first 10
```

### Lazy JSON Parsing

Parse JSON on-demand for improved performance when only accessing specific paths:

```go
// Create lazy parser - parsing happens on first access
lazy := json.NewLazyJSON(jsonBytes)

// Parse only when Get is called
value, err := lazy.Get("user.profile.name")

// Check if already parsed
if lazy.IsParsed() {
    data := lazy.Parsed()
}

// Get parsing error (triggers parsing if not done)
if err := lazy.Error(); err != nil {
    log.Printf("Parse error: %v", err)
}

// Access raw bytes without parsing
rawBytes := lazy.Raw()
```

### Large File Processing

Process very large JSON files efficiently:

```go
// Configure for large files
config := json.LargeFileConfig{
    ChunkSize:       1024 * 1024,       // 1MB chunks
    MaxMemory:       100 * 1024 * 1024, // 100MB max
    BufferSize:      64 * 1024,         // 64KB buffer
    SamplingEnabled: true,
    SampleSize:      1000,
}

processor := json.NewLargeFileProcessor(config)

// Process file element by element
err := processor.ProcessFile("large.json", func(item any) error {
    // Process each item without loading entire file
    return nil
})

// Process in chunks for batch operations
err := processor.ProcessFileChunked("large.json", 100, func(chunk []any) error {
    // Process 100 items at a time
    return nil
})

// Chunked reader for custom processing
reader := json.NewChunkedReader(file, 1024*1024)
err := reader.ReadArray(func(item any) bool {
    // Process each item
    return true
})
```

### NDJSON Processing

Process newline-delimited JSON files efficiently:

```go
processor := json.NewNDJSONProcessor(64 * 1024) // 64KB buffer

// Process file line by line
err := processor.ProcessFile("logs.ndjson", func(lineNum int, obj map[string]any) error {
    fmt.Printf("Line %d: %v\n", lineNum, obj)
    return nil
})

// Process from reader
err := processor.ProcessReader(reader, func(lineNum int, obj map[string]any) error {
    // Process each JSON object
    return nil
})
```

---

## 💡 Examples & Resources

### 📁 Example Code

- **[Basic Usage](examples/1_basic_usage.go)** - Getting started with core operations
- **[Advanced Features](examples/2_advanced_features.go)** - Complex path queries and file operations
- **[Production Ready](examples/3_production_ready.go)** - Thread-safe patterns and monitoring
- **[Type Conversion](examples/7_type_conversion.go)** - Safe type conversion utilities
- **[File Operations](examples/10_file_operations.go)** - Reading and writing JSON files
- **[Iterator Functions](examples/9_iterator_functions.go)** - Iteration and traversal patterns
- **[With Defaults](examples/11_with_defaults.go)** - Default value handling
- **[Advanced Delete](examples/12_advanced_delete.go)** - Complex deletion operations

### 📖 Additional Resources

- **[API Documentation](https://pkg.go.dev/github.com/cybergodev/json)** - Complete API reference
- **[Security Guide](docs/SECURITY.md)** - Security best practices and configuration

---

## 🤝 Contributing

Contributions, issue reports, and suggestions are welcome!

## 📄 License

MIT License - See [LICENSE](LICENSE) file for details.

---

**Crafted with care for the Go community** ❤️ | If this project helps you, please give it a ⭐️ Star!
