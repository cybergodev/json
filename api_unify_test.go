package json

import (
	"bytes"
	"context"
	stdjson "encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"sync"
	"testing"
)

// ============================================================================
// [D-005] Phase 1 — API Unification: package-level cfg passthrough
//
// These tests lock in the Phase 1 change: Marshal / Unmarshal / MarshalIndent /
// Valid / CompareJSON now accept an optional trailing Config, making the
// package-level API a true mirror of the Processor API.
//
// Two invariants are guarded:
//   1. Backward compatibility — calling without cfg behaves exactly as before
//      (drop-in encoding/json compatible).
//   2. Mirror — package Foo(v, cfg) produces the same result as p.Foo(v, cfg).
//
// Notes on test data:
//   - Byte-equality with encoding/json uses structs, not maps. Both libraries
//     encode structs in field-declaration order, so the comparison is stable.
//     Map key order is NOT guaranteed byte-identical (this library defaults
//     SortKeys=false; encoding/json sorts), so maps are only compared by value.
//   - stdjson aliases encoding/json to avoid shadowing the package's own name.
// ============================================================================

// unifyUser is a struct with deterministic field order in both libraries.
type unifyUser struct {
	Name   string `json:"name"`
	Age    int    `json:"age"`
	Active bool   `json:"active"`
}

func TestUnify_Marshal_NoConfig_MatchesStdlib(t *testing.T) {
	v := unifyUser{Name: "Alice", Age: 30, Active: true}
	want, err := stdjson.Marshal(v)
	if err != nil {
		t.Fatalf("stdlib Marshal: %v", err)
	}
	got, err := Marshal(v) // no cfg — must remain drop-in compatible
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("Marshal(v) without cfg drifted from encoding/json:\n got=%s\nwant=%s", got, want)
	}
}

func TestUnify_Marshal_WithConfig_AppliesCfg(t *testing.T) {
	v := unifyUser{Name: "Alice", Age: 30}

	compact, err := Marshal(v)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	pretty, err := Marshal(v, PrettyConfig())
	if err != nil {
		t.Fatalf("Marshal(v, PrettyConfig()): %v", err)
	}
	if bytes.Equal(compact, pretty) {
		t.Errorf("cfg had no effect: compact==pretty (%s)", pretty)
	}
	if !bytes.Contains(pretty, []byte("\n")) {
		t.Errorf("PrettyConfig not honored: %s", pretty)
	}
}

func TestUnify_Marshal_MirrorsProcessor(t *testing.T) {
	cfg := PrettyConfig()
	v := unifyUser{Name: "Alice", Age: 30, Active: true}

	pkgOut, err := Marshal(v, cfg)
	if err != nil {
		t.Fatalf("package Marshal: %v", err)
	}
	p, err := New()
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer p.Close()
	procOut, err := p.Marshal(v, cfg)
	if err != nil {
		t.Fatalf("processor Marshal: %v", err)
	}
	if !bytes.Equal(pkgOut, procOut) {
		t.Errorf("package and processor Marshal diverged:\n pkg=%s\nproc=%s", pkgOut, procOut)
	}
}

func TestUnify_Unmarshal_NoConfig_BehaviorUnchanged(t *testing.T) {
	src := `{"name":"Alice","age":30}`
	var std, ours map[string]any
	if err := stdjson.Unmarshal([]byte(src), &std); err != nil {
		t.Fatalf("stdlib Unmarshal: %v", err)
	}
	if err := Unmarshal([]byte(src), &ours); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if ours["name"] != std["name"] || ours["age"] != std["age"] {
		t.Errorf("Unmarshal without cfg diverged: got=%v want=%v", ours, std)
	}
}

func TestUnify_Unmarshal_WithConfig_Succeeds(t *testing.T) {
	src := `{"name":"Alice"}`
	var got map[string]any
	if err := Unmarshal([]byte(src), &got, DefaultConfig()); err != nil {
		t.Fatalf("Unmarshal with cfg: %v", err)
	}
	if got["name"] != "Alice" {
		t.Errorf("unexpected result: %v", got)
	}
}

func TestUnify_MarshalIndent_NoConfig_MatchesStdlib(t *testing.T) {
	v := unifyUser{Name: "Alice", Age: 30, Active: true}
	want, err := stdjson.MarshalIndent(v, "", "  ")
	if err != nil {
		t.Fatalf("stdlib MarshalIndent: %v", err)
	}
	got, err := MarshalIndent(v, "", "  ")
	if err != nil {
		t.Fatalf("MarshalIndent: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("MarshalIndent without cfg drifted:\n got=%s\nwant=%s", got, want)
	}
}

func TestUnify_MarshalIndent_WithConfig_Succeeds(t *testing.T) {
	v := unifyUser{Name: "Alice"}
	got, err := MarshalIndent(v, "", "    ", DefaultConfig())
	if err != nil {
		t.Fatalf("MarshalIndent with cfg: %v", err)
	}
	if !bytes.Contains(got, []byte("    ")) {
		t.Errorf("indent not applied: %s", got)
	}
}

func TestUnify_Valid_NoConfig_BehaviorUnchanged(t *testing.T) {
	if !Valid([]byte(`{"a":1}`)) {
		t.Error("Valid should accept valid JSON")
	}
	if Valid([]byte(`{not json`)) {
		t.Error("Valid should reject invalid JSON")
	}
}

// TestUnify_Valid_WithConfig_AcceptsCfg confirms Valid accepts an optional
// Config and collapses validation/parse errors to false (its bool return type
// cannot surface them). Per-call options are honored: Processor.Valid resolves
// options from cfg and validates input via validateInputForOptions, so a
// caller-supplied MaxJSONSize / FullSecurityScan / etc. is enforced.
func TestUnify_Valid_WithConfig_AcceptsCfg(t *testing.T) {
	if !Valid([]byte(`{"a":1}`), DefaultConfig()) {
		t.Error("Valid(valid, cfg) should be true")
	}
	if Valid([]byte(`{not json`), DefaultConfig()) {
		t.Error("Valid(invalid, cfg) should be false")
	}
	if !Valid([]byte(`{"a":1}`), SecurityConfig()) {
		t.Error("Valid(valid, SecurityConfig) should be true")
	}

	// A caller-supplied MaxJSONSize is enforced on the per-call path: an input
	// larger than the configured limit is rejected even though it is valid JSON.
	small := DefaultConfig()
	small.MaxJSONSize = 2
	if Valid([]byte(`{"a":1}`), small) {
		t.Error("Valid should reject input exceeding cfg.MaxJSONSize")
	}
}

func TestUnify_CompareJSON_NoConfig_BehaviorUnchanged(t *testing.T) {
	eq, err := CompareJSON(`{"a":1}`, `{"a":1.0}`)
	if err != nil {
		t.Fatalf("CompareJSON: %v", err)
	}
	if !eq {
		t.Error("CompareJSON should treat 1 and 1.0 as equal")
	}
}

func TestUnify_CompareJSON_WithConfig_AgreesWithNoConfig(t *testing.T) {
	a, b := `{"x":[1,2,3]}`, `{"x":[1,2,3]}`
	eqPlain, err := CompareJSON(a, b)
	if err != nil {
		t.Fatalf("CompareJSON plain: %v", err)
	}
	eqCfg, err := CompareJSON(a, b, DefaultConfig())
	if err != nil {
		t.Fatalf("CompareJSON with cfg: %v", err)
	}
	if eqPlain != eqCfg {
		t.Errorf("cfg changed CompareJSON result: plain=%v cfg=%v", eqPlain, eqCfg)
	}
}

// ============================================================================
// [D-006] follow-up — Processor method mirrors for CompareJSON / MergeJSON / MergeMany
//
// These three previously existed only at package level, leaving a symmetry gap
// in the mirror principle. The mirrors are verified for correctness and for
// parity with the package-level functions (json.Foo(args, cfg) == p.Foo(args, cfg)).
//
// Map outputs are compared by parsed value (reflect.DeepEqual), never by bytes:
// the library defaults SortKeys=false, so map key order is not byte-stable.
// ============================================================================

// unifyToMap parses a JSON object string into map[string]any via the stdlib
// (numbers as float64), giving an order-independent representation for DeepEqual.
func unifyToMap(t *testing.T, s string) map[string]any {
	t.Helper()
	var m map[string]any
	if err := stdjson.Unmarshal([]byte(s), &m); err != nil {
		t.Fatalf("unmarshal %q: %v", s, err)
	}
	return m
}

func TestUnify_CompareJSON_MethodMirror(t *testing.T) {
	proc, err := New(DefaultConfig())
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	// Single-key cases keep the symmetric marshal byte-stable (SortKeys=false).
	cases := []struct {
		name      string
		a, b      string
		wantEqual bool
	}{
		{"numeric precision", `{"a":1}`, `{"a":1.0}`, true},
		{"nested equal", `{"x":[1,2,3]}`, `{"x":[1,2,3]}`, true},
		{"differ value", `{"a":1}`, `{"a":2}`, false},
		{"differ key", `{"a":1}`, `{"b":1}`, false},
	}
	for _, tc := range cases {
		got, err := proc.CompareJSON(tc.a, tc.b)
		if err != nil {
			t.Errorf("%s: proc.CompareJSON error: %v", tc.name, err)
			continue
		}
		if got != tc.wantEqual {
			t.Errorf("%s: proc.CompareJSON = %v, want %v", tc.name, got, tc.wantEqual)
		}
		// Mirror parity: method result must equal the package-level result.
		pkgEq, err := CompareJSON(tc.a, tc.b, DefaultConfig())
		if err != nil {
			t.Errorf("%s: package CompareJSON error: %v", tc.name, err)
			continue
		}
		if got != pkgEq {
			t.Errorf("%s: mirror parity broken: method=%v package=%v", tc.name, got, pkgEq)
		}
	}
}

func TestUnify_MergeJSON_MethodMirror(t *testing.T) {
	proc, err := New(DefaultConfig())
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	a := `{"a":1,"nested":{"x":1}}`
	b := `{"b":2,"nested":{"y":2}}`

	got, err := proc.MergeJSON(a, b)
	if err != nil {
		t.Fatalf("proc.MergeJSON: %v", err)
	}

	// Correctness: union deep-merge contains keys from both sides, nested merged.
	want := map[string]any{
		"a":      1.0,
		"b":      2.0,
		"nested": map[string]any{"x": 1.0, "y": 2.0},
	}
	if !reflect.DeepEqual(unifyToMap(t, got), want) {
		t.Errorf("proc.MergeJSON value mismatch:\n got =%v\n want=%v", unifyToMap(t, got), want)
	}

	// Mirror parity: method output equals the package-level output (by value).
	pkgGot, err := MergeJSON(a, b)
	if err != nil {
		t.Fatalf("package MergeJSON: %v", err)
	}
	if !reflect.DeepEqual(unifyToMap(t, got), unifyToMap(t, pkgGot)) {
		t.Errorf("MergeJSON mirror parity broken:\n method=%s\n pkg   =%s", got, pkgGot)
	}
}

func TestUnify_MergeMany_MethodMirror(t *testing.T) {
	proc, err := New(DefaultConfig())
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	jsons := []string{`{"a":1}`, `{"b":2}`, `{"c":3}`}
	got, err := proc.MergeMany(jsons)
	if err != nil {
		t.Fatalf("proc.MergeMany: %v", err)
	}

	// Correctness: all keys folded in.
	want := map[string]any{"a": 1.0, "b": 2.0, "c": 3.0}
	if !reflect.DeepEqual(unifyToMap(t, got), want) {
		t.Errorf("proc.MergeMany value mismatch:\n got =%v\n want=%v", unifyToMap(t, got), want)
	}

	// Mirror parity with the package-level function.
	pkgGot, err := MergeMany(jsons)
	if err != nil {
		t.Fatalf("package MergeMany: %v", err)
	}
	if !reflect.DeepEqual(unifyToMap(t, got), unifyToMap(t, pkgGot)) {
		t.Errorf("MergeMany mirror parity broken:\n method=%s\n pkg   =%s", got, pkgGot)
	}

	// Contract parity: fewer than 2 inputs errors on the method too.
	if _, err := proc.MergeMany([]string{`{"a":1}`}); err == nil {
		t.Error("proc.MergeMany(<2) should error, matching the package contract")
	}
}

// ============================================================================
// [D-005] Phase 2 — per-call Config is now actually enforced
//
// Phase 1 made package-level functions ACCEPT cfg; Phase 2 closes the gap that
// cfg was silently ignored for security limits. validateInput previously used
// the processor's baked-in config; validateInputForOptions now honors a
// caller-supplied cfg across Get/Set/Delete/Valid/Parse/GetMultiple/
// SetMultiple/Prettify/Compact/ValidateSchema/PreParse/WarmupCache.
//
// These tests assert the enforcement end-to-end via the package-level API,
// using MaxNestingDepthSecurity as the signal (its valid range [10,200] allows
// a small per-call limit that a deeply nested payload exceeds). The no-cfg
// path must keep accepting the same payload (default limit 200).
// ============================================================================

// nestedJSON builds a valid JSON object nested `depth` levels deep:
// {"a":{"a":...{"a":1}...}}. depth=50 is well under the default limit (200)
// but exceeds a per-call limit of 15.
func nestedJSON(depth int) string {
	return strings.Repeat(`{"a":`, depth) + `1` + strings.Repeat(`}`, depth)
}

// perCallNestingCfg returns a Config whose nesting limit (15) a 50-deep
// document exceeds but which survives Validate's clamp (min 10, max 200).
func perCallNestingCfg() Config {
	cfg := DefaultConfig()
	cfg.MaxNestingDepthSecurity = 15
	return cfg
}

func TestUnify_Valid_EnforcesPerCallCfg(t *testing.T) {
	nested := nestedJSON(50) // depth 50, valid under the default limit (200)

	// No cfg: default processor accepts it.
	if !Valid([]byte(nested)) {
		t.Error("Valid without cfg should accept 50-deep JSON (default limit 200)")
	}
	// Per-call cfg with a 15-deep limit must reject the 50-deep payload.
	if Valid([]byte(nested), perCallNestingCfg()) {
		t.Error("Valid(nested, cfg{nesting:15}) should reject 50-deep JSON; per-call cfg was not enforced")
	}
}

func TestUnify_Get_EnforcesPerCallCfg(t *testing.T) {
	nested := nestedJSON(50)

	// No cfg: Get succeeds (validateOperationInput uses processor default).
	if _, err := Get(nested, "a"); err != nil {
		t.Errorf("Get without cfg should accept 50-deep JSON: %v", err)
	}
	// Per-call cfg: the 50-deep payload is rejected before navigation.
	// This exercises the prepareOperation -> validateOperationInput path
	// shared by Get/Set/Delete.
	if _, err := Get(nested, "a", perCallNestingCfg()); err == nil {
		t.Error("Get(nested, \"a\", cfg{nesting:15}) should reject; per-call cfg was not enforced")
	}
}

func TestUnify_Parse_EnforcesPerCallCfg(t *testing.T) {
	nested := nestedJSON(50)
	var v any

	// No cfg fast path: parses fine.
	if err := Parse(nested, &v); err != nil {
		t.Errorf("Parse without cfg should accept 50-deep JSON: %v", err)
	}
	// Per-call cfg slow path: rejected before parsing.
	if err := Parse(nested, &v, perCallNestingCfg()); err == nil {
		t.Error("Parse(nested, &v, cfg{nesting:15}) should reject; per-call cfg was not enforced")
	}
}

func TestUnify_Set_EnforcesPerCallCfg(t *testing.T) {
	nested := nestedJSON(50)

	// No cfg: Set succeeds.
	if _, err := Set(nested, "a", 2); err != nil {
		t.Errorf("Set without cfg should accept 50-deep JSON: %v", err)
	}
	// Per-call cfg: rejected.
	if _, err := Set(nested, "a", 2, perCallNestingCfg()); err == nil {
		t.Error("Set(nested, \"a\", 2, cfg{nesting:15}) should reject; per-call cfg was not enforced")
	}
}

func TestUnify_NoCfg_Path_UnaffectedByProcessorConfig(t *testing.T) {
	// A processor built with a tight nesting limit must STILL enforce it when
	// its methods are called WITHOUT a per-call cfg (the fix must not loosen a
	// SecurityConfig processor's own limits). 50-deep exceeds 15.
	p, err := New(perCallNestingCfg())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer p.Close()
	if _, err := p.Get(nestedJSON(50), "a"); err == nil {
		t.Error("SecurityConfig processor should reject 50-deep JSON on no-cfg Get (its own limit is 15)")
	}
}

// TestUnify_Encode_DeprecatedStillWorks locks in that Encode (deprecated in
// Phase 2 as identical to EncodeWithConfig) still functions and stays a mirror
// of EncodeWithConfig. Removal is deferred to a future major version.
func TestUnify_Encode_DeprecatedStillWorks(t *testing.T) {
	v := unifyUser{Name: "Alice", Age: 30}
	out, err := Encode(v)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	want, err := EncodeWithConfig(v)
	if err != nil {
		t.Fatalf("EncodeWithConfig: %v", err)
	}
	if out != want {
		t.Errorf("Encode diverged from EncodeWithConfig:\n Encode=%q\n EWC   =%q", out, want)
	}
}

// ============================================================================
// [D-005] Phase 3 — Compact naming divergence resolved
//
// Phase 3 closes the one place the mirror principle was broken: package-level
// Compact (buffer form, encoding/json-compatible) and Processor.Compact (string
// form) shared a name but differed in signature AND behavior — json.Compact was
// the mirror of p.CompactBuffer, while p.Compact had no package-level mirror.
//
// Fix (additive): CompactString is introduced as the package-level mirror of
// Processor.Compact (json.CompactString(s) ↔ p.Compact(s)), symmetric with
// Prettify mirroring p.Prettify. json.Compact(dst, src) still mirrors
// p.CompactBuffer. No existing symbol changed.
// ============================================================================

func TestUnify_CompactString_RemovesWhitespace(t *testing.T) {
	in := "{\n\t\"name\": \"Alice\",\n\t\"age\": 30\n}"
	got, err := CompactString(in)
	if err != nil {
		t.Fatalf("CompactString: %v", err)
	}
	// Compact form must contain no insignificant whitespace. Map key order is
	// NOT byte-stable (see file header), so verify compactness by whitespace
	// absence and equivalence by parsed value — not by exact bytes.
	if strings.ContainsAny(got, "\n\t") {
		t.Errorf("CompactString left whitespace in output: %q", got)
	}
	var gotVal, wantVal any
	if err := stdjson.Unmarshal([]byte(got), &gotVal); err != nil {
		t.Fatalf("CompactString output is not valid JSON: %v", err)
	}
	if err := stdjson.Unmarshal([]byte(`{"name":"Alice","age":30}`), &wantVal); err != nil {
		t.Fatalf("expected value is not valid JSON: %v", err)
	}
	if !reflect.DeepEqual(gotVal, wantVal) {
		t.Errorf("CompactString value mismatch: got %q", got)
	}
}

func TestUnify_CompactString_MirrorsProcessor(t *testing.T) {
	// Map key order is non-deterministic when SortKeys=false (see file header),
	// so the package and processor outputs are compared by parsed value, not by
	// byte-equality — matching the contract this file documents for maps.
	in := "{\n  \"name\": \"Alice\",\n  \"age\": 30\n}"
	cfg := DefaultConfig()
	cfg.PreserveNumbers = true

	pkgOut, err := CompactString(in, cfg)
	if err != nil {
		t.Fatalf("package CompactString: %v", err)
	}
	p, err := New()
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer p.Close()
	procOut, err := p.Compact(in, cfg)
	if err != nil {
		t.Fatalf("processor Compact: %v", err)
	}

	var pkgVal, procVal any
	if err := stdjson.Unmarshal([]byte(pkgOut), &pkgVal); err != nil {
		t.Fatalf("unmarshal package CompactString output: %v", err)
	}
	if err := stdjson.Unmarshal([]byte(procOut), &procVal); err != nil {
		t.Fatalf("unmarshal processor Compact output: %v", err)
	}
	if !reflect.DeepEqual(pkgVal, procVal) {
		t.Errorf("package CompactString and processor Compact diverged by value:\n pkg=%q\nproc=%q", pkgOut, procOut)
	}
}

// TestUnify_Compact_BufferVsString_Distinct locks in that the buffer form
// (json.Compact, mirror of p.CompactBuffer) and the string form (json.CompactString,
// mirror of p.Compact) coexist without colliding — the original divergence is
// resolved by naming, not by repurposing either symbol.
func TestUnify_Compact_BufferVsString_Distinct(t *testing.T) {
	src := []byte(`{ "a": 1 }`)

	// Buffer form (encoding/json-compatible).
	var buf bytes.Buffer
	if err := Compact(&buf, src); err != nil {
		t.Fatalf("Compact(buffer): %v", err)
	}

	// String form.
	s, err := CompactString(string(src))
	if err != nil {
		t.Fatalf("CompactString: %v", err)
	}
	if buf.String() != s {
		t.Errorf("buffer and string compact forms disagree: buf=%q str=%q", buf.String(), s)
	}
}

// ============================================================================
// [D-005] Phase 4 — iterate family cfg unified
//
// Before Phase 4 the iterate family was split: 5 package-level functions
// (Foreach, ForeachWithPath, ForeachWithPathAndControl, ForeachReturn,
// ForeachNested) accepted cfg, but 3 (ForeachWithError, ForeachNestedWithError,
// ForeachWithPathAndIterator) did not, and none of the *methods* took per-call
// cfg. Phase 4 adds a trailing cfg ...Config to all 8 Processor methods (threaded
// to the internal Get) and to the 3 missing package wrappers, making both layers
// uniform. cfg flows to Get, so per-call security limits apply (Phase 2 semantics).
// ============================================================================

func TestUnify_ForeachWithError_EnforcesPerCallCfg(t *testing.T) {
	nested := nestedJSON(50) // 50-deep, valid under default limit (200)

	// Method, no cfg: default processor accepts 50-deep.
	p, err := New()
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer p.Close()
	if err := p.ForeachWithError(nested, ".", func(key any, item *IterableValue) error { return nil }); err != nil {
		t.Errorf("p.ForeachWithError without cfg should accept 50-deep JSON: %v", err)
	}
	// Method, per-call cfg (limit 15): rejected before iteration.
	if err := p.ForeachWithError(nested, ".", func(key any, item *IterableValue) error { return nil }, perCallNestingCfg()); err == nil {
		t.Error("p.ForeachWithError(nested, cfg{nesting:15}) should reject; per-call cfg was not enforced")
	}
}

func TestUnify_ForeachWithError_PackageForwardsCfg(t *testing.T) {
	nested := nestedJSON(50)

	// Package-level, no cfg: accepts.
	if err := ForeachWithError(nested, ".", func(key any, item *IterableValue) error { return nil }); err != nil {
		t.Errorf("package ForeachWithError without cfg should accept 50-deep JSON: %v", err)
	}
	// Package-level, per-call cfg (limit 15): rejected — confirms the wrapper
	// forwards cfg into the method (and thus into Get's per-call validation).
	if err := ForeachWithError(nested, ".", func(key any, item *IterableValue) error { return nil }, perCallNestingCfg()); err == nil {
		t.Error("package ForeachWithError(nested, cfg{nesting:15}) should reject; cfg was not forwarded")
	}
}

// TestUnify_Foreach_FamilyAcceptsCfg is a compile-time/behavior guard that every
// package-level iterate function now accepts a trailing cfg without changing the
// no-cfg behavior. It exercises one representative of each formerly-cfg-less
// wrapper plus a formerly-cfg-accepting one.
func TestUnify_Foreach_FamilyAcceptsCfg(t *testing.T) {
	data := `{"a":1,"b":2}`
	called := 0

	// Formerly cfg-less wrappers now accept cfg.
	if err := ForeachWithError(data, ".", func(key any, item *IterableValue) error { called++; return nil }, DefaultConfig()); err != nil {
		t.Errorf("ForeachWithError with cfg: %v", err)
	}
	if err := ForeachNestedWithError(data, func(key any, item *IterableValue) error { return nil }, DefaultConfig()); err != nil {
		t.Errorf("ForeachNestedWithError with cfg: %v", err)
	}
	if err := ForeachWithPathAndIterator(data, ".", func(key any, item *IterableValue, currentPath string) IteratorControl { return IteratorNormal }, DefaultConfig()); err != nil {
		t.Errorf("ForeachWithPathAndIterator with cfg: %v", err)
	}
	// Formerly cfg-accepting wrapper still works.
	Foreach(data, func(key any, item *IterableValue) {}, DefaultConfig())

	if called != 2 {
		t.Errorf("ForeachWithError callback count = %d, want 2", called)
	}
}

// ============================================================================
// [D-005] Phase 5 — JSONL/stream family cfg unified
//
// Before Phase 5 the 11 JSONL package wrappers (StreamJSONL, StreamJSONLParallel,
// StreamJSONLParallelWithContext, StreamJSONLChunked, ForeachJSONL, MapJSONL,
// ReduceJSONL, FilterJSONL, StreamJSONLFile, CollectJSONL, FirstJSONL) selected
// the default processor and had no way to honor a per-call Config. Phase 5 routes
// them through processorForCfg: with cfg omitted the default processor is used
// (behavior unchanged); with cfg supplied a config-cached processor is selected,
// whose baked-in JSONL settings (buffer/line sizes, memory limit, nesting cap)
// reflect cfg.
//
// The behavioral signal is MaxNestingDepthSecurity: Processor.StreamJSONL reads
// p.config.MaxNestingDepthSecurity and rejects lines deeper than it
// (processor_streamjsonl.go), so a cfg-cached processor with nesting=15 rejects a
// 50-deep line that the default processor (limit 200) accepts.
// ============================================================================

func TestUnify_StreamJSONL_NoCfg_BehaviorUnchanged(t *testing.T) {
	data := `{"id":1,"name":"a"}` + "\n" + `{"id":2,"name":"b"}` + "\n"
	var seen []int
	err := StreamJSONL(strings.NewReader(data), func(lineNum int, item *IterableValue) error {
		seen = append(seen, item.GetInt("id"))
		return nil
	})
	if err != nil {
		t.Fatalf("StreamJSONL without cfg: %v", err)
	}
	if len(seen) != 2 || seen[0] != 1 || seen[1] != 2 {
		t.Errorf("StreamJSONL processed wrong ids: %v", seen)
	}
}

func TestUnify_StreamJSONL_EnforcesPerCallCfg(t *testing.T) {
	// One 50-deep line: accepted by the default processor (limit 200).
	nested := nestedJSON(50)
	stream := nested + "\n"

	if err := StreamJSONL(strings.NewReader(stream), func(lineNum int, item *IterableValue) error { return nil }); err != nil {
		t.Errorf("StreamJSONL without cfg should accept 50-deep line: %v", err)
	}
	// Same stream with a per-call cfg (nesting 15): the config-cached processor
	// rejects the line — confirming cfg flows into processor selection.
	if err := StreamJSONL(strings.NewReader(stream), func(lineNum int, item *IterableValue) error { return nil }, perCallNestingCfg()); err == nil {
		t.Error("StreamJSONL(nested, cfg{nesting:15}) should reject; per-call cfg was not enforced")
	}
}

func TestUnify_CollectJSONL_AcceptsCfgAndMirrors(t *testing.T) {
	data := `{"x":1}` + "\n" + `{"x":2}` + "\n"

	pkgPlain, err := CollectJSONL(strings.NewReader(data))
	if err != nil {
		t.Fatalf("CollectJSONL no cfg: %v", err)
	}
	pkgCfg, err := CollectJSONL(strings.NewReader(data), DefaultConfig())
	if err != nil {
		t.Fatalf("CollectJSONL with cfg: %v", err)
	}
	if len(pkgPlain) != 2 || len(pkgCfg) != 2 {
		t.Errorf("CollectJSONL counts wrong: plain=%d cfg=%d", len(pkgPlain), len(pkgCfg))
	}
	// Mirror: package CollectJSONL(r, cfg) matches a processor built from cfg.
	p, err := New(DefaultConfig())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer p.Close()
	procOut, err := p.CollectJSONL(strings.NewReader(data))
	if err != nil {
		t.Fatalf("processor CollectJSONL: %v", err)
	}
	if len(procOut) != len(pkgCfg) {
		t.Errorf("package and processor CollectJSONL diverged: pkg=%d proc=%d", len(pkgCfg), len(procOut))
	}
}

// TestUnify_JSONL_FamilyAcceptsCfg is a smoke guard that every formerly-cfg-less
// JSONL wrapper now accepts a trailing cfg and still produces correct results.
func TestUnify_JSONL_FamilyAcceptsCfg(t *testing.T) {
	data := `{"n":1}` + "\n" + `{"n":2}` + "\n"
	r := strings.NewReader

	vals, err := MapJSONL(r(data), func(lineNum int, item *IterableValue) (any, error) {
		return item.GetInt("n"), nil
	}, DefaultConfig())
	if err != nil {
		t.Errorf("MapJSONL with cfg: %v", err)
	}
	if len(vals) != 2 {
		t.Errorf("MapJSONL count = %d, want 2", len(vals))
	}

	item, ok, err := FirstJSONL(r(data), func(item *IterableValue) bool { return true }, DefaultConfig())
	if err != nil || !ok || item == nil {
		t.Errorf("FirstJSONL with cfg: ok=%v err=%v", ok, err)
	}

	filt, err := FilterJSONL(r(data), func(item *IterableValue) bool { return true }, DefaultConfig())
	if err != nil {
		t.Errorf("FilterJSONL with cfg: %v", err)
	}
	if len(filt) != 2 {
		t.Errorf("FilterJSONL count = %d, want 2", len(filt))
	}

	acc, err := ReduceJSONL(r(data), 0, func(acc any, item *IterableValue) any {
		return acc.(int) + item.GetInt("n")
	}, DefaultConfig())
	if err != nil {
		t.Errorf("ReduceJSONL with cfg: %v", err)
	}
	if acc.(int) != 3 {
		t.Errorf("ReduceJSONL sum = %v, want 3", acc)
	}

	if err := ForeachJSONL(r(data), func(lineNum int, item *IterableValue) error { return nil }, DefaultConfig()); err != nil {
		t.Errorf("ForeachJSONL with cfg: %v", err)
	}
	if err := StreamJSONLChunked(r(data), 2, func(chunk []*IterableValue) error { return nil }, DefaultConfig()); err != nil {
		t.Errorf("StreamJSONLChunked with cfg: %v", err)
	}
	if err := StreamJSONLParallel(r(data), 2, func(lineNum int, item *IterableValue) error { return nil }, DefaultConfig()); err != nil {
		t.Errorf("StreamJSONLParallel with cfg: %v", err)
	}
}

// ============================================================================
// [D-005] Phase 2 — mirror completion
//
// The JSONL/stream Processor methods now accept the same trailing cfg as
// their package-level counterparts; p.CompactString / p.ToJSONL /
// p.ToJSONLString / p.ParseJSONL join the mirror set; NewSchema replaces
// NewSchemaWithConfig; Encode becomes the canonical encoder (EncodeWithConfig
// deprecated, decision D1); the void Foreach/ForeachNested forms are
// deprecated in favor of the *WithError variants (decision D2).
// ============================================================================

// unifyLine captures one JSONL callback visit for mirror comparisons.
type unifyLine struct {
	line int
	n    int
}

// collectUnifyStream runs a JSONL stream and records (lineNum, item.n) per visit.
func collectUnifyStream(t *testing.T, stream func(fn func(lineNum int, item *IterableValue) error) error) []unifyLine {
	t.Helper()
	var got []unifyLine
	if err := stream(func(lineNum int, item *IterableValue) error {
		got = append(got, unifyLine{line: lineNum, n: item.GetInt("n")})
		return nil
	}); err != nil {
		t.Fatalf("stream: %v", err)
	}
	return got
}

// TestUnify_StreamJSONL_PerCallCfg locks the per-call Config semantics on the
// Processor side: no cfg → baked config (unchanged behavior); supplied cfg →
// validated replace, not merge.
func TestUnify_StreamJSONL_PerCallCfg(t *testing.T) {
	data := "// comment\n" + `{"n":1}` + "\n"

	skip := DefaultConfig()
	skip.JSONLSkipComments = true

	p, err := New()
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer p.Close()

	// No cfg: baked default config rejects the comment line.
	if err := p.StreamJSONL(strings.NewReader(data), func(_ int, _ *IterableValue) error { return nil }); err == nil {
		t.Errorf("default config accepted a comment line; want parse error")
	}

	// Per-call cfg enables comment skipping on the same processor.
	count := 0
	if err := p.StreamJSONL(strings.NewReader(data), func(_ int, item *IterableValue) error {
		count++
		if item.GetInt("n") != 1 {
			t.Errorf("n = %d, want 1", item.GetInt("n"))
		}
		return nil
	}, skip); err != nil {
		t.Errorf("per-call JSONLSkipComments: %v", err)
	}
	if count != 1 {
		t.Errorf("callback ran %d times, want 1", count)
	}

	// A per-call cfg REPLACES the baked config — a processor baked with
	// JSONLSkipComments still rejects the line when the per-call cfg leaves
	// it off.
	baked, err := New(skip)
	if err != nil {
		t.Fatalf("New(skip): %v", err)
	}
	defer baked.Close()
	plain := DefaultConfig()
	if err := baked.StreamJSONL(strings.NewReader(data), func(_ int, _ *IterableValue) error { return nil }, plain); err == nil {
		t.Errorf("per-call cfg failed to replace baked JSONLSkipComments")
	}
	// ...and the no-cfg call on the same processor still uses the baked value.
	if err := baked.StreamJSONL(strings.NewReader(data), func(_ int, _ *IterableValue) error { return nil }); err != nil {
		t.Errorf("baked JSONLSkipComments not honored on no-cfg call: %v", err)
	}
}

// TestUnify_StreamJSONL_Mirror guards json.StreamJSONL(r, fn, cfg) ≡
// p.StreamJSONL(r, fn, cfg).
func TestUnify_StreamJSONL_Mirror(t *testing.T) {
	data := "// c\n" + `{"n":1}` + "\n" + `{"n":2}` + "\n"
	cfg := DefaultConfig()
	cfg.JSONLSkipComments = true

	pkg := collectUnifyStream(t, func(fn func(lineNum int, item *IterableValue) error) error {
		return StreamJSONL(strings.NewReader(data), fn, cfg)
	})

	p, err := New()
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer p.Close()
	proc := collectUnifyStream(t, func(fn func(lineNum int, item *IterableValue) error) error {
		return p.StreamJSONL(strings.NewReader(data), fn, cfg)
	})

	if !reflect.DeepEqual(pkg, proc) {
		t.Errorf("package and processor StreamJSONL diverged:\n pkg=%v\nproc=%v", pkg, proc)
	}
	// Line 1 is the skipped comment; numbering keeps counting skipped lines.
	want := []unifyLine{{line: 2, n: 1}, {line: 3, n: 2}}
	if !reflect.DeepEqual(pkg, want) {
		t.Errorf("StreamJSONL result = %v, want %v", pkg, want)
	}
}

// TestUnify_JSONL_Family_ProcessorPerCallCfg exercises cfg forwarding through
// the combinator methods and the file/parallel engines.
func TestUnify_JSONL_Family_ProcessorPerCallCfg(t *testing.T) {
	data := "// c\n" + `{"n":1}` + "\n" + `{"n":2}` + "\n"
	skip := DefaultConfig()
	skip.JSONLSkipComments = true

	p, err := New()
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer p.Close()

	vals, err := p.MapJSONL(strings.NewReader(data), func(_ int, item *IterableValue) (any, error) {
		return item.GetInt("n"), nil
	}, skip)
	if err != nil {
		t.Errorf("p.MapJSONL per-call cfg: %v", err)
	} else if !reflect.DeepEqual(vals, []any{1, 2}) {
		t.Errorf("p.MapJSONL = %v, want [1 2]", vals)
	}

	items, err := p.CollectJSONL(strings.NewReader(data), skip)
	if err != nil {
		t.Errorf("p.CollectJSONL per-call cfg: %v", err)
	} else if len(items) != 2 {
		t.Errorf("p.CollectJSONL len = %d, want 2", len(items))
	}

	first, ok, err := p.FirstJSONL(strings.NewReader(data), func(item *IterableValue) bool {
		return item.GetInt("n") == 2
	}, skip)
	if err != nil || !ok || first == nil {
		t.Errorf("p.FirstJSONL per-call cfg: ok=%v err=%v", ok, err)
	}

	filtered, err := p.FilterJSONL(strings.NewReader(data), func(item *IterableValue) bool {
		return item.GetInt("n") == 1
	}, skip)
	if err != nil {
		t.Errorf("p.FilterJSONL per-call cfg: %v", err)
	} else if len(filtered) != 1 {
		t.Errorf("p.FilterJSONL len = %d, want 1", len(filtered))
	}

	sum, err := p.ReduceJSONL(strings.NewReader(data), 0, func(acc any, item *IterableValue) any {
		return acc.(int) + item.GetInt("n")
	}, skip)
	if err != nil {
		t.Errorf("p.ReduceJSONL per-call cfg: %v", err)
	} else if sum.(int) != 3 {
		t.Errorf("p.ReduceJSONL = %v, want 3", sum)
	}

	if err := p.ForeachJSONL(strings.NewReader(data), func(_ int, _ *IterableValue) error { return nil }, skip); err != nil {
		t.Errorf("p.ForeachJSONL per-call cfg: %v", err)
	}
	if err := p.StreamJSONLChunked(strings.NewReader(data), 1, func(_ []*IterableValue) error { return nil }, skip); err != nil {
		t.Errorf("p.StreamJSONLChunked per-call cfg: %v", err)
	}

	// Parallel engine: callback order is nondeterministic, compare as a set.
	// The callback runs concurrently on worker goroutines, so the collector
	// needs its own mutex.
	var mu sync.Mutex
	var lines []int
	if err := p.StreamJSONLParallelWithContext(context.Background(), strings.NewReader(data), 2, func(lineNum int, _ *IterableValue) error {
		mu.Lock()
		defer mu.Unlock()
		lines = append(lines, lineNum)
		return nil
	}, skip); err != nil {
		t.Errorf("p.StreamJSONLParallelWithContext per-call cfg: %v", err)
	}
	slices.Sort(lines)
	if !reflect.DeepEqual(lines, []int{2, 3}) {
		t.Errorf("parallel lines = %v, want [2 3]", lines)
	}

	// File-based variant forwards cfg too.
	path := filepath.Join(t.TempDir(), "unify.jsonl")
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	fileCount := 0
	if err := p.StreamJSONLFile(path, func(_ int, _ *IterableValue) error {
		fileCount++
		return nil
	}, skip); err != nil {
		t.Errorf("p.StreamJSONLFile per-call cfg: %v", err)
	}
	if fileCount != 2 {
		t.Errorf("p.StreamJSONLFile visits = %d, want 2", fileCount)
	}
}

// TestUnify_CompactString_Mirror guards json.CompactString(s, cfg) ≡
// p.CompactString(s, cfg) ≡ p.Compact(s, cfg) (D-005 Phase 2 alias).
func TestUnify_CompactString_Mirror(t *testing.T) {
	src := "{\n  \"name\": \"Alice\",\n  \"age\": 30\n}\n"

	pkg, err := CompactString(src)
	if err != nil {
		t.Fatalf("CompactString: %v", err)
	}

	p, err := New()
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer p.Close()
	alias, err := p.CompactString(src)
	if err != nil {
		t.Fatalf("p.CompactString: %v", err)
	}
	underlying, err := p.Compact(src)
	if err != nil {
		t.Fatalf("p.Compact: %v", err)
	}

	if pkg != alias || alias != underlying {
		t.Errorf("CompactString mirror diverged:\n pkg=%s\nalias=%s\nunder=%s", pkg, alias, underlying)
	}
	if pkg != `{"name":"Alice","age":30}` {
		t.Errorf("CompactString = %q", pkg)
	}
}

// TestUnify_ToJSONL_Mirror guards json.ToJSONL(data, cfg) ≡ p.ToJSONL(data,
// cfg), the string mirror, and that the no-cfg method call uses the baked
// config.
func TestUnify_ToJSONL_Mirror(t *testing.T) {
	data := []any{map[string]any{"url": "<script>"}}

	cfg := DefaultConfig()
	cfg.EscapeHTML = false

	pkgBytes, err := ToJSONL(data, cfg)
	if err != nil {
		t.Fatalf("ToJSONL: %v", err)
	}

	p, err := New()
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer p.Close()
	procBytes, err := p.ToJSONL(data, cfg)
	if err != nil {
		t.Fatalf("p.ToJSONL: %v", err)
	}
	procStr, err := p.ToJSONLString(data, cfg)
	if err != nil {
		t.Fatalf("p.ToJSONLString: %v", err)
	}

	if !bytes.Equal(pkgBytes, procBytes) {
		t.Errorf("ToJSONL mirror diverged:\n pkg=%s\nproc=%s", pkgBytes, procBytes)
	}
	if string(procBytes) != procStr {
		t.Errorf("p.ToJSONLString = %q, want %q", procStr, procBytes)
	}
	if !strings.Contains(procStr, "<script>") {
		t.Errorf("EscapeHTML=false not honored: %q", procStr)
	}

	// No-cfg method call uses the processor's baked config.
	baked, err := New(cfg)
	if err != nil {
		t.Fatalf("New(cfg): %v", err)
	}
	defer baked.Close()
	bakedOut, err := baked.ToJSONLString(data)
	if err != nil {
		t.Fatalf("baked.ToJSONLString: %v", err)
	}
	if bakedOut != procStr {
		t.Errorf("baked config not honored: %q != %q", bakedOut, procStr)
	}
}

// TestUnify_ToJSONL_EmptyData_NoProcessorInteraction locks the re-review fix:
// empty input returns before any processor interaction (the pre-Phase-2
// package contract), so even a closed processor yields an empty result, not
// an error.
func TestUnify_ToJSONL_EmptyData_NoProcessorInteraction(t *testing.T) {
	for _, data := range [][]any{nil, {}} {
		got, err := ToJSONL(data)
		if err != nil || len(got) != 0 {
			t.Errorf("ToJSONL(%#v) = (%d bytes, %v), want (0, nil)", data, len(got), err)
		}
	}

	p, err := New()
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := p.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	got, err := p.ToJSONL(nil)
	if err != nil || len(got) != 0 {
		t.Errorf("closed p.ToJSONL(nil) = (%d bytes, %v), want (0, nil)", len(got), err)
	}
}

// TestUnify_StreamJSONLChunked_PerCallCfgEffect verifies the chunked engine
// actually applies a per-call Config (not just accepts the parameter).
func TestUnify_StreamJSONLChunked_PerCallCfgEffect(t *testing.T) {
	data := "// c\n" + `{"n":1}` + "\n" + `{"n":2}` + "\n"
	skip := DefaultConfig()
	skip.JSONLSkipComments = true

	p, err := New()
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer p.Close()

	// Without cfg the comment line fails to parse.
	if err := p.StreamJSONLChunked(strings.NewReader(data), 10, func(_ []*IterableValue) error {
		return nil
	}); err == nil {
		t.Errorf("chunked engine accepted a comment line without cfg")
	}

	items := 0
	if err := p.StreamJSONLChunked(strings.NewReader(data), 1, func(chunk []*IterableValue) error {
		items += len(chunk)
		return nil
	}, skip); err != nil {
		t.Errorf("chunked per-call cfg: %v", err)
	}
	if items != 2 {
		t.Errorf("chunked per-call cfg visited %d items, want 2", items)
	}
}

// TestUnify_ParseJSONL_Mirror guards json.ParseJSONL(data, cfg) ≡
// p.ParseJSONL(data, cfg).
func TestUnify_ParseJSONL_Mirror(t *testing.T) {
	data := []byte("// c\n" + `{"n":1}` + "\n")
	skip := DefaultConfig()
	skip.JSONLSkipComments = true

	pkg, err := ParseJSONL(data, skip)
	if err != nil {
		t.Fatalf("ParseJSONL: %v", err)
	}

	p, err := New()
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer p.Close()
	proc, err := p.ParseJSONL(data, skip)
	if err != nil {
		t.Fatalf("p.ParseJSONL: %v", err)
	}

	if !reflect.DeepEqual(pkg, proc) {
		t.Errorf("ParseJSONL mirror diverged:\n pkg=%v\nproc=%v", pkg, proc)
	}
	if len(proc) != 1 {
		t.Errorf("p.ParseJSONL len = %d, want 1", len(proc))
	}
}

// TestUnify_NewSchema_ReplacesWithConfig guards that NewSchema(cfg) and the
// deprecated NewSchemaWithConfig(cfg) behave identically.
func TestUnify_NewSchema_ReplacesWithConfig(t *testing.T) {
	newSchema := func(ctor func(SchemaConfig) *Schema) *Schema {
		cfg := DefaultSchemaConfig()
		cfg.Type = "object"
		cfg.Required = []string{"name"}
		return ctor(cfg)
	}

	p, err := New()
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer p.Close()

	for name, ctor := range map[string]func(SchemaConfig) *Schema{
		"NewSchema":           NewSchema,
		"NewSchemaWithConfig": NewSchemaWithConfig,
	} {
		violations, err := p.ValidateSchema(`{"age": 30}`, newSchema(ctor), DefaultConfig())
		if err != nil {
			t.Fatalf("%s: ValidateSchema: %v", name, err)
		}
		if len(violations) == 0 {
			t.Errorf("%s: expected required-field violation", name)
		}
	}
}

// TestUnify_Encode_Canonical locks decision D1: Encode is canonical and
// EncodeWithConfig (now deprecated) stays byte-identical on both layers.
func TestUnify_Encode_Canonical(t *testing.T) {
	v := unifyUser{Name: "Alice", Age: 30, Active: true}

	pkgCanon, err := Encode(v, PrettyConfig())
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	pkgDeprecated, err := EncodeWithConfig(v, PrettyConfig())
	if err != nil {
		t.Fatalf("EncodeWithConfig: %v", err)
	}

	p, err := New()
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer p.Close()
	procCanon, err := p.Encode(v, PrettyConfig())
	if err != nil {
		t.Fatalf("p.Encode: %v", err)
	}
	procDeprecated, err := p.EncodeWithConfig(v, PrettyConfig())
	if err != nil {
		t.Fatalf("p.EncodeWithConfig: %v", err)
	}

	if pkgCanon != pkgDeprecated || pkgDeprecated != procCanon || procCanon != procDeprecated {
		t.Errorf("encode forms diverged:\n pkg=%s\npkgDep=%s\nproc=%s\nprocDep=%s",
			pkgCanon, pkgDeprecated, procCanon, procDeprecated)
	}

	// No-cfg Encode stays the compact default.
	compact, err := Encode(v)
	if err != nil {
		t.Fatalf("Encode (no cfg): %v", err)
	}
	if compact == pkgCanon || !strings.Contains(compact, `"Name":"Alice"`) && !strings.Contains(compact, `"name":"Alice"`) {
		t.Errorf("no-cfg Encode output unexpected: %s", compact)
	}
}

// TestUnify_ForeachDeprecated_VariantsSurfaceErrors locks decision D2: the
// *WithError replacements visit the same items and, unlike the void forms,
// surface failures.
func TestUnify_ForeachDeprecated_VariantsSurfaceErrors(t *testing.T) {
	valid := `{"users":[{"n":1},{"n":2}]}`
	invalid := `{"users":[}`

	// Same visit count as the deprecated void form: Foreach iterates the root
	// container, so the exact replacement is ForeachWithError(jsonStr, ".", fn).
	voidCount := 0
	Foreach(valid, func(_ any, _ *IterableValue) { voidCount++ }) //nolint:staticcheck // deprecated form under test

	errCount := 0
	if err := ForeachWithError(valid, ".", func(_ any, _ *IterableValue) error {
		errCount++
		return nil
	}); err != nil {
		t.Fatalf("ForeachWithError on valid input: %v", err)
	}
	if errCount != voidCount {
		t.Errorf("visit counts diverged: void=%d withError=%d", voidCount, errCount)
	}

	// Path-directed iteration reaches into the array.
	pathCount := 0
	if err := ForeachWithError(valid, "users", func(_ any, _ *IterableValue) error {
		pathCount++
		return nil
	}); err != nil {
		t.Fatalf("ForeachWithError(users): %v", err)
	}
	if pathCount != 2 {
		t.Errorf("ForeachWithError(users) visits = %d, want 2", pathCount)
	}

	// Errors surface; the void form cannot report them.
	if err := ForeachWithError(invalid, "users", func(_ any, _ *IterableValue) error { return nil }); err == nil {
		t.Errorf("ForeachWithError accepted invalid JSON")
	}
	if err := ForeachNestedWithError(invalid, func(_ any, _ *IterableValue) error { return nil }); err == nil {
		t.Errorf("ForeachNestedWithError accepted invalid JSON")
	}

	nestedCount := 0
	if err := ForeachNestedWithError(valid, func(_ any, _ *IterableValue) error {
		nestedCount++
		return nil
	}); err != nil {
		t.Fatalf("ForeachNestedWithError: %v", err)
	}
	if nestedCount == 0 {
		t.Errorf("ForeachNestedWithError visited nothing")
	}
}

// ============================================================================
// [D-006] Dual-layer consistency — the no-cfg path honors the processor's
// baked configuration, making json.Foo(args, cfg) and New(cfg).Foo(args)
// interchangeable for option-derived behavior (MergeMode, encoding options,
// MaxBatchSize). Previously these read the default-config singleton, so a
// custom-built processor silently ignored its own settings — the exact
// divergence class the D-006 audit flagged (P1/P2/P5).
// ============================================================================

func TestUnify_D006_MergeJSON_BakedMergeMode(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MergeMode = MergeIntersection

	p, err := New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer p.Close()

	a := `{"a":1,"x":1}`
	b := `{"b":2,"x":2}`

	method, err := p.MergeJSON(a, b)
	if err != nil {
		t.Fatalf("p.MergeJSON(no cfg): %v", err)
	}
	pkg, err := MergeJSON(a, b, cfg)
	if err != nil {
		t.Fatalf("package MergeJSON(cfg): %v", err)
	}
	if !reflect.DeepEqual(unifyToMap(t, method), unifyToMap(t, pkg)) {
		t.Errorf("baked MergeMode ignored by the method:\n method=%s\n pkg   =%s", method, pkg)
	}
	// Intersection keeps only the shared key.
	want := map[string]any{"x": 2.0}
	if !reflect.DeepEqual(unifyToMap(t, method), want) {
		t.Errorf("intersection result wrong:\n got =%v\n want=%v", unifyToMap(t, method), want)
	}

	// MergeMany folds through MergeJSON, so the fix propagates.
	mMethod, err := p.MergeMany([]string{a, b})
	if err != nil {
		t.Fatalf("p.MergeMany(no cfg): %v", err)
	}
	mPkg, err := MergeMany([]string{a, b}, cfg)
	if err != nil {
		t.Fatalf("package MergeMany(cfg): %v", err)
	}
	if !reflect.DeepEqual(unifyToMap(t, mMethod), unifyToMap(t, mPkg)) {
		t.Errorf("baked MergeMode ignored by MergeMany:\n method=%s\n pkg   =%s", mMethod, mPkg)
	}
}

func TestUnify_D006_Encode_BakedEncodingOptions(t *testing.T) {
	value := map[string]any{"k": 1, "n": map[string]any{"z": true}}

	p, err := New(PrettyConfig())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer p.Close()

	method, err := p.Encode(value)
	if err != nil {
		t.Fatalf("p.Encode(no cfg): %v", err)
	}
	pkg, err := Encode(value, PrettyConfig())
	if err != nil {
		t.Fatalf("package Encode(cfg): %v", err)
	}
	if method != pkg {
		t.Errorf("baked encoding options ignored by Encode:\n method=%q\n pkg   =%q", method, pkg)
	}
	if !strings.Contains(method, "\n") {
		t.Errorf("Encode on a Pretty-baked processor produced compact output: %q", method)
	}

	// The rest of the Encode family resolves through Encode, so the same
	// parity must hold.
	if m, err := p.EncodeStream([]any{value}); err != nil || !strings.Contains(m, "\n") {
		t.Errorf("EncodeStream ignored baked Pretty: %q (err=%v)", m, err)
	}
	if m, err := p.EncodeBatch(map[string]any{"k": 1}); err != nil || !strings.Contains(m, "\n") {
		t.Errorf("EncodeBatch ignored baked Pretty: %q (err=%v)", m, err)
	}
	if m, err := p.EncodePretty(value); err != nil || m != pkg {
		t.Errorf("EncodePretty diverged from package Encode(cfg):\n method=%q\n pkg   =%q (err=%v)", m, pkg, err)
	}

	// Baked Indent is honored by EncodePretty (was forced to PrettyConfig's).
	tabbed := PrettyConfig()
	tabbed.Indent = "\t"
	p2, err := New(tabbed)
	if err != nil {
		t.Fatalf("New(tabbed): %v", err)
	}
	defer p2.Close()
	if m, err := p2.EncodePretty(value); err != nil || !strings.Contains(m, "\t") {
		t.Errorf("EncodePretty ignored baked Indent: %q (err=%v)", m, err)
	}
}

func TestUnify_D006_ProcessBatch_BakedMaxBatchSize(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MaxBatchSize = 1

	p, err := New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer p.Close()

	ops := []BatchOperation{
		{ID: "1", Type: "validate", JSONStr: "1"},
		{ID: "2", Type: "validate", JSONStr: "2"},
	}
	if _, err := p.ProcessBatch(ops); err == nil {
		t.Error("baked MaxBatchSize not enforced on Processor.ProcessBatch")
	}
	// Parity: the package level enforces the same limit via cfg.
	if _, err := ProcessBatch(ops, cfg); err == nil {
		t.Error("per-call MaxBatchSize not enforced on package ProcessBatch")
	}
	// A single op still passes on both layers.
	if _, err := p.ProcessBatch(ops[:1]); err != nil {
		t.Errorf("ProcessBatch(1 op) rejected under MaxBatchSize=1: %v", err)
	}
}

func TestUnify_D006_SetMultiple_BakedMaxBatchSize(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MaxBatchSize = 1

	p, err := New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer p.Close()

	updates := map[string]any{"a": 1, "b": 2}
	if _, err := p.SetMultiple(`{}`, updates); err == nil {
		t.Error("baked MaxBatchSize not enforced on Processor.SetMultiple")
	}
	if _, err := p.SetMultiple(`{}`, map[string]any{"a": 1}); err != nil {
		t.Errorf("SetMultiple(1 update) rejected under MaxBatchSize=1: %v", err)
	}
}

// ============================================================================
// [D-007] Regression round — locks the two directions of the no-cfg/cfg rule
// separately, so neither the baked fallback nor the replace semantics can
// silently drift:
//   1. Per-call cfg REPLACES the baked setting (including loosening it).
//   2. EncodePretty does not force Pretty on an explicit cfg (historical
//      contract; Pretty is forced only on the no-cfg path).
// ============================================================================

func TestUnify_D007_CfgReplacesBakedMergeMode(t *testing.T) {
	// Intersection baked; a per-call union cfg must replace it.
	baked := DefaultConfig()
	baked.MergeMode = MergeIntersection
	p, err := New(baked)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer p.Close()

	perCall := DefaultConfig() // MergeUnion
	got, err := p.MergeJSON(`{"a":1,"x":1}`, `{"b":2,"x":2}`, perCall)
	if err != nil {
		t.Fatalf("p.MergeJSON(cfg): %v", err)
	}
	want := map[string]any{"a": 1.0, "b": 2.0, "x": 2.0}
	if !reflect.DeepEqual(unifyToMap(t, got), want) {
		t.Errorf("per-call cfg did not replace baked MergeMode:\n got =%v\n want=%v", unifyToMap(t, got), want)
	}
}

func TestUnify_D007_CfgReplacesBakedMaxBatchSize(t *testing.T) {
	// Loose baked limit (default), tight per-call cfg: the cfg must win.
	p, err := New(DefaultConfig())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer p.Close()

	tight := DefaultConfig()
	tight.MaxBatchSize = 1
	ops := []BatchOperation{
		{ID: "1", Type: "validate", JSONStr: "1"},
		{ID: "2", Type: "validate", JSONStr: "2"},
	}
	if _, err := p.ProcessBatch(ops, tight); err == nil {
		t.Error("per-call MaxBatchSize=1 not enforced over loose baked limit")
	}
	if _, err := p.SetMultiple(`{}`, map[string]any{"a": 1, "b": 2}, tight); err == nil {
		t.Error("per-call MaxBatchSize=1 not enforced by SetMultiple over loose baked limit")
	}
}

func TestUnify_D007_EncodePretty_DoesNotForcePrettyOnCfg(t *testing.T) {
	p, err := New(DefaultConfig())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer p.Close()

	compact := DefaultConfig()
	compact.Pretty = false
	got, err := p.EncodePretty(map[string]any{"k": 1}, compact)
	if err != nil {
		t.Fatalf("p.EncodePretty(compact cfg): %v", err)
	}
	if strings.Contains(got, "\n") {
		t.Errorf("EncodePretty forced Pretty on an explicit cfg (historical passthrough contract): %q", got)
	}

	// No-cfg still pretty on both layers.
	if got, err := p.EncodePretty(map[string]any{"k": 1}); err != nil || !strings.Contains(got, "\n") {
		t.Errorf("EncodePretty(no cfg) not pretty: %q (err=%v)", got, err)
	}
	if got, err := EncodePretty(map[string]any{"k": 1}); err != nil || !strings.Contains(got, "\n") {
		t.Errorf("package EncodePretty(no cfg) not pretty: %q (err=%v)", got, err)
	}
}
