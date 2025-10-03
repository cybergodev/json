# cybergodev/json - Release Notes


==================================================


## v1.0.3 - 2025-10-03

### Added
- Add the document [QUICK_REFERENCE.md](docs/QUICK_REFERENCE.md)QUICK_REFERENCE.md

### Changed
- Updated document [README.md](README.md)
- Enhance several levels of security

### Fixed
- Return value is returned when the repair path does not exist.

---

## v1.0.2 - 2025-10-02

### 🔧 Fixed

#### Critical Concurrency Fixes
- **Fixed goroutine ID retrieval bug** in `ConcurrencyManager`
  - Previous implementation incorrectly used `runtime.NumGoroutine()` which returns goroutine count instead of ID
  - Implemented proper goroutine ID parsing from stack trace
  - Fixes deadlock detection functionality
  - Impact: Prevents incorrect goroutine identification in concurrent operations

- **Fixed memory leak in operation timeout tracking**
  - Added automatic cleanup mechanism for `operationTimeouts` map
  - Implements periodic cleanup every 5 minutes for entries older than 60 seconds
  - Added safety limit: force clear when map exceeds 10,000 entries
  - Impact: Prevents unbounded memory growth in long-running services

#### Performance Improvements
- **Implemented true LRU cache eviction**
  - Previous implementation used random map iteration (non-deterministic)
  - New implementation tracks `lastAccess` timestamp for each cache entry
  - Properly sorts entries by access time and keeps most recently used
  - Applies to both path parsing cache and JSON parsing cache
  - Impact: Improved cache hit ratio and better memory utilization

### 🚀 Enhanced

#### Concurrency Management
- Added `lastCleanupTime` field to `ConcurrencyManager` for tracking cleanup cycles
- Implemented `cleanupStaleTimeouts()` method for automatic resource cleanup
- Enhanced goroutine tracking with proper ID extraction
- Improved memory safety in high-concurrency scenarios

#### Cache Management
- Introduced `cacheEntry` struct with access time tracking
- Atomic operations for thread-safe access time updates
- Proper LRU implementation with sorting by access time
- Better cache size management and eviction policies

### 📊 Performance Metrics

- **Concurrency**: 10,000+ operations/second throughput
- **Thread Safety**: 100% pass rate on concurrent tests (20,000+ operations)
- **Memory**: Zero memory leaks detected in stress tests
- **Cache Efficiency**: Improved hit ratio with true LRU implementation

### 🔒 Security & Stability

- ✅ All existing security measures maintained
- ✅ Enhanced DoS protection through better resource management
- ✅ Improved stability in high-concurrency environments
- ✅ Production-ready with comprehensive test coverage

### ⚠️ Breaking Changes

None. This release is fully backward compatible.

### 🎯 Upgrade Recommendation

**Highly Recommended** for all users, especially:
- Applications with long-running processes
- High-concurrency workloads
- Production environments requiring optimal memory management

---

## v1.0.1 - Previous Release

### Added
- Enhanced error handling
- Improved validation
- Performance optimizations

### Changed
- Updated documentation
- Refined API

### Fixed
- Various bug fixes

---

## v1.0.0 - Initial version

### Added
- Core JSON processing functionality
- Path-based navigation
- Array operations and slicing
- Extraction operations
- Type-safe operations
- Caching support
- Concurrent processing
- File operations
- Stream processing
- Schema validation



