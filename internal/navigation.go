package internal

import (
	"fmt"
	"strconv"
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

// NeedsDotBefore determines if a dot should be inserted before a character
func NeedsDotBefore(prevChar rune) bool {
	return (prevChar >= 'a' && prevChar <= 'z') ||
		(prevChar >= 'A' && prevChar <= 'Z') ||
		(prevChar >= '0' && prevChar <= '9') ||
		prevChar == '_' || prevChar == ']' || prevChar == '}'
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
		if c == '{' || c == '}' || c == '[' || c == ']' || c == ':' {
			return true
		}
	}
	return false
}

// HasComplexSegments checks if any segment is complex (slice or extract)
func HasComplexSegments(segments []PathSegment) bool {
	for _, segment := range segments {
		switch segment.Type {
		case ArraySliceSegment, ExtractSegment:
			return true
		}
	}
	return false
}

// IsDistributedOperationPath checks if a path contains distributed operation patterns
func IsDistributedOperationPath(path string) bool {
	distributedPatterns := []string{
		"}[",
		"}:",
		"}{",
	}

	for _, pattern := range distributedPatterns {
		if strings.Contains(path, pattern) {
			return true
		}
	}

	if strings.Contains(path, "{flat:") {
		return true
	}

	return false
}

// IsDistributedOperationSegment checks if a segment triggers distributed operations
func IsDistributedOperationSegment(segment PathSegment) bool {
	return segment.Key != ""
}

// ParsePathSegment parses a single path segment and appends to segments slice
func ParsePathSegment(part string, segments []PathSegment) []PathSegment {
	if strings.Contains(part, "[") {
		return ParseArraySegment(part, segments)
	} else if strings.Contains(part, "{") {
		return ParseExtractionSegment(part, segments)
	} else {
		if index, err := strconv.Atoi(part); err == nil {
			segments = append(segments, PathSegment{
				Type:  ArrayIndexSegment,
				Index: index,
			})
			return segments
		}

		segments = append(segments, PathSegment{
			Key:  part,
			Type: PropertySegment,
		})
		return segments
	}
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
		segments = append(segments, PathSegment{
			Key:  propertyName,
			Type: PropertySegment,
		})
	}

	bracketContent := part[openBracket+1 : closeBracket]

	if strings.Contains(bracketContent, ":") {
		var start, end, step int
		var flags uint8

		parts := strings.Split(bracketContent, ":")
		if len(parts) >= 2 {
			if parts[0] != "" {
				if startVal, err := strconv.Atoi(parts[0]); err == nil {
					start = startVal
					flags |= FlagHasStart
				}
			}

			if parts[1] != "" {
				if endVal, err := strconv.Atoi(parts[1]); err == nil {
					end = endVal
					flags |= FlagHasEnd
				}
			}

			if len(parts) == 3 && parts[2] != "" {
				if stepVal, err := strconv.Atoi(parts[2]); err == nil {
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
		segment := PathSegment{
			Type: ArrayIndexSegment,
		}

		if index, err := strconv.Atoi(bracketContent); err == nil {
			segment.Index = index
		}

		segments = append(segments, segment)
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
		segments = append(segments, PathSegment{
			Key:  propertyName,
			Type: PropertySegment,
		})
	}

	braceContent := part[openBrace+1 : closeBrace]

	var flags uint8
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
func SplitPathIntoSegments(path string, segments []PathSegment) []PathSegment {
	parts := strings.Split(path, ".")

	for _, part := range parts {
		if part == "" {
			continue
		}
		segments = ParsePathSegment(part, segments)
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

// NormalizePathSeparators removes duplicate dots and trims leading/trailing dots
// Optimized: single-pass construction using strings.Builder
func NormalizePathSeparators(path string) string {
	if len(path) == 0 {
		return ""
	}

	// Fast path: check if normalization is needed
	needsNormalization := false
	hasLeadingDot := path[0] == '.'
	hasTrailingDot := path[len(path)-1] == '.'

	for i := 0; i < len(path)-1; i++ {
		if path[i] == '.' && path[i+1] == '.' {
			needsNormalization = true
			break
		}
	}

	// If no normalization needed, just trim
	if !needsNormalization && !hasLeadingDot && !hasTrailingDot {
		return path
	}

	// Build normalized path in single pass
	var sb strings.Builder
	sb.Grow(len(path))

	inDotRun := false
	for i := 0; i < len(path); i++ {
		c := path[i]
		if c == '.' {
			if !inDotRun {
				sb.WriteByte(c)
				inDotRun = true
			}
		} else {
			sb.WriteByte(c)
			inDotRun = false
		}
	}

	result := sb.String()
	// Trim leading and trailing dots
	result = strings.Trim(result, ".")

	return result
}

// IsValidPropertyName checks if a name is a valid property name
func IsValidPropertyName(name string) bool {
	return name != "" && !strings.ContainsAny(name, ".[]{}()")
}

// IsValidArrayIndex checks if a string is a valid array index
func IsValidArrayIndex(index string) bool {
	if index == "" {
		return false
	}

	index = strings.TrimPrefix(index, "-")

	_, err := strconv.Atoi(index)
	return err == nil
}

// IsValidSliceRange checks if a range string is a valid slice range
func IsValidSliceRange(rangeStr string) bool {
	parts := strings.Split(rangeStr, ":")
	if len(parts) < 2 || len(parts) > 3 {
		return false
	}

	for _, part := range parts {
		if part != "" {
			if _, err := strconv.Atoi(part); err != nil {
				return false
			}
		}
	}

	return true
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

// IsMapType checks if data is a map type
func IsMapType(data any) bool {
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

// WrapError wraps an error with context
func WrapError(err error, context string) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", context, err)
}

// CreatePathError creates a path-specific error
func CreatePathError(path string, operation string, err error) error {
	return fmt.Errorf("failed to %s at path '%s': %w", operation, path, err)
}
