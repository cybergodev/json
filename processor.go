package json

import (
	"encoding/json"
		"fmt"
	"log/slog"
	"sync"
	"sync/atomic"

	"github.com/cybergodev/json/internal"
)

// Processor state constants for lifecycle management
const (
	processorStateActive        int32 = iota // 0: Processor is active and accepting operations
	processorStateClosing                    // 1: Processor is closing, no new operations
	processorStateClosed                     // 2: Processor is fully closed
	processorStateCloseTimedOut              // 3: Processor close timed out, resources not fully released
)

// Processor is the main JSON processing engine with thread safety and performance optimization
type Processor struct {
	config            Config
	cache             *internal.CacheManager
	state             int32
	cleanupOnce       sync.Once
	resources         *processorResources
	metrics           *processorMetrics
	resourceMonitor   *resourceMonitor
	logger            atomic.Value // *slog.Logger - thread-safe logger storage
	securityValidator *securityValidator
	// Cached recursiveProcessor for reuse across operations (performance optimization)
	recursiveProcessor *recursiveProcessor
	// Extension points for hooks
	hooks   []Hook
	hooksMu sync.Mutex // protects hooks slice for concurrent AddHook
	// Cached processor ID to avoid fmt.Sprintf on every log call
	processorID string
}

type processorResources struct {
	lastPoolReset   int64
	lastMemoryCheck int64
	memoryPressure  int32
}

type processorMetrics struct {
	operationCount       int64
	errorCount           int64
	lastOperationTime    int64
	operationWindow      int64
	concurrencySemaphore chan struct{}
	collector            *internal.MetricsCollector
	enabled              bool // Flag to enable/disable metrics collection
}

// New creates a new JSON processor with the given configuration.
// If no configuration is provided, uses default configuration.
//
// Returns an error if the configuration is invalid (see Config.Validate).
// Always call Close() when done to release resources.
//
// Example:
//
//	// Using default configuration
//	processor, err := json.New()
//	if err != nil {
//	    // Handle configuration error
//	}
//	defer processor.Close()
//
//	// With custom configuration
//	cfg := json.DefaultConfig()
//	cfg.CreatePaths = true
//	cfg.EnableCache = true
//	processor, err := json.New(cfg)
//
//	// Using preset configuration
//	processor, err := json.New(json.SecurityConfig())
func New(cfg ...Config) (*Processor, error) {
	var config Config
	if len(cfg) > 0 {
		config = cfg[0]
	} else {
		config = DefaultConfig()
	}

	// Validate configuration and apply corrections for invalid values
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	p := &Processor{
		config:          config,
		cache:           internal.NewCacheManager(config.EnableCache, config.MaxCacheSize, config.CacheTTL),
		resourceMonitor: newResourceMonitor(),
		securityValidator: newSecurityValidator(
			config.MaxJSONSize,
			maxPathLength,
			config.MaxNestingDepthSecurity,
			config.FullSecurityScan,
			config.DisableDefaultPatterns,
		),
		resources: &processorResources{
			lastPoolReset:   0,
			lastMemoryCheck: 0,
			memoryPressure:  0,
		},
		metrics: &processorMetrics{
			operationWindow:      0, // Disabled by default for better performance
			concurrencySemaphore: make(chan struct{}, config.MaxConcurrency),
			enabled:              config.EnableMetrics,
		},
	}

	// Initialize logger atomically for thread safety
	p.logger.Store(slog.Default().With("component", "json-processor"))

	// Only create metrics collector if metrics are enabled
	if config.EnableMetrics {
		p.metrics.collector = internal.NewMetricsCollector()
	}

	// Initialize cached recursiveProcessor for reuse
	p.recursiveProcessor = newRecursiveProcessor(p)

	// Cache processor ID to avoid fmt.Sprintf per log call
	p.processorID = fmt.Sprintf("proc_%p", p)

	return p, nil
}

// configPool pools Config objects to reduce allocations in hot paths
// PERFORMANCE: Reduces ~6GB allocations from prepareOptions calls
var configPool = sync.Pool{
	New: func() any {
		return new(Config)
	},
}

// releaseConfig returns a pooled Config, clearing all reference-type fields first
// to prevent data leaks back into the pool.
// SECURITY: Must clear all map/slice/interface fields to avoid cross-request contamination.
// PERFORMANCE v3: Only nil out reference-type fields instead of full DefaultConfig() copy.
func releaseConfig(cfg *Config) {
	if cfg == nil {
		return
	}
	// Clear only reference-type fields to prevent data leaks.
	// Value fields (bool, int, string, time.Duration) don't need clearing.
	cfg.CustomEscapes = nil
	cfg.CustomEncoder = nil
	cfg.CustomTypeEncoders = nil
	cfg.CustomValidators = nil
	cfg.AdditionalDangerousPatterns = nil
	cfg.Hooks = nil
	cfg.CustomPathParser = nil
	cfg.Indent = ""
	cfg.Prefix = ""
	configPool.Put(cfg)
}

// prepareOptions prepares and validates processor options.
// Accepts Config values and returns a pointer for internal use.
// PERFORMANCE: When no options are provided, returns a cached default pointer
// without allocation or validation — avoids DefaultConfig() + Validate() per operation.
// SECURITY: Clears reference fields from pooled objects to prevent leaks.
func (p *Processor) prepareOptions(cfg ...Config) (*Config, error) {
	c := configPool.Get().(*Config)
	if len(cfg) == 0 {
		*c = cachedDefaultConfigValue
		return c, nil
	}
	*c = cfg[0]
	if err := c.Validate(); err != nil {
		releaseConfig(c)
		return nil, err
	}
	return c, nil
}

// mergeOptionsWithOverride creates a new Config with overrides applied.
// Returns a Config value (not pointer) to prevent accidental mutation
// and encourage the caller to work with their own copy.
func mergeOptionsWithOverride(opts []Config, override func(*Config)) Config {
	var result Config
	if len(opts) > 0 {
		result = *(&opts[0]).Clone()
	} else {
		result = DefaultConfig()
	}
	override(&result)
	return result
}

// acquireSemaphore reserves a concurrency slot. Returns nil on success.
// Returns ErrConcurrencyLimit when MaxConcurrency concurrent operations are running.
func (p *Processor) acquireSemaphore() error {
	if p == nil {
		return nil
	}
	if p.metrics == nil || p.metrics.concurrencySemaphore == nil {
		return nil
	}
	select {
	case p.metrics.concurrencySemaphore <- struct{}{}:
		return nil
	default:
		return &JsonsError{
			Op:      "concurrency",
			Message: fmt.Sprintf("max concurrent operations (%d) reached", cap(p.metrics.concurrencySemaphore)),
			Err:     ErrConcurrencyLimit,
		}
	}
}

// releaseSemaphore releases a previously acquired concurrency slot.
func (p *Processor) releaseSemaphore() {
	if p == nil {
		return
	}
	if p.metrics != nil && p.metrics.concurrencySemaphore != nil {
		<-p.metrics.concurrencySemaphore
	}
}

// prepareOperation validates input and path for Get/Set/Delete operations.
// Returns prepared options and a cleanup function that must be deferred.
// Reduces duplicated validation boilerplate across operation methods.
func (p *Processor) prepareOperation(op, jsonStr, path string, cfg ...Config) (*Config, func(), error) {
	if err := p.checkClosed(); err != nil {
		return nil, nil, err
	}

	options, err := p.prepareOptions(cfg...)
	if err != nil {
		return nil, nil, err
	}
	cleanup := func() { releaseConfig(options) }

	if !options.SkipValidation {
		if err := p.validateInput(jsonStr); err != nil {
			cleanup()
			return nil, nil, err
		}
		if !isSimplePropertyAccess(path) {
			if err := p.validatePath(path); err != nil {
				cleanup()
				return nil, nil, err
			}
		}
	} else {
		if err := p.validateInputEssential(jsonStr); err != nil {
			cleanup()
			return nil, nil, err
		}
	}
	return options, cleanup, nil
}

// validateOperationInput validates JSON input and path for an operation.
// Handles the SkipValidation flag: when true, only essential size/depth checks run.
func (p *Processor) validateOperationInput(jsonStr, path string, options *Config) error {
	if !options.SkipValidation {
		if err := p.validateInput(jsonStr); err != nil {
			return err
		}
		if !isSimplePropertyAccess(path) {
			if err := p.validatePath(path); err != nil {
				return err
			}
		}
	} else {
		if err := p.validateInputEssential(jsonStr); err != nil {
			return err
		}
	}
	return nil
}

// parseJSON parses a JSON string into a Go value using the appropriate path.
// Uses fast json.Unmarshal for default config, falls back to p.Parse for custom options.
func (p *Processor) parseJSON(jsonStr, op, path string, options *Config, cfg ...Config) (any, error) {
	var data any
	if len(cfg) == 0 && !p.config.PreserveNumbers {
		if err := json.Unmarshal(internal.StringToBytes(jsonStr), &data); err != nil {
			return nil, &JsonsError{
				Op:      op,
				Path:    path,
				Message: fmt.Sprintf("failed to parse JSON: %v", err),
				Err:     ErrInvalidJSON,
			}
		}
		return data, nil
	}
	if err := p.Parse(jsonStr, &data, *options); err != nil {
		return nil, err
	}
	return data, nil
}

