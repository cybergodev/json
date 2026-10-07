package internal

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"iter"
	"math"
	"reflect"
	"sort"
	"strconv"
	"sync"
	"time"
	"unicode/utf8"
)

// SortedEntries iterates the entries of m in lexicographic key order.
//
// encoding/json always emits object keys sorted; the fast encoder matches
// that so its output is deterministic and byte-compatible with the standard
// library (a map[string]any round-trip must not depend on Go's random map
// iteration order).
//
// PERFORMANCE: three size tiers (P-001, profile-driven):
//   - maps with at most one entry are ranged directly (no key slice at all);
//   - small maps (<= smallKeysBufSize keys) collect into a fixed-size stack
//     array — deterministic zero-allocation and, crucially, zero pool
//     traffic. Nested small objects are the dominant shape of real JSON, and
//     an earlier all-pool variant measurably REGRESSED encoding of documents
//     made of 100 two-key objects (+14% ns/op): every nested map paid a
//     pool Get/defer/Put cycle to avoid an allocation that escape analysis
//     had already made free;
//   - larger maps collect into a pooled buffer (see pools.go) — profiling
//     showed that per-encode keys allocation was ~30% of all allocated
//     objects on the Set/Delete path.
//
// range-over-func compiles the caller's loop body inline, so each tier costs
// the same as a plain loop.
func SortedEntries[V any](m map[string]V) iter.Seq2[string, V] {
	return func(yield func(string, V) bool) {
		if len(m) <= 1 {
			for k, v := range m {
				if !yield(k, v) {
					return
				}
			}
			return
		}
		if len(m) <= smallKeysBufSize {
			var buf [smallKeysBufSize]string
			keys := buf[:0]
			for k := range m {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				if !yield(k, m[k]) {
					return
				}
			}
			return
		}
		keysPtr := GetSortedKeysSlice(len(m))
		defer PutSortedKeysSlice(keysPtr)
		keys := *keysPtr
		for k := range m {
			keys = append(keys, k)
		}
		// append may have reallocated; write the final slice back so
		// PutSortedKeysSlice (and its cap check) sees the buffer we sorted.
		*keysPtr = keys
		sort.Strings(keys)
		for _, k := range keys {
			if !yield(k, m[k]) {
				return
			}
		}
	}
}

// ============================================================================
// LOOKUP TABLES FOR STRING ESCAPING
// PERFORMANCE: Pre-computed lookup table avoids byte-by-byte checks
// ============================================================================

// needsEscapeTable is a pre-computed lookup table for characters that need escaping
// Index is the byte value, value is true if escaping is needed
var needsEscapeTable = [256]bool{
	// Control characters (0x00-0x1F) need escaping per RFC 8259
	0x00: true, 0x01: true, 0x02: true, 0x03: true, 0x04: true, 0x05: true, 0x06: true, 0x07: true,
	0x08: true, 0x09: true, 0x0A: true, 0x0B: true, 0x0C: true, 0x0D: true, 0x0E: true, 0x0F: true,
	0x10: true, 0x11: true, 0x12: true, 0x13: true, 0x14: true, 0x15: true, 0x16: true, 0x17: true,
	0x18: true, 0x19: true, 0x1A: true, 0x1B: true, 0x1C: true, 0x1D: true, 0x1E: true, 0x1F: true,
	// Quote and backslash need escaping
	'"':  true,
	'\\': true,
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
	// depth tracks container nesting (set by EncodeMap/EncodeArray) so a
	// self-referential value (m["self"] = m) errors out instead of recursing
	// until the goroutine stack is exhausted - a fatal, unrecoverable crash.
	depth int
	// maxDepthCap overrides maxEncodeDepth when positive. The root package
	// sets it from Config.MaxDepth so the fast path enforces the SAME depth
	// cap as the custom encoder (D-002: the fast path previously always used
	// the package-wide 200, silently accepting trees the config'd path
	// rejects at MaxDepth=100). Zero/negative = package default.
	maxDepthCap int
}

// SetMaxEncodeDepth sets the per-encoder container-depth cap; n <= 0 resets
// to the package default. Stale caps never leak across pool users: both
// checkout (GetEncoder/GetEncoderWithSize) and return (PutEncoder) run
// resetState, which clears maxDepthCap and depth.
func (e *FastEncoder) SetMaxEncodeDepth(n int) {
	e.maxDepthCap = n
}

// effectiveMaxDepth returns the active depth cap for this encoder.
func (e *FastEncoder) effectiveMaxDepth() int {
	if e.maxDepthCap > 0 {
		return e.maxDepthCap
	}
	return maxEncodeDepth
}

// maxEncodeDepth bounds container nesting during encoding. MaxNestingDepth is
// the package-wide limit applied to decoding; encoding uses the same bound.
const maxEncodeDepth = MaxNestingDepth

// enterContainer increments the nesting depth and reports an error once it
// exceeds maxEncodeDepth (a possible reference cycle). leaveContainer pairs
// with it on the way out.
func (e *FastEncoder) enterContainer() error {
	e.depth++
	if limit := e.effectiveMaxDepth(); e.depth > limit {
		e.depth--
		return fmt.Errorf("json: unsupported value: nesting exceeds maximum depth %d (possible reference cycle)", limit)
	}
	return nil
}

// leaveContainer decrements the nesting depth set by enterContainer.
func (e *FastEncoder) leaveContainer() {
	e.depth--
}

// writeNull emits a JSON null literal.
func (e *FastEncoder) writeNull() {
	e.buf = append(e.buf, "null"...)
}

// encoderPool pools encoder objects to reduce allocations
var encoderPool = sync.Pool{
	New: func() any {
		return &FastEncoder{
			buf: make([]byte, 0, 512),
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
// Tiered pools prevent memory bloat from large buffers
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
	e.resetState()
	return e
}

// GetEncoderWithSize retrieves an encoder with appropriate capacity hint
// PERFORMANCE: Use tiered pools for better memory management and reduced allocations
//
// NOTE (D-002/R8 M5): no production caller — every site uses GetEncoder;
// retained for tests/future use.
func GetEncoderWithSize(hint int) *FastEncoder {
	var e *FastEncoder
	switch {
	case hint <= 1024:
		return GetEncoder()
	case hint <= 4096:
		e = mediumEncoderPool.Get().(*FastEncoder)
	case hint <= 65536:
		e = largeEncoderPool.Get().(*FastEncoder)
	default:
		e = largeEncoderPool.Get().(*FastEncoder)
	}
	e.resetState()
	return e
}

// PutEncoder returns an encoder to the appropriate pool
// PERFORMANCE: Use tiered pools - buffers > 64KB are discarded to prevent memory bloat
//
// The encoder's per-session state (depth, maxDepthCap) is cleared BEFORE
// pooling so pooled objects are always clean. Checkout-side resetState makes
// this belt-and-braces: the invariant "no stale caps or depths cross pool
// boundaries" now holds even if one side regresses.
func PutEncoder(e *FastEncoder) {
	if e == nil {
		return
	}
	e.resetState()
	c := cap(e.buf)
	switch {
	case c <= 1024:
		encoderPool.Put(e)
	case c <= 4096:
		mediumEncoderPool.Put(e)
	case c <= 65536: // 64KB threshold
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

// resetState clears ALL per-session encoder state: buffer, nesting depth,
// and the per-instance depth cap. It is the single source of truth for the
// pool's no-leak contract, run on both checkout and return.
//
// Depth matters here: an EncodeValue that fails mid-container skips the
// leaveContainer pairs, leaving depth elevated; resetState guarantees that
// residual never affects a subsequent encode.
func (e *FastEncoder) resetState() {
	e.buf = e.buf[:0]
	e.depth = 0
	e.maxDepthCap = 0
}

// Reset clears the encoder buffer ONLY — depth and maxDepthCap survive so a
// caller can reuse the encoder mid-session with its configured cap intact.
// For a from-scratch encoder (or after an error) prefer returning this one
// to the pool and checking out a fresh one via GetEncoder.
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
		return e.EncodeFloat(float64(val), 32)
	case float64:
		return e.EncodeFloat(val, 64)
	case bool:
		e.EncodeBool(val)
	case time.Time:
		return e.EncodeTime(val)
	case []byte:
		// encoding/json emits null for a nil slice, distinct from "" for an
		// empty non-nil one.
		if val == nil {
			e.writeNull()
			return nil
		}
		e.EncodeBase64(val)
	case map[string]any:
		if val == nil {
			e.writeNull()
			return nil
		}
		return e.EncodeMap(val)
	case map[string]string:
		if val == nil {
			e.writeNull()
			return nil
		}
		return e.EncodeMapStringString(val)
	case map[string]int:
		if val == nil {
			e.writeNull()
			return nil
		}
		return e.EncodeMapStringInt(val)
	case map[string]int64:
		if val == nil {
			e.writeNull()
			return nil
		}
		return e.EncodeMapStringInt64(val)
	case map[string]float64:
		if val == nil {
			e.writeNull()
			return nil
		}
		return e.EncodeMapStringFloat64(val)
	case []any:
		if val == nil {
			e.writeNull()
			return nil
		}
		return e.EncodeArray(val)
	case []string:
		if val == nil {
			e.writeNull()
			return nil
		}
		e.EncodeStringSlice(val)
	case []int:
		if val == nil {
			e.writeNull()
			return nil
		}
		e.EncodeIntSlice(val)
	case []int32:
		if val == nil {
			e.writeNull()
			return nil
		}
		e.EncodeInt32Slice(val)
	case []int64:
		if val == nil {
			e.writeNull()
			return nil
		}
		e.EncodeInt64Slice(val)
	case []uint64:
		if val == nil {
			e.writeNull()
			return nil
		}
		e.EncodeUint64Slice(val)
	case []float32:
		if val == nil {
			e.writeNull()
			return nil
		}
		// Non-finite elements are rejected by EncodeFloat itself (R11 m4); a
		// pre-scan would just duplicate that check on every clean encode.
		return e.EncodeFloat32Slice(val)
	case []float64:
		if val == nil {
			e.writeNull()
			return nil
		}
		// See []float32: EncodeFloat rejects non-finite values (R11 m4).
		return e.EncodeFloatSlice(val)
	case json.Number:
		// SECURITY: Validate json.Number content before appending
		// json.Number should only contain valid JSON number characters
		//
		// KNOWN LIMITATION (D-002/R10): the ROOT package's Number type (also a
		// string-kind literal holder, produced by PreserveNumbers parsing)
		// cannot be case-matched here — internal cannot import the root
		// package. It falls through to encodeSlow, which marshals it correctly
		// via its MarshalJSON (stdlib Marshaler contract) at per-value
		// reflection cost. Sniffing it via an interface case was rejected: a
		// bare MarshalJSON passthrough skips stdlib's compaction of marshaler
		// output and changes error wrapping for user types.
		if !IsValidJSONNumber(string(val)) {
			return fmt.Errorf("invalid json.Number: %s", string(val))
		}
		e.buf = append(e.buf, val...)
	case json.RawMessage:
		// SECURITY: Validate RawMessage is valid JSON before appending
		// This prevents malformed JSON from corrupting the output
		//
		// Match encoding/json: a nil RawMessage encodes as null; an empty
		// non-nil one is an error (appending zero bytes would emit invalid
		// JSON such as {"a":}).
		if len(val) == 0 {
			if val == nil {
				e.buf = append(e.buf, "null"...)
				return nil
			}
			return fmt.Errorf("json: error calling MarshalJSON for type json.RawMessage: unexpected end of JSON input")
		}
		if !json.Valid(val) {
			return fmt.Errorf("invalid json.RawMessage: not valid JSON")
		}
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
// PERFORMANCE: Avoids reflection, uses inline escaping with combined UTF-8 validation
// SECURITY: Validates UTF-8 encoding per RFC 8259. HTML escaping is NOT
// applied here — callers post-process with HTMLEscapeBytes when needed
// (FastEncoder has no escape-config field; the old comment referenced one).
func (e *FastEncoder) EncodeString(s string) {
	e.buf = append(e.buf, '"')

	// Fast path: combined check for escape needs and UTF-8 validity
	// SECURITY: RFC 8259 requires JSON strings to be valid UTF-8
	// PERFORMANCE: Single pass validation instead of separate needsEscape + utf8.ValidString
	if isSafeString(s) {
		e.buf = append(e.buf, s...)
		e.buf = append(e.buf, '"')
		return
	}

	// Slow path: escape special characters and validate/fix UTF-8
	e.escapeString(s)
	e.buf = append(e.buf, '"')
}

// isSafeString checks if a string is safe for direct JSON output (no escaping needed AND valid UTF-8)
// PERFORMANCE: Combined single-pass check with optimized paths for common cases
// Returns true only if the string:
// 1. Contains no characters that need JSON escaping (control chars, quotes, backslashes)
// 2. Is valid UTF-8 (all high bytes are part of valid multi-byte sequences)
func isSafeString(s string) bool {
	n := len(s)
	if n == 0 {
		return true
	}

	// PERFORMANCE: Fast path for short strings (<=8 bytes)
	// Avoids SWAR loop overhead for very common small strings
	if n <= 8 {
		for i := 0; i < n; i++ {
			c := s[i]
			if needsEscapeTable[c] || c >= 0x80 {
				return false
			}
		}
		return true
	}

	// Process 8 bytes at a time using SWAR technique
	for i := 0; i+8 <= n; i += 8 {
		// Load 8 bytes
		b0, b1, b2, b3 := s[i], s[i+1], s[i+2], s[i+3]
		b4, b5, b6, b7 := s[i+4], s[i+5], s[i+6], s[i+7]

		// Check for control characters (< 0x20) using bit manipulation
		// A byte is a control char if (byte - 0x20) has the sign bit set
		ctrlMask := ((b0 - 0x20) & 0x80) | ((b1 - 0x20) & 0x80) | ((b2 - 0x20) & 0x80) | ((b3 - 0x20) & 0x80) |
			((b4 - 0x20) & 0x80) | ((b5 - 0x20) & 0x80) | ((b6 - 0x20) & 0x80) | ((b7 - 0x20) & 0x80)

		// Quick check for common safe case: no control chars, no quotes, no backslashes
		if ctrlMask != 0 ||
			b0 == '"' || b1 == '"' || b2 == '"' || b3 == '"' ||
			b4 == '"' || b5 == '"' || b6 == '"' || b7 == '"' ||
			b0 == '\\' || b1 == '\\' || b2 == '\\' || b3 == '\\' ||
			b4 == '\\' || b5 == '\\' || b6 == '\\' || b7 == '\\' {
			// Found potential escape needed, verify with table lookup
			if needsEscapeTable[b0] || needsEscapeTable[b1] || needsEscapeTable[b2] || needsEscapeTable[b3] ||
				needsEscapeTable[b4] || needsEscapeTable[b5] || needsEscapeTable[b6] || needsEscapeTable[b7] {
				return false
			}
		}

		// UTF-8 validation: check for high bytes (>= 0x80)
		// High bytes indicate multi-byte UTF-8 sequences which need validation
		if (b0 | b1 | b2 | b3 | b4 | b5 | b6 | b7) >= 0x80 {
			// Has high bytes - fall back to full UTF-8 validation
			// This is slower but handles all UTF-8 edge cases correctly
			return false
		}
	}

	// Check remaining bytes (less than 8)
	for i := n &^ 7; i < n; i++ {
		c := s[i]
		if needsEscapeTable[c] {
			return false
		}
		// Check for high bytes in remainder
		if c >= 0x80 {
			return false
		}
	}

	return true
}

// needsEscape checks if a string needs JSON escaping (without UTF-8 validation)
// PERFORMANCE: Uses SWAR (SIMD Within A Register) technique for batch processing
// Note: Use isSafeString for combined escape + UTF-8 check
//
// NOTE (D-002/R8 M5): no production caller — isSafeString superseded it;
// retained for tests/future use.
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
		ctrlMask := ((b0 - 0x20) & 0x80) | ((b1 - 0x20) & 0x80) | ((b2 - 0x20) & 0x80) | ((b3 - 0x20) & 0x80) |
			((b4 - 0x20) & 0x80) | ((b5 - 0x20) & 0x80) | ((b6 - 0x20) & 0x80) | ((b7 - 0x20) & 0x80)

		// Quick check for common safe case: no control chars, no quotes, no backslashes
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
// PERFORMANCE: Batch copies safe segments to reduce append calls
// SECURITY: Validates UTF-8 encoding and replaces invalid sequences.
// Escapes U+2028/U+2029 unconditionally (encoding/json parity — they terminate
// lines in JavaScript). HTML characters (<, >, &) are NOT escaped here: that is
// the caller's post-pass (see NeedsHTMLEscapeBytes/HTMLEscapeBytes in
// html_escape.go and the escapeHTML gate in encoding.go).
func (e *FastEncoder) escapeString(s string) {
	start := 0
	n := len(s)

	for i := 0; i < n; {
		c := s[i]

		// SECURITY: Check for invalid UTF-8 sequences (high bytes)
		if c >= 0x80 {
			r, size := utf8.DecodeRuneInString(s[i:])
			if r == utf8.RuneError && size == 1 {
				// Invalid UTF-8 sequence - batch append safe portion then replace
				if start < i {
					e.buf = append(e.buf, s[start:i]...)
				}
				e.buf = append(e.buf, `\ufffd`...)
				i++
				start = i
				continue
			}
			// GEN-001 (stdlib parity): U+2028/U+2029 are valid JSON but
			// terminate lines in JavaScript; encoding/json escapes them
			// UNCONDITIONALLY \u2014 even under SetEscapeHTML(false) \u2014 and so does
			// escapeRune in the full encoder (see encoding.go). The fast path
			// previously passed all valid multi-byte UTF-8 through raw, so any
			// fast-encoder output consumed without the caller's HTML-escape
			// post-pass (Set/Delete mutation results via FastMarshalToString,
			// Encoder with SetEscapeHTML(false)) could carry a raw line
			// separator into a JS-embedded string.
			if r == 0x2028 || r == 0x2029 {
				if start < i {
					e.buf = append(e.buf, s[start:i]...)
				}
				if r == 0x2028 {
					e.buf = append(e.buf, `\u2028`...)
				} else {
					e.buf = append(e.buf, `\u2029`...)
				}
				i += size
				start = i
				continue
			}
			// Valid multi-byte UTF-8 - skip entire rune
			i += size
			continue
		}

		if c >= 0x20 && c != '"' && c != '\\' {
			i++
			continue
		}

		// Batch append the safe portion before this character
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
	if start < n {
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
// PERFORMANCE: Uses pre-computed lookup tables for integers -9999 to 9999
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

	// Fast path for large negative integers (-1000 to -9999)
	if n <= -1000 && n > -10000 {
		e.buf = append(e.buf, largeIntsNeg[-n-1000]...)
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

// largeIntsNeg contains pre-computed string representations of negative integers -1000 to -9999
// PERFORMANCE: Avoids strconv.AppendInt for common negative 4-digit integer range
var largeIntsNeg [9000][]byte

// init initializes the medium and negative integer lookup tables and common floats
// PERFORMANCE: Optimized initialization using direct byte array construction
// avoids string concatenation allocations
func init() {
	// Pre-allocate a single buffer for building numbers
	var buf [4]byte

	// Initialize medium integers (100-999) - direct byte construction
	for i := range 900 {
		n := i + 100
		buf[0] = byte('0' + n/100)
		buf[1] = byte('0' + (n/10)%10)
		buf[2] = byte('0' + n%10)
		// Create a copy since buf will be reused
		mediumInts[i] = []byte{buf[0], buf[1], buf[2]}
	}

	// Initialize large integers (1000-9999) - direct byte construction
	var buf4 [4]byte
	for i := range 9000 {
		n := i + 1000
		buf4[0] = byte('0' + n/1000)
		buf4[1] = byte('0' + (n/100)%10)
		buf4[2] = byte('0' + (n/10)%10)
		buf4[3] = byte('0' + n%10)
		largeInts[i] = []byte{buf4[0], buf4[1], buf4[2], buf4[3]}
	}

	// Initialize negative integers (-1 to -99)
	smallIntsNeg[0] = []byte{} // unused (index 0)
	for i := 1; i < 100; i++ {
		positive := smallInts[i]
		smallIntsNeg[i] = []byte{'-', positive[0]}
		if len(positive) > 1 {
			smallIntsNeg[i] = append(smallIntsNeg[i], positive[1:]...)
		}
	}

	// Initialize medium negative integers (-100 to -999)
	for i := range 900 {
		mediumIntsNeg[i] = []byte{'-', mediumInts[i][0], mediumInts[i][1], mediumInts[i][2]}
	}

	// Initialize large negative integers (-1000 to -9999)
	for i := range 9000 {
		largeIntsNeg[i] = []byte{'-', largeInts[i][0], largeInts[i][1], largeInts[i][2], largeInts[i][3]}
	}

	// Initialize common floats table
	// These are the most frequently used float values in JSON
	// PERFORMANCE: Extended coverage for 0.0-10.0 range
	commonFloats = map[uint64][]byte{
		// Integers 0-10 (handled by zero check and integer path, but included for completeness)
		0x3ff0000000000000: []byte("1"),  // 1.0
		0x4000000000000000: []byte("2"),  // 2.0
		0x4008000000000000: []byte("3"),  // 3.0
		0x4010000000000000: []byte("4"),  // 4.0
		0x4014000000000000: []byte("5"),  // 5.0
		0x4018000000000000: []byte("6"),  // 6.0
		0x401c000000000000: []byte("7"),  // 7.0
		0x4020000000000000: []byte("8"),  // 8.0
		0x4022000000000000: []byte("9"),  // 9.0
		0x4024000000000000: []byte("10"), // 10.0
		// Decimals 0.1-0.9
		0x3fb999999999999a: []byte("0.1"), // 0.1
		0x3fc999999999999a: []byte("0.2"), // 0.2
		0x3fd3333333333333: []byte("0.3"), // 0.3
		0x3fd999999999999a: []byte("0.4"), // 0.4
		0x3fe0000000000000: []byte("0.5"), // 0.5
		0x3fe3333333333333: []byte("0.6"), // 0.6
		0x3fe6666666666666: []byte("0.7"), // 0.7
		0x3fe999999999999a: []byte("0.8"), // 0.8
		0x3feccccccccccccd: []byte("0.9"), // 0.9
		// 1.x range
		0x3ff199999999999a: []byte("1.1"), // 1.1
		0x3ff3333333333333: []byte("1.2"), // 1.2
		0x3ff4cccccccccccd: []byte("1.3"), // 1.3
		0x3ff6666666666666: []byte("1.4"), // 1.4
		0x3ff8000000000000: []byte("1.5"), // 1.5
		0x3ff999999999999a: []byte("1.6"), // 1.6
		0x3ffb333333333333: []byte("1.7"), // 1.7
		0x3ffccccccccccccd: []byte("1.8"), // 1.8
		0x3ffe666666666666: []byte("1.9"), // 1.9
		// 2.x range
		0x4000666666666666: []byte("2.1"), // 2.1
		0x4000cccccccccccd: []byte("2.2"), // 2.2
		0x4001333333333333: []byte("2.3"), // 2.3
		0x400199999999999a: []byte("2.4"), // 2.4
		0x4002000000000000: []byte("2.5"), // 2.5
		0x4002666666666666: []byte("2.6"), // 2.6
		0x4002cccccccccccd: []byte("2.7"), // 2.7
		0x4003333333333333: []byte("2.8"), // 2.8
		0x400399999999999a: []byte("2.9"), // 2.9
		// 3.x range
		0x4008666666666666: []byte("3.1"), // 3.1
		0x4008cccccccccccd: []byte("3.2"), // 3.2
		0x4009333333333333: []byte("3.3"), // 3.3
		0x400999999999999a: []byte("3.4"), // 3.4
		0x400a000000000000: []byte("3.5"), // 3.5
		0x400a666666666666: []byte("3.6"), // 3.6
		0x400acccccccccccd: []byte("3.7"), // 3.7
		0x400b333333333333: []byte("3.8"), // 3.8
		0x400b99999999999a: []byte("3.9"), // 3.9
		// 4.x range
		0x4010666666666666: []byte("4.1"), // 4.1
		0x4010cccccccccccd: []byte("4.2"), // 4.2
		0x4011333333333333: []byte("4.3"), // 4.3
		0x401199999999999a: []byte("4.4"), // 4.4
		0x4012000000000000: []byte("4.5"), // 4.5
		0x4012666666666666: []byte("4.6"), // 4.6
		0x4012cccccccccccd: []byte("4.7"), // 4.7
		0x4013333333333333: []byte("4.8"), // 4.8
		0x401399999999999a: []byte("4.9"), // 4.9
		// 5.x range
		0x4014666666666666: []byte("5.1"), // 5.1
		0x4014cccccccccccd: []byte("5.2"), // 5.2
		0x4015333333333333: []byte("5.3"), // 5.3
		0x401599999999999a: []byte("5.4"), // 5.4
		0x4016000000000000: []byte("5.5"), // 5.5
		0x4016666666666666: []byte("5.6"), // 5.6
		0x4016cccccccccccd: []byte("5.7"), // 5.7
		0x4017333333333333: []byte("5.8"), // 5.8
		0x401799999999999a: []byte("5.9"), // 5.9
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
		// Powers of 10
		0x403e000000000000: []byte("30"),    // 30.0
		0x4049000000000000: []byte("50"),    // 50.0
		0x4059000000000000: []byte("100"),   // 100.0
		0x408f400000000000: []byte("1000"),  // 1000.0
		0x40c3880000000000: []byte("10000"), // 10000.0
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

// errUnsupportedFloat mirrors encoding/json's UnsupportedValueError message
// for NaN/±Inf, which cannot be represented in JSON.
func errUnsupportedFloat(f float64, bits int) error {
	return fmt.Errorf("json: unsupported value: %s", strconv.FormatFloat(f, 'g', -1, bits))
}

// EncodeFloat encodes a floating point number.
// PERFORMANCE: Uses pre-computed common values and fast integer conversion.
//
// D-002/R11 (m4): NaN and ±Inf are not representable in JSON (RFC 8259), and
// encoding/json returns UnsupportedValueError for them. This encoder used to
// emit `null` for them — contradicting the reject-everywhere policy enforced
// around every production caller (scalar/slice/map pre-checks) and risking
// that a future direct caller silently produces a value-changed document.
// EncodeFloat returns the error itself; the per-caller pre-check loops remain
// only to fail before any partial output is written.
func (e *FastEncoder) EncodeFloat(n float64, bits int) error {
	// Fast path for zero. Negative zero keeps its sign ("-0") for encoding/json
	// parity (stdlib's floatEncoder emits the sign); positive zero stays on the
	// fast path to avoid a strconv call for this very common value.
	if n == 0 {
		if math.Signbit(n) {
			e.buf = append(e.buf, '-', '0')
		} else {
			e.buf = append(e.buf, '0')
		}
		return nil
	}

	// Non-finite values are not representable in JSON — reject like
	// encoding/json (see the method comment, D-002/R11 m4).
	if math.IsNaN(n) || math.IsInf(n, 0) {
		return errUnsupportedFloat(n, bits)
	}

	// PERFORMANCE: Fast path for common float values
	bits64 := float64ToBits(n)
	if cached, ok := commonFloats[bits64]; ok {
		e.buf = append(e.buf, cached...)
		return nil
	}

	// PERFORMANCE: Fast path for small integer floats (0-100)
	// These can be encoded using the smallInts lookup table
	if n >= 0 && n <= 100 {
		intVal := int64(n)
		if n == float64(intVal) {
			e.buf = append(e.buf, smallInts[intVal]...)
			return nil
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
		return nil
	}

	// General case: match encoding/json's floatEncoder so output stays
	// compatible (e.g. 1e21 -> "1e+21", not a 22-digit decimal; very small
	// magnitudes use scientific notation). AppendJSONFloat is the single source
	// of truth shared with the root-package customEncoder.
	e.buf = AppendJSONFloat(e.buf, n, bits)
	return nil
}

// AppendJSONFloat appends f to dst using the same formatting rules as
// encoding/json's floatEncoder (Go stdlib): shortest representation, 'f' for
// moderate magnitudes, 'e' (lowercase, signed exponent) when abs < 1e-6 or
// abs >= 1e21, plus the stdlib "e-09" -> "e-9" cleanup. bits is 32 or 64.
//
// This is the single source of truth for stdlib-compatible float formatting in
// this package; FastEncoder.EncodeFloat and the root customEncoder both route
// through it so their output stays identical and byte-compatible with
// encoding/json for finite values.
//
// Callers must handle NaN/Inf BEFORE calling this — stdlib returns an
// *UnsupportedValueError for those; this function assumes f is finite.
func AppendJSONFloat(dst []byte, f float64, bits int) []byte {
	abs := f
	if f < 0 {
		abs = -f
	}
	fmtByte := byte('f')
	if abs != 0 {
		if bits == 32 {
			a := float32(abs)
			if a < 1e-6 || a >= 1e21 {
				fmtByte = 'e'
			}
		} else {
			if abs < 1e-6 || abs >= 1e21 {
				fmtByte = 'e'
			}
		}
	}
	dst = strconv.AppendFloat(dst, f, fmtByte, -1, bits)
	if fmtByte == 'e' {
		// clean up e-09 to e-9 (mirror encoding/json floatEncoder)
		n := len(dst)
		if n >= 4 && dst[n-4] == 'e' && dst[n-3] == '-' && dst[n-2] == '0' {
			dst[n-2] = dst[n-1]
			dst = dst[:n-1]
		}
	}
	return dst
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
// PERFORMANCE: Pre-allocates buffer capacity based on map size estimate
func (e *FastEncoder) EncodeMap(m map[string]any) error {
	if err := e.enterContainer(); err != nil {
		return err
	}
	defer e.leaveContainer()

	// PERFORMANCE: Pre-grow buffer to reduce reallocations
	// Estimate: each entry needs ~32 bytes on average (key + value + quotes + colon + comma)
	needed := len(m) * 32
	if cap(e.buf)-len(e.buf) < needed {
		e.buf = append(e.buf[:cap(e.buf)], make([]byte, needed)...)[:len(e.buf)]
	}

	e.buf = append(e.buf, '{')
	first := true

	// Sorted iteration keeps output deterministic and encoding/json-compatible.
	for k, v := range SortedEntries(m) {
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
	// PERFORMANCE: Pre-grow buffer to reduce reallocations
	needed := len(m) * 32
	if cap(e.buf)-len(e.buf) < needed {
		e.buf = append(e.buf[:cap(e.buf)], make([]byte, needed)...)[:len(e.buf)]
	}

	e.buf = append(e.buf, '{')
	first := true

	for k, v := range SortedEntries(m) {
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
	// PERFORMANCE: Pre-grow buffer to reduce reallocations
	needed := len(m) * 24
	if cap(e.buf)-len(e.buf) < needed {
		e.buf = append(e.buf[:cap(e.buf)], make([]byte, needed)...)[:len(e.buf)]
	}

	e.buf = append(e.buf, '{')
	first := true

	for k, v := range SortedEntries(m) {
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
// PERFORMANCE: Pre-allocates buffer capacity based on array size estimate
func (e *FastEncoder) EncodeArray(arr []any) error {
	if err := e.enterContainer(); err != nil {
		return err
	}
	defer e.leaveContainer()

	// PERFORMANCE: Pre-grow buffer to reduce reallocations for large arrays
	n := len(arr)
	if n > 8 {
		// Estimate: each element needs ~16 bytes on average
		needed := n * 16
		if cap(e.buf)-len(e.buf) < needed {
			e.buf = append(e.buf[:cap(e.buf)], make([]byte, needed)...)[:len(e.buf)]
		}
	}

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
// PERFORMANCE: Pre-allocates buffer for large slices
func (e *FastEncoder) EncodeStringSlice(arr []string) {
	// Pre-allocate for large slices
	if n := len(arr); n > 8 {
		totalLen := n * 2 // brackets and commas
		for _, s := range arr {
			totalLen += len(s) + 4 // quotes + potential escape overhead
		}
		if cap(e.buf)-len(e.buf) < totalLen {
			e.buf = append(e.buf[:cap(e.buf)], make([]byte, totalLen)...)[:len(e.buf)]
		}
	}

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
// PERFORMANCE: Pre-allocates buffer for large slices
func (e *FastEncoder) EncodeIntSlice(arr []int) {
	// Each int needs at most 20 bytes (max int64 digits + comma)
	if n := len(arr); n > 8 {
		needed := n * 12 // average estimate
		if cap(e.buf)-len(e.buf) < needed {
			e.buf = append(e.buf[:cap(e.buf)], make([]byte, needed)...)[:len(e.buf)]
		}
	}

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
// PERFORMANCE: Pre-allocates buffer for large slices
// D-002/R11 (m4): propagates EncodeFloat's non-finite rejection.
func (e *FastEncoder) EncodeFloatSlice(arr []float64) error {
	// Each float needs at most 24 bytes
	if n := len(arr); n > 8 {
		needed := n * 16 // average estimate
		if cap(e.buf)-len(e.buf) < needed {
			e.buf = append(e.buf[:cap(e.buf)], make([]byte, needed)...)[:len(e.buf)]
		}
	}

	e.buf = append(e.buf, '[')

	for i, v := range arr {
		if i > 0 {
			e.buf = append(e.buf, ',')
		}
		if err := e.EncodeFloat(v, 64); err != nil {
			return err
		}
	}

	e.buf = append(e.buf, ']')
	return nil
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
// D-002/R11 (m4): propagates EncodeFloat's non-finite rejection.
func (e *FastEncoder) EncodeFloat32Slice(arr []float32) error {
	e.buf = append(e.buf, '[')

	for i, v := range arr {
		if i > 0 {
			e.buf = append(e.buf, ',')
		}
		if err := e.EncodeFloat(float64(v), 32); err != nil {
			return err
		}
	}

	e.buf = append(e.buf, ']')
	return nil
}

// ============================================================================
// EXTENDED TYPE ENCODERS
// PERFORMANCE: Specialized encoders avoid reflection overhead
// ============================================================================

// EncodeTime encodes a time.Time in RFC3339Nano format, matching encoding/json
// (which preserves sub-second precision via time.Time.MarshalJSON).
//
// D-002/R11 (M3): RFC3339 timestamps cannot represent years outside [0,9999];
// encoding/json returns an error for those while this fast path used to emit an
// invalid 5-digit-year timestamp with a nil error. Reject with stdlib's
// message so the fast path no longer silently produces invalid JSON.
func (e *FastEncoder) EncodeTime(t time.Time) error {
	if y := t.Year(); y < 0 || y > 9999 {
		return fmt.Errorf("json: error calling MarshalJSON for type time.Time: Time.MarshalJSON: year outside of range [0,9999]")
	}
	e.buf = append(e.buf, '"')
	e.buf = append(e.buf, t.Format(time.RFC3339Nano)...)
	e.buf = append(e.buf, '"')
	return nil
}

// EncodeBase64 encodes a []byte as base64 string
// PERFORMANCE: Encodes directly into buffer to avoid intermediate string allocation
func (e *FastEncoder) EncodeBase64(b []byte) {
	encLen := base64.StdEncoding.EncodedLen(len(b))
	// Ensure capacity: 1 (quote) + encLen + 1 (quote)
	if cap(e.buf)-len(e.buf) < encLen+2 {
		newBuf := make([]byte, len(e.buf), len(e.buf)+encLen+2)
		copy(newBuf, e.buf)
		e.buf = newBuf
	}
	e.buf = append(e.buf, '"')
	start := len(e.buf)
	e.buf = e.buf[:start+encLen]
	base64.StdEncoding.Encode(e.buf[start:], b)
	e.buf = append(e.buf, '"')
}

// EncodeMapStringInt64 encodes a map[string]int64
func (e *FastEncoder) EncodeMapStringInt64(m map[string]int64) error {
	// PERFORMANCE: Pre-grow buffer to reduce reallocations
	needed := len(m) * 24
	if cap(e.buf)-len(e.buf) < needed {
		e.buf = append(e.buf[:cap(e.buf)], make([]byte, needed)...)[:len(e.buf)]
	}

	e.buf = append(e.buf, '{')
	first := true

	for k, v := range SortedEntries(m) {
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
	// PERFORMANCE: Pre-grow buffer to reduce reallocations
	needed := len(m) * 28
	if cap(e.buf)-len(e.buf) < needed {
		e.buf = append(e.buf[:cap(e.buf)], make([]byte, needed)...)[:len(e.buf)]
	}

	e.buf = append(e.buf, '{')
	first := true

	for k, v := range SortedEntries(m) {
		if !first {
			e.buf = append(e.buf, ',')
		}
		first = false

		e.EncodeString(k)
		e.buf = append(e.buf, ':')
		// D-002/R11 (m4): EncodeFloat rejects non-finite values itself — a
		// pre-scan of the map would duplicate that check on every clean encode.
		if err := e.EncodeFloat(v, 64); err != nil {
			return err
		}
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
//
// NOTE (D-002/R8 M5): no production caller — path parsing uses the string
// form (ParseIntFast); retained for tests/future use.
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

	// Parse digits with proper overflow detection
	var n int64
	for i := start; i < len(b); i++ {
		c := b[i]
		if c < '0' || c > '9' {
			return 0, strconv.ErrSyntax
		}
		digit := int64(c - '0')

		// Overflow detection:
		// For positive: n*10 + digit <= MaxInt64 (9223372036854775807)
		// For negative: -(n*10 + digit) >= MinInt64 (-9223372036854775808)
		// MinInt64 = -9223372036854775808, so we need n*10 + digit <= 9223372036854775808
		// MaxInt64 = 9223372036854775807, so we need n*10 + digit <= 9223372036854775807
		if neg {
			// For negative numbers, the absolute value can be up to 9223372036854775808
			// Check: n*10 + digit <= 9223372036854775808
			// Safe threshold: n < 922337203685477580 OR (n == 922337203685477580 AND digit <= 8)
			//
			// SECURITY NOTE: The boundary case "-9223372036854775808" (MinInt64) is valid:
			//   - After parsing "922337203685477580", n = 922337203685477580
			//   - The final digit is 8
			//   - Condition: n == maxVal/10 (922337203685477580) AND digit == 8
			//   - This passes the check (digit > 8 would fail), so we allow it
			//   - After negation: n = -9223372036854775808, which is exactly MinInt64
			//
			// CRITICAL: The check "digit > 8" (not "digit >= 8") is intentional and correct.
			// Changing to ">=" would incorrectly reject the valid MinInt64 value.
			const maxVal = 1 << 63 // 9223372036854775808
			if n > maxVal/10 || (n == maxVal/10 && digit > 8) {
				return 0, strconv.ErrRange
			}
		} else {
			// For positive numbers, the value can be up to 9223372036854775807
			// Check: n*10 + digit <= 9223372036854775807
			// Safe threshold: n < 922337203685477580 OR (n == 922337203685477580 AND digit <= 7)
			//
			// SECURITY NOTE: MaxInt64 is 9223372036854775807, last digit is 7
			//   - After parsing "922337203685477580", n = 922337203685477580
			//   - The final digit must be <= 7
			//   - Condition: digit > 7 would overflow, correctly rejected
			const maxVal = 1<<63 - 1 // 9223372036854775807
			if n > maxVal/10 || (n == maxVal/10 && digit > 7) {
				return 0, strconv.ErrRange
			}
		}
		n = n*10 + digit
	}

	if neg {
		n = -n
	}

	return n, nil
}

// FastParseFloat parses a float from a byte slice
// SECURITY: Rejects NaN and Infinity values which are invalid in standard JSON (RFC 8259)
//
// NOTE (D-002/R8 M5): no production caller; retained for tests/future use.
func FastParseFloat(b []byte) (float64, error) {
	// SECURITY: Fast check for invalid JSON float values
	// NaN, Inf, +Inf, -Inf are not valid JSON numbers per RFC 8259
	if len(b) >= 3 {
		// Check first 3 characters (case-insensitive)
		c0, c1, c2 := b[0], b[1], b[2]
		// Check for "nan", "inf" (with optional sign)
		if (c0 == 'N' || c0 == 'n') && (c1 == 'A' || c1 == 'a') && (c2 == 'N' || c2 == 'n') {
			return 0, strconv.ErrSyntax
		}
		if (c0 == 'I' || c0 == 'i') && (c1 == 'N' || c1 == 'n') && (c2 == 'F' || c2 == 'f') {
			return 0, strconv.ErrSyntax
		}
		// Check for "+inf", "-inf", "+nan", "-nan"
		if len(b) >= 4 && (c0 == '+' || c0 == '-') {
			c1, c2, c3 := b[1], b[2], b[3]
			if (c1 == 'N' || c1 == 'n') && (c2 == 'A' || c2 == 'a') && (c3 == 'N' || c3 == 'n') {
				return 0, strconv.ErrSyntax
			}
			if (c1 == 'I' || c1 == 'i') && (c2 == 'N' || c2 == 'n') && (c3 == 'F' || c3 == 'f') {
				return 0, strconv.ErrSyntax
			}
		}
	}
	// Use strconv for actual parsing
	return strconv.ParseFloat(string(b), 64)
}

// ============================================================================
// FAST MARSHAL/UNMARSHAL FUNCTIONS
// ============================================================================

// FastMarshal marshals a value to JSON using the fast encoder
//
// NOTE (D-002/R8 M5): no production caller — FastMarshalToString is the live
// path; retained for tests/future use.
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
// SECURITY VALIDATION HELPERS
// ============================================================================

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
//
// NOTE (D-002/R8 M5, confirmed R11): no production caller — encodeSlow routes
// complex types through stdlib json.Marshal, so this chain (GetStructEncoder/
// getEncodeFn) is exercised only by tests. ClearStructEncoderCache IS
// production code (called by the root package's ShutdownGlobalProcessor).
// Retained for tests/future use.
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
			IsPointer: f.Type.Kind() == reflect.Pointer,
			EncodeFn:  getEncodeFn(f.Type),
		}

		fields = append(fields, info)
	}

	// Cache it
	actual, _ := structEncoderCache.LoadOrStore(t, fields)
	return actual.([]StructFieldInfo)
}

// ClearStructEncoderCache removes all entries from the struct encoder cache.
// Call this to reclaim memory when the set of encoded struct types changes
// (e.g., after unloading plugins or in long-running services with dynamic types).
func ClearStructEncoderCache() {
	structEncoderCache.Range(func(key, _ any) bool {
		structEncoderCache.Delete(key)
		return true
	})
}

// getEncodeFn returns a type-specific encoding function for the given type
// PERFORMANCE: Avoids reflection for common types by generating specialized encoders
//
// NOTE (D-002/R8 M5, confirmed R11): no production caller — see GetStructEncoder.
func getEncodeFn(t reflect.Type) func(*FastEncoder, reflect.Value) error {
	// Handle pointer types
	if t.Kind() == reflect.Pointer {
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
			return e.EncodeFloat(v.Float(), 32)
		}
	case reflect.Float64:
		return func(e *FastEncoder, v reflect.Value) error {
			return e.EncodeFloat(v.Float(), 64)
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
	case reflect.Interface, reflect.Pointer:
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

// IsValidUTF8 checks if a byte slice is valid UTF-8
//
// NOTE (D-002/R8 M5): no production caller (encoders use utf8.DecodeRune*
// inline); retained for tests/future use.
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
//
// NOTE (D-002/R8 M5): neither GetFastBuffer nor PutFastBuffer has a
// production caller — the encoder/HTML-escape pools above are the live ones;
// retained for tests/future use.
func GetFastBuffer() *bytes.Buffer {
	buf := FastBufferPool.Get().(*bytes.Buffer)
	buf.Reset()
	return buf
}

// PutFastBuffer returns a buffer to the pool
func PutFastBuffer(buf *bytes.Buffer) {
	if buf.Cap() <= MaxPoolBufferSize/4 { // Don't pool very large buffers
		FastBufferPool.Put(buf)
	}
}
