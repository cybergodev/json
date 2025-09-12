package json

import (
	"encoding/json"
	"fmt"
)

// EncodeWithOptions converts any Go value to JSON string with advanced options
func (p *Processor) EncodeWithOptions(value any, encOpts *EncodeConfig, opts ...*ProcessorOptions) (string, error) {
	if err := p.checkClosed(); err != nil {
		return "", err
	}

	if encOpts == nil {
		encOpts = DefaultEncodeConfig()
	}

	// Valid depth
	if encOpts.MaxDepth > 0 {
		if err := p.validateDepth(value, encOpts.MaxDepth, 0); err != nil {
			return "", err
		}
	}

	var result string
	var err error

	// Check if we need to use custom encoding features
	needsCustomEncoding := encOpts.DisableEscaping ||
		encOpts.EscapeUnicode ||
		encOpts.EscapeSlash ||
		!encOpts.EscapeNewlines ||
		!encOpts.EscapeTabs ||
		encOpts.CustomEscapes != nil ||
		encOpts.SortKeys ||
		encOpts.OmitEmpty ||
		!encOpts.EscapeHTML // Add EscapeHTML check

	if needsCustomEncoding {
		// Use custom encoder for advanced options
		encoder := NewCustomEncoder(encOpts)
		defer encoder.Close() // Ensure buffers are returned to pool
		result, err = encoder.Encode(value)
	} else {
		// Use standard JSON encoding for basic options
		var resultBytes []byte
		if encOpts.Pretty {
			resultBytes, err = json.MarshalIndent(value, encOpts.Prefix, encOpts.Indent)
		} else {
			resultBytes, err = json.Marshal(value)
		}
		if err == nil {
			result = string(resultBytes)
		}
	}

	if err != nil {
		return "", &JsonsError{
			Op:      "encode_with_options",
			Message: fmt.Sprintf("failed to encode value: %v", err),
			Err:     ErrOperationFailed,
		}
	}

	// Check size limit
	if int64(len(result)) > p.config.MaxJSONSize {
		return "", &JsonsError{
			Op:      "encode_with_options",
			Message: fmt.Sprintf("encoded JSON size %d exceeds maximum %d", len(result), p.config.MaxJSONSize),
			Err:     ErrSizeLimit,
		}
	}

	return result, nil
}

// validateDepth checks if the data structure exceeds maximum depth
func (p *Processor) validateDepth(value any, maxDepth, currentDepth int) error {
	if currentDepth > maxDepth {
		return &JsonsError{
			Op:      "validate_depth",
			Message: fmt.Sprintf("data structure depth %d exceeds maximum %d", currentDepth, maxDepth),
			Err:     ErrOperationFailed,
		}
	}

	switch v := value.(type) {
	case map[string]any:
		for _, val := range v {
			if err := p.validateDepth(val, maxDepth, currentDepth+1); err != nil {
				return err
			}
		}
	case []any:
		for _, val := range v {
			if err := p.validateDepth(val, maxDepth, currentDepth+1); err != nil {
				return err
			}
		}
	case map[any]any:
		for _, val := range v {
			if err := p.validateDepth(val, maxDepth, currentDepth+1); err != nil {
				return err
			}
		}
	}

	return nil
}

// ToJsonString converts any Go value to JSON string with HTML escaping (safe for web)
func (p *Processor) ToJsonString(value any, opts ...*ProcessorOptions) (string, error) {
	config := DefaultEncodeConfig()
	config.Pretty = false
	config.EscapeHTML = true
	return p.EncodeWithConfig(value, config, opts...)
}

// ToJsonStringPretty converts any Go value to pretty JSON string with HTML escaping
func (p *Processor) ToJsonStringPretty(value any, opts ...*ProcessorOptions) (string, error) {
	config := DefaultEncodeConfig()
	config.Pretty = true
	config.EscapeHTML = true
	return p.EncodeWithConfig(value, config, opts...)
}

// ToJsonStringStandard converts any Go value to compact JSON string without HTML escaping
func (p *Processor) ToJsonStringStandard(value any, opts ...*ProcessorOptions) (string, error) {
	return p.EncodeWithConfig(value, DefaultEncodeConfig(), opts...)
}

// Marshal converts any Go value to JSON bytes (similar to json.Marshal)
func (p *Processor) Marshal(value any, opts ...*ProcessorOptions) ([]byte, error) {
	jsonStr, err := p.ToJsonString(value, opts...)
	if err != nil {
		return nil, err
	}
	return []byte(jsonStr), nil
}

// MarshalIndent converts any Go value to indented JSON bytes (similar to json.MarshalIndent)
func (p *Processor) MarshalIndent(value any, prefix, indent string, opts ...*ProcessorOptions) ([]byte, error) {
	encOpts := DefaultEncodeConfig()
	encOpts.Pretty = true
	encOpts.Prefix = prefix
	encOpts.Indent = indent

	jsonStr, err := p.EncodeWithConfig(value, encOpts, opts...)
	if err != nil {
		return nil, err
	}
	return []byte(jsonStr), nil
}

// Unmarshal parses the JSON-encoded data and stores the result in the value pointed to by v.
// This method is fully compatible with encoding/json.Unmarshal.
func (p *Processor) Unmarshal(data []byte, v any, opts ...*ProcessorOptions) error {
	if err := p.checkClosed(); err != nil {
		return err
	}

	if v == nil {
		return &InvalidUnmarshalError{Type: nil}
	}

	// Convert bytes to string for internal processing
	jsonStr := string(data)

	// Use the existing Parse method which handles all the validation and parsing logic
	return p.Parse(jsonStr, v, opts...)
}

// EncodeStream encodes multiple values as a JSON array stream
func (p *Processor) EncodeStream(values any, pretty bool, opts ...*ProcessorOptions) (string, error) {
	if err := p.checkClosed(); err != nil {
		return "", err
	}

	// Encode as array
	config := DefaultEncodeConfig()
	config.Pretty = pretty
	return p.EncodeWithConfig(values, config, opts...)
}

// EncodeStreamWithOptions encodes multiple values as a JSON array stream with advanced options
func (p *Processor) EncodeStreamWithOptions(values any, encOpts *EncodeConfig, opts ...*ProcessorOptions) (string, error) {
	if err := p.checkClosed(); err != nil {
		return "", err
	}

	// Encode as array with options
	return p.EncodeWithConfig(values, encOpts, opts...)
}

// EncodeBatch encodes multiple key-value pairs as a JSON object
func (p *Processor) EncodeBatch(pairs map[string]any, pretty bool, opts ...*ProcessorOptions) (string, error) {
	config := DefaultEncodeConfig()
	config.Pretty = pretty
	return p.EncodeWithConfig(pairs, config, opts...)
}

// EncodeFields encodes struct fields selectively based on field names
func (p *Processor) EncodeFields(value any, fields []string, pretty bool, opts ...*ProcessorOptions) (string, error) {
	processor := p

	// First convert to JSON and parse back to get map representation
	config := DefaultEncodeConfig()
	config.Pretty = false
	tempJSON, err := processor.EncodeWithConfig(value, config, opts...)
	if err != nil {
		return "", err
	}

	// Parse to any and convert to map
	var anyData any
	err = processor.Parse(tempJSON, &anyData, opts...)
	if err != nil {
		return "", err
	}

	// Check if the result is actually a map
	data, ok := anyData.(map[string]any)
	if !ok {
		return "", &JsonsError{
			Op:      "encode_fields",
			Message: fmt.Sprintf("JSON is not an object, got %T", anyData),
			Err:     ErrTypeMismatch,
		}
	}

	// Filter fields
	filtered := make(map[string]any)
	for _, field := range fields {
		if val, exists := data[field]; exists {
			filtered[field] = val
		}
	}

	finalConfig := DefaultEncodeConfig()
	finalConfig.Pretty = pretty
	return processor.EncodeWithConfig(filtered, finalConfig, opts...)
}

// EncodeWithConfig converts any Go value to JSON string with full configuration control
func (p *Processor) EncodeWithConfig(value any, config *EncodeConfig, opts ...*ProcessorOptions) (string, error) {
	if err := p.checkClosed(); err != nil {
		return "", err
	}

	if config == nil {
		config = DefaultEncodeConfig()
	}

	// Valid depth
	if config.MaxDepth > 0 {
		if err := p.validateDepth(value, config.MaxDepth, 0); err != nil {
			return "", err
		}
	}

	var result string
	var err error

	// Check if we need to use custom encoding features
	needsCustomEncoding := config.DisableEscaping ||
		config.EscapeUnicode ||
		config.EscapeSlash ||
		!config.EscapeNewlines ||
		!config.EscapeTabs ||
		config.CustomEscapes != nil ||
		config.SortKeys ||
		config.OmitEmpty ||
		!config.EscapeHTML ||
		config.FloatPrecision > 0 ||
		!config.IncludeNulls

	if needsCustomEncoding {
		// Use custom encoder for advanced options
		encoder := NewCustomEncoder(config)
		defer encoder.Close() // Ensure buffers are returned to pool
		result, err = encoder.Encode(value)
	} else {
		// Use standard JSON encoding for basic options
		var resultBytes []byte
		if config.Pretty {
			resultBytes, err = json.MarshalIndent(value, config.Prefix, config.Indent)
		} else {
			resultBytes, err = json.Marshal(value)
		}
		if err == nil {
			result = string(resultBytes)
		}
	}

	if err != nil {
		return "", &JsonsError{
			Op:      "encode_with_config",
			Message: fmt.Sprintf("failed to encode value: %v", err),
			Err:     ErrOperationFailed,
		}
	}

	// Check size limit
	if int64(len(result)) > p.config.MaxJSONSize {
		return "", &JsonsError{
			Op:      "encode_with_config",
			Message: fmt.Sprintf("encoded JSON size %d exceeds maximum %d", len(result), p.config.MaxJSONSize),
			Err:     ErrSizeLimit,
		}
	}

	return result, nil
}

// EncodeWithTags encodes struct with custom JSON tags handling
func (p *Processor) EncodeWithTags(value any, pretty bool, opts ...*ProcessorOptions) (string, error) {
	// This is a placeholder for future implementation of custom tag handling
	// For now, use standard encoding
	config := DefaultEncodeConfig()
	config.Pretty = pretty
	return p.EncodeWithConfig(value, config, opts...)
}
