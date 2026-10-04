package json

import (
	"encoding/json"
	stdjson "encoding/json"
	"io"
	"math"
	"strings"
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
//
// Formerly split across regression_test.go, d002_round2_regression_test.go,
// d002_round4_verify_test.go, and d002_round6_regression_test.go; consolidated
// into this single file on 2026-08-30 (test content preserved verbatim).
// asStr/d002fmt live in the shared-helpers section because rounds 1 and 2 both
// use them; round-6 helpers stay inside their section.

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

// M1: cache Get must agree with Set on long (>MaxCacheKeyLength) keys.
func TestD002_Cache_LongKeyRoundTrip(t *testing.T) {
	cm := internal.NewCacheManager(true, 10000, 0)
	longKey := strings.Repeat("k", 2048) // > MaxCacheKeyLength (1024)
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

// TestD002Round7_CacheDeleteLongKey pins the CacheManager.Delete fix: keys
// longer than MaxCacheKeyLength are stored by Set under a truncated key, so
// Delete must apply the same truncation or the entry survives invalidation.
func TestD002Round7_CacheDeleteLongKey(t *testing.T) {
	cm := internal.NewCacheManager(true, 16, 0)
	defer cm.Close()

	longKey := strings.Repeat("k", 2048) // > MaxCacheKeyLength (1024)
	cm.Set(longKey, "v")
	if v, ok := cm.Get(longKey); !ok || v != "v" {
		t.Fatal("setup: long-key Set/Get round trip failed")
	}

	cm.Delete(longKey)
	if v, ok := cm.Get(longKey); ok {
		t.Fatalf("entry survived Delete: value = %v (long-key Delete must hit the truncated stored key)", v)
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
	if _, err := p.Marshal(long, DefaultConfig()); err == nil {
		t.Fatal("Marshal with cfg: expected ErrSizeLimit for oversized output")
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
