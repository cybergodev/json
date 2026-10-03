package internal

import (
	"strings"
)

// NeedsPathPreprocessing checks if a path needs preprocessing before parsing
func NeedsPathPreprocessing(path string) bool {
	for i := 0; i < len(path); i++ {
		c := path[i]
		if c == '[' || c == '{' {
			return true
		}
	}
	return false
}

// NeedsDotBeforeByte determines if a dot should be inserted before a character (byte version for ASCII fast path)
func NeedsDotBeforeByte(prevChar byte) bool {
	return (prevChar >= 'a' && prevChar <= 'z') ||
		(prevChar >= 'A' && prevChar <= 'Z') ||
		(prevChar >= '0' && prevChar <= '9') ||
		prevChar == '_' || prevChar == ']' || prevChar == '}'
}

// NeedsDotBefore determines if a dot should be inserted before a character.
// Delegates to the byte predicate: every accepted character is ASCII, so the
// two forms cannot diverge if one is ever extended.
func NeedsDotBefore(prevChar rune) bool {
	if prevChar >= 0 && prevChar <= 0x7F {
		return NeedsDotBeforeByte(byte(prevChar))
	}
	return false
}

// PreprocessPath adds dots before brackets/braces where needed
func PreprocessPath(path string, sb *strings.Builder) string {
	sb.Reset()

	// Fast ASCII check - avoid rune conversion for ASCII paths
	isASCII := true
	for i := 0; i < len(path); i++ {
		if path[i] >= 0x80 {
			isASCII = false
			break
		}
	}

	if isASCII {
		// Fast path: byte-level processing for ASCII
		for i := 0; i < len(path); i++ {
			c := path[i]
			switch c {
			case '[':
				if i > 0 && NeedsDotBeforeByte(path[i-1]) {
					sb.WriteByte('.')
				}
				sb.WriteByte(c)
			case '{':
				if i > 0 && NeedsDotBeforeByte(path[i-1]) {
					sb.WriteByte('.')
				}
				sb.WriteByte(c)
			default:
				sb.WriteByte(c)
			}
		}
	} else {
		// Slow path: rune processing for non-ASCII
		runes := []rune(path)
		for i, r := range runes {
			switch r {
			case '[':
				if i > 0 && NeedsDotBefore(runes[i-1]) {
					sb.WriteRune('.')
				}
				sb.WriteRune(r)
			case '{':
				if i > 0 && NeedsDotBefore(runes[i-1]) {
					sb.WriteRune('.')
				}
				sb.WriteRune(r)
			default:
				sb.WriteRune(r)
			}
		}
	}

	return sb.String()
}

// IsComplexPath checks if a path contains complex patterns
// Optimized: single scan instead of multiple Contains calls
func IsComplexPath(path string) bool {
	for i := 0; i < len(path); i++ {
		c := path[i]
		if c == '{' || c == '}' || c == '[' || c == ']' || c == ':' || c == '*' {
			return true
		}
	}
	return false
}

// IsExtractionPath checks if a path contains extraction patterns that trigger
// multi-container (distributed) operations: }[, }:, }{, {flat:
func IsExtractionPath(path string) bool {
	extractionPatterns := []string{
		"}[",
		"}:",
		"}{",
		"{flat:",
	}

	for _, pattern := range extractionPatterns {
		if strings.Contains(path, pattern) {
			return true
		}
	}

	return false
}

// IsExtractionSegment checks if a segment triggers extraction operations
func IsExtractionSegment(segment PathSegment) bool {
	return segment.Type == ExtractSegment
}

// ParsePathSegment parses a single path segment and appends to segments slice
func ParsePathSegment(part string, segments []PathSegment) []PathSegment {
	if strings.Contains(part, "[") {
		return ParseArraySegment(part, segments)
	}
	if strings.Contains(part, "{") {
		return ParseExtractionSegment(part, segments)
	}
	if index, ok := ParseIntFast(part); ok {
		segments = append(segments, PathSegment{
			Type:  ArrayIndexSegment,
			Index: index,
		})
		return segments
	}

	segments = append(segments, PathSegment{
		Key:  UnescapePathSegment(part),
		Type: PropertySegment,
	})
	return segments
}

// ParseArraySegment parses array access segments like [0], [1:3], etc.
func ParseArraySegment(part string, segments []PathSegment) []PathSegment {
	openBracket := strings.Index(part, "[")
	closeBracket := strings.LastIndex(part, "]")

	if openBracket == -1 || closeBracket == -1 || closeBracket <= openBracket {
		segments = append(segments, PathSegment{
			Key:  part,
			Type: PropertySegment,
		})
		return segments
	}

	if openBracket > 0 {
		propertyName := part[:openBracket]
		// Unescape like parsePropertyWithArray (internal/path.go): otherwise
		// Set writes a bogus literal key for paths like `key\.[0]` while Get
		// reads the unescaped `key.` — reads and writes diverged (D-002).
		segments = append(segments, PathSegment{
			Key:  UnescapePathSegment(propertyName),
			Type: PropertySegment,
		})
	}

	bracketContent := part[openBracket+1 : closeBracket]

	if strings.Contains(bracketContent, ":") {
		var start, end, step int
		var flags PathSegmentFlags

		parts := strings.Split(bracketContent, ":")
		if len(parts) >= 2 {
			if parts[0] != "" {
				if startVal, ok := ParseIntFast(parts[0]); ok {
					start = startVal
					flags |= FlagHasStart
				}
			}

			if parts[1] != "" {
				if endVal, ok := ParseIntFast(parts[1]); ok {
					end = endVal
					flags |= FlagHasEnd
				}
			}

			if len(parts) == 3 && parts[2] != "" {
				if stepVal, ok := ParseIntFast(parts[2]); ok {
					step = stepVal
					flags |= FlagHasStep
				}
			}
		}

		segments = append(segments, PathSegment{
			Type:  ArraySliceSegment,
			Index: start, // Use Index field for start value
			End:   end,
			Step:  step,
			Flags: flags,
		})
	} else {
		// Check for append syntax [+]
		if bracketContent == "+" {
			segments = append(segments, PathSegment{
				Type: AppendSegment,
			})
		} else {
			segment := PathSegment{
				Type: ArrayIndexSegment,
			}

			if index, ok := ParseIntFast(bracketContent); ok {
				segment.Index = index
			}

			segments = append(segments, segment)
		}
	}

	if closeBracket+1 < len(part) {
		remaining := part[closeBracket+1:]
		if remaining != "" {
			segments = ParsePathSegment(remaining, segments)
		}
	}

	return segments
}

// ParseExtractionSegment parses extraction segments like {key}, {flat:key}, etc.
func ParseExtractionSegment(part string, segments []PathSegment) []PathSegment {
	openBrace := strings.Index(part, "{")
	closeBrace := strings.LastIndex(part, "}")

	if openBrace == -1 || closeBrace == -1 || closeBrace <= openBrace {
		segments = append(segments, PathSegment{
			Key:  part,
			Type: PropertySegment,
		})
		return segments
	}

	if openBrace > 0 {
		propertyName := part[:openBrace]
		// Same unescape as ParseArraySegment — keep Get/Set/Delete consistent
		// for escaped keys preceding an extraction brace (D-002).
		segments = append(segments, PathSegment{
			Key:  UnescapePathSegment(propertyName),
			Type: PropertySegment,
		})
	}

	braceContent := part[openBrace+1 : closeBrace]

	var flags PathSegmentFlags
	var key string
	if strings.HasPrefix(braceContent, "flat:") {
		key = braceContent[5:]
		flags |= FlagIsFlat
	} else {
		key = braceContent
	}

	segments = append(segments, PathSegment{
		Type:  ExtractSegment,
		Key:   key,
		Flags: flags,
	})

	if closeBrace+1 < len(part) {
		remaining := part[closeBrace+1:]
		if remaining != "" {
			segments = ParsePathSegment(remaining, segments)
		}
	}

	return segments
}

// SplitPathIntoSegments splits a path into segments by dots
// ESCAPE: Handles \. \\ \[ \] \{ \} escape sequences
func SplitPathIntoSegments(path string, segments []PathSegment) []PathSegment {
	// Check for escape sequences
	hasEscape := HasEscapeSequence(path)

	// Fast path: no escape sequences
	if !hasEscape {
		for part := range strings.SplitSeq(path, ".") {
			if part == "" {
				continue
			}
			segments = ParsePathSegment(part, segments)
		}
		return segments
	}

	// Slow path: handle escape sequences
	pathLen := len(path)
	start := 0

	for i := 0; i <= pathLen; i++ {
		if i == pathLen {
			// End of path - add remaining segment
			if start < pathLen {
				part := path[start:]
				if part != "" {
					segments = ParsePathSegment(part, segments)
				}
			}
			break
		}

		c := path[i]
		if c == '\\' && i+1 < pathLen {
			// Skip escaped character
			i++
			continue
		}

		if c == '.' {
			// Unescaped dot - split here
			if i > start {
				part := path[start:i]
				if part != "" {
					segments = ParsePathSegment(part, segments)
				}
			}
			start = i + 1
		}
	}

	return segments
}

// ReconstructPath reconstructs a path string from segments
func ReconstructPath(segments []PathSegment) string {
	if len(segments) == 0 {
		return ""
	}

	var sb strings.Builder
	for i, segment := range segments {
		if i > 0 {
			sb.WriteRune('.')
		}
		sb.WriteString(segment.String())
	}

	return sb.String()
}

// IsValidArrayIndex checks if a string is a valid array index
func IsValidArrayIndex(index string) bool {
	if index == "" {
		return false
	}

	index = strings.TrimPrefix(index, "-")

	_, ok := ParseIntFast(index)
	return ok
}

// IsArrayType checks if data is an array type
func IsArrayType(data any) bool {
	switch data.(type) {
	case []any:
		return true
	default:
		return false
	}
}

// IsObjectType checks if data is an object type
func IsObjectType(data any) bool {
	switch data.(type) {
	case map[string]any, map[any]any:
		return true
	default:
		return false
	}
}

// IsNilOrEmpty checks if a value is nil or empty
func IsNilOrEmpty(data any) bool {
	if data == nil {
		return true
	}

	switch v := data.(type) {
	case string:
		return v == ""
	case []any:
		return len(v) == 0
	case map[string]any:
		return len(v) == 0
	case map[any]any:
		return len(v) == 0
	default:
		return false
	}
}
