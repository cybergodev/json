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
    fmt.Println(name) // "Alice"

    age, err := json.GetInt(jsonStr, "user.profile.age")
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
// Result: ["Alice", "Bob"]

// Get all skills (flattened)
skills, err := json.Get(complexData, "users{flat:skills}")
// Result: ["Go", "Python", "Java", "React"]

// Batch get multiple values
paths := []string{"users[0].name", "users[1].name", "users{active}"}
results, err := json.GetMultiple(complexData, paths)
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
json.ForeachNested(data, callback)                  // Nested-safe iteration
json.ForeachWithPath(data, "data.users", callback)  // Iterate specific path

// Iterate and return modified JSON - supports data modification
modifiedJson, err := json.ForeachReturn(data, func (key any, item *json.IterableValue) {
    // Modify data during iteration
    if item.GetString("status") == "inactive" {
        item.Set("status", "active")
        item.Set("updated_at", time.Now().Format("2006-01-02"))
    }
    
    // Batch update user information
    if key == "users" {
        item.SetMultiple(map[string]any{
            "last_login": time.Now().Unix(),
            "version": "2.0",
        })
    }
})
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
    MaxDepth:     10,  // Required: maximum encoding depth
}
jsonStr, err := json.Encode(data, config)  // config is optional
jsonStr, err := json.EncodePretty(data, config)
jsonStr, err := json.EncodeCompact(data, config)

// Formatting operations
pretty, err := json.FormatPretty(jsonStr)
compact, err := json.FormatCompact(jsonStr)

// Buffer operations (encoding/json compatible)
json.Compact(dst, src)
json.Indent(dst, src, prefix, indent)
json.HTMLEscape(dst, src)
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
first, err := json.GetInt(arrayData, "numbers[0]")       // 1
last, err := json.GetInt(arrayData, "numbers[-1]")       // 10 (negative index)
slice, err := json.Get(arrayData, "numbers[1:4]")        // [2, 3, 4]
everyOther, err := json.Get(arrayData, "numbers[::2]")   // [1, 3, 5, 7, 9]
everyOther, err := json.Get(arrayData, "numbers[::-2]")  // [10 8 6 4 2]

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

// 2. Explicit nil - same as default configuration
processor2 := json.New()
defer processor2.Close()

// 3. Custom configuration
customConfig := &json.Config{
    // Cache settings
    EnableCache:      true,             // Enable cache
    MaxCacheSize:     5000,             // Cache entry count
    CacheTTL:         10 * time.Minute, // Cache expiration time

    // Size limits
    MaxJSONSize:      50 * 1024 * 1024, // 50MB JSON size limit
    MaxPathDepth:     200,              // Path depth limit
    MaxBatchSize:     2000,             // Batch operation size limit

    // Concurrency settings
    MaxConcurrency:   100,   // Maximum concurrency
    ParallelThreshold: 20,   // Parallel processing threshold

    // Processing options
    EnableValidation: true,  // Enable validation
    StrictMode:       false, // Non-strict mode
    CreatePaths:      true,  // Auto-create paths
    CleanupNulls:     true,  // Cleanup null values
}

processor3 := json.New(customConfig)
defer processor3.Close()

// 4. Predefined configurations
secureProcessor := json.New(json.HighSecurityConfig())
largeDataProcessor := json.New(json.LargeDataConfig())
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

// Load from Reader
file, err := os.Open("large_data.json")
if err != nil {
    log.Fatal(err)
}
defer file.Close()

data, err := json.LoadFromReader(file)

// Save to Writer
var buffer bytes.Buffer
err = json.SaveToWriter(&buffer, data, true)
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
err := json.SaveToFile("merged_config.json", allConfigs, true)
```

---

### Security Configuration

```go
// Security configuration
secureConfig := &json.Config{
    MaxJSONSize:       10 * 1024 * 1024,    // 10MB JSON size limit
    MaxPathDepth:      50,                  // Path depth limit
    MaxNestingDepth:   100,                 // Object nesting depth limit
    MaxArrayElements:  10000,               // Array element count limit
    MaxObjectKeys:     1000,                // Object key count limit
    ValidateInput:     true,                // Input validation
    EnableValidation:  true,                // Enable validation
    StrictMode:        true,                // Strict mode
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
// Result: success

code, err := json.GetInt(apiResponse, "code")
// Result: 200

// Get pagination information
totalUsers, err := json.GetInt(apiResponse, "data.pagination.total")
// Result: 25

currentPage, err := json.GetInt(apiResponse, "data.pagination.page")
// Result: 1

// Batch extract user information
userNames, err := json.Get(apiResponse, "data.users.profile.name")
// Result: ["Alice Johnson"]

userEmails, err := json.Get(apiResponse, "data.users.profile.email")
// Result: ["alice@example.com"]

// Flatten extract all permissions
allPermissions, err := json.Get(apiResponse, "data.users{flat:permissions}")
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

newConfig, _ := json.SetMultiple(configJSON, updates)
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
eventTypes, _ := json.Get(analyticsData, "events.type")
// Result: ["request", "error"]

// Extract all status codes
statusCodes, _ := json.Get(analyticsData, "events.status_code")
// Result: [200, 500]

// Extract all response times
responseTimes, _ := json.GetTyped[[]float64](analyticsData, "events.response_time")
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

## 💡 Examples & Resources

### 📁 Example Code

- **[Basic Usage](examples/1_basic_usage.go)** - examples/1.basic_usage.go
- **[Advanced Features](examples/2_advanced_features.go)** - examples/2.advanced_features.go
- **[Production Ready](examples/3_production_ready.go)** - examples/3.production_ready.go

### 📖 Additional Resources

- **[Compatibility Guide](docs/COMPATIBILITY.md)** - Drop-in replacement for `encoding/json`
- **[Quick Reference](docs/QUICK_REFERENCE.md)** - Common operations cheat sheet
- **[API Documentation](https://pkg.go.dev/github.com/cybergodev/json)** - Complete API reference

---

## 🤝 Contributing

Contributions, issue reports, and suggestions are welcome!

## 📄 License

MIT License - See [LICENSE](LICENSE) file for details.

---

**Crafted with care for the Go community** ❤️ | If this project helps you, please give it a ⭐️ Star!
