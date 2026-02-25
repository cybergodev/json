package internal

import (
	"bytes"
	"encoding/json"
	"strconv"
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
	// PERFORMANCE: Added byte slice pool for encoding operations
	byteSlicePool = sync.Pool{
		New: func() any {
			b := make([]byte, 0, 1024)
			return &b
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

// GetByteSlice gets a byte slice from the pool
// PERFORMANCE: Reusable byte slices for encoding operations
func GetByteSlice() *[]byte {
	return byteSlicePool.Get().(*[]byte)
}

// PutByteSlice returns a byte slice to the pool
func PutByteSlice(b *[]byte) {
	if b == nil {
		return
	}
	const maxByteSliceCap = 32 * 1024 // 32KB
	const minByteSliceCap = 256
	c := cap(*b)
	if c >= minByteSliceCap && c <= maxByteSliceCap {
		*b = (*b)[:0]
		byteSlicePool.Put(b)
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

// ParseIntFast parses a string as an integer without using strconv
// PERFORMANCE: Avoids strconv.Atoi allocation for common cases
// Returns (value, true) if successful, (0, false) otherwise
func ParseIntFast(s string) (int, bool) {
	if len(s) == 0 {
		return 0, false
	}

	negative := false
	start := 0
	if s[0] == '-' {
		negative = true
		start = 1
		if len(s) == 1 {
			return 0, false
		}
	}

	// Fast path for single digit
	if len(s)-start == 1 {
		c := s[start]
		if c < '0' || c > '9' {
			return 0, false
		}
		val := int(c - '0')
		if negative {
			val = -val
		}
		return val, true
	}

	// Parse multi-digit number
	var result int
	for i := start; i < len(s); i++ {
		c := s[i]
		if c < '0' || c > '9' {
			return 0, false
		}
		// Check for overflow
		if result > (1<<31-1)/10 {
			return 0, false
		}
		result = result*10 + int(c-'0')
	}

	if negative {
		result = -result
	}

	return result, true
}

// smallIntStrings contains pre-computed string representations for integers 0-99
// PERFORMANCE: Avoids strconv.Itoa allocations for common values
var smallIntStrings = [100]string{
	"0", "1", "2", "3", "4", "5", "6", "7", "8", "9",
	"10", "11", "12", "13", "14", "15", "16", "17", "18", "19",
	"20", "21", "22", "23", "24", "25", "26", "27", "28", "29",
	"30", "31", "32", "33", "34", "35", "36", "37", "38", "39",
	"40", "41", "42", "43", "44", "45", "46", "47", "48", "49",
	"50", "51", "52", "53", "54", "55", "56", "57", "58", "59",
	"60", "61", "62", "63", "64", "65", "66", "67", "68", "69",
	"70", "71", "72", "73", "74", "75", "76", "77", "78", "79",
	"80", "81", "82", "83", "84", "85", "86", "87", "88", "89",
	"90", "91", "92", "93", "94", "95", "96", "97", "98", "99",
}

// IntToStringFast converts an integer to string using pre-computed values
// PERFORMANCE: Avoids strconv.Itoa allocations for values 0-99
func IntToStringFast(n int) string {
	if n >= 0 && n < 100 {
		return smallIntStrings[n]
	}
	return strconv.Itoa(n)
}

// EncodeFast attempts to encode a primitive value directly to a buffer
// PERFORMANCE: Inline encoding for primitives avoids reflection and allocations
// Returns true if the value was encoded, false if it needs standard encoding
func EncodeFast(v any, buf *bytes.Buffer) bool {
	switch val := v.(type) {
	case nil:
		buf.WriteString("null")
		return true
	case bool:
		if val {
			buf.WriteString("true")
		} else {
			buf.WriteString("false")
		}
		return true
	case int:
		buf.WriteString(IntToStringFast(val))
		return true
	case int8:
		buf.WriteString(strconv.FormatInt(int64(val), 10))
		return true
	case int16:
		buf.WriteString(strconv.FormatInt(int64(val), 10))
		return true
	case int32:
		buf.WriteString(strconv.FormatInt(int64(val), 10))
		return true
	case int64:
		buf.WriteString(strconv.FormatInt(val, 10))
		return true
	case uint:
		buf.WriteString(strconv.FormatUint(uint64(val), 10))
		return true
	case uint8:
		buf.WriteString(strconv.FormatUint(uint64(val), 10))
		return true
	case uint16:
		buf.WriteString(strconv.FormatUint(uint64(val), 10))
		return true
	case uint32:
		buf.WriteString(strconv.FormatUint(uint64(val), 10))
		return true
	case uint64:
		buf.WriteString(strconv.FormatUint(val, 10))
		return true
	case float32:
		buf.WriteString(strconv.FormatFloat(float64(val), 'f', -1, 32))
		return true
	case float64:
		buf.WriteString(strconv.FormatFloat(val, 'f', -1, 64))
		return true
	case string:
		buf.WriteByte('"')
		writeEscapedStringFast(buf, val)
		buf.WriteByte('"')
		return true
	}
	return false
}

// writeEscapedStringFast writes an escaped JSON string to the buffer
// PERFORMANCE: Optimized escape handling without allocations
func writeEscapedStringFast(buf *bytes.Buffer, s string) {
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch c {
		case '"':
			buf.WriteString(`\"`)
		case '\\':
			buf.WriteString(`\\`)
		case '\b':
			buf.WriteString(`\b`)
		case '\f':
			buf.WriteString(`\f`)
		case '\n':
			buf.WriteString(`\n`)
		case '\r':
			buf.WriteString(`\r`)
		case '\t':
			buf.WriteString(`\t`)
		default:
			if c < 0x20 {
				buf.WriteString(`\u00`)
				buf.WriteByte(hexChars[c>>4])
				buf.WriteByte(hexChars[c&0x0F])
			} else {
				buf.WriteByte(c)
			}
		}
	}
}

// hexChars contains hex characters for escape sequences
var hexChars = [16]byte{
	'0', '1', '2', '3', '4', '5', '6', '7', '8', '9', 'a', 'b', 'c', 'd', 'e', 'f',
}
