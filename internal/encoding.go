package internal

import (
	"bytes"
	"encoding/json"
	"strconv"
	"sync"
	"unsafe"
)

// MarshalJSON marshals a value to JSON string with optional pretty printing.
//
// NOTE (D-002/R8 M5): no production caller — MarshalJSONToBytes is the live
// path; retained for tests/future use.
func MarshalJSON(value any, pretty bool, prefix, indent string) (string, error) {
	resultBytes, err := MarshalJSONToBytes(value, pretty, prefix, indent)
	if err != nil {
		return "", err
	}
	return string(resultBytes), nil
}

// MarshalJSONToBytes marshals a value to JSON bytes with optional pretty printing.
// PERFORMANCE: Returns []byte directly to avoid string conversion when caller needs bytes.
func MarshalJSONToBytes(value any, pretty bool, prefix, indent string) ([]byte, error) {
	if pretty {
		return json.MarshalIndent(value, prefix, indent)
	}
	return json.Marshal(value)
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
	if buf != nil {
		c := buf.Cap()
		if c >= MinPoolBufferSize && c <= MaxPoolBufferSize/4 {
			buf.Reset()
			encoderBufferPool.Put(buf)
		}
	}
}

// StringToBytes converts a string to a byte slice without allocation.
// The returned slice shares memory with the input string.
//
// SECURITY WARNING: The returned []byte MUST NOT be modified. Writing to it
// corrupts the original string, violating Go's string immutability guarantee.
// This can cause data integrity violations and security bypasses (e.g.,
// modifying a validated JSON string after validation passes — TOCTOU).
func StringToBytes(s string) []byte {
	return unsafe.Slice(unsafe.StringData(s), len(s))
}

// ParseIntFast parses a string as an integer without using strconv
// PERFORMANCE: Avoids strconv.Atoi allocation for common cases
// SECURITY: Proper overflow detection for both 32-bit and 64-bit systems
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

	// SECURITY: Use int64 for parsing to ensure consistent behavior across platforms
	// This avoids the const-based approach which behaves differently on 32-bit vs 64-bit
	var result int64
	for i := start; i < len(s); i++ {
		c := s[i]
		if c < '0' || c > '9' {
			return 0, false
		}
		digit := int64(c - '0')

		// SECURITY: Check overflow before multiplication and addition
		// MaxInt64 = 9223372036854775807
		// MinInt64 = -9223372036854775808
		const maxInt64 = 9223372036854775807
		const cutoff = maxInt64 / 10

		if result > cutoff || (result == cutoff && digit > maxInt64%10) {
			// Would overflow int64
			if negative {
				// Check if this is exactly MinInt64: only valid on 64-bit platforms
				const minInt64 int64 = -9223372036854775808
				if result == cutoff && digit == maxInt64%10+1 && int64(int(minInt64)) == minInt64 {
					return int(minInt64), true
				}
			}
			return 0, false
		}
		result = result*10 + digit
	}

	if negative {
		result = -result
	}

	// SECURITY: Verify the result fits in int (platform-dependent)
	converted := int(result)
	if int64(converted) != result {
		return 0, false // Overflow on 32-bit platform
	}

	return converted, true
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

// hexChars contains hex characters for escape sequences
var hexChars = [16]byte{
	'0', '1', '2', '3', '4', '5', '6', '7', '8', '9', 'a', 'b', 'c', 'd', 'e', 'f',
}
