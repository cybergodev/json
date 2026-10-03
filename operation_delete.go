package json

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/cybergodev/json/internal"
)

func (p *Processor) deleteValueAtPath(data any, path string) error {
	// D-002 (M33): custom path syntax routes through the recursive engine
	// exclusively — the dot-notation walker below assumes standard
	// '.'-separated keys and would mis-split custom syntax. The existence
	// pre-check preserves the "delete missing path → error" contract for
	// precise custom paths (batch-style custom paths tolerate misses, same
	// as built-in wildcard/extract paths).
	if p.config.CustomPathParser != nil {
		if _, err := p.recursiveProcessor.ProcessRecursively(data, path, opGet, nil); err != nil {
			return err
		}
		_, err := p.recursiveProcessor.ProcessRecursively(data, path, opDelete, nil)
		return err
	}

	// Handle JSON Pointer format
	if strings.HasPrefix(path, "/") {
		return p.deleteValueJSONPointer(data, path)
	}

	// Complex paths go through the unified recursive processor — the same engine
	// used by Get/Set — which fully supports wildcard, slice, extract, and index
	// segments. This replaces the legacy operation_delete.go complex-path handling
	// that lacked wildcard support and mis-parsed bracketed indices.
	if p.isComplexPath(path) {
		// Precise complex paths (property/index only, no wildcard/extract/slice)
		// must keep the "delete non-existent path → error" public contract.
		// Pre-check existence via Get. Batch paths (containing * / { / :) carry
		// tolerant semantics where missing targets are skipped, so they skip the
		// pre-check; out-of-bounds indices are still caught by opDelete itself.
		if p.isPreciseComplexPath(path) {
			if _, err := p.recursiveProcessor.ProcessRecursively(data, path, opGet, nil); err != nil {
				return err
			}
		}
		_, err := p.recursiveProcessor.ProcessRecursively(data, path, opDelete, nil)
		return err
	}

	// Use dot notation for simple paths
	return p.deleteValueDotNotation(data, path)
}

// isPreciseComplexPath reports whether a complex path targets a precise location
// (property/index only) rather than a batch/container operation. Paths containing
// wildcard (*), extraction ({}), or slice (:) carry batch semantics where missing
// targets are tolerated; precise paths require an existence pre-check to preserve
// the "delete missing path → error" public contract.
func (p *Processor) isPreciseComplexPath(path string) bool {
	for i := 0; i < len(path); i++ {
		switch path[i] {
		case '*', '{', '}', ':':
			return false
		}
	}
	return true
}

// navigateToParent navigates through data to the parent container of the final segment.
// Returns the parent container and the final segment key/index.
// Shared by deleteValueDotNotation and deleteValueJSONPointer to avoid duplicating
// the map[string]any/map[any]any/[]any switch logic.
func (p *Processor) navigateToParent(data any, segments []string) (any, string, error) {
	if len(segments) == 0 {
		return nil, "", fmt.Errorf("empty path")
	}

	current := data
	for i := 0; i < len(segments)-1; i++ {
		segment := segments[i]

		switch v := current.(type) {
		case map[string]any:
			if next, exists := v[segment]; exists {
				current = next
			} else {
				return nil, "", fmt.Errorf("path not found: %s: %w", segment, ErrPathNotFound)
			}
		case map[any]any:
			if next, exists := v[segment]; exists {
				current = next
			} else {
				return nil, "", fmt.Errorf("path not found: %s: %w", segment, ErrPathNotFound)
			}
		case []any:
			if index, ok := internal.ParseAndValidateArrayIndex(segment, len(v)); ok {
				current = v[index]
			} else {
				return nil, "", fmt.Errorf("invalid array index: %s", segment)
			}
		default:
			return nil, "", fmt.Errorf("cannot navigate through %T at segment %s", current, segment)
		}
	}

	return current, segments[len(segments)-1], nil
}

func (p *Processor) deleteValueDotNotation(data any, path string) error {
	segments, err := p.parsePath(path)
	if err != nil {
		return err
	}

	parent, finalSegment, err := p.navigateToParent(data, segments)
	if err != nil {
		return err
	}
	return p.deletePropertyValue(parent, finalSegment)
}

func (p *Processor) deleteValueJSONPointer(data any, path string) error {
	if path == "/" {
		return fmt.Errorf("cannot delete root")
	}

	// Split and unescape JSON Pointer segments upfront
	segments := strings.Split(path[1:], "/")
	for i := range segments {
		if strings.Contains(segments[i], "~") {
			segments[i] = internal.UnescapeJSONPointer(segments[i])
		}
	}

	parent, finalSegment, err := p.navigateToParent(data, segments)
	if err != nil {
		return err
	}
	return p.deletePropertyValue(parent, finalSegment)
}

func (p *Processor) deletePropertyValue(current any, property string) error {
	switch current.(type) {
	case []any:
		if _, err := strconv.Atoi(property); err == nil {
			return p.deleteArrayElement(current, property)
		}
		return fmt.Errorf("invalid array index: %s", property)
	default:
		return p.deletePropertyFromContainer(current, property)
	}
}

func (p *Processor) deletePropertyFromContainer(current any, property string) error {
	if containerDeleteProperty(current, property) {
		return nil
	}
	if containerIsMap(current) {
		return fmt.Errorf("property not found: %s: %w", property, ErrPathNotFound)
	}
	return fmt.Errorf("cannot delete property '%s' from type %T", property, current)
}

func (p *Processor) deleteArrayElement(current any, indexStr string) error {
	// PERFORMANCE: Use fastParseInt instead of strconv.Atoi
	index, ok := internal.ParseArrayIndex(indexStr)
	if !ok {
		return fmt.Errorf("invalid array index: %s", indexStr)
	}
	return p.deleteArrayElementByIndex(current, index)
}

func (p *Processor) deleteArrayElementByIndex(current any, index int) error {
	arr, ok := current.([]any)
	if !ok {
		return fmt.Errorf("cannot delete array element from type %T", current)
	}

	index, err := normalizeNegativeIndex(index, len(arr))
	if err != nil {
		return err
	}

	// Mark element for deletion (set to special marker)
	arr[index] = deletedMarker
	return nil
}
