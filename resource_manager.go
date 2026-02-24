package json

import (
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cybergodev/json/internal"
)

// UnifiedResourceManager consolidates all resource management for optimal performance
// sync.Pool is inherently thread-safe, no additional locks needed for pool operations
type UnifiedResourceManager struct {
	stringBuilderPool *sync.Pool
	pathSegmentPool   *sync.Pool
	bufferPool        *sync.Pool
	optionsPool       *sync.Pool // Pool for ProcessorOptions to reduce allocations

	allocatedBuilders int64
	allocatedSegments int64
	allocatedBuffers  int64
	allocatedOptions  int64

	lastCleanup     int64
	cleanupInterval int64
}

// NewUnifiedResourceManager creates a new unified resource manager
func NewUnifiedResourceManager() *UnifiedResourceManager {
	return &UnifiedResourceManager{
		stringBuilderPool: &sync.Pool{
			New: func() any {
				sb := &strings.Builder{}
				sb.Grow(512)
				return sb
			},
		},
		pathSegmentPool: &sync.Pool{
			New: func() any {
				return make([]internal.PathSegment, 0, 8)
			},
		},
		bufferPool: &sync.Pool{
			New: func() any {
				return make([]byte, 0, 1024)
			},
		},
		optionsPool: &sync.Pool{
			New: func() any {
				opts := DefaultOptionsClone() // Use clone for pool objects
				return opts
			},
		},
		cleanupInterval: 300,
		lastCleanup:     time.Now().Unix(),
	}
}

func (urm *UnifiedResourceManager) GetStringBuilder() *strings.Builder {
	obj := urm.stringBuilderPool.Get()
	sb, ok := obj.(*strings.Builder)
	if !ok {
		// Pool corruption detected: type assertion failed
		// Log this rare event for debugging purposes
		slog.Debug("pool corruption detected: string builder type assertion failed", "type", fmt.Sprintf("%T", obj))
		// Fallback: create new builder if type assertion fails
		sb = &strings.Builder{}
		sb.Grow(512)
	}
	sb.Reset()
	atomic.AddInt64(&urm.allocatedBuilders, 1)
	return sb
}

func (urm *UnifiedResourceManager) PutStringBuilder(sb *strings.Builder) {
	// Use consistent size limits from constants.go
	const maxBuilderCap = MaxPoolBufferSize // 8192 - consistent with constants
	const minBuilderCap = MinPoolBufferSize // 256 - consistent with constants

	if sb == nil {
		return
	}

	c := sb.Cap()
	if c >= minBuilderCap && c <= maxBuilderCap {
		sb.Reset()
		urm.stringBuilderPool.Put(sb)
	}
	// oversized builders are discarded to prevent pool bloat

	// Decrement counter after processing (whether returned to pool or discarded)
	atomic.AddInt64(&urm.allocatedBuilders, -1)
}

func (urm *UnifiedResourceManager) GetPathSegments() []internal.PathSegment {
	obj := urm.pathSegmentPool.Get()
	segments, ok := obj.([]internal.PathSegment)
	if !ok {
		// Pool corruption detected: type assertion failed
		// Log this rare event for debugging purposes
		slog.Debug("pool corruption detected: path segments type assertion failed", "type", fmt.Sprintf("%T", obj))
		// Fallback: create new slice if type assertion fails
		segments = make([]internal.PathSegment, 0, 8)
	}
	segments = segments[:0]
	atomic.AddInt64(&urm.allocatedSegments, 1)
	return segments
}

func (urm *UnifiedResourceManager) PutPathSegments(segments []internal.PathSegment) {
	// Stricter segment pool limits
	const maxSegmentCap = 32 // Reduced from 64
	const minSegmentCap = 4  // Keep minimum

	if segments == nil {
		return
	}

	if cap(segments) >= minSegmentCap && cap(segments) <= maxSegmentCap {
		segments = segments[:0]
		urm.pathSegmentPool.Put(segments)
	}
	// oversized segments are discarded to prevent pool bloat

	// Decrement counter after processing (whether returned to pool or discarded)
	atomic.AddInt64(&urm.allocatedSegments, -1)
}

func (urm *UnifiedResourceManager) GetBuffer() []byte {
	obj := urm.bufferPool.Get()
	buf, ok := obj.([]byte)
	if !ok {
		// Pool corruption detected: type assertion failed
		// Log this rare event for debugging purposes
		slog.Debug("pool corruption detected: buffer type assertion failed", "type", fmt.Sprintf("%T", obj))
		// Fallback: create new buffer if type assertion fails
		buf = make([]byte, 0, 1024)
	}
	buf = buf[:0]
	atomic.AddInt64(&urm.allocatedBuffers, 1)
	return buf
}

func (urm *UnifiedResourceManager) PutBuffer(buf []byte) {
	// Use consistent size limits from constants.go
	const maxBufferCap = MaxPoolBufferSize // 8192 - consistent with constants
	const minBufferCap = MinPoolBufferSize // 256 - consistent with constants

	if buf == nil {
		return
	}

	if cap(buf) >= minBufferCap && cap(buf) <= maxBufferCap {
		buf = buf[:0]
		urm.bufferPool.Put(buf)
	}
	// oversized buffers are discarded to prevent pool bloat

	// Decrement counter after processing (whether returned to pool or discarded)
	atomic.AddInt64(&urm.allocatedBuffers, -1)
}

// GetOptions gets a ProcessorOptions from the pool
func (urm *UnifiedResourceManager) GetOptions() *ProcessorOptions {
	obj := urm.optionsPool.Get()
	opts, ok := obj.(*ProcessorOptions)
	if !ok {
		// Pool corruption detected: type assertion failed
		// Log this rare event for debugging purposes
		slog.Debug("pool corruption detected: options type assertion failed", "type", fmt.Sprintf("%T", obj))
		// Fallback: create new options if type assertion fails
		opts = DefaultOptionsClone()
	}
	// Reset to default values
	*opts = ProcessorOptions{
		CacheResults:    true,
		StrictMode:      false,
		MaxDepth:        50,
		AllowComments:   false,
		PreserveNumbers: false,
		CreatePaths:     false,
		CleanupNulls:    false,
		CompactArrays:   false,
		ContinueOnError: false,
	}
	atomic.AddInt64(&urm.allocatedOptions, 1)
	return opts
}

// PutOptions returns a ProcessorOptions to the pool
func (urm *UnifiedResourceManager) PutOptions(opts *ProcessorOptions) {
	if opts != nil {
		defer atomic.AddInt64(&urm.allocatedOptions, -1)
		// Clear context to prevent memory leaks
		opts.Context = nil
		urm.optionsPool.Put(opts)
	}
}

// PerformMaintenance performs periodic cleanup
// sync.Pool automatically handles cleanup via GC
func (urm *UnifiedResourceManager) PerformMaintenance() {
	now := time.Now().Unix()
	lastCleanup := atomic.LoadInt64(&urm.lastCleanup)

	if now-lastCleanup < urm.cleanupInterval {
		return
	}

	if !atomic.CompareAndSwapInt64(&urm.lastCleanup, lastCleanup, now) {
		return
	}

	atomic.StoreInt64(&urm.lastCleanup, now)
}

func (urm *UnifiedResourceManager) GetStats() ResourceManagerStats {
	return ResourceManagerStats{
		AllocatedBuilders: atomic.LoadInt64(&urm.allocatedBuilders),
		AllocatedSegments: atomic.LoadInt64(&urm.allocatedSegments),
		AllocatedBuffers:  atomic.LoadInt64(&urm.allocatedBuffers),
		AllocatedOptions:  atomic.LoadInt64(&urm.allocatedOptions),
		LastCleanup:       atomic.LoadInt64(&urm.lastCleanup),
	}
}

type ResourceManagerStats struct {
	AllocatedBuilders int64 `json:"allocated_builders"`
	AllocatedSegments int64 `json:"allocated_segments"`
	AllocatedBuffers  int64 `json:"allocated_buffers"`
	AllocatedOptions  int64 `json:"allocated_options"`
	LastCleanup       int64 `json:"last_cleanup"`
}

var (
	globalResourceManager     *UnifiedResourceManager
	globalResourceManagerOnce sync.Once
)

func getGlobalResourceManager() *UnifiedResourceManager {
	globalResourceManagerOnce.Do(func() {
		globalResourceManager = NewUnifiedResourceManager()
	})
	return globalResourceManager
}
