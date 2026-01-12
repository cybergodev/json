package json

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// SecurityValidator provides comprehensive security validation for JSON processing
// This consolidates all validation logic in one place for better maintainability
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

// ValidateAll performs comprehensive validation of both JSON and path inputs
func (sv *SecurityValidator) ValidateAll(jsonStr, path string) error {
	// Validate JSON input first
	if err := sv.ValidateJSONInput(jsonStr); err != nil {
		return err
	}

	// Validate path input
	if err := sv.ValidatePathInput(path); err != nil {
		return err
	}

	return nil
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
	cleanJSON := strings.TrimPrefix(jsonStr, ValidationBOMPrefix)
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

// validateJSONSecurity performs optimized security validation for JSON input
func (sv *SecurityValidator) validateJSONSecurity(jsonStr string) error {
	// Fast path: check for null bytes first (most critical)
	if strings.IndexByte(jsonStr, 0) != -1 {
		return newSecurityError("validate_json_security", "null byte injection detected")
	}

	// Optimize: single pass for multiple checks using byte scanning
	lowerJSON := strings.ToLower(jsonStr)

	// Check dangerous patterns in single pass
	dangerousPatterns := []string{
		"__proto__", "constructor", "prototype",
		"<script", "javascript:", "vbscript:",
		"eval(", "function(",
	}

	for _, pattern := range dangerousPatterns {
		if strings.Contains(lowerJSON, pattern) {
			return newSecurityError("validate_json_security", fmt.Sprintf("dangerous pattern: %s", pattern))
		}
	}

	return nil
}

// validatePathSecurity performs optimized security validation for paths
func (sv *SecurityValidator) validatePathSecurity(path string) error {
	// Fast path: check for null bytes
	if strings.IndexByte(path, 0) != -1 {
		return newPathError(path, "null byte injection detected", ErrSecurityViolation)
	}

	lowerPath := strings.ToLower(path)

	// Check path traversal patterns
	if strings.Contains(path, "..") {
		return newPathError(path, "path traversal detected", ErrSecurityViolation)
	}

	// Check URL encoding bypass (including double encoding)
	if strings.Contains(lowerPath, "%2e") || strings.Contains(lowerPath, "%2f") ||
		strings.Contains(lowerPath, "%5c") || strings.Contains(lowerPath, "%00") ||
		strings.Contains(lowerPath, "%252e") || strings.Contains(lowerPath, "%252f") {
		return newPathError(path, "path traversal via URL encoding detected", ErrSecurityViolation)
	}

	// Check UTF-8 overlong encoding
	if strings.Contains(lowerPath, "%c0%af") || strings.Contains(lowerPath, "%c1%9c") {
		return newPathError(path, "path traversal via UTF-8 overlong encoding detected", ErrSecurityViolation)
	}

	// Check excessive special characters
	if strings.Contains(path, ":::") || strings.Contains(path, "[[[") || strings.Contains(path, "}}}") {
		return newPathError(path, "excessive special characters", ErrSecurityViolation)
	}

	return nil
}

// validateJSONStructure performs basic JSON structure validation
func (sv *SecurityValidator) validateJSONStructure(jsonStr string) error {
	// Trim whitespace for validation
	trimmed := strings.TrimSpace(jsonStr)
	if len(trimmed) == 0 {
		return newOperationError("validate_json_structure", "JSON string is empty after trimming", ErrInvalidJSON)
	}

	// Check for valid JSON start/end characters
	firstChar := trimmed[0]
	lastChar := trimmed[len(trimmed)-1]

	if !((firstChar == '{' && lastChar == '}') || (firstChar == '[' && lastChar == ']') ||
		(firstChar == '"' && lastChar == '"') || isValidJSONPrimitive(trimmed)) {
		return newOperationError("validate_json_structure", "invalid JSON structure", ErrInvalidJSON)
	}

	return nil
}

// validateNestingDepth validates JSON nesting depth (optimized)
func (sv *SecurityValidator) validateNestingDepth(jsonStr string) error {
	depth := 0
	inString := false
	escaped := false

	for _, char := range jsonStr {
		if escaped {
			escaped = false
			continue
		}

		switch char {
		case '\\':
			if inString {
				escaped = true
			}
		case '"':
			inString = !inString
		case '{', '[':
			if !inString {
				depth++
				if depth > sv.maxNestingDepth {
					return newOperationError("validate_nesting_depth",
						fmt.Sprintf("nesting depth %d exceeds maximum %d", depth, sv.maxNestingDepth), ErrDepthLimit)
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

// validateBracketMatching validates bracket and brace matching in paths
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

// validatePathSyntax performs basic path syntax validation
func (sv *SecurityValidator) validatePathSyntax(path string) error {
	// Check for consecutive dots (more than 2)
	if strings.Contains(path, "...") {
		return newPathError(path, "invalid consecutive dots", ErrInvalidPath)
	}

	// Check for invalid characters in path
	for i, char := range path {
		if char < 32 && char != '\t' && char != '\n' && char != '\r' {
			return newPathError(path, fmt.Sprintf("invalid control character at position %d", i), ErrInvalidPath)
		}
	}

	return nil
}

// isValidJSONPrimitive checks if a string represents a valid JSON primitive
func isValidJSONPrimitive(s string) bool {
	return s == "true" || s == "false" || s == "null" || isValidJSONNumber(s)
}

// isValidJSONNumber checks if a string represents a valid JSON number
func isValidJSONNumber(s string) bool {
	if len(s) == 0 {
		return false
	}

	// Simple number validation - starts with digit or minus
	firstChar := s[0]
	return (firstChar >= '0' && firstChar <= '9') || firstChar == '-'
}
