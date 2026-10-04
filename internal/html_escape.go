package internal

import (
	"bytes"
	"sync"
)

// needsHTMLEscapeTable is a pre-computed lookup table for bytes that need HTML escaping.
// Index is the byte value. Set for '<' (0x3C), '>' (0x3E), '&' (0x26), and 0xE2 (U+2028/U+2029 prefix).
// PERFORMANCE: Replaces 24 per-byte comparisons with a single table lookup per byte.
var needsHTMLEscapeTable [256]bool

func init() {
	needsHTMLEscapeTable['<'] = true
	needsHTMLEscapeTable['>'] = true
	needsHTMLEscapeTable['&'] = true
	needsHTMLEscapeTable[0xE2] = true // start of U+2028/U+2029 UTF-8 sequence
}

// HTMLEscapeTo writes HTML-escaped JSON to the destination buffer.
// Byte-level, like encoding/json.HTMLEscape: ranging over the string would
// decode invalid UTF-8 bytes to U+FFFD and re-encode them as EF BF BD,
// silently rewriting bytes that are not part of any escape rule.
func HTMLEscapeTo(dst *bytes.Buffer, s string) {
	n := len(s)
	for i := 0; i < n; i++ {
		c := s[i]
		switch c {
		case '<':
			dst.WriteString("\\u003c")
		case '>':
			dst.WriteString("\\u003e")
		case '&':
			dst.WriteString("\\u0026")
		case 0xE2:
			// U+2028 (E2 80 A8) / U+2029 (E2 80 A9)
			if i+2 < n && s[i+1] == 0x80 && (s[i+2] == 0xA8 || s[i+2] == 0xA9) {
				if s[i+2] == 0xA8 {
					dst.WriteString("\\u2028")
				} else {
					dst.WriteString("\\u2029")
				}
				i += 2
				continue
			}
			dst.WriteByte(c)
		default:
			dst.WriteByte(c)
		}
	}
}

// HTMLEscapeBytesTo writes HTML-escaped data to the destination buffer.
// It is the []byte-input counterpart of HTMLEscapeTo (same byte-level
// semantics, see that function's comment) for callers that already hold
// encoder output as bytes: the JSONL batch encoder appends per-item encodings
// straight into the shared output buffer, with no intermediate string or
// pooled-slice copy (P-001).
func HTMLEscapeBytesTo(dst *bytes.Buffer, data []byte) {
	n := len(data)
	for i := 0; i < n; i++ {
		c := data[i]
		switch c {
		case '<':
			dst.WriteString("\\u003c")
		case '>':
			dst.WriteString("\\u003e")
		case '&':
			dst.WriteString("\\u0026")
		case 0xE2:
			// U+2028 (E2 80 A8) / U+2029 (E2 80 A9)
			if i+2 < n && data[i+1] == 0x80 && (data[i+2] == 0xA8 || data[i+2] == 0xA9) {
				if data[i+2] == 0xA8 {
					dst.WriteString("\\u2028")
				} else {
					dst.WriteString("\\u2029")
				}
				i += 2
				continue
			}
			dst.WriteByte(c)
		default:
			dst.WriteByte(c)
		}
	}
}

// NeedsHTMLEscapeBytes checks if a byte slice needs HTML escaping.
// PERFORMANCE v3: Uses lookup table for single table-access per byte instead of 24 comparisons.
// The table checks for '<', '>', '&', and 0xE2 (start of U+2028/U+2029).
func NeedsHTMLEscapeBytes(data []byte) bool {
	n := len(data)
	if n == 0 {
		return false
	}

	// Main loop: table lookup is faster than chained OR comparisons
	// The CPU can speculate and branch-predict table lookups efficiently
	for i := 0; i < n; i++ {
		if needsHTMLEscapeTable[data[i]] {
			// For <, >, & — confirmed escape needed
			c := data[i]
			if c == '<' || c == '>' || c == '&' {
				return true
			}
			// For 0xE2 — check for U+2028/U+2029
			if c == 0xE2 && i+2 < n && data[i+1] == 0x80 && (data[i+2] == 0xA8 || data[i+2] == 0xA9) {
				return true
			}
		}
	}
	return false
}

// htmlEscapeBytesPool pools byte slices for HTML escaping operations
var htmlEscapeBytesPool = sync.Pool{
	New: func() any {
		b := make([]byte, 0, 256)
		return &b
	},
}

// HTMLEscapeBytes performs HTML escaping on a byte slice and returns the escaped result.
// PERFORMANCE v2: Works directly on byte slices to avoid string conversion.
// This is the most efficient path for encoder output.
func HTMLEscapeBytes(data []byte) []byte {
	n := len(data)
	if n == 0 {
		return data
	}

	// Fast path: no escapable bytes — return the input unchanged.
	// NOTE: the returned slice aliases the caller's memory on this path, so it
	// must NOT be passed to PutHTMLEscapeBytes (which pools the backing array).
	// Production callers pre-check NeedsHTMLEscapeBytes and never reach here.
	if !NeedsHTMLEscapeBytes(data) {
		return data
	}

	// Slow path: perform escaping
	// Estimate: worst case each char becomes 6 chars (\u003c)
	result := htmlEscapeBytesPool.Get().(*[]byte)
	*result = (*result)[:0]
	if cap(*result) < n*2 {
		*result = make([]byte, 0, n*2)
	}

	for i := 0; i < n; i++ {
		c := data[i]
		switch c {
		case '<':
			*result = append(*result, '\\', 'u', '0', '0', '3', 'c')
		case '>':
			*result = append(*result, '\\', 'u', '0', '0', '3', 'e')
		case '&':
			*result = append(*result, '\\', 'u', '0', '0', '2', '6')
		case 0xE2:
			// Check for U+2028 or U+2029
			if i+2 < n && data[i+1] == 0x80 {
				if data[i+2] == 0xA8 {
					*result = append(*result, '\\', 'u', '2', '0', '2', '8')
					i += 2
					continue
				} else if data[i+2] == 0xA9 {
					*result = append(*result, '\\', 'u', '2', '0', '2', '9')
					i += 2
					continue
				}
			}
			*result = append(*result, c)
		default:
			*result = append(*result, c)
		}
	}

	escaped := *result

	// Don't pool large buffers
	if cap(*result) > MaxPoolBufferSize/4 {
		// Return a copy to avoid pool retention
		copied := make([]byte, len(escaped))
		copy(copied, escaped)
		return copied
	}

	return escaped
}

// PutHTMLEscapeBytes returns a byte slice obtained from HTMLEscapeBytes to the pool.
// Call this after using the result if you don't need it anymore.
//
// CONTRACT: only pass slices that HTMLEscapeBytes actually allocated — i.e.
// results of calls whose input contained escapable bytes (checked via
// NeedsHTMLEscapeBytes). HTMLEscapeBytes returns its INPUT unchanged on the
// no-escape fast path; pooling that slice would insert caller-owned memory
// into the pool and the next escape would write into it (data corruption).
func PutHTMLEscapeBytes(b []byte) {
	if cap(b) <= MaxPoolBufferSize/4 && cap(b) > 0 {
		// Only pool reasonably sized buffers
		htmlEscapeBytesPool.Put(&b)
	}
}
