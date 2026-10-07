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
// - Excessive depth (>100 segments)
// - Path traversal attempts (../, ..\, URL-encoded and overlong variants)
// - Zero-width characters; Unicode lookalike detection applies only to file
//   path validation, not JSON paths
//
// Note: script tags, eval(...) and similar dangerous patterns are detected in
// JSON *content*, not in paths (see Dangerous Pattern Detection below).
```

### 4. Secure Caching

Cache operations include security measures:

- **Sensitive data detection**: Automatic exclusion of sensitive data from cache
- **Secure hashing**: FNV-1a cache keys with exact-input verification on every hit
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
// - EnableValidation: true
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
config := json.Config{
    // Size and depth limits
    MaxJSONSize:               5 * 1024 * 1024,  // 5MB
    MaxPathDepth:              30,               // Maximum path depth
    MaxNestingDepthSecurity:   25,               // Maximum nesting depth
    MaxObjectKeys:             5000,             // Maximum object keys
    MaxArrayElements:          5000,             // Maximum array elements
    MaxSecurityValidationSize: 10 * 1024 * 1024, // 10MB validation limit

    // Validation settings (validation is on by default; SkipValidation
    // controls it, not these fields)
    StrictMode:    true, // Enable strict mode
    ValidateInput: true,

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

All JSON inputs are automatically validated. Validation is enabled by default
and runs on every operation; set `SkipValidation: true` only for trusted input:

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
// Define a schema.
// IMPORTANT: numeric and length constraints (MinLength, MaxLength, Minimum,
// Maximum, MinItems, MaxItems, MultipleOf) are only enforced when the schema
// is built with NewSchema. A plain struct literal cannot set the
// internal "constraint present" flags, so such constraints are silently
// skipped — build schemas with NewSchema (pointer fields mark a
// constraint as explicitly set).
minLen, maxLen := 3, 50
zero, maxAge := 0.0, 150.0
schema := json.NewSchema(json.SchemaConfig{
    Type: "object",
    Properties: map[string]*json.Schema{
        "username": json.NewSchema(json.SchemaConfig{
            Type:      "string",
            MinLength: &minLen,
            MaxLength: &maxLen,
            Pattern:   "^[a-zA-Z0-9_]+$",
        }),
        "email": json.NewSchema(json.SchemaConfig{
            Type:   "string",
            Format: "email",
        }),
        "age": json.NewSchema(json.SchemaConfig{
            Type:    "number",
            Minimum: &zero,
            Maximum: &maxAge,
        }),
    },
    Required: []string{"username", "email"},
})

// Validate JSON against schema
validationErrors, err := processor.ValidateSchema(jsonString, schema)
if err != nil {
    log.Printf("Validation failed: %v", err)
}
if len(validationErrors) > 0 {
    for _, validationErr := range validationErrors {
        log.Printf("Validation error at %s: %s",
            validationErr.Path, validationErr.Message)
    }
}
```

### Security Validation

The library performs security-specific validation:

```go
// Checks performed on JSON input:
// 1. JSON size limits
// 2. UTF-8 validity and BOM detection
// 3. Nesting depth limits
// 4. Object key count limits
// 5. Array element limits
// 6. Malicious pattern detection (28 built-in dangerous patterns)
//
// Note: zero-width character and Unicode NFC checks are applied to *paths*
// (see Path Security below), not to JSON content.

// Example: Detecting deeply nested JSON that exceeds the default depth limit
deeplyNested := strings.Repeat(`{"a":`, 250) + `0` + strings.Repeat(`}`, 250)
_, err := processor.Get(deeplyNested, "a")
// Fails when nesting depth exceeds MaxNestingDepthSecurity (default: 200)
```

### Optimized Security Scanning

For large JSON (>4KB), the library uses an optimized security scanning approach:

```go
// Default mode (FullSecurityScan: false) - Optimized for performance:
// - Critical patterns (__proto__, constructor[, prototype.) always fully scanned first
// - Indicator-character fast path: if no indicator characters are present and no
//   per-config additional patterns are set, remaining pattern scanning is skipped
// - Rolling window scan (32KB windows) over the entire JSON content
// - Suspicious character density sampling (4KB beginning, middle, and end regions)
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

JSON paths are capped by fixed internal limits:

```go
processor, err := json.New()
if err != nil {
    log.Fatal(err)
}
defer processor.Close()

// These will be rejected:
// ✗ More than 100 segments (fixed MaxPathParseDepth limit)
_, err = processor.Get(jsonString, strings.Repeat("a.", 101))
// ✗ Longer than 5,000 characters (fixed maxPathLength limit)
_, err = processor.Get(jsonString, strings.Repeat("a.", 5001))

// Note: Config.MaxPathDepth is currently metadata only — it is recorded on
// the Config but NOT enforced on JSON paths. The enforced caps are the fixed
// 100-segment parse depth and 5,000-character length above.
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
// Blocked paths (Unix/Linux):
// - Directory-level: /dev/, /proc/, /sys/, /root/, /boot/, /var/log/,
//   /usr/bin/, /usr/sbin/, /sbin/, /bin/
// - Specific files under /etc/ (the /etc/ directory itself is not blocked):
//   passwd, shadow, sudoers, hosts, fstab, crontab

// Example:
_, err := processor.LoadFromFile("/etc/passwd")
// Returns error: "access to system directory not allowed"
```

### File Size Limits

File operations respect size limits:

```go
config := json.Config{
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
// 4. Write the file atomically (sibling temp file + os.Rename, so a crash
//    mid-write never leaves a truncated file at the target path)
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
// Cache keys are generated using a single FNV-1a hash plus the input length:
// - validationKey struct {length, h1}: one FNV-1a hash
//   (internal.HashStringFNV1a) combined with the exact input length.
// - Returned as a fixed-size struct (not a formatted string) to avoid a
//   per-op heap allocation in the hot path.
// - FNV-1a is non-cryptographic and collision-constructible, so a cache hit
//   is NEVER trusted on the key alone: isValidationCached compares the exact
//   stored input before skipping any security check. A collision merely
//   causes one redundant revalidation, never a false "already validated".
// - Processor result-cache keys use the same single FNV-1a hash, formatted
//   as "op:hash16:path[:optshash]".
// This prevents:
// 1. False "already validated" cache hits (exact-input comparison)
// 2. Cache timing attacks
// 3. Information disclosure through cache keys

// Key generation — internal helper, not part of the public API:
h1 := internal.HashStringFNV1a(input)
cacheKey := validationKey{length: len(input), h1: h1}
```

### Cache Key Validation

Cache keys are validated to prevent injection:

```go
// Invalid cache keys are rejected
// Validation includes:
// - Length checks
// - Character validation
// - Null byte detection
```

### Cache Size Limits

Prevent memory exhaustion through cache limits:

```go
config := json.Config{
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
// Uses frequency-aware LRU eviction: among the 5 least-recently-used tail
// candidates, the one with the lowest frequency (hits) is evicted
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

// Production environment (validation is always on unless SkipValidation
// is explicitly enabled, so it needs no extra setting here)
prodConfig := json.SecurityConfig()
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
// Define strict schema for user input.
// Built with NewSchema so MinLength/MaxLength are enforced (see the
// note under Schema Validation: struct-literal constraints are skipped).
minLen, maxLen := 3, 50
userSchema := json.NewSchema(json.SchemaConfig{
    Type: "object",
    Properties: map[string]*json.Schema{
        "username": json.NewSchema(json.SchemaConfig{
            Type:      "string",
            MinLength: &minLen,
            MaxLength: &maxLen,
            Pattern:   "^[a-zA-Z0-9_]+$",
        }),
        "email": json.NewSchema(json.SchemaConfig{
            Type:   "string",
            Format: "email",
        }),
    },
    Required:             []string{"username", "email"},
    AdditionalProperties: new(bool), // false — reject unknown properties
})

// Validate before processing
validationErrors, err := processor.ValidateSchema(userInput, userSchema)
if err != nil || len(validationErrors) > 0 {
    // Handle validation errors
    return
}
```

### 4. Implement Concurrency Limits

Protect against resource exhaustion with concurrency limits:

```go
config := json.Config{
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
validationErrors, err := processor.ValidateSchema(input, schema)
if err != nil || len(validationErrors) > 0 {
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
- [ ] Set appropriate `MaxJSONSize` limits (default: 100MB, high-security: 10MB)
- [ ] Set `MaxNestingDepthSecurity` appropriately (default: 200; JSON paths are capped by fixed limits: 5,000 chars, 100 segments)
- [ ] Keep input validation enabled (on by default; only `SkipValidation: true` disables it — trusted input only)
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

    // Define strict schema (NewSchema so length/range constraints
    // are enforced — see the note under Schema Validation)
    minLen, maxLen := 1, 100
    zero, maxAge := 0.0, 150.0
    schema := json.NewSchema(json.SchemaConfig{
        Type: "object",
        Properties: map[string]*json.Schema{
            "name": json.NewSchema(json.SchemaConfig{
                Type:      "string",
                MinLength: &minLen,
                MaxLength: &maxLen,
            }),
            "email": json.NewSchema(json.SchemaConfig{
                Type:   "string",
                Format: "email",
            }),
            "age": json.NewSchema(json.SchemaConfig{
                Type:    "number",
                Minimum: &zero,
                Maximum: &maxAge,
            }),
        },
        Required:             []string{"name", "email"},
        AdditionalProperties: new(bool), // false — reject unknown properties
    })

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
    config := json.Config{
        MaxJSONSize:              5 * 1024 * 1024,
        MaxPathDepth:             50,
        MaxNestingDepthSecurity:  30,
        EnableValidation:         true,
        EnableCache:              true,
        MaxCacheSize:             2000, // Config validation silently clamps values above 2000 to 2000 (with a ConfigWarning)
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

    // Define config schema (NewSchema so the port range is
    // enforced — see the note under Schema Validation)
    minPort, maxPort := 1.0, 65535.0
    configSchema := json.NewSchema(json.SchemaConfig{
        Type: "object",
        Properties: map[string]*json.Schema{
            "database": json.NewSchema(json.SchemaConfig{
                Type: "object",
                Properties: map[string]*json.Schema{
                    "host": json.NewSchema(json.SchemaConfig{Type: "string"}),
                    "port": json.NewSchema(json.SchemaConfig{
                        Type: "number", Minimum: &minPort, Maximum: &maxPort,
                    }),
                },
                Required: []string{"host", "port"},
            }),
        },
        Required: []string{"database"},
    })

    // Validate config structure
    validationErrors, err := processor.ValidateSchema(jsonStr, configSchema)
    if err != nil || len(validationErrors) > 0 {
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
3. **DO** report privately via GitHub Security Advisories: https://github.com/cybergodev/json/security/advisories/new
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
│  │ Validation  │  │ (def: 200)  │  │ (FNV-1a)    │             │
│  └─────────────┘  └─────────────┘  └─────────────┘             │
└─────────────────────────────────────────────────────────────────┘
```

### Dangerous Pattern Detection

The library detects **28 dangerous patterns** across multiple categories, plus supports custom patterns via `RegisterDangerousPattern`.

> **Pattern levels:** `PatternLevel` is currently metadata only and does not affect scanning behavior: any registered pattern (custom or global) blocks unconditionally on a match, regardless of level (including Info and Warning). `PatternLevelWarning` and `PatternLevelInfo` are reserved for future tiered enforcement. The 28 built-in patterns carry no level assignment; their behavior is equivalent to `PatternLevelCritical`. Separately, the three prototype-pollution patterns below are members of the `criticalPatterns` set, which means they are **fully scanned regardless of JSON size** even when sampling mode is active.

#### Prototype Pollution Patterns (Critical — always fully scanned)
```go
// These three patterns are in the criticalPatterns set and are always
// fully scanned regardless of JSON size or sampling settings.
"__proto__"           // Prototype pollution
"constructor["        // Constructor access
"prototype."          // Prototype manipulation
```

#### Other Prototype-Related Patterns (optimized scan path)
```go
// These are in the dangerousPatterns list but NOT in criticalPatterns, so they
// follow the optimized/sampling scan path unless FullSecurityScan = true.
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
             = 24 bytes with the built-in patterns (longest is
               "__defineGetter__"/"__defineSetter__" at 16 bytes; grows if
               longer custom patterns are registered)
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
// All JSON sizes: a single FNV-1a hash combined with the input length into
// a fixed-size validationKey struct {length, h1}. A struct (not a formatted
// string) avoids a per-op heap allocation in the hot path.
// SECURITY: FNV-1a is non-cryptographic and collision-constructible, so a
// cache hit is never trusted on the key alone — the exact stored input is
// compared before any security check is skipped.
key := validationKey{length: len(json), h1: HashStringFNV1a(json)}

// Cache management:
// - Maximum entries: ~8,000 (fixed high watermark for the validation cache,
//   independent of MaxCacheSize — MaxCacheSize governs the processor result
//   cache, not the validation cache)
// - Proactive LRU eviction removes the oldest 25% at the watermark
// - Entry includes last access timestamp and the exact validated input
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
"COM0"-"COM99", "LPT0"-"LPT99"

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
"/dev/", "/proc/", "/sys/",
"/root/", "/boot/", "/var/log/",
"/usr/bin/", "/usr/sbin/", "/sbin/", "/bin/"

// Protected system files (only these /etc/ files — the directory
// itself is not blocked, so /etc/myconfig.json remains readable):
"/etc/passwd", "/etc/shadow", "/etc/sudoers",
"/etc/hosts", "/etc/fstab", "/etc/crontab"
```

### Memory Safety Mechanisms

#### DeepCopy Protection
```go
const deepCopyMaxDepth = 200  // Maximum recursion depth

func deepCopyValueWithDepth(data any, depth int) (any, error) {
    if depth > deepCopyMaxDepth {
        return nil, fmt.Errorf("deep copy depth limit exceeded: maximum depth is %d", deepCopyMaxDepth)
    }
    // ... recursive copy with depth tracking
}
```

#### Cache Memory Protection
```go
// Cache configuration:
maxCacheKeyLength: 1024              // Maximum key length
securityCacheHighWatermark: 8000     // Fixed threshold for validation cache (independent of MaxCacheSize)
evictionStrategy: frequency-aware LRU // Evicts the lowest-frequency (hits) entry among the 5 LRU-tail candidates

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
workerCount:   config.MaxConcurrency // Worker count driven by Config (default: 50 with DefaultConfig; 4 is only the fallback when MaxConcurrency <= 0); capped at the input length
semaphorePool: chan struct{}         // Limit concurrent goroutines (ParallelIterator's sem channel only)
hasError:      atomic.Int32          // First-error flag; does not track task counts
// Note: Processor-level in-flight operations are counted by the atomic int64
// activeOps, a counting semaphore rather than a channel.

// Error handling:
atomic.CompareAndSwapInt32()   // First error wins
```

#### Cache Sharding
```go
// Sharded cache for reduced lock contention:
shardCount: dynamic             // Based on cache size and CPU count
                                // (rounded up to a power of 2):
                                //   Large (>10000): max(CPU*4, 32)
                                //   Medium (>1000): max(CPU*2, 16)
                                //   Small (>100): max(CPU, 8)
                                //   Tiny: fixed 4
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

config := json.Config{
    FullSecurityScan: true,  // Full scan instead of sampling
}

// Performance impact:
// - Small JSON (<4KB): No impact (always fully scanned)
// - Large JSON (100KB+): ~10-30% overhead
```

### Security Validation Cache

The library caches validation results to avoid redundant security scans:

#### Cache Key Generation
```go
// All JSON sizes: a single FNV-1a hash (internal.HashStringFNV1a) combined
// with the input length into a fixed-size validationKey struct {length, h1}.
// PERFORMANCE: FNV-1a (~2ns) instead of SHA-256 (~100ns) for ~50x speedup;
// the former second hash (HashBytesFNV1aOffset) was removed after profiling
// showed it cost ~4% of Get CPU on large inputs.
// SECURITY: the key alone is never trusted — isValidationCached compares the
// exact stored input before treating an entry as validated, so a collision
// merely causes one redundant revalidation, never a false "already validated".
// Returning a fixed-size struct (rather than a formatted string) avoids a
// per-op heap allocation in the hot path.

// getValidationCacheKey is a method on securityValidator (internal)
func (sv *securityValidator) getValidationCacheKey(jsonStr string) validationKey {
    return validationKey{
        length: len(jsonStr),
        h1:     internal.HashStringFNV1a(jsonStr),
    }
}
```

#### Cache Eviction Strategy
```go
// LRU (Least Recently Used) eviction:
// - Triggers at 8,000 entries (fixed threshold for the validation cache)
// - Removes oldest 25% of entries
// - Prevents memory spikes

const securityCacheHighWatermark = 8000  // Fixed threshold for validation cache (independent of MaxCacheSize)

func (sv *securityValidator) evictLRUEntries() {
    // Sort by lastAccess time
    // Remove oldest 25%
    // Prevents cache thrashing
}
```

#### Cache Security Guarantees
1. **Collision Safety**: single FNV-1a key plus exact-input comparison on every hit — a hash collision cannot skip validation
2. **Memory Protection**: Proactive eviction at 8,000 entries prevents OOM
3. **Timing Safety**: The validation-cache read path never updates timestamps (avoiding a write lock on hits); lastAccess is refreshed only on writes and re-validation. Batched MoveToFront updates apply to the result cache (internal/cache.go), not this cache

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
│              (32768 - 24 = 32744 bytes)                         │
│                         ↓                                       │
│  ┌──────────────────────────────────────────────────────────┐  │
│  │              Overlapping Next Window                      │  │
│  │         [byte 32744 ............... byte 65512]          │  │
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
// 2. No indicator characters - the pre-computed set covers '<', '(', ':',
//    '.', '_' plus the letters used by dangerous patterns (broad enough to
//    make most XSS/injection patterns impossible)
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
    maxPathDepth   = 100      // Maximum path segments (internal fixed limit)
    maxArrayIndex  = 1000000  // Maximum array index value
)
```

> **Note:** `Config.MaxPathDepth` (default 50, `DefaultMaxPathDepth` in
> config.go) is metadata only - recorded on the config but never enforced in
> any path validation. The enforced limits are the internal fixed values
> above: 5,000 path characters, 100 path segments, array index 1,000,000.

#### Path Security Checks
```go
// 1. Fast Path: Simple alphanumeric paths
//    - Only checks: a-z, A-Z, 0-9, _, .
//    - Skips complex validation

// 2. Slow Path: Complex paths with brackets/braces
//    Syntax checks: bracket matching, array index/slice content validation
//    (max index 1,000,000), segment depth limit
//
//    Security checks (validatePathSecurity, applied to all paths):
//    - Null byte detection
//    - Path traversal detection (../, encoded/overlong variants)
//    - Zero-width character detection
//    - Unicode lookalike detection (file-path layer only, file.go; not applied to JSON paths)
//    - Repeated separator detection (:::, [[[, }}})
```

### Zero-Width Character Detection

The library detects invisible Unicode characters that could bypass pattern
matching. See the canonical list under
[Zero-Width Character Detection](#zero-width-character-detection) above \u2014 the
`containsZeroWidthChars` function is used only in the path-validation layer
(`validatePathSecurity`, security.go); the JSON content-validation layer
(`validateJSONSecurity`) does not detect zero-width characters; it checks
only null bytes, `\uXXXX` normalization, and dangerous patterns. It covers
zero-width characters (`\u200B\u2013\u200F`),
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
    "COM0"-"COM99",
    "LPT0"-"LPT99",
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