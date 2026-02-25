package internal

import (
	"fmt"
	"strconv"
	"strings"
)

// EscapeJSONPointer escapes special characters for JSON Pointer
// Uses single-pass algorithm to avoid multiple allocations
func EscapeJSONPointer(s string) string {
	// Fast path: check if escaping is needed
	hasTilde := false
	hasSlash := false
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '~':
			hasTilde = true
		case '/':
			hasSlash = true
		}
	}
	if !hasTilde && !hasSlash {
		return s // No allocation for simple strings
	}

	// Single-pass escaping with strings.Builder
	var sb strings.Builder
	sb.Grow(len(s) + 4) // Pre-allocate with some extra space for escapes

	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '~':
			sb.WriteString("~0")
		case '/':
			sb.WriteString("~1")
		default:
			sb.WriteByte(s[i])
		}
	}
	return sb.String()
}

// UnescapeJSONPointer unescapes JSON Pointer special characters
// Uses single-pass algorithm to avoid multiple allocations
func UnescapeJSONPointer(s string) string {
	// Fast path: check if unescaping is needed
	hasEscape := false
	for i := 0; i < len(s); i++ {
		if s[i] == '~' && i+1 < len(s) && (s[i+1] == '0' || s[i+1] == '1') {
			hasEscape = true
			break
		}
	}
	if !hasEscape {
		return s // No allocation for simple strings
	}

	// Single-pass unescaping with strings.Builder
	var sb strings.Builder
	sb.Grow(len(s))

	for i := 0; i < len(s); i++ {
		if s[i] == '~' && i+1 < len(s) {
			switch s[i+1] {
			case '0':
				sb.WriteByte('~')
				i++
			case '1':
				sb.WriteByte('/')
				i++
			default:
				sb.WriteByte(s[i])
			}
		} else {
			sb.WriteByte(s[i])
		}
	}
	return sb.String()
}

// Bit flags for PathSegment fields to avoid pointer allocations
const (
	FlagIsNegative uint8 = 1 << iota
	FlagIsWildcard
	FlagIsFlat
	FlagHasStart
	FlagHasEnd
	FlagHasStep
)

// PathSegment represents a single segment in a JSON path
// Optimized to avoid pointer allocations by using direct values and bit flags
type PathSegment struct {
	Type  PathSegmentType
	Key   string // Used for PropertySegment and ExtractSegment
	Index int    // Used for ArrayIndexSegment and slice start
	End   int    // Direct value (was *int) for ArraySliceSegment
	Step  int    // Direct value (was *int) for ArraySliceSegment
	Flags uint8  // Bit-packed flags
}

// HasStart returns true if slice has a start value
func (ps *PathSegment) HasStart() bool { return ps.Flags&FlagHasStart != 0 }

// HasEnd returns true if slice has an end value
func (ps *PathSegment) HasEnd() bool { return ps.Flags&FlagHasEnd != 0 }

// HasStep returns true if slice has a step value
func (ps *PathSegment) HasStep() bool { return ps.Flags&FlagHasStep != 0 }

// IsNegativeIndex returns true if Index is negative
func (ps *PathSegment) IsNegativeIndex() bool { return ps.Flags&FlagIsNegative != 0 }

// IsWildcardSegment returns true for WildcardSegment
func (ps *PathSegment) IsWildcardSegment() bool { return ps.Flags&FlagIsWildcard != 0 }

// IsFlatExtract returns true for flat extraction
func (ps *PathSegment) IsFlatExtract() bool { return ps.Flags&FlagIsFlat != 0 }

// GetStart returns the start value and whether it was set
func (ps *PathSegment) GetStart() (int, bool) { return ps.Index, ps.HasStart() }

// GetEnd returns the end value and whether it was set
func (ps *PathSegment) GetEnd() (int, bool) { return ps.End, ps.HasEnd() }

// GetStep returns the step value and whether it was set
func (ps *PathSegment) GetStep() (int, bool) { return ps.Step, ps.HasStep() }

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
	var flags uint8
	if index < 0 {
		flags |= FlagIsNegative
	}
	return PathSegment{
		Type:  ArrayIndexSegment,
		Index: index,
		Flags: flags,
	}
}

// NewArraySliceSegment creates an array slice access segment
// Now accepts direct values instead of pointers to avoid heap allocations
func NewArraySliceSegment(start, end, step int, hasStart, hasEnd, hasStep bool) PathSegment {
	var flags uint8
	if hasStart {
		flags |= FlagHasStart
	}
	if hasEnd {
		flags |= FlagHasEnd
	}
	if hasStep {
		flags |= FlagHasStep
	}
	return PathSegment{
		Type:  ArraySliceSegment,
		Index: start, // Use Index field for start value
		End:   end,
		Step:  step,
		Flags: flags,
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

	var flags uint8
	if isFlat {
		flags |= FlagIsFlat
	}
	return PathSegment{
		Type:  ExtractSegment,
		Key:   actualExtract,
		Flags: flags,
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
// PERFORMANCE: Pre-calculates segment count to avoid slice growth allocations
func parseDotNotation(path string) ([]PathSegment, error) {
	// Pre-calculate segment count for better allocation
	// Count dots outside brackets and add 1 for the initial segment
	dotCount := 0
	bracketDepth := 0
	for i := 0; i < len(path); i++ {
		switch path[i] {
		case '[':
			bracketDepth++
		case ']':
			bracketDepth--
		case '.':
			if bracketDepth == 0 {
				dotCount++
			}
		}
	}

	// Estimate segment count (dots + 1, with extra for array indices)
	estimatedSegments := dotCount + 1
	segments := make([]PathSegment, 0, estimatedSegments)

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
				var flags uint8
				if index < 0 {
					flags |= FlagIsNegative
				}
				segments = append(segments, PathSegment{
					Type:  ArrayIndexSegment,
					Index: index,
					Flags: flags,
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
// Optimized version: uses byte index tracking instead of strings.Builder to reduce allocations
func smartSplitPath(path string) []string {
	// Pre-estimate capacity: count dots outside brackets
	dotCount := 0
	braceDepth := 0
	bracketDepth := 0
	for i := 0; i < len(path); i++ {
		c := path[i]
		switch c {
		case '{':
			braceDepth++
		case '}':
			braceDepth--
		case '[':
			bracketDepth++
		case ']':
			bracketDepth--
		case '.':
			if braceDepth == 0 && bracketDepth == 0 {
				dotCount++
			}
		}
	}

	// Fast path for simple paths (no brackets or braces)
	if braceDepth == 0 && bracketDepth == 0 && dotCount > 0 {
		// Use strings.Split for simple paths - it's well optimized
		return strings.Split(path, ".")
	}

	// Estimate capacity from dot count
	parts := make([]string, 0, dotCount+1)
	start := 0
	braceDepth = 0
	bracketDepth = 0

	for i := 0; i < len(path); i++ {
		c := path[i]
		switch c {
		case '{':
			braceDepth++
		case '}':
			braceDepth--
		case '[':
			bracketDepth++
		case ']':
			bracketDepth--
		case '.':
			// Only split on dots when we're not inside braces or brackets
			if braceDepth == 0 && bracketDepth == 0 {
				if i > start {
					parts = append(parts, path[start:i])
				}
				start = i + 1
			}
		}
	}

	// Add the last part
	if start < len(path) {
		parts = append(parts, path[start:])
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

			var flags uint8
			if isFlat {
				flags |= FlagIsFlat
			}
			segments = append(segments, PathSegment{
				Type:  ExtractSegment,
				Key:   actualExtract,
				Flags: flags,
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
	// Check for slice notation using direct byte scan (no allocation)
	hasColon := false
	for i := 0; i < len(arrayPart); i++ {
		if arrayPart[i] == ':' {
			hasColon = true
			break
		}
	}
	if hasColon {
		return parseSliceAccess(arrayPart)
	}

	// Check for wildcard
	if arrayPart == "*" {
		return PathSegment{
			Type:  WildcardSegment,
			Flags: FlagIsWildcard,
		}, nil
	}

	// Fast path for single digit indices (0-9)
	if len(arrayPart) == 1 && arrayPart[0] >= '0' && arrayPart[0] <= '9' {
		index := int(arrayPart[0] - '0')
		return PathSegment{
			Type:  ArrayIndexSegment,
			Index: index,
		}, nil
	}

	// Fast path for negative single digit (-9 to -0)
	if len(arrayPart) == 2 && arrayPart[0] == '-' && arrayPart[1] >= '0' && arrayPart[1] <= '9' {
		index := -int(arrayPart[1] - '0')
		return PathSegment{
			Type:  ArrayIndexSegment,
			Index: index,
			Flags: FlagIsNegative,
		}, nil
	}

	// Simple index access
	index, err := strconv.Atoi(arrayPart)
	if err != nil {
		return PathSegment{}, fmt.Errorf("invalid array index '%s': %w", arrayPart, err)
	}

	var flags uint8
	if index < 0 {
		flags |= FlagIsNegative
	}
	return PathSegment{
		Type:  ArrayIndexSegment,
		Index: index,
		Flags: flags,
	}, nil
}

// parseSliceAccess parses slice notation like "1:3", "::2", "::-1"
// Optimized to avoid strings.Split allocation
func parseSliceAccess(slicePart string) (PathSegment, error) {
	// Find colon positions without allocation
	colon1 := -1
	colon2 := -1
	for i := 0; i < len(slicePart); i++ {
		if slicePart[i] == ':' {
			if colon1 == -1 {
				colon1 = i
			} else {
				colon2 = i
				break
			}
		}
	}

	// Validate colon positions
	if colon1 == -1 {
		return PathSegment{}, fmt.Errorf("invalid slice syntax '%s': no colon found", slicePart)
	}
	if colon2 == -1 {
		// Two-part slice (start:end)
		colon2 = len(slicePart) // Mark as no second colon
	} else if colon2-colon1 == 1 {
		// Could be ::step or :: (check if there's more after second colon)
	}

	var start, end, step int
	var flags uint8

	// Parse start (before first colon)
	if colon1 > 0 {
		startVal, err := strconv.Atoi(slicePart[:colon1])
		if err != nil {
			return PathSegment{}, fmt.Errorf("invalid slice start '%s': %w", slicePart[:colon1], err)
		}
		start = startVal
		flags |= FlagHasStart
	}

	// Parse end (between colons)
	if colon2 > colon1+1 {
		// There's content between colons
		endStr := slicePart[colon1+1 : colon2]
		endVal, err := strconv.Atoi(endStr)
		if err != nil {
			return PathSegment{}, fmt.Errorf("invalid slice end '%s': %w", endStr, err)
		}
		end = endVal
		flags |= FlagHasEnd
	}

	// Parse step (after second colon, if exists)
	if colon2 < len(slicePart) && colon2 > colon1 {
		// We have a second colon, check for step
		stepStr := slicePart[colon2+1:]
		if stepStr != "" {
			stepVal, err := strconv.Atoi(stepStr)
			if err != nil {
				return PathSegment{}, fmt.Errorf("invalid slice step '%s': %w", stepStr, err)
			}
			if stepVal == 0 {
				return PathSegment{}, fmt.Errorf("slice step cannot be zero")
			}
			step = stepVal
			flags |= FlagHasStep
		}
	}

	return PathSegment{
		Type:  ArraySliceSegment,
		Index: start, // Use Index field for start value
		End:   end,
		Step:  step,
		Flags: flags,
	}, nil
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
		// Unescape JSON Pointer special characters using helper
		part = UnescapeJSONPointer(part)

		// Try to parse as numeric index
		if index, err := strconv.Atoi(part); err == nil {
			var flags uint8
			if index < 0 {
				flags |= FlagIsNegative
			}
			segments = append(segments, PathSegment{
				Type:  ArrayIndexSegment,
				Index: index,
				Flags: flags,
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
// Uses single-pass validation for optimal performance (no regex)
// PERFORMANCE: Added fast path for simple property paths
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

	// FAST PATH: Simple property path (alphanumeric + dots + underscores only)
	// This handles the vast majority of cases without complex validation
	isSimple := true
	depth := 1 // Start with 1 for the first segment
	for i := 0; i < pathLen; i++ {
		c := path[i]
		// Check for simple characters only
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') ||
			(c >= '0' && c <= '9') || c == '_' || c == '.') {
			isSimple = false
			break
		}
		if c == '.' {
			depth++
		}
	}

	if isSimple {
		// Simple validation passed - just check depth
		if depth > maxPathDepth {
			return fmt.Errorf("path too deep: %d segments (max %d)", depth, maxPathDepth)
		}
		return nil
	}

	// SLOW PATH: Complex path with brackets, braces, etc.
	// Single-pass validation for all checks
	var prevChar byte
	depth = 0
	inBracket := false
	bracketStart := 0

	for i := 0; i < pathLen; i++ {
		c := path[i]

		// Check for null bytes and control characters
		if c == 0 || c < 32 {
			return fmt.Errorf("path contains invalid control characters at position %d", i)
		}

		// Track depth and validate brackets/array indices
		switch c {
		case '.':
			if !inBracket {
				depth++
			}
		case '[':
			if !inBracket {
				inBracket = true
				bracketStart = i
				depth++
			}
		case ']':
			if inBracket {
				// Validate array index/slice content inside brackets
				content := path[bracketStart+1 : i]
				if err := validateArrayIndexContent(content, maxArrayIndex); err != nil {
					return err
				}
				inBracket = false
			}
		}

		// Check for path traversal patterns
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

	if depth > maxPathDepth {
		return fmt.Errorf("path too deep: %d segments (max %d)", depth, maxPathDepth)
	}

	return nil
}

// validateArrayIndexContent validates array index or slice content
// PERFORMANCE: Optimized to avoid strings.Contains and strings.Split allocations
func validateArrayIndexContent(content string, maxIndex int) error {
	if content == "" {
		return fmt.Errorf("empty array index")
	}

	// Handle wildcard - single character check
	if content == "*" {
		return nil
	}

	// Fast path: scan for colon without allocation
	hasColon := false
	for i := 0; i < len(content); i++ {
		if content[i] == ':' {
			hasColon = true
			break
		}
	}

	if hasColon {
		// Parse slice notation without strings.Split allocation
		start := 0
		for i := 0; i <= len(content); i++ {
			if i == len(content) || content[i] == ':' {
				if i > start {
					part := content[start:i]
					if err := validateNumericIndex(part, maxIndex); err != nil {
						return err
					}
				}
				start = i + 1
			}
		}
		return nil
	}

	// Simple index
	return validateNumericIndex(content, maxIndex)
}

// validateNumericIndex validates a single numeric index
// PERFORMANCE: Manual parsing avoids strconv.Atoi allocation
func validateNumericIndex(s string, maxIndex int) error {
	// Allow negative sign
	sLen := len(s)
	if sLen == 0 {
		return fmt.Errorf("empty index")
	}

	negative := false
	start := 0
	if s[0] == '-' {
		negative = true
		start = 1
		if sLen == 1 {
			return fmt.Errorf("invalid index: %s", s)
		}
	}

	// Fast path for single digit
	if sLen-start == 1 {
		c := s[start]
		if c < '0' || c > '9' {
			return fmt.Errorf("invalid array index: %s", s)
		}
		return nil
	}

	// Check all characters are digits and parse value
	index := 0
	for i := start; i < sLen; i++ {
		c := s[i]
		if c < '0' || c > '9' {
			return fmt.Errorf("invalid array index: %s", s)
		}
		// Check for overflow before multiplying
		if index > (maxIndex / 10) {
			return fmt.Errorf("array index out of reasonable range")
		}
		index = index*10 + int(c-'0')
	}

	if negative {
		index = -index
	}

	if index < -maxIndex || index > maxIndex {
		return fmt.Errorf("array index out of reasonable range: %d (range: %d to %d)",
			index, -maxIndex, maxIndex)
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

		if ps.HasStart() {
			start = strconv.Itoa(ps.Index) // Index stores start value for slices
		}
		if ps.HasEnd() {
			end = strconv.Itoa(ps.End)
		}
		if ps.HasStep() {
			step = ":" + strconv.Itoa(ps.Step)
		}

		return fmt.Sprintf("[%s:%s%s]", start, end, step)
	case WildcardSegment:
		return "[*]"
	case ExtractSegment:
		if ps.IsFlatExtract() {
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

// ParseAndValidateArrayIndex parses a string as an array index and validates it against array length
// Returns the index and true if successful, 0 and false otherwise
func ParseAndValidateArrayIndex(s string, arrayLength int) (int, bool) {
	index, ok := ParseArrayIndex(s)
	if !ok {
		return 0, false
	}

	// Handle negative indices
	if index < 0 {
		index = arrayLength + index
	}

	// Validate bounds
	if index < 0 || index >= arrayLength {
		return 0, false
	}

	return index, true
}
