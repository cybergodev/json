package json

import (
	"sync/atomic"
	"time"

	"github.com/cybergodev/json/internal"
)

func calculateSuccessRateInternal(successful, total int64) float64 {
	if total == 0 {
		return 0.0
	}
	return float64(successful) / float64(total) * 100.0
}

func calculateHitRatioInternal(hits, misses int64) float64 {
	total := hits + misses
	if total == 0 {
		return 0.0
	}
	return float64(hits) / float64(total) * 100.0
}

// GetStats returns processor performance statistics
func (p *Processor) GetStats() Stats {
	cacheStats := p.cache.GetStats()

	return Stats{
		CacheSize:        cacheStats.Entries,
		CacheMemory:      cacheStats.TotalMemory,
		MaxCacheSize:     p.config.MaxCacheSize,
		HitCount:         cacheStats.HitCount,
		MissCount:        cacheStats.MissCount,
		HitRatio:         cacheStats.HitRatio,
		CacheTTL:         p.config.CacheTTL,
		CacheEnabled:     p.config.EnableCache,
		IsClosed:         p.IsClosed(),
		MemoryEfficiency: cacheStats.MemoryEfficiency,
		OperationCount:   atomic.LoadInt64(&p.metrics.operationCount),
		ErrorCount:       atomic.LoadInt64(&p.metrics.errorCount),
	}
}

// getDetailedStats returns detailed performance statistics
func (p *Processor) getDetailedStats() DetailedStats {
	stats := p.GetStats()
	resourceStats := getGlobalResourceManager().GetStats()

	return DetailedStats{
		Stats:          stats,
		state:          atomic.LoadInt32(&p.state),
		configSnapshot: *p.config,
		resourcePoolStats: ResourcePoolStats{
			StringBuilderPoolActive: resourceStats.AllocatedBuilders > 0,
			PathSegmentPoolActive:   resourceStats.AllocatedSegments > 0,
		},
	}
}

// getMetrics returns comprehensive processor metrics for internal use
func (p *Processor) getMetrics() ProcessorMetrics {
	if p.metrics == nil {
		// Return empty metrics if collector is not initialized
		return ProcessorMetrics{}
	}
	internalMetrics := p.metrics.collector.GetMetrics()
	// Convert internal.Metrics to ProcessorMetrics
	return ProcessorMetrics{
		TotalOperations:       internalMetrics.TotalOperations,
		SuccessfulOperations:  internalMetrics.SuccessfulOps,
		FailedOperations:      internalMetrics.FailedOps,
		SuccessRate:           calculateSuccessRateInternal(internalMetrics.SuccessfulOps, internalMetrics.TotalOperations),
		CacheHits:             internalMetrics.CacheHits,
		CacheMisses:           internalMetrics.CacheMisses,
		CacheHitRate:          calculateHitRatioInternal(internalMetrics.CacheHits, internalMetrics.CacheMisses),
		AverageProcessingTime: internalMetrics.AvgProcessingTime,
		MaxProcessingTime:     internalMetrics.MaxProcessingTime,
		MinProcessingTime:     internalMetrics.MinProcessingTime,
		TotalMemoryAllocated:  internalMetrics.TotalMemoryAllocated,
		PeakMemoryUsage:       internalMetrics.PeakMemoryUsage,
		CurrentMemoryUsage:    internalMetrics.CurrentMemoryUsage,
		ActiveConcurrentOps:   internalMetrics.ActiveConcurrentOps,
		MaxConcurrentOps:      internalMetrics.MaxConcurrentOps,
		runtimeMemStats:       internalMetrics.RuntimeMemStats,
		uptime:                internalMetrics.Uptime,
		errorsByType:          internalMetrics.ErrorsByType,
	}
}

// GetHealthStatus returns the current health status
func (p *Processor) GetHealthStatus() HealthStatus {
	if p.metrics == nil {
		return HealthStatus{
			Timestamp: time.Now(),
			Healthy:   false,
			Checks: map[string]CheckResult{
				"metrics": {
					Healthy: false,
					Message: "Metrics collector not initialized",
				},
			},
		}
	}

	healthChecker := internal.NewHealthChecker(p.metrics.collector, nil)
	internalStatus := healthChecker.CheckHealth()

	// Convert internal.HealthStatus to HealthStatus
	checks := make(map[string]CheckResult)
	for name, result := range internalStatus.Checks {
		checks[name] = CheckResult{
			Healthy: result.Healthy,
			Message: result.Message,
		}
	}

	return HealthStatus{
		Timestamp: internalStatus.Timestamp,
		Healthy:   internalStatus.Healthy,
		Checks:    checks,
	}
}
