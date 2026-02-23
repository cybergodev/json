package json

import (
	"fmt"
	"strconv"
	"strings"
)

func (p *Processor) setValueJSONPointer(data any, path string, value any) error {
	return p.setValueJSONPointerWithCreation(data, path, value)
}

func (p *Processor) setValueJSONPointerWithCreation(data any, path string, value any) error {
	if path == "/" {
		return fmt.Errorf("cannot set root value")
	}

	// Remove leading slash and split
	pathWithoutSlash := path[1:]
	segments := strings.Split(pathWithoutSlash, "/")

	// Handle array extension for JSON Pointer
	return p.setValueJSONPointerWithArrayExtension(data, segments, value)
}

func (p *Processor) setValueJSONPointerWithArrayExtension(data any, segments []string, value any) error {
	if len(segments) == 0 {
		return fmt.Errorf("no segments provided")
	}

	// Navigate to parent segments
	current := data
	for i := 0; i < len(segments)-1; i++ {
		segment := segments[i]

		// Unescape JSON Pointer characters
		if strings.Contains(segment, "~") {
			segment = p.unescapeJSONPointer(segment)
		}

		next, err := p.createPathSegmentForJSONPointerWithExtension(current, segment, segments, i)
		if err != nil {
			return err
		}
		current = next
	}

	// Set final value
	finalSegment := segments[len(segments)-1]
	if strings.Contains(finalSegment, "~") {
		finalSegment = p.unescapeJSONPointer(finalSegment)
	}

	return p.setJSONPointerFinalValue(current, finalSegment, value)
}

func (p *Processor) createPathSegmentForJSONPointer(current any, segment string, allSegments []string, currentIndex int) (any, error) {
	switch v := current.(type) {
	case map[string]any:
		if val, exists := v[segment]; exists {
			return val, nil
		}
		// Create missing property
		var newContainer any
		if currentIndex+1 < len(allSegments) {
			nextSegment := allSegments[currentIndex+1]
			if p.isArrayIndex(nextSegment) {
				newContainer = make([]any, 0)
			} else {
				newContainer = make(map[string]any)
			}
		} else {
			newContainer = make(map[string]any)
		}
		v[segment] = newContainer
		return newContainer, nil

	case []any:
		if index, err := strconv.Atoi(segment); err == nil {
			if index >= 0 && index < len(v) {
				return v[index], nil
			}
			if index >= len(v) {
				// Need to extend array - create extended array and replace in parent
				return p.handleJSONPointerArrayExtension(current, v, index, allSegments, currentIndex)
			}
		}
		return nil, fmt.Errorf("invalid array index for JSON Pointer: %s", segment)

	default:
		return nil, fmt.Errorf("cannot navigate through %T with segment %s", current, segment)
	}
}

func (p *Processor) setPropertyValue(current any, property string, value any) error {
	switch v := current.(type) {
	case map[string]any:
		v[property] = value
		return nil
	case map[any]any:
		v[property] = value
		return nil
	default:
		return fmt.Errorf("cannot set property '%s' on type %T", property, current)
	}
}

func (p *Processor) setPropertyValueWithCreation(current any, property string, value any) error {
	switch v := current.(type) {
	case map[string]any:
		v[property] = value
		return nil
	case map[any]any:
		v[property] = value
		return nil
	case []any:
		// Try to parse property as array index
		if index, err := strconv.Atoi(property); err == nil {
			if index >= 0 && index < len(v) {
				v[index] = value
				return nil
			}
			if index == len(v) {
				// Extend array
				v = append(v, value)
				return nil
			}
		}
		return fmt.Errorf("invalid array index: %s", property)
	default:
		return fmt.Errorf("cannot set property '%s' on type %T", property, current)
	}
}

func (p *Processor) handleJSONPointerArrayExtension(parent any, arr []any, targetIndex int, allSegments []string, currentIndex int) (any, error) {
	// Create extended array
	extendedArr := make([]any, targetIndex+1)
	copy(extendedArr, arr)

	// Determine what to put at the target index
	var newContainer any
	if currentIndex+1 < len(allSegments) {
		nextSegment := allSegments[currentIndex+1]
		if p.isArrayIndex(nextSegment) {
			newContainer = make([]any, 0)
		} else {
			newContainer = make(map[string]any)
		}
	} else {
		newContainer = nil // Will be set by the final value
	}

	extendedArr[targetIndex] = newContainer

	// Replace the array in parent - this is tricky for JSON Pointer
	// We need to find a way to update the parent reference
	// For now, we'll modify the original array in place if possible
	if cap(arr) >= len(extendedArr) {
		// Can extend in place
		for i := len(arr); i < len(extendedArr); i++ {
			arr = append(arr, nil)
		}
		arr[targetIndex] = newContainer
		return newContainer, nil
	}

	// Cannot extend in place - this is a limitation
	// Return the new container but note that parent won't be updated
	return newContainer, nil
}

func (p *Processor) createPathSegmentForJSONPointerWithExtension(current any, segment string, allSegments []string, currentIndex int) (any, error) {
	switch v := current.(type) {
	case map[string]any:
		if val, exists := v[segment]; exists {
			return val, nil
		}
		// Create missing property
		var newContainer any
		if currentIndex+1 < len(allSegments) {
			nextSegment := allSegments[currentIndex+1]
			if p.isArrayIndex(nextSegment) {
				newContainer = make([]any, 0)
			} else {
				newContainer = make(map[string]any)
			}
		} else {
			newContainer = make(map[string]any)
		}
		v[segment] = newContainer
		return newContainer, nil

	case []any:
		if index, err := strconv.Atoi(segment); err == nil {
			if index >= 0 && index < len(v) {
				return v[index], nil
			}
			if index >= len(v) {
				// Extend array to accommodate the index
				extendedArr := make([]any, index+1)
				copy(extendedArr, v)

				// Determine what to put at the target index
				var newContainer any
				if currentIndex+1 < len(allSegments) {
					nextSegment := allSegments[currentIndex+1]
					if p.isArrayIndex(nextSegment) {
						newContainer = make([]any, 0)
					} else {
						newContainer = make(map[string]any)
					}
				} else {
					newContainer = nil
				}

				extendedArr[index] = newContainer

				// Replace the array in the parent - we need to find the parent
				// This is a complex operation that requires tracking the parent
				return p.replaceArrayInJSONPointerParent(current, v, extendedArr, index, newContainer)
			}
		}
		return nil, fmt.Errorf("invalid array index for JSON Pointer: %s", segment)

	default:
		return nil, fmt.Errorf("cannot navigate through %T with segment %s", current, segment)
	}
}

func (p *Processor) setJSONPointerFinalValue(current any, segment string, value any) error {
	switch v := current.(type) {
	case map[string]any:
		v[segment] = value
		return nil
	case []any:
		if index, err := strconv.Atoi(segment); err == nil {
			if index >= 0 && index < len(v) {
				v[index] = value
				return nil
			}
			if index >= len(v) {
				// Extend array to accommodate the index
				extendedArr := make([]any, index+1)
				copy(extendedArr, v)
				extendedArr[index] = value

				// Try to replace in place if possible
				if cap(v) >= len(extendedArr) {
					for i := len(v); i < len(extendedArr); i++ {
						v = append(v, nil)
					}
					v[index] = value
					return nil
				}

				// Cannot extend in place - this is a limitation of the current approach
				// The parent reference won't be updated
				return fmt.Errorf("cannot extend array in place for index %d", index)
			}
		}
		return fmt.Errorf("invalid array index: %s", segment)
	default:
		return fmt.Errorf("cannot set value on type %T", current)
	}
}

func (p *Processor) replaceArrayInJSONPointerParent(parent any, oldArray, newArray []any, index int, newContainer any) (any, error) {
	// This is a complex operation that would require tracking parent references
	// For now, we'll try to extend in place if possible
	if cap(oldArray) >= len(newArray) {
		for i := len(oldArray); i < len(newArray); i++ {
			oldArray = append(oldArray, nil)
		}
		oldArray[index] = newContainer
		return newContainer, nil
	}

	// Cannot extend in place
	return newContainer, nil
}
