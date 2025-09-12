package json

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/cybergodev/json/internal"
)

// navigateToPath navigates to a specific path
func (p *Processor) navigateToPath(data any, path string) (any, error) {
	// Fast path for root access
	if path == "" || path == "." || path == "/" {
		return data, nil
	}

	// Determine path format and use optimized navigation
	if strings.HasPrefix(path, "/") {
		return p.navigateJSONPointer(data, path)
	}

	// Use dot notation navigation
	return p.navigateDotNotation(data, path)
}

// navigateDotNotation handles dot notation paths
func (p *Processor) navigateDotNotation(data any, path string) (any, error) {
	current := data

	// Use path segment pool for memory efficiency
	segments := p.getPathSegments()
	defer p.putPathSegments(segments)

	// Parse path into segments
	segments = p.splitPath(path, segments)

	// Check for deep extraction patterns and preprocess if needed
	if p.isDeepExtractionPath(segments) {
		segments = p.preprocessPathForDeepExtraction(segments)
	}

	// Navigate through each segment
	for i, segment := range segments {
		// Check for distributed operations
		if p.isDistributedOperationSegment(segment) {
			// Handle distributed operations
			return p.handleDistributedOperation(current, segments[i:])
		}

		// Handle different segment types
		switch segment.TypeString() {
		case "property":
			result := p.handlePropertyAccess(current, segment.Value)
			if !result.Exists {
				return nil, ErrPathNotFound
			}
			current = result.Value

		case "array":
			result := p.handleArrayAccess(current, segment)
			if !result.Exists {
				return nil, ErrPathNotFound
			}
			current = result.Value

		case "slice":
			result := p.handleArraySlice(current, segment)
			if !result.Exists {
				return nil, ErrPathNotFound
			}
			current = result.Value

		case "extract":
			// Handle extraction operations
			extractResult, err := p.handleExtraction(current, segment)
			if err != nil {
				return nil, err
			}
			current = extractResult

			// Check if this is followed by array operations
			if i+1 < len(segments) {
				nextSegment := segments[i+1]
				if nextSegment.TypeString() == "array" || nextSegment.TypeString() == "slice" {
					// Check if the current extraction was flat
					// If it was flat, the result is a flattened array, not an array of arrays
					if segment.IsFlat {
						// For flat extraction results, apply array operations directly
						if nextSegment.TypeString() == "slice" {
							result := p.handleArraySlice(current, nextSegment)
							if result.Exists {
								current = result.Value
							}
						} else {
							result := p.handleArrayAccess(current, nextSegment)
							if result.Exists {
								current = result.Value
							}
						}
					} else {
						// For non-flat extraction, use post-extraction handling
						current = p.handlePostExtractionArrayAccess(current, nextSegment)
					}
					i++ // Skip the next segment as we've processed it
				}
			}

		default:
			return nil, fmt.Errorf("unsupported segment type: %v", segment.TypeString())
		}
	}

	return current, nil
}

// navigateJSONPointer handles JSON Pointer format paths
func (p *Processor) navigateJSONPointer(data any, path string) (any, error) {
	if path == "/" {
		return data, nil
	}

	// Remove leading slash and split with pre-allocated capacity
	pathWithoutSlash := path[1:]
	segmentCount := strings.Count(pathWithoutSlash, "/") + 1
	segments := make([]string, 0, segmentCount)
	segments = strings.Split(pathWithoutSlash, "/")

	current := data

	for _, segment := range segments {
		if segment == "" {
			continue
		}

		// Optimized unescape for JSON Pointer special characters
		if strings.Contains(segment, "~") {
			segment = p.unescapeJSONPointer(segment)
		}

		result := p.handlePropertyAccess(current, segment)
		if !result.Exists {
			return nil, ErrPathNotFound
		}
		current = result.Value
	}

	return current, nil
}

// unescapeJSONPointer unescapes JSON Pointer special characters
func (p *Processor) unescapeJSONPointer(segment string) string {
	// JSON Pointer escaping: ~1 -> /, ~0 -> ~
	segment = strings.ReplaceAll(segment, "~1", "/")
	segment = strings.ReplaceAll(segment, "~0", "~")
	return segment
}

// handlePropertyAccess handles property access with optimized type checking
func (p *Processor) handlePropertyAccess(data any, property string) PropertyAccessResult {
	switch v := data.(type) {
	case map[string]any:
		if val, exists := v[property]; exists {
			return PropertyAccessResult{Value: val, Exists: true}
		}
		return PropertyAccessResult{Exists: false}

	case map[any]any:
		if val, exists := v[property]; exists {
			return PropertyAccessResult{Value: val, Exists: true}
		}
		return PropertyAccessResult{Exists: false}

	case []any:
		// Try to parse property as array index
		if index := p.parseArrayIndex(property); index >= 0 && index < len(v) {
			return PropertyAccessResult{Value: v[index], Exists: true}
		}
		return PropertyAccessResult{Exists: false}

	default:
		// Try struct field access
		if structValue := p.handleStructAccess(data, property); structValue != nil {
			return PropertyAccessResult{Value: structValue, Exists: true}
		}
		return PropertyAccessResult{Exists: false}
	}
}

// handlePropertyAccessValue returns the value directly (for backward compatibility)
func (p *Processor) handlePropertyAccessValue(data any, property string) any {
	result := p.handlePropertyAccess(data, property)
	if result.Exists {
		return result.Value
	}
	return nil
}

// parseArrayIndex parses a string as an array index, returns -1 if invalid
func (p *Processor) parseArrayIndex(property string) int {
	if index, err := strconv.Atoi(property); err == nil && index >= 0 {
		return index
	}
	return -1
}

// splitPath splits a path into segments with memory pooling
func (p *Processor) splitPath(path string, segments []PathSegment) []PathSegment {
	// Clear existing segments
	segments = segments[:0]

	// Preprocess path to handle special cases
	sb := p.getStringBuilder()
	defer p.putStringBuilder(sb)

	processedPath := p.preprocessPath(path, sb)

	// Split into segments
	return p.splitPathIntoSegments(processedPath, segments)
}

// preprocessPath preprocesses the path to handle special syntax
func (p *Processor) preprocessPath(path string, sb *strings.Builder) string {
	sb.Reset()

	runes := []rune(path)
	for i, r := range runes {
		switch r {
		case '[':
			// Add dot before [ if needed
			if i > 0 && p.needsDotBefore(runes[i-1]) {
				sb.WriteRune('.')
			}
			sb.WriteRune(r)
		case '{':
			// Add dot before { if needed
			if i > 0 && p.needsDotBefore(runes[i-1]) {
				sb.WriteRune('.')
			}
			sb.WriteRune(r)
		default:
			sb.WriteRune(r)
		}
	}

	return sb.String()
}

// needsDotBefore checks if a dot is needed before the current character
func (p *Processor) needsDotBefore(prevChar rune) bool {
	return prevChar != '.' && prevChar != '[' && prevChar != '{'
}

// splitPathIntoSegments splits a preprocessed path into segments
func (p *Processor) splitPathIntoSegments(path string, segments []PathSegment) []PathSegment {
	parts := strings.Split(path, ".")

	for _, part := range parts {
		if part == "" {
			continue
		}
		segments = p.parsePathSegment(part, segments)
	}

	return segments
}

// parsePathSegment parses a single path segment
func (p *Processor) parsePathSegment(part string, segments []PathSegment) []PathSegment {
	// Handle different segment types
	if strings.Contains(part, "[") {
		// Array or slice segment
		return p.parseArraySegment(part, segments)
	} else if strings.Contains(part, "{") {
		// Extraction segment
		return p.parseExtractionSegment(part, segments)
	} else {
		// Check if this is a numeric index (dot notation like "0", "1", etc.)
		if index, err := strconv.Atoi(part); err == nil {
			// This is a numeric index, treat as array access
			segments = append(segments, PathSegment{
				Value: part,
				Type:  internal.ArrayIndexSegment,
				Index: index,
			})
			return segments
		}

		// Simple property segment
		segments = append(segments, PathSegment{
			Value: part,
			Type:  internal.PropertySegment,
		})
		return segments
	}
}

// isComplexPath determines if a path requires complex processing
func (p *Processor) isComplexPath(path string) bool {
	// Check for complex patterns
	complexPatterns := []string{
		"{", "}", // Extraction syntax
		"[", "]", // Array access
		":", // Slice syntax
	}

	for _, pattern := range complexPatterns {
		if strings.Contains(path, pattern) {
			return true
		}
	}

	return false
}

// hasComplexSegments checks if segments contain complex operations
func (p *Processor) hasComplexSegments(segments []PathSegment) bool {
	for _, segment := range segments {
		switch segment.TypeString() {
		case "slice", "extract":
			return true
		}
	}
	return false
}

// parsePath parses a path string into segments (legacy method)
func (p *Processor) parsePath(path string) ([]string, error) {
	if path == "" {
		return []string{}, nil
	}

	// Simple split for basic paths
	if !p.isComplexPath(path) {
		return strings.Split(path, "."), nil
	}

	// For complex paths, use the full parser
	segments := p.getPathSegments()
	defer p.putPathSegments(segments)

	segments = p.splitPath(path, segments)

	// Convert to string array
	result := make([]string, len(segments))
	for i, segment := range segments {
		result[i] = segment.Value
	}

	return result, nil
}

// isDistributedOperationPath checks if a path requires distributed operation handling
func (p *Processor) isDistributedOperationPath(path string) bool {
	// Check for patterns that indicate distributed operations
	distributedPatterns := []string{
		"}[", // Extraction followed by array access
		"}:", // Extraction followed by slice
		"}{", // Consecutive extractions
	}

	for _, pattern := range distributedPatterns {
		if strings.Contains(path, pattern) {
			return true
		}
	}

	// Check for flat extraction patterns
	if strings.Contains(path, "{flat:") {
		return true
	}

	return false
}

// isDistributedOperationSegment checks if a segment requires distributed handling
func (p *Processor) isDistributedOperationSegment(segment PathSegment) bool {
	// Check segment properties for distributed operation indicators
	return segment.IsDistributed || segment.Extract != ""
}

// handleDistributedOperation handles distributed operations
func (p *Processor) handleDistributedOperation(data any, segments []PathSegment) (any, error) {
	// This is a placeholder - the actual implementation would be in extraction_handler.go
	return p.getValueWithDistributedOperation(data, p.reconstructPath(segments))
}

// reconstructPath reconstructs a path string from segments
func (p *Processor) reconstructPath(segments []PathSegment) string {
	if len(segments) == 0 {
		return ""
	}

	sb := p.getStringBuilder()
	defer p.putStringBuilder(sb)

	for i, segment := range segments {
		if i > 0 {
			sb.WriteRune('.')
		}
		sb.WriteString(segment.Value)
	}

	return sb.String()
}

// isDeepExtractionPath checks if segments contain deep extraction patterns

// parseArraySegment parses array access segments like [0], [1:3], etc.
func (p *Processor) parseArraySegment(part string, segments []PathSegment) []PathSegment {
	// Find the bracket positions
	openBracket := strings.Index(part, "[")
	closeBracket := strings.LastIndex(part, "]")

	if openBracket == -1 || closeBracket == -1 || closeBracket <= openBracket {
		// Invalid bracket syntax, treat as property
		segments = append(segments, PathSegment{
			Value: part,
			Type:  internal.PropertySegment,
		})
		return segments
	}

	// Extract property name before bracket (if any)
	if openBracket > 0 {
		propertyName := part[:openBracket]
		segments = append(segments, PathSegment{
			Value: propertyName,
			Type:  internal.PropertySegment,
		})
	}

	// Extract bracket content
	bracketContent := part[openBracket+1 : closeBracket]

	// Determine if this is a slice or array index
	if strings.Contains(bracketContent, ":") {
		// Slice syntax - parse slice parameters
		segment := PathSegment{
			Value: bracketContent,
			Type:  internal.ArraySliceSegment,
		}

		// Parse slice parameters
		parts := strings.Split(bracketContent, ":")
		if len(parts) >= 2 {
			// Parse start
			if parts[0] != "" {
				if start, err := strconv.Atoi(parts[0]); err == nil {
					segment.Start = &start
				}
			}

			// Parse end
			if parts[1] != "" {
				if end, err := strconv.Atoi(parts[1]); err == nil {
					segment.End = &end
				}
			}

			// Parse step (if provided)
			if len(parts) == 3 && parts[2] != "" {
				if step, err := strconv.Atoi(parts[2]); err == nil {
					segment.Step = &step
				}
			}
		}

		segments = append(segments, segment)
	} else {
		// Array index
		segment := PathSegment{
			Value: bracketContent,
			Type:  internal.ArrayIndexSegment,
		}

		// Parse index
		if index, err := strconv.Atoi(bracketContent); err == nil {
			segment.Index = index
		}

		segments = append(segments, segment)
	}

	// Handle any remaining part after the bracket
	if closeBracket+1 < len(part) {
		remaining := part[closeBracket+1:]
		if remaining != "" {
			segments = p.parsePathSegment(remaining, segments)
		}
	}

	return segments
}

// parseExtractionSegment parses extraction segments like {key}, {flat:key}, etc.
func (p *Processor) parseExtractionSegment(part string, segments []PathSegment) []PathSegment {
	// Find the brace positions
	openBrace := strings.Index(part, "{")
	closeBrace := strings.LastIndex(part, "}")

	if openBrace == -1 || closeBrace == -1 || closeBrace <= openBrace {
		// Invalid brace syntax, treat as property
		segments = append(segments, PathSegment{
			Value: part,
			Type:  internal.PropertySegment,
		})
		return segments
	}

	// Extract property name before brace (if any)
	if openBrace > 0 {
		propertyName := part[:openBrace]
		segments = append(segments, PathSegment{
			Value: propertyName,
			Type:  internal.PropertySegment,
		})
	}

	// Extract brace content
	braceContent := part[openBrace+1 : closeBrace]

	// Create extraction segment
	extractSegment := PathSegment{
		Value: braceContent,
		Type:  internal.ExtractSegment,
	}

	// Check for flat extraction
	if strings.HasPrefix(braceContent, "flat:") {
		extractSegment.Extract = braceContent[5:] // Remove "flat:" prefix
		extractSegment.IsFlat = true
	} else {
		extractSegment.Extract = braceContent
	}

	segments = append(segments, extractSegment)

	// Handle any remaining part after the brace
	if closeBrace+1 < len(part) {
		remaining := part[closeBrace+1:]
		if remaining != "" {
			segments = p.parsePathSegment(remaining, segments)
		}
	}

	return segments
}
