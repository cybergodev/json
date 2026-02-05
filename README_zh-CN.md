# 🚀 cybergodev/json - 高性能 Go JSON 处理库

[![Go Version](https://img.shields.io/badge/Go-1.24+-blue.svg)](https://golang.org)
[![pkg.go.dev](https://pkg.go.dev/badge/github.com/cybergodev/json.svg)](https://pkg.go.dev/github.com/cybergodev/json)
[![MIT license](https://img.shields.io/badge/license-MIT-brightgreen.svg)](https://opensource.org/licenses/MIT)
[![Performance](https://img.shields.io/badge/performance-high%20performance-green.svg)](https://github.com/cybergodev/json)
[![Thread Safe](https://img.shields.io/badge/thread%20safe-yes-brightgreen.svg)](https://github.com/cybergodev/json)

> 一个高性能、功能丰富的 Go JSON 处理库，100% 兼容 `encoding/json`，提供强大的路径操作、类型安全、性能优化和丰富的高级特性。

#### **[📖 English Documentation](README.md)** - User guide

---

## 🏆 核心优势

- **🔄 完全兼容** - 100% 兼容标准库 `encoding/json`，零学习成本，直接替换
- **🎯 强大路径** - 支持复杂路径表达式，一行代码完成复杂数据操作
- **🚀 高性能** - 智能缓存、并发安全、内存优化，生产级性能
- **🛡️ 类型安全** - 泛型支持、编译时检查、智能类型转换
- **🔧 功能丰富** - 批量操作、数据验证、文件操作、性能监控
- **🏗️ 生产就绪** - 线程安全、错误处理、安全配置、监控指标

### 🎯 应用场景

- **🌐 API 数据处理** - 快速提取和转换复杂响应数据
- **⚙️ 配置管理** - 动态配置读取和批量更新
- **📊 数据分析** - 大量 JSON 数据的统计分析
- **🔄 微服务通信** - 服务间数据交换和格式转换
- **📝 日志处理** - 结构化日志的解析和分析

---

## 📋 基础路径语法

| 语法               | 描述         | 示例              | 结果                     |
|--------------------|--------------|-------------------|----------------------------|
| `.`                | 属性访问     | `user.name`       | 获取用户的 name 属性       |
| `[n]`              | 数组索引     | `users[0]`        | 获取第一个用户             |
| `[-n]`             | 负索引       | `users[-1]`       | 获取最后一个用户           |
| `[start:end:step]` | 数组切片     | `users[1:3]`      | 获取索引 1-2 的用户        |
| `{field}`          | 批量提取     | `users{name}`     | 提取所有用户名             |
| `{flat:field}`     | 扁平化提取   | `users{flat:skills}` | 扁平化提取所有技能       |

## 🚀 快速开始

### 安装

```bash
go get github.com/cybergodev/json
```

### 基础用法

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

### 路径操作示例

```go
// 复杂的 JSON 数据
complexData := `{
  "users": [
    {"name": "Alice", "skills": ["Go", "Python"], "active": true},
    {"name": "Bob", "skills": ["Java", "React"], "active": false}
  ]
}`

// 获取所有用户名
names, err := json.Get(complexData, "users{name}")
if err != nil {
    fmt.Printf("Error: %v\n", err)
    return
}
// 结果: ["Alice", "Bob"]

// 获取所有技能（扁平化）
skills, err := json.Get(complexData, "users{flat:skills}")
if err != nil {
    fmt.Printf("Error: %v\n", err)
    return
}
// 结果: ["Go", "Python", "Java", "React"]

// 批量获取多个值
paths := []string{"users[0].name", "users[1].name", "users{active}"}
results, err := json.GetMultiple(complexData, paths)
if err != nil {
    fmt.Printf("Error: %v\n", err)
    return
}
```


---

## ⚡ 核心功能

### 数据获取

```go
// 基础获取
json.Get(data, "user.name")          // 获取任意类型
json.GetString(data, "user.name")    // 获取字符串
json.GetInt(data, "user.age")        // 获取整数
json.GetFloat64(data, "user.score")  // 获取 float64
json.GetBool(data, "user.active")    // 获取布尔值
json.GetArray(data, "user.tags")     // 获取数组
json.GetObject(data, "user.profile") // 获取对象

// 类型安全获取
json.GetTyped[string](data, "user.name") // 泛型类型安全
json.GetTyped[[]User](data, "users")     // 自定义类型

// 带默认值的获取
json.GetWithDefault(data, "user.name", "Anonymous")
json.GetStringWithDefault(data, "user.name", "Anonymous")
json.GetIntWithDefault(data, "user.age", 0)
json.GetFloat64WithDefault(data, "user.score", 0.0)
json.GetBoolWithDefault(data, "user.active", false)
json.GetArrayWithDefault(data, "user.tags", []any{})
json.GetObjectWithDefault(data, "user.profile", map[string]any{})

// 批量获取
paths := []string{"user.name", "user.age", "user.email"}
results, err := json.GetMultiple(data, paths)
```

### 数据修改

```go
// 基础设置 - 成功返回修改后的数据，失败返回原始数据
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
// 相同行为: 成功 = 修改后的数据，失败 = 原始数据
```

### 数据删除

```go
json.Delete(data, "user.temp") // 删除字段
json.DeleteWithCleanNull(data, "user.temp") // 删除并清理 null 值
```

### 数据迭代

```go
// 基础迭代 - 只读遍历
json.Foreach(data, func (key any, item *json.IterableValue) {
    name := item.GetString("name")
    fmt.Printf("Key: %v, Name: %s\n", key, name)
})

// 高级迭代变体
json.ForeachNested(data, callback)                            // 递归遍历所有嵌套层级
json.ForeachWithPath(data, "data.users", callback)            // 迭代特定路径
json.ForeachReturn(data, callback)                            // 修改并返回修改后的 JSON

// 带控制流的迭代 - 支持提前终止
json.ForeachWithPathAndControl(data, "data.users", func(key any, value any) json.IteratorControl {
    // 处理每个项目
    if shouldStop {
        return json.IteratorBreak  // 停止迭代
    }
    return json.IteratorContinue  // 继续下一项
})

// 带路径信息跟踪的迭代
json.ForeachWithPathAndIterator(data, "data.users", func(key any, item *json.IterableValue, currentPath string) json.IteratorControl {
    name := item.GetString("name")
    fmt.Printf("用户在 %s: %s\n", currentPath, name)
    return json.IteratorContinue
})

// 完整的 Foreach 函数列表：
// - Foreach(data, callback) - 基础迭代
// - ForeachNested(data, callback) - 递归迭代
// - ForeachWithPath(data, path, callback) - 特定路径迭代
// - ForeachWithPathAndControl(data, path, callback) - 带控制流
// - ForeachWithPathAndIterator(data, path, callback) - 带路径信息
// - ForeachReturn(data, callback) - 修改并返回
```

### JSON 编码与格式化

```go
// 标准编码（100% 兼容 encoding/json）
bytes, err := json.Marshal(data)
err = json.Unmarshal(bytes, &target)
bytes, err := json.MarshalIndent(data, "", "  ")

// 高级编码配置
config := &json.EncodeConfig{
    Pretty:       true,
    SortKeys:     true,
    EscapeHTML:   false,
    MaxDepth:     10,  // 可选: 最大编码深度（覆盖默认值 100）
}
jsonStr, err := json.Encode(data, config)           // 使用自定义配置编码（config 可选，为 nil 时使用默认配置）
jsonStr, err := json.EncodePretty(data, config)     // 美化格式编码

// 格式化操作
pretty, err := json.FormatPretty(jsonStr)
compact, err := json.FormatCompact(jsonStr)

// 打印操作（直接输出到标准输出）
// 智能 JSON 检测：string/[]byte 输入会先检查有效性
json.Print(data)           // 以压缩格式打印 JSON 到标准输出
json.PrintPretty(data)     // 以美化格式打印 JSON 到标准输出

// 打印示例
data := map[string]any{
    "monitoring": true,
    "database": map[string]any{
        "name": "myDb",
        "port": "5432",
        "ssl":  true,
    },
}

// 打印 Go 值为压缩 JSON
json.Print(data)
// 输出: {"monitoring":true,"database":{"name":"myDb","port":"5432","ssl":true}}

// 打印 Go 值为美化 JSON
json.PrintPretty(data)
// 输出:
// {
//   "database": {
//     "name": "myDb",
//     "port": "5432",
//     "ssl": true
//   },
//   "monitoring": true
// }

// 直接打印 JSON 字符串（无双重编码）
jsonStr := `{"name":"John","age":30}`
json.Print(jsonStr)
// 输出: {"name":"John","age":30}

// 缓冲区操作（encoding/json 兼容）
json.Compact(dst, src)
json.Indent(dst, src, prefix, indent)
json.HTMLEscape(dst, src)

// 带处理器选项的高级缓冲区操作
json.CompactBuffer(dst, src, opts)   // 使用自定义处理器选项
json.IndentBuffer(dst, src, prefix, indent, opts)
json.HTMLEscapeBuffer(dst, src, opts)

// 高级编码方法
// EncodeStream - 将多个值编码为 JSON 数组流
users := []map[string]any{
    {"name": "Alice", "age": 25},
    {"name": "Bob", "age": 30},
}
stream, err := json.EncodeStream(users, false)  // 压缩格式

// EncodeBatch - 将多个键值对编码为 JSON 对象
pairs := map[string]any{
    "user1": map[string]any{"name": "Alice", "age": 25},
    "user2": map[string]any{"name": "Bob", "age": 30},
}
batch, err := json.EncodeBatch(pairs, true)  // 美化格式

// EncodeFields - 仅编码结构体的指定字段
type User struct {
    Name  string `json:"name"`
    Age   int    `json:"age"`
    Email string `json:"email"`
}
user := User{Name: "Alice", Age: 25, Email: "alice@example.com"}
fields, err := json.EncodeFields(user, []string{"name", "age"}, true)
// 输出: {"name":"Alice","age":25}
```

### 文件操作

```go
// 加载和保存 JSON 文件
jsonStr, err := json.LoadFromFile("data.json")
err = json.SaveToFile("output.json", data, true) // 美化格式

// 使用文件的 Marshal/Unmarshal
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

### 包级便捷方法

库提供了使用默认处理器的便捷包级方法：

```go
// 性能监控（使用默认处理器）
stats := json.GetStats()
fmt.Printf("总操作数: %d\n", stats.OperationCount)
fmt.Printf("缓存命中率: %.2f%%\n", stats.HitRatio*100)
fmt.Printf("缓存内存使用: %d bytes\n", stats.CacheMemory)

// 健康监控
health := json.GetHealthStatus()
fmt.Printf("系统健康状态: %v\n", health.Healthy)

// 缓存管理
json.ClearCache()  // 清除所有缓存数据

// 缓存预热 - 预加载常用路径
paths := []string{"user.name", "user.age", "user.profile"}
warmupResult, err := json.WarmupCache(jsonStr, paths)

// 批量处理 - 高效执行多个操作
operations := []json.BatchOperation{
    {Type: "get", Path: "user.name"},
    {Type: "set", Path: "user.age", Value: 25},
    {Type: "delete", Path: "user.temp"},
}
results, err := json.ProcessBatch(operations)
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
first, err := json.GetInt(arrayData, "numbers[0]")           // 1
last, err := json.GetInt(arrayData, "numbers[-1]")           // 10（负索引）
slice, err := json.Get(arrayData, "numbers[1:4]")            // [2, 3, 4]
everyOther, err := json.Get(arrayData, "numbers[::2]")       // [1, 3, 5, 7, 9]
reverseEveryOther, err := json.Get(arrayData, "numbers[::-2]")  // [10, 8, 6, 4, 2]

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

// 2. 自定义配置
customConfig := &json.Config{
    // 缓存设置
    EnableCache:      true,             // 启用缓存
    MaxCacheSize:     128,              // 缓存条目数（默认值）
    CacheTTL:         5 * time.Minute,  // 缓存过期时间（默认值）

    // 大小限制
    MaxJSONSize:      100 * 1024 * 1024, // 100MB JSON 大小限制（默认值）
    MaxPathDepth:     50,                // 路径深度限制（默认值）
    MaxBatchSize:     2000,              // 批量操作大小限制

    // 并发设置
    MaxConcurrency:   50,   // 最大并发数（默认值）
    ParallelThreshold: 10,   // 并行处理阈值（默认值）

    // 处理选项
    EnableValidation: true,  // 启用验证
    StrictMode:       false, // 非严格模式
    CreatePaths:      true,  // 自动创建路径
    CleanupNulls:     true,  // 清理 null 值
}

processor2 := json.New(customConfig)
defer processor2.Close()

// 3. 预定义配置
// HighSecurityConfig: 用于处理不受信任的 JSON，具有严格的验证限制
secureProcessor := json.New(json.HighSecurityConfig())
defer secureProcessor.Close()

// LargeDataConfig: 用于处理大型 JSON 文件，优化性能
largeDataProcessor := json.New(json.LargeDataConfig())
defer largeDataProcessor.Close()
```

### 操作选项

```go
opts := &json.ProcessorOptions{
    CreatePaths:     true,  // 自动创建路径
    CleanupNulls:    true,  // 清理 null 值
    CompactArrays:   true,  // 压缩数组
    ContinueOnError: false, // 出错时继续
    MaxDepth:        50,    // 最大深度
}

result, err := json.Get(data, "path", opts)
```

### 性能监控

```go
processor := json.New(json.DefaultConfig())
defer processor.Close()

// 获取操作后的统计信息
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
data, err := json.LoadFromFile("example.json")

// 保存到文件（美化格式）
err = json.SaveToFile("output_pretty.json", data, true)

// 保存到文件（紧凑格式）
err = json.SaveToFile("output.json", data, false)

// 从 Reader 加载（使用处理器）
processor := json.New()
defer processor.Close()

file, err := os.Open("large_data.json")
if err != nil {
    log.Fatal(err)
}
defer file.Close()

data, err := processor.LoadFromReader(file)

// 保存到 Writer（使用处理器）
var buffer bytes.Buffer
err = processor.SaveToWriter(&buffer, data, true)
```

### Marshal/Unmarshal 文件操作

```go
// 将数据序列化到文件（默认紧凑格式）
user := map[string]any{
    "name": "Alice",
    "age":  30,
    "email": "alice@example.com",
}
err := json.MarshalToFile("user.json", user)

// 将数据序列化到文件（美化格式）
err = json.MarshalToFile("user_pretty.json", user, true)

// 从文件反序列化数据
var loadedUser map[string]any
err = json.UnmarshalFromFile("user.json", &loadedUser)

// 也支持结构体
type User struct {
    Name  string `json:"name"`
    Age   int    `json:"age"`
    Email string `json:"email"`
}

var person User
err = json.UnmarshalFromFile("user.json", &person)

// 使用处理器进行高级选项操作
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

// 保存合并后的配置
err = json.SaveToFile("merged_config.json", allConfigs, true)
if err != nil {
    log.Printf("保存合并配置失败: %v", err)
    return
}
```

---

### 安全配置

```go
// 安全配置
secureConfig := &json.Config{
    MaxJSONSize:              10 * 1024 * 1024, // 10MB JSON 大小限制
    MaxPathDepth:             50,                // 路径深度限制
    MaxNestingDepthSecurity:  100,               // 对象嵌套深度限制
    MaxArrayElements:         10000,             // 数组元素数量限制
    MaxObjectKeys:            1000,              // 对象键数量限制
    ValidateInput:            true,              // 输入验证
    EnableValidation:         true,              // 启用验证
    StrictMode:               true,              // 严格模式
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
status, err := json.GetString(apiResponse, "status")
if err != nil {
    fmt.Printf("Error: %v\n", err)
    return
}
// 结果: success

code, err := json.GetInt(apiResponse, "code")
if err != nil {
    fmt.Printf("Error: %v\n", err)
    return
}
// 结果: 200

// 获取分页信息
totalUsers, err := json.GetInt(apiResponse, "data.pagination.total")
if err != nil {
    fmt.Printf("Error: %v\n", err)
    return
}
// 结果: 25

currentPage, err := json.GetInt(apiResponse, "data.pagination.page")
if err != nil {
    fmt.Printf("Error: %v\n", err)
    return
}
// 结果: 1

// 批量提取用户信息
userNames, err := json.Get(apiResponse, "data.users.profile.name")
if err != nil {
    fmt.Printf("Error: %v\n", err)
    return
}
// 结果: ["Alice Johnson"]

userEmails, err := json.Get(apiResponse, "data.users.profile.email")
if err != nil {
    fmt.Printf("Error: %v\n", err)
    return
}
// 结果: ["alice@example.com"]

// 扁平化提取所有权限
allPermissions, err := json.Get(apiResponse, "data.users{flat:permissions}")
if err != nil {
    fmt.Printf("Error: %v\n", err)
    return
}
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

// 类型安全配置获取
dbHost := json.GetStringWithDefault(configJSON, "environments.production.database.host", "localhost")
dbPort := json.GetIntWithDefault(configJSON, "environments.production.database.port", 5432)
cacheEnabled := json.GetBoolWithDefault(configJSON, "environments.production.cache.enabled", false)

fmt.Printf("生产环境数据库: %s:%d\n", dbHost, dbPort)
fmt.Printf("缓存启用状态: %v\n", cacheEnabled)

// 动态配置更新
updates := map[string]any{
    "app.version": "1.2.4",
    "environments.production.cache.ttl": 10800, // 3 小时
}

newConfig, err := json.SetMultiple(configJSON, updates)
if err != nil {
    fmt.Printf("配置更新错误: %v\n", err)
    return
}
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
eventTypes, err := json.Get(analyticsData, "events.type")
if err != nil {
    fmt.Printf("Error: %v\n", err)
    return
}
// 结果: ["request", "error"]

// 提取所有状态码
statusCodes, err := json.Get(analyticsData, "events.status_code")
if err != nil {
    fmt.Printf("Error: %v\n", err)
    return
}
// 结果: [200, 500]

// 提取所有响应时间
responseTimes, err := json.GetTyped[[]int](analyticsData, "events.response_time")
if err != nil {
    fmt.Printf("Error: %v\n", err)
    return
}
// 结果: [45, 5000]

// 计算平均响应时间
times := responseTimes
var total float64
for _, t := range times {
    total += float64(t)
}

avgTime := total / float64(len(times))
fmt.Printf("平均响应时间: %.2f ms\n", avgTime)
```

---

## Set 操作 - 数据安全保证

所有 Set 操作都遵循 **默认安全** 模式，确保您的数据永远不会被损坏：

```go
// ✅ 成功: 返回修改后的数据
result, err := json.Set(data, "user.name", "Alice")
if err == nil {
    // result 包含成功修改的 JSON
    fmt.Println("数据已更新:", result)
}

// ❌ 失败: 返回原始未修改的数据
result, err := json.Set(data, "invalid[path", "value")
if err != nil {
    // result 仍然包含有效的原始数据
    // 您的原始数据永远不会损坏
    fmt.Printf("设置失败: %v\n", err)
    fmt.Println("原始数据已保留:", result)
}
```

**核心优势**:
- 🔒 **数据完整性**: 错误时永不损坏原始数据
- ✅ **安全回退**: 始终有有效的 JSON 可用
- 🎯 **可预测**: 所有操作行为一致

---

## 💡 示例与资源

### 📁 示例代码

- **[基础用法](examples/1_basic_usage.go)** - examples/1.basic_usage.go
- **[高级功能](examples/2_advanced_features.go)** - examples/2.advanced_features.go
- **[生产就绪](examples/3_production_ready.go)** - examples/3.production_ready.go

### 📖 更多资源

- **[兼容性指南](docs/COMPATIBILITY.md)** - `encoding/json` 的直接替换
- **[快速参考](docs/QUICK_REFERENCE.md)** - 常用操作速查表
- **[API 文档](https://pkg.go.dev/github.com/cybergodev/json)** - 完整 API 参考

---

## 🤝 贡献

欢迎贡献代码、报告问题和提出建议！

## 📄 许可证

MIT 许可证 - 详见 [LICENSE](LICENSE) 文件。

---

**用心为 Go 社区打造** ❤️ | 如果这个项目对您有帮助，请给它一个 ⭐️ Star！
