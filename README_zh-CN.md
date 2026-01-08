# 🚀 cybergodev/json - 高性能 Go JSON 处理库

[![Go Version](https://img.shields.io/badge/Go-1.24+-blue.svg)](https://golang.org)
[![pkg.go.dev](https://pkg.go.dev/badge/github.com/cybergodev/json.svg)](https://pkg.go.dev/github.com/cybergodev/json)
[![MIT license](https://img.shields.io/badge/license-MIT-brightgreen.svg)](https://opensource.org/licenses/MIT)
[![Performance](https://img.shields.io/badge/performance-high%20performance-green.svg)](https://github.com/cybergodev/json)
[![Thread Safe](https://img.shields.io/badge/thread%20safe-yes-brightgreen.svg)](https://github.com/cybergodev/json)

> 一个高性能、功能丰富的 Go JSON 处理库，100% 兼容 `encoding/json`，提供强大的路径操作、类型安全、性能优化和丰富的高级功能。

#### **[📖 English Documentation](README.md)** - 英文文档

---

## 📚 目录

- [📖 概述](#-概述)
- [📋 基本路径语法](#-基本路径语法)
- [🚀 快速开始](#-快速开始)
- [🏆 核心功能](#-核心功能)
- [🔧 配置选项](#-配置选项)
- [📁 文件操作](#-文件操作)
- [🎯 使用场景](#-使用场景)
- [🌐 示例与资源](#-示例与资源)

---

## 📖 概述

**`cybergodev/json`** 是一个高性能的 Go JSON 处理库，与标准 `encoding/json` 包保持 100% 兼容，同时提供强大的路径操作、类型安全、性能优化和丰富的高级功能。

### 🏆 核心优势

- **🔄 完全兼容** - 100% 兼容标准 `encoding/json`，零学习成本，直接替换
- **🎯 强大路径** - 支持复杂路径表达式，一行代码完成复杂数据操作
- **🚀 高性能** - 智能缓存、并发安全、内存优化，生产级性能
- **🛡️ 类型安全** - 泛型支持、编译时检查、智能类型转换
- **🔧 功能丰富** - 批量操作、数据验证、文件操作、性能监控
- **🏗️ 生产就绪** - 线程安全、错误处理、安全配置、监控指标

### 🎯 使用场景

- **🌐 API 数据处理** - 快速提取和转换复杂响应数据
- **⚙️ 配置管理** - 动态配置读取和批量更新
- **📊 数据分析** - 大量 JSON 数据的统计和分析
- **🔄 微服务通信** - 服务间数据交换和格式转换
- **📝 日志处理** - 结构化日志的解析和分析

### 📚 更多示例与文档

- **[📁 示例代码](examples)** - 三个涵盖所有功能的完整示例
  - **[基本用法](examples/1.basic_usage.go)** - 快速入门和基础操作
  - **[高级功能](examples/2.advanced_features.go)** - 复杂查询和嵌套操作
  - **[生产就绪](examples/3.production_ready.go)** - 生产环境模式和最佳实践
- **[📖 兼容性](docs/COMPATIBILITY.md)** - 兼容性指南和迁移信息
- **[🔄 快速参考](docs/QUICK_REFERENCE.md)** - 常用功能快速参考指南

---

## 📋 基本路径语法

| 语法               | 描述        | 示例                 | 结果                   |
|-------------------|-------------|---------------------|------------------------|
| `.`               | 属性访问     | `user.name`         | 获取用户名属性          |
| `[n]`             | 数组索引     | `users[0]`          | 获取第一个用户          |
| `[-n]`            | 负数索引     | `users[-1]`         | 获取最后一个用户        |
| `[start:end:step]`| 数组切片     | `users[1:3]`        | 获取索引 1-2 的用户     |
| `{field}`         | 批量提取     | `users{name}`       | 提取所有用户名          |
| `{flat:field}`    | 扁平化提取   | `users{flat:skills}`| 扁平化提取所有技能      |

## 🚀 快速开始

### 安装

```bash
go get github.com/cybergodev/json
```

### 基本用法

```go
package main

import (
    "fmt"
    "github.com/cybergodev/json"
)

func main() {
    // 1. 与标准库完全兼容
    data := map[string]any{"name": "Alice", "age": 25}
    jsonBytes, err := json.Marshal(data)

    var result map[string]any
    json.Unmarshal(jsonBytes, &result)

    // 2. 强大的路径操作（增强功能）
    jsonStr := `{"user":{"profile":{"name":"Alice","age":25}}}`

    name, err := json.GetString(jsonStr, "user.profile.name")
    fmt.Println(name) // "Alice"

    age, err := json.GetInt(jsonStr, "user.profile.age")
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
names, err := json.Get(complexData, "users{name}")
// 结果: ["Alice", "Bob"]

// 获取所有技能（扁平化）
skills, err := json.Get(complexData, "users{flat:skills}")
// 结果: ["Go", "Python", "Java", "React"]

// 批量获取多个值
paths := []string{"users[0].name", "users[1].name", "users{active}"}
results, err := json.GetMultiple(complexData, paths)
```

---

## ⚡ 核心功能

### 数据检索

```go
// 基本检索
json.Get(data, "user.name")          // 获取任意类型
json.GetString(data, "user.name")    // 获取字符串
json.GetInt(data, "user.age")        // 获取整数
json.GetFloat64(data, "user.score")  // 获取浮点数
json.GetBool(data, "user.active")    // 获取布尔值
json.GetArray(data, "user.tags")     // 获取数组
json.GetObject(data, "user.profile") // 获取对象

// 类型安全检索
json.GetTyped[string](data, "user.name") // 泛型类型安全
json.GetTyped[[]User](data, "users")     // 自定义类型

// 带默认值的检索
json.GetWithDefault(data, "user.name", "Anonymous")
json.GetStringWithDefault(data, "user.name", "Anonymous")
json.GetIntWithDefault(data, "user.age", 0)
json.GetFloat64WithDefault(data, "user.score", 0.0)
json.GetBoolWithDefault(data, "user.active", false)
json.GetArrayWithDefault(data, "user.tags", []any{})
json.GetObjectWithDefault(data, "user.profile", map[string]any{})

// 批量检索
paths := []string{"user.name", "user.age", "user.email"}
results, err := json.GetMultiple(data, paths)
```

### 数据修改

```go
// 基本设置 - 成功时返回修改后的数据，失败时返回原始数据
data := `{"user":{"name":"Bob","age":25}}`
result, err := json.Set(data, "user.name", "Alice")
// result => {"user":{"name":"Alice","age":25}}

// 自动创建路径
data := `{}`
result, err := json.SetWithAdd(data, "user.name", "Alice")
// result => {"user":{"name":"Alice"}}

// 批量设置
updates := map[string]any{
    "user.name": "Bob",
    "user.age":  30,
    "user.active": true,
}
result, err := json.SetMultiple(data, updates)
result, err := json.SetMultipleWithAdd(data, updates) // 自动创建路径
// 相同行为：成功 = 修改后的数据，失败 = 原始数据
```

### 数据删除

```go
json.Delete(data, "user.temp") // 删除字段
json.DeleteWithCleanNull(data, "user.temp") // 删除并清理空值
```

### 数据迭代

```go
// 基本迭代 - 只读遍历
json.Foreach(data, func (key any, item *json.IterableValue) {
    name := item.GetString("name")
    fmt.Printf("Key: %v, Name: %s\n", key, name)
})

// 高级迭代变体
json.ForeachNested(data, callback)           // 嵌套安全迭代
json.ForeachWithIterator(data, callback)     // 带迭代器访问
json.ForeachWithPath(data, "users", callback) // 迭代特定路径

// 迭代并返回修改的 JSON - 支持数据修改
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

### JSON 编码与格式化

```go
// 标准编码（100% 兼容 encoding/json）
bytes, err := json.Marshal(data)
err = json.Unmarshal(bytes, &target)
bytes, err := json.MarshalIndent(data, "", "  ")

// 带配置的高级编码
config := &json.EncodeConfig{
    Pretty:       true,
    SortKeys:     true,
    EscapeHTML:   false,
}
jsonStr, err := json.Encode(data, config)
jsonStr, err := json.EncodePretty(data, config)
jsonStr, err := json.EncodeCompact(data, config)

// 格式化操作
pretty, err := json.FormatPretty(jsonStr)
compact, err := json.FormatCompact(jsonStr)

// 缓冲区操作（兼容 encoding/json）
json.Compact(dst, src)
json.Indent(dst, src, prefix, indent)
json.HTMLEscape(dst, src)
```

### 文件操作

```go
// 加载和保存 JSON 文件
jsonStr, err := json.LoadFromFile("data.json")
err = json.SaveToFile("output.json", data, true) // 美化格式

// 文件的 Marshal/Unmarshal
err = json.MarshalToFile("user.json", user)
err = json.MarshalToFile("user_pretty.json", user, true)
err = json.UnmarshalFromFile("user.json", &loadedUser)

// 流操作
data, err := processor.LoadFromReader(reader)
err = processor.SaveToWriter(writer, data, true)
```

### 类型转换与工具

```go
// 安全类型转换
intVal, ok := json.ConvertToInt(value)
floatVal, ok := json.ConvertToFloat64(value)
boolVal, ok := json.ConvertToBool(value)
strVal := json.ConvertToString(value)

// 泛型类型转换
result, ok := json.UnifiedTypeConversion[int](value)
result, err := json.TypeSafeConvert[string](value)

// JSON 比较和合并
equal, err := json.CompareJson(json1, json2)
merged, err := json.MergeJson(json1, json2)
copy, err := json.DeepCopy(data)
```

### 处理器管理

```go
// 使用配置创建处理器
config := &json.Config{
    EnableCache:      true,
    MaxCacheSize:     5000,
    MaxJSONSize:      50 * 1024 * 1024,
    MaxConcurrency:   100,
    EnableValidation: true,
}
processor := json.New(config)
defer processor.Close()

// 处理器操作
result, err := processor.Get(jsonStr, path)
stats := processor.GetStats()
health := processor.GetHealthStatus()
processor.ClearCache()

// 缓存预热
paths := []string{"user.name", "user.age", "user.profile"}
warmupResult, err := processor.WarmupCache(jsonStr, paths)

// 全局处理器管理
json.SetGlobalProcessor(processor)
json.ShutdownGlobalProcessor()
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

// 多级嵌套提取
allMembers, err := json.Get(complexData, "company.departments{teams}{flat:members}")
// 结果: [Alice的数据, Bob的数据]

// 提取特定字段
allNames, err := json.Get(complexData, "company.departments{teams}{flat:members}{name}")
// 结果: ["Alice", "Bob"]

// 扁平化技能提取
allSkills, err := json.Get(complexData, "company.departments{teams}{flat:members}{flat:skills}")
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
first, err := json.GetInt(arrayData, "numbers[0]")       // 1
last, err := json.GetInt(arrayData, "numbers[-1]")       // 10 (负索引)
slice, err := json.Get(arrayData, "numbers[1:4]")        // [2, 3, 4]
everyOther, err := json.Get(arrayData, "numbers[::2]")   // [1, 3, 5, 7, 9]
everyOther, err := json.Get(arrayData, "numbers[::-2]")  // [10 8 6 4 2]

// 嵌套数组访问
ages, err := json.Get(arrayData, "users{age}") // [25, 30]
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
    ContinueOnError: false, // 遇到错误时继续
    MaxDepth:        50,    // 最大深度
}

result, err := json.Get(data, "path", opts)
```

### 性能监控

```go
processor := json.New(json.DefaultConfig())
defer processor.Close()

// 操作后获取统计信息
stats := processor.GetStats()
fmt.Printf("总操作数: %d\n", stats.OperationCount)
fmt.Printf("缓存命中率: %.2f%%\n", stats.HitRatio*100)
fmt.Printf("缓存内存使用: %d 字节\n", stats.CacheMemory)

// 获取健康状态
health := processor.GetHealthStatus()
fmt.Printf("系统健康状态: %v\n", health.Healthy)
```

---

## 📁 文件操作

### 基本文件操作

```go
// 从文件加载 JSON
data, err := json.LoadFromFile("example.json")

// 保存到文件（美化格式）
err = json.SaveToFile("output_pretty.json", data, true)

// 保存到文件（紧凑格式）
err = json.SaveToFile("output.json", data, false)

// 从 Reader 加载
file, err := os.Open("large_data.json")
if err != nil {
    log.Fatal(err)
}
defer file.Close()

data, err := json.LoadFromReader(file)

// 保存到 Writer
var buffer bytes.Buffer
err = json.SaveToWriter(&buffer, data, true)
```

### Marshal/Unmarshal 文件操作

```go
// 将数据 Marshal 到文件（默认紧凑格式）
user := map[string]any{
    "name": "Alice",
    "age":  30,
    "email": "alice@example.com",
}
err := json.MarshalToFile("user.json", user)

// 将数据 Marshal 到文件（美化格式）
err = json.MarshalToFile("user_pretty.json", user, true)

// 从文件 Unmarshal 数据
var loadedUser map[string]any
err = json.UnmarshalFromFile("user.json", &loadedUser)

// 也适用于结构体
type User struct {
    Name  string `json:"name"`
    Age   int    `json:"age"`
    Email string `json:"email"`
}

var person User
err = json.UnmarshalFromFile("user.json", &person)

// 使用处理器进行高级选项
processor := json.New()
defer processor.Close()

err = processor.MarshalToFile("advanced.json", user, true)
err = processor.UnmarshalFromFile("advanced.json", &loadedUser, opts...)
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

// 保存合并的配置
err := json.SaveToFile("merged_config.json", allConfigs, true)
```

---

### 安全配置

```go
// 安全配置
secureConfig := &json.Config{
    MaxJSONSize:       10 * 1024 * 1024,    // 10MB JSON 大小限制
    MaxPathDepth:      50,                  // 路径深度限制
    MaxNestingDepth:   100,                 // 对象嵌套深度限制
    MaxArrayElements:  10000,               // 数组元素数量限制
    MaxObjectKeys:     1000,                // 对象键数量限制
    ValidateInput:     true,                // 输入验证
    EnableValidation:  true,                // 启用验证
    StrictMode:        true,                // 严格模式
}

processor := json.New(secureConfig)
defer processor.Close()
```

---

## 🎯 使用场景

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
status, err := json.GetString(apiResponse, "status")
// 结果: success

code, err := json.GetInt(apiResponse, "code")
// 结果: 200

// 获取分页信息
totalUsers, err := json.GetInt(apiResponse, "data.pagination.total")
// 结果: 25

currentPage, err := json.GetInt(apiResponse, "data.pagination.page")
// 结果: 1

// 批量提取用户信息
userNames, err := json.Get(apiResponse, "data.users.profile.name")
// 结果: ["Alice Johnson"]

userEmails, err := json.Get(apiResponse, "data.users.profile.email")
// 结果: ["alice@example.com"]

// 扁平化提取所有权限
allPermissions, err := json.Get(apiResponse, "data.users{flat:permissions}")
// 结果: ["read", "write", "admin"]
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

// 类型安全的配置检索
dbHost := json.GetStringWithDefault(configJSON, "environments.production.database.host", "localhost")
dbPort := json.GetIntWithDefault(configJSON, "environments.production.database.port", 5432)
cacheEnabled := json.GetBoolWithDefault(configJSON, "environments.production.cache.enabled", false)

fmt.Printf("生产数据库: %s:%d\n", dbHost, dbPort)
fmt.Printf("缓存启用: %v\n", cacheEnabled)

// 动态配置更新
updates := map[string]any{
    "app.version": "1.2.4",
    "environments.production.cache.ttl": 10800, // 3 小时
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

## Set 操作 - 数据安全保证

所有 Set 操作都遵循**默认安全**模式，确保您的数据永远不会被损坏：

```go
// ✅ 成功：返回修改后的数据
result, err := json.Set(data, "user.name", "Alice")
if err == nil {
    // result 包含成功修改的 JSON
    fmt.Println("数据已更新:", result)
}

// ❌ 失败：返回原始未修改的数据
result, err := json.Set(data, "invalid[path", "value")
if err != nil {
    // result 仍然包含有效的原始数据
    // 您的原始数据永远不会被损坏
    fmt.Printf("设置失败: %v\n", err)
    fmt.Println("原始数据已保留:", result)
}
```

**主要优势**：
- 🔒 **数据完整性**：错误时原始数据永不损坏
- ✅ **安全回退**：始终有有效的 JSON 可以使用
- 🎯 **可预测性**：所有操作的一致行为

---

## 💡 示例与资源

### 📁 示例代码

- **[基本用法](examples/1.basic_usage.go)** - examples/1.basic_usage.go 
- **[高级功能](examples/2.advanced_features.go)** - examples/2.advanced_features.go 
- **[生产就绪](examples/3.production_ready.go)** - examples/3.production_ready.go 


### 📖 其他资源

- **[兼容性指南](docs/COMPATIBILITY.md)** - `encoding/json` 的直接替换
- **[快速参考](docs/QUICK_REFERENCE.md)** - 常用操作速查表
- **[API 文档](https://pkg.go.dev/github.com/cybergodev/json)** - 完整的 API 参考

---

## 📄 许可证

本项目采用 MIT 许可证 - 详情请参阅 [LICENSE](LICENSE) 文件。

---

## 🤝 贡献

欢迎贡献！请随时提交 Pull Request。对于重大更改，请先开启 issue 讨论您想要更改的内容。

## 🌟 Star 历史

如果您觉得这个项目有用，请考虑给它一个 star！⭐

---

**由 CyberGoDev 团队用 ❤️ 制作**