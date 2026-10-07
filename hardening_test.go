package json

// GEN-001 production-hardening round: tests for the improvements introduced
// by the production-readiness audit — root-path Set, duplicate-key detection,
// the file-directory allowlist, SaveFileMode, the indicator-set case
// invariant, and TOCTOU-narrowed file opening.

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// ============================================================================
// Root-path Set (GEN-001 #10)
// ============================================================================

func TestRootSetReplacesDocument(t *testing.T) {
	p, err := New()
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer func() { _ = p.Close() }()

	cases := []struct {
		name string
		path string
		want string
	}{
		{"empty path", "", `42`},
		{"dot path", ".", `{"fresh":true}`},
		{"json pointer root", "/", `["x"]`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var value any
			switch tc.want {
			case `42`:
				value = 42
			case `{"fresh":true}`:
				value = map[string]any{"fresh": true}
			default:
				value = []string{"x"}
			}
			result, err := p.Set(`{"a":1}`, tc.path, value)
			if err != nil {
				t.Fatalf("root Set: %v", err)
			}
			assertJSONEqual(t, tc.want, result)
		})
	}

	// Root Get keeps returning the whole document.
	root, err := p.Get(`{"a":1}`, "")
	if err != nil {
		t.Fatalf("root Get: %v", err)
	}
	if _, ok := root.(map[string]any); !ok {
		t.Errorf("root Get should return the document, got %T", root)
	}

	// Root Delete remains unsupported.
	if _, err := p.Delete(`{"a":1}`, ""); err == nil {
		t.Error("expected error for root Delete")
	}
}

// ============================================================================
// Duplicate-key detection (GEN-001 #10)
// ============================================================================

func TestDetectDuplicateKeys(t *testing.T) {
	t.Run("default accepts last-wins", func(t *testing.T) {
		p, err := New()
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		defer func() { _ = p.Close() }()
		if _, err := p.Set(`{"a":1,"a":2}`, "a", 3); err != nil {
			t.Fatalf("default semantics must accept duplicate keys: %v", err)
		}
	})

	t.Run("opt-in rejects with ErrDuplicateKey", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.DetectDuplicateKeys = true
		p, err := New(cfg)
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		defer func() { _ = p.Close() }()

		_, err = p.Set(`{"a":1,"a":2}`, "a", 3)
		if !errors.Is(err, ErrDuplicateKey) {
			t.Fatalf("expected ErrDuplicateKey, got %v", err)
		}
	})

	t.Run("per-call config enables detection", func(t *testing.T) {
		p, err := New()
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		defer func() { _ = p.Close() }()

		_, err = p.Set(`{"a":1,"a":2}`, "a", 3, Config{DetectDuplicateKeys: true})
		if !errors.Is(err, ErrDuplicateKey) {
			t.Fatalf("expected ErrDuplicateKey from per-call cfg, got %v", err)
		}
	})

	t.Run("nested duplicates rejected", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.DetectDuplicateKeys = true
		p, err := New(cfg)
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		defer func() { _ = p.Close() }()

		_, err = p.Set(`{"a":1,"b":{"x":1,"x":2}}`, "a", 3)
		if !errors.Is(err, ErrDuplicateKey) {
			t.Fatalf("expected ErrDuplicateKey for nested duplicate, got %v", err)
		}
	})

	t.Run("array string repeats are not keys", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.DetectDuplicateKeys = true
		p, err := New(cfg)
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		defer func() { _ = p.Close() }()

		if _, err := p.Set(`["d","d"]`, "0", "e"); err != nil {
			t.Fatalf("repeated array elements must not be flagged: %v", err)
		}
	})

	t.Run("distinct keys accepted", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.DetectDuplicateKeys = true
		p, err := New(cfg)
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		defer func() { _ = p.Close() }()

		if _, err := p.Set(`{"a":1,"b":2}`, "a", 3); err != nil {
			t.Fatalf("distinct keys must be accepted: %v", err)
		}
	})
}

// ============================================================================
// File-directory allowlist (GEN-001 #4)
// ============================================================================

func TestAllowedFileDirs(t *testing.T) {
	allowed := t.TempDir()
	other := t.TempDir() // outside the allowlist

	cfg := DefaultConfig()
	cfg.AllowedFileDirs = []string{allowed}
	p, err := New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer func() { _ = p.Close() }()

	t.Run("write and read inside allowed dir", func(t *testing.T) {
		path := filepath.Join(allowed, "sub", "data.json")
		if err := p.SaveToFile(path, map[string]any{"k": 1}); err != nil {
			t.Fatalf("SaveToFile inside allowlist: %v", err)
		}
		if _, err := p.LoadFromFile(path); err != nil {
			t.Fatalf("LoadFromFile inside allowlist: %v", err)
		}
	})

	t.Run("write outside rejected", func(t *testing.T) {
		path := filepath.Join(other, "data.json")
		err := p.SaveToFile(path, map[string]any{"k": 1})
		if !errors.Is(err, ErrSecurityViolation) {
			t.Fatalf("expected ErrSecurityViolation, got %v", err)
		}
	})

	t.Run("read outside rejected", func(t *testing.T) {
		path := filepath.Join(other, "data.json")
		if err := os.WriteFile(path, []byte(`{"a":1}`), 0644); err != nil {
			t.Fatal(err)
		}
		_, err := p.LoadFromFile(path)
		if !errors.Is(err, ErrSecurityViolation) {
			t.Fatalf("expected ErrSecurityViolation, got %v", err)
		}
	})

	t.Run("per-call override replaces list", func(t *testing.T) {
		// The per-call list widens access to `other` but no longer includes
		// `allowed` — override semantics, not union.
		override := Config{AllowedFileDirs: []string{other}}
		if err := p.SaveToFile(filepath.Join(other, "ovr.json"), map[string]any{"k": 1}, override); err != nil {
			t.Fatalf("per-call allowlist should permit other: %v", err)
		}
		err := p.SaveToFile(filepath.Join(allowed, "ovr.json"), map[string]any{"k": 1}, override)
		if !errors.Is(err, ErrSecurityViolation) {
			t.Fatalf("per-call override should exclude allowed dir, got %v", err)
		}
	})
}

func TestAllowedFileDirsValidation(t *testing.T) {
	t.Run("relative entries are absolutized with a warning", func(t *testing.T) {
		cfg := Config{AllowedFileDirs: []string{"data", ""}}
		warnings := cfg.ValidateWithWarnings()

		if len(cfg.AllowedFileDirs) != 1 {
			t.Fatalf("empty entry must be dropped, got %v", cfg.AllowedFileDirs)
		}
		if !filepath.IsAbs(cfg.AllowedFileDirs[0]) {
			t.Errorf("relative entry must be absolutized, got %q", cfg.AllowedFileDirs[0])
		}
		found := false
		for _, w := range warnings {
			if w.Field == "AllowedFileDirs" {
				found = true
			}
		}
		if !found {
			t.Error("expected an AllowedFileDirs warning")
		}
	})

	t.Run("sibling-prefix directory is not inside", func(t *testing.T) {
		// /a/base-secret must not be reachable via allowlist /a/base
		if pathWithinDir("/a/base-secret/f.json", "/a/base") {
			t.Error("sibling directory with shared prefix must not match")
		}
		if !pathWithinDir("/a/base/f.json", "/a/base") {
			t.Error("direct child must match")
		}
	})
}

// ============================================================================
// SaveFileMode (GEN-001 #9)
// ============================================================================

func TestSaveFileMode(t *testing.T) {
	t.Run("new file honors configured mode", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("Windows only honors the read-only bit; permission-bit assertions are POSIX-only")
		}
		cfg := DefaultConfig()
		cfg.SaveFileMode = 0600
		p, err := New(cfg)
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		defer func() { _ = p.Close() }()

		path := filepath.Join(t.TempDir(), "secret.json")
		if err := p.SaveToFile(path, map[string]any{"k": 1}); err != nil {
			t.Fatalf("SaveToFile: %v", err)
		}
		fi, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if fi.Mode().Perm() != 0600 {
			t.Errorf("new file mode = %o, want 600", fi.Mode().Perm())
		}
	})

	t.Run("per-call override", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("POSIX-only permission-bit assertion")
		}
		p, err := New()
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		defer func() { _ = p.Close() }()

		path := filepath.Join(t.TempDir(), "secret.json")
		if err := p.SaveToFile(path, map[string]any{"k": 1}, Config{SaveFileMode: 0600}); err != nil {
			t.Fatalf("SaveToFile: %v", err)
		}
		fi, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if fi.Mode().Perm() != 0600 {
			t.Errorf("new file mode = %o, want 600", fi.Mode().Perm())
		}
	})

	t.Run("invalid bits are masked with a warning", func(t *testing.T) {
		cfg := Config{SaveFileMode: os.ModeDir | 0644}
		warnings := cfg.ValidateWithWarnings()
		if cfg.SaveFileMode != 0644 {
			t.Errorf("SaveFileMode = %v, want masked to 0644", cfg.SaveFileMode)
		}
		found := false
		for _, w := range warnings {
			if w.Field == "SaveFileMode" {
				found = true
			}
		}
		if !found {
			t.Error("expected a SaveFileMode warning")
		}
	})
}

// ============================================================================
// Indicator-set case invariant (GEN-001 #5)
// ============================================================================

// TestIndicatorCaseInvariant enforces the security.go indicatorChars
// invariant: every built-in pattern must contain at least one byte position
// whose every attacker-selectable rendering (both letter casings, or the
// fixed symbol byte) is an indicator byte. Otherwise the pattern can be
// written with zero indicator bytes and the large-input scan-skip shortcut
// would never scan it. Guards future pattern additions.
func TestIndicatorCaseInvariant(t *testing.T) {
	all := append(append([]dangerousPattern{}, dangerousPatterns...), criticalPatterns...)
	if len(all) == 0 {
		t.Fatal("pattern lists must not be empty")
	}
	for _, dp := range all {
		blocked := false
		for i := 0; i < len(dp.pattern); i++ {
			b := dp.pattern[i]
			isLower := b >= 'a' && b <= 'z'
			isUpper := b >= 'A' && b <= 'Z'
			if isLower || isUpper {
				other := b ^ 0x20 // flip case
				if indicatorChars[b] && indicatorChars[other] {
					blocked = true
					break
				}
			} else if indicatorChars[b] {
				blocked = true
				break
			}
		}
		if !blocked {
			t.Errorf("pattern %q can be rendered without any indicator byte; "+
				"add its letters (both cases) to indicatorChars", dp.pattern)
		}
	}
}

// TestIndicatorUppercaseNoFalsePositives: adding uppercase indicator bytes
// must not reject legitimate uppercase-only JSON.
func TestIndicatorUppercaseNoFalsePositives(t *testing.T) {
	p, err := New()
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer func() { _ = p.Close() }()

	// Uppercase letters trigger the full scan path now; none match a pattern.
	big := `["` + strings.Repeat("DATAVALUE", 600) + `"]` // >4KB, uppercase-only
	if _, err := p.Get(big, "[0]"); err != nil {
		t.Fatalf("uppercase-only payload must validate: %v", err)
	}
}

// ============================================================================
// TOCTOU-narrowed file open (GEN-001 #3)
// ============================================================================

func TestOpenValidatedFileSymlinkResolution(t *testing.T) {
	dir := t.TempDir()
	real := filepath.Join(dir, "real.json")
	if err := os.WriteFile(real, []byte(`{"ok":true}`), 0644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link.json")
	if err := os.Symlink(real, link); err != nil {
		t.Skipf("symlinks unavailable on this platform/session: %v", err)
	}

	p, err := New()
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer func() { _ = p.Close() }()

	// Reading through the link still works: the resolved physical path is
	// opened, and it passed the same validation.
	data, err := p.LoadFromFile(link)
	if err != nil {
		t.Fatalf("LoadFromFile via symlink: %v", err)
	}
	if !strings.Contains(data, `"ok"`) {
		t.Fatalf("expected file content via symlink, got %q", data)
	}
}

// ============================================================================
// U+2028/U+2029 escaping on the fast paths (GEN-001 must-fix #2, review round)
// ============================================================================

// TestLineSeparatorEscapingFastPaths pins encoding/json parity: the line
// separators U+2028/U+2029 must appear as  /  in EVERY output path,
// including the fast encoder consumed without the caller's HTML-escape
// post-pass (mutation results, Encoder with SetEscapeHTML(false)).
func TestLineSeparatorEscapingFastPaths(t *testing.T) {
	// lsRaw/psRaw carry real U+2028/U+2029 runes; the esc* constants hold
	// the six-character escape TEXT the output must contain (built via byte
	// 92 = backslash, avoiding literal escape sequences in this source).
	lsRaw := "x" + string(rune(0x2028)) + "y"
	psRaw := "x" + string(rune(0x2029)) + "y"
	escLS := string([]byte{92, 'u', '2', '0', '2', '8'})
	escPS := string([]byte{92, 'u', '2', '0', '2', '9'})
	escAmp := string([]byte{92, 'u', '0', '0', '2', '6'})

	t.Run("Set result escapes line separators", func(t *testing.T) {
		p, err := New()
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		defer func() { _ = p.Close() }()

		result, err := p.Set(`{"a":1}`, "a", lsRaw)
		if err != nil {
			t.Fatalf("Set: %v", err)
		}
		if !strings.Contains(result, escLS) {
			t.Errorf("Set result must carry the U+2028 escape text, got %q", result)
		}
		if strings.Contains(result, lsRaw) {
			t.Errorf("Set result leaked a raw U+2028: %q", result)
		}
	})

	t.Run("Encoder with SetEscapeHTML(false) still escapes", func(t *testing.T) {
		var buf bytes.Buffer
		enc := NewEncoder(&buf)
		enc.SetEscapeHTML(false)
		if err := enc.Encode(map[string]any{"v": lsRaw, "w": psRaw, "h": "<&>"}); err != nil {
			t.Fatalf("Encode: %v", err)
		}
		out := buf.String()
		if !strings.Contains(out, escLS) || !strings.Contains(out, escPS) {
			t.Errorf("line separators must be escaped even without EscapeHTML, got %q", out)
		}
		if strings.Contains(out, lsRaw) || strings.Contains(out, psRaw) {
			t.Errorf("raw line separator leaked: %q", out)
		}
		// The HTML option itself must still be honored: < and & stay raw.
		if !strings.Contains(out, "<&>") || strings.Contains(out, escAmp) {
			t.Errorf("SetEscapeHTML(false) must not escape HTML characters, got %q", out)
		}
	})

	t.Run("Marshal escapes (always-HTML contract)", func(t *testing.T) {
		out, err := Marshal(map[string]any{"v": lsRaw})
		if err != nil {
			t.Fatalf("Marshal: %v", err)
		}
		if !strings.Contains(string(out), escLS) {
			t.Errorf("Marshal must escape U+2028, got %q", out)
		}
	})
}

// ============================================================================
// sanitizeError redaction (GEN-001 must-fix #3, review round)
// ============================================================================

func TestSanitizeErrorRedaction(t *testing.T) {
	t.Run("nil error", func(t *testing.T) {
		if got := sanitizeError(nil); got != "" {
			t.Errorf("nil error should sanitize to empty, got %q", got)
		}
	})

	t.Run("sensitive keyword redacts whole message", func(t *testing.T) {
		if got := sanitizeError(errors.New("validation failed for api_key value")); got != "[REDACTED_ERROR]" {
			t.Errorf("expected full redaction, got %q", got)
		}
	})

	t.Run("long quoted spans are masked", func(t *testing.T) {
		msg := sanitizeError(fmt.Errorf("string 'aaaaaaaaaaaaaaaaaaaa' does not match pattern '^\\d+$'"))
		if strings.Contains(msg, "aaaaaaaaaaaaaaaaaaaa") {
			t.Errorf("long quoted value leaked: %q", msg)
		}
		if !strings.Contains(msg, "[REDACTED]") {
			t.Errorf("expected [REDACTED] markers, got %q", msg)
		}
	})

	t.Run("short quoted spans are kept for debugging", func(t *testing.T) {
		msg := sanitizeError(fmt.Errorf("failed to parse path 'a.b': bad segment"))
		if !strings.Contains(msg, "'a.b'") {
			t.Errorf("short spans should stay readable, got %q", msg)
		}
	})

	t.Run("unterminated quote fails closed", func(t *testing.T) {
		// The message also contains the sensitive keyword "token", so the
		// whole-message redaction fires first — either marker is acceptable.
		msg := sanitizeError(errors.New("value 'secret-token-value-truncated-at-200"))
		if strings.Contains(msg, "secret-token") {
			t.Errorf("unterminated span must be masked, got %q", msg)
		}
		if !strings.Contains(msg, "REDACTED") {
			t.Errorf("expected a redaction marker, got %q", msg)
		}
	})

	t.Run("messages without quotes pass through", func(t *testing.T) {
		const in = "context deadline exceeded while decoding"
		if got := sanitizeError(errors.New(in)); got != in {
			t.Errorf("plain message altered: %q", got)
		}
	})
}
