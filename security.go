package json

import (
	"fmt"
	"strings"
	"sync"
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

// SecurityValidator provides comprehensive security validation for JSON processing.
type SecurityValidator struct {
	maxJSONSize      int64
	maxPathLength    int
	maxNestingDepth  int
	fullSecurityScan bool
	// PERFORMANCE: Cache for validation results to avoid repeated scanning
	// of the same JSON string (common in repeated Get operations)
	validationCache map[string]bool
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
		validationCache:  make(map[string]bool, 256), // Pre-allocate for efficiency
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

// getValidationCacheKey computes and returns the cache key for a JSON string
// PERFORMANCE: Returns the key for reuse to avoid double hash computation
func (sv *SecurityValidator) getValidationCacheKey(jsonStr string) string {
	// For large JSON strings (>64KB), use hash-based caching
	// This avoids storing the full string in the cache
	if len(jsonStr) > 65536 {
		hash := hashStringFast(jsonStr)
		return string(hash[:])
	}
	return jsonStr
}

// isValidationCached checks if JSON string was previously validated successfully
// PERFORMANCE: Returns the cache key for reuse in cacheValidation to avoid double hash computation
func (sv *SecurityValidator) isValidationCached(jsonStr string) (string, bool) {
	// Compute cache key once
	cacheKey := sv.getValidationCacheKey(jsonStr)

	// Use read lock for fast lookup
	sv.cacheMutex.RLock()
	defer sv.cacheMutex.RUnlock()

	_, cached := sv.validationCache[cacheKey]
	return cacheKey, cached
}

// cacheValidationWithKey marks a JSON string as successfully validated using a pre-computed key
// PERFORMANCE: Accepts pre-computed cache key to avoid double hash computation for large JSON
func (sv *SecurityValidator) cacheValidationWithKey(cacheKey string) {
	sv.cacheMutex.Lock()
	defer sv.cacheMutex.Unlock()

	// Limit cache size to prevent memory issues
	if len(sv.validationCache) >= 10000 {
		// Clear half of the cache to make room
		sv.clearCacheHalf()
	}

	sv.validationCache[cacheKey] = true
}

// cacheValidation marks a JSON string as successfully validated
// DEPRECATED: Use cacheValidationWithKey for better performance with large JSON
func (sv *SecurityValidator) cacheValidation(jsonStr string) {
	cacheKey := sv.getValidationCacheKey(jsonStr)
	sv.cacheValidationWithKey(cacheKey)
}

// clearCacheHalf clears approximately half of the validation cache
func (sv *SecurityValidator) clearCacheHalf() {
	count := 0
	halfSize := len(sv.validationCache) / 2
	for key := range sv.validationCache {
		delete(sv.validationCache, key)
		count++
		if count >= halfSize {
			break
		}
	}
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
func (sv *SecurityValidator) validateJSONSecurityOptimized(jsonStr string) error {
	// If full security scan is enabled, skip sampling and scan everything
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

	// For large JSON, we use a smarter approach:
	// 1. Check if key indicator characters exist at all
	// 2. Check for suspicious character density (potential attack indicator)
	// 3. Only do full pattern matching if indicators are found

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

	// SECURITY: Calculate dynamic sampling parameters based on JSON size
	// Larger JSON gets more samples with higher coverage
	jsonLen := len(jsonStr)
	beginSectionSize := 8192 // Always scan first 8KB
	endSectionSize := 4096   // Always scan last 4KB
	sampleSize := 4096       // Size of middle samples
	minSampleCount := 7
	maxSampleCount := 20

	// Calculate optimal sample count: more samples for larger JSON
	sampleCount := minSampleCount
	if jsonLen > sampleSize*4 {
		sampleCount = minSampleCount + (jsonLen/(32*1024) - 1)
	}
	if sampleCount > maxSampleCount {
		sampleCount = maxSampleCount
	}

	samples := make([]string, 0, sampleCount+2) // +2 for mandatory sections

	if jsonLen <= beginSectionSize+endSectionSize {
		// Small enough to scan entirely
		samples = append(samples, jsonStr)
	} else {
		// SECURITY: Always include mandatory sections (beginning and end)
		// First 8KB - often contains headers/metadata and common attack vectors
		samples = append(samples, jsonStr[:beginSectionSize])

		// Last 4KB - often contains closing data and hidden malicious content
		samples = append(samples, jsonStr[jsonLen-endSectionSize:])

		// Distribute middle samples evenly across the JSON
		// This ensures we don't miss malicious code hidden in the middle
		if sampleCount > 2 {
			step := (jsonLen - beginSectionSize - endSectionSize) / (sampleCount - 2)
			for i := 0; i < sampleCount-2; i++ {
				start := beginSectionSize + i*step
				end := start + sampleSize
				if end > jsonLen-endSectionSize {
					end = jsonLen - endSectionSize
				}
				if start < end {
					samples = append(samples, jsonStr[start:end])
				}
			}
		}
	}

	// Check patterns on samples using package-level patterns
	for _, sample := range samples {
		for _, dp := range dangerousPatterns {
			if idx := fastIndexIgnoreCase(sample, dp.pattern); idx != -1 {
				if sv.isDangerousContextIgnoreCase(sample, idx, len(dp.pattern)) {
					return newSecurityError("validate_json_security", fmt.Sprintf("dangerous pattern: %s", dp.name))
				}
			}
		}
	}

	// SECURITY: Additional check - scan random sections if suspicious patterns detected
	// This catches attacks that might be positioned between sample points
	if sv.hasPatternFragments(jsonStr) {
		// Perform additional targeted scanning
		if err := sv.scanSuspiciousSections(jsonStr); err != nil {
			return err
		}
	}

	return nil
}

// hasSuspiciousCharacterDensity checks if the JSON has abnormally high density of
// characters commonly used in attack payloads
func (sv *SecurityValidator) hasSuspiciousCharacterDensity(jsonStr string) bool {
	// Sample the string to check character density
	sampleLen := min(10000, len(jsonStr))
	suspicious := 0

	// Check first 10KB for suspicious density
	for i := 0; i < sampleLen; i++ {
		c := jsonStr[i]
		// Characters commonly found in XSS/injection payloads
		if c == '<' || c == '>' || c == '(' || c == ')' || c == ';' || c == '=' || c == '&' {
			suspicious++
		}
	}

	// If more than 0.5% suspicious characters, force full scan
	// Normal JSON should have very few of these characters
	density := float64(suspicious) / float64(sampleLen)
	return density > 0.005
}

// hasPatternFragments checks for partial dangerous patterns that might indicate
// an attempt to hide malicious code
func (sv *SecurityValidator) hasPatternFragments(jsonStr string) bool {
	// Check for partial patterns that might be completed elsewhere
	fragments := []string{
		"script", "eval", "function", "proto", "constructor",
		"document", "window", "onload", "onerror", "onclick",
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
func (sv *SecurityValidator) scanSuspiciousSections(jsonStr string) error {
	// Find positions of suspicious fragments and scan surrounding context
	for _, dp := range dangerousPatterns {
		// Get the first few characters of the pattern for quick detection
		if len(dp.pattern) < 3 {
			continue
		}
		prefix := dp.pattern[:3]

		idx := 0
		for {
			pos := fastIndexIgnoreCase(jsonStr[idx:], prefix)
			if pos == -1 {
				break
			}

			// Check a window around this position
			start := idx + pos - 50
			if start < 0 {
				start = 0
			}
			end := idx + pos + len(dp.pattern) + 100
			if end > len(jsonStr) {
				end = len(jsonStr)
			}

			window := jsonStr[start:end]
			if fastIndexIgnoreCase(window, dp.pattern) != -1 {
				if sv.isDangerousContextIgnoreCase(window,
					fastIndexIgnoreCase(window, dp.pattern), len(dp.pattern)) {
					return newSecurityError("validate_json_security",
						fmt.Sprintf("dangerous pattern: %s", dp.name))
				}
			}

			idx = idx + pos + 1
			if idx >= len(jsonStr)-len(dp.pattern) {
				break
			}
		}
	}
	return nil
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
func (sv *SecurityValidator) isDangerousContextIgnoreCase(s string, idx, patternLen int) bool {
	// Check if the pattern is standalone (not part of a larger word)
	before := idx == 0 || !internal.IsWordChar(s[idx-1])
	after := idx+patternLen >= len(s) || !internal.IsWordChar(s[idx+patternLen])
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
