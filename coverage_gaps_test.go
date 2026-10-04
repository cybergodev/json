package json

// FIX-001 coverage-gap boundary tests (root package).
//
// Each test targets functions or branches the main suite leaves unexecuted
// (identified via `go tool cover -func` on the pre-change baseline):
// encoding.go (Number.UnmarshalJSON, encodeStruct stdlib branches,
// formatMapKey), types.go (AccessResult helpers), iterator_stream.go
// (sizeLimitedReader probe paths), processor_lifecycle.go (close/hook/logger
// gaps), security.go (percent-encoding bypass, unicode escape decoding,
// validation-cache eviction), processor_cache.go (invalidateCachedResult),
// processor_stats.go (truncateString), helpers.go (deepCopySliceWithDepth),
// interfaces.go (executeAfterString), encoding_schema.go (integer/enum
// semantics).

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync/atomic"
	"testing"
	"time"
	"unicode/utf8"
)

// -----------------------------------------------------------------------------
// Number.UnmarshalJSON / jsonKindOfLiteral (encoding.go)
// -----------------------------------------------------------------------------

// TestNumberUnmarshalJSONKindErrors covers the Number unmarshal contract: the
// raw literal is stored verbatim, null is a no-op, and non-number literals
// fail with an encoding/json-style type error naming the actual JSON kind.
func TestNumberUnmarshalJSONKindErrors(t *testing.T) {
	valid := []struct {
		input string
		want  Number
	}{
		{"42", "42"},
		{"-1.5e3", "-1.5e3"},
		{"0.10", "0.10"},
		{"null", ""},
	}
	for _, tc := range valid {
		var n Number
		if err := json.Unmarshal([]byte(tc.input), &n); err != nil {
			t.Errorf("Unmarshal(%s): %v", tc.input, err)
			continue
		}
		if n != tc.want {
			t.Errorf("Unmarshal(%s) = %q, want %q", tc.input, n, tc.want)
		}
	}

	invalid := []struct {
		input      string
		wantKindIn string // substring expected in the error, e.g. "string"
	}{
		{`"s"`, "string"},
		{`{}`, "object"},
		{`[]`, "array"},
		{`true`, "bool"},
		{`nan`, ""}, // invalid number literal, any message
		{`12x`, ""},
	}
	for _, tc := range invalid {
		var n Number
		err := json.Unmarshal([]byte(tc.input), &n)
		if err == nil {
			t.Errorf("Unmarshal(%s): expected error, got Number(%q)", tc.input, n)
			continue
		}
		if tc.wantKindIn != "" && !strings.Contains(err.Error(), tc.wantKindIn) {
			t.Errorf("Unmarshal(%s) error %q does not mention kind %q", tc.input, err.Error(), tc.wantKindIn)
		}
	}

	// Direct jsonKindOfLiteral branch coverage (empty + null + number kinds).
	if got := jsonKindOfLiteral(nil); got != "value" {
		t.Errorf("jsonKindOfLiteral(empty) = %q, want value", got)
	}
	if got := jsonKindOfLiteral([]byte("null")); got != "null" {
		t.Errorf("jsonKindOfLiteral(null) = %q, want null", got)
	}
	if got := jsonKindOfLiteral([]byte("-1")); got != "number" {
		t.Errorf("jsonKindOfLiteral(-1) = %q, want number", got)
	}
}

// -----------------------------------------------------------------------------
// AccessResult.Ok / Unwrap / UnwrapOr (types.go)
// -----------------------------------------------------------------------------

// TestAccessResultOkUnwrapUnwrapOr covers the three plain accessors that had
// no coverage, plus their interaction with a real SafeGet result.
func TestAccessResultOkUnwrapUnwrapOr(t *testing.T) {
	p, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()

	hit := p.SafeGet(`{"a":1}`, "a")
	if !hit.Ok() || hit.Unwrap() != float64(1) || hit.UnwrapOr(9) != float64(1) {
		t.Errorf("hit: Ok=%v Unwrap=%v UnwrapOr=%v", hit.Ok(), hit.Unwrap(), hit.UnwrapOr(9))
	}

	miss := p.SafeGet(`{"a":1}`, "b")
	if miss.Ok() || miss.Unwrap() != nil || miss.UnwrapOr(9) != 9 {
		t.Errorf("miss: Ok=%v Unwrap=%v UnwrapOr=%v", miss.Ok(), miss.Unwrap(), miss.UnwrapOr(9))
	}

	// Zero-value AccessResult behaves as a miss.
	var zero AccessResult
	if zero.Ok() || zero.Unwrap() != nil || zero.UnwrapOr("d") != "d" {
		t.Error("zero AccessResult must behave as a miss")
	}
}

// -----------------------------------------------------------------------------
// sizeLimitedReader probe paths + streamMaxSize (iterator_stream.go)
// -----------------------------------------------------------------------------

// flakyReader returns (0, nil) up to flakeCount times before delegating,
// exercising the sizeLimitedReader probe retry loop (io.Reader contract
// permits (0, nil)).
type flakyReader struct {
	r          bytes.Reader
	flakeCount int32
}

func (f *flakyReader) Read(p []byte) (int, error) {
	if atomic.AddInt32(&f.flakeCount, -1) >= 0 {
		return 0, nil
	}
	return f.r.Read(p)
}

// errReader always fails with a fixed error, exercising the non-EOF probe
// error branch.
type errReader struct{ err error }

func (e errReader) Read([]byte) (int, error) { return 0, e.err }

func smallStreamCfg(max int64) Config {
	cfg := DefaultConfig()
	cfg.MaxJSONSize = max
	return cfg
}

// collectStream drains a StreamIterator, returning the error it terminated
// with.
func collectStream(si *StreamIterator) error {
	for si.Next() {
	}
	return si.Err()
}

func TestStreamIteratorSizeLimitBoundaries(t *testing.T) {
	const doc = `[1,2,3,4,5]` // 15 bytes

	t.Run("exactly at limit with clean EOF is legal", func(t *testing.T) {
		si := NewStreamIterator(strings.NewReader(doc), smallStreamCfg(int64(len(doc))))
		if err := collectStream(si); err != nil {
			t.Fatalf("exactly-at-limit stream failed: %v", err)
		}
	})

	t.Run("content beyond the limit is rejected", func(t *testing.T) {
		longer := `[1,2,3,4,5,6]` // 18 bytes: the decoder must read past byte 15
		si := NewStreamIterator(strings.NewReader(longer), smallStreamCfg(int64(len(doc))))
		err := collectStream(si)
		if !errors.Is(err, ErrSizeLimit) {
			t.Fatalf("over-limit stream: got %v, want ErrSizeLimit", err)
		}
	})

	t.Run("probe retries on zero-length reads", func(t *testing.T) {
		fr := &flakyReader{r: *bytes.NewReader([]byte(doc)), flakeCount: 3}
		si := NewStreamIterator(fr, smallStreamCfg(int64(len(doc))))
		if err := collectStream(si); err != nil {
			t.Fatalf("flaky exactly-at-limit stream failed: %v", err)
		}
	})

	t.Run("underlying non-EOF error propagates", func(t *testing.T) {
		custom := errors.New("boom")
		si := NewStreamIterator(errReader{custom}, smallStreamCfg(1024))
		if err := collectStream(si); !errors.Is(err, custom) {
			t.Fatalf("got %v, want underlying boom", err)
		}
	})

	t.Run("streamMaxSize defaults and overrides", func(t *testing.T) {
		if got := streamMaxSize(); got != int64(DefaultMaxJSONSize) {
			t.Errorf("streamMaxSize() = %d, want DefaultMaxJSONSize=%d", got, DefaultMaxJSONSize)
		}
		if got := streamMaxSize(smallStreamCfg(42)); got != 42 {
			t.Errorf("streamMaxSize(cfg{42}) = %d, want 42", got)
		}
	})
}

// TestStreamObjectIteratorEmptyInput covers the first-Token error path: an
// empty document surfaces io.EOF through Err() instead of a silent clean end.
func TestStreamObjectIteratorEmptyInput(t *testing.T) {
	soi := NewStreamObjectIterator(strings.NewReader(""))
	if soi.Next() {
		t.Fatal("Next on empty input returned true")
	}
	if soi.Err() == nil {
		t.Error("empty input must surface an error (D-002), got nil")
	}
}

// -----------------------------------------------------------------------------
// Processor lifecycle gaps (processor_lifecycle.go)
// -----------------------------------------------------------------------------

// TestWaitForActiveOpsPaths covers both waitForActiveOps exits directly:
// immediate success with no active ops, and the timeout elapse.
func TestWaitForActiveOpsPaths(t *testing.T) {
	p, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()

	if !p.waitForActiveOps(time.Second) {
		t.Error("no active ops: expected immediate true")
	}

	atomic.AddInt64(&p.activeOps, 1)
	start := time.Now()
	if p.waitForActiveOps(20 * time.Millisecond) {
		t.Error("active op held: expected timeout false")
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Errorf("waitForActiveOps ignored its timeout (took %v)", elapsed)
	}
	atomic.AddInt64(&p.activeOps, -1)
	if !p.waitForActiveOps(time.Second) {
		t.Error("after release: expected true")
	}
}

// TestFinishDeferredTeardownOnceGuard verifies the deferred-teardown path is
// guarded by sync.Once: calling it after a normal Close is a safe no-op.
func TestFinishDeferredTeardownOnceGuard(t *testing.T) {
	p, err := New()
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Close(); err != nil {
		t.Fatal(err)
	}
	p.finishDeferredTeardown() // must not panic or double-release
	p.finishDeferredTeardown()
	if !p.IsClosed() {
		t.Error("processor must remain closed after deferred teardown")
	}
}

// TestSnapshotHooksLifecycle covers snapshotHooks across the nil-receiver,
// no-hooks, hooked, and post-Close states, plus hooksForOptions merging of
// per-call cfg.Hooks with installed hooks.
func TestSnapshotHooksLifecycle(t *testing.T) {
	var nilP *Processor
	if nilP.snapshotHooks() != nil {
		t.Error("nil receiver snapshotHooks must return nil")
	}

	p, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()

	if p.snapshotHooks() != nil {
		t.Error("no hooks installed: snapshot must be nil")
	}

	rec := &countingHook{}
	p.AddHook(rec)

	hc := p.snapshotHooks()
	if len(hc) != 1 {
		t.Fatalf("snapshot len = %d, want 1", len(hc))
	}

	// hooksForOptions: nil options → processor chain only.
	if got := len(p.hooksForOptions(nil)); got != 1 {
		t.Errorf("hooksForOptions(nil) len = %d, want 1", got)
	}
	// Per-call hooks are appended after processor hooks (D-002 semantics).
	perCall := &Config{Hooks: []Hook{&HookFunc{}}}
	merged := p.hooksForOptions(perCall)
	if len(merged) != 2 {
		t.Errorf("hooksForOptions(perCall) len = %d, want 2", len(merged))
	}

	// Close clears hooks: snapshots become empty again.
	if err := p.Close(); err != nil {
		t.Fatal(err)
	}
	if p.snapshotHooks() != nil {
		t.Error("post-Close snapshot must be nil")
	}
}

// countingHook is a minimal Hook implementation for lifecycle tests.
type countingHook struct{ calls int }

func (h *countingHook) Before(HookContext) error { h.calls++; return nil }

func (h *countingHook) After(ctx HookContext, result any, err error) (any, error) {
	h.calls++
	return result, err
}

// TestSetLoggerWiring covers both SetLogger branches (nil → default, custom →
// stored) by capturing output through a text handler.
func TestSetLoggerWiring(t *testing.T) {
	p, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()

	var buf bytes.Buffer
	p.SetLogger(slog.New(slog.NewTextHandler(&buf, nil)))
	p.getLogger().Info("greeting")
	if out := buf.String(); !strings.Contains(out, "greeting") || !strings.Contains(out, "json-processor") {
		t.Errorf("custom logger output missing msg/component: %q", out)
	}

	buf.Reset()
	p.SetLogger(nil) // nil restores the default logger
	p.getLogger().Info("dropped")
	if buf.Len() != 0 {
		t.Errorf("nil SetLogger must detach the custom logger, but buffer got %q", buf.String())
	}
}

// -----------------------------------------------------------------------------
// Security escape helpers (security.go)
// -----------------------------------------------------------------------------

// TestContainsPercentEncodingBypassTable covers every pattern arm of the
// percent-encoding traversal detector, including double-encoded forms and
// benign percent uses.
func TestContainsPercentEncodingBypassTable(t *testing.T) {
	cases := []struct {
		input string
		want  bool
	}{
		{"", false},
		{"plain/path", false},
		{"50%", false},      // trailing percent, no hex
		{"%20", false},      // space encoding is legal
		{"%2g", false},      // invalid hex
		{"%252g", false},    // double-encoded invalid hex
		{"a%2", false},      // truncated
		{"%2e", true},       // encoded dot
		{"x%2E/File", true}, // uppercase encoded slash
		{"%2fetc%2fpasswd", true},
		{"%5Cwindows", true}, // encoded backslash (both cases)
		{"%5cwin", true},
		{"%00", true},        // null byte
		{"%252e%252e", true}, // double-encoded dot
		{"%252F", true},      // double-encoded slash, uppercase
		{"prefix%252finject", true},
	}
	for _, tc := range cases {
		if got := containsPercentEncodingBypass(tc.input); got != tc.want {
			t.Errorf("containsPercentEncodingBypass(%q) = %v, want %v", tc.input, got, tc.want)
		}
	}
}

// TestContainsZeroWidthAndOverlong covers the invisible-character and
// overlong-encoding detectors used by path security. Inputs are built from
// runes because zero-width characters cannot appear literally in Go source.
func TestContainsZeroWidthAndOverlong(t *testing.T) {
	zw := func(r rune) string { return "a" + string(r) + "b" }
	zeroWidth := []string{
		zw(0x200B), // zero-width space
		zw(0x200D), // zero-width joiner
		zw(0xFEFF), // byte order mark
		zw(0x00AD), // soft hyphen
		string(rune(0x2060)) + "join",
	}
	for _, s := range zeroWidth {
		if !containsZeroWidthChars(s) {
			t.Errorf("containsZeroWidthChars(%q) = false, want true", s)
		}
	}
	for _, s := range []string{"", "plain", "abc"} {
		if containsZeroWidthChars(s) {
			t.Errorf("containsZeroWidthChars(%q) = true, want false", s)
		}
	}

	// The detector targets the canonical overlong sequences %c0%af and %c1%9c
	// (both hex cases); other sequences — even structurally similar ones — pass.
	overlong := []string{"%c0%af", "%C0%AF", "%c1%9c", "%C1%9C", "x%c0%afy"}
	for _, s := range overlong {
		if !containsOverlongEncoding(s) {
			t.Errorf("containsOverlongEncoding(%q) = false, want true", s)
		}
	}
	for _, s := range []string{"", "%C1%BF", "%c0%ag", "%e0%80%80", "%c2%80"} {
		if containsOverlongEncoding(s) {
			t.Errorf("containsOverlongEncoding(%q) = true, want false", s)
		}
	}
}

// TestUnicodeEscapeDecoding covers decodeJSONUnicodeEscape (BMP, surrogate
// pairs, lone surrogates, invalid hex) via its safe caller. The backslash is
// built from a rune so the test source cannot be mangled by editors.
func TestUnicodeEscapeDecoding(t *testing.T) {
	bs := string(rune(92)) // backslash
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{"plain passthrough", "aAb", "aAb"},
		{"bmp escape", bs + "u0041", "A"},
		{"escape inside text", "z" + bs + "u0041", "zA"},
		{"surrogate pair", bs + "ud83d" + bs + "ude00", string(rune(0x1F600))},
		{"lone high surrogate", bs + "ud800x", string(utf8.RuneError) + "x"},
		{"invalid hex kept verbatim", bs + "uzzzz", bs + "uzzzz"},
		{"truncated escape kept verbatim", "a" + bs + "u12", "a" + bs + "u12"},
		{"backslash without u", bs + "n", bs + "n"},
	}
	for _, tc := range cases {
		if got := normalizeJSONEscapes(tc.input); got != tc.want {
			t.Errorf("%s: normalizeJSONEscapes(%q) = %q, want %q", tc.name, tc.input, got, tc.want)
		}
	}
}

func TestValidationCacheLRUEviction(t *testing.T) {
	p, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()

	sv := p.securityValidator
	if sv == nil {
		t.Fatal("processor has no security validator")
	}

	// Distinct valid inputs, each small enough to qualify for caching
	// (validationCacheMaxInputSize = 256KB).
	const inserts = securityCacheHighWatermark + 300
	for i := range inserts {
		input := fmt.Sprintf(`{"evict":%d,"pad":"%06d"}`, i, i)
		if err := sv.ValidateJSONInput(input); err != nil {
			t.Fatalf("input %d unexpectedly rejected: %v", i, err)
		}
	}

	sv.cacheMutex.Lock()
	size := len(sv.validationCache)
	sv.cacheMutex.Unlock()

	if size > inserts {
		t.Errorf("cache size %d exceeds inserts %d", size, inserts)
	}
	if size >= inserts {
		t.Errorf("cache size %d == inserts; eviction never ran (want <= ~75%% of %d)", size, inserts)
	}
	if size < securityCacheHighWatermark/2 {
		t.Errorf("cache size %d collapsed below half the watermark; eviction too aggressive", size)
	}
}

// -----------------------------------------------------------------------------
// Pattern registry Len + restore contract (security.go)
// -----------------------------------------------------------------------------

// TestPatternRegistryLenAndRestore covers registry.Len() and verifies the
// save/clear/restore cycle used by registry tests leaves defaults intact.
func TestPatternRegistryLenAndRestore(t *testing.T) {
	clearDangerousPatterns()
	if got := globalPatternRegistry.Len(); got != 0 {
		t.Fatalf("after Clear: Len = %d, want 0", got)
	}

	// The registry starts empty by default (built-ins live in the packaged
	// dangerousPatterns slice, not the runtime registry), so registering the
	// defaults is the documented way to repopulate it.
	for _, p := range getDefaultPatterns() {
		RegisterDangerousPattern(p)
	}
	if got, want := globalPatternRegistry.Len(), len(getDefaultPatterns()); got != want || want == 0 {
		t.Errorf("after restore: Len = %d, want %d (and defaults must be non-empty)", got, want)
	}
}

// -----------------------------------------------------------------------------
// Processor cache invalidation + truncateString (processor_cache.go / processor_stats.go)
// -----------------------------------------------------------------------------

// TestInvalidateCachedResult covers the cache-invalidation helper and the
// disabled-cache no-op guard.
func TestInvalidateCachedResult(t *testing.T) {
	cfg := DefaultConfig()
	cfg.EnableCache = true
	p, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()

	p.setCachedResultInternal("invalidate/me", "v")
	if v, ok := p.getCachedResult("invalidate/me"); !ok || v != "v" {
		t.Fatalf("seed failed: got (%v, %v)", v, ok)
	}

	p.invalidateCachedResult("invalidate/me")
	if _, ok := p.getCachedResult("invalidate/me"); ok {
		t.Error("entry survived invalidateCachedResult")
	}

	// No-op when caching is disabled.
	cfg2 := DefaultConfig()
	cfg2.EnableCache = false
	p2, err := New(cfg2)
	if err != nil {
		t.Fatal(err)
	}
	defer p2.Close()
	p2.setCachedResultInternal("k", "v") // must be dropped
	p2.invalidateCachedResult("k")       // must not panic
	if _, ok := p2.getCachedResult("k"); ok {
		t.Error("disabled cache returned an entry")
	}
}

// TestTruncateString covers all three branches of the ellipsis truncater.
func TestTruncateString(t *testing.T) {
	cases := []struct {
		s      string
		maxLen int
		want   string
	}{
		{"hello", 10, "hello"}, // under limit
		{"hello", 5, "hello"},  // exactly at limit
		{"abcdef", 3, "abc"},   // maxLen <= 3: hard cut
		{"abcdef", 2, "ab"},
		{"abcdefgh", 6, "abc..."}, // room for ellipsis
		{"abcdefgh", 0, ""},
	}
	for _, tc := range cases {
		if got := truncateString(tc.s, tc.maxLen); got != tc.want {
			t.Errorf("truncateString(%q, %d) = %q, want %q", tc.s, tc.maxLen, got, tc.want)
		}
	}
}

// -----------------------------------------------------------------------------
// deepCopySliceWithDepth (helpers.go)
// -----------------------------------------------------------------------------

// TestDeepCopySliceWithDepth covers isolation (mutating the copy leaves the
// original untouched) and the depth-limit error branch.
func TestDeepCopySliceWithDepth(t *testing.T) {
	original := []any{
		map[string]any{"k": "v"},
		[]any{1.0, 2.0},
		"scalar",
	}
	copied, err := deepCopySliceWithDepth(original, 0)
	if err != nil {
		t.Fatal(err)
	}
	copied[0].(map[string]any)["k"] = "mutated"
	copied[1].([]any)[0] = 99.0
	if original[0].(map[string]any)["k"] != "v" || original[1].([]any)[0] != 1.0 {
		t.Error("mutating the copy leaked into the original")
	}

	if _, err := deepCopySliceWithDepth(original, deepCopyMaxDepth+1); err == nil {
		t.Error("depth beyond deepCopyMaxDepth: expected error")
	}
}

// -----------------------------------------------------------------------------
// executeAfterString (interfaces.go)
// -----------------------------------------------------------------------------

// afterStringHook records calls and controls its After return.
type afterStringHook struct {
	returnVal any
	returnErr error
}

func (h *afterStringHook) Before(HookContext) error { return nil }

func (h *afterStringHook) After(_ HookContext, result any, err error) (any, error) {
	if h.returnErr != nil {
		return result, h.returnErr
	}
	if h.returnVal != nil {
		return h.returnVal, err
	}
	return result, err
}

// TestExecuteAfterStringHookResultCoercion covers the string-result coercion:
// a string-transforming hook applies, a non-string result keeps the original,
// and errors still propagate.
func TestExecuteAfterStringHookResultCoercion(t *testing.T) {
	ctx := HookContext{Operation: "set"}

	hc := hookChain{&afterStringHook{returnVal: "transformed"}}
	if got, err := hc.executeAfterString(ctx, "original", nil); err != nil || got != "transformed" {
		t.Errorf("string transform: got (%q, %v)", got, err)
	}

	hc = hookChain{&afterStringHook{returnVal: 123}} // non-string: no-op on result
	if got, err := hc.executeAfterString(ctx, "original", nil); err != nil || got != "original" {
		t.Errorf("non-string result: got (%q, %v), want original kept", got, err)
	}

	custom := errors.New("hook-failed")
	hc = hookChain{&afterStringHook{returnErr: custom}}
	if got, err := hc.executeAfterString(ctx, "original", nil); err == nil || !errors.Is(err, custom) || got != "original" {
		t.Errorf("error path: got (%q, %v)", got, err)
	}
}

// -----------------------------------------------------------------------------
// encodeStruct stdlib branches + formatMapKey (encoding.go)
// -----------------------------------------------------------------------------

type gapStdlibStruct struct {
	Name string  `json:"name"`
	N    int     `json:"n"`
	F    float64 `json:"f"`
	Ptr  *int    `json:"ptr"`
}

// TestEncodeStructStdlibBranches reaches customEncoder.encodeStruct's stdlib
// delegation: PreserveNumbers forces the custom encoder (needsCustomEncodingOpts)
// but is NOT one of encodeStruct's own routing conditions, so the struct falls
// through to json.Marshal / MarshalIndent — pinned to byte parity with stdlib.
func TestEncodeStructStdlibBranches(t *testing.T) {
	v := gapStdlibStruct{Name: "x", N: 1, F: 2.5}
	seven := 7
	v.Ptr = &seven

	compactCfg := DefaultConfig()
	compactCfg.PreserveNumbers = true // custom encoder, stdlib struct branch
	pc, err := New(compactCfg)
	if err != nil {
		t.Fatal(err)
	}
	defer pc.Close()

	wantC, stdErr := json.Marshal(v)
	gotC, err := pc.Encode(v)
	if err != nil || stdErr != nil {
		t.Fatalf("compact encode: %v (stdlib: %v)", err, stdErr)
	}
	if gotC != string(wantC) {
		t.Errorf("compact: got %s want %s", gotC, wantC)
	}

	prettyCfg := PrettyConfig()
	prettyCfg.PreserveNumbers = true // custom encoder, stdlib MarshalIndent branch
	pp, err := New(prettyCfg)
	if err != nil {
		t.Fatal(err)
	}
	defer pp.Close()

	wantP, _ := json.MarshalIndent(v, prettyCfg.Prefix, prettyCfg.Indent)
	gotP, err := pp.Encode(v)
	if err != nil {
		t.Fatal(err)
	}
	if gotP != string(wantP) {
		t.Errorf("pretty:\ngot  %s\nwant %s", gotP, wantP)
	}

	// Marshal-error branch: a channel field is unmarshalable by encoding/json.
	type badStruct struct {
		C chan int `json:"c"`
	}
	_, err = pc.Encode(badStruct{C: make(chan int)})
	if err == nil {
		t.Error("channel field: expected marshal error to propagate")
	}
}

// gapTextKey implements encoding.TextMarshaler for map-key formatting.
type gapTextKey struct{ V string }

func (k gapTextKey) MarshalText() ([]byte, error) { return []byte(k.V), nil }

// TestFormatMapKey covers the map-key formatter: string, signed, unsigned,
// bool, interface-wrapped, and TextMarshaler keys. Non-string keys are a
// library extension over encoding/json (which rejects them); they are only
// reachable through the custom encoder, so SortKeys pins that route.
func TestFormatMapKey(t *testing.T) {
	cfg := DefaultConfig()
	cfg.SortKeys = true
	p, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()

	cases := []struct {
		name  string
		value any
		want  string
	}{
		{"int keys", map[int]string{1: "a"}, `{"1":"a"}`},
		{"negative int keys", map[int8]string{-2: "b"}, `{"-2":"b"}`},
		{"uint keys", map[uint]string{3: "c"}, `{"3":"c"}`},
		{"bool keys", map[bool]string{true: "t", false: "f"}, `{"false":"f","true":"t"}`},
		{"text marshaler keys", map[gapTextKey]string{{V: "k1"}: "v"}, `{"k1":"v"}`},
		{"interface-wrapped int key", map[any]string{1: "x"}, `{"1":"x"}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := p.Encode(tc.value)
			if err != nil {
				t.Fatalf("Encode: %v", err)
			}
			if got != tc.want {
				t.Errorf("Encode = %s, want %s", got, tc.want)
			}
		})
	}
}

// -----------------------------------------------------------------------------
// Schema validation: integer semantics + numeric enum matching (encoding_schema.go)
// -----------------------------------------------------------------------------

// TestSchemaIntegerSemantics pins JSON-Schema integer semantics: 1.0 and 1e2
// are integers, 1.5 is not, and Number literals are judged by value.
func TestSchemaIntegerSemantics(t *testing.T) {
	p, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()

	schema := NewSchema(SchemaConfig{Type: "integer"})

	cases := []struct {
		json    string
		wantErr bool
	}{
		{`1`, false},
		{`1.0`, false},
		{`1e2`, false},
		{`-3`, false},
		{`1.5`, true},
		{`"1"`, true},
		{`true`, true},
		{`null`, true},
		{`[]`, true},
	}
	for _, tc := range cases {
		errs, err := p.ValidateSchema(tc.json, schema)
		if err != nil {
			t.Fatalf("ValidateSchema(%s): %v", tc.json, err)
		}
		if got := len(errs) > 0; got != tc.wantErr {
			t.Errorf("ValidateSchema(%s) errors=%v, wantErr=%v (%v)", tc.json, errs, tc.wantErr, errs)
		}
	}

	// PreserveNumbers: the same literals arrive as Number and must be judged
	// by numeric value, not by literal shape.
	cfg := DefaultConfig()
	cfg.PreserveNumbers = true
	pn, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer pn.Close()

	for _, tc := range []struct {
		json    string
		wantErr bool
	}{
		{`2.0`, false},
		{`2e1`, false},
		{`2.5`, true},
	} {
		errs, err := pn.ValidateSchema(tc.json, schema)
		if err != nil {
			t.Fatalf("ValidateSchema(%s): %v", tc.json, err)
		}
		if got := len(errs) > 0; got != tc.wantErr {
			t.Errorf("PreserveNumbers ValidateSchema(%s) errors=%v, wantErr=%v", tc.json, errs, tc.wantErr)
		}
	}
}

// TestSchemaEnumNumericMatching covers valuesEqual/numericAsFloat with
// Go-native enum members (int/uint/float32) matched against JSON numbers.
func TestSchemaEnumNumericMatching(t *testing.T) {
	p, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()

	schema := NewSchema(SchemaConfig{Enum: []any{
		7, uint(9), float32(2.5), "text", nil, true,
		// Wider Go-native numeric kinds exercise numericAsFloat's full switch.
		int8(1), int16(2), int32(3), int64(4),
		uint8(5), uint16(6), uint32(7), uint64(8),
	}})

	valid := []string{`7`, `9`, `2.5`, `"text"`, `null`, `true`,
		`1`, `2`, `3`, `4`, `5`, `6`, `8`}
	for _, in := range valid {
		errs, err := p.ValidateSchema(in, schema)
		if err != nil {
			t.Fatalf("ValidateSchema(%s): %v", in, err)
		}
		if len(errs) != 0 {
			t.Errorf("ValidateSchema(%s) = %v, want match", in, errs)
		}
	}

	invalid := []string{`2.6`, `"other"`, `false`, `[]`, `{}`}
	for _, in := range invalid {
		errs, err := p.ValidateSchema(in, schema)
		if err != nil {
			t.Fatalf("ValidateSchema(%s): %v", in, err)
		}
		if len(errs) == 0 {
			t.Errorf("ValidateSchema(%s) matched, want mismatch", in)
		}
	}
}
