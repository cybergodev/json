# cybergodev/json

[![Go Version](https://img.shields.io/badge/Go-1.25+-00ADD8.svg)](https://golang.org)
[![GoDoc](https://pkg.go.dev/badge/github.com/cybergodev/json.svg)](https://pkg.go.dev/github.com/cybergodev/json)
[![MIT License](https://img.shields.io/badge/License-MIT-green.svg)](https://opensource.org/licenses/MIT)
[![Thread Safe](https://img.shields.io/badge/Thread_Safe-Yes-brightgreen.svg)](#)
[![Security](https://img.shields.io/badge/Security-Hardened-red.svg)](docs/SECURITY.md)

> 一个高性能、线程安全的 Go JSON 处理库，100% 兼容 `encoding/json`。
> 强大的路径语法、类型安全、流式处理、生产级性能。

**[English Documentation](README.md)** | **[www.cybergo.dev/json](https://www.cybergo.dev/json)**

---

## 为什么选择 cybergodev/json

| 功能 | encoding/json | cybergodev/json |
|------|---------------|-----------------|
| 路径访问 | 手动解析 | `json.Get(data, "users[0].name")` |
| 负索引 | - | `items[-1]` 获取最后一个元素 |
| 扁平化嵌套数组 | - | `users{flat:tags}` |
| JSON Pointer (RFC 6901) | - | `/users/0/name` |
| 类型安全默认值 | - | `GetString(data, "path", "default")` |
| 大文件流式处理 | - | 内置流式处理器 |
| 内存池 | - | 热路径使用 `sync.Pool` |
| 路径缓存 | - | 智能 TTL 路径缓存 |
| 批量操作 | - | `ProcessBatch()` 批量处理 |
| 100% 兼容性 | 原生 | 直接替换（签名扩展了可选配置参数） |

---

## 特性

- **100% 兼容** - 直接替换 `encoding/json`，所有标准函数签名支持，并可选扩展配置参数
- **强大路径** - 点号语法、数组切片、字段提取、JSON Pointer (RFC 6901)
- **高性能** - 智能缓存、内存池、优化的热路径
- **类型安全** - 泛型支持 `GetTyped[T]`、内置默认值、`AccessResult` 类型转换
- **功能丰富** - 批量操作、流式处理、文件I/O、Schema验证、深度合并、JSONL
- **生产就绪** - 线程安全、完善的错误处理、安全加固、健康监控

---

## 安装

```bash
go get github.com/cybergodev/json
```

**要求**: Go 1.25 或更高版本

**导入**:

```go
import "github.com/cybergodev/json"
```

---

## 快速开始

```go
package main

import (
    "fmt"
    "github.com/cybergodev/json"
)

func main() {
    data := `{"user": {"name": "Alice", "age": 28, "tags": ["premium", "verified"]}}`

    // 简单字段访问（直接返回值，无 error）
    name := json.GetString(data, "user.name")
    fmt.Println(name) // "Alice"

    // 类型安全获取（泛型）
    age := json.GetTyped[int](data, "user.age", 0)
    fmt.Println(age) // 28

    // 带默认值（路径不存在时不会 panic）
    email := json.GetTyped[string](data, "user.email", "unknown@example.com")
    fmt.Println(email) // "unknown@example.com"

    // 负索引（最后一个元素）
    lastTag, _ := json.Get(data, "user.tags[-1]")
    fmt.Println(lastTag) // "verified"

    // 修改数据
    updated, _ := json.Set(data, "user.age", 29)
    newAge := json.GetInt(updated, "user.age")
    fmt.Println(newAge) // 29

    // 100% encoding/json 兼容
    bytes, _ := json.Marshal(map[string]any{"status": "ok"})
    fmt.Println(string(bytes)) // {"status":"ok"}
}
```

---

## 路径语法参考

| 语法 | 描述 | 示例 |
|------|------|------|
| `.property` | 访问属性 | `user.name` -> "Alice" |
| `[n]` | 数组索引 | `items[0]` -> 第一个元素 |
| `[-n]` | 负索引（从末尾） | `items[-1]` -> 最后一个元素 |
| `[start:end]` | 数组切片 | `items[1:3]` -> 第1-2个元素 |
| `[start:end:step]` | 带步长的切片 | `items[::2]` -> 每隔一个元素 |
| `[+]` | 追加到数组 | `items[+]` -> 追加位置 |
| `{field}` | 提取所有元素的字段 | `users{name}` -> ["Alice", "Bob"] |
| `{flat:field}` | 扁平化嵌套数组 | `users{flat:tags}` -> 合并所有标签 |
| `/pointer` | JSON Pointer (RFC 6901) | `/users/0/name` -> "Alice" |

---

## 核心 API

### 数据获取

```go
// 基础获取器 — 直接返回值，支持可选默认值
// 路径不存在或类型不匹配时：返回零值，或使用提供的默认值
json.Get(data, "user.name")            // (any, error)
json.GetString(data, "user.name")      // string
json.GetInt(data, "user.age")          // int
json.GetFloat(data, "user.score")      // float64
json.GetBool(data, "user.active")      // bool
json.GetArray(data, "user.tags")       // []any
json.GetObject(data, "user.profile")   // map[string]any

// 类型安全的泛型获取
json.GetTyped[string](data, "user.name", "default")
json.GetTyped[[]int](data, "numbers", nil)
json.GetTyped[User](data, "user", User{})      // 自定义结构体

// 带默认值
json.GetString(data, "user.name", "Anonymous")
json.GetInt(data, "user.age", 0)
json.GetBool(data, "user.active", false)
json.GetFloat(data, "user.score", 0.0)
json.GetTyped[[]any](data, "user.tags", []any{})

// 安全访问，返回结果类型（带类型转换）
result := json.SafeGet(data, "user.age")
if result.Ok() {
    age, _ := result.AsInt()
    fmt.Println(age)
}

// 批量获取
results, err := json.GetMultiple(data, []string{"user.name", "user.age"})
```

### 数据修改

```go
// 基础设置 — 成功返回修改后的JSON，失败返回原始数据
result, err := json.Set(data, "user.name", "Bob")

// 使用配置自动创建路径
cfg := json.DefaultConfig()
cfg.CreatePaths = true
result, err := json.Set(data, "user.profile.level", "gold", cfg)

// 追加到数组
result, _ := json.Set(data, "user.tags[+]", "new-tag")

// 批量设置
result, _ := json.SetMultiple(data, map[string]any{
    "user.name": "Bob",
    "user.age":  30,
})

// 删除
result, err := json.Delete(data, "user.temp")

// 删除并清理 null
result, err = json.DeleteClean(data, "user.temp")

// 自动创建路径的 Set（自动创建中间路径）
result, err = json.SetCreate(data, "user.profile.level", "gold")

// 批量自动创建路径的 Set
result, err = json.SetMultipleCreate(data, map[string]any{
    "user.profile.level": "gold",
    "user.profile.badge": "star",
})
```

### 编码与格式化

```go
// 标准编码（100%兼容）
bytes, _ := json.Marshal(data)
json.Unmarshal(bytes, &target)
bytes, _ := json.MarshalIndent(data, "", "  ")

// 快速格式化
pretty, _ := json.Prettify(jsonStr)       // 美化输出

// 使用 encoding/json 兼容的 Buffer API 压缩
var buf bytes.Buffer
json.Compact(&buf, []byte(jsonStr))
compact := buf.String()

// 带配置编码
cfg := json.DefaultConfig()
cfg.Pretty = true
cfg.SortKeys = true
result, _ := json.Encode(data, cfg)

// 预设配置
result, _ := json.Encode(data, json.PrettyConfig())
```

### 文件操作

```go
// 加载和保存（包级别函数）
jsonStr, _ := json.LoadFromFile("data.json")
json.SaveToFile("output.json", data, json.PrettyConfig())

// 结构体/Map 序列化
json.MarshalToFile("user.json", user)
json.UnmarshalFromFile("user.json", &user)

// 写入任意 io.Writer
json.SaveToWriter(writer, data, cfg)

// 文件迭代（无需加载整个文件）
err := json.ForeachFile("data.json", func(key any, item *json.IterableValue) error {
    fmt.Println(item.GetString("name"))
    return nil
})

err = json.ForeachFileChunked("large.json", 100, func(chunk []*json.IterableValue) error {
    // 每次处理 100 个元素
    return nil
})

// 基于 Processor 的文件操作，支持完整配置
processor, _ := json.New(json.DefaultConfig())
defer processor.Close()
jsonStr, _ = processor.LoadFromFile("data.json")
_ = processor.SaveToFile("output.json", data, json.PrettyConfig())
```

### JSON 工具

```go
// 比较两个 JSON 字符串
equal, _  := json.CompareJSON(json1, json2)

// 并集合并（默认）— 合并所有键
merged, _ := json.MergeJSON(json1, json2)

// 交集合并 — 仅保留共有键
cfg := json.DefaultConfig()
cfg.MergeMode = json.MergeIntersection
merged, _ = json.MergeJSON(json1, json2, cfg)

// 差集合并 — 仅保留 json1 中有而 json2 中没有的键
cfg.MergeMode = json.MergeDifference
merged, _ = json.MergeJSON(json1, json2, cfg)

// 合并多个 JSON 对象
merged, _ = json.MergeMany([]string{json1, json2, json3})
```

---

## 配置

### 自定义配置

```go
cfg := json.DefaultConfig()     // 从安全默认值开始
cfg.EnableCache = true
cfg.MaxCacheSize = 256
cfg.CacheTTL = 5 * time.Minute
cfg.MaxConcurrency = 50
cfg.CreatePaths = true          // Set 操作自动创建路径
cfg.CleanupNulls = true         // Delete 后清理 null

processor, err := json.New(cfg)
if err != nil {
    // 处理配置错误
}
defer processor.Close()

// 使用处理器方法
result, _ := processor.Get(jsonStr, "user.name")
stats := processor.GetStats()
health := processor.GetHealthStatus()
processor.ClearCache()
```

### 预设配置

```go
cfg := json.DefaultConfig()   // 平衡的默认配置
cfg := json.SecurityConfig()  // 用于不受信任的输入
cfg := json.PrettyConfig()    // 用于美化输出
```

---

## 高级功能

### 数据迭代

```go
// 基础迭代
json.Foreach(data, func(key any, item *json.IterableValue) {
    name := item.GetString("name")
    fmt.Printf("键: %v, 名称: %s\n", key, name)
})

// 带路径迭代
json.ForeachWithPath(data, "users", func(key any, item *json.IterableValue) {
    name := item.GetString("name")
    fmt.Printf("键: %v, 名称: %s\n", key, name)
})

// 嵌套迭代（指定嵌套字段路径）
json.ForeachNested(data, func(key any, item *json.IterableValue) {
    item.ForeachNested("items", func(nestedKey any, nestedItem *json.IterableValue) {
        fmt.Printf("嵌套: %v\n", nestedItem.Get("id"))
    })
})

// 带路径迭代（使用 ForeachFile 可支持返回 error 的回调）
err := json.ForeachWithPath(data, "users", func(key any, item *json.IterableValue) {
    if item.IsNull("id") {
        log.Printf("警告: 键 %v 缺少 id", key)
    }
})

// 迭代器控制（break / continue）
_ = json.ForeachWithPathAndControl(data, "users", func(key any, value any) json.IteratorControl {
    if key.(int) > 5 {
        return json.IteratorBreak
    }
    return json.IteratorNormal
})

// 迭代并返回原始 JSON 字符串
result, err := json.ForeachReturn(data, func(key any, item *json.IterableValue) {
    // 遍历所有元素；返回的 result 为原始 JSON 字符串（不变）
})
```

### 批量操作

```go
data := `{"user": {"name": "Alice", "age": 28, "temp": "value"}}`

operations := []json.BatchOperation{
    {Type: "get", JSONStr: data, Path: "user.name", ID: "op1"},
    {Type: "set", JSONStr: data, Path: "user.age", Value: 25},
    {Type: "delete", JSONStr: data, Path: "user.temp"},
}
results, err := json.ProcessBatch(operations)
```

### Schema 验证

```go
schema := &json.Schema{
    Type:     "object",
    Required: []string{"name", "email"},
    Properties: map[string]*json.Schema{
        "name":  {Type: "string", MinLength: 1, MaxLength: 100},
        "email": {Type: "string", Format: "email"},
        "age":   {Type: "integer", Minimum: 0, Maximum: 150},
    },
}

validationErrors, err := json.ValidateSchema(jsonStr, schema)
```

`Schema` 类型支持 JSON Schema Draft 7 子集字段：`Type`、`Properties`、`Required`、`Items`、`Pattern`、`AdditionalProperties`、`Enum`、`Const`、`Format`、`MinLength`、`MaxLength`、`Minimum`、`Maximum`、`ExclusiveMinimum`、`ExclusiveMaximum`、`MultipleOf`、`MinItems`、`MaxItems`、`UniqueItems`、`Title`、`Description`、`Default`。

### PreParse 与 CompiledPath

```go
processor, _ := json.New(json.DefaultConfig())
defer processor.Close()

// 一次解析，多次查询（避免重复解析）
parsed, _ := processor.PreParse(jsonStr)
name, _    := processor.GetFromParsed(parsed, "user.name")
age, _     := processor.GetFromParsed(parsed, "user.age")
updated, _ := processor.SetFromParsed(parsed, "user.age", 30)

// 编译路径，用于快速重复访问
compiled, _ := processor.CompilePath("user.profile.settings.theme")
value, _    := processor.GetCompiled(jsonStr, compiled)
```

### 编码工具

```go
// EncodeStream - 将切片编码为 JSON 数组
streamJSON, _ := json.EncodeStream(users, json.PrettyConfig())

// EncodeBatch - 将键值对编码为 JSON 对象
batchJSON, _ := json.EncodeBatch(pairs, cfg)

// EncodeFields - 只编码指定字段（过滤敏感数据）
fieldsJSON, _ := json.EncodeFields(user, []string{"id", "name", "email"}, cfg)
```

### JSONL 处理

```go
// JSON 数组与 JSONL 格式互转
jsonlData, _    := json.ToJSONL(records)          // []any -> JSONL 字节
jsonlString, _  := json.ToJSONLString(records)    // []any -> JSONL 字符串
records, _      := json.ParseJSONL(jsonlData)     // JSONL 字节 -> []any

// 从 Reader 流式处理 JSONL
processor, _ := json.New(json.DefaultConfig())
defer processor.Close()
err := processor.StreamJSONL(reader, func(lineNum int, item *json.IterableValue) error {
    fmt.Printf("第 %d 行: %s\n", lineNum, item.GetString("id"))
    return nil
})

// JSONL 写入器
writer := json.NewJSONLWriter(bufWriter)
writer.Write(record1)
writer.Write(record2)
writer.WriteAll(records)
stats := writer.Stats() // LinesProcessed, BytesWritten

// NDJSON 文件处理器
ndjson := json.NewNDJSONProcessor(json.DefaultConfig())
err = ndjson.ProcessFile("data.ndjson", func(lineNum int, obj map[string]any) error {
    fmt.Printf("第 %d 行: %v\n", lineNum, obj["id"])
    return nil
})

// JSONL 过滤、映射、归约
filtered, _ := processor.FilterJSONL(reader, func(item *json.IterableValue) bool {
    return item.GetBool("active")
})
mapped, _   := processor.MapJSONL(reader, func(lineNum int, item *json.IterableValue) (any, error) {
    return map[string]any{"id": item.GetString("id")}, nil
})
result, _   := processor.ReduceJSONL(reader, 0, func(acc any, item *json.IterableValue) any {
    count := item.GetInt("count")
    return acc.(int) + count
})
```

### 流式迭代器

```go
// 流式处理大型 JSON 数组（无需全部加载到内存）
reader := strings.NewReader(largeJSONArray)
iter := json.NewStreamIterator(reader)
for iter.Next() {
    item := iter.Value()
    // 处理 item
}

// 流式处理 JSON 对象（键值对）
objIter := json.NewStreamObjectIterator(reader)
for objIter.Next() {
    key, value := objIter.Key(), objIter.Value()
}

// 批量处理（使用内存中的数据）
batchIter := json.NewBatchIterator(items, json.DefaultConfig())
for batchIter.HasNext() {
    batch := batchIter.NextBatch()
}
```

### 并行处理

```go
// ParallelIterator - 使用 worker 池并行处理
items := []any{"a", "b", "c", "d"}
iter := json.NewParallelIterator(items)

// 并行处理每个元素
err := iter.ForEach(func(idx int, item any) error {
    // 并发处理每个元素
    return nil
})

// 批量并行处理
err = iter.ForEachBatch(2, func(batchIdx int, batch []any) error {
    // 处理一批元素
    return nil
})

// 并行 JSONL 流处理
processor, _ := json.New(json.DefaultConfig())
defer processor.Close()
err = processor.StreamJSONLParallel(reader, 4, func(lineNum int, item *json.IterableValue) error {
    // 使用 4 个并行 worker 处理
    return nil
})
```

### 钩子 (Hooks)

```go
processor, _ := json.New(json.DefaultConfig())
defer processor.Close()

// 日志钩子 — 接受带有 Info(string, ...any) 方法的任意类型
processor.AddHook(json.LoggingHook(slog.Default()))

// 计时钩子 — 接受带有 Record(op string, duration time.Duration) 方法的接口
type MetricsRecorder struct{}
func (m *MetricsRecorder) Record(op string, d time.Duration) { /* 记录 */ }
processor.AddHook(json.TimingHook(&MetricsRecorder{}))

// 验证钩子 — 接受 func(jsonStr, path string) error
processor.AddHook(json.ValidationHook(func(jsonStr, path string) error {
    if len(path) > 100 {
        return fmt.Errorf("路径过长: %s", path)
    }
    return nil
}))

// 错误钩子 — 接受 func(ctx json.HookContext, err error) error
processor.AddHook(json.ErrorHook(func(ctx json.HookContext, err error) error {
    log.Printf("操作 %s 在路径 %s 失败: %v", ctx.Operation, ctx.Path, err)
    return err // 返回原始或转换后的错误
}))

// 使用 HookFunc 自定义钩子
processor.AddHook(json.HookFunc{
    BeforeFn: func(ctx json.HookContext) error {
        fmt.Printf("before: %s %s\n", ctx.Operation, ctx.Path)
        return nil
    },
    AfterFn: func(ctx json.HookContext, result any, err error) (any, error) {
        fmt.Printf("after: %s (err=%v)\n", ctx.Operation, err)
        return result, err
    },
})
```

---

## 常见用例

### API 响应处理

```go
apiResponse := `{
    "status": "success",
    "data": {
        "users": [{"id": 1, "name": "Alice", "permissions": ["read", "write"]}],
        "pagination": {"total": 25, "page": 1}
    }
}`

// 快速提取
status := json.GetString(apiResponse, "status")
total  := json.GetInt(apiResponse, "data.pagination.total")

// 提取所有用户名
names, _ := json.Get(apiResponse, "data.users{name}")
// 结果: ["Alice"]

// 扁平化所有权限
permissions, _ := json.Get(apiResponse, "data.users{flat:permissions}")
// 结果: ["read", "write"]

// JSON Pointer 访问
name, _ := json.Get(apiResponse, "/data/users/0/name")
// 结果: "Alice"
```

### 配置管理

```go
config := `{
    "database": {"host": "localhost", "port": 5432},
    "cache": {"enabled": true}
}`

// 类型安全带默认值
dbHost       := json.GetString(config, "database.host", "localhost")
dbPort       := json.GetInt(config, "database.port", 5432)
cacheEnabled := json.GetBool(config, "cache.enabled", false)

// 动态更新
updated, _ := json.SetMultiple(config, map[string]any{
    "database.host": "prod-db.example.com",
    "cache.ttl":     3600,
})
```

### 多源数据合并

```go
// 合并来自多个来源的配置
defaults := `{"timeout": 30, "retries": 3, "debug": false}`
file     := `{"timeout": 60, "debug": true}`
env      := `{"retries": 5}`

// 并集合并（默认行为）
merged, _ := json.MergeMany([]string{defaults, file, env})
// 结果: {"timeout": 60, "retries": 5, "debug": true}
```

---

## 性能监控

```go
// 包级别监控
stats := json.GetStats()
fmt.Printf("操作数: %d\n", stats.OperationCount)
fmt.Printf("缓存命中率: %.2f%%\n", stats.HitRatio*100)

health := json.GetHealthStatus()
fmt.Printf("健康状态: %v\n", health.Healthy)

// 缓存管理
json.ClearCache()

// 缓存预热 — 预加载路径以加速后续访问
paths := []string{"user.name", "user.age", "user.profile"}
result, _ := json.WarmupCache(jsonStr, paths)

// Processor 级别监控
processor, _ := json.New(json.DefaultConfig())
defer processor.Close()
stats := processor.GetStats()
health := processor.GetHealthStatus()
processor.ClearCache()
```

---

## 从 encoding/json 迁移

只需修改导入：

```go
// 之前
import "encoding/json"

// 之后
import "github.com/cybergodev/json"
```

所有标准函数完全兼容：
- `json.Marshal()` / `json.Unmarshal()`
- `json.MarshalIndent()`
- `json.NewEncoder()` / `json.NewDecoder()`
- `json.Valid()`
- `json.Compact()` / `json.Indent()` / `json.HTMLEscape()`

兼容类型：`Encoder`、`Decoder`、`Number`、`Token`、`Delim`、`SyntaxError`、`UnmarshalTypeError`、`InvalidUnmarshalError`、`UnsupportedTypeError`、`UnsupportedValueError`、`MarshalerError`。

**注意**：`RawMessage` 目前未重新导出。如需使用，请直接使用 `encoding/json.RawMessage`。

详见 [兼容性指南](docs/COMPATIBILITY.md)。

---

## 安全配置

```go
// 用于处理不受信任的 JSON 输入
secureConfig := json.SecurityConfig()
// 特性:
// - 启用完整安全扫描
// - 保守的大小限制（最大10MB）
// - 严格模式验证
// - 原型污染防护
// - 路径遍历防护

processor, _ := json.New(secureConfig)
defer processor.Close()
```

### 安全工具

```go
// 注册自定义危险模式
json.RegisterDangerousPattern(json.DangerousPattern{
    Pattern: "eval\\(",
    Name:    "eval-injection",
    Level:   json.PatternLevelCritical,
})

// 列出所有已注册的模式
patterns := json.ListDangerousPatterns()

// 安全错误报告（不泄露内部细节）
safeMsg := json.SafeError(err)

// 日志中脱敏敏感路径
redacted := json.RedactedPath("user.password")
```

### 错误处理

```go
// 哨兵错误，用于程序化判断
errors.Is(err, json.ErrInvalidJSON)
errors.Is(err, json.ErrPathNotFound)
errors.Is(err, json.ErrTypeMismatch)
errors.Is(err, json.ErrSizeLimit)
errors.Is(err, json.ErrSecurityViolation)

// 带上下文的结构化错误
var jsonErr *json.JsonsError
if errors.As(err, &jsonErr) {
    fmt.Printf("操作=%s 路径=%s: %s\n", jsonErr.Op, jsonErr.Path, jsonErr.Message)
}
```

详见 [安全指南](docs/SECURITY.md)。

---

## 示例代码

| 文件 | 描述 |
|------|------|
| [1_basic_usage.go](examples/1_basic_usage.go) | 核心操作 |
| [2_advanced_features.go](examples/2_advanced_features.go) | 复杂路径、嵌套提取 |
| [3_production_ready.go](examples/3_production_ready.go) | 线程安全模式 |
| [4_error_handling.go](examples/4_error_handling.go) | 错误处理模式 |
| [5_encoding_options.go](examples/5_encoding_options.go) | 编码配置 |
| [6_validation.go](examples/6_validation.go) | Schema 验证 |
| [7_type_conversion.go](examples/7_type_conversion.go) | 类型转换 |
| [8_helper_functions.go](examples/8_helper_functions.go) | 辅助工具 |
| [9_iterator_functions.go](examples/9_iterator_functions.go) | 迭代模式 |
| [10_file_operations.go](examples/10_file_operations.go) | 文件 I/O |
| [11_with_defaults.go](examples/11_with_defaults.go) | 默认值处理 |
| [12_advanced_delete.go](examples/12_advanced_delete.go) | 删除操作 |
| [13_batch_operations.go](examples/13_batch_operations.go) | 批量处理与缓存 |
| [14_streaming_iterators.go](examples/14_streaming_iterators.go) | 流式迭代器 |
| [15_jsonl_processing.go](examples/15_jsonl_processing.go) | JSONL 格式处理 |
| [16_hooks_and_security.go](examples/16_hooks_and_security.go) | 钩子与安全模式 |
| [17_advanced_patterns.go](examples/17_advanced_patterns.go) | PreParse、CompiledPath、高级模式 |

```bash
# 运行单个示例（需要 build tag）
go run -tags=example examples/1_basic_usage.go
go run -tags=example examples/2_advanced_features.go
```

---

## 文档

- **[API 参考](docs/API_REFERENCE.md)** - 完整 API 文档
- **[安全指南](docs/SECURITY.md)** - 安全最佳实践
- **[快速参考](docs/QUICK_REFERENCE.md)** - 常用模式速查
- **[兼容性](docs/COMPATIBILITY.md)** - encoding/json 兼容性详情
- **[pkg.go.dev](https://pkg.go.dev/github.com/cybergodev/json)** - GoDoc

---

## 许可证

MIT License - 详见 [LICENSE](LICENSE) 文件。
