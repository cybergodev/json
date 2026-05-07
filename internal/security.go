package internal

// IsWordChar returns true if the character is part of a word (alphanumeric or underscore)
func IsWordChar(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_'
}

// IsValidCacheKey validates that a cache key is valid for use.
// Returns false if the key is empty, too long, or contains control characters.
func IsValidCacheKey(key string) bool {
	keyLen := len(key)
	if keyLen == 0 || keyLen > MaxCacheKeyLength {
		return false
	}

	// Check for control characters (0-31, 127)
	for i := range keyLen {
		c := key[i]
		if c < 32 || c == 127 {
			return false
		}
	}

	return true
}

// IsValidJSONPrimitive checks if a string represents a valid JSON primitive (true, false, null, or number)
func IsValidJSONPrimitive(s string) bool {
	return s == "true" || s == "false" || s == "null" || IsValidJSONNumber(s)
}

// IsValidJSONNumber validates if a string represents a valid JSON number format
// according to RFC 8259. Supports integers, decimals, and scientific notation.
func IsValidJSONNumber(s string) bool {
	if len(s) == 0 {
		return false
	}

	i := 0

	// Optional leading minus sign
	if s[0] == '-' {
		i = 1
		if i >= len(s) {
			return false
		}
	}

	// Integer part
	if s[i] == '0' {
		i++
		// Leading zero must be followed by . or exponent or end
		if i < len(s) && s[i] != '.' && s[i] != 'e' && s[i] != 'E' {
			return false
		}
	} else if s[i] >= '1' && s[i] <= '9' {
		i++
		for i < len(s) && IsDigit(s[i]) {
			i++
		}
	} else {
		return false
	}

	// Optional fractional part
	if i < len(s) && s[i] == '.' {
		i++
		if i >= len(s) || !IsDigit(s[i]) {
			return false
		}
		i++
		for i < len(s) && IsDigit(s[i]) {
			i++
		}
	}

	// Optional exponent part
	if i < len(s) && (s[i] == 'e' || s[i] == 'E') {
		i++
		if i < len(s) && (s[i] == '+' || s[i] == '-') {
			i++
		}
		if i >= len(s) || !IsDigit(s[i]) {
			return false
		}
		i++
		for i < len(s) && IsDigit(s[i]) {
			i++
		}
	}

	return i == len(s)
}
