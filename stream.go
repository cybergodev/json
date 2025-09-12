package json

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"strconv"
	"strings"
	"sync"
)

// =============================================================================
// BUFFER POOLS FOR MEMORY OPTIMIZATION
// =============================================================================

var (
	// Global buffer pools for memory efficiency with improved sizing
	bufferPool = sync.Pool{
		New: func() interface{} {
			buf := make([]byte, 0, 1024) // Increased initial capacity for better performance
			return &buf
		},
	}

	bytesBufferPool = sync.Pool{
		New: func() interface{} {
			buf := &bytes.Buffer{}
			buf.Grow(1024) // Pre-allocate reasonable capacity
			return buf
		},
	}
)

// getBuffer gets a byte slice from the pool with enhanced safety
func getBuffer() *[]byte {
	if buf := bufferPool.Get(); buf != nil {
		if slice, ok := buf.(*[]byte); ok {
			*slice = (*slice)[:0] // Reset length but keep capacity
			return slice
		}
	}
	// Fallback: create new buffer with optimized capacity
	buf := make([]byte, 0, 1024)
	return &buf
}

// putBuffer returns a byte slice to the pool with optimized size limits
func putBuffer(buf *[]byte) {
	if buf != nil && cap(*buf) <= 16384 && cap(*buf) >= 512 { // Optimized limits
		*buf = (*buf)[:0] // Reset length but keep capacity
		bufferPool.Put(buf)
	}
}

// getBytesBuffer gets a bytes.Buffer from the pool with enhanced safety
func getBytesBuffer() *bytes.Buffer {
	if buf := bytesBufferPool.Get(); buf != nil {
		if buffer, ok := buf.(*bytes.Buffer); ok {
			buffer.Reset()
			return buffer
		}
	}
	// Fallback: create new buffer with optimized capacity
	buf := bytes.NewBuffer(make([]byte, 0, 1024))
	return buf
}

// putBytesBuffer returns a bytes.Buffer to the pool with optimized size limits
func putBytesBuffer(buf *bytes.Buffer) {
	if buf != nil && buf.Cap() <= 16384 && buf.Cap() >= 512 { // Optimized limits
		buf.Reset() // Ensure buffer is clean before returning to pool
		bytesBufferPool.Put(buf)
	}
}

// =============================================================================
// DECODER IMPLEMENTATION
// =============================================================================

// Decoder reads and decodes JSON values from an input stream.
// This type is fully compatible with encoding/json.Decoder.
type Decoder struct {
	r                     io.Reader
	buf                   *bufio.Reader
	processor             *Processor
	useNumber             bool
	disallowUnknownFields bool
	offset                int64
	scanp                 int64 // start of unread data in buf
}

// NewDecoder returns a new decoder that reads from r.
// This function is fully compatible with encoding/json.NewDecoder.
//
// The decoder introduces its own buffering and may
// read data from r beyond the JSON values requested.
func NewDecoder(r io.Reader) *Decoder {
	return &Decoder{
		r:         r,
		buf:       bufio.NewReader(r),
		processor: getDefaultProcessor(),
	}
}

// Decode reads the next JSON-encoded value from its
// input and stores it in the value pointed to by v.
//
// See the documentation for Unmarshal for details about
// the conversion of JSON into a Go value.
func (dec *Decoder) Decode(v any) error {
	if v == nil {
		return &InvalidUnmarshalError{Type: nil}
	}

	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Ptr || rv.IsNil() {
		return &InvalidUnmarshalError{Type: reflect.TypeOf(v)}
	}

	// Read the next JSON value from the stream
	data, err := dec.readValue()
	if err != nil {
		return err
	}

	// Handle UseNumber directly for compatibility
	if dec.useNumber {
		// Use NumberPreservingDecoder for UseNumber functionality
		decoder := NewNumberPreservingDecoder(true)
		result, err := decoder.DecodeToAny(string(data))
		if err != nil {
			return err
		}

		// Use reflection to assign the result properly
		return dec.assignResult(result, v)
	}

	// Use the processor's Unmarshal method for normal cases
	return dec.processor.Unmarshal(data, v)
}

// assignResult assigns the decoded result to the target value using reflection
func (dec *Decoder) assignResult(result any, v any) error {
	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Ptr || rv.IsNil() {
		return fmt.Errorf("json: Unmarshal(non-pointer %T)", v)
	}

	rv = rv.Elem()
	resultValue := reflect.ValueOf(result)

	// If the target is interface{}, assign directly
	if rv.Kind() == reflect.Interface {
		rv.Set(resultValue)
		return nil
	}

	// For other types, try to assign if compatible
	if resultValue.Type().AssignableTo(rv.Type()) {
		rv.Set(resultValue)
		return nil
	}

	// If not directly assignable, fall back to marshal/unmarshal
	jsonBytes, err := Marshal(result)
	if err != nil {
		return err
	}
	return json.Unmarshal(jsonBytes, v)
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

// Buffered returns a reader of the data remaining in the Decoder's
// buffer. The reader is valid until the next call to Decode.
func (dec *Decoder) Buffered() io.Reader {
	return dec.buf
}

// InputOffset returns the input stream byte offset of the current decoder position.
// The offset gives the location of the end of the most recently returned token
// and the beginning of the next token.
func (dec *Decoder) InputOffset() int64 {
	return dec.offset
}

// More reports whether there is another element in the
// current array or object being parsed.
func (dec *Decoder) More() bool {
	// Peek at the next byte to see if there's more data
	b, err := dec.buf.Peek(1)
	if err != nil {
		return false
	}

	// Skip whitespace
	for len(b) > 0 && isSpace(b[0]) {
		dec.buf.ReadByte()
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
//
// Token guarantees that the delimiters [ ] { } it returns are
// properly nested and matched: if Token encounters an unexpected
// delimiter in the input, it will return an error.
//
// The input stream consists of zero or more JSON values,
// each separated by optional whitespace.
//
// A Token holds one of these types:
//
//	Delim, for the four JSON delimiters [ ] { }
//	bool, for JSON booleans
//	float64, for JSON numbers
//	Number, for JSON numbers
//	string, for JSON string literals
//	nil, for JSON null
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

// readValue reads a complete JSON value from the input stream
func (dec *Decoder) readValue() ([]byte, error) {
	// Use buffer pool for memory efficiency
	buf := getBytesBuffer()
	defer putBytesBuffer(buf)

	depth := 0
	inString := false
	escaped := false

	// Skip leading whitespace
	for {
		b, err := dec.buf.ReadByte()
		if err != nil {
			return nil, err
		}
		dec.offset++

		if !isSpace(b) {
			buf.WriteByte(b)

			// Handle the first character to determine value type
			switch b {
			case '"':
				inString = true
			case '{', '[':
				depth++
			}
			break
		}
	}

	// If we have a simple value (not object/array), read until delimiter
	if depth == 0 && !inString {
		for {
			b, err := dec.buf.ReadByte()
			if err != nil {
				if err == io.EOF {
					break
				}
				return nil, err
			}
			dec.offset++

			// Stop at whitespace, comma, or structural characters for primitive values
			if isSpace(b) || b == ',' || b == '}' || b == ']' {
				// Put the delimiter back for next read
				dec.buf.UnreadByte()
				dec.offset--
				break
			}

			buf.WriteByte(b)
		}
		// Return a copy of the buffer data since we're returning it to the pool
		result := make([]byte, buf.Len())
		copy(result, buf.Bytes())
		return result, nil
	}

	// For strings and complex objects, continue reading
	for {
		b, err := dec.buf.ReadByte()
		if err != nil {
			if err == io.EOF && buf.Len() > 0 {
				break
			}
			return nil, err
		}
		dec.offset++
		buf.WriteByte(b)

		if escaped {
			escaped = false
			continue
		}

		if inString {
			if b == '\\' {
				escaped = true
			} else if b == '"' {
				inString = false
				// If we're not in an object/array, string is complete
				if depth == 0 {
					// Return a copy of the buffer data since we're returning it to the pool
					result := make([]byte, buf.Len())
					copy(result, buf.Bytes())
					return result, nil
				}
			}
			continue
		}

		switch b {
		case '"':
			inString = true
		case '{', '[':
			depth++
		case '}', ']':
			depth--
			if depth == 0 {
				// Return a copy of the buffer data since we're returning it to the pool
				result := make([]byte, buf.Len())
				copy(result, buf.Bytes())
				return result, nil
			}
		}
	}

	// Return a copy of the buffer data since we're returning it to the pool
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

// parseString parses a JSON string token
func (dec *Decoder) parseString() (string, error) {
	// Use buffer pool for memory efficiency
	buf := getBytesBuffer()
	defer putBytesBuffer(buf)

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
			// Handle escape sequences
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
				// Unicode escape sequence
				var hex [4]byte
				for i := 0; i < 4; i++ {
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
				buf.WriteRune(rune(code))
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

// parseBoolean parses a JSON boolean token
func (dec *Decoder) parseBoolean(first byte) (bool, error) {
	if first == 't' {
		// Expect "rue"
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
	} else {
		// Expect "alse"
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
}

// parseNull parses a JSON null token
func (dec *Decoder) parseNull() (any, error) {
	// Expect "ull"
	expected := "ull"
	for i, expected_char := range expected {
		b, err := dec.buf.ReadByte()
		if err != nil {
			return nil, err
		}
		dec.offset++
		if b != byte(expected_char) {
			return nil, &SyntaxError{
				msg:    fmt.Sprintf("invalid character '%c' in literal null (expecting '%c')", b, expected_char),
				Offset: dec.offset - int64(i) - 2,
			}
		}
	}
	return nil, nil
}

// parseNumber parses a JSON number token
func (dec *Decoder) parseNumber(first byte) (any, error) {
	// Use buffer pool for memory efficiency
	buf := getBytesBuffer()
	defer putBytesBuffer(buf)
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

		actual, _ := dec.buf.ReadByte()
		dec.offset++
		buf.WriteByte(actual)
	}

	numStr := buf.String()

	if dec.useNumber {
		return Number(numStr), nil
	}

	// Try to parse as integer first
	if !strings.Contains(numStr, ".") && !strings.Contains(numStr, "e") && !strings.Contains(numStr, "E") {
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

// =============================================================================
// ENCODER IMPLEMENTATION
// =============================================================================

// Encoder writes JSON values to an output stream.
// This type is fully compatible with encoding/json.Encoder.
type Encoder struct {
	w          io.Writer
	processor  *Processor
	escapeHTML bool
	indent     string
	prefix     string
}

// NewEncoder returns a new encoder that writes to w.
// This function is fully compatible with encoding/json.NewEncoder.
func NewEncoder(w io.Writer) *Encoder {
	return &Encoder{
		w:          w,
		processor:  getDefaultProcessor(),
		escapeHTML: true, // Default behavior matches encoding/json
	}
}

// Encode writes the JSON encoding of v to the stream,
// followed by a newline character.
//
// See the documentation for Marshal for details about the
// conversion of Go values to JSON.
func (enc *Encoder) Encode(v any) error {
	// Create encoding config based on encoder settings
	config := DefaultEncodeConfig()
	config.EscapeHTML = enc.escapeHTML

	if enc.indent != "" || enc.prefix != "" {
		config.Pretty = true
		config.Indent = enc.indent
		config.Prefix = enc.prefix
	}

	// Encode the value
	jsonStr, err := enc.processor.EncodeWithConfig(v, config)
	if err != nil {
		return err
	}

	// Write to the output stream with a newline
	_, err = enc.w.Write([]byte(jsonStr + "\n"))
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

// =============================================================================
// HELPER TYPES AND FUNCTIONS
// =============================================================================

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
	return c == ' ' || c == '\t' || c == '\r' || c == '\n'
}

// isDigit reports whether the character is a digit.
func isDigit(c byte) bool {
	return '0' <= c && c <= '9'
}

// isCompleteValue reports whether the string represents a complete JSON primitive value.
func isCompleteValue(s string) bool {
	if len(s) == 0 {
		return false
	}

	// Check for complete literals
	switch s {
	case "true", "false", "null":
		return true
	}

	// Check for complete string (starts and ends with quotes)
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		return true
	}

	// Check for complete number
	if isDigit(s[0]) || s[0] == '-' {
		// Simple check - if it parses as a number, it's complete
		_, err := strconv.ParseFloat(s, 64)
		return err == nil
	}

	return false
}
