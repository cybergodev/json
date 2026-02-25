package internal

import (
	"sync"
	"unsafe"
)

// ============================================================================
// STRING INTERNING
// Reduces memory allocations for frequently used strings (JSON keys, paths)
// PERFORMANCE: Significant memory reduction for JSON with repeated keys
// ============================================================================

// StringIntern stores interned strings for reuse
type StringIntern struct {
	mu      sync.RWMutex
	strings map[string]string
	size    int64
	maxSize int64
	hits    int64
	misses  int64
}

// GlobalStringIntern is the default string interner
var GlobalStringIntern = NewStringIntern(10 * 1024 * 1024) // 10MB max

// NewStringIntern creates a new string interner with a maximum size
func NewStringIntern(maxSize int64) *StringIntern {
	return &StringIntern{
		strings: make(map[string]string),
		maxSize: maxSize,
	}
}

// Intern returns an interned version of the string
// If the string is already interned, returns the existing copy
// Otherwise, stores and returns a copy of the string
func (si *StringIntern) Intern(s string) string {
	if len(s) == 0 {
		return ""
	}

	// Don't intern very long strings
	if len(s) > 256 {
		return s
	}

	// Check if already interned (read lock)
	si.mu.RLock()
	if interned, ok := si.strings[s]; ok {
		si.mu.RUnlock()
		si.hits++
		return interned
	}
	si.mu.RUnlock()

	// Need to add (write lock)
	si.mu.Lock()
	defer si.mu.Unlock()

	// Double-check after acquiring write lock
	if interned, ok := si.strings[s]; ok {
		si.hits++
		return interned
	}

	// Check size limit
	if si.size+int64(len(s)) > si.maxSize {
		// Evict some entries if over limit
		si.evictOldest()
	}

	// Store a copy
	copied := string(unsafe.Slice(unsafe.StringData(s), len(s)))
	si.strings[copied] = copied
	si.size += int64(len(s))
	si.misses++

	return copied
}

// InternBytes returns an interned string from a byte slice
func (si *StringIntern) InternBytes(b []byte) string {
	if len(b) == 0 {
		return ""
	}
	// Convert to string without allocation first
	s := unsafe.String(&b[0], len(b))
	return si.Intern(s)
}

// evictOldest removes entries when size limit is reached
// Uses a simple strategy: remove half the entries
func (si *StringIntern) evictOldest() {
	// Remove half the entries
	count := 0
	target := len(si.strings) / 2
	for k := range si.strings {
		if count >= target {
			break
		}
		si.size -= int64(len(k))
		delete(si.strings, k)
		count++
	}
}

// Stats returns statistics about the string intern
type InternStats struct {
	Entries int
	Size    int64
	Hits    int64
	Misses  int64
}

// GetStats returns current statistics
func (si *StringIntern) GetStats() InternStats {
	si.mu.RLock()
	defer si.mu.RUnlock()

	return InternStats{
		Entries: len(si.strings),
		Size:    si.size,
		Hits:    si.hits,
		Misses:  si.misses,
	}
}

// Clear removes all interned strings
func (si *StringIntern) Clear() {
	si.mu.Lock()
	defer si.mu.Unlock()

	si.strings = make(map[string]string)
	si.size = 0
}

// ============================================================================
// KEY INTERN - Specialized for JSON keys
// ============================================================================

// KeyIntern is a specialized interner for JSON keys
// Uses sharding for better concurrent performance
type KeyIntern struct {
	shards    []*keyInternShard
	shardMask uint64
}

type keyInternShard struct {
	mu      sync.RWMutex
	strings map[string]string
}

// GlobalKeyIntern is the global key interner
var GlobalKeyIntern = NewKeyIntern()

// NewKeyIntern creates a new sharded key interner
func NewKeyIntern() *KeyIntern {
	const shardCount = 16
	shards := make([]*keyInternShard, shardCount)
	for i := range shards {
		shards[i] = &keyInternShard{
			strings: make(map[string]string, 256),
		}
	}
	return &KeyIntern{
		shards:    shards,
		shardMask: uint64(shardCount - 1),
	}
}

// Intern returns an interned version of the key
func (ki *KeyIntern) Intern(key string) string {
	if len(key) == 0 {
		return ""
	}

	// Don't intern very long keys
	if len(key) > 128 {
		return key
	}

	shard := ki.getShard(key)

	// Check read lock first
	shard.mu.RLock()
	if interned, ok := shard.strings[key]; ok {
		shard.mu.RUnlock()
		return interned
	}
	shard.mu.RUnlock()

	// Write lock
	shard.mu.Lock()
	defer shard.mu.Unlock()

	// Double-check
	if interned, ok := shard.strings[key]; ok {
		return interned
	}

	// Store copy
	copied := string(unsafe.Slice(unsafe.StringData(key), len(key)))
	shard.strings[copied] = copied
	return copied
}

// InternBytes returns an interned string from a byte slice
func (ki *KeyIntern) InternBytes(b []byte) string {
	if len(b) == 0 {
		return ""
	}
	s := unsafe.String(&b[0], len(b))
	return ki.Intern(s)
}

// getShard returns the shard for a key using FNV-1a hash
func (ki *KeyIntern) getShard(key string) *keyInternShard {
	h := uint64(14695981039346656037)
	for i := 0; i < len(key); i++ {
		h ^= uint64(key[i])
		h *= 1099511628211
	}
	return ki.shards[h&ki.shardMask]
}

// Clear removes all interned keys
func (ki *KeyIntern) Clear() {
	for _, shard := range ki.shards {
		shard.mu.Lock()
		shard.strings = make(map[string]string, 256)
		shard.mu.Unlock()
	}
}

// ============================================================================
// PATH INTERN - Specialized for JSON paths
// ============================================================================

// PathIntern caches parsed path segments with their string representations
type PathIntern struct {
	mu      sync.RWMutex
	paths   map[string][]PathSegment
	maxSize int
}

// GlobalPathIntern is the global path interner
var GlobalPathIntern = NewPathIntern(50000)

// NewPathIntern creates a new path interner
func NewPathIntern(maxSize int) *PathIntern {
	return &PathIntern{
		paths:   make(map[string][]PathSegment, maxSize/2),
		maxSize: maxSize,
	}
}

// Get retrieves cached path segments
func (pi *PathIntern) Get(path string) ([]PathSegment, bool) {
	pi.mu.RLock()
	segments, ok := pi.paths[path]
	pi.mu.RUnlock()
	return segments, ok
}

// Set stores path segments in cache
func (pi *PathIntern) Set(path string, segments []PathSegment) {
	if len(path) > 256 {
		return // Don't cache very long paths
	}

	pi.mu.Lock()
	defer pi.mu.Unlock()

	// Evict if over limit
	if len(pi.paths) >= pi.maxSize {
		// Remove a random entry (simplified LRU)
		for k := range pi.paths {
			delete(pi.paths, k)
			break
		}
	}

	// Make a copy of segments
	copied := make([]PathSegment, len(segments))
	copy(copied, segments)
	pi.paths[path] = copied
}

// Clear removes all cached paths
func (pi *PathIntern) Clear() {
	pi.mu.Lock()
	defer pi.mu.Unlock()
	pi.paths = make(map[string][]PathSegment, pi.maxSize/2)
}

// ============================================================================
// HELPER FUNCTIONS
// ============================================================================

// InternKey interns a JSON key using the global key interner
func InternKey(key string) string {
	return GlobalKeyIntern.Intern(key)
}

// InternKeyBytes interns a JSON key from bytes
func InternKeyBytes(b []byte) string {
	return GlobalKeyIntern.InternBytes(b)
}

// InternString interns a string using the global string interner
func InternString(s string) string {
	return GlobalStringIntern.Intern(s)
}

// InternStringBytes interns a string from bytes
func InternStringBytes(b []byte) string {
	return GlobalStringIntern.InternBytes(b)
}

// ============================================================================
// BATCH INTERN - For processing multiple keys efficiently
// ============================================================================

// BatchIntern interns multiple strings at once
// More efficient than calling Intern multiple times due to reduced lock overhead
func BatchIntern(strings []string) []string {
	if len(strings) == 0 {
		return strings
	}

	result := make([]string, len(strings))
	intern := GlobalStringIntern
	intern.mu.Lock()

	for i, s := range strings {
		if len(s) == 0 || len(s) > 256 {
			result[i] = s
			continue
		}

		if interned, ok := intern.strings[s]; ok {
			result[i] = interned
			continue
		}

		// Store copy
		copied := string(unsafe.Slice(unsafe.StringData(s), len(s)))
		intern.strings[copied] = copied
		result[i] = copied
	}

	intern.mu.Unlock()
	return result
}

// BatchInternKeys interns multiple keys at once using the key interner
func BatchInternKeys(keys []string) []string {
	if len(keys) == 0 {
		return keys
	}

	result := make([]string, len(keys))
	for i, key := range keys {
		result[i] = GlobalKeyIntern.Intern(key)
	}
	return result
}
