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
	activeOps         int64 // atomic: in-flight op count, lets Close() drain gracefully
	cleanupOnce       sync.Once
	resources         *processorResources
	metrics           *processorMetrics
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
	operationCount    int64
	errorCount        int64
	lastOperationTime int64
	operationWindow   int64
	// concurrencyLimit/concurrentOps form an atomic counting semaphore that
	// caps in-flight operations at MaxConcurrency. Replacing the previous
	// buffered-channel semaphore removes the channel's internal mutex from the
	// hot path — under high parallelism that lock dominated CPU. This is a SOFT
	// limit: acquire rejects when full (never blocks), matching the channel's
	// non-blocking select semantics.
	concurrencyLimit int64 // immutable after construction; 0 = unlimited
	concurrentOps    int64 // atomic: current in-flight (semaphore-acquired) ops
	collector        *internal.MetricsCollector
	enabled          bool // Flag to enable/disable metrics collection
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
			operationWindow:   0, // Disabled by default for better performance
			concurrencyLimit:  int64(config.MaxConcurrency),
			enabled:           config.EnableMetrics,
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

// defaultConfigSingleton is the shared, immutable default Config handed out by
// prepareOptions when no options are supplied. Returning this single address lets
// callers detect the default config with a pointer comparison instead of a 40+
// field scan (see createCacheKeyWithHash). It MUST stay immutable: releaseConfig
// skips it, and no operation-path code mutates the returned options.
var defaultConfigSingleton = cachedDefaultConfigValue

// releaseConfig returns a pooled Config, clearing all reference-type fields first
// to prevent data leaks back into the pool.
// SECURITY: Must clear all map/slice/interface fields to avoid cross-request contamination.
// PERFORMANCE v3: Only nil out reference-type fields instead of full DefaultConfig() copy.
func releaseConfig(cfg *Config) {
	if cfg == nil || cfg == &defaultConfigSingleton {
		// nil, or the shared immutable default — never pool/mutate the singleton.
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
// PERFORMANCE: When no options are provided, returns the shared, immutable
// default-config singleton — no allocation, no validation, and a pointer that
// downstream hot paths can test against &defaultConfigSingleton in O(1).
// SECURITY: Clears reference fields from pooled objects to prevent leaks.
func (p *Processor) prepareOptions(cfg ...Config) (*Config, error) {
	if len(cfg) == 0 {
		return &defaultConfigSingleton, nil
	}
	c := configPool.Get().(*Config)
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
//
// PERFORMANCE: implemented as an atomic counter rather than a buffered channel.
// The channel's internal lock contended heavily under parallel load; atomics
// avoid it. This is a non-blocking (soft) limit — identical semantics to the
// previous select-with-default: reject immediately when at capacity.
func (p *Processor) acquireSemaphore() error {
	if p == nil || p.metrics == nil {
		return nil
	}
	m := p.metrics
	if m.concurrencyLimit <= 0 {
		return nil // unlimited
	}
	// AddInt64 returns the new count. If it exceeds the limit, back out and reject.
	if atomic.AddInt64(&m.concurrentOps, 1) > m.concurrencyLimit {
		atomic.AddInt64(&m.concurrentOps, -1)
		return &JsonsError{
			Op:      "concurrency",
			Message: fmt.Sprintf("max concurrent operations (%d) reached", m.concurrencyLimit),
			Err:     ErrConcurrencyLimit,
		}
	}
	return nil
}

// releaseSemaphore releases a previously acquired concurrency slot.
func (p *Processor) releaseSemaphore() {
	if p == nil || p.metrics == nil {
		return
	}
	if p.metrics.concurrencyLimit > 0 {
		atomic.AddInt64(&p.metrics.concurrentOps, -1)
	}
}

// beginGovernedOp is the shared concurrency-governance prologue for operations.
// It registers the in-flight op (so Close() can drain it via waitForActiveOps)
// and acquires the concurrency semaphore. On success the caller MUST defer
// endGovernedOp() to release both.
//
// Get/Set/Delete route through it (Set/Delete via prepareOperation); SetMultiple
// uses it directly because it validates multiple paths and cannot reuse
// prepareOperation's single-path validation.
//
// Registration is race-free without a mutex via a double-check: the closed state
// is verified both before and after incrementing activeOps, and any failure path
// after the increment backs the counter out. This lets Close() never observe a
// false "zero in-flight" while an operation that will still proceed is
// mid-registration.
//
// PERFORMANCE: split into a begin/end pair rather than returning a closure.
// A returned func() escapes on every call (one heap allocation per governed
// op); deferring the bound method p.endGovernedOp() instead is open-coded by
// the compiler — allocation-free on the hot path.
func (p *Processor) beginGovernedOp() error {
	// Pre-check closed state before claiming an in-flight slot.
	if err := p.checkClosed(); err != nil {
		return err
	}
	atomic.AddInt64(&p.activeOps, 1)
	// Re-check after incrementing: back out if Close() has begun in the window.
	if err := p.checkClosed(); err != nil {
		atomic.AddInt64(&p.activeOps, -1)
		return err
	}
	if err := p.acquireSemaphore(); err != nil {
		atomic.AddInt64(&p.activeOps, -1)
		return err
	}
	return nil
}

// endGovernedOp releases the in-flight-op registration and concurrency slot
// acquired by beginGovernedOp. Kept small so it inlines and the defer in
// callers stays open-coded (allocation-free).
func (p *Processor) endGovernedOp() {
	p.releaseSemaphore()
	atomic.AddInt64(&p.activeOps, -1)
}

// prepareOperation prepares, governs, and validates a single-path operation.
// It acquires concurrency governance (in-flight registration + semaphore via
// beginGovernedOp), prepares and validates options, then validates input and
// path via the shared helper (handles SkipValidation).
//
// On success it returns the prepared options; the caller MUST defer
// releaseConfig(options) and p.endGovernedOp() — in that registration order, so
// they unwind in the correct reverse-acquire sequence (options first, then the
// governance slot). On failure every acquired resource is released here and a
// non-nil error is returned. Replaces the previous closure-returning form so no
// cleanup closure escapes to the heap.
func (p *Processor) prepareOperation(jsonStr, path string, cfg ...Config) (*Config, error) {
	if err := p.beginGovernedOp(); err != nil {
		return nil, err
	}

	options, err := p.prepareOptions(cfg...)
	if err != nil {
		p.endGovernedOp()
		return nil, err
	}

	// Validate input and path via the shared helper (handles SkipValidation).
	// This is the same logic Get uses, keeping all operations consistent.
	if err := p.validateOperationInput(jsonStr, path, options); err != nil {
		releaseConfig(options)
		p.endGovernedOp()
		return nil, err
	}

	return options, nil
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
		// SkipValidation skips input size/depth/security checks for performance,
		// but path SYNTAX is still validated. Syntax validation is a cheap single
		// pass and prevents silent data corruption: without it a non-numeric array
		// index like "arr[xyz]" is coerced to index 0 and overwrites existing
		// data, and an out-of-range index can trigger multi-GB allocations.
		if !isSimplePropertyAccess(path) {
			if err := internal.ValidatePath(path); err != nil {
				return newPathError(path, err.Error(), ErrInvalidPath)
			}
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

// unmarshalRootObject parses a JSON string and returns its root as a
// map[string]any when the root is a JSON object. It returns (nil, false, nil)
// when the root is a non-object value, so callers can fall back to the full
// recursive processor. Shared by the Get/Delete simple-property fast paths to
// avoid duplicating the parse + type-assert sequence (and its error wrapping).
func unmarshalRootObject(jsonStr string) (m map[string]any, isObject bool, err error) {
	var data any
	if uerr := json.Unmarshal(internal.StringToBytes(jsonStr), &data); uerr != nil {
		return nil, false, uerr
	}
	m, isObject = data.(map[string]any)
	return m, isObject, nil
}

