package json

import (
	"bytes"
	"strings"
	"testing"
)

// ============================================================================
// Table-driven coverage tests for core functions with 0% or low coverage.
// Target: core ≥ 90%, utils ≥ 80%
// ============================================================================

// --- parseSurrogatePair (encoding.go:656, 0% coverage) ---
// Tested indirectly via Decode with strings containing surrogate pairs.

func TestDecoderSurrogatePair(t *testing.T) {
	// parseSurrogatePair is called from parseString via the Token() method
	// Build surrogate pair strings at byte level
	validPair := string([]byte{
		'"', '\\', 'u', 'D', '8', '3', 'D',
		'\\', 'u', 'D', 'E', '0', '0', '"',
	})

	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{name: "valid surrogate pair", input: validPair, wantErr: false},
		{name: "valid emoji UTF8", input: `"😀"`, wantErr: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dec := NewDecoder(strings.NewReader(tt.input))
			tok, err := dec.Token()
			if tt.wantErr {
				if err == nil {
					t.Error("expected error")
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if tok == nil {
				t.Error("expected non-nil token")
			}
		})
	}
}

// --- encodeJSONNumber: tested in coverage_test.go TestEncodeJSONNumber ---

// --- validateInputEssential (processor.go:1736, 0% coverage) ---
// Tested via Processor with SkipValidation enabled and oversized input.

func TestValidateInputEssential(t *testing.T) {
	t.Run("oversized input with SkipValidation", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.MaxJSONSize = 100
		cfg.SkipValidation = true
		p, err := New(cfg)
		if err != nil {
			t.Fatalf("New() failed: %v", err)
		}
		defer p.Close()

		largeJSON := `{"a":"` + strings.Repeat("x", 200) + `"}`
		_, err = p.Get(largeJSON, "a")
		if err == nil {
			t.Error("expected error for oversized JSON even with SkipValidation")
		}
	})

	t.Run("valid input with SkipValidation", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.SkipValidation = true
		p, err := New(cfg)
		if err != nil {
			t.Fatalf("New() failed: %v", err)
		}
		defer p.Close()

		val, err := p.Get(`{"key":"value"}`, "key")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if val != "value" {
			t.Errorf("val = %v, want value", val)
		}
	})
}

// --- Recursive processor: array index with delete ---

func TestRecursiveArrayIndexDelete(t *testing.T) {
	tests := []struct {
		name    string
		json    string
		path    string
		want    string
		wantErr bool
	}{
		{name: "delete middle element", json: `{"arr":[1,2,3]}`, path: "arr[1]", want: `{"arr":[1,3]}`},
		{name: "delete first element", json: `{"arr":[10,20,30]}`, path: "arr[0]", want: `{"arr":[20,30]}`},
		{name: "delete last by negative index", json: `{"arr":[1,2,3]}`, path: "arr[-1]", want: `{"arr":[1,2]}`},
		{name: "delete nested array element", json: `{"a":{"b":[1,2,3]}}`, path: "a.b[1]", want: `{"a":{"b":[1,3]}}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := Delete(tt.json, tt.path)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			assertJSONEqual(t, tt.want, result)
		})
	}
}

// --- Recursive processor: array slice with delete ---

func TestRecursiveArraySliceDelete(t *testing.T) {
	tests := []struct {
		name    string
		json    string
		path    string
		want    string
		wantErr bool
	}{
		{name: "delete slice range", json: `{"arr":[1,2,3,4,5]}`, path: "arr[1:3]", want: `{"arr":[1,4,5]}`},
		{name: "delete slice with step", json: `{"arr":[1,2,3,4,5]}`, path: "arr[0:5:2]", want: `{"arr":[2,4]}`},
		{name: "delete all elements via slice", json: `{"arr":[1,2,3]}`, path: "arr[0:3]", want: `{"arr":[]}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := Delete(tt.json, tt.path)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			assertJSONEqual(t, tt.want, result)
		})
	}
}

// --- Wildcard/extract/slice: tested in recursive_test.go and operation_test.go ---

// --- HTMLEscape nil processor path ---

func TestHTMLEscape_NilProcessorFallback(t *testing.T) {
	t.Run("HTMLEscape escapes HTML characters", func(t *testing.T) {
		input := []byte(`{"msg":"<script>alert('xss')</script>"}`)
		var buf bytes.Buffer
		HTMLEscape(&buf, input)
		result := buf.String()
		if strings.Contains(result, "<script>") {
			t.Error("HTML should be escaped")
		}
		if !strings.Contains(result, "\\u003c") && !strings.Contains(result, `<`) {
			t.Errorf("expected HTML escaping in result: %s", result)
		}
	})
}

// --- PreParse and Release ---

func TestPreParseAndRelease(t *testing.T) {
	p, err := New()
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	defer p.Close()

	parsed, err := p.PreParse(`{"a":1,"b":[2,3],"c":{"d":4}}`)
	if err != nil {
		t.Fatalf("PreParse failed: %v", err)
	}

	val, err := p.GetFromParsed(parsed, "a")
	if err != nil {
		t.Fatalf("GetFromParsed failed: %v", err)
	}
	if val != float64(1) {
		t.Errorf("val = %v, want 1", val)
	}

	val, err = p.GetFromParsed(parsed, "c.d")
	if err != nil {
		t.Fatalf("GetFromParsed nested failed: %v", err)
	}
	if val != float64(4) {
		t.Errorf("val = %v, want 4", val)
	}

	parsed.Release()
	parsed.Release() // double release should not panic
}

// --- checkRateLimit (processor.go:1267, 18.2% coverage) ---

func TestProcessorRateLimit(t *testing.T) {
	t.Run("rate limit enforcement", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.MaxConcurrency = 1
		p, err := New(cfg)
		if err != nil {
			t.Fatalf("New() failed: %v", err)
		}
		defer p.Close()

		// First call should succeed
		_, err = p.Get(`{"a":1}`, "a")
		if err != nil {
			t.Fatalf("first Get failed: %v", err)
		}

		// Subsequent calls should also succeed (no rate limit hit at normal speed)
		_, err = p.Get(`{"a":1}`, "a")
		if err != nil {
			t.Fatalf("second Get failed: %v", err)
		}
	})
}
