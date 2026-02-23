package internal

// IsWordChar returns true if the character is part of a word (alphanumeric or underscore)
func IsWordChar(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_'
}

// IsValidJSONPrimitive checks if a string represents a valid JSON primitive (true, false, null, or number)
func IsValidJSONPrimitive(s string) bool {
	return s == "true" || s == "false" || s == "null" || IsValidJSONNumber(s)
}

// IsValidJSONNumber checks if a string represents a valid JSON number format
func IsValidJSONNumber(s string) bool {
	if len(s) == 0 {
		return false
	}
	firstChar := s[0]
	return (firstChar >= '0' && firstChar <= '9') || firstChar == '-'
}
