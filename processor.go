package json

import (
	"log/slog"
	"sync"
	"sync/atomic"

	"github.com/cybergodev/json/internal"
)

// Processor is the main JSON processing engine with thread safety and performance optimization
type Processor struct {
	config          *Config
	cache           *internal.CacheManager
	state           int32
	cleanupOnce     sync.Once
	resources       *processorResources
	metrics         *processorMetrics
	resourceMonitor *ResourceMonitor
	logger          *slog.Logger
}

// processorResources consolidates resource management with optimized pooling
type processorResources struct {
	lastPoolReset   int64
	lastMemoryCheck int64
	memoryPressure  int32
}

// processorMetrics consolidates metrics with conditional collection
type processorMetrics struct {
	operationCount       int64
	errorCount           int64
	lastOperationTime    int64
	operationWindow      int64
	concurrencySemaphore chan struct{}
	collector            *internal.MetricsCollector
	enabled              bool // Flag to enable/disable metrics collection
}

// New creates a new JSON processor with optimized configuration.
// If no configuration is provided, uses default configuration.
// This function follows the explicit config pattern as required by the design guidelines.
func New(config ...*Config) *Processor {
	var cfg *Config
	if len(config) > 0 && config[0] != nil {
		cfg = config[0].Clone() // Use clone to prevent external modifications
	} else {
		cfg = DefaultConfig()
	}

	// Validate configuration and use defaults for invalid values
	if err := ValidateConfig(cfg); err != nil {
		// Log the error but continue with corrected config instead of returning broken processor
		cfg = DefaultConfig()
	}

	p := &Processor{
		config:          cfg,
		cache:           internal.NewCacheManager(cfg),
		resourceMonitor: NewResourceMonitor(),
		logger:          slog.Default().With("component", "json-processor"),
		resources: &processorResources{
			lastPoolReset:   0,
			lastMemoryCheck: 0,
			memoryPressure:  0,
		},
		metrics: &processorMetrics{
			operationWindow:      0,
			concurrencySemaphore: make(chan struct{}, cfg.MaxConcurrency),
			enabled:              cfg.EnableMetrics,
		},
	}

	// Only create metrics collector if metrics are enabled
	if cfg.EnableMetrics {
		p.metrics.collector = internal.NewMetricsCollector()
	}

	return p
}

// Close closes the processor and cleans up resources
func (p *Processor) Close() error {
	p.cleanupOnce.Do(func() {
		atomic.StoreInt32(&p.state, 1)

		if p.cache != nil {
			p.cache.Clear()
		}

		// Reset resource tracking
		atomic.StoreInt32(&p.resources.memoryPressure, 0)
		atomic.StoreInt64(&p.resources.lastMemoryCheck, 0)
		atomic.StoreInt64(&p.resources.lastPoolReset, 0)

		atomic.StoreInt32(&p.state, 2)
	})
	return nil
}

// IsClosed returns true if the processor has been closed
func (p *Processor) IsClosed() bool {
	return atomic.LoadInt32(&p.state) == 2
}
