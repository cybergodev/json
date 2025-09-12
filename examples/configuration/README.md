# JSON Library Configuration Usage Examples

This example demonstrates comprehensive configuration options for the `github.com/cybergodev/json` library, showing how to optimize the library for different use cases and requirements.

## 🎯 Demonstrated Features

### 1. Default Configuration Usage
- **Global processor usage** - Using the library with default settings
- **Default processor creation** - Creating processors with default configuration
- **Configuration inspection** - Examining default configuration values

### 2. Custom Configuration Setup
- **Custom cache settings** - Tailored cache size and TTL
- **Size limits configuration** - JSON size, path depth, and batch size limits
- **Performance tuning** - Concurrency and parallel processing settings
- **Processing options** - Path creation, null cleanup, and array compaction

### 3. Performance-Optimized Configuration
- **Aggressive caching** - Maximum cache size and extended TTL
- **High concurrency** - Optimized for multi-threaded environments
- **Large capacity limits** - Handling big JSON files and complex operations
- **Metrics collection** - Performance monitoring and health checks

### 4. Security-Focused Configuration
- **Conservative limits** - Strict size and depth limitations
- **Rate limiting** - Protection against abuse and DoS attacks
- **Input validation** - Comprehensive data validation
- **Secure defaults** - Preventing auto-modification of data

### 5. Cache Configuration Examples
- **High-frequency access cache** - Long TTL for stable data
- **Memory-conscious cache** - Small cache size with short TTL
- **No-cache configuration** - Disabling cache for dynamic data
- **Cache performance comparison** - Demonstrating cache effectiveness

### 6. Processor Options Examples
- **Strict mode options** - Rigorous parsing and validation
- **Flexible mode options** - Permissive parsing with auto-corrections
- **Performance mode options** - Speed-optimized settings
- **Per-operation configuration** - Different options for different operations

### 7. Configuration Validation and Monitoring
- **Comprehensive metrics** - Operation counts, success rates, timing
- **Cache metrics** - Hit rates, miss counts, efficiency
- **Health monitoring** - Processor status and lifecycle management
- **Performance analysis** - Detailed timing and throughput metrics

## 🚀 Running the Example

```bash
cd examples/configuration
go run example.go
```

### Expected Output
The example will demonstrate:
- ✅ Different configuration approaches and their effects
- ✅ Performance comparisons between configuration types
- ✅ Cache behavior with different settings
- ✅ Security features and rate limiting
- ✅ Comprehensive metrics and monitoring data

## 🎯 Configuration Use Cases

### 🏢 Production Environment
```go
config := &json.Config{
    EnableCache:       true,
    MaxCacheSize:      20000,
    CacheTTL:          30 * time.Minute,
    MaxConcurrency:    100,
    EnableMetrics:     true,
    EnableHealthCheck: true,
    EnableValidation:  false, // Disable for performance
}
```

### 🔒 Security-Critical Applications
```go
config := &json.Config{
    MaxJSONSize:      10 * 1024 * 1024, // 10MB limit
    MaxPathDepth:     20,                // Prevent deep nesting
    MaxBatchSize:     100,               // Limit batch operations
    EnableValidation: true,
    StrictMode:       true,
    EnableRateLimit:  true,
    RateLimitPerSec:  1000,
}
```

### 💾 Memory-Constrained Environment
```go
config := &json.Config{
    EnableCache:  true,
    MaxCacheSize: 500,              // Small cache
    CacheTTL:     2 * time.Minute,  // Short TTL
    MaxJSONSize:  5 * 1024 * 1024,  // 5MB limit
    MaxConcurrency: 5,              // Low concurrency
}
```

### ⚡ High-Performance Computing
```go
config := &json.Config{
    EnableCache:       true,
    MaxCacheSize:      50000,           // Large cache
    CacheTTL:          1 * time.Hour,   // Long TTL
    MaxConcurrency:    200,             // High concurrency
    ParallelThreshold: 2,               // Aggressive parallelization
    EnableValidation:  false,           // Skip validation for speed
}
```

## 📊 Configuration Options Reference

### Cache Settings
- **`EnableCache`** - Enable/disable result caching
- **`MaxCacheSize`** - Maximum number of cached entries
- **`CacheTTL`** - Time-to-live for cache entries

### Performance Settings
- **`MaxConcurrency`** - Maximum concurrent operations
- **`ParallelThreshold`** - Minimum items for parallel processing
- **`EnableMetrics`** - Enable performance metrics collection
- **`EnableHealthCheck`** - Enable health monitoring

### Security Settings
- **`MaxJSONSize`** - Maximum JSON file size (bytes)
- **`MaxPathDepth`** - Maximum path nesting depth
- **`MaxBatchSize`** - Maximum batch operation size
- **`EnableRateLimit`** - Enable rate limiting
- **`RateLimitPerSec`** - Operations per second limit

### Processing Settings
- **`EnableValidation`** - Enable input validation
- **`StrictMode`** - Enable strict parsing mode
- **`CreatePaths`** - Auto-create missing paths in Set operations
- **`CleanupNulls`** - Auto-remove null values after deletions
- **`CompactArrays`** - Auto-compact arrays after modifications

## 💡 Best Practices

### 🎯 Configuration Selection
1. **Start with defaults** - Use `json.DefaultConfig()` for most applications
2. **Profile your usage** - Monitor metrics to identify bottlenecks
3. **Tune incrementally** - Adjust one setting at a time
4. **Test thoroughly** - Validate configuration changes in staging

### 🚀 Performance Optimization
1. **Enable caching** - For repeated access to same data
2. **Tune cache size** - Balance memory usage vs hit rate
3. **Adjust concurrency** - Match your system's capabilities
4. **Monitor metrics** - Use built-in performance monitoring

### 🔒 Security Considerations
1. **Set size limits** - Prevent resource exhaustion attacks
2. **Enable validation** - Validate input data integrity
3. **Use rate limiting** - Protect against abuse
4. **Disable auto-modifications** - Prevent unintended data changes

### 📈 Monitoring and Maintenance
1. **Collect metrics** - Monitor performance and health
2. **Set up alerts** - Monitor cache hit rates and error rates
3. **Regular cleanup** - Close processors when done
4. **Resource monitoring** - Watch memory and CPU usage

## 🔗 Related Examples

- [**basic**](../basic/) - Learn basic operations first
- [**json_get**](../json_get/) - Advanced path expressions
- [**json_set**](../json_set/) - Complex modification operations
- [**flat_extraction**](../flat_extraction/) - Advanced extraction techniques

## 🔗 Related Documentation

- [Main Documentation](../../README.md) - Complete API documentation
- [Chinese Documentation](../../docs/doc_zh_CN.md) - Detailed Chinese documentation
- [Compatibility Guide](../../docs/COMPATIBILITY.md) - Migration guide
