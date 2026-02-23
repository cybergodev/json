package json

import (
	"fmt"
	"strconv"
	"strings"
)

// validateInput validates JSON input string with optimized security checks
func (p *Processor) validateInput(jsonString string) error {
	// Use the unified security validator
	validator := NewSecurityValidator(
		p.config.MaxJSONSize,
		MaxPathLength,
		p.config.MaxNestingDepthSecurity,
	)

	return validator.ValidateJSONInput(jsonString)
}

// isASCIIOnly checks if string contains only ASCII characters (fast path optimization)
func isASCIIOnly(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] > 127 {
			return false
		}
	}
	return true
}

// isValidNumberStart checks if character can start a valid JSON number
func isValidNumberStart(c byte) bool {
	return (c >= '0' && c <= '9') || c == '-'
}

// validatePath validates a JSON path string with enhanced security and efficiency
func (p *Processor) validatePath(path string) error {
	// Use the unified security validator
	validator := NewSecurityValidator(
		p.config.MaxJSONSize,
		MaxPathLength,
		p.config.MaxNestingDepthSecurity,
	)

	return validator.ValidatePathInput(path)
}

// validateSingleSegment validates a single path segment
func (p *Processor) validateSingleSegment(segment string) error {
	// Check for unmatched brackets or braces first
	if strings.Contains(segment, "[") && !strings.Contains(segment, "]") {
		return &JsonsError{
			Op:      "validate_path",
			Path:    segment,
			Message: "unclosed bracket '[' in path segment",
			Err:     ErrInvalidPath,
		}
	}

	if strings.Contains(segment, "]") && !strings.Contains(segment, "[") {
		return &JsonsError{
			Op:      "validate_path",
			Path:    segment,
			Message: "unmatched bracket ']' in path segment",
			Err:     ErrInvalidPath,
		}
	}

	if strings.Contains(segment, "{") && !strings.Contains(segment, "}") {
		return &JsonsError{
			Op:      "validate_path",
			Path:    segment,
			Message: "unclosed brace '{' in path segment",
			Err:     ErrInvalidPath,
		}
	}

	if strings.Contains(segment, "}") && !strings.Contains(segment, "{") {
		return &JsonsError{
			Op:      "validate_path",
			Path:    segment,
			Message: "unmatched brace '}' in path segment",
			Err:     ErrInvalidPath,
		}
	}

	// Check for valid identifier or array index (optimized without regex)
	if isSimpleProperty(segment) {
		return nil
	}

	if isNumericIndex(segment) {
		return nil
	}

	// Check for array access syntax
	if strings.Contains(segment, "[") && strings.Contains(segment, "]") {
		return p.validateArrayAccess(segment)
	}

	// Check for pure slice syntax (starts with [ and contains :)
	if strings.HasPrefix(segment, "[") && strings.Contains(segment, ":") && strings.HasSuffix(segment, "]") {
		return p.validateSliceSyntax(segment)
	}

	// Check for extraction syntax
	if strings.Contains(segment, "{") && strings.Contains(segment, "}") {
		return p.validateExtractionSyntax(segment)
	}

	return &JsonsError{
		Op:      "validate_path",
		Path:    segment,
		Message: "invalid path segment format",
		Err:     ErrInvalidPath,
	}
}

// validateArrayAccess validates array access syntax
func (p *Processor) validateArrayAccess(segment string) error {
	// Check for unmatched brackets
	openCount := strings.Count(segment, "[")
	closeCount := strings.Count(segment, "]")
	if openCount != closeCount {
		return &JsonsError{
			Op:      "validate_path",
			Path:    segment,
			Message: "unmatched brackets in array access",
			Err:     ErrInvalidPath,
		}
	}

	// Find bracket positions
	start := strings.Index(segment, "[")
	end := strings.LastIndex(segment, "]")
	if start == -1 || end == -1 || end <= start {
		return &JsonsError{
			Op:      "validate_path",
			Path:    segment,
			Message: "malformed bracket syntax",
			Err:     ErrInvalidPath,
		}
	}

	indexPart := segment[start+1 : end]

	// Check if it's a slice (contains colon)
	if strings.Contains(indexPart, ":") {
		return p.validateSliceSyntax(segment)
	}

	// Valid simple array index
	if indexPart == "" {
		return &JsonsError{
			Op:      "validate_path",
			Path:    segment,
			Message: "empty array index",
			Err:     ErrInvalidPath,
		}
	}

	// Check for wildcard
	if indexPart == "*" {
		return nil // Wildcard is valid
	}

	// Check if it's a valid number (including negative)
	if _, err := strconv.Atoi(indexPart); err != nil {
		return &JsonsError{
			Op:      "validate_path",
			Path:    segment,
			Message: fmt.Sprintf("invalid array index '%s': must be a number or '*'", indexPart),
			Err:     ErrInvalidPath,
		}
	}

	return nil
}

// validateSliceSyntax validates array slice syntax like [1:3], [::2], [::-1]
func (p *Processor) validateSliceSyntax(segment string) error {
	// Extract the slice part between brackets
	start := strings.Index(segment, "[")
	end := strings.LastIndex(segment, "]")
	if start == -1 || end == -1 || end <= start {
		return &JsonsError{
			Op:      "validate_path",
			Path:    segment,
			Message: "malformed slice syntax",
			Err:     ErrInvalidPath,
		}
	}

	slicePart := segment[start+1 : end]

	// Parse slice components
	_, _, _, err := p.parseSliceComponents(slicePart)
	if err != nil {
		return &JsonsError{
			Op:      "validate_path",
			Path:    segment,
			Message: fmt.Sprintf("invalid slice syntax: %v", err),
			Err:     ErrInvalidPath,
		}
	}

	return nil
}

// validateExtractionSyntax validates extraction syntax like {name}
func (p *Processor) validateExtractionSyntax(segment string) error {
	// Check for unmatched braces
	openCount := strings.Count(segment, "{")
	closeCount := strings.Count(segment, "}")
	if openCount != closeCount {
		return &JsonsError{
			Op:      "validate_path",
			Path:    segment,
			Message: "unmatched braces in extraction syntax",
			Err:     ErrInvalidPath,
		}
	}

	// Find brace positions
	start := strings.Index(segment, "{")
	end := strings.LastIndex(segment, "}")
	if start == -1 || end == -1 || end <= start {
		return &JsonsError{
			Op:      "validate_path",
			Path:    segment,
			Message: "malformed extraction syntax",
			Err:     ErrInvalidPath,
		}
	}

	fieldName := segment[start+1 : end]
	if fieldName == "" {
		return &JsonsError{
			Op:      "validate_path",
			Path:    segment,
			Message: "empty field name in extraction syntax",
			Err:     ErrInvalidPath,
		}
	}

	// Check for unsupported conditional filter syntax
	if strings.HasPrefix(fieldName, "?") {
		return &JsonsError{
			Op:      "validate_path",
			Path:    segment,
			Message: fmt.Sprintf("conditional filter syntax '{%s}' is not supported. Use standard extraction syntax like '{fieldName}' instead", fieldName),
			Err:     ErrInvalidPath,
		}
	}

	// Check for other unsupported query-like syntax patterns
	if strings.Contains(fieldName, "=") || strings.Contains(fieldName, ">") || strings.Contains(fieldName, "<") || strings.Contains(fieldName, "&") || strings.Contains(fieldName, "|") {
		return &JsonsError{
			Op:      "validate_path",
			Path:    segment,
			Message: fmt.Sprintf("query syntax '{%s}' is not supported. Use standard extraction syntax like '{fieldName}' instead", fieldName),
			Err:     ErrInvalidPath,
		}
	}

	return nil
}

// validateJSONPointerPath validates JSON Pointer format paths
func (p *Processor) validateJSONPointerPath(path string) error {
	// Basic JSON Pointer validation
	if !strings.HasPrefix(path, "/") {
		return &JsonsError{
			Op:      "validate_path",
			Path:    path,
			Message: "JSON Pointer must start with /",
			Err:     ErrInvalidPath,
		}
	}

	// Check for trailing slash (invalid except for root "/")
	if len(path) > 1 && strings.HasSuffix(path, "/") {
		return &JsonsError{
			Op:      "validate_path",
			Path:    path,
			Message: "JSON Pointer cannot end with trailing slash",
			Err:     ErrInvalidPath,
		}
	}

	// Check for proper escaping
	segments := strings.Split(path[1:], "/")
	for _, segment := range segments {
		if strings.Contains(segment, "~") {
			// Valid escape sequences
			if !p.isValidJSONPointerEscape(segment) {
				return &JsonsError{
					Op:      "validate_path",
					Path:    path,
					Message: "invalid JSON Pointer escape sequence",
					Err:     ErrInvalidPath,
				}
			}
		}
	}

	return nil
}

// validateDotNotationPath validates dot notation format paths
func (p *Processor) validateDotNotationPath(path string) error {
	// Check for consecutive dots
	if strings.Contains(path, "..") {
		return &JsonsError{
			Op:      "validate_path",
			Path:    path,
			Message: "path contains consecutive dots",
			Err:     ErrInvalidPath,
		}
	}

	// Check for leading/trailing dots
	if strings.HasPrefix(path, ".") || strings.HasSuffix(path, ".") {
		return &JsonsError{
			Op:      "validate_path",
			Path:    path,
			Message: "path has leading or trailing dots",
			Err:     ErrInvalidPath,
		}
	}

	// Split path into segments and validate each one
	segments := strings.Split(path, ".")
	for _, segment := range segments {
		if segment == "" {
			continue // Skip empty segments (shouldn't happen after above checks)
		}

		if err := p.validateSingleSegment(segment); err != nil {
			return err
		}
	}

	return nil
}

// isValidJSONPointerEscape validates JSON Pointer escape sequences
func (p *Processor) isValidJSONPointerEscape(segment string) bool {
	i := 0
	for i < len(segment) {
		if segment[i] == '~' {
			if i+1 >= len(segment) {
				return false // Incomplete escape
			}
			next := segment[i+1]
			if next != '0' && next != '1' {
				return false // Invalid escape
			}
			i += 2
		} else {
			i++
		}
	}
	return true
}

// isSimpleProperty checks if a string is a simple property name without using regex
func isSimpleProperty(s string) bool {
	if len(s) == 0 {
		return false
	}

	// First character must be letter or underscore
	first := s[0]
	if !((first >= 'a' && first <= 'z') || (first >= 'A' && first <= 'Z') || first == '_') {
		return false
	}

	// Remaining characters must be letters, digits, or underscores
	for i := 1; i < len(s); i++ {
		c := s[i]
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_') {
			return false
		}
	}

	return true
}

// isNumericIndex checks if a string represents a numeric index without using regex
func isNumericIndex(s string) bool {
	if len(s) == 0 {
		return false
	}

	start := 0
	if s[0] == '-' {
		if len(s) == 1 {
			return false
		}
		start = 1
	}

	for i := start; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}

	return true
}
