package json

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

// ============================================================================
// FIX-001 boundary tests: previously uncovered error branches and edge cases.
// Each test targets functions that coverage showed at <80%.
// ============================================================================

// TestValidateJSONInputEssential_Boundaries covers every failure branch of
// ValidateJSONInputEssential via a validator with tiny limits: size limit,
// empty input, nesting depth, and per-container counts.
func TestValidateJSONInputEssential_Boundaries(t *testing.T) {
	sv := newSecurityValidator(32, 100, 5, false, false, false, nil, 3, 3)
	defer sv.Close()

	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"valid compact object", `{"a":1,"b":2}`, false},
		{"size limit exceeded", `{"padding":"0123456789012345678901234567890"}`, true},
		{"empty input", ``, true},
		{"nesting within limit", `[[[[[1]]]]]`, false},
		{"nesting exceeds limit", `[[[[[[1]]]]]]`, true},
		{"object keys within limit", `{"a":1,"b":2,"c":3}`, false},
		{"object keys exceed limit", `{"a":1,"b":2,"c":3,"d":4}`, true},
		{"array elements within limit", `[1,2,3]`, false},
		{"array elements exceed limit", `[1,2,3,4]`, true},
		{"brackets inside strings are not containers", `{"k":"[[[[[[[[[[[[[["}`, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := sv.ValidateJSONInputEssential(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateJSONInputEssential(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}

// TestValidateNumber_Boundaries drives validateNumber through ValidateSchema
// across each range constraint, then directly across every numeric Go kind
// (ValidateSchema only ever sees float64 after JSON parsing).
func TestValidateNumber_Boundaries(t *testing.T) {
	p, err := New()
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}

	min, max, multipleOf := 5.0, 10.0, 2.5
	exclusive := true
	build := func(mutate func(s *Schema)) *Schema {
		s := NewSchemaWithConfig(SchemaConfig{
			Type:             "number",
			Minimum:          &min,
			Maximum:          &max,
			MultipleOf:       &multipleOf,
			ExclusiveMinimum: &exclusive,
			ExclusiveMaximum: &exclusive,
		})
		if mutate != nil {
			mutate(s)
		}
		return s
	}

	tests := []struct {
		name     string
		schema   *Schema
		value    string
		wantErrs int
	}{
		{"in range and multiple", build(func(s *Schema) { s.ExclusiveMinimum = false; s.ExclusiveMaximum = false }), `7.5`, 0},
		{"below minimum", build(nil), `2.5`, 1},
		{"above maximum", build(nil), `12.5`, 1},
		{"not a multiple", build(func(s *Schema) { s.ExclusiveMinimum = false; s.ExclusiveMaximum = false }), `6`, 1},
		{"at inclusive minimum", build(func(s *Schema) { s.ExclusiveMinimum = false; s.ExclusiveMaximum = false }), `5`, 0},
		{"exclusive minimum rejects boundary", build(nil), `5`, 1},
		{"exclusive minimum accepts above", build(nil), `7.5`, 0},
		{"exclusive maximum rejects boundary", build(nil), `10`, 1},
		{"exclusive maximum accepts below", build(nil), `7.5`, 0},
		{"non-number value fails the type check", build(nil), `"text"`, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs, err := p.ValidateSchema(tt.value, tt.schema)
			if err != nil {
				t.Fatalf("ValidateSchema failed: %v", err)
			}
			if len(errs) != tt.wantErrs {
				t.Errorf("ValidateSchema(%s) returned %d errors (%v), want %d", tt.value, len(errs), errs, tt.wantErrs)
			}
		})
	}

	// Every numeric kind must reach the range check (a skipped kind would
	// silently skip its constraints for that type). A minimum-only schema
	// keeps the expected error count at exactly one.
	direct := NewSchemaWithConfig(SchemaConfig{Type: "number", Minimum: &min})
	for _, value := range []any{
		int(2), int8(2), int16(2), int32(2), int64(2),
		uint(2), uint8(2), uint16(2), uint32(2), uint64(2),
		float32(2.5), float64(2.5),
	} {
		var errs []ValidationError
		p.validateNumber(value, direct, "num", &errs)
		if len(errs) != 1 {
			t.Errorf("validateNumber(%T): got %d errors (%v), want 1 minimum violation", value, len(errs), errs)
		}
	}
	var errs []ValidationError
	p.validateNumber("not a number", direct, "num", &errs)
	if len(errs) != 0 {
		t.Errorf("validateNumber(string) should be a no-op, got %v", errs)
	}
}

// TestValidateEmailFormat_Boundaries covers each rejection branch of
// validateEmailFormat through ValidateSchema with Format "email".
func TestValidateEmailFormat_Boundaries(t *testing.T) {
	p, err := New()
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	emailSchema := NewSchemaWithConfig(SchemaConfig{Type: "string", Format: "email"})
	schema := NewSchemaWithConfig(SchemaConfig{
		Type:       "object",
		Properties: map[string]*Schema{"email": emailSchema},
	})

	tests := []struct {
		name    string
		email   string
		wantErr bool
	}{
		{"valid email", "user@example.com", false},
		{"exceeds 254 char limit", strings.Repeat("u", 250) + "@e.com", true},
		{"missing at", "user.example.com", true},
		{"empty local part", "@example.com", true},
		{"empty domain", "user@", true},
		{"local part exceeds 64 chars", strings.Repeat("u", 65) + "@example.com", true},
		{"consecutive dots in local part", "us..er@example.com", true},
		{"local part ends with dot", "user.@example.com", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc, err := json.Marshal(map[string]string{"email": tt.email})
			if err != nil {
				t.Fatal(err)
			}
			errs, err := p.ValidateSchema(string(doc), schema)
			if err != nil {
				t.Fatalf("ValidateSchema failed: %v", err)
			}
			if (len(errs) > 0) != tt.wantErr {
				t.Errorf("email %q: got %d errors (%v), wantErr %v", tt.email, len(errs), errs, tt.wantErr)
			}
		})
	}
}

// TestValidateStringFormat_Dispatch covers every Format case the
// validateStringFormat switch routes to, plus the unknown-format default
// (warn-and-pass), through the public ValidateSchema API.
func TestValidateStringFormat_Dispatch(t *testing.T) {
	p, err := New()
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	defer p.Close()

	tests := []struct {
		name    string
		format  string
		value   string
		wantErr bool
	}{
		{"date valid", "date", "2024-01-15", false},
		{"date invalid", "date", "01/15/2024", true},
		{"date-time valid", "date-time", "2024-01-15T10:30:00Z", false},
		{"date-time invalid", "date-time", "not-a-datetime", true},
		{"time valid", "time", "10:30:00", false},
		{"time invalid", "time", "10.30.00", true},
		{"uri valid", "uri", "https://example.com", false},
		{"uri invalid", "uri", "not-a-uri", true},
		{"uuid valid", "uuid", "550e8400-e29b-41d4-a716-446655440000", false},
		{"uuid invalid", "uuid", "not-a-uuid", true},
		{"ipv4 valid", "ipv4", "192.168.1.1", false},
		{"ipv4 invalid", "ipv4", "999.1.1.1", true},
		{"ipv6 valid", "ipv6", "::1", false},
		{"ipv6 invalid", "ipv6", "not-an-ip", true},
		{"unknown format passes", "mystery-format", "anything", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			schema := NewSchemaWithConfig(SchemaConfig{
				Type: "object",
				Properties: map[string]*Schema{
					"v": NewSchemaWithConfig(SchemaConfig{Type: "string", Format: tt.format}),
				},
			})
			doc, err := json.Marshal(map[string]string{"v": tt.value})
			if err != nil {
				t.Fatal(err)
			}
			errs, err := p.ValidateSchema(string(doc), schema)
			if err != nil {
				t.Fatalf("ValidateSchema failed: %v", err)
			}
			if (len(errs) > 0) != tt.wantErr {
				t.Errorf("format %s value %q: got %d errors (%v), wantErr %v", tt.format, tt.value, len(errs), errs, tt.wantErr)
			}
		})
	}
}

// TestContainsUnicodeLookalike_Table covers every lookalike class used by
// file-path validation.
func TestContainsUnicodeLookalike_Table(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"ascii clean", "normal.txt", false},
		{"empty", "", false},
		{"fullwidth dot", "file．txt", true},
		{"one dot leader", "file․txt", true},
		{"ellipsis", "file…txt", true},
		{"fullwidth slash", "dir／file", true},
		{"fullwidth backslash", "dir＼file", true},
		{"fraction slash", "dir⁄file", true},
		{"BOM", "file" + string(rune(0xFEFF)) + ".txt", true},
		{"zero width space", "file\u200b.txt", true},
		{"soft hyphen", "file\u00ad.txt", true},
		{"ideographic space", "file　.txt", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := containsUnicodeLookalike(tt.input); got != tt.want {
				t.Errorf("containsUnicodeLookalike(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

// TestValidatePathSymlinks_Boundaries covers the nonexistent-path, regular-file,
// and symlink branches. Symlink creation requires privileges on some Windows
// configurations; the symlink cases are skipped when the OS refuses.
func TestValidatePathSymlinks_Boundaries(t *testing.T) {
	dir := t.TempDir()

	if _, err := validatePathSymlinks(filepath.Join(dir, "does-not-exist")); err != nil {
		t.Errorf("nonexistent path should pass (nothing to resolve), got %v", err)
	}

	regular := filepath.Join(dir, "regular.txt")
	if err := os.WriteFile(regular, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := validatePathSymlinks(regular); err != nil {
		t.Errorf("regular file should pass, got %v", err)
	}

	link := filepath.Join(dir, "link.txt")
	err := os.Symlink(regular, link)
	if err != nil {
		t.Skipf("os.Symlink unavailable: %v", err)
	}
	if _, err := validatePathSymlinks(link); err != nil {
		t.Errorf("symlink to a regular file inside the temp dir should pass, got %v", err)
	}

	// A broken symlink cannot be resolved by EvalSymlinks.
	broken := filepath.Join(dir, "broken.txt")
	if err := os.Symlink(filepath.Join(dir, "gone.txt"), broken); err != nil {
		t.Skipf("os.Symlink unavailable: %v", err)
	}
	if _, err := validatePathSymlinks(broken); err == nil {
		t.Error("broken symlink should fail to resolve")
	}
}

// fix001Parser is a minimal PathParser used to pin the never-cache branch of
// getProcessorWithConfig for configs carrying a custom parser.
type fix001Parser struct{}

func (fix001Parser) ParsePath(path string) ([]PathSegment, error) {
	return []PathSegment{newPropertySegment(path)}, nil
}

// TestGetProcessorWithConfig_Branches covers the CustomPathParser bypass (two
// calls must never share a processor) and the stale-entry replacement (a
// closed cached processor is dropped and rebuilt).
func TestGetProcessorWithConfig_Branches(t *testing.T) {
	// Configs with a custom path parser bypass the registry entirely.
	cfg := DefaultConfig()
	cfg.CustomPathParser = fix001Parser{}
	pa, err := getProcessorWithConfig(cfg)
	if err != nil {
		t.Fatalf("getProcessorWithConfig: %v", err)
	}
	pb, err := getProcessorWithConfig(cfg)
	if err != nil {
		t.Fatalf("getProcessorWithConfig: %v", err)
	}
	if pa == pb {
		t.Error("configs with CustomPathParser must not be served from the shared cache")
	}
	_ = pa.Close()
	_ = pb.Close()

	// A closed cached entry must be replaced by a live processor.
	shared := DefaultConfig()
	shared.MaxCacheSize = 4096 // distinct cache key from other suites
	p1, err := getProcessorWithConfig(shared)
	if err != nil {
		t.Fatalf("getProcessorWithConfig: %v", err)
	}
	if err := p1.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	p2, err := getProcessorWithConfig(shared)
	if err != nil {
		t.Fatalf("getProcessorWithConfig after close: %v", err)
	}
	if p1 == p2 {
		t.Error("closed processor must not be handed out again")
	}
	if p2.IsClosed() {
		t.Error("replacement processor must be live")
	}
}

// Fix001EmbedBase and friends exercise flattenStructFields promotion rules.
type Fix001EmbedBase struct {
	Base int `json:"base"`
}

type Fix001EmbedPtr struct {
	Ptr int `json:"ptr"`
}

type fix001AnonInt int

type fix001Tags struct {
	Fix001EmbedBase                // promoted: emits "base"
	*Fix001EmbedPtr                // nil: promotes nothing
	Skip            string         `json:"-"`
	hidden          string         // unexported: skipped
	Renamed         string         `json:"renamed"`
	Omit            string         `json:"omit,omitempty"`
	AsString        int            `json:"asInt,string"`
	Anon            fix001AnonInt  `json:"anon"` // anonymous non-struct: ordinary field
	NilMap          map[string]any `json:"nilMap"`
	NilPtr          *int           `json:"nilPtr"`
}

// TestEncodeWithConfig_StructTagSemantics pins flattenStructFields and
// encodeStructCustom: field promotion, tag handling, and omitempty/null
// filtering. The no-divergence case must stay byte-identical to encoding/json.
func TestEncodeWithConfig_StructTagSemantics(t *testing.T) {
	v := fix001Tags{
		Fix001EmbedBase: Fix001EmbedBase{Base: 1},
		Skip:            "skipped",
		hidden:          "hidden",
		Renamed:         "r",
		Anon:            7,
	}

	// Default config delegates to encoding/json for structs.
	want := `{"base":1,"renamed":"r","asInt":"0","anon":7,"nilMap":null,"nilPtr":null}`
	got, err := EncodeWithConfig(v)
	if err != nil {
		t.Fatalf("EncodeWithConfig: %v", err)
	}
	if got != want {
		t.Errorf("default-path encoding:\n got %s\nwant %s", got, want)
	}

	// Any advanced option switches to encodeStructCustom; results must match
	// except where the option itself changes output (IncludeNulls here).
	custom := DefaultConfig()
	custom.FloatPrecision = 6
	gotCustom, err := EncodeWithConfig(v, custom)
	if err != nil {
		t.Fatalf("EncodeWithConfig(custom): %v", err)
	}
	if gotCustom != want {
		t.Errorf("custom-path encoding diverged:\n got %s\nwant %s", gotCustom, want)
	}

	// IncludeNulls=false drops nil pointers (but a typed nil map is not
	// interface-nil, so it survives); omitempty drops empty strings.
	noNulls := DefaultConfig()
	noNulls.FloatPrecision = 6
	noNulls.IncludeNulls = false
	gotNoNulls, err := EncodeWithConfig(v, noNulls)
	if err != nil {
		t.Fatalf("EncodeWithConfig(noNulls): %v", err)
	}
	wantNoNulls := `{"base":1,"renamed":"r","asInt":"0","anon":7,"nilMap":null}`
	if gotNoNulls != wantNoNulls {
		t.Errorf("IncludeNulls=false:\n got %s\nwant %s", gotNoNulls, wantNoNulls)
	}

	// A non-nil embedded pointer promotes its fields.
	withPtr := struct {
		Fix001EmbedBase
		*Fix001EmbedPtr
	}{Fix001EmbedBase{Base: 2}, &Fix001EmbedPtr{Ptr: 3}}
	wantPtr := `{"base":2,"ptr":3}`
	gotPtr, err := EncodeWithConfig(withPtr)
	if err != nil {
		t.Fatalf("EncodeWithConfig(ptr): %v", err)
	}
	if gotPtr != wantPtr {
		t.Errorf("pointer embedding:\n got %s\nwant %s", gotPtr, wantPtr)
	}
}

// TestEscapeRune_ConfigBranches covers escapeRune's config-dependent branches:
// tab/newline/slash escaping toggles, U+2028 handling, and CustomEscapes.
func TestEscapeRune_ConfigBranches(t *testing.T) {
	tests := []struct {
		name string
		cfg  func(*Config)
		in   string
		want string // expected encoded form of the whole input
	}{
		{"tab escaped by default", func(c *Config) {}, "a\tb", `"a\tb"`},
		{"tab raw when EscapeTabs=false", func(c *Config) { c.EscapeTabs = false }, "a\tb", "\"a\tb\""},
		{"newline escaped by default", func(c *Config) {}, "a\nb", `"a\nb"`},
		{"newline raw when EscapeNewlines=false", func(c *Config) { c.EscapeNewlines = false }, "a\nb", "\"a\nb\""},
		{"slash escaped when EscapeSlash=true", func(c *Config) { c.EscapeSlash = true }, "a/b", `"a\/b"`},
		{"slash raw by default", func(c *Config) {}, "a/b", `"a/b"`},
		{"U+2028 escaped with EscapeHTML", func(c *Config) {}, "a\u2028b", `"a\u2028b"`},
		// encoding/json escapes U+2028/U+2029 unconditionally — even under
		// SetEscapeHTML(false) — because raw JS line terminators reintroduce
		// the JSONP risk (D-002 fix; the old rows here tested "ab" without
		// any U+2028 rune and were no-ops).
		{"U+2028 escaped even without EscapeHTML", func(c *Config) { c.EscapeHTML = false }, "a\u2028b", `"a\u2028b"`},
		{"U+2029 escaped even without EscapeHTML", func(c *Config) { c.EscapeHTML = false }, "a\u2029b", `"a\u2029b"`},
		{"custom escape overrides control char", func(c *Config) {
			c.CustomEscapes = map[rune]string{'\x01': `<01>`}
		}, "a\x01b", `"a<01>b"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := DefaultConfig()
			// Advanced options route strings through the custom encoder.
			cfg.FloatPrecision = 6
			tt.cfg(&cfg)
			got, err := EncodeWithConfig(tt.in, cfg)
			if err != nil {
				t.Fatalf("EncodeWithConfig: %v", err)
			}
			if got != tt.want {
				t.Errorf("EncodeWithConfig(%q) = %s, want %s", tt.in, got, tt.want)
			}
		})
	}
}

// TestDeepCopyValueWithDepth_Types covers the type-specific fast paths and the
// marshal/unmarshal fallback, including its error branch.
func TestDeepCopyValueWithDepth_Types(t *testing.T) {
	tests := []struct {
		name  string
		value any
	}{
		{"nil", nil},
		{"json.Number", json.Number("1.5")},
		{"Number alias", Number("2.5")},
		{"map[string]string", map[string]string{"k": "v"}},
		{"[]string", []string{"a"}},
		{"[]int", []int{1}},
		{"[]float64", []float64{1.5}},
		{"[]bool", []bool{true}},
		{"struct via fallback", struct{ X int }{7}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := deepCopyValueWithDepth(tt.value, 0)
			if err != nil {
				t.Fatalf("deepCopyValueWithDepth(%v): %v", tt.value, err)
			}
			if tt.value == nil && got != nil {
				t.Errorf("nil should copy to nil, got %v", got)
			}
		})
	}

	// Unmarshalable values must surface the marshal error, not panic.
	if _, err := deepCopyValueWithDepth(make(chan int), 0); err == nil {
		t.Error("channel should fail to deep-copy via the marshal fallback")
	}

	// The depth limit must trigger instead of recursing to stack overflow.
	deep := any(map[string]any{})
	for range deepCopyMaxDepth + 5 {
		deep = map[string]any{"child": deep}
	}
	if _, err := deepCopyValueWithDepth(deep, 0); err == nil {
		t.Error("deeply nested value should exceed the copy depth limit")
	} else if !strings.Contains(err.Error(), "depth limit") {
		t.Errorf("unexpected depth error: %v", err)
	}
}

// TestDeepCopy_FastPathAndFallback covers the deepCopy dispatcher: the
// JSON-specialized fast path and its fallback for non-JSON types.
func TestDeepCopy_FastPathAndFallback(t *testing.T) {
	fast := map[string]any{"list": []any{1, "two", false}, "nested": map[string]any{"x": 1.5}}
	got, err := deepCopy(fast)
	if err != nil {
		t.Fatalf("deepCopy(fast): %v", err)
	}
	gotMap := got.(map[string]any)
	gotMap["mutated"] = true
	if _, exists := fast["mutated"]; exists {
		t.Error("deepCopy result shares state with the source map")
	}

	slow, err := deepCopy(struct{ A []int }{A: []int{1, 2}})
	if err != nil {
		t.Fatalf("deepCopy(fallback): %v", err)
	}
	if b, err := json.Marshal(slow); err != nil || string(b) != `{"A":[1,2]}` {
		t.Errorf("fallback copy marshaled to %s (err %v), want {\"A\":[1,2]}", b, err)
	}
}

// TestErrors_UnwrapErrors covers the Unwrap branch of wrapped operation
// errors against a nil and non-nil cause.
func TestErrors_UnwrapErrors(t *testing.T) {
	cause := errors.New("root cause")
	wrapped := newOperationError("fix001", "failed", cause)
	if !errors.Is(wrapped, cause) {
		t.Error("wrapped operation error should unwrap to its cause")
	}
	bare := newOperationError("fix001", "failed", nil)
	if errors.Unwrap(bare) != nil {
		t.Error("error without a cause should unwrap to nil")
	}
}

// TestHandlePropertyAccess_Types covers every branch of the legacy property
// accessor: both map key kinds, array-index access, and the struct fallback.
func TestHandlePropertyAccess_Types(t *testing.T) {
	p, err := New()
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	defer p.Close()

	tests := []struct {
		name       string
		data       any
		property   string
		want       any
		wantExists bool
	}{
		{"map hit", map[string]any{"k": 1}, "k", 1, true},
		{"map miss", map[string]any{"k": 1}, "x", nil, false},
		{"any-key map hit", map[any]any{"k": 2}, "k", 2, true},
		{"any-key map miss", map[any]any{"k": 2}, "x", nil, false},
		{"array index in range", []any{7, 8}, "1", 8, true},
		{"array index out of range", []any{7}, "5", nil, false},
		{"array non-numeric property", []any{7}, "k", nil, false},
		{"struct field", struct{ X int }{9}, "X", 9, true},
		{"struct missing field", struct{ X int }{9}, "Y", nil, false},
		{"scalar default", 42, "k", nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := p.handlePropertyAccess(tt.data, tt.property)
			if got.exists != tt.wantExists {
				t.Errorf("handlePropertyAccess(%T, %q) exists = %v, want %v", tt.data, tt.property, got.exists, tt.wantExists)
			}
			if got.value != tt.want {
				t.Errorf("handlePropertyAccess(%T, %q) value = %v, want %v", tt.data, tt.property, got.value, tt.want)
			}
		})
	}
}

// TestConvertNumbers_Types covers the number-preserving decoder's recursion:
// numbers convert, containers recurse, everything else passes through.
func TestConvertNumbers_Types(t *testing.T) {
	d := newNumberPreservingDecoder(true)

	tests := []struct {
		name  string
		input any
		want  any
	}{
		{"integer number", json.Number("42"), 42},
		{"float number", json.Number("3.5"), float64(3.5)},
		{"string passthrough", "text", "text"},
		{"bool passthrough", true, true},
		{"nil passthrough", nil, nil},
		{"nested map", map[string]any{"n": json.Number("7")}, map[string]any{"n": 7}},
		{"nested slice", []any{json.Number("1.5")}, []any{float64(1.5)}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := d.convertNumbers(tt.input); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("convertNumbers(%v) = %#v, want %#v", tt.input, got, tt.want)
			}
		})
	}
}

// TestForeach_Containers iterates objects, arrays, and empty inputs through
// the package-level Foreach.
func TestForeach_Containers(t *testing.T) {
	tests := []struct {
		name      string
		jsonStr   string
		wantCount int
	}{
		{"object", `{"a":1,"b":2}`, 2},
		{"array", `[1,2,3]`, 3},
		{"empty object", `{}`, 0},
		{"empty array", `[]`, 0},
		{"invalid JSON visits nothing", `{invalid`, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			count := 0
			Foreach(tt.jsonStr, func(key any, item *IterableValue) {
				count++
			})
			if count != tt.wantCount {
				t.Errorf("Foreach(%s) visited %d items, want %d", tt.jsonStr, count, tt.wantCount)
			}
		})
	}
}

// ============================================================================
// CORE BOUNDARY TESTS — low-coverage defensive branches.
//
// These exercise paths the happy-path tests skip so core coverage (json.go,
// config.go, types.go) moves toward the >= 90% target:
//   - JSONLWriter nil-receiver guards and writer/marshal error branches
//   - Config.Clone nil receiver + full deep-copy of every composite field
//   - NewSchemaWithConfig optional (pointer) field handling
// ============================================================================

// errMarshaler always fails JSON marshaling — drives the FastMarshalToString
// error branch in JSONLWriter.Write.
type errMarshaler struct{}

func (errMarshaler) MarshalJSON() ([]byte, error) { return nil, errors.New("marshal boom") }

// alwaysErrWriter is an io.Writer that always fails.
type alwaysErrWriter struct{}

func (alwaysErrWriter) Write(p []byte) (int, error) { return 0, errors.New("write boom") }

// failAfterNWriter succeeds until the limit-th Write call (inclusive), then
// fails. Used to reach the JSONLWriter.Write trailing-newline error branch:
// the data write succeeds but the following newline write fails.
type failAfterNWriter struct {
	calls int
	limit int
	buf   bytes.Buffer
}

func (f *failAfterNWriter) Write(p []byte) (int, error) {
	f.calls++
	if f.calls >= f.limit {
		return 0, errors.New("write boom")
	}
	return f.buf.Write(p)
}

// TestJSONLWriter_NilReceiver_Boundary exercises every nil-receiver guard in
// the JSONLWriter API. Write/WriteAll/WriteRaw must error; Err/Stats must
// return their zero values.
func TestJSONLWriter_NilReceiver_Boundary(t *testing.T) {
	var w *JSONLWriter

	if err := w.Write(map[string]any{"a": 1}); err == nil {
		t.Error("nil Write must return an error")
	}
	if err := w.WriteAll([]any{map[string]any{"a": 1}}); err == nil {
		t.Error("nil WriteAll must return an error")
	}
	if err := w.WriteRaw([]byte(`{"a":1}`)); err == nil {
		t.Error("nil WriteRaw must return an error")
	}
	if err := w.Err(); err != nil {
		t.Errorf("nil Err must be nil, got %v", err)
	}
	if s := w.Stats(); s.LinesProcessed != 0 || s.BytesWritten != 0 {
		t.Errorf("nil Stats must be zero-value, got %+v", s)
	}
}

// TestJSONLWriter_WriteErrors_Boundary covers the error branches of Write and
// WriteRaw: marshal failure, underlying-writer failure, the cached-error early
// return, and the trailing-newline write failure.
func TestJSONLWriter_WriteErrors_Boundary(t *testing.T) {
	t.Run("marshal error is stored and surfaces on retry", func(t *testing.T) {
		w := NewJSONLWriter(&bytes.Buffer{})
		if err := w.Write(errMarshaler{}); err == nil {
			t.Error("expected marshal error")
		}
		if w.Err() == nil {
			t.Error("Err() must persist the marshal failure")
		}
		// A second Write short-circuits on the cached error.
		if err := w.Write(map[string]any{"b": 2}); err == nil {
			t.Error("expected cached error on second Write")
		}
	})

	t.Run("writer error on data write", func(t *testing.T) {
		w := NewJSONLWriter(alwaysErrWriter{})
		if err := w.Write(map[string]any{"a": 1}); err == nil {
			t.Error("expected write error")
		}
		if w.Err() == nil {
			t.Error("Err() must persist the write failure")
		}
	})

	t.Run("writer error on trailing newline write", func(t *testing.T) {
		// D-002/R9 (m10): data+newline are emitted as ONE Write call, so a
		// writer failing on its first Write fails the whole line — the old
		// "data succeeded, newline failed" half-line state no longer exists.
		// (The pre-m10 form used limit:2 to fail the second of two writes.)
		w := NewJSONLWriter(&failAfterNWriter{limit: 1})
		if err := w.Write(map[string]any{"a": 1}); err == nil {
			t.Error("expected newline-write error")
		}
	})

	t.Run("WriteRaw writer error then cached error", func(t *testing.T) {
		w := NewJSONLWriter(alwaysErrWriter{})
		if err := w.WriteRaw([]byte(`{"a":1}`)); err == nil {
			t.Error("expected write error")
		}
		// Cached error short-circuits the next WriteRaw.
		if err := w.WriteRaw([]byte(`{"b":2}`)); err == nil {
			t.Error("expected cached error on second WriteRaw")
		}
	})

	t.Run("WriteAll surfaces first Write failure", func(t *testing.T) {
		w := NewJSONLWriter(alwaysErrWriter{})
		if err := w.WriteAll([]any{
			map[string]any{"a": 1},
			map[string]any{"b": 2},
		}); err == nil {
			t.Error("expected WriteAll to surface the write error")
		}
	})
}

// ----------------------------------------------------------------------------
// Config.Clone — nil receiver and full deep-copy independence.
// ----------------------------------------------------------------------------

// cloneTestEncoder/Validator/Hook are no-op implementations used only to
// populate Config composite fields so Clone's deep-copy branches run.
type cloneTestEncoder struct{}

func (cloneTestEncoder) Encode(reflect.Value) (string, error) { return "null", nil }

type cloneTestValidator struct{}

func (cloneTestValidator) Validate(string) error { return nil }

type cloneTestHook struct{}

func (cloneTestHook) Before(HookContext) error { return nil }
func (cloneTestHook) After(_ HookContext, result any, err error) (any, error) {
	return result, err
}

// TestConfigClone_DeepCopy_Boundary covers the nil-receiver branch and the
// deep-copy branches for every composite Config field, asserting that mutating
// the clone never leaks back into the original.
func TestConfigClone_DeepCopy_Boundary(t *testing.T) {
	t.Run("nil receiver returns nil", func(t *testing.T) {
		var cfg *Config
		if got := cfg.Clone(); got != nil {
			t.Errorf("nil Config.Clone must return nil, got %+v", got)
		}
	})

	t.Run("composite fields are independently copied", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.CustomTypeEncoders = map[reflect.Type]TypeEncoder{
			reflect.TypeOf(0): cloneTestEncoder{},
		}
		cfg.CustomValidators = []Validator{cloneTestValidator{}}
		cfg.AdditionalDangerousPatterns = []DangerousPattern{
			{Pattern: "evil", Name: "n", Level: PatternLevelCritical},
		}
		cfg.Hooks = []Hook{cloneTestHook{}}
		cfg.CustomEscapes = map[rune]string{'\n': "\\n"}

		clone := cfg.Clone()

		// Mutate the clone in every composite field; the original must be unaffected.
		clone.CustomTypeEncoders[reflect.TypeOf("")] = cloneTestEncoder{}
		if _, ok := cfg.CustomTypeEncoders[reflect.TypeOf("")]; ok {
			t.Error("CustomTypeEncoders must be deep-copied, not shared")
		}

		clone.CustomValidators = append(clone.CustomValidators, cloneTestValidator{})
		if len(cfg.CustomValidators) != 1 {
			t.Error("CustomValidators must be deep-copied, not shared")
		}

		clone.AdditionalDangerousPatterns[0].Pattern = "changed"
		if cfg.AdditionalDangerousPatterns[0].Pattern != "evil" {
			t.Error("AdditionalDangerousPatterns elements must be independent copies")
		}

		clone.Hooks = append(clone.Hooks, cloneTestHook{})
		if len(cfg.Hooks) != 1 {
			t.Error("Hooks must be deep-copied, not shared")
		}

		clone.CustomEscapes['\r'] = "\\r"
		if _, ok := cfg.CustomEscapes['\r']; ok {
			t.Error("CustomEscapes must be deep-copied, not shared")
		}
	})
}

// ----------------------------------------------------------------------------
// NewSchemaWithConfig — optional (pointer) fields populate value + has-flag.
// ----------------------------------------------------------------------------

// TestNewSchemaWithConfig_OptionalFields_Boundary drives every `if cfg.X != nil`
// branch in NewSchemaWithConfig so the optional-field setters run.
func TestNewSchemaWithConfig_OptionalFields_Boundary(t *testing.T) {
	intPtr := func(v int) *int { return &v }
	floatPtr := func(v float64) *float64 { return &v }
	boolPtr := func(v bool) *bool { return &v }

	s := NewSchemaWithConfig(SchemaConfig{
		MinLength:        intPtr(1),
		MaxLength:        intPtr(10),
		Minimum:          floatPtr(0),
		Maximum:          floatPtr(100),
		MinItems:         intPtr(1),
		MaxItems:         intPtr(5),
		MultipleOf:       floatPtr(2),
		ExclusiveMinimum: boolPtr(true),
		ExclusiveMaximum: boolPtr(true),
	})

	checks := []struct {
		name      string
		got       int
		want      int
		has       bool
		hasWanted bool
	}{
		{"MinLength", s.MinLength, 1, s.hasMinLength, true},
		{"MaxLength", s.MaxLength, 10, s.hasMaxLength, true},
		{"MinItems", s.MinItems, 1, s.hasMinItems, true},
		{"MaxItems", s.MaxItems, 5, s.hasMaxItems, true},
	}
	for _, c := range checks {
		if c.got != c.want {
			t.Errorf("%s = %d, want %d", c.name, c.got, c.want)
		}
		if c.has != c.hasWanted {
			t.Errorf("%s has-flag = %v, want %v", c.name, c.has, c.hasWanted)
		}
	}

	if s.Minimum != 0 || s.Maximum != 100 {
		t.Errorf("Minimum/Maximum = %v/%v, want 0/100", s.Minimum, s.Maximum)
	}
	if !s.hasMinimum || !s.hasMaximum {
		t.Error("Minimum/Maximum has-flags must be set")
	}
	// MultipleOf has no has-flag; it is set directly when provided.
	if s.MultipleOf != 2 {
		t.Errorf("MultipleOf = %v, want 2", s.MultipleOf)
	}
	if !s.ExclusiveMinimum || !s.ExclusiveMaximum {
		t.Error("ExclusiveMinimum/ExclusiveMaximum must be true")
	}

	t.Run("AdditionalProperties true/false/nil", func(t *testing.T) {
		tr, fa := true, false
		cases := []struct {
			name string
			cfg  SchemaConfig
			want bool
		}{
			{"explicit true", SchemaConfig{AdditionalProperties: &tr}, true},
			{"explicit false", SchemaConfig{AdditionalProperties: &fa}, false},
			{"nil defaults to true", SchemaConfig{AdditionalProperties: nil}, true},
		}
		for _, c := range cases {
			if got := NewSchemaWithConfig(c.cfg).AdditionalProperties; got != c.want {
				t.Errorf("%s: AdditionalProperties = %v, want %v", c.name, got, c.want)
			}
		}
	})

	t.Run("Type and Properties populate", func(t *testing.T) {
		s := NewSchemaWithConfig(SchemaConfig{
			Type:       "object",
			Properties: map[string]*Schema{"name": {Type: "string"}},
		})
		if s == nil || s.Type != "object" {
			t.Error("Type not populated")
		}
		if s.Properties["name"] == nil {
			t.Error("Properties not populated")
		}
	})
}

// ----------------------------------------------------------------------------
// Error type .Error() methods — nil-receiver guards and value branches.
// ----------------------------------------------------------------------------

// TestErrorTypes_NilReceiver_Boundary drives the `if e == nil` guard of every
// error type's Error() method. Each must return a non-empty string, not panic.
func TestErrorTypes_NilReceiver_Boundary(t *testing.T) {
	tCases := []struct {
		name string
		call func() string
	}{
		{"InvalidUnmarshalError", func() string { var e *InvalidUnmarshalError; return e.Error() }},
		{"SyntaxError", func() string { var e *SyntaxError; return e.Error() }},
		{"UnmarshalTypeError", func() string { var e *UnmarshalTypeError; return e.Error() }},
		{"UnsupportedTypeError", func() string { var e *UnsupportedTypeError; return e.Error() }},
		{"UnsupportedValueError", func() string { var e *UnsupportedValueError; return e.Error() }},
		{"MarshalerError", func() string { var e *MarshalerError; return e.Error() }},
	}
	for _, tc := range tCases {
		if got := tc.call(); got == "" {
			t.Errorf("%s: nil-receiver Error() returned empty string", tc.name)
		}
	}
}

// TestErrorTypes_Branches_Boundary covers the non-nil value branches of the
// error types (non-pointer vs pointer target, struct/field path, sourceFunc).
func TestErrorTypes_Branches_Boundary(t *testing.T) {
	t.Run("InvalidUnmarshalError nil/non-pointer/pointer Type", func(t *testing.T) {
		var ptr int
		for _, tc := range []struct {
			name string
			typ  reflect.Type
		}{
			{"nil", nil},
			{"non-pointer", reflect.TypeOf(0)},
			{"pointer", reflect.TypeOf(&ptr)},
		} {
			if got := (&InvalidUnmarshalError{Type: tc.typ}).Error(); got == "" {
				t.Errorf("%s Type: empty Error()", tc.name)
			}
		}
	})

	t.Run("UnmarshalTypeError with and without struct/field", func(t *testing.T) {
		base := &UnmarshalTypeError{Value: "number", Type: reflect.TypeOf(0)}
		if got := base.Error(); got == "" {
			t.Error("base: empty Error()")
		}
		withField := &UnmarshalTypeError{Value: "number", Type: reflect.TypeOf(0), Struct: "Foo", Field: "Bar"}
		if got := withField.Error(); !contains(got, "Foo.Bar") {
			t.Errorf("withField: Error() = %q, want struct.field path", got)
		}
	})

	t.Run("SyntaxError with message", func(t *testing.T) {
		if got := (&SyntaxError{msg: "bad token"}).Error(); got != "bad token" {
			t.Errorf("SyntaxError = %q, want %q", got, "bad token")
		}
	})

	t.Run("UnsupportedType/Value with payload", func(t *testing.T) {
		if got := (&UnsupportedTypeError{Type: reflect.TypeOf(0)}).Error(); !contains(got, "int") {
			t.Errorf("UnsupportedTypeError = %q", got)
		}
		if got := (&UnsupportedValueError{Str: "NaN"}).Error(); !contains(got, "NaN") {
			t.Errorf("UnsupportedValueError = %q", got)
		}
	})

	t.Run("MarshalerError default and explicit sourceFunc", func(t *testing.T) {
		blank := &MarshalerError{Type: reflect.TypeOf(0), Err: errors.New("boom")}
		if got := blank.Error(); !contains(got, "MarshalJSON") {
			t.Errorf("blank sourceFunc: Error() = %q, want default MarshalJSON", got)
		}
		named := &MarshalerError{Type: reflect.TypeOf(0), Err: errors.New("boom"), sourceFunc: "MarshalText"}
		if got := named.Error(); !contains(got, "MarshalText") {
			t.Errorf("explicit sourceFunc: Error() = %q, want MarshalText", got)
		}
		if u := named.Unwrap(); u == nil || u.Error() != "boom" {
			t.Errorf("MarshalerError.Unwrap = %v, want boom", u)
		}
	})
}

// contains is a tiny strings.Contains shim to avoid adding a "strings" import.
func contains(haystack, needle string) bool {
	if len(needle) == 0 {
		return true
	}
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}

// ----------------------------------------------------------------------------
// Nil-receiver guards on Config/ParsedJSON helpers.
// ----------------------------------------------------------------------------

// TestNilReceiverHelpers_Boundary covers the `if x == nil` early returns on
// small helpers that the happy-path tests never invoke with a nil receiver.
func TestNilReceiverHelpers_Boundary(t *testing.T) {
	t.Run("Config nil receivers are safe", func(t *testing.T) {
		var c *Config
		c.AddHook(cloneTestHook{})           // must not panic
		c.AddValidator(cloneTestValidator{}) // must not panic
		c.AddDangerousPattern(DangerousPattern{Pattern: "x", Name: "n", Level: PatternLevelCritical})
	})

	t.Run("ParsedJSON nil receivers are safe", func(t *testing.T) {
		var p *ParsedJSON
		if got := p.Data(); got != nil {
			t.Errorf("nil Data = %v, want nil", got)
		}
		p.Release() // must not panic
	})
}

// ----------------------------------------------------------------------------
// Processor concurrency governance — nil receiver and limit branches.
// ----------------------------------------------------------------------------

// TestProcessor_Semaphore_Boundary covers the nil-receiver and concurrency-
// limit branches of acquireSemaphore/releaseSemaphore. (p.metrics is always
// non-nil for a constructed Processor, so only these two branches are
// reachable; the metrics-nil guard is defensive.)
func TestProcessor_Semaphore_Boundary(t *testing.T) {
	t.Run("nil receiver is a no-op", func(t *testing.T) {
		var p *Processor
		if err := p.acquireSemaphore(); err != nil {
			t.Errorf("nil acquireSemaphore = %v, want nil", err)
		}
		p.releaseSemaphore() // must not panic
	})

	t.Run("MaxConcurrency saturated returns ErrConcurrencyLimit", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.MaxConcurrency = 1
		p, err := New(cfg)
		if err != nil {
			t.Fatalf("New() failed: %v", err)
		}
		defer p.Close()

		if err := p.acquireSemaphore(); err != nil {
			t.Fatalf("first acquire failed: %v", err)
		}
		defer p.releaseSemaphore()

		err = p.acquireSemaphore()
		if !errors.Is(err, ErrConcurrencyLimit) {
			t.Errorf("second acquire = %v, want ErrConcurrencyLimit", err)
		}
	})
}

// Boundary condition tests targeting low-coverage functions.
// Coverage targets: core >= 90%, utils >= 80%, overall >= 70%.

// --- GetWithContext (api.go:541, 0% coverage) ---

func TestGetWithContext_Boundary(t *testing.T) {
	t.Run("cancelled context returns error", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // cancel immediately

		_, err := GetWithContext(ctx, `{"key":"value"}`, "key")
		if err == nil {
			t.Error("expected error with cancelled context")
		}
	})

	t.Run("valid context succeeds", func(t *testing.T) {
		ctx := context.Background()
		val, err := GetWithContext(ctx, `{"key":"value"}`, "key")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if val != "value" {
			t.Errorf("val = %v, want value", val)
		}
	})

	t.Run("timeout context with valid JSON", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		val, err := GetWithContext(ctx, `{"a":1,"b":"two"}`, "b")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if val != "two" {
			t.Errorf("val = %v, want two", val)
		}
	})
}

// --- containsOverlongEncoding (security.go:1006) ---
// Detects URL-encoded overlong UTF-8 sequences used in path-traversal attacks
// (%c0%af -> overlong '/', %c1%9c -> overlong '\'). The detector scans for the
// percent-encoded form, not raw bytes.

func TestContainsOverlongEncoding_Boundary(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want bool
	}{
		{"empty", "", false},
		{"no percent sign", "normal text", false},
		{"valid URL-encoded char", "%41", false},        // 'A'
		{"valid 2-byte UTF-8 encoded", "%c3%A9", false}, // 'é' — not overlong
		{"overlong slash lowercase", "%c0%af", true},
		{"overlong slash uppercase", "%C0%AF", true},
		{"overlong backslash lowercase", "%c1%9c", true},
		{"overlong backslash uppercase", "%C1%9C", true},
		{"embedded overlong", "safe%c0%afpath", true},
		{"truncated sequence", "%c0", false},
		{"truncated after percent", "%c0%", false},
		{"percent at end", "path%", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := containsOverlongEncoding(tt.in); got != tt.want {
				t.Errorf("containsOverlongEncoding(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

// --- Integer overflow edge cases ---

func TestIntegerOverflow_Boundary(t *testing.T) {
	t.Run("large integer in JSON", func(t *testing.T) {
		jsonStr := `{"big": 9223372036854775807}`
		val, err := Get(jsonStr, "big")
		if err != nil {
			t.Fatalf("Get failed: %v", err)
		}
		// JSON numbers decode to float64; the magnitude must survive.
		f, ok := val.(float64)
		if !ok {
			t.Fatalf("expected float64 for large int, got %T (%v)", val, val)
		}
		if f <= 9e18 {
			t.Errorf("large int magnitude lost: got %v, want > 9e18", f)
		}
	})

	t.Run("large float that can't fit in int", func(t *testing.T) {
		_, ok := convertToInt(float64(math.MaxFloat64))
		if ok {
			t.Error("should not convert MaxFloat64 to int")
		}
	})
}

// --- Deep nesting edge cases ---

func TestDeepNesting_Boundary(t *testing.T) {
	t.Run("very deep nesting 50 levels", func(t *testing.T) {
		// Build 50 levels of nesting; default MaxDepth is 100, so this must succeed.
		inner := `"value"`
		for i := 0; i < 50; i++ {
			inner = `{"a":` + inner + `}`
		}
		got, err := Get(inner, strings.Repeat("a.", 49)+"a")
		if err != nil {
			t.Fatalf("Get at 50-level nesting failed: %v", err)
		}
		if got != "value" {
			t.Errorf("deep nesting result = %v, want \"value\"", got)
		}
	})

	t.Run("deep path with value assertion", func(t *testing.T) {
		deep := `{"a":{"b":{"c":{"d":{"e":"deep"}}}}}`
		got, err := Get(deep, "a.b.c.d.e")
		if err != nil {
			t.Fatalf("Get deep path failed: %v", err)
		}
		if got != "deep" {
			t.Errorf("deep path = %v, want deep", got)
		}
	})

	t.Run("chained array indices", func(t *testing.T) {
		deep := `{"a":[[[1,2],[3,4]],[[5,6],[7,8]]]}`
		got, err := Get(deep, "a[0][1][0]")
		if err != nil {
			t.Fatalf("Get chained array index failed: %v", err)
		}
		if got != 3.0 {
			t.Errorf("chained array index = %v, want 3", got)
		}
	})
}

// --- Empty/nil input edge cases ---

func TestEmptyInput_Boundary(t *testing.T) {
	t.Run("empty string Get", func(t *testing.T) {
		_, err := Get("", "key")
		if err == nil {
			t.Error("expected error for empty string")
		}
	})

	t.Run("whitespace only Set", func(t *testing.T) {
		_, err := Set("   \n\t  ", "key", "val")
		if err == nil {
			t.Error("expected error for whitespace-only input")
		}
	})

	t.Run("null value in JSON", func(t *testing.T) {
		val, err := Get(`{"a":null}`, "a")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if val != nil {
			t.Errorf("val = %v, want nil", val)
		}
	})

	t.Run("empty path returns root", func(t *testing.T) {
		result, err := Get(`{"key":"value"}`, "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		m, ok := result.(map[string]any)
		if !ok {
			t.Fatalf("expected root map, got %T", result)
		}
		if m["key"] != "value" {
			t.Errorf("root map key = %v, want value", m["key"])
		}
	})

	t.Run("Set nil value", func(t *testing.T) {
		result, err := Set(`{"key":"value"}`, "key", nil)
		if err != nil {
			t.Fatalf("Set nil value failed: %v", err)
		}
		assertJSONEqual(t, `{"key":null}`, result)
	})
}

// --- Path edge cases ---

func TestPathEdgeCases_Boundary(t *testing.T) {
	t.Run("path with dots only", func(t *testing.T) {
		_, err := Get(`{"a":1}`, "..")
		if err == nil {
			t.Error("expected error for path with only dots")
		}
	})

	t.Run("path with unicode key", func(t *testing.T) {
		val, err := Get(`{"日本語":"hello"}`, "日本語")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if val != "hello" {
			t.Errorf("val = %v, want hello", val)
		}
	})

	t.Run("large array index", func(t *testing.T) {
		// GEN-001 P0-4: an out-of-bounds index is a missing path — it returns
		// ErrPathNotFound (the errors.go contract; the compiled fast path
		// already behaved this way) instead of a silent nil.
		val, err := Get(`{"arr":[1,2,3]}`, "arr[999999]")
		if !errors.Is(err, ErrPathNotFound) {
			t.Fatalf("out-of-bounds index: err = %v, want ErrPathNotFound", err)
		}
		if val != nil {
			t.Errorf("out-of-bounds array index should return nil, got %v", val)
		}
	})

	t.Run("path with escaped bracket in key", func(t *testing.T) {
		// A literal key containing a dot must NOT be reachable via the dot-path
		// "a.b" (which denotes nested a -> b). The value 1 must not be returned.
		val, err := Get(`{"a.b":1}`, "a.b")
		if err == nil && val != nil {
			t.Errorf("dot-path should not resolve the literal dotted key, got %v", val)
		}
	})
}

// --- Encoding edge cases ---

func TestEncodingEdgeCases_Boundary(t *testing.T) {
	t.Run("encode NaN float", func(t *testing.T) {
		// NaN is not representable in JSON: it must surface as an error or "null".
		result, err := Encode(map[string]any{"val": math.NaN()}, DefaultConfig())
		if err == nil && !strings.Contains(result, "null") {
			t.Errorf("NaN must surface as an error or null, got %q", result)
		}
	})

	t.Run("encode Infinity float", func(t *testing.T) {
		// +/-Inf is not representable in JSON: it must surface as an error or "null".
		result, err := Encode(map[string]any{"val": math.Inf(1)}, DefaultConfig())
		if err == nil && !strings.Contains(result, "null") {
			t.Errorf("Infinity must surface as an error or null, got %q", result)
		}
	})

	t.Run("encode with EscapeUnicode + CJK", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.EscapeUnicode = true
		result, err := Encode(map[string]any{"msg": "中文テスト한글"}, cfg)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(result, "\\u") {
			t.Error("expected unicode escaping for CJK characters")
		}
	})

	t.Run("encode with FloatTruncate", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.FloatTruncate = true
		result, err := Encode(map[string]any{"val": 3.14159265358979}, cfg)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(result, "3.") {
			t.Errorf("expected float in result: %s", result)
		}
	})
}

// --- Concurrent access edge cases ---

func TestConcurrencyEdgeCases_Boundary(t *testing.T) {
	t.Run("PreParse then concurrent GetFromParsed", func(t *testing.T) {
		p, err := New()
		if err != nil {
			t.Fatalf("New() failed: %v", err)
		}
		defer p.Close()

		parsed, err := p.PreParse(`{"x":1,"y":2,"z":3}`)
		if err != nil {
			t.Fatalf("PreParse failed: %v", err)
		}
		defer parsed.Release()

		// Read from parsed data concurrently
		for i := 0; i < 10; i++ {
			val, err := p.GetFromParsed(parsed, "x")
			if err != nil {
				t.Errorf("GetFromParsed failed: %v", err)
			}
			if val != float64(1) {
				t.Errorf("val = %v, want 1", val)
			}
		}
	})

	t.Run("rapid create and close", func(t *testing.T) {
		for i := 0; i < 50; i++ {
			p, err := New()
			if err != nil {
				t.Fatalf("New() failed: %v", err)
			}
			p.Get(`{"test": "value"}`, "test")
			p.Close()
		}
	})
}

// --- validateFilePathStandalone (file.go:608, 0% coverage) ---

func TestValidateFilePath_Boundary(t *testing.T) {
	t.Run("path traversal attempt", func(t *testing.T) {
		err := validateFilePathStandalone("../../../etc/passwd")
		if err == nil {
			t.Error("expected error for path traversal")
		}
	})

	t.Run("null byte in path", func(t *testing.T) {
		err := validateFilePathStandalone("file\x00.txt")
		if err == nil {
			t.Error("expected error for null byte in path")
		}
	})
}

// TestValidateUnixPath_Boundary exercises the Unix path security check directly.
// validateUnixPath is gated behind runtime.GOOS != "windows" inside the public
// path-validation pipeline, so on Windows it is unreachable via the public API;
// the platform-specific logic is therefore unit-tested here.
func TestValidateUnixPath_Boundary(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		wantErr bool
	}{
		{"safe tmp path", "/tmp/safe.json", false},
		{"safe home path", "/home/user/data.json", false},
		{"dev blocked", "/dev/null", true},
		{"proc blocked", "/proc/self/status", true},
		{"etc passwd blocked", "/etc/passwd", true},
		{"etc shadow blocked", "/etc/shadow", true},
		{"root blocked", "/root/.bashrc", true},
		{"var log blocked", "/var/log/syslog", true},
		{"case-insensitive etc passwd", "/ETC/PASSWD", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateUnixPath(tt.path)
			if tt.wantErr && err == nil {
				t.Error("expected error blocking system path, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error for safe path: %v", err)
			}
		})
	}
}

// --- Config edge cases ---

func TestConfigEdgeCases_Boundary(t *testing.T) {
	t.Run("CacheTTL zero instant expiry", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.CacheTTL = 0
		cfg.EnableCache = true
		p, err := New(cfg)
		if err != nil {
			t.Fatalf("New() failed: %v", err)
		}
		defer p.Close()

		_, err = p.Get(`{"a":1}`, "a")
		if err != nil {
			t.Fatalf("Get failed: %v", err)
		}
	})

	t.Run("MaxCacheSize zero with EnableCache", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.MaxCacheSize = 0
		cfg.EnableCache = true
		if err := cfg.Validate(); err != nil {
			t.Fatalf("Validate failed: %v", err)
		}
	})
}
