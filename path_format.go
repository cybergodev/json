package json

import (
	"bytes"
	"fmt"
)

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
	config := DefaultEncodeConfig()
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

// FormatCompact removes whitespace from JSON string (alias for Compact)
func (p *Processor) FormatCompact(jsonStr string, opts ...*ProcessorOptions) (string, error) {
	return p.Compact(jsonStr, opts...)
}

// CompactBuffer appends to dst the JSON-encoded src with insignificant space characters elided.
//
// This method provides compatibility with the encoding/json.Compact function signature,
// with the addition of optional ProcessorOptions for advanced customization.
//
// API Design Note:
//   - Processor.Compact(jsonStr) operates on strings and returns formatted strings
//   - Processor.CompactBuffer(dst, src) operates on buffers for stream processing
//   - This naming convention distinguishes string operations from buffer operations
//   - Both methods support optional ProcessorOptions for consistency
//
// For package-level usage with standard library signature, see json.Compact(dst, src).
func (p *Processor) CompactBuffer(dst *bytes.Buffer, src []byte, opts ...*ProcessorOptions) error {
	compacted, err := p.Compact(string(src), opts...)
	if err != nil {
		return err
	}
	_, err = dst.WriteString(compacted)
	return err
}

// IndentBuffer appends to dst an indented form of the JSON-encoded src.
//
// This method provides compatibility with the encoding/json.Indent function signature,
// with the addition of optional ProcessorOptions for advanced customization.
//
// API Design Note:
//   - Processor.FormatPretty(jsonStr) operates on strings for pretty formatting
//   - Processor.IndentBuffer(dst, src, prefix, indent) operates on buffers for stream processing
//   - This naming convention distinguishes string operations from buffer operations
//   - Both approaches support optional ProcessorOptions for consistency
//
// For package-level usage with standard library signature, see json.Indent(dst, src, prefix, indent).
func (p *Processor) IndentBuffer(dst *bytes.Buffer, src []byte, prefix, indent string, opts ...*ProcessorOptions) error {
	var data any
	if err := p.Unmarshal(src, &data, opts...); err != nil {
		return err
	}
	indented, err := p.MarshalIndent(data, prefix, indent, opts...)
	if err != nil {
		return err
	}
	_, err = dst.Write(indented)
	return err
}

// HTMLEscapeBuffer appends to dst the JSON-encoded src with HTML-safe escaping.
//
// This method provides compatibility with the encoding/json.HTMLEscape function signature,
// with the addition of optional ProcessorOptions for advanced customization.
//
// The function replaces &, <, and > with \u0026, \u003c, and \u003e to avoid certain
// safety problems that can arise when embedding JSON in HTML.
//
// API Design Note:
//   - The "Buffer" suffix distinguishes this buffer operation from potential string operations
//   - This naming is consistent with CompactBuffer and IndentBuffer
//   - Processor methods use descriptive names to avoid ambiguity
//
// For package-level usage with standard library signature, see json.HTMLEscape(dst, src).
func (p *Processor) HTMLEscapeBuffer(dst *bytes.Buffer, src []byte, opts ...*ProcessorOptions) {
	var data any
	if err := p.Unmarshal(src, &data, opts...); err != nil {
		dst.Write(src)
		return
	}

	config := DefaultEncodeConfig()
	config.EscapeHTML = true
	escaped, err := p.EncodeWithConfig(data, config, opts...)
	if err != nil {
		dst.Write(src)
		return
	}

	dst.WriteString(escaped)
}
