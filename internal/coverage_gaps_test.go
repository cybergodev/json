package internal

// FIX-001 coverage-gap boundary tests.
//
// Each test here targets functions or branches the main suite leaves
// unexecuted (identified via `go tool cover -func` on the pre-change
// baseline). Table-driven where a pure input→output mapping applies.

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sync/atomic"
	"testing"
	"time"
)

// borrowEncoder returns a pooled encoder whose depth cap is guaranteed clean
// on return, so a test-local SetMaxEncodeDepth cannot leak to later pool users.
func borrowEncoder(t *testing.T) *FastEncoder {
	t.Helper()
	return GetEncoder()
}

func returnEncoder(e *FastEncoder) {
	e.SetMaxEncodeDepth(0) // pool reset contract: stale caps must not leak
	PutEncoder(e)
}

// -----------------------------------------------------------------------------
// SetErrorSentinels / sentinel wiring (compiled_path.go)
// -----------------------------------------------------------------------------

// TestSetErrorSentinels_Wiring covers the exported sentinel setter and its
// sync.Once semantics. The root package normally calls this from init(); in
// the internal test binary this test is the first (and only effective) call,
// so both the assignment and nil-argument-keeps-value branches are exercised.
func TestSetErrorSentinels_Wiring(t *testing.T) {
	if ErrPathNotFound.Error() != "path not found" {
		t.Fatalf("precondition: default ErrPathNotFound = %q", ErrPathNotFound.Error())
	}

	custom := errors.New("custom-pnf")
	SetErrorSentinels(custom, nil, nil) // nil args must keep current values

	if ErrPathNotFound != custom {
		t.Errorf("ErrPathNotFound not replaced: got %v", ErrPathNotFound)
	}
	if ErrTypeMismatch.Error() != "type mismatch" {
		t.Errorf("nil typeMismatch must keep default, got %q", ErrTypeMismatch.Error())
	}
	if ErrInvalidPath.Error() != "invalid path format" {
		t.Errorf("nil invalidPath must keep default, got %q", ErrInvalidPath.Error())
	}

	// Once semantics: a second call must be a complete no-op.
	second := errors.New("second-call")
	SetErrorSentinels(second, second, second)
	if ErrPathNotFound != custom || ErrTypeMismatch.Error() != "type mismatch" {
		t.Errorf("SetErrorSentinels applied twice; sync.Once violated: %v / %v", ErrPathNotFound, ErrTypeMismatch)
	}
}

// -----------------------------------------------------------------------------
// FastEncoder: depth cap, nil containers, unsupported floats (fast_encoder.go)
// -----------------------------------------------------------------------------

// nestedMap builds a map nested to the given depth: {"a": {"a": ... {"a": 1}}}.
func nestedMap(depth int) map[string]any {
	root := map[string]any{"a": 1.0}
	for range depth - 1 {
		root = map[string]any{"a": root}
	}
	return root
}

// TestFastEncoderSetMaxEncodeDepth covers the per-encoder depth cap: explicit
// enforcement, the n<=0 reset back to the package default, and effectiveMaxDepth.
func TestFastEncoderSetMaxEncodeDepth(t *testing.T) {
	enc := borrowEncoder(t)
	defer returnEncoder(enc)

	if got := enc.effectiveMaxDepth(); got != MaxNestingDepth {
		t.Errorf("default effectiveMaxDepth = %d, want %d", got, MaxNestingDepth)
	}

	enc.SetMaxEncodeDepth(3)
	if got := enc.effectiveMaxDepth(); got != 3 {
		t.Errorf("cap 3: effectiveMaxDepth = %d, want 3", got)
	}

	cases := []struct {
		name  string
		depth int
		ok    bool
	}{
		{"at cap", 3, true},
		{"one beyond cap", 4, false},
		{"far beyond cap", 50, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			enc.Reset()
			err := enc.EncodeValue(nestedMap(tc.depth))
			if tc.ok && err != nil {
				t.Fatalf("depth %d within cap: unexpected error %v", tc.depth, err)
			}
			if !tc.ok && err == nil {
				t.Fatalf("depth %d exceeds cap: expected error, got %s", tc.depth, enc.Bytes())
			}
		})
	}

	// Reset path: n <= 0 restores the package default.
	enc.SetMaxEncodeDepth(0)
	if got := enc.effectiveMaxDepth(); got != MaxNestingDepth {
		t.Errorf("after reset: effectiveMaxDepth = %d, want %d", got, MaxNestingDepth)
	}
	enc.Reset()
	if err := enc.EncodeValue(nestedMap(MaxNestingDepth + 5)); err == nil {
		t.Error("depth beyond package default: expected error after reset")
	}
}

// TestFastEncoderNilContainers covers the writeNull branches: typed nil maps
// and slices encode as JSON null (distinct from their empty non-nil forms).
func TestFastEncoderNilContainers(t *testing.T) {
	cases := []struct {
		name  string
		value any
		want  string
	}{
		{"nil any", nil, "null"},
		{"nil []byte", []byte(nil), "null"},
		{"empty []byte", []byte{}, `""`},
		{"nil map[string]any", map[string]any(nil), "null"},
		{"empty map[string]any", map[string]any{}, `{}`},
		{"nil map[string]string", map[string]string(nil), "null"},
		{"nil map[string]int", map[string]int(nil), "null"},
		{"nil map[string]int64", map[string]int64(nil), "null"},
		{"nil map[string]float64", map[string]float64(nil), "null"},
		{"nil map[string]bool", map[string]bool(nil), "null"},
		{"nil []any", []any(nil), "null"},
		{"empty []any", []any{}, `[]`},
		{"nil []string", []string(nil), "null"},
		{"nil []int", []int(nil), "null"},
		{"nil []int64", []int64(nil), "null"},
		{"nil []uint64", []uint64(nil), "null"},
		{"nil []float64", []float64(nil), "null"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			enc := borrowEncoder(t)
			defer returnEncoder(enc)
			if err := enc.EncodeValue(tc.value); err != nil {
				t.Fatalf("EncodeValue(%v): %v", tc.value, err)
			}
			if got := string(enc.Bytes()); got != tc.want {
				t.Errorf("EncodeValue(%v) = %s, want %s", tc.value, got, tc.want)
			}
		})
	}
}

// TestFastEncoderUnsupportedFloat covers errUnsupportedFloat: non-finite
// float32/64 values are rejected on the fast path, matching encoding/json.
func TestFastEncoderUnsupportedFloat(t *testing.T) {
	cases := []struct {
		name  string
		value any
	}{
		{"float64 NaN", math.NaN()},
		{"float64 +Inf", math.Inf(1)},
		{"float64 -Inf", math.Inf(-1)},
		{"float32 NaN", float32(math.NaN())},
		{"float32 +Inf", float32(math.Inf(1))},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, stdErr := json.Marshal(tc.value); stdErr == nil {
				t.Fatalf("encoding/json unexpectedly accepted %v; parity check invalid", tc.value)
			}
			enc := borrowEncoder(t)
			defer returnEncoder(enc)
			err := enc.EncodeValue(tc.value)
			if err == nil {
				t.Fatalf("EncodeValue(%v): expected unsupported-value error", tc.value)
			}
		})
	}

	// Slice variants carry the same guard as the scalar cases above.
	enc := borrowEncoder(t)
	defer returnEncoder(enc)
	if err := enc.EncodeValue([]float64{1, math.Inf(1)}); err == nil {
		t.Error("EncodeValue([1, +Inf]): expected error")
	}
	if err := enc.EncodeValue([]float32{1, float32(math.NaN())}); err == nil {
		t.Error("EncodeValue([1, NaN]): expected error")
	}
}

// TestFastEncoderReflectKinds drives getEncodeFn/encodeSlow through named
// (non-fast-switch) types, comparing output byte-for-byte with encoding/json.
func TestFastEncoderReflectKinds(t *testing.T) {
	type tStr string
	type tInt int
	type tUint uint16
	type tF32 float32
	type tF64 float64
	type tBool bool
	type tByteSlice []byte
	type tIntPtr *int

	five := 5
	type gapInner struct {
		X int `json:"x"`
	}
	// gapKitchen routes one field through every getEncodeFn branch: scalar
	// kinds, pointer (nil + non-nil), byte slice, generic slice, the four
	// specialized map types plus a generic map, nested struct, omitempty.
	type gapKitchen struct {
		S      string             `json:"s"`
		I      int                `json:"i"`
		I8     int8               `json:"i8"`
		U      uint               `json:"u"`
		U16    uint16             `json:"u16"`
		F32    float32            `json:"f32"`
		F64    float64            `json:"f64"`
		B      bool               `json:"b"`
		Ptr    *int               `json:"ptr"`
		NilPtr *int               `json:"nilptr"`
		By     []byte             `json:"by"`
		NilBy  []byte             `json:"nilby"`
		Sl     []int              `json:"sl"`
		NilSl  []int              `json:"nilsl"`
		MSS    map[string]string  `json:"mss"`
		MSI    map[string]int     `json:"msi"`
		MS64   map[string]int64   `json:"ms64"`
		MSF    map[string]float64 `json:"msf"`
		MSB    map[string]bool    `json:"msb"`
		N      gapInner           `json:"n"`
		Opt    string             `json:"opt,omitempty"`
	}
	cases := []struct {
		name  string
		value any
	}{
		{"named string", tStr("hello")},
		{"named int", tInt(-42)},
		{"named uint", tUint(7)},
		{"named float32", tF32(1.5)},
		{"named float64", tF64(0.25)},
		{"named bool", tBool(true)},
		{"named byte slice", tByteSlice{1, 2, 3}},
		{"array", [3]int{1, 2, 3}},
		{"non-nil pointer", tIntPtr(&five)},
		{"struct", struct {
			A int    `json:"a"`
			B string `json:"b"`
		}{A: 1, B: "x"}},
		{"nested named types", map[string]any{"k": []any{tInt(9), tStr("v")}}},
		{"kitchen sink struct", gapKitchen{
			S: "v", I: -1, I8: 2, U: 3, U16: 4, F32: 0.5, F64: 1.5, B: true,
			Ptr: &five, NilPtr: nil,
			By: []byte{9, 9}, NilBy: nil,
			Sl: []int{7}, NilSl: nil,
			MSS:  map[string]string{"k": "v"},
			MSI:  map[string]int{"k": 1},
			MS64: map[string]int64{"k": 2},
			MSF:  map[string]float64{"k": 3.5},
			MSB:  map[string]bool{"k": true},
			N:    gapInner{X: 8},
			Opt:  "present",
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			want, stdErr := json.Marshal(tc.value)
			enc := borrowEncoder(t)
			defer returnEncoder(enc)
			err := enc.EncodeValue(tc.value)
			if (err == nil) != (stdErr == nil) {
				t.Fatalf("error mismatch: fast=%v stdlib=%v", err, stdErr)
			}
			if err != nil {
				return
			}
			if got := string(enc.Bytes()); got != string(want) {
				t.Errorf("fast=%s want(stdlib)=%s", got, want)
			}
		})
	}

	// nil pointer encodes as null.
	enc := borrowEncoder(t)
	defer returnEncoder(enc)
	var nilPtr tIntPtr
	if err := enc.EncodeValue(nilPtr); err != nil {
		t.Fatalf("nil pointer: %v", err)
	}
	if got := string(enc.Bytes()); got != "null" {
		t.Errorf("nil pointer = %s, want null", got)
	}
}

// -----------------------------------------------------------------------------
// HTMLEscapeBytesTo (html_escape.go)
// -----------------------------------------------------------------------------

// TestHTMLEscapeBytesTo pins the buffer-writing HTML escaper to byte parity
// with encoding/json.HTMLEscape for the mandatory escapes plus the U+2028 and
// U+2029 line separators.
func TestHTMLEscapeBytesTo(t *testing.T) {
	inputs := []string{
		``,
		`plain`,
		`<div>`,
		`a&b`,
		`"<script>&</script>"`,
		"a\u2028b", // U+2028 line separator
		"a\u2029b", // U+2029 paragraph separator
		"mixed <b>&amp;</b>  ",
	}
	for _, in := range inputs {
		var want bytes.Buffer
		json.HTMLEscape(&want, []byte(in))

		var got bytes.Buffer
		HTMLEscapeBytesTo(&got, []byte(in))

		if got.String() != want.String() {
			t.Errorf("HTMLEscapeBytesTo(%q) = %q, want %q (stdlib parity)", in, got.String(), want.String())
		}
	}
}

// -----------------------------------------------------------------------------
// MergeMode.String (helpers.go)
// -----------------------------------------------------------------------------

// TestMergeModeString covers all branches of the MergeMode Stringer.
func TestMergeModeString(t *testing.T) {
	cases := []struct {
		mode MergeMode
		want string
	}{
		{MergeUnion, "union"},
		{MergeIntersection, "intersection"},
		{MergeDifference, "difference"},
		{MergeMode(99), "unknown(99)"},
	}
	for _, tc := range cases {
		if got := tc.mode.String(); got != tc.want {
			t.Errorf("MergeMode(%d).String() = %q, want %q", tc.mode, got, tc.want)
		}
	}
}

// -----------------------------------------------------------------------------
// CacheManager.Close (cache.go)
// -----------------------------------------------------------------------------

// TestCacheManagerClose covers the cache shutdown path and its idempotency.
func TestCacheManagerClose(t *testing.T) {
	cm := NewCacheManager(true, 100, time.Minute)
	key := CacheKey{Op: "get", JSONHash: 42, Path: "k"}
	cm.Set(key, "v")

	if v, ok := cm.Get(key); !ok || v != "v" {
		t.Fatalf("pre-close Get = (%v, %v), want (v, true)", v, ok)
	}

	cm.Close()
	cm.Close() // second close must not panic or hang
}

// -----------------------------------------------------------------------------
// evictPathCacheEntries (path.go)
// -----------------------------------------------------------------------------

// TestPathCacheEvictionOnOverflow fills the path-segment cache past its hard
// limit and verifies LRU eviction keeps the size bounded and the cache usable.
func TestPathCacheEvictionOnOverflow(t *testing.T) {
	const fill = pathCacheMaxSize + 200
	segments := []PathSegment{{Type: PropertySegment, Key: "x"}}

	for i := range fill {
		setCachedPathSegments(fmt.Sprintf("evict/entry/%d", i), segments)
	}

	if got := atomic.LoadInt64(&pathCacheSize); got > pathCacheMaxSize {
		t.Errorf("pathCacheSize = %d after overflow fill, want <= %d", got, pathCacheMaxSize)
	}

	// The cache must remain functional after eviction: a fresh entry is served.
	setCachedPathSegments("evict/after-overflow", segments)
	if got, ok := getCachedPathSegments("evict/after-overflow"); !ok || len(got) != 1 {
		t.Errorf("post-eviction lookup failed: got=%v ok=%v", got, ok)
	}
}

// -----------------------------------------------------------------------------
// KeyIntern.evictShardLocked (string_intern.go)
// -----------------------------------------------------------------------------

// TestKeyInternEvictShardLocked exercises the shard evictor directly: half the
// entries are removed, size accounting follows, and empty shards report false.
func TestKeyInternEvictShardLocked(t *testing.T) {
	ki := NewKeyIntern()

	// Empty shard: nothing to evict.
	empty := ki.getShard("empty-shard-probe")
	empty.mu.Lock()
	if ki.evictShardLocked(empty) {
		empty.mu.Unlock()
		t.Fatal("evictShardLocked on empty shard returned true")
	}
	empty.mu.Unlock()

	// Populate one shard with 10 tracked entries.
	shard := ki.getShard("seed")
	shard.mu.Lock()
	for i := range 10 {
		k := fmt.Sprintf("key-%02d", i)
		shard.strings[k] = k
		shard.size += int64(len(k))
	}
	sizeBefore := shard.size

	evicted := ki.evictShardLocked(shard)
	shard.mu.Unlock()

	if !evicted {
		t.Fatal("evictShardLocked on populated shard returned false")
	}
	shard.mu.RLock()
	defer shard.mu.RUnlock()
	if got := len(shard.strings); got != 5 {
		t.Errorf("shard retained %d entries, want 5 (half of 10)", got)
	}
	if shard.size >= sizeBefore || shard.size <= 0 {
		t.Errorf("shard.size = %d after eviction, want in (0, %d)", shard.size, sizeBefore)
	}
	if shard.evictions != 5 {
		t.Errorf("shard.evictions = %d, want 5", shard.evictions)
	}
}

// -----------------------------------------------------------------------------
// matchPatternIgnoreCaseFast / safeMultiply / safeAdd (helpers.go, cache.go)
// -----------------------------------------------------------------------------

// TestMatchPatternIgnoreCaseFast covers the length gate plus the 8-byte and
// tail loops of the case-folding pattern matcher.
func TestMatchPatternIgnoreCaseFast(t *testing.T) {
	cases := []struct {
		s, pattern string
		want       bool
	}{
		{"users", "USERS", true},
		{"Users", "uSERS", true},
		{"users", "users", true},
		{"users", "user", false},                 // length mismatch
		{"user", "users", false},                 // length mismatch
		{"usr", "users", false},                  // short non-match
		{"user settings", "USER SETTINGS", true}, // >=8 bytes, main loop
		{"User Settings", "user settings", true},
		{"user settings", "USER SETTIXG", false},
		{"aBcDaBcD", "AbCdAbCd", true}, // mixed-case 8-byte block + folding
		{"", "", true},
	}
	for _, tc := range cases {
		if got := matchPatternIgnoreCaseFast(tc.s, tc.pattern); got != tc.want {
			t.Errorf("matchPatternIgnoreCaseFast(%q, %q) = %v, want %v", tc.s, tc.pattern, got, tc.want)
		}
	}
}

// TestSafeArithmetic covers the overflow guards used for memory estimation.
func TestSafeArithmetic(t *testing.T) {
	if got, ok := safeMultiply(0, 5, 100); !ok || got != 0 {
		t.Errorf("safeMultiply(0,5) = (%d,%v), want (0,true)", got, ok)
	}
	if got, ok := safeMultiply(10, 10, 1000); !ok || got != 100 {
		t.Errorf("safeMultiply(10,10) = (%d,%v), want (100,true)", got, ok)
	}
	if got, ok := safeMultiply(1<<62, 4, math.MaxInt64); ok || got != math.MaxInt64 {
		t.Errorf("safeMultiply overflow = (%d,%v), want (MaxInt64,false)", got, ok)
	}

	if got, ok := safeAdd(50, 60, 1000); !ok || got != 110 {
		t.Errorf("safeAdd(50,60) = (%d,%v), want (110,true)", got, ok)
	}
	if got, ok := safeAdd(90, 20, 100); ok || got != 100 {
		t.Errorf("safeAdd over max = (%d,%v), want (100,false)", got, ok)
	}
}
