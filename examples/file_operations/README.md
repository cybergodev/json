# 📁 文件操作示例

本示例演示了如何使用 JSON 库的包级别文件操作方法，无需创建处理器实例即可进行文件读写操作。

## 🚀 包级别方法 vs 处理器方法

### 包级别方法（推荐用于简单操作）
```go
// 直接使用，无需创建处理器
data, err := json.LoadFromFile("config.json")
err = json.SaveToFile("output.json", data, true)
```

### 处理器方法（用于需要自定义配置的场景）
```go
// 需要创建和管理处理器
processor := json.New(json.DefaultConfig())
defer processor.Close()

data, err := processor.LoadFromFile("config.json")
err = processor.SaveToFile("output.json", data, true)
```

## 📋 示例内容

### 1. 基础文件操作
- **文件加载**: 使用 `json.LoadFromFile()` 从文件加载 JSON 数据
- **数据修改**: 使用 `json.Set()` 和 `json.SetWithAdd()` 修改数据
- **文件保存**: 使用 `json.SaveToFile()` 保存数据到文件
- **格式选择**: 支持美化输出和压缩输出

### 2. 流式操作
- **Reader 加载**: 使用 `json.LoadFromReader()` 从 io.Reader 加载数据
- **Writer 保存**: 使用 `json.SaveToWriter()` 保存数据到 io.Writer
- **内存优化**: 适合处理大文件，避免一次性加载到内存

### 3. 批量文件处理
- **多文件加载**: 批量处理多个配置文件
- **数据合并**: 将多个文件的数据合并为一个配置
- **错误处理**: 单个文件失败不影响其他文件的处理

### 4. 带选项的文件操作
- **严格模式**: 使用 `ProcessorOptions` 进行严格验证
- **灵活模式**: 允许注释、自动创建路径等
- **自定义配置**: 根据需求调整处理选项

### 5. 错误处理
- **文件不存在**: 处理文件不存在的情况
- **路径无效**: 处理无效保存路径
- **优雅降级**: 加载失败时使用默认配置

## 🔧 核心方法

### 文件读取方法

| 方法 | 签名 | 描述 |
|------|------|------|
| `LoadFromFile` | `LoadFromFile(filePath string, opts ...*ProcessorOptions) (any, error)` | 从文件加载 JSON 数据 |
| `LoadFromReader` | `LoadFromReader(reader io.Reader, opts ...*ProcessorOptions) (any, error)` | 从 Reader 加载 JSON 数据 |

### 文件写入方法

| 方法 | 签名 | 描述 |
|------|------|------|
| `SaveToFile` | `SaveToFile(filePath string, data any, pretty bool, opts ...*ProcessorOptions) error` | 保存数据到文件 |
| `SaveToWriter` | `SaveToWriter(writer io.Writer, data any, pretty bool, opts ...*ProcessorOptions) error` | 保存数据到 Writer |

## 🎯 使用场景

### 适合使用包级别方法的场景：
- ✅ 简单的配置文件读写
- ✅ 一次性的文件处理任务
- ✅ 脚本和工具开发
- ✅ 不需要特殊配置的场景

### 适合使用处理器方法的场景：
- 🔧 需要自定义缓存配置
- 🔧 高频率的文件操作
- 🔧 需要特殊验证规则
- 🔧 长期运行的服务

## 🚀 运行示例

```bash
# 进入示例目录
cd examples/file_operations

# 运行示例
go run example.go
```

## 📊 示例输出

```
🚀 JSON 文件操作示例
===================

📁 包级别文件操作方法演示
================================

1️⃣ 基础文件操作
---------------
📖 从文件加载 JSON:
   服务器配置: localhost:3000

🔧 修改配置:
   新端口: 8080
   SSL 启用: true

💾 保存到文件:
   ✅ 美化版本已保存到 config_updated.json
   ✅ 压缩版本已保存到 config_compact.json

2️⃣ 流式操作
------------
📖 从 Reader 加载大文件:
   加载了 1000 条记录

💾 保存到 Writer:
   ✅ 数据已写入 Buffer，大小: 31234 字节
   ✅ 处理后的数据已保存到 processed_data.json

3️⃣ 批量文件处理
---------------
📁 批量处理配置文件:
   处理文件: config.json
   ✅ config.json 加载成功
   处理文件: users.json
   ✅ users.json 加载成功

💾 保存合并配置:
   ✅ 合并配置已保存到 merged_config.json
   📊 合并了 2 个配置文件

4️⃣ 带选项的文件操作
------------------
🔒 严格模式加载:
   ✅ 严格模式加载成功

🔓 灵活模式加载:
   ✅ 灵活模式加载成功
   严格模式结果: localhost
   灵活模式结果: localhost

5️⃣ 错误处理示例
---------------
❌ 加载不存在的文件:
   预期错误: failed to read file nonexistent.json: open nonexistent.json: no such file or directory

❌ 保存到无效路径:
   预期错误: failed to write file /invalid/path/file.json: open /invalid/path/file.json: no such file or directory

✅ 正确的错误处理:
   服务器主机: localhost

🧹 清理示例文件
---------------
   ✅ 已删除: config.json
   ✅ 已删除: users.json
   ✅ 已删除: large_data.json
   ✅ 已删除: config_updated.json
   ✅ 已删除: config_compact.json
   ✅ 已删除: processed_data.json
   ✅ 已删除: merged_config.json
```

## 💡 最佳实践

### 1. 选择合适的方法
```go
// ✅ 简单场景使用包级别方法
data, err := json.LoadFromFile("config.json")

// ✅ 复杂场景使用处理器
processor := json.New(&json.Config{
    EnableCache: true,
    MaxJSONSize: 100 * 1024 * 1024,
})
defer processor.Close()
data, err := processor.LoadFromFile("config.json")
```

### 2. 错误处理
```go
// ✅ 总是检查错误
data, err := json.LoadFromFile("config.json")
if err != nil {
    // 处理错误或使用默认值
    log.Printf("加载配置失败: %v", err)
    data = getDefaultConfig()
}
```

### 3. 文件格式选择
```go
// ✅ 开发环境使用美化格式
err = json.SaveToFile("config.json", data, true)

// ✅ 生产环境使用压缩格式
err = json.SaveToFile("config.json", data, false)
```

### 4. 大文件处理
```go
// ✅ 大文件使用流式操作
file, err := os.Open("large_data.json")
if err != nil {
    return err
}
defer file.Close()

data, err := json.LoadFromReader(file)
```

## 🔗 相关示例

- [基础操作示例](../basic_operations/) - JSON 数据的基础读写操作
- [路径操作示例](../path_operations/) - 复杂路径表达式的使用
- [配置管理示例](../configuration/) - 处理器配置和选项
- [性能优化示例](../performance/) - 高性能 JSON 处理技巧
