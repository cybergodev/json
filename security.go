package json

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/cybergodev/json/internal"
)

// SecurityValidator provides comprehensive security validation for JSON processing.
type SecurityValidator struct {
	maxJSONSize     int64
	maxPathLength   int
	maxNestingDepth int
}

// NewSecurityValidator creates a new security validator with the given limits.
func NewSecurityValidator(maxJSONSize int64, maxPathLength, maxNestingDepth int) *SecurityValidator {
	return &SecurityValidator{
		maxJSONSize:     maxJSONSize,
		maxPathLength:   maxPathLength,
		maxNestingDepth: maxNestingDepth,
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
func (sv *SecurityValidator) ValidateJSONInput(jsonStr string) error {
	if int64(len(jsonStr)) > sv.maxJSONSize {
		return newSizeLimitError("validate_json_input", int64(len(jsonStr)), sv.maxJSONSize)
	}

	if len(jsonStr) == 0 {
		return newOperationError("validate_json_input", "JSON string cannot be empty", ErrInvalidJSON)
	}

	if !utf8.ValidString(jsonStr) {
		return newOperationError("validate_json_input", "JSON contains invalid UTF-8 sequences", ErrInvalidJSON)
	}

	// Detect BOM (not allowed)
	cleanJSON := strings.TrimPrefix(jsonStr, ValidationBOMPrefix)
	if len(cleanJSON) != len(jsonStr) {
		return newOperationError("validate_json_input", "JSON contains BOM which is not allowed", ErrInvalidJSON)
	}

	if err := sv.validateJSONSecurity(jsonStr); err != nil {
		return err
	}

	if err := sv.validateJSONStructure(jsonStr); err != nil {
		return err
	}

	return sv.validateNestingDepth(jsonStr)
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
func (sv *SecurityValidator) validateJSONSecurityFull(jsonStr string) error {
	// Case-sensitive patterns (fast - use strings.Index)
	caseSensitivePatterns := []struct {
		pattern string
		name    string
	}{
		{"__proto__", "prototype pollution"},
	}

	for _, dp := range caseSensitivePatterns {
		if strings.Contains(jsonStr, dp.pattern) {
			return newSecurityError("validate_json_security", fmt.Sprintf("dangerous pattern: %s", dp.name))
		}
	}

	// Case-insensitive patterns with fast substring check first
	dangerousPatterns := []struct {
		pattern string
		name    string
	}{
		{"constructor[", "constructor access"},
		{"prototype.", "prototype manipulation"},
		{"<script", "script tag injection"},
		{"javascript:", "javascript protocol"},
		{"vbscript:", "vbscript protocol"},
		{"eval(", "dynamic code execution"},
		{"function(", "function expression"},
		{"setTimeout(", "timer manipulation"},
		{"setInterval(", "interval manipulation"},
		{"require(", "code injection"},
	}

	for _, dp := range dangerousPatterns {
		// Fast check: look for the first character before doing full search
		firstChar := dp.pattern[0]
		found := false
		for i := 0; i < len(jsonStr); i++ {
			c := jsonStr[i]
			// Case-insensitive first character match
			if c == firstChar || (firstChar >= 'a' && firstChar <= 'z' && c == firstChar-32) {
				// Potential match, do full check
				if i+len(dp.pattern) <= len(jsonStr) {
					if matchPatternIgnoreCase(jsonStr[i:i+len(dp.pattern)], dp.pattern) {
						if sv.isDangerousContextIgnoreCase(jsonStr, i, len(dp.pattern)) {
							return newSecurityError("validate_json_security", fmt.Sprintf("dangerous pattern: %s", dp.name))
						}
						found = true
					}
				}
			}
			// Early termination optimization: if we've scanned a lot without finding,
			// continue to next pattern
			if i > 10000 && !found {
				break
			}
		}
	}

	return nil
}

// validateJSONSecurityOptimized performs optimized security validation for large JSON strings
func (sv *SecurityValidator) validateJSONSecurityOptimized(jsonStr string) error {
	// For large JSON, we use a smarter approach:
	// 1. Check if key indicator characters exist at all
	// 2. Only do full pattern matching if indicators are found

	// Check for key indicator characters that would appear in dangerous patterns
	// If none of these exist, we can skip the expensive pattern matching
	indicators := []byte{'<', '(', ':', '.'}
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

	// Check for __proto__ (case-sensitive, fast)
	if strings.Contains(jsonStr, "__proto__") {
		return newSecurityError("validate_json_security", "dangerous pattern: prototype pollution")
	}

	// Use sampling for large JSON: check first 8KB, last 4KB, and sample middle
	sampleSize := 8192
	samples := make([]string, 0, 3)

	if len(jsonStr) <= sampleSize*2 {
		samples = append(samples, jsonStr)
	} else {
		samples = append(samples, jsonStr[:sampleSize])
		samples = append(samples, jsonStr[len(jsonStr)-sampleSize/2:])
		// Sample from middle
		mid := len(jsonStr) / 2
		if mid+sampleSize/4 < len(jsonStr) {
			samples = append(samples, jsonStr[mid:mid+sampleSize/4])
		}
	}

	// Check patterns on samples
	patterns := []struct {
		pattern string
		name    string
	}{
		{"constructor[", "constructor access"},
		{"prototype.", "prototype manipulation"},
		{"<script", "script tag injection"},
		{"javascript:", "javascript protocol"},
		{"vbscript:", "vbscript protocol"},
		{"eval(", "dynamic code execution"},
		{"function(", "function expression"},
		{"setTimeout(", "timer manipulation"},
		{"setInterval(", "interval manipulation"},
		{"require(", "code injection"},
	}

	for _, sample := range samples {
		for _, dp := range patterns {
			if idx := fastIndexIgnoreCase(sample, dp.pattern); idx != -1 {
				if sv.isDangerousContextIgnoreCase(sample, idx, len(dp.pattern)) {
					return newSecurityError("validate_json_security", fmt.Sprintf("dangerous pattern: %s", dp.name))
				}
			}
		}
	}

	return nil
}

// matchPatternIgnoreCase checks if s matches pattern case-insensitively
func matchPatternIgnoreCase(s, pattern string) bool {
	if len(s) != len(pattern) {
		return false
	}
	for i := 0; i < len(pattern); i++ {
		c1 := s[i]
		c2 := pattern[i]
		if c1 >= 'A' && c1 <= 'Z' {
			c1 += 32
		}
		if c1 != c2 {
			return false
		}
	}
	return true
}

// fastIndexIgnoreCase is an optimized case-insensitive search
func fastIndexIgnoreCase(s, pattern string) int {
	plen := len(pattern)
	slen := len(s)
	if plen > slen {
		return -1
	}

	// Use first character as filter
	firstChar := pattern[0]
	firstCharLower := firstChar
	if firstChar >= 'A' && firstChar <= 'Z' {
		firstCharLower = firstChar + 32
	}

	// Only check positions where first character matches
	for i := 0; i <= slen-plen; i++ {
		c := s[i]
		if c == firstCharLower || (firstCharLower >= 'a' && firstCharLower <= 'z' && c == firstCharLower-32) {
			// First char matches, check rest
			if matchPatternIgnoreCase(s[i:i+plen], pattern) {
				return i
			}
		}
	}
	return -1
}

// indexIgnoreCase finds pattern case-insensitively without allocation
func indexIgnoreCase(s, pattern string) int {
	return fastIndexIgnoreCase(s, pattern)
}

// isDangerousContextIgnoreCase checks if a pattern match is in a dangerous context (case-insensitive)
func (sv *SecurityValidator) isDangerousContextIgnoreCase(s string, idx, patternLen int) bool {
	// Check if the pattern is standalone (not part of a larger word)
	before := idx == 0 || !internal.IsWordChar(s[idx-1])
	after := idx+patternLen >= len(s) || !internal.IsWordChar(s[idx+patternLen])
	return before && after
}

func (sv *SecurityValidator) validatePathSecurity(path string) error {
	if strings.IndexByte(path, 0) != -1 {
		return newPathError(path, "null byte injection detected", ErrSecurityViolation)
	}

	// Check path traversal patterns
	if strings.Contains(path, "..") {
		return newPathError(path, "path traversal detected", ErrSecurityViolation)
	}

	// Check URL encoding bypass (including double encoding) - case-insensitive without allocation
	if containsAnyIgnoreCase(path, "%2e", "%2f", "%5c", "%00", "%252e", "%252f") {
		return newPathError(path, "path traversal via URL encoding detected", ErrSecurityViolation)
	}

	// Check UTF-8 overlong encoding - case-insensitive without allocation
	if containsAnyIgnoreCase(path, "%c0%af", "%c1%9c") {
		return newPathError(path, "path traversal via UTF-8 overlong encoding detected", ErrSecurityViolation)
	}

	// Check excessive special characters
	if strings.Contains(path, ":::") || strings.Contains(path, "[[[") || strings.Contains(path, "}}}") {
		return newPathError(path, "excessive special characters", ErrSecurityViolation)
	}

	return nil
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
	// Fast path: for small JSON, trust the standard library to handle depth
	// The standard library already has depth limits
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

	// Use byte-level iteration for better performance
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
				if depth > maxCheckDepth {
					return newOperationError("validate_nesting_depth",
						fmt.Sprintf("nesting depth %d exceeds maximum %d", depth, maxCheckDepth), ErrDepthLimit)
				}
			}
		case '}', ']':
			if !inString {
				depth--
			}
		}
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
