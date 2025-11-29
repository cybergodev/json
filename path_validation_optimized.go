package json

import (
	"strings"
	"unicode"
)

// Note: isSimpleProperty and isNumericIndex are already defined in processor.go
// This file provides additional validation utilities

// validatePathCharacters validates path characters without regex
func validatePathCharacters(path string) error {
	if len(path) > MaxPathLength {
		return newPathError(path, "path exceeds maximum length", ErrInvalidPath)
	}

	// Check for dangerous characters
	for i, r := range path {
		// Allow alphanumeric, dots, brackets, braces, colons, underscores, hyphens
		if !isAllowedPathChar(r) {
			return newPathError(path, "invalid character at position "+string(rune(i)), ErrInvalidPath)
		}
	}

	return nil
}

// isAllowedPathChar checks if a character is allowed in paths
func isAllowedPathChar(r rune) bool {
	return unicode.IsLetter(r) ||
		unicode.IsDigit(r) ||
		r == '.' ||
		r == '[' ||
		r == ']' ||
		r == '{' ||
		r == '}' ||
		r == ':' ||
		r == '_' ||
		r == '-' ||
		r == '/' ||
		r == '*'
}

// validateBracketBalance validates bracket and brace balance without regex
func validateBracketBalance(path string) error {
	var bracketDepth, braceDepth int

	for i, r := range path {
		switch r {
		case '[':
			bracketDepth++
			if bracketDepth > MaxConsecutiveBrackets {
				return newPathError(path, "too many nested brackets", ErrInvalidPath)
			}
		case ']':
			bracketDepth--
			if bracketDepth < 0 {
				return newPathError(path, "unmatched closing bracket at position "+string(rune(i)), ErrInvalidPath)
			}
		case '{':
			braceDepth++
			if braceDepth > MaxExtractionDepth {
				return newPathError(path, "too many nested braces", ErrInvalidPath)
			}
		case '}':
			braceDepth--
			if braceDepth < 0 {
				return newPathError(path, "unmatched closing brace at position "+string(rune(i)), ErrInvalidPath)
			}
		case ':':
			// Count consecutive colons
			if i > 0 && path[i-1] == ':' {
				consecutiveColons := 1
				for j := i - 1; j >= 0 && path[j] == ':'; j-- {
					consecutiveColons++
				}
				if consecutiveColons > MaxConsecutiveColons {
					return newPathError(path, "too many consecutive colons", ErrInvalidPath)
				}
			}
		}
	}

	if bracketDepth != 0 {
		return newPathError(path, "unmatched opening bracket", ErrInvalidPath)
	}
	if braceDepth != 0 {
		return newPathError(path, "unmatched opening brace", ErrInvalidPath)
	}

	return nil
}

// fastPathValidation performs fast path validation without regex
func fastPathValidation(path string) error {
	if path == "" || path == "." {
		return nil
	}

	// Check path length
	if len(path) > MaxPathLength {
		return newPathError(path, "path too long", ErrInvalidPath)
	}

	// Check for null bytes (security)
	if strings.Contains(path, "\x00") {
		return newPathError(path, "path contains null bytes", ErrInvalidPath)
	}

	// Validate characters
	if err := validatePathCharacters(path); err != nil {
		return err
	}

	// Validate bracket/brace balance
	if err := validateBracketBalance(path); err != nil {
		return err
	}

	return nil
}
