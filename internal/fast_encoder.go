package internal

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"math"
	"reflect"
	"strconv"
	"sync"
	"time"
	"unicode/utf8"
)

// ============================================================================
// LOOKUP TABLES FOR STRING ESCAPING
// PERFORMANCE: Pre-computed lookup table avoids byte-by-byte checks
// ============================================================================

// needsEscapeTable is a pre-computed lookup table for characters that need escaping
// Index is the byte value, value is true if escaping is needed
var needsEscapeTable = [256]bool{
	// Control characters (0x00-0x1F) need escaping
	0x00: true, 0x01: true, 0x02: true, 0x03: true, 0x04: true, 0x05: true, 0x06: true, 0x07: true,
	0x08: true, 0x09: true, 0x0A: true, 0x0B: true, 0x0C: true, 0x0D: true, 0x0E: true, 0x0F: true,
	0x10: true, 0x11: true, 0x12: true, 0x13: true, 0x14: true, 0x15: true, 0x16: true, 0x17: true,
	0x18: true, 0x19: true, 0x1A: true, 0x1B: true, 0x1C: true, 0x1D: true, 0x1E: true, 0x1F: true,
	// Quote and backslash need escaping
	'"':  true,
	'\\': true,
	// All other characters (0x20-0xFF except " and \) are safe
}

// Note: hexChars, StringToBytes, BufferPool are defined in encoding.go

// ============================================================================
// FAST JSON ENCODER
// Provides encoding without reflection for common types
// PERFORMANCE: 2-5x faster than encoding/json for simple types
// ============================================================================

// FastEncoder provides fast JSON encoding without reflection for common types
type FastEncoder struct {
	buf []byte
}

// encoderPool pools encoder objects to reduce allocations
var encoderPool = sync.Pool{
	New: func() any {
		return &FastEncoder{
			buf: make([]byte, 0, 256),
		}
	},
}

// mediumEncoderPool for buffers between 1KB and 4KB
// PERFORMANCE: Better size matching for medium-sized encodings
var mediumEncoderPool = sync.Pool{
	New: func() any {
		return &FastEncoder{
			buf: make([]byte, 0, 2048),
		}
	},
}

// largeEncoderPool for buffers between 4KB and 64KB
// SECURITY FIX: Tiered pools prevent memory bloat from large buffers
var largeEncoderPool = sync.Pool{
	New: func() any {
		return &FastEncoder{
			buf: make([]byte, 0, 8192),
		}
	},
}

// GetEncoder retrieves an encoder from the pool
func GetEncoder() *FastEncoder {
	e := encoderPool.Get().(*FastEncoder)
	e.buf = e.buf[:0]
	return e
}

// GetEncoderWithSize retrieves an encoder with appropriate capacity hint
// PERFORMANCE: Use tiered pools for better memory management and reduced allocations
func GetEncoderWithSize(hint int) *FastEncoder {
	switch {
	case hint <= 1024:
		return GetEncoder()
	case hint <= 4096:
		e := mediumEncoderPool.Get().(*FastEncoder)
		e.buf = e.buf[:0]
		return e
	case hint <= 65536:
		e := largeEncoderPool.Get().(*FastEncoder)
		e.buf = e.buf[:0]
		return e
	default:
		// For very large hints, use large pool but buffer will be discarded
		e := largeEncoderPool.Get().(*FastEncoder)
		e.buf = e.buf[:0]
		return e
	}
}

// PutEncoder returns an encoder to the appropriate pool
// PERFORMANCE: Use tiered pools - buffers > 64KB are discarded to prevent memory bloat
func PutEncoder(e *FastEncoder) {
	if e == nil {
		return
	}
	c := cap(e.buf)
	switch {
	case c <= 1024:
		e.buf = e.buf[:0]
		encoderPool.Put(e)
	case c <= 4096:
		e.buf = e.buf[:0]
		mediumEncoderPool.Put(e)
	case c <= 65536: // 64KB threshold
		e.buf = e.buf[:0]
		largeEncoderPool.Put(e)
	// Buffers larger than 64KB are discarded - let GC handle them
	default:
		// Intentionally not pooled to prevent memory bloat
	}
}

// Bytes returns the encoded bytes
func (e *FastEncoder) Bytes() []byte {
	return e.buf
}

// Reset clears the encoder buffer
func (e *FastEncoder) Reset() {
	e.buf = e.buf[:0]
}

// EncodeValue encodes any value to JSON
// Uses fast paths for common types, falls back to stdlib for complex types
func (e *FastEncoder) EncodeValue(v any) error {
	if v == nil {
		e.buf = append(e.buf, "null"...)
		return nil
	}

	switch val := v.(type) {
	case string:
		e.EncodeString(val)
	case int:
		e.EncodeInt(int64(val))
	case int8:
		e.EncodeInt(int64(val))
	case int16:
		e.EncodeInt(int64(val))
	case int32:
		e.EncodeInt(int64(val))
	case int64:
		e.EncodeInt(val)
	case uint:
		e.EncodeUint(uint64(val))
	case uint8:
		e.EncodeUint(uint64(val))
	case uint16:
		e.EncodeUint(uint64(val))
	case uint32:
		e.EncodeUint(uint64(val))
	case uint64:
		e.EncodeUint(val)
	case float32:
		e.EncodeFloat(float64(val), 32)
	case float64:
		e.EncodeFloat(val, 64)
	case bool:
		e.EncodeBool(val)
	case time.Time:
		e.EncodeTime(val)
	case []byte:
		e.EncodeBase64(val)
	case map[string]any:
		return e.EncodeMap(val)
	case map[string]string:
		return e.EncodeMapStringString(val)
	case map[string]int:
		return e.EncodeMapStringInt(val)
	case map[string]int64:
		return e.EncodeMapStringInt64(val)
	case map[string]float64:
		return e.EncodeMapStringFloat64(val)
	case []any:
		return e.EncodeArray(val)
	case []string:
		e.EncodeStringSlice(val)
	case []int:
		e.EncodeIntSlice(val)
	case []int32:
		e.EncodeInt32Slice(val)
	case []int64:
		e.EncodeInt64Slice(val)
	case []uint64:
		e.EncodeUint64Slice(val)
	case []float32:
		e.EncodeFloat32Slice(val)
	case []float64:
		e.EncodeFloatSlice(val)
	case json.Number:
		e.buf = append(e.buf, val...)
	case json.RawMessage:
		e.buf = append(e.buf, val...)
	default:
		// Fallback to stdlib for complex types
		return e.encodeSlow(v)
	}
	return nil
}

// encodeSlow uses standard library for complex types
func (e *FastEncoder) encodeSlow(v any) error {
	// Use stdlib marshal
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	e.buf = append(e.buf, data...)
	return nil
}

// EncodeString encodes a JSON string
// PERFORMANCE: Avoids reflection, uses inline escaping
func (e *FastEncoder) EncodeString(s string) {
	e.buf = append(e.buf, '"')

	// Fast path: check if escaping is needed
	if !needsEscape(s) {
		e.buf = append(e.buf, s...)
		e.buf = append(e.buf, '"')
		return
	}

	// Slow path: escape special characters
	e.escapeString(s)
	e.buf = append(e.buf, '"')
}

// needsEscape checks if a string needs JSON escaping
// PERFORMANCE: Uses SWAR (SIMD Within A Register) technique for batch processing
// PERFORMANCE: First checks if any byte is potentially problematic using bit operations
func needsEscape(s string) bool {
	n := len(s)
	if n == 0 {
		return false
	}

	// Process 8 bytes at a time using SWAR technique
	for i := 0; i+8 <= n; i += 8 {
		// Load 8 bytes
		b0, b1, b2, b3 := s[i], s[i+1], s[i+2], s[i+3]
		b4, b5, b6, b7 := s[i+4], s[i+5], s[i+6], s[i+7]

		// Check for control characters (< 0x20) using bit manipulation
		// A byte is a control char if (byte - 0x20) has the sign bit set
		// We use: if byte < 0x20, then (byte - 0x20) & 0x80 != 0
		ctrlMask := ((b0 - 0x20) & 0x80) | ((b1 - 0x20) & 0x80) | ((b2 - 0x20) & 0x80) | ((b3 - 0x20) & 0x80) |
			((b4 - 0x20) & 0x80) | ((b5 - 0x20) & 0x80) | ((b6 - 0x20) & 0x80) | ((b7 - 0x20) & 0x80)

		// Quick check for common safe case: no control chars, no quotes, no backslashes
		// This avoids individual comparisons in the common case
		if ctrlMask != 0 ||
			b0 == '"' || b1 == '"' || b2 == '"' || b3 == '"' ||
			b4 == '"' || b5 == '"' || b6 == '"' || b7 == '"' ||
			b0 == '\\' || b1 == '\\' || b2 == '\\' || b3 == '\\' ||
			b4 == '\\' || b5 == '\\' || b6 == '\\' || b7 == '\\' {
			// Found potential escape needed, verify with table lookup
			if needsEscapeTable[b0] || needsEscapeTable[b1] || needsEscapeTable[b2] || needsEscapeTable[b3] ||
				needsEscapeTable[b4] || needsEscapeTable[b5] || needsEscapeTable[b6] || needsEscapeTable[b7] {
				return true
			}
		}
	}

	// Check remaining bytes (less than 8)
	for i := n &^ 7; i < n; i++ {
		if needsEscapeTable[s[i]] {
			return true
		}
	}

	return false
}

// escapeString escapes special characters for JSON
func (e *FastEncoder) escapeString(s string) {
	// Process byte by byte for proper UTF-8 handling
	start := 0
	for i := 0; i < len(s); {
		c := s[i]
		if c >= 0x20 && c != '"' && c != '\\' {
			i++
			continue
		}

		// Append the safe portion
		if start < i {
			e.buf = append(e.buf, s[start:i]...)
		}

		// Escape the special character
		switch c {
		case '"':
			e.buf = append(e.buf, '\\', '"')
		case '\\':
			e.buf = append(e.buf, '\\', '\\')
		case '\b':
			e.buf = append(e.buf, '\\', 'b')
		case '\f':
			e.buf = append(e.buf, '\\', 'f')
		case '\n':
			e.buf = append(e.buf, '\\', 'n')
		case '\r':
			e.buf = append(e.buf, '\\', 'r')
		case '\t':
			e.buf = append(e.buf, '\\', 't')
		default:
			// Control characters
			e.buf = append(e.buf, '\\', 'u', '0', '0')
			e.buf = appendHex(e.buf, c)
		}

		i++
		start = i
	}

	// Append remaining safe portion
	if start < len(s) {
		e.buf = append(e.buf, s[start:]...)
	}
}

// appendHex appends a two-digit hex representation
func appendHex(buf []byte, c byte) []byte {
	buf = append(buf, hexChars[c>>4])
	buf = append(buf, hexChars[c&0x0f])
	return buf
}

// EncodeInt encodes an integer
// PERFORMANCE: Uses pre-computed lookup tables for integers -999 to 9999
func (e *FastEncoder) EncodeInt(n int64) {
	// Fast path for small positive integers (0-99)
	if n >= 0 && n < 100 {
		e.buf = append(e.buf, smallInts[n]...)
		return
	}

	// Fast path for medium positive integers (100-999)
	if n >= 100 && n < 1000 {
		e.buf = append(e.buf, mediumInts[n-100]...)
		return
	}

	// Fast path for large positive integers (1000-9999)
	if n >= 1000 && n < 10000 {
		e.buf = append(e.buf, largeInts[n-1000]...)
		return
	}

	// Fast path for small negative integers (-1 to -99)
	if n < 0 && n > -100 {
		e.buf = append(e.buf, smallIntsNeg[-n]...)
		return
	}

	// Fast path for medium negative integers (-100 to -999)
	if n <= -100 && n > -1000 {
		e.buf = append(e.buf, mediumIntsNeg[-n-100]...)
		return
	}

	// Use strconv for larger values
	e.buf = strconv.AppendInt(e.buf, n, 10)
}

// EncodeUint encodes an unsigned integer
// PERFORMANCE: Uses pre-computed lookup tables for integers 0-9999
func (e *FastEncoder) EncodeUint(n uint64) {
	// Fast path for small integers (0-99)
	if n < 100 {
		e.buf = append(e.buf, smallInts[n]...)
		return
	}

	// Fast path for medium integers (100-999)
	if n < 1000 {
		e.buf = append(e.buf, mediumInts[n-100]...)
		return
	}

	// Fast path for large integers (1000-9999)
	if n < 10000 {
		e.buf = append(e.buf, largeInts[n-1000]...)
		return
	}

	// Use strconv for larger values
	e.buf = strconv.AppendUint(e.buf, n, 10)
}

// Pre-computed string representations of integers 0-99
var smallInts = [100][]byte{
	[]byte("0"), []byte("1"), []byte("2"), []byte("3"), []byte("4"),
	[]byte("5"), []byte("6"), []byte("7"), []byte("8"), []byte("9"),
	[]byte("10"), []byte("11"), []byte("12"), []byte("13"), []byte("14"),
	[]byte("15"), []byte("16"), []byte("17"), []byte("18"), []byte("19"),
	[]byte("20"), []byte("21"), []byte("22"), []byte("23"), []byte("24"),
	[]byte("25"), []byte("26"), []byte("27"), []byte("28"), []byte("29"),
	[]byte("30"), []byte("31"), []byte("32"), []byte("33"), []byte("34"),
	[]byte("35"), []byte("36"), []byte("37"), []byte("38"), []byte("39"),
	[]byte("40"), []byte("41"), []byte("42"), []byte("43"), []byte("44"),
	[]byte("45"), []byte("46"), []byte("47"), []byte("48"), []byte("49"),
	[]byte("50"), []byte("51"), []byte("52"), []byte("53"), []byte("54"),
	[]byte("55"), []byte("56"), []byte("57"), []byte("58"), []byte("59"),
	[]byte("60"), []byte("61"), []byte("62"), []byte("63"), []byte("64"),
	[]byte("65"), []byte("66"), []byte("67"), []byte("68"), []byte("69"),
	[]byte("70"), []byte("71"), []byte("72"), []byte("73"), []byte("74"),
	[]byte("75"), []byte("76"), []byte("77"), []byte("78"), []byte("79"),
	[]byte("80"), []byte("81"), []byte("82"), []byte("83"), []byte("84"),
	[]byte("85"), []byte("86"), []byte("87"), []byte("88"), []byte("89"),
	[]byte("90"), []byte("91"), []byte("92"), []byte("93"), []byte("94"),
	[]byte("95"), []byte("96"), []byte("97"), []byte("98"), []byte("99"),
}

// mediumInts contains pre-computed string representations of integers 100-999
// PERFORMANCE: Avoids strconv.AppendInt for common integer range
var mediumInts [900][]byte

// largeInts contains pre-computed string representations of integers 1000-9999
// PERFORMANCE: Avoids strconv.AppendInt for common 4-digit integer range
var largeInts [9000][]byte

// smallIntsNeg contains pre-computed string representations of negative integers -1 to -99
var smallIntsNeg [100][]byte

// mediumIntsNeg contains pre-computed string representations of negative integers -100 to -999
// PERFORMANCE: Avoids strconv.AppendInt for common negative 3-digit integer range
var mediumIntsNeg [900][]byte

// init initializes the medium and negative integer lookup tables and common floats
func init() {
	// Initialize medium integers (100-999)
	for i := 0; i < 900; i++ {
		n := i + 100
		mediumInts[i] = []byte(
			string(byte('0'+n/100)) +
				string(byte('0'+(n/10)%10)) +
				string(byte('0'+n%10)),
		)
	}

	// Initialize large integers (1000-9999)
	for i := 0; i < 9000; i++ {
		n := i + 1000
		largeInts[i] = []byte(
			string(byte('0'+n/1000)) +
				string(byte('0'+(n/100)%10)) +
				string(byte('0'+(n/10)%10)) +
				string(byte('0'+n%10)),
		)
	}

	// Initialize negative integers (-1 to -99)
	smallIntsNeg[0] = []byte{} // unused (index 0)
	for i := 1; i < 100; i++ {
		smallIntsNeg[i] = []byte("-" + string(smallInts[i]))
	}

	// Initialize medium negative integers (-100 to -999)
	for i := 0; i < 900; i++ {
		mediumIntsNeg[i] = []byte("-" + string(mediumInts[i]))
	}

	// Initialize common floats table
	// These are the most frequently used float values in JSON
	commonFloats = map[uint64][]byte{
		// Common positive values
		0x3ff0000000000000: []byte("1"),    // 1.0
		0x4000000000000000: []byte("2"),    // 2.0
		0x4008000000000000: []byte("3"),    // 3.0
		0x4010000000000000: []byte("4"),    // 4.0
		0x4014000000000000: []byte("5"),    // 5.0
		0x4018000000000000: []byte("6"),    // 6.0
		0x401c000000000000: []byte("7"),    // 7.0
		0x4020000000000000: []byte("8"),    // 8.0
		0x4022000000000000: []byte("9"),    // 9.0
		0x4024000000000000: []byte("10"),   // 10.0
		0x3fe0000000000000: []byte("0.5"),  // 0.5
		0x3fc999999999999a: []byte("0.2"),  // 0.2
		0x3fb999999999999a: []byte("0.1"),  // 0.1
		0x3fd3333333333333: []byte("0.3"),  // 0.3
		0x3fd999999999999a: []byte("0.4"),  // 0.4
		0x3fe3333333333333: []byte("0.6"),  // 0.6
		0x3fe6666666666666: []byte("0.7"),  // 0.7
		0x3feccccccccccccd: []byte("0.8"),  // 0.8
		0x3ff199999999999a: []byte("1.1"),  // 1.1
		0x3ff3333333333333: []byte("1.2"),  // 1.2
		0x3ff4cccccccccccd: []byte("1.3"),  // 1.3
		0x3ff6666666666666: []byte("1.4"),  // 1.4
		0x3ff8000000000000: []byte("1.5"),  // 1.5
		0x3ff999999999999a: []byte("1.6"),  // 1.6
		0x3ffb333333333333: []byte("1.7"),  // 1.7
		0x3ffccccccccccccd: []byte("1.8"),  // 1.8
		0x3ffe666666666666: []byte("1.9"),  // 1.9
		0x400199999999999a: []byte("2.5"),  // 2.5
		0x403e000000000000: []byte("30"),   // 30.0
		0x4049000000000000: []byte("50"),   // 50.0
		0x4059000000000000: []byte("100"),  // 100.0
		0x408f400000000000: []byte("1000"), // 1000.0
		// Additional powers of 10
		0x40c3880000000000: []byte("10000"), // 10000.0
		// Common fractions
		0x3fd0000000000000: []byte("0.25"),  // 0.25
		0x3fe4000000000000: []byte("0.75"),  // 0.75
		0x3fc0000000000000: []byte("0.125"), // 0.125
		0x3fd8000000000000: []byte("0.375"), // 0.375
		// Common percentages
		0x3f847ae147ae147b: []byte("0.01"), // 0.01
		0x3fa999999999999a: []byte("0.05"), // 0.05
		0x3fc3333333333333: []byte("0.15"), // 0.15
		// Small decimals
		0x3f50624dd2f1a9fc: []byte("0.001"), // 0.001
		// Common negative values
		0xbff0000000000000: []byte("-1"),    // -1.0
		0xc000000000000000: []byte("-2"),    // -2.0
		0xc008000000000000: []byte("-3"),    // -3.0
		0xc010000000000000: []byte("-4"),    // -4.0
		0xc014000000000000: []byte("-5"),    // -5.0
		0xbfe0000000000000: []byte("-0.5"),  // -0.5
		0xbfb999999999999a: []byte("-0.1"),  // -0.1
		0xc024000000000000: []byte("-10"),   // -10.0
		0xc059000000000000: []byte("-100"),  // -100.0
		0xc08f400000000000: []byte("-1000"), // -1000.0
	}
}

// commonFloats contains pre-computed string representations of common float values
// PERFORMANCE: Avoids strconv.AppendFloat for frequently used floats
var commonFloats map[uint64][]byte

// float64ToBits converts float64 to its IEEE 754 binary representation
// PERFORMANCE: Uses math.Float64bits for zero-allocation conversion
func float64ToBits(f float64) uint64 {
	return math.Float64bits(f)
}

// isIntegerFloat checks if a float64 is an integer value
// PERFORMANCE: Fast check for integer-like floats that can use integer encoding
func isIntegerFloat(f float64) bool {
	// Fast path: check if the float is an integer
	// A float is an integer if f == float64(int64(f))
	// But we need to handle large values that don't fit in int64
	if f < 0 {
		return f == float64(int64(f)) && f >= -9007199254740992
	}
	return f == float64(int64(f)) && f <= 9007199254740992
}

// EncodeFloat encodes a floating point number
// PERFORMANCE: Uses pre-computed common values and fast integer conversion
func (e *FastEncoder) EncodeFloat(n float64, bits int) {
	// Fast path for zero
	if n == 0 {
		e.buf = append(e.buf, '0')
		return
	}

	// Check for special values
	if n != n { // NaN
		e.buf = append(e.buf, "NaN"...)
		return
	}
	if n > 0 && n/2 == n { // +Inf
		e.buf = append(e.buf, "Infinity"...)
		return
	}
	if n < 0 && n/2 == n { // -Inf
		e.buf = append(e.buf, "-Infinity"...)
		return
	}

	// PERFORMANCE: Fast path for common float values
	bits64 := float64ToBits(n)
	if cached, ok := commonFloats[bits64]; ok {
		e.buf = append(e.buf, cached...)
		return
	}

	// PERFORMANCE: Fast path for small integer floats (0-100)
	// These can be encoded using the smallInts lookup table
	if n >= 0 && n <= 100 {
		intVal := int64(n)
		if n == float64(intVal) {
			e.buf = append(e.buf, smallInts[intVal]...)
			return
		}
	}

	// PERFORMANCE: Fast path for integer-like floats
	// These can be encoded more efficiently as integers
	if isIntegerFloat(n) {
		if n < 0 {
			e.EncodeInt(int64(n))
		} else {
			e.EncodeUint(uint64(n))
		}
		return
	}

	// Use strconv for general case
	e.buf = strconv.AppendFloat(e.buf, n, 'f', -1, bits)
}

// EncodeBool encodes a boolean
func (e *FastEncoder) EncodeBool(b bool) {
	if b {
		e.buf = append(e.buf, "true"...)
	} else {
		e.buf = append(e.buf, "false"...)
	}
}

// EncodeMap encodes a map[string]any
func (e *FastEncoder) EncodeMap(m map[string]any) error {
	e.buf = append(e.buf, '{')
	first := true

	for k, v := range m {
		if !first {
			e.buf = append(e.buf, ',')
		}
		first = false

		e.EncodeString(k)
		e.buf = append(e.buf, ':')

		if err := e.EncodeValue(v); err != nil {
			return err
		}
	}

	e.buf = append(e.buf, '}')
	return nil
}

// EncodeMapStringString encodes a map[string]string
func (e *FastEncoder) EncodeMapStringString(m map[string]string) error {
	e.buf = append(e.buf, '{')
	first := true

	for k, v := range m {
		if !first {
			e.buf = append(e.buf, ',')
		}
		first = false

		e.EncodeString(k)
		e.buf = append(e.buf, ':')
		e.EncodeString(v)
	}

	e.buf = append(e.buf, '}')
	return nil
}

// EncodeMapStringInt encodes a map[string]int
func (e *FastEncoder) EncodeMapStringInt(m map[string]int) error {
	e.buf = append(e.buf, '{')
	first := true

	for k, v := range m {
		if !first {
			e.buf = append(e.buf, ',')
		}
		first = false

		e.EncodeString(k)
		e.buf = append(e.buf, ':')
		e.EncodeInt(int64(v))
	}

	e.buf = append(e.buf, '}')
	return nil
}

// EncodeArray encodes a []any
func (e *FastEncoder) EncodeArray(arr []any) error {
	e.buf = append(e.buf, '[')

	for i, v := range arr {
		if i > 0 {
			e.buf = append(e.buf, ',')
		}
		if err := e.EncodeValue(v); err != nil {
			return err
		}
	}

	e.buf = append(e.buf, ']')
	return nil
}

// EncodeStringSlice encodes a []string
func (e *FastEncoder) EncodeStringSlice(arr []string) {
	e.buf = append(e.buf, '[')

	for i, v := range arr {
		if i > 0 {
			e.buf = append(e.buf, ',')
		}
		e.EncodeString(v)
	}

	e.buf = append(e.buf, ']')
}

// EncodeIntSlice encodes a []int
func (e *FastEncoder) EncodeIntSlice(arr []int) {
	e.buf = append(e.buf, '[')

	for i, v := range arr {
		if i > 0 {
			e.buf = append(e.buf, ',')
		}
		e.EncodeInt(int64(v))
	}

	e.buf = append(e.buf, ']')
}

// EncodeFloatSlice encodes a []float64
func (e *FastEncoder) EncodeFloatSlice(arr []float64) {
	e.buf = append(e.buf, '[')

	for i, v := range arr {
		if i > 0 {
			e.buf = append(e.buf, ',')
		}
		e.EncodeFloat(v, 64)
	}

	e.buf = append(e.buf, ']')
}

// EncodeInt32Slice encodes a []int32
// PERFORMANCE: Specialized encoder avoids interface conversion overhead
func (e *FastEncoder) EncodeInt32Slice(arr []int32) {
	e.buf = append(e.buf, '[')

	for i, v := range arr {
		if i > 0 {
			e.buf = append(e.buf, ',')
		}
		e.EncodeInt(int64(v))
	}

	e.buf = append(e.buf, ']')
}

// EncodeFloat32Slice encodes a []float32
// PERFORMANCE: Specialized encoder avoids interface conversion overhead
func (e *FastEncoder) EncodeFloat32Slice(arr []float32) {
	e.buf = append(e.buf, '[')

	for i, v := range arr {
		if i > 0 {
			e.buf = append(e.buf, ',')
		}
		e.EncodeFloat(float64(v), 32)
	}

	e.buf = append(e.buf, ']')
}

// ============================================================================
// EXTENDED TYPE ENCODERS
// PERFORMANCE: Specialized encoders avoid reflection overhead
// ============================================================================

// EncodeTime encodes a time.Time in RFC3339 format
func (e *FastEncoder) EncodeTime(t time.Time) {
	e.buf = append(e.buf, '"')
	e.buf = append(e.buf, t.Format(time.RFC3339)...)
	e.buf = append(e.buf, '"')
}

// EncodeBase64 encodes a []byte as base64 string
func (e *FastEncoder) EncodeBase64(b []byte) {
	e.buf = append(e.buf, '"')
	encoded := base64.StdEncoding.EncodeToString(b)
	e.buf = append(e.buf, encoded...)
	e.buf = append(e.buf, '"')
}

// EncodeMapStringInt64 encodes a map[string]int64
func (e *FastEncoder) EncodeMapStringInt64(m map[string]int64) error {
	e.buf = append(e.buf, '{')
	first := true

	for k, v := range m {
		if !first {
			e.buf = append(e.buf, ',')
		}
		first = false

		e.EncodeString(k)
		e.buf = append(e.buf, ':')
		e.EncodeInt(v)
	}

	e.buf = append(e.buf, '}')
	return nil
}

// EncodeMapStringFloat64 encodes a map[string]float64
func (e *FastEncoder) EncodeMapStringFloat64(m map[string]float64) error {
	e.buf = append(e.buf, '{')
	first := true

	for k, v := range m {
		if !first {
			e.buf = append(e.buf, ',')
		}
		first = false

		e.EncodeString(k)
		e.buf = append(e.buf, ':')
		e.EncodeFloat(v, 64)
	}

	e.buf = append(e.buf, '}')
	return nil
}

// EncodeInt64Slice encodes a []int64
func (e *FastEncoder) EncodeInt64Slice(arr []int64) {
	e.buf = append(e.buf, '[')

	for i, v := range arr {
		if i > 0 {
			e.buf = append(e.buf, ',')
		}
		e.EncodeInt(v)
	}

	e.buf = append(e.buf, ']')
}

// EncodeUint64Slice encodes a []uint64
func (e *FastEncoder) EncodeUint64Slice(arr []uint64) {
	e.buf = append(e.buf, '[')

	for i, v := range arr {
		if i > 0 {
			e.buf = append(e.buf, ',')
		}
		e.EncodeUint(v)
	}

	e.buf = append(e.buf, ']')
}

// ============================================================================
// FAST DECODING UTILITIES
// ============================================================================

// FastParseInt parses an integer from a byte slice
// PERFORMANCE: Avoids string allocation by parsing directly from bytes
func FastParseInt(b []byte) (int64, error) {
	if len(b) == 0 {
		return 0, strconv.ErrSyntax
	}

	// Handle sign
	neg := false
	start := 0
	if b[0] == '-' {
		neg = true
		start = 1
		if len(b) == 1 {
			return 0, strconv.ErrSyntax
		}
	}

	// Parse digits
	var n int64
	for i := start; i < len(b); i++ {
		c := b[i]
		if c < '0' || c > '9' {
			return 0, strconv.ErrSyntax
		}
		// Check overflow
		if n > (1<<63-1)/10 {
			return 0, strconv.ErrRange
		}
		n = n*10 + int64(c-'0')
	}

	if neg {
		n = -n
	}

	return n, nil
}

// FastParseFloat parses a float from a byte slice
func FastParseFloat(b []byte) (float64, error) {
	// For now, use strconv (float parsing is complex)
	return strconv.ParseFloat(unsafeString(b), 64)
}

// unsafeString converts byte slice to string.
// SECURITY FIX: Uses standard library conversion for safety.
// Go 1.20+ optimizes this conversion in most cases, making unsafe unnecessary.
// The previous unsafe implementation could cause data corruption if the input
// slice was modified after the string was created.
func unsafeString(b []byte) string {
	return string(b)
}

// ============================================================================
// FAST MARSHAL/UNMARSHAL FUNCTIONS
// ============================================================================

// FastMarshal marshals a value to JSON using the fast encoder
func FastMarshal(v any) ([]byte, error) {
	e := GetEncoder()
	defer PutEncoder(e)

	err := e.EncodeValue(v)
	if err != nil {
		return nil, err
	}

	// Make a copy since encoder buffer is reused
	result := make([]byte, len(e.buf))
	copy(result, e.buf)
	return result, nil
}

// FastMarshalToString marshals a value to a JSON string
func FastMarshalToString(v any) (string, error) {
	e := GetEncoder()
	defer PutEncoder(e)

	err := e.EncodeValue(v)
	if err != nil {
		return "", err
	}

	return string(e.buf), nil
}

// ============================================================================
// STRUCT ENCODER CACHE
// Caches struct field information for faster encoding
// PERFORMANCE: Type-specific encoding functions avoid reflection overhead
// ============================================================================

// StructFieldInfo contains cached information about a struct field
type StructFieldInfo struct {
	Index     int
	Name      string
	OmitEmpty bool
	EncodeFn  func(*FastEncoder, reflect.Value) error // Type-specific encoder
	Offset    uintptr                                 // Field offset for direct access
	Type      reflect.Type                            // Cached type information
	IsPointer bool                                    // Whether the field is a pointer type
}

// structEncoderCache caches struct encoding information
var structEncoderCache sync.Map

// GetStructEncoder gets cached struct field info
// PERFORMANCE: Generates type-specific encoding functions for known types
func GetStructEncoder(t reflect.Type) []StructFieldInfo {
	if v, ok := structEncoderCache.Load(t); ok {
		return v.([]StructFieldInfo)
	}

	// Build field info
	fields := make([]StructFieldInfo, 0, t.NumField())
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if !f.IsExported() {
			continue
		}

		jsonTag := f.Tag.Get("json")
		name := f.Name
		omitEmpty := false

		if jsonTag != "" {
			parts := splitTag(jsonTag)
			if parts[0] != "" && parts[0] != "-" {
				name = parts[0]
			}
			for _, p := range parts[1:] {
				if p == "omitempty" {
					omitEmpty = true
				}
			}
		}

		info := StructFieldInfo{
			Index:     i,
			Name:      name,
			OmitEmpty: omitEmpty,
			Offset:    f.Offset,
			Type:      f.Type,
			IsPointer: f.Type.Kind() == reflect.Ptr,
			EncodeFn:  getEncodeFn(f.Type),
		}

		fields = append(fields, info)
	}

	// Cache it
	actual, _ := structEncoderCache.LoadOrStore(t, fields)
	return actual.([]StructFieldInfo)
}

// getEncodeFn returns a type-specific encoding function for the given type
// PERFORMANCE: Avoids reflection for common types by generating specialized encoders
func getEncodeFn(t reflect.Type) func(*FastEncoder, reflect.Value) error {
	// Handle pointer types
	if t.Kind() == reflect.Ptr {
		elemType := t.Elem()
		elemEncoder := getEncodeFn(elemType)
		return func(e *FastEncoder, v reflect.Value) error {
			if v.IsNil() {
				e.buf = append(e.buf, "null"...)
				return nil
			}
			return elemEncoder(e, v.Elem())
		}
	}

	// Generate type-specific encoders for primitive types
	switch t.Kind() {
	case reflect.String:
		return func(e *FastEncoder, v reflect.Value) error {
			e.EncodeString(v.String())
			return nil
		}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return func(e *FastEncoder, v reflect.Value) error {
			e.EncodeInt(v.Int())
			return nil
		}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return func(e *FastEncoder, v reflect.Value) error {
			e.EncodeUint(v.Uint())
			return nil
		}
	case reflect.Float32:
		return func(e *FastEncoder, v reflect.Value) error {
			e.EncodeFloat(v.Float(), 32)
			return nil
		}
	case reflect.Float64:
		return func(e *FastEncoder, v reflect.Value) error {
			e.EncodeFloat(v.Float(), 64)
			return nil
		}
	case reflect.Bool:
		return func(e *FastEncoder, v reflect.Value) error {
			e.EncodeBool(v.Bool())
			return nil
		}
	case reflect.Slice:
		// Check for byte slice
		if t.Elem().Kind() == reflect.Uint8 {
			return func(e *FastEncoder, v reflect.Value) error {
				if v.IsNil() {
					e.buf = append(e.buf, "null"...)
					return nil
				}
				e.EncodeBase64(v.Bytes())
				return nil
			}
		}
		// Fall through to generic handling
	case reflect.Map:
		// Check for common map types
		if t.Key().Kind() == reflect.String {
			switch t.Elem().Kind() {
			case reflect.String:
				return func(e *FastEncoder, v reflect.Value) error {
					if v.IsNil() {
						e.buf = append(e.buf, "null"...)
						return nil
					}
					return e.EncodeMapStringString(v.Interface().(map[string]string))
				}
			case reflect.Int:
				return func(e *FastEncoder, v reflect.Value) error {
					if v.IsNil() {
						e.buf = append(e.buf, "null"...)
						return nil
					}
					return e.EncodeMapStringInt(v.Interface().(map[string]int))
				}
			case reflect.Int64:
				return func(e *FastEncoder, v reflect.Value) error {
					if v.IsNil() {
						e.buf = append(e.buf, "null"...)
						return nil
					}
					return e.EncodeMapStringInt64(v.Interface().(map[string]int64))
				}
			case reflect.Float64:
				return func(e *FastEncoder, v reflect.Value) error {
					if v.IsNil() {
						e.buf = append(e.buf, "null"...)
						return nil
					}
					return e.EncodeMapStringFloat64(v.Interface().(map[string]float64))
				}
			}
		}
		// Fall through to generic handling
	case reflect.Struct:
		// Cache nested struct encoder
		nestedFields := GetStructEncoder(t)
		return func(e *FastEncoder, v reflect.Value) error {
			e.buf = append(e.buf, '{')
			first := true
			for _, f := range nestedFields {
				fieldVal := v.Field(f.Index)
				if f.OmitEmpty && isEmptyValue(fieldVal) {
					continue
				}
				if !first {
					e.buf = append(e.buf, ',')
				}
				first = false
				e.EncodeString(f.Name)
				e.buf = append(e.buf, ':')
				if f.EncodeFn != nil {
					if err := f.EncodeFn(e, fieldVal); err != nil {
						return err
					}
				} else {
					if err := e.EncodeValue(fieldVal.Interface()); err != nil {
						return err
					}
				}
			}
			e.buf = append(e.buf, '}')
			return nil
		}
	}

	// Return nil for types that need generic handling
	// The caller will use EncodeValue for these
	return nil
}

// isEmptyValue checks if a value should be considered empty for omitempty
func isEmptyValue(v reflect.Value) bool {
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
	case reflect.Interface, reflect.Ptr:
		return v.IsNil()
	}
	return false
}

// splitTag splits a struct tag into parts
func splitTag(tag string) []string {
	var parts []string
	start := 0
	for i := 0; i <= len(tag); i++ {
		if i == len(tag) || tag[i] == ',' {
			if i > start {
				parts = append(parts, tag[start:i])
			}
			start = i + 1
		}
	}
	return parts
}

// ============================================================================
// ZERO-COPY STRING OPERATIONS
// ============================================================================

// BytesToString converts a byte slice to a string.
// SECURITY FIX: Uses standard library conversion for safety.
// Note: For performance-critical code where the caller guarantees the slice
// won't be modified, consider using a separate unsafe version with clear documentation.
func BytesToString(b []byte) string {
	return string(b)
}

// IsValidUTF8 checks if a byte slice is valid UTF-8
func IsValidUTF8(b []byte) bool {
	return utf8.Valid(b)
}

// ============================================================================
// ADDITIONAL BUFFER POOLS FOR ENCODING
// ============================================================================

// FastBufferPool is a pool of byte buffers for fast encoding
var FastBufferPool = sync.Pool{
	New: func() any {
		return bytes.NewBuffer(make([]byte, 0, 512))
	},
}

// GetFastBuffer gets a buffer from the pool
func GetFastBuffer() *bytes.Buffer {
	buf := FastBufferPool.Get().(*bytes.Buffer)
	buf.Reset()
	return buf
}

// PutFastBuffer returns a buffer to the pool
func PutFastBuffer(buf *bytes.Buffer) {
	if buf.Cap() <= 8192 { // Don't pool very large buffers
		FastBufferPool.Put(buf)
	}
}
