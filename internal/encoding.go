package internal

import (
	"bytes"
	"encoding/json"
	"sync"
)

// MarshalJSON marshals a value to JSON string with optional pretty printing
func MarshalJSON(value any, pretty bool, prefix, indent string) (string, error) {
	var resultBytes []byte
	var err error

	if pretty {
		resultBytes, err = json.MarshalIndent(value, prefix, indent)
	} else {
		resultBytes, err = json.Marshal(value)
	}

	if err != nil {
		return "", err
	}

	return string(resultBytes), nil
}

// IsSpace reports whether the character is a JSON whitespace character
func IsSpace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\r' || c == '\n'
}

// IsDigit reports whether the character is a digit
func IsDigit(c byte) bool {
	return '0' <= c && c <= '9'
}

// Buffer pools for memory optimization
var (
	encoderBufferPool = sync.Pool{
		New: func() any {
			buf := &bytes.Buffer{}
			buf.Grow(2048)
			return buf
		},
	}
)

// GetEncoderBuffer gets a buffer from the pool
func GetEncoderBuffer() *bytes.Buffer {
	buf := encoderBufferPool.Get().(*bytes.Buffer)
	buf.Reset()
	return buf
}

// PutEncoderBuffer returns a buffer to the pool
func PutEncoderBuffer(buf *bytes.Buffer) {
	const maxPoolBufferSize = 8 * 1024
	const minPoolBufferSize = 256
	if buf != nil {
		c := buf.Cap()
		if c >= minPoolBufferSize && c <= maxPoolBufferSize {
			buf.Reset()
			encoderBufferPool.Put(buf)
		}
	}
}

// StringToBytes converts string to []byte
// Using standard conversion for safety and compatibility
func StringToBytes(s string) []byte {
	return []byte(s)
}

// ContainsAnyByte checks if string contains any of the specified bytes
// This is faster than strings.ContainsAny for single-byte character sets
func ContainsAnyByte(s, chars string) bool {
	for i := 0; i < len(s); i++ {
		for j := 0; j < len(chars); j++ {
			if s[i] == chars[j] {
				return true
			}
		}
	}
	return false
}

// IsValidNumberString checks if a string represents a valid number
func IsValidNumberString(s string) bool {
	if s == "" {
		return false
	}

	// Use json.Number to validate
	num := json.Number(s)
	_, err := num.Float64()
	return err == nil
}
