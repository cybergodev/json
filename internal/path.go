package internal

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// PathSegment represents a single segment in a JSON path
type PathSegment struct {
	Type       PathSegmentType
	Key        string // Used for PropertySegment and ExtractSegment
	Index      int    // Used for ArrayIndexSegment
	Start      *int   // Used for ArraySliceSegment
	End        *int   // Used for ArraySliceSegment
	Step       *int   // Used for ArraySliceSegment
	IsNegative bool   // True if Index is negative
	IsWildcard bool   // True for WildcardSegment
	IsFlat     bool   // True for flat extraction
}

// PathSegmentType represents the type of path segment
type PathSegmentType int

const (
	PropertySegment PathSegmentType = iota
	ArrayIndexSegment
	ArraySliceSegment
	WildcardSegment
	RecursiveSegment
	FilterSegment
	ExtractSegment // For extract operations
)

// String returns the string representation of PathSegmentType
func (pst PathSegmentType) String() string {
	switch pst {
	case PropertySegment:
		return "property"
	case ArrayIndexSegment:
		return "array"
	case ArraySliceSegment:
		return "slice"
	case WildcardSegment:
		return "wildcard"
	case RecursiveSegment:
		return "recursive"
	case FilterSegment:
		return "filter"
	case ExtractSegment:
		return "extract"
	default:
		return "unknown"
	}
}

// TypeString returns the string type for the segment
func (ps PathSegment) TypeString() string {
	// Use the Type enum for consistent behavior
	return ps.Type.String()
}

// Helper functions to create PathSegments with proper types

// NewPropertySegment creates a property access segment
func NewPropertySegment(key string) PathSegment {
	return PathSegment{
		Type: PropertySegment,
		Key:  key,
	}
}

// NewArrayIndexSegment creates an array index access segment
func NewArrayIndexSegment(index int) PathSegment {
	return PathSegment{
		Type:       ArrayIndexSegment,
		Index:      index,
		IsNegative: index < 0,
	}
}

// NewArraySliceSegment creates an array slice access segment
func NewArraySliceSegment(start, end, step *int) PathSegment {
	return PathSegment{
		Type:  ArraySliceSegment,
		Start: start,
		End:   end,
		Step:  step,
	}
}

// NewExtractSegment creates an extraction segment
func NewExtractSegment(extract string) PathSegment {
	// Check if this is a flat extraction
	isFlat := strings.HasPrefix(extract, "flat:")
	actualExtract := extract
	if isFlat {
		actualExtract = strings.TrimPrefix(extract, "flat:")
	}

	return PathSegment{
		Type:   ExtractSegment,
		Key:    actualExtract,
		IsFlat: isFlat,
	}
}



// ParsePath parses a JSON path string into segments
func ParsePath(path string) ([]PathSegment, error) {
	if path == "" {
		return []PathSegment{}, nil
	}

	// Handle different path formats
	if strings.HasPrefix(path, "/") {
		return parseJSONPointer(path)
	}

	return parseDotNotation(path)
}

// ParseComplexSegment parses a complex segment that may contain mixed syntax
func ParseComplexSegment(part string) ([]PathSegment, error) {
	return parseComplexSegment(part)
}

// parseDotNotation parses dot notation paths like "user.name" or "users[0].name"
func parseDotNotation(path string) ([]PathSegment, error) {
	var segments []PathSegment

	// Smart split that respects extraction and array operation boundaries
	parts := smartSplitPath(path)

	for _, part := range parts {
		if part == "" {
			continue
		}

		// Check for complex mixed syntax like {field}[slice] or property[index]{extract}
		if (strings.Contains(part, "[") && strings.Contains(part, "{")) ||
			(strings.Contains(part, "{") && strings.Contains(part, "}")) {
			propSegments, err := parseComplexSegment(part)
			if err != nil {
				return nil, fmt.Errorf("invalid complex segment '%s': %w", part, err)
			}
			segments = append(segments, propSegments...)
		} else if strings.Contains(part, "[") {
			// Traditional array access patterns
			propSegments, err := parsePropertyWithArray(part)
			if err != nil {
				return nil, fmt.Errorf("invalid array access in '%s': %w", part, err)
			}
			segments = append(segments, propSegments...)
		} else if strings.Contains(part, "{") {
			// Pure extraction syntax
			propSegments, err := parseComplexSegment(part)
			if err != nil {
				return nil, fmt.Errorf("invalid extraction syntax in '%s': %w", part, err)
			}
			segments = append(segments, propSegments...)
		} else {
			if index, err := strconv.Atoi(part); err == nil {
				segments = append(segments, PathSegment{
					Type:       ArrayIndexSegment,
					Index:      index,
					IsNegative: index < 0,
				})
			} else {
				segments = append(segments, PathSegment{
					Type: PropertySegment,
					Key:  part,
				})
			}
		}
	}

	return segments, nil
}

// smartSplitPath splits path by dots while respecting extraction and array operation boundaries
func smartSplitPath(path string) []string {
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

// parsePropertyWithArray parses property access with array notation like "users[0]" or "data[1:3]"
func parsePropertyWithArray(part string) ([]PathSegment, error) {
	var segments []PathSegment

	bracketIndex := strings.Index(part, "[")
	if bracketIndex > 0 {
		propertyName := part[:bracketIndex]
		segments = append(segments, PathSegment{
			Type: PropertySegment,
			Key:  propertyName,
		})
	}

	// Parse all array access patterns
	remaining := part[bracketIndex:]
	for len(remaining) > 0 {
		if !strings.HasPrefix(remaining, "[") {
			return nil, fmt.Errorf("expected '[' but found '%s'", remaining)
		}

		closeBracket := strings.Index(remaining, "]")
		if closeBracket == -1 {
			return nil, fmt.Errorf("missing closing bracket in '%s'", remaining)
		}

		arrayPart := remaining[1:closeBracket]
		segment, err := parseArrayAccess(arrayPart)
		if err != nil {
			return nil, fmt.Errorf("invalid array access '%s': %w", arrayPart, err)
		}

		segments = append(segments, segment)
		remaining = remaining[closeBracket+1:]
	}

	return segments, nil
}

// parseComplexSegment parses complex segments that may contain mixed syntax like {field}[slice]
func parseComplexSegment(part string) ([]PathSegment, error) {
	var segments []PathSegment
	remaining := part

	for len(remaining) > 0 {
		// Check for extraction syntax first
		if strings.HasPrefix(remaining, "{") {
			braceEnd := strings.Index(remaining, "}")
			if braceEnd == -1 {
				return nil, fmt.Errorf("missing closing brace in '%s'", remaining)
			}

			extractPart := remaining[1:braceEnd]

			// Check if this is a flat extraction
			isFlat := strings.HasPrefix(extractPart, "flat:")
			actualExtract := extractPart
			if isFlat {
				actualExtract = strings.TrimPrefix(extractPart, "flat:")
			}

			// Validate extraction field name
			if actualExtract == "" {
				return nil, fmt.Errorf("empty extraction field in '%s'", remaining[:braceEnd+1])
			}

			segments = append(segments, PathSegment{
				Type:   ExtractSegment,
				Key:    actualExtract,
				IsFlat: isFlat,
			})

			remaining = remaining[braceEnd+1:]
			continue
		}

		// Check for array access
		if strings.HasPrefix(remaining, "[") {
			bracketEnd := strings.Index(remaining, "]")
			if bracketEnd == -1 {
				return nil, fmt.Errorf("missing closing bracket in '%s'", remaining)
			}

			arrayPart := remaining[1:bracketEnd]

			// Validate array access syntax
			if arrayPart == "" {
				return nil, fmt.Errorf("empty array access in '%s'", remaining[:bracketEnd+1])
			}

			segment, err := parseArrayAccess(arrayPart)
			if err != nil {
				return nil, fmt.Errorf("invalid array access '%s': %w", arrayPart, err)
			}

			segments = append(segments, segment)
			remaining = remaining[bracketEnd+1:]
			continue
		}

		// If we reach here, it's a property name at the beginning
		// Find the next special character
		nextSpecial := len(remaining)
		for i, char := range remaining {
			if char == '[' || char == '{' {
				nextSpecial = i
				break
			}
		}

		if nextSpecial > 0 {
			propertyName := remaining[:nextSpecial]
			segments = append(segments, PathSegment{
				Type: PropertySegment,
				Key:  propertyName,
			})
			remaining = remaining[nextSpecial:]
		} else {
			// No more special characters, treat the rest as property name
			if remaining != "" {
				segments = append(segments, PathSegment{
					Type: PropertySegment,
					Key:  remaining,
				})
			}
			break
		}
	}

	return segments, nil
}

// parseArrayAccess parses array access patterns like "0", "-1", "1:3", "::2"
func parseArrayAccess(arrayPart string) (PathSegment, error) {
	// Check for slice notation
	if strings.Contains(arrayPart, ":") {
		return parseSliceAccess(arrayPart)
	}

	// Check for wildcard
	if arrayPart == "*" {
		return PathSegment{
			Type:       WildcardSegment,
			IsWildcard: true,
		}, nil
	}

	// Simple index access
	index, err := strconv.Atoi(arrayPart)
	if err != nil {
		return PathSegment{}, fmt.Errorf("invalid array index '%s': %w", arrayPart, err)
	}

	return PathSegment{
		Type:       ArrayIndexSegment,
		Index:      index,
		IsNegative: index < 0,
	}, nil
}

// parseSliceAccess parses slice notation like "1:3", "::2", "::-1"
func parseSliceAccess(slicePart string) (PathSegment, error) {
	parts := strings.Split(slicePart, ":")
	if len(parts) < 2 || len(parts) > 3 {
		return PathSegment{}, fmt.Errorf("invalid slice syntax '%s'", slicePart)
	}

	segment := PathSegment{
		Type: ArraySliceSegment,
	}

	// Parse start
	if parts[0] != "" {
		start, err := strconv.Atoi(parts[0])
		if err != nil {
			return PathSegment{}, fmt.Errorf("invalid slice start '%s': %w", parts[0], err)
		}
		segment.Start = &start
	}

	// Parse end
	if parts[1] != "" {
		end, err := strconv.Atoi(parts[1])
		if err != nil {
			return PathSegment{}, fmt.Errorf("invalid slice end '%s': %w", parts[1], err)
		}
		segment.End = &end
	}

	// Parse step (if provided)
	if len(parts) == 3 && parts[2] != "" {
		step, err := strconv.Atoi(parts[2])
		if err != nil {
			return PathSegment{}, fmt.Errorf("invalid slice step '%s': %w", parts[2], err)
		}
		if step == 0 {
			return PathSegment{}, fmt.Errorf("slice step cannot be zero")
		}
		segment.Step = &step
	}

	return segment, nil
}

// parseJSONPointer parses JSON Pointer format paths like "/users/0/name"
func parseJSONPointer(path string) ([]PathSegment, error) {
	if path == "/" {
		return []PathSegment{{Type: PropertySegment, Key: ""}}, nil
	}

	path = strings.TrimPrefix(path, "/")
	parts := strings.Split(path, "/")

	segments := make([]PathSegment, 0, len(parts))
	for _, part := range parts {
		// Unescape JSON Pointer special characters
		part = strings.ReplaceAll(part, "~1", "/")
		part = strings.ReplaceAll(part, "~0", "~")

		// Try to parse as numeric index
		if index, err := strconv.Atoi(part); err == nil {
			segments = append(segments, PathSegment{
				Type:       ArrayIndexSegment,
				Index:      index,
				IsNegative: index < 0,
			})
			continue
		}

		// Property access
		segments = append(segments, PathSegment{
			Type: PropertySegment,
			Key:  part,
		})
	}

	return segments, nil
}

// ValidatePath validates a path string for security and correctness
func ValidatePath(path string) error {
	const (
		maxPathLength = 1000
		maxPathDepth  = 100
		maxArrayIndex = 10000
	)

	pathLen := len(path)
	if pathLen > maxPathLength {
		return fmt.Errorf("path too long: %d characters (max %d)", pathLen, maxPathLength)
	}

	if pathLen == 0 {
		return nil
	}

	// Check path depth (prevent deeply nested paths)
	segmentCount := strings.Count(path, ".") + strings.Count(path, "[")
	if segmentCount > maxPathDepth {
		return fmt.Errorf("path too deep: %d segments (max %d)", segmentCount, maxPathDepth)
	}

	// Validate array indices are within reasonable range
	arrayPattern := regexp.MustCompile(`\[(-?\d+)`)
	matches := arrayPattern.FindAllStringSubmatch(path, -1)
	for _, match := range matches {
		if len(match) > 1 {
			index, err := strconv.Atoi(match[1])
			if err != nil {
				return fmt.Errorf("invalid array index: %s", match[1])
			}
			if index < -maxArrayIndex || index > maxArrayIndex {
				return fmt.Errorf("array index out of reasonable range: %d (range: %d to %d)",
					index, -maxArrayIndex, maxArrayIndex)
			}
		}
	}

	// Single-pass validation for control characters and dangerous patterns
	var prevChar byte
	for i := 0; i < pathLen; i++ {
		c := path[i]

		// Check for null bytes and control characters
		if c == 0 || c < 32 {
			return fmt.Errorf("path contains invalid control characters at position %d", i)
		}

		// Check for path traversal patterns (optimized)
		if c == '.' && i+1 < pathLen && path[i+1] == '.' {
			if i+2 < pathLen && path[i+2] == '/' {
				return fmt.Errorf("path contains traversal patterns at position %d", i)
			}
		}

		// Check for double slashes
		if c == '/' && prevChar == '/' {
			return fmt.Errorf("path contains traversal patterns at position %d", i)
		}

		// Check for backslashes
		if c == '\\' {
			return fmt.Errorf("path contains traversal patterns at position %d", i)
		}

		// Check for template injection patterns
		if c == '$' || c == '#' {
			if i+1 < pathLen && path[i+1] == '{' {
				return fmt.Errorf("path contains template injection patterns at position %d", i)
			}
		}
		if c == '{' && prevChar == '{' {
			return fmt.Errorf("path contains template injection patterns at position %d", i)
		}

		prevChar = c
	}

	return nil
}

// String returns a string representation of the path segment
func (ps PathSegment) String() string {
	switch ps.Type {
	case PropertySegment:
		return ps.Key
	case ArrayIndexSegment:
		return fmt.Sprintf("[%d]", ps.Index)
	case ArraySliceSegment:
		start := ""
		end := ""
		step := ""

		if ps.Start != nil {
			start = strconv.Itoa(*ps.Start)
		}
		if ps.End != nil {
			end = strconv.Itoa(*ps.End)
		}
		if ps.Step != nil {
			step = ":" + strconv.Itoa(*ps.Step)
		}

		return fmt.Sprintf("[%s:%s%s]", start, end, step)
	case WildcardSegment:
		return "[*]"
	case ExtractSegment:
		if ps.IsFlat {
			return fmt.Sprintf("{flat:%s}", ps.Key)
		}
		return fmt.Sprintf("{%s}", ps.Key)
	default:
		return fmt.Sprintf("[unknown:%v]", ps.Type)
	}
}



// IsArrayAccess returns true if this segment accesses an array
func (ps PathSegment) IsArrayAccess() bool {
	return ps.Type == ArrayIndexSegment || ps.Type == ArraySliceSegment || ps.Type == WildcardSegment
}

// GetArrayIndex returns the array index, handling negative indices
func (ps PathSegment) GetArrayIndex(arrayLength int) (int, error) {
	if ps.Type != ArrayIndexSegment {
		return 0, fmt.Errorf("not an array index segment")
	}

	index := ps.Index
	if index < 0 {
		index = arrayLength + index
	}

	if index < 0 || index >= arrayLength {
		return 0, fmt.Errorf("array index %d out of bounds (length %d)", ps.Index, arrayLength)
	}

	return index, nil
}
