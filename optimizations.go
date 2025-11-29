package json

import (
	"hash/fnv"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode"
)

// ============================================================================
// Cache Optimization
// ============================================================================

// OptimizedCache provides a simplified, high-performance cache implementation
type OptimizedCache struct {
	mu        sync.RWMutex
	items     map[uint64]*optimizedCacheItem
	maxSize   int
	ttl       time.Duration
	hits      int64
	misses    int64
	evictions int64
}

type optimizedCacheItem struct {
	value      any
	expireAt   int64 // Unix nano for faster comparison
	accessTime int64 // Unix nano
}

// NewOptimizedCache creates a new optimized cache
func NewOptimizedCache(maxSize int, ttl time.Duration) *OptimizedCache {
	return &OptimizedCache{
		items:   make(map[uint64]*optimizedCacheItem, maxSize),
		maxSize: maxSize,
		ttl:     ttl,
	}
}

// Get retrieves a value from cache
func (c *OptimizedCache) Get(key uint64) (any, bool) {
	c.mu.RLock()
	item, ok := c.items[key]
	c.mu.RUnlock()

	if !ok {
		c.misses++
		return nil, false
	}

	now := time.Now().UnixNano()
	if now > item.expireAt {
		c.misses++
		// Lazy deletion
		c.mu.Lock()
		delete(c.items, key)
		c.mu.Unlock()
		return nil, false
	}

	// Update access time without lock for better performance
	item.accessTime = now
	c.hits++
	return item.value, true
}

// Set stores a value in cache
func (c *OptimizedCache) Set(key uint64, value any) {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now().UnixNano()

	// Check if we need to evict
	if len(c.items) >= c.maxSize {
		c.evictLocked(now)
	}

	c.items[key] = &optimizedCacheItem{
		value:      value,
		expireAt:   now + c.ttl.Nanoseconds(),
		accessTime: now,
	}
}

// evictLocked evicts items (must be called with lock held)
func (c *OptimizedCache) evictLocked(now int64) {
	// First, remove expired items
	for key, item := range c.items {
		if now > item.expireAt {
			delete(c.items, key)
			c.evictions++
		}
	}

	// If still too full, use LRU
	if len(c.items) >= c.maxSize {
		targetSize := c.maxSize * 3 / 4 // Keep 75%
		toEvict := len(c.items) - targetSize

		// Find oldest items
		type keyTime struct {
			key  uint64
			time int64
		}
		oldest := make([]keyTime, 0, toEvict)

		for key, item := range c.items {
			if len(oldest) < toEvict {
				oldest = append(oldest, keyTime{key, item.accessTime})
			} else {
				// Find the newest in oldest list
				maxIdx := 0
				for i := 1; i < len(oldest); i++ {
					if oldest[i].time > oldest[maxIdx].time {
						maxIdx = i
					}
				}
				// Replace if current is older
				if item.accessTime < oldest[maxIdx].time {
					oldest[maxIdx] = keyTime{key, item.accessTime}
				}
			}
		}

		// Evict oldest items
		for _, kt := range oldest {
			delete(c.items, kt.key)
			c.evictions++
		}
	}
}

// Clear removes all items from cache
func (c *OptimizedCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items = make(map[uint64]*optimizedCacheItem, c.maxSize)
}

// Stats returns cache statistics
func (c *OptimizedCache) Stats() CacheStats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	total := c.hits + c.misses
	hitRatio := 0.0
	if total > 0 {
		hitRatio = float64(c.hits) / float64(total)
	}

	return CacheStats{
		HitCount:    c.hits,
		MissCount:   c.misses,
		HitRatio:    hitRatio,
		Evictions:   c.evictions,
		TotalMemory: int64(len(c.items) * 64), // Rough estimate
	}
}

// createOptimizedCacheKey creates a cache key using FNV hash
func createOptimizedCacheKey(operation, jsonStr, path string) uint64 {
	h := fnv.New64a()
	h.Write([]byte(operation))
	h.Write([]byte(path))

	// For small JSON, hash the entire string
	if len(jsonStr) < 1024 {
		h.Write([]byte(jsonStr))
	} else {
		// For large JSON, hash first and last 512 bytes
		h.Write([]byte(jsonStr[:512]))
		h.Write([]byte(jsonStr[len(jsonStr)-512:]))
		// Also include length
		h.Write([]byte(strconv.Itoa(len(jsonStr))))
	}

	return h.Sum64()
}

// ============================================================================
// Memory Optimization
// ============================================================================

// MemoryOptimizer provides memory optimization strategies
type MemoryOptimizer struct {
	// Memory pressure tracking
	lastGC           int64
	gcInterval       time.Duration
	memoryThreshold  int64
	forceGCThreshold int64

	// Pool management
	pools   []PoolCleaner
	poolsMu sync.RWMutex

	// Statistics
	gcCount         int64
	lastMemoryCheck int64
}

// PoolCleaner interface for pools that can be cleaned
type PoolCleaner interface {
	Clean()
}

// NewMemoryOptimizer creates a new memory optimizer
func NewMemoryOptimizer() *MemoryOptimizer {
	return &MemoryOptimizer{
		gcInterval:       30 * time.Second,
		memoryThreshold:  100 * 1024 * 1024, // 100MB
		forceGCThreshold: 500 * 1024 * 1024, // 500MB
		pools:            make([]PoolCleaner, 0),
	}
}

// RegisterPool registers a pool for cleanup
func (mo *MemoryOptimizer) RegisterPool(pool PoolCleaner) {
	mo.poolsMu.Lock()
	defer mo.poolsMu.Unlock()
	mo.pools = append(mo.pools, pool)
}

// CheckMemoryPressure checks if system is under memory pressure
func (mo *MemoryOptimizer) CheckMemoryPressure() bool {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	currentMemory := int64(m.Alloc)
	atomic.StoreInt64(&mo.lastMemoryCheck, time.Now().Unix())

	return currentMemory > mo.memoryThreshold
}

// OptimizeMemory performs memory optimization
func (mo *MemoryOptimizer) OptimizeMemory(force bool) {
	now := time.Now().Unix()
	lastGC := atomic.LoadInt64(&mo.lastGC)

	// Check if we should run GC
	shouldGC := force || (now-lastGC > int64(mo.gcInterval.Seconds()))

	if shouldGC {
		// Clean pools first
		mo.cleanPools()

		// Check memory pressure
		var m runtime.MemStats
		runtime.ReadMemStats(&m)

		// Force GC if memory is high
		if force || int64(m.Alloc) > mo.forceGCThreshold {
			runtime.GC()
			atomic.StoreInt64(&mo.lastGC, now)
			atomic.AddInt64(&mo.gcCount, 1)
		}
	}
}

// cleanPools cleans all registered pools
func (mo *MemoryOptimizer) cleanPools() {
	mo.poolsMu.RLock()
	defer mo.poolsMu.RUnlock()

	for _, pool := range mo.pools {
		if pool != nil {
			pool.Clean()
		}
	}
}

// GetStats returns memory optimization statistics
func (mo *MemoryOptimizer) GetStats() MemoryOptimizerStats {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	return MemoryOptimizerStats{
		CurrentMemory:   int64(m.Alloc),
		TotalAllocated:  int64(m.TotalAlloc),
		SystemMemory:    int64(m.Sys),
		GCCount:         atomic.LoadInt64(&mo.gcCount),
		LastGC:          atomic.LoadInt64(&mo.lastGC),
		NumGoroutines:   int64(runtime.NumGoroutine()),
		MemoryThreshold: mo.memoryThreshold,
	}
}

// MemoryOptimizerStats represents memory optimizer statistics
type MemoryOptimizerStats struct {
	CurrentMemory   int64
	TotalAllocated  int64
	SystemMemory    int64
	GCCount         int64
	LastGC          int64
	NumGoroutines   int64
	MemoryThreshold int64
}

// SmartCache implements a memory-aware cache
type SmartCache struct {
	cache           map[string]any
	mu              sync.RWMutex
	maxSize         int
	currentSize     int
	memoryOptimizer *MemoryOptimizer
}

// NewSmartCache creates a new smart cache
func NewSmartCache(maxSize int, optimizer *MemoryOptimizer) *SmartCache {
	sc := &SmartCache{
		cache:           make(map[string]any, maxSize),
		maxSize:         maxSize,
		memoryOptimizer: optimizer,
	}

	if optimizer != nil {
		optimizer.RegisterPool(sc)
	}

	return sc
}

// Get retrieves a value from cache
func (sc *SmartCache) Get(key string) (any, bool) {
	sc.mu.RLock()
	defer sc.mu.RUnlock()

	value, ok := sc.cache[key]
	return value, ok
}

// Set stores a value in cache
func (sc *SmartCache) Set(key string, value any) {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	// Check if we need to evict
	if sc.currentSize >= sc.maxSize {
		// Check memory pressure
		if sc.memoryOptimizer != nil && sc.memoryOptimizer.CheckMemoryPressure() {
			// Under memory pressure, clear more aggressively
			sc.clearHalf()
		} else {
			// Normal eviction
			sc.evictOne()
		}
	}

	sc.cache[key] = value
	sc.currentSize++
}

// Clean implements PoolCleaner interface
func (sc *SmartCache) Clean() {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	// Clear half of the cache
	sc.clearHalf()
}

// clearHalf clears half of the cache entries
func (sc *SmartCache) clearHalf() {
	targetSize := sc.maxSize / 2
	count := 0

	for key := range sc.cache {
		if count >= targetSize {
			break
		}
		delete(sc.cache, key)
		count++
	}

	sc.currentSize = len(sc.cache)
}

// evictOne evicts one entry from cache
func (sc *SmartCache) evictOne() {
	// Simple eviction: remove first entry
	for key := range sc.cache {
		delete(sc.cache, key)
		sc.currentSize--
		break
	}
}

// Clear clears all cache entries
func (sc *SmartCache) Clear() {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	sc.cache = make(map[string]any, sc.maxSize)
	sc.currentSize = 0
}

// ============================================================================
// String Optimization
// ============================================================================

// StringBuilderPool provides optimized string builder pooling
type StringBuilderPool struct {
	pool sync.Pool
}

// NewStringBuilderPool creates a new string builder pool
func NewStringBuilderPool(initialCapacity int) *StringBuilderPool {
	return &StringBuilderPool{
		pool: sync.Pool{
			New: func() any {
				sb := &strings.Builder{}
				sb.Grow(initialCapacity)
				return sb
			},
		},
	}
}

// Get retrieves a string builder from the pool
func (p *StringBuilderPool) Get() *strings.Builder {
	sb := p.pool.Get().(*strings.Builder)
	sb.Reset()
	return sb
}

// Put returns a string builder to the pool
func (p *StringBuilderPool) Put(sb *strings.Builder) {
	if sb == nil {
		return
	}

	// Only pool builders with reasonable capacity
	if sb.Cap() <= 16384 && sb.Cap() >= 256 {
		sb.Reset()
		p.pool.Put(sb)
	}
}

// OptimizedStringConcat efficiently concatenates strings
func OptimizedStringConcat(parts ...string) string {
	if len(parts) == 0 {
		return ""
	}
	if len(parts) == 1 {
		return parts[0]
	}

	// Calculate total length
	totalLen := 0
	for _, part := range parts {
		totalLen += len(part)
	}

	// Pre-allocate builder
	var sb strings.Builder
	sb.Grow(totalLen)

	for _, part := range parts {
		sb.WriteString(part)
	}

	return sb.String()
}

// JoinPath efficiently joins path segments
func JoinPath(base, segment string) string {
	if base == "" {
		return segment
	}
	if segment == "" {
		return base
	}

	var sb strings.Builder
	sb.Grow(len(base) + len(segment) + 1)
	sb.WriteString(base)
	sb.WriteByte('.')
	sb.WriteString(segment)
	return sb.String()
}

// TruncateString efficiently truncates a string with ellipsis
func TruncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}

	if maxLen <= 3 {
		return s[:maxLen]
	}

	// For long strings, show beginning and end
	if maxLen > 50 {
		prefixLen := maxLen/2 - 7
		suffixLen := maxLen/2 - 7

		var sb strings.Builder
		sb.Grow(maxLen)
		sb.WriteString(s[:prefixLen])
		sb.WriteString("...[truncated]...")
		sb.WriteString(s[len(s)-suffixLen:])
		return sb.String()
	}

	// For shorter strings, just truncate with ...
	var sb strings.Builder
	sb.Grow(maxLen)
	sb.WriteString(s[:maxLen-3])
	sb.WriteString("...")
	return sb.String()
}

// BuildPathWithPrefix efficiently builds a path with prefix
func BuildPathWithPrefix(prefix, path string) string {
	if prefix == "" {
		return path
	}

	// Check if path already starts with prefix
	if strings.HasPrefix(path, prefix+".") {
		return path
	}

	return JoinPath(prefix, path)
}

// RemovePathPrefix efficiently removes a path prefix
func RemovePathPrefix(path, prefix string) string {
	if prefix == "" {
		return path
	}

	prefixWithDot := prefix + "."
	if strings.HasPrefix(path, prefixWithDot) {
		return path[len(prefixWithDot):]
	}

	if path == prefix {
		return ""
	}

	return path
}

// ============================================================================
// Path Validation Optimization
// ============================================================================

// validatePathCharacters validates path characters without regex
func validatePathCharacters(path string) error {
	if len(path) > MaxPathLength {
		return newPathError(path, "path exceeds maximum length", ErrInvalidPath)
	}

	// Check for dangerous characters
	for i, r := range path {
		// Allow alphanumeric, dots, brackets, braces, colons, underscores, hyphens
		if !isAllowedPathChar(r) {
			return newPathError(path, "invalid character at position "+string(rune(i)), ErrInvalidPath)
		}
	}

	return nil
}

// isAllowedPathChar checks if a character is allowed in paths
func isAllowedPathChar(r rune) bool {
	return unicode.IsLetter(r) ||
		unicode.IsDigit(r) ||
		r == '.' ||
		r == '[' ||
		r == ']' ||
		r == '{' ||
		r == '}' ||
		r == ':' ||
		r == '_' ||
		r == '-' ||
		r == '/' ||
		r == '*'
}

// validateBracketBalance validates bracket and brace balance without regex
func validateBracketBalance(path string) error {
	var bracketDepth, braceDepth int

	for i, r := range path {
		switch r {
		case '[':
			bracketDepth++
			if bracketDepth > MaxConsecutiveBrackets {
				return newPathError(path, "too many nested brackets", ErrInvalidPath)
			}
		case ']':
			bracketDepth--
			if bracketDepth < 0 {
				return newPathError(path, "unmatched closing bracket at position "+string(rune(i)), ErrInvalidPath)
			}
		case '{':
			braceDepth++
			if braceDepth > MaxExtractionDepth {
				return newPathError(path, "too many nested braces", ErrInvalidPath)
			}
		case '}':
			braceDepth--
			if braceDepth < 0 {
				return newPathError(path, "unmatched closing brace at position "+string(rune(i)), ErrInvalidPath)
			}
		case ':':
			// Count consecutive colons
			if i > 0 && path[i-1] == ':' {
				consecutiveColons := 1
				for j := i - 1; j >= 0 && path[j] == ':'; j-- {
					consecutiveColons++
				}
				if consecutiveColons > MaxConsecutiveColons {
					return newPathError(path, "too many consecutive colons", ErrInvalidPath)
				}
			}
		}
	}

	if bracketDepth != 0 {
		return newPathError(path, "unmatched opening bracket", ErrInvalidPath)
	}
	if braceDepth != 0 {
		return newPathError(path, "unmatched opening brace", ErrInvalidPath)
	}

	return nil
}

// fastPathValidation performs fast path validation without regex
func fastPathValidation(path string) error {
	if path == "" || path == "." {
		return nil
	}

	// Check path length
	if len(path) > MaxPathLength {
		return newPathError(path, "path too long", ErrInvalidPath)
	}

	// Check for null bytes (security)
	if strings.Contains(path, "\x00") {
		return newPathError(path, "path contains null bytes", ErrInvalidPath)
	}

	// Validate characters
	if err := validatePathCharacters(path); err != nil {
		return err
	}

	// Validate bracket/brace balance
	if err := validateBracketBalance(path); err != nil {
		return err
	}

	return nil
}
