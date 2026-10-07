# cybergodev/json

[![Go Version](https://img.shields.io/badge/Go-1.25+-00ADD8.svg)](https://golang.org)
[![GoDoc](https://pkg.go.dev/badge/github.com/cybergodev/json.svg)](https://pkg.go.dev/github.com/cybergodev/json)
[![MIT License](https://img.shields.io/badge/License-MIT-green.svg)](https://opensource.org/licenses/MIT)
[![Thread Safe](https://img.shields.io/badge/Thread_Safe-Yes-brightgreen.svg)](#)
[![Security](https://img.shields.io/badge/Security-Hardened-red.svg)](docs/SECURITY.md)

> 一个高性能、线程安全的 Go JSON 处理库——`encoding/json` 签名直接替换，默认开启安全加固（见[兼容性说明](docs/COMPATIBILITY.md#security-hardening-differences-d-002)）。
> 强大的路径语法、类型安全、流式处理、生产级性能。

**[English Documentation](README.md)** | **[www.cybergo.dev/json](https://www.cybergo.dev/json)**

---

## 目录

- [为什么选择 cybergodev/json](#为什么选择-cybergodevjson)
- [特性](#特性)
- [安装](#安装)
- [快速开始](#快速开始)
- [路径语法参考](#路径语法参考)
- [核心 API](#核心-api)
  - [数据获取](#数据获取) · [解析](#解析) · [数据修改](#数据修改)
  - [编码与格式化](#编码与格式化) · [文件操作](#文件操作) · [JSON 工具](#json-工具)
- [配置](#配置)
- [高级功能](#高级功能)
- [常见用例](#常见用例)
- [性能监控](#性能监控)
- [从 encoding/json 迁移](#从-encodingjson-迁移)
- [安全配置](#安全配置)
- [示例代码](#示例代码)
- [文档](#文档)
- [许可证](#许可证)

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
| 直接替换兼容 | 原生 | 相同签名（可选配置扩展）；[默认安全加固](docs/COMPATIBILITY.md) |

---

## 特性

- **直接替换兼容** - 所有 `encoding/json` 函数签名支持，可选配置扩展；[默认安全校验](docs/COMPATIBILITY.md#security-hardening-differences-d-002) 会拒绝 `encoding/json` 接受的部分文档（超大、超深、非法 UTF-8、含危险内容模式）
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

    // encoding/json 兼容签名
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
| `{字段1,字段2}` | 多字段提取（子集对象） | `address{city,zip}` -> `{"city":...,"zip":...}` |
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

// 带上下文的获取（支持超时/取消）
value, err := json.GetWithContext(ctx, data, "user.name")
```

### 解析

```go
// 解析 JSON 为 any（使用默认处理器）
data, err := json.ParseAny(jsonStr)

// 解析 JSON 到类型化目标
var user User
err = json.Parse(jsonStr, &user)

// 使用配置
data, err = json.ParseAny(jsonStr, json.SecurityConfig())
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
// 标准编码（典型值输出字节兼容）
bytes, _ := json.Marshal(data)
json.Unmarshal(bytes, &target)
bytes, _ := json.MarshalIndent(data, "", "  ")

// 快速格式化
pretty, _ := json.Prettify(jsonStr)       // 美化输出

// 使用 encoding/json 兼容的 Buffer API 压缩
var buf bytes.Buffer
json.Compact(&buf, []byte(jsonStr))
compact := buf.String()

// 或使用 string-in/string-out 形式（直接压缩 JSON 字符串）
compact, _ = json.CompactString(jsonStr)

// 带配置编码 —— Encode 是规范的字符串编码函数
// （EncodeWithConfig 是保留兼容的已废弃别名）
cfg := json.DefaultConfig()
cfg.Pretty = true
cfg.SortKeys = true
result, _ := json.Encode(data, cfg)

// 预设配置
result, _ = json.Encode(data, json.PrettyConfig())

// 快速美化编码
result, _ = json.EncodePretty(data)

// 带配置验证
valid, err := json.ValidWithConfig(jsonStr, json.SecurityConfig())
```

### 文件操作

```go
// 加载和保存（包级别函数）
jsonStr, _ := json.LoadFromFile("data.json")
json.SaveToFile("output.json", data, json.PrettyConfig())

// 从 Reader 加载（带大小限制）
jsonStr, _ = json.LoadFromReader(reader)

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
// 根据使用场景选择预设：
cfg := json.DefaultConfig()   // 平衡的默认配置
// cfg := json.SecurityConfig()  // 用于不受信任的输入
// cfg := json.PrettyConfig()    // 用于美化输出
```

---

## 高级功能

### 多字段提取

使用 `{}` 内的逗号分隔字段名，从对象（或数组中的每个对象）中提取字段子集。
未列出的字段会被丢弃，不存在的字段会被静默跳过；字段名两侧的空格会被容忍
（`{a, b, c}` 会被规范化为 `{a,b,c}`）。

```go
data := `{
    "address": {
        "city": "Metro City",
        "zip": "12345",
        "country": "Sample Country",
        "street": "1 Main Street"
    }
}`

// 仅提取 "city" 和 "zip" 生成新对象（country 和 street 被丢弃）
subset, _ := json.Get(data, "address{city,zip}")
// subset -> map[string]any{"city": "Metro City", "zip": "12345"}

// GetObject / GetTyped 是结果 map 对应的类型化获取器：
addr := json.GetObject(data, "address{city,zip}")
fmt.Println(addr["city"], addr["zip"]) // Metro City 12345
```

当目标为数组时，会对每个元素应用提取：

```go
arr := `{"users":[
    {"id":1,"name":"Alice","email":"a@x.com"},
    {"id":2,"name":"Bob","email":"b@x.com"}
]}`
names, _ := json.Get(arr, "users{id,name}")
// names -> [{"id":1,"name":"Alice"},{"id":2,"name":"Bob"}]
```

> **注意** —— 多字段提取返回的是*对象*（`map[string]any`），因此请优先使用
> `Get` / `GetObject`。对其调用 `GetString` 会回退到 Go 的默认格式化：
> `json.GetString(data, "address{city,zip}", "")` 返回 `"map[city:Metro City zip:12345]"`，
> 而非 JSON 字符串。单字段 `{city}` 则不同 —— 它返回原始值（`"Metro City"`），
> 而非包装后的对象。

### 数据迭代

```go
// 基础迭代（返回 error 的形式；void 的 Foreach 已废弃，因其丢弃错误）
err := json.ForeachWithError(data, ".", func(key any, item *json.IterableValue) error {
    name := item.GetString("name")
    fmt.Printf("键: %v, 名称: %s\n", key, name)
    return nil
})

// 带路径迭代
err = json.ForeachWithPath(data, "users", func(key any, item *json.IterableValue) {
    name := item.GetString("name")
    fmt.Printf("键: %v, 名称: %s\n", key, name)
})

// 嵌套迭代（指定嵌套字段路径）
err = json.ForeachNestedWithError(data, func(key any, item *json.IterableValue) error {
    item.ForeachNested("items", func(nestedKey any, nestedItem *json.IterableValue) {
        fmt.Printf("嵌套: %v\n", nestedItem.Get("id"))
    })
    return nil
})

// 返回 error 的回调：返回非 nil 的 error 可提前终止迭代。
// （普通回调请用 ForeachWithPath；基于文件输入请用 ForeachFile。）
err = json.ForeachWithError(data, "users", func(key any, item *json.IterableValue) error {
    if item.IsNull("id") {
        log.Printf("警告: 键 %v 缺少 id", key)
    }
    return nil
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

// NDJSON 文件处理（StreamJSONLFile —— NDJSONProcessor 已废弃）
proc, _ := json.New(json.DefaultConfig())
defer proc.Close()
err = proc.StreamJSONLFile("data.ndjson", func(lineNum int, item *json.IterableValue) error {
    fmt.Printf("第 %d 行: %v\n", lineNum, item.Get("id")) // item.GetData() 取整个 map
    return nil
})

// JSONL 过滤、映射、归约（也作为包级别函数提供）
filtered, _ := json.FilterJSONL(reader, func(item *json.IterableValue) bool {
    return item.GetBool("active")
})
mapped, _   := json.MapJSONL(reader, func(lineNum int, item *json.IterableValue) (any, error) {
    return map[string]any{"id": item.GetString("id")}, nil
})
result, _   := json.ReduceJSONL(reader, 0, func(acc any, item *json.IterableValue) any {
    count := item.GetInt("count")
    return acc.(int) + count
})

// 更多 JSONL 工具函数
items, _    := json.CollectJSONL(reader)       // 收集所有元素
first, _, _ := json.FirstJSONL(reader, func(item *json.IterableValue) bool {
    return item.GetBool("target")              // 查找第一个匹配的元素
})
err = json.ForeachJSONL(reader, func(lineNum int, item *json.IterableValue) error {
    fmt.Printf("第 %d 行: %s\n", lineNum, item.GetString("id"))
    return nil
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

// 计时钩子 — 接受带有 Record(op string, duration time.Duration) 方法的类型
// 定义你的记录器：
//
//	type MetricsRecorder struct{}
//	func (m *MetricsRecorder) Record(op string, d time.Duration) { /* 记录 */ }
//
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

### 基准测试

本库内置了一套基准测试套件，将热路径（快速编码器、基于路径的 `Get`、
数组切片、字段提取）与 `encoding/json` 进行对比。运行它可在你的硬件上
实测性能：

```bash
# 完整基准测试套件（含内存分配统计）
go test -run='^$' -bench=. -benchmem ./...
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

**默认安全加固——一处真实行为差异**：与 `encoding/json` 不同，默认配置会对输入做校验。`Unmarshal` 会拒绝超过 `MaxJSONSize` / 嵌套深度限制、包含非法 UTF-8 或携带内置危险内容模式（`<script`、`javascript:`、`onerror`、`eval(` 等）的文档；`Valid` 对它们返回 `false`。若你的数据合法地包含此类内容（HTML 片段、代码示例），请设置 `SkipValidation: true`（与标准库行为完全一致）或 `DisableDefaultPatterns: true`（保留大小/深度限制，仅关闭内容模式）。

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
| [1_basic_usage](examples/1_basic_usage/main.go) | 核心操作 |
| [2_advanced_features](examples/2_advanced_features/main.go) | 复杂路径、嵌套提取 |
| [3_production_ready](examples/3_production_ready/main.go) | 线程安全模式 |
| [4_error_handling](examples/4_error_handling/main.go) | 错误处理模式 |
| [5_encoding_options](examples/5_encoding_options/main.go) | 编码配置 |
| [6_validation](examples/6_validation/main.go) | Schema 验证 |
| [7_type_conversion](examples/7_type_conversion/main.go) | 类型转换 |
| [8_helper_functions](examples/8_helper_functions/main.go) | 辅助工具 |
| [9_iterator_functions](examples/9_iterator_functions/main.go) | 迭代模式 |
| [10_file_operations](examples/10_file_operations/main.go) | 文件 I/O |
| [11_with_defaults](examples/11_with_defaults/main.go) | 默认值处理 |
| [12_advanced_delete](examples/12_advanced_delete/main.go) | 删除操作 |
| [13_batch_operations](examples/13_batch_operations/main.go) | 批量处理与缓存 |
| [14_streaming_iterators](examples/14_streaming_iterators/main.go) | 流式迭代器 |
| [15_jsonl_processing](examples/15_jsonl_processing/main.go) | JSONL 格式处理 |
| [16_hooks_and_security](examples/16_hooks_and_security/main.go) | 钩子与安全模式 |
| [17_advanced_patterns](examples/17_advanced_patterns/main.go) | PreParse、CompiledPath、高级模式 |

```bash
# 运行单个示例（需要 build tag）
go run -tags=example ./examples/1_basic_usage
go run -tags=example ./examples/2_advanced_features

# 一次性编译检查全部示例（与 CI 门禁一致）
go vet -tags example ./examples/...
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
