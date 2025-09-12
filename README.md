# 🚀 cybergodev/json - High-Performance Go JSON Processing Library

[![Go Version](https://img.shields.io/badge/go-%3E%3D1.24-blue.svg)](https://golang.org/)
[![MIT license](https://img.shields.io/badge/license-MIT-brightgreen.svg)](https://opensource.org/licenses/MIT)
[![Performance](https://img.shields.io/badge/performance-enterprise%20grade-green.svg)](https://github.com/cybergodev/json)
[![Thread Safe](https://img.shields.io/badge/thread%20safe-yes-brightgreen.svg)](https://github.com/cybergodev/json)

> A high-performance, feature-rich Go JSON processing library with 100% `encoding/json` compatibility, providing powerful path operations, type safety, performance optimization, and rich advanced features.

#### **[📖 中文文档](docs/doc_zh_CN.md)** - User guide

---

## 📚 Table of Contents

- [📖 Overview](#-overview)
- [🚀 Quick Start](#-quick-start)
- [⚡ Core Features](#-core-features)
- [🎯 Path Expressions](#-Base-Path-Expressions)
- [🔧 Configuration Options](#-configuration-options)
- [📁 File Operations](#-file-operations)
- [🔄 Data Validation](#-data-validation)
- [🎯 Use Cases](#-use-cases)
- [📋 API Reference](#-api-reference)
- [📚 Best Practices](#-best-practices)
- [💡 Examples & Resources](#-examples--resources)


---

## 📖 Overview

**`cybergodev/json`** is a high-performance Go JSON processing library that maintains 100% compatibility with the standard `encoding/json` package while providing powerful path operations, type safety, performance optimization, and rich advanced features.

### 🏆 Core Advantages

- **🔄 Full Compatibility** - 100% compatible with standard `encoding/json`, zero learning curve, drop-in replacement
- **🎯 Powerful Paths** - Support for complex path expressions, complete complex data operations in one line
- **🚀 High Performance** - Smart caching, concurrent safety, memory optimization, enterprise-grade performance
- **🛡️ Type Safety** - Generic support, compile-time checking, intelligent type conversion
- **🔧 Feature Rich** - Batch operations, data validation, file operations, performance monitoring
- **🏗️ Production Ready** - Thread-safe, error handling, security configuration, monitoring metrics

### 🎯 Use Cases

- **🌐 API Data Processing** - Fast extraction and transformation of complex response data
- **⚙️ Configuration Management** - Dynamic configuration reading and batch updates
- **📊 Data Analysis** - Statistics and analysis of large amounts of JSON data
- **🔄 Microservice Communication** - Data exchange and format conversion between services
- **📝 Log Processing** - Parsing and analysis of structured logs

### 📚 More Examples & Documentation

- **[📁 Examples](examples)** - Comprehensive code examples for all features
- **[⚙️ Configuration Guide](examples/configuration)** - Advanced configuration and optimization
- **[📖 Compatibility](docs/compatibility.md)** - Compatibility guide and migration information

---

## 🎯 Base Path Expressions

### Path Syntax

| Syntax                 | Description    | Example                   | Result                |
|-----------------------|----------------|---------------------------|-----------------------|
| `.`                   | Property access | `user.name`              | Get user's name property |
| `[n]`                 | Array index    | `users[0]`               | Get first user           |
| `[-n]`                | Negative index | `users[-1]`              | Get last user            |
| `[start:end:step]`    | Array slice    | `users[1:3]`             | Get users at index 1-2   |
| `{field}`             | Batch extract  | `users{name}`            | Extract all user names   |
| `{flat:field}`        | Flatten extract| `users{flat:skills}`     | Flatten extract all skills |

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
    jsonBytes, _ := json.Marshal(data)

    var result map[string]any
    json.Unmarshal(jsonBytes, &result)

    // 2. Powerful path operations (enhanced features)
    jsonStr := `{"user":{"profile":{"name":"Alice","age":25}}}`

    name, _ := json.GetString(jsonStr, "user.profile.name")
    fmt.Println(name) // "Alice"

    age, _ := json.GetInt(jsonStr, "user.profile.age")
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

// Get all user names
names, _ := json.Get(complexData, "users{name}")
// Result: ["Alice", "Bob"]

// Get all skills (flattened)
skills, _ := json.Get(complexData, "users{flat:skills}")
// Result: ["Go", "Python", "Java", "React"]

// Batch get multiple values
paths := []string{"users[0].name", "users[1].name", "users{active}"}
results, _ := json.GetMultiple(complexData, paths)
```


---

## ⚡ Core Features

### Data Retrieval

```go
// Basic retrieval
json.Get(data, "user.name")          // Get any type
json.GetString(data, "user.name")    // Get string
json.GetInt(data, "user.age")        // Get integer
json.GetBool(data, "user.active")    // Get boolean
json.GetArray(data, "user.tags")     // Get array
json.GetObject(data, "user.profile") // Get object

// Type-safe retrieval
json.GetTyped[string](data, "user.name") // Generic type safety
json.GetTyped[[]User](data, "users")     // Custom types

// Retrieval with default values
json.GetStringWithDefault(data, "user.name", "Anonymous")
json.GetIntWithDefault(data, "user.age", 0)

// Batch retrieval
paths := []string{"user.name", "user.age", "user.email"}
results, _ := json.GetMultiple(data, paths)
```

### Data Modification

```go
// Basic setting - returns modified data on success, original data on failure
result, err := json.Set(data, "user.name", "Alice")
if err != nil {
    // result contains original unmodified data
    fmt.Printf("Set failed: %v, original data preserved\n", err)
} else {
    // result contains modified data
    fmt.Println("Set successful, data modified")
}

// Auto-create paths
result, err := json.SetWithAdd(data, "user.profile.city", "NYC")
if err != nil {
    // result contains original data if creation failed
    fmt.Printf("Path creation failed: %v\n", err)
}

// Batch setting
updates := map[string]any{
    "user.name": "Bob",
    "user.age":  30,
    "user.active": true,
}
result, err := json.SetMultiple(data, updates)
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

// Path iteration - read-only traversal of JSON subset
json.ForeachWithPath(data, "data.list.users", func (key any, user *json.IterableValue) {
    name := user.GetString("name")
    age := user.GetInt("age")

    // Note: ForeachWithPath is read-only, modifications won't affect original data
    fmt.Printf("User: %s, Age: %d\n", name, age)
})

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
allMembers, _ := json.Get(complexData, "company.departments{teams}{flat:members}")
// Result: [Alice's data, Bob's data]

// Extract specific fields
allNames, _ := json.Get(complexData, "company.departments{teams}{flat:members}{name}")
// Result: ["Alice", "Bob"]

// Flatten skills extraction
allSkills, _ := json.Get(complexData, "company.departments{teams}{flat:members}{flat:skills}")
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
first, _ := json.GetInt(arrayData, "numbers[0]")       // 1
last, _ := json.GetInt(arrayData, "numbers[-1]")       // 10 (negative index)
slice, _ := json.Get(arrayData, "numbers[1:4]")        // [2, 3, 4]
everyOther, _ := json.Get(arrayData, "numbers[::2]")   // [1, 3, 5, 7, 9]
everyOther, _ := json.Get(arrayData, "numbers[::-2]")  // [10 8 6 4 2]

// Nested array access
ages, _ := json.Get(arrayData, "users{age}") // [25, 30]
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

result, _ := json.Get(data, "path", opts)
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
data, err := json.LoadFromFile("config.json")
if err != nil {
    log.Printf("File load failed: %v", err)
    return
}

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

data, err = json.LoadFromReader(file)

// Save to Writer
var buffer bytes.Buffer
err = json.SaveToWriter(&buffer, data, true)
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

## 🛡️ Data Validation

### JSON Schema Validation

```go
// Define JSON Schema
schema := &json.Schema{
    Type: "object",
    Properties: map[string]*json.Schema{
        "name": (&json.Schema{
            Type: "string",
        }).SetMinLength(1).SetMaxLength(100),
        "age": (&json.Schema{
            Type: "number",
        }).SetMinimum(0.0).SetMaximum(150.0),
        "email": {
            Type:   "string",
            Format: "email",
        },
    },
    Required: []string{"name", "age", "email"},
}

// Validate data
testData := `{
    "name": "Alice",
    "age": 25,
    "email": "alice@example.com"
}`

processor := json.New(json.DefaultConfig())
errors, err := processor.ValidateSchema(testData, schema)
if len(errors) > 0 {
    fmt.Println("Validation errors:")
    for _, validationErr := range errors {
        fmt.Printf("  Path %s: %s\n", validationErr.Path, validationErr.Message)
    }
} else {
    fmt.Println("Data validation passed")
}
```

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
status, _ := json.GetString(apiResponse, "status")
code, _ := json.GetInt(apiResponse, "code")

// Batch extract user information
userNames, _ := json.Get(apiResponse, "data.users.profile.name")
// Result: ["Alice Johnson"]

userEmails, _ := json.Get(apiResponse, "data.users.profile.email")
// Result: ["alice@example.com"]

// Flatten extract all permissions
allPermissions, _ := json.Get(apiResponse, "data.users{flat:permissions}")
// Result: ["read", "write", "admin"]

// Get pagination information
totalUsers, _ := json.GetInt(apiResponse, "data.pagination.total")
currentPage, _ := json.GetInt(apiResponse, "data.pagination.page")

fmt.Printf("Status: %s (Code: %d)\n", status, code)
fmt.Printf("Total users: %d, Current page: %d\n", totalUsers, currentPage)
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

## 📋 API Reference

### Core Methods

#### Data Retrieval

```go
// Basic retrieval
json.Get(data, path) (any, error)
json.GetString(data, path) (string, error)
json.GetInt(data, path) (int, error)
json.GetBool(data, path) (bool, error)
json.GetFloat64(data, path) (float64, error)
json.GetArray(data, path) ([]any, error)
json.GetObject(data, path) (map[string]any, error)

// Type-safe retrieval
json.GetTyped[T](data, path) (T, error)

// Retrieval with default values
json.GetStringWithDefault(data, path, defaultValue) string
json.GetIntWithDefault(data, path, defaultValue) int
json.GetBoolWithDefault(data, path, defaultValue) bool

// Batch retrieval
json.GetMultiple(data, paths) (map[string]any, error)
```

#### Data Modification

```go
// Basic setting - improved error handling
// Returns: (modified_data, nil) on success, (original_data, error) on failure
json.Set(data, path, value) (string, error)
json.SetWithAdd(data, path, value) (string, error)

// Batch setting - same improved behavior
json.SetMultiple(data, updates) (string, error)
json.SetMultipleWithAdd(data, updates) (string, error)
```

#### Data Deletion

```go
json.Delete(data, path) (string, error)
json.DeleteWithCleanNull(data, path) (string, error)
```

#### Data Iteration

```go
// Basic iteration methods
json.Foreach(data, callback) error
json.ForeachReturn(data, callback) (string, error)

// Path iteration - read-only traversal of specified path data
json.ForeachWithPath(data, path, callback) error

// Nested iteration - prevents state conflicts
json.ForeachNested(data, callback) error
json.ForeachReturnNested(data, callback) (string, error)

// IterableValue nested methods - used within iteration callbacks
item.ForeachReturnNested(path, callback) error
```

**Use case comparison:**

| Method | Return Value | Data Modification | Traversal Range | Usage Scenarios | 
|------|--------|-------------------|----------|----------|
| `Foreach` | `error` | ❌ Not allow       | Complete JSON | Read-only traversal of the entire JSON |
| `ForeachWithPath` | `error` | ❌ Not allow       | Specified path | Read-only traversal of JSON subset |
| `ForeachReturn` | `(string, error)` | ✅ Allow           | Complete JSON | Data modification, batch update |


### File Operation Methods

```go
// File read/write
json.LoadFromFile(filename, ...opts) (string, error)
json.SaveToFile(filename, data, pretty) error

// Stream operations
json.LoadFromReader(reader, ...opts) (string, error)
json.SaveToWriter(writer, data, pretty) error
```

### Validation Methods

```go
// Schema validation
processor.ValidateSchema(data, schema) ([]ValidationError, error)

// Basic validation
json.Valid(data) bool
```

### Processor Methods

```go
// Create processor
json.New(config) *Processor
json.DefaultConfig() *Config

// Cache operations
processor.WarmupCache(data, paths) (*WarmupResult, error)
processor.ClearCache()

// Statistics
processor.GetStats() *Stats
processor.GetHealthStatus() *HealthStatus
```

### Error Handling Strategy

```go
// Recommended error handling approach
result, err := json.GetString(data, "user.name")
if err != nil {
    log.Printf("Failed to get username: %v", err)
    // Use default value or return error
    return "", err
}

// Use methods with default values
name := json.GetStringWithDefault(data, "user.name", "Anonymous")
```

---

## 📚 Best Practices

### Performance Optimization Tips

1. **Enable Caching** - For repeated operations, enabling cache can significantly improve performance
2. **Batch Operations** - Use `GetMultiple` and `SetMultiple` for batch processing
3. **Path Warmup** - Use `WarmupCache` to pre-warm commonly used paths
4. **Reasonable Configuration** - Adjust cache size and TTL according to actual needs

### Security Usage Guidelines

1. **Input Validation** - Enable `ValidateInput` to validate input data
2. **Size Limits** - Set reasonable `MaxJSONSize` and `MaxPathDepth`
3. **Schema Validation** - Use JSON Schema validation for critical data
4. **Error Handling** - Always check returned error information

### Memory Management

1. **Processor Lifecycle** - Always call `processor.Close()` to clean up resources
2. **Avoid Memory Leaks** - Don't hold references to large JSON strings unnecessarily
3. **Batch Size Control** - Set appropriate `MaxBatchSize` for batch operations
4. **Cache Management** - Monitor cache memory usage and adjust size as needed

### Thread Safety

1. **Default Processor** - The global default processor is thread-safe
2. **Custom Processors** - Each processor instance is thread-safe
3. **Concurrent Operations** - Multiple goroutines can safely use the same processor
4. **Resource Sharing** - Processors can be safely shared across goroutines

---

## 💡 Examples & Resources

### 📁 Example Code

The repository includes comprehensive examples demonstrating various features and use cases:

#### Basic Examples
- **[Basic Usage](examples/basic)** - Fundamental operations and getting started
- **[JSON Get Operations](examples/json_get)** - Data retrieval examples with different path expressions
- **[JSON Set Operations](examples/json_set)** - Data modification and batch updates
- **[JSON Delete Operations](examples/json_delete)** - Data deletion and cleanup operations

#### Advanced Examples
- **[File Operations](examples/file_operations)** - File I/O, batch processing, and stream operations
- **[JSON Iteration](examples/json_iteration)** - Data iteration and traversal patterns
- **[Flat Extraction](examples/flat_extraction)** - Complex data extraction and flattening
- **[JSON Encoding](examples/json_encode)** - Custom encoding configurations and formatting

#### Configuration Examples
- **[Configuration Management](examples/configuration)** - Processor configuration and optimization
- **[Compatibility Examples](examples/compatibility)** - Drop-in replacement demonstrations

---

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

---

## 🤝 Contributing

Contributions are welcome! Please feel free to submit a Pull Request. For major changes, please open an issue first to discuss what you would like to change.

## 🌟 Star History

If you find this project useful, please consider giving it a star! ⭐

---

**Made with ❤️ by the CyberGoDev team**