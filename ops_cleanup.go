package json

func (p *Processor) cleanupArrayNulls(arr []any) {
	writeIndex := 0
	for readIndex := 0; readIndex < len(arr); readIndex++ {
		if arr[readIndex] != nil {
			if writeIndex != readIndex {
				arr[writeIndex] = arr[readIndex]
			}
			writeIndex++
		}
	}

	// Clear the remaining elements
	for i := writeIndex; i < len(arr); i++ {
		arr[i] = nil
	}

	// Truncate the slice
	arr = arr[:writeIndex]
}

func (p *Processor) cleanupArrayWithReconstruction(arr []any, compactArrays bool) []any {
	if !compactArrays {
		return arr
	}

	result := make([]any, 0, len(arr))
	for _, item := range arr {
		if item != nil {
			// Recursively clean nested structures
			if nestedArr, ok := item.([]any); ok {
				item = p.cleanupArrayWithReconstruction(nestedArr, compactArrays)
			} else if nestedMap, ok := item.(map[string]any); ok {
				item = p.cleanupNullValuesRecursiveWithReconstruction(nestedMap, compactArrays)
			}
			result = append(result, item)
		}
	}

	return result
}

func (p *Processor) cleanupNullValues(data any) {
	p.cleanupNullValuesRecursive(data)
}

func (p *Processor) cleanupNullValuesRecursive(data any) {
	switch v := data.(type) {
	case map[string]any:
		for key, value := range v {
			if value == nil {
				delete(v, key)
			} else {
				p.cleanupNullValuesRecursive(value)
			}
		}
	case []any:
		p.cleanupArrayNulls(v)
		for _, item := range v {
			if item != nil {
				p.cleanupNullValuesRecursive(item)
			}
		}
	}
}

func (p *Processor) isEmptyContainer(data any) bool {
	switch v := data.(type) {
	case map[string]any:
		return len(v) == 0
	case map[any]any:
		return len(v) == 0
	case []any:
		return len(v) == 0
	case string:
		return v == ""
	default:
		return false
	}
}

func (p *Processor) isContainer(data any) bool {
	switch data.(type) {
	case map[string]any, map[any]any, []any:
		return true
	default:
		return false
	}
}

func (p *Processor) cleanupNullValuesWithReconstruction(data any, compactArrays bool) any {
	return p.cleanupNullValuesRecursiveWithReconstruction(data, compactArrays)
}

func (p *Processor) cleanupNullValuesRecursiveWithReconstruction(data any, compactArrays bool) any {
	switch v := data.(type) {
	case map[string]any:
		result := make(map[string]any)
		for key, value := range v {
			if value != nil {
				cleanedValue := p.cleanupNullValuesRecursiveWithReconstruction(value, compactArrays)
				if cleanedValue != nil && !p.isEmptyContainer(cleanedValue) {
					result[key] = cleanedValue
				}
			}
		}
		return result

	case []any:
		if compactArrays {
			return p.cleanupArrayWithReconstruction(v, compactArrays)
		}
		result := make([]any, len(v))
		for i, item := range v {
			if item != nil {
				result[i] = p.cleanupNullValuesRecursiveWithReconstruction(item, compactArrays)
			}
		}
		return result

	default:
		return data
	}
}

func (p *Processor) cleanupDeletedMarkers(data any) any {
	switch v := data.(type) {
	case []any:
		result := make([]any, 0, len(v))
		for _, item := range v {
			if item != DeletedMarker {
				result = append(result, p.cleanupDeletedMarkers(item))
			}
		}
		return result

	case map[string]any:
		result := make(map[string]any)
		for key, value := range v {
			if value != DeletedMarker {
				result[key] = p.cleanupDeletedMarkers(value)
			}
		}
		return result

	default:
		return data
	}
}
