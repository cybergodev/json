# 快速参考手册 (Quick Reference)

> cybergodev/json 库的快速参考指南 - 常用功能速查

---

## 📦 安装

```bash
go get github.com/cybergodev/json
```

---

## 🚀 基本使用

### 导入库

```go
import "github.com/cybergodev/json"
```

---

## 📖 数据获取 (Get)

### 基本类型获取

```go
// 获取任意类型
value, err := json.Get(data, "path")

// 获取字符串
str, err := json.GetString(data, "user.name")

// 获取整数
num, err := json.GetInt(data, "user.age")

// 获取布尔值
flag, err := json.GetBool(data, "user.active")

// 获取浮点数
price, err := json.GetFloat64(data, "product.price")

// 获取数组
arr, err := json.GetArray(data, "items")

// 获取对象
obj, err := json.GetObject(data, "user.profile")
```

### 带默认值的获取

```go
// 字符串（默认值："匿名"）
name := json.GetStringWithDefault(data, "user.name", "匿名")

// 整数（默认值：0）
age := json.GetIntWithDefault(data, "user.age", 0)

// 布尔值（默认值：false）
active := json.GetBoolWithDefault(data, "user.active", false)
```

### 类型安全获取（泛型）

```go
// 获取字符串
name, err := json.GetTyped[string](data, "user.name")

// 获取整数切片
numbers, err := json.GetTyped[[]int](data, "scores")

// 获取自定义类型
users, err := json.GetTyped[[]User](data, "users")
```

### 批量获取

```go
paths := []string{"user.name", "user.age", "user.email"}
results, err := json.GetMultiple(data, paths)

// 访问结果
name := results["user.name"]
age := results["user.age"]
```

---

## ✏️ 数据修改 (Set)

### 基本设置

```go
// 设置单个值
result, err := json.Set(data, "user.name", "Alice")

// 自动创建路径
result, err := json.SetWithAdd(data, "user.profile.city", "北京")
```

### 批量设置

```go
updates := map[string]any{
    "user.name": "Bob",
    "user.age":  30,
    "user.active": true,
}
result, err := json.SetMultiple(data, updates)

// 批量设置（自动创建路径）
result, err := json.SetMultipleWithAdd(data, updates)
```

---

## 🗑️ 数据删除 (Delete)

```go
// 删除字段
result, err := json.Delete(data, "user.temp")

// 删除并清理 null 值
result, err := json.DeleteWithCleanNull(data, "user.temp")
```

---

## 🔄 数据迭代 (Foreach)

### 基本迭代（只读）

```go
json.Foreach(data, func(key any, item *json.IterableValue) {
    name := item.GetString("name")
    age := item.GetInt("age")
    fmt.Printf("键：%v，名称：%s，年龄：%d\n", key, name, age)
})
```

### 路径迭代（只读）

```go
json.ForeachWithPath(data, "users", func(key any, user *json.IterableValue) {
    name := user.GetString("name")
    fmt.Printf("用户 %v：%s\n", key, name)
})
```

### 迭代并修改

```go
modifiedJson, err := json.ForeachReturn(data, func(key any, item *json.IterableValue) {
    // 修改数据
    if item.GetString("status") == "inactive" {
        item.Set("status", "active")
    }
})
```

---

## 🎯 路径表达式

### 基本语法

| 语法             | 说明    | 示例                   | 结果             |
|----------------|-------|----------------------|----------------|
| `.`            | 属性访问  | `user.name`          | 获取 user 的 name |
| `[n]`          | 数组索引  | `users[0]`           | 第一个用户          |
| `[-n]`         | 负数索引  | `users[-1]`          | 最后一个用户         |
| `[start:end]`  | 数组切片  | `users[1:3]`         | 索引 1-2 的用户     |
| `[::step]`     | 步长切片  | `numbers[::2]`       | 每隔一个元素         |
| `{field}`      | 批量提取  | `users{name}`        | 所有用户名          |
| `{flat:field}` | 扁平化提取 | `users{flat:skills}` | 所有技能（扁平）       |

### 路径示例

```go
data := `{
  "users": [
    {"name": "Alice", "skills": ["Go", "Python"]},
    {"name": "Bob", "skills": ["Java", "React"]}
  ]
}`

// 获取第一个用户
json.Get(data, "users[0]")
// 结果：{"name": "Alice", "skills": ["Go", "Python"]}

// 获取最后一个用户
json.Get(data, "users[-1]")
// 结果：{"name": "Bob", "skills": ["Java", "React"]}

// 获取所有用户名
json.Get(data, "users{name}")
// 结果：["Alice", "Bob"]

// 获取所有技能（扁平化）
json.Get(data, "users{flat:skills}")
// 结果：["Go", "Python", "Java", "React"]
```

---

## 📁 文件操作

### 读取文件

```go
// 从文件加载
data, err := json.LoadFromFile("config.json")

// 从 Reader 加载
file, _ := os.Open("data.json")
defer file.Close()
data, err := json.LoadFromReader(file)
```

### 写入文件

```go
// 保存到文件（美化格式）
err := json.SaveToFile("output.json", data, true)

// 保存到文件（紧凑格式）
err := json.SaveToFile("output.json", data, false)

// 保存到 Writer
var buffer bytes.Buffer
err := json.SaveToWriter(&buffer, data, true)
```

---

## ⚙️ 配置

### 创建处理器

```go
// 使用默认配置
processor := json.New()
defer processor.Close()

// 使用自定义配置
config := &json.Config{
    EnableCache:      true,
    MaxCacheSize:     5000,
    MaxJSONSize:      50 * 1024 * 1024, // 50MB
    EnableValidation: true,
}
processor := json.New(config)
defer processor.Close()

// 使用预定义配置
processor := json.New(json.HighSecurityConfig())
processor := json.New(json.LargeDataConfig())
```

### 性能监控

```go
// 获取统计信息
stats := processor.GetStats()
fmt.Printf("操作数：%d\n", stats.OperationCount)
fmt.Printf("缓存命中率：%.2f%%\n", stats.HitRatio*100)

// 获取健康状态
health := processor.GetHealthStatus()
fmt.Printf("健康状态：%v\n", health.Healthy)
```

---

## 🛡️ 数据验证

### JSON Schema 验证

```go
schema := &json.Schema{
    Type: "object",
    Properties: map[string]*json.Schema{
        "name": {Type: "string"},
        "age":  {Type: "number"},
    },
    Required: []string{"name", "age"},
}

processor := json.New(json.DefaultConfig())
errors, err := processor.ValidateSchema(data, schema)
```

### 基本验证

```go
// 验证 JSON 是否有效
if json.Valid([]byte(jsonStr)) {
    fmt.Println("JSON 有效")
}
```

---

## ❌ 错误处理

### 推荐模式

```go
// 1. 检查错误
result, err := json.GetString(data, "user.name")
if err != nil {
    log.Printf("获取失败：%v", err)
    return err
}

// 2. 使用默认值
name := json.GetStringWithDefault(data, "user.name", "匿名")

// 3. 类型检查
if errors.Is(err, json.ErrTypeMismatch) {
    // 处理类型不匹配
}
```

### Set 操作的安全保证

```go
// 成功：返回修改后的数据
result, err := json.Set(data, "user.name", "Alice")
if err == nil {
    // result 是修改后的 JSON
}

// 失败：返回原始数据（数据不会被破坏）
result, err := json.Set(data, "invalid[path", "value")
if err != nil {
    // result 仍然是有效的原始数据
}
```

---

## 💡 提示

### 性能优化
- ✅ 使用缓存提高重复查询性能
- ✅ 批量操作优于多次单独操作
- ✅ 合理配置大小限制

### 最佳实践
- ✅ 使用类型安全的 GetTyped 方法
- ✅ 为可能缺失的字段使用默认值
- ✅ 在生产环境启用验证
- ✅ 使用 defer processor.Close() 释放资源

### 常见陷阱
- ⚠️ 注意 null 和缺失字段的区别
- ⚠️ 数组索引从 0 开始
- ⚠️ 负数索引从 -1 开始（最后一个元素）
- ⚠️ ForeachWithPath 是只读的，不能修改数据

---

**快速上手，高效开发！** 🚀

