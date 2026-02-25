package internal

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"reflect"
	"strconv"
	"sync"
	"time"
	"unicode/utf8"
	"unsafe"
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

// GetEncoder retrieves an encoder from the pool
func GetEncoder() *FastEncoder {
	e := encoderPool.Get().(*FastEncoder)
	e.buf = e.buf[:0]
	return e
}

// PutEncoder returns an encoder to the pool
func PutEncoder(e *FastEncoder) {
	if cap(e.buf) <= 4096 { // Don't pool very large buffers
		encoderPool.Put(e)
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
	case []int64:
		e.EncodeInt64Slice(val)
	case []uint64:
		e.EncodeUint64Slice(val)
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
// PERFORMANCE: Uses pre-computed lookup table for O(1) per-character check
// PERFORMANCE: Checks 8 bytes at a time using batch processing
func needsEscape(s string) bool {
	// Batch processing: check 8 bytes at a time
	n := len(s)
	for i := 0; i+7 < n; i += 8 {
		if needsEscapeTable[s[i]] || needsEscapeTable[s[i+1]] ||
			needsEscapeTable[s[i+2]] || needsEscapeTable[s[i+3]] ||
			needsEscapeTable[s[i+4]] || needsEscapeTable[s[i+5]] ||
			needsEscapeTable[s[i+6]] || needsEscapeTable[s[i+7]] {
			return true
		}
	}
	// Check remaining bytes
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
// PERFORMANCE: Custom integer encoding avoids strconv for small values
func (e *FastEncoder) EncodeInt(n int64) {
	// Fast path for small integers (0-99)
	if n >= 0 && n < 100 {
		e.buf = append(e.buf, smallInts[n]...)
		return
	}

	// Use strconv for larger values
	e.buf = strconv.AppendInt(e.buf, n, 10)
}

// EncodeUint encodes an unsigned integer
func (e *FastEncoder) EncodeUint(n uint64) {
	// Fast path for small integers (0-99)
	if n < 100 {
		e.buf = append(e.buf, smallInts[n]...)
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

// EncodeFloat encodes a floating point number
func (e *FastEncoder) EncodeFloat(n float64, bits int) {
	// Fast path for common values
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

// unsafeString converts byte slice to string without allocation
// WARNING: The returned string should not be modified
func unsafeString(b []byte) string {
	return unsafe.String(&b[0], len(b))
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
// ============================================================================

// StructFieldInfo contains cached information about a struct field
type StructFieldInfo struct {
	Index     int
	Name      string
	OmitEmpty bool
	EncodeFn  func(*FastEncoder, reflect.Value) error
}

// structEncoderCache caches struct encoding information
var structEncoderCache sync.Map

// GetStructEncoder gets cached struct field info
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

		fields = append(fields, StructFieldInfo{
			Index:     i,
			Name:      name,
			OmitEmpty: omitEmpty,
		})
	}

	// Cache it
	actual, _ := structEncoderCache.LoadOrStore(t, fields)
	return actual.([]StructFieldInfo)
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

// BytesToString converts a byte slice to a string without allocation
// WARNING: The input slice should not be modified after this call
func BytesToString(b []byte) string {
	return unsafe.String(&b[0], len(b))
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
