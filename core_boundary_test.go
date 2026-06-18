package json

import (
	"bytes"
	"errors"
	"reflect"
	"testing"
)

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
		// First Write call (data) succeeds; second (newline) fails.
		w := NewJSONLWriter(&failAfterNWriter{limit: 2})
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
		name   string
		got    int
		want   int
		has    bool
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
		if limits := c.getSecurityLimits(); limits != (SecurityLimits{}) {
			t.Errorf("nil getSecurityLimits = %+v, want zero", limits)
		}
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
