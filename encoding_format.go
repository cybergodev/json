package json

import (
	"bytes"
	"context"
	"errors"
	"fmt"

	"github.com/cybergodev/json/internal"
)

// appendCompactJSON copies src to dst with insignificant whitespace elided.
// String literals (including their escapes) are copied byte-for-byte, so key
// order, number literals ("1e3", "1.10", >2^53 integers), and existing escape
// sequences are preserved exactly — matching encoding/json.Compact, which is
// a pure whitespace filter (D-002: the previous parse→re-encode
// implementation resorted keys, rewrote literals, and lost precision beyond
// 2^53). The caller MUST have validated src as JSON first; this function
// makes no syntactic judgments of its own.
func appendCompactJSON(dst, src []byte) []byte {
	inString := false
	escaped := false
	for i := 0; i < len(src); i++ {
		c := src[i]
		if inString {
			dst = append(dst, c)
			if escaped {
				escaped = false
			} else if c == '\\' {
				escaped = true
			} else if c == '"' {
				inString = false
			}
			continue
		}
		switch c {
		case ' ', '\t', '\r', '\n':
			// Insignificant whitespace outside strings: dropped.
		case '"':
			dst = append(dst, c)
			inString = true
		default:
			dst = append(dst, c)
		}
	}
	return dst
}

// nextSignificant returns the index of the next non-whitespace byte at or
// after i, or -1 when the remainder is all whitespace.
func nextSignificant(src []byte, i int) int {
	for ; i < len(src); i++ {
		switch src[i] {
		case ' ', '\t', '\r', '\n':
		default:
			return i
		}
	}
	return -1
}

// appendIndentJSON copies src to dst re-indented, preserving every non-space
// byte exactly (values, key order, number literals, escapes) — the same
// whitespace-only contract as encoding/json.Indent (D-002: the previous
// unmarshal→re-marshal pipeline corrupted number literals beyond 2^53 and
// resorted keys). Empty containers stay inline ({} and []), a space follows
// each colon, and each nesting level adds one indent after prefix. The
// caller MUST have validated src as JSON first.
func appendIndentJSON(dst, src []byte, prefix, indent string) []byte {
	depth := 0
	inString := false
	escaped := false
	// lineHasContent tracks whether anything was emitted since the last
	// newline: a closing bracket only emits its own newline when the
	// container had members.
	lineHasContent := false

	writeNewline := func() {
		dst = append(dst, '\n')
		dst = append(dst, prefix...)
		for i := 0; i < depth; i++ {
			dst = append(dst, indent...)
		}
	}

	for i := 0; i < len(src); i++ {
		c := src[i]
		if inString {
			dst = append(dst, c)
			if escaped {
				escaped = false
			} else if c == '\\' {
				escaped = true
			} else if c == '"' {
				inString = false
				lineHasContent = true
			}
			continue
		}
		switch c {
		case ' ', '\t', '\r', '\n':
			// Insignificant whitespace: dropped (replaced by structural newlines).
		case '"':
			dst = append(dst, c)
			inString = true
			lineHasContent = true
		case '{', '[':
			if next := nextSignificant(src, i+1); next != -1 &&
				((c == '{' && src[next] == '}') || (c == '[' && src[next] == ']')) {
				// Empty container: emit inline as {} / [] like encoding/json.
				dst = append(dst, c, src[next])
				i = next
				lineHasContent = true
				continue
			}
			dst = append(dst, c)
			depth++
			writeNewline()
			lineHasContent = false
		case '}', ']':
			depth--
			if lineHasContent {
				writeNewline()
			}
			dst = append(dst, c)
			lineHasContent = true
		case ',':
			dst = append(dst, c)
			writeNewline()
			lineHasContent = false
		case ':':
			dst = append(dst, c, ' ')
			lineHasContent = true
		default:
			dst = append(dst, c)
			lineHasContent = true
		}
	}
	return dst
}

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

	if err := p.validateInputForOptions(jsonStr, options); err != nil {
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

	// Syntax-check via the same decoder as before (identical error surface).
	decoder := newNumberPreservingDecoder(options.PreserveNumbers)
	if _, err := decoder.DecodeToAny(jsonStr); err != nil {
		return "", &JsonsError{
			Op:      "pretty",
			Message: fmt.Sprintf("failed to parse JSON: %v", err),
			Err:     ErrInvalidJSON,
		}
	}

	// Reformat with a byte-preserving pass (D-002): only whitespace changes,
	// so key order, number literals (including >2^53 integers), and existing
	// escapes survive exactly as written. The previous parse→re-encode
	// pipeline resorted keys, rewrote "1e3"→1000, corrupted integers beyond
	// 2^53, and added HTML escapes — none of which a formatter should touch.
	indent := options.Indent
	if indent == "" {
		indent = "  "
	}
	buf := make([]byte, 0, len(jsonStr)+len(jsonStr)/4)
	result := string(appendIndentJSON(buf, internal.StringToBytes(jsonStr), options.Prefix, indent))

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
// The package-level mirror of this method is CompactString (json.CompactString(s)
// ↔ processor.Compact(s)). The buffer-based encoding/json-compatible form is
// CompactBuffer (json.Compact(dst, src) ↔ processor.CompactBuffer).
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

	if err := p.validateInputForOptions(jsonStr, options); err != nil {
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

	// Syntax-check via the same decoder as before (identical error surface).
	decoder := newNumberPreservingDecoder(options.PreserveNumbers)
	if _, err := decoder.DecodeToAny(jsonStr); err != nil {
		return "", &JsonsError{
			Op:      "compact",
			Message: fmt.Sprintf("failed to parse JSON: %v", err),
			Err:     ErrInvalidJSON,
		}
	}

	// Elide insignificant whitespace with a byte-preserving pass (D-002):
	// key order, number literals ("1e3", "1.10", >2^53 integers), and
	// existing escapes are kept exactly, matching encoding/json.Compact. The
	// previous parse→re-encode pipeline resorted keys, rewrote literals, and
	// lost precision — data corruption from what claims to be a compaction.
	result := string(appendCompactJSON(make([]byte, 0, len(jsonStr)), internal.StringToBytes(jsonStr)))

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
//
// Errors:
//   - ErrProcessorClosed: processor has been closed
//   - ErrInvalidJSON: src is not valid JSON
//   - ErrSizeLimit: src exceeds MaxJSONSize
//   - any error returned while writing to dst
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
//
// Errors:
//   - ErrProcessorClosed: processor has been closed
//   - ErrInvalidJSON: src is not valid JSON
//   - UnmarshalTypeError: a JSON value does not match the intermediate Go type
//   - UnsupportedTypeError / UnsupportedValueError / MarshalerError: value cannot be encoded
//   - ErrSizeLimit: input or output exceeds MaxJSONSize
//   - ErrDepthLimit: encoding exceeds the maximum nesting depth
//   - any error returned while writing to dst
func (p *Processor) Indent(dst *bytes.Buffer, src []byte, prefix, indent string, cfg ...Config) error {
	// Validate via the same gate as before (security limits + syntax), then
	// re-indent with a byte-preserving pass: values, key order, number
	// literals, and escapes are untouched — matching encoding/json.Indent.
	// The previous unmarshal→re-marshal pipeline corrupted integers beyond
	// 2^53 and resorted keys (D-002).
	var data any
	if err := p.Unmarshal(src, &data, cfg...); err != nil {
		return err
	}
	_, err := dst.Write(appendIndentJSON(make([]byte, 0, len(src)+len(src)/4), src, prefix, indent))
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
