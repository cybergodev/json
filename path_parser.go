package json

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/cybergodev/json/internal"
)

// pathParser implements the PathParser interface
type pathParser struct {
	// Compiled regex patterns for performance
	simplePropertyPattern *regexp.Regexp
	arrayIndexPattern     *regexp.Regexp
	slicePattern          *regexp.Regexp
	extractPattern        *regexp.Regexp

	// String builder pool for efficient string operations
	stringBuilderPool *stringBuilderPool
}

// NewPathParser creates a new path parser instance
func NewPathParser() PathParser {
	return &pathParser{
		simplePropertyPattern: regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`),
		arrayIndexPattern:     regexp.MustCompile(`^\[(-?\d+)\]$`),
		slicePattern:          regexp.MustCompile(`^\[(-?\d*):(-?\d*)(?::(-?\d+))?\]$`),
		extractPattern:        regexp.MustCompile(`^\{([^}]+)\}$`),
		stringBuilderPool:     newStringBuilderPool(),
	}
}

// ParsePath parses a path string into segments
func (pp *pathParser) ParsePath(path string) ([]PathSegmentInfo, error) {
	if path == "" {
		return []PathSegmentInfo{}, nil
	}

	// Handle different path formats
	if strings.HasPrefix(path, "/") {
		return pp.parseJSONPointer(path)
	}

	return pp.parseDotNotation(path)
}

// ValidatePath validates a path string for correctness
func (pp *pathParser) ValidatePath(path string) error {
	if path == "" {
		return nil // Empty path is valid (root access)
	}

	// Check for unmatched brackets and braces
	if err := pp.validateBrackets(path); err != nil {
		return err
	}

	// Check for invalid characters or patterns
	if err := pp.validatePathSyntax(path); err != nil {
		return err
	}

	// Try to parse the path to ensure it's valid
	_, err := pp.ParsePath(path)
	return err
}

// SplitPathIntoSegments splits a path into segments
func (pp *pathParser) SplitPathIntoSegments(path string) []PathSegmentInfo {
	segments, err := pp.ParsePath(path)
	if err != nil {
		// Return empty slice on error
		return []PathSegmentInfo{}
	}
	return segments
}

// PreprocessPath preprocesses a path string for parsing
func (pp *pathParser) PreprocessPath(path string) string {
	if path == "" || strings.HasPrefix(path, "/") {
		return path
	}

	sb := pp.stringBuilderPool.Get()
	defer pp.stringBuilderPool.Put(sb)

	sb.Reset()
	sb.Grow(len(path) + 10) // Pre-allocate with some extra space

	for i, char := range path {
		// Check if we need to add a dot before [ or {
		if (char == '[' || char == '{') && i > 0 {
			prevChar := rune(path[i-1])
			if prevChar != '.' && pp.needsDotBefore(prevChar) {
				// Don't add dot for simple property+array access like "hobbies[0]"
				// Only add dot for complex cases like "a.b[0]" -> "a.b.[0]"
				if char == '[' && pp.isSimplePropertyArrayAccess(path, i) {
					// Skip adding dot for simple cases
				} else {
					sb.WriteRune('.')
				}
			}
		}
		sb.WriteRune(char)
	}

	return sb.String()
}

// NeedsLegacyHandling determines if a path needs legacy complex handling
func (pp *pathParser) NeedsLegacyHandling(path string) bool {
	// Only use legacy handling for very specific complex patterns
	legacyPatterns := []string{
		// Add patterns that require legacy handling
	}

	for _, pattern := range legacyPatterns {
		if strings.Contains(path, pattern) {
			return true
		}
	}

	// Check for multiple consecutive extractions (very complex patterns)
	extractCount := strings.Count(path, "{")
	if extractCount > 4 {
		return true
	}

	// Check for step-based slicing (e.g., [::2], [1:5:2])
	if strings.Contains(path, "::") || (strings.Count(path, ":") > 2) {
		return true
	}

	return false
}

// parseDotNotation parses dot notation paths like "user.name" or "users[0].name"
func (pp *pathParser) parseDotNotation(path string) ([]PathSegmentInfo, error) {
	var segments []PathSegmentInfo

	// Preprocess the path
	preprocessedPath := pp.PreprocessPath(path)

	// Smart split that respects extraction and array operation boundaries
	parts := pp.smartSplitPath(preprocessedPath)

	for _, part := range parts {
		if part == "" {
			continue
		}

		segmentInfo, err := pp.parsePathSegment(part)
		if err != nil {
			return nil, fmt.Errorf("failed to parse segment '%s': %w", part, err)
		}

		segments = append(segments, segmentInfo...)
	}

	return segments, nil
}

// parseJSONPointer parses JSON Pointer format paths
func (pp *pathParser) parseJSONPointer(path string) ([]PathSegmentInfo, error) {
	if path == "/" {
		return []PathSegmentInfo{}, nil
	}

	// Remove leading slash and split
	path = strings.TrimPrefix(path, "/")
	parts := strings.Split(path, "/")

	var segments []PathSegmentInfo
	for _, part := range parts {
		// Unescape JSON Pointer special characters
		part = strings.ReplaceAll(part, "~1", "/")
		part = strings.ReplaceAll(part, "~0", "~")

		// Check if it's a numeric index
		if pp.isNumericIndex(part) {
			index, err := strconv.Atoi(part)
			if err != nil {
				return nil, fmt.Errorf("invalid numeric index '%s': %w", part, err)
			}
			segments = append(segments, PathSegmentInfo{
				Type:  "array",
				Value: fmt.Sprintf("[%d]", index),
				Index: index,
			})
		} else {
			// Property access
			segments = append(segments, PathSegmentInfo{
				Type:  "property",
				Value: part,
				Key:   part,
			})
		}
	}

	return segments, nil
}

// smartSplitPath splits path by dots while respecting extraction and array operation boundaries
func (pp *pathParser) smartSplitPath(path string) []string {
	var parts []string
	var current strings.Builder
	var braceDepth int
	var bracketDepth int

	for _, char := range path {
		switch char {
		case '{':
			braceDepth++
			current.WriteRune(char)
		case '}':
			braceDepth--
			current.WriteRune(char)
		case '[':
			bracketDepth++
			current.WriteRune(char)
		case ']':
			bracketDepth--
			current.WriteRune(char)
		case '.':
			// Only split on dots when we're not inside braces or brackets
			if braceDepth == 0 && bracketDepth == 0 {
				if current.Len() > 0 {
					parts = append(parts, current.String())
					current.Reset()
				}
			} else {
				current.WriteRune(char)
			}
		default:
			current.WriteRune(char)
		}
	}

	// Add the last part
	if current.Len() > 0 {
		parts = append(parts, current.String())
	}

	return parts
}

// parsePathSegment parses a single path segment
func (pp *pathParser) parsePathSegment(part string) ([]PathSegmentInfo, error) {
	if part == "" {
		return []PathSegmentInfo{}, nil
	}

	// Check for complex segments that may contain mixed syntax
	if pp.isComplexSegment(part) {
		return pp.parseComplexSegment(part)
	}

	// Simple segment parsing
	segment, err := pp.parseSimpleSegment(part)
	if err != nil {
		return nil, err
	}

	return []PathSegmentInfo{segment}, nil
}

// parseSimpleSegment parses a simple path segment
func (pp *pathParser) parseSimpleSegment(part string) (PathSegmentInfo, error) {
	// Check for extraction syntax first
	if matches := pp.extractPattern.FindStringSubmatch(part); matches != nil {
		return PathSegmentInfo{
			Type:    "extract",
			Value:   part,
			Extract: matches[1],
		}, nil
	}

	// Check for array slice syntax
	if matches := pp.slicePattern.FindStringSubmatch(part); matches != nil {
		segment := PathSegmentInfo{
			Type:  "slice",
			Value: part,
		}

		// Parse start
		if matches[1] != "" {
			start, err := strconv.Atoi(matches[1])
			if err != nil {
				return PathSegmentInfo{}, fmt.Errorf("invalid slice start: %s", matches[1])
			}
			segment.Start = &start
		}

		// Parse end
		if matches[2] != "" {
			end, err := strconv.Atoi(matches[2])
			if err != nil {
				return PathSegmentInfo{}, fmt.Errorf("invalid slice end: %s", matches[2])
			}
			segment.End = &end
		}

		// Parse step
		if matches[3] != "" {
			step, err := strconv.Atoi(matches[3])
			if err != nil {
				return PathSegmentInfo{}, fmt.Errorf("invalid slice step: %s", matches[3])
			}
			// Step cannot be zero
			if step == 0 {
				return PathSegmentInfo{}, fmt.Errorf("slice step cannot be zero")
			}
			segment.Step = &step
		}

		return segment, nil
	}

	// Check for array index syntax
	if matches := pp.arrayIndexPattern.FindStringSubmatch(part); matches != nil {
		index, err := strconv.Atoi(matches[1])
		if err != nil {
			return PathSegmentInfo{}, fmt.Errorf("invalid array index: %s", matches[1])
		}

		return PathSegmentInfo{
			Type:  "array",
			Value: part,
			Index: index,
		}, nil
	}

	// Check for property with array/slice access (e.g., "users[0]" or "items[1:3]")
	if bracketStart := strings.IndexByte(part, '['); bracketStart >= 0 {
		if bracketEnd := strings.IndexByte(part[bracketStart:], ']'); bracketEnd >= 0 {
			property := part[:bracketStart]
			bracketContent := part[bracketStart+1 : bracketStart+bracketEnd]

			// Check if it's slice syntax (contains colon)
			if strings.Contains(bracketContent, ":") {
				// Parse as slice
				segment := PathSegmentInfo{
					Type:  "slice",
					Value: part,
					Key:   property,
				}

				// Use regex to parse slice parameters
				if matches := pp.slicePattern.FindStringSubmatch(part[bracketStart:]); matches != nil {
					// Parse start
					if matches[1] != "" {
						start, err := strconv.Atoi(matches[1])
						if err != nil {
							return PathSegmentInfo{}, fmt.Errorf("invalid slice start: %s", matches[1])
						}
						segment.Start = &start
					}

					// Parse end
					if matches[2] != "" {
						end, err := strconv.Atoi(matches[2])
						if err != nil {
							return PathSegmentInfo{}, fmt.Errorf("invalid slice end: %s", matches[2])
						}
						segment.End = &end
					}

					// Parse step
					if matches[3] != "" {
						step, err := strconv.Atoi(matches[3])
						if err != nil {
							return PathSegmentInfo{}, fmt.Errorf("invalid slice step: %s", matches[3])
						}
						// Step cannot be zero
						if step == 0 {
							return PathSegmentInfo{}, fmt.Errorf("slice step cannot be zero")
						}
						segment.Step = &step
					}

					return segment, nil
				} else {
					return PathSegmentInfo{}, fmt.Errorf("invalid slice syntax in '%s'", part)
				}
			} else {
				// Parse as simple array index
				index, err := strconv.Atoi(bracketContent)
				if err != nil {
					return PathSegmentInfo{}, fmt.Errorf("invalid array index in '%s': %s", part, bracketContent)
				}

				return PathSegmentInfo{
					Type:  "array",
					Value: part,
					Key:   property,
					Index: index,
				}, nil
			}
		}
	}

	// Simple property access
	return PathSegmentInfo{
		Type:  "property",
		Value: part,
		Key:   part,
	}, nil
}

// parseComplexSegment parses complex segments that may contain mixed syntax
func (pp *pathParser) parseComplexSegment(part string) ([]PathSegmentInfo, error) {
	// For now, delegate to the internal parser for complex segments
	parser := internal.NewPathParser()
	internalSegments, err := parser.ParseComplexSegment(part)
	if err != nil {
		return nil, err
	}

	// Convert internal segments to PathSegmentInfo
	var segments []PathSegmentInfo
	for _, seg := range internalSegments {
		segmentInfo := PathSegmentInfo{
			Type:    seg.TypeString(),
			Value:   seg.Value,
			Key:     seg.Key,
			Index:   seg.Index,
			Extract: seg.Extract,
			IsFlat:  seg.IsFlat,
		}

		// Handle slice parameters
		if seg.Start != nil {
			segmentInfo.Start = seg.Start
		}
		if seg.End != nil {
			segmentInfo.End = seg.End
		}
		if seg.Step != nil {
			segmentInfo.Step = seg.Step
		}

		segments = append(segments, segmentInfo)
	}

	return segments, nil
}

// isComplexSegment checks if a segment contains complex syntax
func (pp *pathParser) isComplexSegment(part string) bool {
	// Check for extraction followed by array operations
	if strings.Contains(part, "{") && (strings.Contains(part, "[") || strings.Contains(part, ":")) {
		return true
	}

	// Check for multiple operations in one segment
	braceCount := strings.Count(part, "{")
	bracketCount := strings.Count(part, "[")

	return braceCount > 1 || bracketCount > 1 || (braceCount > 0 && bracketCount > 0)
}

// validateBrackets validates that brackets and braces are properly matched
func (pp *pathParser) validateBrackets(path string) error {
	var braceDepth, bracketDepth int

	for i, char := range path {
		switch char {
		case '{':
			braceDepth++
		case '}':
			braceDepth--
			if braceDepth < 0 {
				return fmt.Errorf("unmatched closing brace at position %d", i)
			}
		case '[':
			bracketDepth++
		case ']':
			bracketDepth--
			if bracketDepth < 0 {
				return fmt.Errorf("unmatched closing bracket at position %d", i)
			}
		}
	}

	if braceDepth > 0 {
		return fmt.Errorf("unmatched opening brace")
	}
	if bracketDepth > 0 {
		return fmt.Errorf("unmatched opening bracket")
	}

	return nil
}

// validatePathSyntax validates path syntax for common errors
func (pp *pathParser) validatePathSyntax(path string) error {
	// Check for empty extraction
	if strings.Contains(path, "{}") {
		return fmt.Errorf("empty extraction syntax not allowed")
	}

	// Check for empty array access
	if strings.Contains(path, "[]") {
		return fmt.Errorf("empty array access not allowed")
	}

	// Check for invalid characters in property names
	// This is a basic check - more sophisticated validation could be added
	if strings.Contains(path, "..") {
		return fmt.Errorf("consecutive dots not allowed")
	}

	return nil
}

// needsDotBefore checks if a dot should be added before a character
func (pp *pathParser) needsDotBefore(prevChar rune) bool {
	// Add dot before [ or { if the previous character is alphanumeric or underscore
	return (prevChar >= 'a' && prevChar <= 'z') ||
		(prevChar >= 'A' && prevChar <= 'Z') ||
		(prevChar >= '0' && prevChar <= '9') ||
		prevChar == '_' || prevChar == ']' || prevChar == '}'
}

// isSimplePropertyArrayAccess checks if this is a simple property+array access pattern
func (pp *pathParser) isSimplePropertyArrayAccess(path string, bracketIndex int) bool {
	// Check if this looks like "property[index]" without any dots before
	if bracketIndex == 0 {
		return false
	}

	// Find the start of the current property name
	start := 0
	for i := bracketIndex - 1; i >= 0; i-- {
		if path[i] == '.' {
			start = i + 1
			break
		}
	}

	// Extract the property name
	propertyName := path[start:bracketIndex]

	// Check if it's a simple property name (no dots, no complex syntax)
	if strings.Contains(propertyName, ".") || strings.Contains(propertyName, "{") || strings.Contains(propertyName, "[") {
		return false
	}

	// Check if the property name is a valid identifier
	if len(propertyName) == 0 {
		return false
	}

	// Simple heuristic: if there's no dot immediately before the property name,
	// and the property name is a simple identifier, treat it as simple access
	return start == 0 || (start > 0 && path[start-1] == '.')
}

// isNumericIndex checks if a string represents a numeric index
func (pp *pathParser) isNumericIndex(s string) bool {
	if s == "" {
		return false
	}

	// Handle negative indices
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
