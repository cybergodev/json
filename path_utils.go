package json

import (
	"strings"

	"github.com/cybergodev/json/internal"
)

func (p *Processor) escapeJSONPointer(segment string) string {
	// Use the centralized JSON pointer escaping helper
	return EscapeJSONPointer(segment)
}

func (p *Processor) normalizePathSeparators(path string) string {
	return internal.NormalizePathSeparators(path)
}

func (p *Processor) splitPathSegments(path string) []string {
	if path == "" {
		return []string{}
	}

	// Handle JSON Pointer format
	if strings.HasPrefix(path, "/") {
		pathWithoutSlash := path[1:]
		if pathWithoutSlash == "" {
			return []string{}
		}
		return strings.Split(pathWithoutSlash, "/")
	}

	// Handle dot notation
	return strings.Split(path, ".")
}

func (p *Processor) joinPathSegments(segments []string, useJSONPointer bool) string {
	if len(segments) == 0 {
		return ""
	}

	if useJSONPointer {
		return "/" + strings.Join(segments, "/")
	}

	return strings.Join(segments, ".")
}

func (p *Processor) isValidPropertyName(name string) bool {
	return internal.IsValidPropertyName(name)
}

func (p *Processor) isValidArrayIndex(index string) bool {
	return internal.IsValidArrayIndex(index)
}

func (p *Processor) isValidSliceRange(rangeStr string) bool {
	return internal.IsValidSliceRange(rangeStr)
}

func (p *Processor) wrapError(err error, context string) error {
	return internal.WrapError(err, context)
}

// createPathError creates a path-specific error
func (p *Processor) createPathError(path string, operation string, err error) error {
	return internal.CreatePathError(path, operation, err)
}
