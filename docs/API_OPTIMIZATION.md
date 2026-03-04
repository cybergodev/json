# API Optimization Plan

> **Target**: Simplify API design while maintaining backward compatibility
> **Date**: 2026-03-04

---

## Executive Summary

The current API design follows struct-based configuration patterns correctly, but has complexity issues:

| Issue | Impact | Priority |
|-------|--------|----------|
| Config has 21 fields | Overwhelming for new users | High |
| EncodeConfig has 18 fields | Confusing encoding setup | Medium |
| Config and ProcessorOptions overlap | Unclear which to use | High |
| 40+ `New*` constructors | Decision paralysis | Medium |

---

## Current API Analysis

### 1. Configuration Structures

#### Config (21 fields)
```go
type Config struct {
    // Cache settings (3)
    MaxCacheSize int
    CacheTTL     time.Duration
    EnableCache  bool

    // Size limits (3)
    MaxJSONSize  int64
    MaxPathDepth int
    MaxBatchSize int

    // Security limits (4)
    MaxNestingDepthSecurity   int
    MaxSecurityValidationSize int64
    MaxObjectKeys             int
    MaxArrayElements          int

    // Concurrency (2)
    MaxConcurrency    int
    ParallelThreshold int

    // Processing (5)
    EnableValidation bool
    StrictMode       bool
    CreatePaths      bool
    CleanupNulls     bool
    CompactArrays    bool

    // Additional options (7)
    EnableMetrics     bool
    EnableHealthCheck bool
    AllowComments     bool
    PreserveNumbers   bool
    ValidateInput     bool
    ValidateFilePath  bool
    FullSecurityScan  bool
}
```

#### EncodeConfig (18 fields)
```go
type EncodeConfig struct {
    // Basic (8)
    Pretty          bool
    Indent          string
    Prefix          string
    EscapeHTML      bool
    SortKeys        bool
    ValidateUTF8    bool
    MaxDepth        int
    DisallowUnknown bool

    // Number formatting (3)
    PreserveNumbers bool
    FloatPrecision  int
    FloatTruncate   bool

    // Character escaping (5)
    DisableEscaping bool
    EscapeUnicode   bool
    EscapeSlash     bool
    EscapeNewlines  bool
    EscapeTabs      bool

    // Null handling (2)
    IncludeNulls  bool
    CustomEscapes map[rune]string
}
```

#### ProcessorOptions (11 fields) - Overlaps with Config
```go
type ProcessorOptions struct {
    Context         context.Context  // Unique
    CacheResults    bool             // Unique
    StrictMode      bool             // Shared with Config
    MaxDepth        int              // Unique (different from MaxPathDepth)
    AllowComments   bool             // Shared with Config
    PreserveNumbers bool             // Shared with Config
    CreatePaths     bool             // Shared with Config
    CleanupNulls    bool             // Shared with Config
    CompactArrays   bool             // Shared with Config
    ContinueOnError bool             // Unique
    SkipValidation  bool             // Unique (inverse of EnableValidation)
}
```

### 2. Constructor Summary

| Category | Count | Examples |
|----------|-------|----------|
| Core Processor | 1 | `New()` |
| Encoder/Decoder | 4 | `NewEncoder()`, `NewDecoder()`, `NewCustomEncoder()`, `NewNumberPreservingDecoder()` |
| File Processing | 6 | `NewLargeFileProcessor()`, `NewChunkedReader()`, `NewLazyParser()`, etc. |
| Iterators | 12 | `NewIterator()`, `NewStreamIterator()`, `NewParallelIterator()`, etc. |
| Streaming | 4 | `NewStreamingProcessor()`, `NewBulkProcessor()`, `NewLazyJSON()` |
| Config Factories | 8 | `DefaultConfig()`, `HighSecurityConfig()`, `LargeDataConfig()`, etc. |
| Internal | 15+ | Various internal constructors |

---

## Optimization Strategy

### Phase 1: Add Preset Configurations (Non-Breaking)

Add more factory functions for common use cases:

```go
// WebAPIConfig - optimized for web API handlers
// - Moderate limits
// - HTML escaping enabled
// - Full security scan
func WebAPIConfig() *Config

// FastConfig - optimized for trusted internal services
// - Larger limits
// - Sampling-based security
// - Caching enabled
func FastConfig() *Config

// MinimalConfig - minimal overhead for trusted input
// - Security validation disabled
// - No caching
// - Maximum limits
func MinimalConfig() *Config
```

### Phase 2: Add Convenience Shortcuts (Non-Breaking)

Add package-level shortcuts for common patterns:

```go
// QuickGet - simplest possible get operation
func QuickGet(jsonStr, path string) any

// QuickSet - simplest possible set operation
func QuickSet(jsonStr, path string, value any) string

// QuickPretty - simplest pretty print
func QuickPretty(jsonStr string) string

// QuickCompact - simplest compact
func QuickCompact(jsonStr string) string
```

### Phase 3: Deprecate Redundant Constructors

Mark constructors that have simpler alternatives:

| Constructor | Alternative | Deprecation |
|-------------|-------------|-------------|
| `NewPrettyConfig()` | `DefaultEncodeConfig()` + set `Pretty=true` | Keep |
| `NewReadableConfig()` | Keep | - |
| `NewWebSafeConfig()` | Keep | - |
| `NewCleanConfig()` | Keep | - |

### Phase 4: Group Configuration Fields (Future - Breaking)

This is a breaking change reserved for a major version:

```go
type Config struct {
    Cache     CacheConfig
    Security  SecurityConfig
    Limits    LimitsConfig
    Processing ProcessingConfig
}

type CacheConfig struct {
    MaxSize int
    TTL     time.Duration
    Enabled bool
}

type SecurityConfig struct {
    MaxNestingDepth int
    MaxObjectKeys   int
    MaxArrayElements int
    FullScan        bool
}

type LimitsConfig struct {
    MaxJSONSize  int64
    MaxPathDepth int
    MaxBatchSize int
    MaxConcurrency int
}

type ProcessingConfig struct {
    Validation      bool
    StrictMode      bool
    CreatePaths     bool
    CleanupNulls    bool
    CompactArrays   bool
    AllowComments   bool
    PreserveNumbers bool
}
```

---

## New vs Old API Comparison

### Creating a Processor

| Scenario | Old API | New API |
|----------|---------|---------|
| Default | `json.New()` | Same (no change) |
| Custom | `cfg := json.DefaultConfig(); cfg.MaxCacheSize = 1000; json.New(cfg)` | Same (no change) |
| Web API | Manual configuration | `json.New(json.WebAPIConfig())` |
| High Security | `json.New(json.HighSecurityConfig())` | Same (no change) |
| Fast/Trusted | Manual configuration | `json.New(json.FastConfig())` |

### Encoding JSON

| Scenario | Old API | New API |
|----------|---------|---------|
| Compact | `json.Encode(data)` | Same (no change) |
| Pretty | `json.EncodePretty(data)` | Same (no change) |
| Web Safe | `json.Encode(data, json.NewWebSafeConfig())` | Same (no change) |
| Clean Output | `json.Encode(data, json.NewCleanConfig())` | Same (no change) |

### Quick Operations

| Scenario | Old API | New API |
|----------|---------|---------|
| Get value | `json.Get(jsonStr, "path")` | Same |
| Get with error | `json.Get(jsonStr, "path")` | Same |
| Get or default | `json.GetWithDefault(jsonStr, "path", default)` | Same |
| Quick get (panic on error) | N/A | `json.QuickGet(jsonStr, "path")` |
| Quick pretty | `json.FormatPretty(jsonStr)` | Same or `json.QuickPretty(jsonStr)` |

---

## Migration Guide

### No Migration Required (Phase 1-2)

All changes in Phase 1 and 2 are additive. Existing code continues to work:

```go
// This code continues to work unchanged
processor := json.New()
result, err := processor.Get(jsonStr, "path")
```

### Optional Migration (Recommended)

Users can adopt new patterns gradually:

```go
// Before: Manual configuration for web API
cfg := json.DefaultConfig()
cfg.FullSecurityScan = true
cfg.EscapeHTML = true
cfg.StrictMode = true
processor := json.New(cfg)

// After: Use preset
processor := json.New(json.WebAPIConfig())
```

### Breaking Changes (Phase 4 - Future Major Version)

When grouped configuration is introduced:

```go
// v1.x (current)
cfg := json.DefaultConfig()
cfg.MaxCacheSize = 1000
cfg.StrictMode = true

// v2.x (future)
cfg := json.DefaultConfig()
cfg.Cache.MaxSize = 1000
cfg.Processing.StrictMode = true
```

---

## Deprecation Plan

### Timeframe

| Phase | Release | Timeline | Breaking |
|-------|---------|----------|----------|
| Phase 1 | v1.x | Immediate | No |
| Phase 2 | v1.x | +1 month | No |
| Phase 3 | v1.x | +2 months | No (deprecation warnings only) |
| Phase 4 | v2.0 | +6-12 months | Yes |

### Deprecation Process

1. **Announce**: Add `// Deprecated:` comments
2. **Document**: Update docs with migration guide
3. **Support**: Maintain for 2 major versions
4. **Remove**: Delete in next major version

```go
// Example deprecation comment:
//
// OldConstructor is deprecated: Use NewConstructor instead.
// NewConstructor provides better performance and clearer semantics.
// Migration: Replace OldConstructor(x) with NewConstructor(x).
//
// This function will be removed in v2.0.0.
//
// Deprecated: Use NewConstructor.
func OldConstructor(x int) *Type {
    return NewConstructor(x)
}
```

---

## Implementation Checklist

### Phase 1: Add Preset Configurations

- [ ] Add `WebAPIConfig()` to config.go
- [ ] Add `FastConfig()` to config.go
- [ ] Add `MinimalConfig()` to config.go
- [ ] Add documentation for each preset
- [ ] Add examples for each preset

### Phase 2: Add Convenience Shortcuts

- [ ] Add `QuickGet()` to api.go
- [ ] Add `QuickSet()` to api.go
- [ ] Add `QuickPretty()` to api.go
- [ ] Add `QuickCompact()` to api.go
- [ ] Document panic behavior

### Phase 3: Documentation & Deprecation

- [ ] Review all constructors for redundancy
- [ ] Add deprecation comments where needed
- [ ] Update README with new patterns
- [ ] Create migration examples

### Phase 4: Grouped Configuration (Future)

- [ ] Design grouped Config structure
- [ ] Implement backward compatibility layer
- [ ] Update all factory functions
- [ ] Comprehensive testing
- [ ] Migration tool/script

---

## Success Metrics

| Metric | Current | Target |
|--------|---------|--------|
| Lines of code for basic setup | 5-10 | 1-2 |
| Configuration fields to understand | 21 | 0-3 (using presets) |
| Time to first successful operation | 5 min | 1 min |
| Documentation lookups needed | 3-4 | 0-1 |

---

*End of Optimization Plan*
