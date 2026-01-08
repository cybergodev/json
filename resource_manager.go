package json

import (
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// UnifiedResourceManager consolidates all resource management for optimal performance
type UnifiedResourceManager struct {
	// String builder pool with optimized sizing
	stringBuilderPool *sync.Pool

	// Path segment pool for efficient path parsing
	pathSegmentPool *sync.Pool

	// Buffer pool for various operations
	bufferPool *sync.Pool

	// Resource tracking
	allocatedBuilders int64
	allocatedSegments int64
	allocatedBuffers  int64

	// Cleanup tracking
	lastCleanup     int64
	cleanupInterval int64
}

// NewUnifiedResourceManager creates a new unified resource manager
func NewUnifiedResourceManager() *UnifiedResourceManager {
	return &UnifiedResourceManager{
		stringBuilderPool: &sync.Pool{
			New: func() any {
				sb := &strings.Builder{}
				sb.Grow(512) // Optimized initial size
				return sb
			},
		},
		pathSegmentPool: &sync.Pool{
			New: func() any {
				return make([]PathSegment, 0, 8) // Optimized initial capacity
			},
		},
		bufferPool: &sync.Pool{
			New: func() any {
				return make([]byte, 0, 1024) // Optimized initial capacity
			},
		},
		cleanupInterval: 300, // 5 minutes
		lastCleanup:     time.Now().Unix(),
	}
}

// GetStringBuilder retrieves a string builder from the pool
func (urm *UnifiedResourceManager) GetStringBuilder() *strings.Builder {
	sb := urm.stringBuilderPool.Get().(*strings.Builder)
	sb.Reset()
	atomic.AddInt64(&urm.allocatedBuilders, 1)
	return sb
}

// PutStringBuilder returns a string builder to the pool
func (urm *UnifiedResourceManager) PutStringBuilder(sb *strings.Builder) {
	if sb != nil && sb.Cap() <= 8192 { // Prevent oversized builders from staying in pool
		sb.Reset()
		urm.stringBuilderPool.Put(sb)
		atomic.AddInt64(&urm.allocatedBuilders, -1)
	}
}

// GetPathSegments retrieves a path segments slice from the pool
func (urm *UnifiedResourceManager) GetPathSegments() []PathSegment {
	segments := urm.pathSegmentPool.Get().([]PathSegment)
	segments = segments[:0] // Reset length but keep capacity
	atomic.AddInt64(&urm.allocatedSegments, 1)
	return segments
}

// PutPathSegments returns a path segments slice to the pool
func (urm *UnifiedResourceManager) PutPathSegments(segments []PathSegment) {
	if segments != nil && cap(segments) <= 64 && cap(segments) >= 4 { // Keep reasonable sizes
		segments = segments[:0]
		urm.pathSegmentPool.Put(segments)
		atomic.AddInt64(&urm.allocatedSegments, -1)
	}
}

// GetBuffer retrieves a byte buffer from the pool
func (urm *UnifiedResourceManager) GetBuffer() []byte {
	buf := urm.bufferPool.Get().([]byte)
	buf = buf[:0] // Reset length but keep capacity
	atomic.AddInt64(&urm.allocatedBuffers, 1)
	return buf
}

// PutBuffer returns a byte buffer to the pool
func (urm *UnifiedResourceManager) PutBuffer(buf []byte) {
	if buf != nil && cap(buf) <= 16384 && cap(buf) >= 512 { // Keep reasonable sizes
		buf = buf[:0]
		urm.bufferPool.Put(buf)
		atomic.AddInt64(&urm.allocatedBuffers, -1)
	}
}

// PerformMaintenance performs periodic cleanup and optimization
func (urm *UnifiedResourceManager) PerformMaintenance() {
	now := time.Now().Unix()
	lastCleanup := atomic.LoadInt64(&urm.lastCleanup)

	// Fast path: skip if cleanup interval not reached
	if now-lastCleanup < urm.cleanupInterval {
		return
	}

	// Try to acquire cleanup lock
	if !atomic.CompareAndSwapInt64(&urm.lastCleanup, lastCleanup, now) {
		return // Another goroutine is performing cleanup
	}

	// Reset pools if they exceed thresholds
	urm.resetPoolIfNeeded(&urm.allocatedBuilders, 50, func() {
		urm.stringBuilderPool = &sync.Pool{
			New: func() any {
				sb := &strings.Builder{}
				sb.Grow(512)
				return sb
			},
		}
	})

	urm.resetPoolIfNeeded(&urm.allocatedSegments, 30, func() {
		urm.pathSegmentPool = &sync.Pool{
			New: func() any {
				return make([]PathSegment, 0, 8)
			},
		}
	})

	urm.resetPoolIfNeeded(&urm.allocatedBuffers, 30, func() {
		urm.bufferPool = &sync.Pool{
			New: func() any {
				return make([]byte, 0, 1024)
			},
		}
	})
}

// resetPoolIfNeeded resets a pool if the counter exceeds the threshold
func (urm *UnifiedResourceManager) resetPoolIfNeeded(counter *int64, threshold int64, resetFunc func()) {
	if atomic.LoadInt64(counter) > threshold {
		resetFunc()
		atomic.StoreInt64(counter, 0)
	}
}

// GetStats returns resource usage statistics
func (urm *UnifiedResourceManager) GetStats() ResourceManagerStats {
	return ResourceManagerStats{
		AllocatedBuilders: atomic.LoadInt64(&urm.allocatedBuilders),
		AllocatedSegments: atomic.LoadInt64(&urm.allocatedSegments),
		AllocatedBuffers:  atomic.LoadInt64(&urm.allocatedBuffers),
		LastCleanup:       atomic.LoadInt64(&urm.lastCleanup),
	}
}

// ResourceManagerStats provides statistics about resource usage
type ResourceManagerStats struct {
	AllocatedBuilders int64 `json:"allocated_builders"`
	AllocatedSegments int64 `json:"allocated_segments"`
	AllocatedBuffers  int64 `json:"allocated_buffers"`
	LastCleanup       int64 `json:"last_cleanup"`
}

// Global unified resource manager instance
var globalResourceManager = NewUnifiedResourceManager()

// GetGlobalResourceManager returns the global resource manager instance
func GetGlobalResourceManager() *UnifiedResourceManager {
	return globalResourceManager
}
