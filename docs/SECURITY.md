# 🛡️ Security Guide

This document provides comprehensive security guidelines and best practices for using the `cybergodev/json` library in production environments.

---

## 📋 Table of Contents

- [Overview](#overview)
- [Security Features](#security-features)
- [Configuration](#configuration)
- [Input Validation](#input-validation)
- [Path Security](#path-security)
- [File Operations Security](#file-operations-security)
- [Cache Security](#cache-security)
- [Error Handling](#error-handling)
- [Best Practices](#best-practices)
- [Security Checklist](#security-checklist)
- [Vulnerability Reporting](#vulnerability-reporting)

---

## Overview

The `cybergodev/json` library is designed with security as a core principle. It provides multiple layers of protection against common security threats including:

- **Denial of Service (DoS)** attacks through resource exhaustion
- **Path traversal** attacks in file operations
- **Injection attacks** through malicious JSON paths
- **Memory exhaustion** through oversized JSON payloads
- **Stack overflow** through deeply nested JSON structures
- **Information disclosure** through error messages and caching

---

## Security Features

### 1. Input Validation

The library performs comprehensive validation on all JSON inputs:

```go
// Automatic validation with default configuration
processor, err := json.New()
if err != nil {
    log.Fatal(err)
}
defer processor.Close()

// Validation includes:
// - JSON size limits (default: 100MB)
// - Nesting depth limits (default: 200 levels)
// - Object key count limits (default: 100,000 keys)
// - Array element limits (default: 100,000 elements)
// - Malicious pattern detection
```

### 2. Resource Limits

Built-in protection against resource exhaustion:

- **JSON Size Limit**: Prevents processing of oversized payloads
- **Nesting Depth Limit**: Prevents stack overflow attacks
- **Path Depth Limit**: Prevents excessive path traversal
- **Concurrency Limit**: Prevents thread exhaustion
- **Cache Size Limit**: Prevents memory exhaustion

### 3. Path Sanitization

Automatic sanitization of JSON paths to prevent injection attacks:

```go
// Paths are validated for:
// - Null bytes (\x00)
// - Excessive length (>5,000 characters)
// - Suspicious patterns (script tags, eval, etc.)
// - Path traversal attempts (../, ..\)
```

### 4. Secure Caching

Cache operations include security measures:

- **Sensitive data detection**: Automatic exclusion of sensitive data from cache
- **Secure hashing**: Dual FNV-1a cache key generation
- **Cache key validation**: Prevention of cache injection attacks
- **TTL enforcement**: Automatic expiration of cached data

---

## Configuration

### Default Configuration

The default configuration provides balanced security and performance:

```go
config := json.DefaultConfig()
// Default security settings:
// - MaxJSONSize: 100MB (100 * 1024 * 1024)
// - MaxPathDepth: 50
// - MaxNestingDepthSecurity: 200
// - MaxSecurityValidationSize: 10MB (10 * 1024 * 1024)
// - MaxObjectKeys: 100,000
// - MaxArrayElements: 100,000
// - MaxConcurrency: 50
// - ParallelThreshold: 10
// - MaxBatchSize: 2,000
// - EnableValidation: true
// - ValidateInput: true
// - ValidateFilePath: true
// - StrictMode: false
// - FullSecurityScan: false (uses optimized sampling for large JSON)
```

### High Security Configuration

For high-security environments, use the `SecurityConfig`:

```go
config := json.SecurityConfig()
processor, err := json.New(config)
if err != nil {
    log.Fatal(err)
}
defer processor.Close()

// SecurityConfig settings:
// - MaxJSONSize: 10MB (more restrictive)
// - MaxPathDepth: 30 (more restrictive than DefaultConfig's 50)
// - MaxNestingDepthSecurity: 30 (very restrictive)
// - MaxSecurityValidationSize: 10MB
// - MaxObjectKeys: 5,000 (fewer keys)
// - MaxArrayElements: 5,000 (fewer elements)
// - MaxCacheSize: 256 (larger than DefaultConfig's 128)
// - CacheTTL: 3 minutes (shorter than DefaultConfig's 5 minutes)
// - StrictMode: true (strict validation)
// - EnableValidation: true (forced validation)
// - FullSecurityScan: true (disables sampling, full scan all JSON)

// SECURITY NOTE: FullSecurityScan=true ensures complete content inspection
// but adds ~10-30% overhead for JSON >100KB. Use for:
// - Public APIs and authentication endpoints
// - Processing untrusted input from external sources
// - Financial data and sensitive information
// - Compliance requirements mandating full inspection
```

### Custom Security Configuration

Create a custom configuration for specific security requirements:

```go
config := &json.Config{
    // Size and depth limits
    MaxJSONSize:               5 * 1024 * 1024,  // 5MB
    MaxPathDepth:              30,               // Maximum path depth
    MaxNestingDepthSecurity:   25,               // Maximum nesting depth
    MaxObjectKeys:             5000,             // Maximum object keys
    MaxArrayElements:          5000,             // Maximum array elements
    MaxSecurityValidationSize: 10 * 1024 * 1024, // 10MB validation limit

    // Validation settings
    EnableValidation: true,  // Enable input validation
    StrictMode:       true,  // Enable strict mode
    ValidateInput:    true,  // Validate all inputs

    // Concurrency limits
    MaxConcurrency:    20,   // Maximum concurrent operations
    MaxBatchSize:      100,  // Maximum batch size

    // Cache settings
    EnableCache:  true,
    MaxCacheSize: 1000,
    CacheTTL:     5 * time.Minute,
}

processor, err := json.New(config)
if err != nil {
    log.Fatal(err)
}
defer processor.Close()
```

### Viewing Security Limits

Access security-related configuration fields directly:

```go
// Access security limits directly from Config
fmt.Printf("Max Nesting Depth: %d\n", config.MaxNestingDepthSecurity)
fmt.Printf("Max JSON Size: %d\n", config.MaxJSONSize)
fmt.Printf("Max Path Depth: %d\n", config.MaxPathDepth)
fmt.Printf("Max Object Keys: %d\n", config.MaxObjectKeys)
fmt.Printf("Max Array Elements: %d\n", config.MaxArrayElements)
fmt.Printf("Full Security Scan: %v\n", config.FullSecurityScan)
// Output (DefaultConfig values):
// Max Nesting Depth: 200
// Max JSON Size: 104857600
// Max Path Depth: 50
// Max Object Keys: 100000
// Max Array Elements: 100000
// Full Security Scan: false
// Note: SecurityConfig() uses more restrictive values than DefaultConfig().
```

---

## Input Validation

### Automatic Validation

All JSON inputs are automatically validated when `EnableValidation` is true:

```go
processor, err := json.New()
if err != nil {
    log.Fatal(err)
}
defer processor.Close()

// This will be validated automatically
result, err := processor.Get(jsonString, "path.to.data")
if err != nil {
    // Handle validation errors
    var jsonsErr *json.JsonsError
    if errors.As(err, &jsonsErr) {
        if errors.Is(jsonsErr.Err, json.ErrSizeLimit) {
            log.Printf("JSON too large: %v", err)
        } else if errors.Is(jsonsErr.Err, json.ErrInvalidJSON) {
            log.Printf("Invalid JSON: %v", err)
        }
    }
}
```

### Schema Validation

Use schema validation for strict data validation:

```go
// Define a schema
schema := &json.Schema{
    Type: "object",
    Properties: map[string]*json.Schema{
        "username": {
            Type:      "string",
            MinLength: 3,
            MaxLength: 50,
            Pattern:   "^[a-zA-Z0-9_]+$",
        },
        "email": {
            Type:   "string",
            Format: "email",
        },
        "age": {
            Type:    "number",
            Minimum: 0,
            Maximum: 150,
        },
    },
    Required: []string{"username", "email"},
}

// Validate JSON against schema
errors, err := processor.ValidateSchema(jsonString, schema)
if err != nil {
    log.Printf("Validation failed: %v", err)
}
if len(errors) > 0 {
    for _, validationErr := range errors {
        log.Printf("Validation error at %s: %s",
            validationErr.Path, validationErr.Message)
    }
}
```

### Security Validation

The library performs security-specific validation:

```go
// Checks performed:
// 1. JSON size limits
// 2. Nesting depth limits
// 3. Object key count limits
// 4. Array element limits
// 5. Malicious pattern detection
// 6. Null byte detection
// 7. Excessive string length detection
// 8. Zero-width character detection
// 9. Unicode normalization (NFC) for homograph attack prevention

// Example: Detecting deeply nested JSON
deeplyNested := `{"a":{"b":{"c":{"d":{"e":{"f":{"g":{"h":{"i":{"j":"value"}}}}}}}}}}}`
_, err := processor.Get(deeplyNested, "a.b.c.d.e.f.g.h.i.j")
// Will fail if nesting exceeds MaxNestingDepthSecurity
```

### Optimized Security Scanning

For large JSON (>4KB), the library uses an optimized security scanning approach:

```go
// Default mode (FullSecurityScan: false) - Optimized for performance:
// - Rolling window scan (32KB windows) over the entire JSON content
// - Suspicious character density sampling (4KB beginning, middle, and end regions)
// - Critical patterns (__proto__, constructor, prototype) always fully scanned
// - High suspicious density triggers automatic full scan
// - Pattern fragment detection for targeted scanning

// Full scan mode (FullSecurityScan: true) - Maximum security:
// - All JSON is fully scanned regardless of size
// - Recommended for untrusted input and sensitive data
// - Adds ~10-30% overhead for JSON >100KB
config := json.SecurityConfig()  // Has FullSecurityScan: true by default
```

---

## Path Security

### Path Validation

All JSON paths are validated for security:

```go
// Safe paths
processor.Get(jsonString, "user.profile.name")        // ✓ Safe
processor.Get(jsonString, "items[0].id")              // ✓ Safe
processor.Get(jsonString, "data.users{id,name}")      // ✓ Safe

// Unsafe paths (will be rejected)
processor.Get(jsonString, "path\x00injection")        // ✗ Null byte
processor.Get(jsonString, strings.Repeat("a.", 5001)) // ✗ Too long
processor.Get(jsonString, "../../../etc/passwd")      // ✗ Path traversal
```

### Path Depth Limits

Prevent excessive path traversal:

```go
config := &json.Config{
    MaxPathDepth: 20, // Maximum 20 levels deep
}
processor, err := json.New(config)
if err != nil {
    log.Fatal(err)
}
defer processor.Close()

// This will fail if path depth exceeds limit
result, err := processor.Get(jsonString, "a.b.c.d.e.f.g.h.i.j.k.l.m.n.o.p.q.r.s.t.u")
```



---

## File Operations Security

### File Path Validation

File operations include comprehensive path validation:

```go
processor, err := json.New()
if err != nil {
    log.Fatal(err)
}
defer processor.Close()

// Safe file operations
data, err := processor.LoadFromFile("./data/config.json")  // ✓ Safe
err = processor.SaveToFile("./output/result.json", data) // ✓ Safe

// Unsafe operations (will be rejected)
_, err = processor.LoadFromFile("../../../etc/passwd")     // ✗ Path traversal
_, err = processor.LoadFromFile("/etc/shadow")             // ✗ System directory
_, err = processor.LoadFromFile("file\x00.json")           // ✗ Null byte
```

### Protected System Directories

Access to sensitive system directories is blocked:

```go
// Blocked directories (Unix/Linux):
// - /etc/
// - /proc/
// - /sys/
// - /dev/

// Example:
_, err := processor.LoadFromFile("/etc/passwd")
// Returns error: "access to system directories not allowed"
```

### File Size Limits

File operations respect size limits:

```go
config := &json.Config{
    MaxJSONSize: 10 * 1024 * 1024, // 10MB limit
}
processor, err := json.New(config)
if err != nil {
    log.Fatal(err)
}
defer processor.Close()

// Files larger than MaxJSONSize will be rejected
_, err := processor.LoadFromFile("large_file.json")
if err != nil {
    if errors.Is(err, json.ErrSizeLimit) {
        log.Printf("File too large: %v", err)
    }
}
```

### Secure File Writing

File write operations include safety checks:

```go
// Write with validation
data := map[string]interface{}{
    "status": "success",
    "data":   result,
}

err := processor.SaveToFile("output.json", data)
if err != nil {
    log.Printf("Write failed: %v", err)
}

// The library will:
// 1. Validate the file path
// 2. Check for path traversal
// 3. Validate the data before writing
// 4. Use atomic writes when possible
```

---

## Cache Security

### Sensitive Data Detection

The cache automatically excludes sensitive data by detecting patterns:

```go
// Automatically detected sensitive patterns:
// - Authentication: password, passwd, pwd, token, bearer, jwt, secret, apikey
// - PII: ssn, credit_card, passport, driver_license
// - Financial: account_number, pin, routing_number
// - Session: session_id, cookie, csrf
// - Cloud: aws_access_key, azure_key, gcp_credentials
// - Crypto: private_key, encryption_key, certificate

// These patterns prevent sensitive data from being cached
processor, err := json.New()
if err != nil {
    log.Fatal(err)
}
defer processor.Close()

// Cache will skip values containing sensitive patterns
result, err := processor.Get(jsonString, "user.credentials")
// The result won't be cached if it contains sensitive patterns
```

### Secure Cache Keys

Cache keys use secure hashing:

```go
// Cache keys are generated using dual FNV-1a hashing:
// - All JSON sizes: two independent FNV-1a hashes (different seeds) combined
//   with the input length into a fixed-size validationKey struct {length, h1, h2}.
// - Conceptually "len:h1:h2", but returned as a struct (not a formatted string)
//   to avoid a per-op heap allocation in the hot path.
// - This provides collision resistance below 2^-64.
// This prevents:
// 1. Cache key collision attacks
// 2. Cache timing attacks
// 3. Information disclosure through cache keys

// Key generation (all JSON sizes):
h1 := HashBytesFNV1a([]byte(input))
h2 := HashBytesFNV1aOffset([]byte(input))  // independent seed
cacheKey := validationKey{length: len(input), h1: h1, h2: h2}
```

### Cache Key Validation

Cache keys are validated to prevent injection:

```go
// Invalid cache keys are rejected
// Validation includes:
// - Length checks
// - Character validation
// - Pattern matching
// - Null byte detection
```

### Cache Size Limits

Prevent memory exhaustion through cache limits:

```go
config := &json.Config{
    EnableCache:  true,
    MaxCacheSize: 1000,           // Maximum 1000 entries
    CacheTTL:     5 * time.Minute, // 5 minute expiration
}
processor, err := json.New(config)
if err != nil {
    log.Fatal(err)
}
defer processor.Close()

// Cache will automatically evict old entries when full
// Uses LRU (Least Recently Used) eviction policy
```

---

## Error Handling

### Secure Error Messages

Error messages are sanitized to prevent information disclosure:

```go
processor, err := json.New()
if err != nil {
    log.Fatal(err)
}
defer processor.Close()

_, err := processor.Get(jsonString, "user.password")
if err != nil {
    // Default Error() includes full path - do NOT expose to clients
    fmt.Printf("Error: %v\n", err)
    // Output: "JSON get failed at path 'user.password': ..."

    // Use SafeError() for client-safe messages (redacts path details)
    fmt.Printf("Safe: %s\n", json.SafeError(err))
    // Output: "path not found"

    // Or use RedactedPath() for safe logging
    fmt.Printf("Safe path: %s\n", json.RedactedPath("user.password"))
    // Output: "***"
}
```

### Error Type Checking

Use error type checking for proper handling:

```go
result, err := processor.Get(jsonString, "path")
if err != nil {
    var jsonsErr *json.JsonsError
    if errors.As(err, &jsonsErr) {
        switch {
        case errors.Is(jsonsErr.Err, json.ErrSizeLimit):
            log.Printf("Size limit exceeded: %v", err)
        case errors.Is(jsonsErr.Err, json.ErrSecurityViolation):
            log.Printf("Security violation detected: %v", err)
        case errors.Is(jsonsErr.Err, json.ErrInvalidPath):
            log.Printf("Invalid path: %v", err)
        default:
            log.Printf("Operation failed: %v", err)
        }
    }
}
```

### Error Context

Errors include context without exposing sensitive data:

```go
var jsonsErr *json.JsonsError
if errors.As(err, &jsonsErr) {
    fmt.Printf("Operation: %s\n", jsonsErr.Op)
    fmt.Printf("Path: %s\n", jsonsErr.Path)
    fmt.Printf("Message: %s\n", jsonsErr.Message)
    if jsonsErr.Err != nil {
        fmt.Printf("Underlying error: %v\n", jsonsErr.Err)
    }
}
```

---

## Best Practices

### 1. Use Appropriate Configuration

Choose the right configuration for your environment:

```go
// Development environment
devConfig := json.DefaultConfig()
devConfig.StrictMode = false
devConfig.EnableValidation = true

// Production environment
prodConfig := json.SecurityConfig()
prodConfig.EnableValidation = true
prodConfig.StrictMode = true

// High-security environment (financial, healthcare, etc.)
secureConfig := json.SecurityConfig()
secureConfig.MaxJSONSize = 1 * 1024 * 1024  // 1MB only
secureConfig.MaxPathDepth = 10               // Very shallow
secureConfig.MaxNestingDepthSecurity = 10    // Very restrictive
```

### 2. Validate All External Input

Always validate JSON from external sources:

```go
// Bad: No validation
result, _ := processor.Get(untrustedJSON, "data")

// Good: With validation
processor, err := json.New(json.Config{
    EnableValidation: true,
    StrictMode:       true,
})
if err != nil {
    log.Fatal(err)
}
defer processor.Close()

result, err := processor.Get(untrustedJSON, "data")
if err != nil {
    log.Printf("Validation failed: %v", err)
    return
}
```

### 3. Use Schema Validation for Critical Data

Implement schema validation for important data:

```go
// Define strict schema for user input
userSchema := &json.Schema{
    Type: "object",
    Properties: map[string]*json.Schema{
        "username": {
            Type:      "string",
            MinLength: 3,
            MaxLength: 50,
            Pattern:   "^[a-zA-Z0-9_]+$",
        },
        "email": {
            Type:   "string",
            Format: "email",
        },
    },
    Required:             []string{"username", "email"},
    AdditionalProperties: false, // Reject unknown properties
}

// Validate before processing
errors, err := processor.ValidateSchema(userInput, userSchema)
if err != nil || len(errors) > 0 {
    // Handle validation errors
    return
}
```

### 4. Implement Concurrency Limits

Protect against resource exhaustion with concurrency limits:

```go
config := &json.Config{
    MaxConcurrency: 100, // Maximum concurrent operations
}
processor, err := json.New(config)
if err != nil {
    log.Fatal(err)
}
defer processor.Close()

// Operations will respect concurrency limits automatically
var wg sync.WaitGroup
for i := 0; i < 1000; i++ {
    wg.Add(1)
    go func(id int) {
        defer wg.Done()
        result, err := processor.Get(jsonString, "data")
        if err != nil {
            log.Printf("Worker %d error: %v", id, err)
        }
        // Process result...
    }(i)
}
wg.Wait()
```

### 5. Use Timeouts for Operations

Set timeouts to prevent resource exhaustion:

```go
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()

// Use context-aware operations with boundary-level checks
result, err := json.GetWithContext(ctx, jsonString, "path.to.data")
if err != nil {
    if ctx.Err() != nil {
        log.Printf("Operation timed out: %v", ctx.Err())
    }
    return err
}

// Or use the processor method:
result, err := processor.GetWithContext(ctx, jsonString, "path.to.data")
```

### 6. Monitor Resource Usage

Track resource usage in production:

```go
// Get processor statistics
stats := processor.GetStats()
fmt.Printf("Operations: %d\n", stats.OperationCount)
fmt.Printf("Cache Hit Ratio: %.2f%%\n", stats.HitRatio*100)
fmt.Printf("Error Count: %d\n", stats.ErrorCount)

// Monitor memory usage
var m runtime.MemStats
runtime.ReadMemStats(&m)
fmt.Printf("Alloc: %d MB\n", m.Alloc/1024/1024)
fmt.Printf("TotalAlloc: %d MB\n", m.TotalAlloc/1024/1024)
```

### 7. Sanitize Logs

Ensure logs don't contain sensitive information:

```go
// Bad: Logging raw data
log.Printf("Processing: %s", jsonString)

// Good: Logging sanitized information
log.Printf("Processing JSON of size: %d bytes", len(jsonString))

// Good: Using error messages (already sanitized)
if err != nil {
    log.Printf("Operation failed: %v", err)
}
```

### 8. Close Processors Properly

Always close processors to free resources:

```go
// Use defer to ensure cleanup
processor, err := json.New()
if err != nil {
    log.Fatal(err)
}
defer processor.Close()

// Or use explicit cleanup
processor, err := json.New()
// ... use processor ...
processor.Close()

// Check if processor is closed
_, err := processor.Get(jsonString, "data")
if errors.Is(err, json.ErrProcessorClosed) {
    log.Printf("Processor is closed")
}
```

### 9. Handle Concurrent Access Safely

Use proper synchronization for concurrent operations:

```go
// The processor is thread-safe by default
processor, err := json.New()
if err != nil {
    log.Fatal(err)
}
defer processor.Close()

// Safe concurrent access
var wg sync.WaitGroup
for i := 0; i < 100; i++ {
    wg.Add(1)
    go func(id int) {
        defer wg.Done()
        result, err := processor.Get(jsonString, fmt.Sprintf("items[%d]", id))
        if err != nil {
            log.Printf("Worker %d error: %v", id, err)
        }
        // Process result...
    }(i)
}
wg.Wait()
```

### 10. Implement Defense in Depth

Use multiple layers of security:

```go
// Layer 1: Network/Transport security (TLS, authentication)
// Layer 2: Input validation
config := json.SecurityConfig()
processor, err := json.New(config)
if err != nil {
    log.Fatal(err)
}
defer processor.Close()

// Layer 3: Schema validation
schema := defineStrictSchema()
errors, err := processor.ValidateSchema(input, schema)
if err != nil || len(errors) > 0 {
    return fmt.Errorf("validation failed")
}

// Layer 4: Business logic validation
if !isValidBusinessData(input) {
    return fmt.Errorf("business validation failed")
}

// Layer 5: Rate limiting and monitoring
if rateLimitExceeded() {
    return fmt.Errorf("rate limit exceeded")
}

// Process the data
result, err := processor.Get(input, "data")
```

---

## Security Checklist

Use this checklist to ensure your implementation is secure:

### Configuration
- [ ] Use `SecurityConfig()` for production environments with untrusted input
- [ ] Set appropriate `MaxJSONSize` limits (default: 100MB, high-security: 5MB)
- [ ] Configure `MaxPathDepth` (default: 50) and `MaxNestingDepthSecurity` (default: 200)
- [ ] Enable `EnableValidation` and `ValidateInput` (enabled by default)
- [ ] Set reasonable `MaxConcurrency` limits (default: 50)
- [ ] Configure cache limits with `MaxCacheSize` (default: 128) and `CacheTTL` (default: 5 minutes)
- [ ] Enable `FullSecurityScan: true` for maximum protection with untrusted input

### Input Validation
- [ ] Validate all external JSON input
- [ ] Use schema validation for critical data
- [ ] Check JSON size before processing
- [ ] Validate path strings before use
- [ ] Sanitize user-provided paths
- [ ] Implement input sanitization at application level

### File Operations
- [ ] Validate all file paths
- [ ] Restrict file operations to specific directories
- [ ] Check file sizes before reading
- [ ] Use absolute paths when possible
- [ ] Implement file access logging
- [ ] Set appropriate file permissions

### Error Handling
- [ ] Check all error returns
- [ ] Use typed error checking with `errors.As` and `errors.Is`
- [ ] Log errors appropriately (without sensitive data)
- [ ] Implement error recovery mechanisms
- [ ] Don't expose internal errors to users
- [ ] Provide helpful error messages

### Monitoring
- [ ] Monitor resource usage (CPU, memory)
- [ ] Track operation counts and error rates
- [ ] Monitor cache hit ratios
- [ ] Set up alerts for anomalies
- [ ] Log security-relevant events
- [ ] Implement health checks with `processor.GetHealthStatus()`

### Testing
- [ ] Test with malicious inputs
- [ ] Test with oversized payloads
- [ ] Test with deeply nested structures
- [ ] Test concurrent access patterns
- [ ] Test error handling paths
- [ ] Perform security audits regularly

---

## Common Security Scenarios

### Scenario 1: Processing User-Submitted JSON

```go
func ProcessUserJSON(userInput string) error {
    // Use high security configuration
    config := json.SecurityConfig()
    config.MaxJSONSize = 1 * 1024 * 1024 // 1MB limit for user input

    processor, err := json.New(config)
    if err != nil {
        return fmt.Errorf("failed to create processor: %w", err)
    }
    defer processor.Close()

    // Define strict schema
    schema := &json.Schema{
        Type: "object",
        Properties: map[string]*json.Schema{
            "name": {
                Type:      "string",
                MinLength: 1,
                MaxLength: 100,
            },
            "email": {
                Type:   "string",
                Format: "email",
            },
            "age": {
                Type:    "number",
                Minimum: 0,
                Maximum: 150,
            },
        },
        Required:             []string{"name", "email"},
        AdditionalProperties: false,
    }

    // Validate against schema
    validationErrors, err := processor.ValidateSchema(userInput, schema)
    if err != nil {
        return fmt.Errorf("validation error: %w", err)
    }
    if len(validationErrors) > 0 {
        return fmt.Errorf("invalid input: %v", validationErrors)
    }

    // Process the validated data
    name, err := processor.Get(userInput, "name")
    if err != nil {
        return fmt.Errorf("failed to get name: %w", err)
    }

    // Continue processing...
    log.Printf("Processing user: %v", name)
    return nil
}
```

### Scenario 2: High-Volume API Processing

```go
func ProcessAPIRequests() error {
    // Configure for high volume
    config := &json.Config{
        MaxJSONSize:              5 * 1024 * 1024,
        MaxPathDepth:             50,
        MaxNestingDepthSecurity:  30,
        EnableValidation:         true,
        EnableCache:              true,
        MaxCacheSize:             10000,
        CacheTTL:                 10 * time.Minute,
        MaxConcurrency:           100,
    }

    processor, err := json.New(config)
    if err != nil {
        return fmt.Errorf("failed to create processor: %w", err)
    }
    defer processor.Close()

    // Process requests with monitoring
    ticker := time.NewTicker(1 * time.Minute)
    defer ticker.Stop()

    go func() {
        for range ticker.C {
            stats := processor.GetStats()
            log.Printf("Stats - Ops: %d, Cache Hit: %.2f%%, Errors: %d",
                stats.OperationCount, stats.HitRatio*100, stats.ErrorCount)
        }
    }()

    // Handle requests...
}
```

### Scenario 3: Processing Sensitive Data

```go
func ProcessSensitiveData(jsonData string) error {
    // Disable caching for sensitive data
    config := json.SecurityConfig()
    config.EnableCache = false // Don't cache sensitive data

    processor, err := json.New(config)
    if err != nil {
        return fmt.Errorf("failed to create processor: %w", err)
    }
    defer processor.Close()

    // Process without caching - pass config with caching disabled
    cfg := json.SecurityConfig()
    cfg.CacheResults = false

    result, err := processor.Get(jsonData, "sensitive.data", cfg)
    if err != nil {
        // Don't log the actual data
        log.Printf("Failed to process sensitive data: operation failed")
        return err
    }

    // Process result securely
    // ... (ensure result is not logged or cached)

    return nil
}
```

### Scenario 4: File-Based Configuration

```go
func LoadSecureConfig(configPath string) (*Config, error) {
    // Validate file path (implement your own validation logic)
    if strings.Contains(configPath, "..") || filepath.IsAbs(configPath) {
        return nil, fmt.Errorf("invalid config path: potential path traversal")
    }

    processor, err := json.New(json.SecurityConfig())
    if err != nil {
        return nil, fmt.Errorf("failed to create processor: %w", err)
    }
    defer processor.Close()

    // Read and validate config file
    jsonStr, err := processor.LoadFromFile(configPath)
    if err != nil {
        return nil, fmt.Errorf("failed to read config: %w", err)
    }

    // Define config schema
    configSchema := &json.Schema{
        Type: "object",
        Properties: map[string]*json.Schema{
            "database": {
                Type: "object",
                Properties: map[string]*json.Schema{
                    "host": {Type: "string"},
                    "port": {Type: "number", Minimum: 1, Maximum: 65535},
                },
                Required: []string{"host", "port"},
            },
        },
        Required: []string{"database"},
    }

    // Validate config structure
    errors, err := processor.ValidateSchema(jsonStr, configSchema)
    if err != nil || len(errors) > 0 {
        return nil, fmt.Errorf("invalid config structure")
    }

    // Parse config...
    return parseConfig(jsonStr)
}

func isValidConfigPath(path string) bool {
    // Only allow config files in specific directory
    allowedDir := "./config/"
    absPath, err := filepath.Abs(path)
    if err != nil {
        return false
    }
    allowedAbsDir, err := filepath.Abs(allowedDir)
    if err != nil {
        return false
    }
    return strings.HasPrefix(absPath, allowedAbsDir)
}
```

---

## Security Considerations by Feature

### Path Operations
- **Risk**: Path injection attacks
- **Mitigation**: Automatic path validation and sanitization
- **Best Practice**: Validate user-provided paths before use; use `GetWithContext` for context-aware operations with boundary-level checks

### Batch Operations
- **Risk**: Resource exhaustion through large batches
- **Mitigation**: `MaxBatchSize` configuration limit
- **Best Practice**: Set appropriate batch size limits

### Caching
- **Risk**: Information disclosure through cache
- **Mitigation**: Automatic sensitive data detection
- **Best Practice**: Disable caching for sensitive data

### File Operations
- **Risk**: Path traversal and unauthorized file access
- **Mitigation**: Path validation and system directory blocking
- **Best Practice**: Restrict file operations to specific directories

### Concurrent Operations
- **Risk**: Race conditions and resource exhaustion
- **Mitigation**: Thread-safe implementation and concurrency limits
- **Best Practice**: Set appropriate `MaxConcurrency` limits

### Schema Validation
- **Risk**: Accepting invalid or malicious data
- **Mitigation**: Comprehensive schema validation
- **Best Practice**: Define strict schemas for all external input

---

## Vulnerability Reporting

### Reporting Security Issues

If you discover a security vulnerability in this library, please report it responsibly:

1. **Do NOT** open a public GitHub issue
2. **Do NOT** disclose the vulnerability publicly until it has been addressed
3. **DO** email security details to: [security contact email]
4. **DO** provide detailed information about the vulnerability
5. **DO** include steps to reproduce if possible

### What to Include in Your Report

- Description of the vulnerability
- Affected versions
- Steps to reproduce
- Potential impact
- Suggested fix (if available)
- Your contact information

### Response Timeline

- **Initial Response**: Within 48 hours
- **Vulnerability Assessment**: Within 1 week
- **Fix Development**: Depends on severity
- **Public Disclosure**: After fix is released

### Security Updates

- Security updates are released as patch versions
- Critical vulnerabilities are addressed immediately
- Security advisories are published on GitHub
- Users are notified through release notes

---

## Additional Resources

### Documentation
- [Main README](../README.md) - Library overview and features
- [Compatibility Guide](./COMPATIBILITY.md) - Compatibility information
- [Examples](../examples) - Code examples for all features

### Security Tools
- [Go Security Checker](https://github.com/securego/gosec) - Static analysis
- [Go Vulnerability Database](https://pkg.go.dev/vuln/) - Known vulnerabilities
- [OWASP Go Security](https://owasp.org/www-project-go-secure-coding-practices-guide/) - Security guidelines

### Best Practices
- [OWASP Top 10](https://owasp.org/www-project-top-ten/) - Common security risks
- [CWE Top 25](https://cwe.mitre.org/top25/) - Most dangerous software weaknesses
- [Go Security Best Practices](https://golang.org/doc/security/) - Official Go security guide

---

## 🔬 Technical Security Implementation

This section provides detailed technical documentation of the security mechanisms implemented in the library.

### Security Validation Architecture

The library implements a multi-layered security validation system:

```
┌─────────────────────────────────────────────────────────────────┐
│                    INPUT VALIDATION LAYER                       │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐             │
│  │ Size Check  │→ │ UTF-8 Valid │→ │ BOM Check   │             │
│  └─────────────┘  └─────────────┘  └─────────────┘             │
│         ↓                                                       │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │              SECURITY PATTERN SCANNER                    │   │
│  │  • 28 Dangerous Patterns (XSS, Injection, etc.)         │   │
│  │  • Optimized for Large JSON (>4KB)                      │   │
│  │  • Rolling Window with Overlap (No Gaps)                │   │
│  └─────────────────────────────────────────────────────────┘   │
│         ↓                                                       │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐             │
│  │ Structure   │→ │ Depth Check │→ │ Cache Result│             │
│  │ Validation  │  │ (Max: 200)  │  │ (FNV-1a)    │             │
│  └─────────────┘  └─────────────┘  └─────────────┘             │
└─────────────────────────────────────────────────────────────────┘
```

### Dangerous Pattern Detection

The library detects **28 dangerous patterns** across multiple categories, plus supports custom patterns via `RegisterDangerousPattern`:

#### Prototype Pollution Patterns (Critical — always fully scanned)
```go
// Detected patterns:
"__proto__"           // Prototype pollution
"constructor["        // Constructor access
"prototype."          // Prototype manipulation
"__defineGetter__"    // Getter definition
"__defineSetter__"    // Setter definition
```

#### HTML/XML Injection Patterns
```go
"<script"             // Script tag injection
"<iframe"             // Iframe injection
"<object"             // Object injection
"<embed"              // Embed injection
"<svg"                // SVG injection
```

#### Protocol Injection Patterns
```go
"javascript:"         // JavaScript protocol
"vbscript:"           // VBScript protocol
```

#### Code Execution Patterns
```go
"eval("               // Dynamic code execution
"setTimeout("         // Timer manipulation
"setInterval("        // Interval manipulation
"require("            // Code injection
"new function("       // Dynamic function creation
```

#### DOM Access Patterns
```go
"document.cookie"     // Cookie access
"window.location"     // Redirect manipulation
"innerhtml"           // DOM manipulation
```

#### Encoding Bypass Patterns
```go
"fromcharcode("       // Character encoding bypass
"atob("               // Base64 decoding
"expression("         // CSS expression injection
```

#### Event Handler Patterns
```go
// Detected event handlers:
"onerror", "onload", "onclick", "onmouseover", "onfocus"
```

#### Sensitive Data Detection (for cache exclusion)
The library also detects sensitive data patterns to prevent caching of sensitive values. These are separate from dangerous patterns and are used by the cache layer:

```go
// Authentication:
"password", "passwd", "pwd", "token", "bearer", "jwt",
"access_token", "refresh_token", "auth_token", "secret",
"secret_key", "client_secret", "apikey", "api_key", "api-key",
"x-api-key", "auth", "authorization", "authenticate",
"credential", "credentials", "private"

// PII (Personally Identifiable Information):
"ssn", "social_security", "social_security_number",
"credit_card", "creditcard", "card_number", "cvv", "cvc",
"passport", "passport_number",
"driver_license", "license_number"

// Financial:
"account_number", "bank_account", "routing_number",
"pin", "pin_number"

// Cryptographic:
"private_key", "public_key", "encryption_key", "signing_key",
"certificate", "private_certificate"

// Session:
"session", "session_id", "session_key", "cookie", "csrf", "xsrf"

// Database:
"database_url", "db_password", "db_user", "db_pass",
"connection_string", "connectionstring"

// Cloud:
"aws_access_key", "aws_secret", "aws_key",
"azure_key", "gcp_key", "gcp_credentials"
```

**Note:** Sensitive data patterns prevent values from being cached but do not trigger security violations. Use `AdditionalDangerousPatterns` in Config to add custom dangerous patterns that trigger violations.

### Optimized Security Scanning for Large JSON

For JSON larger than 4KB, the library uses an optimized scanning approach:

#### Rolling Window Algorithm
```
JSON Content: [====32KB Window====][overlap][====32KB Window====]...
                     ↓                          ↓
              Pattern Scan               Pattern Scan

Overlap Size = maxPatternLength + 8 bytes
             = ~30 bytes (ensures no pattern can span windows)
```

#### Multi-Region Sampling
```
┌───────────────────────────────────────────────────────────┐
│  [Sample 4KB]       [Sample 4KB]        [Sample 4KB]     │
│   Beginning    →    Middle Region  →        End          │
│       ↓                  ↓                    ↓           │
│  Density Check     Density Check       Density Check     │
└───────────────────────────────────────────────────────────┘
```

#### Suspicious Character Density Check
```go
// Characters monitored for density attacks:
'<', '>', '(', ')', ';', '=', '&'

// Thresholds:
// - Per-region density: > 0.5% triggers full scan
// - Overall density: > 0.3% triggers full scan
```

### Validation Cache Security

The validation cache uses secure key generation to prevent collision attacks:

```go
// All JSON sizes: two independent FNV-1a hashes (different seeds) combined
// with input length into a fixed-size validationKey struct {length, h1, h2}.
// Conceptually "len:h1:h2" but returned as a struct (not a formatted string)
// to avoid a per-op heap allocation in the hot path.
// Collision resistance below 2^-64.
h1 := HashBytesFNV1a([]byte(json))
h2 := HashBytesFNV1aOffset([]byte(json))  // different seed
key := validationKey{length: len(json), h1: h1, h2: h2}

// Cache management:
// - Maximum entries: configurable via MaxCacheSize (default: 128, max: 2,000)
// - Proactive eviction at 8,000 entries (fixed threshold for the validation cache)
// - Entry includes last access timestamp
```

### Path Traversal Protection

The library implements comprehensive path traversal protection:

#### Multi-Layer Encoding Detection
```
Input Path
    ↓
┌─────────────────────────────────────┐
│  Layer 1: Unicode NFC Normalization │  ← Detects homograph attacks
└─────────────────────────────────────┘
    ↓
┌─────────────────────────────────────┐
│  Layer 2: Recursive URL Decoding    │  ← Up to 3 levels
│  %252e%252e → %2e%2e → ..           │
└─────────────────────────────────────┘
    ↓
┌─────────────────────────────────────┐
│  Layer 3: Pattern Detection         │  ← 30+ traversal patterns
└─────────────────────────────────────┘
    ↓
┌─────────────────────────────────────┐
│  Layer 4: Unicode Lookalike Check   │  ← Fullwidth chars, etc.
└─────────────────────────────────────┘
```

> **Note on NFC scope:** This multi-layer detection applies to **file path**
> validation (`file.go:containsPathTraversal`), where Layer 1 NFC normalization
> runs unconditionally. The **JSON path** validator (`security.go:validatePathSecurity`)
> applies NFC only when the path contains non-ASCII characters (a performance
> optimization that is functionally equivalent for pure-ASCII paths, which are
> unaffected by NFC).

#### Detected Traversal Patterns
```go
// Standard patterns:
"../", "..\\", ".."

// URL encoded:
"%2e%2e", "%252e%252e", "%25252e%25252e"

// Mixed case encoding:
"%2E%2E", "%2E%2e", "%2e%2E", "..%2F", "..%5C"

// Mixed encoding:
"..%2f", "..%5c", "..%c0%af", "..%c1%9c"

// Partial encoding:
".%2e", "%2e.", "%2e%2e%2f", "%2e%2e%5c"

// Control character injection:
"..%00", "..%0a", "..%0d", "..%09", "..%20"
"%00", "%0a", "%0d", "%09", "%20"

// Windows patterns:
"..\\/", "..\\"

// UTF-8 overlong encoding:
"%c0%ae", "%c1%1c", "%c1%9c", "..%255c"

// Fullwidth encoding:
"%uff0e%uff0e", "..%ef%bc%8f"

// Partial double encoding:
"%2e%2", "%25%2e", "%2f%2", "%5c%2"

// Double patterns:
"....//", "....\\\\", ".....", "......"
```

**Note:** The path validation uses pattern-matching heuristics. The above list shows the explicitly detected patterns. Additional patterns may be caught by the general `..` and `%` encoding checks.

#### Unicode Lookalike Character Detection
```go
// Dot lookalikes (resemble "."):
'\uFF0E'  // Fullwidth full stop
'\u2024'  // One dot leader
'\u2025'  // Two dot leader
'\u2026'  // Horizontal ellipsis

// Slash lookalikes (resemble "/" or "\"):
'\uFF0F'  // Fullwidth solidus
'\uFF3C'  // Fullwidth reverse solidus
'\u2044'  // Fraction slash
'\u2215'  // Division slash
'\u29F8'  // Big solidus
'\uFE68'  // Small reverse solidus

// Invisible/formatting characters that could bypass checks:
'\u200B'  // Zero-width space
'\u200C'  // Zero-width non-joiner
'\u200D'  // Zero-width joiner
'\uFEFF'  // Byte order mark
'\u2060'  // Word joiner
'\u00AD'  // Soft hyphen
'\u034F'  // Combining grapheme joiner
'\u3000'  // Ideographic space (fullwidth space)
'\u061C'  // Arabic letter mark
'\u115F'  // Hangul jamo filler
'\u1160'  // Hangul jungseong filler
'\u180E'  // Mongolian vowel separator
```

### Zero-Width Character Detection

The library detects all zero-width and invisible Unicode characters:

```go
// Complete list of detected invisible characters:
'\u200B', '\u200C', '\u200D', // Zero-width characters
'\u200E', '\u200F',           // Directional marks
'\uFEFF',                     // BOM
'\u2060', '\u2061', '\u2062', '\u2063', '\u2064', // Format characters
'\u206A', '\u206B', '\u206C', '\u206D', '\u206E', '\u206F', // Deprecated format chars
'\u00AD', '\u034F', '\u061C', // Other invisible
'\u115F', '\u1160', '\u180E', // Jamo fillers
'\u2066', '\u2067', '\u2068', '\u2069', // Isolate controls
'\uFFFD'                      // Replacement character
```

### Platform-Specific Security

#### Windows Protection
```go
// Reserved device names blocked:
"CON", "PRN", "AUX", "NUL", "CONIN$", "CONOUT$", "CLOCK$"
"COM0"-"COM9", "LPT0"-"LPT9"

// UNC paths blocked:
"\\server\share", "//server/share"

// Alternate Data Streams blocked:
"file.txt:stream"  // ADS not allowed

// Invalid characters:
"<", ">", ":", "\"", "|", "?", "*"
```

#### Unix/Linux Protection
```go
// Protected system directories:
"/dev/", "/proc/", "/sys/", "/etc/",
"/root/", "/boot/", "/var/log/",
"/usr/bin/", "/usr/sbin/", "/sbin/", "/bin/"

// Protected system files:
"/etc/passwd", "/etc/shadow", "/etc/sudoers",
"/etc/hosts", "/etc/fstab", "/etc/crontab"
```

### Memory Safety Mechanisms

#### DeepCopy Protection
```go
const deepCopyMaxDepth = 200  // Maximum recursion depth

func deepCopyValueWithDepth(data any, depth int) (any, error) {
    if depth > deepCopyMaxDepth {
        return nil, fmt.Errorf("deep copy depth limit exceeded")
    }
    // ... recursive copy with depth tracking
}
```

#### Cache Memory Protection
```go
// Cache configuration:
maxCacheKeyLength: 1024              // Maximum key length
securityCacheHighWatermark: 8000     // Fixed threshold for validation cache (independent of MaxCacheSize)
evictionStrategy: LRU                // Least Recently Used

// Key truncation for long keys:
func truncateCacheKey(key string) string {
    // Uses FNV-1a hash suffix for fast truncation (~2ns vs ~100ns for SHA-256)
    prefix := key[:maxLen-19]
    h := HashStringFNV1a(key)  // produces 16-hex-char suffix
    return prefix + "..." + hexEncode(h)
}
```

#### String Interning Limits
```go
// General string interning:
maxSize: 10MB                  // Maximum total memory
evictionAt: 80%                // Proactive eviction
maxStringLength: 256           // Don't intern very long strings

// Key interning:
maxStringLength: 128           // Stricter for keys
shards: 64                     // Concurrency optimization
hotKeyCache: sync.Map          // Lock-free hot keys
```

### Integer Overflow Protection

All type conversions include overflow checking:

```go
func (r AccessResult) AsInt() (int, error) {
    if !r.Exists {
        return 0, ErrPathNotFound
    }
    // Strict type check: bool should not convert to int
    // (bool rejection is done here in AsInt, not in convertToInt)
    switch r.Value.(type) {
    case bool:
        return 0, fmt.Errorf("cannot convert bool to int")
    }
    // Delegates to convertToInt() which handles:
    // - int64 overflow checks (platform-adaptive minInt/maxInt)
    // - uint64 overflow checks (math.MaxInt64)
    // - float64 precision loss detection
    result, ok := convertToInt(r.Value)
    if !ok {
        return 0, fmt.Errorf("cannot convert %T to int", r.Value)
    }
    return result, nil
}
```

### Concurrency Safety

#### Worker Pool Protection
```go
// Parallel processing coordination:
workerCount:   config.MaxConcurrency // Worker count driven by Config (default: 4); capped at the input length
semaphorePool: chan struct{}         // Limit concurrent goroutines
taskTracking:  atomic.Int32          // Track pending tasks
conditionVariable: sync.Cond         // Efficient waiting

// Error handling:
atomic.CompareAndSwapInt32()   // First error wins
```

#### Cache Sharding
```go
// Sharded cache for reduced lock contention:
shardCount: dynamic             // Based on cache size and CPU count:
                                //   Large (>10000): max(CPU*4, 32)
                                //   Medium (>1000): max(CPU*2, 16)
                                //   Small (>100): max(CPU, 8)
                                //   Tiny: max(CPU/2, 4)
shardMask: shardCount - 1      // Fast modulo via AND
perShardMutex: sync.RWMutex    // Fine-grained locking
```

### Security Configuration Options

```go
type Config struct {
    // Size Limits
    MaxJSONSize              int64  // Default: 100MB
    MaxSecurityValidationSize int64 // Default: 10MB

    // Depth Limits
    MaxNestingDepthSecurity  int    // Default: 200
    MaxPathDepth             int    // Default: 50

    // Object Limits
    MaxObjectKeys            int    // Default: 100,000
    MaxArrayElements         int    // Default: 100,000

    // Validation Modes
    EnableValidation         bool   // Default: true
    StrictMode               bool   // Default: false
    FullSecurityScan         bool   // Default: false

    // Concurrency
    MaxConcurrency           int    // Default: 50
    MaxBatchSize             int    // Default: 2000
}
```

### FullSecurityScan Mode

When processing untrusted input, enable `FullSecurityScan`:

```go
// Recommended for:
// - External API input
// - User-submitted data
// - Financial/healthcare data
// - Public-facing services

config := &json.Config{
    FullSecurityScan: true,  // Full scan instead of sampling
}

// Performance impact:
// - Small JSON (<4KB): No impact (always fully scanned)
// - Large JSON (>100KB): ~10-30% overhead
// - Very large JSON (>1MB): ~20-40% overhead
```

### Security Validation Cache

The library caches validation results to avoid redundant security scans:

#### Cache Key Generation
```go
// All JSON sizes: two independent FNV-1a hashes (different seeds) combined
// with input length into a fixed-size validationKey struct {length, h1, h2}.
// Conceptually "len:h1:h2" — length + two independent FNV-1a hashes reduce
// collision probability below 2^-64.
// PERFORMANCE: FNV-1a (~2ns) instead of SHA-256 (~100ns) for ~50x speedup.
// The cache is internal-only, so a collision simply means a redundant validation.
// Returning a fixed-size struct (rather than a formatted "len:h1:h2" string)
// avoids a per-op heap allocation in the hot path.

// getValidationCacheKey is a method on securityValidator (internal)
func (sv *securityValidator) getValidationCacheKey(jsonStr string) validationKey {
    b := internal.StringToBytes(jsonStr)

    // Two independent FNV-1a hashes for strong collision resistance
    h1 := internal.HashBytesFNV1a(b)
    h2 := internal.HashBytesFNV1aOffset(b)  // different seed

    return validationKey{length: len(jsonStr), h1: h1, h2: h2}
}
```

#### Cache Eviction Strategy
```go
// LRU (Least Recently Used) eviction:
// - Triggers at 8,000 entries (fixed threshold for the validation cache)
// - Removes oldest 25% of entries
// - Prevents memory spikes

const securityCacheHighWatermark = 8000  // Fixed threshold for validation cache (independent of MaxCacheSize)

func evictLRUEntries() {
    // Sort by lastAccess time
    // Remove oldest 25%
    // Prevents cache thrashing
}
```

#### Cache Security Guarantees
1. **Collision Resistance**: Dual FNV-1a hashes prevent key collisions (below 2^-64)
2. **Memory Protection**: Proactive eviction at 8,000 entries prevents OOM
3. **Timing Safety**: LRU updates are batched to reduce lock contention

### Optimized Security Scanning

For large JSON (>4KB), the library uses optimized scanning:

#### Rolling Window Algorithm
```
┌────────────────────────────────────────────────────────────────┐
│                    32KB Window                                  │
│  ┌──────────────────────────────────────────────────────────┐  │
│  │                    Scan Window                            │  │
│  │  [byte 0 ............... byte 32768]                     │  │
│  └──────────────────────────────────────────────────────────┘  │
│                         ↓                                       │
│              Skip: WindowSize - OverlapSize                     │
│              (32768 - 48 = 32720 bytes)                         │
│                         ↓                                       │
│  ┌──────────────────────────────────────────────────────────┐  │
│  │              Overlapping Next Window                      │  │
│  │         [byte 32720 ............... byte 65488]          │  │
│  │    ↑ overlap ↑                                           │  │
│  └──────────────────────────────────────────────────────────┘  │
└────────────────────────────────────────────────────────────────┘

// Overlap = maxPatternLength + 8
// Guarantees no pattern can be hidden at window boundaries
```

#### Multi-Region Density Check
```go
// Checks suspicious character density in 3 regions:
// 1. Beginning (0 to 4KB)
// 2. Middle ((len - 4KB) / 2 to (len - 4KB) / 2 + 4KB)
// 3. End (len - 4KB to len)

// Suspicious characters: < > ( ) ; = &
// Threshold per region: 0.5% density triggers full scan

func (sv *securityValidator) hasSuspiciousCharacterDensity(jsonStr string) bool {
    // Sample from beginning, middle, and end
    // Detects attacks hidden anywhere in payload
}
```

#### Fast Path Optimizations
```go
// Skip expensive checks if:
// 1. No letters (only numbers/symbols) - safe
// 2. No '<' or '(' characters - most XSS patterns impossible
// 3. Low suspicious character density

// Critical patterns ALWAYS fully scanned:
// - __proto__, constructor[, prototype.
// - These are too dangerous to miss
```

### Path Validation Details

#### Path Length and Depth Limits
```go
const (
    maxPathLength  = 5000     // Maximum path characters
    maxPathDepth   = 50       // Default maximum path segments (Config.MaxPathDepth)
    maxArrayIndex  = 1000000  // Maximum array index value
)
```

#### Path Security Checks
```go
// 1. Fast Path: Simple alphanumeric paths
//    - Only checks: a-z, A-Z, 0-9, _, .
//    - Skips complex validation

// 2. Slow Path: Complex paths with brackets/braces
//    - Null byte detection
//    - Control character detection
//    - Path traversal detection (../)
//    - Template injection detection (${}, ${{
//    - Array index validation
```

### Zero-Width Character Detection

The library detects invisible Unicode characters that could bypass pattern
matching. See the canonical list under
[Zero-Width Character Detection](#zero-width-character-detection) above \u2014 the
same `containsZeroWidthChars` function is used in both the input-validation and
path-validation layers. It covers zero-width characters (`\u200B\u2013\u200F`),
directional marks, the BOM (`\uFEFF`), format characters (`\u2060\u2013\u2064`,
`\u206A\u2013\u206F`, `\u2066\u2013\u2069`), soft hyphen (`\u00AD`), combining grapheme
joiner (`\u034F`), Arabic letter mark (`\u061C`), Jamo fillers (`\u115F`,
`\u1160`), the Mongolian vowel separator (`\u180E`), and the replacement
character (`\uFFFD`).

### System Directory Protection

#### Unix/Linux Protected Paths
```go
blockedPaths := []string{
    "/dev/",       // Device files
    "/proc/",      // Process information
    "/sys/",       // System information
    "/etc/passwd", // User database
    "/etc/shadow", // Password database
    "/etc/sudoers", // Sudo configuration
    "/etc/hosts",  // Hostname mapping
    "/etc/fstab",  // Filesystem table
    "/etc/crontab", // Cron jobs
    "/root/",      // Root home
    "/boot/",      // Boot files
    "/var/log/",   // Log files
    "/usr/bin/",   // System binaries
    "/usr/sbin/",  // System binaries
    "/sbin/",      // System binaries
    "/bin/",       // System binaries
}
```

#### Windows Protected Items
```go
// Reserved device names:
reservedDevices := []string{
    "CON", "PRN", "AUX", "NUL",
    "COM0", "COM1"-"COM9",
    "LPT0", "LPT1"-"LPT9",
    "CONIN$", "CONOUT$", "CLOCK$",
}

// Blocked patterns:
// - UNC paths (\\server\share)
// - Alternate Data Streams (file.txt:stream)
// - Invalid characters: < > : " | ? *
```

### Security Error Types

```go
var (
    ErrSecurityViolation  // General security error
    ErrSizeLimit          // Size limit exceeded
    ErrDepthLimit         // Nesting depth exceeded
    ErrInvalidPath        // Path validation failed
    ErrInvalidJSON        // JSON validation failed
    ErrConcurrencyLimit   // Too many concurrent operations
)

// Error checking:
if errors.Is(err, json.ErrSecurityViolation) {
    // Handle security violation
}
```

---

## 🤝 Contributing

Contributions are welcome! Please feel free to submit a Pull Request. For major changes, please open an issue first to discuss what you would like to change.

## 🌟 Star History

If you find this project useful, please consider giving it a star! ⭐

---

**Made with ❤️ by the CyberGoDev team**