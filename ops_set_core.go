package json

import (
	"fmt"
	"strings"

	"github.com/cybergodev/json/internal"
)

// setValueAtPath sets a value at the specified path
func (p *Processor) setValueAtPath(data any, path string, value any) error {
	return p.setValueAtPathWithOptions(data, path, value, false)
}

func (p *Processor) setValueAtPathWithOptions(data any, path string, value any, createPaths bool) error {
	if path == "" || path == "." {
		return fmt.Errorf("cannot set root value")
	}

	// Use advanced path parsing for full feature support
	return p.setValueAdvancedPath(data, path, value, createPaths)
}

func (p *Processor) setValueAdvancedPath(data any, path string, value any, createPaths bool) error {
	// Handle JSON Pointer format first
	if strings.HasPrefix(path, "/") {
		if createPaths {
			return p.setValueJSONPointerWithCreation(data, path, value)
		}
		return p.setValueJSONPointer(data, path, value)
	}

	// Check if this is a simple array index access that might need extension
	if createPaths && p.isSimpleArrayIndexPath(path) {
		// Use dot notation handler for simple array index access with extension support
		return p.setValueDotNotationWithCreation(data, path, value, createPaths)
	}

	// Check if this is a complex path that should use RecursiveProcessor
	// But exclude simple array slice paths that need array extension support
	if p.isComplexPath(path) && !p.isSimpleArraySlicePath(path) {
		// Use RecursiveProcessor for complex paths like flat extraction
		unifiedProcessor := NewRecursiveProcessor(p)
		_, err := unifiedProcessor.ProcessRecursivelyWithOptions(data, path, OpSet, value, createPaths)
		return err
	}

	// Use dot notation with segments for simple paths
	return p.setValueDotNotationWithCreation(data, path, value, createPaths)
}

func (p *Processor) isSimpleArraySlicePath(path string) bool {
	// Check for simple patterns like "property[start:end]" or "property.subprop[start:end]"
	// These should use legacy handling for array extension support

	// Must contain slice syntax
	if !strings.Contains(path, ":") {
		return false
	}

	// Must not contain extraction syntax (which needs RecursiveProcessor)
	if strings.Contains(path, "{") || strings.Contains(path, "}") {
		return false
	}

	// Check if it's a simple property.array[slice] pattern
	// Count the number of bracket pairs
	openBrackets := strings.Count(path, "[")
	closeBrackets := strings.Count(path, "]")

	// Should have exactly one bracket pair for simple slice
	if openBrackets != 1 || closeBrackets != 1 {
		return false
	}

	// Find the bracket positions
	bracketStart := strings.Index(path, "[")
	bracketEnd := strings.Index(path, "]")

	if bracketStart == -1 || bracketEnd == -1 || bracketEnd <= bracketStart {
		return false
	}

	// Extract the slice part
	slicePart := path[bracketStart+1 : bracketEnd]

	// Check if it's a valid slice syntax (contains colon)
	if !strings.Contains(slicePart, ":") {
		return false
	}

	// Check if the part before brackets is a simple property path (no complex operations)
	beforeBrackets := path[:bracketStart]
	if strings.Contains(beforeBrackets, "{") || strings.Contains(beforeBrackets, "}") {
		return false
	}

	return true
}

func (p *Processor) isSimpleArrayIndexPath(path string) bool {
	// Must contain array index syntax
	if !strings.Contains(path, "[") || !strings.Contains(path, "]") {
		return false
	}

	// Must not contain slice syntax (colons)
	if strings.Contains(path, ":") {
		return false
	}

	// Must not contain extraction syntax
	if strings.Contains(path, "{") || strings.Contains(path, "}") {
		return false
	}

	// Check if it's a simple pattern like "property[index]" or "property.subprop[index]"
	// Count the number of bracket pairs
	openBrackets := strings.Count(path, "[")
	closeBrackets := strings.Count(path, "]")

	// Should have exactly one bracket pair for simple index access
	if openBrackets != 1 || closeBrackets != 1 {
		return false
	}

	// Find the bracket positions
	bracketStart := strings.Index(path, "[")
	bracketEnd := strings.Index(path, "]")

	if bracketStart == -1 || bracketEnd == -1 || bracketEnd <= bracketStart {
		return false
	}

	return true
}

func (p *Processor) setValueWithSegments(data any, segments []PathSegment, value any, createPaths bool) error {
	if len(segments) == 0 {
		return fmt.Errorf("no segments provided")
	}

	// Navigate to the parent of the target
	current := data
	for i := 0; i < len(segments)-1; i++ {
		next, err := p.navigateToSegment(current, segments[i], createPaths, segments, i)
		if err != nil {
			return err
		}
		current = next
	}

	// Set the value for the final segment
	finalSegment := segments[len(segments)-1]

	// Special handling for array index or slice access that might need extension
	if createPaths && (finalSegment.TypeString() == "array" || finalSegment.TypeString() == "slice") {
		return p.setValueForArrayIndexWithExtension(current, finalSegment, value, data, segments)
	}

	err := p.setValueForSegment(current, finalSegment, value, createPaths)

	// Handle array extension error
	if arrayExtErr, ok := err.(*ArrayExtensionNeededError); ok && createPaths {
		// We need to extend the array and then set the values
		return p.handleArrayExtensionAndSet(data, segments, arrayExtErr)
	}

	return err
}

func (p *Processor) setValueDotNotation(data any, path string, value any) error {
	return p.setValueDotNotationWithCreation(data, path, value, false)
}

func (p *Processor) setValueDotNotationWithCreation(data any, path string, value any, createPaths bool) error {
	// Parse path into segments
	segments := p.getPathSegments()
	defer p.putPathSegments(segments)

	segments = p.splitPath(path, segments)

	return p.setValueWithSegments(data, segments, value, createPaths)
}

func (p *Processor) isComplexPath(path string) bool {
	return internal.IsComplexPath(path)
}

func (p *Processor) hasComplexSegments(segments []PathSegment) bool {
	for _, seg := range segments {
		if seg.TypeString() == "extract" || seg.TypeString() == "slice" {
			return true
		}
	}
	return false
}

func (p *Processor) setValueForSegment(current any, segment PathSegment, value any, createPaths bool) error {
	switch segment.TypeString() {
	case "property":
		return p.setValueForProperty(current, segment.Key, value, createPaths)
	case "array":
		index := segment.Index
		return p.setValueForArrayIndex(current, index, value, createPaths)
	case "slice":
		return p.setValueForArraySlice(current, segment, value, createPaths)
	case "extract":
		return p.setValueForExtract(current, segment, value, createPaths)
	default:
		return fmt.Errorf("unsupported segment type for set: %v", segment.TypeString())
	}
}

func (p *Processor) setValueForProperty(current any, property string, value any, createPaths bool) error {
	switch v := current.(type) {
	case map[string]any:
		v[property] = value
		return nil
	case map[any]any:
		v[property] = value
		return nil
	default:
		if createPaths {
			// Cannot convert non-map types to map for property setting
			// This is a fundamental limitation
			return fmt.Errorf("cannot convert %T to map for property setting", current)
		}
		return fmt.Errorf("cannot set property '%s' on type %T", property, current)
	}
}
