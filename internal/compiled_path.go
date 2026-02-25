package internal

import (
	"encoding/json"
	"errors"
	"sync"
)

// ============================================================================
// ERROR TYPES FOR COMPILED PATH
// ============================================================================

var (
	// ErrPathNotFound indicates the requested path does not exist
	ErrPathNotFound = errors.New("path not found")
	// ErrTypeMismatch indicates a type mismatch during path navigation
	ErrTypeMismatch = errors.New("type mismatch")
	// ErrInvalidPath indicates an invalid path format
	ErrInvalidPath = errors.New("invalid path")
)

// ============================================================================
// COMPILED PATH
// PERFORMANCE: Pre-parsed paths eliminate parsing overhead for repeated operations
// ============================================================================

// CompiledPath represents a pre-parsed JSON path ready for fast operations
type CompiledPath struct {
	segments []PathSegment
	hash     uint64
	path     string // Original path for debugging
}

// compiledPathPool pools CompiledPath objects to reduce allocations
var compiledPathPool = sync.Pool{
	New: func() any {
		return &CompiledPath{
			segments: make([]PathSegment, 0, 8),
		}
	},
}

// CompilePath parses and compiles a JSON path string into a CompiledPath
// The returned CompiledPath can be reused for multiple operations
func CompilePath(path string) (*CompiledPath, error) {
	// Validate path first
	if err := ValidatePath(path); err != nil {
		return nil, err
	}

	segments, err := ParsePath(path)
	if err != nil {
		return nil, err
	}

	cp := compiledPathPool.Get().(*CompiledPath)
	cp.segments = cp.segments[:0]
	cp.path = path

	// Copy segments to avoid external modification
	if cap(cp.segments) < len(segments) {
		cp.segments = make([]PathSegment, len(segments))
	} else {
		cp.segments = cp.segments[:len(segments)]
	}
	copy(cp.segments, segments)

	// Compute hash for caching
	cp.hash = computePathHash(path)

	return cp, nil
}

// CompilePathUnsafe compiles a path without validation
// Use only when the path is known to be safe
func CompilePathUnsafe(path string) (*CompiledPath, error) {
	segments, err := ParsePath(path)
	if err != nil {
		return nil, err
	}

	cp := compiledPathPool.Get().(*CompiledPath)
	cp.segments = cp.segments[:0]
	cp.path = path

	if cap(cp.segments) < len(segments) {
		cp.segments = make([]PathSegment, len(segments))
	} else {
		cp.segments = cp.segments[:len(segments)]
	}
	copy(cp.segments, segments)

	cp.hash = computePathHash(path)

	return cp, nil
}

// Release returns the CompiledPath to the pool
// Do not use the CompiledPath after calling Release
func (cp *CompiledPath) Release() {
	if cap(cp.segments) <= 32 { // Don't pool very large segment slices
		cp.segments = cp.segments[:0]
		cp.path = ""
		cp.hash = 0
		compiledPathPool.Put(cp)
	}
}

// Segments returns the parsed path segments
func (cp *CompiledPath) Segments() []PathSegment {
	return cp.segments
}

// Hash returns the pre-computed hash of the path
func (cp *CompiledPath) Hash() uint64 {
	return cp.hash
}

// Path returns the original path string
func (cp *CompiledPath) Path() string {
	return cp.path
}

// String returns the path string representation
func (cp *CompiledPath) String() string {
	return cp.path
}

// Len returns the number of segments in the path
func (cp *CompiledPath) Len() int {
	return len(cp.segments)
}

// IsEmpty returns true if the path has no segments
func (cp *CompiledPath) IsEmpty() bool {
	return len(cp.segments) == 0
}

// computePathHash computes a fast hash of the path string
func computePathHash(path string) uint64 {
	// FNV-1a hash
	h := uint64(14695981039346656037)
	for i := 0; i < len(path); i++ {
		h ^= uint64(path[i])
		h *= 1099511628211
	}
	return h
}

// ============================================================================
// COMPILED PATH OPERATIONS
// PERFORMANCE: Direct navigation using pre-parsed segments
// ============================================================================

// Get retrieves a value from parsed JSON data using the compiled path
func (cp *CompiledPath) Get(data any) (any, error) {
	return cp.navigate(data, false)
}

// GetFromRaw retrieves a value from raw JSON bytes using the compiled path
func (cp *CompiledPath) GetFromRaw(raw []byte) (any, error) {
	var data any
	if err := json.Unmarshal(raw, &data); err != nil {
		return nil, err
	}
	return cp.navigate(data, false)
}

// Exists checks if a value exists at the compiled path
func (cp *CompiledPath) Exists(data any) bool {
	_, err := cp.navigate(data, false)
	return err == nil
}

// navigate traverses the data structure using pre-parsed segments
func (cp *CompiledPath) navigate(data any, createPath bool) (any, error) {
	current := data

	for _, segment := range cp.segments {
		if current == nil {
			return nil, NewPathError(segment.Key, "nil value in path", ErrPathNotFound)
		}

		switch segment.Type {
		case PropertySegment:
			obj, ok := current.(map[string]any)
			if !ok {
				return nil, NewPathError(segment.Key, "cannot access property on non-object", ErrTypeMismatch)
			}

			var exists bool
			current, exists = obj[segment.Key]
			if !exists && !createPath {
				return nil, NewPathError(segment.Key, "key not found", ErrPathNotFound)
			}
			if !exists && createPath {
				// Create the path
				obj[segment.Key] = make(map[string]any)
				current = obj[segment.Key]
			}

		case ArrayIndexSegment:
			arr, ok := current.([]any)
			if !ok {
				return nil, NewPathError("", "cannot access index on non-array", ErrTypeMismatch)
			}

			index := segment.Index
			if index < 0 {
				index = len(arr) + index
			}

			if index < 0 || index >= len(arr) {
				return nil, NewPathError("", "array index out of bounds", ErrPathNotFound)
			}
			current = arr[index]

		case ArraySliceSegment:
			arr, ok := current.([]any)
			if !ok {
				return nil, NewPathError("", "cannot slice non-array", ErrTypeMismatch)
			}

			slice, err := applySlice(arr, &segment)
			if err != nil {
				return nil, err
			}
			current = slice

		case WildcardSegment:
			// For wildcards, return all values as an array
			switch v := current.(type) {
			case []any:
				current = v
			case map[string]any:
				values := make([]any, 0, len(v))
				for _, val := range v {
					values = append(values, val)
				}
				current = values
			default:
				return nil, NewPathError("", "wildcard requires array or object", ErrTypeMismatch)
			}
		}
	}

	return current, nil
}

// applySlice applies a slice segment to an array
func applySlice(arr []any, segment *PathSegment) ([]any, error) {
	n := len(arr)

	start := 0
	if segment.HasStart() {
		start = segment.Index
		if start < 0 {
			start = n + start
		}
	}

	end := n
	if segment.HasEnd() {
		end = segment.End
		if end < 0 {
			end = n + end
		}
	}

	step := 1
	if segment.HasStep() {
		step = segment.Step
		if step == 0 {
			return nil, NewPathError("", "slice step cannot be zero", ErrInvalidPath)
		}
	}

	// Clamp bounds
	if start < 0 {
		start = 0
	}
	if end > n {
		end = n
	}
	if start >= end {
		return []any{}, nil
	}

	// Build result
	var result []any
	if step > 0 {
		result = make([]any, 0, (end-start+step-1)/step)
		for i := start; i < end; i += step {
			result = append(result, arr[i])
		}
	} else {
		result = make([]any, 0, (start-end-step-1)/(-step))
		for i := start; i > end; i += step {
			result = append(result, arr[i])
		}
	}

	return result, nil
}

// NewPathError creates a new path error
func NewPathError(path, message string, err error) error {
	return &CompiledPathError{
		Path:    path,
		Message: message,
		Err:     err,
	}
}

// CompiledPathError represents an error during compiled path operations
type CompiledPathError struct {
	Path    string
	Message string
	Err     error
}

// Error implements the error interface
func (e *CompiledPathError) Error() string {
	if e.Path != "" {
		return "path error at '" + e.Path + "': " + e.Message
	}
	return "path error: " + e.Message
}

// Unwrap returns the underlying error
func (e *CompiledPathError) Unwrap() error {
	return e.Err
}

// ============================================================================
// COMPILED PATH CACHE
// PERFORMANCE: Cache frequently used compiled paths
// ============================================================================

// CompiledPathCache caches compiled paths for reuse
type CompiledPathCache struct {
	paths map[string]*CompiledPath
	mu    sync.RWMutex
	max   int
}

// globalCompiledPathCache is the global cache for compiled paths
var globalCompiledPathCache = NewCompiledPathCache(1000)

// NewCompiledPathCache creates a new compiled path cache
func NewCompiledPathCache(max int) *CompiledPathCache {
	return &CompiledPathCache{
		paths: make(map[string]*CompiledPath, max),
		max:   max,
	}
}

// Get retrieves a compiled path from the cache, compiling it if not found
func (c *CompiledPathCache) Get(path string) (*CompiledPath, error) {
	// Check cache first
	c.mu.RLock()
	if cp, ok := c.paths[path]; ok {
		c.mu.RUnlock()
		return cp, nil
	}
	c.mu.RUnlock()

	// Compile the path
	cp, err := CompilePath(path)
	if err != nil {
		return nil, err
	}

	// Store in cache
	c.mu.Lock()
	// Check if another goroutine already cached it
	if existing, ok := c.paths[path]; ok {
		c.mu.Unlock()
		cp.Release()
		return existing, nil
	}

	// Evict if at capacity
	if len(c.paths) >= c.max {
		// Simple eviction: remove a random entry
		for k := range c.paths {
			delete(c.paths, k)
			break
		}
	}

	c.paths[path] = cp
	c.mu.Unlock()

	return cp, nil
}

// GetGlobalCompiledPathCache returns the global compiled path cache
func GetGlobalCompiledPathCache() *CompiledPathCache {
	return globalCompiledPathCache
}

// Clear clears the cache
func (c *CompiledPathCache) Clear() {
	c.mu.Lock()
	c.paths = make(map[string]*CompiledPath, c.max)
	c.mu.Unlock()
}

// Size returns the number of cached paths
func (c *CompiledPathCache) Size() int {
	c.mu.RLock()
	n := len(c.paths)
	c.mu.RUnlock()
	return n
}
