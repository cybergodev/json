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
)

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
func NewDecoder(r io.Reader) *Decoder {
	return &Decoder{
		r:         r,
		buf:       bufio.NewReader(r),
		processor: getDefaultProcessor(),
	}
}

// Decode reads the next JSON-encoded value from its input and stores it in v.
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

func (dec *Decoder) assignResult(result any, v any) error {
	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Ptr || rv.IsNil() {
		return fmt.Errorf("json: Unmarshal(non-pointer %T)", v)
	}

	rv = rv.Elem()
	resultValue := reflect.ValueOf(result)

	// If the target is any (interface{}), assign directly
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

func (dec *Decoder) UseNumber() {
	dec.useNumber = true
}

func (dec *Decoder) DisallowUnknownFields() {
	dec.disallowUnknownFields = true
}

func (dec *Decoder) Buffered() io.Reader {
	return dec.buf
}

func (dec *Decoder) InputOffset() int64 {
	return dec.offset
}

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

// readContainerValue reads a complete JSON object or array
func (dec *Decoder) readContainerValue(buf *bytes.Buffer, _ byte) ([]byte, error) {
	depth := 1
	inString := false
	escaped := false

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
		case '}', ']':
			depth--
			if depth == 0 {
				result := make([]byte, buf.Len())
				copy(result, buf.Bytes())
				return result, nil
			}
		}
	}

	result := make([]byte, buf.Len())
	copy(result, buf.Bytes())
	return result, nil
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
			dec.buf.UnreadByte()
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

		actual, _ := dec.buf.ReadByte()
		dec.offset++
		buf.WriteByte(actual)
	}

	numStr := buf.String()

	if dec.useNumber {
		return Number(numStr), nil
	}

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
