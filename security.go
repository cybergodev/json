package json

import (
	"cmp"
	"fmt"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode/utf8"

	"github.com/cybergodev/json/internal"
	"golang.org/x/text/unicode/norm"
)

// Security validation thresholds - named constants for clarity
const (
	// securitySmallJSONThreshold is the size threshold for full security scanning (4KB)
	// JSON strings smaller than this are always fully scanned
	securitySmallJSONThreshold = 4096

	// securityScanWindowSize is the window size for rolling security scans (32KB)
	// Fits well in CPU cache for efficient scanning
	securityScanWindowSize = 32768

	// securitySampleSize is the size of samples taken from different regions (4KB)
	// Used for suspicious character density checks
	securitySampleSize = 4096

	// securityNestingValidationThreshold is the size threshold for detailed nesting validation (64KB)
	// Smaller JSON relies on standard library's built-in validation
	securityNestingValidationThreshold = 65536

	// securityMaxTotalBrackets is the maximum total bracket count allowed (1 million)
	// Prevents DoS attacks with massive bracket structures
	securityMaxTotalBrackets = 1000000

	// securityMaxConsecutiveOpens is the maximum consecutive opening brackets (100)
	// Detects anomaly patterns that could indicate attacks
	securityMaxConsecutiveOpens = 100

	// securityCacheHighWatermark is the entry-count high watermark for LRU
	// eviction of the validation cache. Inserting at/above it evicts the oldest
	// 25% first, which also bounds the cache at ~8000 entries.
	securityCacheHighWatermark = 8000

	// securityLocalDensityThreshold is the maximum allowed suspicious character density in sample regions (0.5%)
	// Used for beginning, middle, and end samples of JSON strings
	securityLocalDensityThreshold = 0.005

	// securityOverallDensityThreshold is the maximum allowed suspicious character density across entire string (0.3%)
	// Lower threshold catches attacks that spread malicious content thinly
	securityOverallDensityThreshold = 0.003
)

// dangerousPattern represents a single dangerous pattern for security validation
type dangerousPattern struct {
	pattern string
	name    string
}

// dangerousPatterns contains all dangerous patterns for security validation
// This is defined at package level to avoid allocation on each validation call
var dangerousPatterns = []dangerousPattern{
	// Critical patterns (always checked first)
	{"__proto__", "prototype pollution"},
	{"constructor[", "constructor access"},
	{"prototype.", "prototype manipulation"},
	// HTML/XML injection patterns
	{"<script", "script tag injection"},
	{"<iframe", "iframe injection"},
	{"<object", "object injection"},
	{"<embed", "embed injection"},
	{"<svg", "svg injection"},
	// Protocol patterns
	{"javascript:", "javascript protocol"},
	{"vbscript:", "vbscript protocol"},
	// Code execution patterns
	{"eval(", "dynamic code execution"},
	{"setTimeout(", "timer manipulation"},
	{"setInterval(", "interval manipulation"},
	{"require(", "code injection"},
	{"new function(", "dynamic function creation"},
	// DOM access patterns
	{"document.cookie", "cookie access"},
	{"window.location", "redirect manipulation"},
	{"innerhtml", "DOM manipulation"},
	// Encoding bypass patterns
	{"fromcharcode(", "character encoding bypass"},
	{"atob(", "base64 decoding"},
	{"expression(", "CSS expression injection"},
	// Event handlers (common injection vectors)
	{"onerror", "event handler injection"},
	{"onload", "event handler injection"},
	{"onclick", "event handler injection"},
	{"onmouseover", "event handler injection"},
	{"onfocus", "event handler injection"},
	// Prototype pollution patterns
	{"__defineGetter__", "getter definition"},
	{"__defineSetter__", "setter definition"},
}

// criticalPatterns are always fully scanned regardless of JSON size
// These patterns are too dangerous to miss due to sampling
var criticalPatterns = []dangerousPattern{
	{"__proto__", "prototype pollution"},
	{"constructor[", "constructor access"},
	{"prototype.", "prototype manipulation"},
}

// prefilteredPattern pairs a dangerous pattern with its case-folded second
// byte so the single-pass scanner can reject first-byte coincidences with a
// single compare before paying for the slice construction and the full
// case-insensitive comparison. patternIndex maps back into dangerousPatterns
// so the recording mode of scanWindowPatterns can file matches by pattern
// (P-003).
type prefilteredPattern struct {
	pattern      string
	secondByte   byte // case-folded pattern[1]
	patternIndex int  // index into dangerousPatterns
}

// dangerousPatternGroups buckets dangerousPatterns by their case-folded first
// byte. Computed once at init: dangerousPatterns is never mutated after
// declaration. Drives scanWindowPatterns' single pass.
var dangerousPatternGroups [256][]prefilteredPattern

func init() {
	for i, dp := range dangerousPatterns {
		// Patterns shorter than 2 bytes cannot use the second-byte check and
		// have no slot in this prefilter (none exist in the built-in set).
		if len(dp.pattern) < 2 {
			continue
		}
		c := internal.FoldLowerASCII(dp.pattern[0])
		dangerousPatternGroups[c] = append(dangerousPatternGroups[c], prefilteredPattern{
			pattern:      dp.pattern,
			secondByte:   internal.FoldLowerASCII(dp.pattern[1]),
			patternIndex: i,
		})
	}
	if len(sensitivePatterns) > 0 {
		minSensitivePatternLen = len(sensitivePatterns[0])
		for _, p := range sensitivePatterns[1:] {
			if len(p) < minSensitivePatternLen {
				minSensitivePatternLen = len(p)
			}
		}
	}
	// P-003: first-byte buckets for the sensitive-pattern single-pass scan.
	// Patterns shorter than 2 bytes cannot use the second-byte check and have
	// no slot here (none exist in the built-in set; the shortest is 3).
	for _, p := range sensitivePatterns {
		if len(p) < 2 {
			continue
		}
		sensitivePatternGroups[p[0]] = append(sensitivePatternGroups[p[0]], sensitivePrefiltered{
			pattern:    p,
			secondByte: p[1],
		})
	}
}

// scanWindowPatterns is the single-pass built-in-pattern scanner behind
// scanWindowForPatterns, in two modes:
//
//   - first == nil: pure EXISTENCE check that returns true at the first hit
//     (the shape of the former windowContainsDangerousMatch prefilter);
//   - first != nil (len == len(dangerousPatterns), every entry initialized
//     to -1): scans the whole window recording, for each pattern, the index
//     of its FIRST case-insensitive occurrence, and reports whether any
//     pattern occurred at all.
//
// Recording the first occurrence per pattern is equivalent to calling
// fastIndexIgnoreCase once per pattern: the group bucket (folded first byte),
// the inline second-byte check, and IsMatchPatternIgnoreCase together accept
// exactly the positions a case-insensitive substring search would report,
// and ascending iteration keeps the smallest one. The ordered reporting loop
// in scanWindowForPatterns then fires from those positions — same pattern
// order, same word-context checks — while scanning the window once instead of
// once per pattern (P-003; profiling attributed ~41% of cold-validation CPU
// to those rescans). GEN-001 P0-3: the reporting loop now context-checks
// every occurrence from the recorded first one (indexInDangerousContext), so
// the error differs from the pre-P-003 shape in exactly one case — a benign
// word-internal first occurrence no longer shields a later standalone one.
//
// P-001: common letters ('e', 'o', 's', ...) head several patterns each, so
// most candidates die inside IsMatchPatternIgnoreCase on the second byte.
// The inline second-byte compare rejects them without the call and slice;
// candidates surviving both bytes fall through to the exact comparison, so
// the accepted set is exact.
//
// Custom and globally-registered patterns are NOT covered here; they keep
// their own per-window scan.
func scanWindowPatterns(window string, first []int32) bool {
	found := false
	for i := 0; i < len(window); i++ {
		group := dangerousPatternGroups[internal.FoldLowerASCII(window[i])]
		for j := range group {
			pp := &group[j]
			end := i + len(pp.pattern)
			if end > len(window) {
				continue
			}
			if internal.FoldLowerASCII(window[i+1]) != pp.secondByte {
				continue
			}
			if internal.IsMatchPatternIgnoreCase(window[i:end], pp.pattern) {
				if first == nil {
					return true
				}
				if first[pp.patternIndex] < 0 {
					first[pp.patternIndex] = int32(i)
					found = true
				}
			}
		}
	}
	return found
}

// =============================================================================
// Global Pattern Registry
// =============================================================================

// globalPatternRegistry provides thread-safe registration of dangerous patterns.
// Patterns registered here are used in addition to the default patterns.
var globalPatternRegistry = &patternRegistry{
	patterns: make(map[string]DangerousPattern),
}

// patternRegistry manages dangerous patterns with thread-safe operations.
type patternRegistry struct {
	mu       sync.RWMutex
	patterns map[string]DangerousPattern
}

// Add registers a new dangerous pattern.
func (r *patternRegistry) Add(pattern DangerousPattern) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.patterns[pattern.Pattern] = pattern
	// P-002 RACE FIX: invalidate cachedMaxPatternLen while holding the write
	// lock. recomputeMaxPatternLen reads the registry and publishes its result
	// under RLock, so the lock pair prevents recompute from overwriting a newer
	// invalidation with a stale length (last-writer-wins race that briefly left
	// the rolling-window overlap too small for globally-registered patterns).
	atomic.StoreInt64(&cachedMaxPatternLen, 0)
}

// Remove unregisters a pattern by its pattern string.
func (r *patternRegistry) Remove(pattern string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.patterns, pattern)
	atomic.StoreInt64(&cachedMaxPatternLen, 0) // invalidate under the write lock (see Add)
}

// List returns all registered patterns.
func (r *patternRegistry) List() []DangerousPattern {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]DangerousPattern, 0, len(r.patterns))
	for _, p := range r.patterns {
		result = append(result, p)
	}
	return result
}

// ListByLevel was removed in the D-002 cleanup: it had no callers in
// production or tests.

// Clear removes all registered patterns.
// Len returns the number of registered patterns. Used by the security-scan
// shortcut gates: a non-empty registry must disable them, or custom/global
// patterns composed of non-indicator bytes are silently never scanned.
func (r *patternRegistry) Len() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.patterns)
}

// Clear removes all registered patterns.
func (r *patternRegistry) Clear() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.patterns = make(map[string]DangerousPattern)
	atomic.StoreInt64(&cachedMaxPatternLen, 0) // invalidate under the write lock (see Add)
}

// RegisterDangerousPattern adds a pattern to the global registry.
// Patterns registered here are checked in addition to default patterns.
//
// Example:
//
//	json.RegisterDangerousPattern(json.DangerousPattern{
//	    Pattern: "malicious_keyword",
//	    Name:    "Custom dangerous pattern",
//	    Level:   json.PatternLevelCritical,
//	})
//
// RegisterDangerousPattern adds a pattern to the global registry.
//
// LIMITATION (D-002): patterns at least securityScanWindowSize (32KB) long
// are scanned with a byte-stride window sweep; keep patterns well below that
// length — dangerous-pattern signatures are short substrings by nature.
func RegisterDangerousPattern(pattern DangerousPattern) {
	globalPatternRegistry.Add(pattern)
}

// UnregisterDangerousPattern removes a pattern from the global registry.
func UnregisterDangerousPattern(pattern string) {
	globalPatternRegistry.Remove(pattern)
}

// ListDangerousPatterns returns all registered custom patterns.
func ListDangerousPatterns() []DangerousPattern {
	return globalPatternRegistry.List()
}

// clearDangerousPatterns, getDefaultPatterns, and getCriticalPatterns were
// moved to security_test.go in the D-002 cleanup: they had no production
// callers and exist only as test conveniences.

// indicatorChars is a pre-computed lookup table for indicator characters.
// PERFORMANCE: O(1) lookup per character during security scanning.
//
// INVARIANT (GEN-001): pattern matching is case-insensitive (fastIndexIgnoreCase),
// so for every letter appearing in a built-in pattern, BOTH its lowercase and
// uppercase forms must be indicator bytes. Otherwise an attacker can render a
// pattern entirely in the non-indicator casing, producing an input with zero
// indicator bytes that the scan-skip shortcut in validateJSONSecurityOptimized
// would silently never scan. The lowercase set covers the built-in pattern
// alphabet; the uppercase set is its exact case-symmetric counterpart
// (A B C D E F I J N O P R S V W). TestIndicatorCaseInvariant enforces this
// over dangerousPatterns/criticalPatterns so a future pattern cannot
// reintroduce the gap. Custom patterns are covered separately: any configured
// pattern (per-call or global) disables the shortcut entirely.
var indicatorChars = [256]bool{
	'<': true, '(': true, ':': true, '.': true, '_': true,
	'A': true, 'B': true, 'C': true, 'D': true, 'E': true, 'F': true,
	'I': true, 'J': true, 'N': true, 'O': true, 'P': true, 'R': true,
	'S': true, 'V': true, 'W': true,
	'a': true, 'b': true, 'c': true, 'd': true, 'e': true, 'f': true,
	'i': true, 'j': true, 'n': true, 'o': true, 'p': true, 'r': true,
	's': true, 'v': true, 'w': true,
}

// maxDangerousPatternLen returns the length of the longest dangerous pattern.
// PERFORMANCE: Cached with atomic for lock-free reads; invalidated on pattern registration.
func maxDangerousPatternLen() int {
	cached := atomic.LoadInt64(&cachedMaxPatternLen)
	if cached > 0 {
		return int(cached)
	}
	return recomputeMaxPatternLen()
}

// cachedMaxPatternLen caches the result of maxDangerousPatternLen
var cachedMaxPatternLen int64

// recomputeMaxPatternLen recalculates and caches the max pattern length.
//
// P-002 RACE FIX: the registry read and the result Store happen under the same
// RLock, and Add/Remove/Clear invalidate under the write lock. RLock excludes
// the write lock, so a concurrent registration cannot interleave between this
// read and this Store — the previous unlocked form let recompute overwrite a
// newer invalidation with a stale (smaller) length, shrinking the rolling-window
// overlap until the next registration event.
func recomputeMaxPatternLen() int {
	maxLen := 0
	for _, dp := range dangerousPatterns {
		if len(dp.pattern) > maxLen {
			maxLen = len(dp.pattern)
		}
	}
	globalPatternRegistry.mu.RLock()
	for _, p := range globalPatternRegistry.patterns {
		if len(p.Pattern) > maxLen {
			maxLen = len(p.Pattern)
		}
	}
	atomic.StoreInt64(&cachedMaxPatternLen, int64(maxLen))
	globalPatternRegistry.mu.RUnlock()
	return maxLen
}

// sensitivePatterns contains patterns for detecting sensitive data in cache values
// PERFORMANCE: Defined at package level to avoid allocation on each containsSensitivePatterns() call
var sensitivePatterns = []string{
	// Authentication and authorization
	"password", "passwd", "pwd",
	"token", "bearer", "jwt", "access_token", "refresh_token", "auth_token",
	"secret", "secret_key", "client_secret",
	"apikey", "api_key", "api-key", "x-api-key",
	"auth", "authorization", "authenticate",
	"credential", "credentials",
	"private",

	// Personal Identifiable Information (PII)
	"ssn", "social_security", "social_security_number",
	"credit_card", "creditcard", "card_number", "cvv", "cvc",
	"passport", "passport_number",
	"driver_license", "license_number",

	// Financial sensitive data
	"account_number", "bank_account", "routing_number",
	"pin", "pin_number",

	// Cryptographic keys
	"private_key", "public_key", "encryption_key", "signing_key",
	"certificate", "private_certificate",

	// Session and cookies
	"session", "session_id", "session_key",
	"cookie", "csrf", "xsrf",

	// Database and infrastructure
	"database_url", "db_password", "db_user", "db_pass",
	"connection_string", "connectionstring",

	// Cloud provider keys
	"aws_access_key", "aws_secret", "aws_key",
	"azure_key", "gcp_key", "gcp_credentials",
}

// minSensitivePatternLen is the length of the shortest sensitivePattern,
// computed at init so containsSensitivePatterns can skip strings too short to
// contain any pattern without duplicating the list's minimum here.
var minSensitivePatternLen int

// sensitivePrefiltered pairs a sensitive pattern with its second byte for the
// first-byte-bucketed single-pass scan in containsSensitivePatterns (P-003).
type sensitivePrefiltered struct {
	pattern    string
	secondByte byte // pattern[1]; patterns are lowercase ASCII
}

// sensitivePatternGroups buckets sensitivePatterns by first byte, the same
// prefilter shape as dangerousPatternGroups. Patterns are all lowercase ASCII
// (an uppercase pattern could never match the lowercased input under the
// previous per-pattern Contains loop either), and containsSensitivePatterns
// lowercases its input before scanning, so bucket lookups need no case
// folding. Computed once at init: sensitivePatterns is never mutated.
var sensitivePatternGroups [256][]sensitivePrefiltered

// =============================================================================
// Security Validator Components
// These types separate concerns for better maintainability and testability
// =============================================================================

// validationCacheEntry holds a cache entry with access time for LRU eviction
// SECURITY FIX: Track access time for better cache management
type validationCacheEntry struct {
	validated  bool
	lastAccess int64  // Unix timestamp for LRU eviction
	input      string // exact validated input; compared on hit to defeat hash collisions
}

// validationKey is the map key for the validation cache. It is a fixed-size,
// comparable value (length + one FNV-1a hash), so constructing it allocates
// nothing — unlike the previous "len:h1:h2" string key, which allocated ~50
// bytes on every operation.
type validationKey struct {
	length int
	h1     uint64
}

// securityValidator provides comprehensive security validation for JSON processing.
type securityValidator struct {
	maxJSONSize     int64
	maxPathLength   int
	maxNestingDepth int
	// maxObjectKeys / maxArrayElements cap the number of direct children of any
	// single object or array. <=0 means unlimited. Enforced by
	// validateContainerCounts to stop flat-but-wide structures (e.g. an object
	// with millions of keys at depth 1) that bypass the nesting-depth and
	// total-bracket checks. SECURITY: protects against memory-exhaustion DoS.
	maxObjectKeys          int
	maxArrayElements       int
	fullSecurityScan       bool
	disableDefaultPatterns bool
	// additionalPatterns are config-supplied dangerous patterns (converted from
	// Config.AdditionalDangerousPatterns). They are scanned in addition to the
	// built-in defaults and remain in effect even when disableDefaultPatterns
	// is set, since they are explicitly opted in by the caller. Globally
	// registered patterns (globalPatternRegistry) are scanned live as well.
	additionalPatterns []dangerousPattern
	// maxCustomPatternLen is the length of the longest additional pattern.
	// The rolling-window overlap must cover it: a custom pattern longer than
	// the built-in overlap could otherwise straddle a scan-window boundary,
	// be contained in no window, and evade detection entirely (regression
	// test: TestD002Round6_CustomPatternStraddlesWindowBoundary).
	maxCustomPatternLen int
	// detectDuplicateKeys enables duplicate-object-key detection in
	// validateContainerCounts (Config.DetectDuplicateKeys, GEN-001). Opt-in:
	// the default preserves encoding/json last-wins semantics. Rejection
	// surfaces as ErrDuplicateKey.
	detectDuplicateKeys bool
	// Composed validators for separation of concerns
	// Cache for validation results — created lazily on the first successfully
	// validated input (P-001) so transient one-shot validators and validators
	// that never cache (cacheDisabled) do not allocate the map.
	validationCache map[validationKey]*validationCacheEntry
	// evictBuf is the reusable candidate buffer for evictLRUEntries. Only
	// read/written while holding cacheMutex (see evictLRUEntries).
	evictBuf []evictEntry
	// cachedBytes is the sum of len(entry.input) across validationCache
	// entries — the caller-string memory pinned by the collision-defense
	// inputs. Guarded by cacheMutex (like the map) and bounded by
	// validationCacheBytesBudget via evictLRUEntries (P-003).
	cachedBytes int
	// cacheDisabled permanently disables caching: set by Close() and on
	// transient one-shot validators (validateInputForOptions' per-call cfg
	// path). Distinguished from a merely not-yet-created (nil) cache so lazy
	// creation cannot resurrect caching after Close().
	cacheDisabled bool
	cacheMutex    sync.RWMutex
}

// newSecurityValidator creates a new security validator with the given limits.
// additionalPatterns are config-supplied patterns scanned in addition to the
// built-in defaults (see Config.AdditionalDangerousPatterns). detectDuplicateKeys
// enables duplicate-object-key rejection (see Config.DetectDuplicateKeys).
func newSecurityValidator(maxJSONSize int64, maxPathLength, maxNestingDepth int, fullSecurityScan, disableDefaultPatterns, detectDuplicateKeys bool, additionalPatterns []dangerousPattern, maxObjectKeys, maxArrayElements int) *securityValidator {
	sv := &securityValidator{
		maxJSONSize:            maxJSONSize,
		maxPathLength:          maxPathLength,
		maxNestingDepth:        maxNestingDepth,
		maxObjectKeys:          maxObjectKeys,
		maxArrayElements:       maxArrayElements,
		fullSecurityScan:       fullSecurityScan,
		disableDefaultPatterns: disableDefaultPatterns,
		detectDuplicateKeys:    detectDuplicateKeys,
		additionalPatterns:     additionalPatterns,
	}
	for _, dp := range additionalPatterns {
		if len(dp.pattern) > sv.maxCustomPatternLen {
			sv.maxCustomPatternLen = len(dp.pattern)
		}
	}
	return sv
}

// toInternalPatterns converts public DangerousPattern values to the internal
// dangerousPattern representation used by the security validator. Returns nil
// for empty input so the validator stores no slice in the common case.
func toInternalPatterns(patterns []DangerousPattern) []dangerousPattern {
	if len(patterns) == 0 {
		return nil
	}
	result := make([]dangerousPattern, len(patterns))
	for i, p := range patterns {
		result[i] = dangerousPattern{pattern: p.Pattern, name: p.Name}
	}
	return result
}

// Close releases resources held by the security validator.
// This should be called when the validator is no longer needed to prevent memory leaks.
func (sv *securityValidator) Close() {
	sv.cacheMutex.Lock()
	defer sv.cacheMutex.Unlock()

	// Permanently disable caching and clear the cache to release memory.
	sv.cacheDisabled = true
	sv.validationCache = nil
	sv.cachedBytes = 0
}

// ValidateJSONInputEssential performs only essential safety checks that must
// always be enforced, even when SkipValidation is true.
// SECURITY: Size limits and nesting depth protect the process itself from DoS.
func (sv *securityValidator) ValidateJSONInputEssential(jsonStr string) error {
	// Always enforce size limit — prevents memory exhaustion
	if int64(len(jsonStr)) > sv.maxJSONSize {
		return newSizeLimitError("validate_json_input", int64(len(jsonStr)), sv.maxJSONSize)
	}

	if len(jsonStr) == 0 {
		return newOperationError("validate_json_input", "JSON string cannot be empty", ErrInvalidJSON)
	}

	// Always enforce nesting depth and per-container size in one pass — depth
	// prevents stack overflow during parsing, container caps prevent
	// memory-exhaustion DoS from flat-but-wide structures (e.g. an object with
	// millions of keys at depth 1) that bypass the depth/total-bracket checks.
	if err := sv.validateStructureLimits(jsonStr); err != nil {
		return err
	}

	return nil
}

// ValidateJSONInput performs comprehensive JSON input validation with enhanced security.
// PERFORMANCE: Uses caching to avoid repeated validation of the same JSON string.
func (sv *securityValidator) ValidateJSONInput(jsonStr string) error {
	return sv.validateJSONInputWithKey(jsonStr, sv.getValidationCacheKey(jsonStr))
}

// ValidateJSONInputPrehashed is ValidateJSONInput for callers that already
// computed the document's FNV-1a hash (P-001): Get needs the identical hash
// for its result-cache keys, so sharing it here saves one full scan of the
// input per operation. h1 MUST equal internal.HashStringFNV1a(jsonStr).
func (sv *securityValidator) ValidateJSONInputPrehashed(jsonStr string, h1 uint64) error {
	return sv.validateJSONInputWithKey(jsonStr, validationKey{length: len(jsonStr), h1: h1})
}

// validateJSONInputWithKey is the ValidateJSONInput body with a caller-supplied
// cache key (either freshly computed or shared from Get's prehash).
func (sv *securityValidator) validateJSONInputWithKey(jsonStr string, cacheKey validationKey) error {
	if int64(len(jsonStr)) > sv.maxJSONSize {
		return newSizeLimitError("validate_json_input", int64(len(jsonStr)), sv.maxJSONSize)
	}

	if len(jsonStr) == 0 {
		return newOperationError("validate_json_input", "JSON string cannot be empty", ErrInvalidJSON)
	}

	// PERFORMANCE: Check cache for previously validated JSON strings
	// This is especially effective for repeated Get operations on the same JSON
	// Skip all expensive validations for cached strings
	if sv.isValidationCachedWithKey(jsonStr, cacheKey) {
		return nil
	}

	// First time validation - do all checks
	if !utf8.ValidString(jsonStr) {
		return newOperationError("validate_json_input", "JSON contains invalid UTF-8 sequences", ErrInvalidJSON)
	}

	// Detect BOM (not allowed)
	cleanJSON := strings.TrimPrefix(jsonStr, validationBOMPrefix)
	if len(cleanJSON) != len(jsonStr) {
		return newOperationError("validate_json_input", "JSON contains BOM which is not allowed", ErrInvalidJSON)
	}

	// Do full security scan
	if err := sv.validateJSONSecurity(jsonStr); err != nil {
		return err
	}

	// Validate structure
	if err := sv.validateJSONStructure(jsonStr); err != nil {
		return err
	}

	// Validate nesting depth and per-container sizes (MaxObjectKeys /
	// MaxArrayElements) in one byte-level pass (P-003: the two scans shared
	// the same string/escape discipline, so walking the text once halves the
	// structural-scan cost; error selection is identical to the former
	// nesting-then-container sequence). Runs before caching so the result —
	// including these checks — is reused.
	if err := sv.validateStructureLimits(jsonStr); err != nil {
		return err
	}

	// Cache the successful validation - PERFORMANCE: Use pre-computed cache key
	sv.cacheValidationWithKey(cacheKey, jsonStr)

	return nil
}

// getValidationCacheKey computes and returns the cache key for a JSON string.
// PERFORMANCE: Uses FNV-1a (~2ns) instead of SHA-256 (~100ns) for ~50x speedup.
// The cache is internal-only (not exposed to callers), so FNV-1a's collision
// resistance is sufficient.
//
// SECURITY: h1 is the same hash value Processor cache keys use
// (internal.HashStringFNV1a, see hashStringToUint64). A single hash is enough
// here because a cache hit is NEVER trusted on the key alone —
// isValidationCachedWithKey compares the exact stored input before skipping
// any security check, so a collision merely causes one redundant revalidation,
// never a false "already validated". The former second independent hash
// (HashBytesFNV1aOffset) only reduced the frequency of that harmless event,
// at the cost of a second full scan of every input on every operation
// (~4% of Get CPU on large inputs; see P-001 pprof).
//
// Returning a fixed-size struct (rather than a formatted string) avoids a
// per-op heap allocation in the hot path.
func (sv *securityValidator) getValidationCacheKey(jsonStr string) validationKey {
	return validationKey{
		length: len(jsonStr),
		h1:     internal.HashStringFNV1a(jsonStr),
	}
}

// isValidationCachedWithKey checks if the input stored under cacheKey was
// previously validated successfully. The key is computed (or shared from Get's
// prehash, P-001) by the caller — see getValidationCacheKey.
// RACE-FIX: Access time is not updated in read lock to avoid data race.
// The LRU eviction still works correctly with occasional access time updates during Set operations.
func (sv *securityValidator) isValidationCachedWithKey(jsonStr string, cacheKey validationKey) bool {
	// Use read lock for fast lookup
	sv.cacheMutex.RLock()
	// SAFETY: Check for nil cache (can happen after Close())
	if sv.validationCache == nil {
		sv.cacheMutex.RUnlock()
		return false
	}
	entry, cached := sv.validationCache[cacheKey]
	// RACE-FIX: Do NOT update entry.lastAccess here with read lock
	// The access time will be updated when the entry is re-validated or during eviction
	sv.cacheMutex.RUnlock()

	if !cached || !entry.validated {
		return false
	}
	// SECURITY: the cache key is a non-cryptographic FNV hash pair and is therefore
	// collision-constructible. A hash collision must NOT be trusted as "already
	// validated" — compare the exact input before skipping any security checks.
	return entry.input == jsonStr
}

// cacheValidationWithKey marks a JSON string as successfully validated using a pre-computed key
// PERFORMANCE: Accepts pre-computed cache key to avoid double hash computation for large JSON
// SECURITY FIX: Uses LRU-style eviction at 80% capacity to prevent memory spikes
// SECURITY: Stores the exact validated input so isValidationCached can reject hash
// collisions instead of trusting a non-cryptographic FNV key as identity.
// validationCacheMaxInputSize caps the size of a SINGLE input retained in the
// validation cache. Inputs above it are not cached; the cost is revalidation
// on every use. Deliberately 1/16 of validationCacheBytesBudget so one large
// document cannot evict the rest of the cache.
//
// validationCacheBytesBudget caps the TOTAL bytes pinned by cached inputs.
// The cache holds each entry's exact input string as its collision defense,
// which pins that memory for the entry's lifetime: with count-only eviction
// (securityCacheHighWatermark), 8,000 distinct 1MB documents would pin ~8GB
// per Processor inside the security layer itself — a memory amplification
// the layer exists to prevent (D-002). The byte budget bounds that total
// regardless of input sizes; eviction frees oldest entries until it is
// respected (8,000 entries × 256KB also pinned ~2GB under the old cutoff, so
// the budget is a strictly tighter bound).
//
// P-003: the per-entry cap rose from 256KB so mid-sized documents (hundreds
// of KB to a few MB) reuse validation results. Under the old cutoff they
// re-ran the full security scan on EVERY operation, which dominated CPU for
// repeated Get/Set on such inputs (profiling on 300-690KB documents: ~74% of
// CPU in validation, 5x overall speedup once cached).
const (
	validationCacheMaxInputSize = 2 * 1024 * 1024
	validationCacheBytesBudget  = 32 * 1024 * 1024
)

func (sv *securityValidator) cacheValidationWithKey(cacheKey validationKey, jsonStr string) {
	// MEMORY BOUND (D-002): see validationCacheMaxInputSize. Checked before
	// taking the lock — oversized inputs never reach the map.
	if len(jsonStr) > validationCacheMaxInputSize {
		return
	}

	sv.cacheMutex.Lock()
	defer sv.cacheMutex.Unlock()

	// SAFETY: Skip caching after Close() and on transient one-shot validators.
	if sv.cacheDisabled {
		return
	}

	// Lazy cache creation (P-001): the map is allocated on the first input
	// that qualifies for caching rather than in the constructor, so validators
	// that never cache do not pay for it. nil here means "not yet created".
	if sv.validationCache == nil {
		sv.validationCache = make(map[validationKey]*validationCacheEntry, 256)
	}

	// SECURITY FIX: Proactive cleanup at 80% capacity instead of 100%.
	// Also evict when this insert would push pinned bytes past the budget —
	// the entry-count bound alone cannot cap memory when inputs are large
	// (see validationCacheBytesBudget, P-003).
	const cacheHighWatermark = securityCacheHighWatermark
	if len(sv.validationCache) >= cacheHighWatermark || sv.cachedBytes+len(jsonStr) > validationCacheBytesBudget {
		sv.evictLRUEntries()
	}

	// A replacement (hash collision storing a different input, or re-caching
	// after an entry-level invalidation) releases the old entry's bytes
	// before the new one is counted, so cachedBytes cannot drift upward.
	if old, ok := sv.validationCache[cacheKey]; ok {
		sv.cachedBytes -= len(old.input)
	}
	sv.cachedBytes += len(jsonStr)
	sv.validationCache[cacheKey] = &validationCacheEntry{
		validated:  true,
		lastAccess: time.Now().Unix(),
		input:      jsonStr,
	}
}

// evictEntry is one validation-cache eviction candidate: the map key plus the
// entry's lastAccess stamp, collected for age-order selection.
type evictEntry struct {
	key        validationKey
	lastAccess int64
}

// evictLRUEntries removes oldest entries using LRU strategy.
// SECURITY: Intelligent LRU eviction for validation cache
//
// One age-ordered pass serves both triggers (P-003):
//   - entry count at securityCacheHighWatermark: remove the oldest 25%
//     (batch removal reduces re-trigger thrashing — evicting one entry at a
//     time would re-qualify on nearly every insert);
//   - pinned bytes above validationCacheBytesBudget: keep removing the oldest
//     until the budget is respected (stopping earlier would re-qualify on
//     the next insert).
//
// PERFORMANCE (P-001): reuses sv.evictBuf across evictions. Past the 8000-entry
// high watermark the collect step allocated a ~128KB slice on every qualifying
// insert; the buffer is only touched while cacheMutex (held by the caller,
// cacheValidationWithKey) is locked, so reuse is race-free. sort.Slice is
// replaced by the reflection-free slices.SortFunc.
func (sv *securityValidator) evictLRUEntries() {
	n := len(sv.validationCache)
	if n == 0 {
		return
	}

	if cap(sv.evictBuf) < n {
		sv.evictBuf = make([]evictEntry, 0, n+n/4)
	}
	entries := sv.evictBuf[:0]
	for k, v := range sv.validationCache {
		entries = append(entries, evictEntry{key: k, lastAccess: v.lastAccess})
	}

	// Sort by access time (oldest first)
	slices.SortFunc(entries, func(a, b evictEntry) int {
		return cmp.Compare(a.lastAccess, b.lastAccess)
	})

	// Remove oldest 25% instead of 50% to reduce cache thrashing
	toRemove := max(len(entries)/4, 1)

	// Phase 1 removes the batch above unconditionally; phase 2 keeps removing
	// the oldest while pinned bytes still exceed the budget.
	for i := 0; i < len(entries); i++ {
		if i >= toRemove && sv.cachedBytes <= validationCacheBytesBudget {
			break
		}
		if v, ok := sv.validationCache[entries[i].key]; ok {
			sv.cachedBytes -= len(v.input)
			delete(sv.validationCache, entries[i].key)
		}
	}
	sv.evictBuf = entries[:0]
}

// ValidatePathInput performs comprehensive path validation with enhanced security.
// SECURITY: Combines security checks with syntax validation from internal package.
func (sv *securityValidator) ValidatePathInput(path string) error {
	// Early length check
	if len(path) > sv.maxPathLength {
		return newPathError(path, fmt.Sprintf("path length %d exceeds maximum %d", len(path), sv.maxPathLength), ErrInvalidPath)
	}

	// Empty path is valid (root access)
	if path == "" || path == "." {
		return nil
	}

	// Security validation (injection patterns, traversal, etc.)
	if err := sv.validatePathSecurity(path); err != nil {
		return err
	}

	// Delegate syntax validation to internal package for consistent behavior
	// This includes bracket matching, depth checks, and array index validation
	if err := internal.ValidatePath(path); err != nil {
		return newPathError(path, err.Error(), ErrInvalidPath)
	}

	return nil
}

// normalizeJSONEscapes returns s with JSON \uXXXX escape sequences (including
// surrogate pairs) decoded to their literal characters. Every other byte —
// including other backslash escapes like \n or \" and malformed/truncated \u
// sequences — is copied verbatim. The result feeds pattern scanning only and
// is never returned to callers or used as parsed data.
//
// SECURITY: dangerous-pattern matching runs on the raw JSON text, but JSON
// permits any character to be written as \uXXXX. Without normalization a
// payload like "<script>" or "__proto__" contains none of the
// literal pattern bytes and evades every check, even though the decoded value
// handed to the caller is dangerous. Notably the library's own encoder emits
// < for '<' by default (EscapeHTML), so without this its own output
// would evade its own scanner on re-validation. (D-002)
func normalizeJSONEscapes(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	i := 0
	for i < len(s) {
		// Bulk-copy the run before the next backslash (P-003): between escape
		// sites the document is copied in whole segments — one vectorized
		// IndexByte scan plus one WriteString memmove per segment — instead of
		// one WriteByte call per byte. Scan discipline is unchanged: the same
		// left-to-right greedy walk, so an escaped backslash before a uXXXX
		// run still behaves exactly as the per-byte loop did.
		next := strings.IndexByte(s[i:], '\\')
		if next < 0 {
			b.WriteString(s[i:])
			return b.String()
		}
		next += i
		b.WriteString(s[i:next])
		// At the backslash: decode a \uXXXX escape, else copy the byte and
		// step past it (the following byte is then re-examined on its own
		// merits, as in the original loop).
		if next+5 < len(s) && s[next+1] == 'u' {
			if r, width, ok := decodeJSONUnicodeEscape(s[next:]); ok {
				b.WriteRune(r)
				i = next + width
				continue
			}
		}
		b.WriteByte(s[next])
		i = next + 1
	}
	return b.String()
}

// decodeJSONUnicodeEscape decodes one \uXXXX escape at the start of s,
// combining a high+low surrogate pair into a single rune. It returns the
// rune, the number of bytes consumed (6, or 12 for a surrogate pair), and
// whether the sequence was a well-formed JSON unicode escape.
func decodeJSONUnicodeEscape(s string) (rune, int, bool) {
	r1, ok := parseHex4(s[2:6])
	if !ok {
		return 0, 0, false
	}
	if r1 >= 0xD800 && r1 <= 0xDBFF && len(s) >= 12 && s[6] == '\\' && s[7] == 'u' {
		if r2, ok2 := parseHex4(s[8:12]); ok2 && r2 >= 0xDC00 && r2 <= 0xDFFF {
			r := 0x10000 + (r1-0xD800)<<10 + (r2 - 0xDC00)
			return rune(r), 12, true
		}
	}
	// Lone surrogate: decode to utf8.RuneError so it still occupies a rune in
	// the normalized view instead of being silently dropped or passed through.
	if r1 >= 0xD800 && r1 <= 0xDFFF {
		return utf8.RuneError, 6, true
	}
	return rune(r1), 6, true
}

// parseHex4 parses exactly 4 lowercase-or-uppercase hex digits into a rune.
func parseHex4(s string) (rune, bool) {
	var v rune
	for i := 0; i < 4; i++ {
		c := s[i]
		switch {
		case c >= '0' && c <= '9':
			v = v<<4 | rune(c-'0')
		case c >= 'a' && c <= 'f':
			v = v<<4 | rune(c-'a'+10)
		case c >= 'A' && c <= 'F':
			v = v<<4 | rune(c-'A'+10)
		default:
			return 0, false
		}
	}
	return v, true
}

func (sv *securityValidator) validateJSONSecurity(jsonStr string) error {
	// Fast path: check for null bytes first (most critical). This runs on the
	// RAW text, before escape normalization: a literal NUL byte is invalid
	// JSON and an injection marker, while an escaped \u0000 inside a string
	// literal is valid JSON that encoding/json accepts — and that this
	// library's own encoder emits — so it must not be conflated with a raw
	// NUL (D-002).
	if strings.IndexByte(jsonStr, 0) != -1 {
		return newSecurityError("validate_json_security", "null byte injection detected")
	}

	// SECURITY (D-002): normalize \uXXXX escapes BEFORE the pattern gates and
	// scans so escape-encoded payloads are inspected in their decoded form —
	// otherwise they bypass the pattern check entirely (see
	// normalizeJSONEscapes). The Contains gate keeps documents without \u
	// escapes on a zero-cost path.
	if strings.Contains(jsonStr, `\u`) {
		jsonStr = normalizeJSONEscapes(jsonStr)
	}

	// Fast path: for small JSON strings, use the original approach
	// For large JSON strings (>4KB), use a sampling approach
	if len(jsonStr) < securitySmallJSONThreshold {
		return sv.validateJSONSecurityFull(jsonStr)
	}

	// For large JSON, use optimized scanning with early termination
	// Most legitimate JSON data doesn't contain dangerous patterns
	// We check for common indicators first

	// Fast check: if the JSON contains no letters (only numbers/symbols), skip pattern check
	// This catches numeric arrays and simple data
	hasLetters := false
	for i := 0; i < len(jsonStr); i++ {
		c := jsonStr[i]
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') {
			hasLetters = true
			break
		}
	}
	// D-002 (gate fix): the shortcut previously ignored caller-registered
	// patterns entirely — a numeric-only custom pattern (card numbers, SSNs)
	// on a >4KB numeric payload was never scanned.
	if !hasLetters && len(sv.additionalPatterns) == 0 && globalPatternRegistry.Len() == 0 {
		return nil
	}

	// Use efficient combined scanning for dangerous patterns
	// Check multiple patterns in a single pass where possible
	return sv.validateJSONSecurityOptimized(jsonStr)
}

// validateJSONSecurityFull performs full security validation for small JSON strings.
// Uses unified scanWindowForPatterns to ensure consistency with the optimized path.
// All dangerous patterns are checked from the single source of truth: dangerousPatterns + criticalPatterns.
func (sv *securityValidator) validateJSONSecurityFull(jsonStr string) error {
	return sv.scanWindowForPatterns(jsonStr)
}

// validateJSONSecurityOptimized performs optimized security validation for large JSON strings
//
// SECURITY APPROACH:
// This function uses a multi-layered security approach:
//  1. Full scan of critical patterns (__proto__, constructor, prototype) - always performed
//  2. Indicator character check - skips expensive scanning if no dangerous characters exist
//  3. Suspicious character density check - forces full scan if high density detected
//  4. Rolling window scanning with complete coverage - NO GAPS between scan windows
//  5. Pattern fragment detection - performs targeted scanning if suspicious fragments found
//
// SECURITY FIX: The previous sampling-based approach had gaps that could be exploited.
// This implementation uses a rolling window approach that guarantees 100% coverage
// by ensuring every byte is scanned, while still optimizing for performance by using
// a sliding window with overlap equal to the longest pattern length.
//
// SECURITY NOTE (GEN-001 P1, corrected): the DEFAULT (FullSecurityScan=false)
// already guarantees 100% coverage via the rolling window and is typically
// FASTER on clean input — the indicator checks can skip scanning entirely.
// FullSecurityScan=true selects the simpler single-pass scan of every byte:
// deterministic, but usually slower. For trusted internal data prefer
// SkipValidation (which still enforces size/depth limits), not this flag.
func (sv *securityValidator) validateJSONSecurityOptimized(jsonStr string) error {
	// If full security scan is enabled, use the simpler full scan approach
	if sv.fullSecurityScan {
		return sv.validateJSONSecurityFull(jsonStr)
	}

	// SECURITY: Always scan critical patterns in full regardless of JSON size
	// These patterns are too dangerous to miss due to sampling
	if err := sv.checkCriticalPatterns(jsonStr); err != nil {
		return err
	}

	// PERFORMANCE: Check for indicator characters using the pre-built map
	// If none of these exist, we can skip the expensive pattern matching.
	// Config-supplied custom patterns may use characters outside the built-in
	// indicator set, so bypass the shortcut when any are configured; globally
	// registered patterns are still subject to the shortcut but are scanned
	// live in scanWindowForPatterns once the rolling window runs.
	// D-002 (gate fix): include the GLOBAL registry in the shortcut
	// condition — a registered pattern built only from non-indicator bytes
	// (e.g. "mu-777") was skipped here, contradicting the comment below.
	if !sv.hasIndicatorChars(jsonStr) && len(sv.additionalPatterns) == 0 && globalPatternRegistry.Len() == 0 {
		return nil
	}

	// SECURITY: Check for suspicious character density - if density is high, force full scan
	// This prevents attackers from hiding malicious code in dense payload sections
	if sv.hasSuspiciousCharacterDensity(jsonStr) {
		return sv.validateJSONSecurityFull(jsonStr)
	}

	// SECURITY FIX: Rolling window scan with guaranteed 100% coverage
	return sv.scanWithRollingWindow(jsonStr)
}

// checkCriticalPatterns scans for the most dangerous patterns that must never be missed
func (sv *securityValidator) checkCriticalPatterns(jsonStr string) error {
	for _, cp := range criticalPatterns {
		if strings.Contains(jsonStr, cp.pattern) {
			return newSecurityError("validate_json_security", fmt.Sprintf("dangerous pattern: %s", cp.name))
		}
	}
	return nil
}

// hasIndicatorChars checks if the JSON contains any characters commonly found in dangerous patterns
// PERFORMANCE: Uses pre-built map for O(1) lookup per character
func (sv *securityValidator) hasIndicatorChars(jsonStr string) bool {
	for i := 0; i < len(jsonStr); i++ {
		if indicatorChars[jsonStr[i]] {
			return true
		}
	}
	return false
}

// scanWithRollingWindow performs a rolling window scan with overlap for guaranteed coverage
// SECURITY FIX: Ensures no pattern can straddle window boundaries and be missed
func (sv *securityValidator) scanWithRollingWindow(jsonStr string) error {
	jsonLen := len(jsonStr)

	// Add safety margin to the pre-computed max pattern length.
	// SECURITY: the overlap must also cover config-supplied custom patterns
	// (maxCustomPatternLen) — maxDangerousPatternLen only knows the built-in
	// and globally-registered patterns, so a longer custom pattern could
	// otherwise straddle a window boundary and never be fully contained in
	// any window.
	overlapSize := maxDangerousPatternLen() + 8
	if sv.maxCustomPatternLen+8 > overlapSize {
		overlapSize = sv.maxCustomPatternLen + 8
	}

	// Window size tuned for cache efficiency
	windowSize := securityScanWindowSize
	// D-002: a registered pattern at least as long as the default window can
	// never fit inside any window (the byte-stride fallback below guarantees
	// coverage only up to window length). Grow the window past the longest
	// pattern by a full default window so the stride
	// (windowSize-overlapSize) stays near the normal ~32KB instead of
	// degenerating to a byte-by-byte sweep.
	if overlapSize >= windowSize {
		windowSize = overlapSize + securityScanWindowSize
	}

	// For smaller JSON, just scan it all
	if jsonLen <= windowSize*2 {
		return sv.scanWindowForPatterns(jsonStr)
	}

	// Rolling window scan with overlap - guarantees 100% coverage
	for offset := 0; offset < jsonLen; {
		end := min(offset+windowSize, jsonLen)

		window := jsonStr[offset:end]
		if err := sv.scanWindowForPatterns(window); err != nil {
			return err
		}

		// Move to next window, but overlap by the max pattern length
		nextOffset := offset + windowSize - overlapSize
		if nextOffset <= offset {
			// Overlap >= window size (a registered pattern of at least
			// securityScanWindowSize). The old fallback jumped straight to
			// `end`, leaving a zero-overlap seam a boundary-straddling
			// occurrence could fall through despite the "guarantees 100%
			// coverage" claim. Advance one byte instead: windows then overlap
			// by windowSize-1, covering every pattern up to
			// securityScanWindowSize. The O(n*window) cost only applies to
			// such pathological patterns (D-002).
			nextOffset = offset + 1
		}
		offset = nextOffset
	}

	// Additional check - scan for pattern fragments that might indicate attacks
	if sv.hasPatternFragments(jsonStr) {
		return sv.scanWindowForPatterns(jsonStr)
	}

	return nil
}

// scanWindowForPatterns scans a single window for dangerous patterns.
// When disableDefaultPatterns is true, only critical patterns are scanned
// (the non-critical HTML/event-handler patterns are skipped).
// Critical patterns (__proto__, constructor, prototype) are always enforced.
func (sv *securityValidator) scanWindowForPatterns(window string) error {
	if sv.disableDefaultPatterns {
		// Only scan critical patterns when defaults are disabled.
		// GEN-001 P0-3: all occurrences context-checked (see the reporting
		// loop below) — a benign first hit must not shield a later one.
		for _, cp := range criticalPatterns {
			if idx := fastIndexIgnoreCase(window, cp.pattern); idx != -1 {
				if sv.indexInDangerousContext(window, cp.pattern, idx) >= 0 {
					return newSecurityError("validate_json_security", fmt.Sprintf("dangerous pattern: %s", cp.name))
				}
			}
		}
		return sv.scanCustomPatterns(window)
	}

	// Single-pass scan (P-003): record each pattern's FIRST case-insensitive
	// occurrence in one pass over the window (scanWindowPatterns), then run
	// the ordered reporting loop over the recorded positions — same pattern
	// order, same context checks — but the window is scanned once instead of
	// once per pattern (~29 rescans whenever the old prefilter hit).
	//
	// GEN-001 P0-3: the context check now covers EVERY occurrence of the
	// pattern, not only the recorded first one. Previously a benign
	// word-internal first occurrence (the "onerror" inside "myonerrorx")
	// shielded a later standalone occurrence in the same window — a trivially
	// exploitable detection bypass pinned by an old test. indexInDangerousContext
	// starts at the recorded first position and walks the remaining
	// occurrences; the extra pass is paid only for patterns whose first hit
	// was context-declined, so clean windows keep the single-pass fast path.
	first := make([]int32, len(dangerousPatterns))
	for i := range first {
		first[i] = -1
	}
	scanWindowPatterns(window, first)
	for i, dp := range dangerousPatterns {
		idx := int(first[i])
		if idx < 0 {
			continue
		}
		if sv.indexInDangerousContext(window, dp.pattern, idx) >= 0 {
			return newSecurityError("validate_json_security", fmt.Sprintf("dangerous pattern: %s", dp.name))
		}
	}
	return sv.scanCustomPatterns(window)
}

// scanCustomPatterns scans a window for config-supplied and globally-registered
// dangerous patterns. These are enforced in addition to the built-in defaults
// and remain in effect even when disableDefaultPatterns is set, because they
// are explicitly opted in by the caller. Globally registered patterns are read
// live so RegisterDangerousPattern takes effect for existing processors.
func (sv *securityValidator) scanCustomPatterns(window string) error {
	if err := sv.scanPatternSlice(window, sv.additionalPatterns); err != nil {
		return err
	}
	// Global registry: read live. Usually empty, so List() is a cheap no-op.
	globals := globalPatternRegistry.List()
	if len(globals) == 0 {
		return nil
	}
	return sv.scanPatternSlice(window, toInternalPatterns(globals))
}

// scanPatternSlice scans a window for the given patterns using the same
// case-insensitive context check as the built-in pattern scan.
// GEN-001 P0-3: all occurrences context-checked — a benign word-internal
// first hit must not shield a later standalone one.
func (sv *securityValidator) scanPatternSlice(window string, patterns []dangerousPattern) error {
	for _, dp := range patterns {
		if idx := fastIndexIgnoreCase(window, dp.pattern); idx != -1 {
			if sv.indexInDangerousContext(window, dp.pattern, idx) >= 0 {
				return newSecurityError("validate_json_security", fmt.Sprintf("dangerous pattern: %s", dp.name))
			}
		}
	}
	return nil
}

// hasSuspiciousCharacterDensity checks if the JSON has abnormally high density of
// characters commonly used in attack payloads
// SECURITY FIX: Now samples from multiple regions of the JSON to detect attacks
// hidden in the middle or end of the payload, not just the beginning
func (sv *securityValidator) hasSuspiciousCharacterDensity(jsonStr string) bool {
	jsonLen := len(jsonStr)
	if jsonLen == 0 {
		return false
	}

	// SECURITY FIX: Sample from multiple regions: beginning, middle, and end
	// This prevents attackers from hiding malicious code in any single region
	sampleSize := securitySampleSize

	countSuspicious := func(start, end int) (count int, density float64) {
		if start < 0 {
			start = 0
		}
		if end > jsonLen {
			end = jsonLen
		}
		if start >= end {
			return 0, 0
		}

		for i := start; i < end; i++ {
			c := jsonStr[i]
			// Characters commonly found in XSS/injection payloads
			if c == '<' || c == '>' || c == '(' || c == ')' || c == ';' || c == '=' || c == '&' {
				count++
			}
		}
		return count, float64(count) / float64(end-start)
	}

	// Check beginning
	_, density1 := countSuspicious(0, sampleSize)
	if density1 > securityLocalDensityThreshold {
		return true
	}

	// Check middle region
	if jsonLen > sampleSize*2 {
		midStart := (jsonLen - sampleSize) / 2
		_, density2 := countSuspicious(midStart, midStart+sampleSize)
		if density2 > securityLocalDensityThreshold {
			return true
		}
	}

	// Check end
	if jsonLen > sampleSize {
		_, density3 := countSuspicious(jsonLen-sampleSize, jsonLen)
		if density3 > securityLocalDensityThreshold {
			return true
		}
	}

	// SECURITY FIX: Also check for distributed suspicious characters across entire string
	// This catches attacks that spread malicious content thinly across the payload
	totalSuspicious, _ := countSuspicious(0, jsonLen)
	overallDensity := float64(totalSuspicious) / float64(jsonLen)

	// Use a lower threshold for overall density since attacks might be spread out
	return overallDensity > securityOverallDensityThreshold
}

// patternFragments holds partial dangerous patterns that might indicate an
// attempt to hide malicious code. Package-level to avoid allocating the
// ~40-element slice on every call (hasPatternFragments runs per validation
// of every large JSON input).
var patternFragments = []string{
	// JavaScript execution
	"script", "eval", "function", "settimeout", "setinterval",
	// Prototype manipulation
	"proto", "constructor", "prototype",
	// DOM access
	"document", "window", "innerhtml", "outerhtml",
	// Event handlers (comprehensive)
	"onload", "onerror", "onclick", "onmouse", "onkey", "onfocus", "onblur",
	"onchange", "onsubmit", "onreset", "onscroll", "onwheel", "ondrag",
	// Code execution
	"import(", "require(", "new func",
	// Security-sensitive
	"cookie", "token", "secret", "password", "credential",
	// Encoding bypass attempts
	"fromcharcode", "atob(", "btoa(", "escape(", "unescape(",
	// CSS expression injection
	"expression(", "url(", "behavior:",
	// Data URLs
	"data:", "javascript:", "vbscript:",
}

// hasPatternFragments checks for partial dangerous patterns that might indicate
// an attempt to hide malicious code
// SECURITY FIX: Expanded fragment list for better detection coverage
func (sv *securityValidator) hasPatternFragments(jsonStr string) bool {
	// Check for partial patterns that might be completed elsewhere
	for _, frag := range patternFragments {
		if fastIndexIgnoreCase(jsonStr, frag) != -1 {
			return true
		}
	}
	return false
}

// fastIndexIgnoreCase is an optimized case-insensitive search
// Delegates to shared implementation in internal package
func fastIndexIgnoreCase(s, pattern string) int {
	return internal.IndexIgnoreCase(s, pattern)
}

// isDangerousContextIgnoreCase checks if a pattern match is in a dangerous context (case-insensitive)
// SECURITY FIX: Improved to handle patterns that start/end with special characters
func (sv *securityValidator) isDangerousContextIgnoreCase(s string, idx, patternLen int) bool {
	// Get the pattern being checked from the window
	if idx+patternLen > len(s) {
		return false
	}

	// SECURITY FIX: Check if the pattern starts with a special delimiter character
	// Patterns like <script, <iframe, etc. start with '<' which is already a delimiter
	// In this case, we don't need to check the character before
	firstChar := s[idx]
	startsWithDelimiter := firstChar == '<' || firstChar == '{' || firstChar == '[' || firstChar == '('

	// SECURITY FIX: Check if the pattern ends with a special delimiter character
	// Patterns like eval(, function(, etc. end with '(' which is already a delimiter
	// In this case, we don't need to check the character after
	lastChar := s[idx+patternLen-1]
	endsWithDelimiter := lastChar == '(' || lastChar == '[' || lastChar == '{' || lastChar == ':' || lastChar == '.'

	// Check before context
	before := startsWithDelimiter || idx == 0 || !internal.IsWordChar(s[idx-1])

	// Check after context
	after := endsWithDelimiter || idx+patternLen >= len(s) || !internal.IsWordChar(s[idx+patternLen])

	return before && after
}

// indexInDangerousContext returns the index of the first occurrence of
// pattern in s at or after `from` that sits in a dangerous context, or -1.
//
// GEN-001 P0-3: the pattern gates used to context-check only each pattern's
// FIRST occurrence, so a benign word-internal first occurrence (the "onerror"
// inside "myonerrorx") shielded every later standalone occurrence in the same
// window — a trivially exploitable detection bypass. This helper walks the
// occurrences from `from` until one is context-dangerous (or none is). It is
// only reached for patterns that already occurred, so the extra pass costs
// one window sweep per benignly-occurring pattern, not per pattern.
func (sv *securityValidator) indexInDangerousContext(s, pattern string, from int) int {
	n := len(pattern)
	for i := from; i+n <= len(s); i++ {
		if internal.IsMatchPatternIgnoreCase(s[i:i+n], pattern) &&
			sv.isDangerousContextIgnoreCase(s, i, n) {
			return i
		}
	}
	return -1
}

// validatePathSecurity validates JSON paths for security issues.
// NOTE: For file path validation, see file.go:containsPathTraversal which provides
// more comprehensive checks including recursive URL decoding and Unicode lookalikes.
// This function focuses on JSON path-specific security concerns.
func (sv *securityValidator) validatePathSecurity(path string) error {
	// Normalize the path using Unicode NFC to detect homograph attacks
	// This ensures that visually similar characters are normalized
	// PERFORMANCE: Skip NFC normalization for pure ASCII paths (common case)
	normalizedPath := path
	if !isAllASCII(path) {
		normalizedPath = norm.NFC.String(path)
	}

	if strings.IndexByte(normalizedPath, 0) != -1 {
		return newPathError(path, "null byte injection detected", ErrSecurityViolation)
	}

	// Check for zero-width characters that could be used to bypass pattern matching
	if containsZeroWidthChars(normalizedPath) {
		return newPathError(path, "zero-width characters detected", ErrSecurityViolation)
	}

	// Check path traversal patterns on normalized path
	if strings.Contains(normalizedPath, "..") {
		return newPathError(path, "path traversal detected", ErrSecurityViolation)
	}

	// Check URL encoding bypass (including double encoding) - case-insensitive
	// PERFORMANCE: Use strings.Contains with case folding for short percent-encoded patterns
	// instead of the generic containsAnyIgnoreCase loop
	if containsPercentEncodingBypass(normalizedPath) {
		return newPathError(path, "path traversal via URL encoding detected", ErrSecurityViolation)
	}

	// Check UTF-8 overlong encoding - case-insensitive
	if containsOverlongEncoding(normalizedPath) {
		return newPathError(path, "path traversal via UTF-8 overlong encoding detected", ErrSecurityViolation)
	}

	// Check excessive special characters
	if strings.Contains(normalizedPath, ":::") || strings.Contains(normalizedPath, "[[[") || strings.Contains(normalizedPath, "}}}") {
		return newPathError(path, "excessive special characters", ErrSecurityViolation)
	}

	return nil
}

// isAllASCII checks if a string contains only ASCII characters (< 0x80)
// PERFORMANCE: Used to skip expensive Unicode normalization for common ASCII-only paths
func isAllASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] >= 0x80 {
			return false
		}
	}
	return true
}

// containsPercentEncodingBypass checks for URL-encoded path traversal patterns
// PERFORMANCE: Uses direct string search with case folding instead of generic
// containsAnyIgnoreCase loop, avoiding function call overhead per pattern.
func containsPercentEncodingBypass(s string) bool {
	// Percent-encoded patterns: %2e, %2f, %5c, %00, %252e, %252f
	// Check for '%' presence first as a fast rejection
	idx := strings.IndexByte(s, '%')
	if idx == -1 {
		return false
	}
	// Now check each pattern using case-insensitive comparison
	// Use the single-pass approach: scan for '%' and check following bytes
	remaining := s[idx:]
	for {
		i := strings.IndexByte(remaining, '%')
		if i == -1 {
			return false
		}
		after := remaining[i+1:]
		// Check each pattern against the bytes following '%'
		if len(after) >= 2 {
			twoBytes := after[:2]
			// %2e (%2E) — dot
			if twoBytes[0] == '2' && (twoBytes[1] == 'e' || twoBytes[1] == 'E') {
				return true
			}
			// %2f (%2F) — slash
			if twoBytes[0] == '2' && (twoBytes[1] == 'f' || twoBytes[1] == 'F') {
				return true
			}
			// %5c (%5C) — backslash
			if twoBytes[0] == '5' && (twoBytes[1] == 'c' || twoBytes[1] == 'C') {
				return true
			}
			// %00 — null byte
			if twoBytes[0] == '0' && twoBytes[1] == '0' {
				return true
			}
			// %25 — double encoding start (%252e, %252f)
			if twoBytes[0] == '2' && twoBytes[1] == '5' && len(after) >= 4 {
				fourBytes := after[:4]
				// %252e or %252E
				if (fourBytes[2] == '2') && (fourBytes[3] == 'e' || fourBytes[3] == 'E') {
					return true
				}
				// %252f or %252F
				if (fourBytes[2] == '2') && (fourBytes[3] == 'f' || fourBytes[3] == 'F') {
					return true
				}
			}
		}
		remaining = after
	}
}

// containsOverlongEncoding checks for UTF-8 overlong encoding patterns
// PERFORMANCE: Single-pass scan for %c0/%c1 patterns
func containsOverlongEncoding(s string) bool {
	idx := strings.IndexByte(s, '%')
	if idx == -1 {
		return false
	}
	remaining := s[idx:]
	for {
		i := strings.IndexByte(remaining, '%')
		if i == -1 {
			return false
		}
		after := remaining[i+1:]
		if len(after) >= 5 {
			// %c0%af or %C0%AF
			a, b := after[0], after[1]
			if (a == 'c' || a == 'C') && (b == '0') {
				if after[2] == '%' && len(after) >= 5 {
					c, d := after[3], after[4]
					if (c == 'a' || c == 'A') && (d == 'f' || d == 'F') {
						return true
					}
				}
			}
			// %c1%9c or %C1%9C
			if (a == 'c' || a == 'C') && (b == '1') {
				if after[2] == '%' && len(after) >= 5 {
					c, d := after[3], after[4]
					if (c == '9') && (d == 'c' || d == 'C') {
						return true
					}
				}
			}
		}
		remaining = after
	}
}

// containsZeroWidthChars checks for zero-width and other invisible Unicode characters
func containsZeroWidthChars(s string) bool {
	for _, r := range s {
		// Zero-width characters and other invisible chars that could bypass security checks
		switch r {
		case '\u200B', // Zero-width space
			'\u200C', // Zero-width non-joiner
			'\u200D', // Zero-width joiner
			'\u200E', // Left-to-right mark
			'\u200F', // Right-to-left mark
			'\uFEFF', // Byte order mark (zero-width no-break space)
			'\u2060', // Word joiner
			'\u2061', // Function application
			'\u2062', // Invisible times
			'\u2063', // Invisible separator
			'\u2064', // Invisible plus
			'\u206A', // Inhibit symmetric swapping
			'\u206B', // Activate symmetric swapping
			'\u206C', // Inhibit Arabic form shaping
			'\u206D', // Activate Arabic form shaping
			'\u206E', // National digit shapes
			'\u206F', // Nominal digit shapes
			// Additional invisible characters for comprehensive security
			'\u00AD', // Soft hyphen
			'\u034F', // Combining grapheme joiner
			'\u061C', // Arabic letter mark
			'\u115F', // Korean jamo filler (choseong)
			'\u1160', // Korean jamo filler (jungseong)
			'\u180E', // Mongolian vowel separator
			'\u2066', // Left-to-right isolate
			'\u2067', // Right-to-left isolate
			'\u2068', // First strong isolate
			'\u2069', // Pop directional isolate
			'\uFFFD': // Replacement character
			return true
		}
	}
	return false
}

func (sv *securityValidator) validateJSONStructure(jsonStr string) error {
	// Fast path: trim whitespace without allocation
	start := 0
	end := len(jsonStr)

	// Skip leading whitespace
	for start < end && isSpace(jsonStr[start]) {
		start++
	}
	// Skip trailing whitespace
	for end > start && isSpace(jsonStr[end-1]) {
		end--
	}

	if start >= end {
		return newOperationError("validate_json_structure", "JSON string is empty after trimming", ErrInvalidJSON)
	}

	firstChar := jsonStr[start]
	lastChar := jsonStr[end-1]

	if !((firstChar == '{' && lastChar == '}') || (firstChar == '[' && lastChar == ']') ||
		(firstChar == '"' && lastChar == '"') || isValidJSONPrimitive(jsonStr[start:end])) {
		return newOperationError("validate_json_structure", "invalid JSON structure", ErrInvalidJSON)
	}

	return nil
}

// validateStructureLimits (P-003) enforces, in ONE byte-level pass, both the
// nesting-depth invariants of the former validateNestingDepth and the
// per-container limits (MaxObjectKeys / MaxArrayElements, duplicate keys) of
// the former validateContainerCounts.
//
// EQUIVALENCE CONTRACT: it must behave exactly like running the former
// nesting scan to completion and, only when that returned nil, the former
// container scan:
//   - a nesting violation ALWAYS wins, even when a container violation
//     occurs earlier in the text (the walk records the first error of each
//     class and resolves nesting-first at the end; it aborts early only on a
//     nesting violation, since after one the container scan would never have
//     run);
//   - within each class the FIRST textual violation wins;
//   - the end-of-scan unbalanced-bracket check belongs to the nesting class
//     and precedes any container error;
//   - inputs below securityNestingValidationThreshold keep the historical
//     lighter nesting discipline (no total-bracket / consecutive-open anomaly
//     checks), and container counting is skipped entirely when no container
//     limit is configured and duplicate-key detection is off.
//
// The two former functions live on in security_test.go as the reference
// implementation this contract is tested against (TestP003StructureLimits).
//
// SECURITY: Together the two check families close the gap on
// memory-exhaustion DoS — depth/bracket anomalies for nested bombs,
// container caps for flat-but-wide structures ({"k1":1,...,"kN":1} at depth 1
// sails past every bracket check). Always enforced (including under
// SkipValidation) because they protect the process itself.
//
// PERFORMANCE: one pass instead of two over the input (P-003 profiling: the
// two scans were ~13% of cold-validation CPU on large documents); the string/
// escape discipline and container bookkeeping are byte-for-byte those of the
// former scans.
func (sv *securityValidator) validateStructureLimits(jsonStr string) error {
	maxCheckDepth := sv.maxNestingDepth
	if maxCheckDepth <= 0 {
		maxCheckDepth = 100
	}
	large := len(jsonStr) >= securityNestingValidationThreshold
	checkContainers := sv.maxObjectKeys > 0 || sv.maxArrayElements > 0 || sv.detectDuplicateKeys
	detectDup := sv.detectDuplicateKeys

	depth := 0
	inString := false
	escaped := false
	totalBrackets := 0
	consecutiveOpens := 0
	var stack []containerFrame
	if checkContainers {
		stack = make([]containerFrame, 0, 32)
	}
	// First container-class violation (kept when no nesting violation
	// shadows it). Nesting-class violations return immediately, mirroring the
	// former nesting scan's abort.
	var containerErr error

	for i := 0; i < len(jsonStr); i++ {
		c := jsonStr[i]

		if escaped {
			escaped = false
			continue
		}

		if inString {
			switch c {
			case '\\':
				escaped = true
			case '"':
				inString = false
				// Duplicate-key check (GEN-001): a string just closed in key
				// position of an object frame. Containers cannot open inside
				// a string, so the frame seen here is the one that was on top
				// when the key opened. Keys are compared as raw bytes: two
				// spellings that differ only by escape encoding are treated
				// as distinct — the underlying parse is still last-wins for
				// such pairs.
				if detectDup && len(stack) > 0 {
					top := &stack[len(stack)-1]
					if !top.isArray && top.keyStart >= 0 {
						key := jsonStr[top.keyStart:i]
						top.keyStart = -1
						if top.keySet == nil {
							top.keySet = make(map[string]struct{}, 8)
						}
						if _, dup := top.keySet[key]; dup {
							if containerErr == nil {
								containerErr = newOperationError("validate_container_counts",
									fmt.Sprintf("duplicate object key %q", key), ErrDuplicateKey)
							}
						} else {
							top.keySet[key] = struct{}{}
						}
					}
				}
			default:
				if large {
					consecutiveOpens = 0 // mirrors the nesting scan's default case
				}
			}
			continue
		}

		switch c {
		case '"':
			inString = true
			if checkContainers {
				if detectDup && len(stack) > 0 {
					top := &stack[len(stack)-1]
					// In an object, a string opening while expecting a child is a
					// KEY (a value string only follows ':', which clears
					// expectingChild). Array strings are values — not tracked.
					if !top.isArray && top.expectingChild {
						top.keyStart = i + 1
					}
				}
				noteValueStart(stack)
			}
		case '{', '[':
			depth++
			if large {
				totalBrackets++
				consecutiveOpens++
				// SECURITY: too many consecutive opens (potential attack) —
				// checked before depth, matching the former scan's order.
				if consecutiveOpens > securityMaxConsecutiveOpens {
					return newOperationError("validate_nesting_depth",
						fmt.Sprintf("too many consecutive opening brackets at position %d", i), ErrDepthLimit)
				}
			}
			if depth > maxCheckDepth {
				return newOperationError("validate_nesting_depth",
					fmt.Sprintf("nesting depth %d exceeds maximum %d", depth, maxCheckDepth), ErrDepthLimit)
			}
			if large && totalBrackets > securityMaxTotalBrackets {
				return newOperationError("validate_nesting_depth",
					fmt.Sprintf("total bracket count %d exceeds maximum %d", totalBrackets, securityMaxTotalBrackets), ErrDepthLimit)
			}
			if checkContainers {
				// A container open is itself a value start in its parent...
				noteValueStart(stack)
				// ...then descend into the new container.
				stack = append(stack, containerFrame{
					isArray:        c == '[',
					expectingChild: true,
					keyStart:       -1,
				})
			}
		case '}', ']':
			depth--
			if large {
				totalBrackets++
				consecutiveOpens = 0 // Reset on closing bracket
			}
			if checkContainers && len(stack) > 0 {
				frame := stack[len(stack)-1]
				stack = stack[:len(stack)-1]
				if containerErr == nil {
					if frame.isArray {
						if sv.maxArrayElements > 0 && frame.count > sv.maxArrayElements {
							containerErr = newOperationError("validate_container_counts",
								fmt.Sprintf("array has %d elements, exceeds maximum %d", frame.count, sv.maxArrayElements),
								ErrSizeLimit)
						}
					} else if sv.maxObjectKeys > 0 && frame.count > sv.maxObjectKeys {
						containerErr = newOperationError("validate_container_counts",
							fmt.Sprintf("object has %d keys, exceeds maximum %d", frame.count, sv.maxObjectKeys),
							ErrSizeLimit)
					}
				}
			}
		case '\\':
			// Outside a string a backslash is meaningless to the nesting scan
			// (and never resets consecutiveOpens — it was a matched case there,
			// not the default), but the container scan counts it as the leading
			// byte of a primitive value.
			if checkContainers {
				noteValueStart(stack)
			}
		case ',':
			if large {
				consecutiveOpens = 0 // default case of the former nesting scan
			}
			if checkContainers && len(stack) > 0 {
				stack[len(stack)-1].expectingChild = true
			}
		case ':':
			// Object key/value separator. The key was already counted as a
			// value start; nothing to do. (A ':' outside an object is malformed
			// JSON and is rejected by the parser downstream.)
			if large {
				consecutiveOpens = 0 // default case of the former nesting scan
			}
		default:
			if large {
				consecutiveOpens = 0 // Reset on non-bracket character
			}
			// Whitespace is structural; any other byte is the leading byte of
			// a primitive value (digit, '-', 't'/'f'/'n', etc.).
			if checkContainers && !isSpace(c) {
				noteValueStart(stack)
			}
		}
	}

	// SECURITY: unbalanced brackets — end-of-scan check of the nesting class;
	// precedes any container error (the container scan never ran when nesting
	// failed).
	if depth != 0 {
		return newOperationError("validate_nesting_depth",
			"unbalanced brackets in JSON structure", ErrInvalidJSON)
	}
	return containerErr
}

// containerFrame tracks one open container (object or array) during the
// structural scan performed by validateContainerCounts.
type containerFrame struct {
	isArray        bool
	count          int  // direct children (keys or elements) seen so far
	expectingChild bool // true after '{', '[', or ',': the next value is a new child
	// Duplicate-key detection (Config.DetectDuplicateKeys, GEN-001): for an
	// object frame, keyStart >= 0 marks a key string currently being scanned
	// (set when '"' opens a string in key position, i.e. while expectingChild)
	// and keySet holds the keys already closed in this object. Unused for
	// array frames and when detection is off.
	keyStart int
	keySet   map[string]struct{}
}

// noteValueStart records a direct child in the innermost open container when it
// is expecting one. It mutates the top frame of the stack in place.
func noteValueStart(stack []containerFrame) {
	if len(stack) == 0 {
		return
	}
	top := &stack[len(stack)-1]
	if top.expectingChild {
		top.count++
		top.expectingChild = false
	}
}

func isValidJSONPrimitive(s string) bool {
	return internal.IsValidJSONPrimitive(s)
}

// ============================================================================
// SENSITIVE DATA DETECTION
// Methods for detecting sensitive information in cache values
// ============================================================================

// ContainsSensitiveData checks if the result contains sensitive information
// SECURITY: Uses recursive detection with depth limit to prevent DoS
// CONSISTENCY FIX: Uses internal.MaxSensitiveDataDepth constant for unified limits
func (sv *securityValidator) ContainsSensitiveData(data any) bool {
	return sv.containsSensitiveDataRecursive(data, 0, internal.MaxSensitiveDataDepth)
}

// containsSensitiveDataRecursive recursively checks for sensitive data with depth limit
func (sv *securityValidator) containsSensitiveDataRecursive(data any, depth, maxDepth int) bool {
	// SECURITY: Enforce depth limit to prevent DoS
	if depth > maxDepth {
		return false
	}

	if data == nil {
		return false
	}

	// Fast path for primitive types - they cannot contain sensitive field names
	switch data.(type) {
	case int, int8, int16, int32, int64,
		uint, uint8, uint16, uint32, uint64,
		float32, float64, bool:
		return false
	}

	// Check string values for sensitive patterns
	if str, ok := data.(string); ok {
		return sv.containsSensitivePatterns(str)
	}

	// For maps, check keys and recursively check values
	if m, ok := data.(map[string]any); ok {
		for key, value := range m {
			// Check key for sensitive patterns
			if sv.containsSensitivePatterns(key) {
				return true
			}
			// Recursively check value
			if sv.containsSensitiveDataRecursive(value, depth+1, maxDepth) {
				return true
			}
		}
		return false
	}

	// For slices, recursively check elements using head/tail/sampling strategy.
	// SECURITY: Checks first 100, last 50, and uniform samples in between.
	// Full scan for arrays up to 500 elements to minimize blind spots.
	// Performance is bounded: arrays > 500 sample at most 100 + 20 + 50 = 170 elements.
	if arr, ok := data.([]any); ok {
		n := len(arr)
		if n <= 500 {
			// Small/medium array: check all elements
			for i := 0; i < n; i++ {
				if sv.containsSensitiveDataRecursive(arr[i], depth+1, maxDepth) {
					return true
				}
			}
			return false
		}
		// Large array: check head (first 100), tail (last 50), and sample middle
		for i := 0; i < 100; i++ {
			if sv.containsSensitiveDataRecursive(arr[i], depth+1, maxDepth) {
				return true
			}
		}
		// Check tail (last 50)
		for i := n - 50; i < n; i++ {
			if sv.containsSensitiveDataRecursive(arr[i], depth+1, maxDepth) {
				return true
			}
		}
		// Sample up to 20 elements uniformly from the middle
		step := max(1, (n-150)/20)
		for i := 100; i < n-50; i += step {
			if sv.containsSensitiveDataRecursive(arr[i], depth+1, maxDepth) {
				return true
			}
		}
		return false
	}

	return false
}

// containsSensitivePatterns checks if a string contains sensitive patterns
// SECURITY: Extended pattern list for comprehensive sensitive data detection
// PERFORMANCE: Uses package-level sensitivePatterns slice to avoid allocation
//
// P-001: skips the strings.ToLower copy — and its heap allocation — when the
// input is already all-lowercase ASCII (the dominant shape of JSON keys and
// values). Any uppercase or non-ASCII byte takes the ToLower path so the
// match set is byte-identical to the previous implementation (ToLower maps a
// few non-ASCII runes to ASCII letters, so non-ASCII cannot use the fast path).
func (sv *securityValidator) containsSensitivePatterns(s string) bool {
	// No pattern shorter than minSensitivePatternLen can occur in a shorter
	// string — skip the whole scan for tiny keys/values.
	if len(s) < minSensitivePatternLen {
		return false
	}
	if !isLowercaseASCII(s) {
		s = strings.ToLower(s)
	}
	// Single pass with first-byte buckets (P-003): one walk over s replaces
	// one strings.Contains scan per pattern (~55 on the default set). The
	// inline second-byte compare rejects first-byte coincidences before the
	// exact comparison; candidates surviving both checks are compared
	// verbatim against the (already lowercase) input, so the accepted set is
	// exactly that of the previous per-pattern loop.
	for i := 0; i < len(s); i++ {
		group := sensitivePatternGroups[s[i]]
		for j := range group {
			sp := &group[j]
			end := i + len(sp.pattern)
			if end > len(s) {
				continue
			}
			if s[i+1] != sp.secondByte {
				continue
			}
			if s[i:end] == sp.pattern {
				return true
			}
		}
	}
	return false
}

// isLowercaseASCII reports whether every byte of s is ASCII and none is an
// uppercase letter — exactly the inputs for which ToLower(s) == s, letting
// containsSensitivePatterns search s directly without the lowercased copy.
func isLowercaseASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' || c >= 0x80 {
			return false
		}
	}
	return true
}
