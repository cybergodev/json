package json

import (
	"fmt"
)

// Parse parses a JSON string into the provided target with improved error handling
func (p *Processor) Parse(jsonStr string, target any, opts ...*ProcessorOptions) error {
	if err := p.checkClosed(); err != nil {
		return err
	}

	options, err := p.prepareOptions(opts...)
	if err != nil {
		return err
	}

	if err := p.validateInput(jsonStr); err != nil {
		return err
	}

	if target == nil {
		return &JsonsError{
			Op:      "parse",
			Message: "target cannot be nil, use Parse for any type result",
			Err:     ErrOperationFailed,
		}
	}

	// Parse with number preservation to maintain original format
	if options.PreserveNumbers {
		// Use NumberPreservingDecoder to keep json.Number as-is
		decoder := NewNumberPreservingDecoder(true)
		data, err := decoder.DecodeToAny(jsonStr)
		if err != nil {
			return &JsonsError{
				Op:      "parse",
				Message: fmt.Sprintf("invalid JSON for target type %T: %v", target, err),
				Err:     ErrInvalidJSON,
			}
		}

		// For *any type, directly assign the result
		if anyPtr, ok := target.(*any); ok {
			*anyPtr = data
			return nil
		}

		// For other types, use custom encoder/decoder to preserve numbers
		config := NewPrettyConfig()
		config.PreserveNumbers = true

		encoder := NewCustomEncoder(config)
		defer encoder.Close()

		encodedJson, err := encoder.Encode(data)
		if err != nil {
			return &JsonsError{
				Op:      "parse",
				Message: fmt.Sprintf("failed to encode data for target type %T: %v", target, err),
				Err:     ErrOperationFailed,
			}
		}

		// Use number-preserving unmarshal for final conversion
		if err := PreservingUnmarshal([]byte(encodedJson), target, true); err != nil {
			return &JsonsError{
				Op:      "parse",
				Message: fmt.Sprintf("invalid JSON for target type %T: %v", target, err),
				Err:     ErrInvalidJSON,
			}
		}
	} else {
		// Standard parsing without number preservation
		if err := PreservingUnmarshal([]byte(jsonStr), target, false); err != nil {
			return &JsonsError{
				Op:      "parse",
				Message: fmt.Sprintf("invalid JSON for target type %T: %v", target, err),
				Err:     ErrInvalidJSON,
			}
		}
	}

	return nil
}

// Valid validates JSON format without parsing the entire structure
func (p *Processor) Valid(jsonStr string, opts ...*ProcessorOptions) (bool, error) {
	if err := p.checkClosed(); err != nil {
		return false, err
	}

	if err := p.validateInput(jsonStr); err != nil {
		return false, err
	}

	// Prepare options
	options, err := p.prepareOptions(opts...)
	if err != nil {
		return false, err
	}

	// Check cache first
	cacheKey := p.createCacheKey("validate", jsonStr, "", options)
	if cached, ok := p.getCachedResult(cacheKey); ok {
		return cached.(bool), nil
	}

	// Valid JSON by attempting to parse
	decoder := NewNumberPreservingDecoder(options.PreserveNumbers)
	_, err = decoder.DecodeToAny(jsonStr)

	if err != nil {
		// Return error for invalid JSON
		return false, &JsonsError{
			Op:      "validate",
			Message: fmt.Sprintf("invalid JSON: %v", err),
			Err:     ErrInvalidJSON,
		}
	}

	// Cache result if enabled
	p.setCachedResult(cacheKey, true, options)

	return true, nil
}

// FormatPretty formats JSON string with indentation
func (p *Processor) FormatPretty(jsonStr string, opts ...*ProcessorOptions) (string, error) {
	if err := p.checkClosed(); err != nil {
		return "", err
	}

	options, err := p.prepareOptions(opts...)
	if err != nil {
		return "", err
	}

	if err := p.validateInput(jsonStr); err != nil {
		return "", err
	}

	// Check cache first
	cacheKey := p.createCacheKey("pretty", jsonStr, "", options)
	if cached, ok := p.getCachedResult(cacheKey); ok {
		return cached.(string), nil
	}

	// Parse with number preservation to maintain original number types
	decoder := NewNumberPreservingDecoder(options.PreserveNumbers)
	data, err := decoder.DecodeToAny(jsonStr)
	if err != nil {
		return "", &JsonsError{
			Op:      "pretty",
			Message: fmt.Sprintf("failed to parse JSON: %v", err),
			Err:     ErrInvalidJSON,
		}
	}

	// Use custom encoder with pretty formatting to preserve number types
	config := NewPrettyConfig()
	config.PreserveNumbers = options.PreserveNumbers

	encoder := NewCustomEncoder(config)
	defer encoder.Close()

	result, err := encoder.Encode(data)
	if err != nil {
		return "", &JsonsError{
			Op:      "pretty",
			Message: fmt.Sprintf("failed to format JSON: %v", err),
			Err:     ErrOperationFailed,
		}
	}

	// Cache result if enabled
	p.setCachedResult(cacheKey, result, options)

	return result, nil
}

// Compact removes whitespace from JSON string
func (p *Processor) Compact(jsonStr string, opts ...*ProcessorOptions) (string, error) {
	if err := p.checkClosed(); err != nil {
		return "", err
	}

	options, err := p.prepareOptions(opts...)
	if err != nil {
		return "", err
	}

	if err := p.validateInput(jsonStr); err != nil {
		return "", err
	}

	// Check cache first
	cacheKey := p.createCacheKey("compact", jsonStr, "", options)
	if cached, ok := p.getCachedResult(cacheKey); ok {
		return cached.(string), nil
	}

	// Parse with number preservation to maintain original number types
	decoder := NewNumberPreservingDecoder(options.PreserveNumbers)
	data, err := decoder.DecodeToAny(jsonStr)
	if err != nil {
		return "", &JsonsError{
			Op:      "compact",
			Message: fmt.Sprintf("failed to parse JSON: %v", err),
			Err:     ErrInvalidJSON,
		}
	}

	// Use custom encoder with compact formatting to preserve number types
	config := NewCompactConfig()
	config.PreserveNumbers = options.PreserveNumbers

	encoder := NewCustomEncoder(config)
	defer encoder.Close()

	result, err := encoder.Encode(data)
	if err != nil {
		return "", &JsonsError{
			Op:      "compact",
			Message: fmt.Sprintf("failed to compact JSON: %v", err),
			Err:     ErrOperationFailed,
		}
	}

	// Cache result if enabled
	p.setCachedResult(cacheKey, result, options)

	return result, nil
}
