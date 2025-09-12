# 🚀 cybergodev/json - 高效优雅的 Go JSON 处理库

[![Go Version](https://img.shields.io/badge/go-%3E%3D1.24-blue.svg)](https://golang.org/)
[![MIT license](https://img.shields.io/badge/license-MIT-brightgreen.svg)](https://opensource.org/licenses/MIT)
[![Performance](https://img.shields.io/badge/performance-enterprise%20grade-green.svg)](https://github.com/cybergodev/json)
[![Thread Safe](https://img.shields.io/badge/thread%20safe-yes-brightgreen.svg)](https://github.com/cybergodev/json)

> 高性能、功能强大的 Go JSON 处理库，100% 兼容标准 `encoding/json`，提供强大的路径操作、类型安全、性能优化和丰富的高级功能。

#### **[📖 English Docs](../README.md)** - User guide

---

## 📚 目录

- [📖 概述](#-概述)
- [🚀 快速开始](#-快速开始)
- [⚡ 核心功能](#-核心功能)
- [🎯 路径表达式](#-路径表达式)
- [🔧 配置选项](#-配置选项)
- [📁 文件操作](#-文件操作)
- [🔄 数据验证](#-数据验证)
- [🎯 应用场景](#-应用场景)
- [📋 API 参考](#-api-参考)
- [📚 最佳实践](#-最佳实践)
- [💡 示例与资源](#-示例与资源)

---

## 📖 概述

**`cybergodev/json`** 是一个高效的 Go JSON 处理库，在保持 100% 兼容标准 `encoding/json`
的基础上，提供了强大的路径操作、类型安全、性能优化和丰富的高级功能。

### 🏆 核心优势

- **🔄 完全兼容** - 100% 兼容标准 `encoding/json`，零学习成本，可直接替换
- **🎯 强大路径** - 支持复杂路径表达式，一行代码完成复杂数据操作
- **🚀 高性能** - 智能缓存、并发安全、内存优化、企业级性能
- **📋 类型安全** - 泛型支持、编译时检查、智能类型转换
- **🔧 功能丰富** - 批量操作、数据验证、文件操作、性能监控
- **🏗️ 生产就绪** - 线程安全、错误处理、安全配置、监控指标

### 🎯 适用场景

- **🌐 API 数据处理** - 复杂响应数据的快速提取和转换
- **⚙️ 配置文件管理** - 动态配置读取和批量更新
- **📊 数据分析** - 大量 JSON 数据的统计和分析
- **🔄 微服务通信** - 服务间数据交换和格式转换
- **📝 日志处理** - 结构化日志的解析和分析

### 📚 更多示例与文档

- **[📁 参考示例](../examples)** - 所有功能的综合代码示例
- **[⚙️ 配置指南](../examples/configuration)** - 高级配置与优化
- **[📖 兼容性说明](compatibility.md)** - 兼容性指南及迁移信息

---

## 🎯 基础路径语法

### 路径语法

| 语法                 | 描述    | 示例                   | 结果                |
|--------------------|-------|----------------------|-------------------|
| `.`                | 属性访问  | `user.name`          | 获取 user 的 name 属性 |
| `[n]`              | 数组索引  | `users[0]`           | 获取第一个用户           |
| `[-n]`             | 负数索引  | `users[-1]`          | 获取最后一个用户          |
| `[start:end:step]` | 数组切片  | `users[1:3]`         | 获取索引 1-2 的用户      |
| `{field}`          | 批量提取  | `users{name}`        | 提取所有用户的 name      |
| `{flat:field}`     | 扁平化提取 | `users{flat:skills}` | 扁平化提取所有技能         |

## 🚀 快速开始

### 安装

```bash
go get github.com/cybergodev/json
```

### 基础使用

```go
package main

import (
    "fmt"
    "github.com/cybergodev/json"
)

func main() {
    // 1. 完全兼容标准库
    data := map[string]any{"name": "Alice", "age": 25}
    jsonBytes, _ := json.Marshal(data)

    var result map[string]any
    json.Unmarshal(jsonBytes, &result)

    // 2. 强大的路径操作（增强功能）
    jsonStr := `{"user":{"profile":{"name":"Alice","age":25}}}`

    name, _ := json.GetString(jsonStr, "user.profile.name")
    fmt.Println(name) // "Alice"

    age, _ := json.GetInt(jsonStr, "user.profile.age")
    fmt.Println(age) // 25
}
```

### 路径操作示例

```go
// 复杂 JSON 数据
complexData := `{
  "users": [
    {"name": "Alice", "skills": ["Go", "Python"], "active": true},
    {"name": "Bob", "skills": ["Java", "React"], "active": false}
  ]
}`

// 获取所有用户名
names, _ := json.Get(complexData, "users{name}")
// 结果: ["Alice", "Bob"]

// 获取所有技能（扁平化）
skills, _ := json.Get(complexData, "users{flat:skills}")
// 结果: ["Go", "Python", "Java", "React"]

// 批量获取多个值
paths := []string{"users[0].name", "users[1].name", "users{active}"}
results, _ := json.GetMultiple(complexData, paths)
```

---

## ⚡ 核心功能

### 数据获取

```go
// 基础获取
json.Get(data, "user.name")              // 获取任意类型
json.GetString(data, "user.name")        // 获取字符串
json.GetInt(data, "user.age")            // 获取整数
json.GetBool(data, "user.active")        // 获取布尔值
json.GetArray(data, "user.tags")         // 获取数组
json.GetObject(data, "user.profile")     // 获取对象

// 类型安全获取
json.GetTyped[string](data, "user.name") // 泛型类型安全
json.GetTyped[[]User](data, "users")     // 自定义类型

// 带默认值获取
json.GetStringWithDefault(data, "user.name", "Anonymous")
json.GetIntWithDefault(data, "user.age", 0)

// 批量获取
paths := []string{"user.name", "user.age", "user.email"}
results, _ := json.GetMultiple(data, paths)
```

### 数据修改

```go
// 基础设置 - 成功时返回修改后的数据，失败时返回原始数据
result, err := json.Set(data, "user.name", "Alice")
if err != nil {
    // result 包含原始未修改的数据
    fmt.Printf("设置失败: %v，原始数据已保留\n", err)
} else {
    // result 包含修改后的数据
    fmt.Println("设置成功，数据已修改")
}

// 自动创建路径
result, err := json.SetWithAdd(data, "user.profile.city", "NYC")
if err != nil {
    // result 包含原始数据（如果创建失败）
    fmt.Printf("路径创建失败: %v\n", err)
}

// 批量设置
updates := map[string]any{
    "user.name": "Bob",
    "user.age":  30,
    "user.active": true,
}

result, err := json.SetMultiple(data, updates)
// 同样的行为：成功 = 修改后的数据，失败 = 原始数据
```

### 数据删除

```go
json.Delete(data, "user.temp")              // 删除字段
json.DeleteWithCleanNull(data, "user.temp") // 删除并清理空值
```

### 数据迭代

```go
// 基础迭代 - 只读遍历
json.Foreach(data, func (key any, item *json.IterableValue) {
    name := item.GetString("name")
    fmt.Printf("Key: %v, Name: %s\n", key, name)
})

// 路径迭代 - 只读遍历部分JSON
json.ForeachWithPath(data, "data.list.users", func (key any, user *json.IterableValue) {
    name := user.GetString("name")
    age := user.GetInt("age")
    
    // 注意：ForeachWithPath 是只读的，修改不会影响原始数据
    fmt.Printf("用户: %s, 年龄: %d\n", name, age)
})

// 迭代并返回修改后的JSON - 支持数据修改
modifiedJson, err := json.ForeachReturn(data, func (key any, item *json.IterableValue) {
    // 在迭代过程中修改数据
    if item.GetString("status") == "inactive" {
        item.Set("status", "active")
        item.Set("updated_at", time.Now().Format("2006-01-02"))
    }

    // 批量更新用户信息
    if key == "users" {
        item.SetMultiple(map[string]any{
            "last_login": time.Now().Unix(),
            "version": "2.0",
        })
    }
})
```

### 复杂路径示例

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

// 多层嵌套提取
allMembers, _ := json.Get(complexData, "company.departments{teams}{flat:members}")
// 结果: [Alice的数据, Bob的数据]

// 提取特定字段
allNames, _ := json.Get(complexData, "company.departments{teams}{flat:members}{name}")
// 结果: ["Alice", "Bob"]

// 扁平化技能提取
allSkills, _ := json.Get(complexData, "company.departments{flat:teams}{flat:members}{flat:skills}")
// 结果: ["Go", "Python", "Java", "Spring"]
```

### 数组操作

```go
arrayData := `{
  "numbers": [1, 2, 3, 4, 5, 6, 7, 8, 9, 10],
  "users": [
    {"name": "Alice", "age": 25},
    {"name": "Bob", "age": 30}
  ]
}`

// 数组索引和切片
first, _ := json.GetInt(arrayData, "numbers[0]")        // 1
last, _ := json.GetInt(arrayData, "numbers[-1]")        // 10 (负索引)
slice, _ := json.Get(arrayData, "numbers[1:4]")         // [2, 3, 4]
everyOther, _ := json.Get(arrayData, "numbers[::2]")    // [1, 3, 5, 7, 9]
everyOther, _ := json.Get(arrayData, "numbers[::-2]")   // [10 8 6 4 2]

// 嵌套数组访问
ages, _ := json.Get(arrayData, "users{age}") // [25, 30]
```

---

## 🔧 配置选项

### 处理器配置

`json.New()` 函数现在支持可选的配置参数：

```go
// 1. 无参数 - 使用默认配置
processor1 := json.New()
defer processor1.Close()

// 2. 显式 nil - 与默认配置相同
processor2 := json.New()
defer processor2.Close()

// 3. 自定义配置
customConfig := &json.Config{
    // 缓存设置
    EnableCache:      true,             // 启用缓存
    MaxCacheSize:     5000,             // 缓存条目数
    CacheTTL:         10 * time.Minute, // 缓存过期时间
    
    // 大小限制
    MaxJSONSize:      50 * 1024 * 1024, // 50MB JSON 大小限制
    MaxPathDepth:     200,              // 路径深度限制
    MaxBatchSize:     2000,             // 批量操作大小限制
    
    // 并发设置
    MaxConcurrency:   100,   // 最大并发数
    ParallelThreshold: 20,   // 并行处理阈值
    
    // 处理选项
    EnableValidation: true,  // 启用验证
    StrictMode:       false, // 非严格模式
    CreatePaths:      true,  // 自动创建路径
    CleanupNulls:     true,  // 清理空值
}

processor3 := json.New(customConfig)
defer processor3.Close()

// 4. 预定义配置
secureProcessor := json.New(json.HighSecurityConfig())
largeDataProcessor := json.New(json.LargeDataConfig())
```

### 操作选项

```go
opts := &json.ProcessorOptions{
    CreatePaths:     true,  // 自动创建路径
    CleanupNulls:    true,  // 清理空值
    CompactArrays:   true,  // 压缩数组
    ContinueOnError: false, // 遇错继续
    MaxDepth:        50,    // 最大深度
}

result, _ := json.Get(data, "path", opts)
```

### 性能监控

```go
processor := json.New(json.DefaultConfig())
defer processor.Close()

// 执行操作后获取统计
stats := processor.GetStats()
fmt.Printf("总操作数: %d\n", stats.OperationCount)
fmt.Printf("缓存命中率: %.2f%%\n", stats.HitRatio*100)
fmt.Printf("缓存内存使用: %d bytes\n", stats.CacheMemory)

// 获取健康状态
health := processor.GetHealthStatus()
fmt.Printf("系统健康状态: %v\n", health.Healthy)
```

---

## 📁 文件操作

### 基础文件操作

```go
// 从文件加载 JSON
data, err := json.LoadFromFile("config.json")
if err != nil {
    log.Printf("文件加载失败: %v", err)
    return
}

// 保存到文件（美化格式）
err = json.SaveToFile("output_pretty.json", data, true)

// 保存到文件（压缩格式）
err = json.SaveToFile("output.json", data, false)

// 从 Reader 加载
file, err := os.Open("large_data.json")
if err != nil {
    log.Fatal(err)
}
defer file.Close()

data, err = json.LoadFromReader(file)

// 保存到 Writer
var buffer bytes.Buffer
err = json.SaveToWriter(&buffer, data, true)
```

### 批量文件处理

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
        log.Printf("加载 %s 失败: %v", filename, err)
        continue
    }
    
    configName := strings.TrimSuffix(filename, ".json")
    allConfigs[configName] = config
}

// 保存合并后的配置
err := json.SaveToFile("merged_config.json", allConfigs, true)
```

---

## 🛡️ 数据验证

### JSON Schema 验证

```go
// 定义 JSON Schema
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

// 验证数据
testData := `{
    "name": "Alice",
    "age": 25,
    "email": "alice@example.com"
}`

processor := json.New(json.DefaultConfig())
errors, err := processor.ValidateSchema(testData, schema)
if len(errors) > 0 {
    fmt.Println("验证错误:")
    for _, validationErr := range errors {
        fmt.Printf("  路径 %s: %s\n", validationErr.Path, validationErr.Message)
    }
} else {
    fmt.Println("数据验证通过")
}
```

### 安全配置

```go
// 安全配置
secureConfig := &json.Config{
    MaxJSONSize:       10 * 1024 * 1024, // 10MB JSON 大小限制
    MaxPathDepth:      50,               // 路径深度限制
    MaxNestingDepth:   100,              // 对象嵌套深度限制
    MaxArrayElements:  10000,            // 数组元素数量限制
    MaxObjectKeys:     1000,             // 对象键数量限制
    ValidateInput:     true,             // 输入验证
    EnableValidation:  true,             // 启用验证
    StrictMode:        true,             // 严格模式
}

processor := json.New(secureConfig)
defer processor.Close()
```

---

## 🎯 应用场景

### 示例 - API 响应处理

```go
// 典型的 REST API 响应
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

// 快速提取关键信息
status, _ := json.GetString(apiResponse, "status")
code, _ := json.GetInt(apiResponse, "code")

// 批量提取用户信息
userNames, _ := json.Get(apiResponse, "data.users.profile.name")
// 结果: ["Alice Johnson"]

userEmails, _ := json.Get(apiResponse, "data.users.profile.email")
// 结果: ["alice@example.com"]

// 扁平化提取所有权限
allPermissions, _ := json.Get(apiResponse, "data.users{flat:permissions}")
// 结果: ["read", "write", "admin"]

// 获取分页信息
totalUsers, _ := json.GetInt(apiResponse, "data.pagination.total")
currentPage, _ := json.GetInt(apiResponse, "data.pagination.page")

fmt.Printf("状态: %s (代码: %d)\n", status, code)
fmt.Printf("用户总数: %d, 当前页: %d\n", totalUsers, currentPage)
```

### 示例 - 配置文件管理

```go
// 多环境配置文件
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

// 类型安全的配置获取
dbHost := json.GetStringWithDefault(configJSON, "environments.production.database.host", "localhost")
dbPort := json.GetIntWithDefault(configJSON, "environments.production.database.port", 5432)
cacheEnabled := json.GetBoolWithDefault(configJSON, "environments.production.cache.enabled", false)

fmt.Printf("生产环境数据库: %s:%d\n", dbHost, dbPort)
fmt.Printf("缓存启用: %v\n", cacheEnabled)

// 动态配置更新
updates := map[string]any{
"app.version": "1.2.4",
"environments.production.cache.ttl": 10800, // 3小时
}

newConfig, _ := json.SetMultiple(configJSON, updates)
```

### 示例 - 数据分析处理

```go
// 日志和监控数据
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

// 提取所有事件类型
eventTypes, _ := json.Get(analyticsData, "events.type")
// 结果: ["request", "error"]

// 提取所有状态码
statusCodes, _ := json.Get(analyticsData, "events.status_code")
// 结果: [200, 500]

// 提取所有响应时间
responseTimes, _ := json.GetTyped[[]float64](analyticsData, "events.response_time")
// 结果: [45, 5000]

// 计算平均响应时间
times := responseTimes
var total float64
for _, t := range times {
    total += t
}

avgTime := total / float64(len(times))
fmt.Printf("平均响应时间: %.2f ms\n", avgTime)
```

---

## 📋 API 参考

### 核心方法

#### 数据获取

```go
// 基础获取
json.Get(data, path) (any, error)
json.GetString(data, path) (string, error)
json.GetInt(data, path) (int, error)
json.GetBool(data, path) (bool, error)
json.GetFloat64(data, path) (float64, error)
json.GetArray(data, path) ([]any, error)
json.GetObject(data, path) (map[string]any, error)

// 类型安全获取
json.GetTyped[T](data, path) (T, error)

// 带默认值获取
json.GetStringWithDefault(data, path, defaultValue) string
json.GetIntWithDefault(data, path, defaultValue) int
json.GetBoolWithDefault(data, path, defaultValue) bool

// 批量获取
json.GetMultiple(data, paths) (map[string]any, error)
```

#### 数据修改

```go
// 基础设置 - 改进的错误处理
// 返回：成功时 (修改后的数据, nil)，失败时 (原始数据, error)
json.Set(data, path, value) (string, error)
json.SetWithAdd(data, path, value) (string, error)

// 批量设置 - 同样的改进行为
json.SetMultiple(data, updates) (string, error)
json.SetMultipleWithAdd(data, updates) (string, error)
```

#### 数据删除

```go
json.Delete(data, path) (string, error)
json.DeleteWithCleanNull(data, path) (string, error)
```

#### 数据迭代

```go
// 基础迭代方法
json.Foreach(data, callback) error
json.ForeachReturn(data, callback) (string, error)

// 路径迭代 - 只读遍历指定路径数据
json.ForeachWithPath(data, path, callback) error

// 嵌套迭代 - 防止状态冲突
json.ForeachNested(data, callback) error
json.ForeachReturnNested(data, callback) (string, error)

// IterableValue 嵌套方法 - 在迭代回调中使用
item.ForeachReturnNested(path, callback) error
```

**使用场景对比：**

| 方法                | 返回值               | 数据修改  | 遍历范围   | 使用场景       |
|-------------------|-------------------|-------|--------|------------|
| `Foreach`         | `error`           | ❌ 不支持 | 完整JSON | 只读遍历整个JSON |
| `ForeachWithPath` | `error`           | ❌ 不支持 | 指定路径   | 只读遍历JSON子集 |
| `ForeachReturn`   | `(string, error)` | ✅ 支持  | 完整JSON | 数据修改、批量更新  |

### 文件操作方法

```go
// 文件读写
json.LoadFromFile(filename, ...opts) (string, error)
json.SaveToFile(filename, data, pretty) error

// 流式操作
json.LoadFromReader(reader, ...opts) (string, error)
json.SaveToWriter(writer, data, pretty) error
```

### 验证方法

```go
// Schema 验证
processor.ValidateSchema(data, schema) ([]ValidationError, error)

// 基础验证
json.Valid(data) bool
```

### 处理器方法

```go
// 创建处理器
json.New(config) *Processor
json.DefaultConfig() *Config

// 缓存操作
processor.WarmupCache(data, paths) (*WarmupResult, error)
processor.ClearCache()

// 统计信息
processor.GetStats() *Stats
processor.GetHealthStatus() *HealthStatus
```

### 错误处理策略

```go
// 推荐的错误处理方式
result, err := json.GetString(data, "user.name")
if err != nil {
    log.Printf("获取用户名失败: %v", err)
    return "", err // 使用默认值或返回错误
}

// 使用带默认值的方法
name := json.GetStringWithDefault(data, "user.name", "Anonymous")
```

---

## 📚 最佳实践

### 性能优化建议

1. **启用缓存** - 对于重复操作，启用缓存可显著提升性能
2. **批量操作** - 使用 `GetMultiple` 和 `SetMultiple` 进行批量处理
3. **路径预热** - 对常用路径使用 `WarmupCache` 预热
4. **合理配置** - 根据实际需求调整缓存大小和TTL

### 安全使用指南

1. **输入验证** - 启用 `ValidateInput` 验证输入数据
2. **大小限制** - 设置合理的 `MaxJSONSize` 和 `MaxPathDepth`
3. **Schema验证** - 对关键数据使用 JSON Schema 验证
4. **错误处理** - 始终检查返回的错误信息

### 内存管理

1. **处理器生命周期** - 始终调用 `processor.Close()` 清理资源
2. **避免内存泄漏** - 不要不必要地持有大型 JSON 字符串的引用
3. **批量大小控制** - 为批量操作设置适当的 `MaxBatchSize`
4. **缓存管理** - 监控缓存内存使用并根据需要调整大小

### 线程安全

1. **默认处理器** - 全局默认处理器是线程安全的
2. **自定义处理器** - 每个处理器实例都是线程安全的
3. **并发操作** - 多个 goroutine 可以安全地使用同一个处理器
4. **资源共享** - 处理器可以在 goroutine 之间安全共享

---

## 💡 示例与资源

### 📁 示例代码

仓库包含演示各种功能和用例的综合示例：

#### 基础示例

- **[基础用法](../examples/basic/)** - 基本操作和入门指南
- **[JSON Get 操作](../examples/json_get/)** - 使用不同路径表达式的数据检索示例
- **[JSON Set 操作](../examples/json_set/)** - 数据修改和批量更新
- **[JSON Delete 操作](../examples/json_delete/)** - 数据删除和清理操作

#### 高级示例

- **[文件操作](../examples/file_operations/)** - 文件 I/O、批量处理和流操作
- **[JSON 迭代](../examples/json_iteration/)** - 数据迭代和遍历模式
- **[扁平化提取](../examples/flat_extraction/)** - 复杂数据提取和扁平化
- **[JSON 编码](../examples/json_encode/)** - 自定义编码配置和格式化

#### 配置示例

- **[配置管理](../examples/configuration/)** - 处理器配置和优化
- **[兼容性示例](../examples/compatibility/)** - 直接替换演示

---

## 📄 License

该项目遵循的是 MIT 许可协议——详情请参阅 [许可证](../LICENSE) 文件。

## 🤝 贡献

欢迎贡献！请随时提交拉取请求。对于重大更改，请先打开一个问题来讨论您想要更改的内容。

## 🌟 Star 历史

如果您觉得这个项目有用，请考虑给它一个 star！⭐

---

**由 CyberGoDev 团队用 ❤️ 制作**