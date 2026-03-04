# API Migration Guide

> **Version**: v1.x → v1.x+ (Non-breaking enhancements)
> **Date**: 2026-03-04

---

## 1. New vs Old API Comparison

### 1.1 Processor Creation

| Use Case | Old API | New API (Recommended) |
|----------|---------|----------------------|
| **Default** | `json.New()` | Same |
| **Custom Config** | `cfg := json.DefaultConfig()`<br>`cfg.MaxCacheSize = 1000`<br>`json.New(cfg)` | Same |
| **Web API** | `cfg := json.DefaultConfig()`<br>`cfg.FullSecurityScan = true`<br>`cfg.StrictMode = true`<br>`json.New(cfg)` | `json.New(json.WebAPIConfig())` |
| **High Security** | `json.New(json.HighSecurityConfig())` | Same |
| **Large Data** | `json.New(json.LargeDataConfig())` | Same |
| **Fast/Trusted** | `cfg := json.DefaultConfig()`<br>`cfg.EnableValidation = false`<br>`cfg.EnableCache = true`<br>`json.New(cfg)` | `json.New(json.FastConfig())` |
| **Minimal** | `cfg := json.DefaultConfig()`<br>`cfg.EnableCache = false`<br>`cfg.EnableValidation = false`<br>`json.New(cfg)` | `json.New(json.MinimalConfig())` |

### 1.2 JSON Encoding

| Use Case | Old API | New API |
|----------|---------|---------|
| **Compact** | `json.Encode(data)` | Same |
| **Pretty** | `json.EncodePretty(data)` | Same |
| **Custom Indent** | `cfg := json.NewPrettyConfig()`<br>`cfg.Indent = "    "`<br>`json.Encode(data, cfg)` | Same |
| **Web Safe** | `json.Encode(data, json.NewWebSafeConfig())` | Same |
| **Human Readable** | `json.Encode(data, json.NewReadableConfig())` | Same |
| **Clean (no nulls)** | `json.Encode(data, json.NewCleanConfig())` | Same |
| **HTML Safe** | `cfg := json.DefaultEncodeConfig()`<br>`cfg.EscapeHTML = true`<br>`json.Encode(data, cfg)` | `json.Encode(data, json.NewWebSafeConfig())` |

### 1.3 JSON Path Operations

| Use Case | Old API | New API |
|----------|---------|---------|
| **Get value** | `val, err := json.Get(jsonStr, "path")` | Same |
| **Get typed** | `val, err := json.GetString(jsonStr, "path")` | Same |
| **Get with default** | `val := json.GetStringWithDefault(jsonStr, "path", "default")` | Same |
| **Set value** | `result, err := json.Set(jsonStr, "path", value)` | Same |
| **Set with create** | `result, err := json.SetWithAdd(jsonStr, "path", value)` | Same |
| **Delete** | `result, err := json.Delete(jsonStr, "path")` | Same |
| **Quick get** | N/A | `val := json.QuickGet(jsonStr, "path")` |
| **Quick set** | N/A | `result := json.QuickSet(jsonStr, "path", value)` |

### 1.4 File Operations

| Use Case | Old API | New API |
|----------|---------|---------|
| **Load file** | `data, err := json.LoadFromFile("file.json")` | Same |
| **Save file** | `err := json.SaveToFile("file.json", data, true)` | Same |
| **Marshal to file** | `err := json.MarshalToFile("file.json", data)` | Same |
| **Unmarshal from file** | `err := json.UnmarshalFromFile("file.json", &v)` | Same |

---

## 2. Configuration Presets

### 2.1 Config Presets Comparison

| Preset | Security | Performance | Use Case |
|--------|----------|-------------|----------|
| `DefaultConfig()` | Balanced | Balanced | General use |
| `HighSecurityConfig()` | Maximum | Lower | Untrusted input |
| `LargeDataConfig()` | Moderate | Higher | Large JSON files |
| `WebAPIConfig()` | High | Moderate | Public APIs |
| `FastConfig()` | Minimal | Maximum | Trusted internal |
| `MinimalConfig()` | Disabled | Maximum | Benchmarking |

### 2.2 EncodeConfig Presets Comparison

| Preset | Pretty | Escape HTML | Nulls | Use Case |
|--------|--------|-------------|-------|----------|
| `DefaultEncodeConfig()` | No | Yes | Include | Standard |
| `NewPrettyConfig()` | Yes | Yes | Include | Debugging |
| `NewReadableConfig()` | No | No | Include | Logs |
| `NewWebSafeConfig()` | No | Yes | Include | Web APIs |
| `NewCleanConfig()` | Yes | Yes | Exclude | Clean output |

---

## 3. Migration Examples

### 3.1 Basic Migration (No Changes Required)

```go
// v1.x - Continue to work
processor := json.New()
result, err := processor.Get(jsonStr, "user.name")
```

### 3.2 Adopt Presets

```go
// Before: Manual configuration
cfg := json.DefaultConfig()
cfg.MaxCacheSize = 512
cfg.FullSecurityScan = true
cfg.StrictMode = true
cfg.EscapeHTML = true
processor := json.New(cfg)

// After: Use preset
processor := json.New(json.WebAPIConfig())
```

### 3.3 Quick Operations

```go
// Before: Error handling required
result, err := json.Get(jsonStr, "path")
if err != nil {
    log.Fatal(err)
}
fmt.Println(result)

// After: Quick operation (panics on error - use for trusted input)
fmt.Println(json.QuickGet(jsonStr, "path"))
```

### 3.4 Encoding Migration

```go
// Before: Multiple lines
cfg := json.DefaultEncodeConfig()
cfg.Pretty = true
cfg.Indent = "  "
cfg.SortKeys = true
cfg.IncludeNulls = false
result, err := json.Encode(data, cfg)

// After: Use preset
result, err := json.Encode(data, json.NewCleanConfig())
```

---

## 4. Deprecation Schedule

| API | Status | Removal | Migration |
|-----|--------|---------|-----------|
| All current APIs | Active | N/A | No migration needed |
| Future deprecated APIs | Deprecated | v2.0.0 | See deprecation notice |

**Note**: Phase 1-3 changes are non-breaking. Phase 4 (grouped config) is planned for v2.0.0.

---

## 5. Best Practices

### 5.1 Choosing the Right Config

```
┌─────────────────────────────────────────────────────────────┐
│                    Input Source?                            │
├─────────────────────────────────────────────────────────────┤
│  Untrusted (public API)  →  HighSecurityConfig()            │
│  Semi-trusted (partners) →  WebAPIConfig()                  │
│  Trusted (internal)      →  FastConfig() or DefaultConfig() │
│  Benchmarking/Testing    →  MinimalConfig()                 │
│  Large files (>10MB)     →  LargeDataConfig()               │
└─────────────────────────────────────────────────────────────┘
```

### 5.2 When to Use Quick Functions

```go
// ✅ Good: Quick functions for trusted input in simple scripts
name := json.QuickGet(userJSON, "name")

// ✅ Good: Quick functions in tests
result := json.QuickSet(`{}`, "test", "value")

// ❌ Bad: Quick functions with untrusted input
userInput := getRequest.Body()
data := json.QuickGet(userInput, "data") // May panic on malformed JSON

// ✅ Good: Proper error handling for untrusted input
data, err := json.Get(userInput, "data")
if err != nil {
    return fmt.Errorf("invalid request: %w", err)
}
```

### 5.3 Encoding Choice

```go
// For web responses
json.Encode(data, json.NewWebSafeConfig())

// For logging/debugging
json.EncodePretty(data)

// For clean API responses (no nulls)
json.Encode(data, json.NewCleanConfig())

// For config files
json.Encode(data, json.NewReadableConfig())
```

---

## 6. Common Patterns

### 6.1 Web Handler

```go
func HandleUser(w http.ResponseWriter, r *http.Request) {
    processor := json.New(json.WebAPIConfig())

    body, _ := io.ReadAll(r.Body)
    userID, err := processor.GetString(string(body), "user_id")
    if err != nil {
        http.Error(w, "Invalid request", 400)
        return
    }

    user := getUser(userID)
    result, _ := json.Encode(user, json.NewWebSafeConfig())
    w.Write([]byte(result))
}
```

### 6.2 Config File Processing

```go
func LoadConfig(path string) (*Config, error) {
    processor := json.New(json.FastConfig()) // Trusted config files

    data, err := processor.LoadFromFile(path)
    if err != nil {
        return nil, err
    }

    var cfg Config
    if err := processor.Unmarshal([]byte(data), &cfg); err != nil {
        return nil, err
    }
    return &cfg, nil
}
```

### 6.3 Large File Streaming

```go
func ProcessLargeFile(path string) error {
    processor := json.New(json.LargeDataConfig())

    file, _ := os.Open(path)
    defer file.Close()

    reader := json.NewStreamIterator(file)
    for reader.Next() {
        item := reader.Value()
        // Process each item
    }
    return reader.Error()
}
```

---

*End of Migration Guide*
