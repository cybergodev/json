package json

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// ============================================================================
// FIX-001 boundary tests: previously uncovered error branches and edge cases.
// Each test targets functions that coverage showed at <80%.
// ============================================================================

// TestValidateJSONInputEssential_Boundaries covers every failure branch of
// ValidateJSONInputEssential via a validator with tiny limits: size limit,
// empty input, nesting depth, and per-container counts.
func TestValidateJSONInputEssential_Boundaries(t *testing.T) {
	sv := newSecurityValidator(32, 100, 5, false, false, nil, 3, 3)
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
		{"zero width space", "file​.txt", true},
		{"soft hyphen", "file­.txt", true},
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

	if err := validatePathSymlinks(filepath.Join(dir, "does-not-exist")); err != nil {
		t.Errorf("nonexistent path should pass (nothing to resolve), got %v", err)
	}

	regular := filepath.Join(dir, "regular.txt")
	if err := os.WriteFile(regular, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := validatePathSymlinks(regular); err != nil {
		t.Errorf("regular file should pass, got %v", err)
	}

	link := filepath.Join(dir, "link.txt")
	err := os.Symlink(regular, link)
	if err != nil {
		t.Skipf("os.Symlink unavailable: %v", err)
	}
	if err := validatePathSymlinks(link); err != nil {
		t.Errorf("symlink to a regular file inside the temp dir should pass, got %v", err)
	}

	// A broken symlink cannot be resolved by EvalSymlinks.
	broken := filepath.Join(dir, "broken.txt")
	if err := os.Symlink(filepath.Join(dir, "gone.txt"), broken); err != nil {
		t.Skipf("os.Symlink unavailable: %v", err)
	}
	if err := validatePathSymlinks(broken); err == nil {
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
