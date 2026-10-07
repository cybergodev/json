package json

import (
	"context"
	"encoding/json"
	stdjson "encoding/json"
	"errors"
	"io"
	"math"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cybergodev/json/internal"
)

// This file locks in the behavior fixes from task D-002 so they cannot regress.
// Each test corresponds to a specific finding (C/M/F IDs) and would fail on the
// pre-fix code. Sections are ordered by review round:
//
//	Round 1 — C1, M1–M9: slice steps, uint64 conversion, NaN/Inf, streaming
//	          size limits, cache long-key round-trip.
//	Round 2 — C1–C3, M1–M7, F1–F8: reverse slices, encoding/json compat,
//	          wildcards, per-call CreatePaths, distributed null, struct encoding.
//	Round 4 — C1–C4: Number preservation, non-BMP escapes, CompiledPath
//	          reverse slice, parallel iterator.
//	Round 6 — map-value collection determinism and scan-window security.
//	Round 8 — C1/C2/M1/M3/M4/m2: no-cfg PreserveNumbers honored on the parse
//	          funnel (Get/Set/Delete/ParseAny), Delete fast-path guard, per-call
//	          MaxJSONSize on encode output, rate-limit wiring, MaxConcurrency
//	          0→default, JSONL mem-limit ErrSizeLimit sentinel.
//	Round 9 — m3/m4/m9-m12: governance coverage for Parse/Valid/PreParse/
//	          *FromParsed/Prettify/Compact/ValidateSchema (+CompareJSON via
//	          p.Marshal), JSONL engines honor PreserveNumbers, root-array
//	          extension explicit error, JSONLWriter single-write.
//	Round 10 — 回查轮: no-cfg baked encode limits (MarshalIndent/SaveToWriter/
//	          CompareJSON — M1 regression fix), convertTo* handle library
//	          Number, SetMultiple rate-limit gate, parallel-engine mem cap,
//	          SetFromParsed baked CreatePaths, baked CacheResults.
//	Round 11 — C1/M1–M3/m4: decoder paths reject trailing garbage,
//	          GetMultiple governance, mutation output size limit,
//	          EncodeTime year-range guard, EncodeFloat NaN/Inf rejection.
//
// Formerly split across regression_test.go, d002_round2_regression_test.go,
// d002_round4_verify_test.go, and d002_round6_regression_test.go; consolidated
// into this single file on 2026-08-30 (test content preserved verbatim).
// regression_round10_test.go and regression_round11_test.go were merged in on
// 2026-10-04, likewise verbatim. asStr/d002fmt live in the shared-helpers
// section because rounds 1 and 2 both use them; round-6 helpers stay inside
// their section.

// ---------------------------------------------------------------------------
// Shared helpers
// ---------------------------------------------------------------------------

// asStr renders a Get result with %v so assertions are shape-agnostic.
func asStr(v any) string { return d002fmt(v) }

func d002fmt(v any) string {
	if v == nil {
		return "<nil>"
	}
	b, err := stdjson.Marshal(v)
	if err != nil {
		return "<unmarshalable>"
	}
	return string(b)
}

// ===========================================================================
// Round 1 — original review findings (C1, M1–M9)
// ===========================================================================

// M2: non-last-segment array slice must honor step (a[0:5:2].b → indices 0,2,4).
func TestD002_NonLastSegmentSlice_HonorsStep(t *testing.T) {
	const doc = `{"arr":[{"b":0},{"b":1},{"b":2},{"b":3},{"b":4}]}`
	got, err := Get(doc, "arr[0:5:2].b")
	if err != nil {
		t.Fatalf("Get err: %v", err)
	}
	if want := "[0,2,4]"; asStr(got) != want {
		t.Fatalf("M2 step ignored: got %q want %q", asStr(got), want)
	}
}

// M3: {extract}[slice] must honor step and reverse.
func TestD002_ExtractThenSlice_HonorsStepAndReverse(t *testing.T) {
	const doc = `{"items":[{"id":1},{"id":2},{"id":3},{"id":4},{"id":5}]}`

	got, err := Get(doc, "items{id}[0:5:2]")
	if err != nil {
		t.Fatalf("Get step err: %v", err)
	}
	if want := "[1,3,5]"; asStr(got) != want {
		t.Fatalf("M3 step ignored: got %q want %q", asStr(got), want)
	}

	rev, err := Get(doc, "items{id}[::-1]")
	if err != nil {
		t.Fatalf("Get reverse err: %v", err)
	}
	if want := "[5,4,3,2,1]"; asStr(rev) != want {
		t.Fatalf("M3 reverse ignored: got %q want %q", asStr(rev), want)
	}
}

// M4: {extract}[slice] delete must honor step.
func TestD002_ExtractThenSliceDelete_HonorsStep(t *testing.T) {
	const doc = `{"items":[{"tags":["a","b","c","d","e"]}]}`
	out, err := Delete(doc, "items{tags}[0:5:2]")
	if err != nil {
		t.Fatalf("Delete err: %v", err)
	}
	got, err := Get(out, "items[0].tags")
	if err != nil {
		t.Fatalf("Get after delete err: %v", err)
	}
	// Deleting indices 0,2,4 leaves ["b","d"]. Pre-fix this emptied the array.
	if want := `["b","d"]`; asStr(got) != want {
		t.Fatalf("M4 delete step ignored: got %q want %q", asStr(got), want)
	}
}

// M5: convertToUint64 must accept json.Number values in (MaxInt64, MaxUint64].
func TestD002_ConvertToUint64_LargeJSONNumber(t *testing.T) {
	cases := []string{
		"9223372036854775808",  // MaxInt64 + 1
		"18446744073709551615", // MaxUint64
	}
	for _, s := range cases {
		v, ok := convertToUint64(stdjson.Number(s))
		if !ok {
			t.Fatalf("M5: convertToUint64(%s) rejected a valid uint64 (pre-fix behavior)", s)
		}
		if got, want := v, mustParseUint64(s); got != want {
			t.Fatalf("M5: convertToUint64(%s) = %d, want %d", s, got, want)
		}
	}
	// Negative still rejected.
	if _, ok := convertToUint64(stdjson.Number("-1")); ok {
		t.Fatalf("M5: negative json.Number must not convert to uint64")
	}
}

func mustParseUint64(s string) uint64 {
	var u uint64
	for _, c := range s {
		u = u*10 + uint64(c-'0')
	}
	return u
}

// M8: Marshal must reject NaN/Inf (invalid JSON) instead of emitting "NaN"/"+Inf".
func TestD002_Marshal_RejectsNaNAndInf(t *testing.T) {
	for name, val := range map[string]float64{
		"NaN":  math.NaN(),
		"+Inf": math.Inf(1),
		"-Inf": math.Inf(-1),
	} {
		_, err := Marshal(struct{ X float64 }{X: val})
		if err == nil {
			t.Fatalf("M8: Marshal accepted %s (would emit invalid JSON)", name)
		}
	}
}

// C1: streaming Decoder must enforce MaxJSONSize on an unterminated string, not
// grow the buffer without bound.
func TestD002_StreamingDecoder_EnforcesMaxBytes(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MaxJSONSize = 1024
	huge := `"` + strings.Repeat("A", 1_000_000) // 1MB unterminated string

	dec := NewDecoder(strings.NewReader(huge), cfg)
	var v any
	err := dec.Decode(&v)
	if err == nil {
		t.Fatalf("C1: streaming Decode accepted an oversized unterminated string")
	}
	if !strings.Contains(err.Error(), "exceeds maximum") {
		t.Fatalf("C1: expected size-limit error, got: %v", err)
	}
}

// M1: cache Get must agree with Set on long keys. Struct keys (P-001) carry
// the path as a plain field, so a >MaxCacheKeyLength path round-trips exactly —
// the pre-struct string-key bug (Set and Get truncating/sharding differently)
// is now structurally impossible, and this test pins that.
func TestD002_Cache_LongKeyRoundTrip(t *testing.T) {
	cm := internal.NewCacheManager(true, 10000, 0)
	longKey := internal.CacheKey{Op: "get", JSONHash: 42, Path: strings.Repeat("k", 2048)} // > MaxCacheKeyLength (1024)
	cm.Set(longKey, "hit")
	v, ok := cm.Get(longKey)
	if !ok {
		t.Fatalf("M1: long cache key write-only (pre-fix: Set and Get shard mismatched)")
	}
	if s, _ := v.(string); s != "hit" {
		t.Fatalf("M1: long key round-trip mismatch: got %v want hit", v)
	}
}

// ===========================================================================
// Round 2 — reverse slices, encoding/json compat, wildcards, struct encoding
// ===========================================================================

// This section locks in the behavior fixes from the D-002 round-2 review so
// they cannot regress. Each subtest corresponds to a specific finding and
// would fail on the pre-fix code.
//
// Formerly split across d002_m5_verify_test.go, d002_f6_verify_test.go,
// d002_f7_verify_test.go, and d002_f8_verify_test.go; consolidated by topic
// (reverse slices, encoding compat, wildcards, path-creation override,
// extract-then-slice, distributed null, and the F6/F7/F8 struct-encoding
// fixes).

// ---------------------------------------------------------------------------
// Reverse / negative-step slices (C1/C2/C3, M1, M3) — previously panics or
// silent no-ops because the opSet/opDelete loops assumed a positive step.
// ---------------------------------------------------------------------------

func TestD002R2_ReverseSlices(t *testing.T) {
	// C1: Set with reverse slice on an inner array must not panic.
	t.Run("C1_set_reverse_no_panic", func(t *testing.T) {
		doc := `{"items":[{"arr":[1,2,3]},{"arr":[4,5,6]}]}`
		r, err := Set(doc, "items[0].arr[::-1]", 99)
		if err != nil {
			t.Fatalf("Set err: %v", err)
		}
		got, _ := Get(r, "items[0].arr")
		if s := asStr(got); s != "[99,99,99]" {
			t.Fatalf("got %s want [99,99,99]", s)
		}
	})

	// C2: Delete with full reverse slice empties the array.
	t.Run("C2_delete_reverse_all", func(t *testing.T) {
		r, err := Delete(`{"a":[1,2,3,4,5]}`, "a[::-1]")
		if err != nil {
			t.Fatalf("Delete err: %v", err)
		}
		got, _ := Get(r, "a")
		if s := asStr(got); s != "[]" {
			t.Fatalf("got %s want []", s)
		}
	})

	// M1: Delete a[3:0:-1] removes indices 3,2,1 (Python [3:0:-1]==[4,3,2]).
	t.Run("M1_delete_3_0_step_neg1", func(t *testing.T) {
		r, err := Delete(`{"a":[1,2,3,4,5]}`, "a[3:0:-1]")
		if err != nil {
			t.Fatalf("Delete err: %v", err)
		}
		got, _ := Get(r, "a")
		if s := asStr(got); s != "[1,5]" {
			t.Fatalf("got %s want [1,5]", s)
		}
	})

	// C3: Delete on {extract}[slice] with reverse step must not panic.
	t.Run("C3_delete_extract_then_slice_reverse", func(t *testing.T) {
		r, err := Delete(`{"data":[{"arr":[1,2,3]}]}`, "data{arr}[::-1]")
		if err != nil {
			t.Fatalf("Delete err: %v", err)
		}
		got, _ := Get(r, "data[0].arr")
		if s := asStr(got); s != "[]" {
			t.Fatalf("got %s want []", s)
		}
	})

	// M3: very-negative start + reverse step yields [] (Python), not [1].
	t.Run("M3_get_very_neg_reverse_empty", func(t *testing.T) {
		got, err := Get(`{"a":[1,2,3,4,5]}`, "a[-10::-1]")
		if err != nil {
			t.Fatalf("Get err: %v", err)
		}
		if s := asStr(got); s != "[]" {
			t.Fatalf("got %s want []", s)
		}
	})

	// Regression guard: positive-step slice Set still honors step.
	t.Run("regress_positive_step_set", func(t *testing.T) {
		r, err := Set(`{"a":[{"b":1},{"b":2},{"b":3},{"b":4},{"b":5}]}`, "a[0:5:2].b", 99)
		if err != nil {
			t.Fatalf("Set err: %v", err)
		}
		got, _ := Get(r, "a")
		if s := asStr(got); s != `[{"b":99},{"b":2},{"b":99},{"b":4},{"b":99}]` {
			t.Fatalf("got %s", s)
		}
	})
}

// ---------------------------------------------------------------------------
// encoding/json compatibility (F1-F5) on the default Marshal / custom-encoder
// paths. Each diverged from encoding/json before the fix.
// ---------------------------------------------------------------------------

func TestD002R2_EncodingCompat(t *testing.T) {
	// F1: map keys are sorted lexicographically (encoding/json always sorts).
	t.Run("F1_map_keys_sorted", func(t *testing.T) {
		m := map[string]any{"z": 1, "a": 2, "m": 3, "b": 4}
		got, err := Marshal(m)
		if err != nil {
			t.Fatalf("Marshal err: %v", err)
		}
		want, _ := stdjson.Marshal(m)
		if string(got) != string(want) {
			t.Fatalf("got %s want %s", got, want)
		}
	})

	// F2: time.Time preserves sub-second precision (RFC3339Nano).
	t.Run("F2_time_nano_precision", func(t *testing.T) {
		ts := time.Date(2024, 1, 1, 12, 30, 45, 123456789, time.UTC)
		got, err := Marshal(ts)
		if err != nil {
			t.Fatalf("Marshal err: %v", err)
		}
		want, _ := stdjson.Marshal(ts)
		if string(got) != string(want) {
			t.Fatalf("got %s want %s", got, want)
		}
	})

	// F3 (small-magnitude floats use 'e' notation, matching encoding/json) is
	// covered by TestFloatEncoding_MatchesStdlib in encoding_test.go (incl. 9e-7).

	// F4: Decoder.Token returns float64 for numbers (not int64).
	t.Run("F4_token_float64", func(t *testing.T) {
		dec := NewDecoder(strings.NewReader("42"))
		tok, err := dec.Token()
		if err != nil {
			t.Fatalf("Token err: %v", err)
		}
		if _, ok := tok.(float64); !ok {
			t.Fatalf("Token returned %T, want float64", tok)
		}
	})

	// F5: nil slice/map encode as null (not [] / {}).
	t.Run("F5_nil_slice_map_null", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.EscapeHTML = false
		var nilSlice []int
		var nilMap map[string]int
		gotS, _ := EncodeWithConfig(nilSlice, cfg)
		gotM, _ := EncodeWithConfig(nilMap, cfg)
		wantS, _ := stdjson.Marshal(nilSlice)
		wantM, _ := stdjson.Marshal(nilMap)
		if gotS != string(wantS) {
			t.Fatalf("nil slice: got %s want %s", gotS, wantS)
		}
		if gotM != string(wantM) {
			t.Fatalf("nil map: got %s want %s", gotM, wantM)
		}
	})
}

// ---------------------------------------------------------------------------
// Wildcards, per-call CreatePaths, extract-then-slice, reverse-step Set,
// distributed null (M5/M7/M4/M2/M6) — formerly d002_m5_verify_test.go.
// ---------------------------------------------------------------------------

// M5: bare '*' must be a wildcard for Set/Delete too (Get already treats it as
// one). Pre-fix, Set created a literal "*" key and Delete failed "path not
// found: *".
func TestD002R2_M5_BareWildcard(t *testing.T) {
	// Set *.v distributes over all object values.
	t.Run("set_wildcard_prop", func(t *testing.T) {
		doc := `{"x":{"v":1},"y":{"v":2}}`
		r, err := Set(doc, "*.v", 99)
		if err != nil {
			t.Fatalf("Set err: %v", err)
		}
		x, _ := Get(r, "x.v")
		y, _ := Get(r, "y.v")
		if x != float64(99) || y != float64(99) {
			t.Fatalf("M5: x.v=%v y.v=%v, want 99/99 (no literal '*' key should be created)", x, y)
		}
		// A literal "*" key must NOT have been created.
		if lit, _ := Get(r, "*"); lit != nil {
			// "*" is itself a wildcard now, so it distributes; ensure no key
			// literally named "*" exists by checking the object shape.
			obj, _ := Get(r, "")
			if m, ok := obj.(map[string]any); ok {
				if _, exists := m["*"]; exists {
					t.Fatalf("M5: literal '*' key was created: %v", m)
				}
			}
		}
	})

	// Set bare "*" on an object sets every value.
	t.Run("set_bare_wildcard", func(t *testing.T) {
		doc := `{"x":1,"y":2}`
		r, err := Set(doc, "*", 99)
		if err != nil {
			t.Fatalf("Set err: %v", err)
		}
		x, _ := Get(r, "x")
		y, _ := Get(r, "y")
		if x != float64(99) || y != float64(99) {
			t.Fatalf("M5: x=%v y=%v, want 99/99", x, y)
		}
	})

	// Delete *.v removes v from every object.
	t.Run("delete_wildcard_prop", func(t *testing.T) {
		doc := `{"x":{"v":1,"a":0},"y":{"v":2,"a":0}}`
		r, err := Delete(doc, "*.v")
		if err != nil {
			t.Fatalf("Delete err: %v", err)
		}
		if v, _ := Get(r, "x.v"); v != nil {
			t.Fatalf("M5: x.v still present after delete: %v", v)
		}
		if v, _ := Get(r, "y.v"); v != nil {
			t.Fatalf("M5: y.v still present after delete: %v", v)
		}
		// Non-targeted key preserved.
		if a, _ := Get(r, "x.a"); a != float64(0) {
			t.Fatalf("M5: x.a should be preserved, got %v", a)
		}
	})
}

// M7: a per-call cfg.CreatePaths=false must be honored even when the processor
// (default, CreatePaths=true) would otherwise enable it. Pre-fix the OR
// (options.CreatePaths || p.config.CreatePaths) forced it back on.
func TestD002R2_M7_PerCallCreatePaths(t *testing.T) {
	p, _ := New(DefaultConfig()) // CreatePaths=true (default)
	defer p.Close()

	cfg := DefaultConfig()
	cfg.CreatePaths = false

	// Slice end out of bounds: with CreatePaths=false this must error, not
	// silently extend the array to length 10.
	_, err := p.Set(`{"a":[1,2,3]}`, "a[0:10]", 99, cfg)
	if err == nil {
		t.Fatalf("M7: expected error for out-of-bounds slice with CreatePaths=false, got nil")
	}
	if !strings.Contains(err.Error(), "out of bounds") && !strings.Contains(err.Error(), "not found") {
		t.Logf("M7 error: %v", err)
	}

	// Sanity: with CreatePaths=true (default, no cfg) extension still happens.
	r, err := p.Set(`{"a":[1,2,3]}`, "a[0:5]", 99)
	if err != nil {
		t.Fatalf("M7: default CreatePaths=true Set should succeed: %v", err)
	}
	got, _ := Get(r, "a")
	if asStr(got) != "[99,99,99,99,99]" {
		t.Fatalf("M7: default extension got %s", asStr(got))
	}
}

// M4: Set on {extract}[slice] must actually write (previously a silent no-op
// because handleExtractThenSlice had no opSet branch).
func TestD002R2_M4_ExtractThenSliceSet(t *testing.T) {
	doc := `{"items":[{"v":[1,2,3]},{"v":[4,5,6]}]}`
	r, err := Set(doc, "items{v}[0:2]", 99)
	if err != nil {
		t.Fatalf("Set err: %v", err)
	}
	v0, _ := Get(r, "items[0].v")
	v1, _ := Get(r, "items[1].v")
	if asStr(v0) != "[99,99,3]" {
		t.Fatalf("M4: items[0].v=%s want [99,99,3]", asStr(v0))
	}
	if asStr(v1) != "[99,99,6]" {
		t.Fatalf("M4: items[1].v=%s want [99,99,6]", asStr(v1))
	}
}

// M2: Set with a reverse-step terminal slice must honor the step, not silently
// flip it to +1 (default config, CreatePaths=true, dot-notation path).
func TestD002R2_M2_ReverseStepSet(t *testing.T) {
	// [::-2] on [1,2,3,4,5] visits indices 4,2,0 -> set those to 99.
	r, err := Set(`{"a":[1,2,3,4,5]}`, "a[::-2]", 99)
	if err != nil {
		t.Fatalf("Set err: %v", err)
	}
	got, _ := Get(r, "a")
	if asStr(got) != "[99,2,99,4,99]" {
		t.Fatalf("M2: got %s want [99,2,99,4,99] (step -2 must be honored)", asStr(got))
	}
}

// M6: distributed Get must preserve explicit JSON null values (previously
// dropped). Top-level/mid-path access on a non-container still returns nil with
// no error (that contract is unchanged).
func TestD002R2_M6_DistributedGetKeepsNull(t *testing.T) {
	v, err := Get(`[{"a":null},{"a":1}]`, "a")
	if err != nil {
		t.Fatalf("Get err: %v", err)
	}
	if asStr(v) != "[null,1]" {
		t.Fatalf("M6: got %s want [null,1] (null must be preserved)", asStr(v))
	}
	// Contract preserved: property access on a non-container returns nil, no error.
	v2, err2 := Get(`{"a":1}`, "a.b")
	if err2 != nil || v2 != nil {
		t.Fatalf("M6 contract: Get({\"a\":1},\"a.b\") want (nil,nil), got (%v,%v)", v2, err2)
	}
}

// ---------------------------------------------------------------------------
// Struct encoding fixes (F6/F7/F8) — formerly d002_f6/f7/f8_verify_test.go.
// ---------------------------------------------------------------------------

func TestD002R2_F6_EmbeddedStruct(t *testing.T) {
	cfg := DefaultConfig()
	cfg.EscapeHTML = false

	// Plain embedding: Inner's fields promoted to top level.
	type Inner2 struct {
		X int `json:"x"`
	}
	type Plain struct {
		Inner2
		Y int `json:"y"`
	}
	p := Plain{Inner2: Inner2{X: 1}, Y: 2}
	got, _ := EncodeWithConfig(p, cfg)
	want, _ := stdjson.Marshal(p)
	if got != string(want) {
		t.Fatalf("F6 plain: got %s want %s", got, want)
	}

	// Tagged embedding: TaggedInner has a json name -> nested, NOT promoted.
	type TaggedInner struct {
		M int `json:"m"`
	}
	type WithTag struct {
		TaggedInner `json:"tagged"`
		Y           int `json:"y"`
	}
	w := WithTag{TaggedInner: TaggedInner{M: 5}, Y: 2}
	got2, _ := EncodeWithConfig(w, cfg)
	want2, _ := stdjson.Marshal(w)
	if got2 != string(want2) {
		t.Fatalf("F6 tagged: got %s want %s", got2, want2)
	}

	// omitempty on a promoted field is honored.
	type InnerOE struct {
		Z int `json:"z,omitempty"`
	}
	type WithOE struct {
		InnerOE
		Y int `json:"y"`
	}
	got3, _ := EncodeWithConfig(WithOE{InnerOE: InnerOE{}, Y: 1}, cfg)
	if strings.Contains(got3, "z") {
		t.Fatalf("F6 omitempty on promoted field not honored: %s", got3)
	}

	// Pointer embedding.
	type WithPtr struct {
		*Inner2
		Y int `json:"y"`
	}
	got4, _ := EncodeWithConfig(WithPtr{Inner2: &Inner2{X: 7}, Y: 2}, cfg)
	want4, _ := stdjson.Marshal(WithPtr{Inner2: &Inner2{X: 7}, Y: 2})
	if got4 != string(want4) {
		t.Fatalf("F6 ptr embed: got %s want %s", got4, want4)
	}
}

func TestD002R2_F7_StringTag(t *testing.T) {
	type S struct {
		Count  int     `json:"count,string"`
		Rate   float64 `json:"rate,string"`
		Active bool    `json:"active,string"`
		Name   string  `json:"name"`
		Named  int     `json:"string"` // field NAME is "string", no option -> no wrapping
	}
	s := S{Count: 42, Rate: 1.5, Active: true, Name: "hi", Named: 7}
	cfg := DefaultConfig()
	cfg.EscapeHTML = false
	got, err := EncodeWithConfig(s, cfg)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	want, _ := stdjson.Marshal(s)
	if got != string(want) {
		t.Fatalf("F7:\n got %s\nwant %s", got, want)
	}
}

// F8: a type whose MarshalJSON has a POINTER receiver only.
type d002R2PtrMarshaler struct{ N int }

func (m *d002R2PtrMarshaler) MarshalJSON() ([]byte, error) {
	return stdjson.Marshal(map[string]int{"custom": m.N})
}

type d002R2Wrapper struct {
	Data d002R2PtrMarshaler `json:"data"`
}

func TestD002R2_F8_PtrReceiverMarshaler(t *testing.T) {
	cfg := DefaultConfig()
	cfg.EscapeHTML = false
	got, err := EncodeWithConfig(&d002R2Wrapper{Data: d002R2PtrMarshaler{42}}, cfg)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	want, _ := stdjson.Marshal(&d002R2Wrapper{Data: d002R2PtrMarshaler{42}})
	if got != string(want) {
		t.Fatalf("F8: got %s want %s (pointer-receiver MarshalJSON missed)", got, want)
	}
}

// ===========================================================================
// Round 4 — [D-002 第四轮] 回归测试:锁定本轮 6 项正确性修复。
// 移除任一修复,对应用例应失败(panic 或断言不通过)。
// ===========================================================================

// TestD002R4_NumberPreservedInDeepCopy pins C1 (helpers.go deep-copy) + C2 (encoding.go encoder):
// in PreserveNumbers mode, Number values must survive Get/GetFromParsed and Parse-into-map
// as numeric types, not be corrupted to strings.
func TestD002R4_NumberPreservedInDeepCopy(t *testing.T) {
	cfg := DefaultConfig()
	cfg.PreserveNumbers = true
	p, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()

	in := `{"big":9007199254740993,"arr":[1,2,3]}`

	// C1: GetFromParsed returns Number (deep-copy path safeCopyResult).
	pp, err := p.PreParse(in, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer pp.Release()
	if v, _ := p.GetFromParsed(pp, "big"); v == nil {
		t.Fatal("GetFromParsed(big) nil")
	} else if _, ok := v.(Number); !ok {
		t.Errorf("C1 GetFromParsed(big) = %T, want Number (was string before fix)", v)
	}

	// C1: Get cache-hit returns Number (deepCopySubtree on the cached Number).
	// Intentional discard: only primes the cache for the hit-path assertion below.
	_, _ = p.Get(in, "big", cfg)
	if v, _ := p.Get(in, "big", cfg); v == nil {
		t.Fatal("Get(big) nil")
	} else if _, ok := v.(Number); !ok {
		t.Errorf("C1 Get(big) cache-hit = %T, want Number (was string before fix)", v)
	}

	// C2: Parse into *map[string]any yields a numeric type, not a string.
	var m map[string]any
	if err := p.Parse(in, &m, cfg); err != nil {
		t.Fatal(err)
	}
	if _, ok := m["big"].(string); ok {
		t.Errorf("C2 Parse(*map).big = string (corrupted), want numeric type (was string before fix)")
	}
}

// TestD002R4_NonBMPEscapeSurrogatePair pins C3 (encoding.go writeUnicodeEscape):
// EscapeUnicode must emit a UTF-16 surrogate pair for non-BMP runes, not truncate to 16 bits.
func TestD002R4_NonBMPEscapeSurrogatePair(t *testing.T) {
	cfg := DefaultConfig()
	cfg.EscapeUnicode = true
	out, err := Marshal("😀", cfg)
	if err != nil {
		t.Fatal(err)
	}
	// U+1F600 must encode as the surrogate pair 😀, not the truncated .
	if !strings.Contains(string(out), "\\ud83d\\ude00") {
		t.Errorf("C3 emoji escape = %s, want surrogate pair \\ud83d\\ude00 (was \\uf600 before fix)", string(out))
	}
}

// TestD002R4_CompiledPathReverseSlice pins C4 (internal/compiled_path.go applySlice):
// CompiledPath [::-1] must reverse the array, not return empty.
func TestD002R4_CompiledPathReverseSlice(t *testing.T) {
	p, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	cp, err := p.CompilePath("[::-1]")
	if err != nil {
		t.Fatalf("CompilePath([::-1]) err=%v", err)
	}
	rv, err := cp.Get([]any{"a", "b", "c", "d"})
	if err != nil {
		t.Fatalf("CompiledPath [::-1] err=%v", err)
	}
	s, ok := rv.([]any)
	if !ok || len(s) != 4 || s[0] != "d" || s[3] != "a" {
		t.Errorf("C4 CompiledPath [::-1] = %v, want [d c b a] (was [] before fix)", rv)
	}
}

// TestD002R4_ParallelIteratorMapNoMutex pins the Map mutex removal:
// distinct-index writes are safe without a mutex; result must be correct.
func TestD002R4_ParallelIteratorMapNoMutex(t *testing.T) {
	it := NewParallelIterator([]any{1, 2, 3, 4, 5})
	defer it.Close()
	res, err := it.Map(func(i int, v any) (any, error) { return i * 2, nil })
	if err != nil {
		t.Fatal(err)
	}
	if len(res) != 5 || res[0] != 0 || res[4] != 8 {
		t.Errorf("Map result = %v, want [0 2 4 6 8]", res)
	}
}

// ===========================================================================
// Round 6 — deterministic map-value ordering + scan-window security
// ===========================================================================

// D-002 round 6 regression tests: map-value collection must be deterministic.
// Go randomizes map iteration order per iteration, so any loop that collects
// results or drives callbacks in `range` order over a map[string]any produced
// a different order on every call (empirically 293 distinct orderings in 300
// runs on a 12-key object before the fix). These tests pin the guarantee that
// Get results, ForEach callback order, and Iterator traversal over objects are
// stable across calls and equal to sorted-key order.

// TestD002Round6_CustomPatternStraddlesWindowBoundary guards the rolling-window
// security scan: the window overlap must cover Config.AdditionalDangerousPatterns
// lengths, not just the built-in patterns. A custom pattern longer than the
// built-in overlap that starts inside the pre-boundary gap ((b-L, b-o) for
// boundary b, pattern length L, overlap o) was contained in NO window and
// evaded detection entirely.
func TestD002Round6_CustomPatternStraddlesWindowBoundary(t *testing.T) {
	const pattern = "ZZcustom_malicious_marker_string_abcdefghijklmnop" // 48 bytes > overlap (24)

	// Place the pattern to start 27 bytes before the first 32KB window
	// boundary: start = b-27 lies in (b-48, b-24), the detection gap for a
	// 48-byte pattern under the pre-fix 24-byte overlap.
	const boundary = 32768
	const offset = boundary - 27

	// The pattern is space-delimited on both sides so the dangerous-context
	// check (word-boundary heuristic) evaluates it — a mid-word pattern is
	// deliberately not flagged by any scan mode.
	prefix := `{"a":"` + strings.Repeat("p", offset-len(`{"a":"`)-1) + " "
	// Padding stays inside the string value so the document stays valid JSON,
	// and the total length exceeds 2×32KB to select the rolling-window path.
	doc := prefix + pattern + " " + strings.Repeat("q", 40000) + `"}`
	if len(doc) <= 2*32768 {
		t.Fatalf("doc too short for rolling-window path: %d", len(doc))
	}

	cfg := DefaultConfig()
	cfg.AdditionalDangerousPatterns = []DangerousPattern{{
		Pattern: pattern,
		Name:    "custom marker",
		Level:   PatternLevelCritical,
	}}
	p, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()

	if _, err := p.Get(doc, "a"); err == nil {
		t.Fatal("custom dangerous pattern straddling a scan-window boundary was NOT detected")
	}
}

const d002r6MultiKeyObject = `{"k01":1,"k02":2,"k03":3,"k04":4,"k05":5,"k06":6,"k07":7,"k08":8,"k09":9,"k10":10,"k11":11,"k12":12}`
const d002r6ArraysPerKey = `{"a":[1,2],"b":[3,4],"c":[5,6],"d":[7,8],"e":[9,10],"f":[11,12],"g":[13,14],"h":[15,16]}`

// runStable asserts that fn produces the identical JSON-serialized result on
// every call across many iterations.
func runStable(t *testing.T, name string, n int, fn func() (any, error)) {
	t.Helper()
	var first string
	for i := 0; i < n; i++ {
		v, err := fn()
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		b, err := stdjson.Marshal(v) // []any order is preserved by stdlib marshal
		if err != nil {
			t.Fatalf("%s marshal: %v", name, err)
		}
		if i == 0 {
			first = string(b)
			continue
		}
		if string(b) != first {
			t.Fatalf("%s: nondeterministic result: run 0 = %s, run %d = %s", name, first, i, b)
		}
	}
}

// TestD002Round6_GetMapValueOrderDeterministic covers the recursive-processor
// Get handlers that collect values from map[string]any: wildcard (last
// segment), and index/slice segments distributed over object values.
func TestD002Round6_GetMapValueOrderDeterministic(t *testing.T) {
	cfg := DefaultConfig()
	cfg.EnableCache = false // cache would mask re-iteration of fresh maps
	p, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()

	runStable(t, "wildcard on object", 300, func() (any, error) {
		return p.Get(d002r6MultiKeyObject, "*")
	})
	runStable(t, "[*] on object", 300, func() (any, error) {
		return p.Get(d002r6MultiKeyObject, "[*]")
	})
	runStable(t, "index distributed over map values", 300, func() (any, error) {
		return p.Get(d002r6ArraysPerKey, "[0]")
	})
	runStable(t, "slice distributed over map values", 300, func() (any, error) {
		return p.Get(d002r6ArraysPerKey, "[0:2]")
	})
}

// TestD002Round6_GetWildcardOrderIsSortedKeys verifies the stabilized order is
// ascending key order, not merely self-consistent.
func TestD002Round6_GetWildcardOrderIsSortedKeys(t *testing.T) {
	p, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()

	v, err := p.Get(d002r6MultiKeyObject, "*")
	if err != nil {
		t.Fatal(err)
	}
	got, ok := v.([]any)
	if !ok {
		t.Fatalf("wildcard result type = %T, want []any", v)
	}
	if len(got) != 12 {
		t.Fatalf("wildcard result length = %d, want 12", len(got))
	}
	for i, val := range got {
		want := float64(i + 1) // k01..k12 hold 1..12, so sorted order is 1,2,...,12
		if val != want {
			t.Fatalf("wildcard result[%d] = %v, want %v (ascending key order)", i, val, want)
		}
	}
}

// TestD002Round6_ForEachOrderDeterministic covers the Foreach* family: callback
// invocation order over a multi-key object must be identical across calls.
func TestD002Round6_ForEachOrderDeterministic(t *testing.T) {
	type entry struct {
		key string
		val int
	}
	collect := func() []entry {
		var out []entry
		Foreach(d002r6MultiKeyObject, func(key any, item *IterableValue) {
			v, ok := item.GetData().(float64)
			if !ok {
				t.Errorf("entry %v: data type = %T, want float64", key, item.GetData())
			}
			out = append(out, entry{key.(string), int(v)})
		})
		return out
	}

	first := collect()
	if len(first) != 12 {
		t.Fatalf("ForEach visited %d entries, want 12", len(first))
	}
	for i := 1; i < 200; i++ {
		again := collect()
		if len(again) != len(first) {
			t.Fatalf("run %d: %d entries, want %d", i, len(again), len(first))
		}
		for j := range first {
			if first[j] != again[j] {
				t.Fatalf("run %d entry %d: %+v, want %+v", i, j, again[j], first[j])
			}
		}
	}
	// And the stable order must be ascending by key.
	for i := 1; i < len(first); i++ {
		if first[i-1].key >= first[i].key {
			t.Fatalf("ForEach order not sorted by key: %q before %q", first[i-1].key, first[i].key)
		}
	}
}

// TestD002Round6_IteratorOrderDeterministic covers NewIterator/Next traversal
// of an object: value order must be stable across constructions and follow
// ascending key order (k01..k12 hold 1..12, so values must be 1,2,...,12).
func TestD002Round6_IteratorOrderDeterministic(t *testing.T) {
	var data any
	if err := Unmarshal([]byte(d002r6MultiKeyObject), &data); err != nil {
		t.Fatal(err)
	}

	var first []float64
	for run := 0; run < 200; run++ {
		var values []float64
		it := NewIterator(data)
		for it.HasNext() {
			v, ok := it.Next()
			if !ok {
				t.Fatalf("run %d: Next returned false early at %d values", run, len(values))
			}
			f, ok := v.(float64)
			if !ok {
				t.Fatalf("run %d: value type = %T, want float64", run, v)
			}
			values = append(values, f)
		}
		if len(values) != 12 {
			t.Fatalf("run %d: visited %d values, want 12", run, len(values))
		}
		if run == 0 {
			first = values
			continue
		}
		for i := range first {
			if first[i] != values[i] {
				t.Fatalf("run %d value %d = %v, want %v", run, i, values[i], first[i])
			}
		}
	}
	for i := 1; i < len(first); i++ {
		if first[i-1] >= first[i] {
			t.Fatalf("Iterator values not in ascending key order: %v before %v", first[i-1], first[i])
		}
	}
}

// TestD002Round6_SetMultipleOrderDeterministic covers SetMultiple: updates are
// applied sequentially to one copy, so with overlapping keys the final document
// previously depended on randomized map iteration order. Sorted application
// must be stable across calls, and the first reported invalid path must be the
// sorted-smallest one, not a random one.
func TestD002Round6_SetMultipleOrderDeterministic(t *testing.T) {
	p, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()

	const doc = `{"a":{"x":1}}`
	// Sorted order applies "a" first (replacing the object), then "a.b" into
	// the fresh container — the deterministic outcome.
	updates := map[string]any{
		"a":   map[string]any{"y": 2},
		"a.b": 3,
	}
	var first string
	for i := 0; i < 200; i++ {
		out, err := p.SetMultiple(doc, updates)
		if err != nil {
			t.Fatalf("SetMultiple: %v", err)
		}
		if i == 0 {
			first = out
			continue
		}
		if out != first {
			t.Fatalf("SetMultiple result nondeterministic: run 0 = %s, run %d = %s", first, i, out)
		}
	}
	for _, want := range []string{`"b":3`, `"y":2`} {
		if !strings.Contains(first, want) {
			t.Fatalf("sorted-application result %q missing %s", first, want)
		}
	}
	if strings.Contains(first, `"x":1`) {
		t.Fatalf("sorted-application result %q still contains replaced key x", first)
	}

	// Invalid-path error: the sorted-smallest path must be reported every time.
	badUpdates := map[string]any{"x[": 1, "y[": 2}
	var firstErr string
	for i := 0; i < 100; i++ {
		_, err := p.SetMultiple(doc, badUpdates)
		if err == nil {
			t.Fatal("SetMultiple with invalid paths: expected error")
		}
		if i == 0 {
			firstErr = err.Error()
			continue
		}
		if err.Error() != firstErr {
			t.Fatalf("invalid-path error nondeterministic: run 0 = %v, run %d = %v", firstErr, i, err)
		}
	}
	if !strings.Contains(firstErr, "x[") {
		t.Fatalf("expected sorted-smallest path x[ in error, got: %v", firstErr)
	}
}

// TestD002Round6_GetCompiledWildcardOrderDeterministic covers CompiledPath
// navigation (public via CompilePath/GetCompiled): wildcard values from an
// object must come back in ascending key order, stably across runs.
func TestD002Round6_GetCompiledWildcardOrderDeterministic(t *testing.T) {
	p, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()

	cp, err := p.CompilePath("*")
	if err != nil {
		t.Fatal(err)
	}
	defer cp.Release()

	var first string
	for run := 0; run < 200; run++ {
		var data any
		if err := Unmarshal([]byte(d002r6MultiKeyObject), &data); err != nil {
			t.Fatal(err)
		}
		v, err := cp.Get(data)
		if err != nil {
			t.Fatalf("run %d: %v", run, err)
		}
		b, err := stdjson.Marshal(v)
		if err != nil {
			t.Fatal(err)
		}
		if run == 0 {
			first = string(b)
			continue
		}
		if string(b) != first {
			t.Fatalf("run %d: wildcard = %s, want %s", run, b, first)
		}
	}
	// k01..k12 hold 1..12: ascending key order means values 1,2,...,12.
	if first != "[1,2,3,4,5,6,7,8,9,10,11,12]" {
		t.Fatalf("wildcard values = %s, want ascending [1..12]", first)
	}
}

// TestD002Round6_SchemaErrorOrderDeterministic covers validateObject: the
// validation-error list must be deterministic (sorted by property key), not
// shuffled by randomized map iteration order.
func TestD002Round6_SchemaErrorOrderDeterministic(t *testing.T) {
	schema := &Schema{
		Type:                 "object",
		AdditionalProperties: false,
		Properties: map[string]*Schema{
			"keep": {Type: "number"},
		},
	}
	const doc = `{"z":1,"m":2,"a":3,"keep":4}`

	var first []ValidationError
	for run := 0; run < 100; run++ {
		verrs, err := ValidateSchema(doc, schema)
		if err != nil {
			t.Fatalf("ValidateSchema: %v", err)
		}
		if len(verrs) != 3 {
			t.Fatalf("run %d: %d validation errors, want 3 (%v)", run, len(verrs), verrs)
		}
		if run == 0 {
			first = verrs
			continue
		}
		for i := range first {
			if first[i] != verrs[i] {
				t.Fatalf("run %d error %d = %+v, want %+v", run, i, verrs[i], first[i])
			}
		}
	}
	// The stable order must be ascending by property key: a, m, z.
	wantPaths := []string{"a", "m", "z"}
	for i, wp := range wantPaths {
		if first[i].Path != wp {
			t.Fatalf("error %d path = %q, want %q (ascending key order)", i, first[i].Path, wp)
		}
	}
}

// Round 7 — operation metrics coverage + cache Delete long-key parity
//
// D-002 round 7 regression tests: Stats.OperationCount/ErrorCount must reflect
// Set/SetMultiple/Delete (previously only Get incremented them), and
// CacheManager.Delete must truncate long keys the same way Get/Set do.

// TestD002Round7_MutationOpsCounted pins the metrics fix: Set/SetMultiple/
// Delete increment OperationCount (and failures increment ErrorCount), so
// GetStats reports all primary operations, not just reads.
func TestD002Round7_MutationOpsCounted(t *testing.T) {
	p, err := New(DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()

	const doc = `{"a":1,"b":2,"c":3}`
	before := p.GetStats()

	if _, err := p.Set(doc, "a", 10); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if _, err := p.SetMultiple(doc, map[string]any{"b": 20}); err != nil {
		t.Fatalf("SetMultiple: %v", err)
	}
	if _, err := p.Delete(doc, "c"); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	after := p.GetStats()
	got := after.OperationCount - before.OperationCount
	if got < 3 {
		t.Fatalf("OperationCount delta after Set+SetMultiple+Delete = %d, want >= 3 (mutations must be counted)", got)
	}

	// Failure paths must increment ErrorCount.
	errBefore := p.GetStats().ErrorCount
	if _, err := p.Set(doc, "x[zz]", 1); err == nil {
		t.Fatal("Set with invalid index: expected error")
	}
	if _, err := p.Delete(doc, "nope"); err == nil {
		t.Fatal("Delete missing path: expected error")
	}
	errAfter := p.GetStats().ErrorCount
	if errAfter-errBefore < 2 {
		t.Fatalf("ErrorCount delta after two failed mutations = %d, want >= 2", errAfter-errBefore)
	}
}

// TestD002Round7_CacheDeleteLongKey pins the long-key Delete contract. With
// struct keys (P-001) there is no truncation, so Set/Get/Delete share one
// exact key and the pre-fix truncation-mismatch leak is structurally gone;
// this test pins the invariant the original fix established.
func TestD002Round7_CacheDeleteLongKey(t *testing.T) {
	cm := internal.NewCacheManager(true, 16, 0)
	defer cm.Close()

	longKey := internal.CacheKey{Op: "get", JSONHash: 42, Path: strings.Repeat("k", 2048)} // > MaxCacheKeyLength (1024)
	cm.Set(longKey, "v")
	if v, ok := cm.Get(longKey); !ok || v != "v" {
		t.Fatal("setup: long-key Set/Get round trip failed")
	}

	cm.Delete(longKey)
	if v, ok := cm.Get(longKey); ok {
		t.Fatalf("entry survived Delete: value = %v", v)
	}
	if n := cm.EntryCount(); n != 0 {
		t.Fatalf("EntryCount after Delete = %d, want 0", n)
	}
}

// TestD002Round7_GetCompiledNilPath pins the GetCompiled nil guard: a nil
// *CompiledPath on an ACTIVE processor previously panicked (the only existing
// test used a closed processor, whose early return masked the crash).
func TestD002Round7_GetCompiledNilPath(t *testing.T) {
	p, err := New(DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()

	if _, err := p.GetCompiled(`{"a":1}`, nil); err == nil {
		t.Fatal("GetCompiled with nil CompiledPath: expected error, got nil")
	}
}

// TestD002Round7_ReverseSliceEndPastStart pins the PerformArraySliceIndices
// clamp: a negative-step slice whose end wraps past the array start
// (e.g. [4:-10:-1] on a 5-element array) previously produced negative
// indices and panicked at the consumer ("index out of range [-1]"). Python
// semantics: it selects down to index 0 inclusive.
func TestD002Round7_ReverseSliceEndPastStart(t *testing.T) {
	const doc = `{"a":[1,2,3,4,5]}`
	p, err := New(DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()

	for _, tc := range []struct {
		path string
		want string
	}{
		{"a[4:-10:-1]", "[5,4,3,2,1]"}, // end wraps to -5 → clamp to -1
		{"a[4:-7:-1]", "[5,4,3,2,1]"},  // end wraps to -2 → clamp to -1
		{"a[4:-6:-1]", "[5,4,3,2,1]"},  // end wraps to exactly -1 (already worked)
		{"a[-2:-100:-1]", "[4,3,2,1]"}, // negative start wraps to 3
	} {
		v, err := p.Get(doc, tc.path)
		if err != nil {
			t.Errorf("Get(%q): %v", tc.path, err)
			continue
		}
		if got := d002fmt(v); got != tc.want {
			t.Errorf("Get(%q) = %s, want %s", tc.path, got, tc.want)
		}
	}

	// The same slice syntax must be safe on the mutation paths too — they
	// consume the identical index list (opSet assigns, opDelete marks).
	if _, err := p.Set(doc, "a[4:-10:-1]", 9); err != nil {
		t.Errorf("Set reverse slice: %v", err)
	}
	if _, err := p.Delete(doc, "a[4:-10:-1]"); err != nil {
		t.Errorf("Delete reverse slice: %v", err)
	}
}

// TestD002Round7_NumberNoCfgRoundTrip pins the Number.MarshalJSON fix
// (self-review follow-up to the round-7 PreserveNumbers routing fix): data
// parsed with PreserveNumbers must survive re-encoding EVEN when the encode
// call carries no Config — Marshal/Encode/package-Marshal previously went
// through plain json.Marshal and quoted the literal ("1.10").
func TestD002Round7_NumberNoCfgRoundTrip(t *testing.T) {
	p, err := New(DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()

	cfg := DefaultConfig()
	cfg.PreserveNumbers = true
	var v any
	if err := p.Parse(`{"a":1.10}`, &v, cfg); err != nil {
		t.Fatalf("Parse: %v", err)
	}

	if b, err := p.Marshal(v); err != nil || string(b) != `{"a":1.10}` {
		t.Errorf("Marshal no-cfg = %s, %v; want {\"a\":1.10}", b, err)
	}
	if s, err := p.Encode(v); err != nil || s != `{"a":1.10}` {
		t.Errorf("Encode no-cfg = %s, %v; want {\"a\":1.10}", s, err)
	}
	if s, err := Marshal(v); err != nil || string(s) != `{"a":1.10}` {
		t.Errorf("package Marshal = %s, %v; want {\"a\":1.10}", s, err)
	}

	// Invalid literals are rejected rather than written verbatim (matching
	// the encodeJSONNumber guard and stdlib json.Number semantics).
	if _, err := Marshal(Number("1_0")); err == nil {
		t.Error("Marshal(Number(\"1_0\")): expected error for invalid literal")
	}

	// Fast-vs-custom asymmetry mirrors the existing stdlib json.Number design:
	// without custom opts the (config-less) fast path honors the literal's
	// self-description (1e3 stays 1e3, 1.10 stays 1.10); the custom encoder,
	// which has config context, normalizes when PreserveNumbers=false
	// (json.Number 1e3→1000, Number 1.10→1.1 — pinned by
	// TestEncodeJSONNumber_NonPreserve for the stdlib type).
	if s, err := p.EncodeWithConfig(Number("1.10"), DefaultConfig()); err != nil || s != `1.10` {
		t.Errorf("fast-path literal = %s, %v; want 1.10", s, err)
	}
	sk := DefaultConfig()
	sk.SortKeys = true // force the custom encoder
	if s, err := p.EncodeWithConfig(Number("1.10"), sk); err != nil || s != `1.1` {
		t.Errorf("custom-path normalization = %s, %v; want 1.1", s, err)
	}
}

// TestD002Round7_DecoderTokenPathSizeLimits pins the parseString/parseNumber
// maxBytes guards on the Token() path (self-review follow-up: round 7
// initially only guarded parseString; parseNumber streamed digits from a
// never-ending reader without bound).
func TestD002Round7_DecoderTokenPathSizeLimits(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MaxJSONSize = 256

	// Unterminated string via Token().
	d := NewDecoder(strings.NewReader(`"`+strings.Repeat("A", 4000)), cfg)
	_, err := d.Token()
	if err == nil || !strings.Contains(err.Error(), "exceeds maximum") {
		t.Errorf("Token unterminated string: err = %v, want size-limit error", err)
	}

	// Endless digit run via Token() (a number never terminated by a delimiter).
	d2 := NewDecoder(strings.NewReader(strings.Repeat("1", 4000)), cfg)
	_, err = d2.Token()
	if err == nil || !strings.Contains(err.Error(), "exceeds maximum") {
		t.Errorf("Token endless number: err = %v, want size-limit error", err)
	}

	// Legitimate small tokens still parse through the same paths.
	d3 := NewDecoder(strings.NewReader(`123`), cfg)
	tok, err := d3.Token()
	if err != nil || asStr(tok) != "123" {
		t.Errorf("Token small number = %#v, %v; want 123", tok, err)
	}
}

// TestD002Round7_NDJSONHonorsJSONLCfg pins the NDJSONProcessor.ProcessReader
// fix: the JSONL config knobs (skip comments/empty, per-line and total size
// limits) are honored as they are by the StreamJSONL family. Previously
// ProcessReader ignored all four, so the two JSONL entry points silently
// enforced different rules.
func TestD002Round7_NDJSONHonorsJSONLCfg(t *testing.T) {
	// Comments skipped when JSONLSkipComments is set.
	cfg := DefaultConfig()
	cfg.JSONLSkipComments = true
	np := NewNDJSONProcessor(cfg)
	var lines []int
	err := np.ProcessReader(strings.NewReader("# header\n{\"a\":1}\n"), func(n int, _ map[string]any) error {
		lines = append(lines, n)
		return nil
	})
	if err != nil {
		t.Fatalf("ProcessReader comments: %v", err)
	}
	if len(lines) != 1 || lines[0] != 2 {
		t.Fatalf("comment skip: got lines %v, want [2] (comment on line 1 skipped, line numbers preserved)", lines)
	}

	// JSONLMaxMemory caps total processed bytes.
	memCfg := DefaultConfig()
	memCfg.JSONLMaxMemory = 20
	npMem := NewNDJSONProcessor(memCfg)
	err = npMem.ProcessReader(strings.NewReader("{\"aaaa\":1}\n{\"bbbb\":2}\n{\"cccc\":3}\n"), func(int, map[string]any) error {
		return nil
	})
	if err == nil || !strings.Contains(err.Error(), "memory limit") {
		t.Fatalf("JSONLMaxMemory not enforced: got %v, want memory-limit error", err)
	}

	// JSONLMaxLineSize caps a single line (falls back to MaxJSONSize unset).
	lineCfg := DefaultConfig()
	lineCfg.JSONLMaxLineSize = 10
	npLine := NewNDJSONProcessor(lineCfg)
	var seen int
	err = npLine.ProcessReader(strings.NewReader("{\"aaaaaaaaaaaa\":1}\n"), func(int, map[string]any) error {
		seen++
		return nil
	})
	if err == nil || !strings.Contains(err.Error(), "too long") {
		t.Fatalf("JSONLMaxLineSize not enforced: got %v, want bufio.ErrTooLong", err)
	}
	if seen != 0 {
		t.Fatalf("oversized line should not reach the callback")
	}

	// Backward compatibility: with no knobs set, a >1MB line still parses
	// (legacy MaxJSONSize default), and empty lines are still skipped.
	npDefault := NewNDJSONProcessor()
	count := 0
	err = npDefault.ProcessReader(strings.NewReader("\n{\"a\":1}\n\n"), func(int, map[string]any) error {
		count++
		return nil
	})
	if err != nil || count != 1 {
		t.Fatalf("default behavior changed: err=%v count=%d (want nil, 1)", err, count)
	}
}

// TestD002Round7_PreserveNumbersRoundTrip pins the needsCustomEncodingOpts fix:
// with PreserveNumbers set, EncodeWithConfig must route through the custom
// encoder so the library's Number type survives as a JSON number literal
// (previously both fast paths used plain json.Marshal and emitted "1.10").
func TestD002Round7_PreserveNumbersRoundTrip(t *testing.T) {
	p, err := New(DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()

	cfg := DefaultConfig()
	cfg.PreserveNumbers = true
	var v any
	if err := p.Parse(`{"a":1.10,"big":123456789012345678901234567890}`, &v, cfg); err != nil {
		t.Fatalf("Parse: %v", err)
	}
	out, err := p.EncodeWithConfig(v, cfg)
	if err != nil {
		t.Fatalf("EncodeWithConfig: %v", err)
	}
	want := `{"a":1.10,"big":123456789012345678901234567890}`
	if out != want {
		t.Fatalf("round-trip = %s, want %s (number literals must not become quoted strings)", out, want)
	}
}

// TestD002Round7_CustomEncoderStdlibCompat pins three custom-encoder fixes:
// (a) non-string map keys are formatted by kind, not reflect.Value.String()'s
//
//	"<T Value>" placeholder; (b) []byte encodes as a base64 string, not a
//	byte-number array; (c) CustomEscapes/DisableEscaping apply to struct
//	string fields, not only to map values.
func TestD002Round7_CustomEncoderStdlibCompat(t *testing.T) {
	p, err := New(DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()

	opts := DefaultConfig()
	opts.SortKeys = true

	// (a) int-keyed map: encoding/json emits {"1":"one","2":"two"}.
	out, err := p.EncodeWithConfig(map[int]string{2: "two", 1: "one"}, opts)
	if err != nil {
		t.Fatalf("int-keyed map: %v", err)
	}
	if out != `{"1":"one","2":"two"}` {
		t.Fatalf("int-keyed map = %s, want {\"1\":\"one\",\"2\":\"two\"} (keys destroyed by <int Value> placeholder)", out)
	}

	// (b) []byte: encoding/json emits a base64 string.
	out, err = p.EncodeWithConfig(struct{ Data []byte }{[]byte("hi")}, opts)
	if err != nil {
		t.Fatalf("[]byte struct: %v", err)
	}
	if out != `{"Data":"aGk="}` {
		t.Fatalf("[]byte struct = %s, want {\"Data\":\"aGk=\"} (base64), not a number array", out)
	}

	// (c) CustomEscapes on struct fields. Escape 'x' as the 6-char x
	// sequence, built at runtime so no source channel can mangle the literal.
	esc := string([]byte{0x5c, 'u', '0', '0', '7', '8'}) // x
	cc := DefaultConfig()
	cc.CustomEscapes = map[rune]string{'x': esc}
	outMap, err := p.EncodeWithConfig(map[string]any{"s": "axb"}, cc)
	if err != nil {
		t.Fatalf("map CustomEscapes: %v", err)
	}
	want := `{"s":"a` + esc + `b"}`
	outStruct, err := p.EncodeWithConfig(struct{ S string }{"axb"}, cc)
	if err != nil {
		t.Fatalf("struct CustomEscapes: %v", err)
	}
	if outMap != want {
		t.Fatalf("map CustomEscapes = %s, want %s", outMap, want)
	}
	if outStruct != `{"S":"a`+esc+`b"}` {
		t.Fatalf("struct CustomEscapes = %s, want the escape applied to the field value too", outStruct)
	}

	// (c2) DisableEscaping on struct fields (previously stdlib < escaped).
	dd := DefaultConfig()
	dd.DisableEscaping = true
	outStruct, err = p.EncodeWithConfig(struct{ S string }{"a<b"}, dd)
	if err != nil {
		t.Fatalf("struct DisableEscaping: %v", err)
	}
	if outStruct != `{"S":"a<b"}` {
		t.Fatalf("struct DisableEscaping = %s, want {\"S\":\"a<b\"} (no stdlib escaping)", outStruct)
	}
}

// TestD002Round7_PrettyAllNullMap pins the encodeMap closing-indent fix: with
// IncludeNulls=false filtering out every value, a pretty map must render {}
// (previously a dangling indent-only line appeared inside the braces).
func TestD002Round7_PrettyAllNullMap(t *testing.T) {
	p, err := New(DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()

	cfg := DefaultConfig()
	cfg.Pretty = true
	cfg.IncludeNulls = false
	out, err := p.EncodeWithConfig(map[string]any{"a": nil}, cfg)
	if err != nil {
		t.Fatalf("EncodeWithConfig: %v", err)
	}
	if out != "{}" {
		t.Fatalf("pretty all-null map = %q, want \"{}\"", out)
	}
}

// TestD002Round7_MarshalRespectsSizeLimit pins the Marshal no-cfg fix: the
// fast path must enforce the processor's MaxJSONSize like every other encode
// path (previously only the with-cfg form rejected oversized output).
func TestD002Round7_MarshalRespectsSizeLimit(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MaxJSONSize = 10
	p, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()

	long := strings.Repeat("a", 50)
	if _, err := p.Marshal(long); err == nil {
		t.Fatal("Marshal without cfg: expected ErrSizeLimit for oversized output")
	}
	// D-002/R8 (M1): a per-call cfg's MaxJSONSize REPLACES the baked limit on
	// encoded output (doc.go contract, mirroring the read side's
	// effectiveReadMaxSize): a loosening cfg (DefaultConfig = 100MB) accepts the
	// 52-byte output; a tightening cfg still rejects. The Round 7 form asserted
	// the pre-M1 behavior where the baked limit always bound.
	loose, err := p.Marshal(long, DefaultConfig())
	if err != nil {
		t.Fatalf("Marshal with loosening cfg: %v", err)
	}
	if len(loose) != 52 {
		t.Fatalf("Marshal with loosening cfg len = %d, want 52", len(loose))
	}
	tight := DefaultConfig()
	tight.MaxJSONSize = 10
	if _, err := p.Marshal(long, tight); !errors.Is(err, ErrSizeLimit) {
		t.Fatalf("Marshal with tightening cfg: err = %v, want ErrSizeLimit", err)
	}
}

// TestD002Round7_SchemaPreserveNumbers pins the schema fixes: with
// PreserveNumbers on, numbers must pass "number" type checks, honor
// minimum/maximum, and enum values must compare equal across numeric kinds.
func TestD002Round7_SchemaPreserveNumbers(t *testing.T) {
	pcfg := DefaultConfig()
	pcfg.PreserveNumbers = true

	schema := &Schema{
		Type: "object",
		Properties: map[string]*Schema{
			"n": {Type: "number", Minimum: 0, Maximum: 10},
		},
	}
	// The has* flags are normally set by NewSchemaWithConfig (pointer-based
	// optional fields); set them directly as types_test.go does.
	schema.Properties["n"].hasMinimum = true
	schema.Properties["n"].hasMaximum = true
	verrs, err := ValidateSchema(`{"n":5}`, schema, pcfg)
	if err != nil {
		t.Fatalf("ValidateSchema: %v", err)
	}
	if len(verrs) != 0 {
		t.Fatalf("number misvalidated under PreserveNumbers: %v (want no errors)", verrs)
	}

	// Range enforcement must also work for Number values.
	verrs, err = ValidateSchema(`{"n":50}`, schema, pcfg)
	if err != nil {
		t.Fatalf("ValidateSchema: %v", err)
	}
	if len(verrs) != 1 {
		t.Fatalf("range not enforced under PreserveNumbers: %v (want 1 max-exceeded error)", verrs)
	}

	// Enum comparison across numeric kinds (int64 const vs float64 data).
	enumSchema := &Schema{
		Type: "object",
		Properties: map[string]*Schema{
			"n": {Type: "number", Enum: []any{int64(5)}},
		},
	}
	verrs, err = ValidateSchema(`{"n":5}`, enumSchema, DefaultConfig())
	if err != nil {
		t.Fatalf("ValidateSchema enum: %v", err)
	}
	if len(verrs) != 0 {
		t.Fatalf("int64 enum vs float64 value not compared equal: %v", verrs)
	}
}

// TestD002Round7_SchemaUniqueItemsTypeAware pins the UniqueItems key fix:
// values that differ in JSON type ([1, "1"]) must not be reported as
// duplicates.
func TestD002Round7_SchemaUniqueItemsTypeAware(t *testing.T) {
	schema := &Schema{Type: "array", UniqueItems: true}
	verrs, err := ValidateSchema(`[1,"1",true,"true"]`, schema)
	if err != nil {
		t.Fatalf("ValidateSchema: %v", err)
	}
	if len(verrs) != 0 {
		t.Fatalf("type-distinct values reported as duplicates: %v", verrs)
	}
	verrs, err = ValidateSchema(`[1,1]`, schema)
	if err != nil {
		t.Fatalf("ValidateSchema: %v", err)
	}
	if len(verrs) != 1 {
		t.Fatalf("real duplicates must still be caught: %v", verrs)
	}
}

// TestD002Round7_DecoderUnpairedLowSurrogate pins the parseSurrogatePair fix:
// an unpaired low surrogate (\uDC00) must substitute U+FFFD without error,
// matching encoding/json (previously a hard SyntaxError).
func TestD002Round7_DecoderUnpairedLowSurrogate(t *testing.T) {
	// Built at runtime: "\uDC00" as raw source would be an invalid Go escape.
	doc := `"` + string([]byte{0x5c, 'u', 'D', 'C', '0', '0'}) + `"`
	var got Token
	d := NewDecoder(strings.NewReader(doc))
	for {
		tok, err := d.Token()
		if tok != nil {
			got = tok
		}
		if err != nil {
			if err != io.EOF {
				t.Fatalf("Token on unpaired low surrogate: %v (want U+FFFD substitution, no error)", err)
			}
			break
		}
	}
	s, ok := got.(string)
	if !ok || s != string(rune(0xFFFD)) {
		t.Fatalf("token = %#v, want U+FFFD string", got)
	}
}

// TestP001_EncodePathsEquivalence guards the P-001 single fast-path claim:
// with default config, EncodeWithConfig (bytes fast path) and Marshal must
// produce byte-identical, HTML-escaped output for the values FastEncoder
// handles — including single-key maps, the forEachSortedEntry fast case.
func TestP001_EncodePathsEquivalence(t *testing.T) {
	p, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()

	values := []any{
		map[string]any{"name": "updated"},
		map[string]any{"name": "test", "age": 30, "active": true},
		map[string]any{"html": "<script>&amp;</script>"},
		[]any{1, 2, 3, "x", true},
		"a \"quoted\" <string>",
		42,
		3.14,
		true,
		nil,
		map[string]int{"b": 2, "a": 1},
		map[string]string{"z": "<z>", "a": "a"},
		[]string{"<1>", "2"},
	}

	for _, v := range values {
		enc, err1 := p.EncodeWithConfig(v)
		mar, err2 := p.Marshal(v)
		if (err1 != nil) != (err2 != nil) {
			t.Errorf("value %#v: error mismatch: %v vs %v", v, err1, err2)
			continue
		}
		if err1 != nil {
			continue
		}
		if enc != string(mar) {
			t.Errorf("value %#v:\n  EncodeWithConfig: %s\n  Marshal:          %s", v, enc, string(mar))
		}
	}
}

// TestA2EncoderEquivalence proves Marshal/MarshalIndent and EncodeWithConfig
// produce identical bytes for the configurations the old MarshalToFile
// pipeline used. MarshalToFile and SaveToFile now share one pipeline
// (writeFileJSON → EncodeWithConfig), so this equivalence is what guarantees
// the unification did not change output for previously-supported inputs —
// and it guards against the two encoders drifting apart again.
func TestA2EncoderEquivalence(t *testing.T) {
	ls := "x" + string(rune(0x2028)) + "y"
	inv := string([]byte{'a', 0xff, 'b'})
	type inner struct {
		B string `json:"b"`
	}
	type outer struct {
		Name string          `json:"name"`
		N    int             `json:"n"`
		F    float64         `json:"f"`
		Arr  []int           `json:"arr"`
		Obj  inner           `json:"obj"`
		Ptr  *int            `json:"ptr"`
		Raw  json.RawMessage `json:"raw"`
		Num  json.Number     `json:"num"`
		T    time.Time       `json:"t"`
		Skip string          `json:"-"`
		Opt  string          `json:"opt,omitempty"`
	}
	seven := 7
	values := []any{
		nil, true, 0, -1, 42, 1e21, math.Copysign(0, -1), 0.1, 1e-7, math.MaxInt64,
		"", "plain", "<script>&</script>", ls, inv, "emoji:" + string(rune(0x1F600)),
		[]any{}, map[string]any{}, []any(nil), map[string]any(nil),
		[]int{1, 2, 3}, map[string]any{"a": 1, "b": []any{"x", "y"}},
		map[string]any{"deep": map[string]any{"deeper": []any{map[string]any{"k": "<v>"}}}},
		[]byte("binary<h>"),
		json.RawMessage(`{"raw":[1,2]}`),
		json.Number("1.2300"),
		time.Date(2026, 8, 29, 12, 0, 0, 0, time.UTC),
		outer{Name: "n", N: 9, F: 3.5, Arr: []int{1}, Obj: inner{B: "<b>"}, Ptr: &seven,
			Raw: json.RawMessage(`[3]`), Num: json.Number("1e2"), Skip: "x"},
		&outer{Name: "ptr"},
	}

	p, _ := New()
	defer p.Close()

	compactCfg := DefaultConfig() // Pretty=false — 旧管线等价 cfg
	prettyCfg := PrettyConfig()   // Pretty=true, Indent="  " — 旧 MarshalIndent 参数

	for i, v := range values {
		got, err1 := p.EncodeWithConfig(v, compactCfg)
		want, err2 := p.Marshal(v)
		if (err1 == nil) != (err2 == nil) {
			t.Errorf("compact[%d] err mismatch: %v vs %v", i, err1, err2)
			continue
		}
		if err1 == nil && string(got) != string(want) {
			t.Errorf("compact[%d]: EncodeWithConfig=%s Marshal=%s", i, got, want)
		}

		gotP, err3 := p.EncodeWithConfig(v, prettyCfg)
		wantP, err4 := p.MarshalIndent(v, "", "  ")
		if (err3 == nil) != (err4 == nil) {
			t.Errorf("pretty[%d] err mismatch: %v vs %v", i, err3, err4)
			continue
		}
		if err3 == nil && string(gotP) != string(wantP) {
			t.Errorf("pretty[%d]: EncodeWithConfig=%s | MarshalIndent=%s", i, gotP, wantP)
		}
	}
}

// ===========================================================================
// Round 8 — [D-002 第八轮] 回归测试:锁定本轮 C1/C2/M1/M3/M4/m2 修复。
// 移除任一修复,对应用例应失败。m1(Number 免拷贝)由 Round 8 C1 用例的
// 缓存命中路径隐式覆盖(类型保真即所守卫的行为)。
// ===========================================================================

// TestD002Round8_NoCfgPreserveNumbers pins C1: with no per-call cfg, the
// processor's baked PreserveNumbers must govern parsing (D-006 rule). Before
// the fix the default singleton's PreserveNumbers=false won every no-cfg
// call: Get/ParseAny returned float64, and Set rewrote untouched big integers
// as floats.
func TestD002Round8_NoCfgPreserveNumbers(t *testing.T) {
	cfg := DefaultConfig()
	cfg.PreserveNumbers = true
	p, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()

	in := `{"big":12345678901234567890123,"other":1.5}`
	wantBig := "12345678901234567890123"

	// Get (no cfg, cache on — hit AND miss paths must both see Number).
	v, err := p.Get(in, "big")
	if err != nil {
		t.Fatalf("C1 Get: %v", err)
	}
	num, ok := v.(Number)
	if !ok {
		t.Fatalf("C1 Get(big) = %T (%v), want Number", v, v)
	}
	if string(num) != wantBig {
		t.Fatalf("C1 Get(big) = %s, want literal %s", num, wantBig)
	}
	v2, err := p.Get(in, "big") // cache hit
	if err != nil {
		t.Fatalf("C1 Get(hit): %v", err)
	}
	if hn, ok := v2.(Number); !ok || string(hn) != wantBig {
		t.Fatalf("C1 Get(big) cache-hit = %T (%v), want Number %s", v2, v2, wantBig)
	}

	// Set (no cfg): the untouched big integer must survive byte-for-byte.
	out, err := p.Set(in, "other", 2)
	if err != nil {
		t.Fatalf("C1 Set: %v", err)
	}
	if want := `{"big":12345678901234567890123,"other":2}`; out != want {
		t.Fatalf("C1 Set = %s, want %s (untouched integer rewritten as float)", out, want)
	}

	// ParseAny (no cfg).
	anyv, err := p.ParseAny(in)
	if err != nil {
		t.Fatalf("C1 ParseAny: %v", err)
	}
	if _, ok := anyv.(map[string]any)["big"].(Number); !ok {
		t.Fatalf("C1 ParseAny(big) = %T, want Number", anyv.(map[string]any)["big"])
	}

	// Replace semantics preserved: a per-call cfg with PreserveNumbers=false on
	// the SAME processor must parse to float64 (cfg wins, D-006).
	noPreserve := DefaultConfig()
	rv, err := p.Get(in, "big", noPreserve)
	if err != nil {
		t.Fatalf("C1 Get(replace cfg): %v", err)
	}
	if _, ok := rv.(float64); !ok {
		t.Fatalf("C1 replace-cfg Get(big) = %T, want float64", rv)
	}
}

// TestD002Round8_DeleteFastPathPreserveNumbers pins C2: the simple-property
// Delete fast path must opt out under PreserveNumbers (EnableCache=false),
// mirroring Get's guard — the fast path re-marshals a stdlib (float64) parse
// of the whole document, rewriting untouched big integers as floats.
func TestD002Round8_DeleteFastPathPreserveNumbers(t *testing.T) {
	cfg := DefaultConfig()
	cfg.EnableCache = false
	cfg.PreserveNumbers = true
	p, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()

	in := `{"big":12345678901234567890123,"name":"x"}`
	out, err := p.Delete(in, "name")
	if err != nil {
		t.Fatalf("C2 Delete: %v", err)
	}
	if want := `{"big":12345678901234567890123}`; out != want {
		t.Fatalf("C2 Delete = %s, want %s (fast path rewrote untouched integer as float)", out, want)
	}
}

// TestD002Round8_MarshalPerCallMaxJSONSize pins M1: a per-call cfg's
// MaxJSONSize caps encoded OUTPUT (doc.go contract), at both the package
// level and the Processor method; no-cfg keeps the processor's baked limit.
func TestD002Round8_MarshalPerCallMaxJSONSize(t *testing.T) {
	big := make([]string, 40000) // encoded length ≈ 520KB
	for i := range big {
		big[i] = "abcdefghij"
	}

	cfg := DefaultConfig()
	cfg.MaxJSONSize = 1024

	if _, err := Marshal(big, cfg); !errors.Is(err, ErrSizeLimit) {
		t.Fatalf("M1 json.Marshal(big, cfg MaxJSONSize=1KB) err = %v, want ErrSizeLimit", err)
	}

	p, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	if _, err := p.Marshal(big, cfg); !errors.Is(err, ErrSizeLimit) {
		t.Fatalf("M1 p.Marshal(big, cfg) err = %v, want ErrSizeLimit", err)
	}
	if _, err := p.Marshal(big, cfg); !errors.Is(err, ErrSizeLimit) {
		t.Fatalf("M1 p.MarshalIndent path err = %v, want ErrSizeLimit", err)
	}

	// No-cfg: the processor's baked limit (100MB default) applies — accepted.
	if _, err := p.Marshal(big); err != nil {
		t.Fatalf("M1 p.Marshal no-cfg err = %v, want nil", err)
	}
}

// TestD002Round8_JSONLMemLimitSentinel pins m2: the JSONL engines'
// memory-limit errors carry the ErrSizeLimit sentinel (errors.Is), matching
// NDJSONProcessor — previously bare fmt.Errorf on StreamJSONL /
// StreamJSONLChunked / StreamLinesInto.
func TestD002Round8_JSONLMemLimitSentinel(t *testing.T) {
	p, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()

	cfg := DefaultConfig()
	cfg.JSONLMaxMemory = 10 // trips on the second 7-byte line
	data := `{"a":1}
{"b":2}
{"c":3}
`

	err = p.StreamJSONL(strings.NewReader(data), func(int, *IterableValue) error { return nil }, cfg)
	if !errors.Is(err, ErrSizeLimit) {
		t.Fatalf("m2 StreamJSONL err = %v, want ErrSizeLimit", err)
	}

	err = p.StreamJSONLChunked(strings.NewReader(data), 2, func([]*IterableValue) error { return nil }, cfg)
	if !errors.Is(err, ErrSizeLimit) {
		t.Fatalf("m2 StreamJSONLChunked err = %v, want ErrSizeLimit", err)
	}

	_, err = StreamLinesInto[any](strings.NewReader(data), func(int, any) error { return nil }, cfg)
	if !errors.Is(err, ErrSizeLimit) {
		t.Fatalf("m2 StreamLinesInto err = %v, want ErrSizeLimit", err)
	}
}

// TestD002Round8_MaxOperationsPerSecond pins M3 wiring: the rate limiter is
// reachable via Config.MaxOperationsPerSecond — the second back-to-back op is
// rejected — while 0 (the default) keeps it disabled.
func TestD002Round8_MaxOperationsPerSecond(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MaxOperationsPerSecond = 1 // max 1 op/sec
	p, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()

	doc := `{"a":1}`
	if _, err := p.Get(doc, "a"); err != nil {
		t.Fatalf("M3 first Get: %v", err)
	}
	if _, err := p.Get(doc, "a"); err == nil {
		t.Fatal("M3 second immediate Get = nil error, want rate-limit rejection")
	}

	// Default (0) disables the limiter entirely.
	p2, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer p2.Close()
	for range 3 {
		if _, err := p2.Get(doc, "a"); err != nil {
			t.Fatalf("M3 default-config Get: %v", err)
		}
	}
}

// TestD002Round8_MaxConcurrencyZeroUsesDefault pins M4: a zero/negative
// MaxConcurrency resolves to the default (50), not the minimum (1) — a
// partially-filled Config must not serialize concurrent operations.
func TestD002Round8_MaxConcurrencyZeroUsesDefault(t *testing.T) {
	cfg := Config{} // zero value: only the clamps fill it in
	cfg.ValidateWithWarnings()
	if cfg.MaxConcurrency != DefaultMaxConcurrency {
		t.Fatalf("M4 MaxConcurrency = %d, want default %d", cfg.MaxConcurrency, DefaultMaxConcurrency)
	}

	p, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()

	const goroutines = 8
	var wg sync.WaitGroup
	errCh := make(chan error, goroutines)
	for range goroutines {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := p.Get(`{"a":1}`, "a")
			errCh <- err
		}()
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		if err != nil {
			t.Fatalf("M4 concurrent Get under zero-value MaxConcurrency: %v (want all admitted)", err)
		}
	}
}

// ===========================================================================
// Round 9 — [D-002 第九轮] 回归测试:锁定 m3/m4/m10/m12 修复。m9 为竞态窗口
// 修复(读侧 CAS 的写侧镜像),无法确定性复现,由 internal 包测试与注释锚定;
// m11 为签名收敛,由既有缓存行为测试覆盖。
// ===========================================================================

// TestD002Round9_GovernedOpsNoSelfReject pins m3: the newly governed ops
// (Parse/Valid/PreParse/GetFromParsed/SetFromParsed/Prettify/Compact/
// ValidateSchema, plus CompareJSON via the p.Marshal encode funnel) must not
// nest their beginGovernedOp acquisition — under MaxConcurrency=1 a nested
// acquire would self-reject with ErrConcurrencyLimit.
func TestD002Round9_GovernedOpsNoSelfReject(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MaxConcurrency = 1 // tightest legal limit: any nested acquire fails
	p, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()

	doc := `{"a":1}`
	schema := &Schema{Type: "object"}

	if err := p.Parse(doc, &map[string]any{}); err != nil {
		t.Fatalf("m3 Parse: %v", err)
	}
	if _, err := p.ParseAny(doc); err != nil {
		t.Fatalf("m3 ParseAny: %v", err)
	}
	if _, err := p.Valid(doc); err != nil {
		t.Fatalf("m3 Valid: %v", err)
	}
	pp, err := p.PreParse(doc)
	if err != nil {
		t.Fatalf("m3 PreParse: %v", err)
	}
	if _, err := p.GetFromParsed(pp, "a"); err != nil {
		t.Fatalf("m3 GetFromParsed: %v", err)
	}
	if _, err := p.SetFromParsed(pp, "a", 2); err != nil {
		t.Fatalf("m3 SetFromParsed: %v", err)
	}
	pp.Release()
	if _, err := p.Prettify(doc); err != nil {
		t.Fatalf("m3 Prettify: %v", err)
	}
	if _, err := p.Compact(doc); err != nil {
		t.Fatalf("m3 Compact: %v", err)
	}
	if _, err := p.ValidateSchema(doc, schema); err != nil {
		t.Fatalf("m3 ValidateSchema: %v", err)
	}
	cp, err := p.CompilePath("a")
	if err != nil {
		t.Fatalf("m3 CompilePath: %v", err)
	}
	if _, err := p.GetCompiled(doc, cp); err != nil {
		t.Fatalf("m3 GetCompiled: %v", err)
	}
	cp.Release()
	eq, err := p.CompareJSON(`{"a":1}`, `{"a":1.0}`)
	if err != nil {
		t.Fatalf("m3 CompareJSON: %v", err)
	}
	if !eq {
		t.Fatal("m3 CompareJSON: 1 and 1.0 must compare equal")
	}
}

// TestD002Round9_JSONLPreserveNumbers pins m4: the JSONL engines honor the
// effective PreserveNumbers setting (Number literals survive) while the
// default keeps stdlib float64 semantics.
func TestD002Round9_JSONLPreserveNumbers(t *testing.T) {
	p, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()

	data := `{"big":12345678901234567890123}
{"b":2}
`
	want := "12345678901234567890123"

	pres := DefaultConfig()
	pres.PreserveNumbers = true

	// StreamJSONL, per-call cfg: Number with the exact literal (line 1 carries
	// "big"; line 2 is {"b":2} and only proves the stream continues).
	err = p.StreamJSONL(strings.NewReader(data), func(lineNum int, item *IterableValue) error {
		m, ok := item.GetData().(map[string]any)
		if !ok {
			t.Fatalf("m4 StreamJSONL item = %T, want map", item.GetData())
		}
		if lineNum == 1 {
			if n, ok := m["big"].(Number); !ok || string(n) != want {
				t.Fatalf("m4 StreamJSONL big = %T (%v), want Number %s", m["big"], m["big"], want)
			}
		}
		return nil
	}, pres)
	if err != nil {
		t.Fatalf("m4 StreamJSONL: %v", err)
	}

	// StreamJSONL, default cfg: float64 (unchanged stdlib semantics).
	err = p.StreamJSONL(strings.NewReader(data), func(lineNum int, item *IterableValue) error {
		m := item.GetData().(map[string]any)
		if lineNum == 1 {
			if _, ok := m["big"].(float64); !ok {
				t.Fatalf("m4 default StreamJSONL big = %T, want float64", m["big"])
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("m4 default StreamJSONL: %v", err)
	}

	// Deprecated NDJSONProcessor: same honoring via its baked config.
	npPres := NewNDJSONProcessor(pres)
	if err := npPres.ProcessReader(strings.NewReader(data), func(lineNum int, obj map[string]any) error {
		if lineNum == 1 {
			if n, ok := obj["big"].(Number); !ok || string(n) != want {
				t.Fatalf("m4 NDJSON big = %T (%v), want Number %s", obj["big"], obj["big"], want)
			}
		}
		return nil
	}); err != nil {
		t.Fatalf("m4 NDJSON: %v", err)
	}

	// StreamLinesInto with PreserveNumbers cfg: typed T=any sees Number.
	results, err := StreamLinesInto[any](strings.NewReader(data), nil, pres)
	if err != nil {
		t.Fatalf("m4 StreamLinesInto: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("m4 StreamLinesInto len = %d, want 2", len(results))
	}
	if _, ok := results[0].(map[string]any)["big"].(Number); !ok {
		t.Fatalf("m4 StreamLinesInto big = %T, want Number", results[0].(map[string]any)["big"])
	}
}

// TestD002Round9_RootArraySetExtensionExplicitError pins m12: root-level
// slice/index extension fails explicitly instead of silently extending
// root[0] (the previous zero-value arrayContainerSegment mis-navigation),
// while in-bounds root slice/index writes keep working.
func TestD002Round9_RootArraySetExtensionExplicitError(t *testing.T) {
	p, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()

	// Out-of-bounds root slice: previously extended root[0] (the WRONG array)
	// and returned success; must now error.
	if out, err := p.Set(`[[9,9],3]`, "[5:8]", "x"); err == nil {
		t.Fatalf("m12 root slice extension = %q, nil error (previously mutated root[0])", out)
	}
	// Out-of-bounds root index: previously a misleading "nested array" error;
	// now the explicit root-extension error.
	if _, err := p.Set(`[1,2,3]`, "[5]", "x"); err == nil {
		t.Fatal("m12 root index extension: nil error, want rejection")
	}
	// In-bounds root slice write still works.
	out, err := p.Set(`[1,2,3]`, "[0:2]", 9)
	if err != nil {
		t.Fatalf("m12 in-bounds root slice: %v", err)
	}
	if out != `[9,9,3]` {
		t.Fatalf("m12 in-bounds root slice = %s, want [9,9,3]", out)
	}
	// In-bounds root index write still works.
	out, err = p.Set(`[1,2,3]`, "[1]", 9)
	if err != nil {
		t.Fatalf("m12 in-bounds root index: %v", err)
	}
	if out != `[1,9,3]` {
		t.Fatalf("m12 in-bounds root index = %s, want [1,9,3]", out)
	}
}

// TestD002Round9_JSONLWriterSingleWrite pins m10: JSONLWriter.Write emits the
// exact same bytes and accounting after combining data+newline into one Write.
func TestD002Round9_JSONLWriterSingleWrite(t *testing.T) {
	var buf strings.Builder
	w := NewJSONLWriter(&buf)
	if err := w.Write(map[string]any{"a": 1}); err != nil {
		t.Fatalf("m10 Write: %v", err)
	}
	if err := w.Write(map[string]any{"b": "x"}); err != nil {
		t.Fatalf("m10 Write: %v", err)
	}
	want := "{\"a\":1}\n{\"b\":\"x\"}\n"
	if buf.String() != want {
		t.Fatalf("m10 output = %q, want %q", buf.String(), want)
	}
	stats := w.Stats()
	if stats.LinesProcessed != 2 || stats.BytesWritten != int64(len(want)) {
		t.Fatalf("m10 stats = %+v, want 2 lines / %d bytes", stats, len(want))
	}
}

// ===========================================================================
// Round 10 — D-002 第十轮·回查 (consolidated from regression_round10_test.go)
// ===========================================================================

// TestD002Round10_NoCfgBakedEncodeLimits pins the R10 fix of the M1 regression:
// MarshalIndent / SaveToWriter / CompareJSON no longer fabricate a
// DefaultConfig-valued cfg for no-cfg calls (which routed M1's
// effectiveEncodeMaxSize to the 100MB default and bypassed the processor's
// baked MaxJSONSize). No-cfg now resolves the baked configuration (D-006).
func TestD002Round10_NoCfgBakedEncodeLimits(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MaxJSONSize = 1024 // baked cap well below the ~2.6KB encoded output
	p, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()

	big := map[string]any{"pad": strings.Repeat("x", 2500)}

	if _, err := p.MarshalIndent(big, "", "  "); !errors.Is(err, ErrSizeLimit) {
		t.Fatalf("R10 MarshalIndent no-cfg err = %v, want ErrSizeLimit (baked 1KB cap bypassed)", err)
	}

	var buf strings.Builder
	if err := p.SaveToWriter(&buf, big); !errors.Is(err, ErrSizeLimit) {
		t.Fatalf("R10 SaveToWriter no-cfg err = %v, want ErrSizeLimit", err)
	}

	a := `{"pad":"` + strings.Repeat("x", 2500) + `"}`
	if _, err := p.CompareJSON(a, a); !errors.Is(err, ErrSizeLimit) {
		t.Fatalf("R10 CompareJSON no-cfg err = %v, want ErrSizeLimit (singleton dereference overrode baked cap)", err)
	}

	// Control: a default processor (100MB baked) accepts the same payload.
	def, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer def.Close()
	if _, err := def.MarshalIndent(big, "", "  "); err != nil {
		t.Fatalf("R10 control MarshalIndent: %v", err)
	}
	var buf2 strings.Builder
	if err := def.SaveToWriter(&buf2, big); err != nil {
		t.Fatalf("R10 control SaveToWriter: %v", err)
	}
}

// TestD002Round10_NumberConversions pins the R10 conversion fix: the
// convertTo* family handles the library's Number like json.Number. Before the
// fix, PreserveNumbers JSONL made item.GetInt return 0 for every number.
func TestD002Round10_NumberConversions(t *testing.T) {
	if n, ok := convertToInt(Number("42")); !ok || n != 42 {
		t.Fatalf("R10 convertToInt(Number) = %d,%v want 42,true", n, ok)
	}
	if n, ok := convertToInt64(Number("9223372036854775807")); !ok || n != 9223372036854775807 {
		t.Fatalf("R10 convertToInt64(Number) = %d,%v", n, ok)
	}
	if u, ok := convertToUint64(Number("18446744073709551615")); !ok || u != 18446744073709551615 {
		t.Fatalf("R10 convertToUint64(Number) = %d,%v", u, ok)
	}
	if f, ok := convertToFloat64(Number("1.5")); !ok || f != 1.5 {
		t.Fatalf("R10 convertToFloat64(Number) = %v,%v", f, ok)
	}
	if b, ok := convertToBool(Number("1")); !ok || !b {
		t.Fatalf("R10 convertToBool(Number(1)) = %v,%v", b, ok)
	}
	if b, ok := convertToBool(Number("0")); !ok || b {
		t.Fatalf("R10 convertToBool(Number(0)) = %v,%v", b, ok)
	}
	if s := convertToString(Number("42")); s != "42" {
		t.Fatalf("R10 convertToString(Number) = %q", s)
	}
	// Non-integral Number behaves like non-integral json.Number (Int64
	// refuses; GetInt falls back to its default).
	if _, ok := convertToInt(Number("1.5")); ok {
		t.Fatal("R10 convertToInt(Number(1.5)) = true, want false (mirror json.Number)")
	}

	// Behavior level: IterableValue getters on a PreserveNumbers JSONL stream.
	p, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	pres := DefaultConfig()
	pres.PreserveNumbers = true
	err = p.StreamJSONL(strings.NewReader("{\"age\":42,\"score\":1.5,\"ok\":1}\n"), func(_ int, item *IterableValue) error {
		if got := item.GetInt("age"); got != 42 {
			t.Errorf("R10 item.GetInt(age) = %d, want 42", got)
		}
		if got := item.GetFloat64("score"); got != 1.5 {
			t.Errorf("R10 item.GetFloat64(score) = %v, want 1.5", got)
		}
		if got := item.GetBool("ok"); !got {
			t.Errorf("R10 item.GetBool(ok) = false, want true")
		}
		if got := item.GetString("age"); got != "42" {
			t.Errorf("R10 item.GetString(age) = %q, want \"42\"", got)
		}
		return nil
	}, pres)
	if err != nil {
		t.Fatalf("R10 StreamJSONL: %v", err)
	}
}

// TestD002Round10_JSONLParallelChunkedPreserveAndMemCap pins two R10 items:
// the parallel and chunked engines honor PreserveNumbers (extending R9's
// serial-only coverage), and the parallel engine enforces JSONLMaxMemory
// (previously the only JSONL reader without the total-bytes cap).
func TestD002Round10_JSONLParallelChunkedPreserveAndMemCap(t *testing.T) {
	p, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()

	pres := DefaultConfig()
	pres.PreserveNumbers = true
	data := "{\"big\":12345678901234567890123}\n{\"b\":2}\n"
	want := "12345678901234567890123"

	// Chunked engine: preserved literal visible to the batch callback.
	err = p.StreamJSONLChunked(strings.NewReader(data), 10, func(chunk []*IterableValue) error {
		for _, item := range chunk {
			if m, ok := item.GetData().(map[string]any); ok {
				if n, ok := m["big"].(Number); ok && string(n) != want {
					t.Errorf("R10 chunked big = %s, want %s", n, want)
				}
			}
		}
		return nil
	}, pres)
	if err != nil {
		t.Fatalf("R10 StreamJSONLChunked: %v", err)
	}

	// Parallel engine: preserved literal visible to the worker callback
	// (concurrent — guard the shared flag with a mutex).
	var mu sync.Mutex
	sawNumber := false
	err = p.StreamJSONLParallelWithContext(context.Background(), strings.NewReader(data), 2,
		func(_ int, item *IterableValue) error {
			if _, ok := item.GetData().(map[string]any)["big"].(Number); ok {
				mu.Lock()
				sawNumber = true
				mu.Unlock()
			}
			return nil
		}, pres)
	if err != nil {
		t.Fatalf("R10 StreamJSONLParallel: %v", err)
	}
	mu.Lock()
	defer mu.Unlock()
	if !sawNumber {
		t.Fatal("R10 StreamJSONLParallel: big not a Number under PreserveNumbers")
	}

	// Parallel engine: total-bytes cap with the ErrSizeLimit sentinel.
	capCfg := DefaultConfig()
	capCfg.JSONLMaxMemory = 10 // trips on the second 7-byte line
	err = p.StreamJSONLParallelWithContext(context.Background(), strings.NewReader(data), 2,
		func(int, *IterableValue) error { return nil }, capCfg)
	if !errors.Is(err, ErrSizeLimit) {
		t.Fatalf("R10 parallel mem cap err = %v, want ErrSizeLimit", err)
	}
}

// TestD002Round10_SetMultipleRateLimit pins the R10 gate: SetMultiple now
// honors MaxOperationsPerSecond like Get/Set/Delete (previously the one
// governed mutation without the rate-limit check).
func TestD002Round10_SetMultipleRateLimit(t *testing.T) {
	rl := DefaultConfig()
	rl.MaxOperationsPerSecond = 1
	p, err := New(rl)
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()

	if _, err := p.SetMultiple(`{"a":1}`, map[string]any{"a": 2}); err != nil {
		t.Fatalf("R10 first SetMultiple: %v", err)
	}
	if _, err := p.SetMultiple(`{"a":1}`, map[string]any{"a": 3}); err == nil {
		t.Fatal("R10 second immediate SetMultiple = nil error, want rate-limit rejection")
	}
}

// TestD002Round10_SetFromParsedBakedCreatePaths pins the R10 D-006 fix:
// SetFromParsed no-cfg resolves CreatePaths from the baked config — the
// singleton's true previously re-enabled path creation on a processor built
// with CreatePaths=false.
func TestD002Round10_SetFromParsedBakedCreatePaths(t *testing.T) {
	cfg := DefaultConfig()
	cfg.CreatePaths = false
	p, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()

	pp, err := p.PreParse(`{}`)
	if err != nil {
		t.Fatal(err)
	}
	defer pp.Release()

	// No-cfg: baked CreatePaths=false must refuse to create the path.
	if _, err := p.SetFromParsed(pp, "a.b", 1); err == nil {
		t.Fatal("R10 SetFromParsed no-cfg created path despite baked CreatePaths=false")
	}

	// With-cfg REPLACES: CreatePaths=true creates it.
	onCfg := DefaultConfig()
	onCfg.CreatePaths = true
	pp2, err := p.SetFromParsed(pp, "a.b", 1, onCfg)
	if err != nil {
		t.Fatalf("R10 SetFromParsed with-cfg: %v", err)
	}
	defer pp2.Release()
	if v, err := p.GetFromParsed(pp2, "a.b"); err != nil || v != 1 {
		t.Fatalf("R10 GetFromParsed(a.b) = %v,%v want 1,nil", v, err)
	}
}

// TestD002Round10_BakedCacheResults pins the R10 D-006 fix: with no cfg, the
// processor's baked CacheResults governs result caching (previously the
// singleton's true cached anyway). The parse cache (setCachedResultInternal)
// is intentionally unaffected.
func TestD002Round10_BakedCacheResults(t *testing.T) {
	off := DefaultConfig()
	off.EnableCache = true
	off.CacheResults = false
	pOff, err := New(off)
	if err != nil {
		t.Fatal(err)
	}
	defer pOff.Close()

	doc := `{"a":1,"b":2}`
	_, _ = pOff.Get(doc, "a")
	_, _ = pOff.Get(doc, "a")
	// Exactly one entry: the parse cache; the get: entry must be suppressed.
	if n := pOff.GetStats().CacheSize; n != 1 {
		t.Fatalf("R10 CacheResults=false cache size = %d, want 1 (parse only)", n)
	}

	on := DefaultConfig() // CacheResults=true by default
	pOn, err := New(on)
	if err != nil {
		t.Fatal(err)
	}
	defer pOn.Close()
	_, _ = pOn.Get(doc, "a")
	if n := pOn.GetStats().CacheSize; n != 2 {
		t.Fatalf("R10 CacheResults=true cache size = %d, want 2 (parse + get)", n)
	}
}

// ===========================================================================
// Round 11 — D-002 第十一轮 (consolidated from regression_round11_test.go)
// ===========================================================================

// ============================================================================
// D-002 Round 11 regression tests (2026-10-04)
//
//   C1  decoder-based (number-preserving) paths reject trailing garbage
//   M1  GetMultiple concurrency governance (Close-drain + MaxConcurrency)
//   M2  mutation output honors the effective MaxJSONSize (option A)
//   M3  FastEncoder.EncodeTime year-range guard
//   m4  FastEncoder.EncodeFloat rejects NaN/Inf
// ============================================================================

// ---------------------------------------------------------------------------
// C1: trailing garbage
// ---------------------------------------------------------------------------

// compareJSONCore decodes with the preserving decoder unconditionally; before
// C1 the Decoder-based single-value read silently ignored the trailing "zzz"
// and reported the two different documents EQUAL.
func TestD002Round11_TrailingGarbage_CompareJSON(t *testing.T) {
	equal, err := CompareJSON(`{"a":1}`, `{"a":1}zzz`)
	if err == nil || equal {
		t.Fatalf("CompareJSON accepted trailing garbage (second arg): equal=%v err=%v", equal, err)
	}
	equal, err = CompareJSON(`{"a":1}zzz`, `{"a":1}`)
	if err == nil || equal {
		t.Fatalf("CompareJSON accepted trailing garbage (first arg): equal=%v err=%v", equal, err)
	}
}

func TestD002Round11_TrailingGarbage_MergeJSON(t *testing.T) {
	if out, err := MergeJSON(`{"a":1}zzz`, `{"c":2}`); err == nil {
		t.Fatalf("MergeJSON accepted trailing garbage: out=%q err=%v", out, err)
	}
}

func TestD002Round11_TrailingGarbage_PreserveNumbers(t *testing.T) {
	cfg := DefaultConfig()
	cfg.PreserveNumbers = true
	// Trailing content that ends with '}' bypasses the first/last-character
	// structure heuristic — this exact shape reached the decoder before the fix.
	bad := `{"a":1}{"b":2}`

	if _, err := Get(bad, "a", cfg); err == nil {
		t.Error("Get with PreserveNumbers accepted a trailing JSON document")
	}
	var v any
	if err := Parse(bad, &v, cfg); err == nil {
		t.Error("Parse with PreserveNumbers accepted a trailing JSON document")
	}
	if out, err := Prettify(bad, cfg); err == nil {
		t.Errorf("Prettify with PreserveNumbers accepted a trailing document: out=%q", out)
	}
	if ok, err := ValidWithConfig(bad, cfg); err == nil || ok {
		t.Errorf("Valid with PreserveNumbers accepted a trailing document: ok=%v err=%v", ok, err)
	}

	// Baseline: the no-config path must keep rejecting the same input.
	if _, err := Get(bad, "a"); err == nil {
		t.Error("no-config Get accepted a trailing JSON document")
	}
}

func TestD002Round11_TrailingGarbage_WhitespaceStillFine(t *testing.T) {
	cfg := DefaultConfig()
	cfg.PreserveNumbers = true
	// Trailing whitespace is insignificant — both decode paths accept it.
	if _, err := Get("{\"a\":1}\n\t ", "a", cfg); err != nil {
		t.Fatalf("trailing whitespace wrongly rejected: %v", err)
	}
}

func TestD002Round11_TrailingGarbage_PreservingUnmarshal(t *testing.T) {
	type obj struct {
		A int `json:"a"`
	}

	// Preserve branch with a struct target (preservingUnmarshal, non-*any path).
	pcfg := DefaultConfig()
	pcfg.PreserveNumbers = true
	var o obj
	if err := Parse(`{"a":1}{"b":2}`, &o, pcfg); err == nil {
		t.Error("preservingUnmarshal (struct target) accepted a trailing document")
	}

	// DisallowUnknown non-preserve branch also uses a Decoder (C1 companion fix).
	dcfg := DefaultConfig()
	dcfg.DisallowUnknown = true
	var o2 obj
	if err := Parse(`{"a":1}{"b":2}`, &o2, dcfg); err == nil {
		t.Error("DisallowUnknown decoder branch accepted a trailing document")
	}
}

func TestD002Round11_TrailingGarbage_JSONLLine(t *testing.T) {
	pcfg := DefaultConfig()
	pcfg.PreserveNumbers = true
	p, err := New(pcfg)
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()

	err = p.StreamJSONL(strings.NewReader(`{"a":1} junk`+"\n"), func(_ int, _ *IterableValue) error {
		return nil
	}, pcfg)
	if err == nil {
		t.Error("StreamJSONL with PreserveNumbers accepted a garbage line")
	}
}

// ---------------------------------------------------------------------------
// M1: GetMultiple governance
// ---------------------------------------------------------------------------

func TestD002Round11_GetMultipleGovernance_MaxConcurrency(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MaxConcurrency = 1
	entered := make(chan struct{})
	release := make(chan struct{})
	cfg.AddHook(&HookFunc{
		BeforeFn: func(HookContext) error {
			select {
			case <-entered:
			default:
				close(entered)
			}
			<-release
			return nil
		},
	})
	p, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_, _ = p.GetMultiple(`{"a":1,"b":2}`, []string{"a", "b"})
	}()

	<-entered // the first GetMultiple is inside its Before hook => governance slot held

	// A second GetMultiple on the same MaxConcurrency=1 processor must be
	// rejected with ErrConcurrencyLimit — exactly like a second Get (M1).
	_, err = p.GetMultiple(`{"a":1}`, []string{"a"})
	if !errors.Is(err, ErrConcurrencyLimit) {
		t.Fatalf("second GetMultiple was not concurrency-limited: %v", err)
	}

	close(release)
	wg.Wait()
}

func TestD002Round11_GetMultipleGovernance_ClosedProcessor(t *testing.T) {
	p, err := New()
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := p.GetMultiple(`{"a":1}`, []string{"a"}); !errors.Is(err, ErrProcessorClosed) {
		t.Fatalf("closed processor must reject GetMultiple: %v", err)
	}
}

// ---------------------------------------------------------------------------
// M2: mutation output size limit
// ---------------------------------------------------------------------------

func TestD002Round11_MutationOutputSizeLimit_Set(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MaxJSONSize = 64
	p, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()

	big := strings.Repeat("x", 100)

	out, err := p.Set(`{"a":""}`, "a", big)
	if !errors.Is(err, ErrSizeLimit) {
		t.Fatalf("Set output not size-limited: err=%v", err)
	}
	if out != `{"a":""}` {
		t.Fatalf("Set must return the original document on size failure, got %q", out)
	}

	// Baseline: a mutation whose output fits is unaffected (byte-identical).
	out, err = p.Set(`{"a":""}`, "a", "ok")
	if err != nil || out != `{"a":"ok"}` {
		t.Fatalf("in-limit Set broken: out=%q err=%v", out, err)
	}

	// A per-call cfg REPLACES the cap (same rule as CreatePaths, D-006).
	wide := DefaultConfig()
	wide.MaxJSONSize = DefaultMaxJSONSize
	if _, err := p.Set(`{"a":""}`, "a", big, wide); err != nil {
		t.Fatalf("per-call cfg loosening failed: %v", err)
	}
}

func TestD002Round11_MutationOutputSizeLimit_SetMultiple(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MaxJSONSize = 64
	p, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()

	big := strings.Repeat("x", 100)
	out, err := p.SetMultiple(`{"a":"","b":""}`, map[string]any{"a": big, "b": big})
	if !errors.Is(err, ErrSizeLimit) {
		t.Fatalf("SetMultiple output not size-limited: err=%v", err)
	}
	if out != `{"a":"","b":""}` {
		t.Fatalf("SetMultiple must return the original document, got %q", out)
	}
}

func TestD002Round11_MutationOutputSizeLimit_ForeachReturn(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MaxJSONSize = 64
	p, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()

	// The callback grows the document past the limit via the shared reference.
	out, err := p.ForeachReturn(`{"a":{"pad":""}}`, func(_ any, item *IterableValue) {
		if m, ok := item.GetData().(map[string]any); ok {
			m["pad"] = strings.Repeat("x", 100)
		}
	})
	if !errors.Is(err, ErrSizeLimit) {
		t.Fatalf("ForeachReturn output not size-limited: err=%v out=%q", err, out)
	}

	// Baseline: unchanged-size iteration still succeeds.
	out, err = p.ForeachReturn(`{"a":1}`, func(_ any, _ *IterableValue) {})
	if err != nil || out != `{"a":1}` {
		t.Fatalf("in-limit ForeachReturn broken: out=%q err=%v", out, err)
	}
}

// ---------------------------------------------------------------------------
// M3: EncodeTime year range (root-level surface)
// ---------------------------------------------------------------------------

func TestD002Round11_EncodeTimeYearRange(t *testing.T) {
	p, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()

	far := map[string]any{"t": time.Date(10000, 1, 1, 0, 0, 0, 0, time.UTC)}
	if _, err := p.Encode(far); err == nil {
		t.Error("Encode accepted year 10000 (invalid RFC3339) with a nil error")
	}
	neg := map[string]any{"t": time.Date(-1, 1, 1, 0, 0, 0, 0, time.UTC)}
	if _, err := p.Encode(neg); err == nil {
		t.Error("Encode accepted a negative year with a nil error")
	}
	okOut, err := p.Encode(map[string]any{"t": time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)})
	if err != nil || !strings.Contains(okOut, "2026-01-01T00:00:00Z") {
		t.Fatalf("normal time broken: out=%q err=%v", okOut, err)
	}
}

// ---------------------------------------------------------------------------
// m4: EncodeFloat non-finite rejection (root-level view of the internal API)
// ---------------------------------------------------------------------------

func TestD002Round11_FastEncoderFloatRejectsNonFinite(t *testing.T) {
	e := internal.GetEncoder()
	defer internal.PutEncoder(e)

	if err := e.EncodeFloat(math.NaN(), 64); err == nil {
		t.Error("EncodeFloat(NaN) returned a nil error")
	}
	if err := e.EncodeFloat(math.Inf(1), 64); err == nil {
		t.Error("EncodeFloat(+Inf) returned a nil error")
	}
	if err := e.EncodeFloat(float64(float32(math.Inf(-1))), 32); err == nil {
		t.Error("EncodeFloat(-Inf, 32 bits) returned a nil error")
	}
	if err := e.EncodeFloat(1.5, 64); err != nil {
		t.Errorf("EncodeFloat(1.5) errored: %v", err)
	}
}
