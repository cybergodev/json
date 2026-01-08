package internal

import (
	"fmt"
	"runtime"
	"strings"
	"time"
)

const (
	defaultMaxMemoryBytes      = 1024 * 1024 * 1024 // 1GB
	defaultMaxErrorRatePercent = 10.0                // 10%
)

// HealthChecker provides health checking functionality for the JSON processor
type HealthChecker struct {
	metrics             *MetricsCollector
	maxMemoryBytes      uint64
	maxErrorRatePercent float64
}

// HealthCheckerConfig holds configuration for health checker
type HealthCheckerConfig struct {
	MaxMemoryBytes      uint64
	MaxErrorRatePercent float64
}

// NewHealthChecker creates a new health checker with optional custom thresholds
func NewHealthChecker(metrics *MetricsCollector, config *HealthCheckerConfig) *HealthChecker {
	maxMemory := uint64(defaultMaxMemoryBytes)
	maxErrorRate := defaultMaxErrorRatePercent

	if config != nil {
		if config.MaxMemoryBytes > 0 {
			maxMemory = config.MaxMemoryBytes
		}
		if config.MaxErrorRatePercent > 0 && config.MaxErrorRatePercent <= 100 {
			maxErrorRate = config.MaxErrorRatePercent
		}
	}

	return &HealthChecker{
		metrics:             metrics,
		maxMemoryBytes:      maxMemory,
		maxErrorRatePercent: maxErrorRate,
	}
}

// CheckHealth performs health checks and returns overall status
func (hc *HealthChecker) CheckHealth() HealthStatus {
	checks := map[string]CheckResult{
		"metrics":    hc.checkMetrics(),
		"memory":     hc.checkMemoryUsage(),
		"error_rate": hc.checkErrorRates(),
	}

	// Calculate overall health
	overall := true
	for _, result := range checks {
		if !result.Healthy {
			overall = false
			break
		}
	}

	return HealthStatus{
		Timestamp: time.Now(),
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

	const mbDivisor = 1024 * 1024
	allocMB := float64(memStats.Alloc) / mbDivisor
	limitMB := float64(hc.maxMemoryBytes) / mbDivisor

	if memStats.Alloc > hc.maxMemoryBytes {
		return CheckResult{
			Healthy: false,
			Message: fmt.Sprintf("High memory usage: %.2f MB (limit: %.2f MB)", allocMB, limitMB),
		}
	}

	return CheckResult{
		Healthy: true,
		Message: fmt.Sprintf("Memory: %.2f MB (%.1f%% of limit)", allocMB, (allocMB/limitMB)*100),
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

	if errorRate > hc.maxErrorRatePercent {
		return CheckResult{
			Healthy: false,
			Message: fmt.Sprintf("High error rate: %.2f%% (limit: %.1f%%)", errorRate, hc.maxErrorRatePercent),
		}
	}

	return CheckResult{
		Healthy: true,
		Message: fmt.Sprintf("Error rate healthy: %.2f%% (%d/%d)", errorRate, failed, total),
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

	// Pre-calculate size for better performance
	estimatedSize := 100 + len(hs.Checks)*80
	var b strings.Builder
	b.Grow(estimatedSize)

	b.WriteString("Health Status: ")
	b.WriteString(status)
	b.WriteString(" (checked at ")
	b.WriteString(hs.Timestamp.Format(time.RFC3339))
	b.WriteString(")\n")

	for checkName, result := range hs.Checks {
		if result.Healthy {
			b.WriteString("  ✓ ")
		} else {
			b.WriteString("  ✗ ")
		}
		b.WriteString(checkName)
		b.WriteString(": ")
		b.WriteString(result.Message)
		b.WriteByte('\n')
	}

	return b.String()
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
