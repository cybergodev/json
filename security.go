package json

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// SecurityValidator provides comprehensive security validation for JSON processing
type SecurityValidator struct {
	maxJSONSize     int64
	maxPathLength   int
	maxNestingDepth int
}

// NewSecurityValidator creates a new security validator with the given limits
func NewSecurityValidator(maxJSONSize int64, maxPathLength, maxNestingDepth int) *SecurityValidator {
	return &SecurityValidator{
		maxJSONSize:     maxJSONSize,
		maxPathLength:   maxPathLength,
		maxNestingDepth: maxNestingDepth,
	}
}

// ValidateJSONInput performs comprehensive JSON input validation with enhanced security
func (sv *SecurityValidator) ValidateJSONInput(jsonStr string) error {
	// Size validation
	if int64(len(jsonStr)) > sv.maxJSONSize {
		return newSizeLimitError("validate_json_input", int64(len(jsonStr)), sv.maxJSONSize)
	}

	// Empty input check
	if len(jsonStr) == 0 {
		return newOperationError("validate_json_input", "JSON string cannot be empty", ErrInvalidJSON)
	}

	// UTF-8 validation with BOM detection
	if !utf8.ValidString(jsonStr) {
		return newOperationError("validate_json_input", "JSON contains invalid UTF-8 sequences", ErrInvalidJSON)
	}

	// Remove BOM if present and validate
	cleanJSON := strings.TrimPrefix(jsonStr, "\uFEFF")
	if len(cleanJSON) != len(jsonStr) {
		return newOperationError("validate_json_input", "JSON contains BOM which is not allowed", ErrInvalidJSON)
	}

	// Enhanced security validation
	if err := sv.validateJSONSecurity(jsonStr); err != nil {
		return err
	}

	// Basic structure validation
	if err := sv.validateJSONStructure(jsonStr); err != nil {
		return err
	}

	// Nesting depth validation
	if err := sv.validateNestingDepth(jsonStr); err != nil {
		return err
	}

	return nil
}

// ValidatePathInput performs comprehensive path validation with enhanced security
func (sv *SecurityValidator) ValidatePathInput(path string) error {
	// Length validation
	if len(path) > sv.maxPathLength {
		return newPathError(path, fmt.Sprintf("path length %d exceeds maximum %d", len(path), sv.maxPathLength), ErrInvalidPath)
	}

	// Empty path is valid (root access)
	if path == "" || path == "." {
		return nil
	}

	// Enhanced security validation
	if err := sv.validatePathSecurity(path); err != nil {
		return err
	}

	// Bracket/brace matching validation
	if err := sv.validateBracketMatching(path); err != nil {
		return err
	}

	// Syntax validation
	if err := sv.validatePathSyntax(path); err != nil {
		return err
	}

	return nil
}

// validateJSONSecurity performs enhanced security validation for JSON input
func (sv *SecurityValidator) validateJSONSecurity(jsonStr string) error {
	// Check for null bytes (both literal and encoded)
	if strings.Contains(jsonStr, "\x00") {
		return newSecurityError("validate_json_security", "null byte injection detected")
	}

	// Check for excessive control characters
	controlCharCount := 0
	for _, r := range jsonStr {
		if r < 32 && r != '\t' && r != '\n' && r != '\r' {
			controlCharCount++
			if controlCharCount > 10 { // Allow some control chars but not excessive
				return newSecurityError("validate_json_security", "excessive control characters detected")
			}
		}
	}

	// Check for potential JSON injection patterns
	suspiciousPatterns := []string{
		"__proto__",
		"constructor",
		"prototype",
		"eval(",
		"Function(",
		"setTimeout(",
		"setInterval(",
	}

	lowerJSON := strings.ToLower(jsonStr)
	for _, pattern := range suspiciousPatterns {
		if strings.Contains(lowerJSON, pattern) {
			return newSecurityError("validate_json_security", fmt.Sprintf("suspicious pattern detected: %s", pattern))
		}
	}

	return nil
}

// validatePathSecurity performs enhanced security validation for paths
func (sv *SecurityValidator) validatePathSecurity(path string) error {
	// Convert to lowercase for case-insensitive detection
	lowerPath := strings.ToLower(path)

	// Enhanced path traversal patterns with comprehensive bypass detection
	dangerousPatterns := []string{
		"..",
		"%2e%2e", "%2E%2E", // URL-encoded ..
		"..%2f", "..%2F", // .. with encoded /
		"%2e%2e%2f", "%2E%2E%2F", // Fully encoded ../
		"..\\", "%2e%2e%5c", "%2E%2E%5C", // Windows-style with \
		"%252e", "%252E", // Double-encoded .
		"%c0%af", "%c1%9c", // UTF-8 overlong encoding
		"....//", "....\\\\", // Multiple dots with separators
		".%2e", "%2e.", // Mixed encoding
	}

	for _, pattern := range dangerousPatterns {
		if strings.Contains(lowerPath, pattern) {
			return newSecurityError("validate_path_security", fmt.Sprintf("path traversal pattern detected: %s", pattern))
		}
	}

	// Check for null bytes (both literal and encoded)
	if strings.Contains(path, "\x00") || strings.Contains(lowerPath, "%00") {
		return newSecurityError("validate_path_security", "null byte injection detected")
	}

	// Check for excessive consecutive special characters
	if strings.Contains(path, ":::") || strings.Contains(path, "[[[") || strings.Contains(path, "}}}") {
		return newSecurityError("validate_path_security", "excessive consecutive special characters detected")
	}

	// Check for Windows reserved device names
	windowsReserved := []string{
		"con", "prn", "aux", "nul",
		"com1", "com2", "com3", "com4", "com5", "com6", "com7", "com8", "com9",
		"lpt1", "lpt2", "lpt3", "lpt4", "lpt5", "lpt6", "lpt7", "lpt8", "lpt9",
	}

	pathParts := strings.FieldsFunc(lowerPath, func(r rune) bool {
		return r == '.' || r == '/' || r == '[' || r == ']' || r == '{' || r == '}'
	})

	for _, part := range pathParts {
		for _, reserved := range windowsReserved {
			if part == reserved {
				return newSecurityError("validate_path_security", fmt.Sprintf("Windows reserved device name detected: %s", reserved))
			}
		}
	}

	return nil
}

// validateJSONStructure performs fast structural validation
func (sv *SecurityValidator) validateJSONStructure(jsonStr string) error {
	trimmed := strings.TrimSpace(jsonStr)
	if len(trimmed) == 0 {
		return newOperationError("validate_json_structure", "JSON string is empty after trimming", ErrInvalidJSON)
	}

	first := trimmed[0]
	last := trimmed[len(trimmed)-1]

	// Check for valid JSON start/end characters
	switch first {
	case '{':
		if last != '}' {
			return newOperationError("validate_json_structure", "JSON object not properly closed", ErrInvalidJSON)
		}
	case '[':
		if last != ']' {
			return newOperationError("validate_json_structure", "JSON array not properly closed", ErrInvalidJSON)
		}
	case '"':
		if last != '"' || len(trimmed) < 2 {
			return newOperationError("validate_json_structure", "JSON string not properly closed", ErrInvalidJSON)
		}
	case 't', 'f':
		// Boolean values
		if !strings.HasPrefix(trimmed, "true") && !strings.HasPrefix(trimmed, "false") {
			return newOperationError("validate_json_structure", "invalid boolean value", ErrInvalidJSON)
		}
	case 'n':
		// Null value
		if !strings.HasPrefix(trimmed, "null") {
			return newOperationError("validate_json_structure", "invalid null value", ErrInvalidJSON)
		}
	default:
		// Should be a number
		if !sv.isValidNumberStart(rune(first)) {
			return newOperationError("validate_json_structure", "JSON starts with invalid character", ErrInvalidJSON)
		}
	}

	return nil
}

// validateNestingDepth validates JSON nesting depth efficiently
func (sv *SecurityValidator) validateNestingDepth(jsonStr string) error {
	depth := 0
	maxDepth := 0
	inString := false
	escaped := false

	for i, r := range jsonStr {
		if escaped {
			escaped = false
			continue
		}

		if inString {
			if r == '\\' {
				escaped = true
			} else if r == '"' {
				inString = false
			}
			continue
		}

		switch r {
		case '"':
			inString = true
		case '{', '[':
			depth++
			if depth > maxDepth {
				maxDepth = depth
			}
			if depth > sv.maxNestingDepth {
				return newDepthLimitError("validate_nesting_depth", depth, sv.maxNestingDepth)
			}
		case '}', ']':
			depth--
			if depth < 0 {
				return newOperationError("validate_nesting_depth", fmt.Sprintf("unmatched closing bracket at position %d", i), ErrInvalidJSON)
			}
		}
	}

	if depth != 0 {
		return newOperationError("validate_nesting_depth", "unmatched brackets in JSON", ErrInvalidJSON)
	}

	return nil
}

// validateBracketMatching validates bracket and brace matching
func (sv *SecurityValidator) validateBracketMatching(path string) error {
	var braceDepth, bracketDepth int
	inString := false
	escaped := false

	for i, r := range path {
		if escaped {
			escaped = false
			continue
		}

		if inString {
			if r == '\\' {
				escaped = true
			} else if r == '"' {
				inString = false
			}
			continue
		}

		switch r {
		case '"':
			inString = true
		case '{':
			braceDepth++
		case '}':
			braceDepth--
			if braceDepth < 0 {
				return newPathError(path, fmt.Sprintf("unmatched closing brace at position %d", i), ErrInvalidPath)
			}
		case '[':
			bracketDepth++
		case ']':
			bracketDepth--
			if bracketDepth < 0 {
				return newPathError(path, fmt.Sprintf("unmatched closing bracket at position %d", i), ErrInvalidPath)
			}
		}
	}

	if braceDepth > 0 {
		return newPathError(path, "unmatched opening brace", ErrInvalidPath)
	}
	if bracketDepth > 0 {
		return newPathError(path, "unmatched opening bracket", ErrInvalidPath)
	}

	return nil
}

// validatePathSyntax validates path syntax for common errors
func (sv *SecurityValidator) validatePathSyntax(path string) error {
	// Check for empty extraction
	if strings.Contains(path, "{}") {
		return newPathError(path, "empty extraction syntax not allowed", ErrInvalidPath)
	}

	// Check for empty array access
	if strings.Contains(path, "[]") {
		return newPathError(path, "empty array access not allowed", ErrInvalidPath)
	}

	// Check for invalid consecutive dots
	if strings.Contains(path, "..") {
		return newPathError(path, "consecutive dots not allowed", ErrInvalidPath)
	}

	// Check for invalid starting/ending characters
	if strings.HasPrefix(path, ".") && len(path) > 1 {
		return newPathError(path, "path cannot start with dot (except root '.')", ErrInvalidPath)
	}

	if strings.HasSuffix(path, ".") && path != "." {
		return newPathError(path, "path cannot end with dot", ErrInvalidPath)
	}

	return nil
}

// isValidNumberStart checks if a character can start a valid JSON number
func (sv *SecurityValidator) isValidNumberStart(r rune) bool {
	return (r >= '0' && r <= '9') || r == '-'
}

// FastValidateJSON performs ultra-fast JSON validation for hot paths
func FastValidateJSON(jsonStr string) bool {
	if len(jsonStr) == 0 {
		return false
	}

	trimmed := strings.TrimSpace(jsonStr)
	if len(trimmed) < 1 {
		return false
	}

	first := trimmed[0]
	last := trimmed[len(trimmed)-1]

	// Quick structural check
	switch first {
	case '{':
		return last == '}'
	case '[':
		return last == ']'
	case '"':
		return last == '"' && len(trimmed) >= 2
	case 't':
		return strings.HasPrefix(trimmed, "true")
	case 'f':
		return strings.HasPrefix(trimmed, "false")
	case 'n':
		return strings.HasPrefix(trimmed, "null")
	default:
		// Numbers
		return (first >= '0' && first <= '9') || first == '-'
	}
}

// FastValidatePath performs ultra-fast path validation for hot paths
func FastValidatePath(path string) bool {
	if len(path) > MaxPathLength {
		return false
	}

	// Quick security check
	return !strings.Contains(path, "..") &&
		!strings.Contains(path, "\x00") &&
		!strings.Contains(path, ":::")
}
