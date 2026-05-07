package json

import (
	"crypto/sha256"
	"fmt"
	"sort"
	"strconv"
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
	// Detects anomalypatterns that could indicate attacks
	securityMaxConsecutiveOpens = 100

	// securityCacheHighWatermark is the cache size threshold for LRU eviction (8000 = 80% of 10000)
	// Triggers proactive cleanup to prevent memory spikes
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
}

// Remove unregisters a pattern by its pattern string.
func (r *patternRegistry) Remove(pattern string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.patterns, pattern)
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

// ListByLevel returns patterns filtered by severity level.
func (r *patternRegistry) ListByLevel(level PatternLevel) []DangerousPattern {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]DangerousPattern, 0)
	for _, p := range r.patterns {
		if p.Level == level {
			result = append(result, p)
		}
	}
	return result
}

// Clear removes all registered patterns.
func (r *patternRegistry) Clear() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.patterns = make(map[string]DangerousPattern)
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
func RegisterDangerousPattern(pattern DangerousPattern) {
	globalPatternRegistry.Add(pattern)
	atomic.StoreInt64(&cachedMaxPatternLen, 0) // Invalidate cache
}

// UnregisterDangerousPattern removes a pattern from the global registry.
func UnregisterDangerousPattern(pattern string) {
	globalPatternRegistry.Remove(pattern)
	atomic.StoreInt64(&cachedMaxPatternLen, 0) // Invalidate cache
}

// ListDangerousPatterns returns all registered custom patterns.
func ListDangerousPatterns() []DangerousPattern {
	return globalPatternRegistry.List()
}

// clearDangerousPatterns removes all custom patterns from the global registry.
// Use with caution - this does not affect built-in patterns.
func clearDangerousPatterns() {
	globalPatternRegistry.Clear()
	atomic.StoreInt64(&cachedMaxPatternLen, 0) // Invalidate cache
}

// getDefaultPatterns returns the built-in dangerous patterns as DangerousPattern values.
// All default patterns are considered Critical level.
// PERFORMANCE: Cached to avoid repeated allocation — the result is immutable.
var getDefaultPatterns = sync.OnceValue(func() []DangerousPattern {
	result := make([]DangerousPattern, len(dangerousPatterns))
	for i, p := range dangerousPatterns {
		result[i] = DangerousPattern{
			Pattern: p.pattern,
			Name:    p.name,
			Level:   PatternLevelCritical,
		}
	}
	return result
})

// getCriticalPatterns returns patterns that are always fully scanned.
// PERFORMANCE: Cached to avoid repeated allocation — the result is immutable.
var getCriticalPatterns = sync.OnceValue(func() []DangerousPattern {
	result := make([]DangerousPattern, len(criticalPatterns))
	for i, p := range criticalPatterns {
		result[i] = DangerousPattern{
			Pattern: p.pattern,
			Name:    p.name,
			Level:   PatternLevelCritical,
		}
	}
	return result
})

// indicatorChars is a pre-computed lookup table for indicator characters.
// PERFORMANCE: O(1) lookup per character during security scanning.
var indicatorChars = [256]bool{
	'<': true, '(': true, ':': true, '.': true, '_': true,
	'O': true, 'R': true, 'P': true, 'S': true,
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

// recomputeMaxPatternLen recalculates and caches the max pattern length
func recomputeMaxPatternLen() int {
	maxLen := 0
	for _, dp := range dangerousPatterns {
		if len(dp.pattern) > maxLen {
			maxLen = len(dp.pattern)
		}
	}
	for _, p := range globalPatternRegistry.List() {
		if len(p.Pattern) > maxLen {
			maxLen = len(p.Pattern)
		}
	}
	atomic.StoreInt64(&cachedMaxPatternLen, int64(maxLen))
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

// =============================================================================
// Security Validator Components
// These types separate concerns for better maintainability and testability
// =============================================================================

// validationCacheEntry holds a cache entry with access time for LRU eviction
// SECURITY FIX: Track access time for better cache management
type validationCacheEntry struct {
	validated  bool
	lastAccess int64 // Unix timestamp for LRU eviction
}

// securityValidator provides comprehensive security validation for JSON processing.
type securityValidator struct {
	maxJSONSize      int64
	maxPathLength    int
	maxNestingDepth  int
	fullSecurityScan bool
	// Composed validators for separation of concerns
	// Cache for validation results
	validationCache map[string]*validationCacheEntry
	cacheMutex      sync.RWMutex
}

// newSecurityValidator creates a new security validator with the given limits.
func newSecurityValidator(maxJSONSize int64, maxPathLength, maxNestingDepth int, fullSecurityScan bool) *securityValidator {
	sv := &securityValidator{
		maxJSONSize:      maxJSONSize,
		maxPathLength:    maxPathLength,
		maxNestingDepth:  maxNestingDepth,
		fullSecurityScan: fullSecurityScan,
		validationCache:  make(map[string]*validationCacheEntry, 256),
	}
	return sv
}

// Close releases resources held by the security validator.
// This should be called when the validator is no longer needed to prevent memory leaks.
func (sv *securityValidator) Close() {
	sv.cacheMutex.Lock()
	defer sv.cacheMutex.Unlock()

	// Clear validation cache to release memory
	sv.validationCache = nil
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

	// Always enforce nesting depth — prevents stack overflow during parsing
	if err := sv.validateNestingDepth(jsonStr); err != nil {
		return err
	}

	return nil
}

// ValidateJSONInput performs comprehensive JSON input validation with enhanced security.
// PERFORMANCE: Uses caching to avoid repeated validation of the same JSON string.
func (sv *securityValidator) ValidateJSONInput(jsonStr string) error {
	if int64(len(jsonStr)) > sv.maxJSONSize {
		return newSizeLimitError("validate_json_input", int64(len(jsonStr)), sv.maxJSONSize)
	}

	if len(jsonStr) == 0 {
		return newOperationError("validate_json_input", "JSON string cannot be empty", ErrInvalidJSON)
	}

	// PERFORMANCE: Check cache for previously validated JSON strings
	// This is especially effective for repeated Get operations on the same JSON
	// Skip all expensive validations for cached strings
	// PERFORMANCE: Get cache key for reuse in cacheValidationWithKey
	cacheKey, cached := sv.isValidationCached(jsonStr)
	if cached {
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

	// Validate nesting depth
	if err := sv.validateNestingDepth(jsonStr); err != nil {
		return err
	}

	// Cache the successful validation - PERFORMANCE: Use pre-computed cache key
	sv.cacheValidationWithKey(cacheKey)

	return nil
}


// getValidationCacheKey computes and returns the cache key for a JSON string
// PERFORMANCE: Returns the key for reuse to avoid double hash computation
// SECURITY: Uses SHA-256 for all string sizes to prevent collision attacks
// OPTIMIZED: Uses manual buffer building to avoid fmt.Sprintf allocations
func (sv *securityValidator) getValidationCacheKey(jsonStr string) string {
	strLen := len(jsonStr)

	// SECURITY: Use SHA-256 for all sizes to prevent collision attacks.
	// SHA-256 provides strong collision resistance (2^128 birthday bound)
	// and the computational overhead on small strings is negligible.
	hash := sha256.Sum256(internal.StringToBytes(jsonStr))

	// Build key manually: "len:hash" format
	// Need up to ~80 bytes: "9999999999:" (11) + 64 hex chars = 75 bytes max
	var buf [80]byte
	lenBytes := strconv.AppendInt(buf[:0], int64(strLen), 10)
	buf[len(lenBytes)] = ':'

	// Write ALL 32 bytes of SHA-256 as hex (64 chars) for full collision resistance
	const hexChars = "0123456789abcdef"
	start := len(lenBytes) + 1
	for i := range 32 {
		buf[start+i*2] = hexChars[hash[i]>>4]
		buf[start+i*2+1] = hexChars[hash[i]&0xF]
	}
	return string(buf[:start+64])
}

// isValidationCached checks if JSON string was previously validated successfully
// PERFORMANCE: Returns the cache key for reuse in cacheValidation to avoid double hash computation
// RACE-FIX: Access time is not updated in read lock to avoid data race.
// The LRU eviction still works correctly with occasional access time updates during Set operations.
func (sv *securityValidator) isValidationCached(jsonStr string) (string, bool) {
	// Compute cache key once
	cacheKey := sv.getValidationCacheKey(jsonStr)

	// Use read lock for fast lookup
	sv.cacheMutex.RLock()
	// SAFETY: Check for nil cache (can happen after Close())
	if sv.validationCache == nil {
		sv.cacheMutex.RUnlock()
		return cacheKey, false
	}
	entry, cached := sv.validationCache[cacheKey]
	// RACE-FIX: Do NOT update entry.lastAccess here with read lock
	// The access time will be updated when the entry is re-validated or during eviction
	sv.cacheMutex.RUnlock()

	return cacheKey, cached && entry.validated
}

// cacheValidationWithKey marks a JSON string as successfully validated using a pre-computed key
// PERFORMANCE: Accepts pre-computed cache key to avoid double hash computation for large JSON
// SECURITY FIX: Uses LRU-style eviction at 80% capacity to prevent memory spikes
func (sv *securityValidator) cacheValidationWithKey(cacheKey string) {
	sv.cacheMutex.Lock()
	defer sv.cacheMutex.Unlock()

	// SAFETY: Skip caching after Close()
	if sv.validationCache == nil {
		return
	}

	// SECURITY FIX: Proactive cleanup at 80% capacity instead of 100%
	const cacheHighWatermark = securityCacheHighWatermark
	if len(sv.validationCache) >= cacheHighWatermark {
		sv.evictLRUEntries()
	}

	sv.validationCache[cacheKey] = &validationCacheEntry{
		validated:  true,
		lastAccess: time.Now().Unix(),
	}
}

// evictLRUEntries removes oldest 25% of entries using LRU strategy
// SECURITY: Intelligent LRU eviction for validation cache
func (sv *securityValidator) evictLRUEntries() {
	if len(sv.validationCache) == 0 {
		return
	}

	// Collect all entries with their access times
	type entryWithTime struct {
		key        string
		lastAccess int64
	}

	entries := make([]entryWithTime, 0, len(sv.validationCache))
	for k, v := range sv.validationCache {
		entries = append(entries, entryWithTime{key: k, lastAccess: v.lastAccess})
	}

	// Sort by access time (oldest first)
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].lastAccess < entries[j].lastAccess
	})

	// Remove oldest 25% instead of 50% to reduce cache thrashing
	toRemove := max(len(entries)/4, 1)

	for i := 0; i < toRemove && i < len(entries); i++ {
		delete(sv.validationCache, entries[i].key)
	}
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

func (sv *securityValidator) validateJSONSecurity(jsonStr string) error {
	// Fast path: check for null bytes first (most critical)
	if strings.IndexByte(jsonStr, 0) != -1 {
		return newSecurityError("validate_json_security", "null byte injection detected")
	}

	// Fast path: for small JSON strings, use the original approach
	// For large JSON strings (>4KB), use a samplingapproach
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
	if !hasLetters {
		return nil
	}

	// Use efficient combined scanning for dangerous patterns
	// Check multiple patterns in a single pass where possible
	return sv.validateJSONSecurityOptimized(jsonStr)
}

// validateJSONSecurityFull performs full security validation for small JSON strings
// PERFORMANCE: Optimized to scan all patterns in a single pass
func (sv *securityValidator) validateJSONSecurityFull(jsonStr string) error {
	// SECURITY: Always scan critical patterns in full - these cannot be bypassed
	// Critical patterns: __proto__, constructor[, prototype.
	// PERFORMANCE: Use combined check instead of multiple Contains calls
	if strings.Contains(jsonStr, "__") {
		if strings.Contains(jsonStr, "__proto__") {
			return newSecurityError("validate_json_security", "dangerous pattern: prototype pollution")
		}
	}
	if strings.Contains(jsonStr, "constructor") {
		if strings.Contains(jsonStr, "constructor[") {
			return newSecurityError("validate_json_security", "dangerous pattern: constructor access")
		}
	}
	if strings.Contains(jsonStr, "prototype") {
		if strings.Contains(jsonStr, "prototype.") {
			return newSecurityError("validate_json_security", "dangerous pattern: prototype manipulation")
		}
	}

	// PERFORMANCE: Group patterns by their first character for efficient scanning
	// This allows us to scan the string once and check multiple patterns at each position

	// Pre-check: if no HTML/XML tags or function calls exist, skip expensive pattern matching
	// PERFORMANCE: Use IndexByte for fast single-character search instead of loop
	hasAngleBracket := strings.IndexByte(jsonStr, '<') != -1
	hasFunctionCall := strings.IndexByte(jsonStr, '(') != -1

	// If neither exists, the JSON is very likely safe
	// Most JSON data doesn't contain < or (, so this fast path is common
	if !hasAngleBracket && !hasFunctionCall {
		// Only check for a few critical patterns that don't require < or (
		// Use strings.Contains which is much faster than case-insensitive search
		if strings.Contains(jsonStr, "javascript:") ||
			strings.Contains(jsonStr, "vbscript:") ||
			strings.Contains(jsonStr, "data:") {
			return newSecurityError("validate_json_security", "dangerous protocol pattern detected")
		}
		return nil
	}

	// PERFORMANCE: For JSON with < or (, use smarter scanning
	// Most legitimate JSON with these characters is still safe
	// We only need to scan for actual dangerous patterns,
	// Check HTML/XSS related patterns (require '<')
	// SECURITY: Use case-insensitive matching to prevent bypass via mixed case (e.g., <Script>)
	if hasAngleBracket {
		lower := strings.ToLower(jsonStr)
		htmlPatterns := []string{"<script", "<iframe", "<object", "<embed", "<svg", "onerror", "onload", "onclick"}
		for _, pattern := range htmlPatterns {
			if strings.Contains(lower, pattern) {
				return newSecurityError("validate_json_security", fmt.Sprintf("dangerous HTML pattern: %s", pattern))
			}
		}
	}

	// Check function-related patterns (require '(')
	// SECURITY: Case-insensitive to prevent bypass via mixed case
	if hasFunctionCall {
		lower := strings.ToLower(jsonStr)
		funcPatterns := []string{"eval(", "function(", "settimeout(", "setinterval(", "new function("}
		for _, pattern := range funcPatterns {
			if strings.Contains(lower, pattern) {
				return newSecurityError("validate_json_security", fmt.Sprintf("dangerous function pattern: %s", pattern))
			}
		}
	}

	return nil
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
// SECURITY RECOMMENDATION: Use FullSecurityScan=true for maximum performance when
// processing trusted internal data. The optimized mode now provides full coverage.
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
	// If none of these exist, we can skip the expensive pattern matching
	if !sv.hasIndicatorChars(jsonStr) {
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

	// Add safety margin to the pre-computed max pattern length
	overlapSize := maxDangerousPatternLen() + 8

	// Window size tuned for cache efficiency
	windowSize := securityScanWindowSize

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
		offset += windowSize - overlapSize

		// Ensure we make progress and don't infinite loop
		if offset <= 0 {
			offset = end
		}
	}

	// Additional check - scan for pattern fragments that might indicate attacks
	if sv.hasPatternFragments(jsonStr) {
		return sv.scanWindowForPatterns(jsonStr)
	}

	return nil
}

// scanWindowForPatterns scans a single window for all dangerous patterns
// SECURITY FIX: Extracted to ensure consistent scanning logic
func (sv *securityValidator) scanWindowForPatterns(window string) error {
	for _, dp := range dangerousPatterns {
		if idx := fastIndexIgnoreCase(window, dp.pattern); idx != -1 {
			if sv.isDangerousContextIgnoreCase(window, idx, len(dp.pattern)) {
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

// hasPatternFragments checks for partial dangerous patterns that might indicate
// an attempt to hide malicious code
// SECURITY FIX: Expanded fragment list for better detection coverage
func (sv *securityValidator) hasPatternFragments(jsonStr string) bool {
	// Check for partial patterns that might be completed elsewhere
	// SECURITY FIX: Expanded list to catch more attack variants
	fragments := []string{
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

	for _, frag := range fragments {
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
	for start < end && isWhitespace(jsonStr[start]) {
		start++
	}
	// Skip trailing whitespace
	for end > start && isWhitespace(jsonStr[end-1]) {
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

// isWhitespace checks if a byte is JSON whitespace
func isWhitespace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r'
}

func (sv *securityValidator) validateNestingDepth(jsonStr string) error {
	// SECURITY: Validate nesting depth for all inputs regardless of size.
	// Use a faster scan for small JSON (< 64KB) by only checking depth,
	// and full scan for larger inputs that also track total brackets.
	// Small but deeply nested JSON can still cause stack overflow during processing.
	if len(jsonStr) < securityNestingValidationThreshold {
		// Fast path for small JSON: only check max depth, no bracket counting
		depth := 0
		inString := false
		escaped := false
		maxCheckDepth := sv.maxNestingDepth
		if maxCheckDepth <= 0 {
			maxCheckDepth = 100
		}
		for i := 0; i < len(jsonStr); i++ {
			c := jsonStr[i]
			if escaped {
				escaped = false
				continue
			}
			if inString {
				if c == byte(0x5c) {
					escaped = true
				} else if c == '"' {
					inString = false
				}
				continue
			}
			switch c {
			case '"':
				inString = true
			case '{', '[':
				depth++
				if depth > maxCheckDepth {
					return newOperationError("validate_nesting_depth",
						fmt.Sprintf("nesting depth %d exceeds maximum %d", depth, maxCheckDepth), ErrDepthLimit)
				}
			case '}', ']':
				depth--
			case byte(0x5c):
				if inString {
					escaped = true
				}
			}
		}
		return nil
	}

	depth := 0
	inString := false
	escaped := false
	maxCheckDepth := sv.maxNestingDepth
	if maxCheckDepth <= 0 {
		maxCheckDepth = 100 // Default max depth
	}

	// SECURITY: Track total bracket count to prevent DoS attacks
	// Attackers can create shallow but massive bracket structures
	// Set limit high enough for normal use but prevent excessive structures
	totalBrackets := 0
	maxTotalBrackets := securityMaxTotalBrackets

	// SECURITY: Track consecutive opening brackets for anomaly detection
	consecutiveOpens := 0
	maxConsecutiveOpens := securityMaxConsecutiveOpens

	// Use byte-level iteration for better performance
	// Check all JSON regardless of size to prevent depth-based attacks
	for i := 0; i < len(jsonStr); i++ {
		c := jsonStr[i]

		if escaped {
			escaped = false
			continue
		}

		switch c {
		case '\\':
			if inString {
				escaped = true
			}
		case '"':
			inString = !inString
		case '{', '[':
			if !inString {
				depth++
				totalBrackets++
				consecutiveOpens++

				// SECURITY: Check for too many consecutive opens (potential attack)
				if consecutiveOpens > maxConsecutiveOpens {
					return newOperationError("validate_nesting_depth",
						fmt.Sprintf("too many consecutive opening brackets at position %d", i), ErrDepthLimit)
				}

				if depth > maxCheckDepth {
					return newOperationError("validate_nesting_depth",
						fmt.Sprintf("nesting depth %d exceeds maximum %d", depth, maxCheckDepth), ErrDepthLimit)
				}

				// SECURITY: Check total bracket count
				if totalBrackets > maxTotalBrackets {
					return newOperationError("validate_nesting_depth",
						fmt.Sprintf("total bracket count %d exceeds maximum %d", totalBrackets, maxTotalBrackets), ErrDepthLimit)
				}
			}
		case '}', ']':
			if !inString {
				depth--
				totalBrackets++
				consecutiveOpens = 0 // Reset on closing bracket
			}
		default:
			consecutiveOpens = 0 // Reset on non-bracket character
		}
	}

	// SECURITY: Check for unbalanced brackets
	if depth != 0 {
		return newOperationError("validate_nesting_depth",
			"unbalanced brackets in JSON structure", ErrInvalidJSON)
	}

	return nil
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
func (sv *securityValidator) containsSensitivePatterns(s string) bool {
	// Fast lowercase conversion and check
	s = strings.ToLower(s)
	for _, pattern := range sensitivePatterns {
		if strings.Contains(s, pattern) {
			return true
		}
	}
	return false
}
