package json

import (
	"bufio"
	"bytes"
	"encoding"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"reflect"
	"slices"
	"sort"
	"strconv"
	"strings"
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

// MarshalJSON emits the number literal unchanged (unquoted), matching how
// encoding/json treats its own json.Number. Without this method every
// stdlib-marshal path — Marshal/Encode without cfg, the fast encoder's
// fallback, internal.MarshalJSONToBytes — serialized Number as a QUOTED
// string ("1.10"), corrupting round-trips for data parsed with
// PreserveNumbers and later re-encoded without a per-call Config.
//
// Config-less paths therefore honor the literal verbatim; the custom encoder
// (which has config context) still normalizes when PreserveNumbers=false —
// the same fast-vs-custom split stdlib json.Number already exhibits
// (json.Number("1e3") encodes as 1e3 on the fast path, 1000 via the custom
// encoder). An invalid literal is rejected rather than written verbatim.
func (n Number) MarshalJSON() ([]byte, error) {
	if !internal.IsValidJSONNumber(string(n)) {
		return nil, fmt.Errorf("json: invalid number literal %q", string(n))
	}
	return []byte(n), nil
}

// UnmarshalJSON stores the raw number literal, matching encoding/json's
// special-casing of its own json.Number: decoding `42` into a *Number
// succeeds without any UseNumber-style setup. Previously both
// Unmarshal(data, &n) and Decoder.Decode(&n) failed with "cannot unmarshal
// number into Go value of type json.Number", breaking the drop-in
// replacement for the standard `var n json.Number; dec.Decode(&n)` pattern
// (D-002). JSON null is a no-op (zero Number), per the Unmarshaler contract.
func (n *Number) UnmarshalJSON(b []byte) error {
	s := string(b)
	if s == "null" {
		*n = ""
		return nil
	}
	if !internal.IsValidJSONNumber(s) {
		return fmt.Errorf("json: cannot unmarshal %s into Go value of type json.Number", jsonKindOfLiteral(b))
	}
	*n = Number(s)
	return nil
}

// jsonKindOfLiteral names the JSON kind of a raw literal for error messages,
// mirroring encoding/json's UnmarshalTypeError wording.
func jsonKindOfLiteral(b []byte) string {
	if len(b) == 0 {
		return "value"
	}
	switch b[0] {
	case '"':
		return "string"
	case '{':
		return "object"
	case '[':
		return "array"
	case 't', 'f':
		return "bool"
	case 'n':
		return "null"
	default:
		return "number"
	}
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
//
// Errors:
//   - UnsupportedTypeError: v contains a value of an unsupported type
//   - UnsupportedValueError: v contains a value that cannot be represented in JSON
//   - MarshalerError: a MarshalJSON/MarshalText method returned an error
//   - ErrSizeLimit: the encoded output exceeds MaxJSONSize
//   - any error returned while writing to the underlying stream
func (enc *Encoder) Encode(v any) error {
	// Reuse the shared package-level newline slice instead of allocating
	// []byte{'\n'} on every Encode call (it is passed to io.Writer.Write).
	newline := jsonNewline
	// Get the current processor on each Encode call to avoid stale references
	// after SetGlobalProcessor or ShutdownGlobalProcessor.
	processor := getDefaultProcessor()
	if processor == nil {
		return errInternalError
	}

	// PERFORMANCE: Fast path for simple types with no custom encoding
	// Avoids Config creation and Encode overhead for common cases.
	// D-002: the fast encoder honors NONE of the processor's encoding
	// options (FloatPrecision, CustomEscapes, EscapeUnicode, ...), so it may
	// only run when the processor's config leaves every such option at its
	// default — otherwise a FloatPrecision=2 processor emitted full precision
	// for simple values and rounded values for complex ones.
	if enc.indent == "" && enc.prefix == "" && encodingFastPathEligible(&processor.config) {
		// Try fast encoder directly
		encoder := internal.GetEncoder()
		if processor.config.MaxDepth > 0 {
			encoder.SetMaxEncodeDepth(processor.config.MaxDepth)
		}
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
	jsonStr, err := processor.Encode(v, config)
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
		// Default the per-value byte cap to DefaultMaxJSONSize so the
		// incremental checks in readStringValue/readContainerValue bound
		// buffering. Without it a hostile stream grows the buffer without
		// limit before the post-read size validation ever runs.
		maxBytes: DefaultMaxJSONSize,
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
//
// Errors:
//   - InvalidUnmarshalError: v is nil or not a non-nil pointer
//   - ErrInvalidJSON: the input is not valid JSON
//   - UnmarshalTypeError: a JSON value does not match the target Go type
//   - ErrProcessorClosed: the default processor has been closed
//   - io.EOF: when there are no more values at the end of the stream
//   - any error returned while reading the stream, or if a single value
//     exceeds the configured MaxJSONSize or nesting-depth limit
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
			// "null" decodes to a nil any; reflect.ValueOf(nil) is the zero
			// Value and Value.Set would panic, so store the target's zero.
			if result == nil {
				rv.Set(reflect.Zero(rv.Type()))
				return nil
			}
			// Only fast-path the assignment when the decoded value actually
			// fits the target. A JSON number decoded into map[string]any must
			// fall through to the stdlib path below, which reports a proper
			// *UnmarshalTypeError instead of panicking in Value.Set.
			if rv.Kind() == reflect.Interface || reflect.TypeOf(result).AssignableTo(rv.Type()) {
				rv.Set(reflect.ValueOf(result))
				return nil
			}
			inner := json.NewDecoder(bytes.NewReader(data))
			inner.UseNumber()
			if dec.disallowUnknownFields {
				inner.DisallowUnknownFields()
			}
			return inner.Decode(v)
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
		// Consume bookkeeping: every other read path advances dec.offset per
		// byte; skipping it here made InputOffset under-report and skewed
		// later SyntaxError offsets by the number of bytes More() swallowed.
		dec.offset++
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
			if err == io.EOF {
				// The opening quote was already consumed (readValue step 1),
				// so EOF here means the string is truncated mid-value — NOT a
				// clean end-of-stream. Returning bare io.EOF would be
				// indistinguishable from "no more values" and make streaming
				// callers silently drop the final value; stdlib returns
				// io.ErrUnexpectedEOF here (D-002).
				return nil, fmt.Errorf("unexpected EOF in JSON string: %w", io.ErrUnexpectedEOF)
			}
			return nil, err
		}
		dec.offset++
		buf.WriteByte(b)
		// Enforce the stream size limit on every byte so an unterminated string
		// cannot grow the buffer without bound (mirrors readContainerValue).
		if dec.maxBytes > 0 && int64(buf.Len()) > dec.maxBytes {
			return nil, fmt.Errorf("streaming value size %d exceeds maximum allowed %d bytes: %w", buf.Len(), dec.maxBytes, ErrSizeLimit)
		}

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

// closeDelimiter returns the closing delimiter matching an opening one.
func closeDelimiter(open byte) byte {
	if open == '[' {
		return ']'
	}
	return '}'
}

// readContainerValue reads a complete JSON object or array.
// openChar is the opening delimiter ('{' or '[') used to validate matching close delimiters.
// Enforces maxNestingDepth to prevent stack exhaustion from deeply nested input.
func (dec *Decoder) readContainerValue(buf *bytes.Buffer, openChar byte) ([]byte, error) {
	maxDepth := dec.maxNestingDepth
	if maxDepth <= 0 {
		maxDepth = DefaultMaxNestingDepth
	}
	inString := false
	escaped := false

	// closers holds the expected closing delimiter for each open container,
	// innermost last. Matching must be per level: deriving a single
	// expectedClose from the root openChar rejects valid mixed nesting such
	// as {"a":[1]}. len(closers) doubles as the current nesting depth.
	closers := make([]byte, 0, 16)
	closers = append(closers, closeDelimiter(openChar))

	for {
		b, err := dec.buf.ReadByte()
		if err != nil {
			if err == io.EOF && buf.Len() > 0 {
				// Wrap with %w so errors.Is(err, io.ErrUnexpectedEOF) matches,
				// consistent with readStringValue's truncation error (D-002).
				return nil, fmt.Errorf("unexpected EOF in JSON container: %w", io.ErrUnexpectedEOF)
			}
			return nil, err
		}
		dec.offset++
		buf.WriteByte(b)
		if dec.maxBytes > 0 && int64(buf.Len()) > dec.maxBytes {
			return nil, fmt.Errorf("streaming value size %d exceeds maximum allowed %d bytes: %w", buf.Len(), dec.maxBytes, ErrSizeLimit)
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
			closers = append(closers, closeDelimiter(b))
			if len(closers) > maxDepth {
				return nil, fmt.Errorf("JSON nesting depth %d exceeds maximum allowed depth %d: %w", len(closers), maxDepth, ErrDepthLimit)
			}
		case '}', ']':
			expected := byte('}')
			if len(closers) > 0 {
				expected = closers[len(closers)-1]
			}
			if b != expected {
				return nil, fmt.Errorf("mismatched JSON delimiters: expected '%c' but got '%c'", expected, b)
			}
			closers = closers[:len(closers)-1]
			if len(closers) == 0 {
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
		// Enforce the stream size limit so a giant unquoted primitive cannot grow
		// the buffer without bound (mirrors readContainerValue / readStringValue).
		if dec.maxBytes > 0 && int64(buf.Len()) > dec.maxBytes {
			return nil, fmt.Errorf("streaming value size %d exceeds maximum allowed %d bytes: %w", buf.Len(), dec.maxBytes, ErrSizeLimit)
		}
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
		// Enforce the stream size limit on every byte so an unterminated
		// string from a never-ending reader cannot grow the pooled buffer
		// without bound (mirrors readStringValue; every other read path in
		// this decoder already checks maxBytes per byte).
		if dec.maxBytes > 0 && int64(buf.Len()) > dec.maxBytes {
			return "", fmt.Errorf("streaming value size %d exceeds maximum allowed %d bytes: %w", buf.Len(), dec.maxBytes, ErrSizeLimit)
		}

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
					// Wrap as SyntaxError like every other parse failure here
					// (the raw *strconv.NumError leaks strconv internals).
					return "", &SyntaxError{
						msg:    fmt.Sprintf("invalid \\u escape %q", string(hex[:])),
						Offset: dec.offset - 4,
					}
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
	if high >= 0xDC00 && high <= 0xDFFF {
		// An unpaired LOW surrogate: the \uXXXX sequence is already fully
		// consumed, so substitute U+FFFD and continue — matching encoding/json,
		// which accepts lone surrogates of either half with a replacement
		// char. (Previously this returned a hard error whose message even
		// said "high surrogate".)
		return unicode.ReplacementChar, nil
	}

	// Expect \u followed by low surrogate
	b, err := dec.buf.ReadByte()
	if err != nil {
		return unicode.ReplacementChar, &SyntaxError{
			msg:    "expected \\u low surrogate in surrogate pair",
			Offset: dec.offset,
		}
	}
	dec.offset++
	if b != '\\' {
		// Lone high surrogate followed by a normal character. encoding/json
		// substitutes U+FFFD and lets the character be processed normally, so
		// unread it and return the replacement rune without an error.
		_ = dec.buf.UnreadByte() // best-effort; decoder is buffered
		dec.offset--
		return unicode.ReplacementChar, nil
	}

	b, err = dec.buf.ReadByte()
	if err != nil {
		return unicode.ReplacementChar, &SyntaxError{
			msg:    "expected \\u low surrogate after backslash",
			Offset: dec.offset,
		}
	}
	dec.offset++
	if b != 'u' {
		_ = dec.buf.UnreadByte() // best-effort; decoder is buffered
		dec.offset--
		return unicode.ReplacementChar, nil
	}

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
		for _, expectedChar := range expected {
			b, err := dec.buf.ReadByte()
			if err != nil {
				return false, err
			}
			dec.offset++
			if b != byte(expectedChar) {
				return false, &SyntaxError{
					msg: fmt.Sprintf("invalid character '%c' in literal true (expecting '%c')", b, expectedChar),
					// Offset includes the offending byte, matching encoding/json.
					Offset: dec.offset,
				}
			}
		}
		return true, nil
	}

	expected := "alse"
	for _, expectedChar := range expected {
		b, err := dec.buf.ReadByte()
		if err != nil {
			return false, err
		}
		dec.offset++
		if b != byte(expectedChar) {
			return false, &SyntaxError{
				msg: fmt.Sprintf("invalid character '%c' in literal false (expecting '%c')", b, expectedChar),
				// Offset includes the offending byte, matching encoding/json.
				Offset: dec.offset,
			}
		}
	}
	return false, nil
}

func (dec *Decoder) parseNull() (any, error) {
	expected := "ull"
	for _, expectedChar := range expected {
		b, err := dec.buf.ReadByte()
		if err != nil {
			return nil, err
		}
		dec.offset++
		if b != byte(expectedChar) {
			return nil, &SyntaxError{
				msg: fmt.Sprintf("invalid character '%c' in literal null (expecting '%c')", b, expectedChar),
				// Offset includes the offending byte, matching encoding/json.
				Offset: dec.offset,
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
		// Enforce the stream size limit on every byte so a hostile reader
		// streaming digits forever cannot grow the buffer without bound
		// (mirrors readStringValue/parseString).
		if dec.maxBytes > 0 && int64(buf.Len()) > dec.maxBytes {
			return nil, fmt.Errorf("streaming value size %d exceeds maximum allowed %d bytes: %w", buf.Len(), dec.maxBytes, ErrSizeLimit)
		}
	}

	numStr := buf.String()

	if dec.useNumber {
		return Number(numStr), nil
	}

	// encoding/json decodes every JSON number into interface{} as float64
	// (and Token() likewise returns float64). The previous ParseInt fast path
	// returned int64 for integer literals, which (a) broke type-switching on
	// Token results (the documented Token contract is float64/Number only) and
	// (b) diverged from the stdlib-backed fast path used by Get/Unmarshal,
	// producing different types for the same input depending on the code path.
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
			Err:     ErrDepthLimit,
		}
	}

	switch v := value.(type) {
	case map[string]any:
		for _, val := range v {
			if err := p.validateDepth(val, maxDepth, currentDepth+1); err != nil {
				return err
			}
		}
		return nil
	case []any:
		for _, val := range v {
			if err := p.validateDepth(val, maxDepth, currentDepth+1); err != nil {
				return err
			}
		}
		return nil
	case map[any]any:
		for _, val := range v {
			if err := p.validateDepth(val, maxDepth, currentDepth+1); err != nil {
				return err
			}
		}
		return nil
	case nil, bool, float64, string, json.Number, Number,
		int, int8, int16, int32, int64,
		uint, uint8, uint16, uint32, uint64, float32:
		// JSON leaf values: no container recursion, skip the reflective walk
		// below (hot path — every leaf of every document passes here).
		return nil
	}

	// D-002 (M17): typed containers (structs, typed slices/maps, pointers)
	// previously fell through here and bypassed the depth cap entirely — a
	// 150-deep struct chain encoded fine while a 150-deep []any chain was
	// rejected, for the same configured MaxDepth. Walk them reflectively,
	// with a pointer-identity visited set so cyclic graphs cannot loop
	// (recursion is additionally bounded by maxDepth).
	return validateDepthReflect(value, maxDepth, currentDepth, nil)
}

// validateDepthReflect walks non-JSON-native container kinds for
// validateDepth. visited tracks in-progress pointers (identity, not deep
// equality) to guard against cyclic structures; a shared map is threaded
// through the walk and only allocated when a pointer is first seen.
func validateDepthReflect(value any, maxDepth, currentDepth int, visited map[any]bool) error {
	if currentDepth > maxDepth {
		return &JsonsError{
			Op:      "validate_depth",
			Message: fmt.Sprintf("data structure depth %d exceeds maximum %d", currentDepth, maxDepth),
			Err:     ErrDepthLimit,
		}
	}

	v := reflect.ValueOf(value)
	switch v.Kind() {
	case reflect.Pointer, reflect.Interface:
		if v.IsNil() {
			return nil
		}
		// Cycle guard: only pointer identities are tracked (interfaces re-box
		// and would never match).
		if v.Kind() == reflect.Pointer {
			key := v.Interface()
			if visited[key] {
				return nil
			}
			if visited == nil {
				visited = make(map[any]bool, 8)
			}
			visited[key] = true
		}
		return validateDepthReflect(v.Elem().Interface(), maxDepth, currentDepth, visited)
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			if err := validateDepthReflect(fieldValue(v, i), maxDepth, currentDepth+1, visited); err != nil {
				return err
			}
		}
	case reflect.Slice, reflect.Array:
		for i := 0; i < v.Len(); i++ {
			if err := validateDepthReflect(v.Index(i).Interface(), maxDepth, currentDepth+1, visited); err != nil {
				return err
			}
		}
	case reflect.Map:
		// Map KEYS cannot be containers in JSON-producing trees; walk values.
		iter := v.MapRange()
		for iter.Next() {
			if err := validateDepthReflect(iter.Value().Interface(), maxDepth, currentDepth+1, visited); err != nil {
				return err
			}
		}
	}
	return nil
}

// fieldValue returns struct field i as an any, tolerating unexported fields
// (they are skipped: they cannot carry JSON-marshaled data without an
// exported accessor, and Interface() would panic on them).
func fieldValue(v reflect.Value, i int) any {
	f := v.Field(i)
	if !f.CanInterface() {
		return nil
	}
	return f.Interface()
}

// needsCustomEncodingOpts checks if the encoding options require custom encoding logic
// Note: Go std lib json.Marshal escapes HTML by default (this behavior predates Go 1.0),
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
		!cfg.IncludeNulls ||
		// PreserveNumbers yields the library's Number type (a string-kind named
		// type). Both fast paths serialize it with plain json.Marshal, which
		// emits a QUOTED string and corrupts the round-trip (1.10 → "1.10").
		// The custom encoder's encodeJSONNumber branch preserves the literal.
		cfg.PreserveNumbers
}

// Marshal converts any Go value to JSON bytes (similar to json.Marshal)
// PERFORMANCE: Uses FastEncoder for simple types to avoid reflection overhead.
// Uses encodeWithConfigToBytes for complex types to avoid string round-trip.
//
// NOTE: like encoding/json.Marshal, the output is always HTML-escaped: a
// supplied cfg.EscapeHTML = false is overridden on this path. Use
// Encode when caller-controlled escaping is required.
//
// Errors:
//   - ErrProcessorClosed: processor has been closed
//   - UnsupportedTypeError / UnsupportedValueError / MarshalerError: value cannot be encoded
//   - ErrSizeLimit: encoded output exceeds MaxJSONSize
//   - ErrDepthLimit: encoding exceeds the maximum nesting depth
func (p *Processor) Marshal(value any, cfg ...Config) ([]byte, error) {
	if err := p.checkClosed(); err != nil {
		return nil, err
	}

	// PERFORMANCE: Fast path for simple types - avoid config processing overhead
	// Uses HTML escaping to match encoding/json behavior
	// Encodes directly to []byte to avoid string round-trip
	if len(cfg) == 0 {
		if result, ok := p.fastEncodeSimpleToBytes(value, p.config.MaxDepth); ok {
			// Same MaxJSONSize enforcement as encodeWithConfigToBytes' fast
			// path — previously this early return skipped it, so Marshal
			// without cfg accepted output the with-cfg form rejects.
			if int64(len(result)) > p.config.MaxJSONSize {
				return nil, &JsonsError{
					Op:      "marshal",
					Message: fmt.Sprintf("encoded JSON size %d exceeds maximum %d", len(result), p.config.MaxJSONSize),
					Err:     ErrSizeLimit,
				}
			}
			return result, nil
		}
	}

	// Fallback: encode directly to []byte, avoiding []byte->string->[]byte round-trip.
	// D-002/R10: no-cfg starts from the processor's baked configuration (D-006)
	// — getConfigOrDefault forced DefaultConfig values, which (with M1's
	// effectiveEncodeMaxSize) bypassed the baked MaxJSONSize and ignored baked
	// encoding options on custom processors. EscapeHTML is still forced below
	// (the documented Marshal contract: output is always HTML-escaped).
	base := p.config
	if len(cfg) > 0 {
		base = cfg[0]
	}
	base.EscapeHTML = true
	return p.encodeWithConfigToBytes(value, base)
}

// MarshalIndent converts any Go value to indented JSON bytes (similar to json.MarshalIndent)
//
// Errors:
//   - ErrProcessorClosed: processor has been closed
//   - UnsupportedTypeError / UnsupportedValueError / MarshalerError: value cannot be encoded
//   - ErrSizeLimit: encoded output exceeds MaxJSONSize
//   - ErrDepthLimit: encoding exceeds the maximum nesting depth
func (p *Processor) MarshalIndent(value any, prefix, indent string, cfg ...Config) ([]byte, error) {
	// Nil/closed guard BEFORE reading p.config below (D-002/R10: the baked
	// base made this the first p dereference; checkClosed keeps the
	// nil-receiver contract every public method honors).
	if err := p.checkClosed(); err != nil {
		return nil, err
	}
	// D-002/R10: no-cfg starts from the processor's baked configuration
	// (D-006) — the previous DefaultConfig() base ignored baked encoding
	// options and, with M1's effectiveEncodeMaxSize, the baked MaxJSONSize.
	// The prefix and indent arguments still override the Config fields.
	encOpts := p.config
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
//
// Errors:
//   - ErrProcessorClosed: processor has been closed
//   - InvalidUnmarshalError: value is nil or not a non-nil pointer
//   - ErrInvalidJSON: data is not valid JSON
//   - UnmarshalTypeError: a JSON value does not match the target Go type
func (p *Processor) Unmarshal(data []byte, value any, cfg ...Config) error {
	if err := p.checkClosed(); err != nil {
		return err
	}

	if value == nil {
		return &InvalidUnmarshalError{Type: nil}
	}

	// Fast path when no options are provided. Validate against the processor's
	// baked-in security limits (size, nesting depth, dangerous patterns) for
	// consistency with Parse — which always validates — then delegate to
	// encoding/json to avoid string conversion overhead. Without this check the
	// drop-in Unmarshal signature bypassed all security validation.
	//
	// NOTE (D-002/R11 m1): this branch is deliberately NOT wired into
	// beginGovernedOp (the cfg branch below is, via governed Parse). It touches
	// no mutable processor state beyond the mutex-guarded securityValidator,
	// and gating the encoding/json drop-in on the MaxConcurrency semaphore
	// would newly reject calls that have always succeeded — an accepted,
	// documented divergence rather than a behavioral change.
	if len(cfg) == 0 {
		if err := p.validateInput(string(data)); err != nil {
			return err
		}
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
//
// Errors:
//   - ErrProcessorClosed: processor has been closed
//   - UnsupportedTypeError / UnsupportedValueError / MarshalerError: values cannot be encoded
//   - ErrSizeLimit: encoded output exceeds MaxJSONSize
//   - ErrDepthLimit: encoding exceeds the maximum nesting depth
func (p *Processor) EncodeStream(values any, cfg ...Config) (string, error) {
	if err := p.checkClosed(); err != nil {
		return "", err
	}
	// Variadic passthrough: with no cfg, Encode resolves the processor's baked
	// configuration (D-006 — previously getConfigOrDefault forced DefaultConfig,
	// ignoring baked encoding options on a custom processor).
	return p.Encode(values, cfg...)
}

// EncodeBatch encodes multiple key-value pairs as a JSON object.
// This method accepts variadic Config for unified API pattern.
//
// Example:
//
//	result, err := processor.EncodeBatch(pairs, json.PrettyConfig())
//
// Errors:
//   - ErrProcessorClosed: processor has been closed
//   - UnsupportedTypeError / UnsupportedValueError / MarshalerError: pairs cannot be encoded
//   - ErrSizeLimit: encoded output exceeds MaxJSONSize
//   - ErrDepthLimit: encoding exceeds the maximum nesting depth
func (p *Processor) EncodeBatch(pairs map[string]any, cfg ...Config) (string, error) {
	if err := p.checkClosed(); err != nil {
		return "", err
	}
	// Variadic passthrough: with no cfg, Encode resolves the processor's baked
	// configuration (D-006 — see EncodeStream).
	return p.Encode(pairs, cfg...)
}

// EncodeFields encodes struct fields selectively based on field names.
// This method accepts variadic Config for unified API pattern.
//
// Example:
//
//	result, err := processor.EncodeFields(value, []string{"name", "email"}, json.PrettyConfig())
//
// Errors:
//   - ErrProcessorClosed: processor has been closed
//   - ErrInvalidJSON: value cannot be parsed into an object
//   - ErrTypeMismatch: value is not a JSON object, so its fields cannot be filtered
//   - UnsupportedTypeError / UnsupportedValueError / MarshalerError: value cannot be encoded
//   - ErrSizeLimit: encoded output exceeds MaxJSONSize
func (p *Processor) EncodeFields(value any, fields []string, cfg ...Config) (string, error) {
	if err := p.checkClosed(); err != nil {
		return "", err
	}

	// First convert to JSON and parse back to get map representation
	config := DefaultConfig()
	config.Pretty = false
	tempJSON, err := p.Encode(value, config)
	if err != nil {
		return "", err
	}

	// Parse to any and convert to map
	var anyData any
	err = p.Parse(tempJSON, &anyData)
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

	// Variadic passthrough — the final encode honors cfg when supplied,
	// otherwise the processor's baked encoding options (D-006; the two
	// DefaultConfig() round-trip steps above are deliberate normalization).
	return p.Encode(filtered, cfg...)
}

// EncodeWithConfig converts any Go value to JSON string with full configuration control.
//
// Deprecated: EncodeWithConfig is functionally identical to Encode — the name
// predates Encode accepting an optional trailing Config. Use Encode(value, cfg).
// EncodeWithConfig will not be removed within v1 (per D-005 the module stays
// on v1.x).
//
// Errors: see Encode.
func (p *Processor) EncodeWithConfig(value any, cfg ...Config) (string, error) {
	return p.Encode(value, cfg...)
}

// Encode converts any Go value to JSON string with full configuration control.
// PERFORMANCE: Uses FastEncoder for simple types to avoid reflection overhead.
//
// The optional trailing Config selects the encoding behavior for this call —
// Pretty, Indent, EscapeHTML, SortKeys, FloatPrecision, CustomEscapes, and the
// depth/size limits. Omitted, the processor's baked configuration applies
// (DefaultConfig for a processor from New()).
//
// Example:
//
//	// Default configuration
//	result, err := processor.Encode(data)
//
//	// With custom configuration
//	cfg := json.DefaultConfig()
//	cfg.Pretty = true
//	result, err := processor.Encode(data, cfg)
//
//	// With preset configuration
//	result, err := processor.Encode(data, json.PrettyConfig())
//
// Errors:
//   - ErrProcessorClosed: processor has been closed
//   - UnsupportedTypeError / UnsupportedValueError / MarshalerError: value cannot be encoded
//   - ErrSizeLimit: encoded output exceeds MaxJSONSize
//   - ErrDepthLimit: encoding exceeds the maximum nesting depth
func (p *Processor) Encode(value any, cfg ...Config) (string, error) {
	b, err := p.encodeWithConfigToBytes(value, cfg...)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// encodeWithConfigToBytes encodes value to []byte directly, avoiding string round-trip.
// PERFORMANCE: Used by Marshal/MarshalIndent to eliminate []byte->string->[]byte conversion.
func (p *Processor) encodeWithConfigToBytes(value any, cfg ...Config) ([]byte, error) {
	// Concurrency governance: register as an in-flight op so a concurrent Close()
	// (e.g. cache eviction of a config-cached processor) drains via waitForActiveOps
	// rather than tearing down resources mid-encode. This is the single funnel for all
	// encode entry points (Marshal/MarshalIndent/Encode, and via those
	// EncodeStream/EncodeBatch), none of which are reached from within an already-
	// governed op, so acquiring here is never nested. In unlimited-concurrency mode
	// (the default) acquireSemaphore is a no-op, so the cost is two atomic ops.
	if err := p.beginGovernedOp(); err != nil {
		return nil, err
	}
	defer p.endGovernedOp()

	var config Config
	if len(cfg) > 0 {
		config = cfg[0]
	} else {
		// No-cfg: the processor's baked configuration applies (doc.go contract).
		// D-006: previously DefaultConfig(), so New(cfg).Encode(v) ignored the
		// baked encoding options (Pretty, EscapeHTML, FloatPrecision, MaxDepth)
		// and diverged from json.Encode(v, cfg).
		config = p.config
	}

	return p.encodeConfiguredToBytes(value, config)
}

// effectiveEncodeMaxSize returns the output-size cap for an encode operation:
// a positive per-call config value wins; otherwise the processor's baked limit
// applies. D-002/R8 (M1): every size check previously read p.config.MaxJSONSize,
// so a cfg-supplied MaxJSONSize was silently ignored on encoded output despite
// the doc.go contract ("its security limits take effect on that call"). Mirrors
// effectiveReadMaxSize in file.go.
func (p *Processor) effectiveEncodeMaxSize(config Config) int64 {
	if config.MaxJSONSize > 0 {
		return config.MaxJSONSize
	}
	return p.config.MaxJSONSize
}

// checkMutationOutputSize enforces the effective MaxJSONSize cap on a mutation
// operation's re-encoded output (Set/SetMultiple/Delete/ForeachReturn).
// D-002/R11 (M2, option A): the encode funnel gained this check in R8 M1, but
// the mutation paths re-marshal their modified documents via
// internal.FastMarshalToString and silently skipped it — Set could return
// output that the with-cfg Encode of the same configuration rejects. Only the
// SIZE is enforced: output bytes stay byte-identical to the previous
// FastMarshalToString behavior (notably, no HTML escaping), and depth is
// deliberately not re-checked (the parse already bounded the tree at
// MaxNestingDepthSecurity).
func (p *Processor) checkMutationOutputSize(result string, maxJSONSize int64, op, path string) error {
	if int64(len(result)) > maxJSONSize {
		return &JsonsError{
			Op:      op,
			Path:    path,
			Message: fmt.Sprintf("encoded JSON size %d exceeds maximum %d", len(result), maxJSONSize),
			Err:     ErrSizeLimit,
		}
	}
	return nil
}

// mutationOutputMaxSize resolves the output cap for a mutation operation. It
// mirrors the no-cfg rule the mutation paths already use for CreatePaths
// (D-006): with no cfg the processor's baked limit applies; a supplied cfg
// REPLACES it. options is the prepareOptions-validated per-call config, so its
// value is always positive (Config.Validate clamps non-positive to 1MB).
// effectiveEncodeMaxSize cannot be reused here — in the no-cfg case it would
// pick the shared default singleton's 100MB instead of the processor's baked
// limit (D-002/R11 M2).
func (p *Processor) mutationOutputMaxSize(options *Config, hasCfg bool) int64 {
	if hasCfg {
		return options.MaxJSONSize
	}
	return p.config.MaxJSONSize
}

// encodeConfiguredToBytes is the non-governed core of encodeWithConfigToBytes:
// config is already resolved (per-call cfg or the processor's baked config) and
// the caller holds the governed-op slot. Extracted so buffer-writing callers
// (see encodeConfiguredToBuffer) can share the exact same encode funnel.
func (p *Processor) encodeConfiguredToBytes(value any, config Config) ([]byte, error) {
	// needsCustomEncodingOpts is pure; compute once and reuse on the full-encode
	// branch below to avoid a second evaluation of the same config.
	customOpts := needsCustomEncodingOpts(config)

	// Fast path for simple types. The EscapeHTML==false case never reaches here:
	// needsCustomEncodingOpts returns true for it, so the fast path is always
	// the HTML-escaped encoding (which FastEncoder applies by post-processing).
	if !config.Pretty && !customOpts {
		if result, ok := p.fastEncodeSimpleToBytes(value, config.MaxDepth); ok {
			if maxJSONSize := p.effectiveEncodeMaxSize(config); int64(len(result)) > maxJSONSize {
				return nil, &JsonsError{
					Op:      "encode_with_config",
					Message: fmt.Sprintf("encoded JSON size %d exceeds maximum %d", len(result), maxJSONSize),
					Err:     ErrSizeLimit,
				}
			}
			return result, nil
		}
	}

	if config.MaxDepth > 0 {
		if err := p.validateDepth(value, config.MaxDepth, 0); err != nil {
			return nil, err
		}
	}

	var result []byte
	var err error

	if customOpts {
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

	if maxJSONSize := p.effectiveEncodeMaxSize(config); int64(len(result)) > maxJSONSize {
		return nil, &JsonsError{
			Op:      "encode_with_config",
			Message: fmt.Sprintf("encoded JSON size %d exceeds maximum %d", len(result), maxJSONSize),
			Err:     ErrSizeLimit,
		}
	}

	return result, nil
}

// encodingFastPathEligible reports whether every processor encoding option
// that the fast encoder does NOT honor is at its default. The fast encoder
// (internal.FastEncoder) fixes escaping, precision, and inclusion behavior at
// stdlib defaults, so running it with a non-default option silently ignores
// that option (D-002: a processor configured with FloatPrecision=2 emitted
// full precision here, while Encode rounded). Pretty/Indent/Prefix
// are handled by the caller's enc.indent/enc.prefix check plus !Pretty here.
func encodingFastPathEligible(cfg *Config) bool {
	return !cfg.Pretty &&
		cfg.EscapeHTML &&
		cfg.FloatPrecision < 0 &&
		!cfg.FloatTruncate &&
		cfg.CustomEscapes == nil &&
		cfg.CustomEncoder == nil &&
		len(cfg.CustomTypeEncoders) == 0 &&
		!cfg.EscapeUnicode &&
		!cfg.EscapeSlash &&
		!cfg.DisableEscaping &&
		cfg.EscapeNewlines &&
		cfg.EscapeTabs &&
		cfg.IncludeNulls
}

// isJSONNativeValue reports whether the value is a type the fast encoder
// handles without its stdlib fallback (and therefore with its own depth cap).
func isJSONNativeValue(value any) bool {
	switch value.(type) {
	case nil, bool, float64, string, json.Number, Number,
		int, int8, int16, int32, int64,
		uint, uint8, uint16, uint32, uint64, float32,
		map[string]any, []any:
		return true
	}
	return false
}

// fastEncodeSimpleToBytes encodes simple types directly to []byte, avoiding a
// []byte -> string -> []byte round-trip when the caller needs bytes
// (encodeConfiguredToBytes fast path, Marshal). Output is always
// HTML-escaped, matching encoding/json. Returns (nil, false) if the value
// needs the full encoder.
//
// maxDepth is the effective Config.MaxDepth (D-002): the fast encoder
// previously always used the package-wide nesting bound, silently accepting
// trees the config'd encoder rejects. n <= 0 keeps the package default.
//
// PERFORMANCE: Uses append(nil, data...) which is optimized by the compiler
// into a single allocation (runtime.memmove) without the explicit make+copy.
//
// Typed (non-JSON-native) roots are depth-pre-walked here: the encoder's
// internal stdlib fallback for such values has no cap of its own. KNOWN
// LIMITATION: a typed value NESTED inside a JSON-native root (e.g.
// map[string]any{"x": deepStructChain}) is not pre-walked — only the
// FastEncoder's own container cap (SetMaxEncodeDepth) applies to the native
// layers around it.
func (p *Processor) fastEncodeSimpleToBytes(value any, maxDepth int) ([]byte, bool) {
	if !isJSONNativeValue(value) && maxDepth > 0 {
		if err := p.validateDepth(value, maxDepth, 0); err != nil {
			// Return false: the full path re-runs validateDepth and surfaces
			// the proper typed error.
			return nil, false
		}
	}

	encoder := internal.GetEncoder()
	defer internal.PutEncoder(encoder)
	if maxDepth > 0 {
		encoder.SetMaxEncodeDepth(maxDepth)
	}

	err := encoder.EncodeValue(value)
	if err != nil {
		return nil, false
	}

	data := encoder.Bytes()
	if internal.NeedsHTMLEscapeBytes(data) {
		escaped := internal.HTMLEscapeBytes(data)
		// P-002 doc fix: escaped is NOT guaranteed fresh — below the pool cap
		// threshold it aliases an htmlEscapeBytesPool backing array. It is
		// deliberately NOT returned to the pool: the slice escapes to the
		// caller, forfeiting the buffer to the GC. Do not add a PutHTMLEscapeBytes
		// here (double-ownership of pool memory) and do not assume exclusive
		// ownership of the input's aliasing properties.
		return escaped, true
	}

	// Use append to clone — compiler optimizes append(nil, data...) to a
	// single alloc+copy without the separate make call overhead.
	return append([]byte(nil), data...), true
}

// fastEncodeSimpleToBuffer is fastEncodeSimpleToBytes writing into dst instead
// of returning a fresh []byte: the pooled encoder's output is copied straight
// into the caller's accumulation buffer, so the per-item clone allocation
// (and its memmove) disappears (P-001). Eligibility, depth handling, and the
// (nil, false)-style fallback contract are identical to fastEncodeSimpleToBytes.
func (p *Processor) fastEncodeSimpleToBuffer(value any, maxDepth int, dst *bytes.Buffer) bool {
	if !isJSONNativeValue(value) && maxDepth > 0 {
		if err := p.validateDepth(value, maxDepth, 0); err != nil {
			// Return false: the full path re-runs validateDepth and surfaces
			// the proper typed error (see fastEncodeSimpleToBytes).
			return false
		}
	}

	encoder := internal.GetEncoder()
	defer internal.PutEncoder(encoder)
	if maxDepth > 0 {
		encoder.SetMaxEncodeDepth(maxDepth)
	}

	if err := encoder.EncodeValue(value); err != nil {
		return false
	}

	data := encoder.Bytes()
	if internal.NeedsHTMLEscapeBytes(data) {
		internal.HTMLEscapeBytesTo(dst, data)
		return true
	}
	dst.Write(data)
	return true
}

// encodeConfiguredToBuffer appends the JSON encoding of value to dst — the
// buffer-writing counterpart of encodeConfiguredToBytes, used by ToJSONL so a
// batch of items is encoded into one shared output buffer instead of one
// transient []byte per item. Funnel parity with encodeConfiguredToBytes: same
// fast-path eligibility, same depth validation, same custom-opts fallback, and
// the same per-item MaxJSONSize enforcement (measured on the bytes appended
// for this item). Callers must already hold the governed-op slot — this helper
// never acquires governance itself.
func (p *Processor) encodeConfiguredToBuffer(value any, config Config, dst *bytes.Buffer) error {
	start := dst.Len()
	// D-002/R8 (M1): resolve the effective per-item cap once (per-call cfg
	// wins, else the processor's baked limit).
	maxJSONSize := p.effectiveEncodeMaxSize(config)

	// needsCustomEncodingOpts is pure; compute once and reuse on the full-encode
	// branch below to avoid a second evaluation of the same config.
	customOpts := needsCustomEncodingOpts(config)

	// Fast path for simple types (see encodeConfiguredToBytes; the
	// EscapeHTML==false case never reaches here for the same reasons).
	if !config.Pretty && !customOpts {
		if p.fastEncodeSimpleToBuffer(value, config.MaxDepth, dst) {
			return p.checkEncodedBufferSize(dst, start, maxJSONSize)
		}
	}

	if config.MaxDepth > 0 {
		if err := p.validateDepth(value, config.MaxDepth, 0); err != nil {
			return err
		}
	}

	var result []byte
	var err error

	if customOpts {
		encoder := newCustomEncoder(config)
		defer encoder.Close()
		result, err = encoder.EncodeToBytes(value)
	} else {
		result, err = internal.MarshalJSONToBytes(value, config.Pretty, config.Prefix, config.Indent)
	}

	if err != nil {
		return &JsonsError{
			Op:      "encode_with_config",
			Message: "failed to encode value",
			Err:     err,
		}
	}

	dst.Write(result)
	return p.checkEncodedBufferSize(dst, start, maxJSONSize)
}

// checkEncodedBufferSize enforces the per-item MaxJSONSize bound for
// buffer-accumulated encodings: written is the number of bytes appended to dst
// since start. maxJSONSize is the caller-resolved effective cap
// (effectiveEncodeMaxSize — D-002/R8 M1), mirroring the size check
// encodeConfiguredToBytes applies to its returned []byte, including the error
// shape.
func (p *Processor) checkEncodedBufferSize(dst *bytes.Buffer, start int, maxJSONSize int64) error {
	if n := int64(dst.Len() - start); n > maxJSONSize {
		return &JsonsError{
			Op:      "encode_with_config",
			Message: fmt.Sprintf("encoded JSON size %d exceeds maximum %d", n, maxJSONSize),
			Err:     ErrSizeLimit,
		}
	}
	return nil
}

// EncodePretty converts any Go value to pretty-formatted JSON string
// This is a convenience method that matches the package-level EncodePretty signature
//
// Errors: see Encode.
func (p *Processor) EncodePretty(value any, cfg ...Config) (string, error) {
	var config Config
	if len(cfg) > 0 {
		// Caller-supplied cfg passes through unchanged (historical contract:
		// EncodePretty does not force Pretty on an explicit cfg).
		config = cfg[0]
	} else {
		// No-cfg: the processor's baked configuration with Pretty forced
		// (D-006 — previously PrettyConfig(), which ignored a custom
		// processor's baked Indent/escaping). For a default processor this
		// is byte-identical to PrettyConfig(): DefaultConfig sets Indent "  ".
		config = p.config
		config.Pretty = true
	}
	return p.Encode(value, config)
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

// marshalerType / textMarshalerType are the reflect Types of the marshaler and
// encoding.TextMarshaler interfaces, used to detect pointer-receiver methods.
var (
	marshalerType     = reflect.TypeOf((*marshaler)(nil)).Elem()
	textMarshalerType = reflect.TypeOf((*encoding.TextMarshaler)(nil)).Elem()
)

// addressablePointer returns a pointer to an addressable form of v: v's own
// address when it is addressable, otherwise a pointer to a fresh copy. This
// lets customEncoder call pointer-receiver MarshalJSON/MarshalText methods,
// matching encoding/json (which marshals the addressable form). The boxed-any
// round-trip through encodeValue otherwise loses addressability.
func addressablePointer(v reflect.Value) reflect.Value {
	if v.CanAddr() {
		return v.Addr()
	}
	ptr := reflect.New(v.Type())
	ptr.Elem().Set(v)
	return ptr
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

	// Handle json.Number and the library's Number BEFORE the Marshaler checks:
	// Number.MarshalJSON (added so stdlib-marshal paths emit the raw literal)
	// makes the value satisfy the marshaler interface, which would bypass the
	// PreserveNumbers=false normalization below and change this encoder's
	// behavior for mixed-config round-trips.
	if jsonNum, ok := value.(json.Number); ok {
		return e.encodeJSONNumber(jsonNum)
	}
	if num, ok := value.(Number); ok {
		return e.encodeJSONNumber(json.Number(num))
	}

	// Check if the value implements json.Marshaler (value-receiver) first.
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
	// Pointer-receiver MarshalJSON on an addressable value (encoding/json calls
	// it on the addressable form). v is non-addressable here, so detect the
	// method via the pointer type and invoke on an addressable copy.
	//
	// KNOWN LIMITATION (D-002/R11 m6): for a NON-addressable root value whose
	// type implements MarshalJSON only on the POINTER receiver, encoding/json
	// does NOT call the method at all — newTypeEncoder selects the addressable
	// form only for struct fields/slice elements, and a boxed root value falls
	// through to plain struct encoding. This encoder calls it on a fresh copy,
	// so such values encode differently here than under stdlib (and any state
	// the method mutates does not persist). Matching stdlib exactly would drop
	// what is usually the user's intent, so the divergence is documented rather
	// than changed; it only applies to values that reach this custom encoder.
	if v.IsValid() && v.CanInterface() && reflect.PointerTo(v.Type()).Implements(marshalerType) {
		if m, ok := addressablePointer(v).Interface().(marshaler); ok {
			data, err := m.MarshalJSON()
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
	}

	// Check if the value implements encoding.TextMarshaler (value-receiver)
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
	// Pointer-receiver MarshalText on an addressable value (same rationale).
	if v.IsValid() && v.CanInterface() && reflect.PointerTo(v.Type()).Implements(textMarshalerType) {
		if m, ok := addressablePointer(v).Interface().(encoding.TextMarshaler); ok {
			text, err := m.MarshalText()
			if err != nil {
				return &MarshalerError{
					Type:       reflect.TypeOf(value),
					Err:        err,
					sourceFunc: "MarshalText",
				}
			}
			return e.encodeString(string(text))
		}
	}

	// (No time.Time special case: time.Time implements MarshalJSON with a
	// value receiver, so the marshaler checks above always handle it first —
	// the former special-case branches here were unreachable dead code.)

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
	case reflect.Slice:
		if v.Type().Elem().Kind() == reflect.Uint8 {
			// encoding/json encodes []byte as a base64 string; the previous
			// fall-through to encodeArray emitted a byte-number array
			// ([104,105]), silently changing the output shape whenever any
			// custom option routed values through this encoder. Arrays
			// ([N]byte) are intentionally NOT base64'd — stdlib only
			// special-cases slices.
			return e.encodeString(base64.StdEncoding.EncodeToString(v.Bytes()))
		}
		return e.encodeArray(v)
	case reflect.Array:
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

	// SECURITY: same guard as the fast encoder — an invalid literal (e.g.
	// "1_0") must not be written verbatim, which would emit invalid JSON
	// with a nil error.
	if !internal.IsValidJSONNumber(numStr) {
		return fmt.Errorf("invalid json.Number: %s", numStr)
	}

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
	// Reject non-finite values: NaN/Inf are not representable in JSON. encoding/json
	// returns UnsupportedValueError for these; emit an error with the same intent
	// instead of writing an invalid "NaN"/"+Inf" token (truncateFloat in particular
	// would otherwise produce garbage like "NaN.000000").
	if math.IsNaN(f) || math.IsInf(f, 0) {
		return fmt.Errorf("json: unsupported value: %s", strconv.FormatFloat(f, 'g', -1, bits))
	}
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

	// Default precision: route through AppendJSONFloat, which matches
	// encoding/json's floatEncoder exactly for all magnitudes — it uses 'e'
	// format for |f| < 1e-6 or |f| >= 1e21 and 'f' otherwise. The previous
	// 'f'-format branch for [-1e15, 1e15] emitted 0.0000001 for 1e-7 (and
	// 0.0000000001 for 1e-10) where stdlib emits 1e-7 / 1e-10.
	b := internal.AppendJSONFloat(nil, f, bits)
	e.buffer.Write(b)
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
		// Byte-level iteration: ranging over the string would decode invalid
		// UTF-8 bytes to U+FFFD before we can see them, so ValidateUTF8 could
		// never fire and the raw EF BF BD encoding would be written where
		// encoding/json writes the 6-character escape text (backslash u f f f d).
		for i := 0; i < len(s); i++ {
			r, size := utf8.DecodeRuneInString(s[i:])
			if r == utf8.RuneError && size == 1 {
				// Invalid UTF-8 byte. Note: ValidateUTF8 intentionally does
				// not reject here (documented behavior — see the ValidateUTF8
				// coverage test); emit stdlib's 6-character escape text
				// instead of the raw U+FFFD encoding.
				e.buffer.WriteString("\\ufffd")
				continue
			}
			if err := e.escapeRune(r); err != nil {
				return err
			}
			i += size - 1
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
	// Non-BMP code points (r > 0xFFFF, e.g. emoji) cannot fit in a single
	// \uXXXX escape. Emit a UTF-16 surrogate pair instead — matching
	// encoding/json and RFC 8259 §7. The previous code masked to 16 bits and
	// silently dropped the high bits (U+1F600 -> ).
	if r > 0xFFFF {
		r1, r2 := utf16.EncodeRune(r)
		e.writeUnicodeEscape(r1)
		e.writeUnicodeEscape(r2)
		return
	}
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
	case 0x2028, 0x2029:
		// U+2028/U+2029 are valid JSON but terminate lines in JavaScript;
		// encoding/json escapes them UNCONDITIONALLY — even under
		// SetEscapeHTML(false) — because emitting them raw reintroduces the
		// JSONP line-termination risk (see encoding/json encode.go: "It is
		// valid JSON to escape them, so we do so unconditionally"). The old
		// EscapeHTML-only condition here diverged from stdlib and silently
		// un-escaped these line terminators (D-002).
		e.writeUnicodeEscape(r)
	default:
		if r < 0x20 {
			e.writeUnicodeEscape(r)
		} else if e.config.EscapeHTML && (r == '<' || r == '>' || r == '&') {
			e.writeUnicodeEscape(r)
		} else if e.config.EscapeUnicode && r > 0x7F {
			e.writeUnicodeEscape(r)
		} else {
			e.buffer.WriteRune(r)
		}
	}

	return nil
}

func (e *customEncoder) encodeArray(v reflect.Value) error {
	// encoding/json emits null for a nil slice (distinct from [] for an empty
	// non-nil slice). Match that so callers can distinguish "absent" from "empty".
	if v.IsNil() {
		e.buffer.WriteString("null")
		return nil
	}
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

// formatMapKey renders a map key as the JSON property name encoding/json
// would use: string-kind keys verbatim, integer kinds in decimal, bool as
// true/false, interface keys by their element, and TextMarshaler via its
// text form. reflect.Value.String() cannot be used — for any non-String kind
// it returns the "<T Value>" placeholder, which destroyed every non-string
// key (map[int]string{1,2} → two identical "<int Value>" keys).
func formatMapKey(k reflect.Value) string {
	if k.Kind() == reflect.Interface {
		k = k.Elem()
	}
	switch k.Kind() {
	case reflect.String:
		return k.String()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return strconv.FormatInt(k.Int(), 10)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return strconv.FormatUint(k.Uint(), 10)
	case reflect.Bool:
		if k.Bool() {
			return "true"
		}
		return "false"
	}
	if k.CanInterface() {
		if tm, ok := k.Interface().(encoding.TextMarshaler); ok {
			if text, err := tm.MarshalText(); err == nil {
				return string(text)
			}
		}
	}
	// Exotic key kinds (floats, complex...) — encoding/json rejects these;
	// approximate with the old placeholder rather than erroring here.
	return k.String()
}

func (e *customEncoder) encodeMap(v reflect.Value) error {
	// encoding/json emits null for a nil map (distinct from {} for an empty
	// non-nil map). Match that so callers can distinguish "absent" from "empty".
	if v.IsNil() {
		e.buffer.WriteString("null")
		return nil
	}
	e.buffer.WriteByte('{')
	e.depth++

	keys := v.MapKeys()
	// Pre-render property names once (formatMapKey is not free) and sort the
	// key/name pairs by name. encoding/json always sorts object keys; match it
	// regardless of SortKeys — Go maps have no insertion order, so unsorted
	// output is nondeterministic and never useful.
	type kvPair struct {
		key  reflect.Value
		name string
	}
	pairs := make([]kvPair, len(keys))
	for i, key := range keys {
		pairs[i] = kvPair{key: key, name: formatMapKey(key)}
	}
	sort.Slice(pairs, func(i, j int) bool {
		return pairs[i].name < pairs[j].name
	})

	first := true
	for _, pair := range pairs {
		value := v.MapIndex(pair.key)

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

		if err := e.encodeString(pair.name); err != nil {
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
	// `first` (post-filter), not len(keys): when IncludeNulls=false removes
	// every value, the pre-filter length would still write a dangling
	// indent-only line inside the braces ({\n  \n} instead of {}).
	if e.config.Pretty && !first {
		e.writeIndent()
	}
	e.buffer.WriteByte('}')

	return nil
}

func (e *customEncoder) encodeStruct(v reflect.Value) error {
	// Use custom encoding when any of these advanced features are enabled.
	// CustomEscapes and DisableEscaping must be listed too: both force the
	// custom encoder at the routing level (needsCustomEncodingOpts), and
	// falling through to json.Marshal here silently applied stdlib escaping
	// to struct string fields while map values honored the config.
	if !e.config.IncludeNulls || e.config.SortKeys || !e.config.EscapeHTML ||
		e.config.FloatPrecision >= 0 || !e.config.EscapeNewlines || !e.config.EscapeTabs ||
		e.config.EscapeSlash || e.config.EscapeUnicode ||
		e.config.CustomEscapes != nil || e.config.DisableEscaping {
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

// cField is a resolved JSON struct field: its encoded name, the reflect.Value
// to encode, and the omitempty / ,string tag options.
type cField struct {
	name      string
	value     reflect.Value
	omitempty bool
	stringTag bool
}

// flattenStructFields walks the struct v and returns its JSON fields, promoting
// anonymous (embedded) struct fields into the parent — matching encoding/json.
// Rules applied:
//   - unexported fields and `json:"-"` fields are skipped;
//   - an anonymous struct field with NO explicit json name has its exported
//     fields promoted (recursively, through pointer embedding);
//   - an anonymous field WITH a json name, or an anonymous non-struct (e.g. an
//     embedded named integer type), is treated as an ordinary named field;
//   - name conflicts resolve first-wins (a pragmatic approximation of
//     encoding/json's shallowest-depth dominance rule).
func (e *customEncoder) flattenStructFields(v reflect.Value) []cField {
	var out []cField
	seen := make(map[string]bool)
	var visit func(sv reflect.Value)
	visit = func(sv reflect.Value) {
		t := sv.Type()
		for i := 0; i < t.NumField(); i++ {
			field := t.Field(i)
			if !field.IsExported() {
				continue
			}
			jsonTag := field.Tag.Get("json")
			if jsonTag == "-" {
				continue
			}
			tagParts := strings.Split(jsonTag, ",")
			fv := sv.Field(i)

			// Anonymous promotion: dereference pointer embedding, then recurse
			// into the embedded struct when it has no explicit json name.
			if field.Anonymous {
				afv := fv
				if afv.Kind() == reflect.Pointer {
					if afv.IsNil() {
						continue // nil embedded pointer: nothing to promote
					}
					afv = afv.Elem()
				}
				hasName := jsonTag != "" && tagParts[0] != ""
				if afv.Kind() == reflect.Struct && !hasName {
					visit(afv)
					continue
				}
				// tagged-anonymous or anonymous non-struct: fall through and
				// treat as an ordinary named field.
				fv = afv
			}

			name := field.Name
			if jsonTag != "" && tagParts[0] != "" {
				name = tagParts[0]
			}
			if seen[name] {
				continue
			}
			seen[name] = true
			out = append(out, cField{
				name:      name,
				value:     fv,
				omitempty: slices.Contains(tagParts[1:], "omitempty"),
				stringTag: slices.Contains(tagParts[1:], "string"),
			})
		}
	}
	visit(v)
	return out
}

func (e *customEncoder) encodeStructCustom(v reflect.Value) error {
	e.buffer.WriteByte('{')
	e.depth++

	// Resolve fields with anonymous-field promotion, then apply omitempty /
	// IncludeNulls filtering. flattenStructFields already extracted names and
	// the ,string / omitempty options.
	allFields := e.flattenStructFields(v)
	fields := make([]cField, 0, len(allFields))
	for _, f := range allFields {
		if f.omitempty && e.isEmpty(f.value) {
			continue
		}
		if !e.config.IncludeNulls {
			if f.value.Interface() == nil || (f.value.Kind() == reflect.Pointer && f.value.IsNil()) {
				continue
			}
		}
		fields = append(fields, f)
	}

	if e.config.SortKeys {
		sort.Slice(fields, func(i, j int) bool {
			return fields[i].name < fields[j].name
		})
	}

	for i, f := range fields {
		if i > 0 {
			e.buffer.WriteByte(',')
		}

		if e.config.Pretty {
			e.writeIndent()
		}

		if err := e.encodeString(f.name); err != nil {
			return err
		}

		e.buffer.WriteByte(':')
		if e.config.Pretty {
			e.buffer.WriteByte(' ')
		}

		if f.stringTag {
			// json:",string" wraps the field's JSON encoding as a JSON string
			// (e.g. an int 42 -> "42"), matching encoding/json.
			tmp := getEncoderBuffer()
			saved := e.buffer
			e.buffer = tmp
			verr := e.encodeValue(f.value.Interface())
			e.buffer = saved
			if verr != nil {
				putEncoderBuffer(tmp)
				return verr
			}
			serr := e.encodeString(tmp.String())
			putEncoderBuffer(tmp)
			if serr != nil {
				return serr
			}
		} else {
			if err := e.encodeValue(f.value.Interface()); err != nil {
				return err
			}
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
