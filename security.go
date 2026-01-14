package json

import (
	"fmt"
	"strings"
	"unicode/utf8"
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

	// Check dangerous patterns
	lowerJSON := strings.ToLower(jsonStr)
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

func (sv *SecurityValidator) validatePathSecurity(path string) error {
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

func (sv *SecurityValidator) validateJSONStructure(jsonStr string) error {
	trimmed := strings.TrimSpace(jsonStr)
	if len(trimmed) == 0 {
		return newOperationError("validate_json_structure", "JSON string is empty after trimming", ErrInvalidJSON)
	}

	firstChar := trimmed[0]
	lastChar := trimmed[len(trimmed)-1]

	if !((firstChar == '{' && lastChar == '}') || (firstChar == '[' && lastChar == ']') ||
		(firstChar == '"' && lastChar == '"') || isValidJSONPrimitive(trimmed)) {
		return newOperationError("validate_json_structure", "invalid JSON structure", ErrInvalidJSON)
	}

	return nil
}

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
	return s == "true" || s == "false" || s == "null" || isValidJSONNumber(s)
}

func isValidJSONNumber(s string) bool {
	if len(s) == 0 {
		return false
	}
	firstChar := s[0]
	return (firstChar >= '0' && firstChar <= '9') || firstChar == '-'
}
