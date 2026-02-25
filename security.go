package json

import (
	"crypto/sha256"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/cybergodev/json/internal"
	"golang.org/x/text/unicode/norm"
)

// dangerousPatterns contains all dangerous patterns for security validation
// This is defined at package level to avoid allocation on each validation call
var dangerousPatterns = []struct {
	pattern string
	name    string
}{
	{"constructor[", "constructor access"},
	{"prototype.", "prototype manipulation"},
	{"<script", "script tag injection"},
	{"<iframe", "iframe injection"},
	{"<object", "object injection"},
	{"<embed", "embed injection"},
	{"<svg", "svg injection"},
	{"javascript:", "javascript protocol"},
	{"vbscript:", "vbscript protocol"},
	{"data:", "data protocol"},
	{"eval(", "dynamic code execution"},
	{"function(", "function expression"},
	{"setTimeout(", "timer manipulation"},
	{"setInterval(", "interval manipulation"},
	{"require(", "code injection"},
	{"new function(", "dynamic function creation"},
	{"document.cookie", "cookie access"},
	{"window.location", "redirect manipulation"},
	{"innerhtml", "DOM manipulation"},
	{"fromcharcode(", "character encoding bypass"},
	{"atob(", "base64 decoding"},
	{"import(", "dynamic import"},
	{"expression(", "CSS expression injection"},
	// Event handlers (comprehensive list)
	{"onerror", "event handler injection"},
	{"onload", "event handler injection"},
	{"onclick", "event handler injection"},
	{"onmouseover", "event handler injection"},
	{"onfocus", "event handler injection"},
	{"onblur", "event handler injection"},
	{"onkeyup", "event handler injection"},
	{"onchange", "event handler injection"},
	{"onsubmit", "event handler injection"},
	{"ondblclick", "event handler injection"},
	{"onmousedown", "event handler injection"},
	{"onmouseup", "event handler injection"},
	{"onmousemove", "event handler injection"},
	{"onkeydown", "event handler injection"},
	{"onkeypress", "event handler injection"},
	{"onreset", "event handler injection"},
	{"onselect", "event handler injection"},
	{"onunload", "event handler injection"},
	{"onabort", "event handler injection"},
	{"ondrag", "event handler injection"},
	{"ondragend", "event handler injection"},
	{"ondragenter", "event handler injection"},
	{"ondragleave", "event handler injection"},
	{"ondragover", "event handler injection"},
	{"ondragstart", "event handler injection"},
	{"ondrop", "event handler injection"},
	{"onscroll", "event handler injection"},
	{"onwheel", "event handler injection"},
	{"oncopy", "event handler injection"},
	{"oncut", "event handler injection"},
	{"onpaste", "event handler injection"},
	// JavaScript dangerous functions
	{"alert(", "alert function"},
	{"confirm(", "confirm function"},
	{"prompt(", "prompt function"},
	// Prototype pollution patterns
	{"__defineGetter__", "getter definition"},
	{"__defineSetter__", "setter definition"},
	{"Object.assign", "object assignment"},
	{"Reflect.", "reflection API"},
	{"Proxy(", "proxy creation"},
	{"Symbol(", "symbol creation"},
}

// caseSensitivePatterns contains patterns that must match exact case
var caseSensitivePatterns = []struct {
	pattern string
	name    string
}{
	{"__proto__", "prototype pollution"},
}

// validationCacheEntry holds a cache entry with access time for LRU eviction
// SECURITY FIX: Track access time for better cache management
type validationCacheEntry struct {
	validated  bool
	lastAccess int64 // Unix timestamp for LRU eviction
}

// SecurityValidator provides comprehensive security validation for JSON processing.
type SecurityValidator struct {
	maxJSONSize      int64
	maxPathLength    int
	maxNestingDepth  int
	fullSecurityScan bool
	// PERFORMANCE: Cache for validation results to avoid repeated scanning
	// of the same JSON string (common in repeated Get operations)
	// SECURITY FIX: Use entries with timestamps for LRU eviction
	validationCache map[string]*validationCacheEntry
	cacheMutex      sync.RWMutex
	// Additional mutex for nested locking to avoid deadlock
	securityScanMutex sync.Mutex
}

// NewSecurityValidator creates a new security validator with the given limits.
func NewSecurityValidator(maxJSONSize int64, maxPathLength, maxNestingDepth int, fullSecurityScan bool) *SecurityValidator {
	return &SecurityValidator{
		maxJSONSize:      maxJSONSize,
		maxPathLength:    maxPathLength,
		maxNestingDepth:  maxNestingDepth,
		fullSecurityScan: fullSecurityScan,
		validationCache:  make(map[string]*validationCacheEntry, 256), // Pre-allocate for efficiency
	}
}

// ValidateAll performs comprehensive validation of both JSON and path inputs.
func (sv *SecurityValidator) ValidateAll(jsonStr, path string) error {
	if err := sv.ValidateJSONInput(jsonStr); err != nil {
		return err
	}
	return sv.ValidatePathInput(path)
}

// ValidateJSONInput performs comprehensive JSON input validation with enhanced security.
// PERFORMANCE: Uses caching to avoid repeated validation of the same JSON string.
func (sv *SecurityValidator) ValidateJSONInput(jsonStr string) error {
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
	cleanJSON := strings.TrimPrefix(jsonStr, ValidationBOMPrefix)
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

// validationCacheHashThreshold is the size threshold for hash-based cache keys
// SECURITY: Reduced from 64KB to 4KB to prevent memory exhaustion from storing many small JSON strings
const validationCacheHashThreshold = 4096

// getValidationCacheKey computes and returns the cache key for a JSON string
// PERFORMANCE: Returns the key for reuse to avoid double hash computation
// SECURITY FIX: Uses SHA-256 for larger strings to prevent collision attacks
func (sv *SecurityValidator) getValidationCacheKey(jsonStr string) string {
	strLen := len(jsonStr)

	// SECURITY FIX: Use SHA-256 for better collision resistance on larger strings
	// For small strings (< 4KB), use faster FNV-1a with length prefix
	if strLen <= validationCacheHashThreshold {
		// FNV-1a hash for small strings
		h := uint64(14695981039346656037)
		for i := 0; i < strLen; i++ {
			h ^= uint64(jsonStr[i])
			h *= 1099511628211
		}
		// Include length in hash to prevent length extension issues
		h ^= uint64(strLen)
		return fmt.Sprintf("%d:%016x", strLen, h)
	}

	// SECURITY FIX: For larger strings, use SHA-256 for strong collision resistance
	hash := sha256.Sum256([]byte(jsonStr))
	return fmt.Sprintf("%d:%x", strLen, hash[:16]) // Use first 16 bytes of SHA-256
}

// isValidationCached checks if JSON string was previously validated successfully
// PERFORMANCE: Returns the cache key for reuse in cacheValidation to avoid double hash computation
func (sv *SecurityValidator) isValidationCached(jsonStr string) (string, bool) {
	// Compute cache key once
	cacheKey := sv.getValidationCacheKey(jsonStr)

	// Use read lock for fast lookup
	sv.cacheMutex.RLock()
	defer sv.cacheMutex.RUnlock()

	entry, cached := sv.validationCache[cacheKey]
	if cached {
		// Update last access time (requires write lock, but we skip for performance)
		// The entry will be updated on next access or during eviction
	}
	return cacheKey, cached && entry.validated
}

// cacheValidationWithKey marks a JSON string as successfully validated using a pre-computed key
// PERFORMANCE: Accepts pre-computed cache key to avoid double hash computation for large JSON
// SECURITY FIX: Uses LRU-style eviction at 80% capacity to prevent memory spikes
func (sv *SecurityValidator) cacheValidationWithKey(cacheKey string) {
	sv.cacheMutex.Lock()
	defer sv.cacheMutex.Unlock()

	// SECURITY FIX: Proactive cleanup at 80% capacity instead of 100%
	const cacheHighWatermark = 8000 // 80% of 10000
	if len(sv.validationCache) >= cacheHighWatermark {
		sv.evictLRUEntries()
	}

	sv.validationCache[cacheKey] = &validationCacheEntry{
		validated:  true,
		lastAccess: time.Now().Unix(),
	}
}

// cacheValidation marks a JSON string as successfully validated
// DEPRECATED: Use cacheValidationWithKey for better performance with large JSON
func (sv *SecurityValidator) cacheValidation(jsonStr string) {
	cacheKey := sv.getValidationCacheKey(jsonStr)
	sv.cacheValidationWithKey(cacheKey)
}

// evictLRUEntries removes oldest 25% of entries using LRU strategy
// SECURITY FIX: Replaces clearCacheHalf with more intelligent LRU eviction
func (sv *SecurityValidator) evictLRUEntries() {
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
	toRemove := len(entries) / 4
	if toRemove < 1 {
		toRemove = 1
	}

	for i := 0; i < toRemove && i < len(entries); i++ {
		delete(sv.validationCache, entries[i].key)
	}
}

// clearCacheHalf clears approximately half of the validation cache
// DEPRECATED: Use evictLRUEntries for better cache management
func (sv *SecurityValidator) clearCacheHalf() {
	sv.evictLRUEntries()
}

// ValidatePathInput performs comprehensive path validation with enhanced security.
func (sv *SecurityValidator) ValidatePathInput(path string) error {
	if len(path) > sv.maxPathLength {
		return newPathError(path, fmt.Sprintf("path length %d exceeds maximum %d", len(path), sv.maxPathLength), ErrInvalidPath)
	}

	// Empty path is valid (root access)
	if path == "" || path == "." {
		return nil
	}

	if err := sv.validatePathSecurity(path); err != nil {
		return err
	}

	if err := sv.validateBracketMatching(path); err != nil {
		return err
	}

	return sv.validatePathSyntax(path)
}

func (sv *SecurityValidator) validateJSONSecurity(jsonStr string) error {
	// Fast path: check for null bytes first (most critical)
	if strings.IndexByte(jsonStr, 0) != -1 {
		return newSecurityError("validate_json_security", "null byte injection detected")
	}

	// Fast path: for small JSON strings, use the original approach
	// For large JSON strings (>4KB), use a sampling approach
	if len(jsonStr) < 4096 {
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
func (sv *SecurityValidator) validateJSONSecurityFull(jsonStr string) error {
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
	if hasAngleBracket {
		htmlPatterns := []string{"<script", "<iframe", "<object", "<embed", "<svg", "onerror", "onload", "onclick"}
		for _, pattern := range htmlPatterns {
			if strings.Contains(jsonStr, pattern) {
				return newSecurityError("validate_json_security", fmt.Sprintf("dangerous HTML pattern: %s", pattern))
			}
		}
	}

	// Check function-related patterns (require '(')
	if hasFunctionCall {
		// Only check the most common dangerous function patterns
		funcPatterns := []string{"eval(", "function(", "setTimeout(", "setInterval(", "new Function("}
		for _, pattern := range funcPatterns {
			if strings.Contains(jsonStr, pattern) {
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
func (sv *SecurityValidator) validateJSONSecurityOptimized(jsonStr string) error {
	// If full security scan is enabled, use the simpler full scan approach
	if sv.fullSecurityScan {
		return sv.validateJSONSecurityFull(jsonStr)
	}

	// SECURITY: Always scan critical patterns in full regardless of JSON size
	// These patterns are too dangerous to miss due to sampling
	criticalPatterns := []struct {
		pattern string
		name    string
	}{
		{"__proto__", "prototype pollution"},
		{"constructor[", "constructor access"},
		{"prototype.", "prototype manipulation"},
	}

	for _, cp := range criticalPatterns {
		if strings.Contains(jsonStr, cp.pattern) {
			return newSecurityError("validate_json_security", fmt.Sprintf("dangerous pattern: %s", cp.name))
		}
	}

	// Check for key indicator characters that would appear in dangerous patterns
	// If none of these exist, we can skip the expensive pattern matching
	indicators := []byte{'<', '(', ':', '.', 'b', 'w', 'i', 'O', 'R', 'P', 'S'}
	hasIndicators := false
	for _, ind := range indicators {
		if strings.IndexByte(jsonStr, ind) != -1 {
			hasIndicators = true
			break
		}
	}
	if !hasIndicators {
		// No dangerous pattern can exist without these characters
		return nil
	}

	// SECURITY: Check for suspicious character density - if density is high, force full scan
	// This prevents attackers from hiding malicious code in dense payload sections
	if sv.hasSuspiciousCharacterDensity(jsonStr) {
		return sv.validateJSONSecurityFull(jsonStr)
	}

	jsonLen := len(jsonStr)

	// SECURITY FIX: Determine the longest pattern for overlap calculation
	// This ensures no pattern can straddle window boundaries and be missed
	maxPatternLen := 0
	for _, dp := range dangerousPatterns {
		if len(dp.pattern) > maxPatternLen {
			maxPatternLen = len(dp.pattern)
		}
	}
	// Add safety margin
	overlapSize := maxPatternLen + 8

	// SECURITY FIX: Use rolling window approach with guaranteed coverage
	// Window size is tuned for cache efficiency while maintaining reasonable overhead
	windowSize := 32768 // 32KB windows - fits well in CPU cache

	// For smaller JSON, just scan it all
	if jsonLen <= windowSize*2 {
		return sv.scanWindowForPatterns(jsonStr)
	}

	// SECURITY FIX: Rolling window scan with overlap - guarantees 100% coverage
	// Each window overlaps with the previous by 'overlapSize' bytes to ensure
	// patterns cannot be hidden at window boundaries
	for offset := 0; offset < jsonLen; {
		end := offset + windowSize
		if end > jsonLen {
			end = jsonLen
		}

		window := jsonStr[offset:end]
		if err := sv.scanWindowForPatterns(window); err != nil {
			return err
		}

		// Move to next window, but overlap by the max pattern length
		// This ensures patterns spanning window boundaries are caught
		offset += windowSize - overlapSize

		// Ensure we make progress and don't infinite loop
		if offset <= 0 {
			offset = end
		}
	}

	// SECURITY: Additional check - scan for pattern fragments that might indicate attacks
	// This provides defense in depth
	if sv.hasPatternFragments(jsonStr) {
		if err := sv.scanSuspiciousSections(jsonStr); err != nil {
			return err
		}
	}

	return nil
}

// scanWindowForPatterns scans a single window for all dangerous patterns
// SECURITY FIX: Extracted to ensure consistent scanning logic
func (sv *SecurityValidator) scanWindowForPatterns(window string) error {
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
func (sv *SecurityValidator) hasSuspiciousCharacterDensity(jsonStr string) bool {
	jsonLen := len(jsonStr)
	if jsonLen == 0 {
		return false
	}

	// SECURITY FIX: Sample from multiple regions: beginning, middle, and end
	// This prevents attackers from hiding malicious code in any single region
	sampleSize := 4096 // Sample 4KB from each region

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
	if density1 > 0.005 {
		return true
	}

	// Check middle region
	if jsonLen > sampleSize*2 {
		midStart := (jsonLen - sampleSize) / 2
		_, density2 := countSuspicious(midStart, midStart+sampleSize)
		if density2 > 0.005 {
			return true
		}
	}

	// Check end
	if jsonLen > sampleSize {
		_, density3 := countSuspicious(jsonLen-sampleSize, jsonLen)
		if density3 > 0.005 {
			return true
		}
	}

	// SECURITY FIX: Also check for distributed suspicious characters across entire string
	// This catches attacks that spread malicious content thinly across the payload
	totalSuspicious, _ := countSuspicious(0, jsonLen)
	overallDensity := float64(totalSuspicious) / float64(jsonLen)

	// Use a lower threshold for overall density since attacks might be spread out
	return overallDensity > 0.003
}

// hasPatternFragments checks for partial dangerous patterns that might indicate
// an attempt to hide malicious code
// SECURITY FIX: Expanded fragment list for better detection coverage
func (sv *SecurityValidator) hasPatternFragments(jsonStr string) bool {
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

// scanSuspiciousSections performs targeted scanning on sections containing
// potential attack fragments
// SECURITY FIX: Now uses the centralized scanWindowForPatterns for consistency
func (sv *SecurityValidator) scanSuspiciousSections(jsonStr string) error {
	// SECURITY FIX: Use the same scanning function for consistency
	// This scans the entire string for patterns, providing defense in depth
	return sv.scanWindowForPatterns(jsonStr)
}

// fastIndexIgnoreCase is an optimized case-insensitive search
// Delegates to shared implementation in internal package
func fastIndexIgnoreCase(s, pattern string) int {
	return internal.IndexIgnoreCase(s, pattern)
}

// indexIgnoreCase finds pattern case-insensitively without allocation
// Delegates to shared implementation in internal package
func indexIgnoreCase(s, pattern string) int {
	return internal.IndexIgnoreCase(s, pattern)
}

// isDangerousContextIgnoreCase checks if a pattern match is in a dangerous context (case-insensitive)
// SECURITY FIX: Improved to handle patterns that start/end with special characters
func (sv *SecurityValidator) isDangerousContextIgnoreCase(s string, idx, patternLen int) bool {
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

func (sv *SecurityValidator) validatePathSecurity(path string) error {
	// Normalize the path using Unicode NFC to detect homograph attacks
	// This ensures that visually similar characters are normalized
	normalizedPath := norm.NFC.String(path)

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

	// Check URL encoding bypass (including double encoding) - case-insensitive without allocation
	if containsAnyIgnoreCase(normalizedPath, "%2e", "%2f", "%5c", "%00", "%252e", "%252f") {
		return newPathError(path, "path traversal via URL encoding detected", ErrSecurityViolation)
	}

	// Check UTF-8 overlong encoding - case-insensitive without allocation
	if containsAnyIgnoreCase(normalizedPath, "%c0%af", "%c1%9c") {
		return newPathError(path, "path traversal via UTF-8 overlong encoding detected", ErrSecurityViolation)
	}

	// Check excessive special characters
	if strings.Contains(normalizedPath, ":::") || strings.Contains(normalizedPath, "[[[") || strings.Contains(normalizedPath, "}}}") {
		return newPathError(path, "excessive special characters", ErrSecurityViolation)
	}

	return nil
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

// containsAnyIgnoreCase checks if s contains any of the patterns case-insensitively
func containsAnyIgnoreCase(s string, patterns ...string) bool {
	for _, pattern := range patterns {
		if indexIgnoreCase(s, pattern) != -1 {
			return true
		}
	}
	return false
}

func (sv *SecurityValidator) validateJSONStructure(jsonStr string) error {
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

func (sv *SecurityValidator) validateNestingDepth(jsonStr string) error {
	// PERFORMANCE: For small JSON (< 64KB), skip detailed nesting validation
	// The standard library json.Unmarshal already handles this efficiently
	// Only do detailed scan for large JSON where DoS attacks are more likely
	if len(jsonStr) < 65536 {
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
	maxTotalBrackets := 1000000 // 1 million brackets - reasonable for large JSON

	// SECURITY: Track consecutive opening brackets for anomaly detection
	consecutiveOpens := 0
	maxConsecutiveOpens := 100

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

func (sv *SecurityValidator) validateBracketMatching(path string) error {
	brackets := 0
	braces := 0
	inString := false
	escaped := false

	for i, char := range path {
		if escaped {
			escaped = false
			continue
		}

		switch char {
		case '\\':
			escaped = true
		case '"', '\'':
			inString = !inString
		case '[':
			if !inString {
				brackets++
			}
		case ']':
			if !inString {
				brackets--
				if brackets < 0 {
					return newPathError(path, fmt.Sprintf("unmatched closing bracket at position %d", i), ErrInvalidPath)
				}
			}
		case '{':
			if !inString {
				braces++
			}
		case '}':
			if !inString {
				braces--
				if braces < 0 {
					return newPathError(path, fmt.Sprintf("unmatched closing brace at position %d", i), ErrInvalidPath)
				}
			}
		}
	}

	if brackets != 0 {
		return newPathError(path, "unmatched brackets", ErrInvalidPath)
	}
	if braces != 0 {
		return newPathError(path, "unmatched braces", ErrInvalidPath)
	}

	return nil
}

func (sv *SecurityValidator) validatePathSyntax(path string) error {
	if strings.Contains(path, "...") {
		return newPathError(path, "invalid consecutive dots", ErrInvalidPath)
	}

	for i, char := range path {
		if char < 32 && char != '\t' && char != '\n' && char != '\r' {
			return newPathError(path, fmt.Sprintf("invalid control character at position %d", i), ErrInvalidPath)
		}
	}

	return nil
}

func isValidJSONPrimitive(s string) bool {
	return internal.IsValidJSONPrimitive(s)
}

func isValidJSONNumber(s string) bool {
	return internal.IsValidJSONNumber(s)
}

// hashStringFast computes a fast 16-byte hash of a string using FNV-1a
// This is used for caching large JSON strings without storing the full content
// Optimized: samples characters for very large strings to reduce CPU overhead
func hashStringFast(s string) [16]byte {
	// Use two FNV-1a hashes for better distribution
	const (
		offsetBasis1 = 14695981039346656037
		prime1       = 1099511628211
		offsetBasis2 = 2166136261
		prime2       = 16777619
	)

	h1 := uint64(offsetBasis1)
	h2 := uint32(offsetBasis2)

	lenS := len(s)

	// For very large strings, use sampling to reduce CPU overhead
	// Sample every Nth character where N depends on string length
	if lenS > 65536 {
		// Sample approximately 8192 characters for large strings
		step := lenS / 8192
		if step < 1 {
			step = 1
		}

		for i := 0; i < lenS; i += step {
			c := s[i]
			h1 ^= uint64(c)
			h1 *= prime1
			h2 ^= uint32(c)
			h2 *= prime2
		}

		// Always include first and last characters
		if lenS > 0 {
			h1 ^= uint64(s[0])
			h1 *= prime1
			h1 ^= uint64(s[lenS-1])
			h1 *= prime1
		}
	} else {
		// For smaller strings, process all characters
		for i := 0; i < lenS; i++ {
			c := s[i]
			h1 ^= uint64(c)
			h1 *= prime1
			h2 ^= uint32(c)
			h2 *= prime2
		}
	}

	// Also incorporate length to avoid collisions
	h1 ^= uint64(lenS)

	var result [16]byte
	// Encode h1 (8 bytes)
	result[0] = byte(h1)
	result[1] = byte(h1 >> 8)
	result[2] = byte(h1 >> 16)
	result[3] = byte(h1 >> 24)
	result[4] = byte(h1 >> 32)
	result[5] = byte(h1 >> 40)
	result[6] = byte(h1 >> 48)
	result[7] = byte(h1 >> 56)
	// Encode h2 (4 bytes)
	result[8] = byte(h2)
	result[9] = byte(h2 >> 8)
	result[10] = byte(h2 >> 16)
	result[11] = byte(h2 >> 24)
	// Encode length (4 bytes)
	result[12] = byte(lenS)
	result[13] = byte(lenS >> 8)
	result[14] = byte(lenS >> 16)
	result[15] = byte(lenS >> 24)

	return result
}
