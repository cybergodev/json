package internal

import (
	"fmt"
	"runtime"
	"strings"
	"time"
)

// HealthChecker provides health checking functionality for the JSON processor
type HealthChecker struct {
	processor any // Reference to processor (using any to avoid circular dependency)
	metrics   *MetricsCollector
}

// NewHealthChecker creates a new health checker
func NewHealthChecker(processor any, metrics *MetricsCollector) *HealthChecker {
	return &HealthChecker{
		processor: processor,
		metrics:   metrics,
	}
}

// CheckHealth performs comprehensive health checks
func (hc *HealthChecker) CheckHealth() HealthStatus {
	timestamp := time.Now()
	checks := make(map[string]CheckResult)
	overall := true

	// Check metrics availability
	metricsResult := hc.checkMetrics()
	checks["metrics"] = metricsResult
	if !metricsResult.Healthy {
		overall = false
	}

	// Check memory usage
	memoryResult := hc.checkMemoryUsage()
	checks["memory"] = memoryResult
	if !memoryResult.Healthy {
		overall = false
	}

	// Check error rates
	errorResult := hc.checkErrorRates()
	checks["error_rate"] = errorResult
	if !errorResult.Healthy {
		overall = false
	}

	// Check performance
	performanceResult := hc.checkPerformance()
	checks["performance"] = performanceResult
	if !performanceResult.Healthy {
		overall = false
	}

	// Check concurrency
	concurrencyResult := hc.checkConcurrency()
	checks["concurrency"] = concurrencyResult
	if !concurrencyResult.Healthy {
		overall = false
	}

	// Check cache health
	cacheResult := hc.checkCacheHealth()
	checks["cache"] = cacheResult
	if !cacheResult.Healthy {
		overall = false
	}

	return HealthStatus{
		Timestamp: timestamp,
		Healthy:   overall,
		Checks:    checks,
	}
}

// checkMetrics verifies metrics collection is working
func (hc *HealthChecker) checkMetrics() CheckResult {
	if hc.metrics == nil {
		return CheckResult{
			Healthy: false,
			Message: "Metrics collector not initialized",
		}
	}

	metrics := hc.metrics.GetMetrics()
	if metrics.TotalOperations < 0 {
		return CheckResult{
			Healthy: false,
			Message: "Invalid metrics data detected",
		}
	}

	return CheckResult{
		Healthy: true,
		Message: fmt.Sprintf("Metrics healthy: %d operations processed", metrics.TotalOperations),
	}
}

// checkMemoryUsage checks if memory usage is within acceptable limits
func (hc *HealthChecker) checkMemoryUsage() CheckResult {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	// Check if memory usage is excessive (>1GB)
	const maxMemoryBytes = 1024 * 1024 * 1024 // 1GB
	if memStats.Alloc > maxMemoryBytes {
		return CheckResult{
			Healthy: false,
			Message: fmt.Sprintf("High memory usage: %d bytes (>%d)", memStats.Alloc, maxMemoryBytes),
		}
	}

	// Check for memory leaks (growing heap)
	if memStats.HeapInuse > memStats.HeapAlloc*2 {
		return CheckResult{
			Healthy: false,
			Message: fmt.Sprintf("Potential memory leak detected: heap in use %d vs allocated %d",
				memStats.HeapInuse, memStats.HeapAlloc),
		}
	}

	return CheckResult{
		Healthy: true,
		Message: fmt.Sprintf("Memory usage healthy: %d bytes allocated", memStats.Alloc),
	}
}

// checkErrorRates checks if error rates are within acceptable limits
func (hc *HealthChecker) checkErrorRates() CheckResult {
	if hc.metrics == nil {
		return CheckResult{
			Healthy: false,
			Message: "Cannot check error rates: metrics not available",
		}
	}

	metrics := hc.metrics.GetMetrics()
	total := metrics.TotalOperations
	failed := metrics.FailedOps

	if total == 0 {
		return CheckResult{
			Healthy: true,
			Message: "No operations processed yet",
		}
	}

	errorRate := float64(failed) / float64(total) * 100.0
	const maxErrorRate = 10.0 // 10% max error rate

	if errorRate > maxErrorRate {
		return CheckResult{
			Healthy: false,
			Message: fmt.Sprintf("High error rate: %.2f%% (>%.1f%%)", errorRate, maxErrorRate),
		}
	}

	return CheckResult{
		Healthy: true,
		Message: fmt.Sprintf("Error rate healthy: %.2f%% (%d/%d)", errorRate, failed, total),
	}
}

// checkPerformance checks if performance metrics are within acceptable ranges
func (hc *HealthChecker) checkPerformance() CheckResult {
	if hc.metrics == nil {
		return CheckResult{
			Healthy: false,
			Message: "Cannot check performance: metrics not available",
		}
	}

	metrics := hc.metrics.GetMetrics()

	// Check average processing time (should be < 100ms for most operations)
	const maxAvgTime = 100 * time.Millisecond
	if metrics.AvgProcessingTime > maxAvgTime {
		return CheckResult{
			Healthy: false,
			Message: fmt.Sprintf("Slow average processing time: %v (>%v)",
				metrics.AvgProcessingTime, maxAvgTime),
		}
	}

	// Check maximum processing time (should be < 1s)
	const maxProcessingTime = 1 * time.Second
	if metrics.MaxProcessingTime > maxProcessingTime {
		return CheckResult{
			Healthy: false,
			Message: fmt.Sprintf("Slow maximum processing time: %v (>%v)",
				metrics.MaxProcessingTime, maxProcessingTime),
		}
	}

	return CheckResult{
		Healthy: true,
		Message: fmt.Sprintf("Performance healthy: avg %v, max %v",
			metrics.AvgProcessingTime, metrics.MaxProcessingTime),
	}
}

// checkConcurrency checks if concurrency levels are reasonable
func (hc *HealthChecker) checkConcurrency() CheckResult {
	if hc.metrics == nil {
		return CheckResult{
			Healthy: false,
			Message: "Cannot check concurrency: metrics not available",
		}
	}

	metrics := hc.metrics.GetMetrics()
	active := metrics.ActiveConcurrentOps
	maxInt := metrics.MaxConcurrentOps

	// Check if too many concurrent operations
	const maxConcurrentOps = 1000
	if active > maxConcurrentOps {
		return CheckResult{
			Healthy: false,
			Message: fmt.Sprintf("Too many concurrent operations: %d (>%d)", active, maxConcurrentOps),
		}
	}

	return CheckResult{
		Healthy: true,
		Message: fmt.Sprintf("Concurrency healthy: %d active, %d max", active, maxInt),
	}
}

// checkCacheHealth checks cache performance and health
func (hc *HealthChecker) checkCacheHealth() CheckResult {
	if hc.metrics == nil {
		return CheckResult{
			Healthy: false,
			Message: "Cannot check cache: metrics not available",
		}
	}

	metrics := hc.metrics.GetMetrics()
	hits := metrics.CacheHits
	misses := metrics.CacheMisses
	total := hits + misses

	if total == 0 {
		return CheckResult{
			Healthy: true,
			Message: "Cache not used yet",
		}
	}

	hitRate := float64(hits) / float64(total) * 100.0
	const minHitRate = 50.0 // 50% minimum hit rate

	if hitRate < minHitRate {
		return CheckResult{
			Healthy: false,
			Message: fmt.Sprintf("Low cache hit rate: %.2f%% (<%.1f%%)", hitRate, minHitRate),
		}
	}

	return CheckResult{
		Healthy: true,
		Message: fmt.Sprintf("Cache healthy: %.2f%% hit rate (%d/%d)", hitRate, hits, total),
	}
}

// HealthStatus represents the health status of the processor
type HealthStatus struct {
	Timestamp time.Time              `json:"timestamp"`
	Healthy   bool                   `json:"healthy"`
	Checks    map[string]CheckResult `json:"checks"`
}

// CheckResult represents the result of a single health check
type CheckResult struct {
	Healthy bool   `json:"healthy"`
	Message string `json:"message"`
}

// GetSummary returns a formatted summary of the health status
func (hs *HealthStatus) GetSummary() string {
	status := "HEALTHY"
	if !hs.Healthy {
		status = "UNHEALTHY"
	}

	var summary strings.Builder
	summary.WriteString(fmt.Sprintf("Health Status: %s (checked at %s)\n", status, hs.Timestamp.Format(time.RFC3339)))

	for checkName, result := range hs.Checks {
		checkStatus := "✓"
		if !result.Healthy {
			checkStatus = "✗"
		}
		summary.WriteString(fmt.Sprintf("  %s %s: %s\n", checkStatus, checkName, result.Message))
	}

	return summary.String()
}

// IsHealthy returns true if all health checks passed
func (hs *HealthStatus) IsHealthy() bool {
	return hs.Healthy
}

// GetFailedChecks returns a list of failed health check names
func (hs *HealthStatus) GetFailedChecks() []string {
	var failed []string
	for name, result := range hs.Checks {
		if !result.Healthy {
			failed = append(failed, name)
		}
	}
	return failed
}
