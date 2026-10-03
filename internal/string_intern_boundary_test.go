package internal

import (
	"strings"
	"testing"
)

// ============================================================================
// Boundary tests for internal/string_intern.go low-coverage paths.
// ============================================================================

// --- KeyIntern.Size & Intern round-trip (string_intern.go:468, 0% coverage) ---

func TestKeyIntern_SizeAndRoundTrip(t *testing.T) {
	ki := NewKeyIntern()
	if got := ki.Size(); got != 0 {
		t.Errorf("empty interner Size = %d, want 0", got)
	}

	a := ki.Intern("hello")
	if a != "hello" {
		t.Errorf("Intern(hello) = %q, want %q", a, "hello")
	}
	// Re-interning the same key must not grow the set.
	ki.Intern("hello")
	if got := ki.Size(); got != 1 {
		t.Errorf("after duplicate Intern Size = %d, want 1", got)
	}

	ki.Intern("world")
	if got := ki.Size(); got != 2 {
		t.Errorf("after two unique keys Size = %d, want 2", got)
	}
}

// --- InternBytes (string_intern.go:411) ---

func TestKeyIntern_InternBytes(t *testing.T) {
	ki := NewKeyIntern()
	got := ki.InternBytes([]byte("bytekey"))
	if got != "bytekey" {
		t.Errorf("InternBytes = %q, want %q", got, "bytekey")
	}
	// Empty input short-circuits.
	if got := ki.InternBytes(nil); got != "" {
		t.Errorf("InternBytes(nil) = %q, want empty", got)
	}
}

// --- promoteToHotCache trim path (string_intern.go:310/327, 0% coverage) ---
//
// Interning more than maxHotKeys (10000) unique keys forces promoteToHotCache
// over the threshold and invokes trimHotCache.

func TestKeyIntern_HotCacheTrim(t *testing.T) {
	ki := NewKeyIntern()
	for i := 0; i < 10000+50; i++ {
		ki.Intern(strconvItoa(i))
	}
	// Must not panic and must remain internally consistent.
	if got := ki.Size(); got != 10000+50 {
		t.Errorf("Size = %d, want %d", got, 10000+50)
	}
	// A previously interned key is still resolvable (returns an equal string).
	first := ki.Intern(strconvItoa(0))
	if first != "0" {
		t.Errorf("re-intern of key 0 = %q, want %q", first, "0")
	}
}

// --- copyString force-copy path (string_intern.go:25, 60% coverage) ---

func TestCopyString_LargeInput(t *testing.T) {
	// Above maxStringCopyThreshold (8192) the function takes the force-copy
	// branch via []byte conversion; content must be preserved.
	s := strings.Repeat("x", 8193)
	got := copyString(s)
	if got != s {
		t.Error("copyString altered content for large input")
	}
}

// strconvItoa avoids importing strconv just for one call site.
func strconvItoa(i int) string {
	if i == 0 {
		return "0"
	}
	neg := i < 0
	if neg {
		i = -i
	}
	var b [20]byte
	pos := len(b)
	for i > 0 {
		pos--
		b[pos] = byte('0' + i%10)
		i /= 10
	}
	if neg {
		pos--
		b[pos] = '-'
	}
	return string(b[pos:])
}
