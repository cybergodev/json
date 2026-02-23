package json

import (
	"fmt"
	"strings"

	"github.com/cybergodev/json/internal"
)

func (p *Processor) navigateToPath(data any, path string) (any, error) {
	if path == "" || path == "." || path == "/" {
		return data, nil
	}

	if strings.HasPrefix(path, "/") {
		return p.navigateJSONPointer(data, path)
	}

	return p.navigateDotNotation(data, path)
}

func (p *Processor) navigateDotNotation(data any, path string) (any, error) {
	current := data

	segments := p.getPathSegments()
	defer p.putPathSegments(segments)

	segments = p.splitPath(path, segments)

	for i, segment := range segments {
		if p.isDistributedOperationSegment(segment) {
			return p.handleDistributedOperation(current, segments[i:])
		}

		switch segment.TypeString() {
		case "property":
			result := p.handlePropertyAccess(current, segment.Key)
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
			extractResult, err := p.handleExtraction(current, segment)
			if err != nil {
				return nil, err
			}
			current = extractResult

			if i+1 < len(segments) {
				nextSegment := segments[i+1]
				if nextSegment.TypeString() == "array" || nextSegment.TypeString() == "slice" {
					if segment.IsFlat {
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
						current = p.handlePostExtractionArrayAccess(current, nextSegment)
					}
					i++
				}
			}

		default:
			return nil, fmt.Errorf("unsupported segment type: %v", segment.TypeString())
		}
	}

	return current, nil
}

func (p *Processor) navigateJSONPointer(data any, path string) (any, error) {
	if path == "/" {
		return data, nil
	}

	pathWithoutSlash := path[1:]
	segmentCount := strings.Count(pathWithoutSlash, "/") + 1
	segments := make([]string, 0, segmentCount)
	segments = strings.Split(pathWithoutSlash, "/")

	current := data

	for _, segment := range segments {
		if segment == "" {
			continue
		}

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
	return internal.UnescapeJSONPointer(segment)
}

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
		if index := p.parseArrayIndex(property); index >= 0 && index < len(v) {
			return PropertyAccessResult{Value: v[index], Exists: true}
		}
		return PropertyAccessResult{Exists: false}

	default:
		if structValue := p.handleStructAccess(data, property); structValue != nil {
			return PropertyAccessResult{Value: structValue, Exists: true}
		}
		return PropertyAccessResult{Exists: false}
	}
}

func (p *Processor) handlePropertyAccessValue(data any, property string) any {
	result := p.handlePropertyAccess(data, property)
	if result.Exists {
		return result.Value
	}
	return nil
}

func (p *Processor) parseArrayIndexFromPath(property string) int {
	if index, ok := internal.ParseArrayIndex(property); ok {
		return index
	}
	return -1
}
