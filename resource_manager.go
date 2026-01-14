package json

import (
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

	allocatedBuilders int64
	allocatedSegments int64
	allocatedBuffers  int64

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
		cleanupInterval: 300,
		lastCleanup:     time.Now().Unix(),
	}
}

func (urm *UnifiedResourceManager) GetStringBuilder() *strings.Builder {
	sb := urm.stringBuilderPool.Get().(*strings.Builder)
	sb.Reset()
	atomic.AddInt64(&urm.allocatedBuilders, 1)
	return sb
}

func (urm *UnifiedResourceManager) PutStringBuilder(sb *strings.Builder) {
	if sb != nil && sb.Cap() <= 8192 {
		sb.Reset()
		urm.stringBuilderPool.Put(sb)
		atomic.AddInt64(&urm.allocatedBuilders, -1)
	}
}

func (urm *UnifiedResourceManager) GetPathSegments() []internal.PathSegment {
	segments := urm.pathSegmentPool.Get().([]internal.PathSegment)
	segments = segments[:0]
	atomic.AddInt64(&urm.allocatedSegments, 1)
	return segments
}

func (urm *UnifiedResourceManager) PutPathSegments(segments []internal.PathSegment) {
	if segments != nil && cap(segments) <= 64 && cap(segments) >= 4 {
		segments = segments[:0]
		urm.pathSegmentPool.Put(segments)
		atomic.AddInt64(&urm.allocatedSegments, -1)
	}
}

func (urm *UnifiedResourceManager) GetBuffer() []byte {
	buf := urm.bufferPool.Get().([]byte)
	buf = buf[:0]
	atomic.AddInt64(&urm.allocatedBuffers, 1)
	return buf
}

func (urm *UnifiedResourceManager) PutBuffer(buf []byte) {
	if buf != nil && cap(buf) <= 16384 && cap(buf) >= 512 {
		buf = buf[:0]
		urm.bufferPool.Put(buf)
		atomic.AddInt64(&urm.allocatedBuffers, -1)
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
		LastCleanup:       atomic.LoadInt64(&urm.lastCleanup),
	}
}

type ResourceManagerStats struct {
	AllocatedBuilders int64 `json:"allocated_builders"`
	AllocatedSegments int64 `json:"allocated_segments"`
	AllocatedBuffers  int64 `json:"allocated_buffers"`
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
