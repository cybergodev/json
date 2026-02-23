package json

import (
	"fmt"
)

// Navigation methods for path traversal

func (p *Processor) navigateToSegment(current any, segment PathSegment, createPaths bool, allSegments []PathSegment, currentIndex int) (any, error) {
	switch segment.TypeString() {
	case "property":
		return p.navigateToProperty(current, segment.Key, createPaths, allSegments, currentIndex)
	case "array":
		// Get array index from segment
		index := segment.Index
		return p.navigateToArrayIndexWithNegative(current, index, createPaths)
	case "slice":
		// Check if this is the last segment before an extract operation
		if currentIndex+1 < len(allSegments) && allSegments[currentIndex+1].TypeString() == "extract" {
			// This is a slice followed by extract - return the current array for slice processing
			return current, nil
		}
		// For other cases, array slices are not supported as intermediate paths
		return nil, fmt.Errorf("array slice not supported as intermediate path segment")
	case "extract":
		// Handle extract operations as intermediate path segments
		return p.navigateToExtraction(current, segment, createPaths, allSegments, currentIndex)
	default:
		return nil, fmt.Errorf("unsupported segment type: %v", segment.TypeString())
	}
}

func (p *Processor) navigateToExtraction(current any, segment PathSegment, createPaths bool, allSegments []PathSegment, currentIndex int) (any, error) {
	field := segment.Key
	if field == "" {
		return nil, fmt.Errorf("invalid extraction syntax: %s", segment.String())
	}

	// For set operations on extractions, we need to handle this differently
	// This is a complex case that might require distributed operations
	if _, ok := current.([]any); ok {
		// For arrays, we need to set values in each extracted field
		// This is handled by distributed operations
		return current, nil
	}

	// For single objects, extract the field
	if obj, ok := current.(map[string]any); ok {
		if value := p.handlePropertyAccessValue(obj, field); value != nil {
			return value, nil
		}
		if createPaths {
			// Create the field if it doesn't exist
			newContainer, err := p.createContainerForNextSegment(allSegments, currentIndex)
			if err != nil {
				return nil, err
			}
			obj[field] = newContainer
			return newContainer, nil
		}
	}

	return nil, fmt.Errorf("extraction field '%s' not found", field)
}

func (p *Processor) navigateToProperty(current any, property string, createPaths bool, allSegments []PathSegment, currentIndex int) (any, error) {
	switch v := current.(type) {
	case map[string]any:
		if val, exists := v[property]; exists {
			return val, nil
		}
		if createPaths {
			// Create missing property
			newContainer, err := p.createContainerForNextSegment(allSegments, currentIndex)
			if err != nil {
				return nil, err
			}
			v[property] = newContainer
			return newContainer, nil
		}
		return nil, fmt.Errorf("property '%s' not found", property)
	case map[any]any:
		if val, exists := v[property]; exists {
			return val, nil
		}
		if createPaths {
			newContainer, err := p.createContainerForNextSegment(allSegments, currentIndex)
			if err != nil {
				return nil, err
			}
			v[property] = newContainer
			return newContainer, nil
		}
		return nil, fmt.Errorf("property '%s' not found", property)
	default:
		return nil, fmt.Errorf("cannot access property '%s' on type %T", property, current)
	}
}

func (p *Processor) createContainerForNextSegment(allSegments []PathSegment, currentIndex int) (any, error) {
	if currentIndex+1 >= len(allSegments) {
		// This is the last segment, return nil (will be replaced by the actual value)
		return nil, nil
	}

	nextSegment := allSegments[currentIndex+1]
	switch nextSegment.TypeString() {
	case "property", "extract":
		return make(map[string]any), nil
	case "array":
		// For array access, create an empty array that can be extended
		return make([]any, 0), nil
	case "slice":
		// For slice access, we need to create an array large enough for the slice
		end := 0
		if nextSegment.End != nil {
			end = *nextSegment.End
		}
		if end > 0 {
			return make([]any, end), nil
		}
		return make([]any, 0), nil
	default:
		return make(map[string]any), nil // Default to object
	}
}
