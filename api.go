package json

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"reflect"
	"slices"
	"sync"

	"github.com/cybergodev/json/internal"
)

// Cache size limits for configProcessorCache
const (
	configProcessorCacheLimit    = 64 // Maximum cached processors
	configProcessorCacheEvictNum = 16 // Number to evict when limit reached
	maxConcurrentCloses          = 8  // Limit concurrent async close goroutines
)

// Processor cache for config-based processor reuse
var (
	configProcessorCache   sync.Map   // map[uint64]*Processor
	configProcessorCacheMu sync.Mutex // Mutex for eviction serialization
)

// getProcessorOrFail returns the default processor or an error if unavailable.
// SAFETY: Use this for public APIs that return errors.
func getProcessorOrFail() (*Processor, error) {
	p := getDefaultProcessor()
	if p == nil {
		return nil, errInternalError
	}
	return p, nil
}

// processorForCfg returns the default processor when cfg is omitted, or a
// config-cached processor whose baked-in settings match the supplied cfg.
//
// It lets package-level functions that delegate to Processor methods honor an
// optional trailing Config by selecting the right processor, rather than
// threading cfg through every method signature. Since D-005 Phase 2 the
// JSONL/stream Processor methods also accept a per-call Config themselves, but
// the package level keeps the baked-processor route: it reuses the config-keyed
// processor cache across repeated calls. With no cfg it is identical to
// getProcessorOrFail (behavior unchanged). This mirrors how CompareJSON applies
// cfg via getProcessorWithConfig.
func processorForCfg(cfg ...Config) (*Processor, error) {
	if len(cfg) == 0 {
		return getProcessorOrFail()
	}
	return getProcessorWithConfig(cfg[0])
}

// =============================================================================
// Generic Processor Helpers - Reduces repetitive error handling patterns
// =============================================================================

// withProcessor is a generic helper that handles processor retrieval and error checking.
// This eliminates repetitive getProcessorOrFail() patterns across API functions.
func withProcessor[T any](fn func(*Processor) (T, error)) (T, error) {
	p, err := getProcessorOrFail()
	if err != nil {
		var zero T
		return zero, err
	}
	return fn(p)
}

// withProcessorStringResult handles operations that return string and should
// preserve the original jsonStr on error.
func withProcessorStringResult(fn func(*Processor) (string, error), jsonStr string) (string, error) {
	p, err := getProcessorOrFail()
	if err != nil {
		return jsonStr, err
	}
	return fn(p)
}

// withProcessorError handles operations that only return an error.
func withProcessorError(fn func(*Processor) error) error {
	p, err := getProcessorOrFail()
	if err != nil {
		return err
	}
	return fn(p)
}

// withTypedGetter handles the processor-or-default pattern for typed getters.
// Eliminates boilerplate in GetString, GetInt, GetFloat, GetBool, GetArray, GetObject.
func withTypedGetter[T any](fn func(*Processor, string, string, ...T) T, jsonStr, path string, defaultValue ...T) T {
	p, err := getProcessorOrFail()
	if err != nil {
		if len(defaultValue) > 0 {
			return defaultValue[0]
		}
		var zero T
		return zero
	}
	return fn(p, jsonStr, path, defaultValue...)
}

// hashConfig generates a cache key for Config for processor caching.
//
// ROBUSTNESS: Uses field-by-field hashing to include ALL Config fields.
// This ensures accurate cache keys and prevents collisions.
//
// PERFORMANCE: For the common case of default configs, uses a fast path that
// compares against default config using reflect-lite comparison.
func hashConfig(cfg Config) uint64 {
	// Fast path: check if this is a default config (most common case)
	if isDefaultConfig(cfg) {
		return 1 // Reserved hash for default config
	}

	// Slow path: hash all fields explicitly
	return hashConfigFields(cfg)
}

// isDefaultConfig checks if the config matches the default configuration.
// Performs complete comparison across every Config field via configFieldsEqual.
// PERFORMANCE: Uses short-circuit evaluation for common mismatches first.
func isDefaultConfig(cfg Config) bool {
	// Fast checks for common non-default values
	// These are ordered by likelihood of being modified
	if cfg.Pretty ||
		cfg.StrictMode ||
		!cfg.CreatePaths ||
		!cfg.EnableCache ||
		!cfg.EnableValidation ||
		cfg.CacheSharedResults {
		return false
	}

	// Check all fields against cached default
	return configFieldsEqual(cfg, cachedDefaultConfigValue)
}

// configFieldAccessor defines how to access and compare/hash a Config field.
// MAINTENANCE: Add new Config fields to this slice to ensure they are included
// in both comparison and hashing operations. This single source of truth prevents
// the functions from getting out of sync.
type configFieldAccessor struct {
	name  string
	equal func(a, b Config) bool
	hash  func(h uint64, cfg Config) uint64
}

// configFieldList defines all Config fields that should be compared/hashed.
// IMPORTANT: When adding new fields to Config, add them to this list.
var configFieldList = []configFieldAccessor{
	// Cache settings
	{"MaxCacheSize",
		func(a, b Config) bool { return a.MaxCacheSize == b.MaxCacheSize },
		func(h uint64, c Config) uint64 { return internal.HashInt(h, c.MaxCacheSize) }},
	{"CacheTTL",
		func(a, b Config) bool { return a.CacheTTL == b.CacheTTL },
		func(h uint64, c Config) uint64 { return internal.HashInt64(h, int64(c.CacheTTL)) }},
	{"EnableCache",
		func(a, b Config) bool { return a.EnableCache == b.EnableCache },
		func(h uint64, c Config) uint64 { return internal.HashBool(h, c.EnableCache) }},
	{"CacheResults",
		func(a, b Config) bool { return a.CacheResults == b.CacheResults },
		func(h uint64, c Config) uint64 { return internal.HashBool(h, c.CacheResults) }},
	{"CacheSharedResults",
		func(a, b Config) bool { return a.CacheSharedResults == b.CacheSharedResults },
		func(h uint64, c Config) uint64 { return internal.HashBool(h, c.CacheSharedResults) }},
	// Size limits
	{"MaxJSONSize",
		func(a, b Config) bool { return a.MaxJSONSize == b.MaxJSONSize },
		func(h uint64, c Config) uint64 { return internal.HashInt64(h, c.MaxJSONSize) }},
	{"MaxPathDepth",
		func(a, b Config) bool { return a.MaxPathDepth == b.MaxPathDepth },
		func(h uint64, c Config) uint64 { return internal.HashInt(h, c.MaxPathDepth) }},
	{"MaxBatchSize",
		func(a, b Config) bool { return a.MaxBatchSize == b.MaxBatchSize },
		func(h uint64, c Config) uint64 { return internal.HashInt(h, c.MaxBatchSize) }},
	// Security limits
	{"MaxNestingDepthSecurity",
		func(a, b Config) bool { return a.MaxNestingDepthSecurity == b.MaxNestingDepthSecurity },
		func(h uint64, c Config) uint64 { return internal.HashInt(h, c.MaxNestingDepthSecurity) }},
	{"MaxSecurityValidationSize",
		func(a, b Config) bool { return a.MaxSecurityValidationSize == b.MaxSecurityValidationSize },
		func(h uint64, c Config) uint64 { return internal.HashInt64(h, c.MaxSecurityValidationSize) }},
	{"MaxObjectKeys",
		func(a, b Config) bool { return a.MaxObjectKeys == b.MaxObjectKeys },
		func(h uint64, c Config) uint64 { return internal.HashInt(h, c.MaxObjectKeys) }},
	{"MaxArrayElements",
		func(a, b Config) bool { return a.MaxArrayElements == b.MaxArrayElements },
		func(h uint64, c Config) uint64 { return internal.HashInt(h, c.MaxArrayElements) }},
	{"FullSecurityScan",
		func(a, b Config) bool { return a.FullSecurityScan == b.FullSecurityScan },
		func(h uint64, c Config) uint64 { return internal.HashBool(h, c.FullSecurityScan) }},
	// Concurrency
	{"MaxConcurrency",
		func(a, b Config) bool { return a.MaxConcurrency == b.MaxConcurrency },
		func(h uint64, c Config) uint64 { return internal.HashInt(h, c.MaxConcurrency) }},
	{"MaxOperationsPerSecond",
		func(a, b Config) bool { return a.MaxOperationsPerSecond == b.MaxOperationsPerSecond },
		func(h uint64, c Config) uint64 { return internal.HashInt(h, c.MaxOperationsPerSecond) }},
	{"ParallelThreshold",
		func(a, b Config) bool { return a.ParallelThreshold == b.ParallelThreshold },
		func(h uint64, c Config) uint64 { return internal.HashInt(h, c.ParallelThreshold) }},
	// Processing options
	{"EnableValidation",
		func(a, b Config) bool { return a.EnableValidation == b.EnableValidation },
		func(h uint64, c Config) uint64 { return internal.HashBool(h, c.EnableValidation) }},
	{"StrictMode",
		func(a, b Config) bool { return a.StrictMode == b.StrictMode },
		func(h uint64, c Config) uint64 { return internal.HashBool(h, c.StrictMode) }},
	{"CreatePaths",
		func(a, b Config) bool { return a.CreatePaths == b.CreatePaths },
		func(h uint64, c Config) uint64 { return internal.HashBool(h, c.CreatePaths) }},
	{"CleanupNulls",
		func(a, b Config) bool { return a.CleanupNulls == b.CleanupNulls },
		func(h uint64, c Config) uint64 { return internal.HashBool(h, c.CleanupNulls) }},
	{"CompactArrays",
		func(a, b Config) bool { return a.CompactArrays == b.CompactArrays },
		func(h uint64, c Config) uint64 { return internal.HashBool(h, c.CompactArrays) }},
	{"ContinueOnError",
		func(a, b Config) bool { return a.ContinueOnError == b.ContinueOnError },
		func(h uint64, c Config) uint64 { return internal.HashBool(h, c.ContinueOnError) }},
	// Input/Output options
	{"AllowComments",
		func(a, b Config) bool { return a.AllowComments == b.AllowComments },
		func(h uint64, c Config) uint64 { return internal.HashBool(h, c.AllowComments) }},
	{"PreserveNumbers",
		func(a, b Config) bool { return a.PreserveNumbers == b.PreserveNumbers },
		func(h uint64, c Config) uint64 { return internal.HashBool(h, c.PreserveNumbers) }},
	{"ValidateInput",
		func(a, b Config) bool { return a.ValidateInput == b.ValidateInput },
		func(h uint64, c Config) uint64 { return internal.HashBool(h, c.ValidateInput) }},
	{"ValidateFilePath",
		func(a, b Config) bool { return a.ValidateFilePath == b.ValidateFilePath },
		func(h uint64, c Config) uint64 { return internal.HashBool(h, c.ValidateFilePath) }},
	{"SkipValidation",
		func(a, b Config) bool { return a.SkipValidation == b.SkipValidation },
		func(h uint64, c Config) uint64 { return internal.HashBool(h, c.SkipValidation) }},
	// Encoding options
	{"Pretty",
		func(a, b Config) bool { return a.Pretty == b.Pretty },
		func(h uint64, c Config) uint64 { return internal.HashBool(h, c.Pretty) }},
	{"Indent",
		func(a, b Config) bool { return a.Indent == b.Indent },
		func(h uint64, c Config) uint64 { return internal.HashString(h, c.Indent) }},
	{"Prefix",
		func(a, b Config) bool { return a.Prefix == b.Prefix },
		func(h uint64, c Config) uint64 { return internal.HashString(h, c.Prefix) }},
	{"EscapeHTML",
		func(a, b Config) bool { return a.EscapeHTML == b.EscapeHTML },
		func(h uint64, c Config) uint64 { return internal.HashBool(h, c.EscapeHTML) }},
	{"SortKeys",
		func(a, b Config) bool { return a.SortKeys == b.SortKeys },
		func(h uint64, c Config) uint64 { return internal.HashBool(h, c.SortKeys) }},
	{"ValidateUTF8",
		func(a, b Config) bool { return a.ValidateUTF8 == b.ValidateUTF8 },
		func(h uint64, c Config) uint64 { return internal.HashBool(h, c.ValidateUTF8) }},
	{"MaxDepth",
		func(a, b Config) bool { return a.MaxDepth == b.MaxDepth },
		func(h uint64, c Config) uint64 { return internal.HashInt(h, c.MaxDepth) }},
	{"DisallowUnknown",
		func(a, b Config) bool { return a.DisallowUnknown == b.DisallowUnknown },
		func(h uint64, c Config) uint64 { return internal.HashBool(h, c.DisallowUnknown) }},
	{"FloatPrecision",
		func(a, b Config) bool { return a.FloatPrecision == b.FloatPrecision },
		func(h uint64, c Config) uint64 { return internal.HashInt(h, c.FloatPrecision) }},
	{"FloatTruncate",
		func(a, b Config) bool { return a.FloatTruncate == b.FloatTruncate },
		func(h uint64, c Config) uint64 { return internal.HashBool(h, c.FloatTruncate) }},
	{"DisableEscaping",
		func(a, b Config) bool { return a.DisableEscaping == b.DisableEscaping },
		func(h uint64, c Config) uint64 { return internal.HashBool(h, c.DisableEscaping) }},
	{"EscapeUnicode",
		func(a, b Config) bool { return a.EscapeUnicode == b.EscapeUnicode },
		func(h uint64, c Config) uint64 { return internal.HashBool(h, c.EscapeUnicode) }},
	{"EscapeSlash",
		func(a, b Config) bool { return a.EscapeSlash == b.EscapeSlash },
		func(h uint64, c Config) uint64 { return internal.HashBool(h, c.EscapeSlash) }},
	{"EscapeNewlines",
		func(a, b Config) bool { return a.EscapeNewlines == b.EscapeNewlines },
		func(h uint64, c Config) uint64 { return internal.HashBool(h, c.EscapeNewlines) }},
	{"EscapeTabs",
		func(a, b Config) bool { return a.EscapeTabs == b.EscapeTabs },
		func(h uint64, c Config) uint64 { return internal.HashBool(h, c.EscapeTabs) }},
	{"IncludeNulls",
		func(a, b Config) bool { return a.IncludeNulls == b.IncludeNulls },
		func(h uint64, c Config) uint64 { return internal.HashBool(h, c.IncludeNulls) }},
	// Observability
	{"EnableMetrics",
		func(a, b Config) bool { return a.EnableMetrics == b.EnableMetrics },
		func(h uint64, c Config) uint64 { return internal.HashBool(h, c.EnableMetrics) }},
	{"EnableHealthCheck",
		func(a, b Config) bool { return a.EnableHealthCheck == b.EnableHealthCheck },
		func(h uint64, c Config) uint64 { return internal.HashBool(h, c.EnableHealthCheck) }},
	// Merge Options
	{"MergeMode",
		func(a, b Config) bool { return a.MergeMode == b.MergeMode },
		func(h uint64, c Config) uint64 { return internal.HashInt(h, int(c.MergeMode)) }},
	// Large File Processing
	{"ChunkSize",
		func(a, b Config) bool { return a.ChunkSize == b.ChunkSize },
		func(h uint64, c Config) uint64 { return internal.HashInt64(h, c.ChunkSize) }},
	{"MaxMemory",
		func(a, b Config) bool { return a.MaxMemory == b.MaxMemory },
		func(h uint64, c Config) uint64 { return internal.HashInt64(h, c.MaxMemory) }},
	{"BufferSize",
		func(a, b Config) bool { return a.BufferSize == b.BufferSize },
		func(h uint64, c Config) uint64 { return internal.HashInt(h, c.BufferSize) }},
	{"SamplingEnabled",
		func(a, b Config) bool { return a.SamplingEnabled == b.SamplingEnabled },
		func(h uint64, c Config) uint64 { return internal.HashBool(h, c.SamplingEnabled) }},
	{"SampleSize",
		func(a, b Config) bool { return a.SampleSize == b.SampleSize },
		func(h uint64, c Config) uint64 { return internal.HashInt(h, c.SampleSize) }},
	// JSONL Configuration
	{"JSONLBufferSize",
		func(a, b Config) bool { return a.JSONLBufferSize == b.JSONLBufferSize },
		func(h uint64, c Config) uint64 { return internal.HashInt(h, c.JSONLBufferSize) }},
	{"JSONLMaxLineSize",
		func(a, b Config) bool { return a.JSONLMaxLineSize == b.JSONLMaxLineSize },
		func(h uint64, c Config) uint64 { return internal.HashInt(h, c.JSONLMaxLineSize) }},
	{"JSONLSkipEmpty",
		func(a, b Config) bool { return a.JSONLSkipEmpty == b.JSONLSkipEmpty },
		func(h uint64, c Config) uint64 { return internal.HashBool(h, c.JSONLSkipEmpty) }},
	{"JSONLSkipComments",
		func(a, b Config) bool { return a.JSONLSkipComments == b.JSONLSkipComments },
		func(h uint64, c Config) uint64 { return internal.HashBool(h, c.JSONLSkipComments) }},
	{"JSONLContinueOnErr",
		func(a, b Config) bool { return a.JSONLContinueOnErr == b.JSONLContinueOnErr },
		func(h uint64, c Config) uint64 { return internal.HashBool(h, c.JSONLContinueOnErr) }},
	{"JSONLWorkers",
		func(a, b Config) bool { return a.JSONLWorkers == b.JSONLWorkers },
		func(h uint64, c Config) uint64 { return internal.HashInt(h, c.JSONLWorkers) }},
	{"JSONLChunkSize",
		func(a, b Config) bool { return a.JSONLChunkSize == b.JSONLChunkSize },
		func(h uint64, c Config) uint64 { return internal.HashInt(h, c.JSONLChunkSize) }},
	{"JSONLMaxMemory",
		func(a, b Config) bool { return a.JSONLMaxMemory == b.JSONLMaxMemory },
		func(h uint64, c Config) uint64 { return internal.HashInt64(h, c.JSONLMaxMemory) }},
	// Extension fields
	{"CustomEscapes",
		func(a, b Config) bool { return customEscapesEqual(a.CustomEscapes, b.CustomEscapes) },
		func(h uint64, c Config) uint64 { return hashCustomEscapes(h, c.CustomEscapes) }},
	{"CustomEncoder",
		func(a, b Config) bool {
			if (a.CustomEncoder == nil) != (b.CustomEncoder == nil) {
				return false
			}
			if a.CustomEncoder == nil {
				return true
			}
			return fmt.Sprintf("%T", a.CustomEncoder) == fmt.Sprintf("%T", b.CustomEncoder)
		},
		func(h uint64, c Config) uint64 {
			if c.CustomEncoder != nil {
				h = internal.HashBool(h, true)
				h = internal.HashString(h, fmt.Sprintf("%T", c.CustomEncoder))
				return h
			}
			return h
		}},
	{"CustomTypeEncoders",
		func(a, b Config) bool {
			if len(a.CustomTypeEncoders) != len(b.CustomTypeEncoders) {
				return false
			}
			for k, v := range a.CustomTypeEncoders {
				// Type-only comparison, matching the hash: encoder values are
				// often funcs, which are not comparable with == (a func-based
				// TypeEncoder would panic on the interface != below).
				if bv, ok := b.CustomTypeEncoders[k]; !ok || reflect.TypeOf(v) != reflect.TypeOf(bv) {
					return false
				}
			}
			return true
		},
		func(h uint64, c Config) uint64 {
			h = internal.HashInt(h, len(c.CustomTypeEncoders))
			// DETERMINISM FIX: map iteration order is random, so combining per-entry
			// sub-hashes with XOR (commutative & associative) yields a stable key
			// regardless of iteration order. Each sub-hash incorporates the encoder's
			// concrete type (not just nil-ness) to reduce collisions between distinct
			// encoder functions. (Same residual instance-collision limitation as
			// CustomEncoder/Hooks above.)
			var combined uint64
			for typ, enc := range c.CustomTypeEncoders {
				eh := internal.FNVOffsetBasis
				eh = internal.HashString(eh, typ.String())
				eh = internal.HashString(eh, fmt.Sprintf("%T", enc))
				combined ^= eh
			}
			return internal.HashUint64(h, combined)
		}},
	{"CustomValidators",
		func(a, b Config) bool {
			if len(a.CustomValidators) != len(b.CustomValidators) {
				return false
			}
			for i, v := range a.CustomValidators {
				// Type-only comparison, matching the hash: validator values are
				// funcs and not comparable with ==.
				if reflect.TypeOf(v) != reflect.TypeOf(b.CustomValidators[i]) {
					return false
				}
			}
			return true
		},
		func(h uint64, c Config) uint64 {
			h = internal.HashInt(h, len(c.CustomValidators))
			// Incorporate each validator's concrete type (not just nil-ness) to reduce
			// collisions between distinct validator functions. Slice order is stable,
			// so order-dependent folding is safe here (unlike the map cases above).
			for _, v := range c.CustomValidators {
				h = internal.HashString(h, fmt.Sprintf("%T", v))
			}
			return h
		}},
	{"AdditionalDangerousPatterns",
		func(a, b Config) bool {
			if len(a.AdditionalDangerousPatterns) != len(b.AdditionalDangerousPatterns) {
				return false
			}
			for i, p := range a.AdditionalDangerousPatterns {
				if p != b.AdditionalDangerousPatterns[i] {
					return false
				}
			}
			return true
		},
		func(h uint64, c Config) uint64 {
			h = internal.HashInt(h, len(c.AdditionalDangerousPatterns))
			for _, p := range c.AdditionalDangerousPatterns {
				h = internal.HashString(h, p.Pattern)
				h = internal.HashInt(h, int(p.Level))
			}
			return h
		}},
	{"DisableDefaultPatterns",
		func(a, b Config) bool { return a.DisableDefaultPatterns == b.DisableDefaultPatterns },
		func(h uint64, c Config) uint64 { return internal.HashBool(h, c.DisableDefaultPatterns) }},
	{"DetectDuplicateKeys",
		func(a, b Config) bool { return a.DetectDuplicateKeys == b.DetectDuplicateKeys },
		func(h uint64, c Config) uint64 { return internal.HashBool(h, c.DetectDuplicateKeys) }},
	{"SaveFileMode",
		func(a, b Config) bool { return a.SaveFileMode == b.SaveFileMode },
		func(h uint64, c Config) uint64 { return internal.HashInt(h, int(c.SaveFileMode)) }},
	{"AllowedFileDirs",
		func(a, b Config) bool {
			if len(a.AllowedFileDirs) != len(b.AllowedFileDirs) {
				return false
			}
			for i, d := range a.AllowedFileDirs {
				if d != b.AllowedFileDirs[i] {
					return false
				}
			}
			return true
		},
		func(h uint64, c Config) uint64 {
			h = internal.HashInt(h, len(c.AllowedFileDirs))
			for _, d := range c.AllowedFileDirs {
				h = internal.HashString(h, d)
			}
			return h
		}},
	{"Hooks",
		func(a, b Config) bool {
			if len(a.Hooks) != len(b.Hooks) {
				return false
			}
			for i := range a.Hooks {
				// Type-only comparison, matching the hash: a Hook implemented
				// as a func type is not comparable with == (panics at runtime).
				if reflect.TypeOf(a.Hooks[i]) != reflect.TypeOf(b.Hooks[i]) {
					return false
				}
			}
			return true
		},
		func(h uint64, c Config) uint64 {
			h = internal.HashInt(h, len(c.Hooks))
			for _, hook := range c.Hooks {
				h = internal.HashString(h, fmt.Sprintf("%T", hook))
			}
			return h
		}},
	{"CustomPathParser",
		func(a, b Config) bool { return (a.CustomPathParser == nil) == (b.CustomPathParser == nil) },
		func(h uint64, c Config) uint64 {
			if c.CustomPathParser != nil {
				return internal.HashBool(h, true)
			}
			return h
		}},
}

// configFieldsEqual compares all fields of two Config structs.
// MAINTENANCE: To add new Config fields, add entries to configFieldList only.
// This function is the single consumer of configFieldList.equal for comparisons.
func configFieldsEqual(a, b Config) bool {
	for _, field := range configFieldList {
		if !field.equal(a, b) {
			return false
		}
	}
	return true
}

// customEscapesEqual compares two CustomEscapes maps
func customEscapesEqual(a, b map[rune]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if bv, ok := b[k]; !ok || bv != v {
			return false
		}
	}
	return true
}

// hashConfigFields hashes all Config fields using the unified field list.
// MAINTENANCE: To add new Config fields, add entries to configFieldList only.
// This function is the single consumer of configFieldList.hash for hashing.
func hashConfigFields(cfg Config) uint64 {
	h := internal.FNVOffsetBasis
	for _, field := range configFieldList {
		h = field.hash(h, cfg)
	}
	return h
}

// hashCustomEscapes hashes a CustomEscapes map
func hashCustomEscapes(h uint64, m map[rune]string) uint64 {
	if m == nil {
		return h
	}
	h = internal.HashInt(h, len(m))
	// DETERMINISM FIX: Sort keys to ensure consistent hash regardless of map iteration order
	keys := make([]rune, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	for _, k := range keys {
		h = internal.HashInt(h, int(k))
		h = internal.HashString(h, m[k])
	}
	return h
}

// =============================================================================
// Core Get Operations - Unified API
// =============================================================================

// Get retrieves a value from JSON at the specified path.
// Returns the value as any and requires type assertion.
//
// Errors:
//   - ErrInvalidJSON: jsonStr is not valid JSON
//   - ErrPathNotFound: path does not exist in the JSON structure
//   - ErrProcessorClosed: processor has been closed
//
// Example:
//
//	value, err := json.Get(`{"user":{"name":"Alice"}}`, "user.name")
//	if err != nil {
//	    // Handle error
//	}
//	name := value.(string)
func Get(jsonStr, path string, cfg ...Config) (any, error) {
	return withProcessor(func(p *Processor) (any, error) {
		return p.Get(jsonStr, path, cfg...)
	})
}

// GetWithContext retrieves a value from JSON with boundary-level context checks.
// Context is checked before and after the operation, NOT during parsing/navigation.
// For large JSON documents, the operation may not respond to cancellation mid-parse.
// This is the context-aware version of Get() that supports timeout deadlines.
//
// Example:
//
//	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
//	defer cancel()
//	value, err := json.GetWithContext(ctx, `{"user":{"name":"Alice"}}`, "user.name")
func GetWithContext(ctx context.Context, jsonStr, path string, cfg ...Config) (any, error) {
	return withProcessor(func(p *Processor) (any, error) {
		return p.GetWithContext(ctx, jsonStr, path, cfg...)
	})
}

// =============================================================================
// Typed Get Operations
//
// The typed getters below (GetTyped, GetString, GetInt, ...) do NOT accept a
// Config argument: their variadic parameter is already consumed by the default
// value (Go permits only one variadic parameter per function). They use the
// default processor. For Config-controlled validation/security on a typed read,
// use SafeGet(str, path, cfg) (returns an AccessResult with AsString/AsInt/...),
// or construct a processor with the desired config: New(cfg).GetString(...).
// =============================================================================

// GetTyped retrieves a typed value from JSON at the specified path.
// Returns defaultValue if provided, otherwise zero value of T when: path not found, value is null, or type conversion fails.
//
// Example:
//
//	name := json.GetTyped[string](data, "user.name", "unknown")
//	age := json.GetTyped[int](data, "user.age", 0)
//	name := json.GetTyped[string](data, "user.name") // returns "" if not found
func GetTyped[T any](jsonStr, path string, defaultValue ...T) T {
	return withTypedGetter(getTypedWithDefault[T], jsonStr, path, defaultValue...)
}

// GetString retrieves a string value from JSON at the specified path.
// Returns defaultValue if provided, otherwise "" when: path not found, value is null, or type conversion fails.
func GetString(jsonStr, path string, defaultValue ...string) string {
	return withTypedGetter((*Processor).GetString, jsonStr, path, defaultValue...)
}

// GetInt retrieves an int value from JSON at the specified path.
// Returns defaultValue if provided, otherwise 0 when: path not found, value is null, or type conversion fails.
func GetInt(jsonStr, path string, defaultValue ...int) int {
	return withTypedGetter((*Processor).GetInt, jsonStr, path, defaultValue...)
}

// GetFloat retrieves a float64 value from JSON at the specified path.
// Returns defaultValue if provided, otherwise 0.0 when: path not found, value is null, or type conversion fails.
func GetFloat(jsonStr, path string, defaultValue ...float64) float64 {
	return withTypedGetter((*Processor).GetFloat, jsonStr, path, defaultValue...)
}

// GetBool retrieves a bool value from JSON at the specified path.
// Returns defaultValue if provided, otherwise false when: path not found, value is null, or type conversion fails.
func GetBool(jsonStr, path string, defaultValue ...bool) bool {
	return withTypedGetter((*Processor).GetBool, jsonStr, path, defaultValue...)
}

// GetArray retrieves an array value from JSON at the specified path.
// Returns defaultValue if provided, otherwise nil when: path not found, value is null, or type conversion fails.
func GetArray(jsonStr, path string, defaultValue ...[]any) []any {
	return withTypedGetter((*Processor).GetArray, jsonStr, path, defaultValue...)
}

// GetObject retrieves an object value from JSON at the specified path.
// Returns defaultValue if provided, otherwise nil when: path not found, value is null, or type conversion fails.
func GetObject(jsonStr, path string, defaultValue ...map[string]any) map[string]any {
	return withTypedGetter((*Processor).GetObject, jsonStr, path, defaultValue...)
}

// GetMultiple retrieves multiple values from JSON at the specified paths.
// Returns a map of path to value for each successfully retrieved path.
//
// Errors:
//   - ErrInvalidJSON: jsonStr is not valid JSON
func GetMultiple(jsonStr string, paths []string, cfg ...Config) (map[string]any, error) {
	return withProcessor(func(p *Processor) (map[string]any, error) {
		return p.GetMultiple(jsonStr, paths, cfg...)
	})
}

// SafeGet performs a type-safe get operation returning an AccessResult
// with type conversion methods (AsString, AsInt, AsFloat64, AsBool).
// Accepts optional Config for controlling validation, security, and caching behavior.
//
// Example:
//
//	result := json.SafeGet(data, "user.age")
//	if result.Ok() {
//	    age, _ := result.AsInt()
//	}
func SafeGet(jsonStr, path string, cfg ...Config) AccessResult {
	p, err := getProcessorOrFail()
	if err != nil {
		return AccessResult{Exists: false}
	}
	return p.SafeGet(jsonStr, path, cfg...)
}

// Set sets a value in JSON at the specified path.
// Creates intermediate paths if Config.CreatePaths is true.
// A root path ("", ".", or the JSON Pointer root "/") replaces the entire
// document with value (GEN-001).
//
// Returns:
//   - On success: modified JSON string and nil error
//   - On failure: original unmodified JSON string and error information
//
// Errors:
//   - ErrInvalidJSON: jsonStr is not valid JSON
//   - ErrInvalidPath: path syntax is invalid
//   - ErrPathNotFound: path does not exist and CreatePaths is false
//   - ErrTypeMismatch: cannot set value at path due to type conflict
func Set(jsonStr, path string, value any, cfg ...Config) (string, error) {
	return withProcessorStringResult(func(p *Processor) (string, error) {
		return p.Set(jsonStr, path, value, cfg...)
	}, jsonStr)
}

// SetMultiple sets multiple values using a map of path-value pairs.
// Creates intermediate paths if Config.CreatePaths is true.
//
// Errors:
//   - ErrInvalidJSON: jsonStr is not valid JSON
//   - ErrInvalidPath: any path syntax is invalid
//   - ErrPathNotFound: path does not exist and CreatePaths is false
func SetMultiple(jsonStr string, updates map[string]any, cfg ...Config) (string, error) {
	return withProcessorStringResult(func(p *Processor) (string, error) {
		return p.SetMultiple(jsonStr, updates, cfg...)
	}, jsonStr)
}

// SetCreate sets a value in JSON at the specified path, creating intermediate paths as needed.
// This is equivalent to calling Set with CreatePaths enabled.
//
// Example:
//
//	result, err := json.SetCreate(data, "users[0].profile.name", "Alice")
func SetCreate(jsonStr, path string, value any, cfg ...Config) (string, error) {
	return withProcessorStringResult(func(p *Processor) (string, error) {
		return p.SetCreate(jsonStr, path, value, cfg...)
	}, jsonStr)
}

// SetMultipleCreate sets multiple values, creating intermediate paths as needed.
// This is equivalent to calling SetMultiple with CreatePaths enabled.
//
// Example:
//
//	result, err := json.SetMultipleCreate(data, map[string]any{"user.name": "Alice", "user.age": 30})
func SetMultipleCreate(jsonStr string, updates map[string]any, cfg ...Config) (string, error) {
	return withProcessorStringResult(func(p *Processor) (string, error) {
		return p.SetMultipleCreate(jsonStr, updates, cfg...)
	}, jsonStr)
}

// Delete deletes a value from JSON at the specified path.
//
// Errors:
//   - ErrInvalidJSON: jsonStr is not valid JSON
//   - ErrPathNotFound: path does not exist
//   - ErrInvalidPath: path syntax is invalid
func Delete(jsonStr, path string, cfg ...Config) (string, error) {
	return withProcessorStringResult(func(p *Processor) (string, error) {
		return p.Delete(jsonStr, path, cfg...)
	}, jsonStr)
}

// DeleteClean deletes a value from JSON and cleans up null values and empty arrays.
// This is equivalent to calling Delete with CleanupNulls and CompactArrays enabled.
//
// Example:
//
//	result, err := json.DeleteClean(data, "users[0].profile")
func DeleteClean(jsonStr, path string, cfg ...Config) (string, error) {
	return withProcessorStringResult(func(p *Processor) (string, error) {
		return p.DeleteClean(jsonStr, path, cfg...)
	}, jsonStr)
}

// Marshal returns the JSON encoding of v.
// Signature-compatible with encoding/json.Marshal and byte-compatible for
// typical values, with one deliberate difference (D-002 doc correction): the
// encoded output is capped at Config.MaxJSONSize (default 100MB) and returns
// ErrSizeLimit instead of succeeding — stdlib has no such cap.
//
// For configuration options (indentation, number handling, etc.), pass an
// optional Config. This mirrors Processor.Marshal, making the package-level
// and processor-level APIs true mirrors of each other.
//
// Example:
//
//	// Drop-in compatible with encoding/json (no config)
//	b, err := json.Marshal(value)
//
//	// With configuration (non-breaking, optional trailing Config)
//	b, err = json.Marshal(value, json.PrettyConfig())
func Marshal(value any, cfg ...Config) ([]byte, error) {
	return withProcessor(func(p *Processor) ([]byte, error) {
		return p.Marshal(value, cfg...)
	})
}

// Unmarshal parses the JSON-encoded data and stores the result in v.
// Signature-compatible with encoding/json.Unmarshal, but the no-config call
// applies this library's default security hardening — deliberate differences
// from the standard library (D-002 doc correction):
//   - input larger than Config.MaxJSONSize (default 100MB) → ErrSizeLimit
//   - dangerous-pattern substrings in string values (e.g. "__proto__",
//     "<script") → security violation; stdlib accepts them as plain data
//   - invalid UTF-8 in string values → rejected; stdlib replaces it
//
// Pass SkipValidation: true in a Config for stdlib-exact lenient behavior on
// trusted input, or a custom Config to tighten/loosen the limits.
//
// For configuration options (security limits, number preservation, etc.), pass
// an optional Config. This mirrors Processor.Unmarshal.
//
// Example:
//
//	// Drop-in compatible with encoding/json (no config)
//	err := json.Unmarshal(data, &v)
//
//	// With configuration (non-breaking, optional trailing Config)
//	err = json.Unmarshal(data, &v, json.SecurityConfig())
func Unmarshal(data []byte, value any, cfg ...Config) error {
	return withProcessorError(func(p *Processor) error {
		return p.Unmarshal(data, value, cfg...)
	})
}

// MarshalIndent is like Marshal but applies indentation to format the output.
// This function is 100% compatible with encoding/json.MarshalIndent: calling
// it as json.MarshalIndent(v, prefix, indent) behaves identically to the
// standard library.
//
// For additional configuration options, pass an optional Config. This mirrors
// Processor.MarshalIndent. The prefix and indent arguments override the
// corresponding Config fields.
//
// Example:
//
//	// Drop-in compatible with encoding/json (no config)
//	b, err := json.MarshalIndent(v, "", "  ")
//
//	// With configuration (non-breaking, optional trailing Config)
//	b, err = json.MarshalIndent(v, "", "  ", json.SecurityConfig())
func MarshalIndent(v any, prefix, indent string, cfg ...Config) ([]byte, error) {
	return withProcessor(func(p *Processor) ([]byte, error) {
		return p.MarshalIndent(v, prefix, indent, cfg...)
	})
}

// Compact appends to dst the JSON-encoded src with insignificant space characters elided.
// This function is 100% compatible with encoding/json.Compact.
// Accepts optional Config to control compact behavior (e.g., number preservation).
//
// This is the buffer form: json.Compact(dst, src) mirrors Processor.CompactBuffer.
// For a string-in/string-out form, see CompactString, which mirrors Processor.Compact.
//
// Example:
//
//	// encoding/json compatible usage (no Config)
//	var buf bytes.Buffer
//	err := json.Compact(&buf, []byte(`{"name": "Alice"}`))
//
//	// With configuration
//	cfg := json.DefaultConfig()
//	cfg.PreserveNumbers = true
//	err = json.Compact(&buf, []byte(jsonStr), cfg)
func Compact(dst *bytes.Buffer, src []byte, cfg ...Config) error {
	return withProcessorError(func(p *Processor) error {
		return p.CompactBuffer(dst, src, cfg...)
	})
}

// CompactString removes insignificant whitespace from a JSON string and returns
// the compacted result. It is the package-level mirror of Processor.Compact
// (json.CompactString(s, cfg) behaves like p.Compact(s, cfg) on a processor with
// the matching configuration), symmetric with Prettify mirroring Processor.Prettify.
//
// Note: this is distinct from Compact. Compact is the encoding/json-compatible
// buffer form (json.Compact(dst, src) ↔ Processor.CompactBuffer); CompactString is
// the string-in/string-out form (json.CompactString(s) ↔ Processor.Compact). The two
// share a name with their Processor counterparts respectively, preserving the mirror.
//
// Example:
//
//	compact, err := json.CompactString(`{
//	    "name": "Alice",
//	    "age": 30
//	}`)
//	// compact == `{"name":"Alice","age":30}`
//
//	// With configuration (e.g., preserve original number formatting)
//	cfg := json.DefaultConfig()
//	cfg.PreserveNumbers = true
//	compact, err = json.CompactString(jsonStr, cfg)
func CompactString(jsonStr string, cfg ...Config) (string, error) {
	return withProcessor(func(p *Processor) (string, error) {
		return p.Compact(jsonStr, cfg...)
	})
}

// Indent appends to dst an indented form of the JSON-encoded src.
// This function is 100% compatible with encoding/json.Indent.
// Accepts optional Config for controlling indentation behavior.
//
// Example:
//
//	var buf bytes.Buffer
//	err := json.Indent(&buf, []byte(`{"name":"Alice"}`), "", "  ")
func Indent(dst *bytes.Buffer, src []byte, prefix, indent string, cfg ...Config) error {
	return withProcessorError(func(p *Processor) error {
		return p.Indent(dst, src, prefix, indent, cfg...)
	})
}

// HTMLEscape appends to dst the JSON-encoded src with <, >, &, U+2028, and U+2029 characters escaped.
// This function is 100% compatible with encoding/json.HTMLEscape.
// Accepts optional Config for consistent API pattern.
//
// Example:
//
//	var buf bytes.Buffer
//	json.HTMLEscape(&buf, []byte(`{"url":"<script>alert(1)</script>"}`))
func HTMLEscape(dst *bytes.Buffer, src []byte, cfg ...Config) {
	p := getDefaultProcessor()
	if p == nil {
		internal.HTMLEscapeTo(dst, string(src))
		return
	}
	p.HTMLEscape(dst, src, cfg...)
}

// Encode converts any Go value to JSON string.
// This is the canonical string-returning encoder; Marshal is the []byte-returning
// encoding/json drop-in. The optional trailing Config selects the encoding
// behavior for this call (Pretty, EscapeHTML, SortKeys, FloatPrecision, ...);
// omitted, the default configuration applies.
//
// Example:
//
//	// Default configuration
//	result, err := json.Encode(data)
//
//	// Pretty output
//	result, err := json.Encode(data, json.PrettyConfig())
//
//	// Custom configuration
//	cfg := json.DefaultConfig()
//	cfg.SortKeys = true
//	result, err := json.Encode(data, cfg)
//
// Errors:
//   - ErrProcessorClosed: the default processor has been closed
//   - UnsupportedTypeError / UnsupportedValueError / MarshalerError: value cannot be encoded
//   - ErrSizeLimit: encoded output exceeds MaxJSONSize
//   - ErrDepthLimit: encoding exceeds the maximum nesting depth
func Encode(value any, cfg ...Config) (string, error) {
	return withProcessor(func(p *Processor) (string, error) {
		return p.Encode(value, cfg...)
	})
}

// EncodePretty converts any Go value to pretty-printed JSON string.
// This is the package-level equivalent of Processor.EncodePretty().
//
// Example:
//
//	result, err := json.EncodePretty(data)
//
//	// With custom configuration
//	cfg := json.DefaultConfig()
//	cfg.Indent = "    "
//	result, err := json.EncodePretty(data, cfg)
func EncodePretty(value any, cfg ...Config) (string, error) {
	return withProcessor(func(p *Processor) (string, error) {
		return p.EncodePretty(value, cfg...)
	})
}

// EncodeWithConfig converts any Go value to JSON string using the unified Config.
//
// Deprecated: EncodeWithConfig is functionally identical to Encode — the name
// predates Encode accepting an optional trailing Config. Use Encode(value, cfg)
// (or PrettyConfig()/SecurityConfig() presets). EncodeWithConfig will not be
// removed within v1 (per D-005 the module stays on v1.x).
//
// Errors: see Encode.
func EncodeWithConfig(value any, cfg ...Config) (string, error) {
	return withProcessor(func(p *Processor) (string, error) {
		return p.Encode(value, cfg...)
	})
}

// Prettify formats JSON string with pretty indentation.
// This is the recommended function for formatting JSON strings.
//
// Example:
//
//	pretty, err := json.Prettify(`{"name":"Alice","age":30}`)
//	// Output:
//	// {
//	//   "name": "Alice",
//	//   "age": 30
//	// }
func Prettify(jsonStr string, cfg ...Config) (string, error) {
	return withProcessor(func(p *Processor) (string, error) {
		return p.Prettify(jsonStr, cfg...)
	})
}

// Valid reports whether data is valid JSON.
// Signature-compatible with encoding/json.Valid and returns a plain bool.
//
// NOTE (D-002 doc correction): the no-config call is NOT purely syntactic
// like the standard library — it routes through the default processor's
// security validation, so input that is syntactically valid JSON can still
// return false when it exceeds MaxJSONSize, contains invalid UTF-8, or
// carries a dangerous pattern (e.g. "__proto__", "<script") in a string
// value. Callers needing stdlib-exact syntax-only checks should use
// encoding/json.Valid (or a Config with SkipValidation: true via
// ValidWithConfig).
//
// Example:
//
//	// Drop-in compatible with encoding/json (no config)
//	if json.Valid(data) { /* ... */ }
//
//	// With configuration (non-breaking, optional trailing Config)
//	if json.Valid(data, json.SecurityConfig()) { /* ... */ }
//
// Note: ValidWithConfig returns (bool, error) for callers that need to inspect
// the validation error; Valid intentionally collapses errors to a bool.
func Valid(data []byte, cfg ...Config) bool {
	p := getDefaultProcessor()
	if p == nil {
		// Fallback: use simple validation when processor is unavailable
		return isValidJSON(string(data))
	}
	if len(cfg) == 0 {
		return p.ValidBytes(data)
	}
	// Config-aware path: forward to Processor.Valid and collapse error to false.
	ok, err := p.Valid(string(data), cfg...)
	return err == nil && ok
}

// ValidWithConfig reports whether jsonStr is valid JSON, returning the parse
// error when it is not.
//
// Note: the name predates Valid accepting an optional Config — both functions
// now take cfg. The actual difference from Valid is the return type: Valid
// collapses any error to a single bool (encoding/json drop-in), whereas
// ValidWithConfig returns (bool, error) so callers can inspect why validation
// failed. The cfg argument governs security limits and number handling in both.
//
// Example:
//
//	cfg := json.SecurityConfig()
//	valid, err := json.ValidWithConfig(jsonStr, cfg)
//	if err != nil {
//	    // inspect the validation failure
//	}
func ValidWithConfig(jsonStr string, cfg ...Config) (bool, error) {
	return withProcessor(func(p *Processor) (bool, error) {
		return p.Valid(jsonStr, cfg...)
	})
}

// ValidateSchema validates JSON data against a schema
func ValidateSchema(jsonStr string, schema *Schema, cfg ...Config) ([]ValidationError, error) {
	return withProcessor(func(p *Processor) ([]ValidationError, error) {
		return p.ValidateSchema(jsonStr, schema, cfg...)
	})
}

// LoadFromFile loads JSON data from a file with optional configuration
// Uses the default processor with support for Config such as security validation
func LoadFromFile(filePath string, cfg ...Config) (string, error) {
	return withProcessor(func(p *Processor) (string, error) {
		return p.LoadFromFile(filePath, cfg...)
	})
}

// LoadFromReader loads JSON data from an io.Reader with size limiting.
// The reader is limited to MaxJSONSize to prevent excessive memory usage.
//
// Example:
//
//	file, _ := os.Open("data.json")
//	defer file.Close()
//	jsonStr, err := json.LoadFromReader(file)
func LoadFromReader(reader io.Reader, cfg ...Config) (string, error) {
	return withProcessor(func(p *Processor) (string, error) {
		return p.LoadFromReader(reader, cfg...)
	})
}

// UnmarshalFromFile reads JSON from a file and unmarshals it into v.
// This is a convenience function that combines file reading and unmarshalling.
// Uses the default processor for security validation and decoding.
//
// Parameters:
//   - path: file path to read JSON from
//   - v: pointer to the target variable where JSON will be unmarshaled
//   - cfg: optional Config for security validation and processing
//
// Returns error if file reading fails or JSON cannot be unmarshaled.
func UnmarshalFromFile(filePath string, v any, cfg ...Config) error {
	return withProcessorError(func(p *Processor) error {
		return p.UnmarshalFromFile(filePath, v, cfg...)
	})
}

// ProcessBatch processes multiple JSON operations in a single batch.
// This is more efficient than processing each operation individually.
func ProcessBatch(operations []BatchOperation, cfg ...Config) ([]BatchResult, error) {
	return withProcessor(func(p *Processor) ([]BatchResult, error) {
		return p.ProcessBatch(operations, cfg...)
	})
}

// WarmupCache pre-warms the cache for frequently accessed paths.
// This can improve performance for subsequent operations on the same JSON.
func WarmupCache(jsonStr string, paths []string, cfg ...Config) (*WarmupResult, error) {
	return withProcessor(func(p *Processor) (*WarmupResult, error) {
		return p.WarmupCache(jsonStr, paths, cfg...)
	})
}

// ClearCache clears the processor's internal cache.
func ClearCache() {
	p := getDefaultProcessor()
	if p != nil {
		p.ClearCache()
	}
}

// GetStats returns statistics about the default processor.
func GetStats() Stats {
	p := getDefaultProcessor()
	if p == nil {
		return Stats{}
	}
	return p.GetStats()
}

// GetHealthStatus returns the health status of the default processor.
func GetHealthStatus() HealthStatus {
	p := getDefaultProcessor()
	if p == nil {
		return HealthStatus{Healthy: false}
	}
	return p.GetHealthStatus()
}

// =============================================================================
// Unified API - Use these functions for common scenarios
// =============================================================================

// ParseAny parses a JSON string and returns the root value as any.
// This is the unified name matching Processor.ParseAny().
//
// For unmarshaling into a specific target type, use Parse() instead.
//
// Example:
//
//	// Parse to any (uses default processor)
//	data, err := json.ParseAny(jsonStr)
//
//	// With configuration (uses config-cached processor)
//	cfg := json.SecurityConfig()
//	data, err := json.ParseAny(jsonStr, cfg)
func ParseAny(jsonStr string, cfg ...Config) (any, error) {
	return withProcessor(func(p *Processor) (any, error) {
		return p.ParseAny(jsonStr, cfg...)
	})
}

// Parse parses a JSON string into the target variable.
// This is the unified package-level method matching Processor.Parse().
//
// target must be a non-nil pointer. For parsing to any, use ParseAny() instead.
//
// Example:
//
//	// Parse into a map
//	var obj map[string]any
//	err := json.Parse(jsonStr, &obj)
//
//	// Parse into a struct
//	var user User
//	err := json.Parse(jsonStr, &user)
//
//	// With configuration
//	cfg := json.DefaultConfig()
//	cfg.PreserveNumbers = true
//	err := json.Parse(jsonStr, &data, cfg)
func Parse(jsonStr string, target any, cfg ...Config) error {
	return withProcessorError(func(p *Processor) error {
		return p.Parse(jsonStr, target, cfg...)
	})
}

// SaveToFile saves JSON data to a file with optional configuration.
// This is the unified API that replaces SaveToFileWithOpts.
//
// Example:
//
//	// Simple save
//	err := json.SaveToFile("data.json", data)
//
//	// With pretty printing
//	cfg := json.PrettyConfig()
//	err := json.SaveToFile("data.json", data, cfg)
func SaveToFile(filePath string, data any, cfg ...Config) error {
	return withProcessorError(func(p *Processor) error {
		return p.SaveToFile(filePath, data, cfg...)
	})
}

// MarshalToFile marshals data to JSON and writes to a file.
// This is the unified API that replaces MarshalToFileWithOpts.
//
// Example:
//
//	err := json.MarshalToFile("data.json", myStruct, json.PrettyConfig())
func MarshalToFile(filePath string, data any, cfg ...Config) error {
	return withProcessorError(func(p *Processor) error {
		return p.MarshalToFile(filePath, data, cfg...)
	})
}

// SaveToWriter writes JSON data to an io.Writer.
// This is the unified API that replaces SaveToWriterWithOpts.
//
// Example:
//
//	var buf bytes.Buffer
//	err := json.SaveToWriter(&buf, data, json.PrettyConfig())
func SaveToWriter(writer io.Writer, data any, cfg ...Config) error {
	return withProcessorError(func(p *Processor) error {
		return p.SaveToWriter(writer, data, cfg...)
	})
}

// EncodeBatch encodes multiple key-value pairs as a JSON object.
// This is the unified API that replaces EncodeBatchWithOpts.
//
// Example:
//
//	result, err := json.EncodeBatch(map[string]any{"name": "Alice", "age": 30})
func EncodeBatch(pairs map[string]any, cfg ...Config) (string, error) {
	return withProcessor(func(p *Processor) (string, error) {
		return p.EncodeBatch(pairs, cfg...)
	})
}

// EncodeFields encodes specific fields from a struct or map.
// This is the unified API that replaces EncodeFieldsWithOpts.
//
// Example:
//
//	result, err := json.EncodeFields(user, []string{"name", "email"})
func EncodeFields(value any, fields []string, cfg ...Config) (string, error) {
	return withProcessor(func(p *Processor) (string, error) {
		return p.EncodeFields(value, fields, cfg...)
	})
}

// EncodeStream encodes multiple values as a JSON array.
// This is the unified API that replaces EncodeStreamWithOpts.
//
// Example:
//
//	result, err := json.EncodeStream([]any{1, 2, 3}, json.PrettyConfig())
func EncodeStream(values any, cfg ...Config) (string, error) {
	return withProcessor(func(p *Processor) (string, error) {
		return p.EncodeStream(values, cfg...)
	})
}

// asyncCloseSem limits concurrent async close goroutines to prevent unbounded
// goroutine growth when many stale processors are replaced simultaneously.
var asyncCloseSem = make(chan struct{}, maxConcurrentCloses)

// asyncCloseProcessor closes a processor asynchronously with bounded concurrency.
//
// Close() is internally bounded by waitForActiveOps(closeOperationTimeout), so a
// synchronous call here cannot hang indefinitely. The previous design spawned a
// second goroutine to run Close() and raced it against an inner time.After: when
// the timeout fired, the outer goroutine released the semaphore and exited while
// the inner Close() goroutine was orphaned with no one to join it — an unbounded
// goroutine leak under cache-churn eviction (maybeEvictConfigCache calls this on
// every pressure event). Collapsing to a single synchronous Close() removes the
// leak and the redundant double-timeout.
func asyncCloseProcessor(p *Processor) {
	go func() {
		// SAFETY (SEC-003): this goroutine runs library cleanup as a side effect of
		// public cache-eviction calls; a panic in Close() must not crash the caller.
		// Registered first so the semaphore-release defer still runs on panic (LIFO).
		defer func() {
			if r := recover(); r != nil {
				fmt.Fprintf(os.Stderr, "json: warning: async processor close panicked: %v\n", r)
			}
		}()
		// Non-blocking acquire: never stall the eviction caller. getProcessorWithConfig
		// (and thus maybeEvictConfigCache) runs on the user's goroutine, so a blocking
		// send here — under cache-churn eviction with maxConcurrentCloses already in
		// flight, each held for up to closeOperationTimeout — would block a user calling
		// a package-level function with a fresh config. When the close-concurrency budget
		// is full we run Close() without a slot rather than block: Close() is internally
		// bounded by closeOperationTimeout, so this goroutine still cannot hang. The
		// stale processor is already detached from the cache, so the worst case under
		// extreme churn is a few extra transient goroutines, each terminating within the
		// close timeout.
		select {
		case asyncCloseSem <- struct{}{}:
			defer func() { <-asyncCloseSem }()
		default:
		}
		_ = p.Close() // best-effort; bounded internally by closeOperationTimeout
	}()
}

// getProcessorWithConfig returns a processor configured with the given config.
// Uses caching for identical configurations to improve performance.
func getProcessorWithConfig(cfg Config) (*Processor, error) {
	// The cache key (hashConfig) cannot distinguish two different CustomPathParser
	// functions — the field contributes a single "is set" bit (see configFieldList).
	// A cached processor built with parser A would silently serve every call made
	// with parser B, so configs carrying a custom parser are never served from the
	// registry. (Hooks/CustomEncoder/CustomValidators have the same instance-vs-type
	// collision, which is documented as an accepted limitation on those entries.)
	if cfg.CustomPathParser != nil {
		return New(cfg)
	}

	// Compute cache key from config
	cacheKey := hashConfig(cfg)

	// Fast path: return a live cached processor if present. A stale entry (closed
	// or wrong type) is best-effort removed via CompareAndDelete, which deletes the
	// slot only if it still holds that same stale value — so we never drop a valid
	// processor another goroutine just stored, and the load-check-delete is atomic
	// (no TOCTOU window between Load and Delete). On a miss — including the rare
	// race where a concurrent store lands right after our delete — we fall through
	// to the slow path, whose LoadOrStore resolves the winner correctly (closing
	// the loser).
	if cached, ok := configProcessorCache.Load(cacheKey); ok {
		if p, ok := cached.(*Processor); ok && !p.IsClosed() {
			return p, nil
		}
		configProcessorCache.CompareAndDelete(cacheKey, cached)
	}

	// Slow path: create new processor
	p, err := New(cfg)
	if err != nil {
		return nil, err
	}

	// Try to store in cache with retry for stale entries
	for range 3 {
		if existing, loaded := configProcessorCache.LoadOrStore(cacheKey, p); loaded {
			// Another goroutine stored first
			if ep, ok := existing.(*Processor); ok && !ep.IsClosed() {
				// Theirs is valid, close ours and use theirs
				_ = p.Close() // best-effort cleanup; error ignored as we're returning a valid processor
				return ep, nil
			}
			// Existing entry is stale; try to replace it atomically
			if configProcessorCache.CompareAndSwap(cacheKey, existing, p) {
				// Successfully replaced stale entry
				// Close the old stale processor asynchronously with timeout
				if staleProc, ok := existing.(*Processor); ok {
					asyncCloseProcessor(staleProc)
				}
				// Check cache size and evict if necessary, protecting the key
				// we just stored (see maybeEvictConfigCache)
				maybeEvictConfigCache(cacheKey)
				return p, nil
			}
			// CAS failed - close our processor and create a fresh one for retry
			_ = p.Close() // best-effort cleanup; orphaned processor on CAS failure
			p, err = New(cfg)
			if err != nil {
				return nil, err
			}
			continue
		}
		// Successfully stored new entry
		// Check cache size and evict if necessary, protecting the key we just
		// stored (see maybeEvictConfigCache)
		maybeEvictConfigCache(cacheKey)
		return p, nil
	}

	// All attempts exhausted under heavy contention. The processor cache is only
	// a performance optimization — it must never fail the caller's operation.
	// Return the freshly-built processor uncached instead of erroring out. It is
	// GC-collected once the caller drops the reference (its cache cleanup
	// goroutines are transient), so this does not leak under the extreme churn
	// required to actually reach this branch.
	return p, nil
}

// maybeEvictConfigCache checks if the cache exceeds the size limit and evicts if needed.
// Uses a mutex to serialize eviction; counts entries via Range to avoid counter drift.
// RACE SAFETY: Deletes from cache BEFORE closing to minimize the window where another
// goroutine could retrieve a processor being closed. Closes asynchronously to avoid
// blocking eviction on Close() timeout (5s). Any goroutine that retrieves a processor
// between our delete and their new creation will get a fresh processor, which is safe.
// GOROUTINE FIX: Uses buffered channel as semaphore to limit concurrent close goroutines
// and prevent unbounded goroutine growth.
// DETERMINISM FIX: Uses hash-based eviction order instead of random map iteration
// to ensure consistent behavior across runs.
//
// protect is the cache key the CALLER is about to return a processor for: it
// is excluded from eviction candidates. Without this, the freshly stored key
// ranked among all entries and, with the cache at its limit, had a
// ~evictCount/limit chance of being evicted-and-closed by its own store —
// handing the caller a processor that asyncCloseProcessor was concurrently
// tearing down, producing spurious ErrProcessorClosed failures (D-002).
func maybeEvictConfigCache(protect uint64) {
	configProcessorCacheMu.Lock()

	var count int
	configProcessorCache.Range(func(_, _ any) bool {
		count++
		return true
	})

	if count < configProcessorCacheLimit {
		configProcessorCacheMu.Unlock()
		return
	}

	var keysToDelete []uint64
	var validEntries []struct {
		key  uint64
		proc *Processor
	}

	// Scan and categorize processors
	configProcessorCache.Range(func(key, value any) bool {
		cacheKey, keyOk := key.(uint64)
		if !keyOk {
			return true // skip invalid cache key type
		}
		// Never evict the caller's freshly stored key (see func comment).
		if cacheKey == protect {
			return true
		}
		if p, ok := value.(*Processor); ok {
			if p.IsClosed() {
				keysToDelete = append(keysToDelete, cacheKey)
			} else {
				validEntries = append(validEntries, struct {
					key  uint64
					proc *Processor
				}{cacheKey, p})
			}
		} else {
			keysToDelete = append(keysToDelete, cacheKey)
		}
		return true
	})

	// Delete closed/invalid processors first
	for _, key := range keysToDelete {
		configProcessorCache.Delete(key)
	}

	// If still over limit, evict entries using deterministic hash-based order
	// This ensures consistent eviction behavior across runs
	var toClose []*Processor
	if len(validEntries) >= configProcessorCacheLimit {
		// Sort by key hash to get deterministic eviction order
		// Keys with lower hash values are evicted first
		slices.SortFunc(validEntries, func(a, b struct {
			key  uint64
			proc *Processor
		}) int {
			if a.key < b.key {
				return -1
			}
			if a.key > b.key {
				return 1
			}
			return 0
		})

		evictCount := min(configProcessorCacheEvictNum, len(validEntries))

		for i := range evictCount {
			configProcessorCache.Delete(validEntries[i].key)
			toClose = append(toClose, validEntries[i].proc)
		}
	}

	// Release the cache mutex BEFORE the async close sends on asyncCloseSem.
	// Each asyncCloseProcessor blocks acquiring a slot on the 8-slot
	// asyncCloseSem; holding configProcessorCacheMu across those sends would
	// stall every getProcessorWithConfig slow-path (and thus every
	// config-based package-level call) for up to ~5s when the semaphore is
	// saturated under churn.
	configProcessorCacheMu.Unlock()

	// Close evicted processors with bounded concurrency
	for _, proc := range toClose {
		asyncCloseProcessor(proc)
	}
}
