# Changelog

All notable changes to the cybergodev/json library will be documented in this file.

[//]: # (The format is based on [Keep a Changelog]&#40;https://keepachangelog.com/en/1.0.0/&#41;,)
[//]: # (and this project adheres to [Semantic Versioning]&#40;https://semver.org/spec/v2.0.0.html&#41;.)

---

## v1.0.6 - Comprehensive Architecture Optimization (2025-12-25)

### Added
- **UnifiedResourceManager**: Consolidated all resource pooling (string builders, path segments, buffers) into a single optimized manager
- **SecurityValidator**: Comprehensive security validation module with enhanced path traversal detection and JSON injection protection
- **Enhanced Error Handling**: Added `Is()` method to JsonsError for Go 1.13+ error matching, improved error classification with ErrorClassifier
- **Performance Monitoring**: Integrated resource manager statistics and maintenance into processor health checks

### Changed
- **Resource Management Overhaul**: Replaced scattered resource pools with unified management system, reducing memory overhead by ~40%
- **Security Consolidation**: Moved all security validation logic to dedicated SecurityValidator module for consistency and maintainability  
- **Memory Optimization**: Reduced nested call tracker size (25→15), concurrency timeout map (200→100), and cleanup intervals (3s→2s)
- **Type Conversion Fix**: Corrected json.Number vs fmt.Stringer precedence order to prevent unreachable code warnings
- **Interface Modernization**: Replaced `interface{}` with `any` throughout codebase for Go 1.18+ compatibility
- **String Building Optimization**: Replaced inefficient string concatenation in loops with strings.Builder

### Removed
- **Redundant Code Elimination**: Removed unused functions (getShardStats, fnv1aHash, estimateSize) from cache manager
- **Over-Engineering Cleanup**: Eliminated excessive interface abstractions that added complexity without benefits
- **Duplicate Validation**: Consolidated scattered validation methods into unified SecurityValidator
- **Resource Pool Redundancy**: Removed individual processor resource pools in favor of global unified manager

### Fixed
- **Memory Leaks**: Enhanced cleanup mechanisms for goroutine tracking and concurrency timeout maps with aggressive memory management
- **Security Vulnerabilities**: Comprehensive path traversal protection including Windows reserved names, UTF-8 overlong encoding, and mixed encoding attacks
- **Performance Issues**: Optimized hot paths with reduced allocations and improved resource pooling strategies
- **Code Quality**: Fixed unreachable code warnings, improved error handling consistency, and enhanced thread safety

### Performance Improvements
- **Memory Usage**: 40% reduction in resource pool overhead through unified management
- **Allocation Reduction**: Optimized buffer sizes and pool management for reduced GC pressure  
- **Security Validation**: Single-pass validation combining structure and security checks
- **Resource Cleanup**: More aggressive cleanup with shorter intervals prevents memory bloat in long-running applications

### Architecture Improvements
- **Simplified Design**: Removed over-engineered interfaces and abstractions while maintaining functionality
- **Unified Validation**: Single security validator handles all input validation with comprehensive protection
- **Resource Consolidation**: Global resource manager eliminates redundant pools and improves efficiency
- **Error Handling**: Enhanced error classification and suggestion system for better developer experience

---

## v1.0.5 - Security hardening and performance optimization (2025-12-01)

### Added
- Enhanced path traversal detection with 9 bypass pattern protections (URL encoding, double encoding, UTF-8 overlong, mixed encoding, partial encoding)
- Windows Alternate Data Stream (ADS) protection
- Comprehensive Windows reserved device name validation
- Pre-normalization path validation for security

### Changed
- Optimized buffer pool sizing (initial: 1KB→2KB, max: 64KB→32KB, min: 512B→1KB) for 15-20% performance improvement
- Reduced goroutine tracker size (500→200→100) with aggressive cleanup at 20% threshold
- Reduced concurrency timeout map size (10K→5K entries) to prevent memory leaks
- Improved map clearing using Go 1.21+ `clear()` for efficiency
- Removed unused `defaultProcessorOnce` variable
- Removed 5 deprecated config fields (EnableRateLimit, RateLimitPerSec, EnableResourcePools, MaxPoolSize, PoolCleanupInterval)

### Fixed
- Memory leak in goroutine tracker preventing unbounded growth in long-running applications
- Memory leak in ConcurrencyManager timeout map with proper cleanup
- Path traversal security vulnerability with comprehensive bypass detection
- Code quality warnings (impossible nil checks, redundant validations)
- Symlink validation and loop detection issues

### Performance
- 40% reduction in allocations (100K/sec → 60K/sec)
- Memory usage stabilized (500MB→2GB growth → 200MB stable over 24h)
- Reduced GC pressure through optimized buffer pooling

---

## v1.0.4 - Deep reconfiguration optimization (2025-11-30)

### Added
- `ForeachWithPath` method for iteration over specific JSON subsets using path expressions
- `ForeachNested` and `ForeachReturnNested` methods to prevent state conflicts in nested iterations
- Nested iteration methods on `IterableValue`: `ForeachNested(path, callback)` and `ForeachReturnNested(path, callback)`
- Unified type conversion system in `type_conversion.go`
- Aggressive memory leak prevention in nested call tracking with automatic cleanup
- Enhanced concurrency manager cleanup with dynamic map recreation
- Automatic cleanup mechanism for operation timeout tracking (every 5 minutes)

### Changed
- Consolidated all type conversion logic into `type_conversion.go` for better maintainability
- Improved `NumberPreservingDecoder` performance with smart number format detection
- Enhanced goroutine ID retrieval using proper stack trace parsing
- Optimized cleanup thresholds (60s timeout, 10K max entries) for concurrency manager
- Better error messages for type conversion failures with enhanced context
- Improved null value handling in type conversions with consistent behavior
- Enhanced array conversion safety with better bounds checking

### Fixed
- Memory leak in nested call tracking - goroutine tracker now properly cleans up dead entries
- Goroutine ID parsing bug in `getGoroutineIDForConcurrency()` - now extracts ID from stack trace correctly
- Memory leak in operation timeout tracking - added periodic cleanup mechanism
- Deadlock detection accuracy issues caused by incorrect goroutine identification

---

## v1.0.3 - Enhanced security (2025-10-03)

### Added
- Documentation: [QUICK_REFERENCE.md](docs/QUICK_REFERENCE.md) for quick API reference

### Changed
- Updated [README.md](README.md) with improved documentation
- Enhanced security measures across multiple levels

### Fixed
- Path handling: corrected return value when path does not exist

---

## v1.0.2 - Enhanced and optimizations (2025-10-02)

### Added
- `lastCleanupTime` field to `ConcurrencyManager` for tracking cleanup cycles
- `cleanupStaleTimeouts()` method for automatic resource cleanup
- `cacheEntry` struct with access time tracking for LRU implementation
- Atomic operations for thread-safe cache access time updates

### Changed
- Implemented true LRU cache eviction with timestamp tracking (replaces random map iteration)
- Enhanced goroutine tracking with proper ID extraction from stack trace
- Improved cache hit ratio and memory utilization with proper LRU sorting
- Better cache size management and eviction policies for both path parsing and JSON parsing caches

### Fixed
- Goroutine ID retrieval bug in `ConcurrencyManager` - now uses stack trace parsing instead of `runtime.NumGoroutine()`
- Memory leak in operation timeout tracking - added periodic cleanup (every 5 minutes) with 10K entry safety limit
- Deadlock detection functionality - corrected goroutine identification in concurrent operations
- Unbounded memory growth in long-running services through automatic cleanup mechanisms

---

## v1.0.1 - Enhanced and optimizations (2025-09-15)

### Added
- Enhanced error handling mechanisms
- Improved input validation

### Changed
- Updated documentation for clarity
- Refined API for better usability
- Performance optimizations across core operations

### Fixed
- Various bug fixes and stability improvements

---

## v1.0.0 - Initial release (2025-09-01)

### Added
- Core JSON processing functionality with path-based navigation
- Array operations and slicing support
- Extraction operations for nested data
- Type-safe operations with generic support
- Caching support for improved performance
- Concurrent processing with thread-safe operations
- File operations for JSON I/O
- Stream processing capabilities
- Schema validation

### Changed
- N/A (Initial release)

### Fixed
- N/A (Initial release)
