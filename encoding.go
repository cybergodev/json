package json

import (
	"bufio"
	"bytes"
	"encoding"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/cybergodev/json/internal"
)

// Token holds a value of one of these types:
//
//	Delim, for the four JSON delimiters [ ] { }
//	bool, for JSON booleans
//	float64, for JSON numbers
//	Number, for JSON numbers
//	string, for JSON string literals
//	nil, for JSON null
type Token any

// Delim is a JSON delimiter.
type Delim rune

// String returns the string representation of the delimiter.
func (d Delim) String() string {
	return string(d)
}

// Number represents a JSON number literal.
type Number string

// String returns the literal text of the number.
func (n Number) String() string { return string(n) }

// Float64 returns the number as a float64.
func (n Number) Float64() (float64, error) {
	return strconv.ParseFloat(string(n), 64)
}

// Int64 returns the number as an int64.
func (n Number) Int64() (int64, error) {
	return strconv.ParseInt(string(n), 10, 64)
}

// isSpace reports whether the character is a JSON whitespace character.
func isSpace(c byte) bool {
	return internal.IsSpace(c)
}

// isDigit reports whether the character is a digit.
func isDigit(c byte) bool {
	return internal.IsDigit(c)
}

func getEncoderBuffer() *bytes.Buffer {
	return internal.GetEncoderBuffer()
}

func putEncoderBuffer(buf *bytes.Buffer) {
	internal.PutEncoderBuffer(buf)
}

// Encoder writes JSON values to an output stream.
// This type is fully compatible with encoding/json.Encoder.
type Encoder struct {
	w          io.Writer
	escapeHTML bool
	indent     string
	prefix     string
}

// NewEncoder returns a new encoder that writes to w.
// This function is fully compatible with encoding/json.NewEncoder.
//
// The optional cfg parameter allows customization of encoding behavior.
// If no configuration is provided, default settings are used.
//
// Example:
//
//	// Default encoder
//	encoder := json.NewEncoder(writer)
//
//	// With configuration
//	cfg := json.DefaultConfig()
//	cfg.Pretty = true
//	encoder := json.NewEncoder(writer, cfg)
func NewEncoder(w io.Writer, cfg ...Config) *Encoder {
	enc := &Encoder{
		w:          w,
		escapeHTML: true, // Default behavior matches encoding/json
	}

	// Apply configuration if provided
	if len(cfg) > 0 {
		enc.escapeHTML = cfg[0].EscapeHTML
		if cfg[0].Pretty {
			enc.prefix = cfg[0].Prefix
			enc.indent = cfg[0].Indent
			if enc.indent == "" {
				enc.indent = "  " // Default indent
			}
		}
	}

	return enc
}

// Encode writes the JSON encoding of v to the stream,
// followed by a newline character.
//
// See the documentation for Marshal for details about the
// conversion of Go values to JSON.
func (enc *Encoder) Encode(v any) error {
	var newline = []byte{'\n'}
	// Get the current processor on each Encode call to avoid stale references
	// after SetGlobalProcessor or ShutdownGlobalProcessor.
	processor := getDefaultProcessor()
	if processor == nil {
		return errInternalError
	}

	// PERFORMANCE: Fast path for simple types with no custom encoding
	// Avoids Config creation and EncodeWithConfig overhead for common cases
	if enc.indent == "" && enc.prefix == "" {
		// Try fast encoder directly
		encoder := internal.GetEncoder()
		err := encoder.EncodeValue(v)
		if err == nil {
			data := encoder.Bytes()
			// SECURITY: Check output size against configured limit
			if int64(len(data)) > processor.config.MaxJSONSize {
				internal.PutEncoder(encoder)
				return newSizeLimitError("encode", int64(len(data)), processor.config.MaxJSONSize)
			}
			// Apply HTML escaping if needed
			if enc.escapeHTML && internal.NeedsHTMLEscapeBytes(data) {
				escaped := internal.HTMLEscapeBytes(data)
				_, err = enc.w.Write(escaped)
				internal.PutHTMLEscapeBytes(escaped)
			} else {
				_, err = enc.w.Write(data)
			}
			if err == nil {
				_, err = enc.w.Write(newline)
			}
			internal.PutEncoder(encoder)
			return err
		}
		// Fast path failed, fall through to full encoding
		internal.PutEncoder(encoder)
	}

	// Full encoding path for complex cases with indentation or config
	// Use processor's config as base to inherit settings like PreserveNumbers,
	// FloatPrecision, etc. Only override EscapeHTML when Encoder was explicitly
	// set to false via SetEscapeHTML(false).
	config := processor.GetConfig()
	if !enc.escapeHTML {
		config.EscapeHTML = false
	}

	if enc.indent != "" || enc.prefix != "" {
		config.Pretty = true
		config.Indent = enc.indent
		config.Prefix = enc.prefix
	}

	// Encode the value using internal method that accepts pre-built config
	jsonStr, err := processor.EncodeWithConfig(v, config)
	if err != nil {
		return err
	}

	// Write to the output stream with a newline
	if _, err := enc.w.Write(internal.StringToBytes(jsonStr)); err != nil {
		return err
	}
	_, err = enc.w.Write(newline)
	return err
}

// SetEscapeHTML specifies whether problematic HTML characters
// should be escaped inside JSON quoted strings.
// The default behavior is to escape &, <, and > to \u0026, \u003c, and \u003e
// to avoid certain safety problems that can arise when embedding JSON in HTML.
//
// In non-HTML settings where the escaping interferes with the readability
// of the output, SetEscapeHTML(false) disables this behavior.
func (enc *Encoder) SetEscapeHTML(on bool) {
	enc.escapeHTML = on
}

// SetIndent instructs the encoder to format each subsequent encoded
// value as if indented by the package-level function Indent(dst, src, prefix, indent).
// Calling SetIndent("", "") disables indentation.
func (enc *Encoder) SetIndent(prefix, indent string) {
	enc.prefix = prefix
	enc.indent = indent
}

// Decoder reads and decodes JSON values from an input stream.
// This type is fully compatible with encoding/json.Decoder.
type Decoder struct {
	r                     io.Reader
	buf                   *bufio.Reader
	useNumber             bool
	disallowUnknownFields bool
	offset                int64 // total bytes read from input
	maxNestingDepth       int   // maximum allowed nesting depth for containers
	maxBytes              int64 // maximum bytes for a single value (0 = unlimited)
}

// NewDecoder returns a new decoder that reads from r.
// This function is fully compatible with encoding/json.NewDecoder.
//
// The optional cfg parameter allows customization of decoding behavior.
// If no configuration is provided, default settings are used.
//
// Example:
//
//	// Default decoder
//	decoder := json.NewDecoder(reader)
//
//	// With custom configuration
//	cfg := json.DefaultConfig()
//	cfg.DisallowUnknown = true
//	decoder := json.NewDecoder(reader, cfg)
func NewDecoder(r io.Reader, cfg ...Config) *Decoder {
	maxDepth := DefaultMaxNestingDepth
	dec := &Decoder{
		r:               r,
		buf:             bufio.NewReader(r),
		maxNestingDepth: maxDepth,
	}
	// Apply config if provided
	if len(cfg) > 0 {
		dec.disallowUnknownFields = cfg[0].DisallowUnknown
		if cfg[0].MaxNestingDepthSecurity > 0 {
			dec.maxNestingDepth = cfg[0].MaxNestingDepthSecurity
		}
		if cfg[0].MaxJSONSize > 0 {
			dec.maxBytes = cfg[0].MaxJSONSize
		}
	}
	return dec
}

// Decode reads the next JSON-encoded value from its input and stores it in v.
func (dec *Decoder) Decode(v any) error {
	// Get the current processor on each Decode call to avoid stale references.
	processor := getDefaultProcessor()
	if processor == nil {
		return errInternalError
	}

	if v == nil {
		return &InvalidUnmarshalError{Type: nil}
	}

	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Pointer || rv.IsNil() {
		return &InvalidUnmarshalError{Type: reflect.TypeOf(v)}
	}

	// Read the next JSON value from the stream
	data, err := dec.readValue()
	if err != nil {
		return err
	}

	// Handle UseNumber directly for compatibility
	if dec.useNumber {
		rv = rv.Elem()
		switch rv.Kind() {
		case reflect.Interface, reflect.Map, reflect.Slice:
			// For interface{}, map[string]any, and []any targets,
			// use NumberPreservingDecoder to convert json.Number → Number.
			decoder := newNumberPreservingDecoder(true)
			result, err := decoder.DecodeToAny(string(data))
			if err != nil {
				return err
			}
			rv.Set(reflect.ValueOf(result))
			return nil
		default:
			// For concrete struct types, decode directly using UseNumber
			// to avoid the intermediate any → marshal/unmarshal round-trip.
			inner := json.NewDecoder(bytes.NewReader(data))
			inner.UseNumber()
			if dec.disallowUnknownFields {
				inner.DisallowUnknownFields()
			}
			return inner.Decode(v)
		}
	}

	// Use the processor's Unmarshal method for normal cases
	if dec.disallowUnknownFields {
		cfg := DefaultConfig()
		cfg.DisallowUnknown = true
		return processor.Unmarshal(data, v, cfg)
	}
	return processor.Unmarshal(data, v)
}

// UseNumber causes the Decoder to unmarshal a number into an interface{} as a
// Number instead of as a float64.
func (dec *Decoder) UseNumber() {
	dec.useNumber = true
}

// DisallowUnknownFields causes the Decoder to return an error when the destination
// is a struct and the input contains object keys which do not match any
// non-ignored, exported fields in the destination.
func (dec *Decoder) DisallowUnknownFields() {
	dec.disallowUnknownFields = true
}

// Buffered returns a reader of the data remaining in the Decoder's buffer.
// The reader is valid until the next call to Decode.
func (dec *Decoder) Buffered() io.Reader {
	return dec.buf
}

// InputOffset returns the input stream byte offset of the current decoder position.
// The offset gives the location of the end of the most recently returned token
// and the beginning of the next token.
func (dec *Decoder) InputOffset() int64 {
	return dec.offset
}

// More reports whether there is another element in the current array or object being parsed.
func (dec *Decoder) More() bool {
	// Peek at the next byte to see if there's more data
	b, err := dec.buf.Peek(1)
	if err != nil {
		return false
	}

	// Skip whitespace
	for len(b) > 0 && isSpace(b[0]) {
		if _, err := dec.buf.ReadByte(); err != nil {
			return false
		}
		b, err = dec.buf.Peek(1)
		if err != nil {
			return false
		}
	}

	if len(b) == 0 {
		return false
	}

	// Check if we're at the end of an array or object
	return b[0] != ']' && b[0] != '}'
}

// Token returns the next JSON token in the input stream.
// At the end of the input stream, Token returns nil, io.EOF.
func (dec *Decoder) Token() (Token, error) {
	// Skip whitespace and separators
	for {
		b, err := dec.buf.ReadByte()
		if err != nil {
			return nil, err
		}
		dec.offset++

		if !isSpace(b) && b != ':' && b != ',' {
			return dec.parseToken(b)
		}
	}
}

// readValue reads a complete JSON value from the input stream.
// It handles objects, arrays, strings, numbers, booleans, and null.
func (dec *Decoder) readValue() ([]byte, error) {
	buf := getEncoderBuffer()
	defer putEncoderBuffer(buf)

	// Step 1: Find the first non-whitespace character to determine value type
	var firstChar byte
	for {
		b, err := dec.buf.ReadByte()
		if err != nil {
			return nil, err
		}
		dec.offset++

		if !isSpace(b) {
			firstChar = b
			buf.WriteByte(b)
			break
		}
	}

	// Step 2: Handle based on value type
	switch firstChar {
	case '"':
		// String value - read until closing quote
		return dec.readStringValue(buf)
	case '{', '[':
		// Object or array - track depth to find matching close
		return dec.readContainerValue(buf, firstChar)
	default:
		// Primitive value (number, boolean, null) - read until delimiter
		return dec.readPrimitiveValue(buf)
	}
}

// readStringValue reads a complete JSON string value
func (dec *Decoder) readStringValue(buf *bytes.Buffer) ([]byte, error) {
	escaped := false

	for {
		b, err := dec.buf.ReadByte()
		if err != nil {
			return nil, err
		}
		dec.offset++
		buf.WriteByte(b)

		if escaped {
			escaped = false
			continue
		}

		switch b {
		case '\\':
			escaped = true
		case '"':
			// String complete
			result := make([]byte, buf.Len())
			copy(result, buf.Bytes())
			return result, nil
		}
	}
}

// readContainerValue reads a complete JSON object or array.
// openChar is the opening delimiter ('{' or '[') used to validate matching close delimiters.
// Enforces maxNestingDepth to prevent stack exhaustion from deeply nested input.
func (dec *Decoder) readContainerValue(buf *bytes.Buffer, openChar byte) ([]byte, error) {
	depth := 1
	maxDepth := dec.maxNestingDepth
	if maxDepth <= 0 {
		maxDepth = DefaultMaxNestingDepth
	}
	inString := false
	escaped := false

	for {
		b, err := dec.buf.ReadByte()
		if err != nil {
			if err == io.EOF && buf.Len() > 0 {
				return nil, fmt.Errorf("unexpected EOF in JSON container")
			}
			return nil, err
		}
		dec.offset++
		buf.WriteByte(b)
		if dec.maxBytes > 0 && int64(buf.Len()) > dec.maxBytes {
			return nil, fmt.Errorf("streaming value size %d exceeds maximum allowed %d bytes", buf.Len(), dec.maxBytes)
		}

		if escaped {
			escaped = false
			continue
		}

		if inString {
			switch b {
			case '\\':
				escaped = true
			case '"':
				inString = false
			}
			continue
		}

		switch b {
		case '"':
			inString = true
		case '{', '[':
			depth++
			if depth > maxDepth {
				return nil, fmt.Errorf("JSON nesting depth %d exceeds maximum allowed depth %d", depth, maxDepth)
			}
		case '}', ']':
			expectedClose := byte('}')
			if openChar == '[' {
				expectedClose = ']'
			}
			if b != expectedClose {
				return nil, fmt.Errorf("mismatched JSON delimiters: expected '%c' but got '%c'", expectedClose, b)
			}
			depth--
			if depth == 0 {
				result := make([]byte, buf.Len())
				copy(result, buf.Bytes())
				return result, nil
			}
		}
	}
}

// readPrimitiveValue reads a JSON primitive (number, boolean, null)
func (dec *Decoder) readPrimitiveValue(buf *bytes.Buffer) ([]byte, error) {
	for {
		b, err := dec.buf.ReadByte()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
		dec.offset++

		// Check for value terminators
		if isSpace(b) || b == ',' || b == '}' || b == ']' {
			if err := dec.buf.UnreadByte(); err != nil {
				return nil, fmt.Errorf("unread failed: %w", err)
			}
			dec.offset--
			break
		}

		buf.WriteByte(b)
	}

	result := make([]byte, buf.Len())
	copy(result, buf.Bytes())
	return result, nil
}

// parseToken parses a single JSON token starting with the given byte
func (dec *Decoder) parseToken(b byte) (Token, error) {
	switch b {
	case '{':
		return Delim('{'), nil
	case '}':
		return Delim('}'), nil
	case '[':
		return Delim('['), nil
	case ']':
		return Delim(']'), nil
	case '"':
		return dec.parseString()
	case 't', 'f':
		return dec.parseBoolean(b)
	case 'n':
		return dec.parseNull()
	default:
		if isDigit(b) || b == '-' {
			return dec.parseNumber(b)
		}
		return nil, &SyntaxError{
			msg:    fmt.Sprintf("invalid character '%c' looking for beginning of value", b),
			Offset: dec.offset - 1,
		}
	}
}

func (dec *Decoder) parseString() (string, error) {
	buf := getEncoderBuffer()
	defer putEncoderBuffer(buf)

	for {
		b, err := dec.buf.ReadByte()
		if err != nil {
			return "", err
		}
		dec.offset++

		if b == '"' {
			return buf.String(), nil
		}

		if b == '\\' {
			next, err := dec.buf.ReadByte()
			if err != nil {
				return "", err
			}
			dec.offset++

			switch next {
			case '"', '\\', '/':
				buf.WriteByte(next)
			case 'b':
				buf.WriteByte('\b')
			case 'f':
				buf.WriteByte('\f')
			case 'n':
				buf.WriteByte('\n')
			case 'r':
				buf.WriteByte('\r')
			case 't':
				buf.WriteByte('\t')
			case 'u':
				var hex [4]byte
				for i := range 4 {
					hex[i], err = dec.buf.ReadByte()
					if err != nil {
						return "", err
					}
					dec.offset++
				}

				code, err := strconv.ParseUint(string(hex[:]), 16, 16)
				if err != nil {
					return "", err
				}
				r := rune(code)
				// Handle UTF-16 surrogate pairs per RFC 8259 §7
				if utf16.IsSurrogate(r) {
					r, err = dec.parseSurrogatePair(r)
					if err != nil {
						return "", err
					}
				}
				buf.WriteRune(r)
			default:
				return "", &SyntaxError{
					msg:    fmt.Sprintf("invalid escape sequence '\\%c'", next),
					Offset: dec.offset - 2,
				}
			}
		} else {
			buf.WriteByte(b)
		}
	}
}

// parseSurrogatePair handles UTF-16 surrogate pair decoding per RFC 8259 section 7.
// When a high surrogate (U+D800-U+DBFF) is encountered, it reads the expected
// low surrogate (\uDC00-\uDFFF) and returns the decoded Unicode code point.
func (dec *Decoder) parseSurrogatePair(high rune) (rune, error) {
	if high < 0xD800 || high > 0xDBFF {
		return unicode.ReplacementChar, &SyntaxError{
			msg:    "invalid UTF-16 high surrogate",
			Offset: dec.offset,
		}
	}

	// Expect \u followed by low surrogate
	b, err := dec.buf.ReadByte()
	if err != nil || b != '\\' {
		return unicode.ReplacementChar, &SyntaxError{
			msg:    "expected \\u low surrogate in surrogate pair",
			Offset: dec.offset,
		}
	}
	dec.offset++

	b, err = dec.buf.ReadByte()
	if err != nil || b != 'u' {
		return unicode.ReplacementChar, &SyntaxError{
			msg:    "expected \\u low surrogate after backslash",
			Offset: dec.offset,
		}
	}
	dec.offset++

	var hex [4]byte
	for i := range 4 {
		hex[i], err = dec.buf.ReadByte()
		if err != nil {
			return unicode.ReplacementChar, err
		}
		dec.offset++
	}

	code, err := strconv.ParseUint(string(hex[:]), 16, 16)
	if err != nil {
		return unicode.ReplacementChar, &SyntaxError{
			msg:    "invalid hex in surrogate pair",
			Offset: dec.offset - 4,
		}
	}

	low := rune(code)
	if low < 0xDC00 || low > 0xDFFF {
		return unicode.ReplacementChar, &SyntaxError{
			msg:    "invalid low surrogate in surrogate pair",
			Offset: dec.offset - 4,
		}
	}

	return utf16.DecodeRune(high, low), nil
}

func (dec *Decoder) parseBoolean(first byte) (bool, error) {
	if first == 't' {
		expected := "rue"
		for i, expected_char := range expected {
			b, err := dec.buf.ReadByte()
			if err != nil {
				return false, err
			}
			dec.offset++
			if b != byte(expected_char) {
				return false, &SyntaxError{
					msg:    fmt.Sprintf("invalid character '%c' in literal true (expecting '%c')", b, expected_char),
					Offset: dec.offset - int64(i) - 2,
				}
			}
		}
		return true, nil
	}

	expected := "alse"
	for i, expected_char := range expected {
		b, err := dec.buf.ReadByte()
		if err != nil {
			return false, err
		}
		dec.offset++
		if b != byte(expected_char) {
			return false, &SyntaxError{
				msg:    fmt.Sprintf("invalid character '%c' in literal false (expecting '%c')", b, expected_char),
				Offset: dec.offset - int64(i) - 2,
			}
		}
	}
	return false, nil
}

func (dec *Decoder) parseNull() (any, error) {
	expected := "ull"
	for i, expectedChar := range expected {
		b, err := dec.buf.ReadByte()
		if err != nil {
			return nil, err
		}
		dec.offset++
		if b != byte(expectedChar) {
			return nil, &SyntaxError{
				msg:    fmt.Sprintf("invalid character '%c' in literal null (expecting '%c')", b, expectedChar),
				Offset: dec.offset - int64(i) - 2,
			}
		}
	}
	return nil, nil
}

func (dec *Decoder) parseNumber(first byte) (any, error) {
	buf := getEncoderBuffer()
	defer putEncoderBuffer(buf)
	buf.WriteByte(first)

	for {
		b, err := dec.buf.Peek(1)
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}

		if !isDigit(b[0]) && b[0] != '.' && b[0] != 'e' && b[0] != 'E' && b[0] != '+' && b[0] != '-' {
			break
		}

		// FIX: Check error from ReadByte to prevent data corruption
		actual, readErr := dec.buf.ReadByte()
		if readErr != nil {
			return nil, fmt.Errorf("failed to read number character: %w", readErr)
		}
		dec.offset++
		buf.WriteByte(actual)
	}

	numStr := buf.String()

	if dec.useNumber {
		return Number(numStr), nil
	}

	if strings.IndexByte(numStr, '.') < 0 && strings.IndexByte(numStr, 'e') < 0 && strings.IndexByte(numStr, 'E') < 0 {
		if val, err := strconv.ParseInt(numStr, 10, 64); err == nil {
			return val, nil
		}
	}

	// Parse as float64
	val, err := strconv.ParseFloat(numStr, 64)
	if err != nil {
		return nil, &SyntaxError{
			msg:    fmt.Sprintf("invalid number: %s", numStr),
			Offset: dec.offset - int64(len(numStr)),
		}
	}

	return val, nil
}

// validateDepth checks if the data structure exceeds maximum depth
func (p *Processor) validateDepth(value any, maxDepth, currentDepth int) error {
	if currentDepth > maxDepth {
		return &JsonsError{
			Op:      "validate_depth",
			Message: fmt.Sprintf("data structure depth %d exceeds maximum %d", currentDepth, maxDepth),
			Err:     errOperationFailed,
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

// needsCustomEncodingOpts checks if the encoding options require custom encoding logic
// Note: Go std lib json.Marshal escapes HTML by default since Go 1.13,
// so we only need custom encoding when EscapeHTML is explicitly set to false.
func needsCustomEncodingOpts(cfg Config) bool {
	return cfg.DisableEscaping ||
		cfg.EscapeUnicode ||
		cfg.EscapeSlash ||
		!cfg.EscapeNewlines || // When false, need custom encoding to NOT escape
		!cfg.EscapeTabs || // When false, need custom encoding to NOT escape
		cfg.CustomEscapes != nil ||
		cfg.SortKeys ||
		!cfg.EscapeHTML || // When false, need custom encoding to NOT escape (std lib escapes by default)
		cfg.FloatPrecision >= 0 ||
		!cfg.IncludeNulls
}

// Marshal converts any Go value to JSON bytes (similar to json.Marshal)
// PERFORMANCE: Uses FastEncoder for simple types to avoid reflection overhead.
// Uses encodeWithConfigToBytes for complex types to avoid string round-trip.
func (p *Processor) Marshal(value any, cfg ...Config) ([]byte, error) {
	if err := p.checkClosed(); err != nil {
		return nil, err
	}

	// PERFORMANCE: Fast path for simple types - avoid config processing overhead
	// Uses HTML escaping to match encoding/json behavior
	// Encodes directly to []byte to avoid string round-trip
	if len(cfg) == 0 {
		if result, ok := fastEncodeSimpleToBytes(value); ok {
			return result, nil
		}
	}

	// Fallback: encode directly to []byte, avoiding []byte->string->[]byte round-trip
	config := getConfigOrDefault(cfg...)
	config.EscapeHTML = true
	return p.encodeWithConfigToBytes(value, config)
}

// MarshalIndent converts any Go value to indented JSON bytes (similar to json.MarshalIndent)
func (p *Processor) MarshalIndent(value any, prefix, indent string, cfg ...Config) ([]byte, error) {
	encOpts := DefaultConfig()
	if len(cfg) > 0 {
		encOpts = cfg[0]
	}
	encOpts.Pretty = true
	encOpts.Prefix = prefix
	encOpts.Indent = indent

	// PERFORMANCE: Encode directly to []byte, avoiding []byte->string->[]byte round-trip
	return p.encodeWithConfigToBytes(value, encOpts)
}

// Unmarshal parses the JSON-encoded data and stores the result in the value pointed to by v.
// This method is fully compatible with encoding/json.Unmarshal.
// PERFORMANCE: Fast path for simple cases to avoid string conversion overhead.
func (p *Processor) Unmarshal(data []byte, value any, cfg ...Config) error {
	if err := p.checkClosed(); err != nil {
		return err
	}

	if value == nil {
		return &InvalidUnmarshalError{Type: nil}
	}

	// PERFORMANCE: Fast path when no options are provided
	// Use encoding/json directly to avoid string conversion overhead
	if len(cfg) == 0 {
		return json.Unmarshal(data, value)
	}

	// Slow path for options: convert to string for internal processing
	jsonStr := string(data)

	// Use the existing Parse method which handles all the validation and parsing logic
	return p.Parse(jsonStr, value, cfg...)
}

// EncodeStream encodes multiple values as a JSON array stream.
// This method accepts variadic Config for unified API pattern.
//
// Example:
//
//	result, err := processor.EncodeStream(values, json.PrettyConfig())
func (p *Processor) EncodeStream(values any, cfg ...Config) (string, error) {
	if err := p.checkClosed(); err != nil {
		return "", err
	}
	config := getConfigOrDefault(cfg...)
	return p.EncodeWithConfig(values, config)
}

// EncodeBatch encodes multiple key-value pairs as a JSON object.
// This method accepts variadic Config for unified API pattern.
//
// Example:
//
//	result, err := processor.EncodeBatch(pairs, json.PrettyConfig())
func (p *Processor) EncodeBatch(pairs map[string]any, cfg ...Config) (string, error) {
	if err := p.checkClosed(); err != nil {
		return "", err
	}
	config := getConfigOrDefault(cfg...)
	return p.EncodeWithConfig(pairs, config)
}

// EncodeFields encodes struct fields selectively based on field names.
// This method accepts variadic Config for unified API pattern.
//
// Example:
//
//	result, err := processor.EncodeFields(value, []string{"name", "email"}, json.PrettyConfig())
func (p *Processor) EncodeFields(value any, fields []string, cfg ...Config) (string, error) {
	if err := p.checkClosed(); err != nil {
		return "", err
	}
	processor := p

	// First convert to JSON and parse back to get map representation
	config := DefaultConfig()
	config.Pretty = false
	tempJSON, err := processor.EncodeWithConfig(value, config)
	if err != nil {
		return "", err
	}

	// Parse to any and convert to map
	var anyData any
	err = processor.Parse(tempJSON, &anyData)
	if err != nil {
		return "", err
	}

	// Check if the result is actually a map
	data, ok := anyData.(map[string]any)
	if !ok {
		return "", &JsonsError{
			Op:      "encode_fields",
			Message: "value is not an object, cannot filter fields",
			Err:     ErrTypeMismatch,
		}
	}

	// Filter fields
	filtered := make(map[string]any, len(fields))
	for _, field := range fields {
		if val, exists := data[field]; exists {
			filtered[field] = val
		}
	}

	finalConfig := DefaultConfig()
	if len(cfg) > 0 {
		finalConfig = cfg[0]
	}
	return processor.EncodeWithConfig(filtered, finalConfig)
}

// EncodeWithConfig converts any Go value to JSON string with full configuration control.
// PERFORMANCE: Uses FastEncoder for simple types to avoid reflection overhead.
//
// Example:
//
//	// Default configuration
//	result, err := processor.EncodeWithConfig(data)
//
//	// With custom configuration
//	cfg := json.DefaultConfig()
//	cfg.Pretty = true
//	result, err := processor.EncodeWithConfig(data, cfg)
//
//	// With preset configuration
//	result, err := processor.EncodeWithConfig(data, json.PrettyConfig())
func (p *Processor) EncodeWithConfig(value any, cfg ...Config) (string, error) {
	b, err := p.encodeWithConfigToBytes(value, cfg...)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// encodeWithConfigToBytes encodes value to []byte directly, avoiding string round-trip.
// PERFORMANCE: Used by Marshal/MarshalIndent to eliminate []byte->string->[]byte conversion.
func (p *Processor) encodeWithConfigToBytes(value any, cfg ...Config) ([]byte, error) {
	if err := p.checkClosed(); err != nil {
		return nil, err
	}

	var config Config
	if len(cfg) > 0 {
		config = cfg[0]
	} else {
		config = DefaultConfig()
	}

	// Fast path for simple types
	if !config.Pretty && !needsCustomEncodingOpts(config) {
		if config.EscapeHTML {
			if result, ok := fastEncodeSimpleWithHTMLEscape(value); ok {
				if int64(len(result)) > p.config.MaxJSONSize {
					return nil, &JsonsError{
						Op:      "encode_with_config",
						Message: fmt.Sprintf("encoded JSON size %d exceeds maximum %d", len(result), p.config.MaxJSONSize),
						Err:     ErrSizeLimit,
					}
				}
				return []byte(result), nil
			}
		} else {
			if result, ok := fastEncodeSimple(value); ok {
				if int64(len(result)) > p.config.MaxJSONSize {
					return nil, &JsonsError{
						Op:      "encode_with_config",
						Message: fmt.Sprintf("encoded JSON size %d exceeds maximum %d", len(result), p.config.MaxJSONSize),
						Err:     ErrSizeLimit,
					}
				}
				return []byte(result), nil
			}
		}
	}

	if config.MaxDepth > 0 {
		if err := p.validateDepth(value, config.MaxDepth, 0); err != nil {
			return nil, err
		}
	}

	var result []byte
	var err error

	if needsCustomEncodingOpts(config) {
		encoder := newCustomEncoder(config)
		defer encoder.Close()
		result, err = encoder.EncodeToBytes(value)
	} else {
		result, err = internal.MarshalJSONToBytes(value, config.Pretty, config.Prefix, config.Indent)
	}

	if err != nil {
		return nil, &JsonsError{
			Op:      "encode_with_config",
			Message: "failed to encode value",
			Err:     err,
		}
	}

	if int64(len(result)) > p.config.MaxJSONSize {
		return nil, &JsonsError{
			Op:      "encode_with_config",
			Message: fmt.Sprintf("encoded JSON size %d exceeds maximum %d", len(result), p.config.MaxJSONSize),
			Err:     ErrSizeLimit,
		}
	}

	return result, nil
}

// fastEncodeSimple attempts to encode simple types using FastEncoder
// Returns (result, true) if successful, ("", false) if type not supported
// PERFORMANCE: Avoids reflection overhead for common types
// NOTE: This does NOT escape HTML characters - use only when HTML escaping is not needed
func fastEncodeSimple(value any) (string, bool) {
	encoder := internal.GetEncoder()
	defer internal.PutEncoder(encoder)

	err := encoder.EncodeValue(value)
	if err != nil {
		return "", false
	}

	return string(encoder.Bytes()), true
}

// fastEncodeSimpleWithHTMLEscape encodes simple types with HTML escaping
// Returns (result, true) if successful, ("", false) if type not supported
// PERFORMANCE v3: Direct byte-level HTML escaping to minimize allocations
func fastEncodeSimpleWithHTMLEscape(value any) (string, bool) {
	encoder := internal.GetEncoder()
	defer internal.PutEncoder(encoder)

	err := encoder.EncodeValue(value)
	if err != nil {
		return "", false
	}

	// PERFORMANCE: Work directly with bytes to avoid string conversions
	data := encoder.Bytes()
	if internal.NeedsHTMLEscapeBytes(data) {
		escaped := internal.HTMLEscapeBytes(data)
		result := string(escaped)
		internal.PutHTMLEscapeBytes(escaped)
		return result, true
	}

	return string(data), true
}

// fastEncodeSimpleToBytes encodes directly to []byte, avoiding the
// []byte -> string -> []byte round-trip when Marshal needs bytes.
// PERFORMANCE v2: Uses append(nil, data...) which is optimized by the compiler
// into a single allocation (runtime.memmove) without the explicit make+copy.
func fastEncodeSimpleToBytes(value any) ([]byte, bool) {
	encoder := internal.GetEncoder()
	defer internal.PutEncoder(encoder)

	err := encoder.EncodeValue(value)
	if err != nil {
		return nil, false
	}

	data := encoder.Bytes()
	if internal.NeedsHTMLEscapeBytes(data) {
		escaped := internal.HTMLEscapeBytes(data)
		// escaped is already a fresh []byte, return directly
		return escaped, true
	}

	// Use append to clone — compiler optimizes append(nil, data...) to a
	// single alloc+copy without the separate make call overhead.
	return append([]byte(nil), data...), true
}

// Encode converts any Go value to JSON string
// This is a convenience method that matches the package-level Encode signature
func (p *Processor) Encode(value any, config ...Config) (string, error) {
	var cfg Config
	if len(config) > 0 {
		cfg = config[0]
	} else {
		cfg = DefaultConfig()
	}
	return p.EncodeWithConfig(value, cfg)
}

// EncodePretty converts any Go value to pretty-formatted JSON string
// This is a convenience method that matches the package-level EncodePretty signature
func (p *Processor) EncodePretty(value any, config ...Config) (string, error) {
	var cfg Config
	if len(config) > 0 {
		cfg = config[0]
	} else {
		cfg = PrettyConfig()
	}
	return p.EncodeWithConfig(value, cfg)
}

// customEncoder provides advanced JSON encoding with configurable options
type customEncoder struct {
	config *Config
	buffer *bytes.Buffer
	depth  int
}

// newCustomEncoder creates a new custom encoder with the given configuration
func newCustomEncoder(config Config) *customEncoder {
	return &customEncoder{
		config: &config,
		buffer: getEncoderBuffer(),
		depth:  0,
	}
}

// Close releases the encoder's buffers back to the pool
func (e *customEncoder) Close() {
	if e.buffer != nil {
		putEncoderBuffer(e.buffer)
		e.buffer = nil
	}
}

// Encode encodes the given value to JSON string using custom options
func (e *customEncoder) Encode(value any) (string, error) {
	e.buffer.Reset()
	e.depth = 0

	if err := e.encodeValue(value); err != nil {
		return "", err
	}

	return e.buffer.String(), nil
}

// EncodeToBytes encodes the given value to JSON bytes using custom options.
// PERFORMANCE: Returns []byte directly to avoid string round-trip when caller needs bytes.
func (e *customEncoder) EncodeToBytes(value any) ([]byte, error) {
	e.buffer.Reset()
	e.depth = 0

	if err := e.encodeValue(value); err != nil {
		return nil, err
	}

	// Copy buffer contents to avoid aliasing with pooled buffer
	result := make([]byte, e.buffer.Len())
	copy(result, e.buffer.Bytes())
	return result, nil
}

// encodeValue encodes any value recursively
func (e *customEncoder) encodeValue(value any) error {
	if e.config.MaxDepth > 0 && e.depth > e.config.MaxDepth {
		return &JsonsError{
			Op:      "custom_encode",
			Message: fmt.Sprintf("encoding depth %d exceeds maximum %d", e.depth, e.config.MaxDepth),
			Err:     ErrDepthLimit,
		}
	}

	if value == nil {
		e.buffer.WriteString("null")
		return nil
	}

	v := reflect.ValueOf(value)

	// Handle pointers
	for v.Kind() == reflect.Pointer {
		if v.IsNil() {
			e.buffer.WriteString("null")
			return nil
		}
		v = v.Elem()
	}

	// Check if the value implements json.Marshaler interface first
	if marshaler, ok := value.(marshaler); ok {
		data, err := marshaler.MarshalJSON()
		if err != nil {
			return &MarshalerError{
				Type:       reflect.TypeOf(value),
				Err:        err,
				sourceFunc: "MarshalJSON",
			}
		}
		e.buffer.Write(data)
		return nil
	}

	// Check if the value implements encoding.TextMarshaler interface
	if textMarshaler, ok := value.(encoding.TextMarshaler); ok {
		text, err := textMarshaler.MarshalText()
		if err != nil {
			return &MarshalerError{
				Type:       reflect.TypeOf(value),
				Err:        err,
				sourceFunc: "MarshalText",
			}
		}
		return e.encodeString(string(text))
	}

	// Handle json.Number type specially to preserve original format
	if jsonNum, ok := value.(json.Number); ok {
		return e.encodeJSONNumber(jsonNum)
	}

	// Handle time.Time type specially to convert to RFC3339 string
	if timeVal, ok := value.(time.Time); ok {
		return e.encodeString(timeVal.Format(time.RFC3339))
	}
	if timeVal, ok := v.Interface().(time.Time); ok {
		return e.encodeString(timeVal.Format(time.RFC3339))
	}

	switch v.Kind() {
	case reflect.Bool:
		if v.Bool() {
			e.buffer.WriteString("true")
		} else {
			e.buffer.WriteString("false")
		}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		e.buffer.WriteString(strconv.FormatInt(v.Int(), 10))
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		e.buffer.WriteString(strconv.FormatUint(v.Uint(), 10))
	case reflect.Float32, reflect.Float64:
		return e.encodeFloat(v.Float(), v.Type().Bits())
	case reflect.String:
		return e.encodeString(v.String())
	case reflect.Array, reflect.Slice:
		return e.encodeArray(v)
	case reflect.Map:
		return e.encodeMap(v)
	case reflect.Struct:
		return e.encodeStruct(v)
	case reflect.Interface:
		return e.encodeValue(v.Interface())
	default:
		// Fallback to standard JSON encoding
		data, err := json.Marshal(value)
		if err != nil {
			return err
		}
		e.buffer.Write(data)
	}

	return nil
}

// encodeJSONNumber encodes json.Number while preserving original format
func (e *customEncoder) encodeJSONNumber(num json.Number) error {
	numStr := string(num)

	// If PreserveNumbers is enabled, keep the original string representation
	if e.config.PreserveNumbers {
		e.buffer.WriteString(numStr)
		return nil
	}

	// Otherwise, try to convert to appropriate Go type
	// Check if it's an integer (no decimal point and no scientific notation)
	if !strings.Contains(numStr, ".") && !strings.ContainsAny(numStr, "eE") {
		// Integer format
		if i, err := num.Int64(); err == nil {
			e.buffer.WriteString(strconv.FormatInt(i, 10))
			return nil
		}
	}

	// Float format
	if f, err := num.Float64(); err == nil {
		return e.encodeFloat(f, 64)
	}

	// Fallback: use original string
	e.buffer.WriteString(numStr)
	return nil
}

func (e *customEncoder) encodeFloat(f float64, bits int) error {
	if e.config.FloatPrecision >= 0 {
		if e.config.FloatTruncate {
			// Truncate mode: format with higher precision then truncate
			formatted := e.truncateFloat(f, e.config.FloatPrecision, bits)
			e.buffer.WriteString(formatted)
			return nil
		}
		// Default: round using standard FormatFloat
		formatted := strconv.FormatFloat(f, 'f', e.config.FloatPrecision, bits)
		e.buffer.WriteString(formatted)
		return nil
	}

	if f >= -1e15 && f <= 1e15 {
		formatted := strconv.FormatFloat(f, 'f', -1, bits)
		e.buffer.WriteString(formatted)
	} else {
		formatted := strconv.FormatFloat(f, 'g', -1, bits)
		e.buffer.WriteString(formatted)
	}

	return nil
}

// truncateFloat truncates a float to the specified precision without rounding
func (e *customEncoder) truncateFloat(f float64, precision int, bits int) string {
	// Format with high precision to get all digits
	formatted := strconv.FormatFloat(f, 'f', 20, bits)

	// Find decimal point
	dotIdx := strings.Index(formatted, ".")
	if dotIdx == -1 {
		// No decimal point, add trailing zeros if precision > 0
		if precision > 0 {
			return formatted + "." + strings.Repeat("0", precision)
		}
		return formatted
	}

	// Calculate how many digits after decimal
	afterDot := len(formatted) - dotIdx - 1

	if precision == 0 {
		// Return only integer part
		return formatted[:dotIdx]
	}

	if afterDot <= precision {
		// Need to pad with zeros
		return formatted + strings.Repeat("0", precision-afterDot)
	}

	// Truncate to desired precision (simply cut off extra digits)
	return formatted[:dotIdx+1+precision]
}

func (e *customEncoder) encodeString(s string) error {
	e.buffer.WriteByte('"')

	if e.config.DisableEscaping {
		for i := 0; i < len(s); i++ {
			b := s[i]
			switch b {
			case '"':
				e.buffer.WriteString(`\"`)
			case '\\':
				e.buffer.WriteString(`\\`)
			default:
				if b < 0x80 {
					e.buffer.WriteByte(b)
				} else {
					r, size := utf8.DecodeRuneInString(s[i:])
					e.buffer.WriteRune(r)
					i += size - 1
				}
			}
		}
	} else {
		for _, r := range s {
			if err := e.escapeRune(r); err != nil {
				return err
			}
		}
	}

	e.buffer.WriteByte('"')
	return nil
}

// hexDigits is used for fast Unicode escape encoding without fmt.Fprintf
var hexDigits = [16]byte{'0', '1', '2', '3', '4', '5', '6', '7', '8', '9', 'a', 'b', 'c', 'd', 'e', 'f'}

// writeUnicodeEscape writes \uXXXX directly to buffer, avoiding fmt.Fprintf overhead.
// PERFORMANCE: ~10-50x faster than fmt.Fprintf for Unicode escapes.
func (e *customEncoder) writeUnicodeEscape(r rune) {
	var buf [6]byte
	buf[0] = '\\'
	buf[1] = 'u'
	buf[2] = hexDigits[(r>>12)&0xF]
	buf[3] = hexDigits[(r>>8)&0xF]
	buf[4] = hexDigits[(r>>4)&0xF]
	buf[5] = hexDigits[r&0xF]
	e.buffer.Write(buf[:])
}

func (e *customEncoder) escapeRune(r rune) error {
	if e.config.CustomEscapes != nil {
		if escape, exists := e.config.CustomEscapes[r]; exists {
			e.buffer.WriteString(escape)
			return nil
		}
	}

	switch r {
	case '"':
		e.buffer.WriteString(`\"`)
	case '\\':
		e.buffer.WriteString(`\\`)
	case '\b':
		e.buffer.WriteString(`\b`)
	case '\f':
		e.buffer.WriteString(`\f`)
	case '\n':
		if e.config.EscapeNewlines {
			e.buffer.WriteString(`\n`)
		} else {
			e.buffer.WriteRune(r)
		}
	case '\r':
		e.buffer.WriteString(`\r`)
	case '\t':
		if e.config.EscapeTabs {
			e.buffer.WriteString(`\t`)
		} else {
			e.buffer.WriteRune(r)
		}
	case '/':
		if e.config.EscapeSlash {
			e.buffer.WriteString(`\/`)
		} else {
			e.buffer.WriteRune(r)
		}
	default:
		if r < 0x20 {
			e.writeUnicodeEscape(r)
		} else if e.config.EscapeHTML && (r == '<' || r == '>' || r == '&') {
			e.writeUnicodeEscape(r)
		} else if e.config.EscapeUnicode && r > 0x7F {
			e.writeUnicodeEscape(r)
		} else if !utf8.ValidRune(r) && e.config.ValidateUTF8 {
			return &JsonsError{
				Op:      "escape_rune",
				Message: fmt.Sprintf("invalid UTF-8 rune: %U", r),
				Err:     errOperationFailed,
			}
		} else {
			e.buffer.WriteRune(r)
		}
	}

	return nil
}

func (e *customEncoder) encodeArray(v reflect.Value) error {
	e.buffer.WriteByte('[')
	e.depth++

	length := v.Len()
	for i := 0; i < length; i++ {
		if i > 0 {
			e.buffer.WriteByte(',')
		}

		if e.config.Pretty {
			e.writeIndent()
		}

		if err := e.encodeValue(v.Index(i).Interface()); err != nil {
			return err
		}
	}

	e.depth--
	if e.config.Pretty && length > 0 {
		e.writeIndent()
	}
	e.buffer.WriteByte(']')

	return nil
}

func (e *customEncoder) encodeMap(v reflect.Value) error {
	e.buffer.WriteByte('{')
	e.depth++

	keys := v.MapKeys()
	if e.config.SortKeys {
		sort.Slice(keys, func(i, j int) bool {
			return keys[i].String() < keys[j].String()
		})
	}

	first := true
	for _, key := range keys {
		value := v.MapIndex(key)

		if !e.config.IncludeNulls && (value.Interface() == nil || (value.Kind() == reflect.Pointer && value.IsNil())) {
			continue
		}

		if !first {
			e.buffer.WriteByte(',')
		}
		first = false

		if e.config.Pretty {
			e.writeIndent()
		}

		if err := e.encodeString(key.String()); err != nil {
			return err
		}

		e.buffer.WriteByte(':')
		if e.config.Pretty {
			e.buffer.WriteByte(' ')
		}

		if err := e.encodeValue(value.Interface()); err != nil {
			return err
		}
	}

	e.depth--
	if e.config.Pretty && len(keys) > 0 {
		e.writeIndent()
	}
	e.buffer.WriteByte('}')

	return nil
}

func (e *customEncoder) encodeStruct(v reflect.Value) error {
	// Use custom encoding when any of these advanced features are enabled
	if !e.config.IncludeNulls || e.config.SortKeys || !e.config.EscapeHTML ||
		e.config.FloatPrecision >= 0 || !e.config.EscapeNewlines || !e.config.EscapeTabs ||
		e.config.EscapeSlash || e.config.EscapeUnicode {
		return e.encodeStructCustom(v)
	}

	if e.config.Pretty {
		data, err := json.MarshalIndent(v.Interface(), e.config.Prefix, e.config.Indent)
		if err != nil {
			return err
		}
		e.buffer.Write(data)
		return nil
	}

	data, err := json.Marshal(v.Interface())
	if err != nil {
		return err
	}
	e.buffer.Write(data)
	return nil
}

func (e *customEncoder) encodeStructCustom(v reflect.Value) error {
	e.buffer.WriteByte('{')
	e.depth++

	t := v.Type()
	var fields []reflect.StructField
	var fieldValues []reflect.Value

	for i := 0; i < v.NumField(); i++ {
		field := t.Field(i)
		fieldValue := v.Field(i)

		if !field.IsExported() {
			continue
		}

		jsonTag := field.Tag.Get("json")
		if jsonTag == "-" {
			continue
		}

		tagParts := strings.Split(jsonTag, ",")

		hasOmitEmpty := slices.Contains(tagParts[1:], "omitempty")

		shouldSkip := false

		// Only respect struct omitempty tags for empty field handling
		if hasOmitEmpty && e.isEmpty(fieldValue) {
			shouldSkip = true
		}

		if !e.config.IncludeNulls {
			if fieldValue.Interface() == nil || (fieldValue.Kind() == reflect.Pointer && fieldValue.IsNil()) {
				shouldSkip = true
			}
		}

		if !shouldSkip {
			fields = append(fields, field)
			fieldValues = append(fieldValues, fieldValue)
		}
	}

	if e.config.SortKeys {
		indices := make([]int, len(fields))
		for i := range indices {
			indices[i] = i
		}

		sort.Slice(indices, func(i, j int) bool {
			nameI := fields[indices[i]].Name
			nameJ := fields[indices[j]].Name

			if tag := fields[indices[i]].Tag.Get("json"); tag != "" && tag != "-" {
				if tagParts := strings.Split(tag, ","); len(tagParts) > 0 && tagParts[0] != "" {
					nameI = tagParts[0]
				}
			}
			if tag := fields[indices[j]].Tag.Get("json"); tag != "" && tag != "-" {
				if tagParts := strings.Split(tag, ","); len(tagParts) > 0 && tagParts[0] != "" {
					nameJ = tagParts[0]
				}
			}

			return nameI < nameJ
		})

		sortedFields := make([]reflect.StructField, len(fields))
		sortedValues := make([]reflect.Value, len(fieldValues))
		for i, idx := range indices {
			sortedFields[i] = fields[idx]
			sortedValues[i] = fieldValues[idx]
		}
		fields = sortedFields
		fieldValues = sortedValues
	}

	for i, field := range fields {
		fieldValue := fieldValues[i]

		if i > 0 {
			e.buffer.WriteByte(',')
		}

		if e.config.Pretty {
			e.writeIndent()
		}

		jsonTag := field.Tag.Get("json")
		fieldName := field.Name
		if jsonTag != "" && jsonTag != "-" {
			if tagParts := strings.Split(jsonTag, ","); len(tagParts) > 0 && tagParts[0] != "" {
				fieldName = tagParts[0]
			}
		}

		if err := e.encodeString(fieldName); err != nil {
			return err
		}

		e.buffer.WriteByte(':')
		if e.config.Pretty {
			e.buffer.WriteByte(' ')
		}

		if err := e.encodeValue(fieldValue.Interface()); err != nil {
			return err
		}
	}

	e.depth--
	if e.config.Pretty && len(fields) > 0 {
		e.writeIndent()
	}
	e.buffer.WriteByte('}')

	return nil
}

func (e *customEncoder) writeIndent() {
	e.buffer.WriteByte('\n')
	e.buffer.WriteString(e.config.Prefix)
	for i := 0; i < e.depth; i++ {
		e.buffer.WriteString(e.config.Indent)
	}
}

func (e *customEncoder) isEmpty(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.Array, reflect.Map, reflect.Slice, reflect.String:
		return v.Len() == 0
	case reflect.Bool:
		return !v.Bool()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return v.Int() == 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return v.Uint() == 0
	case reflect.Float32, reflect.Float64:
		return v.Float() == 0
	case reflect.Interface, reflect.Pointer:
		return v.IsNil()
	case reflect.Chan, reflect.Func, reflect.UnsafePointer:
		return v.IsNil()
	}
	return false
}
