package json

import (
	"runtime"
	"sync/atomic"
	"time"
)

// ResourceMonitor provides resource monitoring and leak detection
type ResourceMonitor struct {
	allocatedBytes    int64
	freedBytes        int64
	peakMemoryUsage   int64
	poolHits          int64
	poolMisses        int64
	poolEvictions     int64
	maxGoroutines     int64
	currentGoroutines int64
	lastLeakCheck     int64
	leakCheckInterval int64
	avgResponseTime   int64
	totalOperations   int64
}

// NewResourceMonitor creates a new resource monitor
func NewResourceMonitor() *ResourceMonitor {
	return &ResourceMonitor{
		leakCheckInterval: 300, // 5 minutes
		lastLeakCheck:     time.Now().Unix(),
	}
}

// RecordAllocation records an allocation of the specified size
func (rm *ResourceMonitor) RecordAllocation(bytes int64) {
	atomic.AddInt64(&rm.allocatedBytes, bytes)

	current := atomic.LoadInt64(&rm.allocatedBytes) - atomic.LoadInt64(&rm.freedBytes)
	for {
		peak := atomic.LoadInt64(&rm.peakMemoryUsage)
		if current <= peak || atomic.CompareAndSwapInt64(&rm.peakMemoryUsage, peak, current) {
			break
		}
	}
}

// RecordDeallocation records a deallocation of the specified size
func (rm *ResourceMonitor) RecordDeallocation(bytes int64) {
	atomic.AddInt64(&rm.freedBytes, bytes)
}

// RecordPoolHit records a pool cache hit
func (rm *ResourceMonitor) RecordPoolHit() {
	atomic.AddInt64(&rm.poolHits, 1)
}

// RecordPoolMiss records a pool cache miss
func (rm *ResourceMonitor) RecordPoolMiss() {
	atomic.AddInt64(&rm.poolMisses, 1)
}

// RecordPoolEviction records a pool eviction
func (rm *ResourceMonitor) RecordPoolEviction() {
	atomic.AddInt64(&rm.poolEvictions, 1)
}

// RecordOperation records an operation with its duration
func (rm *ResourceMonitor) RecordOperation(duration time.Duration) {
	atomic.AddInt64(&rm.totalOperations, 1)

	newTime := duration.Nanoseconds()
	for {
		oldAvg := atomic.LoadInt64(&rm.avgResponseTime)
		newAvg := oldAvg + (newTime-oldAvg)/10
		if atomic.CompareAndSwapInt64(&rm.avgResponseTime, oldAvg, newAvg) {
			break
		}
	}
}

// CheckForLeaks checks for potential resource leaks
func (rm *ResourceMonitor) CheckForLeaks() []string {
	for {
		now := time.Now().Unix()
		lastCheck := atomic.LoadInt64(&rm.lastLeakCheck)

		if now-lastCheck < rm.leakCheckInterval {
			return nil
		}

		if atomic.CompareAndSwapInt64(&rm.lastLeakCheck, lastCheck, now) {
			break
		}
	}

	var issues []string

	allocated := atomic.LoadInt64(&rm.allocatedBytes)
	freed := atomic.LoadInt64(&rm.freedBytes)
	netMemory := allocated - freed

	if netMemory > 100*1024*1024 {
		issues = append(issues, "High memory usage detected")
	}

	currentGoroutines := int64(runtime.NumGoroutine())
	atomic.StoreInt64(&rm.currentGoroutines, currentGoroutines)

	maxGoroutines := atomic.LoadInt64(&rm.maxGoroutines)
	if currentGoroutines > maxGoroutines {
		atomic.StoreInt64(&rm.maxGoroutines, currentGoroutines)
	}

	if currentGoroutines > 1000 {
		issues = append(issues, "High goroutine count detected")
	}

	hits := atomic.LoadInt64(&rm.poolHits)
	misses := atomic.LoadInt64(&rm.poolMisses)

	if hits+misses > 1000 && hits < misses {
		issues = append(issues, "Poor pool cache efficiency")
	}

	return issues
}

// GetStats returns current resource statistics
func (rm *ResourceMonitor) GetStats() ResourceStats {
	return ResourceStats{
		AllocatedBytes:    atomic.LoadInt64(&rm.allocatedBytes),
		FreedBytes:        atomic.LoadInt64(&rm.freedBytes),
		PeakMemoryUsage:   atomic.LoadInt64(&rm.peakMemoryUsage),
		PoolHits:          atomic.LoadInt64(&rm.poolHits),
		PoolMisses:        atomic.LoadInt64(&rm.poolMisses),
		PoolEvictions:     atomic.LoadInt64(&rm.poolEvictions),
		MaxGoroutines:     atomic.LoadInt64(&rm.maxGoroutines),
		CurrentGoroutines: atomic.LoadInt64(&rm.currentGoroutines),
		AvgResponseTime:   time.Duration(atomic.LoadInt64(&rm.avgResponseTime)),
		TotalOperations:   atomic.LoadInt64(&rm.totalOperations),
	}
}

// ResourceStats represents resource usage statistics
type ResourceStats struct {
	AllocatedBytes    int64         `json:"allocated_bytes"`
	FreedBytes        int64         `json:"freed_bytes"`
	PeakMemoryUsage   int64         `json:"peak_memory_usage"`
	PoolHits          int64         `json:"pool_hits"`
	PoolMisses        int64         `json:"pool_misses"`
	PoolEvictions     int64         `json:"pool_evictions"`
	MaxGoroutines     int64         `json:"max_goroutines"`
	CurrentGoroutines int64         `json:"current_goroutines"`
	AvgResponseTime   time.Duration `json:"avg_response_time"`
	TotalOperations   int64         `json:"total_operations"`
}

// Reset resets all resource statistics
func (rm *ResourceMonitor) Reset() {
	atomic.StoreInt64(&rm.allocatedBytes, 0)
	atomic.StoreInt64(&rm.freedBytes, 0)
	atomic.StoreInt64(&rm.peakMemoryUsage, 0)
	atomic.StoreInt64(&rm.poolHits, 0)
	atomic.StoreInt64(&rm.poolMisses, 0)
	atomic.StoreInt64(&rm.poolEvictions, 0)
	atomic.StoreInt64(&rm.maxGoroutines, 0)
	atomic.StoreInt64(&rm.currentGoroutines, 0)
	atomic.StoreInt64(&rm.avgResponseTime, 0)
	atomic.StoreInt64(&rm.totalOperations, 0)
	atomic.StoreInt64(&rm.lastLeakCheck, time.Now().Unix())
}

// GetMemoryEfficiency returns the memory efficiency percentage
func (rm *ResourceMonitor) GetMemoryEfficiency() float64 {
	allocated := atomic.LoadInt64(&rm.allocatedBytes)
	freed := atomic.LoadInt64(&rm.freedBytes)

	if allocated == 0 {
		return 100.0
	}

	return float64(freed) / float64(allocated) * 100.0
}

// GetPoolEfficiency returns the pool efficiency percentage
func (rm *ResourceMonitor) GetPoolEfficiency() float64 {
	hits := atomic.LoadInt64(&rm.poolHits)
	misses := atomic.LoadInt64(&rm.poolMisses)
	total := hits + misses

	if total == 0 {
		return 100.0
	}

	return float64(hits) / float64(total) * 100.0
}
