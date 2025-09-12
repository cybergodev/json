package json

import (
	"runtime"
	"sync/atomic"
	"time"
)

// ResourceMonitor provides enhanced resource monitoring and leak detection
type ResourceMonitor struct {
	// Memory statistics
	allocatedBytes   int64 // Total allocated bytes
	freedBytes       int64 // Total freed bytes
	peakMemoryUsage  int64 // Peak memory usage
	
	// Pool statistics
	poolHits         int64 // Pool cache hits
	poolMisses       int64 // Pool cache misses
	poolEvictions    int64 // Pool evictions
	
	// Goroutine tracking
	maxGoroutines    int64 // Maximum goroutines seen
	currentGoroutines int64 // Current goroutine count
	
	// Leak detection
	lastLeakCheck    int64 // Last leak detection check
	leakCheckInterval int64 // Interval between leak checks (seconds)
	
	// Performance metrics
	avgResponseTime  int64 // Average response time in nanoseconds
	totalOperations  int64 // Total operations processed
}

// NewResourceMonitor creates a new resource monitor
func NewResourceMonitor() *ResourceMonitor {
	return &ResourceMonitor{
		leakCheckInterval: 300, // 5 minutes
		lastLeakCheck:     time.Now().Unix(),
	}
}

// RecordAllocation records memory allocation
func (rm *ResourceMonitor) RecordAllocation(bytes int64) {
	atomic.AddInt64(&rm.allocatedBytes, bytes)
	
	// Update peak memory usage
	current := atomic.LoadInt64(&rm.allocatedBytes) - atomic.LoadInt64(&rm.freedBytes)
	for {
		peak := atomic.LoadInt64(&rm.peakMemoryUsage)
		if current <= peak || atomic.CompareAndSwapInt64(&rm.peakMemoryUsage, peak, current) {
			break
		}
	}
}

// RecordDeallocation records memory deallocation
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

// RecordOperation records an operation with timing
func (rm *ResourceMonitor) RecordOperation(duration time.Duration) {
	atomic.AddInt64(&rm.totalOperations, 1)
	
	// Update average response time using exponential moving average
	newTime := duration.Nanoseconds()
	for {
		oldAvg := atomic.LoadInt64(&rm.avgResponseTime)
		// Simple exponential moving average with alpha = 0.1
		newAvg := oldAvg + (newTime-oldAvg)/10
		if atomic.CompareAndSwapInt64(&rm.avgResponseTime, oldAvg, newAvg) {
			break
		}
	}
}

// CheckForLeaks performs leak detection and returns potential issues
func (rm *ResourceMonitor) CheckForLeaks() []string {
	now := time.Now().Unix()
	lastCheck := atomic.LoadInt64(&rm.lastLeakCheck)
	
	if now-lastCheck < rm.leakCheckInterval {
		return nil // Too soon for another check
	}
	
	if !atomic.CompareAndSwapInt64(&rm.lastLeakCheck, lastCheck, now) {
		return nil // Another goroutine is checking
	}
	
	var issues []string
	
	// Check memory growth
	allocated := atomic.LoadInt64(&rm.allocatedBytes)
	freed := atomic.LoadInt64(&rm.freedBytes)
	netMemory := allocated - freed
	
	if netMemory > 100*1024*1024 { // 100MB threshold
		issues = append(issues, "High memory usage detected")
	}
	
	// Check goroutine count
	currentGoroutines := int64(runtime.NumGoroutine())
	atomic.StoreInt64(&rm.currentGoroutines, currentGoroutines)
	
	maxGoroutines := atomic.LoadInt64(&rm.maxGoroutines)
	if currentGoroutines > maxGoroutines {
		atomic.StoreInt64(&rm.maxGoroutines, currentGoroutines)
	}
	
	if currentGoroutines > 1000 { // High goroutine count
		issues = append(issues, "High goroutine count detected")
	}
	
	// Check pool efficiency
	hits := atomic.LoadInt64(&rm.poolHits)
	misses := atomic.LoadInt64(&rm.poolMisses)
	
	if hits+misses > 1000 && hits < misses { // Poor pool hit rate
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
	AllocatedBytes    int64         // Total allocated bytes
	FreedBytes        int64         // Total freed bytes
	PeakMemoryUsage   int64         // Peak memory usage
	PoolHits          int64         // Pool cache hits
	PoolMisses        int64         // Pool cache misses
	PoolEvictions     int64         // Pool evictions
	MaxGoroutines     int64         // Maximum goroutines seen
	CurrentGoroutines int64         // Current goroutine count
	AvgResponseTime   time.Duration // Average response time
	TotalOperations   int64         // Total operations processed
}

// Reset resets all statistics
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

// GetMemoryEfficiency returns memory efficiency as a percentage (0-100)
func (rm *ResourceMonitor) GetMemoryEfficiency() float64 {
	allocated := atomic.LoadInt64(&rm.allocatedBytes)
	freed := atomic.LoadInt64(&rm.freedBytes)
	
	if allocated == 0 {
		return 100.0
	}
	
	return float64(freed) / float64(allocated) * 100.0
}

// GetPoolEfficiency returns pool efficiency as a percentage (0-100)
func (rm *ResourceMonitor) GetPoolEfficiency() float64 {
	hits := atomic.LoadInt64(&rm.poolHits)
	misses := atomic.LoadInt64(&rm.poolMisses)
	total := hits + misses
	
	if total == 0 {
		return 100.0
	}
	
	return float64(hits) / float64(total) * 100.0
}
