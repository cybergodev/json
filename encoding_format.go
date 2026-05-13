package json

import (
	"bytes"
	"context"
	"errors"
	"fmt"

	"github.com/cybergodev/json/internal"
)

// Prettify formats JSON string with indentation.
// This is the recommended method for formatting JSON strings.
// Uses default indentation of 2 spaces, configurable via Config.Indent and Config.Prefix.
//
// Errors:
//   - ErrProcessorClosed: processor has been closed
//   - ErrInvalidJSON: jsonStr is not valid JSON
//   - ErrSizeLimit: JSON exceeds MaxJSONSize
//
// Example:
//
//	pretty, err := processor.Prettify(`{"name":"Alice","age":30}`)
//	// Output:
//	// {
//	//   "name": "Alice",
//	//   "age": 30
//	// }
//
//	// Custom indentation
//	cfg := json.DefaultConfig()
//	cfg.Indent = "    " // 4 spaces
//	pretty, err := processor.Prettify(jsonStr, cfg)
func (p *Processor) Prettify(jsonStr string, cfg ...Config) (string, error) {
	if err := p.checkClosed(); err != nil {
		return "", err
	}

	options, err := p.prepareOptions(cfg...)
	if err != nil {
		return "", err
	}
	defer releaseConfig(options)

	if err := p.validateInput(jsonStr); err != nil {
		return "", err
	}

	// Check cache first
	cacheKey := p.createCacheKey("pretty", jsonStr, "", options)
	if cached, ok := p.getCachedResult(cacheKey); ok {
		if val, typeOk := cached.(string); typeOk {
			return val, nil
		}
		// Cache type mismatch - evict corrupted entry
		p.invalidateCachedResult(cacheKey)
	}

	// Parse with number preservation to maintain original number types
	decoder := newNumberPreservingDecoder(options.PreserveNumbers)
	data, err := decoder.DecodeToAny(jsonStr)
	if err != nil {
		return "", &JsonsError{
			Op:      "pretty",
			Message: fmt.Sprintf("failed to parse JSON: %v", err),
			Err:     ErrInvalidJSON,
		}
	}

	// Use custom encoder with pretty formatting to preserve number types
	config := PrettyConfig()
	config.PreserveNumbers = options.PreserveNumbers
	// Respect caller's Indent/Prefix if explicitly provided via options
	if options.Indent != "" {
		config.Indent = options.Indent
	}
	if options.Prefix != "" {
		config.Prefix = options.Prefix
	}

	encoder := newCustomEncoder(config)
	defer encoder.Close()

	result, err := encoder.Encode(data)
	if err != nil {
		return "", &JsonsError{
			Op:      "pretty",
			Message: "failed to format JSON",
			Err:     err,
		}
	}

	// Cache result if enabled
	p.setCachedResult(cacheKey, result, options)

	return result, nil
}

// formatJSONString formats a JSON string or encodes a non-JSON string.
func (p *Processor) formatJSONString(jsonStr string, pretty bool) (string, error) {
	isValid, validErr := p.Valid(jsonStr)
	if validErr != nil {
		// Distinguish processor errors (closed, context) from invalid JSON
		// Processor errors should propagate; invalid JSON falls through to string encoding
		if errors.Is(validErr, ErrProcessorClosed) || errors.Is(validErr, context.Canceled) {
			return "", validErr
		}
	}
	if isValid {
		if pretty {
			return p.Prettify(jsonStr)
		}
		return p.Compact(jsonStr)
	}
	// Not valid JSON - encode as a string value
	cfg := DefaultConfig()
	cfg.Pretty = pretty
	return p.EncodeWithConfig(jsonStr, cfg)
}

// Compact removes whitespace from JSON string.
// This is useful for minimizing JSON size for transmission or storage.
// The result is a single-line JSON string with no unnecessary whitespace.
//
// Errors:
//   - ErrProcessorClosed: processor has been closed
//   - ErrInvalidJSON: jsonStr is not valid JSON
//   - ErrSizeLimit: JSON exceeds MaxJSONSize
//
// Example:
//
//	compact, err := processor.Compact(`{
//	    "name": "Alice",
//	    "age": 30
//	}`)
//	// Output: {"name":"Alice","age":30}
func (p *Processor) Compact(jsonStr string, cfg ...Config) (string, error) {
	if err := p.checkClosed(); err != nil {
		return "", err
	}

	options, err := p.prepareOptions(cfg...)
	if err != nil {
		return "", err
	}
	defer releaseConfig(options)

	if err := p.validateInput(jsonStr); err != nil {
		return "", err
	}

	// Check cache first
	cacheKey := p.createCacheKey("compact", jsonStr, "", options)
	if cached, ok := p.getCachedResult(cacheKey); ok {
		if val, typeOk := cached.(string); typeOk {
			return val, nil
		}
		// Cache type mismatch - evict corrupted entry
		p.invalidateCachedResult(cacheKey)
	}

	// Parse with number preservation to maintain original number types
	decoder := newNumberPreservingDecoder(options.PreserveNumbers)
	data, err := decoder.DecodeToAny(jsonStr)
	if err != nil {
		return "", &JsonsError{
			Op:      "compact",
			Message: fmt.Sprintf("failed to parse JSON: %v", err),
			Err:     ErrInvalidJSON,
		}
	}

	// Use custom encoder with compact formatting to preserve number types
	config := DefaultConfig()
	config.PreserveNumbers = options.PreserveNumbers

	encoder := newCustomEncoder(config)
	defer encoder.Close()

	result, err := encoder.Encode(data)
	if err != nil {
		return "", &JsonsError{
			Op:      "compact",
			Message: "failed to compact JSON",
			Err:     err,
		}
	}

	// Cache result if enabled
	p.setCachedResult(cacheKey, result, options)

	return result, nil
}

// CompactBuffer appends to dst the JSON-encoded src with insignificant space characters elided.
// Compatible with encoding/json.Compact with optional Config support.
// This is the buffer-based counterpart to Compact, matching the encoding/json.Compact signature.
//
// Example:
//
//	var buf bytes.Buffer
//	err := processor.CompactBuffer(&buf, []byte(`{"name": "Alice"}`))
func (p *Processor) CompactBuffer(dst *bytes.Buffer, src []byte, cfg ...Config) error {
	compacted, err := p.Compact(string(src), cfg...)
	if err != nil {
		return err
	}
	_, err = dst.WriteString(compacted)
	return err
}

// Indent appends to dst an indented form of the JSON-encoded src.
// Compatible with encoding/json.Indent with optional Config support.
//
// Example:
//
//	var buf bytes.Buffer
//	err := processor.Indent(&buf, []byte(`{"name":"Alice"}`), "", "  ")
func (p *Processor) Indent(dst *bytes.Buffer, src []byte, prefix, indent string, cfg ...Config) error {
	var data any
	if err := p.Unmarshal(src, &data, cfg...); err != nil {
		return err
	}
	indented, err := p.MarshalIndent(data, prefix, indent, cfg...)
	if err != nil {
		return err
	}
	_, err = dst.Write(indented)
	return err
}

// HTMLEscape appends to dst the JSON-encoded src with HTML-safe escaping.
// Performs character-level escaping of <, >, &, U+2028, and U+2029 without re-encoding.
// Compatible with encoding/json.HTMLEscape.
//
// Example:
//
//	var buf bytes.Buffer
//	processor.HTMLEscape(&buf, []byte(`{"url":"<script>alert(1)</script>"}`))
func (p *Processor) HTMLEscape(dst *bytes.Buffer, src []byte, cfg ...Config) {
	_ = cfg // Config not used; character-level escaping requires no re-encoding
	internal.HTMLEscapeTo(dst, string(src))
}

