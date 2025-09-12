package internal

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// PathSegment represents a single segment in a JSON path
type PathSegment struct {
	// New structured fields
	Type       PathSegmentType `json:"type"`
	Key        string          `json:"key"`
	Index      int             `json:"index"`
	Start      *int            `json:"start,omitempty"`
	End        *int            `json:"end,omitempty"`
	Step       *int            `json:"step,omitempty"`
	IsNegative bool            `json:"is_negative"`
	IsSlice    bool            `json:"is_slice"`
	IsWildcard bool            `json:"is_wildcard"`
	IsFlat     bool            `json:"is_flat"` // Whether this is a flat extraction operation

	// Distributed operation support
	IsDistributed    bool              `json:"is_distributed"`    // Whether this is a distributed operation
	OperationContext *OperationContext `json:"operation_context"` // Operation context for distributed operations
	MappingInfo      *MappingInfo      `json:"mapping_info"`      // Reverse mapping information

	// Legacy compatibility fields (minimal set for backward compatibility)
	Value   string `json:"value"`   // The actual segment value (for legacy parsing)
	Extract string `json:"extract"` // For extract operations: the property to extract
}

// OperationContext holds context information for distributed operations
type OperationContext struct {
	ExtractionPath string          `json:"extraction_path"` // Path used for extraction
	TargetArrays   []ArrayLocation `json:"target_arrays"`   // Target array locations
	OperationType  string          `json:"operation_type"`  // "get", "set", "delete"
}

// ArrayLocation represents the location of an array in the data structure
type ArrayLocation struct {
	ContainerPath  string `json:"container_path"`   // Path to the container
	ArrayFieldName string `json:"array_field_name"` // Name of the array field
	ContainerRef   any    `json:"-"`                // Reference to the container (not serialized)
}

// MappingInfo holds information for reverse mapping operations
type MappingInfo struct {
	OriginalContainers []any  `json:"-"`                // Original container references
	ArrayFieldName     string `json:"array_field_name"` // Array field name
	TargetIndices      []int  `json:"target_indices"`   // Target indices for each container
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
		Type:  ArrayIndexSegment,
		Index: index,
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
		Type:    ExtractSegment,
		Key:     actualExtract,
		Extract: actualExtract, // For backward compatibility
		IsFlat:  isFlat,
	}
}

// NewLegacyPathSegment creates a PathSegment from legacy string type
func NewLegacyPathSegment(typeStr, value string) PathSegment {
	segment := PathSegment{
		Value: value,
	}

	switch typeStr {
	case "property":
		segment.Type = PropertySegment
		segment.Key = value
	case "array":
		segment.Type = ArrayIndexSegment
		// Try to parse index from value
		if strings.HasPrefix(value, "[") && strings.HasSuffix(value, "]") {
			indexStr := value[1 : len(value)-1]
			if index, err := strconv.Atoi(indexStr); err == nil {
				segment.Index = index
			}
		}
	case "slice":
		segment.Type = ArraySliceSegment
	case "extract":
		segment.Type = ExtractSegment
		if strings.HasPrefix(value, "{") && strings.HasSuffix(value, "}") {
			extract := value[1 : len(value)-1]

			// Check if this is a flat extraction
			isFlat := strings.HasPrefix(extract, "flat:")
			actualExtract := extract
			if isFlat {
				actualExtract = strings.TrimPrefix(extract, "flat:")
			}

			segment.Extract = actualExtract
			segment.Key = actualExtract
			segment.IsFlat = isFlat
		}
	case "pointer":
		segment.Type = PropertySegment // JSON Pointer is treated as property access
	default:
		segment.Type = PropertySegment
		segment.Key = value
	}

	return segment
}

// PathPatterns holds compiled regex patterns for path parsing
type PathPatterns struct {
	dotNotation   *regexp.Regexp
	arrayIndex    *regexp.Regexp
	jsonPointer   *regexp.Regexp
	extractSyntax *regexp.Regexp
	rangeSyntax   *regexp.Regexp

	// Compiled patterns for common operations
	SimpleProperty *regexp.Regexp
	NumericIndex   *regexp.Regexp
}

// NewPathPatterns creates and compiles all path parsing patterns
func NewPathPatterns() *PathPatterns {
	return &PathPatterns{
		dotNotation:    regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`),
		arrayIndex:     regexp.MustCompile(`^\[(-?\d+)\]$`),
		jsonPointer:    regexp.MustCompile(`^/([^/]*)$`),
		extractSyntax:  regexp.MustCompile(`^\{([^}]+)\}$`),
		rangeSyntax:    regexp.MustCompile(`^\[(-?\d*):(-?\d*)(?::(-?\d+))?\]$`),
		SimpleProperty: regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`),
		NumericIndex:   regexp.MustCompile(`^-?\d+$`),
	}
}

// PathParser handles parsing of JSON paths
type PathParser struct {
	patterns *PathPatterns
}

// NewPathParser creates a new path parser
func NewPathParser() *PathParser {
	return &PathParser{
		patterns: NewPathPatterns(),
	}
}

// ParseComplexSegment parses a complex segment that may contain mixed syntax
func (pp *PathParser) ParseComplexSegment(part string) ([]PathSegment, error) {
	return pp.parseComplexSegment(part)
}

// ParsePath parses a JSON path string into segments
func (pp *PathParser) ParsePath(path string) ([]PathSegment, error) {
	if path == "" {
		return []PathSegment{}, nil
	}

	// Handle different path formats
	if strings.HasPrefix(path, "/") {
		return pp.parseJSONPointer(path)
	}

	return pp.parseDotNotation(path)
}

// parseDotNotation parses dot notation paths like "user.name" or "users[0].name"
func (pp *PathParser) parseDotNotation(path string) ([]PathSegment, error) {
	var segments []PathSegment

	// Smart split that respects extraction and array operation boundaries
	parts := pp.smartSplitPath(path)

	// fmt.Printf("DEBUG: Path '%s' split into parts: %v\n", path, parts)

	for _, part := range parts {
		if part == "" {
			continue
		}

		// Check for complex mixed syntax like {field}[slice] or property[index]{extract}
		if (strings.Contains(part, "[") && strings.Contains(part, "{")) ||
			(strings.Contains(part, "{") && strings.Contains(part, "}")) {
			propSegments, err := pp.parseComplexSegment(part)
			if err != nil {
				return nil, fmt.Errorf("invalid complex segment '%s': %w", part, err)
			}
			segments = append(segments, propSegments...)
		} else if strings.Contains(part, "[") {
			// Traditional array access patterns
			propSegments, err := pp.parsePropertyWithArray(part)
			if err != nil {
				return nil, fmt.Errorf("invalid array access in '%s': %w", part, err)
			}
			segments = append(segments, propSegments...)
		} else if strings.Contains(part, "{") {
			// Pure extraction syntax
			propSegments, err := pp.parseComplexSegment(part)
			if err != nil {
				return nil, fmt.Errorf("invalid extraction syntax in '%s': %w", part, err)
			}
			segments = append(segments, propSegments...)
		} else {
			// Check if this is a numeric property that should be treated as array index
			if index, err := strconv.Atoi(part); err == nil {
				// This is a numeric property, treat as array index for dot notation
				segments = append(segments, PathSegment{
					Type:       ArrayIndexSegment,
					Index:      index,
					IsNegative: index < 0,
					Value:      part, // Set Value for backward compatibility
				})
			} else {
				// Simple property access
				segments = append(segments, PathSegment{
					Type:  PropertySegment,
					Key:   part,
					Value: part, // Set Value for backward compatibility
				})
			}
		}
	}

	return segments, nil
}

// smartSplitPath splits path by dots while respecting extraction and array operation boundaries
func (pp *PathParser) smartSplitPath(path string) []string {
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
func (pp *PathParser) parsePropertyWithArray(part string) ([]PathSegment, error) {
	var segments []PathSegment

	// Find the property name before the first bracket
	bracketIndex := strings.Index(part, "[")
	if bracketIndex > 0 {
		propertyName := part[:bracketIndex]
		segments = append(segments, PathSegment{
			Type:  PropertySegment,
			Key:   propertyName,
			Value: propertyName, // Set Value for backward compatibility
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
		segment, err := pp.parseArrayAccess(arrayPart)
		if err != nil {
			return nil, fmt.Errorf("invalid array access '%s': %w", arrayPart, err)
		}

		segments = append(segments, segment)
		remaining = remaining[closeBracket+1:]
	}

	return segments, nil
}

// isExtractionFollowedByArrayOp checks if the part contains extraction followed by array operation
func (pp *PathParser) isExtractionFollowedByArrayOp(part string) bool {
	// Look for ANY extraction followed by array operation, not just the last one
	// This handles patterns like {teams}[0]{members}{name}

	// Find all extraction positions
	pos := 0
	for {
		extractStart := strings.Index(part[pos:], "{")
		if extractStart == -1 {
			break
		}
		extractStart += pos

		extractEnd := strings.Index(part[extractStart:], "}")
		if extractEnd == -1 {
			break
		}
		extractEnd += extractStart

		// Check what follows this extraction
		remaining := part[extractEnd+1:]
		if strings.HasPrefix(remaining, "[") && strings.Contains(remaining, "]") {
			// Found extraction followed by array operation
			return true
		}

		pos = extractEnd + 1
	}

	return false
}

// parseDistributedArrayOperation parses extraction followed by array operation as distributed operation
func (pp *PathParser) parseDistributedArrayOperation(part string) ([]PathSegment, error) {
	var segments []PathSegment

	// Find the FIRST extraction that is followed by array operation
	pos := 0
	var extractionEnd int = -1
	for {
		extractStart := strings.Index(part[pos:], "{")
		if extractStart == -1 {
			break
		}
		extractStart += pos

		extractEnd := strings.Index(part[extractStart:], "}")
		if extractEnd == -1 {
			break
		}
		extractEnd += extractStart

		// Check what follows this extraction
		remaining := part[extractEnd+1:]
		if strings.HasPrefix(remaining, "[") && strings.Contains(remaining, "]") {
			// Found the extraction followed by array operation
			extractionEnd = extractEnd
			break
		}

		pos = extractEnd + 1
	}

	if extractionEnd == -1 {
		return nil, fmt.Errorf("no extraction followed by array operation found in '%s'", part)
	}

	// Split into prefix (everything up to and including the extraction) and the array operation
	prefix := part[:extractionEnd+1]
	arrayPart := part[extractionEnd+1:]

	// Parse all segments before the distributed operation
	if prefix != "" {
		// Find the last extraction in the prefix
		lastExtractStart := strings.LastIndex(prefix, "{")
		if lastExtractStart == -1 {
			return nil, fmt.Errorf("invalid extraction syntax in prefix '%s'", prefix)
		}

		// Parse everything before the last extraction as regular segments
		beforeLastExtract := part[:lastExtractStart]
		if beforeLastExtract != "" {
			beforeSegments, err := pp.parseComplexSegmentRecursive(beforeLastExtract)
			if err != nil {
				return nil, err
			}
			segments = append(segments, beforeSegments...)
		}

		// Parse the last extraction
		lastExtractPart := prefix[lastExtractStart:]
		if strings.HasPrefix(lastExtractPart, "{") && strings.HasSuffix(lastExtractPart, "}") {
			extract := lastExtractPart[1 : len(lastExtractPart)-1]

			// Check if this is a flat extraction
			isFlat := strings.HasPrefix(extract, "flat:")
			actualExtract := extract
			if isFlat {
				actualExtract = strings.TrimPrefix(extract, "flat:")
			}

			segments = append(segments, PathSegment{
				Type:    ExtractSegment,
				Key:     actualExtract,
				Extract: actualExtract,
				IsFlat:  isFlat,
				Value:   lastExtractPart,
			})
		}
	}

	// Parse array operation segment and mark as distributed
	if strings.HasPrefix(arrayPart, "[") {
		// Find the end of the array operation
		bracketEnd := strings.Index(arrayPart, "]")
		if bracketEnd == -1 {
			return nil, fmt.Errorf("missing closing bracket in '%s'", arrayPart)
		}

		arrayOpPart := arrayPart[:bracketEnd+1]
		arrayContent := arrayOpPart[1 : len(arrayOpPart)-1]

		segment, err := pp.parseArrayAccess(arrayContent)
		if err != nil {
			return nil, fmt.Errorf("invalid array access '%s': %w", arrayContent, err)
		}

		// Mark as distributed operation
		segment.IsDistributed = true
		segment.Value = arrayOpPart

		segments = append(segments, segment)

		// Parse remaining parts after the array operation
		remaining := arrayPart[bracketEnd+1:]
		if remaining != "" {
			remainingSegments, err := pp.parseComplexSegmentRecursive(remaining)
			if err != nil {
				return nil, fmt.Errorf("failed to parse remaining part '%s': %w", remaining, err)
			}
			segments = append(segments, remainingSegments...)
		}
	}

	return segments, nil
}

// parseComplexSegmentRecursive parses complex segments recursively without distributed operation detection
func (pp *PathParser) parseComplexSegmentRecursive(part string) ([]PathSegment, error) {
	var segments []PathSegment
	remaining := part

	for len(remaining) > 0 {
		// Check for extraction syntax
		if strings.HasPrefix(remaining, "{") {
			braceEnd := strings.Index(remaining, "}")
			if braceEnd == -1 {
				return nil, fmt.Errorf("missing closing brace in '%s'", remaining)
			}

			extractPart := remaining[:braceEnd+1]
			extract := extractPart[1 : len(extractPart)-1]

			// Check if this is a flat extraction
			isFlat := strings.HasPrefix(extract, "flat:")
			actualExtract := extract
			if isFlat {
				actualExtract = strings.TrimPrefix(extract, "flat:")
			}

			segments = append(segments, PathSegment{
				Type:    ExtractSegment,
				Key:     actualExtract,
				Extract: actualExtract,
				IsFlat:  isFlat,
				Value:   extractPart,
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
			segment, err := pp.parseArrayAccess(arrayPart)
			if err != nil {
				return nil, fmt.Errorf("invalid array access '%s': %w", arrayPart, err)
			}

			// Check if this array operation follows an extraction - if so, mark as distributed
			if len(segments) > 0 && segments[len(segments)-1].Type == ExtractSegment {
				segment.IsDistributed = true
			}

			// Set Value field for backward compatibility
			segment.Value = remaining[:bracketEnd+1]
			segments = append(segments, segment)
			remaining = remaining[bracketEnd+1:]
			continue
		}

		// Check for property name before any special syntax
		nextSpecial := len(remaining)
		if bracketPos := strings.Index(remaining, "["); bracketPos != -1 && bracketPos < nextSpecial {
			nextSpecial = bracketPos
		}
		if bracePos := strings.Index(remaining, "{"); bracePos != -1 && bracePos < nextSpecial {
			nextSpecial = bracePos
		}

		if nextSpecial > 0 {
			// There's a property name before special syntax
			propertyName := remaining[:nextSpecial]
			segments = append(segments, PathSegment{
				Type:  PropertySegment,
				Key:   propertyName,
				Value: propertyName,
			})
			remaining = remaining[nextSpecial:]
			continue
		}

		// If we get here, there's an error in parsing
		return nil, fmt.Errorf("unable to parse remaining part: '%s'", remaining)
	}

	return segments, nil
}

// parseComplexSegment parses complex segments that may contain mixed syntax like {field}[slice]
func (pp *PathParser) parseComplexSegment(part string) ([]PathSegment, error) {
	// Check if this is extraction followed by array operation (distributed operation)
	if pp.isExtractionFollowedByArrayOp(part) {
		return pp.parseDistributedArrayOperation(part)
	}

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
				Type:    ExtractSegment,
				Extract: actualExtract,
				Key:     actualExtract,
				IsFlat:  isFlat,
				Value:   remaining[:braceEnd+1], // Set Value for backward compatibility
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

			segment, err := pp.parseArrayAccess(arrayPart)
			if err != nil {
				return nil, fmt.Errorf("invalid array access '%s': %w", arrayPart, err)
			}

			// Set Value field for backward compatibility
			segment.Value = remaining[:bracketEnd+1]
			// Legacy fields are no longer needed - they were cleaned up

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
func (pp *PathParser) parseArrayAccess(arrayPart string) (PathSegment, error) {
	// Check for slice notation
	if strings.Contains(arrayPart, ":") {
		return pp.parseSliceAccess(arrayPart)
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
		Value:      fmt.Sprintf("[%d]", index), // Set Value for backward compatibility
	}, nil
}

// parseSliceAccess parses slice notation like "1:3", "::2", "::-1"
func (pp *PathParser) parseSliceAccess(slicePart string) (PathSegment, error) {
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

	// Set Value field for backward compatibility
	if segment.Type == ArrayIndexSegment {
		segment.Value = fmt.Sprintf("[%d]", segment.Index)
	} else if segment.Type == ArraySliceSegment {
		startStr := ""
		endStr := ""
		stepStr := ""
		if segment.Start != nil {
			startStr = fmt.Sprintf("%d", *segment.Start)
		}
		if segment.End != nil {
			endStr = fmt.Sprintf("%d", *segment.End)
		}
		if segment.Step != nil && *segment.Step > 1 {
			stepStr = fmt.Sprintf(":%d", *segment.Step)
		}
		segment.Value = fmt.Sprintf("[%s:%s%s]", startStr, endStr, stepStr)
	}

	return segment, nil
}

// parseJSONPointer parses JSON Pointer format paths like "/users/0/name"
func (pp *PathParser) parseJSONPointer(path string) ([]PathSegment, error) {
	if path == "/" {
		return []PathSegment{{Type: PropertySegment, Key: ""}}, nil
	}

	// Remove leading slash and split
	path = strings.TrimPrefix(path, "/")
	parts := strings.Split(path, "/")

	var segments []PathSegment
	for _, part := range parts {
		// Unescape JSON Pointer special characters
		part = strings.ReplaceAll(part, "~1", "/")
		part = strings.ReplaceAll(part, "~0", "~")

		// Optimized: Check if it's a numeric index without regex
		if isNumericIndex(part) {
			index, err := strconv.Atoi(part)
			if err != nil {
				return nil, fmt.Errorf("invalid numeric index '%s': %w", part, err)
			}
			segments = append(segments, PathSegment{
				Type:       ArrayIndexSegment,
				Index:      index,
				IsNegative: index < 0,
			})
		} else {
			// Property access
			segments = append(segments, PathSegment{
				Type: PropertySegment,
				Key:  part,
			})
		}
	}

	return segments, nil
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

// ValidatePath validates a path string for security and correctness
func (pp *PathParser) ValidatePath(path string) error {
	if len(path) > 1000 {
		return fmt.Errorf("path too long: %d characters (max 1000)", len(path))
	}

	// Check for null bytes
	if strings.Contains(path, "\x00") {
		return fmt.Errorf("path contains null bytes")
	}

	// Check for suspicious patterns
	suspiciousPatterns := []string{
		"../", "./", "//", "\\", "\\\\",
		"<script", "javascript:", "data:", "vbscript:",
		"file://", "ftp://", "http://", "https://",
		"eval(", "exec(", "system(", "cmd(",
		"${", "#{", "%{", "{{",
		"\r", "\n", "\t",
	}

	lowerPath := strings.ToLower(path)
	for _, pattern := range suspiciousPatterns {
		if strings.Contains(lowerPath, pattern) {
			return fmt.Errorf("suspicious pattern detected: %s", pattern)
		}
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
